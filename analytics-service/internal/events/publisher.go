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

// Publisher wraps the shared events publisher for analytics-specific events
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
		config.Name = "analytics-service"

		pub, err := events.NewPublisher(config, logger)
		if err != nil {
			initErr = err
			return
		}

		ctx := context.Background()
		if err := pub.EnsureStream(ctx, events.StreamAnalytics, []string{"analytics.>"}); err != nil {
			logger.WithError(err).Warn("Failed to ensure ANALYTICS_EVENTS stream")
		}

		publisherMu.Lock()
		publisher = &Publisher{
			publisher: pub,
			logger:    logger.WithField("component", "events.publisher"),
		}
		publisherMu.Unlock()

		logger.Info("NATS events publisher initialized for analytics-service")
	})
	return initErr
}

// GetPublisher returns the singleton publisher instance
func GetPublisher() *Publisher {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return publisher
}

// PublishEventTracked publishes an analytics event tracked event
func (p *Publisher) PublishEventTracked(ctx context.Context, tenantID, analyticsID, eventName, eventCategory, userID, sessionID, pageURL string, properties map[string]interface{}) error {
	event := events.NewAnalyticsEvent(events.AnalyticsEventTracked, tenantID)
	event.AnalyticsID = analyticsID
	event.EventName = eventName
	event.EventCategory = eventCategory
	event.UserID = userID
	event.SessionID = sessionID
	event.PageURL = pageURL
	event.Properties = properties

	return p.publisher.Publish(ctx, event)
}

// PublishPageViewed publishes a page view event
func (p *Publisher) PublishPageViewed(ctx context.Context, tenantID, analyticsID, userID, sessionID, pageURL, pageTitle, pagePath, referrer, deviceType, browser, os string) error {
	event := events.NewAnalyticsEvent(events.AnalyticsPageViewed, tenantID)
	event.AnalyticsID = analyticsID
	event.EventName = "page_view"
	event.EventCategory = "pageview"
	event.UserID = userID
	event.SessionID = sessionID
	event.PageURL = pageURL
	event.PageTitle = pageTitle
	event.PagePath = pagePath
	event.Referrer = referrer
	event.DeviceType = deviceType
	event.Browser = browser
	event.OS = os

	return p.publisher.Publish(ctx, event)
}

// PublishGoalCompleted publishes a goal/conversion completed event
func (p *Publisher) PublishGoalCompleted(ctx context.Context, tenantID, analyticsID, userID, sessionID, goalID, goalName string, goalValue float64, properties map[string]interface{}) error {
	event := events.NewAnalyticsEvent(events.AnalyticsGoalCompleted, tenantID)
	event.AnalyticsID = analyticsID
	event.EventName = "goal_completed"
	event.EventCategory = "conversion"
	event.UserID = userID
	event.SessionID = sessionID
	event.GoalID = goalID
	event.GoalName = goalName
	event.GoalValue = goalValue
	event.Properties = properties

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
