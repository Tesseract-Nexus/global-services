package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"notification-service/internal/models"
	"notification-service/internal/repository"
)

// UnsubscribeHandler handles email unsubscribe requests
type UnsubscribeHandler struct {
	prefRepo   repository.PreferenceRepository
	signingKey string
}

// NewUnsubscribeHandler creates a new unsubscribe handler
func NewUnsubscribeHandler(prefRepo repository.PreferenceRepository) *UnsubscribeHandler {
	signingKey := os.Getenv("UNSUBSCRIBE_SIGNING_KEY")
	if signingKey == "" {
		signingKey = "tesseract-hub-unsubscribe-key" // Default for development
	}
	return &UnsubscribeHandler{
		prefRepo:   prefRepo,
		signingKey: signingKey,
	}
}

// UnsubscribeToken contains the data for generating/validating unsubscribe links
type UnsubscribeToken struct {
	TenantID  string    `json:"t"`
	UserID    string    `json:"u"`
	Email     string    `json:"e"`
	Category  string    `json:"c"` // marketing, orders, all
	Timestamp time.Time `json:"ts"`
}

// GenerateUnsubscribeURL generates a signed unsubscribe URL
func (h *UnsubscribeHandler) GenerateUnsubscribeURL(baseURL, tenantID string, userID uuid.UUID, email, category string) string {
	// Create token data
	data := fmt.Sprintf("%s|%s|%s|%s|%d", tenantID, userID.String(), email, category, time.Now().Unix())

	// Sign the data
	signature := h.signData(data)

	// Encode the data
	encodedData := base64.URLEncoding.EncodeToString([]byte(data))

	// Build URL
	return fmt.Sprintf("%s/unsubscribe?d=%s&s=%s", baseURL, url.QueryEscape(encodedData), url.QueryEscape(signature))
}

// GeneratePreferenceURL generates a URL to the preference center
func (h *UnsubscribeHandler) GeneratePreferenceURL(baseURL, tenantID string, userID uuid.UUID, email string) string {
	// Create token data
	data := fmt.Sprintf("%s|%s|%s|preferences|%d", tenantID, userID.String(), email, time.Now().Unix())

	// Sign the data
	signature := h.signData(data)

	// Encode the data
	encodedData := base64.URLEncoding.EncodeToString([]byte(data))

	// Build URL
	return fmt.Sprintf("%s/preferences?d=%s&s=%s", baseURL, url.QueryEscape(encodedData), url.QueryEscape(signature))
}

// signData creates an HMAC signature for the given data
func (h *UnsubscribeHandler) signData(data string) string {
	mac := hmac.New(sha256.New, []byte(h.signingKey))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifySignature verifies an HMAC signature
func (h *UnsubscribeHandler) verifySignature(data, signature string) bool {
	expectedSig := h.signData(data)
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// HandleUnsubscribe handles unsubscribe requests (GET for one-click, POST for confirmation)
func (h *UnsubscribeHandler) HandleUnsubscribe(c *gin.Context) {
	encodedData := c.Query("d")
	signature := c.Query("s")

	if encodedData == "" || signature == "" {
		h.renderUnsubscribePage(c, "", "", "", "Invalid unsubscribe link", true)
		return
	}

	// Decode data
	decodedData, err := base64.URLEncoding.DecodeString(encodedData)
	if err != nil {
		h.renderUnsubscribePage(c, "", "", "", "Invalid unsubscribe link", true)
		return
	}
	data := string(decodedData)

	// Verify signature
	if !h.verifySignature(data, signature) {
		h.renderUnsubscribePage(c, "", "", "", "Invalid or expired unsubscribe link", true)
		return
	}

	// Parse data: tenantID|userID|email|category|timestamp
	var tenantID, userIDStr, email, category string
	var timestamp int64
	_, err = fmt.Sscanf(data, "%s|%s|%s|%s|%d", &tenantID, &userIDStr, &email, &category, &timestamp)
	if err != nil {
		// Try splitting by |
		parts := splitString(data, '|')
		if len(parts) < 5 {
			h.renderUnsubscribePage(c, "", "", "", "Invalid unsubscribe link format", true)
			return
		}
		tenantID = parts[0]
		userIDStr = parts[1]
		email = parts[2]
		category = parts[3]
	}

	// Check if link is expired (30 days)
	if timestamp > 0 {
		linkTime := time.Unix(timestamp, 0)
		if time.Since(linkTime) > 30*24*time.Hour {
			h.renderUnsubscribePage(c, email, "", "", "This unsubscribe link has expired. Please use the link from a more recent email.", true)
			return
		}
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.renderUnsubscribePage(c, email, "", "", "Invalid user identifier", true)
		return
	}

	// For GET requests, show confirmation page
	if c.Request.Method == http.MethodGet {
		h.renderUnsubscribePage(c, email, category, data+"|"+signature, "", false)
		return
	}

	// For POST requests, process unsubscribe
	if err := h.processUnsubscribe(c, tenantID, userID, email, category); err != nil {
		h.renderUnsubscribePage(c, email, category, "", "Failed to process unsubscribe request. Please try again.", true)
		return
	}

	h.renderUnsubscribeSuccess(c, email, category)
}

// HandleOneClickUnsubscribe handles RFC 8058 one-click unsubscribe (POST only)
func (h *UnsubscribeHandler) HandleOneClickUnsubscribe(c *gin.Context) {
	// One-click unsubscribe must be POST
	if c.Request.Method != http.MethodPost {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "POST required for one-click unsubscribe"})
		return
	}

	encodedData := c.PostForm("d")
	signature := c.PostForm("s")

	if encodedData == "" {
		encodedData = c.Query("d")
		signature = c.Query("s")
	}

	if encodedData == "" || signature == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Decode and verify
	decodedData, err := base64.URLEncoding.DecodeString(encodedData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data"})
		return
	}
	data := string(decodedData)

	if !h.verifySignature(data, signature) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid signature"})
		return
	}

	// Parse data
	parts := splitString(data, '|')
	if len(parts) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data format"})
		return
	}

	tenantID := parts[0]
	userID, err := uuid.Parse(parts[1])
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	email := parts[2]
	category := parts[3]

	if err := h.processUnsubscribe(c, tenantID, userID, email, category); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unsubscribe"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Successfully unsubscribed"})
}

