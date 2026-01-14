package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"

	"tenant-router-service/internal/config"
	"tenant-router-service/internal/models"
	"tenant-router-service/internal/reconciler"
)

// Event subjects
const (
	SubjectTenantCreated = "tenant.created"
	SubjectTenantDeleted = "tenant.deleted"
	StreamName           = "TENANT_EVENTS"
)

// Subscriber handles NATS JetStream subscriptions
type Subscriber struct {
	conn       *nats.Conn
	js         nats.JetStreamContext
	reconciler *reconciler.TenantReconciler
	config     *config.Config
	subs       []*nats.Subscription
}

// NewSubscriber creates a new NATS subscriber
func NewSubscriber(cfg *config.Config, reconciler *reconciler.TenantReconciler) (*Subscriber, error) {
	log.Printf("[NATS] Connecting to %s", cfg.NATS.URL)

	// Connect with retry options
	opts := []nats.Option{
		nats.Name("tenant-router-service"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1), // Unlimited reconnects
		nats.ReconnectWait(2 * time.Second),
		nats.ReconnectBufSize(8 * 1024 * 1024), // 8MB buffer
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

	conn, err := nats.Connect(cfg.NATS.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context with longer timeout for operations
	js, err := conn.JetStream(
		nats.PublishAsyncMaxPending(256),
		nats.MaxWait(30*time.Second), // Longer timeout for JetStream operations
	)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	log.Printf("[NATS] Connected successfully to %s", cfg.NATS.URL)

	// Ensure the tenant events stream exists before subscribing
	// This makes the service resilient and not dependent on tenant-service starting first
	// Use retries for cluster initialization scenarios
	var streamErr error
	for i := 0; i < 3; i++ {
		_, streamErr = js.AddStream(&nats.StreamConfig{
			Name:        StreamName,
			Description: "Stream for tenant lifecycle events",
			Subjects:    []string{"tenant.>"},
			Storage:     nats.FileStorage,
			Retention:   nats.LimitsPolicy, // Allow multiple consumers
			MaxAge:      24 * time.Hour * 7, // Keep messages for 7 days
			MaxMsgs:     100000,
			Discard:     nats.DiscardOld,
		})
		if streamErr == nil {
			log.Printf("[NATS] Created stream %s", StreamName)
			break
		}
		if streamErr == nats.ErrStreamNameAlreadyInUse {
			log.Printf("[NATS] Stream %s already exists", StreamName)
			streamErr = nil
			break
		}
		log.Printf("[NATS] Attempt %d: Could not create stream: %v, retrying...", i+1, streamErr)
		time.Sleep(5 * time.Second)
	}
	if streamErr != nil {
		log.Printf("[NATS] Warning: Failed to create stream after retries: %v (will try subscribing anyway)", streamErr)
	}

	return &Subscriber{
		conn:       conn,
		js:         js,
		reconciler: reconciler,
		config:     cfg,
		subs:       make([]*nats.Subscription, 0),
	}, nil
}

// Start begins subscribing to tenant events
func (s *Subscriber) Start(ctx context.Context) error {
	log.Printf("[NATS] Starting subscriptions...")

	// Check and delete any potentially stale consumer before creating a new subscription
	// This ensures we always get fresh message delivery
	consumerName := "tenant-router-consumer"
	if _, err := s.js.ConsumerInfo(StreamName, consumerName); err == nil {
		// Consumer exists - delete it to ensure fresh state
		log.Printf("[NATS] Found existing consumer %s, deleting to ensure fresh state...", consumerName)
		if err := s.js.DeleteConsumer(StreamName, consumerName); err != nil {
			log.Printf("[NATS] Warning: Failed to delete existing consumer: %v", err)
		} else {
			log.Printf("[NATS] Deleted existing consumer %s", consumerName)
		}
	}

	// Subscribe using QueueSubscribe pattern for horizontal scaling
	// This allows multiple pods to share the workload during rolling restarts
	// The queue group "tenant-router-workers" distributes messages across all subscribers
	// Use retries for subscription in case of cluster initialization
	//
	// IMPORTANT: We use DeliverNew() to only process new messages going forward.
	// Historical messages should have been processed, and we have HTTP fallback
	// for any missed messages. This avoids reprocessing all historical events.
	var sub *nats.Subscription
	var err error
	for i := 0; i < 3; i++ {
		sub, err = s.js.QueueSubscribe(
			"tenant.>", // Subscribe to all tenant events
			"tenant-router-workers", // Queue group for load balancing
			s.handleTenantEvent,
			nats.Durable("tenant-router-consumer"),
			nats.DeliverNew(),            // Only process new messages (HTTP fallback handles missed ones)
			nats.ManualAck(),
			nats.AckWait(60*time.Second), // Give more time for K8s operations
			nats.MaxDeliver(5),           // Retry up to 5 times at NATS level
			nats.MaxAckPending(10),       // Limit concurrent processing per subscriber
			nats.BindStream(StreamName),  // Explicitly bind to TENANT_EVENTS stream
		)
		if err == nil {
			break
		}
		log.Printf("[NATS] Attempt %d: Failed to subscribe: %v, retrying...", i+1, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to subscribe to tenant events after retries: %w", err)
	}
	s.subs = append(s.subs, sub)
	log.Printf("[NATS] Subscribed to tenant.> events on stream %s with queue group tenant-router-workers", StreamName)

	log.Printf("[NATS] All subscriptions started")
	return nil
}

// handleTenantEvent routes events to appropriate handlers based on subject
func (s *Subscriber) handleTenantEvent(msg *nats.Msg) {
	subject := msg.Subject
	log.Printf("[NATS] Received event on subject: %s", subject)

	switch subject {
	case SubjectTenantCreated:
		s.handleTenantCreated(msg)
	case SubjectTenantDeleted:
		s.handleTenantDeleted(msg)
	default:
		// Ignore other tenant events (tenant.updated, tenant.verified, etc.)
		log.Printf("[NATS] Ignoring event on subject: %s", subject)
		msg.Ack()
	}
}

// handleTenantCreated processes tenant.created events
func (s *Subscriber) handleTenantCreated(msg *nats.Msg) {
	log.Printf("[NATS] Received %s event", SubjectTenantCreated)

	var event models.TenantCreatedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal tenant.created event: %v", err)
		// Ack anyway to prevent infinite retries for malformed messages
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing tenant.created: slug=%s tenant_id=%s admin_host=%s storefront_host=%s",
		event.Slug, event.TenantID, event.AdminHost, event.StorefrontHost)

	// Enqueue for reconciliation (non-blocking)
	if err := s.reconciler.EnqueueCreate(&event); err != nil {
		log.Printf("[NATS] Failed to enqueue create for %s: %v", event.Slug, err)
		// NAK to retry later
		msg.Nak()
		return
	}

	// ACK immediately - reconciler handles retries internally
	log.Printf("[NATS] Enqueued tenant.created for %s", event.Slug)
	msg.Ack()
}

// handleTenantDeleted processes tenant.deleted events
func (s *Subscriber) handleTenantDeleted(msg *nats.Msg) {
	log.Printf("[NATS] Received %s event", SubjectTenantDeleted)

	var event models.TenantDeletedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal tenant.deleted event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing tenant.deleted: slug=%s tenant_id=%s", event.Slug, event.TenantID)

	// Enqueue for reconciliation (non-blocking)
	if err := s.reconciler.EnqueueDelete(&event); err != nil {
		log.Printf("[NATS] Failed to enqueue delete for %s: %v", event.Slug, err)
		msg.Nak()
		return
	}

	log.Printf("[NATS] Enqueued tenant.deleted for %s", event.Slug)
	msg.Ack()
}

// Stop stops all subscriptions gracefully
// This is called during shutdown to properly release the consumer binding
func (s *Subscriber) Stop() error {
	log.Printf("[NATS] Stopping subscriptions gracefully...")

	// Drain subscriptions first - this processes any pending messages
	// and properly releases the consumer binding for queue groups
	for _, sub := range s.subs {
		if sub.IsValid() {
			log.Printf("[NATS] Draining subscription...")
			if err := sub.Drain(); err != nil {
				log.Printf("[NATS] Failed to drain subscription: %v", err)
			} else {
				// Wait for drain to complete (with timeout)
				time.Sleep(2 * time.Second)
				log.Printf("[NATS] Subscription drained successfully")
			}
		}
	}

	// Close connection after subscriptions are drained
	if s.conn != nil && !s.conn.IsClosed() {
		// Drain connection to process any pending messages
		if err := s.conn.Drain(); err != nil {
			log.Printf("[NATS] Failed to drain connection: %v", err)
		}
		log.Printf("[NATS] Connection closed gracefully")
	}

	return nil
}

// IsConnected returns true if connected to NATS
func (s *Subscriber) IsConnected() bool {
	return s.conn != nil && s.conn.IsConnected()
}
