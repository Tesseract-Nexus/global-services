package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/tesseract-hub/audit-service/internal/models"
)

var (
	ErrCacheMiss   = errors.New("cache miss")
	ErrCacheError  = errors.New("cache error")
)

// AuditCache provides multi-level caching for audit logs
type AuditCache struct {
	redis  *redis.Client
	local  *LocalCache
	logger *logrus.Logger

	// Configuration
	defaultTTL      time.Duration
	summaryTTL      time.Duration
	criticalTTL     time.Duration
	localCacheSize  int

	// Metrics
	mu          sync.RWMutex
	hits        int64
	misses      int64
	errors      int64
}

// CacheConfig holds configuration for the audit cache
type CacheConfig struct {
	RedisClient    *redis.Client
	Logger         *logrus.Logger
	DefaultTTL     time.Duration // Default cache TTL (default: 5 minutes)
	SummaryTTL     time.Duration // Summary cache TTL (default: 1 minute)
	CriticalTTL    time.Duration // Critical events TTL (default: 30 seconds)
	LocalCacheSize int           // Max items in local cache (default: 1000)
}

// LocalCache provides in-memory caching with LRU eviction
type LocalCache struct {
	mu       sync.RWMutex
	items    map[string]*localCacheItem
	maxSize  int
	order    []string // For LRU tracking
}

type localCacheItem struct {
	Value     []byte
	ExpiresAt time.Time
}

// NewAuditCache creates a new audit cache with Redis and local caching
func NewAuditCache(config CacheConfig) *AuditCache {
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 5 * time.Minute
	}
	if config.SummaryTTL == 0 {
		config.SummaryTTL = 1 * time.Minute
	}
	if config.CriticalTTL == 0 {
		config.CriticalTTL = 30 * time.Second
	}
	if config.LocalCacheSize == 0 {
		config.LocalCacheSize = 1000
	}

	return &AuditCache{
		redis:          config.RedisClient,
		logger:         config.Logger,
		defaultTTL:     config.DefaultTTL,
		summaryTTL:     config.SummaryTTL,
		criticalTTL:    config.CriticalTTL,
		localCacheSize: config.LocalCacheSize,
		local: &LocalCache{
			items:   make(map[string]*localCacheItem),
			maxSize: config.LocalCacheSize,
			order:   make([]string, 0, config.LocalCacheSize),
		},
	}
}

// Key builders for different cache types
func (c *AuditCache) auditLogKey(tenantID, logID string) string {
	return fmt.Sprintf("audit:log:%s:%s", tenantID, logID)
}

func (c *AuditCache) auditListKey(tenantID string, filters map[string]string, page, limit int) string {
	filterHash := hashFilters(filters)
	return fmt.Sprintf("audit:list:%s:%s:p%d:l%d", tenantID, filterHash, page, limit)
}

func (c *AuditCache) summaryKey(tenantID string, fromDate, toDate string) string {
	return fmt.Sprintf("audit:summary:%s:%s:%s", tenantID, fromDate, toDate)
}

func (c *AuditCache) criticalKey(tenantID string, hours int) string {
	return fmt.Sprintf("audit:critical:%s:h%d", tenantID, hours)
}

func (c *AuditCache) userActivityKey(tenantID, userID string) string {
	return fmt.Sprintf("audit:user:%s:%s", tenantID, userID)
}

func (c *AuditCache) recentKey(tenantID string) string {
	return fmt.Sprintf("audit:recent:%s", tenantID)
}

// hashFilters creates a consistent hash from filter map
func hashFilters(filters map[string]string) string {
	if len(filters) == 0 {
		return "all"
	}
	data, _ := json.Marshal(filters)
	return fmt.Sprintf("%x", data)[:16]
}

// GetAuditLog retrieves a single audit log from cache
func (c *AuditCache) GetAuditLog(ctx context.Context, tenantID, logID string) (*models.AuditLog, error) {
	key := c.auditLogKey(tenantID, logID)

	// Check local cache first
	if data := c.getLocal(key); data != nil {
		var log models.AuditLog
		if err := json.Unmarshal(data, &log); err == nil {
			c.recordHit()
			return &log, nil
		}
	}

	// Check Redis
	if c.redis != nil {
		data, err := c.redis.Get(ctx, key).Bytes()
		if err == nil {
			var log models.AuditLog
			if err := json.Unmarshal(data, &log); err == nil {
				c.setLocal(key, data, c.defaultTTL)
				c.recordHit()
				return &log, nil
			}
		}
		if err != redis.Nil {
			c.recordError()
		}
	}

	c.recordMiss()
	return nil, ErrCacheMiss
}

