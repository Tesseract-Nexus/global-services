package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck returns the health status of the service
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "tickets-service",
		"version":   "1.0.0",
		"timestamp": c.Request.Header.Get("X-Request-Time"),
	})
}
