package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sony/gobreaker"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/tesseract-hub/audit-service/internal/models"
	"github.com/tesseract-hub/audit-service/internal/tenant"
)

var (
	ErrConnectionFailed    = errors.New("failed to establish database connection")
	ErrCircuitOpen         = errors.New("circuit breaker is open, database unavailable")
	ErrTenantNotConfigured = errors.New("tenant database not configured")
	ErrPoolExhausted       = errors.New("connection pool exhausted")
)

// ConnectionPool represents a connection pool for a single tenant's database
type ConnectionPool struct {
	DB            *gorm.DB
	TenantID      string
	ProductID     string
	CreatedAt     time.Time
	LastUsed      time.Time
	IsHealthy     bool
	HealthCheck   time.Time
	ConnectionDSN string // For logging (password masked)
}

// FallbackDBConfig holds configuration for the fallback database
type FallbackDBConfig struct {
	Enabled      bool
	Host         string
	Port         int
	Database     string
	User         string
	Password     string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

// Manager handles dynamic multi-database connections with pooling and circuit breakers
type Manager struct {
	mu              sync.RWMutex
	pools           map[string]*ConnectionPool // tenantID -> connection pool
	circuitBreakers map[string]*gobreaker.CircuitBreaker

	registry        *tenant.Registry
	logger          *logrus.Logger

	// Fallback database (used when tenant-specific config not available)
	fallbackDB      *gorm.DB
	fallbackConfig  *FallbackDBConfig

	// Configuration
	maxPoolsPerService int           // Max number of tenant connections to maintain
	poolCleanupInterval time.Duration
	healthCheckInterval time.Duration
	connectionTimeout   time.Duration

	// Metrics
	activeConnections   int
	totalConnections    int64
	failedConnections   int64
	circuitBreakerTrips int64

	// Context for background tasks
	ctx    context.Context
	cancel context.CancelFunc
}

// ManagerConfig holds configuration for the database manager
type ManagerConfig struct {
	Registry            *tenant.Registry
	Logger              *logrus.Logger
	MaxPoolsPerService  int           // Default: 100
	PoolCleanupInterval time.Duration // Default: 5 minutes
	HealthCheckInterval time.Duration // Default: 30 seconds
	ConnectionTimeout   time.Duration // Default: 10 seconds
	FallbackDB          *FallbackDBConfig // Fallback database for when tenant config isn't available
}

// NewManager creates a new multi-database connection manager
func NewManager(config ManagerConfig) *Manager {
	if config.MaxPoolsPerService == 0 {
		config.MaxPoolsPerService = 100
	}
	if config.PoolCleanupInterval == 0 {
		config.PoolCleanupInterval = 5 * time.Minute
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		pools:               make(map[string]*ConnectionPool),
		circuitBreakers:     make(map[string]*gobreaker.CircuitBreaker),
		registry:            config.Registry,
		logger:              config.Logger,
		fallbackConfig:      config.FallbackDB,
		maxPoolsPerService:  config.MaxPoolsPerService,
		poolCleanupInterval: config.PoolCleanupInterval,
		healthCheckInterval: config.HealthCheckInterval,
		connectionTimeout:   config.ConnectionTimeout,
		ctx:                 ctx,
		cancel:              cancel,
	}

	// Initialize fallback database if configured
	if config.FallbackDB != nil && config.FallbackDB.Enabled {
		if err := m.initFallbackDB(); err != nil {
			config.Logger.WithError(err).Warn("Failed to initialize fallback database, continuing without it")
		} else {
			config.Logger.Info("Fallback database initialized successfully")
		}
	}

	// Start background tasks
	go m.startPoolCleanup()
	go m.startHealthChecks()

	return m
}

// initFallbackDB initializes the fallback database connection
func (m *Manager) initFallbackDB() error {
	if m.fallbackConfig == nil || !m.fallbackConfig.Enabled {
		return nil
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		m.fallbackConfig.Host,
		m.fallbackConfig.Port,
		m.fallbackConfig.User,
		m.fallbackConfig.Password,
		m.fallbackConfig.Database,
		m.fallbackConfig.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to fallback database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying DB: %w", err)
	}

	// Configure connection pool
	if m.fallbackConfig.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(m.fallbackConfig.MaxOpenConns)
	}
	if m.fallbackConfig.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(m.fallbackConfig.MaxIdleConns)
	}
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// Run migrations on fallback database
	if err := m.runMigrations(db); err != nil {
		m.logger.WithError(err).Warn("Failed to run migrations on fallback database")
	}

	m.fallbackDB = db
	return nil
}

