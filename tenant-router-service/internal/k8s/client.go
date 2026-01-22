package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certmanagerclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	securityv1beta1 "istio.io/api/security/v1beta1"
	typev1beta1 "istio.io/api/type/v1beta1"
	istioversionedclient "istio.io/client-go/pkg/clientset/versioned"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"tenant-router-service/internal/config"
)

// Client wraps Kubernetes clients for Istio and cert-manager
type Client struct {
	core             kubernetes.Interface
	istio            istioversionedclient.Interface
	certmanager      certmanagerclient.Interface
	config           *config.Config
	vsNamespaceCache map[string]string // Cache: vsName -> namespace
	gwNamespaceCache map[string]string // Cache: gwName -> namespace
}

// NewClient creates a new Kubernetes client
func NewClient(cfg *config.Config) (*Client, error) {
	// Use in-cluster config
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	// Create Istio client
	istioClient, err := istioversionedclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Istio client: %w", err)
	}

	// Create cert-manager client
	cmClient, err := certmanagerclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create cert-manager client: %w", err)
	}

	// Create core Kubernetes client for Service lookups
	coreClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create core Kubernetes client: %w", err)
	}

	log.Println("[K8s] Kubernetes clients initialized successfully")

	return &Client{
		core:             coreClient,
		istio:            istioClient,
		certmanager:      cmClient,
		config:           cfg,
		vsNamespaceCache: make(map[string]string),
		gwNamespaceCache: make(map[string]string),
	}, nil
}

// GetCustomDomainGatewayIP fetches the LoadBalancer IP of the custom domain gateway Service
func (c *Client) GetCustomDomainGatewayIP(ctx context.Context) (string, error) {
	serviceName := c.config.Kubernetes.CustomDomainGateway
	namespace := c.config.Kubernetes.CustomDomainGatewayNS

	svc, err := c.core.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get service %s/%s: %w", namespace, serviceName, err)
	}

	// Extract LoadBalancer IP
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return "", fmt.Errorf("service %s/%s has no LoadBalancer ingress", namespace, serviceName)
	}

	ingress := svc.Status.LoadBalancer.Ingress[0]
	ip := ingress.IP
	if ip == "" {
		// Some cloud providers use hostname instead of IP
		ip = ingress.Hostname
	}

	if ip == "" {
		return "", fmt.Errorf("service %s/%s has no IP or hostname", namespace, serviceName)
	}

	return ip, nil
}

// CreateCertificate creates a Certificate resource for a tenant (default domain)
func (c *Client) CreateCertificate(ctx context.Context, slug, adminHost, storefrontHost string) error {
	return c.createCertificateInNamespace(ctx, slug, []string{adminHost, storefrontHost}, c.config.Kubernetes.Namespace, c.config.Kubernetes.ClusterIssuer)
}

// CreateCustomDomainCertificate creates a Certificate resource for a custom domain tenant
// Certificates are created in the custom domain gateway namespace (istio-ingress)
// and use the HTTP-01 ClusterIssuer for Let's Encrypt validation
func (c *Client) CreateCustomDomainCertificate(ctx context.Context, slug string, domains []string) error {
	namespace := c.config.Kubernetes.CustomDomainGatewayNS
	issuer := c.config.Kubernetes.CustomDomainClusterIssuer

	log.Printf("[K8s] Creating custom domain certificate for %s in namespace %s with issuer %s", slug, namespace, issuer)
	return c.createCertificateInNamespace(ctx, slug, domains, namespace, issuer)
}

// createCertificateInNamespace creates a Certificate resource in a specific namespace
func (c *Client) createCertificateInNamespace(ctx context.Context, slug string, domains []string, namespace, clusterIssuer string) error {
	certName := fmt.Sprintf("%s-tenant-tls", slug)

	// Check if certificate already exists
	_, err := c.certmanager.CertmanagerV1().Certificates(namespace).Get(ctx, certName, metav1.GetOptions{})
	if err == nil {
		log.Printf("[K8s] Certificate %s already exists in %s, skipping creation", certName, namespace)
		return nil
	}

	// Create Certificate resource
	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "tenant-router-service",
				"tenant-slug":                  slug,
			},
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: certName,
			DNSNames:   domains,
			IssuerRef: cmmeta.ObjectReference{
				Name: clusterIssuer,
				Kind: "ClusterIssuer",
			},
		},
	}

	_, err = c.certmanager.CertmanagerV1().Certificates(namespace).Create(ctx, cert, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	log.Printf("[K8s] Created Certificate %s for domains: %v in namespace %s", certName, domains, namespace)
	return nil
}

// DeleteCertificate deletes a Certificate resource for a tenant
func (c *Client) DeleteCertificate(ctx context.Context, slug string) error {
	certName := fmt.Sprintf("%s-tenant-tls", slug)
	namespace := c.config.Kubernetes.Namespace

	err := c.certmanager.CertmanagerV1().Certificates(namespace).Delete(ctx, certName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete certificate: %w", err)
	}

	log.Printf("[K8s] Deleted Certificate %s", certName)
	return nil
}

