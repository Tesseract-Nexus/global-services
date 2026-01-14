package tenant

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

var (
	ErrTenantNotFound      = errors.New("tenant not found in registry")
	ErrInvalidCredentials  = errors.New("invalid or corrupted credentials")
	ErrRegistryUnavailable = errors.New("tenant registry service unavailable")
	ErrInvalidTenantID     = errors.New("invalid tenant ID format")
	ErrTenantInactive      = errors.New("tenant is inactive")
	ErrAuditDisabled       = errors.New("audit logs disabled for tenant")
)

// UUID regex for tenant ID validation
var tenantIDRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// DatabaseConfig holds the connection configuration for a tenant's database
type DatabaseConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"` // Encrypted at rest
	DatabaseName string `json:"database_name"`
	SSLMode      string `json:"ssl_mode"`
	MaxOpenConns int    `json:"max_open_conns"`
	MaxIdleConns int    `json:"max_idle_conns"`
	MaxLifetime  int    `json:"max_lifetime_seconds"` // Connection max lifetime in seconds
}

// TenantInfo contains all tenant-specific configuration
type TenantInfo struct {
	TenantID      string         `json:"tenant_id"`
	ProductID     string         `json:"product_id"`
	ProductName   string         `json:"product_name"`
	VendorID      string         `json:"vendor_id"`
	VendorName    string         `json:"vendor_name"`
	DatabaseConfig DatabaseConfig `json:"database_config"`
	IsActive      bool           `json:"is_active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`

	// Feature flags for this tenant
	Features      TenantFeatures `json:"features"`
}

// TenantFeatures controls what features are enabled for the tenant
type TenantFeatures struct {
	AuditLogsEnabled     bool `json:"audit_logs_enabled"`
	RealTimeEnabled      bool `json:"real_time_enabled"`
	ExportEnabled        bool `json:"export_enabled"`
	RetentionDays        int  `json:"retention_days"` // 0 = unlimited
	MaxLogsPerDay        int  `json:"max_logs_per_day"` // 0 = unlimited
	EncryptionAtRest     bool `json:"encryption_at_rest"`
}

// Registry manages tenant configurations with caching and dynamic refresh
type Registry struct {
	mu              sync.RWMutex
	cache           map[string]*TenantInfo
	cacheExpiry     map[string]time.Time
	cacheTTL        time.Duration

	redisClient     *redis.Client
	registryURL     string // URL to tenant registry service
	encryptionKey   []byte // For decrypting database credentials

	logger          *logrus.Logger
	httpClient      *http.Client

	// Retry configuration
	maxRetries      int
	retryBaseDelay  time.Duration
	retryMaxDelay   time.Duration

	// Metrics
	cacheHits       int64
	cacheMisses     int64
	registryErrors  int64
	retryCount      int64
}

// RegistryConfig holds configuration for the tenant registry
type RegistryConfig struct {
	RegistryURL     string        // URL to tenant/settings service for fetching tenant configs
	EncryptionKey   string        // Base64 encoded AES-256 key for credential decryption
	CacheTTL        time.Duration // How long to cache tenant info (default: 5 minutes)
	RedisClient     *redis.Client // Optional Redis for distributed caching
	Logger          *logrus.Logger

	// Retry configuration (with sensible defaults)
	MaxRetries      int           // Max retry attempts (default: 3)
	RetryBaseDelay  time.Duration // Base delay for exponential backoff (default: 100ms)
	RetryMaxDelay   time.Duration // Max delay between retries (default: 2s)
}

