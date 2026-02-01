// Package main implements a migration script for the dual-realm architecture.
// This migrates staff/admin users from customer realm to internal realm and
// configures redirect URIs for both realms.
//
// Usage:
//
//	./migrate-dual-realm --dry-run          # Preview changes without applying
//	./migrate-dual-realm                    # Execute full migration
//	./migrate-dual-realm --tenant=<slug>    # Migrate specific tenant only
//	./migrate-dual-realm --users-only       # Only migrate users
//	./migrate-dual-realm --redirects-only   # Only fix redirect URIs
//
// Environment Variables:
//
//	DATABASE_URL                    - PostgreSQL connection string
//	KEYCLOAK_CUSTOMER_URL           - Customer IDP URL
//	KEYCLOAK_CUSTOMER_REALM         - Customer realm name (e.g., tesserix-customer)
//	KEYCLOAK_CUSTOMER_CLIENT_ID     - Customer admin client ID
//	KEYCLOAK_CUSTOMER_CLIENT_SECRET - Customer admin client secret
//	KEYCLOAK_INTERNAL_URL           - Internal IDP URL
//	KEYCLOAK_INTERNAL_REALM         - Internal realm name (e.g., tesserix-internal)
//	KEYCLOAK_INTERNAL_CLIENT_ID     - Internal admin client ID
//	KEYCLOAK_INTERNAL_CLIENT_SECRET - Internal admin client secret
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Tesseract-Nexus/go-shared/auth"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Staff represents a staff member in the tenant database
type Staff struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primaryKey"`
	TenantID           uuid.UUID  `gorm:"type:uuid"`
	Email              string     `gorm:"column:email"`
	FirstName          string     `gorm:"column:first_name"`
	LastName           string     `gorm:"column:last_name"`
	Role               string     `gorm:"column:role"`
	IsActive           bool       `gorm:"column:is_active"`
	KeycloakID         *uuid.UUID `gorm:"column:keycloak_id"`
	KeycloakInternalID *uuid.UUID `gorm:"column:keycloak_internal_id"`
}

func (Staff) TableName() string {
	return "staff"
}

// Tenant represents a tenant in the database
type Tenant struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Name          string     `gorm:"column:name"`
	Slug          string     `gorm:"column:slug"`
	Status        string     `gorm:"column:status"`
	KeycloakOrgID *uuid.UUID `gorm:"column:keycloak_org_id"`
}

func (Tenant) TableName() string {
	return "tenants"
}

// CustomDomain represents a custom domain for a tenant
type CustomDomain struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid"`
	Domain   string    `gorm:"column:domain"`
	Type     string    `gorm:"column:type"` // "storefront" or "admin"
	Status   string    `gorm:"column:status"`
}

func (CustomDomain) TableName() string {
	return "custom_domains"
}

// MigrationStats tracks migration progress
type MigrationStats struct {
	StartTime         time.Time
	EndTime           time.Time
	TenantsFound      int
	TenantsProcessed  int
	TenantsFailed     int
	UsersFound        int
	UsersMigrated     int
	UsersSkipped      int
	UsersFailed       int
	RedirectsAdded    int
	RedirectsFailed   int
}

func (s *MigrationStats) Print() {
	duration := s.EndTime.Sub(s.StartTime)
	fmt.Println("\n========================================")
	fmt.Println("DUAL-REALM MIGRATION SUMMARY")
	fmt.Println("========================================")
	fmt.Printf("Duration:           %v\n", duration.Round(time.Second))
	fmt.Printf("Tenants Found:      %d\n", s.TenantsFound)
	fmt.Printf("Tenants Processed:  %d\n", s.TenantsProcessed)
	fmt.Printf("Tenants Failed:     %d\n", s.TenantsFailed)
	fmt.Println("----------------------------------------")
	fmt.Printf("Users Found:        %d\n", s.UsersFound)
	fmt.Printf("Users Migrated:     %d\n", s.UsersMigrated)
	fmt.Printf("Users Skipped:      %d\n", s.UsersSkipped)
	fmt.Printf("Users Failed:       %d\n", s.UsersFailed)
	fmt.Println("----------------------------------------")
	fmt.Printf("Redirects Added:    %d\n", s.RedirectsAdded)
	fmt.Printf("Redirects Failed:   %d\n", s.RedirectsFailed)
	fmt.Println("========================================")
}

