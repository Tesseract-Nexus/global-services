package repository

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"settings-service/internal/models"
)

// TenantRepository handles tenant data access for audit configuration
type TenantRepository struct {
	db *gorm.DB
}

// NewTenantRepository creates a new tenant repository
func NewTenantRepository(db *gorm.DB) *TenantRepository {
	return &TenantRepository{db: db}
}

// GetByID retrieves a tenant by ID
func (r *TenantRepository) GetByID(id uuid.UUID) (*models.Tenant, error) {
	var tenant models.Tenant
	result := r.db.First(&tenant, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &tenant, nil
}

// GetAllActive retrieves all active tenants
func (r *TenantRepository) GetAllActive() ([]models.Tenant, error) {
	var tenants []models.Tenant
	result := r.db.Where("status = ?", "active").Find(&tenants)
	if result.Error != nil {
		return nil, result.Error
	}
	return tenants, nil
}

// GetBySlug retrieves a tenant by slug
func (r *TenantRepository) GetBySlug(slug string) (*models.Tenant, error) {
	var tenant models.Tenant
	result := r.db.First(&tenant, "slug = ?", slug)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &tenant, nil
}
