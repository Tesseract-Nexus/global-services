package handlers

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"notification-hub/internal/config"
	"notification-hub/internal/repository"
	ws "notification-hub/internal/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, validate origin
		return true
	},
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub       *ws.Hub
	notifRepo repository.NotificationRepository
	config    *config.WebSocketConfig
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(
	hub *ws.Hub,
	notifRepo repository.NotificationRepository,
	cfg *config.WebSocketConfig,
) *WebSocketHandler {
	return &WebSocketHandler{
		hub:       hub,
		notifRepo: notifRepo,
		config:    cfg,
	}
}

// Handle upgrades HTTP connection to WebSocket
func (h *WebSocketHandler) Handle(c *gin.Context) {
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

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket: %v", err)
		return
	}

	// Create client
	client := ws.NewClient(h.hub, conn, tenantID, userID, h.config)

	// Set up message handlers
	client.OnMarkRead = func(notificationIDs []string) {
		ids := make([]uuid.UUID, 0, len(notificationIDs))
		for _, idStr := range notificationIDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}

		if len(ids) > 0 {
			if err := h.notifRepo.MarkAsRead(context.Background(), tenantID, userID, ids); err != nil {
				log.Printf("Failed to mark notifications as read: %v", err)
				return
			}

			// Get updated unread count
			count, _ := h.notifRepo.GetUnreadCount(context.Background(), tenantID, userID)
			h.hub.BroadcastReadStatus(tenantID, userID, notificationIDs, true)
			h.hub.BroadcastUnreadCount(tenantID, userID, int(count))
		}
	}

	client.OnMarkAllRead = func() {
		if _, err := h.notifRepo.MarkAllAsRead(context.Background(), tenantID, userID); err != nil {
			log.Printf("Failed to mark all notifications as read: %v", err)
			return
		}
		h.hub.BroadcastUnreadCount(tenantID, userID, 0)
	}

	// Register client with hub
	h.hub.Register(client)

	// Send initial unread count
	count, _ := h.notifRepo.GetUnreadCount(context.Background(), tenantID, userID)
	h.hub.BroadcastUnreadCount(tenantID, userID, int(count))

	// Start read and write pumps
	go client.WritePump()
	go client.ReadPump()
}

// GetStatus returns WebSocket connection status for a user
func (h *WebSocketHandler) GetStatus(c *gin.Context) {
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

	connected := h.hub.IsUserConnected(tenantID, userID)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"connected": connected,
	})
}
