package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// DefaultValidProducts is the default list of valid product IDs
var DefaultValidProducts = map[string]bool{
	"marketplace": true,
	"bookkeeping": true,
	"hms":         true,
	"fanzone":     true,
	"homechef":    true,
	"global":      true,
}

// validProducts holds the runtime list of valid products (can be overridden via env)
var validProducts map[string]bool

func init() {
	// Initialize from environment or use defaults
	validProducts = make(map[string]bool)
	if envProducts := os.Getenv("VALID_PRODUCTS"); envProducts != "" {
		for _, p := range strings.Split(envProducts, ",") {
			validProducts[strings.TrimSpace(strings.ToLower(p))] = true
		}
	} else {
		validProducts = DefaultValidProducts
	}
}

// ProductMiddleware extracts and validates the X-Product-ID header
// This enables multi-product support where each product has isolated buckets
func ProductMiddleware(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip for health check endpoints
		if strings.HasPrefix(c.Request.URL.Path, "/health") {
			c.Next()
			return
		}

		productID := c.GetHeader("X-Product-ID")

		// Default to marketplace for backwards compatibility
		if productID == "" {
			productID = "marketplace"
			logger.Debug("No X-Product-ID header, defaulting to marketplace")
		}

		// Normalize product ID
		productID = strings.ToLower(strings.TrimSpace(productID))

		// Validate product ID
		if !validProducts[productID] {
			logger.WithField("product_id", productID).Warn("Invalid product ID")
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "INVALID_PRODUCT",
					"message": "Invalid X-Product-ID. Valid values: " + getValidProductsList(),
				},
			})
			c.Abort()
			return
		}

		// Set product ID in context for downstream handlers
		c.Set("product_id", productID)
		logger.WithField("product_id", productID).Debug("Product context set")
		c.Next()
	}
}

// GetProductID extracts product ID from gin context
func GetProductID(c *gin.Context) string {
	if productID, exists := c.Get("product_id"); exists {
		if product, ok := productID.(string); ok {
			return product
		}
	}
	return "marketplace" // Default fallback
}

// ValidateBucketAccess checks if the product can access the requested bucket
// Bucket naming convention: {product}-{env}-{type}-{region}
// Examples: marketplace-devtest-assets-au, bookkeeping-prod-public-in
func ValidateBucketAccess(productID, bucketName string) bool {
	if productID == "" || bucketName == "" {
		return false
	}
	// Bucket must start with product name followed by hyphen
	return strings.HasPrefix(bucketName, productID+"-")
}

// IsValidProduct checks if a product ID is valid
func IsValidProduct(productID string) bool {
	return validProducts[strings.ToLower(productID)]
}

// getValidProductsList returns a comma-separated list of valid products
func getValidProductsList() string {
	products := make([]string, 0, len(validProducts))
	for p := range validProducts {
		products = append(products, p)
	}
	return strings.Join(products, ", ")
}

// GetAllValidProducts returns all valid product IDs
func GetAllValidProducts() []string {
	products := make([]string, 0, len(validProducts))
	for p := range validProducts {
		products = append(products, p)
	}
	return products
}
