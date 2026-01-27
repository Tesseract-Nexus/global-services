package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/clients"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/config"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/models"
	"github.com/Tesseract-Nexus/global-services/secret-provisioner/internal/repository"
	"github.com/sirupsen/logrus"
)

// NamingMigrationService handles migration of secrets with inconsistent naming
// It detects secrets with underscores in key names and migrates them to hyphenated format
type NamingMigrationService struct {
	cfg          *config.Config
	gcpClient    *clients.GCPSecretManagerClient
	metadataRepo repository.SecretMetadataRepository
	auditRepo    repository.AuditRepository
	logger       *logrus.Entry
}

// MigrationResult contains the results of a naming migration run
type MigrationResult struct {
	SecretsScanned   int      `json:"secrets_scanned"`
	SecretsMigrated  int      `json:"secrets_migrated"`
	SecretsSkipped   int      `json:"secrets_skipped"`
	Errors           []string `json:"errors,omitempty"`
	MigratedSecrets  []string `json:"migrated_secrets,omitempty"`
	DeletedOldNames  []string `json:"deleted_old_names,omitempty"`
	DryRun           bool     `json:"dry_run,omitempty"`
	PendingMigrations []string `json:"pending_migrations,omitempty"`
}

// NewNamingMigrationService creates a new naming migration service
func NewNamingMigrationService(
	cfg *config.Config,
	gcpClient *clients.GCPSecretManagerClient,
	metadataRepo repository.SecretMetadataRepository,
	auditRepo repository.AuditRepository,
	logger *logrus.Entry,
) *NamingMigrationService {
	return &NamingMigrationService{
		cfg:          cfg,
		gcpClient:    gcpClient,
		metadataRepo: metadataRepo,
		auditRepo:    auditRepo,
		logger:       logger.WithField("component", "naming-migration"),
	}
}

// knownKeyPatterns maps underscore patterns to their correct hyphenated versions
var knownKeyPatterns = map[string]string{
	"key_id":         "key-id",
	"key_secret":     "key-secret",
	"webhook_secret": "webhook-secret",
	"api_key":        "api-key",
	"secret_key":     "secret-key",
	"public_key":     "public-key",
	"private_key":    "private-key",
}

// secretNamePattern matches our secret naming convention
// Format: {env}-tenant-{tenant_id}[-vendor-{vendor_id}]-{provider}-{key_name}
var secretNamePattern = regexp.MustCompile(`^(devtest|staging|prod)-tenant-([a-f0-9-]+)(?:-vendor-([a-f0-9-]+))?-(\w+)-(.+)$`)

