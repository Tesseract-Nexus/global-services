package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// EmailRateLimitConfig holds configuration for email rate limiting
type EmailRateLimitConfig struct {
	// Per-tenant limits
	TenantHourlyLimit int // Max emails per hour per tenant (default: 1000)
	TenantDailyLimit  int // Max emails per day per tenant (default: 10000)

	// Per-recipient limits
	RecipientHourlyLimit int // Max emails per hour to same recipient (default: 10)

	// Per-action limits (security-sensitive emails)
	PasswordResetLimit  int           // Max password reset emails per hour per user (default: 3)
	VerificationLimit   int           // Max verification emails per hour per user (default: 5)
	PasswordResetWindow time.Duration // Window for password reset limits (default: 1 hour)
	VerificationWindow  time.Duration // Window for verification limits (default: 1 hour)

	// Redis settings
	RedisKeyPrefix string // Prefix for Redis keys (default: "email:ratelimit:")
}

// DefaultEmailRateLimitConfig returns sensible defaults
func DefaultEmailRateLimitConfig() EmailRateLimitConfig {
	return EmailRateLimitConfig{
		TenantHourlyLimit:    1000,
		TenantDailyLimit:     10000,
		RecipientHourlyLimit: 10,
		PasswordResetLimit:   3,
		VerificationLimit:    5,
		PasswordResetWindow:  time.Hour,
		VerificationWindow:   time.Hour,
		RedisKeyPrefix:       "email:ratelimit:",
	}
}

// EmailRateLimiter handles rate limiting for email sending
type EmailRateLimiter struct {
	config      EmailRateLimitConfig
	redisClient *redis.Client
	logger      *logrus.Entry

	// In-memory fallback when Redis is unavailable
	localRateLimits map[string]*rateLimitState
	localMu         sync.RWMutex
}

// rateLimitState tracks rate limit counters
type rateLimitState struct {
	Count     int       `json:"count"`
	FirstSent time.Time `json:"first_sent"`
	ExpiresAt time.Time `json:"expires_at"`
}

// EmailAction represents the type of email being sent
type EmailAction string

const (
	ActionPasswordReset   EmailAction = "password_reset"
	ActionVerification    EmailAction = "verification"
	ActionGeneral         EmailAction = "general"
	ActionOTP             EmailAction = "otp"
	ActionSecurityAlert   EmailAction = "security_alert"
	ActionAccountLockout  EmailAction = "account_lockout"
)

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	Allowed       bool          `json:"allowed"`
	Remaining     int           `json:"remaining"`
	ResetAfter    time.Duration `json:"reset_after"`
	LimitType     string        `json:"limit_type"`      // Which limit was exceeded
	RetryAfterSec int           `json:"retry_after_sec"` // Seconds until retry is allowed
}

// NewEmailRateLimiter creates a new email rate limiter
func NewEmailRateLimiter(redisClient *redis.Client, logger *logrus.Logger) *EmailRateLimiter {
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	return &EmailRateLimiter{
		config:          DefaultEmailRateLimitConfig(),
		redisClient:     redisClient,
		logger:          logger.WithField("component", "email_rate_limiter"),
		localRateLimits: make(map[string]*rateLimitState),
	}
}

// NewEmailRateLimiterWithConfig creates a new email rate limiter with custom config
func NewEmailRateLimiterWithConfig(redisClient *redis.Client, logger *logrus.Logger, config EmailRateLimitConfig) *EmailRateLimiter {
	limiter := NewEmailRateLimiter(redisClient, logger)
	limiter.config = config
	return limiter
}

