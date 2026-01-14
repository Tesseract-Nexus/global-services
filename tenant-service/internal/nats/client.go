package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

// Event types
const (
	EventTenantCreated               = "tenant.created"
	EventTenantUpdated               = "tenant.updated"
	EventTenantDeleted               = "tenant.deleted"
	EventTenantVerified              = "tenant.verified"
	EventSessionCompleted            = "tenant.session_completed"
	EventTenantVerificationRequested = "tenant.verification.requested"
	EventTenantOnboardingCompleted   = "tenant.onboarding.completed"
)

// TenantCreatedEvent is published when a new tenant is created
type TenantCreatedEvent struct {
	EventType    string    `json:"event_type"`
	TenantID     string    `json:"tenant_id"`
	SessionID    string    `json:"session_id"`
	Product      string    `json:"product"`
	BusinessName string    `json:"business_name"`
	Slug         string    `json:"slug"`
	Email        string    `json:"email"`
	// Host URLs for routing configuration
	AdminHost      string `json:"admin_host"`      // e.g., "mystore-admin.tesserix.app"
	StorefrontHost string `json:"storefront_host"` // e.g., "mystore.tesserix.app"
	BaseDomain     string `json:"base_domain"`     // e.g., "tesserix.app"
	Timestamp      time.Time `json:"timestamp"`
}

// TenantDeletedEvent is published when a tenant is deleted
type TenantDeletedEvent struct {
	EventType      string    `json:"event_type"`
	TenantID       string    `json:"tenant_id"`
	Slug           string    `json:"slug"`
	AdminHost      string    `json:"admin_host"`
	StorefrontHost string    `json:"storefront_host"`
	Timestamp      time.Time `json:"timestamp"`
}

// SessionCompletedEvent is published when an onboarding session is completed (after email verification)
// This triggers document migration from onboarding storage to tenant storage
type SessionCompletedEvent struct {
	EventType    string    `json:"event_type"`
	SessionID    string    `json:"session_id"`
	Product      string    `json:"product"`
	BusinessName string    `json:"business_name"`
	Email        string    `json:"email"`
	Timestamp    time.Time `json:"timestamp"`
}

// TenantVerificationRequestedEvent is published when email verification is needed for onboarding
// This triggers notification-service to send a verification link email
type TenantVerificationRequestedEvent struct {
	EventType    string `json:"event_type"`
	TenantID     string `json:"tenant_id"`
	SessionID    string `json:"session_id"`
	Product      string `json:"product"`
	BusinessName string `json:"business_name"`
	Slug         string `json:"slug"`
	Email        string `json:"email"`
	// Host URLs for context
	AdminHost      string `json:"admin_host"`
	StorefrontHost string `json:"storefront_host"`
	BaseDomain     string `json:"base_domain"`
	// Verification specific fields
	VerificationToken  string    `json:"verification_token"`
	VerificationLink   string    `json:"verification_link"`
	VerificationExpiry string    `json:"verification_expiry"` // RFC3339 format
	Timestamp          time.Time `json:"timestamp"`
}

// Client wraps the NATS connection
type Client struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

// Config holds NATS connection configuration
type Config struct {
	URL string
}

// DefaultConfig returns the default NATS configuration
func DefaultConfig() *Config {
	url := os.Getenv("NATS_URL")
	if url == "" {
		url = "nats://nats.nats.svc.cluster.local:4222"
	}
	return &Config{
		URL: url,
	}
}

// NewClient creates a new NATS client
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	log.Printf("[NATS] Connecting to %s", cfg.URL)

	// Connect with retry options - production-ready settings
	opts := []nats.Option{
		nats.Name("tenant-service"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),                   // Unlimited reconnects for production resilience
		nats.ReconnectWait(2 * time.Second),
		nats.ReconnectBufSize(8 * 1024 * 1024),   // 8MB buffer for messages during reconnect
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("[NATS] Disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("[NATS] Reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("[NATS] Error: %v", err)
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Printf("[NATS] Connection closed")
		}),
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context for persistent messaging
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	// Ensure the tenant events stream exists
	// Using LimitsPolicy to allow multiple consumers (tenant-router-service, notification-service, etc.)
	_, err = js.AddStream(&nats.StreamConfig{
		Name:        "TENANT_EVENTS",
		Description: "Stream for tenant lifecycle events",
		Subjects:    []string{"tenant.>"},
		Storage:     nats.FileStorage,
		Retention:   nats.LimitsPolicy, // Allow multiple consumers
		MaxAge:      24 * time.Hour * 7, // Keep messages for 7 days
		MaxMsgs:     100000,
		Discard:     nats.DiscardOld,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		log.Printf("[NATS] Warning: Could not create stream (may already exist): %v", err)
	}

	log.Printf("[NATS] Connected successfully to %s", cfg.URL)

	return &Client{
		conn: conn,
		js:   js,
	}, nil
}

