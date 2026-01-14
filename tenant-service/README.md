# Tenant Service (Common)

A comprehensive tenant onboarding service built with Go, Gin, and GORM following clean architecture principles. This service manages the complete tenant registration and setup process for any business application in the Tesseract Hub ecosystem.

## Features

- **Multi-Application Support**: E-commerce, SaaS, Marketplace, B2B platforms
- **Configurable Onboarding Flows**: Customizable steps per application type
- **Business Registration**: Complete business information capture and validation
- **Email & Phone Verification**: Secure verification process with OTP codes
- **Payment Integration**: Multiple payment providers and subscription models
- **Application-Specific Setup**: Configurable setup steps per business domain
- **Task Management**: Progress tracking with dependencies and status management
- **Multi-tenant Support**: Isolated tenant onboarding flows
- **Template System**: Predefined templates for different business types
- **RESTful API**: Complete HTTP API with OpenAPI/Swagger documentation
- **Integration Ready**: Designed to work with any business application

## Architecture

The service follows clean architecture principles with clear separation of concerns:

```
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── config/                 # Configuration management
│   ├── handlers/               # HTTP handlers/controllers
│   ├── middleware/             # HTTP middleware
│   ├── models/                 # Domain models/entities
│   ├── repository/             # Data access layer
│   ├── services/               # Business logic layer
│   ├── templates/              # Onboarding templates
│   └── utils/                  # Utility functions
├── migrations/                 # Database migrations
├── templates/                  # Flow templates (JSON)
├── Dockerfile                  # Docker configuration
├── docker-compose.yml          # Local development setup
└── README.md                   # This file
```

## Supported Application Types

### E-commerce
- Store setup and configuration
- Product catalog initialization
- Payment gateway setup
- Shipping configuration
- Theme selection

### SaaS Platform
- Workspace creation
- User role configuration
- API key generation
- Integration setup
- Feature toggles

### B2B Marketplace
- Vendor onboarding
- Product categories setup
- Commission structure
- Compliance verification
- Multi-vendor configuration

### Service Platform
- Service catalog setup
- Booking system configuration
- Provider verification
- Pricing models
- Availability management

## API Endpoints

All API endpoints are prefixed with `/api/v1`.

### Onboarding Sessions
- `POST /api/v1/onboarding/sessions` - Start new onboarding session
- `GET /api/v1/onboarding/sessions/:sessionId` - Get session details
- `GET /api/v1/onboarding/sessions/:sessionId/events` - SSE endpoint for real-time events
- `POST /api/v1/onboarding/sessions/:sessionId/complete` - Complete onboarding
- `POST /api/v1/onboarding/sessions/:sessionId/account-setup` - Create tenant and user account
- `GET /api/v1/onboarding/sessions/:sessionId/progress` - Get progress percentage
- `GET /api/v1/onboarding/sessions/:sessionId/tasks` - Get all tasks
- `PUT /api/v1/onboarding/sessions/:sessionId/tasks/:taskId` - Update task status

### Business Information
- `POST /api/v1/onboarding/sessions/:sessionId/business-information` - Create business info
- `PUT /api/v1/onboarding/sessions/:sessionId/business-information` - Update business info
- `POST /api/v1/onboarding/sessions/:sessionId/contact-information` - Save contact info
- `POST /api/v1/onboarding/sessions/:sessionId/business-addresses` - Save business & billing addresses

### Verification
- `POST /api/v1/onboarding/sessions/:sessionId/verification/email` - Start email verification
- `POST /api/v1/onboarding/sessions/:sessionId/verification/phone` - Start phone verification
- `POST /api/v1/onboarding/sessions/:sessionId/verification/verify` - Verify OTP code
- `POST /api/v1/onboarding/sessions/:sessionId/verification/resend` - Resend verification code
- `GET /api/v1/onboarding/sessions/:sessionId/verification/status` - Get verification status
- `GET /api/v1/onboarding/sessions/:sessionId/verification/:type/check` - Check specific verification

### Link-Based Verification
- `GET /api/v1/verify/method` - Get verification method (otp/link)
- `POST /api/v1/verify/token` - Verify email by token
- `GET /api/v1/verify/token-info` - Get token information

### Validation
- `GET /api/v1/onboarding/validate/subdomain?subdomain=xxx&session_id=xxx` - Validate/reserve subdomain
- `GET /api/v1/onboarding/validate/business-name?business_name=xxx` - Check business name
- `GET /api/v1/onboarding/validate/slug?slug=xxx` - Check slug availability
- `GET /api/v1/onboarding/validate/slug/generate?name=xxx` - Generate slug from name

