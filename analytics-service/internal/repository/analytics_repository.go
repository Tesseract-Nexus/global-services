package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/tesseract-hub/analytics-service/internal/models"
)

// AnalyticsRepository handles data aggregation for analytics
type AnalyticsRepository struct {
	db *gorm.DB
}

// NewAnalyticsRepository creates a new analytics repository
func NewAnalyticsRepository(db *gorm.DB) *AnalyticsRepository {
	return &AnalyticsRepository{
		db: db,
	}
}

// GetSalesDashboard generates sales dashboard metrics
func (r *AnalyticsRepository) GetSalesDashboard(ctx context.Context, tenantID string, from, to time.Time) (*models.SalesDashboard, error) {
	dashboard := &models.SalesDashboard{
		DateRange: models.DateRange{From: from, To: to},
	}

	// Total revenue and orders
	var result struct {
		TotalRevenue float64
		TotalOrders  int64
		TotalItems   int64
	}

	err := r.db.WithContext(ctx).
		Table("orders").
		Select("COALESCE(SUM(total), 0) as total_revenue, COUNT(*) as total_orders, 0 as total_items").
		Where("tenant_id = ? AND created_at BETWEEN ? AND ? AND status NOT IN (?)", tenantID, from, to, []string{"CANCELLED", "FAILED"}).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	dashboard.TotalRevenue = result.TotalRevenue
	dashboard.TotalOrders = result.TotalOrders
	dashboard.TotalItems = result.TotalItems

	if dashboard.TotalOrders > 0 {
		dashboard.AverageOrderValue = dashboard.TotalRevenue / float64(dashboard.TotalOrders)
	}

	// Previous period comparison
	previousPeriod := to.Sub(from)
	previousFrom := from.Add(-previousPeriod)
	previousTo := from

	var previousResult struct {
		TotalRevenue float64
		TotalOrders  int64
	}

	err = r.db.WithContext(ctx).
		Table("orders").
		Select("COALESCE(SUM(total), 0) as total_revenue, COUNT(*) as total_orders").
		Where("tenant_id = ? AND created_at BETWEEN ? AND ? AND status NOT IN (?)", tenantID, previousFrom, previousTo, []string{"CANCELLED", "FAILED"}).
		Scan(&previousResult).Error

	if err == nil && previousResult.TotalRevenue > 0 {
		dashboard.RevenueChange = ((dashboard.TotalRevenue - previousResult.TotalRevenue) / previousResult.TotalRevenue) * 100
	}
	if err == nil && previousResult.TotalOrders > 0 {
		dashboard.OrdersChange = ((float64(dashboard.TotalOrders) - float64(previousResult.TotalOrders)) / float64(previousResult.TotalOrders)) * 100
	}

	// Revenue by day
	var dailyRevenue []struct {
		Date  time.Time
		Value float64
		Count int64
	}

	err = r.db.WithContext(ctx).
		Table("orders").
		Select("DATE(created_at) as date, COALESCE(SUM(total), 0) as value, COUNT(*) as count").
		Where("tenant_id = ? AND created_at BETWEEN ? AND ? AND status NOT IN (?)", tenantID, from, to, []string{"CANCELLED", "FAILED"}).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&dailyRevenue).Error

	if err == nil {
		for _, dr := range dailyRevenue {
			dashboard.RevenueByDay = append(dashboard.RevenueByDay, models.TimeSeriesData{
				Date:  dr.Date,
				Value: dr.Value,
				Count: dr.Count,
			})
		}
	}

	// Top products
	var topProducts []struct {
		ProductID    string
		ProductName  string
		SKU          string
		UnitsSold    int64
		Revenue      float64
	}

	err = r.db.WithContext(ctx).
		Table("order_items oi").
		Select("oi.product_id, p.name as product_name, p.sku, SUM(oi.quantity) as units_sold, SUM(oi.total_price) as revenue").
		Joins("JOIN orders o ON o.id = oi.order_id").
		Joins("JOIN products p ON p.id = oi.product_id").
		Where("o.tenant_id = ? AND o.created_at BETWEEN ? AND ? AND o.status NOT IN (?)", tenantID, from, to, []string{"CANCELLED", "FAILED"}).
		Group("oi.product_id, p.name, p.sku").
		Order("revenue DESC").
		Limit(10).
		Scan(&topProducts).Error

	if err == nil {
		for _, tp := range topProducts {
			avgPrice := float64(0)
			if tp.UnitsSold > 0 {
				avgPrice = tp.Revenue / float64(tp.UnitsSold)
			}
			dashboard.TopProducts = append(dashboard.TopProducts, models.ProductSales{
				ProductID:    tp.ProductID,
				ProductName:  tp.ProductName,
				SKU:          tp.SKU,
				UnitsSold:    tp.UnitsSold,
				Revenue:      tp.Revenue,
				AveragePrice: avgPrice,
			})
		}
	}

	// Top categories
	var topCategories []struct {
		CategoryID   string
		CategoryName string
		UnitsSold    int64
		Revenue      float64
		OrderCount   int64
	}

	err = r.db.WithContext(ctx).
		Table("order_items oi").
		Select("c.id as category_id, c.name as category_name, SUM(oi.quantity) as units_sold, SUM(oi.total_price) as revenue, COUNT(DISTINCT o.id) as order_count").
		Joins("JOIN orders o ON o.id = oi.order_id").
		Joins("JOIN products p ON p.id = oi.product_id").
		Joins("JOIN categories c ON c.id = CAST(p.category_id AS uuid)").
		Where("o.tenant_id = ? AND o.created_at BETWEEN ? AND ? AND o.status NOT IN (?)", tenantID, from, to, []string{"CANCELLED", "FAILED"}).
		Group("c.id, c.name").
		Order("revenue DESC").
		Limit(10).
		Scan(&topCategories).Error

	if err == nil {
		for _, tc := range topCategories {
			var productCount int64
			r.db.WithContext(ctx).Table("products").Where("CAST(category_id AS uuid) = ? AND tenant_id = ?", tc.CategoryID, tenantID).Count(&productCount)

			dashboard.TopCategories = append(dashboard.TopCategories, models.CategorySales{
				CategoryID:   tc.CategoryID,
				CategoryName: tc.CategoryName,
				ProductCount: productCount,
				UnitsSold:    tc.UnitsSold,
				Revenue:      tc.Revenue,
				OrderCount:   tc.OrderCount,
			})
		}
	}

	// Revenue by status
	var revenueByStatus []struct {
		Status      string
		OrderCount  int64
		TotalRevenue float64
	}

	err = r.db.WithContext(ctx).
		Table("orders").
		Select("status, COUNT(*) as order_count, COALESCE(SUM(total), 0) as total_revenue").
		Where("tenant_id = ? AND created_at BETWEEN ?  AND ?", tenantID, from, to).
		Group("status").
		Scan(&revenueByStatus).Error

	if err == nil {
		for _, rs := range revenueByStatus {
			percentage := float64(0)
			if dashboard.TotalRevenue > 0 {
				percentage = (rs.TotalRevenue / dashboard.TotalRevenue) * 100
			}
			dashboard.RevenueByStatus = append(dashboard.RevenueByStatus, models.StatusRevenue{
				Status:       rs.Status,
				OrderCount:   rs.OrderCount,
				TotalRevenue: rs.TotalRevenue,
				Percentage:   percentage,
			})
		}
	}

	// Payment methods
	var paymentMethods []struct {
		Method      string
		OrderCount  int64
		TotalAmount float64
	}

	err = r.db.WithContext(ctx).
		Table("orders o").
		Joins("LEFT JOIN order_payments op ON op.order_id = o.id").
		Select("COALESCE(op.method, 'Unknown') as method, COUNT(*) as order_count, COALESCE(SUM(o.total), 0) as total_amount").
		Where("o.tenant_id = ? AND o.created_at BETWEEN ? AND ? AND o.status NOT IN (?)", tenantID, from, to, []string{"CANCELLED", "FAILED"}).
		Group("op.method").
		Scan(&paymentMethods).Error

	if err == nil {
		for _, pm := range paymentMethods {
			percentage := float64(0)
			if dashboard.TotalRevenue > 0 {
				percentage = (pm.TotalAmount / dashboard.TotalRevenue) * 100
			}
			dashboard.PaymentMethods = append(dashboard.PaymentMethods, models.PaymentMethodStats{
				Method:      pm.Method,
				OrderCount:  pm.OrderCount,
				TotalAmount: pm.TotalAmount,
				Percentage:  percentage,
				SuccessRate: 100.0, // Calculate actual success rate if payment tracking available
			})
		}
	}

	return dashboard, nil
}