// PublishTenantCreated publishes a tenant created event with retry logic
func (c *Client) PublishTenantCreated(ctx context.Context, event *TenantCreatedEvent) error {
	if c == nil || c.js == nil {
		return fmt.Errorf("NATS client not initialized")
	}

	event.EventType = EventTenantCreated
	event.Timestamp = time.Now().UTC()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish with JetStream for guaranteed delivery with retry
	var ack *nats.PubAck
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ack, err = c.js.Publish(EventTenantCreated, data)
		if err == nil {
			break
		}
		log.Printf("[NATS] Attempt %d/%d: Failed to publish %s event: %v", attempt, maxRetries, EventTenantCreated, err)
		if attempt < maxRetries {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while retrying publish: %w", ctx.Err())
			case <-time.After(backoff):
				continue
			}
		}
	}
	if err != nil {
		return fmt.Errorf("failed to publish event after %d attempts: %w", maxRetries, err)
	}

	log.Printf("[NATS] Published %s event for tenant %s (seq: %d)", EventTenantCreated, event.TenantID, ack.Sequence)
	return nil
}

// PublishSessionCompleted publishes a session completed event (triggers document migration)
func (c *Client) PublishSessionCompleted(ctx context.Context, event *SessionCompletedEvent) error {
	if c == nil || c.js == nil {
		log.Printf("[NATS] Client not initialized, skipping publish")
		return nil
	}

	event.EventType = EventSessionCompleted
	event.Timestamp = time.Now().UTC()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish with JetStream for guaranteed delivery
	ack, err := c.js.Publish(EventSessionCompleted, data)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("[NATS] Published %s event for session %s (seq: %d)", EventSessionCompleted, event.SessionID, ack.Sequence)
	return nil
}

// PublishTenantDeleted publishes a tenant deleted event with retry logic
func (c *Client) PublishTenantDeleted(ctx context.Context, event *TenantDeletedEvent) error {
	if c == nil || c.js == nil {
		return fmt.Errorf("NATS client not initialized")
	}

	event.EventType = EventTenantDeleted
	event.Timestamp = time.Now().UTC()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish with JetStream for guaranteed delivery with retry
	var ack *nats.PubAck
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ack, err = c.js.Publish(EventTenantDeleted, data)
		if err == nil {
			break
		}
		log.Printf("[NATS] Attempt %d/%d: Failed to publish %s event: %v", attempt, maxRetries, EventTenantDeleted, err)
		if attempt < maxRetries {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while retrying publish: %w", ctx.Err())
			case <-time.After(backoff):
				continue
			}
		}
	}
	if err != nil {
		return fmt.Errorf("failed to publish event after %d attempts: %w", maxRetries, err)
	}

	log.Printf("[NATS] Published %s event for tenant %s (seq: %d)", EventTenantDeleted, event.TenantID, ack.Sequence)
	return nil
}

// PublishTenantVerificationRequested publishes a verification requested event
// This triggers notification-service to send verification email
func (c *Client) PublishTenantVerificationRequested(ctx context.Context, event *TenantVerificationRequestedEvent) error {
	if c == nil || c.js == nil {
		log.Printf("[NATS] Client not initialized, skipping publish")
		return nil
	}

	event.EventType = EventTenantVerificationRequested
	event.Timestamp = time.Now().UTC()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish with JetStream for guaranteed delivery
	ack, err := c.js.Publish(EventTenantVerificationRequested, data)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf("[NATS] Published %s event for session %s (seq: %d)", EventTenantVerificationRequested, event.SessionID, ack.Sequence)
	return nil
}

// Close closes the NATS connection
func (c *Client) Close() {
	if c != nil && c.conn != nil {
		c.conn.Close()
		log.Printf("[NATS] Connection closed")
	}
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
	return c != nil && c.conn != nil && c.conn.IsConnected()
}

// SessionCompletedHandler is a callback for session completed events
type SessionCompletedHandler func(event *SessionCompletedEvent)

// SubscribeSessionCompleted subscribes to session completed events
// This is used by the SSE hub to broadcast verification events to connected clients
func (c *Client) SubscribeSessionCompleted(handler SessionCompletedHandler) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("NATS client not initialized")
	}

	// Subscribe to the session completed subject
	_, err := c.conn.Subscribe(EventSessionCompleted, func(msg *nats.Msg) {
		var event SessionCompletedEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			log.Printf("[NATS] Failed to unmarshal session completed event: %v", err)
			return
		}

		log.Printf("[NATS] Received session.completed event for session %s", event.SessionID)
		handler(&event)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to session completed events: %w", err)
	}

	log.Printf("[NATS] Subscribed to %s events for SSE broadcasting", EventSessionCompleted)
	return nil
}
