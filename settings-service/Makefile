# Service variables
SERVICE_NAME=settings-service
SERVICE_PORT=8085
DB_NAME=settings_db
DB_USER=settings_user
DB_PASSWORD=settings_password_dev
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@localhost:5442/$(DB_NAME)?sslmode=disable

# Build variables
BINARY_NAME=bin/$(SERVICE_NAME)
DOCKER_IMAGE=$(SERVICE_NAME):latest
DOCKER_REGISTRY=tesseract-hub

# Go variables
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin
GOFILES=$(wildcard *.go)

# Make settings
.PHONY: all build run test lint clean docker-build docker-run docker-stop migrate migrate-down help

# Default target
all: test build

# Build the binary
build:
	@echo "Building $(SERVICE_NAME)..."
	@go build -o $(BINARY_NAME) -v ./cmd/main.go
	@echo "Build complete: $(BINARY_NAME)"

# Run the application locally
run: build
	@echo "Running $(SERVICE_NAME)..."
	@$(BINARY_NAME)

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./... -cover -coverprofile=coverage.out
	@echo "Tests complete"

# Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run --timeout=5m
	@echo "Lint complete"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@go clean
	@rm -rf $(GOBIN)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

# Docker commands
docker-build:
	@echo "Building Docker image..."
	@docker build -f Dockerfile -t $(DOCKER_IMAGE) ../../../../
	@echo "Docker build complete"

docker-run: docker-build
	@echo "Starting services with Docker Compose..."
	@docker-compose up -d
	@echo "Services started. Settings service available at http://localhost:$(SERVICE_PORT)"

docker-stop:
	@echo "Stopping services..."
	@docker-compose down
	@echo "Services stopped"

docker-logs:
	@docker-compose logs -f $(SERVICE_NAME)

# Database migrations
migrate:
	@echo "Running database migrations..."
	@migrate -path ./migrations -database "$(DB_URL)" up
	@echo "Migrations complete"

migrate-down:
	@echo "Rolling back database migrations..."
	@migrate -path ./migrations -database "$(DB_URL)" down 1
	@echo "Migration rollback complete"

migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir ./migrations -seq $$name

# Development helpers
dev: docker-run
	@echo "Starting development environment..."
	@docker-compose logs -f $(SERVICE_NAME)

dev-rebuild:
	@echo "Rebuilding and restarting services..."
	@docker-compose down
	@docker-compose build --no-cache
	@docker-compose up -d
	@docker-compose logs -f $(SERVICE_NAME)

# Generate Swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	@swag init -g cmd/main.go -o docs
	@echo "Swagger documentation generated in ./docs"

# Install development dependencies
install-deps:
	@echo "Installing dependencies..."
	@go get -u github.com/swaggo/swag/cmd/swag
	@go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	@go get -u -d github.com/golang-migrate/migrate/v4/cmd/migrate
	@go mod download
	@echo "Dependencies installed"

# Help
help:
	@echo "Available targets:"
	@echo "  make build          - Build the binary"
	@echo "  make run            - Run the application locally"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make lint           - Run linter"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-run     - Run with Docker Compose"
	@echo "  make docker-stop    - Stop Docker Compose services"
	@echo "  make docker-logs    - View service logs"
	@echo "  make migrate        - Run database migrations"
	@echo "  make migrate-down   - Rollback last migration"
	@echo "  make migrate-create - Create new migration"
	@echo "  make dev            - Start development environment"
	@echo "  make dev-rebuild    - Rebuild and restart services"
	@echo "  make swagger        - Generate Swagger documentation"
	@echo "  make install-deps   - Install development dependencies"