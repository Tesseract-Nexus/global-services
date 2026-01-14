# Audit Service

Comprehensive audit logging and trail management microservice for the Tesseract Hub platform. Tracks all user actions, system events, and changes for security, compliance, and forensic analysis.

## Features

- **Automatic Request Logging**: Middleware captures all HTTP requests
- **Change Tracking**: Old/new values for UPDATE operations
- **Security Monitoring**: Critical events, failed auth, suspicious activity
- **Analytics**: Summary statistics with time-range aggregation
- **Export**: JSON and CSV export capabilities
- **Data Retention**: Configurable automatic cleanup

## Tech Stack

- **Language**: Go
- **Framework**: Gin
- **Database**: PostgreSQL with GORM

## API Endpoints

### Audit Logs
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/audit-logs` | Create audit log |
| GET | `/api/v1/audit-logs` | List with filters |
| GET | `/api/v1/audit-logs/:id` | Get by ID |

### Resource & User History
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/audit-logs/resource/:type/:id` | Resource modification history |
| GET | `/api/v1/audit-logs/user/:userId` | User activity |
| GET | `/api/v1/audit-logs/user/:userId/ip-history` | User IP addresses |

### Security & Analytics
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/audit-logs/critical` | Recent critical events |
| GET | `/api/v1/audit-logs/failed-auth` | Failed authentication attempts |
| GET | `/api/v1/audit-logs/suspicious-activity` | Suspicious patterns |
| GET | `/api/v1/audit-logs/summary` | Aggregated statistics |

### Export
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/audit-logs/export` | Export logs (JSON/CSV) |

## Query Parameters

### Filtering
- `action` - Filter by action type
- `resource` - Filter by resource type
- `resource_id` - Filter by resource ID
- `user_id` - Filter by user
- `status` - SUCCESS, FAILURE, PENDING
- `severity` - LOW, MEDIUM, HIGH, CRITICAL
- `ip_address` - Filter by IP
- `service_name` - Filter by service
- `search` - Full-text search
- `from_date`, `to_date` - Date range (RFC3339)

### Pagination
- `limit` - Items per page (default: 50)
- `offset` - Pagination offset
- `sort_by` - Sort field (default: timestamp)
- `sort_order` - ASC or DESC

## Action Types

**Authentication**: LOGIN, LOGOUT, LOGIN_FAILED, PASSWORD_RESET, PASSWORD_CHANGE
**CRUD**: CREATE, READ, UPDATE, DELETE
**Data**: EXPORT, IMPORT
**Workflow**: APPROVE, REJECT, COMPLETE, CANCEL
**RBAC**: ROLE_ASSIGN, ROLE_REMOVE, PERMISSION_GRANT, PERMISSION_REVOKE
**Config**: CONFIG_UPDATE, SETTING_CHANGE

## Resource Types

User, Role, Permission, Category, Product, Order, Customer, Vendor, Return, Refund, Shipment, Payment, Notification, Document, Settings, Config, Auth

## Severity Levels

- **LOW**: Read operations, routine actions
- **MEDIUM**: Standard CRUD operations
- **HIGH**: Sensitive operations, config changes
- **CRITICAL**: Security events, failed auth, permission changes

## Suspicious Activity Detection

- Multiple failed logins from same IP
- High volume of critical events
- Unusual access patterns
- Brute-force detection

## Data Model

### AuditLog
- UUID primary key with tenant isolation
- User info: ID, username, email
- Action and resource details
- Request context: method, path, IP, user agent
- Change tracking: old values, new values, diff
- Severity and status
- Service name and version
- Metadata and tags (JSONB)

## Database Indexes

- 12 single-column indexes for common queries
- 5 composite indexes for tenant-scoped queries
- 3 GIN indexes for JSONB fields
- Full-text search indexes on description and resource_name

## Integration

This service is designed to be:
1. Imported as a Go module in other services
2. Registered as middleware in Gin router
3. Called directly via AuditService interface
4. Queried via REST API for analysis

## License

MIT
