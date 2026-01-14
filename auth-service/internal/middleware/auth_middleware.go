package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"auth-service/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthMiddleware struct {
	authService *services.AuthService
}

func NewAuthMiddleware(authService *services.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
	}
}

// AuthRequired middleware that requires a valid JWT token
func (m *AuthMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.extractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization token required",
				"code":  "MISSING_TOKEN",
			})
			c.Abort()
			return
		}

		claims, err := m.authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
				"code":  "INVALID_TOKEN",
			})
			c.Abort()
			return
		}

		// Set user context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_name", claims.Name)
		c.Set("tenant_id", claims.TenantID)
		c.Set("user_roles", claims.Roles)
		c.Set("user_permissions", claims.Permissions)
		c.Set("session_id", claims.SessionID)

		c.Next()
	}
}

// RequirePermission middleware that requires a specific permission
func (m *AuthMiddleware) RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.AuthRequired()(c)
		if c.IsAborted() {
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User context not found",
				"code":  "MISSING_USER_CONTEXT",
			})
			c.Abort()
			return
		}

		userUUID, ok := userID.(uuid.UUID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
				"code":  "INVALID_USER_ID",
			})
			c.Abort()
			return
		}

		hasPermission, err := m.authService.HasPermission(userUUID, permission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to check permission",
				"code":  "PERMISSION_CHECK_FAILED",
			})
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Insufficient permissions",
				"code":     "INSUFFICIENT_PERMISSIONS",
				"required": permission,
				"user_id":  userUUID,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyPermission middleware that requires any of the specified permissions
func (m *AuthMiddleware) RequireAnyPermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.AuthRequired()(c)
		if c.IsAborted() {
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User context not found",
				"code":  "MISSING_USER_CONTEXT",
			})
			c.Abort()
			return
		}

		userUUID, ok := userID.(uuid.UUID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
				"code":  "INVALID_USER_ID",
			})
			c.Abort()
			return
		}

		// Check if user has any of the required permissions
		for _, permission := range permissions {
			hasPermission, err := m.authService.HasPermission(userUUID, permission)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to check permission",
					"code":  "PERMISSION_CHECK_FAILED",
				})
				c.Abort()
				return
			}

			if hasPermission {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":    "Insufficient permissions",
			"code":     "INSUFFICIENT_PERMISSIONS",
			"required": permissions,
			"user_id":  userUUID,
		})
		c.Abort()
	}
}

// RequireRole middleware that requires a specific role
func (m *AuthMiddleware) RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.AuthRequired()(c)
		if c.IsAborted() {
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User context not found",
				"code":  "MISSING_USER_CONTEXT",
			})
			c.Abort()
			return
		}

		userUUID, ok := userID.(uuid.UUID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
				"code":  "INVALID_USER_ID",
			})
			c.Abort()
			return
		}

		hasRole, err := m.authService.HasRole(userUUID, role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to check role",
				"code":  "ROLE_CHECK_FAILED",
			})
			c.Abort()
			return
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Insufficient role",
				"code":     "INSUFFICIENT_ROLE",
				"required": role,
				"user_id":  userUUID,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyRole middleware that requires any of the specified roles
func (m *AuthMiddleware) RequireAnyRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.AuthRequired()(c)
		if c.IsAborted() {
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User context not found",
				"code":  "MISSING_USER_CONTEXT",
			})
			c.Abort()
			return
		}

		userUUID, ok := userID.(uuid.UUID)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
				"code":  "INVALID_USER_ID",
			})
			c.Abort()
			return
		}

		hasAnyRole, err := m.authService.HasAnyRole(userUUID, roles)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to check roles",
				"code":  "ROLE_CHECK_FAILED",
			})
			c.Abort()
			return
		}

		if !hasAnyRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Insufficient role",
				"code":     "INSUFFICIENT_ROLE",
				"required": roles,
				"user_id":  userUUID,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminOnly middleware that requires super admin or tenant admin role
func (m *AuthMiddleware) AdminOnly() gin.HandlerFunc {
	return m.RequireAnyRole("super_admin", "tenant_admin")
}

// SuperAdminOnly middleware that requires super admin role
func (m *AuthMiddleware) SuperAdminOnly() gin.HandlerFunc {
	return m.RequireRole("super_admin")
}

// TenantFilter middleware that ensures users can only access their tenant's data
func (m *AuthMiddleware) TenantFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.AuthRequired()(c)
		if c.IsAborted() {
			return
		}

		tenantID, exists := c.Get("tenant_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Tenant context not found",
				"code":  "MISSING_TENANT_CONTEXT",
			})
			c.Abort()
			return
		}

		tenantIDStr, ok := tenantID.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid tenant ID format",
				"code":  "INVALID_TENANT_ID",
			})
			c.Abort()
			return
		}

		// Add tenant ID to query parameters for filtering
		c.Set("filtered_tenant_id", tenantIDStr)
		c.Next()
	}
}

// extractToken extracts the JWT token from the Authorization header
func (m *AuthMiddleware) extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	// Check for Bearer token format
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}

// GetUserID utility function to get user ID from context
func GetUserID(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, fmt.Errorf("user ID not found in context")
	}

	userUUID, ok := userID.(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid user ID format")
	}

	return userUUID, nil
}

// GetTenantID utility function to get tenant ID from context
func GetTenantID(c *gin.Context) (string, error) {
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		return "", fmt.Errorf("tenant ID not found in context")
	}

	tenantIDStr, ok := tenantID.(string)
	if !ok {
		return "", fmt.Errorf("invalid tenant ID format")
	}

	return tenantIDStr, nil
}

// GetUserRoles utility function to get user roles from context
func GetUserRoles(c *gin.Context) ([]string, error) {
	roles, exists := c.Get("user_roles")
	if !exists {
		return nil, fmt.Errorf("user roles not found in context")
	}

	rolesSlice, ok := roles.([]string)
	if !ok {
		return nil, fmt.Errorf("invalid user roles format")
	}

	return rolesSlice, nil
}

// GetUserPermissions utility function to get user permissions from context
func GetUserPermissions(c *gin.Context) ([]string, error) {
	permissions, exists := c.Get("user_permissions")
	if !exists {
		return nil, fmt.Errorf("user permissions not found in context")
	}

	permissionsSlice, ok := permissions.([]string)
	if !ok {
		return nil, fmt.Errorf("invalid user permissions format")
	}

	return permissionsSlice, nil
}
