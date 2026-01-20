package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"custom-domain-service/internal/clients"
	"custom-domain-service/internal/config"
	"custom-domain-service/internal/models"
	"custom-domain-service/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// DomainService handles domain business logic
type DomainService struct {
	cfg          *config.Config
	repo         *repository.DomainRepository
	dnsVerifier  *DNSVerifier
	k8sClient    *clients.KubernetesClient
	keycloak     *clients.KeycloakClient
	tenantClient *clients.TenantClient
	redisClient  *redis.Client
}

// NewDomainService creates a new domain service
func NewDomainService(
	cfg *config.Config,
	repo *repository.DomainRepository,
	dnsVerifier *DNSVerifier,
	k8sClient *clients.KubernetesClient,
	keycloak *clients.KeycloakClient,
	tenantClient *clients.TenantClient,
	redisClient *redis.Client,
) *DomainService {
	return &DomainService{
		cfg:          cfg,
		repo:         repo,
		dnsVerifier:  dnsVerifier,
		k8sClient:    k8sClient,
		keycloak:     keycloak,
		tenantClient: tenantClient,
		redisClient:  redisClient,
	}
}

// CreateDomain creates a new custom domain
func (s *DomainService) CreateDomain(ctx context.Context, tenantID uuid.UUID, req *models.CreateDomainRequest, createdBy uuid.UUID) (*models.DomainResponse, error) {
	// Validate domain format
	domainName := strings.ToLower(strings.TrimSpace(req.Domain))
	if err := s.dnsVerifier.ValidateDomainFormat(domainName); err != nil {
		return nil, fmt.Errorf("invalid domain format: %w", err)
	}

	// Check if domain already exists
	exists, err := s.repo.DomainExists(ctx, domainName)
	if err != nil {
		return nil, fmt.Errorf("failed to check domain existence: %w", err)
	}
	if exists {
		return nil, repository.ErrDomainAlreadyExists
	}

	// Check tenant domain limits
	currentCount, err := s.repo.CountByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to count domains: %w", err)
	}

	canAdd, maxAllowed, err := s.tenantClient.CanAddCustomDomain(ctx, tenantID, currentCount)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check tenant limits, using default")
		maxAllowed = s.cfg.Limits.MaxDomainsPerTenant
		canAdd = currentCount < int64(maxAllowed)
	}

	if !canAdd {
		return nil, fmt.Errorf("domain limit reached: maximum %d domains allowed", maxAllowed)
	}

	// Get tenant info for slug
	tenant, err := s.tenantClient.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant info: %w", err)
	}

	// Determine domain type
	domainType := s.dnsVerifier.DetectDomainType(domainName)

	// Set default target type
	targetType := req.TargetType
	if targetType == "" {
		targetType = models.TargetTypeStorefront
	}

	// Create domain record
	domain := &models.CustomDomain{
		TenantID:           tenantID,
		TenantSlug:         tenant.Slug,
		Domain:             domainName,
		DomainType:         domainType,
		TargetType:         targetType,
		VerificationMethod: models.VerificationMethodTXT,
		IncludeWWW:         req.IncludeWWW,
		Status:             models.DomainStatusPending,
		StatusMessage:      "Waiting for DNS verification",
		CreatedBy:          createdBy,
	}

	if err := s.repo.Create(ctx, domain); err != nil {
		return nil, fmt.Errorf("failed to create domain: %w", err)
	}

	// Log activity
	s.logActivity(ctx, domain, "created", "success", "Domain created, awaiting DNS verification")

	// If set as primary, update it
	if req.SetPrimary {
		if err := s.repo.SetPrimaryDomain(ctx, tenantID, domain.ID); err != nil {
			log.Warn().Err(err).Msg("Failed to set domain as primary")
		}
		domain.PrimaryDomain = true
	}

	return s.toDomainResponse(domain), nil
}

// GetDomain retrieves a domain by ID
func (s *DomainService) GetDomain(ctx context.Context, tenantID, domainID uuid.UUID) (*models.DomainResponse, error) {
	domain, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return nil, err
	}

	// Verify tenant ownership
	if domain.TenantID != tenantID {
		return nil, repository.ErrDomainNotFound
	}

	// Add DNS records to response
	response := s.toDomainResponse(domain)
	response.DNSRecords = s.dnsVerifier.GetRequiredDNSRecords(domain)

	return response, nil
}

