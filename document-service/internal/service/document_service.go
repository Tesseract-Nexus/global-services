package service

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/tesseract-hub/document-service/internal/cache"
	"github.com/tesseract-hub/document-service/internal/models"
	"github.com/tesseract-hub/document-service/internal/utils"
)

// documentService implements the DocumentService interface
type documentService struct {
	provider         models.CloudStorageProvider
	repository       models.DocumentRepository
	config           models.ConfigProvider
	cache            cache.Cache
	presignedURLTTL  time.Duration
	metadataTTL      time.Duration
	logger           *logrus.Logger
}

// ServiceOptions contains optional configuration for the document service
type ServiceOptions struct {
	Cache           cache.Cache
	PresignedURLTTL time.Duration // TTL for caching presigned URLs
	MetadataTTL     time.Duration // TTL for caching metadata
}

// NewDocumentService creates a new document service
func NewDocumentService(
	provider models.CloudStorageProvider,
	repository models.DocumentRepository,
	config models.ConfigProvider,
	logger *logrus.Logger,
) models.DocumentService {
	return NewDocumentServiceWithOptions(provider, repository, config, logger, nil)
}

// NewDocumentServiceWithOptions creates a new document service with additional options
func NewDocumentServiceWithOptions(
	provider models.CloudStorageProvider,
	repository models.DocumentRepository,
	config models.ConfigProvider,
	logger *logrus.Logger,
	opts *ServiceOptions,
) models.DocumentService {
	if logger == nil {
		logger = logrus.New()
	}

	svc := &documentService{
		provider:        provider,
		repository:      repository,
		config:          config,
		logger:          logger,
		presignedURLTTL: 55 * time.Minute,  // Default: 55 min (URLs expire in 1 hour)
		metadataTTL:     5 * time.Minute,   // Default: 5 min
	}

	if opts != nil {
		if opts.Cache != nil {
			svc.cache = opts.Cache
			logger.Info("Document service initialized with Redis cache")
		}
		if opts.PresignedURLTTL > 0 {
			svc.presignedURLTTL = opts.PresignedURLTTL
		}
		if opts.MetadataTTL > 0 {
			svc.metadataTTL = opts.MetadataTTL
		}
	}

	// Use no-op cache if none provided
	if svc.cache == nil {
		svc.cache = cache.NewNoOpCache()
	}

	return svc
}

