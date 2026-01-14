package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tesseract-hub/feature-flags-service/internal/clients"
	"github.com/tesseract-hub/feature-flags-service/internal/config"
	gosharedmw "github.com/tesseract-hub/go-shared/middleware"
)

// FeaturesHandler handles feature flag operations
type FeaturesHandler struct {
	client *clients.GrowthbookClient
	config *config.Config
}

// NewFeaturesHandler creates a new FeaturesHandler
func NewFeaturesHandler(client *clients.GrowthbookClient, cfg *config.Config) *FeaturesHandler {
	return &FeaturesHandler{
		client: client,
		config: cfg,
	}
}

// GetFeatures proxies features request to Growthbook
// @Summary Get features for SDK client
// @Description Proxy features request to Growthbook for SDK clients
// @Tags SDK
// @Produce json
// @Param clientKey path string true "Growthbook SDK client key"
// @Success 200 {object} map[string]interface{} "Feature flags configuration"
// @Failure 400 {object} map[string]interface{} "Client key required"
// @Failure 500 {object} map[string]interface{} "Error fetching features"
// @Router /features/{clientKey} [get]
func (h *FeaturesHandler) GetFeatures(c *gin.Context) {
	clientKey := c.Param("clientKey")
	if clientKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "CLIENT_KEY_REQUIRED",
				"message": "Client key is required",
			},
		})
		return
	}

	features, err := h.client.GetFeatures(clientKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "FETCH_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, features)
}

// ListFeatures lists all features for the tenant
// @Summary List all features
// @Description Get all feature flags for the authenticated tenant
// @Tags Features
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param client_key query string true "Growthbook SDK client key"
// @Success 200 {object} map[string]interface{} "List of features"
// @Failure 400 {object} map[string]interface{} "Missing required parameters"
// @Failure 500 {object} map[string]interface{} "Error fetching features"
// @Router /features [get]
func (h *FeaturesHandler) ListFeatures(c *gin.Context) {
	tenantID := gosharedmw.GetVendorID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TENANT_REQUIRED",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	// Get client key from query or use default
	clientKey := c.Query("client_key")
	if clientKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "CLIENT_KEY_REQUIRED",
				"message": "Client key is required. Get it from Growthbook SDK Connections.",
			},
		})
		return
	}

	features, err := h.client.GetFeatures(clientKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "FETCH_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    features,
	})
}

