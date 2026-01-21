package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"custom-domain-service/internal/config"
	"custom-domain-service/internal/models"

	"github.com/rs/zerolog/log"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certmanagerclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	networkingv1beta1 "istio.io/api/networking/v1beta1"
	securityv1beta1 "istio.io/api/security/v1beta1"
	typev1beta1 "istio.io/api/type/v1beta1"
	"istio.io/client-go/pkg/apis/networking/v1beta1"
	securityclientv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubernetesClient handles Kubernetes operations for cert-manager and Istio
type KubernetesClient struct {
	cfg              *config.Config
	kubeClient       *kubernetes.Clientset
	certmanagerClient certmanagerclient.Interface
	istioClient      istioclient.Interface
}

// NewKubernetesClient creates a new Kubernetes client
func NewKubernetesClient(cfg *config.Config) (*KubernetesClient, error) {
	// Use in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	certmanagerClient, err := certmanagerclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create cert-manager client: %w", err)
	}

	istioClient, err := istioclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create istio client: %w", err)
	}

	return &KubernetesClient{
		cfg:              cfg,
		kubeClient:       kubeClient,
		certmanagerClient: certmanagerClient,
		istioClient:      istioClient,
	}, nil
}

// CertificateResult contains the result of certificate operations
type CertificateResult struct {
	SecretName  string
	Status      models.SSLStatus
	ExpiresAt   *time.Time
	Error       string
	IsReady     bool
}

// CreateCertificate creates a cert-manager Certificate resource for the domain
func (k *KubernetesClient) CreateCertificate(ctx context.Context, domain *models.CustomDomain) (*CertificateResult, error) {
	result := &CertificateResult{}

	// Generate certificate name from domain
	certName := generateResourceName(domain.Domain, "cert")
	secretName := generateResourceName(domain.Domain, "tls")
	result.SecretName = secretName

	// Build DNS names including www if enabled
	dnsNames := []string{domain.Domain}
	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		dnsNames = append(dnsNames, "www."+domain.Domain)
	}

	// Use HTTP-01 issuer for custom domains (external domains can't use DNS-01)
	issuerName := k.cfg.SSL.HTTP01IssuerName
	if issuerName == "" {
		issuerName = k.cfg.SSL.IssuerName
	}

	cert := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certName,
			Namespace: k.cfg.SSL.CertificateNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "custom-domain-service",
				"tesserix.app/tenant-id":       domain.TenantID.String(),
				"tesserix.app/domain-id":       domain.ID.String(),
			},
			Annotations: map[string]string{
				"tesserix.app/domain":     domain.Domain,
				"tesserix.app/created-at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: secretName,
			DNSNames:   dnsNames,
			IssuerRef: cmmeta.ObjectReference{
				Name: issuerName,
				Kind: k.cfg.SSL.IssuerKind,
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				Algorithm: certmanagerv1.RSAKeyAlgorithm,
				Size:      2048,
			},
		},
	}

	// Check if certificate already exists
	existing, err := k.certmanagerClient.CertmanagerV1().Certificates(k.cfg.SSL.CertificateNamespace).Get(ctx, certName, metav1.GetOptions{})
	if err == nil {
		// Certificate exists, update it
		existing.Spec = cert.Spec
		_, err = k.certmanagerClient.CertmanagerV1().Certificates(k.cfg.SSL.CertificateNamespace).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			result.Error = fmt.Sprintf("failed to update certificate: %v", err)
			result.Status = models.SSLStatusFailed
			return result, err
		}
		log.Info().Str("cert", certName).Msg("Certificate updated")
	} else {
		// Create new certificate
		_, err = k.certmanagerClient.CertmanagerV1().Certificates(k.cfg.SSL.CertificateNamespace).Create(ctx, cert, metav1.CreateOptions{})
		if err != nil {
			result.Error = fmt.Sprintf("failed to create certificate: %v", err)
			result.Status = models.SSLStatusFailed
			return result, err
		}
		log.Info().Str("cert", certName).Msg("Certificate created")
	}

	result.Status = models.SSLStatusProvisioning
	return result, nil
}

