package gcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"github.com/tesseract-hub/document-service/internal/models"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSProvider implements the CloudStorageProvider interface for Google Cloud Storage
type GCSProvider struct {
	client *storage.Client
	config *models.GCPConfig
	logger *logrus.Logger
}

// NewGCSProvider creates a new Google Cloud Storage provider instance
func NewGCSProvider(cfg *models.GCPConfig, logger *logrus.Logger) (*GCSProvider, error) {
	if logger == nil {
		logger = logrus.New()
	}

	// Validate configuration
	if cfg.ProjectID == "" {
		return nil, errors.New("GCP project ID is required")
	}

	// Create GCS client options
	var opts []option.ClientOption

	if cfg.KeyFilename != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.KeyFilename))
	} else if cfg.Credentials != nil {
		// Support JSON credentials passed as interface{}
		if jsonCreds, ok := cfg.Credentials.([]byte); ok {
			opts = append(opts, option.WithCredentialsJSON(jsonCreds))
		}
	}

	if cfg.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(cfg.Endpoint))
	}

	// Create GCS client
	ctx := context.Background()
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSProvider{
		client: client,
		config: cfg,
		logger: logger,
	}, nil
}

// GetProviderName returns the provider name
func (p *GCSProvider) GetProviderName() models.CloudProvider {
	return models.ProviderGCP
}

// Upload uploads content to Google Cloud Storage
func (p *GCSProvider) Upload(ctx context.Context, bucket, path string, content io.Reader, metadata map[string]string) error {
	obj := p.client.Bucket(bucket).Object(path)
	writer := obj.NewWriter(ctx)

	// Set metadata
	if metadata != nil {
		writer.Metadata = metadata
	}

	// Set storage class if configured
	if p.config.StorageClass != "" {
		writer.StorageClass = p.config.StorageClass
	}

	// Copy content to GCS
	if _, err := io.Copy(writer, content); err != nil {
		writer.Close()
		p.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"path":   path,
		}).Error("Failed to upload to GCS")
		return fmt.Errorf("failed to upload to GCS: %w", err)
	}

	// Close writer to finalize upload
	if err := writer.Close(); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"path":   path,
		}).Error("Failed to finalize GCS upload")
		return fmt.Errorf("failed to finalize GCS upload: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"bucket": bucket,
		"path":   path,
	}).Info("Successfully uploaded to GCS")

	return nil
}

// UploadFromURL uploads content from a URL to GCS
func (p *GCSProvider) UploadFromURL(ctx context.Context, bucket, path, sourceURL string, metadata map[string]string) error {
	// This would typically involve downloading from the URL first, then uploading
	// For now, return an error indicating it's not implemented
	return fmt.Errorf("upload from URL not implemented for GCS provider")
}

// Download downloads content from Google Cloud Storage
func (p *GCSProvider) Download(ctx context.Context, bucket, path string) ([]byte, error) {
	obj := p.client.Bucket(bucket).Object(path)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"path":   path,
		}).Error("Failed to download from GCS")
		return nil, fmt.Errorf("failed to download from GCS: %w", err)
	}
	defer reader.Close()

	// Read all content
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read GCS object content: %w", err)
	}

	return content, nil
}

// DownloadStream downloads content as a stream from GCS
func (p *GCSProvider) DownloadStream(ctx context.Context, bucket, path string) (io.ReadCloser, error) {
	obj := p.client.Bucket(bucket).Object(path)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"path":   path,
		}).Error("Failed to download stream from GCS")
		return nil, fmt.Errorf("failed to download stream from GCS: %w", err)
	}

	return reader, nil
}

// GetMetadata retrieves object metadata from GCS
func (p *GCSProvider) GetMetadata(ctx context.Context, bucket, path string) (map[string]string, error) {
	obj := p.client.Bucket(bucket).Object(path)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GCS object metadata: %w", err)
	}

	return attrs.Metadata, nil
}