// EvaluateFeatureRequest represents a feature evaluation request
type EvaluateFeatureRequest struct {
	FeatureKey string                 `json:"feature_key" binding:"required"`
	ClientKey  string                 `json:"client_key" binding:"required"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// EvaluateFeature evaluates a feature for given attributes
// @Summary Evaluate a feature flag
// @Description Evaluate a specific feature flag with user attributes for targeting
// @Tags Features
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body EvaluateFeatureRequest true "Feature evaluation request"
// @Success 200 {object} map[string]interface{} "Feature evaluation result"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Evaluation error"
// @Router /features/evaluate [post]
func (h *FeaturesHandler) EvaluateFeature(c *gin.Context) {
	var req EvaluateFeatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	tenantID := gosharedmw.GetVendorID(c)
	tenantSlug := gosharedmw.GetTenantSlug(c)
	storefrontID := gosharedmw.GetStorefrontID(c)

	// Add tenant context to attributes
	attributes := req.Attributes
	if attributes == nil {
		attributes = make(map[string]interface{})
	}
	attributes["tenantId"] = tenantID
	if tenantSlug != "" {
		attributes["tenantSlug"] = tenantSlug
	}
	if storefrontID != "" {
		attributes["storefrontId"] = storefrontID
	}

	value, err := h.client.EvaluateFeature(req.ClientKey, req.FeatureKey, attributes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "EVALUATE_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"feature_key": req.FeatureKey,
			"value":       value,
			"tenant_id":   tenantID,
		},
	})
}

// EvaluateFeatureByKey evaluates a feature by key (GET)
// @Summary Evaluate a feature by key
// @Description Evaluate a specific feature flag by its key using query parameters
// @Tags Features
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param key path string true "Feature key"
// @Param client_key query string true "Growthbook SDK client key"
// @Success 200 {object} map[string]interface{} "Feature evaluation result"
// @Failure 400 {object} map[string]interface{} "Missing required parameters"
// @Failure 500 {object} map[string]interface{} "Evaluation error"
// @Router /features/evaluate/{key} [get]
func (h *FeaturesHandler) EvaluateFeatureByKey(c *gin.Context) {
	featureKey := c.Param("key")
	clientKey := c.Query("client_key")

	if featureKey == "" || clientKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "PARAMS_REQUIRED",
				"message": "Feature key and client_key are required",
			},
		})
		return
	}

	tenantID := gosharedmw.GetVendorID(c)
	attributes := map[string]interface{}{
		"tenantId": tenantID,
	}

	if tenantSlug := gosharedmw.GetTenantSlug(c); tenantSlug != "" {
		attributes["tenantSlug"] = tenantSlug
	}
	if storefrontID := gosharedmw.GetStorefrontID(c); storefrontID != "" {
		attributes["storefrontId"] = storefrontID
	}
	if userID := c.GetString("user_id"); userID != "" {
		attributes["userId"] = userID
	}

	value, err := h.client.EvaluateFeature(clientKey, featureKey, attributes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "EVALUATE_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"feature_key": featureKey,
			"value":       value,
			"tenant_id":   tenantID,
		},
	})
}

// BatchEvaluateRequest represents a batch evaluation request
type BatchEvaluateRequest struct {
	ClientKey   string                 `json:"client_key" binding:"required"`
	FeatureKeys []string               `json:"feature_keys" binding:"required"`
	Attributes  map[string]interface{} `json:"attributes,omitempty"`
}

// BatchEvaluate evaluates multiple features at once
// @Summary Batch evaluate features
// @Description Evaluate multiple feature flags in a single request
// @Tags Features
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body BatchEvaluateRequest true "Batch evaluation request"
// @Success 200 {object} map[string]interface{} "Batch evaluation results"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Router /features/evaluate/batch [post]
func (h *FeaturesHandler) BatchEvaluate(c *gin.Context) {
	var req BatchEvaluateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	tenantID := gosharedmw.GetVendorID(c)

	// Add tenant context
	attributes := req.Attributes
	if attributes == nil {
		attributes = make(map[string]interface{})
	}
	attributes["tenantId"] = tenantID

	results := make(map[string]interface{})
	for _, key := range req.FeatureKeys {
		value, err := h.client.EvaluateFeature(req.ClientKey, key, attributes)
		if err != nil {
			results[key] = gin.H{"error": err.Error()}
		} else {
			results[key] = value
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"features":  results,
			"tenant_id": tenantID,
		},
	})
}

// OverrideRequest represents a feature override request
type OverrideRequest struct {
	FeatureKey string      `json:"feature_key" binding:"required"`
	Value      interface{} `json:"value" binding:"required"`
}

// SetOverride sets a feature override (for testing)
// @Summary Set feature override
// @Description Set a local feature override for testing purposes
// @Tags Features
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body OverrideRequest true "Override request"
// @Success 200 {object} map[string]interface{} "Override set"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Router /features/override [post]
func (h *FeaturesHandler) SetOverride(c *gin.Context) {
	// TODO: Implement feature overrides
	// This would typically store overrides in a local cache or database
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Override set (not yet implemented)",
	})
}

// ClearOverride clears a feature override
// @Summary Clear feature override
// @Description Clear a local feature override
// @Tags Features
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param key path string true "Feature key to clear"
// @Success 200 {object} map[string]interface{} "Override cleared"
// @Router /features/override/{key} [delete]
func (h *FeaturesHandler) ClearOverride(c *gin.Context) {
	// TODO: Implement feature overrides
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Override cleared (not yet implemented)",
	})
}

// GetSDKConfig returns SDK configuration
// @Summary Get SDK configuration
// @Description Get Growthbook SDK configuration for the tenant
// @Tags SDK
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Success 200 {object} map[string]interface{} "SDK configuration"
// @Router /sdk/config [get]
func (h *FeaturesHandler) GetSDKConfig(c *gin.Context) {
	tenantID := gosharedmw.GetVendorID(c)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"api_host":    "https://dev-growthbook.tesserix.app",
			"tenant_id":   tenantID,
			"environment": h.config.Environment,
			"features": gin.H{
				"default_features": h.config.DefaultFeatures,
			},
		},
	})
}
