# =============================================================================
# settings-service - Dockerfile
# =============================================================================

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo -o settings-service ./cmd/main.go

FROM alpine:3.19 AS production

RUN apk --no-cache add ca-certificates wget tzdata

RUN addgroup -g 1000 -S appgroup && adduser -D -u 1000 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /build/settings-service .

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./settings-service"]
