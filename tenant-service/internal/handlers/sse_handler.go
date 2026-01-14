package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// SessionEventData represents the data sent when a session event occurs
type SessionEventData struct {
	SessionID string `json:"session_id"`
	Event     string `json:"event"`
	Email     string `json:"email,omitempty"`
	Verified  bool   `json:"verified,omitempty"`
	Timestamp string `json:"timestamp"`
}

// SSEHub manages SSE connections for session events
type SSEHub struct {
	// Map of sessionID -> list of client channels
	clients map[string]map[string]chan SSEEvent
	mu      sync.RWMutex
}

// Global SSE hub instance
var sseHub = NewSSEHub()

// NewSSEHub creates a new SSE hub
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[string]map[string]chan SSEEvent),
	}
}

// GetSSEHub returns the global SSE hub instance
func GetSSEHub() *SSEHub {
	return sseHub
}

// Subscribe adds a client to receive events for a specific session
func (h *SSEHub) Subscribe(sessionID, clientID string) chan SSEEvent {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[sessionID] == nil {
		h.clients[sessionID] = make(map[string]chan SSEEvent)
	}

	ch := make(chan SSEEvent, 10) // Buffered channel
	h.clients[sessionID][clientID] = ch

	log.Printf("[SSE] Client %s subscribed to session %s (total clients for session: %d)",
		clientID, sessionID, len(h.clients[sessionID]))

	return ch
}

// Unsubscribe removes a client from receiving events
func (h *SSEHub) Unsubscribe(sessionID, clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if sessionClients, ok := h.clients[sessionID]; ok {
		if ch, ok := sessionClients[clientID]; ok {
			close(ch)
			delete(sessionClients, clientID)
			log.Printf("[SSE] Client %s unsubscribed from session %s", clientID, sessionID)
		}

		// Clean up empty session map
		if len(sessionClients) == 0 {
			delete(h.clients, sessionID)
		}
	}
}

// Broadcast sends an event to all clients subscribed to a session
func (h *SSEHub) Broadcast(sessionID string, event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if sessionClients, ok := h.clients[sessionID]; ok {
		log.Printf("[SSE] Broadcasting event '%s' to %d clients for session %s",
			event.Event, len(sessionClients), sessionID)

		for clientID, ch := range sessionClients {
			select {
			case ch <- event:
				log.Printf("[SSE] Sent event to client %s", clientID)
			default:
				log.Printf("[SSE] Client %s channel full, skipping", clientID)
			}
		}
	} else {
		log.Printf("[SSE] No clients subscribed to session %s", sessionID)
	}
}

// BroadcastSessionVerified broadcasts a verification completed event
func (h *SSEHub) BroadcastSessionVerified(sessionID, email string) {
	event := SSEEvent{
		Event: "session.verified",
		Data: SessionEventData{
			SessionID: sessionID,
			Event:     "session.verified",
			Email:     email,
			Verified:  true,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
	h.Broadcast(sessionID, event)
}

// BroadcastSessionCompleted broadcasts a session completed event
func (h *SSEHub) BroadcastSessionCompleted(sessionID, email string) {
	event := SSEEvent{
		Event: "session.completed",
		Data: SessionEventData{
			SessionID: sessionID,
			Event:     "session.completed",
			Email:     email,
			Verified:  true,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
	h.Broadcast(sessionID, event)
}

// SSEHandler handles SSE connections for session events
type SSEHandler struct {
	hub *SSEHub
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler() *SSEHandler {
	return &SSEHandler{
		hub: GetSSEHub(),
	}
}

// StreamSessionEvents handles SSE connections for a specific session
// GET /api/v1/onboarding/sessions/:sessionId/events
func (h *SSEHandler) StreamSessionEvents(c *gin.Context) {
	sessionIDStr := c.Param("sessionId")
	if sessionIDStr == "" {
		c.JSON(400, gin.H{"error": "session_id is required"})
		return
	}

	// Validate session ID format
	if _, err := uuid.Parse(sessionIDStr); err != nil {
		c.JSON(400, gin.H{"error": "invalid session_id format"})
		return
	}

	// Generate unique client ID
	clientID := uuid.New().String()

	// Set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Subscribe to session events
	eventChan := h.hub.Subscribe(sessionIDStr, clientID)
	defer h.hub.Unsubscribe(sessionIDStr, clientID)

	// Send initial connection event
	initialEvent := SSEEvent{
		Event: "connected",
		Data: map[string]string{
			"session_id": sessionIDStr,
			"client_id":  clientID,
			"message":    "Connected to session events",
		},
	}
	sendSSEEvent(c, initialEvent)

	// Create a ticker for keepalive pings
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Listen for events or client disconnect
	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			log.Printf("[SSE] Client %s disconnected from session %s", clientID, sessionIDStr)
			return

		case event, ok := <-eventChan:
			if !ok {
				// Channel closed
				return
			}
			sendSSEEvent(c, event)

		case <-ticker.C:
			// Send keepalive ping
			pingEvent := SSEEvent{
				Event: "ping",
				Data: map[string]string{
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				},
			}
			sendSSEEvent(c, pingEvent)
		}
	}
}

// sendSSEEvent sends a single SSE event to the client
func sendSSEEvent(c *gin.Context, event SSEEvent) {
	data, err := json.Marshal(event.Data)
	if err != nil {
		log.Printf("[SSE] Failed to marshal event data: %v", err)
		return
	}

	// Format: event: <event-name>\ndata: <json-data>\n\n
	fmt.Fprintf(c.Writer, "event: %s\n", event.Event)
	fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
	c.Writer.Flush()
}
