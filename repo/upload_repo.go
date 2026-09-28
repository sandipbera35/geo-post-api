package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"

	"github.com/sandipbera35/geo-post-api/db"
)

// UploadRepository generates presigned URLs for direct-to-storage uploads.
type UploadRepository interface {
	PresignedPutURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
}

type uploadRepository struct {
	client *minio.Client
}

// NewUploadRepository wires an UploadRepository backed by an existing
// MinIO client (see db.NewMinio).
func NewUploadRepository(client *minio.Client) UploadRepository {
	return &uploadRepository{client: client}
}

// PresignedPutURL returns a URL that lets the caller PUT an object named
// objectName directly into db.MediaBucket, valid for expiry.
func (r *uploadRepository) PresignedPutURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	u, err := r.client.PresignedPutObject(ctx, db.MediaBucket, objectName, expiry)
	if err != nil {
		return "", fmt.Errorf("presigned put url: %w", err)
	}
	return u.String(), nil
}
