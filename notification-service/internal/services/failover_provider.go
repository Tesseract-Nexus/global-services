package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// FailoverEmailProvider implements email sending with automatic failover
// Priority: Postal (SMTP) -> Mautic (newsletters) -> SendGrid (fallback)
type FailoverEmailProvider struct {
	providers       []Provider
	enableFailover  bool
	maxRetries      int
	retryDelay      time.Duration
}

// FailoverConfig configures the failover behavior
type FailoverConfig struct {
	EnableFailover bool
	MaxRetries     int
	RetryDelay     time.Duration
}

// NewFailoverEmailProvider creates a new failover email provider
// Providers are tried in order: first provider is primary, others are fallbacks
func NewFailoverEmailProvider(providers []Provider, config *FailoverConfig) *FailoverEmailProvider {
	if config == nil {
		config = &FailoverConfig{
			EnableFailover: true,
			MaxRetries:     1,
			RetryDelay:     2 * time.Second,
		}
	}

	// Filter out nil providers
	validProviders := make([]Provider, 0, len(providers))
	for _, p := range providers {
		if p != nil {
			validProviders = append(validProviders, p)
		}
	}

	return &FailoverEmailProvider{
		providers:      validProviders,
		enableFailover: config.EnableFailover,
		maxRetries:     config.MaxRetries,
		retryDelay:     config.RetryDelay,
	}
}

// Send sends an email with automatic failover
func (f *FailoverEmailProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	if len(f.providers) == 0 {
		return &SendResult{
			ProviderName: "Failover",
			Success:      false,
			Error:        fmt.Errorf("no email providers configured"),
		}, fmt.Errorf("no email providers configured")
	}

	startTime := time.Now()
	var lastError error
	var allErrors []string

	// Try each provider in order
	for i, provider := range f.providers {
		providerName := provider.GetName()

		// Skip if context is cancelled
		if ctx.Err() != nil {
			return &SendResult{
				ProviderName: "Failover",
				Success:      false,
				Error:        ctx.Err(),
			}, ctx.Err()
		}

		// Retry logic for current provider
		for attempt := 0; attempt <= f.maxRetries; attempt++ {
			if attempt > 0 {
				log.Printf("[FAILOVER] Retry %d/%d for %s", attempt, f.maxRetries, providerName)
				time.Sleep(f.retryDelay)
			}

			log.Printf("[FAILOVER] Attempting to send via %s (provider %d/%d)", providerName, i+1, len(f.providers))

			result, err := provider.Send(ctx, message)
			if err == nil && result.Success {
				log.Printf("[FAILOVER] Successfully sent via %s (took %v)", providerName, time.Since(startTime))
				// Ensure ProviderData is initialized before writing to it
				if result.ProviderData == nil {
					result.ProviderData = make(map[string]interface{})
				}
				result.ProviderData["failover_attempts"] = i + 1
				result.ProviderData["failover_total_duration"] = time.Since(startTime).String()
				return result, nil
			}

			// Record error
			if err != nil {
				lastError = err
				allErrors = append(allErrors, fmt.Sprintf("%s: %v", providerName, err))
				log.Printf("[FAILOVER] %s failed (attempt %d): %v", providerName, attempt+1, err)
			} else if result != nil && !result.Success {
				lastError = result.Error
				if result.Error != nil {
					allErrors = append(allErrors, fmt.Sprintf("%s: %v", providerName, result.Error))
				} else {
					allErrors = append(allErrors, fmt.Sprintf("%s: send failed without error", providerName))
				}
				log.Printf("[FAILOVER] %s returned failure (attempt %d)", providerName, attempt+1)
			}
		}

		// Check if failover is enabled before trying next provider
		if !f.enableFailover {
			log.Printf("[FAILOVER] Failover disabled, not trying next provider")
			break
		}
	}

	// All providers failed
	errorSummary := strings.Join(allErrors, "; ")
	finalError := fmt.Errorf("all email providers failed: %s", errorSummary)

	log.Printf("[FAILOVER] All providers failed after %v: %s", time.Since(startTime), errorSummary)

	return &SendResult{
		ProviderName: "Failover",
		Success:      false,
		Error:        lastError,
		ProviderData: map[string]interface{}{
			"all_errors":     allErrors,
			"total_attempts": len(f.providers),
			"duration":       time.Since(startTime).String(),
		},
	}, finalError
}

// GetName returns the provider name
func (f *FailoverEmailProvider) GetName() string {
	if len(f.providers) == 0 {
		return "Failover(none)"
	}

	names := make([]string, len(f.providers))
	for i, p := range f.providers {
		names[i] = p.GetName()
	}
	return fmt.Sprintf("Failover(%s)", strings.Join(names, "->"))
}

// SupportsChannel returns the supported channel
func (f *FailoverEmailProvider) SupportsChannel() string {
	return "EMAIL"
}

// GetProviders returns the list of configured providers
func (f *FailoverEmailProvider) GetProviders() []Provider {
	return f.providers
}

// GetPrimaryProvider returns the primary (first) provider
func (f *FailoverEmailProvider) GetPrimaryProvider() Provider {
	if len(f.providers) > 0 {
		return f.providers[0]
	}
	return nil
}

// IsHealthy checks if at least one provider is available
func (f *FailoverEmailProvider) IsHealthy() bool {
	return len(f.providers) > 0
}

// ProviderStatus represents the status of a provider in the chain
type ProviderStatus struct {
	Name      string `json:"name"`
	Position  int    `json:"position"`
	Available bool   `json:"available"`
}

// GetProviderStatuses returns status information for all providers
func (f *FailoverEmailProvider) GetProviderStatuses() []ProviderStatus {
	statuses := make([]ProviderStatus, len(f.providers))
	for i, p := range f.providers {
		statuses[i] = ProviderStatus{
			Name:      p.GetName(),
			Position:  i + 1,
			Available: true, // Assume available if configured
		}
	}
	return statuses
}

