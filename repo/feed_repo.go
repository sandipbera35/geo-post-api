package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/sandipbera35/geo-post-api/models"
)

// FeedRepository answers proximity queries against the events:geo Redis
// index, hydrated from Postgres.
type FeedRepository interface {
	// GetNearbyEvents returns up to limit events within radiusKm of (lat,
	// lng), strictly older than cursor, newest first.
	GetNearbyEvents(ctx context.Context, lat, lng, radiusKm float64, cursor time.Time, limit int) ([]models.LocalEvent, error)
}

type feedRepository struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewFeedRepository wires a FeedRepository backed by the same Postgres
// table and Redis geo index that EventRepository writes to.
func NewFeedRepository(db *gorm.DB, rdb *redis.Client) FeedRepository {
	return &feedRepository{db: db, rdb: rdb}
}

// GetNearbyEvents first fetches ALL nearby event ids from Redis via
// GEOSEARCH (the non-deprecated replacement for GEORADIUS — cheap, no
// pagination needed on this side), then applies cursor-based pagination in
// Postgres: only ids strictly older than cursor, newest-first, capped at
// limit.
func (r *feedRepository) GetNearbyEvents(ctx context.Context, lat, lng, radiusKm float64, cursor time.Time, limit int) ([]models.LocalEvent, error) {
	locations, err := r.rdb.GeoSearchLocation(ctx, eventsGeoKey, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lng,
			Latitude:   lat,
			Radius:     radiusKm,
			RadiusUnit: "km",
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("geosearch nearby events: %w", err)
	}

	if len(locations) == 0 {
		return []models.LocalEvent{}, nil
	}

	ids := make([]string, len(locations))
	for i, loc := range locations {
		ids[i] = loc.Name
	}

	var events []models.LocalEvent
	if err := r.db.WithContext(ctx).
		Where("id IN ? AND created_at < ?", ids, cursor).
		Order("created_at DESC").
		Limit(limit).
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("query nearby events: %w", err)
	}

	return events, nil
}
