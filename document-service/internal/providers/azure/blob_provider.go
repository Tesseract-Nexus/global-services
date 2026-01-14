package azure

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/sirupsen/logrus"
	"github.com/tesseract-hub/document-service/internal/models"
)

// BlobProvider implements the CloudStorageProvider interface for Azure Blob Storage
type BlobProvider struct {
	serviceURL    azblob.ServiceURL
	credential    azblob.Credential
	sharedKeyCred *azblob.SharedKeyCredential // For SAS token generation
	config        *models.AzureConfig
	logger        *logrus.Logger
}

// NewBlobProvider creates a new Azure Blob provider instance
func NewBlobProvider(cfg *models.AzureConfig, logger *logrus.Logger) (*BlobProvider, error) {
	if logger == nil {
		logger = logrus.New()
	}

	// Create credential based on configuration
	credential, sharedKeyCred, err := createCredential(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	// Create service URL
	serviceURL, err := createServiceURL(cfg, credential)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure service URL: %w", err)
	}

	return &BlobProvider{
		serviceURL:    serviceURL,
		credential:    credential,
		sharedKeyCred: sharedKeyCred,
		config:        cfg,
		logger:        logger,
	}, nil
}

// createCredential creates Azure credentials based on configuration
// Returns the credential and optionally a SharedKeyCredential for SAS generation
func createCredential(cfg *models.AzureConfig) (azblob.Credential, *azblob.SharedKeyCredential, error) {
	if cfg.ConnectionString != "" {
		// Parse connection string
		accountName, accountKey, err := parseConnectionString(cfg.ConnectionString)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse connection string: %w", err)
		}
		cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
		if err != nil {
			return nil, nil, err
		}
		return cred, cred, nil
	}

	if cfg.AccountKey != "" {
		cred, err := azblob.NewSharedKeyCredential(cfg.AccountName, cfg.AccountKey)
		if err != nil {
			return nil, nil, err
		}
		return cred, cred, nil
	}

	if cfg.SASToken != "" {
		return azblob.NewAnonymousCredential(), nil, nil
	}

	return nil, nil, fmt.Errorf("no valid Azure credentials found")
}

// createServiceURL creates the Azure Blob service URL
func createServiceURL(cfg *models.AzureConfig, credential azblob.Credential) (azblob.ServiceURL, error) {
	var endpoint string
	if cfg.Endpoint != "" {
		endpoint = cfg.Endpoint
	} else {
		endpoint = fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.AccountName)
	}

	if cfg.SASToken != "" {
		endpoint += "?" + cfg.SASToken
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return azblob.ServiceURL{}, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	return azblob.NewServiceURL(*u, azblob.NewPipeline(credential, azblob.PipelineOptions{})), nil
}

// parseConnectionString parses Azure storage connection string
func parseConnectionString(connStr string) (string, string, error) {
	parts := strings.Split(connStr, ";")
	var accountName, accountKey string

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		switch kv[0] {
		case "AccountName":
			accountName = kv[1]
		case "AccountKey":
			accountKey = kv[1]
		}
	}

	if accountName == "" || accountKey == "" {
		return "", "", fmt.Errorf("connection string missing AccountName or AccountKey")
	}

	return accountName, accountKey, nil
}

// GetProviderName returns the provider name
func (p *BlobProvider) GetProviderName() models.CloudProvider {
	return models.ProviderAzure
}

// Upload uploads content to Azure Blob Storage
func (p *BlobProvider) Upload(ctx context.Context, bucket, path string, content io.Reader, metadata map[string]string) error {
	containerURL := p.serviceURL.NewContainerURL(bucket)
	blobURL := containerURL.NewBlockBlobURL(path)

	// Convert metadata to Azure format
	azureMetadata := azblob.Metadata{}
	if metadata != nil {
		for key, value := range metadata {
			azureMetadata[key] = value
		}
	}

	// Set blob properties
	blobHTTPHeaders := azblob.BlobHTTPHeaders{}
	if mimeType, exists := metadata["mime-type"]; exists {
		blobHTTPHeaders.ContentType = mimeType
	}

	// Set access tier
	accessTier := azblob.AccessTierNone
	if p.config.AccessTier != "" {
		switch p.config.AccessTier {
		case "Hot":
			accessTier = azblob.AccessTierHot
		case "Cool":
			accessTier = azblob.AccessTierCool
		case "Archive":
			accessTier = azblob.AccessTierArchive
		}
	}

	// Upload the blob
	_, err := azblob.UploadStreamToBlockBlob(ctx, content, blobURL, azblob.UploadStreamToBlockBlobOptions{
		BufferSize:      4 * 1024 * 1024, // 4MB
		MaxBuffers:      3,
		BlobHTTPHeaders: blobHTTPHeaders,
		Metadata:        azureMetadata,
		BlobAccessTier:  accessTier,
	})

	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"container": bucket,
			"blob":      path,
		}).Error("Failed to upload to Azure Blob Storage")
		return fmt.Errorf("failed to upload to Azure Blob Storage: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"container": bucket,
		"blob":      path,
	}).Info("Successfully uploaded to Azure Blob Storage")

	return nil
}

