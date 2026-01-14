package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tenant-router-service/internal/config"
	"tenant-router-service/internal/models"
)

// NewConnection creates a new database connection
func NewConnection(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
	)

	// Configure GORM logger based on environment
	logLevel := logger.Warn // Default to warn level

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying sql.DB for connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	log.Printf("Connected to database: %s:%s/%s", cfg.Host, cfg.Port, cfg.Name)

	return db, nil
}

// AutoMigrate runs database migrations
func AutoMigrate(db *gorm.DB) error {
	log.Println("Running database migrations...")

	// Enable UUID extension
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		log.Printf("Warning: could not create uuid-ossp extension (may already exist): %v", err)
	}

	// Migrate models
	modelsToMigrate := []interface{}{
		&models.TenantHostRecord{},
		&models.ProvisioningActivityLog{},
	}

	for _, model := range modelsToMigrate {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
		log.Printf("Migrated: %T", model)
	}

	// Create additional indexes if they don't exist
	if err := createIndexes(db); err != nil {
		log.Printf("Warning: could not create additional indexes: %v", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// createIndexes creates additional database indexes
func createIndexes(db *gorm.DB) error {
	indexes := []string{
		// Composite index for listing by status with ordering
		`CREATE INDEX IF NOT EXISTS idx_tenant_host_status_created
		 ON tenant_host_records (status, created_at DESC)`,

		// Index for finding failed records eligible for retry
		`CREATE INDEX IF NOT EXISTS idx_tenant_host_retry
		 ON tenant_host_records (status, retry_count, last_retry_at)
		 WHERE status = 'failed'`,

		// Index for activity logs lookup
		`CREATE INDEX IF NOT EXISTS idx_activity_created
		 ON provisioning_activity_logs (tenant_host_id, created_at DESC)`,
	}

	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			return err
		}
	}

	return nil
}

// HealthCheck verifies database connectivity
func HealthCheck(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
