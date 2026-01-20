package config

import (
	"os"
	"strconv"

	"github.com/Tesseract-Nexus/go-shared/secrets")

type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	App      AppConfig      `json:"app"`
	Redis    RedisConfig    `json:"redis"`
}

type ServerConfig struct {
	Port string `json:"port"`
	Host string `json:"host"`
	Mode string `json:"mode"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	SSLMode  string `json:"sslmode"`
}

type AppConfig struct {
	Environment string `json:"environment"`
	Debug       bool   `json:"debug"`
	Version     string `json:"version"`
}

type RedisConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Password string `json:"password"`
	DB       string `json:"db"`
	URL      string `json:"url"` // Built from components or can be overridden
}

// NewConfig creates a new configuration instance with environment variables
func NewConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8085"),
			Host: getEnv("HOST", "0.0.0.0"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			DBName:   getEnv("DB_NAME", "settings_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		App: AppConfig{
			Environment: getEnv("ENVIRONMENT", "development"),
			Debug:       getBoolEnv("DEBUG", true),
			Version:     getEnv("VERSION", "1.0.0"),
		},
		Redis: buildRedisConfig(),
	}
}

// buildRedisConfig builds the Redis configuration from environment variables
func buildRedisConfig() RedisConfig {
	// First check for explicit REDIS_URL override
	if url := os.Getenv("REDIS_URL"); url != "" {
		return RedisConfig{URL: url}
	}

	// Build URL from separate components
	host := getEnv("REDIS_HOST", "redis.redis-marketplace.svc.cluster.local")
	port := getEnv("REDIS_PORT", "6379")
	password := os.Getenv("REDIS_PASSWORD")
	db := getEnv("REDIS_DB", "0")

	// Build Redis URL with or without password
	var url string
	if password != "" {
		url = "redis://:" + password + "@" + host + ":" + port + "/" + db
	} else {
		url = "redis://" + host + ":" + port + "/" + db
	}

	return RedisConfig{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       db,
		URL:      url,
	}
}

// DSN returns the database connection string
func (c *DatabaseConfig) DSN() string {
	return "host=" + c.Host +
		" port=" + c.Port +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.DBName +
		" sslmode=" + c.SSLMode
}

// IsDevelopment checks if the app is running in development mode
func (c *AppConfig) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction checks if the app is running in production mode
func (c *AppConfig) IsProduction() bool {
	return c.Environment == "production"
}

// getEnv gets environment variable with fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// getBoolEnv gets boolean environment variable with fallback
func getBoolEnv(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return fallback
}