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

	"github.com/Tesseract-Nexus/go-shared/events"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// DomainService handles domain business logic
type DomainService struct {
	cfg            *config.Config
	repo           *repository.DomainRepository
	dnsVerifier    *DNSVerifier
	k8sClient      *clients.KubernetesClient
	keycloak       *clients.KeycloakClient
	tenantClient   *clients.TenantClient
	cloudflare     *clients.CloudflareClient
	redisClient    *redis.Client
	eventPublisher *events.Publisher
}

// NewDomainService creates a new domain service
func NewDomainService(
	cfg *config.Config,
	repo *repository.DomainRepository,
	dnsVerifier *DNSVerifier,
	k8sClient *clients.KubernetesClient,
	keycloak *clients.KeycloakClient,
	tenantClient *clients.TenantClient,
	cloudflare *clients.CloudflareClient,
	redisClient *redis.Client,
	eventPublisher *events.Publisher,
) *DomainService {
	return &DomainService{
		cfg:            cfg,
		repo:           repo,
		dnsVerifier:    dnsVerifier,
		k8sClient:      k8sClient,
		keycloak:       keycloak,
		tenantClient:   tenantClient,
		cloudflare:     cloudflare,
		redisClient:    redisClient,
		eventPublisher: eventPublisher,
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

	// Publish domain added event
	s.publishDomainEvent(ctx, events.DomainAdded, domain, "")

	// If set as primary, update it
	if req.SetPrimary {
		if err := s.repo.SetPrimaryDomain(ctx, tenantID, domain.ID); err != nil {
			log.Warn().Err(err).Msg("Failed to set domain as primary")
		}
		domain.PrimaryDomain = true
	}

	return s.toDomainResponse(domain), nil
}

// ValidateDomainRequest contains the domain to validate
type ValidateDomainRequest struct {
	Domain   string `json:"domain"`
	CheckDNS bool   `json:"check_dns"`
}

// ValidateDomainResponse contains validation results
type ValidateDomainResponse struct {
	Valid               bool                `json:"valid"`
	Available           bool                `json:"available"`
	DomainExists        bool                `json:"domain_exists"`
	DNSConfigured       bool                `json:"dns_configured"`
	Message             string              `json:"message,omitempty"`
	VerificationRecord  *models.DNSRecord   `json:"verification_record,omitempty"`
	VerificationRecords []models.DNSRecord  `json:"verification_records,omitempty"`
	DomainType          string              `json:"domain_type,omitempty"`
}

// ValidateDomain validates a domain without creating it
func (s *DomainService) ValidateDomain(ctx context.Context, req *ValidateDomainRequest) (*ValidateDomainResponse, error) {
	response := &ValidateDomainResponse{
		Valid:        false,
		Available:    false,
		DomainExists: false,
	}

	// Validate domain format
	domainName := strings.ToLower(strings.TrimSpace(req.Domain))
	if err := s.dnsVerifier.ValidateDomainFormat(domainName); err != nil {
		response.Message = err.Error()
		return response, nil
	}

	response.Valid = true

	// Check if domain actually exists (is registered) by looking up NS records
	// This is a quick check with a 3-second timeout
	domainExists, err := s.dnsVerifier.CheckDomainExists(ctx, domainName)
	if err != nil {
		log.Warn().Err(err).Str("domain", domainName).Msg("Failed to check domain existence, continuing anyway")
		domainExists = true // Assume exists on error to avoid blocking
	}
	response.DomainExists = domainExists

	if !domainExists {
		response.Message = "This domain does not appear to be registered. Please verify you own this domain and try again."
		return response, nil
	}

	// Check if domain already exists in our system
	exists, err := s.repo.DomainExists(ctx, domainName)
	if err != nil {
		return nil, fmt.Errorf("failed to check domain existence: %w", err)
	}

	if exists {
		response.Available = false
		response.Message = "This domain is already registered with another store"
		return response, nil
	}

	response.Available = true

	// Detect domain type
	domainType := s.dnsVerifier.DetectDomainType(domainName)
	response.DomainType = string(domainType)

	// Generate verification records (both CNAME and TXT options)
	verificationToken := uuid.New().String()[:32]

	// Primary option: CNAME record (recommended - simpler)
	cnameRecord := models.DNSRecord{
		RecordType: "CNAME",
		Host:       "_tesserix." + domainName,
		Value:      "verify.tesserix.app",
		TTL:        3600,
		Purpose:    "verification",
	}

	// Alternative option: TXT record (for DNS providers that don't support CNAME on subdomains)
	txtRecord := models.DNSRecord{
		RecordType: "TXT",
		Host:       "_tesserix." + domainName,
		Value:      "tesserix-verify=" + verificationToken,
		TTL:        3600,
		Purpose:    "verification",
	}

	// Return primary as VerificationRecord (for backward compatibility)
	response.VerificationRecord = &cnameRecord

	// Return both options in VerificationRecords
	response.VerificationRecords = []models.DNSRecord{cnameRecord, txtRecord}

	response.Message = "Domain is valid and available"

	// Check DNS if requested
	if req.CheckDNS {
		// Create a temporary domain object for DNS verification
		tempDomain := &models.CustomDomain{
			Domain:             domainName,
			VerificationMethod: models.VerificationMethodCNAME,
			VerificationToken:  verificationToken,
		}

		result, err := s.dnsVerifier.VerifyDomain(ctx, tempDomain)
		if err == nil && result.IsVerified {
			response.DNSConfigured = true
			response.Message = "Domain is valid and DNS is configured"
		}
	}

	return response, nil
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

	// Cleanup Cloudflare Tunnel route
	if s.cfg.Cloudflare.Enabled && s.cloudflare != nil {
		if err := s.cloudflare.RemoveDomainFromTunnel(ctx, domain); err != nil {
			log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to remove domain from Cloudflare Tunnel")
		}
	}

	// Cleanup Kubernetes resources
	if domain.Status == models.DomainStatusActive || domain.RoutingStatus == models.RoutingStatusActive {
		if err := s.k8sClient.DeleteVirtualService(ctx, domain); err != nil {
			log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to delete VirtualService")
		}

		// Only remove from Gateway if not using Cloudflare Tunnel
		if !s.cfg.Cloudflare.Enabled {
			if err := s.k8sClient.RemoveFromGateway(ctx, domain); err != nil {
				log.Warn().Err(err).Str("domain", domain.Domain).Msg("Failed to remove from gateway")
			}
		}
	}

	// Cleanup certificate (only if not using Cloudflare Tunnel)
	if !s.cfg.Cloudflare.Enabled && domain.SSLStatus != models.SSLStatusPending {
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

	// Capture domain info before deletion for event
	domainCopy := *domain

	// Delete domain record
	if err := s.repo.Delete(ctx, domainID); err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	s.logActivity(ctx, domain, "deleted", "success", "Domain and all resources deleted")

	// Publish domain removed event
	domainCopy.Status = models.DomainStatusDeleting
	domainCopy.StatusMessage = "Domain has been deleted"
	s.publishDomainEvent(ctx, events.DomainRemoved, &domainCopy, string(domain.Status))

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
		// Publish DNS verified event
		s.publishDomainEvent(ctx, events.DomainVerified, domain, string(models.DomainStatusPending))
	} else {
		s.logActivity(ctx, domain, "verification_attempted", "pending", result.Message)
	}

	return s.toDNSStatusResponse(domain, result.Message), nil
}

// provisionDomain handles the full provisioning flow after DNS verification
func (s *DomainService) provisionDomain(ctx context.Context, domain *models.CustomDomain) {
	log.Info().Str("domain", domain.Domain).Msg("Starting domain provisioning")

	// Check if Cloudflare Tunnel is enabled
	if s.cfg.Cloudflare.Enabled && s.cloudflare != nil {
		s.provisionDomainWithCloudflare(ctx, domain)
		return
	}

	// Legacy provisioning with cert-manager
	s.provisionDomainWithCertManager(ctx, domain)
}

// provisionDomainWithCloudflare provisions a domain using Cloudflare Tunnel
// SSL is handled by Cloudflare, no cert-manager needed
func (s *DomainService) provisionDomainWithCloudflare(ctx context.Context, domain *models.CustomDomain) {
	log.Info().Str("domain", domain.Domain).Msg("Provisioning domain via Cloudflare Tunnel")

	// Step 1: Add domain to Cloudflare Tunnel
	if err := s.cloudflare.AddDomainToTunnel(ctx, domain); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to add domain to Cloudflare Tunnel")
		s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "Failed to configure Cloudflare Tunnel. Please contact support.")
		s.logActivity(ctx, domain, "cloudflare_tunnel", "failed", "Failed to add domain to tunnel")
		domain.Status = models.DomainStatusFailed
		domain.StatusMessage = "Cloudflare Tunnel configuration failed"
		s.publishDomainEvent(ctx, events.DomainFailed, domain, string(models.DomainStatusPending))
		return
	}

	// Mark SSL as active (Cloudflare handles SSL)
	s.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusActive, "cloudflare-managed", nil, "")
	s.logActivity(ctx, domain, "ssl_provisioning", "success", "SSL managed by Cloudflare")
	domain.SSLStatus = models.SSLStatusActive
	s.publishDomainEvent(ctx, events.DomainSSLProvisioned, domain, string(models.SSLStatusProvisioning))

	// Step 2: Configure routing (VirtualService only, no Gateway patching needed)
	vsResult, err := s.k8sClient.CreateVirtualService(ctx, domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to create VirtualService")
		s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "Routing configuration failed. Please contact support.")
		s.logActivity(ctx, domain, "routing", "failed", "VirtualService creation failed")
		domain.Status = models.DomainStatusFailed
		domain.StatusMessage = "Routing configuration failed"
		s.publishDomainEvent(ctx, events.DomainFailed, domain, string(models.DomainStatusPending))
		return
	}

	s.repo.UpdateRoutingStatus(ctx, domain.ID, models.RoutingStatusActive, vsResult.Name, true)
	s.logActivity(ctx, domain, "routing", "success", "VirtualService configured for Cloudflare Tunnel")

	// Step 3: Update Keycloak redirect URIs
	if err := s.keycloak.AddDomainRedirectURIs(ctx, domain); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to update Keycloak")
		s.logActivity(ctx, domain, "keycloak", "failed", "Authentication configuration update failed")
	} else {
		s.repo.UpdateKeycloakStatus(ctx, domain.ID, true)
		s.logActivity(ctx, domain, "keycloak", "success", "Keycloak redirect URIs updated")
	}

	// Step 4: Activate domain
	previousStatus := string(domain.Status)
	if err := s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusActive, "Domain is active and ready to use"); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to activate domain")
		return
	}

	// Cache the domain resolution
	s.cacheDomainResolution(ctx, domain)

	log.Info().Str("domain", domain.Domain).Msg("Domain provisioning via Cloudflare Tunnel completed successfully")
	s.logActivity(ctx, domain, "activated", "success", "Domain is now active via Cloudflare Tunnel")

	// Publish domain activated event
	domain.Status = models.DomainStatusActive
	domain.StatusMessage = "Domain is active and ready to use"
	domain.RoutingStatus = models.RoutingStatusActive
	s.publishDomainEvent(ctx, events.DomainActivated, domain, previousStatus)

	// Notify tenant service
	s.tenantClient.NotifyDomainStatusChange(ctx, domain.TenantID, domain.Domain, "active")
}

