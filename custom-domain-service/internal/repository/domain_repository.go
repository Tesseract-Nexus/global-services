package repository

import (
	"context"
	"errors"
	"time"

	"custom-domain-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrDomainNotFound      = errors.New("domain not found")
	ErrDomainAlreadyExists = errors.New("domain already exists")
	ErrDomainLimitExceeded = errors.New("domain limit exceeded for tenant")
)

// DomainRepository handles database operations for custom domains
type DomainRepository struct {
	db *gorm.DB
}

// NewDomainRepository creates a new domain repository
func NewDomainRepository(db *gorm.DB) *DomainRepository {
	return &DomainRepository{db: db}
}

// Create creates a new custom domain
func (r *DomainRepository) Create(ctx context.Context, domain *models.CustomDomain) error {
	return r.db.WithContext(ctx).Create(domain).Error
}

// GetByID retrieves a domain by ID
func (r *DomainRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.CustomDomain, error) {
	var domain models.CustomDomain
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&domain).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDomainNotFound
	}
	return &domain, err
}

// GetByDomain retrieves a domain by domain name
func (r *DomainRepository) GetByDomain(ctx context.Context, domainName string) (*models.CustomDomain, error) {
	var domain models.CustomDomain
	err := r.db.WithContext(ctx).Where("domain = ?", domainName).First(&domain).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDomainNotFound
	}
	return &domain, err
}

// GetByTenantID retrieves all domains for a tenant
func (r *DomainRepository) GetByTenantID(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.CustomDomain, int64, error) {
	var domains []models.CustomDomain
	var total int64

	query := r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("tenant_id = ?", tenantID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	err := query.Order("created_at DESC").Find(&domains).Error
	return domains, total, err
}

// GetActiveDomainsByTenantID retrieves active domains for a tenant
func (r *DomainRepository) GetActiveDomainsByTenantID(ctx context.Context, tenantID uuid.UUID) ([]models.CustomDomain, error) {
	var domains []models.CustomDomain
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, models.DomainStatusActive).
		Find(&domains).Error
	return domains, err
}

// GetPendingVerification retrieves domains pending DNS verification
func (r *DomainRepository) GetPendingVerification(ctx context.Context, limit int) ([]models.CustomDomain, error) {
	var domains []models.CustomDomain
	err := r.db.WithContext(ctx).
		Where("status IN (?, ?) AND dns_verified = ?",
			models.DomainStatusPending, models.DomainStatusVerifying, false).
		Order("dns_last_checked_at ASC NULLS FIRST").
		Limit(limit).
		Find(&domains).Error
	return domains, err
}

// GetExpiringCertificates retrieves domains with certificates expiring soon
func (r *DomainRepository) GetExpiringCertificates(ctx context.Context, daysBeforeExpiry int) ([]models.CustomDomain, error) {
	var domains []models.CustomDomain
	expiryThreshold := time.Now().AddDate(0, 0, daysBeforeExpiry)
	err := r.db.WithContext(ctx).
		Where("status = ? AND ssl_status = ? AND ssl_expires_at < ?",
			models.DomainStatusActive, models.SSLStatusActive, expiryThreshold).
		Find(&domains).Error
	return domains, err
}

// GetAllActive retrieves all active domains
func (r *DomainRepository) GetAllActive(ctx context.Context) ([]models.CustomDomain, error) {
	var domains []models.CustomDomain
	err := r.db.WithContext(ctx).
		Where("status = ?", models.DomainStatusActive).
		Find(&domains).Error
	return domains, err
}

// Update updates a domain
func (r *DomainRepository) Update(ctx context.Context, domain *models.CustomDomain) error {
	return r.db.WithContext(ctx).Save(domain).Error
}

