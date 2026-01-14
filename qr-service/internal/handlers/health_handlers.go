package handlers

import (
	"net/http"
	"time"

	"qr-service/internal/models"

	"github.com/gin-gonic/gin"
)

const (
	ServiceName    = "qr-service"
	ServiceVersion = "1.0.0"
)

// HealthHandlers handles health check requests
type HealthHandlers struct{}

// NewHealthHandlers creates a new HealthHandlers instance
func NewHealthHandlers() *HealthHandlers {
	return &HealthHandlers{}
}

// Health godoc
// @Summary Health check
// @Description Returns the health status of the service
// @Tags Health
// @Produce json
// @Success 200 {object} models.HealthResponse "Service is healthy"
// @Router /health [get]
func (h *HealthHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{
		Status:    "healthy",
		Service:   ServiceName,
		Version:   ServiceVersion,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Ready godoc
// @Summary Readiness check
// @Description Returns the readiness status of the service
// @Tags Health
// @Produce json
// @Success 200 {object} models.HealthResponse "Service is ready"
// @Router /ready [get]
func (h *HealthHandlers) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{
		Status:    "ready",
		Service:   ServiceName,
		Version:   ServiceVersion,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
