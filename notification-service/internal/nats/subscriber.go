package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/Tesseract-Nexus/go-shared/events"
	"notification-service/internal/models"
	"notification-service/internal/repository"
	"notification-service/internal/services"
	"notification-service/internal/template"
	"notification-service/internal/templates"
)

// NotificationCategory maps event types to preference categories
var eventCategoryMap = map[string]string{
	events.OrderCreated:       "orders",
	events.OrderConfirmed:     "orders",
	events.OrderShipped:       "orders",
	events.OrderDelivered:     "orders",
	events.OrderCancelled:     "orders",
	events.PaymentCaptured:    "orders",
	events.PaymentFailed:      "orders",
	events.CustomerRegistered: "marketing",
	events.CustomerCreated:    "marketing",
	events.PasswordReset:      "security",
	events.VerificationCode:   "security",
	events.ReviewCreated:      "orders", // Review notifications go with orders
	events.ReviewApproved:     "orders",
	events.ReviewRejected:     "orders",

	// Inventory events (admin alerts)
	events.InventoryLowStock:   "orders",
	events.InventoryOutOfStock: "orders",

	// Ticket events (support)
	events.TicketCreated:       "orders",
	events.TicketAssigned:      "orders",
	events.TicketStatusChanged: "orders",
	events.TicketResolved:      "orders",

	// Coupon events (marketing)
	events.CouponCreated: "marketing",
	events.CouponApplied: "orders",
	events.CouponExpired: "marketing",

	// Vendor events (admin)
	events.VendorCreated:   "orders",
	events.VendorApproved:  "orders",
	events.VendorRejected:  "orders",
	events.VendorSuspended: "orders",

	// Approval workflow events (admin)
	events.ApprovalRequested: "orders",
	events.ApprovalGranted:   "orders",
	events.ApprovalRejected:  "orders",
	events.ApprovalCancelled: "orders",
	events.ApprovalExpired:   "orders",
	events.ApprovalEscalated: "orders",

	// Domain events (admin/security)
	events.DomainAdded:           "security",
	events.DomainVerified:        "security",
	events.DomainSSLProvisioned:  "security",
	events.DomainActivated:       "security",
	events.DomainFailed:          "security",
	events.DomainRemoved:         "security",
	events.DomainMigrated:        "security",
	events.DomainSSLExpiringSoon: "security",
}

// Subscriber handles NATS event subscriptions for sending external notifications
type Subscriber struct {
	client        *Client
	notifRepo     repository.NotificationRepository
	templateRepo  repository.TemplateRepository
	prefRepo      repository.PreferenceRepository
	emailProvider services.Provider
	smsProvider   services.Provider
	pushProvider  services.Provider
	templateEng   *template.Engine
	subs          []*nats.Subscription
	// Configurable admin emails (from environment)
	adminEmail   string
	supportEmail string
	// Tenant client for dynamic URL construction
	tenantClient *services.TenantClient
}

// NewSubscriber creates a new NATS subscriber
func NewSubscriber(
	client *Client,
	notifRepo repository.NotificationRepository,
	templateRepo repository.TemplateRepository,
	prefRepo repository.PreferenceRepository,
	emailProvider services.Provider,
	smsProvider services.Provider,
	pushProvider services.Provider,
	adminEmail string,
	supportEmail string,
) *Subscriber {
	// Use defaults if not provided
	if adminEmail == "" {
		adminEmail = "admin@tesserix.app"
	}
	if supportEmail == "" {
		supportEmail = "support@tesserix.app"
	}
	return &Subscriber{
		client:        client,
		notifRepo:     notifRepo,
		templateRepo:  templateRepo,
		prefRepo:      prefRepo,
		emailProvider: emailProvider,
		smsProvider:   smsProvider,
		pushProvider:  pushProvider,
		templateEng:   template.NewEngine(),
		subs:          make([]*nats.Subscription, 0),
		adminEmail:    adminEmail,
		supportEmail:  supportEmail,
		tenantClient:  services.NewTenantClient(),
	}
}

// ensureStream creates a stream if it doesn't exist
// This makes notification-service resilient to startup ordering
func (s *Subscriber) ensureStream(js nats.JetStreamContext, name, subject, description string) error {
	// Check if stream exists
	_, err := js.StreamInfo(name)
	if err == nil {
		log.Printf("[NATS] Stream %s already exists", name)
		return nil
	}

	// Stream doesn't exist, create it
	if err == nats.ErrStreamNotFound {
		log.Printf("[NATS] Creating stream %s for subject %s", name, subject)
		_, err = js.AddStream(&nats.StreamConfig{
			Name:        name,
			Description: description,
			Subjects:    []string{subject},
			Storage:     nats.FileStorage,
			Retention:   nats.LimitsPolicy,
			MaxAge:      7 * 24 * time.Hour, // 7 days
			MaxMsgs:     100000,
			Discard:     nats.DiscardOld,
		})
		if err != nil && err != nats.ErrStreamNameAlreadyInUse {
			return fmt.Errorf("failed to create stream %s: %w", name, err)
		}
		log.Printf("[NATS] Stream %s created successfully", name)
		return nil
	}

	return fmt.Errorf("failed to get stream info for %s: %w", name, err)
}

