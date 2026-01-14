# Analytics Service

Business analytics and reporting microservice for the Tesseract Hub platform. Provides comprehensive dashboards for sales, inventory, customers, and financial metrics.

## Features

- **Sales Analytics**: Revenue, orders, AOV, top products and categories
- **Inventory Reports**: Stock levels, low stock alerts, turnover metrics
- **Customer Analytics**: Segmentation, retention, lifetime value, cohorts
- **Financial Reports**: Revenue breakdown, margins, refunds analysis
- **Export Capabilities**: CSV and JSON export for all reports

## Tech Stack

- **Language**: Go
- **Framework**: Gin
- **Database**: PostgreSQL with GORM
- **Logging**: Logrus

## API Endpoints

### Analytics
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/analytics/sales` | Sales dashboard metrics |
| GET | `/api/v1/analytics/inventory` | Inventory report |
| GET | `/api/v1/analytics/customers` | Customer analytics |
| GET | `/api/v1/analytics/financial` | Financial report |

### Export
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/analytics/sales/export` | Export sales data |
| GET | `/api/v1/analytics/inventory/export` | Export inventory data |
| GET | `/api/v1/analytics/customers/export` | Export customer data |
| GET | `/api/v1/analytics/financial/export` | Export financial data |

## Query Parameters

### Date Range Presets
- `today`, `yesterday`
- `last7days`, `last30days`
- `thisMonth`, `lastMonth`, `thisYear`

### Custom Date Range
- `from`: RFC3339 formatted start date
- `to`: RFC3339 formatted end date

### Export Format
- `format`: `json` or `csv` (default: `json`)

## Sales Dashboard Metrics

- Total revenue, orders, and items sold
- Average order value (AOV)
- Period-over-period comparisons
- Daily revenue breakdown
- Top 10 products by revenue
- Top 10 categories by revenue
- Revenue by order status
- Payment method statistics

## Inventory Report Metrics

- Total products, SKUs, inventory value
- Low stock and out-of-stock counts
- Top 20 low stock products
- Top 20 out-of-stock products
- Inventory by category
- Inventory turnover metrics
- Fast/slow moving items

## Customer Analytics Metrics

- Total, new, returning customers
- Average lifetime value (ALV)
- Orders per customer
- Retention rates
- Top 20 customers by spending
- Customer segmentation
- Growth trends
- Geographic distribution
- Cohort analysis

## Financial Report Metrics

- Gross and net revenue
- Discounts, refunds, shipping costs
- Tax collected and processing fees
- Gross profit and margin
- Revenue by category
- Revenue by payment method
- Monthly trends
- Returns analysis with reasons

## Environment Variables

```env
# Database (inherited from main service)
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=tesseract_hub

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

## Data Sources

The service reads from these tables (read-only):
- `orders` - Order data
- `order_items` - Line items
- `products` - Product catalog
- `customers` - Customer data
- `categories` - Category hierarchy
- `returns` - Return/refund data

## Running Locally

```bash
# Service is typically embedded in main application
# Run with the main service

go run cmd/main.go
```

## License

MIT
