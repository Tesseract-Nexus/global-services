package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sirupsen/logrus"

	"audit-service/internal/models"
	"audit-service/internal/services"
)

// DomainEventConsumer consumes domain events from all services and creates audit logs
type DomainEventConsumer struct {
	nc           *nats.Conn
	js           jetstream.JetStream
	auditService *services.AuditService
	logger       *logrus.Logger
	consumers    []jetstream.Consumer
	mu           sync.Mutex
	running      bool
	stopCh       chan struct{}
}

// ConsumerConfig holds configuration for the domain event consumer
type ConsumerConfig struct {
	NATSURL       string
	MaxReconnects int
	ReconnectWait time.Duration
}

// NewDomainEventConsumer creates a new domain event consumer with production-ready settings
func NewDomainEventConsumer(config ConsumerConfig, auditService *services.AuditService, logger *logrus.Logger) (*DomainEventConsumer, error) {
	// Use unlimited reconnects for production resilience
	maxReconnects := config.MaxReconnects
	if maxReconnects == 0 || maxReconnects > 0 {
		maxReconnects = -1 // Override to unlimited for production
	}

	opts := []nats.Option{
		nats.Name("audit-service-domain-consumer"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(maxReconnects),
		nats.ReconnectWait(config.ReconnectWait),
		nats.ReconnectBufSize(8 * 1024 * 1024), // 8MB buffer for messages during reconnect
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logger.WithError(err).Warn("[NATS] Disconnected")
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.WithField("url", nc.ConnectedUrl()).Info("[NATS] Reconnected")
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Info("[NATS] Connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.WithError(err).Error("[NATS] Error")
		}),
	}

	nc, err := nats.Connect(config.NATSURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &DomainEventConsumer{
		nc:           nc,
		js:           js,
		auditService: auditService,
		logger:       logger,
		consumers:    make([]jetstream.Consumer, 0),
		stopCh:       make(chan struct{}),
	}, nil
}

// Start starts consuming domain events from all streams
func (c *DomainEventConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = true
	c.mu.Unlock()

	// Define all streams to consume
	streams := []struct {
		name     string
		subjects []string
	}{
		{name: "ORDER_EVENTS", subjects: []string{"order.>"}},
		{name: "PAYMENT_EVENTS", subjects: []string{"payment.>"}},
		{name: "CUSTOMER_EVENTS", subjects: []string{"customer.>"}},
		{name: "AUTH_EVENTS", subjects: []string{"auth.>"}},
		{name: "INVENTORY_EVENTS", subjects: []string{"inventory.>"}},
		{name: "PRODUCT_EVENTS", subjects: []string{"product.>"}},
		{name: "RETURN_EVENTS", subjects: []string{"return.>"}},
		{name: "REVIEW_EVENTS", subjects: []string{"review.>"}},
		{name: "COUPON_EVENTS", subjects: []string{"coupon.>"}},
		{name: "VENDOR_EVENTS", subjects: []string{"vendor.>"}},
		{name: "GIFT_CARD_EVENTS", subjects: []string{"gift_card.>"}},
		{name: "TICKET_EVENTS", subjects: []string{"ticket.>"}},
		{name: "STAFF_EVENTS", subjects: []string{"staff.>"}},
		{name: "TENANT_EVENTS", subjects: []string{"tenant.>"}},
		{name: "APPROVAL_EVENTS", subjects: []string{"approval.>"}},
		{name: "CATEGORY_EVENTS", subjects: []string{"category.>"}},
		{name: "SHIPPING_EVENTS", subjects: []string{"shipping.>"}},
	}

	for _, stream := range streams {
		if err := c.subscribeToStream(ctx, stream.name, stream.subjects); err != nil {
			c.logger.WithError(err).WithField("stream", stream.name).Warn("Failed to subscribe to stream (may not exist yet)")
			// Continue with other streams - some may not exist yet
		}
	}

	c.logger.Info("Domain event consumer started")
	return nil
}

// subscribeToStream subscribes to a specific stream
func (c *DomainEventConsumer) subscribeToStream(ctx context.Context, streamName string, subjects []string) error {
	// Check if stream exists
	stream, err := c.js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("stream %s not found: %w", streamName, err)
	}

	// Create durable consumer for audit service
	consumerName := fmt.Sprintf("audit-service-%s", streamName)
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverNewPolicy,
		FilterSubjects: subjects,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer for %s: %w", streamName, err)
	}

	c.mu.Lock()
	c.consumers = append(c.consumers, consumer)
	c.mu.Unlock()

	// Start consuming in a goroutine
	go c.consumeMessages(ctx, consumer, streamName)

	c.logger.WithFields(logrus.Fields{
		"stream":   streamName,
		"consumer": consumerName,
	}).Info("Subscribed to domain event stream")

	return nil
}

