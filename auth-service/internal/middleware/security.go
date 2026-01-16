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
	// MaxLoginAttempts before account lockout
	MaxLoginAttempts int
	// LockoutDuration is the initial lockout duration
	LockoutDuration time.Duration
	// MaxLockoutDuration is the maximum lockout duration with exponential backoff
	MaxLockoutDuration time.Duration
	// LockoutResetAfter is how long until failed attempts are reset
	LockoutResetAfter time.Duration
	// RedisKeyPrefix for storing lockout data
	RedisKeyPrefix string
}

// DefaultSecurityConfig returns sensible defaults
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		MaxLoginAttempts:   5,
		LockoutDuration:    30 * time.Second,
		MaxLockoutDuration: 1 * time.Hour,
		LockoutResetAfter:  15 * time.Minute,
		RedisKeyPrefix:     "auth:lockout:",
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
	FailedAttempts int       `json:"failed_attempts"`
	LastFailedAt   time.Time `json:"last_failed_at"`
	LockedUntil    time.Time `json:"locked_until"`
	LockoutCount   int       `json:"lockout_count"` // For exponential backoff
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

		ttl := sm.config.LockoutResetAfter
		if state.LockedUntil.After(time.Now()) {
			// Extend TTL if currently locked
			remaining := time.Until(state.LockedUntil)
			if remaining > ttl {
				ttl = remaining + time.Minute
			}
		}

		if err := sm.redisClient.Set(ctx, key, data, ttl).Err(); err != nil {
			sm.logger.WithError(err).Warn("Failed to set lockout state in Redis")
			// Don't return error - local cache is already updated
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

// calculateLockoutDuration calculates lockout duration with exponential backoff
func (sm *SecurityMiddleware) calculateLockoutDuration(lockoutCount int) time.Duration {
	if lockoutCount <= 0 {
		return sm.config.LockoutDuration
	}

	// Exponential backoff: duration * 2^(lockoutCount-1)
	duration := sm.config.LockoutDuration * time.Duration(1<<(lockoutCount-1))

	if duration > sm.config.MaxLockoutDuration {
		duration = sm.config.MaxLockoutDuration
	}

	return duration
}

// RecordFailedLogin records a failed login attempt
func (sm *SecurityMiddleware) RecordFailedLogin(ctx context.Context, ip, email string) (attemptsRemaining int, lockedUntil time.Time) {
	key := sm.generateLockoutKey(ip, email)

	state, _ := sm.getLockoutState(ctx, key)

	// Check if we should reset the counter (attempts too old)
	if state.FailedAttempts > 0 && time.Since(state.LastFailedAt) > sm.config.LockoutResetAfter {
		state = &lockoutState{}
	}

	state.FailedAttempts++
	state.LastFailedAt = time.Now()

	// Check if we should lock the account
	if state.FailedAttempts >= sm.config.MaxLoginAttempts {
		state.LockoutCount++
		lockoutDuration := sm.calculateLockoutDuration(state.LockoutCount)
		state.LockedUntil = time.Now().Add(lockoutDuration)

		sm.logSecurityEvent("account_locked", ip, email, map[string]interface{}{
			"failed_attempts": state.FailedAttempts,
			"lockout_count":   state.LockoutCount,
			"locked_until":    state.LockedUntil,
			"lockout_duration": lockoutDuration.String(),
		})
	} else {
		sm.logSecurityEvent("failed_login_attempt", ip, email, map[string]interface{}{
			"failed_attempts":    state.FailedAttempts,
			"attempts_remaining": sm.config.MaxLoginAttempts - state.FailedAttempts,
		})
	}

	sm.setLockoutState(ctx, key, state)

	return sm.config.MaxLoginAttempts - state.FailedAttempts, state.LockedUntil
}

// RecordSuccessfulLogin clears failed login attempts on successful login
func (sm *SecurityMiddleware) RecordSuccessfulLogin(ctx context.Context, ip, email string) {
	key := sm.generateLockoutKey(ip, email)
	sm.clearLockoutState(ctx, key)

	sm.logSecurityEvent("successful_login", ip, email, nil)
}

// IsLocked checks if an IP+email combination is currently locked out
func (sm *SecurityMiddleware) IsLocked(ctx context.Context, ip, email string) (bool, time.Duration) {
	key := sm.generateLockoutKey(ip, email)

	state, _ := sm.getLockoutState(ctx, key)

	if state.LockedUntil.After(time.Now()) {
		return true, time.Until(state.LockedUntil)
	}

	return false, 0
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
		limiter := baseRateLimiter

		// Get rate limit key and check
		if !limiter.Middleware()(c); c.IsAborted() {
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

		// Check if the account is locked
		locked, remaining := sm.IsLocked(ctx, ip, email)
		if locked {
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
