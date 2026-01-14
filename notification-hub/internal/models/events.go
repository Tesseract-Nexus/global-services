package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event subjects
const (
	// Order events
	SubjectOrderCreated       = "order.created"
	SubjectOrderStatusChanged = "order.status_changed"
	SubjectOrderCancelled     = "order.cancelled"
	SubjectOrderShipped       = "order.shipped"
	SubjectOrderDelivered     = "order.delivered"

	// Payment events
	SubjectPaymentCaptured = "payment.captured"
	SubjectPaymentFailed   = "payment.failed"
	SubjectPaymentRefunded = "payment.refunded"

	// Inventory events
	SubjectInventoryLowStock = "inventory.low_stock"
	SubjectInventoryOutOfStock = "inventory.out_of_stock"

	// Customer events
	SubjectCustomerRegistered = "customer.registered"

	// Return events
	SubjectReturnRequested = "return.requested"
	SubjectReturnApproved  = "return.approved"
	SubjectReturnRejected  = "return.rejected"

	// Review events
	SubjectReviewSubmitted = "review.submitted"
	SubjectReviewApproved  = "review.approved"
)

// Stream names
const (
	StreamOrderEvents     = "ORDER_EVENTS"
	StreamPaymentEvents   = "PAYMENT_EVENTS"
	StreamInventoryEvents = "INVENTORY_EVENTS"
	StreamCustomerEvents  = "CUSTOMER_EVENTS"
	StreamReturnEvents    = "RETURN_EVENTS"
	StreamReviewEvents    = "REVIEW_EVENTS"
)

// BaseEvent is the common structure for all events
// NOTE: JSON field names must match go-shared/events library (camelCase)
type BaseEvent struct {
	EventType string    `json:"eventType"`
	TenantID  string    `json:"tenantId"`
	Timestamp time.Time `json:"timestamp"`
	SourceID  string    `json:"sourceId"` // Message ID for deduplication
}

// OrderEvent represents order-related events
type OrderEvent struct {
	BaseEvent
	OrderID     string  `json:"orderId"`
	OrderNumber string  `json:"orderNumber"`
	CustomerID  string  `json:"customerId"`
	Status      string  `json:"status"`
	Total       float64 `json:"totalAmount"`
	Currency    string  `json:"currency"`
}

