package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tesseract-nexus/tesseract-hub/services/notification-service/internal/models"
	"gorm.io/gorm"
)

// TemplateRepository handles template database operations
type TemplateRepository interface {
	Create(ctx context.Context, template *models.NotificationTemplate) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.NotificationTemplate, error)
	GetByName(ctx context.Context, tenantID, name string) (*models.NotificationTemplate, error)
	List(ctx context.Context, tenantID string, filters TemplateFilters) ([]models.NotificationTemplate, int64, error)
	Update(ctx context.Context, template *models.NotificationTemplate) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetSystemTemplates(ctx context.Context) ([]models.NotificationTemplate, error)
}

// TemplateFilters for listing templates
type TemplateFilters struct {
	Channel  string
	Category string
	IsActive *bool
	IsSystem *bool
	Search   string
	Limit    int
	Offset   int
}

type templateRepository struct {
	db *gorm.DB
}

// NewTemplateRepository creates a new template repository
func NewTemplateRepository(db *gorm.DB) TemplateRepository {
	return &templateRepository{db: db}
}

func (r *templateRepository) Create(ctx context.Context, template *models.NotificationTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

func (r *templateRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.NotificationTemplate, error) {
	var template models.NotificationTemplate
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&template).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &template, nil
}

func (r *templateRepository) GetByName(ctx context.Context, tenantID, name string) (*models.NotificationTemplate, error) {
	var template models.NotificationTemplate
	err := r.db.WithContext(ctx).
		Where("(tenant_id = ? OR tenant_id = 'default-tenant') AND name = ? AND is_active = true", tenantID, name).
		Order(gorm.Expr("CASE WHEN tenant_id = ? THEN 0 ELSE 1 END", tenantID)). // Prefer tenant-specific templates
		First(&template).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &template, nil
}

func (r *templateRepository) List(ctx context.Context, tenantID string, filters TemplateFilters) ([]models.NotificationTemplate, int64, error) {
	var templates []models.NotificationTemplate
	var total int64

	query := r.db.WithContext(ctx).Where("tenant_id = ? OR tenant_id = 'default-tenant'", tenantID)

	if filters.Channel != "" {
		query = query.Where("channel = ?", filters.Channel)
	}
	if filters.Category != "" {
		query = query.Where("category = ?", filters.Category)
	}
	if filters.IsActive != nil {
		query = query.Where("is_active = ?", *filters.IsActive)
	}
	if filters.IsSystem != nil {
		query = query.Where("is_system = ?", *filters.IsSystem)
	}
	if filters.Search != "" {
		query = query.Where("name ILIKE ? OR description ILIKE ?", "%"+filters.Search+"%", "%"+filters.Search+"%")
	}

	// Count total
	if err := query.Model(&models.NotificationTemplate{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}

	err := query.Order("category, name").
		Limit(filters.Limit).
		Offset(filters.Offset).
		Find(&templates).Error

	return templates, total, err
}

func (r *templateRepository) Update(ctx context.Context, template *models.NotificationTemplate) error {
	return r.db.WithContext(ctx).Save(template).Error
}

func (r *templateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Check if system template
	var template models.NotificationTemplate
	if err := r.db.WithContext(ctx).First(&template, id).Error; err != nil {
		return err
	}
	if template.IsSystem {
		return gorm.ErrRecordNotFound // Can't delete system templates
	}
	return r.db.WithContext(ctx).Delete(&models.NotificationTemplate{}, id).Error
}

func (r *templateRepository) GetSystemTemplates(ctx context.Context) ([]models.NotificationTemplate, error) {
	var templates []models.NotificationTemplate
	err := r.db.WithContext(ctx).
		Where("is_system = true AND is_active = true").
		Order("category, name").
		Find(&templates).Error
	return templates, err
}
