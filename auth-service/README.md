# Authentication Service

> ⛔ **DEPRECATED - SCHEDULED FOR DECOMMISSION**
>
> This service is **deprecated** and is being replaced by **Keycloak**.
> All new development should use Keycloak via the `auth-bff` service.
>
> ## Keycloak Configuration
>
> | Environment | Keycloak URL | Realm | Client ID |
> |-------------|--------------|-------|-----------|
> | Development | https://devtest-customer-idp.tesserix.app | tesserix-customer | marketplace-dashboard |
> | Internal Staff | https://devtest-internal-idp.tesserix.app | tesserix-internal | admin-dashboard |
>
> ## Migration Guide
>
> 1. **New User Registration**: Use Keycloak via `tenant-service` or `auth-bff`
> 2. **Token Validation**: Configure `AUTH_VALIDATOR_TYPE=hybrid` to accept both Keycloak (RS256) and legacy (HS256) tokens
> 3. **User Migration**: Use tools in `/tools/keycloak-migration/` to migrate existing users
> 4. **Keycloak Setup**: Use `/tools/keycloak-setup/setup-keycloak.sh` for realm configuration
>
> ## Decommission Timeline
>
> 1. ✅ Hybrid mode enabled (current state)
> 2. ✅ Keycloak realms configured (tesserix-customer, tesserix-internal)
> 3. ✅ Auth BFF service ready (`services/auth-bff`)
> 4. ⏳ User migration to Keycloak
> 5. ⏳ Update API Gateway to route auth to `auth-bff`
> 6. ⏳ Switch to Keycloak-only mode (`AUTH_VALIDATOR_TYPE=keycloak`)
> 7. ⏳ Decommission this service
>
> ## Replacement Services
>
> - **auth-bff**: BFF service for web app OIDC flows (login, logout, session management)
> - **Keycloak**: Identity provider for all authentication and authorization
> - **packages/go-shared/auth**: Keycloak token validation for backend services

A comprehensive authentication and role-based access control (RBAC) service for the Tesseract Hub e-commerce platform.

## Features

### 🔐 Authentication
- JWT-based authentication with access and refresh tokens
- Password authentication with bcrypt
- Azure AD integration support
- Session management with Redis storage
- Token validation and refresh
- Secure logout with token revocation

### 🔑 Two-Factor Authentication (2FA)
- TOTP-based 2FA with authenticator apps
- Backup codes for recovery
- QR code generation for easy setup
- Compatible with Google Authenticator, Authy, etc.

### 📧 Email Features
- Email verification for new accounts
- Password reset flow with secure tokens
- Professional email templates
- SMTP support with development mode

### 👥 Role-Based Access Control (RBAC)
- Hierarchical role system
- Fine-grained permissions
- Multi-tenant support
- System and custom roles
- Permission inheritance through roles

### 🏢 Multi-Tenant Support
- B2C multi-tenant architecture
- Store isolation
- Tenant-specific roles and permissions
- Cross-tenant data protection

### 🛡️ Security Features
- CORS protection
- Security headers
- Rate limiting (configurable)
- Session timeout management
- SQL injection prevention
- 2FA enforcement options

## Quick Start with Docker

### Prerequisites
- Docker and Docker Compose installed
- Make (optional, for easier commands)

### 1. Clone and Setup
```bash
# Copy environment variables
cp .env.example .env

# Edit .env with your configuration
nano .env
```

### 2. Start Services
```bash
# Using Make
make up

# Or using Docker Compose directly
docker-compose up -d
```

### 3. Check Service Health
```bash
# Using Make
make health

# Or manually
curl http://localhost:3080/health
```

The service will be available at:
- Auth Service: `http://localhost:3080`
- PostgreSQL: `localhost:5432`
- Redis: `localhost:6379`
- Adminer (DB UI): `http://localhost:8080` (when using debug profile)

### 4. Stop Services
```bash
# Using Make
make down

# Or using Docker Compose
docker-compose down
```

## System Roles

