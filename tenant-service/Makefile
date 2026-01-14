# Tenant Service Makefile

.PHONY: help build run test test-integration test-coverage clean migrate seed dev docker-build docker-run

# Default target
help:
	@echo "Available targets:"
	@echo "  build              - Build the service binary"
	@echo "  run                - Run the service locally"
	@echo "  dev                - Run in development mode with hot reload"
	@echo "  test               - Run all tests"
	@echo "  test-integration   - Run integration tests only"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  migrate            - Run database migrations"
	@echo "  seed               - Seed database with initial data"
	@echo "  clean              - Clean build artifacts"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-run         - Run in Docker container"

# Build variables
BINARY_NAME=tenant-service
BUILD_DIR=./bin
MAIN_PATH=./main.go
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Build the service
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "✅ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the service locally
run: build
	@echo "Starting $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

# Development mode (requires air for hot reload)
dev:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "❌ 'air' not found. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Or run 'make run' instead"; \
	fi

# Run all tests
test:
	@echo "Running all tests..."
	@go test ./... -v -race

# Run integration tests only
test-integration:
	@echo "Running integration tests..."
	@go test ./tests/integration/... -v

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=$(COVERAGE_FILE) -covermode=atomic
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "✅ Coverage report generated: $(COVERAGE_HTML)"
	@go tool cover -func=$(COVERAGE_FILE) | grep total

# Clean build artifacts and test files
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@echo "✅ Clean complete"

# Database migrations (requires DB to be running)
migrate:
	@echo "Running database migrations..."
	@go run $(MAIN_PATH) migrate
	@echo "✅ Migrations complete"

# Seed database with initial data
seed:
	@echo "Seeding database..."
	@go run $(MAIN_PATH) seed
	@echo "✅ Seeding complete"

# Docker targets
docker-build:
	@echo "Building Docker image..."
	@docker build -t tesseract/$(BINARY_NAME):latest .
	@echo "✅ Docker image built"

docker-run:
	@echo "Running Docker container..."
	@docker run -p 8086:8086 --env-file .env tesseract/$(BINARY_NAME):latest

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod verify
	@echo "✅ Dependencies installed"

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy
	@echo "✅ Dependencies tidied"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatted"

# Lint code (requires golangci-lint)
lint:
	@if command -v golangci-lint > /dev/null; then \
		echo "Running linters..."; \
		golangci-lint run ./...; \
		echo "✅ Linting complete"; \
	else \
		echo "❌ 'golangci-lint' not found. Install from: https://golangci-lint.run/usage/install/"; \
	fi

# Security scan (requires gosec)
security:
	@if command -v gosec > /dev/null; then \
		echo "Running security scan..."; \
		gosec ./...; \
		echo "✅ Security scan complete"; \
	else \
		echo "❌ 'gosec' not found. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi

# Generate swagger docs (if using swag)
swagger:
	@if command -v swag > /dev/null; then \
		echo "Generating Swagger documentation..."; \
		swag init -g $(MAIN_PATH); \
		echo "✅ Swagger docs generated"; \
	else \
		echo "❌ 'swag' not found. Install with: go install github.com/swaggo/swag/cmd/swag@latest"; \
	fi

# Run all quality checks
quality: fmt lint test security
	@echo "✅ All quality checks passed"

# Setup development environment
setup:
	@echo "Setting up development environment..."
	@cp .env.example .env 2>/dev/null || echo "⚠️  .env.example not found, skipping"
	@make deps
	@echo "✅ Development environment setup complete"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Update .env with your configuration"
	@echo "  2. Start PostgreSQL: docker-compose up -d postgres"
	@echo "  3. Run migrations: make migrate"
	@echo "  4. Start service: make run"

# Watch and run tests on file changes (requires entr)
test-watch:
	@if command -v entr > /dev/null; then \
		find . -name '*.go' | entr -c go test ./... -v; \
	else \
		echo "❌ 'entr' not found. Install from your package manager"; \
		echo "  macOS: brew install entr"; \
		echo "  Linux: apt-get install entr"; \
	fi

# Benchmark tests
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Create test database
test-db-create:
	@echo "Creating test database..."
	@psql -U dev -h localhost -c "CREATE DATABASE tesseract_hub_test;" 2>/dev/null || echo "Database may already exist"
	@echo "✅ Test database ready"

# Drop test database
test-db-drop:
	@echo "Dropping test database..."
	@psql -U dev -h localhost -c "DROP DATABASE IF EXISTS tesseract_hub_test;"
	@echo "✅ Test database dropped"

# Reset test database
test-db-reset: test-db-drop test-db-create
	@echo "✅ Test database reset complete"
