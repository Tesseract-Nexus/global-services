.PHONY: build run dev test clean migrate

# Build the application
build:
	go build -o bin/verification-service cmd/main.go

# Run the application
run: build
	./bin/verification-service

# Run in development mode with live reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Run the service
start:
	go run cmd/main.go

# Generate a random encryption key
generate-key:
	@openssl rand -base64 32 | tr -d '\n' | head -c 32; echo
