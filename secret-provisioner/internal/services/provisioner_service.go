package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/clients"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/config"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/repository"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/validators"
	"github.com/sirupsen/logrus"
)

// ProvisionerService handles secret provisioning operations
type ProvisionerService struct {
	cfg          *config.Config
	gcpClient    *clients.GCPSecretManagerClient
	metadataRepo repository.SecretMetadataRepository
	auditRepo    repository.AuditRepository
	validators   *validators.Registry
	logger       *logrus.Entry
}

// NewProvisionerService creates a new provisioner service
func NewProvisionerService(
	cfg *config.Config,
	gcpClient *clients.GCPSecretManagerClient,
	metadataRepo repository.SecretMetadataRepository,
	auditRepo repository.AuditRepository,
	validatorRegistry *validators.Registry,
	logger *logrus.Entry,
) *ProvisionerService {
	return &ProvisionerService{
		cfg:          cfg,
		gcpClient:    gcpClient,
		metadataRepo: metadataRepo,
		auditRepo:    auditRepo,
		validators:   validatorRegistry,
		logger:       logger,
	}
}

// ProvisionSecrets creates or updates secrets in GCP Secret Manager
func (s *ProvisionerService) ProvisionSecrets(ctx context.Context, req *models.ProvisionSecretsRequest, actorID, actorService, requestID string) (*models.ProvisionSecretsResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"tenant_id":  req.TenantID,
		"category":   req.Category,
		"scope":      req.Scope,
		"scope_id":   req.ScopeID,
		"provider":   req.Provider,
		"key_count":  len(req.Secrets),
		"validate":   req.Validate,
		"request_id": requestID,
	}).Info("provisioning secrets")

	// Validate request
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}

	// Run credential validation if requested
	var validationResult *models.ValidationResult
	if req.Validate && s.validators.HasValidator(req.Category, req.Provider) {
		var err error
		validationResult, err = s.validators.ValidateSecrets(ctx, req.Category, req.Provider, req.Secrets)
		if err != nil {
			s.logger.WithError(err).Warn("validation failed")
			validationResult = &models.ValidationResult{
				Status:  models.ValidationUnknown,
				Message: fmt.Sprintf("Validation error: %v", err),
			}
		}
	}

	// Provision each secret
	secretRefs := make([]models.SecretReference, 0, len(req.Secrets))
	for keyName, secretValue := range req.Secrets {
		secretRef, err := s.provisionSingleSecret(ctx, req, keyName, secretValue, validationResult, actorID, actorService, requestID)
		if err != nil {
			// Log audit failure
			s.logAudit(ctx, req.TenantID, "", req.Category, req.Provider, models.AuditActionCreated, models.AuditStatusFailure, err.Error(), actorID, actorService, requestID)
			return nil, fmt.Errorf("failed to provision secret %s: %w", keyName, err)
		}
		secretRefs = append(secretRefs, *secretRef)
	}

	return &models.ProvisionSecretsResponse{
		Status:     "ok",
		SecretRefs: secretRefs,
		Validation: validationResult,
	}, nil
}

