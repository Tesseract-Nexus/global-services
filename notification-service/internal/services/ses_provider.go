package services

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// SESProvider implements email sending via AWS SES
type SESProvider struct {
	client   *ses.Client
	from     string
	fromName string
	region   string
}

// NewSESProvider creates a new AWS SES email provider
func NewSESProvider(cfg *ProviderConfig) (*SESProvider, error) {
	// Build AWS config options
	var awsOpts []func(*config.LoadOptions) error

	// Set region
	if cfg.AWSRegion != "" {
		awsOpts = append(awsOpts, config.WithRegion(cfg.AWSRegion))
	}

	// If explicit credentials provided, use them; otherwise fall back to default chain
	// (environment vars, shared config, EC2 instance role, EKS pod identity, etc.)
	if cfg.AWSAccessKeyID != "" && cfg.AWSSecretAccessKey != "" {
		awsOpts = append(awsOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AWSAccessKeyID,
				cfg.AWSSecretAccessKey,
				"", // session token (optional)
			),
		))
	}

	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(context.Background(), awsOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &SESProvider{
		client:   ses.NewFromConfig(awsCfg),
		from:     cfg.SESFrom,
		fromName: cfg.SESFromName,
		region:   cfg.AWSRegion,
	}, nil
}

// Send sends an email via AWS SES
func (p *SESProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	// Build source (from) address
	source := p.from
	if p.fromName != "" {
		source = fmt.Sprintf("%s <%s>", p.fromName, p.from)
	}

	// Override from if provided in message
	if message.From != "" {
		source = message.From
		if message.FromName != "" {
			source = fmt.Sprintf("%s <%s>", message.FromName, message.From)
		}
	}

	// Build destination
	destination := &types.Destination{
		ToAddresses: []string{message.To},
	}

	// Add CC recipients
	if len(message.CC) > 0 {
		destination.CcAddresses = message.CC
	}

	// Add BCC recipients
	if len(message.BCC) > 0 {
		destination.BccAddresses = message.BCC
	}

	// Check if we have attachments - if so, use raw email
	if len(message.Attachments) > 0 {
		return p.sendRawEmail(ctx, source, destination, message)
	}

	// Build email body
	body := &types.Body{}
	if message.BodyHTML != "" {
		body.Html = &types.Content{
			Charset: aws.String("UTF-8"),
			Data:    aws.String(message.BodyHTML),
		}
	}
	if message.Body != "" {
		body.Text = &types.Content{
			Charset: aws.String("UTF-8"),
			Data:    aws.String(message.Body),
		}
	}

	// Build the message
	sesMessage := &types.Message{
		Subject: &types.Content{
			Charset: aws.String("UTF-8"),
			Data:    aws.String(message.Subject),
		},
		Body: body,
	}

	// Build send email input
	input := &ses.SendEmailInput{
		Source:      aws.String(source),
		Destination: destination,
		Message:     sesMessage,
	}

	// Add Reply-To if provided
	if message.ReplyTo != "" {
		input.ReplyToAddresses = []string{message.ReplyTo}
	}

	// Send the email
	result, err := p.client.SendEmail(ctx, input)
	if err != nil {
		return &SendResult{
			ProviderName: "AWS SES",
			Success:      false,
			Error:        fmt.Errorf("SES send failed: %w", err),
		}, err
	}

	return &SendResult{
		ProviderID:   aws.ToString(result.MessageId),
		ProviderName: "AWS SES",
		Success:      true,
		ProviderData: map[string]interface{}{
			"message_id": aws.ToString(result.MessageId),
			"to":         message.To,
			"subject":    message.Subject,
			"region":     p.region,
		},
	}, nil
}

// sendRawEmail sends email with attachments using raw MIME format
func (p *SESProvider) sendRawEmail(ctx context.Context, source string, destination *types.Destination, message *Message) (*SendResult, error) {
	// Build MIME message
	boundary := "----=_Part_0_1234567890"

	// Build headers
	rawMessage := fmt.Sprintf("From: %s\r\n", source)
	rawMessage += fmt.Sprintf("To: %s\r\n", message.To)
	if len(message.CC) > 0 {
		rawMessage += fmt.Sprintf("Cc: %s\r\n", joinStrings(message.CC, ", "))
	}
	rawMessage += fmt.Sprintf("Subject: %s\r\n", message.Subject)
	if message.ReplyTo != "" {
		rawMessage += fmt.Sprintf("Reply-To: %s\r\n", message.ReplyTo)
	}
	rawMessage += "MIME-Version: 1.0\r\n"
	rawMessage += fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", boundary)

	// Add body
	rawMessage += fmt.Sprintf("--%s\r\n", boundary)
	if message.BodyHTML != "" {
		rawMessage += "Content-Type: text/html; charset=UTF-8\r\n"
		rawMessage += "Content-Transfer-Encoding: 7bit\r\n\r\n"
		rawMessage += message.BodyHTML + "\r\n"
	} else {
		rawMessage += "Content-Type: text/plain; charset=UTF-8\r\n"
		rawMessage += "Content-Transfer-Encoding: 7bit\r\n\r\n"
		rawMessage += message.Body + "\r\n"
	}

	// Add attachments
	for _, att := range message.Attachments {
		rawMessage += fmt.Sprintf("--%s\r\n", boundary)
		rawMessage += fmt.Sprintf("Content-Type: %s; name=\"%s\"\r\n", att.ContentType, att.Filename)
		rawMessage += "Content-Transfer-Encoding: base64\r\n"
		rawMessage += fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", att.Filename)
		rawMessage += base64.StdEncoding.EncodeToString(att.Content) + "\r\n"
	}

	// End boundary
	rawMessage += fmt.Sprintf("--%s--\r\n", boundary)

	// Collect all destinations
	destinations := destination.ToAddresses
	destinations = append(destinations, destination.CcAddresses...)
	destinations = append(destinations, destination.BccAddresses...)

	// Send raw email
	input := &ses.SendRawEmailInput{
		Source:       aws.String(source),
		Destinations: destinations,
		RawMessage: &types.RawMessage{
			Data: []byte(rawMessage),
		},
	}

	result, err := p.client.SendRawEmail(ctx, input)
	if err != nil {
		return &SendResult{
			ProviderName: "AWS SES",
			Success:      false,
			Error:        fmt.Errorf("SES raw send failed: %w", err),
		}, err
	}

	return &SendResult{
		ProviderID:   aws.ToString(result.MessageId),
		ProviderName: "AWS SES",
		Success:      true,
		ProviderData: map[string]interface{}{
			"message_id":  aws.ToString(result.MessageId),
			"to":          message.To,
			"subject":     message.Subject,
			"region":      p.region,
			"attachments": len(message.Attachments),
		},
	}, nil
}

// GetName returns the provider name
func (p *SESProvider) GetName() string {
	return "AWS SES"
}

// SupportsChannel returns the supported channel
func (p *SESProvider) SupportsChannel() string {
	return "EMAIL"
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
