package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"search-service/internal/clients"
	"search-service/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("sync-service")

// SyncService handles synchronization of data from backend services to Typesense
type SyncService struct {
	typesenseClient *clients.TypesenseClient
	redisClient     *redis.Client
	config          *config.Config
	httpClient      *http.Client
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Collection    string    `json:"collection"`
	TenantID      string    `json:"tenantId"`
	StartedAt     time.Time `json:"startedAt"`
	CompletedAt   time.Time `json:"completedAt"`
	Duration      string    `json:"duration"`
	TotalFetched  int       `json:"totalFetched"`
	TotalIndexed  int       `json:"totalIndexed"`
	TotalFailed   int       `json:"totalFailed"`
	TotalDeleted  int       `json:"totalDeleted"`
	IsIncremental bool      `json:"isIncremental"`
	Error         string    `json:"error,omitempty"`
	Errors        []string  `json:"errors,omitempty"`
}

// NewSyncService creates a new SyncService
func NewSyncService(typesenseClient *clients.TypesenseClient, redisClient *redis.Client, cfg *config.Config) *SyncService {
	return &SyncService{
		typesenseClient: typesenseClient,
		redisClient:     redisClient,
		config:          cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.SyncTimeout) * time.Second,
		},
	}
}

// SyncProducts synchronizes all products from products-service
func (s *SyncService) SyncProducts(ctx context.Context, tenantID, authToken string) (*SyncResult, error) {
	ctx, span := tracer.Start(ctx, "SyncProducts")
	defer span.End()

	span.SetAttributes(attribute.String("tenant_id", tenantID))

	result := &SyncResult{
		Collection: "products",
		TenantID:   tenantID,
		StartedAt:  time.Now(),
	}

	// Get last sync time from Redis for incremental sync
	var updatedAfter *time.Time
	redisKey := fmt.Sprintf("sync:products:last_run:%s", tenantID)
	if lastSyncStr, err := s.redisClient.Get(ctx, redisKey).Result(); err == nil && lastSyncStr != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncStr); err == nil {
			updatedAfter = &t
		}
	}

	// Fetch products from products-service with pagination
	products, err := s.fetchAllProducts(ctx, tenantID, authToken, updatedAfter)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	result.TotalFetched = len(products)

	// Transform and index products in batches
	if len(products) > 0 {
		indexed, failed, errors := s.indexDocuments(ctx, "products", products)
		result.TotalIndexed = indexed
		result.TotalFailed = failed
		result.Errors = append(result.Errors, errors...)
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt).String()

	// Update last sync time in Redis if successful
	if result.TotalFailed == 0 && len(result.Errors) == 0 {
		s.redisClient.Set(ctx, redisKey, result.StartedAt.Format(time.RFC3339), 0)
	}

	return result, nil
}

// SyncCustomers synchronizes all customers from customers-service
func (s *SyncService) SyncCustomers(ctx context.Context, tenantID, authToken string) (*SyncResult, error) {
	ctx, span := tracer.Start(ctx, "SyncCustomers")
	defer span.End()

	span.SetAttributes(attribute.String("tenant_id", tenantID))

	result := &SyncResult{
		Collection: "customers",
		TenantID:   tenantID,
		StartedAt:  time.Now(),
	}

	// Fetch customers from customers-service
	customers, err := s.fetchAllCustomers(ctx, tenantID, authToken)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	result.TotalFetched = len(customers)

	// Transform and index customers in batches
	if len(customers) > 0 {
		indexed, failed, errors := s.indexDocuments(ctx, "customers", customers)
		result.TotalIndexed = indexed
		result.TotalFailed = failed
		result.Errors = append(result.Errors, errors...)
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt).String()

	return result, nil
}

