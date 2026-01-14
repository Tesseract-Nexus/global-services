package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Tesseract-Nexus/go-shared/security"
	"notification-service/internal/config"
)

// TwilioVerifyProvider implements OTP sending and verification via Twilio Verify API
// Uses API Key authentication (SKxxxxxxx + secret)
type TwilioVerifyProvider struct {
	accountSID       string
	apiKeySID        string
	apiKeySecret     string
	verifyServiceSID string
	testPhoneNumber  string
	otpExpiry        int
	otpLength        int
	httpClient       *http.Client
}

// VerifyChannel represents the channel for sending verification codes
type VerifyChannel string

const (
	VerifyChannelSMS   VerifyChannel = "sms"
	VerifyChannelEmail VerifyChannel = "email"
	VerifyChannelCall  VerifyChannel = "call"
	VerifyChannelWhatsApp VerifyChannel = "whatsapp"
)

// VerificationRequest represents a request to send an OTP
type VerificationRequest struct {
	To       string        `json:"to"`       // Phone number (E.164) or email
	Channel  VerifyChannel `json:"channel"`  // sms, email, call, whatsapp
	Locale   string        `json:"locale"`   // e.g., "en" for English
	CustomCode string      `json:"custom_code,omitempty"` // Custom OTP (if allowed)
}

// VerificationResponse represents the response from Twilio Verify API
type VerificationResponse struct {
	SID         string `json:"sid"`
	ServiceSID  string `json:"service_sid"`
	AccountSID  string `json:"account_sid"`
	To          string `json:"to"`
	Channel     string `json:"channel"`
	Status      string `json:"status"`
	Valid       bool   `json:"valid"`
	DateCreated string `json:"date_created"`
	DateUpdated string `json:"date_updated"`
	SendCodeAttempts []struct {
		Time    string `json:"time"`
		Channel string `json:"channel"`
	} `json:"send_code_attempts,omitempty"`
	URL string `json:"url,omitempty"`
}

// VerificationCheckRequest represents a request to check an OTP
type VerificationCheckRequest struct {
	To   string `json:"to"`   // Phone number (E.164) or email
	Code string `json:"code"` // The OTP code to verify
}

// VerificationCheckResponse represents the response from OTP verification
type VerificationCheckResponse struct {
	SID        string `json:"sid"`
	ServiceSID string `json:"service_sid"`
	AccountSID string `json:"account_sid"`
	To         string `json:"to"`
	Channel    string `json:"channel"`
	Status     string `json:"status"` // "approved" or "pending"
	Valid      bool   `json:"valid"`
	DateCreated string `json:"date_created"`
	DateUpdated string `json:"date_updated"`
}

// TwilioErrorResponse represents an error from Twilio API
type TwilioErrorResponse struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	MoreInfo string `json:"more_info"`
	Status   int    `json:"status"`
}

