package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"settings-service/internal/models"
	"settings-service/internal/services"
	"settings-service/internal/workers"
)

// CurrencyHandler handles currency-related HTTP requests
type CurrencyHandler struct {
	currencyService services.CurrencyService
	rateUpdater     *workers.RateUpdater
}

// NewCurrencyHandler creates a new currency handler
func NewCurrencyHandler(currencyService services.CurrencyService, rateUpdater *workers.RateUpdater) *CurrencyHandler {
	return &CurrencyHandler{
		currencyService: currencyService,
		rateUpdater:     rateUpdater,
	}
}

// Convert handles currency conversion requests
// @Summary Convert currency
// @Description Convert an amount from one currency to another
// @Tags currency
// @Produce json
// @Param amount query number true "Amount to convert"
// @Param from query string true "Source currency code (ISO 4217)"
// @Param to query string true "Target currency code (ISO 4217)"
// @Success 200 {object} models.CurrencyConvertResponse
// @Failure 400 {object} models.CurrencyConvertResponse
// @Failure 500 {object} models.CurrencyConvertResponse
// @Router /api/v1/currency/convert [get]
func (h *CurrencyHandler) Convert(c *gin.Context) {
	amountStr := c.Query("amount")
	from := strings.ToUpper(c.Query("from"))
	to := strings.ToUpper(c.Query("to"))

	if amountStr == "" {
		c.JSON(http.StatusBadRequest, models.CurrencyConvertResponse{
			Success: false,
			Message: "amount parameter is required",
		})
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.CurrencyConvertResponse{
			Success: false,
			Message: "invalid amount format",
		})
		return
	}

	if from == "" || len(from) != 3 {
		c.JSON(http.StatusBadRequest, models.CurrencyConvertResponse{
			Success: false,
			Message: "from parameter must be a 3-character ISO 4217 currency code",
		})
		return
	}

	if to == "" || len(to) != 3 {
		c.JSON(http.StatusBadRequest, models.CurrencyConvertResponse{
			Success: false,
			Message: "to parameter must be a 3-character ISO 4217 currency code",
		})
		return
	}

	convertedAmount, err := h.currencyService.Convert(amount, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.CurrencyConvertResponse{
			Success: false,
			Message: "Failed to convert currency: " + err.Error(),
		})
		return
	}

	rate, _ := h.currencyService.GetRate(from, to)
	rateDate, _ := h.currencyService.GetRateDate()

	c.JSON(http.StatusOK, models.CurrencyConvertResponse{
		Success:         true,
		OriginalAmount:  amount,
		ConvertedAmount: convertedAmount,
		FromCurrency:    from,
		ToCurrency:      to,
		Rate:            rate,
		RateDate:        rateDate,
	})
}

// BulkConvert handles bulk currency conversion requests
// @Summary Bulk convert currencies
// @Description Convert multiple amounts to a single target currency
// @Tags currency
// @Accept json
// @Produce json
// @Param request body models.BulkConvertRequest true "Bulk conversion request"
// @Success 200 {object} models.BulkConvertResponse
// @Failure 400 {object} models.BulkConvertResponse
// @Failure 500 {object} models.BulkConvertResponse
// @Router /api/v1/currency/bulk-convert [post]
func (h *CurrencyHandler) BulkConvert(c *gin.Context) {
	var req models.BulkConvertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.BulkConvertResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	if len(req.Amounts) == 0 {
		c.JSON(http.StatusBadRequest, models.BulkConvertResponse{
			Success: false,
			Message: "amounts array is required and cannot be empty",
		})
		return
	}

	response, err := h.currencyService.BulkConvert(req.Amounts, req.To)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.BulkConvertResponse{
			Success: false,
			Message: "Failed to convert currencies: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetRates returns exchange rates for a base currency
// @Summary Get exchange rates
// @Description Get all exchange rates for a specified base currency
// @Tags currency
// @Produce json
// @Param base query string false "Base currency code (default: EUR)"
// @Success 200 {object} models.CurrencyRateResponse
// @Failure 400 {object} models.CurrencyRateResponse
// @Failure 500 {object} models.CurrencyRateResponse
// @Router /api/v1/currency/rates [get]
func (h *CurrencyHandler) GetRates(c *gin.Context) {
	baseCurrency := strings.ToUpper(c.DefaultQuery("base", "EUR"))

	if len(baseCurrency) != 3 {
		c.JSON(http.StatusBadRequest, models.CurrencyRateResponse{
			Success: false,
			Message: "base parameter must be a 3-character ISO 4217 currency code",
		})
		return
	}

	rates, err := h.currencyService.GetAllRates(baseCurrency)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.CurrencyRateResponse{
			Success: false,
			Message: "Failed to get exchange rates: " + err.Error(),
		})
		return
	}

	rateDate, _ := h.currencyService.GetRateDate()

	c.JSON(http.StatusOK, models.CurrencyRateResponse{
		Success: true,
		Base:    baseCurrency,
		Date:    rateDate,
		Rates:   rates,
	})
}

