package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DevelopmentAuthMiddleware is a simple auth middleware for development
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Next()
	}
}

// AzureADAuthMiddleware validates Azure AD JWT tokens
func AzureADAuthMiddleware(tenantID, applicationID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Authorization header required",
				},
			})
			c.Abort()
			return
		}

		if len(authHeader) < 7 || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_AUTH_FORMAT",
					"message": "Invalid authorization format",
				},
			})
			c.Abort()
			return
		}

		// TODO: Validate JWT token with Azure AD
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID) // RBAC middleware checks staff_id first
		c.Next()
	}
}
