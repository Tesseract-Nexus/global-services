package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"document-service/internal/middleware"
	"document-service/internal/models"
)

// DocumentHandler handles HTTP requests for document operations
type DocumentHandler struct {
	service models.DocumentService
	config  models.ConfigProvider
	logger  *logrus.Logger
}

// NewDocumentHandler creates a new document handler
func NewDocumentHandler(service models.DocumentService, config models.ConfigProvider, logger *logrus.Logger) *DocumentHandler {
	if logger == nil {
		logger = logrus.New()
	}

	return &DocumentHandler{
		service: service,
		config:  config,
		logger:  logger,
	}
}

// normalizePath removes the leading slash from wildcard path parameters
// Gin's *path wildcard includes the leading slash, but GCS paths don't have it
func normalizePath(path string) string {
	return strings.TrimPrefix(path, "/")
}

// validateBucketAccess validates that the product can access the requested bucket
// Returns true if access is allowed, false if denied (and sends error response)
func (h *DocumentHandler) validateBucketAccess(c *gin.Context, bucket string) bool {
	if bucket == "" {
		h.respondError(c, http.StatusBadRequest, "Bucket is required", nil)
		return false
	}

	productID := middleware.GetProductID(c)
	if !middleware.ValidateBucketAccess(productID, bucket) {
		h.logger.WithFields(logrus.Fields{
			"product_id": productID,
			"bucket":     bucket,
		}).Warn("Bucket access denied - bucket must start with product name")
		h.respondError(c, http.StatusForbidden, "Bucket access denied",
			fmt.Errorf("product '%s' cannot access bucket '%s' - bucket must start with '%s-'",
				productID, bucket, productID))
		return false
	}
	return true
}

// UploadDocument handles document upload
// @Summary Upload a document
// @Description Upload a document to cloud storage
// @Tags documents
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Document file"
// @Param bucket formData string false "Target bucket"
// @Param path formData string false "Custom storage path"
// @Param tags formData string false "JSON string of tags"
// @Param isPublic formData boolean false "Whether the document should be publicly accessible"
// @Success 201 {object} models.Document
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/upload [post]
func (h *DocumentHandler) UploadDocument(c *gin.Context) {
	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, "Failed to get file from request", err)
		return
	}
	defer file.Close()

	// Parse form data
	bucket := c.PostForm("bucket")

	// Validate bucket access for this product
	if !h.validateBucketAccess(c, bucket) {
		return
	}

	// Get tenant ID from context
	tenantIDVal, _ := c.Get("tenant_id")
	tenantID := ""
	if tenantIDVal != nil {
		tenantID = tenantIDVal.(string)
	}

	// Get user ID from context
	userIDVal, _ := c.Get("user_id")
	userID := ""
	if userIDVal != nil {
		userID = userIDVal.(string)
	}

	request := models.UploadRequest{
		Filename:  header.Filename,
		Size:      header.Size,
		MimeType:  header.Header.Get("Content-Type"),
		Bucket:    bucket,
		Path:      c.PostForm("path"),
		IsPublic:  c.PostForm("isPublic") == "true",
		TenantID:  tenantID,
		UserID:    userID,
		ProductID: middleware.GetProductID(c),
	}

	// Parse tags if provided - supports both JSON and comma-separated key:value format
	if tagsStr := c.PostForm("tags"); tagsStr != "" {
		tags := make(map[string]string)
		tagsStr = strings.TrimSpace(tagsStr)

		// Try JSON format first (more robust)
		if strings.HasPrefix(tagsStr, "{") {
			if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
				h.logger.WithError(err).Debug("Tags not valid JSON, falling back to key:value format")
				tags = nil
			}
		}

		// Fallback to comma-separated key:value format for backward compatibility
		if tags == nil || len(tags) == 0 {
			tags = make(map[string]string)
			pairs := strings.Split(tagsStr, ",")
			for _, pair := range pairs {
				kv := strings.Split(pair, ":")
				if len(kv) == 2 {
					tags[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
		}

		request.Tags = tags
	}

	// Upload document
	ctx := c.Request.Context()
	document, err := h.service.UploadDocument(ctx, request, file)
	if err != nil {
		h.logger.WithError(err).Error("Failed to upload document")
		h.respondError(c, http.StatusInternalServerError, "Failed to upload document", err)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"document_id": document.ID,
		"filename":    document.Filename,
		"size":        document.Size,
	}).Info("Document uploaded successfully")

	c.JSON(http.StatusCreated, document)
}