### Template Management
- `GET /api/v1/onboarding/templates` - List all templates
- `POST /api/v1/onboarding/templates` - Create template
- `GET /api/v1/onboarding/templates/:templateId` - Get template by ID
- `PUT /api/v1/onboarding/templates/:templateId` - Update template
- `DELETE /api/v1/onboarding/templates/:templateId` - Delete template
- `POST /api/v1/onboarding/templates/:templateId/set-default` - Set as default
- `GET /api/v1/onboarding/templates/by-type/:applicationType` - Get by app type
- `GET /api/v1/onboarding/templates/default/:applicationType` - Get default template
- `GET /api/v1/onboarding/templates/active` - Get active templates
- `POST /api/v1/onboarding/templates/validate-config` - Validate template config

### Tenant Management
- `POST /api/v1/tenants/create-for-user` - Create tenant for existing user
- `GET /api/v1/tenants/check-slug` - Check slug availability
- `GET /api/v1/tenants/:slug/context` - Get tenant context by slug
- `GET /api/v1/tenants/:slug/access` - Verify user access to tenant
- `POST /api/v1/tenants/:tenantId/members/invite` - Invite member
- `DELETE /api/v1/tenants/:tenantId/members/:memberId` - Remove member
- `PUT /api/v1/tenants/:tenantId/members/:memberId/role` - Update member role

### User Tenants
- `GET /api/v1/users/me/tenants` - Get user's tenants
- `GET /api/v1/users/me/tenants/default` - Get user's default tenant
- `PUT /api/v1/users/me/tenants/default` - Set user's default tenant

### Invitations
- `POST /api/v1/invitations/accept` - Accept invitation

### Draft Management
- `POST /api/v1/onboarding/draft/save` - Save draft
- `GET /api/v1/onboarding/draft/:sessionId` - Get draft
- `DELETE /api/v1/onboarding/draft/:sessionId` - Delete draft
- `POST /api/v1/onboarding/draft/heartbeat` - Process heartbeat
- `POST /api/v1/onboarding/draft/browser-close` - Mark browser closed

### Health & Monitoring
- `GET /health` - Health check
- `GET /ready` - Readiness check
- `GET /metrics` - Prometheus metrics

## Environment Variables

```bash
# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8086

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=tesseract_hub
DB_SSL_MODE=disable

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# Application Configuration
APP_ENV=development
LOG_LEVEL=info
JWT_SECRET=your-secret-key

# Email Configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-password
FROM_EMAIL=noreply@tesseract-hub.com
FROM_NAME=Tesseract Hub

# SMS Configuration (Twilio)
SMS_PROVIDER=twilio
TWILIO_ACCOUNT_SID=your-account-sid
TWILIO_AUTH_TOKEN=your-auth-token
TWILIO_PHONE_NUMBER=+1234567890

# Payment Configuration
STRIPE_SECRET_KEY=sk_test_...
STRIPE_PUBLISHABLE_KEY=pk_test_...
PAYPAL_CLIENT_ID=your-client-id
PAYPAL_CLIENT_SECRET=your-client-secret
RAZORPAY_KEY_ID=your-key-id
RAZORPAY_KEY_SECRET=your-key-secret

# Integration Services
SETTINGS_SERVICE_URL=http://localhost:8104
AUTH_SERVICE_URL=http://localhost:8080
NOTIFICATION_SERVICE_URL=http://localhost:8087

# Draft Management
DRAFT_EXPIRY_HOURS=168              # 7 days
DRAFT_REMINDER_INTERVAL_HOURS=24
DRAFT_MAX_REMINDERS=7
DRAFT_CLEANUP_INTERVAL_MINS=60

# Verification Configuration
VERIFICATION_METHOD=link            # "otp" or "link"
VERIFICATION_TOKEN_EXPIRY_HOURS=24
ONBOARDING_APP_URL=http://localhost:3000

# URL Configuration (for tenant subdomains)
BASE_DOMAIN=tesserix.app            # Pattern: {slug}-admin.tesserix.app
```

## Key API Examples

### Start Onboarding Session
```bash
POST /api/v1/onboarding/sessions
Content-Type: application/json

{
  "application_type": "ecommerce",
  "template_id": "uuid-of-template"  # Optional - uses default if not provided
}
```

### Complete Account Setup
Creates the tenant and user account after all onboarding steps are complete.

