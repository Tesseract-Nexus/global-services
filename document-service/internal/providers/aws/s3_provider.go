package aws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sirupsen/logrus"
	"document-service/internal/models"
)

// S3Provider implements the CloudStorageProvider interface for AWS S3
type S3Provider struct {
	client     *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
	config     *models.AWSConfig
	logger     *logrus.Logger
}

// NewS3Provider creates a new S3 provider instance
func NewS3Provider(cfg *models.AWSConfig, logger *logrus.Logger) (*S3Provider, error) {
	if logger == nil {
		logger = logrus.New()
	}

	// Create AWS config
	awsConfig, err := createAWSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		if cfg.ForcePathStyle {
			o.UsePathStyle = true
		}
	})

	// Create uploader and downloader
	uploader := manager.NewUploader(client)
	downloader := manager.NewDownloader(client)

	return &S3Provider{
		client:     client,
		uploader:   uploader,
		downloader: downloader,
		config:     cfg,
		logger:     logger,
	}, nil
}

// createAWSConfig creates AWS configuration based on provided settings
func createAWSConfig(cfg *models.AWSConfig) (aws.Config, error) {
	// Load default config
	awsConfig, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override with custom credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)
	}

	return awsConfig, nil
}

// GetProviderName returns the provider name
func (p *S3Provider) GetProviderName() models.CloudProvider {
	return models.ProviderAWS
}

// Upload uploads content to S3
func (p *S3Provider) Upload(ctx context.Context, bucket, path string, content io.Reader, metadata map[string]string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
		Body:   content,
	}

	// Add metadata
	if metadata != nil {
		input.Metadata = metadata
	}

	// Set storage class if configured
	if p.config.StorageClass != "" {
		input.StorageClass = types.StorageClass(p.config.StorageClass)
	}

	// Set server-side encryption if configured
	if p.config.ServerSideEncryption != "" {
		input.ServerSideEncryption = types.ServerSideEncryption(p.config.ServerSideEncryption)
		if p.config.KMSKeyID != "" && p.config.ServerSideEncryption == "aws:kms" {
			input.SSEKMSKeyId = aws.String(p.config.KMSKeyID)
		}
	}

	_, err := p.uploader.Upload(ctx, input)
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"path":   path,
		}).Error("Failed to upload to S3")
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"bucket": bucket,
		"path":   path,
	}).Info("Successfully uploaded to S3")

	return nil
}

// UploadFromURL uploads content from a URL to S3
func (p *S3Provider) UploadFromURL(ctx context.Context, bucket, path, sourceURL string, metadata map[string]string) error {
	// This would typically involve downloading from the URL first, then uploading
	// For now, return an error indicating it's not implemented
	return fmt.Errorf("upload from URL not implemented for S3 provider")
}

// Download downloads content from S3
func (p *S3Provider) Download(ctx context.Context, bucket, path string) ([]byte, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}

	result, err := p.client.GetObject(ctx, input)
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"path":   path,
		}).Error("Failed to download from S3")
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	// Read all content
	content, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read S3 object content: %w", err)
	}

	return content, nil
}

// DownloadStream downloads content as a stream from S3
func (p *S3Provider) DownloadStream(ctx context.Context, bucket, path string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}

	result, err := p.client.GetObject(ctx, input)
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"path":   path,
		}).Error("Failed to download stream from S3")
		return nil, fmt.Errorf("failed to download stream from S3: %w", err)
	}

	return result.Body, nil
}

// GetMetadata retrieves object metadata from S3
func (p *S3Provider) GetMetadata(ctx context.Context, bucket, path string) (map[string]string, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}

	result, err := p.client.HeadObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get S3 object metadata: %w", err)
	}

	return result.Metadata, nil
}

