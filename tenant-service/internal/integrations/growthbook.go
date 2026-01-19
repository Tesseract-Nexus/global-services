package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// GrowthBookConfig holds configuration for GrowthBook API
type GrowthBookConfig struct {
	APIHost       string
	AdminEmail    string
	AdminPassword string
}

// GrowthBookClient handles communication with GrowthBook API
type GrowthBookClient struct {
	config     GrowthBookConfig
	httpClient *http.Client
	token      string
	tokenExp   time.Time
}

// GrowthBookProvisionResult contains the result of provisioning a tenant org
type GrowthBookProvisionResult struct {
	OrgID    string `json:"org_id"`
	SDKKey   string `json:"sdk_key"`
	AdminKey string `json:"admin_key,omitempty"` // For future use with API keys
}

// DefaultFeatureFlag represents a feature flag to seed
type DefaultFeatureFlag struct {
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	DefaultValue bool     `json:"defaultValue"`
	Tags         []string `json:"tags"`
}

// NewGrowthBookClient creates a new GrowthBook API client
func NewGrowthBookClient() *GrowthBookClient {
	return &GrowthBookClient{
		config: GrowthBookConfig{
			APIHost:       getEnvOrDefault("GROWTHBOOK_API_HOST", "http://growthbook.growthbook.svc.cluster.local:3100"),
			AdminEmail:    getEnvOrDefault("GROWTHBOOK_ADMIN_EMAIL", "admin@tesserix.app"),
			AdminPassword: os.Getenv("GROWTHBOOK_ADMIN_PASSWORD"),
		},
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewGrowthBookClientWithConfig creates a client with custom config
func NewGrowthBookClientWithConfig(config GrowthBookConfig) *GrowthBookClient {
	return &GrowthBookClient{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// getEnvOrDefault returns env var value or default
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// authenticate gets an auth token from GrowthBook
func (c *GrowthBookClient) authenticate() error {
	// Return cached token if still valid
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return nil
	}

	if c.config.AdminPassword == "" {
		return fmt.Errorf("GROWTHBOOK_ADMIN_PASSWORD not configured")
	}

	payload := map[string]string{
		"email":    c.config.AdminEmail,
		"password": c.config.AdminPassword,
	}
	body, _ := json.Marshal(payload)

	resp, err := c.httpClient.Post(
		c.config.APIHost+"/auth/login",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to login to GrowthBook: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if result.Token == "" {
		return fmt.Errorf("no token received from GrowthBook login")
	}

	c.token = result.Token
	c.tokenExp = time.Now().Add(25 * time.Minute) // Tokens expire in 30min
	return nil
}

// doRequest performs an authenticated HTTP request
func (c *GrowthBookClient) doRequest(method, path string, body interface{}, orgID string) ([]byte, int, error) {
	if err := c.authenticate(); err != nil {
		return nil, 0, err
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.config.APIHost+path, reqBody)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	if orgID != "" {
		req.Header.Set("x-organization", orgID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

// ProvisionTenantOrg creates a new GrowthBook organization for a tenant
func (c *GrowthBookClient) ProvisionTenantOrg(tenantSlug, tenantName string) (*GrowthBookProvisionResult, error) {
	// Step 1: Create organization
	orgID, err := c.createOrganization(tenantSlug, tenantName)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	// Step 2: Create SDK connection
	sdkKey, err := c.createSDKConnection(orgID, tenantName+" SDK")
	if err != nil {
		return nil, fmt.Errorf("failed to create SDK connection: %w", err)
	}

	// Step 3: Seed default feature flags
	if err := c.seedDefaultFeatures(orgID); err != nil {
		// Log but don't fail - features can be added later
		fmt.Printf("[GrowthBook] Warning: failed to seed features for %s: %v\n", tenantSlug, err)
	}

	return &GrowthBookProvisionResult{
		OrgID:  orgID,
		SDKKey: sdkKey,
	}, nil
}

// createOrganization creates a new GrowthBook organization
func (c *GrowthBookClient) createOrganization(slug, name string) (string, error) {
	payload := map[string]interface{}{
		"company": name,
		"name":    slug, // Use slug as org name for uniqueness
	}

	body, status, err := c.doRequest("POST", "/organization", payload, "")
	if err != nil {
		return "", err
	}

	if status != 200 && status != 201 {
		return "", fmt.Errorf("failed to create org (status %d): %s", status, string(body))
	}

	var result struct {
		OrgID string `json:"orgId"`
		Org   struct {
			ID string `json:"id"`
		} `json:"organization"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	orgID := result.OrgID
	if orgID == "" {
		orgID = result.Org.ID
	}
	if orgID == "" {
		return "", fmt.Errorf("no organization ID returned")
	}

	return orgID, nil
}

// createSDKConnection creates an SDK connection for the organization
func (c *GrowthBookClient) createSDKConnection(orgID, name string) (string, error) {
	payload := map[string]interface{}{
		"name":           name,
		"languages":      []string{"javascript", "nocode-other", "react"},
		"environment":    "production",
		"projects":       []string{},
		"encryptPayload": false,
	}

	body, status, err := c.doRequest("POST", "/sdk-connections", payload, orgID)
	if err != nil {
		return "", err
	}

	if status != 200 && status != 201 {
		return "", fmt.Errorf("failed to create SDK connection (status %d): %s", status, string(body))
	}

	var result struct {
		Connection struct {
			Key string `json:"key"`
		} `json:"sdkConnection"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	sdkKey := result.Connection.Key
	if sdkKey == "" {
		sdkKey = result.Key
	}
	if sdkKey == "" {
		return "", fmt.Errorf("no SDK key returned")
	}

	return sdkKey, nil
}

// seedDefaultFeatures creates the default feature flags for a new tenant org
func (c *GrowthBookClient) seedDefaultFeatures(orgID string) error {
	features := getDefaultFeatureFlags()

	for _, feat := range features {
		payload := map[string]interface{}{
			"id":           feat.ID,
			"description":  feat.Description,
			"valueType":    "boolean",
			"defaultValue": feat.DefaultValue,
			"tags":         feat.Tags,
			"environmentSettings": map[string]interface{}{
				"production": map[string]interface{}{
					"enabled": true,
					"rules":   []interface{}{},
				},
			},
		}

		_, status, err := c.doRequest("POST", "/feature", payload, orgID)
		if err != nil {
			fmt.Printf("[GrowthBook] Warning: failed to create feature %s: %v\n", feat.ID, err)
			continue
		}

		if status != 200 && status != 201 {
			// Feature might already exist, that's OK
			continue
		}
	}

	return nil
}

// GetSDKKey retrieves the SDK key for an organization
func (c *GrowthBookClient) GetSDKKey(orgID string) (string, error) {
	body, status, err := c.doRequest("GET", "/sdk-connections", nil, orgID)
	if err != nil {
		return "", err
	}

	if status != 200 {
		return "", fmt.Errorf("failed to get SDK connections (status %d)", status)
	}

	var result struct {
		Connections []struct {
			Key string `json:"key"`
		} `json:"sdkConnections"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Connections) == 0 {
		return "", fmt.Errorf("no SDK connections found")
	}

	return result.Connections[0].Key, nil
}

// getDefaultFeatureFlags returns the list of default features to seed
func getDefaultFeatureFlags() []DefaultFeatureFlag {
	return []DefaultFeatureFlag{
		// Search features
		{ID: "global_search_enabled", Description: "Enable global search functionality", DefaultValue: true, Tags: []string{"search"}},
		{ID: "search_autocomplete", Description: "Enable search autocomplete suggestions", DefaultValue: true, Tags: []string{"search"}},
		{ID: "search_typo_tolerance", Description: "Enable typo tolerance in search", DefaultValue: true, Tags: []string{"search"}},
		{ID: "advanced_search_filters", Description: "Enable advanced search filters", DefaultValue: false, Tags: []string{"search"}},

		// E-commerce features
		{ID: "multi_currency", Description: "Enable multi-currency support", DefaultValue: true, Tags: []string{"ecommerce"}},
		{ID: "guest_checkout", Description: "Allow guest checkout", DefaultValue: true, Tags: []string{"ecommerce"}},
		{ID: "wishlist_enabled", Description: "Enable wishlist functionality", DefaultValue: true, Tags: []string{"ecommerce"}},
		{ID: "product_reviews", Description: "Enable product reviews", DefaultValue: true, Tags: []string{"ecommerce"}},
		{ID: "product_compare", Description: "Enable product comparison", DefaultValue: false, Tags: []string{"ecommerce"}},
		{ID: "product_recommendations", Description: "Enable product recommendations", DefaultValue: true, Tags: []string{"ecommerce"}},
		{ID: "recently_viewed", Description: "Show recently viewed products", DefaultValue: true, Tags: []string{"ecommerce"}},

		// Payment features
		{ID: "apple_pay", Description: "Enable Apple Pay", DefaultValue: false, Tags: []string{"payments"}},
		{ID: "google_pay", Description: "Enable Google Pay", DefaultValue: false, Tags: []string{"payments"}},
		{ID: "buy_now_pay_later", Description: "Enable BNPL options", DefaultValue: false, Tags: []string{"payments"}},
		{ID: "saved_payment_methods", Description: "Enable saved payment methods", DefaultValue: true, Tags: []string{"payments"}},

		// UI features
		{ID: "dark_mode", Description: "Enable dark mode", DefaultValue: false, Tags: []string{"ui"}},
		{ID: "product_quick_view", Description: "Enable product quick view", DefaultValue: true, Tags: []string{"ui"}},
		{ID: "sticky_header", Description: "Enable sticky header", DefaultValue: true, Tags: []string{"ui"}},
		{ID: "mega_menu", Description: "Enable mega menu navigation", DefaultValue: true, Tags: []string{"ui"}},
		{ID: "mini_cart", Description: "Enable mini cart dropdown", DefaultValue: true, Tags: []string{"ui"}},

		// Admin features
		{ID: "bulk_product_edit", Description: "Enable bulk product editing", DefaultValue: true, Tags: []string{"admin"}},
		{ID: "inventory_alerts", Description: "Enable inventory alerts", DefaultValue: true, Tags: []string{"admin"}},
		{ID: "audit_logs", Description: "Enable audit logs", DefaultValue: true, Tags: []string{"admin"}},
		{ID: "analytics_dashboard_v2", Description: "Enable analytics dashboard v2", DefaultValue: false, Tags: []string{"admin"}},

		// Mobile features
		{ID: "push_notifications", Description: "Enable push notifications", DefaultValue: true, Tags: []string{"mobile"}},
		{ID: "biometric_auth", Description: "Enable biometric auth", DefaultValue: false, Tags: []string{"mobile"}},

		// Performance features
		{ID: "image_lazy_loading", Description: "Enable lazy loading", DefaultValue: true, Tags: []string{"performance"}},
		{ID: "prefetch_enabled", Description: "Enable prefetching", DefaultValue: true, Tags: []string{"performance"}},

		// Multi-tenant features
		{ID: "tenant_custom_domain", Description: "Enable custom domains", DefaultValue: true, Tags: []string{"tenant"}},
		{ID: "tenant_custom_theme", Description: "Enable custom themes", DefaultValue: true, Tags: []string{"tenant"}},
		{ID: "tenant_analytics", Description: "Enable tenant analytics", DefaultValue: true, Tags: []string{"tenant"}},

		// Marketing features
		{ID: "coupons_enabled", Description: "Enable coupons", DefaultValue: true, Tags: []string{"marketing"}},
		{ID: "gift_cards_enabled", Description: "Enable gift cards", DefaultValue: true, Tags: []string{"marketing"}},
		{ID: "abandoned_cart_emails", Description: "Enable abandoned cart emails", DefaultValue: true, Tags: []string{"marketing"}},
	}
}
