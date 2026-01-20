package config

import (
	"log"
	"os"
	"time"

	"github.com/spf13/viper"
	"github.com/Tesseract-Nexus/go-shared/secrets"
)

type Config struct {
	Server                 ServerConfig   `mapstructure:"server"`
	Database               DatabaseConfig `mapstructure:"database"`
	Redis                  RedisConfig    `mapstructure:"redis"`
	JWT                    JWTConfig      `mapstructure:"jwt"`
	Azure                  AzureConfig    `mapstructure:"azure"`
	Security               SecurityConfig `mapstructure:"security"`
	NotificationServiceURL string         `mapstructure:"notification_service_url"`
	TenantServiceURL       string         `mapstructure:"tenant_service_url"`
}

// SecurityConfig holds security-related settings for account lockout
type SecurityConfig struct {
	// MaxLoginAttempts is the number of failed attempts before lockout (per tier)
	MaxLoginAttempts int `mapstructure:"max_login_attempts"`
	// Tier1LockoutMinutes is the lockout duration for tier 1 (5 failed attempts)
	Tier1LockoutMinutes int `mapstructure:"tier1_lockout_minutes"`
	// Tier2LockoutMinutes is the lockout duration for tier 2 (10 failed attempts)
	Tier2LockoutMinutes int `mapstructure:"tier2_lockout_minutes"`
	// Tier3LockoutMinutes is the lockout duration for tier 3 (15 failed attempts)
	Tier3LockoutMinutes int `mapstructure:"tier3_lockout_minutes"`
	// PermanentLockoutThreshold is the total attempts before permanent lockout
	PermanentLockoutThreshold int `mapstructure:"permanent_lockout_threshold"`
	// LockoutResetHours is how long until failed attempts are reset (if no lockouts)
	LockoutResetHours int `mapstructure:"lockout_reset_hours"`
}

// GetTier1Duration returns the tier 1 lockout duration
func (c *SecurityConfig) GetTier1Duration() time.Duration {
	return time.Duration(c.Tier1LockoutMinutes) * time.Minute
}

// GetTier2Duration returns the tier 2 lockout duration
func (c *SecurityConfig) GetTier2Duration() time.Duration {
	return time.Duration(c.Tier2LockoutMinutes) * time.Minute
}

// GetTier3Duration returns the tier 3 lockout duration
func (c *SecurityConfig) GetTier3Duration() time.Duration {
	return time.Duration(c.Tier3LockoutMinutes) * time.Minute
}

