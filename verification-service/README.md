# Verification Service

OTP verification and transactional email microservice for the Tesseract Hub platform. Provides secure verification code generation, validation, and templated email delivery.

## Features

- **OTP Generation**: Cryptographically secure 6-digit codes
- **Code Encryption**: AES-256-GCM encryption for stored codes
- **Rate Limiting**: Configurable limits on code sends and attempts
- **Email Delivery**: Resend and SendGrid provider support
- **Email Templates**: Pre-built templates for common scenarios
- **Prometheus Metrics**: Built-in monitoring and metrics

## Tech Stack

- **Language**: Go 1.25
- **Framework**: Gin v1.11
- **Database**: PostgreSQL with GORM v1.31
- **Port**: 8088

## API Endpoints

### Verification
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/verify/send` | Send verification code |
| POST | `/api/v1/verify/code` | Verify a code |
| POST | `/api/v1/verify/resend` | Resend verification code |
| GET | `/api/v1/verify/status` | Check verification status |

### Email
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/email/send` | Send templated email |

### Health & Monitoring
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| GET | `/metrics` | Prometheus metrics |

## Environment Variables

```env
# Server
PORT=8088
GIN_MODE=debug

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=tesseract_hub
DB_SSLMODE=disable

# Email Provider
EMAIL_PROVIDER=resend
EMAIL_API_KEY=your-api-key
EMAIL_FROM=onboarding@tesserix.app
EMAIL_FROM_NAME=Tesserix Hub

# Security
API_KEY=your-api-key
ENCRYPTION_KEY=32-character-encryption-key

# OTP Configuration
OTP_LENGTH=6
OTP_EXPIRY_MINUTES=10

# Rate Limiting
MAX_VERIFICATION_ATTEMPTS=3
MAX_CODES_PER_HOUR=5
COOLDOWN_MINUTES=60
```

## Email Templates

- **welcome**: Welcome email (green theme)
- **account_created**: Account confirmation (indigo theme)
- **email_verification_link**: Verification link email
- **welcome_pack**: Comprehensive onboarding email

## Data Models

### VerificationCode
- Encrypted OTP code with SHA-256 hash for lookup
- Purpose: email_verification, password_reset, etc.
- Session and tenant context linking
- Attempt tracking with max attempts
- Expiration management

### VerificationAttempt
- Audit trail of verification attempts
- IP address and user agent tracking
- Success/failure tracking with reasons

### RateLimit
- Per-identifier rate limiting
- Sliding window implementation
- Separate limits for send and verify operations

## Security Features

- AES-256-GCM encryption for codes
- SHA-256 hashing for secure lookup
- API key authentication
- Rate limiting to prevent abuse
- Attempt tracking with lockout

## Prometheus Metrics

- `codes_generated_total`: Counter by type
- `attempts_total`: Counter by type and result
- `rate_limits_hit_total`: Counter by limit type
- `active_verifications`: Gauge of pending codes
- `db_connections_*`: Database pool metrics

## Running Locally

```bash
# Set environment variables
cp .env.example .env

# Run with Docker
docker-compose up

# Or run directly
go run cmd/main.go
```

## License

MIT