// Start begins subscribing to all event streams
func (s *Subscriber) Start(ctx context.Context) error {
	js := s.client.JetStream()

	// Ensure all required streams exist before subscribing
	// This makes notification-service self-sufficient and resilient to startup ordering
	streams := []struct {
		name        string
		subject     string
		description string
	}{
		{"ORDER_EVENTS", "order.>", "Order lifecycle events"},
		{"PAYMENT_EVENTS", "payment.>", "Payment lifecycle events"},
		{"CUSTOMER_EVENTS", "customer.>", "Customer lifecycle events"},
		{"AUTH_EVENTS", "auth.>", "Authentication events"},
		{"REVIEW_EVENTS", "review.>", "Review lifecycle events"},
		{"INVENTORY_EVENTS", "inventory.>", "Inventory alert events"},
		{"TICKET_EVENTS", "ticket.>", "Support ticket events"},
		{"VENDOR_EVENTS", "vendor.>", "Vendor lifecycle events"},
		{"COUPON_EVENTS", "coupon.>", "Coupon lifecycle events"},
		{"TENANT_EVENTS", "tenant.>", "Tenant lifecycle events"},
		{"APPROVAL_EVENTS", "approval.>", "Approval workflow events"},
		{"DOMAIN_EVENTS", "domain.>", "Domain lifecycle events"},
	}

	for _, stream := range streams {
		if err := s.ensureStream(js, stream.name, stream.subject, stream.description); err != nil {
			log.Printf("[NATS] Warning: %v", err)
		}
	}

	// Subscribe to order events
	// NOTE: BindStream ensures consumer is created on the correct stream
	orderSub, err := js.QueueSubscribe(
		"order.>",
		"notification-service-workers",
		s.handleOrderEvent,
		nats.BindStream("ORDER_EVENTS"),
		nats.Durable("notification-service-orders"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to order events: %v", err)
	} else {
		s.subs = append(s.subs, orderSub)
		log.Println("[NATS] Subscribed to order.> events")
	}

	// Subscribe to payment events
	paymentSub, err := js.QueueSubscribe(
		"payment.>",
		"notification-service-workers",
		s.handlePaymentEvent,
		nats.BindStream("PAYMENT_EVENTS"),
		nats.Durable("notification-service-payments"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to payment events: %v", err)
	} else {
		s.subs = append(s.subs, paymentSub)
		log.Println("[NATS] Subscribed to payment.> events")
	}

	// Subscribe to customer events
	customerSub, err := js.QueueSubscribe(
		"customer.>",
		"notification-service-workers",
		s.handleCustomerEvent,
		nats.BindStream("CUSTOMER_EVENTS"),
		nats.Durable("notification-service-customers"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to customer events: %v", err)
	} else {
		s.subs = append(s.subs, customerSub)
		log.Println("[NATS] Subscribed to customer.> events")
	}

	// Subscribe to auth events (password reset, verification codes)
	authSub, err := js.QueueSubscribe(
		"auth.>",
		"notification-service-workers",
		s.handleAuthEvent,
		nats.BindStream("AUTH_EVENTS"),
		nats.Durable("notification-service-auth"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to auth events: %v", err)
	} else {
		s.subs = append(s.subs, authSub)
		log.Println("[NATS] Subscribed to auth.> events")
	}

	// Subscribe to review events
	reviewSub, err := js.QueueSubscribe(
		"review.>",
		"notification-service-workers",
		s.handleReviewEvent,
		nats.BindStream("REVIEW_EVENTS"),
		nats.Durable("notification-service-reviews"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to review events: %v", err)
	} else {
		s.subs = append(s.subs, reviewSub)
		log.Println("[NATS] Subscribed to review.> events")
	}

	// Subscribe to inventory events (low stock alerts)
	inventorySub, err := js.QueueSubscribe(
		"inventory.>",
		"notification-service-workers",
		s.handleInventoryEvent,
		nats.BindStream("INVENTORY_EVENTS"),
		nats.Durable("notification-service-inventory"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to inventory events: %v", err)
	} else {
		s.subs = append(s.subs, inventorySub)
		log.Println("[NATS] Subscribed to inventory.> events")
	}

	// Subscribe to ticket events (support tickets)
	ticketSub, err := js.QueueSubscribe(
		"ticket.>",
		"notification-service-workers",
		s.handleTicketEvent,
		nats.BindStream("TICKET_EVENTS"),
		nats.Durable("notification-service-tickets"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to ticket events: %v", err)
	} else {
		s.subs = append(s.subs, ticketSub)
		log.Println("[NATS] Subscribed to ticket.> events")
	}

	// Subscribe to vendor events
	vendorSub, err := js.QueueSubscribe(
		"vendor.>",
		"notification-service-workers",
		s.handleVendorEvent,
		nats.BindStream("VENDOR_EVENTS"),
		nats.Durable("notification-service-vendors"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to vendor events: %v", err)
	} else {
		s.subs = append(s.subs, vendorSub)
		log.Println("[NATS] Subscribed to vendor.> events")
	}

	// Subscribe to coupon events
	couponSub, err := js.QueueSubscribe(
		"coupon.>",
		"notification-service-workers",
		s.handleCouponEvent,
		nats.BindStream("COUPON_EVENTS"),
		nats.Durable("notification-service-coupons"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to coupon events: %v", err)
	} else {
		s.subs = append(s.subs, couponSub)
		log.Println("[NATS] Subscribed to coupon.> events")
	}

	// Subscribe to tenant events (onboarding welcome emails)
	// NOTE: Must explicitly bind to TENANT_EVENTS stream to ensure consumer is created
	tenantSub, err := js.QueueSubscribe(
		"tenant.>",
		"notification-service-workers",
		s.handleTenantEvent,
		nats.BindStream("TENANT_EVENTS"),
		nats.Durable("notification-service-tenants"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to tenant events: %v", err)
	} else {
		s.subs = append(s.subs, tenantSub)
		log.Println("[NATS] Subscribed to tenant.> events")
	}

	// Subscribe to approval events
	approvalSub, err := js.QueueSubscribe(
		"approval.>",
		"notification-service-workers",
		s.handleApprovalEvent,
		nats.BindStream("APPROVAL_EVENTS"),
		nats.Durable("notification-service-approvals"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to approval events: %v", err)
	} else {
		s.subs = append(s.subs, approvalSub)
		log.Println("[NATS] Subscribed to approval.> events")
	}

	// Subscribe to domain events (custom domain lifecycle)
	domainSub, err := js.QueueSubscribe(
		"domain.>",
		"notification-service-workers",
		s.handleDomainEvent,
		nats.BindStream("DOMAIN_EVENTS"),
		nats.Durable("notification-service-domains"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to domain events: %v", err)
	} else {
		s.subs = append(s.subs, domainSub)
		log.Println("[NATS] Subscribed to domain.> events")
	}

	log.Printf("[NATS] Subscriber started with %d subscriptions", len(s.subs))
	return nil
}

// Stop unsubscribes from all streams
func (s *Subscriber) Stop() {
	for _, sub := range s.subs {
		if err := sub.Drain(); err != nil {
			log.Printf("[NATS] Error draining subscription: %v", err)
		}
	}
	log.Println("[NATS] Subscriber stopped")
}

// handleOrderEvent processes order-related events
func (s *Subscriber) handleOrderEvent(msg *nats.Msg) {
	var event events.OrderEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal order event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing order event: %s for order %s", event.EventType, event.OrderNumber)

	ctx := context.Background()

	// Get user preferences if we have a customer ID
	var prefs *models.NotificationPreference
	if event.CustomerID != "" {
		if customerUUID, err := uuid.Parse(event.CustomerID); err == nil {
			prefs, _ = s.prefRepo.GetByUserID(ctx, event.TenantID, customerUUID)
		}
	}

	// Determine template based on event type
	templateName := s.getOrderTemplateName(event.EventType)
	if templateName == "" {
		log.Printf("[NATS] No template for event type: %s", event.EventType)
		msg.Ack()
		return
	}

	// Prepare template variables
	variables := map[string]interface{}{
		"orderNumber":       event.OrderNumber,
		"orderID":           event.OrderID,
		"orderDate":         event.OrderDate,
		"totalAmount":       event.TotalAmount,
		"subtotal":          event.Subtotal,
		"discount":          event.Discount,
		"shipping":          event.ShippingCost,
		"tax":               event.Tax,
		"currency":          event.Currency,
		"status":            event.Status,
		"customerName":      event.CustomerName,
		"customerEmail":     event.CustomerEmail,
		"paymentMethod":     event.PaymentMethod,
		"trackingUrl":       event.TrackingURL,
		"trackingNumber":    event.TrackingNumber,
		"carrierName":       event.CarrierName,
		"estimatedDelivery": event.EstimatedDelivery,
	}

	// Convert order items to template format
	if len(event.Items) > 0 {
		var items []map[string]interface{}
		for _, item := range event.Items {
			itemCurrency := item.Currency
			if itemCurrency == "" {
				itemCurrency = event.Currency
			}
			items = append(items, map[string]interface{}{
				"name":       item.Name,
				"sku":        item.SKU,
				"imageUrl":   item.ImageURL,
				"quantity":   item.Quantity,
				"price":      item.UnitPrice,
				"currency":   itemCurrency,
				"vendorId":   item.VendorID,
				"vendorName": item.VendorName,
			})
		}
		variables["items"] = items
	}

	// Add shipping address if provided
	if event.ShippingAddress != nil {
		variables["shippingName"] = event.ShippingAddress.Name
		variables["shippingLine1"] = event.ShippingAddress.Line1
		variables["shippingLine2"] = event.ShippingAddress.Line2
		variables["shippingCity"] = event.ShippingAddress.City
		variables["shippingState"] = event.ShippingAddress.State
		variables["shippingPostalCode"] = event.ShippingAddress.PostalCode
		variables["shippingCountry"] = event.ShippingAddress.Country
	}

	// Check preferences and send notifications
	category := eventCategoryMap[event.EventType]

	// Send email if enabled
	if s.shouldSendEmail(prefs, category) && event.CustomerEmail != "" {
		log.Printf("[EMAIL] Sending %s to %s", templateName, event.CustomerEmail)
		s.sendTemplatedEmail(ctx, event.TenantID, templateName, event.CustomerEmail, variables)
	}

	// Send SMS for important events (shipped, delivered) if enabled
	if s.shouldSendSMS(prefs, category) && event.CustomerPhone != "" {
		if event.EventType == events.OrderShipped || event.EventType == events.OrderDelivered {
			log.Printf("[SMS] Sending %s-sms to %s", templateName, event.CustomerPhone)
			s.sendTemplatedSMS(ctx, event.TenantID, templateName+"-sms", event.CustomerPhone, variables)
		}
	}

	msg.Ack()
	log.Printf("[NATS] Processed order event: %s for order %s", event.EventType, event.OrderNumber)
}

// handlePaymentEvent processes payment-related events
func (s *Subscriber) handlePaymentEvent(msg *nats.Msg) {
	var event events.PaymentEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal payment event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing payment event: %s for order %s", event.EventType, event.OrderNumber)

	ctx := context.Background()

	// Get user preferences
	var prefs *models.NotificationPreference
	if event.CustomerID != "" {
		if customerUUID, err := uuid.Parse(event.CustomerID); err == nil {
			prefs, _ = s.prefRepo.GetByUserID(ctx, event.TenantID, customerUUID)
		}
	}

	// Determine template based on payment event type
	var templateName string
	switch event.EventType {
	case events.PaymentCaptured:
		templateName = "payment-confirmation"
	case events.PaymentFailed:
		templateName = "payment-failed"
	case events.PaymentRefunded:
		templateName = "payment-refunded"
	default:
		log.Printf("[NATS] Unknown payment event type: %s", event.EventType)
		msg.Ack()
		return
	}

	variables := map[string]interface{}{
		"paymentID":     event.PaymentID,
		"orderID":       event.OrderID,
		"orderNumber":   event.OrderNumber,
		"amount":        event.Amount,
		"currency":      event.Currency,
		"provider":      event.Provider,
		"status":        event.Status,
		"errorMessage":  event.ErrorMessage,
		"customerName":  event.CustomerName,
		"customerEmail": event.CustomerEmail,
		// Refund-specific fields
		"refundId":     event.RefundID,
		"refundAmount": event.RefundAmount,
		"refundReason": event.RefundReason,
	}

	category := eventCategoryMap[event.EventType]

	// Send email if enabled
	if s.shouldSendEmail(prefs, category) && event.CustomerEmail != "" {
		log.Printf("[EMAIL] Sending %s to %s", templateName, event.CustomerEmail)
		s.sendTemplatedEmail(ctx, event.TenantID, templateName, event.CustomerEmail, variables)
	}

	// For failed payments, also send SMS if enabled
	if event.EventType == events.PaymentFailed {
		if s.shouldSendSMS(prefs, category) && event.CustomerPhone != "" {
			log.Printf("[SMS] Sending %s-sms to %s", templateName, event.CustomerPhone)
			s.sendTemplatedSMS(ctx, event.TenantID, templateName+"-sms", event.CustomerPhone, variables)
		}
	}

	msg.Ack()
	log.Printf("[NATS] Processed payment event: %s", event.EventType)
}

// handleCustomerEvent processes customer-related events
func (s *Subscriber) handleCustomerEvent(msg *nats.Msg) {
	var event events.CustomerEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal customer event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing customer event: %s for %s", event.EventType, event.CustomerEmail)

	ctx := context.Background()

	// For new customer registration, preferences might not exist yet
	// Send welcome email by default
	// Handle both customer.registered (from auth-service) and customer.created (from storefront/customers-service)
	if event.EventType == events.CustomerRegistered || event.EventType == events.CustomerCreated {
		variables := map[string]interface{}{
			"customerName":  event.CustomerName,
			"customerEmail": event.CustomerEmail,
		}

		if event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending welcome-email to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "welcome-email", event.CustomerEmail, variables)
		}
	}

	msg.Ack()
	log.Printf("[NATS] Processed customer event: %s", event.EventType)
}

// handleAuthEvent processes authentication-related events
func (s *Subscriber) handleAuthEvent(msg *nats.Msg) {
	var event events.AuthEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal auth event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing auth event: %s for %s", event.EventType, event.Email)

	ctx := context.Background()

	// Get tenant info for URLs
	tenantInfo, _ := s.tenantClient.GetTenantInfo(event.TenantID)
	businessName := tenantInfo.BusinessName
	if businessName == "" {
		businessName = tenantInfo.Name
	}

	// Security notifications are always sent (no preference check for password reset)
	switch event.EventType {
	case events.PasswordReset:
		variables := map[string]interface{}{
			"resetUrl":   event.ResetURL,
			"resetToken": event.ResetToken,
			"email":      event.Email,
		}

		if event.Email != "" {
			log.Printf("[EMAIL] Sending password-reset to %s", event.Email)
			s.sendTemplatedEmail(ctx, event.TenantID, "password-reset", event.Email, variables)
		}

	case events.VerificationCode:
		variables := map[string]interface{}{
			"verificationCode": event.VerificationCode,
			"email":            event.Email,
			"phone":            event.Phone,
		}

		// Send verification code via email
		if event.Email != "" {
			log.Printf("[EMAIL] Sending verification-code to %s", event.Email)
			s.sendTemplatedEmail(ctx, event.TenantID, "verification-code", event.Email, variables)
		}

		// Also send via SMS if phone provided
		if event.Phone != "" {
			log.Printf("[SMS] Sending verification-code-sms to %s", event.Phone)
			s.sendTemplatedSMS(ctx, event.TenantID, "verification-code-sms", event.Phone, variables)
		}

	case events.LoginSuccess:
		// Send login notification for security awareness
		// Parse user agent to get device info
		deviceInfo := formatUserAgent(event.UserAgent, event.DeviceType)

		// Format login time
		loginTime := event.Timestamp.Format("January 2, 2006 at 3:04 PM MST")

		// Build reset password URL
		resetPasswordURL := fmt.Sprintf("%s/forgot-password", tenantInfo.StorefrontURL)

		// Get location info - use Location field or default
		loginLocation := event.Location
		if loginLocation == "" {
			loginLocation = "Unknown location"
		}

		variables := map[string]interface{}{
			"customerName":     "", // Will be populated by template if available
			"customerEmail":    event.Email,
			"email":            event.Email,
			"loginTime":        loginTime,
			"loginLocation":    loginLocation,
			"ipAddress":        event.IPAddress,
			"deviceInfo":       deviceInfo,
			"userAgent":        event.UserAgent,
			"loginMethod":      event.LoginMethod,
			"businessName":     businessName,
			"resetPasswordURL": resetPasswordURL,
		}

		if event.Email != "" {
			log.Printf("[EMAIL] Sending login-notification to %s (IP: %s)", event.Email, event.IPAddress)
			s.sendTemplatedEmail(ctx, event.TenantID, "login-notification", event.Email, variables)
		}
	}

	msg.Ack()
	log.Printf("[NATS] Processed auth event: %s", event.EventType)
}

// handleReviewEvent processes review-related events
func (s *Subscriber) handleReviewEvent(msg *nats.Msg) {
	var event events.ReviewEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal review event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing review event: %s for product %s", event.EventType, event.ProductName)

	ctx := context.Background()

	// Get user preferences if we have a customer ID
	var prefs *models.NotificationPreference
	if event.CustomerID != "" {
		if customerUUID, err := uuid.Parse(event.CustomerID); err == nil {
			prefs, _ = s.prefRepo.GetByUserID(ctx, event.TenantID, customerUUID)
		}
	}

	// Get tenant-specific URLs
	tenantInfo, _ := s.tenantClient.GetTenantInfo(event.TenantID)
	reviewsURL := fmt.Sprintf("%s/reviews", tenantInfo.AdminURL)
	productURL := fmt.Sprintf("%s/products/%s", tenantInfo.StorefrontURL, event.ProductID)
	businessName := tenantInfo.BusinessName
	if businessName == "" {
		businessName = tenantInfo.Name
	}

	// Prepare template variables
	variables := map[string]interface{}{
		"reviewId":      event.ReviewID,
		"productId":     event.ProductID,
		"productName":   event.ProductName,
		"productSku":    event.ProductSKU,
		"rating":        event.Rating,
		"maxRating":     5,
		"reviewTitle":   event.Title,
		"reviewContent": event.Content,
		"reviewStatus":  event.Status,
		"isVerified":    event.Verified,
		"customerName":  event.CustomerName,
		"customerEmail": event.CustomerEmail,
		"moderatedBy":   event.ModeratedBy,
		"moderatedAt":   event.ModeratedAt,
		"rejectReason":  event.RejectReason,
		"businessName":  businessName,
		"reviewsUrl":    reviewsURL,
		"productUrl":    productURL,
	}

	category := eventCategoryMap[event.EventType]

	switch event.EventType {
	case events.ReviewCreated:
		// Send customer copy of their review
		if s.shouldSendEmail(prefs, category) && event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending review-submitted-customer to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "review-submitted-customer", event.CustomerEmail, variables)
		}

		// Send admin notification (always for new reviews)
		log.Printf("[EMAIL] Sending review-submitted-admin to %s", s.adminEmail)
		s.sendTemplatedEmail(ctx, event.TenantID, "review-submitted-admin", s.adminEmail, variables)

	case events.ReviewApproved:
		// Notify customer their review is published
		if s.shouldSendEmail(prefs, category) && event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending review-approved to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "review-approved", event.CustomerEmail, variables)
		}

	case events.ReviewRejected:
		// Notify customer their review was rejected
		if s.shouldSendEmail(prefs, category) && event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending review-rejected to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "review-rejected", event.CustomerEmail, variables)
		}
	}

	msg.Ack()
	log.Printf("[NATS] Processed review event: %s for review %s", event.EventType, event.ReviewID)
}

// handleInventoryEvent processes inventory-related events (low stock alerts)
func (s *Subscriber) handleInventoryEvent(msg *nats.Msg) {
	var event events.InventoryEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal inventory event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing inventory event: %s with %d items", event.EventType, len(event.Items))

	ctx := context.Background()

	// Inventory alerts are always sent to admin (not customer preference based)

	// Build product list for template
	var products []map[string]interface{}
	for _, item := range event.Items {
		products = append(products, map[string]interface{}{
			"name":       item.Name,
			"sku":        item.SKU,
			"imageUrl":   item.ImageURL,
			"stockLevel": item.CurrentStock,
		})
	}

	variables := map[string]interface{}{
		"products":              products,
		"totalLowStockItems":    event.TotalLowStock,
		"totalOutOfStockItems":  event.TotalOutOfStock,
		"totalAffectedProducts": event.TotalAffected,
		"alertLevel":            event.AlertLevel,
		"inventoryUrl":          event.InventoryURL,
	}

	switch event.EventType {
	case events.InventoryLowStock, events.InventoryOutOfStock:
		log.Printf("[EMAIL] Sending low-stock-alert to %s", s.adminEmail)
		s.sendTemplatedEmail(ctx, event.TenantID, "low-stock-alert", s.adminEmail, variables)
	}

	msg.Ack()
	log.Printf("[NATS] Processed inventory event: %s", event.EventType)
}

// handleTicketEvent processes support ticket events
func (s *Subscriber) handleTicketEvent(msg *nats.Msg) {
	var event events.TicketEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal ticket event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing ticket event: %s for ticket %s", event.EventType, event.TicketID)

	ctx := context.Background()

	// Get tenant-specific URL
	ticketURL := s.tenantClient.BuildTicketURL(event.TenantID, event.TicketID)

	variables := map[string]interface{}{
		"ticketId":       event.TicketID,
		"ticketNumber":   event.TicketNumber,
		"subject":        event.Subject,
		"description":    event.Description,
		"category":       event.Category,
		"priority":       event.Priority,
		"status":         event.Status,
		"customerName":   event.CustomerName,
		"customerEmail":  event.CustomerEmail,
		"assignedTo":     event.AssignedTo,
		"assignedToName": event.AssignedToName,
		"resolution":     event.Resolution,
		"ticketUrl":      ticketURL,
	}

	switch event.EventType {
	case events.TicketCreated:
		// Send confirmation to customer
		if event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending ticket-created to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "ticket-created", event.CustomerEmail, variables)
		}
		// Notify support team
		log.Printf("[EMAIL] Sending ticket-created-admin to %s", s.supportEmail)
		s.sendTemplatedEmail(ctx, event.TenantID, "ticket-created-admin", s.supportEmail, variables)

	case events.TicketAssigned:
		// Notify assigned agent
		if event.AssignedTo != "" {
			// TODO: Get agent email from staff service
			log.Printf("[NATS] Ticket %s assigned to %s", event.TicketID, event.AssignedToName)
		}

	case events.TicketStatusChanged:
		// Notify customer of status change
		if event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending ticket-updated to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "ticket-updated", event.CustomerEmail, variables)
		}

	case events.TicketResolved:
		// Notify customer of resolution
		if event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending ticket-resolved to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "ticket-resolved", event.CustomerEmail, variables)
		}

	case events.TicketClosed:
		// Notify customer with satisfaction survey
		if event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending ticket-closed to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "ticket-closed", event.CustomerEmail, variables)
		}
	}

	msg.Ack()
	log.Printf("[NATS] Processed ticket event: %s for ticket %s", event.EventType, event.TicketID)
}