// ListDomains lists domains for a tenant
func (s *DomainService) ListDomains(ctx context.Context, tenantID uuid.UUID, limit, offset int) (*models.DomainListResponse, error) {
	domains, total, err := s.repo.GetByTenantID(ctx, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}

	response := &models.DomainListResponse{
		Domains: make([]models.DomainResponse, len(domains)),
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: int64(offset+limit) < total,
	}

	for i, d := range domains {
		response.Domains[i] = *s.toDomainResponse(&d)
	}

	return response, nil
}

// UpdateDomain updates domain settings
func (s *DomainService) UpdateDomain(ctx context.Context, tenantID, domainID uuid.UUID, req *models.UpdateDomainRequest) (*models.DomainResponse, error) {
	domain, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return nil, err
	}

	if domain.TenantID != tenantID {
		return nil, repository.ErrDomainNotFound
	}

	updated := false

	if req.RedirectWWW != nil {
		domain.RedirectWWW = *req.RedirectWWW
		updated = true
	}

	if req.ForceHTTPS != nil {
		domain.ForceHTTPS = *req.ForceHTTPS
		updated = true
	}

	if req.PrimaryDomain != nil && *req.PrimaryDomain && !domain.PrimaryDomain {
		if err := s.repo.SetPrimaryDomain(ctx, tenantID, domainID); err != nil {
			return nil, fmt.Errorf("failed to set primary domain: %w", err)
		}
		domain.PrimaryDomain = true
		updated = true
	}

	if updated {
		if err := s.repo.Update(ctx, domain); err != nil {
			return nil, fmt.Errorf("failed to update domain: %w", err)
		}

		// If domain is active, update VirtualService
		if domain.Status == models.DomainStatusActive {
			go func() {
				bgCtx := context.Background()
				if _, err := s.k8sClient.CreateVirtualService(bgCtx, domain); err != nil {
					log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to update VirtualService")
				}
			}()
		}

		s.logActivity(ctx, domain, "updated", "success", "Domain settings updated")
	}

	return s.toDomainResponse(domain), nil
}

// DeleteDomain deletes a domain and cleans up resources
func (s *DomainService) DeleteDomain(ctx context.Context, tenantID, domainID uuid.UUID) error {
	domain, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return err
	}

	if domain.TenantID != tenantID {
		return repository.ErrDomainNotFound
	}

	// Update status to deleting
	if err := s.repo.UpdateStatus(ctx, domainID, models.DomainStatusDeleting, "Cleaning up resources"); err != nil {
		log.Warn().Err(err).Msg("Failed to update status to deleting")
	}

	// Cleanup Kubernetes resources
	if domain.Status == models.DomainStatusActive || domain.RoutingStatus == models.RoutingStatusActive {
		if err := s.k8sClient.DeleteVirtualService(ctx, domain); err != nil {
			log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to delete VirtualService")
		}

		if err := s.k8sClient.RemoveFromGateway(ctx, domain); err != nil {
			log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to remove from gateway")
		}
	}

	// Cleanup certificate
	if domain.SSLStatus != models.SSLStatusPending {
		if err := s.k8sClient.DeleteCertificate(ctx, domain); err != nil {
			log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to delete certificate")
		}
	}

	// Remove Keycloak redirect URIs
	if domain.KeycloakUpdated {
		if err := s.keycloak.RemoveDomainRedirectURIs(ctx, domain); err != nil {
			log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to remove Keycloak URIs")
		}
	}

	// Invalidate cache
	s.invalidateDomainCache(ctx, domain.Domain)

	// Delete domain record
	if err := s.repo.Delete(ctx, domainID); err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	s.logActivity(ctx, domain, "deleted", "success", "Domain and all resources deleted")

	return nil
}

// VerifyDomain triggers DNS verification
func (s *DomainService) VerifyDomain(ctx context.Context, tenantID, domainID uuid.UUID, force bool) (*models.DNSStatusResponse, error) {
	domain, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return nil, err
	}

	if domain.TenantID != tenantID {
		return nil, repository.ErrDomainNotFound
	}

	// Check if already verified and not forcing
	if domain.DNSVerified && !force {
		return s.toDNSStatusResponse(domain, "Domain already verified"), nil
	}

	// Verify DNS
	result, err := s.dnsVerifier.VerifyDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("DNS verification failed: %w", err)
	}

	// Update verification status
	if err := s.repo.UpdateDNSVerification(ctx, domainID, result.IsVerified, domain.DNSCheckAttempts+1); err != nil {
		return nil, fmt.Errorf("failed to update verification status: %w", err)
	}

	// Reload domain to get updated values
	domain, _ = s.repo.GetByID(ctx, domainID)

	// If verified, start provisioning
	if result.IsVerified {
		go s.provisionDomain(context.Background(), domain)
		s.logActivity(ctx, domain, "verified", "success", "DNS verification successful, starting provisioning")
	} else {
		s.logActivity(ctx, domain, "verification_attempted", "pending", result.Message)
	}

	return s.toDNSStatusResponse(domain, result.Message), nil
}

