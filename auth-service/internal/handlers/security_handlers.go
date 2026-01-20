package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"auth-service/internal/events"
	"auth-service/internal/middleware"
	"auth-service/internal/repository"
)

// SecurityHandlers handles security-related admin endpoints
type SecurityHandlers struct {
	securityMw *middleware.SecurityMiddleware
	authRepo   *repository.AuthRepository
	events     *events.Publisher
}

// NewSecurityHandlers creates a new SecurityHandlers instance
func NewSecurityHandlers(securityMw *middleware.SecurityMiddleware, authRepo *repository.AuthRepository, events *events.Publisher) *SecurityHandlers {
	return &SecurityHandlers{
		securityMw: securityMw,
		authRepo:   authRepo,
		events:     events,
	}
}

// UnlockAccountRequest represents the request to unlock an account
type UnlockAccountRequest struct {
	Email string `json:"email"` // Optional: unlock by email instead of user_id
}

// UnlockAccount handles POST /api/v1/admin/users/:user_id/unlock
// Unlocks a permanently locked account
func (h *SecurityHandlers) UnlockAccount(c *gin.Context) {
	userIDStr := c.Param("user_id")

	// Get the admin user ID from context (set by auth middleware)
	adminUserID := c.GetString("user_id")
	if adminUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Admin authentication required",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	// Parse user ID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
			"code":  "INVALID_USER_ID",
		})
		return
	}

	// Get user from repository to get their email
	user, err := h.authRepo.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
			"code":  "USER_NOT_FOUND",
		})
		return
	}

	// Unlock the account
	ctx := c.Request.Context()
	if err := h.securityMw.UnlockAccount(ctx, user.Email, adminUserID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "UNLOCK_FAILED",
		})
		return
	}

	// Publish audit event
	if h.events != nil {
		tenantID := c.GetString("tenant_id")
		h.events.PublishAccountUnlocked(ctx, tenantID, userIDStr, user.Email, adminUserID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Account unlocked successfully",
		"user_id": userIDStr,
		"email":   user.Email,
	})
}

// UnlockAccountByEmail handles POST /api/v1/admin/security/unlock-by-email
// Unlocks an account by email address
func (h *SecurityHandlers) UnlockAccountByEmail(c *gin.Context) {
	var req UnlockAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"code":  "INVALID_REQUEST",
		})
		return
	}

	if req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Email is required",
			"code":  "EMAIL_REQUIRED",
		})
		return
	}

	// Get the admin user ID from context
	adminUserID := c.GetString("user_id")
	if adminUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Admin authentication required",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	// Unlock the account
	ctx := c.Request.Context()
	if err := h.securityMw.UnlockAccount(ctx, req.Email, adminUserID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"code":  "UNLOCK_FAILED",
		})
		return
	}

	// Try to find the user to get their ID for the event
	user, _ := h.authRepo.GetUserByEmail(req.Email)
	var userIDStr string
	if user != nil {
		userIDStr = user.ID.String()
	}

	// Publish audit event
	if h.events != nil {
		tenantID := c.GetString("tenant_id")
		h.events.PublishAccountUnlocked(ctx, tenantID, userIDStr, req.Email, adminUserID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Account unlocked successfully",
		"email":   req.Email,
	})
}

// GetLockoutStatus handles GET /api/v1/admin/users/:user_id/lockout-status
// Returns the lockout status for a user
func (h *SecurityHandlers) GetLockoutStatus(c *gin.Context) {
	userIDStr := c.Param("user_id")

	// Parse user ID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
			"code":  "INVALID_USER_ID",
		})
		return
	}

	// Get user from repository to get their email
	user, err := h.authRepo.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
			"code":  "USER_NOT_FOUND",
		})
		return
	}

	// Get lockout status
	ctx := c.Request.Context()
	status, err := h.securityMw.GetLockoutStatusByEmail(ctx, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get lockout status",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id":            userIDStr,
			"email":              user.Email,
			"failed_attempts":    status.FailedAttempts,
			"current_tier":       status.CurrentTier,
			"lockout_count":      status.LockoutCount,
			"is_locked":          status.IsLocked,
			"permanently_locked": status.PermanentlyLocked,
			"locked_until":       status.LockedUntil,
			"permanent_locked_at": status.PermanentLockedAt,
			"last_failed_at":     status.LastFailedAt,
			"unlocked_by":        status.UnlockedBy,
			"unlocked_at":        status.UnlockedAt,
		},
	})
}

// ListLockedAccounts handles GET /api/v1/admin/locked-accounts
// Returns all permanently locked accounts
func (h *SecurityHandlers) ListLockedAccounts(c *gin.Context) {
	// Get limit from query param, default to 100
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := parseInt(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	ctx := c.Request.Context()
	accounts, err := h.securityMw.ListPermanentlyLockedAccounts(ctx, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list locked accounts",
			"code":  "INTERNAL_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    accounts,
		"count":   len(accounts),
	})
}

// GetSecurityConfig handles GET /api/v1/admin/security/config
// Returns the current security configuration (for debugging)
func (h *SecurityHandlers) GetSecurityConfig(c *gin.Context) {
	config := h.securityMw.GetSecurityConfig()

	// Convert tiers to a more readable format
	tiers := make([]gin.H, len(config.LockoutTiers))
	for i, tier := range config.LockoutTiers {
		var duration string
		if tier.Duration == 0 {
			duration = "permanent"
		} else {
			duration = tier.Duration.String()
		}
		tiers[i] = gin.H{
			"tier":     tier.Tier,
			"duration": duration,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"max_login_attempts":          config.MaxLoginAttempts,
			"lockout_tiers":               tiers,
			"permanent_lockout_threshold": config.PermanentLockoutThreshold,
			"lockout_reset_after":         config.LockoutResetAfter.String(),
		},
	})
}

// parseInt parses a string to int
func parseInt(s string) (int, error) {
	var i int
	_, err := parseIntInto(s, &i)
	return i, err
}

func parseIntInto(s string, i *int) (bool, error) {
	if s == "" {
		return false, nil
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
		n = n*10 + int(c-'0')
	}
	*i = n
	return true, nil
}
