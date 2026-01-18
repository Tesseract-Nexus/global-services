// Package fdw provides PostgreSQL Foreign Data Wrapper (FDW) initialization and management.
// This package ensures that the analytics service can query data from other service databases
// (orders, customers, products) through PostgreSQL FDW foreign tables.
//
// The FDW setup is designed to be:
// - Idempotent: Safe to run on every service startup
// - Self-healing: Automatically creates missing components
// - Fail-fast: Returns clear errors if setup cannot be completed
// - Robust: Handles edge cases and provides detailed logging
package fdw

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// FDWConfig holds configuration for Foreign Data Wrapper setup
type FDWConfig struct {
	// Database connection details (used for FDW server options)
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string

	// FDW Server Host - the host used by PostgreSQL for FDW connections
	// IMPORTANT: When all databases (analytics_db, products_db, orders_db, customers_db)
	// are on the SAME PostgreSQL instance, this should be "localhost" because
	// FDW connections happen FROM PostgreSQL, not from the application.
	// If databases are on different servers, use the actual hostnames.
	FDWServerHost string

	// Source database names
	ProductsDB  string
	OrdersDB    string
	CustomersDB string

	// Tables to import from each database
	ProductsTables  []string
	OrdersTables    []string
	CustomersTables []string

	// Whether FDW is enabled
	Enabled bool

	// Retry configuration
	MaxRetries    int
	RetryInterval time.Duration
}

// FDWManager manages Foreign Data Wrapper setup and health
type FDWManager struct {
	db     *gorm.DB
	config *FDWConfig
	logger *logrus.Logger

	// Status tracking
	mu            sync.RWMutex
	initialized   bool
	lastError     error
	lastCheckTime time.Time
}

// DefaultConfig returns a default FDW configuration from environment variables
func DefaultConfig() *FDWConfig {
	return &FDWConfig{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),

		// FDW Server Host defaults to "localhost" because all databases are typically
		// on the same PostgreSQL instance. FDW connections happen FROM PostgreSQL,
		// not from the application, so localhost is correct when DBs share an instance.
		FDWServerHost: getEnv("FDW_SERVER_HOST", "localhost"),

		ProductsDB:  getEnv("FDW_PRODUCTS_DB", "products_db"),
		OrdersDB:    getEnv("FDW_ORDERS_DB", "orders_db"),
		CustomersDB: getEnv("FDW_CUSTOMERS_DB", "customers_db"),

		ProductsTables:  []string{"products"},
		OrdersTables:    []string{"orders", "order_items"},
		CustomersTables: []string{"customers"},

		Enabled:       getEnv("FDW_ENABLED", "true") == "true",
		MaxRetries:    3,
		RetryInterval: 5 * time.Second,
	}
}

// NewFDWManager creates a new FDW manager
func NewFDWManager(db *gorm.DB, config *FDWConfig, logger *logrus.Logger) *FDWManager {
	if config == nil {
		config = DefaultConfig()
	}
	return &FDWManager{
		db:     db,
		config: config,
		logger: logger,
	}
}

// Initialize sets up the FDW with retry logic
// This is safe to call on every service startup - it's idempotent
func (m *FDWManager) Initialize(ctx context.Context) error {
	if !m.config.Enabled {
		m.logger.Info("FDW is disabled, skipping initialization")
		return nil
	}

	m.logger.Info("Starting FDW initialization...")

	var lastErr error
	for attempt := 1; attempt <= m.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := m.initializeFDW(ctx); err != nil {
			lastErr = err
			m.logger.WithError(err).WithField("attempt", attempt).Warn("FDW initialization attempt failed")

			if attempt < m.config.MaxRetries {
				m.logger.WithField("retry_in", m.config.RetryInterval).Info("Retrying FDW initialization...")
				time.Sleep(m.config.RetryInterval)
				continue
			}
		} else {
			m.mu.Lock()
			m.initialized = true
			m.lastError = nil
			m.lastCheckTime = time.Now()
			m.mu.Unlock()

			m.logger.Info("FDW initialization completed successfully")
			return nil
		}
	}

	m.mu.Lock()
	m.initialized = false
	m.lastError = lastErr
	m.lastCheckTime = time.Now()
	m.mu.Unlock()

	return fmt.Errorf("FDW initialization failed after %d attempts: %w", m.config.MaxRetries, lastErr)
}

