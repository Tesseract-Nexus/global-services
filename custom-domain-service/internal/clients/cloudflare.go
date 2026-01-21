package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"custom-domain-service/internal/config"
	"custom-domain-service/internal/models"

	"github.com/rs/zerolog/log"
)

// CloudflareClient handles Cloudflare Tunnel API operations
type CloudflareClient struct {
	cfg        *config.CloudflareConfig
	httpClient *http.Client
	baseURL    string
}

// TunnelConfig represents the Cloudflare Tunnel configuration
type TunnelConfig struct {
	Ingress []IngressRule `json:"ingress"`
}

// IngressRule represents a single ingress rule in the tunnel
type IngressRule struct {
	Hostname      string         `json:"hostname,omitempty"`
	Service       string         `json:"service"`
	OriginRequest *OriginRequest `json:"originRequest,omitempty"`
}

// OriginRequest contains origin-specific settings
type OriginRequest struct {
	NoTLSVerify bool `json:"noTLSVerify,omitempty"`
}

// TunnelConfigResponse represents the API response for tunnel configuration
type TunnelConfigResponse struct {
	Success  bool                   `json:"success"`
	Errors   []CloudflareError      `json:"errors"`
	Messages []interface{}          `json:"messages"`
	Result   TunnelConfigResult     `json:"result"`
}

// TunnelConfigResult contains the tunnel configuration result
type TunnelConfigResult struct {
	TunnelID  string                 `json:"tunnel_id"`
	Version   int                    `json:"version"`
	Config    *TunnelConfigPayload   `json:"config"`
	Source    string                 `json:"source"`
	CreatedAt string                 `json:"created_at"`
}

// TunnelConfigPayload wraps the configuration for API requests
type TunnelConfigPayload struct {
	Ingress []IngressRule `json:"ingress"`
}

// CloudflareError represents an error from the Cloudflare API
type CloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewCloudflareClient creates a new Cloudflare client
func NewCloudflareClient(cfg *config.CloudflareConfig) *CloudflareClient {
	return &CloudflareClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.cloudflare.com/client/v4",
	}
}

// GetTunnelConfig retrieves the current tunnel configuration
func (c *CloudflareClient) GetTunnelConfig(ctx context.Context) (*TunnelConfigResult, error) {
	url := fmt.Sprintf("%s/accounts/%s/cfd_tunnel/%s/configurations",
		c.baseURL, c.cfg.AccountID, c.cfg.TunnelID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result TunnelConfigResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("cloudflare API error: %s", result.Errors[0].Message)
		}
		return nil, fmt.Errorf("cloudflare API request failed")
	}

	return &result.Result, nil
}

// UpdateTunnelConfig updates the tunnel configuration with new ingress rules
func (c *CloudflareClient) UpdateTunnelConfig(ctx context.Context, config *TunnelConfigPayload) (*TunnelConfigResult, error) {
	url := fmt.Sprintf("%s/accounts/%s/cfd_tunnel/%s/configurations",
		c.baseURL, c.cfg.AccountID, c.cfg.TunnelID)

	payload := map[string]interface{}{
		"config": config,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result TunnelConfigResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("cloudflare API error: %s", result.Errors[0].Message)
		}
		return nil, fmt.Errorf("cloudflare API request failed")
	}

	return &result.Result, nil
}