```bash
POST /api/v1/onboarding/sessions/:sessionId/account-setup
Content-Type: application/json

{
  "password": "SecurePassword123!",
  "auth_method": "password",          # Required: "password" or "social"
  "timezone": "America/New_York",     # Optional, defaults to "UTC"
  "currency": "USD",                  # Optional, defaults to "USD"
  "business_model": "ONLINE_STORE"    # Optional: "ONLINE_STORE" or "MARKETPLACE"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Account created successfully",
  "data": {
    "tenant_id": "uuid",
    "user_id": "uuid",
    "admin_url": "https://mystore-admin.tesserix.app",
    "storefront_url": "https://mystore-store.tesserix.app",
    "access_token": "jwt-token",
    "refresh_token": "refresh-token"
  }
}
```

### Update Business Address with Billing
```bash
POST /api/v1/onboarding/sessions/:sessionId/business-addresses
Content-Type: application/json

{
  "street_address": "123 Main St",
  "city": "New York",
  "state_province": "NY",
  "postal_code": "10001",
  "country": "US",
  "billing_same_as_business": false,
  "billing_street_address": "456 Billing Ave",
  "billing_city": "New York",
  "billing_state": "NY",
  "billing_postal_code": "10002",
  "billing_country": "US"
}
```

### Validate Subdomain with Reservation
```bash
GET /api/v1/onboarding/validate/subdomain?subdomain=mystore&session_id=uuid

# Response includes suggestions if taken:
{
  "available": false,
  "message": "Subdomain is taken",
  "suggestions": ["mystore-1", "mystore-shop", "mystore-store"]
}
```

## Quick Start

### Local Development

1. **Clone and navigate to the service:**
   ```bash
   cd services/tenant-service
   ```

2. **Install dependencies:**
   ```bash
   go mod tidy
   ```

3. **Set up environment variables:**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Run database migrations:**
   ```bash
   psql tenant_db < migrations/001_initial_schema.sql
   ```

5. **Run the service:**
   ```bash
   go run cmd/main.go
   ```

6. **Access the API:**
   - Service: http://localhost:8086
   - Health: http://localhost:8086/health
   - Swagger: http://localhost:8086/swagger/index.html

### Docker

1. **Run with Docker Compose:**
   ```bash
   docker-compose up -d
   ```

## Database Schema

The service manages the following entities:

- **Onboarding Sessions**: Main onboarding flow tracking
- **Business Information**: Company details and registration data
- **Contact Information**: Primary contact details
- **Verification Records**: Email and phone verification status
- **Payment Information**: Subscription and billing details
- **Application Configuration**: App-specific settings
- **Tasks**: Individual onboarding task tracking
- **Templates**: Reusable onboarding flow templates
- **Step Definitions**: Configurable onboarding steps

## Onboarding Flow Templates

### E-commerce Template
```json
{
  "id": "ecommerce-standard",
  "name": "E-commerce Store Setup",
  "description": "Complete setup for online stores",
  "application_type": "ecommerce",
  "steps": [
    {
      "id": "business-registration",
      "name": "Business Registration",
      "type": "business_info",
      "required": true,
      "order": 1
    },
    {
      "id": "email-verification",
      "name": "Email Verification",
      "type": "verification",
      "required": true,
      "order": 2,
      "dependencies": ["business-registration"]
    },
    {
      "id": "store-setup",
      "name": "Store Configuration",
      "type": "application_setup",
      "required": true,
      "order": 3,
      "config": {
        "fields": ["store_name", "currency", "timezone", "categories"]
      }
    },
    {
      "id": "payment-setup",
      "name": "Payment Configuration",
      "type": "payment",
      "required": true,
      "order": 4
    },
    {
      "id": "theme-selection",
      "name": "Choose Theme",
      "type": "application_setup",
      "required": false,
      "order": 5,
      "config": {
        "type": "theme_selection"
      }
    }
  ]
}
```

### SaaS Template
```json
{
  "id": "saas-platform",
  "name": "SaaS Platform Setup",
  "description": "Setup for SaaS applications",
  "application_type": "saas",
  "steps": [
    {
      "id": "business-registration",
      "name": "Business Registration",
      "type": "business_info",
      "required": true,
      "order": 1
    },
    {
      "id": "workspace-setup",
      "name": "Workspace Configuration",
      "type": "application_setup",
      "required": true,
      "order": 2,
      "config": {
        "fields": ["workspace_name", "subdomain", "timezone"]
      }
    },
    {
      "id": "user-roles",
      "name": "User Roles Setup",
      "type": "application_setup",
      "required": true,
      "order": 3,
      "config": {
        "type": "user_management"
      }
    },
    {
      "id": "integrations",
      "name": "API & Integrations",
      "type": "application_setup",
      "required": false,
      "order": 4,
      "config": {
        "type": "api_setup"
      }
    }
  ]
}
```

## Integration

This service integrates with:

