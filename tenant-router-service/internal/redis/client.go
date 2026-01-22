package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"tenant-router-service/internal/config"
)

const (
	// PlatformSettingsPrefix is the prefix for platform-wide settings in Redis
	PlatformSettingsPrefix = "platform:settings:"

	// CustomDomainGatewayIPKey is the Redis key for the custom domain gateway IP
	CustomDomainGatewayIPKey = PlatformSettingsPrefix + "custom_domain_gateway_ip"

	// DefaultTTL is the default TTL for platform settings (24 hours)
	// The value will be refreshed periodically, so even if TTL expires,
	// it will be re-populated on the next sync
	DefaultTTL = 24 * time.Hour
)

// Client wraps the Redis client with platform-specific operations
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis client
func NewClient(cfg config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}

// SetCustomDomainGatewayIP stores the custom domain gateway LoadBalancer IP
func (c *Client) SetCustomDomainGatewayIP(ctx context.Context, ip string) error {
	if ip == "" {
		return fmt.Errorf("gateway IP cannot be empty")
	}

	err := c.rdb.Set(ctx, CustomDomainGatewayIPKey, ip, DefaultTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to set gateway IP in Redis: %w", err)
	}

	log.Printf("[Redis] Stored custom domain gateway IP: %s (TTL: %v)", ip, DefaultTTL)
	return nil
}

// GetCustomDomainGatewayIP retrieves the custom domain gateway LoadBalancer IP
func (c *Client) GetCustomDomainGatewayIP(ctx context.Context) (string, error) {
	ip, err := c.rdb.Get(ctx, CustomDomainGatewayIPKey).Result()
	if err == redis.Nil {
		return "", nil // Key doesn't exist
	}
	if err != nil {
		return "", fmt.Errorf("failed to get gateway IP from Redis: %w", err)
	}
	return ip, nil
}