// consumeMessages processes messages from a consumer
func (c *DomainEventConsumer) consumeMessages(ctx context.Context, consumer jetstream.Consumer, streamName string) {
	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		// Fetch messages in batches
		msgs, err := consumer.Fetch(10, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if err != context.DeadlineExceeded && err != nats.ErrTimeout {
				c.logger.WithError(err).WithField("stream", streamName).Warn("Error fetching messages")
			}
			continue
		}

		for msg := range msgs.Messages() {
			if err := c.processMessage(ctx, msg, streamName); err != nil {
				c.logger.WithError(err).WithField("stream", streamName).Error("Failed to process message")
				msg.Nak()
			} else {
				msg.Ack()
			}
		}
	}
}

// BaseEvent represents the common fields in all domain events
type BaseEvent struct {
	EventType     string                 `json:"eventType"`
	TenantID      string                 `json:"tenantId"`
	SourceID      string                 `json:"sourceId,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	TraceID       string                 `json:"traceId,omitempty"`
	CorrelationID string                 `json:"correlationId,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// processMessage processes a single domain event message
func (c *DomainEventConsumer) processMessage(ctx context.Context, msg jetstream.Msg, streamName string) error {
	// Parse base event to get common fields
	var baseEvent BaseEvent
	if err := json.Unmarshal(msg.Data(), &baseEvent); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Skip if no tenant ID
	if baseEvent.TenantID == "" {
		c.logger.Warn("Skipping event without tenant ID")
		return nil
	}

	// Convert to audit log
	auditLog := c.convertToAuditLog(msg.Subject(), &baseEvent, msg.Data())

	// Create audit log
	if err := c.auditService.LogAction(ctx, baseEvent.TenantID, auditLog); err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"tenant_id":  baseEvent.TenantID,
		"event_type": baseEvent.EventType,
		"action":     auditLog.Action,
		"resource":   auditLog.Resource,
	}).Debug("Created audit log from domain event")

	return nil
}

