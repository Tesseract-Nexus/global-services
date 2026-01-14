package models

import (
	"time"
)

// DateRange represents a time period for analytics
type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// SalesDashboard represents overall sales metrics
type SalesDashboard struct {
	DateRange DateRange `json:"dateRange"`

	// Revenue metrics
	TotalRevenue       float64 `json:"totalRevenue"`
	AverageOrderValue  float64 `json:"averageOrderValue"`
	TotalOrders        int64   `json:"totalOrders"`
	TotalItems         int64   `json:"totalItemsSold"`

	// Comparison with previous period
	RevenueChange      float64 `json:"revenueChange"`      // Percentage
	OrdersChange       float64 `json:"ordersChange"`       // Percentage
	AOVChange          float64 `json:"aovChange"`          // Percentage

	// Breakdown
	RevenueByDay       []TimeSeriesData `json:"revenueByDay"`
	OrdersByDay        []TimeSeriesData `json:"ordersByDay"`
	TopProducts        []ProductSales   `json:"topProducts"`
	TopCategories      []CategorySales  `json:"topCategories"`
	RevenueByStatus    []StatusRevenue  `json:"revenueByStatus"`
	PaymentMethods     []PaymentMethodStats `json:"paymentMethods"`
}

// TimeSeriesData represents a data point over time
type TimeSeriesData struct {
	Date  time.Time `json:"date"`
	Value float64   `json:"value"`
	Count int64     `json:"count,omitempty"`
}

// ProductSales represents sales data for a product
type ProductSales struct {
	ProductID    string  `json:"productId"`
	ProductName  string  `json:"productName"`
	SKU          string  `json:"sku"`
	UnitsSold    int64   `json:"unitsSold"`
	Revenue      float64 `json:"revenue"`
	AveragePrice float64 `json:"averagePrice"`
}

// CategorySales represents sales data for a category
type CategorySales struct {
	CategoryID   string  `json:"categoryId"`
	CategoryName string  `json:"categoryName"`
	ProductCount int64   `json:"productCount"`
	UnitsSold    int64   `json:"unitsSold"`
	Revenue      float64 `json:"revenue"`
	OrderCount   int64   `json:"orderCount"`
}

// StatusRevenue represents revenue by order status
type StatusRevenue struct {
	Status      string  `json:"status"`
	OrderCount  int64   `json:"orderCount"`
	TotalRevenue float64 `json:"totalRevenue"`
	Percentage  float64 `json:"percentage"`
}

// PaymentMethodStats represents payment method statistics
type PaymentMethodStats struct {
	Method       string  `json:"method"`
	OrderCount   int64   `json:"orderCount"`
	TotalAmount  float64 `json:"totalAmount"`
	Percentage   float64 `json:"percentage"`
	SuccessRate  float64 `json:"successRate"`
}

// InventoryReport represents inventory analytics
type InventoryReport struct {
	TotalProducts      int64                `json:"totalProducts"`
	TotalSKUs          int64                `json:"totalSkus"`
	TotalValue         float64              `json:"totalValue"`
	LowStockCount      int64                `json:"lowStockCount"`
	OutOfStockCount    int64                `json:"outOfStockCount"`

	LowStockProducts   []InventoryItem      `json:"lowStockProducts"`
	OutOfStockProducts []InventoryItem      `json:"outOfStockProducts"`
	TopMovingProducts  []InventoryMovement  `json:"topMovingProducts"`
	SlowMovingProducts []InventoryMovement  `json:"slowMovingProducts"`
	InventoryByCategory []CategoryInventory `json:"inventoryByCategory"`
	InventoryTurnover  []TurnoverRate       `json:"inventoryTurnover"`
}

// InventoryItem represents a product inventory item
type InventoryItem struct {
	ProductID    string  `json:"productId"`
	ProductName  string  `json:"productName"`
	SKU          string  `json:"sku"`
	StockLevel   int64   `json:"stockLevel"`
	ReorderLevel int64   `json:"reorderLevel"`
	Value        float64 `json:"value"`
	LastRestocked *time.Time `json:"lastRestocked,omitempty"`
}