// GetFallbackDB returns the fallback database connection
func (m *Manager) GetFallbackDB() *gorm.DB {
	return m.fallbackDB
}

// HasFallbackDB returns true if a fallback database is configured and connected
func (m *Manager) HasFallbackDB() bool {
	return m.fallbackDB != nil
}

// GetDB retrieves or creates a database connection for the tenant
func (m *Manager) GetDB(ctx context.Context, tenantID string) (*gorm.DB, error) {
	// Check circuit breaker first
	cb := m.getCircuitBreaker(tenantID)
	_, err := cb.Execute(func() (interface{}, error) {
		return nil, nil // Just checking if circuit is open
	})
	if err != nil {
		return nil, ErrCircuitOpen
	}

	// Try to get existing pool
	if pool := m.getPool(tenantID); pool != nil {
		if pool.IsHealthy {
			m.updateLastUsed(tenantID)
			return pool.DB, nil
		}
		// Pool exists but unhealthy, try to reconnect
		m.removePool(tenantID)
	}

	// Create new connection
	return m.createConnection(ctx, tenantID, cb)
}

// getPool retrieves an existing connection pool
func (m *Manager) getPool(tenantID string) *ConnectionPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pools[tenantID]
}

// createConnection establishes a new database connection for the tenant
func (m *Manager) createConnection(ctx context.Context, tenantID string, cb *gobreaker.CircuitBreaker) (*gorm.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check if another goroutine created the pool
	if pool, exists := m.pools[tenantID]; exists && pool.IsHealthy {
		return pool.DB, nil
	}

	// Check pool limit
	if len(m.pools) >= m.maxPoolsPerService {
		m.evictLRUPool()
	}

	// Get tenant configuration
	tenantInfo, err := m.registry.GetTenant(ctx, tenantID)
	if err != nil {
		// If tenant config not found but we have a fallback database, use it
		if m.fallbackDB != nil {
			m.logger.WithFields(logrus.Fields{
				"tenant_id": tenantID,
				"error":     err.Error(),
			}).Info("Using fallback database for tenant (config not found)")
			return m.fallbackDB, nil
		}
		m.failedConnections++
		return nil, fmt.Errorf("%w: %v", ErrTenantNotConfigured, err)
	}

	if !tenantInfo.IsActive {
		// Use fallback database for inactive tenants if available
		if m.fallbackDB != nil {
			m.logger.WithField("tenant_id", tenantID).Info("Using fallback database for inactive tenant")
			return m.fallbackDB, nil
		}
		return nil, fmt.Errorf("tenant %s is inactive", tenantID)
	}

	if !tenantInfo.Features.AuditLogsEnabled {
		// Use fallback database if audit logs disabled but fallback available
		if m.fallbackDB != nil {
			m.logger.WithField("tenant_id", tenantID).Info("Using fallback database (audit logs disabled for tenant)")
			return m.fallbackDB, nil
		}
		return nil, fmt.Errorf("audit logs not enabled for tenant %s", tenantID)
	}

	// Build DSN
	dsn := m.buildDSN(tenantInfo.DatabaseConfig)
	maskedDSN := m.maskDSN(dsn)

	// Attempt connection with circuit breaker
	result, err := cb.Execute(func() (interface{}, error) {
		return m.connectWithTimeout(ctx, dsn, tenantInfo.DatabaseConfig)
	})

	if err != nil {
		m.failedConnections++
		m.circuitBreakerTrips++
		m.logger.WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"dsn":       maskedDSN,
		}).WithError(err).Error("Failed to connect to tenant database")
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	db := result.(*gorm.DB)

	// Run migrations for this tenant
	if err := m.runMigrations(db); err != nil {
		m.logger.WithField("tenant_id", tenantID).WithError(err).Error("Failed to run migrations")
		// Don't fail the connection, migrations might already exist
	}

	// Create pool entry
	pool := &ConnectionPool{
		DB:            db,
		TenantID:      tenantID,
		ProductID:     tenantInfo.ProductID,
		CreatedAt:     time.Now(),
		LastUsed:      time.Now(),
		IsHealthy:     true,
		HealthCheck:   time.Now(),
		ConnectionDSN: maskedDSN,
	}

	m.pools[tenantID] = pool
	m.activeConnections = len(m.pools)
	m.totalConnections++

	m.logger.WithFields(logrus.Fields{
		"tenant_id":  tenantID,
		"product_id": tenantInfo.ProductID,
		"dsn":        maskedDSN,
	}).Info("Established new database connection for tenant")

	return db, nil
}