// GatewayServerPatch represents a patch operation for Gateway servers
type GatewayServerPatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// PatchGatewayServer adds or removes a server entry from the Gateway
// It automatically discovers the Gateway namespace if not in the configured Istio namespace
// Returns the namespace where the Gateway was found
func (c *Client) PatchGatewayServer(ctx context.Context, slug, adminHost, storefrontHost, operation string) (string, error) {
	gatewayName := c.config.Kubernetes.GatewayName
	certName := fmt.Sprintf("%s-tenant-tls", slug)
	certSecretName := fmt.Sprintf("%s/%s", c.config.Kubernetes.Namespace, certName)

	// Find the Gateway across namespaces
	namespace, err := c.FindGatewayByName(ctx, gatewayName)
	if err != nil {
		return "", err
	}

	// Get current Gateway
	gateway, err := c.istio.NetworkingV1beta1().Gateways(namespace).Get(ctx, gatewayName, metav1.GetOptions{})
	if err != nil {
		return namespace, fmt.Errorf("failed to get gateway: %w", err)
	}

	if operation == "add" {
		// Check if servers already exist
		adminExists := false
		storefrontExists := false
		for _, server := range gateway.Spec.Servers {
			for _, host := range server.Hosts {
				if host == adminHost {
					adminExists = true
				}
				if host == storefrontHost {
					storefrontExists = true
				}
			}
		}

		// Add admin server if not exists
		if !adminExists {
			adminServer := &networkingv1beta1.Server{
				Port: &networkingv1beta1.Port{
					Number:   443,
					Name:     fmt.Sprintf("https-%s-admin", slug),
					Protocol: "HTTPS",
				},
				Hosts: []string{adminHost},
				Tls: &networkingv1beta1.ServerTLSSettings{
					Mode:           networkingv1beta1.ServerTLSSettings_SIMPLE,
					CredentialName: certSecretName,
				},
			}
			gateway.Spec.Servers = append(gateway.Spec.Servers, adminServer)
		}

		// Add storefront server if not exists
		if !storefrontExists {
			storefrontServer := &networkingv1beta1.Server{
				Port: &networkingv1beta1.Port{
					Number:   443,
					Name:     fmt.Sprintf("https-%s-store", slug),
					Protocol: "HTTPS",
				},
				Hosts: []string{storefrontHost},
				Tls: &networkingv1beta1.ServerTLSSettings{
					Mode:           networkingv1beta1.ServerTLSSettings_SIMPLE,
					CredentialName: certSecretName,
				},
			}
			gateway.Spec.Servers = append(gateway.Spec.Servers, storefrontServer)
		}
	} else if operation == "remove" {
		// Remove servers for this tenant
		var filteredServers []*networkingv1beta1.Server
		for _, server := range gateway.Spec.Servers {
			keep := true
			for _, host := range server.Hosts {
				if host == adminHost || host == storefrontHost {
					keep = false
					break
				}
			}
			if keep {
				filteredServers = append(filteredServers, server)
			}
		}
		gateway.Spec.Servers = filteredServers
	}

	// Update the Gateway
	_, err = c.istio.NetworkingV1beta1().Gateways(namespace).Update(ctx, gateway, metav1.UpdateOptions{})
	if err != nil {
		return namespace, fmt.Errorf("failed to update gateway: %w", err)
	}

	log.Printf("[K8s] Updated Gateway %s (%s servers for %s)", gatewayName, operation, slug)
	return namespace, nil
}

// VSHostsPatch represents a JSON patch for VirtualService hosts
type VSHostsPatch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value,omitempty"`
}

// VirtualServiceLocation represents a found VirtualService with its namespace
type VirtualServiceLocation struct {
	Name      string
	Namespace string
}

// FindVirtualServiceByName searches for a VirtualService by name across all namespaces
// Returns the namespace where the VirtualService was found
// Uses caching to avoid repeated lookups for the same VirtualService
func (c *Client) FindVirtualServiceByName(ctx context.Context, vsName string) (*VirtualServiceLocation, error) {
	// Check cache first
	if cachedNS, ok := c.vsNamespaceCache[vsName]; ok {
		log.Printf("[K8s] Found VirtualService %s in cache (namespace: %s)", vsName, cachedNS)
		return &VirtualServiceLocation{Name: vsName, Namespace: cachedNS}, nil
	}

	// First, try the configured namespace (fast path)
	configuredNS := c.config.Kubernetes.Namespace
	vs, err := c.istio.NetworkingV1beta1().VirtualServices(configuredNS).Get(ctx, vsName, metav1.GetOptions{})
	if err == nil {
		log.Printf("[K8s] Found VirtualService %s in configured namespace %s", vsName, configuredNS)
		c.vsNamespaceCache[vsName] = configuredNS
		return &VirtualServiceLocation{Name: vs.Name, Namespace: configuredNS}, nil
	}

	// If not found in configured namespace, search all namespaces
	log.Printf("[K8s] VirtualService %s not in %s, searching all namespaces...", vsName, configuredNS)

	// List all VirtualServices across all namespaces (empty string = all namespaces)
	vsList, err := c.istio.NetworkingV1beta1().VirtualServices("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list VirtualServices across namespaces: %w", err)
	}

	for _, vs := range vsList.Items {
		if vs.Name == vsName {
			log.Printf("[K8s] Found VirtualService %s in namespace %s", vsName, vs.Namespace)
			c.vsNamespaceCache[vsName] = vs.Namespace
			return &VirtualServiceLocation{Name: vs.Name, Namespace: vs.Namespace}, nil
		}
	}

	return nil, fmt.Errorf("virtualService %s not found in any namespace", vsName)
}

