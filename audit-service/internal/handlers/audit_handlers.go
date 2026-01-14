package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"audit-service/internal/models"
	auditNats "audit-service/internal/nats"
	"audit-service/internal/services"
)

// AuditHandlers handles HTTP requests for audit logs
type AuditHandlers struct {
	service    *services.AuditService
	logger     *logrus.Logger
	subscriber *auditNats.Subscriber
}

// NewAuditHandlers creates a new audit handlers instance
func NewAuditHandlers(service *services.AuditService, logger *logrus.Logger, subscriber *auditNats.Subscriber) *AuditHandlers {
	return &AuditHandlers{
		service:    service,
		logger:     logger,
		subscriber: subscriber,
	}
}

// CreateAuditLog creates a new audit log entry
// POST /api/v1/audit-logs
func (h *AuditHandlers) CreateAuditLog(c *gin.Context) {
	var log models.AuditLog

	if err := c.ShouldBindJSON(&log); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Get tenant ID from context (set by middleware)
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	if err := h.service.LogAction(c.Request.Context(), tenantID, &log); err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to create audit log")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create audit log"})
		return
	}

	c.JSON(http.StatusCreated, log)
}

// GetAuditLog retrieves a single audit log by ID
// GET /api/v1/audit-logs/:id
func (h *AuditHandlers) GetAuditLog(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid audit log ID"})
		return
	}

	log, err := h.service.GetAuditLog(c.Request.Context(), tenantID, id)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"id":        id,
			"tenant_id": tenantID,
		}).Error("Failed to get audit log")
		c.JSON(http.StatusNotFound, gin.H{"error": "Audit log not found"})
		return
	}

	c.JSON(http.StatusOK, log)
}

// ListAuditLogs lists audit logs with filtering and pagination
// GET /api/v1/audit-logs
func (h *AuditHandlers) ListAuditLogs(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	filter := &models.AuditLogFilter{
		TenantID:    tenantID,
		Action:      models.AuditAction(c.Query("action")),
		Resource:    models.AuditResource(c.Query("resource")),
		ResourceID:  c.Query("resource_id"),
		Status:      models.AuditStatus(c.Query("status")),
		Severity:    models.AuditSeverity(c.Query("severity")),
		IPAddress:   c.Query("ip_address"),
		ServiceName: c.Query("service_name"),
		SearchText:  c.Query("search"),
		SortBy:      c.DefaultQuery("sort_by", "timestamp"),
		SortOrder:   c.DefaultQuery("sort_order", "DESC"),
	}

	// Parse user ID
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		userID, err := uuid.Parse(userIDStr)
		if err == nil {
			filter.UserID = &userID
		}
	}

	// Parse date range
	if fromDateStr := c.Query("from_date"); fromDateStr != "" {
		fromDate, err := time.Parse(time.RFC3339, fromDateStr)
		if err == nil {
			filter.FromDate = &fromDate
		}
	}
	if toDateStr := c.Query("to_date"); toDateStr != "" {
		toDate, err := time.Parse(time.RFC3339, toDateStr)
		if err == nil {
			filter.ToDate = &toDate
		}
	}

	// Parse pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter.Limit = limit
	filter.Offset = offset

	logs, total, err := h.service.SearchAuditLogs(c.Request.Context(), tenantID, filter)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to list audit logs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetResourceHistory retrieves audit history for a specific resource
// GET /api/v1/audit-logs/resource/:resource_type/:resource_id
func (h *AuditHandlers) GetResourceHistory(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	resourceType := c.Param("resource_type")
	resourceID := c.Param("resource_id")

	logs, err := h.service.GetResourceHistory(c.Request.Context(), tenantID, resourceType, resourceID)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"tenant_id":     tenantID,
		}).Error("Failed to get resource history")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get resource history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"resource_type": resourceType,
		"resource_id":   resourceID,
		"history":       logs,
		"count":         len(logs),
	})
}

// GetUserActivity retrieves audit activity for a specific user
// GET /api/v1/audit-logs/user/:user_id
func (h *AuditHandlers) GetUserActivity(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	userID := c.Param("user_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	logs, err := h.service.GetUserActivity(c.Request.Context(), tenantID, userID, limit)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"user_id":   userID,
			"tenant_id": tenantID,
		}).Error("Failed to get user activity")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user activity"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"activity": logs,
		"count":    len(logs),
	})
}

// GetCriticalEvents retrieves recent critical events
// GET /api/v1/audit-logs/critical
func (h *AuditHandlers) GetCriticalEvents(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))

	logs, err := h.service.GetRecentCriticalEvents(c.Request.Context(), tenantID, hours)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get critical events")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get critical events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": logs,
		"count":  len(logs),
		"hours":  hours,
	})
}

