package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/sandipbera35/geo-post-api/db"
	"github.com/sandipbera35/geo-post-api/repo"
)

// uploadURLExpiry is how long a presigned PUT URL stays valid.
const uploadURLExpiry = 15 * time.Minute

// UploadService issues presigned upload URLs for post media.
type UploadService interface {
	GenerateUploadURL(ctx context.Context, ext string) (presignedURL, mediaURL string, err error)
}

type uploadService struct {
	repo     repo.UploadRepository
	endpoint string
	useSSL   bool
}

// NewUploadService wires an UploadService backed by repo. endpoint/useSSL
// are used to build the public media_url returned alongside the presigned
// URL (they describe how the frontend reaches MinIO, not the presigned URL
// itself, which the repo layer already builds from its own client config).
func NewUploadService(r repo.UploadRepository, endpoint string, useSSL bool) UploadService {
	return &uploadService{repo: r, endpoint: endpoint, useSSL: useSSL}
}

// GenerateUploadURL creates a unique object name (uuid.ext) and returns a
// presigned PUT URL for it plus the public URL it will be reachable at
// once uploaded.
func (s *uploadService) GenerateUploadURL(ctx context.Context, ext string) (string, string, error) {
	objectName := fmt.Sprintf("%s.%s", uuid.New().String(), ext)

	presignedURL, err := s.repo.PresignedPutURL(ctx, objectName, uploadURLExpiry)
	if err != nil {
		return "", "", err
	}

	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	mediaURL := fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, db.MediaBucket, objectName)

	return presignedURL, mediaURL, nil
}