// handleVendorEvent processes vendor-related events
func (s *Subscriber) handleVendorEvent(msg *nats.Msg) {
	var event events.VendorEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal vendor event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing vendor event: %s for vendor %s", event.EventType, event.VendorName)

	ctx := context.Background()

	// Get tenant-specific URL
	tenantInfo, _ := s.tenantClient.GetTenantInfo(event.TenantID)
	vendorURL := fmt.Sprintf("%s/vendors/%s", tenantInfo.AdminURL, event.VendorID)

	variables := map[string]interface{}{
		"vendorId":       event.VendorID,
		"vendorName":     event.VendorName,
		"vendorEmail":    event.VendorEmail,
		"businessName":   event.BusinessName,
		"status":         event.Status,
		"previousStatus": event.PreviousStatus,
		"statusReason":   event.StatusReason,
		"rejectReason":   event.RejectReason,
		"reviewedBy":     event.ReviewedBy,
		"vendorUrl":      vendorURL,
	}

	switch event.EventType {
	case events.VendorCreated:
		// Notify admin of new vendor application
		log.Printf("[EMAIL] Sending vendor-application to %s", s.adminEmail)
		s.sendTemplatedEmail(ctx, event.TenantID, "vendor-application", s.adminEmail, variables)

		// Send welcome email to vendor
		if event.VendorEmail != "" {
			log.Printf("[EMAIL] Sending vendor-welcome to %s", event.VendorEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "vendor-welcome", event.VendorEmail, variables)
		}

	case events.VendorApproved:
		// Notify vendor of approval
		if event.VendorEmail != "" {
			log.Printf("[EMAIL] Sending vendor-approved to %s", event.VendorEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "vendor-approved", event.VendorEmail, variables)
		}

	case events.VendorRejected:
		// Notify vendor of rejection
		if event.VendorEmail != "" {
			log.Printf("[EMAIL] Sending vendor-rejected to %s", event.VendorEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "vendor-rejected", event.VendorEmail, variables)
		}

	case events.VendorSuspended:
		// Notify vendor of suspension
		if event.VendorEmail != "" {
			log.Printf("[EMAIL] Sending vendor-suspended to %s", event.VendorEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "vendor-suspended", event.VendorEmail, variables)
		}
	}

	msg.Ack()
	log.Printf("[NATS] Processed vendor event: %s for vendor %s", event.EventType, event.VendorName)
}

