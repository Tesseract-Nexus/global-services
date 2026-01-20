package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"notification-service/internal/middleware"
	"notification-service/internal/models"
	"notification-service/internal/repository"
	"notification-service/internal/services"
	"notification-service/internal/template"
	"notification-service/internal/templates"
)

// NotificationHandler handles notification-related requests
type NotificationHandler struct {
	notifRepo    repository.NotificationRepository
	templateRepo repository.TemplateRepository
	prefRepo     repository.PreferenceRepository
	sender       *NotificationSender
	templateEng  *template.Engine
	rateLimiter  *middleware.EmailRateLimiter
}

// NotificationSender sends notifications via different channels
type NotificationSender struct {
	emailProvider services.Provider
	smsProvider   services.Provider
	pushProvider  services.Provider
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(
	notifRepo repository.NotificationRepository,
	templateRepo repository.TemplateRepository,
	prefRepo repository.PreferenceRepository,
	emailProvider services.Provider,
	smsProvider services.Provider,
	pushProvider services.Provider,
) *NotificationHandler {
	return &NotificationHandler{
		notifRepo:    notifRepo,
		templateRepo: templateRepo,
		prefRepo:     prefRepo,
		sender: &NotificationSender{
			emailProvider: emailProvider,
			smsProvider:   smsProvider,
			pushProvider:  pushProvider,
		},
		templateEng: template.NewEngine(),
	}
}

// NewNotificationHandlerWithRateLimiter creates a new notification handler with rate limiting
func NewNotificationHandlerWithRateLimiter(
	notifRepo repository.NotificationRepository,
	templateRepo repository.TemplateRepository,
	prefRepo repository.PreferenceRepository,
	emailProvider services.Provider,
	smsProvider services.Provider,
	pushProvider services.Provider,
	rateLimiter *middleware.EmailRateLimiter,
) *NotificationHandler {
	return &NotificationHandler{
		notifRepo:    notifRepo,
		templateRepo: templateRepo,
		prefRepo:     prefRepo,
		sender: &NotificationSender{
			emailProvider: emailProvider,
			smsProvider:   smsProvider,
			pushProvider:  pushProvider,
		},
		templateEng: template.NewEngine(),
		rateLimiter: rateLimiter,
	}
}

// SetRateLimiter sets the email rate limiter (for optional initialization)
func (h *NotificationHandler) SetRateLimiter(rateLimiter *middleware.EmailRateLimiter) {
	h.rateLimiter = rateLimiter
}

// SendRequest represents a send notification request
type SendRequest struct {
	Channel        string                 `json:"channel" binding:"required,oneof=EMAIL SMS PUSH"`
	TemplateID     string                 `json:"templateId"`
	TemplateName   string                 `json:"templateName"`
	RecipientEmail string                 `json:"recipientEmail"`
	RecipientPhone string                 `json:"recipientPhone"`
	RecipientToken string                 `json:"recipientToken"`
	RecipientID    string                 `json:"recipientId"`
	Subject        string                 `json:"subject"`
	Body           string                 `json:"body"`
	BodyHTML       string                 `json:"bodyHtml"`
	Variables      map[string]interface{} `json:"variables"`
	Metadata       map[string]interface{} `json:"metadata"`
	Priority       string                 `json:"priority"`
	ScheduledFor   *time.Time             `json:"scheduledFor"`
}

// Send sends a notification
func (h *NotificationHandler) Send(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id"})
		return
	}

	// Get user ID from context (set by middleware)
	userIDStr := c.GetString("user_id")
	var userID uuid.UUID
	if userIDStr != "" {
		if parsed, err := uuid.Parse(userIDStr); err == nil {
			userID = parsed
		}
	}
	// Use a system user ID if not provided
	if userID == uuid.Nil {
		userID = uuid.MustParse("00000000-0000-0000-0000-000000000001") // System user
	}

	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate recipient based on channel
	switch models.NotificationChannel(req.Channel) {
	case models.ChannelEmail:
		if req.RecipientEmail == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "recipientEmail required for EMAIL channel"})
			return
		}
	case models.ChannelSMS:
		if req.RecipientPhone == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "recipientPhone required for SMS channel"})
			return
		}
	case models.ChannelPush:
		if req.RecipientToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "recipientToken required for PUSH channel"})
			return
		}
	}

	// Check email rate limits for EMAIL channel
	if models.NotificationChannel(req.Channel) == models.ChannelEmail && h.rateLimiter != nil {
		// Determine the action type based on template name or metadata
		action := h.getEmailAction(req.TemplateName, req.Metadata)

		result, err := h.rateLimiter.CheckLimit(c.Request.Context(), tenantID, req.RecipientEmail, action)
		if err != nil {
			log.Printf("[NotificationHandler] Rate limit check error: %v", err)
			// Continue even if rate limit check fails (fail-open for availability)
		} else if !result.Allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":          "Email rate limit exceeded",
				"code":           "EMAIL_RATE_LIMITED",
				"limit_type":     result.LimitType,
				"remaining":      result.Remaining,
				"retry_after_sec": result.RetryAfterSec,
			})
			return
		}
	}

	// Derive title from subject or use default
	title := req.Subject
	if title == "" {
		title = "Notification"
	}

	// Derive notification type from channel
	notifType := "notification." + string(models.NotificationChannel(req.Channel))

	// Create notification
	notification := &models.Notification{
		TenantID:       tenantID,
		UserID:         userID,
		Type:           notifType,
		Title:          title,
		Message:        req.Body,
		SourceService:  "notification-service",
		Channel:        models.NotificationChannel(req.Channel),
		Status:         models.StatusPending,
		Priority:       models.NotificationPriority(req.Priority),
		RecipientEmail: req.RecipientEmail,
		RecipientPhone: req.RecipientPhone,
		RecipientToken: req.RecipientToken,
		Subject:        req.Subject,
		Body:           req.Body,
		BodyHTML:       req.BodyHTML,
		ScheduledFor:   req.ScheduledFor,
	}

	// Parse recipient ID if provided
	if req.RecipientID != "" {
		if recipientID, err := uuid.Parse(req.RecipientID); err == nil {
			notification.RecipientID = &recipientID
		}
	}

	// Set priority default
	if notification.Priority == "" {
		notification.Priority = models.PriorityNormal
	}

	// Handle template if provided
	if req.TemplateName != "" || req.TemplateID != "" {
		var tmpl *models.NotificationTemplate
		var err error
		var usedEmbeddedTemplate bool

		// First try database template
		if req.TemplateID != "" {
			templateID, _ := uuid.Parse(req.TemplateID)
			tmpl, err = h.templateRepo.GetByID(c.Request.Context(), templateID)
		} else {
			tmpl, err = h.templateRepo.GetByName(c.Request.Context(), tenantID, req.TemplateName)
		}

		// If database template found, use it
		if err == nil && tmpl != nil {
			notification.TemplateID = &tmpl.ID
			notification.TemplateName = tmpl.Name

			// Render database template
			if tmpl.Subject != "" && notification.Subject == "" {
				if rendered, err := h.templateEng.RenderText(tmpl.Subject, req.Variables); err == nil {
					notification.Subject = rendered
				}
			}
			if tmpl.BodyTemplate != "" && notification.Body == "" {
				if rendered, err := h.templateEng.RenderText(tmpl.BodyTemplate, req.Variables); err == nil {
					notification.Body = rendered
				}
			}
			if tmpl.HTMLTemplate != "" && notification.BodyHTML == "" {
				if rendered, err := h.templateEng.RenderHTML(tmpl.HTMLTemplate, req.Variables); err == nil {
					notification.BodyHTML = rendered
				}
			}
		} else if req.TemplateName != "" {
			// Fallback to embedded templates
			// Normalize template name: convert dashes to underscores (e.g., "ticket-created" -> "ticket_created")
			embeddedTemplateName := strings.ReplaceAll(req.TemplateName, "-", "_")

			renderer, renderErr := templates.GetDefaultRenderer()
			if renderErr != nil {
				log.Printf("[NotificationHandler] Failed to get embedded template renderer: %v", renderErr)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Template renderer not available"})
				return
			}

			// Build EmailData from request variables
			emailData := buildEmailDataFromVariables(req.Variables, notification.Subject, req.RecipientEmail)

			// Try to render the embedded template
			renderedHTML, renderErr := renderer.Render(embeddedTemplateName, emailData)
			if renderErr != nil {
				log.Printf("[NotificationHandler] Template '%s' not found in DB or embedded templates: %v", req.TemplateName, renderErr)
				c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
				return
			}

			notification.TemplateName = embeddedTemplateName
			notification.BodyHTML = renderedHTML
			usedEmbeddedTemplate = true
			log.Printf("[NotificationHandler] Using embedded template: %s", embeddedTemplateName)
		}

		_ = usedEmbeddedTemplate // silence unused variable warning
	}

	// Store variables and metadata
	if req.Variables != nil {
		if jsonData, err := json.Marshal(req.Variables); err == nil {
			notification.Variables = jsonData
		}
	}
	if req.Metadata != nil {
		if jsonData, err := json.Marshal(req.Metadata); err == nil {
			notification.Metadata = jsonData
		}
	}

	// Save notification
	if err := h.notifRepo.Create(c.Request.Context(), notification); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
		return
	}

	// Send immediately if not scheduled
	if notification.ScheduledFor == nil || notification.ScheduledFor.Before(time.Now()) {
		// For HIGH/CRITICAL priority notifications (like verification emails), send synchronously
		// so the caller knows immediately if sending failed
		if notification.Priority == models.PriorityHigh || notification.Priority == models.PriorityCritical {
			sendErr := h.sendNotificationSync(c.Request.Context(), notification)
			if sendErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   sendErr.Error(),
					"data":    notification,
				})
				return
			}
		} else {
			// For normal/low priority, send asynchronously
			go h.sendNotification(context.Background(), notification)
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    notification,
	})
}

