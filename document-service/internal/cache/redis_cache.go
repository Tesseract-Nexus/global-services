package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// Cache defines the cache interface
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	GetJSON(ctx context.Context, key string, dest interface{}) error
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	PoolSize int
}

// RedisCache implements Cache using Redis
type RedisCache struct {
	client *redis.Client
	logger *logrus.Logger
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(cfg RedisConfig, logger *logrus.Logger) (*RedisCache, error) {
	if logger == nil {
		logger = logrus.New()
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	if cfg.Port == "" {
		addr = fmt.Sprintf("%s:6379", cfg.Host)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.WithError(err).Warn("Failed to connect to Redis, cache will be disabled")
		return nil, err
	}

	logger.Info("Connected to Redis cache")
	return &RedisCache{
		client: client,
		logger: logger,
	}, nil
}

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Set stores a value in cache
func (c *RedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// GetJSON retrieves and unmarshals a JSON value from cache
func (c *RedisCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

// SetJSON marshals and stores a JSON value in cache
func (c *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

// Delete removes a value from cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// NoOpCache is a no-op cache for when Redis is unavailable
type NoOpCache struct{}

func NewNoOpCache() *NoOpCache {
	return &NoOpCache{}
}

func (c *NoOpCache) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (c *NoOpCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}

func (c *NoOpCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	return nil
}

func (c *NoOpCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}

func (c *NoOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (c *NoOpCache) Close() error {
	return nil
}

// PresignedURLCacheKey generates a cache key for presigned URLs
// Format: {product}:presigned:{bucket}:{path}:{method}
func PresignedURLCacheKey(productID, bucket, path, method string) string {
	if productID == "" {
		productID = "marketplace" // Default for backwards compatibility
	}
	return fmt.Sprintf("%s:presigned:%s:%s:%s", productID, bucket, path, method)
}

// MetadataCacheKey generates a cache key for document metadata
// Format: {product}:metadata:{bucket}:{path}
func MetadataCacheKey(productID, bucket, path string) string {
	if productID == "" {
		productID = "marketplace" // Default for backwards compatibility
	}
	return fmt.Sprintf("%s:metadata:%s:%s", productID, bucket, path)
}

// CachedPresignedURL represents a cached presigned URL
type CachedPresignedURL struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// IsExpired checks if the cached URL is expired (with 5 min buffer)
func (c *CachedPresignedURL) IsExpired() bool {
	return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}
