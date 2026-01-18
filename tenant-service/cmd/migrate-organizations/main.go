// Package main implements a migration script to create Keycloak Organizations
// for existing tenants and sync their members.
//
// Usage:
//
//	./migrate-organizations --dry-run          # Preview changes without applying
//	./migrate-organizations                     # Execute migration
//	./migrate-organizations --tenant=<slug>    # Migrate specific tenant only
//
// Environment Variables:
//
//	DATABASE_URL           - PostgreSQL connection string
//	KEYCLOAK_BASE_URL      - Keycloak base URL (e.g., https://devtest-customer-idp.tesserix.app)
//	KEYCLOAK_REALM         - Keycloak realm (e.g., tesserix-customer)
//	KEYCLOAK_ADMIN_CLIENT_ID     - Admin client ID
//	KEYCLOAK_ADMIN_CLIENT_SECRET - Admin client secret
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Tesseract-Nexus/go-shared/auth"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Tenant represents the tenant model (simplified for migration)
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

// UserTenantMembership represents a user's membership in a tenant
type UserTenantMembership struct {
	UserID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Role     string    `gorm:"column:role"`
	IsActive bool      `gorm:"column:is_active"`
}

func (UserTenantMembership) TableName() string {
	return "user_tenant_memberships"
}

// User represents a user (simplified for migration)
type User struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Email      string     `gorm:"column:email"`
	KeycloakID *uuid.UUID `gorm:"column:keycloak_id"`
}

func (User) TableName() string {
	return "tenant_users"
}

// MigrationStats tracks migration progress
type MigrationStats struct {
	TenantsFound       int
	TenantsProcessed   int
	TenantsSkipped     int
	TenantsFailed      int
	OrgsCreated        int
	MembersAdded       int
	MembersFailed      int
	StartTime          time.Time
	EndTime            time.Time
}

func (s *MigrationStats) Print() {
	duration := s.EndTime.Sub(s.StartTime)
	fmt.Println("\n========================================")
	fmt.Println("MIGRATION SUMMARY")
	fmt.Println("========================================")
	fmt.Printf("Duration:           %v\n", duration.Round(time.Second))
	fmt.Printf("Tenants Found:      %d\n", s.TenantsFound)
	fmt.Printf("Tenants Processed:  %d\n", s.TenantsProcessed)
	fmt.Printf("Tenants Skipped:    %d\n", s.TenantsSkipped)
	fmt.Printf("Tenants Failed:     %d\n", s.TenantsFailed)
	fmt.Printf("Orgs Created:       %d\n", s.OrgsCreated)
	fmt.Printf("Members Added:      %d\n", s.MembersAdded)
	fmt.Printf("Members Failed:     %d\n", s.MembersFailed)
	fmt.Println("========================================")
}