// AddDomainToTunnel adds a custom domain to the Cloudflare Tunnel configuration
func (c *CloudflareClient) AddDomainToTunnel(ctx context.Context, domain *models.CustomDomain) error {
	if !c.cfg.Enabled {
		log.Debug().Str("domain", domain.Domain).Msg("Cloudflare Tunnel disabled, skipping")
		return nil
	}

	log.Info().Str("domain", domain.Domain).Msg("Adding domain to Cloudflare Tunnel")

	// Get current configuration
	currentConfig, err := c.GetTunnelConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current tunnel config: %w", err)
	}

	// Build new ingress rules
	var ingress []IngressRule

	// Copy existing rules (except catch-all)
	if currentConfig.Config != nil {
		for _, rule := range currentConfig.Config.Ingress {
			// Skip if this domain already exists or if it's the catch-all
			if rule.Hostname == domain.Domain || rule.Hostname == "" {
				continue
			}
			ingress = append(ingress, rule)
		}
	}

	// Add new domain rule
	newRule := IngressRule{
		Hostname: domain.Domain,
		Service:  c.cfg.OriginService,
		OriginRequest: &OriginRequest{
			NoTLSVerify: true,
		},
	}
	ingress = append(ingress, newRule)

	// Add www subdomain if requested
	if domain.IncludeWWW && !isWWWDomain(domain.Domain) {
		wwwRule := IngressRule{
			Hostname: "www." + domain.Domain,
			Service:  c.cfg.OriginService,
			OriginRequest: &OriginRequest{
				NoTLSVerify: true,
			},
		}
		ingress = append(ingress, wwwRule)
	}

	// Add catch-all rule at the end
	ingress = append(ingress, IngressRule{
		Service: "http_status:404",
	})

	// Update configuration
	newConfig := &TunnelConfigPayload{
		Ingress: ingress,
	}

	_, err = c.UpdateTunnelConfig(ctx, newConfig)
	if err != nil {
		return fmt.Errorf("failed to update tunnel config: %w", err)
	}

	log.Info().Str("domain", domain.Domain).Msg("Successfully added domain to Cloudflare Tunnel")
	return nil
}

// RemoveDomainFromTunnel removes a custom domain from the Cloudflare Tunnel configuration
func (c *CloudflareClient) RemoveDomainFromTunnel(ctx context.Context, domain *models.CustomDomain) error {
	if !c.cfg.Enabled {
		log.Debug().Str("domain", domain.Domain).Msg("Cloudflare Tunnel disabled, skipping")
		return nil
	}

	log.Info().Str("domain", domain.Domain).Msg("Removing domain from Cloudflare Tunnel")

	// Get current configuration
	currentConfig, err := c.GetTunnelConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current tunnel config: %w", err)
	}

	// Build new ingress rules without the domain
	var ingress []IngressRule

	if currentConfig.Config != nil {
		for _, rule := range currentConfig.Config.Ingress {
			// Skip the domain being removed and its www variant
			if rule.Hostname == domain.Domain ||
			   rule.Hostname == "www."+domain.Domain ||
			   rule.Hostname == "" {
				continue
			}
			ingress = append(ingress, rule)
		}
	}

	// Add catch-all rule at the end
	ingress = append(ingress, IngressRule{
		Service: "http_status:404",
	})

	// Update configuration
	newConfig := &TunnelConfigPayload{
		Ingress: ingress,
	}

	_, err = c.UpdateTunnelConfig(ctx, newConfig)
	if err != nil {
		return fmt.Errorf("failed to update tunnel config: %w", err)
	}

	log.Info().Str("domain", domain.Domain).Msg("Successfully removed domain from Cloudflare Tunnel")
	return nil
}

// GetTunnelCNAME returns the CNAME target for custom domains
func (c *CloudflareClient) GetTunnelCNAME() string {
	return fmt.Sprintf("%s.cfargotunnel.com", c.cfg.TunnelID)
}

// GetMaskedTunnelCNAME returns a masked version of the tunnel CNAME for logging
func (c *CloudflareClient) GetMaskedTunnelCNAME() string {
	return MaskSensitiveID(c.cfg.TunnelID) + ".cfargotunnel.com"
}

// MaskSensitiveID masks a sensitive ID for logging (shows first 4 and last 4 chars)
func MaskSensitiveID(id string) string {
	if len(id) <= 8 {
		return "****"
	}
	return id[:4] + "****" + id[len(id)-4:]
}

// isWWWDomain checks if the domain already starts with www.
func isWWWDomain(domain string) bool {
	return len(domain) > 4 && domain[:4] == "www."
}

// =====================================================
// DNS MANAGEMENT METHODS
// =====================================================

// DNSRecord represents a Cloudflare DNS record
type DNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// DNSRecordResponse represents the API response for DNS records
type DNSRecordResponse struct {
	Success  bool            `json:"success"`
	Errors   []CloudflareError `json:"errors"`
	Result   *DNSRecord      `json:"result,omitempty"`
	ResultInfo *struct {
		TotalCount int `json:"total_count"`
	} `json:"result_info,omitempty"`
}

