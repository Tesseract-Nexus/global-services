package nats

import (
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// Client wraps the NATS connection
type Client struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

// NewClient creates a new NATS client with production-ready settings
// maxReconnects: use -1 for unlimited reconnects (recommended for production)
func NewClient(url string, maxReconnects int, reconnectWait time.Duration) (*Client, error) {
	// Override to unlimited if not explicitly set
	if maxReconnects == 0 {
		maxReconnects = -1 // Default to unlimited for production
	}

	opts := []nats.Option{
		nats.Name("notification-service"),
		nats.MaxReconnects(maxReconnects),
		nats.ReconnectWait(reconnectWait),
		nats.ReconnectBufSize(8 * 1024 * 1024), // 8MB buffer for messages during reconnect
		nats.Timeout(10 * time.Second),
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Printf("[NATS] Disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[NATS] Reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Println("[NATS] Connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("[NATS] Error: %v", err)
		}),
	}

	conn, err := nats.Connect(url, opts...)
	if err != nil {
		return nil, err
	}

	// Create JetStream context
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, err
	}

	log.Printf("Connected to NATS at %s", url)

	return &Client{
		conn: conn,
		js:   js,
	}, nil
}

// Connection returns the underlying NATS connection
func (c *Client) Connection() *nats.Conn {
	return c.conn
}

// JetStream returns the JetStream context
func (c *Client) JetStream() nats.JetStreamContext {
	return c.js
}

// Close closes the NATS connection
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Drain()
	}
}

// IsConnected checks if the client is connected
func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}
