package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
	"github.com/Tesseract-Nexus/go-shared/secrets"
)

type Config struct {
	Server                 ServerConfig   `mapstructure:"server"`
	Database               DatabaseConfig `mapstructure:"database"`
	Redis                  RedisConfig    `mapstructure:"redis"`
	JWT                    JWTConfig      `mapstructure:"jwt"`
	Azure                  AzureConfig    `mapstructure:"azure"`
	NotificationServiceURL string         `mapstructure:"notification_service_url"`
	TenantServiceURL       string         `mapstructure:"tenant_service_url"`
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

	viper.SetDefault("jwt.secret", "your-super-secret-jwt-key-change-in-production")
	viper.SetDefault("jwt.refresh_secret", "your-refresh-token-secret-here")
	viper.SetDefault("jwt.access_expiry_hours", 8)
	viper.SetDefault("jwt.refresh_expiry_days", 30)

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

	// Unmarshal into config struct
	if err := viper.Unmarshal(config); err != nil {
		log.Fatalf("Unable to decode config: %v", err)
	}

	return config
}
