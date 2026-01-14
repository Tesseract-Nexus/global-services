// Package templates provides email template rendering for the notification service.
// This file contains all vendor-related email render methods using the unified vendor_customer template.
package templates

import (
	"errors"
	"fmt"
)

// Vendor status constants for the unified vendor_customer template
const (
	VendorStatusApplied   = "APPLIED"
	VendorStatusApproved  = "APPROVED"
	VendorStatusRejected  = "REJECTED"
	VendorStatusSuspended = "SUSPENDED"
)

// ErrMissingVendorName is returned when vendor name is empty.
var ErrMissingVendorName = errors.New("vendor name is required")

// validateVendorData validates required vendor fields.
func validateVendorData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	if data.VendorName == "" && data.VendorBusinessName == "" {
		return ErrMissingVendorName
	}
	// Use VendorBusinessName as fallback for VendorName
	if data.VendorName == "" {
		data.VendorName = data.VendorBusinessName
	}
	return nil
}

// RenderVendorCustomer renders the unified vendor customer template.
// The VendorStatus field determines the content displayed.
// Returns subject, body, and error.
func (r *Renderer) RenderVendorCustomer(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateVendorData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	displayName := data.BusinessName
	if displayName == "" {
		displayName = data.VendorName
	}

	// Set subject and preheader based on vendor status
	switch data.VendorStatus {
	case VendorStatusApplied:
		if data.BusinessName != "" {
			data.Subject = fmt.Sprintf("Welcome to %s, %s!", data.BusinessName, data.VendorName)
		} else {
			data.Subject = fmt.Sprintf("Welcome, %s!", data.VendorName)
		}
		data.Preheader = "Your vendor application has been received. We'll review it shortly."
	case VendorStatusApproved:
		data.Subject = fmt.Sprintf("Congratulations! You're Approved - %s", displayName)
		data.Preheader = "Great news! Your vendor application has been approved. Start selling today!"
	case VendorStatusRejected:
		data.Subject = fmt.Sprintf("Vendor Application Update - %s", displayName)
		data.Preheader = "We've reviewed your vendor application."
	case VendorStatusSuspended:
		data.Subject = fmt.Sprintf("Account Suspended - %s", displayName)
		data.Preheader = "Your vendor account has been temporarily suspended."
	default:
		data.Subject = fmt.Sprintf("Vendor Update - %s", displayName)
		data.Preheader = "Here's an update on your vendor account."
	}

	body, err := r.Render("vendor_customer", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render vendor_customer: %w", err)
	}

	return data.Subject, body, nil
}

// RenderVendorApplication renders the vendor application admin notification.
// Returns subject, body, and error.
func (r *Renderer) RenderVendorApplication(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateVendorData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.Subject = fmt.Sprintf("New Vendor Application - %s", data.VendorName)
	data.Preheader = fmt.Sprintf("A new vendor %s has applied to join your marketplace.", data.VendorName)

	body, err := r.Render("vendor_application", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render vendor_application: %w", err)
	}

	return data.Subject, body, nil
}

// RenderVendorWelcome renders the vendor welcome email.
// This is a convenience wrapper around RenderVendorCustomer with status APPLIED.
// Returns subject, body, and error.
func (r *Renderer) RenderVendorWelcome(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateVendorData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.VendorStatus = VendorStatusApplied
	return r.RenderVendorCustomer(data)
}

// RenderVendorApproved renders the vendor approval notification.
// This is a convenience wrapper around RenderVendorCustomer with status APPROVED.
// Returns subject, body, and error.
func (r *Renderer) RenderVendorApproved(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateVendorData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.VendorStatus = VendorStatusApproved
	return r.RenderVendorCustomer(data)
}

// RenderVendorRejected renders the vendor rejection notification.
// This is a convenience wrapper around RenderVendorCustomer with status REJECTED.
// Returns subject, body, and error.
func (r *Renderer) RenderVendorRejected(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateVendorData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.VendorStatus = VendorStatusRejected
	return r.RenderVendorCustomer(data)
}

// RenderVendorSuspended renders the vendor suspension notification.
// This is a convenience wrapper around RenderVendorCustomer with status SUSPENDED.
// Returns subject, body, and error.
func (r *Renderer) RenderVendorSuspended(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateVendorData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.VendorStatus = VendorStatusSuspended
	return r.RenderVendorCustomer(data)
}