// provisionDomain handles the full provisioning flow after DNS verification
func (s *DomainService) provisionDomain(ctx context.Context, domain *models.CustomDomain) {
	log.Info().Str("domain", domain.Domain).Msg("Starting domain provisioning")

	// Step 1: Create SSL certificate
	certResult, err := s.k8sClient.CreateCertificate(ctx, domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to create certificate")
		s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "SSL certificate creation failed: "+err.Error())
		s.logActivity(ctx, domain, "ssl_provisioning", "failed", err.Error())
		return
	}

	s.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusProvisioning, certResult.SecretName, nil, "")
	s.logActivity(ctx, domain, "ssl_provisioning", "in_progress", "Certificate requested from Let's Encrypt")

	// Step 2: Wait for certificate to be ready (with timeout)
	certCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-certCtx.Done():
			log.Warn().Str("domain", domain.Domain).Msg("Certificate provisioning timed out")
			s.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusFailed, certResult.SecretName, nil, "Certificate provisioning timed out")
			s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "SSL certificate provisioning timed out")
			return
		case <-ticker.C:
			certStatus, err := s.k8sClient.GetCertificateStatus(ctx, domain)
			if err != nil {
				log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to get certificate status")
				continue
			}

			if certStatus.IsReady {
				s.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusActive, certStatus.SecretName, certStatus.ExpiresAt, "")
				s.logActivity(ctx, domain, "ssl_provisioning", "success", "SSL certificate issued and active")
				goto configureRouting
			}

			if certStatus.Status == models.SSLStatusFailed {
				s.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusFailed, certStatus.SecretName, nil, certStatus.Error)
				s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "SSL certificate failed: "+certStatus.Error)
				s.logActivity(ctx, domain, "ssl_provisioning", "failed", certStatus.Error)
				return
			}
		}
	}

configureRouting:
	// Step 3: Configure routing (VirtualService and Gateway)
	vsResult, err := s.k8sClient.CreateVirtualService(ctx, domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to create VirtualService")
		s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "Routing configuration failed: "+err.Error())
		s.logActivity(ctx, domain, "routing", "failed", err.Error())
		return
	}

	if err := s.k8sClient.PatchGateway(ctx, domain); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to patch gateway")
		s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "Gateway configuration failed: "+err.Error())
		s.logActivity(ctx, domain, "routing", "failed", err.Error())
		return
	}

	s.repo.UpdateRoutingStatus(ctx, domain.ID, models.RoutingStatusActive, vsResult.Name, true)
	s.logActivity(ctx, domain, "routing", "success", "VirtualService and Gateway configured")

	// Step 4: Update Keycloak redirect URIs
	if err := s.keycloak.AddDomainRedirectURIs(ctx, domain); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to update Keycloak")
		// Non-critical, don't fail the whole process
		s.logActivity(ctx, domain, "keycloak", "failed", err.Error())
	} else {
		s.repo.UpdateKeycloakStatus(ctx, domain.ID, true)
		s.logActivity(ctx, domain, "keycloak", "success", "Keycloak redirect URIs updated")
	}

	// Step 5: Activate domain
	if err := s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusActive, "Domain is active and ready to use"); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to activate domain")
		return
	}

	// Cache the domain resolution
	s.cacheDomainResolution(ctx, domain)

	log.Info().Str("domain", domain.Domain).Msg("Domain provisioning completed successfully")
	s.logActivity(ctx, domain, "activated", "success", "Domain is now active and serving traffic")

	// Notify tenant service
	s.tenantClient.NotifyDomainStatusChange(ctx, domain.TenantID, domain.Domain, "active")
}

