package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Tesseract-Nexus/go-shared/secrets")

// Config holds all configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	App      AppConfig
	NATS     NATSConfig
	AWS      AWSConfig
	Email    EmailConfig
	SMS      SMSConfig
	Push     PushConfig
	Verify   VerifyConfig
}

// ServerConfig holds server settings
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DatabaseConfig holds database settings
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// AppConfig holds application settings
type AppConfig struct {
	Environment       string
	MaxRetryAttempts  int
	WorkerConcurrency int
	// Admin email for alerts and notifications (inventory, vendor applications, etc.)
	AdminEmail string
	// Support email for support-related notifications (tickets, etc.)
	SupportEmail string
}

// NATSConfig holds NATS settings
type NATSConfig struct {
	URL           string
	MaxReconnects int
	ReconnectWait time.Duration
}

// AWSConfig holds AWS credentials and settings (shared for SES and SNS)
type AWSConfig struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

// EmailConfig holds email provider settings
type EmailConfig struct {
	// AWS SES settings (primary)
	SESFrom     string
	SESFromName string

	// Postal HTTP API settings (fallback)
	PostalAPIURL   string // e.g., http://postal-web.email.svc.cluster.local:5000
	PostalAPIKey   string // Server API key from Postal admin
	PostalFrom     string
	PostalFromName string

	// Postal SMTP settings (legacy - use HTTP API instead)
	PostalHost     string
	PostalPort     int
	PostalUsername string
	PostalPassword string

	// Generic SMTP settings (legacy/fallback)
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// SendGrid settings (fallback)
	SendGridAPIKey string
	SendGridFrom   string

	// Mautic settings (newsletters + automated emails)
	MauticURL      string
	MauticUsername string
	MauticPassword string
	MauticFrom     string
	MauticFromName string

	// Provider priority: SES > Postal > SendGrid
	// EnableFailover enables automatic failover to next provider
	EnableFailover bool
}

// SMSConfig holds SMS provider settings
type SMSConfig struct {
	// AWS SNS settings (primary)
	SNSFrom string // Sender ID or phone number

	// Twilio settings (fallback)
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFrom       string

	// Enable failover from SNS to Twilio
	EnableFailover bool
}

// PushConfig holds push notification settings
type PushConfig struct {
	FCMProjectID   string
	FCMCredentials string
}

// VerifyConfig holds Twilio Verify settings for OTP and account verification
type VerifyConfig struct {
	// Twilio Verify Service SID (starts with VA)
	TwilioVerifyServiceSID string
	// Twilio Account SID (starts with AC)
	TwilioAccountSID string
	// API Key authentication
	TwilioAPIKeySID    string
	TwilioAPIKeySecret string
	// Test phone number for development (e.g., +17744896582)
	TestPhoneNumber string
	// OTP expiry in minutes
	OTPExpiryMinutes int
	// OTP length (4-10 digits)
	OTPLength int
	// DevtestEnabled restricts OTP to test phone number only when true
	DevtestEnabled bool
}

