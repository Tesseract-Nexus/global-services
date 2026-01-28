package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"location-service/internal/models"
	"location-service/internal/services"
)

// LocationHandler handles location-related HTTP requests
type LocationHandler struct {
	locationService *services.LocationService
	geoService      *services.GeoLocationService
}

// NewLocationHandler creates a new location handler
func NewLocationHandler(locationService *services.LocationService, geoService *services.GeoLocationService) *LocationHandler {
	return &LocationHandler{
		locationService: locationService,
		geoService:      geoService,
	}
}

// DetectLocation godoc
// @Summary Detect location from IP
// @Description Detect user's location based on their IP address
// @Tags Location Detection
// @Accept json
// @Produce json
// @Param ip query string false "IP address to detect location for"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/location/detect [get]
func (h *LocationHandler) DetectLocation(c *gin.Context) {
	// Get IP from query parameter or extract from request
	ip := c.Query("ip")
	if ip == "" {
		ip = h.geoService.GetIPFromRequest(
			c.GetHeader("X-Forwarded-For"),
			c.GetHeader("X-Real-IP"),
			c.Request.RemoteAddr,
		)
	}

	locationData, err := h.geoService.DetectLocationFromIP(c.Request.Context(), ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to detect location",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "LOCATION_DETECTION_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Location detected successfully",
		"timestamp": time.Now(),
		"data":      locationData,
	})
}

// GetCountries godoc
// @Summary Get all countries
// @Description Retrieve a list of all supported countries
// @Tags Countries
// @Accept json
// @Produce json
// @Param search query string false "Search countries by name or code"
// @Param region query string false "Filter by region"
// @Param limit query int false "Limit number of results" default(50)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/countries [get]
func (h *LocationHandler) GetCountries(c *gin.Context) {
	search := c.Query("search")
	region := c.Query("region")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Validate pagination parameters
	// Countries max is 250 (there are ~250 countries worldwide)
	if limit < 1 {
		limit = 50
	} else if limit > 250 {
		limit = 250
	}
	if offset < 0 {
		offset = 0
	}

	countries, total, err := h.locationService.GetCountries(c.Request.Context(), search, region, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to retrieve countries",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "COUNTRIES_RETRIEVAL_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	hasNext := int64(offset+limit) < total
	hasPrevious := offset > 0

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Countries retrieved successfully",
		"timestamp": time.Now(),
		"data":      countries,
		"pagination": gin.H{
			"total":        total,
			"limit":        limit,
			"offset":       offset,
			"has_next":     hasNext,
			"has_previous": hasPrevious,
		},
	})
}

// GetCountry godoc
// @Summary Get country by ID
// @Description Retrieve detailed information about a specific country
// @Tags Countries
// @Accept json
// @Produce json
// @Param countryId path string true "Country ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/countries/{countryId} [get]
func (h *LocationHandler) GetCountry(c *gin.Context) {
	countryID := c.Param("countryId")

	country, err := h.locationService.GetCountryByID(c.Request.Context(), countryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "Country not found",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "COUNTRY_NOT_FOUND",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Country retrieved successfully",
		"timestamp": time.Now(),
		"data":      country,
	})
}

// GetStates godoc
// @Summary Get states for a country
// @Description Retrieve all states/provinces for a specific country
// @Tags States
// @Accept json
// @Produce json
// @Param countryId path string true "Country ID"
// @Param search query string false "Search states by name or code"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/countries/{countryId}/states [get]
func (h *LocationHandler) GetStates(c *gin.Context) {
	countryID := c.Param("countryId")
	search := c.Query("search")

	states, err := h.locationService.GetStatesByCountryID(c.Request.Context(), countryID, search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to retrieve states",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "STATES_RETRIEVAL_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "States retrieved successfully",
		"timestamp": time.Now(),
		"data":      states,
	})
}

