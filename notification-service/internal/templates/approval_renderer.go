// Package templates provides email template rendering for the notification service.
// This file contains all approval workflow email render methods.
package templates

import (
	"errors"
	"fmt"
	"strings"
)

// Approval status constants for templates
const (
	ApprovalStatusPending   = "PENDING"
	ApprovalStatusApproved  = "APPROVED"
	ApprovalStatusRejected  = "REJECTED"
	ApprovalStatusCancelled = "CANCELLED"
	ApprovalStatusExpired   = "EXPIRED"
	ApprovalStatusEscalated = "ESCALATED"
)

// Approval priority constants
const (
	ApprovalPriorityLow    = "low"
	ApprovalPriorityNormal = "normal"
	ApprovalPriorityHigh   = "high"
	ApprovalPriorityUrgent = "urgent"
)

// ErrMissingApprovalID is returned when approval ID is empty.
var ErrMissingApprovalID = errors.New("approval ID is required")

// ErrMissingApproverInfo is returned when approver info is empty.
var ErrMissingApproverInfo = errors.New("approver name or email is required")

// ErrMissingRequesterInfo is returned when requester info is empty.
var ErrMissingRequesterInfo = errors.New("requester name or email is required")

// validateApprovalApproverData validates required fields for approver notifications.
func validateApprovalApproverData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	if data.ApprovalID == "" {
		return ErrMissingApprovalID
	}
	if data.ApproverName == "" && data.ApproverEmail == "" {
		return ErrMissingApproverInfo
	}
	if data.RequesterName == "" && data.RequesterEmail == "" {
		return ErrMissingRequesterInfo
	}
	// Use email as fallback for name
	if data.ApproverName == "" {
		data.ApproverName = data.ApproverEmail
	}
	if data.RequesterName == "" {
		data.RequesterName = data.RequesterEmail
	}
	return nil
}

// validateApprovalRequesterData validates required fields for requester notifications.
func validateApprovalRequesterData(data *EmailData) error {
	if data == nil {
		return errors.New("email data is nil")
	}
	if data.ApprovalID == "" {
		return ErrMissingApprovalID
	}
	if data.RequesterName == "" && data.RequesterEmail == "" {
		return ErrMissingRequesterInfo
	}
	// Use email as fallback for name
	if data.RequesterName == "" {
		data.RequesterName = data.RequesterEmail
	}
	return nil
}

// formatActionTypeDisplay converts action_type to human-readable display format.
func formatActionTypeDisplay(actionType string) string {
	if actionType == "" {
		return "Action"
	}

	// Replace underscores with spaces and title case
	display := strings.ReplaceAll(actionType, "_", " ")

	// Title case each word
	words := strings.Split(display, " ")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}

	return strings.Join(words, " ")
}

// RenderApprovalApprover renders the approval request notification for approvers.
// This is sent when a new approval request is created or escalated.
// Returns subject, body, and error.
func (r *Renderer) RenderApprovalApprover(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateApprovalApproverData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	// Set action type display if not already set
	if data.ActionTypeDisplay == "" {
		data.ActionTypeDisplay = formatActionTypeDisplay(data.ActionType)
	}

	// Set subject and preheader based on priority/status
	switch {
	case data.ApprovalStatus == ApprovalStatusEscalated:
		data.Subject = fmt.Sprintf("[Escalated] Approval Required: %s", data.ActionTypeDisplay)
		data.Preheader = fmt.Sprintf("An approval request has been escalated to you. Request from %s.", data.RequesterName)
	case data.ApprovalPriority == ApprovalPriorityUrgent:
		data.Subject = fmt.Sprintf("[URGENT] Approval Required: %s", data.ActionTypeDisplay)
		data.Preheader = fmt.Sprintf("Urgent approval needed from %s. Please review immediately.", data.RequesterName)
	case data.ApprovalPriority == ApprovalPriorityHigh:
		data.Subject = fmt.Sprintf("[High Priority] Approval Required: %s", data.ActionTypeDisplay)
		data.Preheader = fmt.Sprintf("High priority approval request from %s awaiting your review.", data.RequesterName)
	default:
		data.Subject = fmt.Sprintf("Approval Required: %s", data.ActionTypeDisplay)
		data.Preheader = fmt.Sprintf("A new approval request from %s needs your review.", data.RequesterName)
	}

	body, err := r.Render("approval_approver", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render approval_approver: %w", err)
	}

	return data.Subject, body, nil
}

