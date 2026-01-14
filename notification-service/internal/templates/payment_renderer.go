// Package templates provides email template rendering for the notification service.
// This file contains all payment-related email render methods using the unified payment_customer template.
package templates

import (
	"errors"
	"fmt"
)

// Payment status constants for the unified payment_customer template
const (
	PaymentStatusCaptured = "CAPTURED"
	PaymentStatusFailed   = "FAILED"
	PaymentStatusRefunded = "REFUNDED"
)

// validatePaymentData validates required payment fields and sets defaults.
func validatePaymentData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	// Set default currency if not provided
	if data.Currency == "" {
		data.Currency = "$"
	}
	return nil
}

// RenderPaymentCustomer renders the unified payment customer template.
// The PaymentStatus field determines the content displayed.
// Returns subject, body, and error.
func (r *Renderer) RenderPaymentCustomer(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validatePaymentData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	// Set subject and preheader based on payment status
	switch data.PaymentStatus {
	case PaymentStatusCaptured:
		if data.Amount != "" {
			data.Subject = fmt.Sprintf("Payment Received - %s%s", data.Currency, data.Amount)
			data.Preheader = fmt.Sprintf("We've received your payment of %s%s. Thank you!", data.Currency, data.Amount)
		} else {
			data.Subject = "Payment Received - Thank You!"
			data.Preheader = "We've received your payment. Thank you for your order!"
		}
	case PaymentStatusFailed:
		data.Subject = "Payment Failed - Action Required"
		data.Preheader = "We couldn't process your payment. Please update your payment method."
	case PaymentStatusRefunded:
		if data.RefundAmount != "" {
			data.Subject = fmt.Sprintf("Refund Processed - %s%s", data.Currency, data.RefundAmount)
			data.Preheader = fmt.Sprintf("Your refund of %s%s has been processed successfully.", data.Currency, data.RefundAmount)
		} else if data.Amount != "" {
			data.Subject = fmt.Sprintf("Refund Processed - %s%s", data.Currency, data.Amount)
			data.Preheader = fmt.Sprintf("Your refund of %s%s has been processed successfully.", data.Currency, data.Amount)
		} else {
			data.Subject = "Refund Processed"
			data.Preheader = "Your refund has been processed successfully."
		}
	default:
		data.Subject = "Payment Update"
		data.Preheader = "Here's an update on your payment."
	}

	body, err := r.Render("payment_customer", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render payment_customer: %w", err)
	}

	return data.Subject, body, nil
}

// RenderPaymentConfirmation renders the payment confirmation email.
// This is a convenience wrapper around RenderPaymentCustomer with status CAPTURED.
// Returns subject, body, and error.
func (r *Renderer) RenderPaymentConfirmation(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validatePaymentData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.PaymentStatus = PaymentStatusCaptured
	return r.RenderPaymentCustomer(data)
}

// RenderPaymentFailed renders the payment failed email.
// This is a convenience wrapper around RenderPaymentCustomer with status FAILED.
// Returns subject, body, and error.
func (r *Renderer) RenderPaymentFailed(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validatePaymentData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.PaymentStatus = PaymentStatusFailed
	return r.RenderPaymentCustomer(data)
}

// RenderPaymentRefunded renders the payment refunded email.
// This is a convenience wrapper around RenderPaymentCustomer with status REFUNDED.
// Returns subject, body, and error.
func (r *Renderer) RenderPaymentRefunded(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validatePaymentData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.PaymentStatus = PaymentStatusRefunded
	return r.RenderPaymentCustomer(data)
}
