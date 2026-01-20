package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"custom-domain-service/internal/config"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// TenantClient handles communication with the tenant service
type TenantClient struct {
	cfg        *config.Config
	httpClient *http.Client
}

// NewTenantClient creates a new tenant client
func NewTenantClient(cfg *config.Config) *TenantClient {
	return &TenantClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// TenantInfo represents tenant information from tenant service
type TenantInfo struct {
	ID               uuid.UUID `json:"id"`
	Slug             string    `json:"slug"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	Subdomain        string    `json:"subdomain"`
	CustomDomainSlot int       `json:"custom_domain_slot"`
	Plan             string    `json:"plan"`
	PlanFeatures     struct {
		MaxCustomDomains int  `json:"max_custom_domains"`
		CustomDomains    bool `json:"custom_domains"`
	} `json:"plan_features"`
}

// GetTenant retrieves tenant information by ID
func (t *TenantClient) GetTenant(ctx context.Context, tenantID uuid.UUID) (*TenantInfo, error) {
	url := fmt.Sprintf("%s/api/v1/internal/tenants/%s", t.cfg.Tenant.ServiceURL, tenantID.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("tenant not found")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get tenant: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tenant TenantInfo
	if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		return nil, fmt.Errorf("failed to decode tenant response: %w", err)
	}

	return &tenant, nil
}

// GetTenantBySlug retrieves tenant information by slug
func (t *TenantClient) GetTenantBySlug(ctx context.Context, slug string) (*TenantInfo, error) {
	url := fmt.Sprintf("%s/api/v1/internal/tenants/by-slug/%s", t.cfg.Tenant.ServiceURL, slug)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("tenant not found")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get tenant: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tenant TenantInfo
	if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		return nil, fmt.Errorf("failed to decode tenant response: %w", err)
	}

	return &tenant, nil
}

// CanAddCustomDomain checks if a tenant can add more custom domains
func (t *TenantClient) CanAddCustomDomain(ctx context.Context, tenantID uuid.UUID, currentDomainCount int64) (bool, int, error) {
	tenant, err := t.GetTenant(ctx, tenantID)
	if err != nil {
		log.Warn().Err(err).Str("tenant_id", tenantID.String()).Msg("Failed to get tenant info, using default limits")
		// Default to basic limit if tenant service is unavailable
		return currentDomainCount < 5, 5, nil
	}

	if !tenant.PlanFeatures.CustomDomains {
		return false, 0, nil
	}

	maxAllowed := tenant.PlanFeatures.MaxCustomDomains
	if maxAllowed == 0 {
		maxAllowed = 5 // Default
	}

	return currentDomainCount < int64(maxAllowed), maxAllowed, nil
}

// ValidateTenantStatus checks if tenant is active
func (t *TenantClient) ValidateTenantStatus(ctx context.Context, tenantID uuid.UUID) error {
	tenant, err := t.GetTenant(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to validate tenant: %w", err)
	}

	if tenant.Status != "active" {
		return fmt.Errorf("tenant is not active: status is %s", tenant.Status)
	}

	return nil
}

// NotifyDomainStatusChange notifies tenant service of domain status changes
func (t *TenantClient) NotifyDomainStatusChange(ctx context.Context, tenantID uuid.UUID, domain string, status string) error {
	url := fmt.Sprintf("%s/api/v1/internal/tenants/%s/domain-status", t.cfg.Tenant.ServiceURL, tenantID.String())

	payload := map[string]string{
		"domain": domain,
		"status": status,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Use a short timeout for notifications
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("tenant_id", tenantID.String()).Msg("Failed to notify tenant service")
		return nil // Non-critical, don't fail the operation
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Warn().
			Int("status", resp.StatusCode).
			Str("tenant_id", tenantID.String()).
			Msg("Tenant service returned error")
	}

	_ = payloadBytes // suppress unused warning
	return nil
}