// FailoverSMSProvider implements SMS sending with automatic failover
// Priority: AWS SNS (primary) -> Twilio (fallback)
type FailoverSMSProvider struct {
	providers      []Provider
	enableFailover bool
	maxRetries     int
	retryDelay     time.Duration
}

// NewFailoverSMSProvider creates a new failover SMS provider
// Providers are tried in order: first provider is primary, others are fallbacks
func NewFailoverSMSProvider(providers []Provider, config *FailoverConfig) *FailoverSMSProvider {
	if config == nil {
		config = &FailoverConfig{
			EnableFailover: true,
			MaxRetries:     1,
			RetryDelay:     2 * time.Second,
		}
	}

	// Filter out nil providers
	validProviders := make([]Provider, 0, len(providers))
	for _, p := range providers {
		if p != nil {
			validProviders = append(validProviders, p)
		}
	}

	return &FailoverSMSProvider{
		providers:      validProviders,
		enableFailover: config.EnableFailover,
		maxRetries:     config.MaxRetries,
		retryDelay:     config.RetryDelay,
	}
}

// Send sends an SMS with automatic failover
func (f *FailoverSMSProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	if len(f.providers) == 0 {
		return &SendResult{
			ProviderName: "SMS Failover",
			Success:      false,
			Error:        fmt.Errorf("no SMS providers configured"),
		}, fmt.Errorf("no SMS providers configured")
	}

	startTime := time.Now()
	var lastError error
	var allErrors []string

	// Try each provider in order
	for i, provider := range f.providers {
		providerName := provider.GetName()

		// Skip if context is cancelled
		if ctx.Err() != nil {
			return &SendResult{
				ProviderName: "SMS Failover",
				Success:      false,
				Error:        ctx.Err(),
			}, ctx.Err()
		}

		// Retry logic for current provider
		for attempt := 0; attempt <= f.maxRetries; attempt++ {
			if attempt > 0 {
				log.Printf("[SMS FAILOVER] Retry %d/%d for %s", attempt, f.maxRetries, providerName)
				time.Sleep(f.retryDelay)
			}

			log.Printf("[SMS FAILOVER] Attempting to send via %s (provider %d/%d)", providerName, i+1, len(f.providers))

			result, err := provider.Send(ctx, message)
			if err == nil && result.Success {
				log.Printf("[SMS FAILOVER] Successfully sent via %s (took %v)", providerName, time.Since(startTime))
				// Ensure ProviderData is initialized before writing to it
				if result.ProviderData == nil {
					result.ProviderData = make(map[string]interface{})
				}
				result.ProviderData["failover_attempts"] = i + 1
				result.ProviderData["failover_total_duration"] = time.Since(startTime).String()
				return result, nil
			}

			// Record error
			if err != nil {
				lastError = err
				allErrors = append(allErrors, fmt.Sprintf("%s: %v", providerName, err))
				log.Printf("[SMS FAILOVER] %s failed (attempt %d): %v", providerName, attempt+1, err)
			} else if result != nil && !result.Success {
				lastError = result.Error
				if result.Error != nil {
					allErrors = append(allErrors, fmt.Sprintf("%s: %v", providerName, result.Error))
				} else {
					allErrors = append(allErrors, fmt.Sprintf("%s: send failed without error", providerName))
				}
				log.Printf("[SMS FAILOVER] %s returned failure (attempt %d)", providerName, attempt+1)
			}
		}

		// Check if failover is enabled before trying next provider
		if !f.enableFailover {
			log.Printf("[SMS FAILOVER] Failover disabled, not trying next provider")
			break
		}
	}

	// All providers failed
	errorSummary := strings.Join(allErrors, "; ")
	finalError := fmt.Errorf("all SMS providers failed: %s", errorSummary)

	log.Printf("[SMS FAILOVER] All providers failed after %v: %s", time.Since(startTime), errorSummary)

	return &SendResult{
		ProviderName: "SMS Failover",
		Success:      false,
		Error:        lastError,
		ProviderData: map[string]interface{}{
			"all_errors":     allErrors,
			"total_attempts": len(f.providers),
			"duration":       time.Since(startTime).String(),
		},
	}, finalError
}

// GetName returns the provider name
func (f *FailoverSMSProvider) GetName() string {
	if len(f.providers) == 0 {
		return "SMS Failover(none)"
	}

	names := make([]string, len(f.providers))
	for i, p := range f.providers {
		names[i] = p.GetName()
	}
	return fmt.Sprintf("SMS Failover(%s)", strings.Join(names, "->"))
}

// SupportsChannel returns the supported channel
func (f *FailoverSMSProvider) SupportsChannel() string {
	return "SMS"
}

// GetProviders returns the list of configured providers
func (f *FailoverSMSProvider) GetProviders() []Provider {
	return f.providers
}

// GetPrimaryProvider returns the primary (first) provider
func (f *FailoverSMSProvider) GetPrimaryProvider() Provider {
	if len(f.providers) > 0 {
		return f.providers[0]
	}
	return nil
}

// IsHealthy checks if at least one provider is available
func (f *FailoverSMSProvider) IsHealthy() bool {
	return len(f.providers) > 0
}

// GetProviderStatuses returns status information for all SMS providers
func (f *FailoverSMSProvider) GetProviderStatuses() []ProviderStatus {
	statuses := make([]ProviderStatus, len(f.providers))
	for i, p := range f.providers {
		statuses[i] = ProviderStatus{
			Name:      p.GetName(),
			Position:  i + 1,
			Available: true, // Assume available if configured
		}
	}
	return statuses
}
