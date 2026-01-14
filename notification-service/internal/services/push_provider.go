package services

import (
	"context"
	"encoding/json"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMProvider implements push notifications via Firebase Cloud Messaging
type FCMProvider struct {
	projectID   string
	client      *messaging.Client
	credentials string
}

// NewFCMProvider creates a new FCM push notification provider
func NewFCMProvider(config *ProviderConfig) (*FCMProvider, error) {
	ctx := context.Background()

	// Setup Firebase app options
	var opts []option.ClientOption
	if config.FCMCredentials != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(config.FCMCredentials)))
	}

	// Create Firebase app
	conf := &firebase.Config{
		ProjectID: config.FCMProjectID,
	}
	app, err := firebase.NewApp(ctx, conf, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firebase app: %w", err)
	}

	// Get messaging client
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get FCM client: %w", err)
	}

	return &FCMProvider{
		projectID:   config.FCMProjectID,
		client:      client,
		credentials: config.FCMCredentials,
	}, nil
}

// Send sends a push notification via FCM
func (p *FCMProvider) Send(ctx context.Context, message *Message) (*SendResult, error) {
	// Build FCM message
	fcmMessage := &messaging.Message{
		Token: message.To,
		Notification: &messaging.Notification{
			Title: message.Subject,
			Body:  message.Body,
		},
	}

	// Add custom data from metadata
	if message.Metadata != nil {
		data := make(map[string]string)
		for key, value := range message.Metadata {
			// Convert to string
			switch v := value.(type) {
			case string:
				data[key] = v
			case int, int64, float64, bool:
				data[key] = fmt.Sprintf("%v", v)
			default:
				// Try to marshal to JSON
				if jsonBytes, err := json.Marshal(v); err == nil {
					data[key] = string(jsonBytes)
				}
			}
		}
		fcmMessage.Data = data
	}

	// Add Android-specific config if needed
	fcmMessage.Android = &messaging.AndroidConfig{
		Priority: "high",
	}

	// Add iOS-specific config if needed
	fcmMessage.APNS = &messaging.APNSConfig{
		Headers: map[string]string{
			"apns-priority": "10",
		},
		Payload: &messaging.APNSPayload{
			Aps: &messaging.Aps{
				Alert: &messaging.ApsAlert{
					Title: message.Subject,
					Body:  message.Body,
				},
				Badge: new(int),
				Sound: "default",
			},
		},
	}

	// Send the message
	response, err := p.client.Send(ctx, fcmMessage)
	if err != nil {
		return &SendResult{
			ProviderName: "FCM",
			Success:      false,
			Error:        err,
		}, err
	}

	return &SendResult{
		ProviderID:   response,
		ProviderName: "FCM",
		Success:      true,
		ProviderData: map[string]interface{}{
			"message_id": response,
			"token":      message.To,
		},
	}, nil
}

// SendMulticast sends a push notification to multiple devices
func (p *FCMProvider) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) (*messaging.BatchResponse, error) {
	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		APNS: &messaging.APNSConfig{
			Headers: map[string]string{
				"apns-priority": "10",
			},
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Alert: &messaging.ApsAlert{
						Title: title,
						Body:  body,
					},
					Badge: new(int),
					Sound: "default",
				},
			},
		},
	}

	return p.client.SendMulticast(ctx, message)
}

// GetName returns the provider name
func (p *FCMProvider) GetName() string {
	return "FCM"
}

// SupportsChannel returns the supported channel
func (p *FCMProvider) SupportsChannel() string {
	return "PUSH"
}
