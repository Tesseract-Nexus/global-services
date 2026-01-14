package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"status-dashboard-service/internal/models"
	"status-dashboard-service/internal/services"
)

// Handler contains all HTTP handlers
type Handler struct {
	healthChecker *services.HealthChecker
}

// NewHandler creates a new handler
func NewHandler(hc *services.HealthChecker) *Handler {
	return &Handler{
		healthChecker: hc,
	}
}

// Health returns the health status of this service
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetStatus returns the overall platform status
func (h *Handler) GetStatus(c *gin.Context) {
	// Get service summaries
	summaries := h.healthChecker.GetServiceSummaries()

	// Get overall stats
	stats := h.healthChecker.GetOverallStats()

	// Get active incidents
	incidents := h.healthChecker.GetActiveIncidents()

	// Determine overall status
	overallStatus := "operational"
	if stats.UnhealthyServices > 0 {
		overallStatus = "outage"
	} else if stats.DegradedServices > 0 {
		overallStatus = "degraded"
	}

	response := models.StatusResponse{
		Status:      overallStatus,
		Services:    summaries,
		Incidents:   incidents,
		Stats:       *stats,
		LastUpdated: time.Now(),
	}

	c.JSON(http.StatusOK, response)
}

// GetServices returns all monitored services
func (h *Handler) GetServices(c *gin.Context) {
	summaries := h.healthChecker.GetServiceSummaries()

	c.JSON(http.StatusOK, gin.H{
		"services": summaries,
		"total":    len(summaries),
	})
}

// GetService returns a specific service with details
func (h *Handler) GetService(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service ID"})
		return
	}

	service := h.healthChecker.GetServiceByID(id)
	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
		return
	}

	// Calculate uptime from in-memory stats
	uptime := 100.0
	if service.TotalChecks > 0 {
		uptime = float64(service.SuccessCount) / float64(service.TotalChecks) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"service":   service,
		"uptime":    uptime,
		"slaMet":    uptime >= service.SLATarget,
	})
}

// GetIncidents returns active incidents
func (h *Handler) GetIncidents(c *gin.Context) {
	incidents := h.healthChecker.GetActiveIncidents()

	c.JSON(http.StatusOK, gin.H{
		"incidents": incidents,
		"total":     len(incidents),
	})
}

// SSEStream handles Server-Sent Events for real-time updates
func (h *Handler) SSEStream(c *gin.Context) {
	// Generate unique subscriber ID
	subID := uuid.New().String()

	// Subscribe to health check updates
	ch := h.healthChecker.Subscribe(subID)
	defer h.healthChecker.Unsubscribe(subID)

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Send initial data
	stats := h.healthChecker.GetOverallStats()
	c.SSEvent("status", stats)
	c.Writer.Flush()

	// Stream updates
	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case check, ok := <-ch:
			if !ok {
				return
			}
			// Get service info
			service := h.healthChecker.GetServiceByID(check.ServiceID)
			data := gin.H{
				"check":   check,
				"service": service,
			}
			c.SSEvent("health_check", data)
			c.Writer.Flush()
		}
	}
}

// Dashboard serves the HTML dashboard page
func (h *Handler) Dashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Tesseract Status Dashboard",
	})
}
