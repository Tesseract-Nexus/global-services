# =============================================================================
# Document Service - Dockerfile
# Multi-stage build for Go microservice
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Builder
# -----------------------------------------------------------------------------
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for runtime
RUN addgroup -g 1000 -S appgroup && adduser -D -u 1000 -S appuser -G appgroup

# Set working directory
WORKDIR /build

# Copy go-shared package (required by replace directive in go.mod)
# The go.mod uses: replace github.com/tesseract-hub/go-shared => ../../packages/go-shared
COPY packages/go-shared/ ./packages/go-shared/

# Copy go.mod and go.sum first for better layer caching
COPY services/document-service/go.mod services/document-service/go.sum ./services/document-service/

# Download dependencies
WORKDIR /build/services/document-service
RUN go mod download

# Copy service source
WORKDIR /build
COPY packages/go-shared/ ./packages/go-shared/
COPY services/document-service/ ./services/document-service/

# Build the application with optimization flags
WORKDIR /build/services/document-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo -o document-service ./cmd/main.go

# -----------------------------------------------------------------------------
# Stage 2: Production
# -----------------------------------------------------------------------------
FROM alpine:3.19 AS production

# Install ca-certificates and wget for health checks
RUN apk --no-cache add ca-certificates wget tzdata

# Create non-root user
RUN addgroup -g 1000 -S appgroup && adduser -D -u 1000 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/services/document-service/document-service .

# Create necessary directories
RUN mkdir -p /var/log/document-service && \
    chown -R appuser:appgroup /var/log/document-service

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
LABEL org.label-schema.name="document-service" \
      org.label-schema.description="Tesseract Hub Document Service" \
      org.label-schema.vcs-url="https://github.com/tesseract-hub/tesseract-hub"

# Run the service
CMD ["./document-service"]
