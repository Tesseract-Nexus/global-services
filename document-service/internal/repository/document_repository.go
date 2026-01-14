package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tesseract-hub/document-service/internal/models"
	"gorm.io/gorm"
)

// documentRepository implements the DocumentRepository interface
type documentRepository struct {
	db *gorm.DB
}

// NewDocumentRepository creates a new document repository
func NewDocumentRepository(db *gorm.DB) models.DocumentRepository {
	return &documentRepository{
		db: db,
	}
}

// Create creates a new document record
func (r *documentRepository) Create(ctx context.Context, document *models.Document) error {
	if err := r.db.WithContext(ctx).Create(document).Error; err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}
	return nil
}

// GetByID retrieves a document by ID
func (r *documentRepository) GetByID(ctx context.Context, id string) (*models.Document, error) {
	var document models.Document

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid document ID: %w", err)
	}

	if err := r.db.WithContext(ctx).Where("id = ?", parsedID).First(&document).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("document not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return &document, nil
}

// GetByPath retrieves a document by path, bucket, and tenant
// TenantID is required for multi-tenant isolation - prevents cross-tenant access
func (r *documentRepository) GetByPath(ctx context.Context, path, bucket, tenantID string) (*models.Document, error) {
	var document models.Document

	query := r.db.WithContext(ctx).Where("path = ?", path)
	if bucket != "" {
		query = query.Where("bucket = ?", bucket)
	}
	// Enforce tenant isolation - tenantID can be empty for internal/system operations
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.First(&document).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("document not found: %s/%s", bucket, path)
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return &document, nil
}

// Update updates a document record
func (r *documentRepository) Update(ctx context.Context, document *models.Document) error {
	if err := r.db.WithContext(ctx).Save(document).Error; err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}
	return nil
}

// Delete deletes a document by ID
func (r *documentRepository) Delete(ctx context.Context, id string) error {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid document ID: %w", err)
	}

	result := r.db.WithContext(ctx).Delete(&models.Document{}, parsedID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete document: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found: %s", id)
	}

	return nil
}

// DeleteByPath deletes a document by path, bucket, and tenant
// TenantID is required for multi-tenant isolation - prevents cross-tenant deletion
func (r *documentRepository) DeleteByPath(ctx context.Context, path, bucket, tenantID string) error {
	query := r.db.WithContext(ctx).Where("path = ?", path)
	if bucket != "" {
		query = query.Where("bucket = ?", bucket)
	}
	// Enforce tenant isolation - tenantID can be empty for internal/system operations
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result := query.Delete(&models.Document{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete document: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found: %s/%s", bucket, path)
	}

	return nil
}

// List retrieves documents with filters, pagination
func (r *documentRepository) List(ctx context.Context, filters map[string]interface{}, limit, offset int) ([]*models.Document, int64, error) {
	var documents []*models.Document
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Document{})

	// Apply filters
	query = r.applyFilters(query, filters)

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// Apply pagination and get results
	if err := query.Limit(limit).Offset(offset).Find(&documents).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}

	return documents, total, nil
}

// ListByBucket retrieves documents from a specific bucket
func (r *documentRepository) ListByBucket(ctx context.Context, bucket string, limit, offset int) ([]*models.Document, int64, error) {
	filters := map[string]interface{}{
		"bucket": bucket,
	}
	return r.List(ctx, filters, limit, offset)
}

// ListByTenant retrieves documents for a specific tenant
func (r *documentRepository) ListByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*models.Document, int64, error) {
	filters := map[string]interface{}{
		"tenant_id": tenantID,
	}
	return r.List(ctx, filters, limit, offset)
}

// BatchDelete deletes multiple documents by paths with tenant isolation
// TenantID is required for multi-tenant isolation - prevents cross-tenant deletion
func (r *documentRepository) BatchDelete(ctx context.Context, paths []string, bucket, tenantID string) error {
	if len(paths) == 0 {
		return nil
	}

	query := r.db.WithContext(ctx).Where("path IN ?", paths)
	if bucket != "" {
		query = query.Where("bucket = ?", bucket)
	}
	// Enforce tenant isolation - tenantID can be empty for internal/system operations
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if err := query.Delete(&models.Document{}).Error; err != nil {
		return fmt.Errorf("failed to batch delete documents: %w", err)
	}

	return nil
}

// GetStorageStats returns storage statistics for a bucket
func (r *documentRepository) GetStorageStats(ctx context.Context, bucket string) (*models.StorageUsage, error) {
	var stats struct {
		TotalSize int64 `json:"totalSize"`
		Count     int64 `json:"count"`
	}

	query := r.db.WithContext(ctx).Model(&models.Document{})
	if bucket != "" {
		query = query.Where("bucket = ?", bucket)
	}

	if err := query.Select("COALESCE(SUM(size), 0) as total_size, COUNT(*) as count").Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("failed to get storage stats: %w", err)
	}

	return &models.StorageUsage{
		TotalSize:     stats.TotalSize,
		DocumentCount: stats.Count,
		LastUpdated:   time.Now(),
	}, nil
}

