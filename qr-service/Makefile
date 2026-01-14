.PHONY: build run test test-race test-cover lint deps docker-build docker-run clean help

BINARY_NAME=qr-service
DOCKER_IMAGE=ghcr.io/tesseract-nexus/tesseract-hub/qr-service

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) ./cmd/main.go

run:
	@echo "Running $(BINARY_NAME)..."
	go run ./cmd/main.go

test:
	@echo "Running tests..."
	go test -v ./...

test-race:
	@echo "Running tests with race detector..."
	go test -v -race ./...

test-cover:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	@echo "Running linter..."
	golangci-lint run

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):latest .

docker-run:
	@echo "Running Docker container..."
	docker-compose up -d qr-service

docker-run-gcs:
	@echo "Running Docker container with GCS..."
	docker-compose --profile gcs up -d qr-service-gcs

docker-stop:
	@echo "Stopping Docker container..."
	docker-compose down

docker-logs:
	@echo "Showing Docker logs..."
	docker-compose logs -f qr-service

clean:
	@echo "Cleaning up..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	docker-compose down -v

dev-setup:
	@echo "Setting up development environment..."
	go mod download
	go mod tidy

generate-key:
	@echo "Generating encryption key..."
	@openssl rand -hex 32

help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  run          - Run the service locally"
	@echo "  test         - Run tests"
	@echo "  test-race    - Run tests with race detector"
	@echo "  test-cover   - Run tests with coverage report"
	@echo "  lint         - Run linter"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container (local storage)"
	@echo "  docker-run-gcs - Run Docker container (GCS storage)"
	@echo "  docker-stop  - Stop Docker container"
	@echo "  docker-logs  - Show Docker logs"
	@echo "  clean        - Clean build artifacts"
	@echo "  dev-setup    - Setup development environment"
	@echo "  generate-key - Generate encryption key"
	@echo "  help         - Show this help message"
