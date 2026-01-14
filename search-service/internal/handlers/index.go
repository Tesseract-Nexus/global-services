package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"search-service/internal/clients"
	"search-service/internal/services"
	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
)

// IndexHandler handles document indexing operations
type IndexHandler struct {
	client      *clients.TypesenseClient
	syncService *services.SyncService
}

// NewIndexHandler creates a new IndexHandler
func NewIndexHandler(client *clients.TypesenseClient, syncService *services.SyncService) *IndexHandler {
	return &IndexHandler{
		client:      client,
		syncService: syncService,
	}
}

// IndexDocumentRequest represents an index document request
type IndexDocumentRequest struct {
	Document map[string]interface{} `json:"document" binding:"required"`
}

// BatchIndexRequest represents a batch index request
type BatchIndexRequest struct {
	Documents []map[string]interface{} `json:"documents" binding:"required"`
	Action    string                   `json:"action,omitempty"` // create, upsert, update
}

// IndexDocument indexes a single document
// @Summary Index a document
// @Description Index a single document to a collection
// @Tags index
// @Accept json
// @Produce json
// @Param collection path string true "Collection name"
// @Param request body IndexDocumentRequest true "Document to index"
// @Success 200 {object} map[string]interface{}
// @Router /index/documents/{collection} [post]
func (h *IndexHandler) IndexDocument(c *gin.Context) {
	collection := c.Param("collection")
	if collection == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "COLLECTION_REQUIRED",
				"message": "Collection name is required",
			},
		})
		return
	}

	var req IndexDocumentRequest
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

	// Ensure tenant_id is set on the document
	req.Document["tenant_id"] = tenantID

	ctx := c.Request.Context()
	result, err := h.client.UpsertDocument(ctx, collection, req.Document)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INDEX_ERROR",
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

// BatchIndex indexes multiple documents
// @Summary Batch index documents
// @Description Index multiple documents to a collection with automatic tenant_id assignment
// @Tags Index
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param collection path string true "Collection name"
// @Param request body BatchIndexRequest true "Documents to index"
// @Success 200 {object} map[string]interface{} "Documents indexed successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Batch indexing error"
// @Router /index/documents/{collection}/batch [post]
func (h *IndexHandler) BatchIndex(c *gin.Context) {
	collection := c.Param("collection")
	if collection == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "COLLECTION_REQUIRED",
				"message": "Collection name is required",
			},
		})
		return
	}

	var req BatchIndexRequest
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

	// Ensure tenant_id is set on all documents
	documents := make([]interface{}, len(req.Documents))
	for i, doc := range req.Documents {
		doc["tenant_id"] = tenantID
		documents[i] = doc
	}

	action := "upsert"
	if req.Action != "" {
		action = req.Action
	}

	ctx := c.Request.Context()
	results, err := h.client.ImportDocuments(ctx, collection, documents, action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "BATCH_INDEX_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	// Count successful imports
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"indexed": successCount,
			"total":   len(results),
			"results": results,
		},
	})
}

// UpdateDocument updates an existing document
// @Summary Update a document
// @Description Update an existing document in a collection
// @Tags Index
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param collection path string true "Collection name"
// @Param id path string true "Document ID"
// @Param request body IndexDocumentRequest true "Updated document"
// @Success 200 {object} map[string]interface{} "Document updated"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Update error"
// @Router /index/documents/{collection}/{id} [put]
func (h *IndexHandler) UpdateDocument(c *gin.Context) {
	collection := c.Param("collection")
	id := c.Param("id")
	if collection == "" || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "PARAMS_REQUIRED",
				"message": "Collection name and document ID are required",
			},
		})
		return
	}

	var req IndexDocumentRequest
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

	// Ensure tenant_id cannot be changed
	req.Document["tenant_id"] = tenantID

	ctx := c.Request.Context()
	result, err := h.client.UpdateDocument(ctx, collection, id, req.Document)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UPDATE_ERROR",
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

// DeleteDocument deletes a document by ID
// @Summary Delete a document
// @Description Delete a document from a collection by ID
// @Tags Index
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param collection path string true "Collection name"
// @Param id path string true "Document ID"
// @Success 200 {object} map[string]interface{} "Document deleted"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Delete error"
// @Router /index/documents/{collection}/{id} [delete]
func (h *IndexHandler) DeleteDocument(c *gin.Context) {
	collection := c.Param("collection")
	id := c.Param("id")
	if collection == "" || id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "PARAMS_REQUIRED",
				"message": "Collection name and document ID are required",
			},
		})
		return
	}

	ctx := c.Request.Context()
	result, err := h.client.DeleteDocument(ctx, collection, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "DELETE_ERROR",
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

// DeleteByQueryRequest represents a delete by query request
type DeleteByQueryRequest struct {
	FilterBy string `json:"filter_by" binding:"required"`
}

// DeleteByQuery deletes documents matching a filter
// @Summary Delete documents by query
// @Description Delete all documents matching a filter (tenant-scoped)
// @Tags Index
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param collection path string true "Collection name"
// @Param request body DeleteByQueryRequest true "Filter expression"
// @Success 200 {object} map[string]interface{} "Documents deleted"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Delete error"
// @Router /index/documents/{collection} [delete]
func (h *IndexHandler) DeleteByQuery(c *gin.Context) {
	collection := c.Param("collection")
	if collection == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "COLLECTION_REQUIRED",
				"message": "Collection name is required",
			},
		})
		return
	}

	var req DeleteByQueryRequest
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

	// Always include tenant filter for security
	filter := "tenant_id:=" + tenantID
	if req.FilterBy != "" {
		filter += " && " + req.FilterBy
	}

	ctx := c.Request.Context()
	deleted, err := h.client.DeleteByQuery(ctx, collection, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "DELETE_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"deleted": deleted,
		},
	})
}

