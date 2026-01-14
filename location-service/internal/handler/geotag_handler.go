package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tesseract-hub/domains/common/services/location-service/internal/models"
	"github.com/tesseract-hub/domains/common/services/location-service/internal/services"
)

// GeoTagHandler handles GeoTag API requests
type GeoTagHandler struct {
	geotagSvc *services.GeoTagService
}

// NewGeoTagHandler creates a new GeoTag handler
func NewGeoTagHandler(geotagSvc *services.GeoTagService) *GeoTagHandler {
	return &GeoTagHandler{
		geotagSvc: geotagSvc,
	}
}

// RegisterRoutes registers all GeoTag API routes
func (h *GeoTagHandler) RegisterRoutes(router *gin.RouterGroup) {
	geotag := router.Group("/geotag")
	{
		// Core geocoding endpoints
		geotag.GET("/geocode", h.Geocode)
		geotag.GET("/reverse", h.ReverseGeocode)
		geotag.GET("/autocomplete", h.Autocomplete)

		// Places endpoints
		places := geotag.Group("/places")
		{
			places.GET("/search", h.SearchPlaces)
			places.GET("/nearby", h.FindNearby)
			places.GET("/:id", h.GetPlace)
			places.POST("/validate", h.ValidateAndStore)
			places.PUT("/:id/verify", h.SetPlaceVerified)
			places.DELETE("/:id", h.DeletePlace)
		}

		// Bulk operations
		bulk := geotag.Group("/bulk")
		{
			bulk.POST("/geocode", h.BulkGeocode)
		}

		// Cache management (admin)
		cache := geotag.Group("/cache")
		{
			cache.GET("/stats", h.GetCacheStats)
			cache.POST("/clear", h.ClearCache)
		}

		// Stats
		geotag.GET("/stats", h.GetStats)
	}
}

// Geocode handles forward geocoding requests
// GET /api/v1/geotag/geocode?address=123+Main+St
func (h *GeoTagHandler) Geocode(c *gin.Context) {
	address := c.Query("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "MISSING_PARAMETER",
				Message: "address parameter is required",
			},
		})
		return
	}

	result, cacheInfo, err := h.geotagSvc.Geocode(c.Request.Context(), address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "GEOCODE_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success:   true,
		Data:      result,
		CacheInfo: cacheInfo,
	})
}

// ReverseGeocode handles reverse geocoding requests
// GET /api/v1/geotag/reverse?lat=-33.8688&lng=151.2093
func (h *GeoTagHandler) ReverseGeocode(c *gin.Context) {
	latStr := c.Query("lat")
	lngStr := c.Query("lng")

	if latStr == "" || lngStr == "" {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "MISSING_PARAMETER",
				Message: "lat and lng parameters are required",
			},
		})
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "INVALID_PARAMETER",
				Message: "invalid latitude value",
			},
		})
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "INVALID_PARAMETER",
				Message: "invalid longitude value",
			},
		})
		return
	}

	result, cacheInfo, err := h.geotagSvc.ReverseGeocode(c.Request.Context(), lat, lng)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "REVERSE_GEOCODE_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success:   true,
		Data:      result,
		CacheInfo: cacheInfo,
	})
}

// Autocomplete handles address autocomplete requests
// GET /api/v1/geotag/autocomplete?input=123+Main&country=AU
func (h *GeoTagHandler) Autocomplete(c *gin.Context) {
	input := c.Query("input")
	if input == "" {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "MISSING_PARAMETER",
				Message: "input parameter is required",
			},
		})
		return
	}

	countryCode := c.Query("country")

	suggestions, cacheInfo, err := h.geotagSvc.Autocomplete(c.Request.Context(), input, countryCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "AUTOCOMPLETE_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success:   true,
		Data:      suggestions,
		CacheInfo: cacheInfo,
	})
}

// GetPlace retrieves a place by ID
// GET /api/v1/geotag/places/:id
func (h *GeoTagHandler) GetPlace(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "INVALID_ID",
				Message: "invalid place ID format",
			},
		})
		return
	}

	place, err := h.geotagSvc.GetPlace(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "NOT_FOUND",
				Message: "place not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data:    place.ToGeoTagResult(),
	})
}

// SearchPlaces searches for places
// GET /api/v1/geotag/places/search?q=sydney&country=AU&limit=20
func (h *GeoTagHandler) SearchPlaces(c *gin.Context) {
	filters := models.SearchFilters{
		Query:       c.Query("q"),
		CountryCode: c.Query("country"),
		City:        c.Query("city"),
		StateCode:   c.Query("state"),
		PostalCode:  c.Query("postal_code"),
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filters.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filters.Offset = offset
		}
	}

	if verifiedStr := c.Query("verified"); verifiedStr != "" {
		verified := verifiedStr == "true"
		filters.Verified = &verified
	}

	places, total, err := h.geotagSvc.SearchPlaces(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "SEARCH_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	// Convert to GeoTagResults
	results := make([]*models.GeoTagResult, len(places))
	for i, place := range places {
		results[i] = place.ToGeoTagResult()
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data:    results,
		Meta: &models.PaginationMeta{
			Total:  total,
			Limit:  filters.Limit,
			Offset: filters.Offset,
		},
	})
}

