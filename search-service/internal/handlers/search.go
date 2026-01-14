package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tesseract-hub/search-service/internal/cache"
	"github.com/tesseract-hub/search-service/internal/clients"
	gosharedmw "github.com/tesseract-hub/go-shared/middleware"
	"github.com/typesense/typesense-go/v2/typesense/api"
)

// SearchHandler handles search operations
type SearchHandler struct {
	client *clients.TypesenseClient
	cache  *cache.SearchCache
}

// NewSearchHandler creates a new SearchHandler
func NewSearchHandler(client *clients.TypesenseClient) *SearchHandler {
	return &SearchHandler{
		client: client,
		cache:  cache.NewSearchCache(),
	}
}

// NewSearchHandlerWithCache creates a SearchHandler with a custom cache
func NewSearchHandlerWithCache(client *clients.TypesenseClient, searchCache *cache.SearchCache) *SearchHandler {
	return &SearchHandler{
		client: client,
		cache:  searchCache,
	}
}

// SearchRequest represents a search request body
// @Description Search request parameters
type SearchRequest struct {
	Query    string   `json:"query" binding:"required" example:"wireless headphones"`
	QueryBy  []string `json:"query_by,omitempty" example:"name,description"`
	FilterBy string   `json:"filter_by,omitempty" example:"price:>100 && in_stock:true"`
	SortBy   string   `json:"sort_by,omitempty" example:"price:asc"`
	FacetBy  []string `json:"facet_by,omitempty" example:"category,brand"`
	Page     int      `json:"page,omitempty" example:"1"`
	PerPage  int      `json:"per_page,omitempty" example:"20"`
}

// MultiSearchRequest represents a multi-search request
// @Description Multi-search request for parallel searches across collections
type MultiSearchRequest struct {
	Searches []SearchRequestWithCollection `json:"searches" binding:"required"`
}

// SearchRequestWithCollection is a search request with collection info
// @Description Search request for a specific collection
type SearchRequestWithCollection struct {
	Collection string   `json:"collection" binding:"required" example:"products"`
	Query      string   `json:"query" binding:"required" example:"laptop"`
	QueryBy    []string `json:"query_by,omitempty" example:"name,description"`
	FilterBy   string   `json:"filter_by,omitempty" example:"price:>500"`
	SortBy     string   `json:"sort_by,omitempty" example:"price:desc"`
	FacetBy    []string `json:"facet_by,omitempty" example:"brand"`
	Page       int      `json:"page,omitempty" example:"1"`
	PerPage    int      `json:"per_page,omitempty" example:"10"`
}

