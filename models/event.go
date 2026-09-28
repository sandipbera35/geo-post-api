package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LocalEvent is a hyper-local, time-bound event post pinned to a lat/lng.
type LocalEvent struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	// UserID holds the JWT's sub claim verbatim: some issuers hand out UUIDs,
	// others (like this project's auth-server) use plain numeric ids, so it
	// is stored as an opaque string rather than parsed as a uuid.UUID.
	UserID      string    `gorm:"index;not null" json:"user_id"`
	Title       string    `gorm:"not null" json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Latitude    float64   `gorm:"not null" json:"latitude"`
	Longitude   float64   `gorm:"not null" json:"longitude"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	// MediaURL is optional: a pointer so an absent value is a real SQL NULL
	// (not an empty string) and is omitted from the JSON response entirely.
	MediaURL *string `json:"media_url,omitempty"`
}

// BeforeCreate fills in a client-side UUID and the default 24h expiry when
// the caller hasn't already set them, since GORM's static "default" tag
// can't express a computed "now + 24h" value.
func (e *LocalEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.ExpiresAt.IsZero() {
		e.ExpiresAt = time.Now().Add(24 * time.Hour)
	}
	return nil
}