// provisionDomainWithCertManager provisions a domain using cert-manager (legacy)
func (s *DomainService) provisionDomainWithCertManager(ctx context.Context, domain *models.CustomDomain) {
	log.Info().Str("domain", domain.Domain).Msg("Provisioning domain via cert-manager (legacy)")

	// Step 1: Create SSL certificate
	certResult, err := s.k8sClient.CreateCertificate(ctx, domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to create certificate")
		s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "SSL certificate creation failed. Please contact support if the issue persists.")
		s.logActivity(ctx, domain, "ssl_provisioning", "failed", "Certificate creation failed")
		domain.Status = models.DomainStatusFailed
		domain.StatusMessage = "SSL certificate creation failed"
		s.publishDomainEvent(ctx, events.DomainFailed, domain, string(models.DomainStatusPending))
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
			domain.Status = models.DomainStatusFailed
			domain.StatusMessage = "SSL certificate provisioning timed out"
			s.publishDomainEvent(ctx, events.DomainFailed, domain, string(models.DomainStatusPending))
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
				domain.SSLStatus = models.SSLStatusActive
				domain.SSLExpiresAt = certStatus.ExpiresAt
				s.publishDomainEvent(ctx, events.DomainSSLProvisioned, domain, string(models.SSLStatusProvisioning))
				goto configureRouting
			}

			if certStatus.Status == models.SSLStatusFailed {
				log.Error().Str("domain", domain.Domain).Str("cert_error", certStatus.Error).Msg("Certificate provisioning failed")
				s.repo.UpdateSSLStatus(ctx, domain.ID, models.SSLStatusFailed, certStatus.SecretName, nil, "Certificate validation failed")
				s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "SSL certificate provisioning failed. Please verify your DNS configuration.")
				s.logActivity(ctx, domain, "ssl_provisioning", "failed", "Certificate provisioning failed")
				domain.Status = models.DomainStatusFailed
				domain.StatusMessage = "SSL certificate provisioning failed"
				s.publishDomainEvent(ctx, events.DomainFailed, domain, string(models.DomainStatusPending))
				return
			}
		}
	}

