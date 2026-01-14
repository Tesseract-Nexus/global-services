package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tesseract-hub/go-shared/security"
)

// MauticProvider implements email sending via Mautic API
// Mautic is an open-source marketing automation platform
// It sends emails through Postal SMTP under the hood
type MauticProvider struct {
	baseURL    string
	username   string
	password   string
	from       string
	fromName   string
	httpClient *http.Client
}

// NewMauticProvider creates a new Mautic provider
func NewMauticProvider(config *ProviderConfig) *MauticProvider {
	return &MauticProvider{
		baseURL:  strings.TrimSuffix(config.MauticURL, "/"),
		username: config.MauticUsername,
		password: config.MauticPassword,
		from:     config.MauticFrom,
		fromName: config.MauticFromName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// MauticEmail represents an email to send via Mautic
type MauticEmail struct {
	Subject       string   `json:"subject"`
	Body          string   `json:"body"`
	PlainText     string   `json:"plainText,omitempty"`
	FromAddress   string   `json:"fromAddress,omitempty"`
	FromName      string   `json:"fromName,omitempty"`
	ReplyToEmail  string   `json:"replyToEmail,omitempty"`
	IsPublished   bool     `json:"isPublished"`
	EmailType     string   `json:"emailType"` // "list" or "template"
	Contacts      []int    `json:"contacts,omitempty"`
	ContactEmails []string `json:"-"` // Internal use for direct sending
}

// MauticContact represents a contact in Mautic
type MauticContact struct {
	Email     string            `json:"email"`
	FirstName string            `json:"firstname,omitempty"`
	LastName  string            `json:"lastname,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Fields    map[string]string `json:"-"`
}

// MauticEmailResponse represents the API response for email operations
type MauticEmailResponse struct {
	Email struct {
		ID int `json:"id"`
	} `json:"email"`
	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"errors,omitempty"`
}

// MauticContactResponse represents the API response for contact operations
type MauticContactResponse struct {
	Contact struct {
		ID int `json:"id"`
	} `json:"contact"`
	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"errors,omitempty"`
}

// Send sends an email via Mautic
// This creates/updates a contact and sends a transactional email
func (p *MauticProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	startTime := time.Now()
	log.Printf("[MAUTIC] Sending email to %s, subject: %s", security.MaskEmail(message.To), message.Subject)

	// Step 1: Create or update contact
	contactID, err := p.createOrUpdateContact(ctx, message.To, message.Metadata)
	if err != nil {
		log.Printf("[MAUTIC] Failed to create/update contact: %v", err)
		return &SendResult{
			ProviderName: "Mautic",
			Success:      false,
			Error:        fmt.Errorf("contact creation failed: %w", err),
		}, err
	}

	// Step 2: Create transactional email
	email := &MauticEmail{
		Subject:     message.Subject,
		Body:        message.BodyHTML,
		PlainText:   message.Body,
		FromAddress: p.from,
		FromName:    p.fromName,
		IsPublished: true,
		EmailType:   "template",
	}

	if message.From != "" {
		email.FromAddress = message.From
	}
	if message.FromName != "" {
		email.FromName = message.FromName
	}
	if message.ReplyTo != "" {
		email.ReplyToEmail = message.ReplyTo
	}

	emailID, err := p.createEmail(ctx, email)
	if err != nil {
		log.Printf("[MAUTIC] Failed to create email: %v", err)
		return &SendResult{
			ProviderName: "Mautic",
			Success:      false,
			Error:        fmt.Errorf("email creation failed: %w", err),
		}, err
	}

	// Step 3: Send email to contact
	if err := p.sendEmailToContact(ctx, emailID, contactID); err != nil {
		log.Printf("[MAUTIC] Failed to send email: %v", err)
		return &SendResult{
			ProviderName: "Mautic",
			Success:      false,
			Error:        fmt.Errorf("email send failed: %w", err),
		}, err
	}

	log.Printf("[MAUTIC] Email sent successfully to %s (took %v)", security.MaskEmail(message.To), time.Since(startTime))
	return &SendResult{
		ProviderName: "Mautic",
		ProviderID:   fmt.Sprintf("mautic-email-%d-contact-%d", emailID, contactID),
		Success:      true,
		ProviderData: map[string]interface{}{
			"emailId":   emailID,
			"contactId": contactID,
			"to":        message.To,
			"subject":   message.Subject,
			"duration":  time.Since(startTime).String(),
		},
	}, nil
}

// createOrUpdateContact creates or updates a contact in Mautic
func (p *MauticProvider) createOrUpdateContact(ctx context.Context, email string, metadata map[string]interface{}) (int, error) {
	contact := map[string]interface{}{
		"email":     email,
		"overwriteWithBlank": false,
	}

	// Extract name from metadata if available
	if metadata != nil {
		if firstName, ok := metadata["firstName"].(string); ok {
			contact["firstname"] = firstName
		}
		if lastName, ok := metadata["lastName"].(string); ok {
			contact["lastname"] = lastName
		}
		if customerName, ok := metadata["customerName"].(string); ok && contact["firstname"] == nil {
			// Split customer name into first/last if not already set
			parts := strings.SplitN(customerName, " ", 2)
			if len(parts) > 0 {
				contact["firstname"] = parts[0]
			}
			if len(parts) > 1 {
				contact["lastname"] = parts[1]
			}
		}
	}

	body, err := json.Marshal(contact)
	if err != nil {
		return 0, err
	}

	// Use create/edit endpoint which handles both
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/contacts/new", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("Mautic API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var contactResp MauticContactResponse
	if err := json.Unmarshal(respBody, &contactResp); err != nil {
		return 0, err
	}

	if len(contactResp.Errors) > 0 {
		return 0, fmt.Errorf("Mautic error: %s", contactResp.Errors[0].Message)
	}

	return contactResp.Contact.ID, nil
}

// createEmail creates a transactional email in Mautic
func (p *MauticProvider) createEmail(ctx context.Context, email *MauticEmail) (int, error) {
	body, err := json.Marshal(email)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/emails/new", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	p.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("Mautic API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var emailResp MauticEmailResponse
	if err := json.Unmarshal(respBody, &emailResp); err != nil {
		return 0, err
	}

	if len(emailResp.Errors) > 0 {
		return 0, fmt.Errorf("Mautic error: %s", emailResp.Errors[0].Message)
	}

	return emailResp.Email.ID, nil
}

// sendEmailToContact sends an existing email to a specific contact
func (p *MauticProvider) sendEmailToContact(ctx context.Context, emailID, contactID int) error {
	url := fmt.Sprintf("%s/api/emails/%d/contact/%d/send", p.baseURL, emailID, contactID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	p.setAuthHeader(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Mautic send error: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SubscribeToNewsletter subscribes an email to a newsletter segment
func (p *MauticProvider) SubscribeToNewsletter(ctx context.Context, email, firstName, lastName string, segmentIDs []int) error {
	log.Printf("[MAUTIC] Subscribing %s to newsletter segments: %v", security.MaskEmail(email), segmentIDs)

	// Create/update contact
	contactID, err := p.createOrUpdateContact(ctx, email, map[string]interface{}{
		"firstName": firstName,
		"lastName":  lastName,
	})
	if err != nil {
		return err
	}

	// Add to segments
	for _, segmentID := range segmentIDs {
		if err := p.addContactToSegment(ctx, contactID, segmentID); err != nil {
			log.Printf("[MAUTIC] Warning: Failed to add contact to segment %d: %v", segmentID, err)
		}
	}

	return nil
}

// addContactToSegment adds a contact to a segment (list)
func (p *MauticProvider) addContactToSegment(ctx context.Context, contactID, segmentID int) error {
	url := fmt.Sprintf("%s/api/segments/%d/contact/%d/add", p.baseURL, segmentID, contactID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	p.setAuthHeader(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("segment add error: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// setAuthHeader sets the Basic Auth header for API requests
func (p *MauticProvider) setAuthHeader(req *http.Request) {
	auth := base64.StdEncoding.EncodeToString([]byte(p.username + ":" + p.password))
	req.Header.Set("Authorization", "Basic "+auth)
}

// GetName returns the provider name
func (p *MauticProvider) GetName() string {
	return "Mautic"
}

// SupportsChannel returns the supported channel
func (p *MauticProvider) SupportsChannel() string {
	return "EMAIL"
}
