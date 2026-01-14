package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/tesseract-hub/translation-service/internal/models"
)

// TranslationCache provides Redis-based caching for translations
type TranslationCache struct {
	client *redis.Client
	ttl    time.Duration
	logger *logrus.Entry
}

// CachedTranslation represents a cached translation entry
type CachedTranslation struct {
	TranslatedText string    `json:"translated_text"`
	SourceLang     string    `json:"source_lang"`
	TargetLang     string    `json:"target_lang"`
	Provider       string    `json:"provider"`
	CachedAt       time.Time `json:"cached_at"`
}

// NewTranslationCache creates a new Redis cache instance
func NewTranslationCache(host string, port int, password string, db int, ttl time.Duration, logger *logrus.Entry) (*TranslationCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Password:     password,
		DB:           db,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Connected to Redis cache")

	return &TranslationCache{
		client: client,
		ttl:    ttl,
		logger: logger,
	}, nil
}

// generateKey creates a cache key for a translation
func (c *TranslationCache) generateKey(tenantID, sourceLang, targetLang, context string, sourceHash string) string {
	return fmt.Sprintf("trans:%s:%s:%s:%s:%s", tenantID, sourceLang, targetLang, context, sourceHash)
}

// Get retrieves a cached translation
func (c *TranslationCache) Get(ctx context.Context, tenantID, sourceLang, targetLang, sourceText, translationContext string) (*CachedTranslation, error) {
	sourceHash := models.GenerateSourceHash(sourceLang, targetLang, sourceText, translationContext)
	key := c.generateKey(tenantID, sourceLang, targetLang, translationContext, sourceHash)

	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		c.logger.WithError(err).Warn("Failed to get from cache")
		return nil, nil // Treat errors as cache miss
	}

	var cached CachedTranslation
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		c.logger.WithError(err).Warn("Failed to unmarshal cached translation")
		return nil, nil
	}

	return &cached, nil
}

// Set stores a translation in cache
func (c *TranslationCache) Set(ctx context.Context, tenantID, sourceLang, targetLang, sourceText, translatedText, translationContext, provider string) error {
	sourceHash := models.GenerateSourceHash(sourceLang, targetLang, sourceText, translationContext)
	key := c.generateKey(tenantID, sourceLang, targetLang, translationContext, sourceHash)

	cached := CachedTranslation{
		TranslatedText: translatedText,
		SourceLang:     sourceLang,
		TargetLang:     targetLang,
		Provider:       provider,
		CachedAt:       time.Now(),
	}

	val, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal translation: %w", err)
	}

	if err := c.client.Set(ctx, key, val, c.ttl).Err(); err != nil {
		c.logger.WithError(err).Warn("Failed to set cache")
		return err
	}

	return nil
}

// Delete removes a translation from cache
func (c *TranslationCache) Delete(ctx context.Context, tenantID, sourceLang, targetLang, sourceText, translationContext string) error {
	sourceHash := models.GenerateSourceHash(sourceLang, targetLang, sourceText, translationContext)
	key := c.generateKey(tenantID, sourceLang, targetLang, translationContext, sourceHash)

	return c.client.Del(ctx, key).Err()
}

// InvalidateTenant removes all cached translations for a tenant
func (c *TranslationCache) InvalidateTenant(ctx context.Context, tenantID string) error {
	pattern := fmt.Sprintf("trans:%s:*", tenantID)
	return c.deleteByPattern(ctx, pattern)
}

// InvalidateLanguagePair removes all cached translations for a language pair
func (c *TranslationCache) InvalidateLanguagePair(ctx context.Context, tenantID, sourceLang, targetLang string) error {
	pattern := fmt.Sprintf("trans:%s:%s:%s:*", tenantID, sourceLang, targetLang)
	return c.deleteByPattern(ctx, pattern)
}

// deleteByPattern deletes all keys matching a pattern
func (c *TranslationCache) deleteByPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	var deletedCount int64

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan keys: %w", err)
		}

		if len(keys) > 0 {
			deleted, err := c.client.Del(ctx, keys...).Result()
			if err != nil {
				c.logger.WithError(err).Warn("Failed to delete keys")
			} else {
				deletedCount += deleted
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	c.logger.WithField("deleted_count", deletedCount).Info("Invalidated cache entries")
	return nil
}

// GetStats returns cache statistics
func (c *TranslationCache) GetStats(ctx context.Context) (map[string]interface{}, error) {
	info, err := c.client.Info(ctx, "stats", "memory").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis info: %w", err)
	}

	dbSize, err := c.client.DBSize(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get DB size: %w", err)
	}

	return map[string]interface{}{
		"db_size": dbSize,
		"info":    info,
	}, nil
}

// HealthCheck checks if Redis is healthy
func (c *TranslationCache) HealthCheck(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (c *TranslationCache) Close() error {
	return c.client.Close()
}