// FindGatewayByName searches for a Gateway by name across all namespaces
// Returns the namespace where the Gateway was found
func (c *Client) FindGatewayByName(ctx context.Context, gwName string) (string, error) {
	// Check cache first
	if cachedNS, ok := c.gwNamespaceCache[gwName]; ok {
		log.Printf("[K8s] Found Gateway %s in cache (namespace: %s)", gwName, cachedNS)
		return cachedNS, nil
	}

	// First, try the Istio namespace (fast path)
	istioNS := c.config.Kubernetes.IstioNamespace
	_, err := c.istio.NetworkingV1beta1().Gateways(istioNS).Get(ctx, gwName, metav1.GetOptions{})
	if err == nil {
		log.Printf("[K8s] Found Gateway %s in Istio namespace %s", gwName, istioNS)
		c.gwNamespaceCache[gwName] = istioNS
		return istioNS, nil
	}

	// Try the configured namespace
	configuredNS := c.config.Kubernetes.Namespace
	if configuredNS != istioNS {
		_, err := c.istio.NetworkingV1beta1().Gateways(configuredNS).Get(ctx, gwName, metav1.GetOptions{})
		if err == nil {
			log.Printf("[K8s] Found Gateway %s in configured namespace %s", gwName, configuredNS)
			c.gwNamespaceCache[gwName] = configuredNS
			return configuredNS, nil
		}
	}

	// If not found, search all namespaces
	log.Printf("[K8s] Gateway %s not in %s or %s, searching all namespaces...", gwName, istioNS, configuredNS)

	gwList, err := c.istio.NetworkingV1beta1().Gateways("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list Gateways across namespaces: %w", err)
	}

	for _, gw := range gwList.Items {
		if gw.Name == gwName {
			log.Printf("[K8s] Found Gateway %s in namespace %s", gwName, gw.Namespace)
			c.gwNamespaceCache[gwName] = gw.Namespace
			return gw.Namespace, nil
		}
	}

	return "", fmt.Errorf("gateway %s not found in any namespace", gwName)
}

// CreateTenantVirtualService creates a new VirtualService for a tenant by copying the template
// This is the preferred approach for multi-tenant isolation - each tenant gets their own VS
// The template VS should have a placeholder host (e.g., "template-admin.internal") that won't receive traffic
//
// Production considerations:
// - Deep copies all routes, timeouts, retries, and other configurations from template
// - Updates the host to the tenant-specific hostname
// - Injects X-Vendor-ID header with tenant UUID for backend service identification
// - Replaces CORS origins with tenant-specific domains for security isolation
// - Adds shared domains (onboarding) that may be needed for cross-origin flows
// - Preserves all route matching rules, destinations, and service configurations
func (c *Client) CreateTenantVirtualService(ctx context.Context, slug, tenantID, templateVSName, tenantHost, adminHost, storefrontHost string, cloudflareProxied bool) error {
	return c.CreateTenantVirtualServiceWithSuffix(ctx, slug, tenantID, templateVSName, tenantHost, adminHost, storefrontHost, "", cloudflareProxied)
}

// CreateCustomDomainVirtualService creates a VirtualService for a custom domain tenant
// Custom domain VirtualServices reference the custom-domain-gateway instead of the default gateway
// This allows custom domains to use a dedicated LoadBalancer with direct access (no Cloudflare)
func (c *Client) CreateCustomDomainVirtualService(ctx context.Context, slug, tenantID, templateVSName, tenantHost, adminHost, storefrontHost, nameSuffix string) error {
	return c.createVirtualServiceWithGateway(ctx, slug, tenantID, templateVSName, tenantHost, adminHost, storefrontHost, nameSuffix, false, true)
}

// CreateTenantVirtualServiceWithSuffix creates a new VirtualService for a tenant with an optional name suffix
// The suffix is used when creating multiple VirtualServices from the same template (e.g., storefront + www)
// cloudflareProxied controls the external-dns.alpha.kubernetes.io/cloudflare-proxied annotation:
// - true: DNS record will be proxied through Cloudflare (orange cloud) - use for default domain tenants
// - false: DNS record will be DNS-only (gray cloud) - use for custom domain tenants' platform subdomains (CNAME targets)
func (c *Client) CreateTenantVirtualServiceWithSuffix(ctx context.Context, slug, tenantID, templateVSName, tenantHost, adminHost, storefrontHost, nameSuffix string, cloudflareProxied bool) error {
	return c.createVirtualServiceWithGateway(ctx, slug, tenantID, templateVSName, tenantHost, adminHost, storefrontHost, nameSuffix, cloudflareProxied, false)
}

