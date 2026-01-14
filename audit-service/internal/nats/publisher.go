package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"audit-service/internal/models"
)

// AuditEvent represents an audit event for NATS publishing
type AuditEvent struct {
	Type     string           `json:"type"`     // "created", "updated", "deleted"
	TenantID string           `json:"tenant_id"`
	Log      *models.AuditLog `json:"log"`
}

// Publisher handles publishing audit events to NATS
type Publisher struct {
	client *Client
	logger *logrus.Logger
}

// NewPublisher creates a new audit event publisher
func NewPublisher(client *Client, logger *logrus.Logger) *Publisher {
	return &Publisher{
		client: client,
		logger: logger,
	}
}

// PublishAuditLog publishes an audit log event to NATS
func (p *Publisher) PublishAuditLog(ctx context.Context, eventType string, tenantID string, log *models.AuditLog) error {
	if p.client == nil || !p.client.IsConnected() {
		p.logger.Warn("NATS not connected, skipping event publish")
		return nil
	}

	event := AuditEvent{
		Type:     eventType,
		TenantID: tenantID,
		Log:      log,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Publish to tenant-specific subject: audit.{tenant_id}.{event_type}
	subject := fmt.Sprintf("audit.%s.%s", tenantID, eventType)

	// Use JetStream for guaranteed delivery
	ack, err := p.client.JetStream().Publish(subject, data, nats.Context(ctx))
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"event_type": eventType,
			"subject":    subject,
		}).WithError(err).Error("Failed to publish audit event")
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"tenant_id":  tenantID,
		"event_type": eventType,
		"sequence":   ack.Sequence,
		"stream":     ack.Stream,
	}).Debug("Published audit event")

	return nil
}

// PublishBatch publishes multiple audit logs
func (p *Publisher) PublishBatch(ctx context.Context, eventType string, tenantID string, logs []*models.AuditLog) error {
	for _, log := range logs {
		if err := p.PublishAuditLog(ctx, eventType, tenantID, log); err != nil {
			// Log error but continue with other logs
			p.logger.WithError(err).Warn("Failed to publish batch audit event")
		}
	}
	return nil
}
