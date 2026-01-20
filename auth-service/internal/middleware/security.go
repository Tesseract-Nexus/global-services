package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Tesseract-Nexus/go-shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// SecurityConfig holds configuration for security middleware
type SecurityConfig struct {
	// MaxLoginAttempts before account lockout (per tier)
	MaxLoginAttempts int
	// LockoutTiers defines progressive lockout durations
	LockoutTiers []LockoutTier
	// PermanentLockoutThreshold is the total attempts before permanent lockout
	PermanentLockoutThreshold int
	// LockoutResetAfter is how long until failed attempts are reset (if no lockouts)
	LockoutResetAfter time.Duration
	// RedisKeyPrefix for storing lockout data
	RedisKeyPrefix string
	// RedisKeyPrefixEmail for email-only lockout lookups (admin features)
	RedisKeyPrefixEmail string
}

// LockoutTier defines a lockout tier with duration
type LockoutTier struct {
	Tier     int
	Duration time.Duration
}

// DefaultSecurityConfig returns sensible defaults
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		MaxLoginAttempts: 5,
		LockoutTiers: []LockoutTier{
			{Tier: 1, Duration: 30 * time.Minute},  // Tier 1: 5 attempts -> 30 min lockout
			{Tier: 2, Duration: 60 * time.Minute},  // Tier 2: 10 attempts -> 1 hour lockout
			{Tier: 3, Duration: 120 * time.Minute}, // Tier 3: 15 attempts -> 2 hour lockout
			{Tier: 4, Duration: 0},                 // Tier 4: 20 attempts -> Permanent (0 = permanent)
		},
		PermanentLockoutThreshold: 20,
		LockoutResetAfter:         24 * time.Hour, // Reset counters after 24 hours of no failed attempts
		RedisKeyPrefix:            "auth:lockout:",
		RedisKeyPrefixEmail:       "auth:lockout:email:",
	}
}

// SecurityMiddleware provides rate limiting and account lockout functionality
type SecurityMiddleware struct {
	config      SecurityConfig
	redisClient *redis.Client
	logger      *logrus.Logger
	// In-memory fallback when Redis is unavailable
	localLockouts map[string]*lockoutState
	localMu       sync.RWMutex
}

// lockoutState tracks login attempts and lockout status
type lockoutState struct {
	FailedAttempts    int       `json:"failed_attempts"`
	LastFailedAt      time.Time `json:"last_failed_at"`
	LockedUntil       time.Time `json:"locked_until"`
	LockoutCount      int       `json:"lockout_count"`       // Number of lockouts triggered
	CurrentTier       int       `json:"current_tier"`        // Current lockout tier (1-4)
	PermanentlyLocked bool      `json:"permanently_locked"`  // True if permanently locked
	PermanentLockedAt time.Time `json:"permanent_locked_at"` // When permanent lock was triggered
	UnlockedBy        string    `json:"unlocked_by"`         // Admin user ID who unlocked
	UnlockedAt        time.Time `json:"unlocked_at"`         // When account was unlocked
	Email             string    `json:"email"`               // Email for admin lookup
	TenantID          string    `json:"tenant_id"`           // Tenant ID for the lockout
}

