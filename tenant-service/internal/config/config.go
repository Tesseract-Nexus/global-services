package config

import (
	"os"
	"strconv"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

// Config holds all configuration for the service
type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	Redis        RedisConfig
	App          AppConfig
	Email        EmailConfig
	SMS          SMSConfig
	Payment      PaymentConfig
	Integration  IntegrationConfig
	Draft        DraftConfig
	Verification VerificationConfig
	URL          URLConfig
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// DraftConfig holds draft persistence configuration
type DraftConfig struct {
	ExpiryHours      int // Draft expiry in hours (default: 48)
	ReminderInterval int // Reminder interval in hours (default: 4)
	MaxReminders     int // Maximum reminders to send (default: 12)
	CleanupInterval  int // Cleanup job interval in minutes (default: 15)
}

// VerificationConfig holds email verification configuration
type VerificationConfig struct {
	Method           string // "otp" or "link" (default: "link")
	TokenExpiryHours int    // Verification token expiry in hours (default: 24)
	OnboardingAppURL string // Base URL for onboarding app (for verification links)
	BaseDomain       string // Base domain for tenant URLs (e.g., "tesserix.app")
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host string
	Port string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// AppConfig holds application configuration
type AppConfig struct {
	Environment string
	LogLevel    string
	JWTSecret   string
}

// EmailConfig holds email configuration
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	FromEmail    string
	FromName     string
}

// SMSConfig holds SMS configuration
type SMSConfig struct {
	Provider    string // twilio, nexmo, etc.
	AccountSID  string
	AuthToken   string
	PhoneNumber string
}

// PaymentConfig holds payment provider configuration
type PaymentConfig struct {
	StripeSecretKey      string
	StripePublishableKey string
	PayPalClientID       string
	PayPalClientSecret   string
	RazorPayKeyID        string
	RazorPayKeySecret    string
}

// IntegrationConfig holds configuration for external service integrations
type IntegrationConfig struct {
	SettingsServiceURL      string
	NotificationServiceURL  string
	CustomDomainServiceURL  string
	TenantRouterServiceURL  string
}

// URLConfig holds URL generation configuration for tenant subdomains
type URLConfig struct {
	// BaseDomain is the base domain for all tenant subdomains (e.g., "tesserix.app")
	// URL patterns:
	// - Admin: {slug}-admin.{baseDomain} (e.g., mystore-admin.tesserix.app)
	// - Store: {slug}-store.{baseDomain} (e.g., mystore-store.tesserix.app)
	BaseDomain string
}

// New creates a new configuration instance
func New() *Config {
	return &Config{
		Server: ServerConfig{
			Host: getEnvWithDefault("SERVER_HOST", "0.0.0.0"),
			Port: getEnvWithDefault("SERVER_PORT", "8086"),
		},
		Database: DatabaseConfig{
			Host:     getEnvWithDefault("DB_HOST", "localhost"),
			Port:     getEnvWithDefault("DB_PORT", "5432"),
			User:     getEnvWithDefault("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(), // Fetch from GCP Secret Manager if enabled
			Name:     getEnvWithDefault("DB_NAME", "onboarding_db"),
			SSLMode:  getEnvWithDefault("DB_SSLMODE", "disable"),
		},
		App: AppConfig{
			Environment: getEnvWithDefault("APP_ENV", "development"),
			LogLevel:    getEnvWithDefault("LOG_LEVEL", "info"),
			JWTSecret:   secrets.GetJWTSecret(), // Fetch from GCP Secret Manager if enabled
		},
		Email: EmailConfig{
			SMTPHost:     getEnvWithDefault("SMTP_HOST", "smtp.gmail.com"),
			SMTPPort:     getEnvWithDefault("SMTP_PORT", "587"),
			SMTPUser:     getEnvWithDefault("SMTP_USER", ""),
			SMTPPassword: getEnvWithDefault("SMTP_PASSWORD", ""),
			FromEmail:    getEnvWithDefault("FROM_EMAIL", "noreply@tesseract-hub.com"),
			FromName:     getEnvWithDefault("FROM_NAME", "Tesseract Hub"),
		},
		SMS: SMSConfig{
			Provider:    getEnvWithDefault("SMS_PROVIDER", "twilio"),
			AccountSID:  getEnvWithDefault("TWILIO_ACCOUNT_SID", ""),
			AuthToken:   getEnvWithDefault("TWILIO_AUTH_TOKEN", ""),
			PhoneNumber: getEnvWithDefault("TWILIO_PHONE_NUMBER", ""),
		},
		Payment: PaymentConfig{
			StripeSecretKey:      getEnvWithDefault("STRIPE_SECRET_KEY", ""),
			StripePublishableKey: getEnvWithDefault("STRIPE_PUBLISHABLE_KEY", ""),
			PayPalClientID:       getEnvWithDefault("PAYPAL_CLIENT_ID", ""),
			PayPalClientSecret:   getEnvWithDefault("PAYPAL_CLIENT_SECRET", ""),
			RazorPayKeyID:        getEnvWithDefault("RAZORPAY_KEY_ID", ""),
			RazorPayKeySecret:    getEnvWithDefault("RAZORPAY_KEY_SECRET", ""),
		},
		Integration: IntegrationConfig{
			SettingsServiceURL:      getEnvWithDefault("SETTINGS_SERVICE_URL", "http://localhost:8104"),
			NotificationServiceURL:  getEnvWithDefault("NOTIFICATION_SERVICE_URL", "http://localhost:8087"),
			CustomDomainServiceURL:  getEnvWithDefault("CUSTOM_DOMAIN_SERVICE_URL", "http://custom-domain-service.marketplace.svc.cluster.local:8093"),
			TenantRouterServiceURL:  getEnvWithDefault("TENANT_ROUTER_SERVICE_URL", "http://tenant-router-service.marketplace.svc.cluster.local:8089"),
		},
		Redis: RedisConfig{
			Host:     getEnvWithDefault("REDIS_HOST", "localhost"),
			Port:     getEnvWithDefault("REDIS_PORT", "6379"),
			Password: getEnvWithDefault("REDIS_PASSWORD", ""),
			DB:       getEnvAsIntWithDefault("REDIS_DB", 0),
		},
		Draft: DraftConfig{
			ExpiryHours:      getEnvAsIntWithDefault("DRAFT_EXPIRY_HOURS", 168), // 7 days
			ReminderInterval: getEnvAsIntWithDefault("DRAFT_REMINDER_INTERVAL_HOURS", 24),
			MaxReminders:     getEnvAsIntWithDefault("DRAFT_MAX_REMINDERS", 7),
			CleanupInterval:  getEnvAsIntWithDefault("DRAFT_CLEANUP_INTERVAL_MINS", 60),
		},
		Verification: VerificationConfig{
			Method:           getEnvWithDefault("VERIFICATION_METHOD", "link"),
			TokenExpiryHours: getEnvAsIntWithDefault("VERIFICATION_TOKEN_EXPIRY_HOURS", 24),
			OnboardingAppURL: getEnvWithDefault("ONBOARDING_APP_URL", "http://localhost:3000"),
			BaseDomain:       getEnvWithDefault("BASE_DOMAIN", "tesserix.app"),
		},
		URL: URLConfig{
			// Base domain for subdomain-based tenant URLs
			// Pattern: {slug}-admin.{baseDomain} for admin, {slug}-store.{baseDomain} for storefront
			BaseDomain: getEnvWithDefault("BASE_DOMAIN", "tesserix.app"),
		},
	}
}

// getEnvWithDefault gets environment variable with a default fallback
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsIntWithDefault gets environment variable as integer with default fallback
func getEnvAsIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsBoolWithDefault gets environment variable as boolean with default fallback
func getEnvAsBoolWithDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
