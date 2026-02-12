package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"tenant-service/internal/integrations"
	"tenant-service/internal/models"
	natsClient "tenant-service/internal/nats"
	"tenant-service/internal/redis"
	"github.com/Tesseract-Nexus/go-shared/auth"
	"github.com/Tesseract-Nexus/go-shared/secrets"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// OffboardingService handles tenant deletion and offboarding logic
type OffboardingService struct {
	db                     *gorm.DB
	membershipSvc          *MembershipService
	natsClient             *natsClient.Client
	keycloakClient         *auth.KeycloakAdminClient // Internal realm (tesserix-internal)
	keycloakCustomerClient *auth.KeycloakAdminClient // Customer realm (tesserix-customer)
	redisClient            *redis.Client
	growthBookClient       *integrations.GrowthBookClient
}

// NewOffboardingService creates a new offboarding service
func NewOffboardingService(
	db *gorm.DB,
	membershipSvc *MembershipService,
	natsClient *natsClient.Client,
	keycloakClient *auth.KeycloakAdminClient,
	redisClient *redis.Client,
	growthBookClient *integrations.GrowthBookClient,
) *OffboardingService {
	svc := &OffboardingService{
		db:               db,
		membershipSvc:    membershipSvc,
		natsClient:       natsClient,
		keycloakClient:   keycloakClient,
		redisClient:      redisClient,
		growthBookClient: growthBookClient,
	}

	// Initialize customer realm Keycloak client for organization cleanup
	svc.keycloakCustomerClient = initCustomerKeycloakClient()

	return svc
}

// initCustomerKeycloakClient creates the Keycloak admin client for the customer realm
func initCustomerKeycloakClient() *auth.KeycloakAdminClient {
	customerBaseURL := os.Getenv("KEYCLOAK_CUSTOMER_BASE_URL")
	if customerBaseURL == "" {
		customerBaseURL = "https://devtest-customer-idp.tesserix.app"
	}

	customerRealm := os.Getenv("KEYCLOAK_CUSTOMER_REALM")
	if customerRealm == "" {
		customerRealm = "tesserix-customer"
	}

	customerAdminClientID := os.Getenv("KEYCLOAK_CUSTOMER_ADMIN_CLIENT_ID")
	if customerAdminClientID == "" {
		customerAdminClientID = "admin-cli"
	}

	customerAdminClientSecret := secrets.GetSecretOrEnv("KEYCLOAK_CUSTOMER_ADMIN_CLIENT_SECRET_NAME", "KEYCLOAK_CUSTOMER_ADMIN_CLIENT_SECRET", "")

	if customerAdminClientSecret != "" {
		client := auth.NewKeycloakAdminClient(auth.KeycloakAdminConfig{
			BaseURL:      customerBaseURL,
			Realm:        customerRealm,
			ClientID:     customerAdminClientID,
			ClientSecret: customerAdminClientSecret,
			Timeout:      30 * time.Second,
		})
		log.Printf("[OffboardingService] Keycloak CUSTOMER realm client initialized: %s/realms/%s", customerBaseURL, customerRealm)
		return client
	}

	log.Printf("[OffboardingService] Warning: KEYCLOAK_CUSTOMER_ADMIN_CLIENT_SECRET not set - customer realm cleanup will be skipped")
	return nil
}

// DeleteTenantRequest represents the request to delete a tenant
type DeleteTenantRequest struct {
	TenantID         uuid.UUID
	UserID           uuid.UUID
	ConfirmationText string
	Reason           string
}

// AdminDeleteTenantRequest represents a platform-admin delete request (no owner check)
type AdminDeleteTenantRequest struct {
	TenantID uuid.UUID
	Reason   string
}

// DeleteTenantResponse represents the response after deleting a tenant
type DeleteTenantResponse struct {
	Message          string    `json:"message"`
	TenantID         string    `json:"tenant_id"`
	ArchivedRecordID string    `json:"archived_record_id"`
	ArchivedAt       time.Time `json:"archived_at"`
}