// createVirtualServiceWithGateway is the internal implementation for creating VirtualServices
// isCustomDomain controls which gateway the VirtualService references:
// - false: Uses the default gateway (for platform domains via Cloudflare Tunnel)
// - true: Uses the custom-domain-gateway (for custom domains via direct LoadBalancer)
func (c *Client) createVirtualServiceWithGateway(ctx context.Context, slug, tenantID, templateVSName, tenantHost, adminHost, storefrontHost, nameSuffix string, cloudflareProxied, isCustomDomain bool) error {
	// Find the template VirtualService
	vsLocation, err := c.FindVirtualServiceByName(ctx, templateVSName)
	if err != nil {
		return fmt.Errorf("failed to find template VirtualService %s: %w", templateVSName, err)
	}

	// Get the template VirtualService
	templateVS, err := c.istio.NetworkingV1beta1().VirtualServices(vsLocation.Namespace).Get(ctx, templateVSName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get template VirtualService %s: %w", templateVSName, err)
	}

	// Determine the new VS name based on template type
	var newVSName string
	switch templateVSName {
	case c.config.Kubernetes.AdminVSName:
		newVSName = fmt.Sprintf("%s-admin-vs", slug)
	case c.config.Kubernetes.StorefrontVSName:
		newVSName = fmt.Sprintf("%s-storefront-vs", slug)
	case c.config.Kubernetes.APIVSName:
		newVSName = fmt.Sprintf("%s-api-vs", slug)
	default:
		newVSName = fmt.Sprintf("%s-storefront-vs", slug)
	}

	// Apply suffix if provided (e.g., for www subdomain: custom-store-storefront-www-vs)
	if nameSuffix != "" {
		// Insert suffix before "-vs"
		newVSName = fmt.Sprintf("%s-%s-vs", newVSName[:len(newVSName)-3], nameSuffix)
	}

	// Check if VS already exists (idempotent operation)
	_, err = c.istio.NetworkingV1beta1().VirtualServices(vsLocation.Namespace).Get(ctx, newVSName, metav1.GetOptions{})
	if err == nil {
		log.Printf("[K8s] VirtualService %s already exists, skipping creation", newVSName)
		return nil
	}

	// Create a new VirtualService by deep copying the template
	// This preserves all routes, timeouts, retries, destinations, and other configurations
	newVS := templateVS.DeepCopy()

	// Determine cloudflare-proxied annotation value
	// - "true": DNS will be proxied through Cloudflare (DDoS protection, WAF)
	// - "false": DNS-only mode, allows external domains to CNAME to this host
	cloudflareProxiedValue := "false"
	if cloudflareProxied {
		cloudflareProxiedValue = "true"
	}

	newVS.ObjectMeta = metav1.ObjectMeta{
		Name:      newVSName,
		Namespace: vsLocation.Namespace,
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "tenant-router-service",
			"app.kubernetes.io/component":  "virtualservice",
			"tenant-slug":                  slug,
		},
		Annotations: map[string]string{
			"tenant-router-service/created-from":                templateVSName,
			"tenant-router-service/tenant-slug":                 slug,
			"tenant-router-service/admin-host":                  adminHost,
			"tenant-router-service/storefront-host":             storefrontHost,
			"external-dns.alpha.kubernetes.io/cloudflare-proxied": cloudflareProxiedValue,
		},
	}

	// Update the hosts to use the tenant's specific hostname
	// This ensures this VirtualService only handles traffic for this tenant
	newVS.Spec.Hosts = []string{tenantHost}

	// Update the gateway reference based on whether this is a custom domain
	// Custom domains use a dedicated gateway with LoadBalancer for direct access
	// Default domains use the platform gateway (via Cloudflare Tunnel)
	if isCustomDomain {
		customGatewayRef := fmt.Sprintf("%s/%s", c.config.Kubernetes.CustomDomainGatewayNS, c.config.Kubernetes.CustomDomainGateway)
		newVS.Spec.Gateways = []string{customGatewayRef}
		log.Printf("[K8s] Using custom domain gateway %s for tenant %s", customGatewayRef, slug)
	}

	// Build CORS origins for tenant isolation
	// Include tenant-specific domains + shared onboarding domain for cross-origin flows
	baseDomain := c.config.Domain.BaseDomain
	onboardingHost := fmt.Sprintf("dev-onboarding.%s", baseDomain) // Shared onboarding domain

	tenantOrigins := []*networkingv1beta1.StringMatch{
		// Tenant's own domains (required for admin<->storefront communication)
		{MatchType: &networkingv1beta1.StringMatch_Exact{Exact: fmt.Sprintf("https://%s", adminHost)}},
		{MatchType: &networkingv1beta1.StringMatch_Exact{Exact: fmt.Sprintf("https://%s", storefrontHost)}},
		// Shared onboarding domain (for tenant setup flows)
		{MatchType: &networkingv1beta1.StringMatch_Exact{Exact: fmt.Sprintf("https://%s", onboardingHost)}},
	}

	// Update CORS policy in all HTTP routes that have CORS configured
	// This ensures proper cross-origin request handling for the tenant
	corsUpdated := false
	for i, httpRoute := range newVS.Spec.Http {
		if httpRoute.CorsPolicy != nil {
			// Replace CORS origins with tenant-specific origins
			// This provides security isolation - tenant A's admin cannot make CORS requests to tenant B's API
			newVS.Spec.Http[i].CorsPolicy.AllowOrigins = tenantOrigins
			corsUpdated = true
		}
	}

	if corsUpdated {
		log.Printf("[K8s] Updated CORS policy for tenant %s: admin=%s, storefront=%s, onboarding=%s",
			slug, adminHost, storefrontHost, onboardingHost)
	}

	// Inject X-Vendor-ID header with tenant UUID for all routes
	// This allows backend services to identify the tenant without the client needing to pass the UUID
	if tenantID != "" {
		for i := range newVS.Spec.Http {
			// Initialize headers if nil
			if newVS.Spec.Http[i].Headers == nil {
				newVS.Spec.Http[i].Headers = &networkingv1beta1.Headers{}
			}
			if newVS.Spec.Http[i].Headers.Request == nil {
				newVS.Spec.Http[i].Headers.Request = &networkingv1beta1.Headers_HeaderOperations{}
			}
			if newVS.Spec.Http[i].Headers.Request.Set == nil {
				newVS.Spec.Http[i].Headers.Request.Set = make(map[string]string)
			}
			// Set the X-Vendor-ID header to the tenant UUID
			newVS.Spec.Http[i].Headers.Request.Set["X-Vendor-ID"] = tenantID
			// Also set X-Tenant-ID for backwards compatibility
			newVS.Spec.Http[i].Headers.Request.Set["X-Tenant-ID"] = tenantID
		}
		log.Printf("[K8s] Injected X-Vendor-ID header for tenant %s (UUID: %s)", slug, tenantID)
	}

	// Create the new VirtualService
	_, err = c.istio.NetworkingV1beta1().VirtualServices(vsLocation.Namespace).Create(ctx, newVS, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create VirtualService %s: %w", newVSName, err)
	}

	log.Printf("[K8s] Created VirtualService %s for tenant %s with host %s (copied from template %s)",
		newVSName, slug, tenantHost, templateVSName)
	return nil
}