- **Any Frontend Application**: Via standard REST API
- **Settings Service**: For tenant configuration storage
- **Auth Service**: For user authentication and authorization
- **Notification Service**: For email/SMS communications
- **Payment Providers**: Stripe, PayPal, Razorpay for billing
- **Domain Services**: For subdomain and custom domain setup

## Port Allocation

Following the service standards:
- **tenant-service**: Port 8086 (Common domain services 8080-8099)

## Development

### Adding New Application Types

1. **Create Template**: Add new template JSON in `templates/`
2. **Add Handlers**: Extend handlers for specific setup steps
3. **Update Models**: Add application-specific configuration models
4. **Configure Validation**: Add validation rules for new fields

### Template Configuration

Templates are stored as JSON files and can be customized per tenant:

```go
type OnboardingTemplate struct {
    ID              string                 `json:"id"`
    Name            string                 `json:"name"`
    Description     string                 `json:"description"`
    ApplicationType string                 `json:"application_type"`
    Steps           []OnboardingStep       `json:"steps"`
    Metadata        map[string]interface{} `json:"metadata"`
}

type OnboardingStep struct {
    ID           string                 `json:"id"`
    Name         string                 `json:"name"`
    Type         string                 `json:"type"`
    Required     bool                   `json:"required"`
    Order        int                    `json:"order"`
    Dependencies []string               `json:"dependencies"`
    Config       map[string]interface{} `json:"config"`
}
```

## Security

- CORS protection for cross-origin requests
- Request ID tracking for audit trails
- Input validation on all endpoints
- SQL injection protection via GORM
- Secure password hashing with bcrypt
- Rate limiting for verification endpoints
- Tenant isolation and data protection
- Panic recovery middleware

## Monitoring

The service exposes:

- Health check endpoint for liveness/readiness probes
- Request logging with correlation IDs
- Error tracking and recovery middleware
- Database connection monitoring
- Onboarding completion metrics
- Template usage analytics

## Production Readiness

### ✅ Completed Features

#### Database & Models
- [x] All database models defined and migrated (Tenant, User, OnboardingSession, etc.)
- [x] UUID primary keys for all entities
- [x] Proper foreign key relationships
- [x] Auto-migration on service startup
- [x] Database connection pooling configured

#### Onboarding Flow
- [x] Multi-step onboarding process (Business Info → Contact Details → Address → Store Setup)
- [x] Email verification with OTP codes
- [x] Session-based progress tracking
- [x] Template-based configuration per application type
- [x] Task management with dependencies
- [x] Welcome page with password setup
- [x] Account creation with tenant and user records

#### API Endpoints
- [x] Session management (create, get, update, complete)
- [x] Business information capture
- [x] Contact information management
- [x] Address information handling
- [x] Email verification workflow
- [x] Account setup endpoint
- [x] Progress tracking
- [x] Health and readiness checks

#### Security
- [x] CORS configuration for allowed origins
- [x] Request ID tracking for audit trails
- [x] Structured logging with correlation IDs
- [x] Panic recovery middleware
- [x] Input validation
- [x] SQL injection protection via GORM

#### UI/UX
- [x] Tenant onboarding app (Next.js 16 with Turbopack)
- [x] Welcome page with password setup
- [x] Consistent black button theme across onboarding and welcome flows
- [x] Password strength indicator
- [x] Success/error toast notifications
- [x] Mobile-responsive design

### ⚠️ Production Blockers

#### Critical
- [ ] **Password Hashing**: Currently storing passwords in plain text (Line 432 in onboarding_service.go)
  - Need to implement bcrypt hashing before account creation
  - Update User model to handle hashed passwords

- [ ] **Environment Variables**: Hardcoded URLs in frontend components
  - Replace hardcoded `http://localhost:8086` in Welcome.tsx with environment variables
  - Configure proper production URLs for all services

- [ ] **Email Service Integration**: Mock implementation for welcome emails
  - Integrate with real SMTP service or email provider (SendGrid, AWS SES, etc.)
  - Configure email templates
  - Add email delivery tracking

#### High Priority
- [ ] **Rate Limiting**: Verification endpoint needs rate limiting
  - Implement rate limiting for verification code requests
  - Add cooldown period tracking
  - Configure max attempts per time window

- [ ] **Subdomain Generation**: Current implementation uses simple hashing
  - Improve subdomain generation algorithm
  - Add collision detection and resolution
  - Validate subdomain availability against reserved words

- [ ] **Session Expiry**: Sessions need automatic cleanup
  - Implement background job to clean expired sessions
  - Add session renewal mechanism
  - Configure session timeout policies