// buildDSN constructs the PostgreSQL connection string
func (m *Manager) buildDSN(config tenant.DatabaseConfig) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.DatabaseName,
		config.SSLMode,
	)
}

// maskDSN masks the password in DSN for logging
func (m *Manager) maskDSN(dsn string) string {
	// Simple masking - in production use regex
	return fmt.Sprintf("%s...password=***...%s", dsn[:20], dsn[len(dsn)-20:])
}

// connectWithTimeout establishes a database connection with timeout
func (m *Manager) connectWithTimeout(ctx context.Context, dsn string, config tenant.DatabaseConfig) (*gorm.DB, error) {
	ctx, cancel := context.WithTimeout(ctx, m.connectionTimeout)
	defer cancel()

	done := make(chan struct {
		db  *gorm.DB
		err error
	}, 1)

	go func() {
		gormConfig := &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
		}

		db, err := gorm.Open(postgres.Open(dsn), gormConfig)
		if err != nil {
			done <- struct {
				db  *gorm.DB
				err error
			}{nil, err}
			return
		}

		// Configure connection pool
		sqlDB, err := db.DB()
		if err != nil {
			done <- struct {
				db  *gorm.DB
				err error
			}{nil, err}
			return
		}

		maxOpen := config.MaxOpenConns
		if maxOpen == 0 {
			maxOpen = 25
		}
		maxIdle := config.MaxIdleConns
		if maxIdle == 0 {
			maxIdle = 10
		}
		maxLifetime := config.MaxLifetime
		if maxLifetime == 0 {
			maxLifetime = 3600 // 1 hour
		}

		sqlDB.SetMaxOpenConns(maxOpen)
		sqlDB.SetMaxIdleConns(maxIdle)
		sqlDB.SetConnMaxLifetime(time.Duration(maxLifetime) * time.Second)

		// Test connection
		if err := sqlDB.PingContext(ctx); err != nil {
			done <- struct {
				db  *gorm.DB
				err error
			}{nil, err}
			return
		}

		done <- struct {
			db  *gorm.DB
			err error
		}{db, nil}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-done:
		return result.db, result.err
	}
}

// runMigrations creates the audit_logs table if it doesn't exist
func (m *Manager) runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(&models.AuditLog{})
}

// getCircuitBreaker gets or creates a circuit breaker for a tenant
func (m *Manager) getCircuitBreaker(tenantID string) *gobreaker.CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cb, exists := m.circuitBreakers[tenantID]; exists {
		return cb
	}

	// Create new circuit breaker
	settings := gobreaker.Settings{
		Name:        fmt.Sprintf("db-%s", tenantID),
		MaxRequests: 3,                // Allow 3 requests in half-open state
		Interval:    30 * time.Second, // Clear counts after 30 seconds
		Timeout:     60 * time.Second, // Stay open for 60 seconds before half-open
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip if 5 consecutive failures or 50% failure rate with at least 10 requests
			return counts.ConsecutiveFailures >= 5 ||
				(counts.Requests >= 10 && float64(counts.TotalFailures)/float64(counts.Requests) >= 0.5)
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			m.logger.WithFields(logrus.Fields{
				"circuit_breaker": name,
				"from":            from.String(),
				"to":              to.String(),
			}).Info("Circuit breaker state changed")
		},
	}

	cb := gobreaker.NewCircuitBreaker(settings)
	m.circuitBreakers[tenantID] = cb
	return cb
}

// updateLastUsed updates the last used timestamp for a pool
func (m *Manager) updateLastUsed(tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pool, exists := m.pools[tenantID]; exists {
		pool.LastUsed = time.Now()
	}
}

