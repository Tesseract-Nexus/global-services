package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// MauticClient handles Mautic API operations
type MauticClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewMauticClient creates a new Mautic API client
func NewMauticClient(baseURL, username, password string) *MauticClient {
	return &MauticClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *MauticClient) setAuthHeader(req *http.Request) {
	auth := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.password))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
}

func (c *MauticClient) doRequest(method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Segment represents a Mautic segment
type Segment struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name"`
	Alias       string `json:"alias"`
	Description string `json:"description"`
	IsPublished bool   `json:"isPublished"`
	IsGlobal    bool   `json:"isGlobal"`
}

// Email represents a Mautic email template
type Email struct {
	ID          int    `json:"id,omitempty"`
	Name        string `json:"name"`
	Subject     string `json:"subject"`
	CustomHTML  string `json:"customHtml"`
	PlainText   string `json:"plainText,omitempty"`
	EmailType   string `json:"emailType"` // "list" or "template"
	IsPublished bool   `json:"isPublished"`
	FromAddress string `json:"fromAddress,omitempty"`
	FromName    string `json:"fromName,omitempty"`
	ReplyTo     string `json:"replyToAddress,omitempty"`
	Lists       []int  `json:"lists,omitempty"` // Segment IDs for list emails
}

// Campaign represents a Mautic campaign
type Campaign struct {
	ID             int             `json:"id,omitempty"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	IsPublished    bool            `json:"isPublished"`
	AllowRestart   int             `json:"allowRestart"`
	Events         []CampaignEvent `json:"events,omitempty"`
	CanvasSettings map[string]any  `json:"canvasSettings,omitempty"`
}

// CampaignEvent represents a campaign event/action
type CampaignEvent struct {
	ID                  int            `json:"id,omitempty"`
	Name                string         `json:"name"`
	Description         string         `json:"description,omitempty"`
	Type                string         `json:"type"`
	EventType           string         `json:"eventType"` // "action", "decision", "condition"
	Order               int            `json:"order"`
	Properties          map[string]any `json:"properties"`
	TriggerMode         string         `json:"triggerMode,omitempty"`         // "immediate", "interval", "date"
	TriggerInterval     int            `json:"triggerInterval,omitempty"`     // nolint:unused
	TriggerIntervalUnit string         `json:"triggerIntervalUnit,omitempty"` // "i" (minutes), "h", "d", "m", "y"
	DecisionPath        string         `json:"decisionPath,omitempty"`        // "yes", "no"
	Parent              *int           `json:"parent,omitempty"`
	TempID              string         `json:"tempId,omitempty"`
}

// Contact represents a Mautic contact
type Contact struct {
	ID        int    `json:"id,omitempty"`
	Email     string `json:"email"`
	FirstName string `json:"firstname,omitempty"`
	LastName  string `json:"lastname,omitempty"`
	Tags      string `json:"tags,omitempty"`
}

// CreateSegment creates a new segment in Mautic
func (c *MauticClient) CreateSegment(segment *Segment) (int, error) {
	resp, err := c.doRequest("POST", "/api/segments/new", segment)
	if err != nil {
		// Check if segment already exists
		if strings.Contains(err.Error(), "400") {
			log.Printf("Segment '%s' may already exist, skipping...", segment.Name)
			return 0, nil
		}
		return 0, err
	}

	var result struct {
		List struct {
			ID int `json:"id"`
		} `json:"list"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, err
	}

	return result.List.ID, nil
}

// CreateEmail creates a new email in Mautic
func (c *MauticClient) CreateEmail(email *Email) (int, error) {
	resp, err := c.doRequest("POST", "/api/emails/new", email)
	if err != nil {
		if strings.Contains(err.Error(), "400") {
			log.Printf("Email '%s' may already exist, skipping...", email.Name)
			return 0, nil
		}
		return 0, err
	}

	var result struct {
		Email struct {
			ID int `json:"id"`
		} `json:"email"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, err
	}

	return result.Email.ID, nil
}

// CreateCampaign creates a new campaign in Mautic
func (c *MauticClient) CreateCampaign(campaign *Campaign) (int, error) {
	resp, err := c.doRequest("POST", "/api/campaigns/new", campaign)
	if err != nil {
		if strings.Contains(err.Error(), "400") {
			log.Printf("Campaign '%s' may already exist, skipping...", campaign.Name)
			return 0, nil
		}
		return 0, err
	}

	var result struct {
		Campaign struct {
			ID int `json:"id"`
		} `json:"campaign"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, err
	}

	return result.Campaign.ID, nil
}