// NewTwilioVerifyProvider creates a new Twilio Verify provider
func NewTwilioVerifyProvider(cfg *config.VerifyConfig) (*TwilioVerifyProvider, error) {
	if cfg.TwilioVerifyServiceSID == "" {
		return nil, fmt.Errorf("TWILIO_VERIFY_SERVICE_SID is required")
	}
	if cfg.TwilioAccountSID == "" {
		return nil, fmt.Errorf("TWILIO_ACCOUNT_SID is required")
	}
	if cfg.TwilioAPIKeySID == "" {
		return nil, fmt.Errorf("TWILIO_API_KEY_SID is required")
	}
	if cfg.TwilioAPIKeySecret == "" {
		return nil, fmt.Errorf("TWILIO_API_KEY_SECRET is required")
	}

	provider := &TwilioVerifyProvider{
		accountSID:       cfg.TwilioAccountSID,
		apiKeySID:        cfg.TwilioAPIKeySID,
		apiKeySecret:     cfg.TwilioAPIKeySecret,
		verifyServiceSID: cfg.TwilioVerifyServiceSID,
		testPhoneNumber:  cfg.TestPhoneNumber,
		otpExpiry:        cfg.OTPExpiryMinutes,
		otpLength:        cfg.OTPLength,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	log.Printf("[TWILIO-VERIFY] Initialized with API Key authentication")
	log.Printf("[TWILIO-VERIFY] Service SID: %s, Account SID: %s, API Key: %s",
		provider.verifyServiceSID, provider.accountSID, provider.apiKeySID)

	return provider, nil
}

// SendVerification sends an OTP to the specified recipient
func (p *TwilioVerifyProvider) SendVerification(ctx context.Context, req *VerificationRequest) (*VerificationResponse, error) {
	startTime := time.Now()

	// Default channel to SMS if not specified
	if req.Channel == "" {
		req.Channel = VerifyChannelSMS
	}

	log.Printf("[TWILIO-VERIFY] Sending %s verification to %s", req.Channel, security.MaskPhone(req.To))

	// Build API URL (global endpoint)
	apiURL := fmt.Sprintf(
		"https://verify.twilio.com/v2/Services/%s/Verifications",
		p.verifyServiceSID,
	)

	// Build form data
	formData := url.Values{}
	formData.Set("To", req.To)
	formData.Set("Channel", string(req.Channel))

	if req.Locale != "" {
		formData.Set("Locale", req.Locale)
	}
	if req.CustomCode != "" {
		formData.Set("CustomCode", req.CustomCode)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p.setAuthHeader(httpReq)

	// Execute request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode >= 400 {
		var twilioErr TwilioErrorResponse
		if err := json.Unmarshal(body, &twilioErr); err == nil {
			log.Printf("[TWILIO-VERIFY] Error %d: %s (more info: %s)",
				twilioErr.Code, twilioErr.Message, twilioErr.MoreInfo)
			return nil, fmt.Errorf("Twilio error %d: %s", twilioErr.Code, twilioErr.Message)
		}
		return nil, fmt.Errorf("Twilio error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse success response
	var verifyResp VerificationResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("[TWILIO-VERIFY] Verification sent successfully to %s via %s (SID: %s, took %v)",
		security.MaskPhone(req.To), req.Channel, verifyResp.SID, time.Since(startTime))

	return &verifyResp, nil
}

// CheckVerification verifies an OTP code
func (p *TwilioVerifyProvider) CheckVerification(ctx context.Context, req *VerificationCheckRequest) (*VerificationCheckResponse, error) {
	startTime := time.Now()

	log.Printf("[TWILIO-VERIFY] Checking verification for %s", security.MaskPhone(req.To))

	// Build API URL (global endpoint)
	apiURL := fmt.Sprintf(
		"https://verify.twilio.com/v2/Services/%s/VerificationCheck",
		p.verifyServiceSID,
	)

	// Build form data
	formData := url.Values{}
	formData.Set("To", req.To)
	formData.Set("Code", req.Code)

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p.setAuthHeader(httpReq)

	// Execute request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode >= 400 {
		var twilioErr TwilioErrorResponse
		if err := json.Unmarshal(body, &twilioErr); err == nil {
			log.Printf("[TWILIO-VERIFY] Check error %d: %s", twilioErr.Code, twilioErr.Message)
			return nil, fmt.Errorf("Twilio error %d: %s", twilioErr.Code, twilioErr.Message)
		}
		return nil, fmt.Errorf("Twilio error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse success response
	var checkResp VerificationCheckResponse
	if err := json.Unmarshal(body, &checkResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("[TWILIO-VERIFY] Verification check for %s: status=%s, valid=%v (took %v)",
		security.MaskPhone(req.To), checkResp.Status, checkResp.Valid, time.Since(startTime))

	return &checkResp, nil
}

// CancelVerification cancels a pending verification
func (p *TwilioVerifyProvider) CancelVerification(ctx context.Context, to string) error {
	log.Printf("[TWILIO-VERIFY] Cancelling verification for %s", security.MaskPhone(to))

	// Build API URL - update verification status to "canceled"
	apiURL := fmt.Sprintf(
		"https://verify.twilio.com/v2/Services/%s/Verifications/%s",
		p.verifyServiceSID,
		url.PathEscape(to),
	)

	// Build form data
	formData := url.Values{}
	formData.Set("Status", "canceled")

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p.setAuthHeader(httpReq)

	// Execute request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Twilio error: %d - %s", resp.StatusCode, string(body))
	}

	log.Printf("[TWILIO-VERIFY] Verification cancelled for %s", security.MaskPhone(to))
	return nil
}

// GetVerificationStatus gets the status of a verification
func (p *TwilioVerifyProvider) GetVerificationStatus(ctx context.Context, verificationSID string) (*VerificationResponse, error) {
	log.Printf("[TWILIO-VERIFY] Getting verification status for SID: %s", verificationSID)

	// Build API URL (global endpoint)
	apiURL := fmt.Sprintf(
		"https://verify.twilio.com/v2/Services/%s/Verifications/%s",
		p.verifyServiceSID,
		verificationSID,
	)

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setAuthHeader(httpReq)

	// Execute request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Twilio error: %d - %s", resp.StatusCode, string(body))
	}

	var verifyResp VerificationResponse
	if err := json.Unmarshal(body, &verifyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &verifyResp, nil
}

// setAuthHeader sets the API Key authentication header
func (p *TwilioVerifyProvider) setAuthHeader(req *http.Request) {
	// API Key authentication: API Key SID : API Key Secret
	auth := base64.StdEncoding.EncodeToString([]byte(p.apiKeySID + ":" + p.apiKeySecret))
	req.Header.Set("Authorization", "Basic "+auth)
}

// GetAuthMode returns the authentication mode (always "api_key")
func (p *TwilioVerifyProvider) GetAuthMode() string {
	return "api_key"
}

// GetTestPhoneNumber returns the test phone number for development
func (p *TwilioVerifyProvider) GetTestPhoneNumber() string {
	return p.testPhoneNumber
}

// Send implements the Provider interface for generic message sending
// This sends an OTP via SMS by default
func (p *TwilioVerifyProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	// Extract phone number from To field
	req := &VerificationRequest{
		To:      message.To,
		Channel: VerifyChannelSMS,
	}

	// Check if channel is specified in metadata
	if message.Metadata != nil {
		if channel, ok := message.Metadata["channel"].(string); ok {
			req.Channel = VerifyChannel(channel)
		}
		if locale, ok := message.Metadata["locale"].(string); ok {
			req.Locale = locale
		}
	}

	resp, err := p.SendVerification(ctx, req)
	if err != nil {
		return &SendResult{
			ProviderName: "TwilioVerify",
			Success:      false,
			Error:        err,
		}, err
	}

	return &SendResult{
		ProviderID:   resp.SID,
		ProviderName: "TwilioVerify",
		Success:      true,
		ProviderData: map[string]interface{}{
			"sid":     resp.SID,
			"to":      resp.To,
			"channel": resp.Channel,
			"status":  resp.Status,
		},
	}, nil
}

// GetName returns the provider name
func (p *TwilioVerifyProvider) GetName() string {
	return "TwilioVerify"
}

// SupportsChannel returns the supported channel
func (p *TwilioVerifyProvider) SupportsChannel() string {
	return "VERIFY"
}

// VerifyService provides a high-level interface for verification operations
type VerifyService struct {
	provider *TwilioVerifyProvider
}

// NewVerifyService creates a new verification service
func NewVerifyService(cfg *config.VerifyConfig) (*VerifyService, error) {
	provider, err := NewTwilioVerifyProvider(cfg)
	if err != nil {
		return nil, err
	}
	return &VerifyService{provider: provider}, nil
}

// SendOTP sends an OTP to the specified phone number or email
func (s *VerifyService) SendOTP(ctx context.Context, to string, channel VerifyChannel) (*VerificationResponse, error) {
	return s.provider.SendVerification(ctx, &VerificationRequest{
		To:      to,
		Channel: channel,
	})
}

// VerifyOTP verifies an OTP code
func (s *VerifyService) VerifyOTP(ctx context.Context, to string, code string) (*VerificationCheckResponse, error) {
	return s.provider.CheckVerification(ctx, &VerificationCheckRequest{
		To:   to,
		Code: code,
	})
}

// IsApproved checks if a verification was approved
func (s *VerifyService) IsApproved(resp *VerificationCheckResponse) bool {
	return resp.Status == "approved" && resp.Valid
}

// ResendOTP resends an OTP (creates a new verification)
func (s *VerifyService) ResendOTP(ctx context.Context, to string, channel VerifyChannel) (*VerificationResponse, error) {
	// Twilio automatically handles resending by creating a new verification
	return s.SendOTP(ctx, to, channel)
}

// CancelOTP cancels a pending verification
func (s *VerifyService) CancelOTP(ctx context.Context, to string) error {
	return s.provider.CancelVerification(ctx, to)
}

// GetProvider returns the underlying provider
func (s *VerifyService) GetProvider() *TwilioVerifyProvider {
	return s.provider
}

// Helper function to format phone numbers to E.164
func FormatPhoneE164(phone string, countryCode string) string {
	// Remove all non-numeric characters except +
	var buf bytes.Buffer
	for _, c := range phone {
		if c == '+' || (c >= '0' && c <= '9') {
			buf.WriteRune(c)
		}
	}
	formatted := buf.String()

	// If doesn't start with +, add country code
	if !strings.HasPrefix(formatted, "+") {
		if countryCode == "" {
			countryCode = "1" // Default to US
		}
		if !strings.HasPrefix(countryCode, "+") {
			countryCode = "+" + countryCode
		}
		formatted = countryCode + formatted
	}

	return formatted
}