// initializeFDW performs the actual FDW setup
func (m *FDWManager) initializeFDW(ctx context.Context) error {
	// Step 1: Create postgres_fdw extension
	if err := m.createExtension(ctx); err != nil {
		return fmt.Errorf("failed to create FDW extension: %w", err)
	}

	// Step 2: Create foreign servers
	servers := []struct {
		name   string
		dbname string
	}{
		{"products_server", m.config.ProductsDB},
		{"orders_server", m.config.OrdersDB},
		{"customers_server", m.config.CustomersDB},
	}

	for _, server := range servers {
		if err := m.createForeignServer(ctx, server.name, server.dbname); err != nil {
			return fmt.Errorf("failed to create foreign server %s: %w", server.name, err)
		}
	}

	// Step 3: Create/update user mappings
	for _, server := range servers {
		if err := m.createUserMapping(ctx, server.name); err != nil {
			return fmt.Errorf("failed to create user mapping for %s: %w", server.name, err)
		}
	}

	// Step 4: Import foreign tables
	tableImports := []struct {
		serverName string
		tables     []string
	}{
		{"products_server", m.config.ProductsTables},
		{"orders_server", m.config.OrdersTables},
		{"customers_server", m.config.CustomersTables},
	}

	for _, ti := range tableImports {
		if err := m.importForeignTables(ctx, ti.serverName, ti.tables); err != nil {
			return fmt.Errorf("failed to import foreign tables from %s: %w", ti.serverName, err)
		}
	}

	// Step 5: Verify FDW is working
	if err := m.verifyFDW(ctx); err != nil {
		return fmt.Errorf("FDW verification failed: %w", err)
	}

	return nil
}

// createExtension creates the postgres_fdw extension if it doesn't exist
func (m *FDWManager) createExtension(ctx context.Context) error {
	m.logger.Debug("Creating postgres_fdw extension...")

	sql := `CREATE EXTENSION IF NOT EXISTS postgres_fdw`
	if err := m.db.WithContext(ctx).Exec(sql).Error; err != nil {
		return err
	}

	m.logger.Debug("postgres_fdw extension ready")
	return nil
}

// createForeignServer creates a foreign server if it doesn't exist
func (m *FDWManager) createForeignServer(ctx context.Context, serverName, dbname string) error {
	m.logger.WithFields(logrus.Fields{
		"server": serverName,
		"dbname": dbname,
	}).Debug("Checking foreign server...")

	// Check if server exists
	var count int64
	checkSQL := `SELECT COUNT(*) FROM pg_foreign_server WHERE srvname = ?`
	if err := m.db.WithContext(ctx).Raw(checkSQL, serverName).Scan(&count).Error; err != nil {
		return err
	}

	// Determine the host to use for FDW connections
	// FDWServerHost is used because FDW connections happen FROM PostgreSQL itself
	// When all databases are on the same instance, this should be "localhost"
	fdwHost := m.config.FDWServerHost
	if fdwHost == "" {
		fdwHost = "localhost" // Default to localhost for same-instance databases
	}

	m.logger.WithFields(logrus.Fields{
		"server":   serverName,
		"fdw_host": fdwHost,
		"port":     m.config.DBPort,
		"dbname":   dbname,
	}).Debug("Configuring foreign server...")

	if count > 0 {
		// Server exists, update options in case they changed
		m.logger.WithField("server", serverName).Debug("Foreign server exists, updating options...")

		// Update server options to ensure they're correct
		updateSQL := fmt.Sprintf(`
			ALTER SERVER %s OPTIONS (
				SET host '%s',
				SET port '%s',
				SET dbname '%s'
			)
		`, serverName, fdwHost, m.config.DBPort, dbname)

		if err := m.db.WithContext(ctx).Exec(updateSQL).Error; err != nil {
			// If ALTER fails, the server might have been created with different initial options
			// This is OK - the server exists and we can use it
			m.logger.WithError(err).WithField("server", serverName).Debug("Could not update server options (non-fatal)")
		}
	} else {
		// Create new server with FDWServerHost (typically "localhost" for same-instance DBs)
		createSQL := fmt.Sprintf(`
			CREATE SERVER %s FOREIGN DATA WRAPPER postgres_fdw
			OPTIONS (host '%s', port '%s', dbname '%s')
		`, serverName, fdwHost, m.config.DBPort, dbname)

		if err := m.db.WithContext(ctx).Exec(createSQL).Error; err != nil {
			return err
		}
		m.logger.WithFields(logrus.Fields{
			"server":   serverName,
			"fdw_host": fdwHost,
		}).Info("Created foreign server")
	}

	return nil
}

