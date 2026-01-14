package nats

import (
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"notification-hub/internal/config"
)

// Client wraps the NATS connection and JetStream context
type Client struct {
	conn   *nats.Conn
	js     nats.JetStreamContext
	config *config.NATSConfig
}

// NewClient creates a new NATS client with production-ready settings
func NewClient(cfg *config.NATSConfig) (*Client, error) {
	// Use unlimited reconnects for production resilience
	maxReconnects := cfg.MaxReconnects
	if maxReconnects == 0 || maxReconnects > 0 {
		maxReconnects = -1 // Override to unlimited for production
	}

	// Connection options - production ready
	opts := []nats.Option{
		nats.Name("notification-hub"),
		nats.Timeout(10 * time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(maxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.ReconnectBufSize(8 * 1024 * 1024), // 8MB buffer for messages during reconnect
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Printf("[NATS] Disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[NATS] Reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Printf("[NATS] Connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("[NATS] Error: %v", err)
		}),
	}

	// Connect to NATS
	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := conn.JetStream(nats.PublishAsyncMaxPending(256))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	client := &Client{
		conn:   conn,
		js:     js,
		config: cfg,
	}

	// Ensure streams exist
	if err := client.ensureStreams(); err != nil {
		log.Printf("Warning: failed to ensure streams: %v", err)
		// Don't fail - streams might be created by other services
	}

	log.Printf("Connected to NATS at %s", cfg.URL)
	return client, nil
}

// Close closes the NATS connection
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Drain()
		c.conn.Close()
	}
}

// JetStream returns the JetStream context
func (c *Client) JetStream() nats.JetStreamContext {
	return c.js
}

// Conn returns the underlying NATS connection
func (c *Client) Conn() *nats.Conn {
	return c.conn
}

// IsConnected returns true if connected to NATS
func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}

// ensureStreams creates streams if they don't exist
func (c *Client) ensureStreams() error {
	streams := []nats.StreamConfig{
		{
			Name:        "ORDER_EVENTS",
			Description: "Order lifecycle events",
			Subjects:    []string{"order.>"},
			Storage:     nats.FileStorage,
			Retention:   nats.LimitsPolicy,
			MaxAge:      24 * time.Hour * 7, // 7 days
			MaxMsgs:     100000,
			Discard:     nats.DiscardOld,
		},
		{
			Name:        "PAYMENT_EVENTS",
			Description: "Payment events",
			Subjects:    []string{"payment.>"},
			Storage:     nats.FileStorage,
			Retention:   nats.LimitsPolicy,
			MaxAge:      24 * time.Hour * 7,
			MaxMsgs:     100000,
			Discard:     nats.DiscardOld,
		},
		{
			Name:        "INVENTORY_EVENTS",
			Description: "Inventory events",
			Subjects:    []string{"inventory.>"},
			Storage:     nats.FileStorage,
			Retention:   nats.LimitsPolicy,
			MaxAge:      24 * time.Hour * 7,
			MaxMsgs:     100000,
			Discard:     nats.DiscardOld,
		},
		{
			Name:        "CUSTOMER_EVENTS",
			Description: "Customer events",
			Subjects:    []string{"customer.>"},
			Storage:     nats.FileStorage,
			Retention:   nats.LimitsPolicy,
			MaxAge:      24 * time.Hour * 7,
			MaxMsgs:     100000,
			Discard:     nats.DiscardOld,
		},
		{
			Name:        "RETURN_EVENTS",
			Description: "Return/refund events",
			Subjects:    []string{"return.>"},
			Storage:     nats.FileStorage,
			Retention:   nats.LimitsPolicy,
			MaxAge:      24 * time.Hour * 7,
			MaxMsgs:     100000,
			Discard:     nats.DiscardOld,
		},
		{
			Name:        "REVIEW_EVENTS",
			Description: "Product review events",
			Subjects:    []string{"review.>"},
			Storage:     nats.FileStorage,
			Retention:   nats.LimitsPolicy,
			MaxAge:      24 * time.Hour * 7,
			MaxMsgs:     100000,
			Discard:     nats.DiscardOld,
		},
	}

	for _, streamCfg := range streams {
		_, err := c.js.StreamInfo(streamCfg.Name)
		if err == nats.ErrStreamNotFound {
			// Create stream
			_, err = c.js.AddStream(&streamCfg)
			if err != nil {
				log.Printf("Failed to create stream %s: %v", streamCfg.Name, err)
			} else {
				log.Printf("Created stream: %s", streamCfg.Name)
			}
		} else if err != nil {
			log.Printf("Failed to check stream %s: %v", streamCfg.Name, err)
		} else {
			log.Printf("Stream exists: %s", streamCfg.Name)
		}
	}

	return nil
}