// FindNearby finds places near a location
// GET /api/v1/geotag/places/nearby?lat=-33.8688&lng=151.2093&radius=5
func (h *GeoTagHandler) FindNearby(c *gin.Context) {
	latStr := c.Query("lat")
	lngStr := c.Query("lng")
	radiusStr := c.DefaultQuery("radius", "5")
	limitStr := c.DefaultQuery("limit", "20")

	if latStr == "" || lngStr == "" {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "MISSING_PARAMETER",
				Message: "lat and lng parameters are required",
			},
		})
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "INVALID_PARAMETER",
				Message: "invalid latitude value",
			},
		})
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "INVALID_PARAMETER",
				Message: "invalid longitude value",
			},
		})
		return
	}

	radius, err := strconv.ParseFloat(radiusStr, 64)
	if err != nil || radius <= 0 || radius > 100 {
		radius = 5
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 20
	}

	query := models.NearbyQuery{
		Latitude:  lat,
		Longitude: lng,
		RadiusKm:  radius,
		Limit:     limit,
	}

	places, err := h.geotagSvc.FindNearby(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "NEARBY_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	// Convert to GeoTagResults
	results := make([]*models.GeoTagResult, len(places))
	for i, place := range places {
		results[i] = place.ToGeoTagResult()
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data:    results,
	})
}

// ValidateAndStore validates and stores an address
// POST /api/v1/geotag/places/validate
func (h *GeoTagHandler) ValidateAndStore(c *gin.Context) {
	var req models.ValidateAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	place, err := h.geotagSvc.ValidateAndStore(c.Request.Context(), req.Address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "VALIDATION_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data:    place.ToGeoTagResult(),
	})
}

// SetPlaceVerified marks a place as verified
// PUT /api/v1/geotag/places/:id/verify
func (h *GeoTagHandler) SetPlaceVerified(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "INVALID_ID",
				Message: "invalid place ID format",
			},
		})
		return
	}

	var req struct {
		Verified bool `json:"verified"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to verified = true if no body
		req.Verified = true
	}

	if err := h.geotagSvc.SetPlaceVerified(c.Request.Context(), id, req.Verified); err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "UPDATE_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":       id.String(),
			"verified": req.Verified,
		},
	})
}

// DeletePlace deletes a place
// DELETE /api/v1/geotag/places/:id
func (h *GeoTagHandler) DeletePlace(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "INVALID_ID",
				Message: "invalid place ID format",
			},
		})
		return
	}

	if err := h.geotagSvc.DeletePlace(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "DELETE_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":      id.String(),
			"deleted": true,
		},
	})
}

// BulkGeocode geocodes multiple addresses
// POST /api/v1/geotag/bulk/geocode
func (h *GeoTagHandler) BulkGeocode(c *gin.Context) {
	var req models.BulkGeocodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	results, err := h.geotagSvc.BulkGeocode(c.Request.Context(), req.Addresses)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "BULK_GEOCODE_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	// Calculate summary
	successCount := 0
	cachedCount := 0
	for _, r := range results {
		if r.Result != nil {
			successCount++
		}
		if r.Cached {
			cachedCount++
		}
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data: map[string]interface{}{
			"results": results,
			"summary": map[string]interface{}{
				"total":      len(req.Addresses),
				"successful": successCount,
				"failed":     len(req.Addresses) - successCount,
				"cached":     cachedCount,
			},
		},
	})
}

// GetCacheStats returns cache statistics
// GET /api/v1/geotag/cache/stats
func (h *GeoTagHandler) GetCacheStats(c *gin.Context) {
	stats, err := h.geotagSvc.GetCacheStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "STATS_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	// Add hit rate from metrics
	metrics := h.geotagSvc.GetCacheMetrics()
	hitRate := 0.0
	if metrics != nil {
		hitRate = metrics.GetHitRate()
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data: map[string]interface{}{
			"cache":   stats,
			"hitRate": hitRate,
		},
	})
}

// ClearCache clears expired cache entries
// POST /api/v1/geotag/cache/clear
func (h *GeoTagHandler) ClearCache(c *gin.Context) {
	var req struct {
		BatchSize int `json:"batch_size"`
	}
	c.ShouldBindJSON(&req)

	if req.BatchSize <= 0 {
		req.BatchSize = 1000
	}

	deleted, err := h.geotagSvc.ClearExpiredCache(c.Request.Context(), req.BatchSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "CLEAR_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data: map[string]interface{}{
			"deleted": deleted,
		},
	})
}

// GetStats returns combined cache and places statistics
// GET /api/v1/geotag/stats
func (h *GeoTagHandler) GetStats(c *gin.Context) {
	cacheStats, err := h.geotagSvc.GetCacheStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "STATS_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	placesStats, err := h.geotagSvc.GetPlacesStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.GeoTagAPIResponse{
			Success: false,
			Error: &models.APIError{
				Code:    "STATS_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	metrics := h.geotagSvc.GetCacheMetrics()
	hitRate := 0.0
	if metrics != nil {
		hitRate = metrics.GetHitRate()
	}

	c.JSON(http.StatusOK, models.GeoTagAPIResponse{
		Success: true,
		Data: map[string]interface{}{
			"cache":   cacheStats,
			"places":  placesStats,
			"hitRate": hitRate,
		},
	})
}