// createUserMapping creates or updates a user mapping for a foreign server
func (m *FDWManager) createUserMapping(ctx context.Context, serverName string) error {
	m.logger.WithField("server", serverName).Debug("Checking user mapping...")

	// Check if user mapping exists
	var count int64
	checkSQL := `SELECT COUNT(*) FROM pg_user_mappings WHERE srvname = ? AND usename = ?`
	if err := m.db.WithContext(ctx).Raw(checkSQL, serverName, m.config.DBUser).Scan(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		// Update existing mapping (password might have changed)
		updateSQL := fmt.Sprintf(`
			ALTER USER MAPPING FOR %s SERVER %s
			OPTIONS (SET user '%s', SET password '%s')
		`, m.config.DBUser, serverName, m.config.DBUser, m.config.DBPassword)

		if err := m.db.WithContext(ctx).Exec(updateSQL).Error; err != nil {
			// Try without SET (for fresh mappings)
			updateSQL2 := fmt.Sprintf(`
				ALTER USER MAPPING FOR %s SERVER %s
				OPTIONS (ADD user '%s', ADD password '%s')
			`, m.config.DBUser, serverName, m.config.DBUser, m.config.DBPassword)

			if err2 := m.db.WithContext(ctx).Exec(updateSQL2).Error; err2 != nil {
				m.logger.WithError(err).WithField("server", serverName).Debug("Could not update user mapping (non-fatal)")
			}
		}
		m.logger.WithField("server", serverName).Debug("User mapping updated")
	} else {
		// Create new mapping
		createSQL := fmt.Sprintf(`
			CREATE USER MAPPING FOR %s SERVER %s
			OPTIONS (user '%s', password '%s')
		`, m.config.DBUser, serverName, m.config.DBUser, m.config.DBPassword)

		if err := m.db.WithContext(ctx).Exec(createSQL).Error; err != nil {
			return err
		}
		m.logger.WithField("server", serverName).Info("Created user mapping")
	}

	return nil
}

// importForeignTables imports specified tables from a foreign server
func (m *FDWManager) importForeignTables(ctx context.Context, serverName string, tables []string) error {
	for _, table := range tables {
		m.logger.WithFields(logrus.Fields{
			"server": serverName,
			"table":  table,
		}).Debug("Checking foreign table...")

		// Check if foreign table exists
		var count int64
		checkSQL := `SELECT COUNT(*) FROM information_schema.foreign_tables WHERE foreign_table_name = ?`
		if err := m.db.WithContext(ctx).Raw(checkSQL, table).Scan(&count).Error; err != nil {
			return err
		}

		if count > 0 {
			// Table exists - verify it's accessible
			m.logger.WithField("table", table).Debug("Foreign table exists, verifying accessibility...")

			// Try a simple query to verify the table works
			verifySQL := fmt.Sprintf(`SELECT 1 FROM %s LIMIT 0`, table)
			if err := m.db.WithContext(ctx).Exec(verifySQL).Error; err != nil {
				// Table exists but is broken - drop and reimport
				m.logger.WithError(err).WithField("table", table).Warn("Foreign table exists but is inaccessible, recreating...")

				dropSQL := fmt.Sprintf(`DROP FOREIGN TABLE IF EXISTS %s CASCADE`, table)
				if err := m.db.WithContext(ctx).Exec(dropSQL).Error; err != nil {
					return fmt.Errorf("failed to drop broken foreign table %s: %w", table, err)
				}
			} else {
				// Table works fine
				m.logger.WithField("table", table).Debug("Foreign table is accessible")
				continue
			}
		}

		// Import the table
		importSQL := fmt.Sprintf(`
			IMPORT FOREIGN SCHEMA public
			LIMIT TO (%s)
			FROM SERVER %s INTO public
		`, table, serverName)

		if err := m.db.WithContext(ctx).Exec(importSQL).Error; err != nil {
			return fmt.Errorf("failed to import table %s: %w", table, err)
		}
		m.logger.WithFields(logrus.Fields{
			"server": serverName,
			"table":  table,
		}).Info("Imported foreign table")
	}

	return nil
}