// GetSupportedCurrencies returns the list of supported currencies
// @Summary Get supported currencies
// @Description Get a list of all supported currencies
// @Tags currency
// @Produce json
// @Success 200 {object} models.SupportedCurrenciesResponse
// @Failure 500 {object} models.SupportedCurrenciesResponse
// @Router /api/v1/currency/supported [get]
func (h *CurrencyHandler) GetSupportedCurrencies(c *gin.Context) {
	currencies, err := h.currencyService.GetSupportedCurrencies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.SupportedCurrenciesResponse{
			Success: false,
			Message: "Failed to get supported currencies: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SupportedCurrenciesResponse{
		Success:    true,
		Currencies: currencies,
	})
}

// GetRate returns the exchange rate between two currencies
// @Summary Get exchange rate
// @Description Get the exchange rate between two currencies
// @Tags currency
// @Produce json
// @Param from query string true "Source currency code (ISO 4217)"
// @Param to query string true "Target currency code (ISO 4217)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/currency/rate [get]
func (h *CurrencyHandler) GetRate(c *gin.Context) {
	from := strings.ToUpper(c.Query("from"))
	to := strings.ToUpper(c.Query("to"))

	if from == "" || len(from) != 3 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "from parameter must be a 3-character ISO 4217 currency code",
		})
		return
	}

	if to == "" || len(to) != 3 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "to parameter must be a 3-character ISO 4217 currency code",
		})
		return
	}

	rate, err := h.currencyService.GetRate(from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get exchange rate: " + err.Error(),
		})
		return
	}

	rateDate, _ := h.currencyService.GetRateDate()

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"from_currency": from,
		"to_currency":   to,
		"rate":          rate,
		"date":          rateDate,
	})
}

// RefreshRates triggers a manual refresh of exchange rates
// @Summary Refresh exchange rates
// @Description Manually trigger a refresh of exchange rates from the external API
// @Tags currency
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/currency/refresh [post]
func (h *CurrencyHandler) RefreshRates(c *gin.Context) {
	if h.rateUpdater == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Rate updater not configured",
		})
		return
	}

	if err := h.rateUpdater.ForceUpdate(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to refresh rates: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Exchange rates refreshed successfully",
	})
}

// GetUpdaterStatus returns the status of the rate updater
// @Summary Get rate updater status
// @Description Get the current status of the background rate updater
// @Tags currency
// @Produce json
// @Success 200 {object} workers.UpdaterStatus
// @Router /api/v1/currency/status [get]
func (h *CurrencyHandler) GetUpdaterStatus(c *gin.Context) {
	if h.rateUpdater == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"status": gin.H{
				"running": false,
				"message": "Rate updater not configured",
			},
		})
		return
	}

	status := h.rateUpdater.Status()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  status,
	})
}
