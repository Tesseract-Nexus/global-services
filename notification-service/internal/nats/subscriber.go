package nats

import (
	"context"
	"encoding/json"
	"os"
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

	// Gift card events (marketing)
	events.GiftCardCreated:   "marketing",
	events.GiftCardActivated: "marketing",
	events.GiftCardApplied:   "orders",
	events.GiftCardRefunded:  "orders",

	// Campaign events (marketing)
	events.CampaignCreated:   "marketing",
	events.CampaignUpdated:   "marketing",
	events.CampaignSent:      "marketing",
	events.CampaignScheduled: "marketing",
	events.CampaignDeleted:   "marketing",

	// Loyalty events (marketing)
	events.LoyaltyProgramCreated:   "marketing",
	events.LoyaltyProgramUpdated:   "marketing",
	events.LoyaltyCustomerEnrolled: "marketing",
	events.LoyaltyPointsRedeemed:   "marketing",
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
	webPushProvider *services.WebPushProvider
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
	webPushProvider *services.WebPushProvider,
	adminEmail string,
	supportEmail string,
) *Subscriber {
	// Use defaults if not provided
	if adminEmail == "" {
		if envAdmin := os.Getenv("ADMIN_EMAIL"); envAdmin != "" {
			adminEmail = envAdmin
		} else {
			adminEmail = "admin@tesserix.app"
		}
	}
	if supportEmail == "" {
		if envSupport := os.Getenv("SUPPORT_EMAIL"); envSupport != "" {
			supportEmail = envSupport
		} else {
			supportEmail = "support@tesserix.app"
		}
	}
	return &Subscriber{
		client:          client,
		notifRepo:       notifRepo,
		templateRepo:    templateRepo,
		prefRepo:        prefRepo,
		emailProvider:   emailProvider,
		smsProvider:     smsProvider,
		pushProvider:    pushProvider,
		webPushProvider: webPushProvider,
		templateEng:     template.NewEngine(),
		subs:            make([]*nats.Subscription, 0),
		adminEmail:      adminEmail,
		supportEmail:    supportEmail,
		tenantClient:    services.NewTenantClient(),
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
		{"GIFT_CARD_EVENTS", "gift_card.>", "Gift card lifecycle events"},
		{"CAMPAIGN_EVENTS", "campaign.>", "Marketing campaign events"},
		{"LOYALTY_EVENTS", "loyalty.>", "Loyalty program events"},
		{"PRODUCT_EVENTS", "product.>", "Product lifecycle events"},
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

	// Subscribe to gift card events (recipient notifications)
	giftCardSub, err := js.QueueSubscribe(
		"gift_card.>",
		"notification-service-workers",
		s.handleGiftCardEvent,
		nats.BindStream("GIFT_CARD_EVENTS"),
		nats.Durable("notification-service-giftcards"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to gift card events: %v", err)
	} else {
		s.subs = append(s.subs, giftCardSub)
		log.Println("[NATS] Subscribed to gift_card.> events")
	}

	// Subscribe to campaign events
	campaignSub, err := js.QueueSubscribe(
		"campaign.>",
		"notification-service-workers",
		s.handleCampaignEvent,
		nats.BindStream("CAMPAIGN_EVENTS"),
		nats.Durable("notification-service-campaigns"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to campaign events: %v", err)
	} else {
		s.subs = append(s.subs, campaignSub)
		log.Println("[NATS] Subscribed to campaign.> events")
	}

	// Subscribe to loyalty events
	loyaltySub, err := js.QueueSubscribe(
		"loyalty.>",
		"notification-service-workers",
		s.handleLoyaltyEvent,
		nats.BindStream("LOYALTY_EVENTS"),
		nats.Durable("notification-service-loyalty"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to loyalty events: %v", err)
	} else {
		s.subs = append(s.subs, loyaltySub)
		log.Println("[NATS] Subscribed to loyalty.> events")
	}

	// Subscribe to product events (admin push for new/updated products)
	productSub, err := js.QueueSubscribe(
		"product.>",
		"notification-service-workers",
		s.handleProductEvent,
		nats.BindStream("PRODUCT_EVENTS"),
		nats.Durable("notification-service-products"),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
	)
	if err != nil {
		log.Printf("[NATS] Warning: failed to subscribe to product events: %v", err)
	} else {
		s.subs = append(s.subs, productSub)
		log.Println("[NATS] Subscribed to product.> events")
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

	// Customer push notification
	if s.shouldSendPush(prefs, category) && event.CustomerID != "" {
		if customerUUID, err := uuid.Parse(event.CustomerID); err == nil {
			var pushTitle, pushBody string
			switch event.EventType {
			case events.OrderCreated, events.OrderConfirmed:
				pushTitle = "Order Confirmed"
				pushBody = fmt.Sprintf("Your order #%s has been placed", event.OrderNumber)
			case events.OrderShipped:
				pushTitle = "Order Shipped"
				pushBody = fmt.Sprintf("Your order #%s is on its way", event.OrderNumber)
			case events.OrderDelivered:
				pushTitle = "Order Delivered"
				pushBody = fmt.Sprintf("Your order #%s has been delivered", event.OrderNumber)
			case events.OrderCancelled:
				pushTitle = "Order Cancelled"
				pushBody = fmt.Sprintf("Your order #%s has been cancelled", event.OrderNumber)
			}
			if pushTitle != "" {
				s.sendPushNotification(ctx, event.TenantID, customerUUID, pushTitle, pushBody, map[string]interface{}{
					"type":      event.EventType,
					"orderId":   event.OrderID,
					"actionUrl": "/account/orders/" + event.OrderID,
				})
			}
		}
	}

	// Admin push for new orders
	if event.EventType == events.OrderCreated || event.EventType == events.OrderConfirmed {
		s.sendAdminPush(ctx, event.TenantID,
			"New Order",
			fmt.Sprintf("New order #%s — %s %.2f", event.OrderNumber, event.Currency, event.TotalAmount),
			map[string]interface{}{
				"type":      event.EventType,
				"orderId":   event.OrderID,
				"actionUrl": "/orders/" + event.OrderID,
			},
		)
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

	// Customer push notification for payment events
	if s.shouldSendPush(prefs, category) && event.CustomerID != "" {
		if customerUUID, err := uuid.Parse(event.CustomerID); err == nil {
			var pushTitle, pushBody string
			switch event.EventType {
			case events.PaymentFailed:
				pushTitle = "Payment Failed"
				pushBody = fmt.Sprintf("Payment for order #%s failed", event.OrderNumber)
			case events.PaymentRefunded:
				pushTitle = "Refund Processed"
				pushBody = fmt.Sprintf("Your refund for order #%s has been processed", event.OrderNumber)
			case events.PaymentCaptured:
				pushTitle = "Payment Confirmed"
				pushBody = fmt.Sprintf("Payment for order #%s confirmed", event.OrderNumber)
			}
			if pushTitle != "" {
				s.sendPushNotification(ctx, event.TenantID, customerUUID, pushTitle, pushBody, map[string]interface{}{
					"type":      event.EventType,
					"orderId":   event.OrderID,
					"actionUrl": "/account/orders/" + event.OrderID,
				})
			}
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

		// Admin push for new reviews
		s.sendAdminPush(ctx, event.TenantID,
			"New Review",
			fmt.Sprintf("New %d-star review for %s needs approval", event.Rating, event.ProductName),
			map[string]interface{}{
				"type":      event.EventType,
				"reviewId":  event.ReviewID,
				"actionUrl": "/reviews",
			},
		)

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

		// Admin push for inventory alerts
		var pushTitle, pushBody string
		if event.EventType == events.InventoryOutOfStock {
			pushTitle = "Out of Stock"
			pushBody = fmt.Sprintf("%d product(s) are out of stock", event.TotalOutOfStock)
		} else {
			pushTitle = "Low Stock Alert"
			pushBody = fmt.Sprintf("%d product(s) are running low on stock", event.TotalLowStock)
		}
		s.sendAdminPush(ctx, event.TenantID, pushTitle, pushBody, map[string]interface{}{
			"type":      event.EventType,
			"actionUrl": "/inventory",
		})
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

		// Admin push for new tickets
		s.sendAdminPush(ctx, event.TenantID,
			"New Support Ticket",
			fmt.Sprintf("Ticket: %s", event.Subject),
			map[string]interface{}{
				"type":      event.EventType,
				"ticketId":  event.TicketID,
				"actionUrl": "/tickets/" + event.TicketID,
			},
		)

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

		// Admin push for approval requests
		s.sendAdminPush(ctx, event.TenantID,
			"Approval Required",
			fmt.Sprintf("%s by %s needs approval", formatActionTypeForDisplay(event.ActionType), event.RequesterName),
			map[string]interface{}{
				"type":       event.EventType,
				"approvalId": event.ApprovalRequestID,
				"actionUrl":  "/approvals/" + event.ApprovalRequestID,
			},
		)

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
		// Push to requester
		if event.RequesterID != "" {
			if requesterUUID, err := uuid.Parse(event.RequesterID); err == nil {
				s.sendPushNotification(ctx, event.TenantID, requesterUUID,
					"Request Approved",
					fmt.Sprintf("Your %s request has been approved", formatActionTypeForDisplay(event.ActionType)),
					map[string]interface{}{
						"type":       event.EventType,
						"approvalId": event.ApprovalRequestID,
						"actionUrl":  "/approvals/" + event.ApprovalRequestID,
					},
				)
			}
		}

	case events.ApprovalRejected:
		// Notify requester that their request was rejected
		variables["approvalStatus"] = "REJECTED"
		if event.RequesterEmail != "" {
			log.Printf("[EMAIL] Sending approval-rejected to requester %s", event.RequesterEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "approval-rejected", event.RequesterEmail, variables)
		}
		// Push to requester
		if event.RequesterID != "" {
			if requesterUUID, err := uuid.Parse(event.RequesterID); err == nil {
				s.sendPushNotification(ctx, event.TenantID, requesterUUID,
					"Request Rejected",
					fmt.Sprintf("Your %s request has been rejected", formatActionTypeForDisplay(event.ActionType)),
					map[string]interface{}{
						"type":       event.EventType,
						"approvalId": event.ApprovalRequestID,
						"actionUrl":  "/approvals/" + event.ApprovalRequestID,
					},
				)
			}
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

// handleGiftCardEvent processes gift card events (send gift card details to recipients via email)
func (s *Subscriber) handleGiftCardEvent(msg *nats.Msg) {
	var event events.GiftCardEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal gift card event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing gift card event: %s for gift card %s", event.EventType, event.GiftCardID)

	ctx := context.Background()

	variables := map[string]interface{}{
		"giftCardId":     event.GiftCardID,
		"giftCardCode":   event.GiftCardCode,
		"purchaserEmail": event.PurchaserEmail,
		"purchaserName":  event.PurchaserName,
		"recipientEmail": event.RecipientEmail,
		"recipientName":  event.RecipientName,
		"initialBalance": event.InitialBalance,
		"currentBalance": event.CurrentBalance,
		"currency":       event.Currency,
		"status":         event.Status,
		"message":        event.Message,
		"expiresAt":      event.ExpiresAt,
	}

	switch event.EventType {
	case events.GiftCardCreated:
		// Send gift card details to recipient if email is provided
		if event.RecipientEmail != "" {
			log.Printf("[EMAIL] Sending gift-card-recipient to %s", event.RecipientEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "gift-card-recipient", event.RecipientEmail, variables)
		}
		// Also notify purchaser if different from recipient
		if event.PurchaserEmail != "" && event.PurchaserEmail != event.RecipientEmail {
			log.Printf("[EMAIL] Sending gift-card-purchaser to %s", event.PurchaserEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "gift-card-purchaser", event.PurchaserEmail, variables)
		}

	case events.GiftCardActivated:
		if event.RecipientEmail != "" {
			log.Printf("[EMAIL] Sending gift-card-activated to %s", event.RecipientEmail)
			s.sendTemplatedEmail(ctx, event.TenantID, "gift-card-activated", event.RecipientEmail, variables)
		}

	default:
		log.Printf("[NATS] Skipping unhandled gift card event type: %s", event.EventType)
	}

	msg.Ack()
	log.Printf("[NATS] Processed gift card event: %s for gift card %s", event.EventType, event.GiftCardID)
}

// handleCampaignEvent processes campaign-related events
func (s *Subscriber) handleCampaignEvent(msg *nats.Msg) {
	var event events.CampaignEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal campaign event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing campaign event: %s for campaign %s", event.EventType, event.CampaignName)

	ctx := context.Background()

	variables := map[string]interface{}{
		"campaignId":        event.CampaignID,
		"campaignName":      event.CampaignName,
		"campaignType":      event.CampaignType,
		"campaignChannel":   event.Channel,
		"campaignStatus":    event.Status,
		"totalRecipients":   event.TotalRecipients,
		"campaignDelivered": event.Delivered,
		"campaignOpened":    event.Opened,
		"campaignClicked":   event.Clicked,
		"campaignConverted": event.Converted,
		"campaignRevenue":   event.Revenue,
		"campaignScheduledAt": event.ScheduledAt,
		"campaignActorName":   event.ActorName,
	}

	switch event.EventType {
	case events.CampaignSent, events.CampaignCompleted, events.CampaignCancelled,
		events.CampaignScheduled, events.CampaignPaused:
		log.Printf("[EMAIL] Sending campaign-admin to %s for campaign %s", s.adminEmail, event.CampaignName)
		s.sendTemplatedEmail(ctx, event.TenantID, "campaign-admin", s.adminEmail, variables)

	default:
		log.Printf("[NATS] Skipping unhandled campaign event type: %s", event.EventType)
	}

	msg.Ack()
	log.Printf("[NATS] Processed campaign event: %s for campaign %s", event.EventType, event.CampaignName)
}

// handleLoyaltyEvent processes loyalty program-related events
func (s *Subscriber) handleLoyaltyEvent(msg *nats.Msg) {
	var event events.LoyaltyEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal loyalty event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing loyalty event: %s", event.EventType)

	// Loyalty events are primarily for in-app notifications (handled by notification-hub)
	// and audit logging (handled by audit-service).
	// Email notifications for loyalty events can be added here if needed.

	msg.Ack()
	log.Printf("[NATS] Processed loyalty event: %s", event.EventType)
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

// handleProductEvent processes product lifecycle events (admin push only)
func (s *Subscriber) handleProductEvent(msg *nats.Msg) {
	var event events.ProductEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("[NATS] Failed to unmarshal product event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[NATS] Processing product event: %s for product %s", event.EventType, event.ProductName)

	ctx := context.Background()

	switch event.EventType {
	case events.ProductCreated:
		s.sendAdminPush(ctx, event.TenantID,
			"New Product",
			fmt.Sprintf("Product \"%s\" has been created", event.ProductName),
			map[string]interface{}{
				"type":      event.EventType,
				"productId": event.ProductID,
				"actionUrl": "/catalog/products/" + event.ProductID,
			},
		)

	case events.ProductPublished:
		s.sendAdminPush(ctx, event.TenantID,
			"Product Published",
			fmt.Sprintf("Product \"%s\" is now live", event.ProductName),
			map[string]interface{}{
				"type":      event.EventType,
				"productId": event.ProductID,
				"actionUrl": "/catalog/products/" + event.ProductID,
			},
		)
	}

	msg.Ack()
	log.Printf("[NATS] Processed product event: %s for product %s", event.EventType, event.ProductName)
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

// camelToSnake converts camelCase or PascalCase strings to snake_case.
// Handles consecutive uppercase letters like "orderID" → "order_id", "trackingURL" → "tracking_url".
func camelToSnake(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				prev := s[i-1]
				// Insert underscore before uppercase if preceded by lowercase,
				// or if preceded by uppercase and followed by lowercase (e.g. "URL" in "trackingURLPath")
				if prev >= 'a' && prev <= 'z' {
					result = append(result, '_')
				} else if prev >= 'A' && prev <= 'Z' && i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
					result = append(result, '_')
				}
			}
			result = append(result, byte(c)+32) // toLower
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

// normalizeVariables converts camelCase variable keys to snake_case and adds semantic aliases
// so that DB templates (which use snake_case like order_total, store_name) work with the
// camelCase variables built by event handlers (totalAmount, businessName, etc.).
func normalizeVariables(vars map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(vars)*2)
	// First pass: copy all original keys AND add snake_case versions
	for k, v := range vars {
		result[k] = v // keep original camelCase key for embedded fallback
		snakeKey := camelToSnake(k)
		if snakeKey != k {
			result[snakeKey] = v
		}
	}
	// Second pass: semantic aliases (code snake_case → DB template variable names)
	aliases := map[string]string{
		"total_amount":   "order_total",
		"business_name":  "store_name",
		"storefront_url": "store_url",
		"amount":         "payment_amount",
		"payment_id":     "transaction_id",
	}
	for src, tgt := range aliases {
		if v, ok := result[src]; ok {
			if _, exists := result[tgt]; !exists {
				result[tgt] = v
			}
		}
	}
	return result
}

func (s *Subscriber) sendTemplatedEmail(ctx context.Context, tenantID, templateName, recipient string, variables map[string]interface{}) {
	if s.emailProvider == nil {
		log.Println("[EMAIL] Provider not configured, skipping")
		return
	}

	// Fetch tenant info for branding
	if tenantID != "" && tenantID != "system" {
		tenantInfo, err := s.tenantClient.GetTenantInfo(tenantID)
		if err == nil && tenantInfo != nil {
			// Add tenant branding to variables
			if variables["businessName"] == nil || variables["businessName"] == "" {
				variables["businessName"] = tenantInfo.BusinessName
			}
			if variables["supportEmail"] == nil || variables["supportEmail"] == "" {
				variables["supportEmail"] = tenantInfo.SupportEmail
			}
			// Add branding colors
			if tenantInfo.BrandPrimaryColor != "" {
				variables["brandPrimaryColor"] = tenantInfo.BrandPrimaryColor
			}
			if tenantInfo.BrandSecondaryColor != "" {
				variables["brandSecondaryColor"] = tenantInfo.BrandSecondaryColor
			}
			if tenantInfo.BrandAccentColor != "" {
				variables["brandAccentColor"] = tenantInfo.BrandAccentColor
			}
			if tenantInfo.BrandTextColor != "" {
				variables["brandTextColor"] = tenantInfo.BrandTextColor
			}
			if tenantInfo.BrandLogoURL != "" {
				variables["brandLogoUrl"] = tenantInfo.BrandLogoURL
			}
			// Add URLs if not already set
			if variables["adminUrl"] == nil || variables["adminUrl"] == "" {
				variables["adminUrl"] = tenantInfo.AdminURL
			}
			if variables["storefrontUrl"] == nil || variables["storefrontUrl"] == "" {
				variables["storefrontUrl"] = tenantInfo.StorefrontURL
			}
		}
	}

	// Normalize variable keys: camelCase → snake_case + semantic aliases
	normalizedVars := normalizeVariables(variables)

	var subject, body string
	var tmplID *uuid.UUID

	// Get template by slug (try tenant-specific first, then default-tenant)
	tmpl, err := s.templateRepo.GetBySlug(ctx, tenantID, templateName)
	if err != nil || tmpl == nil {
		tmpl, err = s.templateRepo.GetBySlug(ctx, "system", templateName)
	}

	if tmpl != nil {
		tmplID = &tmpl.ID

		// Render subject using normalized variables for DB templates
		subject = tmpl.Subject
		if subject != "" {
			rendered, err := s.templateEng.RenderText(subject, normalizedVars)
			if err != nil {
				log.Printf("[EMAIL] Failed to render subject: %v", err)
			} else {
				subject = rendered
			}
		}

		// Render body using normalized variables for DB templates
		if tmpl.HTMLTemplate != "" {
			rendered, err := s.templateEng.RenderHTML(tmpl.HTMLTemplate, normalizedVars)
			if err != nil {
				log.Printf("[EMAIL] Failed to render HTML template: %v", err)
			} else {
				body = rendered
			}
		} else if tmpl.BodyTemplate != "" {
			rendered, err := s.templateEng.RenderText(tmpl.BodyTemplate, normalizedVars)
			if err != nil {
				log.Printf("[EMAIL] Failed to render body template: %v", err)
			} else {
				body = rendered
			}
		}
	} else {
		log.Printf("[EMAIL] ERROR: No DB template for slug '%s' (tenant: %s)", templateName, tenantID)
		return
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
	// Get user's push preferences
	prefs, err := s.prefRepo.GetByUserID(ctx, tenantID, userID)
	if err != nil || prefs == nil {
		log.Printf("[PUSH] No preferences found for user %s", userID)
		return
	}

	// Web Push subscriptions (VAPID) — primary path
	if s.webPushProvider != nil {
		var subs []models.PushSubscription
		if prefs.PushSubscriptions != nil {
			if err := json.Unmarshal(prefs.PushSubscriptions, &subs); err != nil {
				log.Printf("[PUSH] Failed to parse push subscriptions: %v", err)
			}
		}
		for _, sub := range subs {
			payload := services.WebPushPayload{
				Title: title,
				Body:  body,
				Icon:  "/logo-icon.png",
				Badge: "/logo-icon.png",
				Data:  data,
			}
			if err := s.webPushProvider.SendToSubscription(ctx, &sub, payload); err != nil {
				log.Printf("[WebPush] Failed to send to subscription: %v", err)
			}
		}
	}

	// FCM tokens (mobile fallback) — only if FCM provider configured
	if s.pushProvider != nil {
		var tokens []string
		if prefs.PushTokens != nil {
			if err := json.Unmarshal(prefs.PushTokens, &tokens); err != nil {
				log.Printf("[PUSH] Failed to parse push tokens: %v", err)
			}
		}

		for _, token := range tokens {
			message := &services.Message{
				To:       token,
				Subject:  title,
				Body:     body,
				Metadata: data,
			}

			result, err := s.pushProvider.Send(ctx, message)
			if err != nil {
				tokenPreview := token
				if len(tokenPreview) > 20 {
					tokenPreview = tokenPreview[:20] + "..."
				}
				log.Printf("[PUSH] Failed to send to token %s: %v", tokenPreview, err)
				continue
			}

			if result.Success {
				log.Printf("[PUSH] Successfully sent to user %s", userID)
			} else {
				log.Printf("[PUSH] Failed to send to user %s: %v", userID, result.Error)
			}
		}
	}
}

// sendAdminPush sends a push notification to all push-enabled staff members in a tenant
func (s *Subscriber) sendAdminPush(ctx context.Context, tenantID, title, body string, data map[string]interface{}) {
	staffPrefs, err := s.prefRepo.GetPushEnabledByTenant(ctx, tenantID)
	if err != nil {
		log.Printf("[PUSH] Failed to get push-enabled staff for tenant %s: %v", tenantID, err)
		return
	}
	for _, pref := range staffPrefs {
		s.sendPushNotification(ctx, tenantID, pref.UserID, title, body, data)
	}
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
