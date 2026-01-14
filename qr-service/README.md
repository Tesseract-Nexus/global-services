# QR Service

A comprehensive QR code generation microservice built with Go. Supports multiple QR code types including URLs, WiFi credentials, vCards, payments (UPI, Bitcoin, Ethereum), and more.

## Features

- **10+ QR Code Types**: URL, Text, WiFi, vCard, Email, Phone, SMS, Geo Location, App Store Links, Payment
- **Multiple Output Formats**: PNG image or Base64 encoded string
- **Configurable Quality**: Low, Medium, High, Highest error correction levels
- **Batch Generation**: Generate up to 50 QR codes in a single request
- **GCP Cloud Storage**: Optional integration for persistent QR code storage
- **AES-256 Encryption**: Secure encryption for sensitive data (WiFi passwords, payment info)
- **Multi-Tenant Support**: Built-in tenant isolation via X-Tenant-ID header
- **Production Ready**: Health checks, structured logging, CORS, request tracing

## Quick Start

### Prerequisites

- Go 1.25+
- Docker (optional)
- GCP credentials (optional, for cloud storage)

### Running Locally

```bash
# Clone the repository
cd services/qr-service

# Download dependencies
make deps

# Run the service
make run
```

The service will be available at `http://localhost:8080`

### Using Docker

```bash
# Build the Docker image
make docker-build

# Run with docker-compose
make docker-run

# View logs
make docker-logs
```

## API Reference

### Base URL

```
http://localhost:8080
```

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/qr/generate` | Generate QR code (any type) |
| `GET` | `/api/v1/qr/image` | Generate QR as PNG image (simple types) |
| `POST` | `/api/v1/qr/download` | Download QR as file attachment |
| `POST` | `/api/v1/qr/batch` | Batch generate multiple QR codes |
| `GET` | `/api/v1/qr/types` | List all supported QR types |
| `GET` | `/health` | Health check |
| `GET` | `/ready` | Readiness check |

### Generate QR Code

**POST** `/api/v1/qr/generate`

Generate a QR code for any supported type.

#### Request Body

```json
{
  "type": "url",
  "data": {
    "url": "https://example.com"
  },
  "size": 256,
  "quality": "medium",
  "format": "base64",
  "save": false,
  "tenant_id": "optional-tenant-id",
  "label": "My QR Code"
}
```

#### Response

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "url",
  "qr_code": "iVBORw0KGgoAAAANSUhEUgAA...",
  "format": "base64",
  "size": 256,
  "quality": "medium",
  "encrypted": false,
  "created_at": "2024-01-15T10:30:00Z"
}
```

### Supported QR Types

#### 1. URL

```json
{
  "type": "url",
  "data": {
    "url": "https://example.com"
  }
}
```

#### 2. Plain Text

```json
{
  "type": "text",
  "data": {
    "text": "Hello, World!"
  }
}
```

#### 3. WiFi Network

```json
{
  "type": "wifi",
  "data": {
    "wifi": {
      "ssid": "MyNetwork",
      "password": "secretpass123",
      "encryption": "WPA",
      "hidden": false
    }
  }
}
```

Encryption options: `WPA`, `WEP`, `nopass`

#### 4. Contact (vCard)

```json
{
  "type": "vcard",
  "data": {
    "vcard": {
      "first_name": "John",
      "last_name": "Doe",
      "email": "john.doe@example.com",
      "phone": "+1234567890",
      "mobile": "+0987654321",
      "organization": "Acme Inc",
      "title": "Software Engineer",
      "address": "123 Main St",
      "city": "New York",
      "state": "NY",
      "zip": "10001",
      "country": "USA",
      "website": "https://johndoe.com",
      "note": "Met at conference"
    }
  }
}
```

#### 5. Email

```json
{
  "type": "email",
  "data": {
    "email": {
      "address": "contact@example.com",
      "subject": "Hello from QR",
      "body": "I scanned your QR code!"
    }
  }
}
```

#### 6. Phone Call

```json
{
  "type": "phone",
  "data": {
    "phone": "+1234567890"
  }
}
```

#### 7. SMS Message

```json
{
  "type": "sms",
  "data": {
    "sms": {
      "phone": "+1234567890",
      "message": "Hello from QR!"
    }
  }
}
```

#### 8. Geographic Location

```json
{
  "type": "geo",
  "data": {
    "geo": {
      "latitude": 40.7128,
      "longitude": -74.0060,
      "altitude": 10.5
    }
  }
}
```

#### 9. App Store Links

```json
{
  "type": "app",
  "data": {
    "app": {
      "ios_url": "https://apps.apple.com/app/id123456",
      "android_url": "https://play.google.com/store/apps/details?id=com.example",
      "fallback_url": "https://example.com/download"
    }
  }
}
```

#### 10. Payment (UPI)

```json
{
  "type": "payment",
  "data": {
    "payment": {
      "type": "upi",
      "upi_id": "merchant@upi",
      "name": "Store Name",
      "amount": 100.50,
      "currency": "INR",
      "reference": "ORDER123"
    }
  }
}
```

#### 11. Payment (Bitcoin)

```json
{
  "type": "payment",
  "data": {
    "payment": {
      "type": "bitcoin",
      "address": "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
      "amount": 0.001
    }
  }
}
```

#### 12. Payment (Ethereum)

```json
{
  "type": "payment",
  "data": {
    "payment": {
      "type": "ethereum",
      "address": "0x742d35Cc6634C0532925a3b844Bc9e7595f...",
      "amount": 0.5
    }
  }
}
```

