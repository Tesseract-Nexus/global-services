# =============================================================================
# Secret Provisioner Service - Dockerfile
# Multi-stage build for Go microservice
# =============================================================================

# Stage 1: Builder
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for runtime
RUN addgroup -g 1000 -S appgroup && adduser -D -u 1000 -S appuser -G appgroup

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with optimization flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo -o secret-provisioner ./cmd/main.go

# Stage 2: Production
FROM alpine:3.19 AS production

# Install ca-certificates and wget for health checks
RUN apk --no-cache add ca-certificates wget tzdata

# Create non-root user
RUN addgroup -g 1000 -S appgroup && adduser -D -u 1000 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/secret-provisioner .

# Copy migrations
COPY --from=builder /build/migrations ./migrations/

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Labels
LABEL org.opencontainers.image.title="secret-provisioner" \
      org.opencontainers.image.description="Secret Provisioner Service for Tesseract Hub" \
      org.opencontainers.image.vendor="Tesseract Nexus" \
      org.opencontainers.image.source="https://github.com/Tesseract-Nexus/global-services"

# Run the service
CMD ["./secret-provisioner"]
