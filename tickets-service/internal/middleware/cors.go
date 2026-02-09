package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS returns a CORS middleware with proper configuration
// Note: AllowAllOrigins and AllowCredentials cannot both be true per CORS spec
func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-Tenant-ID", "X-User-ID", "X-Storefront-ID"}
	// Cannot use AllowCredentials=true with AllowAllOrigins=true per CORS spec
	// Credentials should be sent via Authorization header instead of cookies
	config.AllowCredentials = false

	return cors.New(config)
}
