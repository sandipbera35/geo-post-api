package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sandipbera35/geo-post-api/service"
)

const (
	// defaultFeedRadiusKm is used when the request omits ?radius.
	defaultFeedRadiusKm = 5.0
	// defaultFeedLimit and maxFeedLimit bound ?limit.
	defaultFeedLimit = 20
	maxFeedLimit     = 50
)

// GetFeedHandler reads lat/lng/radius/limit/cursor from the query string
// and returns a page of enriched nearby events via svc.GetNearbyEvents,
// plus a next_cursor for the following page.
func GetFeedHandler(svc service.FeedService) gin.HandlerFunc {
	return func(c *gin.Context) {
		lat, err := strconv.ParseFloat(c.Query("lat"), 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing lat"})
			return
		}

		lng, err := strconv.ParseFloat(c.Query("lng"), 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing lng"})
			return
		}

		radius := defaultFeedRadiusKm
		if raw := c.Query("radius"); raw != "" {
			radius, err = strconv.ParseFloat(raw, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid radius"})
				return
			}
		}

		limit := defaultFeedLimit
		if raw := c.Query("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
				return
			}
			limit = parsed
		}
		if limit > maxFeedLimit {
			limit = maxFeedLimit
		}

		cursor := time.Now()
		if raw := c.Query("cursor"); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor, expected RFC3339"})
				return
			}
			cursor = parsed
		}

		items, err := svc.GetNearbyEvents(c.Request.Context(), lat, lng, radius, cursor, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch feed"})
			return
		}

		var nextCursor *string
		if len(items) > 0 {
			nc := items[len(items)-1].CreatedAt.Format(time.RFC3339)
			nextCursor = &nc
		}

		c.JSON(http.StatusOK, gin.H{
			"events":      items,
			"next_cursor": nextCursor,
		})
	}
}