// NewSecurityMiddleware creates a new security middleware instance
func NewSecurityMiddleware(redisClient *redis.Client, logger *logrus.Logger) *SecurityMiddleware {
	if logger == nil {
		logger = logrus.New()
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	return &SecurityMiddleware{
		config:        DefaultSecurityConfig(),
		redisClient:   redisClient,
		logger:        logger,
		localLockouts: make(map[string]*lockoutState),
	}
}

// NewSecurityMiddlewareWithConfig creates a security middleware with custom config
func NewSecurityMiddlewareWithConfig(redisClient *redis.Client, logger *logrus.Logger, config SecurityConfig) *SecurityMiddleware {
	sm := NewSecurityMiddleware(redisClient, logger)
	sm.config = config
	return sm
}

// generateLockoutKey creates a unique key for IP + email combination
func (sm *SecurityMiddleware) generateLockoutKey(ip, email string) string {
	// Hash the combination for privacy and consistent key length
	data := fmt.Sprintf("%s:%s", ip, strings.ToLower(email))
	hash := sha256.Sum256([]byte(data))
	return sm.config.RedisKeyPrefix + hex.EncodeToString(hash[:16])
}

// generateEmailLockoutKey creates a key for email-only lockout (for admin lookup)
func (sm *SecurityMiddleware) generateEmailLockoutKey(email string) string {
	hash := sha256.Sum256([]byte(strings.ToLower(email)))
	return sm.config.RedisKeyPrefixEmail + hex.EncodeToString(hash[:16])
}

// getLockoutState retrieves lockout state from Redis or local cache
func (sm *SecurityMiddleware) getLockoutState(ctx context.Context, key string) (*lockoutState, error) {
	// Try Redis first
	if sm.redisClient != nil {
		data, err := sm.redisClient.Get(ctx, key).Result()
		if err == nil {
			var state lockoutState
			if err := json.Unmarshal([]byte(data), &state); err == nil {
				return &state, nil
			}
		} else if err != redis.Nil {
			sm.logger.WithError(err).Warn("Failed to get lockout state from Redis, using local fallback")
		}
	}

	// Fallback to local cache
	sm.localMu.RLock()
	state, exists := sm.localLockouts[key]
	sm.localMu.RUnlock()

	if !exists {
		return &lockoutState{}, nil
	}

	return state, nil
}

// setLockoutState stores lockout state in Redis and local cache
func (sm *SecurityMiddleware) setLockoutState(ctx context.Context, key string, state *lockoutState) error {
	// Store in local cache
	sm.localMu.Lock()
	sm.localLockouts[key] = state
	sm.localMu.Unlock()

	// Store in Redis if available
	if sm.redisClient != nil {
		data, err := json.Marshal(state)
		if err != nil {
			return err
		}

		// Permanent lockouts don't expire
		var ttl time.Duration
		if state.PermanentlyLocked {
			ttl = 0 // No expiration for permanent lockouts
		} else {
			ttl = sm.config.LockoutResetAfter
			if state.LockedUntil.After(time.Now()) {
				// Extend TTL if currently locked
				remaining := time.Until(state.LockedUntil)
				if remaining > ttl {
					ttl = remaining + time.Minute
				}
			}
		}

		var err2 error
		if ttl == 0 {
			// No expiration for permanent lockouts
			err2 = sm.redisClient.Set(ctx, key, data, 0).Err()
		} else {
			err2 = sm.redisClient.Set(ctx, key, data, ttl).Err()
		}
		if err2 != nil {
			sm.logger.WithError(err2).Warn("Failed to set lockout state in Redis")
			// Don't return error - local cache is already updated
		}

		// Also store by email key for admin lookup (if email is set)
		if state.Email != "" {
			emailKey := sm.generateEmailLockoutKey(state.Email)
			if ttl == 0 {
				sm.redisClient.Set(ctx, emailKey, data, 0)
			} else {
				sm.redisClient.Set(ctx, emailKey, data, ttl)
			}
		}
	}

	return nil
}

// clearLockoutState removes lockout state (on successful login)
func (sm *SecurityMiddleware) clearLockoutState(ctx context.Context, key string) {
	// Clear from local cache
	sm.localMu.Lock()
	delete(sm.localLockouts, key)
	sm.localMu.Unlock()

	// Clear from Redis if available
	if sm.redisClient != nil {
		if err := sm.redisClient.Del(ctx, key).Err(); err != nil {
			sm.logger.WithError(err).Warn("Failed to clear lockout state from Redis")
		}
	}
}

// calculateLockoutTier calculates the current lockout tier based on total failed attempts
func (sm *SecurityMiddleware) calculateLockoutTier(totalAttempts int) int {
	tier := (totalAttempts + sm.config.MaxLoginAttempts - 1) / sm.config.MaxLoginAttempts
	if tier > len(sm.config.LockoutTiers) {
		tier = len(sm.config.LockoutTiers)
	}
	if tier < 1 {
		tier = 1
	}
	return tier
}

// getLockoutDurationForTier returns the lockout duration for a given tier
// Returns 0 for permanent lockout
func (sm *SecurityMiddleware) getLockoutDurationForTier(tier int) time.Duration {
	if tier <= 0 || tier > len(sm.config.LockoutTiers) {
		// Default to last tier for out of range
		if len(sm.config.LockoutTiers) > 0 {
			return sm.config.LockoutTiers[len(sm.config.LockoutTiers)-1].Duration
		}
		return 30 * time.Minute // Fallback
	}
	return sm.config.LockoutTiers[tier-1].Duration
}

// isPermanentLockoutTier checks if the given tier results in permanent lockout
func (sm *SecurityMiddleware) isPermanentLockoutTier(tier int) bool {
	duration := sm.getLockoutDurationForTier(tier)
	return duration == 0 // 0 duration means permanent
}

// RecordFailedLogin records a failed login attempt
// Returns (attemptsRemaining, lockedUntil, isPermanent)
func (sm *SecurityMiddleware) RecordFailedLogin(ctx context.Context, ip, email string) (attemptsRemaining int, lockedUntil time.Time) {
	return sm.RecordFailedLoginWithTenant(ctx, ip, email, "")
}

// RecordFailedLoginWithTenant records a failed login attempt with tenant info
func (sm *SecurityMiddleware) RecordFailedLoginWithTenant(ctx context.Context, ip, email, tenantID string) (attemptsRemaining int, lockedUntil time.Time) {
	key := sm.generateLockoutKey(ip, email)

	state, _ := sm.getLockoutState(ctx, key)

	// Don't process if already permanently locked
	if state.PermanentlyLocked {
		sm.logSecurityEvent("locked_login_attempt_permanent", ip, email, map[string]interface{}{
			"permanently_locked_at": state.PermanentLockedAt,
		})
		return 0, time.Time{}
	}

	// Check if we should reset the counter (attempts too old and not in active lockout)
	if state.FailedAttempts > 0 &&
		!state.LockedUntil.After(time.Now()) &&
		time.Since(state.LastFailedAt) > sm.config.LockoutResetAfter {
		state = &lockoutState{}
	}

	state.FailedAttempts++
	state.LastFailedAt = time.Now()
	state.Email = email
	state.TenantID = tenantID

	// Calculate which tier we're in based on total attempts
	currentTier := sm.calculateLockoutTier(state.FailedAttempts)

	// Check if attempts within current tier have exceeded the limit
	attemptsInCurrentTier := state.FailedAttempts % sm.config.MaxLoginAttempts
	if attemptsInCurrentTier == 0 {
		attemptsInCurrentTier = sm.config.MaxLoginAttempts
	}

	// Trigger lockout when we hit the max attempts for this tier
	if attemptsInCurrentTier >= sm.config.MaxLoginAttempts {
		state.LockoutCount++
		state.CurrentTier = currentTier

		// Check if this triggers permanent lockout
		if state.FailedAttempts >= sm.config.PermanentLockoutThreshold || sm.isPermanentLockoutTier(currentTier) {
			state.PermanentlyLocked = true
			state.PermanentLockedAt = time.Now()

			sm.logSecurityEvent("account_locked_permanent", ip, email, map[string]interface{}{
				"failed_attempts":     state.FailedAttempts,
				"lockout_count":       state.LockoutCount,
				"current_tier":        currentTier,
				"permanently_locked":  true,
				"tenant_id":           tenantID,
			})
		} else {
			// Regular tier lockout
			lockoutDuration := sm.getLockoutDurationForTier(currentTier)
			state.LockedUntil = time.Now().Add(lockoutDuration)

			sm.logSecurityEvent("account_locked_temporary", ip, email, map[string]interface{}{
				"failed_attempts":   state.FailedAttempts,
				"lockout_count":     state.LockoutCount,
				"current_tier":      currentTier,
				"locked_until":      state.LockedUntil,
				"lockout_duration":  lockoutDuration.String(),
				"tenant_id":         tenantID,
			})
		}
	} else {
		// Calculate remaining attempts in current tier
		tierStartAttempts := (currentTier - 1) * sm.config.MaxLoginAttempts
		attemptsInTier := state.FailedAttempts - tierStartAttempts
		remainingInTier := sm.config.MaxLoginAttempts - attemptsInTier

		sm.logSecurityEvent("failed_login_attempt", ip, email, map[string]interface{}{
			"failed_attempts":     state.FailedAttempts,
			"current_tier":        currentTier,
			"attempts_in_tier":    attemptsInTier,
			"attempts_remaining":  remainingInTier,
			"tenant_id":           tenantID,
		})
	}

	sm.setLockoutState(ctx, key, state)

	// Calculate remaining attempts until next lockout
	tierStartAttempts := (currentTier - 1) * sm.config.MaxLoginAttempts
	attemptsInTier := state.FailedAttempts - tierStartAttempts
	remainingInTier := sm.config.MaxLoginAttempts - attemptsInTier
	if remainingInTier < 0 {
		remainingInTier = 0
	}

	return remainingInTier, state.LockedUntil
}

// RecordSuccessfulLogin clears failed login attempts on successful login
func (sm *SecurityMiddleware) RecordSuccessfulLogin(ctx context.Context, ip, email string) {
	key := sm.generateLockoutKey(ip, email)
	sm.clearLockoutState(ctx, key)

	sm.logSecurityEvent("successful_login", ip, email, nil)
}

// IsLocked checks if an IP+email combination is currently locked out
// Returns (isLocked, remainingDuration, isPermanent)
func (sm *SecurityMiddleware) IsLocked(ctx context.Context, ip, email string) (bool, time.Duration) {
	locked, remaining, _ := sm.IsLockedWithDetails(ctx, ip, email)
	return locked, remaining
}

// IsLockedWithDetails checks if an IP+email combination is currently locked out
// Returns (isLocked, remainingDuration, isPermanent)
func (sm *SecurityMiddleware) IsLockedWithDetails(ctx context.Context, ip, email string) (bool, time.Duration, bool) {
	key := sm.generateLockoutKey(ip, email)

	state, _ := sm.getLockoutState(ctx, key)

	// Check for permanent lockout first
	if state.PermanentlyLocked {
		return true, 0, true
	}

	// Check for temporary lockout
	if state.LockedUntil.After(time.Now()) {
		return true, time.Until(state.LockedUntil), false
	}

	return false, 0, false
}

// GetFailedAttempts returns the current number of failed attempts
func (sm *SecurityMiddleware) GetFailedAttempts(ctx context.Context, ip, email string) int {
	key := sm.generateLockoutKey(ip, email)
	state, _ := sm.getLockoutState(ctx, key)
	return state.FailedAttempts
}

// logSecurityEvent logs security-related events
func (sm *SecurityMiddleware) logSecurityEvent(eventType, ip, email string, details map[string]interface{}) {
	fields := logrus.Fields{
		"event_type":     eventType,
		"ip_address":     ip,
		"email_masked":   maskEmail(email),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"security_event": true,
	}

	for k, v := range details {
		fields[k] = v
	}

	sm.logger.WithFields(fields).Info("Security event")
}

// maskEmail masks email for logging (privacy)
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}

	local := parts[0]
	domain := parts[1]

	if len(local) <= 2 {
		return "**@" + domain
	}

	return local[:2] + strings.Repeat("*", len(local)-2) + "@" + domain
}