// InventoryMovement represents product movement metrics
type InventoryMovement struct {
	ProductID      string  `json:"productId"`
	ProductName    string  `json:"productName"`
	SKU            string  `json:"sku"`
	UnitsSold      int64   `json:"unitsSold"`
	DaysInStock    int64   `json:"daysInStock"`
	TurnoverRate   float64 `json:"turnoverRate"`
	CurrentStock   int64   `json:"currentStock"`
}

// CategoryInventory represents inventory by category
type CategoryInventory struct {
	CategoryID    string  `json:"categoryId"`
	CategoryName  string  `json:"categoryName"`
	ProductCount  int64   `json:"productCount"`
	TotalStock    int64   `json:"totalStock"`
	TotalValue    float64 `json:"totalValue"`
	LowStockCount int64   `json:"lowStockCount"`
}

// TurnoverRate represents inventory turnover metrics
type TurnoverRate struct {
	Period        string  `json:"period"`
	TurnoverRate  float64 `json:"turnoverRate"`
	DaysToSell    float64 `json:"daysToSell"`
}

// CustomerAnalytics represents customer behavior metrics
type CustomerAnalytics struct {
	DateRange DateRange `json:"dateRange"`

	// Overall metrics
	TotalCustomers       int64   `json:"totalCustomers"`
	NewCustomers         int64   `json:"newCustomers"`
	ReturningCustomers   int64   `json:"returningCustomers"`
	AverageLifetimeValue float64 `json:"averageLifetimeValue"`
	AverageOrdersPerCustomer float64 `json:"averageOrdersPerCustomer"`
	CustomerRetentionRate float64 `json:"customerRetentionRate"`

	// Segmentation
	CustomersByValue     []CustomerSegment    `json:"customersByValue"`
	CustomersByOrders    []CustomerSegment    `json:"customersByOrders"`
	TopCustomers         []CustomerMetrics    `json:"topCustomers"`
	CustomerGrowth       []TimeSeriesData     `json:"customerGrowth"`
	GeographicDistribution []GeographicData   `json:"geographicDistribution"`
	CustomerCohorts      []CohortAnalysis     `json:"customerCohorts"`
}

// CustomerSegment represents a customer segment
type CustomerSegment struct {
	SegmentName    string  `json:"segmentName"`
	CustomerCount  int64   `json:"customerCount"`
	TotalRevenue   float64 `json:"totalRevenue"`
	AverageValue   float64 `json:"averageValue"`
	Percentage     float64 `json:"percentage"`
}

// CustomerMetrics represents individual customer metrics
type CustomerMetrics struct {
	CustomerID       string    `json:"customerId"`
	CustomerName     string    `json:"customerName"`
	Email            string    `json:"email"`
	TotalOrders      int64     `json:"totalOrders"`
	TotalSpent       float64   `json:"totalSpent"`
	AverageOrderValue float64  `json:"averageOrderValue"`
	FirstOrderDate   time.Time `json:"firstOrderDate"`
	LastOrderDate    time.Time `json:"lastOrderDate"`
	DaysSinceLastOrder int64   `json:"daysSinceLastOrder"`
}

// GeographicData represents sales by location
type GeographicData struct {
	Country       string  `json:"country"`
	State         string  `json:"state,omitempty"`
	City          string  `json:"city,omitempty"`
	CustomerCount int64   `json:"customerCount"`
	OrderCount    int64   `json:"orderCount"`
	Revenue       float64 `json:"revenue"`
}

// CohortAnalysis represents cohort retention analysis
type CohortAnalysis struct {
	CohortMonth    string    `json:"cohortMonth"`
	CustomerCount  int64     `json:"customerCount"`
	RetentionRates []float64 `json:"retentionRates"` // Month 0, 1, 2, 3...
}

