package models

import "github.com/google/uuid"

// SendVerificationRequest represents a request to send a verification code
type SendVerificationRequest struct {
	Recipient string                 `json:"recipient" binding:"required"` // email or phone
	Channel   string                 `json:"channel" binding:"required,oneof=email sms"`
	Purpose   string                 `json:"purpose" binding:"required"`
	SessionID *uuid.UUID             `json:"session_id,omitempty"`
	TenantID  *uuid.UUID             `json:"tenant_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// VerifyCodeRequest represents a request to verify a code
type VerifyCodeRequest struct {
	Recipient string `json:"recipient" binding:"required"`
	Code      string `json:"code" binding:"required"`
	Purpose   string `json:"purpose" binding:"required"`
}

// ResendCodeRequest represents a request to resend a verification code
type ResendCodeRequest struct {
	Recipient string     `json:"recipient" binding:"required"`
	Channel   string     `json:"channel" binding:"required,oneof=email sms"`
	Purpose   string     `json:"purpose" binding:"required"`
	SessionID *uuid.UUID `json:"session_id,omitempty"`
}

// CheckStatusRequest represents a request to check verification status
type CheckStatusRequest struct {
	Recipient string `json:"recipient" binding:"required"`
	Purpose   string `json:"purpose" binding:"required"`
}

