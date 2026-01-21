package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// isWWWDomain checks if the domain already starts with www.
func isWWWDomain(domain string) bool {
	return len(domain) > 4 && domain[:4] == "www."
}