// GetFailedAuthAttempts retrieves failed authentication attempts
// GET /api/v1/audit-logs/failed-auth
func (h *AuditHandlers) GetFailedAuthAttempts(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))

	logs, err := h.service.GetFailedAuthAttempts(c.Request.Context(), tenantID, hours)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get failed auth attempts")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get failed auth attempts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"attempts": logs,
		"count":    len(logs),
		"hours":    hours,
	})
}

// GetSummary retrieves audit log summary statistics
// GET /api/v1/audit-logs/summary
func (h *AuditHandlers) GetSummary(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	// Parse date range (default to last 30 days)
	toDate := time.Now()
	fromDate := toDate.AddDate(0, 0, -30)

	if fromDateStr := c.Query("from_date"); fromDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromDateStr); err == nil {
			fromDate = parsed
		}
	}
	if toDateStr := c.Query("to_date"); toDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toDateStr); err == nil {
			toDate = parsed
		}
	}

	summary, err := h.service.GetSummary(c.Request.Context(), tenantID, fromDate, toDate)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get audit summary")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get audit summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// ExportAuditLogs exports audit logs in JSON or CSV format
// GET /api/v1/audit-logs/export
func (h *AuditHandlers) ExportAuditLogs(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	format := c.DefaultQuery("format", "json")

	filter := &models.AuditLogFilter{
		TenantID: tenantID,
		Action:   models.AuditAction(c.Query("action")),
		Resource: models.AuditResource(c.Query("resource")),
		Status:   models.AuditStatus(c.Query("status")),
		Severity: models.AuditSeverity(c.Query("severity")),
	}

	// Parse date range
	if fromDateStr := c.Query("from_date"); fromDateStr != "" {
		fromDate, err := time.Parse(time.RFC3339, fromDateStr)
		if err == nil {
			filter.FromDate = &fromDate
		}
	}
	if toDateStr := c.Query("to_date"); toDateStr != "" {
		toDate, err := time.Parse(time.RFC3339, toDateStr)
		if err == nil {
			filter.ToDate = &toDate
		}
	}

	var data []byte
	var contentType string
	var filename string
	var err error

	switch format {
	case "csv":
		data, err = h.service.ExportToCSV(c.Request.Context(), tenantID, filter)
		contentType = "text/csv"
		filename = fmt.Sprintf("audit-logs-%s.csv", time.Now().Format("2006-01-02"))
	case "json":
		data, err = h.service.ExportToJSON(c.Request.Context(), tenantID, filter)
		contentType = "application/json"
		filename = fmt.Sprintf("audit-logs-%s.json", time.Now().Format("2006-01-02"))
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Use 'json' or 'csv'"})
		return
	}

	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to export audit logs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export audit logs"})
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, contentType, data)
}

// GetSuspiciousActivity detects suspicious activity patterns
// GET /api/v1/audit-logs/suspicious-activity
func (h *AuditHandlers) GetSuspiciousActivity(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	logs, err := h.service.GetSuspiciousActivity(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to detect suspicious activity")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to detect suspicious activity"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"suspicious_events": logs,
		"count":             len(logs),
	})
}

// GetUserIPHistory retrieves all IP addresses used by a user
// GET /api/v1/audit-logs/user/:user_id/ip-history
func (h *AuditHandlers) GetUserIPHistory(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	userID := c.Param("user_id")

	entries, err := h.service.GetUserIPHistory(c.Request.Context(), tenantID, userID)
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"user_id":   userID,
			"tenant_id": tenantID,
		}).Error("Failed to get user IP history")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user IP history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":    userID,
		"ip_history": entries,
		"count":      len(entries),
	})
}

// GetRecentLogs retrieves recent logs for real-time updates
// GET /api/v1/audit-logs/recent
func (h *AuditHandlers) GetRecentLogs(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	logs, err := h.service.GetRecentLogs(c.Request.Context(), tenantID, limit)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get recent logs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get recent logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"count": len(logs),
	})
}

