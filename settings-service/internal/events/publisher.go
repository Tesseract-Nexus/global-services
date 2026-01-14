package events

import (
	"context"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

var (
	publisher     *Publisher
	publisherOnce sync.Once
	publisherMu   sync.RWMutex
)

// Publisher wraps the shared events publisher for settings-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// InitPublisher initializes the singleton NATS publisher
func InitPublisher(logger *logrus.Logger) error {
	var initErr error
	publisherOnce.Do(func() {
		natsURL := os.Getenv("NATS_URL")
		if natsURL == "" {
			logger.Warn("NATS_URL not set, event publishing disabled")
			return
		}

		config := events.DefaultPublisherConfig(natsURL)
		config.Name = "settings-service"

		pub, err := events.NewPublisher(config, logger)
		if err != nil {
			initErr = err
			return
		}

		ctx := context.Background()
		if err := pub.EnsureStream(ctx, events.StreamSettings, []string{"settings.>"}); err != nil {
			logger.WithError(err).Warn("Failed to ensure SETTINGS_EVENTS stream")
		}

		publisherMu.Lock()
		publisher = &Publisher{
			publisher: pub,
			logger:    logger.WithField("component", "events.publisher"),
		}
		publisherMu.Unlock()

		logger.Info("NATS events publisher initialized for settings-service")
	})
	return initErr
}

// GetPublisher returns the singleton publisher instance
func GetPublisher() *Publisher {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return publisher
}

// PublishSettingUpdated publishes a setting updated event
func (p *Publisher) PublishSettingUpdated(ctx context.Context, tenantID, settingKey, category string, oldValue, newValue interface{}, changedBy, changedByName string) error {
	event := events.NewSettingsEvent(events.SettingsUpdated, tenantID)
	event.SettingKey = settingKey
	event.SettingCategory = category
	event.OldValue = oldValue
	event.NewValue = newValue
	event.ChangedBy = changedBy
	event.ChangedByName = changedByName

	return p.publisher.Publish(ctx, event)
}

// PublishSettingCreated publishes a setting created event
func (p *Publisher) PublishSettingCreated(ctx context.Context, tenantID, settingKey, category string, value interface{}, changedBy, changedByName string) error {
	event := events.NewSettingsEvent(events.SettingsCreated, tenantID)
	event.SettingKey = settingKey
	event.SettingCategory = category
	event.NewValue = value
	event.ChangedBy = changedBy
	event.ChangedByName = changedByName

	return p.publisher.Publish(ctx, event)
}

// PublishBulkSettingsUpdated publishes a bulk settings update event
func (p *Publisher) PublishBulkSettingsUpdated(ctx context.Context, tenantID string, changedSettings []string, changedBy, changedByName string) error {
	event := events.NewSettingsEvent(events.SettingsBulkUpdated, tenantID)
	event.ChangedSettings = changedSettings
	event.SettingsCount = len(changedSettings)
	event.ChangedBy = changedBy
	event.ChangedByName = changedByName

	return p.publisher.Publish(ctx, event)
}

// IsConnected returns true if connected to NATS
func (p *Publisher) IsConnected() bool {
	return p.publisher != nil && p.publisher.IsConnected()
}

// Close closes the publisher connection
func (p *Publisher) Close() {
	if p.publisher != nil {
		p.publisher.Close()
	}
}
