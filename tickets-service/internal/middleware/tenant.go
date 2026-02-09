package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts and validates tenant information
// SECURITY: No default tenant fallback - requests without tenant context are rejected
// NOTE: First checks if tenant_id was already set by IstioAuth middleware
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First, check if tenant_id was already set by IstioAuth middleware
		tenantID := c.GetString("tenant_id")

		// If not set by IstioAuth, try X-Vendor-ID header (standard)
		if tenantID == "" {
			tenantID = c.GetHeader("X-Vendor-ID")
		}

		// Fall back to X-Tenant-ID header (legacy)
		if tenantID == "" {
			tenantID = c.GetHeader("X-Tenant-ID")
		}

		// SECURITY: No default fallback - fail closed
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

		// Set tenant ID in context for use by handlers (both keys for compatibility)
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

// GetVendorID retrieves the vendor ID from gin context
func GetVendorID(c *gin.Context) string {
	if vid := c.GetString("vendor_id"); vid != "" {
		return vid
	}
	return GetTenantID(c)
}