// NewRegistry creates a new tenant registry with caching
func NewRegistry(config RegistryConfig) (*Registry, error) {
	// Apply defaults
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryBaseDelay == 0 {
		config.RetryBaseDelay = 100 * time.Millisecond
	}
	if config.RetryMaxDelay == 0 {
		config.RetryMaxDelay = 2 * time.Second
	}

	// Validate registry URL
	if config.RegistryURL == "" {
		return nil, fmt.Errorf("registry URL is required")
	}

	var encKey []byte
	if config.EncryptionKey != "" {
		var err error
		encKey, err = base64.StdEncoding.DecodeString(config.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("invalid encryption key: %w", err)
		}
		if len(encKey) != 32 {
			return nil, fmt.Errorf("encryption key must be 32 bytes (AES-256)")
		}
	}

	return &Registry{
		cache:          make(map[string]*TenantInfo),
		cacheExpiry:    make(map[string]time.Time),
		cacheTTL:       config.CacheTTL,
		redisClient:    config.RedisClient,
		registryURL:    config.RegistryURL,
		encryptionKey:  encKey,
		logger:         config.Logger,
		maxRetries:     config.MaxRetries,
		retryBaseDelay: config.RetryBaseDelay,
		retryMaxDelay:  config.RetryMaxDelay,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// ValidateTenantID validates the tenant ID format
func (r *Registry) ValidateTenantID(tenantID string) error {
	if tenantID == "" {
		return fmt.Errorf("%w: tenant ID cannot be empty", ErrInvalidTenantID)
	}
	if !tenantIDRegex.MatchString(tenantID) {
		return fmt.Errorf("%w: expected UUID format", ErrInvalidTenantID)
	}
	return nil
}

// GetTenant retrieves tenant configuration with multi-level caching
// Cache hierarchy: Local Memory -> Redis -> Registry Service
func (r *Registry) GetTenant(ctx context.Context, tenantID string) (*TenantInfo, error) {
	// Validate tenant ID format
	if err := r.ValidateTenantID(tenantID); err != nil {
		return nil, err
	}

	// Check context cancellation early
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Level 1: Check local memory cache
	if info := r.getFromLocalCache(tenantID); info != nil {
		// Validate cached tenant is still active
		if !info.IsActive {
			r.InvalidateCache(ctx, tenantID)
			return nil, ErrTenantInactive
		}
		r.mu.Lock()
		r.cacheHits++
		r.mu.Unlock()
		return info, nil
	}

	// Level 2: Check Redis cache (if available)
	if r.redisClient != nil {
		if info := r.getFromRedisCache(ctx, tenantID); info != nil {
			if !info.IsActive {
				r.InvalidateCache(ctx, tenantID)
				return nil, ErrTenantInactive
			}
			r.setLocalCache(tenantID, info)
			r.mu.Lock()
			r.cacheHits++
			r.mu.Unlock()
			return info, nil
		}
	}

	r.mu.Lock()
	r.cacheMisses++
	r.mu.Unlock()

	// Level 3: Fetch from registry service with retries
	info, err := r.fetchFromRegistryWithRetry(ctx, tenantID)
	if err != nil {
		r.mu.Lock()
		r.registryErrors++
		r.mu.Unlock()
		return nil, err
	}

	// Validate tenant is active
	if !info.IsActive {
		return nil, ErrTenantInactive
	}

	// Check if audit logs are enabled for this tenant
	if !info.Features.AuditLogsEnabled {
		return nil, ErrAuditDisabled
	}

	// Decrypt credentials before caching
	if err := r.decryptCredentials(info); err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Populate caches
	r.setLocalCache(tenantID, info)
	if r.redisClient != nil {
		r.setRedisCache(ctx, tenantID, info)
	}

	return info, nil
}

// fetchFromRegistryWithRetry fetches tenant config with exponential backoff retry
func (r *Registry) fetchFromRegistryWithRetry(ctx context.Context, tenantID string) (*TenantInfo, error) {
	var lastErr error
	delay := r.retryBaseDelay

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		info, err := r.fetchFromRegistry(ctx, tenantID)
		if err == nil {
			return info, nil
		}

		lastErr = err

		// Don't retry on certain errors
		if errors.Is(err, ErrTenantNotFound) {
			return nil, err
		}

		// Log retry attempt
		if r.logger != nil && attempt < r.maxRetries {
			r.logger.WithFields(logrus.Fields{
				"tenant_id": tenantID,
				"attempt":   attempt + 1,
				"max":       r.maxRetries + 1,
				"delay":     delay.String(),
			}).WithError(err).Warn("Retrying registry fetch")
		}

		r.mu.Lock()
		r.retryCount++
		r.mu.Unlock()

		// Wait before next attempt (with context awareness)
		if attempt < r.maxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// Exponential backoff with cap
			delay *= 2
			if delay > r.retryMaxDelay {
				delay = r.retryMaxDelay
			}
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", r.maxRetries+1, lastErr)
}

// getFromLocalCache retrieves from local memory cache
func (r *Registry) getFromLocalCache(tenantID string) *TenantInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	expiry, exists := r.cacheExpiry[tenantID]
	if !exists || time.Now().After(expiry) {
		return nil
	}

	return r.cache[tenantID]
}

// setLocalCache stores in local memory cache
func (r *Registry) setLocalCache(tenantID string, info *TenantInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache[tenantID] = info
	r.cacheExpiry[tenantID] = time.Now().Add(r.cacheTTL)
}

// getFromRedisCache retrieves from Redis
func (r *Registry) getFromRedisCache(ctx context.Context, tenantID string) *TenantInfo {
	key := fmt.Sprintf("audit:tenant:%s", tenantID)
	data, err := r.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		return nil
	}

	var info TenantInfo
	if err := json.Unmarshal(data, &info); err != nil {
		r.logger.WithError(err).Warn("Failed to unmarshal tenant info from Redis")
		return nil
	}

	return &info
}

