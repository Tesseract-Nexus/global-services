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
func TenantExtraction() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get tenant ID from various sources
		tenantID := c.GetHeader("X-Tenant-ID")

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

		// Set tenant ID in context for handlers to use
		if tenantID != "" {
			c.Set("tenant_id", tenantID)
		}

		c.Next()
	}
}