// SearchResponse represents a standard search response
// @Description Standard API response wrapper
type SearchResponse struct {
	Success bool        `json:"success" example:"true"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
}

// ErrorInfo represents error details
// @Description Error information
type ErrorInfo struct {
	Code    string `json:"code" example:"INVALID_REQUEST"`
	Message string `json:"message" example:"Query parameter is required"`
}

// SuggestResponse represents autocomplete response
// @Description Autocomplete/suggestion response
type SuggestResponse struct {
	Success bool `json:"success" example:"true"`
	Data    struct {
		Suggestions []map[string]interface{} `json:"suggestions"`
		Found       int                      `json:"found" example:"5"`
	} `json:"data"`
}

// buildTenantFilter creates a filter that includes tenant isolation
func buildTenantFilter(tenantID, additionalFilter string) string {
	tenantFilter := fmt.Sprintf("tenant_id:=%s", tenantID)
	if additionalFilter != "" {
		return fmt.Sprintf("%s && %s", tenantFilter, additionalFilter)
	}
	return tenantFilter
}

// GlobalSearch handles global search across all collections
// @Summary Global search across all collections
// @Description Search across products, customers, orders, and categories simultaneously with tenant isolation
// @Tags Search
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body SearchRequest true "Search request parameters"
// @Success 200 {object} SearchResponse "Search results from all collections"
// @Failure 400 {object} SearchResponse "Invalid request or missing tenant"
// @Failure 401 {object} SearchResponse "Unauthorized"
// @Failure 500 {object} SearchResponse "Internal server error"
// @Router /search [post]
func (h *SearchHandler) GlobalSearch(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	tenantID := gosharedmw.GetVendorID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TENANT_REQUIRED",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	ctx := c.Request.Context()

	// Search across all collections
	collections := []string{"products", "customers", "orders", "categories"}
	results := make(map[string]interface{})

	for _, collection := range collections {
		queryBy := getQueryByForCollection(collection)
		params := buildSearchParams(req.Query, queryBy, buildTenantFilter(tenantID, req.FilterBy), req.SortBy, 5)

		result, err := h.client.Search(ctx, collection, params)
		if err != nil {
			results[collection] = gin.H{"error": err.Error(), "found": 0}
			continue
		}
		results[collection] = result
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}

// GlobalSearchGet handles global search via GET
// @Summary Global search via GET
// @Description Search across all collections using query parameters
// @Tags Search
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param q query string true "Search query" example("wireless")
// @Param filter_by query string false "Filter expression" example("price:>100")
// @Param sort_by query string false "Sort expression" example("price:asc")
// @Success 200 {object} SearchResponse "Search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Failure 401 {object} SearchResponse "Unauthorized"
// @Router /search [get]
func (h *SearchHandler) GlobalSearchGet(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "QUERY_REQUIRED",
				"message": "Query parameter 'q' is required",
			},
		})
		return
	}

	req := SearchRequest{
		Query:    query,
		FilterBy: c.Query("filter_by"),
		SortBy:   c.Query("sort_by"),
	}

	c.Set("search_request", req)
	h.GlobalSearch(c)
}

// SearchProducts searches the products collection
// @Summary Search products
// @Description Search products with full-text search, filtering, and faceting
// @Tags Search
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body SearchRequest true "Search request"
// @Success 200 {object} SearchResponse "Product search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Failure 401 {object} SearchResponse "Unauthorized"
// @Failure 500 {object} SearchResponse "Search error"
// @Router /search/products [post]
func (h *SearchHandler) SearchProducts(c *gin.Context) {
	h.searchCollection(c, "products", []string{"name", "description", "sku", "brand", "tags"})
}

// SearchProductsGet searches products via GET
// @Summary Search products via GET
// @Description Search products using query parameters
// @Tags Search
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param q query string true "Search query" example("laptop")
// @Param filter_by query string false "Filter expression" example("brand:=Apple")
// @Param sort_by query string false "Sort expression" example("price:asc")
// @Param facet_by query string false "Facet fields" example("brand,category")
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Results per page" default(20)
// @Success 200 {object} SearchResponse "Product search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Failure 401 {object} SearchResponse "Unauthorized"
// @Router /search/products [get]
func (h *SearchHandler) SearchProductsGet(c *gin.Context) {
	h.searchCollectionGet(c, "products", []string{"name", "description", "sku", "brand", "tags"})
}

// SearchCustomers searches the customers collection
// @Summary Search customers
// @Description Search customers by name, email, phone, or company
// @Tags Search
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body SearchRequest true "Search request"
// @Success 200 {object} SearchResponse "Customer search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Failure 401 {object} SearchResponse "Unauthorized"
// @Failure 500 {object} SearchResponse "Search error"
// @Router /search/customers [post]
func (h *SearchHandler) SearchCustomers(c *gin.Context) {
	h.searchCollection(c, "customers", []string{"name", "email", "phone", "company"})
}

// SearchCustomersGet searches customers via GET
// @Summary Search customers via GET
// @Description Search customers using query parameters
// @Tags Search
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param q query string true "Search query" example("john@example.com")
// @Param filter_by query string false "Filter expression"
// @Param sort_by query string false "Sort expression"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Results per page" default(20)
// @Success 200 {object} SearchResponse "Customer search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Router /search/customers [get]
func (h *SearchHandler) SearchCustomersGet(c *gin.Context) {
	h.searchCollectionGet(c, "customers", []string{"name", "email", "phone", "company"})
}

// SearchOrders searches the orders collection
// @Summary Search orders
// @Description Search orders by order number, customer name, email, or items
// @Tags Search
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body SearchRequest true "Search request"
// @Success 200 {object} SearchResponse "Order search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Failure 401 {object} SearchResponse "Unauthorized"
// @Failure 500 {object} SearchResponse "Search error"
// @Router /search/orders [post]
func (h *SearchHandler) SearchOrders(c *gin.Context) {
	h.searchCollection(c, "orders", []string{"order_number", "customer_name", "customer_email", "items"})
}

// SearchOrdersGet searches orders via GET
// @Summary Search orders via GET
// @Description Search orders using query parameters
// @Tags Search
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param q query string true "Search query" example("ORD-12345")
// @Param filter_by query string false "Filter expression" example("status:=completed")
// @Param sort_by query string false "Sort expression" example("created_at:desc")
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Results per page" default(20)
// @Success 200 {object} SearchResponse "Order search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Router /search/orders [get]
func (h *SearchHandler) SearchOrdersGet(c *gin.Context) {
	h.searchCollectionGet(c, "orders", []string{"order_number", "customer_name", "customer_email", "items"})
}

// SearchCategories searches the categories collection
// @Summary Search categories
// @Description Search categories by name, slug, or description
// @Tags Search
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body SearchRequest true "Search request"
// @Success 200 {object} SearchResponse "Category search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Failure 401 {object} SearchResponse "Unauthorized"
// @Failure 500 {object} SearchResponse "Search error"
// @Router /search/categories [post]
func (h *SearchHandler) SearchCategories(c *gin.Context) {
	h.searchCollection(c, "categories", []string{"name", "slug", "description"})
}

// SearchCategoriesGet searches categories via GET
// @Summary Search categories via GET
// @Description Search categories using query parameters
// @Tags Search
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param q query string true "Search query" example("electronics")
// @Param filter_by query string false "Filter expression"
// @Param sort_by query string false "Sort expression"
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Results per page" default(20)
// @Success 200 {object} SearchResponse "Category search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Router /search/categories [get]
func (h *SearchHandler) SearchCategoriesGet(c *gin.Context) {
	h.searchCollectionGet(c, "categories", []string{"name", "slug", "description"})
}

// searchCollection performs a search on a specific collection
func (h *SearchHandler) searchCollection(c *gin.Context, collection string, defaultQueryBy []string) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	tenantID := gosharedmw.GetVendorID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TENANT_REQUIRED",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	queryBy := defaultQueryBy
	if len(req.QueryBy) > 0 {
		queryBy = req.QueryBy
	}

	perPage := 20
	if req.PerPage > 0 && req.PerPage <= 100 {
		perPage = req.PerPage
	}

	page := 1
	if req.Page > 0 {
		page = req.Page
	}

	// Check cache first
	cacheFilters := map[string]interface{}{
		"filter_by": req.FilterBy,
		"sort_by":   req.SortBy,
		"facet_by":  req.FacetBy,
		"page":      page,
		"per_page":  perPage,
	}
	if cachedResult, found := h.cache.GetSearchResult(collection, tenantID, req.Query, cacheFilters); found {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    cachedResult,
			"cached":  true,
		})
		return
	}

	q := req.Query
	queryByStr := joinStrings(queryBy)
	filterBy := buildTenantFilter(tenantID, req.FilterBy)

	params := &api.SearchCollectionParams{
		Q:        &q,
		QueryBy:  &queryByStr,
		FilterBy: &filterBy,
		Page:     &page,
		PerPage:  &perPage,
	}

	if req.SortBy != "" {
		params.SortBy = &req.SortBy
	}

	if len(req.FacetBy) > 0 {
		facetBy := joinStrings(req.FacetBy)
		params.FacetBy = &facetBy
	}

	ctx := c.Request.Context()
	result, err := h.client.Search(ctx, collection, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SEARCH_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	// Cache the result
	h.cache.CacheSearchResult(collection, tenantID, req.Query, cacheFilters, result)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
		"cached":  false,
	})
}

// GetCacheStats returns cache statistics
// @Summary Get cache statistics
// @Description Get search cache statistics
// @Tags Admin
// @Produce json
// @Success 200 {object} map[string]interface{} "Cache statistics"
// @Router /admin/cache/stats [get]
func (h *SearchHandler) GetCacheStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    h.cache.Stats(),
	})
}

// ClearCache clears the search cache
// @Summary Clear search cache
// @Description Clear all cached search results
// @Tags Admin
// @Produce json
// @Success 200 {object} map[string]interface{} "Cache cleared"
// @Router /admin/cache/clear [post]
func (h *SearchHandler) ClearCache(c *gin.Context) {
	h.cache.Clear()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cache cleared",
	})
}

// searchCollectionGet performs a search via GET parameters
func (h *SearchHandler) searchCollectionGet(c *gin.Context, collection string, defaultQueryBy []string) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "QUERY_REQUIRED",
				"message": "Query parameter 'q' is required",
			},
		})
		return
	}

	tenantID := gosharedmw.GetVendorID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TENANT_REQUIRED",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	perPage := 20
	if pp := c.Query("per_page"); pp != "" {
		if p, err := strconv.Atoi(pp); err == nil && p > 0 && p <= 100 {
			perPage = p
		}
	}

	page := 1
	if pg := c.Query("page"); pg != "" {
		if p, err := strconv.Atoi(pg); err == nil && p > 0 {
			page = p
		}
	}

	queryByStr := joinStrings(defaultQueryBy)
	filterBy := buildTenantFilter(tenantID, c.Query("filter_by"))

	params := &api.SearchCollectionParams{
		Q:        &query,
		QueryBy:  &queryByStr,
		FilterBy: &filterBy,
		Page:     &page,
		PerPage:  &perPage,
	}

	if sortBy := c.Query("sort_by"); sortBy != "" {
		params.SortBy = &sortBy
	}

	if facetBy := c.Query("facet_by"); facetBy != "" {
		params.FacetBy = &facetBy
	}

	ctx := c.Request.Context()
	result, err := h.client.Search(ctx, collection, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SEARCH_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// MultiSearch handles parallel multi-search requests
// @Summary Multi-search across collections
// @Description Execute multiple search queries in parallel across different collections
// @Tags Search
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body MultiSearchRequest true "Multi-search request with array of searches"
// @Success 200 {object} SearchResponse "Multi-search results"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Failure 401 {object} SearchResponse "Unauthorized"
// @Failure 500 {object} SearchResponse "Search error"
// @Router /search/multi [post]
func (h *SearchHandler) MultiSearch(c *gin.Context) {
	var req MultiSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	tenantID := gosharedmw.GetVendorID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TENANT_REQUIRED",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	// Build multi-search parameters
	searches := make([]api.MultiSearchCollectionParameters, len(req.Searches))
	for i, search := range req.Searches {
		queryBy := getQueryByForCollection(search.Collection)
		if len(search.QueryBy) > 0 {
			queryBy = search.QueryBy
		}

		perPage := 20
		if search.PerPage > 0 && search.PerPage <= 100 {
			perPage = search.PerPage
		}

		page := 1
		if search.Page > 0 {
			page = search.Page
		}

		q := search.Query
		queryByStr := joinStrings(queryBy)
		filterBy := buildTenantFilter(tenantID, search.FilterBy)

		searches[i] = api.MultiSearchCollectionParameters{
			Collection: search.Collection,
			Q:          &q,
			QueryBy:    &queryByStr,
			FilterBy:   &filterBy,
			Page:       &page,
			PerPage:    &perPage,
		}

		if search.SortBy != "" {
			searches[i].SortBy = &search.SortBy
		}
	}

	params := &api.MultiSearchParams{}
	searchesParam := api.MultiSearchSearchesParameter{
		Searches: searches,
	}

	ctx := c.Request.Context()
	result, err := h.client.MultiSearch(ctx, params, searchesParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "MULTI_SEARCH_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// Suggest handles autocomplete/suggestion requests
// @Summary Autocomplete suggestions
// @Description Get autocomplete suggestions for a search query with prefix matching
// @Tags Search
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param q query string true "Search prefix query" example("wire")
// @Param collection query string false "Collection to search" default(products) Enums(products, customers, orders, categories)
// @Success 200 {object} SuggestResponse "Autocomplete suggestions"
// @Failure 400 {object} SearchResponse "Invalid request"
// @Failure 401 {object} SearchResponse "Unauthorized"
// @Failure 500 {object} SearchResponse "Suggestion error"
// @Router /search/suggest [get]
func (h *SearchHandler) Suggest(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "QUERY_REQUIRED",
				"message": "Query parameter 'q' is required",
			},
		})
		return
	}

	collection := c.DefaultQuery("collection", "products")
	tenantID := gosharedmw.GetVendorID(c)
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "TENANT_REQUIRED",
				"message": "Tenant ID is required",
			},
		})
		return
	}

	queryBy := getQueryByForCollection(collection)
	queryByStr := joinStrings(queryBy)
	filterBy := buildTenantFilter(tenantID, "")
	page := 1
	perPage := 10
	prefix := "true"

	params := &api.SearchCollectionParams{
		Q:        &query,
		QueryBy:  &queryByStr,
		FilterBy: &filterBy,
		Page:     &page,
		PerPage:  &perPage,
		Prefix:   &prefix,
	}

	ctx := c.Request.Context()
	result, err := h.client.Search(ctx, collection, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SUGGEST_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	// Extract just the suggestions
	suggestions := make([]map[string]interface{}, 0)
	if result.Hits != nil {
		for _, hit := range *result.Hits {
			if hit.Document != nil {
				suggestions = append(suggestions, *hit.Document)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"suggestions": suggestions,
			"found":       result.Found,
		},
	})
}

// Helper functions
func getQueryByForCollection(collection string) []string {
	switch collection {
	case "products":
		return []string{"name", "description", "sku", "brand", "tags"}
	case "customers":
		return []string{"name", "email", "phone", "company"}
	case "orders":
		return []string{"order_number", "customer_name", "customer_email", "items"}
	case "categories":
		return []string{"name", "slug", "description"}
	default:
		return []string{"name"}
	}
}

func buildSearchParams(query string, queryBy []string, filterBy string, sortBy string, perPage int) *api.SearchCollectionParams {
	queryByStr := joinStrings(queryBy)
	page := 1

	params := &api.SearchCollectionParams{
		Q:        &query,
		QueryBy:  &queryByStr,
		FilterBy: &filterBy,
		Page:     &page,
		PerPage:  &perPage,
	}
	if sortBy != "" {
		params.SortBy = &sortBy
	}
	return params
}

func joinStrings(s []string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for i := 1; i < len(s); i++ {
		result += "," + s[i]
	}
	return result
}
