package health

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

// HealthChecker manages health check state and metrics
type HealthChecker struct {
	db        *gorm.DB
	ready     atomic.Bool
	startTime time.Time
	version   string
}

// Prometheus metrics
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "settings_service_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "settings_service_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	dbConnectionStatus = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "settings_service_db_connection_status",
		Help: "Database connection status (1 = connected, 0 = disconnected)",
	})

	serviceInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "settings_service_info",
			Help: "Service information",
		},
		[]string{"version"},
	)

	storefrontThemeOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "settings_service_storefront_theme_operations_total",
			Help: "Total number of storefront theme operations",
		},
		[]string{"operation", "status"},
	)

	settingsOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "settings_service_settings_operations_total",
			Help: "Total number of settings operations",
		},
		[]string{"operation", "status"},
	)
)

// NewHealthChecker creates a new health checker instance
func NewHealthChecker(db *gorm.DB, version string) *HealthChecker {
	hc := &HealthChecker{
		db:        db,
		startTime: time.Now(),
		version:   version,
	}

	// Set service info metric
	serviceInfo.WithLabelValues(version).Set(1)

	return hc
}

// SetReady marks the service as ready to receive traffic
func (h *HealthChecker) SetReady(ready bool) {
	h.ready.Store(ready)
}

// IsReady returns whether the service is ready
func (h *HealthChecker) IsReady() bool {
	return h.ready.Load()
}

// CheckDatabase verifies database connectivity
func (h *HealthChecker) CheckDatabase() error {
	sqlDB, err := h.db.DB()
	if err != nil {
		dbConnectionStatus.Set(0)
		return err
	}

	if err := sqlDB.Ping(); err != nil {
		dbConnectionStatus.Set(0)
		return err
	}

	dbConnectionStatus.Set(1)
	return nil
}

// LivezHandler handles liveness probe requests
// Returns 200 if the process is running (simple check)
func (h *HealthChecker) LivezHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// ReadyzHandler handles readiness probe requests
// Returns 200 only if the service can handle traffic (DB connected)
func (h *HealthChecker) ReadyzHandler(c *gin.Context) {
	// Check if marked as ready
	if !h.IsReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"reason": "service not initialized",
		})
		return
	}

	// Check database connectivity
	if err := h.CheckDatabase(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"reason": "database unavailable",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// HealthHandler handles general health check requests (backwards compatible)
func (h *HealthChecker) HealthHandler(c *gin.Context) {
	uptime := time.Since(h.startTime)

	dbStatus := "connected"
	if err := h.CheckDatabase(); err != nil {
		dbStatus = "disconnected"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "settings-service",
		"version": h.version,
		"uptime":  uptime.String(),
		"database": gin.H{
			"status": dbStatus,
		},
	})
}

// MetricsHandler returns Prometheus metrics handler
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// MetricsMiddleware records HTTP request metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := c.Writer.Status()
		statusStr := http.StatusText(status)
		if statusStr == "" {
			statusStr = "unknown"
		}

		// Skip metrics for health/metrics endpoints to avoid noise
		if path != "/livez" && path != "/readyz" && path != "/metrics" && path != "/health" {
			httpRequestsTotal.WithLabelValues(c.Request.Method, path, statusStr).Inc()
			httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
		}
	}
}

// RecordStorefrontThemeOperation records a storefront theme operation metric
func RecordStorefrontThemeOperation(operation string, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	storefrontThemeOperations.WithLabelValues(operation, status).Inc()
}

// RecordSettingsOperation records a settings operation metric
func RecordSettingsOperation(operation string, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	settingsOperations.WithLabelValues(operation, status).Inc()
}