// SetMetadata sets object metadata in GCS
func (p *GCSProvider) SetMetadata(ctx context.Context, bucket, path string, metadata map[string]string) error {
	obj := p.client.Bucket(bucket).Object(path)

	// GCS supports updating metadata directly
	attrs := storage.ObjectAttrsToUpdate{
		Metadata: metadata,
	}

	if _, err := obj.Update(ctx, attrs); err != nil {
		return fmt.Errorf("failed to update GCS object metadata: %w", err)
	}

	return nil
}

// Exists checks if an object exists in GCS
func (p *GCSProvider) Exists(ctx context.Context, bucket, path string) (bool, error) {
	obj := p.client.Bucket(bucket).Object(path)
	_, err := obj.Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, fmt.Errorf("failed to check GCS object existence: %w", err)
	}

	return true, nil
}

// Delete deletes an object from GCS
func (p *GCSProvider) Delete(ctx context.Context, bucket, path string) error {
	obj := p.client.Bucket(bucket).Object(path)

	if err := obj.Delete(ctx); err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"path":   path,
		}).Error("Failed to delete from GCS")
		return fmt.Errorf("failed to delete from GCS: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"bucket": bucket,
		"path":   path,
	}).Info("Successfully deleted from GCS")

	return nil
}

// Copy copies an object within GCS
func (p *GCSProvider) Copy(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error {
	src := p.client.Bucket(sourceBucket).Object(sourcePath)
	dst := p.client.Bucket(destBucket).Object(destPath)

	if _, err := dst.CopierFrom(src).Run(ctx); err != nil {
		return fmt.Errorf("failed to copy GCS object: %w", err)
	}

	return nil
}

// Move moves an object within GCS (copy then delete)
func (p *GCSProvider) Move(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error {
	// First copy the object
	if err := p.Copy(ctx, sourceBucket, sourcePath, destBucket, destPath); err != nil {
		return fmt.Errorf("failed to copy during move: %w", err)
	}

	// Then delete the original
	if err := p.Delete(ctx, sourceBucket, sourcePath); err != nil {
		return fmt.Errorf("failed to delete original during move: %w", err)
	}

	return nil
}

// List lists objects in GCS
func (p *GCSProvider) List(ctx context.Context, bucket, prefix string, limit int, continuationToken string) ([]models.CloudStorageObject, string, error) {
	query := &storage.Query{
		Prefix: prefix,
	}

	if continuationToken != "" {
		query.StartOffset = continuationToken
	}

	it := p.client.Bucket(bucket).Objects(ctx, query)

	var objects []models.CloudStorageObject
	count := 0
	var nextToken string

	for {
		if limit > 0 && count >= limit {
			break
		}

		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("failed to list GCS objects: %w", err)
		}

		objects = append(objects, models.CloudStorageObject{
			Key:          attrs.Name,
			Size:         attrs.Size,
			LastModified: attrs.Updated,
			ETag:         attrs.Etag,
			StorageClass: attrs.StorageClass,
			Metadata:     attrs.Metadata,
		})

		count++
		nextToken = attrs.Name // Use last object name as continuation token
	}

	// Only return continuation token if we hit the limit
	if limit > 0 && count >= limit {
		return objects, nextToken, nil
	}

	return objects, "", nil
}

// GeneratePresignedURL generates a presigned URL for GCS
func (p *GCSProvider) GeneratePresignedURL(ctx context.Context, bucket, path, method string, expiresIn int) (string, error) {
	opts := &storage.SignedURLOptions{
		Method:  method,
		Expires: time.Now().Add(time.Duration(expiresIn) * time.Second),
	}

	url, err := p.client.Bucket(bucket).SignedURL(path, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate GCS presigned URL: %w", err)
	}

	return url, nil
}

// CreateBucket creates a new GCS bucket
func (p *GCSProvider) CreateBucket(ctx context.Context, bucketName string, options map[string]interface{}) error {
	bucket := p.client.Bucket(bucketName)

	attrs := &storage.BucketAttrs{
		Location: "US",
	}

	// Set storage class if configured
	if p.config.StorageClass != "" {
		attrs.StorageClass = p.config.StorageClass
	}

	// Set uniform bucket-level access if configured
	if p.config.UniformBucketLevelAccess {
		attrs.UniformBucketLevelAccess = storage.UniformBucketLevelAccess{
			Enabled: true,
		}
	}

	// Apply options from map
	if location, ok := options["location"].(string); ok {
		attrs.Location = location
	}
	if storageClass, ok := options["storageClass"].(string); ok {
		attrs.StorageClass = storageClass
	}

	if err := bucket.Create(ctx, p.config.ProjectID, attrs); err != nil {
		return fmt.Errorf("failed to create GCS bucket: %w", err)
	}

	return nil
}

// DeleteBucket deletes a GCS bucket
func (p *GCSProvider) DeleteBucket(ctx context.Context, bucketName string) error {
	bucket := p.client.Bucket(bucketName)

	if err := bucket.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete GCS bucket: %w", err)
	}

	return nil
}