// UpdateStatus updates domain status
func (r *DomainRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.DomainStatus, message string) error {
	updates := map[string]interface{}{
		"status":         status,
		"status_message": message,
		"updated_at":     time.Now(),
	}
	if status == models.DomainStatusActive {
		now := time.Now()
		updates["activated_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateDNSVerification updates DNS verification status
func (r *DomainRepository) UpdateDNSVerification(ctx context.Context, id uuid.UUID, verified bool, attempts int) error {
	updates := map[string]interface{}{
		"dns_verified":        verified,
		"dns_last_checked_at": time.Now(),
		"dns_check_attempts":  attempts,
		"updated_at":          time.Now(),
	}
	if verified {
		now := time.Now()
		updates["dns_verified_at"] = &now
		updates["status"] = models.DomainStatusProvisioning
	}
	return r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateNSDelegationVerification updates NS delegation verification status
func (r *DomainRepository) UpdateNSDelegationVerification(ctx context.Context, id uuid.UUID, verified bool, attempts int) error {
	updates := map[string]interface{}{
		"ns_delegation_verified":        verified,
		"ns_delegation_last_checked_at": time.Now(),
		"ns_delegation_check_attempts":  attempts,
		"updated_at":                    time.Now(),
	}
	if verified {
		now := time.Now()
		updates["ns_delegation_verified_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("id = ?", id).Updates(updates).Error
}

// EnableNSDelegation enables NS delegation for a domain
func (r *DomainRepository) EnableNSDelegation(ctx context.Context, id uuid.UUID, enabled bool) error {
	return r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("id = ?", id).Updates(map[string]interface{}{
		"ns_delegation_enabled": enabled,
		"updated_at":            time.Now(),
	}).Error
}

// UpdateSSLStatus updates SSL certificate status
func (r *DomainRepository) UpdateSSLStatus(ctx context.Context, id uuid.UUID, status models.SSLStatus, secretName string, expiresAt *time.Time, lastError string) error {
	updates := map[string]interface{}{
		"ssl_status":          status,
		"ssl_cert_secret_name": secretName,
		"ssl_last_error":      lastError,
		"updated_at":          time.Now(),
	}
	if expiresAt != nil {
		updates["ssl_expires_at"] = expiresAt
	}
	return r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateRoutingStatus updates routing configuration status
func (r *DomainRepository) UpdateRoutingStatus(ctx context.Context, id uuid.UUID, status models.RoutingStatus, vsName string, gatewayPatched bool) error {
	return r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("id = ?", id).Updates(map[string]interface{}{
		"routing_status":       status,
		"virtual_service_name": vsName,
		"gateway_patched":      gatewayPatched,
		"updated_at":           time.Now(),
	}).Error
}

// UpdateKeycloakStatus updates Keycloak integration status
func (r *DomainRepository) UpdateKeycloakStatus(ctx context.Context, id uuid.UUID, updated bool) error {
	updates := map[string]interface{}{
		"keycloak_updated": updated,
		"updated_at":       time.Now(),
	}
	if updated {
		now := time.Now()
		updates["keycloak_updated_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("id = ?", id).Updates(updates).Error
}

// Delete soft-deletes a domain
func (r *DomainRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.CustomDomain{}, "id = ?", id).Error
}

// HardDelete permanently deletes a domain
func (r *DomainRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&models.CustomDomain{}, "id = ?", id).Error
}

// CountByTenantID counts domains for a tenant
func (r *DomainRepository) CountByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("tenant_id = ?", tenantID).Count(&count).Error
	return count, err
}

// CountActiveByTenantID counts active domains for a tenant
func (r *DomainRepository) CountActiveByTenantID(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.CustomDomain{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.DomainStatusActive).
		Count(&count).Error
	return count, err
}

// DomainExists checks if a domain already exists
func (r *DomainRepository) DomainExists(ctx context.Context, domainName string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("domain = ?", domainName).Count(&count).Error
	return count > 0, err
}

// SetPrimaryDomain sets a domain as primary and unsets others
func (r *DomainRepository) SetPrimaryDomain(ctx context.Context, tenantID, domainID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Unset all other primary domains for this tenant
		if err := tx.Model(&models.CustomDomain{}).
			Where("tenant_id = ? AND id != ?", tenantID, domainID).
			Update("primary_domain", false).Error; err != nil {
			return err
		}
		// Set this domain as primary
		return tx.Model(&models.CustomDomain{}).
			Where("id = ?", domainID).
			Update("primary_domain", true).Error
	})
}

// GetPrimaryDomain retrieves the primary domain for a tenant
func (r *DomainRepository) GetPrimaryDomain(ctx context.Context, tenantID uuid.UUID) (*models.CustomDomain, error) {
	var domain models.CustomDomain
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND primary_domain = ? AND status = ?", tenantID, true, models.DomainStatusActive).
		First(&domain).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // No primary domain set
	}
	return &domain, err
}

// LogActivity logs a domain activity
func (r *DomainRepository) LogActivity(ctx context.Context, activity *models.DomainActivity) error {
	return r.db.WithContext(ctx).Create(activity).Error
}

// GetActivities retrieves activities for a domain
func (r *DomainRepository) GetActivities(ctx context.Context, domainID uuid.UUID, limit int) ([]models.DomainActivity, error) {
	var activities []models.DomainActivity
	err := r.db.WithContext(ctx).
		Where("domain_id = ?", domainID).
		Order("created_at DESC").
		Limit(limit).
		Find(&activities).Error
	return activities, err
}

// SaveHealthCheck saves a health check result
func (r *DomainRepository) SaveHealthCheck(ctx context.Context, health *models.DomainHealth) error {
	return r.db.WithContext(ctx).Create(health).Error
}

// GetLatestHealthCheck retrieves the latest health check for a domain
func (r *DomainRepository) GetLatestHealthCheck(ctx context.Context, domainID uuid.UUID) (*models.DomainHealth, error) {
	var health models.DomainHealth
	err := r.db.WithContext(ctx).
		Where("domain_id = ?", domainID).
		Order("checked_at DESC").
		First(&health).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &health, err
}

// CleanupOldHealthChecks removes health checks older than specified duration
func (r *DomainRepository) CleanupOldHealthChecks(ctx context.Context, olderThan time.Duration) (int64, error) {
	threshold := time.Now().Add(-olderThan)
	result := r.db.WithContext(ctx).
		Where("checked_at < ?", threshold).
		Delete(&models.DomainHealth{})
	return result.RowsAffected, result.Error
}

// CleanupOldActivities removes activities older than specified duration
func (r *DomainRepository) CleanupOldActivities(ctx context.Context, olderThan time.Duration) (int64, error) {
	threshold := time.Now().Add(-olderThan)
	result := r.db.WithContext(ctx).
		Where("created_at < ?", threshold).
		Delete(&models.DomainActivity{})
	return result.RowsAffected, result.Error
}

// UpdateCloudflareStatus updates Cloudflare tunnel and DNS status
func (r *DomainRepository) UpdateCloudflareStatus(ctx context.Context, id uuid.UUID, tunnelConfigured, dnsConfigured bool, zoneID string) error {
	updates := map[string]interface{}{
		"cloudflare_tunnel_configured": tunnelConfigured,
		"cloudflare_dns_configured":    dnsConfigured,
		"tunnel_last_checked_at":       time.Now(),
		"updated_at":                   time.Now(),
	}
	if zoneID != "" {
		updates["cloudflare_zone_id"] = zoneID
	}
	return r.db.WithContext(ctx).Model(&models.CustomDomain{}).Where("id = ?", id).Updates(updates).Error
}

// GetPendingCloudflareConfig retrieves domains pending Cloudflare configuration
func (r *DomainRepository) GetPendingCloudflareConfig(ctx context.Context, limit int) ([]models.CustomDomain, error) {
	var domains []models.CustomDomain
	err := r.db.WithContext(ctx).
		Where("status IN (?, ?) AND cloudflare_tunnel_configured = ?",
			models.DomainStatusPending, models.DomainStatusProvisioning, false).
		Order("created_at ASC").
		Limit(limit).
		Find(&domains).Error
	return domains, err
}

// GetStats retrieves domain statistics for a tenant
func (r *DomainRepository) GetStats(ctx context.Context, tenantID uuid.UUID) (*models.DomainStatsResponse, error) {
	var stats models.DomainStatsResponse
	stats.TenantID = tenantID

	// Total domains
	var totalCount int64
	if err := r.db.WithContext(ctx).Model(&models.CustomDomain{}).
		Where("tenant_id = ?", tenantID).
		Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats.TotalDomains = int(totalCount)

	// Active domains
	var activeCount int64
	if err := r.db.WithContext(ctx).Model(&models.CustomDomain{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.DomainStatusActive).
		Count(&activeCount).Error; err != nil {
		return nil, err
	}
	stats.ActiveDomains = int(activeCount)

	// Pending domains
	var pendingCount int64
	if err := r.db.WithContext(ctx).Model(&models.CustomDomain{}).
		Where("tenant_id = ? AND status IN (?, ?, ?)", tenantID,
			models.DomainStatusPending, models.DomainStatusVerifying, models.DomainStatusProvisioning).
		Count(&pendingCount).Error; err != nil {
		return nil, err
	}
	stats.PendingDomains = int(pendingCount)

	// Failed domains
	var failedCount int64
	if err := r.db.WithContext(ctx).Model(&models.CustomDomain{}).
		Where("tenant_id = ? AND status = ?", tenantID, models.DomainStatusFailed).
		Count(&failedCount).Error; err != nil {
		return nil, err
	}
	stats.FailedDomains = int(failedCount)

	return &stats, nil
}
