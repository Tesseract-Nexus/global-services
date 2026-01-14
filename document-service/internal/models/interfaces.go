package models

import (
	"context"
	"io"
	"time"
)

// DocumentService defines the interface for document storage operations
type DocumentService interface {
	// Upload operations
	UploadDocument(ctx context.Context, request UploadRequest, content io.Reader) (*Document, error)
	UploadFromURL(ctx context.Context, request UploadRequest, sourceURL string) (*Document, error)

	// Download operations
	DownloadDocument(ctx context.Context, path, bucket string) (*DownloadResponse, error)
	DownloadDocumentStream(ctx context.Context, path, bucket string) (io.ReadCloser, *DocumentMetadata, error)

	// Metadata operations
	GetDocumentMetadata(ctx context.Context, path, bucket string) (*DocumentMetadata, error)
	UpdateDocumentMetadata(ctx context.Context, path, bucket string, updates map[string]interface{}) (*DocumentMetadata, error)

	// Existence and operations
	DocumentExists(ctx context.Context, path, bucket string) (bool, error)
	DeleteDocument(ctx context.Context, path, bucket string) error

	// Copy and move operations
	CopyDocument(ctx context.Context, sourcePath, destPath, sourceBucket, destBucket string) error
	MoveDocument(ctx context.Context, sourcePath, destPath, sourceBucket, destBucket string) error

	// List operations
	ListDocuments(ctx context.Context, request ListRequest) (*ListResponse, error)

	// Presigned URL operations
	GeneratePresignedURL(ctx context.Context, request PresignedURLRequest) (*PresignedURLResponse, error)

	// Batch operations
	BatchDeleteDocuments(ctx context.Context, request BatchRequest) (*BatchResponse, error)

	// Bucket operations
	CreateBucket(ctx context.Context, bucketName string, options map[string]interface{}) error
	DeleteBucket(ctx context.Context, bucketName string) error
	ListBuckets(ctx context.Context) ([]string, error)

	// Storage usage
	GetStorageUsage(ctx context.Context, bucket string) (*StorageUsage, error)

	// Health check
	TestConnection(ctx context.Context) error
}

// DocumentRepository defines the interface for document metadata persistence
type DocumentRepository interface {
	// CRUD operations
	Create(ctx context.Context, document *Document) error
	GetByID(ctx context.Context, id string) (*Document, error)
	GetByPath(ctx context.Context, path, bucket, tenantID string) (*Document, error)
	Update(ctx context.Context, document *Document) error
	Delete(ctx context.Context, id string) error
	DeleteByPath(ctx context.Context, path, bucket, tenantID string) error

	// List operations
	List(ctx context.Context, filters map[string]interface{}, limit, offset int) ([]*Document, int64, error)
	ListByBucket(ctx context.Context, bucket string, limit, offset int) ([]*Document, int64, error)
	ListByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*Document, int64, error)

	// Batch operations
	BatchDelete(ctx context.Context, paths []string, bucket, tenantID string) error

	// Storage statistics
	GetStorageStats(ctx context.Context, bucket string) (*StorageUsage, error)
	GetStorageStatsByTenant(ctx context.Context, tenantID string) (*StorageUsage, error)

	// Additional metadata operations
	UpdateMetadata(ctx context.Context, id string, updates map[string]interface{}) error
	GetDocumentsByPathPrefix(ctx context.Context, bucket, pathPrefix, tenantID string, limit, offset int) ([]*Document, int64, error)

	// Search operations
	Search(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]*Document, int64, error)
}