// GetLockoutResetDuration returns the lockout reset duration
func (c *SecurityConfig) GetLockoutResetDuration() time.Duration {
	return time.Duration(c.LockoutResetHours) * time.Hour
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
	SSLMode  string `mapstructure:"sslmode"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret            string `mapstructure:"secret"`
	RefreshSecret     string `mapstructure:"refresh_secret"`
	AccessExpiryHours int    `mapstructure:"access_expiry_hours"`
	RefreshExpiryDays int    `mapstructure:"refresh_expiry_days"`
}

type AzureConfig struct {
	TenantID     string `mapstructure:"tenant_id"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

func LoadConfig() *Config {
	config := &Config{}

	// Set default values
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", "3080")
	viper.SetDefault("server.mode", "debug")

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", "5432")
	viper.SetDefault("database.user", "dev")
	viper.SetDefault("database.password", "devpass")
	viper.SetDefault("database.name", "auth")
	viper.SetDefault("database.sslmode", "disable")

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", "6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// SECURITY: No default secrets - must be configured via environment or GCP Secret Manager
	viper.SetDefault("jwt.secret", "")
	viper.SetDefault("jwt.refresh_secret", "")
	viper.SetDefault("jwt.access_expiry_hours", 8)
	viper.SetDefault("jwt.refresh_expiry_days", 30)

	// Security / Account Lockout defaults
	viper.SetDefault("security.max_login_attempts", 5)
	viper.SetDefault("security.tier1_lockout_minutes", 30)   // 30 minutes
	viper.SetDefault("security.tier2_lockout_minutes", 60)   // 1 hour
	viper.SetDefault("security.tier3_lockout_minutes", 120)  // 2 hours
	viper.SetDefault("security.permanent_lockout_threshold", 20) // 4 tiers x 5 attempts
	viper.SetDefault("security.lockout_reset_hours", 24)     // 24 hours

	// Read from environment variables
	viper.AutomaticEnv()

	// Override with environment variables if they exist
	if host := os.Getenv("SERVER_HOST"); host != "" {
		viper.Set("server.host", host)
	}
	if port := os.Getenv("PORT"); port != "" {
		viper.Set("server.port", port)
	}
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		viper.Set("server.mode", mode)
	}

	// Database environment variables
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		// Parse DATABASE_URL if provided (PostgreSQL URL format)
		// For now, we'll use individual env vars
	}
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		viper.Set("database.host", dbHost)
	}
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		viper.Set("database.port", dbPort)
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		viper.Set("database.user", dbUser)
	}
	// Get DB password from GCP Secret Manager (if enabled) or env var
	dbPassword := secrets.GetDBPassword()
	viper.Set("database.password", dbPassword)
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		viper.Set("database.name", dbName)
	}
	if dbSSLMode := os.Getenv("DB_SSLMODE"); dbSSLMode != "" {
		viper.Set("database.sslmode", dbSSLMode)
	}

	// Redis environment variables
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		// Parse REDIS_URL if provided
		// For now, we'll use individual env vars
	}
	if redisHost := os.Getenv("REDIS_HOST"); redisHost != "" {
		viper.Set("redis.host", redisHost)
	}
	if redisPort := os.Getenv("REDIS_PORT"); redisPort != "" {
		viper.Set("redis.port", redisPort)
	}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		viper.Set("redis.password", redisPassword)
	}

	// JWT environment variables - fetch from GCP Secret Manager if enabled
	jwtSecret := secrets.GetJWTSecret()
	viper.Set("jwt.secret", jwtSecret)
	if refreshSecret := os.Getenv("JWT_REFRESH_SECRET"); refreshSecret != "" {
		viper.Set("jwt.refresh_secret", refreshSecret)
	}

	// Azure environment variables
	if tenantID := os.Getenv("AZURE_AD_TENANT_ID"); tenantID != "" {
		viper.Set("azure.tenant_id", tenantID)
	}
	if clientID := os.Getenv("AZURE_AD_CLIENT_ID"); clientID != "" {
		viper.Set("azure.client_id", clientID)
	}
	if clientSecret := os.Getenv("AZURE_AD_CLIENT_SECRET"); clientSecret != "" {
		viper.Set("azure.client_secret", clientSecret)
	}

	// Notification service environment variables
	if notificationURL := os.Getenv("NOTIFICATION_SERVICE_URL"); notificationURL != "" {
		viper.Set("notification_service_url", notificationURL)
	}
	if tenantURL := os.Getenv("TENANT_SERVICE_URL"); tenantURL != "" {
		viper.Set("tenant_service_url", tenantURL)
	}

	// Security / Account Lockout environment variables
	if maxAttempts := os.Getenv("MAX_LOGIN_ATTEMPTS"); maxAttempts != "" {
		viper.Set("security.max_login_attempts", maxAttempts)
	}
	if tier1 := os.Getenv("TIER1_LOCKOUT_MINUTES"); tier1 != "" {
		viper.Set("security.tier1_lockout_minutes", tier1)
	}
	if tier2 := os.Getenv("TIER2_LOCKOUT_MINUTES"); tier2 != "" {
		viper.Set("security.tier2_lockout_minutes", tier2)
	}
	if tier3 := os.Getenv("TIER3_LOCKOUT_MINUTES"); tier3 != "" {
		viper.Set("security.tier3_lockout_minutes", tier3)
	}
	if threshold := os.Getenv("PERMANENT_LOCKOUT_THRESHOLD"); threshold != "" {
		viper.Set("security.permanent_lockout_threshold", threshold)
	}
	if resetHours := os.Getenv("LOCKOUT_RESET_HOURS"); resetHours != "" {
		viper.Set("security.lockout_reset_hours", resetHours)
	}

	// Unmarshal into config struct
	if err := viper.Unmarshal(config); err != nil {
		log.Fatalf("Unable to decode config: %v", err)
	}

	// SECURITY: Validate required secrets in production
	// In development mode (debug), allow empty secrets for local testing
	// In production mode (release), require all secrets to be properly configured
	if config.Server.Mode != "debug" {
		if config.JWT.Secret == "" {
			log.Fatal("SECURITY ERROR: JWT_SECRET is required in production. " +
				"Set JWT_SECRET environment variable or configure GCP Secret Manager.")
		}
		if config.JWT.RefreshSecret == "" {
			log.Fatal("SECURITY ERROR: JWT_REFRESH_SECRET is required in production. " +
				"Set JWT_REFRESH_SECRET environment variable.")
		}
		if config.Database.Password == "" {
			log.Fatal("SECURITY ERROR: DB_PASSWORD is required in production. " +
				"Set DB_PASSWORD environment variable or configure GCP Secret Manager.")
		}
		log.Println("✓ Security validation passed: all required secrets are configured")
	}

	return config
}
