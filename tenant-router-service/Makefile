# =============================================================================
# Tenant Router Service - Makefile
# =============================================================================

# Variables
SERVICE_NAME := tenant-router-service
DOCKER_REGISTRY := us-west1-docker.pkg.dev/tesserix/tesserix-hub
VERSION ?= latest

# Go variables
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags="-w -s"

.PHONY: all build run test clean docker-build docker-push deps tidy fmt lint

# Default target
all: build

# Download dependencies
deps:
	$(GO) mod download

# Tidy dependencies
tidy:
	$(GO) mod tidy

# Format code
fmt:
	$(GO) fmt ./...

# Lint code
lint:
	golangci-lint run ./...

# Build the service
build: deps
	CGO_ENABLED=0 $(GO) build $(LDFLAGS) -o bin/$(SERVICE_NAME) ./cmd

# Run the service locally
run:
	$(GO) run ./cmd

# Run tests
test:
	$(GO) test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/
	$(GO) clean

# Build Docker image
docker-build:
	docker build -t $(DOCKER_REGISTRY)/$(SERVICE_NAME):$(VERSION) \
		-f Dockerfile \
		../..

# Push Docker image
docker-push: docker-build
	docker push $(DOCKER_REGISTRY)/$(SERVICE_NAME):$(VERSION)

# Build and push with latest tag
docker-release: docker-push
	docker tag $(DOCKER_REGISTRY)/$(SERVICE_NAME):$(VERSION) $(DOCKER_REGISTRY)/$(SERVICE_NAME):latest
	docker push $(DOCKER_REGISTRY)/$(SERVICE_NAME):latest

# Local development with hot reload (requires air)
dev:
	air -c .air.toml

# Generate go.sum if needed
generate-sum:
	$(GO) mod tidy
	$(GO) mod download

# Help
help:
	@echo "Available targets:"
	@echo "  all          - Build the service (default)"
	@echo "  build        - Build the service binary"
	@echo "  run          - Run the service locally"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  tidy         - Tidy dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-push  - Push Docker image"
	@echo "  docker-release - Build, push and tag as latest"
	@echo "  dev          - Run with hot reload (requires air)"
	@echo "  help         - Show this help"
