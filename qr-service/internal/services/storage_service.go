package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
)

type StorageService struct {
	client     *storage.Client
	bucketName string
	basePath   string
	publicURL  string
}

type StorageConfig struct {
	BucketName string
	BasePath   string
	PublicURL  string
}

func NewStorageService(ctx context.Context, cfg StorageConfig) (*StorageService, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	return &StorageService{
		client:     client,
		bucketName: cfg.BucketName,
		basePath:   cfg.BasePath,
		publicURL:  cfg.PublicURL,
	}, nil
}

func (s *StorageService) Upload(ctx context.Context, objectName string, data []byte, contentType string) (string, error) {
	fullPath := fmt.Sprintf("%s/%s", s.basePath, objectName)

	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(fullPath)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	writer.CacheControl = "public, max-age=86400"

	if _, err := writer.Write(data); err != nil {
		return "", fmt.Errorf("failed to write object: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close writer: %w", err)
	}

	// Note: Object ACL not set as bucket uses uniform bucket-level access
	// Public access is controlled via bucket IAM policy

	url := fmt.Sprintf("%s/%s/%s", s.publicURL, s.bucketName, fullPath)
	return url, nil
}

func (s *StorageService) Download(ctx context.Context, objectName string) ([]byte, error) {
	fullPath := fmt.Sprintf("%s/%s", s.basePath, objectName)

	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(fullPath)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return data, nil
}

func (s *StorageService) Delete(ctx context.Context, objectName string) error {
	fullPath := fmt.Sprintf("%s/%s", s.basePath, objectName)

	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(fullPath)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

func (s *StorageService) GetSignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	fullPath := fmt.Sprintf("%s/%s", s.basePath, objectName)

	opts := &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expiry),
	}

	url, err := s.client.Bucket(s.bucketName).SignedURL(fullPath, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return url, nil
}

func (s *StorageService) Exists(ctx context.Context, objectName string) (bool, error) {
	fullPath := fmt.Sprintf("%s/%s", s.basePath, objectName)

	bucket := s.client.Bucket(s.bucketName)
	obj := bucket.Object(fullPath)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := obj.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	return true, nil
}

func (s *StorageService) Close() error {
	return s.client.Close()
}

type LocalStorageService struct {
	basePath  string
	publicURL string
}

func NewLocalStorageService(basePath, publicURL string) *LocalStorageService {
	return &LocalStorageService{
		basePath:  basePath,
		publicURL: publicURL,
	}
}

func (s *LocalStorageService) Upload(ctx context.Context, objectName string, data []byte, contentType string) (string, error) {
	return fmt.Sprintf("%s/%s", s.publicURL, objectName), nil
}

func (s *LocalStorageService) Download(ctx context.Context, objectName string) ([]byte, error) {
	return nil, fmt.Errorf("local storage download not implemented")
}

func (s *LocalStorageService) Delete(ctx context.Context, objectName string) error {
	return nil
}

func (s *LocalStorageService) Close() error {
	return nil
}
