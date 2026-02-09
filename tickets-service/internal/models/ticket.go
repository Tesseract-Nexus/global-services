package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TicketStatus represents the status of a ticket
type TicketStatus string

const (
	TicketStatusOpen            TicketStatus = "OPEN"
	TicketStatusInProgress      TicketStatus = "IN_PROGRESS"
	TicketStatusOnHold          TicketStatus = "ON_HOLD"
	TicketStatusResolved        TicketStatus = "RESOLVED"
	TicketStatusClosed          TicketStatus = "CLOSED"
	TicketStatusReopened        TicketStatus = "REOPENED"
	TicketStatusCancelled       TicketStatus = "CANCELLED"
	TicketStatusPendingApproval TicketStatus = "PENDING_APPROVAL"
	TicketStatusEscalated       TicketStatus = "ESCALATED"
)

// TicketPriority represents the priority of a ticket
type TicketPriority string

const (
	TicketPriorityLow      TicketPriority = "LOW"
	TicketPriorityMedium   TicketPriority = "MEDIUM"
	TicketPriorityHigh     TicketPriority = "HIGH"
	TicketPriorityCritical TicketPriority = "CRITICAL"
	TicketPriorityUrgent   TicketPriority = "URGENT"
)

// TicketType represents the type of ticket
type TicketType string

const (
	TicketTypeBug           TicketType = "BUG"
	TicketTypeFeature       TicketType = "FEATURE"
	TicketTypeImprovement   TicketType = "IMPROVEMENT"
	TicketTypeSupport       TicketType = "SUPPORT"
	TicketTypeIncident      TicketType = "INCIDENT"
	TicketTypeChangeRequest TicketType = "CHANGE_REQUEST"
	TicketTypeMaintenance   TicketType = "MAINTENANCE"
	TicketTypeConsultation  TicketType = "CONSULTATION"
	TicketTypeComplaint     TicketType = "COMPLAINT"
	TicketTypeQuestion      TicketType = "QUESTION"
	TicketTypeTask          TicketType = "TASK"
	TicketTypeReturnRequest TicketType = "RETURN_REQUEST"
)

// JSON type for PostgreSQL JSONB
type JSON map[string]interface{}

func (j JSON) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Ticket represents a support ticket
type Ticket struct {
	ID             uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TicketNumber   string          `json:"ticketNumber" gorm:"column:ticket_number;uniqueIndex:idx_ticket_number_tenant;not null"`
	TenantID       string          `json:"tenantId" gorm:"not null;index;uniqueIndex:idx_ticket_number_tenant"`
	ApplicationID  string          `json:"applicationId" gorm:"not null"`
	Title          string          `json:"title" gorm:"not null"`
	Description    string          `json:"description" gorm:"not null"`
	Type           TicketType      `json:"type" gorm:"not null"`
	Status         TicketStatus    `json:"status" gorm:"not null;default:'OPEN'"`
	Priority       TicketPriority  `json:"priority" gorm:"not null;default:'MEDIUM'"`
	Tags           *JSON           `json:"tags,omitempty" gorm:"type:jsonb"`
	CreatedBy      string          `json:"createdBy" gorm:"not null;index"`
	CreatedByName  string          `json:"createdByName,omitempty" gorm:"column:created_by_name"`
	CreatedByEmail string          `json:"createdByEmail,omitempty" gorm:"column:created_by_email"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	DueDate        *time.Time      `json:"dueDate,omitempty"`
	EstimatedTime  *int            `json:"estimatedTime,omitempty"` // in minutes
	ActualTime     *int            `json:"actualTime,omitempty"`    // in minutes
	ParentTicketID *uuid.UUID      `json:"parentTicketId,omitempty"`
	Assignees      *JSON           `json:"assignees,omitempty" gorm:"type:jsonb"`
	Attachments    *JSON           `json:"attachments,omitempty" gorm:"type:jsonb"`
	Comments       *JSON           `json:"comments,omitempty" gorm:"type:jsonb"`
	SLA            *JSON           `json:"sla,omitempty" gorm:"type:jsonb"`
	History        *JSON           `json:"history,omitempty" gorm:"type:jsonb"`
	DeletedAt      *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	UpdatedBy      *string         `json:"updatedBy,omitempty"`
	Metadata       *JSON           `json:"metadata,omitempty" gorm:"type:jsonb"`
}

// CreateTicketRequest represents a request to create a new ticket
type CreateTicketRequest struct {
	Title         string         `json:"title" binding:"required"`
	Description   string         `json:"description" binding:"required"`
	Type          TicketType     `json:"type" binding:"required"`
	Priority      TicketPriority `json:"priority" binding:"required"`
	Tags          []string       `json:"tags,omitempty"`
	DueDate       *time.Time     `json:"dueDate,omitempty"`
	EstimatedTime *int           `json:"estimatedTime,omitempty"`
	AssigneeIDs   []string       `json:"assigneeIds,omitempty"`
	Metadata      *JSON          `json:"metadata,omitempty"`
}

// UpdateTicketRequest represents a request to update a ticket
type UpdateTicketRequest struct {
	Title         *string         `json:"title,omitempty"`
	Description   *string         `json:"description,omitempty"`
	Priority      *TicketPriority `json:"priority,omitempty"`
	Tags          []string        `json:"tags,omitempty"`
	DueDate       *time.Time      `json:"dueDate,omitempty"`
	EstimatedTime *int            `json:"estimatedTime,omitempty"`
	ActualTime    *int            `json:"actualTime,omitempty"`
	Metadata      *JSON           `json:"metadata,omitempty"`
}

// PaginationInfo represents pagination information
type PaginationInfo struct {
	Page        int   `json:"page"`
	Limit       int   `json:"limit"`
	Total       int64 `json:"total"`
	TotalPages  int   `json:"totalPages"`
	HasNext     bool  `json:"hasNext"`
	HasPrevious bool  `json:"hasPrevious"`
}

// TicketResponse represents a single ticket response
type TicketResponse struct {
	Success bool    `json:"success"`
	Data    *Ticket `json:"data"`
	Message *string `json:"message,omitempty"`
}

// TicketListResponse represents a list of tickets response
type TicketListResponse struct {
	Success    bool            `json:"success"`
	Data       []Ticket        `json:"data"`
	Pagination *PaginationInfo `json:"pagination"`
	Metadata   *JSON           `json:"metadata,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success   bool   `json:"success"`
	Error     Error  `json:"error"`
	Timestamp string `json:"timestamp,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

// Error represents error details
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Details *JSON  `json:"details,omitempty"`
}

// TableName returns the table name for the Ticket model
func (Ticket) TableName() string {
	return "tickets"
}
