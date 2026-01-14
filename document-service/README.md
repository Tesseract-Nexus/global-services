# Document Service

A cloud-agnostic document management service that provides a unified API for document storage operations across AWS S3, Azure Blob Storage, and Google Cloud Storage.

## Features

- **Cloud Agnostic**: Support for AWS S3, Azure Blob Storage, and Google Cloud Storage
- **RESTful API**: Complete REST API for document operations
- **File Validation**: MIME type validation, file size limits, and security checks
- **Metadata Management**: Rich metadata support with custom tags
- **Presigned URLs**: Generate secure, time-limited URLs for direct client access
- **Batch Operations**: Efficient batch upload/delete operations
- **Streaming Support**: Large file streaming for memory-efficient operations
- **Multi-tenancy**: Built-in tenant isolation support
- **Health Checks**: Comprehensive health, readiness, and liveness endpoints
- **Observability**: Structured logging and metrics-ready
- **Security**: CORS, rate limiting, authentication support

## Quick Start

### Using Docker Compose (Recommended for Development)

1. Clone the repository and navigate to the document service directory
2. Copy the example configuration:
   ```bash
   cp config.example.yaml config.yaml
   ```
3. Update the configuration with your settings
4. Start the services:
   ```bash
   docker-compose up -d
   ```

The service will be available at `http://localhost:8082`

### Local Development

#### Prerequisites

- Go 1.21 or later
- PostgreSQL 12 or later
- Access to AWS S3, Azure Blob Storage, or Google Cloud Storage

#### Setup

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Set up your database:
   ```bash
   createdb document_service
   ```

3. Copy and configure the settings:
   ```bash
   cp config.example.yaml config.yaml
   # Edit config.yaml with your settings
   ```

4. Set environment variables:
   ```bash
   export DB_PASSWORD=your_db_password
   export AWS_ACCESS_KEY_ID=your_aws_key
   export AWS_SECRET_ACCESS_KEY=your_aws_secret
   # Or use IAM roles/instance profiles
   ```

5. Run the service:
   ```bash
   go run cmd/main.go
   ```

## Configuration

### Environment Variables

Key environment variables (see `config.example.yaml` for all options):

| Variable | Description | Example |
|----------|-------------|---------|
| `STORAGE_PROVIDER` | Cloud provider (aws/azure/gcp) | `aws` |
| `STORAGE_DEFAULT_BUCKET` | Default storage bucket | `my-documents` |
| `DB_HOST` | Database host | `localhost` |
| `DB_PASSWORD` | Database password | `password` |
| `AWS_REGION` | AWS region | `us-east-1` |
| `AWS_ACCESS_KEY_ID` | AWS access key | `AKIA...` |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | `...` |

### Cloud Provider Setup

#### AWS S3

1. Create an S3 bucket
2. Set up IAM user/role with appropriate permissions:
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "s3:GetObject",
           "s3:PutObject",
           "s3:DeleteObject",
           "s3:ListBucket"
         ],
         "Resource": [
           "arn:aws:s3:::your-bucket-name",
           "arn:aws:s3:::your-bucket-name/*"
         ]
       }
     ]
   }
   ```

#### Azure Blob Storage

1. Create a storage account
2. Create a container
3. Set up authentication (account key, SAS token, or managed identity)

#### Google Cloud Storage

1. Create a project and enable Cloud Storage API
2. Create a bucket
3. Set up service account with Storage Object Admin role
4. Download service account key file

## API Documentation

### Core Endpoints

#### Upload Document
```http
POST /api/v1/documents/upload
Content-Type: multipart/form-data

file: [binary data]
bucket: optional-bucket-name
isPublic: true/false
tags: key1:value1,key2:value2
```

#### Download Document
```http
GET /api/v1/documents/{bucket}/{path}
```

#### Get Document Metadata
```http
GET /api/v1/documents/{bucket}/{path}/metadata
```

#### Delete Document
```http
DELETE /api/v1/documents/{bucket}/{path}
```

#### List Documents
```http
GET /api/v1/documents?bucket=mybucket&prefix=2024/&limit=50
```

#### Generate Presigned URL
```http
POST /api/v1/documents/presigned-url
Content-Type: application/json

