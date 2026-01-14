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
		nats.DeliverNew(),
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
		nats.DeliverNew(),
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
		nats.DeliverNew(),
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
		nats.DeliverNew(),
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
		nats.DeliverNew(),
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
		nats.DeliverNew(),
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
