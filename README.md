# Global Services

Platform infrastructure microservices for Tesseract multi-tenant SaaS platform. These services are deployed in the **global namespace** and shared across all tenants.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        GLOBAL NAMESPACE                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────────┐    │
│  │auth-service │→ │  auth-bff   │→ │ tenant-router-service│    │
│  └─────────────┘  └─────────────┘  └──────────────────────┘    │
│         ↓                                    ↓                   │
│  ┌─────────────┐  ┌──────────────────┐  ┌─────────────────┐    │
│  │tenant-service│→ │verification-svc  │→ │notification-svc │    │
│  └─────────────┘  └──────────────────┘  └─────────────────┘    │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │location-svc │  │document-svc │  │  search-svc │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
```

## Services

### Core Authentication & Tenant Management

| Service | Port | Description |
|---------|------|-------------|
| `auth-service` | 8081 | Keycloak integration, JWT validation, session management |
| `auth-bff` | 8082 | Backend-for-frontend for authentication flows |
| `tenant-service` | 8083 | Tenant onboarding, multi-tenant management, slug routing |
| `tenant-router-service` | 8084 | Request routing based on tenant subdomains/slugs |
| `verification-service` | 8085 | Email/phone verification, OTP generation |

### Communication & Notifications

| Service | Port | Description |
|---------|------|-------------|
| `notification-service` | 8086 | Email/SMS delivery (Postal, SES, Twilio, SendGrid) |
| `notification-hub` | 8087 | Real-time bell notifications, SSE streaming |

### Shared Business Logic

| Service | Port | Description |
|---------|------|-------------|
| `location-service` | 8088 | Countries, states, cities, timezones, currencies |
| `translation-service` | 8089 | i18n/l10n translations |
| `document-service` | 8090 | File upload, GCS storage, document management |
| `search-service` | 8091 | Elasticsearch integration for global search |
| `qr-service` | 8092 | QR code generation for products, orders |

### Platform Management

| Service | Port | Description |
|---------|------|-------------|
| `settings-service` | 8093 | Global and tenant-specific settings |
| `audit-service` | 8094 | Audit trails, compliance logging |
| `feature-flags-service` | 8095 | Feature toggles, A/B testing |

### Analytics & Monitoring

| Service | Port | Description |
|---------|------|-------------|
| `analytics-service` | 8096 | Platform-wide analytics, reporting |
| `status-dashboard-service` | 8097 | Health monitoring, service status |

## Dependencies

All services depend on:
- **go-shared**: `github.com/Tesseract-Nexus/go-shared`
- **PostgreSQL**: Primary database
- **Redis**: Caching, rate limiting, sessions
- **NATS**: Event streaming
- **Keycloak**: Identity provider

## Quick Start

### Prerequisites
- Go 1.25+
- Docker & Docker Compose
- PostgreSQL 14+
- Redis 7+
- NATS 2.9+

### Running Locally

```bash
# Run all services with Docker Compose
docker-compose up -d

# Or run individual service
cd auth-service
go run main.go
```

### Environment Variables

Each service requires a `.env` file. Copy from example:
```bash
cp .env.example .env
```

Common variables:
```env
# Server
PORT=8081
ENVIRONMENT=development

# Database
DATABASE_URL=postgres://user:pass@localhost:5432/dbname

# Redis
REDIS_URL=redis://localhost:6379

# NATS
NATS_URL=nats://localhost:4222

# Keycloak
KEYCLOAK_URL=http://localhost:8080
KEYCLOAK_REALM=tesseract

# GCP (for secrets)
GCP_PROJECT_ID=your-project
```

## Service Communication

### NATS Event Streams

| Stream | Subjects | Publishers |
|--------|----------|------------|
| `TENANT_EVENTS` | tenant.* | tenant-service |
| `AUTH_EVENTS` | auth.* | auth-service |
| `NOTIFICATION_EVENTS` | notification.* | notification-service |
| `VERIFICATION_EVENTS` | verification.* | verification-service |

### Internal HTTP APIs

Services communicate via internal HTTP APIs:
- Auth validation: `auth-service/api/v1/validate`
- Tenant lookup: `tenant-service/api/v1/tenants/{id}`
- Notification send: `notification-service/api/v1/send`

## Deployment

### Kubernetes

Services are deployed to the `global` namespace:
```bash
kubectl apply -f k8s/ -n global
```

### CI/CD

Each service has its own GitHub Actions workflow:
- Build on push to main
- Run tests
- Build Docker image
- Push to GHCR
- Deploy to GKE

## Project Structure

```
global-services/
├── auth-service/
│   ├── cmd/
│   ├── internal/
│   ├── api/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── tenant-service/
│   └── ...
├── notification-service/
│   └── ...
├── .github/
│   └── workflows/
├── docker-compose.yml
├── .gitignore
└── README.md
```

## License

Proprietary - Tesseract Nexus
