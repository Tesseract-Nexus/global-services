package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	db *gorm.DB
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Health returns basic health status
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "notification-service",
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

	c.JSON(httpStatus, gin.H{
		"status": status,
		"checks": checks,
	})
}
