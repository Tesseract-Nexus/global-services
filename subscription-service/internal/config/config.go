package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

type Config struct {
	Port        string
	Environment string
	DatabaseURL string

	StripeSecretKey     string
	StripeWebhookSecret string

	TenantServiceURL string
	NatsURL          string
}

func buildDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	dbname := getEnv("DB_NAME", "tesseract_hub")
	sslmode := getEnv("DB_SSLMODE", "disable")
	password := getPasswordFromGCPOrEnv()

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode)
}

func getPasswordFromGCPOrEnv() string {
	useGCP := os.Getenv("USE_GCP_SECRET_MANAGER")
	if useGCP != "true" {
		return getEnv("DB_PASSWORD", "password")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	secretFetcher, err := secrets.NewEnvSecretFetcher(ctx)
	if err != nil {
		log.Printf("Warning: Failed to initialize GCP Secret Manager: %v (using env var)", err)
		return getEnv("DB_PASSWORD", "password")
	}
	defer secretFetcher.Close()

	password := secrets.LoadDatabasePassword(ctx, secretFetcher)
	if password == "" || password == "password" {
		log.Printf("Warning: Got empty/default password from GCP Secret Manager, using env var")
		return getEnv("DB_PASSWORD", "password")
	}

	log.Printf("Database password loaded from GCP Secret Manager")
	return password
}

func Load() *Config {
	config := &Config{
		Port:                getEnv("PORT", "8093"),
		Environment:         getEnv("ENVIRONMENT", "devtest"),
		DatabaseURL:         buildDatabaseURL(),
		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		TenantServiceURL:    getEnv("TENANT_SERVICE_URL", "http://tenant-service.marketplace.svc.cluster.local:8080"),
		NatsURL:             getEnv("NATS_URL", ""),
	}

	if config.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	return config
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
