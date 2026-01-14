// Package templates provides email template rendering for the notification service.
// This file contains all order-related email render methods.
// All order notifications now use unified dynamic templates (order_customer.html).
package templates

import (
	"errors"
	"fmt"
)

// ErrMissingOrderNumber is returned when order number is empty.
var ErrMissingOrderNumber = errors.New("order number is required")

// Order status constants for unified templates
const (
	OrderStatusConfirmed = "CONFIRMED"
	OrderStatusShipped   = "SHIPPED"
	OrderStatusDelivered = "DELIVERED"
	OrderStatusCancelled = "CANCELLED"
	OrderStatusRefunded  = "REFUNDED"
)

// validateOrderData validates required order fields and sets defaults.
func validateOrderData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	if data.OrderNumber == "" {
		return ErrMissingOrderNumber
	}
	// Set defaults for optional fields
	if data.Currency == "" {
		data.Currency = "$"
	}
	return nil
}

// getOrderSubjectAndPreheader returns subject and preheader for customer-facing order emails.
func getOrderSubjectAndPreheader(status, orderNumber string) (string, string) {
	switch status {
	case OrderStatusConfirmed:
		return fmt.Sprintf("Order Confirmed - #%s", orderNumber),
			"Thank you for your purchase! Your order has been confirmed."
	case OrderStatusShipped:
		return fmt.Sprintf("Your Order is On Its Way - #%s", orderNumber),
			fmt.Sprintf("Great news! Order #%s has been shipped and is on its way.", orderNumber)
	case OrderStatusDelivered:
		return fmt.Sprintf("Your Order Has Arrived - #%s", orderNumber),
			fmt.Sprintf("Order #%s was delivered successfully!", orderNumber)
	case OrderStatusCancelled:
		return fmt.Sprintf("Order Cancelled - #%s", orderNumber),
			fmt.Sprintf("Order #%s has been cancelled. Refund processing.", orderNumber)
	case OrderStatusRefunded:
		return fmt.Sprintf("Refund Processed - #%s", orderNumber),
			fmt.Sprintf("Your refund for order #%s has been processed.", orderNumber)
	default:
		return fmt.Sprintf("Order Update - #%s", orderNumber),
			"Your order status has been updated."
	}
}

// RenderOrderCustomer renders a unified customer order notification.
// Set data.OrderStatus before calling (e.g., "CONFIRMED", "SHIPPED", "DELIVERED").
// Returns subject, body, and error.
func (r *Renderer) RenderOrderCustomer(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateOrderData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.Subject, data.Preheader = getOrderSubjectAndPreheader(data.OrderStatus, data.OrderNumber)

	body, err := r.Render("order_customer", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render order_customer: %w", err)
	}

	return data.Subject, body, nil
}

// ============================================================================
// Legacy methods - Now use unified templates internally
// These are kept for backwards compatibility with existing code.
// ============================================================================

// RenderOrderConfirmation renders the order confirmation notification.
func (r *Renderer) RenderOrderConfirmation(data *EmailData) (string, string, error) {
	data.OrderStatus = OrderStatusConfirmed
	return r.RenderOrderCustomer(data)
}

// RenderOrderShipped renders the order shipped notification.
func (r *Renderer) RenderOrderShipped(data *EmailData) (string, string, error) {
	data.OrderStatus = OrderStatusShipped
	return r.RenderOrderCustomer(data)
}

// RenderOrderDelivered renders the order delivered notification.
func (r *Renderer) RenderOrderDelivered(data *EmailData) (string, string, error) {
	data.OrderStatus = OrderStatusDelivered
	return r.RenderOrderCustomer(data)
}

// RenderOrderCancelled renders the order cancelled notification.
func (r *Renderer) RenderOrderCancelled(data *EmailData) (string, string, error) {
	data.OrderStatus = OrderStatusCancelled
	return r.RenderOrderCustomer(data)
}

// RenderOrderRefunded renders the order refunded notification.
func (r *Renderer) RenderOrderRefunded(data *EmailData) (string, string, error) {
	data.OrderStatus = OrderStatusRefunded
	return r.RenderOrderCustomer(data)
}

// RenderOrderStatusUpdate renders a generic order status update notification.
func (r *Renderer) RenderOrderStatusUpdate(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateOrderData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	// Use the provided status
	if data.OrderStatus == "" {
		data.OrderStatus = "UPDATED"
	}

	data.Subject, data.Preheader = getOrderSubjectAndPreheader(data.OrderStatus, data.OrderNumber)

	body, err := r.Render("order_customer", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render order_customer: %w", err)
	}

	return data.Subject, body, nil
}