// UpdateTenantVirtualService updates an existing tenant VirtualService with routes from the template
// This allows syncing new routes (like swagger-docs) to existing tenant VSes
// It preserves the tenant-specific hosts and CORS configuration while updating routes
func (c *Client) UpdateTenantVirtualService(ctx context.Context, slug, templateVSName, tenantHost, adminHost, storefrontHost string) error {
	// Find the template VirtualService
	vsLocation, err := c.FindVirtualServiceByName(ctx, templateVSName)
	if err != nil {
		return fmt.Errorf("failed to find template VirtualService %s: %w", templateVSName, err)
	}

	// Get the template VirtualService
	templateVS, err := c.istio.NetworkingV1beta1().VirtualServices(vsLocation.Namespace).Get(ctx, templateVSName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get template VirtualService %s: %w", templateVSName, err)
	}

	// Determine the tenant VS name based on template type
	var tenantVSName string
	switch templateVSName {
	case c.config.Kubernetes.AdminVSName:
		tenantVSName = fmt.Sprintf("%s-admin-vs", slug)
	case c.config.Kubernetes.StorefrontVSName:
		tenantVSName = fmt.Sprintf("%s-storefront-vs", slug)
	case c.config.Kubernetes.APIVSName:
		tenantVSName = fmt.Sprintf("%s-api-vs", slug)
	default:
		tenantVSName = fmt.Sprintf("%s-storefront-vs", slug)
	}

	// Get the existing tenant VirtualService
	tenantVS, err := c.istio.NetworkingV1beta1().VirtualServices(vsLocation.Namespace).Get(ctx, tenantVSName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get tenant VirtualService %s: %w", tenantVSName, err)
	}

	// Build tenant-specific CORS origins
	baseDomain := c.config.Domain.BaseDomain
	onboardingHost := fmt.Sprintf("dev-onboarding.%s", baseDomain)

	tenantOrigins := []*networkingv1beta1.StringMatch{
		{MatchType: &networkingv1beta1.StringMatch_Exact{Exact: fmt.Sprintf("https://%s", adminHost)}},
		{MatchType: &networkingv1beta1.StringMatch_Exact{Exact: fmt.Sprintf("https://%s", storefrontHost)}},
		{MatchType: &networkingv1beta1.StringMatch_Exact{Exact: fmt.Sprintf("https://%s", onboardingHost)}},
	}

	// Deep copy the template's HTTP routes
	updatedRoutes := make([]*networkingv1beta1.HTTPRoute, len(templateVS.Spec.Http))
	for i, route := range templateVS.Spec.Http {
		// Deep copy each route
		routeCopy := &networkingv1beta1.HTTPRoute{
			Name:      route.Name,
			Match:     route.Match,
			Route:     route.Route,
			Redirect:  route.Redirect,
			Rewrite:   route.Rewrite,
			Timeout:   route.Timeout,
			Retries:   route.Retries,
			Fault:     route.Fault,
			Mirror:    route.Mirror,
			Headers:   route.Headers,
		}
		// Apply tenant-specific CORS if the route has CORS configured
		if route.CorsPolicy != nil {
			routeCopy.CorsPolicy = &networkingv1beta1.CorsPolicy{
				AllowOrigins:     tenantOrigins,
				AllowMethods:     route.CorsPolicy.AllowMethods,
				AllowHeaders:     route.CorsPolicy.AllowHeaders,
				ExposeHeaders:    route.CorsPolicy.ExposeHeaders,
				MaxAge:           route.CorsPolicy.MaxAge,
				AllowCredentials: route.CorsPolicy.AllowCredentials,
			}
		}
		updatedRoutes[i] = routeCopy
	}

	// Update the tenant VS with new routes (preserving hosts)
	tenantVS.Spec.Http = updatedRoutes

	// Update the VirtualService
	_, err = c.istio.NetworkingV1beta1().VirtualServices(vsLocation.Namespace).Update(ctx, tenantVS, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update VirtualService %s: %w", tenantVSName, err)
	}

	log.Printf("[K8s] Updated VirtualService %s for tenant %s (synced %d routes from template %s)",
		tenantVSName, slug, len(updatedRoutes), templateVSName)
	return nil
}

