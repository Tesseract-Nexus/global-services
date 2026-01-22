package config

import (
	"os"
	"strconv"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

// Config holds the application configuration
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	NATS       NATSConfig
	Kubernetes K8sConfig
	Domain     DomainConfig
}

// RedisConfig holds Redis configuration for caching platform settings
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// DatabaseConfig holds PostgreSQL database configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
	Mode string // "debug" or "release"
}

// NATSConfig holds NATS configuration
type NATSConfig struct {
	URL string
}

// K8sConfig holds Kubernetes configuration
type K8sConfig struct {
	Namespace          string
	IstioNamespace     string
	GatewayName        string
	AdminVSName        string
	StorefrontVSName   string
	APIVSName          string // Template VirtualService for mobile/external API access
	ClusterIssuer      string
	SkipGatewayPatch   bool   // Skip gateway patching when using wildcard certificate
	WildcardCertName   string // Name of the wildcard certificate (e.g., "storefront-wildcard-tls")

	// Custom domain configuration
	// Custom domains use a separate gateway and ClusterIssuer for Let's Encrypt HTTP-01 challenges
	CustomDomainGateway       string // Gateway for custom domains (e.g., "custom-domain-gateway")
	CustomDomainGatewayNS     string // Namespace for custom domain gateway (e.g., "istio-ingress")
	CustomDomainClusterIssuer string // ClusterIssuer for custom domain certs (HTTP-01)

	// Shared AuthorizationPolicy for custom domain RBAC
	// When custom domain VirtualServices are created, the hosts need to be added to this
	// AuthorizationPolicy to allow traffic through the custom-ingressgateway
	SharedAuthPolicyName      string // Name of the shared AuthorizationPolicy
	SharedAuthPolicyNamespace string // Namespace where the policy is located
	SharedAuthPolicySelector  string // Workload selector label (e.g., "custom-ingressgateway")
}

// DomainConfig holds domain configuration
type DomainConfig struct {
	BaseDomain string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8089"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "postgresql.database.svc.cluster.local"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			Name:     getEnv("DB_NAME", "tesseract_hub"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "redis.redis-marketplace.svc.cluster.local"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		NATS: NATSConfig{
			URL: getEnv("NATS_URL", "nats://nats.devtest.svc.cluster.local:4222"),
		},
		Kubernetes: K8sConfig{
			Namespace:        getEnv("K8S_NAMESPACE", "devtest"),
			IstioNamespace:   getEnv("ISTIO_NAMESPACE", "istio-system"),
			GatewayName:      getEnv("GATEWAY_NAME", "main-gateway"),
			AdminVSName:      getEnv("ADMIN_VS_NAME", "admin-vs"),
			StorefrontVSName: getEnv("STOREFRONT_VS_NAME", "storefront-vs"),
			APIVSName:        getEnv("API_VS_NAME", "api-vs"),
			ClusterIssuer:    getEnv("CLUSTER_ISSUER", "letsencrypt-prod"),
			SkipGatewayPatch: getEnvBool("SKIP_GATEWAY_PATCH", true), // Default true: use wildcard cert
			WildcardCertName: getEnv("WILDCARD_CERT_NAME", "storefront-wildcard-tls"),
			// Custom domain configuration - uses separate gateway for direct LoadBalancer access
			// This allows customers to point their domains directly to the platform without Cloudflare
			// NOTE: CustomDomainGateway is the default/fallback gateway. Each custom domain gets its own dedicated gateway.
			CustomDomainGateway:       getEnv("CUSTOM_DOMAIN_GATEWAY", "custom-domain-gateway"),
			CustomDomainGatewayNS:     getEnv("CUSTOM_DOMAIN_GATEWAY_NS", "istio-ingress"),
			CustomDomainClusterIssuer: getEnv("CUSTOM_DOMAIN_CLUSTER_ISSUER", "letsencrypt-custom-domain"),
			// Shared AuthorizationPolicy for custom domain RBAC
			SharedAuthPolicyName:      getEnv("SHARED_AUTH_POLICY_NAME", "allow-frontend-apps-public-custom"),
			SharedAuthPolicyNamespace: getEnv("SHARED_AUTH_POLICY_NAMESPACE", "istio-ingress"),
			SharedAuthPolicySelector:  getEnv("SHARED_AUTH_POLICY_SELECTOR", "custom-ingressgateway"),
		},
		Domain: DomainConfig{
			BaseDomain: getEnv("BASE_DOMAIN", "tesserix.app"),
		},
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as int with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets an environment variable as bool with a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