// DownloadDocument handles document download
// @Summary Download a document
// @Description Download a document from cloud storage (redirects to presigned URL)
// @Tags documents
// @Produce application/octet-stream
// @Param bucket path string true "Bucket name"
// @Param path path string true "Document path"
// @Success 307 {string} string "Redirects to presigned URL"
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/{bucket}/{path} [get]
func (h *DocumentHandler) DownloadDocument(c *gin.Context) {
	bucket := c.Param("bucket")
	path := normalizePath(c.Param("path"))

	// Validate bucket access for this product
	if !h.validateBucketAccess(c, bucket) {
		return
	}

	ctx := c.Request.Context()

	// Generate presigned URL instead of streaming
	response, err := h.service.GeneratePresignedURL(ctx, models.PresignedURLRequest{
		Bucket:    bucket,
		Path:      path,
		Method:    "GET",
		ExpiresIn: 3600, // 1 hour expiration
	})

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(c, http.StatusNotFound, "Document not found", err)
		} else {
			h.respondError(c, http.StatusInternalServerError, "Failed to generate download URL", err)
		}
		return
	}

	// Redirect to the presigned URL
	c.Redirect(http.StatusTemporaryRedirect, response.URL)
}

// GetDocumentMetadata handles getting document metadata
// @Summary Get document metadata
// @Description Get metadata for a document without downloading the content
// @Tags documents
// @Produce json
// @Param bucket path string true "Bucket name"
// @Param path path string true "Document path"
// @Success 200 {object} models.DocumentMetadata
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/{bucket}/{path}/metadata [get]
func (h *DocumentHandler) GetDocumentMetadata(c *gin.Context) {
	bucket := c.Param("bucket")
	path := normalizePath(c.Param("path"))

	// Validate bucket access for this product
	if !h.validateBucketAccess(c, bucket) {
		return
	}

	ctx := c.Request.Context()
	metadata, err := h.service.GetDocumentMetadata(ctx, path, bucket)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(c, http.StatusNotFound, "Document not found", err)
		} else {
			h.respondError(c, http.StatusInternalServerError, "Failed to get document metadata", err)
		}
		return
	}

	c.JSON(http.StatusOK, metadata)
}

// UpdateDocumentMetadata handles updating document metadata
// @Summary Update document metadata
// @Description Update metadata for a document
// @Tags documents
// @Accept json
// @Produce json
// @Param bucket path string true "Bucket name"
// @Param path path string true "Document path"
// @Param updates body map[string]interface{} true "Metadata updates"
// @Success 200 {object} models.DocumentMetadata
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/{bucket}/{path}/metadata [patch]
func (h *DocumentHandler) UpdateDocumentMetadata(c *gin.Context) {
	bucket := c.Param("bucket")
	path := normalizePath(c.Param("path"))

	// Validate bucket access for this product
	if !h.validateBucketAccess(c, bucket) {
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	ctx := c.Request.Context()
	metadata, err := h.service.UpdateDocumentMetadata(ctx, path, bucket, updates)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(c, http.StatusNotFound, "Document not found", err)
		} else {
			h.respondError(c, http.StatusInternalServerError, "Failed to update document metadata", err)
		}
		return
	}

	c.JSON(http.StatusOK, metadata)
}

