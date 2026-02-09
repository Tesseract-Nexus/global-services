package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tickets-service/internal/clients"
	"tickets-service/internal/events"
	"tickets-service/internal/models"
	"tickets-service/internal/repository"

	gosharedmw "github.com/Tesseract-Nexus/go-shared/middleware"
)

type TicketsHandler struct {
	repo               *repository.TicketsRepository
	notificationClient *clients.NotificationClient
	tenantClient       *clients.TenantClient
	eventsPublisher    *events.Publisher
}

func NewTicketsHandler(repo *repository.TicketsRepository, notificationClient *clients.NotificationClient, tenantClient *clients.TenantClient, eventsPublisher *events.Publisher) *TicketsHandler {
	return &TicketsHandler{
		repo:               repo,
		notificationClient: notificationClient,
		tenantClient:       tenantClient,
		eventsPublisher:    eventsPublisher,
	}
}

// isAdminRole checks if the user role has admin-level permissions
// Recognizes: admin, staff, super_admin, owner, manager
func isAdminRole(role string) bool {
	return role == "admin" || role == "staff" || role == "super_admin" || role == "owner" || role == "manager"
}

// CreateTicket creates a new ticket
func (h *TicketsHandler) CreateTicket(c *gin.Context) {
	var req models.CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")
	userName := c.GetString("userName")
	userEmail := c.GetString("userEmail")

	ticket := &models.Ticket{
		ApplicationID:  "default-app",
		Title:          req.Title,
		Description:    req.Description,
		Type:           req.Type,
		Status:         models.TicketStatusOpen,
		Priority:       req.Priority,
		CreatedBy:      userID,
		CreatedByName:  userName,
		CreatedByEmail: userEmail,
	}

	// Convert tags to JSON
	if len(req.Tags) > 0 {
		tagsJSON := make(models.JSON)
		for i, tag := range req.Tags {
			tagsJSON[strconv.Itoa(i)] = tag
		}
		ticket.Tags = &tagsJSON
	}

	if err := h.repo.CreateTicket(tenantID, ticket); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "CREATE_FAILED",
				Message: "Failed to create ticket",
			},
		})
		return
	}

	// Publish event for audit trail
	if h.eventsPublisher != nil {
		actor := gosharedmw.GetActorInfo(c)
		category := ""
		if ticket.Type != "" {
			category = string(ticket.Type)
		}
		_ = h.eventsPublisher.PublishTicketCreated(
			c.Request.Context(),
			tenantID,
			ticket.ID.String(),
			ticket.TicketNumber,
			ticket.CreatedByEmail,
			ticket.CreatedByName,
			ticket.Title,
			category,
			string(ticket.Priority),
			actor.ActorID,
			actor.ActorName,
			actor.ActorEmail,
			actor.ClientIP,
			actor.UserAgent,
		)
	}

	// Send email notifications (non-blocking)
	if h.notificationClient != nil {
		go func() {
			// Build the ticket URL using tenant slug
			ticketURL := ""
			if h.tenantClient != nil {
				ticketURL = h.tenantClient.BuildTicketURL(context.Background(), tenantID, ticket.ID.String())
			}

			notification := &clients.TicketNotification{
				TenantID:      tenantID,
				TicketID:      ticket.ID.String(),
				TicketNumber:  ticket.TicketNumber,
				Subject:       ticket.Title,
				Description:   ticket.Description,
				Status:        string(ticket.Status),
				Priority:      string(ticket.Priority),
				CustomerEmail: ticket.CreatedByEmail,
				CustomerName:  ticket.CreatedByName,
				TicketURL:     ticketURL,
			}

			if err := h.notificationClient.SendTicketCreatedNotification(context.Background(), notification); err != nil {
				log.Printf("[TicketsHandler] Failed to send ticket notification: %v", err)
			}
		}()
	}

	c.JSON(http.StatusCreated, models.TicketResponse{
		Success: true,
		Data:    ticket,
	})
}

