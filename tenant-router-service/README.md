# Tenant Router Service

Dynamic tenant routing infrastructure microservice for the Tesseract Hub platform. Manages SSL certificates, Istio Gateways, and VirtualServices for multi-tenant environments in Kubernetes.

## Features

- **Event-Driven**: NATS JetStream subscription for tenant events
- **Certificate Management**: Automatic Let's Encrypt SSL via cert-manager
- **Istio Integration**: Gateway and VirtualService provisioning
- **Domain Routing**: Subdomain-based tenant identification
- **Kubernetes Native**: Full K8s API integration

## Tech Stack

- **Language**: Go
- **Framework**: Gin
- **Messaging**: NATS JetStream
- **Infrastructure**: Kubernetes, Istio, cert-manager
- **Port**: 8089

## API Endpoints

### Health
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Liveness probe |
| GET | `/ready` | Readiness probe with dependency checks |

### Host Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/hosts` | List all tenant hosts |
| GET | `/api/v1/hosts/:slug` | Get specific tenant host |
| POST | `/api/v1/hosts/:slug` | Add/provision tenant host |
| DELETE | `/api/v1/hosts/:slug` | Remove tenant host |
| POST | `/api/v1/hosts/:slug/sync` | Force sync tenant config |

## Event Subscriptions

### NATS JetStream Topics
- `tenant.created` - Triggers provisioning
- `tenant.deleted` - Triggers deprovisioning

### Event Models
```go
TenantCreatedEvent {
    TenantID, SessionID, Product
    BusinessName, Slug, Email
    Timestamp
}

TenantDeletedEvent {
    TenantID, Slug, Timestamp
}
```

## Provisioning Flow

When a `tenant.created` event is received:

1. **Certificate**: Creates TLS certificate for both domains
   - Admin: `{slug}-admin.{baseDomain}`
   - Storefront: `{slug}.{baseDomain}`

2. **Gateway**: Adds server entries to Istio Gateway

3. **VirtualService**: Adds routing rules for traffic

## Environment Variables

```env
# Server
SERVER_PORT=8089
GIN_MODE=debug

# NATS
NATS_URL=nats://nats.devtest.svc.cluster.local:4222

# Kubernetes
K8S_NAMESPACE=devtest
ISTIO_NAMESPACE=istio-system
GATEWAY_NAME=main-gateway
ADMIN_VS_NAME=admin-vs
STOREFRONT_VS_NAME=storefront-vs
CLUSTER_ISSUER=letsencrypt-prod

# Domain
BASE_DOMAIN=tesserix.app
```

## Slug Validation

- Regex: `^[a-z0-9][a-z0-9-]*[a-z0-9]$`
- Length: 2-63 characters (DNS-compliant)
- Only lowercase alphanumeric and hyphens
- Cannot start or end with hyphen

## Generated Domains

For a tenant with slug `acme`:
- Admin: `acme-admin.tesserix.app`
- Storefront: `acme.tesserix.app`

## Kubernetes Resources Created

### Certificate
- Let's Encrypt backed via ClusterIssuer
- Both admin and storefront domains

### Gateway Server Entry
- TLS termination
- Port 443 HTTPS

### VirtualService Hosts
- Routing rules for admin traffic
- Routing rules for storefront traffic

## In-Memory Cache

- Thread-safe with RWMutex
- Caches provisioned tenant hosts
- Caches K8s resource namespaces

## Reliability Features

- Manual ACK for NATS messages
- Retry up to 3 times on failure
- 30-second ACK wait timeout
- Graceful shutdown (10 seconds)
- Idempotent operations

## Dependencies

- **NATS**: Event streaming
- **cert-manager**: Certificate automation
- **Istio**: Service mesh (Gateway, VirtualService)
- **Kubernetes**: Container orchestration

## Running Locally

```bash
# Requires Kubernetes cluster with Istio and cert-manager

# Set environment variables
cp .env.example .env

# Build and run
make docker-build
make docker-push

# Or run directly (with kubeconfig)
go run cmd/main.go
```

## License

MIT
