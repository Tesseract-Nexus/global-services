package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
)

// Claims represents the JWT claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	TenantID string   `json:"tenant_id"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates JWT tokens and extracts user information
func AuthMiddleware(jwtSecret string, logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health check endpoints
		if strings.HasPrefix(c.Request.URL.Path, "/health") {
			c.Next()
			return
		}

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Warn("Missing authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "MISSING_TOKEN",
					"message": "Authorization header is required",
				},
			})
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			logger.Warn("Invalid authorization header format")
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN_FORMAT",
					"message": "Authorization header must be in format: Bearer <token>",
				},
			})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Parse and validate token
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			// Make sure token method is HMAC
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil {
			logger.WithError(err).Warn("Invalid or expired token")
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Invalid or expired token",
				},
			})
			c.Abort()
			return
		}

		// Extract claims
		if claims, ok := token.Claims.(*Claims); ok && token.Valid {
			// Set user info in context
			c.Set("user_id", claims.UserID)
			c.Set("user_email", claims.Email)
			c.Set("user_roles", claims.Roles)
			c.Set("tenant_id", claims.TenantID)

			logger.WithFields(logrus.Fields{
				"user_id":   claims.UserID,
				"tenant_id": claims.TenantID,
				"email":     claims.Email,
			}).Debug("User authenticated successfully")

			c.Next()
		} else {
			logger.Warn("Invalid token claims")
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_CLAIMS",
					"message": "Invalid token claims",
				},
			})
			c.Abort()
			return
		}
	}
}

// RequireRole middleware checks if user has required role
func RequireRole(requiredRole string, logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			logger.Warn("User roles not found in context")
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NO_ROLES",
					"message": "User roles not found",
				},
			})
			c.Abort()
			return
		}

		userRoles, ok := roles.([]string)
		if !ok {
			logger.Warn("Invalid user roles format")
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ROLES",
					"message": "Invalid user roles format",
				},
			})
			c.Abort()
			return
		}

		// Check if user has required role
		hasRole := false
		for _, role := range userRoles {
			if role == requiredRole || role == "super_admin" {
				hasRole = true
				break
			}
		}

		if !hasRole {
			logger.WithFields(logrus.Fields{
				"user_roles":    userRoles,
				"required_role": requiredRole,
			}).Warn("Insufficient permissions")

			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INSUFFICIENT_PERMISSIONS",
					"message": fmt.Sprintf("Required role: %s", requiredRole),
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyRole middleware checks if user has any of the required roles
func RequireAnyRole(requiredRoles []string, logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			logger.Warn("User roles not found in context")
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "NO_ROLES",
					"message": "User roles not found",
				},
			})
			c.Abort()
			return
		}

		userRoles, ok := roles.([]string)
		if !ok {
			logger.Warn("Invalid user roles format")
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_ROLES",
					"message": "Invalid user roles format",
				},
			})
			c.Abort()
			return
		}

		// Check if user has any required role
		hasRole := false
		for _, userRole := range userRoles {
			if userRole == "super_admin" {
				hasRole = true
				break
			}
			for _, requiredRole := range requiredRoles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			logger.WithFields(logrus.Fields{
				"user_roles":     userRoles,
				"required_roles": requiredRoles,
			}).Warn("Insufficient permissions")

			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INSUFFICIENT_PERMISSIONS",
					"message": fmt.Sprintf("Required one of roles: %v", requiredRoles),
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
