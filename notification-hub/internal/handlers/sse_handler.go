package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"notification-hub/internal/models"
	"notification-hub/internal/repository"
)

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID       string
	TenantID string
	UserID   uuid.UUID
	Events   chan *SSEEvent
	Done     chan struct{}
}

// SSEEvent represents an SSE event to send
type SSEEvent struct {
	Event string
	Data  interface{}
}

// SSEHub manages SSE client connections
type SSEHub struct {
	// clients maps tenantID -> userID -> clientID -> SSEClient
	clients map[string]map[string]map[string]*SSEClient
	mu      sync.RWMutex
}

// NewSSEHub creates a new SSE hub
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[string]map[string]map[string]*SSEClient),
	}
}

// Register adds a client to the hub
func (h *SSEHub) Register(client *SSEClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	tenantID := client.TenantID
	userID := client.UserID.String()
	clientID := client.ID

	if h.clients[tenantID] == nil {
		h.clients[tenantID] = make(map[string]map[string]*SSEClient)
	}
	if h.clients[tenantID][userID] == nil {
		h.clients[tenantID][userID] = make(map[string]*SSEClient)
	}

	h.clients[tenantID][userID][clientID] = client
	log.Printf("SSE client registered: tenant=%s, user=%s, client=%s", tenantID, userID, clientID)
}

// Unregister removes a client from the hub
func (h *SSEHub) Unregister(client *SSEClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	tenantID := client.TenantID
	userID := client.UserID.String()
	clientID := client.ID

	if h.clients[tenantID] != nil && h.clients[tenantID][userID] != nil {
		if _, ok := h.clients[tenantID][userID][clientID]; ok {
			delete(h.clients[tenantID][userID], clientID)
			close(client.Events)
			log.Printf("SSE client unregistered: tenant=%s, user=%s, client=%s", tenantID, userID, clientID)

			// Cleanup empty maps
			if len(h.clients[tenantID][userID]) == 0 {
				delete(h.clients[tenantID], userID)
			}
			if len(h.clients[tenantID]) == 0 {
				delete(h.clients, tenantID)
			}
		}
	}
}

// BroadcastToUser sends a notification to all SSE clients of a user
func (h *SSEHub) BroadcastToUser(tenantID string, userID uuid.UUID, notification *models.Notification) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userIDStr := userID.String()
	if h.clients[tenantID] != nil && h.clients[tenantID][userIDStr] != nil {
		event := &SSEEvent{
			Event: "notification",
			Data:  notification,
		}

		for _, client := range h.clients[tenantID][userIDStr] {
			select {
			case client.Events <- event:
			default:
				// Buffer full, skip
				log.Printf("SSE client buffer full, skipping: client=%s", client.ID)
			}
		}
	}
}

// BroadcastUnreadCount sends unread count to all SSE clients of a user
func (h *SSEHub) BroadcastUnreadCount(tenantID string, userID uuid.UUID, count int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userIDStr := userID.String()
	if h.clients[tenantID] != nil && h.clients[tenantID][userIDStr] != nil {
		event := &SSEEvent{
			Event: "unread_count",
			Data:  map[string]int{"count": count},
		}

		for _, client := range h.clients[tenantID][userIDStr] {
			select {
			case client.Events <- event:
			default:
			}
		}
	}
}

// GetConnectedUserIDs returns all connected user IDs for a tenant
func (h *SSEHub) GetConnectedUserIDs(tenantID string) []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.clients[tenantID] == nil {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(h.clients[tenantID]))
	for userIDStr := range h.clients[tenantID] {
		if userID, err := uuid.Parse(userIDStr); err == nil {
			userIDs = append(userIDs, userID)
		}
	}
	return userIDs
}

// SSEHandler handles SSE connections
type SSEHandler struct {
	hub       *SSEHub
	notifRepo repository.NotificationRepository
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(hub *SSEHub, notifRepo repository.NotificationRepository) *SSEHandler {
	return &SSEHandler{
		hub:       hub,
		notifRepo: notifRepo,
	}
}

// Stream handles SSE stream connections
func (h *SSEHandler) Stream(c *gin.Context) {
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

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Create client
	client := &SSEClient{
		ID:       uuid.New().String(),
		TenantID: tenantID,
		UserID:   userID,
		Events:   make(chan *SSEEvent, 256),
		Done:     make(chan struct{}),
	}

	// Register client
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	// Send initial connection event
	h.sendEvent(c, "connected", map[string]interface{}{
		"client_id": client.ID,
		"message":   "Connected to notification stream",
	})

	// Send current unread count
	count, _ := h.notifRepo.GetUnreadCount(context.Background(), tenantID, userID)
	h.sendEvent(c, "unread_count", map[string]int{"count": int(count)})

	// Heartbeat ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Listen for events
	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			log.Printf("SSE client disconnected: %s", client.ID)
			return
		case event, ok := <-client.Events:
			if !ok {
				return
			}
			h.sendEvent(c, event.Event, event.Data)
		case <-ticker.C:
			h.sendEvent(c, "heartbeat", map[string]string{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
}

func (h *SSEHandler) sendEvent(c *gin.Context, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to marshal SSE event: %v", err)
		return
	}

	// SSE format: event: <event>\ndata: <data>\n\n
	fmt.Fprintf(c.Writer, "event: %s\n", event)
	fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
	c.Writer.Flush()
}