// PaymentEvent represents payment-related events
type PaymentEvent struct {
	BaseEvent
	PaymentID string  `json:"paymentId"`
	OrderID   string  `json:"orderId"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Status    string  `json:"status"`
	Method    string  `json:"method"`
}

// InventoryEvent represents inventory-related events
type InventoryEvent struct {
	BaseEvent
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	SKU         string `json:"sku"`
	Quantity    int    `json:"currentStock"`
	Threshold   int    `json:"reorderPoint"`
}

// CustomerEvent represents customer-related events
type CustomerEvent struct {
	BaseEvent
	CustomerID string `json:"customerId"`
	Email      string `json:"customerEmail"`
	Name       string `json:"customerName"`
}

// ReturnEvent represents return-related events
type ReturnEvent struct {
	BaseEvent
	ReturnID    string  `json:"returnId"`
	OrderID     string  `json:"orderId"`
	OrderNumber string  `json:"orderNumber"`
	CustomerID  string  `json:"customerId"`
	Reason      string  `json:"reason"`
	Amount      float64 `json:"refundAmount"`
	Status      string  `json:"status"`
}

// ReviewEvent represents review-related events
type ReviewEvent struct {
	BaseEvent
	ReviewID    string `json:"reviewId"`
	ProductID   string `json:"productId"`
	ProductName string `json:"productName"`
	CustomerID  string `json:"customerId"`
	Rating      int    `json:"rating"`
}

// EventToNotification converts an event to a notification
func EventToNotification(event interface{}, targetUserID uuid.UUID) *Notification {
	switch e := event.(type) {
	case *OrderEvent:
		return orderEventToNotification(e, targetUserID)
	case *PaymentEvent:
		return paymentEventToNotification(e, targetUserID)
	case *InventoryEvent:
		return inventoryEventToNotification(e, targetUserID)
	case *CustomerEvent:
		return customerEventToNotification(e, targetUserID)
	case *ReturnEvent:
		return returnEventToNotification(e, targetUserID)
	case *ReviewEvent:
		return reviewEventToNotification(e, targetUserID)
	default:
		return nil
	}
}

func orderEventToNotification(e *OrderEvent, userID uuid.UUID) *Notification {
	var title, message, icon string
	priority := PriorityNormal
	formattedAmount := formatCurrency(e.Currency, e.Total)

	switch e.EventType {
	case SubjectOrderCreated:
		title = "🛒 New Order Received"
		message = "Order " + e.OrderNumber + " for " + formattedAmount + " has been placed and needs processing"
		icon = "shopping-cart"
		priority = PriorityHigh
	case SubjectOrderStatusChanged:
		title = "📦 Order Status Updated"
		message = "Order " + e.OrderNumber + " status changed to: " + formatStatus(e.Status)
		icon = "package"
	case SubjectOrderCancelled:
		title = "❌ Order Cancelled"
		message = "Order " + e.OrderNumber + " (" + formattedAmount + ") has been cancelled"
		icon = "x-circle"
		priority = PriorityHigh
	case SubjectOrderShipped:
		title = "🚚 Order Shipped"
		message = "Order " + e.OrderNumber + " is now on its way to the customer"
		icon = "truck"
	case SubjectOrderDelivered:
		title = "✅ Order Delivered"
		message = "Order " + e.OrderNumber + " has been successfully delivered"
		icon = "check-circle"
	}

	orderID, _ := uuid.Parse(e.OrderID)
	return &Notification{
		TenantID:      e.TenantID,
		UserID:        userID,
		Channel:       "in_app",
		Type:          e.EventType,
		Title:         title,
		Message:       message,
		Icon:          icon,
		ActionURL:     "/orders/" + e.OrderID,
		SourceService: "orders-service",
		SourceEventID: e.SourceID,
		EntityType:    "order",
		EntityID:      &orderID,
		Priority:      priority,
		GroupKey:      "order:" + e.OrderID,
		Metadata: JSONB{
			"orderNumber": e.OrderNumber,
			"total":       e.Total,
			"currency":    e.Currency,
			"status":      e.Status,
		},
	}
}

func paymentEventToNotification(e *PaymentEvent, userID uuid.UUID) *Notification {
	var title, message, icon string
	priority := PriorityNormal
	formattedAmount := formatCurrency(e.Currency, e.Amount)

	switch e.EventType {
	case SubjectPaymentCaptured:
		title = "💳 Payment Received"
		message = "Payment of " + formattedAmount + " has been successfully captured"
		icon = "credit-card"
		priority = PriorityHigh
	case SubjectPaymentFailed:
		title = "⚠️ Payment Failed"
		message = "Payment of " + formattedAmount + " failed - action required"
		icon = "alert-circle"
		priority = PriorityUrgent
	case SubjectPaymentRefunded:
		title = "💸 Refund Processed"
		message = "Refund of " + formattedAmount + " has been processed"
		icon = "rotate-ccw"
	}

	paymentID, _ := uuid.Parse(e.PaymentID)
	return &Notification{
		TenantID:      e.TenantID,
		UserID:        userID,
		Channel:       "in_app",
		Type:          e.EventType,
		Title:         title,
		Message:       message,
		Icon:          icon,
		ActionURL:     "/orders/" + e.OrderID,
		SourceService: "payment-service",
		SourceEventID: e.SourceID,
		EntityType:    "payment",
		EntityID:      &paymentID,
		Priority:      priority,
		GroupKey:      "payment:" + e.PaymentID,
		Metadata: JSONB{
			"amount":   e.Amount,
			"currency": e.Currency,
			"method":   e.Method,
			"orderId":  e.OrderID,
		},
	}
}

func inventoryEventToNotification(e *InventoryEvent, userID uuid.UUID) *Notification {
	var title, message, icon string
	priority := PriorityNormal

	switch e.EventType {
	case SubjectInventoryLowStock:
		title = "📉 Low Stock Alert"
		message = "\"" + e.ProductName + "\" is running low - only " + formatInt(e.Quantity) + " units remaining"
		icon = "alert-triangle"
		priority = PriorityHigh
	case SubjectInventoryOutOfStock:
		title = "🚫 Out of Stock"
		message = "\"" + e.ProductName + "\" is now out of stock - immediate attention needed"
		icon = "x-octagon"
		priority = PriorityUrgent
	}

	productID, _ := uuid.Parse(e.ProductID)
	return &Notification{
		TenantID:      e.TenantID,
		UserID:        userID,
		Channel:       "in_app",
		Type:          e.EventType,
		Title:         title,
		Message:       message,
		Icon:          icon,
		ActionURL:     "/inventory/" + e.ProductID,
		SourceService: "inventory-service",
		SourceEventID: e.SourceID,
		EntityType:    "product",
		EntityID:      &productID,
		Priority:      priority,
		GroupKey:      "inventory:" + e.ProductID,
		Metadata: JSONB{
			"productName": e.ProductName,
			"sku":         e.SKU,
			"quantity":    e.Quantity,
			"threshold":   e.Threshold,
		},
	}
}

func customerEventToNotification(e *CustomerEvent, userID uuid.UUID) *Notification {
	var title, message, icon string

	switch e.EventType {
	case SubjectCustomerRegistered:
		title = "👤 New Customer Registered"
		message = e.Name + " (" + e.Email + ") has joined your store"
		icon = "user-plus"
	}

	customerID, _ := uuid.Parse(e.CustomerID)
	return &Notification{
		TenantID:      e.TenantID,
		UserID:        userID,
		Channel:       "in_app",
		Type:          e.EventType,
		Title:         title,
		Message:       message,
		Icon:          icon,
		ActionURL:     "/customers/" + e.CustomerID,
		SourceService: "customers-service",
		SourceEventID: e.SourceID,
		EntityType:    "customer",
		EntityID:      &customerID,
		Priority:      PriorityNormal,
		GroupKey:      "customer:" + e.CustomerID,
		Metadata: JSONB{
			"email": e.Email,
			"name":  e.Name,
		},
	}
}

func returnEventToNotification(e *ReturnEvent, userID uuid.UUID) *Notification {
	var title, message, icon string
	priority := PriorityNormal

	switch e.EventType {
	case SubjectReturnRequested:
		title = "📦 Return Request Received"
		message = "Customer requested return for order " + e.OrderNumber + " - Reason: " + e.Reason
		icon = "package-x"
		priority = PriorityHigh
	case SubjectReturnApproved:
		title = "✅ Return Approved"
		message = "Return for order " + e.OrderNumber + " has been approved"
		icon = "check-circle"
	case SubjectReturnRejected:
		title = "❌ Return Rejected"
		message = "Return for order " + e.OrderNumber + " has been rejected"
		icon = "x-circle"
	}

	returnID, _ := uuid.Parse(e.ReturnID)
	return &Notification{
		TenantID:      e.TenantID,
		UserID:        userID,
		Channel:       "in_app",
		Type:          e.EventType,
		Title:         title,
		Message:       message,
		Icon:          icon,
		ActionURL:     "/returns/" + e.ReturnID,
		SourceService: "orders-service",
		SourceEventID: e.SourceID,
		EntityType:    "return",
		EntityID:      &returnID,
		Priority:      priority,
		GroupKey:      "return:" + e.ReturnID,
		Metadata: JSONB{
			"orderNumber": e.OrderNumber,
			"reason":      e.Reason,
			"amount":      e.Amount,
		},
	}
}

func reviewEventToNotification(e *ReviewEvent, userID uuid.UUID) *Notification {
	var title, message, icon string

	switch e.EventType {
	case SubjectReviewSubmitted:
		title = "⭐ New Product Review"
		message = formatInt(e.Rating) + "-star review submitted for \"" + e.ProductName + "\" - awaiting moderation"
		icon = "star"
	case SubjectReviewApproved:
		title = "✅ Review Published"
		message = "Review for \"" + e.ProductName + "\" has been approved and is now live"
		icon = "check"
	}

	reviewID, _ := uuid.Parse(e.ReviewID)
	return &Notification{
		TenantID:      e.TenantID,
		UserID:        userID,
		Channel:       "in_app",
		Type:          e.EventType,
		Title:         title,
		Message:       message,
		Icon:          icon,
		ActionURL:     "/reviews/" + e.ReviewID,
		SourceService: "reviews-service",
		SourceEventID: e.SourceID,
		EntityType:    "review",
		EntityID:      &reviewID,
		Priority:      PriorityNormal,
		GroupKey:      "review:" + e.ReviewID,
		Metadata: JSONB{
			"productName": e.ProductName,
			"rating":      e.Rating,
		},
	}
}

func formatAmount(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

func formatInt(n int) string {
	return fmt.Sprintf("%d", n)
}

// formatCurrency formats amount with currency symbol (Amazon/Tesla style)
func formatCurrency(currency string, amount float64) string {
	symbols := map[string]string{
		"USD": "$",
		"AUD": "A$",
		"EUR": "€",
		"GBP": "£",
		"CAD": "C$",
		"NZD": "NZ$",
		"JPY": "¥",
		"INR": "₹",
	}
	symbol := symbols[currency]
	if symbol == "" {
		symbol = currency + " "
	}
	return symbol + fmt.Sprintf("%.2f", amount)
}

// formatStatus converts status codes to human-readable format
func formatStatus(status string) string {
	statuses := map[string]string{
		"pending":    "Pending",
		"confirmed":  "Confirmed",
		"processing": "Processing",
		"shipped":    "Shipped",
		"delivered":  "Delivered",
		"cancelled":  "Cancelled",
		"refunded":   "Refunded",
	}
	if s, ok := statuses[status]; ok {
		return s
	}
	return status
}

// ==========================================
// Customer-facing notification converters
// ==========================================

// CustomerEventToNotification converts an event to a customer-facing notification
func CustomerEventToNotification(event interface{}, customerID uuid.UUID) *Notification {
	switch e := event.(type) {
	case *OrderEvent:
		return customerOrderEventToNotification(e, customerID)
	case *PaymentEvent:
		return customerPaymentEventToNotification(e, customerID)
	case *ReturnEvent:
		return customerReturnEventToNotification(e, customerID)
	default:
		return nil
	}
}

func customerOrderEventToNotification(e *OrderEvent, customerID uuid.UUID) *Notification {
	var title, message, icon string
	priority := PriorityNormal
	formattedAmount := formatCurrency(e.Currency, e.Total)

	switch e.EventType {
	case SubjectOrderCreated:
		title = "🎉 Order Confirmed!"
		message = "Your order " + e.OrderNumber + " for " + formattedAmount + " has been placed successfully"
		icon = "check-circle"
		priority = PriorityHigh
	case SubjectOrderStatusChanged:
		title = "📦 Order Update"
		message = "Your order " + e.OrderNumber + " status has been updated to: " + formatStatus(e.Status)
		icon = "package"
	case SubjectOrderCancelled:
		title = "❌ Order Cancelled"
		message = "Your order " + e.OrderNumber + " has been cancelled. Refund will be processed shortly."
		icon = "x-circle"
		priority = PriorityHigh
	case SubjectOrderShipped:
		title = "🚚 Your Order is On Its Way!"
		message = "Great news! Order " + e.OrderNumber + " has been shipped and is on its way to you"
		icon = "truck"
		priority = PriorityHigh
	case SubjectOrderDelivered:
		title = "✅ Order Delivered!"
		message = "Your order " + e.OrderNumber + " has been delivered. Enjoy your purchase!"
		icon = "check-circle"
		priority = PriorityHigh
	default:
		return nil
	}

	orderID, _ := uuid.Parse(e.OrderID)
	return &Notification{
		TenantID:      e.TenantID,
		UserID:        customerID,
		Channel:       "in_app",
		Type:          e.EventType,
		Title:         title,
		Message:       message,
		Icon:          icon,
		ActionURL:     "/account/orders/" + e.OrderID,
		SourceService: "orders-service",
		SourceEventID: e.SourceID + "-customer",
		EntityType:    "order",
		EntityID:      &orderID,
		Priority:      priority,
		GroupKey:      "customer-order:" + e.OrderID,
		Metadata: JSONB{
			"orderNumber": e.OrderNumber,
			"total":       e.Total,
			"currency":    e.Currency,
			"status":      e.Status,
		},
	}
}

func customerPaymentEventToNotification(e *PaymentEvent, customerID uuid.UUID) *Notification {
	var title, message, icon string
	priority := PriorityNormal
	formattedAmount := formatCurrency(e.Currency, e.Amount)

	switch e.EventType {
	case SubjectPaymentCaptured:
		title = "💳 Payment Successful"
		message = "Your payment of " + formattedAmount + " has been processed successfully"
		icon = "credit-card"
	case SubjectPaymentFailed:
		title = "⚠️ Payment Issue"
		message = "Your payment of " + formattedAmount + " could not be processed. Please update your payment method."
		icon = "alert-circle"
		priority = PriorityUrgent
	case SubjectPaymentRefunded:
		title = "💸 Refund Processed"
		message = "Your refund of " + formattedAmount + " has been processed. It may take 5-10 business days to appear."
		icon = "rotate-ccw"
		priority = PriorityHigh
	default:
		return nil
	}

	paymentID, _ := uuid.Parse(e.PaymentID)
	return &Notification{
		TenantID:      e.TenantID,
		UserID:        customerID,
		Channel:       "in_app",
		Type:          e.EventType,
		Title:         title,
		Message:       message,
		Icon:          icon,
		ActionURL:     "/account/orders/" + e.OrderID,
		SourceService: "payment-service",
		SourceEventID: e.SourceID + "-customer",
		EntityType:    "payment",
		EntityID:      &paymentID,
		Priority:      priority,
		GroupKey:      "customer-payment:" + e.PaymentID,
		Metadata: JSONB{
			"amount":   e.Amount,
			"currency": e.Currency,
			"method":   e.Method,
			"orderId":  e.OrderID,
		},
	}
}

func customerReturnEventToNotification(e *ReturnEvent, customerID uuid.UUID) *Notification {
	var title, message, icon string
	priority := PriorityNormal
	formattedAmount := formatCurrency("USD", e.Amount)

	switch e.EventType {
	case SubjectReturnRequested:
		title = "📦 Return Request Received"
		message = "Your return request for order " + e.OrderNumber + " has been submitted"
		icon = "package"
	case SubjectReturnApproved:
		title = "✅ Return Approved"
		message = "Your return for order " + e.OrderNumber + " has been approved. Refund of " + formattedAmount + " will be processed soon."
		icon = "check-circle"
		priority = PriorityHigh
	case SubjectReturnRejected:
		title = "❌ Return Not Approved"
		message = "Unfortunately, your return request for order " + e.OrderNumber + " could not be approved."
		icon = "x-circle"
		priority = PriorityHigh
	default:
		return nil
	}

	returnID, _ := uuid.Parse(e.ReturnID)
	return &Notification{
		TenantID:      e.TenantID,
		UserID:        customerID,
		Channel:       "in_app",
		Type:          e.EventType,
		Title:         title,
		Message:       message,
		Icon:          icon,
		ActionURL:     "/account/orders/" + e.OrderID,
		SourceService: "orders-service",
		SourceEventID: e.SourceID + "-customer",
		EntityType:    "return",
		EntityID:      &returnID,
		Priority:      priority,
		GroupKey:      "customer-return:" + e.ReturnID,
		Metadata: JSONB{
			"orderNumber": e.OrderNumber,
			"reason":      e.Reason,
			"amount":      e.Amount,
		},
	}
}
