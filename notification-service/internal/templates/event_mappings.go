// Package templates provides email template rendering for the notification service.
// This file contains event-to-template mapping functions for NATS event processing.
package templates

// eventToTemplateMap maps NATS event types to customer-facing template names.
// This is a package-level variable to avoid recreating the map on each call.
var eventToTemplateMap = map[string]string{
	// Order events - all use unified dynamic template
	"order.created":   "order_customer",
	"order.confirmed": "order_customer",
	"order.shipped":   "order_customer",
	"order.delivered": "order_customer",
	"order.cancelled": "order_customer",
	"order.refunded":  "order_customer",

	// Payment events - all use unified dynamic template
	"payment.captured":  "payment_customer",
	"payment.succeeded": "payment_customer",
	"payment.failed":    "payment_customer",
	"payment.refunded":  "payment_customer",

	// Customer events
	"customer.registered": "customer_welcome",
	"customer.created":    "customer_welcome",

	// Inventory events
	"inventory.low_stock":    "low_stock_alert",
	"inventory.out_of_stock": "low_stock_alert",

	// Review events - all use unified dynamic template
	"review.created":  "review_customer",
	"review.approved": "review_customer",
	"review.rejected": "review_customer",

	// Ticket events - all use unified dynamic template
	"ticket.created":        "ticket_customer",
	"ticket.updated":        "ticket_customer",
	"ticket.status_changed": "ticket_customer",
	"ticket.resolved":       "ticket_customer",
	"ticket.closed":         "ticket_customer",
	"ticket.in_progress":    "ticket_customer",
	"ticket.on_hold":        "ticket_customer",
	"ticket.escalated":      "ticket_customer",
	"ticket.reopened":       "ticket_customer",
	"ticket.cancelled":      "ticket_customer",

	// Vendor events - all use unified dynamic template
	"vendor.created":   "vendor_customer",
	"vendor.approved":  "vendor_customer",
	"vendor.rejected":  "vendor_customer",
	"vendor.suspended": "vendor_customer",

	// Coupon events
	"coupon.applied": "coupon_applied",
	"coupon.created": "coupon_created",
	"coupon.expired": "coupon_expired",

	// Auth events
	"auth.password_reset": "password_reset",

	// Tenant events
	"tenant.created":                "tenant_welcome_pack",
	"tenant.onboarding.completed":   "tenant_welcome_pack",
	"tenant.verification.requested": "verification_link",

	// Approval workflow events - for requesters
	"approval.granted":   "approval_requester",
	"approval.rejected":  "approval_requester",
	"approval.cancelled": "approval_requester",
	"approval.expired":   "approval_requester",
}

// adminTemplateMap maps NATS event types to admin-facing template names.
// These templates are used for internal notifications to store administrators.
var adminTemplateMap = map[string]string{
	"review.created":         "review_submitted_admin",
	// Ticket events - all use unified dynamic admin template
	"ticket.created":         "ticket_admin",
	"ticket.resolved":        "ticket_admin",
	"ticket.closed":          "ticket_admin",
	"ticket.escalated":       "ticket_admin",
	"ticket.reopened":        "ticket_admin",
	"inventory.low_stock":    "low_stock_alert",
	"inventory.out_of_stock": "low_stock_alert",
	"vendor.created":         "vendor_application",
	"coupon.expired":         "coupon_expired",
	// Approval workflow events - for approvers
	"approval.requested": "approval_approver",
	"approval.escalated": "approval_approver",
}

// GetTemplateForEvent returns the appropriate customer-facing template name for a NATS event.
// Returns an empty string if no template is mapped for the event type.
func GetTemplateForEvent(eventType string) string {
	if eventType == "" {
		return ""
	}
	if template, ok := eventToTemplateMap[eventType]; ok {
		return template
	}
	return ""
}

// GetAdminTemplateForEvent returns admin notification template for events that need admin alerts.
// Returns an empty string if the event type does not require admin notification.
func GetAdminTemplateForEvent(eventType string) string {
	if eventType == "" {
		return ""
	}
	if template, ok := adminTemplateMap[eventType]; ok {
		return template
	}
	return ""
}

// IsValidEventType checks if the given event type has a mapped template.
func IsValidEventType(eventType string) bool {
	_, ok := eventToTemplateMap[eventType]
	return ok
}

// HasAdminTemplate checks if the given event type requires admin notification.
func HasAdminTemplate(eventType string) bool {
	_, ok := adminTemplateMap[eventType]
	return ok
}

// GetAllEventTypes returns a list of all supported event types.
func GetAllEventTypes() []string {
	types := make([]string, 0, len(eventToTemplateMap))
	for eventType := range eventToTemplateMap {
		types = append(types, eventType)
	}
	return types
}

// GetAllAdminEventTypes returns a list of event types that require admin notification.
func GetAllAdminEventTypes() []string {
	types := make([]string, 0, len(adminTemplateMap))
	for eventType := range adminTemplateMap {
		types = append(types, eventType)
	}
	return types
}
