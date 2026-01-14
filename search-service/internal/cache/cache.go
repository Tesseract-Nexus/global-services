package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// CacheEntry represents a cached item
type CacheEntry struct {
	Data      interface{} `json:"data"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// Cache is an in-memory cache with TTL support
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
	maxSize int
}

// Config holds cache configuration
type Config struct {
	TTL     time.Duration // Time to live for cache entries
	MaxSize int           // Maximum number of entries
}

// DefaultConfig returns sensible defaults for search caching
func DefaultConfig() Config {
	return Config{
		TTL:     30 * time.Second, // Short TTL for search results
		MaxSize: 10000,            // Max entries
	}
}

// NewCache creates a new cache instance
func NewCache(cfg Config) *Cache {
	if cfg.TTL == 0 {
		cfg.TTL = 30 * time.Second
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 10000
	}

	c := &Cache{
		entries: make(map[string]*CacheEntry),
		ttl:     cfg.TTL,
		maxSize: cfg.MaxSize,
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// GenerateKey creates a cache key from search parameters
func GenerateKey(collection, tenantID string, params interface{}) string {
	data, _ := json.Marshal(params)
	hash := sha256.Sum256(append([]byte(collection+":"+tenantID+":"), data...))
	return hex.EncodeToString(hash[:])
}

// Get retrieves an item from cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		c.Delete(key)
		return nil, false
	}

	return entry.Data, true
}

// Set stores an item in cache
func (c *Cache) Set(key string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entries if at max size
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// SetWithTTL stores an item with a custom TTL
func (c *Cache) SetWithTTL(key string, data interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// Delete removes an item from cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// InvalidateByPrefix removes all entries matching a prefix
func (c *Cache) InvalidateByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.entries, key)
		}
	}
}

// InvalidateByTenant removes all entries for a specific tenant
func (c *Cache) InvalidateByTenant(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Since tenant is embedded in key hash, we need to track this differently
	// For now, we'll just clear all entries (simple but effective)
	// In production, consider using a more sophisticated key structure
	c.entries = make(map[string]*CacheEntry)
}

// Clear removes all entries from cache
func (c *Cache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*CacheEntry)
	c.mu.Unlock()
}

// Stats returns cache statistics
func (c *Cache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"entries":  len(c.entries),
		"max_size": c.maxSize,
		"ttl":      c.ttl.String(),
	}
}

// evictOldest removes the oldest 10% of entries
func (c *Cache) evictOldest() {
	// Simple eviction: remove entries that will expire soonest
	toRemove := c.maxSize / 10
	if toRemove == 0 {
		toRemove = 1
	}

	type entryWithKey struct {
		key       string
		expiresAt time.Time
	}

	// Find oldest entries
	var oldest []entryWithKey
	for k, v := range c.entries {
		oldest = append(oldest, entryWithKey{k, v.ExpiresAt})
	}

	// Sort by expiry time (simple bubble sort for small slices)
	for i := 0; i < len(oldest)-1; i++ {
		for j := i + 1; j < len(oldest); j++ {
			if oldest[j].expiresAt.Before(oldest[i].expiresAt) {
				oldest[i], oldest[j] = oldest[j], oldest[i]
			}
		}
	}

	// Remove oldest entries
	for i := 0; i < toRemove && i < len(oldest); i++ {
		delete(c.entries, oldest[i].key)
	}
}

// cleanup periodically removes expired entries
func (c *Cache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.entries {
			if now.After(entry.ExpiresAt) {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

// SearchCache is a specialized cache for search results
type SearchCache struct {
	*Cache
}

// NewSearchCache creates a cache optimized for search results
func NewSearchCache() *SearchCache {
	return &SearchCache{
		Cache: NewCache(Config{
			TTL:     30 * time.Second, // Short TTL for fresh results
			MaxSize: 5000,             // Limit memory usage
		}),
	}
}

// CacheSearchResult caches a search result
func (sc *SearchCache) CacheSearchResult(collection, tenantID, query string, filters map[string]interface{}, result interface{}) {
	key := GenerateKey(collection, tenantID, map[string]interface{}{
		"query":   query,
		"filters": filters,
	})
	sc.Set(key, result)
}

// GetSearchResult retrieves a cached search result
func (sc *SearchCache) GetSearchResult(collection, tenantID, query string, filters map[string]interface{}) (interface{}, bool) {
	key := GenerateKey(collection, tenantID, map[string]interface{}{
		"query":   query,
		"filters": filters,
	})
	return sc.Get(key)
}

// InvalidateCollection invalidates all cached results for a collection
func (sc *SearchCache) InvalidateCollection(collection, tenantID string) {
	// Since we use hash keys, we need to clear all cache
	// In a production system, consider using Redis with proper key patterns
	sc.Clear()
}
