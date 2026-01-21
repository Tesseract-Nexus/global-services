package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"tenant-service/internal/config"
)

// Client wraps the Redis client with application-specific methods
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

// Ping checks the connection to Redis
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Key prefixes
const (
	DraftKeyPrefix           = "draft:"
	HeartbeatKeyPrefix       = "draft:heartbeat:"
	ReminderKeyPrefix        = "draft:reminder:"
	VerificationTokenPrefix  = "verify:token:"
	VerificationStatusPrefix = "verify:status:"
	SessionTokensPrefix      = "session:tokens:" // Tracks active tokens per session
)

// DraftData represents the cached draft data
type DraftData struct {
	SessionID       string                 `json:"session_id"`
	FormData        map[string]interface{} `json:"form_data"`
	CurrentStep     int                    `json:"current_step"`
	Progress        int                    `json:"progress"`
	LastSavedAt     time.Time              `json:"last_saved_at"`
	BrowserClosedAt *time.Time             `json:"browser_closed_at,omitempty"`
	Email           string                 `json:"email,omitempty"`
	Phone           string                 `json:"phone,omitempty"`
}

// SaveDraft saves draft data to Redis
func (c *Client) SaveDraft(ctx context.Context, sessionID string, data *DraftData, ttl time.Duration) error {
	key := DraftKeyPrefix + sessionID
	data.LastSavedAt = time.Now()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal draft data: %w", err)
	}

	return c.rdb.Set(ctx, key, jsonData, ttl).Err()
}

// GetDraft retrieves draft data from Redis
func (c *Client) GetDraft(ctx context.Context, sessionID string) (*DraftData, error) {
	key := DraftKeyPrefix + sessionID

	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get draft data: %w", err)
	}

	var draft DraftData
	if err := json.Unmarshal(data, &draft); err != nil {
		return nil, fmt.Errorf("failed to unmarshal draft data: %w", err)
	}

	return &draft, nil
}

// DeleteDraft removes draft data from Redis
func (c *Client) DeleteDraft(ctx context.Context, sessionID string) error {
	key := DraftKeyPrefix + sessionID
	return c.rdb.Del(ctx, key).Err()
}

// UpdateHeartbeat updates the last heartbeat timestamp for a session
func (c *Client) UpdateHeartbeat(ctx context.Context, sessionID string, ttl time.Duration) error {
	key := HeartbeatKeyPrefix + sessionID
	return c.rdb.Set(ctx, key, time.Now().Unix(), ttl).Err()
}

// GetLastHeartbeat gets the last heartbeat timestamp for a session
func (c *Client) GetLastHeartbeat(ctx context.Context, sessionID string) (*time.Time, error) {
	key := HeartbeatKeyPrefix + sessionID

	timestamp, err := c.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return nil, nil // No heartbeat
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get heartbeat: %w", err)
	}

	t := time.Unix(timestamp, 0)
	return &t, nil
}

// MarkBrowserClosed marks that the browser was closed for a session
func (c *Client) MarkBrowserClosed(ctx context.Context, sessionID string) error {
	draft, err := c.GetDraft(ctx, sessionID)
	if err != nil {
		return err
	}
	if draft == nil {
		return nil // No draft to update
	}

	now := time.Now()
	draft.BrowserClosedAt = &now

	// Get remaining TTL
	ttl, err := c.rdb.TTL(ctx, DraftKeyPrefix+sessionID).Result()
	if err != nil {
		ttl = 48 * time.Hour // Default
	}

	return c.SaveDraft(ctx, sessionID, draft, ttl)
}

// ReminderStatus tracks reminder state for a session
type ReminderStatus struct {
	SessionID      string    `json:"session_id"`
	ReminderCount  int       `json:"reminder_count"`
	LastReminderAt time.Time `json:"last_reminder_at"`
}

// GetReminderStatus gets the reminder status for a session
func (c *Client) GetReminderStatus(ctx context.Context, sessionID string) (*ReminderStatus, error) {
	key := ReminderKeyPrefix + sessionID

	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return &ReminderStatus{SessionID: sessionID, ReminderCount: 0}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get reminder status: %w", err)
	}

	var status ReminderStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reminder status: %w", err)
	}

	return &status, nil
}

// UpdateReminderStatus updates the reminder status for a session
func (c *Client) UpdateReminderStatus(ctx context.Context, status *ReminderStatus, ttl time.Duration) error {
	key := ReminderKeyPrefix + status.SessionID
	status.LastReminderAt = time.Now()

	jsonData, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal reminder status: %w", err)
	}

	return c.rdb.Set(ctx, key, jsonData, ttl).Err()
}