// SyncTenantVirtualServices syncs all tenant VirtualServices for a given template
// This is useful when the template is updated with new routes
func (c *Client) SyncTenantVirtualServices(ctx context.Context, templateVSName string) (int, error) {
	namespace := c.config.Kubernetes.Namespace

	// List all tenant VirtualServices managed by this service
	vsList, err := c.istio.NetworkingV1beta1().VirtualServices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=tenant-router-service",
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list VirtualServices: %w", err)
	}

	syncedCount := 0
	for _, vs := range vsList.Items {
		// Check if this VS was created from the specified template
		createdFrom, ok := vs.Annotations["tenant-router-service/created-from"]
		if !ok || createdFrom != templateVSName {
			continue
		}

		slug := vs.Labels["tenant-slug"]
		if slug == "" {
			log.Printf("[K8s] Skipping VS %s: no tenant-slug label", vs.Name)
			continue
		}

		// Get tenant hosts from annotations
		adminHost := vs.Annotations["tenant-router-service/admin-host"]
		storefrontHost := vs.Annotations["tenant-router-service/storefront-host"]

		// Determine tenant host based on template type
		var tenantHost string
		baseDomain := c.config.Domain.BaseDomain
		switch templateVSName {
		case c.config.Kubernetes.AdminVSName:
			tenantHost = fmt.Sprintf("%s-admin.%s", slug, baseDomain)
		case c.config.Kubernetes.StorefrontVSName:
			tenantHost = fmt.Sprintf("%s.%s", slug, baseDomain)
		case c.config.Kubernetes.APIVSName:
			tenantHost = fmt.Sprintf("%s-api.%s", slug, baseDomain)
		default:
			tenantHost = fmt.Sprintf("%s.%s", slug, baseDomain)
		}

		err := c.UpdateTenantVirtualService(ctx, slug, templateVSName, tenantHost, adminHost, storefrontHost)
		if err != nil {
			log.Printf("[K8s] Failed to sync VS for tenant %s: %v", slug, err)
			continue
		}
		syncedCount++
	}

	log.Printf("[K8s] Synced %d VirtualServices from template %s", syncedCount, templateVSName)
	return syncedCount, nil
}

// VirtualServiceExists checks if a VirtualService exists in the configured namespace
func (c *Client) VirtualServiceExists(ctx context.Context, vsName string) bool {
	namespace := c.config.Kubernetes.Namespace
	_, err := c.istio.NetworkingV1beta1().VirtualServices(namespace).Get(ctx, vsName, metav1.GetOptions{})
	return err == nil
}

// DeleteTenantVirtualService deletes a tenant's VirtualService
func (c *Client) DeleteTenantVirtualService(ctx context.Context, slug, templateVSName string) error {
	// Determine the VS name based on template type
	var vsName string
	switch templateVSName {
	case c.config.Kubernetes.AdminVSName:
		vsName = fmt.Sprintf("%s-admin-vs", slug)
	case c.config.Kubernetes.StorefrontVSName:
		vsName = fmt.Sprintf("%s-storefront-vs", slug)
	case c.config.Kubernetes.APIVSName:
		vsName = fmt.Sprintf("%s-api-vs", slug)
	default:
		vsName = fmt.Sprintf("%s-storefront-vs", slug)
	}

	namespace := c.config.Kubernetes.Namespace

	err := c.istio.NetworkingV1beta1().VirtualServices(namespace).Delete(ctx, vsName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete VirtualService %s: %w", vsName, err)
	}

	log.Printf("[K8s] Deleted VirtualService %s for tenant %s", vsName, slug)
	return nil
}

// PatchVirtualServiceHosts adds or removes a host from a VirtualService (legacy method)
// Deprecated: Use CreateTenantVirtualService instead for better multi-tenant isolation
func (c *Client) PatchVirtualServiceHosts(ctx context.Context, vsName, host, operation string) error {
	// Find the VirtualService across namespaces
	vsLocation, err := c.FindVirtualServiceByName(ctx, vsName)
	if err != nil {
		return err
	}
	namespace := vsLocation.Namespace

	// Get current VirtualService
	vs, err := c.istio.NetworkingV1beta1().VirtualServices(namespace).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get VirtualService %s: %w", vsName, err)
	}

	if operation == "add" {
		// Check if host already exists
		for _, existingHost := range vs.Spec.Hosts {
			if existingHost == host {
				log.Printf("[K8s] Host %s already exists in VirtualService %s", host, vsName)
				return nil
			}
		}

		// Use JSON Patch to add host
		patch := []VSHostsPatch{
			{
				Op:    "add",
				Path:  "/spec/hosts/-",
				Value: host,
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("failed to marshal patch: %w", err)
		}

		_, err = c.istio.NetworkingV1beta1().VirtualServices(namespace).Patch(
			ctx, vsName, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch VirtualService: %w", err)
		}
	} else if operation == "remove" {
		// Find and remove the host
		hostIndex := -1
		for i, existingHost := range vs.Spec.Hosts {
			if existingHost == host {
				hostIndex = i
				break
			}
		}

		if hostIndex == -1 {
			log.Printf("[K8s] Host %s not found in VirtualService %s", host, vsName)
			return nil
		}

		// Use JSON Patch to remove host
		patch := []VSHostsPatch{
			{
				Op:   "remove",
				Path: fmt.Sprintf("/spec/hosts/%d", hostIndex),
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("failed to marshal patch: %w", err)
		}

		_, err = c.istio.NetworkingV1beta1().VirtualServices(namespace).Patch(
			ctx, vsName, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			return fmt.Errorf("failed to patch VirtualService: %w", err)
		}
	}

	log.Printf("[K8s] Patched VirtualService %s: %s host %s", vsName, operation, host)
	return nil
}

// GetVirtualServiceHosts returns all hosts for a VirtualService
// It automatically discovers the VirtualService namespace
func (c *Client) GetVirtualServiceHosts(ctx context.Context, vsName string) ([]string, error) {
	// Find the VirtualService across namespaces
	vsLocation, err := c.FindVirtualServiceByName(ctx, vsName)
	if err != nil {
		return nil, err
	}

	vs, err := c.istio.NetworkingV1beta1().VirtualServices(vsLocation.Namespace).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get VirtualService %s: %w", vsName, err)
	}

	return vs.Spec.Hosts, nil
}