// GetTickets retrieves tickets with pagination
func (h *TicketsHandler) GetTickets(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")
	userRole := c.GetString("userRole")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Build filters
	filters := &repository.TicketFilters{
		Status:   c.Query("status"),
		Priority: c.Query("priority"),
		Type:     c.Query("type"),
	}

	// For non-admin users (storefront), filter by their own user ID
	// Admin users (from admin portal) can see all tickets
	if !isAdminRole(userRole) {
		// Regular user can only see their own tickets
		filters.CreatedBy = userID
	} else if createdBy := c.Query("createdBy"); createdBy != "" {
		// Admin can optionally filter by createdBy
		filters.CreatedBy = createdBy
	}

	tickets, total, err := h.repo.GetTickets(tenantID, page, limit, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to retrieve tickets",
			},
		})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	c.JSON(http.StatusOK, models.TicketListResponse{
		Success:    true,
		Data:       tickets,
		Pagination: pagination,
	})
}

// GetTicket retrieves a single ticket by ID
func (h *TicketsHandler) GetTicket(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")
	userRole := c.GetString("userRole")
	ticketIDStr := c.Param("id")

	ticketID, err := uuid.Parse(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid ticket ID format",
			},
		})
		return
	}

	ticket, err := h.repo.GetTicketByID(tenantID, ticketID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Ticket not found",
			},
		})
		return
	}

	// For non-admin users, only allow viewing their own tickets
	if !isAdminRole(userRole) {
		if ticket.CreatedBy != userID {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "FORBIDDEN",
					Message: "You don't have permission to view this ticket",
				},
			})
			return
		}
	}

	c.JSON(http.StatusOK, models.TicketResponse{
		Success: true,
		Data:    ticket,
	})
}

// UpdateTicket updates a ticket
func (h *TicketsHandler) UpdateTicket(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")
	userRole := c.GetString("userRole")
	ticketIDStr := c.Param("id")

	ticketID, err := uuid.Parse(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid ticket ID format",
			},
		})
		return
	}

	// Get existing ticket to check permissions
	existingTicket, err := h.repo.GetTicketByID(tenantID, ticketID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Ticket not found",
			},
		})
		return
	}

	// Only admin/staff can update any ticket; users can only update their own
	if !isAdminRole(userRole) {
		if existingTicket.CreatedBy != userID {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "FORBIDDEN",
					Message: "You don't have permission to update this ticket",
				},
			})
			return
		}
	}

	var req models.UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REQUEST",
				Message: err.Error(),
			},
		})
		return
	}

	// Build update struct
	updates := &models.Ticket{
		UpdatedBy: &userID,
	}

	if req.Title != nil {
		updates.Title = *req.Title
	}
	if req.Description != nil {
		updates.Description = *req.Description
	}
	if req.Priority != nil {
		updates.Priority = *req.Priority
	}
	if req.DueDate != nil {
		updates.DueDate = req.DueDate
	}
	if req.EstimatedTime != nil {
		updates.EstimatedTime = req.EstimatedTime
	}
	if req.ActualTime != nil {
		updates.ActualTime = req.ActualTime
	}
	if req.Metadata != nil {
		updates.Metadata = req.Metadata
	}

	// Convert tags to JSON
	if len(req.Tags) > 0 {
		tagsJSON := make(models.JSON)
		for i, tag := range req.Tags {
			tagsJSON[strconv.Itoa(i)] = tag
		}
		updates.Tags = &tagsJSON
	}

	if err := h.repo.UpdateTicket(tenantID, ticketID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update ticket",
			},
		})
		return
	}

	// Fetch the updated ticket
	updatedTicket, err := h.repo.GetTicketByID(tenantID, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch updated ticket",
			},
		})
		return
	}

	// Publish event for audit trail
	if h.eventsPublisher != nil {
		actor := gosharedmw.GetActorInfo(c)
		_ = h.eventsPublisher.PublishTicketUpdated(
			c.Request.Context(),
			tenantID,
			updatedTicket.ID.String(),
			updatedTicket.TicketNumber,
			updatedTicket.CreatedByEmail,
			updatedTicket.Title,
			string(updatedTicket.Status),
			actor.ActorID,
			actor.ActorName,
			actor.ActorEmail,
			actor.ClientIP,
			actor.UserAgent,
		)
	}

	c.JSON(http.StatusOK, models.TicketResponse{
		Success: true,
		Data:    updatedTicket,
	})
}