// GetAllDraftKeys returns all draft keys (for cleanup job)
func (c *Client) GetAllDraftKeys(ctx context.Context) ([]string, error) {
	pattern := DraftKeyPrefix + "*"
	var cursor uint64
	var keys []string

	for {
		var batch []string
		var err error
		batch, cursor, err = c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan keys: %w", err)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// GetDraftTTL gets the remaining TTL for a draft
func (c *Client) GetDraftTTL(ctx context.Context, sessionID string) (time.Duration, error) {
	key := DraftKeyPrefix + sessionID
	return c.rdb.TTL(ctx, key).Result()
}

// VerificationTokenData represents email verification token data
type VerificationTokenData struct {
	Token     string    `json:"token"`
	SessionID string    `json:"session_id"`
	Email     string    `json:"email"`
	Purpose   string    `json:"purpose"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// EmailVerificationStatus tracks verification status by email
type EmailVerificationStatus struct {
	Email      string     `json:"email"`
	SessionID  string     `json:"session_id"`
	IsVerified bool       `json:"is_verified"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

// SaveVerificationToken saves a verification token to Redis and tracks it under the session
func (c *Client) SaveVerificationToken(ctx context.Context, token string, data *VerificationTokenData, ttl time.Duration) error {
	key := VerificationTokenPrefix + token
	data.CreatedAt = time.Now()
	data.ExpiresAt = time.Now().Add(ttl)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal verification token data: %w", err)
	}

	// Save the token
	if err := c.rdb.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		return err
	}

	// Track this token under the session for invalidation purposes
	// Store the token in a set associated with the session
	sessionTokensKey := SessionTokensPrefix + data.SessionID
	if err := c.rdb.SAdd(ctx, sessionTokensKey, token).Err(); err != nil {
		// Log but don't fail - token is already saved
		fmt.Printf("Warning: failed to track token under session: %v\n", err)
	}
	// Set expiry on the session tokens set (same as token TTL)
	c.rdb.Expire(ctx, sessionTokensKey, ttl)

	return nil
}

// InvalidateSessionTokens invalidates all verification tokens for a session
// This should be called when a new verification email is sent to prevent old tokens from being used
func (c *Client) InvalidateSessionTokens(ctx context.Context, sessionID string) error {
	sessionTokensKey := SessionTokensPrefix + sessionID

	// Get all tokens for this session
	tokens, err := c.rdb.SMembers(ctx, sessionTokensKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // No tokens to invalidate
		}
		return fmt.Errorf("failed to get session tokens: %w", err)
	}

	// Delete each token
	for _, token := range tokens {
		tokenKey := VerificationTokenPrefix + token
		if err := c.rdb.Del(ctx, tokenKey).Err(); err != nil {
			fmt.Printf("Warning: failed to delete token %s: %v\n", token, err)
		}
	}

	// Clear the session tokens set
	if err := c.rdb.Del(ctx, sessionTokensKey).Err(); err != nil {
		return fmt.Errorf("failed to delete session tokens set: %w", err)
	}

	return nil
}

// GetVerificationToken retrieves verification token data from Redis
func (c *Client) GetVerificationToken(ctx context.Context, token string) (*VerificationTokenData, error) {
	key := VerificationTokenPrefix + token

	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Not found or expired
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get verification token: %w", err)
	}

	var tokenData VerificationTokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal verification token: %w", err)
	}

	return &tokenData, nil
}

// DeleteVerificationToken removes a verification token from Redis
func (c *Client) DeleteVerificationToken(ctx context.Context, token string) error {
	key := VerificationTokenPrefix + token
	return c.rdb.Del(ctx, key).Err()
}

// SaveEmailVerificationStatus saves email verification status to Redis
func (c *Client) SaveEmailVerificationStatus(ctx context.Context, email, sessionID string, isVerified bool, ttl time.Duration) error {
	key := VerificationStatusPrefix + email

	status := &EmailVerificationStatus{
		Email:      email,
		SessionID:  sessionID,
		IsVerified: isVerified,
	}
	if isVerified {
		now := time.Now()
		status.VerifiedAt = &now
	}

	jsonData, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal verification status: %w", err)
	}

	return c.rdb.Set(ctx, key, jsonData, ttl).Err()
}

// GetEmailVerificationStatus retrieves email verification status from Redis
func (c *Client) GetEmailVerificationStatus(ctx context.Context, email string) (*EmailVerificationStatus, error) {
	key := VerificationStatusPrefix + email

	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get verification status: %w", err)
	}

	var status EmailVerificationStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal verification status: %w", err)
	}

	return &status, nil
}

// GetVerificationTokenBySession retrieves a verification token by session ID and email
// This is used for E2E testing to retrieve the verification link token
func (c *Client) GetVerificationTokenBySession(ctx context.Context, sessionID, email string) (string, error) {
	// Scan for keys matching the verification token pattern for this session
	pattern := fmt.Sprintf("verification:token:*:%s:%s", sessionID, email)
	var token string

	iter := c.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// Extract token from key: verification:token:{token}:{sessionID}:{email}
		parts := make([]string, 0)
		current := ""
		for _, c := range key {
			if c == ':' {
				parts = append(parts, current)
				current = ""
			} else {
				current += string(c)
			}
		}
		if current != "" {
			parts = append(parts, current)
		}
		if len(parts) >= 3 {
			token = parts[2] // The token is the 3rd part
			break
		}
	}

	if err := iter.Err(); err != nil {
		return "", fmt.Errorf("failed to scan for verification token: %w", err)
	}

	if token == "" {
		return "", fmt.Errorf("verification token not found for session %s and email %s", sessionID, email)
	}

	return token, nil
}