// verifyFDW verifies that all required foreign tables are accessible
func (m *FDWManager) verifyFDW(ctx context.Context) error {
	m.logger.Debug("Verifying FDW setup...")

	// List of tables we need to verify
	requiredTables := append(append(
		m.config.ProductsTables,
		m.config.OrdersTables...),
		m.config.CustomersTables...,
	)

	for _, table := range requiredTables {
		// Try to access each table
		verifySQL := fmt.Sprintf(`SELECT 1 FROM %s LIMIT 0`, table)
		if err := m.db.WithContext(ctx).Exec(verifySQL).Error; err != nil {
			return fmt.Errorf("foreign table %s is not accessible: %w", table, err)
		}
		m.logger.WithField("table", table).Debug("Foreign table verified")
	}

	// Count foreign tables
	var count int64
	countSQL := `SELECT COUNT(*) FROM information_schema.foreign_tables`
	if err := m.db.WithContext(ctx).Raw(countSQL).Scan(&count).Error; err != nil {
		return err
	}

	m.logger.WithField("table_count", count).Info("FDW verification completed")
	return nil
}

// IsHealthy checks if FDW is properly configured and working
func (m *FDWManager) IsHealthy(ctx context.Context) (bool, error) {
	if !m.config.Enabled {
		return true, nil // FDW disabled is considered healthy
	}

	m.mu.RLock()
	initialized := m.initialized
	m.mu.RUnlock()

	if !initialized {
		return false, fmt.Errorf("FDW not initialized")
	}

	// Quick verification - just check one table
	verifySQL := `SELECT 1 FROM orders LIMIT 0`
	if err := m.db.WithContext(ctx).Exec(verifySQL).Error; err != nil {
		return false, fmt.Errorf("FDW health check failed: %w", err)
	}

	return true, nil
}

// Status returns the current FDW status
func (m *FDWManager) Status() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]interface{}{
		"enabled":     m.config.Enabled,
		"initialized": m.initialized,
		"last_check":  m.lastCheckTime.Format(time.RFC3339),
	}

	if m.lastError != nil {
		status["last_error"] = m.lastError.Error()
	}

	return status
}

// Reinitialize forces a re-initialization of FDW (useful after schema changes)
func (m *FDWManager) Reinitialize(ctx context.Context) error {
	m.logger.Info("Forcing FDW re-initialization...")

	m.mu.Lock()
	m.initialized = false
	m.mu.Unlock()

	// Drop all foreign tables first
	tables := append(append(
		m.config.ProductsTables,
		m.config.OrdersTables...),
		m.config.CustomersTables...,
	)

	for _, table := range tables {
		dropSQL := fmt.Sprintf(`DROP FOREIGN TABLE IF EXISTS %s CASCADE`, table)
		if err := m.db.WithContext(ctx).Exec(dropSQL).Error; err != nil {
			m.logger.WithError(err).WithField("table", table).Warn("Failed to drop foreign table")
		}
	}

	return m.Initialize(ctx)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
