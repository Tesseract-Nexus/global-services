package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tesseract-hub/go-shared/security"
)

// PostalHTTPProvider implements email sending via Postal HTTP API
// This is simpler and more reliable than SMTP, especially in Kubernetes
// Postal API docs: https://docs.postalserver.io/developer/api
type PostalHTTPProvider struct {
	apiURL   string
	apiKey   string
	from     string
	fromName string
	client   *http.Client
}

// PostalHTTPConfig holds configuration for Postal HTTP API
type PostalHTTPConfig struct {
	APIURL   string // e.g., http://postal-web.email.svc.cluster.local:5000
	APIKey   string // Server API key from Postal (format: key@server)
	From     string // From email address
	FromName string // From display name
}

// PostalSendRequest represents the Postal API send request
type PostalSendRequest struct {
	To        []string `json:"to"`
	CC        []string `json:"cc,omitempty"`
	BCC       []string `json:"bcc,omitempty"`
	From      string   `json:"from"`
	Sender    string   `json:"sender"`
	Subject   string   `json:"subject"`
	PlainBody string   `json:"plain_body,omitempty"`
	HTMLBody  string   `json:"html_body,omitempty"`
	ReplyTo   string   `json:"reply_to,omitempty"`
	Tag       string   `json:"tag,omitempty"`
}

// PostalSendResponse represents the Postal API response
type PostalSendResponse struct {
	Status string `json:"status"`
	Data   struct {
		MessageID string                 `json:"message_id"`
		Messages  map[string]interface{} `json:"messages"`
	} `json:"data"`
	Time float64 `json:"time"`
}

// NewPostalHTTPProvider creates a new Postal HTTP API provider
func NewPostalHTTPProvider(config *PostalHTTPConfig) *PostalHTTPProvider {
	return &PostalHTTPProvider{
		apiURL:   strings.TrimSuffix(config.APIURL, "/"),
		apiKey:   config.APIKey,
		from:     config.From,
		fromName: config.FromName,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewPostalHTTPProviderFromConfig creates a provider from ProviderConfig
func NewPostalHTTPProviderFromConfig(config *ProviderConfig) *PostalHTTPProvider {
	return NewPostalHTTPProvider(&PostalHTTPConfig{
		APIURL:   config.PostalAPIURL,
		APIKey:   config.PostalAPIKey,
		From:     config.PostalFrom,
		FromName: config.PostalFromName,
	})
}

// Send sends an email via Postal HTTP API
func (p *PostalHTTPProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	startTime := time.Now()
	log.Printf("[POSTAL-HTTP] Sending email to %s, subject: %s", security.MaskEmail(message.To), message.Subject)

	// Build from address
	from := p.from
	if message.From != "" {
		from = message.From
	}

	// Build sender (envelope from)
	sender := from
	if strings.Contains(from, "<") {
		// Extract email from "Name <email>" format
		parts := strings.Split(from, "<")
		if len(parts) == 2 {
			sender = strings.TrimSuffix(parts[1], ">")
		}
	}

	// Build request
	req := PostalSendRequest{
		To:        []string{message.To},
		From:      from,
		Sender:    sender,
		Subject:   message.Subject,
		PlainBody: message.Body,
		HTMLBody:  message.BodyHTML,
	}

	// Add CC if provided
	if len(message.CC) > 0 {
		req.CC = message.CC
	}

	// Add BCC if provided
	if len(message.BCC) > 0 {
		req.BCC = message.BCC
	}

	// Add Reply-To if provided
	if message.ReplyTo != "" {
		req.ReplyTo = message.ReplyTo
	}

	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return &SendResult{
			ProviderName: "Postal-HTTP",
			Success:      false,
			Error:        fmt.Errorf("failed to marshal request: %w", err),
		}, err
	}

	// Create HTTP request
	apiEndpoint := fmt.Sprintf("%s/api/v1/send/message", p.apiURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, bytes.NewBuffer(body))
	if err != nil {
		return &SendResult{
			ProviderName: "Postal-HTTP",
			Success:      false,
			Error:        fmt.Errorf("failed to create request: %w", err),
		}, err
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Server-API-Key", p.apiKey)

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		log.Printf("[POSTAL-HTTP] Request failed: %v (took %v)", err, time.Since(startTime))
		return &SendResult{
			ProviderName: "Postal-HTTP",
			Success:      false,
			Error:        fmt.Errorf("HTTP request failed: %w", err),
		}, err
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &SendResult{
			ProviderName: "Postal-HTTP",
			Success:      false,
			Error:        fmt.Errorf("failed to read response: %w", err),
		}, err
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Postal API returned %d: %s", resp.StatusCode, string(respBody))
		log.Printf("[POSTAL-HTTP] %s (took %v)", errMsg, time.Since(startTime))
		return &SendResult{
			ProviderName: "Postal-HTTP",
			Success:      false,
			Error:        fmt.Errorf("%s", errMsg),
		}, fmt.Errorf("%s", errMsg)
	}

	// Parse response
	var postalResp PostalSendResponse
	if err := json.Unmarshal(respBody, &postalResp); err != nil {
		log.Printf("[POSTAL-HTTP] Failed to parse response: %v", err)
		// Still consider it a success if status code was 200
	}

	// Check response status
	if postalResp.Status != "success" && postalResp.Status != "" {
		errMsg := fmt.Sprintf("Postal API error: status=%s", postalResp.Status)
		log.Printf("[POSTAL-HTTP] %s (took %v)", errMsg, time.Since(startTime))
		return &SendResult{
			ProviderName: "Postal-HTTP",
			Success:      false,
			Error:        fmt.Errorf("%s", errMsg),
		}, fmt.Errorf("%s", errMsg)
	}

	log.Printf("[POSTAL-HTTP] Email sent successfully to %s, message_id=%s (took %v)",
		security.MaskEmail(message.To), postalResp.Data.MessageID, time.Since(startTime))

	return &SendResult{
		ProviderID:   postalResp.Data.MessageID,
		ProviderName: "Postal-HTTP",
		Success:      true,
		ProviderData: map[string]interface{}{
			"to":         message.To,
			"subject":    message.Subject,
			"message_id": postalResp.Data.MessageID,
			"duration":   time.Since(startTime).String(),
		},
	}, nil
}

// GetName returns the provider name
func (p *PostalHTTPProvider) GetName() string {
	return "Postal-HTTP"
}

// SupportsChannel returns the supported channel
func (p *PostalHTTPProvider) SupportsChannel() string {
	return "EMAIL"
}

// IsConfigured returns true if the provider has required configuration
func (p *PostalHTTPProvider) IsConfigured() bool {
	return p.apiURL != "" && p.apiKey != "" && p.from != ""
}
