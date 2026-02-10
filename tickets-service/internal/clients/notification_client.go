package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// NotificationClient sends notifications via notification-service API
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewNotificationClient creates a new notification client
func NewNotificationClient() *NotificationClient {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://notification-service.marketplace.svc.cluster.local:8090"
	}

	return &NotificationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendNotificationRequest represents the API request to notification-service
type SendNotificationRequest struct {
	Channel        string                 `json:"channel"`
	RecipientEmail string                 `json:"recipientEmail"`
	Subject        string                 `json:"subject"`
	TemplateName   string                 `json:"templateName,omitempty"`
	Variables      map[string]interface{} `json:"variables,omitempty"`
	BodyHTML       string                 `json:"bodyHtml,omitempty"`
}

// TicketNotification contains ticket details for notification
type TicketNotification struct {
	TenantID      string
	TicketID      string
	TicketNumber  string
	Subject       string
	Description   string
	Status        string
	Priority      string
	CustomerEmail string
	CustomerName  string
	TicketURL     string
}

// SendTicketCreatedNotification sends email notifications when a ticket is created
func (c *NotificationClient) SendTicketCreatedNotification(ctx context.Context, ticket *TicketNotification) error {
	// Send to customer
	if ticket.CustomerEmail != "" {
		if err := c.sendCustomerTicketEmail(ctx, ticket); err != nil {
			log.Printf("[NotificationClient] Failed to send customer email: %v", err)
			// Don't return error - continue to send admin email
		}
	}

	// Send to admin
	if err := c.sendAdminTicketEmail(ctx, ticket); err != nil {
		log.Printf("[NotificationClient] Failed to send admin email: %v", err)
	}

	return nil
}

func (c *NotificationClient) sendCustomerTicketEmail(ctx context.Context, ticket *TicketNotification) error {
	req := SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: ticket.CustomerEmail,
		Subject:        fmt.Sprintf("Support Ticket Created - #%s", ticket.TicketNumber),
		TemplateName:   "ticket_customer",
		Variables: map[string]interface{}{
			"ticketId":       ticket.TicketID,
			"ticketNumber":   ticket.TicketNumber,
			"ticketSubject":  ticket.Subject,
			"description":    ticket.Description,
			"ticketStatus":   "CREATED",
			"ticketPriority": ticket.Priority,
			"email":          ticket.CustomerEmail,
			"customerName":   ticket.CustomerName,
			"ticketUrl":      ticket.TicketURL,
		},
	}

	return c.send(ctx, ticket.TenantID, req)
}

func (c *NotificationClient) sendAdminTicketEmail(ctx context.Context, ticket *TicketNotification) error {
	// Get admin email from env or use default
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "support@tesserix.app"
	}

	req := SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: adminEmail,
		Subject:        fmt.Sprintf("[%s] New Support Ticket - #%s", ticket.Priority, ticket.TicketNumber),
		TemplateName:   "ticket_admin",
		Variables: map[string]interface{}{
			"ticketId":       ticket.TicketID,
			"ticketNumber":   ticket.TicketNumber,
			"ticketSubject":  ticket.Subject,
			"description":    ticket.Description,
			"ticketStatus":   "CREATED",
			"ticketPriority": ticket.Priority,
			"email":          ticket.CustomerEmail,
			"customerName":   ticket.CustomerName,
			"ticketUrl":      ticket.TicketURL,
		},
	}

	return c.send(ctx, ticket.TenantID, req)
}