// UploadDocument uploads a document to cloud storage
func (s *documentService) UploadDocument(ctx context.Context, request models.UploadRequest, content io.Reader) (*models.Document, error) {
	// Validate request
	if err := s.validateUploadRequest(request); err != nil {
		return nil, fmt.Errorf("invalid upload request: %w", err)
	}

	// Bucket is now required - no default fallback
	bucket := request.Bucket
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	path := request.Path
	if path == "" {
		path = s.generateFilePath(request.Filename)
	}

	// Detect MIME type if not provided
	mimeType := request.MimeType
	if mimeType == "" {
		mimeType = utils.DetectMimeType(request.Filename)
	}

	// Validate MIME type
	if err := s.validateMimeType(mimeType); err != nil {
		return nil, err
	}

	// Read content to calculate checksum and validate size
	contentBytes, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	// Validate file size
	if err := s.validateFileSize(int64(len(contentBytes))); err != nil {
		return nil, err
	}

	// Calculate checksum
	checksum := fmt.Sprintf("%x", md5.Sum(contentBytes))

	// Prepare metadata for cloud storage
	metadata := map[string]string{
		"original-name": request.Filename,
		"mime-type":     mimeType,
		"checksum":      checksum,
	}

	// Add custom tags to metadata
	for key, value := range request.Tags {
		metadata["tag-"+key] = value
	}

	// Upload to cloud storage
	contentReader := strings.NewReader(string(contentBytes))
	if err := s.provider.Upload(ctx, bucket, path, contentReader, metadata); err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"bucket":   bucket,
			"path":     path,
			"filename": request.Filename,
		}).Error("Failed to upload to cloud storage")
		return nil, fmt.Errorf("failed to upload to cloud storage: %w", err)
	}

	// Extract entity fields from request or tags
	entityType := request.EntityType
	entityID := request.EntityID
	mediaType := request.MediaType
	position := request.Position

	// Fallback to extracting from tags if not set in request
	if request.Tags != nil {
		if entityType == "" {
			entityType = request.Tags["entityType"]
		}
		if entityID == "" {
			// Try common tag names for entity ID
			if id := request.Tags["productId"]; id != "" {
				entityType = "product"
				entityID = id
			} else if id := request.Tags["categoryId"]; id != "" {
				entityType = "category"
				entityID = id
			} else if id := request.Tags["vendorId"]; id != "" {
				entityType = "vendor"
				entityID = id
			}
		}
		if mediaType == "" {
			mediaType = request.Tags["imageType"]
			if mediaType == "" {
				mediaType = request.Tags["mediaType"]
			}
		}
		if position == 0 {
			if posStr, ok := request.Tags["position"]; ok {
				if p, err := strconv.Atoi(posStr); err == nil {
					position = p
				}
			}
		}
	}

	// Create document record
	document := &models.Document{
		ID:              uuid.New(),
		Filename:        s.sanitizeFilename(request.Filename),
		OriginalName:    request.Filename,
		MimeType:        mimeType,
		Size:            int64(len(contentBytes)),
		Path:            path,
		Bucket:          bucket,
		Provider:        s.provider.GetProviderName(),
		Checksum:        checksum,
		Tags:            request.Tags,
		IsPublic:        request.IsPublic,
		ContentEncoding: request.ContentEncoding,
		CacheControl:    request.CacheControl,
		EntityType:      entityType,
		EntityID:        entityID,
		MediaType:       mediaType,
		Position:        position,
		TenantID:        request.TenantID,
		UserID:          request.UserID,
		ProductID:       request.ProductID,
	}

	// Generate URL if public
	if request.IsPublic {
		url, err := s.provider.GeneratePresignedURL(ctx, bucket, path, "GET", 365*24*3600) // 1 year for public files
		if err != nil {
			s.logger.WithError(err).Warn("Failed to generate public URL")
		} else {
			document.URL = url
		}
	}

	// Save to database
	if err := s.repository.Create(ctx, document); err != nil {
		// Cleanup cloud storage on database error
		if deleteErr := s.provider.Delete(ctx, bucket, path); deleteErr != nil {
			s.logger.WithError(deleteErr).Error("Failed to cleanup cloud storage after database error")
		}
		return nil, fmt.Errorf("failed to save document metadata: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"document_id": document.ID,
		"bucket":      bucket,
		"path":        path,
		"filename":    request.Filename,
		"size":        document.Size,
	}).Info("Document uploaded successfully")

	return document, nil
}

// UploadFromURL uploads a document from a URL
func (s *documentService) UploadFromURL(ctx context.Context, request models.UploadRequest, sourceURL string) (*models.Document, error) {
	return nil, fmt.Errorf("upload from URL not implemented")
}

// DownloadDocument downloads a document from cloud storage
func (s *documentService) DownloadDocument(ctx context.Context, path, bucket string) (*models.DownloadResponse, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	// Get document metadata from database
	// Note: tenantID is empty for internal service operations - tenant isolation should be enforced at handler level
	document, err := s.repository.GetByPath(ctx, path, bucket, "")
	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	// Download content from cloud storage
	content, err := s.provider.Download(ctx, bucket, path)
	if err != nil {
		return nil, fmt.Errorf("failed to download from cloud storage: %w", err)
	}

	return &models.DownloadResponse{
		Content:  content,
		Metadata: document.ToMetadata(),
	}, nil
}

// DownloadDocumentStream downloads a document as a stream
func (s *documentService) DownloadDocumentStream(ctx context.Context, path, bucket string) (io.ReadCloser, *models.DocumentMetadata, error) {
	if bucket == "" {
		return nil, nil, fmt.Errorf("bucket is required")
	}

	// Get document metadata from database
	// Note: tenantID is empty for internal service operations - tenant isolation should be enforced at handler level
	document, err := s.repository.GetByPath(ctx, path, bucket, "")
	if err != nil {
		return nil, nil, fmt.Errorf("document not found: %w", err)
	}

	// Download content stream from cloud storage
	stream, err := s.provider.DownloadStream(ctx, bucket, path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download stream from cloud storage: %w", err)
	}

	metadata := document.ToMetadata()
	return stream, &metadata, nil
}