// convertToAuditLog converts a domain event to an audit log entry
func (c *DomainEventConsumer) convertToAuditLog(subject string, event *BaseEvent, rawData []byte) *models.AuditLog {
	action, resource, severity := c.mapEventToAudit(event.EventType)

	// Parse additional details from raw data
	var eventData map[string]interface{}
	json.Unmarshal(rawData, &eventData)

	auditLog := &models.AuditLog{
		ID:          uuid.New(),
		TenantID:    event.TenantID,
		Action:      action,
		Resource:    resource,
		Status:      models.StatusSuccess,
		Severity:    severity,
		Timestamp:   event.Timestamp,
		ServiceName: c.extractServiceName(subject),
		RequestID:   event.CorrelationID,
	}

	// Extract user info if present
	if userID, ok := eventData["userId"].(string); ok {
		if uid, err := uuid.Parse(userID); err == nil {
			auditLog.UserID = uid
		}
	}
	if customerID, ok := eventData["customerId"].(string); ok && auditLog.UserID == uuid.Nil {
		if uid, err := uuid.Parse(customerID); err == nil {
			auditLog.UserID = uid
		}
	}
	if staffID, ok := eventData["staffId"].(string); ok && auditLog.UserID == uuid.Nil {
		if uid, err := uuid.Parse(staffID); err == nil {
			auditLog.UserID = uid
		}
	}

	// Extract username/email
	if email, ok := eventData["customerEmail"].(string); ok {
		auditLog.UserEmail = email
	}
	if email, ok := eventData["email"].(string); ok && auditLog.UserEmail == "" {
		auditLog.UserEmail = email
	}
	if name, ok := eventData["customerName"].(string); ok {
		auditLog.Username = name
	}
	if name, ok := eventData["staffName"].(string); ok && auditLog.Username == "" {
		auditLog.Username = name
	}

	// Extract resource info
	auditLog.ResourceID, auditLog.ResourceName = c.extractResourceInfo(event.EventType, eventData)

	// Extract IP if present
	if ip, ok := eventData["ipAddress"].(string); ok {
		auditLog.IPAddress = ip
	}

	// Set description
	auditLog.Description = c.generateDescription(event.EventType, eventData)

	// Store old/new values for update events
	if oldValue, ok := eventData["oldValue"]; ok {
		if jsonBytes, err := json.Marshal(oldValue); err == nil {
			auditLog.OldValue = jsonBytes
		}
	}
	if newValue, ok := eventData["newValue"]; ok {
		if jsonBytes, err := json.Marshal(newValue); err == nil {
			auditLog.NewValue = jsonBytes
		}
	}

	// Check for failure status
	if status, ok := eventData["status"].(string); ok {
		if status == "FAILED" || status == "FAILURE" || status == "failed" {
			auditLog.Status = models.StatusFailure
		}
	}
	if _, ok := eventData["errorCode"]; ok {
		auditLog.Status = models.StatusFailure
	}
	if errMsg, ok := eventData["errorMessage"].(string); ok {
		auditLog.ErrorMessage = errMsg
		auditLog.Status = models.StatusFailure
	}

	return auditLog
}

