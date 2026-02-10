package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	webpush "github.com/SherClockHolmes/webpush-go"
	"notification-service/internal/models"
)

// WebPushPayload represents the payload sent to the browser
type WebPushPayload struct {
	Title string                 `json:"title"`
	Body  string                 `json:"body"`
	Icon  string                 `json:"icon,omitempty"`
	Badge string                 `json:"badge,omitempty"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

// WebPushProvider sends Web Push notifications using VAPID
type WebPushProvider struct {
	vapidPublicKey  string
	vapidPrivateKey string
	vapidSubject    string
}

// NewWebPushProvider creates a new Web Push provider
func NewWebPushProvider(publicKey, privateKey, subject string) (*WebPushProvider, error) {
	if publicKey == "" || privateKey == "" {
		return nil, fmt.Errorf("VAPID public and private keys are required")
	}
	if subject == "" {
		subject = "mailto:push@tesserix.app"
	}
	return &WebPushProvider{
		vapidPublicKey:  publicKey,
		vapidPrivateKey: privateKey,
		vapidSubject:    subject,
	}, nil
}

// SendToSubscription sends a push notification to a single Web Push subscription
func (p *WebPushProvider) SendToSubscription(ctx context.Context, sub *models.PushSubscription, payload WebPushPayload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal push payload: %w", err)
	}

	s := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.Keys.P256dh,
			Auth:   sub.Keys.Auth,
		},
	}

	resp, err := webpush.SendNotification(payloadBytes, s, &webpush.Options{
		Subscriber:      p.vapidSubject,
		VAPIDPublicKey:  p.vapidPublicKey,
		VAPIDPrivateKey: p.vapidPrivateKey,
		TTL:             86400, // 24 hours
	})
	if err != nil {
		return fmt.Errorf("failed to send web push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 410 || resp.StatusCode == 404 {
		return fmt.Errorf("subscription expired or invalid (status %d)", resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("push service returned status %d", resp.StatusCode)
	}

	endpointPreview := sub.Endpoint
	if len(endpointPreview) > 50 {
		endpointPreview = endpointPreview[:50] + "..."
	}
	log.Printf("[WebPush] Notification sent to %s (status: %d)", endpointPreview, resp.StatusCode)
	return nil
}

// GetVAPIDPublicKey returns the VAPID public key for client-side subscription
func (p *WebPushProvider) GetVAPIDPublicKey() string {
	return p.vapidPublicKey
}