// DeleteTicket deletes a ticket
func (h *TicketsHandler) DeleteTicket(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")
	userRole := c.GetString("userRole")
	ticketIDStr := c.Param("id")

	ticketID, err := uuid.Parse(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid ticket ID format",
			},
		})
		return
	}

	// Get existing ticket to check permissions and for audit
	existingTicket, err := h.repo.GetTicketByID(tenantID, ticketID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Ticket not found",
			},
		})
		return
	}

	// Only admin/staff can delete tickets
	if !isAdminRole(userRole) {
		if existingTicket.CreatedBy != userID {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "FORBIDDEN",
					Message: "You don't have permission to delete this ticket",
				},
			})
			return
		}
	}

	if err := h.repo.DeleteTicket(tenantID, ticketID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DELETE_FAILED",
				Message: "Failed to delete ticket",
			},
		})
		return
	}

	// Publish event for audit trail
	if h.eventsPublisher != nil {
		actor := gosharedmw.GetActorInfo(c)
		_ = h.eventsPublisher.PublishTicketDeleted(
			c.Request.Context(),
			tenantID,
			existingTicket.ID.String(),
			existingTicket.TicketNumber,
			existingTicket.Title,
			actor.ActorID,
			actor.ActorName,
			actor.ActorEmail,
			actor.ClientIP,
			actor.UserAgent,
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Ticket deleted successfully",
	})
}

