package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// TenantClient handles HTTP communication with tenant-service for slug lookups
type TenantClient struct {
	baseURL       string
	baseDomain    string
	idToSlugCache map[string]*TenantInfo  // tenantID -> slug
	slugToIDCache map[string]*TenantIDInfo // slug -> tenantID
	cacheTTL      time.Duration
	mu            sync.RWMutex
	httpClient    *http.Client
}

// TenantInfo contains cached tenant information (ID -> slug mapping)
type TenantInfo struct {
	Slug      string
	ExpiresAt time.Time
}

// TenantIDInfo contains cached tenant ID information (slug -> ID mapping)
type TenantIDInfo struct {
	ID        string
	ExpiresAt time.Time
}

// tenantResponse is the API response format from tenant-service
type tenantResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID          string `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		Subdomain   string `json:"subdomain"`
	} `json:"data"`
}

// NewTenantClient creates a new tenant client
func NewTenantClient() *TenantClient {
	baseURL := os.Getenv("TENANT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tenant-service.devtest.svc.cluster.local:8087"
	}

	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}

	return &TenantClient{
		baseURL:       baseURL,
		baseDomain:    baseDomain,
		idToSlugCache: make(map[string]*TenantInfo),
		slugToIDCache: make(map[string]*TenantIDInfo),
		cacheTTL:      15 * time.Minute,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetTenantSlug fetches the tenant slug from tenant-service with caching
func (c *TenantClient) GetTenantSlug(ctx context.Context, tenantID string) string {
	// Check cache first
	c.mu.RLock()
	if info, ok := c.idToSlugCache[tenantID]; ok && time.Now().Before(info.ExpiresAt) {
		c.mu.RUnlock()
		return info.Slug
	}
	c.mu.RUnlock()

	// Fetch from tenant-service
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/internal/tenants/%s", c.baseURL, tenantID), nil)
	if err != nil {
		log.Printf("[TENANT] Failed to create request: %v", err)
		return tenantID
	}

	req.Header.Set("X-Internal-Service", "auth-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[TENANT] Failed to fetch tenant %s: %v", tenantID, err)
		return tenantID
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[TENANT] Non-200 response for tenant %s: %d", tenantID, resp.StatusCode)
		return tenantID
	}

	var tenantResp tenantResponse
	if err := json.NewDecoder(resp.Body).Decode(&tenantResp); err != nil {
		log.Printf("[TENANT] Failed to decode response: %v", err)
		return tenantID
	}

	slug := tenantResp.Data.Slug
	if slug == "" {
		slug = tenantID
	}

	// Update both caches (bidirectional)
	c.mu.Lock()
	c.idToSlugCache[tenantID] = &TenantInfo{
		Slug:      slug,
		ExpiresAt: time.Now().Add(c.cacheTTL),
	}
	if tenantResp.Data.ID != "" {
		c.slugToIDCache[slug] = &TenantIDInfo{
			ID:        tenantResp.Data.ID,
			ExpiresAt: time.Now().Add(c.cacheTTL),
		}
	}
	c.mu.Unlock()

	return slug
}

// GetTenantID fetches the tenant ID from a slug with caching
func (c *TenantClient) GetTenantID(ctx context.Context, slug string) (string, error) {
	// Check cache first
	c.mu.RLock()
	if info, ok := c.slugToIDCache[slug]; ok && time.Now().Before(info.ExpiresAt) {
		c.mu.RUnlock()
		return info.ID, nil
	}
	c.mu.RUnlock()

	// Fetch from tenant-service using the by-slug endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/internal/tenants/by-slug/%s", c.baseURL, slug), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Internal-Service", "auth-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch tenant by slug %s: %w", slug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("tenant with slug '%s' not found", slug)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d for tenant slug %s", resp.StatusCode, slug)
	}

	var tenantResp tenantResponse
	if err := json.NewDecoder(resp.Body).Decode(&tenantResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	tenantID := tenantResp.Data.ID
	if tenantID == "" {
		return "", fmt.Errorf("tenant-service returned empty ID for slug %s", slug)
	}

	// Update both caches (bidirectional)
	c.mu.Lock()
	c.slugToIDCache[slug] = &TenantIDInfo{
		ID:        tenantID,
		ExpiresAt: time.Now().Add(c.cacheTTL),
	}
	if tenantResp.Data.Slug != "" {
		c.idToSlugCache[tenantID] = &TenantInfo{
			Slug:      tenantResp.Data.Slug,
			ExpiresAt: time.Now().Add(c.cacheTTL),
		}
	}
	c.mu.Unlock()

	return tenantID, nil
}

// isValidUUID checks if a string is a valid UUID format
func isValidUUID(s string) bool {
	// UUID format: 8-4-4-4-12 hex characters with dashes
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// ResolveTenantIdentifier accepts either a UUID or slug and returns the UUID
// This allows X-Tenant-ID header to accept "demo-store" or "36beed50-b1f8-4ad0-a735-38da903719cb"
func (c *TenantClient) ResolveTenantIdentifier(ctx context.Context, identifier string) (string, error) {
	if identifier == "" {
		return "", fmt.Errorf("tenant identifier is empty")
	}

	// If it's already a valid UUID, return as-is
	if isValidUUID(identifier) {
		return identifier, nil
	}

	// Otherwise, treat as slug and resolve to UUID
	return c.GetTenantID(ctx, identifier)
}

// BuildStorefrontURL builds the tenant storefront URL
func (c *TenantClient) BuildStorefrontURL(ctx context.Context, tenantID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s.%s", slug, c.baseDomain)
}

// BuildAdminURL builds the tenant admin URL
func (c *TenantClient) BuildAdminURL(ctx context.Context, tenantID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s-admin.%s", slug, c.baseDomain)
}

// BuildPasswordResetURL builds the password reset URL
func (c *TenantClient) BuildPasswordResetURL(ctx context.Context, tenantID string, token string) string {
	storefrontURL := c.BuildStorefrontURL(ctx, tenantID)
	return fmt.Sprintf("%s/reset-password?token=%s", storefrontURL, token)
}
