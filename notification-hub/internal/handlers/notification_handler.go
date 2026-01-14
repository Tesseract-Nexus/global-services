package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tesseract-nexus/tesseract-hub/services/notification-hub/internal/models"
	"github.com/tesseract-nexus/tesseract-hub/services/notification-hub/internal/repository"
	"github.com/tesseract-nexus/tesseract-hub/services/notification-hub/internal/websocket"
)

// NotificationHandler handles notification-related HTTP requests
type NotificationHandler struct {
	notifRepo repository.NotificationRepository
	hub       *websocket.Hub
	sseHub    *SSEHub
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(
	notifRepo repository.NotificationRepository,
	hub *websocket.Hub,
	sseHub *SSEHub,
) *NotificationHandler {
	return &NotificationHandler{
		notifRepo: notifRepo,
		hub:       hub,
		sseHub:    sseHub,
	}
}

// List returns a paginated list of notifications
func (h *NotificationHandler) List(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	// Parse filters
	filters := repository.NotificationFilters{
		IsRead:   parseBoolPtr(c.Query("is_read")),
		Type:     c.Query("type"),
		Priority: c.Query("priority"),
		GroupKey: c.Query("group_key"),
		Limit:    parseIntWithDefault(c.Query("limit"), 50),
		Offset:   parseIntWithDefault(c.Query("offset"), 0),
	}

	// Validate limit
	if filters.Limit > 100 {
		filters.Limit = 100
	}

	notifications, total, err := h.notifRepo.List(c.Request.Context(), tenantID, userID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list notifications"})
		return
	}

	// Get unread count for the user
	unreadCount, _ := h.notifRepo.GetUnreadCount(c.Request.Context(), tenantID, userID)

	c.JSON(http.StatusOK, models.NotificationListResponse{
		Success: true,
		Data:    notifications,
		Pagination: &models.Pagination{
			Limit:  filters.Limit,
			Offset: filters.Offset,
			Total:  total,
		},
		UnreadCount: unreadCount,
	})
}

// Get returns a single notification by ID
func (h *NotificationHandler) Get(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	notificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	notification, err := h.notifRepo.GetByID(c.Request.Context(), tenantID, userID, notificationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notification"})
		return
	}
	if notification == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    notification,
	})
}

// MarkRead marks a notification as read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	notificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	if err := h.notifRepo.MarkAsRead(c.Request.Context(), tenantID, userID, []uuid.UUID{notificationID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}

	// Get updated unread count
	count, _ := h.notifRepo.GetUnreadCount(c.Request.Context(), tenantID, userID)

	// Broadcast to connected clients
	h.hub.BroadcastReadStatus(tenantID, userID, []string{notificationID.String()}, true)
	h.hub.BroadcastUnreadCount(tenantID, userID, int(count))
	h.sseHub.BroadcastUnreadCount(tenantID, userID, int(count))

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Notification marked as read",
		"unread_count": count,
	})
}

// MarkUnread marks a notification as unread
func (h *NotificationHandler) MarkUnread(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	notificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	if err := h.notifRepo.MarkAsUnread(c.Request.Context(), tenantID, userID, notificationID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as unread"})
		return
	}

	// Get updated unread count
	count, _ := h.notifRepo.GetUnreadCount(c.Request.Context(), tenantID, userID)

	// Broadcast to connected clients
	h.hub.BroadcastReadStatus(tenantID, userID, []string{notificationID.String()}, false)
	h.hub.BroadcastUnreadCount(tenantID, userID, int(count))
	h.sseHub.BroadcastUnreadCount(tenantID, userID, int(count))

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Notification marked as unread",
		"unread_count": count,
	})
}

// MarkBatchRead marks multiple notifications as read
func (h *NotificationHandler) MarkBatchRead(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	var req struct {
		NotificationIDs []string `json:"notification_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Parse UUIDs
	ids := make([]uuid.UUID, 0, len(req.NotificationIDs))
	for _, idStr := range req.NotificationIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid notification IDs provided"})
		return
	}

	if err := h.notifRepo.MarkAsRead(c.Request.Context(), tenantID, userID, ids); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notifications as read"})
		return
	}

	// Get updated unread count
	count, _ := h.notifRepo.GetUnreadCount(c.Request.Context(), tenantID, userID)

	// Broadcast to connected clients
	h.hub.BroadcastReadStatus(tenantID, userID, req.NotificationIDs, true)
	h.hub.BroadcastUnreadCount(tenantID, userID, int(count))
	h.sseHub.BroadcastUnreadCount(tenantID, userID, int(count))

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Notifications marked as read",
		"count":        len(ids),
		"unread_count": count,
	})
}

// MarkAllRead marks all notifications as read for a user
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	affected, err := h.notifRepo.MarkAllAsRead(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark all notifications as read"})
		return
	}

	// Broadcast to connected clients
	h.hub.BroadcastUnreadCount(tenantID, userID, 0)
	h.sseHub.BroadcastUnreadCount(tenantID, userID, 0)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "All notifications marked as read",
		"count":        affected,
		"unread_count": 0,
	})
}

// GetUnreadCount returns the unread notification count
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	count, err := h.notifRepo.GetUnreadCount(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get unread count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   count,
	})
}

// Delete deletes a notification
func (h *NotificationHandler) Delete(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	notificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	if err := h.notifRepo.Delete(c.Request.Context(), tenantID, userID, notificationID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete notification"})
		return
	}

	// Get updated unread count
	count, _ := h.notifRepo.GetUnreadCount(c.Request.Context(), tenantID, userID)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Notification deleted",
		"unread_count": count,
	})
}

// DeleteAll deletes all notifications for a user
func (h *NotificationHandler) DeleteAll(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	affected, err := h.notifRepo.DeleteAll(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete all notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "All notifications deleted",
		"count":        affected,
		"unread_count": 0,
	})
}

// Helper functions
func parseBoolPtr(s string) *bool {
	if s == "" {
		return nil
	}
	b := s == "true" || s == "1"
	return &b
}

func parseIntWithDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}
