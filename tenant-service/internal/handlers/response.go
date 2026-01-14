package handlers

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorResponse sends a standardized error response
// Internal errors are logged but not exposed to clients
func ErrorResponse(c *gin.Context, statusCode int, message string, err error) {
	requestID := getRequestID(c)

	// Log internal error details
	if err != nil {
		log.Printf("[ERROR] [%s] %s: %v", requestID, message, err)
	}

	// Send user-friendly response (don't expose internal errors)
	response := gin.H{
		"success":    false,
		"message":    message,
		"request_id": requestID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	// Only include error details in development mode
	if gin.Mode() == gin.DebugMode && err != nil {
		response["error_details"] = err.Error()
	}

	c.JSON(statusCode, response)
}

// SuccessResponse sends a standardized success response
func SuccessResponse(c *gin.Context, statusCode int, message string, data interface{}) {
	requestID := getRequestID(c)

	response := gin.H{
		"success":    true,
		"message":    message,
		"request_id": requestID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	if data != nil {
		response["data"] = data
	}

	c.JSON(statusCode, response)
}

// ValidationErrorResponse sends a validation error response
func ValidationErrorResponse(c *gin.Context, errors map[string]string) {
	requestID := getRequestID(c)

	response := gin.H{
		"success":    false,
		"message":    "Validation failed",
		"errors":     errors,
		"request_id": requestID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}

	c.JSON(400, response)
}

// getRequestID retrieves or generates a request ID
func getRequestID(c *gin.Context) string {
	// Check if request ID was set by middleware
	if requestID, exists := c.Get("request_id"); exists {
		return requestID.(string)
	}
	// Fallback to X-Request-ID header
	if requestID := c.GetHeader("X-Request-ID"); requestID != "" {
		return requestID
	}
	// Generate a simple ID (in production, use UUID)
	return time.Now().Format("20060102150405")
}
