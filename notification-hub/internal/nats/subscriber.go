package nats

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"notification-hub/internal/models"
	"notification-hub/internal/repository"
	"notification-hub/internal/websocket"
)

// TargetUserResolver resolves the target user(s) for a notification
type TargetUserResolver interface {
	// GetConnectedUsers returns connected users for a tenant who should receive notifications
	GetConnectedUsers(tenantID string) []uuid.UUID
}

// DefaultTargetUserResolver is a simple resolver that returns empty list
type DefaultTargetUserResolver struct{}

func (r *DefaultTargetUserResolver) GetConnectedUsers(tenantID string) []uuid.UUID {
	return nil
}

// Subscriber handles NATS event subscriptions
type Subscriber struct {
	client       *Client
	hub          *websocket.Hub
	notifRepo    repository.NotificationRepository
	userResolver TargetUserResolver
	subs         []*nats.Subscription
}

// NewSubscriber creates a new NATS subscriber
func NewSubscriber(
	client *Client,
	hub *websocket.Hub,
	notifRepo repository.NotificationRepository,
	userResolver TargetUserResolver,
) *Subscriber {
	if userResolver == nil {
		userResolver = &DefaultTargetUserResolver{}
	}
	return &Subscriber{
		client:       client,
		hub:          hub,
		notifRepo:    notifRepo,
		userResolver: userResolver,
		subs:         make([]*nats.Subscription, 0),
	}
}

