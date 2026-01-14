package clients

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TranslationOrchestrator manages multiple translation providers with fallback chain
// Provider Order (by priority):
//   1. LibreTranslate (open-source, self-hosted, free)
//   2. Hugging Face (open-source models, free API with limits)
//   3. Google Translate (paid, last resort)
//
// The orchestrator:
//   - Tries providers in priority order
//   - Skips providers that don't support the language pair
//   - Falls back to next provider on failure
//   - Tracks health and metrics per provider
//   - Implements circuit breaker pattern for unhealthy providers
type TranslationOrchestrator struct {
	providers []TranslationProvider
	logger    *logrus.Entry

	// Metrics tracking
	metrics   map[ProviderName]*ProviderMetrics
	metricsMu sync.RWMutex

	// Health tracking
	health   map[ProviderName]*ProviderHealth
	healthMu sync.RWMutex
}

// OrchestratorConfig configures the orchestrator
type OrchestratorConfig struct {
	// Enable detailed logging for debugging
	DebugLogging bool
}

// NewTranslationOrchestrator creates a new orchestrator with the given providers
// Providers are sorted by priority (lower number = higher priority)
func NewTranslationOrchestrator(providers []TranslationProvider, logger *logrus.Entry) *TranslationOrchestrator {
	// Filter out nil and unconfigured providers
	configuredProviders := make([]TranslationProvider, 0, len(providers))
	for _, p := range providers {
		if p != nil && p.IsConfigured() {
			configuredProviders = append(configuredProviders, p)
		}
	}

	// Sort by priority (lowest first = highest priority)
	sort.Slice(configuredProviders, func(i, j int) bool {
		return configuredProviders[i].Priority() < configuredProviders[j].Priority()
	})

	o := &TranslationOrchestrator{
		providers: configuredProviders,
		logger:    logger,
		metrics:   make(map[ProviderName]*ProviderMetrics),
		health:    make(map[ProviderName]*ProviderHealth),
	}

	// Initialize metrics and health for each provider
	for _, p := range configuredProviders {
		o.metrics[p.Name()] = &ProviderMetrics{Provider: p.Name()}
		o.health[p.Name()] = &ProviderHealth{
			Provider:    p.Name(),
			Healthy:     true,
			LastChecked: time.Now(),
		}
	}

	// Log provider chain
	providerNames := make([]string, len(configuredProviders))
	for i, p := range configuredProviders {
		providerNames[i] = string(p.Name())
	}
	logger.WithField("providers", providerNames).Info("Translation orchestrator initialized with provider chain")

	return o
}

// Translate attempts translation using the provider chain
// Returns the result from the first successful provider
func (o *TranslationOrchestrator) Translate(ctx context.Context, text, sourceLang, targetLang string) (*TranslationResult, error) {
	if len(o.providers) == 0 {
		return nil, fmt.Errorf("no translation providers configured")
	}

	var lastErr error
	attemptedProviders := make([]string, 0)

	for _, provider := range o.providers {
		providerName := provider.Name()

		// Check if provider is healthy
		if !provider.IsHealthy(ctx) {
			o.logger.WithFields(logrus.Fields{
				"provider": providerName,
			}).Debug("Skipping unhealthy provider")
			continue
		}

		// Check if provider supports this language pair
		if !provider.SupportsLanguagePair(sourceLang, targetLang) {
			o.logger.WithFields(logrus.Fields{
				"provider":    providerName,
				"source_lang": sourceLang,
				"target_lang": targetLang,
			}).Debug("Provider doesn't support language pair, skipping")
			continue
		}

		attemptedProviders = append(attemptedProviders, string(providerName))

		// Attempt translation
		start := time.Now()
		result, err := provider.Translate(ctx, text, sourceLang, targetLang)
		latency := time.Since(start)

		if err != nil {
			lastErr = err
			o.recordFailure(providerName, err.Error(), latency)
			o.logger.WithFields(logrus.Fields{
				"provider": providerName,
				"error":    err.Error(),
				"latency":  latency.String(),
			}).Warn("Translation failed, trying next provider")
			continue
		}

		// Success!
		o.recordSuccess(providerName, int64(len(text)), latency)
		o.logger.WithFields(logrus.Fields{
			"provider":    providerName,
			"source_lang": sourceLang,
			"target_lang": targetLang,
			"latency":     latency.String(),
		}).Debug("Translation successful")

		return result, nil
	}

	// All providers failed
	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed (tried: %v): %w", attemptedProviders, lastErr)
	}

	return nil, fmt.Errorf("no suitable provider found for %s->%s (tried: %v)", sourceLang, targetLang, attemptedProviders)
}

