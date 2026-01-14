package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tesseract-hub/domains/common/services/location-service/internal/models"
	"github.com/tesseract-hub/domains/common/services/location-service/internal/services"
)

// AddressHandler handles address-related HTTP requests
type AddressHandler struct {
	addressService *services.AddressService
}

// NewAddressHandler creates a new address handler
func NewAddressHandler(addressService *services.AddressService) *AddressHandler {
	return &AddressHandler{
		addressService: addressService,
	}
}

// AutocompleteRequest represents the request body for address autocomplete
type AutocompleteRequest struct {
	Input        string   `json:"input" binding:"required"`
	SessionToken string   `json:"session_token,omitempty"`
	Types        []string `json:"types,omitempty"`
	Components   string   `json:"components,omitempty"` // e.g., "country:us|country:ca"
	Language     string   `json:"language,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	Radius       int      `json:"radius,omitempty"`
}

// GeocodeRequest represents the request body for geocoding
type GeocodeRequest struct {
	Address string `json:"address" binding:"required"`
}

// ReverseGeocodeRequest represents the request body for reverse geocoding
type ReverseGeocodeRequest struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
}

// PlaceDetailsRequest represents the request body for place details
type PlaceDetailsRequest struct {
	PlaceID string `json:"place_id" binding:"required"`
}

// ValidateAddressRequest represents the request body for address validation
type ValidateAddressRequest struct {
	Address string `json:"address" binding:"required"`
}

// ManualAddressRequest represents a manually entered address
type ManualAddressRequest struct {
	StreetAddress string   `json:"street_address" binding:"required"`
	ApartmentUnit string   `json:"apartment_unit,omitempty"`
	City          string   `json:"city" binding:"required"`
	State         string   `json:"state" binding:"required"`
	PostalCode    string   `json:"postal_code" binding:"required"`
	Country       string   `json:"country" binding:"required"`
	Latitude      *float64 `json:"latitude,omitempty"`
	Longitude     *float64 `json:"longitude,omitempty"`
}

// ParseAddressRequest represents a request to parse an address string
type ParseAddressRequest struct {
	RawAddress string `json:"raw_address" binding:"required"`
}

// Autocomplete godoc
// @Summary Address autocomplete
// @Description Get address suggestions based on user input
// @Tags Address
// @Accept json
// @Produce json
// @Param input query string true "Search input"
// @Param session_token query string false "Session token for billing optimization"
// @Param types query string false "Address types (comma-separated: address,geocode,establishment)"
// @Param components query string false "Country restriction (e.g., country:us)"
// @Param language query string false "Response language"
// @Param latitude query number false "Bias latitude"
// @Param longitude query number false "Bias longitude"
// @Param radius query int false "Bias radius in meters"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/autocomplete [get]
func (h *AddressHandler) Autocomplete(c *gin.Context) {
	input := c.Query("input")
	if input == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Input is required",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_INPUT",
				"details": "The 'input' query parameter is required",
			},
		})
		return
	}

	opts := services.AutocompleteOptions{
		SessionToken: c.Query("session_token"),
		Components:   c.Query("components"),
		Language:     c.DefaultQuery("language", "en"),
	}

	// Parse types
	if types := c.Query("types"); types != "" {
		opts.Types = []string{types}
	}

	// Parse location bias
	if lat := c.Query("latitude"); lat != "" {
		if lng := c.Query("longitude"); lng != "" {
			latF, _ := strconv.ParseFloat(lat, 64)
			lngF, _ := strconv.ParseFloat(lng, 64)
			opts.Location = &models.GeoLocation{
				Latitude:  latF,
				Longitude: lngF,
			}
			if radius := c.Query("radius"); radius != "" {
				opts.Radius, _ = strconv.Atoi(radius)
			}
		}
	}

	suggestions, err := h.addressService.Autocomplete(c.Request.Context(), input, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to get address suggestions",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "AUTOCOMPLETE_FAILED",
				"details": err.Error(),
			},
			// Include fallback flag so frontend knows to show manual entry
			"allow_manual_entry": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Address suggestions retrieved successfully",
		"timestamp": time.Now(),
		"data": gin.H{
			"suggestions":        suggestions,
			"allow_manual_entry": true, // Always allow manual entry as fallback
		},
	})
}

// AutocompletePost godoc
// @Summary Address autocomplete (POST)
// @Description Get address suggestions based on user input (POST method)
// @Tags Address
// @Accept json
// @Produce json
// @Param request body AutocompleteRequest true "Autocomplete request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/autocomplete [post]
func (h *AddressHandler) AutocompletePost(c *gin.Context) {
	var req AutocompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid request body",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"details": err.Error(),
			},
			"allow_manual_entry": true,
		})
		return
	}

	opts := services.AutocompleteOptions{
		SessionToken: req.SessionToken,
		Types:        req.Types,
		Components:   req.Components,
		Language:     req.Language,
		Radius:       req.Radius,
	}

	if req.Latitude != nil && req.Longitude != nil {
		opts.Location = &models.GeoLocation{
			Latitude:  *req.Latitude,
			Longitude: *req.Longitude,
		}
	}

	suggestions, err := h.addressService.Autocomplete(c.Request.Context(), req.Input, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to get address suggestions",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "AUTOCOMPLETE_FAILED",
				"details": err.Error(),
			},
			"allow_manual_entry": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Address suggestions retrieved successfully",
		"timestamp": time.Now(),
		"data": gin.H{
			"suggestions":        suggestions,
			"allow_manual_entry": true,
		},
	})
}

// Geocode godoc
// @Summary Geocode an address
// @Description Convert an address to geographic coordinates
// @Tags Address
// @Accept json
// @Produce json
// @Param address query string true "Address to geocode"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/address/geocode [get]
func (h *AddressHandler) Geocode(c *gin.Context) {
	address := c.Query("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Address is required",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_INPUT",
				"details": "The 'address' query parameter is required",
			},
		})
		return
	}

	result, err := h.addressService.Geocode(c.Request.Context(), address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to geocode address",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "GEOCODE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "Address not found",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "ADDRESS_NOT_FOUND",
				"details": "Could not find coordinates for the provided address",
			},
			"allow_manual_entry": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Address geocoded successfully",
		"timestamp": time.Now(),
		"data":      result,
	})
}

// GeocodePost godoc
// @Summary Geocode an address (POST)
// @Description Convert an address to geographic coordinates (POST method)
// @Tags Address
// @Accept json
// @Produce json
// @Param request body GeocodeRequest true "Geocode request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/geocode [post]
func (h *AddressHandler) GeocodePost(c *gin.Context) {
	var req GeocodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid request body",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"details": err.Error(),
			},
		})
		return
	}

	result, err := h.addressService.Geocode(c.Request.Context(), req.Address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to geocode address",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "GEOCODE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "Address not found",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "ADDRESS_NOT_FOUND",
				"details": "Could not find coordinates for the provided address",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Address geocoded successfully",
		"timestamp": time.Now(),
		"data":      result,
	})
}

// ReverseGeocode godoc
// @Summary Reverse geocode coordinates
// @Description Convert geographic coordinates to an address
// @Tags Address
// @Accept json
// @Produce json
// @Param latitude query number true "Latitude"
// @Param longitude query number true "Longitude"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/reverse-geocode [get]
func (h *AddressHandler) ReverseGeocode(c *gin.Context) {
	latStr := c.Query("latitude")
	lngStr := c.Query("longitude")

	if latStr == "" || lngStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Latitude and longitude are required",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_INPUT",
				"details": "Both 'latitude' and 'longitude' query parameters are required",
			},
		})
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid latitude",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_LATITUDE",
				"details": "Latitude must be a valid number",
			},
		})
		return
	}

	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid longitude",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_LONGITUDE",
				"details": "Longitude must be a valid number",
			},
		})
		return
	}

	result, err := h.addressService.ReverseGeocode(c.Request.Context(), lat, lng)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to reverse geocode coordinates",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "REVERSE_GEOCODE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "No address found for coordinates",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "ADDRESS_NOT_FOUND",
				"details": "Could not find an address for the provided coordinates",
			},
			"allow_manual_entry": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Reverse geocoding successful",
		"timestamp": time.Now(),
		"data":      result,
	})
}

// ReverseGeocodePost godoc
// @Summary Reverse geocode coordinates (POST)
// @Description Convert geographic coordinates to an address (POST method)
// @Tags Address
// @Accept json
// @Produce json
// @Param request body ReverseGeocodeRequest true "Reverse geocode request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/reverse-geocode [post]
func (h *AddressHandler) ReverseGeocodePost(c *gin.Context) {
	var req ReverseGeocodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid request body",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"details": err.Error(),
			},
		})
		return
	}

	result, err := h.addressService.ReverseGeocode(c.Request.Context(), req.Latitude, req.Longitude)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to reverse geocode coordinates",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "REVERSE_GEOCODE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "No address found for coordinates",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "ADDRESS_NOT_FOUND",
				"details": "Could not find an address for the provided coordinates",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Reverse geocoding successful",
		"timestamp": time.Now(),
		"data":      result,
	})
}

// GetPlaceDetails godoc
// @Summary Get place details
// @Description Get detailed information about a place by its ID
// @Tags Address
// @Accept json
// @Produce json
// @Param place_id query string true "Place ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/place-details [get]
func (h *AddressHandler) GetPlaceDetails(c *gin.Context) {
	placeID := c.Query("place_id")
	if placeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Place ID is required",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_INPUT",
				"details": "The 'place_id' query parameter is required",
			},
		})
		return
	}

	result, err := h.addressService.GetPlaceDetails(c.Request.Context(), placeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to get place details",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "PLACE_DETAILS_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "Place not found",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "PLACE_NOT_FOUND",
				"details": "Could not find details for the provided place ID",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Place details retrieved successfully",
		"timestamp": time.Now(),
		"data":      result,
	})
}

// ValidateAddress godoc
// @Summary Validate an address
// @Description Validate and standardize an address
// @Tags Address
// @Accept json
// @Produce json
// @Param address query string true "Address to validate"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/validate [get]
func (h *AddressHandler) ValidateAddress(c *gin.Context) {
	address := c.Query("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Address is required",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_INPUT",
				"details": "The 'address' query parameter is required",
			},
		})
		return
	}

	result, err := h.addressService.ValidateAddress(c.Request.Context(), address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to validate address",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "VALIDATION_FAILED",
				"details": err.Error(),
			},
			"allow_manual_entry": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Address validation completed",
		"timestamp": time.Now(),
		"data":      result,
	})
}

// ValidateAddressPost godoc
// @Summary Validate an address (POST)
// @Description Validate and standardize an address (POST method)
// @Tags Address
// @Accept json
// @Produce json
// @Param request body ValidateAddressRequest true "Validate address request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/validate [post]
func (h *AddressHandler) ValidateAddressPost(c *gin.Context) {
	var req ValidateAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid request body",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"details": err.Error(),
			},
		})
		return
	}

	result, err := h.addressService.ValidateAddress(c.Request.Context(), req.Address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to validate address",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "VALIDATION_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Address validation completed",
		"timestamp": time.Now(),
		"data":      result,
	})
}

// FormatManualAddress godoc
// @Summary Format a manually entered address
// @Description Parse and format a manually entered address into standardized components
// @Tags Address
// @Accept json
// @Produce json
// @Param request body ManualAddressRequest true "Manual address request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/format-manual [post]
func (h *AddressHandler) FormatManualAddress(c *gin.Context) {
	var req ManualAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid request body",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"details": err.Error(),
			},
		})
		return
	}

	// Build formatted address
	addressParts := []string{req.StreetAddress}
	if req.ApartmentUnit != "" {
		addressParts[0] = req.StreetAddress + ", " + req.ApartmentUnit
	}
	formattedAddress := addressParts[0] + ", " + req.City + ", " + req.State + " " + req.PostalCode + ", " + req.Country

	// Build address components
	components := []models.AddressComponent{
		{Type: "route", LongName: req.StreetAddress, ShortName: req.StreetAddress},
		{Type: "locality", LongName: req.City, ShortName: req.City},
		{Type: "administrative_area_level_1", LongName: req.State, ShortName: req.State},
		{Type: "postal_code", LongName: req.PostalCode, ShortName: req.PostalCode},
		{Type: "country", LongName: req.Country, ShortName: req.Country},
	}

	if req.ApartmentUnit != "" {
		components = append([]models.AddressComponent{
			{Type: "subpremise", LongName: req.ApartmentUnit, ShortName: req.ApartmentUnit},
		}, components...)
	}

	result := gin.H{
		"formatted_address": formattedAddress,
		"components":        components,
		"manual_entry":      true,
	}

	if req.Latitude != nil && req.Longitude != nil {
		result["location"] = models.GeoLocation{
			Latitude:  *req.Latitude,
			Longitude: *req.Longitude,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Manual address formatted successfully",
		"timestamp": time.Now(),
		"data":      result,
	})
}

// ParseAddress godoc
// @Summary Parse a raw address string
// @Description Attempt to parse a raw address string into components
// @Tags Address
// @Accept json
// @Produce json
// @Param request body ParseAddressRequest true "Parse address request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/address/parse [post]
func (h *AddressHandler) ParseAddress(c *gin.Context) {
	var req ParseAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid request body",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"details": err.Error(),
			},
		})
		return
	}

	// Try to geocode the address to parse it
	result, err := h.addressService.Geocode(c.Request.Context(), req.RawAddress)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   "Address parsing completed with suggestions",
			"timestamp": time.Now(),
			"data": gin.H{
				"parsed":             false,
				"raw_address":        req.RawAddress,
				"allow_manual_entry": true,
				"suggestion":         "We couldn't automatically parse this address. Please enter the address details manually.",
			},
		})
		return
	}

	if result == nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   "Address parsing completed",
			"timestamp": time.Now(),
			"data": gin.H{
				"parsed":             false,
				"raw_address":        req.RawAddress,
				"allow_manual_entry": true,
				"suggestion":         "We couldn't find this address. Please verify or enter the details manually.",
			},
		})
		return
	}

	// Extract parsed components
	parsed := gin.H{
		"parsed":            true,
		"formatted_address": result.FormattedAddress,
		"components":        result.Components,
		"location":          result.Location,
		"place_id":          result.PlaceID,
	}

	// Extract individual fields for convenience
	for _, comp := range result.Components {
		switch comp.Type {
		case "street_number":
			parsed["street_number"] = comp.LongName
		case "route":
			parsed["street_name"] = comp.LongName
		case "locality":
			parsed["city"] = comp.LongName
		case "administrative_area_level_1":
			parsed["state"] = comp.LongName
			parsed["state_code"] = comp.ShortName
		case "country":
			parsed["country"] = comp.LongName
			parsed["country_code"] = comp.ShortName
		case "postal_code":
			parsed["postal_code"] = comp.LongName
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Address parsed successfully",
		"timestamp": time.Now(),
		"data":      parsed,
	})
}