// BatchDeleteResult represents the result of a batch delete operation
type BatchDeleteResult struct {
	Succeeded []string             `json:"succeeded"`
	Failed    []BatchDeleteFailure `json:"failed"`
}

// BatchDeleteFailure represents a single failure in a batch delete
type BatchDeleteFailure struct {
	ID    string `json:"id"`
	Slug  string `json:"slug,omitempty"`
	Error string `json:"error"`
}

// DeletionPreview contains counts of data that would be deleted
type DeletionPreview struct {
	TenantID string                    `json:"tenant_id"`
	Slug     string                    `json:"slug"`
	Name     string                    `json:"name"`
	Counts   map[string]map[string]int `json:"counts"` // database -> table -> count
}

// externalDBCleanup defines tables to clean for an external database
type externalDBCleanup struct {
	DBName string
	// Tables in deletion order (children first due to foreign keys)
	Tables []string
}

// getExternalDBCleanups returns the list of external databases and their tables to clean
func getExternalDBCleanups() []externalDBCleanup {
	return []externalDBCleanup{
		{
			DBName: "staff_db",
			Tables: []string{
				"staff_role_assignments", "staff_sessions", "staff_login_audit",
				"staff_documents", "staff_emergency_contacts", "staff_invitations",
				"staff_password_history", "staff_oauth_providers", "staff_rbac_audit_log",
				"staff_roles", "teams", "departments", "staff",
			},
		},
		{
			DBName: "vendors_db",
			Tables: []string{"storefronts", "vendors"},
		},
		{
			DBName: "subscription_db",
			Tables: []string{"subscription_events", "invoices", "subscriptions"},
		},
		{
			DBName: "notifications_db",
			Tables: []string{"notification_preferences", "notification_templates", "notifications"},
		},
		{
			DBName: "settings_db",
			Tables: []string{"setting_history", "setting_presets", "settings"},
		},
		{
			DBName: "audit_db",
			Tables: []string{"audit_logs"},
		},
		{
			DBName: "verification_db",
			Tables: []string{"verification_attempts", "verification_rate_limits", "verification_codes"},
		},
		{
			DBName: "tickets_db",
			Tables: []string{"ticket_comments", "tickets"},
		},
		{
			DBName: "documents_db",
			Tables: []string{"documents"},
		},
		{
			DBName: "notifications_hub_db",
			Tables: []string{"in_app_notifications"},
		},
		{
			DBName: "custom_domains_db",
			Tables: []string{"domain_health_checks", "domain_activities", "custom_domains"},
		},
		{
			DBName: "translation_db",
			Tables: []string{"translation_cache", "translation_stats", "translation_preferences"},
		},
		{
			DBName: "qr_db",
			Tables: []string{"qr_codes"},
		},
		{
			DBName: "auth_db",
			Tables: []string{"user_sessions", "user_permissions", "user_roles", "users"},
		},
	}
}

// connectExternalDB opens a GORM connection to an external database
// using the same PG host/port/user/password but a different dbname
func connectExternalDB(dbName string) (*gorm.DB, error) {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}
	password := secrets.GetDBPassword()
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbName, sslmode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", dbName, err)
	}

	// Set conservative connection pool for cleanup connections
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB for %s: %w", dbName, err)
	}
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(30 * time.Second)

	return db, nil
}