// GetCertificateStatus checks the status of a certificate
func (k *KubernetesClient) GetCertificateStatus(ctx context.Context, domain *models.CustomDomain) (*CertificateResult, error) {
	result := &CertificateResult{}
	certName := generateResourceName(domain.Domain, "cert")
	result.SecretName = generateResourceName(domain.Domain, "tls")

	cert, err := k.certmanagerClient.CertmanagerV1().Certificates(k.cfg.SSL.CertificateNamespace).Get(ctx, certName, metav1.GetOptions{})
	if err != nil {
		result.Status = models.SSLStatusPending
		result.Error = fmt.Sprintf("certificate not found: %v", err)
		return result, nil
	}

	// Check certificate conditions
	for _, condition := range cert.Status.Conditions {
		if condition.Type == certmanagerv1.CertificateConditionReady {
			if condition.Status == cmmeta.ConditionTrue {
				result.IsReady = true
				result.Status = models.SSLStatusActive
				if cert.Status.NotAfter != nil {
					expiresAt := cert.Status.NotAfter.Time
					result.ExpiresAt = &expiresAt
				}
			} else {
				result.Status = models.SSLStatusProvisioning
				result.Error = condition.Message
			}
			break
		}
	}

	return result, nil
}

// DeleteCertificate deletes the certificate resource
func (k *KubernetesClient) DeleteCertificate(ctx context.Context, domain *models.CustomDomain) error {
	certName := generateResourceName(domain.Domain, "cert")
	err := k.certmanagerClient.CertmanagerV1().Certificates(k.cfg.SSL.CertificateNamespace).Delete(ctx, certName, metav1.DeleteOptions{})
	if err != nil {
		log.Warn().Err(err).Str("cert", certName).Msg("Failed to delete certificate")
		return err
	}
	log.Info().Str("cert", certName).Msg("Certificate deleted")
	return nil
}

// VirtualServiceResult contains the result of VirtualService operations
type VirtualServiceResult struct {
	Name           string
	Status         models.RoutingStatus
	Error          string
	GatewayPatched bool
}

// CreateVirtualService creates or updates an Istio VirtualService for the domain.
// Custom domains always get their own dedicated VirtualService to keep tenant
// default domain VirtualServices clean and isolated.
func (k *KubernetesClient) CreateVirtualService(ctx context.Context, domain *models.CustomDomain) (*VirtualServiceResult, error) {
	// Build hosts list for the custom domain
	customHosts := []string{domain.Domain}
	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		customHosts = append(customHosts, "www."+domain.Domain)
	}

	// Always create a dedicated VirtualService for custom domains
	// This keeps tenant's default domain VS (e.g., oh-my-god-storefront-vs) clean
	// and custom domain routing isolated
	return k.createNewVirtualService(ctx, domain, customHosts)
}

// getTenantVSName returns the expected VirtualService name for a tenant
func (k *KubernetesClient) getTenantVSName(tenantSlug string, targetType models.TargetType) string {
	switch targetType {
	case models.TargetTypeStorefront:
		return tenantSlug + "-storefront-vs"
	case models.TargetTypeAdmin:
		return tenantSlug + "-admin-vs"
	case models.TargetTypeAPI:
		return tenantSlug + "-api-vs"
	default:
		return tenantSlug + "-storefront-vs"
	}
}

