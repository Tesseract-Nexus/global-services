package repository

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"tickets-service/internal/models"
)

type TicketsRepository struct {
	db *gorm.DB
}

func NewTicketsRepository(db *gorm.DB) *TicketsRepository {
	return &TicketsRepository{db: db}
}

// GetNextTicketNumber generates the next sequential ticket number for a tenant
// Format: TKT-00000001, TKT-00000002, etc.
func (r *TicketsRepository) GetNextTicketNumber(tenantID string) (string, error) {
	var maxNumber int64

	// Get the max ticket number for this tenant
	// Extract the numeric part from ticket_number like 'TKT-00000001'
	err := r.db.Model(&models.Ticket{}).
		Where("tenant_id = ?", tenantID).
		Select("COALESCE(MAX(CAST(SUBSTRING(ticket_number FROM 5) AS BIGINT)), 0)").
		Scan(&maxNumber).Error

	if err != nil {
		return "", err
	}

	// Generate next number with 8-digit padding
	nextNumber := maxNumber + 1
	return "TKT-" + fmt.Sprintf("%08d", nextNumber), nil
}

// CreateTicket creates a new ticket
func (r *TicketsRepository) CreateTicket(tenantID string, ticket *models.Ticket) error {
	ticket.TenantID = tenantID
	ticket.CreatedAt = time.Now()
	ticket.UpdatedAt = time.Now()

	// Generate sequential ticket number if not already set
	if ticket.TicketNumber == "" {
		ticketNumber, err := r.GetNextTicketNumber(tenantID)
		if err != nil {
			return fmt.Errorf("failed to generate ticket number: %w", err)
		}
		ticket.TicketNumber = ticketNumber
	}

	return r.db.Create(ticket).Error
}

// GetTicketByID retrieves a ticket by ID
func (r *TicketsRepository) GetTicketByID(tenantID string, ticketID uuid.UUID) (*models.Ticket, error) {
	var ticket models.Ticket
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, ticketID).First(&ticket).Error
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

// TicketFilters contains optional filters for ticket queries
type TicketFilters struct {
	CreatedBy string
	Status    string
	Priority  string
	Type      string
}

// GetTickets retrieves tickets with pagination and optional filters
func (r *TicketsRepository) GetTickets(tenantID string, page, limit int, filters *TicketFilters) ([]models.Ticket, int64, error) {
	var tickets []models.Ticket
	var total int64

	query := r.db.Model(&models.Ticket{}).Where("tenant_id = ?", tenantID)

	// Apply filters if provided
	if filters != nil {
		if filters.CreatedBy != "" {
			query = query.Where("created_by = ?", filters.CreatedBy)
		}
		if filters.Status != "" {
			query = query.Where("status = ?", filters.Status)
		}
		if filters.Priority != "" {
			query = query.Where("priority = ?", filters.Priority)
		}
		if filters.Type != "" {
			query = query.Where("type = ?", filters.Type)
		}
	}

	// Count total results
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&tickets).Error; err != nil {
		return nil, 0, err
	}

	return tickets, total, nil
}

// UpdateTicket updates a ticket
func (r *TicketsRepository) UpdateTicket(tenantID string, ticketID uuid.UUID, updates *models.Ticket) error {
	updates.UpdatedAt = time.Now()
	return r.db.Model(&models.Ticket{}).
		Where("tenant_id = ? AND id = ?", tenantID, ticketID).
		Updates(updates).Error
}

// DeleteTicket soft deletes a ticket
func (r *TicketsRepository) DeleteTicket(tenantID string, ticketID uuid.UUID) error {
	return r.db.Where("tenant_id = ? AND id = ?", tenantID, ticketID).
		Delete(&models.Ticket{}).Error
}

// UpdateTicketStatus updates only the status of a ticket
func (r *TicketsRepository) UpdateTicketStatus(tenantID string, ticketID uuid.UUID, status models.TicketStatus, updatedBy string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
		"updated_by": updatedBy,
	}

	// If status is RESOLVED, set resolved_at timestamp
	if status == models.TicketStatusResolved {
		updates["resolved_at"] = time.Now()
	}

	return r.db.Model(&models.Ticket{}).
		Where("tenant_id = ? AND id = ?", tenantID, ticketID).
		Updates(updates).Error
}

// AddComment adds a comment to a ticket's inline comments JSONB array
func (r *TicketsRepository) AddComment(tenantID string, ticketID uuid.UUID, comment map[string]interface{}) (*models.Ticket, error) {
	// First get the current ticket
	ticket, err := r.GetTicketByID(tenantID, ticketID)
	if err != nil {
		return nil, err
	}

	// Initialize comments if nil
	if ticket.Comments == nil {
		emptyComments := make(models.JSON)
		ticket.Comments = &emptyComments
	}

	// Add the new comment with an index key
	comments := *ticket.Comments
	newIndex := len(comments)
	comments[strconv.Itoa(newIndex)] = comment

	// Update the ticket
	ticket.Comments = &comments
	ticket.UpdatedAt = time.Now()

	err = r.db.Model(&models.Ticket{}).
		Where("tenant_id = ? AND id = ?", tenantID, ticketID).
		Updates(map[string]interface{}{
			"comments":   ticket.Comments,
			"updated_at": ticket.UpdatedAt,
		}).Error

	if err != nil {
		return nil, err
	}

	return ticket, nil
}
