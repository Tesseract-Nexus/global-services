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

// CustomDomainDNSConfig holds DNS configuration for custom domain verification emails
type CustomDomainDNSConfig struct {
	IsCustomDomain bool   // If true, show DNS instructions in the email
	CustomDomain   string // The custom domain (e.g., "customdomain.com")

	// Customer's subdomain hosts (what they configure DNS for)
	AdminHost      string // Admin panel host (e.g., "admin.customdomain.com")
	StorefrontHost string // Storefront host (e.g., "www.customdomain.com" or "customdomain.com")
	APIHost        string // API host (e.g., "api.customdomain.com")

	// Routing configuration
	// UseARecords: true = A records to IP, false = CNAME records
	// Custom domains must use A records due to Cloudflare cross-account CNAME restrictions
	UseARecords        bool   // If true, show A records instead of CNAME for routing
	RoutingIP          string // LoadBalancer IP for A records (e.g., "34.151.169.37")
	RoutingCNAMETarget string // CNAME target for routing (fallback if IP not set)

	// Tenant identification
	TenantSlug string // The tenant slug (e.g., "awesome-store")
	BaseDomain string // Platform base domain (e.g., "tesserix.app")

	// CNAME Delegation for automatic SSL certificate management
	// Customer adds: _acme-challenge.theirdomain.com CNAME theirdomain-com.acme.tesserix.app
	// cert-manager follows the CNAME and creates TXT records in our Cloudflare zone
	UseCNAMEDelegation bool   // If true, show CNAME delegation option in email
	ACMEChallengeHost  string // The _acme-challenge subdomain (e.g., "_acme-challenge.customdomain.com")
	ACMECNAMETarget    string // The CNAME target for ACME challenges (e.g., "customdomain-com.acme.tesserix.app")
}

// SendVerificationLinkEmail sends an email with a verification link via notification-service
func (c *NotificationClient) SendVerificationLinkEmail(ctx context.Context, email, verificationLink, businessName string) error {
	return c.SendVerificationLinkEmailWithDNS(ctx, email, verificationLink, businessName, nil)
}