// Start begins subscribing to all event streams
func (s *Subscriber) Start(ctx context.Context) error {
	js := s.client.JetStream()

	// Subscribe to order events
	orderSub, err := js.QueueSubscribe(
		"order.>",
		"notification-hub-workers",
		s.handleOrderEvent,
		nats.BindStream("ORDER_EVENTS"),
		nats.Durable("notification-hub-orders"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to order events: %v", err)
	} else {
		s.subs = append(s.subs, orderSub)
		log.Println("Subscribed to order.> events")
	}

	// Subscribe to payment events
	paymentSub, err := js.QueueSubscribe(
		"payment.>",
		"notification-hub-workers",
		s.handlePaymentEvent,
		nats.BindStream("PAYMENT_EVENTS"),
		nats.Durable("notification-hub-payments"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to payment events: %v", err)
	} else {
		s.subs = append(s.subs, paymentSub)
		log.Println("Subscribed to payment.> events")
	}

	// Subscribe to inventory events
	inventorySub, err := js.QueueSubscribe(
		"inventory.>",
		"notification-hub-workers",
		s.handleInventoryEvent,
		nats.BindStream("INVENTORY_EVENTS"),
		nats.Durable("notification-hub-inventory"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to inventory events: %v", err)
	} else {
		s.subs = append(s.subs, inventorySub)
		log.Println("Subscribed to inventory.> events")
	}

	// Subscribe to customer events
	customerSub, err := js.QueueSubscribe(
		"customer.>",
		"notification-hub-workers",
		s.handleCustomerEvent,
		nats.BindStream("CUSTOMER_EVENTS"),
		nats.Durable("notification-hub-customers"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to customer events: %v", err)
	} else {
		s.subs = append(s.subs, customerSub)
		log.Println("Subscribed to customer.> events")
	}

	// Subscribe to return events
	returnSub, err := js.QueueSubscribe(
		"return.>",
		"notification-hub-workers",
		s.handleReturnEvent,
		nats.BindStream("RETURN_EVENTS"),
		nats.Durable("notification-hub-returns"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to return events: %v", err)
	} else {
		s.subs = append(s.subs, returnSub)
		log.Println("Subscribed to return.> events")
	}

	// Subscribe to review events
	reviewSub, err := js.QueueSubscribe(
		"review.>",
		"notification-hub-workers",
		s.handleReviewEvent,
		nats.BindStream("REVIEW_EVENTS"),
		nats.Durable("notification-hub-reviews"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to review events: %v", err)
	} else {
		s.subs = append(s.subs, reviewSub)
		log.Println("Subscribed to review.> events")
	}

	// Subscribe to product events
	productSub, err := js.QueueSubscribe(
		"product.>",
		"notification-hub-workers",
		s.handleProductEvent,
		nats.BindStream("PRODUCT_EVENTS"),
		nats.Durable("notification-hub-products"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to product events: %v", err)
	} else {
		s.subs = append(s.subs, productSub)
		log.Println("Subscribed to product.> events")
	}

	// Subscribe to category events
	categorySub, err := js.QueueSubscribe(
		"category.>",
		"notification-hub-workers",
		s.handleCategoryEvent,
		nats.BindStream("CATEGORY_EVENTS"),
		nats.Durable("notification-hub-categories"),
		nats.DeliverAll(), // Changed from DeliverNew to receive all messages
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to category events: %v", err)
	} else {
		s.subs = append(s.subs, categorySub)
		log.Println("Subscribed to category.> events")
	}

	// Subscribe to ticket events
	ticketSub, err := js.QueueSubscribe(
		"ticket.>",
		"notification-hub-workers",
		s.handleTicketEvent,
		nats.BindStream("TICKET_EVENTS"),
		nats.Durable("notification-hub-tickets"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to ticket events: %v", err)
	} else {
		s.subs = append(s.subs, ticketSub)
		log.Println("Subscribed to ticket.> events")
	}

	// Subscribe to staff events
	staffSub, err := js.QueueSubscribe(
		"staff.>",
		"notification-hub-workers",
		s.handleStaffEvent,
		nats.BindStream("STAFF_EVENTS"),
		nats.Durable("notification-hub-staff"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to staff events: %v", err)
	} else {
		s.subs = append(s.subs, staffSub)
		log.Println("Subscribed to staff.> events")
	}

	// Subscribe to coupon events
	couponSub, err := js.QueueSubscribe(
		"coupon.>",
		"notification-hub-workers",
		s.handleCouponEvent,
		nats.BindStream("COUPON_EVENTS"),
		nats.Durable("notification-hub-coupons"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to coupon events: %v", err)
	} else {
		s.subs = append(s.subs, couponSub)
		log.Println("Subscribed to coupon.> events")
	}

	// Subscribe to vendor events
	vendorSub, err := js.QueueSubscribe(
		"vendor.>",
		"notification-hub-workers",
		s.handleVendorEvent,
		nats.BindStream("VENDOR_EVENTS"),
		nats.Durable("notification-hub-vendors"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to vendor events: %v", err)
	} else {
		s.subs = append(s.subs, vendorSub)
		log.Println("Subscribed to vendor.> events")
	}

	// Subscribe to approval events
	approvalSub, err := js.QueueSubscribe(
		"approval.>",
		"notification-hub-workers",
		s.handleApprovalEvent,
		nats.BindStream("APPROVAL_EVENTS"),
		nats.Durable("notification-hub-approvals"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to approval events: %v", err)
	} else {
		s.subs = append(s.subs, approvalSub)
		log.Println("Subscribed to approval.> events")
	}

	// Subscribe to gift card events
	giftCardSub, err := js.QueueSubscribe(
		"gift_card.>",
		"notification-hub-workers",
		s.handleGiftCardEvent,
		nats.BindStream("GIFT_CARD_EVENTS"),
		nats.Durable("notification-hub-gift-cards"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to gift card events: %v", err)
	} else {
		s.subs = append(s.subs, giftCardSub)
		log.Println("Subscribed to gift_card.> events")
	}

	// Subscribe to campaign events
	campaignSub, err := js.QueueSubscribe(
		"campaign.>",
		"notification-hub-workers",
		s.handleCampaignEvent,
		nats.BindStream("CAMPAIGN_EVENTS"),
		nats.Durable("notification-hub-campaigns"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to campaign events: %v", err)
	} else {
		s.subs = append(s.subs, campaignSub)
		log.Println("Subscribed to campaign.> events")
	}

	// Subscribe to loyalty events
	loyaltySub, err := js.QueueSubscribe(
		"loyalty.>",
		"notification-hub-workers",
		s.handleLoyaltyEvent,
		nats.BindStream("LOYALTY_EVENTS"),
		nats.Durable("notification-hub-loyalty"),
		nats.DeliverAll(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3),
		nats.InactiveThreshold(24*time.Hour),
	)
	if err != nil {
		log.Printf("Warning: failed to subscribe to loyalty events: %v", err)
	} else {
		s.subs = append(s.subs, loyaltySub)
		log.Println("Subscribed to loyalty.> events")
	}

	log.Printf("NATS subscriber started with %d subscriptions", len(s.subs))
	return nil
}

// Stop unsubscribes from all streams
func (s *Subscriber) Stop() {
	for _, sub := range s.subs {
		if err := sub.Drain(); err != nil {
			log.Printf("Error draining subscription: %v", err)
		}
	}
	log.Println("NATS subscriber stopped")
}

func (s *Subscriber) handleOrderEvent(msg *nats.Msg) {
	var event models.OrderEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal order event: %v", err)
		msg.Ack() // Don't retry malformed messages
		return
	}

	// Check for deduplication (admin notification)
	exists, err := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if err != nil {
		log.Printf("Failed to check for duplicate: %v", err)
		msg.Nak()
		return
	}
	if exists {
		log.Printf("Duplicate event ignored: %s", event.SourceID)
		msg.Ack()
		return
	}

	// ========================================
	// 1. Create ADMIN notification (broadcast)
	// ========================================
	targetUsers := s.getTargetUsers(event.TenantID)
	if len(targetUsers) == 0 {
		log.Printf("No connected admin users for tenant %s, storing broadcast notification", event.TenantID)
		notification := models.EventToNotification(&event, uuid.Nil)
		if notification != nil {
			if err := s.notifRepo.Create(context.Background(), notification); err != nil {
				log.Printf("Failed to create broadcast notification: %v", err)
			} else {
				log.Printf("Created admin broadcast notification for tenant %s", event.TenantID)
			}
		}
	} else {
		for _, userID := range targetUsers {
			notification := models.EventToNotification(&event, userID)
			if notification == nil {
				continue
			}
			if err := s.notifRepo.Create(context.Background(), notification); err != nil {
				log.Printf("Failed to create admin notification: %v", err)
				continue
			}
			s.hub.BroadcastToUser(event.TenantID, userID, notification)
			count, _ := s.notifRepo.GetUnreadCount(context.Background(), event.TenantID, userID)
			s.hub.BroadcastUnreadCount(event.TenantID, userID, int(count))
		}
	}

	// ========================================
	// 2. Create CUSTOMER notification
	// ========================================
	if event.CustomerID != "" {
		customerID, err := uuid.Parse(event.CustomerID)
		if err == nil {
			customerNotif := models.CustomerEventToNotification(&event, customerID)
			if customerNotif != nil {
				if err := s.notifRepo.Create(context.Background(), customerNotif); err != nil {
					log.Printf("Failed to create customer notification: %v", err)
				} else {
					log.Printf("Created customer notification for %s: %s", customerID, event.EventType)
					// Broadcast to customer if connected
					s.hub.BroadcastToUser(event.TenantID, customerID, customerNotif)
					count, _ := s.notifRepo.GetUnreadCount(context.Background(), event.TenantID, customerID)
					s.hub.BroadcastUnreadCount(event.TenantID, customerID, int(count))
				}
			}
		}
	}

	msg.Ack()
	log.Printf("Processed order event: %s", event.EventType)
}

func (s *Subscriber) handlePaymentEvent(msg *nats.Msg) {
	var event models.PaymentEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal payment event: %v", err)
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	targetUsers := s.getTargetUsers(event.TenantID)
	if len(targetUsers) == 0 {
		// Store as broadcast notification
		notification := models.EventToNotification(&event, uuid.Nil)
		if notification != nil {
			s.notifRepo.Create(context.Background(), notification)
		}
	} else {
		for _, userID := range targetUsers {
			notification := models.EventToNotification(&event, userID)
			if notification == nil {
				continue
			}
			if err := s.notifRepo.Create(context.Background(), notification); err != nil {
				continue
			}
			s.hub.BroadcastToUser(event.TenantID, userID, notification)
			count, _ := s.notifRepo.GetUnreadCount(context.Background(), event.TenantID, userID)
			s.hub.BroadcastUnreadCount(event.TenantID, userID, int(count))
		}
	}

	msg.Ack()
	log.Printf("Processed payment event: %s", event.EventType)
}

func (s *Subscriber) handleInventoryEvent(msg *nats.Msg) {
	var event models.InventoryEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	targetUsers := s.getTargetUsers(event.TenantID)
	if len(targetUsers) == 0 {
		notification := models.EventToNotification(&event, uuid.Nil)
		if notification != nil {
			s.notifRepo.Create(context.Background(), notification)
		}
	} else {
		for _, userID := range targetUsers {
			notification := models.EventToNotification(&event, userID)
			if notification == nil {
				continue
			}
			s.notifRepo.Create(context.Background(), notification)
			s.hub.BroadcastToUser(event.TenantID, userID, notification)
			count, _ := s.notifRepo.GetUnreadCount(context.Background(), event.TenantID, userID)
			s.hub.BroadcastUnreadCount(event.TenantID, userID, int(count))
		}
	}

	msg.Ack()
	log.Printf("Processed inventory event: %s", event.EventType)
}

func (s *Subscriber) handleCustomerEvent(msg *nats.Msg) {
	var event models.CustomerEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	targetUsers := s.getTargetUsers(event.TenantID)
	if len(targetUsers) == 0 {
		notification := models.EventToNotification(&event, uuid.Nil)
		if notification != nil {
			s.notifRepo.Create(context.Background(), notification)
		}
	} else {
		for _, userID := range targetUsers {
			notification := models.EventToNotification(&event, userID)
			if notification == nil {
				continue
			}
			s.notifRepo.Create(context.Background(), notification)
			s.hub.BroadcastToUser(event.TenantID, userID, notification)
			count, _ := s.notifRepo.GetUnreadCount(context.Background(), event.TenantID, userID)
			s.hub.BroadcastUnreadCount(event.TenantID, userID, int(count))
		}
	}

	msg.Ack()
	log.Printf("Processed customer event: %s", event.EventType)
}

func (s *Subscriber) handleReturnEvent(msg *nats.Msg) {
	var event models.ReturnEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	// 1. Create ADMIN notification
	targetUsers := s.getTargetUsers(event.TenantID)
	if len(targetUsers) == 0 {
		notification := models.EventToNotification(&event, uuid.Nil)
		if notification != nil {
			s.notifRepo.Create(context.Background(), notification)
		}
	} else {
		for _, userID := range targetUsers {
			notification := models.EventToNotification(&event, userID)
			if notification == nil {
				continue
			}
			s.notifRepo.Create(context.Background(), notification)
			s.hub.BroadcastToUser(event.TenantID, userID, notification)
			count, _ := s.notifRepo.GetUnreadCount(context.Background(), event.TenantID, userID)
			s.hub.BroadcastUnreadCount(event.TenantID, userID, int(count))
		}
	}

	// 2. Create CUSTOMER notification
	if event.CustomerID != "" {
		customerID, err := uuid.Parse(event.CustomerID)
		if err == nil {
			customerNotif := models.CustomerEventToNotification(&event, customerID)
			if customerNotif != nil {
				if err := s.notifRepo.Create(context.Background(), customerNotif); err != nil {
					log.Printf("Failed to create customer return notification: %v", err)
				} else {
					log.Printf("Created customer return notification for %s: %s", customerID, event.EventType)
					s.hub.BroadcastToUser(event.TenantID, customerID, customerNotif)
					count, _ := s.notifRepo.GetUnreadCount(context.Background(), event.TenantID, customerID)
					s.hub.BroadcastUnreadCount(event.TenantID, customerID, int(count))
				}
			}
		}
	}

	msg.Ack()
	log.Printf("Processed return event: %s", event.EventType)
}

func (s *Subscriber) handleReviewEvent(msg *nats.Msg) {
	var event models.ReviewEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	targetUsers := s.getTargetUsers(event.TenantID)
	if len(targetUsers) == 0 {
		notification := models.EventToNotification(&event, uuid.Nil)
		if notification != nil {
			s.notifRepo.Create(context.Background(), notification)
		}
	} else {
		for _, userID := range targetUsers {
			notification := models.EventToNotification(&event, userID)
			if notification == nil {
				continue
			}
			s.notifRepo.Create(context.Background(), notification)
			s.hub.BroadcastToUser(event.TenantID, userID, notification)
			count, _ := s.notifRepo.GetUnreadCount(context.Background(), event.TenantID, userID)
			s.hub.BroadcastUnreadCount(event.TenantID, userID, int(count))
		}
	}

	msg.Ack()
	log.Printf("Processed review event: %s", event.EventType)
}

// getTargetUsers returns the list of users who should receive notifications for a tenant
func (s *Subscriber) getTargetUsers(tenantID string) []uuid.UUID {
	// Get connected users from WebSocket/SSE hubs
	users := s.userResolver.GetConnectedUsers(tenantID)
	if len(users) > 0 {
		log.Printf("Found %d connected users for tenant %s", len(users), tenantID)
	}
	return users
}

func (s *Subscriber) handleProductEvent(msg *nats.Msg) {
	var event models.ProductEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal product event: %v", err)
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed product event: %s for product %s", event.EventType, event.ProductName)
}

func (s *Subscriber) handleCategoryEvent(msg *nats.Msg) {
	log.Printf("[DEBUG] Received category event, raw data: %s", string(msg.Data))

	var event models.CategoryEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal category event: %v", err)
		msg.Ack()
		return
	}

	log.Printf("[DEBUG] Unmarshaled category event: type=%s, categoryID=%s, tenantID=%s, sourceID=%s",
		event.EventType, event.CategoryID, event.TenantID, event.SourceID)

	exists, err := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if err != nil {
		log.Printf("[DEBUG] Error checking if event exists: %v", err)
	}
	if exists {
		log.Printf("[DEBUG] Event already exists, skipping: %s", event.SourceID)
		msg.Ack()
		return
	}

	log.Printf("[DEBUG] Broadcasting notification for category event")
	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed category event: %s for category %s", event.EventType, event.CategoryName)
}

func (s *Subscriber) handleTicketEvent(msg *nats.Msg) {
	var event models.TicketEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal ticket event: %v", err)
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed ticket event: %s for ticket %s", event.EventType, event.TicketNumber)
}

func (s *Subscriber) handleStaffEvent(msg *nats.Msg) {
	var event models.StaffEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal staff event: %v", err)
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed staff event: %s for staff %s", event.EventType, event.StaffName)
}

func (s *Subscriber) handleCouponEvent(msg *nats.Msg) {
	var event models.CouponEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal coupon event: %v", err)
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed coupon event: %s for coupon %s", event.EventType, event.CouponCode)
}

func (s *Subscriber) handleVendorEvent(msg *nats.Msg) {
	var event models.VendorEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal vendor event: %v", err)
		msg.Ack()
		return
	}

	exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
	if exists {
		msg.Ack()
		return
	}

	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed vendor event: %s for vendor %s", event.EventType, event.VendorName)
}

func (s *Subscriber) handleApprovalEvent(msg *nats.Msg) {
	var event models.ApprovalEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal approval event: %v", err)
		msg.Ack()
		return
	}

	// Only check dedup if SourceID is set (empty SourceID would match other empty records)
	if event.SourceID != "" {
		exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
		if exists {
			msg.Ack()
			return
		}
	}

	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed approval event: %s for %s", event.EventType, event.ResourceType)
}

func (s *Subscriber) handleGiftCardEvent(msg *nats.Msg) {
	var event models.GiftCardEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal gift card event: %v", err)
		msg.Ack()
		return
	}

	// Only check dedup if SourceID is set (empty SourceID would match other empty records)
	if event.SourceID != "" {
		exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
		if exists {
			msg.Ack()
			return
		}
	}

	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed gift card event: %s for gift card %s", event.EventType, event.GiftCardCode)
}

func (s *Subscriber) handleCampaignEvent(msg *nats.Msg) {
	var event models.CampaignEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal campaign event: %v", err)
		msg.Ack()
		return
	}

	if event.SourceID != "" {
		exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
		if exists {
			msg.Ack()
			return
		}
	}

	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed campaign event: %s for campaign %s", event.EventType, event.CampaignName)
}

func (s *Subscriber) handleLoyaltyEvent(msg *nats.Msg) {
	var event models.LoyaltyEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal loyalty event: %v", err)
		msg.Ack()
		return
	}

	if event.SourceID != "" {
		exists, _ := s.notifRepo.ExistsBySourceEventID(context.Background(), event.SourceID)
		if exists {
			msg.Ack()
			return
		}
	}

	s.broadcastNotification(&event, event.TenantID)
	msg.Ack()
	log.Printf("Processed loyalty event: %s for program %s", event.EventType, event.ProgramName)
}

// broadcastNotification is a helper that creates and broadcasts notifications to all connected users
func (s *Subscriber) broadcastNotification(event interface{}, tenantID string) {
	targetUsers := s.getTargetUsers(tenantID)
	log.Printf("[DEBUG] Target users for tenant %s: %d users", tenantID, len(targetUsers))

	if len(targetUsers) == 0 {
		// Store as broadcast notification (for when users connect later)
		log.Printf("[DEBUG] No connected users, creating broadcast notification")
		notification := models.EventToNotification(event, uuid.Nil)
		if notification != nil {
			if err := s.notifRepo.Create(context.Background(), notification); err != nil {
				log.Printf("Failed to create broadcast notification: %v", err)
			} else {
				log.Printf("[DEBUG] Broadcast notification created successfully (userID: nil)")
			}
		} else {
			log.Printf("[DEBUG] EventToNotification returned nil")
		}
	} else {
		for _, userID := range targetUsers {
			log.Printf("[DEBUG] Creating notification for user: %s", userID)
			notification := models.EventToNotification(event, userID)
			if notification == nil {
				log.Printf("[DEBUG] EventToNotification returned nil for user %s", userID)
				continue
			}
			if err := s.notifRepo.Create(context.Background(), notification); err != nil {
				log.Printf("Failed to create notification: %v", err)
				continue
			}
			log.Printf("[DEBUG] Notification created for user %s", userID)
			s.hub.BroadcastToUser(tenantID, userID, notification)
			count, _ := s.notifRepo.GetUnreadCount(context.Background(), tenantID, userID)
			s.hub.BroadcastUnreadCount(tenantID, userID, int(count))
		}
	}
}
