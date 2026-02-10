package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Tesseract-Nexus/go-shared/secrets"
	"github.com/stripe/stripe-go/v76"
)

type Config struct {
	Port        string
	Environment string
	DatabaseURL string

	StripeSecretKey     string
	StripeWebhookSecret string

	TenantServiceURL string
	NatsURL          string

	DefaultTrialPlan string
	GracePeriodDays  int

	GCPProjectID        string
	UseGCPSecretManager bool

	mu sync.RWMutex
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
		StripeSecretKey:     secrets.GetSecretOrEnv("STRIPE_SECRET_KEY_SECRET_NAME", "STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: secrets.GetSecretOrEnv("STRIPE_WEBHOOK_SECRET_SECRET_NAME", "STRIPE_WEBHOOK_SECRET", ""),
		TenantServiceURL:    getEnv("TENANT_SERVICE_URL", "http://tenant-service.marketplace.svc.cluster.local:8080"),
		NatsURL:             getEnv("NATS_URL", ""),
		DefaultTrialPlan:    getEnv("DEFAULT_TRIAL_PLAN", "starter_inr"),
		GracePeriodDays:     getEnvInt("GRACE_PERIOD_DAYS", 7),
		GCPProjectID:        getEnv("GCP_PROJECT_ID", ""),
		UseGCPSecretManager: os.Getenv("USE_GCP_SECRET_MANAGER") == "true",
	}

	if config.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	return config
}

// GetStripeSecretKey returns the current Stripe secret key (thread-safe).
func (c *Config) GetStripeSecretKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.StripeSecretKey
}

// GetStripeWebhookSecret returns the current Stripe webhook secret (thread-safe).
func (c *Config) GetStripeWebhookSecret() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.StripeWebhookSecret
}

// ReloadStripeKeys re-reads Stripe keys from GCP Secret Manager (or env vars)
// and updates the in-memory config and stripe.Key global.
func (c *Config) ReloadStripeKeys() {
	newSecretKey := secrets.GetSecretOrEnv("STRIPE_SECRET_KEY_SECRET_NAME", "STRIPE_SECRET_KEY", "")
	newWebhookSecret := secrets.GetSecretOrEnv("STRIPE_WEBHOOK_SECRET_SECRET_NAME", "STRIPE_WEBHOOK_SECRET", "")

	c.mu.Lock()
	c.StripeSecretKey = newSecretKey
	c.StripeWebhookSecret = newWebhookSecret
	c.mu.Unlock()

	if newSecretKey != "" {
		stripe.Key = newSecretKey
		log.Println("Stripe API key reloaded")
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intVal
}