// CloudStorageProvider defines the interface that all cloud providers must implement
type CloudStorageProvider interface {
	// Provider identification
	GetProviderName() CloudProvider

	// Upload operations
	Upload(ctx context.Context, bucket, path string, content io.Reader, metadata map[string]string) error
	UploadFromURL(ctx context.Context, bucket, path, sourceURL string, metadata map[string]string) error

	// Download operations
	Download(ctx context.Context, bucket, path string) ([]byte, error)
	DownloadStream(ctx context.Context, bucket, path string) (io.ReadCloser, error)

	// Metadata operations
	GetMetadata(ctx context.Context, bucket, path string) (map[string]string, error)
	SetMetadata(ctx context.Context, bucket, path string, metadata map[string]string) error

	// File operations
	Exists(ctx context.Context, bucket, path string) (bool, error)
	Delete(ctx context.Context, bucket, path string) error
	Copy(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error
	Move(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error

	// List operations
	List(ctx context.Context, bucket, prefix string, limit int, continuationToken string) ([]CloudStorageObject, string, error)

	// Presigned URL operations
	GeneratePresignedURL(ctx context.Context, bucket, path, method string, expiresIn int) (string, error)

	// Bucket operations
	CreateBucket(ctx context.Context, bucketName string, options map[string]interface{}) error
	DeleteBucket(ctx context.Context, bucketName string) error
	ListBuckets(ctx context.Context) ([]string, error)
	BucketExists(ctx context.Context, bucketName string) (bool, error)

	// Batch operations
	BatchDelete(ctx context.Context, bucket string, paths []string) ([]string, []BatchError, error)

	// Usage statistics
	GetUsageStats(ctx context.Context, bucket string) (*StorageUsage, error)

	// Health check
	TestConnection(ctx context.Context) error
}

// CloudStorageObject represents an object in cloud storage
type CloudStorageObject struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	LastModified time.Time         `json:"lastModified"`
	ETag         string            `json:"etag"`
	StorageClass string            `json:"storageClass,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ConfigProvider defines the interface for configuration management
type ConfigProvider interface {
	GetCloudProvider() CloudProvider
	GetDefaultBucket() string
	GetPublicBucket() string      // Public bucket for marketplace assets (categories, products, etc.)
	GetPublicBucketURL() string   // CDN URL or direct GCS URL for public bucket
	GetMaxFileSize() int64
	GetAllowedMimeTypes() []string
	IsValidationEnabled() bool
	GetAWSConfig() *AWSConfig
	GetAzureConfig() *AzureConfig
	GetGCPConfig() *GCPConfig
	GetLocalConfig() *LocalConfig
}

// AWSConfig represents AWS S3 configuration
type AWSConfig struct {
	Region          string `json:"region" mapstructure:"region"`
	AccessKeyID     string `json:"accessKeyId,omitempty" mapstructure:"access_key_id"`
	SecretAccessKey string `json:"secretAccessKey,omitempty" mapstructure:"secret_access_key"`
	SessionToken    string `json:"sessionToken,omitempty" mapstructure:"session_token"`
	Endpoint        string `json:"endpoint,omitempty" mapstructure:"endpoint"`
	ForcePathStyle  bool   `json:"forcePathStyle,omitempty" mapstructure:"force_path_style"`

	// S3 specific settings
	StorageClass         string `json:"storageClass,omitempty" mapstructure:"storage_class"`
	ServerSideEncryption string `json:"serverSideEncryption,omitempty" mapstructure:"server_side_encryption"`
	KMSKeyID             string `json:"kmsKeyId,omitempty" mapstructure:"kms_key_id"`
}

// AzureConfig represents Azure Blob Storage configuration
type AzureConfig struct {
	AccountName      string `json:"accountName" mapstructure:"account_name"`
	AccountKey       string `json:"accountKey,omitempty" mapstructure:"account_key"`
	SASToken         string `json:"sasToken,omitempty" mapstructure:"sas_token"`
	ConnectionString string `json:"connectionString,omitempty" mapstructure:"connection_string"`
	Endpoint         string `json:"endpoint,omitempty" mapstructure:"endpoint"`

	// Azure specific settings
	AccessTier string `json:"accessTier,omitempty" mapstructure:"access_tier"`
	BlobType   string `json:"blobType,omitempty" mapstructure:"blob_type"`
}

// GCPConfig represents Google Cloud Storage configuration
type GCPConfig struct {
	ProjectID   string      `json:"projectId" mapstructure:"project_id"`
	KeyFilename string      `json:"keyFilename,omitempty" mapstructure:"key_filename"`
	Credentials interface{} `json:"credentials,omitempty" mapstructure:"credentials"`
	Endpoint    string      `json:"endpoint,omitempty" mapstructure:"endpoint"`

	// GCS specific settings
	StorageClass             string `json:"storageClass,omitempty" mapstructure:"storage_class"`
	UniformBucketLevelAccess bool   `json:"uniformBucketLevelAccess,omitempty" mapstructure:"uniform_bucket_level_access"`
}

// LocalConfig represents local filesystem storage configuration
type LocalConfig struct {
	BasePath string `json:"basePath" mapstructure:"base_path"`
}