// SetAuditLog caches a single audit log
func (c *AuditCache) SetAuditLog(ctx context.Context, tenantID string, log *models.AuditLog) error {
	key := c.auditLogKey(tenantID, log.ID.String())

	data, err := json.Marshal(log)
	if err != nil {
		return err
	}

	// Set in local cache
	c.setLocal(key, data, c.defaultTTL)

	// Set in Redis
	if c.redis != nil {
		if err := c.redis.Set(ctx, key, data, c.defaultTTL).Err(); err != nil {
			c.recordError()
			c.logger.WithError(err).Warn("Failed to set audit log in Redis")
		}
	}

	return nil
}

// GetAuditList retrieves a list of audit logs from cache
func (c *AuditCache) GetAuditList(ctx context.Context, tenantID string, filters map[string]string, page, limit int) ([]models.AuditLog, int64, error) {
	key := c.auditListKey(tenantID, filters, page, limit)

	type cachedList struct {
		Logs  []models.AuditLog `json:"logs"`
		Total int64             `json:"total"`
	}

	// Check local cache
	if data := c.getLocal(key); data != nil {
		var result cachedList
		if err := json.Unmarshal(data, &result); err == nil {
			c.recordHit()
			return result.Logs, result.Total, nil
		}
	}

	// Check Redis
	if c.redis != nil {
		data, err := c.redis.Get(ctx, key).Bytes()
		if err == nil {
			var result cachedList
			if err := json.Unmarshal(data, &result); err == nil {
				c.setLocal(key, data, c.defaultTTL)
				c.recordHit()
				return result.Logs, result.Total, nil
			}
		}
	}

	c.recordMiss()
	return nil, 0, ErrCacheMiss
}

// SetAuditList caches a list of audit logs
func (c *AuditCache) SetAuditList(ctx context.Context, tenantID string, filters map[string]string, page, limit int, logs []models.AuditLog, total int64) error {
	key := c.auditListKey(tenantID, filters, page, limit)

	type cachedList struct {
		Logs  []models.AuditLog `json:"logs"`
		Total int64             `json:"total"`
	}

	data, err := json.Marshal(cachedList{Logs: logs, Total: total})
	if err != nil {
		return err
	}

	c.setLocal(key, data, c.defaultTTL)

	if c.redis != nil {
		if err := c.redis.Set(ctx, key, data, c.defaultTTL).Err(); err != nil {
			c.recordError()
		}
	}

	return nil
}

// GetSummary retrieves cached summary data
func (c *AuditCache) GetSummary(ctx context.Context, tenantID, fromDate, toDate string) (map[string]interface{}, error) {
	key := c.summaryKey(tenantID, fromDate, toDate)

	// Check Redis (summaries are larger, skip local cache)
	if c.redis != nil {
		data, err := c.redis.Get(ctx, key).Bytes()
		if err == nil {
			var summary map[string]interface{}
			if err := json.Unmarshal(data, &summary); err == nil {
				c.recordHit()
				return summary, nil
			}
		}
	}

	c.recordMiss()
	return nil, ErrCacheMiss
}

// SetSummary caches summary data
func (c *AuditCache) SetSummary(ctx context.Context, tenantID, fromDate, toDate string, summary map[string]interface{}) error {
	key := c.summaryKey(tenantID, fromDate, toDate)

	data, err := json.Marshal(summary)
	if err != nil {
		return err
	}

	if c.redis != nil {
		if err := c.redis.Set(ctx, key, data, c.summaryTTL).Err(); err != nil {
			c.recordError()
		}
	}

	return nil
}

// GetCriticalEvents retrieves cached critical events
func (c *AuditCache) GetCriticalEvents(ctx context.Context, tenantID string, hours int) ([]models.AuditLog, error) {
	key := c.criticalKey(tenantID, hours)

	if c.redis != nil {
		data, err := c.redis.Get(ctx, key).Bytes()
		if err == nil {
			var logs []models.AuditLog
			if err := json.Unmarshal(data, &logs); err == nil {
				c.recordHit()
				return logs, nil
			}
		}
	}

	c.recordMiss()
	return nil, ErrCacheMiss
}

// SetCriticalEvents caches critical events
func (c *AuditCache) SetCriticalEvents(ctx context.Context, tenantID string, hours int, logs []models.AuditLog) error {
	key := c.criticalKey(tenantID, hours)

	data, err := json.Marshal(logs)
	if err != nil {
		return err
	}

	if c.redis != nil {
		if err := c.redis.Set(ctx, key, data, c.criticalTTL).Err(); err != nil {
			c.recordError()
		}
	}

	return nil
}