// DeleteDocument handles document deletion
// @Summary Delete a document
// @Description Delete a document from cloud storage
// @Tags documents
// @Param bucket path string true "Bucket name"
// @Param path path string true "Document path"
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/{bucket}/{path} [delete]
func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	bucket := c.Param("bucket")
	path := normalizePath(c.Param("path"))

	// Validate bucket access for this product
	if !h.validateBucketAccess(c, bucket) {
		return
	}

	ctx := c.Request.Context()
	err := h.service.DeleteDocument(ctx, path, bucket)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(c, http.StatusNotFound, "Document not found", err)
		} else {
			h.respondError(c, http.StatusInternalServerError, "Failed to delete document", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// ListDocuments handles listing documents
// @Summary List documents
// @Description List documents with optional filtering and pagination
// @Tags documents
// @Produce json
// @Param bucket query string false "Bucket name"
// @Param prefix query string false "Path prefix filter"
// @Param limit query int false "Maximum number of results" default(50)
// @Param continuationToken query string false "Pagination token"
// @Param includeMetadata query boolean false "Include metadata in results" default(true)
// @Success 200 {object} models.ListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents [get]
func (h *DocumentHandler) ListDocuments(c *gin.Context) {
	var request models.ListRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid query parameters", err)
		return
	}

	// Validate bucket access for this product
	if !h.validateBucketAccess(c, request.Bucket) {
		return
	}

	// Get tenant ID from context
	tenantIDVal, _ := c.Get("tenant_id")
	if tenantIDVal != nil {
		request.TenantID = tenantIDVal.(string)
	}

	ctx := c.Request.Context()
	response, err := h.service.ListDocuments(ctx, request)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to list documents", err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GeneratePresignedURL handles presigned URL generation
// @Summary Generate presigned URL
// @Description Generate a presigned URL for direct client access
// @Tags documents
// @Accept json
// @Produce json
// @Param request body models.PresignedURLRequest true "Presigned URL request"
// @Success 200 {object} models.PresignedURLResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/presigned-url [post]
func (h *DocumentHandler) GeneratePresignedURL(c *gin.Context) {
	var request models.PresignedURLRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate bucket access for this product
	if !h.validateBucketAccess(c, request.Bucket) {
		return
	}

	// Set product ID for cache key prefixing
	request.ProductID = middleware.GetProductID(c)

	ctx := c.Request.Context()
	response, err := h.service.GeneratePresignedURL(ctx, request)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to generate presigned URL", err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// BatchDeleteDocuments handles batch document deletion
// @Summary Batch delete documents
// @Description Delete multiple documents in a single request
// @Tags documents
// @Accept json
// @Produce json
// @Param request body models.BatchRequest true "Batch delete request"
// @Success 200 {object} models.BatchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/batch/delete [post]
func (h *DocumentHandler) BatchDeleteDocuments(c *gin.Context) {
	var request models.BatchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate bucket access for this product
	if !h.validateBucketAccess(c, request.Bucket) {
		return
	}

	// Get tenant ID from context
	tenantIDVal, _ := c.Get("tenant_id")
	if tenantIDVal != nil {
		request.TenantID = tenantIDVal.(string)
	}

	ctx := c.Request.Context()
	response, err := h.service.BatchDeleteDocuments(ctx, request)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to batch delete documents", err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// CopyDocument handles document copying
// @Summary Copy a document
// @Description Copy a document within the same or different bucket
// @Tags documents
// @Accept json
// @Produce json
// @Param request body CopyMoveRequest true "Copy request"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/copy [post]
func (h *DocumentHandler) CopyDocument(c *gin.Context) {
	var request CopyMoveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate bucket access for source and destination buckets
	if request.SourceBucket != "" && !h.validateBucketAccess(c, request.SourceBucket) {
		return
	}
	if request.DestinationBucket != "" && !h.validateBucketAccess(c, request.DestinationBucket) {
		return
	}

	ctx := c.Request.Context()
	err := h.service.CopyDocument(ctx, request.SourcePath, request.DestinationPath, request.SourceBucket, request.DestinationBucket)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(c, http.StatusNotFound, "Source document not found", err)
		} else {
			h.respondError(c, http.StatusInternalServerError, "Failed to copy document", err)
		}
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "Document copied successfully",
	})
}

// MoveDocument handles document moving
// @Summary Move a document
// @Description Move a document within the same or different bucket
// @Tags documents
// @Accept json
// @Produce json
// @Param request body CopyMoveRequest true "Move request"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/move [post]
func (h *DocumentHandler) MoveDocument(c *gin.Context) {
	var request CopyMoveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate bucket access for source and destination buckets
	if request.SourceBucket != "" && !h.validateBucketAccess(c, request.SourceBucket) {
		return
	}
	if request.DestinationBucket != "" && !h.validateBucketAccess(c, request.DestinationBucket) {
		return
	}

	ctx := c.Request.Context()
	err := h.service.MoveDocument(ctx, request.SourcePath, request.DestinationPath, request.SourceBucket, request.DestinationBucket)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.respondError(c, http.StatusNotFound, "Source document not found", err)
		} else {
			h.respondError(c, http.StatusInternalServerError, "Failed to move document", err)
		}
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Success: true,
		Message: "Document moved successfully",
	})
}

// GetStorageUsage handles storage usage statistics
// @Summary Get storage usage statistics
// @Description Get storage usage statistics for a bucket
// @Tags storage
// @Produce json
// @Param bucket query string false "Bucket name"
// @Success 200 {object} models.StorageUsage
// @Failure 500 {object} ErrorResponse
// @Router /storage/usage [get]
func (h *DocumentHandler) GetStorageUsage(c *gin.Context) {
	bucket := c.Query("bucket")

	// Validate bucket access if bucket is specified
	if bucket != "" && !h.validateBucketAccess(c, bucket) {
		return
	}

	ctx := c.Request.Context()
	usage, err := h.service.GetStorageUsage(ctx, bucket)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to get storage usage", err)
		return
	}

	c.JSON(http.StatusOK, usage)
}

