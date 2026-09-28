package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sandipbera35/geo-post-api/service"
)

// defaultFeedRadiusKm is used when the request omits ?radius.
const defaultFeedRadiusKm = 5.0

// GetFeedHandler reads lat/lng/radius from the query string and returns
// nearby events via svc.GetNearbyEvents.
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

		events, err := svc.GetNearbyEvents(c.Request.Context(), lat, lng, radius)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch feed"})
			return
		}

		c.JSON(http.StatusOK, events)
	}
}
