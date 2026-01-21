package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/clients"
	"tenant-service/internal/models"
	natsClient "tenant-service/internal/nats"
	"tenant-service/internal/repository"
	"github.com/Tesseract-Nexus/go-shared/auth"
	"github.com/Tesseract-Nexus/go-shared/secrets"
	"github.com/Tesseract-Nexus/go-shared/security"
	"gorm.io/gorm"
)

// retryConfig holds configuration for retry operations
type retryConfig struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

// defaultRetryConfig returns standard retry settings for critical operations
func defaultRetryConfig() retryConfig {
	return retryConfig{
		maxAttempts: 3,
		baseDelay:   500 * time.Millisecond,
		maxDelay:    5 * time.Second,
	}
}

// retryWithBackoff executes an operation with exponential backoff retry
// Returns the result and final error after all retries are exhausted
func retryWithBackoff[T any](ctx context.Context, cfg retryConfig, operation string, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		// Log retry attempt
		log.Printf("[OnboardingService] %s failed (attempt %d/%d): %v", operation, attempt, cfg.maxAttempts, lastErr)

		// Don't sleep after the last attempt
		if attempt < cfg.maxAttempts {
			// Calculate exponential backoff with jitter
			delay := time.Duration(float64(cfg.baseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > cfg.maxDelay {
				delay = cfg.maxDelay
			}

			select {
			case <-ctx.Done():
				return result, fmt.Errorf("%s cancelled: %w", operation, ctx.Err())
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	return result, fmt.Errorf("%s failed after %d attempts: %w", operation, cfg.maxAttempts, lastErr)
}

// OnboardingService handles onboarding business logic
type OnboardingService struct {
	onboardingRepo       *repository.OnboardingRepository
	taskRepo             *repository.TaskRepository
	businessRepo         *repository.BusinessInformationRepository
	contactRepo          *repository.ContactInformationRepository
	credentialRepo       *repository.CredentialRepository
	verificationSvc      *VerificationService
	paymentSvc           *PaymentService
	notificationSvc      *NotificationService
	membershipSvc        *MembershipService
	vendorClient         *clients.VendorClient
	staffClient          *clients.StaffClient
	tenantRouterClient   *clients.TenantRouterClient
	customDomainClient   *clients.CustomDomainClient
	natsClient           *natsClient.Client
	keycloakClient       *auth.KeycloakAdminClient
	keycloakConfig       *KeycloakOnboardingConfig
	db                   *gorm.DB
}

// KeycloakOnboardingConfig holds Keycloak configuration for onboarding
type KeycloakOnboardingConfig struct {
	ClientID         string // Public client ID for password grant (e.g., "tesserix-onboarding")
	ClientSecret     string // Client secret (empty for public clients)
	DefaultRole      string // Default role to assign (e.g., "store_owner")
	AdminClientID    string // Client ID for admin portal redirect URIs (e.g., "marketplace-dashboard")
	BaseDomain       string // Base domain for redirect URIs (e.g., "tesserix.app")
}

// NewOnboardingService creates a new onboarding service
func NewOnboardingService(
	onboardingRepo *repository.OnboardingRepository,
	taskRepo *repository.TaskRepository,
	verificationSvc *VerificationService,
	paymentSvc *PaymentService,
	membershipSvc *MembershipService,
	nc *natsClient.Client,
	db *gorm.DB,
) *OnboardingService {
	// Initialize vendor client for creating vendors when tenants are created
	vendorServiceURL := os.Getenv("VENDOR_SERVICE_URL")
	if vendorServiceURL == "" {
		vendorServiceURL = "http://localhost:8085" // Default for local development
	}
	vendorClient := clients.NewVendorClient(vendorServiceURL)

	// Initialize staff client for bootstrapping owner RBAC roles
	staffServiceURL := os.Getenv("STAFF_SERVICE_URL")
	if staffServiceURL == "" {
		staffServiceURL = "http://staff-service.devtest.svc.cluster.local:8082" // Default for k8s
	}
	staffClient := clients.NewStaffClient(staffServiceURL)

	// Initialize tenant-router client for direct VS provisioning (fallback for NATS)
	tenantRouterServiceURL := os.Getenv("TENANT_ROUTER_SERVICE_URL")
	if tenantRouterServiceURL == "" {
		tenantRouterServiceURL = "http://tenant-router-service.marketplace.svc.cluster.local:8089"
	}
	tenantRouterClient := clients.NewTenantRouterClient(tenantRouterServiceURL)

	// Initialize custom-domain-service client for custom domain provisioning
	customDomainServiceURL := os.Getenv("CUSTOM_DOMAIN_SERVICE_URL")
	if customDomainServiceURL == "" {
		customDomainServiceURL = "http://custom-domain-service.marketplace.svc.cluster.local:8093"
	}
	customDomainClient := clients.NewCustomDomainClient(customDomainServiceURL)

	// Initialize Keycloak admin client for user registration
	keycloakClient, keycloakConfig := initKeycloakClient()

	return &OnboardingService{
		onboardingRepo:       onboardingRepo,
		taskRepo:             taskRepo,
		businessRepo:         repository.NewBusinessInformationRepository(db),
		contactRepo:          repository.NewContactInformationRepository(db),
		credentialRepo:       repository.NewCredentialRepository(db),
		verificationSvc:      verificationSvc,
		paymentSvc:           paymentSvc,
		notificationSvc:      NewNotificationService(),
		membershipSvc:        membershipSvc,
		vendorClient:         vendorClient,
		staffClient:          staffClient,
		tenantRouterClient:   tenantRouterClient,
		customDomainClient:   customDomainClient,
		natsClient:           nc,
		keycloakClient:       keycloakClient,
		keycloakConfig:       keycloakConfig,
		db:                   db,
	}
}

// initKeycloakClient initializes the Keycloak admin client from environment variables
func initKeycloakClient() (*auth.KeycloakAdminClient, *KeycloakOnboardingConfig) {
	// Keycloak Admin API configuration
	keycloakBaseURL := os.Getenv("KEYCLOAK_BASE_URL")
	if keycloakBaseURL == "" {
		keycloakBaseURL = "https://devtest-internal-idp.tesserix.app"
	}

	keycloakRealm := os.Getenv("KEYCLOAK_REALM")
	if keycloakRealm == "" {
		keycloakRealm = "tesserix-internal"
	}

	// Admin client credentials (service account)
	adminClientID := os.Getenv("KEYCLOAK_ADMIN_CLIENT_ID")
	if adminClientID == "" {
		adminClientID = "admin-cli"
	}

	// Try to get admin client secret from GCP Secret Manager, fall back to env var
	adminClientSecret := secrets.GetSecretOrEnv("KEYCLOAK_ADMIN_CLIENT_SECRET_NAME", "KEYCLOAK_ADMIN_CLIENT_SECRET", "")

	// Create admin client
	var keycloakClient *auth.KeycloakAdminClient
	if adminClientSecret != "" {
		keycloakClient = auth.NewKeycloakAdminClient(auth.KeycloakAdminConfig{
			BaseURL:      keycloakBaseURL,
			Realm:        keycloakRealm,
			ClientID:     adminClientID,
			ClientSecret: adminClientSecret,
			Timeout:      30 * time.Second,
		})
		log.Printf("Keycloak admin client initialized for realm: %s", keycloakRealm)
	} else {
		log.Printf("Warning: KEYCLOAK_ADMIN_CLIENT_SECRET not set - user registration will fail")
	}

	// Public client configuration for password grant (auto-login after registration)
	publicClientID := os.Getenv("KEYCLOAK_PUBLIC_CLIENT_ID")
	if publicClientID == "" {
		publicClientID = "tesserix-onboarding"
	}

	// Get client secret from GCP Secret Manager (via KEYCLOAK_CLIENT_SECRET_NAME) or fall back to env var
	// This enables auto-login with confidential clients like marketplace-dashboard
	publicClientSecret := secrets.GetSecretOrEnv("KEYCLOAK_CLIENT_SECRET_NAME", "KEYCLOAK_PUBLIC_CLIENT_SECRET", "")
	if publicClientSecret != "" {
		log.Printf("Keycloak public client secret loaded for auto-login")
	} else {
		log.Printf("Warning: No Keycloak public client secret - auto-login after registration will fail")
	}

	defaultRole := os.Getenv("KEYCLOAK_DEFAULT_ROLE")
	if defaultRole == "" {
		defaultRole = "store_owner"
	}

	// Admin portal client ID for registering tenant-specific redirect URIs
	adminPortalClientID := os.Getenv("KEYCLOAK_ADMIN_PORTAL_CLIENT_ID")
	if adminPortalClientID == "" {
		adminPortalClientID = "marketplace-dashboard"
	}

	// Base domain for constructing redirect URIs
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}

	config := &KeycloakOnboardingConfig{
		ClientID:      publicClientID,
		ClientSecret:  publicClientSecret,
		DefaultRole:   defaultRole,
		AdminClientID: adminPortalClientID,
		BaseDomain:    baseDomain,
	}

	return keycloakClient, config
}

// StartOnboardingRequest represents a request to start onboarding
type StartOnboardingRequest struct {
	TemplateID      uuid.UUID              `json:"template_id" validate:"required"`
	ApplicationType string                 `json:"application_type" validate:"required,oneof=ecommerce saas marketplace b2b"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// StartOnboarding creates a new onboarding session
func (s *OnboardingService) StartOnboarding(ctx context.Context, req *StartOnboardingRequest) (*models.OnboardingSession, error) {
	// Create onboarding session
	metadata, _ := models.NewJSONB(req.Metadata)
	now := time.Now()
	expiresAt := now.Add(7 * 24 * time.Hour) // 7 days

	session := &models.OnboardingSession{
		ID:                 uuid.New(),
		TemplateID:         req.TemplateID,
		ApplicationType:    req.ApplicationType,
		Status:             "started",
		CurrentStep:        "business_information",
		ProgressPercentage: 0,
		ExpiresAt:          expiresAt,
		DraftExpiresAt:     &expiresAt, // Draft expires same as session
		DraftSavedAt:       &now,       // Mark as saved to enable draft recovery
		Metadata:           metadata,
	}

	// Create the session
	createdSession, err := s.onboardingRepo.CreateSession(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to create onboarding session: %w", err)
	}

	// Initialize tasks for the session (this would be based on the template)
	if err := s.initializeSessionTasks(ctx, createdSession.ID); err != nil {
		return nil, fmt.Errorf("failed to initialize session tasks: %w", err)
	}

	return createdSession, nil
}

// GetOnboardingSession retrieves an onboarding session with optional related data
func (s *OnboardingService) GetOnboardingSession(ctx context.Context, sessionID uuid.UUID, includeRelations []string) (*models.OnboardingSession, error) {
	return s.onboardingRepo.GetSessionByID(ctx, sessionID, includeRelations)
}

// SaveApplicationConfiguration saves or updates an application configuration for a session
func (s *OnboardingService) SaveApplicationConfiguration(ctx context.Context, sessionID uuid.UUID, config *models.ApplicationConfiguration) (*models.ApplicationConfiguration, error) {
	// Verify session exists and is active
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	if session.Status == "completed" || session.Status == "failed" || session.Status == "abandoned" {
		return nil, fmt.Errorf("cannot update configuration for session in %s status", session.Status)
	}

	config.OnboardingSessionID = sessionID

	// Check if configuration already exists for this type
	var existing models.ApplicationConfiguration
	err = s.db.WithContext(ctx).
		Where("onboarding_session_id = ? AND application_type = ?", sessionID, config.ApplicationType).
		First(&existing).Error

	if err == nil {
		// Update existing
		existing.ConfigurationData = config.ConfigurationData
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return nil, fmt.Errorf("failed to update application configuration: %w", err)
		}
		return &existing, nil
	}

	// Create new
	if err := s.db.WithContext(ctx).Create(config).Error; err != nil {
		return nil, fmt.Errorf("failed to create application configuration: %w", err)
	}

	return config, nil
}

// UpdateBusinessInformation updates business information for a session
func (s *OnboardingService) UpdateBusinessInformation(ctx context.Context, sessionID uuid.UUID, businessInfo *models.BusinessInformation) (*models.BusinessInformation, error) {
	// Verify session exists and is active
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	if session.Status == "completed" || session.Status == "failed" || session.Status == "abandoned" {
		return nil, fmt.Errorf("cannot update business information for session in %s status", session.Status)
	}

	// Validate business name uniqueness before saving
	// This ensures no two tenants can have the same business name
	if businessInfo.BusinessName != "" {
		validationResult, err := s.ValidateBusinessNameWithSuggestions(ctx, businessInfo.BusinessName, &sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to validate business name: %w", err)
		}
		if !validationResult.Available {
			// Return validation error with suggestions
			return nil, NewValidationError(
				"business_name",
				fmt.Sprintf("'%s' is already taken", businessInfo.BusinessName),
				validationResult.Suggestions,
			)
		}
	}

	// Save business information
	savedBusinessInfo, err := s.businessRepo.CreateOrUpdate(ctx, sessionID, businessInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to save business information: %w", err)
	}

	// Mark the business_info task as completed
	if err := s.completeTaskByID(ctx, sessionID, "business_info"); err != nil {
		log.Printf("Warning: failed to complete business_info task: %v", err)
	}

	// Update session progress
	if err := s.updateSessionProgress(ctx, sessionID, "contact_information", 25); err != nil {
		return nil, fmt.Errorf("failed to update session progress: %w", err)
	}

	return savedBusinessInfo, nil
}

// UpdateContactInformation adds contact information for a session
func (s *OnboardingService) UpdateContactInformation(ctx context.Context, sessionID uuid.UUID, contact *models.ContactInformation) (*models.ContactInformation, error) {
	// Verify session exists and is active
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	if session.Status == "completed" || session.Status == "failed" || session.Status == "abandoned" {
		return nil, fmt.Errorf("cannot update contact information for session in %s status", session.Status)
	}

	// Save contact information
	savedContact, err := s.contactRepo.CreateContact(ctx, sessionID, contact)
	if err != nil {
		return nil, fmt.Errorf("failed to save contact information: %w", err)
	}

	// Mark the contact_info task as completed
	if err := s.completeTaskByID(ctx, sessionID, "contact_info"); err != nil {
		log.Printf("Warning: failed to complete contact_info task: %v", err)
	}

	// Update session progress
	if err := s.updateSessionProgress(ctx, sessionID, "business_address", 50); err != nil {
		return nil, fmt.Errorf("failed to update session progress: %w", err)
	}

	return savedContact, nil
}

// UpdateBusinessAddress adds business address for a session
func (s *OnboardingService) UpdateBusinessAddress(ctx context.Context, sessionID uuid.UUID, address *models.BusinessAddress) (*models.BusinessAddress, error) {
	// Verify session exists and is active
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	if session.Status == "completed" || session.Status == "failed" || session.Status == "abandoned" {
		return nil, fmt.Errorf("cannot update business address for session in %s status", session.Status)
	}

	// Create the address
	address.OnboardingSessionID = sessionID
	if address.ID == uuid.Nil {
		address.ID = uuid.New()
	}

	// Save to database
	if err := s.db.WithContext(ctx).Create(address).Error; err != nil {
		return nil, fmt.Errorf("failed to save business address: %w", err)
	}

	// Mark the business_address task as completed
	if err := s.completeTaskByID(ctx, sessionID, "business_address"); err != nil {
		log.Printf("Warning: failed to complete business_address task: %v", err)
	}

	// Update session progress
	if err := s.updateSessionProgress(ctx, sessionID, "verification", 75); err != nil {
		return nil, fmt.Errorf("failed to update session progress: %w", err)
	}

	return address, nil
}

// CompleteOnboarding marks an onboarding session as completed
func (s *OnboardingService) CompleteOnboarding(ctx context.Context, sessionID uuid.UUID) (*models.OnboardingSession, error) {
	// Get session
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"tasks"})
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Check if all required tasks are completed
	incompleteTasks, err := s.taskRepo.GetRequiredIncompleteTasks(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to check incomplete tasks: %w", err)
	}

	if len(incompleteTasks) > 0 {
		return nil, fmt.Errorf("cannot complete onboarding: %d required tasks are incomplete", len(incompleteTasks))
	}

	// Update session status
	now := time.Now()
	session.Status = "completed"
	session.CompletedAt = &now
	session.ProgressPercentage = 100

	updatedSession, err := s.onboardingRepo.UpdateSession(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	// Trigger post-completion tasks (webhooks, notifications, etc.)
	go s.handlePostCompletion(context.Background(), sessionID)

	return updatedSession, nil
}

// GetProgress retrieves the progress of an onboarding session
func (s *OnboardingService) GetProgress(ctx context.Context, sessionID uuid.UUID) (map[string]interface{}, error) {
	// Get session
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Get task progress
	taskProgress, err := s.taskRepo.GetTaskProgress(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task progress: %w", err)
	}

	return map[string]interface{}{
		"session_id":          session.ID,
		"status":              session.Status,
		"current_step":        session.CurrentStep,
		"progress_percentage": session.ProgressPercentage,
		"started_at":          session.StartedAt,
		"completed_at":        session.CompletedAt,
		"expires_at":          session.ExpiresAt,
		"task_progress":       taskProgress,
	}, nil
}

// GetTasks retrieves tasks for a session
func (s *OnboardingService) GetTasks(ctx context.Context, sessionID uuid.UUID) ([]models.OnboardingTask, error) {
	return s.taskRepo.GetTasksBySession(ctx, sessionID)
}

// UpdateTaskStatus updates the status of a specific task
func (s *OnboardingService) UpdateTaskStatus(ctx context.Context, sessionID, taskID uuid.UUID, status string, completionData map[string]interface{}) error {
	// Verify task belongs to session
	task, err := s.taskRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	if task.OnboardingSessionID != sessionID {
		return fmt.Errorf("task does not belong to this session")
	}

	// Update task status
	if err := s.taskRepo.UpdateTaskStatus(ctx, taskID, status, completionData); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Log the task execution
	details, _ := models.NewJSONB(completionData)
	log := &models.TaskExecutionLog{
		OnboardingTaskID: taskID,
		Action:           status,
		Details:          details,
		PerformedBy:      "system", // This could be extracted from context
	}

	if _, err := s.taskRepo.CreateTaskExecutionLog(ctx, log); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("failed to create task execution log: %v", err)
	}

	return nil
}

// ValidateSubdomain validates if a subdomain/slug is available
func (s *OnboardingService) ValidateSubdomain(ctx context.Context, subdomain string) (bool, error) {
	if s.membershipSvc == nil {
		// Fallback if membership service not initialized
		return true, nil
	}
	return s.membershipSvc.IsSlugAvailable(ctx, subdomain)
}

// ValidateSlugWithSuggestions validates a slug and returns suggestions if taken
// If sessionID is provided, it excludes that session's own reservation from the check
func (s *OnboardingService) ValidateSlugWithSuggestions(ctx context.Context, slug string, sessionID *uuid.UUID) (*repository.SlugValidationResult, error) {
	if s.membershipSvc == nil {
		// Fallback if membership service not initialized
		return &repository.SlugValidationResult{
			Slug:      slug,
			Available: true,
			Message:   "Validation service unavailable",
		}, nil
	}
	return s.membershipSvc.ValidateSlugWithSuggestions(ctx, slug, sessionID)
}

// ValidateBusinessName validates if a business name is available
func (s *OnboardingService) ValidateBusinessName(ctx context.Context, businessName string) (bool, error) {
	if s.membershipSvc == nil {
		// Fallback if membership service not initialized
		return true, nil
	}
	return s.membershipSvc.IsBusinessNameAvailable(ctx, businessName)
}

// ValidateBusinessNameWithSuggestions validates a business name and returns suggestions if taken
// If sessionID is provided, it excludes that session's own business information from the check
func (s *OnboardingService) ValidateBusinessNameWithSuggestions(ctx context.Context, businessName string, sessionID *uuid.UUID) (*repository.BusinessNameValidationResult, error) {
	if s.membershipSvc == nil {
		// Fallback if membership service not initialized
		return &repository.BusinessNameValidationResult{
			BusinessName: businessName,
			Available:    true,
			Message:      "Validation service unavailable",
		}, nil
	}
	return s.membershipSvc.ValidateBusinessNameWithSuggestions(ctx, businessName, sessionID)
}

// ValidateAndReserveSlug validates a slug and reserves it for the session if available
// This prevents race conditions where two users try to claim the same slug
// Also saves the optional storefrontSlug for the customer-facing store URL
func (s *OnboardingService) ValidateAndReserveSlug(ctx context.Context, slug string, sessionID uuid.UUID, storefrontSlug string) (*repository.SlugValidationResult, error) {
	if s.membershipSvc == nil {
		// Fallback if membership service not initialized
		return &repository.SlugValidationResult{
			Slug:      slug,
			Available: true,
			Message:   "Validation service unavailable",
		}, nil
	}

	// Get the session to use email as reservedBy identifier
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"contact_information", "business_information"})
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Get reservedBy - use primary contact email if available
	reservedBy := sessionID.String()
	if len(session.ContactInformation) > 0 {
		for _, contact := range session.ContactInformation {
			if contact.IsPrimaryContact && contact.Email != "" {
				reservedBy = contact.Email
				break
			}
		}
	}

	// Validate and reserve the slug
	result, err := s.membershipSvc.ValidateAndReserveSlug(ctx, slug, sessionID, reservedBy)
	if err != nil {
		return nil, err
	}

	// If slug is available and reserved, store it in the session's business information
	if result.Available && session.BusinessInformation != nil {
		session.BusinessInformation.TenantSlug = result.Slug
		// Save storefront slug (defaults to admin slug if not provided)
		if storefrontSlug != "" {
			session.BusinessInformation.StorefrontSlug = storefrontSlug
		} else {
			session.BusinessInformation.StorefrontSlug = result.Slug
		}
		if _, err := s.businessRepo.CreateOrUpdate(ctx, sessionID, session.BusinessInformation); err != nil {
			log.Printf("[OnboardingService] Warning: Failed to update business info with slug: %v", err)
			// Don't fail the request, the slug is reserved
		} else {
			log.Printf("[OnboardingService] Stored reserved slug '%s' and storefront slug '%s' for session %s", result.Slug, session.BusinessInformation.StorefrontSlug, sessionID)
		}
	}

	return result, nil
}

// UpdateStorefrontSlug updates the storefront slug in the session's business information
// This is called when a user validates a storefront slug with session_id
func (s *OnboardingService) UpdateStorefrontSlug(ctx context.Context, sessionID uuid.UUID, storefrontSlug string) error {
	// Get the session with business information (use lowercase key to match switch case)
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"business_information"})
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Create or update business information with the storefront slug
	if session.BusinessInformation == nil {
		session.BusinessInformation = &models.BusinessInformation{
			OnboardingSessionID: sessionID,
		}
	}
	session.BusinessInformation.StorefrontSlug = storefrontSlug

	// Save the updated business information
	if _, err := s.businessRepo.CreateOrUpdate(ctx, sessionID, session.BusinessInformation); err != nil {
		return fmt.Errorf("failed to update business info with storefront slug: %w", err)
	}

	log.Printf("[OnboardingService] Updated storefront slug to '%s' for session %s", storefrontSlug, sessionID)
	return nil
}

// Private helper methods

// initializeSessionTasks creates initial tasks for a session based on template
func (s *OnboardingService) initializeSessionTasks(ctx context.Context, sessionID uuid.UUID) error {
	// This would typically read from the template and create appropriate tasks
	// For now, we'll create some default tasks
	defaultTasks := []models.OnboardingTask{
		{
			OnboardingSessionID:   sessionID,
			TaskID:                "business_info",
			Name:                  "Complete Business Information",
			Description:           "Provide basic business details",
			TaskType:              "form",
			Status:                "pending",
			IsRequired:            true,
			OrderIndex:            1,
			EstimatedDurationMins: 10,
		},
		{
			OnboardingSessionID:   sessionID,
			TaskID:                "contact_info",
			Name:                  "Add Contact Information",
			Description:           "Provide primary contact details",
			TaskType:              "form",
			Status:                "pending",
			IsRequired:            true,
			OrderIndex:            2,
			EstimatedDurationMins: 5,
		},
		{
			OnboardingSessionID:   sessionID,
			TaskID:                "business_address",
			Name:                  "Add Business Address",
			Description:           "Provide business address details",
			TaskType:              "form",
			Status:                "pending",
			IsRequired:            true,
			OrderIndex:            3,
			EstimatedDurationMins: 5,
		},
		{
			OnboardingSessionID:   sessionID,
			TaskID:                "email_verification",
			Name:                  "Verify Email Address",
			Description:           "Verify your email address",
			TaskType:              "verification",
			Status:                "pending",
			IsRequired:            true,
			OrderIndex:            4,
			EstimatedDurationMins: 2,
		},
	}

	_, err := s.taskRepo.CreateTasksBatch(ctx, defaultTasks)
	return err
}

// updateSessionProgress updates the progress of a session
func (s *OnboardingService) updateSessionProgress(ctx context.Context, sessionID uuid.UUID, currentStep string, progressPercentage int) error {
	return s.onboardingRepo.UpdateSessionProgress(ctx, sessionID, currentStep, progressPercentage)
}

// CompleteTask marks a specific task as completed by its task_id (public API)
// This is used by handlers to mark tasks complete after successful operations
func (s *OnboardingService) CompleteTask(ctx context.Context, sessionID uuid.UUID, taskID string) error {
	if err := s.completeTaskByID(ctx, sessionID, taskID); err != nil {
		return fmt.Errorf("failed to complete task %s: %w", taskID, err)
	}

	// Recalculate and update session progress
	tasks, err := s.taskRepo.GetTasksBySession(ctx, sessionID)
	if err != nil {
		return nil // Task completed, progress update is best-effort
	}

	completedCount := 0
	for _, t := range tasks {
		if t.Status == "completed" {
			completedCount++
		}
	}

	progress := 0
	if len(tasks) > 0 {
		progress = (completedCount * 100) / len(tasks)
	}

	// Update session progress
	s.updateSessionProgress(ctx, sessionID, "", progress)

	return nil
}

// completeTaskByID marks a task as completed by its task_id
func (s *OnboardingService) completeTaskByID(ctx context.Context, sessionID uuid.UUID, taskID string) error {
	// Get the task by session ID and task ID
	task, err := s.taskRepo.GetTaskBySessionAndTaskID(ctx, sessionID, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Update task status to completed
	now := time.Now()
	task.Status = "completed"
	task.CompletedAt = &now

	if err := s.taskRepo.UpdateTask(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	return nil
}

// CompleteEmailVerificationTask marks the email verification task as completed
// and also marks the session as completed with 100% progress
func (s *OnboardingService) CompleteEmailVerificationTask(ctx context.Context, sessionID uuid.UUID) error {
	// First, mark the email verification task as completed
	if err := s.completeTaskByID(ctx, sessionID, "email_verification"); err != nil {
		log.Printf("Warning: failed to complete email_verification task: %v", err)
		// Continue anyway - the task might not exist or already be completed
	}

	// Store verification status in Redis so SetupAccount can verify the email was verified
	// This is critical for the account-setup step to succeed
	if s.verificationSvc != nil {
		// Get the session to retrieve the email
		sessionForEmail, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"contact_information"})
		if err == nil && len(sessionForEmail.ContactInformation) > 0 && sessionForEmail.ContactInformation[0].Email != "" {
			email := sessionForEmail.ContactInformation[0].Email
			// Save verification status via verificationSvc which has access to Redis
			if err := s.verificationSvc.SaveVerificationStatusToRedis(ctx, email, sessionID.String()); err != nil {
				log.Printf("[OnboardingService] Warning: Failed to save verification status to Redis: %v", err)
			}
		}
	}

	// Also mark all other required tasks as completed (in case they weren't marked during form submission)
	// This ensures backward compatibility and handles edge cases
	for _, taskID := range []string{"business_info", "contact_info", "business_address"} {
		if err := s.completeTaskByID(ctx, sessionID, taskID); err != nil {
			log.Printf("Warning: failed to complete %s task: %v", taskID, err)
			// Continue anyway
		}
	}

	// Update session to completed status with 100% progress
	if err := s.updateSessionProgress(ctx, sessionID, "completed", 100); err != nil {
		return fmt.Errorf("failed to update session to completed: %w", err)
	}

	// Mark session as completed
	now := time.Now()
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, nil)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	session.Status = "completed"
	session.CompletedAt = &now
	session.ProgressPercentage = 100

	if _, err := s.onboardingRepo.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	log.Printf("Session %s marked as completed after email verification", sessionID)

	// Publish session completed event for document migration
	// This allows documents to be migrated from onboarding to tenant storage
	go func() {
		if s.natsClient != nil {
			// Get full session data for the event
			fullSession, err := s.onboardingRepo.GetSessionByID(context.Background(), sessionID, []string{"business_information", "contact_information"})
			if err != nil {
				log.Printf("[OnboardingService] Failed to get session for event: %v", err)
				return
			}

			var email string
			if len(fullSession.ContactInformation) > 0 {
				email = fullSession.ContactInformation[0].Email
			}

			var businessName string
			if fullSession.BusinessInformation != nil {
				businessName = fullSession.BusinessInformation.BusinessName
			}

			event := &natsClient.SessionCompletedEvent{
				SessionID:    sessionID.String(),
				Product:      fullSession.ApplicationType,
				BusinessName: businessName,
				Email:        email,
			}
			if err := s.natsClient.PublishSessionCompleted(context.Background(), event); err != nil {
				log.Printf("[OnboardingService] Failed to publish session.completed event: %v", err)
			} else {
				log.Printf("[OnboardingService] Published session.completed event for session %s", sessionID)
			}
		}
	}()

	return nil
}

// handlePostCompletion handles post-completion tasks
func (s *OnboardingService) handlePostCompletion(ctx context.Context, sessionID uuid.UUID) {
	// This would trigger webhooks, send notifications, etc.
	// For now, we'll just log
	fmt.Printf("Handling post-completion for session: %s\n", sessionID)
}

// CompleteAccountSetupResponse represents the response after account setup
type CompleteAccountSetupResponse struct {
	TenantID     uuid.UUID `json:"tenant_id"`
	TenantSlug   string    `json:"tenant_slug"`
	UserID       uuid.UUID `json:"user_id"`
	Email        string    `json:"email"`
	BusinessName string    `json:"business_name"`
	AdminURL     string    `json:"admin_url"`
	Token        string    `json:"token,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	Message      string    `json:"message"`
}

// KeycloakSetupRequest contains all data needed for Keycloak user setup
type KeycloakSetupRequest struct {
	Email      string
	Password   string
	FirstName  string
	LastName   string
	TenantID   string
	TenantSlug string
	Role       string // e.g., "store_owner"
}

// KeycloakSetupResult represents the complete result of Keycloak user setup
type KeycloakSetupResult struct {
	UserID       string
	Email        string
	RoleAssigned bool
	Verified     bool // True only if we've verified login works
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

// CompleteAccountSetup creates tenant and user account from onboarding session
func (s *OnboardingService) CompleteAccountSetup(ctx context.Context, sessionID uuid.UUID, password, authMethod, timezone, currency, businessModel string) (*CompleteAccountSetupResponse, error) {
	// Get onboarding session with all related data including application_configurations
	// which contains store setup data (currency, timezone) saved during onboarding
	session, err := s.onboardingRepo.GetSessionByID(ctx, sessionID, []string{"business_information", "contact_information", "business_addresses", "application_configurations"})
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Extract currency/timezone from application_configurations if not provided in request
	// This ensures user selections from onboarding are preserved
	if timezone == "" || currency == "" {
		for _, config := range session.ApplicationConfigurations {
			if config.ApplicationType == "store_setup" {
				// Parse the JSONB configuration data (JSONB is json.RawMessage)
				var configData map[string]interface{}
				if err := json.Unmarshal(config.ConfigurationData, &configData); err == nil {
					if timezone == "" {
						if tz, exists := configData["timezone"]; exists {
							if tzStr, ok := tz.(string); ok && tzStr != "" {
								timezone = tzStr
							}
						}
					}
					if currency == "" {
						if curr, exists := configData["currency"]; exists {
							if currStr, ok := curr.(string); ok && currStr != "" {
								currency = currStr
							}
						}
					}
				}
				break
			}
		}
	}

	// Set defaults only if still not available from request or application_configurations
	if timezone == "" {
		timezone = "UTC"
	}
	if currency == "" {
		currency = "USD"
	}

	// Validate session is ready for account setup (completed OR at verification step with 75%+ progress)
	// This allows users to set up their account after completing business info, contact, and address
	if session.Status != "completed" && session.ProgressPercentage < 75 {
		return nil, fmt.Errorf("onboarding session is not ready for account setup (progress: %d%%)", session.ProgressPercentage)
	}

	// ============================================================================
	// COMPREHENSIVE DATA VALIDATION
	// Validate all required data exists before proceeding with account setup
	// This prevents partial state creation and cryptic errors downstream
	// ============================================================================

	// Validate business information exists
	if session.BusinessInformation == nil {
		return nil, fmt.Errorf("business information is required - please complete the business details step")
	}
	if session.BusinessInformation.BusinessName == "" {
		return nil, fmt.Errorf("business name is required - please complete the business details step")
	}

	// Validate contact information exists and has email
	if len(session.ContactInformation) == 0 {
		return nil, fmt.Errorf("contact information is required - please complete the contact details step")
	}
	primaryContact := session.ContactInformation[0]
	if primaryContact.Email == "" {
		return nil, fmt.Errorf("email address is required - please complete the contact details step")
	}
	if primaryContact.FirstName == "" || primaryContact.LastName == "" {
		return nil, fmt.Errorf("first and last name are required - please complete the contact details step")
	}

	// Validate email is verified (check verification status)
	if s.verificationSvc != nil {
		isVerified, verifyErr := s.verificationSvc.IsEmailVerifiedByRecipient(ctx, primaryContact.Email, "email_verification")
		if verifyErr != nil {
			log.Printf("[OnboardingService] Warning: Could not check email verification status: %v", verifyErr)
			// Don't fail - verification service might be unavailable
		} else if !isVerified {
			return nil, fmt.Errorf("email address must be verified before setting up your account")
		}
	}

	// Validate password requirements
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}
	if len(password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}

	// Check if tenant already created for this session
	if session.TenantID != nil && *session.TenantID != uuid.Nil {
		// Tenant already exists - use unified Keycloak setup with verification
		// This handles re-running account setup after initial creation
		tenantID := *session.TenantID

		// Find the tenant
		var tenant models.Tenant
		if err := s.db.First(&tenant, "id = ?", tenantID).Error; err != nil {
			return nil, fmt.Errorf("failed to find existing tenant: %w", err)
		}

		// Find the user by email (users are now global, not tied to tenant)
		var user models.User
		if err := s.db.Where("email = ?", primaryContact.Email).First(&user).Error; err != nil {
			return nil, fmt.Errorf("failed to find user for existing tenant: %w", err)
		}

		// Use unified Keycloak setup with verification (P0 FIX)
		// This ensures: password is set, role is assigned, and login is verified
		keycloakResult, keycloakErr := s.setupUserInKeycloak(ctx, &KeycloakSetupRequest{
			Email:      primaryContact.Email,
			Password:   password,
			FirstName:  primaryContact.FirstName,
			LastName:   primaryContact.LastName,
			TenantID:   tenantID.String(),
			TenantSlug: tenant.Slug,
			Role:       s.keycloakConfig.DefaultRole, // "store_owner" - NOW INCLUDED FOR EXISTING USERS
		})
		if keycloakErr != nil {
			log.Printf("[OnboardingService] CRITICAL: Failed to setup user in Keycloak: %v", keycloakErr)
			return nil, fmt.Errorf("failed to set up authentication: %w", keycloakErr)
		}

		// Generate admin URL (subdomain-based routing)
		// URL pattern: https://{slug}-admin.{baseDomain}
		baseDomain := os.Getenv("BASE_DOMAIN")
		if baseDomain == "" {
			baseDomain = "tesserix.app"
		}
		adminURL := fmt.Sprintf("https://%s-admin.%s", tenant.Slug, baseDomain)

		// Tokens are already obtained and verified by setupUserInKeycloak
		return &CompleteAccountSetupResponse{
			TenantID:     tenantID,
			TenantSlug:   tenant.Slug,
			UserID:       user.ID,
			Email:        primaryContact.Email,
			BusinessName: session.BusinessInformation.BusinessName,
			AdminURL:     adminURL,
			AccessToken:  keycloakResult.AccessToken,
			RefreshToken: keycloakResult.RefreshToken,
			ExpiresIn:    keycloakResult.ExpiresIn,
			Message:      "Account already set up. User logged in successfully.",
		}, nil
	}

	// Begin transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Use reserved slug if available, otherwise generate a new one
	var slug string
	if session.BusinessInformation.TenantSlug != "" {
		// Use the pre-reserved slug from onboarding
		slug = session.BusinessInformation.TenantSlug
		log.Printf("[OnboardingService] Using reserved slug '%s' for tenant creation", slug)

		// Verify it's still available (double-check to catch any edge cases)
		var slugCount int64
		tx.WithContext(ctx).Model(&models.Tenant{}).Where("slug = ?", slug).Count(&slugCount)
		if slugCount > 0 {
			// Reserved slug was somehow taken - this should be rare
			// Fall back to generating a unique variant
			log.Printf("[OnboardingService] Warning: Reserved slug '%s' was taken, generating variant", slug)
			originalSlug := slug
			counter := 1
			for {
				slug = fmt.Sprintf("%s-%d", originalSlug, counter)
				tx.WithContext(ctx).Model(&models.Tenant{}).Where("slug = ?", slug).Count(&slugCount)
				if slugCount == 0 {
					break
				}
				counter++
			}
		}
	} else {
		// No reserved slug - generate from business name (legacy flow)
		log.Printf("[OnboardingService] No reserved slug found, generating from business name")
		slug = generateSlug(session.BusinessInformation.BusinessName)
		// Ensure slug is unique
		var slugCount int64
		counter := 0
		originalSlug := slug
		for {
			tx.WithContext(ctx).Model(&models.Tenant{}).Where("slug = ?", slug).Count(&slugCount)
			if slugCount == 0 {
				break
			}
			counter++
			slug = fmt.Sprintf("%s-%d", originalSlug, counter)
		}
	}

	// Create tenant
	tenantID := uuid.New()
	subdomain := slug // Use slug as subdomain for consistency

	// Get auth user ID if available (passed from frontend after auth)
	var ownerUserID *uuid.UUID
	if authMethod == "existing_user" {
		// TODO: Extract user ID from auth token
		// For now, we'll create a new auth user
	}

	tenant := &models.Tenant{
		ID:              tenantID,
		Name:            session.BusinessInformation.BusinessName,
		Slug:            slug,
		Subdomain:       subdomain,
		DisplayName:     session.BusinessInformation.BusinessName,
		BusinessType:    session.BusinessInformation.BusinessType,
		Industry:        session.BusinessInformation.Industry,
		Status:          "creating", // Start as "creating", update to "active" after vendor/storefront succeed
		Mode:            "development", // Start in development mode
		DefaultTimezone: timezone,      // From frontend store setup
		DefaultCurrency: currency,      // From frontend store setup
		BusinessModel:   businessModel, // ONLINE_STORE or MARKETPLACE
		OwnerUserID:     ownerUserID,
		// Pricing - default to free tier for all new tenants
		// Other tiers are disabled until monetization is enabled
		PricingTier:  models.PricingTierFree,
		BillingEmail: primaryContact.Email, // Use primary contact email for billing
	}

	if err := tx.WithContext(ctx).Create(tenant).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	// ============================================================================
	// KEYCLOAK ORGANIZATION: Create organization for tenant identity isolation
	// This enables users to have separate identities per tenant (store)
	// ============================================================================
	var keycloakOrgID string
	if s.keycloakClient != nil {
		log.Printf("[OnboardingService] Creating Keycloak Organization for tenant %s (slug: %s)...", tenantID, slug)
		orgID, orgErr := s.keycloakClient.CreateOrganizationForTenant(ctx, tenantID.String(), tenant.Name, slug)
		if orgErr != nil {
			// Log error but don't fail onboarding - organization can be created later
			// This ensures backward compatibility during rollout
			log.Printf("[OnboardingService] Warning: Failed to create Keycloak Organization for tenant %s: %v", tenantID, orgErr)
		} else {
			keycloakOrgID = orgID
			log.Printf("[OnboardingService] Created Keycloak Organization %s for tenant %s", orgID, tenantID)

			// Update tenant with organization ID
			orgUUID, parseErr := uuid.Parse(orgID)
			if parseErr == nil {
				if updateErr := tx.WithContext(ctx).Model(&models.Tenant{}).
					Where("id = ?", tenantID).
					Update("keycloak_org_id", orgUUID).Error; updateErr != nil {
					log.Printf("[OnboardingService] Warning: Failed to update tenant with keycloak_org_id: %v", updateErr)
				} else {
					tenant.KeycloakOrgID = &orgUUID
					log.Printf("[OnboardingService] Updated tenant %s with keycloak_org_id: %s", tenantID, orgID)
				}
			}
		}
	}

	// ============================================================================
	// KEYCLOAK REDIRECT URIs: Register tenant-specific redirect URIs
	// This allows OAuth login to work for the new tenant's admin portal and storefront
	// ============================================================================
	if s.keycloakClient != nil && s.keycloakConfig != nil && s.keycloakConfig.AdminClientID != "" {
		baseDomain := s.keycloakConfig.BaseDomain
		if baseDomain == "" {
			baseDomain = "tesserix.app"
		}

		// Build redirect URIs for this tenant
		redirectURIs := []string{
			fmt.Sprintf("https://%s-admin.%s/*", slug, baseDomain),
			fmt.Sprintf("https://%s.%s/*", slug, baseDomain),
		}

		log.Printf("[OnboardingService] Registering Keycloak redirect URIs for tenant %s: %v", tenantID, redirectURIs)
		if err := s.keycloakClient.AddClientRedirectURIs(ctx, s.keycloakConfig.AdminClientID, redirectURIs); err != nil {
			// Log error but don't fail onboarding - redirect URIs can be added manually if needed
			log.Printf("[OnboardingService] Warning: Failed to register Keycloak redirect URIs for tenant %s: %v", tenantID, err)
		} else {
			log.Printf("[OnboardingService] Successfully registered Keycloak redirect URIs for tenant %s", tenantID)
		}
	}

	// ============================================================================
	// USER ID STRATEGY: Keycloak is the source of truth for user IDs
	//
	// Flow:
	// 1. Call Keycloak FIRST to register/get user → get authoritative user ID
	// 2. Create/update tenant_users record with that same ID
	// 3. Create membership with that same ID
	// 4. All systems (Keycloak, tenant, membership) now use the SAME user ID
	// ============================================================================

	// Step 1: Use unified Keycloak setup with verification (P0 FIX)
	// This ensures: user is created, password is set, role is assigned, and login is verified
	// CRITICAL: If this fails, we cannot guarantee the user can log in, so fail the entire onboarding
	log.Printf("[OnboardingService] Setting up user %s in Keycloak with verification...", security.MaskEmail(primaryContact.Email))
	keycloakResult, keycloakErr := s.setupUserInKeycloak(ctx, &KeycloakSetupRequest{
		Email:      primaryContact.Email,
		Password:   password,
		FirstName:  primaryContact.FirstName,
		LastName:   primaryContact.LastName,
		TenantID:   tenantID.String(),
		TenantSlug: slug,
		Role:       s.keycloakConfig.DefaultRole, // "store_owner"
	})

	var userID uuid.UUID
	if keycloakErr != nil {
		// Keycloak setup failed - this is a CRITICAL error, fail onboarding
		// Without Keycloak registration, user won't be able to log in
		tx.Rollback()
		log.Printf("[OnboardingService] CRITICAL: Failed to setup user in Keycloak: %v", keycloakErr)
		return nil, fmt.Errorf("failed to register user: %w", keycloakErr)
	}

	if keycloakResult.UserID == "" {
		// Keycloak returned empty user ID - this is also a CRITICAL error
		tx.Rollback()
		log.Printf("[OnboardingService] CRITICAL: Keycloak returned empty user ID")
		return nil, fmt.Errorf("failed to register user: invalid response from authentication service")
	}

	// Parse the Keycloak user ID - this is our authoritative ID
	parsedID, parseErr := uuid.Parse(keycloakResult.UserID)
	if parseErr != nil {
		tx.Rollback()
		log.Printf("[OnboardingService] CRITICAL: Could not parse Keycloak user ID: %v", parseErr)
		return nil, fmt.Errorf("failed to register user: invalid user ID format")
	}
	userID = parsedID
	log.Printf("[OnboardingService] Got verified user ID from Keycloak: %s (role: %s assigned: %v)", userID, s.keycloakConfig.DefaultRole, keycloakResult.RoleAssigned)

	// Step 2: Create/update tenant_users record with the Keycloak user ID
	// NOTE: Password is stored in Keycloak only - no local password storage

	// Check if user already exists in tenant_users
	// Store the Keycloak ID for reference (auth-bff uses this for session lookup)
	keycloakUserID := userID

	var existingUser models.User
	userExistsInTenantDB := false
	if err := tx.WithContext(ctx).Where("email = ?", primaryContact.Email).First(&existingUser).Error; err == nil {
		userExistsInTenantDB = true
		log.Printf("[OnboardingService] User %s already exists in tenant_users (ID: %s)", security.MaskEmail(primaryContact.Email), existingUser.ID)

		// Use the existing local user ID for consistency with existing data
		// Store the Keycloak ID for auth lookup (used by GetUserTenants)
		if existingUser.ID != userID {
			log.Printf("[OnboardingService] Using existing local user ID %s (Keycloak ID: %s)", existingUser.ID, keycloakUserID)
			userID = existingUser.ID
		}

		// Update the keycloak_id field to enable auth-bff session lookups
		if existingUser.KeycloakID == nil || *existingUser.KeycloakID != keycloakUserID {
			if err := tx.WithContext(ctx).Model(&existingUser).Update("keycloak_id", keycloakUserID).Error; err != nil {
				log.Printf("[OnboardingService] Warning: Failed to update keycloak_id for user %s: %v", security.MaskEmail(primaryContact.Email), err)
				// Non-fatal: continue with membership creation
			} else {
				log.Printf("[OnboardingService] Updated keycloak_id for user %s to %s", security.MaskEmail(primaryContact.Email), keycloakUserID)
			}
		}
	}

	if !userExistsInTenantDB {
		// Create new user in tenant_users with the Keycloak ID
		// Password is stored in Keycloak only - not in local database
		user := &models.User{
			ID:         userID, // Use Keycloak user ID as local ID for new users
			KeycloakID: &keycloakUserID,
			Email:      primaryContact.Email,
			FirstName:  primaryContact.FirstName,
			LastName:   primaryContact.LastName,
			Phone:      primaryContact.Phone,
			Status:     "active",
		}

		if err := tx.WithContext(ctx).Create(user).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
		log.Printf("[OnboardingService] Created new user %s with Keycloak ID: %s", security.MaskEmail(primaryContact.Email), userID)
	}

	// Update onboarding session with tenant ID
	if err := tx.WithContext(ctx).Model(&models.OnboardingSession{}).
		Where("id = ?", sessionID).
		Update("tenant_id", tenantID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update session with tenant ID: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Use userID for all subsequent operations (membership, etc.)
	// This is the local user ID (may differ from keycloakUserID for existing users)

	// Create owner membership for multi-tenant access
	// This links the local user to the tenant with owner role
	// CRITICAL: Without membership, user cannot access the tenant - fail onboarding
	if s.membershipSvc != nil {
		if _, membershipErr := s.membershipSvc.CreateOwnerMembership(ctx, tenantID, userID); membershipErr != nil {
			log.Printf("[OnboardingService] CRITICAL: Failed to create owner membership for user %s in tenant %s: %v", userID, tenantID, membershipErr)
			// Mark tenant as inactive since user won't be able to access it
			s.db.Model(&models.Tenant{}).Where("id = ?", tenantID).Update("status", "inactive")
			return nil, fmt.Errorf("failed to create user access to tenant: %w - please contact support", membershipErr)
		}
		log.Printf("[OnboardingService] Created owner membership for user %s in tenant %s", userID, tenantID)
	} else {
		log.Printf("[OnboardingService] CRITICAL: Membership service not configured - user %s will not have access to tenant %s", userID, tenantID)
		return nil, fmt.Errorf("membership service not configured - cannot complete onboarding")
	}

	// ============================================================================
	// KEYCLOAK ORGANIZATION MEMBERSHIP
	// Add owner to the organization for identity isolation
	// ============================================================================
	if s.keycloakClient != nil && keycloakOrgID != "" {
		// keycloakUserID is the Keycloak user ID (may differ from local userID for existing users)
		log.Printf("[OnboardingService] Adding user %s to Keycloak Organization %s...", keycloakUserID, keycloakOrgID)
		if addErr := s.keycloakClient.AddOrganizationMember(ctx, keycloakOrgID, keycloakUserID.String()); addErr != nil {
			// Log error but don't fail - membership can be added later
			log.Printf("[OnboardingService] Warning: Failed to add user %s to organization %s: %v", keycloakUserID, keycloakOrgID, addErr)
		} else {
			log.Printf("[OnboardingService] Added user %s to Keycloak Organization %s", keycloakUserID, keycloakOrgID)
		}
	}

	// ============================================================================
	// TENANT CREDENTIAL RECORD (WITHOUT PASSWORD)
	// ============================================================================
	// Create tenant credential record for MFA settings, login tracking, session management
	// NOTE: Password is stored ONLY in Keycloak (single source of truth)
	// This simplifies the auth flow and prevents password sync issues
	if s.credentialRepo != nil {
		if _, credErr := s.credentialRepo.CreateCredentialWithoutPassword(ctx, userID, tenantID, &userID); credErr != nil {
			// Log error but don't fail - credential record can be created on first login
			log.Printf("[OnboardingService] Warning: Failed to create tenant credential record for user %s in tenant %s: %v", userID, tenantID, credErr)
		} else {
			log.Printf("[OnboardingService] Created tenant credential record (Keycloak-only password) for user %s in tenant %s", userID, tenantID)
		}

		// Create default auth policy for the tenant
		if _, policyErr := s.credentialRepo.CreateAuthPolicy(ctx, tenantID); policyErr != nil {
			log.Printf("[OnboardingService] Warning: Failed to create auth policy for tenant %s: %v", tenantID, policyErr)
		} else {
			log.Printf("[OnboardingService] Created default auth policy for tenant %s", tenantID)
		}

		// Log the account creation event for audit
		auditLog := &models.TenantAuthAuditLog{
			TenantID:    tenantID,
			UserID:      &userID,
			EventType:   models.AuthEventLoginSuccess, // Account created = first login
			EventStatus: models.AuthEventStatusSuccess,
			Details:     models.MustNewJSONB(map[string]interface{}{"event": "account_created", "source": "onboarding"}),
		}
		if auditErr := s.credentialRepo.LogAuthEvent(ctx, auditLog); auditErr != nil {
			log.Printf("[OnboardingService] Warning: Failed to log auth audit event: %v", auditErr)
		}
	}

	// Bootstrap owner RBAC roles in staff-service
	// This creates the staff record and assigns the Owner role with full permissions
	// CRITICAL: This enables the owner to access the admin panel - MUST succeed
	if s.staffClient == nil {
		// staffClient being nil is a configuration error - owner RBAC is mandatory
		log.Printf("[OnboardingService] CRITICAL: staffClient is nil, cannot bootstrap owner RBAC for tenant %s", tenantID)
		return nil, fmt.Errorf("staff service client not configured - cannot create owner permissions")
	}

	bootstrapCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	bootstrapResult, bootstrapErr := s.staffClient.BootstrapOwner(
		bootstrapCtx,
		tenantID,
		keycloakUserID,
		primaryContact.Email,
		primaryContact.FirstName,
		primaryContact.LastName,
	)
	if bootstrapErr != nil {
		// CRITICAL: Owner RBAC bootstrap failure means owner can't access admin panel
		// This is a fatal error - tenant without owner permissions is unusable
		log.Printf("[OnboardingService] CRITICAL: Failed to bootstrap owner RBAC for tenant %s: %v", tenantID, bootstrapErr)

		// ============================================================================
		// COMPENSATION: Clean up resources created before this failure
		// Since transaction was already committed, we need to manually compensate
		// ============================================================================

		// 1. Mark tenant as failed
		if updateErr := s.db.Model(&models.Tenant{}).
			Where("id = ?", tenantID).
			Updates(map[string]interface{}{
				"status":      "failed",
				"description": fmt.Sprintf("Staff bootstrap failed: %v", bootstrapErr),
			}).Error; updateErr != nil {
			log.Printf("[OnboardingService] Warning: Failed to mark tenant as failed: %v", updateErr)
		}

		// 2. Update session status to reflect the failure
		if updateErr := s.db.Model(&models.OnboardingSession{}).
			Where("id = ?", sessionID).
			Update("status", "failed").Error; updateErr != nil {
			log.Printf("[OnboardingService] Warning: Failed to update session status to failed: %v", updateErr)
		}

		// 3. Clean up the membership that was created
		if s.membershipSvc != nil {
			if cleanupErr := s.membershipSvc.DeleteMembershipInternal(ctx, userID, tenantID); cleanupErr != nil {
				log.Printf("[OnboardingService] Warning: Failed to cleanup membership for user %s in tenant %s: %v", userID, tenantID, cleanupErr)
			} else {
				log.Printf("[OnboardingService] Cleaned up membership for user %s in tenant %s due to bootstrap failure", userID, tenantID)
			}
		}

		// 4. Release the slug reservation back to pending (so user can retry)
		if s.membershipSvc != nil {
			// Note: We don't have a method to release slug, but marking tenant as failed
			// should prevent it from being used. The slug can be reclaimed on retry.
			log.Printf("[OnboardingService] Slug '%s' will remain reserved for retry. Tenant marked as failed.", slug)
		}

		return nil, fmt.Errorf("failed to bootstrap owner permissions: %w", bootstrapErr)
	}
	log.Printf("[OnboardingService] Bootstrapped owner RBAC for user %s in tenant %s", keycloakUserID, tenantID)

	// FIX-P0: Update Keycloak user with staff_id from bootstrap result
	// This is CRITICAL - without staff_id in the JWT, all service calls return 403
	// FIX-P1: Make this mandatory - tenant_id mismatch between Keycloak and staff_db causes RBAC failures
	if s.keycloakClient != nil && bootstrapResult != nil && bootstrapResult.StaffID != "" {
		keycloakAttrs := map[string][]string{
			"staff_id":    {bootstrapResult.StaffID},
			"tenant_id":   {tenantID.String()},
			"tenant_slug": {slug},
		}
		if updateErr := s.keycloakClient.UpdateUserAttributes(ctx, keycloakUserID.String(), keycloakAttrs); updateErr != nil {
			log.Printf("[OnboardingService] CRITICAL: Failed to update Keycloak user %s with staff_id: %v", keycloakUserID, updateErr)

			// FIX-P1: Keycloak update is now MANDATORY to prevent tenant_id mismatch
			// Without this, the JWT will have wrong/no tenant_id, causing all RBAC checks to fail
			// Mark tenant as failed and return error
			if updateErr := s.db.Model(&models.Tenant{}).
				Where("id = ?", tenantID).
				Updates(map[string]interface{}{
					"status":      "failed",
					"description": fmt.Sprintf("Keycloak sync failed: %v", updateErr),
				}).Error; updateErr != nil {
				log.Printf("[OnboardingService] Warning: Failed to mark tenant as failed: %v", updateErr)
			}

			// Update session status
			if updateErr := s.db.Model(&models.OnboardingSession{}).
				Where("id = ?", sessionID).
				Update("status", "failed").Error; updateErr != nil {
				log.Printf("[OnboardingService] Warning: Failed to update session status: %v", updateErr)
			}

			return nil, fmt.Errorf("failed to sync Keycloak user attributes (tenant_id, staff_id): %w - this would cause RBAC permission failures", updateErr)
		}
		log.Printf("[OnboardingService] Updated Keycloak user %s with staff_id=%s, tenant_id=%s, tenant_slug=%s",
			keycloakUserID, bootstrapResult.StaffID, tenantID, slug)
	} else {
		// FIX-P1: Missing keycloakClient or bootstrapResult is a configuration error
		log.Printf("[OnboardingService] CRITICAL: Cannot update Keycloak - keycloakClient=%v, bootstrapResult=%v",
			s.keycloakClient != nil, bootstrapResult != nil)
		return nil, fmt.Errorf("keycloak sync not possible: keycloakClient=%v, bootstrapResult=%v - this would cause RBAC failures",
			s.keycloakClient != nil, bootstrapResult != nil)
	}

	// Update tenant with owner user ID
	if err := s.db.Model(&models.Tenant{}).Where("id = ?", tenantID).Update("owner_user_id", keycloakUserID).Error; err != nil {
		fmt.Printf("Warning: Failed to update tenant owner_user_id: %v\n", err)
	}

	// Activate the slug reservation (convert from pending to active)
	// This permanently claims the slug for this tenant
	if s.membershipSvc != nil {
		if activateErr := s.membershipSvc.ActivateSlugReservation(ctx, slug, tenantID); activateErr != nil {
			log.Printf("Warning: Failed to activate slug reservation for '%s': %v", slug, activateErr)
		} else {
			log.Printf("[OnboardingService] Activated slug reservation '%s' for tenant %s", slug, tenantID)
		}
	}

	// Create default vendor record for the tenant with retry logic
	// The vendor gets its own auto-generated ID and is linked to tenant via Vendor.TenantID
	// Relationship: Tenant (1) ---> (N) Vendors ---> (N) Storefronts
	// CRITICAL: A tenant without a vendor is functionally useless - fail onboarding if this fails
	if s.vendorClient != nil {
		retryCfg := defaultRetryConfig()
		businessName := session.BusinessInformation.BusinessName
		contactName := fmt.Sprintf("%s %s", primaryContact.FirstName, primaryContact.LastName)

		// Retry vendor creation with exponential backoff
		vendorData, vendorErr := retryWithBackoff(ctx, retryCfg, "Vendor creation", func() (*clients.VendorData, error) {
			return s.vendorClient.CreateVendorForTenant(
				ctx,
				tenantID,
				businessName,
				primaryContact.Email,
				contactName,
			)
		})

		if vendorErr != nil {
			// CRITICAL: Vendor creation is essential - fail onboarding
			// Without a vendor, the tenant cannot manage storefronts, products, or orders
			log.Printf("[OnboardingService] CRITICAL: Failed to create vendor for tenant %s after retries: %v", tenantID, vendorErr)

			// Update session status to "failed" to reflect provisioning failure
			if updateErr := s.db.Model(&models.OnboardingSession{}).
				Where("id = ?", sessionID).
				Update("status", "failed").Error; updateErr != nil {
				log.Printf("[OnboardingService] Warning: Failed to update session status to failed: %v", updateErr)
			}

			// Mark tenant as inactive since provisioning failed (tenant exists but is unusable)
			if updateErr := s.db.Model(&models.Tenant{}).
				Where("id = ?", tenantID).
				Update("status", "inactive").Error; updateErr != nil {
				log.Printf("[OnboardingService] Warning: Failed to update tenant status to inactive: %v", updateErr)
			}

			return nil, fmt.Errorf("failed to create vendor for tenant %s: %w - onboarding cannot complete without a vendor", tenantID, vendorErr)
		}

		log.Printf("[OnboardingService] Created vendor %s for tenant %s", vendorData.ID, tenantID)

		// FIX-P0: Update Keycloak user with vendor_id
		// This is required for vendor-scoped operations (inventory, products, etc.)
		if s.keycloakClient != nil && vendorData.ID != uuid.Nil {
			vendorAttrs := map[string][]string{
				"vendor_id": {vendorData.ID.String()},
			}
			if updateErr := s.keycloakClient.UpdateUserAttributes(ctx, keycloakUserID.String(), vendorAttrs); updateErr != nil {
				log.Printf("[OnboardingService] Warning: Failed to update Keycloak user %s with vendor_id: %v", keycloakUserID, updateErr)
			} else {
				log.Printf("[OnboardingService] Updated Keycloak user %s with vendor_id=%s", keycloakUserID, vendorData.ID)
			}
		}

		// CRITICAL: Verify tenant_id consistency
		// This prevents data corruption from race conditions or partial failures
		if vendorData.TenantID != tenantID.String() {
			log.Printf("[OnboardingService] CRITICAL: Vendor tenant_id mismatch! Expected %s, got %s. Failing onboarding.", tenantID.String(), vendorData.TenantID)

			// Update session and tenant status to reflect data integrity failure
			s.db.Model(&models.OnboardingSession{}).Where("id = ?", sessionID).Update("status", "failed")
			s.db.Model(&models.Tenant{}).Where("id = ?", tenantID).Update("status", "inactive")

			return nil, fmt.Errorf("vendor tenant_id mismatch: expected %s, got %s - data integrity issue detected", tenantID.String(), vendorData.TenantID)
		}

		// Create default storefront for the vendor with retry logic
		// Use StorefrontSlug from business info, or fall back to tenant slug
		storefrontSlug := session.BusinessInformation.StorefrontSlug
		if storefrontSlug == "" {
			storefrontSlug = slug // Default to same as admin slug
		}
		storefrontName := businessName + " Store"
		vendorID := vendorData.ID

		// Retry storefront creation with exponential backoff
		storefrontData, storefrontErr := retryWithBackoff(ctx, retryCfg, "Storefront creation", func() (*clients.StorefrontData, error) {
			return s.vendorClient.CreateStorefront(
				ctx,
				tenantID,
				vendorID,
				storefrontName,
				storefrontSlug,
				true, // isDefault = true for the first storefront
			)
		})

		if storefrontErr != nil {
			// CRITICAL: Storefront creation is essential - fail onboarding
			// Without a storefront, customers cannot access the store
			log.Printf("[OnboardingService] CRITICAL: Failed to create storefront for vendor %s after retries: %v", vendorData.ID, storefrontErr)

			// Update session status to "failed" to reflect provisioning failure
			if updateErr := s.db.Model(&models.OnboardingSession{}).
				Where("id = ?", sessionID).
				Update("status", "failed").Error; updateErr != nil {
				log.Printf("[OnboardingService] Warning: Failed to update session status to failed: %v", updateErr)
			}

			// Mark tenant as inactive since provisioning failed (tenant exists but is unusable)
			if updateErr := s.db.Model(&models.Tenant{}).
				Where("id = ?", tenantID).
				Update("status", "inactive").Error; updateErr != nil {
				log.Printf("[OnboardingService] Warning: Failed to update tenant status to inactive: %v", updateErr)
			}

			return nil, fmt.Errorf("failed to create storefront for vendor %s: %w - onboarding cannot complete without a storefront", vendorData.ID, storefrontErr)
		}

		log.Printf("[OnboardingService] Created storefront %s (slug: %s) for vendor %s", storefrontData.ID, storefrontSlug, vendorData.ID)

		// Note: Storefront doesn't have TenantID field - it inherits tenant association
		// through its parent vendor, which we already verified above

		// ============================================================================
		// TENANT ACTIVATION: Mark tenant as active after vendor/storefront succeed
		// ============================================================================
		// Tenant was created with status "creating" to prevent orphaned active tenants
		// Now that vendor and storefront are successfully created, activate the tenant
		if err := s.db.Model(&models.Tenant{}).Where("id = ?", tenantID).Update("status", "active").Error; err != nil {
			log.Printf("[OnboardingService] Warning: Failed to activate tenant %s: %v", tenantID, err)
			// Don't fail onboarding - tenant is usable, just not marked active
		} else {
			log.Printf("[OnboardingService] Activated tenant %s (status: creating -> active)", tenantID)
		}
	} else {
		// No vendor client configured - this is a critical configuration error
		log.Printf("[OnboardingService] CRITICAL: Vendor client not configured - cannot create vendor for tenant %s", tenantID)
		return nil, fmt.Errorf("vendor client not configured - cannot complete onboarding without vendor creation capability")
	}

	// Generate admin URL for the tenant (subdomain-based routing)
	// URL pattern: https://{slug}-admin.{baseDomain}
	// Example: https://mystore-admin.tesserix.app
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}
	adminURL := fmt.Sprintf("https://%s-admin.%s", slug, baseDomain)

	// Register redirect URIs in Keycloak for the admin dashboard only
	// NOTE: Storefront redirect URIs are NOT registered to marketplace-dashboard client
	// because the storefront is public-facing and should not use admin OAuth flow.
	// Customers on the storefront will use a separate authentication flow (customer realm/client)
	// or can browse anonymously.
	if s.keycloakClient != nil {
		// Admin dashboard redirect URIs only - storefront is public and uses separate auth
		adminWildcard := fmt.Sprintf("https://%s-admin.%s/*", slug, baseDomain)
		adminCallback := fmt.Sprintf("https://%s-admin.%s/auth/callback", slug, baseDomain)

		redirectURIs := []string{
			adminWildcard,
			adminCallback,
		}

		redirectCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Add redirect URIs to the marketplace-dashboard client (admin only)
		if err := s.keycloakClient.AddClientRedirectURIs(redirectCtx, "marketplace-dashboard", redirectURIs); err != nil {
			log.Printf("[OnboardingService] Warning: Failed to register redirect URIs for tenant %s: %v", slug, err)
			// Don't fail onboarding - admin can add manually if needed
		} else {
			log.Printf("[OnboardingService] Registered admin Keycloak redirect URIs for tenant %s: %v", slug, redirectURIs)
		}
	}

	// FIX-P0: Refresh token AFTER all Keycloak attributes are set
	// The original token from setupUserInKeycloak was issued BEFORE staff_id, tenant_id, vendor_id
	// were set as user attributes. Without refreshing, the JWT won't have these claims and
	// all backend services will return 401/403 "Failed to verify permissions".
	finalAccessToken := keycloakResult.AccessToken
	finalRefreshToken := keycloakResult.RefreshToken
	finalExpiresIn := keycloakResult.ExpiresIn

	if s.keycloakClient != nil && s.keycloakConfig != nil {
		log.Printf("[OnboardingService] Refreshing token to include updated claims (staff_id, tenant_id, vendor_id)...")
		log.Printf("[OnboardingService] Token refresh config: clientID=%s, hasSecret=%v",
			s.keycloakConfig.ClientID, s.keycloakConfig.ClientSecret != "")

		refreshedTokens, refreshErr := s.loginAndGetTokens(primaryContact.Email, password)
		if refreshErr != nil {
			log.Printf("[OnboardingService] CRITICAL: Failed to refresh token with updated claims: %v", refreshErr)
			log.Printf("[OnboardingService] User %s will receive a token WITHOUT staff_id/vendor_id claims", security.MaskEmail(primaryContact.Email))
			log.Printf("[OnboardingService] User MUST log out and log back in to get proper permissions")
			// Don't fail - user can still use the old token, they'll just need to re-login
			// But log this as CRITICAL since it means the P0 fix didn't work
		} else {
			finalAccessToken = refreshedTokens.AccessToken
			finalRefreshToken = refreshedTokens.RefreshToken
			finalExpiresIn = refreshedTokens.ExpiresIn
			log.Printf("[OnboardingService] Successfully refreshed token with updated claims for user %s", security.MaskEmail(primaryContact.Email))
		}
	} else {
		log.Printf("[OnboardingService] WARNING: Cannot refresh token - keycloakClient=%v, keycloakConfig=%v",
			s.keycloakClient != nil, s.keycloakConfig != nil)
	}

	response := &CompleteAccountSetupResponse{
		TenantID:     tenantID,
		TenantSlug:   slug,
		UserID:       userID,
		Email:        primaryContact.Email,
		BusinessName: session.BusinessInformation.BusinessName,
		AdminURL:     adminURL,
		AccessToken:  finalAccessToken,
		RefreshToken: finalRefreshToken,
		ExpiresIn:    finalExpiresIn,
		Message:      "Account created successfully. You can now access your admin dashboard.",
	}

	// Send welcome pack email in background with complete tenant URLs
	go s.sendWelcomePackEmail(context.Background(), &WelcomePackEmailRequest{
		Email:        primaryContact.Email,
		FirstName:    primaryContact.FirstName,
		BusinessName: session.BusinessInformation.BusinessName,
		TenantSlug:   slug,
		AdminURL:     adminURL,
	})

	// Extract custom domain config from store_setup if present
	var useCustomDomain bool
	var customDomain string
	var customAdminSubdomain string = "admin"         // Default admin subdomain
	var customStorefrontSubdomain string = ""         // Default empty (root domain)
	for _, config := range session.ApplicationConfigurations {
		if config.ApplicationType == "store_setup" {
			var configData map[string]interface{}
			if err := json.Unmarshal(config.ConfigurationData, &configData); err == nil {
				if useCD, exists := configData["use_custom_domain"]; exists {
					if v, ok := useCD.(bool); ok {
						useCustomDomain = v
					}
				}
				if cd, exists := configData["custom_domain"]; exists {
					if v, ok := cd.(string); ok {
						customDomain = v
					}
				}
				if cas, exists := configData["custom_admin_subdomain"]; exists {
					if v, ok := cas.(string); ok && v != "" {
						customAdminSubdomain = v
					}
				}
				if css, exists := configData["custom_storefront_subdomain"]; exists {
					if v, ok := css.(string); ok {
						customStorefrontSubdomain = v
					}
				}
			}
			break
		}
	}

	// Generate host URLs for tenant-router-service
	var adminHost, storefrontHost string
	if useCustomDomain && customDomain != "" {
		// For custom domains, use the custom subdomains
		adminHost = fmt.Sprintf("%s.%s", customAdminSubdomain, customDomain)
		if customStorefrontSubdomain != "" {
			storefrontHost = fmt.Sprintf("%s.%s", customStorefrontSubdomain, customDomain)
		} else {
			storefrontHost = customDomain // Root domain
		}
		log.Printf("[OnboardingService] Using custom domain hosts: admin=%s, storefront=%s", adminHost, storefrontHost)
	} else {
		// Default tesserix.app domain
		adminHost = fmt.Sprintf("%s-admin.%s", slug, baseDomain)
		storefrontHost = fmt.Sprintf("%s.%s", slug, baseDomain)
	}

	// Publish tenant.created event for document migration, routing, and other subscribers
	// This is synchronous to ensure the VS is created before returning success
	natsPublished := false
	if s.natsClient != nil {
		event := &natsClient.TenantCreatedEvent{
			TenantID:       tenantID.String(),
			SessionID:      sessionID.String(),
			Product:        session.ApplicationType, // e.g., "marketplace", "ecommerce"
			BusinessName:   session.BusinessInformation.BusinessName,
			Slug:           slug,
			Email:          primaryContact.Email,
			AdminHost:      adminHost,
			StorefrontHost: storefrontHost,
			BaseDomain:     baseDomain,
		}
		// Use a context with timeout for event publishing
		publishCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.natsClient.PublishTenantCreated(publishCtx, event); err != nil {
			log.Printf("[OnboardingService] WARNING: Failed to publish tenant.created event: %v", err)
		} else {
			log.Printf("[OnboardingService] Published tenant.created event for %s", slug)
			natsPublished = true
		}
	} else {
		log.Printf("[OnboardingService] WARNING: NATS client not initialized")
	}

	// Direct HTTP call to tenant-router-service as fallback/backup
	// This ensures VS is created even if NATS doesn't deliver the message
	if s.tenantRouterClient != nil {
		provisionReq := &clients.ProvisionTenantHostRequest{
			Slug:           slug,
			TenantID:       tenantID.String(),
			AdminHost:      adminHost,
			StorefrontHost: storefrontHost,
			Product:        session.ApplicationType,
			BusinessName:   session.BusinessInformation.BusinessName,
			Email:          primaryContact.Email,
		}
		provisionCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.tenantRouterClient.ProvisionTenantHost(provisionCtx, provisionReq); err != nil {
			log.Printf("[OnboardingService] WARNING: HTTP fallback to tenant-router-service failed: %v", err)
			if !natsPublished {
				log.Printf("[OnboardingService] CRITICAL: Both NATS and HTTP provisioning failed for %s - manual intervention may be required", slug)
			}
		} else {
			log.Printf("[OnboardingService] Successfully triggered VS provisioning via HTTP for %s", slug)
		}
	} else {
		log.Printf("[OnboardingService] WARNING: Tenant router client not initialized")
	}

	// If custom domain was specified, create it in custom-domain-service
	if useCustomDomain && customDomain != "" && s.customDomainClient != nil {
		log.Printf("[OnboardingService] Creating custom domain %s for tenant %s", customDomain, tenantID)
		domainReq := &clients.CreateDomainRequest{
			Domain:      customDomain,
			TargetType:  "storefront",
			RedirectWWW: true,
			ForceHTTPS:  true,
			IsPrimary:   true,
		}
		domainCtx, domainCancel := context.WithTimeout(context.Background(), 30*time.Second)
		domainResp, domainErr := s.customDomainClient.CreateDomain(domainCtx, tenantID.String(), domainReq)
		domainCancel()
		if domainErr != nil {
			// Log but don't fail - custom domain is not critical for basic tenant operation
			log.Printf("[OnboardingService] WARNING: Failed to create custom domain %s for tenant %s: %v", customDomain, tenantID, domainErr)
		} else {
			log.Printf("[OnboardingService] Created custom domain %s for tenant %s (status: %s)", customDomain, tenantID, domainResp.Status)
		}
	} else if useCustomDomain && customDomain != "" && s.customDomainClient == nil {
		log.Printf("[OnboardingService] WARNING: Custom domain requested but custom-domain-service client not initialized")
	}

	return response, nil
}

// generateSlug creates a URL-friendly slug from a business name
// Example: "My Apparels Store" -> "my-apparels-store"
func generateSlug(businessName string) string {
	// Convert to lowercase
	slug := strings.ToLower(businessName)
	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	// Limit length to 45 chars to allow for numeric suffix
	if len(slug) > 45 {
		slug = slug[:45]
	}
	// Ensure minimum length
	if len(slug) < 3 {
		slug = "store-" + slug
	}
	return slug
}

// generateSubdomain creates a subdomain from business name (legacy, uses slug now)
func generateSubdomain(businessName string) string {
	return generateSlug(businessName)
}

// WelcomePackEmailRequest contains data for sending welcome pack email
type WelcomePackEmailRequest struct {
	Email        string
	FirstName    string
	BusinessName string
	TenantSlug   string
	AdminURL     string
}

// sendWelcomePackEmail sends a comprehensive welcome pack email with all tenant URLs
func (s *OnboardingService) sendWelcomePackEmail(ctx context.Context, req *WelcomePackEmailRequest) {
	// Build all tenant URLs
	storefrontURL := fmt.Sprintf("https://%s.tesserix.app", req.TenantSlug)
	dashboardURL := req.AdminURL + "/dashboard"

	data := &WelcomePackEmailData{
		Email:         req.Email,
		FirstName:     req.FirstName,
		BusinessName:  req.BusinessName,
		TenantSlug:    req.TenantSlug,
		AdminURL:      req.AdminURL,
		StorefrontURL: storefrontURL,
		DashboardURL:  dashboardURL,
	}

	if err := s.notificationSvc.SendWelcomePackEmail(ctx, data); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Failed to send welcome pack email: %v\n", err)
	} else {
		fmt.Printf("Successfully sent welcome pack email to %s\n", security.MaskEmail(req.Email))
	}
}

// sendAccountCreatedEmail sends account created email to new user (legacy)
func (s *OnboardingService) sendAccountCreatedEmail(ctx context.Context, email, firstName, businessName, subdomain string) {
	if err := s.notificationSvc.SendAccountCreatedEmail(ctx, email, firstName, businessName, subdomain); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("Failed to send account created email: %v\n", err)
	} else {
		fmt.Printf("Successfully sent account created email to %s\n", security.MaskEmail(email))
	}
}

// registerUserInAuthService registers a user in the identity provider (Keycloak)
// Supports multi-tenant: same user can have multiple tenants
// Returns the user ID if successful
func (s *OnboardingService) registerUserInAuthService(email, password, name, tenantID, tenantSlug string) (string, error) {
	if s.keycloakClient == nil {
		return "", fmt.Errorf("Keycloak client not configured - authentication service unavailable")
	}
	return s.registerUserInKeycloak(email, password, name, tenantID, tenantSlug)
}

// registerUserInKeycloak registers a user in Keycloak or updates existing user for multi-tenant support
func (s *OnboardingService) registerUserInKeycloak(email, password, name, tenantID, tenantSlug string) (string, error) {
	ctx := context.Background()

	// First check if user already exists (supports multi-tenant - same user can have multiple tenants)
	existingUser, err := s.keycloakClient.GetUserByEmail(ctx, email)
	if err != nil {
		log.Printf("Warning: Failed to check if user exists: %v", err)
		// Continue with user creation attempt
	}

	// If user exists, add the new tenant_id to their attributes and update password if provided
	if existingUser != nil && existingUser.ID != "" {
		log.Printf("User %s already exists in Keycloak, adding tenant %s to their attributes", security.MaskEmail(email), tenantID)
		return s.addTenantToExistingUser(ctx, existingUser, tenantID, tenantSlug, password)
	}

	// Parse name into first/last name
	firstName, lastName := parseFullName(name)

	// Create new user in Keycloak with both tenant_id and tenant_slug attributes
	// These are required for the auth-bff to correctly identify the user's tenant
	user := auth.UserRepresentation{
		Username:      email,
		Email:         email,
		FirstName:     firstName,
		LastName:      lastName,
		Enabled:       true,
		EmailVerified: true, // Auto-verify since onboarding handles verification
		Attributes: map[string][]string{
			"tenant_id":   {tenantID},
			"tenant_slug": {tenantSlug},
		},
	}

	userID, err := s.keycloakClient.CreateUser(ctx, user)
	if err != nil {
		return "", fmt.Errorf("failed to create user in Keycloak: %w", err)
	}

	// Set the user's password
	if err := s.keycloakClient.SetUserPassword(ctx, userID, password, false); err != nil {
		// Try to clean up the user if password setting fails
		_ = s.keycloakClient.DeleteUser(ctx, userID)
		return "", fmt.Errorf("failed to set user password: %w", err)
	}

	// Assign default role - CRITICAL: This must succeed for user to access admin panel
	if s.keycloakConfig.DefaultRole != "" {
		if err := s.keycloakClient.AssignRealmRole(ctx, userID, s.keycloakConfig.DefaultRole); err != nil {
			// FIX-CRITICAL: Role assignment failure means user cannot access admin panel
			// Clean up the created user and fail the operation
			log.Printf("[OnboardingService] CRITICAL: Failed to assign role %s to user %s: %v", s.keycloakConfig.DefaultRole, userID, err)
			if delErr := s.keycloakClient.DeleteUser(ctx, userID); delErr != nil {
				log.Printf("[OnboardingService] Warning: Failed to cleanup user after role assignment failure: %v", delErr)
			}
			return "", fmt.Errorf("failed to assign role %s to user: %w - user creation rolled back", s.keycloakConfig.DefaultRole, err)
		}
		log.Printf("[OnboardingService] Successfully assigned role %s to user %s", s.keycloakConfig.DefaultRole, userID)
	} else {
		log.Printf("[OnboardingService] Warning: No default role configured (KEYCLOAK_DEFAULT_ROLE), user %s will have no realm roles", userID)
	}

	log.Printf("Successfully registered user %s in Keycloak with ID: %s", security.MaskEmail(email), userID)
	return userID, nil
}

// addTenantToExistingUser adds a new tenant_id and tenant_slug to an existing Keycloak user's attributes
// If password is provided, it also updates the user's password in Keycloak
func (s *OnboardingService) addTenantToExistingUser(ctx context.Context, user *auth.UserRepresentation, tenantID, tenantSlug, password string) (string, error) {
	// Get existing tenant_ids and slugs from attributes
	existingTenantIDs := []string{}
	existingTenantSlugs := []string{}
	if user.Attributes != nil {
		if tenantIDs, ok := user.Attributes["tenant_id"]; ok {
			existingTenantIDs = tenantIDs
		}
		if tenantSlugs, ok := user.Attributes["tenant_slug"]; ok {
			existingTenantSlugs = tenantSlugs
		}
	}

	// Check if tenant_id already exists to avoid duplicates
	for _, id := range existingTenantIDs {
		if id == tenantID {
			log.Printf("Tenant %s already associated with user %s", tenantID, security.MaskEmail(user.Email))
			return user.ID, nil
		}
	}

	// Add the new tenant_id and tenant_slug to the lists
	updatedTenantIDs := append(existingTenantIDs, tenantID)
	updatedTenantSlugs := append(existingTenantSlugs, tenantSlug)

	// Update user attributes with both tenant_id and tenant_slug
	updatedAttributes := map[string][]string{
		"tenant_id":   updatedTenantIDs,
		"tenant_slug": updatedTenantSlugs,
	}

	if err := s.keycloakClient.UpdateUserAttributes(ctx, user.ID, updatedAttributes); err != nil {
		return "", fmt.Errorf("failed to update user attributes: %w", err)
	}

	// Update password if provided (e.g., during onboarding with password setup)
	if password != "" {
		if err := s.keycloakClient.SetUserPassword(ctx, user.ID, password, false); err != nil {
			// Password update failure is critical during onboarding - user won't be able to log in to new tenant
			log.Printf("Error: Failed to update password for existing user %s: %v", security.MaskEmail(user.Email), err)
			return "", fmt.Errorf("failed to set password for user: %w - please try again or use password reset", err)
		}
		log.Printf("Successfully updated password for existing user %s", security.MaskEmail(user.Email))
	}

	// FIX-CRITICAL: Ensure existing user has the default role for the new tenant
	// Multi-tenant users need the store_owner role to access their new tenant's admin panel
	if s.keycloakConfig.DefaultRole != "" {
		// Try to assign the role (idempotent - Keycloak handles already-assigned roles gracefully)
		if err := s.keycloakClient.AssignRealmRole(ctx, user.ID, s.keycloakConfig.DefaultRole); err != nil {
			// Check if error is because role already exists (which is fine)
			errStr := err.Error()
			if !strings.Contains(errStr, "already") && !strings.Contains(errStr, "409") {
				log.Printf("[OnboardingService] CRITICAL: Failed to assign role %s to existing user %s: %v", s.keycloakConfig.DefaultRole, user.ID, err)
				return "", fmt.Errorf("failed to assign role %s to user: %w", s.keycloakConfig.DefaultRole, err)
			}
			log.Printf("[OnboardingService] Role %s already assigned to user %s (expected for multi-tenant)", s.keycloakConfig.DefaultRole, user.ID)
		} else {
			log.Printf("[OnboardingService] Successfully assigned role %s to existing user %s", s.keycloakConfig.DefaultRole, user.ID)
		}
	}

	log.Printf("Successfully added tenant %s (slug: %s) to user %s (now has %d tenants)", tenantID, tenantSlug, security.MaskEmail(user.Email), len(updatedTenantIDs))
	return user.ID, nil
}

// parseFullName splits a full name into first and last name
func parseFullName(fullName string) (firstName, lastName string) {
	parts := strings.Fields(strings.TrimSpace(fullName))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// AuthTokenResponse represents tokens returned from auth service login
type AuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token,omitempty"`
}

// loginAndGetTokens logs in the user via Keycloak and returns tokens
func (s *OnboardingService) loginAndGetTokens(email, password string) (*AuthTokenResponse, error) {
	if s.keycloakClient == nil {
		return nil, fmt.Errorf("Keycloak client not configured - authentication service unavailable")
	}

	ctx := context.Background()
	tokenResp, err := s.keycloakClient.GetTokenWithPassword(
		ctx,
		s.keycloakConfig.ClientID,
		s.keycloakConfig.ClientSecret,
		email,
		password,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get tokens from Keycloak: %w", err)
	}

	log.Printf("Successfully logged in user %s via Keycloak", security.MaskEmail(email))
	return &AuthTokenResponse{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresIn:    tokenResp.ExpiresIn,
		IDToken:      tokenResp.IDToken,
	}, nil
}

// ============================================================================
// UNIFIED KEYCLOAK USER SETUP (P0 FIX)
// ============================================================================
// These methods provide a single, verified path for Keycloak user setup.
// Key principles:
// 1. Password setting is CRITICAL - must succeed
// 2. Role assignment is CRITICAL - must succeed
// 3. Verification via login is REQUIRED before returning success
// ============================================================================

// setupUserInKeycloak performs complete Keycloak user setup with verification.
// This is the ONLY method that should be used for Keycloak user setup during onboarding.
//
// Operations performed (in order):
// 1. Create user OR update existing user's tenant attributes
// 2. Set password (required for both new and existing users)
// 3. Assign role (required - failure is fatal)
// 4. Verify setup by attempting authentication
//
// If ANY step fails, appropriate cleanup is performed and error is returned.
// Success is ONLY returned when user can actually authenticate.
func (s *OnboardingService) setupUserInKeycloak(ctx context.Context, req *KeycloakSetupRequest) (*KeycloakSetupResult, error) {
	if s.keycloakClient == nil {
		return nil, fmt.Errorf("keycloak client not configured - authentication service unavailable")
	}

	result := &KeycloakSetupResult{
		Email: req.Email,
	}

	// Step 1: Check if user exists
	existingUser, err := s.keycloakClient.GetUserByEmail(ctx, req.Email)
	if err != nil {
		log.Printf("[KeycloakSetup] Warning: Failed to check if user exists: %v", err)
		// Continue with user creation attempt - GetUserByEmail failure shouldn't block onboarding
	}

	var userID string
	isNewUser := existingUser == nil || existingUser.ID == ""

	if isNewUser {
		// Step 1a: Create new user
		userID, err = s.createKeycloakUser(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to create user in authentication service: %w", err)
		}
		log.Printf("[KeycloakSetup] Created new user %s with ID: %s", security.MaskEmail(req.Email), userID)
	} else {
		// Step 1b: Update existing user's tenant attributes
		userID = existingUser.ID
		if err := s.updateUserTenantAttributes(ctx, existingUser, req.TenantID, req.TenantSlug); err != nil {
			return nil, fmt.Errorf("failed to update user tenant attributes: %w", err)
		}
		log.Printf("[KeycloakSetup] Updated existing user %s with tenant %s", security.MaskEmail(req.Email), req.TenantID)
	}
	result.UserID = userID

	// Step 2: Set password (CRITICAL - must succeed)
	if err := s.keycloakClient.SetUserPassword(ctx, userID, req.Password, false); err != nil {
		if isNewUser {
			// Cleanup: delete the user we just created
			log.Printf("[KeycloakSetup] Cleaning up user %s after password failure", userID)
			_ = s.keycloakClient.DeleteUser(ctx, userID)
		}
		return nil, fmt.Errorf("failed to set password: %w", err)
	}
	log.Printf("[KeycloakSetup] Password set for user %s", security.MaskEmail(req.Email))

	// Step 3: Assign role (CRITICAL - must succeed)
	if req.Role != "" {
		if err := s.assignRoleWithRetry(ctx, userID, req.Role); err != nil {
			if isNewUser {
				// Cleanup: delete the user we just created
				log.Printf("[KeycloakSetup] Cleaning up user %s after role assignment failure", userID)
				_ = s.keycloakClient.DeleteUser(ctx, userID)
			}
			return nil, fmt.Errorf("failed to assign role '%s': %w", req.Role, err)
		}
		result.RoleAssigned = true
		log.Printf("[KeycloakSetup] Role '%s' assigned to user %s", req.Role, security.MaskEmail(req.Email))
	}

	// Step 4: VERIFY - Attempt authentication to confirm setup worked
	tokenResp, err := s.keycloakClient.GetTokenWithPassword(
		ctx,
		s.keycloakConfig.ClientID,
		s.keycloakConfig.ClientSecret,
		req.Email,
		req.Password,
	)
	if err != nil {
		// This is a critical failure - user was "set up" but can't actually login
		log.Printf("[KeycloakSetup] CRITICAL: User %s setup completed but verification failed: %v", security.MaskEmail(req.Email), err)
		// Don't cleanup for existing users - they may have other tenants
		// For new users, cleanup since they can't login anyway
		if isNewUser {
			log.Printf("[KeycloakSetup] Cleaning up user %s after verification failure", userID)
			_ = s.keycloakClient.DeleteUser(ctx, userID)
		}
		return nil, fmt.Errorf("authentication verification failed after setup - please try again: %w", err)
	}

	result.Verified = true
	result.AccessToken = tokenResp.AccessToken
	result.RefreshToken = tokenResp.RefreshToken
	result.ExpiresIn = tokenResp.ExpiresIn
	log.Printf("[KeycloakSetup] Verified user %s can authenticate successfully", security.MaskEmail(req.Email))

	return result, nil
}

// assignRoleWithRetry attempts to assign a role with retry logic and verification
func (s *OnboardingService) assignRoleWithRetry(ctx context.Context, userID, roleName string) error {
	cfg := retryConfig{
		maxAttempts: 3,
		baseDelay:   500 * time.Millisecond,
		maxDelay:    5 * time.Second,
	}

	_, err := retryWithBackoff(ctx, cfg, "Role assignment", func() (bool, error) {
		if err := s.keycloakClient.AssignRealmRole(ctx, userID, roleName); err != nil {
			return false, err
		}
		return true, nil
	})

	if err != nil {
		return err
	}

	// Verify role was actually assigned
	roles, err := s.keycloakClient.GetUserRealmRoles(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to verify role assignment: %w", err)
	}

	for _, role := range roles {
		if role.Name == roleName {
			return nil // Role verified
		}
	}

	return fmt.Errorf("role '%s' not found on user after assignment - verification failed", roleName)
}

// createKeycloakUser creates a new user in Keycloak (without password/role - those are set separately)
func (s *OnboardingService) createKeycloakUser(ctx context.Context, req *KeycloakSetupRequest) (string, error) {
	user := auth.UserRepresentation{
		Username:      req.Email,
		Email:         req.Email,
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		Enabled:       true,
		EmailVerified: true, // Auto-verify since onboarding handles verification
		Attributes: map[string][]string{
			"tenant_id":   {req.TenantID},
			"tenant_slug": {req.TenantSlug},
		},
	}

	return s.keycloakClient.CreateUser(ctx, user)
}

// updateUserTenantAttributes adds a new tenant to an existing user's attributes
func (s *OnboardingService) updateUserTenantAttributes(ctx context.Context, user *auth.UserRepresentation, tenantID, tenantSlug string) error {
	existingTenantIDs := []string{}
	existingTenantSlugs := []string{}

	if user.Attributes != nil {
		if ids, ok := user.Attributes["tenant_id"]; ok {
			existingTenantIDs = ids
		}
		if slugs, ok := user.Attributes["tenant_slug"]; ok {
			existingTenantSlugs = slugs
		}
	}

	// Check if already associated
	for _, id := range existingTenantIDs {
		if id == tenantID {
			log.Printf("[KeycloakSetup] Tenant %s already associated with user %s", tenantID, security.MaskEmail(user.Email))
			return nil // Already associated, nothing to do
		}
	}

	// Add new tenant
	updatedAttributes := map[string][]string{
		"tenant_id":   append(existingTenantIDs, tenantID),
		"tenant_slug": append(existingTenantSlugs, tenantSlug),
	}

	return s.keycloakClient.UpdateUserAttributes(ctx, user.ID, updatedAttributes)
}
