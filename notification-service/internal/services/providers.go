package services

import (
	"context"
)

// Provider represents a notification provider interface
type Provider interface {
	Send(ctx context.Context, message *Message) (*SendResult, error)
	GetName() string
	SupportsChannel() string
}

// Message represents a message to be sent
type Message struct {
	To          string
	Subject     string
	Body        string
	BodyHTML    string
	From        string
	FromName    string
	ReplyTo     string
	CC          []string
	BCC         []string
	Attachments []Attachment
	Headers     map[string]string
	Metadata    map[string]interface{}
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string
	Content     []byte
	ContentType string
}

// SendResult represents the result of a send operation
type SendResult struct {
	ProviderID      string
	ProviderName    string
	Success         bool
	Error           error
	ProviderData    map[string]interface{}
}

// ProviderConfig represents provider configuration
type ProviderConfig struct {
	// AWS Credentials (shared for SES and SNS)
	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string

	// AWS SES (primary email)
	SESFrom     string
	SESFromName string

	// AWS SNS (primary SMS)
	SNSFrom string // Sender ID or phone number for SMS

	// Postal HTTP API (fallback email)
	PostalAPIURL   string // e.g., http://postal-web.email.svc.cluster.local:5000
	PostalAPIKey   string // Server API key from Postal admin
	PostalFrom     string
	PostalFromName string

	// Postal SMTP (legacy - use HTTP API instead)
	PostalHost     string
	PostalPort     int
	PostalUsername string
	PostalPassword string

	// Generic SMTP (legacy)
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// SendGrid (fallback email)
	SendGridAPIKey string
	SendGridFrom   string

	// Mautic (newsletters + automated emails)
	MauticURL      string
	MauticUsername string
	MauticPassword string
	MauticFrom     string
	MauticFromName string

	// SMS providers (fallback)
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFrom       string

	// Push providers
	FCMServerKey   string
	FCMProjectID   string
	FCMCredentials string // JSON credentials for GCP

	// GCP
	GCPProjectID   string
	GCPPubSubTopic string
}