// addHostsToExistingVS adds custom domain hosts to an existing VirtualService
func (k *KubernetesClient) addHostsToExistingVS(ctx context.Context, vsName string, hosts []string, domain *models.CustomDomain) (bool, error) {
	existing, err := k.istioClient.NetworkingV1beta1().VirtualServices(k.cfg.Istio.VSNamespace).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("VirtualService %s not found: %w", vsName, err)
	}

	// Add new hosts that aren't already present
	updated := false
	for _, host := range hosts {
		found := false
		for _, existingHost := range existing.Spec.Hosts {
			if existingHost == host {
				found = true
				break
			}
		}
		if !found {
			existing.Spec.Hosts = append(existing.Spec.Hosts, host)
			updated = true
		}
	}

	if !updated {
		log.Debug().Str("vs", vsName).Msg("Hosts already present in VirtualService")
		return true, nil // Hosts already present, consider it success
	}

	// Add annotation to track custom domains
	if existing.Annotations == nil {
		existing.Annotations = make(map[string]string)
	}
	customDomains := existing.Annotations["tesserix.app/custom-domains"]
	if customDomains != "" {
		customDomains += "," + domain.Domain
	} else {
		customDomains = domain.Domain
	}
	existing.Annotations["tesserix.app/custom-domains"] = customDomains

	_, err = k.istioClient.NetworkingV1beta1().VirtualServices(k.cfg.Istio.VSNamespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to update VirtualService: %w", err)
	}

	return true, nil
}

// createNewVirtualService creates a new VirtualService for the custom domain
func (k *KubernetesClient) createNewVirtualService(ctx context.Context, domain *models.CustomDomain, hosts []string) (*VirtualServiceResult, error) {
	result := &VirtualServiceResult{}
	vsName := generateResourceName(domain.Domain, "vs")
	result.Name = vsName

	// Determine target service based on target type
	targetService, targetPort := k.getTargetService(domain.TargetType)

	// Build HTTP routes
	httpRoutes := []*networkingv1beta1.HTTPRoute{}

	// If redirectWWW is enabled and includeWWW is enabled, add redirect route
	if domain.RedirectWWW && domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		httpRoutes = append(httpRoutes, &networkingv1beta1.HTTPRoute{
			Name: "www-redirect",
			Match: []*networkingv1beta1.HTTPMatchRequest{
				{
					Headers: map[string]*networkingv1beta1.StringMatch{
						":authority": {
							MatchType: &networkingv1beta1.StringMatch_Prefix{
								Prefix: "www.",
							},
						},
					},
				},
			},
			Redirect: &networkingv1beta1.HTTPRedirect{
				Authority:    domain.Domain,
				RedirectCode: 301,
			},
		})
	}

	// Main routing rule
	httpRoutes = append(httpRoutes, &networkingv1beta1.HTTPRoute{
		Name: "main",
		Route: []*networkingv1beta1.HTTPRouteDestination{
			{
				Destination: &networkingv1beta1.Destination{
					Host: targetService,
					Port: &networkingv1beta1.PortSelector{
						Number: targetPort,
					},
				},
			},
		},
		Headers: &networkingv1beta1.Headers{
			Request: &networkingv1beta1.Headers_HeaderOperations{
				Set: map[string]string{
					"x-tenant-id":      domain.TenantID.String(),
					"x-tenant-slug":    domain.TenantSlug,
					"x-custom-domain":  domain.Domain,
					"x-forwarded-host": domain.Domain,
				},
			},
		},
	})

	vs := &v1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vsName,
			Namespace: k.cfg.Istio.VSNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "custom-domain-service",
				"tesserix.app/tenant-id":       domain.TenantID.String(),
				"tesserix.app/domain-id":       domain.ID.String(),
				"tesserix.app/target-type":     string(domain.TargetType),
			},
			Annotations: map[string]string{
				"tesserix.app/domain":     domain.Domain,
				"tesserix.app/created-at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		Spec: networkingv1beta1.VirtualService{
			Hosts:    hosts,
			Gateways: []string{k.cfg.Istio.GatewayNamespace + "/" + k.cfg.Istio.GatewayName},
			Http:     httpRoutes,
		},
	}

	// Check if VirtualService already exists
	existing, err := k.istioClient.NetworkingV1beta1().VirtualServices(k.cfg.Istio.VSNamespace).Get(ctx, vsName, metav1.GetOptions{})
	if err == nil {
		// Update existing
		existing.Spec = vs.Spec
		existing.Labels = vs.Labels
		existing.Annotations = vs.Annotations
		_, err = k.istioClient.NetworkingV1beta1().VirtualServices(k.cfg.Istio.VSNamespace).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			result.Error = fmt.Sprintf("failed to update VirtualService: %v", err)
			result.Status = models.RoutingStatusFailed
			return result, err
		}
		log.Info().Str("vs", vsName).Msg("VirtualService updated")
	} else {
		// Create new
		_, err = k.istioClient.NetworkingV1beta1().VirtualServices(k.cfg.Istio.VSNamespace).Create(ctx, vs, metav1.CreateOptions{})
		if err != nil {
			result.Error = fmt.Sprintf("failed to create VirtualService: %v", err)
			result.Status = models.RoutingStatusFailed
			return result, err
		}
		log.Info().Str("vs", vsName).Msg("VirtualService created")
	}

	result.Status = models.RoutingStatusActive
	return result, nil
}