// DNSRecordsListResponse represents the API response for listing DNS records
type DNSRecordsListResponse struct {
	Success bool              `json:"success"`
	Errors  []CloudflareError `json:"errors"`
	Result  []DNSRecord       `json:"result"`
}

// ZoneResponse represents the API response for zone lookup
type ZoneResponse struct {
	Success bool              `json:"success"`
	Errors  []CloudflareError `json:"errors"`
	Result  []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"result"`
}

// GetZoneIDForDomain looks up the Cloudflare zone ID for a domain
// It traverses up the domain hierarchy to find the matching zone
func (c *CloudflareClient) GetZoneIDForDomain(ctx context.Context, domain string) (string, error) {
	// Try to find zone by walking up the domain hierarchy
	parts := splitDomain(domain)

	for i := 0; i < len(parts)-1; i++ {
		zoneName := joinDomain(parts[i:])
		zoneID, err := c.lookupZone(ctx, zoneName)
		if err != nil {
			log.Debug().Err(err).Str("zone", zoneName).Msg("Zone not found, trying parent")
			continue
		}
		if zoneID != "" {
			log.Info().Str("domain", domain).Str("zone", zoneName).Str("zone_id", zoneID).Msg("Found zone for domain")
			return zoneID, nil
		}
	}

	return "", fmt.Errorf("no Cloudflare zone found for domain %s", domain)
}

// lookupZone looks up a zone by name
func (c *CloudflareClient) lookupZone(ctx context.Context, zoneName string) (string, error) {
	url := fmt.Sprintf("%s/zones?name=%s&status=active", c.baseURL, zoneName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result ZoneResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success || len(result.Result) == 0 {
		return "", nil
	}

	return result.Result[0].ID, nil
}

// CreateOrUpdateCNAME creates or updates a CNAME record pointing to the tunnel
func (c *CloudflareClient) CreateOrUpdateCNAME(ctx context.Context, domain *models.CustomDomain) error {
	if !c.cfg.Enabled || !c.cfg.AutoConfigureDNS {
		log.Debug().Str("domain", domain.Domain).Msg("Cloudflare DNS auto-config disabled, skipping")
		return nil
	}

	// Get zone ID for the domain
	zoneID, err := c.GetZoneIDForDomain(ctx, domain.Domain)
	if err != nil {
		log.Warn().Err(err).Str("domain", domain.Domain).Msg("Could not find zone for domain, customer must configure DNS manually")
		return nil // Not an error - customer will configure their own DNS
	}

	tunnelCNAME := c.GetTunnelCNAME()

	// Create CNAME for the main domain
	if err := c.upsertCNAMERecord(ctx, zoneID, domain.Domain, tunnelCNAME); err != nil {
		return fmt.Errorf("failed to create CNAME for %s: %w", domain.Domain, err)
	}

	// Create CNAME for www subdomain if enabled
	if domain.IncludeWWW && !isWWWDomain(domain.Domain) {
		wwwDomain := "www." + domain.Domain
		if err := c.upsertCNAMERecord(ctx, zoneID, wwwDomain, tunnelCNAME); err != nil {
			log.Warn().Err(err).Str("domain", wwwDomain).Msg("Failed to create www CNAME")
		}
	}

	log.Info().Str("domain", domain.Domain).Str("target", c.GetMaskedTunnelCNAME()).Msg("DNS CNAME records configured in Cloudflare")
	return nil
}

// upsertCNAMERecord creates or updates a single CNAME record
func (c *CloudflareClient) upsertCNAMERecord(ctx context.Context, zoneID, name, target string) error {
	// First, check if record exists
	existingID, err := c.findDNSRecord(ctx, zoneID, "CNAME", name)
	if err != nil {
		log.Debug().Err(err).Str("name", name).Msg("Error finding existing record, will try to create")
	}

	record := DNSRecord{
		Type:    "CNAME",
		Name:    name,
		Content: target,
		TTL:     1, // Auto TTL
		Proxied: true, // Enable Cloudflare proxy (orange cloud)
	}

	var url string
	var method string

	if existingID != "" {
		// Update existing record
		url = fmt.Sprintf("%s/zones/%s/dns_records/%s", c.baseURL, zoneID, existingID)
		method = "PUT"
	} else {
		// Create new record
		url = fmt.Sprintf("%s/zones/%s/dns_records", c.baseURL, zoneID)
		method = "POST"
	}

	jsonData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result DNSRecordResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return fmt.Errorf("cloudflare API error: %s", result.Errors[0].Message)
		}
		return fmt.Errorf("cloudflare API request failed")
	}

	action := "created"
	if existingID != "" {
		action = "updated"
	}
	log.Info().Str("name", name).Str("target", target).Str("action", action).Msg("DNS CNAME record configured")

	return nil
}

// findDNSRecord finds an existing DNS record by type and name
func (c *CloudflareClient) findDNSRecord(ctx context.Context, zoneID, recordType, name string) (string, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records?type=%s&name=%s", c.baseURL, zoneID, recordType, name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result DNSRecordsListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success || len(result.Result) == 0 {
		return "", nil
	}

	return result.Result[0].ID, nil
}

// DeleteDNSRecords deletes DNS records for a domain
func (c *CloudflareClient) DeleteDNSRecords(ctx context.Context, domain *models.CustomDomain) error {
	if !c.cfg.Enabled || !c.cfg.AutoConfigureDNS {
		return nil
	}

	zoneID, err := c.GetZoneIDForDomain(ctx, domain.Domain)
	if err != nil {
		log.Debug().Err(err).Str("domain", domain.Domain).Msg("Could not find zone, skipping DNS cleanup")
		return nil
	}

	// Delete main domain CNAME
	if err := c.deleteDNSRecord(ctx, zoneID, "CNAME", domain.Domain); err != nil {
		log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to delete CNAME record")
	}

	// Delete www CNAME if applicable
	if domain.IncludeWWW && !isWWWDomain(domain.Domain) {
		wwwDomain := "www." + domain.Domain
		if err := c.deleteDNSRecord(ctx, zoneID, "CNAME", wwwDomain); err != nil {
			log.Warn().Err(err).Str("domain", wwwDomain).Msg("Failed to delete www CNAME record")
		}
	}

	log.Info().Str("domain", domain.Domain).Msg("DNS records deleted from Cloudflare")
	return nil
}

// deleteDNSRecord deletes a single DNS record
func (c *CloudflareClient) deleteDNSRecord(ctx context.Context, zoneID, recordType, name string) error {
	recordID, err := c.findDNSRecord(ctx, zoneID, recordType, name)
	if err != nil || recordID == "" {
		return nil // Record doesn't exist, nothing to delete
	}

	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.baseURL, zoneID, recordID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to delete record: status %d", resp.StatusCode)
	}

	return nil
}

// VerifyDNSPropagation checks if the domain's CNAME points to the tunnel
func (c *CloudflareClient) VerifyDNSPropagation(ctx context.Context, domain string) (bool, string, error) {
	tunnelCNAME := c.GetTunnelCNAME()

	// Use DNS lookup to verify the CNAME
	cname, err := lookupCNAME(ctx, domain)
	if err != nil {
		return false, "", fmt.Errorf("DNS lookup failed: %w", err)
	}

	// Check if CNAME matches tunnel
	cname = strings.TrimSuffix(cname, ".")
	tunnelCNAME = strings.TrimSuffix(tunnelCNAME, ".")

	if strings.EqualFold(cname, tunnelCNAME) {
		return true, cname, nil
	}

	return false, cname, nil
}

// lookupCNAME performs a CNAME lookup using the system resolver
func lookupCNAME(ctx context.Context, domain string) (string, error) {
	resolver := &net.Resolver{PreferGo: true}
	cname, err := resolver.LookupCNAME(ctx, domain)
	if err != nil {
		return "", err
	}
	return cname, nil
}

// Helper functions for domain parsing

// splitDomain splits a domain into parts
func splitDomain(domain string) []string {
	return strings.Split(domain, ".")
}

// joinDomain joins domain parts
func joinDomain(parts []string) string {
	return strings.Join(parts, ".")
}

// IsTunnelConfigured checks if the domain is configured in the tunnel
func (c *CloudflareClient) IsTunnelConfigured(ctx context.Context, domain string) (bool, error) {
	if !c.cfg.Enabled {
		return false, nil
	}

	config, err := c.GetTunnelConfig(ctx)
	if err != nil {
		return false, err
	}

	if config.Config == nil {
		return false, nil
	}

	for _, rule := range config.Config.Ingress {
		if rule.Hostname == domain || rule.Hostname == "*."+getBaseDomain(domain) {
			return true, nil
		}
	}

	return false, nil
}

// getBaseDomain extracts the base domain (e.g., example.com from shop.example.com)
func getBaseDomain(domain string) string {
	parts := splitDomain(domain)
	if len(parts) <= 2 {
		return domain
	}
	return joinDomain(parts[len(parts)-2:])
}

// =====================================================
// CLOUDFLARE FOR SAAS (CUSTOM HOSTNAMES) API
// =====================================================
// This is the correct approach for multi-tenant custom domains.
// Customers CNAME their domain to your fallback origin (e.g., customers.tesserix.app)
// You register their domain as a Custom Hostname in Cloudflare
// Cloudflare issues SSL cert and routes traffic to your origin

// CustomHostname represents a Cloudflare Custom Hostname (for SaaS)
type CustomHostname struct {
	ID                        string                   `json:"id,omitempty"`
	Hostname                  string                   `json:"hostname"`
	SSL                       *CustomHostnameSSL       `json:"ssl,omitempty"`
	CustomOriginServer        string                   `json:"custom_origin_server,omitempty"`
	CustomOriginSNI           string                   `json:"custom_origin_sni,omitempty"`
	Status                    string                   `json:"status,omitempty"`
	VerificationErrors        []string                 `json:"verification_errors,omitempty"`
	OwnershipVerification     *OwnershipVerification   `json:"ownership_verification,omitempty"`
	OwnershipVerificationHTTP *OwnershipVerificationHTTP `json:"ownership_verification_http,omitempty"`
	CreatedAt                 string                   `json:"created_at,omitempty"`
}

// CustomHostnameSSL represents SSL configuration for a custom hostname
type CustomHostnameSSL struct {
	ID                   string                 `json:"id,omitempty"`
	Status               string                 `json:"status,omitempty"`
	Method               string                 `json:"method,omitempty"`
	Type                 string                 `json:"type,omitempty"`
	CertificateAuthority string                 `json:"certificate_authority,omitempty"`
	ValidationRecords    []SSLValidationRecord  `json:"validation_records,omitempty"`
	ValidationErrors     []SSLValidationError   `json:"validation_errors,omitempty"`
	Settings             *CustomHostnameSSLSettings `json:"settings,omitempty"`
	Wildcard             bool                   `json:"wildcard,omitempty"`
	BundleMethod         string                 `json:"bundle_method,omitempty"`
}

// CustomHostnameSSLSettings represents SSL settings for custom hostname
type CustomHostnameSSLSettings struct {
	HTTP2         string   `json:"http2,omitempty"`
	MinTLSVersion string   `json:"min_tls_version,omitempty"`
	TLS13         string   `json:"tls_1_3,omitempty"`
	Ciphers       []string `json:"ciphers,omitempty"`
}

// SSLValidationRecord represents a validation record for custom hostname SSL
type SSLValidationRecord struct {
	TXTName  string `json:"txt_name,omitempty"`
	TXTValue string `json:"txt_value,omitempty"`
	HTTPUrl  string `json:"http_url,omitempty"`
	HTTPBody string `json:"http_body,omitempty"`
	CnameName string `json:"cname_name,omitempty"`
	CnameTarget string `json:"cname_target,omitempty"`
}

// SSLValidationError represents a validation error
type SSLValidationError struct {
	Message string `json:"message"`
}

// OwnershipVerification represents DNS TXT verification for custom hostname
type OwnershipVerification struct {
	Type  string `json:"type,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// OwnershipVerificationHTTP represents HTTP verification for custom hostname
type OwnershipVerificationHTTP struct {
	HTTPUrl  string `json:"http_url,omitempty"`
	HTTPBody string `json:"http_body,omitempty"`
}

// CustomHostnameResponse represents the API response for a single custom hostname
type CustomHostnameResponse struct {
	Success  bool              `json:"success"`
	Errors   []CloudflareError `json:"errors"`
	Messages []interface{}     `json:"messages"`
	Result   *CustomHostname   `json:"result,omitempty"`
}

// CustomHostnamesListResponse represents the API response for listing custom hostnames
type CustomHostnamesListResponse struct {
	Success  bool              `json:"success"`
	Errors   []CloudflareError `json:"errors"`
	Messages []interface{}     `json:"messages"`
	Result   []CustomHostname  `json:"result"`
}

// CreateCustomHostnameRequest represents the request body for creating a custom hostname
type CreateCustomHostnameRequest struct {
	Hostname           string             `json:"hostname"`
	SSL                *CustomHostnameSSL `json:"ssl,omitempty"`
	CustomOriginServer string             `json:"custom_origin_server,omitempty"`
}

// GetCustomerCNAMETarget returns the CNAME target that customers should point their domain to
// This is the fallback origin for Cloudflare for SaaS (e.g., customers.tesserix.app)
func (c *CloudflareClient) GetCustomerCNAMETarget() string {
	if c.cfg.FallbackOrigin != "" {
		return c.cfg.FallbackOrigin
	}
	// Fallback to tunnel CNAME if SaaS not configured (for backwards compatibility)
	return c.GetTunnelCNAME()
}

// CreateCustomHostname creates a new custom hostname in Cloudflare for SaaS
// This registers the customer's domain so Cloudflare will issue SSL and route traffic
func (c *CloudflareClient) CreateCustomHostname(ctx context.Context, domain *models.CustomDomain) (*CustomHostname, error) {
	if !c.cfg.Enabled {
		log.Debug().Str("domain", domain.Domain).Msg("Cloudflare disabled, skipping custom hostname creation")
		return nil, nil
	}

	if c.cfg.SaaSZoneID == "" {
		log.Warn().Str("domain", domain.Domain).Msg("SaaS Zone ID not configured, falling back to tunnel ingress")
		return nil, nil
	}

	log.Info().Str("domain", domain.Domain).Str("zone_id", c.cfg.SaaSZoneID).Msg("Creating Cloudflare Custom Hostname")

	url := fmt.Sprintf("%s/zones/%s/custom_hostnames", c.baseURL, c.cfg.SaaSZoneID)

	// Create request with SSL settings
	reqBody := CreateCustomHostnameRequest{
		Hostname: domain.Domain,
		SSL: &CustomHostnameSSL{
			Method:               "http", // Use HTTP validation (via CNAME proxy)
			Type:                 "dv",   // Domain Validation
			CertificateAuthority: "digicert", // Or "lets_encrypt"
			Settings: &CustomHostnameSSLSettings{
				MinTLSVersion: "1.2",
				TLS13:         "on",
			},
		},
	}

	// If fallback origin is configured, use it as custom origin
	if c.cfg.FallbackOrigin != "" {
		reqBody.CustomOriginServer = c.cfg.FallbackOrigin
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result CustomHostnameResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			// Check if hostname already exists
			for _, cfErr := range result.Errors {
				if cfErr.Code == 1406 { // Hostname already exists
					log.Info().Str("domain", domain.Domain).Msg("Custom hostname already exists, fetching existing")
					return c.GetCustomHostnameByName(ctx, domain.Domain)
				}
			}
			return nil, fmt.Errorf("cloudflare API error: %s (code: %d)", result.Errors[0].Message, result.Errors[0].Code)
		}
		return nil, fmt.Errorf("cloudflare API request failed")
	}

	log.Info().
		Str("domain", domain.Domain).
		Str("id", result.Result.ID).
		Str("status", result.Result.Status).
		Msg("Successfully created Cloudflare Custom Hostname")

	return result.Result, nil
}

