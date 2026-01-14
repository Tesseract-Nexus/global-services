.PHONY: help build up down logs shell migrate test clean

# Default target
help:
	@echo "Available commands:"
	@echo "  make build       - Build Docker images"
	@echo "  make up          - Start all services"
	@echo "  make down        - Stop all services"
	@echo "  make logs        - View service logs"
	@echo "  make shell       - Access auth-service shell"
	@echo "  make db-shell    - Access PostgreSQL shell"
	@echo "  make migrate     - Run database migrations"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean up volumes and images"
	@echo "  make dev         - Start in development mode"
	@echo "  make prod        - Start in production mode"

# Build Docker images
build:
	docker-compose build

# Start all services
up:
	docker-compose up -d

# Start with logs
up-logs:
	docker-compose up

# Stop all services
down:
	docker-compose down

# Stop and remove volumes
down-v:
	docker-compose down -v

# View logs
logs:
	docker-compose logs -f

# View specific service logs
logs-auth:
	docker-compose logs -f auth-service

logs-db:
	docker-compose logs -f postgres

logs-redis:
	docker-compose logs -f redis

# Access shells
shell:
	docker-compose exec auth-service sh

db-shell:
	docker-compose exec postgres psql -U authuser -d authdb

redis-shell:
	docker-compose exec redis redis-cli

# Run migrations
migrate:
	docker-compose exec postgres psql -U authuser -d authdb -f /docker-entrypoint-initdb.d/001_init.sql
	docker-compose exec postgres psql -U authuser -d authdb -f /docker-entrypoint-initdb.d/002_b2c_multitenant_setup.sql
	docker-compose exec postgres psql -U authuser -d authdb -f /docker-entrypoint-initdb.d/003_add_2fa_support.sql

# Development mode
dev:
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml up

# Production mode
prod:
	docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Run tests
test:
	docker-compose exec auth-service go test ./...

# Clean up
clean:
	docker-compose down -v
	docker rmi auth-service_auth-service || true

# Rebuild and restart
restart: down build up

# Check service health
health:
	@echo "Checking service health..."
	@curl -f http://localhost:3080/health || echo "Auth service is not healthy"
	@docker-compose exec postgres pg_isready -U authuser -d authdb || echo "PostgreSQL is not ready"
	@docker-compose exec redis redis-cli ping || echo "Redis is not responding"

# Initialize database with sample data
init-db:
	@echo "Initializing database with sample data..."
	docker-compose exec auth-service go run cmd/seed/main.go

# Backup database
backup:
	@mkdir -p backups
	docker-compose exec postgres pg_dump -U authuser authdb > backups/authdb_$(shell date +%Y%m%d_%H%M%S).sql
	@echo "Database backed up to backups/"

# Restore database
restore:
	@echo "Available backups:"
	@ls -la backups/*.sql
	@echo "To restore, run: docker-compose exec -T postgres psql -U authuser authdb < backups/[filename].sql"