package boltdb

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cludden/concourse-go-sdk/pkg/archive/settings"
	"github.com/stretchr/testify/assert"
)

func TestArchive(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	dir := t.TempDir()
	err := os.Chdir(dir)
	if !assert.NoError(t, err) {
		return
	}

	id, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if !assert.NoError(t, err) {
		return
	}

	cfg := Config{
		Bucket:   fmt.Sprintf("test-%s", id.String()),
		Endpoint: "http://localhost:4566",
		Region:   "us-east-1",
		Key:      "my-team/my-pipeline/my-resource/archive.db",
		Credentials: &Credentials{
			AccessKey: "abc",
			SecretKey: "123",
		},
	}

	ctx := context.Background()

	// initialize test s3 client
	sess, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.Credentials.AccessKey, cfg.Credentials.SecretKey, cfg.Credentials.SessionToken)),
	)
	if !assert.NoError(t, err) {
		return
	}
	s3client := s3.NewFromConfig(sess,
		s3.WithEndpointResolver(s3.EndpointResolverFromURL(cfg.Endpoint)),
		func(o *s3.Options) {
			o.UsePathStyle = true
		},
	)

	// create test bucket
	if _, err := s3client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &cfg.Bucket,
	}); err != nil {
		t.Fatalf("error creating s3 bucket: %v", err)
	}
	defer func() {
		_, err = s3client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: &cfg.Bucket,
			Key:    &cfg.Key,
		})
		assert.NoError(t, err)
		_, err = s3client.DeleteBucket(ctx, &s3.DeleteBucketInput{
			Bucket: &cfg.Bucket,
		})
		assert.NoError(t, err)
	}()

	a, err := New(ctx, cfg, &settings.Settings{})
	if !assert.NoError(t, err) {
		return
	}

	history, err := a.History(ctx, nil)
	assert.NoError(t, err)
	assert.Len(t, history, 0)

	history = [][]byte{
		[]byte(`{"id":"foo"}`),
		[]byte(`{"id":"bar"}`),
		[]byte(`{"id":"baz"}`),
	}

	// load history
	err = a.Put(ctx, history...)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// retrieve and compare version order with latest version, no force history
	versions, err := a.History(ctx, []byte(`{"id":"baz"}`))
	assert.NoError(t, err)
	assert.Len(t, versions, 0)

	// retrieve and compare version order with no latest version
	versions, err = a.History(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, history, versions)

	// retrieve and compare version order with latest version, no force history
	a.settings.ForceHistory = true
	forcedHistory, err := a.History(ctx, []byte(`{"id":"baz"}`))
	assert.NoError(t, err)
	assert.Equal(t, history, forcedHistory)

	// close archive
	err = a.Close(ctx)
	assert.NoError(t, err)

	// open new archive
	a, err = New(ctx, cfg, &settings.Settings{})
	assert.NoError(t, err)
	defer func() {
		err = a.Close(ctx)
		assert.NoError(t, err)
	}()

	additional := [][]byte{
		[]byte(`{"id":"foo"}`),
		[]byte(`{"id":"z"}`),
		[]byte(`{"id":"x"}`),
		[]byte(`{"id":"y"}`),
		[]byte(`{"id":"a"}`),
		[]byte(`{"id":"b"}`),
		[]byte(`{"id":"c"}`),
		[]byte(`{"id":"g"}`),
		[]byte(`{"id":"A"}`),
	}

	// load additional
	err = a.Put(ctx, additional...)
	if !assert.NoError(t, err) {
		return
	}

	// retrieve and compare version order
	versions, err = a.History(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, [][]byte{
		[]byte(`{"id":"foo"}`),
		[]byte(`{"id":"bar"}`),
		[]byte(`{"id":"baz"}`),
		[]byte(`{"id":"z"}`),
		[]byte(`{"id":"x"}`),
		[]byte(`{"id":"y"}`),
		[]byte(`{"id":"a"}`),
		[]byte(`{"id":"b"}`),
		[]byte(`{"id":"c"}`),
		[]byte(`{"id":"g"}`),
		[]byte(`{"id":"A"}`),
	}, versions)
}
