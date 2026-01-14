package handlers

import (
	"net/http"
	"time"

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

// Health godoc
// @Summary Health check
// @Description Get the health status of the location service
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Service is healthy",
		"timestamp": time.Now(),
		"data": gin.H{
			"status":  "healthy",
			"service": "location-service",
			"version": "1.0.0",
		},
	})
}

// Ready godoc
// @Summary Readiness check
// @Description Get the readiness status of the location service
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	// Check if database is configured
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success":   false,
			"message":   "Service is not ready",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "DATABASE_NOT_CONFIGURED",
				"details": "Database connection not initialized",
			},
		})
		return
	}

	// Check database connection
	sqlDB, err := h.db.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success":   false,
			"message":   "Service is not ready",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "DATABASE_CONNECTION_FAILED",
				"details": "Failed to get database connection",
			},
		})
		return
	}

	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success":   false,
			"message":   "Service is not ready",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "DATABASE_PING_FAILED",
				"details": "Database ping failed",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Service is ready",
		"timestamp": time.Now(),
		"data": gin.H{
			"status":   "ready",
			"service":  "location-service",
			"database": "connected",
		},
	})
}
