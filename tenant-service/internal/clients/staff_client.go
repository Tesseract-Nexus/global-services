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

// StaffClient handles communication with the staff service
// Used to bootstrap owner RBAC roles when tenants are created
type StaffClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewStaffClient creates a new staff service client
func NewStaffClient(baseURL string) *StaffClient {
	return &StaffClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// BootstrapOwnerRequest represents a request to bootstrap an owner for a tenant
type BootstrapOwnerRequest struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// BootstrapOwnerResponse represents the response from bootstrapping an owner
type BootstrapOwnerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	StaffID string `json:"staff_id,omitempty"`
	RoleID  string `json:"role_id,omitempty"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// BootstrapOwner creates the owner staff record and assigns the Owner role
// This should be called during tenant onboarding after the tenant and membership are created
func (c *StaffClient) BootstrapOwner(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, email, firstName, lastName string) (*BootstrapOwnerResponse, error) {
	req := BootstrapOwnerRequest{
		UserID:    userID.String(),
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/internal/bootstrap-owner", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID.String())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response BootstrapOwnerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if response.Error != nil {
			return nil, fmt.Errorf("staff service error: %s - %s", response.Error.Code, response.Error.Message)
		}
		return nil, fmt.Errorf("staff service returned status %d: %s", resp.StatusCode, string(body))
	}

	if !response.Success {
		if response.Error != nil {
			return nil, fmt.Errorf("bootstrap failed: %s - %s", response.Error.Code, response.Error.Message)
		}
		return nil, fmt.Errorf("bootstrap failed: unknown error")
	}

	return &response, nil
}

// StaffTenantInfo represents tenant info from staff service
// Now includes enriched tenant data (slug, name, logo_url) fetched from tenant-service
type StaffTenantInfo struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug,omitempty"`
	Name        string     `json:"name,omitempty"`
	DisplayName string     `json:"display_name,omitempty"`
	LogoURL     string     `json:"logo_url,omitempty"`
	StaffID     uuid.UUID  `json:"staff_id"`
	Role        string     `json:"role"`
	VendorID    *uuid.UUID `json:"vendor_id,omitempty"`
}

// GetStaffTenantsResponse represents the response from getting staff tenants
type GetStaffTenantsResponse struct {
	Success bool `json:"success"`
	Data    *struct {
		Tenants []StaffTenantInfo `json:"tenants"`
		Count   int               `json:"count"`
	} `json:"data,omitempty"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GetStaffTenants returns all tenants a staff member has access to by email
// Used for login tenant lookup to include staff members
func (c *StaffClient) GetStaffTenants(ctx context.Context, email string) ([]StaffTenantInfo, error) {
	jsonBody, err := json.Marshal(map[string]string{"email": email})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/auth/tenants", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response GetStaffTenantsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if response.Error != nil {
			return nil, fmt.Errorf("staff service error: %s - %s", response.Error.Code, response.Error.Message)
		}
		return nil, fmt.Errorf("staff service returned status %d: %s", resp.StatusCode, string(body))
	}

	if response.Data == nil {
		return []StaffTenantInfo{}, nil
	}

	return response.Data.Tenants, nil
}

// StaffMemberInfo represents info about a staff member for authentication
type StaffMemberInfo struct {
	ID             uuid.UUID  `json:"id"`
	Email          string     `json:"email"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	KeycloakUserID string     `json:"keycloak_user_id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	AccountStatus  string     `json:"account_status"`
	IsActive       bool       `json:"is_active"`
}

// GetStaffByEmailResponse represents the response from getting staff by email
type GetStaffByEmailResponse struct {
	Success bool `json:"success"`
	Data    *struct {
		Staff StaffMemberInfo `json:"staff"`
	} `json:"data,omitempty"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GetStaffTenantsById returns all tenants a staff member has access to by their staff ID
// Used for /users/me/tenants when user is not in tenant_users (they're staff)
func (c *StaffClient) GetStaffTenantsById(ctx context.Context, staffID uuid.UUID) ([]StaffTenantInfo, error) {
	url := fmt.Sprintf("%s/api/v1/internal/staff/%s/tenants", c.baseURL, staffID.String())
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Staff not found - not an error
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response GetStaffTenantsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if response.Error != nil {
			return nil, fmt.Errorf("staff service error: %s - %s", response.Error.Code, response.Error.Message)
		}
		return nil, fmt.Errorf("staff service returned status %d: %s", resp.StatusCode, string(body))
	}

	if response.Data == nil {
		return []StaffTenantInfo{}, nil
	}

	return response.Data.Tenants, nil
}

// GetStaffByEmailForTenant returns staff member info by email for a specific tenant
// Used for credential validation when user is not in tenant_users table
func (c *StaffClient) GetStaffByEmailForTenant(ctx context.Context, email string, tenantID uuid.UUID) (*StaffMemberInfo, error) {
	url := fmt.Sprintf("%s/api/v1/internal/staff/by-email?email=%s", c.baseURL, email)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID.String())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Staff not found - not an error
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response GetStaffByEmailResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if response.Error != nil {
			return nil, fmt.Errorf("staff service error: %s - %s", response.Error.Code, response.Error.Message)
		}
		return nil, fmt.Errorf("staff service returned status %d: %s", resp.StatusCode, string(body))
	}

	if response.Data == nil {
		return nil, nil
	}

	return &response.Data.Staff, nil
}

// SyncKeycloakUserID updates the keycloak_user_id for a staff member after successful login
// This ensures the stored keycloak_user_id matches the actual Keycloak user
func (c *StaffClient) SyncKeycloakUserID(ctx context.Context, tenantID, staffID uuid.UUID, keycloakUserID string) error {
	req := map[string]string{
		"staff_id":         staffID.String(),
		"keycloak_user_id": keycloakUserID,
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/internal/staff/sync-keycloak-id", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", tenantID.String())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("staff service returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
