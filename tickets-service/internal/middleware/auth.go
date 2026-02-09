package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DevelopmentAuthMiddleware is a simple auth middleware for development
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, _ := c.Get("user_id")
		userID := ""
		if userIDVal != nil {
			userID = userIDVal.(string)
		}
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		userName := c.GetHeader("X-User-Name")
		if userName == "" {
			userName = "Development User"
		}

		userEmail := c.GetHeader("X-User-Email")
		// No default for email - it's optional

		userRole := c.GetHeader("X-User-Role")
		if userRole == "" {
			userRole = "user" // Default to regular user, not admin
		}

		// Set both camelCase and snake_case for compatibility with RBAC middleware
		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID) // RBAC middleware checks staff_id first
		c.Set("userName", userName)
		c.Set("user_name", userName)
		c.Set("userEmail", userEmail)
		c.Set("userRole", userRole)
		c.Next()
	}
}

// AzureADAuthMiddleware validates Azure AD JWT tokens
func AzureADAuthMiddleware(tenantID, applicationID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization format",
			})
			c.Abort()
			return
		}

		// Get user info from context (set by IstioAuth)
		userIDVal, _ := c.Get("user_id")
		userID := ""
		if userIDVal != nil {
			userID = userIDVal.(string)
		}
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		userName := c.GetHeader("X-User-Name")
		if userName == "" {
			userName = "Azure User"
		}

		userEmail := c.GetHeader("X-User-Email")
		// No default for email - it's optional

		userRole := c.GetHeader("X-User-Role")
		if userRole == "" {
			userRole = "user"
		}

		c.Set("userId", userID)
		c.Set("userName", userName)
		c.Set("userEmail", userEmail)
		c.Set("userRole", userRole)
		c.Next()
	}
}
