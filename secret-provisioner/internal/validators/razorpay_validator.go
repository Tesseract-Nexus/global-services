package validators

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
)

// RazorpayValidator validates Razorpay credentials
type RazorpayValidator struct {
	httpClient *http.Client
}

// NewRazorpayValidator creates a new Razorpay validator
func NewRazorpayValidator() *RazorpayValidator {
	return &RazorpayValidator{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Provider returns the provider name
func (v *RazorpayValidator) Provider() string {
	return "razorpay"
}

// Category returns the category this validator handles
func (v *RazorpayValidator) Category() models.SecretCategory {
	return models.CategoryPayment
}

// RequiredKeys returns required keys for Razorpay
func (v *RazorpayValidator) RequiredKeys() []string {
	return []string{"key-id", "key-secret"}
}

// OptionalKeys returns optional keys for Razorpay
func (v *RazorpayValidator) OptionalKeys() []string {
	return []string{"webhook-secret"}
}

// Validate validates Razorpay credentials by calling the Razorpay API
func (v *RazorpayValidator) Validate(ctx context.Context, secrets map[string]string) (*models.ValidationResult, error) {
	keyID := secrets["key-id"]
	keySecret := secrets["key-secret"]

	if keyID == "" || keySecret == "" {
		return &models.ValidationResult{
			Status:  models.ValidationInvalid,
			Message: "Both key-id and key-secret are required",
		}, nil
	}

	// Call Razorpay API to validate credentials
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.razorpay.com/v1/payments?count=1", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(keyID, keySecret)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return &models.ValidationResult{
			Status:  models.ValidationUnknown,
			Message: fmt.Sprintf("Failed to connect to Razorpay: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return &models.ValidationResult{
			Status:  models.ValidationValid,
			Message: "Razorpay credentials are valid",
		}, nil
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return &models.ValidationResult{
			Status:  models.ValidationInvalid,
			Message: "Invalid Razorpay credentials",
		}, nil
	}

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		return &models.ValidationResult{
			Status:  models.ValidationUnknown,
			Message: "Razorpay rate limit exceeded, please try again later",
		}, nil
	}

	return &models.ValidationResult{
		Status:  models.ValidationUnknown,
		Message: fmt.Sprintf("Unexpected response from Razorpay: %d", resp.StatusCode),
	}, nil
}
