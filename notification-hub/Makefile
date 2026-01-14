.PHONY: build run test clean docker-build docker-push lint fmt tidy

# Variables
APP_NAME := notification-hub
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DOCKER_REGISTRY := us-west1-docker.pkg.dev/tesserix-devtest/tesserix
IMAGE := $(DOCKER_REGISTRY)/$(APP_NAME)

# Go settings
GO := go
GOFLAGS := -v

# Build the application
build:
	$(GO) build $(GOFLAGS) -o bin/$(APP_NAME) ./cmd/main.go

# Run locally
run:
	$(GO) run ./cmd/main.go

# Run tests
test:
	$(GO) test -v ./...

# Run tests with coverage
test-coverage:
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Build Docker image
docker-build:
	docker build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

# Push Docker image
docker-push:
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

# Build and push
docker-all: docker-build docker-push

# Lint
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GO) fmt ./...

# Tidy modules
tidy:
	$(GO) mod tidy

# Download dependencies
deps:
	$(GO) mod download

# Verify dependencies
verify:
	$(GO) mod verify

# Generate (if needed)
generate:
	$(GO) generate ./...

# Run all checks
check: fmt lint test

# Development mode with hot reload (requires air)
dev:
	air

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  run           - Run locally"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  clean         - Clean build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-push   - Push Docker image"
	@echo "  docker-all    - Build and push Docker image"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  tidy          - Tidy modules"
	@echo "  deps          - Download dependencies"
	@echo "  verify        - Verify dependencies"
	@echo "  check         - Run all checks (fmt, lint, test)"
	@echo "  dev           - Run in development mode with hot reload"
