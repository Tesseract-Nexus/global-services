package local

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"document-service/internal/models"
)

// LocalProvider implements the CloudStorageProvider interface for local filesystem storage
type LocalProvider struct {
	basePath string
	logger   *logrus.Logger
}

// NewLocalProvider creates a new local filesystem provider instance
func NewLocalProvider(cfg *models.LocalConfig, logger *logrus.Logger) (*LocalProvider, error) {
	if logger == nil {
		logger = logrus.New()
	}

	if cfg.BasePath == "" {
		return nil, fmt.Errorf("base path is required for local provider")
	}

	// Ensure base path exists
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	return &LocalProvider{
		basePath: cfg.BasePath,
		logger:   logger,
	}, nil
}

// GetProviderName returns the provider name
func (p *LocalProvider) GetProviderName() models.CloudProvider {
	return models.ProviderLocal
}

// getFullPath returns the full filesystem path for a bucket and path
func (p *LocalProvider) getFullPath(bucket, path string) string {
	return filepath.Join(p.basePath, bucket, path)
}

// getBucketPath returns the filesystem path for a bucket
func (p *LocalProvider) getBucketPath(bucket string) string {
	return filepath.Join(p.basePath, bucket)
}

// Upload uploads content to local filesystem
func (p *LocalProvider) Upload(ctx context.Context, bucket, path string, content io.Reader, metadata map[string]string) error {
	fullPath := p.getFullPath(bucket, path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy content
	if _, err := io.Copy(file, content); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"bucket": bucket,
		"path":   path,
	}).Info("Successfully uploaded to local filesystem")

	return nil
}

// UploadFromURL uploads content from a URL to local filesystem
func (p *LocalProvider) UploadFromURL(ctx context.Context, bucket, path, sourceURL string, metadata map[string]string) error {
	return fmt.Errorf("upload from URL not implemented for local provider")
}

