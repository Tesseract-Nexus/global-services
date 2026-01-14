package events

import (
	"context"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/tesseract-hub/go-shared/events"
)

var (
	publisher     *Publisher
	publisherOnce sync.Once
	publisherMu   sync.RWMutex
)

// Publisher wraps the shared events publisher for location-specific events
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
		config.Name = "location-service"

		pub, err := events.NewPublisher(config, logger)
		if err != nil {
			initErr = err
			return
		}

		ctx := context.Background()
		if err := pub.EnsureStream(ctx, events.StreamLocation, []string{"location.>"}); err != nil {
			logger.WithError(err).Warn("Failed to ensure LOCATION_EVENTS stream")
		}

		publisherMu.Lock()
		publisher = &Publisher{
			publisher: pub,
			logger:    logger.WithField("component", "events.publisher"),
		}
		publisherMu.Unlock()

		logger.Info("NATS events publisher initialized for location-service")
	})
	return initErr
}

// GetPublisher returns the singleton publisher instance
func GetPublisher() *Publisher {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return publisher
}

// PublishGeocoded publishes a geocoding result event
func (p *Publisher) PublishGeocoded(ctx context.Context, tenantID, queryID, queryAddress, formattedAddress string, lat, lng float64, city, state, postalCode, country, countryCode, placeID, provider string, responseTime int64, cacheHit bool) error {
	event := events.NewLocationEvent(events.LocationGeocoded, tenantID)
	event.QueryID = queryID
	event.QueryAddress = queryAddress
	event.FormattedAddress = formattedAddress
	event.Latitude = lat
	event.Longitude = lng
	event.City = city
	event.State = state
	event.PostalCode = postalCode
	event.Country = country
	event.CountryCode = countryCode
	event.PlaceID = placeID
	event.Provider = provider
	event.ResponseTime = responseTime
	event.CacheHit = cacheHit

	return p.publisher.Publish(ctx, event)
}

// PublishReverseLookup publishes a reverse geocoding result event
func (p *Publisher) PublishReverseLookup(ctx context.Context, tenantID, queryID string, lat, lng float64, formattedAddress, city, state, postalCode, country, provider string, responseTime int64, cacheHit bool) error {
	event := events.NewLocationEvent(events.LocationReverseLooked, tenantID)
	event.QueryID = queryID
	event.Latitude = lat
	event.Longitude = lng
	event.FormattedAddress = formattedAddress
	event.City = city
	event.State = state
	event.PostalCode = postalCode
	event.Country = country
	event.Provider = provider
	event.ResponseTime = responseTime
	event.CacheHit = cacheHit

	return p.publisher.Publish(ctx, event)
}

// PublishCached publishes a cache event
func (p *Publisher) PublishCached(ctx context.Context, tenantID, cacheKey string, cacheTTL int) error {
	event := events.NewLocationEvent(events.LocationCached, tenantID)
	event.CacheKey = cacheKey
	event.CacheTTL = cacheTTL
	event.CacheHit = false // This is a cache write

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
