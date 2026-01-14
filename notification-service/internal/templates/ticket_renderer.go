// Package templates provides email template rendering for the notification service.
// This file contains all ticket-related email render methods.
// All ticket notifications now use unified dynamic templates (ticket_customer.html and ticket_admin.html).
package templates

import (
	"errors"
	"fmt"
)

// ErrNilRenderer is returned when a render method is called on a nil Renderer.
var ErrNilRenderer = errors.New("renderer is nil")

// ErrMissingTicketNumber is returned when ticket number is empty.
var ErrMissingTicketNumber = errors.New("ticket number is required")

// Ticket status constants for unified templates
const (
	TicketStatusCreated    = "CREATED"
	TicketStatusInProgress = "IN_PROGRESS"
	TicketStatusOnHold     = "ON_HOLD"
	TicketStatusEscalated  = "ESCALATED"
	TicketStatusResolved   = "RESOLVED"
	TicketStatusReopened   = "REOPENED"
	TicketStatusClosed     = "CLOSED"
	TicketStatusCancelled  = "CANCELLED"
)

// validateTicketData validates required ticket fields and sets defaults.
func validateTicketData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	if data.TicketNumber == "" {
		return ErrMissingTicketNumber
	}
	// Set defaults for optional fields
	if data.TicketPriority == "" {
		data.TicketPriority = "Normal"
	}
	return nil
}

// getCustomerSubjectAndPreheader returns subject and preheader for customer-facing ticket emails.
func getCustomerSubjectAndPreheader(status, ticketNumber, ticketSubject string) (string, string) {
	switch status {
	case TicketStatusCreated:
		subject := fmt.Sprintf("Support Ticket Created - #%s", ticketNumber)
		preheader := "We've received your support request and will respond shortly."
		if ticketSubject != "" {
			preheader = fmt.Sprintf("We've received your support request: %s", ticketSubject)
		}
		return subject, preheader
	case TicketStatusInProgress:
		return fmt.Sprintf("Work Has Started on Your Ticket - #%s", ticketNumber),
			fmt.Sprintf("Good news! A support agent is now working on your ticket #%s.", ticketNumber)
	case TicketStatusOnHold:
		return fmt.Sprintf("Your Support Ticket Is On Hold - #%s", ticketNumber),
			fmt.Sprintf("Your ticket #%s has been placed on hold. Action may be required.", ticketNumber)
	case TicketStatusEscalated:
		return fmt.Sprintf("Your Support Ticket Has Been Escalated - #%s", ticketNumber),
			fmt.Sprintf("Your ticket #%s is now receiving priority attention from our senior team.", ticketNumber)
	case TicketStatusResolved:
		return fmt.Sprintf("Ticket Resolved - #%s", ticketNumber),
			"Good news! Your support ticket has been resolved."
	case TicketStatusReopened:
		return fmt.Sprintf("Your Support Ticket Has Been Reopened - #%s", ticketNumber),
			fmt.Sprintf("Your ticket #%s has been reopened and our team will continue working on it.", ticketNumber)
	case TicketStatusClosed:
		return fmt.Sprintf("Ticket Closed - #%s", ticketNumber),
			"Your support ticket has been closed. We hope we were able to help!"
	case TicketStatusCancelled:
		return fmt.Sprintf("Your Support Ticket Has Been Cancelled - #%s", ticketNumber),
			fmt.Sprintf("Ticket #%s has been cancelled. You can open a new ticket if needed.", ticketNumber)
	default:
		return fmt.Sprintf("Ticket Update - #%s", ticketNumber),
			"Your support ticket has been updated."
	}
}

// getAdminSubjectAndPreheader returns subject and preheader for admin-facing ticket emails.
func getAdminSubjectAndPreheader(status, ticketNumber, priority, customerName, ticketSubject, assignedTo string) (string, string) {
	switch status {
	case TicketStatusCreated:
		subject := fmt.Sprintf("[%s] New Support Ticket - #%s", priority, ticketNumber)
		preheader := fmt.Sprintf("New support ticket: %s", ticketSubject)
		if customerName != "" {
			preheader = fmt.Sprintf("New ticket from %s: %s", customerName, ticketSubject)
		}
		return subject, preheader
	case TicketStatusEscalated:
		return fmt.Sprintf("[ESCALATED] Urgent Attention Required - Ticket #%s", ticketNumber),
			fmt.Sprintf("Ticket #%s has been escalated and requires immediate attention.", ticketNumber)
	case TicketStatusResolved:
		preheader := fmt.Sprintf("Ticket #%s has been resolved", ticketNumber)
		if assignedTo != "" {
			preheader = fmt.Sprintf("Ticket #%s has been resolved by %s", ticketNumber, assignedTo)
		}
		return fmt.Sprintf("Ticket Resolved - #%s", ticketNumber), preheader
	case TicketStatusReopened:
		return fmt.Sprintf("Ticket Reopened - #%s", ticketNumber),
			fmt.Sprintf("Ticket #%s has been reopened and needs follow-up.", ticketNumber)
	case TicketStatusClosed:
		preheader := fmt.Sprintf("Ticket #%s has been closed", ticketNumber)
		if assignedTo != "" {
			preheader = fmt.Sprintf("Ticket #%s has been closed by %s", ticketNumber, assignedTo)
		}
		return fmt.Sprintf("Ticket Closed - #%s", ticketNumber), preheader
	default:
		return fmt.Sprintf("Ticket Update - #%s", ticketNumber),
			fmt.Sprintf("Ticket #%s has been updated.", ticketNumber)
	}
}

