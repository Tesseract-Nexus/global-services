package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// EventHandler is called when an audit event is received
type EventHandler func(event *AuditEvent)

// Subscriber handles subscribing to audit events from NATS
type Subscriber struct {
	client       *Client
	logger       *logrus.Logger
	subscriptions map[string]*nats.Subscription
	mu           sync.Mutex
}

// NewSubscriber creates a new audit event subscriber
func NewSubscriber(client *Client, logger *logrus.Logger) *Subscriber {
	return &Subscriber{
		client:       client,
		logger:       logger,
		subscriptions: make(map[string]*nats.Subscription),
	}
}

// SubscribeToTenant subscribes to audit events for a specific tenant
// Returns a channel that receives audit events and a cleanup function
func (s *Subscriber) SubscribeToTenant(ctx context.Context, tenantID string) (<-chan *AuditEvent, func(), error) {
	if s.client == nil || !s.client.IsConnected() {
		return nil, nil, fmt.Errorf("NATS not connected")
	}

	// Create buffered channel for events
	eventChan := make(chan *AuditEvent, 100)

	// Subscribe to tenant-specific events: audit.{tenant_id}.>
	subject := fmt.Sprintf("audit.%s.>", tenantID)

	// Use simple subscription (not JetStream consumer) for real-time
	sub, err := s.client.Conn().Subscribe(subject, func(msg *nats.Msg) {
		var event AuditEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			s.logger.WithError(err).Warn("Failed to unmarshal audit event")
			return
		}

		// Non-blocking send to channel
		select {
		case eventChan <- &event:
		default:
			s.logger.Warn("Event channel full, dropping event")
		}
	})

	if err != nil {
		close(eventChan)
		return nil, nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	// Store subscription for tracking
	subKey := fmt.Sprintf("%s-%p", tenantID, eventChan)
	s.mu.Lock()
	s.subscriptions[subKey] = sub
	s.mu.Unlock()

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"subject":   subject,
	}).Info("Subscribed to tenant audit events")

	// Cleanup function
	cleanup := func() {
		s.mu.Lock()
		delete(s.subscriptions, subKey)
		s.mu.Unlock()

		if err := sub.Unsubscribe(); err != nil {
			s.logger.WithError(err).Warn("Error unsubscribing")
		}
		close(eventChan)

		s.logger.WithField("tenant_id", tenantID).Debug("Unsubscribed from tenant audit events")
	}

	return eventChan, cleanup, nil
}

// SubscribeToAll subscribes to all audit events (for admin use)
func (s *Subscriber) SubscribeToAll(ctx context.Context) (<-chan *AuditEvent, func(), error) {
	if s.client == nil || !s.client.IsConnected() {
		return nil, nil, fmt.Errorf("NATS not connected")
	}

	eventChan := make(chan *AuditEvent, 100)

	// Subscribe to all audit events
	sub, err := s.client.Conn().Subscribe("audit.>", func(msg *nats.Msg) {
		var event AuditEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			s.logger.WithError(err).Warn("Failed to unmarshal audit event")
			return
		}

		select {
		case eventChan <- &event:
		default:
			s.logger.Warn("Event channel full, dropping event")
		}
	})

	if err != nil {
		close(eventChan)
		return nil, nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	subKey := fmt.Sprintf("all-%p", eventChan)
	s.mu.Lock()
	s.subscriptions[subKey] = sub
	s.mu.Unlock()

	cleanup := func() {
		s.mu.Lock()
		delete(s.subscriptions, subKey)
		s.mu.Unlock()

		sub.Unsubscribe()
		close(eventChan)
	}

	return eventChan, cleanup, nil
}

// Close closes all subscriptions
func (s *Subscriber) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, sub := range s.subscriptions {
		if err := sub.Unsubscribe(); err != nil {
			s.logger.WithError(err).Warn("Error closing subscription")
		}
		delete(s.subscriptions, key)
	}
}

// GetStats returns subscription statistics
func (s *Subscriber) GetStats() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]interface{}{
		"active_subscriptions": len(s.subscriptions),
		"connected":           s.client != nil && s.client.IsConnected(),
	}
}