// PatchGateway adds dedicated HTTPS server entries for the custom domain
func (k *KubernetesClient) PatchGateway(ctx context.Context, domain *models.CustomDomain) error {
	gateway, err := k.istioClient.NetworkingV1beta1().Gateways(k.cfg.Istio.GatewayNamespace).Get(ctx, k.cfg.Istio.GatewayName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get gateway: %w", err)
	}

	// Build hosts list
	hosts := []string{domain.Domain}
	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		hosts = append(hosts, "www."+domain.Domain)
	}

	tlsSecretName := generateResourceName(domain.Domain, "tls")
	serverPortName := "https-" + sanitizeDomain(domain.Domain)

	// Check if HTTPS server for this domain already exists
	serverExists := false
	for _, server := range gateway.Spec.Servers {
		if server.Port != nil && server.Port.Name == serverPortName {
			serverExists = true
			break
		}
	}

	if !serverExists {
		// Create a new HTTPS server entry for this custom domain
		newServer := &networkingv1beta1.Server{
			Port: &networkingv1beta1.Port{
				Number:   443,
				Name:     serverPortName,
				Protocol: "HTTPS",
			},
			Hosts: hosts,
			Tls: &networkingv1beta1.ServerTLSSettings{
				Mode:               networkingv1beta1.ServerTLSSettings_SIMPLE,
				CredentialName:     tlsSecretName,
				MinProtocolVersion: networkingv1beta1.ServerTLSSettings_TLSV1_2,
				CipherSuites: []string{
					"ECDHE-RSA-AES256-GCM-SHA384",
					"ECDHE-RSA-AES128-GCM-SHA256",
				},
			},
		}
		gateway.Spec.Servers = append(gateway.Spec.Servers, newServer)

		// Also add to HTTP server for ACME challenges and redirects
		k.addHostsToHTTPServer(gateway, hosts)

		_, err = k.istioClient.NetworkingV1beta1().Gateways(k.cfg.Istio.GatewayNamespace).Update(ctx, gateway, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update gateway: %w", err)
		}
		log.Info().Str("gateway", k.cfg.Istio.GatewayName).Str("domain", domain.Domain).Msg("Gateway HTTPS server created for custom domain")
	} else {
		log.Debug().Str("gateway", k.cfg.Istio.GatewayName).Str("domain", domain.Domain).Msg("Gateway server already exists for domain")
	}

	return nil
}

// addHostsToHTTPServer adds hosts to the HTTP (port 80) server for ACME challenges
func (k *KubernetesClient) addHostsToHTTPServer(gateway *v1beta1.Gateway, hosts []string) {
	for i, server := range gateway.Spec.Servers {
		if server.Port != nil && server.Port.Number == 80 && server.Port.Protocol == "HTTP" {
			for _, host := range hosts {
				found := false
				for _, h := range server.Hosts {
					if h == host {
						found = true
						break
					}
				}
				if !found {
					gateway.Spec.Servers[i].Hosts = append(gateway.Spec.Servers[i].Hosts, host)
				}
			}
			break
		}
	}
}

