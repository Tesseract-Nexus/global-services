package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	natsClient "tenant-service/internal/nats"
	"gorm.io/gorm"
)

var startTime = time.Now()

// HealthHandler handles health check endpoints
type HealthHandler struct {
	db         *gorm.DB
	natsClient *natsClient.Client
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{
		db: db,
	}
}

// NewHealthHandlerWithNATS creates a new health handler with NATS client
func NewHealthHandlerWithNATS(db *gorm.DB, nc *natsClient.Client) *HealthHandler {
	return &HealthHandler{
		db:         db,
		natsClient: nc,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string           `json:"status"`
	Service   string           `json:"service"`
	Version   string           `json:"version"`
	Uptime    string           `json:"uptime"`
	Timestamp string           `json:"timestamp"`
	Checks    map[string]Check `json:"checks,omitempty"`
	System    *SystemInfo      `json:"system,omitempty"`
}

// Check represents a health check result
type Check struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// SystemInfo represents system runtime information
type SystemInfo struct {
	Goroutines  int    `json:"goroutines"`
	MemoryAlloc uint64 `json:"memory_alloc_mb"`
	MemoryTotal uint64 `json:"memory_total_mb"`
	MemorySys   uint64 `json:"memory_sys_mb"`
	NumCPU      int    `json:"num_cpu"`
	GoVersion   string `json:"go_version"`
}

// Health godoc
// @Summary Health check
// @Description Get the health status of the service with detailed information
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	response := HealthResponse{
		Status:    "healthy",
		Service:   "tenant-service",
		Version:   "1.0.0",
		Uptime:    time.Since(startTime).String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Include detailed checks if requested
	if c.Query("detailed") == "true" {
		response.Checks = h.performHealthChecks()
		response.System = h.getSystemInfo()
	}

	c.JSON(http.StatusOK, response)
}

// performHealthChecks runs all health checks
func (h *HealthHandler) performHealthChecks() map[string]Check {
	checks := make(map[string]Check)

	// Database check
	dbCheck := h.checkDatabase()
	checks["database"] = dbCheck

	// NATS check
	natsCheck := h.checkNATS()
	checks["nats"] = natsCheck

	return checks
}

// checkNATS checks NATS connectivity
func (h *HealthHandler) checkNATS() Check {
	if h.natsClient == nil {
		return Check{
			Status:  "unhealthy",
			Message: "NATS client not initialized",
		}
	}

	if !h.natsClient.IsConnected() {
		return Check{
			Status:  "unhealthy",
			Message: "NATS disconnected",
		}
	}

	return Check{
		Status:  "healthy",
		Message: "NATS connected",
	}
}

// checkDatabase checks database connectivity and stats
func (h *HealthHandler) checkDatabase() Check {
	sqlDB, err := h.db.DB()
	if err != nil {
		return Check{
			Status:  "unhealthy",
			Message: "Failed to get database instance",
		}
	}

	// Ping database
	if err := sqlDB.Ping(); err != nil {
		return Check{
			Status:  "unhealthy",
			Message: "Database ping failed",
		}
	}

	// Get connection pool stats
	stats := sqlDB.Stats()

	return Check{
		Status:  "healthy",
		Message: "Database connected",
		Details: map[string]interface{}{
			"open_connections": stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
			"max_open":         stats.MaxOpenConnections,
			"wait_count":       stats.WaitCount,
			"wait_duration_ms": stats.WaitDuration.Milliseconds(),
		},
	}
}

// getSystemInfo returns system runtime information
func (h *HealthHandler) getSystemInfo() *SystemInfo {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return &SystemInfo{
		Goroutines:  runtime.NumGoroutine(),
		MemoryAlloc: mem.Alloc / 1024 / 1024,      // MB
		MemoryTotal: mem.TotalAlloc / 1024 / 1024, // MB
		MemorySys:   mem.Sys / 1024 / 1024,        // MB
		NumCPU:      runtime.NumCPU(),
		GoVersion:   runtime.Version(),
	}
}

// Ready godoc
// @Summary Readiness check
// @Description Get the readiness status of the service and dependencies
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	response := HealthResponse{
		Service:   "tenant-service",
		Version:   "1.0.0",
		Uptime:    time.Since(startTime).String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    make(map[string]Check),
	}

	// Perform all readiness checks
	allReady := true

	// Database check
	dbCheck := h.checkDatabase()
	response.Checks["database"] = dbCheck
	if dbCheck.Status != "healthy" {
		allReady = false
	}

	// NATS check (optional - service can work without NATS but VS won't be created)
	natsCheck := h.checkNATS()
	response.Checks["nats"] = natsCheck
	// NATS is critical for VS creation, so mark as not ready if disconnected
	if natsCheck.Status != "healthy" {
		allReady = false
	}

	// Set overall status
	if allReady {
		response.Status = "ready"
		c.JSON(http.StatusOK, response)
	} else {
		response.Status = "not ready"
		c.JSON(http.StatusServiceUnavailable, response)
	}
}
