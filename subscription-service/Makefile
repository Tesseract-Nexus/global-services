.PHONY: help build run clean test deps migrate-up migrate-down docker-build docker-run dev

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the subscription service binary
	@echo "Building subscription service..."
	@go build -o bin/subscription-service ./cmd/main.go
	@echo "Build complete: bin/subscription-service"

run: ## Run the subscription service
	@echo "Starting subscription service..."
	@if [ -f .env ]; then \
		export $$(cat .env | xargs) && go run cmd/main.go; \
	else \
		go run cmd/main.go; \
	fi

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@echo "Clean complete"

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies downloaded"

migrate-up: ## Run database migrations
	@echo "Running migrations..."
	@psql $(DATABASE_URL) -f migrations/001_create_subscription_tables.sql
	@psql $(DATABASE_URL) -f migrations/002_seed_subscription_plans.sql
	@echo "Migrations complete"

migrate-down: ## Rollback database migrations
	@echo "Rolling back migrations..."
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS subscription_events CASCADE;"
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS subscription_invoices CASCADE;"
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS tenant_subscriptions CASCADE;"
	@psql $(DATABASE_URL) -c "DROP TABLE IF EXISTS subscription_plans CASCADE;"
	@echo "Rollback complete"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t subscription-service:latest .
	@echo "Docker image built"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run -p 8093:8080 --env-file .env subscription-service:latest

dev: ## Run in development mode with hot reload
	@echo "Starting development server..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not found. Install it with: go install github.com/cosmtrek/air@latest"; \
		make run; \
	fi