// handleCouponEvent processes coupon-related events
func (s *Subscriber) handleCouponEvent(msg *nats.Msg) {
	var event events.CouponEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal coupon event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing coupon event: %s for coupon %s", event.EventType, event.CouponCode)

	ctx := context.Background()

	// Get user preferences if we have a customer ID
	var prefs *models.NotificationPreference
	if event.CustomerID != "" {
		if customerUUID, err := uuid.Parse(event.CustomerID); err == nil {
			prefs, _ = s.prefRepo.GetByUserID(ctx, event.TenantID, customerUUID)
		}
	}

	variables := map[string]interface{}{
		"couponId":       event.CouponID,
		"couponCode":     event.CouponCode,
		"discountType":   event.DiscountType,
		"discountValue":  event.DiscountValue,
		"discountAmount": event.DiscountAmount,
		"orderNumber":    event.OrderNumber,
		"orderValue":     event.OrderValue,
		"customerName":   event.CustomerName,
		"customerEmail":  event.CustomerEmail,
		"validFrom":      event.ValidFrom,
		"validUntil":     event.ValidUntil,
		"status":         event.Status,
		"currency":       event.Currency,
	}

	category := eventCategoryMap[event.EventType]

	switch event.EventType {
	case events.CouponApplied:
		// Send confirmation to customer that coupon was applied
		if s.shouldSendEmail(prefs, category) && event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending coupon-applied to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "coupon-applied", event.CustomerEmail, variables)
		}

	case events.CouponExpired:
		// Admin notification about expired coupons
		log.Printf("[EMAIL] Sending coupon-expired to %s", s.adminEmail)
		s.sendTemplatedEmail(ctx, event.TenantID, "coupon-expired", s.adminEmail, variables)

	case events.CouponCreated:
		// Could be used for marketing campaigns
		if s.shouldSendEmail(prefs, category) && event.CustomerEmail != "" {
			log.Printf("[EMAIL] Sending coupon-created to %s", event.CustomerEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "coupon-created", event.CustomerEmail, variables)
		}
	}

	msg.Ack()
	log.Printf("[NATS] Processed coupon event: %s for coupon %s", event.EventType, event.CouponCode)
}

// TenantEventPayload matches the event structure from tenant-service
// This is different from go-shared/events.TenantEvent as it includes onboarding-specific fields
type TenantEventPayload struct {
	EventType      string `json:"event_type"`
	TenantID       string `json:"tenant_id"`
	SessionID      string `json:"session_id"`
	Product        string `json:"product"`
	BusinessName   string `json:"business_name"`
	Slug           string `json:"slug"`
	Email          string `json:"email"`
	AdminHost      string `json:"admin_host"`
	StorefrontHost string `json:"storefront_host"`
	BaseDomain     string `json:"base_domain"`
	Timestamp      string `json:"timestamp"`

	// Verification fields (for tenant.verification.requested events)
	VerificationToken  string `json:"verification_token,omitempty"`
	VerificationLink   string `json:"verification_link,omitempty"`
	VerificationExpiry string `json:"verification_expiry,omitempty"`
}

// handleTenantEvent processes tenant-related events (verification emails, welcome pack)
func (s *Subscriber) handleTenantEvent(msg *nats.Msg) {
	var event TenantEventPayload
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal tenant event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing tenant event: %s for tenant %s (%s)", event.EventType, event.TenantID, event.BusinessName)

	ctx := context.Background()

	// Validate required fields
	if event.Email == "" {
		log.Printf("[NATS] Skipping tenant event: no email address provided")
		msg.Ack()
		return
	}

	// Prepare common template variables
	variables := map[string]interface{}{
		"tenantId":       event.TenantID,
		"sessionId":      event.SessionID,
		"businessName":   event.BusinessName,
		"slug":           event.Slug,
		"email":          event.Email,
		"product":        event.Product,
		"adminHost":      event.AdminHost,
		"storefrontHost": event.StorefrontHost,
		"baseDomain":     event.BaseDomain,
		// Build full URLs for the email
		"adminUrl":      fmt.Sprintf("https://%s", event.AdminHost),
		"storefrontUrl": fmt.Sprintf("https://%s", event.StorefrontHost),
	}

	switch event.EventType {
	case events.TenantVerificationRequested:
		// Email verification for new tenant onboarding
		if event.VerificationLink == "" {
			log.Printf("[NATS] Skipping verification event: no verification link provided")
			msg.Ack()
			return
		}

		variables["verificationLink"] = event.VerificationLink
		variables["verificationToken"] = event.VerificationToken
		variables["verificationExpiry"] = event.VerificationExpiry

		log.Printf("[EMAIL] Sending verification-link to %s for %s", event.Email, event.BusinessName)
		s.sendTemplatedEmail(ctx, event.TenantID, "verification-link", event.Email, variables)
		log.Printf("[NATS] Processed tenant.verification.requested event for %s (%s)", event.BusinessName, event.TenantID)

	case events.TenantOnboardingCompleted, events.TenantCreated:
		// Send welcome pack email after onboarding is complete
		log.Printf("[EMAIL] Sending tenant-welcome-pack to %s for %s", event.Email, event.BusinessName)
		s.sendTemplatedEmail(ctx, event.TenantID, "tenant-welcome-pack", event.Email, variables)
		log.Printf("[NATS] Processed %s event for %s (%s)", event.EventType, event.BusinessName, event.TenantID)

	default:
		log.Printf("[NATS] Skipping unhandled tenant event type: %s", event.EventType)
	}

	msg.Ack()
}