// provisionSingleSecret handles provisioning of a single secret
func (s *ProvisionerService) provisionSingleSecret(
	ctx context.Context,
	req *models.ProvisionSecretsRequest,
	keyName, secretValue string,
	validationResult *models.ValidationResult,
	actorID, actorService, requestID string,
) (*models.SecretReference, error) {
	// Build secret name
	secretName := s.buildSecretName(req, keyName)

	// Build labels
	labels := s.buildLabels(req, keyName)

	// Create or update secret in GCP
	versionName, err := s.gcpClient.CreateOrUpdateSecret(ctx, secretName, []byte(secretValue), labels)
	if err != nil {
		return nil, err
	}

	version := clients.ExtractVersion(versionName)

	// Determine validation status
	validationStatus := models.ValidationUnknown
	var validationMsg *string
	if validationResult != nil {
		validationStatus = validationResult.Status
		if validationResult.Message != "" {
			msg := validationResult.Message
			validationMsg = &msg
		}
	}

	// Store metadata in database
	metadata := &models.SecretMetadata{
		TenantID:          req.TenantID,
		Category:          req.Category,
		Scope:             req.Scope,
		ScopeID:           req.ScopeID,
		Provider:          req.Provider,
		KeyName:           keyName,
		GCPSecretName:     secretName,
		GCPSecretVersion:  &version,
		Configured:        true,
		ValidationStatus:  validationStatus,
		ValidationMessage: validationMsg,
		LastUpdatedBy:     &actorID,
	}

	if err := s.metadataRepo.Upsert(ctx, metadata); err != nil {
		s.logger.WithError(err).Error("failed to save metadata")
		// Don't fail the operation, secret is already created
	}

	// Log audit success
	s.logAudit(ctx, req.TenantID, secretName, req.Category, req.Provider, models.AuditActionCreated, models.AuditStatusSuccess, "", actorID, actorService, requestID)

	return &models.SecretReference{
		Name:      secretName,
		Category:  string(req.Category),
		Provider:  req.Provider,
		Key:       keyName,
		Version:   version,
		CreatedAt: time.Now(),
	}, nil
}

// GetSecretMetadata retrieves metadata for secrets
func (s *ProvisionerService) GetSecretMetadata(ctx context.Context, req *models.GetSecretMetadataRequest) (*models.SecretMetadataResponse, error) {
	var metas []*models.SecretMetadata
	var err error

	if req.Provider != "" {
		metas, err = s.metadataRepo.GetByTenantAndProvider(ctx, req.TenantID, req.Category, req.Provider)
	} else {
		category := &req.Category
		if req.Category == "" {
			category = nil
		}
		metas, err = s.metadataRepo.ListByTenant(ctx, req.TenantID, category)
	}

	if err != nil {
		return nil, err
	}

	items := make([]models.SecretMetadataItem, len(metas))
	for i, m := range metas {
		items[i] = models.SecretMetadataItem{
			Name:             m.GCPSecretName,
			Category:         m.Category,
			Provider:         m.Provider,
			KeyName:          m.KeyName,
			Scope:            m.Scope,
			ScopeID:          m.ScopeID,
			Configured:       m.Configured,
			ValidationStatus: m.ValidationStatus,
			LastUpdated:      m.UpdatedAt,
		}
	}

	return &models.SecretMetadataResponse{Secrets: items}, nil
}

// ListProviders lists configured providers for a tenant
func (s *ProvisionerService) ListProviders(ctx context.Context, req *models.ListProvidersRequest) (*models.ListProvidersResponse, error) {
	results, err := s.metadataRepo.ListProviders(ctx, req.TenantID, req.Category)
	if err != nil {
		return nil, err
	}

	// Aggregate results by provider
	providerMap := make(map[string]*models.ProviderStatus)
	for _, r := range results {
		ps, exists := providerMap[r.Provider]
		if !exists {
			ps = &models.ProviderStatus{
				Provider:             r.Provider,
				TenantConfigured:     false,
				VendorConfigurations: []models.VendorConfigStatus{},
			}
			providerMap[r.Provider] = ps
		}

		if r.Scope == models.ScopeTenant {
			ps.TenantConfigured = true
		} else if r.Scope == models.ScopeVendor && r.ScopeID != nil {
			ps.VendorConfigurations = append(ps.VendorConfigurations, models.VendorConfigStatus{
				VendorID:   *r.ScopeID,
				Configured: true,
			})
		}
	}

	providers := make([]models.ProviderStatus, 0, len(providerMap))
	for _, ps := range providerMap {
		providers = append(providers, *ps)
	}

	return &models.ListProvidersResponse{
		TenantID:  req.TenantID,
		Category:  req.Category,
		Providers: providers,
	}, nil
}