// CheckMigration performs a dry-run scan to see what secrets need migration without making any changes
func (s *NamingMigrationService) CheckMigration(ctx context.Context) (*MigrationResult, error) {
	result := &MigrationResult{DryRun: true}

	s.logger.Info("starting secret naming migration dry-run check")

	// List all secrets in the project with our environment prefix
	env := s.cfg.Server.Environment
	filter := fmt.Sprintf("name:%s-tenant-", env)

	secrets, err := s.gcpClient.ListSecrets(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	result.SecretsScanned = len(secrets)
	s.logger.WithField("count", len(secrets)).Info("found secrets to scan (dry-run)")

	for _, secretName := range secrets {
		// Check if this secret has underscore naming that needs migration
		if needsMigration, newName := s.checkNeedsMigration(secretName); needsMigration {
			// Check if new name already exists
			exists, err := s.gcpClient.SecretExists(ctx, newName)
			if err != nil {
				errMsg := fmt.Sprintf("failed to check if %s exists: %v", newName, err)
				result.Errors = append(result.Errors, errMsg)
				continue
			}

			if exists {
				s.logger.WithField("secret", newName).Debug("correctly named secret already exists")
				result.SecretsSkipped++
				continue
			}

			// Would need migration
			result.PendingMigrations = append(result.PendingMigrations, fmt.Sprintf("%s -> %s", secretName, newName))
		} else {
			result.SecretsSkipped++
		}
	}

	s.logger.WithFields(logrus.Fields{
		"scanned":            result.SecretsScanned,
		"pending_migrations": len(result.PendingMigrations),
		"skipped":            result.SecretsSkipped,
		"errors":             len(result.Errors),
	}).Info("naming migration dry-run check completed")

	return result, nil
}

// RunMigration scans GCP secrets and migrates any with underscore naming to hyphenated format
func (s *NamingMigrationService) RunMigration(ctx context.Context, deleteOldSecrets bool) (*MigrationResult, error) {
	result := &MigrationResult{}

	s.logger.Info("starting secret naming migration scan")

	// List all secrets in the project with our environment prefix
	env := s.cfg.Server.Environment
	filter := fmt.Sprintf("name:%s-tenant-", env)

	secrets, err := s.gcpClient.ListSecrets(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	result.SecretsScanned = len(secrets)
	s.logger.WithField("count", len(secrets)).Info("found secrets to scan")

	for _, secretName := range secrets {
		// Check if this secret has underscore naming that needs migration
		if needsMigration, newName := s.checkNeedsMigration(secretName); needsMigration {
			s.logger.WithFields(logrus.Fields{
				"old_name": secretName,
				"new_name": newName,
			}).Info("secret needs migration")

			// Check if new name already exists
			exists, err := s.gcpClient.SecretExists(ctx, newName)
			if err != nil {
				errMsg := fmt.Sprintf("failed to check if %s exists: %v", newName, err)
				result.Errors = append(result.Errors, errMsg)
				s.logger.WithError(err).Error(errMsg)
				continue
			}

			if exists {
				s.logger.WithField("secret", newName).Info("correctly named secret already exists, skipping migration")
				result.SecretsSkipped++
				continue
			}

			// Migrate the secret
			if err := s.migrateSecret(ctx, secretName, newName, deleteOldSecrets); err != nil {
				errMsg := fmt.Sprintf("failed to migrate %s: %v", secretName, err)
				result.Errors = append(result.Errors, errMsg)
				s.logger.WithError(err).Error(errMsg)
				continue
			}

			result.SecretsMigrated++
			result.MigratedSecrets = append(result.MigratedSecrets, fmt.Sprintf("%s -> %s", secretName, newName))

			if deleteOldSecrets {
				result.DeletedOldNames = append(result.DeletedOldNames, secretName)
			}
		} else {
			result.SecretsSkipped++
		}
	}

	s.logger.WithFields(logrus.Fields{
		"scanned":  result.SecretsScanned,
		"migrated": result.SecretsMigrated,
		"skipped":  result.SecretsSkipped,
		"errors":   len(result.Errors),
	}).Info("naming migration scan completed")

	return result, nil
}

// checkNeedsMigration checks if a secret name contains underscores that should be hyphens
// Returns (needsMigration, newName)
func (s *NamingMigrationService) checkNeedsMigration(secretName string) (bool, string) {
	// Extract the key name part from the secret name
	matches := secretNamePattern.FindStringSubmatch(secretName)
	if matches == nil {
		return false, ""
	}

	// matches[5] is the key_name part
	keyName := matches[5]

	// Check if key name has underscore patterns that need migration
	for underscorePattern, hyphenPattern := range knownKeyPatterns {
		if strings.Contains(keyName, underscorePattern) {
			// Build the new secret name with hyphenated key
			newKeyName := strings.ReplaceAll(keyName, underscorePattern, hyphenPattern)
			newSecretName := strings.Replace(secretName, keyName, newKeyName, 1)
			return true, newSecretName
		}
	}

	return false, ""
}

// migrateSecret copies a secret to the new name and optionally deletes the old one
func (s *NamingMigrationService) migrateSecret(ctx context.Context, oldName, newName string, deleteOld bool) error {
	// Get the secret value from the old name
	value, err := s.gcpClient.AccessSecretVersion(ctx, oldName, "latest")
	if err != nil {
		return fmt.Errorf("failed to access secret %s: %w", oldName, err)
	}

	// Get labels from the old secret
	labels, err := s.gcpClient.GetSecretLabels(ctx, oldName)
	if err != nil {
		s.logger.WithError(err).Warn("failed to get labels from old secret, using empty labels")
		labels = make(map[string]string)
	}

	// Add migration metadata to labels
	labels["migrated_from"] = SanitizeLabel(oldName)
	labels["migrated_at"] = SanitizeLabel(time.Now().UTC().Format("2006-01-02"))

	// Create the new secret with the correct name
	_, err = s.gcpClient.CreateOrUpdateSecret(ctx, newName, value, labels)
	if err != nil {
		return fmt.Errorf("failed to create secret %s: %w", newName, err)
	}

	s.logger.WithFields(logrus.Fields{
		"old_name": oldName,
		"new_name": newName,
	}).Info("secret migrated successfully")

	// Update database metadata if it exists for the old name
	if err := s.updateMetadata(ctx, oldName, newName); err != nil {
		s.logger.WithError(err).Warn("failed to update metadata, secret still migrated")
	}

	// Log audit entry
	s.logMigrationAudit(ctx, oldName, newName, "migrated")

	// Optionally delete the old secret
	if deleteOld {
		if err := s.gcpClient.DeleteSecret(ctx, oldName); err != nil {
			s.logger.WithError(err).Warn("failed to delete old secret, migration still successful")
		} else {
			s.logger.WithField("secret", oldName).Info("old secret deleted")
			s.logMigrationAudit(ctx, oldName, newName, "deleted_old")
		}
	}

	return nil
}

// updateMetadata updates the database metadata to reflect the new secret name
func (s *NamingMigrationService) updateMetadata(ctx context.Context, oldName, newName string) error {
	// Try to find existing metadata by old GCP name
	metadata, err := s.metadataRepo.GetByGCPName(ctx, oldName)
	if err != nil {
		// No metadata found, that's okay - might not have been provisioned through this service
		return nil
	}

	if metadata == nil {
		return nil
	}

	// Update the GCP secret name
	metadata.GCPSecretName = newName

	// Also fix the key name if it has underscores
	for underscorePattern, hyphenPattern := range knownKeyPatterns {
		if strings.Contains(metadata.KeyName, underscorePattern) {
			metadata.KeyName = strings.ReplaceAll(metadata.KeyName, underscorePattern, hyphenPattern)
		}
	}

	return s.metadataRepo.Upsert(ctx, metadata)
}

// logMigrationAudit logs an audit entry for the migration
func (s *NamingMigrationService) logMigrationAudit(ctx context.Context, oldName, newName, action string) {
	// Extract tenant ID from the secret name
	matches := secretNamePattern.FindStringSubmatch(oldName)
	tenantID := ""
	if matches != nil && len(matches) > 2 {
		tenantID = matches[2]
	}

	log := &models.SecretAuditLog{
		TenantID:   tenantID,
		SecretName: newName,
		Category:   "payment", // Assuming payment secrets for now
		Provider:   "migration",
		Action:     fmt.Sprintf("naming_migration_%s", action),
		Status:     "success",
	}

	actorService := "secret-provisioner-migration"
	log.ActorService = &actorService

	note := fmt.Sprintf("Migrated from underscore naming: %s -> %s", oldName, newName)
	log.ErrorMessage = &note

	if err := s.auditRepo.Create(ctx, log); err != nil {
		s.logger.WithError(err).Warn("failed to create migration audit log")
	}
}

// ParseSecretName extracts tenant, vendor, provider and key info from a secret name
func ParseSecretName(secretName string) (*SecretNameInfo, error) {
	matches := secretNamePattern.FindStringSubmatch(secretName)
	if matches == nil {
		return nil, fmt.Errorf("secret name does not match expected pattern: %s", secretName)
	}

	info := &SecretNameInfo{
		Environment: matches[1],
		TenantID:    matches[2],
		Provider:    matches[4],
		KeyName:     matches[5],
	}

	if matches[3] != "" {
		info.VendorID = &matches[3]
	}

	return info, nil
}

// SecretNameInfo contains parsed information from a secret name
type SecretNameInfo struct {
	Environment string
	TenantID    string
	VendorID    *string
	Provider    string
	KeyName     string
}

// BuildStandardSecretName builds a properly formatted secret name
func BuildStandardSecretName(env, tenantID string, vendorID *string, provider, keyName string) string {
	// Ensure key name uses hyphens not underscores
	standardKeyName := keyName
	for underscorePattern, hyphenPattern := range knownKeyPatterns {
		standardKeyName = strings.ReplaceAll(standardKeyName, underscorePattern, hyphenPattern)
	}

	parts := []string{env, "tenant", tenantID}

	if vendorID != nil && *vendorID != "" {
		parts = append(parts, "vendor", *vendorID)
	}

	parts = append(parts, provider, standardKeyName)
	return strings.Join(parts, "-")
}
