# Notification Service

Multi-channel notification service for Tesseract Hub with email, SMS, and push notification support. Features automatic failover, template rendering, and event-driven notifications via NATS.

## Table of Contents

- [Architecture](#architecture)
- [Features](#features)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Providers](#providers)
- [NATS Events](#nats-events)
- [Templates](#templates)
- [User Preferences](#user-preferences)

## Architecture

```
                                    ┌──────────────────────────┐
                                    │      NATS JetStream      │
                                    │  (order.>, payment.>,    │
                                    │   customer.>, auth.>)    │
                                    └───────────┬──────────────┘
                                                │
                                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         notification-service                            │
│                                                                         │
│  ┌──────────────┐   ┌─────────────────┐   ┌──────────────────────────┐ │
│  │   REST API   │   │ NATS Subscriber │   │    Email Failover        │ │
│  │  /api/v1/*   │   │                 │   │  Postal→SES→SendGrid     │ │
│  └──────────────┘   └─────────────────┘   └──────────────────────────┘ │
│          │                   │                        │                │
│          │                   │                        │                │
│          ▼                   ▼                        ▼                │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    Notification Handler                          │   │
│  │  • Check user preferences                                        │   │
│  │  • Render templates                                              │   │
│  │  • Route to providers                                            │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│          │                                                             │
│          ▼                                                             │
│  ┌───────────────┐   ┌───────────────┐   ┌───────────────┐            │
│  │ Email Provider│   │ SMS Provider  │   │ Push Provider │            │
│  │ (Failover)    │   │ (Twilio)      │   │ (FCM)         │            │
│  └───────────────┘   └───────────────┘   └───────────────┘            │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
                │
                ▼
        ┌───────────────┐
        │  PostgreSQL   │
        │  (templates,  │
        │  preferences) │
        └───────────────┘
```

## Features

- **Multi-Channel Delivery**: Email, SMS, and Push notifications
- **Email Failover Chain**: Postal (primary) → AWS SES (secondary) → SendGrid (fallback)
- **Template Engine**: Go templates with HTML and text support
- **User Preferences**: Per-user channel and category preferences
- **Event-Driven**: NATS JetStream integration for real-time notifications
- **Preference-Based Routing**: Respects user notification preferences before sending
- **Multi-Tenant**: Full tenant isolation with `tenant_id`
- **Scheduled Notifications**: Send at specific times
- **Priority Queue**: Critical, High, Normal, Low priorities
- **Retry Logic**: Automatic retries with exponential backoff
- **Delivery Tracking**: Track sent, delivered, bounced, failed status

## Quick Start

### Prerequisites

- Go 1.23+
- PostgreSQL 15+
- NATS with JetStream (optional)

### Local Development

```bash
# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=your_password
export DB_NAME=tesseract_hub
export SENDGRID_API_KEY=your_key
export SENDGRID_FROM=noreply@example.com

# Run the service
go run ./cmd
```

### Docker

```bash
docker build -t notification-service .
docker run -p 8090:8090 \
  -e DB_HOST=host.docker.internal \
  -e DB_PASSWORD=your_password \
  notification-service
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | HTTP server port | `8090` |
| `ENVIRONMENT` | Environment (development/production) | `development` |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL user | `postgres` |
| `DB_PASSWORD` | PostgreSQL password | (required) |
| `DB_NAME` | PostgreSQL database | `tesseract_hub` |
| `DB_SSL_MODE` | PostgreSQL SSL mode | `disable` |
| `NATS_URL` | NATS server URL | `nats://localhost:4222` |
| `MAX_RETRY_ATTEMPTS` | Max retry attempts | `3` |
| `WORKER_CONCURRENCY` | Worker concurrency | `10` |

### Email Provider Configuration

#### Postal HTTP API (Primary - Self-hosted)

| Variable | Description |
|----------|-------------|
| `POSTAL_API_URL` | Postal HTTP API URL (e.g., `http://postal-web.email.svc.cluster.local:5000`) |
| `POSTAL_API_KEY` | Postal server API key |
| `POSTAL_FROM` | From email address |
| `POSTAL_FROM_NAME` | From display name |

#### AWS SES (Secondary - Managed Fallback)

| Variable | Description |
|----------|-------------|
| `AWS_REGION` | AWS region (e.g., `ap-south-1`) |
| `AWS_ACCESS_KEY_ID` | IAM Access Key ID (20 chars, starts with AKIA) |
| `AWS_SECRET_ACCESS_KEY` | IAM Secret Access Key (40 chars) |
| `AWS_SES_FROM` | From email address |
| `AWS_SES_FROM_NAME` | From display name |

**Note:** Use IAM credentials, NOT SMTP credentials. See [Production Setup Guide](../../docs/notification-service-production-setup.md) for detailed setup instructions.

#### Postal SMTP (Legacy - Self-hosted)

| Variable | Description |
|----------|-------------|
| `POSTAL_HOST` | Postal SMTP host |
| `POSTAL_PORT` | Postal SMTP port (25, 465, 587) |
| `POSTAL_USERNAME` | SMTP username |
| `POSTAL_PASSWORD` | SMTP password |
| `POSTAL_FROM` | From email address |
| `POSTAL_FROM_NAME` | From display name |

#### Mautic (Secondary - Marketing Automation)

| Variable | Description |
|----------|-------------|
| `MAUTIC_URL` | Mautic API URL (e.g., https://mautic.example.com) |
| `MAUTIC_USERNAME` | Mautic API username |
| `MAUTIC_PASSWORD` | Mautic API password |
| `MAUTIC_FROM` | From email address |
| `MAUTIC_FROM_NAME` | From display name |

#### SendGrid (Fallback - Cloud)

| Variable | Description |
|----------|-------------|
| `SENDGRID_API_KEY` | SendGrid API key |
| `SENDGRID_FROM` | From email address |

#### Legacy SMTP (Fallback if no others)

| Variable | Description |
|----------|-------------|
| `SMTP_HOST` | SMTP host |
| `SMTP_PORT` | SMTP port |
| `SMTP_USERNAME` | SMTP username |
| `SMTP_PASSWORD` | SMTP password |
| `SMTP_FROM` | From email address |

#### Failover Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `EMAIL_FAILOVER_ENABLED` | Enable automatic failover | `true` |

### SMS Provider Configuration (Twilio)

| Variable | Description |
|----------|-------------|
| `TWILIO_ACCOUNT_SID` | Twilio Account SID |
| `TWILIO_AUTH_TOKEN` | Twilio Auth Token |
| `TWILIO_FROM` | Twilio phone number |

### OTP/Verification Configuration (Twilio Verify)

Twilio Verify provides secure OTP delivery for account verification, password reset, and login verification.

| Variable | Description | Default |
|----------|-------------|---------|
| `TWILIO_VERIFY_SERVICE_SID` | Verify Service SID (starts with VA) | (required) |
| `TWILIO_ACCOUNT_SID` | Twilio Account SID (starts with AC) | (required) |
| `TWILIO_AUTH_TOKEN` | Auth Token (for devtest/pilot) | (required) |
| `TWILIO_API_KEY_SID` | API Key SID (for production) | (optional) |
| `TWILIO_API_KEY_SECRET` | API Key Secret (for production) | (optional) |
| `TWILIO_AUTH_MODE` | `auth_token` or `api_key` | auto-detect |
| `TWILIO_TEST_PHONE` | Test phone number (E.164) | (optional) |
| `TWILIO_OTP_EXPIRY_MINUTES` | OTP expiry time | `10` |
| `TWILIO_OTP_LENGTH` | OTP code length | `6` |

**Authentication Modes:**

- **`auth_token`** (devtest/pilot): Uses Account SID + Auth Token
- **`api_key`** (production): Uses API Key SID + API Key Secret

If `TWILIO_AUTH_MODE` is not set, it auto-detects based on environment and available credentials.

### Push Provider Configuration (FCM)

| Variable | Description |
|----------|-------------|
| `FCM_PROJECT_ID` | Firebase project ID |
| `FCM_CREDENTIALS_JSON` | Firebase service account JSON |

## API Reference

Base URL: `/api/v1`

All endpoints require `X-Tenant-ID` header.

### Health Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/livez` | Liveness probe (Kubernetes) |
| GET | `/readyz` | Readiness probe (Kubernetes) |

### Notifications

#### Send Notification

```http
POST /api/v1/notifications/send
Content-Type: application/json
X-Tenant-ID: tenant-123

{
  "channel": "EMAIL",
  "templateName": "order-confirmation",
  "recipientEmail": "customer@example.com",
  "variables": {
    "customerName": "John Doe",
    "orderNumber": "ORD-12345",
    "totalAmount": 99.99,
    "currency": "USD"
  },
  "metadata": {
    "orderId": "uuid-here"
  },
  "priority": "HIGH"
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `channel` | string | Yes | `EMAIL`, `SMS`, or `PUSH` |
| `templateId` | string | No | Template UUID |
| `templateName` | string | No | Template name (alternative to templateId) |
| `recipientEmail` | string | For EMAIL | Recipient email address |
| `recipientPhone` | string | For SMS | Recipient phone number (E.164 format) |
| `recipientToken` | string | For PUSH | FCM device token |
| `recipientId` | string | No | User UUID for tracking |
| `subject` | string | No | Email subject (overrides template) |
| `body` | string | No | Plain text body |
| `bodyHtml` | string | No | HTML body |
| `variables` | object | No | Template variables |
| `metadata` | object | No | Additional metadata |
| `priority` | string | No | `LOW`, `NORMAL`, `HIGH`, `CRITICAL` |
| `scheduledFor` | string | No | ISO 8601 datetime for scheduling |

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "tenantId": "tenant-123",
    "channel": "EMAIL",
    "status": "PENDING",
    "recipientEmail": "customer@example.com",
    "subject": "Order Confirmed - #ORD-12345",
    "createdAt": "2025-01-15T10:30:00Z"
  }
}
```

#### List Notifications

```http
GET /api/v1/notifications?channel=EMAIL&status=SENT&limit=20&offset=0
X-Tenant-ID: tenant-123
```

**Query Parameters:**

| Parameter | Description |
|-----------|-------------|
| `channel` | Filter by channel (EMAIL, SMS, PUSH) |
| `status` | Filter by status (PENDING, QUEUED, SENDING, SENT, DELIVERED, FAILED, BOUNCED, CANCELLED) |
| `limit` | Page size (default: 50, max: 100) |
| `offset` | Page offset (default: 0) |

**Response:**

```json
{
  "success": true,
  "data": [...],
  "pagination": {
    "limit": 20,
    "offset": 0,
    "total": 150
  }
}
```

#### Get Notification

```http
GET /api/v1/notifications/:id
X-Tenant-ID: tenant-123
```

#### Get Notification Status

```http
GET /api/v1/notifications/:id/status
X-Tenant-ID: tenant-123
```

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "status": "DELIVERED",
    "provider": "SendGrid",
    "providerId": "sg-message-id",
    "sentAt": "2025-01-15T10:30:05Z",
    "deliveredAt": "2025-01-15T10:30:07Z",
    "failedAt": null,
    "errorMessage": null,
    "retryCount": 0
  }
}
```

#### Cancel Notification

```http
POST /api/v1/notifications/:id/cancel
X-Tenant-ID: tenant-123
```

Only works for `PENDING` or `QUEUED` notifications.

### Templates

#### List Templates

```http
GET /api/v1/templates?channel=EMAIL&category=orders&search=confirm
X-Tenant-ID: tenant-123
```

**Query Parameters:**

| Parameter | Description |
|-----------|-------------|
| `channel` | Filter by channel (EMAIL, SMS, PUSH) |
| `category` | Filter by category (orders, marketing, security) |
| `search` | Search in name and description |
| `limit` | Page size (default: 50) |
| `offset` | Page offset |

#### Get Template

```http
GET /api/v1/templates/:id
X-Tenant-ID: tenant-123
```

#### Create Template

```http
POST /api/v1/templates
Content-Type: application/json
X-Tenant-ID: tenant-123

{
  "name": "order-shipped",
  "description": "Email sent when order ships",
  "channel": "EMAIL",
  "category": "orders",
  "subject": "Your Order #{{.orderNumber}} Has Shipped!",
  "bodyTemplate": "Hi {{.customerName}},\n\nYour order has shipped!\n\nTracking: {{.trackingUrl}}",
  "htmlTemplate": "<h1>Your Order Has Shipped!</h1><p>Hi {{.customerName}},</p><p><a href=\"{{.trackingUrl}}\">Track your package</a></p>"
}
```

#### Update Template

```http
PUT /api/v1/templates/:id
Content-Type: application/json
X-Tenant-ID: tenant-123
```

Note: System templates cannot be modified.

#### Delete Template

```http
DELETE /api/v1/templates/:id
X-Tenant-ID: tenant-123
```

Note: System templates cannot be deleted.

#### Test Template

```http
POST /api/v1/templates/:id/test
Content-Type: application/json
X-Tenant-ID: tenant-123

{
  "variables": {
    "customerName": "Test User",
    "orderNumber": "TEST-123",
    "trackingUrl": "https://track.example.com/123"
  }
}
```

**Response:**

```json
{
  "success": true,
  "subject": "Your Order #TEST-123 Has Shipped!",
  "body": "Hi Test User,\n\nYour order has shipped!\n\nTracking: https://track.example.com/123",
  "html": "<h1>Your Order Has Shipped!</h1><p>Hi Test User,</p><p><a href=\"https://track.example.com/123\">Track your package</a></p>"
}
```

### User Preferences

#### Get Preferences

```http
GET /api/v1/preferences/:userId
X-Tenant-ID: tenant-123
```

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "userId": "user-uuid",
    "emailEnabled": true,
    "smsEnabled": true,
    "pushEnabled": true,
    "marketingEnabled": true,
    "ordersEnabled": true,
    "securityEnabled": true,
    "email": "user@example.com",
    "phone": "+1234567890"
  }
}
```

#### Update Preferences

```http
PUT /api/v1/preferences/:userId
Content-Type: application/json
X-Tenant-ID: tenant-123

{
  "emailEnabled": true,
  "smsEnabled": false,
  "pushEnabled": true,
  "marketingEnabled": false,
  "ordersEnabled": true,
  "securityEnabled": true
}
```

#### Register Push Token

```http
POST /api/v1/preferences/:userId/push-token
Content-Type: application/json
X-Tenant-ID: tenant-123

{
  "token": "fcm-device-token",
  "platform": "android"
}
```

### OTP Verification (Twilio Verify)

These endpoints are only available when Twilio Verify is configured.

#### Send OTP

```http
POST /api/v1/verify/send
Content-Type: application/json
X-Tenant-ID: tenant-123

{
  "to": "+17744896582",
  "channel": "sms",
  "locale": "en",
  "purpose": "account_verification"
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `to` | string | Yes | Phone number (E.164) or email |
| `channel` | string | No | `sms`, `email`, `call`, `whatsapp` (default: `sms`) |
| `locale` | string | No | Language locale (e.g., `en`) |
| `purpose` | string | No | Purpose for logging (e.g., `account_verification`) |

**Response:**

```json
{
  "success": true,
  "data": {
    "sid": "VE1234567890",
    "to": "+17744896582",
    "channel": "sms",
    "status": "pending"
  },
  "message": "Verification code sent successfully"
}
```

#### Check OTP

```http
POST /api/v1/verify/check
Content-Type: application/json
X-Tenant-ID: tenant-123

{
  "to": "+17744896582",
  "code": "123456"
}
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `to` | string | Yes | Phone number (E.164) or email |
| `code` | string | Yes | The OTP code to verify |

**Response (Success):**

```json
{
  "success": true,
  "valid": true,
  "data": {
    "sid": "VE1234567890",
    "to": "+17744896582",
    "status": "approved",
    "channel": "sms"
  },
  "message": "Verification successful"
}
```

**Response (Invalid Code):**

```json
{
  "success": true,
  "valid": false,
  "data": {
    "sid": "VE1234567890",
    "to": "+17744896582",
    "status": "pending"
  },
  "message": "Invalid verification code"
}
```

#### Resend OTP

```http
POST /api/v1/verify/resend
Content-Type: application/json
X-Tenant-ID: tenant-123

{
  "to": "+17744896582",
  "channel": "sms"
}
```

#### Cancel OTP

```http
POST /api/v1/verify/cancel
Content-Type: application/json
X-Tenant-ID: tenant-123

{
  "to": "+17744896582"
}
```

#### Get Auth Mode (Debug)

```http
GET /api/v1/verify/auth-mode
X-Tenant-ID: tenant-123
```

**Response:**

```json
{
  "success": true,
  "data": {
    "authMode": "auth_token",
    "testPhoneNumber": "+17744896582"
  }
}
```

### Webhooks

#### SendGrid Webhook

```http
POST /webhooks/sendgrid
```

Receives delivery status updates from SendGrid.

#### Twilio Webhook

```http
POST /webhooks/twilio
```

Receives SMS delivery status updates from Twilio.

## Providers

### Email Failover Chain

The service implements automatic email failover:

```
1. Postal (Primary)
   │
   │ ── Success ──→ Done
   │
   │ Fail (retry 1x)
   ▼
2. AWS SES (Secondary)
   │
   │ ── Success ──→ Done
   │
   │ Fail (retry 1x)
   ▼
3. SendGrid (Fallback)
   │
   │ ── Success ──→ Done
   │
   │ Fail (retry 1x)
   ▼
   Return Error
```

Each provider is tried with 1 retry before moving to the next. Set `EMAIL_FAILOVER_ENABLED=false` to disable failover.

**Important:** HIGH and CRITICAL priority notifications (like verification emails) are sent synchronously to provide immediate error feedback to callers.

### Postal HTTP API (Primary)

Self-hosted Postal server with HTTP API:

- Uses HTTP API for sending (more reliable than SMTP)
- Full template control within Postal
- Configurable from name and address
- Internal cluster access via Kubernetes service
- Postal can be configured to use AWS SES SMTP for actual delivery

### AWS SES (Secondary)

Amazon Simple Email Service integration:

- Uses AWS SDK v2 for direct API calls (not SMTP)
- Requires IAM credentials with `ses:SendEmail` and `ses:SendRawEmail` permissions
- Supports HTML and plain text emails
- High deliverability with built-in bounce/complaint handling
- Production requires moving SES out of sandbox mode

**Common Issues:**
- `SignatureDoesNotMatch`: Using SMTP credentials instead of IAM credentials
- `Email address is not verified`: SES still in sandbox mode
- `Not authorized to perform ses:SendEmail`: Missing IAM policy

See [Production Setup Guide](../../docs/notification-service-production-setup.md) for detailed AWS SES configuration.

### Postal SMTP (Legacy)

Self-hosted mail server SMTP integration:

- **Port 465**: Direct TLS connection
- **Port 587**: STARTTLS connection
- **Port 25**: Plain SMTP (not recommended)
- Proper MIME multipart for HTML + plain text
- Configurable from name and address

### Mautic

Marketing automation platform integration:

- **Contact Management**: Automatically creates/updates contacts
- **Transactional Email**: Sends via Mautic's email system
- **Newsletter Subscription**: Use `SubscribeToNewsletter()` method
- Uses Basic Auth for API authentication
- Sends through Postal SMTP under the hood

### SendGrid

Cloud email service (fallback):

- Full SendGrid API v3 integration
- CC/BCC support
- Attachment support
- Reply-To header
- Custom headers
- Delivery tracking via webhooks

### Twilio SMS

```json
{
  "channel": "SMS",
  "recipientPhone": "+1234567890",
  "body": "Your verification code is 123456"
}
```

Features:
- E.164 phone number format
- Delivery status callbacks
- Two-way messaging support

### Twilio Verify (OTP)

Secure OTP delivery for account verification, password reset, and login:

```json
POST /api/v1/verify/send
{
  "to": "+17744896582",
  "channel": "sms"
}

POST /api/v1/verify/check
{
  "to": "+17744896582",
  "code": "123456"
}
```

**Features:**
- SMS, Email, Voice Call, and WhatsApp channels
- Automatic OTP generation and validation
- Rate limiting and fraud protection (built into Twilio)
- 10-minute OTP expiry (configurable)
- 6-digit codes (configurable)

**Authentication Modes:**

| Mode | Use Case | Credentials |
|------|----------|-------------|
| `auth_token` | devtest/pilot | Account SID + Auth Token |
| `api_key` | production | API Key SID + API Key Secret |

API Key mode is recommended for production as it provides:
- Revocable credentials
- Better security isolation
- No master account access

**Status Values:**

| Status | Description |
|--------|-------------|
| `pending` | OTP sent, waiting for verification |
| `approved` | OTP verified successfully |
| `canceled` | Verification cancelled |
| `expired` | OTP expired (10 minutes) |

### Firebase Cloud Messaging (FCM)

```json
{
  "channel": "PUSH",
  "recipientToken": "fcm-device-token",
  "subject": "New Order",
  "body": "You have a new order!",
  "metadata": {
    "orderId": "uuid",
    "action": "VIEW_ORDER"
  }
}
```

Features:
- Android/iOS/Web push
- Data messages with custom payload
- Priority settings
- TTL configuration

## NATS Events

The service subscribes to NATS JetStream for event-driven notifications.

### Subscribed Subjects

| Subject | Events | Template | Channels |
|---------|--------|----------|----------|
| `order.created` | New order placed | order-confirmation | Email |
| `order.shipped` | Order shipped | order-shipped, order-shipped-sms | Email, SMS |
| `order.delivered` | Order delivered | order-delivered, order-delivered-sms | Email, SMS |
| `order.cancelled` | Order cancelled | order-cancelled | Email |
| `payment.captured` | Payment received | payment-confirmation | Email |
| `payment.failed` | Payment failed | payment-failed, payment-failed-sms | Email, SMS |
| `customer.registered` | New customer | welcome-email | Email |
| `auth.password_reset` | Password reset | password-reset | Email |
| `auth.verification` | Email verification | verification-code, verification-code-sms | Email, SMS |

### Event Processing Flow

```
1. Receive Event from NATS
         │
         ▼
2. Parse Event Data
   - tenant_id
   - user_id
   - email/phone
   - event_type
   - data (order_number, etc.)
         │
         ▼
3. Lookup User Preferences
   - Get NotificationPreference for user
         │
         ▼
4. Check Category Preference
   - orders → ordersEnabled
   - marketing → marketingEnabled
   - security → securityEnabled (always true)
         │
         ▼
5. Check Channel Preference
   - Email → emailEnabled
   - SMS → smsEnabled
   - Push → pushEnabled
         │
         ▼
6. Load System Template
   - By event type + channel
         │
         ▼
7. Render Template
   - Apply variables from event data
         │
         ▼
8. Send via Provider
   - Route to appropriate provider
```

### Example Event Payload

```json
{
  "tenant_id": "tenant-123",
  "user_id": "user-uuid",
  "email": "customer@example.com",
  "phone": "+1234567890",
  "event_type": "order.created",
  "data": {
    "order_id": "order-uuid",
    "order_number": "ORD-12345",
    "total_amount": 99.99,
    "currency": "USD",
    "customer_name": "John Doe"
  }
}
```

## Templates

### System Templates

Pre-seeded templates for common notifications:

| Name | Channel | Category | Description |
|------|---------|----------|-------------|
| `order-confirmation` | EMAIL | orders | Order placed confirmation |
| `order-shipped` | EMAIL | orders | Order shipped notification |
| `order-shipped-sms` | SMS | orders | Order shipped SMS |
| `order-delivered` | EMAIL | orders | Order delivered notification |
| `order-delivered-sms` | SMS | orders | Order delivered SMS |
| `order-cancelled` | EMAIL | orders | Order cancellation |
| `payment-confirmation` | EMAIL | orders | Payment received |
| `payment-failed` | EMAIL | orders | Payment failure |
| `payment-failed-sms` | SMS | orders | Payment failure SMS |
| `welcome-email` | EMAIL | marketing | New customer welcome |
| `password-reset` | EMAIL | security | Password reset link |
| `verification-code` | EMAIL | security | Email verification code |
| `verification-code-sms` | SMS | security | SMS verification code |

### Template Syntax

Templates use Go's `text/template` and `html/template` syntax:

```
Subject: Order Confirmed - #{{.orderNumber}}

Hi {{.customerName}},

Thank you for your order!

Order Number: {{.orderNumber}}
Total: {{.currency}} {{.totalAmount}}

{{if .trackingUrl}}Track your order: {{.trackingUrl}}{{end}}

Thank you for shopping with us!
```

### Common Template Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `customerName` | Customer's full name | "John Doe" |
| `orderNumber` | Order reference number | "ORD-12345" |
| `totalAmount` | Order total (number) | 99.99 |
| `currency` | Currency code | "USD" |
| `trackingUrl` | Shipment tracking URL | "https://track.me/123" |
| `carrierName` | Shipping carrier | "FedEx" |
| `verificationCode` | OTP or verification code | "123456" |
| `resetUrl` | Password reset URL | "https://app.com/reset/token" |

### Template Functions

Available functions in templates:

```
{{.Amount | currency}}      → Format as currency
{{.Date | formatDate}}      → Format date
{{.Name | upper}}           → UPPERCASE
{{.Name | lower}}           → lowercase
{{if .Condition}}...{{end}} → Conditional
{{range .Items}}...{{end}}  → Loop/iteration
```

## User Preferences

### Channel Preferences

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `emailEnabled` | bool | true | Receive email notifications |
| `smsEnabled` | bool | true | Receive SMS notifications |
| `pushEnabled` | bool | true | Receive push notifications |

### Category Preferences

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ordersEnabled` | bool | true | Order-related notifications |
| `marketingEnabled` | bool | true | Marketing and promotional |
| `securityEnabled` | bool | true | Security alerts (always sent if possible) |

### Preference Priority

```
For each notification:

1. Check if CHANNEL is enabled
   - emailEnabled / smsEnabled / pushEnabled
   - If disabled → Skip this channel

2. Check if CATEGORY is enabled
   - ordersEnabled / marketingEnabled / securityEnabled
   - Security category is ALWAYS considered enabled
   - If disabled → Skip notification for this category

3. If both checks pass → Send notification
```

## Database Schema

### Tables

| Table | Description |
|-------|-------------|
| `notifications` | Individual notification records |
| `notification_templates` | Reusable templates |
| `notification_preferences` | User channel/category preferences |
| `notification_logs` | Audit trail for notifications |
| `notification_batches` | Batch notification campaigns |

### Notification Status Flow

```
PENDING → QUEUED → SENDING → SENT → DELIVERED
                      │
                      └──→ FAILED → (retry) → SENT
                              │
                              └──→ BOUNCED

PENDING/QUEUED → CANCELLED (via cancel API)
```

## Monitoring

### Logging Prefixes

The service logs with prefixes for easy filtering:

| Prefix | Description |
|--------|-------------|
| `[POSTAL]` | Postal SMTP operations |
| `[MAUTIC]` | Mautic API operations |
| `[SENDGRID]` | SendGrid API operations |
| `[TWILIO]` | Twilio SMS operations |
| `[TWILIO-VERIFY]` | Twilio Verify OTP operations |
| `[VERIFY-HANDLER]` | OTP verification handler |
| `[FCM]` | Firebase push operations |
| `[NATS]` | NATS event processing |
| `[FAILOVER]` | Failover chain operations |
| `[EMAIL]` | Email sending (preference check) |
| `[SMS]` | SMS sending (preference check) |
| `[PUSH]` | Push sending (preference check) |

### Health Checks

| Endpoint | Description |
|----------|-------------|
| `/livez` | Returns 200 if service is alive |
| `/readyz` | Returns 200 if database is connected |
| `/health` | Full health check with provider status |

## Kubernetes Deployment

See the Helm chart in `tesserix-k8s/charts/apps/notification-service/`.

### Enable Providers in values.yaml

```yaml
# Postal SMTP (primary)
postal:
  enabled: true
  existingSecret: "postal-credentials"

# Mautic (secondary)
mautic:
  enabled: true
  existingSecret: "mautic-credentials"

# SendGrid (fallback)
sendgrid:
  enabled: true
  existingSecret: "sendgrid-credentials"

# Twilio SMS
twilio:
  enabled: true
  existingSecret: "twilio-credentials"

# Firebase Push
fcm:
  enabled: true
  existingSecret: "fcm-credentials"

# Twilio Verify (OTP)
twilioVerify:
  enabled: true
  existingSecret: "twilio-verify-credentials"
  authMode: ""  # auto-detect, or "auth_token" / "api_key"
  testPhoneNumber: "+17744896582"  # For devtest
```

### Create Sealed Secrets

```bash
# Postal credentials
kubectl create secret generic postal-credentials \
  --from-literal=POSTAL_HOST=smtp.postal.example.com \
  --from-literal=POSTAL_PORT=587 \
  --from-literal=POSTAL_USERNAME=user \
  --from-literal=POSTAL_PASSWORD=pass \
  --from-literal=POSTAL_FROM=noreply@example.com \
  --from-literal=POSTAL_FROM_NAME="Tesseract Hub" \
  --dry-run=client -o yaml | kubeseal > postal-sealed-secret.yaml

# Mautic credentials
kubectl create secret generic mautic-credentials \
  --from-literal=MAUTIC_URL=https://mautic.example.com \
  --from-literal=MAUTIC_USERNAME=api-user \
  --from-literal=MAUTIC_PASSWORD=api-pass \
  --from-literal=MAUTIC_FROM=noreply@example.com \
  --from-literal=MAUTIC_FROM_NAME="Tesseract Hub" \
  --dry-run=client -o yaml | kubeseal > mautic-sealed-secret.yaml

# Twilio Verify credentials (devtest with auth_token)
kubectl create secret generic twilio-verify-credentials \
  --from-literal=TWILIO_VERIFY_SERVICE_SID=VAxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --from-literal=TWILIO_ACCOUNT_SID=ACxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --from-literal=TWILIO_AUTH_TOKEN=your_auth_token \
  --namespace=devtest \
  --dry-run=client -o yaml | kubeseal > twilio-verify-sealed-secret.yaml

# Twilio Verify credentials (production with api_key)
kubectl create secret generic twilio-verify-credentials \
  --from-literal=TWILIO_VERIFY_SERVICE_SID=VAxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --from-literal=TWILIO_ACCOUNT_SID=ACxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --from-literal=TWILIO_AUTH_TOKEN="" \
  --from-literal=TWILIO_API_KEY_SID=SKxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --from-literal=TWILIO_API_KEY_SECRET=your_api_key_secret \
  --namespace=production \
  --dry-run=client -o yaml | kubeseal > twilio-verify-sealed-secret.yaml
```

## Best Practices

1. **Use Templates**: Create reusable templates instead of hardcoding messages
2. **Respect Preferences**: Always check user notification preferences before sending
3. **Handle Failures**: Service implements automatic retry and failover
4. **Rate Limiting**: Implement frequency caps in your application
5. **Test Templates**: Use the `/test` endpoint before deploying new templates
6. **Monitor Metrics**: Track delivery rates using the logs
7. **Secure Credentials**: Use Kubernetes secrets for API keys
8. **Use NATS for Events**: Prefer event-driven notifications over direct API calls

## Production Setup

For production deployment, see the comprehensive [Production Setup Guide](../../docs/notification-service-production-setup.md) which covers:

- AWS SES IAM policy and permissions
- Moving SES out of sandbox mode
- GCP Secret Manager integration
- Kubernetes deployment configuration
- SendGrid fallback setup
- Production checklist
- Troubleshooting common issues

## License

Proprietary - Tesseract Nexus
