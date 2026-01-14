package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware extracts user information from request headers
// This middleware expects the proxy/gateway to pass user information
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health check endpoints
		if strings.HasPrefix(c.Request.URL.Path, "/health") ||
			strings.HasPrefix(c.Request.URL.Path, "/ready") {
			c.Next()
			return
		}

		// Extract user ID from headers (set by gateway/proxy)
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			userID = c.GetHeader("X-Staff-ID")
		}

		// Set user context for RBAC middleware
		if userID != "" {
			c.Set("user_id", userID)
			c.Set("staff_id", userID)
		}

		// Extract additional user info if available
		if userEmail := c.GetHeader("X-User-Email"); userEmail != "" {
			c.Set("user_email", userEmail)
		}

		if userName := c.GetHeader("X-User-Name"); userName != "" {
			c.Set("user_name", userName)
		}

		c.Next()
	}
}
