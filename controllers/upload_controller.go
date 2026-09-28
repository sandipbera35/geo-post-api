package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sandipbera35/geo-post-api/service"
)

// allowedUploadExtensions restricts the ?ext value used to build the
// storage object name, since it otherwise flows straight into a MinIO
// object key from an unvalidated query param.
var allowedUploadExtensions = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true,
}

// GenerateUploadURL handles GET /api/v1/uploads/presigned-url?ext=jpg,
// returning a presigned MinIO PUT URL and the public URL the uploaded
// object will be reachable at afterwards.
func GenerateUploadURL(svc service.UploadService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ext := strings.ToLower(strings.TrimPrefix(c.Query("ext"), "."))
		if !allowedUploadExtensions[ext] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ext must be one of: jpg, jpeg, png, gif, webp"})
			return
		}

		presignedURL, mediaURL, err := svc.GenerateUploadURL(c.Request.Context(), ext)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate upload url"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"presigned_url": presignedURL,
			"media_url":     mediaURL,
		})
	}
}
