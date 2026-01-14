package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"feature-flags-service/internal/config"
)

// GrowthbookClient wraps Growthbook API calls
type GrowthbookClient struct {
	config     *config.Config
	httpClient *http.Client
	cache      map[string]*CacheEntry
	cacheMu    sync.RWMutex
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// FeaturesResponse represents Growthbook features response
type FeaturesResponse struct {
	Status      int                    `json:"status"`
	Features    map[string]Feature     `json:"features"`
	DateUpdated string                 `json:"dateUpdated,omitempty"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

// Feature represents a single feature flag
type Feature struct {
	DefaultValue interface{}     `json:"defaultValue"`
	Rules        []FeatureRule   `json:"rules,omitempty"`
}

// FeatureRule represents a targeting rule
type FeatureRule struct {
	Condition    map[string]interface{} `json:"condition,omitempty"`
	Force        interface{}            `json:"force,omitempty"`
	Variations   []interface{}          `json:"variations,omitempty"`
	Coverage     float64                `json:"coverage,omitempty"`
	HashAttribute string               `json:"hashAttribute,omitempty"`
}

// NewGrowthbookClient creates a new Growthbook client
func NewGrowthbookClient(cfg *config.Config) *GrowthbookClient {
	return &GrowthbookClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: make(map[string]*CacheEntry),
	}
}

// Health checks if Growthbook is reachable
func (c *GrowthbookClient) Health() error {
	url := fmt.Sprintf("http://%s:%d/healthcheck", c.config.GrowthbookAPIHost, c.config.GrowthbookAPIPort)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("growthbook returned status %d", resp.StatusCode)
	}
	return nil
}

// GetFeatures fetches features from Growthbook
func (c *GrowthbookClient) GetFeatures(clientKey string) (*FeaturesResponse, error) {
	// Check cache first
	if c.config.EnableCache {
		c.cacheMu.RLock()
		if entry, ok := c.cache[clientKey]; ok && time.Now().Before(entry.ExpiresAt) {
			c.cacheMu.RUnlock()
			return entry.Data.(*FeaturesResponse), nil
		}
		c.cacheMu.RUnlock()
	}

	// Fetch from Growthbook
	url := fmt.Sprintf("http://%s:%d/api/features/%s", c.config.GrowthbookAPIHost, c.config.GrowthbookAPIPort, clientKey)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch features: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var features FeaturesResponse
	if err := json.Unmarshal(body, &features); err != nil {
		return nil, fmt.Errorf("failed to parse features: %w", err)
	}

	// Cache the response
	if c.config.EnableCache {
		c.cacheMu.Lock()
		c.cache[clientKey] = &CacheEntry{
			Data:      &features,
			ExpiresAt: time.Now().Add(time.Duration(c.config.CacheTTLSeconds) * time.Second),
		}
		c.cacheMu.Unlock()
	}

	return &features, nil
}

// EvaluateFeature evaluates a feature for given attributes
func (c *GrowthbookClient) EvaluateFeature(clientKey, featureKey string, attributes map[string]interface{}) (interface{}, error) {
	features, err := c.GetFeatures(clientKey)
	if err != nil {
		return nil, err
	}

	feature, ok := features.Features[featureKey]
	if !ok {
		return nil, fmt.Errorf("feature '%s' not found", featureKey)
	}

	// Simple evaluation - in production, use proper Growthbook SDK
	// This is a simplified version that checks conditions
	for _, rule := range feature.Rules {
		if c.matchesCondition(rule.Condition, attributes) {
			if rule.Force != nil {
				return rule.Force, nil
			}
		}
	}

	return feature.DefaultValue, nil
}

// matchesCondition checks if attributes match a rule condition
func (c *GrowthbookClient) matchesCondition(condition map[string]interface{}, attributes map[string]interface{}) bool {
	if condition == nil {
		return true
	}

	for key, expected := range condition {
		actual, ok := attributes[key]
		if !ok {
			return false
		}

		// Handle $in operator
		if expectedMap, ok := expected.(map[string]interface{}); ok {
			if inValues, ok := expectedMap["$in"].([]interface{}); ok {
				found := false
				for _, v := range inValues {
					if v == actual {
						found = true
						break
					}
				}
				if !found {
					return false
				}
				continue
			}
		}

		// Simple equality
		if expected != actual {
			return false
		}
	}

	return true
}

// InvalidateCache clears the cache for a specific key or all keys
func (c *GrowthbookClient) InvalidateCache(clientKey string) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	if clientKey == "" {
		c.cache = make(map[string]*CacheEntry)
	} else {
		delete(c.cache, clientKey)
	}
}