// ListCertificates lists all tenant certificates
func (c *Client) ListCertificates(ctx context.Context) ([]certmanagerv1.Certificate, error) {
	namespace := c.config.Kubernetes.Namespace

	list, err := c.certmanager.CertmanagerV1().Certificates(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=tenant-router-service",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list certificates: %w", err)
	}

	return list.Items, nil
}

// GetCertificateStatus returns the status of a tenant certificate
func (c *Client) GetCertificateStatus(ctx context.Context, slug string) (string, error) {
	certName := fmt.Sprintf("%s-tenant-tls", slug)
	namespace := c.config.Kubernetes.Namespace

	cert, err := c.certmanager.CertmanagerV1().Certificates(namespace).Get(ctx, certName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get certificate: %w", err)
	}

	for _, condition := range cert.Status.Conditions {
		if condition.Type == certmanagerv1.CertificateConditionReady {
			if condition.Status == cmmeta.ConditionTrue {
				return "ready", nil
			}
			return "pending", nil
		}
	}

	return "unknown", nil
}

// IsConnected checks if the Kubernetes connection is healthy
func (c *Client) IsConnected(ctx context.Context) bool {
	_, err := c.istio.NetworkingV1beta1().VirtualServices(c.config.Kubernetes.Namespace).List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// AddHostsToSharedAuthPolicy adds custom domain hosts to the tenant-router-managed AuthorizationPolicy
// This creates a SEPARATE policy from the ArgoCD-managed one to avoid sync conflicts
// The policy is created if it doesn't exist, and hosts are added to it dynamically
func (c *Client) AddHostsToSharedAuthPolicy(ctx context.Context, hosts []string) error {
	// Use a dedicated policy name for tenant-router-service managed hosts
	// This is separate from the ArgoCD-managed allow-frontend-apps-public-custom
	policyName := "tenant-router-custom-domain-hosts"
	namespace := c.config.Kubernetes.SharedAuthPolicyNamespace
	selector := c.config.Kubernetes.SharedAuthPolicySelector

	if namespace == "" {
		namespace = "istio-ingress"
	}
	if selector == "" {
		selector = "custom-ingressgateway"
	}

	// Try to get the existing policy
	policy, err := c.istio.SecurityV1beta1().AuthorizationPolicies(namespace).Get(ctx, policyName, metav1.GetOptions{})
	if err != nil {
		// Policy doesn't exist - create it
		log.Printf("[K8s] Creating new AuthorizationPolicy %s/%s for custom domain hosts", namespace, policyName)
		newPolicy := &istiosecurityv1beta1.AuthorizationPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyName,
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "tenant-router-service",
					"app.kubernetes.io/component":  "custom-domain-access",
				},
			},
		}
		newPolicy.Spec.Selector = &typev1beta1.WorkloadSelector{
			MatchLabels: map[string]string{
				"istio": selector,
			},
		}
		newPolicy.Spec.Action = securityv1beta1.AuthorizationPolicy_ALLOW
		newPolicy.Spec.Rules = []*securityv1beta1.Rule{
			{
				To: []*securityv1beta1.Rule_To{
					{
						Operation: &securityv1beta1.Operation{
							Hosts: hosts,
						},
					},
				},
			},
		}

		_, err = c.istio.SecurityV1beta1().AuthorizationPolicies(namespace).Create(ctx, newPolicy, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create AuthorizationPolicy %s/%s: %w", namespace, policyName, err)
		}
		log.Printf("[K8s] Created AuthorizationPolicy %s/%s with hosts: %v", namespace, policyName, hosts)
		return nil
	}

	// Policy exists - check which hosts need to be added
	hostsToAdd := make([]string, 0)
	existingHosts := make(map[string]bool)

	// Collect all existing hosts from all rules
	for _, rule := range policy.Spec.Rules {
		if rule.To != nil {
			for _, to := range rule.To {
				if to.Operation != nil {
					for _, host := range to.Operation.Hosts {
						existingHosts[host] = true
					}
				}
			}
		}
	}

	// Check which hosts need to be added
	for _, host := range hosts {
		if !existingHosts[host] {
			hostsToAdd = append(hostsToAdd, host)
		}
	}

	if len(hostsToAdd) == 0 {
		log.Printf("[K8s] All hosts already in AuthorizationPolicy %s/%s, no update needed", namespace, policyName)
		return nil
	}

	// Add hosts to the first rule's operation
	if len(policy.Spec.Rules) > 0 && policy.Spec.Rules[0].To != nil && len(policy.Spec.Rules[0].To) > 0 {
		if policy.Spec.Rules[0].To[0].Operation != nil {
			policy.Spec.Rules[0].To[0].Operation.Hosts = append(
				policy.Spec.Rules[0].To[0].Operation.Hosts,
				hostsToAdd...,
			)
		}
	}

	// Update the AuthorizationPolicy
	_, err = c.istio.SecurityV1beta1().AuthorizationPolicies(namespace).Update(ctx, policy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update AuthorizationPolicy %s/%s: %w", namespace, policyName, err)
	}

	log.Printf("[K8s] Added hosts to AuthorizationPolicy %s/%s: %v", namespace, policyName, hostsToAdd)
	return nil
}

