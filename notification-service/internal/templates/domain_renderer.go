// Package templates provides email template rendering for the notification service.
// This file contains all domain-related email render methods using the unified domain_customer template.
package templates

import (
	"errors"
	"fmt"
)

// Domain status constants for the unified domain_customer template
const (
	DomainStatusAdded        = "ADDED"
	DomainStatusVerified     = "VERIFIED"
	DomainStatusSSLReady     = "SSL_READY"
	DomainStatusActivated    = "ACTIVATED"
	DomainStatusFailed       = "FAILED"
	DomainStatusRemoved      = "REMOVED"
	DomainStatusMigrated     = "MIGRATED"
	DomainStatusSSLExpiring  = "SSL_EXPIRING"
	DomainStatusHealthFailed = "HEALTH_FAILED"
)

// ErrMissingDomain is returned when domain is empty.
var ErrMissingDomain = errors.New("domain is required")

// validateDomainData validates required domain fields.
func validateDomainData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	if data.Domain == "" {
		return ErrMissingDomain
	}
	return nil
}

// RenderDomainCustomer renders the unified domain customer template.
// The DomainStatus field determines the content displayed.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainCustomer(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	displayName := data.BusinessName
	if displayName == "" {
		displayName = data.Domain
	}

	// Set subject and preheader based on domain status
	switch data.DomainStatus {
	case DomainStatusAdded:
		data.Subject = fmt.Sprintf("Domain Added - Verify %s", data.Domain)
		data.Preheader = "Your custom domain has been added. Complete DNS verification to activate it."
	case DomainStatusVerified:
		data.Subject = fmt.Sprintf("DNS Verified - %s", data.Domain)
		data.Preheader = "Your domain's DNS records have been verified. SSL provisioning is in progress."
	case DomainStatusSSLReady:
		data.Subject = fmt.Sprintf("SSL Certificate Ready - %s", data.Domain)
		data.Preheader = "Your domain now has a valid SSL certificate. Your site is secure!"
	case DomainStatusActivated:
		data.Subject = fmt.Sprintf("Domain Active - %s is Live!", data.Domain)
		data.Preheader = "Great news! Your custom domain is now fully active and serving traffic."
	case DomainStatusFailed:
		data.Subject = fmt.Sprintf("Domain Setup Issue - %s", data.Domain)
		data.Preheader = "There was an issue setting up your domain. Action required."
	case DomainStatusRemoved:
		data.Subject = fmt.Sprintf("Domain Removed - %s", data.Domain)
		data.Preheader = "Your custom domain has been removed from your store."
	case DomainStatusMigrated:
		data.Subject = fmt.Sprintf("Domain Migrated - %s", data.Domain)
		data.Preheader = "Your domain has been successfully migrated to our new infrastructure."
	case DomainStatusSSLExpiring:
		data.Subject = fmt.Sprintf("SSL Certificate Expiring Soon - %s", data.Domain)
		data.Preheader = "Your domain's SSL certificate is expiring soon. Action may be required."
	case DomainStatusHealthFailed:
		data.Subject = fmt.Sprintf("Domain Health Alert - %s", data.Domain)
		data.Preheader = "We detected an issue with your domain. Please check your DNS settings."
	default:
		data.Subject = fmt.Sprintf("Domain Update - %s", data.Domain)
		data.Preheader = "Here's an update on your custom domain."
	}

	body, err := r.Render("domain_customer", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render domain_customer: %w", err)
	}

	return data.Subject, body, nil
}

// RenderDomainAdded renders the domain added notification.
// This is a convenience wrapper around RenderDomainCustomer with status ADDED.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainAdded(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.DomainStatus = DomainStatusAdded
	return r.RenderDomainCustomer(data)
}

// RenderDomainVerified renders the domain verified notification.
// This is a convenience wrapper around RenderDomainCustomer with status VERIFIED.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainVerified(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.DomainStatus = DomainStatusVerified
	return r.RenderDomainCustomer(data)
}

// RenderDomainSSLReady renders the SSL certificate ready notification.
// This is a convenience wrapper around RenderDomainCustomer with status SSL_READY.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainSSLReady(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.DomainStatus = DomainStatusSSLReady
	return r.RenderDomainCustomer(data)
}

// RenderDomainActivated renders the domain activated notification.
// This is a convenience wrapper around RenderDomainCustomer with status ACTIVATED.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainActivated(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.DomainStatus = DomainStatusActivated
	return r.RenderDomainCustomer(data)
}

// RenderDomainFailed renders the domain setup failure notification.
// This is a convenience wrapper around RenderDomainCustomer with status FAILED.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainFailed(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.DomainStatus = DomainStatusFailed
	return r.RenderDomainCustomer(data)
}

// RenderDomainRemoved renders the domain removed notification.
// This is a convenience wrapper around RenderDomainCustomer with status REMOVED.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainRemoved(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.DomainStatus = DomainStatusRemoved
	return r.RenderDomainCustomer(data)
}

// RenderDomainMigrated renders the domain migrated notification.
// This is a convenience wrapper around RenderDomainCustomer with status MIGRATED.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainMigrated(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.DomainStatus = DomainStatusMigrated
	return r.RenderDomainCustomer(data)
}

// RenderDomainSSLExpiring renders the SSL expiring soon notification.
// This is a convenience wrapper around RenderDomainCustomer with status SSL_EXPIRING.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainSSLExpiring(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.DomainStatus = DomainStatusSSLExpiring
	return r.RenderDomainCustomer(data)
}

// RenderDomainHealthFailed renders the domain health check failure notification.
// This is a convenience wrapper around RenderDomainCustomer with status HEALTH_FAILED.
// Returns subject, body, and error.
func (r *Renderer) RenderDomainHealthFailed(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateDomainData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.DomainStatus = DomainStatusHealthFailed
	return r.RenderDomainCustomer(data)
}