// UploadFromURL uploads content from a URL to Azure Blob Storage
func (p *BlobProvider) UploadFromURL(ctx context.Context, bucket, path, sourceURL string, metadata map[string]string) error {
	containerURL := p.serviceURL.NewContainerURL(bucket)
	blobURL := containerURL.NewBlockBlobURL(path)

	// Convert metadata to Azure format
	azureMetadata := azblob.Metadata{}
	if metadata != nil {
		for key, value := range metadata {
			azureMetadata[key] = value
		}
	}

	// Upload from URL
	sourceURLParsed, err := url.Parse(sourceURL)
	if err != nil {
		return fmt.Errorf("invalid source URL: %w", err)
	}

	_, err = blobURL.StartCopyFromURL(ctx, *sourceURLParsed, azureMetadata, azblob.ModifiedAccessConditions{}, azblob.BlobAccessConditions{}, azblob.DefaultAccessTier, nil)
	if err != nil {
		return fmt.Errorf("failed to upload from URL to Azure Blob Storage: %w", err)
	}

	return nil
}

// Download downloads content from Azure Blob Storage
func (p *BlobProvider) Download(ctx context.Context, bucket, path string) ([]byte, error) {
	containerURL := p.serviceURL.NewContainerURL(bucket)
	blobURL := containerURL.NewBlockBlobURL(path)

	// Download blob
	response, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"container": bucket,
			"blob":      path,
		}).Error("Failed to download from Azure Blob Storage")
		return nil, fmt.Errorf("failed to download from Azure Blob Storage: %w", err)
	}

	// Read all content
	bodyStream := response.Body(azblob.RetryReaderOptions{})
	defer bodyStream.Close()

	content, err := io.ReadAll(bodyStream)
	if err != nil {
		return nil, fmt.Errorf("failed to read Azure blob content: %w", err)
	}

	return content, nil
}

// DownloadStream downloads content as a stream from Azure Blob Storage
func (p *BlobProvider) DownloadStream(ctx context.Context, bucket, path string) (io.ReadCloser, error) {
	containerURL := p.serviceURL.NewContainerURL(bucket)
	blobURL := containerURL.NewBlockBlobURL(path)

	// Download blob
	response, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"container": bucket,
			"blob":      path,
		}).Error("Failed to download stream from Azure Blob Storage")
		return nil, fmt.Errorf("failed to download stream from Azure Blob Storage: %w", err)
	}

	return response.Body(azblob.RetryReaderOptions{}), nil
}

// GetMetadata retrieves blob metadata from Azure Blob Storage
func (p *BlobProvider) GetMetadata(ctx context.Context, bucket, path string) (map[string]string, error) {
	containerURL := p.serviceURL.NewContainerURL(bucket)
	blobURL := containerURL.NewBlockBlobURL(path)

	// Get blob properties
	response, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure blob metadata: %w", err)
	}

	// Convert Azure metadata to map
	metadata := make(map[string]string)
	for key, value := range response.NewMetadata() {
		metadata[key] = value
	}

	return metadata, nil
}

// SetMetadata sets blob metadata in Azure Blob Storage
func (p *BlobProvider) SetMetadata(ctx context.Context, bucket, path string, metadata map[string]string) error {
	containerURL := p.serviceURL.NewContainerURL(bucket)
	blobURL := containerURL.NewBlockBlobURL(path)

	// Convert metadata to Azure format
	azureMetadata := azblob.Metadata{}
	if metadata != nil {
		for key, value := range metadata {
			azureMetadata[key] = value
		}
	}

	// Set blob metadata
	_, err := blobURL.SetMetadata(ctx, azureMetadata, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return fmt.Errorf("failed to set Azure blob metadata: %w", err)
	}

	return nil
}

