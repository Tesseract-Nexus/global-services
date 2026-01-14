package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tesseract-nexus/tesseract-hub/services/notification-hub/internal/models"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	MessageTypeNotification       MessageType = "notification"
	MessageTypeNotificationsBatch MessageType = "notifications_batch"
	MessageTypeReadStatusUpdated  MessageType = "read_status_updated"
	MessageTypeUnreadCount        MessageType = "unread_count"
	MessageTypePong               MessageType = "pong"
	MessageTypeError              MessageType = "error"
	MessageTypeConnected          MessageType = "connected"
)

// OutgoingMessage represents a message sent to clients
type OutgoingMessage struct {
	Type MessageType `json:"type"`
	Data interface{} `json:"data"`
}

// IncomingMessage represents a message received from clients
type IncomingMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// MarkReadData represents the data for mark_read message
type MarkReadData struct {
	NotificationIDs []string `json:"notification_ids"`
}

// ConnectedData represents the data sent on connection
type ConnectedData struct {
	ClientID string `json:"client_id"`
	Message  string `json:"message"`
}

// UnreadCountData represents the unread count data
type UnreadCountData struct {
	Count int `json:"count"`
}

// ErrorData represents error message data
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Hub manages all WebSocket client connections
type Hub struct {
	// clients maps tenantID -> userID -> clientID -> Client
	clients map[string]map[string]map[string]*Client

	// Channels for client management
	register   chan *Client
	unregister chan *Client

	// Mutex for thread-safe access
	mu sync.RWMutex

	// Shutdown channel
	shutdown chan struct{}
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[string]map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		shutdown:   make(chan struct{}),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case <-h.shutdown:
			h.closeAllClients()
			return
		}
	}
}

// Shutdown gracefully shuts down the hub
func (h *Hub) Shutdown() {
	close(h.shutdown)
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	tenantID := client.TenantID
	userID := client.UserID.String()
	clientID := client.ID

	// Initialize nested maps if needed
	if h.clients[tenantID] == nil {
		h.clients[tenantID] = make(map[string]map[string]*Client)
	}
	if h.clients[tenantID][userID] == nil {
		h.clients[tenantID][userID] = make(map[string]*Client)
	}

	h.clients[tenantID][userID][clientID] = client
	log.Printf("Client registered: tenant=%s, user=%s, client=%s", tenantID, userID, clientID)

	// Send connected message
	client.SendMessage(&OutgoingMessage{
		Type: MessageTypeConnected,
		Data: ConnectedData{
			ClientID: clientID,
			Message:  "Connected to notification stream",
		},
	})
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	tenantID := client.TenantID
	userID := client.UserID.String()
	clientID := client.ID

	if h.clients[tenantID] != nil && h.clients[tenantID][userID] != nil {
		if _, ok := h.clients[tenantID][userID][clientID]; ok {
			delete(h.clients[tenantID][userID], clientID)
			close(client.send)
			log.Printf("Client unregistered: tenant=%s, user=%s, client=%s", tenantID, userID, clientID)

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

func (h *Hub) closeAllClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, users := range h.clients {
		for _, clients := range users {
			for _, client := range clients {
				close(client.send)
			}
		}
	}
	h.clients = make(map[string]map[string]map[string]*Client)
}

// BroadcastToUser sends a notification to all connected clients of a specific user
func (h *Hub) BroadcastToUser(tenantID string, userID uuid.UUID, notification *models.Notification) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userIDStr := userID.String()
	if h.clients[tenantID] != nil && h.clients[tenantID][userIDStr] != nil {
		message := &OutgoingMessage{
			Type: MessageTypeNotification,
			Data: notification,
		}

		for _, client := range h.clients[tenantID][userIDStr] {
			client.SendMessage(message)
		}
	}
}

// BroadcastUnreadCount sends an unread count update to all connected clients of a user
func (h *Hub) BroadcastUnreadCount(tenantID string, userID uuid.UUID, count int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userIDStr := userID.String()
	if h.clients[tenantID] != nil && h.clients[tenantID][userIDStr] != nil {
		message := &OutgoingMessage{
			Type: MessageTypeUnreadCount,
			Data: UnreadCountData{Count: count},
		}

		for _, client := range h.clients[tenantID][userIDStr] {
			client.SendMessage(message)
		}
	}
}

// BroadcastReadStatus sends read status updates to all connected clients of a user
func (h *Hub) BroadcastReadStatus(tenantID string, userID uuid.UUID, notificationIDs []string, isRead bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userIDStr := userID.String()
	if h.clients[tenantID] != nil && h.clients[tenantID][userIDStr] != nil {
		message := &OutgoingMessage{
			Type: MessageTypeReadStatusUpdated,
			Data: map[string]interface{}{
				"notification_ids": notificationIDs,
				"is_read":          isRead,
			},
		}

		for _, client := range h.clients[tenantID][userIDStr] {
			client.SendMessage(message)
		}
	}
}

// GetConnectedUserCount returns the number of connected users for a tenant
func (h *Hub) GetConnectedUserCount(tenantID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.clients[tenantID] == nil {
		return 0
	}
	return len(h.clients[tenantID])
}

// IsUserConnected checks if a user has any connected clients
func (h *Hub) IsUserConnected(tenantID string, userID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userIDStr := userID.String()
	if h.clients[tenantID] == nil || h.clients[tenantID][userIDStr] == nil {
		return false
	}
	return len(h.clients[tenantID][userIDStr]) > 0
}

// PingAllClients sends a ping to all connected clients
func (h *Hub) PingAllClients() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	message := &OutgoingMessage{
		Type: MessageTypePong,
		Data: map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	}

	for _, users := range h.clients {
		for _, clients := range users {
			for _, client := range clients {
				client.SendMessage(message)
			}
		}
	}
}

// GetConnectedUserIDs returns all connected user IDs for a tenant
func (h *Hub) GetConnectedUserIDs(tenantID string) []uuid.UUID {
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