// GetDocumentMetadata retrieves document metadata
func (s *documentService) GetDocumentMetadata(ctx context.Context, path, bucket string) (*models.DocumentMetadata, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	document, err := s.repository.GetByPath(ctx, path, bucket, "")
	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	metadata := document.ToMetadata()
	return &metadata, nil
}

// UpdateDocumentMetadata updates document metadata
func (s *documentService) UpdateDocumentMetadata(ctx context.Context, path, bucket string, updates map[string]interface{}) (*models.DocumentMetadata, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	// Get existing document
	document, err := s.repository.GetByPath(ctx, path, bucket, "")
	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	// Update cloud storage metadata if tags are being updated
	if tags, ok := updates["tags"]; ok {
		if tagsMap, ok := tags.(map[string]string); ok {
			cloudMetadata := make(map[string]string)
			for key, value := range tagsMap {
				cloudMetadata["tag-"+key] = value
			}
			if err := s.provider.SetMetadata(ctx, bucket, path, cloudMetadata); err != nil {
				s.logger.WithError(err).Warn("Failed to update cloud storage metadata")
			}
		}
	}

	// Update database record
	if err := s.repository.UpdateMetadata(ctx, document.ID.String(), updates); err != nil {
		return nil, fmt.Errorf("failed to update document metadata: %w", err)
	}

	// Return updated metadata
	return s.GetDocumentMetadata(ctx, path, bucket)
}

// DocumentExists checks if a document exists
func (s *documentService) DocumentExists(ctx context.Context, path, bucket string) (bool, error) {
	if bucket == "" {
		return false, fmt.Errorf("bucket is required")
	}

	// Check in database first (faster)
	_, err := s.repository.GetByPath(ctx, path, bucket, "")
	if err != nil {
		return false, nil
	}

	// Optionally verify in cloud storage
	return s.provider.Exists(ctx, bucket, path)
}

// DeleteDocument deletes a document
func (s *documentService) DeleteDocument(ctx context.Context, path, bucket string) error {
	if bucket == "" {
		return fmt.Errorf("bucket is required")
	}

	// Delete from cloud storage first
	if err := s.provider.Delete(ctx, bucket, path); err != nil {
		return fmt.Errorf("failed to delete from cloud storage: %w", err)
	}

	// Delete from database
	// Note: tenantID is empty for internal service operations - tenant isolation should be enforced at handler level
	if err := s.repository.DeleteByPath(ctx, path, bucket, ""); err != nil {
		s.logger.WithError(err).Warn("Failed to delete from database after cloud deletion")
		return fmt.Errorf("failed to delete from database: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"bucket": bucket,
		"path":   path,
	}).Info("Document deleted successfully")

	return nil
}

// CopyDocument copies a document
func (s *documentService) CopyDocument(ctx context.Context, sourcePath, destPath, sourceBucket, destBucket string) error {
	if sourceBucket == "" {
		return fmt.Errorf("source bucket is required")
	}
	if destBucket == "" {
		return fmt.Errorf("destination bucket is required")
	}

	// Get source document metadata
	sourceDoc, err := s.repository.GetByPath(ctx, sourcePath, sourceBucket, "")
	if err != nil {
		return fmt.Errorf("source document not found: %w", err)
	}

	// Copy in cloud storage
	if err := s.provider.Copy(ctx, sourceBucket, sourcePath, destBucket, destPath); err != nil {
		return fmt.Errorf("failed to copy in cloud storage: %w", err)
	}

	// Create new database record
	newDoc := *sourceDoc
	newDoc.ID = uuid.New()
	newDoc.Path = destPath
	newDoc.Bucket = destBucket
	newDoc.CreatedAt = time.Now()
	newDoc.UpdatedAt = time.Now()

	if err := s.repository.Create(ctx, &newDoc); err != nil {
		// Cleanup cloud storage on database error
		if deleteErr := s.provider.Delete(ctx, destBucket, destPath); deleteErr != nil {
			s.logger.WithError(deleteErr).Error("Failed to cleanup cloud storage after database error")
		}
		return fmt.Errorf("failed to create copy metadata: %w", err)
	}

	return nil
}