// StreamAuditLogs streams audit logs via Server-Sent Events
// GET /api/v1/audit-logs/stream
func (h *AuditHandlers) StreamAuditLogs(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	// Create a channel to signal client disconnect
	clientGone := c.Request.Context().Done()

	// Initial fetch - send recent logs on connect
	logs, err := h.service.GetRecentLogs(c.Request.Context(), tenantID, 20)
	if err == nil && len(logs) > 0 {
		data, _ := json.Marshal(gin.H{"type": "initial", "logs": logs})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		c.Writer.Flush()
	}

	// Use NATS subscription for real-time updates if available
	if h.subscriber != nil {
		eventChan, cleanup, err := h.subscriber.SubscribeToTenant(c.Request.Context(), tenantID)
		if err != nil {
			h.logger.WithError(err).WithField("tenant_id", tenantID).Warn("Failed to subscribe to NATS, falling back to polling")
			h.streamWithPolling(c, tenantID, clientGone)
			return
		}
		defer cleanup()

		h.logger.WithField("tenant_id", tenantID).Info("SSE client connected with NATS subscription")

		// Send connected event to confirm NATS subscription
		connectedData, _ := json.Marshal(gin.H{"type": "connected", "nats": true, "message": "Connected to real-time updates via NATS"})
		fmt.Fprintf(c.Writer, "data: %s\n\n", connectedData)
		c.Writer.Flush()

		// Heartbeat ticker
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-clientGone:
				h.logger.WithField("tenant_id", tenantID).Debug("SSE client disconnected")
				return
			case event, ok := <-eventChan:
				if !ok {
					h.logger.WithField("tenant_id", tenantID).Debug("NATS event channel closed")
					return
				}
				// Forward NATS event to SSE client
				if event.Log != nil {
					data, _ := json.Marshal(gin.H{"type": "update", "logs": []*models.AuditLog{event.Log}})
					fmt.Fprintf(c.Writer, "data: %s\n\n", data)
					c.Writer.Flush()
				}
			case <-heartbeat.C:
				// Send heartbeat to keep connection alive
				fmt.Fprintf(c.Writer, ": heartbeat\n\n")
				c.Writer.Flush()
			}
		}
	} else {
		// Fallback to polling if NATS is not available
		h.streamWithPolling(c, tenantID, clientGone)
	}
}

// GetRetentionSettings retrieves retention settings for the tenant
// GET /api/v1/audit-logs/retention
func (h *AuditHandlers) GetRetentionSettings(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	settings, err := h.service.GetRetentionSettings(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to get retention settings")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get retention settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"settings": settings,
		"options":  h.service.GetRetentionOptions(),
	})
}

// SetRetentionSettings updates retention settings for the tenant
// PUT /api/v1/audit-logs/retention
func (h *AuditHandlers) SetRetentionSettings(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	var request struct {
		RetentionDays int `json:"retentionDays" binding:"required,min=90,max=365"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
			"hint":    "Retention days must be between 90 (3 months) and 365 (1 year)",
		})
		return
	}

	settings, err := h.service.SetRetentionSettings(c.Request.Context(), tenantID, request.RetentionDays)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to set retention settings")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set retention settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Retention settings updated successfully",
		"settings": settings,
	})
}

// TriggerCleanup manually triggers cleanup for the tenant (admin only)
// POST /api/v1/audit-logs/cleanup
func (h *AuditHandlers) TriggerCleanup(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant ID is required"})
		return
	}

	deleted, err := h.service.CleanupOldLogs(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", tenantID).Error("Failed to trigger cleanup")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trigger cleanup"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Cleanup completed successfully",
		"logs_deleted": deleted,
	})
}

// streamWithPolling is the fallback polling-based implementation
func (h *AuditHandlers) streamWithPolling(c *gin.Context, tenantID string, clientGone <-chan struct{}) {
	h.logger.WithField("tenant_id", tenantID).Info("SSE client connected with polling fallback")

	// Send connected event (polling mode)
	connectedData, _ := json.Marshal(gin.H{"type": "connected", "nats": false, "message": "Connected to real-time updates via polling"})
	fmt.Fprintf(c.Writer, "data: %s\n\n", connectedData)
	c.Writer.Flush()

	var lastTimestamp time.Time

	// Get initial timestamp from recent logs
	logs, err := h.service.GetRecentLogs(c.Request.Context(), tenantID, 1)
	if err == nil && len(logs) > 0 {
		lastTimestamp = logs[0].Timestamp
	}

	// Poll every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-clientGone:
			h.logger.WithField("tenant_id", tenantID).Debug("SSE client disconnected")
			return
		case <-ticker.C:
			logs, err := h.service.GetRecentLogs(c.Request.Context(), tenantID, 50)
			if err != nil {
				continue
			}

			var newLogs []models.AuditLog
			for _, log := range logs {
				if log.Timestamp.After(lastTimestamp) {
					newLogs = append(newLogs, log)
				}
			}

			if len(newLogs) > 0 {
				lastTimestamp = newLogs[0].Timestamp
				data, _ := json.Marshal(gin.H{"type": "update", "logs": newLogs})
				fmt.Fprintf(c.Writer, "data: %s\n\n", data)
				c.Writer.Flush()
			} else {
				fmt.Fprintf(c.Writer, ": heartbeat\n\n")
				c.Writer.Flush()
			}
		}
	}
}