| Role | Description | Permissions |
|------|-------------|-------------|
| `super_admin` | System administrator | All permissions |
| `tenant_admin` | Tenant administrator | Tenant-level management |
| `category_manager` | Category management | Category CRUD + approval |
| `product_manager` | Product management | Product CRUD + approval |
| `vendor_manager` | Vendor management | Vendor CRUD + approval |
| `staff` | Staff member | Read-only access |
| `vendor` | Vendor user | Own products management |
| `customer` | Customer user | Order management |

## API Endpoints

### Authentication
```http
POST /api/v1/auth/login          # User login (password or Azure AD)
POST /api/v1/auth/register       # Register new user with password
POST /api/v1/auth/verify-email   # Verify email address
POST /api/v1/auth/refresh        # Refresh access token
POST /api/v1/auth/logout         # User logout
POST /api/v1/auth/validate       # Validate token
```

### User Profile (Protected)
```http
GET /api/v1/profile              # Get current user profile
```

### Permission Checking (Protected)
```http
GET /api/v1/permissions/:permission      # Check single permission
POST /api/v1/permissions/check           # Check multiple permissions
```

### Metadata (Protected)
```http
GET /api/v1/roles/available             # Available roles
GET /api/v1/permissions/available       # Available permissions
```

### Admin - User Management
```http
GET /api/v1/admin/users                  # List users
GET /api/v1/admin/users/:user_id         # Get user details
GET /api/v1/admin/users/:user_id/roles   # Get user's roles
POST /api/v1/admin/users/:user_id/roles  # Assign role to user
DELETE /api/v1/admin/users/:user_id/roles # Remove role from user
```

### Admin - Role Management
```http
GET /api/v1/admin/roles                       # List all roles
GET /api/v1/admin/roles/:role_id              # Get role details
POST /api/v1/admin/roles                      # Create new role
PUT /api/v1/admin/roles/:role_id              # Update role
DELETE /api/v1/admin/roles/:role_id           # Delete role
GET /api/v1/admin/roles/:role_id/permissions  # Get role's permissions
POST /api/v1/admin/roles/:role_id/permissions # Assign permission to role
DELETE /api/v1/admin/roles/:role_id/permissions # Remove permission from role
```

### Admin - Permission Management
```http
GET /api/v1/admin/permissions            # List all permissions
```

### Health & Monitoring
```http
GET /health                              # Health check
GET /ready                               # Readiness check
```

## Usage Examples

### 1. User Login
```bash
curl -X POST http://localhost:3080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "name": "John Doe",
    "azure_object_id": "12345-67890",
    "tenant_id": "default-tenant"
  }'
```

### 2. Check Permission
```bash
curl -X GET http://localhost:3080/api/v1/permissions/product:create \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

### 3. Assign Role
```bash
curl -X POST http://localhost:3080/api/v1/admin/users/USER_ID/roles \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role": "product_manager"}'
```

## Environment Variables

```env
# Server Configuration
PORT=3080
SERVER_HOST=0.0.0.0
GIN_MODE=debug

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=dev
DB_PASSWORD=devpass
DB_NAME=auth
DATABASE_URL=postgresql://dev:devpass@localhost:5432/auth

# Redis Configuration
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_URL=redis://localhost:6379

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-change-in-production
JWT_REFRESH_SECRET=your-refresh-token-secret-here

# Azure AD Configuration (Optional)
AZURE_AD_TENANT_ID=your-tenant-id
AZURE_AD_CLIENT_ID=your-client-id
AZURE_AD_CLIENT_SECRET=your-client-secret
```

## Development Setup

### 1. Prerequisites
- Go 1.21+
- PostgreSQL 13+
- Redis 6+ (optional)

### 2. Database Setup
```bash
# Create database
createdb auth

# Run migrations
psql -d auth -f migrations/001_create_auth_tables.sql
```

### 3. Run Locally
```bash
# Install dependencies
go mod download

# Set environment variables
export DB_HOST=localhost
export DB_NAME=auth
export JWT_SECRET=your-secret-key

