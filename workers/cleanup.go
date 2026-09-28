package workers

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/sandipbera35/geo-post-api/models"
)

// eventsGeoKey mirrors the constant of the same name in repo/event_repo.go.
// It's duplicated rather than imported so this package doesn't need to
// depend on repo just for one shared string.
const eventsGeoKey = "events:geo"

// StartExpiryWorker runs a cleanup loop every interval, deleting LocalEvents
// whose ExpiresAt has passed from both the events:geo Redis index and
// Postgres. It blocks forever on its own time.Ticker, so callers launch it
// with go, e.g.:
//
//	go workers.StartExpiryWorker(pg, rdb, 15*time.Minute)
func StartExpiryWorker(db *gorm.DB, redisClient *redis.Client, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		cleanupExpiredEvents(db, redisClient)
	}
}

// cleanupExpiredEvents deletes one batch of expired events. Redis is
// cleaned up before Postgres: leaving a stale id in the geo index (if
// Postgres delete failed) only means a dead entry gets filtered out by the
// next feed hydration query, whereas the reverse order could leave a
// nearby-search returning ids for rows that no longer exist.
func cleanupExpiredEvents(db *gorm.DB, redisClient *redis.Client) {

	ctx := context.Background()

	var ids []uuid.UUID
	if err := db.WithContext(ctx).
		Model(&models.LocalEvent{}).
		Where("expires_at < ?", time.Now()).
		Pluck("id", &ids).Error; err != nil {
		log.Printf("expiry worker: query expired events: %v", err)
		return
	}

	if len(ids) == 0 {
		return
	}

	members := make([]any, len(ids))
	for i, id := range ids {
		members[i] = id.String()
	}
	if err := redisClient.ZRem(ctx, eventsGeoKey, members...).Err(); err != nil {
		log.Printf("expiry worker: zrem %d expired event(s) from %s: %v", len(ids), eventsGeoKey, err)
		return
	}

	if err := db.WithContext(ctx).Where("id IN ?", ids).Delete(&models.LocalEvent{}).Error; err != nil {
		log.Printf("expiry worker: delete %d expired event(s) from postgres: %v", len(ids), err)
		return
	}

	log.Printf("expiry worker: cleaned up %d expired event(s)", len(ids))
}