{
  "path": "documents/file.pdf",
  "bucket": "my-bucket",
  "method": "GET",
  "expiresIn": 3600
}
```

#### Batch Operations
```http
POST /api/v1/documents/batch/delete
Content-Type: application/json

{
  "paths": ["file1.pdf", "file2.pdf"],
  "bucket": "my-bucket"
}
```

### Health Endpoints

- `GET /health` - Basic health check
- `GET /health/ready` - Readiness probe
- `GET /health/live` - Liveness probe

## Usage Examples

### Upload a File
```bash
curl -X POST http://localhost:8082/api/v1/documents/upload \
  -F "file=@document.pdf" \
  -F "bucket=my-documents" \
  -F "isPublic=false" \
  -F "tags=category:invoice,year:2024"
```

### Download a File
```bash
curl -O http://localhost:8082/api/v1/documents/my-documents/2024/01/15/document.pdf
```

### Get Document Metadata
```bash
curl http://localhost:8082/api/v1/documents/my-documents/2024/01/15/document.pdf/metadata
```

### Generate Presigned URL
```bash
curl -X POST http://localhost:8082/api/v1/documents/presigned-url \
  -H "Content-Type: application/json" \
  -d '{
    "path": "2024/01/15/document.pdf",
    "bucket": "my-documents",
    "method": "GET",
    "expiresIn": 3600
  }'
```

## Security

### Authentication
The service supports JWT-based authentication. Include the JWT token in the Authorization header:
```http
Authorization: Bearer your-jwt-token
```

### Multi-tenancy
Use the `X-Tenant-ID` header to isolate documents by tenant:
```http
X-Tenant-ID: tenant-123
```

### CORS
Configure allowed origins in the configuration file:
```yaml
security:
  enable_cors: true
  allowed_origins:
    - "https://your-frontend.com"
```

## Monitoring and Observability

### Health Checks
- **Health**: `/health` - Basic service health
- **Readiness**: `/health/ready` - Ready to accept traffic
- **Liveness**: `/health/live` - Service is alive

### Logging
Structured JSON logging with configurable levels:
```yaml
logging:
  level: "info"
  format: "json"
  time_format: "2006-01-02T15:04:05Z07:00"
```

### Metrics
The service is designed to be metrics-ready. You can integrate with:
- Prometheus (add metrics middleware)
- New Relic
- DataDog
- OpenTelemetry

## Development

### Project Structure
```
document-service/
├── cmd/                    # Application entry points
│   └── main.go
├── internal/              # Private application code
│   ├── config/           # Configuration management
│   ├── handlers/         # HTTP handlers
│   ├── models/           # Data models and interfaces
│   ├── providers/        # Cloud provider implementations
│   │   ├── aws/         # AWS S3 provider
│   │   ├── azure/       # Azure Blob provider
│   │   └── gcp/         # Google Cloud Storage provider
│   ├── repository/      # Data persistence layer
│   ├── service/         # Business logic
│   └── utils/           # Utility functions
├── docs/                # Documentation
├── migrations/          # Database migrations
├── config.example.yaml  # Configuration template
├── docker-compose.yml   # Development environment
└── Dockerfile          # Container definition
```

### Running Tests
```bash
go test ./...
```

### Building
```bash
go build -o document-service cmd/main.go
```

### Adding a New Cloud Provider
1. Implement the `CloudStorageProvider` interface in `internal/models/interfaces.go`
2. Add the provider implementation in `internal/providers/{provider}/`
3. Update the provider factory in `cmd/main.go`
4. Add configuration options in `internal/config/config.go`

## Production Deployment

### Docker
```bash
docker build -t document-service .
docker run -d \
  -p 8082:8082 \
  -e DB_HOST=your-db-host \
  -e AWS_REGION=us-east-1 \
  -e STORAGE_DEFAULT_BUCKET=your-bucket \
  document-service
```

### Kubernetes
See the `docs/kubernetes/` directory for example manifests.

### Environment Considerations
- Use IAM roles instead of access keys in AWS
- Enable SSL/TLS in production
- Configure proper CORS origins
- Set up monitoring and alerting
- Use secrets management for sensitive data
- Configure rate limiting
- Set appropriate file size limits

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `go fmt`, `go vet`, and tests
6. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Support

For issues and questions:
- Create an issue in the repository
- Check the documentation in `docs/`
- Review the configuration examples