// UpdateTicketStatus updates the status of a ticket
func (h *TicketsHandler) UpdateTicketStatus(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")
	userRole := c.GetString("userRole")
	ticketIDStr := c.Param("id")

	ticketID, err := uuid.Parse(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid ticket ID format",
			},
		})
		return
	}

	// Get existing ticket to check permissions
	existingTicket, err := h.repo.GetTicketByID(tenantID, ticketID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Ticket not found",
			},
		})
		return
	}

	// Only admin/staff can update status on any ticket
	if !isAdminRole(userRole) {
		if existingTicket.CreatedBy != userID {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "FORBIDDEN",
					Message: "You don't have permission to update this ticket",
				},
			})
			return
		}
	}

	var req struct {
		Status models.TicketStatus `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REQUEST",
				Message: "Status is required",
			},
		})
		return
	}

	if err := h.repo.UpdateTicketStatus(tenantID, ticketID, req.Status, userID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "UPDATE_FAILED",
				Message: "Failed to update ticket status",
			},
		})
		return
	}

	// Fetch the updated ticket
	updatedTicket, err := h.repo.GetTicketByID(tenantID, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FETCH_FAILED",
				Message: "Failed to fetch updated ticket",
			},
		})
		return
	}

	// Send email notifications for all status changes (non-blocking)
	if h.notificationClient != nil {
		// Capture user info and old status for goroutine
		userName := c.GetString("userName")
		if userName == "" {
			userName = "Support Team"
		}
		oldStatus := string(existingTicket.Status)

		go func() {
			ticketURL := ""
			if h.tenantClient != nil {
				ticketURL = h.tenantClient.BuildTicketURL(context.Background(), tenantID, updatedTicket.ID.String())
			}

			notification := &clients.TicketNotification{
				TenantID:      tenantID,
				TicketID:      updatedTicket.ID.String(),
				TicketNumber:  updatedTicket.TicketNumber,
				Subject:       updatedTicket.Title,
				Description:   updatedTicket.Description,
				Status:        string(updatedTicket.Status),
				Priority:      string(updatedTicket.Priority),
				CustomerEmail: updatedTicket.CreatedByEmail,
				CustomerName:  updatedTicket.CreatedByName,
				TicketURL:     ticketURL,
			}

			var notifErr error
			switch req.Status {
			case models.TicketStatusResolved:
				notifErr = h.notificationClient.SendTicketResolvedNotification(context.Background(), notification, "Your issue has been resolved.", userName)
			case models.TicketStatusClosed:
				notifErr = h.notificationClient.SendTicketClosedNotification(context.Background(), notification, userName)
			case models.TicketStatusInProgress:
				notifErr = h.notificationClient.SendTicketInProgressNotification(context.Background(), notification, userName)
			case models.TicketStatusOnHold:
				notifErr = h.notificationClient.SendTicketOnHoldNotification(context.Background(), notification, "Awaiting additional information or resources.", userName)
			case models.TicketStatusEscalated:
				notifErr = h.notificationClient.SendTicketEscalatedNotification(context.Background(), notification, userName, "Requires immediate attention from senior support.")
			case models.TicketStatusReopened:
				notifErr = h.notificationClient.SendTicketReopenedNotification(context.Background(), notification, userName)
			case models.TicketStatusCancelled:
				notifErr = h.notificationClient.SendTicketCancelledNotification(context.Background(), notification, userName, "Ticket has been cancelled as requested.")
			case models.TicketStatusPendingApproval:
				notifErr = h.notificationClient.SendTicketStatusUpdateNotification(context.Background(), notification, oldStatus, string(req.Status), userName)
			case models.TicketStatusOpen:
				// No notification for OPEN status (handled by CreateTicket)
			default:
				// Generic status update for any other status
				notifErr = h.notificationClient.SendTicketStatusUpdateNotification(context.Background(), notification, oldStatus, string(req.Status), userName)
			}
			if notifErr != nil {
				log.Printf("[TicketsHandler] Failed to send status notification: %v", notifErr)
			}
		}()
	}

	// Publish event for audit trail
	if h.eventsPublisher != nil {
		actor := gosharedmw.GetActorInfo(c)
		_ = h.eventsPublisher.PublishTicketStatusChanged(
			c.Request.Context(),
			tenantID,
			updatedTicket.ID.String(),
			updatedTicket.TicketNumber,
			updatedTicket.CreatedByEmail,
			updatedTicket.Title,
			string(existingTicket.Status),
			string(updatedTicket.Status),
			actor.ActorID,
			actor.ActorName,
			actor.ActorEmail,
			actor.ClientIP,
			actor.UserAgent,
		)
	}

	c.JSON(http.StatusOK, models.TicketResponse{
		Success: true,
		Data:    updatedTicket,
	})
}

func (h *TicketsHandler) AssignTicket(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) UnassignTicket(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}

// AddComment adds a comment to a ticket
func (h *TicketsHandler) AddComment(c *gin.Context) {
	tenantID := c.GetString("tenantId")
	userID := c.GetString("userId")
	userName := c.GetString("userName")
	userRole := c.GetString("userRole")
	ticketIDStr := c.Param("id")

	ticketID, err := uuid.Parse(ticketIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ID",
				Message: "Invalid ticket ID format",
			},
		})
		return
	}

	// Get existing ticket to check permissions
	existingTicket, err := h.repo.GetTicketByID(tenantID, ticketID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NOT_FOUND",
				Message: "Ticket not found",
			},
		})
		return
	}

	// Only admin/staff can comment on any ticket; users can only comment on their own
	if !isAdminRole(userRole) {
		if existingTicket.CreatedBy != userID {
			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Success: false,
				Error: models.Error{
					Code:    "FORBIDDEN",
					Message: "You don't have permission to comment on this ticket",
				},
			})
			return
		}
	}

	var req struct {
		Content    string `json:"content" binding:"required"`
		IsInternal bool   `json:"isInternal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_REQUEST",
				Message: "Comment content is required",
			},
		})
		return
	}

	// Build comment object
	comment := map[string]interface{}{
		"userId":     userID,
		"userName":   userName,
		"content":    req.Content,
		"isInternal": req.IsInternal,
		"createdAt":  c.GetTime("requestTime"),
	}

	// Use current time if requestTime not set
	if comment["createdAt"] == nil || comment["createdAt"].(interface{}) == nil {
		comment["createdAt"] = time.Now().Format(time.RFC3339)
	}

	updatedTicket, err := h.repo.AddComment(tenantID, ticketID, comment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "ADD_COMMENT_FAILED",
				Message: "Failed to add comment",
			},
		})
		return
	}

	// Publish event for audit trail
	if h.eventsPublisher != nil {
		actor := gosharedmw.GetActorInfo(c)
		_ = h.eventsPublisher.PublishTicketCommentAdded(
			c.Request.Context(),
			tenantID,
			updatedTicket.ID.String(),
			updatedTicket.TicketNumber,
			updatedTicket.Title,
			req.Content,
			req.IsInternal,
			actor.ActorID,
			actor.ActorName,
			actor.ActorEmail,
			actor.ClientIP,
			actor.UserAgent,
		)
	}

	c.JSON(http.StatusCreated, models.TicketResponse{
		Success: true,
		Data:    updatedTicket,
	})
}
func (h *TicketsHandler) UpdateComment(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) DeleteComment(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) AddAttachment(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) DeleteAttachment(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) BulkUpdateStatus(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) BulkAssign(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) BulkUpdatePriority(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) GetAnalytics(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) GetStats(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) ExportTickets(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) EscalateTicket(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) CloneTicket(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) SearchTickets(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
func (h *TicketsHandler) GetSimilarTickets(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "Not implemented yet"})
}