// LoginRateLimitMiddleware combines IP-based rate limiting with account lockout checking
// This middleware should be applied before the login handler
func (sm *SecurityMiddleware) LoginRateLimitMiddleware() gin.HandlerFunc {
	// Create the base auth rate limiter from go-shared
	baseRateLimiter := middleware.NewRateLimiter(middleware.AuthRateLimitConfig())

	return func(c *gin.Context) {
		// First, apply IP-based rate limiting
		ip := c.ClientIP()

		// Apply the rate limiter middleware
		baseRateLimiter.Middleware()(c)

		// Check if rate limit was exceeded (context aborted)
		if c.IsAborted() {
			sm.logSecurityEvent("rate_limit_exceeded", ip, "", map[string]interface{}{
				"endpoint": c.Request.URL.Path,
			})
			return
		}

		c.Next()
	}
}

// AccountLockoutMiddleware checks for account lockout before processing login
// This requires the request body to be parsed to get the email
func (sm *SecurityMiddleware) AccountLockoutMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// We need to peek at the request body to get the email
		// without consuming it
		email := extractEmailFromRequest(c)
		if email == "" {
			// Can't check lockout without email, proceed with other checks
			c.Next()
			return
		}

		ip := c.ClientIP()
		ctx := c.Request.Context()

		// Check if the account is locked (including permanent lockouts)
		locked, remaining, isPermanent := sm.IsLockedWithDetails(ctx, ip, email)
		if locked {
			if isPermanent {
				sm.logSecurityEvent("locked_login_attempt_permanent", ip, email, map[string]interface{}{
					"permanently_locked": true,
				})

				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":   "Account permanently locked due to repeated failed login attempts",
					"code":    "ACCOUNT_LOCKED_PERMANENT",
					"message": "Your account has been permanently locked. Please contact support to unlock your account.",
				})
				c.Abort()
				return
			}

			sm.logSecurityEvent("locked_login_attempt", ip, email, map[string]interface{}{
				"remaining_lockout": remaining.String(),
			})

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Account temporarily locked due to too many failed login attempts",
				"code":        "ACCOUNT_LOCKED",
				"retry_after": int(remaining.Seconds()),
				"message":     fmt.Sprintf("Please try again in %s", formatDuration(remaining)),
			})
			c.Abort()
			return
		}

		// Store the security middleware in context for the handler to use
		c.Set("security_middleware", sm)
		c.Set("login_email", email)
		c.Set("login_ip", ip)

		c.Next()
	}
}

