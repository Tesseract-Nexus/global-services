package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// TenantMiddleware ensures that every request has a valid tenant context
func TenantMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for health check endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/health/ready" || c.Request.URL.Path == "/health/live" {
			c.Next()
			return
		}

		// Try to get tenant ID from JWT claims (set by auth middleware)
		tenantID, exists := c.Get("tenant_id")
		if !exists {
			// Fallback to header
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		// SECURITY: No default fallback - fail closed
		if tenantID == "" || tenantID == nil {
			logger.Warn("Missing tenant ID - rejecting request")
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "TENANT_REQUIRED",
					"message": "Vendor/Tenant ID is required. Include X-Vendor-ID or X-Tenant-ID header.",
				},
			})
			c.Abort()
			return
		}

		// Validate tenant ID
		tenantStr, ok := tenantID.(string)
		if !ok || tenantStr == "" {
			logger.Warn("Invalid or missing tenant ID")
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TENANT",
					"message": "Valid tenant ID is required",
				},
			})
			c.Abort()
			return
		}

		// Set tenant ID in context for handlers to use
		c.Set("tenant_id", tenantStr)

		logger.WithField("tenant_id", tenantStr).Debug("Tenant context set")
		c.Next()
	}
}

// GetTenantID extracts tenant ID from gin context
func GetTenantID(c *gin.Context) string {
	if tenantID, exists := c.Get("tenant_id"); exists {
		if tenant, ok := tenantID.(string); ok {
			return tenant
		}
	}
	return ""
}

// GetUserID extracts user ID from gin context
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		if user, ok := userID.(string); ok {
			return user
		}
	}
	return ""
}

// GetUserEmail extracts user email from gin context
func GetUserEmail(c *gin.Context) string {
	if userEmail, exists := c.Get("user_email"); exists {
		if email, ok := userEmail.(string); ok {
			return email
		}
	}
	return ""
}

// GetUserRoles extracts user roles from gin context
func GetUserRoles(c *gin.Context) []string {
	if roles, exists := c.Get("user_roles"); exists {
		if userRoles, ok := roles.([]string); ok {
			return userRoles
		}
	}
	return []string{}
}

// HasRole checks if user has a specific role
func HasRole(c *gin.Context, role string) bool {
	roles := GetUserRoles(c)
	for _, userRole := range roles {
		if userRole == role || userRole == "super_admin" {
			return true
		}
	}
	return false
}

// HasAnyRole checks if user has any of the specified roles
func HasAnyRole(c *gin.Context, requiredRoles []string) bool {
	userRoles := GetUserRoles(c)
	for _, userRole := range userRoles {
		if userRole == "super_admin" {
			return true
		}
		for _, requiredRole := range requiredRoles {
			if userRole == requiredRole {
				return true
			}
		}
	}
	return false
}
