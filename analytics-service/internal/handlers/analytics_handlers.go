package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/tesseract-hub/analytics-service/internal/services"
)

// AnalyticsHandlers handles HTTP requests for analytics
type AnalyticsHandlers struct {
	service *services.AnalyticsService
	logger  *logrus.Logger
}

// NewAnalyticsHandlers creates a new analytics handlers instance
func NewAnalyticsHandlers(service *services.AnalyticsService, logger *logrus.Logger) *AnalyticsHandlers {
	return &AnalyticsHandlers{
		service: service,
		logger:  logger,
	}
}

// GetSalesDashboard retrieves sales dashboard metrics
// GET /api/v1/analytics/sales
func (h *AnalyticsHandlers) GetSalesDashboard(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	from, to, err := h.parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dashboard, err := h.service.GetSalesDashboard(c.Request.Context(), tenantID, from, to)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get sales dashboard")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sales dashboard"})
		return
	}

	c.JSON(http.StatusOK, dashboard)
}

// GetInventoryReport retrieves inventory report
// GET /api/v1/analytics/inventory
func (h *AnalyticsHandlers) GetInventoryReport(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	report, err := h.service.GetInventoryReport(c.Request.Context(), tenantID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get inventory report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get inventory report"})
		return
	}

	c.JSON(http.StatusOK, report)
}

// GetCustomerAnalytics retrieves customer analytics
// GET /api/v1/analytics/customers
func (h *AnalyticsHandlers) GetCustomerAnalytics(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	from, to, err := h.parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	analytics, err := h.service.GetCustomerAnalytics(c.Request.Context(), tenantID, from, to)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get customer analytics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get customer analytics"})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// GetFinancialReport retrieves financial report
// GET /api/v1/analytics/financial
func (h *AnalyticsHandlers) GetFinancialReport(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	from, to, err := h.parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	report, err := h.service.GetFinancialReport(c.Request.Context(), tenantID, from, to)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get financial report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get financial report"})
		return
	}

	c.JSON(http.StatusOK, report)
}

// ExportSalesReport exports sales data
// GET /api/v1/analytics/sales/export
func (h *AnalyticsHandlers) ExportSalesReport(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	format := c.DefaultQuery("format", "csv")

	from, to, err := h.parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var data []byte
	var contentType string
	var filename string

	switch format {
	case "csv":
		data, err = h.service.ExportSalesReport(c.Request.Context(), tenantID, from, to)
		contentType = "text/csv"
		filename = "sales-report.csv"
	case "json":
		dashboard, err := h.service.GetSalesDashboard(c.Request.Context(), tenantID, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sales data"})
			return
		}
		data, err = h.service.ExportToJSON(c.Request.Context(), dashboard)
		contentType = "application/json"
		filename = "sales-report.json"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Use 'csv' or 'json'"})
		return
	}

	if err != nil {
		h.logger.WithError(err).Error("Failed to export sales report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export sales report"})
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, data)
}

// ExportInventoryReport exports inventory data
// GET /api/v1/analytics/inventory/export
func (h *AnalyticsHandlers) ExportInventoryReport(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	format := c.DefaultQuery("format", "csv")

	var data []byte
	var contentType string
	var filename string
	var err error

	switch format {
	case "csv":
		data, err = h.service.ExportInventoryReport(c.Request.Context(), tenantID)
		contentType = "text/csv"
		filename = "inventory-report.csv"
	case "json":
		report, err := h.service.GetInventoryReport(c.Request.Context(), tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get inventory data"})
			return
		}
		data, err = h.service.ExportToJSON(c.Request.Context(), report)
		contentType = "application/json"
		filename = "inventory-report.json"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Use 'csv' or 'json'"})
		return
	}

	if err != nil {
		h.logger.WithError(err).Error("Failed to export inventory report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export inventory report"})
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, data)
}

// ExportCustomerReport exports customer data
// GET /api/v1/analytics/customers/export
func (h *AnalyticsHandlers) ExportCustomerReport(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	format := c.DefaultQuery("format", "csv")

	from, to, err := h.parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var data []byte
	var contentType string
	var filename string

	switch format {
	case "csv":
		data, err = h.service.ExportCustomerReport(c.Request.Context(), tenantID, from, to)
		contentType = "text/csv"
		filename = "customer-report.csv"
	case "json":
		analytics, err := h.service.GetCustomerAnalytics(c.Request.Context(), tenantID, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get customer data"})
			return
		}
		data, err = h.service.ExportToJSON(c.Request.Context(), analytics)
		contentType = "application/json"
		filename = "customer-report.json"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Use 'csv' or 'json'"})
		return
	}

	if err != nil {
		h.logger.WithError(err).Error("Failed to export customer report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export customer report"})
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, data)
}

// ExportFinancialReport exports financial data
// GET /api/v1/analytics/financial/export
func (h *AnalyticsHandlers) ExportFinancialReport(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	format := c.DefaultQuery("format", "csv")

	from, to, err := h.parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var data []byte
	var contentType string
	var filename string

	switch format {
	case "csv":
		data, err = h.service.ExportFinancialReport(c.Request.Context(), tenantID, from, to)
		contentType = "text/csv"
		filename = "financial-report.csv"
	case "json":
		report, err := h.service.GetFinancialReport(c.Request.Context(), tenantID, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get financial data"})
			return
		}
		data, err = h.service.ExportToJSON(c.Request.Context(), report)
		contentType = "application/json"
		filename = "financial-report.json"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Use 'csv' or 'json'"})
		return
	}

	if err != nil {
		h.logger.WithError(err).Error("Failed to export financial report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export financial report"})
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, contentType, data)
}

// parseDateRange parses date range from query parameters
func (h *AnalyticsHandlers) parseDateRange(c *gin.Context) (time.Time, time.Time, error) {
	// Check for preset
	if preset := c.Query("preset"); preset != "" {
		from, to := services.GetDateRangePresets(preset)
		return from, to, nil
	}

	// Parse custom dates
	fromStr := c.Query("from")
	toStr := c.Query("to")

	// Default to last 30 days if not provided
	if fromStr == "" || toStr == "" {
		to := time.Now()
		from := to.AddDate(0, 0, -30)
		return from, to, nil
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	return from, to, nil
}
