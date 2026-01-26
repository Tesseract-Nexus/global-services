package validators

import (
	"context"
	"fmt"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
)

// Validator validates credentials for a specific provider
type Validator interface {
	// Provider returns the provider name this validator handles
	Provider() string

	// Category returns the category this validator handles
	Category() models.SecretCategory

	// Validate checks if the provided credentials are valid
	// Returns validation result (never the credentials themselves)
	Validate(ctx context.Context, secrets map[string]string) (*models.ValidationResult, error)

	// RequiredKeys returns the keys required for this provider
	RequiredKeys() []string

	// OptionalKeys returns optional keys for this provider
	OptionalKeys() []string
}

// Registry manages validators for different providers
type Registry struct {
	validators map[string]Validator
}

// NewRegistry creates a new validator registry with built-in validators
func NewRegistry() *Registry {
	r := &Registry{
		validators: make(map[string]Validator),
	}

	// Register payment validators
	r.Register(NewStripeValidator())
	r.Register(NewRazorpayValidator())

	return r
}

// Register adds a validator to the registry
func (r *Registry) Register(v Validator) {
	key := fmt.Sprintf("%s:%s", v.Category(), v.Provider())
	r.validators[key] = v
}

// Get retrieves a validator for a category and provider
func (r *Registry) Get(category models.SecretCategory, provider string) (Validator, bool) {
	key := fmt.Sprintf("%s:%s", category, provider)
	v, ok := r.validators[key]
	return v, ok
}

// ValidateSecrets validates secrets using the appropriate validator
func (r *Registry) ValidateSecrets(ctx context.Context, category models.SecretCategory, provider string, secrets map[string]string) (*models.ValidationResult, error) {
	validator, ok := r.Get(category, provider)
	if !ok {
		return &models.ValidationResult{
			Status:  models.ValidationUnknown,
			Message: fmt.Sprintf("No validator registered for %s:%s", category, provider),
		}, nil
	}

	// Check required keys
	for _, key := range validator.RequiredKeys() {
		if _, exists := secrets[key]; !exists {
			return &models.ValidationResult{
				Status:  models.ValidationInvalid,
				Message: fmt.Sprintf("Missing required key: %s", key),
			}, nil
		}
	}

	return validator.Validate(ctx, secrets)
}

// HasValidator checks if a validator exists for the given category and provider
func (r *Registry) HasValidator(category models.SecretCategory, provider string) bool {
	_, ok := r.Get(category, provider)
	return ok
}

// ListValidators returns all registered validators
func (r *Registry) ListValidators() []Validator {
	result := make([]Validator, 0, len(r.validators))
	for _, v := range r.validators {
		result = append(result, v)
	}
	return result
}