// GetAllStates godoc
// @Summary Get all states
// @Description Retrieve all states/provinces across all countries
// @Tags States
// @Accept json
// @Produce json
// @Param search query string false "Search states by name or code"
// @Param country_id query string false "Filter by country ID"
// @Param limit query int false "Limit number of results" default(50)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/states [get]
func (h *LocationHandler) GetAllStates(c *gin.Context) {
	search := c.Query("search")
	countryID := c.Query("country_id")

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Validate pagination parameters
	// States max is 500 (to accommodate countries with many subdivisions)
	if limit < 1 {
		limit = 50
	} else if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	states, total, err := h.locationService.GetStates(c.Request.Context(), search, countryID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to retrieve states",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "STATES_RETRIEVAL_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	hasNext := int64(offset+limit) < total
	hasPrevious := offset > 0

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "States retrieved successfully",
		"timestamp": time.Now(),
		"data":      states,
		"pagination": gin.H{
			"total":        total,
			"limit":        limit,
			"offset":       offset,
			"has_next":     hasNext,
			"has_previous": hasPrevious,
		},
	})
}

// GetState godoc
// @Summary Get state by ID
// @Description Retrieve detailed information about a specific state
// @Tags States
// @Accept json
// @Produce json
// @Param stateId path string true "State ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/states/{stateId} [get]
func (h *LocationHandler) GetState(c *gin.Context) {
	stateID := c.Param("stateId")

	state, err := h.locationService.GetStateByID(c.Request.Context(), stateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "State not found",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "STATE_NOT_FOUND",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "State retrieved successfully",
		"timestamp": time.Now(),
		"data":      state,
	})
}

// GetCurrencies godoc
// @Summary Get all currencies
// @Description Retrieve a list of all supported currencies
// @Tags Currencies
// @Accept json
// @Produce json
// @Param search query string false "Search currencies by name or code"
// @Param active_only query bool false "Return only active currencies" default(true)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/currencies [get]
func (h *LocationHandler) GetCurrencies(c *gin.Context) {
	search := c.Query("search")
	activeOnly, _ := strconv.ParseBool(c.DefaultQuery("active_only", "true"))

	currencies, err := h.locationService.GetCurrencies(c.Request.Context(), search, activeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to retrieve currencies",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "CURRENCIES_RETRIEVAL_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Currencies retrieved successfully",
		"timestamp": time.Now(),
		"data":      currencies,
	})
}

// GetCurrency godoc
// @Summary Get currency by code
// @Description Retrieve detailed information about a specific currency
// @Tags Currencies
// @Accept json
// @Produce json
// @Param currencyCode path string true "Currency code"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/currencies/{currencyCode} [get]
func (h *LocationHandler) GetCurrency(c *gin.Context) {
	currencyCode := c.Param("currencyCode")

	currency, err := h.locationService.GetCurrencyByCode(c.Request.Context(), currencyCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to retrieve currency",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "CURRENCY_RETRIEVAL_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	if currency == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "Currency not found",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "CURRENCY_NOT_FOUND",
				"details": "Currency with the specified code was not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Currency retrieved successfully",
		"timestamp": time.Now(),
		"data":      currency,
	})
}

// GetTimezones godoc
// @Summary Get all timezones
// @Description Retrieve a list of all supported timezones
// @Tags Timezones
// @Accept json
// @Produce json
// @Param search query string false "Search timezones by name"
// @Param country_id query string false "Filter by country ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/timezones [get]
func (h *LocationHandler) GetTimezones(c *gin.Context) {
	search := c.Query("search")
	countryID := c.Query("country_id")

	timezones, err := h.locationService.GetTimezones(c.Request.Context(), search, countryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to retrieve timezones",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "TIMEZONES_RETRIEVAL_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Timezones retrieved successfully",
		"timestamp": time.Now(),
		"data":      timezones,
	})
}

// GetTimezone godoc
// @Summary Get timezone details
// @Description Retrieve detailed information about a specific timezone
// @Tags Timezones
// @Accept json
// @Produce json
// @Param timezone path string true "Timezone identifier"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/timezones/{timezone} [get]
func (h *LocationHandler) GetTimezone(c *gin.Context) {
	timezoneID := c.Param("timezone")

	timezone, err := h.locationService.GetTimezoneByID(c.Request.Context(), timezoneID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to retrieve timezone",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "TIMEZONE_RETRIEVAL_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	if timezone == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "Timezone not found",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "TIMEZONE_NOT_FOUND",
				"details": "Timezone with the specified identifier was not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Timezone retrieved successfully",
		"timestamp": time.Now(),
		"data":      timezone,
	})
}

// ==================== ADMIN CRUD ENDPOINTS ====================