// FinancialReport represents financial metrics
type FinancialReport struct {
	DateRange DateRange `json:"dateRange"`

	// Revenue
	GrossRevenue    float64 `json:"grossRevenue"`
	NetRevenue      float64 `json:"netRevenue"`
	Refunds         float64 `json:"refunds"`
	Discounts       float64 `json:"discounts"`
	ShippingRevenue float64 `json:"shippingRevenue"`
	TaxCollected    float64 `json:"taxCollected"`

	// Costs
	ShippingCosts   float64 `json:"shippingCosts"`
	RefundCosts     float64 `json:"refundCosts"`
	ProcessingFees  float64 `json:"processingFees"`

	// Profit
	GrossProfit     float64 `json:"grossProfit"`
	GrossProfitMargin float64 `json:"grossProfitMargin"`

	// Breakdown
	RevenueByCategory []CategoryFinancials `json:"revenueByCategory"`
	RevenueByPayment  []PaymentMethodStats `json:"revenueByPayment"`
	MonthlyTrends     []MonthlyFinancials  `json:"monthlyTrends"`
	ReturnsAnalysis   ReturnsAnalysis      `json:"returnsAnalysis"`
}

// CategoryFinancials represents financial data by category
type CategoryFinancials struct {
	CategoryID    string  `json:"categoryId"`
	CategoryName  string  `json:"categoryName"`
	Revenue       float64 `json:"revenue"`
	Refunds       float64 `json:"refunds"`
	NetRevenue    float64 `json:"netRevenue"`
	OrderCount    int64   `json:"orderCount"`
	AvgOrderValue float64 `json:"avgOrderValue"`
}

// MonthlyFinancials represents monthly financial trends
type MonthlyFinancials struct {
	Month       string  `json:"month"`
	Revenue     float64 `json:"revenue"`
	Refunds     float64 `json:"refunds"`
	NetRevenue  float64 `json:"netRevenue"`
	Orders      int64   `json:"orders"`
	Customers   int64   `json:"customers"`
}

// ReturnsAnalysis represents returns/refunds analysis
type ReturnsAnalysis struct {
	TotalReturns      int64   `json:"totalReturns"`
	TotalRefundAmount float64 `json:"totalRefundAmount"`
	ReturnRate        float64 `json:"returnRate"`
	RefundRate        float64 `json:"refundRate"`
	AvgRefundAmount   float64 `json:"avgRefundAmount"`
	TopReturnReasons  []ReturnReason `json:"topReturnReasons"`
	ReturnsByProduct  []ProductReturns `json:"returnsByProduct"`
}

// ReturnReason represents return reason statistics
type ReturnReason struct {
	Reason      string  `json:"reason"`
	Count       int64   `json:"count"`
	Percentage  float64 `json:"percentage"`
	TotalRefund float64 `json:"totalRefund"`
}

// ProductReturns represents returns for a specific product
type ProductReturns struct {
	ProductID     string  `json:"productId"`
	ProductName   string  `json:"productName"`
	ReturnCount   int64   `json:"returnCount"`
	TotalSold     int64   `json:"totalSold"`
	ReturnRate    float64 `json:"returnRate"`
	RefundAmount  float64 `json:"refundAmount"`
}

// PerformanceMetrics represents operational performance metrics
type PerformanceMetrics struct {
	DateRange DateRange `json:"dateRange"`

	// Order fulfillment
	AverageFulfillmentTime float64 `json:"averageFulfillmentTime"` // Hours
	OrderFulfillmentRate   float64 `json:"orderFulfillmentRate"`   // Percentage
	OnTimeDeliveryRate     float64 `json:"onTimeDeliveryRate"`     // Percentage

	// Customer service
	AverageReturnProcessingTime float64 `json:"averageReturnProcessingTime"` // Days
	CustomerSatisfactionScore   float64 `json:"customerSatisfactionScore"`   // 1-5 scale

	// Conversion
	ConversionRate         float64 `json:"conversionRate"`
	CartAbandonmentRate    float64 `json:"cartAbandonmentRate"`
	AverageCartValue       float64 `json:"averageCartValue"`
}

// ExportRequest represents a request to export analytics data
type ExportRequest struct {
	ReportType string    `json:"reportType"` // sales, inventory, customers, financial
	DateRange  DateRange `json:"dateRange"`
	Format     string    `json:"format"` // csv, excel, pdf
	Filters    map[string]interface{} `json:"filters,omitempty"`
}
