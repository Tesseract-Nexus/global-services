package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth middleware validates API key for inter-service authentication
func APIKeyAuth(validAPIKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")

		// Also check Authorization header with "Bearer" prefix
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "API key is required",
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Missing API key",
				},
			})
			c.Abort()
			return
		}

		if apiKey != validAPIKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid API key",
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid API key",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
