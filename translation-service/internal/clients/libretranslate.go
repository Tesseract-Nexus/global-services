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

// LibreTranslateClient handles communication with LibreTranslate API
// Primary translation provider - open source, self-hosted
type LibreTranslateClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *logrus.Entry
	priority   int

	// Language cache
	mu        sync.RWMutex
	languages []LibreTranslateLanguage
	lastFetch time.Time

	// Health tracking
	healthy      bool
	lastHealthy  time.Time
	failureCount int
	healthMu     sync.RWMutex
}

// LibreTranslateLanguage represents a language from LibreTranslate
type LibreTranslateLanguage struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// LibreTranslateRequest represents a translation request to LibreTranslate
type LibreTranslateRequest struct {
	Q       string `json:"q"`
	Source  string `json:"source"`
	Target  string `json:"target"`
	Format  string `json:"format,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
}

// LibreTranslateResponse represents a translation response from LibreTranslate
type LibreTranslateResponse struct {
	TranslatedText string `json:"translatedText"`
}

// LibreTranslateDetectRequest represents a language detection request
type LibreTranslateDetectRequest struct {
	Q      string `json:"q"`
	APIKey string `json:"api_key,omitempty"`
}

// LibreTranslateDetectResponse represents a language detection response
type LibreTranslateDetectResponse struct {
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
}

// NewLibreTranslateClient creates a new LibreTranslate client
func NewLibreTranslateClient(baseURL, apiKey string, logger *logrus.Entry) *LibreTranslateClient {
	return &LibreTranslateClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		logger:   logger,
		priority: 1, // Highest priority (primary provider)
		healthy:  true,
	}
}

// Name returns the provider name
func (c *LibreTranslateClient) Name() ProviderName {
	return ProviderLibreTranslate
}

// Priority returns the provider priority (1 = highest)
func (c *LibreTranslateClient) Priority() int {
	return c.priority
}

// IsConfigured returns true if the client is configured
func (c *LibreTranslateClient) IsConfigured() bool {
	return c.baseURL != ""
}

// IsHealthy checks if the provider is currently healthy
func (c *LibreTranslateClient) IsHealthy(ctx context.Context) bool {
	c.healthMu.RLock()
	healthy := c.healthy
	lastHealthy := c.lastHealthy
	failureCount := c.failureCount
	c.healthMu.RUnlock()

	// If unhealthy, check if enough time has passed for retry
	if !healthy && failureCount > 0 {
		backoffDuration := time.Duration(failureCount) * 10 * time.Second
		if backoffDuration > 2*time.Minute {
			backoffDuration = 2 * time.Minute
		}
		if time.Since(lastHealthy) < backoffDuration {
			return false
		}
	}

	return healthy
}

// SupportsLanguagePair checks if LibreTranslate supports the language pair
func (c *LibreTranslateClient) SupportsLanguagePair(sourceLang, targetLang string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	languages, err := c.GetLanguages(ctx)
	if err != nil {
		return false
	}

	sourceSupported := false
	targetSupported := false
	for _, lang := range languages {
		if lang.Code == sourceLang {
			sourceSupported = true
		}
		if lang.Code == targetLang {
			targetSupported = true
		}
	}

	return sourceSupported && targetSupported
}

// markHealthy marks the provider as healthy
func (c *LibreTranslateClient) markHealthy() {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	c.healthy = true
	c.lastHealthy = time.Now()
	c.failureCount = 0
}

// markUnhealthy marks the provider as unhealthy
func (c *LibreTranslateClient) markUnhealthy(reason string) {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	c.healthy = false
	c.failureCount++
	c.logger.WithFields(logrus.Fields{
		"reason":        reason,
		"failure_count": c.failureCount,
	}).Warn("LibreTranslate marked unhealthy")
}

// TranslateText translates text from source to target language (legacy method for backward compatibility)
func (c *LibreTranslateClient) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	result, err := c.Translate(ctx, text, sourceLang, targetLang)
	if err != nil {
		return "", err
	}
	return result.TranslatedText, nil
}

// Translate translates text from source to target language (implements TranslationProvider)
func (c *LibreTranslateClient) Translate(ctx context.Context, text, sourceLang, targetLang string) (*TranslationResult, error) {
	start := time.Now()

	// Auto-detect source language if not specified
	if sourceLang == "" || sourceLang == "auto" {
		detected, err := c.DetectLanguage(ctx, text)
		if err != nil {
			c.logger.WithError(err).Warn("Failed to detect language, defaulting to English")
			sourceLang = "en"
		} else {
			sourceLang = detected.Language
		}
	}

	req := LibreTranslateRequest{
		Q:      text,
		Source: sourceLang,
		Target: targetLang,
		Format: "text",
	}

	if c.apiKey != "" {
		req.APIKey = c.apiKey
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/translate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.markUnhealthy(err.Error())
		return nil, fmt.Errorf("translation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("translation API returned status %d: %s", resp.StatusCode, string(bodyBytes))
		c.markUnhealthy(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	var result LibreTranslateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.markHealthy()

	return &TranslationResult{
		TranslatedText: result.TranslatedText,
		SourceLang:     sourceLang,
		TargetLang:     targetLang,
		Provider:       ProviderLibreTranslate,
		Latency:        time.Since(start),
	}, nil
}

// TranslateBatch translates multiple texts in a batch (implements TranslationProvider)
func (c *LibreTranslateClient) TranslateBatch(ctx context.Context, texts []string, sourceLang, targetLang string) ([]TranslationResult, error) {
	results := make([]TranslationResult, len(texts))
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	// Use semaphore for concurrency control
	sem := make(chan struct{}, 10) // Max 10 concurrent requests

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
					Provider:       ProviderLibreTranslate,
				}
			} else {
				results[idx] = *result
			}
		}(i, text)
	}

	wg.Wait()

	if len(errors) > 0 {
		c.logger.WithField("error_count", len(errors)).Warn("Some translations failed in batch")
	}

	if len(errors) == len(texts) {
		return nil, fmt.Errorf("all translations failed")
	}

	return results, nil
}

// TranslateBatchStrings is a legacy method returning strings for backward compatibility
func (c *LibreTranslateClient) TranslateBatchStrings(ctx context.Context, texts []string, sourceLang, targetLang string) ([]string, error) {
	results, err := c.TranslateBatch(ctx, texts, sourceLang, targetLang)
	if err != nil {
		return nil, err
	}

	strings := make([]string, len(results))
	for i, r := range results {
		strings[i] = r.TranslatedText
	}
	return strings, nil
}

// DetectLanguage detects the language of the given text
func (c *LibreTranslateClient) DetectLanguage(ctx context.Context, text string) (*LibreTranslateDetectResponse, error) {
	req := LibreTranslateDetectRequest{
		Q: text,
	}

	if c.apiKey != "" {
		req.APIKey = c.apiKey
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/detect", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("detect request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("detect API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// LibreTranslate returns an array of detections
	var detections []LibreTranslateDetectResponse
	if err := json.NewDecoder(resp.Body).Decode(&detections); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(detections) == 0 {
		return nil, fmt.Errorf("no language detected")
	}

	return &detections[0], nil
}

// GetLanguages returns the list of supported languages
func (c *LibreTranslateClient) GetLanguages(ctx context.Context) ([]LibreTranslateLanguage, error) {
	c.mu.RLock()
	// Cache languages for 1 hour
	if len(c.languages) > 0 && time.Since(c.lastFetch) < time.Hour {
		languages := c.languages
		c.mu.RUnlock()
		return languages, nil
	}
	c.mu.RUnlock()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/languages", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("languages request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("languages API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var languages []LibreTranslateLanguage
	if err := json.NewDecoder(resp.Body).Decode(&languages); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.mu.Lock()
	c.languages = languages
	c.lastFetch = time.Now()
	c.mu.Unlock()

	return languages, nil
}

// HealthCheck checks if the LibreTranslate service is healthy
func (c *LibreTranslateClient) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/languages", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service unhealthy, status: %d", resp.StatusCode)
	}

	return nil
}