// GetStorageStatsByTenant returns storage statistics for a tenant
func (r *documentRepository) GetStorageStatsByTenant(ctx context.Context, tenantID string) (*models.StorageUsage, error) {
	var stats struct {
		TotalSize int64 `json:"totalSize"`
		Count     int64 `json:"count"`
	}

	query := r.db.WithContext(ctx).Model(&models.Document{}).Where("tenant_id = ?", tenantID)

	if err := query.Select("COALESCE(SUM(size), 0) as total_size, COUNT(*) as count").Scan(&stats).Error; err != nil {
		return nil, fmt.Errorf("failed to get tenant storage stats: %w", err)
	}

	return &models.StorageUsage{
		TotalSize:     stats.TotalSize,
		DocumentCount: stats.Count,
		LastUpdated:   time.Now(),
	}, nil
}

// Search searches documents by query with filters
func (r *documentRepository) Search(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]*models.Document, int64, error) {
	var documents []*models.Document
	var total int64

	dbQuery := r.db.WithContext(ctx).Model(&models.Document{})

	// Apply text search
	if query != "" {
		searchPattern := fmt.Sprintf("%%%s%%", query)
		dbQuery = dbQuery.Where(
			"filename ILIKE ? OR original_name ILIKE ? OR mime_type ILIKE ?",
			searchPattern, searchPattern, searchPattern,
		)
	}

	// Apply additional filters
	dbQuery = r.applyFilters(dbQuery, filters)

	// Get total count
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Apply pagination and get results
	if err := dbQuery.Limit(limit).Offset(offset).Find(&documents).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search documents: %w", err)
	}

	return documents, total, nil
}

// applyFilters applies filters to a GORM query
func (r *documentRepository) applyFilters(query *gorm.DB, filters map[string]interface{}) *gorm.DB {
	for key, value := range filters {
		switch key {
		case "bucket":
			query = query.Where("bucket = ?", value)
		case "provider":
			query = query.Where("provider = ?", value)
		case "mime_type":
			query = query.Where("mime_type = ?", value)
		case "mime_type_prefix":
			query = query.Where("mime_type LIKE ?", fmt.Sprintf("%s%%", value))
		case "tenant_id":
			query = query.Where("tenant_id = ?", value)
		case "user_id":
			query = query.Where("user_id = ?", value)
		case "is_public":
			query = query.Where("is_public = ?", value)
		case "size_min":
			query = query.Where("size >= ?", value)
		case "size_max":
			query = query.Where("size <= ?", value)
		case "created_after":
			query = query.Where("created_at >= ?", value)
		case "created_before":
			query = query.Where("created_at <= ?", value)
		case "updated_after":
			query = query.Where("updated_at >= ?", value)
		case "updated_before":
			query = query.Where("updated_at <= ?", value)
		case "tags":
			if tagFilters, ok := value.(map[string]string); ok {
				for tagKey, tagValue := range tagFilters {
					query = query.Where("tags->>? = ?", tagKey, tagValue)
				}
			}
		case "has_tag":
			if tagKey, ok := value.(string); ok {
				query = query.Where("tags ? ?", tagKey)
			}
		}
	}

	return query
}

// GetDocumentsByIDs retrieves multiple documents by their IDs
func (r *documentRepository) GetDocumentsByIDs(ctx context.Context, ids []string) ([]*models.Document, error) {
	if len(ids) == 0 {
		return []*models.Document{}, nil
	}

	var documents []*models.Document
	var uuids []uuid.UUID

	// Parse UUIDs
	for _, id := range ids {
		parsedID, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("invalid document ID %s: %w", id, err)
		}
		uuids = append(uuids, parsedID)
	}

	if err := r.db.WithContext(ctx).Where("id IN ?", uuids).Find(&documents).Error; err != nil {
		return nil, fmt.Errorf("failed to get documents by IDs: %w", err)
	}

	return documents, nil
}

// UpdateMetadata updates only the metadata fields of a document
func (r *documentRepository) UpdateMetadata(ctx context.Context, id string, updates map[string]interface{}) error {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid document ID: %w", err)
	}

	// Add updated_at field
	updates["updated_at"] = time.Now()

	result := r.db.WithContext(ctx).Model(&models.Document{}).Where("id = ?", parsedID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update document metadata: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found: %s", id)
	}

	return nil
}

// CountDocuments returns the total number of documents with optional filters
func (r *documentRepository) CountDocuments(ctx context.Context, filters map[string]interface{}) (int64, error) {
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Document{})
	query = r.applyFilters(query, filters)

	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}

	return count, nil
}

// GetDocumentsByPathPrefix retrieves documents with a specific path prefix
// TenantID is required for multi-tenant isolation - prevents cross-tenant access
func (r *documentRepository) GetDocumentsByPathPrefix(ctx context.Context, bucket, pathPrefix, tenantID string, limit, offset int) ([]*models.Document, int64, error) {
	var documents []*models.Document
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Document{}).Where("path LIKE ?", pathPrefix+"%")
	if bucket != "" {
		query = query.Where("bucket = ?", bucket)
	}
	// Enforce tenant isolation - tenantID can be empty for internal/system operations
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count documents by path prefix: %w", err)
	}

	// Apply pagination and get results
	if err := query.Limit(limit).Offset(offset).Find(&documents).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get documents by path prefix: %w", err)
	}

	return documents, total, nil
}