configureRouting:
	// Step 3: Configure routing (VirtualService and Gateway)
	vsResult, err := s.k8sClient.CreateVirtualService(ctx, domain)
	if err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to create VirtualService")
		s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "Routing configuration failed. Please contact support.")
		s.logActivity(ctx, domain, "routing", "failed", "VirtualService creation failed")
		domain.Status = models.DomainStatusFailed
		domain.StatusMessage = "Routing configuration failed"
		s.publishDomainEvent(ctx, events.DomainFailed, domain, string(models.DomainStatusPending))
		return
	}

	if err := s.k8sClient.PatchGateway(ctx, domain); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to patch gateway")
		s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusFailed, "Gateway configuration failed. Please contact support.")
		s.logActivity(ctx, domain, "routing", "failed", "Gateway configuration failed")
		domain.Status = models.DomainStatusFailed
		domain.StatusMessage = "Gateway configuration failed"
		s.publishDomainEvent(ctx, events.DomainFailed, domain, string(models.DomainStatusPending))
		return
	}

	s.repo.UpdateRoutingStatus(ctx, domain.ID, models.RoutingStatusActive, vsResult.Name, true)
	s.logActivity(ctx, domain, "routing", "success", "VirtualService and Gateway configured")

	// Step 4: Update Keycloak redirect URIs
	if err := s.keycloak.AddDomainRedirectURIs(ctx, domain); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to update Keycloak")
		s.logActivity(ctx, domain, "keycloak", "failed", "Authentication configuration update failed")
	} else {
		s.repo.UpdateKeycloakStatus(ctx, domain.ID, true)
		s.logActivity(ctx, domain, "keycloak", "success", "Keycloak redirect URIs updated")
	}

	// Step 5: Activate domain
	previousStatus := string(domain.Status)
	if err := s.repo.UpdateStatus(ctx, domain.ID, models.DomainStatusActive, "Domain is active and ready to use"); err != nil {
		log.Error().Err(err).Str("domain", domain.Domain).Msg("Failed to activate domain")
		return
	}

	// Cache the domain resolution
	s.cacheDomainResolution(ctx, domain)

	log.Info().Str("domain", domain.Domain).Msg("Domain provisioning completed successfully")
	s.logActivity(ctx, domain, "activated", "success", "Domain is now active and serving traffic")

	// Publish domain activated event
	domain.Status = models.DomainStatusActive
	domain.StatusMessage = "Domain is active and ready to use"
	domain.RoutingStatus = models.RoutingStatusActive
	s.publishDomainEvent(ctx, events.DomainActivated, domain, previousStatus)

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