// GetCustomHostname retrieves a custom hostname by its ID
func (c *CloudflareClient) GetCustomHostname(ctx context.Context, hostnameID string) (*CustomHostname, error) {
	if !c.cfg.Enabled || c.cfg.SaaSZoneID == "" {
		return nil, nil
	}

	url := fmt.Sprintf("%s/zones/%s/custom_hostnames/%s", c.baseURL, c.cfg.SaaSZoneID, hostnameID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result CustomHostnameResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("cloudflare API error: %s", result.Errors[0].Message)
		}
		return nil, fmt.Errorf("cloudflare API request failed")
	}

	return result.Result, nil
}

// GetCustomHostnameByName retrieves a custom hostname by domain name
func (c *CloudflareClient) GetCustomHostnameByName(ctx context.Context, domain string) (*CustomHostname, error) {
	if !c.cfg.Enabled || c.cfg.SaaSZoneID == "" {
		return nil, nil
	}

	url := fmt.Sprintf("%s/zones/%s/custom_hostnames?hostname=%s", c.baseURL, c.cfg.SaaSZoneID, domain)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result CustomHostnamesListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("cloudflare API error: %s", result.Errors[0].Message)
		}
		return nil, fmt.Errorf("cloudflare API request failed")
	}

	if len(result.Result) == 0 {
		return nil, nil
	}

	return &result.Result[0], nil
}