// mapEventToAudit maps event types to audit action, resource, and severity
func (c *DomainEventConsumer) mapEventToAudit(eventType string) (models.AuditAction, models.AuditResource, models.AuditSeverity) {
	switch eventType {
	// Order events
	case "order.created":
		return models.ActionCreate, models.ResourceOrder, models.SeverityLow
	case "order.confirmed":
		return models.ActionUpdate, models.ResourceOrder, models.SeverityLow
	case "order.paid":
		return models.ActionUpdate, models.ResourceOrder, models.SeverityMedium
	case "order.shipped":
		return models.ActionUpdate, models.ResourceOrder, models.SeverityLow
	case "order.delivered":
		return models.ActionUpdate, models.ResourceOrder, models.SeverityLow
	case "order.cancelled":
		return models.ActionDelete, models.ResourceOrder, models.SeverityMedium
	case "order.refunded":
		return models.ActionUpdate, models.ResourceOrder, models.SeverityHigh

	// Payment events
	case "payment.pending":
		return models.ActionCreate, models.ResourcePayment, models.SeverityLow
	case "payment.captured", "payment.succeeded":
		return models.ActionUpdate, models.ResourcePayment, models.SeverityMedium
	case "payment.failed":
		return models.ActionUpdate, models.ResourcePayment, models.SeverityHigh
	case "payment.refunded":
		return models.ActionUpdate, models.ResourcePayment, models.SeverityHigh

	// Customer events
	case "customer.registered", "customer.created":
		return models.ActionCreate, models.ResourceCustomer, models.SeverityLow
	case "customer.updated":
		return models.ActionUpdate, models.ResourceCustomer, models.SeverityLow
	case "customer.deleted":
		return models.ActionDelete, models.ResourceCustomer, models.SeverityMedium

	// Auth events
	case "auth.login_success":
		return models.ActionLogin, models.ResourceAuth, models.SeverityLow
	case "auth.login_failed":
		return models.ActionLogin, models.ResourceAuth, models.SeverityHigh
	case "auth.logout":
		return models.ActionLogout, models.ResourceAuth, models.SeverityLow
	case "auth.password_reset":
		return models.ActionUpdate, models.ResourceAuth, models.SeverityMedium
	case "auth.password_changed":
		return models.ActionUpdate, models.ResourceAuth, models.SeverityMedium
	case "auth.account_locked":
		return models.ActionUpdate, models.ResourceAuth, models.SeverityCritical
	case "auth.email_verified", "auth.phone_verified":
		return models.ActionUpdate, models.ResourceAuth, models.SeverityLow

	// Product events
	case "product.created":
		return models.ActionCreate, models.ResourceProduct, models.SeverityLow
	case "product.updated":
		return models.ActionUpdate, models.ResourceProduct, models.SeverityLow
	case "product.deleted":
		return models.ActionDelete, models.ResourceProduct, models.SeverityMedium
	case "product.published":
		return models.ActionUpdate, models.ResourceProduct, models.SeverityLow
	case "product.archived":
		return models.ActionUpdate, models.ResourceProduct, models.SeverityLow

	// Inventory events
	case "inventory.low_stock":
		return models.ActionUpdate, models.ResourceInventory, models.SeverityMedium
	case "inventory.out_of_stock":
		return models.ActionUpdate, models.ResourceInventory, models.SeverityHigh
	case "inventory.restocked":
		return models.ActionUpdate, models.ResourceInventory, models.SeverityLow
	case "inventory.adjusted":
		return models.ActionUpdate, models.ResourceInventory, models.SeverityMedium

	// Staff events
	case "staff.created":
		return models.ActionCreate, models.ResourceStaff, models.SeverityMedium
	case "staff.updated":
		return models.ActionUpdate, models.ResourceStaff, models.SeverityMedium
	case "staff.deactivated":
		return models.ActionUpdate, models.ResourceStaff, models.SeverityHigh
	case "staff.reactivated":
		return models.ActionUpdate, models.ResourceStaff, models.SeverityMedium
	case "staff.role_changed":
		return models.ActionUpdate, models.ResourceStaff, models.SeverityHigh

	// Vendor events
	case "vendor.created":
		return models.ActionCreate, models.ResourceVendor, models.SeverityMedium
	case "vendor.updated":
		return models.ActionUpdate, models.ResourceVendor, models.SeverityLow
	case "vendor.approved":
		return models.ActionUpdate, models.ResourceVendor, models.SeverityMedium
	case "vendor.rejected":
		return models.ActionUpdate, models.ResourceVendor, models.SeverityMedium
	case "vendor.suspended":
		return models.ActionUpdate, models.ResourceVendor, models.SeverityHigh

	// Coupon events
	case "coupon.created":
		return models.ActionCreate, models.ResourceCoupon, models.SeverityLow
	case "coupon.applied":
		return models.ActionUpdate, models.ResourceCoupon, models.SeverityLow
	case "coupon.expired", "coupon.deleted":
		return models.ActionDelete, models.ResourceCoupon, models.SeverityLow

	// Approval events
	case "approval.requested":
		return models.ActionCreate, models.ResourceApproval, models.SeverityMedium
	case "approval.granted":
		return models.ActionApprove, models.ResourceApproval, models.SeverityMedium
	case "approval.rejected":
		return models.ActionReject, models.ResourceApproval, models.SeverityMedium
	case "approval.cancelled":
		return models.ActionDelete, models.ResourceApproval, models.SeverityLow
	case "approval.escalated":
		return models.ActionUpdate, models.ResourceApproval, models.SeverityHigh

	// Ticket events
	case "ticket.created":
		return models.ActionCreate, models.ResourceTicket, models.SeverityLow
	case "ticket.updated", "ticket.assigned", "ticket.status_changed":
		return models.ActionUpdate, models.ResourceTicket, models.SeverityLow
	case "ticket.resolved", "ticket.closed":
		return models.ActionUpdate, models.ResourceTicket, models.SeverityLow

	// Tenant events
	case "tenant.created":
		return models.ActionCreate, models.ResourceTenant, models.SeverityMedium
	case "tenant.activated":
		return models.ActionUpdate, models.ResourceTenant, models.SeverityMedium
	case "tenant.deactivated":
		return models.ActionUpdate, models.ResourceTenant, models.SeverityCritical
	case "tenant.settings_updated":
		return models.ActionUpdate, models.ResourceSettings, models.SeverityMedium

	// Gift card events
	case "gift_card.created":
		return models.ActionCreate, models.ResourceGiftCard, models.SeverityLow
	case "gift_card.activated", "gift_card.applied":
		return models.ActionUpdate, models.ResourceGiftCard, models.SeverityMedium
	case "gift_card.refunded":
		return models.ActionUpdate, models.ResourceGiftCard, models.SeverityHigh

	// Return events
	case "return.requested":
		return models.ActionCreate, models.ResourceReturn, models.SeverityMedium
	case "return.approved":
		return models.ActionApprove, models.ResourceReturn, models.SeverityMedium
	case "return.rejected":
		return models.ActionReject, models.ResourceReturn, models.SeverityMedium
	case "return.completed":
		return models.ActionUpdate, models.ResourceReturn, models.SeverityMedium

	// Review events
	case "review.created":
		return models.ActionCreate, models.ResourceReview, models.SeverityLow
	case "review.approved":
		return models.ActionApprove, models.ResourceReview, models.SeverityLow
	case "review.rejected":
		return models.ActionReject, models.ResourceReview, models.SeverityLow

	// Category events
	case "category.created":
		return models.ActionCreate, models.ResourceCategory, models.SeverityLow
	case "category.updated":
		return models.ActionUpdate, models.ResourceCategory, models.SeverityLow
	case "category.deleted":
		return models.ActionDelete, models.ResourceCategory, models.SeverityMedium

	// Shipping events
	case "shipping.shipment_created":
		return models.ActionCreate, models.ResourceShipment, models.SeverityLow
	case "shipping.shipment_updated":
		return models.ActionUpdate, models.ResourceShipment, models.SeverityLow
	case "shipping.shipped":
		return models.ActionUpdate, models.ResourceShipment, models.SeverityMedium
	case "shipping.delivered":
		return models.ActionUpdate, models.ResourceShipment, models.SeverityLow
	case "shipping.failed":
		return models.ActionUpdate, models.ResourceShipment, models.SeverityHigh
	case "shipping.rate_created":
		return models.ActionCreate, models.ResourceShippingRate, models.SeverityLow
	case "shipping.rate_updated":
		return models.ActionUpdate, models.ResourceShippingRate, models.SeverityLow
	case "shipping.rate_deleted":
		return models.ActionDelete, models.ResourceShippingRate, models.SeverityMedium

	default:
		// Generic mapping for unknown events
		return models.ActionOther, models.ResourceOther, models.SeverityLow
	}
}

