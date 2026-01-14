package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"analytics-service/internal/models"
	"analytics-service/internal/repository"
)

// AnalyticsService handles business logic for analytics
type AnalyticsService struct {
	repo   *repository.AnalyticsRepository
	logger *logrus.Logger
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(repo *repository.AnalyticsRepository, logger *logrus.Logger) *AnalyticsService {
	return &AnalyticsService{
		repo:   repo,
		logger: logger,
	}
}

// GetSalesDashboard retrieves sales dashboard metrics
func (s *AnalyticsService) GetSalesDashboard(ctx context.Context, tenantID string, from, to time.Time) (*models.SalesDashboard, error) {
	dashboard, err := s.repo.GetSalesDashboard(ctx, tenantID, from, to)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get sales dashboard")
		return nil, fmt.Errorf("failed to get sales dashboard: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id":     tenantID,
		"total_revenue": dashboard.TotalRevenue,
		"total_orders":  dashboard.TotalOrders,
	}).Info("Generated sales dashboard")

	return dashboard, nil
}

// GetInventoryReport retrieves inventory report
func (s *AnalyticsService) GetInventoryReport(ctx context.Context, tenantID string) (*models.InventoryReport, error) {
	report, err := s.repo.GetInventoryReport(ctx, tenantID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get inventory report")
		return nil, fmt.Errorf("failed to get inventory report: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id":       tenantID,
		"total_products":  report.TotalProducts,
		"low_stock_count": report.LowStockCount,
		"out_of_stock":    report.OutOfStockCount,
	}).Info("Generated inventory report")

	return report, nil
}

// GetCustomerAnalytics retrieves customer analytics
func (s *AnalyticsService) GetCustomerAnalytics(ctx context.Context, tenantID string, from, to time.Time) (*models.CustomerAnalytics, error) {
	analytics, err := s.repo.GetCustomerAnalytics(ctx, tenantID, from, to)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get customer analytics")
		return nil, fmt.Errorf("failed to get customer analytics: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id":       tenantID,
		"total_customers": analytics.TotalCustomers,
		"new_customers":   analytics.NewCustomers,
	}).Info("Generated customer analytics")

	return analytics, nil
}

// GetFinancialReport retrieves financial report
func (s *AnalyticsService) GetFinancialReport(ctx context.Context, tenantID string, from, to time.Time) (*models.FinancialReport, error) {
	report, err := s.repo.GetFinancialReport(ctx, tenantID, from, to)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get financial report")
		return nil, fmt.Errorf("failed to get financial report: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id":     tenantID,
		"gross_revenue": report.GrossRevenue,
		"net_revenue":   report.NetRevenue,
		"gross_profit":  report.GrossProfit,
	}).Info("Generated financial report")

	return report, nil
}

// ExportSalesReport exports sales data to CSV
func (s *AnalyticsService) ExportSalesReport(ctx context.Context, tenantID string, from, to time.Time) ([]byte, error) {
	dashboard, err := s.repo.GetSalesDashboard(ctx, tenantID, from, to)
	if err != nil {
		return nil, err
	}

	var csvData [][]string

	// Header
	csvData = append(csvData, []string{
		"Metric", "Value",
	})

	// Summary metrics
	csvData = append(csvData, []string{"Total Revenue", fmt.Sprintf("%.2f", dashboard.TotalRevenue)})
	csvData = append(csvData, []string{"Total Orders", fmt.Sprintf("%d", dashboard.TotalOrders)})
	csvData = append(csvData, []string{"Average Order Value", fmt.Sprintf("%.2f", dashboard.AverageOrderValue)})
	csvData = append(csvData, []string{"Total Items Sold", fmt.Sprintf("%d", dashboard.TotalItems)})
	csvData = append(csvData, []string{"Revenue Change (%)", fmt.Sprintf("%.2f", dashboard.RevenueChange)})

	csvData = append(csvData, []string{""}) // Empty row

	// Top Products
	csvData = append(csvData, []string{"Top Products"})
	csvData = append(csvData, []string{"Product Name", "SKU", "Units Sold", "Revenue", "Avg Price"})
	for _, p := range dashboard.TopProducts {
		csvData = append(csvData, []string{
			p.ProductName,
			p.SKU,
			fmt.Sprintf("%d", p.UnitsSold),
			fmt.Sprintf("%.2f", p.Revenue),
			fmt.Sprintf("%.2f", p.AveragePrice),
		})
	}

	csvData = append(csvData, []string{""}) // Empty row

	// Daily Revenue
	csvData = append(csvData, []string{"Daily Revenue"})
	csvData = append(csvData, []string{"Date", "Revenue", "Orders"})
	for _, dr := range dashboard.RevenueByDay {
		csvData = append(csvData, []string{
			dr.Date.Format("2006-01-02"),
			fmt.Sprintf("%.2f", dr.Value),
			fmt.Sprintf("%d", dr.Count),
		})
	}

	// Convert to CSV bytes
	var buf []byte
	writer := csv.NewWriter(&csvWriter{data: &buf})
	if err := writer.WriteAll(csvData); err != nil {
		return nil, fmt.Errorf("failed to write CSV: %w", err)
	}

	s.logger.WithField("tenant_id", tenantID).Info("Exported sales report")

	return buf, nil
}

