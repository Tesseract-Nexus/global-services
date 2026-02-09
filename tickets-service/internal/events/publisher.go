package events

import (
	"context"
	"os"

	"github.com/Tesseract-Nexus/go-shared/events"
	"github.com/sirupsen/logrus"
)

// Publisher wraps the shared events publisher for ticket-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new ticket events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "tickets-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamTickets, []string{"ticket.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure TICKET_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishTicketCreated publishes a ticket created event
func (p *Publisher) PublishTicketCreated(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, customerName, subject, category, priority, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketCreated, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.CustomerName = customerName
	event.Subject = subject
	event.Category = category
	event.Priority = priority
	event.Status = "OPEN"
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent

	return p.publisher.Publish(ctx, event)
}

// PublishTicketUpdated publishes a ticket updated event
func (p *Publisher) PublishTicketUpdated(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, subject, status, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketUpdated, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.Subject = subject
	event.Status = status
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent

	return p.publisher.Publish(ctx, event)
}

// PublishTicketDeleted publishes a ticket deleted event
func (p *Publisher) PublishTicketDeleted(ctx context.Context, tenantID, ticketID, ticketNumber, subject, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketDeleted, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.Subject = subject
	event.Status = "DELETED"
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent

	return p.publisher.Publish(ctx, event)
}

// PublishTicketStatusChanged publishes a ticket status changed event
func (p *Publisher) PublishTicketStatusChanged(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, subject, oldStatus, newStatus, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketStatusChanged, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.Subject = subject
	event.Status = newStatus
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.Metadata = map[string]interface{}{
		"oldStatus": oldStatus,
		"newStatus": newStatus,
	}

	return p.publisher.Publish(ctx, event)
}

// PublishTicketAssigned publishes a ticket assigned event
func (p *Publisher) PublishTicketAssigned(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, subject, assignedTo, assignedToName, team, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketAssigned, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.Subject = subject
	event.AssignedTo = assignedTo
	event.AssignedToName = assignedToName
	event.Team = team
	event.Status = "IN_PROGRESS"
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent

	return p.publisher.Publish(ctx, event)
}

// PublishTicketUnassigned publishes a ticket unassigned event
func (p *Publisher) PublishTicketUnassigned(ctx context.Context, tenantID, ticketID, ticketNumber, subject, previousAssignee, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketUnassigned, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.Subject = subject
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.Metadata = map[string]interface{}{
		"previousAssignee": previousAssignee,
	}

	return p.publisher.Publish(ctx, event)
}

// PublishTicketCommentAdded publishes a ticket comment added event
func (p *Publisher) PublishTicketCommentAdded(ctx context.Context, tenantID, ticketID, ticketNumber, subject, commentContent string, isInternal bool, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketCommentAdded, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.Subject = subject
	event.CommentContent = commentContent
	event.CommentBy = actorID
	event.CommentByName = actorName
	event.IsInternal = isInternal
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent

	return p.publisher.Publish(ctx, event)
}

// PublishTicketResolved publishes a ticket resolved event
func (p *Publisher) PublishTicketResolved(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, subject, resolution, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketResolved, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.Subject = subject
	event.Resolution = resolution
	event.ResolvedBy = actorID
	event.Status = "RESOLVED"
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent

	return p.publisher.Publish(ctx, event)
}

// PublishTicketClosed publishes a ticket closed event
func (p *Publisher) PublishTicketClosed(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, subject, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketClosed, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.Subject = subject
	event.ClosedBy = actorID
	event.Status = "CLOSED"
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent

	return p.publisher.Publish(ctx, event)
}

// PublishTicketEscalated publishes a ticket escalated event
func (p *Publisher) PublishTicketEscalated(ctx context.Context, tenantID, ticketID, ticketNumber, subject, reason, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := events.NewTicketEvent(events.TicketEscalated, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.Subject = subject
	event.Status = "ESCALATED"
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.Metadata = map[string]interface{}{
		"reason": reason,
	}

	return p.publisher.Publish(ctx, event)
}

// IsConnected returns true if connected to NATS
func (p *Publisher) IsConnected() bool {
	return p.publisher.IsConnected()
}

// Close closes the publisher connection
func (p *Publisher) Close() {
	p.publisher.Close()
}
