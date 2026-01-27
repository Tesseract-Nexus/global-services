package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the secret provisioner service
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	GCP       GCPConfig
	NATS      NATSConfig
	Auth      AuthConfig
	Cache     CacheConfig
	Migration MigrationConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port        string
	Host        string
	Environment string
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// GCPConfig holds GCP-specific configuration
type GCPConfig struct {
	ProjectID string
	// Credentials are loaded via Workload Identity - no explicit credentials needed
}

// NATSConfig holds NATS messaging configuration
type NATSConfig struct {
	URL string
}

// AuthConfig holds authentication/authorization configuration
type AuthConfig struct {
	AllowedServices []string
}

// CacheConfig holds caching configuration
type CacheConfig struct {
	SecretTTL time.Duration
}

// MigrationConfig holds secret naming migration configuration
type MigrationConfig struct {
	EnableStartupMigration bool // Whether to run migration check on startup
	DryRunOnly             bool // If true, only log what would be migrated without making changes
}

// NewConfig creates a new Config from environment variables
func NewConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:        getEnv("PORT", "8080"),
			Host:        getEnv("HOST", "0.0.0.0"),
			Environment: getEnv("ENVIRONMENT", "devtest"),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", ""),
			Name:            getEnv("DB_NAME", "secret_provisioner"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getIntEnv("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getIntEnv("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		GCP: GCPConfig{
			ProjectID: getEnv("GCP_PROJECT_ID", ""),
		},
		NATS: NATSConfig{
			URL: getEnv("NATS_URL", "nats://nats:4222"),
		},
		Auth: AuthConfig{
			AllowedServices: getSliceEnv("ALLOWED_SERVICES", []string{"admin-bff", "payment-service"}),
		},
		Cache: CacheConfig{
			SecretTTL: getDurationEnv("SECRET_CACHE_TTL", 10*time.Minute),
		},
		Migration: MigrationConfig{
			EnableStartupMigration: getBoolEnv("ENABLE_STARTUP_MIGRATION", true),
			DryRunOnly:             getBoolEnv("MIGRATION_DRY_RUN_ONLY", false),
		},
	}
}

// DSN returns the database connection string
func (c *DatabaseConfig) DSN() string {
	return "host=" + c.Host +
		" port=" + c.Port +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.Name +
		" sslmode=" + c.SSLMode
}

// IsProd returns true if running in production environment
func (c *ServerConfig) IsProd() bool {
	return c.Environment == "prod" || c.Environment == "production"
}

// Helper functions

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		switch strings.ToLower(value) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return fallback
}

func getSliceEnv(key string, fallback []string) []string {
	if value, exists := os.LookupEnv(key); exists {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return fallback
}