// SetMetadata sets object metadata in S3
func (p *S3Provider) SetMetadata(ctx context.Context, bucket, path string, metadata map[string]string) error {
	// S3 doesn't support updating metadata directly, need to copy object with new metadata
	copySource := fmt.Sprintf("%s/%s", bucket, path)

	input := &s3.CopyObjectInput{
		Bucket:            aws.String(bucket),
		Key:               aws.String(path),
		CopySource:        aws.String(copySource),
		Metadata:          metadata,
		MetadataDirective: types.MetadataDirectiveReplace,
	}

	_, err := p.client.CopyObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update S3 object metadata: %w", err)
	}

	return nil
}

// Exists checks if an object exists in S3
func (p *S3Provider) Exists(ctx context.Context, bucket, path string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}

	_, err := p.client.HeadObject(ctx, input)
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check S3 object existence: %w", err)
	}

	return true, nil
}

// Delete deletes an object from S3
func (p *S3Provider) Delete(ctx context.Context, bucket, path string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}

	_, err := p.client.DeleteObject(ctx, input)
	if err != nil {
		p.logger.WithError(err).WithFields(logrus.Fields{
			"bucket": bucket,
			"path":   path,
		}).Error("Failed to delete from S3")
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"bucket": bucket,
		"path":   path,
	}).Info("Successfully deleted from S3")

	return nil
}

// Copy copies an object within S3
func (p *S3Provider) Copy(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error {
	copySource := fmt.Sprintf("%s/%s", sourceBucket, sourcePath)

	input := &s3.CopyObjectInput{
		Bucket:     aws.String(destBucket),
		Key:        aws.String(destPath),
		CopySource: aws.String(copySource),
	}

	_, err := p.client.CopyObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to copy S3 object: %w", err)
	}

	return nil
}

// Move moves an object within S3 (copy then delete)
func (p *S3Provider) Move(ctx context.Context, sourceBucket, sourcePath, destBucket, destPath string) error {
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

// List lists objects in S3
func (p *S3Provider) List(ctx context.Context, bucket, prefix string, limit int, continuationToken string) ([]models.CloudStorageObject, string, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}

	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	if limit > 0 {
		input.MaxKeys = aws.Int32(int32(limit))
	}

	if continuationToken != "" {
		input.ContinuationToken = aws.String(continuationToken)
	}

	result, err := p.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list S3 objects: %w", err)
	}

	// Convert to CloudStorageObject
	objects := make([]models.CloudStorageObject, len(result.Contents))
	for i, obj := range result.Contents {
		objects[i] = models.CloudStorageObject{
			Key:          *obj.Key,
			Size:         *obj.Size,
			LastModified: *obj.LastModified,
			ETag:         strings.Trim(*obj.ETag, "\""),
			StorageClass: string(obj.StorageClass),
		}
	}

	nextToken := ""
	if result.NextContinuationToken != nil {
		nextToken = *result.NextContinuationToken
	}

	return objects, nextToken, nil
}

// GeneratePresignedURL generates a presigned URL for S3
func (p *S3Provider) GeneratePresignedURL(ctx context.Context, bucket, path, method string, expiresIn int) (string, error) {
	presignClient := s3.NewPresignClient(p.client)

	duration := time.Duration(expiresIn) * time.Second

	switch strings.ToUpper(method) {
	case "GET":
		req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(path),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = duration
		})
		if err != nil {
			return "", fmt.Errorf("failed to presign GET request: %w", err)
		}
		return req.URL, nil

	case "PUT":
		req, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(path),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = duration
		})
		if err != nil {
			return "", fmt.Errorf("failed to presign PUT request: %w", err)
		}
		return req.URL, nil

	case "DELETE":
		req, err := presignClient.PresignDeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(path),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = duration
		})
		if err != nil {
			return "", fmt.Errorf("failed to presign DELETE request: %w", err)
		}
		return req.URL, nil

	default:
		return "", fmt.Errorf("unsupported HTTP method: %s", method)
	}
}

// CreateBucket creates a new S3 bucket
func (p *S3Provider) CreateBucket(ctx context.Context, bucketName string, options map[string]interface{}) error {
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}

	// Set bucket configuration for regions other than us-east-1
	if p.config.Region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(p.config.Region),
		}
	}

	_, err := p.client.CreateBucket(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create S3 bucket: %w", err)
	}

	return nil
}

