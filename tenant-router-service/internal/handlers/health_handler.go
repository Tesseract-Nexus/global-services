package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tenant-router-service/internal/database"
	"tenant-router-service/internal/k8s"
	natsClient "tenant-router-service/internal/nats"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	k8sClient      *k8s.Client
	natsSubscriber *natsClient.Subscriber
	db             *gorm.DB
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(k8sClient *k8s.Client, natsSubscriber *natsClient.Subscriber, db *gorm.DB) *HealthHandler {
	return &HealthHandler{
		k8sClient:      k8sClient,
		natsSubscriber: natsSubscriber,
		db:             db,
	}
}

// Health handles the liveness check
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "tenant-router-service",
	})
}

// Ready handles the readiness check
func (h *HealthHandler) Ready(c *gin.Context) {
	ready := true
	checks := make(map[string]string)

	// Check database connectivity
	if h.db != nil && database.HealthCheck(h.db) == nil {
		checks["database"] = "connected"
	} else {
		checks["database"] = "disconnected"
		ready = false
	}

	// Check K8s connectivity
	if h.k8sClient != nil && h.k8sClient.IsConnected(c.Request.Context()) {
		checks["kubernetes"] = "connected"
	} else {
		checks["kubernetes"] = "disconnected"
		ready = false
	}

	// Check NATS connectivity
	if h.natsSubscriber != nil && h.natsSubscriber.IsConnected() {
		checks["nats"] = "connected"
	} else {
		checks["nats"] = "disconnected"
		ready = false
	}

	status := http.StatusOK
	statusText := "ready"
	if !ready {
		status = http.StatusServiceUnavailable
		statusText = "not_ready"
	}

	c.JSON(status, gin.H{
		"status":  statusText,
		"service": "tenant-router-service",
		"checks":  checks,
	})
}