// DeleteSecret deletes a secret
func (s *ProvisionerService) DeleteSecret(ctx context.Context, secretName, tenantID, actorID, actorService, requestID string) error {
	// Verify ownership
	meta, err := s.metadataRepo.GetByGCPName(ctx, secretName)
	if err != nil {
		return fmt.Errorf("secret not found: %w", err)
	}
	if meta.TenantID != tenantID {
		return fmt.Errorf("access denied: secret belongs to different tenant")
	}

	// Delete from GCP
	if err := s.gcpClient.DeleteSecret(ctx, secretName); err != nil {
		s.logAudit(ctx, tenantID, secretName, meta.Category, meta.Provider, models.AuditActionDeleted, models.AuditStatusFailure, err.Error(), actorID, actorService, requestID)
		return err
	}

	// Delete from database
	if err := s.metadataRepo.Delete(ctx, secretName); err != nil {
		s.logger.WithError(err).Warn("failed to delete metadata")
	}

	s.logAudit(ctx, tenantID, secretName, meta.Category, meta.Provider, models.AuditActionDeleted, models.AuditStatusSuccess, "", actorID, actorService, requestID)
	return nil
}

// Helper methods

func (s *ProvisionerService) validateRequest(req *models.ProvisionSecretsRequest) error {
	if req.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if req.Category == "" {
		return fmt.Errorf("category is required")
	}
	if req.Scope == "" {
		return fmt.Errorf("scope is required")
	}
	if req.Scope == models.ScopeVendor && (req.ScopeID == nil || *req.ScopeID == "") {
		return fmt.Errorf("scope_id is required for vendor scope")
	}
	if req.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if len(req.Secrets) == 0 {
		return fmt.Errorf("at least one secret is required")
	}
	return nil
}

func (s *ProvisionerService) buildSecretName(req *models.ProvisionSecretsRequest, keyName string) string {
	env := s.cfg.Server.Environment
	parts := []string{env, "tenant", sanitize(req.TenantID)}

	if req.Scope == models.ScopeVendor && req.ScopeID != nil {
		parts = append(parts, "vendor", sanitize(*req.ScopeID))
	}

	parts = append(parts, sanitize(req.Provider), sanitize(keyName))
	return strings.Join(parts, "-")
}

func (s *ProvisionerService) buildLabels(req *models.ProvisionSecretsRequest, keyName string) map[string]string {
	labels := map[string]string{
		"environment": sanitizeLabel(s.cfg.Server.Environment),
		"category":    sanitizeLabel(string(req.Category)),
		"provider":    sanitizeLabel(req.Provider),
		"tenant_id":   sanitizeLabel(req.TenantID),
		"scope":       sanitizeLabel(string(req.Scope)),
		"key_name":    sanitizeLabel(keyName),
		"managed_by":  "secret-provisioner",
	}

	if req.Scope == models.ScopeVendor && req.ScopeID != nil {
		labels["vendor_id"] = sanitizeLabel(*req.ScopeID)
	}

	return labels
}

func (s *ProvisionerService) logAudit(ctx context.Context, tenantID, secretName string, category models.SecretCategory, provider, action, status, errorMsg, actorID, actorService, requestID string) {
	log := &models.SecretAuditLog{
		TenantID:     tenantID,
		SecretName:   secretName,
		Category:     string(category),
		Provider:     provider,
		Action:       action,
		Status:       status,
		ActorID:      &actorID,
		ActorService: &actorService,
		RequestID:    &requestID,
	}

	if errorMsg != "" {
		log.ErrorMessage = &errorMsg
	}

	if err := s.auditRepo.Create(ctx, log); err != nil {
		s.logger.WithError(err).Error("failed to create audit log")
	}
}

// sanitize cleans a string for use in secret names
func sanitize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	// Keep only alphanumeric, underscore, and dash
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			result = append(result, c)
		} else {
			result = append(result, '-')
		}
	}
	return string(result)
}

// sanitizeLabel cleans a string for use in GCP labels
func sanitizeLabel(s string) string {
	s = sanitize(s)
	if len(s) > 63 {
		s = s[:63]
	}
	return s
}

// MetadataToJSON converts metadata to JSON for audit logs
func MetadataToJSON(m map[string]string) []byte {
	if m == nil {
		return nil
	}
	data, _ := json.Marshal(m)
	return data
}
