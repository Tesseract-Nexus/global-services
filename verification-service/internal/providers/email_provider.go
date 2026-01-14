package providers

import (
	"fmt"
	"log"
	"os"

	"github.com/tesseract-hub/domains/common/services/verification-service/internal/templates"
)

// EmailProvider defines the interface for email providers
type EmailProvider interface {
	SendVerificationEmail(recipient, code, purpose string) error
	SendEmail(recipient, subject, htmlBody string) error
	GetName() string
}

// EmailMessage represents an email message
type EmailMessage struct {
	To      string
	Subject string
	Body    string
	HTML    string
}

// EmailProviderFactory creates an email provider
// All emails are routed through notification-service which handles Postal/SendGrid delivery
func EmailProviderFactory(providerName, apiKey, fromEmail, fromName string) (EmailProvider, error) {
	// Initialize template renderer
	if err := templates.Init(); err != nil {
		log.Printf("Warning: Failed to initialize email templates: %v. Using fallback templates.", err)
	}

	// All emails must go through notification-service
	// notification-service handles the actual delivery via Postal/SendGrid
	if providerName != "notification-service" && providerName != "" {
		log.Printf("Warning: Provider '%s' is deprecated. Using notification-service instead.", providerName)
	}

	baseURL := getEnvOrDefault("NOTIFICATION_SERVICE_URL", "http://notification-service.devtest.svc.cluster.local:8090")
	return NewNotificationServiceProvider(baseURL, apiKey, fromEmail, fromName), nil
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// FormatVerificationEmail formats the verification email content
func FormatVerificationEmail(code, purpose string) (string, string) {
	var subject, htmlBody string

	switch purpose {
	case "email_verification":
		// Try new template first
		if s, h, err := templates.RenderEmailVerificationDefault("", code, "", 10); err == nil {
			return s, h
		}
		// Fallback to legacy template
		subject = "Verify Your Email Address"
		htmlBody = fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4F46E5; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background-color: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .code { font-size: 32px; font-weight: bold; color: #4F46E5; letter-spacing: 8px; text-align: center; padding: 20px; background-color: white; border-radius: 8px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Email Verification</h1>
        </div>
        <div class="content">
            <p>Hello,</p>
            <p>Thank you for starting your onboarding with Tesseract Hub. Please use the following verification code to verify your email address:</p>
            <div class="code">%s</div>
            <p>This code will expire in 10 minutes.</p>
            <p>If you didn't request this code, please ignore this email.</p>
        </div>
        <div class="footer">
            <p>&copy; 2025 Tesseract Hub. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, code)

	case "password_reset":
		// Try new template first
		if s, h, err := templates.RenderPasswordResetDefault("", code, 10); err == nil {
			return s, h
		}
		// Fallback to legacy template
		subject = "Reset Your Password"
		htmlBody = fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #DC2626; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background-color: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .code { font-size: 32px; font-weight: bold; color: #DC2626; letter-spacing: 8px; text-align: center; padding: 20px; background-color: white; border-radius: 8px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Reset</h1>
        </div>
        <div class="content">
            <p>Hello,</p>
            <p>You requested to reset your password. Please use the following verification code:</p>
            <div class="code">%s</div>
            <p>This code will expire in 10 minutes.</p>
            <p>If you didn't request a password reset, please ignore this email and ensure your account is secure.</p>
        </div>
        <div class="footer">
            <p>&copy; 2025 Tesseract Hub. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, code)

	case "welcome":
		subject = "Welcome to Tesseract Hub!"
		htmlBody = fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #10B981; color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background-color: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .welcome-message { background-color: white; padding: 20px; border-radius: 8px; margin: 20px 0; border-left: 4px solid #10B981; }
        .cta-button { display: inline-block; background-color: #10B981; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .next-steps { background-color: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .next-steps li { margin: 10px 0; }
        .footer { text-align: center; margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎉 Welcome to Tesseract Hub!</h1>
        </div>
        <div class="content">
            <div class="welcome-message">
                <h2>Hello %s,</h2>
                <p>Congratulations! Your account has been successfully created and your store is ready to go.</p>
                <p>We're excited to have you on board and can't wait to see what you build with Tesseract Hub.</p>
            </div>

            <div class="next-steps">
                <h3>🚀 Next Steps:</h3>
                <ul>
                    <li><strong>Complete your store setup</strong> - Add your products, configure shipping, and customize your storefront</li>
                    <li><strong>Test your store</strong> - Make test orders in development mode</li>
                    <li><strong>Go live</strong> - When ready, connect your payment gateway and launch!</li>
                </ul>
            </div>

            <div style="text-align: center;">
                <a href="https://admin.tesserix.app/dashboard" class="cta-button">Go to Dashboard</a>
            </div>

            <p style="margin-top: 30px;">Need help? Check out our <a href="https://docs.tesserix.app">documentation</a> or reach out to our support team.</p>
        </div>
        <div class="footer">
            <p>&copy; 2025 Tesseract Hub. All rights reserved.</p>
            <p>You're receiving this email because you created an account at Tesseract Hub.</p>
        </div>
    </div>
</body>
</html>
`, code)

	case "account_created":
		subject = "Your Tesseract Hub Account is Ready!"
		htmlBody = fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #6366F1; color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background-color: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .info-box { background-color: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .info-row { display: flex; justify-content: space-between; padding: 10px 0; border-bottom: 1px solid #e5e7eb; }
        .info-label { font-weight: bold; color: #6b7280; }
        .info-value { color: #111827; }
        .highlight { background-color: #EEF2FF; padding: 15px; border-radius: 6px; margin: 15px 0; border-left: 4px solid #6366F1; }
        .cta-button { display: inline-block; background-color: #6366F1; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>✓ Account Created Successfully!</h1>
        </div>
        <div class="content">
            <p>Hello %s,</p>
            <p>Great news! Your Tesseract Hub account has been created successfully.</p>

            <div class="info-box">
                <h3>Your Account Details:</h3>
                <div class="info-row">
                    <span class="info-label">Business Name:</span>
                    <span class="info-value">%s</span>
                </div>
                <div class="info-row">
                    <span class="info-label">Store URL:</span>
                    <span class="info-value">https://%s.tesserix.app</span>
                </div>
                <div class="info-row">
                    <span class="info-label">Admin Portal:</span>
                    <span class="info-value">https://admin.tesserix.app</span>
                </div>
            </div>

            <div class="highlight">
                <strong>💡 Pro Tip:</strong> Your store is currently in <strong>development mode</strong>. This means you can test everything without going live. When you're ready to accept real orders, you can switch to live mode from your dashboard.
            </div>

            <div style="text-align: center;">
                <a href="https://admin.tesserix.app/dashboard" class="cta-button">Access Your Dashboard</a>
            </div>

            <p style="margin-top: 30px;">Questions? We're here to help! Contact us at <a href="mailto:support@tesserix.app">support@tesserix.app</a></p>
        </div>
        <div class="footer">
            <p>&copy; 2025 Tesseract Hub. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, code, code, code)

	default:
		subject = "Verification Code"
		htmlBody = fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .code { font-size: 32px; font-weight: bold; letter-spacing: 8px; text-align: center; padding: 20px; background-color: #f0f0f0; border-radius: 8px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <p>Your verification code is:</p>
        <div class="code">%s</div>
        <p>This code will expire in 10 minutes.</p>
    </div>
</body>
</html>
`, code)
	}

	return subject, htmlBody
}

// FormatWelcomeEmail formats the welcome email content
func FormatWelcomeEmail(firstName string) (string, string) {
	subject := "Welcome to Tesseract Hub!"
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #10B981; color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background-color: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .welcome-message { background-color: white; padding: 20px; border-radius: 8px; margin: 20px 0; border-left: 4px solid #10B981; }
        .cta-button { display: inline-block; background-color: #10B981; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .next-steps { background-color: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .next-steps li { margin: 10px 0; }
        .footer { text-align: center; margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎉 Welcome to Tesseract Hub!</h1>
        </div>
        <div class="content">
            <div class="welcome-message">
                <h2>Hello %s,</h2>
                <p>Congratulations! Your account has been successfully created and your store is ready to go.</p>
                <p>We're excited to have you on board and can't wait to see what you build with Tesseract Hub.</p>
            </div>

            <div class="next-steps">
                <h3>🚀 Next Steps:</h3>
                <ul>
                    <li><strong>Complete your store setup</strong> - Add your products, configure shipping, and customize your storefront</li>
                    <li><strong>Test your store</strong> - Make test orders in development mode</li>
                    <li><strong>Go live</strong> - When ready, connect your payment gateway and launch!</li>
                </ul>
            </div>

            <div style="text-align: center;">
                <a href="https://admin.tesserix.app/dashboard" class="cta-button">Go to Dashboard</a>
            </div>

            <p style="margin-top: 30px;">Need help? Check out our <a href="https://docs.tesserix.app">documentation</a> or reach out to our support team.</p>
        </div>
        <div class="footer">
            <p>&copy; 2025 Tesseract Hub. All rights reserved.</p>
            <p>You're receiving this email because you created an account at Tesseract Hub.</p>
        </div>
    </div>
</body>
</html>
`, firstName)
	return subject, htmlBody
}

// FormatVerificationLinkEmail formats the verification link email content
func FormatVerificationLinkEmail(verificationLink, businessName, email string) (string, string) {
	// Try new template first
	if s, h, err := templates.RenderVerificationLinkDefault(email, verificationLink, businessName); err == nil {
		return s, h
	}
	// Fallback to legacy template
	subject := "Verify your email address - " + businessName
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <title>Verify your email</title>
    <!--[if mso]>
    <noscript>
        <xml>
            <o:OfficeDocumentSettings>
                <o:PixelsPerInch>96</o:PixelsPerInch>
            </o:OfficeDocumentSettings>
        </xml>
    </noscript>
    <![endif]-->
</head>
<body style="margin: 0; padding: 0; background-color: #f4f4f5; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;">
    <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="background-color: #f4f4f5;">
        <tr>
            <td style="padding: 40px 20px;">
                <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="max-width: 520px; margin: 0 auto;">
                    <!-- Logo/Brand -->
                    <tr>
                        <td style="text-align: center; padding-bottom: 32px;">
                            <span style="font-size: 24px; font-weight: 700; color: #18181b; letter-spacing: -0.5px;">%s</span>
                        </td>
                    </tr>

                    <!-- Main Card -->
                    <tr>
                        <td>
                            <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="background-color: #ffffff; border-radius: 12px; box-shadow: 0 1px 3px rgba(0,0,0,0.08);">
                                <!-- Card Content -->
                                <tr>
                                    <td style="padding: 48px 40px;">
                                        <!-- Icon -->
                                        <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%">
                                            <tr>
                                                <td style="text-align: center; padding-bottom: 24px;">
                                                    <div style="display: inline-block; width: 56px; height: 56px; background-color: #f0fdf4; border-radius: 50%%; line-height: 56px; text-align: center;">
                                                        <span style="font-size: 28px;">✉️</span>
                                                    </div>
                                                </td>
                                            </tr>
                                        </table>

                                        <!-- Heading -->
                                        <h1 style="margin: 0 0 16px 0; font-size: 22px; font-weight: 600; color: #18181b; text-align: center; line-height: 1.3;">
                                            Verify your email address
                                        </h1>

                                        <!-- Description -->
                                        <p style="margin: 0 0 32px 0; font-size: 15px; color: #52525b; text-align: center; line-height: 1.6;">
                                            Click the button below to confirm your email and complete your account setup.
                                        </p>

                                        <!-- CTA Button -->
                                        <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%">
                                            <tr>
                                                <td style="text-align: center; padding-bottom: 32px;">
                                                    <a href="%s" style="display: inline-block; background-color: #18181b; color: #ffffff; font-size: 15px; font-weight: 600; text-decoration: none; padding: 14px 32px; border-radius: 8px; transition: background-color 0.2s;">
                                                        Verify Email Address
                                                    </a>
                                                </td>
                                            </tr>
                                        </table>

                                        <!-- Divider -->
                                        <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%">
                                            <tr>
                                                <td style="padding: 0 0 24px 0;">
                                                    <div style="height: 1px; background-color: #e4e4e7;"></div>
                                                </td>
                                            </tr>
                                        </table>

                                        <!-- Alternative Link -->
                                        <p style="margin: 0 0 12px 0; font-size: 13px; color: #71717a; text-align: center;">
                                            Or copy this link into your browser:
                                        </p>
                                        <p style="margin: 0; font-size: 12px; color: #3b82f6; text-align: center; word-break: break-all; background-color: #f9fafb; padding: 12px 16px; border-radius: 6px; border: 1px solid #e4e4e7;">
                                            %s
                                        </p>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="padding-top: 32px; text-align: center;">
                            <p style="margin: 0 0 8px 0; font-size: 13px; color: #a1a1aa;">
                                This link expires in 24 hours.
                            </p>
                            <p style="margin: 0 0 16px 0; font-size: 13px; color: #a1a1aa;">
                                If you didn't request this email, you can safely ignore it.
                            </p>
                            <p style="margin: 0; font-size: 12px; color: #d4d4d8;">
                                Sent to %s
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>
`, businessName, verificationLink, verificationLink, email)
	return subject, htmlBody
}

// FormatAccountCreatedEmail formats the account created email content
func FormatAccountCreatedEmail(firstName, businessName, subdomain string) (string, string) {
	subject := "Your Tesseract Hub Account is Ready!"
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #6366F1; color: white; padding: 30px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background-color: #f9fafb; padding: 30px; border-radius: 0 0 8px 8px; }
        .info-box { background-color: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .info-row { padding: 10px 0; border-bottom: 1px solid #e5e7eb; }
        .info-label { font-weight: bold; color: #6b7280; display: block; margin-bottom: 5px; }
        .info-value { color: #111827; }
        .highlight { background-color: #EEF2FF; padding: 15px; border-radius: 6px; margin: 15px 0; border-left: 4px solid #6366F1; }
        .cta-button { display: inline-block; background-color: #6366F1; color: white; padding: 12px 30px; text-decoration: none; border-radius: 6px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>✓ Account Created Successfully!</h1>
        </div>
        <div class="content">
            <p>Hello %s,</p>
            <p>Great news! Your Tesseract Hub account has been created successfully.</p>

            <div class="info-box">
                <h3>Your Account Details:</h3>
                <div class="info-row">
                    <span class="info-label">Business Name:</span>
                    <span class="info-value">%s</span>
                </div>
                <div class="info-row">
                    <span class="info-label">Store URL:</span>
                    <span class="info-value">https://%s.tesserix.app</span>
                </div>
                <div class="info-row">
                    <span class="info-label">Admin Portal:</span>
                    <span class="info-value">https://admin.tesserix.app</span>
                </div>
            </div>

            <div class="highlight">
                <strong>💡 Pro Tip:</strong> Your store is currently in <strong>development mode</strong>. This means you can test everything without going live. When you're ready to accept real orders, you can switch to live mode from your dashboard.
            </div>

            <div style="text-align: center;">
                <a href="https://admin.tesserix.app/dashboard" class="cta-button">Access Your Dashboard</a>
            </div>

            <p style="margin-top: 30px;">Questions? We're here to help! Contact us at <a href="mailto:support@tesserix.app">support@tesserix.app</a></p>
        </div>
        <div class="footer">
            <p>&copy; 2025 Tesseract Hub. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, firstName, businessName, subdomain)
	return subject, htmlBody
}

// FormatWelcomePackEmail formats a comprehensive welcome email with tenant URLs and login info
func FormatWelcomePackEmail(firstName, businessName, tenantSlug, adminURL, storefrontURL, email string) (string, string) {
	// Try new template first
	if s, h, err := templates.RenderWelcomePackDefault(email, firstName, businessName, adminURL, storefrontURL); err == nil {
		return s, h
	}
	// Fallback to legacy template
	subject := fmt.Sprintf("Welcome to %s - Your Store is Ready!", businessName)
	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome to %s</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f4f5; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;">
    <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="background-color: #f4f4f5;">
        <tr>
            <td style="padding: 40px 20px;">
                <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="max-width: 600px; margin: 0 auto;">
                    <!-- Logo -->
                    <tr>
                        <td style="text-align: center; padding-bottom: 32px;">
                            <span style="font-size: 28px; font-weight: 700; color: #18181b;">%s</span>
                        </td>
                    </tr>

                    <!-- Main Card -->
                    <tr>
                        <td>
                            <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="background-color: #ffffff; border-radius: 16px; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
                                <!-- Header -->
                                <tr>
                                    <td style="background: linear-gradient(135deg, #10b981 0%%, #059669 100%%); padding: 40px; border-radius: 16px 16px 0 0; text-align: center;">
                                        <span style="font-size: 48px;">🎉</span>
                                        <h1 style="margin: 16px 0 8px 0; font-size: 28px; font-weight: 700; color: #ffffff;">
                                            Welcome, %s!
                                        </h1>
                                        <p style="margin: 0; font-size: 16px; color: rgba(255,255,255,0.9);">
                                            Your store is ready to go live
                                        </p>
                                    </td>
                                </tr>

                                <!-- Content -->
                                <tr>
                                    <td style="padding: 40px;">
                                        <p style="margin: 0 0 24px 0; font-size: 16px; color: #3f3f46; line-height: 1.6;">
                                            Congratulations! Your <strong>%s</strong> store has been successfully created. Here's everything you need to get started:
                                        </p>

                                        <!-- URLs Box -->
                                        <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="background-color: #f9fafb; border-radius: 12px; margin-bottom: 24px;">
                                            <tr>
                                                <td style="padding: 24px;">
                                                    <h3 style="margin: 0 0 16px 0; font-size: 14px; font-weight: 600; color: #71717a; text-transform: uppercase; letter-spacing: 0.05em;">
                                                        Your Store URLs
                                                    </h3>

                                                    <!-- Admin URL -->
                                                    <div style="margin-bottom: 16px;">
                                                        <p style="margin: 0 0 4px 0; font-size: 12px; color: #71717a; font-weight: 500;">Admin Dashboard</p>
                                                        <a href="%s" style="display: block; font-size: 14px; color: #2563eb; text-decoration: none; word-break: break-all;">%s</a>
                                                    </div>

                                                    <!-- Storefront URL -->
                                                    <div>
                                                        <p style="margin: 0 0 4px 0; font-size: 12px; color: #71717a; font-weight: 500;">Your Storefront</p>
                                                        <a href="%s" style="display: block; font-size: 14px; color: #2563eb; text-decoration: none; word-break: break-all;">%s</a>
                                                    </div>
                                                </td>
                                            </tr>
                                        </table>

                                        <!-- Login Info -->
                                        <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%" style="background-color: #eff6ff; border-radius: 12px; border-left: 4px solid #2563eb; margin-bottom: 24px;">
                                            <tr>
                                                <td style="padding: 20px;">
                                                    <p style="margin: 0 0 8px 0; font-size: 14px; font-weight: 600; color: #1e40af;">
                                                        🔐 Login Credentials
                                                    </p>
                                                    <p style="margin: 0; font-size: 14px; color: #3b82f6;">
                                                        Email: <strong>%s</strong><br>
                                                        Use the password you created during signup.
                                                    </p>
                                                </td>
                                            </tr>
                                        </table>

                                        <!-- CTA Button -->
                                        <table role="presentation" cellspacing="0" cellpadding="0" border="0" width="100%%">
                                            <tr>
                                                <td style="text-align: center; padding: 8px 0 24px 0;">
                                                    <a href="%s" style="display: inline-block; background-color: #18181b; color: #ffffff; font-size: 16px; font-weight: 600; text-decoration: none; padding: 16px 32px; border-radius: 8px;">
                                                        Go to Dashboard →
                                                    </a>
                                                </td>
                                            </tr>
                                        </table>

                                        <!-- Next Steps -->
                                        <h3 style="margin: 0 0 16px 0; font-size: 16px; font-weight: 600; color: #18181b;">
                                            🚀 Next Steps
                                        </h3>
                                        <ol style="margin: 0 0 24px 0; padding-left: 20px; color: #52525b; font-size: 14px; line-height: 1.8;">
                                            <li>Add your first products</li>
                                            <li>Configure payment methods</li>
                                            <li>Set up shipping options</li>
                                            <li>Customize your storefront theme</li>
                                            <li>Go live and start selling!</li>
                                        </ol>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>

                    <!-- Footer -->
                    <tr>
                        <td style="padding-top: 32px; text-align: center;">
                            <p style="margin: 0 0 8px 0; font-size: 13px; color: #a1a1aa;">
                                Need help? Contact us at <a href="mailto:support@tesserix.app" style="color: #2563eb; text-decoration: none;">support@tesserix.app</a>
                            </p>
                            <p style="margin: 0; font-size: 12px; color: #d4d4d8;">
                                © 2026 Tesseract Hub. All rights reserved.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>
`, businessName, businessName, firstName, businessName, adminURL, adminURL, storefrontURL, storefrontURL, email, adminURL)
	return subject, htmlBody
}