// GetDNSStatus returns DNS verification status
func (s *DomainService) GetDNSStatus(ctx context.Context, tenantID, domainID uuid.UUID) (*models.DNSStatusResponse, error) {
	domain, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return nil, err
	}

	if domain.TenantID != tenantID {
		return nil, repository.ErrDomainNotFound
	}

	return s.toDNSStatusResponse(domain, ""), nil
}

// GetSSLStatus returns SSL certificate status
func (s *DomainService) GetSSLStatus(ctx context.Context, tenantID, domainID uuid.UUID) (*models.SSLStatusResponse, error) {
	domain, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return nil, err
	}

	if domain.TenantID != tenantID {
		return nil, repository.ErrDomainNotFound
	}

	response := &models.SSLStatusResponse{
		DomainID:  domain.ID,
		Domain:    domain.Domain,
		Status:    domain.SSLStatus,
		Provider:  domain.SSLProvider,
		AutoRenew: true,
		LastError: domain.SSLLastError,
	}

	if domain.SSLExpiresAt != nil {
		exp := domain.SSLExpiresAt.Format(time.RFC3339)
		response.ExpiresAt = &exp
		daysRemaining := int(time.Until(*domain.SSLExpiresAt).Hours() / 24)
		response.DaysRemaining = &daysRemaining
	}

	return response, nil
}

// GetStats returns domain statistics for a tenant
func (s *DomainService) GetStats(ctx context.Context, tenantID uuid.UUID) (*models.DomainStatsResponse, error) {
	stats, err := s.repo.GetStats(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Get max allowed from tenant service
	canAdd, maxAllowed, _ := s.tenantClient.CanAddCustomDomain(ctx, tenantID, int64(stats.TotalDomains))
	stats.MaxAllowed = maxAllowed
	stats.CanAddMore = canAdd

	return stats, nil
}

// HealthCheck performs a health check on a domain
func (s *DomainService) HealthCheck(ctx context.Context, tenantID, domainID uuid.UUID) (*models.HealthCheckResponse, error) {
	domain, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return nil, err
	}

	if domain.TenantID != tenantID {
		return nil, repository.ErrDomainNotFound
	}

	// Get latest health check from DB
	health, err := s.repo.GetLatestHealthCheck(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get health check: %w", err)
	}

	if health == nil {
		return &models.HealthCheckResponse{
			DomainID:  domain.ID,
			Domain:    domain.Domain,
			IsHealthy: false,
			Message:   "No health check data available",
			CheckedAt: time.Now().Format(time.RFC3339),
		}, nil
	}

	return &models.HealthCheckResponse{
		DomainID:     domain.ID,
		Domain:       domain.Domain,
		IsHealthy:    health.IsHealthy,
		ResponseTime: health.ResponseTime,
		StatusCode:   health.StatusCode,
		SSLValid:     health.SSLValid,
		SSLExpiresIn: health.SSLExpiresIn,
		CheckedAt:    health.CheckedAt.Format(time.RFC3339),
		Message:      health.ErrorMessage,
	}, nil
}

// ResolveDomain resolves a domain for internal services
func (s *DomainService) ResolveDomain(ctx context.Context, domainName string) (*models.InternalResolveResponse, error) {
	// Check cache first
	cached, err := s.getDomainFromCache(ctx, domainName)
	if err == nil && cached != nil {
		return cached, nil
	}

	// Query database
	domain, err := s.repo.GetByDomain(ctx, domainName)
	if err != nil {
		return nil, err
	}

	response := &models.InternalResolveResponse{
		Domain:     domain.Domain,
		TenantID:   domain.TenantID,
		TenantSlug: domain.TenantSlug,
		TargetType: domain.TargetType,
		IsActive:   domain.Status == models.DomainStatusActive,
		IsPrimary:  domain.PrimaryDomain,
	}

	// Cache the response
	s.cacheDomainResolution(ctx, domain)

	return response, nil
}

// GetActivities returns activity log for a domain
func (s *DomainService) GetActivities(ctx context.Context, tenantID, domainID uuid.UUID, limit int) ([]models.DomainActivity, error) {
	domain, err := s.repo.GetByID(ctx, domainID)
	if err != nil {
		return nil, err
	}

	if domain.TenantID != tenantID {
		return nil, repository.ErrDomainNotFound
	}

	if limit <= 0 {
		limit = 20
	}

	return s.repo.GetActivities(ctx, domainID, limit)
}

// Helper methods

