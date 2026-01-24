package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// Auth BFF URL for ticket validation
var authBffURL = getAuthBffURL()

func getAuthBffURL() string {
	if url := os.Getenv("AUTH_BFF_URL"); url != "" {
		return url
	}
	// Default to marketplace namespace where auth-bff runs
	return "http://auth-bff.marketplace.svc.cluster.local:8080"
}

// TicketValidationResponse represents the response from auth-bff ticket validation
type TicketValidationResponse struct {
	Valid      bool   `json:"valid"`
	UserID     string `json:"user_id"`
	TenantID   string `json:"tenant_id"`
	TenantSlug string `json:"tenant_slug"`
	SessionID  string `json:"session_id"`
	Error      string `json:"error"`
}

// validateTicket calls auth-bff to validate a WebSocket ticket
func validateTicket(ticket string) (*TicketValidationResponse, error) {
	reqBody, _ := json.Marshal(map[string]string{"ticket": ticket})
	url := fmt.Sprintf("%s/internal/validate-ws-ticket", authBffURL)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call auth-bff: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp TicketValidationResponse
		json.Unmarshal(body, &errResp)
		return nil, fmt.Errorf("ticket validation failed: %s", errResp.Error)
	}

	var result TicketValidationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// NOTE: JWT decoding without verification has been removed for security.
// All authentication must come from either:
// 1. Istio x-jwt-claim-* headers (verified by Istio at ingress)
// 2. BFF ticket validation (verified by auth-bff)

// TenantAuth extracts tenant_id and user_id from TRUSTED sources only.
//
// SECURITY: This middleware only accepts identity from verified sources:
// 1. Istio x-jwt-claim-* headers (JWT verified by Istio at ingress)
// 2. BFF ticket validation (ticket verified by auth-bff)
//
// REMOVED for security (these were spoofable):
// - Direct X-Tenant-ID/X-User-ID headers from clients
// - Query parameters (tenant_id, user_id)
// - JWT token decoding without verification
func TenantAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tenantID, userID string

		// Priority 1: Check Istio JWT claim headers (TRUSTED - verified by Istio)
		// These headers are set by Istio after validating the JWT signature
		if istioUserID := c.GetHeader("x-jwt-claim-sub"); istioUserID != "" {
			userID = istioUserID
			tenantID = c.GetHeader("x-jwt-claim-tenant-id")
			if tenantID == "" {
				tenantID = c.GetHeader("x-jwt-claim-tenant_id")
			}

			if tenantID != "" && userID != "" {
				c.Set("tenant_id", tenantID)
				c.Set("user_id", userID)
				c.Next()
				return
			}
		}

		// Priority 2: Check for BFF ticket (TRUSTED - verified by auth-bff)
		// Used for WebSocket connections where headers can't be sent
		if ticket := c.Query("ticket"); ticket != "" {
			result, err := validateTicket(ticket)
			if err != nil {
				log.Printf("Ticket validation failed: %v", err)
			} else if result != nil && result.Valid {
				log.Printf("Ticket validated: userID=%s, tenantID=%s", result.UserID, result.TenantID)
				c.Set("tenant_id", result.TenantID)
				c.Set("user_id", result.UserID)
				c.Set("session_id", result.SessionID)
				c.Next()
				return
			}
		}

		// Priority 3: For internal service-to-service calls only
		// Check if request is from within the cluster (has mesh identity)
		// This allows internal services to set headers directly
		if meshPrincipal := c.GetHeader("x-forwarded-client-cert"); meshPrincipal != "" {
			// Request is from within the service mesh, use IstioAuth context keys
			tenantIDVal, _ := c.Get("tenant_id")
			if tenantIDVal != nil {
				tenantID = tenantIDVal.(string)
			}
			userIDVal, _ := c.Get("user_id")
			if userIDVal != nil {
				userID = userIDVal.(string)
			}

			if tenantID != "" && userID != "" {
				c.Set("tenant_id", tenantID)
				c.Set("user_id", userID)
				c.Next()
				return
			}
		}

		// No valid authentication found
		log.Printf("Auth failed: no valid authentication (path=%s)", c.Request.URL.Path)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required. Use valid JWT token or WebSocket ticket.",
		})
		c.Abort()
	}
}

// OptionalTenantAuth extracts tenant_id and user_id if present from TRUSTED sources.
// Does not require authentication but will set context if valid auth is present.
func OptionalTenantAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check Istio JWT claim headers (TRUSTED)
		if userID := c.GetHeader("x-jwt-claim-sub"); userID != "" {
			c.Set("user_id", userID)

			tenantID := c.GetHeader("x-jwt-claim-tenant-id")
			if tenantID == "" {
				tenantID = c.GetHeader("x-jwt-claim-tenant_id")
			}
			if tenantID != "" {
				c.Set("tenant_id", tenantID)
			}
		} else if meshPrincipal := c.GetHeader("x-forwarded-client-cert"); meshPrincipal != "" {
			// Internal service-to-service call - use IstioAuth context keys
			tenantIDVal, _ := c.Get("tenant_id")
			if tenantIDVal != nil {
				c.Set("tenant_id", tenantIDVal.(string))
			}
			userIDVal, _ := c.Get("user_id")
			if userIDVal != nil {
				c.Set("user_id", userIDVal.(string))
			}
		}

		c.Next()
	}
}

// Logger logs request details
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		log.Printf("[%d] %s %s %s %v",
			statusCode,
			method,
			path,
			clientIP,
			latency,
		)
	}
}

// CORS handles Cross-Origin Resource Sharing
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Tenant-ID, X-User-ID")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Recovery recovers from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// RateLimit implements basic rate limiting
// TODO: Use Redis-based rate limiting in production
func RateLimit(requestsPerSecond int) gin.HandlerFunc {
	// Simple token bucket implementation
	// For production, use a proper rate limiting library with Redis
	return func(c *gin.Context) {
		// For now, pass through
		c.Next()
	}
}