// SendVerificationLinkEmailWithDNS sends a verification email with optional DNS configuration for custom domains
func (c *NotificationClient) SendVerificationLinkEmailWithDNS(ctx context.Context, email, verificationLink, businessName string, dnsConfig *CustomDomainDNSConfig) error {
	// Generate the beautiful HTML email with optional DNS instructions
	htmlBody, err := renderVerificationEmailTemplate(verificationLink, businessName, email, dnsConfig)
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	subject := fmt.Sprintf("Verify your email for %s", businessName)
	if dnsConfig != nil && dnsConfig.IsCustomDomain {
		subject = fmt.Sprintf("Verify your email & configure DNS for %s", businessName)
	}

	req := &NotificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: email,
		Subject:        subject,
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

// renderVerificationEmailTemplate generates the verification email HTML with optional DNS instructions
func renderVerificationEmailTemplate(verificationLink, businessName, email string, dnsConfig *CustomDomainDNSConfig) (string, error) {
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

                            {{if .IsCustomDomain}}
                            <!-- Custom Domain DNS Instructions - IMPORTANT -->
                            <div style="background: linear-gradient(135deg, #fef3c7 0%, #fde68a 100%); border-radius: 12px; padding: 24px; margin-bottom: 24px; border: 2px solid #f59e0b;">
                                <h2 style="color: #92400e; font-size: 20px; font-weight: 700; margin: 0 0 16px;">
                                    🌐 ACTION REQUIRED: Set Up Your DNS Records
                                </h2>
                                <p style="color: #78350f; font-size: 15px; line-height: 1.6; margin: 0 0 20px;">
                                    To use your custom domain <strong style="font-size: 16px;">{{.CustomDomain}}</strong>, you need to add CNAME records pointing to our platform. <strong>You can do this now while your email is being verified!</strong>
                                </p>

                                {{if .UseCNAMEDelegation}}
                                <!-- CNAME Delegation Section - Automatic SSL Certificates (FIRST - Most Important) -->
                                <div style="background: linear-gradient(135deg, #10b981 0%, #059669 100%); border-radius: 12px; padding: 24px; margin-bottom: 20px;">
                                    <h3 style="color: #ffffff; font-size: 18px; font-weight: 700; margin: 0 0 12px;">
                                        🔐 Step 1: Enable Automatic SSL (Recommended)
                                    </h3>
                                    <p style="color: #d1fae5; font-size: 14px; line-height: 1.6; margin: 0 0 16px;">
                                        Add this CNAME record once, and we'll <strong>automatically issue and renew SSL certificates</strong> for your domain forever!
                                    </p>

                                    <div style="background-color: rgba(255,255,255,0.95); border-radius: 8px; padding: 16px;">
                                        <table style="width: 100%; border-collapse: collapse; font-size: 13px;">
                                            <tr style="background-color: #f0fdf4;">
                                                <th style="padding: 10px; text-align: left; color: #166534; border-bottom: 2px solid #10b981;">Type</th>
                                                <th style="padding: 10px; text-align: left; color: #166534; border-bottom: 2px solid #10b981;">Host</th>
                                                <th style="padding: 10px; text-align: left; color: #166534; border-bottom: 2px solid #10b981;">Value</th>
                                            </tr>
                                            <tr>
                                                <td style="padding: 12px 10px; font-family: monospace; font-weight: 600; color: #18181b;">CNAME</td>
                                                <td style="padding: 12px 10px; font-family: monospace; color: #4b5563;">{{.ACMEChallengeHost}}</td>
                                                <td style="padding: 12px 10px; font-family: monospace; color: #10b981; font-weight: 600;">{{.ACMECNAMETarget}}</td>
                                            </tr>
                                        </table>
                                    </div>

                                    <p style="color: #d1fae5; font-size: 12px; margin: 12px 0 0; line-height: 1.6;">
                                        ✅ Certificates auto-renew without any action from you<br>
                                        ✅ SSL can be issued <strong>before</strong> your domain points to us
                                    </p>
                                </div>
                                {{end}}

                                <!-- Routing target highlight -->
                                <div style="background-color: #ffffff; border-radius: 8px; padding: 20px; margin-bottom: 16px; text-align: center; border: 2px solid #6366f1;">
                                    <p style="color: #374151; font-size: 14px; margin: 0 0 8px;">All your domains should point to:</p>
                                    {{if .UseARecords}}
                                    <p style="color: #6366f1; font-size: 22px; font-weight: 700; font-family: monospace; margin: 0;">{{.RoutingIP}}</p>
                                    <p style="color: #6b7280; font-size: 12px; margin: 8px 0 0;">(Use A records pointing to this IP address)</p>
                                    {{else}}
                                    <p style="color: #6366f1; font-size: 22px; font-weight: 700; font-family: monospace; margin: 0;">{{.RoutingCNAMETarget}}</p>
                                    {{end}}
                                </div>

                                <!-- Step-by-step Instructions for Routing -->
                                <div style="background-color: #ffffff; border-radius: 8px; padding: 20px; margin-bottom: 16px;">
                                    <p style="color: #1f2937; font-size: 14px; font-weight: 600; margin: 0 0 16px;">
                                        📝 {{if .UseCNAMEDelegation}}Step 2: {{end}}Add these {{if .UseARecords}}A{{else}}CNAME{{end}} records for routing:
                                    </p>

                                    <!-- Root domain -->
                                    <div style="margin-bottom: 16px; padding-left: 12px; border-left: 3px solid #8b5cf6;">
                                        <p style="color: #8b5cf6; font-size: 12px; font-weight: 600; margin: 0 0 4px; text-transform: uppercase;">Root Domain</p>
                                        <table style="width: 100%; border-collapse: collapse; background-color: #f9fafb; border-radius: 6px;">
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280; width: 60px;">Type:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; font-weight: 600; color: #18181b;">{{if .UseARecords}}A{{else}}CNAME{{end}}</td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280;">Name:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; font-weight: 600; color: #18181b;">@</td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280;">Value:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; color: #6366f1;"><strong>{{if .UseARecords}}{{.RoutingIP}}{{else}}{{.RoutingCNAMETarget}}{{end}}</strong></td>
                                            </tr>
                                        </table>
                                    </div>

                                    <!-- Storefront (www) -->
                                    <div style="margin-bottom: 16px; padding-left: 12px; border-left: 3px solid #6366f1;">
                                        <p style="color: #6366f1; font-size: 12px; font-weight: 600; margin: 0 0 4px; text-transform: uppercase;">Storefront (www)</p>
                                        <table style="width: 100%; border-collapse: collapse; background-color: #f9fafb; border-radius: 6px;">
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280; width: 60px;">Type:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; font-weight: 600; color: #18181b;">{{if .UseARecords}}A{{else}}CNAME{{end}}</td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280;">Name:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; font-weight: 600; color: #18181b;">www</td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280;">Value:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; color: #6366f1;"><strong>{{if .UseARecords}}{{.RoutingIP}}{{else}}{{.RoutingCNAMETarget}}{{end}}</strong></td>
                                            </tr>
                                        </table>
                                    </div>

                                    <!-- Admin -->
                                    <div style="margin-bottom: 16px; padding-left: 12px; border-left: 3px solid #10b981;">
                                        <p style="color: #10b981; font-size: 12px; font-weight: 600; margin: 0 0 4px; text-transform: uppercase;">Admin Panel</p>
                                        <table style="width: 100%; border-collapse: collapse; background-color: #f9fafb; border-radius: 6px;">
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280; width: 60px;">Type:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; font-weight: 600; color: #18181b;">{{if .UseARecords}}A{{else}}CNAME{{end}}</td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280;">Name:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; font-weight: 600; color: #18181b;">admin</td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280;">Value:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; color: #6366f1;"><strong>{{if .UseARecords}}{{.RoutingIP}}{{else}}{{.RoutingCNAMETarget}}{{end}}</strong></td>
                                            </tr>
                                        </table>
                                    </div>

                                    <!-- API -->
                                    <div style="padding-left: 12px; border-left: 3px solid #f59e0b;">
                                        <p style="color: #f59e0b; font-size: 12px; font-weight: 600; margin: 0 0 4px; text-transform: uppercase;">API (Optional)</p>
                                        <table style="width: 100%; border-collapse: collapse; background-color: #f9fafb; border-radius: 6px;">
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280; width: 60px;">Type:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; font-weight: 600; color: #18181b;">{{if .UseARecords}}A{{else}}CNAME{{end}}</td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280;">Name:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; font-weight: 600; color: #18181b;">api</td>
                                            </tr>
                                            <tr>
                                                <td style="padding: 8px 12px; font-size: 12px; color: #6b7280;">Value:</td>
                                                <td style="padding: 8px 12px; font-size: 14px; font-family: monospace; color: #6366f1;"><strong>{{if .UseARecords}}{{.RoutingIP}}{{else}}{{.RoutingCNAMETarget}}{{end}}</strong></td>
                                            </tr>
                                        </table>
                                    </div>
                                </div>

                                <!-- Summary table -->
                                <div style="background-color: #ffffff; border-radius: 8px; padding: 16px; margin-bottom: 16px; border: 1px dashed #d1d5db;">
                                    <p style="color: #374151; font-size: 13px; font-weight: 600; margin: 0 0 12px;">📋 Quick Reference - All DNS Records</p>
                                    <table style="width: 100%; border-collapse: collapse; font-size: 12px;">
                                        <tr style="background-color: #f3f4f6;">
                                            <th style="padding: 8px; text-align: left; color: #374151; border-bottom: 1px solid #e5e7eb;">Type</th>
                                            <th style="padding: 8px; text-align: left; color: #374151; border-bottom: 1px solid #e5e7eb;">Name</th>
                                            <th style="padding: 8px; text-align: left; color: #374151; border-bottom: 1px solid #e5e7eb;">Value</th>
                                        </tr>
                                        {{if .UseCNAMEDelegation}}
                                        <tr style="background-color: #f0fdf4;">
                                            <td style="padding: 8px; font-family: monospace; color: #166534; font-weight: 600;">CNAME</td>
                                            <td style="padding: 8px; font-family: monospace; color: #166534;">{{.ACMEChallengeHost}}</td>
                                            <td style="padding: 8px; font-family: monospace; color: #166534;">{{.ACMECNAMETarget}}</td>
                                        </tr>
                                        {{end}}
                                        <tr>
                                            <td style="padding: 8px; font-family: monospace; color: #4b5563;">{{if .UseARecords}}A{{else}}CNAME{{end}}</td>
                                            <td style="padding: 8px; font-family: monospace; color: #4b5563;">@</td>
                                            <td style="padding: 8px; font-family: monospace; color: #6366f1;">{{if .UseARecords}}{{.RoutingIP}}{{else}}{{.RoutingCNAMETarget}}{{end}}</td>
                                        </tr>
                                        <tr style="background-color: #f9fafb;">
                                            <td style="padding: 8px; font-family: monospace; color: #4b5563;">{{if .UseARecords}}A{{else}}CNAME{{end}}</td>
                                            <td style="padding: 8px; font-family: monospace; color: #4b5563;">www</td>
                                            <td style="padding: 8px; font-family: monospace; color: #6366f1;">{{if .UseARecords}}{{.RoutingIP}}{{else}}{{.RoutingCNAMETarget}}{{end}}</td>
                                        </tr>
                                        <tr>
                                            <td style="padding: 8px; font-family: monospace; color: #4b5563;">{{if .UseARecords}}A{{else}}CNAME{{end}}</td>
                                            <td style="padding: 8px; font-family: monospace; color: #4b5563;">admin</td>
                                            <td style="padding: 8px; font-family: monospace; color: #6366f1;">{{if .UseARecords}}{{.RoutingIP}}{{else}}{{.RoutingCNAMETarget}}{{end}}</td>
                                        </tr>
                                        <tr style="background-color: #f9fafb;">
                                            <td style="padding: 8px; font-family: monospace; color: #4b5563;">{{if .UseARecords}}A{{else}}CNAME{{end}}</td>
                                            <td style="padding: 8px; font-family: monospace; color: #4b5563;">api</td>
                                            <td style="padding: 8px; font-family: monospace; color: #6366f1;">{{if .UseARecords}}{{.RoutingIP}}{{else}}{{.RoutingCNAMETarget}}{{end}}</td>
                                        </tr>
                                    </table>
                                </div>

                                <!-- Tips -->
                                <div style="background-color: #ffffff; border-radius: 8px; padding: 12px; border-left: 4px solid #3b82f6;">
                                    <p style="color: #1e40af; font-size: 13px; margin: 0; line-height: 1.6;">
                                        💡 <strong>Tips:</strong><br>
                                        {{if .UseARecords}}
                                        • All records point to the <strong>same IP address</strong> - easy to configure!<br>
                                        • DNS changes usually take <strong>5-30 minutes</strong> to propagate<br>
                                        • We'll automatically provision your <strong>SSL certificate</strong><br>
                                        • If using <strong>Cloudflare</strong>, set the proxy status to <strong>"DNS only"</strong> (grey cloud)
                                        {{else}}
                                        • All subdomains use the <strong>same CNAME target</strong> - easy to configure!<br>
                                        • DNS changes usually take <strong>5-30 minutes</strong> to propagate<br>
                                        • We'll automatically provision your <strong>SSL certificate</strong><br>
                                        • If using <strong>Cloudflare</strong>, you can keep proxy mode enabled
                                        {{end}}
                                    </p>
                                </div>
                            </div>
                            {{end}}

                            <!-- Security notice -->
                            <div style="border-left: 4px solid #f59e0b; background-color: #fffbeb; padding: 16px; border-radius: 0 8px 8px 0;">
                                <p style="color: #92400e; font-size: 14px; margin: 0;">
                                    ⏰ This link expires in <strong>{{.ExpiryTime}}</strong>. If you didn't request this, you can safely ignore this email.
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
		VerificationLink   string
		BusinessName       string
		Email              string
		ExpiryTime         string // Human-readable expiry time (e.g., "1 hour")
		IsCustomDomain     bool
		CustomDomain       string
		AdminHost          string
		StorefrontHost     string
		APIHost            string
		TenantSlug         string
		BaseDomain         string
		// Routing configuration - A records vs CNAME
		UseARecords        bool   // If true, show A records instead of CNAME
		RoutingIP          string // LoadBalancer IP for A records
		RoutingCNAMETarget string // CNAME target for routing (fallback)
		// CNAME Delegation fields for automatic SSL
		UseCNAMEDelegation bool
		ACMEChallengeHost  string // e.g., "_acme-challenge.customdomain.com"
		ACMECNAMETarget    string // e.g., "customdomain-com.acme.tesserix.app"
	}{
		VerificationLink: verificationLink,
		BusinessName:     businessName,
		Email:            email,
		ExpiryTime:       "1 hour", // Matches VERIFICATION_TOKEN_EXPIRY_HOURS config
	}

	// Add DNS config if provided
	if dnsConfig != nil && dnsConfig.IsCustomDomain {
		data.IsCustomDomain = true
		data.CustomDomain = dnsConfig.CustomDomain
		data.AdminHost = dnsConfig.AdminHost
		data.StorefrontHost = dnsConfig.StorefrontHost
		data.APIHost = dnsConfig.APIHost
		data.TenantSlug = dnsConfig.TenantSlug
		data.BaseDomain = dnsConfig.BaseDomain
		// Routing configuration
		data.UseARecords = dnsConfig.UseARecords
		data.RoutingIP = dnsConfig.RoutingIP
		data.RoutingCNAMETarget = dnsConfig.RoutingCNAMETarget
		// CNAME Delegation fields for automatic SSL
		data.UseCNAMEDelegation = dnsConfig.UseCNAMEDelegation
		data.ACMEChallengeHost = dnsConfig.ACMEChallengeHost
		data.ACMECNAMETarget = dnsConfig.ACMECNAMETarget
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

// SendCustomerWelcomeEmail sends a welcome email to a new customer who registered on a storefront
func (c *NotificationClient) SendCustomerWelcomeEmail(ctx context.Context, email, firstName, storeName string) error {
	htmlBody := renderCustomerWelcomeEmailTemplate(firstName, storeName)

	req := &NotificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: email,
		Subject:        fmt.Sprintf("Welcome to %s!", storeName),
		Body:           fmt.Sprintf("Welcome %s! Your account at %s has been created.", firstName, storeName),
		BodyHTML:       htmlBody,
	}

	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	return c.makeRequest(ctx, "POST", "/api/v1/notifications/send", req, &response)
}

// renderCustomerWelcomeEmailTemplate generates a welcome email for storefront customers
func renderCustomerWelcomeEmailTemplate(firstName, storeName string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome to %s!</title>
</head>
<body style="margin: 0; padding: 0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f7fa;">
    <table role="presentation" style="width: 100%%; border-collapse: collapse;">
        <tr>
            <td align="center" style="padding: 40px 0;">
                <table role="presentation" style="width: 600px; max-width: 100%%; border-collapse: collapse; background-color: #ffffff; border-radius: 16px; box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);">
                    <!-- Header with gradient -->
                    <tr>
                        <td style="background: linear-gradient(135deg, #6366f1 0%%, #8b5cf6 50%%, #a855f7 100%%); padding: 40px 40px 30px; border-radius: 16px 16px 0 0; text-align: center;">
                            <h1 style="color: #ffffff; margin: 0; font-size: 28px; font-weight: 600;">
                                🎉 Welcome to %s!
                            </h1>
                        </td>
                    </tr>

                    <!-- Main content -->
                    <tr>
                        <td style="padding: 40px;">
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                Hi %s! 👋
                            </p>
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                Thank you for creating an account with <strong>%s</strong>. We're excited to have you as part of our community!
                            </p>
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                You can now:
                            </p>
                            <ul style="color: #374151; font-size: 16px; line-height: 1.8; margin: 0 0 24px; padding-left: 24px;">
                                <li>Browse our products and collections</li>
                                <li>Save items to your wishlist</li>
                                <li>Track your orders</li>
                                <li>Manage your account settings</li>
                            </ul>

                            <!-- Check email notice -->
                            <div style="border-left: 4px solid #6366f1; background-color: #f0f0ff; padding: 16px; border-radius: 0 8px 8px 0; margin-top: 24px;">
                                <p style="color: #4338ca; font-size: 14px; margin: 0;">
                                    📧 Please check your inbox for an email verification code to complete your account setup.
                                </p>
                            </div>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="background-color: #f9fafb; padding: 24px 40px; border-radius: 0 0 16px 16px; text-align: center;">
                            <p style="color: #9ca3af; font-size: 13px; margin: 0 0 8px;">
                                This email was sent because you created an account at %s
                            </p>
                            <p style="color: #9ca3af; font-size: 13px; margin: 0;">
                                © 2026 Powered by Tesseract Hub
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`, storeName, storeName, firstName, storeName, storeName)
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

// GoodbyeEmailData contains data for the goodbye/deactivation email
type GoodbyeEmailData struct {
	Email            string
	FirstName        string
	StoreName        string
	DeactivatedAt    time.Time
	ScheduledPurgeAt time.Time
	ReactivationURL  string
}

// SendGoodbyeEmail sends a goodbye email when a customer deactivates their account
func (c *NotificationClient) SendGoodbyeEmail(ctx context.Context, data *GoodbyeEmailData) error {
	htmlBody := renderGoodbyeEmailTemplate(data)

	req := &NotificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: data.Email,
		Subject:        fmt.Sprintf("We're sorry to see you go from %s", data.StoreName),
		Body:           fmt.Sprintf("Your account at %s has been deactivated. You have 90 days to reactivate.", data.StoreName),
		BodyHTML:       htmlBody,
	}

	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	return c.makeRequest(ctx, "POST", "/api/v1/notifications/send", req, &response)
}

// PasswordResetEmailData contains data for password reset emails
type PasswordResetEmailData struct {
	Email        string
	FirstName    string
	StoreName    string
	ResetLink    string
	ExpiresIn    string // e.g., "1 hour"
}

// SendPasswordResetEmail sends a password reset email with a secure link
func (c *NotificationClient) SendPasswordResetEmail(ctx context.Context, data *PasswordResetEmailData) error {
	htmlBody := renderPasswordResetEmailTemplate(data)

	req := &NotificationSendRequest{
		Channel:        "EMAIL",
		RecipientEmail: data.Email,
		Subject:        fmt.Sprintf("Reset your password for %s", data.StoreName),
		Body:           fmt.Sprintf("Click this link to reset your password: %s. This link expires in %s.", data.ResetLink, data.ExpiresIn),
		BodyHTML:       htmlBody,
		Priority:       "high",
	}

	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	return c.makeRequest(ctx, "POST", "/api/v1/notifications/send", req, &response)
}

// renderPasswordResetEmailTemplate generates a password reset email
func renderPasswordResetEmailTemplate(data *PasswordResetEmailData) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Reset Your Password</title>
</head>
<body style="margin: 0; padding: 0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f7fa;">
    <table role="presentation" style="width: 100%%; border-collapse: collapse;">
        <tr>
            <td align="center" style="padding: 40px 0;">
                <table role="presentation" style="width: 600px; max-width: 100%%; border-collapse: collapse; background-color: #ffffff; border-radius: 16px; box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);">
                    <!-- Header -->
                    <tr>
                        <td style="background: linear-gradient(135deg, #6366f1 0%%, #8b5cf6 50%%, #a855f7 100%%); padding: 40px 40px 30px; border-radius: 16px 16px 0 0; text-align: center;">
                            <h1 style="color: #ffffff; margin: 0; font-size: 28px; font-weight: 600;">
                                🔐 Reset Your Password
                            </h1>
                        </td>
                    </tr>

                    <!-- Main content -->
                    <tr>
                        <td style="padding: 40px;">
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                Hi %s,
                            </p>
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                We received a request to reset your password for your <strong>%s</strong> account. Click the button below to create a new password.
                            </p>

                            <!-- CTA Button -->
                            <table role="presentation" style="width: 100%%; border-collapse: collapse;">
                                <tr>
                                    <td align="center" style="padding: 16px 0 32px;">
                                        <a href="%s" style="display: inline-block; background: linear-gradient(135deg, #6366f1 0%%, #8b5cf6 100%%); color: #ffffff; text-decoration: none; padding: 16px 48px; border-radius: 12px; font-size: 16px; font-weight: 600; box-shadow: 0 4px 14px rgba(99, 102, 241, 0.4);">
                                            Reset Password
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
                                    %s
                                </p>
                            </div>

                            <!-- Security notice -->
                            <div style="border-left: 4px solid #f59e0b; background-color: #fffbeb; padding: 16px; border-radius: 0 8px 8px 0;">
                                <p style="color: #92400e; font-size: 14px; margin: 0;">
                                    ⏰ This link expires in <strong>%s</strong>. If you didn't request this password reset, you can safely ignore this email.
                                </p>
                            </div>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="background-color: #f9fafb; padding: 24px 40px; border-radius: 0 0 16px 16px; text-align: center;">
                            <p style="color: #9ca3af; font-size: 13px; margin: 0 0 8px;">
                                This email was sent to %s
                            </p>
                            <p style="color: #9ca3af; font-size: 13px; margin: 0;">
                                © 2026 Powered by Tesseract Hub
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`, data.FirstName, data.StoreName, data.ResetLink, data.ResetLink, data.ExpiresIn, data.Email)
}

