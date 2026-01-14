package config

import (
	"os"
	"strconv"
)

// Config holds all configuration for the search service
type Config struct {
	Environment string

	// Typesense configuration
	TypesenseHost     string
	TypesensePort     int
	TypesenseProtocol string
	TypesenseAPIKey   string

	// Redis configuration
	RedisURL string

	// Service URLs for data sync
	ProductsServiceURL   string
	CustomersServiceURL  string
	OrdersServiceURL     string
	CategoriesServiceURL string

	// Sync settings
	SyncBatchSize int
	SyncTimeout   int // seconds

	// Performance settings
	SearchTimeout    int // seconds
	MaxResultsPerPage int
	DefaultPageSize   int
}

// Load creates a Config from environment variables
func Load() *Config {
	return &Config{
		Environment: getEnv("ENVIRONMENT", "development"),

		TypesenseHost:     getEnv("TYPESENSE_HOST", "typesense.typesense.svc.cluster.local"),
		TypesensePort:     getEnvInt("TYPESENSE_PORT", 8108),
		TypesenseProtocol: getEnv("TYPESENSE_PROTOCOL", "http"),
		TypesenseAPIKey:   getEnv("TYPESENSE_API_KEY", ""),

		RedisURL: getEnv("REDIS_URL", "redis://redis:6379/0"),

		ProductsServiceURL:   getEnv("PRODUCTS_SERVICE_URL", "http://products-service:8087"),
		CustomersServiceURL:  getEnv("CUSTOMERS_SERVICE_URL", "http://customers-service:8084"),
		OrdersServiceURL:     getEnv("ORDERS_SERVICE_URL", "http://orders-service:8081"),
		CategoriesServiceURL: getEnv("CATEGORIES_SERVICE_URL", "http://products-service:8087"), // Categories are part of products-service

		SyncBatchSize: getEnvInt("SYNC_BATCH_SIZE", 100),
		SyncTimeout:   getEnvInt("SYNC_TIMEOUT", 300), // 5 minutes for full sync

		SearchTimeout:     getEnvInt("SEARCH_TIMEOUT", 10),
		MaxResultsPerPage: getEnvInt("MAX_RESULTS_PER_PAGE", 100),
		DefaultPageSize:   getEnvInt("DEFAULT_PAGE_SIZE", 20),
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