// SendTicketResolvedNotification sends email when ticket is resolved
func (c *NotificationClient) SendTicketResolvedNotification(ctx context.Context, ticket *TicketNotification, resolution, resolvedBy string) error {
	// Send to customer
	if ticket.CustomerEmail != "" {
		req := SendNotificationRequest{
			Channel:        "EMAIL",
			RecipientEmail: ticket.CustomerEmail,
			Subject:        fmt.Sprintf("Your Support Ticket Has Been Resolved - #%s", ticket.TicketNumber),
			TemplateName:   "ticket_customer",
			Variables: map[string]interface{}{
				"ticketId":      ticket.TicketID,
				"ticketNumber":  ticket.TicketNumber,
				"ticketSubject": ticket.Subject,
				"ticketStatus":  "RESOLVED",
				"resolution":    resolution,
				"customerName":  ticket.CustomerName,
				"ticketUrl":     ticket.TicketURL,
			},
		}
		if err := c.send(ctx, ticket.TenantID, req); err != nil {
			log.Printf("[NotificationClient] Failed to send customer resolved email: %v", err)
		}
	}

	// Send to admin
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "support@tesserix.app"
	}

	adminReq := SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: adminEmail,
		Subject:        fmt.Sprintf("Ticket Resolved - #%s", ticket.TicketNumber),
		TemplateName:   "ticket_admin",
		Variables: map[string]interface{}{
			"ticketId":      ticket.TicketID,
			"ticketNumber":  ticket.TicketNumber,
			"ticketSubject": ticket.Subject,
			"ticketStatus":  "RESOLVED",
			"resolution":    resolution,
			"email":         ticket.CustomerEmail,
			"customerName":  ticket.CustomerName,
			"resolvedBy":    resolvedBy,
			"ticketUrl":     ticket.TicketURL,
		},
	}
	if err := c.send(ctx, ticket.TenantID, adminReq); err != nil {
		log.Printf("[NotificationClient] Failed to send admin resolved email: %v", err)
	}

	return nil
}

// SendTicketClosedNotification sends email when ticket is closed
func (c *NotificationClient) SendTicketClosedNotification(ctx context.Context, ticket *TicketNotification, closedBy string) error {
	// Send to customer
	if ticket.CustomerEmail != "" {
		req := SendNotificationRequest{
			Channel:        "EMAIL",
			RecipientEmail: ticket.CustomerEmail,
			Subject:        fmt.Sprintf("Your Support Ticket Has Been Closed - #%s", ticket.TicketNumber),
			TemplateName:   "ticket_customer",
			Variables: map[string]interface{}{
				"ticketId":      ticket.TicketID,
				"ticketNumber":  ticket.TicketNumber,
				"ticketSubject": ticket.Subject,
				"ticketStatus":  "CLOSED",
				"customerName":  ticket.CustomerName,
				"ticketUrl":     ticket.TicketURL,
			},
		}
		if err := c.send(ctx, ticket.TenantID, req); err != nil {
			log.Printf("[NotificationClient] Failed to send customer closed email: %v", err)
		}
	}

	// Send to admin
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "support@tesserix.app"
	}

	adminReq := SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: adminEmail,
		Subject:        fmt.Sprintf("Ticket Closed - #%s", ticket.TicketNumber),
		TemplateName:   "ticket_admin",
		Variables: map[string]interface{}{
			"ticketId":      ticket.TicketID,
			"ticketNumber":  ticket.TicketNumber,
			"ticketSubject": ticket.Subject,
			"ticketStatus":  "CLOSED",
			"email":         ticket.CustomerEmail,
			"customerName":  ticket.CustomerName,
			"closedBy":      closedBy,
			"ticketUrl":     ticket.TicketURL,
		},
	}
	if err := c.send(ctx, ticket.TenantID, adminReq); err != nil {
		log.Printf("[NotificationClient] Failed to send admin closed email: %v", err)
	}

	return nil
}

// SendTicketInProgressNotification sends email when ticket work begins
func (c *NotificationClient) SendTicketInProgressNotification(ctx context.Context, ticket *TicketNotification, assignedTo string) error {
	// Send to customer only - they should know work has started
	if ticket.CustomerEmail != "" {
		req := SendNotificationRequest{
			Channel:        "EMAIL",
			RecipientEmail: ticket.CustomerEmail,
			Subject:        fmt.Sprintf("Work Has Started on Your Ticket - #%s", ticket.TicketNumber),
			TemplateName:   "ticket_customer",
			Variables: map[string]interface{}{
				"ticketId":      ticket.TicketID,
				"ticketNumber":  ticket.TicketNumber,
				"ticketSubject": ticket.Subject,
				"ticketStatus":  "IN_PROGRESS",
				"customerName":  ticket.CustomerName,
				"assignedTo":    assignedTo,
				"ticketUrl":     ticket.TicketURL,
			},
		}
		if err := c.send(ctx, ticket.TenantID, req); err != nil {
			log.Printf("[NotificationClient] Failed to send in-progress email: %v", err)
		}
	}
	return nil
}

