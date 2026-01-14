package services

import (
	"bytes"
	"fmt"
	"net/smtp"
	"os"
	"strconv"

	"github.com/Tesseract-Nexus/go-shared/security"
)

// EmailService handles email sending operations
type EmailService struct {
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string
	fromEmail    string
	fromName     string
	baseURL      string
}

// NewEmailService creates a new email service
func NewEmailService() *EmailService {
	port, _ := strconv.Atoi(getEnvWithDefault("SMTP_PORT", "587"))

	return &EmailService{
		smtpHost:     getEnvWithDefault("SMTP_HOST", "smtp.gmail.com"),
		smtpPort:     port,
		smtpUsername: getEnvWithDefault("SMTP_USERNAME", ""),
		smtpPassword: getEnvWithDefault("SMTP_PASSWORD", ""),
		fromEmail:    getEnvWithDefault("FROM_EMAIL", "noreply@tesseract-hub.com"),
		fromName:     getEnvWithDefault("FROM_NAME", "Tesseract Hub"),
		baseURL:      getEnvWithDefault("BASE_URL", "http://localhost:3000"),
	}
}

// getEnvWithDefault gets environment variable with fallback
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// EmailTemplate represents an email template
type EmailTemplate struct {
	Subject string
	HTML    string
	Text    string
}

// SendVerificationEmail sends an email verification email
func (es *EmailService) SendVerificationEmail(to, name, token string) error {
	verificationURL := fmt.Sprintf("%s/auth/verify-email?token=%s", es.baseURL, token)

	template := EmailTemplate{
		Subject: "Verify Your Email Address",
		HTML: fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verify Your Email</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #f8f9fa; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: white; padding: 30px; border-radius: 0 0 8px 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .button { display: inline-block; padding: 15px 30px; background: #007bff; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to Tesseract Hub!</h1>
        </div>
        <div class="content">
            <h2>Hi %s,</h2>
            <p>Thank you for signing up! Please verify your email address to complete your registration.</p>
            <p>Click the button below to verify your email:</p>
            <a href="%s" class="button">Verify Email Address</a>
            <p>Or copy and paste this link into your browser:</p>
            <p><a href="%s">%s</a></p>
            <p><strong>Note:</strong> This verification link will expire in 24 hours.</p>
            <p>If you didn't create an account, you can safely ignore this email.</p>
        </div>
        <div class="footer">
            <p>Best regards,<br>The Tesseract Hub Team</p>
        </div>
    </div>
</body>
</html>`, name, verificationURL, verificationURL, verificationURL),
		Text: fmt.Sprintf(`
Hi %s,

Thank you for signing up for Tesseract Hub! Please verify your email address to complete your registration.

Verification Link: %s

Note: This verification link will expire in 24 hours.

If you didn't create an account, you can safely ignore this email.

Best regards,
The Tesseract Hub Team
`, name, verificationURL),
	}

	return es.sendEmail(to, template)
}

// SendPasswordResetEmail sends a password reset email
func (es *EmailService) SendPasswordResetEmail(to, name, token string) error {
	resetURL := fmt.Sprintf("%s/auth/reset-password?token=%s", es.baseURL, token)

	template := EmailTemplate{
		Subject: "Reset Your Password",
		HTML: fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Reset Your Password</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #f8f9fa; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: white; padding: 30px; border-radius: 0 0 8px 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .button { display: inline-block; padding: 15px 30px; background: #dc3545; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; color: #666; font-size: 14px; }
        .warning { background: #fff3cd; border: 1px solid #ffeaa7; padding: 15px; border-radius: 5px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Reset Request</h1>
        </div>
        <div class="content">
            <h2>Hi %s,</h2>
            <p>We received a request to reset your password for your Tesseract Hub account.</p>
            <p>Click the button below to reset your password:</p>
            <a href="%s" class="button">Reset Password</a>
            <p>Or copy and paste this link into your browser:</p>
            <p><a href="%s">%s</a></p>
            <div class="warning">
                <p><strong>Important Security Information:</strong></p>
                <ul>
                    <li>This password reset link will expire in 1 hour</li>
                    <li>If you didn't request this reset, please ignore this email</li>
                    <li>Your password won't change until you access the link above and create a new one</li>
                </ul>
            </div>
            <p>For security reasons, if you don't use this link, your password will remain unchanged.</p>
        </div>
        <div class="footer">
            <p>Best regards,<br>The Tesseract Hub Team</p>
        </div>
    </div>
</body>
</html>`, name, resetURL, resetURL, resetURL),
		Text: fmt.Sprintf(`
Hi %s,

We received a request to reset your password for your Tesseract Hub account.

Reset Password Link: %s

Important Security Information:
- This password reset link will expire in 1 hour
- If you didn't request this reset, please ignore this email
- Your password won't change until you access the link above and create a new one

For security reasons, if you don't use this link, your password will remain unchanged.

Best regards,
The Tesseract Hub Team
`, name, resetURL),
	}

	return es.sendEmail(to, template)
}

