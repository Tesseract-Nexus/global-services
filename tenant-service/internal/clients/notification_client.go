package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// NotificationClient handles communication with notification-service for sending emails.
// All email rendering is delegated to notification-service via DB templates.
type NotificationClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewNotificationClient creates a new notification service client.
func NewNotificationClient(baseURL string, apiKey string) *NotificationClient {
	notificationURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notificationURL == "" {
		notificationURL = "http://notification-service.marketplace.svc.cluster.local:8090"
	}
	return &NotificationClient{
		baseURL: notificationURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// notificationSendRequest represents a request to notification-service /api/v1/notifications/send.
// Uses templateName + variables so notification-service renders HTML from DB templates.
type notificationSendRequest struct {
	Channel        string                 `json:"channel"`
	TemplateName   string                 `json:"templateName,omitempty"`
	RecipientEmail string                 `json:"recipientEmail"`
	Variables      map[string]interface{} `json:"variables,omitempty"`
	Priority       string                 `json:"priority,omitempty"`
	// Legacy fields kept for backward compat — only used if TemplateName is empty
	Subject  string `json:"subject,omitempty"`
	Body     string `json:"body,omitempty"`
	BodyHTML string `json:"bodyHtml,omitempty"`
}

type sendResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// CustomDomainDNSConfig holds DNS configuration for custom domains.
// Used by the GetDNSConfig API endpoint (UI-facing).
type CustomDomainDNSConfig struct {
	IsCustomDomain     bool   `json:"isCustomDomain"`
	CustomDomain       string `json:"customDomain"`
	AdminHost          string `json:"adminHost"`
	StorefrontHost     string `json:"storefrontHost"`
	APIHost            string `json:"apiHost"`
	UseARecords        bool   `json:"useARecords"`
	RoutingIP          string `json:"routingIP"`
	RoutingCNAMETarget string `json:"routingCNAMETarget"`
	TenantSlug         string `json:"tenantSlug"`
	BaseDomain         string `json:"baseDomain"`
	UseCNAMEDelegation bool   `json:"useCNAMEDelegation"`
	ACMEChallengeHost  string `json:"acmeChallengeHost"`
	ACMECNAMETarget    string `json:"acmeCNAMETarget"`
}

// GoodbyeEmailData contains data for the goodbye/deactivation email.
type GoodbyeEmailData struct {
	Email            string
	FirstName        string
	StoreName        string
	DeactivatedAt    time.Time
	ScheduledPurgeAt time.Time
	ReactivationURL  string
}

// PasswordResetEmailData contains data for password reset emails.
type PasswordResetEmailData struct {
	Email     string
	FirstName string
	StoreName string
	ResetLink string
	ExpiresIn string // e.g., "1 hour"
}

// WelcomePackData contains all data for sending a welcome pack email.
type WelcomePackData struct {
	Email         string
	FirstName     string
	BusinessName  string
	TenantSlug    string
	AdminURL      string
	StorefrontURL string
	DashboardURL  string
}

// ---------- Public Send methods ----------

// SendVerificationLinkEmail sends a verification link email via notification-service template.
func (c *NotificationClient) SendVerificationLinkEmail(ctx context.Context, email, verificationLink, businessName string) error {
	return c.SendVerificationLinkEmailWithDNS(ctx, email, verificationLink, businessName, nil)
}

// SendVerificationLinkEmailWithDNS sends a verification link email.
// DNS config is used only to determine whether this is a custom domain onboarding;
// actual DNS records are shown in the onboarding dashboard, not in the email.
func (c *NotificationClient) SendVerificationLinkEmailWithDNS(ctx context.Context, email, verificationLink, businessName string, dnsConfig *CustomDomainDNSConfig) error {
	vars := map[string]interface{}{
		"verification_link":   verificationLink,
		"verification_expiry": "1 hour",
		"store_name":          businessName,
		"business_name":       businessName,
		"email":               email,
	}
	if dnsConfig != nil && dnsConfig.IsCustomDomain {
		vars["is_custom_domain"] = true
		vars["custom_domain"] = dnsConfig.CustomDomain
	}
	return c.sendTemplated(ctx, email, "verification-link", vars, "high")
}

// SendVerificationSuccessEmail is a no-op — handled by the welcome pack.
func (c *NotificationClient) SendVerificationSuccessEmail(ctx context.Context, email, businessName string) error {
	return nil
}

// SendWelcomeEmail sends a generic welcome email.
func (c *NotificationClient) SendWelcomeEmail(ctx context.Context, email, firstName string) error {
	return c.sendTemplated(ctx, email, "welcome-email", map[string]interface{}{
		"customer_name": firstName,
		"first_name":    firstName,
	}, "")
}

// SendAccountCreatedEmail is a no-op — handled by the welcome pack.
func (c *NotificationClient) SendAccountCreatedEmail(ctx context.Context, email, firstName, businessName, subdomain string) error {
	return nil
}

// SendCustomerWelcomeEmail sends a welcome email to a storefront customer.
func (c *NotificationClient) SendCustomerWelcomeEmail(ctx context.Context, email, firstName, storeName, storefrontURL string) error {
	return c.sendTemplated(ctx, email, "welcome-email", map[string]interface{}{
		"customer_name": firstName,
		"first_name":    firstName,
		"store_name":    storeName,
		"store_url":     storefrontURL,
	}, "")
}

// SendWelcomePackEmail sends a comprehensive welcome pack email with tenant URLs.
func (c *NotificationClient) SendWelcomePackEmail(ctx context.Context, data *WelcomePackData) error {
	return c.sendTemplated(ctx, data.Email, "tenant-welcome-pack", map[string]interface{}{
		"business_name":  data.BusinessName,
		"first_name":     data.FirstName,
		"admin_url":      data.AdminURL,
		"storefront_url": data.StorefrontURL,
		"email":          data.Email,
		"tenant_slug":    data.TenantSlug,
	}, "high")
}

// SendGoodbyeEmail sends a goodbye email when a customer deactivates their account.
func (c *NotificationClient) SendGoodbyeEmail(ctx context.Context, data *GoodbyeEmailData) error {
	return c.sendTemplated(ctx, data.Email, "customer-goodbye", map[string]interface{}{
		"first_name":       data.FirstName,
		"store_name":       data.StoreName,
		"purge_date":       data.ScheduledPurgeAt.Format("January 2, 2006"),
		"reactivation_url": data.ReactivationURL,
	}, "")
}

// SendPasswordResetEmail sends a password reset email with a secure link.
func (c *NotificationClient) SendPasswordResetEmail(ctx context.Context, data *PasswordResetEmailData) error {
	return c.sendTemplated(ctx, data.Email, "password-reset", map[string]interface{}{
		"first_name": data.FirstName,
		"store_name": data.StoreName,
		"reset_url":  data.ResetLink,
		"expires_in": data.ExpiresIn,
	}, "high")
}

// SendDraftReminderEmail sends a reminder for incomplete onboarding drafts.
func (c *NotificationClient) SendDraftReminderEmail(ctx context.Context, email, firstName, sessionID string) error {
	onboardingBaseURL := os.Getenv("ONBOARDING_APP_URL")
	if onboardingBaseURL == "" {
		onboardingBaseURL = "http://localhost:3002"
	}
	continueURL := fmt.Sprintf("%s/onboarding?session=%s", onboardingBaseURL, sessionID)

	return c.sendTemplated(ctx, email, "draft-reminder", map[string]interface{}{
		"first_name":   firstName,
		"continue_url": continueURL,
		"session_id":   sessionID,
	}, "")
}

// ---------- Internal helpers ----------

// sendTemplated sends an email using a notification-service DB template.
func (c *NotificationClient) sendTemplated(ctx context.Context, recipientEmail, templateName string, variables map[string]interface{}, priority string) error {
	req := &notificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: recipientEmail,
		TemplateName:   templateName,
		Variables:      variables,
		Priority:       priority,
	}

	var resp sendResponse
	if err := c.makeRequest(ctx, "POST", "/api/v1/notifications/send", req, &resp); err != nil {
		return fmt.Errorf("failed to call notification-service: %w", err)
	}
	if !resp.Success && resp.Error != "" {
		return fmt.Errorf("notification-service error: %s", resp.Error)
	}
	return nil
}

// makeRequest makes an HTTP request to notification-service.
func (c *NotificationClient) makeRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "onboarding")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification-service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
