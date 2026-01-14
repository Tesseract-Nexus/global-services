package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration
type Config struct {
	ServerAddress  string
	Environment    string
	CheckInterval  time.Duration
	RequestTimeout time.Duration
	Services       []ServiceConfig
}

// ServiceConfig defines a service to monitor
type ServiceConfig struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url"`
	HealthPath  string `json:"healthPath"`
	Category    string `json:"category"`
	SLATarget   float64 `json:"slaTarget"` // Target uptime percentage (e.g., 99.9)
}

// Load loads configuration from environment variables
func Load() *Config {
	checkInterval, _ := strconv.Atoi(getEnv("CHECK_INTERVAL_SECONDS", "30"))
	requestTimeout, _ := strconv.Atoi(getEnv("REQUEST_TIMEOUT_SECONDS", "5"))

	cfg := &Config{
		ServerAddress:  getEnv("SERVER_ADDRESS", ":8097"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		CheckInterval:  time.Duration(checkInterval) * time.Second,
		RequestTimeout: time.Duration(requestTimeout) * time.Second,
		Services:       loadServicesConfig(),
	}

	return cfg
}

// loadServicesConfig loads the services to monitor from environment
func loadServicesConfig() []ServiceConfig {
	namespace := getEnv("SERVICES_NAMESPACE", "devtest")

	// Build service URL with correct port
	svcURL := func(name string, port int) string {
		return "http://" + name + "." + namespace + ".svc.cluster.local:" + strconv.Itoa(port)
	}

	// Services with their actual ports
	services := []ServiceConfig{
		// Infrastructure Services (cross-namespace)
		{Name: "nats", DisplayName: "NATS Messaging", URL: "http://nats.nats.svc.cluster.local:8222", HealthPath: "/healthz", Category: "Infrastructure", SLATarget: 99.9},
		{Name: "growthbook", DisplayName: "GrowthBook", URL: "http://growthbook.growthbook.svc.cluster.local:3000", HealthPath: "/", Category: "Infrastructure", SLATarget: 99.0},

		// Identity Services (use realms endpoint as health check - port 8080 only exposed)
		{Name: "keycloak-customer", DisplayName: "Keycloak (Customer)", URL: "http://keycloak.identity-customer.svc.cluster.local:8080", HealthPath: "/realms/master/.well-known/openid-configuration", Category: "Identity", SLATarget: 99.9},

		// API Documentation
		{Name: "swagger-ui", DisplayName: "Swagger UI", URL: svcURL("swagger-ui", 8080), HealthPath: "/api/docs/", Category: "Documentation", SLATarget: 98.0},

		// Core Services (auth is handled by Keycloak)
		{Name: "tenant-service", DisplayName: "Tenant Management", URL: svcURL("tenant-service", 8080), HealthPath: "/health", Category: "Core", SLATarget: 99.9},
		{Name: "staff-service", DisplayName: "Staff Management", URL: svcURL("staff-service", 8080), HealthPath: "/health", Category: "Core", SLATarget: 99.5},

		// Commerce Services
		{Name: "products-service", DisplayName: "Products", URL: svcURL("products-service", 8080), HealthPath: "/health", Category: "Commerce", SLATarget: 99.9},
		{Name: "orders-service", DisplayName: "Orders", URL: svcURL("orders-service", 8080), HealthPath: "/health", Category: "Commerce", SLATarget: 99.9},
		{Name: "inventory-service", DisplayName: "Inventory", URL: svcURL("inventory-service", 8088), HealthPath: "/health", Category: "Commerce", SLATarget: 99.5},
		{Name: "payment-service", DisplayName: "Payments", URL: svcURL("payment-service", 8080), HealthPath: "/health", Category: "Commerce", SLATarget: 99.9},
		{Name: "shipping-service", DisplayName: "Shipping", URL: svcURL("shipping-service", 8080), HealthPath: "/health", Category: "Commerce", SLATarget: 99.5},
		{Name: "tax-service", DisplayName: "Tax Calculation", URL: svcURL("tax-service", 8091), HealthPath: "/health", Category: "Commerce", SLATarget: 99.5},
		{Name: "coupons-service", DisplayName: "Coupons", URL: svcURL("coupons-service", 8080), HealthPath: "/health", Category: "Commerce", SLATarget: 99.0},
		{Name: "gift-cards-service", DisplayName: "Gift Cards", URL: svcURL("gift-cards-service", 8080), HealthPath: "/health", Category: "Commerce", SLATarget: 99.0},

		// Customer Services
		{Name: "customers-service", DisplayName: "Customers", URL: svcURL("customers-service", 8080), HealthPath: "/health", Category: "Customer", SLATarget: 99.5},
		{Name: "reviews-service", DisplayName: "Reviews", URL: svcURL("reviews-service", 8080), HealthPath: "/health", Category: "Customer", SLATarget: 99.0},
		{Name: "tickets-service", DisplayName: "Support Tickets", URL: svcURL("tickets-service", 8080), HealthPath: "/health", Category: "Customer", SLATarget: 99.5},

		// Catalog Services
		{Name: "categories-service", DisplayName: "Categories", URL: svcURL("categories-service", 8080), HealthPath: "/health", Category: "Catalog", SLATarget: 99.5},
		{Name: "search-service", DisplayName: "Search", URL: svcURL("search-service", 8080), HealthPath: "/health", Category: "Catalog", SLATarget: 99.5},

		// Communication Services
		{Name: "notification-service", DisplayName: "Notifications", URL: svcURL("notification-service", 8090), HealthPath: "/health", Category: "Communication", SLATarget: 99.0},
		{Name: "notification-hub", DisplayName: "Notification Hub", URL: svcURL("notification-hub", 8080), HealthPath: "/health", Category: "Communication", SLATarget: 99.0},

		// Vendor Services
		{Name: "vendor-service", DisplayName: "Vendors", URL: svcURL("vendor-service", 8080), HealthPath: "/health", Category: "Vendor", SLATarget: 99.5},
		{Name: "marketplace-connector-service", DisplayName: "Marketplace Connector", URL: svcURL("marketplace-connector-service", 8080), HealthPath: "/health", Category: "Vendor", SLATarget: 99.0},

		// Supporting Services
		{Name: "analytics-service", DisplayName: "Analytics", URL: svcURL("analytics-service", 8092), HealthPath: "/health", Category: "Supporting", SLATarget: 98.0},
		{Name: "audit-service", DisplayName: "Audit Logs", URL: svcURL("audit-service", 8080), HealthPath: "/health", Category: "Supporting", SLATarget: 99.0},
		{Name: "document-service", DisplayName: "Documents", URL: svcURL("document-service", 8080), HealthPath: "/health", Category: "Supporting", SLATarget: 99.0},
		{Name: "marketing-service", DisplayName: "Marketing", URL: svcURL("marketing-service", 8080), HealthPath: "/health", Category: "Supporting", SLATarget: 98.0},
		{Name: "settings-service", DisplayName: "Settings", URL: svcURL("settings-service", 8085), HealthPath: "/health", Category: "Supporting", SLATarget: 99.5},
		{Name: "location-service", DisplayName: "Location", URL: svcURL("location-service", 8080), HealthPath: "/health", Category: "Supporting", SLATarget: 99.0},
		{Name: "translation-service", DisplayName: "Translation", URL: svcURL("translation-service", 8080), HealthPath: "/health", Category: "Supporting", SLATarget: 98.0},
		{Name: "verification-service", DisplayName: "Verification", URL: svcURL("verification-service", 8080), HealthPath: "/health", Category: "Supporting", SLATarget: 99.0},
		{Name: "feature-flags-service", DisplayName: "Feature Flags", URL: svcURL("feature-flags-service", 8080), HealthPath: "/health", Category: "Supporting", SLATarget: 99.5},
		{Name: "qr-service", DisplayName: "QR Codes", URL: svcURL("qr-service", 8080), HealthPath: "/health", Category: "Supporting", SLATarget: 98.0},
		{Name: "approval-service", DisplayName: "Approvals", URL: svcURL("approval-service", 8099), HealthPath: "/health", Category: "Supporting", SLATarget: 99.0},
	}

	return services
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
