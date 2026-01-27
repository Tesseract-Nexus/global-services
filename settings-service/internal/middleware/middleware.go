package middleware

import (
	"fmt"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupCORS configures CORS middleware
func SetupCORS() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins: []string{
			"http://localhost:3000", // Next.js storefront / API Gateway
			"http://localhost:4200", // Admin shell app (NEW PORT)
			"http://localhost:4201", // Tenant onboarding app
			"http://localhost:4301", // Categories MFE
			"http://localhost:4302", // Products MFE
			"http://localhost:4303", // Orders MFE
			"http://localhost:4304", // Coupons MFE
			"http://localhost:4305", // Reviews MFE
			"http://localhost:4306", // Staff MFE
			"http://localhost:4307", // Tickets MFE
			"http://localhost:4308", // User Management MFE
			"http://localhost:4309", // Vendor MFE
			"https://admin.tesseract-hub.com",
			"https://app.tesseract-hub.com",
			"https://storefront.tesseract-hub.com",
		},
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization",
			"X-Requested-With", "X-Tenant-ID", "X-User-ID", "X-Application-ID",
		},
		ExposeHeaders: []string{
			"Content-Length", "X-Total-Count",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	return cors.New(config)
}

// RequestLogger logs HTTP requests
func RequestLogger() gin.HandlerFunc {
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

// Recovery recovers from panics
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			c.JSON(500, gin.H{
				"success": false,
				"message": "Internal server error",
				"error":   err,
			})
		}
		c.AbortWithStatus(500)
	})
}

// TenantMiddleware validates tenant ID header
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// For now, just pass through - in production, you'd validate tenant access
		tenantIDVal, _ := c.Get("tenant_id")
		tenantID := ""
		if tenantIDVal != nil {
			tenantID = tenantIDVal.(string)
		}
		if tenantID != "" {
			c.Set("tenant_id", tenantID)
		}
		c.Next()
	}
}

// AuthMiddleware handles authentication (simplified for demo)
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// For development/demo purposes, accept user ID from context
		// In production, this would validate JWT tokens
		userIDVal, _ := c.Get("user_id")
		userID := ""
		if userIDVal != nil {
			userID = userIDVal.(string)
		}
		if userID != "" {
			c.Set("user_id", userID)
			c.Set("staff_id", userID) // RBAC middleware checks staff_id first
		}
		c.Next()
	}
}

// InternalServiceMiddleware allows requests from internal services
// Internal services identify themselves via X-Internal-Service header
// This is protected at network level by Kubernetes network policies and Istio mTLS
var allowedInternalServices = map[string]bool{
	"audit-service":        true,
	"notification-service": true,
	"products-service":     true,
	"orders-service":       true,
	"categories-service":   true,
}

func InternalServiceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceName := c.GetHeader("X-Internal-Service")
		if serviceName == "" {
			c.AbortWithStatusJSON(401, gin.H{
				"success": false,
				"error":   "Internal service authentication required",
			})
			return
		}

		if !allowedInternalServices[serviceName] {
			c.AbortWithStatusJSON(403, gin.H{
				"success": false,
				"error":   "Unauthorized service",
			})
			return
		}

		c.Set("internal_service", serviceName)
		c.Next()
	}
}