// cleanupExternalDatabases connects to each external database and deletes tenant data.
// Each database is cleaned in its own transaction. Errors are logged but don't block deletion.
func (s *OffboardingService) cleanupExternalDatabases(ctx context.Context, tenantID string) map[string]map[string]int64 {
	results := make(map[string]map[string]int64)

	for _, cleanup := range getExternalDBCleanups() {
		dbResults := make(map[string]int64)

		extDB, err := connectExternalDB(cleanup.DBName)
		if err != nil {
			log.Printf("[OffboardingService] Warning: Could not connect to %s: %v", cleanup.DBName, err)
			continue
		}

		// Close connection when done with this database
		sqlDB, _ := extDB.DB()
		defer sqlDB.Close()

		// Run all table deletions in a single transaction per database
		tx := extDB.WithContext(ctx).Begin()
		if tx.Error != nil {
			log.Printf("[OffboardingService] Warning: Could not start transaction for %s: %v", cleanup.DBName, tx.Error)
			continue
		}

		for _, table := range cleanup.Tables {
			result := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE tenant_id = ?", table), tenantID)
			if result.Error != nil {
				// Table might not exist or have different schema — log and continue
				log.Printf("[OffboardingService] Warning: Failed to clean %s.%s: %v", cleanup.DBName, table, result.Error)
			} else if result.RowsAffected > 0 {
				dbResults[table] = result.RowsAffected
				log.Printf("[OffboardingService] Cleaned %d rows from %s.%s for tenant %s", result.RowsAffected, cleanup.DBName, table, tenantID)
			}
		}

		if err := tx.Commit().Error; err != nil {
			log.Printf("[OffboardingService] Warning: Failed to commit cleanup for %s: %v", cleanup.DBName, err)
		} else if len(dbResults) > 0 {
			results[cleanup.DBName] = dbResults
		}
	}

	return results
}

// cleanupRedisCache removes tenant-specific keys from Redis using SCAN + DEL
func (s *OffboardingService) cleanupRedisCache(ctx context.Context, tenantID string) {
	if s.redisClient == nil {
		log.Printf("[OffboardingService] Redis client not initialized, skipping cache cleanup")
		return
	}

	patterns := []string{
		fmt.Sprintf("tenant:%s:*", tenantID),
		fmt.Sprintf("permissions:%s:*", tenantID),
	}

	for _, pattern := range patterns {
		deleted, err := s.redisClient.DeleteByPattern(ctx, pattern)
		if err != nil {
			log.Printf("[OffboardingService] Warning: Failed to clean Redis keys %s: %v", pattern, err)
		} else if deleted > 0 {
			log.Printf("[OffboardingService] Deleted %d Redis keys matching %s", deleted, pattern)
		}
	}
}

// cleanupGrowthBook deletes the tenant's GrowthBook organization
func (s *OffboardingService) cleanupGrowthBook(tenant *models.Tenant) {
	if s.growthBookClient == nil {
		log.Printf("[OffboardingService] GrowthBook client not initialized, skipping org cleanup")
		return
	}

	// GrowthBook org ID is stored as the slug (used as org name during provisioning)
	// We also check the GrowthBookOrgID field if available
	orgID := tenant.Slug
	if tenant.GrowthBookOrgID != "" {
		orgID = tenant.GrowthBookOrgID
	}

	if err := s.growthBookClient.DeleteOrganization(orgID); err != nil {
		log.Printf("[OffboardingService] Warning: Failed to delete GrowthBook org %s: %v", orgID, err)
	} else {
		log.Printf("[OffboardingService] Deleted GrowthBook organization %s for tenant %s", orgID, tenant.ID)
	}
}

