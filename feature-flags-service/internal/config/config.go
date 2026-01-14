package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the feature flags service
type Config struct {
	Environment string

	// Growthbook configuration
	GrowthbookAPIHost     string
	GrowthbookAPIPort     int
	GrowthbookAdminAPIKey string // For admin operations

	// Cache settings
	CacheTTLSeconds  int
	EnableCache      bool

	// Feature defaults
	DefaultFeatures map[string]bool
}

// Load creates a Config from environment variables
func Load() *Config {
	return &Config{
		Environment: getEnv("ENVIRONMENT", "development"),

		GrowthbookAPIHost:     getEnv("GROWTHBOOK_API_HOST", "growthbook.growthbook.svc.cluster.local"),
		GrowthbookAPIPort:     getEnvInt("GROWTHBOOK_API_PORT", 3100),
		GrowthbookAdminAPIKey: getEnv("GROWTHBOOK_ADMIN_API_KEY", ""),

		CacheTTLSeconds: getEnvInt("CACHE_TTL_SECONDS", 60),
		EnableCache:     getEnvBool("ENABLE_CACHE", true),

		DefaultFeatures: map[string]bool{
			"global_search_enabled": true,
			"dark_mode":             false,
			"new_checkout_flow":     false,
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
