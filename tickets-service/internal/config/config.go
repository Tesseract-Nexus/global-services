package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"tickets-service/internal/models"

	"github.com/Tesseract-Nexus/go-shared/secrets"
)

type Config struct {
	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Server
	Port        string
	Environment string

	// JWT
	JWTSecret string

	// Services
	DocumentServiceURL     string
	NotificationServiceURL string
	EscalationServiceURL   string
	ProductID              string // Product identifier for document-service

	// Pagination
	DefaultPageSize int
	MaxPageSize     int

	// Ticket specific settings
	MaxTicketLength         int
	MaxAttachmentsPerTicket int
	DefaultSLAHours         int
	AutoEscalationEnabled   bool
}

func Load() *Config {
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	defaultPageSize, _ := strconv.Atoi(getEnv("DEFAULT_PAGE_SIZE", "20"))
	maxPageSize, _ := strconv.Atoi(getEnv("MAX_PAGE_SIZE", "100"))
	maxTicketLength, _ := strconv.Atoi(getEnv("MAX_TICKET_LENGTH", "10000"))
	maxAttachments, _ := strconv.Atoi(getEnv("MAX_ATTACHMENTS_PER_TICKET", "20"))
	defaultSLA, _ := strconv.Atoi(getEnv("DEFAULT_SLA_HOURS", "24"))
	autoEscalation, _ := strconv.ParseBool(getEnv("AUTO_ESCALATION_ENABLED", "true"))

	return &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: secrets.GetDBPassword(),
		DBName:     getEnv("DB_NAME", "tickets_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// Server
		Port:        getEnv("PORT", "8085"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// JWT
		JWTSecret: secrets.GetJWTSecret(),

		// Services
		DocumentServiceURL:     getEnv("DOCUMENT_SERVICE_URL", "http://localhost:8082"),
		NotificationServiceURL: getEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8090"),
		EscalationServiceURL:   getEnv("ESCALATION_SERVICE_URL", "http://localhost:8093"),
		ProductID:              getEnv("PRODUCT_ID", "marketplace"),

		// Pagination
		DefaultPageSize: defaultPageSize,
		MaxPageSize:     maxPageSize,

		// Ticket specific settings
		MaxTicketLength:         maxTicketLength,
		MaxAttachmentsPerTicket: maxAttachments,
		DefaultSLAHours:         defaultSLA,
		AutoEscalationEnabled:   autoEscalation,
	}
}

func InitDB(cfg *Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)

	var logLevel logger.LogLevel
	if cfg.Environment == "production" {
		logLevel = logger.Error
	} else {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migration for ticket_number column
	if err := migrateTicketNumber(db); err != nil {
		log.Printf("Warning: ticket_number migration failed: %v", err)
	}

	// Auto-migrate models to keep schema in sync
	if err := db.AutoMigrate(&models.Ticket{}); err != nil {
		log.Printf("Warning: AutoMigrate failed: %v", err)
		// Don't return error - table may already exist with correct schema
	} else {
		log.Println("Database schema synchronized")
	}

	return db, nil
}

// migrateTicketNumber handles migration for the ticket_number column
// It adds the column as nullable, populates existing rows, then adds NOT NULL constraint
func migrateTicketNumber(db *gorm.DB) error {
	// Check if tickets table exists
	if !db.Migrator().HasTable("tickets") {
		log.Println("Tickets table does not exist yet, skipping ticket_number migration")
		return nil
	}

	// Check if ticket_number column exists
	if db.Migrator().HasColumn(&models.Ticket{}, "ticket_number") {
		log.Println("ticket_number column already exists")
		return nil
	}

	log.Println("Adding ticket_number column to tickets table...")

	// Step 1: Add column as nullable first
	if err := db.Exec("ALTER TABLE tickets ADD COLUMN IF NOT EXISTS ticket_number VARCHAR(20)").Error; err != nil {
		return fmt.Errorf("failed to add ticket_number column: %w", err)
	}

	// Step 2: Populate existing rows with sequential ticket numbers per tenant
	// Uses a window function to generate sequential numbers per tenant
	updateSQL := `
		WITH numbered AS (
			SELECT id, tenant_id,
				   'TKT-' || LPAD(ROW_NUMBER() OVER (PARTITION BY tenant_id ORDER BY created_at)::TEXT, 8, '0') as new_number
			FROM tickets
			WHERE ticket_number IS NULL
		)
		UPDATE tickets t
		SET ticket_number = n.new_number
		FROM numbered n
		WHERE t.id = n.id
	`
	result := db.Exec(updateSQL)
	if result.Error != nil {
		return fmt.Errorf("failed to populate ticket_number: %w", result.Error)
	}
	log.Printf("Updated %d existing tickets with sequential numbers", result.RowsAffected)

	// Step 3: Add NOT NULL constraint
	if err := db.Exec("ALTER TABLE tickets ALTER COLUMN ticket_number SET NOT NULL").Error; err != nil {
		return fmt.Errorf("failed to set ticket_number NOT NULL: %w", err)
	}

	// Step 4: Add unique index (tenant_id + ticket_number)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_ticket_number_tenant ON tickets(tenant_id, ticket_number)").Error; err != nil {
		log.Printf("Warning: failed to create unique index: %v", err)
	}

	log.Println("ticket_number migration completed successfully")
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