type Config struct {
	DryRun        bool
	TenantSlug    string
	Verbose       bool
	UsersOnly     bool
	RedirectsOnly bool
}

func main() {
	// Parse command line flags
	config := Config{}
	flag.BoolVar(&config.DryRun, "dry-run", false, "Preview changes without applying them")
	flag.StringVar(&config.TenantSlug, "tenant", "", "Migrate specific tenant only (by slug)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&config.UsersOnly, "users-only", false, "Only migrate users to internal realm")
	flag.BoolVar(&config.RedirectsOnly, "redirects-only", false, "Only fix redirect URIs")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if config.DryRun {
		log.Println("=== DRY RUN MODE - No changes will be made ===")
	}

	// Initialize database
	db, err := initDatabase(config.Verbose)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize Keycloak clients
	customerClient, err := initKeycloakClient("CUSTOMER")
	if err != nil {
		log.Fatalf("Failed to initialize customer Keycloak client: %v", err)
	}

	internalClient, err := initKeycloakClient("INTERNAL")
	if err != nil {
		log.Fatalf("Failed to initialize internal Keycloak client: %v", err)
	}

	// Run migration
	ctx := context.Background()
	stats := &MigrationStats{StartTime: time.Now()}

	if err := runMigration(ctx, db, customerClient, internalClient, config, stats); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	stats.EndTime = time.Now()
	stats.Print()

	if stats.TenantsFailed > 0 || stats.UsersFailed > 0 {
		os.Exit(1)
	}
}

func initDatabase(verbose bool) (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	logLevel := logger.Silent
	if verbose {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("Connected to database")
	return db, nil
}

func initKeycloakClient(prefix string) (*auth.KeycloakAdminClient, error) {
	baseURL := os.Getenv(fmt.Sprintf("KEYCLOAK_%s_URL", prefix))
	if baseURL == "" {
		return nil, fmt.Errorf("KEYCLOAK_%s_URL environment variable is required", prefix)
	}

	realm := os.Getenv(fmt.Sprintf("KEYCLOAK_%s_REALM", prefix))
	if realm == "" {
		return nil, fmt.Errorf("KEYCLOAK_%s_REALM environment variable is required", prefix)
	}

	clientID := os.Getenv(fmt.Sprintf("KEYCLOAK_%s_CLIENT_ID", prefix))
	if clientID == "" {
		clientID = "admin-cli"
	}

	clientSecret := os.Getenv(fmt.Sprintf("KEYCLOAK_%s_CLIENT_SECRET", prefix))
	if clientSecret == "" {
		return nil, fmt.Errorf("KEYCLOAK_%s_CLIENT_SECRET environment variable is required", prefix)
	}

	client := auth.NewKeycloakAdminClient(auth.KeycloakAdminConfig{
		BaseURL:      baseURL,
		Realm:        realm,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Timeout:      30 * time.Second,
	})

	log.Printf("Keycloak %s client initialized for %s/realms/%s", prefix, baseURL, realm)
	return client, nil
}

func runMigration(ctx context.Context, db *gorm.DB, customerClient, internalClient *auth.KeycloakAdminClient, config Config, stats *MigrationStats) error {
	// Find tenants to migrate
	var tenants []Tenant
	query := db.Where("status = ?", "active")

	if config.TenantSlug != "" {
		query = query.Where("slug = ?", config.TenantSlug)
	}

	if err := query.Find(&tenants).Error; err != nil {
		return fmt.Errorf("failed to query tenants: %w", err)
	}

	stats.TenantsFound = len(tenants)
	log.Printf("Found %d active tenants to process", len(tenants))

	if len(tenants) == 0 {
		log.Println("No tenants to process")
		return nil
	}

	for i, tenant := range tenants {
		log.Printf("\n[%d/%d] Processing tenant: %s (ID: %s)", i+1, len(tenants), tenant.Slug, tenant.ID)

		if err := processTenant(ctx, db, customerClient, internalClient, &tenant, config, stats); err != nil {
			log.Printf("ERROR: Failed to process tenant %s: %v", tenant.Slug, err)
			stats.TenantsFailed++
			continue
		}

		stats.TenantsProcessed++
	}

	return nil
}

func processTenant(ctx context.Context, db *gorm.DB, customerClient, internalClient *auth.KeycloakAdminClient, tenant *Tenant, config Config, stats *MigrationStats) error {
	// Step 1: Migrate staff users to internal realm
	if !config.RedirectsOnly {
		if err := migrateStaffUsers(ctx, db, customerClient, internalClient, tenant, config, stats); err != nil {
			return fmt.Errorf("failed to migrate users: %w", err)
		}
	}

	// Step 2: Add redirect URIs for this tenant
	if !config.UsersOnly {
		if err := addRedirectURIs(ctx, db, customerClient, internalClient, tenant, config, stats); err != nil {
			return fmt.Errorf("failed to add redirect URIs: %w", err)
		}
	}

	return nil
}

func migrateStaffUsers(ctx context.Context, db *gorm.DB, customerClient, internalClient *auth.KeycloakAdminClient, tenant *Tenant, config Config, stats *MigrationStats) error {
	// Get all active staff for this tenant who haven't been migrated yet
	var staffMembers []Staff
	query := db.Where("tenant_id = ? AND is_active = ? AND keycloak_id IS NOT NULL", tenant.ID, true)

	// Only get users not yet migrated (keycloak_internal_id is null)
	query = query.Where("keycloak_internal_id IS NULL")

	if err := query.Find(&staffMembers).Error; err != nil {
		return fmt.Errorf("failed to query staff: %w", err)
	}

	stats.UsersFound += len(staffMembers)
	log.Printf("  Found %d staff members to migrate", len(staffMembers))

	for _, staff := range staffMembers {
		log.Printf("  Processing: %s (%s)", staff.Email, staff.Role)

		// Get user from customer realm
		user, err := customerClient.GetUserByID(ctx, staff.KeycloakID.String())
		if err != nil {
			log.Printf("    WARNING: Could not find user in customer realm: %v", err)
			stats.UsersFailed++
			continue
		}

		// Check if user already exists in internal realm
		existingUser, _ := internalClient.GetUserByEmail(ctx, staff.Email)
		if existingUser != nil {
			log.Printf("    User already exists in internal realm, updating database")
			if !config.DryRun {
				internalID, _ := uuid.Parse(existingUser.ID)
				db.Model(&Staff{}).Where("id = ?", staff.ID).Update("keycloak_internal_id", internalID)
			}
			stats.UsersSkipped++
			continue
		}

		if config.DryRun {
			log.Printf("    [DRY RUN] Would migrate user to internal realm")
			stats.UsersMigrated++
			continue
		}

		// Create user in internal realm
		newUser := auth.UserRepresentation{
			Username:      user.Email,
			Email:         user.Email,
			FirstName:     user.FirstName,
			LastName:      user.LastName,
			Enabled:       true,
			EmailVerified: true,
			Attributes: map[string][]string{
				"tenant_id":   {tenant.ID.String()},
				"tenant_slug": {tenant.Slug},
				"staff_id":    {staff.ID.String()},
			},
		}

		// Map role from customer realm to internal realm
		internalRole := mapRoleToInternal(staff.Role)

		createdUserID, err := internalClient.CreateUser(ctx, newUser)
		if err != nil {
			log.Printf("    ERROR: Failed to create user in internal realm: %v", err)
			stats.UsersFailed++
			continue
		}

		// Assign role in internal realm
		if err := internalClient.AssignRealmRole(ctx, createdUserID, internalRole); err != nil {
			log.Printf("    WARNING: Failed to assign role %s: %v", internalRole, err)
		}

		// Update database with internal realm user ID
		internalID, _ := uuid.Parse(createdUserID)
		if err := db.Model(&Staff{}).Where("id = ?", staff.ID).Update("keycloak_internal_id", internalID).Error; err != nil {
			log.Printf("    WARNING: Failed to update database: %v", err)
		}

		log.Printf("    Migrated to internal realm (ID: %s, Role: %s)", createdUserID, internalRole)
		stats.UsersMigrated++
	}

	return nil
}

func mapRoleToInternal(customerRole string) string {
	// Map customer realm roles to internal realm roles
	roleMapping := map[string]string{
		"owner":     "tenant_admin",
		"admin":     "admin",
		"manager":   "staff",
		"staff":     "staff",
		"employee":  "employee",
		"vendor":    "staff",
	}

	if internalRole, ok := roleMapping[strings.ToLower(customerRole)]; ok {
		return internalRole
	}
	return "staff" // Default to staff role
}

func addRedirectURIs(ctx context.Context, db *gorm.DB, customerClient, internalClient *auth.KeycloakAdminClient, tenant *Tenant, config Config, stats *MigrationStats) error {
	// Standard subdomain redirect URIs
	storefrontURI := fmt.Sprintf("https://%s.tesserix.app/*", tenant.Slug)
	adminURI := fmt.Sprintf("https://%s-admin.tesserix.app/*", tenant.Slug)

	log.Printf("  Adding redirect URIs:")
	log.Printf("    Storefront: %s", storefrontURI)
	log.Printf("    Admin: %s", adminURI)

	// Add storefront URI to customer realm clients
	storefrontClients := []string{"storefront-web", "web-storefront", "mobile-app"}
	for _, clientID := range storefrontClients {
		if err := addRedirectURIToClient(ctx, customerClient, clientID, storefrontURI, config.DryRun, stats); err != nil {
			log.Printf("    WARNING: Failed to add URI to %s: %v", clientID, err)
		}
	}

	// Add admin URI to internal realm clients
	adminClients := []string{"admin-web", "marketplace-dashboard", "admin-bff"}
	for _, clientID := range adminClients {
		if err := addRedirectURIToClient(ctx, internalClient, clientID, adminURI, config.DryRun, stats); err != nil {
			log.Printf("    WARNING: Failed to add URI to %s: %v", clientID, err)
		}
	}

	// Handle custom domains
	var customDomains []CustomDomain
	db.Where("tenant_id = ? AND status = ?", tenant.ID, "active").Find(&customDomains)

	for _, domain := range customDomains {
		domainURI := fmt.Sprintf("https://%s/*", domain.Domain)
		log.Printf("    Custom domain (%s): %s", domain.Type, domainURI)

		if domain.Type == "storefront" {
			for _, clientID := range storefrontClients {
				addRedirectURIToClient(ctx, customerClient, clientID, domainURI, config.DryRun, stats)
			}
		} else if domain.Type == "admin" {
			for _, clientID := range adminClients {
				addRedirectURIToClient(ctx, internalClient, clientID, domainURI, config.DryRun, stats)
			}
		}
	}

	return nil
}

func addRedirectURIToClient(ctx context.Context, kc *auth.KeycloakAdminClient, clientID, redirectURI string, dryRun bool, stats *MigrationStats) error {
	if dryRun {
		log.Printf("      [DRY RUN] Would add %s to client %s", redirectURI, clientID)
		stats.RedirectsAdded++
		return nil
	}

	// Get current client config
	client, err := kc.GetClientByClientID(ctx, clientID)
	if err != nil {
		stats.RedirectsFailed++
		return fmt.Errorf("client not found: %w", err)
	}

	// Check if redirect URI already exists
	for _, existing := range client.RedirectUris {
		if existing == redirectURI {
			log.Printf("      URI already exists in %s", clientID)
			return nil
		}
	}

	// Add new redirect URI
	client.RedirectUris = append(client.RedirectUris, redirectURI)

	if err := kc.UpdateClient(ctx, client.ID, *client); err != nil {
		stats.RedirectsFailed++
		return fmt.Errorf("failed to update client: %w", err)
	}

	log.Printf("      Added to %s", clientID)
	stats.RedirectsAdded++
	return nil
}
