package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CloudProvider represents the supported cloud storage providers
type CloudProvider string

const (
	ProviderAWS   CloudProvider = "aws"
	ProviderAzure CloudProvider = "azure"
	ProviderGCP   CloudProvider = "gcp"
	ProviderLocal CloudProvider = "local"
)

// Document represents a document stored in cloud storage
type Document struct {
	ID           uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Filename     string            `json:"filename" gorm:"not null"`
	OriginalName string            `json:"originalName" gorm:"not null"`
	MimeType     string            `json:"mimeType" gorm:"not null"`
	Size         int64             `json:"size" gorm:"not null"`
	Path         string            `json:"path" gorm:"not null"` // unique index created manually in migrations
	Bucket       string            `json:"bucket" gorm:"not null"`
	Provider     CloudProvider     `json:"provider" gorm:"not null"`
	Checksum     string            `json:"checksum,omitempty"`
	Tags         map[string]string `json:"tags,omitempty" gorm:"type:jsonb"`
	IsPublic     bool              `json:"isPublic" gorm:"default:false"`
	URL          string            `json:"url,omitempty"`

	// Metadata
	ContentEncoding string `json:"contentEncoding,omitempty"`
	CacheControl    string `json:"cacheControl,omitempty"`

	// Entity association fields (for better querying)
	EntityType string `json:"entityType,omitempty" gorm:"index:idx_documents_entity"` // product, category, vendor, user, etc.
	EntityID   string `json:"entityId,omitempty" gorm:"index:idx_documents_entity"`   // ID of the associated entity
	MediaType  string `json:"mediaType,omitempty"`                                     // primary, gallery, icon, banner, thumbnail, etc.
	Position   int    `json:"position" gorm:"default:0"`                               // Display order position for galleries

	// Audit fields
	TenantID  string         `json:"tenantId,omitempty" gorm:"index"`
	UserID    string         `json:"userId,omitempty" gorm:"index"`
	ProductID string         `json:"productId,omitempty" gorm:"index"` // Product that owns this document (marketplace, bookkeeping, etc.)
	CreatedAt time.Time      `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// DocumentMetadata represents document metadata without the full document record
type DocumentMetadata struct {
	ID           uuid.UUID         `json:"id"`
	Filename     string            `json:"filename"`
	OriginalName string            `json:"originalName"`
	MimeType     string            `json:"mimeType"`
	Size         int64             `json:"size"`
	Path         string            `json:"path"`
	Bucket       string            `json:"bucket"`
	Provider     CloudProvider     `json:"provider"`
	Checksum     string            `json:"checksum,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	IsPublic     bool              `json:"isPublic"`
	URL          string            `json:"url,omitempty"`
	EntityType   string            `json:"entityType,omitempty"`
	EntityID     string            `json:"entityId,omitempty"`
	MediaType    string            `json:"mediaType,omitempty"`
	Position     int               `json:"position"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

// UploadRequest represents a document upload request
type UploadRequest struct {
	Filename        string            `json:"filename" binding:"required"`
	MimeType        string            `json:"mimeType,omitempty"`
	Size            int64             `json:"size" binding:"required"`
	Bucket          string            `json:"bucket" binding:"required"` // Required: must match product naming convention
	Path            string            `json:"path,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
	IsPublic        bool              `json:"isPublic,omitempty"`
	ContentEncoding string            `json:"contentEncoding,omitempty"`
	CacheControl    string            `json:"cacheControl,omitempty"`
	TenantID        string            `json:"tenantId,omitempty"`
	UserID          string            `json:"userId,omitempty"`
	ProductID       string            `json:"productId,omitempty"` // Product ID (extracted from X-Product-ID header)
	// Entity association (optional - can also be extracted from tags)
	EntityType string `json:"entityType,omitempty"` // product, category, vendor, etc.
	EntityID   string `json:"entityId,omitempty"`   // ID of the associated entity
	MediaType  string `json:"mediaType,omitempty"`  // primary, gallery, icon, banner, etc.
	Position   int    `json:"position,omitempty"`   // Display order for galleries
}

// DownloadResponse represents a document download response
type DownloadResponse struct {
	Content  []byte           `json:"-"`
	Metadata DocumentMetadata `json:"metadata"`
}

// PresignedURLRequest represents a request for a presigned URL
type PresignedURLRequest struct {
	Path      string `json:"path" binding:"required"`
	Bucket    string `json:"bucket" binding:"required"` // Required: must match product naming convention
	Method    string `json:"method,omitempty"`          // GET, PUT, DELETE
	ExpiresIn int    `json:"expiresIn,omitempty"`       // seconds
	ProductID string `json:"productId,omitempty"`       // Product ID (from X-Product-ID header)
}

// PresignedURLResponse represents a presigned URL response
type PresignedURLResponse struct {
	URL       string    `json:"url"`
	Method    string    `json:"method"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// ListRequest represents parameters for listing documents
type ListRequest struct {
	Bucket            string `form:"bucket" binding:"required"` // Required: must match product naming convention
	Prefix            string `form:"prefix"`
	Limit             int    `form:"limit,default=50"`
	ContinuationToken string `form:"continuationToken"`
	IncludeMetadata   bool   `form:"includeMetadata,default=true"`
	TenantID          string `form:"tenantId"`
}

// ListResponse represents a paginated list of documents
type ListResponse struct {
	Documents         []DocumentMetadata `json:"documents"`
	ContinuationToken string             `json:"continuationToken,omitempty"`
	TotalCount        int64              `json:"totalCount,omitempty"`
	HasMore           bool               `json:"hasMore"`
}

// BatchRequest represents a batch operation request
type BatchRequest struct {
	Paths    []string `json:"paths" binding:"required"`
	Bucket   string   `json:"bucket" binding:"required"` // Required: must match product naming convention
	TenantID string   `json:"tenantId,omitempty"`
}

// BatchResponse represents a batch operation response
type BatchResponse struct {
	Successful     []string     `json:"successful"`
	Failed         []BatchError `json:"failed"`
	TotalProcessed int          `json:"totalProcessed"`
}

// BatchError represents an error in a batch operation
type BatchError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// StorageUsage represents storage usage statistics
type StorageUsage struct {
	TotalSize     int64     `json:"totalSize"`
	DocumentCount int64     `json:"documentCount"`
	LastUpdated   time.Time `json:"lastUpdated"`
}

// OperationResult represents the result of a document operation
type OperationResult struct {
	Success  bool                   `json:"success"`
	Data     interface{}            `json:"data,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToMetadata converts a Document to DocumentMetadata
func (d *Document) ToMetadata() DocumentMetadata {
	return DocumentMetadata{
		ID:           d.ID,
		Filename:     d.Filename,
		OriginalName: d.OriginalName,
		MimeType:     d.MimeType,
		Size:         d.Size,
		Path:         d.Path,
		Bucket:       d.Bucket,
		Provider:     d.Provider,
		Checksum:     d.Checksum,
		Tags:         d.Tags,
		IsPublic:     d.IsPublic,
		URL:          d.URL,
		EntityType:   d.EntityType,
		EntityID:     d.EntityID,
		MediaType:    d.MediaType,
		Position:     d.Position,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

// TableName returns the table name for the Document model
func (Document) TableName() string {
	return "documents"
}