// sendNotificationSync sends the notification synchronously and returns any error
// Used for high-priority notifications where the caller needs immediate feedback
func (h *NotificationHandler) sendNotificationSync(ctx context.Context, notification *models.Notification) error {
	var provider services.Provider
	var recipient string

	switch notification.Channel {
	case models.ChannelEmail:
		provider = h.sender.emailProvider
		recipient = notification.RecipientEmail
	case models.ChannelSMS:
		provider = h.sender.smsProvider
		recipient = notification.RecipientPhone
	case models.ChannelPush:
		provider = h.sender.pushProvider
		recipient = notification.RecipientToken
	default:
		err := fmt.Errorf("unknown channel: %s", notification.Channel)
		h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", err.Error())
		return err
	}

	if provider == nil {
		err := fmt.Errorf("provider not configured for channel: %s", notification.Channel)
		h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", err.Error())
		return err
	}

	// Update status to sending
	h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusSending, "", "")

	// Prepare message
	message := &services.Message{
		To:       recipient,
		Subject:  notification.Subject,
		Body:     notification.Body,
		BodyHTML: notification.BodyHTML,
	}

	// Parse metadata for push notifications
	if notification.Channel == models.ChannelPush && notification.Metadata != nil {
		var metadata map[string]interface{}
		if err := json.Unmarshal(notification.Metadata, &metadata); err == nil {
			message.Metadata = metadata
		}
	}

	// Send
	result, err := provider.Send(ctx, message)
	if err != nil {
		h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", err.Error())
		return fmt.Errorf("failed to send notification: %w", err)
	}

	if result.Success {
		h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusSent, result.ProviderID, "")

		// Record successful email send for rate limiting
		if notification.Channel == models.ChannelEmail && h.rateLimiter != nil {
			action := h.getEmailAction(notification.TemplateName, nil)
			if err := h.rateLimiter.RecordSend(ctx, notification.TenantID, notification.RecipientEmail, action); err != nil {
				log.Printf("[NotificationHandler] Failed to record email send for rate limiting: %v", err)
			}
		}

		return nil
	}

	errorMsg := "send failed"
	if result.Error != nil {
		errorMsg = result.Error.Error()
	}
	h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", errorMsg)
	return fmt.Errorf("notification send failed: %s", errorMsg)
}

