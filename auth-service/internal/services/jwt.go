package services

import (
	"fmt"
	"time"

	"auth-service/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTService struct {
	accessSecret      string
	refreshSecret     string
	accessExpiryTime  time.Duration
	refreshExpiryTime time.Duration
}

type Claims struct {
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	TenantID    string    `json:"tenant_id"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	SessionID   uuid.UUID `json:"session_id"`
	jwt.RegisteredClaims
}

func NewJWTService(accessSecret, refreshSecret string, accessExpiryHours, refreshExpiryDays int) *JWTService {
	return &JWTService{
		accessSecret:      accessSecret,
		refreshSecret:     refreshSecret,
		accessExpiryTime:  time.Duration(accessExpiryHours) * time.Hour,
		refreshExpiryTime: time.Duration(refreshExpiryDays) * 24 * time.Hour,
	}
}

// GenerateTokens generates both access and refresh tokens
func (j *JWTService) GenerateTokens(user *models.User, sessionID uuid.UUID) (string, string, error) {
	// Extract roles and permissions
	roles := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roles[i] = role.Name
	}

	permissions := make([]string, len(user.Permissions))
	for i, permission := range user.Permissions {
		permissions[i] = permission.Name
	}

	// Generate access token
	accessToken, err := j.generateAccessToken(user, roles, permissions, sessionID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := j.generateRefreshToken(user, sessionID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// generateAccessToken creates a JWT access token
func (j *JWTService) generateAccessToken(user *models.User, roles, permissions []string, sessionID uuid.UUID) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:      user.ID,
		Email:       user.Email,
		Name:        user.Name,
		TenantID:    user.TenantID.String(),
		Roles:       roles,
		Permissions: permissions,
		SessionID:   sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.accessExpiryTime)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "tesseract-hub-auth",
			Subject:   user.ID.String(),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.accessSecret))
}

// generateRefreshToken creates a JWT refresh token
func (j *JWTService) generateRefreshToken(user *models.User, sessionID uuid.UUID) (string, error) {
	now := time.Now()
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(j.refreshExpiryTime)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Issuer:    "tesseract-hub-auth",
		Subject:   user.ID.String(),
		ID:        sessionID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.refreshSecret))
}

// ValidateAccessToken validates and parses an access token
func (j *JWTService) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.accessSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// ValidateRefreshToken validates and parses a refresh token
func (j *JWTService) ValidateRefreshToken(tokenString string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.refreshSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse refresh token: %w", err)
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid refresh token")
}

// RefreshTokens generates new access and refresh tokens from a valid refresh token
func (j *JWTService) RefreshTokens(refreshToken string, authRepo AuthRepository) (string, string, error) {
	// Validate refresh token
	claims, err := j.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", "", fmt.Errorf("invalid refresh token: %w", err)
	}

	// Parse session ID from refresh token
	sessionID, err := uuid.Parse(claims.ID)
	if err != nil {
		return "", "", fmt.Errorf("invalid session ID in refresh token: %w", err)
	}

	// Get session from database
	session, err := authRepo.GetSession(sessionID)
	if err != nil {
		return "", "", fmt.Errorf("session not found: %w", err)
	}

	// Verify session is active and refresh token matches
	if !session.IsActive || session.RefreshToken != refreshToken {
		return "", "", fmt.Errorf("invalid or inactive session")
	}

	// Get user with roles and permissions
	user, err := authRepo.GetUserWithRolesAndPermissions(session.UserID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get user: %w", err)
	}

	// Generate new tokens
	newAccessToken, newRefreshToken, err := j.GenerateTokens(user, sessionID)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate new tokens: %w", err)
	}

	// Update session with new tokens
	session.AccessToken = newAccessToken
	session.RefreshToken = newRefreshToken
	session.ExpiresAt = time.Now().Add(j.refreshExpiryTime)
	session.UpdatedAt = time.Now()

	if err := authRepo.UpdateSession(session); err != nil {
		return "", "", fmt.Errorf("failed to update session: %w", err)
	}

	return newAccessToken, newRefreshToken, nil
}

// RevokeToken revokes a token by marking the session as inactive
func (j *JWTService) RevokeToken(tokenString string, authRepo AuthRepository) error {
	// Validate access token to get session ID
	claims, err := j.ValidateAccessToken(tokenString)
	if err != nil {
		return fmt.Errorf("invalid access token: %w", err)
	}

	// Deactivate session
	return authRepo.DeactivateSession(claims.SessionID)
}

// GetTokenExpiry returns the expiry time for access tokens
func (j *JWTService) GetTokenExpiry() time.Duration {
	return j.accessExpiryTime
}

// GetRefreshTokenExpiry returns the expiry time for refresh tokens
func (j *JWTService) GetRefreshTokenExpiry() time.Duration {
	return j.refreshExpiryTime
}

// Interface for auth repository (to avoid circular imports)
type AuthRepository interface {
	GetSession(sessionID uuid.UUID) (*models.Session, error)
	UpdateSession(session *models.Session) error
	DeactivateSession(sessionID uuid.UUID) error
	GetUserWithRolesAndPermissions(userID uuid.UUID) (*models.User, error)
}
