package otp

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// Generator generates one-time passwords
type Generator struct {
	length int
}

// NewGenerator creates a new OTP generator
func NewGenerator(length int) *Generator {
	if length < 4 {
		length = 4
	}
	if length > 10 {
		length = 10
	}
	return &Generator{
		length: length,
	}
}

// Generate generates a random numeric OTP
func (g *Generator) Generate() (string, error) {
	// Generate random numeric code
	max := new(big.Int)
	max.Exp(big.NewInt(10), big.NewInt(int64(g.length)), nil)

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("failed to generate random number: %w", err)
	}

	// Format with leading zeros to ensure exact length
	code := fmt.Sprintf("%0*d", g.length, n)
	return code, nil
}

// GenerateAlphanumeric generates a random alphanumeric OTP
func (g *Generator) GenerateAlphanumeric() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, g.length)

	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		result[i] = charset[num.Int64()]
	}

	return string(result), nil
}

// Validate checks if the provided code matches the format
func (g *Generator) Validate(code string) bool {
	if len(code) != g.length {
		return false
	}

	// Check if all characters are digits
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// NormalizeCode normalizes a code by removing spaces and converting to uppercase
func NormalizeCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")
	return strings.ToUpper(code)
}
