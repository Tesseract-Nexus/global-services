package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/tesseract-hub/go-shared/secrets"
	"github.com/tesseract-hub/go-shared/security"
)

// NotificationService handles notification business logic
type NotificationService struct {
	verificationServiceURL string
	verificationAPIKey     string
	httpClient             *http.Client
}

// NewNotificationService creates a new notification service
func NewNotificationService() *NotificationService {
	verificationServiceURL := os.Getenv("VERIFICATION_SERVICE_URL")
	if verificationServiceURL == "" {
		verificationServiceURL = "http://localhost:8088" // default
	}

	// Get API key from GCP Secret Manager or environment variable
	// Uses VERIFICATION_API_KEY_SECRET_NAME to look up in GCP Secret Manager,
	// falls back to VERIFICATION_API_KEY env var, then default for dev
	verificationAPIKey := secrets.GetSecretOrEnv(
		"VERIFICATION_API_KEY_SECRET_NAME",
		"VERIFICATION_API_KEY",
		"tesseract_verification_dev_key_2025", // default for local dev only
	)

	if verificationAPIKey != "" && verificationAPIKey != "tesseract_verification_dev_key_2025" {
		log.Printf("[NotificationService] Verification API key loaded from secret manager")
	} else {
		log.Printf("[NotificationService] Warning: Using default verification API key - emails may fail in production")
	}

	return &NotificationService{
		verificationServiceURL: verificationServiceURL,
		verificationAPIKey:     verificationAPIKey,
		httpClient:             &http.Client{},
	}
}

// SendVerificationEmail sends a verification email
func (s *NotificationService) SendVerificationEmail(ctx context.Context, email, code string) error {
	// TODO: Integrate with actual email service (SendGrid, AWS SES, etc.)
	// For now, we'll simulate sending email
	// SECURITY: Never log verification codes - they are authentication credentials
	fmt.Printf("Sending verification email to %s with code: %s\n", security.MaskEmail(email), security.MaskVerificationCode(code))

	// Email template would be something like:
	// Subject: "Verify your email address"
	// Body: "Your verification code is: {code}. This code will expire in 15 minutes."

	return nil
}

// SendVerificationSMS sends a verification SMS
func (s *NotificationService) SendVerificationSMS(ctx context.Context, phone, code string) error {
	// TODO: Integrate with actual SMS service (Twilio, AWS SNS, etc.)
	// For now, we'll simulate sending SMS
	// SECURITY: Never log verification codes - they are authentication credentials
	fmt.Printf("Sending verification SMS to %s with code: %s\n", security.MaskPhone(phone), security.MaskVerificationCode(code))

	// SMS template would be something like:
	// "Your verification code is: {code}. This code will expire in 10 minutes."

	return nil
}

// SendWelcomeEmail sends a welcome email after onboarding completion
func (s *NotificationService) SendWelcomeEmail(ctx context.Context, email, firstName string) error {
	payload := map[string]interface{}{
		"recipient":  email,
		"email_type": "welcome",
		"first_name": firstName,
	}

	return s.sendEmail(ctx, payload)
}

// SendAccountCreatedEmail sends account created email with account details (legacy)
func (s *NotificationService) SendAccountCreatedEmail(ctx context.Context, email, firstName, businessName, subdomain string) error {
	payload := map[string]interface{}{
		"recipient":     email,
		"email_type":    "account_created",
		"first_name":    firstName,
		"business_name": businessName,
		"subdomain":     subdomain,
	}

	return s.sendEmail(ctx, payload)
}

// WelcomePackEmailData contains all data for the welcome pack email
type WelcomePackEmailData struct {
	Email         string
	FirstName     string
	BusinessName  string
	TenantSlug    string
	AdminURL      string
	StorefrontURL string
	DashboardURL  string
}

