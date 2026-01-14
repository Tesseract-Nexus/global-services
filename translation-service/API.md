# Translation Service API Documentation

> **Private API** - Internal use only. All endpoints require proper authentication headers.

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Base URL](#base-url)
- [Rate Limiting](#rate-limiting)
- [API Endpoints](#api-endpoints)
  - [Translation](#translation)
  - [Language Detection](#language-detection)
  - [Languages](#languages)
  - [Tenant Preferences](#tenant-preferences)
  - [User Preferences](#user-preferences)
  - [Statistics](#statistics)
  - [Cache Management](#cache-management)
  - [Health Checks](#health-checks)
- [Error Handling](#error-handling)
- [Data Models](#data-models)
- [Configuration](#configuration)
- [Deployment](#deployment)

---

## Overview

The Translation Service provides multi-language translation capabilities for the Tesseract platform. It supports:

- Single and batch text translations
- Automatic language detection
- Multi-tenant isolation
- Per-user language preferences
- Two-tier caching (Redis + PostgreSQL)
- 30+ supported languages

### Technology Stack

| Component | Technology |
|-----------|------------|
| Framework | Go (Gin) |
| Database | PostgreSQL |
| Cache | Redis |
| Translation Engine | LibreTranslate |
| Observability | OpenTelemetry, Prometheus |

---

## Authentication

This is a **private API** that uses header-based tenant and user identification.

### Required Headers

| Header | Required | Description |
|--------|----------|-------------|
| `X-Tenant-ID` | Conditional | Tenant identifier. Required for preference, stats, and cache endpoints |
| `X-User-ID` | Conditional | User UUID. Required for user preference endpoints |
| `X-Request-ID` | Optional | Request correlation ID (auto-generated if not provided) |

### Header Requirements by Endpoint

| Endpoint | X-Tenant-ID | X-User-ID |
|----------|-------------|-----------|
| `POST /translate` | Optional | No |
| `POST /translate/batch` | Optional | No |
| `POST /detect` | Optional | No |
| `GET /languages` | No | No |
| `GET /preferences` | **Required** | No |
| `PUT /preferences` | **Required** | No |
| `GET /users/me/language` | **Required** | **Required** |
| `PUT /users/me/language` | **Required** | **Required** |
| `DELETE /users/me/language` | **Required** | **Required** |
| `GET /stats` | **Required** | No |
| `DELETE /cache` | **Required** | No |

---

## Base URL

```
/api/v1
```

Health endpoints are at the root level (`/health`, `/livez`, `/readyz`).

---

## Rate Limiting

Translation endpoints are rate-limited per tenant/IP.

| Setting | Default |
|---------|---------|
| Requests per window | 100 |
| Window duration | 1 minute |

### Rate Limit Headers

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1703980800
```

### Rate Limit Exceeded Response

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json

{
  "error": "rate limit exceeded",
  "retry_after": 45
}
```

---

## API Endpoints

### Translation

#### Single Translation

Translate a single text string.

```http
POST /api/v1/translate
```

**Request Body**

```json
{
  "text": "Hello, how are you?",
  "source_lang": "en",
  "target_lang": "hi",
  "context": "greeting"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `text` | string | Yes | Text to translate |
| `source_lang` | string | No | Source language code (auto-detected if omitted) |
| `target_lang` | string | Yes | Target language code |
| `context` | string | No | Context hint for better translation (e.g., "product_name", "greeting") |

**Response**

```json
{
  "original_text": "Hello, how are you?",
  "translated_text": "नमस्ते, आप कैसे हैं?",
  "source_lang": "en",
  "target_lang": "hi",
  "cached": false,
  "provider": "libretranslate"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `original_text` | string | Original input text |
| `translated_text` | string | Translated text |
| `source_lang` | string | Detected or provided source language |
| `target_lang` | string | Target language |
| `cached` | boolean | Whether result was served from cache |
| `provider` | string | Translation provider used |

**Example**

```bash
curl -X POST http://localhost:8080/api/v1/translate \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant-123" \
  -d '{
    "text": "Hello, how are you?",
    "target_lang": "hi"
  }'
```

---

#### Batch Translation

Translate multiple texts in a single request.

```http
POST /api/v1/translate/batch
```

**Request Body**

```json
{
  "items": [
    {
      "id": "item-1",
      "text": "Hello",
      "context": "greeting"
    },
    {
      "id": "item-2",
      "text": "Goodbye",
      "context": "farewell"
    },
    {
      "id": "item-3",
      "text": "Thank you",
      "source_lang": "en"
    }
  ],
  "source_lang": "en",
  "target_lang": "hi"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `items` | array | Yes | Array of translation items (max 50) |
| `items[].id` | string | No | Client-provided ID for matching responses |
| `items[].text` | string | Yes | Text to translate |
| `items[].source_lang` | string | No | Override batch-level source_lang |
| `items[].context` | string | No | Context hint |
| `source_lang` | string | No | Default source language for all items |
| `target_lang` | string | Yes | Target language |

**Response**

```json
{
  "items": [
    {
      "id": "item-1",
      "original_text": "Hello",
      "translated_text": "नमस्ते",
      "source_lang": "en",
      "cached": true
    },
    {
      "id": "item-2",
      "original_text": "Goodbye",
      "translated_text": "अलविदा",
      "source_lang": "en",
      "cached": false
    },
    {
      "id": "item-3",
      "original_text": "Thank you",
      "translated_text": "धन्यवाद",
      "source_lang": "en",
      "cached": false
    }
  ],
  "total_count": 3,
  "cached_count": 1,
  "target_lang": "hi"
}
```

**Example**

```bash
curl -X POST http://localhost:8080/api/v1/translate/batch \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant-123" \
  -d '{
    "items": [
      {"id": "1", "text": "Hello"},
      {"id": "2", "text": "World"}
    ],
    "target_lang": "hi"
  }'
```

---

### Language Detection

Detect the language of a text.

```http
POST /api/v1/detect
```

**Request Body**

```json
{
  "text": "Bonjour, comment allez-vous?"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `text` | string | Yes | Text to analyze |

**Response**

```json
{
  "language": "fr",
  "confidence": 0.95
}
```

| Field | Type | Description |
|-------|------|-------------|
| `language` | string | ISO 639-1 language code |
| `confidence` | float | Confidence score (0.0 - 1.0) |

**Example**

```bash
curl -X POST http://localhost:8080/api/v1/detect \
  -H "Content-Type: application/json" \
  -d '{"text": "Bonjour, comment allez-vous?"}'
```

---

### Languages

Get list of supported languages.

```http
GET /api/v1/languages
```

**Response**

```json
{
  "languages": [
    {
      "code": "en",
      "name": "English",
      "native_name": "English",
      "rtl": false,
      "is_active": true,
      "region": "Global"
    },
    {
      "code": "hi",
      "name": "Hindi",
      "native_name": "हिन्दी",
      "rtl": false,
      "is_active": true,
      "region": "India"
    },
    {
      "code": "ar",
      "name": "Arabic",
      "native_name": "العربية",
      "rtl": true,
      "is_active": true,
      "region": "Middle East"
    }
  ],
  "count": 30
}
```

**Supported Languages by Region**

| Region | Languages |
|--------|-----------|
| **India** | Hindi (hi), Tamil (ta), Telugu (te), Marathi (mr), Bengali (bn), Gujarati (gu), Kannada (kn), Malayalam (ml), Punjabi (pa), Odia (or) |
| **Global** | English (en), Spanish (es), French (fr), German (de), Portuguese (pt), Italian (it), Dutch (nl), Russian (ru) |
| **Asia** | Chinese (zh), Japanese (ja), Korean (ko) |
| **Southeast Asia** | Thai (th), Vietnamese (vi), Indonesian (id), Malay (ms), Filipino (fil) |
| **Middle East** | Arabic (ar), Persian (fa), Hebrew (he), Turkish (tr) |

**Example**

```bash
curl http://localhost:8080/api/v1/languages
```

---

### Tenant Preferences

#### Get Tenant Preferences

```http
GET /api/v1/preferences
```

**Headers Required:** `X-Tenant-ID`

**Response**

```json
{
  "tenant_id": "tenant-123",
  "default_source_lang": "en",
  "default_target_lang": "hi",
  "enabled_languages": ["en", "hi", "ta", "te", "mr"],
  "auto_detect": true
}
```

**Example**

```bash
curl http://localhost:8080/api/v1/preferences \
  -H "X-Tenant-ID: tenant-123"
```

---

#### Update Tenant Preferences

```http
PUT /api/v1/preferences
```

**Headers Required:** `X-Tenant-ID`

**Request Body**

```json
{
  "default_source_lang": "en",
  "default_target_lang": "ta",
  "enabled_languages": ["en", "hi", "ta", "te"],
  "auto_detect": true
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `default_source_lang` | string | No | Default source language |
| `default_target_lang` | string | No | Default target language |
| `enabled_languages` | array | No | List of enabled language codes |
| `auto_detect` | boolean | No | Enable auto-detection |

**Response**

```json
{
  "message": "Preferences updated successfully"
}
```

**Example**

```bash
curl -X PUT http://localhost:8080/api/v1/preferences \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant-123" \
  -d '{
    "default_target_lang": "ta",
    "enabled_languages": ["en", "hi", "ta"]
  }'
```

---

### User Preferences

#### Get User Language Preference

```http
GET /api/v1/users/me/language
```

**Headers Required:** `X-Tenant-ID`, `X-User-ID`

**Response**

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "tenant_id": "tenant-123",
    "user_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "preferred_language": "hi",
    "source_language": "en",
    "auto_detect_source": true,
    "rtl_enabled": false,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
}
```

If no preference exists, returns default values:

```json
{
  "success": true,
  "data": {
    "preferred_language": "en",
    "source_language": "en",
    "auto_detect_source": true,
    "rtl_enabled": false
  },
  "message": "No preference found, returning default"
}
```

**Example**

```bash
curl http://localhost:8080/api/v1/users/me/language \
  -H "X-Tenant-ID: tenant-123" \
  -H "X-User-ID: 7c9e6679-7425-40de-944b-e07fc1f90ae7"
```

---

#### Set User Language Preference

```http
PUT /api/v1/users/me/language
```

**Headers Required:** `X-Tenant-ID`, `X-User-ID`

**Request Body**

```json
{
  "preferred_language": "hi",
  "source_language": "en",
  "auto_detect_source": true,
  "rtl_enabled": false
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `preferred_language` | string | Yes | Preferred language code (2-10 chars) |
| `source_language` | string | No | Default source language |
| `auto_detect_source` | boolean | No | Auto-detect source language |
| `rtl_enabled` | boolean | No | Enable RTL (auto-set for RTL languages) |

**Response**

```json
{
  "success": true,
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "tenant_id": "tenant-123",
    "user_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "preferred_language": "hi",
    "source_language": "en",
    "auto_detect_source": true,
    "rtl_enabled": false,
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  },
  "message": "Language preference saved successfully"
}
```

**Example**

```bash
curl -X PUT http://localhost:8080/api/v1/users/me/language \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: tenant-123" \
  -H "X-User-ID: 7c9e6679-7425-40de-944b-e07fc1f90ae7" \
  -d '{
    "preferred_language": "hi",
    "auto_detect_source": true
  }'
```

---

#### Reset User Language Preference

```http
DELETE /api/v1/users/me/language
```

**Headers Required:** `X-Tenant-ID`, `X-User-ID`

**Response**

```json
{
  "success": true,
  "message": "Language preference reset to default (English)"
}
```

**Example**

```bash
curl -X DELETE http://localhost:8080/api/v1/users/me/language \
  -H "X-Tenant-ID: tenant-123" \
  -H "X-User-ID: 7c9e6679-7425-40de-944b-e07fc1f90ae7"
```

---

### Statistics

Get translation statistics for a tenant.

```http
GET /api/v1/stats
```

**Headers Required:** `X-Tenant-ID`

**Response**

```json
{
  "tenant_id": "tenant-123",
  "total_requests": 15420,
  "cache_hits": 12850,
  "cache_misses": 2570,
  "cache_hit_rate": 83.35,
  "total_characters": 2456780,
  "last_request_at": "2024-01-15T14:30:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `tenant_id` | string | Tenant identifier |
| `total_requests` | integer | Total translation requests |
| `cache_hits` | integer | Requests served from cache |
| `cache_misses` | integer | Requests requiring translation |
| `cache_hit_rate` | float | Cache hit percentage (0-100) |
| `total_characters` | integer | Total characters translated |
| `last_request_at` | string | Timestamp of last request |

**Example**

```bash
curl http://localhost:8080/api/v1/stats \
  -H "X-Tenant-ID: tenant-123"
```

---

### Cache Management

Invalidate all cached translations for a tenant.

```http
DELETE /api/v1/cache
```

**Headers Required:** `X-Tenant-ID`

**Response**

```json
{
  "message": "Cache invalidated successfully"
}
```

**Example**

```bash
curl -X DELETE http://localhost:8080/api/v1/cache \
  -H "X-Tenant-ID: tenant-123"
```

---

### Health Checks

#### Health Check

Comprehensive health status including dependencies.

```http
GET /health
```

**Response (Healthy)**

```json
{
  "status": "healthy",
  "checks": {
    "libretranslate": "healthy",
    "redis": "healthy"
  }
}
```

**Response (Degraded)**

```json
{
  "status": "degraded",
  "checks": {
    "libretranslate": "healthy",
    "redis": "unhealthy: connection refused"
  }
}
```

---

#### Liveness Probe

Kubernetes liveness probe endpoint.

```http
GET /livez
```

**Response**

```json
{
  "status": "alive"
}
```

---

#### Readiness Probe

Kubernetes readiness probe endpoint.

```http
GET /readyz
```

**Response (Ready)**

```json
{
  "status": "ready"
}
```

**Response (Not Ready)**

```http
HTTP/1.1 503 Service Unavailable

{
  "status": "not ready",
  "error": "LibreTranslate service unavailable"
}
```

---

#### Prometheus Metrics

```http
GET /metrics
```

Returns Prometheus-format metrics (text/plain).

---

## Error Handling

### Error Response Format

All errors follow a consistent format:

```json
{
  "error": "error message description"
}
```

### HTTP Status Codes

| Status | Description |
|--------|-------------|
| `200` | Success |
| `400` | Bad Request - Invalid input or missing required fields |
| `401` | Unauthorized - Missing required authentication header |
| `429` | Too Many Requests - Rate limit exceeded |
| `500` | Internal Server Error - Server-side error |
| `503` | Service Unavailable - Dependency unavailable |

### Common Errors

**Missing Tenant ID**
```json
{
  "error": "X-Tenant-ID header is required"
}
```

**Missing User ID**
```json
{
  "error": "X-User-ID header is required"
}
```

**Invalid Request Body**
```json
{
  "error": "invalid request body"
}
```

**Missing Target Language**
```json
{
  "error": "target_lang is required"
}
```

**Batch Size Exceeded**
```json
{
  "error": "batch size exceeds maximum of 50 items"
}
```

**Translation Failed**
```json
{
  "error": "translation failed: <reason>"
}
```

---

## Data Models

### TranslationRequest

```typescript
interface TranslationRequest {
  text: string;         // Required
  source_lang?: string; // Optional, auto-detected
  target_lang: string;  // Required
  context?: string;     // Optional
}
```

### TranslationResponse

```typescript
interface TranslationResponse {
  original_text: string;
  translated_text: string;
  source_lang: string;
  target_lang: string;
  cached: boolean;
  provider: string;
}
```

### BatchTranslationRequest

```typescript
interface BatchTranslationRequest {
  items: TranslationItem[];  // Max 50 items
  source_lang?: string;
  target_lang: string;       // Required
}

interface TranslationItem {
  id?: string;
  text: string;              // Required
  source_lang?: string;
  context?: string;
}
```

### UserLanguagePreference

```typescript
interface UserLanguagePreference {
  id: string;                   // UUID
  tenant_id: string;
  user_id: string;              // UUID
  preferred_language: string;   // Default: "en"
  source_language: string;      // Default: "en"
  auto_detect_source: boolean;  // Default: true
  rtl_enabled: boolean;         // Default: false
  created_at: string;           // ISO 8601
  updated_at: string;           // ISO 8601
}
```

### Language

```typescript
interface Language {
  code: string;         // ISO 639-1 code
  name: string;         // English name
  native_name: string;  // Native script name
  rtl: boolean;         // Right-to-left support
  is_active: boolean;
  region: string;       // Geographic region
}
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `0.0.0.0` | Server bind address |
| `SERVER_PORT` | `8080` | Server port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | `postgres` | Database password |
| `DB_NAME` | `translation_db` | Database name |
| `DB_SSLMODE` | `disable` | SSL mode |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | `` | Redis password |
| `REDIS_DB` | `0` | Redis database |
| `APP_NAME` | `translation-service` | Application name |
| `APP_ENV` | `development` | Environment (development/production) |
| `LOG_LEVEL` | `info` | Log level |
| `LIBRETRANSLATE_URL` | `http://libretranslate:5000` | LibreTranslate URL |
| `LIBRETRANSLATE_API_KEY` | `` | LibreTranslate API key |
| `CACHE_ENABLED` | `true` | Enable caching |
| `CACHE_TTL` | `24h` | Cache TTL |
| `RATE_LIMIT` | `100` | Requests per window |
| `RATE_LIMIT_WINDOW` | `1m` | Rate limit window |
| `MAX_BATCH_SIZE` | `50` | Maximum batch size |
| `BATCH_TIMEOUT` | `30s` | Batch timeout |
| `DEFAULT_SOURCE_LANG` | `en` | Default source language |
| `DEFAULT_TARGET_LANG` | `hi` | Default target language |

---

## Deployment

### Docker

```bash
# Build
docker build -t translation-service .

# Run
docker run -p 8080:8080 \
  -e DB_HOST=postgres \
  -e REDIS_HOST=redis \
  -e LIBRETRANSLATE_URL=http://libretranslate:5000 \
  translation-service
```

### Docker Compose

```bash
cd services/translation-service
docker-compose up -d
```

This starts:
- Translation Service (port 8089)
- PostgreSQL (port 5450)
- Redis (port 6389)
- LibreTranslate (port 5001)

### Kubernetes

Use the health endpoints for probes:

```yaml
livenessProbe:
  httpGet:
    path: /livez
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

---

## Caching Strategy

The service implements a two-tier caching strategy:

1. **Redis (Tier 1)**: Fast in-memory cache
   - Key: `trans:{tenantID}:{sourceLang}:{targetLang}:{context}:{hash}`
   - TTL: Configurable (default 24h)

2. **PostgreSQL (Tier 2)**: Persistent cache
   - Table: `translation_caches`
   - TTL: Via `expires_at` column
   - Cleanup: Hourly background job

### Cache Lookup Flow

```
Request → Redis Cache → PostgreSQL Cache → LibreTranslate API
              ↓               ↓                    ↓
          (if hit)       (if hit)            (translate)
              ↓               ↓                    ↓
          Return         Return +              Cache in
                       update Redis         Redis + PostgreSQL
```

---

## Best Practices

1. **Always provide `X-Tenant-ID`** for proper isolation and statistics tracking
2. **Use batch translation** for multiple texts to reduce latency
3. **Leverage caching** by providing consistent `context` values
4. **Handle rate limits** with exponential backoff
5. **Monitor `/stats` endpoint** to track cache efficiency
6. **Use language detection** sparingly - prefer explicit `source_lang` when known