// CheckLimit checks if sending an email is allowed
func (r *EmailRateLimiter) CheckLimit(ctx context.Context, tenantID, recipient string, action EmailAction) (*RateLimitResult, error) {
	// Check per-action limits first (most restrictive for security emails)
	if action == ActionPasswordReset || action == ActionVerification {
		result, err := r.checkActionLimit(ctx, tenantID, recipient, action)
		if err != nil {
			return nil, err
		}
		if !result.Allowed {
			return result, nil
		}
	}

	// Check per-recipient limits
	result, err := r.checkRecipientLimit(ctx, tenantID, recipient)
	if err != nil {
		return nil, err
	}
	if !result.Allowed {
		return result, nil
	}

	// Check per-tenant hourly limit
	result, err = r.checkTenantHourlyLimit(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !result.Allowed {
		return result, nil
	}

	// Check per-tenant daily limit
	result, err = r.checkTenantDailyLimit(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// RecordSend records a successful email send for rate limiting
func (r *EmailRateLimiter) RecordSend(ctx context.Context, tenantID, recipient string, action EmailAction) error {
	// Record for per-action limits
	if action == ActionPasswordReset || action == ActionVerification {
		if err := r.incrementCounter(ctx, r.actionKey(tenantID, recipient, action), r.getActionWindow(action)); err != nil {
			return err
		}
	}

	// Record for per-recipient limits
	if err := r.incrementCounter(ctx, r.recipientKey(tenantID, recipient), time.Hour); err != nil {
		return err
	}

	// Record for per-tenant hourly limit
	if err := r.incrementCounter(ctx, r.tenantHourlyKey(tenantID), time.Hour); err != nil {
		return err
	}

	// Record for per-tenant daily limit
	if err := r.incrementCounter(ctx, r.tenantDailyKey(tenantID), 24*time.Hour); err != nil {
		return err
	}

	r.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"recipient": maskEmail(recipient),
		"action":    action,
	}).Debug("Email send recorded for rate limiting")

	return nil
}

// GetRemainingQuota returns the remaining quota for various limits
func (r *EmailRateLimiter) GetRemainingQuota(ctx context.Context, tenantID, recipient string, action EmailAction) map[string]int {
	quota := make(map[string]int)

	// Action-specific remaining
	if action == ActionPasswordReset || action == ActionVerification {
		count, _ := r.getCounter(ctx, r.actionKey(tenantID, recipient, action))
		limit := r.getActionLimit(action)
		quota["action_remaining"] = max(0, limit-count)
	}

	// Per-recipient remaining
	count, _ := r.getCounter(ctx, r.recipientKey(tenantID, recipient))
	quota["recipient_remaining"] = max(0, r.config.RecipientHourlyLimit-count)

	// Per-tenant hourly remaining
	count, _ = r.getCounter(ctx, r.tenantHourlyKey(tenantID))
	quota["tenant_hourly_remaining"] = max(0, r.config.TenantHourlyLimit-count)

	// Per-tenant daily remaining
	count, _ = r.getCounter(ctx, r.tenantDailyKey(tenantID))
	quota["tenant_daily_remaining"] = max(0, r.config.TenantDailyLimit-count)

	return quota
}

// Private helper methods

func (r *EmailRateLimiter) checkActionLimit(ctx context.Context, tenantID, recipient string, action EmailAction) (*RateLimitResult, error) {
	key := r.actionKey(tenantID, recipient, action)
	count, err := r.getCounter(ctx, key)
	if err != nil {
		return nil, err
	}

	limit := r.getActionLimit(action)
	remaining := limit - count

	if remaining <= 0 {
		ttl := r.getTTL(ctx, key)
		return &RateLimitResult{
			Allowed:       false,
			Remaining:     0,
			ResetAfter:    ttl,
			LimitType:     string(action) + "_limit",
			RetryAfterSec: int(ttl.Seconds()),
		}, nil
	}

	return &RateLimitResult{
		Allowed:   true,
		Remaining: remaining,
		LimitType: string(action) + "_limit",
	}, nil
}

func (r *EmailRateLimiter) checkRecipientLimit(ctx context.Context, tenantID, recipient string) (*RateLimitResult, error) {
	key := r.recipientKey(tenantID, recipient)
	count, err := r.getCounter(ctx, key)
	if err != nil {
		return nil, err
	}

	remaining := r.config.RecipientHourlyLimit - count

	if remaining <= 0 {
		ttl := r.getTTL(ctx, key)
		return &RateLimitResult{
			Allowed:       false,
			Remaining:     0,
			ResetAfter:    ttl,
			LimitType:     "recipient_hourly_limit",
			RetryAfterSec: int(ttl.Seconds()),
		}, nil
	}

	return &RateLimitResult{
		Allowed:   true,
		Remaining: remaining,
		LimitType: "recipient_hourly_limit",
	}, nil
}

func (r *EmailRateLimiter) checkTenantHourlyLimit(ctx context.Context, tenantID string) (*RateLimitResult, error) {
	key := r.tenantHourlyKey(tenantID)
	count, err := r.getCounter(ctx, key)
	if err != nil {
		return nil, err
	}

	remaining := r.config.TenantHourlyLimit - count

	if remaining <= 0 {
		ttl := r.getTTL(ctx, key)
		return &RateLimitResult{
			Allowed:       false,
			Remaining:     0,
			ResetAfter:    ttl,
			LimitType:     "tenant_hourly_limit",
			RetryAfterSec: int(ttl.Seconds()),
		}, nil
	}

	return &RateLimitResult{
		Allowed:   true,
		Remaining: remaining,
		LimitType: "tenant_hourly_limit",
	}, nil
}

func (r *EmailRateLimiter) checkTenantDailyLimit(ctx context.Context, tenantID string) (*RateLimitResult, error) {
	key := r.tenantDailyKey(tenantID)
	count, err := r.getCounter(ctx, key)
	if err != nil {
		return nil, err
	}

	remaining := r.config.TenantDailyLimit - count

	if remaining <= 0 {
		ttl := r.getTTL(ctx, key)
		return &RateLimitResult{
			Allowed:       false,
			Remaining:     0,
			ResetAfter:    ttl,
			LimitType:     "tenant_daily_limit",
			RetryAfterSec: int(ttl.Seconds()),
		}, nil
	}

	return &RateLimitResult{
		Allowed:   true,
		Remaining: remaining,
		LimitType: "tenant_daily_limit",
	}, nil
}

func (r *EmailRateLimiter) incrementCounter(ctx context.Context, key string, window time.Duration) error {
	if r.redisClient != nil {
		// Use Redis INCR with EXPIRE
		pipe := r.redisClient.Pipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, window)
		_, err := pipe.Exec(ctx)
		if err != nil {
			r.logger.WithError(err).Warn("Redis increment failed, using local fallback")
		} else {
			r.logger.WithFields(logrus.Fields{
				"key":   key,
				"count": incr.Val(),
			}).Debug("Counter incremented in Redis")
			return nil
		}
	}

	// Fallback to local storage
	r.localMu.Lock()
	defer r.localMu.Unlock()

	state, exists := r.localRateLimits[key]
	if !exists || time.Now().After(state.ExpiresAt) {
		state = &rateLimitState{
			Count:     1,
			FirstSent: time.Now(),
			ExpiresAt: time.Now().Add(window),
		}
	} else {
		state.Count++
	}
	r.localRateLimits[key] = state

	return nil
}

