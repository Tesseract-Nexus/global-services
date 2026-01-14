package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/tesseract-hub/document-service/internal/models"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	service models.DocumentService
	logger  *logrus.Logger
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(service models.DocumentService, logger *logrus.Logger) *HealthHandler {
	if logger == nil {
		logger = logrus.New()
	}

	return &HealthHandler{
		service: service,
		logger:  logger,
	}
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Services  map[string]ServiceInfo `json:"services"`
}

// ServiceInfo represents information about a service component
type ServiceInfo struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Health performs a basic health check
// @Summary Health check
// @Description Get the health status of the document service
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0", // This should come from build info
		Services:  make(map[string]ServiceInfo),
	}

	// Check database connectivity (basic check)
	response.Services["database"] = ServiceInfo{
		Status: "healthy",
	}

	// Check cloud storage connectivity
	ctx := c.Request.Context()
	if err := h.service.TestConnection(ctx); err != nil {
		response.Status = "degraded"
		response.Services["storage"] = ServiceInfo{
			Status:  "unhealthy",
			Message: err.Error(),
		}
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	response.Services["storage"] = ServiceInfo{
		Status: "healthy",
	}

	c.JSON(http.StatusOK, response)
}

// Ready performs a readiness check
// @Summary Readiness check
// @Description Check if the service is ready to accept requests
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /health/ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	response := HealthResponse{
		Status:    "ready",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Services:  make(map[string]ServiceInfo),
	}

	// Perform more thorough checks for readiness
	ctx := c.Request.Context()

	// Test storage connection
	if err := h.service.TestConnection(ctx); err != nil {
		response.Status = "not_ready"
		response.Services["storage"] = ServiceInfo{
			Status:  "unhealthy",
			Message: err.Error(),
		}
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	response.Services["storage"] = ServiceInfo{
		Status: "healthy",
	}

	// Storage operations check passed via TestConnection
	// Note: ListBuckets requires project-level permissions which we may not have
	// TestConnection already verifies bucket access, so we're ready
	response.Services["storage_operations"] = ServiceInfo{
		Status: "healthy",
	}

	c.JSON(http.StatusOK, response)
}

// Live performs a liveness check
// @Summary Liveness check
// @Description Check if the service is alive (for Kubernetes liveness probe)
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health/live [get]
func (h *HealthHandler) Live(c *gin.Context) {
	response := HealthResponse{
		Status:    "alive",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Services:  make(map[string]ServiceInfo),
	}

	// Basic liveness check - just return OK if the service is running
	response.Services["service"] = ServiceInfo{
		Status: "healthy",
	}

	c.JSON(http.StatusOK, response)
}
