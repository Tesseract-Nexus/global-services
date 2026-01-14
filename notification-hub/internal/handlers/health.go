package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	natsc "github.com/tesseract-nexus/tesseract-hub/services/notification-hub/internal/nats"
	"gorm.io/gorm"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db         *gorm.DB
	natsClient *natsc.Client
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *gorm.DB, natsClient *natsc.Client) *HealthHandler {
	return &HealthHandler{
		db:         db,
		natsClient: natsClient,
	}
}

// SetNATSClient updates the NATS client (used for deferred connection)
func (h *HealthHandler) SetNATSClient(client *natsc.Client) {
	h.natsClient = client
}

// Health returns basic health status
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "notification-hub",
	})
}

// Livez returns liveness status
func (h *HealthHandler) Livez(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// Readyz returns readiness status
func (h *HealthHandler) Readyz(c *gin.Context) {
	status := "ready"
	httpStatus := http.StatusOK

	checks := make(map[string]string)

	// Check database
	sqlDB, err := h.db.DB()
	if err != nil {
		checks["database"] = "error: " + err.Error()
		status = "not ready"
		httpStatus = http.StatusServiceUnavailable
	} else if err := sqlDB.Ping(); err != nil {
		checks["database"] = "error: " + err.Error()
		status = "not ready"
		httpStatus = http.StatusServiceUnavailable
	} else {
		checks["database"] = "connected"
	}

	// Check NATS (optional - service can work without real-time notifications)
	if h.natsClient != nil && h.natsClient.IsConnected() {
		checks["nats"] = "connected"
	} else {
		checks["nats"] = "disconnected (optional)"
		// Don't mark service as not ready - REST API still works without NATS
	}

	c.JSON(httpStatus, gin.H{
		"status": status,
		"checks": checks,
	})
}