func (r *EmailRateLimiter) getCounter(ctx context.Context, key string) (int, error) {
	if r.redisClient != nil {
		val, err := r.redisClient.Get(ctx, key).Int()
		if err == redis.Nil {
			return 0, nil
		}
		if err != nil {
			r.logger.WithError(err).Warn("Redis get failed, using local fallback")
		} else {
			return val, nil
		}
	}

	// Fallback to local storage
	r.localMu.RLock()
	defer r.localMu.RUnlock()

	state, exists := r.localRateLimits[key]
	if !exists || time.Now().After(state.ExpiresAt) {
		return 0, nil
	}
	return state.Count, nil
}

func (r *EmailRateLimiter) getTTL(ctx context.Context, key string) time.Duration {
	if r.redisClient != nil {
		ttl, err := r.redisClient.TTL(ctx, key).Result()
		if err == nil && ttl > 0 {
			return ttl
		}
	}

	// Fallback to local storage
	r.localMu.RLock()
	defer r.localMu.RUnlock()

	state, exists := r.localRateLimits[key]
	if exists && state.ExpiresAt.After(time.Now()) {
		return time.Until(state.ExpiresAt)
	}

	return 0
}

// Key generation methods
func (r *EmailRateLimiter) actionKey(tenantID, recipient string, action EmailAction) string {
	return fmt.Sprintf("%s%s:%s:%s", r.config.RedisKeyPrefix, tenantID, recipient, action)
}

func (r *EmailRateLimiter) recipientKey(tenantID, recipient string) string {
	return fmt.Sprintf("%s%s:%s:hourly", r.config.RedisKeyPrefix, tenantID, recipient)
}

func (r *EmailRateLimiter) tenantHourlyKey(tenantID string) string {
	return fmt.Sprintf("%s%s:tenant:hourly", r.config.RedisKeyPrefix, tenantID)
}

func (r *EmailRateLimiter) tenantDailyKey(tenantID string) string {
	return fmt.Sprintf("%s%s:tenant:daily", r.config.RedisKeyPrefix, tenantID)
}

func (r *EmailRateLimiter) getActionLimit(action EmailAction) int {
	switch action {
	case ActionPasswordReset:
		return r.config.PasswordResetLimit
	case ActionVerification:
		return r.config.VerificationLimit
	default:
		return 100 // High limit for general emails
	}
}

func (r *EmailRateLimiter) getActionWindow(action EmailAction) time.Duration {
	switch action {
	case ActionPasswordReset:
		return r.config.PasswordResetWindow
	case ActionVerification:
		return r.config.VerificationWindow
	default:
		return time.Hour
	}
}

// maskEmail masks email for logging (privacy)
func maskEmail(email string) string {
	if len(email) < 5 {
		return "***"
	}
	// Show first 2 chars and domain
	atIdx := -1
	for i, c := range email {
		if c == '@' {
			atIdx = i
			break
		}
	}
	if atIdx < 2 {
		return "***"
	}
	return email[:2] + "***" + email[atIdx:]
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetConfig returns the current rate limit configuration
func (r *EmailRateLimiter) GetConfig() EmailRateLimitConfig {
	return r.config
}

// SetConfig updates the rate limit configuration
func (r *EmailRateLimiter) SetConfig(config EmailRateLimitConfig) {
	r.config = config
}

// ToJSON returns a JSON representation of the rate limit result
func (r *RateLimitResult) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}
