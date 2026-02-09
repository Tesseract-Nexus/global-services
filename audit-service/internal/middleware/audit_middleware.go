package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Tesseract-Nexus/go-shared/security"

	"audit-service/internal/models"
	"audit-service/internal/services"
)

// AuditMiddleware logs all API requests
func AuditMiddleware(auditService *services.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip health check endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/health/ready" || c.Request.URL.Path == "/health/live" {
			c.Next()
			return
		}

		// Capture start time
		startTime := time.Now()

		// Get request body for logging (if applicable)
		var requestBody []byte
		if c.Request.Body != nil && c.Request.Method != "GET" {
			requestBody, _ = io.ReadAll(c.Request.Body)
			// Restore the body for the actual handler
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Process request
		c.Next()

		// Create audit log
		log := &models.AuditLog{
			TenantID:  c.GetString("tenant_id"),
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Query:     c.Request.URL.RawQuery,
			IPAddress: c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			RequestID: c.GetString("request_id"),
			Timestamp: startTime,
		}

		// Get user info from context
		if userID, exists := c.Get("user_id"); exists {
			if uid, ok := userID.(uuid.UUID); ok {
				log.UserID = uid
			} else if uidStr, ok := userID.(string); ok {
				if uid, err := uuid.Parse(uidStr); err == nil {
					log.UserID = uid
				}
			}
		}
		if username, exists := c.Get("username"); exists {
			if uname, ok := username.(string); ok {
				log.Username = uname
			}
		}
		if email, exists := c.Get("user_email"); exists {
			if userEmail, ok := email.(string); ok {
				log.UserEmail = userEmail
			}
		}

		// Map HTTP status to audit status
		statusCode := c.Writer.Status()
		if statusCode >= 200 && statusCode < 300 {
			log.Status = models.StatusSuccess
		} else if statusCode >= 400 {
			log.Status = models.StatusFailure
		} else {
			log.Status = models.StatusPending
		}

		// Determine action and resource from method and path
		log.Action = mapMethodToAction(c.Request.Method, statusCode)
		log.Resource = extractResourceFromPath(c.Request.URL.Path)

		// Set severity based on method and status
		log.Severity = determineSeverity(c.Request.Method, statusCode, log.Resource)

		// Set service name
		log.ServiceName = c.GetString("service_name")
		if log.ServiceName == "" {
			log.ServiceName = "unknown"
		}

		// Capture error if any
		if len(c.Errors) > 0 {
			log.ErrorMessage = c.Errors.String()
		}

		// Build description
		log.Description = buildDescription(log)

		// Store request/response data in metadata (be careful with sensitive data)
		metadata := make(map[string]interface{})
		metadata["status_code"] = statusCode
		metadata["duration_ms"] = time.Since(startTime).Milliseconds()
		metadata["method"] = c.Request.Method
		metadata["path"] = c.Request.URL.Path

		// Log request body with appropriate masking strategy
		if len(requestBody) > 0 && len(requestBody) < 1024 {
			var bodyJSON map[string]interface{}
			if err := json.Unmarshal(requestBody, &bodyJSON); err == nil {
				if isPaymentEndpoint(c.Request.URL.Path) {
					// Whitelist approach for payment endpoints — only allow safe fields through
					bodyJSON = whitelistPaymentFields(bodyJSON)
				} else if !isSensitiveEndpoint(c.Request.URL.Path) {
					// Remove auth-sensitive fields entirely
					delete(bodyJSON, "password")
					delete(bodyJSON, "token")
					delete(bodyJSON, "secret")
					delete(bodyJSON, "card_number")
					delete(bodyJSON, "cvv")
					delete(bodyJSON, "pan")
					delete(bodyJSON, "gstin")
					// Mask PII fields using go-shared security utilities
					bodyJSON = security.SecureLogFields(bodyJSON)
				} else {
					// Sensitive endpoint — don't log body at all
					bodyJSON = nil
				}
				if bodyJSON != nil {
					metadata["request_body"] = bodyJSON
				}
			}
		}

		// Track PII field access for compliance (on read operations to PII-containing resources)
		if c.Request.Method == "GET" && isPIIResource(c.Request.URL.Path) && statusCode >= 200 && statusCode < 300 {
			piiFields := detectPIIFieldsInPath(c.Request.URL.Path)
			if len(piiFields) > 0 {
				piiJSON, _ := json.Marshal(piiFields)
				log.PIIFieldsAccessed = piiJSON
				log.Action = models.ActionPIIAccess
			}
		}

		// Enrich failed authorization logs with context
		if statusCode == 403 {
			log.Action = models.ActionAuthzDenied
			log.Severity = models.SeverityHigh
			authzCtx, _ := json.Marshal(map[string]interface{}{
				"path":       c.Request.URL.Path,
				"method":     c.Request.Method,
				"user_agent": c.Request.UserAgent(),
			})
			log.AuthzContext = authzCtx
		}

		if metadataJSON, err := json.Marshal(metadata); err == nil {
			log.Metadata = metadataJSON
		}

		// Log asynchronously to avoid blocking the response
		tenantID := log.TenantID
		go func() {
			_ = auditService.LogAction(c.Request.Context(), tenantID, log)
		}()
	}
}

// mapMethodToAction maps HTTP method to audit action
func mapMethodToAction(method string, statusCode int) models.AuditAction {
	if statusCode >= 400 {
		// Failed actions still get appropriate action type
		switch method {
		case "POST":
			return models.ActionCreate
		case "PUT", "PATCH":
			return models.ActionUpdate
		case "DELETE":
			return models.ActionDelete
		case "GET":
			return models.ActionRead
		}
	}

	switch method {
	case "POST":
		return models.ActionCreate
	case "PUT", "PATCH":
		return models.ActionUpdate
	case "DELETE":
		return models.ActionDelete
	case "GET":
		return models.ActionRead
	default:
		return models.ActionRead
	}
}

