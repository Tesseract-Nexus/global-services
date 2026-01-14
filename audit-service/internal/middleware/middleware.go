package middleware

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupCORS configures CORS middleware
func SetupCORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://localhost:3004",
			"http://localhost:4200",
			"https://*.civica.tech",
			"https://*.tesserix.app",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "accept", "origin", "Cache-Control", "X-Requested-With", "X-Tenant-ID", "X-User-ID"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}

// Logger returns a gin.HandlerFunc for logging requests
func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

// Recovery returns a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			log.Printf("Panic recovered: %s", err)
			c.JSON(500, gin.H{
				"error":   "Internal Server Error",
				"message": "An unexpected error occurred",
			})
		}
		c.AbortWithStatus(500)
	})
}

// RequestID adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// TenantID middleware extracts tenant ID from headers
func TenantID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = c.Query("tenantId")
		}
		if tenantID != "" {
			c.Set("tenant_id", tenantID)
		}
		c.Next()
	}
}

// RequireTenantID middleware requires tenant ID for all requests
func RequireTenantID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID == "" {
			tenantID = c.Query("tenantId")
		}
		if tenantID == "" {
			c.AbortWithStatusJSON(400, gin.H{
				"error":   "MISSING_TENANT_ID",
				"message": "X-Tenant-ID header is required for multi-tenant isolation",
			})
			return
		}
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

// ValidateTenantUUID ensures tenant ID is a valid UUID format
func ValidateTenantUUID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, exists := c.Get("tenant_id")
		if !exists {
			c.Next()
			return
		}

		tenantStr, ok := tenantID.(string)
		if !ok {
			c.AbortWithStatusJSON(400, gin.H{
				"error":   "INVALID_TENANT_ID",
				"message": "Tenant ID must be a string",
			})
			return
		}

		if len(tenantStr) != 36 {
			c.AbortWithStatusJSON(400, gin.H{
				"error":   "INVALID_TENANT_ID_FORMAT",
				"message": "Tenant ID must be a valid UUID",
			})
			return
		}

		c.Next()
	}
}
