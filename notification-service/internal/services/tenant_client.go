package services

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// TenantInfo holds cached tenant information
type TenantInfo struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	AdminURL      string `json:"adminUrl"`
	StorefrontURL string `json:"storefrontUrl"`
	SupportEmail  string `json:"supportEmail"`
	BusinessName  string `json:"businessName"`
	CachedAt      time.Time
}

// TenantClient provides tenant information with caching
type TenantClient struct {
	baseURL    string
	baseDomain string
	cache      map[string]*TenantInfo
	cacheTTL   time.Duration
	mu         sync.RWMutex
	httpClient *http.Client
}

// NewTenantClient creates a new tenant client
func NewTenantClient() *TenantClient {
	baseURL := os.Getenv("TENANT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tenant-service.devtest.svc.cluster.local:8080"
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
		cacheTTL:   15 * time.Minute, // Cache tenant info for 15 minutes
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
		},
	}
}

// GetTenantInfo retrieves tenant info by ID, using cache when possible
func (c *TenantClient) GetTenantInfo(tenantID string) (*TenantInfo, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant ID is empty")
	}

	// Check cache first
	c.mu.RLock()
	if info, ok := c.cache[tenantID]; ok {
		if time.Since(info.CachedAt) < c.cacheTTL {
			c.mu.RUnlock()
			return info, nil
		}
	}
	c.mu.RUnlock()

	// Fetch from tenant-service
	info, err := c.fetchTenantInfo(tenantID)
	if err != nil {
		// If fetch fails, check if we have stale cache
		c.mu.RLock()
		if staleInfo, ok := c.cache[tenantID]; ok {
			c.mu.RUnlock()
			log.Printf("[TenantClient] Using stale cache for tenant %s: %v", tenantID, err)
			return staleInfo, nil
		}
		c.mu.RUnlock()

		// Return default info as fallback
		return c.getDefaultTenantInfo(tenantID), nil
	}

	// Update cache
	c.mu.Lock()
	c.cache[tenantID] = info
	c.mu.Unlock()

	return info, nil
}

// fetchTenantInfo fetches tenant info from tenant-service internal API
func (c *TenantClient) fetchTenantInfo(tenantID string) (*TenantInfo, error) {
	url := fmt.Sprintf("%s/internal/tenants/%s", c.baseURL, tenantID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Service", "notification-service")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tenant: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tenant-service returned status %d", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			ID           string `json:"id"`
			Slug         string `json:"slug"`
			Name         string `json:"name"`
			DisplayName  string `json:"displayName"`
			Subdomain    string `json:"subdomain"`
			BillingEmail string `json:"billingEmail"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success || result.Data.Slug == "" {
		return nil, fmt.Errorf("invalid tenant response")
	}

	// Use DisplayName as business name, fallback to Name
	businessName := result.Data.DisplayName
	if businessName == "" {
		businessName = result.Data.Name
	}

	info := &TenantInfo{
		ID:            result.Data.ID,
		Slug:          result.Data.Slug,
		Name:          result.Data.Name,
		AdminURL:      c.buildAdminURL(result.Data.Slug),
		StorefrontURL: c.buildStorefrontURL(result.Data.Slug),
		SupportEmail:  result.Data.BillingEmail, // Use billing email as support email
		BusinessName:  businessName,
		CachedAt:      time.Now(),
	}

	log.Printf("[TenantClient] Fetched tenant %s: slug=%s, adminURL=%s", tenantID, info.Slug, info.AdminURL)
	return info, nil
}

// getDefaultTenantInfo returns default tenant info when lookup fails
func (c *TenantClient) getDefaultTenantInfo(tenantID string) *TenantInfo {
	log.Printf("[TenantClient] Using default tenant info for %s", tenantID)
	return &TenantInfo{
		ID:            tenantID,
		Slug:          "store",
		Name:          "Store",
		AdminURL:      fmt.Sprintf("https://admin.%s", c.baseDomain),
		StorefrontURL: fmt.Sprintf("https://store.%s", c.baseDomain),
		CachedAt:      time.Now(),
	}
}

// buildAdminURL constructs the admin URL for a tenant
func (c *TenantClient) buildAdminURL(slug string) string {
	return fmt.Sprintf("https://%s-admin.%s", slug, c.baseDomain)
}

// buildStorefrontURL constructs the storefront URL for a tenant
func (c *TenantClient) buildStorefrontURL(slug string) string {
	return fmt.Sprintf("https://%s.%s", slug, c.baseDomain)
}

// BuildTicketURL constructs a ticket URL for a tenant
func (c *TenantClient) BuildTicketURL(tenantID, ticketID string) string {
	info, _ := c.GetTenantInfo(tenantID)
	return fmt.Sprintf("%s/support/%s", info.AdminURL, ticketID)
}

// BuildOrderURL constructs an order URL for a tenant (admin view)
func (c *TenantClient) BuildOrderURL(tenantID, orderID string) string {
	info, _ := c.GetTenantInfo(tenantID)
	return fmt.Sprintf("%s/orders/%s", info.AdminURL, orderID)
}

// BuildStorefrontOrderURL constructs an order URL for customer (storefront view)
func (c *TenantClient) BuildStorefrontOrderURL(tenantID, orderID string) string {
	info, _ := c.GetTenantInfo(tenantID)
	return fmt.Sprintf("%s/account/orders/%s", info.StorefrontURL, orderID)
}

// ClearCache clears the tenant cache
func (c *TenantClient) ClearCache() {
	c.mu.Lock()
	c.cache = make(map[string]*TenantInfo)
	c.mu.Unlock()
}