// DeleteCustomHostname removes a custom hostname from Cloudflare
func (c *CloudflareClient) DeleteCustomHostname(ctx context.Context, hostnameID string) error {
	if !c.cfg.Enabled || c.cfg.SaaSZoneID == "" {
		return nil
	}

	log.Info().Str("hostname_id", hostnameID).Msg("Deleting Cloudflare Custom Hostname")

	url := fmt.Sprintf("%s/zones/%s/custom_hostnames/%s", c.baseURL, c.cfg.SaaSZoneID, hostnameID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Success bool              `json:"success"`
		Errors  []CloudflareError `json:"errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			// Ignore "not found" errors
			for _, cfErr := range result.Errors {
				if cfErr.Code == 1404 {
					log.Debug().Str("hostname_id", hostnameID).Msg("Custom hostname not found, already deleted")
					return nil
				}
			}
			return fmt.Errorf("cloudflare API error: %s", result.Errors[0].Message)
		}
		return fmt.Errorf("cloudflare API request failed")
	}

	log.Info().Str("hostname_id", hostnameID).Msg("Successfully deleted Cloudflare Custom Hostname")
	return nil
}

// DeleteCustomHostnameByName removes a custom hostname by domain name
func (c *CloudflareClient) DeleteCustomHostnameByName(ctx context.Context, domain string) error {
	hostname, err := c.GetCustomHostnameByName(ctx, domain)
	if err != nil {
		return err
	}
	if hostname == nil {
		log.Debug().Str("domain", domain).Msg("Custom hostname not found, nothing to delete")
		return nil
	}
	return c.DeleteCustomHostname(ctx, hostname.ID)
}

// GetCustomHostnameStatus returns the status of a custom hostname
// Possible statuses: pending, active, moved, deleted
func (c *CloudflareClient) GetCustomHostnameStatus(ctx context.Context, domain string) (string, *CustomHostname, error) {
	hostname, err := c.GetCustomHostnameByName(ctx, domain)
	if err != nil {
		return "", nil, err
	}
	if hostname == nil {
		return "not_found", nil, nil
	}
	return hostname.Status, hostname, nil
}

// RefreshCustomHostnameSSL triggers a re-validation of SSL for a custom hostname
func (c *CloudflareClient) RefreshCustomHostnameSSL(ctx context.Context, hostnameID string) (*CustomHostname, error) {
	if !c.cfg.Enabled || c.cfg.SaaSZoneID == "" {
		return nil, nil
	}

	url := fmt.Sprintf("%s/zones/%s/custom_hostnames/%s", c.baseURL, c.cfg.SaaSZoneID, hostnameID)

	reqBody := map[string]interface{}{
		"ssl": map[string]interface{}{
			"method": "http",
			"type":   "dv",
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result CustomHostnameResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("cloudflare API error: %s", result.Errors[0].Message)
		}
		return nil, fmt.Errorf("cloudflare API request failed")
	}

	return result.Result, nil
}

// IsSaaSEnabled returns true if Cloudflare for SaaS is configured
func (c *CloudflareClient) IsSaaSEnabled() bool {
	return c.cfg.Enabled && c.cfg.SaaSZoneID != "" && c.cfg.FallbackOrigin != ""
}
