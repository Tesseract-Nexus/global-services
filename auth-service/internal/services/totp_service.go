package services

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPService handles TOTP (Time-based One-Time Password) operations for 2FA
type TOTPService struct {
	issuer      string
	accountName string
}

// NewTOTPService creates a new TOTP service
func NewTOTPService(issuer, accountName string) *TOTPService {
	if issuer == "" {
		issuer = "Tesseract Hub"
	}
	if accountName == "" {
		accountName = "user@tesseract-hub.com"
	}

	return &TOTPService{
		issuer:      issuer,
		accountName: accountName,
	}
}

// TOTPSecret represents a TOTP secret and associated data
type TOTPSecret struct {
	Secret      string   `json:"secret"`
	QRCodeURL   string   `json:"qr_code_url"`
	BackupCodes []string `json:"backup_codes"`
}

// GenerateSecret generates a new TOTP secret and QR code URL
func (ts *TOTPService) GenerateSecret(userEmail string) (*TOTPSecret, error) {
	// Generate a random secret (32 bytes = 160 bits, recommended for TOTP)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}

	// Encode secret in base32 (required for TOTP)
	secret := base32.StdEncoding.EncodeToString(secretBytes)

	// Generate backup codes
	backupCodes, err := ts.generateBackupCodes()
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	// Create QR code URL for authenticator apps
	qrCodeURL, err := ts.generateQRCodeURL(userEmail, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code URL: %w", err)
	}

	return &TOTPSecret{
		Secret:      secret,
		QRCodeURL:   qrCodeURL,
		BackupCodes: backupCodes,
	}, nil
}

// ValidateCode validates a TOTP code against a secret
func (ts *TOTPService) ValidateCode(secret, code string) bool {
	// Remove any spaces or formatting from the code
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")

	// Validate the code with some time skew tolerance (±1 period = ±30 seconds)
	return totp.Validate(code, secret)
}

// ValidateCodeWithSkew validates a TOTP code with custom time skew
func (ts *TOTPService) ValidateCodeWithSkew(secret, code string, skew uint) bool {
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")

	opts := totp.ValidateOpts{
		Period:    30,                // 30-second periods (standard)
		Skew:      skew,              // Allow specified periods of skew
		Digits:    otp.DigitsSix,     // 6-digit codes
		Algorithm: otp.AlgorithmSHA1, // SHA1 algorithm (standard)
	}

	valid, _ := totp.ValidateCustom(code, secret, time.Now(), opts)
	return valid
}

// GenerateCurrentCode generates the current TOTP code (mainly for testing)
func (ts *TOTPService) GenerateCurrentCode(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}

// generateQRCodeURL creates a QR code URL for authenticator apps
func (ts *TOTPService) generateQRCodeURL(userEmail, secret string) (string, error) {
	// Construct the TOTP URL according to the standard format
	// otpauth://totp/Issuer:AccountName?secret=SECRET&issuer=ISSUER

	key, err := otp.NewKeyFromURL(fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s",
		url.QueryEscape(ts.issuer),
		url.QueryEscape(userEmail),
		secret,
		url.QueryEscape(ts.issuer),
	))
	if err != nil {
		return "", fmt.Errorf("failed to create TOTP key: %w", err)
	}

	return key.URL(), nil
}

// GenerateBackupCodes generates backup codes for 2FA recovery (public method)
func (ts *TOTPService) GenerateBackupCodes() ([]string, error) {
	return ts.generateBackupCodes()
}

// generateBackupCodes generates backup codes for 2FA recovery
func (ts *TOTPService) generateBackupCodes() ([]string, error) {
	const numCodes = 10
	const codeLength = 8

	backupCodes := make([]string, numCodes)

	for i := 0; i < numCodes; i++ {
		// Generate random bytes
		bytes := make([]byte, codeLength/2)
		if _, err := rand.Read(bytes); err != nil {
			return nil, fmt.Errorf("failed to generate backup code: %w", err)
		}

		// Convert to hex and format as backup code
		code := fmt.Sprintf("%x", bytes)
		// Format as XXXX-XXXX for readability
		backupCodes[i] = fmt.Sprintf("%s-%s", code[:4], code[4:])
	}

	return backupCodes, nil
}