// SendWelcomeEmail sends a welcome email after successful email verification
func (es *EmailService) SendWelcomeEmail(to, name string) error {
	template := EmailTemplate{
		Subject: "Welcome to Tesseract Hub!",
		HTML: fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome to Tesseract Hub</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #28a745; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: white; padding: 30px; border-radius: 0 0 8px 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .button { display: inline-block; padding: 15px 30px; background: #007bff; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; color: #666; font-size: 14px; }
        .feature { background: #f8f9fa; padding: 15px; margin: 10px 0; border-radius: 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎉 Welcome to Tesseract Hub!</h1>
        </div>
        <div class="content">
            <h2>Hi %s,</h2>
            <p>Congratulations! Your email has been verified and your account is now active.</p>
            <p>You're now part of the Tesseract Hub community and can access all our features:</p>
            
            <div class="feature">
                <h3>🛍️ E-commerce Platform</h3>
                <p>Create and manage your online store with ease</p>
            </div>
            
            <div class="feature">
                <h3>📊 Analytics Dashboard</h3>
                <p>Track your performance with detailed analytics</p>
            </div>
            
            <div class="feature">
                <h3>🔧 Customizable Tools</h3>
                <p>Access our suite of business management tools</p>
            </div>
            
            <p>Ready to get started?</p>
            <a href="%s/dashboard" class="button">Go to Dashboard</a>
            
            <p>If you have any questions, feel free to reach out to our support team.</p>
        </div>
        <div class="footer">
            <p>Best regards,<br>The Tesseract Hub Team</p>
        </div>
    </div>
</body>
</html>`, name, es.baseURL),
		Text: fmt.Sprintf(`
Hi %s,

Congratulations! Your email has been verified and your account is now active.

You're now part of the Tesseract Hub community and can access all our features:

- E-commerce Platform: Create and manage your online store with ease
- Analytics Dashboard: Track your performance with detailed analytics  
- Customizable Tools: Access our suite of business management tools

Ready to get started? Visit: %s/dashboard

If you have any questions, feel free to reach out to our support team.

Best regards,
The Tesseract Hub Team
`, name, es.baseURL),
	}

	return es.sendEmail(to, template)
}

// sendEmail sends an email using SMTP
func (es *EmailService) sendEmail(to string, emailTemplate EmailTemplate) error {
	// Skip sending if SMTP not configured (for development)
	if es.smtpUsername == "" || es.smtpPassword == "" {
		// SECURITY: Mask email in logs
		fmt.Printf("Email would be sent to %s: %s\n", security.MaskEmail(to), emailTemplate.Subject)
		fmt.Printf("Content: [REDACTED - contains sensitive data]\n")
		return nil
	}

	// Create SMTP authentication
	auth := smtp.PlainAuth("", es.smtpUsername, es.smtpPassword, es.smtpHost)

	// Prepare email headers and body
	from := fmt.Sprintf("%s <%s>", es.fromName, es.fromEmail)

	// Create multipart message
	boundary := "boundary-tesseract-hub"

	var emailBody bytes.Buffer
	emailBody.WriteString(fmt.Sprintf("From: %s\n", from))
	emailBody.WriteString(fmt.Sprintf("To: %s\n", to))
	emailBody.WriteString(fmt.Sprintf("Subject: %s\n", emailTemplate.Subject))
	emailBody.WriteString("MIME-Version: 1.0\n")
	emailBody.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\n\n", boundary))

	// Text part
	emailBody.WriteString(fmt.Sprintf("--%s\n", boundary))
	emailBody.WriteString("Content-Type: text/plain; charset=\"utf-8\"\n\n")
	emailBody.WriteString(emailTemplate.Text)
	emailBody.WriteString("\n\n")

	// HTML part
	emailBody.WriteString(fmt.Sprintf("--%s\n", boundary))
	emailBody.WriteString("Content-Type: text/html; charset=\"utf-8\"\n\n")
	emailBody.WriteString(emailTemplate.HTML)
	emailBody.WriteString("\n\n")

	emailBody.WriteString(fmt.Sprintf("--%s--", boundary))

	// Send email
	addr := fmt.Sprintf("%s:%d", es.smtpHost, es.smtpPort)
	err := smtp.SendMail(addr, auth, es.fromEmail, []string{to}, emailBody.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// ValidateEmailConfig validates the email service configuration
func (es *EmailService) ValidateEmailConfig() error {
	if es.smtpHost == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if es.smtpPort == 0 {
		return fmt.Errorf("SMTP port is required")
	}
	if es.fromEmail == "" {
		return fmt.Errorf("from email is required")
	}

	// SMTP credentials are optional for development
	if es.smtpUsername == "" || es.smtpPassword == "" {
		fmt.Println("Warning: SMTP credentials not configured. Emails will be logged instead of sent.")
	}

	return nil
}

// GetEmailServiceStatus returns the current status of the email service
func (es *EmailService) GetEmailServiceStatus() map[string]interface{} {
	status := map[string]interface{}{
		"smtp_host":        es.smtpHost,
		"smtp_port":        es.smtpPort,
		"from_email":       es.fromEmail,
		"from_name":        es.fromName,
		"base_url":         es.baseURL,
		"configured":       es.smtpUsername != "" && es.smtpPassword != "",
		"development_mode": es.smtpUsername == "" || es.smtpPassword == "",
	}

	return status
}