// TranslateBatch attempts batch translation using the provider chain
func (o *TranslationOrchestrator) TranslateBatch(ctx context.Context, texts []string, sourceLang, targetLang string) ([]TranslationResult, error) {
	if len(o.providers) == 0 {
		return nil, fmt.Errorf("no translation providers configured")
	}

	var lastErr error
	attemptedProviders := make([]string, 0)

	for _, provider := range o.providers {
		providerName := provider.Name()

		// Check if provider is healthy
		if !provider.IsHealthy(ctx) {
			continue
		}

		// Check if provider supports this language pair
		if !provider.SupportsLanguagePair(sourceLang, targetLang) {
			continue
		}

		attemptedProviders = append(attemptedProviders, string(providerName))

		// Attempt batch translation
		start := time.Now()
		results, err := provider.TranslateBatch(ctx, texts, sourceLang, targetLang)
		latency := time.Since(start)

		if err != nil {
			lastErr = err
			o.recordFailure(providerName, err.Error(), latency)
			o.logger.WithFields(logrus.Fields{
				"provider":   providerName,
				"error":      err.Error(),
				"batch_size": len(texts),
			}).Warn("Batch translation failed, trying next provider")
			continue
		}

		// Success!
		totalChars := int64(0)
		for _, t := range texts {
			totalChars += int64(len(t))
		}
		o.recordSuccess(providerName, totalChars, latency)

		return results, nil
	}

	// All providers failed
	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed for batch: %w", lastErr)
	}

	return nil, fmt.Errorf("no suitable provider found for batch %s->%s", sourceLang, targetLang)
}

// TranslateWithFallback translates text, falling back through providers until one succeeds
// Similar to Translate but with more detailed error reporting
func (o *TranslationOrchestrator) TranslateWithFallback(ctx context.Context, text, sourceLang, targetLang string) (*TranslationResult, []ProviderAttempt, error) {
	attempts := make([]ProviderAttempt, 0)

	if len(o.providers) == 0 {
		return nil, attempts, fmt.Errorf("no translation providers configured")
	}

	for _, provider := range o.providers {
		providerName := provider.Name()
		attempt := ProviderAttempt{
			Provider: providerName,
			Started:  time.Now(),
		}

		// Check health and language support
		if !provider.IsHealthy(ctx) {
			attempt.Skipped = true
			attempt.SkipReason = "unhealthy"
			attempts = append(attempts, attempt)
			continue
		}

		if !provider.SupportsLanguagePair(sourceLang, targetLang) {
			attempt.Skipped = true
			attempt.SkipReason = "unsupported_language_pair"
			attempts = append(attempts, attempt)
			continue
		}

		// Attempt translation
		result, err := provider.Translate(ctx, text, sourceLang, targetLang)
		attempt.Latency = time.Since(attempt.Started)

		if err != nil {
			attempt.Success = false
			attempt.Error = err.Error()
			attempts = append(attempts, attempt)
			o.recordFailure(providerName, err.Error(), attempt.Latency)
			continue
		}

		// Success
		attempt.Success = true
		attempts = append(attempts, attempt)
		o.recordSuccess(providerName, int64(len(text)), attempt.Latency)

		return result, attempts, nil
	}

	return nil, attempts, fmt.Errorf("all providers failed")
}

// ProviderAttempt records details about a translation attempt
type ProviderAttempt struct {
	Provider   ProviderName  `json:"provider"`
	Started    time.Time     `json:"started"`
	Latency    time.Duration `json:"latency"`
	Success    bool          `json:"success"`
	Error      string        `json:"error,omitempty"`
	Skipped    bool          `json:"skipped,omitempty"`
	SkipReason string        `json:"skip_reason,omitempty"`
}