// GetInventoryReport generates inventory report
func (r *AnalyticsRepository) GetInventoryReport(ctx context.Context, tenantID string) (*models.InventoryReport, error) {
	report := &models.InventoryReport{}

	// Total products and value
	var totals struct {
		TotalProducts int64
		TotalSKUs     int64
		TotalValue    float64
	}

	err := r.db.WithContext(ctx).
		Table("products").
		Select("COUNT(*) as total_products, COUNT(DISTINCT sku) as total_skus, COALESCE(SUM(CAST(price AS DECIMAL) * COALESCE(quantity, 0)), 0) as total_value").
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Scan(&totals).Error

	if err != nil {
		return nil, err
	}

	report.TotalProducts = totals.TotalProducts
	report.TotalSKUs = totals.TotalSKUs
	report.TotalValue = totals.TotalValue

	// Low stock and out of stock counts
	var stockCounts struct {
		LowStock   int64
		OutOfStock int64
	}

	r.db.WithContext(ctx).
		Table("products").
		Where("tenant_id = ? AND deleted_at IS NULL AND quantity <= low_stock_threshold AND quantity > 0", tenantID).
		Count(&stockCounts.LowStock)

	r.db.WithContext(ctx).
		Table("products").
		Where("tenant_id = ? AND deleted_at IS NULL AND (quantity = 0 OR quantity IS NULL)", tenantID).
		Count(&stockCounts.OutOfStock)

	report.LowStockCount = stockCounts.LowStock
	report.OutOfStockCount = stockCounts.OutOfStock

	// Low stock products
	var lowStockProducts []struct {
		ID                 string
		Name               string
		SKU                string
		Quantity           int64
		LowStockThreshold  int64
		Price              string
	}

	err = r.db.WithContext(ctx).
		Table("products").
		Select("id, name, sku, COALESCE(quantity, 0) as quantity, COALESCE(low_stock_threshold, 0) as low_stock_threshold, price").
		Where("tenant_id = ? AND deleted_at IS NULL AND quantity <= low_stock_threshold AND quantity > 0", tenantID).
		Order("quantity ASC").
		Limit(20).
		Scan(&lowStockProducts).Error

	if err == nil {
		for _, p := range lowStockProducts {
			var priceFloat float64
			fmt.Sscanf(p.Price, "%f", &priceFloat)
			report.LowStockProducts = append(report.LowStockProducts, models.InventoryItem{
				ProductID:    p.ID,
				ProductName:  p.Name,
				SKU:          p.SKU,
				StockLevel:   p.Quantity,
				ReorderLevel: p.LowStockThreshold,
				Value:        float64(p.Quantity) * priceFloat,
			})
		}
	}

	// Out of stock products
	var outOfStockProducts []struct {
		ID    string
		Name  string
		SKU   string
		Price string
	}

	err = r.db.WithContext(ctx).
		Table("products").
		Select("id, name, sku, price").
		Where("tenant_id = ? AND deleted_at IS NULL AND (quantity = 0 OR quantity IS NULL)", tenantID).
		Limit(20).
		Scan(&outOfStockProducts).Error

	if err == nil {
		for _, p := range outOfStockProducts {
			report.OutOfStockProducts = append(report.OutOfStockProducts, models.InventoryItem{
				ProductID:   p.ID,
				ProductName: p.Name,
				SKU:         p.SKU,
				StockLevel:  0,
				Value:       0,
			})
		}
	}

	// Inventory by category
	var inventoryByCategory []struct {
		CategoryID   string
		CategoryName string
		ProductCount int64
		TotalStock   int64
		TotalValue   float64
		LowStockCount int64
	}

	err = r.db.WithContext(ctx).
		Table("products p").
		Select("c.id as category_id, c.name as category_name, COUNT(*) as product_count, COALESCE(SUM(p.quantity), 0) as total_stock, COALESCE(SUM(CAST(p.price AS DECIMAL) * COALESCE(p.quantity, 0)), 0) as total_value, SUM(CASE WHEN p.quantity <= p.low_stock_threshold THEN 1 ELSE 0 END) as low_stock_count").
		Joins("JOIN categories c ON c.id = CAST(p.category_id AS uuid)").
		Where("p.tenant_id = ? AND p.deleted_at IS NULL", tenantID).
		Group("c.id, c.name").
		Scan(&inventoryByCategory).Error

	if err == nil {
		for _, ic := range inventoryByCategory {
			report.InventoryByCategory = append(report.InventoryByCategory, models.CategoryInventory{
				CategoryID:    ic.CategoryID,
				CategoryName:  ic.CategoryName,
				ProductCount:  ic.ProductCount,
				TotalStock:    ic.TotalStock,
				TotalValue:    ic.TotalValue,
				LowStockCount: ic.LowStockCount,
			})
		}
	}

	return report, nil
}

