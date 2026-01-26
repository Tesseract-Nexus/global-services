package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	// Context keys
	KeyTenantID        = "tenant_id"
	KeyActorID         = "actor_id"
	KeyRequestID       = "request_id"
	KeyInternalService = "internal_service"
)

// RequestID middleware adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set(KeyRequestID, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// TenantID middleware extracts tenant ID from headers
func TenantID() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetHeader("X-Tenant-ID")
		if tenantID != "" {
			c.Set(KeyTenantID, tenantID)
		}
		c.Next()
	}
}

// VerifyInternalService middleware verifies the calling service is allowed
func VerifyInternalService(allowedServices []string) gin.HandlerFunc {
	allowed := make(map[string]bool)
	for _, s := range allowedServices {
		allowed[s] = true
	}

	return func(c *gin.Context) {
		service := c.GetHeader("X-Internal-Service")
		if service == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "UNAUTHORIZED",
				"message": "X-Internal-Service header is required",
			})
			return
		}

		if !allowed[service] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "FORBIDDEN",
				"message": "Service not authorized to access this API",
			})
			return
		}

		c.Set(KeyInternalService, service)

		// Also extract actor ID if present
		if actorID := c.GetHeader("X-Actor-ID"); actorID != "" {
			c.Set(KeyActorID, actorID)
		}

		c.Next()
	}
}

// RequestLogger middleware logs request information
func RequestLogger(logger *logrus.Entry) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		entry := logger.WithFields(logrus.Fields{
			"status":     statusCode,
			"method":     c.Request.Method,
			"path":       path,
			"latency":    latency,
			"request_id": GetRequestID(c),
		})

		if tenantID := GetTenantID(c); tenantID != "" {
			entry = entry.WithField("tenant_id", tenantID)
		}

		if service := GetInternalService(c); service != "" {
			entry = entry.WithField("service", service)
		}

		if statusCode >= 500 {
			entry.Error("request completed with error")
		} else if statusCode >= 400 {
			entry.Warn("request completed with client error")
		} else {
			entry.Info("request completed")
		}
	}
}

// Helper functions to get context values

// GetTenantID retrieves the tenant ID from context
func GetTenantID(c *gin.Context) string {
	if val, exists := c.Get(KeyTenantID); exists {
		return val.(string)
	}
	return ""
}

// GetActorID retrieves the actor ID from context
func GetActorID(c *gin.Context) string {
	if val, exists := c.Get(KeyActorID); exists {
		return val.(string)
	}
	return ""
}

// GetRequestID retrieves the request ID from context
func GetRequestID(c *gin.Context) string {
	if val, exists := c.Get(KeyRequestID); exists {
		return val.(string)
	}
	return ""
}

// GetInternalService retrieves the internal service name from context
func GetInternalService(c *gin.Context) string {
	if val, exists := c.Get(KeyInternalService); exists {
		return val.(string)
	}
	return ""
}