// extractResourceInfo extracts resource ID and name from event data
func (c *DomainEventConsumer) extractResourceInfo(eventType string, data map[string]interface{}) (string, string) {
	var resourceID, resourceName string

	// Try common ID fields
	idFields := []string{"orderId", "orderNumber", "paymentId", "customerId", "productId", "staffId", "vendorId", "couponId", "ticketId", "giftCardId", "returnId", "reviewId", "approvalRequestId", "categoryId", "shipmentId", "rateId"}
	for _, field := range idFields {
		if id, ok := data[field].(string); ok && id != "" {
			resourceID = id
			break
		}
	}

	// Try common name fields
	nameFields := []string{"orderNumber", "customerName", "productName", "staffName", "vendorName", "couponCode", "ticketNumber", "giftCardCode", "rmaNumber", "categoryName", "trackingNumber", "rateName", "carrier"}
	for _, field := range nameFields {
		if name, ok := data[field].(string); ok && name != "" {
			resourceName = name
			break
		}
	}

	return resourceID, resourceName
}

// extractServiceName extracts the service name from the subject
func (c *DomainEventConsumer) extractServiceName(subject string) string {
	// Subject format: domain.action (e.g., order.created)
	// Return the domain as service name
	if len(subject) > 0 {
		for i, char := range subject {
			if char == '.' {
				return subject[:i] + "-service"
			}
		}
	}
	return "unknown-service"
}

