package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tickets-service/internal/models"
)

type DocumentHandler struct {
	documentServiceURL string
	productID          string
	httpClient         *http.Client
}

type DocumentUploadRequest struct {
	TicketID       string `form:"ticket_id" binding:"required"`
	AttachmentType string `form:"attachment_type" binding:"required"` // screenshot, log_file, document, evidence, solution
	IsPublic       bool   `form:"isPublic"`
	Tags           string `form:"tags"`
	Bucket         string `form:"bucket"`
	Description    string `form:"description"`
}

type DocumentListRequest struct {
	TicketID       string `json:"ticket_id" binding:"required"`
	AttachmentType string `json:"attachment_type"`
	Bucket         string `json:"bucket"`
	Prefix         string `json:"prefix"`
	Limit          int    `json:"limit"`
}

type DocumentResponse struct {
	Success bool          `json:"success"`
	Data    interface{}   `json:"data,omitempty"`
	Error   *models.Error `json:"error,omitempty"`
}

func NewDocumentHandler(documentServiceURL, productID string) *DocumentHandler {
	if productID == "" {
		productID = "marketplace" // Default for backwards compatibility
	}
	return &DocumentHandler{
		documentServiceURL: strings.TrimSuffix(documentServiceURL, "/"),
		productID:          productID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UploadTicketAttachment uploads an attachment for a ticket
// @Summary Upload attachment for ticket
// @Description Upload files (screenshots, logs, documents) and associate them with a ticket
// @Tags ticket-attachments
// @Accept multipart/form-data
// @Produce json
// @Param ticket_id formData string true "Ticket ID"
// @Param attachment_type formData string true "Attachment type (screenshot, log_file, document, evidence, solution)"
// @Param file formData file true "Attachment file"
// @Param isPublic formData bool false "Is attachment public"
// @Param tags formData string false "Attachment tags (comma-separated key:value pairs)"
// @Param bucket formData string false "Storage bucket name"
// @Param description formData string false "Attachment description"
// @Success 201 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /tickets/attachments/upload [post]
func (h *DocumentHandler) UploadTicketAttachment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	if tenantID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "MISSING_TENANT",
				Message: "Tenant ID is required",
			},
		})
		return
	}

	var req DocumentUploadRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Validate ticket ID format
	if _, err := uuid.Parse(req.TicketID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_TICKET_ID",
				Message: "Invalid ticket ID format",
			},
		})
		return
	}

	// Validate attachment type
	validTypes := []string{"screenshot", "log_file", "document", "evidence", "solution", "config_file", "error_dump"}
	isValidType := false
	for _, validType := range validTypes {
		if req.AttachmentType == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_ATTACHMENT_TYPE",
				Message: fmt.Sprintf("Attachment type must be one of: %s", strings.Join(validTypes, ", ")),
			},
		})
		return
	}

	// Get the uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "NO_FILE",
				Message: "No file uploaded",
			},
		})
		return
	}
	defer file.Close()

	// Validate file type based on attachment type
	contentType := header.Header.Get("Content-Type")
	var allowedTypes []string

	switch req.AttachmentType {
	case "screenshot":
		allowedTypes = []string{"image/jpeg", "image/png", "image/jpg", "image/gif", "image/webp"}
	case "log_file", "config_file", "error_dump":
		allowedTypes = []string{"text/plain", "text/log", "application/json", "text/xml", "application/xml"}
	case "document", "evidence", "solution":
		allowedTypes = []string{
			"application/pdf",
			"application/msword",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"text/plain",
			"application/vnd.ms-excel",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"image/jpeg", "image/png", "image/jpg",
		}
	default:
		// Allow common file types for other attachment types
		allowedTypes = []string{
			"application/pdf", "text/plain", "image/jpeg", "image/png", "image/jpg",
			"application/zip", "application/json", "text/xml",
		}
	}

	// Extract base media type (strip parameters like charset)
	mediaType := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	isValidFileType := false
	for _, allowedType := range allowedTypes {
		if mediaType == allowedType {
			isValidFileType = true
			break
		}
	}

	if !isValidFileType {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_FILE_TYPE",
				Message: fmt.Sprintf("File type not allowed for %s. Allowed types: %s", req.AttachmentType, strings.Join(allowedTypes, ", ")),
			},
		})
		return
	}

	// Check file size limits
	const maxImageSize = 10 << 20    // 10MB for images
	const maxDocumentSize = 50 << 20 // 50MB for documents
	const maxLogSize = 100 << 20     // 100MB for log files

	fileSize := header.Size
	var maxSize int64

	switch req.AttachmentType {
	case "screenshot":
		maxSize = maxImageSize
	case "log_file", "config_file", "error_dump":
		maxSize = maxLogSize
	case "document", "evidence", "solution":
		maxSize = maxDocumentSize
	default:
		maxSize = maxDocumentSize
	}

	if fileSize > maxSize {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FILE_TOO_LARGE",
				Message: fmt.Sprintf("File size exceeds maximum allowed size of %dMB for %s", maxSize>>20, req.AttachmentType),
			},
		})
		return
	}

	// Create a new multipart form for the document service
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add form fields
	if req.Bucket == "" {
		req.Bucket = "ticket-attachments"
	}
	writer.WriteField("bucket", req.Bucket)
	writer.WriteField("isPublic", fmt.Sprintf("%t", req.IsPublic))

	// Add ticket-specific tags
	ticketTags := fmt.Sprintf("ticket_id:%s,tenant_id:%s,attachment_type:%s", req.TicketID, tenantID, req.AttachmentType)
	if req.Description != "" {
		ticketTags += fmt.Sprintf(",description:%s", req.Description)
	}
	if req.Tags != "" {
		ticketTags += "," + req.Tags
	}
	writer.WriteField("tags", ticketTags)

	// Add the file
	part, err := writer.CreateFormFile("file", header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FORM_CREATE_FAILED",
				Message: "Failed to create form",
			},
		})
		return
	}

	if _, err := io.Copy(part, file); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "FILE_COPY_FAILED",
				Message: "Failed to copy file",
			},
		})
		return
	}

	writer.Close()

	// Forward request to document service
	docReq, err := http.NewRequest("POST", h.documentServiceURL+"/api/v1/documents/upload", &body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REQUEST_CREATE_FAILED",
				Message: "Failed to create request",
			},
		})
		return
	}

	// Set headers
	docReq.Header.Set("Content-Type", writer.FormDataContentType())
	docReq.Header.Set("X-Tenant-ID", tenantID)
	docReq.Header.Set("X-Product-ID", h.productID)
	if auth := c.GetHeader("Authorization"); auth != "" {
		docReq.Header.Set("Authorization", auth)
	}

	// Make request to document service
	resp, err := h.httpClient.Do(docReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DOCUMENT_SERVICE_ERROR",
				Message: "Failed to communicate with document service",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RESPONSE_READ_FAILED",
				Message: "Failed to read response",
			},
		})
		return
	}

	// Forward response
	c.Header("Content-Type", "application/json")
	c.Data(resp.StatusCode, "application/json", respBody)
}