// SendWelcomePackEmail sends a comprehensive welcome pack email with all tenant URLs and login info
func (s *NotificationService) SendWelcomePackEmail(ctx context.Context, data *WelcomePackEmailData) error {
	// Get base domain from environment
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}

	// Build storefront URL if not provided (subdomain-based: {slug}-store.{baseDomain})
	storefrontURL := data.StorefrontURL
	if storefrontURL == "" {
		storefrontURL = fmt.Sprintf("https://%s-store.%s", data.TenantSlug, baseDomain)
	}

	// Build dashboard URL if not provided
	dashboardURL := data.DashboardURL
	if dashboardURL == "" && data.AdminURL != "" {
		dashboardURL = data.AdminURL + "/dashboard"
	}

	payload := map[string]interface{}{
		"recipient":      data.Email,
		"email_type":     "welcome_pack",
		"first_name":     data.FirstName,
		"business_name":  data.BusinessName,
		"tenant_slug":    data.TenantSlug,
		"admin_url":      data.AdminURL,
		"storefront_url": storefrontURL,
		"dashboard_url":  dashboardURL,
		"login_email":    data.Email,
		// Include helpful links
		"help_center_url":   "https://help.tesserix.app",
		"documentation_url": "https://docs.tesserix.app",
		"support_email":     "support@tesserix.app",
	}

	// Try to send via email service
	if err := s.sendEmail(ctx, payload); err != nil {
		// Log but continue - email service might not be configured in dev
		// SECURITY: Mask PII in logs
		fmt.Printf("Warning: Failed to send welcome pack via service: %v\n", err)
		fmt.Printf("[Welcome Pack Email]\n")
		fmt.Printf("  To: %s\n", security.MaskEmail(data.Email))
		fmt.Printf("  Business: %s\n", data.BusinessName)
		fmt.Printf("  Admin URL: %s\n", data.AdminURL)
		fmt.Printf("  Storefront URL: %s\n", storefrontURL)
		fmt.Printf("  Dashboard URL: %s\n", dashboardURL)
		fmt.Printf("  Login Email: %s\n", security.MaskEmail(data.Email))
		return nil // Don't fail the operation
	}

	fmt.Printf("Sent welcome pack email to %s for business %s\n", security.MaskEmail(data.Email), data.BusinessName)
	return nil
}

// sendEmail sends an email via the verification service
func (s *NotificationService) sendEmail(ctx context.Context, payload map[string]interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/email/send", s.verificationServiceURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.verificationAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResponse map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorResponse)
		return fmt.Errorf("verification service error (status %d): %v", resp.StatusCode, errorResponse)
	}

	return nil
}

// SendOnboardingStatusUpdate sends status update notifications
func (s *NotificationService) SendOnboardingStatusUpdate(ctx context.Context, email, status string, details map[string]interface{}) error {
	// TODO: Send status update email
	fmt.Printf("Sending status update to %s: %s\n", security.MaskEmail(email), status)
	return nil
}

// SendDocumentationEmail sends documentation and resources
func (s *NotificationService) SendDocumentationEmail(ctx context.Context, email string, resources []string) error {
	// TODO: Send email with helpful resources and documentation
	fmt.Printf("Sending documentation email to %s with resources: %v\n", security.MaskEmail(email), resources)
	return nil
}

// SendPaymentSetupReminder sends payment setup reminder
func (s *NotificationService) SendPaymentSetupReminder(ctx context.Context, email string) error {
	// TODO: Send reminder to complete payment setup
	fmt.Printf("Sending payment setup reminder to %s\n", security.MaskEmail(email))
	return nil
}

// SendDraftReminderEmail sends a reminder email for incomplete onboarding drafts
func (s *NotificationService) SendDraftReminderEmail(ctx context.Context, email, firstName, sessionID string) error {
	// Build continue onboarding URL
	onboardingBaseURL := os.Getenv("ONBOARDING_APP_URL")
	if onboardingBaseURL == "" {
		onboardingBaseURL = "http://localhost:3002" // default for dev
	}
	continueURL := fmt.Sprintf("%s/onboarding?session=%s", onboardingBaseURL, sessionID)

	payload := map[string]interface{}{
		"recipient":    email,
		"email_type":   "draft_reminder",
		"first_name":   firstName,
		"session_id":   sessionID,
		"continue_url": continueURL,
	}

	// Try to send via verification service
	if err := s.sendEmail(ctx, payload); err != nil {
		// Log but continue - email service might not be configured
		// SECURITY: Mask PII in logs
		fmt.Printf("Warning: Failed to send draft reminder via service: %v\n", err)
		fmt.Printf("[Draft Reminder] To: %s, Name: %s, Session: %s, URL: %s\n",
			security.MaskEmail(email), security.MaskName(firstName), sessionID, continueURL)
		return nil // Don't fail the whole operation
	}

	fmt.Printf("Sent draft reminder email to %s for session %s\n", security.MaskEmail(email), sessionID)
	return nil
}
