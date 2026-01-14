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

// Publisher wraps the shared events publisher for QR-specific events
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
		config.Name = "qr-service"

		pub, err := events.NewPublisher(config, logger)
		if err != nil {
			initErr = err
			return
		}

		ctx := context.Background()
		if err := pub.EnsureStream(ctx, events.StreamQR, []string{"qr.>"}); err != nil {
			logger.WithError(err).Warn("Failed to ensure QR_EVENTS stream")
		}

		publisherMu.Lock()
		publisher = &Publisher{
			publisher: pub,
			logger:    logger.WithField("component", "events.publisher"),
		}
		publisherMu.Unlock()

		logger.Info("NATS events publisher initialized for qr-service")
	})
	return initErr
}

// GetPublisher returns the singleton publisher instance
func GetPublisher() *Publisher {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return publisher
}

// PublishQRGenerated publishes a QR code generated event
func (p *Publisher) PublishQRGenerated(ctx context.Context, tenantID, qrID, qrType, content, format string, size int, entityType, entityID, expiresAt string) error {
	event := events.NewQREvent(events.QRGenerated, tenantID)
	event.QRID = qrID
	event.QRType = qrType
	event.Content = content
	event.Format = format
	event.Size = size
	event.EntityType = entityType
	event.EntityID = entityID
	event.ExpiresAt = expiresAt

	return p.publisher.Publish(ctx, event)
}

// PublishQRScanned publishes a QR code scanned event
func (p *Publisher) PublishQRScanned(ctx context.Context, tenantID, qrID, qrType, content, scannedBy, scannerIP, scannerType string) error {
	event := events.NewQREvent(events.QRScanned, tenantID)
	event.QRID = qrID
	event.QRType = qrType
	event.Content = content
	event.ScannedBy = scannedBy
	event.ScannerIP = scannerIP
	event.ScannerType = scannerType

	return p.publisher.Publish(ctx, event)
}

// PublishQRExpired publishes a QR code expired event
func (p *Publisher) PublishQRExpired(ctx context.Context, tenantID, qrID, qrType, entityType, entityID string) error {
	event := events.NewQREvent(events.QRExpired, tenantID)
	event.QRID = qrID
	event.QRType = qrType
	event.EntityType = entityType
	event.EntityID = entityID
	event.IsExpired = true

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
