-- Migration: Add entity tracking fields and optimize indexes for multi-product support
-- This migration adds columns for better entity tracking and composite indexes for performance

-- Add entity tracking columns for better association with business entities
ALTER TABLE documents ADD COLUMN IF NOT EXISTS entity_type VARCHAR(50);
ALTER TABLE documents ADD COLUMN IF NOT EXISTS entity_id VARCHAR(255);
ALTER TABLE documents ADD COLUMN IF NOT EXISTS media_type VARCHAR(50);
ALTER TABLE documents ADD COLUMN IF NOT EXISTS position INTEGER DEFAULT 0;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS product_id VARCHAR(50);

-- Create composite index for multi-tenant + bucket queries (common pattern)
CREATE INDEX IF NOT EXISTS idx_documents_bucket_tenant
ON documents(bucket, tenant_id)
WHERE deleted_at IS NULL;

-- Create composite index for entity lookups (e.g., get all images for a product)
CREATE INDEX IF NOT EXISTS idx_documents_entity
ON documents(entity_type, entity_id)
WHERE deleted_at IS NULL;

-- Create composite index for product-based bucket access validation
CREATE INDEX IF NOT EXISTS idx_documents_product_bucket
ON documents(product_id, bucket)
WHERE deleted_at IS NULL;

-- Create index for path prefix searches (LIKE queries with leading wildcard don't use B-tree)
-- This uses text_pattern_ops for efficient prefix matching
CREATE INDEX IF NOT EXISTS idx_documents_path_prefix
ON documents(path text_pattern_ops)
WHERE deleted_at IS NULL;

-- Create composite index for listing by tenant + bucket + path prefix
CREATE INDEX IF NOT EXISTS idx_documents_tenant_bucket_path
ON documents(tenant_id, bucket, path)
WHERE deleted_at IS NULL;

-- Add comments for new columns
COMMENT ON COLUMN documents.entity_type IS 'Type of business entity (product, category, vendor, etc.)';
COMMENT ON COLUMN documents.entity_id IS 'ID of the associated business entity';
COMMENT ON COLUMN documents.media_type IS 'Type of media (primary, gallery, icon, banner, etc.)';
COMMENT ON COLUMN documents.position IS 'Display order position for galleries';
COMMENT ON COLUMN documents.product_id IS 'Product identifier for bucket access control (marketplace, bookkeeping, etc.)';
