package db

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/sandipbera35/geo-post-api/config"
)

// MediaBucket is the bucket all uploaded post media lives in.
const MediaBucket = "pulse-media"

// mediaBucketPublicReadPolicy grants anonymous s3:GetObject on every object
// in MediaBucket, so uploaded images can be viewed directly by the frontend
// without an auth round-trip. Writes still require a presigned URL.
const mediaBucketPublicReadPolicy = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::` + MediaBucket + `/*"]
		}
	]
}`

// NewMinio connects to an existing MinIO instance using cfg, ensures
// MediaBucket exists (creating it if needed), and makes it public-read.
func NewMinio(cfg *config.Config) (*minio.Client, error) {
	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to minio: %w", err)
	}

	ctx := context.Background()

	exists, err := client.BucketExists(ctx, MediaBucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q exists: %w", MediaBucket, err)
	}

	if !exists {
		if err := client.MakeBucket(ctx, MediaBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket %q: %w", MediaBucket, err)
		}
	}

	if err := client.SetBucketPolicy(ctx, MediaBucket, mediaBucketPublicReadPolicy); err != nil {
		return nil, fmt.Errorf("set public-read policy on bucket %q: %w", MediaBucket, err)
	}

	return client, nil
}