// handleApprovalEvent processes approval workflow events
func (s *Subscriber) handleApprovalEvent(msg *nats.Msg) {
	var event events.ApprovalEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal approval event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing approval event: %s for request %s", event.EventType, event.ApprovalRequestID)

	ctx := context.Background()

	// Get tenant-specific URL
	tenantInfo, _ := s.tenantClient.GetTenantInfo(event.TenantID)
	approvalURL := fmt.Sprintf("%s/approvals/%s", tenantInfo.AdminURL, event.ApprovalRequestID)
	businessName := tenantInfo.BusinessName
	if businessName == "" {
		businessName = tenantInfo.Name
	}

	// Prepare common template variables
	variables := map[string]interface{}{
		"approvalId":        event.ApprovalRequestID,
		"approvalStatus":    event.Status,
		"approvalPriority":  event.Priority,
		"actionType":        event.ActionType,
		"actionTypeDisplay": formatActionTypeForDisplay(event.ActionType),
		"resourceType":      event.ResourceType,
		"resourceId":        event.ResourceID,
		"requesterId":       event.RequesterID,
		"requesterName":     event.RequesterName,
		"requesterEmail":    event.RequesterEmail,
		"approverId":        event.ApproverID,
		"approverName":      event.ApproverName,
		"approverEmail":     event.ApproverEmail,
		"approverRole":      event.ApproverRole,
		"approvalReason":    event.DecisionReason,
		"approvalComment":   event.DecisionNotes,
		"approvalExpiresAt": event.ExpiresAt,
		"approvalCreatedAt": event.RequestedAt,
		"approvalDecidedAt": event.DecisionAt,
		"approvalUrl":       approvalURL,
		"businessName":      businessName,
		"escalatedFromId":   event.EscalatedFrom,
		"escalatedFromName": event.EscalatedTo,
		"escalationLevel":   event.EscalationLevel,
	}

	switch event.EventType {
	case events.ApprovalRequested:
		// Send notification to approver(s)
		if event.ApproverEmail != "" {
			log.Printf("[EMAIL] Sending approval-request to approver %s", event.ApproverEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "approval-request", event.ApproverEmail, variables)
		}

	case events.ApprovalEscalated:
		// Send escalation notification to new approver
		variables["approvalStatus"] = "ESCALATED"
		if event.ApproverEmail != "" {
			log.Printf("[EMAIL] Sending approval-escalated to approver %s", event.ApproverEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "approval-escalated", event.ApproverEmail, variables)
		}

	case events.ApprovalGranted:
		// Notify requester that their request was approved
		variables["approvalStatus"] = "APPROVED"
		if event.RequesterEmail != "" {
			log.Printf("[EMAIL] Sending approval-granted to requester %s", event.RequesterEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "approval-granted", event.RequesterEmail, variables)
		}

	case events.ApprovalRejected:
		// Notify requester that their request was rejected
		variables["approvalStatus"] = "REJECTED"
		if event.RequesterEmail != "" {
			log.Printf("[EMAIL] Sending approval-rejected to requester %s", event.RequesterEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "approval-rejected", event.RequesterEmail, variables)
		}

	case events.ApprovalCancelled:
		// Notify requester that their request was cancelled
		variables["approvalStatus"] = "CANCELLED"
		if event.RequesterEmail != "" {
			log.Printf("[EMAIL] Sending approval-cancelled to requester %s", event.RequesterEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "approval-cancelled", event.RequesterEmail, variables)
		}

	case events.ApprovalExpired:
		// Notify requester that their request expired
		variables["approvalStatus"] = "EXPIRED"
		if event.RequesterEmail != "" {
			log.Printf("[EMAIL] Sending approval-expired to requester %s", event.RequesterEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "approval-expired", event.RequesterEmail, variables)
		}

	default:
		log.Printf("[NATS] Skipping unhandled approval event type: %s", event.EventType)
	}

	msg.Ack()
	log.Printf("[NATS] Processed approval event: %s for request %s", event.EventType, event.ApprovalRequestID)
}

// handleDomainEvent processes custom domain lifecycle events
func (s *Subscriber) handleDomainEvent(msg *nats.Msg) {
	var event events.DomainEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal domain event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing domain event: %s for domain %s", event.EventType, event.Domain)

	ctx := context.Background()

	// Validate required fields
	if event.OwnerEmail == "" {
		log.Printf("[NATS] Skipping domain event: no owner email provided")
		msg.Ack()
		return
	}

	// Get tenant-specific URLs
	tenantInfo, _ := s.tenantClient.GetTenantInfo(event.TenantID)
	businessName := tenantInfo.BusinessName
	if businessName == "" {
		businessName = tenantInfo.Name
	}

	// Build domain management URL
	domainSettingsURL := fmt.Sprintf("%s/settings/domains", tenantInfo.AdminURL)
	if event.DomainID != "" {
		domainSettingsURL = fmt.Sprintf("%s/settings/domains/%s", tenantInfo.AdminURL, event.DomainID)
	}

	// Prepare common template variables
	variables := map[string]interface{}{
		"domainId":          event.DomainID,
		"domain":            event.Domain,
		"domainType":        event.DomainType,
		"tenantId":          event.TenantID,
		"tenantSlug":        event.TenantSlug,
		"ownerEmail":        event.OwnerEmail,
		"ownerName":         event.OwnerName,
		"status":            event.Status,
		"previousStatus":    event.PreviousStatus,
		"verificationToken": event.VerificationToken,
		"dnsRecordType":     event.DNSRecordType,
		"dnsRecordName":     event.DNSRecordName,
		"dnsRecordValue":    event.DNSRecordValue,
		"sslStatus":         event.SSLStatus,
		"sslExpiresAt":      event.SSLExpiresAt,
		"sslProvider":       event.SSLProvider,
		"routingTarget":     event.RoutingTarget,
		"routingPath":       event.RoutingPath,
		"migratedFrom":      event.MigratedFrom,
		"migratedTo":        event.MigratedTo,
		"migrationReason":   event.MigrationReason,
		"failureReason":     event.FailureReason,
		"failureCode":       event.FailureCode,
		"businessName":      businessName,
		"domainSettingsUrl": domainSettingsURL,
		"adminUrl":          tenantInfo.AdminURL,
		"storefrontUrl":     tenantInfo.StorefrontURL,
	}

	switch event.EventType {
	case events.DomainAdded:
		// Send notification that domain was added and verification is pending
		log.Printf("[EMAIL] Sending domain-added to %s for %s", event.OwnerEmail, event.Domain)
		s.sendTemplatedEmail(ctx, event.TenantID, "domain-added", event.OwnerEmail, variables)

	case events.DomainVerified:
		// Send notification that domain DNS verification succeeded
		log.Printf("[EMAIL] Sending domain-verified to %s for %s", event.OwnerEmail, event.Domain)
		s.sendTemplatedEmail(ctx, event.TenantID, "domain-verified", event.OwnerEmail, variables)

	case events.DomainSSLProvisioned:
		// Send notification that SSL certificate was provisioned
		log.Printf("[EMAIL] Sending domain-ssl-ready to %s for %s", event.OwnerEmail, event.Domain)
		s.sendTemplatedEmail(ctx, event.TenantID, "domain-ssl-ready", event.OwnerEmail, variables)

	case events.DomainActivated:
		// Send notification that domain is now fully active and live
		log.Printf("[EMAIL] Sending domain-activated to %s for %s", event.OwnerEmail, event.Domain)
		s.sendTemplatedEmail(ctx, event.TenantID, "domain-activated", event.OwnerEmail, variables)

	case events.DomainFailed:
		// Send notification about domain setup failure with actionable info
		log.Printf("[EMAIL] Sending domain-failed to %s for %s (reason: %s)", event.OwnerEmail, event.Domain, event.FailureReason)
		s.sendTemplatedEmail(ctx, event.TenantID, "domain-failed", event.OwnerEmail, variables)

	case events.DomainRemoved:
		// Send confirmation that domain was removed
		log.Printf("[EMAIL] Sending domain-removed to %s for %s", event.OwnerEmail, event.Domain)
		s.sendTemplatedEmail(ctx, event.TenantID, "domain-removed", event.OwnerEmail, variables)

	case events.DomainMigrated:
		// Send notification that domain was migrated to new infrastructure
		log.Printf("[EMAIL] Sending domain-migrated to %s for %s", event.OwnerEmail, event.Domain)
		s.sendTemplatedEmail(ctx, event.TenantID, "domain-migrated", event.OwnerEmail, variables)

	case events.DomainSSLExpiringSoon:
		// Send warning about SSL certificate expiring soon
		log.Printf("[EMAIL] Sending domain-ssl-expiring to %s for %s", event.OwnerEmail, event.Domain)
		s.sendTemplatedEmail(ctx, event.TenantID, "domain-ssl-expiring", event.OwnerEmail, variables)

	case events.DomainHealthCheckFailed:
		// Send alert about domain health check failure
		log.Printf("[EMAIL] Sending domain-health-failed to %s for %s", event.OwnerEmail, event.Domain)
		s.sendTemplatedEmail(ctx, event.TenantID, "domain-health-failed", event.OwnerEmail, variables)

	default:
		log.Printf("[NATS] Skipping unhandled domain event type: %s", event.EventType)
	}

	msg.Ack()
	log.Printf("[NATS] Processed domain event: %s for domain %s", event.EventType, event.Domain)
}

// formatActionTypeForDisplay converts action_type to human-readable format
func formatActionTypeForDisplay(actionType string) string {
	if actionType == "" {
		return "Action"
	}
	// Replace underscores with spaces
	display := actionType
	for i := 0; i < len(display); i++ {
		if display[i] == '_' {
			display = display[:i] + " " + display[i+1:]
		}
	}
	// Title case first letter of each word
	words := []byte(display)
	capNext := true
	for i := 0; i < len(words); i++ {
		if words[i] == ' ' {
			capNext = true
		} else if capNext && words[i] >= 'a' && words[i] <= 'z' {
			words[i] = words[i] - 32 // Convert to uppercase
			capNext = false
		} else {
			capNext = false
		}
	}
	return string(words)
}

// shouldSendEmail checks if email should be sent based on preferences
func (s *Subscriber) shouldSendEmail(prefs *models.NotificationPreference, category string) bool {
	// If no preferences, default to enabled
	if prefs == nil {
		return true
	}

	// Check if email is globally enabled
	if !prefs.EmailEnabled {
		return false
	}

	// Check category-specific preferences
	switch category {
	case "orders":
		return prefs.OrdersEnabled
	case "marketing":
		return prefs.MarketingEnabled
	case "security":
		return prefs.SecurityEnabled
	default:
		return true
	}
}

// shouldSendSMS checks if SMS should be sent based on preferences
func (s *Subscriber) shouldSendSMS(prefs *models.NotificationPreference, category string) bool {
	// If no preferences, default to enabled
	if prefs == nil {
		return true
	}

	// Check if SMS is globally enabled
	if !prefs.SMSEnabled {
		return false
	}

	// Check category-specific preferences
	switch category {
	case "orders":
		return prefs.OrdersEnabled
	case "marketing":
		return prefs.MarketingEnabled
	case "security":
		return prefs.SecurityEnabled
	default:
		return true
	}
}