// RenderApprovalRequester renders the approval decision notification for requesters.
// This is sent when an approval request is approved, rejected, cancelled, or expired.
// Returns subject, body, and error.
func (r *Renderer) RenderApprovalRequester(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateApprovalRequesterData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	// Set action type display if not already set
	if data.ActionTypeDisplay == "" {
		data.ActionTypeDisplay = formatActionTypeDisplay(data.ActionType)
	}

	// Set subject and preheader based on approval status
	switch data.ApprovalStatus {
	case ApprovalStatusApproved:
		data.Subject = fmt.Sprintf("Approved: Your %s Request", data.ActionTypeDisplay)
		data.Preheader = fmt.Sprintf("Your %s request has been approved!", data.ActionTypeDisplay)
	case ApprovalStatusRejected:
		data.Subject = fmt.Sprintf("Rejected: Your %s Request", data.ActionTypeDisplay)
		data.Preheader = fmt.Sprintf("Your %s request was not approved.", data.ActionTypeDisplay)
	case ApprovalStatusCancelled:
		data.Subject = fmt.Sprintf("Cancelled: Your %s Request", data.ActionTypeDisplay)
		data.Preheader = fmt.Sprintf("Your %s request has been cancelled.", data.ActionTypeDisplay)
	case ApprovalStatusExpired:
		data.Subject = fmt.Sprintf("Expired: Your %s Request", data.ActionTypeDisplay)
		data.Preheader = fmt.Sprintf("Your %s request has expired without a decision.", data.ActionTypeDisplay)
	default:
		data.Subject = fmt.Sprintf("Update: Your %s Request", data.ActionTypeDisplay)
		data.Preheader = fmt.Sprintf("There's an update on your %s request.", data.ActionTypeDisplay)
	}

	body, err := r.Render("approval_requester", data)
	if err != nil {
		return "", "", fmt.Errorf("failed to render approval_requester: %w", err)
	}

	return data.Subject, body, nil
}

// RenderApprovalEscalated renders the escalation notification for new approvers.
// This is a convenience wrapper around RenderApprovalApprover with status ESCALATED.
// Returns subject, body, and error.
func (r *Renderer) RenderApprovalEscalated(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateApprovalApproverData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.ApprovalStatus = ApprovalStatusEscalated
	return r.RenderApprovalApprover(data)
}

// RenderApprovalApproved renders the approval notification for requesters.
// This is a convenience wrapper around RenderApprovalRequester with status APPROVED.
// Returns subject, body, and error.
func (r *Renderer) RenderApprovalApproved(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateApprovalRequesterData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.ApprovalStatus = ApprovalStatusApproved
	return r.RenderApprovalRequester(data)
}

// RenderApprovalRejected renders the rejection notification for requesters.
// This is a convenience wrapper around RenderApprovalRequester with status REJECTED.
// Returns subject, body, and error.
func (r *Renderer) RenderApprovalRejected(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateApprovalRequesterData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.ApprovalStatus = ApprovalStatusRejected
	return r.RenderApprovalRequester(data)
}

// RenderApprovalCancelled renders the cancellation notification for requesters.
// This is a convenience wrapper around RenderApprovalRequester with status CANCELLED.
// Returns subject, body, and error.
func (r *Renderer) RenderApprovalCancelled(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateApprovalRequesterData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.ApprovalStatus = ApprovalStatusCancelled
	return r.RenderApprovalRequester(data)
}

// RenderApprovalExpired renders the expiration notification for requesters.
// This is a convenience wrapper around RenderApprovalRequester with status EXPIRED.
// Returns subject, body, and error.
func (r *Renderer) RenderApprovalExpired(data *EmailData) (string, string, error) {
	if r == nil {
		return "", "", ErrNilRenderer
	}
	if err := validateApprovalRequesterData(data); err != nil {
		return "", "", fmt.Errorf("validation failed: %w", err)
	}

	data.ApprovalStatus = ApprovalStatusExpired
	return r.RenderApprovalRequester(data)
}