// sendNotification sends the notification via the appropriate channel
func (h *NotificationHandler) sendNotification(ctx context.Context, notification *models.Notification) {
	var provider services.Provider
	var recipient string

	switch notification.Channel {
	case models.ChannelEmail:
		provider = h.sender.emailProvider
		recipient = notification.RecipientEmail
	case models.ChannelSMS:
		provider = h.sender.smsProvider
		recipient = notification.RecipientPhone
	case models.ChannelPush:
		provider = h.sender.pushProvider
		recipient = notification.RecipientToken
	default:
		h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", "Unknown channel")
		return
	}

	if provider == nil {
		h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", "Provider not configured")
		return
	}

	// Update status to sending
	h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusSending, "", "")

	// Prepare message
	message := &services.Message{
		To:       recipient,
		Subject:  notification.Subject,
		Body:     notification.Body,
		BodyHTML: notification.BodyHTML,
	}

	// Parse metadata for push notifications
	if notification.Channel == models.ChannelPush && notification.Metadata != nil {
		var metadata map[string]interface{}
		if err := json.Unmarshal(notification.Metadata, &metadata); err == nil {
			message.Metadata = metadata
		}
	}

	// Send
	result, err := provider.Send(ctx, message)
	if err != nil {
		h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", err.Error())
		return
	}

	if result.Success {
		h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusSent, result.ProviderID, "")

		// Record successful email send for rate limiting
		if notification.Channel == models.ChannelEmail && h.rateLimiter != nil {
			action := h.getEmailAction(notification.TemplateName, nil)
			if err := h.rateLimiter.RecordSend(ctx, notification.TenantID, notification.RecipientEmail, action); err != nil {
				log.Printf("[NotificationHandler] Failed to record email send for rate limiting: %v", err)
			}
		}
	} else {
		errorMsg := "Send failed"
		if result.Error != nil {
			errorMsg = result.Error.Error()
		}
		h.notifRepo.UpdateStatus(ctx, notification.ID, models.StatusFailed, "", errorMsg)
	}
}