// MoveDocument moves a document
func (s *documentService) MoveDocument(ctx context.Context, sourcePath, destPath, sourceBucket, destBucket string) error {
	if sourceBucket == "" {
		return fmt.Errorf("source bucket is required")
	}
	if destBucket == "" {
		return fmt.Errorf("destination bucket is required")
	}

	// Get source document metadata
	sourceDoc, err := s.repository.GetByPath(ctx, sourcePath, sourceBucket, "")
	if err != nil {
		return fmt.Errorf("source document not found: %w", err)
	}

	// Move in cloud storage
	if err := s.provider.Move(ctx, sourceBucket, sourcePath, destBucket, destPath); err != nil {
		return fmt.Errorf("failed to move in cloud storage: %w", err)
	}

	// Update database record
	sourceDoc.Path = destPath
	sourceDoc.Bucket = destBucket
	sourceDoc.UpdatedAt = time.Now()

	if err := s.repository.Update(ctx, sourceDoc); err != nil {
		return fmt.Errorf("failed to update document metadata: %w", err)
	}

	return nil
}

// ListDocuments lists documents with pagination
func (s *documentService) ListDocuments(ctx context.Context, request models.ListRequest) (*models.ListResponse, error) {
	// Bucket is now required - no default fallback
	bucket := request.Bucket
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	// Build filters
	filters := make(map[string]interface{})
	filters["bucket"] = bucket

	if request.Prefix != "" {
		// For prefix filtering, we'll use a custom method
		// Note: tenantID is empty for internal service operations - tenant isolation should be enforced at handler level
		documents, total, err := s.repository.GetDocumentsByPathPrefix(ctx, bucket, request.Prefix, "", request.Limit, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to list documents: %w", err)
		}

		// Convert to metadata
		metadataList := make([]models.DocumentMetadata, len(documents))
		for i, doc := range documents {
			metadataList[i] = doc.ToMetadata()
		}

		return &models.ListResponse{
			Documents:  metadataList,
			TotalCount: total,
			HasMore:    int64(len(documents)) == int64(request.Limit),
		}, nil
	}

	if request.TenantID != "" {
		filters["tenant_id"] = request.TenantID
	}

	// List from database
	documents, total, err := s.repository.List(ctx, filters, request.Limit, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	// Convert to metadata
	metadataList := make([]models.DocumentMetadata, len(documents))
	for i, doc := range documents {
		metadataList[i] = doc.ToMetadata()
	}

	return &models.ListResponse{
		Documents:  metadataList,
		TotalCount: total,
		HasMore:    int64(len(documents)) == int64(request.Limit),
	}, nil
}

// GeneratePresignedURL generates a presigned URL with caching
func (s *documentService) GeneratePresignedURL(ctx context.Context, request models.PresignedURLRequest) (*models.PresignedURLResponse, error) {
	// Bucket is now required - no default fallback
	bucket := request.Bucket
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	method := request.Method
	if method == "" {
		method = "GET"
	}

	expiresIn := request.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600 // 1 hour default
	}

	// Get product ID for cache key prefixing (defaults to "marketplace" if empty)
	productID := request.ProductID

	// Check cache first (only for GET requests)
	if method == "GET" {
		cacheKey := cache.PresignedURLCacheKey(productID, bucket, request.Path, method)
		var cached cache.CachedPresignedURL
		if err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && cached.URL != "" && !cached.IsExpired() {
			s.logger.WithFields(logrus.Fields{
				"product_id": productID,
				"bucket":     bucket,
				"path":       request.Path,
			}).Debug("Presigned URL cache hit")
			return &models.PresignedURLResponse{
				URL:       cached.URL,
				Method:    method,
				ExpiresAt: cached.ExpiresAt,
			}, nil
		}
	}

	// Generate new presigned URL
	url, err := s.provider.GeneratePresignedURL(ctx, bucket, request.Path, method, expiresIn)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	// Cache the URL (only for GET requests)
	if method == "GET" {
		cacheKey := cache.PresignedURLCacheKey(productID, bucket, request.Path, method)
		cached := cache.CachedPresignedURL{
			URL:       url,
			ExpiresAt: expiresAt,
		}
		// Cache for slightly less than the URL expiry to ensure freshness
		if err := s.cache.SetJSON(ctx, cacheKey, cached, s.presignedURLTTL); err != nil {
			s.logger.WithError(err).Warn("Failed to cache presigned URL")
		}
	}

	return &models.PresignedURLResponse{
		URL:       url,
		Method:    method,
		ExpiresAt: expiresAt,
	}, nil
}

