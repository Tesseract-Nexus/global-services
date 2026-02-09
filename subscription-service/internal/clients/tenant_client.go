package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type TenantClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewTenantClient() *TenantClient {
	baseURL := os.Getenv("TENANT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tenant-service.marketplace.svc.cluster.local:8080"
	}
	return &TenantClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (c *TenantClient) UpdatePricingTier(ctx context.Context, tenantID, pricingTier string) error {
	payload := map[string]string{
		"pricing_tier": pricingTier,
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("%s/internal/tenants/%s", c.baseURL, tenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	internalKey := os.Getenv("INTERNAL_SERVICE_KEY")
	if internalKey != "" {
		req.Header.Set("X-Internal-Service-Key", internalKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call tenant-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("tenant-service returned status %d", resp.StatusCode)
	}

	return nil
}
