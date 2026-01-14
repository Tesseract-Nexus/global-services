-- Create documents table
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL CHECK (size > 0),
    path TEXT NOT NULL,
    bucket VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL CHECK (provider IN ('aws', 'azure', 'gcp')),
    checksum VARCHAR(64),
    tags JSONB DEFAULT '{}',
    is_public BOOLEAN DEFAULT FALSE,
    url TEXT,
    content_encoding VARCHAR(100),
    cache_control VARCHAR(255),
    tenant_id VARCHAR(255),
    user_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_documents_path_bucket ON documents(path, bucket) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_tenant_id ON documents(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_user_id ON documents(user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_provider ON documents(provider) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_mime_type ON documents(mime_type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_bucket ON documents(bucket) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_is_public ON documents(is_public) WHERE deleted_at IS NULL;

-- Create GIN index for JSONB tags
CREATE INDEX IF NOT EXISTS idx_documents_tags ON documents USING GIN(tags) WHERE deleted_at IS NULL;

-- Create unique constraint for path + bucket combination
CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_path_bucket_unique 
ON documents(path, bucket) 
WHERE deleted_at IS NULL;

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_documents_updated_at 
    BEFORE UPDATE ON documents 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Add comments for documentation
COMMENT ON TABLE documents IS 'Stores metadata for documents in cloud storage';
COMMENT ON COLUMN documents.id IS 'Unique identifier for the document';
COMMENT ON COLUMN documents.filename IS 'Sanitized filename for storage';
COMMENT ON COLUMN documents.original_name IS 'Original filename as uploaded';
COMMENT ON COLUMN documents.mime_type IS 'MIME type of the document';
COMMENT ON COLUMN documents.size IS 'File size in bytes';
COMMENT ON COLUMN documents.path IS 'Storage path/key in cloud provider';
COMMENT ON COLUMN documents.bucket IS 'Storage bucket/container name';
COMMENT ON COLUMN documents.provider IS 'Cloud storage provider (aws, azure, gcp)';
COMMENT ON COLUMN documents.checksum IS 'MD5 checksum for integrity verification';
COMMENT ON COLUMN documents.tags IS 'Custom metadata tags as JSON';
COMMENT ON COLUMN documents.is_public IS 'Whether document is publicly accessible';
COMMENT ON COLUMN documents.url IS 'Public URL if document is public';
COMMENT ON COLUMN documents.content_encoding IS 'Content encoding (e.g., gzip)';
COMMENT ON COLUMN documents.cache_control IS 'Cache control header value';
COMMENT ON COLUMN documents.tenant_id IS 'Tenant identifier for multi-tenancy';
COMMENT ON COLUMN documents.user_id IS 'User who uploaded the document';
COMMENT ON COLUMN documents.created_at IS 'When the document was created';
COMMENT ON COLUMN documents.updated_at IS 'When the document was last updated';
COMMENT ON COLUMN documents.deleted_at IS 'When the document was soft deleted';