// GetTicketAttachments retrieves attachments for a ticket
// @Summary Get attachments for ticket
// @Description Get a list of file attachments associated with a ticket
// @Tags ticket-attachments
// @Produce json
// @Param id path string true "Ticket ID"
// @Param attachment_type query string false "Attachment type filter"
// @Param bucket query string false "Storage bucket name"
// @Param limit query int false "Limit results" default(50)
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /tickets/{id}/attachments [get]
func (h *DocumentHandler) GetTicketAttachments(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	ticketID := c.Param("id")

	// Validate ticket ID format
	if _, err := uuid.Parse(ticketID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_TICKET_ID",
				Message: "Invalid ticket ID format",
			},
		})
		return
	}

	bucket := c.DefaultQuery("bucket", "ticket-attachments")
	attachmentType := c.Query("attachment_type")
	limitInt, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limitInt < 1 || limitInt > 100 {
		limitInt = 50
	}
	limit := strconv.Itoa(limitInt)

	// Build query parameters
	tags := fmt.Sprintf("ticket_id:%s,tenant_id:%s", ticketID, tenantID)
	if attachmentType != "" {
		tags += fmt.Sprintf(",attachment_type:%s", attachmentType)
	}

	queryParams := fmt.Sprintf("bucket=%s&prefix=&limit=%s&tags=%s",
		bucket, limit, tags)

	// Forward request to document service
	docReq, err := http.NewRequest("GET",
		h.documentServiceURL+"/api/v1/documents?"+queryParams, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REQUEST_CREATE_FAILED",
				Message: "Failed to create request",
			},
		})
		return
	}

	// Set headers
	docReq.Header.Set("X-Tenant-ID", tenantID)
	docReq.Header.Set("X-Product-ID", h.productID)
	if auth := c.GetHeader("Authorization"); auth != "" {
		docReq.Header.Set("Authorization", auth)
	}

	// Make request to document service
	resp, err := h.httpClient.Do(docReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DOCUMENT_SERVICE_ERROR",
				Message: "Failed to communicate with document service",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Read and forward response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RESPONSE_READ_FAILED",
				Message: "Failed to read response",
			},
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(resp.StatusCode, "application/json", respBody)
}

