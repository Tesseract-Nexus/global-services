package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"notification-hub/internal/models"
	"notification-hub/internal/repository"
)

// DebugHandler handles debug/seed endpoints (non-production only)
type DebugHandler struct {
	notifRepo   repository.NotificationRepository
	environment string
}

// NewDebugHandler creates a new debug handler
func NewDebugHandler(notifRepo repository.NotificationRepository, environment string) *DebugHandler {
	return &DebugHandler{
		notifRepo:   notifRepo,
		environment: environment,
	}
}

// SeedNotifications creates test notifications for a user
func (h *DebugHandler) SeedNotifications(c *gin.Context) {
	// Only allow in non-production
	if h.environment == "production" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not available in production"})
		return
	}

	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	// Create sample notifications
	notifications := []models.Notification{
		{
			TenantID:      tenantID,
			UserID:        userID,
			Type:          "order.created",
			Title:         "New Order Received",
			Message:       "Order #ORD-2024-001 has been placed for $149.99",
			Icon:          "shopping-cart",
			ActionURL:     "/orders/ORD-2024-001",
			SourceService: "orders-service",
			SourceEventID: "seed-order-1-" + time.Now().Format("20060102150405"),
			EntityType:    "order",
			Priority:      models.PriorityHigh,
			GroupKey:      "order:ORD-2024-001",
			Metadata: models.JSONB{
				"orderNumber": "ORD-2024-001",
				"total":       149.99,
				"currency":    "USD",
			},
		},
		{
			TenantID:      tenantID,
			UserID:        userID,
			Type:          "payment.captured",
			Title:         "Payment Received",
			Message:       "Payment of $149.99 USD received via Credit Card",
			Icon:          "credit-card",
			ActionURL:     "/orders/ORD-2024-001",
			SourceService: "payment-service",
			SourceEventID: "seed-payment-1-" + time.Now().Format("20060102150405"),
			EntityType:    "payment",
			Priority:      models.PriorityHigh,
			GroupKey:      "payment:PAY-001",
			Metadata: models.JSONB{
				"amount":   149.99,
				"currency": "USD",
				"method":   "credit_card",
			},
		},
		{
			TenantID:      tenantID,
			UserID:        userID,
			Type:          "inventory.low_stock",
			Title:         "Low Stock Alert",
			Message:       "Premium Wireless Headphones has only 5 units left",
			Icon:          "alert-triangle",
			ActionURL:     "/inventory/PROD-001",
			SourceService: "inventory-service",
			SourceEventID: "seed-inventory-1-" + time.Now().Format("20060102150405"),
			EntityType:    "product",
			Priority:      models.PriorityHigh,
			GroupKey:      "inventory:PROD-001",
			Metadata: models.JSONB{
				"productName": "Premium Wireless Headphones",
				"sku":         "HDPH-001",
				"quantity":    5,
				"threshold":   10,
			},
		},
		{
			TenantID:      tenantID,
			UserID:        userID,
			Type:          "customer.registered",
			Title:         "New Customer",
			Message:       "John Smith has registered and created an account",
			Icon:          "user-plus",
			ActionURL:     "/customers/CUST-001",
			SourceService: "customers-service",
			SourceEventID: "seed-customer-1-" + time.Now().Format("20060102150405"),
			EntityType:    "customer",
			Priority:      models.PriorityNormal,
			GroupKey:      "customer:CUST-001",
			Metadata: models.JSONB{
				"name":  "John Smith",
				"email": "john.smith@example.com",
			},
		},
		{
			TenantID:      tenantID,
			UserID:        userID,
			Type:          "return.requested",
			Title:         "Return Request",
			Message:       "Return requested for order #ORD-2024-002 - Damaged product",
			Icon:          "package-x",
			ActionURL:     "/returns/RET-001",
			SourceService: "orders-service",
			SourceEventID: "seed-return-1-" + time.Now().Format("20060102150405"),
			EntityType:    "return",
			Priority:      models.PriorityHigh,
			GroupKey:      "return:RET-001",
			Metadata: models.JSONB{
				"orderNumber": "ORD-2024-002",
				"reason":      "Damaged product",
				"amount":      79.99,
			},
		},
		{
			TenantID:      tenantID,
			UserID:        userID,
			Type:          "review.submitted",
			Title:         "New Review",
			Message:       "5-star review on Ultra HD Smart TV",
			Icon:          "star",
			ActionURL:     "/reviews/REV-001",
			SourceService: "reviews-service",
			SourceEventID: "seed-review-1-" + time.Now().Format("20060102150405"),
			EntityType:    "review",
			Priority:      models.PriorityNormal,
			GroupKey:      "review:REV-001",
			Metadata: models.JSONB{
				"productName": "Ultra HD Smart TV",
				"rating":      5,
			},
		},
		{
			TenantID:      tenantID,
			UserID:        userID,
			Type:          "order.shipped",
			Title:         "Order Shipped",
			Message:       "Order #ORD-2024-003 is on its way",
			Icon:          "truck",
			ActionURL:     "/orders/ORD-2024-003",
			SourceService: "orders-service",
			SourceEventID: "seed-order-2-" + time.Now().Format("20060102150405"),
			EntityType:    "order",
			Priority:      models.PriorityNormal,
			GroupKey:      "order:ORD-2024-003",
			Metadata: models.JSONB{
				"orderNumber": "ORD-2024-003",
				"status":      "shipped",
			},
		},
		{
			TenantID:      tenantID,
			UserID:        userID,
			Type:          "payment.failed",
			Title:         "Payment Failed",
			Message:       "Payment of $299.99 USD failed - Card declined",
			Icon:          "alert-circle",
			ActionURL:     "/orders/ORD-2024-004",
			SourceService: "payment-service",
			SourceEventID: "seed-payment-2-" + time.Now().Format("20060102150405"),
			EntityType:    "payment",
			Priority:      models.PriorityUrgent,
			GroupKey:      "payment:PAY-002",
			Metadata: models.JSONB{
				"amount":   299.99,
				"currency": "USD",
				"method":   "credit_card",
				"error":    "Card declined",
			},
		},
	}

	created := 0
	for i := range notifications {
		// Check for duplicates
		exists, _ := h.notifRepo.ExistsBySourceEventID(context.Background(), notifications[i].SourceEventID)
		if exists {
			continue
		}

		if err := h.notifRepo.Create(context.Background(), &notifications[i]); err != nil {
			continue
		}
		created++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Seed notifications created",
		"created": created,
		"total":   len(notifications),
	})
}

// ClearNotifications deletes all notifications for a user (debug only)
func (h *DebugHandler) ClearNotifications(c *gin.Context) {
	// Only allow in non-production
	if h.environment == "production" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not available in production"})
		return
	}

	tenantID := c.GetString("tenant_id")
	userIDStr := c.GetString("user_id")
	if tenantID == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id or user_id"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	deleted, err := h.notifRepo.DeleteAll(context.Background(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All notifications cleared",
		"deleted": deleted,
	})
}
