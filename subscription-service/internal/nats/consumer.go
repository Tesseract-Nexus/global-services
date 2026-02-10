package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"subscription-service/internal/services"
)

// TenantCreatedEvent mirrors tenant-service's event payload
type TenantCreatedEvent struct {
	EventType    string    `json:"eventType"`
	TenantID     string    `json:"tenantId"`
	Email        string    `json:"email"`
	BusinessName string    `json:"businessName"`
	Slug         string    `json:"slug"`
	Country      string    `json:"country"`
	Timestamp    time.Time `json:"timestamp"`
}

// TenantEventConsumer consumes tenant.created events to auto-create trial subscriptions
type TenantEventConsumer struct {
	nc      *nats.Conn
	js      jetstream.JetStream
	service *services.SubscriptionService
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewTenantEventConsumer creates a new NATS consumer for tenant events
func NewTenantEventConsumer(natsURL string, service *services.SubscriptionService) (*TenantEventConsumer, error) {
	opts := []nats.Option{
		nats.Name("subscription-service-tenant-consumer"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.ReconnectBufSize(4 * 1024 * 1024),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("[NATS] Disconnected: %v", err)
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

	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &TenantEventConsumer{
		nc:      nc,
		js:      js,
		service: service,
		stopCh:  make(chan struct{}),
	}, nil
}

// Start begins consuming tenant.created events
func (c *TenantEventConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = true
	c.mu.Unlock()

	stream, err := c.js.Stream(ctx, "TENANT_EVENTS")
	if err != nil {
		log.Printf("[NATS] TENANT_EVENTS stream not found, will retry: %v", err)
		go c.retrySubscribe(ctx)
		return nil
	}

	return c.subscribe(ctx, stream)
}

func (c *TenantEventConsumer) retrySubscribe(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			stream, err := c.js.Stream(ctx, "TENANT_EVENTS")
			if err != nil {
				log.Printf("[NATS] TENANT_EVENTS stream still not found: %v", err)
				continue
			}
			if err := c.subscribe(ctx, stream); err != nil {
				log.Printf("[NATS] Failed to subscribe: %v", err)
				continue
			}
			log.Println("[NATS] Successfully subscribed to TENANT_EVENTS after retry")
			return
		}
	}
}

func (c *TenantEventConsumer) subscribe(ctx context.Context, stream jetstream.Stream) error {
	consumerName := "subscription-service-tenant-events"
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:           consumerName,
		Durable:        consumerName,
		AckPolicy:      jetstream.AckExplicitPolicy,
		DeliverPolicy:  jetstream.DeliverNewPolicy,
		FilterSubjects: []string{"tenant.created"},
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	go c.consumeMessages(ctx, consumer)
	log.Printf("[NATS] Subscribed to TENANT_EVENTS (filter: tenant.created)")
	return nil
}

func (c *TenantEventConsumer) consumeMessages(ctx context.Context, consumer jetstream.Consumer) {
	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		msgs, err := consumer.Fetch(10, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if err != context.DeadlineExceeded && err != nats.ErrTimeout {
				log.Printf("[NATS] Error fetching messages: %v", err)
			}
			continue
		}

		for msg := range msgs.Messages() {
			if err := c.handleMessage(ctx, msg); err != nil {
				log.Printf("[NATS] Failed to process tenant.created: %v", err)
				msg.Nak()
			} else {
				msg.Ack()
			}
		}
	}
}

func (c *TenantEventConsumer) handleMessage(ctx context.Context, msg jetstream.Msg) error {
	var event TenantCreatedEvent
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return fmt.Errorf("failed to unmarshal tenant event: %w", err)
	}

	if event.TenantID == "" {
		log.Println("[NATS] Skipping event with empty tenantId")
		return nil
	}

	region := "global"
	if event.Country != "" {
		region = strings.ToLower(event.Country)
	}

	log.Printf("[NATS] Processing tenant.created for tenant %s (region: %s)", event.TenantID, region)

	if err := c.service.CreateTrialSubscription(ctx, event.TenantID, event.Email, event.BusinessName, region); err != nil {
		return fmt.Errorf("failed to create trial subscription for tenant %s: %w", event.TenantID, err)
	}

	log.Printf("[NATS] Trial subscription created for tenant %s", event.TenantID)
	return nil
}

// Stop stops the consumer gracefully
func (c *TenantEventConsumer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	close(c.stopCh)
	c.running = false

	if c.nc != nil {
		c.nc.Drain()
		c.nc.Close()
	}

	log.Println("[NATS] Tenant event consumer stopped")
}