// Load loads configuration from environment
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnvInt("SERVER_PORT", 8090),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			DBName:   getEnv("DB_NAME", "tesseract_hub"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		App: AppConfig{
			Environment:       getEnv("ENVIRONMENT", "development"),
			MaxRetryAttempts:  getEnvInt("MAX_RETRY_ATTEMPTS", 3),
			WorkerConcurrency: getEnvInt("WORKER_CONCURRENCY", 10),
			AdminEmail:        getEnv("ADMIN_EMAIL", "admin@tesserix.app"),
			SupportEmail:      getEnv("SUPPORT_EMAIL", "support@tesserix.app"),
		},
		NATS: NATSConfig{
			URL:           getEnv("NATS_URL", "nats://nats.nats.svc.cluster.local:4222"),
			MaxReconnects: getEnvInt("NATS_MAX_RECONNECTS", -1), // -1 = unlimited reconnects for production resilience
			ReconnectWait: time.Duration(getEnvInt("NATS_RECONNECT_WAIT_SECONDS", 2)) * time.Second,
		},
		AWS: AWSConfig{
			Region:          getEnv("AWS_REGION", "ap-south-1"), // Mumbai, India
			AccessKeyID:     secrets.GetSecretOrEnv("AWS_ACCESS_KEY_ID_SECRET_NAME", "AWS_ACCESS_KEY_ID", ""),
			SecretAccessKey: secrets.GetSecretOrEnv("AWS_SECRET_ACCESS_KEY_SECRET_NAME", "AWS_SECRET_ACCESS_KEY", ""),
		},
		Email: EmailConfig{
			// AWS SES (primary)
			SESFrom:     getEnvWithFallback("AWS_SES_FROM", "POSTAL_FROM", ""), // Fallback to POSTAL_FROM if SES_FROM not set
			SESFromName: getEnv("AWS_SES_FROM_NAME", "Tesseract Hub"),
			// Postal HTTP API (fallback)
			PostalAPIURL:   getEnv("POSTAL_API_URL", ""),
			PostalAPIKey:   secrets.GetSecretOrEnv("POSTAL_API_KEY_SECRET_NAME", "POSTAL_API_KEY", ""),
			PostalFrom:     getEnv("POSTAL_FROM", ""),
			PostalFromName: getEnv("POSTAL_FROM_NAME", "Tesseract Hub"),
			// Postal SMTP (legacy - use HTTP API instead)
			PostalHost:     getEnv("POSTAL_HOST", ""),
			PostalPort:     getEnvInt("POSTAL_PORT", 25),
			PostalUsername: getEnv("POSTAL_USERNAME", ""),
			PostalPassword: secrets.GetSecretOrEnv("POSTAL_PASSWORD_SECRET_NAME", "POSTAL_PASSWORD", ""),
			// Legacy SMTP
			SMTPHost:     getEnv("SMTP_HOST", ""),
			SMTPPort:     getEnvInt("SMTP_PORT", 587),
			SMTPUsername: getEnv("SMTP_USERNAME", ""),
			SMTPPassword: secrets.GetSecretOrEnv("SMTP_PASSWORD_SECRET_NAME", "SMTP_PASSWORD", ""),
			SMTPFrom:     getEnv("SMTP_FROM", ""),
			// SendGrid (fallback)
			SendGridAPIKey: secrets.GetSecretOrEnv("SENDGRID_API_KEY_SECRET_NAME", "SENDGRID_API_KEY", ""),
			SendGridFrom:   getEnv("SENDGRID_FROM", ""),
			// Mautic (newsletters)
			MauticURL:      getEnv("MAUTIC_URL", ""),
			MauticUsername: getEnv("MAUTIC_USERNAME", ""),
			MauticPassword: getEnv("MAUTIC_PASSWORD", ""),
			MauticFrom:     getEnv("MAUTIC_FROM", ""),
			MauticFromName: getEnv("MAUTIC_FROM_NAME", "Tesseract Hub"),
			// Failover: SES > Postal > SendGrid
			EnableFailover: getEnvBool("EMAIL_FAILOVER_ENABLED", true),
		},
		SMS: SMSConfig{
			// AWS SNS (primary)
			SNSFrom: getEnv("AWS_SNS_FROM", ""),
			// Twilio (fallback)
			TwilioAccountSID: getEnv("TWILIO_ACCOUNT_SID", ""),
			TwilioAuthToken:  getEnv("TWILIO_AUTH_TOKEN", ""),
			TwilioFrom:       getEnv("TWILIO_FROM", ""),
			// Failover: SNS > Twilio
			EnableFailover: getEnvBool("SMS_FAILOVER_ENABLED", true),
		},
		Push: PushConfig{
			FCMProjectID:   getEnv("FCM_PROJECT_ID", ""),
			FCMCredentials: getEnv("FCM_CREDENTIALS_JSON", ""),
		},
		Verify: VerifyConfig{
			TwilioVerifyServiceSID: getEnv("TWILIO_VERIFY_SERVICE_SID", ""),
			TwilioAccountSID:       getEnv("TWILIO_ACCOUNT_SID", ""),
			TwilioAPIKeySID:        getEnv("TWILIO_API_KEY_SID", ""),
			TwilioAPIKeySecret:     getEnv("TWILIO_API_KEY_SECRET", ""),
			TestPhoneNumber:        getEnv("TWILIO_TEST_PHONE", ""),
			OTPExpiryMinutes:       getEnvInt("TWILIO_OTP_EXPIRY_MINUTES", 10),
			OTPLength:              getEnvInt("TWILIO_OTP_LENGTH", 6),
			DevtestEnabled:         getEnvBool("TWILIO_DEVTEST_ENABLED", false),
		},
	}

	return cfg, nil
}

// DSN returns the database connection string
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvWithFallback(primaryKey, fallbackKey, defaultValue string) string {
	if value := os.Getenv(primaryKey); value != "" {
		return value
	}
	if value := os.Getenv(fallbackKey); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}
