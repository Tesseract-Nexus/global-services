# =============================================================================
# Database Backup Tools - Dockerfile
# Pre-built image with PostgreSQL client and Google Cloud SDK for GCS backups
# =============================================================================

FROM google/cloud-sdk:alpine

# Install PostgreSQL 16 client and useful utilities
RUN apk add --no-cache \
    postgresql16-client \
    bash \
    curl \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/*

# Verify installations
RUN pg_dump --version && \
    psql --version && \
    gcloud --version && \
    gsutil --version

# Create non-root user for running backups
RUN addgroup -g 1000 -S backup && \
    adduser -D -u 1000 -S backup -G backup

# Create working directories
RUN mkdir -p /backup /tmp/backups && \
    chown -R backup:backup /backup /tmp/backups

# Set working directory
WORKDIR /backup

# Default to root for package operations, but scripts can switch to backup user
# The CronJob will run as root to ensure GCS authentication works with Workload Identity

# Labels
LABEL org.opencontainers.image.title="db-backup-tools" \
      org.opencontainers.image.description="PostgreSQL backup tools with Google Cloud SDK for GCS" \
      org.opencontainers.image.vendor="Tesseract Nexus" \
      org.opencontainers.image.source="https://github.com/Tesseract-Nexus/global-services"

# Default command - show versions
CMD ["sh", "-c", "echo 'PostgreSQL:' && pg_dump --version && echo '' && echo 'Google Cloud SDK:' && gcloud --version"]
