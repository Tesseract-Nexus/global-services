package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// VerificationCode represents a verification code record
type VerificationCode struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Recipient    string         `gorm:"type:varchar(255);not null;index" json:"recipient"` // email or phone
	Channel      string         `gorm:"type:varchar(20);not null" json:"channel"`          // email, sms
	Code         string         `gorm:"type:text;not null" json:"-"`                       // encrypted OTP
	CodeHash     string         `gorm:"type:varchar(64);not null;index" json:"-"`          // for lookup
	Purpose      string         `gorm:"type:varchar(50);not null" json:"purpose"`          // email_verification, password_reset, etc.
	SessionID    *uuid.UUID     `gorm:"type:uuid;index" json:"session_id,omitempty"`       // optional link to onboarding session
	TenantID     *uuid.UUID     `gorm:"type:uuid;index" json:"tenant_id,omitempty"`        // optional tenant context
	ExpiresAt    time.Time      `gorm:"not null;index" json:"expires_at"`
	VerifiedAt   *time.Time     `gorm:"index" json:"verified_at,omitempty"`
	AttemptCount int            `gorm:"default:0" json:"attempt_count"`
	MaxAttempts  int            `gorm:"default:3" json:"max_attempts"`
	IsUsed       bool           `gorm:"default:false;index" json:"is_used"`
	Metadata     []byte         `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name
func (VerificationCode) TableName() string {
	return "verification_codes"
}

// BeforeCreate hook to generate UUID
func (v *VerificationCode) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}

// IsExpired checks if the code has expired
func (v *VerificationCode) IsExpired() bool {
	return time.Now().After(v.ExpiresAt)
}

// IsValid checks if the code is valid for verification
func (v *VerificationCode) IsValid() bool {
	return !v.IsExpired() && !v.IsUsed && v.VerifiedAt == nil && v.AttemptCount < v.MaxAttempts
}

// CanResend checks if a new code can be sent
func (v *VerificationCode) CanResend() bool {
	// Allow resend if expired or max attempts reached
	return v.IsExpired() || v.AttemptCount >= v.MaxAttempts
}

// VerificationAttempt represents a verification attempt log
type VerificationAttempt struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	VerificationCodeID uuid.UUID      `gorm:"type:uuid;not null;index" json:"verification_code_id"`
	IPAddress          string         `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent          string         `gorm:"type:text" json:"user_agent"`
	Success            bool           `gorm:"default:false" json:"success"`
	FailureReason      string         `gorm:"type:varchar(255)" json:"failure_reason,omitempty"`
	CreatedAt          time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name
func (VerificationAttempt) TableName() string {
	return "verification_attempts"
}

// RateLimit represents rate limiting tracking
type RateLimit struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Identifier  string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"identifier"` // email/phone/ip
	Type        string         `gorm:"type:varchar(20);not null" json:"type"`                    // send, verify
	Count       int            `gorm:"default:0" json:"count"`
	WindowStart time.Time      `gorm:"not null" json:"window_start"`
	WindowEnd   time.Time      `gorm:"not null;index" json:"window_end"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name
func (RateLimit) TableName() string {
	return "rate_limits"
}

// IsWithinWindow checks if the current time is within the rate limit window
func (r *RateLimit) IsWithinWindow() bool {
	now := time.Now()
	return now.After(r.WindowStart) && now.Before(r.WindowEnd)
}

// ShouldReset checks if the rate limit window should be reset
func (r *RateLimit) ShouldReset() bool {
	return time.Now().After(r.WindowEnd)
}
