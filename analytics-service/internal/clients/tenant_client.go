package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TenantClient resolves tenant slugs to UUIDs
type TenantClient interface {
	// ResolveTenantID resolves a tenant slug or UUID to the actual tenant UUID
	// If the input is already a UUID, it's returned as-is
	// If the input is a slug, it fetches the tenant info and returns the UUID
	ResolveTenantID(ctx context.Context, tenantSlugOrID string) (string, error)
}

// TenantInfo holds tenant information
type TenantInfo struct {
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	CachedAt time.Time
}

type tenantClient struct {
	baseURL    string
	cache      map[string]*TenantInfo
	slugCache  map[string]*TenantInfo // slug -> TenantInfo cache
	cacheTTL   time.Duration
	mu         sync.RWMutex
	httpClient *http.Client
}

// NewTenantClient creates a new tenant client
func NewTenantClient() TenantClient {
	baseURL := os.Getenv("TENANT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tenant-service.devtest.svc.cluster.local:8080"
	}

	return &tenantClient{
		baseURL:   baseURL,
		cache:     make(map[string]*TenantInfo),
		slugCache: make(map[string]*TenantInfo),
		cacheTTL:  15 * time.Minute,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ResolveTenantID resolves a tenant slug or UUID to the actual tenant UUID
func (c *tenantClient) ResolveTenantID(ctx context.Context, tenantSlugOrID string) (string, error) {
	if tenantSlugOrID == "" {
		return "", fmt.Errorf("tenant ID or slug is required")
	}

	// Check if it's already a valid UUID
	if _, err := uuid.Parse(tenantSlugOrID); err == nil {
		// It's already a UUID, return it as-is
		return tenantSlugOrID, nil
	}

	// It's a slug, resolve it to a UUID
	// Check cache first
	c.mu.RLock()
	if info, ok := c.slugCache[tenantSlugOrID]; ok && time.Since(info.CachedAt) < c.cacheTTL {
		c.mu.RUnlock()
		return info.ID, nil
	}
	c.mu.RUnlock()

	// Fetch from tenant-service
	info, err := c.fetchTenantBySlug(ctx, tenantSlugOrID)
	if err != nil {
		return "", err
	}

	// Cache result
	c.mu.Lock()
	c.slugCache[tenantSlugOrID] = info
	c.cache[info.ID] = info
	c.mu.Unlock()

	return info.ID, nil
}

func (c *tenantClient) fetchTenantBySlug(ctx context.Context, slug string) (*TenantInfo, error) {
	// Use the /api/v1/tenants/:id/context endpoint which supports both slug and UUID
	// Note: We'll try the internal endpoint first for service-to-service calls
	url := fmt.Sprintf("%s/internal/tenants/by-slug/%s", c.baseURL, slug)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Internal-Service", "analytics-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tenant: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("tenant not found for slug: %s", slug)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status from tenant-service: %d", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
			Name string `json:"name"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success || result.Data.ID == "" {
		return nil, fmt.Errorf("tenant not found for slug: %s", slug)
	}

	return &TenantInfo{
		ID:       result.Data.ID,
		Slug:     result.Data.Slug,
		Name:     result.Data.Name,
		CachedAt: time.Now(),
	}, nil
}
