package repo

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/sandipbera35/geo-post-api/models"
)

// eventsGeoKey is the Redis geo index all events are added to.
const eventsGeoKey = "events:geo"

// EventRepository persists LocalEvents and indexes them for geo queries.
type EventRepository interface {
	CreateEvent(ctx context.Context, event *models.LocalEvent) error
}

type eventRepository struct {
	db  *gorm.DB
	rdb *redis.Client
}

// NewEventRepository wires an EventRepository backed by Postgres (system of
// record) and Redis (GEOADD index for proximity queries).
func NewEventRepository(db *gorm.DB, rdb *redis.Client) EventRepository {
	return &eventRepository{db: db, rdb: rdb}
}

// CreateEvent writes event to Postgres, then indexes its coordinates in
// Redis via GEOADD. If the Redis step fails after a successful Postgres
// insert, the error is returned as-is and the row is left in Postgres —
// a production system would reconcile via an outbox, but that's out of
// scope here.
func (r *eventRepository) CreateEvent(ctx context.Context, event *models.LocalEvent) error {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	loc := &redis.GeoLocation{
		Name:      event.ID.String(),
		Longitude: event.Longitude,
		Latitude:  event.Latitude,
	}
	if err := r.rdb.GeoAdd(ctx, eventsGeoKey, loc).Err(); err != nil {
		return fmt.Errorf("geoadd event: %w", err)
	}

	return nil
}