// RemoveHostsFromSharedAuthPolicy removes custom domain hosts from the tenant-router-managed AuthorizationPolicy
// This is called when a custom domain tenant is deleted to clean up the RBAC rules
func (c *Client) RemoveHostsFromSharedAuthPolicy(ctx context.Context, hosts []string) error {
	// Use the same dedicated policy name as AddHostsToSharedAuthPolicy
	policyName := "tenant-router-custom-domain-hosts"
	namespace := c.config.Kubernetes.SharedAuthPolicyNamespace

	if namespace == "" {
		namespace = "istio-ingress"
	}

	// Get the existing AuthorizationPolicy
	policy, err := c.istio.SecurityV1beta1().AuthorizationPolicies(namespace).Get(ctx, policyName, metav1.GetOptions{})
	if err != nil {
		// Policy doesn't exist - nothing to remove
		log.Printf("[K8s] AuthorizationPolicy %s/%s not found, nothing to remove", namespace, policyName)
		return nil
	}

	// Create a set of hosts to remove for efficient lookup
	hostsToRemove := make(map[string]bool)
	for _, h := range hosts {
		hostsToRemove[h] = true
	}

	// Remove hosts from all rules
	removedCount := 0
	for i, rule := range policy.Spec.Rules {
		if rule.To != nil {
			for j, to := range rule.To {
				if to.Operation != nil && len(to.Operation.Hosts) > 0 {
					// Filter out hosts that should be removed
					filteredHosts := make([]string, 0, len(to.Operation.Hosts))
					for _, h := range to.Operation.Hosts {
						if !hostsToRemove[h] {
							filteredHosts = append(filteredHosts, h)
						} else {
							removedCount++
						}
					}
					policy.Spec.Rules[i].To[j].Operation.Hosts = filteredHosts
				}
			}
		}
	}

	if removedCount == 0 {
		log.Printf("[K8s] No hosts to remove from AuthorizationPolicy %s/%s", namespace, policyName)
		return nil
	}

	// Update the AuthorizationPolicy
	_, err = c.istio.SecurityV1beta1().AuthorizationPolicies(namespace).Update(ctx, policy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update AuthorizationPolicy %s/%s: %w", namespace, policyName, err)
	}

	log.Printf("[K8s] Removed %d hosts from AuthorizationPolicy %s/%s: %v", removedCount, namespace, policyName, hosts)
	return nil
}

// EnsureSharedAuthPolicy ensures the shared AuthorizationPolicy exists
// If it doesn't exist, this method creates it with the configured selector
// This is called during reconciliation to ensure the policy is ready before adding hosts
func (c *Client) EnsureSharedAuthPolicy(ctx context.Context) error {
	policyName := c.config.Kubernetes.SharedAuthPolicyName
	namespace := c.config.Kubernetes.SharedAuthPolicyNamespace
	selector := c.config.Kubernetes.SharedAuthPolicySelector

	if policyName == "" {
		log.Println("[K8s] SharedAuthPolicyName not configured, skipping AuthorizationPolicy check")
		return nil
	}

	// Check if policy already exists
	_, err := c.istio.SecurityV1beta1().AuthorizationPolicies(namespace).Get(ctx, policyName, metav1.GetOptions{})
	if err == nil {
		log.Printf("[K8s] AuthorizationPolicy %s/%s already exists", namespace, policyName)
		return nil
	}

	// Create the policy if it doesn't exist
	policy := &istiosecurityv1beta1.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "tenant-router-service",
			},
		},
		Spec: securityv1beta1.AuthorizationPolicy{
			Selector: &typev1beta1.WorkloadSelector{
				MatchLabels: map[string]string{
					"istio": selector,
				},
			},
			Action: securityv1beta1.AuthorizationPolicy_ALLOW,
			Rules: []*securityv1beta1.Rule{
				{
					To: []*securityv1beta1.Rule_To{
						{
							Operation: &securityv1beta1.Operation{
								Hosts: []string{}, // Will be populated as custom domains are added
							},
						},
					},
				},
			},
		},
	}

	_, err = c.istio.SecurityV1beta1().AuthorizationPolicies(namespace).Create(ctx, policy, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create AuthorizationPolicy %s/%s: %w", namespace, policyName, err)
	}

	log.Printf("[K8s] Created AuthorizationPolicy %s/%s with selector istio=%s", namespace, policyName, selector)
	return nil
}
