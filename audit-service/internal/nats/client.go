package nats

import (
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// Config holds NATS connection configuration
type Config struct {
	URL           string
	MaxReconnects int
	ReconnectWait time.Duration
}

// DefaultConfig returns a default NATS configuration with production-ready settings
func DefaultConfig() Config {
	return Config{
		URL:           "nats://nats.nats.svc.cluster.local:4222",
		MaxReconnects: -1, // Unlimited reconnects for production resilience
		ReconnectWait: 2 * time.Second,
	}
}

// Client wraps the NATS connection and JetStream context
type Client struct {
	conn   *nats.Conn
	js     nats.JetStreamContext
	config Config
	logger *logrus.Logger
}

// NewClient creates a new NATS client with production-ready settings
func NewClient(cfg Config, logger *logrus.Logger) (*Client, error) {
	// Use unlimited reconnects for production resilience
	maxReconnects := cfg.MaxReconnects
	if maxReconnects == 0 || maxReconnects > 0 {
		maxReconnects = -1 // Override to unlimited for production
	}

	// Connection options - production ready
	opts := []nats.Option{
		nats.Name("audit-service"),
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
		logger: logger,
	}

	// Ensure AUDIT_EVENTS stream exists
	if err := client.ensureStream(); err != nil {
		log.Printf("Warning: failed to ensure audit stream: %v", err)
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

// ensureStream creates the AUDIT_EVENTS stream if it doesn't exist
func (c *Client) ensureStream() error {
	streamCfg := nats.StreamConfig{
		Name:        "AUDIT_EVENTS",
		Description: "Audit log events for real-time streaming",
		Subjects:    []string{"audit.>"},
		Storage:     nats.FileStorage,
		Retention:   nats.LimitsPolicy,
		MaxAge:      24 * time.Hour, // Keep events for 24 hours
		MaxMsgs:     100000,
		Discard:     nats.DiscardOld,
		Replicas:    1,
	}

	_, err := c.js.StreamInfo(streamCfg.Name)
	if err == nats.ErrStreamNotFound {
		// Create stream
		_, err = c.js.AddStream(&streamCfg)
		if err != nil {
			return fmt.Errorf("failed to create stream: %w", err)
		}
		log.Printf("Created AUDIT_EVENTS stream")
	} else if err != nil {
		return fmt.Errorf("failed to check stream: %w", err)
	} else {
		log.Printf("AUDIT_EVENTS stream exists")
	}

	return nil
}