// ExportInventoryReport exports inventory data to CSV
func (s *AnalyticsService) ExportInventoryReport(ctx context.Context, tenantID string) ([]byte, error) {
	report, err := s.repo.GetInventoryReport(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var csvData [][]string

	// Header
	csvData = append(csvData, []string{
		"Metric", "Value",
	})

	// Summary
	csvData = append(csvData, []string{"Total Products", fmt.Sprintf("%d", report.TotalProducts)})
	csvData = append(csvData, []string{"Total SKUs", fmt.Sprintf("%d", report.TotalSKUs)})
	csvData = append(csvData, []string{"Total Inventory Value", fmt.Sprintf("%.2f", report.TotalValue)})
	csvData = append(csvData, []string{"Low Stock Items", fmt.Sprintf("%d", report.LowStockCount)})
	csvData = append(csvData, []string{"Out of Stock Items", fmt.Sprintf("%d", report.OutOfStockCount)})

	csvData = append(csvData, []string{""}) // Empty row

	// Low Stock Products
	csvData = append(csvData, []string{"Low Stock Products"})
	csvData = append(csvData, []string{"Product Name", "SKU", "Current Stock", "Reorder Level", "Value"})
	for _, p := range report.LowStockProducts {
		csvData = append(csvData, []string{
			p.ProductName,
			p.SKU,
			fmt.Sprintf("%d", p.StockLevel),
			fmt.Sprintf("%d", p.ReorderLevel),
			fmt.Sprintf("%.2f", p.Value),
		})
	}

	csvData = append(csvData, []string{""}) // Empty row

	// Out of Stock Products
	csvData = append(csvData, []string{"Out of Stock Products"})
	csvData = append(csvData, []string{"Product Name", "SKU"})
	for _, p := range report.OutOfStockProducts {
		csvData = append(csvData, []string{
			p.ProductName,
			p.SKU,
		})
	}

	csvData = append(csvData, []string{""}) // Empty row

	// Inventory by Category
	csvData = append(csvData, []string{"Inventory by Category"})
	csvData = append(csvData, []string{"Category", "Products", "Total Stock", "Value", "Low Stock Count"})
	for _, c := range report.InventoryByCategory {
		csvData = append(csvData, []string{
			c.CategoryName,
			fmt.Sprintf("%d", c.ProductCount),
			fmt.Sprintf("%d", c.TotalStock),
			fmt.Sprintf("%.2f", c.TotalValue),
			fmt.Sprintf("%d", c.LowStockCount),
		})
	}

	// Convert to CSV bytes
	var buf []byte
	writer := csv.NewWriter(&csvWriter{data: &buf})
	if err := writer.WriteAll(csvData); err != nil {
		return nil, fmt.Errorf("failed to write CSV: %w", err)
	}

	s.logger.WithField("tenant_id", tenantID).Info("Exported inventory report")

	return buf, nil
}

// ExportCustomerReport exports customer data to CSV
func (s *AnalyticsService) ExportCustomerReport(ctx context.Context, tenantID string, from, to time.Time) ([]byte, error) {
	analytics, err := s.repo.GetCustomerAnalytics(ctx, tenantID, from, to)
	if err != nil {
		return nil, err
	}

	var csvData [][]string

	// Header
	csvData = append(csvData, []string{
		"Metric", "Value",
	})

	// Summary
	csvData = append(csvData, []string{"Total Customers", fmt.Sprintf("%d", analytics.TotalCustomers)})
	csvData = append(csvData, []string{"New Customers", fmt.Sprintf("%d", analytics.NewCustomers)})
	csvData = append(csvData, []string{"Returning Customers", fmt.Sprintf("%d", analytics.ReturningCustomers)})
	csvData = append(csvData, []string{"Average Lifetime Value", fmt.Sprintf("%.2f", analytics.AverageLifetimeValue)})

	csvData = append(csvData, []string{""}) // Empty row

	// Top Customers
	csvData = append(csvData, []string{"Top Customers"})
	csvData = append(csvData, []string{"Name", "Email", "Total Orders", "Total Spent", "Avg Order Value", "Last Order Date"})
	for _, c := range analytics.TopCustomers {
		csvData = append(csvData, []string{
			c.CustomerName,
			c.Email,
			fmt.Sprintf("%d", c.TotalOrders),
			fmt.Sprintf("%.2f", c.TotalSpent),
			fmt.Sprintf("%.2f", c.AverageOrderValue),
			c.LastOrderDate.Format("2006-01-02"),
		})
	}

	// Convert to CSV bytes
	var buf []byte
	writer := csv.NewWriter(&csvWriter{data: &buf})
	if err := writer.WriteAll(csvData); err != nil {
		return nil, fmt.Errorf("failed to write CSV: %w", err)
	}

	s.logger.WithField("tenant_id", tenantID).Info("Exported customer report")

	return buf, nil
}

// ExportFinancialReport exports financial data to CSV
func (s *AnalyticsService) ExportFinancialReport(ctx context.Context, tenantID string, from, to time.Time) ([]byte, error) {
	report, err := s.repo.GetFinancialReport(ctx, tenantID, from, to)
	if err != nil {
		return nil, err
	}

	var csvData [][]string

	// Header
	csvData = append(csvData, []string{
		"Metric", "Value",
	})

	// Summary
	csvData = append(csvData, []string{"Gross Revenue", fmt.Sprintf("%.2f", report.GrossRevenue)})
	csvData = append(csvData, []string{"Discounts", fmt.Sprintf("%.2f", report.Discounts)})
	csvData = append(csvData, []string{"Refunds", fmt.Sprintf("%.2f", report.Refunds)})
	csvData = append(csvData, []string{"Net Revenue", fmt.Sprintf("%.2f", report.NetRevenue)})
	csvData = append(csvData, []string{"Shipping Revenue", fmt.Sprintf("%.2f", report.ShippingRevenue)})
	csvData = append(csvData, []string{"Tax Collected", fmt.Sprintf("%.2f", report.TaxCollected)})
	csvData = append(csvData, []string{"Shipping Costs", fmt.Sprintf("%.2f", report.ShippingCosts)})
	csvData = append(csvData, []string{"Processing Fees", fmt.Sprintf("%.2f", report.ProcessingFees)})
	csvData = append(csvData, []string{"Gross Profit", fmt.Sprintf("%.2f", report.GrossProfit)})
	csvData = append(csvData, []string{"Gross Profit Margin (%)", fmt.Sprintf("%.2f", report.GrossProfitMargin)})

	// Convert to CSV bytes
	var buf []byte
	writer := csv.NewWriter(&csvWriter{data: &buf})
	if err := writer.WriteAll(csvData); err != nil {
		return nil, fmt.Errorf("failed to write CSV: %w", err)
	}

	s.logger.WithField("tenant_id", tenantID).Info("Exported financial report")

	return buf, nil
}

// ExportToJSON exports any report to JSON format
func (s *AnalyticsService) ExportToJSON(ctx context.Context, data interface{}) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return jsonData, nil
}

// csvWriter is a helper to write CSV to byte slice
type csvWriter struct {
	data *[]byte
}

func (w *csvWriter) Write(p []byte) (n int, err error) {
	*w.data = append(*w.data, p...)
	return len(p), nil
}

// GetDateRangePresets returns common date range presets
func GetDateRangePresets(preset string) (time.Time, time.Time) {
	now := time.Now()
	var from, to time.Time

	switch preset {
	case "today":
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		to = now
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		from = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		to = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 0, yesterday.Location())
	case "last7days":
		from = now.AddDate(0, 0, -7)
		to = now
	case "last30days":
		from = now.AddDate(0, 0, -30)
		to = now
	case "thisMonth":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to = now
	case "lastMonth":
		lastMonth := now.AddDate(0, -1, 0)
		from = time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, lastMonth.Location())
		to = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Add(-time.Second)
	case "thisYear":
		from = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		to = now
	default: // last30days
		from = now.AddDate(0, 0, -30)
		to = now
	}

	return from, to
}
