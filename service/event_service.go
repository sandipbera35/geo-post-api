package service

import (
	"context"

	"github.com/sandipbera35/geo-post-api/models"
	"github.com/sandipbera35/geo-post-api/repo"
)

// EventService holds business logic for creating events, sitting between
// controllers and the repo layer.
type EventService interface {
	CreateEvent(ctx context.Context, event *models.LocalEvent) error
}

type eventService struct {
	repo repo.EventRepository
}

// NewEventService wires an EventService backed by repo.
func NewEventService(r repo.EventRepository) EventService {
	return &eventService{repo: r}
}

func (s *eventService) CreateEvent(ctx context.Context, event *models.LocalEvent) error {
	return s.repo.CreateEvent(ctx, event)
}
