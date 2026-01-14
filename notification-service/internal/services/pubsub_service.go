package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/sirupsen/logrus"
)

// PubSubService handles GCP Pub/Sub integration for async notification processing
type PubSubService struct {
	client       *pubsub.Client
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
	projectID    string
	topicID      string
	subID        string
	logger       *logrus.Logger
}

// NotificationMessage represents a message to be sent via Pub/Sub
type NotificationMessage struct {
	NotificationID string                 `json:"notification_id"`
	TenantID       string                 `json:"tenant_id"`
	Channel        string                 `json:"channel"`
	Priority       string                 `json:"priority"`
	To             string                 `json:"to"`
	Subject        string                 `json:"subject"`
	Body           string                 `json:"body"`
	BodyHTML       string                 `json:"body_html"`
	TemplateID     string                 `json:"template_id"`
	Variables      map[string]interface{} `json:"variables"`
	Metadata       map[string]interface{} `json:"metadata"`
	ScheduledFor   *time.Time             `json:"scheduled_for"`
}

// NewPubSubService creates a new Pub/Sub service
func NewPubSubService(projectID, topicID, subscriptionID string, logger *logrus.Logger) (*PubSubService, error) {
	ctx := context.Background()

	// Create Pub/Sub client
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Pub/Sub client: %w", err)
	}

	// Get or create topic
	topic := client.Topic(topicID)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if topic exists: %w", err)
	}

	if !exists {
		topic, err = client.CreateTopic(ctx, topicID)
		if err != nil {
			return nil, fmt.Errorf("failed to create topic: %w", err)
		}
		logger.WithField("topic", topicID).Info("Created Pub/Sub topic")
	}

	// Get or create subscription
	subscription := client.Subscription(subscriptionID)
	exists, err = subscription.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if subscription exists: %w", err)
	}

	if !exists {
		subscription, err = client.CreateSubscription(ctx, subscriptionID, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 60 * time.Second,
			RetryPolicy: &pubsub.RetryPolicy{
				MinimumBackoff: 10 * time.Second,
				MaximumBackoff: 600 * time.Second,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create subscription: %w", err)
		}
		logger.WithField("subscription", subscriptionID).Info("Created Pub/Sub subscription")
	}

	return &PubSubService{
		client:       client,
		topic:        topic,
		subscription: subscription,
		projectID:    projectID,
		topicID:      topicID,
		subID:        subscriptionID,
		logger:       logger,
	}, nil
}

// Publish publishes a notification message to Pub/Sub
func (s *PubSubService) Publish(ctx context.Context, msg *NotificationMessage) error {
	// Marshal message to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create Pub/Sub message
	pubsubMsg := &pubsub.Message{
		Data: data,
		Attributes: map[string]string{
			"notification_id": msg.NotificationID,
			"tenant_id":       msg.TenantID,
			"channel":         msg.Channel,
			"priority":        msg.Priority,
		},
	}

	// Set ordering key based on tenant for better distribution
	pubsubMsg.OrderingKey = msg.TenantID

	// Publish message
	result := s.topic.Publish(ctx, pubsubMsg)

	// Wait for result
	messageID, err := result.Get(ctx)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"notification_id": msg.NotificationID,
			"channel":         msg.Channel,
		}).Error("Failed to publish message to Pub/Sub")
		return fmt.Errorf("failed to publish message: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"message_id":      messageID,
		"notification_id": msg.NotificationID,
		"channel":         msg.Channel,
		"priority":        msg.Priority,
	}).Debug("Published message to Pub/Sub")

	return nil
}

// PublishBatch publishes multiple notification messages to Pub/Sub
func (s *PubSubService) PublishBatch(ctx context.Context, messages []*NotificationMessage) error {
	results := make([]*pubsub.PublishResult, len(messages))

	// Publish all messages
	for i, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			s.logger.WithError(err).Error("Failed to marshal message")
			continue
		}

		pubsubMsg := &pubsub.Message{
			Data: data,
			Attributes: map[string]string{
				"notification_id": msg.NotificationID,
				"tenant_id":       msg.TenantID,
				"channel":         msg.Channel,
				"priority":        msg.Priority,
			},
			OrderingKey: msg.TenantID,
		}

		results[i] = s.topic.Publish(ctx, pubsubMsg)
	}

	// Wait for all results
	var errors []error
	for i, result := range results {
		if result == nil {
			continue
		}

		_, err := result.Get(ctx)
		if err != nil {
			errors = append(errors, fmt.Errorf("message %d: %w", i, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to publish %d messages: %v", len(errors), errors)
	}

	s.logger.WithField("count", len(messages)).Info("Published batch messages to Pub/Sub")
	return nil
}

// Subscribe starts consuming messages from Pub/Sub
func (s *PubSubService) Subscribe(ctx context.Context, handler func(context.Context, *NotificationMessage) error) error {
	// Configure subscription settings
	s.subscription.ReceiveSettings.MaxOutstandingMessages = 10
	s.subscription.ReceiveSettings.MaxOutstandingBytes = 10e6 // 10MB
	s.subscription.ReceiveSettings.NumGoroutines = 10

	s.logger.WithField("subscription", s.subID).Info("Starting Pub/Sub subscriber")

	// Start receiving messages
	err := s.subscription.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		// Parse message
		var notificationMsg NotificationMessage
		if err := json.Unmarshal(msg.Data, &notificationMsg); err != nil {
			s.logger.WithError(err).Error("Failed to unmarshal message")
			msg.Nack()
			return
		}

		// Log received message
		s.logger.WithFields(logrus.Fields{
			"notification_id": notificationMsg.NotificationID,
			"channel":         notificationMsg.Channel,
			"priority":        notificationMsg.Priority,
		}).Debug("Received message from Pub/Sub")

		// Process message
		if err := handler(ctx, &notificationMsg); err != nil {
			s.logger.WithError(err).WithField("notification_id", notificationMsg.NotificationID).Error("Failed to process message")
			msg.Nack()
			return
		}

		// Acknowledge message
		msg.Ack()

		s.logger.WithField("notification_id", notificationMsg.NotificationID).Debug("Acknowledged message")
	})

	if err != nil {
		return fmt.Errorf("failed to receive messages: %w", err)
	}

	return nil
}

// Close closes the Pub/Sub client
func (s *PubSubService) Close() error {
	s.topic.Stop()

	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// GetQueueDepth returns the approximate number of messages in the queue
func (s *PubSubService) GetQueueDepth(ctx context.Context) (int64, error) {
	// This is an approximation - Pub/Sub doesn't provide exact queue depth
	// We use the NumUndeliveredMessages metric from subscription config
	config, err := s.subscription.Config(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get subscription config: %w", err)
	}

	// Note: This is not a real-time metric
	// For real-time metrics, you'd need to use Cloud Monitoring API
	_ = config // Subscription config doesn't directly provide queue depth

	// Return 0 for now - implement Cloud Monitoring integration if needed
	return 0, nil
}

// PurgeQueue removes all messages from the subscription (for testing/admin purposes)
func (s *PubSubService) PurgeQueue(ctx context.Context) error {
	// Seek to current time to skip all existing messages
	if err := s.subscription.SeekToTime(ctx, time.Now()); err != nil {
		return fmt.Errorf("failed to purge queue: %w", err)
	}

	s.logger.WithField("subscription", s.subID).Info("Purged Pub/Sub queue")
	return nil
}