// RemoveFromGateway removes the custom domain's HTTPS server entry from the Gateway
func (k *KubernetesClient) RemoveFromGateway(ctx context.Context, domain *models.CustomDomain) error {
	gateway, err := k.istioClient.NetworkingV1beta1().Gateways(k.cfg.Istio.GatewayNamespace).Get(ctx, k.cfg.Istio.GatewayName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get gateway: %w", err)
	}

	serverPortName := "https-" + sanitizeDomain(domain.Domain)

	// Build hosts list to remove from HTTP server
	hostsToRemove := map[string]bool{domain.Domain: true}
	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		hostsToRemove["www."+domain.Domain] = true
	}

	// Remove the dedicated HTTPS server and hosts from HTTP server
	newServers := []*networkingv1beta1.Server{}
	updated := false

	for _, server := range gateway.Spec.Servers {
		// Skip the dedicated HTTPS server for this domain
		if server.Port != nil && server.Port.Name == serverPortName {
			updated = true
			continue
		}

		// For HTTP server, remove the custom domain hosts
		if server.Port != nil && server.Port.Number == 80 && server.Port.Protocol == "HTTP" {
			newHosts := []string{}
			for _, h := range server.Hosts {
				if !hostsToRemove[h] {
					newHosts = append(newHosts, h)
				} else {
					updated = true
				}
			}
			server.Hosts = newHosts
		}

		newServers = append(newServers, server)
	}

	if updated {
		gateway.Spec.Servers = newServers
		_, err = k.istioClient.NetworkingV1beta1().Gateways(k.cfg.Istio.GatewayNamespace).Update(ctx, gateway, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update gateway: %w", err)
		}
		log.Info().Str("gateway", k.cfg.Istio.GatewayName).Str("domain", domain.Domain).Msg("Custom domain removed from gateway")
	}

	return nil
}

// DeleteVirtualService deletes the dedicated VirtualService for a custom domain
func (k *KubernetesClient) DeleteVirtualService(ctx context.Context, domain *models.CustomDomain) error {
	// Delete the dedicated VirtualService for this custom domain
	vsName := generateResourceName(domain.Domain, "vs")
	err := k.istioClient.NetworkingV1beta1().VirtualServices(k.cfg.Istio.VSNamespace).Delete(ctx, vsName, metav1.DeleteOptions{})
	if err != nil {
		log.Warn().Err(err).Str("vs", vsName).Msg("Failed to delete VirtualService")
		return err
	}
	log.Info().Str("vs", vsName).Msg("VirtualService deleted")
	return nil
}

// removeHostsFromExistingVS removes custom domain hosts from an existing VirtualService
func (k *KubernetesClient) removeHostsFromExistingVS(ctx context.Context, vsName string, hostsToRemove map[string]bool, domainName string) (bool, error) {
	existing, err := k.istioClient.NetworkingV1beta1().VirtualServices(k.cfg.Istio.VSNamespace).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("VirtualService %s not found: %w", vsName, err)
	}

	// Check if this VS has custom domains annotation
	if existing.Annotations == nil {
		return false, nil
	}
	customDomains := existing.Annotations["tesserix.app/custom-domains"]
	if customDomains == "" || !strings.Contains(customDomains, domainName) {
		return false, nil // This VS doesn't track this custom domain
	}

	// Remove hosts
	newHosts := []string{}
	removed := false
	for _, host := range existing.Spec.Hosts {
		if hostsToRemove[host] {
			removed = true
		} else {
			newHosts = append(newHosts, host)
		}
	}

	if !removed {
		return false, nil
	}

	existing.Spec.Hosts = newHosts

	// Update custom domains annotation
	domains := strings.Split(customDomains, ",")
	newDomains := []string{}
	for _, d := range domains {
		if d != domainName {
			newDomains = append(newDomains, d)
		}
	}
	if len(newDomains) > 0 {
		existing.Annotations["tesserix.app/custom-domains"] = strings.Join(newDomains, ",")
	} else {
		delete(existing.Annotations, "tesserix.app/custom-domains")
	}

	_, err = k.istioClient.NetworkingV1beta1().VirtualServices(k.cfg.Istio.VSNamespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to update VirtualService: %w", err)
	}

	return true, nil
}