// setRedisCache stores in Redis
func (r *Registry) setRedisCache(ctx context.Context, tenantID string, info *TenantInfo) {
	key := fmt.Sprintf("audit:tenant:%s", tenantID)
	data, err := json.Marshal(info)
	if err != nil {
		r.logger.WithError(err).Warn("Failed to marshal tenant info for Redis")
		return
	}

	// Store with TTL
	if err := r.redisClient.Set(ctx, key, data, r.cacheTTL).Err(); err != nil {
		r.logger.WithError(err).Warn("Failed to cache tenant info in Redis")
	}
}

// fetchFromRegistry fetches tenant config from the registry service
func (r *Registry) fetchFromRegistry(ctx context.Context, tenantID string) (*TenantInfo, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/audit-config", r.registryURL, tenantID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service", "audit-service")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRegistryUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrTenantNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var info TenantInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &info, nil
}

// decryptCredentials decrypts the database password
func (r *Registry) decryptCredentials(info *TenantInfo) error {
	if r.encryptionKey == nil || info.DatabaseConfig.Password == "" {
		return nil
	}

	ciphertext, err := base64.StdEncoding.DecodeString(info.DatabaseConfig.Password)
	if err != nil {
		return ErrInvalidCredentials
	}

	block, err := aes.NewCipher(r.encryptionKey)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return ErrInvalidCredentials
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return ErrInvalidCredentials
	}

	info.DatabaseConfig.Password = string(plaintext)
	return nil
}

// InvalidateCache removes a tenant from all caches (useful when config changes)
func (r *Registry) InvalidateCache(ctx context.Context, tenantID string) {
	r.mu.Lock()
	delete(r.cache, tenantID)
	delete(r.cacheExpiry, tenantID)
	r.mu.Unlock()

	if r.redisClient != nil {
		key := fmt.Sprintf("audit:tenant:%s", tenantID)
		r.redisClient.Del(ctx, key)
	}
}

// GetStats returns cache statistics
func (r *Registry) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalRequests := r.cacheHits + r.cacheMisses
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(r.cacheHits) / float64(totalRequests)
	}

	return map[string]interface{}{
		"cache_size":      len(r.cache),
		"cache_hits":      r.cacheHits,
		"cache_misses":    r.cacheMisses,
		"registry_errors": r.registryErrors,
		"retry_count":     r.retryCount,
		"hit_rate":        hitRate,
		"redis_enabled":   r.redisClient != nil,
	}
}

// GetAllTenants returns all known tenant IDs from the cache and registry
func (r *Registry) GetAllTenants(ctx context.Context) ([]string, error) {
	// First get all cached tenant IDs
	r.mu.RLock()
	tenantIDs := make([]string, 0, len(r.cache))
	for tenantID := range r.cache {
		tenantIDs = append(tenantIDs, tenantID)
	}
	r.mu.RUnlock()

	// If we have some cached tenants, return them
	// The scheduler will process these and discover more via DB connections
	if len(tenantIDs) > 0 {
		return tenantIDs, nil
	}

	// If no cached tenants, try to fetch list from registry
	url := fmt.Sprintf("%s/api/v1/tenants/audit-enabled", r.registryURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		// If registry is unavailable, return cached tenants (even if empty)
		r.logger.WithError(err).Warn("Failed to fetch tenant list from registry, using cached tenants")
		return tenantIDs, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.logger.WithField("status", resp.StatusCode).Warn("Failed to fetch tenant list from registry")
		return tenantIDs, nil
	}

	var response struct {
		TenantIDs []string `json:"tenant_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		r.logger.WithError(err).Warn("Failed to decode tenant list response")
		return tenantIDs, nil
	}

	return response.TenantIDs, nil
}

// WarmCache pre-loads tenant configurations for known tenants
func (r *Registry) WarmCache(ctx context.Context, tenantIDs []string) {
	for _, tenantID := range tenantIDs {
		if _, err := r.GetTenant(ctx, tenantID); err != nil {
			r.logger.WithField("tenant_id", tenantID).WithError(err).Warn("Failed to warm cache for tenant")
		}
	}
}

// StartBackgroundRefresh periodically refreshes cached tenants
func (r *Registry) StartBackgroundRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				r.refreshAllCached(ctx)
			}
		}
	}()
}

func (r *Registry) refreshAllCached(ctx context.Context) {
	r.mu.RLock()
	tenantIDs := make([]string, 0, len(r.cache))
	for tenantID := range r.cache {
		tenantIDs = append(tenantIDs, tenantID)
	}
	r.mu.RUnlock()

	for _, tenantID := range tenantIDs {
		info, err := r.fetchFromRegistry(ctx, tenantID)
		if err != nil {
			r.logger.WithField("tenant_id", tenantID).WithError(err).Debug("Failed to refresh tenant config")
			continue
		}
		if err := r.decryptCredentials(info); err != nil {
			continue
		}
		r.setLocalCache(tenantID, info)
		if r.redisClient != nil {
			r.setRedisCache(ctx, tenantID, info)
		}
	}
}
