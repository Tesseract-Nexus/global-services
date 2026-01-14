package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// LoggerMiddleware logs HTTP requests
func LoggerMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		endTime := time.Now()
		latency := endTime.Sub(startTime)

		entry := logger.WithFields(logrus.Fields{
			"status":     c.Writer.Status(),
			"method":     c.Request.Method,
			"path":       path,
			"query":      query,
			"ip":         c.ClientIP(),
			"latency":    latency.String(),
			"user-agent": c.Request.UserAgent(),
			"tenant_id":  c.GetString("tenant_id"),
		})

		if c.Writer.Status() >= 500 {
			entry.Error("Server error")
		} else if c.Writer.Status() >= 400 {
			entry.Warn("Client error")
		} else {
			entry.Info("Request completed")
		}
	}
}

// RecoveryMiddleware handles panics
func RecoveryMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.WithField("error", err).Error("Panic recovered")
				c.AbortWithStatusJSON(500, gin.H{
					"success": false,
					"error":   "Internal server error",
				})
			}
		}()
		c.Next()
	}
}
