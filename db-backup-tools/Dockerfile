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

# Create working directories
RUN mkdir -p /backup /tmp/backups

# Set working directory
WORKDIR /backup

# Labels
LABEL org.opencontainers.image.title="db-backup-tools" \
      org.opencontainers.image.description="PostgreSQL backup tools with Google Cloud SDK for GCS" \
      org.opencontainers.image.vendor="Tesseract Nexus" \
      org.opencontainers.image.source="https://github.com/Tesseract-Nexus/global-services"

# Default command - show versions
CMD ["sh", "-c", "echo 'PostgreSQL:' && pg_dump --version && echo '' && echo 'Google Cloud SDK:' && gcloud --version"]
