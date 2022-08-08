package boltdb

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/boltdb/bolt"
	"github.com/fatih/color"
	"github.com/oklog/ulid/v2"
)

const (
	versionsBucket = "versions"
	indexBucket    = "versions_index"
)

type (
	Config struct {
		Bucket      string       `json:"bucket" validate:"required"`
		Credentials *Credentials `json:"credentials,omitempty" validate:"omitempty,dive"`
		Endpoint    string       `json:"endpoint"`
		Region      string       `json:"region" validate:"required"`
		Key         string       `json:"key" validate:"required"`
	}

	Credentials struct {
		AccessKey    string `json:"access_key" validate:"required"`
		SecretKey    string `json:"secret_key" validate:"required"`
		SessionToken string `json:"session_token"`
	}
)

type Archive struct {
	cfg   *Config
	db    *bolt.DB
	s3    *s3.Client
	stats bolt.BucketStats
}

func New(ctx context.Context, cfg Config) (*Archive, error) {
	a := &Archive{cfg: &cfg}
	if err := a.initS3(ctx); err != nil {
		return nil, err
	}

	file, err := a.downloadDB(ctx)
	if err != nil {
		return nil, err
	}

	if err := a.initDB(ctx, file); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *Archive) Close(ctx context.Context) error {
	var finalStats *bolt.BucketStats
	err := a.db.View(func(tx *bolt.Tx) error {
		stats := tx.Bucket([]byte(versionsBucket)).Stats()
		finalStats = &stats
		return nil
	})
	if err != nil {
		color.Red("error retrieving final bucket statistics")
	}

	if err := a.db.Close(); err != nil {
		return fmt.Errorf("error closing database: %v", err)
	}
	if finalStats != nil && a.stats.KeyN == finalStats.KeyN {
		return nil
	}

	f, err := os.Open("archive.db")
	if err != nil {
		return fmt.Errorf("error opening database file for upload: %v", err)
	}
	defer f.Close()

	_, err = a.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &a.cfg.Bucket,
		Key:    &a.cfg.Key,
		Body:   f,
	})
	return err
}

func (a *Archive) History(ctx context.Context) ([][]byte, error) {
	var history [][]byte
	err := a.db.View(func(tx *bolt.Tx) error {
		versions := tx.Bucket([]byte(versionsBucket))
		if versions == nil {
			return fmt.Errorf("database missing %s bucket", versionsBucket)
		}

		return versions.ForEach(func(_, v []byte) error {
			history = append(history, v)
			return nil
		})
	})
	return history, err
}

func (a *Archive) Put(ctx context.Context, next ...[]byte) error {
	return a.db.Update(func(tx *bolt.Tx) error {
		versions, err := tx.CreateBucketIfNotExists([]byte(versionsBucket))
		if err != nil {
			return fmt.Errorf("error creating versions bucket: %v", err)
		}

		index, err := tx.CreateBucketIfNotExists([]byte(indexBucket))
		if err != nil {
			return fmt.Errorf("error creating versions_index bucket: %v", err)
		}

		for _, version := range next {
			sum := sha1.Sum(version)
			if value := index.Get(sum[:]); value == nil {
				if err := index.Put(sum[:], []byte{}); err != nil {
					return fmt.Errorf("error updating index: %v", err)
				}

				id := ulid.Make()
				if err := versions.Put(id.Bytes(), version); err != nil {
					return fmt.Errorf("error updating versions: %v", err)
				}
			}
		}
		return nil
	})
}

// downloadDB downloads a boltdb file from s3
func (a *Archive) downloadDB(ctx context.Context) (string, error) {
	resp, err := a.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &a.cfg.Bucket,
		Key:    &a.cfg.Key,
	})
	if err != nil {
		var notFound *types.NoSuchKey
		if errors.As(err, &notFound) {
			return "archive.db", nil
		}
		return "", fmt.Errorf("error downloading database: %v", err)
	}
	defer resp.Body.Close()

	db, err := os.Create("archive.db")
	if err != nil {
		return "", fmt.Errorf("error creating archive.db: %v", err)
	}
	defer db.Close()

	if _, err := io.Copy(db, resp.Body); err != nil {
		return "", fmt.Errorf("error writing archive.db: %v", err)
	}
	return db.Name(), nil
}

// initDB initializes a bolt database
func (a *Archive) initDB(ctx context.Context, file string) error {
	db, err := bolt.Open(file, 0600, nil)
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		versions, err := tx.CreateBucketIfNotExists([]byte(versionsBucket))
		if err != nil {
			return fmt.Errorf("error creating versions bucket: %v", err)
		}
		a.stats = versions.Stats()

		_, err = tx.CreateBucketIfNotExists([]byte(indexBucket))
		if err != nil {
			return fmt.Errorf("error creating versions_index bucket: %v", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error initializing database: %v", err)
	}

	a.db = db
	return nil
}

// initS3 initializes an s3 client
func (a *Archive) initS3(ctx context.Context) error {
	if a.s3 != nil {
		return nil
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(a.cfg.Region),
	}
	if creds := a.cfg.Credentials; creds != nil {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(creds.AccessKey, creds.SecretKey, creds.SessionToken)))
	}

	sess, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("error initializing aws session: %v", err)
	}

	var s3opts []func(*s3.Options)
	if a.cfg.Endpoint != "" {
		s3opts = append(s3opts,
			s3.WithEndpointResolver(s3.EndpointResolverFromURL(a.cfg.Endpoint)),
			func(o *s3.Options) {
				o.UsePathStyle = true
			},
		)
	}
	a.s3 = s3.NewFromConfig(sess, s3opts...)
	return nil
}