// renderGoodbyeEmailTemplate generates a goodbye email for deactivated accounts
func renderGoodbyeEmailTemplate(data *GoodbyeEmailData) string {
	purgeDate := data.ScheduledPurgeAt.Format("January 2, 2006")

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>We're sorry to see you go</title>
</head>
<body style="margin: 0; padding: 0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f7fa;">
    <table role="presentation" style="width: 100%%; border-collapse: collapse;">
        <tr>
            <td align="center" style="padding: 40px 0;">
                <table role="presentation" style="width: 600px; max-width: 100%%; border-collapse: collapse; background-color: #ffffff; border-radius: 16px; box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);">
                    <!-- Header -->
                    <tr>
                        <td style="background: linear-gradient(135deg, #64748b 0%%, #475569 100%%); padding: 40px 40px 30px; border-radius: 16px 16px 0 0; text-align: center;">
                            <h1 style="color: #ffffff; margin: 0; font-size: 28px; font-weight: 600;">
                                We're sorry to see you go
                            </h1>
                        </td>
                    </tr>

                    <!-- Main content -->
                    <tr>
                        <td style="padding: 40px;">
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                Hi %s,
                            </p>
                            <p style="color: #374151; font-size: 16px; line-height: 1.6; margin: 0 0 24px;">
                                Your account at <strong>%s</strong> has been deactivated as requested.
                            </p>

                            <!-- Info box -->
                            <div style="background-color: #fef3c7; border-left: 4px solid #f59e0b; padding: 20px; border-radius: 0 8px 8px 0; margin-bottom: 24px;">
                                <h3 style="color: #92400e; margin: 0 0 12px; font-size: 16px;">What happens next?</h3>
                                <ul style="color: #92400e; font-size: 14px; margin: 0; padding-left: 20px; line-height: 1.8;">
                                    <li>Your data will be safely retained for <strong>90 days</strong></li>
                                    <li>You can reactivate your account anytime before <strong>%s</strong></li>
                                    <li>After 90 days, your data will be permanently deleted</li>
                                </ul>
                            </div>

                            <!-- Changed your mind section -->
                            <div style="background-color: #f0fdf4; border-left: 4px solid #22c55e; padding: 20px; border-radius: 0 8px 8px 0; margin-bottom: 24px;">
                                <h3 style="color: #166534; margin: 0 0 12px; font-size: 16px;">Changed your mind?</h3>
                                <p style="color: #166534; font-size: 14px; margin: 0;">
                                    Simply log back in to reactivate your account. All your data will be restored instantly.
                                </p>
                            </div>

                            <!-- CTA Button -->
                            <table role="presentation" style="width: 100%%; border-collapse: collapse;">
                                <tr>
                                    <td align="center" style="padding: 16px 0;">
                                        <a href="%s" style="display: inline-block; background: linear-gradient(135deg, #22c55e 0%%, #16a34a 100%%); color: #ffffff; text-decoration: none; padding: 16px 48px; border-radius: 12px; font-size: 16px; font-weight: 600; box-shadow: 0 4px 14px rgba(34, 197, 94, 0.4);">
                                            Reactivate My Account
                                        </a>
                                    </td>
                                </tr>
                            </table>

                            <p style="color: #6b7280; font-size: 14px; line-height: 1.6; margin: 24px 0 0; text-align: center;">
                                We'd love to have you back! If you have any feedback on how we can improve, please let us know.
                            </p>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="background-color: #f9fafb; padding: 24px 40px; border-radius: 0 0 16px 16px; text-align: center;">
                            <p style="color: #9ca3af; font-size: 13px; margin: 0 0 8px;">
                                This email was sent because you deactivated your account at %s
                            </p>
                            <p style="color: #9ca3af; font-size: 13px; margin: 0;">
                                © 2026 Powered by Tesseract Hub
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>`, data.FirstName, data.StoreName, purgeDate, data.ReactivationURL, data.StoreName)
}