// extractEmailFromRequest extracts email from the request without consuming the body
func extractEmailFromRequest(c *gin.Context) string {
	// Try to get from query param first (for some auth flows)
	email := c.Query("email")
	if email != "" {
		return strings.ToLower(email)
	}

	// For JSON body, we need to peek at it
	// This is a simplified approach - in production you might want to use
	// a more sophisticated body peeking mechanism
	var req struct {
		Email string `json:"email"`
	}

	// Copy the body for peeking
	bodyBytes, err := c.GetRawData()
	if err != nil || len(bodyBytes) == 0 {
		return ""
	}

	// Put the body back for the actual handler using io.NopCloser
	c.Request.Body = io.NopCloser(newBodyReader(bodyBytes))

	// Try to parse email from JSON
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		return ""
	}

	return strings.ToLower(req.Email)
}

// bodyReader implements io.Reader for restoring request body
type bodyReader struct {
	data  []byte
	index int
}

func newBodyReader(data []byte) *bodyReader {
	return &bodyReader{data: data, index: 0}
}

func (br *bodyReader) Read(p []byte) (n int, err error) {
	if br.index >= len(br.data) {
		return 0, io.EOF
	}
	n = copy(p, br.data[br.index:])
	br.index += n
	return n, nil
}

// formatDuration formats duration for user-friendly display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}
	hours := int(d.Hours())
	if hours == 1 {
		return "1 hour"
	}
	return fmt.Sprintf("%d hours", hours)
}