// CreateCountry godoc
// @Summary Create a new country
// @Description Create a new country in the database
// @Tags Admin - Countries
// @Accept json
// @Produce json
// @Param country body models.Country true "Country data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/admin/countries [post]
func (h *LocationHandler) CreateCountry(c *gin.Context) {
	var country models.Country
	if err := c.ShouldBindJSON(&country); err != nil {
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

	if err := h.locationService.CreateCountry(c.Request.Context(), &country); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to create country",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "COUNTRY_CREATE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"message":   "Country created successfully",
		"timestamp": time.Now(),
		"data":      country,
	})
}

// UpdateCountry godoc
// @Summary Update a country
// @Description Update an existing country
// @Tags Admin - Countries
// @Accept json
// @Produce json
// @Param countryId path string true "Country ID"
// @Param country body models.Country true "Country data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/admin/countries/{countryId} [put]
func (h *LocationHandler) UpdateCountry(c *gin.Context) {
	countryID := c.Param("countryId")

	var country models.Country
	if err := c.ShouldBindJSON(&country); err != nil {
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

	country.ID = countryID

	if err := h.locationService.UpdateCountry(c.Request.Context(), &country); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to update country",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "COUNTRY_UPDATE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Country updated successfully",
		"timestamp": time.Now(),
		"data":      country,
	})
}

// DeleteCountry godoc
// @Summary Delete a country
// @Description Soft delete a country
// @Tags Admin - Countries
// @Accept json
// @Produce json
// @Param countryId path string true "Country ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/admin/countries/{countryId} [delete]
func (h *LocationHandler) DeleteCountry(c *gin.Context) {
	countryID := c.Param("countryId")

	if err := h.locationService.DeleteCountry(c.Request.Context(), countryID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to delete country",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "COUNTRY_DELETE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Country deleted successfully",
		"timestamp": time.Now(),
	})
}

// CreateState godoc
// @Summary Create a new state
// @Description Create a new state/province in the database
// @Tags Admin - States
// @Accept json
// @Produce json
// @Param state body models.State true "State data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/admin/states [post]
func (h *LocationHandler) CreateState(c *gin.Context) {
	var state models.State
	if err := c.ShouldBindJSON(&state); err != nil {
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

	if err := h.locationService.CreateState(c.Request.Context(), &state); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to create state",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "STATE_CREATE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"message":   "State created successfully",
		"timestamp": time.Now(),
		"data":      state,
	})
}

// UpdateState godoc
// @Summary Update a state
// @Description Update an existing state/province
// @Tags Admin - States
// @Accept json
// @Produce json
// @Param stateId path string true "State ID"
// @Param state body models.State true "State data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/admin/states/{stateId} [put]
func (h *LocationHandler) UpdateState(c *gin.Context) {
	stateID := c.Param("stateId")

	var state models.State
	if err := c.ShouldBindJSON(&state); err != nil {
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

	state.ID = stateID

	if err := h.locationService.UpdateState(c.Request.Context(), &state); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to update state",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "STATE_UPDATE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "State updated successfully",
		"timestamp": time.Now(),
		"data":      state,
	})
}

// DeleteState godoc
// @Summary Delete a state
// @Description Soft delete a state/province
// @Tags Admin - States
// @Accept json
// @Produce json
// @Param stateId path string true "State ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/admin/states/{stateId} [delete]
func (h *LocationHandler) DeleteState(c *gin.Context) {
	stateID := c.Param("stateId")

	if err := h.locationService.DeleteState(c.Request.Context(), stateID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to delete state",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "STATE_DELETE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "State deleted successfully",
		"timestamp": time.Now(),
	})
}

// CreateCurrency godoc
// @Summary Create a new currency
// @Description Create a new currency in the database
// @Tags Admin - Currencies
// @Accept json
// @Produce json
// @Param currency body models.Currency true "Currency data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/admin/currencies [post]
func (h *LocationHandler) CreateCurrency(c *gin.Context) {
	var currency models.Currency
	if err := c.ShouldBindJSON(&currency); err != nil {
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

	if err := h.locationService.CreateCurrency(c.Request.Context(), &currency); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to create currency",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "CURRENCY_CREATE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"message":   "Currency created successfully",
		"timestamp": time.Now(),
		"data":      currency,
	})
}

