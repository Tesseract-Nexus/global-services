package validators

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
)

// StripeValidator validates Stripe credentials
type StripeValidator struct {
	httpClient *http.Client
}

// NewStripeValidator creates a new Stripe validator
func NewStripeValidator() *StripeValidator {
	return &StripeValidator{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Provider returns the provider name
func (v *StripeValidator) Provider() string {
	return "stripe"
}

// Category returns the category this validator handles
func (v *StripeValidator) Category() models.SecretCategory {
	return models.CategoryPayment
}

// RequiredKeys returns required keys for Stripe
func (v *StripeValidator) RequiredKeys() []string {
	return []string{"api-key"}
}

// OptionalKeys returns optional keys for Stripe
func (v *StripeValidator) OptionalKeys() []string {
	return []string{"webhook-secret"}
}

// Validate validates Stripe credentials by calling the Stripe API
func (v *StripeValidator) Validate(ctx context.Context, secrets map[string]string) (*models.ValidationResult, error) {
	apiKey, ok := secrets["api-key"]
	if !ok || apiKey == "" {
		return &models.ValidationResult{
			Status:  models.ValidationInvalid,
			Message: "API key is required",
		}, nil
	}

	// Call Stripe /v1/account endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.stripe.com/v1/account", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(apiKey, "")
	req.Header.Set("Stripe-Version", "2023-10-16")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return &models.ValidationResult{
			Status:  models.ValidationUnknown,
			Message: fmt.Sprintf("Failed to connect to Stripe: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Parse response to get account info (non-secret)
		var account struct {
			ID      string `json:"id"`
			Country string `json:"country"`
			Email   string `json:"email"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&account); err == nil {
			return &models.ValidationResult{
				Status:  models.ValidationValid,
				Message: "Stripe API key is valid",
				Details: map[string]string{
					"account_id": account.ID,
					"country":    account.Country,
				},
			}, nil
		}
		return &models.ValidationResult{
			Status:  models.ValidationValid,
			Message: "Stripe API key is valid",
		}, nil
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return &models.ValidationResult{
			Status:  models.ValidationInvalid,
			Message: "Invalid Stripe API key",
		}, nil
	}

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		return &models.ValidationResult{
			Status:  models.ValidationUnknown,
			Message: "Stripe rate limit exceeded, please try again later",
		}, nil
	}

	return &models.ValidationResult{
		Status:  models.ValidationUnknown,
		Message: fmt.Sprintf("Unexpected response from Stripe: %d", resp.StatusCode),
	}, nil
}
