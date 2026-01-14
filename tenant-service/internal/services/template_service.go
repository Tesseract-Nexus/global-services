package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"tenant-service/internal/models"
	"tenant-service/internal/repository"
)

// TemplateService handles onboarding template business logic
type TemplateService struct {
	templateRepo *repository.TemplateRepository
}

// NewTemplateService creates a new template service
func NewTemplateService(templateRepo *repository.TemplateRepository) *TemplateService {
	return &TemplateService{
		templateRepo: templateRepo,
	}
}

// CreateTemplate creates a new onboarding template
func (s *TemplateService) CreateTemplate(ctx context.Context, template *models.OnboardingTemplate) (*models.OnboardingTemplate, error) {
	// Validate template
	if template.Name == "" {
		return nil, fmt.Errorf("template name is required")
	}
	if template.ApplicationType == "" {
		return nil, fmt.Errorf("application type is required")
	}

	return s.templateRepo.CreateTemplate(ctx, template)
}

// GetTemplate retrieves a template by ID
func (s *TemplateService) GetTemplate(ctx context.Context, id uuid.UUID) (*models.OnboardingTemplate, error) {
	return s.templateRepo.GetTemplateByID(ctx, id)
}

// GetTemplatesByApplicationType retrieves templates for a specific application type
func (s *TemplateService) GetTemplatesByApplicationType(ctx context.Context, applicationType string) ([]models.OnboardingTemplate, error) {
	return s.templateRepo.GetTemplatesByApplicationType(ctx, applicationType)
}

// GetDefaultTemplate retrieves the default template for an application type
func (s *TemplateService) GetDefaultTemplate(ctx context.Context, applicationType string) (*models.OnboardingTemplate, error) {
	return s.templateRepo.GetDefaultTemplate(ctx, applicationType)
}

// GetActiveTemplates retrieves all active templates
func (s *TemplateService) GetActiveTemplates(ctx context.Context) ([]models.OnboardingTemplate, error) {
	return s.templateRepo.GetActiveTemplates(ctx)
}

// UpdateTemplate updates an existing template
func (s *TemplateService) UpdateTemplate(ctx context.Context, template *models.OnboardingTemplate) (*models.OnboardingTemplate, error) {
	// Validate template exists
	existing, err := s.templateRepo.GetTemplateByID(ctx, template.ID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	// Update fields
	existing.Name = template.Name
	existing.Description = template.Description
	existing.ApplicationType = template.ApplicationType
	existing.TemplateConfig = template.TemplateConfig
	existing.IsActive = template.IsActive
	existing.IsDefault = template.IsDefault

	return s.templateRepo.UpdateTemplate(ctx, existing)
}

// SetDefaultTemplate sets a template as the default for its application type
func (s *TemplateService) SetDefaultTemplate(ctx context.Context, id uuid.UUID) error {
	return s.templateRepo.SetDefaultTemplate(ctx, id)
}

// DeleteTemplate deletes a template
func (s *TemplateService) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	return s.templateRepo.DeleteTemplate(ctx, id)
}

// ListTemplates lists templates with pagination and filtering
func (s *TemplateService) ListTemplates(ctx context.Context, page, pageSize int, filters map[string]interface{}) ([]models.OnboardingTemplate, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	return s.templateRepo.ListTemplates(ctx, page, pageSize, filters)
}

// ValidateTemplateConfiguration validates template configuration
func (s *TemplateService) ValidateTemplateConfiguration(ctx context.Context, config map[string]interface{}) error {
	// Add template configuration validation logic here
	// For now, we'll do basic validation
	if config == nil {
		return fmt.Errorf("template configuration cannot be empty")
	}

	// Validate required fields
	if _, ok := config["steps"]; !ok {
		return fmt.Errorf("template configuration must include 'steps'")
	}

	return nil
}
