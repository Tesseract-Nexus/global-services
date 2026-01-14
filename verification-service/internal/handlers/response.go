package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/tesseract-hub/domains/common/services/verification-service/internal/models"
)

// SuccessResponse sends a success response
func SuccessResponse(c *gin.Context, status int, message string, data interface{}) {
	c.JSON(status, models.APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// ErrorResponse sends an error response
func ErrorResponse(c *gin.Context, status int, message string, err error) {
	apiError := &models.APIError{
		Code:    "ERROR",
		Message: message,
	}
	if err != nil {
		apiError.Details = err.Error()
	}

	c.JSON(status, models.APIResponse{
		Success: false,
		Message: message,
		Error:   apiError,
	})
}
