package handlers

import (
	"net/http"
	"strconv"

	"auth-service/internal/events"
	"auth-service/internal/middleware"
	"auth-service/internal/models"
	"auth-service/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandlers struct {
	authService     *services.AuthService
	eventsPublisher *events.Publisher
}

func NewAuthHandlers(authService *services.AuthService, eventsPublisher *events.Publisher) *AuthHandlers {
	return &AuthHandlers{
		authService:     authService,
		eventsPublisher: eventsPublisher,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email         string `json:"email" binding:"required,email"`
	Password      string `json:"password"` // Optional - for password-based login
	Name          string `json:"name"`     // Optional - for JWT mock login
	AzureObjectID string `json:"azure_object_id"`
	TenantID      string `json:"tenant_id"` // Optional - backend can determine from email
}

// LoginResponse represents a login response
type LoginResponse struct {
	User         *models.User `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
}

// RefreshTokenRequest represents a refresh token request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Login authenticates a user
func (h *AuthHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get client info
	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	var user *models.User
	var accessToken string
	var refreshToken string
	var err error

	// Determine authentication method
	if req.Password != "" {
		// Password-based authentication
		user, accessToken, refreshToken, err = h.authService.AuthenticateWithPassword(
			req.Email, req.Password, nil, req.TenantID, ipAddress, userAgent,
		)
		if err != nil {
			// Record failed login attempt for account lockout
			middleware.RecordLoginAttemptFromContext(c, false)

			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid credentials",
				"details": err.Error(),
			})
			return
		}

		// Record successful login attempt (clears lockout state)
		middleware.RecordLoginAttemptFromContext(c, true)

		// TODO: Re-enable email verification check after integrating with onboarding verification
		// For now, we allow login without email verification since it's handled in onboarding
		// if !user.EmailVerified {
		// 	c.JSON(http.StatusForbidden, gin.H{
		// 		"error":          "Email not verified",
		// 		"message":        "Please verify your email before logging in",
		// 		"email_verified": false,
		// 	})
		// 	return
		// }
	} else {
		// JWT/Azure AD authentication (fallback for backward compatibility)
		if req.Name == "" || req.TenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "For non-password login, both name and tenant_id are required",
			})
			return
		}

		user, accessToken, refreshToken, err = h.authService.AuthenticateUser(
			req.Email, req.AzureObjectID, req.Name, req.TenantID, ipAddress, userAgent,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Authentication failed",
				"details": err.Error(),
			})
			return
		}
	}

	// Return response
	c.JSON(http.StatusOK, LoginResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(h.authService.GetTokenExpiry().Seconds()),
	})
}

// RefreshToken refreshes an access token
func (h *AuthHandlers) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Refresh tokens
	newAccessToken, newRefreshToken, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Token refresh failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
		"expires_in":    int(h.authService.GetTokenExpiry().Seconds()),
	})
}

// Logout revokes a user's token
func (h *AuthHandlers) Logout(c *gin.Context) {
	// Extract token from header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Authorization header required",
		})
		return
	}

	// Extract token
	token := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid authorization header format",
		})
		return
	}

	// Revoke token
	if err := h.authService.RevokeToken(token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to revoke token",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully logged out",
	})
}

// GetProfile returns the current user's profile
func (h *AuthHandlers) GetProfile(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User context not found",
		})
		return
	}

	user, err := h.authService.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get user profile",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// ValidateToken validates a JWT token
func (h *AuthHandlers) ValidateToken(c *gin.Context) {
	// Extract token from header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Authorization header required",
		})
		return
	}

	// Extract token
	token := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid authorization header format",
		})
		return
	}

	// Validate token
	claims, err := h.authService.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid token",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":  true,
		"claims": claims,
	})
}

// CheckPermission checks if the current user has a specific permission
func (h *AuthHandlers) CheckPermission(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User context not found",
		})
		return
	}

	permission := c.Param("permission")
	if permission == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Permission parameter required",
		})
		return
	}

	hasPermission, err := h.authService.HasPermission(userID, permission)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check permission",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_permission": hasPermission,
		"permission":     permission,
		"user_id":        userID,
	})
}

// CheckPermissions checks multiple permissions at once
func (h *AuthHandlers) CheckPermissions(c *gin.Context) {
	userID, err := middleware.GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User context not found",
		})
		return
	}

	var req struct {
		Permissions []string `json:"permissions" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	permissions, err := h.authService.CheckPermissions(userID, req.Permissions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check permissions",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"permissions": permissions,
		"user_id":     userID,
	})
}

// User Management Handlers

// ListUsers lists users with pagination
func (h *AuthHandlers) ListUsers(c *gin.Context) {
	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant context not found",
		})
		return
	}

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 10
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	users, err := h.authService.ListUsers(tenantID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list users",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users":  users,
		"limit":  limit,
		"offset": offset,
	})
}

// GetUser retrieves a specific user
func (h *AuthHandlers) GetUser(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	user, err := h.authService.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "User not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// AssignRole assigns a role to a user
func (h *AuthHandlers) AssignRole(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant context not found",
		})
		return
	}

	if err := h.authService.AssignRole(userID, req.Role, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to assign role",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Role assigned successfully",
		"user_id": userID,
		"role":    req.Role,
	})
}

// RemoveRole removes a role from a user
func (h *AuthHandlers) RemoveRole(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID format",
		})
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	tenantID, err := middleware.GetTenantID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Tenant context not found",
		})
		return
	}

	if err := h.authService.RemoveRole(userID, req.Role, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to remove role",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Role removed successfully",
		"user_id": userID,
		"role":    req.Role,
	})
}

// GetAvailableRoles returns available roles for assignment
func (h *AuthHandlers) GetAvailableRoles(c *gin.Context) {
	roles := h.authService.GetAvailableRoles()
	c.JSON(http.StatusOK, gin.H{
		"roles": roles,
	})
}

// GetAvailablePermissions returns all available permissions
func (h *AuthHandlers) GetAvailablePermissions(c *gin.Context) {
	permissions := h.authService.GetAvailablePermissions()
	c.JSON(http.StatusOK, gin.H{
		"permissions": permissions,
	})
}

// Health check handler
func (h *AuthHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "auth-service",
		"version": "1.0.0",
	})
}

// Ready check handler - indicates the service is ready to accept traffic
func (h *AuthHandlers) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ready",
		"service": "auth-service",
	})
}