// List returns notifications for a tenant
func (h *NotificationHandler) List(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing tenant_id"})
		return
	}

	filters := repository.NotificationFilters{
		Channel: c.Query("channel"),
		Status:  c.Query("status"),
		Limit:   parseIntWithDefault(c.Query("limit"), 50),
		Offset:  parseIntWithDefault(c.Query("offset"), 0),
	}

	notifications, total, err := h.notifRepo.List(c.Request.Context(), tenantID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    notifications,
		"pagination": gin.H{
			"limit":  filters.Limit,
			"offset": filters.Offset,
			"total":  total,
		},
	})
}

// Get returns a single notification
func (h *NotificationHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	notification, err := h.notifRepo.GetByID(c.Request.Context(), id)
	if err != nil || notification == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    notification,
	})
}

// GetStatus returns the delivery status of a notification
func (h *NotificationHandler) GetStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	notification, err := h.notifRepo.GetByID(c.Request.Context(), id)
	if err != nil || notification == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":          notification.ID,
			"status":      notification.Status,
			"provider":    notification.Provider,
			"providerId":  notification.ProviderID,
			"sentAt":      notification.SentAt,
			"deliveredAt": notification.DeliveredAt,
			"failedAt":    notification.FailedAt,
			"errorMessage": notification.ErrorMessage,
			"retryCount":  notification.RetryCount,
		},
	})
}

// Cancel cancels a scheduled notification
func (h *NotificationHandler) Cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	notification, err := h.notifRepo.GetByID(c.Request.Context(), id)
	if err != nil || notification == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		return
	}

	// Can only cancel pending/queued notifications
	if notification.Status != models.StatusPending && notification.Status != models.StatusQueued {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot cancel notification in current status"})
		return
	}

	if err := h.notifRepo.UpdateStatus(c.Request.Context(), id, models.StatusCancelled, "", "Cancelled by user"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel notification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Notification cancelled",
	})
}

// Helper functions
func parseIntWithDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var val int
	if err := json.Unmarshal([]byte(s), &val); err != nil {
		return defaultVal
	}
	return val
}

