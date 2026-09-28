package service

import (
	"context"

	"github.com/sandipbera35/geo-post-api/models"
	"github.com/sandipbera35/geo-post-api/repo"
)

// FeedService holds business logic for proximity queries, sitting between
// controllers and the repo layer.
type FeedService interface {
	GetNearbyEvents(ctx context.Context, lat, lng, radiusKm float64) ([]models.LocalEvent, error)
}

type feedService struct {
	repo repo.FeedRepository
}

// NewFeedService wires a FeedService backed by repo.
func NewFeedService(r repo.FeedRepository) FeedService {
	return &feedService{repo: r}
}

func (s *feedService) GetNearbyEvents(ctx context.Context, lat, lng, radiusKm float64) ([]models.LocalEvent, error) {
	return s.repo.GetNearbyEvents(ctx, lat, lng, radiusKm)
}
