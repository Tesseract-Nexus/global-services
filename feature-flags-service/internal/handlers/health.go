package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"feature-flags-service/internal/clients"
)

// HealthCheck handles health check requests
// @Summary Health check
// @Description Check if the service is running
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]interface{} "Service is healthy"
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "feature-flags-service",
	})
}

// ReadyCheck handles readiness check requests
// @Summary Readiness check
// @Description Check if the service is ready to accept requests (Growthbook connection verified)
// @Tags Health
// @Produce json
// @Success 200 {object} map[string]interface{} "Service is ready"
// @Failure 503 {object} map[string]interface{} "Service is not ready"
// @Router /ready [get]
func ReadyCheck(client *clients.GrowthbookClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := client.Health(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "not ready",
				"service": "feature-flags-service",
				"error":   "Growthbook is not available",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "ready",
			"service": "feature-flags-service",
		})
	}
}