// SendTicketOnHoldNotification sends email when ticket is put on hold
func (c *NotificationClient) SendTicketOnHoldNotification(ctx context.Context, ticket *TicketNotification, reason, updatedBy string) error {
	// Send to customer
	if ticket.CustomerEmail != "" {
		req := SendNotificationRequest{
			Channel:        "EMAIL",
			RecipientEmail: ticket.CustomerEmail,
			Subject:        fmt.Sprintf("Your Support Ticket Is On Hold - #%s", ticket.TicketNumber),
			TemplateName:   "ticket_customer",
			Variables: map[string]interface{}{
				"ticketId":      ticket.TicketID,
				"ticketNumber":  ticket.TicketNumber,
				"ticketSubject": ticket.Subject,
				"ticketStatus":  "ON_HOLD",
				"customerName":  ticket.CustomerName,
				"reason":        reason,
				"ticketUrl":     ticket.TicketURL,
			},
		}
		if err := c.send(ctx, ticket.TenantID, req); err != nil {
			log.Printf("[NotificationClient] Failed to send on-hold email: %v", err)
		}
	}
	return nil
}

// SendTicketEscalatedNotification sends email when ticket is escalated
func (c *NotificationClient) SendTicketEscalatedNotification(ctx context.Context, ticket *TicketNotification, escalatedBy, escalationReason string) error {
	// Send to customer
	if ticket.CustomerEmail != "" {
		req := SendNotificationRequest{
			Channel:        "EMAIL",
			RecipientEmail: ticket.CustomerEmail,
			Subject:        fmt.Sprintf("Your Support Ticket Has Been Escalated - #%s", ticket.TicketNumber),
			TemplateName:   "ticket_customer",
			Variables: map[string]interface{}{
				"ticketId":      ticket.TicketID,
				"ticketNumber":  ticket.TicketNumber,
				"ticketSubject": ticket.Subject,
				"ticketStatus":  "ESCALATED",
				"customerName":  ticket.CustomerName,
				"ticketUrl":     ticket.TicketURL,
			},
		}
		if err := c.send(ctx, ticket.TenantID, req); err != nil {
			log.Printf("[NotificationClient] Failed to send customer escalation email: %v", err)
		}
	}

	// Send to admin - important for escalations
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "support@tesserix.app"
	}

	adminReq := SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: adminEmail,
		Subject:        fmt.Sprintf("[ESCALATED] Urgent Attention Required - Ticket #%s", ticket.TicketNumber),
		TemplateName:   "ticket_admin",
		Variables: map[string]interface{}{
			"ticketId":         ticket.TicketID,
			"ticketNumber":     ticket.TicketNumber,
			"ticketSubject":    ticket.Subject,
			"description":      ticket.Description,
			"ticketStatus":     "ESCALATED",
			"ticketPriority":   ticket.Priority,
			"email":            ticket.CustomerEmail,
			"customerName":     ticket.CustomerName,
			"escalatedBy":      escalatedBy,
			"escalationReason": escalationReason,
			"ticketUrl":        ticket.TicketURL,
		},
	}
	if err := c.send(ctx, ticket.TenantID, adminReq); err != nil {
		log.Printf("[NotificationClient] Failed to send admin escalation email: %v", err)
	}

	return nil
}

