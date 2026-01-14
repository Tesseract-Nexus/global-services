package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// L1 cache (in-memory) TTL
	L1CacheTTL = 5 * time.Minute

	// L2 cache (Redis) TTL
	L2CacheTTL = 1 * time.Hour

	// Redis key prefix for exchange rates
	ExchangeRateKeyPrefix = "currency:rate:"

	// Redis key for all rates
	AllRatesKey = "currency:rates:all"

	// Redis key for supported currencies
	SupportedCurrenciesKey = "currency:supported"
)

// CachedRate represents a cached exchange rate
type CachedRate struct {
	Rate      float64   `json:"rate"`
	FetchedAt time.Time `json:"fetched_at"`
	CachedAt  time.Time `json:"cached_at"`
}

// L1CacheEntry represents an entry in the L1 cache
type L1CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// CurrencyCache provides multi-layer caching for currency rates
type CurrencyCache struct {
	// L1 cache (in-memory)
	l1Cache sync.Map

	// L2 cache (Redis) - optional
	redisClient *redis.Client

	// Whether Redis is available
	redisEnabled bool
}

// NewCurrencyCache creates a new currency cache
func NewCurrencyCache(redisClient *redis.Client) *CurrencyCache {
	cache := &CurrencyCache{
		redisClient:  redisClient,
		redisEnabled: redisClient != nil,
	}

	// Start background cleanup for L1 cache
	go cache.cleanupL1Cache()

	return cache
}

// NewCurrencyCacheWithoutRedis creates a cache without Redis
func NewCurrencyCacheWithoutRedis() *CurrencyCache {
	cache := &CurrencyCache{
		redisEnabled: false,
	}

	go cache.cleanupL1Cache()

	return cache
}

// GetRate retrieves a rate from cache (L1 first, then L2)
func (c *CurrencyCache) GetRate(baseCurrency, targetCurrency string) (*CachedRate, bool) {
	key := c.rateKey(baseCurrency, targetCurrency)

	// Try L1 cache first
	if entry, ok := c.l1Cache.Load(key); ok {
		l1Entry := entry.(L1CacheEntry)
		if time.Now().Before(l1Entry.ExpiresAt) {
			if rate, ok := l1Entry.Data.(*CachedRate); ok {
				return rate, true
			}
		}
		// Expired, remove from L1
		c.l1Cache.Delete(key)
	}

	// Try L2 cache (Redis)
	if c.redisEnabled {
		if rate, ok := c.getFromRedis(key); ok {
			// Populate L1 cache
			c.setL1Cache(key, rate)
			return rate, true
		}
	}

	return nil, false
}

// SetRate stores a rate in both L1 and L2 caches
func (c *CurrencyCache) SetRate(baseCurrency, targetCurrency string, rate float64, fetchedAt time.Time) {
	key := c.rateKey(baseCurrency, targetCurrency)
	cachedRate := &CachedRate{
		Rate:      rate,
		FetchedAt: fetchedAt,
		CachedAt:  time.Now(),
	}

	// Set in L1 cache
	c.setL1Cache(key, cachedRate)

	// Set in L2 cache (Redis)
	if c.redisEnabled {
		c.setToRedis(key, cachedRate, L2CacheTTL)
	}
}

// GetAllRates retrieves all cached rates
func (c *CurrencyCache) GetAllRates() (map[string]*CachedRate, bool) {
	// Try L1 cache first
	if entry, ok := c.l1Cache.Load(AllRatesKey); ok {
		l1Entry := entry.(L1CacheEntry)
		if time.Now().Before(l1Entry.ExpiresAt) {
			if rates, ok := l1Entry.Data.(map[string]*CachedRate); ok {
				return rates, true
			}
		}
		c.l1Cache.Delete(AllRatesKey)
	}

	// Try L2 cache
	if c.redisEnabled {
		if rates, ok := c.getAllRatesFromRedis(); ok {
			c.setL1Cache(AllRatesKey, rates)
			return rates, true
		}
	}

	return nil, false
}

// SetAllRates stores all rates in cache
func (c *CurrencyCache) SetAllRates(rates map[string]*CachedRate) {
	// Set in L1 cache
	c.setL1Cache(AllRatesKey, rates)

	// Set in L2 cache
	if c.redisEnabled {
		c.setAllRatesToRedis(rates)
	}
}

// InvalidateAll clears all cached rates
func (c *CurrencyCache) InvalidateAll() {
	// Clear L1 cache
	c.l1Cache.Range(func(key, _ interface{}) bool {
		c.l1Cache.Delete(key)
		return true
	})

	// Clear L2 cache
	if c.redisEnabled {
		ctx := context.Background()
		keys, err := c.redisClient.Keys(ctx, ExchangeRateKeyPrefix+"*").Result()
		if err == nil && len(keys) > 0 {
			c.redisClient.Del(ctx, keys...)
		}
		c.redisClient.Del(ctx, AllRatesKey, SupportedCurrenciesKey)
	}
}

// setL1Cache sets a value in the L1 cache
func (c *CurrencyCache) setL1Cache(key string, data interface{}) {
	c.l1Cache.Store(key, L1CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(L1CacheTTL),
	})
}

// rateKey generates a cache key for a currency pair
func (c *CurrencyCache) rateKey(baseCurrency, targetCurrency string) string {
	return fmt.Sprintf("%s%s:%s", ExchangeRateKeyPrefix, baseCurrency, targetCurrency)
}

// getFromRedis retrieves a rate from Redis
func (c *CurrencyCache) getFromRedis(key string) (*CachedRate, bool) {
	ctx := context.Background()
	data, err := c.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var rate CachedRate
	if err := json.Unmarshal(data, &rate); err != nil {
		return nil, false
	}

	return &rate, true
}

// setToRedis stores a rate in Redis
func (c *CurrencyCache) setToRedis(key string, rate *CachedRate, ttl time.Duration) {
	ctx := context.Background()
	data, err := json.Marshal(rate)
	if err != nil {
		return
	}
	c.redisClient.Set(ctx, key, data, ttl)
}

// getAllRatesFromRedis retrieves all rates from Redis
func (c *CurrencyCache) getAllRatesFromRedis() (map[string]*CachedRate, bool) {
	ctx := context.Background()
	data, err := c.redisClient.Get(ctx, AllRatesKey).Bytes()
	if err != nil {
		return nil, false
	}

	var rates map[string]*CachedRate
	if err := json.Unmarshal(data, &rates); err != nil {
		return nil, false
	}

	return rates, true
}

// setAllRatesToRedis stores all rates in Redis
func (c *CurrencyCache) setAllRatesToRedis(rates map[string]*CachedRate) {
	ctx := context.Background()
	data, err := json.Marshal(rates)
	if err != nil {
		return
	}
	c.redisClient.Set(ctx, AllRatesKey, data, L2CacheTTL)
}

// cleanupL1Cache periodically removes expired entries from L1 cache
func (c *CurrencyCache) cleanupL1Cache() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		c.l1Cache.Range(func(key, value interface{}) bool {
			entry := value.(L1CacheEntry)
			if now.After(entry.ExpiresAt) {
				c.l1Cache.Delete(key)
			}
			return true
		})
	}
}