// GetCustomerAnalytics generates customer analytics
func (r *AnalyticsRepository) GetCustomerAnalytics(ctx context.Context, tenantID string, from, to time.Time) (*models.CustomerAnalytics, error) {
	analytics := &models.CustomerAnalytics{
		DateRange: models.DateRange{From: from, To: to},
	}

	// Total customers
	r.db.WithContext(ctx).
		Table("customers").
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Count(&analytics.TotalCustomers)

	// New customers in period
	r.db.WithContext(ctx).
		Table("customers").
		Where("tenant_id = ? AND created_at BETWEEN ? AND ? AND deleted_at IS NULL", tenantID, from, to).
		Count(&analytics.NewCustomers)

	// Returning customers (had orders before this period)
	var returningCustomers int64
	r.db.WithContext(ctx).
		Table("customers c").
		Joins("JOIN orders o ON o.customer_id = c.id").
		Where("c.tenant_id = ? AND o.created_at BETWEEN ? AND ?", tenantID, from, to).
		Where("EXISTS (SELECT 1 FROM orders o2 WHERE o2.customer_id = c.id AND o2.created_at < ?)", from).
		Group("c.id").
		Count(&returningCustomers)

	analytics.ReturningCustomers = returningCustomers

	// Average lifetime value
	var avgLTV struct {
		AvgLTV float64
	}

	r.db.WithContext(ctx).
		Table("customers c").
		Select("AVG(total_spent) as avg_ltv").
		Where("c.tenant_id = ? AND c.deleted_at IS NULL", tenantID).
		Scan(&avgLTV)

	analytics.AverageLifetimeValue = avgLTV.AvgLTV

	// Top customers
	var topCustomers []struct {
		CustomerID       string
		FirstName        string
		LastName         string
		Email            string
		TotalOrders      int64
		TotalSpent       float64
		FirstOrderDate   time.Time
		LastOrderDate    time.Time
	}

	err := r.db.WithContext(ctx).
		Table("customers c").
		Select("c.id as customer_id, c.first_name, c.last_name, c.email, COUNT(o.id) as total_orders, COALESCE(SUM(o.total), 0) as total_spent, MIN(o.created_at) as first_order_date, MAX(o.created_at) as last_order_date").
		Joins("LEFT JOIN orders o ON o.customer_id = c.id AND o.status NOT IN (?)", []string{"CANCELLED", "FAILED"}).
		Where("c.tenant_id = ? AND c.deleted_at IS NULL", tenantID).
		Group("c.id, c.first_name, c.last_name, c.email").
		Having("COUNT(o.id) > 0").
		Order("total_spent DESC").
		Limit(20).
		Scan(&topCustomers).Error

	if err == nil {
		for _, tc := range topCustomers {
			avgOrderValue := float64(0)
			if tc.TotalOrders > 0 {
				avgOrderValue = tc.TotalSpent / float64(tc.TotalOrders)
			}
			daysSinceLastOrder := int64(time.Since(tc.LastOrderDate).Hours() / 24)

			analytics.TopCustomers = append(analytics.TopCustomers, models.CustomerMetrics{
				CustomerID:         tc.CustomerID,
				CustomerName:       fmt.Sprintf("%s %s", tc.FirstName, tc.LastName),
				Email:              tc.Email,
				TotalOrders:        tc.TotalOrders,
				TotalSpent:         tc.TotalSpent,
				AverageOrderValue:  avgOrderValue,
				FirstOrderDate:     tc.FirstOrderDate,
				LastOrderDate:      tc.LastOrderDate,
				DaysSinceLastOrder: daysSinceLastOrder,
			})
		}
	}

	return analytics, nil
}

