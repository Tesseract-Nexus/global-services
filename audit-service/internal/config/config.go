package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tesseract-hub/go-shared/secrets")

// Config holds all configuration for the audit service
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	FallbackDB FallbackDBConfig
	App        AppConfig
	Redis      RedisConfig
	NATS       NATSConfig
	Tenant     TenantConfig
	Pool       PoolConfig
	Retention  RetentionConfig
}

// FallbackDBConfig holds fallback database configuration (used when tenant config unavailable)
type FallbackDBConfig struct {
	Enabled      bool
	Host         string
	Port         int
	Database     string
	User         string
	Password     string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

// RetentionConfig holds audit log retention configuration
type RetentionConfig struct {
	DefaultDays     int    // Default retention period in days (180 = 6 months)
	MinDays         int    // Minimum allowed retention (90 = 3 months)
	MaxDays         int    // Maximum allowed retention (365 = 1 year)
	CleanupEnabled  bool   // Whether auto-cleanup is enabled
	CleanupSchedule string // Cron schedule for cleanup job
	BatchSize       int    // Batch size for cleanup operations
}

// NATSConfig holds NATS configuration for real-time event streaming
type NATSConfig struct {
	URL           string
	Enabled       bool
	MaxReconnects int
	ReconnectWait int // In seconds
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host string
	Port int
}

// DatabaseConfig holds legacy database configuration (for backwards compatibility)
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// RedisConfig holds Redis configuration for caching
type RedisConfig struct {
	URL            string
	MaxRetries     int
	PoolSize       int
	MinIdleConns   int
}

// TenantConfig holds tenant registry configuration
type TenantConfig struct {
	RegistryURL   string
	EncryptionKey string // Base64 encoded AES-256 key
	CacheTTL      int    // Cache TTL in seconds
}

// PoolConfig holds connection pool configuration
type PoolConfig struct {
	MaxDBPools      int
	CleanupInterval int // In seconds
	HealthInterval  int // In seconds
	IdleTimeout     int // In seconds
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	Environment string
	LogLevel    string
	JWTSecret   string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvAsInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			DBName:   getEnv("DB_NAME", "audit_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		FallbackDB: FallbackDBConfig{
			Enabled:      getEnvAsBool("FALLBACK_DB_ENABLED", false),
			Host:         getEnv("FALLBACK_DB_HOST", ""),
			Port:         getEnvAsInt("FALLBACK_DB_PORT", 5432),
			Database:     getEnv("FALLBACK_DB_NAME", "audit_logs"),
			User:         getEnv("FALLBACK_DB_USER", "postgres"),
			Password:     getEnv("FALLBACK_DB_PASSWORD", ""),
			SSLMode:      getEnv("FALLBACK_DB_SSLMODE", "disable"),
			MaxOpenConns: getEnvAsInt("FALLBACK_DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvAsInt("FALLBACK_DB_MAX_IDLE_CONNS", 10),
		},
		Redis: RedisConfig{
			URL:          getEnv("REDIS_URL", ""),
			MaxRetries:   getEnvAsInt("REDIS_MAX_RETRIES", 3),
			PoolSize:     getEnvAsInt("REDIS_POOL_SIZE", 10),
			MinIdleConns: getEnvAsInt("REDIS_MIN_IDLE_CONNS", 5),
		},
		NATS: NATSConfig{
			URL:           getEnv("NATS_URL", "nats://nats.nats.svc.cluster.local:4222"),
			Enabled:       getEnvAsBool("NATS_ENABLED", true),
			MaxReconnects: getEnvAsInt("NATS_MAX_RECONNECTS", -1), // -1 = unlimited reconnects for production resilience
			ReconnectWait: getEnvAsInt("NATS_RECONNECT_WAIT", 2),  // seconds
		},
		Tenant: TenantConfig{
			RegistryURL:   getEnv("TENANT_REGISTRY_URL", "http://settings-service:8080"),
			EncryptionKey: getEnv("CREDENTIAL_ENCRYPTION_KEY", ""),
			CacheTTL:      getEnvAsInt("TENANT_CACHE_TTL", 300), // 5 minutes
		},
		Pool: PoolConfig{
			MaxDBPools:      getEnvAsInt("MAX_DB_POOLS", 100),
			CleanupInterval: getEnvAsInt("POOL_CLEANUP_INTERVAL", 300),  // 5 minutes
			HealthInterval:  getEnvAsInt("POOL_HEALTH_INTERVAL", 30),    // 30 seconds
			IdleTimeout:     getEnvAsInt("POOL_IDLE_TIMEOUT", 600),      // 10 minutes
		},
		Retention: RetentionConfig{
			DefaultDays:     getEnvAsInt("AUDIT_RETENTION_DAYS", 180),      // 6 months default
			MinDays:         getEnvAsInt("AUDIT_MIN_RETENTION_DAYS", 90),   // 3 months minimum
			MaxDays:         getEnvAsInt("AUDIT_MAX_RETENTION_DAYS", 365),  // 1 year maximum
			CleanupEnabled:  getEnvAsBool("AUDIT_CLEANUP_ENABLED", true),
			CleanupSchedule: getEnv("AUDIT_CLEANUP_SCHEDULE", "0 2 * * *"), // 2 AM daily
			BatchSize:       getEnvAsInt("AUDIT_BATCH_SIZE", 100),
		},
		App: AppConfig{
			Environment: getEnv("APP_ENV", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
			JWTSecret: secrets.GetJWTSecret(),
		},
	}

	return config, nil
}

// Convenience accessors for backwards compatibility

// RedisURL returns the Redis URL
func (c *Config) RedisURL() string {
	return c.Redis.URL
}

// TenantRegistryURL returns the tenant registry URL
func (c *Config) TenantRegistryURL() string {
	return c.Tenant.RegistryURL
}

// CredentialEncryptionKey returns the credential encryption key
func (c *Config) CredentialEncryptionKey() string {
	return c.Tenant.EncryptionKey
}

// MaxDBPools returns the maximum number of DB pools
func (c *Config) MaxDBPools() int {
	return c.Pool.MaxDBPools
}

// GetDatabaseDSN returns the legacy database connection string
func (c *Config) GetDatabaseDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.DBName,
		c.Database.SSLMode,
	)
}

// GetServerAddress returns the server address
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// IsProduction returns true if running in production
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.App.Environment) == "production"
}

// IsDevelopment returns true if running in development
func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.App.Environment) == "development"
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return defaultValue
}
