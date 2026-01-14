package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID middleware generates or extracts correlation IDs for request tracing
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID exists in header
		requestID := c.GetHeader("X-Request-ID")

		// Generate new UUID if not provided
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Set request ID in context and response header
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// StructuredLogger middleware logs requests with structured fields
func StructuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Calculate request duration
		duration := time.Since(start)

		// Get request ID
		requestID, _ := c.Get("request_id")

		// Log with structured fields
		log.Printf(
			"[%s] method=%s path=%s status=%d duration=%v ip=%s user_agent=%s request_id=%s",
			time.Now().Format(time.RFC3339),
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
			c.ClientIP(),
			c.Request.UserAgent(),
			requestID,
		)
	}
}

// TenantExtraction extracts tenant information and validates tenant access
// Supports multiple tenant identification methods:
// 1. X-Tenant-ID header (UUID)
// 2. X-Tenant-Slug header (slug string)
// 3. JWT claims (tenant_id from auth)
// 4. Query parameter (tenant_id or slug)
// 5. URL path parameter (:slug in routes)
func TenantExtraction() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get tenant ID from various sources
		tenantID := c.GetHeader("X-Tenant-ID")
		tenantSlug := c.GetHeader("X-Tenant-Slug")

		// If no tenant ID in header, check from JWT claims (set by auth middleware)
		if tenantID == "" {
			if jwtTenantID, exists := c.Get("tenant_id"); exists {
				tenantID = jwtTenantID.(string)
			}
		}

		// If still no tenant ID, check query parameter
		if tenantID == "" {
			tenantID = c.Query("tenant_id")
		}

		// Check for slug in query parameter
		if tenantSlug == "" {
			tenantSlug = c.Query("slug")
		}

		// Check for slug in URL path parameter (for routes like /tenants/:slug/...)
		if tenantSlug == "" {
			if slugParam := c.Param("slug"); slugParam != "" {
				tenantSlug = slugParam
			}
		}

		// Set tenant ID and slug in context for handlers to use
		if tenantID != "" {
			c.Set("tenant_id", tenantID)
		}
		if tenantSlug != "" {
			c.Set("tenant_slug", tenantSlug)
		}

		c.Next()
	}
}

// TenantContextKey is the context key for tenant context data
const (
	TenantIDKey   = "tenant_id"
	TenantSlugKey = "tenant_slug"
	UserIDKey     = "user_id"
	UserRoleKey   = "user_role"
)

// GetTenantID extracts tenant ID from gin context
func GetTenantID(c *gin.Context) string {
	if id, exists := c.Get(TenantIDKey); exists {
		return id.(string)
	}
	return ""
}

// GetTenantSlug extracts tenant slug from gin context
func GetTenantSlug(c *gin.Context) string {
	if slug, exists := c.Get(TenantSlugKey); exists {
		return slug.(string)
	}
	return ""
}

// GetUserID extracts user ID from gin context
func GetUserID(c *gin.Context) string {
	// First check context
	if id, exists := c.Get(UserIDKey); exists {
		return id.(string)
	}
	// Then check header (set by API gateway/auth)
	return c.GetHeader("X-User-ID")
}

// GetUserRole extracts user role from gin context
func GetUserRole(c *gin.Context) string {
	if role, exists := c.Get(UserRoleKey); exists {
		return role.(string)
	}
	return ""
}
