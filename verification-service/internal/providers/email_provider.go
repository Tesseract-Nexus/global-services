package providers

import (
	"log"
	"os"
)

// EmailProvider defines the interface for email providers
type EmailProvider interface {
	SendVerificationEmail(recipient, code, purpose string) error
	SendEmail(recipient, subject, htmlBody string) error
	SendTemplatedEmail(recipient, templateName string, variables map[string]interface{}) error
	GetName() string
}

// EmailMessage represents an email message
type EmailMessage struct {
	To      string
	Subject string
	Body    string
	HTML    string
}

// EmailProviderFactory creates an email provider.
// All emails are routed through notification-service which handles Postal/SendGrid delivery.
func EmailProviderFactory(providerName, apiKey, fromEmail, fromName string) (EmailProvider, error) {
	if providerName != "notification-service" && providerName != "" {
		log.Printf("Warning: Provider '%s' is deprecated. Using notification-service instead.", providerName)
	}

	baseURL := getEnvOrDefault("NOTIFICATION_SERVICE_URL", "http://notification-service.marketplace.svc.cluster.local:8090")
	return NewNotificationServiceProvider(baseURL, apiKey, fromEmail, fromName), nil
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