// GetPublicURL handles generating a direct public URL for public bucket assets
// @Summary Get direct public URL
// @Description Get a direct public URL for assets in the public bucket (no presigning needed)
// @Tags documents
// @Produce json
// @Param path path string true "Asset path"
// @Success 200 {object} PublicURLResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/public/{path} [get]
func (h *DocumentHandler) GetPublicURL(c *gin.Context) {
	path := normalizePath(c.Param("path"))

	if path == "" {
		h.respondError(c, http.StatusBadRequest, "Path is required", nil)
		return
	}

	publicBucket := h.config.GetPublicBucket()
	publicBucketURL := h.config.GetPublicBucketURL()

	if publicBucket == "" {
		h.respondError(c, http.StatusInternalServerError, "Public bucket not configured", nil)
		return
	}

	// Generate direct public URL
	var url string
	if publicBucketURL != "" {
		// Use CDN URL if configured
		url = fmt.Sprintf("%s/%s", strings.TrimSuffix(publicBucketURL, "/"), path)
	} else {
		// Use direct GCS URL
		url = fmt.Sprintf("https://storage.googleapis.com/%s/%s", publicBucket, path)
	}

	c.JSON(http.StatusOK, PublicURLResponse{
		URL:    url,
		Bucket: publicBucket,
		Path:   path,
	})
}

// GetBucketConfig returns the bucket configuration for the client
// @Summary Get bucket configuration
// @Description Get the public and private bucket configuration
// @Tags storage
// @Produce json
// @Success 200 {object} BucketConfigResponse
// @Router /storage/config [get]
func (h *DocumentHandler) GetBucketConfig(c *gin.Context) {
	publicBucket := h.config.GetPublicBucket()
	publicBucketURL := h.config.GetPublicBucketURL()
	defaultBucket := h.config.GetDefaultBucket()

	// If no CDN URL is configured, use direct GCS URL
	if publicBucketURL == "" && publicBucket != "" {
		publicBucketURL = fmt.Sprintf("https://storage.googleapis.com/%s", publicBucket)
	}

	c.JSON(http.StatusOK, BucketConfigResponse{
		PublicBucket:    publicBucket,
		PublicBucketURL: publicBucketURL,
		PrivateBucket:   defaultBucket,
	})
}

// DocumentExists handles checking if a document exists
// @Summary Check if document exists
// @Description Check if a document exists in storage
// @Tags documents
// @Produce json
// @Param bucket path string true "Bucket name"
// @Param path path string true "Document path"
// @Success 200 {object} ExistenceResponse
// @Failure 500 {object} ErrorResponse
// @Router /documents/{bucket}/{path}/exists [get]
func (h *DocumentHandler) DocumentExists(c *gin.Context) {
	bucket := c.Param("bucket")
	path := normalizePath(c.Param("path"))

	// Validate bucket access for this product
	if !h.validateBucketAccess(c, bucket) {
		return
	}

	ctx := c.Request.Context()
	exists, err := h.service.DocumentExists(ctx, path, bucket)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "Failed to check document existence", err)
		return
	}

	c.JSON(http.StatusOK, ExistenceResponse{
		Exists: exists,
		Path:   path,
		Bucket: bucket,
	})
}

// Helper methods and types

// CopyMoveRequest represents a copy or move request
type CopyMoveRequest struct {
	SourcePath        string `json:"sourcePath" binding:"required"`
	DestinationPath   string `json:"destinationPath" binding:"required"`
	SourceBucket      string `json:"sourceBucket"`
	DestinationBucket string `json:"destinationBucket"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ExistenceResponse represents a document existence check response
type ExistenceResponse struct {
	Exists bool   `json:"exists"`
	Path   string `json:"path"`
	Bucket string `json:"bucket"`
}

// PublicURLResponse represents a public URL response
type PublicURLResponse struct {
	URL    string `json:"url"`
	Bucket string `json:"bucket"`
	Path   string `json:"path"`
}

// BucketConfigResponse represents the bucket configuration response
type BucketConfigResponse struct {
	PublicBucket    string `json:"publicBucket"`
	PublicBucketURL string `json:"publicBucketUrl"`
	PrivateBucket   string `json:"privateBucket"`
}

// respondError sends an error response
func (h *DocumentHandler) respondError(c *gin.Context, statusCode int, message string, err error) {
	errorMsg := message
	if err != nil {
		errorMsg = err.Error()
	}

	h.logger.WithFields(logrus.Fields{
		"status_code": statusCode,
		"error":       errorMsg,
		"path":        c.Request.URL.Path,
		"method":      c.Request.Method,
	}).Error("Request failed")

	c.JSON(statusCode, ErrorResponse{
		Error:   errorMsg,
		Message: message,
		Code:    statusCode,
	})
}
