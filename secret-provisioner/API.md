# Secret Provisioner Service API

## Overview

The Secret Provisioner Service is a global service that manages secrets in GCP Secret Manager. It provides a secure, centralized way for services to provision and manage secrets without direct access to GCP.

**Key Principles:**
- Never returns secret values in API responses
- All operations are logged for audit
- Internal service-to-service authentication required
- Multi-tenant with strict isolation

## Authentication

All API endpoints (except health checks) require the `X-Internal-Service` header with an authorized service name.

```http
X-Internal-Service: admin-bff
X-Tenant-ID: t_123
X-Actor-ID: user_456
X-Request-ID: req_789
```

## Base URL

```
http://secret-provisioner.global-services.svc.cluster.local/api/v1
```

---

## Endpoints

### Health Check

#### GET /health
Liveness probe - returns basic health status.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-26T12:00:00Z"
}
```

#### GET /ready
Readiness probe - checks database connectivity.

**Response:**
```json
{
  "status": "healthy",
  "checks": {
    "database": "healthy"
  },
  "timestamp": "2025-01-26T12:00:00Z"
}
```

---

### Provision Secrets

#### POST /api/v1/secrets

Creates or updates secrets in GCP Secret Manager.

**Request Headers:**
```http
X-Internal-Service: admin-bff
X-Tenant-ID: t_123
X-Actor-ID: user_456
Content-Type: application/json
```

**Request Body:**
```json
{
  "tenant_id": "t_123",
  "category": "payment",
  "scope": "vendor",
  "scope_id": "v_99",
  "provider": "stripe",
  "secrets": {
    "api-key": "sk_live_xxx",
    "webhook-secret": "whsec_xxx"
  },
  "metadata": {
    "created_by": "admin@example.com"
  },
  "validate": true
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| tenant_id | string | Yes | Tenant identifier |
| category | string | Yes | Secret category: `payment`, `integration`, `api-key`, `oauth`, `database`, `webhook` |
| scope | string | Yes | Scope level: `tenant`, `vendor`, `service` |
| scope_id | string | No* | Required if scope is `vendor` |
| provider | string | Yes | Provider name (e.g., `stripe`, `razorpay`) |
| secrets | object | Yes | Key-value pairs of secrets to provision |
| metadata | object | No | Additional metadata (non-secret) |
| validate | boolean | No | Whether to validate credentials with provider |

**Response (200 OK):**
```json
{
  "status": "ok",
  "secret_refs": [
    {
      "name": "prod-tenant-t_123-vendor-v_99-stripe-api-key",
      "category": "payment",
      "provider": "stripe",
      "key": "api-key",
      "version": "1",
      "created_at": "2025-01-26T12:00:00Z"
    },
    {
      "name": "prod-tenant-t_123-vendor-v_99-stripe-webhook-secret",
      "category": "payment",
      "provider": "stripe",
      "key": "webhook-secret",
      "version": "1",
      "created_at": "2025-01-26T12:00:00Z"
    }
  ],
  "validation": {
    "status": "VALID",
    "message": "Stripe API key is valid",
    "details": {
      "account_id": "acct_xxx",
      "country": "US"
    }
  }
}
```

**Note:** Response NEVER contains secret values.

---

### Get Secret Metadata

#### GET /api/v1/secrets/metadata

Retrieves metadata about configured secrets.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| tenant_id | string | Yes | Tenant identifier |
| category | string | No | Filter by category |
| provider | string | No | Filter by provider |
| scope | string | No | Filter by scope |
| scope_id | string | No | Filter by scope ID |

**Example:**
```
GET /api/v1/secrets/metadata?tenant_id=t_123&category=payment&provider=stripe
```

**Response (200 OK):**
```json
{
  "secrets": [
    {
      "name": "prod-tenant-t_123-payment-stripe-api-key",
      "category": "payment",
      "provider": "stripe",
      "key_name": "api-key",
      "scope": "tenant",
      "configured": true,
      "validation_status": "VALID",
      "last_updated": "2025-01-26T12:00:00Z"
    }
  ]
}
```

---

### List Configured Providers

#### GET /api/v1/secrets/providers

Lists which providers are configured for a tenant.

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| tenant_id | string | Yes | Tenant identifier |
| category | string | Yes | Category to list providers for |

**Example:**
```
GET /api/v1/secrets/providers?tenant_id=t_123&category=payment
```

**Response (200 OK):**
```json
{
  "tenant_id": "t_123",
  "category": "payment",
  "providers": [
    {
      "provider": "stripe",
      "tenant_configured": true,
      "vendor_configurations": [
        {
          "vendor_id": "v_99",
          "configured": true
        }
      ]
    },
    {
      "provider": "razorpay",
      "tenant_configured": false,
      "vendor_configurations": []
    }
  ]
}
```

---

### Delete Secret

#### DELETE /api/v1/secrets/:name

Deletes a secret from GCP Secret Manager.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| name | string | GCP secret name to delete |

**Request Headers:**
```http
X-Internal-Service: admin-bff
X-Tenant-ID: t_123
X-Actor-ID: user_456
```

**Response (200 OK):**
```json
{
  "status": "ok",
  "message": "Secret deleted successfully"
}
```

---

## Error Responses

All errors follow this format:

```json
{
  "error": "ERROR_CODE",
  "message": "Human-readable error message",
  "details": "Additional details if available"
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| INVALID_REQUEST | 400 | Request validation failed |
| MISSING_TENANT_ID | 400 | Tenant ID header/parameter missing |
| UNAUTHORIZED | 401 | X-Internal-Service header missing |
| FORBIDDEN | 403 | Service not authorized |
| TENANT_MISMATCH | 403 | Tenant ID doesn't match authenticated tenant |
| PROVISIONING_FAILED | 500 | Failed to provision secret |
| METADATA_FETCH_FAILED | 500 | Failed to retrieve metadata |
| DELETE_FAILED | 500 | Failed to delete secret |

---

## Secret Naming Convention

Secrets in GCP Secret Manager follow this naming pattern:

**Tenant-level (with category):**
```
{env}-tenant-{tenantId}-{category}-{provider}-{keyName}
```

**Vendor-level (with category):**
```
{env}-tenant-{tenantId}-vendor-{vendorId}-{category}-{provider}-{keyName}
```

**Tenant-level (without category):**
```
{env}-tenant-{tenantId}-{provider}-{keyName}
```

**Examples:**
- `prod-tenant-t_123-payment-stripe-api-key`
- `devtest-tenant-t_456-shipping-delhivery-api-token`
- `prod-tenant-t_123-vendor-v_99-payment-razorpay-key-id`
- `devtest-tenant-t_456-integration-mautic-api-key`

---

## Supported Categories

| Category | Description |
|----------|-------------|
| payment | Payment provider credentials |
| integration | Third-party integration credentials |
| api-key | API keys for external services |
| oauth | OAuth client credentials |
| database | Database connection credentials |
| webhook | Webhook secrets for signature verification |

---

## Supported Payment Providers

### Production Ready
| Provider | Key Names |
|----------|-----------|
| stripe | `api-key`, `webhook-secret` |
| razorpay | `key-id`, `key-secret`, `webhook-secret` |

### Coming Soon
| Provider | Status |
|----------|--------|
| paypal | Coming Soon |
| phonepe | Coming Soon |
| afterpay | Coming Soon |
| zippay | Coming Soon |
| googlepay | Coming Soon |
| applepay | Coming Soon |
| paytm | Coming Soon |
| upi-direct | Coming Soon |

---

## Security Considerations

1. **Never log secret values** - All logging excludes actual secret data
2. **TLS required** - All communication must use TLS
3. **Audit trail** - All operations are logged for compliance
4. **Least privilege** - Service uses separate GCP service accounts for read/write
5. **Tenant isolation** - Strict enforcement of tenant boundaries
