package models

import (
	"time"

	"github.com/google/uuid"
)

// APIResponse is the standard API response wrapper
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents an error response
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SendVerificationResponse represents the response after sending a verification code
type SendVerificationResponse struct {
	ID        uuid.UUID `json:"id"`
	Recipient string    `json:"recipient"`
	Channel   string    `json:"channel"`
	Purpose   string    `json:"purpose"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int       `json:"expires_in_seconds"`
	ResendIn  *int      `json:"resend_in_seconds,omitempty"`
}

// VerifyCodeResponse represents the response after verifying a code
type VerifyCodeResponse struct {
	Success    bool       `json:"success"`
	Verified   bool       `json:"verified"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	SessionID  *uuid.UUID `json:"session_id,omitempty"`
	TenantID   *uuid.UUID `json:"tenant_id,omitempty"`
	Message    string     `json:"message,omitempty"`
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
