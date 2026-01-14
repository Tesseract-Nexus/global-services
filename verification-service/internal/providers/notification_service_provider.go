package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NotificationServiceProvider implements email sending via the notification-service API
// This centralizes all email delivery through notification-service which handles Postal/SendGrid
type NotificationServiceProvider struct {
	baseURL   string
	apiKey    string
	fromEmail string
	fromName  string
	client    *http.Client
}

// notificationSendRequest represents the notification-service send request
type notificationSendRequest struct {
	Channel        string                 `json:"channel"`
	RecipientEmail string                 `json:"recipientEmail,omitempty"`
	RecipientPhone string                 `json:"recipientPhone,omitempty"`
	Subject        string                 `json:"subject"`
	Body           string                 `json:"body,omitempty"`
	BodyHTML       string                 `json:"bodyHtml,omitempty"`
	TemplateName   string                 `json:"templateName,omitempty"`
	Variables      map[string]interface{} `json:"variables,omitempty"`
	Priority       string                 `json:"priority,omitempty"`
}

// notificationResponse represents the notification-service API response
type notificationResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"data"`
}

// NewNotificationServiceProvider creates a new notification service provider
// baseURL should be the notification-service internal URL (e.g., http://notification-service.devtest.svc.cluster.local:8090)
// apiKey is used for inter-service authentication
func NewNotificationServiceProvider(baseURL, apiKey, fromEmail, fromName string) *NotificationServiceProvider {
	return &NotificationServiceProvider{
		baseURL:   baseURL,
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetName returns the provider name
func (p *NotificationServiceProvider) GetName() string {
	return "notification-service"
}

// SendVerificationEmail sends a verification email via notification-service
func (p *NotificationServiceProvider) SendVerificationEmail(recipient, code, purpose string) error {
	subject, htmlBody := FormatVerificationEmail(code, purpose)
	return p.SendEmail(recipient, subject, htmlBody)
}

// SendEmail sends a generic email via notification-service API
func (p *NotificationServiceProvider) SendEmail(recipient, subject, htmlBody string) error {
	// Create request payload
	payload := notificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: recipient,
		Subject:        subject,
		BodyHTML:       htmlBody,
		Priority:       "HIGH", // Verification emails are high priority
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build API URL
	apiEndpoint := fmt.Sprintf("%s/api/v1/notifications/send", p.baseURL)

	// Create HTTP request
	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Use system tenant and user for inter-service communication
	req.Header.Set("X-Tenant-ID", "system")
	req.Header.Set("X-User-ID", "00000000-0000-0000-0000-000000000001") // System user
	if p.apiKey != "" {
		req.Header.Set("X-API-Key", p.apiKey)
	}

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to notification-service: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("notification-service API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var notifResp notificationResponse
	if err := json.Unmarshal(body, &notifResp); err != nil {
		return fmt.Errorf("failed to parse notification-service response: %w", err)
	}

	if !notifResp.Success {
		return fmt.Errorf("notification-service returned error: %s", notifResp.Error)
	}

	return nil
}

// SendTemplatedEmail sends an email using a named template in notification-service
func (p *NotificationServiceProvider) SendTemplatedEmail(recipient, templateName string, variables map[string]interface{}) error {
	// Create request payload
	payload := notificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: recipient,
		TemplateName:   templateName,
		Variables:      variables,
		Priority:       "HIGH",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build API URL
	apiEndpoint := fmt.Sprintf("%s/api/v1/notifications/send", p.baseURL)

	// Create HTTP request
	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "system")
	req.Header.Set("X-User-ID", "00000000-0000-0000-0000-000000000001")
	if p.apiKey != "" {
		req.Header.Set("X-API-Key", p.apiKey)
	}

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to notification-service: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("notification-service API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var notifResp notificationResponse
	if err := json.Unmarshal(body, &notifResp); err != nil {
		return fmt.Errorf("failed to parse notification-service response: %w", err)
	}

	if !notifResp.Success {
		return fmt.Errorf("notification-service returned error: %s", notifResp.Error)
	}

	return nil
}