// shouldSendPush checks if push should be sent based on preferences
func (s *Subscriber) shouldSendPush(prefs *models.NotificationPreference, category string) bool {
	// If no preferences, default to enabled
	if prefs == nil {
		return true
	}

	// Check if push is globally enabled
	if !prefs.PushEnabled {
		return false
	}

	// Check category-specific preferences
	switch category {
	case "orders":
		return prefs.OrdersEnabled
	case "marketing":
		return prefs.MarketingEnabled
	case "security":
		return prefs.SecurityEnabled
	default:
		return true
	}
}

func (s *Subscriber) getOrderTemplateName(eventType string) string {
	switch eventType {
	case events.OrderCreated, events.OrderConfirmed:
		return "order-confirmation"
	case events.OrderShipped:
		return "order-shipped"
	case events.OrderDelivered:
		return "order-delivered"
	case events.OrderCancelled:
		return "order-cancelled"
	default:
		return ""
	}
}

func (s *Subscriber) sendTemplatedEmail(ctx context.Context, tenantID, templateName, recipient string, variables map[string]interface{}) {
	if s.emailProvider == nil {
		log.Println("[EMAIL] Provider not configured, skipping")
		return
	}

	var subject, body string
	var tmplID *uuid.UUID

	// Get template (try tenant-specific first, then system)
	tmpl, err := s.templateRepo.GetByName(ctx, tenantID, templateName)
	if err != nil || tmpl == nil {
		tmpl, err = s.templateRepo.GetByName(ctx, "system", templateName)
	}

	if tmpl != nil {
		tmplID = &tmpl.ID

		// Render subject
		subject = tmpl.Subject
		if subject != "" {
			rendered, err := s.templateEng.RenderText(subject, variables)
			if err != nil {
				log.Printf("[EMAIL] Failed to render subject: %v", err)
			} else {
				subject = rendered
			}
		}

		// Render body
		if tmpl.HTMLTemplate != "" {
			rendered, err := s.templateEng.RenderHTML(tmpl.HTMLTemplate, variables)
			if err != nil {
				log.Printf("[EMAIL] Failed to render HTML template: %v", err)
			} else {
				body = rendered
			}
		} else if tmpl.BodyTemplate != "" {
			rendered, err := s.templateEng.RenderText(tmpl.BodyTemplate, variables)
			if err != nil {
				log.Printf("[EMAIL] Failed to render body template: %v", err)
			} else {
				body = rendered
			}
		}
	} else {
		// Fallback to embedded templates
		log.Printf("[EMAIL] Using embedded template for: %s", templateName)
		subject, body, err = s.renderEmbeddedTemplate(templateName, variables)
		if err != nil {
			log.Printf("[EMAIL] Embedded template error: %v", err)
			return
		}
	}

	// Create notification record for tracking
	notification := &models.Notification{
		TenantID:       tenantID,
		Channel:        models.ChannelEmail,
		Status:         models.StatusPending,
		Priority:       models.PriorityNormal,
		TemplateID:     tmplID,
		TemplateName:   templateName,
		RecipientEmail: recipient,
		Subject:        subject,
		BodyHTML:       body,
	}

	if jsonVars, err := json.Marshal(variables); err == nil {
		notification.Variables = jsonVars
	}

	if err := s.notifRepo.Create(ctx, notification); err != nil {
		log.Printf("[EMAIL] Failed to create notification record: %v", err)
		return
	}

	// Update status to sending
	s.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusSending, "", "")

	// Send the email
	message := &services.Message{
		To:       recipient,
		Subject:  subject,
		BodyHTML: body,
	}

	result, err := s.emailProvider.Send(ctx, message)
	if err != nil {
		s.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", err.Error())
		log.Printf("[EMAIL] Failed to send to %s: %v", recipient, err)
		return
	}

	if result.Success {
		s.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusSent, result.ProviderID, "")
		log.Printf("[EMAIL] Successfully sent to %s (provider_id: %s)", recipient, result.ProviderID)
	} else {
		errorMsg := "Send failed"
		if result.Error != nil {
			errorMsg = result.Error.Error()
		}
		s.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", errorMsg)
		log.Printf("[EMAIL] Failed to send to %s: %s", recipient, errorMsg)
	}
}

func (s *Subscriber) sendTemplatedSMS(ctx context.Context, tenantID, templateName, recipient string, variables map[string]interface{}) {
	if s.smsProvider == nil {
		log.Println("[SMS] Provider not configured, skipping")
		return
	}

	// Get template
	tmpl, err := s.templateRepo.GetByName(ctx, tenantID, templateName)
	if err != nil || tmpl == nil {
		tmpl, err = s.templateRepo.GetByName(ctx, "system", templateName)
		if err != nil || tmpl == nil {
			log.Printf("[SMS] Template not found: %s", templateName)
			return
		}
	}

	// Render body
	body := ""
	if tmpl.BodyTemplate != "" {
		rendered, err := s.templateEng.RenderText(tmpl.BodyTemplate, variables)
		if err != nil {
			log.Printf("[SMS] Failed to render body template: %v", err)
		} else {
			body = rendered
		}
	}

	// Create notification record
	notification := &models.Notification{
		TenantID:       tenantID,
		Channel:        models.ChannelSMS,
		Status:         models.StatusPending,
		Priority:       models.PriorityNormal,
		TemplateID:     &tmpl.ID,
		TemplateName:   templateName,
		RecipientPhone: recipient,
		Body:           body,
	}

	if jsonVars, err := json.Marshal(variables); err == nil {
		notification.Variables = jsonVars
	}

	if err := s.notifRepo.Create(ctx, notification); err != nil {
		log.Printf("[SMS] Failed to create notification record: %v", err)
		return
	}

	// Update status to sending
	s.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusSending, "", "")

	// Send the SMS
	message := &services.Message{
		To:   recipient,
		Body: body,
	}

	result, err := s.smsProvider.Send(ctx, message)
	if err != nil {
		s.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", err.Error())
		log.Printf("[SMS] Failed to send to %s: %v", recipient, err)
		return
	}

	if result.Success {
		s.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusSent, result.ProviderID, "")
		log.Printf("[SMS] Successfully sent to %s (provider_id: %s)", recipient, result.ProviderID)
	} else {
		errorMsg := "Send failed"
		if result.Error != nil {
			errorMsg = result.Error.Error()
		}
		s.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", errorMsg)
		log.Printf("[SMS] Failed to send to %s: %s", recipient, errorMsg)
	}
}

func (s *Subscriber) sendPushNotification(ctx context.Context, tenantID string, userID uuid.UUID, title, body string, data map[string]interface{}) {
	if s.pushProvider == nil {
		log.Println("[PUSH] Provider not configured, skipping")
		return
	}

	// Get user's push tokens from preferences
	prefs, err := s.prefRepo.GetByUserID(ctx, tenantID, userID)
	if err != nil || prefs == nil {
		log.Printf("[PUSH] No preferences found for user %s", userID)
		return
	}

	// Parse push tokens
	var tokens []string
	if prefs.PushTokens != nil {
		if err := json.Unmarshal(prefs.PushTokens, &tokens); err != nil {
			log.Printf("[PUSH] Failed to parse push tokens: %v", err)
			return
		}
	}

	if len(tokens) == 0 {
		log.Printf("[PUSH] No push tokens registered for user %s", userID)
		return
	}

	// Send to each token
	for _, token := range tokens {
		message := &services.Message{
			To:       token,
			Subject:  title,
			Body:     body,
			Metadata: data,
		}

		result, err := s.pushProvider.Send(ctx, message)
		if err != nil {
			log.Printf("[PUSH] Failed to send to token %s: %v", token[:20]+"...", err)
			continue
		}

		if result.Success {
			log.Printf("[PUSH] Successfully sent to user %s", userID)
		} else {
			log.Printf("[PUSH] Failed to send to user %s: %v", userID, result.Error)
		}
	}
}