// generateDescription generates a human-readable description
func (c *DomainEventConsumer) generateDescription(eventType string, data map[string]interface{}) string {
	switch eventType {
	case "order.created":
		if orderNum, ok := data["orderNumber"].(string); ok {
			return fmt.Sprintf("Order %s was created", orderNum)
		}
		return "New order was created"
	case "order.paid":
		if orderNum, ok := data["orderNumber"].(string); ok {
			return fmt.Sprintf("Payment received for order %s", orderNum)
		}
		return "Payment received for order"
	case "order.cancelled":
		if reason, ok := data["cancellationReason"].(string); ok {
			return fmt.Sprintf("Order cancelled: %s", reason)
		}
		return "Order was cancelled"
	case "payment.succeeded":
		if amount, ok := data["amount"].(float64); ok {
			currency, _ := data["currency"].(string)
			return fmt.Sprintf("Payment of %.2f %s succeeded", amount, currency)
		}
		return "Payment succeeded"
	case "payment.failed":
		if errMsg, ok := data["errorMessage"].(string); ok {
			return fmt.Sprintf("Payment failed: %s", errMsg)
		}
		return "Payment failed"
	case "auth.login_success":
		if email, ok := data["email"].(string); ok {
			return fmt.Sprintf("User %s logged in successfully", email)
		}
		return "User logged in successfully"
	case "auth.login_failed":
		if email, ok := data["email"].(string); ok {
			return fmt.Sprintf("Login failed for %s", email)
		}
		return "Login attempt failed"
	case "staff.role_changed":
		if oldRole, ok := data["oldRole"].(string); ok {
			newRole, _ := data["newRole"].(string)
			return fmt.Sprintf("Role changed from %s to %s", oldRole, newRole)
		}
		return "Staff role was changed"
	case "category.created":
		if name, ok := data["categoryName"].(string); ok {
			return fmt.Sprintf("Category '%s' was created", name)
		}
		return "New category was created"
	case "category.updated":
		if name, ok := data["categoryName"].(string); ok {
			return fmt.Sprintf("Category '%s' was updated", name)
		}
		return "Category was updated"
	case "category.deleted":
		if name, ok := data["categoryName"].(string); ok {
			return fmt.Sprintf("Category '%s' was deleted", name)
		}
		return "Category was deleted"
	case "shipping.shipment_created":
		if orderNum, ok := data["orderNumber"].(string); ok {
			return fmt.Sprintf("Shipment created for order %s", orderNum)
		}
		return "New shipment was created"
	case "shipping.shipped":
		if tracking, ok := data["trackingNumber"].(string); ok {
			carrier, _ := data["carrier"].(string)
			return fmt.Sprintf("Package shipped via %s (tracking: %s)", carrier, tracking)
		}
		return "Package was shipped"
	case "shipping.delivered":
		if tracking, ok := data["trackingNumber"].(string); ok {
			return fmt.Sprintf("Package delivered (tracking: %s)", tracking)
		}
		return "Package was delivered"
	case "shipping.failed":
		if errMsg, ok := data["errorMessage"].(string); ok {
			return fmt.Sprintf("Shipment failed: %s", errMsg)
		}
		return "Shipment failed"
	case "shipping.rate_created":
		if name, ok := data["rateName"].(string); ok {
			carrier, _ := data["carrier"].(string)
			return fmt.Sprintf("Shipping rate '%s' (%s) was created", name, carrier)
		}
		return "Shipping rate was created"
	case "shipping.rate_updated":
		if name, ok := data["rateName"].(string); ok {
			return fmt.Sprintf("Shipping rate '%s' was updated", name)
		}
		return "Shipping rate was updated"
	default:
		// Generate generic description from event type
		return fmt.Sprintf("Event: %s", eventType)
	}
}

// Stop stops the consumer
func (c *DomainEventConsumer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	close(c.stopCh)
	c.running = false

	if c.nc != nil {
		c.nc.Drain()
		c.nc.Close()
	}

	c.logger.Info("Domain event consumer stopped")
}

// IsRunning returns whether the consumer is running
func (c *DomainEventConsumer) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// GetStats returns consumer statistics
func (c *DomainEventConsumer) GetStats() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	return map[string]interface{}{
		"running":          c.running,
		"consumer_count":   len(c.consumers),
		"nats_connected":   c.nc != nil && c.nc.IsConnected(),
	}
}
