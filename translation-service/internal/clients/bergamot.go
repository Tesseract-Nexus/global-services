package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// BergamotClient handles translation via self-hosted Bergamot service
// Bergamot is Mozilla's fast, privacy-focused translation engine
type BergamotClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *logrus.Entry
	priority   int

	// Health tracking
	healthy      bool
	lastHealthy  time.Time
	failureCount int
	healthMu     sync.RWMutex
}

// Bergamot request/response types
type bergamotTranslationRequest struct {
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

type bergamotTranslationResponse struct {
	TranslatedText string  `json:"translated_text"`
	SourceLang     string  `json:"source_lang"`
	TargetLang     string  `json:"target_lang"`
	Engine         string  `json:"engine"`
	LatencyMs      float64 `json:"latency_ms"`
}

type bergamotBatchRequest struct {
	Texts      []string `json:"texts"`
	SourceLang string   `json:"source_lang"`
	TargetLang string   `json:"target_lang"`
}

type bergamotBatchResponse struct {
	Translations []string `json:"translations"`
	SourceLang   string   `json:"source_lang"`
	TargetLang   string   `json:"target_lang"`
	Engine       string   `json:"engine"`
	Count        int      `json:"count"`
	LatencyMs    float64  `json:"latency_ms"`
}

type bergamotErrorResponse struct {
	Detail string `json:"detail"`
}

type bergamotLanguagePair struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Supported language pairs for Bergamot (based on Mozilla's model registry)
var bergamotLanguagePairs = map[string]bool{
	// English to European languages
	"en-es": true, "en-de": true, "en-fr": true, "en-pt": true,
	"en-it": true, "en-nl": true, "en-ru": true, "en-pl": true,
	"en-cs": true, "en-et": true,
	// European to English
	"es-en": true, "de-en": true, "fr-en": true, "pt-en": true,
	"it-en": true, "nl-en": true, "ru-en": true, "pl-en": true,
	"cs-en": true, "et-en": true,
}

// NewBergamotClient creates a new Bergamot translation client
func NewBergamotClient(baseURL string, logger *logrus.Entry) *BergamotClient {
	if baseURL == "" {
		baseURL = "http://bergamot-service:8080"
	}

	return &BergamotClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger:   logger,
		priority: 2, // Same priority as HuggingFace, before Google
		healthy:  true,
	}
}

// Name returns the provider name
func (c *BergamotClient) Name() ProviderName {
	return ProviderBergamot
}

// Priority returns the provider priority
func (c *BergamotClient) Priority() int {
	return c.priority
}

// IsConfigured returns true if the client has a base URL configured
func (c *BergamotClient) IsConfigured() bool {
	return c.baseURL != ""
}

// IsHealthy checks if the provider is currently healthy
func (c *BergamotClient) IsHealthy(ctx context.Context) bool {
	c.healthMu.RLock()
	healthy := c.healthy
	lastHealthy := c.lastHealthy
	failureCount := c.failureCount
	c.healthMu.RUnlock()

	// If we've had recent failures, check if enough time has passed for retry
	if !healthy && failureCount > 0 {
		backoffDuration := time.Duration(failureCount) * 30 * time.Second
		if backoffDuration > 5*time.Minute {
			backoffDuration = 5 * time.Minute
		}
		if time.Since(lastHealthy) < backoffDuration {
			return false
		}
	}

	return healthy
}

// SupportsLanguagePair checks if Bergamot supports this language pair
func (c *BergamotClient) SupportsLanguagePair(sourceLang, targetLang string) bool {
	key := fmt.Sprintf("%s-%s", sourceLang, targetLang)
	return bergamotLanguagePairs[key]
}

// Translate translates text using Bergamot service
func (c *BergamotClient) Translate(ctx context.Context, text, sourceLang, targetLang string) (*TranslationResult, error) {
	start := time.Now()

	if !c.IsConfigured() {
		return nil, fmt.Errorf("Bergamot not configured")
	}

	// Build request
	reqBody := bergamotTranslationRequest{
		Text:       text,
		SourceLang: sourceLang,
		TargetLang: targetLang,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/translate", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.markUnhealthy(err.Error())
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusBadRequest {
		var errResp bergamotErrorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("unsupported language pair %s->%s: %s", sourceLang, targetLang, errResp.Detail)
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		c.markUnhealthy("service unavailable")
		return nil, fmt.Errorf("bergamot service unavailable")
	}

	if resp.StatusCode != http.StatusOK {
		var errResp bergamotErrorResponse
		json.Unmarshal(respBody, &errResp)
		c.markUnhealthy(errResp.Detail)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, errResp.Detail)
	}

	var result bergamotTranslationResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.TranslatedText == "" {
		return nil, fmt.Errorf("empty translation response")
	}

	c.markHealthy()

	return &TranslationResult{
		TranslatedText: result.TranslatedText,
		SourceLang:     sourceLang,
		TargetLang:     targetLang,
		Provider:       ProviderBergamot,
		Latency:        time.Since(start),
	}, nil
}

// TranslateBatch translates multiple texts using Bergamot batch endpoint
func (c *BergamotClient) TranslateBatch(ctx context.Context, texts []string, sourceLang, targetLang string) ([]TranslationResult, error) {
	start := time.Now()

	if !c.IsConfigured() {
		return nil, fmt.Errorf("Bergamot not configured")
	}

	reqBody := bergamotBatchRequest{
		Texts:      texts,
		SourceLang: sourceLang,
		TargetLang: targetLang,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	url := fmt.Sprintf("%s/translate/batch", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.markUnhealthy(err.Error())
		return nil, fmt.Errorf("batch request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp bergamotErrorResponse
		json.Unmarshal(respBody, &errResp)
		c.markUnhealthy(errResp.Detail)
		return nil, fmt.Errorf("batch API error %d: %s", resp.StatusCode, errResp.Detail)
	}

	var batchResp bergamotBatchResponse
	if err := json.Unmarshal(respBody, &batchResp); err != nil {
		return nil, fmt.Errorf("failed to parse batch response: %w", err)
	}

	if len(batchResp.Translations) != len(texts) {
		return nil, fmt.Errorf("batch response count mismatch: got %d, expected %d", len(batchResp.Translations), len(texts))
	}

	c.markHealthy()
	latency := time.Since(start)

	results := make([]TranslationResult, len(texts))
	for i, translation := range batchResp.Translations {
		results[i] = TranslationResult{
			TranslatedText: translation,
			SourceLang:     sourceLang,
			TargetLang:     targetLang,
			Provider:       ProviderBergamot,
			Latency:        latency,
		}
	}

	return results, nil
}

// markHealthy marks the provider as healthy
func (c *BergamotClient) markHealthy() {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	c.healthy = true
	c.lastHealthy = time.Now()
	c.failureCount = 0
}

// markUnhealthy marks the provider as unhealthy
func (c *BergamotClient) markUnhealthy(reason string) {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	c.healthy = false
	c.failureCount++
	c.logger.WithFields(logrus.Fields{
		"reason":        reason,
		"failure_count": c.failureCount,
	}).Warn("Bergamot marked unhealthy")
}

// GetSupportedLanguages returns all supported language codes
func (c *BergamotClient) GetSupportedLanguages() []string {
	languages := make(map[string]bool)
	for pair := range bergamotLanguagePairs {
		if len(pair) >= 5 && pair[2] == '-' {
			languages[pair[:2]] = true
			languages[pair[3:]] = true
		}
	}

	result := make([]string, 0, len(languages))
	for lang := range languages {
		result = append(result, lang)
	}
	return result
}
