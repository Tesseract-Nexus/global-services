package clients

import (
	"context"
	"time"
)

// ProviderName identifies a translation provider
type ProviderName string

const (
	ProviderLibreTranslate ProviderName = "libretranslate"
	ProviderBergamot       ProviderName = "bergamot"
	ProviderHuggingFace    ProviderName = "huggingface"
	ProviderGoogle         ProviderName = "google"
)

// TranslationProvider defines the interface that all translation providers must implement
type TranslationProvider interface {
	// Name returns the provider's identifier
	Name() ProviderName

	// Priority returns the provider's priority (lower = higher priority)
	Priority() int

	// IsConfigured returns true if the provider is properly configured
	IsConfigured() bool

	// IsHealthy checks if the provider service is available
	IsHealthy(ctx context.Context) bool

	// SupportsLanguagePair checks if the provider supports translating from source to target language
	SupportsLanguagePair(sourceLang, targetLang string) bool

	// Translate translates text from source to target language
	Translate(ctx context.Context, text, sourceLang, targetLang string) (*TranslationResult, error)

	// TranslateBatch translates multiple texts (optional optimization, can fall back to single calls)
	TranslateBatch(ctx context.Context, texts []string, sourceLang, targetLang string) ([]TranslationResult, error)
}

// TranslationResult represents the result of a translation
type TranslationResult struct {
	TranslatedText string       `json:"translated_text"`
	SourceLang     string       `json:"source_lang"`
	TargetLang     string       `json:"target_lang"`
	Provider       ProviderName `json:"provider"`
	Latency        time.Duration `json:"latency"`
	FromCache      bool         `json:"from_cache"`
}

// ProviderHealth tracks the health status of a provider
type ProviderHealth struct {
	Provider      ProviderName `json:"provider"`
	Healthy       bool         `json:"healthy"`
	LastChecked   time.Time    `json:"last_checked"`
	FailureCount  int          `json:"failure_count"`
	LastError     string       `json:"last_error,omitempty"`
	AvgLatencyMs  float64      `json:"avg_latency_ms"`
}

// ProviderMetrics tracks usage metrics for a provider
type ProviderMetrics struct {
	Provider         ProviderName `json:"provider"`
	TotalRequests    int64        `json:"total_requests"`
	SuccessfulCount  int64        `json:"successful_count"`
	FailedCount      int64        `json:"failed_count"`
	TotalLatencyMs   int64        `json:"total_latency_ms"`
	CharactersCount  int64        `json:"characters_count"`
}