// UpdateCurrency godoc
// @Summary Update a currency
// @Description Update an existing currency
// @Tags Admin - Currencies
// @Accept json
// @Produce json
// @Param currencyCode path string true "Currency Code"
// @Param currency body models.Currency true "Currency data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/admin/currencies/{currencyCode} [put]
func (h *LocationHandler) UpdateCurrency(c *gin.Context) {
	currencyCode := c.Param("currencyCode")

	var currency models.Currency
	if err := c.ShouldBindJSON(&currency); err != nil {
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

	currency.Code = currencyCode

	if err := h.locationService.UpdateCurrency(c.Request.Context(), &currency); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to update currency",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "CURRENCY_UPDATE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Currency updated successfully",
		"timestamp": time.Now(),
		"data":      currency,
	})
}

// DeleteCurrency godoc
// @Summary Delete a currency
// @Description Soft delete a currency
// @Tags Admin - Currencies
// @Accept json
// @Produce json
// @Param currencyCode path string true "Currency Code"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/admin/currencies/{currencyCode} [delete]
func (h *LocationHandler) DeleteCurrency(c *gin.Context) {
	currencyCode := c.Param("currencyCode")

	if err := h.locationService.DeleteCurrency(c.Request.Context(), currencyCode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to delete currency",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "CURRENCY_DELETE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Currency deleted successfully",
		"timestamp": time.Now(),
	})
}

// CreateTimezone godoc
// @Summary Create a new timezone
// @Description Create a new timezone in the database
// @Tags Admin - Timezones
// @Accept json
// @Produce json
// @Param timezone body models.Timezone true "Timezone data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/admin/timezones [post]
func (h *LocationHandler) CreateTimezone(c *gin.Context) {
	var timezone models.Timezone
	if err := c.ShouldBindJSON(&timezone); err != nil {
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

	if err := h.locationService.CreateTimezone(c.Request.Context(), &timezone); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to create timezone",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "TIMEZONE_CREATE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"message":   "Timezone created successfully",
		"timestamp": time.Now(),
		"data":      timezone,
	})
}

// UpdateTimezone godoc
// @Summary Update a timezone
// @Description Update an existing timezone
// @Tags Admin - Timezones
// @Accept json
// @Produce json
// @Param timezoneId path string true "Timezone ID"
// @Param timezone body models.Timezone true "Timezone data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/admin/timezones/{timezoneId} [put]
func (h *LocationHandler) UpdateTimezone(c *gin.Context) {
	timezoneID := c.Param("timezoneId")

	var timezone models.Timezone
	if err := c.ShouldBindJSON(&timezone); err != nil {
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

	timezone.ID = timezoneID

	if err := h.locationService.UpdateTimezone(c.Request.Context(), &timezone); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to update timezone",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "TIMEZONE_UPDATE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Timezone updated successfully",
		"timestamp": time.Now(),
		"data":      timezone,
	})
}

// DeleteTimezone godoc
// @Summary Delete a timezone
// @Description Soft delete a timezone
// @Tags Admin - Timezones
// @Accept json
// @Produce json
// @Param timezoneId path string true "Timezone ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/admin/timezones/{timezoneId} [delete]
func (h *LocationHandler) DeleteTimezone(c *gin.Context) {
	timezoneID := c.Param("timezoneId")

	if err := h.locationService.DeleteTimezone(c.Request.Context(), timezoneID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to delete timezone",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "TIMEZONE_DELETE_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Timezone deleted successfully",
		"timestamp": time.Now(),
	})
}

// GetCacheStats godoc
// @Summary Get cache statistics
// @Description Retrieve location cache statistics
// @Tags Admin - Cache
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/admin/cache/stats [get]
func (h *LocationHandler) GetCacheStats(c *gin.Context) {
	stats, err := h.locationService.GetCacheStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to retrieve cache stats",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "CACHE_STATS_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Cache statistics retrieved successfully",
		"timestamp": time.Now(),
		"data":      stats,
	})
}

// CleanupCache godoc
// @Summary Cleanup expired cache entries
// @Description Remove all expired entries from the location cache
// @Tags Admin - Cache
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/admin/cache/cleanup [post]
func (h *LocationHandler) CleanupCache(c *gin.Context) {
	deleted, err := h.locationService.CleanupExpiredCache(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to cleanup cache",
			"timestamp": time.Now(),
			"error": gin.H{
				"code":    "CACHE_CLEANUP_FAILED",
				"details": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Cache cleanup completed successfully",
		"timestamp": time.Now(),
		"data": gin.H{
			"deleted_entries": deleted,
		},
	})
}