// buildEmailDataFromVariables converts request variables map to EmailData struct for embedded templates
func buildEmailDataFromVariables(variables map[string]interface{}, subject, recipientEmail string) *templates.EmailData {
	data := &templates.EmailData{
		Subject: subject,
		Email:   recipientEmail,
	}

	if variables == nil {
		return data
	}

	// Helper functions for type conversion
	getString := func(key string) string {
		if val, ok := variables[key]; ok {
			if s, ok := val.(string); ok {
				return s
			}
		}
		return ""
	}

	getInt := func(key string) int {
		if val, ok := variables[key]; ok {
			switch v := val.(type) {
			case int:
				return v
			case float64:
				return int(v)
			case int64:
				return int(v)
			}
		}
		return 0
	}

	// Common fields
	data.BusinessName = getString("businessName")
	data.SupportEmail = getString("supportEmail")
	data.CustomerName = getString("customerName")

	// Ticket fields
	data.TicketID = getString("ticketId")
	data.TicketNumber = getString("ticketNumber")
	data.TicketSubject = getString("subject")
	data.TicketCategory = getString("category")
	data.TicketPriority = getString("priority")
	data.TicketStatus = getString("status")
	data.Description = getString("description")
	data.Resolution = getString("resolution")
	data.TicketURL = getString("ticketUrl")

	// Order fields
	data.OrderNumber = getString("orderNumber")
	data.OrderDate = getString("orderDate")
	data.Currency = getString("currency")
	data.Subtotal = getString("subtotal")
	data.Discount = getString("discount")
	data.Shipping = getString("shipping")
	data.Tax = getString("tax")
	data.Total = getString("total")
	data.PaymentMethod = getString("paymentMethod")
	data.TrackingURL = getString("trackingUrl")
	data.OrderDetailsURL = getString("orderDetailsUrl")

	// Shipping fields
	data.Carrier = getString("carrier")
	data.TrackingNumber = getString("trackingNumber")
	data.EstimatedDelivery = getString("estimatedDelivery")
	data.DeliveryDate = getString("deliveryDate")

	// Payment fields
	data.TransactionID = getString("transactionId")
	data.Amount = getString("amount")
	data.PaymentDate = getString("paymentDate")
	data.FailureReason = getString("failureReason")
	data.RetryURL = getString("retryUrl")
	data.RefundAmount = getString("refundAmount")

	// Review fields
	data.ReviewID = getString("reviewId")
	data.ProductName = getString("productName")
	data.ProductSKU = getString("productSku")
	data.ReviewTitle = getString("reviewTitle")
	data.ReviewContent = getString("reviewContent")
	data.Rating = getInt("rating")
	data.MaxRating = getInt("maxRating")
	if data.MaxRating == 0 {
		data.MaxRating = 5
	}
	data.ReviewDate = getString("reviewDate")
	data.ReviewStatus = getString("reviewStatus")
	data.RejectReason = getString("rejectReason")
	data.ReviewsURL = getString("reviewsUrl")
	data.ProductURL = getString("productUrl")

	// Vendor fields
	data.VendorID = getString("vendorId")
	data.VendorName = getString("vendorName")
	data.VendorEmail = getString("vendorEmail")
	data.VendorBusinessName = getString("vendorBusinessName")
	data.VendorStatus = getString("vendorStatus")
	data.StatusReason = getString("statusReason")
	data.VendorURL = getString("vendorUrl")

	// Coupon fields
	data.CouponID = getString("couponId")
	data.CouponCode = getString("couponCode")
	data.DiscountType = getString("discountType")
	data.DiscountValue = getString("discountValue")
	data.DiscountAmount = getString("discountAmount")
	data.ValidFrom = getString("validFrom")
	data.ValidUntil = getString("validUntil")

	// Auth fields
	data.ResetCode = getString("resetCode")
	data.ResetURL = getString("resetUrl")

	// Tenant fields
	data.TenantSlug = getString("tenantSlug")
	data.AdminURL = getString("adminUrl")
	data.StorefrontURL = getString("storefrontUrl")

	// Staff invitation fields
	data.StaffName = getString("staffName")
	data.StaffEmail = getString("staffEmail")
	data.InviterName = getString("inviterName")
	data.Role = getString("role")
	data.ActivationLink = getString("activationLink")

	return data
}

// getEmailAction determines the email action type based on template name or metadata
func (h *NotificationHandler) getEmailAction(templateName string, metadata map[string]interface{}) middleware.EmailAction {
	// Normalize template name to lowercase for comparison
	name := strings.ToLower(templateName)

	// Password reset related templates
	if strings.Contains(name, "password_reset") || strings.Contains(name, "password-reset") ||
		strings.Contains(name, "reset_password") || strings.Contains(name, "reset-password") {
		return middleware.ActionPasswordReset
	}

	// Email verification templates
	if strings.Contains(name, "verification") || strings.Contains(name, "verify") ||
		strings.Contains(name, "confirm_email") || strings.Contains(name, "confirm-email") ||
		strings.Contains(name, "email_confirm") || strings.Contains(name, "email-confirm") {
		return middleware.ActionVerification
	}

	// OTP templates
	if strings.Contains(name, "otp") || strings.Contains(name, "one_time") || strings.Contains(name, "one-time") {
		return middleware.ActionOTP
	}

	// Security alert templates
	if strings.Contains(name, "security") || strings.Contains(name, "alert") ||
		strings.Contains(name, "suspicious") || strings.Contains(name, "lockout") {
		return middleware.ActionSecurityAlert
	}

	// Account lockout templates
	if strings.Contains(name, "account_locked") || strings.Contains(name, "account-locked") {
		return middleware.ActionAccountLockout
	}

	// Check metadata for action type hint
	if metadata != nil {
		if action, ok := metadata["email_action"].(string); ok {
			switch strings.ToLower(action) {
			case "password_reset":
				return middleware.ActionPasswordReset
			case "verification":
				return middleware.ActionVerification
			case "otp":
				return middleware.ActionOTP
			case "security_alert":
				return middleware.ActionSecurityAlert
			}
		}
	}

	// Default to general email
	return middleware.ActionGeneral
}