// Exists checks if a blob exists in Azure Blob Storage
func (p *BlobProvider) Exists(ctx context.Context, bucket, path string) (bool, error) {
	containerURL := p.serviceURL.NewContainerURL(bucket)
	blobURL := containerURL.NewBlockBlobURL(path)

	// Try to get blob properties
	_, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		// Check if it's a "not found" error
		if stgErr, ok := err.(azblob.StorageError); ok {
			if stgErr.ServiceCode() == azblob.ServiceCodeBlobNotFound {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check Azure blob existence: %w", err)
	}

	return true, nil
}

// Delete deletes a blob from Azure Blob Storage
func (p *BlobProvider) Delete(ctx context.Context, bucket, path string) error {
	containerURL := p.serviceURL.NewContainerURL(bucket)
	blobURL := containerURL.NewBlockBlobURL(path)

	// Delete blob
	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"container": bucket,
			"blob":      path,
		}).Error("Failed to delete from Azure Blob Storage")
		return fmt.Errorf("failed to delete from Azure Blob Storage: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"container": bucket,
		"blob":      path,
	}).Info("Successfully deleted from Azure Blob Storage")

	return nil
}

// Copy copies a blob within Azure Blob Storage
func (p *BlobProvider) Copy(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error {
	sourceContainerURL := p.serviceURL.NewContainerURL(sourceBucket)
	sourceBlobURL := sourceContainerURL.NewBlockBlobURL(sourcePath)

	destContainerURL := p.serviceURL.NewContainerURL(destBucket)
	destBlobURL := destContainerURL.NewBlockBlobURL(destPath)

	// Start copy operation
	_, err := destBlobURL.StartCopyFromURL(ctx, sourceBlobURL.URL(), nil, azblob.ModifiedAccessConditions{}, azblob.BlobAccessConditions{}, azblob.DefaultAccessTier, nil)
	if err != nil {
		return fmt.Errorf("failed to copy Azure blob: %w", err)
	}

	return nil
}

// Move moves a blob within Azure Blob Storage (copy then delete)
func (p *BlobProvider) Move(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error {
	// First copy the blob
	if err := p.Copy(ctx, sourceBucket, sourcePath, destBucket, destPath); err != nil {
		return fmt.Errorf("failed to copy during move: %w", err)
	}

	// Then delete the original
	if err := p.Delete(ctx, sourceBucket, sourcePath); err != nil {
		return fmt.Errorf("failed to delete original during move: %w", err)
	}

	return nil
}

// List lists blobs in Azure Blob Storage
func (p *BlobProvider) List(ctx context.Context, bucket, prefix string, limit int, continuationToken string) ([]models.CloudStorageObject, string, error) {
	containerURL := p.serviceURL.NewContainerURL(bucket)

	// Set up list options
	options := azblob.ListBlobsSegmentOptions{
		Details: azblob.BlobListingDetails{
			Metadata: true,
		},
	}

	if prefix != "" {
		options.Prefix = prefix
	}

	if limit > 0 {
		options.MaxResults = int32(limit)
	}

	// Parse continuation token
	var marker azblob.Marker
	if continuationToken != "" {
		marker = azblob.Marker{Val: &continuationToken}
	}

	// List blobs
	response, err := containerURL.ListBlobsFlatSegment(ctx, marker, options)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list Azure blobs: %w", err)
	}

	// Convert to CloudStorageObject
	objects := make([]models.CloudStorageObject, len(response.Segment.BlobItems))
	for i, item := range response.Segment.BlobItems {
		metadata := make(map[string]string)
		for key, value := range item.Metadata {
			metadata[key] = value
		}

		objects[i] = models.CloudStorageObject{
			Key:          item.Name,
			Size:         *item.Properties.ContentLength,
			LastModified: item.Properties.LastModified,
			ETag:         string(item.Properties.Etag),
			StorageClass: string(item.Properties.AccessTier),
			Metadata:     metadata,
		}
	}

	// Get next continuation token
	nextToken := ""
	if response.NextMarker.Val != nil {
		nextToken = *response.NextMarker.Val
	}

	return objects, nextToken, nil
}

// GeneratePresignedURL generates a presigned URL for Azure Blob Storage
func (p *BlobProvider) GeneratePresignedURL(ctx context.Context, bucket, path, method string, expiresIn int) (string, error) {
	// SAS generation requires SharedKeyCredential
	if p.sharedKeyCred == nil {
		return "", fmt.Errorf("cannot generate presigned URL: shared key credential not available (using SAS token auth)")
	}

	containerURL := p.serviceURL.NewContainerURL(bucket)
	blobURL := containerURL.NewBlockBlobURL(path)

	// Set permissions based on method
	var permissions azblob.BlobSASPermissions
	switch strings.ToUpper(method) {
	case "GET":
		permissions = azblob.BlobSASPermissions{Read: true}
	case "PUT":
		permissions = azblob.BlobSASPermissions{Write: true, Create: true}
	case "DELETE":
		permissions = azblob.BlobSASPermissions{Delete: true}
	default:
		return "", fmt.Errorf("unsupported HTTP method: %s", method)
	}

	// Generate SAS query parameters
	sasQueryParams, err := azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,
		ExpiryTime:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		ContainerName: bucket,
		BlobName:      path,
		Permissions:   permissions.String(),
	}.NewSASQueryParameters(p.sharedKeyCred)

	if err != nil {
		return "", fmt.Errorf("failed to generate Azure SAS token: %w", err)
	}

	// Create the full URL
	qp := sasQueryParams.Encode()
	blobURLValue := blobURL.URL()
	fullURL := fmt.Sprintf("%s?%s", blobURLValue.String(), qp)

	return fullURL, nil
}