// removePool removes a connection pool
func (m *Manager) removePool(tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pool, exists := m.pools[tenantID]; exists {
		if sqlDB, err := pool.DB.DB(); err == nil {
			sqlDB.Close()
		}
		delete(m.pools, tenantID)
		m.activeConnections = len(m.pools)
	}
}

// evictLRUPool removes the least recently used pool
func (m *Manager) evictLRUPool() {
	var oldestTenant string
	var oldestTime time.Time

	for tenantID, pool := range m.pools {
		if oldestTenant == "" || pool.LastUsed.Before(oldestTime) {
			oldestTenant = tenantID
			oldestTime = pool.LastUsed
		}
	}

	if oldestTenant != "" {
		if pool := m.pools[oldestTenant]; pool != nil {
			if sqlDB, err := pool.DB.DB(); err == nil {
				sqlDB.Close()
			}
		}
		delete(m.pools, oldestTenant)
		m.logger.WithField("tenant_id", oldestTenant).Debug("Evicted LRU connection pool")
	}
}

// startPoolCleanup runs periodic cleanup of idle pools
func (m *Manager) startPoolCleanup() {
	ticker := time.NewTicker(m.poolCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupIdlePools()
		}
	}
}

func (m *Manager) cleanupIdlePools() {
	m.mu.Lock()
	defer m.mu.Unlock()

	idleThreshold := 10 * time.Minute
	now := time.Now()

	for tenantID, pool := range m.pools {
		if now.Sub(pool.LastUsed) > idleThreshold {
			if sqlDB, err := pool.DB.DB(); err == nil {
				sqlDB.Close()
			}
			delete(m.pools, tenantID)
			m.logger.WithField("tenant_id", tenantID).Debug("Closed idle connection pool")
		}
	}

	m.activeConnections = len(m.pools)
}

// startHealthChecks runs periodic health checks on all pools
func (m *Manager) startHealthChecks() {
	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.runHealthChecks()
		}
	}
}

func (m *Manager) runHealthChecks() {
	m.mu.RLock()
	tenantIDs := make([]string, 0, len(m.pools))
	for tenantID := range m.pools {
		tenantIDs = append(tenantIDs, tenantID)
	}
	m.mu.RUnlock()

	for _, tenantID := range tenantIDs {
		m.checkPoolHealth(tenantID)
	}
}

func (m *Manager) checkPoolHealth(tenantID string) {
	m.mu.Lock()
	pool, exists := m.pools[tenantID]
	if !exists {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sqlDB, err := pool.DB.DB()
	if err != nil {
		m.markPoolUnhealthy(tenantID)
		return
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		m.markPoolUnhealthy(tenantID)
		m.logger.WithField("tenant_id", tenantID).WithError(err).Warn("Database health check failed")
		return
	}

	m.mu.Lock()
	if pool, exists := m.pools[tenantID]; exists {
		pool.IsHealthy = true
		pool.HealthCheck = time.Now()
	}
	m.mu.Unlock()
}

func (m *Manager) markPoolUnhealthy(tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pool, exists := m.pools[tenantID]; exists {
		pool.IsHealthy = false
	}
}

// GetStats returns connection manager statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	poolDetails := make([]map[string]interface{}, 0, len(m.pools))
	for tenantID, pool := range m.pools {
		poolDetails = append(poolDetails, map[string]interface{}{
			"tenant_id":    tenantID,
			"product_id":   pool.ProductID,
			"is_healthy":   pool.IsHealthy,
			"created_at":   pool.CreatedAt,
			"last_used":    pool.LastUsed,
			"health_check": pool.HealthCheck,
		})
	}

	return map[string]interface{}{
		"active_connections":     m.activeConnections,
		"total_connections":      m.totalConnections,
		"failed_connections":     m.failedConnections,
		"circuit_breaker_trips":  m.circuitBreakerTrips,
		"max_pools":              m.maxPoolsPerService,
		"pools":                  poolDetails,
	}
}

// Close gracefully closes all connections
func (m *Manager) Close() error {
	m.cancel() // Stop background tasks

	m.mu.Lock()
	defer m.mu.Unlock()

	for tenantID, pool := range m.pools {
		if sqlDB, err := pool.DB.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				m.logger.WithField("tenant_id", tenantID).WithError(err).Warn("Error closing pool")
			}
		}
	}

	m.pools = make(map[string]*ConnectionPool)
	m.activeConnections = 0
	return nil
}
