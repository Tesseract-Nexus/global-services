package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"tenant-service/internal/services"
)

// DraftHandler handles draft-related HTTP requests
type DraftHandler struct {
	draftSvc *services.DraftService
}

// NewDraftHandler creates a new draft handler
func NewDraftHandler(draftSvc *services.DraftService) *DraftHandler {
	return &DraftHandler{draftSvc: draftSvc}
}

// SaveDraft handles POST /api/v1/onboarding/draft/save
func (h *DraftHandler) SaveDraft(c *gin.Context) {
	var req services.SaveDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate session ID
	if req.SessionID == uuid.Nil {
		ErrorResponse(c, http.StatusBadRequest, "session_id is required", nil)
		return
	}

	resp, err := h.draftSvc.SaveDraft(c.Request.Context(), &req)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to save draft", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Draft saved successfully", resp)
}

// GetDraft handles GET /api/v1/onboarding/draft/:sessionId
func (h *DraftHandler) GetDraft(c *gin.Context) {
	sessionIDStr := c.Param("sessionId")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	resp, err := h.draftSvc.GetDraft(c.Request.Context(), sessionID)
	if err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to get draft", err)
		return
	}

	if !resp.Found {
		ErrorResponse(c, http.StatusNotFound, "Draft not found or expired", nil)
		return
	}

	SuccessResponse(c, http.StatusOK, "Draft retrieved successfully", resp)
}

// ProcessHeartbeat handles POST /api/v1/onboarding/draft/heartbeat
func (h *DraftHandler) ProcessHeartbeat(c *gin.Context) {
	var req struct {
		SessionID uuid.UUID `json:"session_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := h.draftSvc.ProcessHeartbeat(c.Request.Context(), req.SessionID); err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to process heartbeat", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Heartbeat processed", nil)
}

// MarkBrowserClosed handles POST /api/v1/onboarding/draft/browser-close
func (h *DraftHandler) MarkBrowserClosed(c *gin.Context) {
	var req struct {
		SessionID uuid.UUID `json:"session_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := h.draftSvc.MarkBrowserClosed(c.Request.Context(), req.SessionID); err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to mark browser closed", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Browser close recorded", nil)
}

// DeleteDraft handles DELETE /api/v1/onboarding/draft/:sessionId
func (h *DraftHandler) DeleteDraft(c *gin.Context) {
	sessionIDStr := c.Param("sessionId")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		ErrorResponse(c, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	if err := h.draftSvc.DeleteDraft(c.Request.Context(), sessionID); err != nil {
		ErrorResponse(c, http.StatusInternalServerError, "Failed to delete draft", err)
		return
	}

	SuccessResponse(c, http.StatusOK, "Draft deleted", nil)
}
