package middleware

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// AzureADClaims represents Azure AD JWT token claims
type AzureADClaims struct {
	ObjectID          string   `json:"oid"`   // User object ID
	TenantID          string   `json:"tid"`   // Tenant ID
	Email             string   `json:"email"` // User email
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`        // Display name
	GivenName         string   `json:"given_name"`  // First name
	FamilyName        string   `json:"family_name"` // Last name
	Roles             []string `json:"roles"`       // Application roles
	Groups            []string `json:"groups"`      // Group memberships
	AppID             string   `json:"appid"`       // Application ID
	Audience          string   `json:"aud"`         // Audience
	Issuer            string   `json:"iss"`         // Issuer
	jwt.RegisteredClaims
}

// JWKSResponse represents the response from Azure AD JWKS endpoint
type JWKSResponse struct {
	Keys []JSONWebKey `json:"keys"`
}

// JSONWebKey represents a JSON Web Key
type JSONWebKey struct {
	Kty string   `json:"kty"`
	Use string   `json:"use"`
	Kid string   `json:"kid"`
	X5t string   `json:"x5t"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

// AzureADAuthMiddleware validates Azure AD JWT tokens
func AzureADAuthMiddleware(tenantID string, applicationID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health check endpoints
		if strings.HasPrefix(c.Request.URL.Path, "/health") {
			c.Next()
			return
		}

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
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

		// Parse and validate the Azure AD token
		claims, err := validateAzureADToken(tokenString, tenantID, applicationID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": fmt.Sprintf("Token validation failed: %v", err),
				},
			})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.ObjectID)
		c.Set("user_email", getEmailFromClaims(claims))
		c.Set("user_name", claims.Name)
		c.Set("tenant_id", claims.TenantID)
		c.Set("user_roles", mapAzureRolesToAppRoles(claims.Roles))
		c.Set("user_groups", claims.Groups)

		c.Next()
	}
}

// validateAzureADToken validates an Azure AD JWT token
func validateAzureADToken(tokenString, tenantID, applicationID string) (*AzureADClaims, error) {
	// Parse the token without verification first to get the header
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &AzureADClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Get the key ID from the token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("token missing kid claim")
	}

	// Get the public key from Azure AD JWKS endpoint
	publicKey, err := getAzureADPublicKey(tenantID, kid)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}

	// Parse and validate the token with the public key
	claims := &AzureADClaims{}
	validatedToken, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !validatedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Validate audience and issuer
	expectedIssuer := fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenantID)
	if claims.Issuer != expectedIssuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", expectedIssuer, claims.Issuer)
	}

	// Validate audience (can be application ID or api:// URI)
	validAudience := claims.Audience == applicationID ||
		claims.Audience == fmt.Sprintf("api://%s", applicationID) ||
		claims.AppID == applicationID

	if !validAudience {
		return nil, fmt.Errorf("invalid audience: %s", claims.Audience)
	}

	// Check if token is expired
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// getAzureADPublicKey retrieves the public key from Azure AD JWKS endpoint
func getAzureADPublicKey(tenantID, kid string) (*rsa.PublicKey, error) {
	// This is a simplified implementation
	// In production, you should cache the keys and handle rotation

	jwksURL := fmt.Sprintf("https://login.microsoftonline.com/%s/discovery/v2.0/keys", tenantID)

	resp, err := http.Get(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	var jwks JWKSResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	// Find the key with matching kid
	for _, key := range jwks.Keys {
		if key.Kid == kid {
			return parseRSAPublicKeyFromJWK(key)
		}
	}

	return nil, fmt.Errorf("key with kid %s not found", kid)
}

// parseRSAPublicKeyFromJWK parses RSA public key from JWK
func parseRSAPublicKeyFromJWK(key JSONWebKey) (*rsa.PublicKey, error) {
	// This is a simplified implementation
	// In production, use a proper JWK library like go-jose
	return nil, fmt.Errorf("JWK parsing not implemented - use a JWK library in production")
}

// getEmailFromClaims extracts email from claims with fallback
func getEmailFromClaims(claims *AzureADClaims) string {
	if claims.Email != "" {
		return claims.Email
	}
	return claims.PreferredUsername
}

// mapAzureRolesToAppRoles maps Azure AD roles to application roles
func mapAzureRolesToAppRoles(azureRoles []string) []string {
	roleMapping := map[string]string{
		"Global Administrator":      "super_admin",
		"User Administrator":        "admin",
		"Application Administrator": "admin",
		"User":                      "employee",
		"Guest":                     "guest",
	}

	var appRoles []string
	for _, role := range azureRoles {
		if mappedRole, exists := roleMapping[role]; exists {
			appRoles = append(appRoles, mappedRole)
		}
	}

	// Default role if no roles found
	if len(appRoles) == 0 {
		appRoles = append(appRoles, "employee")
	}

	return appRoles
}

// Development/Fallback Auth Middleware for when Azure AD is not available
func DevelopmentAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health check endpoints
		if strings.HasPrefix(c.Request.URL.Path, "/health") {
			c.Next()
			return
		}

		// In development, prefer X-User-ID header if provided (for testing with real IDs)
		// Otherwise use a valid dev UUID
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			userID = "00000000-0000-0000-0000-000000000001" // Valid UUID for dev
		}
		c.Set("user_id", userID)
		c.Set("user_email", "dev@example.com")
		c.Set("user_name", "Development User")
		// IMPORTANT: Only set tenant_id if not already set by TenantMiddleware
		// This preserves the X-Tenant-ID header value for proper multi-tenant isolation
		if _, exists := c.Get("tenant_id"); !exists {
			c.Set("tenant_id", "00000000-0000-0000-0000-000000000001")
		}
		c.Set("user_roles", []string{"admin", "employee"})
		c.Set("user_groups", []string{})

		c.Next()
	}
}