### Get QR Code as Image

**GET** `/api/v1/qr/image`

Generate a QR code and return it directly as a PNG image. Only supports simple types (url, text, phone).

#### Query Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `type` | string | No | `url` | QR code type (url, text, phone) |
| `url` | string | Yes* | - | URL for URL type |
| `text` | string | Yes* | - | Text for text type |
| `phone` | string | Yes* | - | Phone for phone type |
| `size` | int | No | `256` | Size in pixels (64-1024) |
| `quality` | string | No | `medium` | Error correction level |

*Required based on type

#### Example

```bash
curl "http://localhost:8080/api/v1/qr/image?type=url&url=https://example.com&size=512" \
  --output qrcode.png
```

### Batch Generate QR Codes

**POST** `/api/v1/qr/batch`

Generate multiple QR codes in a single request.

#### Request Body

```json
{
  "items": [
    {
      "type": "url",
      "data": { "url": "https://example1.com" },
      "label": "Product 1"
    },
    {
      "type": "url",
      "data": { "url": "https://example2.com" },
      "label": "Product 2"
    },
    {
      "type": "wifi",
      "data": {
        "wifi": { "ssid": "GuestWiFi", "password": "welcome123" }
      },
      "label": "Guest WiFi"
    }
  ],
  "size": 256,
  "quality": "medium",
  "save": false
}
```

#### Response

```json
{
  "results": [
    {
      "label": "Product 1",
      "qr_code": "iVBORw0KGgoAAAANSUhEUgAA..."
    },
    {
      "label": "Product 2",
      "qr_code": "iVBORw0KGgoAAAANSUhEUgAA..."
    },
    {
      "label": "Guest WiFi",
      "qr_code": "iVBORw0KGgoAAAANSUhEUgAA..."
    }
  ],
  "total": 3,
  "success": 3,
  "failed": 0
}
```

## Configuration

The service is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `GIN_MODE` | Gin mode (debug/release) | `release` |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `QR_DEFAULT_SIZE` | Default QR code size | `256` |
| `QR_MAX_SIZE` | Maximum QR code size | `1024` |
| `QR_MIN_SIZE` | Minimum QR code size | `64` |
| `QR_DEFAULT_QUALITY` | Default error correction | `medium` |
| `STORAGE_PROVIDER` | Storage provider (local/gcs) | `local` |
| `GCS_BUCKET_NAME` | GCS bucket name | - |
| `GCS_BASE_PATH` | Base path in bucket | `qr-codes` |
| `GCS_PUBLIC_URL` | GCS public URL | `https://storage.googleapis.com` |
| `ENCRYPTION_ENABLED` | Enable encryption | `true` |
| `ENCRYPTION_KEY` | AES-256 encryption key | - |

### GCP Cloud Storage Setup

To enable GCP Cloud Storage:

1. Create a GCS bucket
2. Create a service account with Storage Admin role
3. Download the credentials JSON
4. Set environment variables:

```bash
export STORAGE_PROVIDER=gcs
export GCS_BUCKET_NAME=your-bucket-name
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json
```

### Encryption

Sensitive data (WiFi passwords, payment info) is automatically encrypted when `ENCRYPTION_ENABLED=true`.

Generate an encryption key:

```bash
make generate-key
# or
openssl rand -hex 32
```

## Development

### Project Structure

```
qr-service/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration
│   ├── handlers/
│   │   ├── health_handlers.go
│   │   ├── qr_handlers.go
│   │   └── handlers_test.go
│   ├── middleware/
│   │   └── middleware.go    # CORS, logging, etc.
│   ├── models/
│   │   └── qr.go            # Data models
│   └── services/
│       ├── encryption_service.go
│       ├── encryption_service_test.go
│       ├── qr_service.go
│       ├── qr_service_test.go
│       └── storage_service.go
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
├── Makefile
├── openapi.yaml
└── README.md
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with race detector
make test-race

# Run tests with coverage
make test-cover
```

### Building

```bash
# Build binary
make build

# Build Docker image
make docker-build
```

### Linting

```bash
make lint
```

## Deployment

### Kubernetes (Helm)

The Helm chart is located at `tesserix-k8s/charts/apps/qr-service/`.

```bash
# Install
helm install qr-service ./charts/apps/qr-service -n default

# Install with production values
helm install qr-service ./charts/apps/qr-service \
  -f ./charts/apps/qr-service/values-prod.yaml \
  -n production
```

### ArgoCD

The ArgoCD application manifest is at `tesserix-k8s/argocd/devtest/apps/qr-service.yaml`.

## API Documentation

- **OpenAPI Spec**: `services/qr-service/openapi.yaml`
- **Swagger UI**: Import the OpenAPI spec into Swagger UI

## Error Codes

| Code | Description |
|------|-------------|
| `INVALID_REQUEST` | Request validation failed |
| `MISSING_URL` | URL parameter is required |
| `MISSING_TEXT` | Text parameter is required |
| `MISSING_PHONE` | Phone parameter is required |
| `USE_POST` | Complex types require POST endpoint |
| `GENERATION_FAILED` | QR code generation failed |

## Health Checks

- **Liveness**: `GET /health`
- **Readiness**: `GET /ready`

Both return:

```json
{
  "status": "healthy",
  "service": "qr-service",
  "version": "1.0.0",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## License

MIT License - see LICENSE file for details.