// cleanupKeycloakCustomerRealm disables the org and removes redirect URIs in the customer realm
func (s *OffboardingService) cleanupKeycloakCustomerRealm(ctx context.Context, tenant *models.Tenant) {
	if s.keycloakCustomerClient == nil {
		log.Printf("[OffboardingService] Customer realm client not initialized, skipping customer realm cleanup")
		return
	}

	// Disable customer realm organization (same org ID, different realm)
	if tenant.KeycloakOrgID != nil {
		orgID := tenant.KeycloakOrgID.String()
		log.Printf("[OffboardingService] Disabling customer realm Organization %s for tenant %s...", orgID, tenant.ID)

		org, getErr := s.keycloakCustomerClient.GetOrganization(ctx, orgID)
		if getErr != nil {
			log.Printf("[OffboardingService] Warning: Failed to get customer realm Organization %s: %v", orgID, getErr)
		} else if org != nil {
			org.Enabled = false
			org.Description = fmt.Sprintf("[DELETED] %s - Tenant deleted at %s", org.Description, time.Now().Format(time.RFC3339))

			if updateErr := s.keycloakCustomerClient.UpdateOrganization(ctx, orgID, *org); updateErr != nil {
				log.Printf("[OffboardingService] Warning: Failed to disable customer realm Organization %s: %v", orgID, updateErr)
			} else {
				log.Printf("[OffboardingService] Disabled customer realm Organization %s for tenant %s", orgID, tenant.ID)
			}
		}
	}

	// Remove customer realm redirect URIs (storefront clients)
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "tesserix.app"
	}

	// Build redirect URIs that were registered during onboarding
	storefrontWildcard := fmt.Sprintf("https://%s.%s/*", tenant.Slug, baseDomain)
	storefrontCallback := fmt.Sprintf("https://%s.%s/auth/callback", tenant.Slug, baseDomain)
	adminWildcard := fmt.Sprintf("https://%s-admin.%s/*", tenant.Slug, baseDomain)
	adminCallback := fmt.Sprintf("https://%s-admin.%s/auth/callback", tenant.Slug, baseDomain)

	redirectURIs := []string{storefrontWildcard, storefrontCallback, adminWildcard, adminCallback}

	// Remove from marketplace-dashboard client in customer realm
	keycloakCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := s.keycloakCustomerClient.RemoveClientRedirectURIs(keycloakCtx, "marketplace-dashboard", redirectURIs); err != nil {
		log.Printf("[OffboardingService] Warning: Failed to remove customer realm redirect URIs for marketplace-dashboard: %v", err)
	} else {
		log.Printf("[OffboardingService] Removed customer realm redirect URIs from marketplace-dashboard for tenant %s", tenant.Slug)
	}

	// Also remove from storefront-specific clients
	storefrontClientIDsEnv := os.Getenv("KEYCLOAK_STOREFRONT_CLIENT_IDS")
	var storefrontClientIDs []string
	if storefrontClientIDsEnv != "" {
		for _, id := range strings.Split(storefrontClientIDsEnv, ",") {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				storefrontClientIDs = append(storefrontClientIDs, trimmed)
			}
		}
	}
	if len(storefrontClientIDs) == 0 {
		storefrontClientIDs = []string{"storefront-web", "web-storefront", "mobile-app"}
	}

	for _, clientID := range storefrontClientIDs {
		sfCtx, sfCancel := context.WithTimeout(ctx, 15*time.Second)
		if err := s.keycloakCustomerClient.RemoveClientRedirectURIs(sfCtx, clientID, redirectURIs); err != nil {
			log.Printf("[OffboardingService] Warning: Failed to remove customer realm redirect URIs for %s: %v", clientID, err)
		}
		sfCancel()
	}
}