// BatchDeleteDocuments deletes multiple documents
func (s *documentService) BatchDeleteDocuments(ctx context.Context, request models.BatchRequest) (*models.BatchResponse, error) {
	// Bucket is now required - no default fallback
	bucket := request.Bucket
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	// Delete from cloud storage
	successful, failed, err := s.provider.BatchDelete(ctx, bucket, request.Paths)
	if err != nil {
		return nil, fmt.Errorf("failed to batch delete from cloud storage: %w", err)
	}

	// Delete successful ones from database
	// Note: tenantID is empty for internal service operations - tenant isolation should be enforced at handler level
	if len(successful) > 0 {
		if err := s.repository.BatchDelete(ctx, successful, bucket, ""); err != nil {
			s.logger.WithError(err).Warn("Failed to delete some documents from database after cloud deletion")
		}
	}

	return &models.BatchResponse{
		Successful:     successful,
		Failed:         failed,
		TotalProcessed: len(request.Paths),
	}, nil
}

// CreateBucket creates a new bucket
func (s *documentService) CreateBucket(ctx context.Context, bucketName string, options map[string]interface{}) error {
	return s.provider.CreateBucket(ctx, bucketName, options)
}

// DeleteBucket deletes a bucket
func (s *documentService) DeleteBucket(ctx context.Context, bucketName string) error {
	return s.provider.DeleteBucket(ctx, bucketName)
}

// ListBuckets lists all buckets
func (s *documentService) ListBuckets(ctx context.Context) ([]string, error) {
	return s.provider.ListBuckets(ctx)
}

// GetStorageUsage returns storage usage statistics
func (s *documentService) GetStorageUsage(ctx context.Context, bucket string) (*models.StorageUsage, error) {
	// Bucket is optional for GetStorageUsage - allows getting total usage
	// If empty, returns usage across all accessible buckets

	return s.repository.GetStorageStats(ctx, bucket)
}

// TestConnection tests the connection to the cloud provider
func (s *documentService) TestConnection(ctx context.Context) error {
	return s.provider.TestConnection(ctx)
}

// Helper methods

func (s *documentService) validateUploadRequest(request models.UploadRequest) error {
	if request.Filename == "" {
		return fmt.Errorf("filename is required")
	}

	if request.Size <= 0 {
		return fmt.Errorf("file size must be greater than 0")
	}

	return nil
}

func (s *documentService) validateFileSize(size int64) error {
	maxSize := s.config.GetMaxFileSize()
	if maxSize > 0 && size > maxSize {
		return fmt.Errorf("file size %d bytes exceeds maximum allowed size %d bytes", size, maxSize)
	}
	return nil
}

func (s *documentService) validateMimeType(mimeType string) error {
	allowedTypes := s.config.GetAllowedMimeTypes()
	if len(allowedTypes) == 0 {
		return nil // All types allowed
	}

	for _, allowed := range allowedTypes {
		if mimeType == allowed {
			return nil
		}
		// Check wildcard patterns
		if strings.HasSuffix(allowed, "/*") {
			prefix := strings.TrimSuffix(allowed, "/*")
			if strings.HasPrefix(mimeType, prefix+"/") {
				return nil
			}
		}
	}

	return fmt.Errorf("MIME type %s is not allowed", mimeType)
}

func (s *documentService) generateFilePath(filename string) string {
	timestamp := time.Now().Format("2006/01/02")
	id := uuid.New().String()
	ext := filepath.Ext(filename)
	return fmt.Sprintf("%s/%s%s", timestamp, id, ext)
}

func (s *documentService) sanitizeFilename(filename string) string {
	return utils.SanitizeFilename(filename)
}