// UpdateSecretWithCertificate updates a secret with custom certificate data
func (k *KubernetesClient) UpdateSecretWithCertificate(ctx context.Context, secretName, namespace string, certData, keyData []byte) error {
	secret, err := k.kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	secret.Data["tls.crt"] = certData
	secret.Data["tls.key"] = keyData

	_, err = k.kubeClient.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	return nil
}

// getTargetService returns the target service and port based on target type
func (k *KubernetesClient) getTargetService(targetType models.TargetType) (string, uint32) {
	switch targetType {
	case models.TargetTypeStorefront:
		return "storefront.marketplace.svc.cluster.local", 80
	case models.TargetTypeAdmin:
		return "admin.marketplace.svc.cluster.local", 80
	case models.TargetTypeAPI:
		return "api-gateway.marketplace.svc.cluster.local", 80
	default:
		return "storefront.marketplace.svc.cluster.local", 80
	}
}

// generateResourceName generates a K8s-safe resource name from domain
func generateResourceName(domain, suffix string) string {
	// Replace dots with dashes and limit length
	name := fmt.Sprintf("%s-%s", sanitizeDomain(domain), suffix)
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// sanitizeDomain converts domain to a valid K8s name
func sanitizeDomain(domain string) string {
	// Replace dots with dashes
	name := ""
	for _, r := range domain {
		if r == '.' {
			name += "-"
		} else if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			name += string(r)
		} else if r >= 'A' && r <= 'Z' {
			name += string(r + 32) // to lowercase
		}
	}
	return name
}

// AnnotateDomainResources adds annotations to track domain resources
func (k *KubernetesClient) AnnotateDomainResources(ctx context.Context, domain *models.CustomDomain, resources map[string]string) error {
	resourcesJSON, err := json.Marshal(resources)
	if err != nil {
		return err
	}

	// Store resource references as annotation on a ConfigMap
	cmName := fmt.Sprintf("domain-%s-resources", domain.ID.String()[:8])
	cm := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      cmName,
			"namespace": k.cfg.Istio.VSNamespace,
			"labels": map[string]string{
				"app.kubernetes.io/managed-by": "custom-domain-service",
				"tesserix.app/domain-id":       domain.ID.String(),
			},
		},
		"data": map[string]string{
			"resources": string(resourcesJSON),
		},
	}

	cmData, _ := json.Marshal(cm)
	_, err = k.kubeClient.CoreV1().ConfigMaps(k.cfg.Istio.VSNamespace).Patch(
		ctx,
		cmName,
		types.ApplyPatchType,
		cmData,
		metav1.PatchOptions{FieldManager: "custom-domain-service"},
	)
	return err
}