// ValidateBackupCode validates a backup code (simple string comparison)
func (ts *TOTPService) ValidateBackupCode(providedCode, storedCode string) bool {
	// Remove any formatting from provided code
	providedCode = strings.ReplaceAll(providedCode, "-", "")
	providedCode = strings.ReplaceAll(providedCode, " ", "")
	providedCode = strings.ToLower(providedCode)

	// Remove formatting from stored code for comparison
	storedCode = strings.ReplaceAll(storedCode, "-", "")
	storedCode = strings.ReplaceAll(storedCode, " ", "")
	storedCode = strings.ToLower(storedCode)

	return providedCode == storedCode
}

// GetTOTPWindow returns the current time window for TOTP
func (ts *TOTPService) GetTOTPWindow() int64 {
	return time.Now().Unix() / 30 // 30-second periods
}

// VerifySetup verifies that 2FA setup is working by validating a code
func (ts *TOTPService) VerifySetup(secret, code string) (bool, error) {
	if secret == "" {
		return false, fmt.Errorf("secret is required")
	}

	if code == "" {
		return false, fmt.Errorf("verification code is required")
	}

	// Validate with slightly more tolerance during setup
	valid := ts.ValidateCodeWithSkew(secret, code, 2) // ±2 periods = ±60 seconds

	if !valid {
		return false, fmt.Errorf("invalid verification code")
	}

	return true, nil
}

// GenerateQRCodeSVG generates an SVG QR code (you'd need a QR code library)
// This is a placeholder - in production you'd use a library like "github.com/skip2/go-qrcode"
func (ts *TOTPService) GenerateQRCodeSVG(url string) (string, error) {
	// Placeholder implementation
	// In production, use: qrcode.WriteFile(url, qrcode.Medium, 256, "qr.png")
	return fmt.Sprintf(`
<svg width="200" height="200" xmlns="http://www.w3.org/2000/svg">
  <rect width="200" height="200" fill="white"/>
  <text x="100" y="100" text-anchor="middle" fill="black" font-size="12">
    QR Code for: %s
  </text>
  <text x="100" y="120" text-anchor="middle" fill="gray" font-size="8">
    Use a QR code generator in production
  </text>
</svg>`, url), nil
}

// GetTOTPOptions returns the TOTP configuration options
func (ts *TOTPService) GetTOTPOptions() map[string]interface{} {
	return map[string]interface{}{
		"issuer":    ts.issuer,
		"algorithm": "SHA1",
		"digits":    6,
		"period":    30,
		"window":    1, // ±30 seconds tolerance
	}
}

// IsValidTOTPSecret validates that a TOTP secret is properly formatted
func (ts *TOTPService) IsValidTOTPSecret(secret string) bool {
	if secret == "" {
		return false
	}

	// Check if it's valid base32
	_, err := base32.StdEncoding.DecodeString(secret)
	return err == nil
}

// FormatBackupCode formats a backup code for display
func (ts *TOTPService) FormatBackupCode(code string) string {
	// Remove any existing formatting
	clean := strings.ReplaceAll(code, "-", "")
	clean = strings.ReplaceAll(clean, " ", "")

	// Add formatting: XXXX-XXXX
	if len(clean) == 8 {
		return fmt.Sprintf("%s-%s", clean[:4], clean[4:])
	}

	return clean
}

// GetRemainingTime returns seconds until next TOTP code
func (ts *TOTPService) GetRemainingTime() int {
	return 30 - int(time.Now().Unix()%30)
}

// TOTPStatus represents the current TOTP status
type TOTPStatus struct {
	CurrentWindow int64 `json:"current_window"`
	RemainingTime int   `json:"remaining_time"`
	NextWindow    int64 `json:"next_window"`
	TimeUntilNext int   `json:"time_until_next"`
}

// GetTOTPStatus reports the current TOTP timing status
func (ts *TOTPService) GetTOTPStatus() TOTPStatus {
	now := time.Now().Unix()
	currentWindow := now / 30
	remainingTime := 30 - int(now%30)

	return TOTPStatus{
		CurrentWindow: currentWindow,
		RemainingTime: remainingTime,
		NextWindow:    currentWindow + 1,
		TimeUntilNext: remainingTime,
	}
}
