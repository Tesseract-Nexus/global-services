package middleware

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"analytics-service/internal/clients"
)

// TenantMiddlewareWithResolver creates a tenant middleware with slug resolution
func TenantMiddlewareWithResolver(tenantClient clients.TenantClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get tenant ID from context first (set by IstioAuth)
		tenantIDVal, _ := c.Get("tenant_id")
		tenantSlugOrID := ""
		if tenantIDVal != nil {
			tenantSlugOrID = tenantIDVal.(string)
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
		// Try to get tenant ID from context first (set by IstioAuth)
		tenantIDVal, _ := c.Get("tenant_id")
		tenantID := ""
		if tenantIDVal != nil {
			tenantID = tenantIDVal.(string)
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
