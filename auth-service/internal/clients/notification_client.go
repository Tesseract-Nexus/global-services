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

// NotificationClient handles HTTP communication with notification-service
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// AuthNotification contains data for sending auth-related email notifications
type AuthNotification struct {
	TenantID      string
	UserID        string
	UserEmail     string
	UserName      string
	EventType     string // REGISTERED, EMAIL_VERIFIED, PASSWORD_RESET
	ResetCode     string
	ResetURL      string
	VerifyURL     string
	AdminURL      string
	StorefrontURL string
}

// notificationRequest is the API request format for notification-service
type notificationRequest struct {
	To        string            `json:"to"`
	Subject   string            `json:"subject"`
	Template  string            `json:"template"`
	Variables map[string]string `json:"variables"`
}

// NewNotificationClient creates a new notification client
func NewNotificationClient() *NotificationClient {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://notification-service.devtest.svc.cluster.local:8090"
	}

	return &NotificationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendUserRegisteredNotification sends a welcome email when a user registers
func (c *NotificationClient) SendUserRegisteredNotification(ctx context.Context, notification *AuthNotification) error {
	if notification.UserEmail == "" {
		log.Printf("[AUTH] No user email for user %s, skipping notification", notification.UserID)
		return nil
	}

	req := &notificationRequest{
		To:       notification.UserEmail,
		Subject:  "Welcome to Tesseract Hub!",
		Template: "customer_welcome",
		Variables: map[string]string{
			"customerName":  notification.UserName,
			"customerEmail": notification.UserEmail,
			"userId":        notification.UserID,
			"storefrontUrl": notification.StorefrontURL,
			"tenantId":      notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, req)
}

// SendPasswordResetNotification sends a password reset email
func (c *NotificationClient) SendPasswordResetNotification(ctx context.Context, notification *AuthNotification) error {
	if notification.UserEmail == "" {
		log.Printf("[AUTH] No user email for user %s, skipping notification", notification.UserID)
		return nil
	}

	req := &notificationRequest{
		To:       notification.UserEmail,
		Subject:  "Password Reset Request",
		Template: "password_reset",
		Variables: map[string]string{
			"customerName":  notification.UserName,
			"customerEmail": notification.UserEmail,
			"resetCode":     notification.ResetCode,
			"resetUrl":      notification.ResetURL,
			"userId":        notification.UserID,
			"tenantId":      notification.TenantID,
		},
	}

	return c.sendNotification(ctx, notification.TenantID, req)
}

// SendEmailVerifiedNotification sends a confirmation when email is verified
func (c *NotificationClient) SendEmailVerifiedNotification(ctx context.Context, notification *AuthNotification) error {
	if notification.UserEmail == "" {
		log.Printf("[AUTH] No user email for user %s, skipping notification", notification.UserID)
		return nil
	}

	// Email verification confirmation is optional - most flows don't need this
	// since the user is immediately redirected to login after verification
	log.Printf("[AUTH] Email verified for user %s, no email notification needed", notification.UserID)
	return nil
}

// sendNotification sends a notification request to notification-service
func (c *NotificationClient) sendNotification(ctx context.Context, tenantID string, req *notificationRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal notification request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/notifications/send", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)
	httpReq.Header.Set("X-Internal-Service", "auth-service")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	log.Printf("[AUTH] Notification sent successfully to %s (template: %s)", req.To, req.Template)
	return nil
}
