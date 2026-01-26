package repository

import (
	"context"
	"fmt"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SecretMetadataRepository provides database operations for secret metadata
type SecretMetadataRepository interface {
	Create(ctx context.Context, meta *models.SecretMetadata) error
	Update(ctx context.Context, meta *models.SecretMetadata) error
	Upsert(ctx context.Context, meta *models.SecretMetadata) error
	Delete(ctx context.Context, gcpSecretName string) error
	GetByGCPName(ctx context.Context, gcpSecretName string) (*models.SecretMetadata, error)
	GetByTenantAndProvider(ctx context.Context, tenantID string, category models.SecretCategory, provider string) ([]*models.SecretMetadata, error)
	ListByTenant(ctx context.Context, tenantID string, category *models.SecretCategory) ([]*models.SecretMetadata, error)
	ListProviders(ctx context.Context, tenantID string, category models.SecretCategory) ([]ProviderStatusResult, error)
	IsConfigured(ctx context.Context, tenantID string, category models.SecretCategory, scope models.SecretScope, scopeID *string, provider string) (bool, error)
}

// ProviderStatusResult holds provider configuration status from DB queries
type ProviderStatusResult struct {
	Provider         string
	Scope            models.SecretScope
	ScopeID          *string
	ConfiguredCount  int64
}

// secretMetadataRepository implements SecretMetadataRepository
type secretMetadataRepository struct {
	db *gorm.DB
}

// NewSecretMetadataRepository creates a new repository instance
func NewSecretMetadataRepository(db *gorm.DB) SecretMetadataRepository {
	return &secretMetadataRepository{db: db}
}

// Create creates a new secret metadata record
func (r *secretMetadataRepository) Create(ctx context.Context, meta *models.SecretMetadata) error {
	return r.db.WithContext(ctx).Create(meta).Error
}

// Update updates an existing secret metadata record
func (r *secretMetadataRepository) Update(ctx context.Context, meta *models.SecretMetadata) error {
	return r.db.WithContext(ctx).Save(meta).Error
}

// Upsert creates or updates a secret metadata record
func (r *secretMetadataRepository) Upsert(ctx context.Context, meta *models.SecretMetadata) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "gcp_secret_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"gcp_secret_version",
			"configured",
			"validation_status",
			"validation_message",
			"last_updated_by",
			"updated_at",
		}),
	}).Create(meta).Error
}

// Delete removes a secret metadata record
func (r *secretMetadataRepository) Delete(ctx context.Context, gcpSecretName string) error {
	return r.db.WithContext(ctx).
		Where("gcp_secret_name = ?", gcpSecretName).
		Delete(&models.SecretMetadata{}).Error
}

// GetByGCPName retrieves a secret metadata by GCP secret name
func (r *secretMetadataRepository) GetByGCPName(ctx context.Context, gcpSecretName string) (*models.SecretMetadata, error) {
	var meta models.SecretMetadata
	err := r.db.WithContext(ctx).
		Where("gcp_secret_name = ?", gcpSecretName).
		First(&meta).Error
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// GetByTenantAndProvider retrieves all secrets for a tenant and provider
func (r *secretMetadataRepository) GetByTenantAndProvider(ctx context.Context, tenantID string, category models.SecretCategory, provider string) ([]*models.SecretMetadata, error) {
	var metas []*models.SecretMetadata
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND category = ? AND provider = ?", tenantID, category, provider).
		Find(&metas).Error
	if err != nil {
		return nil, err
	}
	return metas, nil
}

// ListByTenant retrieves all secrets for a tenant, optionally filtered by category
func (r *secretMetadataRepository) ListByTenant(ctx context.Context, tenantID string, category *models.SecretCategory) ([]*models.SecretMetadata, error) {
	var metas []*models.SecretMetadata
	query := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)
	if category != nil {
		query = query.Where("category = ?", *category)
	}
	err := query.Find(&metas).Error
	if err != nil {
		return nil, err
	}
	return metas, nil
}

// ListProviders returns provider configuration status for a tenant and category
func (r *secretMetadataRepository) ListProviders(ctx context.Context, tenantID string, category models.SecretCategory) ([]ProviderStatusResult, error) {
	var results []ProviderStatusResult

	err := r.db.WithContext(ctx).
		Model(&models.SecretMetadata{}).
		Select("provider, scope, scope_id, COUNT(*) as configured_count").
		Where("tenant_id = ? AND category = ? AND configured = ?", tenantID, category, true).
		Group("provider, scope, scope_id").
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}

	return results, nil
}

// IsConfigured checks if a provider is configured for the given scope
func (r *secretMetadataRepository) IsConfigured(ctx context.Context, tenantID string, category models.SecretCategory, scope models.SecretScope, scopeID *string, provider string) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).
		Model(&models.SecretMetadata{}).
		Where("tenant_id = ? AND category = ? AND scope = ? AND provider = ? AND configured = ?",
			tenantID, category, scope, provider, true)

	if scopeID != nil {
		query = query.Where("scope_id = ?", *scopeID)
	} else {
		query = query.Where("scope_id IS NULL")
	}

	err := query.Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