// recordSuccess records a successful translation
func (o *TranslationOrchestrator) recordSuccess(provider ProviderName, chars int64, latency time.Duration) {
	o.metricsMu.Lock()
	defer o.metricsMu.Unlock()

	if m, ok := o.metrics[provider]; ok {
		m.TotalRequests++
		m.SuccessfulCount++
		m.TotalLatencyMs += latency.Milliseconds()
		m.CharactersCount += chars
	}

	o.healthMu.Lock()
	defer o.healthMu.Unlock()

	if h, ok := o.health[provider]; ok {
		h.Healthy = true
		h.LastChecked = time.Now()
		h.FailureCount = 0
		h.LastError = ""
		// Update average latency
		totalReqs := float64(o.metrics[provider].SuccessfulCount)
		if totalReqs > 0 {
			h.AvgLatencyMs = float64(o.metrics[provider].TotalLatencyMs) / totalReqs
		}
	}
}

// recordFailure records a failed translation attempt
func (o *TranslationOrchestrator) recordFailure(provider ProviderName, errMsg string, latency time.Duration) {
	o.metricsMu.Lock()
	defer o.metricsMu.Unlock()

	if m, ok := o.metrics[provider]; ok {
		m.TotalRequests++
		m.FailedCount++
		m.TotalLatencyMs += latency.Milliseconds()
	}

	o.healthMu.Lock()
	defer o.healthMu.Unlock()

	if h, ok := o.health[provider]; ok {
		h.FailureCount++
		h.LastError = errMsg
		h.LastChecked = time.Now()

		// Mark unhealthy after 3 consecutive failures
		if h.FailureCount >= 3 {
			h.Healthy = false
		}
	}
}

// GetProviders returns the list of configured providers in priority order
func (o *TranslationOrchestrator) GetProviders() []ProviderName {
	names := make([]ProviderName, len(o.providers))
	for i, p := range o.providers {
		names[i] = p.Name()
	}
	return names
}

// GetProviderHealth returns health status for all providers
func (o *TranslationOrchestrator) GetProviderHealth() map[ProviderName]*ProviderHealth {
	o.healthMu.RLock()
	defer o.healthMu.RUnlock()

	result := make(map[ProviderName]*ProviderHealth)
	for k, v := range o.health {
		// Copy to avoid race conditions
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetProviderMetrics returns metrics for all providers
func (o *TranslationOrchestrator) GetProviderMetrics() map[ProviderName]*ProviderMetrics {
	o.metricsMu.RLock()
	defer o.metricsMu.RUnlock()

	result := make(map[ProviderName]*ProviderMetrics)
	for k, v := range o.metrics {
		// Copy to avoid race conditions
		copy := *v
		result[k] = &copy
	}
	return result
}

// SupportsLanguagePair checks if any provider supports the language pair
func (o *TranslationOrchestrator) SupportsLanguagePair(sourceLang, targetLang string) bool {
	for _, p := range o.providers {
		if p.IsHealthy(context.Background()) && p.SupportsLanguagePair(sourceLang, targetLang) {
			return true
		}
	}
	return false
}

// GetBestProviderForPair returns the best available provider for a language pair
func (o *TranslationOrchestrator) GetBestProviderForPair(sourceLang, targetLang string) (ProviderName, bool) {
	for _, p := range o.providers {
		if p.IsHealthy(context.Background()) && p.SupportsLanguagePair(sourceLang, targetLang) {
			return p.Name(), true
		}
	}
	return "", false
}

// RefreshHealth refreshes health status for all providers
func (o *TranslationOrchestrator) RefreshHealth(ctx context.Context) {
	for _, p := range o.providers {
		healthy := p.IsHealthy(ctx)

		o.healthMu.Lock()
		if h, ok := o.health[p.Name()]; ok {
			h.Healthy = healthy
			h.LastChecked = time.Now()
		}
		o.healthMu.Unlock()
	}
}
