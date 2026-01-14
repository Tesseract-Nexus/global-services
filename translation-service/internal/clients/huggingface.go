package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HuggingFaceClient handles translation via Hugging Face
// Supports both:
// - Self-hosted mode: Connects to local huggingface-mt-service
// - API mode: Uses Hugging Face Inference API (requires API key)
type HuggingFaceClient struct {
	apiKey       string
	baseURL      string
	httpClient   *http.Client
	logger       *logrus.Entry
	priority     int
	selfHosted   bool  // true if using self-hosted service

	// Language pair to model mapping cache
	modelCache   map[string]string
	modelCacheMu sync.RWMutex

	// Health tracking
	healthy       bool
	lastHealthy   time.Time
	failureCount  int
	healthMu      sync.RWMutex
}

// HuggingFace API request/response types (for external HF API)
type hfTranslationRequest struct {
	Inputs  string                 `json:"inputs"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type hfTranslationResponse struct {
	TranslationText string `json:"translation_text"`
}

type hfErrorResponse struct {
	Error         string  `json:"error"`
	EstimatedTime float64 `json:"estimated_time,omitempty"`
}

// Self-hosted HuggingFace MT service request/response types
type selfHostedTranslationRequest struct {
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

type selfHostedTranslationResponse struct {
	TranslatedText string  `json:"translated_text"`
	SourceLang     string  `json:"source_lang"`
	TargetLang     string  `json:"target_lang"`
	Model          string  `json:"model"`
	LatencyMs      float64 `json:"latency_ms"`
}

type selfHostedBatchRequest struct {
	Texts      []string `json:"texts"`
	SourceLang string   `json:"source_lang"`
	TargetLang string   `json:"target_lang"`
}

type selfHostedBatchResponse struct {
	Translations []string `json:"translations"`
	SourceLang   string   `json:"source_lang"`
	TargetLang   string   `json:"target_lang"`
	Model        string   `json:"model"`
	Count        int      `json:"count"`
	LatencyMs    float64  `json:"latency_ms"`
}

// Supported language pairs using Helsinki-NLP/OPUS-MT models
// Format: "source-target" -> "Helsinki-NLP/opus-mt-{source}-{target}"
var huggingFaceLanguagePairs = map[string]string{
	// English to Indian languages
	"en-hi": "Helsinki-NLP/opus-mt-en-hi",
	"en-ta": "Helsinki-NLP/opus-mt-en-mul", // Multilingual model for Tamil
	"en-te": "Helsinki-NLP/opus-mt-en-mul", // Multilingual model for Telugu
	"en-bn": "Helsinki-NLP/opus-mt-en-mul", // Multilingual model for Bengali
	"en-mr": "Helsinki-NLP/opus-mt-en-mul", // Multilingual model for Marathi
	"en-gu": "Helsinki-NLP/opus-mt-en-mul", // Multilingual model for Gujarati

	// European languages
	"en-es": "Helsinki-NLP/opus-mt-en-es",
	"en-fr": "Helsinki-NLP/opus-mt-en-fr",
	"en-de": "Helsinki-NLP/opus-mt-en-de",
	"en-it": "Helsinki-NLP/opus-mt-en-it",
	"en-pt": "Helsinki-NLP/opus-mt-en-pt",
	"en-nl": "Helsinki-NLP/opus-mt-en-nl",
	"en-ru": "Helsinki-NLP/opus-mt-en-ru",
	"en-pl": "Helsinki-NLP/opus-mt-en-pl",

	// Asian languages
	"en-zh": "Helsinki-NLP/opus-mt-en-zh",
	"en-ja": "Helsinki-NLP/opus-mt-en-jap",
	"en-ko": "Helsinki-NLP/opus-mt-en-ko",
	"en-vi": "Helsinki-NLP/opus-mt-en-vi",
	"en-th": "Helsinki-NLP/opus-mt-en-th",
	"en-id": "Helsinki-NLP/opus-mt-en-id",

	// Arabic and Middle Eastern
	"en-ar": "Helsinki-NLP/opus-mt-en-ar",
	"en-tr": "Helsinki-NLP/opus-mt-en-tr",
	"en-he": "Helsinki-NLP/opus-mt-en-he",

	// Reverse directions (target to English)
	"hi-en": "Helsinki-NLP/opus-mt-hi-en",
	"es-en": "Helsinki-NLP/opus-mt-es-en",
	"fr-en": "Helsinki-NLP/opus-mt-fr-en",
	"de-en": "Helsinki-NLP/opus-mt-de-en",
	"it-en": "Helsinki-NLP/opus-mt-it-en",
	"pt-en": "Helsinki-NLP/opus-mt-pt-en",
	"nl-en": "Helsinki-NLP/opus-mt-nl-en",
	"ru-en": "Helsinki-NLP/opus-mt-ru-en",
	"zh-en": "Helsinki-NLP/opus-mt-zh-en",
	"ja-en": "Helsinki-NLP/opus-mt-jap-en",
	"ko-en": "Helsinki-NLP/opus-mt-ko-en",
	"ar-en": "Helsinki-NLP/opus-mt-ar-en",
	"tr-en": "Helsinki-NLP/opus-mt-tr-en",

	// Multilingual Romance
	"es-fr": "Helsinki-NLP/opus-mt-es-fr",
	"fr-es": "Helsinki-NLP/opus-mt-fr-es",
	"es-it": "Helsinki-NLP/opus-mt-es-it",
	"it-es": "Helsinki-NLP/opus-mt-it-es",
	"es-pt": "Helsinki-NLP/opus-mt-es-pt",
	"pt-es": "Helsinki-NLP/opus-mt-pt-es",
}

// NewHuggingFaceClient creates a new Hugging Face translation client
// If baseURL points to a self-hosted service, apiKey can be empty
func NewHuggingFaceClient(apiKey string, baseURL string, logger *logrus.Entry) *HuggingFaceClient {
	selfHosted := false

	if baseURL == "" {
		baseURL = "https://api-inference.huggingface.co/models"
	} else {
		// Check if this is a self-hosted URL (not huggingface.co)
		selfHosted = !strings.Contains(baseURL, "huggingface.co")
		baseURL = strings.TrimSuffix(baseURL, "/")
	}

	return &HuggingFaceClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		selfHosted: selfHosted,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // HF can be slow for cold starts
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger:     logger,
		priority:   2, // Second priority after LibreTranslate
		modelCache: make(map[string]string),
		healthy:    true,
	}
}

// Name returns the provider name
func (c *HuggingFaceClient) Name() ProviderName {
	return ProviderHuggingFace
}

// Priority returns the provider priority
func (c *HuggingFaceClient) Priority() int {
	return c.priority
}

// IsConfigured returns true if the client is properly configured
// For self-hosted mode, no API key is required
// For HF API mode, an API key is required
func (c *HuggingFaceClient) IsConfigured() bool {
	return c.selfHosted || c.apiKey != ""
}

// IsHealthy checks if the provider is currently healthy
func (c *HuggingFaceClient) IsHealthy(ctx context.Context) bool {
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

// SupportsLanguagePair checks if Hugging Face has a model for this language pair
func (c *HuggingFaceClient) SupportsLanguagePair(sourceLang, targetLang string) bool {
	// Self-hosted service handles its own language pair validation
	// We'll return true and let the service return an error if unsupported
	if c.selfHosted {
		return true
	}
	key := fmt.Sprintf("%s-%s", sourceLang, targetLang)
	_, exists := huggingFaceLanguagePairs[key]
	return exists
}

// getModelForPair returns the model ID for a language pair
func (c *HuggingFaceClient) getModelForPair(sourceLang, targetLang string) (string, bool) {
	key := fmt.Sprintf("%s-%s", sourceLang, targetLang)

	c.modelCacheMu.RLock()
	model, exists := c.modelCache[key]
	c.modelCacheMu.RUnlock()

	if exists {
		return model, true
	}

	// Check static mapping
	if model, exists = huggingFaceLanguagePairs[key]; exists {
		c.modelCacheMu.Lock()
		c.modelCache[key] = model
		c.modelCacheMu.Unlock()
		return model, true
	}

	return "", false
}

// Translate translates text using Hugging Face (self-hosted or API)
func (c *HuggingFaceClient) Translate(ctx context.Context, text, sourceLang, targetLang string) (*TranslationResult, error) {
	start := time.Now()

	if !c.IsConfigured() {
		return nil, fmt.Errorf("Hugging Face not configured")
	}

	// Use different translation methods based on mode
	if c.selfHosted {
		return c.translateSelfHosted(ctx, text, sourceLang, targetLang, start)
	}
	return c.translateAPI(ctx, text, sourceLang, targetLang, start)
}

// translateSelfHosted handles translation via self-hosted huggingface-mt-service
func (c *HuggingFaceClient) translateSelfHosted(ctx context.Context, text, sourceLang, targetLang string, start time.Time) (*TranslationResult, error) {
	// Build request for self-hosted service
	reqBody := selfHostedTranslationRequest{
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
		var errResp hfErrorResponse
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("unsupported language pair %s->%s: %s", sourceLang, targetLang, errResp.Error)
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		c.markUnhealthy("service unavailable")
		return nil, fmt.Errorf("self-hosted HF service unavailable")
	}

	if resp.StatusCode != http.StatusOK {
		var errResp hfErrorResponse
		json.Unmarshal(respBody, &errResp)
		c.markUnhealthy(errResp.Error)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, errResp.Error)
	}

	// Parse self-hosted response
	var result selfHostedTranslationResponse
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
		Provider:       ProviderHuggingFace,
		Latency:        time.Since(start),
	}, nil
}

// translateAPI handles translation via HuggingFace Inference API
func (c *HuggingFaceClient) translateAPI(ctx context.Context, text, sourceLang, targetLang string, start time.Time) (*TranslationResult, error) {
	model, ok := c.getModelForPair(sourceLang, targetLang)
	if !ok {
		return nil, fmt.Errorf("no Hugging Face model available for %s->%s", sourceLang, targetLang)
	}

	// Build request for HF API
	reqBody := hfTranslationRequest{
		Inputs: text,
		Options: map[string]interface{}{
			"wait_for_model": true, // Wait if model is loading
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s", c.baseURL, model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.markUnhealthy(err.Error())
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		c.logger.Warn("Hugging Face rate limit reached")
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Handle model loading
	if resp.StatusCode == http.StatusServiceUnavailable {
		var errResp hfErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.EstimatedTime > 0 {
			c.logger.WithField("estimated_time", errResp.EstimatedTime).Warn("Hugging Face model is loading")
			return nil, fmt.Errorf("model loading, estimated wait: %.0fs", errResp.EstimatedTime)
		}
		c.markUnhealthy("service unavailable")
		return nil, fmt.Errorf("service unavailable")
	}

	if resp.StatusCode != http.StatusOK {
		var errResp hfErrorResponse
		json.Unmarshal(respBody, &errResp)
		c.markUnhealthy(errResp.Error)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, errResp.Error)
	}

	// Parse response - HF returns array of results
	var results []hfTranslationResponse
	if err := json.Unmarshal(respBody, &results); err != nil {
		// Try single object format
		var single hfTranslationResponse
		if err := json.Unmarshal(respBody, &single); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		results = []hfTranslationResponse{single}
	}

	if len(results) == 0 || results[0].TranslationText == "" {
		return nil, fmt.Errorf("empty translation response")
	}

	c.markHealthy()

	return &TranslationResult{
		TranslatedText: results[0].TranslationText,
		SourceLang:     sourceLang,
		TargetLang:     targetLang,
		Provider:       ProviderHuggingFace,
		Latency:        time.Since(start),
	}, nil
}

// TranslateBatch translates multiple texts
func (c *HuggingFaceClient) TranslateBatch(ctx context.Context, texts []string, sourceLang, targetLang string) ([]TranslationResult, error) {
	// Self-hosted service supports native batch processing
	if c.selfHosted {
		return c.translateBatchSelfHosted(ctx, texts, sourceLang, targetLang)
	}

	// For HF API, use concurrent individual requests
	results := make([]TranslationResult, len(texts))
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	// Use semaphore for concurrency control (HF has rate limits)
	sem := make(chan struct{}, 3) // Max 3 concurrent requests

	for i, text := range texts {
		wg.Add(1)
		go func(idx int, txt string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := c.Translate(ctx, txt, sourceLang, targetLang)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errors = append(errors, fmt.Errorf("text %d: %w", idx, err))
				results[idx] = TranslationResult{
					TranslatedText: txt, // Return original on error
					SourceLang:     sourceLang,
					TargetLang:     targetLang,
					Provider:       ProviderHuggingFace,
				}
			} else {
				results[idx] = *result
			}
		}(i, text)
	}

	wg.Wait()

	if len(errors) == len(texts) {
		return nil, fmt.Errorf("all translations failed")
	}

	return results, nil
}

// translateBatchSelfHosted handles batch translation via self-hosted service
func (c *HuggingFaceClient) translateBatchSelfHosted(ctx context.Context, texts []string, sourceLang, targetLang string) ([]TranslationResult, error) {
	start := time.Now()

	reqBody := selfHostedBatchRequest{
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
		var errResp hfErrorResponse
		json.Unmarshal(respBody, &errResp)
		c.markUnhealthy(errResp.Error)
		return nil, fmt.Errorf("batch API error %d: %s", resp.StatusCode, errResp.Error)
	}

	var batchResp selfHostedBatchResponse
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
			Provider:       ProviderHuggingFace,
			Latency:        latency,
		}
	}

	return results, nil
}

// markHealthy marks the provider as healthy
func (c *HuggingFaceClient) markHealthy() {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	c.healthy = true
	c.lastHealthy = time.Now()
	c.failureCount = 0
}

// markUnhealthy marks the provider as unhealthy
func (c *HuggingFaceClient) markUnhealthy(reason string) {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	c.healthy = false
	c.failureCount++
	c.logger.WithFields(logrus.Fields{
		"reason":        reason,
		"failure_count": c.failureCount,
	}).Warn("Hugging Face marked unhealthy")
}

// GetSupportedLanguages returns all supported language codes
func (c *HuggingFaceClient) GetSupportedLanguages() []string {
	languages := make(map[string]bool)
	for pair := range huggingFaceLanguagePairs {
		parts := strings.Split(pair, "-")
		if len(parts) == 2 {
			languages[parts[0]] = true
			languages[parts[1]] = true
		}
	}

	result := make([]string, 0, len(languages))
	for lang := range languages {
		result = append(result, lang)
	}
	return result
}
