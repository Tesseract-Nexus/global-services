package config

import (
	"os"
	"strconv"
	"time"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

type Config struct {
	Server       ServerConfig       `json:"server"`
	Database     DatabaseConfig     `json:"database"`
	Redis        RedisConfig        `json:"redis"`
	NATS         NATSConfig         `json:"nats"`
	Keycloak     KeycloakConfig     `json:"keycloak"`
	Istio        IstioConfig        `json:"istio"`
	DNS             DNSConfig             `json:"dns"`
	SSL             SSLConfig             `json:"ssl"`
	CNAMEDelegation CNAMEDelegationConfig `json:"cname_delegation"`
	Cloudflare      CloudflareConfig      `json:"cloudflare"`
	Limits       LimitsConfig       `json:"limits"`
	Tenant       TenantConfig       `json:"tenant"`
	Workers      WorkersConfig      `json:"workers"`
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

type RedisConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Password string `json:"password"`
	DB       string `json:"db"`
	URL      string `json:"url"`
}

type NATSConfig struct {
	URL string `json:"url"`
}

type KeycloakConfig struct {
	AdminURL      string `json:"admin_url"`
	Realm         string `json:"realm"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
	ClientPattern string `json:"client_pattern"`
}

type IstioConfig struct {
	GatewayName      string `json:"gateway_name"`
	GatewayNamespace string `json:"gateway_namespace"`
	VSNamespace      string `json:"vs_namespace"`

	// Shared AuthorizationPolicy configuration for custom domains
	// Instead of creating per-domain policies, we patch the shared policy with domain hosts
	SharedAuthPolicyName      string `json:"shared_auth_policy_name"`
	SharedAuthPolicyNamespace string `json:"shared_auth_policy_namespace"`
	SharedAuthPolicySelector  string `json:"shared_auth_policy_selector"` // e.g., "custom-ingressgateway"
}

type DNSConfig struct {
	VerificationDomain string `json:"verification_domain"`
	ProxyDomain        string `json:"proxy_domain"`
	ProxyIP            string `json:"proxy_ip"`
	PlatformDomain     string `json:"platform_domain"` // Base domain for tenant subdomains (e.g., tesserix.app)
}

type SSLConfig struct {
	IssuerName                 string `json:"issuer_name"`
	IssuerKind                 string `json:"issuer_kind"`
	HTTP01IssuerName           string `json:"http01_issuer_name"`             // Issuer for HTTP-01 challenges (custom domains)
	CNAMEDelegationIssuerName  string `json:"cname_delegation_issuer_name"`   // Issuer for DNS-01 challenges (CNAME-delegated domains)
	RenewalDaysBefore          int    `json:"renewal_days_before"`
	CertificateNamespace       string `json:"certificate_namespace"`
}

// CNAMEDelegationConfig holds CNAME delegation configuration for automatic certificate management
// Customers add: _acme-challenge.theirdomain.com CNAME theirdomain-com.acme.tesserix.app
// cert-manager follows the CNAME and creates TXT records in our Cloudflare zone
type CNAMEDelegationConfig struct {
	Enabled              bool          `json:"enabled"`               // Enable CNAME delegation feature
	ACMEZone             string        `json:"acme_zone"`             // Zone for CNAME targets (e.g., "acme.tesserix.app")
	VerificationInterval time.Duration `json:"verification_interval"` // How often to check CNAME delegation status
	MaxAttempts          int           `json:"max_attempts"`          // Max verification attempts before marking as failed
}

type LimitsConfig struct {
	MaxDomainsPerTenant          int `json:"max_domains_per_tenant"`
	MaxVerificationAttemptsHour  int `json:"max_verification_attempts_hour"`
	MaxSSLProvisioningAttemptsHour int `json:"max_ssl_provisioning_attempts_hour"`
}

type TenantConfig struct {
	ServiceURL string `json:"service_url"`
}

type WorkersConfig struct {
	DNSVerificationInterval time.Duration `json:"dns_verification_interval"`
	CertMonitorInterval     time.Duration `json:"cert_monitor_interval"`
	HealthCheckInterval     time.Duration `json:"health_check_interval"`
	CleanupInterval         time.Duration `json:"cleanup_interval"`
}

type CloudflareConfig struct {
	Enabled       bool   `json:"enabled"`        // Enable Cloudflare Tunnel for custom domains
	APIToken      string `json:"api_token"`      // Cloudflare API token with Tunnel permissions
	AccountID     string `json:"account_id"`     // Cloudflare account ID
	TunnelID      string `json:"tunnel_id"`      // Cloudflare Tunnel ID
	OriginService string `json:"origin_service"` // Origin service URL (Istio ingress)

	// DNS Management
	AutoConfigureDNS bool   `json:"auto_configure_dns"` // Auto-create DNS records in Cloudflare
	DefaultZoneID    string `json:"default_zone_id"`    // Default zone ID for tesserix.app (if customer uses our managed DNS)

	// SSL Configuration
	SSLMode string `json:"ssl_mode"` // "full" (Cloudflare handles SSL) or "flexible" (HTTP to origin)

	// Cloudflare for SaaS (Custom Hostnames)
	// This is the correct approach for multi-tenant custom domains where customers
	// use their OWN domain (not in your Cloudflare account)
	SaaSZoneID     string `json:"saas_zone_id"`     // Zone ID where Custom Hostnames are created (tesserix.app zone)
	FallbackOrigin string `json:"fallback_origin"`  // CNAME target for customers (e.g., customers.tesserix.app)
}

func NewConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8093"),
			Host: getEnv("HOST", "0.0.0.0"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: secrets.GetDBPassword(),
			DBName:   getEnv("DB_NAME", "custom_domains_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis:    buildRedisConfig(),
		NATS: NATSConfig{
			URL: getEnv("NATS_URL", "nats://localhost:4222"),
		},
		Keycloak: KeycloakConfig{
			AdminURL:      getEnv("KEYCLOAK_ADMIN_URL", ""),
			Realm:         getEnv("KEYCLOAK_REALM", "tesserix-customer"),
			ClientID:      getEnv("KEYCLOAK_ADMIN_CLIENT_ID", "admin-cli"),
			ClientSecret:  getEnv("KEYCLOAK_ADMIN_CLIENT_SECRET", ""),
			ClientPattern: getEnv("KEYCLOAK_CLIENT_PATTERN", "marketplace-storefront"),
		},
		Istio: IstioConfig{
			GatewayName:      getEnv("ISTIO_GATEWAY_NAME", "custom-domains-gateway"),
			GatewayNamespace: getEnv("ISTIO_GATEWAY_NAMESPACE", "istio-system"),
			VSNamespace:      getEnv("ISTIO_VS_NAMESPACE", "marketplace"),
			// Shared AuthorizationPolicy for custom domain traffic at the custom ingress gateway
			SharedAuthPolicyName:      getEnv("ISTIO_SHARED_AUTH_POLICY_NAME", "allow-frontend-apps-public-custom"),
			SharedAuthPolicyNamespace: getEnv("ISTIO_SHARED_AUTH_POLICY_NAMESPACE", "istio-ingress"),
			SharedAuthPolicySelector:  getEnv("ISTIO_SHARED_AUTH_POLICY_SELECTOR", "custom-ingressgateway"),
		},
		DNS: DNSConfig{
			VerificationDomain: getEnv("DNS_VERIFICATION_DOMAIN", "tesserix.app"),
			ProxyDomain:        getEnv("DNS_PROXY_DOMAIN", "proxy.tesserix.app"),
			ProxyIP:            getEnv("DNS_PROXY_IP", ""),
			PlatformDomain:     getEnv("DNS_PLATFORM_DOMAIN", "tesserix.app"),
		},
		SSL: SSLConfig{
			IssuerName:                getEnv("SSL_ISSUER_NAME", "letsencrypt-prod"),
			IssuerKind:                getEnv("SSL_ISSUER_KIND", "ClusterIssuer"),
			HTTP01IssuerName:          getEnv("SSL_HTTP01_ISSUER_NAME", "letsencrypt-prod-http01"),
			CNAMEDelegationIssuerName: getEnv("SSL_CNAME_DELEGATION_ISSUER_NAME", "letsencrypt-prod-cname-delegation"),
			RenewalDaysBefore:         getIntEnv("SSL_RENEWAL_DAYS_BEFORE", 30),
			CertificateNamespace:      getEnv("SSL_CERTIFICATE_NAMESPACE", "istio-system"),
		},
		CNAMEDelegation: CNAMEDelegationConfig{
			Enabled:              getBoolEnv("CNAME_DELEGATION_ENABLED", false),
			ACMEZone:             getEnv("CNAME_DELEGATION_ACME_ZONE", "acme.tesserix.app"),
			VerificationInterval: getDurationEnv("CNAME_DELEGATION_VERIFICATION_INTERVAL", 5*time.Minute),
			MaxAttempts:          getIntEnv("CNAME_DELEGATION_MAX_ATTEMPTS", 100),
		},
		Cloudflare: CloudflareConfig{
			Enabled:          getBoolEnv("CLOUDFLARE_TUNNEL_ENABLED", true),
			APIToken:         getEnv("CLOUDFLARE_API_TOKEN", ""),
			AccountID:        getEnv("CLOUDFLARE_ACCOUNT_ID", ""),
			TunnelID:         getEnv("CLOUDFLARE_TUNNEL_ID", ""),
			OriginService:    getEnv("CLOUDFLARE_ORIGIN_SERVICE", "http://istio-ingressgateway.istio-ingress.svc.cluster.local:80"),
			AutoConfigureDNS: getBoolEnv("CLOUDFLARE_AUTO_CONFIGURE_DNS", true),
			DefaultZoneID:    getEnv("CLOUDFLARE_DEFAULT_ZONE_ID", ""),
			SSLMode:          getEnv("CLOUDFLARE_SSL_MODE", "full"),
			// Cloudflare for SaaS - required for custom domains from external Cloudflare accounts
			SaaSZoneID:     getEnv("CLOUDFLARE_SAAS_ZONE_ID", ""),     // Zone ID of tesserix.app
			FallbackOrigin: getEnv("CLOUDFLARE_FALLBACK_ORIGIN", ""), // e.g., customers.tesserix.app
		},
		Limits: LimitsConfig{
			MaxDomainsPerTenant:            getIntEnv("MAX_DOMAINS_PER_TENANT", 5),
			MaxVerificationAttemptsHour:    getIntEnv("MAX_VERIFICATION_ATTEMPTS_HOUR", 10),
			MaxSSLProvisioningAttemptsHour: getIntEnv("MAX_SSL_PROVISIONING_ATTEMPTS_HOUR", 3),
		},
		Tenant: TenantConfig{
			ServiceURL: getEnv("TENANT_SERVICE_URL", "http://tenant-service:8080"),
		},
		Workers: WorkersConfig{
			DNSVerificationInterval: getDurationEnv("DNS_VERIFICATION_INTERVAL", 5*time.Minute),
			CertMonitorInterval:     getDurationEnv("CERT_MONITOR_INTERVAL", 24*time.Hour),
			HealthCheckInterval:     getDurationEnv("HEALTH_CHECK_INTERVAL", 15*time.Minute),
			CleanupInterval:         getDurationEnv("CLEANUP_INTERVAL", 24*time.Hour),
		},
	}
}

func (c *DatabaseConfig) DSN() string {
	return "host=" + c.Host +
		" port=" + c.Port +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.DBName +
		" sslmode=" + c.SSLMode
}

func buildRedisConfig() RedisConfig {
	if url := os.Getenv("REDIS_URL"); url != "" {
		return RedisConfig{URL: url}
	}

	host := getEnv("REDIS_HOST", "localhost")
	port := getEnv("REDIS_PORT", "6379")
	password := os.Getenv("REDIS_PASSWORD")
	db := getEnv("REDIS_DB", "0")

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

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return fallback
}

func getStringSliceEnv(key string, fallback []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma, trim whitespace
		parts := make([]string, 0)
		for _, part := range splitAndTrim(value, ",") {
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) > 0 {
			return parts
		}
	}
	return fallback
}

func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		parts = append(parts, trimmed)
	}
	return parts
}

func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	result := make([]string, 0)
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
