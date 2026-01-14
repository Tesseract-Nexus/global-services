package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tesseract-nexus/tesseract-hub/services/notification-hub/internal/config"
)

// Client represents a single WebSocket connection
type Client struct {
	ID       string
	TenantID string
	UserID   uuid.UUID
	Hub      *Hub
	Conn     *websocket.Conn
	send     chan []byte
	config   *config.WebSocketConfig

	// Handler for incoming messages
	OnMarkRead    func(notificationIDs []string)
	OnMarkAllRead func()
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, tenantID string, userID uuid.UUID, cfg *config.WebSocketConfig) *Client {
	return &Client{
		ID:       uuid.New().String(),
		TenantID: tenantID,
		UserID:   userID,
		Hub:      hub,
		Conn:     conn,
		send:     make(chan []byte, 256),
		config:   cfg,
	}
}

// SendMessage sends a message to the client
func (c *Client) SendMessage(msg *OutgoingMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	select {
	case c.send <- data:
	default:
		// Buffer full, skip message
		log.Printf("Client send buffer full, skipping message: client=%s", c.ID)
	}
}

// ReadPump reads messages from the WebSocket connection
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(c.config.MaxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// WritePump writes messages to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.Conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if !ok {
				// Hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(c.config.WriteWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(message []byte) {
	var msg IncomingMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		c.SendMessage(&OutgoingMessage{
			Type: MessageTypeError,
			Data: ErrorData{
				Code:    "INVALID_JSON",
				Message: "Failed to parse message",
			},
		})
		return
	}

	switch msg.Type {
	case "ping":
		c.SendMessage(&OutgoingMessage{
			Type: MessageTypePong,
			Data: map[string]interface{}{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		})

	case "mark_read":
		var data MarkReadData
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			c.sendError("INVALID_DATA", "Failed to parse mark_read data")
			return
		}
		if c.OnMarkRead != nil {
			c.OnMarkRead(data.NotificationIDs)
		}

	case "mark_all_read":
		if c.OnMarkAllRead != nil {
			c.OnMarkAllRead()
		}

	case "subscribe":
		// Handle subscription to specific categories (optional feature)
		// For now, all notifications are sent to the user
		log.Printf("Client %s subscribed with data: %s", c.ID, string(msg.Data))

	default:
		c.sendError("UNKNOWN_TYPE", "Unknown message type: "+msg.Type)
	}
}

func (c *Client) sendError(code, message string) {
	c.SendMessage(&OutgoingMessage{
		Type: MessageTypeError,
		Data: ErrorData{
			Code:    code,
			Message: message,
		},
	})
}
