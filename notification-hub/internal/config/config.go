package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Tesseract-Nexus/go-shared/secrets")

// Config holds all configuration for the notification-hub service
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	NATS      NATSConfig
	WebSocket WebSocketConfig
	App       AppConfig
	Auth      AuthConfig
}

// AuthConfig holds auth-bff configuration for ticket validation
type AuthConfig struct {
	BffURL string
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host string
	Port int
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NATSConfig holds NATS connection configuration
type NATSConfig struct {
	URL           string
	ConsumerName  string
	MaxReconnects int
	ReconnectWait time.Duration
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	PingInterval    time.Duration
	PongWait        time.Duration
	WriteWait       time.Duration
	MaxMessageSize  int64
}

// AppConfig holds application-specific configuration
type AppConfig struct {
	Environment string
	LogLevel    string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvAsInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			DBName:   getEnv("DB_NAME", "tesseract_hub"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		NATS: NATSConfig{
			URL:           getEnv("NATS_URL", "nats://nats.nats.svc.cluster.local:4222"),
			ConsumerName:  getEnv("NATS_CONSUMER_NAME", "notification-hub"),
			MaxReconnects: getEnvAsInt("NATS_MAX_RECONNECTS", -1), // -1 = unlimited reconnects for production resilience
			ReconnectWait: getEnvAsDuration("NATS_RECONNECT_WAIT", 2*time.Second),
		},
		WebSocket: WebSocketConfig{
			ReadBufferSize:  getEnvAsInt("WS_READ_BUFFER_SIZE", 1024),
			WriteBufferSize: getEnvAsInt("WS_WRITE_BUFFER_SIZE", 1024),
			PingInterval:    getEnvAsDuration("WS_PING_INTERVAL", 30*time.Second),
			PongWait:        getEnvAsDuration("WS_PONG_WAIT", 60*time.Second),
			WriteWait:       getEnvAsDuration("WS_WRITE_WAIT", 10*time.Second),
			MaxMessageSize:  getEnvAsInt64("WS_MAX_MESSAGE_SIZE", 512*1024), // 512KB
		},
		App: AppConfig{
			Environment: getEnv("APP_ENV", "development"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
		},
		Auth: AuthConfig{
			BffURL: getEnv("AUTH_BFF_URL", "http://auth-bff.identity.svc.cluster.local:8080"),
		},
	}, nil
}

// GetServerAddress returns the server address in host:port format
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// GetDatabaseDSN returns the PostgreSQL connection string
func (c *Config) GetDatabaseDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.DBName,
		c.Database.SSLMode,
	)
}

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