// GetFinancialReport generates financial report
func (r *AnalyticsRepository) GetFinancialReport(ctx context.Context, tenantID string, from, to time.Time) (*models.FinancialReport, error) {
	report := &models.FinancialReport{
		DateRange: models.DateRange{From: from, To: to},
	}

	// Revenue metrics
	var revenue struct {
		GrossRevenue    float64
		Discounts       float64
		ShippingRevenue float64
		TaxCollected    float64
	}

	err := r.db.WithContext(ctx).
		Table("orders").
		Select("COALESCE(SUM(total), 0) as gross_revenue, COALESCE(SUM(discount_amount), 0) as discounts, COALESCE(SUM(shipping_cost), 0) as shipping_revenue, COALESCE(SUM(tax_amount), 0) as tax_collected").
		Where("tenant_id = ? AND created_at BETWEEN ? AND ? AND status NOT IN (?)", tenantID, from, to, []string{"CANCELLED", "FAILED"}).
		Scan(&revenue).Error

	if err != nil {
		return nil, err
	}

	report.GrossRevenue = revenue.GrossRevenue
	report.Discounts = revenue.Discounts
	report.ShippingRevenue = revenue.ShippingRevenue
	report.TaxCollected = revenue.TaxCollected

	// Refunds
	var refunds struct {
		TotalRefunds float64
	}

	r.db.WithContext(ctx).
		Table("returns").
		Select("COALESCE(SUM(refund_amount), 0) as total_refunds").
		Where("tenant_id = ? AND created_at BETWEEN ? AND ? AND status = ?", tenantID, from, to, "COMPLETED").
		Scan(&refunds)

	report.Refunds = refunds.TotalRefunds
	report.NetRevenue = report.GrossRevenue - report.Refunds - report.Discounts
	report.GrossProfit = report.NetRevenue - report.ShippingCosts - report.ProcessingFees

	if report.GrossRevenue > 0 {
		report.GrossProfitMargin = (report.GrossProfit / report.GrossRevenue) * 100
	}

	return report, nil
}
