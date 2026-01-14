package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// GoogleTranslateClient handles communication with Google Cloud Translation API
// Used as the final fallback for languages not supported by other providers
// Priority: 3 (last resort due to cost)
type GoogleTranslateClient struct {
	apiKey     string
	httpClient *http.Client
	logger     *logrus.Entry
	baseURL    string
	priority   int

	// Health tracking
	healthy      bool
	lastHealthy  time.Time
	failureCount int
	healthMu     sync.RWMutex
}

// GoogleTranslateRequest represents a translation request to Google API
type GoogleTranslateRequest struct {
	Q      []string `json:"q"`
	Source string   `json:"source,omitempty"`
	Target string   `json:"target"`
	Format string   `json:"format,omitempty"`
}

// GoogleTranslateResponse represents a translation response from Google API
type GoogleTranslateResponse struct {
	Data struct {
		Translations []struct {
			TranslatedText         string `json:"translatedText"`
			DetectedSourceLanguage string `json:"detectedSourceLanguage,omitempty"`
		} `json:"translations"`
	} `json:"data"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GoogleLanguagesResponse represents the languages response from Google API
type GoogleLanguagesResponse struct {
	Data struct {
		Languages []struct {
			Language string `json:"language"`
			Name     string `json:"name,omitempty"`
		} `json:"languages"`
	} `json:"data"`
}

// NewGoogleTranslateClient creates a new Google Translate client
func NewGoogleTranslateClient(apiKey string, logger *logrus.Entry) *GoogleTranslateClient {
	return &GoogleTranslateClient{
		apiKey:  apiKey,
		baseURL: "https://translation.googleapis.com/language/translate/v2",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:   logger,
		priority: 3, // Lowest priority (paid service, use as last resort)
		healthy:  true,
	}
}

// Name returns the provider name
func (c *GoogleTranslateClient) Name() ProviderName {
	return ProviderGoogle
}

// Priority returns the provider priority (3 = last resort)
func (c *GoogleTranslateClient) Priority() int {
	return c.priority
}

// IsConfigured returns true if the Google Translate client is properly configured
func (c *GoogleTranslateClient) IsConfigured() bool {
	return c.apiKey != ""
}

// IsHealthy checks if the provider is currently healthy
func (c *GoogleTranslateClient) IsHealthy(ctx context.Context) bool {
	c.healthMu.RLock()
	healthy := c.healthy
	lastHealthy := c.lastHealthy
	failureCount := c.failureCount
	c.healthMu.RUnlock()

	// If unhealthy, check if enough time has passed for retry
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

// SupportsLanguagePair checks if Google supports the language pair (almost always true)
func (c *GoogleTranslateClient) SupportsLanguagePair(sourceLang, targetLang string) bool {
	// Google Translate supports almost all language pairs
	// We could query the API but it's expensive, so we assume support
	// Only filter out clearly unsupported codes
	unsupported := map[string]bool{
		"xx": true, // Invalid code
	}
	return !unsupported[sourceLang] && !unsupported[targetLang]
}

// markHealthy marks the provider as healthy
func (c *GoogleTranslateClient) markHealthy() {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	c.healthy = true
	c.lastHealthy = time.Now()
	c.failureCount = 0
}

// markUnhealthy marks the provider as unhealthy
func (c *GoogleTranslateClient) markUnhealthy(reason string) {
	c.healthMu.Lock()
	defer c.healthMu.Unlock()
	c.healthy = false
	c.failureCount++
	c.logger.WithFields(logrus.Fields{
		"reason":        reason,
		"failure_count": c.failureCount,
	}).Warn("Google Translate marked unhealthy")
}

// TranslateText translates text and returns string (legacy method for backward compatibility)
func (c *GoogleTranslateClient) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	result, err := c.Translate(ctx, text, sourceLang, targetLang)
	if err != nil {
		return "", err
	}
	return result.TranslatedText, nil
}

// Translate translates text from source to target language using Google API (implements TranslationProvider)
func (c *GoogleTranslateClient) Translate(ctx context.Context, text, sourceLang, targetLang string) (*TranslationResult, error) {
	start := time.Now()

	if !c.IsConfigured() {
		return nil, fmt.Errorf("Google Translate API key not configured")
	}

	// Build URL with API key
	translateURL := fmt.Sprintf("%s?key=%s", c.baseURL, url.QueryEscape(c.apiKey))

	req := GoogleTranslateRequest{
		Q:      []string{text},
		Target: targetLang,
		Format: "text",
	}

	if sourceLang != "" && sourceLang != "auto" {
		req.Source = sourceLang
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", translateURL, bytes.NewReader(body))
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

	bodyBytes, _ := io.ReadAll(resp.Body)

	var result GoogleTranslateResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		errMsg := fmt.Sprintf("Google API error %d: %s", result.Error.Code, result.Error.Message)
		c.markUnhealthy(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	if len(result.Data.Translations) == 0 {
		return nil, fmt.Errorf("no translation returned")
	}

	c.markHealthy()

	// Handle auto-detected source language
	detectedSource := sourceLang
	if sourceLang == "" || sourceLang == "auto" {
		if result.Data.Translations[0].DetectedSourceLanguage != "" {
			detectedSource = result.Data.Translations[0].DetectedSourceLanguage
		} else {
			detectedSource = "en" // Default
		}
	}

	return &TranslationResult{
		TranslatedText: result.Data.Translations[0].TranslatedText,
		SourceLang:     detectedSource,
		TargetLang:     targetLang,
		Provider:       ProviderGoogle,
		Latency:        time.Since(start),
	}, nil
}

// TranslateBatch translates multiple texts in a batch (implements TranslationProvider)
func (c *GoogleTranslateClient) TranslateBatch(ctx context.Context, texts []string, sourceLang, targetLang string) ([]TranslationResult, error) {
	start := time.Now()

	if !c.IsConfigured() {
		return nil, fmt.Errorf("Google Translate API key not configured")
	}

	// Google API supports batch translation natively
	translateURL := fmt.Sprintf("%s?key=%s", c.baseURL, url.QueryEscape(c.apiKey))

	req := GoogleTranslateRequest{
		Q:      texts,
		Target: targetLang,
		Format: "text",
	}

	if sourceLang != "" && sourceLang != "auto" {
		req.Source = sourceLang
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", translateURL, bytes.NewReader(body))
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

	bodyBytes, _ := io.ReadAll(resp.Body)

	var result GoogleTranslateResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		errMsg := fmt.Sprintf("Google API error %d: %s", result.Error.Code, result.Error.Message)
		c.markUnhealthy(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	c.markHealthy()
	latency := time.Since(start)

	results := make([]TranslationResult, len(result.Data.Translations))
	for i, t := range result.Data.Translations {
		detectedSource := sourceLang
		if sourceLang == "" || sourceLang == "auto" {
			if t.DetectedSourceLanguage != "" {
				detectedSource = t.DetectedSourceLanguage
			} else {
				detectedSource = "en"
			}
		}
		results[i] = TranslationResult{
			TranslatedText: t.TranslatedText,
			SourceLang:     detectedSource,
			TargetLang:     targetLang,
			Provider:       ProviderGoogle,
			Latency:        latency,
		}
	}

	return results, nil
}

// TranslateBatchStrings is a legacy method returning strings for backward compatibility
func (c *GoogleTranslateClient) TranslateBatchStrings(ctx context.Context, texts []string, sourceLang, targetLang string) ([]string, error) {
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

// GetSupportedLanguages returns the list of languages supported by Google Translate
// This is a static list of commonly used languages that LibreTranslate doesn't support
func (c *GoogleTranslateClient) GetSupportedLanguages() []string {
	// Languages that Google supports but LibreTranslate (Argos) doesn't
	return []string{
		"mr",  // Marathi
		"ta",  // Tamil
		"te",  // Telugu
		"bn",  // Bengali (Google has better support)
		"gu",  // Gujarati
		"kn",  // Kannada
		"ml",  // Malayalam
		"pa",  // Punjabi
		"or",  // Odia
		"as",  // Assamese
		"ne",  // Nepali
		"si",  // Sinhala
		"my",  // Myanmar (Burmese)
		"km",  // Khmer
		"lo",  // Lao
		"am",  // Amharic
		"sw",  // Swahili
		"yo",  // Yoruba
		"ig",  // Igbo
		"ha",  // Hausa
	}
}

// SupportsLanguage checks if Google Translate supports the given language
func (c *GoogleTranslateClient) SupportsLanguage(langCode string) bool {
	for _, lang := range c.GetSupportedLanguages() {
		if lang == langCode {
			return true
		}
	}
	return false
}