// DeleteTicketAttachment deletes an attachment from a ticket
// @Summary Delete ticket attachment
// @Description Delete a file attachment associated with a ticket
// @Tags ticket-attachments
// @Produce json
// @Param id path string true "Ticket ID"
// @Param bucket path string true "Storage bucket name"
// @Param path path string true "Attachment path"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /tickets/{id}/attachments/{bucket}/*path [delete]
func (h *DocumentHandler) DeleteTicketAttachment(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	ticketID := c.Param("id")
	bucket := c.Param("bucket")
	attachmentPath := c.Param("path")

	// Validate ticket ID format
	if _, err := uuid.Parse(ticketID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_TICKET_ID",
				Message: "Invalid ticket ID format",
			},
		})
		return
	}

	// Forward request to document service
	docReq, err := http.NewRequest("DELETE",
		h.documentServiceURL+"/api/v1/documents/"+bucket+"/"+attachmentPath, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REQUEST_CREATE_FAILED",
				Message: "Failed to create request",
			},
		})
		return
	}

	// Set headers
	docReq.Header.Set("X-Tenant-ID", tenantID)
	docReq.Header.Set("X-Product-ID", h.productID)
	if auth := c.GetHeader("Authorization"); auth != "" {
		docReq.Header.Set("Authorization", auth)
	}

	// Make request to document service
	resp, err := h.httpClient.Do(docReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DOCUMENT_SERVICE_ERROR",
				Message: "Failed to communicate with document service",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Read and forward response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RESPONSE_READ_FAILED",
				Message: "Failed to read response",
			},
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(resp.StatusCode, "application/json", respBody)
}

// GenerateTicketAttachmentPresignedURL generates a presigned URL for ticket attachment access
// @Summary Generate presigned URL for ticket attachment
// @Description Generate a presigned URL for secure attachment access
// @Tags ticket-attachments
// @Accept json
// @Produce json
// @Param id path string true "Ticket ID"
// @Param request body gin.H true "Presigned URL request"
// @Success 200 {object} DocumentResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security BearerAuth
// @Router /tickets/{id}/attachments/presigned-url [post]
func (h *DocumentHandler) GenerateTicketAttachmentPresignedURL(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	ticketID := c.Param("id")

	// Validate ticket ID format
	if _, err := uuid.Parse(ticketID); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_TICKET_ID",
				Message: "Invalid ticket ID format",
			},
		})
		return
	}

	var reqBody map[string]interface{}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "INVALID_INPUT",
				Message: err.Error(),
			},
		})
		return
	}

	// Forward request to document service
	jsonBody, _ := json.Marshal(reqBody)
	docReq, err := http.NewRequest("POST",
		h.documentServiceURL+"/api/v1/documents/presigned-url",
		bytes.NewBuffer(jsonBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "REQUEST_CREATE_FAILED",
				Message: "Failed to create request",
			},
		})
		return
	}

	// Set headers
	docReq.Header.Set("Content-Type", "application/json")
	docReq.Header.Set("X-Tenant-ID", tenantID)
	docReq.Header.Set("X-Product-ID", h.productID)
	if auth := c.GetHeader("Authorization"); auth != "" {
		docReq.Header.Set("Authorization", auth)
	}

	// Make request to document service
	resp, err := h.httpClient.Do(docReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "DOCUMENT_SERVICE_ERROR",
				Message: "Failed to communicate with document service",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Read and forward response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Success: false,
			Error: models.Error{
				Code:    "RESPONSE_READ_FAILED",
				Message: "Failed to read response",
			},
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(resp.StatusCode, "application/json", respBody)
}
