package clients

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// K8sClient provides access to Kubernetes resources
type K8sClient struct {
	clientset *kubernetes.Clientset

	// Cache for gateway IP to avoid frequent API calls
	gatewayIP      string
	gatewayIPMu    sync.RWMutex
	lastFetched    time.Time
	cacheDuration  time.Duration

	// Configuration
	gatewayServiceName      string
	gatewayServiceNamespace string
}

// K8sClientConfig holds configuration for the K8s client
type K8sClientConfig struct {
	// GatewayServiceName is the name of the custom domain gateway Service
	// Default: "custom-ingressgateway"
	GatewayServiceName string

	// GatewayServiceNamespace is the namespace of the gateway Service
	// Default: "istio-ingress"
	GatewayServiceNamespace string

	// CacheDuration is how long to cache the gateway IP
	// Default: 5 minutes
	CacheDuration time.Duration
}

// NewK8sClient creates a new Kubernetes client
// Uses in-cluster config when running in Kubernetes
func NewK8sClient(cfg K8sClientConfig) (*K8sClient, error) {
	// Use in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Set defaults
	if cfg.GatewayServiceName == "" {
		cfg.GatewayServiceName = "custom-ingressgateway"
	}
	if cfg.GatewayServiceNamespace == "" {
		cfg.GatewayServiceNamespace = "istio-ingress"
	}
	if cfg.CacheDuration == 0 {
		cfg.CacheDuration = 5 * time.Minute
	}

	client := &K8sClient{
		clientset:               clientset,
		gatewayServiceName:      cfg.GatewayServiceName,
		gatewayServiceNamespace: cfg.GatewayServiceNamespace,
		cacheDuration:           cfg.CacheDuration,
	}

	// Pre-fetch the gateway IP
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if ip, err := client.GetCustomDomainGatewayIP(ctx); err != nil {
			log.Printf("[K8sClient] Warning: Failed to pre-fetch gateway IP: %v", err)
		} else {
			log.Printf("[K8sClient] Pre-fetched custom domain gateway IP: %s", ip)
		}
	}()

	return client, nil
}

// GetCustomDomainGatewayIP returns the LoadBalancer IP of the custom domain gateway
// Results are cached to minimize API calls
func (c *K8sClient) GetCustomDomainGatewayIP(ctx context.Context) (string, error) {
	// Check cache first
	c.gatewayIPMu.RLock()
	if c.gatewayIP != "" && time.Since(c.lastFetched) < c.cacheDuration {
		ip := c.gatewayIP
		c.gatewayIPMu.RUnlock()
		return ip, nil
	}
	c.gatewayIPMu.RUnlock()

	// Fetch from Kubernetes API
	c.gatewayIPMu.Lock()
	defer c.gatewayIPMu.Unlock()

	// Double-check after acquiring write lock
	if c.gatewayIP != "" && time.Since(c.lastFetched) < c.cacheDuration {
		return c.gatewayIP, nil
	}

	svc, err := c.clientset.CoreV1().Services(c.gatewayServiceNamespace).Get(
		ctx,
		c.gatewayServiceName,
		metav1.GetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get gateway service %s/%s: %w",
			c.gatewayServiceNamespace, c.gatewayServiceName, err)
	}

	// Extract LoadBalancer IP
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return "", fmt.Errorf("gateway service %s/%s has no LoadBalancer ingress",
			c.gatewayServiceNamespace, c.gatewayServiceName)
	}

	ingress := svc.Status.LoadBalancer.Ingress[0]
	ip := ingress.IP
	if ip == "" {
		// Some cloud providers use hostname instead of IP
		ip = ingress.Hostname
	}

	if ip == "" {
		return "", fmt.Errorf("gateway service %s/%s has no IP or hostname",
			c.gatewayServiceNamespace, c.gatewayServiceName)
	}

	// Update cache
	c.gatewayIP = ip
	c.lastFetched = time.Now()
	log.Printf("[K8sClient] Fetched custom domain gateway IP: %s (cached for %v)", ip, c.cacheDuration)

	return ip, nil
}

// RefreshGatewayIP forces a refresh of the cached gateway IP
func (c *K8sClient) RefreshGatewayIP(ctx context.Context) (string, error) {
	c.gatewayIPMu.Lock()
	c.gatewayIP = ""
	c.lastFetched = time.Time{}
	c.gatewayIPMu.Unlock()

	return c.GetCustomDomainGatewayIP(ctx)
}