# Run the service
go run cmd/main.go
```

### 4. Docker Setup
```bash
# Build image
docker build -t tesseract-auth-service .

# Run container
docker run -p 3080:3080 \
  -e DB_HOST=host.docker.internal \
  -e JWT_SECRET=your-secret-key \
  tesseract-auth-service
```

## Integration with Frontend

### 1. Login Flow
```typescript
// Login request
const loginResponse = await fetch('/api/v1/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    email: user.email,
    name: user.name,
    azure_object_id: user.objectId,
    tenant_id: 'default-tenant'
  })
});

const { access_token, refresh_token, user } = await loginResponse.json();

// Store tokens
localStorage.setItem('access_token', access_token);
localStorage.setItem('refresh_token', refresh_token);
```

### 2. API Requests with Authentication
```typescript
// Add token to requests
const apiCall = async (url: string, options: RequestInit = {}) => {
  const token = localStorage.getItem('access_token');
  
  return fetch(url, {
    ...options,
    headers: {
      ...options.headers,
      'Authorization': `Bearer ${token}`,
      'X-Tenant-ID': 'default-tenant'
    }
  });
};
```

### 3. Permission Checking
```typescript
// Check single permission
const hasPermission = async (permission: string) => {
  const response = await apiCall(`/api/v1/permissions/${permission}`);
  const result = await response.json();
  return result.has_permission;
};

// Check multiple permissions
const checkPermissions = async (permissions: string[]) => {
  const response = await apiCall('/api/v1/permissions/check', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ permissions })
  });
  const result = await response.json();
  return result.permissions;
};
```

## Middleware Usage in Other Services

```go
// In other Go services
import "auth-service/internal/middleware"

// Protect routes
router.Use(authMiddleware.AuthRequired())

// Require specific permission
router.GET("/products", authMiddleware.RequirePermission("product:read"), handler)

// Require admin role
router.POST("/admin/users", authMiddleware.AdminOnly(), handler)
```

## Testing

### Unit Tests
```bash
go test ./internal/...
```

### Integration Tests
```bash
# Start test database
docker run -d --name test-postgres -p 5433:5432 -e POSTGRES_DB=auth_test postgres:13

# Run integration tests
DB_PORT=5433 DB_NAME=auth_test go test ./tests/...
```

### Load Testing
```bash
# Install hey
go install github.com/rakyll/hey@latest

# Test login endpoint
hey -n 1000 -c 10 -m POST \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","name":"Test User","tenant_id":"default-tenant"}' \
  http://localhost:3080/api/v1/auth/login
```

## Monitoring & Observability

### Health Check
```bash
curl http://localhost:3080/health
```

### Metrics (Prometheus format)
```bash
curl http://localhost:3080/metrics
```

### Logging
- Structured JSON logging
- Request/response logging
- Error tracking with stack traces
- Performance metrics

## Security Considerations

1. **Token Security**
   - Short-lived access tokens (8 hours)
   - Secure refresh token storage
   - Token rotation on refresh

2. **Database Security**
   - Parameterized queries (no SQL injection)
   - Connection pooling with limits
   - Encrypted passwords (bcrypt)

3. **Network Security**
   - HTTPS only in production
   - CORS properly configured
   - Security headers enabled

4. **Session Management**
   - Session timeout
   - Concurrent session limits
   - Session invalidation on logout

## Production Deployment

### Docker Compose
```yaml
version: '3.8'
services:
  auth-service:
    image: tesseract-auth-service:latest
    ports:
      - "3080:3080"
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis
      - JWT_SECRET=${JWT_SECRET}
    depends_on:
      - postgres
      - redis
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: auth-service
  template:
    spec:
      containers:
      - name: auth-service
        image: tesseract-auth-service:latest
        ports:
        - containerPort: 3080
        env:
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: auth-secrets
              key: jwt-secret
```

## Performance

- **Throughput**: ~5000 requests/second (login)
- **Latency**: <50ms (token validation)
- **Memory**: ~100MB base usage
- **Database**: Connection pooling optimized

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

MIT License - see LICENSE file for details.