// renderEmbeddedTemplate renders using embedded Go templates as fallback
func (s *Subscriber) renderEmbeddedTemplate(templateName string, variables map[string]interface{}) (string, string, error) {
	renderer, err := templates.GetDefaultRenderer()
	if err != nil {
		return "", "", fmt.Errorf("failed to initialize template renderer: %w", err)
	}

	// Map template names to embedded template functions
	// Template names from database use hyphens, embedded templates use underscores
	embeddedName := s.mapTemplateNameToEmbedded(templateName)

	// Build EmailData from variables
	data := s.buildEmailData(variables)

	switch embeddedName {
	// Order templates
	case "order_confirmation":
		return renderer.RenderOrderConfirmation(data)
	case "order_shipped":
		return renderer.RenderOrderShipped(data)
	case "order_delivered":
		return renderer.RenderOrderDelivered(data)
	case "order_cancelled":
		return renderer.RenderOrderCancelled(data)
	// Payment templates
	case "payment_confirmation":
		return renderer.RenderPaymentConfirmation(data)
	case "payment_failed":
		return renderer.RenderPaymentFailed(data)
	case "payment_refunded":
		return renderer.RenderPaymentRefunded(data)
	// Customer templates
	case "customer_welcome", "welcome_email":
		return renderer.RenderCustomerWelcome(data)
	// Inventory templates
	case "low_stock_alert":
		return renderer.RenderLowStockAlert(data)
	// Review templates
	case "review_submitted_customer":
		return renderer.RenderReviewSubmittedCustomer(data)
	case "review_submitted_admin":
		return renderer.RenderReviewSubmittedAdmin(data)
	case "review_approved":
		return renderer.RenderReviewApproved(data)
	case "review_rejected":
		return renderer.RenderReviewRejected(data)
	// Ticket templates
	case "ticket_created":
		return renderer.RenderTicketCreated(data)
	case "ticket_created_admin":
		return renderer.RenderTicketCreatedAdmin(data)
	case "ticket_updated":
		return renderer.RenderTicketUpdated(data)
	case "ticket_resolved":
		return renderer.RenderTicketResolved(data)
	case "ticket_closed":
		return renderer.RenderTicketClosed(data)
	// Vendor templates
	case "vendor_application":
		return renderer.RenderVendorApplication(data)
	case "vendor_welcome":
		return renderer.RenderVendorWelcome(data)
	case "vendor_approved":
		return renderer.RenderVendorApproved(data)
	case "vendor_rejected":
		return renderer.RenderVendorRejected(data)
	case "vendor_suspended":
		return renderer.RenderVendorSuspended(data)
	// Coupon templates
	case "coupon_applied":
		return renderer.RenderCouponApplied(data)
	case "coupon_created":
		return renderer.RenderCouponCreated(data)
	case "coupon_expired":
		return renderer.RenderCouponExpired(data)
	// Auth templates
	case "password_reset":
		return renderer.RenderPasswordReset(data)
	// Tenant onboarding templates
	case "tenant_welcome_pack":
		return renderer.RenderTenantWelcomePack(data)
	case "verification_link":
		return renderer.RenderVerificationLink(data)
	// Approval workflow templates
	case "approval_approver":
		return renderer.RenderApprovalApprover(data)
	case "approval_requester":
		return renderer.RenderApprovalRequester(data)
	// Domain lifecycle templates
	case "domain_added":
		return renderer.RenderDomainAdded(data)
	case "domain_verified":
		return renderer.RenderDomainVerified(data)
	case "domain_ssl_ready":
		return renderer.RenderDomainSSLReady(data)
	case "domain_activated":
		return renderer.RenderDomainActivated(data)
	case "domain_failed":
		return renderer.RenderDomainFailed(data)
	case "domain_removed":
		return renderer.RenderDomainRemoved(data)
	case "domain_migrated":
		return renderer.RenderDomainMigrated(data)
	case "domain_ssl_expiring":
		return renderer.RenderDomainSSLExpiring(data)
	case "domain_health_failed":
		return renderer.RenderDomainHealthFailed(data)
	default:
		return "", "", fmt.Errorf("no embedded template found for: %s", templateName)
	}
}

// mapTemplateNameToEmbedded converts database template names to embedded template names
func (s *Subscriber) mapTemplateNameToEmbedded(templateName string) string {
	// Map hyphenated database names to underscored embedded names
	mapping := map[string]string{
		// Order templates
		"order-confirmation":        "order_confirmation",
		"order-shipped":             "order_shipped",
		"order-delivered":           "order_delivered",
		"order-cancelled":           "order_cancelled",
		// Payment templates
		"payment-confirmation":      "payment_confirmation",
		"payment-failed":            "payment_failed",
		"payment-refunded":          "payment_refunded",
		// Customer templates
		"welcome-email":             "customer_welcome",
		"customer-welcome":          "customer_welcome",
		// Inventory templates
		"low-stock-alert":           "low_stock_alert",
		// Review templates
		"review-submitted-customer": "review_submitted_customer",
		"review-submitted-admin":    "review_submitted_admin",
		"review-approved":           "review_approved",
		"review-rejected":           "review_rejected",
		// Ticket templates
		"ticket-created":            "ticket_created",
		"ticket-created-admin":      "ticket_created_admin",
		"ticket-updated":            "ticket_updated",
		"ticket-resolved":           "ticket_resolved",
		"ticket-closed":             "ticket_closed",
		// Vendor templates
		"vendor-application":        "vendor_application",
		"vendor-welcome":            "vendor_welcome",
		"vendor-approved":           "vendor_approved",
		"vendor-rejected":           "vendor_rejected",
		"vendor-suspended":          "vendor_suspended",
		// Coupon templates
		"coupon-applied":            "coupon_applied",
		"coupon-created":            "coupon_created",
		"coupon-expired":            "coupon_expired",
		// Auth templates
		"password-reset":            "password_reset",
		// Tenant onboarding templates
		"tenant-welcome-pack":       "tenant_welcome_pack",
		"verification-link":         "verification_link",
		// Approval workflow templates
		"approval-request":          "approval_approver",
		"approval-escalated":        "approval_approver",
		"approval-granted":          "approval_requester",
		"approval-rejected":         "approval_requester",
		"approval-cancelled":        "approval_requester",
		"approval-expired":          "approval_requester",
		// Domain lifecycle templates
		"domain-added":        "domain_added",
		"domain-verified":     "domain_verified",
		"domain-ssl-ready":    "domain_ssl_ready",
		"domain-activated":    "domain_activated",
		"domain-failed":       "domain_failed",
		"domain-removed":      "domain_removed",
		"domain-migrated":     "domain_migrated",
		"domain-ssl-expiring": "domain_ssl_expiring",
		"domain-health-failed": "domain_health_failed",
	}

	if mapped, ok := mapping[templateName]; ok {
		return mapped
	}
	return templateName
}

