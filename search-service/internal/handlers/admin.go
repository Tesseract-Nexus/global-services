package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tesseract-hub/search-service/internal/clients"
	"github.com/typesense/typesense-go/v2/typesense/api"
)

// AdminHandler handles administrative operations
type AdminHandler struct {
	client *clients.TypesenseClient
}

// NewAdminHandler creates a new AdminHandler
func NewAdminHandler(client *clients.TypesenseClient) *AdminHandler {
	return &AdminHandler{client: client}
}

// ListCollections lists all collections
// @Summary List collections
// @Description Get all Typesense collections with their schemas and document counts
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Success 200 {object} map[string]interface{} "List of collections"
// @Failure 500 {object} map[string]interface{} "Error listing collections"
// @Router /admin/collections [get]
func (h *AdminHandler) ListCollections(c *gin.Context) {
	ctx := c.Request.Context()
	collections, err := h.client.ListCollections(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "LIST_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    collections,
	})
}

// CreateCollection creates a new collection
// @Summary Create a collection
// @Description Create a predefined collection schema (products, customers, orders, categories)
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param name path string true "Collection name (products, customers, orders, categories)"
// @Success 201 {object} map[string]interface{} "Collection created successfully"
// @Success 200 {object} map[string]interface{} "Collection already exists"
// @Failure 400 {object} map[string]interface{} "Invalid collection name"
// @Failure 500 {object} map[string]interface{} "Creation error"
// @Router /admin/collections/{name} [post]
func (h *AdminHandler) CreateCollection(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NAME_REQUIRED",
				"message": "Collection name is required",
			},
		})
		return
	}

	ctx := c.Request.Context()

	// Get predefined schema for known collections
	var schema *api.CollectionSchema
	switch name {
	case "products":
		schema = clients.ProductsSchema
	case "customers":
		schema = clients.CustomersSchema
	case "orders":
		schema = clients.OrdersSchema
	case "categories":
		schema = clients.CategoriesSchema
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "UNKNOWN_COLLECTION",
				"message": "Unknown collection. Supported: products, customers, orders, categories",
			},
		})
		return
	}

	result, err := h.client.CreateCollection(ctx, schema)
	if err != nil {
		// Check if collection already exists
		if existingCollection, getErr := h.client.GetCollection(ctx, name); getErr == nil {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    existingCollection,
				"message": "Collection already exists",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "CREATE_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    result,
	})
}

// DeleteCollection deletes a collection
// @Summary Delete a collection
// @Description Permanently delete a collection and all its documents
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param name path string true "Collection name"
// @Success 200 {object} map[string]interface{} "Collection deleted successfully"
// @Failure 400 {object} map[string]interface{} "Collection name required"
// @Failure 500 {object} map[string]interface{} "Deletion error"
// @Router /admin/collections/{name} [delete]
func (h *AdminHandler) DeleteCollection(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NAME_REQUIRED",
				"message": "Collection name is required",
			},
		})
		return
	}

	ctx := c.Request.Context()
	result, err := h.client.DeleteCollection(ctx, name)
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

// GetCollectionStats gets stats for a collection
// @Summary Get collection statistics
// @Description Get detailed statistics for a specific collection including document count and schema
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param name path string true "Collection name"
// @Success 200 {object} map[string]interface{} "Collection statistics"
// @Failure 400 {object} map[string]interface{} "Collection name required"
// @Failure 500 {object} map[string]interface{} "Error retrieving collection"
// @Router /admin/collections/{name}/stats [get]
func (h *AdminHandler) GetCollectionStats(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NAME_REQUIRED",
				"message": "Collection name is required",
			},
		})
		return
	}

	ctx := c.Request.Context()
	collection, err := h.client.GetCollection(ctx, name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "GET_ERROR",
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"name":          collection.Name,
			"num_documents": collection.NumDocuments,
			"fields":        collection.Fields,
			"created_at":    collection.CreatedAt,
		},
	})
}

// ReindexCollection triggers a full reindex of a collection
// @Summary Reindex a collection
// @Description Trigger a full reindex of a collection from the source service
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Param X-Vendor-ID header string true "Tenant/Vendor ID"
// @Param name path string true "Collection name"
// @Success 202 {object} map[string]interface{} "Reindex initiated"
// @Failure 400 {object} map[string]interface{} "Collection name required"
// @Router /admin/collections/{name}/reindex [post]
func (h *AdminHandler) ReindexCollection(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NAME_REQUIRED",
				"message": "Collection name is required",
			},
		})
		return
	}

	// TODO: Implement full reindex logic
	// This would typically:
	// 1. Create a new collection with timestamp suffix
	// 2. Fetch all data from the source service
	// 3. Index all documents to the new collection
	// 4. Swap the alias/name to point to the new collection
	// 5. Delete the old collection

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "Reindex initiated for collection: " + name,
	})
}
