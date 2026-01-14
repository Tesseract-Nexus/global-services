package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tesseract-hub/analytics-service/internal/clients"
)

// TenantMiddlewareWithResolver creates a tenant middleware with slug resolution
func TenantMiddlewareWithResolver(tenantClient clients.TenantClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get tenant ID from X-Vendor-ID header first (standard)
		tenantSlugOrID := c.GetHeader("X-Vendor-ID")

		// Fall back to X-Tenant-ID header (legacy)
		if tenantSlugOrID == "" {
			tenantSlugOrID = c.GetHeader("X-Tenant-ID")
		}

		// Require tenant ID for all analytics requests
		if tenantSlugOrID == "" {
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

		// Resolve tenant slug to UUID
		tenantID, err := tenantClient.ResolveTenantID(c.Request.Context(), tenantSlugOrID)
		if err != nil {
			log.Printf("[TenantMiddleware] Failed to resolve tenant ID for %s: %v", tenantSlugOrID, err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "TENANT_NOT_FOUND",
					"message": "Unable to resolve tenant. Tenant not found or invalid.",
				},
			})
			c.Abort()
			return
		}

		// Set resolved tenant UUID in context for use by handlers
		c.Set("tenantId", tenantID)
		c.Set("tenant_id", tenantID)
		c.Set("vendor_id", tenantID)
		c.Set("tenant_slug", tenantSlugOrID) // Keep original slug for reference
		c.Next()
	}
}

// TenantMiddleware extracts and validates tenant information (legacy - no resolution)
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get tenant ID from X-Vendor-ID header first (standard)
		tenantID := c.GetHeader("X-Vendor-ID")

		// Fall back to X-Tenant-ID header (legacy)
		if tenantID == "" {
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		// Require tenant ID for all analytics requests
		if tenantID == "" {
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

		// Set tenant ID in context for use by handlers
		c.Set("tenantId", tenantID)
		c.Set("tenant_id", tenantID)
		c.Set("vendor_id", tenantID)
		c.Next()
	}
}

// GetTenantID retrieves the tenant ID from gin context
func GetTenantID(c *gin.Context) string {
	if tid := c.GetString("tenant_id"); tid != "" {
		return tid
	}
	return c.GetString("tenantId")
}