// ListBuckets lists all GCS buckets
func (p *GCSProvider) ListBuckets(ctx context.Context) ([]string, error) {
	it := p.client.Buckets(ctx, p.config.ProjectID)

	var buckets []string
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list GCS buckets: %w", err)
		}
		buckets = append(buckets, attrs.Name)
	}

	return buckets, nil
}

// BucketExists checks if a GCS bucket exists
func (p *GCSProvider) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	bucket := p.client.Bucket(bucketName)
	_, err := bucket.Attrs(ctx)
	if err != nil {
		if err == storage.ErrBucketNotExist {
			return false, nil
		}
		return false, fmt.Errorf("failed to check GCS bucket existence: %w", err)
	}

	return true, nil
}

// BatchDelete deletes multiple objects from GCS
func (p *GCSProvider) BatchDelete(ctx context.Context, bucket string, paths []string) ([]string, []models.BatchError, error) {
	if len(paths) == 0 {
		return []string{}, []models.BatchError{}, nil
	}

	var successful []string
	var failed []models.BatchError

	// GCS doesn't have native batch delete, so we delete one by one
	for _, path := range paths {
		err := p.Delete(ctx, bucket, path)
		if err != nil {
			failed = append(failed, models.BatchError{
				Path:  path,
				Error: err.Error(),
			})
		} else {
			successful = append(successful, path)
		}
	}

	return successful, failed, nil
}

// GetUsageStats gets storage usage statistics for GCS
func (p *GCSProvider) GetUsageStats(ctx context.Context, bucket string) (*models.StorageUsage, error) {
	// List all objects in the bucket and calculate total size
	var totalSize int64
	var count int64

	it := p.client.Bucket(bucket).Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get GCS usage stats: %w", err)
		}

		totalSize += attrs.Size
		count++
	}

	return &models.StorageUsage{
		TotalSize:     totalSize,
		DocumentCount: count,
		LastUpdated:   time.Now(),
	}, nil
}

// TestConnection tests the connection to GCS
func (p *GCSProvider) TestConnection(ctx context.Context) error {
	// Use BucketExists on the configured bucket - this only requires bucket-level permissions
	// ListBuckets requires project-level storage.buckets.list permission
	defaultBucket := os.Getenv("STORAGE_DEFAULT_BUCKET")
	if defaultBucket == "" {
		defaultBucket = "default"
	}

	exists, err := p.BucketExists(ctx, defaultBucket)
	if err != nil {
		return fmt.Errorf("GCS connection test failed: %w", err)
	}
	if !exists {
		return fmt.Errorf("GCS connection test: bucket %s does not exist", defaultBucket)
	}

	return nil
}

// Close closes the GCS client
func (p *GCSProvider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}
