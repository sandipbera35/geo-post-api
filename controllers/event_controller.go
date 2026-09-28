package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sandipbera35/geo-post-api/models"
	"github.com/sandipbera35/geo-post-api/service"
)

// createEventRequest is the wire-level DTO for POST /api/v1/events.
type createEventRequest struct {
	Title       string  `json:"title" binding:"required"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Latitude    float64 `json:"latitude" binding:"min=-90,max=90"`
	Longitude   float64 `json:"longitude" binding:"min=-180,max=180"`
	MediaURL    *string `json:"media_url"`
}

// CreateEventHandler binds a createEventRequest, attaches the caller's
// user_id (set by middlewares.RequireAuth), and persists it via svc.
func CreateEventHandler(svc service.EventService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createEventRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		event := &models.LocalEvent{
			UserID:      c.GetString("user_id"),
			Title:       req.Title,
			Description: req.Description,
			Category:    req.Category,
			Latitude:    req.Latitude,
			Longitude:   req.Longitude,
			MediaURL:    req.MediaURL,
		}

		if err := svc.CreateEvent(c.Request.Context(), event); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create event"})
			return
		}

		c.JSON(http.StatusCreated, event)
	}
}