// DeleteBucket deletes an S3 bucket
func (p *S3Provider) DeleteBucket(ctx context.Context, bucketName string) error {
	input := &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	}

	_, err := p.client.DeleteBucket(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete S3 bucket: %w", err)
	}

	return nil
}

// ListBuckets lists all S3 buckets
func (p *S3Provider) ListBuckets(ctx context.Context) ([]string, error) {
	result, err := p.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 buckets: %w", err)
	}

	buckets := make([]string, len(result.Buckets))
	for i, bucket := range result.Buckets {
		buckets[i] = *bucket.Name
	}

	return buckets, nil
}

// BucketExists checks if an S3 bucket exists
func (p *S3Provider) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	input := &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	}

	_, err := p.client.HeadBucket(ctx, input)
	if err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check S3 bucket existence: %w", err)
	}

	return true, nil
}

// BatchDelete deletes multiple objects from S3
func (p *S3Provider) BatchDelete(ctx context.Context, bucket string, paths []string) ([]string, []models.BatchError, error) {
	if len(paths) == 0 {
		return []string{}, []models.BatchError{}, nil
	}

	// S3 supports batch delete up to 1000 objects at a time
	const batchSize = 1000
	var successful []string
	var failed []models.BatchError

	for i := 0; i < len(paths); i += batchSize {
		end := i + batchSize
		if end > len(paths) {
			end = len(paths)
		}

		batch := paths[i:end]
		batchSuccessful, batchFailed := p.deleteBatch(ctx, bucket, batch)

		successful = append(successful, batchSuccessful...)
		failed = append(failed, batchFailed...)
	}

	return successful, failed, nil
}

// deleteBatch deletes a batch of objects
func (p *S3Provider) deleteBatch(ctx context.Context, bucket string, paths []string) ([]string, []models.BatchError) {
	objects := make([]types.ObjectIdentifier, len(paths))
	for i, path := range paths {
		objects[i] = types.ObjectIdentifier{
			Key: aws.String(path),
		}
	}

	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{
			Objects: objects,
		},
	}

	result, err := p.client.DeleteObjects(ctx, input)
	if err != nil {
		// If the entire batch fails, mark all as failed
		failed := make([]models.BatchError, len(paths))
		for i, path := range paths {
			failed[i] = models.BatchError{
				Path:  path,
				Error: err.Error(),
			}
		}
		return []string{}, failed
	}

	// Collect successful deletions
	successful := make([]string, len(result.Deleted))
	for i, deleted := range result.Deleted {
		successful[i] = *deleted.Key
	}

	// Collect failed deletions
	failed := make([]models.BatchError, len(result.Errors))
	for i, deleteError := range result.Errors {
		failed[i] = models.BatchError{
			Path:  *deleteError.Key,
			Error: fmt.Sprintf("%s: %s", *deleteError.Code, *deleteError.Message),
		}
	}

	return successful, failed
}

// GetUsageStats gets storage usage statistics for S3
func (p *S3Provider) GetUsageStats(ctx context.Context, bucket string) (*models.StorageUsage, error) {
	// Note: AWS S3 doesn't provide real-time usage statistics through API
	// This would typically require CloudWatch metrics or other AWS services
	// For now, we'll calculate by listing all objects (expensive for large buckets)

	var totalSize int64
	var count int64

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}

	paginator := s3.NewListObjectsV2Paginator(p.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get S3 usage stats: %w", err)
		}

		for _, obj := range page.Contents {
			totalSize += *obj.Size
			count++
		}
	}

	return &models.StorageUsage{
		TotalSize:     totalSize,
		DocumentCount: count,
		LastUpdated:   time.Now(),
	}, nil
}

// TestConnection tests the connection to S3
func (p *S3Provider) TestConnection(ctx context.Context) error {
	// Try to list buckets as a connection test
	_, err := p.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("S3 connection test failed: %w", err)
	}

	return nil
}