// SendTicketReopenedNotification sends email when ticket is reopened
func (c *NotificationClient) SendTicketReopenedNotification(ctx context.Context, ticket *TicketNotification, reopenedBy string) error {
	// Send to customer
	if ticket.CustomerEmail != "" {
		req := SendNotificationRequest{
			Channel:        "EMAIL",
			RecipientEmail: ticket.CustomerEmail,
			Subject:        fmt.Sprintf("Your Support Ticket Has Been Reopened - #%s", ticket.TicketNumber),
			TemplateName:   "ticket_customer",
			Variables: map[string]interface{}{
				"ticketId":      ticket.TicketID,
				"ticketNumber":  ticket.TicketNumber,
				"ticketSubject": ticket.Subject,
				"ticketStatus":  "REOPENED",
				"customerName":  ticket.CustomerName,
				"ticketUrl":     ticket.TicketURL,
			},
		}
		if err := c.send(ctx, ticket.TenantID, req); err != nil {
			log.Printf("[NotificationClient] Failed to send reopened email: %v", err)
		}
	}

	// Send to admin
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "support@tesserix.app"
	}

	adminReq := SendNotificationRequest{
		Channel:        "EMAIL",
		RecipientEmail: adminEmail,
		Subject:        fmt.Sprintf("Ticket Reopened - #%s", ticket.TicketNumber),
		TemplateName:   "ticket_admin",
		Variables: map[string]interface{}{
			"ticketId":      ticket.TicketID,
			"ticketNumber":  ticket.TicketNumber,
			"ticketSubject": ticket.Subject,
			"ticketStatus":  "REOPENED",
			"email":         ticket.CustomerEmail,
			"customerName":  ticket.CustomerName,
			"reopenedBy":    reopenedBy,
			"ticketUrl":     ticket.TicketURL,
		},
	}
	if err := c.send(ctx, ticket.TenantID, adminReq); err != nil {
		log.Printf("[NotificationClient] Failed to send admin reopened email: %v", err)
	}

	return nil
}

// SendTicketCancelledNotification sends email when ticket is cancelled
func (c *NotificationClient) SendTicketCancelledNotification(ctx context.Context, ticket *TicketNotification, cancelledBy, cancellationReason string) error {
	// Send to customer
	if ticket.CustomerEmail != "" {
		req := SendNotificationRequest{
			Channel:        "EMAIL",
			RecipientEmail: ticket.CustomerEmail,
			Subject:        fmt.Sprintf("Your Support Ticket Has Been Cancelled - #%s", ticket.TicketNumber),
			TemplateName:   "ticket_customer",
			Variables: map[string]interface{}{
				"ticketId":      ticket.TicketID,
				"ticketNumber":  ticket.TicketNumber,
				"ticketSubject": ticket.Subject,
				"ticketStatus":  "CANCELLED",
				"customerName":  ticket.CustomerName,
				"reason":        cancellationReason,
				"ticketUrl":     ticket.TicketURL,
			},
		}
		if err := c.send(ctx, ticket.TenantID, req); err != nil {
			log.Printf("[NotificationClient] Failed to send cancelled email: %v", err)
		}
	}
	return nil
}

// SendTicketStatusUpdateNotification sends generic status update email
// Used for statuses that don't have specific templates (e.g., PENDING_APPROVAL)
func (c *NotificationClient) SendTicketStatusUpdateNotification(ctx context.Context, ticket *TicketNotification, oldStatus, newStatus, updatedBy string) error {
	// Send to customer
	if ticket.CustomerEmail != "" {
		req := SendNotificationRequest{
			Channel:        "EMAIL",
			RecipientEmail: ticket.CustomerEmail,
			Subject:        fmt.Sprintf("Your Support Ticket Status Updated - #%s", ticket.TicketNumber),
			TemplateName:   "ticket_customer",
			Variables: map[string]interface{}{
				"ticketId":      ticket.TicketID,
				"ticketNumber":  ticket.TicketNumber,
				"ticketSubject": ticket.Subject,
				"ticketStatus":  newStatus,
				"customerName":  ticket.CustomerName,
				"oldStatus":     oldStatus,
				"ticketUrl":     ticket.TicketURL,
			},
		}
		if err := c.send(ctx, ticket.TenantID, req); err != nil {
			log.Printf("[NotificationClient] Failed to send status update email: %v", err)
		}
	}
	return nil
}

func (c *NotificationClient) send(ctx context.Context, tenantID string, req SendNotificationRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/notifications/send", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-Internal-Service", "tickets-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification-service returned status %d", resp.StatusCode)
	}

	log.Printf("[NotificationClient] Email sent successfully to %s", req.RecipientEmail)
	return nil
}

// BuildTicketURL builds the admin URL for viewing a ticket
func BuildTicketURL(tenantSlug, ticketID string) string {
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}
	return fmt.Sprintf("https://%s-admin.%s/support/%s", tenantSlug, baseDomain, ticketID)
}