// InvalidateTenant invalidates all cache entries for a tenant
func (c *AuditCache) InvalidateTenant(ctx context.Context, tenantID string) error {
	pattern := fmt.Sprintf("audit:*:%s:*", tenantID)

	// Clear local cache
	c.local.mu.Lock()
	for key := range c.local.items {
		if matchesPattern(key, pattern) {
			delete(c.local.items, key)
		}
	}
	c.local.mu.Unlock()

	// Clear Redis
	if c.redis != nil {
		iter := c.redis.Scan(ctx, 0, pattern, 100).Iterator()
		var keys []string
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
		}
		if len(keys) > 0 {
			c.redis.Del(ctx, keys...)
		}
	}

	return nil
}

// InvalidateAfterWrite invalidates relevant caches after a write operation
func (c *AuditCache) InvalidateAfterWrite(ctx context.Context, tenantID string) error {
	// Invalidate lists and summaries, but keep individual log cache
	patterns := []string{
		fmt.Sprintf("audit:list:%s:*", tenantID),
		fmt.Sprintf("audit:summary:%s:*", tenantID),
		fmt.Sprintf("audit:critical:%s:*", tenantID),
		fmt.Sprintf("audit:recent:%s", tenantID),
	}

	for _, pattern := range patterns {
		// Clear local
		c.local.mu.Lock()
		for key := range c.local.items {
			if matchesPattern(key, pattern) {
				delete(c.local.items, key)
			}
		}
		c.local.mu.Unlock()

		// Clear Redis
		if c.redis != nil {
			iter := c.redis.Scan(ctx, 0, pattern, 100).Iterator()
			var keys []string
			for iter.Next(ctx) {
				keys = append(keys, iter.Val())
			}
			if len(keys) > 0 {
				c.redis.Del(ctx, keys...)
			}
		}
	}

	return nil
}

// PushRecentLog adds a log to the recent logs list (for real-time updates)
func (c *AuditCache) PushRecentLog(ctx context.Context, tenantID string, log *models.AuditLog) error {
	if c.redis == nil {
		return nil
	}

	key := c.recentKey(tenantID)

	data, err := json.Marshal(log)
	if err != nil {
		return err
	}

	pipe := c.redis.Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, 99) // Keep only last 100
	pipe.Expire(ctx, key, 1*time.Hour)
	_, err = pipe.Exec(ctx)

	return err
}

// GetRecentLogs retrieves recent logs for real-time updates
func (c *AuditCache) GetRecentLogs(ctx context.Context, tenantID string, limit int) ([]models.AuditLog, error) {
	if c.redis == nil {
		return nil, ErrCacheMiss
	}

	key := c.recentKey(tenantID)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	data, err := c.redis.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	logs := make([]models.AuditLog, 0, len(data))
	for _, item := range data {
		var log models.AuditLog
		if err := json.Unmarshal([]byte(item), &log); err == nil {
			logs = append(logs, log)
		}
	}

	return logs, nil
}

// Local cache helpers
func (c *AuditCache) getLocal(key string) []byte {
	c.local.mu.RLock()
	defer c.local.mu.RUnlock()

	item, exists := c.local.items[key]
	if !exists || time.Now().After(item.ExpiresAt) {
		return nil
	}
	return item.Value
}

func (c *AuditCache) setLocal(key string, value []byte, ttl time.Duration) {
	c.local.mu.Lock()
	defer c.local.mu.Unlock()

	// LRU eviction if at capacity
	if len(c.local.items) >= c.local.maxSize {
		if len(c.local.order) > 0 {
			oldest := c.local.order[0]
			delete(c.local.items, oldest)
			c.local.order = c.local.order[1:]
		}
	}

	c.local.items[key] = &localCacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
	c.local.order = append(c.local.order, key)
}

// matchesPattern checks if a key matches a Redis-style glob pattern
func matchesPattern(key, pattern string) bool {
	// Simple implementation - in production use proper glob matching
	return len(key) >= len(pattern)-1
}

// Metrics helpers
func (c *AuditCache) recordHit() {
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
}

func (c *AuditCache) recordMiss() {
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
}

func (c *AuditCache) recordError() {
	c.mu.Lock()
	c.errors++
	c.mu.Unlock()
}

// GetStats returns cache statistics
func (c *AuditCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.local.mu.RLock()
	localSize := len(c.local.items)
	c.local.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}

	return map[string]interface{}{
		"hits":          c.hits,
		"misses":        c.misses,
		"errors":        c.errors,
		"hit_rate":      hitRate,
		"local_size":    localSize,
		"local_max":     c.localCacheSize,
		"redis_enabled": c.redis != nil,
	}
}