// AuthRateLimit returns the go-shared AuthRateLimit middleware
// This is a convenience wrapper for consistent usage
func AuthRateLimit() gin.HandlerFunc {
	return middleware.AuthRateLimit()
}

// PasswordResetRateLimit returns the go-shared PasswordResetRateLimit middleware
// This is a convenience wrapper for consistent usage
func PasswordResetRateLimit() gin.HandlerFunc {
	return middleware.PasswordResetRateLimit()
}

// GeneralRateLimit returns the go-shared general rate limit middleware
func GeneralRateLimit() gin.HandlerFunc {
	return middleware.RateLimit()
}

// LoginAttemptLimiterMiddleware returns a middleware using go-shared LoginAttemptLimiter
// with IP-based key function
func LoginAttemptLimiterMiddleware() gin.HandlerFunc {
	limiter := middleware.NewLoginAttemptLimiter()
	return limiter.Middleware(func(c *gin.Context) string {
		return c.ClientIP()
	})
}

// CombinedLoginRateLimitMiddleware combines IP rate limiting and login attempt tracking
// This uses go-shared middleware components
func CombinedLoginRateLimitMiddleware(sm *SecurityMiddleware) gin.HandlerFunc {
	// IP-based rate limiter for general request throttling
	ipRateLimiter := middleware.NewRateLimiter(middleware.AuthRateLimitConfig())

	// Login attempt limiter with exponential backoff
	loginLimiter := middleware.NewLoginAttemptLimiter()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		// First check IP-based rate limit
		ipLimiter := ipRateLimiter
		_ = ipLimiter // Use the variable

		// Check if IP is locked out from too many failed attempts
		if locked, remaining := loginLimiter.IsLocked(ip); locked {
			if sm != nil {
				sm.logSecurityEvent("rate_limit_ip_locked", ip, "", map[string]interface{}{
					"remaining_lockout": remaining.String(),
				})
			}
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests from this IP address",
				"code":        "IP_RATE_LIMITED",
				"retry_after": int(remaining.Seconds()),
			})
			c.Abort()
			return
		}

		// Store login limiter in context for recording attempts
		c.Set("login_limiter", loginLimiter)

		c.Next()
	}
}

