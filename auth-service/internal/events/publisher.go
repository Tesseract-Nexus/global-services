package events

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Publisher wraps the shared events publisher for auth-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new auth events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "auth-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	// Ensure the AUTH_EVENTS stream exists
	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamAuth, []string{"auth.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure AUTH_EVENTS stream (may already exist)")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishLoginSuccess publishes a successful login event
func (p *Publisher) PublishLoginSuccess(ctx context.Context, tenantID, userID, email, ipAddress, userAgent string) error {
	event := events.NewAuthEvent(events.LoginSuccess, tenantID)
	event.UserID = userID
	event.Email = email
	event.IPAddress = ipAddress
	event.UserAgent = userAgent
	event.LoginMethod = "password"

	if err := p.publisher.PublishAuth(ctx, event); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"user_id":   userID,
			"email":     email,
		}).Error("Failed to publish login success event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"user_id":   userID,
	}).Info("Login success event published")
	return nil
}

// PublishLoginFailed publishes a failed login event
func (p *Publisher) PublishLoginFailed(ctx context.Context, tenantID, email, ipAddress, userAgent, reason string, failedAttempts int) error {
	event := events.NewAuthEvent(events.LoginFailed, tenantID)
	event.Email = email
	event.IPAddress = ipAddress
	event.UserAgent = userAgent
	event.FailedAttempts = failedAttempts
	event.Metadata = map[string]interface{}{
		"reason": reason,
	}

	if err := p.publisher.PublishAuth(ctx, event); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"email":     email,
		}).Error("Failed to publish login failed event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"tenant_id":       tenantID,
		"email":           email,
		"failed_attempts": failedAttempts,
	}).Warn("Login failed event published")
	return nil
}

// PublishPasswordReset publishes a password reset request event
func (p *Publisher) PublishPasswordReset(ctx context.Context, tenantID, userID, email string) error {
	event := events.NewAuthEvent(events.PasswordReset, tenantID)
	event.UserID = userID
	event.Email = email

	if err := p.publisher.PublishAuth(ctx, event); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"email":     email,
		}).Error("Failed to publish password reset event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"user_id":   userID,
	}).Info("Password reset event published")
	return nil
}

// PublishPasswordChanged publishes a password changed event
func (p *Publisher) PublishPasswordChanged(ctx context.Context, tenantID, userID, email, ipAddress string) error {
	event := events.NewAuthEvent(events.PasswordChanged, tenantID)
	event.UserID = userID
	event.Email = email
	event.IPAddress = ipAddress

	if err := p.publisher.PublishAuth(ctx, event); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"user_id":   userID,
		}).Error("Failed to publish password changed event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"user_id":   userID,
	}).Info("Password changed event published")
	return nil
}

// PublishEmailVerified publishes an email verification event
func (p *Publisher) PublishEmailVerified(ctx context.Context, tenantID, userID, email string) error {
	event := events.NewAuthEvent(events.EmailVerified, tenantID)
	event.UserID = userID
	event.Email = email

	if err := p.publisher.PublishAuth(ctx, event); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"email":     email,
		}).Error("Failed to publish email verified event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"user_id":   userID,
	}).Info("Email verified event published")
	return nil
}

// PublishAccountLocked publishes an account locked event
func (p *Publisher) PublishAccountLocked(ctx context.Context, tenantID, userID, email, reason string, lockedUntil string) error {
	event := events.NewAuthEvent(events.AccountLocked, tenantID)
	event.UserID = userID
	event.Email = email
	event.LockReason = reason
	event.LockedUntil = lockedUntil

	if err := p.publisher.PublishAuth(ctx, event); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"user_id":   userID,
		}).Error("Failed to publish account locked event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"tenant_id":    tenantID,
		"user_id":      userID,
		"locked_until": lockedUntil,
	}).Warn("Account locked event published")
	return nil
}

// PublishAccountUnlocked publishes an account unlocked event (admin action)
func (p *Publisher) PublishAccountUnlocked(ctx context.Context, tenantID, userID, email, adminUserID string) error {
	event := events.NewAuthEvent(events.AccountUnlocked, tenantID)
	event.UserID = userID
	event.Email = email
	event.Metadata = map[string]interface{}{
		"unlocked_by": adminUserID,
	}

	if err := p.publisher.PublishAuth(ctx, event); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"tenant_id":   tenantID,
			"user_id":     userID,
			"unlocked_by": adminUserID,
		}).Error("Failed to publish account unlocked event")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"tenant_id":   tenantID,
		"user_id":     userID,
		"unlocked_by": adminUserID,
	}).Info("Account unlocked event published")
	return nil
}

// IsConnected returns true if connected to NATS
func (p *Publisher) IsConnected() bool {
	return p.publisher.IsConnected()
}

// Close closes the publisher connection
func (p *Publisher) Close() {
	p.publisher.Close()
}
