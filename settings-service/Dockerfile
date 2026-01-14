# =============================================================================
# Settings Service - Dockerfile
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

# Copy service source
COPY services/settings-service/ ./services/settings-service/

# Set working directory to service
WORKDIR /build/services/settings-service

# Download dependencies
RUN go mod download

# Build the application with optimization flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo -o settings-service ./cmd/main.go

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
COPY --from=builder /build/services/settings-service/settings-service .

# Copy migrations if they exist
COPY --from=builder /build/services/settings-service/migrations ./migrations/

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8085

# Health check (uses /livez for simple liveness, /readyz checks DB connectivity)
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8085/livez || exit 1

# Labels
LABEL org.label-schema.name="settings-service" \
      org.label-schema.description="Tesseract Hub Settings Service" \
      org.label-schema.vcs-url="https://github.com/tesseract-hub/tesseract-hub"

# Run the service
CMD ["./settings-service"]