func main() {
	// Parse command line flags
	dryRun := flag.Bool("dry-run", false, "Preview changes without applying them")
	tenantSlug := flag.String("tenant", "", "Migrate specific tenant only (by slug)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if *dryRun {
		log.Println("=== DRY RUN MODE - No changes will be made ===")
	}

	// Initialize database connection
	db, err := initDatabase(*verbose)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Initialize Keycloak client
	keycloakClient, err := initKeycloakClient()
	if err != nil {
		log.Fatalf("Failed to initialize Keycloak client: %v", err)
	}

	// Run migration
	stats := &MigrationStats{StartTime: time.Now()}
	ctx := context.Background()

	if err := runMigration(ctx, db, keycloakClient, *dryRun, *tenantSlug, stats); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	stats.EndTime = time.Now()
	stats.Print()

	if stats.TenantsFailed > 0 {
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

func initKeycloakClient() (*auth.KeycloakAdminClient, error) {
	baseURL := os.Getenv("KEYCLOAK_BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("KEYCLOAK_BASE_URL environment variable is required")
	}

	realm := os.Getenv("KEYCLOAK_REALM")
	if realm == "" {
		return nil, fmt.Errorf("KEYCLOAK_REALM environment variable is required")
	}

	clientID := os.Getenv("KEYCLOAK_ADMIN_CLIENT_ID")
	if clientID == "" {
		clientID = "admin-cli"
	}

	clientSecret := os.Getenv("KEYCLOAK_ADMIN_CLIENT_SECRET")
	if clientSecret == "" {
		return nil, fmt.Errorf("KEYCLOAK_ADMIN_CLIENT_SECRET environment variable is required")
	}

	client := auth.NewKeycloakAdminClient(auth.KeycloakAdminConfig{
		BaseURL:      baseURL,
		Realm:        realm,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Timeout:      30 * time.Second,
	})

	log.Printf("Keycloak client initialized for %s/realms/%s", baseURL, realm)
	return client, nil
}

func runMigration(ctx context.Context, db *gorm.DB, kc *auth.KeycloakAdminClient, dryRun bool, tenantSlug string, stats *MigrationStats) error {
	// Find tenants to migrate
	var tenants []Tenant
	query := db.Where("status = ? AND keycloak_org_id IS NULL", "active")

	if tenantSlug != "" {
		query = query.Where("slug = ?", tenantSlug)
	}

	if err := query.Find(&tenants).Error; err != nil {
		return fmt.Errorf("failed to query tenants: %w", err)
	}

	stats.TenantsFound = len(tenants)
	log.Printf("Found %d tenants to migrate", len(tenants))

	if len(tenants) == 0 {
		log.Println("No tenants need migration")
		return nil
	}

	// Process each tenant
	for i, tenant := range tenants {
		log.Printf("\n[%d/%d] Processing tenant: %s (ID: %s)", i+1, len(tenants), tenant.Slug, tenant.ID)

		if err := migrateTenant(ctx, db, kc, &tenant, dryRun, stats); err != nil {
			log.Printf("ERROR: Failed to migrate tenant %s: %v", tenant.Slug, err)
			stats.TenantsFailed++
			// Continue with next tenant
			continue
		}

		stats.TenantsProcessed++
	}

	return nil
}

func migrateTenant(ctx context.Context, db *gorm.DB, kc *auth.KeycloakAdminClient, tenant *Tenant, dryRun bool, stats *MigrationStats) error {
	// Step 1: Create Keycloak Organization
	log.Printf("  Creating Keycloak Organization for tenant: %s", tenant.Name)

	var orgID string
	if dryRun {
		orgID = "dry-run-org-id"
		log.Printf("  [DRY RUN] Would create organization: name=%s, alias=%s", tenant.Name, tenant.Slug)
	} else {
		var err error
		orgID, err = kc.CreateOrganizationForTenant(ctx, tenant.ID.String(), tenant.Name, tenant.Slug)
		if err != nil {
			return fmt.Errorf("failed to create organization: %w", err)
		}
		log.Printf("  Created organization: %s", orgID)
		stats.OrgsCreated++
	}

	// Step 2: Update tenant with org ID
	if !dryRun {
		orgUUID, err := uuid.Parse(orgID)
		if err != nil {
			return fmt.Errorf("failed to parse org ID: %w", err)
		}

		if err := db.Model(&Tenant{}).Where("id = ?", tenant.ID).Update("keycloak_org_id", orgUUID).Error; err != nil {
			// Rollback: delete the org we just created
			log.Printf("  WARNING: Failed to update tenant, deleting organization...")
			_ = kc.DeleteOrganization(ctx, orgID)
			return fmt.Errorf("failed to update tenant: %w", err)
		}
		log.Printf("  Updated tenant with keycloak_org_id: %s", orgID)
	} else {
		log.Printf("  [DRY RUN] Would update tenant with keycloak_org_id")
	}

	// Step 3: Get active members for this tenant
	var memberships []UserTenantMembership
	if err := db.Where("tenant_id = ? AND is_active = ?", tenant.ID, true).Find(&memberships).Error; err != nil {
		return fmt.Errorf("failed to get memberships: %w", err)
	}

	log.Printf("  Found %d active members to sync", len(memberships))

	// Step 4: Add each member to the organization
	for _, membership := range memberships {
		// Get user's Keycloak ID
		var user User
		if err := db.Where("id = ?", membership.UserID).First(&user).Error; err != nil {
			log.Printf("  WARNING: Could not find user %s: %v", membership.UserID, err)
			stats.MembersFailed++
			continue
		}

		if user.KeycloakID == nil {
			log.Printf("  WARNING: User %s has no Keycloak ID, skipping", user.Email)
			stats.MembersFailed++
			continue
		}

		keycloakUserID := user.KeycloakID.String()

		if dryRun {
			log.Printf("  [DRY RUN] Would add member: %s (Keycloak: %s, Role: %s)", user.Email, keycloakUserID, membership.Role)
			stats.MembersAdded++
		} else {
			if err := kc.AddOrganizationMember(ctx, orgID, keycloakUserID); err != nil {
				log.Printf("  WARNING: Failed to add member %s to organization: %v", user.Email, err)
				stats.MembersFailed++
				continue
			}
			log.Printf("  Added member: %s (Role: %s)", user.Email, membership.Role)
			stats.MembersAdded++
		}
	}

	return nil
}
