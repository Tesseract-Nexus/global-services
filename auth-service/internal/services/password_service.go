package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// PasswordService handles password-related operations
type PasswordService struct{}

// NewPasswordService creates a new password service
func NewPasswordService() *PasswordService {
	return &PasswordService{}
}

// HashPassword hashes a password using bcrypt
func (ps *PasswordService) HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", errors.New("password must be at least 8 characters long")
	}

	// Generate hash with cost 12 (good balance of security and performance)
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hashedBytes), nil
}

// VerifyPassword verifies a password against its hash
func (ps *PasswordService) VerifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// ValidatePasswordStrength validates password strength
func (ps *PasswordService) ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasNumber = true
		case char >= 33 && char <= 126: // Special characters
			if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')) {
				hasSpecial = true
			}
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}

// GenerateSecureToken generates a cryptographically secure random token
func (ps *PasswordService) GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateResetToken generates a password reset token
func (ps *PasswordService) GenerateResetToken() (string, error) {
	return ps.GenerateSecureToken(32) // 64 character hex string
}

// GenerateVerificationToken generates an email verification token
func (ps *PasswordService) GenerateVerificationToken() (string, error) {
	return ps.GenerateSecureToken(32) // 64 character hex string
}

// IsTokenExpired checks if a token has expired
func (ps *PasswordService) IsTokenExpired(expiresAt time.Time) bool {
	return time.Now().After(expiresAt)
}

// GetPasswordResetExpiry returns the expiry time for password reset tokens (1 hour)
func (ps *PasswordService) GetPasswordResetExpiry() time.Time {
	return time.Now().Add(1 * time.Hour)
}

// GetEmailVerificationExpiry returns the expiry time for email verification tokens (24 hours)
func (ps *PasswordService) GetEmailVerificationExpiry() time.Time {
	return time.Now().Add(24 * time.Hour)
}
