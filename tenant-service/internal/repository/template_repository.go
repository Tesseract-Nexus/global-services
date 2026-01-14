package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tesseract-hub/domains/common/services/tenant-service/internal/models"
	"gorm.io/gorm"
)

// TemplateRepository handles onboarding template operations
type TemplateRepository struct {
	db *gorm.DB
}

// NewTemplateRepository creates a new template repository
func NewTemplateRepository(db *gorm.DB) *TemplateRepository {
	return &TemplateRepository{
		db: db,
	}
}

// CreateTemplate creates a new onboarding template
func (r *TemplateRepository) CreateTemplate(ctx context.Context, template *models.OnboardingTemplate) (*models.OnboardingTemplate, error) {
	if template.ID == uuid.Nil {
		template.ID = uuid.New()
	}

	if err := r.db.WithContext(ctx).Create(template).Error; err != nil {
		return nil, fmt.Errorf("failed to create onboarding template: %w", err)
	}

	return template, nil
}

// GetTemplateByID retrieves a template by ID
func (r *TemplateRepository) GetTemplateByID(ctx context.Context, id uuid.UUID) (*models.OnboardingTemplate, error) {
	var template models.OnboardingTemplate

	if err := r.db.WithContext(ctx).First(&template, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("onboarding template not found")
		}
		return nil, fmt.Errorf("failed to get onboarding template: %w", err)
	}

	return &template, nil
}

// GetActiveTemplates retrieves all active templates
func (r *TemplateRepository) GetActiveTemplates(ctx context.Context) ([]models.OnboardingTemplate, error) {
	var templates []models.OnboardingTemplate

	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("failed to get active templates: %w", err)
	}

	return templates, nil
}

// GetTemplatesByApplicationType retrieves templates by application type
func (r *TemplateRepository) GetTemplatesByApplicationType(ctx context.Context, applicationType string) ([]models.OnboardingTemplate, error) {
	var templates []models.OnboardingTemplate

	if err := r.db.WithContext(ctx).Where("application_type = ? AND is_active = ?", applicationType, true).Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("failed to get templates by application type: %w", err)
	}

	return templates, nil
}

// GetDefaultTemplate retrieves the default template for an application type
func (r *TemplateRepository) GetDefaultTemplate(ctx context.Context, applicationType string) (*models.OnboardingTemplate, error) {
	var template models.OnboardingTemplate

	if err := r.db.WithContext(ctx).Where("application_type = ? AND is_default = ? AND is_active = ?",
		applicationType, true, true).First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("default template not found for application type: %s", applicationType)
		}
		return nil, fmt.Errorf("failed to get default template: %w", err)
	}

	return &template, nil
}

// UpdateTemplate updates an onboarding template
func (r *TemplateRepository) UpdateTemplate(ctx context.Context, template *models.OnboardingTemplate) (*models.OnboardingTemplate, error) {
	if err := r.db.WithContext(ctx).Save(template).Error; err != nil {
		return nil, fmt.Errorf("failed to update onboarding template: %w", err)
	}

	return template, nil
}

// DeleteTemplate deletes an onboarding template
func (r *TemplateRepository) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&models.OnboardingTemplate{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete onboarding template: %w", err)
	}

	return nil
}

// ListTemplates lists templates with pagination and filters
func (r *TemplateRepository) ListTemplates(ctx context.Context, page, pageSize int, filters map[string]interface{}) ([]models.OnboardingTemplate, int64, error) {
	var templates []models.OnboardingTemplate
	var total int64

	query := r.db.WithContext(ctx).Model(&models.OnboardingTemplate{})

	// Apply filters
	for field, value := range filters {
		switch field {
		case "application_type":
			query = query.Where("application_type = ?", value)
		case "is_active":
			query = query.Where("is_active = ?", value)
		case "is_default":
			query = query.Where("is_default = ?", value)
		}
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count templates: %w", err)
	}

	// Apply pagination
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&templates).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list templates: %w", err)
	}

	return templates, total, nil
}

// SetDefaultTemplate sets a template as default for its application type
func (r *TemplateRepository) SetDefaultTemplate(ctx context.Context, id uuid.UUID) error {
	// First get the template to know its application type
	template, err := r.GetTemplateByID(ctx, id)
	if err != nil {
		return err
	}

	// Start transaction
	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Unset all default templates for this application type
	if err := tx.Model(&models.OnboardingTemplate{}).
		Where("application_type = ? AND is_default = ?", template.ApplicationType, true).
		Update("is_default", false).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to unset existing default templates: %w", err)
	}

	// Set the new default template
	if err := tx.Model(&models.OnboardingTemplate{}).
		Where("id = ?", id).
		Update("is_default", true).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to set default template: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
