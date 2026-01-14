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

// TenantRouterClient handles communication with the tenant-router-service
// Used to check slug availability including recently deleted slugs
type TenantRouterClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewTenantRouterClient creates a new tenant-router service client
func NewTenantRouterClient(baseURL string) *TenantRouterClient {
	return &TenantRouterClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SlugAvailabilityResponse represents the response from slug availability check
type SlugAvailabilityResponse struct {
	Slug           string    `json:"slug"`
	Available      bool      `json:"available"`
	Reason         string    `json:"reason,omitempty"`         // "slug_in_use", "recently_deleted"
	Message        string    `json:"message,omitempty"`        // Human-readable message
	DeletedAt      *string   `json:"deleted_at,omitempty"`     // When the slug was deleted
	AvailableAfter *string   `json:"available_after,omitempty"` // When it will be available
	DaysRemaining  *int      `json:"days_remaining,omitempty"` // Days until available
}

// CheckSlugAvailability checks if a slug is available considering recently deleted slugs
// Returns nil if the slug is available, or availability info if it's blocked
func (c *TenantRouterClient) CheckSlugAvailability(ctx context.Context, slug string) (*SlugAvailabilityResponse, error) {
	url := fmt.Sprintf("%s/api/v1/slugs/%s/availability", c.baseURL, slug)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// If tenant-router-service is unavailable, don't block the request
		// Log the error and return nil (slug is considered available from routing perspective)
		return nil, nil // Graceful degradation
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Service error - graceful degradation
		return nil, nil
	}

	var response SlugAvailabilityResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// IsSlugRecentlyDeleted checks if a slug was recently deleted and is in cooling-off period
// Returns true with remaining days if the slug is blocked, false if available
func (c *TenantRouterClient) IsSlugRecentlyDeleted(ctx context.Context, slug string) (bool, int, string, error) {
	resp, err := c.CheckSlugAvailability(ctx, slug)
	if err != nil || resp == nil {
		// Graceful degradation - if we can't check, assume it's available
		return false, 0, "", nil
	}

	if resp.Available {
		return false, 0, "", nil
	}

	if resp.Reason == "recently_deleted" && resp.DaysRemaining != nil {
		return true, *resp.DaysRemaining, resp.Message, nil
	}

	// Slug is blocked for other reasons (in_use)
	return false, 0, "", nil
}

// ProvisionTenantHostRequest represents the request to provision a tenant host
type ProvisionTenantHostRequest struct {
	Slug           string `json:"slug"`
	TenantID       string `json:"tenant_id"`
	AdminHost      string `json:"admin_host,omitempty"`
	StorefrontHost string `json:"storefront_host,omitempty"`
	Product        string `json:"product,omitempty"`
	BusinessName   string `json:"business_name,omitempty"`
	Email          string `json:"email,omitempty"`
}

// ProvisionTenantHostResponse represents the response from provisioning
type ProvisionTenantHostResponse struct {
	Message        string `json:"message"`
	Slug           string `json:"slug"`
	TenantID       string `json:"tenant_id"`
	AdminHost      string `json:"admin_host"`
	StorefrontHost string `json:"storefront_host"`
	Error          string `json:"error,omitempty"`
}

// ProvisionTenantHost calls tenant-router-service to provision routing for a tenant
// This is a fallback mechanism when NATS events might be delayed or lost
func (c *TenantRouterClient) ProvisionTenantHost(ctx context.Context, req *ProvisionTenantHostRequest) (*ProvisionTenantHostResponse, error) {
	url := fmt.Sprintf("%s/api/v1/hosts", c.baseURL)

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
		return nil, fmt.Errorf("failed to call tenant-router-service: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response ProvisionTenantHostResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tenant-router-service error: %s", response.Error)
	}

	log.Printf("[TenantRouterClient] Provisioned tenant host %s via HTTP fallback", req.Slug)
	return &response, nil
}
