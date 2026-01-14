package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// VendorClient handles communication with the vendor service
// Used to create vendor records when tenants are created
type VendorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewVendorClient creates a new vendor service client
func NewVendorClient(baseURL string) *VendorClient {
	return &VendorClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateVendorRequest represents a request to create a vendor
type CreateVendorRequest struct {
	ID             *uuid.UUID `json:"id,omitempty"`            // Optional: Use the same ID as the tenant
	Name           string     `json:"name"`                    // Required
	PrimaryContact string     `json:"primaryContact"`          // Required
	Email          string     `json:"email"`                   // Required
	CommissionRate float64    `json:"commissionRate"`          // Required, default 0
	IsOwnerVendor  *bool      `json:"isOwnerVendor,omitempty"` // True for tenant's own vendor
	Details        *string    `json:"details,omitempty"`
	Location       *string    `json:"location,omitempty"`
}

// CreateVendorResponse represents the response from creating a vendor
type CreateVendorResponse struct {
	Success bool        `json:"success"`
	Data    *VendorData `json:"data,omitempty"`
	Error   *ErrorData  `json:"error,omitempty"`
}

// VendorData represents the vendor data in the response
type VendorData struct {
	ID       uuid.UUID `json:"id"`
	TenantID string    `json:"tenantId"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Status   string    `json:"status"`
	IsActive bool      `json:"isActive"`
}

// ErrorData represents error details in the response
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CreateVendorForTenant creates a default vendor record for a tenant
// The vendor gets its own auto-generated ID and is linked to the tenant via TenantID
// Relationship: Tenant (1) ---> (N) Vendors ---> (N) Storefronts
func (c *VendorClient) CreateVendorForTenant(ctx context.Context, tenantID uuid.UUID, name, email, primaryContact string) (*VendorData, error) {
	isOwner := true
	req := CreateVendorRequest{
		// ID is NOT set - vendor service will auto-generate a unique vendor ID
		// The tenant relationship is established via X-Tenant-ID header
		Name:           name,
		PrimaryContact: primaryContact,
		Email:          email,
		CommissionRate: 0,        // Default commission rate
		IsOwnerVendor:  &isOwner, // Mark as owner vendor to skip marketplace role seeding
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use internal endpoint for service-to-service calls (bypasses RBAC)
	url := fmt.Sprintf("%s/internal/vendors", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	// Set the tenant ID header - vendor service uses this to set Vendor.TenantID
	httpReq.Header.Set("X-Tenant-ID", tenantID.String())
	httpReq.Header.Set("X-Vendor-ID", tenantID.String()) // For middleware compatibility

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp CreateVendorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error != nil {
			return nil, fmt.Errorf("vendor service error: %s - %s", errorResp.Error.Code, errorResp.Error.Message)
		}
		return nil, fmt.Errorf("vendor service returned status %d: %s", resp.StatusCode, string(body))
	}

	var response CreateVendorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !response.Success || response.Data == nil {
		if response.Error != nil {
			return nil, fmt.Errorf("vendor creation failed: %s - %s", response.Error.Code, response.Error.Message)
		}
		return nil, fmt.Errorf("vendor creation failed: unknown error")
	}

	return response.Data, nil
}

// GetVendor retrieves a vendor by ID
func (c *VendorClient) GetVendor(ctx context.Context, vendorID uuid.UUID) (*VendorData, error) {
	url := fmt.Sprintf("%s/api/v1/vendors/%s", c.baseURL, vendorID.String())
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-Vendor-ID", vendorID.String())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Vendor doesn't exist
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vendor service returned status %d", resp.StatusCode)
	}

	var response CreateVendorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.Data, nil
}

// EnsureVendorExists checks if a vendor exists for the tenant, creates one if not
func (c *VendorClient) EnsureVendorExists(ctx context.Context, tenantID uuid.UUID, name, email, primaryContact string) (*VendorData, error) {
	// First check if vendor already exists
	existing, err := c.GetVendor(ctx, tenantID)
	if err != nil {
		// Log error but continue to try creating
		fmt.Printf("Warning: error checking existing vendor: %v\n", err)
	}
	if existing != nil {
		return existing, nil
	}

	// Create vendor
	return c.CreateVendorForTenant(ctx, tenantID, name, email, primaryContact)
}

// CreateStorefrontRequest represents a request to create a storefront
type CreateStorefrontRequest struct {
	VendorID    uuid.UUID `json:"vendorId"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	IsDefault   bool      `json:"isDefault"`
}

// StorefrontData represents the storefront data in the response
type StorefrontData struct {
	ID        uuid.UUID `json:"id"`
	VendorID  uuid.UUID `json:"vendorId"`
	TenantID  string    `json:"tenantId"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	IsDefault bool      `json:"isDefault"`
	IsActive  bool      `json:"isActive"`
}

// CreateStorefrontResponse represents the response from creating a storefront
type CreateStorefrontResponse struct {
	Success bool            `json:"success"`
	Data    *StorefrontData `json:"data,omitempty"`
	Error   *ErrorData      `json:"error,omitempty"`
}

// CreateStorefront creates a storefront for a vendor
// URL pattern: {slug}-store.{baseDomain}
func (c *VendorClient) CreateStorefront(ctx context.Context, tenantID, vendorID uuid.UUID, name, slug string, isDefault bool) (*StorefrontData, error) {
	req := CreateStorefrontRequest{
		VendorID:  vendorID,
		Name:      name,
		Slug:      slug,
		IsDefault: isDefault,
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use internal endpoint for service-to-service calls (bypasses RBAC)
	url := fmt.Sprintf("%s/internal/storefronts", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID.String())
	httpReq.Header.Set("X-Vendor-ID", vendorID.String())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp CreateStorefrontResponse
		if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error != nil {
			return nil, fmt.Errorf("vendor service error: %s - %s", errorResp.Error.Code, errorResp.Error.Message)
		}
		return nil, fmt.Errorf("vendor service returned status %d: %s", resp.StatusCode, string(body))
	}

	var response CreateStorefrontResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !response.Success || response.Data == nil {
		if response.Error != nil {
			return nil, fmt.Errorf("storefront creation failed: %s - %s", response.Error.Code, response.Error.Message)
		}
		return nil, fmt.Errorf("storefront creation failed: unknown error")
	}

	return response.Data, nil
}