// SyncProducts syncs products from products-service
// @Summary Sync products
// @Description Trigger full sync of products from products-service to search index
// @Tags Sync
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Success 200 {object} map[string]interface{} "Sync completed"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Sync error"
// @Router /index/sync/products [post]
func (h *IndexHandler) SyncProducts(c *gin.Context) {
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

	// Get auth token from request
	authToken := c.GetHeader("Authorization")

	ctx := c.Request.Context()
	result, err := h.syncService.SyncProducts(ctx, tenantID, authToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SYNC_ERROR",
				"message": err.Error(),
			},
			"data": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Products sync completed",
		"data":    result,
	})
}

// SyncCustomers syncs customers from customers-service
// @Summary Sync customers
// @Description Trigger full sync of customers from customers-service to search index
// @Tags Sync
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Success 200 {object} map[string]interface{} "Sync completed"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Sync error"
// @Router /index/sync/customers [post]
func (h *IndexHandler) SyncCustomers(c *gin.Context) {
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

	// Get auth token from request
	authToken := c.GetHeader("Authorization")

	ctx := c.Request.Context()
	result, err := h.syncService.SyncCustomers(ctx, tenantID, authToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SYNC_ERROR",
				"message": err.Error(),
			},
			"data": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Customers sync completed",
		"data":    result,
	})
}

// SyncOrders syncs orders from orders-service
// @Summary Sync orders
// @Description Trigger full sync of orders from orders-service to search index
// @Tags Sync
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Success 200 {object} map[string]interface{} "Sync completed"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Sync error"
// @Router /index/sync/orders [post]
func (h *IndexHandler) SyncOrders(c *gin.Context) {
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

	// Get auth token from request
	authToken := c.GetHeader("Authorization")

	ctx := c.Request.Context()
	result, err := h.syncService.SyncOrders(ctx, tenantID, authToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SYNC_ERROR",
				"message": err.Error(),
			},
			"data": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Orders sync completed",
		"data":    result,
	})
}

// SyncCategories syncs categories from products-service
// @Summary Sync categories
// @Description Trigger full sync of categories from products-service to search index
// @Tags Sync
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Success 200 {object} map[string]interface{} "Sync completed"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Sync error"
// @Router /index/sync/categories [post]
func (h *IndexHandler) SyncCategories(c *gin.Context) {
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

	// Get auth token from request
	authToken := c.GetHeader("Authorization")

	ctx := c.Request.Context()
	result, err := h.syncService.SyncCategories(ctx, tenantID, authToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "SYNC_ERROR",
				"message": err.Error(),
			},
			"data": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Categories sync completed",
		"data":    result,
	})
}

// SyncAllRequest represents a sync all collections request
type SyncAllRequest struct {
	Collections []string `json:"collections,omitempty"` // Optional: specific collections to sync
}

// SyncAll syncs all collections from backend services
// @Summary Sync all collections
// @Description Trigger full sync of all collections from backend services
// @Tags Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param request body SyncAllRequest false "Optional: specific collections to sync"
// @Success 200 {object} map[string]interface{} "Sync completed"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 500 {object} map[string]interface{} "Sync error"
// @Router /index/sync/all [post]
func (h *IndexHandler) SyncAll(c *gin.Context) {
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

	var req SyncAllRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to all collections if no body provided
		req.Collections = []string{"products", "categories", "customers", "orders"}
	}

	if len(req.Collections) == 0 {
		req.Collections = []string{"products", "categories", "customers", "orders"}
	}

	// Get auth token from request
	authToken := c.GetHeader("Authorization")
	ctx := c.Request.Context()

	results := make(map[string]interface{})
	var errors []string

	for _, collection := range req.Collections {
		var result *services.SyncResult
		var err error

		switch collection {
		case "products":
			result, err = h.syncService.SyncProducts(ctx, tenantID, authToken)
		case "customers":
			result, err = h.syncService.SyncCustomers(ctx, tenantID, authToken)
		case "orders":
			result, err = h.syncService.SyncOrders(ctx, tenantID, authToken)
		case "categories":
			result, err = h.syncService.SyncCategories(ctx, tenantID, authToken)
		default:
			errors = append(errors, "unknown collection: "+collection)
			continue
		}

		if err != nil {
			errors = append(errors, collection+": "+err.Error())
		}
		if result != nil {
			results[collection] = result
		}
	}

	if len(errors) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Sync completed with some errors",
			"data":    results,
			"errors":  errors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All collections synced successfully",
		"data":    results,
	})
}