// SyncOrders synchronizes all orders from orders-service
func (s *SyncService) SyncOrders(ctx context.Context, tenantID, authToken string) (*SyncResult, error) {
	ctx, span := tracer.Start(ctx, "SyncOrders")
	defer span.End()

	span.SetAttributes(attribute.String("tenant_id", tenantID))

	result := &SyncResult{
		Collection: "orders",
		TenantID:   tenantID,
		StartedAt:  time.Now(),
	}

	// Fetch orders from orders-service
	orders, err := s.fetchAllOrders(ctx, tenantID, authToken)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	result.TotalFetched = len(orders)

	// Transform and index orders in batches
	if len(orders) > 0 {
		indexed, failed, errors := s.indexDocuments(ctx, "orders", orders)
		result.TotalIndexed = indexed
		result.TotalFailed = failed
		result.Errors = append(result.Errors, errors...)
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt).String()

	return result, nil
}

// SyncCategories synchronizes all categories from products-service
func (s *SyncService) SyncCategories(ctx context.Context, tenantID, authToken string) (*SyncResult, error) {
	ctx, span := tracer.Start(ctx, "SyncCategories")
	defer span.End()

	span.SetAttributes(attribute.String("tenant_id", tenantID))

	result := &SyncResult{
		Collection: "categories",
		TenantID:   tenantID,
		StartedAt:  time.Now(),
	}

	// Fetch categories from products-service
	categories, err := s.fetchAllCategories(ctx, tenantID, authToken)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	result.TotalFetched = len(categories)

	// Transform and index categories in batches
	if len(categories) > 0 {
		indexed, failed, errors := s.indexDocuments(ctx, "categories", categories)
		result.TotalIndexed = indexed
		result.TotalFailed = failed
		result.Errors = append(result.Errors, errors...)
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt).String()

	return result, nil
}

// fetchAllProducts fetches all products from products-service with pagination
func (s *SyncService) fetchAllProducts(ctx context.Context, tenantID, authToken string, updatedAfter *time.Time) ([]map[string]interface{}, error) {
	var allProducts []map[string]interface{}
	page := 1
	limit := s.config.SyncBatchSize

	for {
		url := fmt.Sprintf("%s/api/v1/products?page=%d&limit=%d", s.config.ProductsServiceURL, page, limit)
		if updatedAfter != nil {
			url += fmt.Sprintf("&updatedAfter=%s", updatedAfter.Format(time.RFC3339))
		}

		data, err := s.makeRequest(ctx, url, tenantID, authToken)
		if err != nil {
			return allProducts, fmt.Errorf("failed to fetch products page %d: %w", page, err)
		}

		// Parse response
		products, hasMore := s.parseProductsResponse(data, tenantID)
		allProducts = append(allProducts, products...)

		if !hasMore || len(products) < limit {
			break
		}
		page++
	}

	return allProducts, nil
}

// fetchAllCustomers fetches all customers from customers-service with pagination
func (s *SyncService) fetchAllCustomers(ctx context.Context, tenantID, authToken string) ([]map[string]interface{}, error) {
	var allCustomers []map[string]interface{}
	page := 1
	limit := s.config.SyncBatchSize

	for {
		url := fmt.Sprintf("%s/api/v1/customers?page=%d&limit=%d", s.config.CustomersServiceURL, page, limit)

		data, err := s.makeRequest(ctx, url, tenantID, authToken)
		if err != nil {
			return allCustomers, fmt.Errorf("failed to fetch customers page %d: %w", page, err)
		}

		// Parse response
		customers, hasMore := s.parseCustomersResponse(data, tenantID)
		allCustomers = append(allCustomers, customers...)

		if !hasMore || len(customers) < limit {
			break
		}
		page++
	}

	return allCustomers, nil
}