// CreateContact creates a new contact in Mautic
func (c *MauticClient) CreateContact(contact *Contact) (int, error) {
	resp, err := c.doRequest("POST", "/api/contacts/new", contact)
	if err != nil {
		return 0, err
	}

	var result struct {
		Contact struct {
			ID int `json:"id"`
		} `json:"contact"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, err
	}

	return result.Contact.ID, nil
}

// AddContactToSegment adds a contact to a segment
func (c *MauticClient) AddContactToSegment(contactID, segmentID int) error {
	_, err := c.doRequest("POST", fmt.Sprintf("/api/segments/%d/contact/%d/add", segmentID, contactID), nil)
	return err
}

// SendEmailToContact sends an email to a specific contact
func (c *MauticClient) SendEmailToContact(emailID, contactID int) error {
	_, err := c.doRequest("POST", fmt.Sprintf("/api/emails/%d/contact/%d/send", emailID, contactID), nil)
	return err
}

// ============================================================================
// BENCHMARK CAMPAIGNS AND TEMPLATES
// ============================================================================

// getBenchmarkSegments returns the benchmark segments to create
func getBenchmarkSegments() []Segment {
	return []Segment{
		{
			Name:        "All Customers",
			Alias:       "all-customers",
			Description: "All registered customers in the marketplace",
			IsPublished: true,
			IsGlobal:    true,
		},
		{
			Name:        "New Customers (Last 30 Days)",
			Alias:       "new-customers-30d",
			Description: "Customers who registered in the last 30 days",
			IsPublished: true,
			IsGlobal:    true,
		},
		{
			Name:        "VIP Customers",
			Alias:       "vip-customers",
			Description: "High-value customers with VIP status",
			IsPublished: true,
			IsGlobal:    true,
		},
		{
			Name:        "Wholesale Customers",
			Alias:       "wholesale-customers",
			Description: "B2B wholesale customers",
			IsPublished: true,
			IsGlobal:    true,
		},
		{
			Name:        "Inactive Customers (90+ Days)",
			Alias:       "inactive-customers-90d",
			Description: "Customers who haven't ordered in 90+ days",
			IsPublished: true,
			IsGlobal:    true,
		},
		{
			Name:        "Abandoned Cart",
			Alias:       "abandoned-cart",
			Description: "Customers with items in cart but no checkout",
			IsPublished: true,
			IsGlobal:    true,
		},
		{
			Name:        "Newsletter Subscribers",
			Alias:       "newsletter-subscribers",
			Description: "Customers opted-in for marketing emails",
			IsPublished: true,
			IsGlobal:    true,
		},
		{
			Name:        "High Value (LTV > $1000)",
			Alias:       "high-value-ltv-1000",
			Description: "Customers with lifetime value over $1000",
			IsPublished: true,
			IsGlobal:    true,
		},
	}
}

// getBenchmarkEmails returns the benchmark email templates
func getBenchmarkEmails(fromEmail, fromName string) []Email {
	return []Email{
		// ===== WELCOME SERIES =====
		{
			Name:        "[Benchmark] Welcome Email",
			Subject:     "Welcome to Tesseract Hub! Let's Get Started",
			EmailType:   "template",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Welcome to Tesseract Hub</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; }
    .header { background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%); padding: 40px 20px; text-align: center; }
    .header h1 { color: #fff; margin: 0; font-size: 28px; }
    .content { padding: 40px 30px; }
    .cta-button { display: inline-block; background: #6366f1; color: #fff !important; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; margin: 20px 0; }
    .features { margin: 30px 0; }
    .feature { display: flex; margin: 15px 0; padding: 15px; background: #f8fafc; border-radius: 8px; }
    .feature-icon { font-size: 24px; margin-right: 15px; }
    .footer { background: #1e293b; color: #94a3b8; padding: 30px; text-align: center; font-size: 14px; }
    .footer a { color: #94a3b8; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Welcome to Tesseract Hub!</h1>
    </div>
    <div class="content">
      <p>Hi {contactfield=firstname|there},</p>
      <p>Thank you for joining Tesseract Hub! We're thrilled to have you as part of our community.</p>

      <div class="features">
        <div class="feature">
          <span class="feature-icon">🛒</span>
          <div>
            <strong>Discover Products</strong>
            <p style="margin: 5px 0 0 0; color: #64748b;">Browse thousands of products from verified sellers</p>
          </div>
        </div>
        <div class="feature">
          <span class="feature-icon">🚚</span>
          <div>
            <strong>Fast Delivery</strong>
            <p style="margin: 5px 0 0 0; color: #64748b;">Track your orders in real-time</p>
          </div>
        </div>
        <div class="feature">
          <span class="feature-icon">💳</span>
          <div>
            <strong>Secure Payments</strong>
            <p style="margin: 5px 0 0 0; color: #64748b;">Multiple payment options with full encryption</p>
          </div>
        </div>
      </div>

      <center>
        <a href="https://marketplace.tesserix.app" class="cta-button">Start Shopping</a>
      </center>

      <p>As a welcome gift, use code <strong>WELCOME10</strong> for 10% off your first order!</p>

      <p>Happy shopping!<br>The Tesseract Hub Team</p>
    </div>
    <div class="footer">
      <p>Tesseract Hub - Your Trusted Marketplace</p>
      <p><a href="{unsubscribe_url}">Unsubscribe</a> | <a href="{webview_url}">View in browser</a></p>
    </div>
  </div>
</body>
</html>`,
			PlainText: `Welcome to Tesseract Hub!

Hi {contactfield=firstname|there},

Thank you for joining Tesseract Hub! We're thrilled to have you as part of our community.

What you can do:
- Discover thousands of products from verified sellers
- Track your orders in real-time
- Enjoy secure payments with multiple options

Start shopping: https://marketplace.tesserix.app

As a welcome gift, use code WELCOME10 for 10% off your first order!

Happy shopping!
The Tesseract Hub Team

Unsubscribe: {unsubscribe_url}`,
		},

		// ===== ABANDONED CART =====
		{
			Name:        "[Benchmark] Abandoned Cart - Reminder",
			Subject:     "You left something behind! Your cart is waiting",
			EmailType:   "template",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; }
    .header { background: #f59e0b; padding: 30px 20px; text-align: center; }
    .header h1 { color: #fff; margin: 0; font-size: 24px; }
    .content { padding: 40px 30px; }
    .cart-icon { font-size: 60px; text-align: center; margin: 20px 0; }
    .cta-button { display: inline-block; background: #f59e0b; color: #fff !important; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; margin: 20px 0; }
    .urgency { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 15px; margin: 20px 0; }
    .footer { background: #1e293b; color: #94a3b8; padding: 30px; text-align: center; font-size: 14px; }
    .footer a { color: #94a3b8; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Don't Forget Your Items!</h1>
    </div>
    <div class="content">
      <div class="cart-icon">🛒</div>

      <p>Hi {contactfield=firstname|there},</p>
      <p>We noticed you left some items in your shopping cart. They're waiting for you!</p>

      <div class="urgency">
        <strong>⏰ Items in your cart may sell out!</strong>
        <p style="margin: 5px 0 0 0;">Complete your purchase before someone else grabs them.</p>
      </div>

      <center>
        <a href="https://marketplace.tesserix.app/cart" class="cta-button">Complete My Order</a>
      </center>

      <p>Need help? Our support team is here for you 24/7.</p>

      <p>Best regards,<br>The Tesseract Hub Team</p>
    </div>
    <div class="footer">
      <p>Tesseract Hub - Your Trusted Marketplace</p>
      <p><a href="{unsubscribe_url}">Unsubscribe</a></p>
    </div>
  </div>
</body>
</html>`,
		},

		{
			Name:        "[Benchmark] Abandoned Cart - Incentive",
			Subject:     "Here's 5% off to complete your order!",
			EmailType:   "template",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; }
    .header { background: linear-gradient(135deg, #10b981 0%, #059669 100%); padding: 30px 20px; text-align: center; }
    .header h1 { color: #fff; margin: 0; font-size: 24px; }
    .content { padding: 40px 30px; }
    .coupon-box { background: #ecfdf5; border: 2px dashed #10b981; padding: 20px; text-align: center; margin: 20px 0; border-radius: 8px; }
    .coupon-code { font-size: 28px; font-weight: bold; color: #059669; letter-spacing: 2px; }
    .cta-button { display: inline-block; background: #10b981; color: #fff !important; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; margin: 20px 0; }
    .footer { background: #1e293b; color: #94a3b8; padding: 30px; text-align: center; font-size: 14px; }
    .footer a { color: #94a3b8; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Special Offer Just For You!</h1>
    </div>
    <div class="content">
      <p>Hi {contactfield=firstname|there},</p>
      <p>We really want you to complete your order, so here's an exclusive discount:</p>

      <div class="coupon-box">
        <p style="margin: 0 0 10px 0;">Use code:</p>
        <div class="coupon-code">CART5OFF</div>
        <p style="margin: 10px 0 0 0; color: #64748b;">For 5% off your order</p>
      </div>

      <center>
        <a href="https://marketplace.tesserix.app/cart" class="cta-button">Use My Discount</a>
      </center>

      <p style="color: #94a3b8; font-size: 14px;">*This offer expires in 24 hours</p>

      <p>Happy shopping!<br>The Tesseract Hub Team</p>
    </div>
    <div class="footer">
      <p><a href="{unsubscribe_url}">Unsubscribe</a></p>
    </div>
  </div>
</body>
</html>`,
		},

		// ===== ORDER CONFIRMATION =====
		{
			Name:        "[Benchmark] Order Confirmation",
			Subject:     "Order Confirmed! Thank you for your purchase",
			EmailType:   "template",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; }
    .header { background: #10b981; padding: 30px 20px; text-align: center; }
    .header h1 { color: #fff; margin: 0; font-size: 24px; }
    .check-icon { font-size: 48px; }
    .content { padding: 40px 30px; }
    .order-box { background: #f8fafc; border-radius: 8px; padding: 20px; margin: 20px 0; }
    .order-number { font-size: 18px; font-weight: bold; color: #1e293b; }
    .cta-button { display: inline-block; background: #6366f1; color: #fff !important; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; margin: 20px 0; }
    .footer { background: #1e293b; color: #94a3b8; padding: 30px; text-align: center; font-size: 14px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <div class="check-icon">✓</div>
      <h1>Order Confirmed!</h1>
    </div>
    <div class="content">
      <p>Hi {contactfield=firstname|there},</p>
      <p>Great news! Your order has been confirmed and is being processed.</p>

      <div class="order-box">
        <p class="order-number">Order #TH-{contactfield=order_number|000000}</p>
        <p style="margin: 5px 0; color: #64748b;">Placed on: {contactfield=order_date|Today}</p>
        <hr style="border: none; border-top: 1px solid #e2e8f0; margin: 15px 0;">
        <p style="margin: 0;"><strong>What's Next?</strong></p>
        <p style="margin: 5px 0 0 0; color: #64748b;">We'll send you a shipping confirmation with tracking details once your order is on its way.</p>
      </div>

      <center>
        <a href="https://marketplace.tesserix.app/orders" class="cta-button">Track My Order</a>
      </center>

      <p>Thank you for shopping with us!</p>
      <p>The Tesseract Hub Team</p>
    </div>
    <div class="footer">
      <p>Need help? Contact us at support@tesserix.app</p>
    </div>
  </div>
</body>
</html>`,
		},

		// ===== REVIEW REQUEST =====
		{
			Name:        "[Benchmark] Review Request",
			Subject:     "How was your order? We'd love your feedback!",
			EmailType:   "template",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; }
    .header { background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%); padding: 30px 20px; text-align: center; }
    .header h1 { color: #fff; margin: 0; font-size: 24px; }
    .content { padding: 40px 30px; text-align: center; }
    .stars { font-size: 40px; margin: 20px 0; }
    .cta-button { display: inline-block; background: #8b5cf6; color: #fff !important; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; margin: 20px 0; }
    .incentive { background: #f5f3ff; padding: 15px; border-radius: 8px; margin: 20px 0; }
    .footer { background: #1e293b; color: #94a3b8; padding: 30px; text-align: center; font-size: 14px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>How Did We Do?</h1>
    </div>
    <div class="content">
      <div class="stars">⭐⭐⭐⭐⭐</div>

      <p>Hi {contactfield=firstname|there},</p>
      <p>Your order has been delivered! We hope you love your purchase.</p>
      <p>Would you take a moment to share your experience?</p>

      <a href="https://marketplace.tesserix.app/review" class="cta-button">Write a Review</a>

      <div class="incentive">
        <strong>🎁 Get 50 Reward Points!</strong>
        <p style="margin: 5px 0 0 0; color: #64748b;">Leave a review and earn points towards your next purchase.</p>
      </div>

      <p>Your feedback helps other shoppers and helps us improve.</p>
      <p>Thank you!<br>The Tesseract Hub Team</p>
    </div>
    <div class="footer">
      <p><a href="{unsubscribe_url}" style="color: #94a3b8;">Unsubscribe</a></p>
    </div>
  </div>
</body>
</html>`,
		},

		// ===== WIN-BACK CAMPAIGN =====
		{
			Name:        "[Benchmark] Win-Back - We Miss You",
			Subject:     "We miss you! Come back for something special",
			EmailType:   "template",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; }
    .header { background: linear-gradient(135deg, #ec4899 0%, #f43f5e 100%); padding: 40px 20px; text-align: center; }
    .header h1 { color: #fff; margin: 0; font-size: 28px; }
    .heart { font-size: 48px; }
    .content { padding: 40px 30px; }
    .coupon-box { background: #fdf2f8; border: 2px dashed #ec4899; padding: 25px; text-align: center; margin: 25px 0; border-radius: 12px; }
    .coupon-code { font-size: 32px; font-weight: bold; color: #be185d; letter-spacing: 3px; }
    .cta-button { display: inline-block; background: #ec4899; color: #fff !important; text-decoration: none; padding: 16px 36px; border-radius: 8px; font-weight: 600; margin: 20px 0; font-size: 16px; }
    .footer { background: #1e293b; color: #94a3b8; padding: 30px; text-align: center; font-size: 14px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <div class="heart">💝</div>
      <h1>We Miss You!</h1>
    </div>
    <div class="content">
      <p>Hi {contactfield=firstname|there},</p>
      <p>It's been a while since we've seen you, and we've missed having you around!</p>
      <p>A lot has changed since your last visit - new products, new features, and an exclusive offer just for you:</p>

      <div class="coupon-box">
        <p style="margin: 0 0 10px 0; font-size: 18px;">Welcome back with:</p>
        <div class="coupon-code">COMEBACK15</div>
        <p style="margin: 15px 0 0 0; color: #64748b; font-size: 16px;"><strong>15% OFF</strong> your next order</p>
      </div>

      <center>
        <a href="https://marketplace.tesserix.app" class="cta-button">Shop Now</a>
      </center>

      <p style="color: #94a3b8; font-size: 14px; text-align: center;">*Offer valid for 7 days. One-time use only.</p>

      <p>We can't wait to see you again!</p>
      <p>With love,<br>The Tesseract Hub Team</p>
    </div>
    <div class="footer">
      <p><a href="{unsubscribe_url}" style="color: #94a3b8;">Unsubscribe</a></p>
    </div>
  </div>
</body>
</html>`,
		},

		// ===== VIP EXCLUSIVE =====
		{
			Name:        "[Benchmark] VIP Exclusive Offer",
			Subject:     "🌟 VIP Exclusive: Early Access & Special Perks",
			EmailType:   "template",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #0f172a; }
    .container { max-width: 600px; margin: 0 auto; background: linear-gradient(180deg, #1e293b 0%, #0f172a 100%); }
    .header { padding: 50px 20px; text-align: center; }
    .vip-badge { display: inline-block; background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%); color: #1e293b; padding: 8px 20px; border-radius: 20px; font-weight: bold; font-size: 14px; letter-spacing: 2px; }
    .header h1 { color: #fff; margin: 20px 0 0 0; font-size: 32px; }
    .content { padding: 30px; color: #e2e8f0; }
    .perks { margin: 30px 0; }
    .perk { display: flex; align-items: center; padding: 15px; background: rgba(255,255,255,0.05); border-radius: 8px; margin: 10px 0; }
    .perk-icon { font-size: 24px; margin-right: 15px; }
    .cta-button { display: inline-block; background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%); color: #1e293b !important; text-decoration: none; padding: 16px 40px; border-radius: 8px; font-weight: bold; margin: 20px 0; font-size: 16px; }
    .footer { padding: 30px; text-align: center; color: #64748b; font-size: 14px; }
    .footer a { color: #64748b; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <span class="vip-badge">⭐ VIP MEMBER</span>
      <h1>Exclusive Access Awaits</h1>
    </div>
    <div class="content">
      <p>Dear {contactfield=firstname|Valued Customer},</p>
      <p>As one of our most valued VIP members, you deserve the very best. Here's what we have in store for you:</p>

      <div class="perks">
        <div class="perk">
          <span class="perk-icon">🎯</span>
          <div>
            <strong style="color: #fbbf24;">Early Access</strong>
            <p style="margin: 5px 0 0 0; color: #94a3b8;">Shop new arrivals 48 hours before everyone else</p>
          </div>
        </div>
        <div class="perk">
          <span class="perk-icon">🎁</span>
          <div>
            <strong style="color: #fbbf24;">Free Express Shipping</strong>
            <p style="margin: 5px 0 0 0; color: #94a3b8;">On all orders, no minimum required</p>
          </div>
        </div>
        <div class="perk">
          <span class="perk-icon">💎</span>
          <div>
            <strong style="color: #fbbf24;">Double Points Week</strong>
            <p style="margin: 5px 0 0 0; color: #94a3b8;">Earn 2x rewards on every purchase this week</p>
          </div>
        </div>
      </div>

      <center>
        <a href="https://marketplace.tesserix.app/vip" class="cta-button">Explore VIP Benefits</a>
      </center>

      <p style="text-align: center; color: #94a3b8;">Thank you for being an amazing customer.</p>
    </div>
    <div class="footer">
      <p>You're receiving this because you're a VIP member.</p>
      <p><a href="{unsubscribe_url}">Manage preferences</a></p>
    </div>
  </div>
</body>
</html>`,
		},

		// ===== NEWSLETTER =====
		{
			Name:        "[Benchmark] Monthly Newsletter",
			Subject:     "This Month at Tesseract Hub: News, Deals & More!",
			EmailType:   "list",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; }
    .header { background: #1e293b; padding: 30px 20px; text-align: center; }
    .header h1 { color: #fff; margin: 0; font-size: 24px; }
    .content { padding: 30px; }
    .section { margin: 25px 0; padding-bottom: 25px; border-bottom: 1px solid #e2e8f0; }
    .section:last-child { border-bottom: none; }
    .section-title { color: #6366f1; font-size: 18px; font-weight: bold; margin-bottom: 15px; }
    .product-grid { display: flex; gap: 15px; }
    .product { flex: 1; text-align: center; }
    .product img { width: 100%; border-radius: 8px; }
    .cta-button { display: inline-block; background: #6366f1; color: #fff !important; text-decoration: none; padding: 12px 24px; border-radius: 6px; font-weight: 600; }
    .footer { background: #1e293b; color: #94a3b8; padding: 30px; text-align: center; font-size: 14px; }
    .footer a { color: #94a3b8; }
    .social-links { margin: 15px 0; }
    .social-links a { margin: 0 10px; font-size: 20px; text-decoration: none; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>📰 Tesseract Hub Newsletter</h1>
    </div>
    <div class="content">
      <p>Hi {contactfield=firstname|there},</p>
      <p>Here's what's happening at Tesseract Hub this month!</p>

      <div class="section">
        <div class="section-title">🔥 Featured Products</div>
        <p>Check out our top picks this month - handpicked just for you.</p>
        <center><a href="https://marketplace.tesserix.app/featured" class="cta-button">View All</a></center>
      </div>

      <div class="section">
        <div class="section-title">🎉 Upcoming Sale</div>
        <p>Mark your calendar! Our biggest sale of the season is coming up. Stay tuned for exclusive early access.</p>
      </div>

      <div class="section">
        <div class="section-title">📱 New Features</div>
        <p>We've been busy improving your shopping experience:</p>
        <ul>
          <li>Enhanced search with AI recommendations</li>
          <li>Faster checkout with saved addresses</li>
          <li>Real-time order tracking updates</li>
        </ul>
      </div>

      <div class="section">
        <div class="section-title">💡 Pro Tip</div>
        <p>Did you know you can save items to your wishlist and get notified when prices drop? Try it today!</p>
      </div>
    </div>
    <div class="footer">
      <div class="social-links">
        <a href="#">📘</a>
        <a href="#">🐦</a>
        <a href="#">📸</a>
      </div>
      <p>Tesseract Hub - Your Trusted Marketplace</p>
      <p><a href="{unsubscribe_url}">Unsubscribe</a> | <a href="{preference_url}">Update Preferences</a></p>
    </div>
  </div>
</body>
</html>`,
		},

		// ===== PAYMENT FAILED =====
		{
			Name:        "[Benchmark] Payment Failed",
			Subject:     "⚠️ Payment Issue - Action Required",
			EmailType:   "template",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; }
    .header { background: #ef4444; padding: 30px 20px; text-align: center; }
    .header h1 { color: #fff; margin: 0; font-size: 24px; }
    .alert-icon { font-size: 48px; }
    .content { padding: 40px 30px; }
    .alert-box { background: #fef2f2; border-left: 4px solid #ef4444; padding: 20px; margin: 20px 0; }
    .cta-button { display: inline-block; background: #ef4444; color: #fff !important; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; margin: 20px 0; }
    .help-section { background: #f8fafc; padding: 20px; border-radius: 8px; margin: 20px 0; }
    .footer { background: #1e293b; color: #94a3b8; padding: 30px; text-align: center; font-size: 14px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <div class="alert-icon">⚠️</div>
      <h1>Payment Unsuccessful</h1>
    </div>
    <div class="content">
      <p>Hi {contactfield=firstname|there},</p>

      <div class="alert-box">
        <strong>Your recent payment could not be processed.</strong>
        <p style="margin: 10px 0 0 0;">Don't worry - your order is still saved and your items are reserved for the next 24 hours.</p>
      </div>

      <p>This might have happened because:</p>
      <ul>
        <li>Insufficient funds in your account</li>
        <li>Card details were entered incorrectly</li>
        <li>Your bank declined the transaction</li>
        <li>Card has expired</li>
      </ul>

      <center>
        <a href="https://marketplace.tesserix.app/checkout/retry" class="cta-button">Try Again</a>
      </center>

      <div class="help-section">
        <strong>Need Help?</strong>
        <p style="margin: 10px 0 0 0;">Contact our support team at <a href="mailto:support@tesserix.app">support@tesserix.app</a> or call us at 1-800-TESSERACT.</p>
      </div>

      <p>We're here to help!</p>
      <p>The Tesseract Hub Team</p>
    </div>
    <div class="footer">
      <p>This is an automated message regarding your order.</p>
    </div>
  </div>
</body>
</html>`,
		},

		// ===== SHIPPING NOTIFICATION =====
		{
			Name:        "[Benchmark] Order Shipped",
			Subject:     "🚚 Your order is on its way!",
			EmailType:   "template",
			IsPublished: true,
			FromAddress: fromEmail,
			FromName:    fromName,
			CustomHTML: `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f5f5f5; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; }
    .header { background: #3b82f6; padding: 30px 20px; text-align: center; }
    .header h1 { color: #fff; margin: 0; font-size: 24px; }
    .truck-icon { font-size: 48px; }
    .content { padding: 40px 30px; }
    .tracking-box { background: #eff6ff; border-radius: 12px; padding: 25px; margin: 20px 0; text-align: center; }
    .tracking-number { font-size: 20px; font-weight: bold; color: #1e40af; letter-spacing: 1px; margin: 10px 0; }
    .timeline { margin: 30px 0; }
    .timeline-item { display: flex; align-items: flex-start; margin: 15px 0; }
    .timeline-dot { width: 12px; height: 12px; background: #3b82f6; border-radius: 50%; margin-right: 15px; margin-top: 5px; }
    .timeline-dot.inactive { background: #cbd5e1; }
    .cta-button { display: inline-block; background: #3b82f6; color: #fff !important; text-decoration: none; padding: 14px 32px; border-radius: 8px; font-weight: 600; margin: 20px 0; }
    .footer { background: #1e293b; color: #94a3b8; padding: 30px; text-align: center; font-size: 14px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <div class="truck-icon">🚚</div>
      <h1>Your Order Has Shipped!</h1>
    </div>
    <div class="content">
      <p>Hi {contactfield=firstname|there},</p>
      <p>Great news! Your order is now on its way to you.</p>

      <div class="tracking-box">
        <p style="margin: 0; color: #64748b;">Tracking Number</p>
        <div class="tracking-number">{contactfield=tracking_number|TH1234567890}</div>
        <p style="margin: 0; color: #64748b;">Carrier: {contactfield=carrier|Standard Shipping}</p>
      </div>

      <div class="timeline">
        <div class="timeline-item">
          <span class="timeline-dot"></span>
          <div>
            <strong>Order Confirmed</strong>
            <p style="margin: 0; color: #64748b;">Your order was received</p>
          </div>
        </div>
        <div class="timeline-item">
          <span class="timeline-dot"></span>
          <div>
            <strong>Shipped</strong>
            <p style="margin: 0; color: #64748b;">Package picked up by carrier</p>
          </div>
        </div>
        <div class="timeline-item">
          <span class="timeline-dot inactive"></span>
          <div style="color: #94a3b8;">
            <strong>Out for Delivery</strong>
            <p style="margin: 0;">Coming soon</p>
          </div>
        </div>
        <div class="timeline-item">
          <span class="timeline-dot inactive"></span>
          <div style="color: #94a3b8;">
            <strong>Delivered</strong>
            <p style="margin: 0;">Estimated: {contactfield=delivery_date|3-5 business days}</p>
          </div>
        </div>
      </div>

      <center>
        <a href="https://marketplace.tesserix.app/track/{contactfield=order_id|order}" class="cta-button">Track My Package</a>
      </center>

      <p>Thank you for shopping with us!</p>
      <p>The Tesseract Hub Team</p>
    </div>
    <div class="footer">
      <p>Questions about your delivery? <a href="mailto:support@tesserix.app" style="color: #94a3b8;">Contact Support</a></p>
    </div>
  </div>
</body>
</html>`,
		},
	}
}

func main() {
	// Get configuration from environment
	mauticURL := os.Getenv("MAUTIC_URL")
	if mauticURL == "" {
		mauticURL = "https://dev-mautic.tesserix.app"
	}

	username := os.Getenv("MAUTIC_USERNAME")
	if username == "" {
		username = "admin"
	}

	password := os.Getenv("MAUTIC_PASSWORD")
	if password == "" {
		log.Fatal("MAUTIC_PASSWORD environment variable is required")
	}

	fromEmail := os.Getenv("FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "noreply@tesserix.app"
	}

	fromName := os.Getenv("FROM_NAME")
	if fromName == "" {
		fromName = "Tesseract Hub"
	}

	testEmail := os.Getenv("TEST_EMAIL")

	log.Println("===========================================")
	log.Println("  Mautic Benchmark Campaign Initialization")
	log.Println("===========================================")
	log.Printf("Mautic URL: %s", mauticURL)
	log.Printf("Username: %s", username)
	log.Printf("From: %s <%s>", fromName, fromEmail)
	log.Println()

	client := NewMauticClient(mauticURL, username, password)

	// =========================================
	// Step 1: Create Benchmark Segments
	// =========================================
	log.Println("📁 Creating benchmark segments...")
	segments := getBenchmarkSegments()
	segmentIDs := make(map[string]int)

	for _, segment := range segments {
		id, err := client.CreateSegment(&segment)
		if err != nil {
			log.Printf("  ⚠️  Failed to create segment '%s': %v", segment.Name, err)
		} else if id > 0 {
			log.Printf("  ✅ Created segment: %s (ID: %d)", segment.Name, id)
			segmentIDs[segment.Alias] = id
		} else {
			log.Printf("  ⏭️  Segment '%s' already exists", segment.Name)
		}
	}
	log.Println()

	// =========================================
	// Step 2: Create Benchmark Email Templates
	// =========================================
	log.Println("📧 Creating benchmark email templates...")
	emails := getBenchmarkEmails(fromEmail, fromName)
	emailIDs := make(map[string]int)

	for _, email := range emails {
		id, err := client.CreateEmail(&email)
		if err != nil {
			log.Printf("  ⚠️  Failed to create email '%s': %v", email.Name, err)
		} else if id > 0 {
			log.Printf("  ✅ Created email: %s (ID: %d)", email.Name, id)
			emailIDs[email.Name] = id
		} else {
			log.Printf("  ⏭️  Email '%s' already exists", email.Name)
		}
	}
	log.Println()

	// =========================================
	// Step 3: Send Test Email (if TEST_EMAIL provided)
	// =========================================
	if testEmail != "" {
		log.Printf("📬 Sending test email to %s...", testEmail)

		// Create test contact
		contact := &Contact{
			Email:     testEmail,
			FirstName: "Test",
			LastName:  "User",
		}
		contactID, err := client.CreateContact(contact)
		if err != nil {
			log.Printf("  ⚠️  Failed to create test contact: %v", err)
		} else {
			log.Printf("  ✅ Created test contact (ID: %d)", contactID)

			// Send welcome email
			welcomeEmailID := emailIDs["[Benchmark] Welcome Email"]
			if welcomeEmailID > 0 {
				if err := client.SendEmailToContact(welcomeEmailID, contactID); err != nil {
					log.Printf("  ⚠️  Failed to send test email: %v", err)
				} else {
					log.Printf("  ✅ Sent welcome email to %s", testEmail)
				}
			}

			// Add to newsletter segment
			newsletterSegmentID := segmentIDs["newsletter-subscribers"]
			if newsletterSegmentID > 0 {
				if err := client.AddContactToSegment(contactID, newsletterSegmentID); err != nil {
					log.Printf("  ⚠️  Failed to add to newsletter segment: %v", err)
				} else {
					log.Printf("  ✅ Added to newsletter segment")
				}
			}
		}
		log.Println()
	}

	// =========================================
	// Summary
	// =========================================
	log.Println("===========================================")
	log.Println("  ✅ Initialization Complete!")
	log.Println("===========================================")
	log.Println()
	log.Println("Created Resources:")
	log.Printf("  • Segments: %d", len(segments))
	log.Printf("  • Email Templates: %d", len(emails))
	log.Println()
	log.Println("Next Steps:")
	log.Println("  1. Login to Mautic: " + mauticURL)
	log.Println("  2. Review and customize email templates")
	log.Println("  3. Configure segment filters for dynamic lists")
	log.Println("  4. Create campaigns using the templates")
	log.Println("  5. Set up webhook triggers from your services")
	log.Println()
	log.Println("Benchmark Campaigns to Create Manually:")
	log.Println("  • Welcome Series (3 emails over 7 days)")
	log.Println("  • Abandoned Cart Recovery (3 emails)")
	log.Println("  • Post-Purchase Review Request (3 days after delivery)")
	log.Println("  • Win-Back Campaign (for inactive customers)")
	log.Println("  • VIP Exclusive Offers (monthly)")
	log.Println()
}