// CreateAuthorizationPolicy creates an AuthorizationPolicy to allow traffic to the custom domain
func (k *KubernetesClient) CreateAuthorizationPolicy(ctx context.Context, domain *models.CustomDomain) error {
	policyName := generateResourceName(domain.Domain, "authz")

	// Build hosts list
	hosts := []string{domain.Domain}
	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		hosts = append(hosts, "www."+domain.Domain)
	}

	policy := &securityclientv1beta1.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: k.cfg.Istio.GatewayNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "custom-domain-service",
				"tesserix.app/tenant-id":       domain.TenantID.String(),
				"tesserix.app/domain-id":       domain.ID.String(),
			},
			Annotations: map[string]string{
				"tesserix.app/domain":     domain.Domain,
				"tesserix.app/created-at": time.Now().UTC().Format(time.RFC3339),
			},
		},
		Spec: securityv1beta1.AuthorizationPolicy{
			Selector: &typev1beta1.WorkloadSelector{
				MatchLabels: map[string]string{
					"istio": "ingressgateway",
				},
			},
			Action: securityv1beta1.AuthorizationPolicy_ALLOW,
			Rules: []*securityv1beta1.Rule{
				{
					To: []*securityv1beta1.Rule_To{
						{
							Operation: &securityv1beta1.Operation{
								Hosts: hosts,
							},
						},
					},
				},
			},
		},
	}

	// Check if policy already exists
	existing, err := k.istioClient.SecurityV1beta1().AuthorizationPolicies(k.cfg.Istio.GatewayNamespace).Get(ctx, policyName, metav1.GetOptions{})
	if err == nil {
		// Update existing
		existing.Spec = policy.Spec
		existing.Labels = policy.Labels
		existing.Annotations = policy.Annotations
		_, err = k.istioClient.SecurityV1beta1().AuthorizationPolicies(k.cfg.Istio.GatewayNamespace).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update authorization policy: %w", err)
		}
		log.Info().Str("policy", policyName).Msg("Authorization policy updated")
	} else {
		// Create new
		_, err = k.istioClient.SecurityV1beta1().AuthorizationPolicies(k.cfg.Istio.GatewayNamespace).Create(ctx, policy, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create authorization policy: %w", err)
		}
		log.Info().Str("policy", policyName).Msg("Authorization policy created")
	}

	return nil
}

// DeleteAuthorizationPolicy removes the AuthorizationPolicy for the custom domain
func (k *KubernetesClient) DeleteAuthorizationPolicy(ctx context.Context, domain *models.CustomDomain) error {
	policyName := generateResourceName(domain.Domain, "authz")
	err := k.istioClient.SecurityV1beta1().AuthorizationPolicies(k.cfg.Istio.GatewayNamespace).Delete(ctx, policyName, metav1.DeleteOptions{})
	if err != nil {
		log.Warn().Err(err).Str("policy", policyName).Msg("Failed to delete authorization policy")
		return err
	}
	log.Info().Str("policy", policyName).Msg("Authorization policy deleted")
	return nil
}

// CreateACMEChallengeVirtualService prepares Gateway for ACME HTTP-01 challenges
// Note: Cert-manager creates its own Ingress resources for challenge routing.
// This method ensures the Gateway accepts HTTP traffic for the custom domain hosts.
func (k *KubernetesClient) CreateACMEChallengeVirtualService(ctx context.Context, domain *models.CustomDomain) error {
	// Build hosts list
	hosts := []string{domain.Domain}
	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		hosts = append(hosts, "www."+domain.Domain)
	}

	// Note: Cert-manager handles ACME challenge routing via Ingress resources.
	// We just ensure the Gateway accepts HTTP traffic for these hosts (done in PatchGateway).
	log.Info().Str("domain", domain.Domain).Strs("hosts", hosts).Msg("ACME challenge hosts prepared, cert-manager will handle routing")
	return nil
}

// DeleteACMEChallengeVirtualService removes the ACME challenge VirtualService
func (k *KubernetesClient) DeleteACMEChallengeVirtualService(ctx context.Context, domain *models.CustomDomain) error {
	vsName := generateResourceName(domain.Domain, "acme-vs")
	err := k.istioClient.NetworkingV1beta1().VirtualServices(k.cfg.Istio.GatewayNamespace).Delete(ctx, vsName, metav1.DeleteOptions{})
	if err != nil {
		// Not an error if it doesn't exist
		log.Debug().Err(err).Str("vs", vsName).Msg("ACME challenge VirtualService not found or already deleted")
	}
	return nil
}
