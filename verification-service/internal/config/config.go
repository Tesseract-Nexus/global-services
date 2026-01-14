package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/tesseract-hub/go-shared/secrets")

// Config holds all configuration for the verification service
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Email     EmailConfig
	Security  SecurityConfig
	RateLimit RateLimitConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
	Mode string // debug, release
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// EmailConfig holds email provider configuration
type EmailConfig struct {
	Provider               string // notification-service (recommended), resend, sendgrid
	APIKey                 string // Not required when using notification-service
	FromEmail              string
	FromName               string
	NotificationServiceURL string // URL for notification-service
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	APIKey           string
	EncryptionKey    string // 32 bytes for AES-256
	OTPLength        int
	OTPExpiryMinutes int
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	MaxAttempts     int
	MaxCodesPerHour int
	CooldownMinutes int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8088"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			DBName:   getEnv("DB_NAME", "tesseract_hub"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Email: EmailConfig{
			Provider:               getEnv("EMAIL_PROVIDER", "notification-service"),
			APIKey:                 getEnv("EMAIL_API_KEY", ""), // Not required for notification-service
			FromEmail:              getEnv("EMAIL_FROM", "no-reply@tesserix.app"),
			FromName:               getEnv("EMAIL_FROM_NAME", "Tesseract Hub"),
			NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://notification-service.devtest.svc.cluster.local:8090"),
		},
		Security: SecurityConfig{
			APIKey:           secrets.GetAPIKey(),
			EncryptionKey:    secrets.GetEncryptionKey(),
			OTPLength:        getEnvAsInt("OTP_LENGTH", 6),
			OTPExpiryMinutes: getEnvAsInt("OTP_EXPIRY_MINUTES", 10),
		},
		RateLimit: RateLimitConfig{
			MaxAttempts:     getEnvAsInt("MAX_VERIFICATION_ATTEMPTS", 3),
			MaxCodesPerHour: getEnvAsInt("MAX_CODES_PER_HOUR", 5),
			CooldownMinutes: getEnvAsInt("COOLDOWN_MINUTES", 60),
		},
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// EMAIL_API_KEY is no longer required - emails go through notification-service
	// which handles Postal/SendGrid delivery

	if c.Security.APIKey == "" {
		return fmt.Errorf("API_KEY is required for inter-service authentication")
	}

	if c.Security.EncryptionKey == "" {
		return fmt.Errorf("ENCRYPTION_KEY is required")
	}

	if len(c.Security.EncryptionKey) != 32 {
		return fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes for AES-256")
	}

	return nil
}

// GetDSN returns the database connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.DBName,
		c.Database.SSLMode,
	)
}

// GetOTPExpiry returns the OTP expiry duration
func (c *Config) GetOTPExpiry() time.Duration {
	return time.Duration(c.Security.OTPExpiryMinutes) * time.Minute
}

// GetCooldownDuration returns the cooldown duration
func (c *Config) GetCooldownDuration() time.Duration {
	return time.Duration(c.RateLimit.CooldownMinutes) * time.Minute
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
