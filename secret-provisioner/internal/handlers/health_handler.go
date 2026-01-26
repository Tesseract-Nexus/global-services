package handlers

import (
	"net/http"
	"time"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
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

// Health handles GET /health
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
	})
}

// Ready handles GET /ready
func (h *HealthHandler) Ready(c *gin.Context) {
	checks := make(map[string]string)
	status := "healthy"

	// Check database connection
	sqlDB, err := h.db.DB()
	if err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		status = "unhealthy"
	} else if err := sqlDB.Ping(); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		status = "unhealthy"
	} else {
		checks["database"] = "healthy"
	}

	statusCode := http.StatusOK
	if status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, models.HealthResponse{
		Status:    status,
		Checks:    checks,
		Timestamp: time.Now(),
	})
}