// RenderTicketCustomer renders a unified customer ticket notification.
// Set data.TicketStatus before calling (e.g., "CREATED", "RESOLVED", "ESCALATED").
// Returns subject, body, and error.
func (r *Renderer) RenderTicketCustomer(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateTicketData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.Subject, data.Preheader = getCustomerSubjectAndPreheader(data.TicketStatus, data.TicketNumber, data.TicketSubject)

	body, err := r.Render("ticket_customer", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render ticket_customer: %w", err)
	}

	return data.Subject, body, nil
}

// RenderTicketAdmin renders a unified admin ticket notification.
// Set data.TicketStatus before calling (e.g., "CREATED", "ESCALATED", "RESOLVED").
// Returns subject, body, and error.
func (r *Renderer) RenderTicketAdmin(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateTicketData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.Subject, data.Preheader = getAdminSubjectAndPreheader(
		data.TicketStatus, data.TicketNumber, data.TicketPriority,
		data.CustomerName, data.TicketSubject, data.AssignedToName,
	)

	body, err := r.Render("ticket_admin", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render ticket_admin: %w", err)
	}

	return data.Subject, body, nil
}

// ============================================================================
// Legacy methods - Now use unified templates internally
// These are kept for backwards compatibility with existing code.
// ============================================================================

// RenderTicketCreated renders the ticket created customer notification.
func (r *Renderer) RenderTicketCreated(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusCreated
	return r.RenderTicketCustomer(data)
}

// RenderTicketCreatedAdmin renders the ticket created admin notification.
func (r *Renderer) RenderTicketCreatedAdmin(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusCreated
	return r.RenderTicketAdmin(data)
}

// RenderTicketUpdated renders the ticket status updated notification.
func (r *Renderer) RenderTicketUpdated(data *EmailData) (string, string, error) {
	// Use the current status or default
	if data.TicketStatus == "" {
		data.TicketStatus = "UPDATED"
	}
	return r.RenderTicketCustomer(data)
}

// RenderTicketResolved renders the ticket resolved customer notification.
func (r *Renderer) RenderTicketResolved(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusResolved
	return r.RenderTicketCustomer(data)
}

// RenderTicketClosed renders the ticket closed customer notification.
func (r *Renderer) RenderTicketClosed(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusClosed
	return r.RenderTicketCustomer(data)
}

// RenderTicketResolvedAdmin renders the ticket resolved admin notification.
func (r *Renderer) RenderTicketResolvedAdmin(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusResolved
	return r.RenderTicketAdmin(data)
}

// RenderTicketClosedAdmin renders the ticket closed admin notification.
func (r *Renderer) RenderTicketClosedAdmin(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusClosed
	return r.RenderTicketAdmin(data)
}

// RenderTicketInProgress renders the ticket in progress customer notification.
func (r *Renderer) RenderTicketInProgress(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusInProgress
	return r.RenderTicketCustomer(data)
}

// RenderTicketOnHold renders the ticket on hold customer notification.
func (r *Renderer) RenderTicketOnHold(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusOnHold
	return r.RenderTicketCustomer(data)
}

// RenderTicketEscalated renders the ticket escalated customer notification.
func (r *Renderer) RenderTicketEscalated(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusEscalated
	return r.RenderTicketCustomer(data)
}

// RenderTicketEscalatedAdmin renders the ticket escalated admin notification.
func (r *Renderer) RenderTicketEscalatedAdmin(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusEscalated
	return r.RenderTicketAdmin(data)
}

// RenderTicketReopened renders the ticket reopened customer notification.
func (r *Renderer) RenderTicketReopened(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusReopened
	return r.RenderTicketCustomer(data)
}

// RenderTicketReopenedAdmin renders the ticket reopened admin notification.
func (r *Renderer) RenderTicketReopenedAdmin(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusReopened
	return r.RenderTicketAdmin(data)
}

// RenderTicketCancelled renders the ticket cancelled customer notification.
func (r *Renderer) RenderTicketCancelled(data *EmailData) (string, string, error) {
	data.TicketStatus = TicketStatusCancelled
	return r.RenderTicketCustomer(data)
}

// RenderTicketStatusUpdate renders a generic ticket status update notification.
func (r *Renderer) RenderTicketStatusUpdate(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateTicketData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.Subject = fmt.Sprintf("Your Support Ticket Status Updated - #%s", data.TicketNumber)
	if data.OldStatus != "" && data.NewStatus != "" {
		data.Preheader = fmt.Sprintf("Your ticket #%s status has changed from %s to %s.", data.TicketNumber, data.OldStatus, data.NewStatus)
	} else if data.NewStatus != "" {
		data.Preheader = fmt.Sprintf("Your ticket #%s status is now: %s.", data.TicketNumber, data.NewStatus)
	} else {
		data.Preheader = fmt.Sprintf("Your ticket #%s status has been updated.", data.TicketNumber)
	}

	// Use the new status if provided
	if data.NewStatus != "" {
		data.TicketStatus = data.NewStatus
	}

	body, err := r.Render("ticket_customer", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render ticket_customer: %w", err)
	}

	return data.Subject, body, nil
}