- [ ] **Error Handling**: More granular error responses needed
  - Add specific error codes for different failure scenarios
  - Improve error messages for client consumption
  - Implement error tracking and monitoring integration

#### Medium Priority
- [ ] **Verification Service Integration**: Currently mocked
  - Complete integration with verification-service
  - Implement retry logic for failed verifications
  - Add webhook notifications for verification status

- [ ] **Payment Integration**: Payment service implementation pending
  - Integrate with Stripe/PayPal/Razorpay
  - Implement subscription management
  - Add payment webhook handlers

- [ ] **Metrics & Observability**: Basic metrics implemented
  - Add business metrics (conversion rates, drop-off points)
  - Implement distributed tracing
  - Add alerting for critical errors
  - Configure log aggregation

- [ ] **Testing**: Comprehensive test coverage needed
  - Unit tests for all services and handlers
  - Integration tests for API endpoints
  - End-to-end tests for onboarding flow
  - Load testing for concurrent onboarding sessions

#### Low Priority
- [ ] **Documentation**: API documentation needs completion
  - Add OpenAPI/Swagger spec
  - Document all error codes
  - Add integration guides
  - Create deployment runbooks

- [ ] **Tenant Isolation**: Multi-tenancy needs verification
  - Add tenant context middleware
  - Implement tenant-scoped queries
  - Add tenant data isolation tests

- [ ] **Backup & Recovery**: Disaster recovery plan needed
  - Implement database backup strategy
  - Add point-in-time recovery capability
  - Document recovery procedures

### 🚀 Deployment Checklist

Before deploying to production:

1. **Database**
   - [ ] Run all migrations in staging environment
   - [ ] Configure database connection pooling
   - [ ] Set up automated backups
   - [ ] Configure read replicas (if needed)

2. **Security**
   - [ ] Enable password hashing with bcrypt
   - [ ] Configure HTTPS/TLS certificates
   - [ ] Set up API rate limiting
   - [ ] Review and update CORS policies
   - [ ] Rotate all secrets and API keys
   - [ ] Enable security headers

3. **Configuration**
   - [ ] Set all environment variables
   - [ ] Configure email service credentials
   - [ ] Set up payment provider keys
   - [ ] Configure production URLs
   - [ ] Set proper session timeouts

4. **Monitoring**
   - [ ] Set up application monitoring (DataDog, New Relic, etc.)
   - [ ] Configure log aggregation (ELK, Splunk, etc.)
   - [ ] Enable metrics collection (Prometheus)
   - [ ] Set up alerts for critical errors
   - [ ] Configure uptime monitoring

5. **Testing**
   - [ ] Run all unit and integration tests
   - [ ] Perform load testing
   - [ ] Execute security testing
   - [ ] Validate all user flows end-to-end

6. **Operations**
   - [ ] Document deployment procedures
   - [ ] Create rollback plan
   - [ ] Set up on-call rotation
   - [ ] Prepare incident response plan

### 📝 Known Issues

1. **Password Storage**: Passwords are currently stored in plain text (SECURITY RISK)
   - Location: `internal/services/onboarding_service.go:432`
   - Fix: Implement bcrypt hashing before production deployment

2. **Hardcoded URLs**: Frontend has hardcoded localhost URLs
   - Location: `domains/ecommerce/apps/admin/src/pages/Welcome.tsx`
   - Fix: Use environment variables for all service URLs

3. **Mock Email Service**: Welcome emails are logged but not sent
   - Location: `internal/services/onboarding_service.go:455`
   - Fix: Integrate with real email service provider

### 🎯 Recommended Production Architecture

```
┌─────────────────┐
│   Load Balancer │
│   (SSL/TLS)     │
└────────┬────────┘
         │
    ┌────┴────┐
    │  tenant │
    │ service │
    │ (8086)  │
    └────┬────┘
         │
    ┌────┴──────────────────────┐
    │                           │
┌───▼────┐              ┌──────▼─────┐
│ PostgreSQL │          │ Verification│
│ (Primary)  │          │  Service    │
└────────────┘          └─────────────┘
    │
┌───▼────────┐
│ PostgreSQL │
│ (Replica)  │
└────────────┘
```

### 📊 Performance Targets

- **Response Time**: < 200ms for 95th percentile
- **Throughput**: Handle 100 concurrent onboarding sessions
- **Availability**: 99.9% uptime SLA
- **Database**: < 50ms query latency
- **Session Completion**: < 10 minutes average time

### 🔄 Versioning & Updates

- Current Version: **v1.0.0-beta**
- API Version: **v1**
- Database Schema Version: **1.0**

For production deployment, ensure all critical and high-priority items are addressed.