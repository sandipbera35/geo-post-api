package service

import (
	"context"
	"time"

	"github.com/sandipbera35/geo-post-api/models"
	"github.com/sandipbera35/geo-post-api/repo"
)

// Author is the enriched author info attached to each feed item. It comes
// from a Redis cache the Auth service populates asynchronously — this
// service never calls Auth synchronously in the request path.
type Author struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// FeedResponseItem is a LocalEvent enriched with its author's profile.
type FeedResponseItem struct {
	models.LocalEvent
	Author Author `json:"author"`
}

// fallbackAuthor is used when a user's profile isn't cached in Redis yet.
func fallbackAuthor(userID string) Author {
	return Author{ID: userID, Name: "Unknown User"}
}

// FeedService holds business logic for proximity queries, sitting between
// controllers and the repo layer.
type FeedService interface {
	// GetNearbyEvents returns up to limit enriched events within radiusKm
	// of (lat, lng), strictly older than cursor, newest first.
	GetNearbyEvents(ctx context.Context, lat, lng, radiusKm float64, cursor time.Time, limit int) ([]FeedResponseItem, error)
}

type feedService struct {
	repo        repo.FeedRepository
	profileRepo repo.UserProfileRepository
}

// NewFeedService wires a FeedService backed by repo (events) and
// profileRepo (author enrichment).
func NewFeedService(r repo.FeedRepository, profileRepo repo.UserProfileRepository) FeedService {
	return &feedService{repo: r, profileRepo: profileRepo}
}

func (s *feedService) GetNearbyEvents(ctx context.Context, lat, lng, radiusKm float64, cursor time.Time, limit int) ([]FeedResponseItem, error) {
	events, err := s.repo.GetNearbyEvents(ctx, lat, lng, radiusKm, cursor, limit)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return []FeedResponseItem{}, nil
	}

	userIDs := make([]string, 0, len(events))
	seen := make(map[string]bool, len(events))
	for _, e := range events {
		if !seen[e.UserID] {
			seen[e.UserID] = true
			userIDs = append(userIDs, e.UserID)
		}
	}

	profiles, err := s.profileRepo.GetProfiles(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	items := make([]FeedResponseItem, len(events))
	for i, e := range events {
		author := fallbackAuthor(e.UserID)
		if p, ok := profiles[e.UserID]; ok {
			author = Author{ID: p.ID, Name: p.Name, AvatarURL: p.AvatarURL}
		}
		items[i] = FeedResponseItem{LocalEvent: e, Author: author}
	}

	return items, nil
}
