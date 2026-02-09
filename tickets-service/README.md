# Tickets Service

Enterprise support ticket management microservice for the Tesseract Hub platform. Provides multi-tenant ticket lifecycle management with attachments, comments, and bulk operations.

## Features

- **Ticket Management**: Full CRUD with status and priority tracking
- **Comments System**: Internal and public comments with threading
- **Attachments**: File upload with type validation and presigned URLs
- **Bulk Operations**: Batch status, assignment, and priority updates
- **Search & Analytics**: Full-text search and ticket statistics

## Tech Stack

- **Language**: Go 1.25
- **Framework**: Gin v1.9.1
- **Database**: PostgreSQL with GORM v1.25.2
- **Port**: 8085

## API Endpoints

### Tickets
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/tickets` | Create ticket |
| GET | `/api/v1/tickets` | List tickets with pagination |
| GET | `/api/v1/tickets/:id` | Get ticket by ID |
| PUT | `/api/v1/tickets/:id` | Update ticket |
| DELETE | `/api/v1/tickets/:id` | Delete ticket |

### Status & Assignment
| Method | Endpoint | Description |
|--------|----------|-------------|
| PUT | `/api/v1/tickets/:id/status` | Update status |
| POST | `/api/v1/tickets/:id/assign` | Assign to user(s) |
| DELETE | `/api/v1/tickets/:id/assign/:assigneeId` | Unassign user |

### Comments
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/tickets/:id/comments` | Add comment |
| PUT | `/api/v1/tickets/:id/comments/:commentId` | Update comment |
| DELETE | `/api/v1/tickets/:id/comments/:commentId` | Delete comment |

### Attachments
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/tickets/attachments/upload` | Upload attachment |
| GET | `/api/v1/tickets/:id/attachments` | List attachments |
| POST | `/api/v1/tickets/:id/attachments/presigned-url` | Get presigned URL |
| DELETE | `/api/v1/tickets/:id/attachments/:attachmentId` | Delete attachment |

### Bulk Operations
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/tickets/bulk/status` | Bulk update status |
| POST | `/api/v1/tickets/bulk/assign` | Bulk assign |
| POST | `/api/v1/tickets/bulk/priority` | Bulk update priority |

### Advanced Features
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/tickets/:id/escalate` | Escalate ticket |
| POST | `/api/v1/tickets/:id/clone` | Clone ticket |
| POST | `/api/v1/tickets/search` | Full-text search |
| GET | `/api/v1/tickets/:id/similar` | Find similar tickets |

### Analytics
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/tickets/analytics` | Get analytics |
| GET | `/api/v1/tickets/stats` | Get statistics |
| POST | `/api/v1/tickets/export` | Export tickets |

### Health
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |

## Environment Variables

```env
# Database
DB_HOST=localhost
DB_PORT=5435
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=tickets_db
DB_SSL_MODE=disable

# Server
PORT=8085
ENVIRONMENT=development
JWT_SECRET=your-secret-key

# External Services
DOCUMENT_SERVICE_URL=http://localhost:8082
NOTIFICATION_SERVICE_URL=http://localhost:8092
ESCALATION_SERVICE_URL=http://localhost:8093

# Ticket Settings
MAX_TICKET_LENGTH=10000
MAX_ATTACHMENTS_PER_TICKET=20
DEFAULT_SLA_HOURS=24
AUTO_ESCALATION_ENABLED=true

# Pagination
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
```

## Data Models

### Ticket Status
- OPEN, IN_PROGRESS, ON_HOLD, RESOLVED, CLOSED
- REOPENED, CANCELLED, PENDING_APPROVAL, ESCALATED

### Ticket Priority
- LOW, MEDIUM, HIGH, CRITICAL, URGENT

### Ticket Types
- BUG, FEATURE, SUPPORT, INCIDENT, IMPROVEMENT
- CHANGE_REQUEST, MAINTENANCE, CONSULTATION, COMPLAINT, QUESTION

### Attachment Types
- screenshot, log_file, document, evidence
- solution, config_file, error_dump

## File Size Limits
- Images: 10MB
- Documents: 50MB
- Log files: 100MB

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
