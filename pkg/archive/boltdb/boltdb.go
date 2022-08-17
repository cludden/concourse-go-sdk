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
	"github.com/cludden/concourse-go-sdk/pkg/archive/settings"
	"github.com/fatih/color"
	"github.com/oklog/ulid/v2"
)

const (
	versionsBucket = "versions"
	indexBucket    = "versions_index"
)

type (
	// Config describes the available resource-specific configuration settings
	Config struct {
		// The bucket name where the boltdb database file is persisted in between builds
		Bucket string `json:"bucket" validate:"required"`
		// AWS session credentials
		Credentials *Credentials `json:"credentials,omitempty" validate:"omitempty,dive"`
		// A custom S3 endpoint, useful for testing
		Endpoint string `json:"endpoint"`
		// The AWS region where the bucket was created
		Region string `json:"region" validate:"required"`
		// The fully qualified S3 object key used for persisting the database file in
		// between builds
		Key string `json:"key" validate:"required"`
	}

	// Credentials describes AWS session credentials used for authenticating with S3
	Credentials struct {
		// The AWS_ACCESS_KEY_ID value to use for authenticating with S3
		AccessKey string `json:"access_key" validate:"required"`
		// The AWS_SECRET_ACCESS_KEY value to use for authenticating with S3
		SecretKey string `json:"secret_key" validate:"required"`
		// The AWS_SESSION_TOKEN value to use for authenticating with S3
		SessionToken string `json:"session_token"`
	}
)

// Archive implements a resource version archive using BoltDB backed by AWS S3.
type Archive struct {
	cfg      *Config
	db       *bolt.DB
	s3       *s3.Client
	settings *settings.Settings
	stats    bolt.BucketStats
}

func New(ctx context.Context, cfg Config, s *settings.Settings) (*Archive, error) {
	a := &Archive{cfg: &cfg, settings: s}
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

func (a *Archive) History(ctx context.Context, latest []byte) (history [][]byte, err error) {
	// exit early if concourse has version history
	if latest != nil && !a.settings.ForceHistory {
		return nil, nil
	}

	err = a.db.View(func(tx *bolt.Tx) error {
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
				id := ulid.Make().Bytes()
				if err := index.Put(sum[:], id); err != nil {
					return fmt.Errorf("error updating index: %v", err)
				}

				if err := versions.Put(id, version); err != nil {
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