func (s *DomainService) toDomainResponse(domain *models.CustomDomain) *models.DomainResponse {
	response := &models.DomainResponse{
		ID:                 domain.ID,
		TenantID:           domain.TenantID,
		Domain:             domain.Domain,
		DomainType:         domain.DomainType,
		TargetType:         domain.TargetType,
		Status:             domain.Status,
		StatusMessage:      domain.StatusMessage,
		DNSVerified:        domain.DNSVerified,
		SSLStatus:          domain.SSLStatus,
		RoutingStatus:      domain.RoutingStatus,
		RedirectWWW:        domain.RedirectWWW,
		ForceHTTPS:         domain.ForceHTTPS,
		PrimaryDomain:      domain.PrimaryDomain,
		IncludeWWW:         domain.IncludeWWW,
		CreatedAt:          domain.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          domain.UpdatedAt.Format(time.RFC3339),
		VerificationMethod: domain.VerificationMethod,
	}

	if domain.DNSVerifiedAt != nil {
		v := domain.DNSVerifiedAt.Format(time.RFC3339)
		response.DNSVerifiedAt = &v
	}

	if domain.SSLExpiresAt != nil {
		v := domain.SSLExpiresAt.Format(time.RFC3339)
		response.SSLExpiresAt = &v
	}

	if domain.ActivatedAt != nil {
		v := domain.ActivatedAt.Format(time.RFC3339)
		response.ActivatedAt = &v
	}

	return response
}

func (s *DomainService) toDNSStatusResponse(domain *models.CustomDomain, message string) *models.DNSStatusResponse {
	response := &models.DNSStatusResponse{
		DomainID:      domain.ID,
		Domain:        domain.Domain,
		IsVerified:    domain.DNSVerified,
		CheckAttempts: domain.DNSCheckAttempts,
		Records:       s.dnsVerifier.GetRequiredDNSRecords(domain),
		Message:       message,
	}

	if domain.DNSVerifiedAt != nil {
		v := domain.DNSVerifiedAt.Format(time.RFC3339)
		response.VerifiedAt = &v
	}

	if domain.DNSLastCheckedAt != nil {
		v := domain.DNSLastCheckedAt.Format(time.RFC3339)
		response.LastCheckedAt = &v
	}

	return response
}

func (s *DomainService) logActivity(ctx context.Context, domain *models.CustomDomain, action, status, message string) {
	activity := &models.DomainActivity{
		DomainID:  domain.ID,
		TenantID:  domain.TenantID,
		Action:    action,
		Status:    status,
		Message:   message,
		CreatedAt: time.Now(),
	}

	if err := s.repo.LogActivity(ctx, activity); err != nil {
		log.Warn().Err(err).Str("domain", domain.Domain).Str("action", action).Msg("Failed to log activity")
	}
}

func (s *DomainService) cacheDomainResolution(ctx context.Context, domain *models.CustomDomain) {
	if s.redisClient == nil {
		return
	}

	key := fmt.Sprintf("domain:resolve:%s", domain.Domain)
	data := fmt.Sprintf("%s:%s:%s:%t:%t",
		domain.TenantID.String(),
		domain.TenantSlug,
		string(domain.TargetType),
		domain.Status == models.DomainStatusActive,
		domain.PrimaryDomain,
	)

	if err := s.redisClient.Set(ctx, key, data, 5*time.Minute).Err(); err != nil {
		log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to cache domain resolution")
	}
}

func (s *DomainService) getDomainFromCache(ctx context.Context, domainName string) (*models.InternalResolveResponse, error) {
	if s.redisClient == nil {
		return nil, fmt.Errorf("redis not available")
	}

	key := fmt.Sprintf("domain:resolve:%s", domainName)
	data, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// Parse cached data
	parts := strings.Split(data, ":")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid cache format")
	}

	tenantID, err := uuid.Parse(parts[0])
	if err != nil {
		return nil, err
	}

	return &models.InternalResolveResponse{
		Domain:     domainName,
		TenantID:   tenantID,
		TenantSlug: parts[1],
		TargetType: models.TargetType(parts[2]),
		IsActive:   parts[3] == "true",
		IsPrimary:  parts[4] == "true",
	}, nil
}

func (s *DomainService) invalidateDomainCache(ctx context.Context, domainName string) {
	if s.redisClient == nil {
		return
	}

	key := fmt.Sprintf("domain:resolve:%s", domainName)
	s.redisClient.Del(ctx, key)
}