// processUnsubscribe processes the actual unsubscribe
func (h *UnsubscribeHandler) processUnsubscribe(c *gin.Context, tenantID string, userID uuid.UUID, email, category string) error {
	// Get existing preferences
	pref, err := h.prefRepo.GetByUserID(c.Request.Context(), tenantID, userID)
	if err != nil {
		// If no preferences exist, create new with unsubscribed state
		pref = &models.NotificationPreference{
			ID:       uuid.New(),
			TenantID: tenantID,
			UserID:   userID,
			Email:    email,
		}
	}

	// Update based on category
	switch category {
	case "marketing":
		pref.MarketingEnabled = false
	case "orders":
		pref.OrdersEnabled = false
	case "all":
		pref.EmailEnabled = false
		pref.MarketingEnabled = false
	default:
		pref.MarketingEnabled = false // Default to marketing
	}

	return h.prefRepo.Upsert(c.Request.Context(), pref)
}

// renderUnsubscribePage renders the unsubscribe confirmation page
func (h *UnsubscribeHandler) renderUnsubscribePage(c *gin.Context, email, category, token, errorMsg string, isError bool) {
	categoryDisplay := "marketing emails"
	switch category {
	case "orders":
		categoryDisplay = "order notifications"
	case "all":
		categoryDisplay = "all emails"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Unsubscribe - Tesseract Hub</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .card {
            background: white;
            border-radius: 16px;
            padding: 40px;
            max-width: 480px;
            width: 100%%;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            text-align: center;
        }
        .logo { font-size: 48px; margin-bottom: 20px; }
        h1 { color: #1e293b; margin-bottom: 16px; font-size: 24px; }
        p { color: #64748b; margin-bottom: 24px; line-height: 1.6; }
        .email {
            font-weight: 600;
            color: #1e293b;
            background: #f1f5f9;
            padding: 8px 16px;
            border-radius: 8px;
            display: inline-block;
            margin: 8px 0;
        }
        .error { color: #dc2626; background: #fef2f2; padding: 16px; border-radius: 8px; margin-bottom: 24px; }
        .btn {
            display: inline-block;
            padding: 14px 32px;
            border-radius: 8px;
            font-weight: 600;
            text-decoration: none;
            cursor: pointer;
            border: none;
            font-size: 16px;
            margin: 8px;
        }
        .btn-primary { background: #ef4444; color: white; }
        .btn-primary:hover { background: #dc2626; }
        .btn-secondary { background: #e2e8f0; color: #475569; }
        .btn-secondary:hover { background: #cbd5e1; }
        form { display: inline; }
        .footer { margin-top: 32px; font-size: 14px; color: #94a3b8; }
        .footer a { color: #6366f1; text-decoration: none; }
    </style>
</head>
<body>
    <div class="card">
        <div class="logo">📧</div>
        %s
    </div>
</body>
</html>`, func() string {
		if isError {
			return fmt.Sprintf(`
        <h1>Oops!</h1>
        <div class="error">%s</div>
        <a href="https://marketplace.tesserix.app" class="btn btn-secondary">Go to Homepage</a>
        <div class="footer">
            <p>Need help? <a href="mailto:support@tesserix.app">Contact Support</a></p>
        </div>`, errorMsg)
		}
		return fmt.Sprintf(`
        <h1>Unsubscribe</h1>
        <p>You are about to unsubscribe from <strong>%s</strong> for:</p>
        <div class="email">%s</div>
        <p>Are you sure you want to continue?</p>
        <form method="POST" action="/api/v1/unsubscribe">
            <input type="hidden" name="d" value="%s">
            <input type="hidden" name="s" value="%s">
            <button type="submit" class="btn btn-primary">Yes, Unsubscribe</button>
        </form>
        <a href="https://marketplace.tesserix.app" class="btn btn-secondary">Cancel</a>
        <div class="footer">
            <p>You can also <a href="/api/v1/preferences?d=%s&s=%s">manage your preferences</a></p>
        </div>`, categoryDisplay, email, splitToken(token, 0), splitToken(token, 1), splitToken(token, 0), splitToken(token, 1))
	}())

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

// renderUnsubscribeSuccess renders the success page
func (h *UnsubscribeHandler) renderUnsubscribeSuccess(c *gin.Context, email, category string) {
	categoryDisplay := "marketing emails"
	switch category {
	case "orders":
		categoryDisplay = "order notifications"
	case "all":
		categoryDisplay = "all emails"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Unsubscribed - Tesseract Hub</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        .card {
            background: white;
            border-radius: 16px;
            padding: 40px;
            max-width: 480px;
            width: 100%%;
            box-shadow: 0 20px 60px rgba(0,0,0,0.3);
            text-align: center;
        }
        .success-icon { font-size: 64px; margin-bottom: 20px; }
        h1 { color: #059669; margin-bottom: 16px; font-size: 24px; }
        p { color: #64748b; margin-bottom: 24px; line-height: 1.6; }
        .email {
            font-weight: 600;
            color: #1e293b;
            background: #f1f5f9;
            padding: 8px 16px;
            border-radius: 8px;
            display: inline-block;
            margin: 8px 0;
        }
        .btn {
            display: inline-block;
            padding: 14px 32px;
            border-radius: 8px;
            font-weight: 600;
            text-decoration: none;
            cursor: pointer;
            border: none;
            font-size: 16px;
            margin: 8px;
        }
        .btn-primary { background: #6366f1; color: white; }
        .btn-primary:hover { background: #4f46e5; }
        .resubscribe { margin-top: 32px; padding-top: 24px; border-top: 1px solid #e2e8f0; }
        .resubscribe p { font-size: 14px; margin-bottom: 12px; }
        .footer { margin-top: 24px; font-size: 14px; color: #94a3b8; }
    </style>
</head>
<body>
    <div class="card">
        <div class="success-icon">✅</div>
        <h1>You've Been Unsubscribed</h1>
        <p>You will no longer receive <strong>%s</strong> at:</p>
        <div class="email">%s</div>
        <p>We're sorry to see you go! You can always resubscribe from your account settings.</p>
        <a href="https://marketplace.tesserix.app" class="btn btn-primary">Continue Shopping</a>
        <div class="resubscribe">
            <p>Changed your mind?</p>
            <a href="https://marketplace.tesserix.app/account/notifications">Manage Preferences</a>
        </div>
        <div class="footer">
            <p>Tesseract Hub - Your Trusted Marketplace</p>
        </div>
    </div>
</body>
</html>`, categoryDisplay, email)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

// Helper functions
func splitString(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func splitToken(token string, index int) string {
	parts := splitString(token, '|')
	if index == 0 && len(parts) > 0 {
		// Return everything except the last part (signature)
		if len(parts) > 1 {
			result := parts[0]
			for i := 1; i < len(parts)-1; i++ {
				result += "|" + parts[i]
			}
			return base64.URLEncoding.EncodeToString([]byte(result))
		}
		return ""
	}
	if index == 1 && len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
