package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DevelopmentAuthMiddleware is a simple auth middleware for development
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In development, we'll use simple header-based auth
		userIDVal, _ := c.Get("user_id")
		userID := ""
		if userIDVal != nil {
			userID = userIDVal.(string)
		}
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID) // RBAC middleware checks staff_id first
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

		// Extract token from "Bearer <token>"
		if len(authHeader) < 7 || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_AUTH_FORMAT",
					"message": "Invalid authorization format. Expected: Bearer <token>",
				},
			})
			c.Abort()
			return
		}

		// TODO: Validate JWT token with Azure AD
		// token := authHeader[7:]
		// claims, err := validateAzureADToken(token, tenantID, applicationID)

		// For now, extract user ID from context or set default
		userIDVal, _ := c.Get("user_id")
		userID := ""
		if userIDVal != nil {
			userID = userIDVal.(string)
		}
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}

		c.Set("userId", userID)
		c.Set("user_id", userID)
		c.Set("staff_id", userID) // RBAC middleware checks staff_id first
		c.Next()
	}
}

// ServiceAuthMiddleware validates service-to-service authentication
// Used for internal API calls from other services
func ServiceAuthMiddleware(serviceAPIKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-Service-API-Key")
		if apiKey == "" {
			apiKey = c.GetHeader("X-API-Key")
		}

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "API_KEY_REQUIRED",
					"message": "X-Service-API-Key header is required for service calls",
				},
			})
			c.Abort()
			return
		}

		if apiKey != serviceAPIKey {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_API_KEY",
					"message": "Invalid service API key",
				},
			})
			c.Abort()
			return
		}

		c.Set("is_service_call", true)
		c.Next()
	}
}

// RoleMiddleware checks if user has required role
func RoleMiddleware(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user roles from context (set by auth middleware)
		rolesInterface, exists := c.Get("user_roles")
		if !exists {
			// Check header as fallback
			roleHeader := c.GetHeader("X-User-Roles")
			if roleHeader != "" {
				rolesInterface = strings.Split(roleHeader, ",")
			}
		}

		if rolesInterface == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NO_ROLES",
					"message": "No roles found for user",
				},
			})
			c.Abort()
			return
		}

		userRoles, ok := rolesInterface.([]string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ROLES",
					"message": "Invalid roles format",
				},
			})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, required := range requiredRoles {
			for _, userRole := range userRoles {
				if strings.TrimSpace(userRole) == required {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INSUFFICIENT_PERMISSIONS",
					"message": "User does not have required permissions",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
