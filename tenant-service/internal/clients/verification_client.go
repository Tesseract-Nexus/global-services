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

// VerificationClient handles communication with the verification service
type VerificationClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewVerificationClient creates a new verification service client
func NewVerificationClient(baseURL, apiKey string) *VerificationClient {
	return &VerificationClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendVerificationCodeRequest represents a request to send a verification code
type SendVerificationCodeRequest struct {
	Recipient string                 `json:"recipient"`
	Channel   string                 `json:"channel"`
	Purpose   string                 `json:"purpose"`
	SessionID *uuid.UUID             `json:"session_id,omitempty"`
	TenantID  *uuid.UUID             `json:"tenant_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SendVerificationCodeResponse represents the response from sending a verification code
type SendVerificationCodeResponse struct {
	ID        uuid.UUID `json:"id"`
	Recipient string    `json:"recipient"`
	Channel   string    `json:"channel"`
	Purpose   string    `json:"purpose"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int       `json:"expires_in_seconds"`
	ResendIn  *int      `json:"resend_in_seconds,omitempty"`
}

// VerifyCodeRequest represents a request to verify a code
type VerifyCodeRequest struct {
	Recipient string `json:"recipient"`
	Code      string `json:"code"`
	Purpose   string `json:"purpose"`
}

// VerifyCodeResponse represents the response from verifying a code
type VerifyCodeResponse struct {
	Success    bool       `json:"success"`
	Verified   bool       `json:"verified"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	SessionID  *uuid.UUID `json:"session_id,omitempty"`
	TenantID   *uuid.UUID `json:"tenant_id,omitempty"`
	Message    string     `json:"message,omitempty"`
}

// ResendCodeRequest represents a request to resend a verification code
type ResendCodeRequest struct {
	Recipient string     `json:"recipient"`
	Channel   string     `json:"channel"`
	Purpose   string     `json:"purpose"`
	SessionID *uuid.UUID `json:"session_id,omitempty"`
}

// VerificationStatusResponse represents the verification status
type VerificationStatusResponse struct {
	Recipient    string     `json:"recipient"`
	Purpose      string     `json:"purpose"`
	IsVerified   bool       `json:"is_verified"`
	VerifiedAt   *time.Time `json:"verified_at,omitempty"`
	PendingCode  bool       `json:"pending_code"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CanResend    bool       `json:"can_resend"`
	AttemptsLeft int        `json:"attempts_left"`
}

// APIResponse represents the standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents an API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SendCode sends a verification code
func (c *VerificationClient) SendCode(ctx context.Context, req *SendVerificationCodeRequest) (*SendVerificationCodeResponse, error) {
	var response APIResponse
	var result SendVerificationCodeResponse

	if err := c.makeRequest(ctx, "POST", "/api/v1/verify/send", req, &response); err != nil {
		return nil, err
	}

	if !response.Success {
		if response.Error != nil {
			return nil, fmt.Errorf("%s: %s", response.Error.Message, response.Error.Details)
		}
		return nil, fmt.Errorf("failed to send verification code: %s", response.Message)
	}

	// Convert response.Data to SendVerificationCodeResponse
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response data: %w", err)
	}

	return &result, nil
}

// VerifyCode verifies a verification code
func (c *VerificationClient) VerifyCode(ctx context.Context, req *VerifyCodeRequest) (*VerifyCodeResponse, error) {
	var response APIResponse
	var result VerifyCodeResponse

	if err := c.makeRequest(ctx, "POST", "/api/v1/verify/code", req, &response); err != nil {
		return nil, err
	}

	// Even if success is false, we want to return the verification result
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response data: %w", err)
	}

	return &result, nil
}

// ResendCode resends a verification code
func (c *VerificationClient) ResendCode(ctx context.Context, req *ResendCodeRequest) (*SendVerificationCodeResponse, error) {
	var response APIResponse
	var result SendVerificationCodeResponse

	if err := c.makeRequest(ctx, "POST", "/api/v1/verify/resend", req, &response); err != nil {
		return nil, err
	}

	if !response.Success {
		if response.Error != nil {
			return nil, fmt.Errorf("%s: %s", response.Error.Message, response.Error.Details)
		}
		return nil, fmt.Errorf("failed to resend verification code: %s", response.Message)
	}

	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response data: %w", err)
	}

	return &result, nil
}

// GetStatus retrieves verification status
func (c *VerificationClient) GetStatus(ctx context.Context, recipient, purpose string) (*VerificationStatusResponse, error) {
	var response APIResponse
	var result VerificationStatusResponse

	url := fmt.Sprintf("/api/v1/verify/status?recipient=%s&purpose=%s", recipient, purpose)
	if err := c.makeRequest(ctx, "GET", url, nil, &response); err != nil {
		return nil, err
	}

	if !response.Success {
		if response.Error != nil {
			return nil, fmt.Errorf("%s: %s", response.Error.Message, response.Error.Details)
		}
		return nil, fmt.Errorf("failed to get verification status: %s", response.Message)
	}

	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response data: %w", err)
	}

	return &result, nil
}

// makeRequest makes an HTTP request to the verification service
func (c *VerificationClient) makeRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
