package services

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// SNSProvider implements SMS sending via AWS SNS
type SNSProvider struct {
	client *sns.Client
	from   string // Sender ID or origination number
	region string
}

// NewSNSProvider creates a new AWS SNS SMS provider
func NewSNSProvider(cfg *ProviderConfig) (*SNSProvider, error) {
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

	return &SNSProvider{
		client: sns.NewFromConfig(awsCfg),
		from:   cfg.SNSFrom,
		region: cfg.AWSRegion,
	}, nil
}

// Send sends an SMS via AWS SNS
func (p *SNSProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	// Build the publish input
	input := &sns.PublishInput{
		Message:     aws.String(message.Body),
		PhoneNumber: aws.String(message.To),
	}

	// Add message attributes for SMS settings
	input.MessageAttributes = make(map[string]types.MessageAttributeValue)

	// Set SMS type - Transactional for OTP/MFA, Promotional for marketing
	smsType := "Transactional"
	if msgType, ok := message.Metadata["sms_type"].(string); ok {
		smsType = msgType
	}
	input.MessageAttributes["AWS.SNS.SMS.SMSType"] = types.MessageAttributeValue{
		DataType:    aws.String("String"),
		StringValue: aws.String(smsType),
	}

	// Set sender ID if configured (not supported in all regions/countries)
	if p.from != "" {
		input.MessageAttributes["AWS.SNS.SMS.SenderID"] = types.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(p.from),
		}
	}

	// Set max price if specified in metadata
	if maxPrice, ok := message.Metadata["max_price"].(string); ok {
		input.MessageAttributes["AWS.SNS.SMS.MaxPrice"] = types.MessageAttributeValue{
			DataType:    aws.String("Number"),
			StringValue: aws.String(maxPrice),
		}
	}

	// Send the SMS
	result, err := p.client.Publish(ctx, input)
	if err != nil {
		return &SendResult{
			ProviderName: "AWS SNS",
			Success:      false,
			Error:        fmt.Errorf("SNS send failed: %w", err),
		}, err
	}

	return &SendResult{
		ProviderID:   aws.ToString(result.MessageId),
		ProviderName: "AWS SNS",
		Success:      true,
		ProviderData: map[string]interface{}{
			"message_id": aws.ToString(result.MessageId),
			"to":         message.To,
			"sms_type":   smsType,
			"region":     p.region,
		},
	}, nil
}

// GetName returns the provider name
func (p *SNSProvider) GetName() string {
	return "AWS SNS"
}

// SupportsChannel returns the supported channel
func (p *SNSProvider) SupportsChannel() string {
	return "SMS"
}

// SendOTP sends an OTP/verification code via SMS with optimized settings
func (p *SNSProvider) SendOTP(ctx context.Context, phoneNumber, code string) (*SendResult, error) {
	message := &Message{
		To:   phoneNumber,
		Body: fmt.Sprintf("Your verification code is: %s. This code expires in 10 minutes.", code),
		Metadata: map[string]interface{}{
			"sms_type":  "Transactional", // Higher delivery priority
			"max_price": "0.50",          // Max price per SMS segment
		},
	}
	return p.Send(ctx, message)
}

// SendMFA sends an MFA code via SMS
func (p *SNSProvider) SendMFA(ctx context.Context, phoneNumber, code string) (*SendResult, error) {
	message := &Message{
		To:   phoneNumber,
		Body: fmt.Sprintf("Your Tesseract Hub login code is: %s", code),
		Metadata: map[string]interface{}{
			"sms_type": "Transactional",
		},
	}
	return p.Send(ctx, message)
}
