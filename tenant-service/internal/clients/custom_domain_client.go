package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// CustomDomainClient handles communication with the custom-domain-service
// Used to create and manage custom domains during tenant provisioning
type CustomDomainClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewCustomDomainClient creates a new custom-domain service client
func NewCustomDomainClient(baseURL string) *CustomDomainClient {
	return &CustomDomainClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateDomainRequest represents the request to create a custom domain
type CreateDomainRequest struct {
	Domain       string `json:"domain"`
	StorefrontID string `json:"storefront_id,omitempty"`
	TargetType   string `json:"target_type,omitempty"` // storefront, admin, api
	RedirectWWW  bool   `json:"redirect_www"`
	ForceHTTPS   bool   `json:"force_https"`
	IsPrimary    bool   `json:"is_primary"`
}

// CreateDomainResponse represents the response from creating a domain
type CreateDomainResponse struct {
	ID                 string      `json:"id"`
	TenantID           string      `json:"tenant_id"`
	Domain             string      `json:"domain"`
	DomainType         string      `json:"domain_type"`
	Status             string      `json:"status"`
	VerificationMethod string      `json:"verification_method"`
	VerificationToken  string      `json:"verification_token"`
	VerificationRecord DNSRecord   `json:"verification_record,omitempty"`
	DNSRecords         []DNSRecord `json:"dns_records,omitempty"` // All DNS records including routing and CNAME delegation
	DNSVerified        bool        `json:"dns_verified"`
	SSLStatus          string      `json:"ssl_status"`
	CreatedAt          string      `json:"created_at"`
	Error              string      `json:"error,omitempty"`

	// CNAME delegation for automatic SSL
	CNAMEDelegationEnabled bool `json:"cname_delegation_enabled,omitempty"`
}

// DNSRecord represents a DNS record for domain verification
type DNSRecord struct {
	RecordType string `json:"record_type"`
	Host       string `json:"host"`
	Value      string `json:"value"`
	TTL        int    `json:"ttl"`
}

// DomainAPIResponse wraps the response from custom-domain-service
type DomainAPIResponse struct {
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error,omitempty"`
	Message string          `json:"message,omitempty"`
}

// CreateDomain creates a new custom domain for a tenant
// This is called during tenant provisioning if the user specified a custom domain
func (c *CustomDomainClient) CreateDomain(ctx context.Context, tenantID string, req *CreateDomainRequest) (*CreateDomainResponse, error) {
	url := fmt.Sprintf("%s/api/v1/domains", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call custom-domain-service: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the API response wrapper
	var apiResp DomainAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		if apiResp.Error != "" {
			return nil, fmt.Errorf("custom-domain-service error: %s", apiResp.Error)
		}
		return nil, fmt.Errorf("custom-domain-service returned status %d", resp.StatusCode)
	}

	// Parse the domain data
	var response CreateDomainResponse
	if len(apiResp.Data) > 0 {
		if err := json.Unmarshal(apiResp.Data, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal domain response: %w", err)
		}
	}

	log.Printf("[CustomDomainClient] Created domain %s for tenant %s", req.Domain, tenantID)
	return &response, nil
}

// ValidateDomainRequest represents the request to validate a domain
type ValidateDomainRequest struct {
	Domain   string `json:"domain"`
	CheckDNS bool   `json:"check_dns"`
}

// ValidateDomainResponse represents the response from validating a domain
type ValidateDomainResponse struct {
	Valid              bool        `json:"valid"`
	Available          bool        `json:"available"`
	DNSConfigured      bool        `json:"dns_configured"`
	Message            string      `json:"message,omitempty"`
	VerificationRecord *DNSRecord  `json:"verification_record,omitempty"`
	DomainType         string      `json:"domain_type,omitempty"`
	VerificationToken  string      `json:"verification_token,omitempty"`
	VerificationID     string      `json:"verification_id,omitempty"`

	// Complete DNS configuration for customer setup
	RoutingRecords         []DNSRecord `json:"routing_records,omitempty"`          // CNAME records for routing traffic
	CNAMEDelegationRecord  *DNSRecord  `json:"cname_delegation_record,omitempty"`  // CNAME for automatic SSL
	CNAMEDelegationEnabled bool        `json:"cname_delegation_enabled"`           // Whether CNAME delegation is available
	ProxyTarget            string      `json:"proxy_target,omitempty"`             // Target for routing
}

// ValidateDomain validates a domain before creating it
func (c *CustomDomainClient) ValidateDomain(ctx context.Context, domain string) (*ValidateDomainResponse, error) {
	url := fmt.Sprintf("%s/api/v1/domains/validate", c.baseURL)

	req := ValidateDomainRequest{
		Domain:   domain,
		CheckDNS: false, // Don't check DNS during validation, just format
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Graceful degradation - if service is unavailable, allow the request
		log.Printf("[CustomDomainClient] Warning: custom-domain-service unavailable for validation: %v", err)
		return &ValidateDomainResponse{Valid: true, Available: true}, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp DomainAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	var response ValidateDomainResponse
	if len(apiResp.Data) > 0 {
		if err := json.Unmarshal(apiResp.Data, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal validation response: %w", err)
		}
	}

	return &response, nil
}