// fetchAllOrders fetches all orders from orders-service with pagination
func (s *SyncService) fetchAllOrders(ctx context.Context, tenantID, authToken string) ([]map[string]interface{}, error) {
	var allOrders []map[string]interface{}
	page := 1
	limit := s.config.SyncBatchSize

	for {
		url := fmt.Sprintf("%s/api/v1/orders?page=%d&limit=%d", s.config.OrdersServiceURL, page, limit)

		data, err := s.makeRequest(ctx, url, tenantID, authToken)
		if err != nil {
			return allOrders, fmt.Errorf("failed to fetch orders page %d: %w", page, err)
		}

		// Parse response
		orders, hasMore := s.parseOrdersResponse(data, tenantID)
		allOrders = append(allOrders, orders...)

		if !hasMore || len(orders) < limit {
			break
		}
		page++
	}

	return allOrders, nil
}

// fetchAllCategories fetches all categories from products-service
func (s *SyncService) fetchAllCategories(ctx context.Context, tenantID, authToken string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/categories", s.config.CategoriesServiceURL)

	data, err := s.makeRequest(ctx, url, tenantID, authToken)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}

	// Parse response
	categories, _ := s.parseCategoriesResponse(data, tenantID)
	return categories, nil
}

// makeRequest makes an HTTP request to a backend service
func (s *SyncService) makeRequest(ctx context.Context, url, tenantID, authToken string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vendor-ID", tenantID)
	req.Header.Set("X-Tenant-ID", tenantID)
	if authToken != "" {
		if !strings.HasPrefix(authToken, "Bearer ") {
			authToken = "Bearer " + authToken
		}
		req.Header.Set("Authorization", authToken)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// parseProductsResponse parses the products API response and transforms to search documents
func (s *SyncService) parseProductsResponse(data map[string]interface{}, tenantID string) ([]map[string]interface{}, bool) {
	var documents []map[string]interface{}
	hasMore := false

	// Check for pagination
	if pagination, ok := data["pagination"].(map[string]interface{}); ok {
		if page, ok := pagination["page"].(float64); ok {
			if totalPages, ok := pagination["total_pages"].(float64); ok {
				hasMore = page < totalPages
			}
		}
	}

	// Extract products from data array
	var products []interface{}
	if dataArr, ok := data["data"].([]interface{}); ok {
		products = dataArr
	} else if productsArr, ok := data["products"].([]interface{}); ok {
		products = productsArr
	}

	for _, p := range products {
		product, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		// Handle currency - check both snake_case and camelCase
		currency := getString(product, "currency")
		if currency == "" {
			currency = getString(product, "currencyCode")
		}
		if currency == "" {
			currency = "USD" // Default
		}

		// Handle in_stock - check quantity or status
		inStock := getBool(product, "in_stock")
		if !inStock {
			if qty := getFloat(product, "quantity"); qty > 0 {
				inStock = true
			}
			if status := getString(product, "status"); status == "active" || status == "ACTIVE" {
				inStock = true
			}
		}

		// Handle timestamps - check both snake_case and camelCase
		createdAt := getTimestamp(product, "created_at")
		if createdAt == 0 {
			createdAt = getTimestamp(product, "createdAt")
		}
		updatedAt := getTimestamp(product, "updated_at")
		if updatedAt == 0 {
			updatedAt = getTimestamp(product, "updatedAt")
		}

		// Handle sale_price - check comparePrice as well
		salePrice := getFloat(product, "sale_price")
		if salePrice == 0 {
			salePrice = getFloat(product, "comparePrice")
		}

		doc := map[string]interface{}{
			"id":         getString(product, "id"),
			"tenant_id":  tenantID,
			"name":       getString(product, "name"),
			"description": getString(product, "description"),
			"sku":        getString(product, "sku"),
			"brand":      getString(product, "brand"),
			"price":      getFloat(product, "price"),
			"sale_price": salePrice,
			"currency":   currency,
			"in_stock":   inStock,
			"image_url":  getString(product, "image_url"),
			"created_at": createdAt,
			"updated_at": updatedAt,
		}

		// Handle categories as array - check multiple field names (camelCase and snake_case)
		if categories := getStringArray(product, "categories"); len(categories) > 0 {
			doc["category"] = categories
		} else if category := getString(product, "category"); category != "" {
			doc["category"] = []string{category}
		} else if categoryID := getString(product, "category_id"); categoryID != "" {
			doc["category"] = []string{categoryID}
		} else if categoryID := getString(product, "categoryId"); categoryID != "" {
			// Handle camelCase from API response
			doc["category"] = []string{categoryID}
		} else {
			// Default empty array to satisfy schema requirement
			doc["category"] = []string{}
		}

		// Handle tags as array
		if tags := getStringArray(product, "tags"); len(tags) > 0 {
			doc["tags"] = tags
		}

		// Handle images - get first image URL if available
		if images, ok := product["images"].([]interface{}); ok && len(images) > 0 {
			if img, ok := images[0].(map[string]interface{}); ok {
				if url := getString(img, "url"); url != "" {
					doc["image_url"] = url
				}
			}
		}

		documents = append(documents, doc)
	}

	return documents, hasMore
}

// parseCustomersResponse parses the customers API response and transforms to search documents
func (s *SyncService) parseCustomersResponse(data map[string]interface{}, tenantID string) ([]map[string]interface{}, bool) {
	var documents []map[string]interface{}
	hasMore := false

	// Check for pagination
	if pagination, ok := data["pagination"].(map[string]interface{}); ok {
		if page, ok := pagination["page"].(float64); ok {
			if totalPages, ok := pagination["total_pages"].(float64); ok {
				hasMore = page < totalPages
			}
		}
	}

	// Extract customers from data array
	var customers []interface{}
	if dataArr, ok := data["data"].([]interface{}); ok {
		customers = dataArr
	} else if customersArr, ok := data["customers"].([]interface{}); ok {
		customers = customersArr
	}

	for _, c := range customers {
		customer, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		// Build full name
		firstName := getString(customer, "first_name")
		lastName := getString(customer, "last_name")
		name := getString(customer, "name")
		if name == "" && (firstName != "" || lastName != "") {
			name = strings.TrimSpace(firstName + " " + lastName)
		}

		doc := map[string]interface{}{
			"id":           getString(customer, "id"),
			"tenant_id":    tenantID,
			"name":         name,
			"email":        getString(customer, "email"),
			"phone":        getString(customer, "phone"),
			"company":      getString(customer, "company"),
			"total_orders": getInt(customer, "total_orders"),
			"total_spent":  getFloat(customer, "total_spent"),
			"status":       getString(customer, "status"),
			"created_at":   getTimestamp(customer, "created_at"),
		}

		documents = append(documents, doc)
	}

	return documents, hasMore
}

// parseOrdersResponse parses the orders API response and transforms to search documents
func (s *SyncService) parseOrdersResponse(data map[string]interface{}, tenantID string) ([]map[string]interface{}, bool) {
	var documents []map[string]interface{}
	hasMore := false

	// Check for pagination
	if pagination, ok := data["pagination"].(map[string]interface{}); ok {
		if page, ok := pagination["page"].(float64); ok {
			if totalPages, ok := pagination["total_pages"].(float64); ok {
				hasMore = page < totalPages
			}
		}
	}

	// Extract orders from data array
	var orders []interface{}
	if dataArr, ok := data["data"].([]interface{}); ok {
		orders = dataArr
	} else if ordersArr, ok := data["orders"].([]interface{}); ok {
		orders = ordersArr
	}

	for _, o := range orders {
		order, ok := o.(map[string]interface{})
		if !ok {
			continue
		}

		// Get customer info
		customerName := ""
		customerEmail := ""
		if customer, ok := order["customer"].(map[string]interface{}); ok {
			customerName = getString(customer, "name")
			if customerName == "" {
				firstName := getString(customer, "first_name")
				lastName := getString(customer, "last_name")
				customerName = strings.TrimSpace(firstName + " " + lastName)
			}
			customerEmail = getString(customer, "email")
		}

		// Get items as string array
		var items []string
		if orderItems, ok := order["items"].([]interface{}); ok {
			for _, item := range orderItems {
				if itemMap, ok := item.(map[string]interface{}); ok {
					itemName := getString(itemMap, "name")
					if itemName == "" {
						if product, ok := itemMap["product"].(map[string]interface{}); ok {
							itemName = getString(product, "name")
						}
					}
					if itemName != "" {
						items = append(items, itemName)
					}
				}
			}
		}

		doc := map[string]interface{}{
			"id":             getString(order, "id"),
			"tenant_id":      tenantID,
			"order_number":   getString(order, "order_number"),
			"customer_name":  customerName,
			"customer_email": customerEmail,
			"total":          getFloat(order, "total"),
			"currency":       getString(order, "currency"),
			"status":         getString(order, "status"),
			"items":          items,
			"created_at":     getTimestamp(order, "created_at"),
		}

		documents = append(documents, doc)
	}

	return documents, hasMore
}

// parseCategoriesResponse parses the categories API response and transforms to search documents
func (s *SyncService) parseCategoriesResponse(data map[string]interface{}, tenantID string) ([]map[string]interface{}, bool) {
	var documents []map[string]interface{}

	// Extract categories from data array
	var categories []interface{}
	if dataArr, ok := data["data"].([]interface{}); ok {
		categories = dataArr
	} else if categoriesArr, ok := data["categories"].([]interface{}); ok {
		categories = categoriesArr
	}

	// Process categories recursively to handle nested structure
	s.flattenCategories(categories, tenantID, &documents, 0)

	return documents, false // Categories typically don't use pagination
}

// flattenCategories recursively flattens nested category structure
func (s *SyncService) flattenCategories(categories []interface{}, tenantID string, documents *[]map[string]interface{}, level int) {
	for _, c := range categories {
		category, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		doc := map[string]interface{}{
			"id":            getString(category, "id"),
			"tenant_id":     tenantID,
			"name":          getString(category, "name"),
			"slug":          getString(category, "slug"),
			"description":   getString(category, "description"),
			"parent_id":     getString(category, "parent_id"),
			"level":         int32(level),
			"product_count": getInt(category, "product_count"),
		}

		*documents = append(*documents, doc)

		// Process children recursively
		if children, ok := category["children"].([]interface{}); ok {
			s.flattenCategories(children, tenantID, documents, level+1)
		}
	}
}

// indexDocuments indexes documents in batches
func (s *SyncService) indexDocuments(ctx context.Context, collection string, documents []map[string]interface{}) (int, int, []string) {
	var errors []string
	indexed := 0
	failed := 0

	// Process in batches
	batchSize := s.config.SyncBatchSize
	for i := 0; i < len(documents); i += batchSize {
		end := i + batchSize
		if end > len(documents) {
			end = len(documents)
		}

		batch := documents[i:end]

		// Convert to []interface{} for ImportDocuments
		docs := make([]interface{}, len(batch))
		for j, doc := range batch {
			docs[j] = doc
		}

		results, err := s.typesenseClient.ImportDocuments(ctx, collection, docs, "upsert")
		if err != nil {
			errors = append(errors, fmt.Sprintf("batch %d failed: %v", i/batchSize+1, err))
			failed += len(batch)
			continue
		}

		for _, r := range results {
			if r.Success {
				indexed++
			} else {
				failed++
				if r.Error != "" {
					errors = append(errors, r.Error)
				}
			}
		}
	}

	return indexed, failed, errors
}

// Helper functions for type conversion
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	if v, ok := m[key].(int); ok {
		return float64(v)
	}
	return 0
}

func getInt(m map[string]interface{}, key string) int32 {
	if v, ok := m[key].(float64); ok {
		return int32(v)
	}
	if v, ok := m[key].(int); ok {
		return int32(v)
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getTimestamp(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.Unix()
		}
	}
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return time.Now().Unix()
}

func getStringArray(m map[string]interface{}, key string) []string {
	var result []string
	if arr, ok := m[key].([]interface{}); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
	}
	return result
}