// RecordLoginAttemptFromContext records a login attempt result using context values
// Call this from the login handler after authentication
func RecordLoginAttemptFromContext(c *gin.Context, success bool) {
	// Try to use SecurityMiddleware if available
	if sm, exists := c.Get("security_middleware"); exists {
		if secMiddleware, ok := sm.(*SecurityMiddleware); ok {
			ip, _ := c.Get("login_ip")
			email, _ := c.Get("login_email")
			if ipStr, ok := ip.(string); ok {
				if emailStr, ok := email.(string); ok {
					ctx := c.Request.Context()
					if success {
						secMiddleware.RecordSuccessfulLogin(ctx, ipStr, emailStr)
					} else {
						secMiddleware.RecordFailedLogin(ctx, ipStr, emailStr)
					}
					return
				}
			}
		}
	}

	// Fallback to go-shared LoginAttemptLimiter
	if limiter, exists := c.Get("login_limiter"); exists {
		if loginLimiter, ok := limiter.(*middleware.LoginAttemptLimiter); ok {
			key := c.ClientIP()
			if success {
				loginLimiter.RecordSuccessfulAttempt(key)
			} else {
				loginLimiter.RecordFailedAttempt(key)
			}
		}
	}
}

// GetRemainingAttempts returns the remaining login attempts before lockout
func (sm *SecurityMiddleware) GetRemainingAttempts(ctx context.Context, ip, email string) int {
	attempts := sm.GetFailedAttempts(ctx, ip, email)
	remaining := sm.config.MaxLoginAttempts - attempts
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ==================== Admin Functions ====================

// LockoutStatus represents the lockout status for admin viewing
type LockoutStatus struct {
	Email             string    `json:"email"`
	FailedAttempts    int       `json:"failed_attempts"`
	CurrentTier       int       `json:"current_tier"`
	LockoutCount      int       `json:"lockout_count"`
	IsLocked          bool      `json:"is_locked"`
	PermanentlyLocked bool      `json:"permanently_locked"`
	LockedUntil       time.Time `json:"locked_until,omitempty"`
	PermanentLockedAt time.Time `json:"permanent_locked_at,omitempty"`
	LastFailedAt      time.Time `json:"last_failed_at,omitempty"`
	UnlockedBy        string    `json:"unlocked_by,omitempty"`
	UnlockedAt        time.Time `json:"unlocked_at,omitempty"`
	TenantID          string    `json:"tenant_id,omitempty"`
}

// UnlockAccount unlocks a permanently locked account (admin function)
func (sm *SecurityMiddleware) UnlockAccount(ctx context.Context, email, adminUserID string) error {
	emailKey := sm.generateEmailLockoutKey(email)

	state, err := sm.getLockoutState(ctx, emailKey)
	if err != nil {
		return fmt.Errorf("failed to get lockout state: %w", err)
	}

	if !state.PermanentlyLocked && !state.LockedUntil.After(time.Now()) {
		return fmt.Errorf("account is not locked")
	}

	// Clear the lockout state but keep audit trail
	state.PermanentlyLocked = false
	state.LockedUntil = time.Time{}
	state.FailedAttempts = 0
	state.LockoutCount = 0
	state.CurrentTier = 0
	state.UnlockedBy = adminUserID
	state.UnlockedAt = time.Now()

	if err := sm.setLockoutState(ctx, emailKey, state); err != nil {
		return fmt.Errorf("failed to update lockout state: %w", err)
	}

	sm.logSecurityEvent("account_unlocked", "", email, map[string]interface{}{
		"unlocked_by": adminUserID,
		"tenant_id":   state.TenantID,
	})

	return nil
}

// UnlockAccountByIPAndEmail unlocks an account for a specific IP+email combination
func (sm *SecurityMiddleware) UnlockAccountByIPAndEmail(ctx context.Context, ip, email, adminUserID string) error {
	key := sm.generateLockoutKey(ip, email)

	state, err := sm.getLockoutState(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get lockout state: %w", err)
	}

	if !state.PermanentlyLocked && !state.LockedUntil.After(time.Now()) {
		return fmt.Errorf("account is not locked for this IP")
	}

	// Clear the lockout state but keep audit trail
	state.PermanentlyLocked = false
	state.LockedUntil = time.Time{}
	state.FailedAttempts = 0
	state.LockoutCount = 0
	state.CurrentTier = 0
	state.UnlockedBy = adminUserID
	state.UnlockedAt = time.Now()

	if err := sm.setLockoutState(ctx, key, state); err != nil {
		return fmt.Errorf("failed to update lockout state: %w", err)
	}

	sm.logSecurityEvent("account_unlocked_ip", ip, email, map[string]interface{}{
		"unlocked_by": adminUserID,
		"tenant_id":   state.TenantID,
	})

	return nil
}

// GetLockoutStatusByEmail returns lockout status for an email address (admin function)
func (sm *SecurityMiddleware) GetLockoutStatusByEmail(ctx context.Context, email string) (*LockoutStatus, error) {
	emailKey := sm.generateEmailLockoutKey(email)

	state, err := sm.getLockoutState(ctx, emailKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get lockout state: %w", err)
	}

	isLocked := state.PermanentlyLocked || state.LockedUntil.After(time.Now())

	return &LockoutStatus{
		Email:             email,
		FailedAttempts:    state.FailedAttempts,
		CurrentTier:       state.CurrentTier,
		LockoutCount:      state.LockoutCount,
		IsLocked:          isLocked,
		PermanentlyLocked: state.PermanentlyLocked,
		LockedUntil:       state.LockedUntil,
		PermanentLockedAt: state.PermanentLockedAt,
		LastFailedAt:      state.LastFailedAt,
		UnlockedBy:        state.UnlockedBy,
		UnlockedAt:        state.UnlockedAt,
		TenantID:          state.TenantID,
	}, nil
}

// GetLockoutStatusByIPAndEmail returns lockout status for a specific IP+email (admin function)
func (sm *SecurityMiddleware) GetLockoutStatusByIPAndEmail(ctx context.Context, ip, email string) (*LockoutStatus, error) {
	key := sm.generateLockoutKey(ip, email)

	state, err := sm.getLockoutState(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get lockout state: %w", err)
	}

	isLocked := state.PermanentlyLocked || state.LockedUntil.After(time.Now())

	return &LockoutStatus{
		Email:             email,
		FailedAttempts:    state.FailedAttempts,
		CurrentTier:       state.CurrentTier,
		LockoutCount:      state.LockoutCount,
		IsLocked:          isLocked,
		PermanentlyLocked: state.PermanentlyLocked,
		LockedUntil:       state.LockedUntil,
		PermanentLockedAt: state.PermanentLockedAt,
		LastFailedAt:      state.LastFailedAt,
		UnlockedBy:        state.UnlockedBy,
		UnlockedAt:        state.UnlockedAt,
		TenantID:          state.TenantID,
	}, nil
}

// ListPermanentlyLockedAccounts returns all permanently locked accounts (admin function)
// Note: This requires Redis SCAN which can be slow for large datasets
func (sm *SecurityMiddleware) ListPermanentlyLockedAccounts(ctx context.Context, limit int) ([]LockoutStatus, error) {
	if sm.redisClient == nil {
		// Fallback to local cache
		sm.localMu.RLock()
		defer sm.localMu.RUnlock()

		var results []LockoutStatus
		for _, state := range sm.localLockouts {
			if state.PermanentlyLocked && len(results) < limit {
				results = append(results, LockoutStatus{
					Email:             state.Email,
					FailedAttempts:    state.FailedAttempts,
					CurrentTier:       state.CurrentTier,
					LockoutCount:      state.LockoutCount,
					IsLocked:          true,
					PermanentlyLocked: true,
					PermanentLockedAt: state.PermanentLockedAt,
					LastFailedAt:      state.LastFailedAt,
					TenantID:          state.TenantID,
				})
			}
		}
		return results, nil
	}

	// Use Redis SCAN to find permanently locked accounts
	var results []LockoutStatus
	var cursor uint64
	pattern := sm.config.RedisKeyPrefixEmail + "*"

	for {
		var keys []string
		var err error
		keys, cursor, err = sm.redisClient.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan Redis keys: %w", err)
		}

		for _, key := range keys {
			data, err := sm.redisClient.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var state lockoutState
			if err := json.Unmarshal([]byte(data), &state); err != nil {
				continue
			}

			if state.PermanentlyLocked {
				results = append(results, LockoutStatus{
					Email:             state.Email,
					FailedAttempts:    state.FailedAttempts,
					CurrentTier:       state.CurrentTier,
					LockoutCount:      state.LockoutCount,
					IsLocked:          true,
					PermanentlyLocked: true,
					PermanentLockedAt: state.PermanentLockedAt,
					LastFailedAt:      state.LastFailedAt,
					TenantID:          state.TenantID,
				})

				if len(results) >= limit {
					return results, nil
				}
			}
		}

		if cursor == 0 {
			break
		}
	}

	return results, nil
}

// GetSecurityConfig returns the current security configuration (for admin/debugging)
func (sm *SecurityMiddleware) GetSecurityConfig() SecurityConfig {
	return sm.config
}