// extractResourceFromPath extracts the resource type from the URL path
func extractResourceFromPath(path string) models.AuditResource {
	// Simple extraction - can be made more sophisticated
	switch {
	case contains(path, "/users") || contains(path, "/user"):
		return models.ResourceUser
	case contains(path, "/roles") || contains(path, "/role"):
		return models.ResourceRole
	case contains(path, "/permissions") || contains(path, "/permission"):
		return models.ResourcePermission
	case contains(path, "/categories") || contains(path, "/category"):
		return models.ResourceCategory
	case contains(path, "/products") || contains(path, "/product"):
		return models.ResourceProduct
	case contains(path, "/orders") || contains(path, "/order"):
		return models.ResourceOrder
	case contains(path, "/customers") || contains(path, "/customer"):
		return models.ResourceCustomer
	case contains(path, "/vendors") || contains(path, "/vendor"):
		return models.ResourceVendor
	case contains(path, "/returns") || contains(path, "/return"):
		return models.ResourceReturn
	case contains(path, "/refunds") || contains(path, "/refund"):
		return models.ResourceRefund
	case contains(path, "/shipments") || contains(path, "/shipment"):
		return models.ResourceShipment
	case contains(path, "/payments") || contains(path, "/payment"):
		return models.ResourcePayment
	case contains(path, "/notifications") || contains(path, "/notification"):
		return models.ResourceNotification
	case contains(path, "/documents") || contains(path, "/document"):
		return models.ResourceDocument
	case contains(path, "/settings") || contains(path, "/setting"):
		return models.ResourceSettings
	case contains(path, "/config"):
		return models.ResourceConfig
	case contains(path, "/auth") || contains(path, "/login") || contains(path, "/logout"):
		return models.ResourceAuth
	default:
		return "UNKNOWN"
	}
}

// determineSeverity determines the severity based on action and status
func determineSeverity(method string, statusCode int, resource models.AuditResource) models.AuditSeverity {
	// Failed auth attempts are critical
	if resource == models.ResourceAuth && statusCode >= 400 {
		return models.SeverityCritical
	}

	// RBAC changes are high severity
	if resource == models.ResourceRole || resource == models.ResourcePermission {
		return models.SeverityHigh
	}

	// Deletes are high severity
	if method == "DELETE" {
		return models.SeverityHigh
	}

	// Failed requests are medium-high
	if statusCode >= 400 {
		return models.SeverityMedium
	}

	// Normal operations are low-medium
	return models.SeverityLow
}

// buildDescription builds a human-readable description
func buildDescription(log *models.AuditLog) string {
	action := string(log.Action)
	resource := string(log.Resource)
	username := log.Username
	if username == "" {
		username = "Anonymous"
	}

	if log.Status == models.StatusSuccess {
		return fmt.Sprintf("%s %s %s successfully", username, action, resource)
	}
	return fmt.Sprintf("%s attempted to %s %s (failed)", username, action, resource)
}

// isSensitiveEndpoint checks if the endpoint handles sensitive data
func isSensitiveEndpoint(path string) bool {
	sensitivePatterns := []string{
		"/login",
		"/password",
		"/auth",
		"/token",
		"/secret",
		"/key",
		"/payment",
		"/card",
		"/customer",
		"/order",
		"/address",
		"/checkout",
		"/refund",
		"/profile",
	}

	for _, pattern := range sensitivePatterns {
		if contains(path, pattern) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexAt(s, substr) >= 0)
}

func indexAt(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// isPaymentEndpoint checks if the endpoint is payment-related (uses whitelist masking)
func isPaymentEndpoint(path string) bool {
	paymentPatterns := []string{"/payment", "/card", "/checkout", "/refund"}
	for _, pattern := range paymentPatterns {
		if contains(path, pattern) {
			return true
		}
	}
	return false
}

// whitelistPaymentFields only keeps explicitly safe fields from payment request bodies.
// Everything not on this whitelist is dropped — the opposite of a blacklist approach.
func whitelistPaymentFields(body map[string]interface{}) map[string]interface{} {
	allowedFields := map[string]bool{
		"amount":        true,
		"currency":      true,
		"method":        true,
		"status":        true,
		"order_id":      true,
		"payment_id":    true,
		"provider":      true,
		"is_test_mode":  true,
		"gateway_type":  true,
		"display_order": true,
		"is_enabled":    true,
		"reason":        true,
	}

	safe := make(map[string]interface{})
	for key, val := range body {
		if allowedFields[key] {
			safe[key] = val
		}
	}
	return safe
}

// isPIIResource checks if the endpoint accesses PII-containing resources
func isPIIResource(path string) bool {
	piiResources := []string{"/customer", "/order", "/user", "/staff", "/profile", "/address"}
	for _, resource := range piiResources {
		if contains(path, resource) {
			return true
		}
	}
	return false
}

// detectPIIFieldsInPath returns which PII fields are likely accessed based on the resource path
func detectPIIFieldsInPath(path string) []string {
	var fields []string
	switch {
	case contains(path, "/customer"):
		fields = []string{"email", "phone", "first_name", "last_name", "address"}
	case contains(path, "/order"):
		fields = []string{"customer_email", "customer_name", "shipping_address", "billing_address"}
	case contains(path, "/user") || contains(path, "/staff"):
		fields = []string{"email", "phone", "first_name", "last_name"}
	case contains(path, "/address"):
		fields = []string{"street", "city", "state", "postal_code", "phone"}
	case contains(path, "/profile"):
		fields = []string{"email", "phone", "first_name", "last_name", "address"}
	}
	return fields
}
