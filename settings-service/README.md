# Settings Service

A comprehensive settings management service for Tesseract Hub applications, providing centralized configuration management across different scopes (global, tenant, application, user).

## Features

- **Multi-scope Settings**: Support for global, tenant, application, and user-specific settings
- **Comprehensive Configuration**: Branding, themes, layouts, animations, localization, features, and more
- **Settings Inheritance**: Automatic fallback from user → application → tenant → global
- **Presets System**: Pre-defined settings configurations for quick setup
- **Change History**: Full audit trail of all settings modifications
- **Validation**: Built-in validation for settings data
- **RESTful API**: Complete HTTP API with OpenAPI/Swagger documentation

## Quick Start

### Development with Docker Compose

```bash
# Start the service with PostgreSQL
docker-compose up -d

# Check service health
curl http://localhost:8085/health

# View API documentation
open http://localhost:8085/swagger/index.html
```

### Local Development

1. **Prerequisites**:
   - Go 1.22+
   - PostgreSQL 15+

2. **Environment Setup**:
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Database Setup**:
   ```bash
   # Create database
   createdb settings_db
   
   # Run migrations
   psql settings_db < migrations/001_initial_schema.sql
   ```

4. **Run Service**:
   ```bash
   go mod download
   go run cmd/main.go
   ```

## API Endpoints

### Settings Management

- `POST /api/v1/settings` - Create new settings
- `GET /api/v1/settings` - List settings with filtering
- `GET /api/v1/settings/{id}` - Get settings by ID
- `GET /api/v1/settings/context` - Get settings by context
- `GET /api/v1/settings/inherited` - Get inherited settings with fallback
- `PUT /api/v1/settings/{id}` - Update settings
- `DELETE /api/v1/settings/{id}` - Delete settings
- `GET /api/v1/settings/{id}/history` - Get settings change history

### Presets

- `GET /api/v1/presets` - List available presets
- `POST /api/v1/settings/{settingsId}/apply-preset/{presetId}` - Apply preset to settings

### Headers

All requests require:
- `X-Tenant-ID`: Tenant identifier (UUID)
- `X-User-ID`: User identifier (UUID, optional)

## Settings Structure

### Context
Settings are organized by context:
- **Global**: System-wide defaults
- **Tenant**: Organization-specific settings
- **Application**: App-specific settings
- **User**: Personal user preferences

### Categories

1. **Branding**: Company info, logos, colors, fonts
2. **Theme**: Color schemes, dark/light mode, typography
3. **Layout**: Sidebar, navigation, page layouts
4. **Animations**: Transitions, effects, motion preferences
5. **Localization**: Language, currency, timezone, formats
6. **Features**: Feature flags, module toggles
7. **User Preferences**: Dashboard layout, notifications, privacy
8. **Application**: Security, performance, API endpoints

## Examples

### Create Settings
```bash
curl -X POST http://localhost:8085/api/v1/settings \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: 123e4567-e89b-12d3-a456-426614174000" \
  -d '{
    "context": {
      "tenantId": "123e4567-e89b-12d3-a456-426614174000",
      "applicationId": "987fcdeb-51a2-43d1-9f45-123456789abc",
      "scope": "application"
    },
    "theme": {
      "colorMode": "dark",
      "colorScheme": "blue",
      "borderRadius": 0.75
    },
    "layout": {
      "sidebar": {
        "position": "left",
        "width": 300
      }
    }
  }'
```

### Get Inherited Settings
```bash
curl "http://localhost:8085/api/v1/settings/inherited?applicationId=987fcdeb-51a2-43d1-9f45-123456789abc&scope=user&userId=456e7890-e89b-12d3-a456-426614174000" \
  -H "X-Tenant-ID: 123e4567-e89b-12d3-a456-426614174000"
```

## Configuration

Environment variables:

- `PORT`: Server port (default: 8085)
- `HOST`: Server host (default: 0.0.0.0)
- `GIN_MODE`: Gin mode (debug/release)
- `DB_HOST`: PostgreSQL host
- `DB_PORT`: PostgreSQL port
- `DB_USER`: Database user
- `DB_PASSWORD`: Database password
- `DB_NAME`: Database name
- `DB_SSL_MODE`: SSL mode (disable/require)
- `ENVIRONMENT`: Environment (development/staging/production)
- `DEBUG`: Enable debug logging
- `VERSION`: Service version

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │────│   Gin Router    │────│    Handlers     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                       ┌─────────────────┐    ┌─────────────────┐
                       │   Middleware    │    │    Services     │
                       └─────────────────┘    └─────────────────┘
                                                        │
                       ┌─────────────────┐    ┌─────────────────┐
                       │     Models      │    │   Repository    │
                       └─────────────────┘    └─────────────────┘
                                                        │
                                              ┌─────────────────┐
                                              │   PostgreSQL    │
                                              └─────────────────┘
```

## Contributing

1. Follow the established patterns from other services
2. Update API documentation when adding endpoints
3. Include tests for new functionality
4. Follow Go best practices and formatting
5. Update migration files for schema changes

## License

MIT License - see LICENSE file for details.