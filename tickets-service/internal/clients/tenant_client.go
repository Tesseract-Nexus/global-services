package clients

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// TenantClient fetches tenant info for URL building
type TenantClient struct {
	baseURL    string
	baseDomain string
	cache      map[string]*TenantInfo
	cacheTTL   time.Duration
	mu         sync.RWMutex
	httpClient *http.Client
}

// TenantInfo holds tenant information
type TenantInfo struct {
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	CachedAt time.Time
}

// NewTenantClient creates a new tenant client
func NewTenantClient() *TenantClient {
	baseURL := os.Getenv("TENANT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tenant-service.marketplace.svc.cluster.local:8080"
	}

	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}

	// Create optimized transport with connection pooling for high-throughput
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return &TenantClient{
		baseURL:    baseURL,
		baseDomain: baseDomain,
		cache:      make(map[string]*TenantInfo),
		cacheTTL:   15 * time.Minute,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
		},
	}
}

// GetTenantSlug returns the tenant slug, with caching
func (c *TenantClient) GetTenantSlug(ctx context.Context, tenantID string) string {
	if tenantID == "" {
		return "store"
	}

	// Check cache
	c.mu.RLock()
	if info, ok := c.cache[tenantID]; ok && time.Since(info.CachedAt) < c.cacheTTL {
		c.mu.RUnlock()
		return info.Slug
	}
	c.mu.RUnlock()

	// Fetch from tenant-service
	slug := c.fetchTenantSlug(ctx, tenantID)

	// Cache result
	c.mu.Lock()
	c.cache[tenantID] = &TenantInfo{
		ID:       tenantID,
		Slug:     slug,
		CachedAt: time.Now(),
	}
	c.mu.Unlock()

	return slug
}

func (c *TenantClient) fetchTenantSlug(ctx context.Context, tenantID string) string {
	url := fmt.Sprintf("%s/internal/tenants/%s", c.baseURL, tenantID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "store" // fallback
	}
	req.Header.Set("X-Internal-Service", "tickets-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "store" // fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "store" // fallback
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "store"
	}

	if result.Data.Slug == "" {
		return "store"
	}

	return result.Data.Slug
}

// BuildTicketURL builds the admin URL for a ticket
func (c *TenantClient) BuildTicketURL(ctx context.Context, tenantID, ticketID string) string {
	slug := c.GetTenantSlug(ctx, tenantID)
	return fmt.Sprintf("https://%s-admin.%s/support/%s", slug, c.baseDomain, ticketID)
}
