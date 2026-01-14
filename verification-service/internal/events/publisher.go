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

// Publisher wraps the shared events publisher for verification-specific events
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
		config.Name = "verification-service"

		pub, err := events.NewPublisher(config, logger)
		if err != nil {
			initErr = err
			return
		}

		ctx := context.Background()
		if err := pub.EnsureStream(ctx, events.StreamVerification, []string{"verification.>"}); err != nil {
			logger.WithError(err).Warn("Failed to ensure VERIFICATION_EVENTS stream")
		}

		publisherMu.Lock()
		publisher = &Publisher{
			publisher: pub,
			logger:    logger.WithField("component", "events.publisher"),
		}
		publisherMu.Unlock()

		logger.Info("NATS events publisher initialized for verification-service")
	})
	return initErr
}

// GetPublisher returns the singleton publisher instance
func GetPublisher() *Publisher {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return publisher
}

// PublishCodeSent publishes a verification code sent event
func (p *Publisher) PublishCodeSent(ctx context.Context, tenantID, verificationID, verificationType, userID, email, phone, purpose string, expiresAt string) error {
	event := events.NewVerificationEvent(events.VerificationCodeSent, tenantID)
	event.VerificationID = verificationID
	event.VerificationType = verificationType
	event.UserID = userID
	event.Email = email
	event.Phone = phone
	event.Purpose = purpose
	event.ExpiresAt = expiresAt
	event.Status = "PENDING"

	return p.publisher.Publish(ctx, event)
}

// PublishVerified publishes a verification success event
func (p *Publisher) PublishVerified(ctx context.Context, tenantID, verificationID, verificationType, userID, email, phone string) error {
	event := events.NewVerificationEvent(events.VerificationVerified, tenantID)
	event.VerificationID = verificationID
	event.VerificationType = verificationType
	event.UserID = userID
	event.Email = email
	event.Phone = phone
	event.Status = "VERIFIED"

	return p.publisher.Publish(ctx, event)
}

// PublishFailed publishes a verification failed event
func (p *Publisher) PublishFailed(ctx context.Context, tenantID, verificationID, verificationType, userID, failureReason string, attemptCount int) error {
	event := events.NewVerificationEvent(events.VerificationFailed, tenantID)
	event.VerificationID = verificationID
	event.VerificationType = verificationType
	event.UserID = userID
	event.FailureReason = failureReason
	event.AttemptCount = attemptCount
	event.Status = "FAILED"

	return p.publisher.Publish(ctx, event)
}

// PublishExpired publishes a verification expired event
func (p *Publisher) PublishExpired(ctx context.Context, tenantID, verificationID, verificationType, userID string) error {
	event := events.NewVerificationEvent(events.VerificationExpired, tenantID)
	event.VerificationID = verificationID
	event.VerificationType = verificationType
	event.UserID = userID
	event.Status = "EXPIRED"

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