// deleteTenantCore contains the shared deletion logic used by both owner-initiated and admin-initiated deletion.
// It handles archiving, DB cleanup, Keycloak, Redis, GrowthBook, external DBs, and NATS events.
func (s *OffboardingService) deleteTenantCore(ctx context.Context, tenant *models.Tenant, reason string, deletedByUserID uuid.UUID) (*DeleteTenantResponse, error) {
	// Find the owner email for the archive record
	var ownerEmail string
	var ownerUserID uuid.UUID
	for _, membership := range tenant.Memberships {
		if membership.Role == models.MembershipRoleOwner {
			ownerUserID = membership.UserID
			var user models.User
			if err := s.db.WithContext(ctx).First(&user, "id = ?", membership.UserID).Error; err == nil {
				ownerEmail = user.Email
			}
			break
		}
	}

	// === Pre-transaction cleanup ===
	// Delete dependent records OUTSIDE the transaction. PostgreSQL aborts the entire
	// transaction on any SQL error (even caught ones like "relation does not exist"),
	// so each cleanup runs independently — failures don't affect other tables.
	dependentTables := []string{
		"passkey_credentials",
		"tenant_auth_audit_logs",
		"tenant_auth_policies",
		"tenant_credentials",
		"password_reset_tokens",
		"deactivated_memberships",
		"tenant_activity_logs",
		"user_tenant_memberships",
	}

	for _, table := range dependentTables {
		result := s.db.WithContext(ctx).Exec(
			fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table), tenant.ID)
		if result.Error != nil {
			log.Printf("[OffboardingService] Warning: Failed to delete from %s: %v", table, result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("[OffboardingService] Deleted %d rows from %s for tenant %s", result.RowsAffected, table, tenant.ID)
		}
	}

	// Nullify tenant_id in tables with nullable FK references
	nullableFKTables := []string{
		"onboarding_sessions",
		"tenant_slug_reservations",
	}
	for _, table := range nullableFKTables {
		result := s.db.WithContext(ctx).Exec(
			fmt.Sprintf("UPDATE %s SET tenant_id = NULL WHERE tenant_id = $1", table), tenant.ID)
		if result.Error != nil {
			log.Printf("[OffboardingService] Warning: Failed to nullify tenant_id in %s: %v", table, result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("[OffboardingService] Nullified tenant_id in %d rows of %s for tenant %s", result.RowsAffected, table, tenant.ID)
		}
	}

	// === Transaction: archive + delete tenant ===
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create JSON snapshots for archival
	tenantJSON, err := json.Marshal(tenant)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to serialize tenant data: %w", err)
	}

	membershipsJSON, err := json.Marshal(tenant.Memberships)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to serialize memberships data: %w", err)
	}

	vendorsJSON, _ := json.Marshal([]map[string]interface{}{})
	storefrontsJSON, _ := json.Marshal([]map[string]interface{}{})

	// Create archived tenant record
	deletedTenant := &models.DeletedTenant{
		OriginalTenantID: tenant.ID,
		Slug:             tenant.Slug,
		BusinessName:     tenant.Name,
		OwnerUserID:      ownerUserID,
		OwnerEmail:       ownerEmail,
		TenantData:       models.JSONB(tenantJSON),
		MembershipsData:  models.JSONB(membershipsJSON),
		VendorsData:      models.JSONB(vendorsJSON),
		StorefrontsData:  models.JSONB(storefrontsJSON),
		DeletedByUserID:  deletedByUserID,
		DeletionReason:   reason,
	}

	if err := tx.WithContext(ctx).Create(deletedTenant).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create archived tenant record: %w", err)
	}

	log.Printf("[OffboardingService] Archived tenant %s (ID: %s) to deleted_tenants", tenant.Slug, tenant.ID)

	// Hard delete the tenant (FK dependencies already cleared above)
	if err := tx.WithContext(ctx).
		Unscoped().
		Delete(&models.Tenant{}, "id = ?", tenant.ID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to delete tenant: %w", err)
	}

	log.Printf("[OffboardingService] Deleted tenant %s (ID: %s)", tenant.Slug, tenant.ID)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// === Post-commit cleanup (best-effort, errors logged but don't fail deletion) ===

	// Clean up external databases
	externalResults := s.cleanupExternalDatabases(ctx, tenant.ID.String())
	if len(externalResults) > 0 {
		log.Printf("[OffboardingService] External DB cleanup results for tenant %s: %v", tenant.ID, externalResults)
	}

	// Clean up Redis cache
	s.cleanupRedisCache(ctx, tenant.ID.String())

	// Clean up GrowthBook organization
	s.cleanupGrowthBook(tenant)

	// Disable Keycloak Organization in INTERNAL realm
	if s.keycloakClient != nil && tenant.KeycloakOrgID != nil {
		orgID := tenant.KeycloakOrgID.String()
		log.Printf("[OffboardingService] Disabling Keycloak Organization %s for deleted tenant %s...", orgID, tenant.ID)

		org, getErr := s.keycloakClient.GetOrganization(ctx, orgID)
		if getErr != nil {
			log.Printf("[OffboardingService] Warning: Failed to get Keycloak Organization %s: %v", orgID, getErr)
		} else if org != nil {
			org.Enabled = false
			org.Description = fmt.Sprintf("[DELETED] %s - Tenant deleted at %s", org.Description, time.Now().Format(time.RFC3339))

			if updateErr := s.keycloakClient.UpdateOrganization(ctx, orgID, *org); updateErr != nil {
				log.Printf("[OffboardingService] Warning: Failed to disable Keycloak Organization %s: %v", orgID, updateErr)
			} else {
				log.Printf("[OffboardingService] Disabled Keycloak Organization %s for deleted tenant %s", orgID, tenant.ID)
			}
		}
	}

	// Disable Keycloak Organization in CUSTOMER realm + remove redirect URIs
	s.cleanupKeycloakCustomerRealm(ctx, tenant)

	// Remove redirect URIs from INTERNAL realm
	if s.keycloakClient != nil {
		baseDomain := os.Getenv("BASE_DOMAIN")
		if baseDomain == "" {
			baseDomain = "tesserix.app"
		}

		adminWildcard := fmt.Sprintf("https://%s-admin.%s/*", tenant.Slug, baseDomain)
		adminCallback := fmt.Sprintf("https://%s-admin.%s/auth/callback", tenant.Slug, baseDomain)
		storefrontWildcard := fmt.Sprintf("https://%s.%s/*", tenant.Slug, baseDomain)
		storefrontCallback := fmt.Sprintf("https://%s.%s/auth/callback", tenant.Slug, baseDomain)

		redirectURIsToRemove := []string{adminWildcard, adminCallback, storefrontWildcard, storefrontCallback}

		keycloakCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := s.keycloakClient.RemoveClientRedirectURIs(keycloakCtx, "marketplace-dashboard", redirectURIsToRemove); err != nil {
			log.Printf("[OffboardingService] WARNING: Failed to remove internal realm redirect URIs for tenant %s: %v", tenant.Slug, err)
		} else {
			log.Printf("[OffboardingService] Removed internal realm redirect URIs for tenant %s", tenant.Slug)
		}
	}

	// Publish NATS event for K8s resource cleanup
	if s.natsClient != nil {
		baseDomain := os.Getenv("BASE_DOMAIN")
		if baseDomain == "" {
			baseDomain = "tesserix.app"
		}

		event := &natsClient.TenantDeletedEvent{
			TenantID:       tenant.ID.String(),
			Slug:           tenant.Slug,
			AdminHost:      fmt.Sprintf("%s-admin.%s", tenant.Slug, baseDomain),
			StorefrontHost: fmt.Sprintf("%s.%s", tenant.Slug, baseDomain),
		}

		publishCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.natsClient.PublishTenantDeleted(publishCtx, event); err != nil {
			log.Printf("[OffboardingService] WARNING: Failed to publish tenant.deleted event: %v - K8s resources will be cleaned up on reconciliation", err)
		} else {
			log.Printf("[OffboardingService] Published tenant.deleted event for %s", tenant.Slug)
		}
	} else {
		log.Printf("[OffboardingService] WARNING: NATS client not initialized, tenant.deleted event not published")
	}

	return &DeleteTenantResponse{
		Message:          "Tenant deleted successfully",
		TenantID:         tenant.ID.String(),
		ArchivedRecordID: deletedTenant.ID.String(),
		ArchivedAt:       deletedTenant.DeletedAt,
	}, nil
}

