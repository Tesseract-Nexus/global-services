package events

import (
	"context"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/tesseract-hub/go-shared/events"
)

var (
	publisher     *Publisher
	publisherOnce sync.Once
	publisherMu   sync.RWMutex
)

// Publisher wraps the shared events publisher for document-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// InitPublisher initializes the singleton NATS publisher
func InitPublisher(logger *logrus.Logger) error {
	var initErr error
	publisherOnce.Do(func() {
		natsURL := os.Getenv("NATS_URL")
		if natsURL == "" {
			logger.Warn("NATS_URL not set, event publishing disabled")
			return
		}

		config := events.DefaultPublisherConfig(natsURL)
		config.Name = "document-service"

		pub, err := events.NewPublisher(config, logger)
		if err != nil {
			initErr = err
			return
		}

		ctx := context.Background()
		if err := pub.EnsureStream(ctx, events.StreamDocuments, []string{"document.>"}); err != nil {
			logger.WithError(err).Warn("Failed to ensure DOCUMENT_EVENTS stream")
		}

		publisherMu.Lock()
		publisher = &Publisher{
			publisher: pub,
			logger:    logger.WithField("component", "events.publisher"),
		}
		publisherMu.Unlock()

		logger.Info("NATS events publisher initialized for document-service")
	})
	return initErr
}

// GetPublisher returns the singleton publisher instance
func GetPublisher() *Publisher {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return publisher
}

// PublishDocumentUploaded publishes a document uploaded event
func (p *Publisher) PublishDocumentUploaded(ctx context.Context, tenantID, productID, documentID, documentType, fileName, mimeType, bucketName, objectPath string, fileSize int64, ownerID, ownerType, uploadedBy string) error {
	event := events.NewDocumentEvent(events.DocumentUploaded, tenantID)
	event.DocumentID = documentID
	event.DocumentType = documentType
	event.FileName = fileName
	event.MimeType = mimeType
	event.FileSize = fileSize
	event.BucketName = bucketName
	event.ObjectPath = objectPath
	event.ProductID = productID
	event.SourceService = "document-service"
	event.OwnerID = ownerID
	event.OwnerType = ownerType
	event.UploadedBy = uploadedBy
	event.Status = "UPLOADED"

	return p.publisher.Publish(ctx, event)
}

// PublishDocumentProcessed publishes a document processed event
func (p *Publisher) PublishDocumentProcessed(ctx context.Context, tenantID, productID, documentID, processingType string) error {
	event := events.NewDocumentEvent(events.DocumentProcessed, tenantID)
	event.DocumentID = documentID
	event.ProductID = productID
	event.SourceService = "document-service"
	event.ProcessingType = processingType
	event.Status = "PROCESSED"

	return p.publisher.Publish(ctx, event)
}

// PublishDocumentDeleted publishes a document deleted event
func (p *Publisher) PublishDocumentDeleted(ctx context.Context, tenantID, productID, documentID, bucketName, objectPath, deletedBy string) error {
	event := events.NewDocumentEvent(events.DocumentDeleted, tenantID)
	event.DocumentID = documentID
	event.ProductID = productID
	event.SourceService = "document-service"
	event.BucketName = bucketName
	event.ObjectPath = objectPath
	event.UploadedBy = deletedBy // Reusing field for actor
	event.Status = "DELETED"

	return p.publisher.Publish(ctx, event)
}

// PublishDocumentVerified publishes a document verified event
func (p *Publisher) PublishDocumentVerified(ctx context.Context, tenantID, productID, documentID, documentType, verifiedBy, verifiedAt string) error {
	event := events.NewDocumentEvent(events.DocumentVerified, tenantID)
	event.DocumentID = documentID
	event.ProductID = productID
	event.SourceService = "document-service"
	event.DocumentType = documentType
	event.VerifiedBy = verifiedBy
	event.VerifiedAt = verifiedAt
	event.Status = "VERIFIED"

	return p.publisher.Publish(ctx, event)
}

// PublishDocumentExpired publishes a document expired event
func (p *Publisher) PublishDocumentExpired(ctx context.Context, tenantID, productID, documentID, documentType, ownerID, ownerType string) error {
	event := events.NewDocumentEvent(events.DocumentExpired, tenantID)
	event.DocumentID = documentID
	event.ProductID = productID
	event.SourceService = "document-service"
	event.DocumentType = documentType
	event.OwnerID = ownerID
	event.OwnerType = ownerType
	event.Status = "EXPIRED"

	return p.publisher.Publish(ctx, event)
}

// IsConnected returns true if connected to NATS
func (p *Publisher) IsConnected() bool {
	return p.publisher != nil && p.publisher.IsConnected()
}

// Close closes the publisher connection
func (p *Publisher) Close() {
	if p.publisher != nil {
		p.publisher.Close()
	}
}
