package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// NotificationClient handles communication with notification-service for sending emails
type NotificationClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewNotificationClient creates a new notification service client
// This connects directly to notification-service for email delivery
func NewNotificationClient(baseURL string, apiKey string) *NotificationClient {
	// Use notification-service URL (override verification-service URL)
	notificationURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notificationURL == "" {
		notificationURL = "http://notification-service.devtest.svc.cluster.local:8090"
	}
	return &NotificationClient{
		baseURL: notificationURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NotificationSendRequest represents a request to notification-service /api/v1/notifications/send
type NotificationSendRequest struct {
	Channel        string `json:"channel"`
	RecipientEmail string `json:"recipientEmail"`
	Subject        string `json:"subject"`
	Body           string `json:"body"`
	BodyHTML       string `json:"bodyHtml"`
	Priority       string `json:"priority,omitempty"`
}

// SendEmailResponse represents the response from sending an email
type SendEmailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// SendVerificationLinkEmail sends an email with a verification link via notification-service
func (c *NotificationClient) SendVerificationLinkEmail(ctx context.Context, email, verificationLink, businessName string) error {
	// Generate the beautiful HTML email
	htmlBody, err := renderVerificationEmailTemplate(verificationLink, businessName, email)
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	req := &NotificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: email,
		Subject:        fmt.Sprintf("Verify your email for %s", businessName),
		Body:           fmt.Sprintf("Click this link to verify your email: %s", verificationLink),
		BodyHTML:       htmlBody,
		Priority:       "high",
	}

	var response struct {
		Success bool        `json:"success"`
		Data    interface{} `json:"data,omitempty"`
		Error   string      `json:"error,omitempty"`
	}

	if err := c.makeRequest(ctx, "POST", "/api/v1/notifications/send", req, &response); err != nil {
		return fmt.Errorf("failed to call notification-service: %w", err)
	}

	if !response.Success && response.Error != "" {
		return fmt.Errorf("notification-service error: %s", response.Error)
	}

	return nil
}

// renderVerificationEmailTemplate generates the verification email HTML
func renderVerificationEmailTemplate(verificationLink, businessName, email string) (string, error) {
	const emailTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verify Your Email</title>
</head>
<body style="margin: 0; padding: 0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f7fa;">
    <table role="presentation" style="width: 100%; border-collapse: collapse;">
        <tr>
            <td align="center" style="padding: 40px 0;">
                <table role="presentation" style="width: 600px; max-width: 100%; border-collapse: collapse; background-color: #ffffff; border-radius: 16px; box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);">
                    <!-- Header with gradient -->
                    <tr>
                        <td style="background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 50%, #a855f7 100%); padding: 40px 40px 30px; border-radius: 16px 16px 0 0; text-align: center;">
                            <h1 style="color: #ffffff; margin: 0; font-size: 28px; font-weight: 600;">
                                ✉️ Verify Your Email
                            </h1>
                            <p style="color: rgba(255, 255, 255, 0.9); margin: 12px 0 0; font-size: 16px;">
                                One click away from launching {{.BusinessName}}
                            </p>
                        </td>
                    </tr>

                    <!-- Main content -->
                    <tr>
                        <td style="padding: 40px;">
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                Hi there! 👋
                            </p>
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                You're almost ready to launch <strong>{{.BusinessName}}</strong>. Just click the button below to verify your email address and complete your store setup.
                            </p>

                            <!-- CTA Button -->
                            <table role="presentation" style="width: 100%; border-collapse: collapse;">
                                <tr>
                                    <td align="center" style="padding: 16px 0 32px;">
                                        <a href="{{.VerificationLink}}" style="display: inline-block; background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%); color: #ffffff; text-decoration: none; padding: 16px 48px; border-radius: 12px; font-size: 16px; font-weight: 600; box-shadow: 0 4px 14px rgba(99, 102, 241, 0.4);">
                                            Verify Email Address
                                        </a>
                                    </td>
                                </tr>
                            </table>

                            <!-- Alternative link -->
                            <div style="background-color: #f9fafb; border-radius: 12px; padding: 20px; margin-bottom: 24px;">
                                <p style="color: #6b7280; font-size: 14px; margin: 0 0 8px;">
                                    Or copy this link into your browser:
                                </p>
                                <p style="color: #6366f1; font-size: 13px; margin: 0; word-break: break-all;">
                                    {{.VerificationLink}}
                                </p>
                            </div>

                            <!-- Security notice -->
                            <div style="border-left: 4px solid #f59e0b; background-color: #fffbeb; padding: 16px; border-radius: 0 8px 8px 0;">
                                <p style="color: #92400e; font-size: 14px; margin: 0;">
                                    ⏰ This link expires in <strong>24 hours</strong>. If you didn't request this, you can safely ignore this email.
                                </p>
                            </div>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="background-color: #f9fafb; padding: 24px 40px; border-radius: 0 0 16px 16px; text-align: center;">
                            <p style="color: #9ca3af; font-size: 13px; margin: 0 0 8px;">
                                Sent to {{.Email}}
                            </p>
                            <p style="color: #9ca3af; font-size: 13px; margin: 0;">
                                © 2026 Tesseract Hub. All rights reserved.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

	tmpl, err := template.New("verification").Parse(emailTemplate)
	if err != nil {
		return "", err
	}

	data := struct {
		VerificationLink string
		BusinessName     string
		Email            string
	}{
		VerificationLink: verificationLink,
		BusinessName:     businessName,
		Email:            email,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// SendVerificationSuccessEmail sends a confirmation email after successful verification
func (c *NotificationClient) SendVerificationSuccessEmail(ctx context.Context, email, businessName string) error {
	// For now, we don't send a success email - handled in welcome pack
	return nil
}

// SendWelcomeEmail sends a welcome email to the user
func (c *NotificationClient) SendWelcomeEmail(ctx context.Context, email, firstName string) error {
	// Generate welcome email HTML
	htmlBody := renderWelcomeEmailTemplate(firstName)

	req := &NotificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: email,
		Subject:        "Welcome to Tesseract Hub!",
		Body:           fmt.Sprintf("Welcome %s! Your account has been created.", firstName),
		BodyHTML:       htmlBody,
	}

	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	return c.makeRequest(ctx, "POST", "/api/v1/notifications/send", req, &response)
}

// SendAccountCreatedEmail sends an account created email
func (c *NotificationClient) SendAccountCreatedEmail(ctx context.Context, email, firstName, businessName, subdomain string) error {
	// Handled by welcome pack email
	return nil
}

// WelcomePackData contains all data for sending a welcome pack email
type WelcomePackData struct {
	Email         string
	FirstName     string
	BusinessName  string
	TenantSlug    string
	AdminURL      string
	StorefrontURL string
	DashboardURL  string
}

// SendWelcomePackEmail sends a comprehensive welcome pack email with tenant URLs and login info
func (c *NotificationClient) SendWelcomePackEmail(ctx context.Context, data *WelcomePackData) error {
	htmlBody, err := renderWelcomePackEmailTemplate(data)
	if err != nil {
		return fmt.Errorf("failed to render welcome pack template: %w", err)
	}

	req := &NotificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: data.Email,
		Subject:        fmt.Sprintf("🎉 Your store %s is ready!", data.BusinessName),
		Body:           fmt.Sprintf("Welcome to %s! Your store is now live.", data.BusinessName),
		BodyHTML:       htmlBody,
		Priority:       "high",
	}

	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	return c.makeRequest(ctx, "POST", "/api/v1/notifications/send", req, &response)
}

// renderWelcomeEmailTemplate generates a simple welcome email
func renderWelcomeEmailTemplate(firstName string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><body style="font-family: Arial, sans-serif; background-color: #f4f7fa; padding: 40px;">
<div style="max-width: 600px; margin: 0 auto; background: white; border-radius: 16px; padding: 40px;">
<h1 style="color: #6366f1;">Welcome, %s! 🎉</h1>
<p>Your account has been created successfully.</p>
<p>Get started by exploring your dashboard.</p>
</div></body></html>`, firstName)
}

// renderWelcomePackEmailTemplate generates the welcome pack email HTML
func renderWelcomePackEmailTemplate(data *WelcomePackData) (string, error) {
	const emailTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Your Store is Ready!</title>
</head>
<body style="margin: 0; padding: 0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f7fa;">
    <table role="presentation" style="width: 100%; border-collapse: collapse;">
        <tr>
            <td align="center" style="padding: 40px 0;">
                <table role="presentation" style="width: 600px; max-width: 100%; border-collapse: collapse; background-color: #ffffff; border-radius: 16px; box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);">
                    <!-- Header -->
                    <tr>
                        <td style="background: linear-gradient(135deg, #10b981 0%, #059669 100%); padding: 40px; border-radius: 16px 16px 0 0; text-align: center;">
                            <h1 style="color: #ffffff; margin: 0; font-size: 32px;">🎉 Congratulations!</h1>
                            <p style="color: rgba(255,255,255,0.9); margin: 12px 0 0; font-size: 18px;">
                                {{.BusinessName}} is now live!
                            </p>
                        </td>
                    </tr>

                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                Hi {{.FirstName}}! 👋
                            </p>
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 32px;">
                                Your store has been set up successfully. Here are your important links:
                            </p>

                            <!-- Links -->
                            <div style="background: #f9fafb; border-radius: 12px; padding: 24px; margin-bottom: 24px;">
                                <h3 style="color: #111827; margin: 0 0 16px; font-size: 16px;">📋 Your Store URLs</h3>

                                <p style="margin: 0 0 12px;">
                                    <strong style="color: #6b7280;">Admin Panel:</strong><br>
                                    <a href="{{.AdminURL}}" style="color: #6366f1; text-decoration: none;">{{.AdminURL}}</a>
                                </p>

                                <p style="margin: 0;">
                                    <strong style="color: #6b7280;">Storefront:</strong><br>
                                    <a href="{{.StorefrontURL}}" style="color: #6366f1; text-decoration: none;">{{.StorefrontURL}}</a>
                                </p>
                            </div>

                            <!-- CTA -->
                            <table role="presentation" style="width: 100%;">
                                <tr>
                                    <td align="center">
                                        <a href="{{.AdminURL}}" style="display: inline-block; background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%); color: #ffffff; text-decoration: none; padding: 16px 48px; border-radius: 12px; font-size: 16px; font-weight: 600;">
                                            Open Admin Panel →
                                        </a>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="background-color: #f9fafb; padding: 24px 40px; border-radius: 0 0 16px 16px; text-align: center;">
                            <p style="color: #9ca3af; font-size: 13px; margin: 0;">
                                © 2026 Tesseract Hub. All rights reserved.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`

	tmpl, err := template.New("welcomepack").Parse(emailTemplate)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// makeRequest makes an HTTP request to notification-service
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
	// notification-service requires tenant_id - use "onboarding" for system emails
	req.Header.Set("X-Tenant-ID", "onboarding")
	// Add API key if provided
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-2xx status codes
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification-service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