// buildEmailData converts variables map to EmailData struct
func (s *Subscriber) buildEmailData(variables map[string]interface{}) *templates.EmailData {
	data := &templates.EmailData{}

	// Helper to safely get string values
	getString := func(key string) string {
		if v, ok := variables[key]; ok {
			if str, ok := v.(string); ok {
				return str
			}
		}
		return ""
	}

	// Helper to safely get float values as string
	getFloat := func(key string) string {
		if v, ok := variables[key]; ok {
			switch val := v.(type) {
			case float64:
				return fmt.Sprintf("%.2f", val)
			case float32:
				return fmt.Sprintf("%.2f", val)
			case int:
				return fmt.Sprintf("%d.00", val)
			case string:
				return val
			}
		}
		return ""
	}

	// Common fields
	data.Email = getString("customerEmail")
	data.BusinessName = getString("businessName")
	data.SupportEmail = getString("supportEmail")
	if data.SupportEmail == "" {
		data.SupportEmail = "support@tesserix.app"
	}

	// Order fields
	data.OrderNumber = getString("orderNumber")
	data.OrderDate = getString("orderDate")
	data.Currency = getString("currency")
	if data.Currency == "" {
		data.Currency = "$"
	}
	data.Subtotal = getFloat("subtotal")
	data.Discount = getFloat("discount")
	data.Shipping = getFloat("shipping")
	data.Tax = getFloat("tax")
	data.Total = getFloat("totalAmount")
	data.TrackingURL = getString("trackingUrl")
	data.OrderDetailsURL = getString("orderDetailsUrl")
	data.PaymentMethod = getString("paymentMethod")

	// Shipping fields
	data.Carrier = getString("carrierName")
	data.TrackingNumber = getString("trackingNumber")
	data.EstimatedDelivery = getString("estimatedDelivery")
	data.DeliveryDate = getString("deliveryDate")
	data.DeliveryLocation = getString("deliveryLocation")

	// Cancellation fields
	data.CancelledDate = getString("cancelledDate")
	data.CancellationReason = getString("cancellationReason")
	data.RefundAmount = getFloat("refundAmount")
	data.RefundDays = getString("refundDays")
	data.ShopURL = getString("shopUrl")

	// Payment fields
	data.TransactionID = getString("paymentID")
	data.Amount = getFloat("amount")
	data.PaymentDate = getString("paymentDate")
	data.FailureReason = getString("errorMessage")
	data.RetryURL = getString("retryUrl")

	// Customer fields
	data.CustomerName = getString("customerName")
	data.WelcomeOffer = getString("welcomeOffer")
	data.PromoCode = getString("promoCode")
	data.ReviewURL = getString("reviewUrl")

	// Review fields
	data.ReviewID = getString("reviewId")
	data.ProductName = getString("productName")
	data.ProductSKU = getString("productSku")
	data.ReviewTitle = getString("reviewTitle")
	data.ReviewContent = getString("reviewContent")
	data.ReviewStatus = getString("reviewStatus")
	data.RejectReason = getString("rejectReason")
	data.ModeratedBy = getString("moderatedBy")
	data.ReviewsURL = getString("reviewsUrl")
	data.ProductURL = getString("productUrl")

	// Handle rating as int
	if rating, ok := variables["rating"]; ok {
		switch v := rating.(type) {
		case int:
			data.Rating = v
		case float64:
			data.Rating = int(v)
		}
	}
	if maxRating, ok := variables["maxRating"]; ok {
		switch v := maxRating.(type) {
		case int:
			data.MaxRating = v
		case float64:
			data.MaxRating = int(v)
		}
	}
	if data.MaxRating == 0 {
		data.MaxRating = 5
	}

	// Handle isVerified as bool
	if verified, ok := variables["isVerified"]; ok {
		if v, ok := verified.(bool); ok {
			data.IsVerified = v
		}
	}

	// Parse order items if present
	if items, ok := variables["items"]; ok {
		if itemsList, ok := items.([]map[string]interface{}); ok {
			for _, item := range itemsList {
				orderItem := templates.OrderItem{}
				if name, ok := item["name"].(string); ok {
					orderItem.Name = name
				}
				if sku, ok := item["sku"].(string); ok {
					orderItem.SKU = sku
				}
				if imageUrl, ok := item["imageUrl"].(string); ok {
					orderItem.ImageURL = imageUrl
				}
				if qty, ok := item["quantity"].(int); ok {
					orderItem.Quantity = qty
				} else if qty, ok := item["quantity"].(float64); ok {
					orderItem.Quantity = int(qty)
				}
				if price, ok := item["price"].(float64); ok {
					orderItem.Price = fmt.Sprintf("%.2f", price)
				} else if price, ok := item["price"].(string); ok {
					orderItem.Price = price
				}
				if currency, ok := item["currency"].(string); ok {
					orderItem.Currency = currency
				} else {
					orderItem.Currency = data.Currency
				}
				data.Items = append(data.Items, orderItem)
			}
		}
	}

	// Shipping Address - initialize with available data or use customer name as fallback
	shippingName := getString("shippingName")
	if shippingName == "" {
		shippingName = data.CustomerName
	}
	shippingLine1 := getString("shippingLine1")
	if shippingLine1 == "" {
		shippingLine1 = getString("shippingAddress")
	}

	// Only create ShippingAddress if we have at least some data
	if shippingName != "" || shippingLine1 != "" {
		data.ShippingAddress = &templates.Address{
			Name:       shippingName,
			Line1:      shippingLine1,
			Line2:      getString("shippingLine2"),
			City:       getString("shippingCity"),
			State:      getString("shippingState"),
			PostalCode: getString("shippingPostalCode"),
			Country:    getString("shippingCountry"),
		}
	} else {
		// Create a minimal address to prevent nil pointer errors
		data.ShippingAddress = &templates.Address{
			Name:       data.CustomerName,
			Line1:      "Address on file",
			City:       "",
			State:      "",
			PostalCode: "",
		}
	}

	// Ticket fields
	data.TicketID = getString("ticketId")
	data.TicketNumber = getString("ticketNumber")
	data.TicketSubject = getString("subject")
	data.TicketCategory = getString("category")
	data.TicketPriority = getString("priority")
	data.TicketStatus = getString("status")
	data.Description = getString("description")
	data.AssignedTo = getString("assignedTo")
	data.AssignedToName = getString("assignedToName")
	data.Resolution = getString("resolution")
	data.TicketURL = getString("ticketUrl")

	// Inventory fields
	data.InventoryURL = getString("inventoryUrl")
	if totalLow, ok := variables["totalLowStockItems"]; ok {
		switch v := totalLow.(type) {
		case int:
			data.TotalLowStockItems = v
		case float64:
			data.TotalLowStockItems = int(v)
		}
	}
	if totalOut, ok := variables["totalOutOfStockItems"]; ok {
		switch v := totalOut.(type) {
		case int:
			data.TotalOutOfStockItems = v
		case float64:
			data.TotalOutOfStockItems = int(v)
		}
	}
	if totalAffected, ok := variables["totalAffectedProducts"]; ok {
		switch v := totalAffected.(type) {
		case int:
			data.TotalAffectedProducts = v
		case float64:
			data.TotalAffectedProducts = int(v)
		}
	}

	// Parse product stock items if present
	if products, ok := variables["products"]; ok {
		if productsList, ok := products.([]map[string]interface{}); ok {
			for _, product := range productsList {
				stockItem := templates.ProductStock{}
				if name, ok := product["name"].(string); ok {
					stockItem.Name = name
				}
				if sku, ok := product["sku"].(string); ok {
					stockItem.SKU = sku
				}
				if imageUrl, ok := product["imageUrl"].(string); ok {
					stockItem.ImageURL = imageUrl
				}
				if stockLevel, ok := product["stockLevel"].(int); ok {
					stockItem.StockLevel = stockLevel
				} else if stockLevel, ok := product["stockLevel"].(float64); ok {
					stockItem.StockLevel = int(stockLevel)
				}
				data.Products = append(data.Products, stockItem)
			}
		}
	}

	// Vendor fields
	data.VendorID = getString("vendorId")
	data.VendorName = getString("vendorName")
	data.VendorEmail = getString("vendorEmail")
	data.VendorBusinessName = getString("businessName")
	data.VendorStatus = getString("status")
	data.PreviousStatus = getString("previousStatus")
	data.StatusReason = getString("statusReason")
	data.ReviewedBy = getString("reviewedBy")
	data.VendorURL = getString("vendorUrl")

	// Coupon fields
	data.CouponID = getString("couponId")
	data.CouponCode = getString("couponCode")
	data.DiscountType = getString("discountType")
	data.DiscountValue = getFloat("discountValue")
	data.DiscountAmount = getFloat("discountAmount")
	data.OrderValue = getFloat("orderValue")
	data.ValidFrom = getString("validFrom")
	data.ValidUntil = getString("validUntil")
	data.CouponStatus = getString("status")
	data.CouponsURL = getString("couponsUrl")
	if data.ShopURL == "" {
		data.ShopURL = getString("shopUrl")
	}

	// Auth/Security fields
	data.ResetCode = getString("resetToken")
	data.ResetURL = getString("resetUrl")
	data.VerificationLink = getString("verificationLink")
	data.VerificationToken = getString("verificationToken")
	data.VerificationExpiry = getString("verificationExpiry")

	// Tenant onboarding fields
	data.SessionID = getString("sessionId")
	data.BusinessName = getString("businessName")
	data.TenantSlug = getString("slug")
	data.Product = getString("product")
	data.AdminHost = getString("adminHost")
	data.StorefrontHost = getString("storefrontHost")
	data.BaseDomain = getString("baseDomain")
	data.AdminURL = getString("adminUrl")
	data.StorefrontURL = getString("storefrontUrl")

	// Approval workflow fields
	data.ApprovalID = getString("approvalId")
	data.ApprovalStatus = getString("approvalStatus")
	data.ApprovalPriority = getString("approvalPriority")
	data.ActionType = getString("actionType")
	data.ActionTypeDisplay = getString("actionTypeDisplay")
	data.ResourceType = getString("resourceType")
	data.ResourceID = getString("resourceId")
	data.RequesterID = getString("requesterId")
	data.RequesterName = getString("requesterName")
	data.RequesterEmail = getString("requesterEmail")
	data.ApproverID = getString("approverId")
	data.ApproverName = getString("approverName")
	data.ApproverEmail = getString("approverEmail")
	data.ApproverRole = getString("approverRole")
	data.ApprovalReason = getString("approvalReason")
	data.ApprovalComment = getString("approvalComment")
	data.ApprovalExpiresAt = getString("approvalExpiresAt")
	data.ApprovalCreatedAt = getString("approvalCreatedAt")
	data.ApprovalDecidedAt = getString("approvalDecidedAt")
	data.ApprovalURL = getString("approvalUrl")
	data.EscalatedFromID = getString("escalatedFromId")
	data.EscalatedFromName = getString("escalatedFromName")
	if escalationLevel, ok := variables["escalationLevel"]; ok {
		switch v := escalationLevel.(type) {
		case int:
			data.EscalationLevel = v
		case float64:
			data.EscalationLevel = int(v)
		}
	}

	// Login notification fields
	data.LoginTime = getString("loginTime")
	data.LoginLocation = getString("loginLocation")
	data.IPAddress = getString("ipAddress")
	data.DeviceInfo = getString("deviceInfo")
	data.UserAgent = getString("userAgent")
	data.LoginMethod = getString("loginMethod")
	data.ResetPasswordURL = getString("resetPasswordURL")

	// Domain lifecycle fields
	data.DomainID = getString("domainId")
	data.Domain = getString("domain")
	data.DomainType = getString("domainType")
	data.DomainStatus = getString("status")
	data.DomainPreviousStatus = getString("previousStatus")
	data.DNSRecordType = getString("dnsRecordType")
	data.DNSRecordName = getString("dnsRecordName")
	data.DNSRecordValue = getString("dnsRecordValue")
	data.SSLStatus = getString("sslStatus")
	data.SSLExpiresAt = getString("sslExpiresAt")
	data.SSLProvider = getString("sslProvider")
	data.RoutingTarget = getString("routingTarget")
	data.RoutingPath = getString("routingPath")
	data.MigratedFrom = getString("migratedFrom")
	data.MigratedTo = getString("migratedTo")
	data.MigrationReason = getString("migrationReason")
	data.DomainFailureReason = getString("failureReason")
	data.DomainFailureCode = getString("failureCode")
	data.DomainSettingsURL = getString("domainSettingsUrl")
	data.OwnerEmail = getString("ownerEmail")
	data.OwnerName = getString("ownerName")

	return data
}

// formatUserAgent parses user agent string and returns a human-readable device info
func formatUserAgent(userAgent string, deviceType string) string {
	if userAgent == "" {
		if deviceType != "" {
			return deviceType
		}
		return "Unknown device"
	}

	// Simple parsing for common browsers/devices
	device := "Unknown device"

	// Check for mobile devices first
	if strings.Contains(userAgent, "iPhone") {
		device = "iPhone"
	} else if strings.Contains(userAgent, "iPad") {
		device = "iPad"
	} else if strings.Contains(userAgent, "Android") {
		device = "Android device"
	} else if strings.Contains(userAgent, "Windows") {
		device = "Windows PC"
	} else if strings.Contains(userAgent, "Macintosh") || strings.Contains(userAgent, "Mac OS") {
		device = "Mac"
	} else if strings.Contains(userAgent, "Linux") {
		device = "Linux PC"
	}

	// Add browser info
	browser := ""
	if strings.Contains(userAgent, "Chrome") && !strings.Contains(userAgent, "Edg") {
		browser = "Chrome"
	} else if strings.Contains(userAgent, "Safari") && !strings.Contains(userAgent, "Chrome") {
		browser = "Safari"
	} else if strings.Contains(userAgent, "Firefox") {
		browser = "Firefox"
	} else if strings.Contains(userAgent, "Edg") {
		browser = "Edge"
	}

	if browser != "" {
		return fmt.Sprintf("%s using %s", device, browser)
	}

	return device
}
