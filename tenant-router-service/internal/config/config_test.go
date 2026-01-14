package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear environment to test defaults
	os.Clearenv()

	cfg := LoadConfig()

	// Test default values
	if cfg.Server.Port != "8089" {
		t.Errorf("expected default port 8089, got %s", cfg.Server.Port)
	}

	if cfg.Server.Mode != "debug" {
		t.Errorf("expected default mode debug, got %s", cfg.Server.Mode)
	}

	// Test database defaults
	if cfg.Database.Host != "postgresql.database.svc.cluster.local" {
		t.Errorf("expected default DB host, got %s", cfg.Database.Host)
	}

	if cfg.Database.Port != "5432" {
		t.Errorf("expected default DB port 5432, got %s", cfg.Database.Port)
	}

	if cfg.Database.Name != "tesseract_hub" {
		t.Errorf("expected default DB name tesseract_hub, got %s", cfg.Database.Name)
	}

	if cfg.NATS.URL != "nats://nats.devtest.svc.cluster.local:4222" {
		t.Errorf("expected default NATS URL, got %s", cfg.NATS.URL)
	}

	if cfg.Kubernetes.Namespace != "devtest" {
		t.Errorf("expected default namespace devtest, got %s", cfg.Kubernetes.Namespace)
	}

	if cfg.Kubernetes.IstioNamespace != "istio-system" {
		t.Errorf("expected default Istio namespace istio-system, got %s", cfg.Kubernetes.IstioNamespace)
	}

	if cfg.Kubernetes.GatewayName != "main-gateway" {
		t.Errorf("expected default gateway name main-gateway, got %s", cfg.Kubernetes.GatewayName)
	}

	if cfg.Kubernetes.AdminVSName != "admin-vs" {
		t.Errorf("expected default admin VS name admin-vs, got %s", cfg.Kubernetes.AdminVSName)
	}

	if cfg.Kubernetes.StorefrontVSName != "storefront-vs" {
		t.Errorf("expected default storefront VS name storefront-vs, got %s", cfg.Kubernetes.StorefrontVSName)
	}

	if cfg.Kubernetes.ClusterIssuer != "letsencrypt-prod" {
		t.Errorf("expected default cluster issuer letsencrypt-prod, got %s", cfg.Kubernetes.ClusterIssuer)
	}

	if cfg.Domain.BaseDomain != "tesserix.app" {
		t.Errorf("expected default base domain tesserix.app, got %s", cfg.Domain.BaseDomain)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("SERVER_PORT", "9000")
	os.Setenv("GIN_MODE", "release")
	os.Setenv("NATS_URL", "nats://custom-nats:4222")
	os.Setenv("K8S_NAMESPACE", "production")
	os.Setenv("ISTIO_NAMESPACE", "custom-istio")
	os.Setenv("GATEWAY_NAME", "custom-gateway")
	os.Setenv("ADMIN_VS_NAME", "custom-admin-vs")
	os.Setenv("STOREFRONT_VS_NAME", "custom-storefront-vs")
	os.Setenv("CLUSTER_ISSUER", "custom-issuer")
	os.Setenv("BASE_DOMAIN", "example.com")
	defer os.Clearenv()

	cfg := LoadConfig()

	if cfg.Server.Port != "9000" {
		t.Errorf("expected port 9000, got %s", cfg.Server.Port)
	}

	if cfg.Server.Mode != "release" {
		t.Errorf("expected mode release, got %s", cfg.Server.Mode)
	}

	if cfg.NATS.URL != "nats://custom-nats:4222" {
		t.Errorf("expected custom NATS URL, got %s", cfg.NATS.URL)
	}

	if cfg.Kubernetes.Namespace != "production" {
		t.Errorf("expected namespace production, got %s", cfg.Kubernetes.Namespace)
	}

	if cfg.Kubernetes.IstioNamespace != "custom-istio" {
		t.Errorf("expected Istio namespace custom-istio, got %s", cfg.Kubernetes.IstioNamespace)
	}

	if cfg.Kubernetes.GatewayName != "custom-gateway" {
		t.Errorf("expected gateway name custom-gateway, got %s", cfg.Kubernetes.GatewayName)
	}

	if cfg.Domain.BaseDomain != "example.com" {
		t.Errorf("expected base domain example.com, got %s", cfg.Domain.BaseDomain)
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	// Test with existing env var
	value := getEnv("TEST_VAR", "default")
	if value != "test_value" {
		t.Errorf("expected test_value, got %s", value)
	}

	// Test with non-existing env var
	value = getEnv("NON_EXISTING_VAR", "default")
	if value != "default" {
		t.Errorf("expected default, got %s", value)
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	// Test with valid int
	value := getEnvInt("TEST_INT", 10)
	if value != 42 {
		t.Errorf("expected 42, got %d", value)
	}

	// Test with invalid int
	os.Setenv("TEST_INVALID_INT", "not_a_number")
	value = getEnvInt("TEST_INVALID_INT", 10)
	if value != 10 {
		t.Errorf("expected default 10, got %d", value)
	}

	// Test with non-existing env var
	value = getEnvInt("NON_EXISTING_INT", 99)
	if value != 99 {
		t.Errorf("expected default 99, got %d", value)
	}
}