// publishDomainEvent publishes a domain event to NATS
func (s *DomainService) publishDomainEvent(ctx context.Context, eventType string, domain *models.CustomDomain, previousStatus string) {
	if s.eventPublisher == nil {
		log.Debug().Str("event", eventType).Msg("Event publisher not configured, skipping event")
		return
	}

	event := events.NewDomainEvent(eventType, domain.TenantID.String())
	event.DomainID = domain.ID.String()
	event.Domain = domain.Domain
	event.DomainType = string(domain.DomainType)
	event.TenantSlug = domain.TenantSlug
	event.Status = string(domain.Status)
	event.PreviousStatus = previousStatus
	event.StatusMessage = domain.StatusMessage
	event.DNSVerified = domain.DNSVerified
	event.SSLStatus = string(domain.SSLStatus)
	event.RoutingStatus = string(domain.RoutingStatus)
	event.IsPrimary = domain.PrimaryDomain
	event.TargetType = string(domain.TargetType)
	event.DomainURL = fmt.Sprintf("https://%s", domain.Domain)
	event.AdminURL = fmt.Sprintf("https://%s-admin.tesserix.app", domain.TenantSlug)

	if domain.DNSVerifiedAt != nil {
		event.DNSVerifiedAt = domain.DNSVerifiedAt.Format(time.RFC3339)
	}
	if domain.SSLExpiresAt != nil {
		event.SSLExpiresAt = domain.SSLExpiresAt.Format(time.RFC3339)
	}

	// Publish asynchronously to avoid blocking the main flow
	go func() {
		publishCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.eventPublisher.PublishDomain(publishCtx, event); err != nil {
			log.Error().Err(err).
				Str("event_type", eventType).
				Str("domain", domain.Domain).
				Msg("Failed to publish domain event")
		} else {
			log.Info().
				Str("event_type", eventType).
				Str("domain", domain.Domain).
				Str("tenant_id", domain.TenantID.String()).
				Msg("Domain event published")
		}
	}()
}
