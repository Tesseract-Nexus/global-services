package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware configures CORS for the auth service
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// List of allowed origins - in production, this should be configurable
		allowedOrigins := []string{
			"http://localhost:3010", // Admin Dashboard
			"http://localhost:3020", // Tenant Onboarding
			"http://localhost:3030", // Storefront
			"http://localhost:3031", // Categories MFE
			"http://localhost:3032", // Products MFE
			"http://localhost:3033", // Orders MFE
			"http://localhost:3034", // Vendors MFE
			"http://localhost:80",   // Nginx proxy
		}

		// Check if origin is allowed
		isAllowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				isAllowed = true
				break
			}
		}

		// Set CORS headers
		if isAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Tenant-ID")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// TenantResolver interface for resolving tenant identifiers
type TenantResolver interface {
	ResolveTenantIdentifier(ctx context.Context, identifier string) (string, error)
}

// TenantMiddleware extracts tenant ID from context (set by Istio auth middleware)
// Accepts either UUID or slug
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get tenant ID from context (set by Istio auth middleware)
		tenantIDVal, _ := c.Get("tenant_id")
		tenantID := ""
		if tenantIDVal != nil {
			tenantID = tenantIDVal.(string)
		}

		// If found, also set as request_tenant_id for backward compatibility
		if tenantID != "" {
			c.Set("request_tenant_id", tenantID)
		}

		c.Next()
	}
}

// TenantMiddlewareWithResolver extracts tenant ID from context and resolves slugs to UUIDs
// This is the production-ready version that accepts either UUID or slug
func TenantMiddlewareWithResolver(resolver TenantResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get tenant identifier from context (set by Istio auth middleware)
		tenantIDVal, _ := c.Get("tenant_id")
		tenantIdentifier := ""
		if tenantIDVal != nil {
			tenantIdentifier = tenantIDVal.(string)
		}

		// If found, resolve the identifier to a UUID (handles both UUID and slug)
		if tenantIdentifier != "" {
			tenantID, err := resolver.ResolveTenantIdentifier(c.Request.Context(), tenantIdentifier)
			if err != nil {
				log.Printf("[TenantMiddleware] Failed to resolve tenant '%s': %v", tenantIdentifier, err)
				// Still set the original value - downstream handlers can decide what to do
				c.Set("request_tenant_id", tenantIdentifier)
				c.Set("tenant_resolve_error", err.Error())
			} else {
				c.Set("request_tenant_id", tenantID)
				// Also store the original identifier for logging
				if tenantID != tenantIdentifier {
					log.Printf("[TenantMiddleware] Resolved tenant slug '%s' to ID '%s'", tenantIdentifier, tenantID)
					c.Set("request_tenant_slug", tenantIdentifier)
				}
			}
		}

		c.Next()
	}
}

// RequestLoggingMiddleware logs incoming requests
func RequestLoggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[AUTH-SERVICE] %s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format("02/Jan/2006:15:04:05 -0700"),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

// SecurityHeaders adds security headers to responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")

		c.Next()
	}
}

// RateLimitMiddleware implements basic rate limiting
func RateLimitMiddleware() gin.HandlerFunc {
	// In production, use Redis-based rate limiting
	// For now, this is a placeholder
	return func(c *gin.Context) {
		// TODO: Implement rate limiting logic
		c.Next()
	}
}
