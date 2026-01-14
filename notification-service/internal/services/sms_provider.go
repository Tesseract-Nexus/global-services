package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// TwilioProvider implements SMS sending via Twilio
type TwilioProvider struct {
	accountSID string
	authToken  string
	from       string
	client     *http.Client
}

// NewTwilioProvider creates a new Twilio SMS provider
func NewTwilioProvider(config *ProviderConfig) *TwilioProvider {
	return &TwilioProvider{
		accountSID: config.TwilioAccountSID,
		authToken:  config.TwilioAuthToken,
		from:       config.TwilioFrom,
		client:     &http.Client{},
	}
}

// Send sends an SMS via Twilio
func (p *TwilioProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	// Twilio API endpoint
	urlStr := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", p.accountSID)

	// Prepare form data
	data := url.Values{}
	data.Set("To", message.To)
	data.Set("From", p.from)
	data.Set("Body", message.Body)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, strings.NewReader(data.Encode()))
	if err != nil {
		return &SendResult{
			ProviderName: "Twilio",
			Success:      false,
			Error:        err,
		}, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(p.accountSID, p.authToken)

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return &SendResult{
			ProviderName: "Twilio",
			Success:      false,
			Error:        err,
		}, err
	}
	defer resp.Body.Close()

	// Parse response
	var twilioResp TwilioResponse
	if err := json.NewDecoder(resp.Body).Decode(&twilioResp); err != nil {
		return &SendResult{
			ProviderName: "Twilio",
			Success:      false,
			Error:        err,
		}, err
	}

	// Check if successful
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &SendResult{
			ProviderID:   twilioResp.SID,
			ProviderName: "Twilio",
			Success:      true,
			ProviderData: map[string]interface{}{
				"sid":    twilioResp.SID,
				"status": twilioResp.Status,
				"to":     twilioResp.To,
				"from":   twilioResp.From,
			},
		}, nil
	}

	// Request failed
	errorMsg := fmt.Sprintf("Twilio API error: %d", resp.StatusCode)
	if twilioResp.Message != "" {
		errorMsg = fmt.Sprintf("%s - %s", errorMsg, twilioResp.Message)
	}

	return &SendResult{
		ProviderName: "Twilio",
		Success:      false,
		Error:        fmt.Errorf("%s", errorMsg),
	}, fmt.Errorf("%s", errorMsg)
}

// GetName returns the provider name
func (p *TwilioProvider) GetName() string {
	return "Twilio"
}

// SupportsChannel returns the supported channel
func (p *TwilioProvider) SupportsChannel() string {
	return "SMS"
}

// TwilioResponse represents Twilio API response
type TwilioResponse struct {
	SID          string `json:"sid"`
	DateCreated  string `json:"date_created"`
	DateUpdated  string `json:"date_updated"`
	DateSent     string `json:"date_sent"`
	AccountSID   string `json:"account_sid"`
	To           string `json:"to"`
	From         string `json:"from"`
	Body         string `json:"body"`
	Status       string `json:"status"`
	NumSegments  string `json:"num_segments"`
	NumMedia     string `json:"num_media"`
	Direction    string `json:"direction"`
	APIVersion   string `json:"api_version"`
	Price        string `json:"price"`
	PriceUnit    string `json:"price_unit"`
	ErrorCode    int    `json:"error_code"`
	ErrorMessage string `json:"error_message"`
	URI          string `json:"uri"`
	SubresourceURIs map[string]string `json:"subresource_uris"`

	// Error response fields
	Code     int    `json:"code"`
	Message  string `json:"message"`
	MoreInfo string `json:"more_info"`
}