// Download downloads content from local filesystem
func (p *LocalProvider) Download(ctx context.Context, bucket, path string) ([]byte, error) {
	fullPath := p.getFullPath(bucket, path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return content, nil
}

// DownloadStream downloads content as a stream from local filesystem
func (p *LocalProvider) DownloadStream(ctx context.Context, bucket, path string) (io.ReadCloser, error) {
	fullPath := p.getFullPath(bucket, path)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// GetMetadata retrieves file metadata from local filesystem
func (p *LocalProvider) GetMetadata(ctx context.Context, bucket, path string) (map[string]string, error) {
	fullPath := p.getFullPath(bucket, path)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	metadata := map[string]string{
		"size":         fmt.Sprintf("%d", info.Size()),
		"lastModified": info.ModTime().Format(time.RFC3339),
	}

	return metadata, nil
}

// SetMetadata sets file metadata (limited support for local filesystem)
func (p *LocalProvider) SetMetadata(ctx context.Context, bucket, path string, metadata map[string]string) error {
	// Local filesystem has limited metadata support
	// We could store metadata in a sidecar file, but for simplicity, return nil
	return nil
}

// Exists checks if a file exists in local filesystem
func (p *LocalProvider) Exists(ctx context.Context, bucket, path string) (bool, error) {
	fullPath := p.getFullPath(bucket, path)

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// Delete deletes a file from local filesystem
func (p *LocalProvider) Delete(ctx context.Context, bucket, path string) error {
	fullPath := p.getFullPath(bucket, path)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"bucket": bucket,
		"path":   path,
	}).Info("Successfully deleted from local filesystem")

	return nil
}

// Copy copies a file within local filesystem
func (p *LocalProvider) Copy(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error {
	sourceFullPath := p.getFullPath(sourceBucket, sourcePath)
	destFullPath := p.getFullPath(destBucket, destPath)

	// Ensure destination directory exists
	destDir := filepath.Dir(destFullPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source file
	sourceFile, err := os.Open(sourceFullPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(destFullPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy content
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// Move moves a file within local filesystem
func (p *LocalProvider) Move(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error {
	sourceFullPath := p.getFullPath(sourceBucket, sourcePath)
	destFullPath := p.getFullPath(destBucket, destPath)

	// Ensure destination directory exists
	destDir := filepath.Dir(destFullPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if err := os.Rename(sourceFullPath, destFullPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}

// List lists files in local filesystem
func (p *LocalProvider) List(ctx context.Context, bucket, prefix string, limit int, continuationToken string) ([]models.CloudStorageObject, string, error) {
	bucketPath := p.getBucketPath(bucket)

	// Check if bucket directory exists
	if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
		return []models.CloudStorageObject{}, "", nil
	}

	var objects []models.CloudStorageObject
	startIndex := 0

	// Parse continuation token (simple offset-based pagination)
	if continuationToken != "" {
		fmt.Sscanf(continuationToken, "%d", &startIndex)
	}

	currentIndex := 0
	err := filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(bucketPath, path)
		if err != nil {
			return err
		}

		// Filter by prefix
		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		// Apply pagination
		if currentIndex < startIndex {
			currentIndex++
			return nil
		}

		// Check limit
		if limit > 0 && len(objects) >= limit {
			return filepath.SkipDir
		}

		// Calculate checksum
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}
		hash := md5.Sum(content)
		etag := hex.EncodeToString(hash[:])

		objects = append(objects, models.CloudStorageObject{
			Key:          relPath,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			ETag:         etag,
		})

		currentIndex++
		return nil
	})

	if err != nil && err != filepath.SkipDir {
		return nil, "", fmt.Errorf("failed to list files: %w", err)
	}

	// Generate next continuation token
	nextToken := ""
	if limit > 0 && len(objects) == limit {
		nextToken = fmt.Sprintf("%d", startIndex+limit)
	}

	return objects, nextToken, nil
}

// GeneratePresignedURL generates a presigned URL (not supported for local filesystem)
func (p *LocalProvider) GeneratePresignedURL(ctx context.Context, bucket, path, method string, expiresIn int) (string, error) {
	// Local filesystem doesn't support presigned URLs
	// Return a file:// URL for local access
	fullPath := p.getFullPath(bucket, path)
	return "file://" + fullPath, nil
}

// CreateBucket creates a new bucket (directory) in local filesystem
func (p *LocalProvider) CreateBucket(ctx context.Context, bucketName string, options map[string]interface{}) error {
	bucketPath := p.getBucketPath(bucketName)

	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		return fmt.Errorf("failed to create bucket directory: %w", err)
	}

	return nil
}

// DeleteBucket deletes a bucket (directory) from local filesystem
func (p *LocalProvider) DeleteBucket(ctx context.Context, bucketName string) error {
	bucketPath := p.getBucketPath(bucketName)

	if err := os.RemoveAll(bucketPath); err != nil {
		return fmt.Errorf("failed to delete bucket directory: %w", err)
	}

	return nil
}

// ListBuckets lists all buckets (directories) in local filesystem
func (p *LocalProvider) ListBuckets(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(p.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var buckets []string
	for _, entry := range entries {
		if entry.IsDir() {
			buckets = append(buckets, entry.Name())
		}
	}

	return buckets, nil
}

// BucketExists checks if a bucket (directory) exists in local filesystem
func (p *LocalProvider) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	bucketPath := p.getBucketPath(bucketName)

	info, err := os.Stat(bucketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	return info.IsDir(), nil
}

// BatchDelete deletes multiple files from local filesystem
func (p *LocalProvider) BatchDelete(ctx context.Context, bucket string, paths []string) ([]string, []models.BatchError, error) {
	var successful []string
	var failed []models.BatchError

	for _, path := range paths {
		if err := p.Delete(ctx, bucket, path); err != nil {
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

// GetUsageStats gets storage usage statistics for local filesystem
func (p *LocalProvider) GetUsageStats(ctx context.Context, bucket string) (*models.StorageUsage, error) {
	bucketPath := p.getBucketPath(bucket)

	var totalSize int64
	var count int64

	err := filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			totalSize += info.Size()
			count++
		}

		return nil
	})

	if err != nil {
		if os.IsNotExist(err) {
			return &models.StorageUsage{
				TotalSize:     0,
				DocumentCount: 0,
				LastUpdated:   time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	return &models.StorageUsage{
		TotalSize:     totalSize,
		DocumentCount: count,
		LastUpdated:   time.Now(),
	}, nil
}

// TestConnection tests the connection to local filesystem
func (p *LocalProvider) TestConnection(ctx context.Context) error {
	// Check if base path exists and is writable
	testFile := filepath.Join(p.basePath, ".test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("local filesystem test failed: %w", err)
	}
	file.Close()
	os.Remove(testFile)

	return nil
}
