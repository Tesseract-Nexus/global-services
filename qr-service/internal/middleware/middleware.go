package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Request-ID, X-Tenant-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		duration := time.Since(startTime)

		tenantIDVal, _ := c.Get("tenant_id")
		tenantID := ""
		if tenantIDVal != nil {
			tenantID = tenantIDVal.(string)
		}

		logrus.WithFields(logrus.Fields{
			"request_id": c.GetString("request_id"),
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
			"status":     c.Writer.Status(),
			"duration":   duration.String(),
			"client_ip":  c.ClientIP(),
			"user_agent": c.Request.UserAgent(),
			"tenant_id":  tenantID,
		}).Info("Request completed")
	}
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			logrus.WithFields(logrus.Fields{
				"request_id": c.GetString("request_id"),
				"error":      err.Error(),
				"path":       c.Request.URL.Path,
				"method":     c.Request.Method,
			}).Error("Request failed")
		}
	}
}

func RateLimiter(requestsPerSecond int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func TenantExtractor() gin.HandlerFunc {
	return func(c *gin.Context) {
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