// DeleteTenant deletes a tenant and archives all data for audit purposes (owner-initiated)
func (s *OffboardingService) DeleteTenant(ctx context.Context, req *DeleteTenantRequest) (*DeleteTenantResponse, error) {
	// Get the tenant with memberships
	var tenant models.Tenant
	if err := s.db.WithContext(ctx).
		Preload("Memberships").
		First(&tenant, "id = ?", req.TenantID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Verify the user is the tenant owner
	isOwner := false
	for _, membership := range tenant.Memberships {
		if membership.UserID == req.UserID && membership.Role == models.MembershipRoleOwner {
			isOwner = true
			break
		}
	}

	if !isOwner {
		return nil, fmt.Errorf("only the tenant owner can delete the tenant")
	}

	// Verify confirmation text
	expectedConfirmation := fmt.Sprintf("DELETE %s", tenant.Slug)
	if req.ConfirmationText != expectedConfirmation {
		return nil, fmt.Errorf("invalid confirmation text: expected '%s'", expectedConfirmation)
	}

	return s.deleteTenantCore(ctx, &tenant, req.Reason, req.UserID)
}

// AdminDeleteTenant deletes a tenant initiated by a platform admin (no owner check, no confirmation text)
func (s *OffboardingService) AdminDeleteTenant(ctx context.Context, req *AdminDeleteTenantRequest) (*DeleteTenantResponse, error) {
	var tenant models.Tenant
	if err := s.db.WithContext(ctx).
		Preload("Memberships").
		First(&tenant, "id = ?", req.TenantID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// For admin-initiated deletion, use a zero UUID as the deletedBy user
	// (platform admin, not a specific tenant user)
	adminUserID := uuid.Nil

	return s.deleteTenantCore(ctx, &tenant, req.Reason, adminUserID)
}

// BatchDeleteTenants deletes multiple tenants, collecting successes and failures
func (s *OffboardingService) BatchDeleteTenants(ctx context.Context, tenantIDs []uuid.UUID, reason string) *BatchDeleteResult {
	result := &BatchDeleteResult{
		Succeeded: make([]string, 0),
		Failed:    make([]BatchDeleteFailure, 0),
	}

	for _, tenantID := range tenantIDs {
		_, err := s.AdminDeleteTenant(ctx, &AdminDeleteTenantRequest{
			TenantID: tenantID,
			Reason:   reason,
		})
		if err != nil {
			// Try to get the slug for better error reporting
			var tenant models.Tenant
			slug := ""
			if findErr := s.db.WithContext(ctx).Select("slug").First(&tenant, "id = ?", tenantID).Error; findErr == nil {
				slug = tenant.Slug
			}
			result.Failed = append(result.Failed, BatchDeleteFailure{
				ID:    tenantID.String(),
				Slug:  slug,
				Error: err.Error(),
			})
		} else {
			result.Succeeded = append(result.Succeeded, tenantID.String())
		}
	}

	return result
}

// DeleteAllTenants fetches all active tenants and deletes them all
func (s *OffboardingService) DeleteAllTenants(ctx context.Context, reason string) (*BatchDeleteResult, int, error) {
	var tenants []models.Tenant
	if err := s.db.WithContext(ctx).Find(&tenants).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch tenants: %w", err)
	}

	total := len(tenants)
	ids := make([]uuid.UUID, len(tenants))
	for i, t := range tenants {
		ids[i] = t.ID
	}

	result := s.BatchDeleteTenants(ctx, ids, reason)
	return result, total, nil
}

// GetDeletionPreview returns counts of data that would be deleted per database (dry run)
func (s *OffboardingService) GetDeletionPreview(ctx context.Context, tenantIDs []uuid.UUID) ([]DeletionPreview, error) {
	previews := make([]DeletionPreview, 0, len(tenantIDs))

	for _, tenantID := range tenantIDs {
		var tenant models.Tenant
		if err := s.db.WithContext(ctx).
			Preload("Memberships").
			First(&tenant, "id = ?", tenantID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, fmt.Errorf("failed to get tenant %s: %w", tenantID, err)
		}

		preview := DeletionPreview{
			TenantID: tenant.ID.String(),
			Slug:     tenant.Slug,
			Name:     tenant.Name,
			Counts:   make(map[string]map[string]int),
		}

		// Count in main (tesseract_hub) database
		hubCounts := make(map[string]int)
		hubCounts["memberships"] = len(tenant.Memberships)

		var vendorCount int64
		s.db.WithContext(ctx).Table("vendors").Where("tenant_id = ?", tenantID.String()).Count(&vendorCount)
		hubCounts["vendors"] = int(vendorCount)

		var storefrontCount int64
		s.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM storefronts WHERE vendor_id IN (SELECT id FROM vendors WHERE tenant_id = ?)", tenantID.String()).Scan(&storefrontCount)
		hubCounts["storefronts"] = int(storefrontCount)

		preview.Counts["tesseract_hub"] = hubCounts

		// Count in external databases
		for _, cleanup := range getExternalDBCleanups() {
			extDB, err := connectExternalDB(cleanup.DBName)
			if err != nil {
				continue
			}
			sqlDB, _ := extDB.DB()

			dbCounts := make(map[string]int)
			for _, table := range cleanup.Tables {
				var count int64
				if err := extDB.WithContext(ctx).Raw(
					fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE tenant_id = ?", table),
					tenantID.String(),
				).Scan(&count).Error; err == nil && count > 0 {
					dbCounts[table] = int(count)
				}
			}

			if len(dbCounts) > 0 {
				preview.Counts[cleanup.DBName] = dbCounts
			}

			sqlDB.Close()
		}

		previews = append(previews, preview)
	}

	return previews, nil
}

// DB returns the underlying database connection (used by handlers for simple queries)
func (s *OffboardingService) DB() *gorm.DB {
	return s.db
}

// GetTenantForDeletion gets tenant information needed for the deletion UI
func (s *OffboardingService) GetTenantForDeletion(ctx context.Context, tenantID, userID uuid.UUID) (map[string]interface{}, error) {
	// Get the tenant
	var tenant models.Tenant
	if err := s.db.WithContext(ctx).
		Preload("Memberships").
		First(&tenant, "id = ?", tenantID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Verify the user is the tenant owner
	isOwner := false
	for _, membership := range tenant.Memberships {
		if membership.UserID == userID && membership.Role == models.MembershipRoleOwner {
			isOwner = true
			break
		}
	}

	if !isOwner {
		return nil, fmt.Errorf("only the tenant owner can view deletion information")
	}

	// Count resources that will be affected
	memberCount := len(tenant.Memberships)

	return map[string]interface{}{
		"tenant_id":             tenant.ID.String(),
		"name":                  tenant.Name,
		"slug":                  tenant.Slug,
		"member_count":          memberCount,
		"created_at":            tenant.CreatedAt,
		"is_owner":              isOwner,
		"confirmation_required": fmt.Sprintf("DELETE %s", tenant.Slug),
	}, nil
}

// ListDeletedTenants lists all archived/deleted tenants (admin function)
func (s *OffboardingService) ListDeletedTenants(ctx context.Context, limit, offset int) ([]models.DeletedTenant, int64, error) {
	var deletedTenants []models.DeletedTenant
	var total int64

	// Count total
	if err := s.db.WithContext(ctx).Model(&models.DeletedTenant{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count deleted tenants: %w", err)
	}

	// Get paginated results
	if err := s.db.WithContext(ctx).
		Order("deleted_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&deletedTenants).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list deleted tenants: %w", err)
	}

	return deletedTenants, total, nil
}

// GetDeletedTenant gets a specific archived tenant record
func (s *OffboardingService) GetDeletedTenant(ctx context.Context, id uuid.UUID) (*models.DeletedTenant, error) {
	var deletedTenant models.DeletedTenant
	if err := s.db.WithContext(ctx).First(&deletedTenant, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("deleted tenant record not found")
		}
		return nil, fmt.Errorf("failed to get deleted tenant: %w", err)
	}
	return &deletedTenant, nil
}