// CreateBucket creates a new container in Azure Blob Storage
func (p *BlobProvider) CreateBucket(ctx context.Context, bucketName string, options map[string]interface{}) error {
	containerURL := p.serviceURL.NewContainerURL(bucketName)

	// Set access level
	accessType := azblob.PublicAccessNone
	if isPublic, ok := options["public"]; ok && isPublic.(bool) {
		accessType = azblob.PublicAccessBlob
	}

	// Create container
	_, err := containerURL.Create(ctx, azblob.Metadata{}, accessType)
	if err != nil {
		return fmt.Errorf("failed to create Azure container: %w", err)
	}

	return nil
}

// DeleteBucket deletes a container from Azure Blob Storage
func (p *BlobProvider) DeleteBucket(ctx context.Context, bucketName string) error {
	containerURL := p.serviceURL.NewContainerURL(bucketName)

	// Delete container
	_, err := containerURL.Delete(ctx, azblob.ContainerAccessConditions{})
	if err != nil {
		return fmt.Errorf("failed to delete Azure container: %w", err)
	}

	return nil
}

// ListBuckets lists all containers in Azure Blob Storage
func (p *BlobProvider) ListBuckets(ctx context.Context) ([]string, error) {
	// List containers
	response, err := p.serviceURL.ListContainersSegment(ctx, azblob.Marker{}, azblob.ListContainersSegmentOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list Azure containers: %w", err)
	}

	// Extract container names
	buckets := make([]string, len(response.ContainerItems))
	for i, container := range response.ContainerItems {
		buckets[i] = container.Name
	}

	return buckets, nil
}

// BucketExists checks if a container exists in Azure Blob Storage
func (p *BlobProvider) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	containerURL := p.serviceURL.NewContainerURL(bucketName)

	// Try to get container properties
	_, err := containerURL.GetProperties(ctx, azblob.LeaseAccessConditions{})
	if err != nil {
		// Check if it's a "not found" error
		if stgErr, ok := err.(azblob.StorageError); ok {
			if stgErr.ServiceCode() == azblob.ServiceCodeContainerNotFound {
				return false, nil
			}
		}
		return false, fmt.Errorf("failed to check Azure container existence: %w", err)
	}

	return true, nil
}

// BatchDelete deletes multiple blobs from Azure Blob Storage
func (p *BlobProvider) BatchDelete(ctx context.Context, bucket string, paths []string) ([]string, []models.BatchError, error) {
	if len(paths) == 0 {
		return []string{}, []models.BatchError{}, nil
	}

	var successful []string
	var failed []models.BatchError

	// Azure doesn't have native batch delete, so we'll delete one by one
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

// GetUsageStats gets storage usage statistics for Azure Blob Storage
func (p *BlobProvider) GetUsageStats(ctx context.Context, bucket string) (*models.StorageUsage, error) {
	// Azure doesn't provide direct API for usage stats
	// We need to calculate by listing all blobs
	var totalSize int64
	var count int64

	containerURL := p.serviceURL.NewContainerURL(bucket)
	marker := azblob.Marker{}

	for {
		response, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{
			Details: azblob.BlobListingDetails{},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get Azure usage stats: %w", err)
		}

		for _, item := range response.Segment.BlobItems {
			if item.Properties.ContentLength != nil {
				totalSize += *item.Properties.ContentLength
			}
			count++
		}

		marker = response.NextMarker
		if !marker.NotDone() {
			break
		}
	}

	return &models.StorageUsage{
		TotalSize:     totalSize,
		DocumentCount: count,
		LastUpdated:   time.Now(),
	}, nil
}

// TestConnection tests the connection to Azure Blob Storage
func (p *BlobProvider) TestConnection(ctx context.Context) error {
	// Try to list containers as a connection test
	_, err := p.serviceURL.ListContainersSegment(ctx, azblob.Marker{}, azblob.ListContainersSegmentOptions{
		MaxResults: 1,
	})
	if err != nil {
		return fmt.Errorf("Azure Blob Storage connection test failed: %w", err)
	}

	return nil
}
