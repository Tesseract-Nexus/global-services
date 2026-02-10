package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/account"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"subscription-service/internal/config"
	"subscription-service/internal/models"
)

type StripeSettingsService struct {
	cfg      *config.Config
	smClient *secretmanager.Client
	mu       sync.Mutex // prevents concurrent key verifications
}

func NewStripeSettingsService(cfg *config.Config) (*StripeSettingsService, error) {
	svc := &StripeSettingsService{cfg: cfg}

	if cfg.UseGCPSecretManager {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create GCP Secret Manager client: %w", err)
		}
		svc.smClient = client
	}

	return svc, nil
}

func (s *StripeSettingsService) GetStripeStatus(ctx context.Context) models.StripeSettingsResponse {
	secretKey := s.cfg.GetStripeSecretKey()
	webhookSecret := s.cfg.GetStripeWebhookSecret()

	keySource := "env"
	if s.cfg.UseGCPSecretManager {
		keySource = "gcp"
	}

	return models.StripeSettingsResponse{
		SecretKeyConfigured:     secretKey != "",
		SecretKeyHint:           maskKey(secretKey),
		WebhookSecretConfigured: webhookSecret != "",
		WebhookSecretHint:       maskKey(webhookSecret),
		KeySource:               keySource,
	}
}

func (s *StripeSettingsService) VerifyStripeKey(ctx context.Context, secretKey string) models.VerifyStripeKeyResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use a client with the provided key (not the global stripe.Key)
	acctClient := account.Client{
		B:   stripe.GetBackend(stripe.APIBackend),
		Key: secretKey,
	}

	acct, err := acctClient.Get()
	if err != nil {
		return models.VerifyStripeKeyResponse{
			Valid: false,
			Error: err.Error(),
		}
	}

	accountName := ""
	if acct.BusinessProfile != nil {
		accountName = acct.BusinessProfile.Name
	}
	if accountName == "" && acct.Settings != nil && acct.Settings.Dashboard != nil {
		accountName = acct.Settings.Dashboard.DisplayName
	}

	return models.VerifyStripeKeyResponse{
		Valid:       true,
		AccountID:   acct.ID,
		AccountName: accountName,
		Livemode:    strings.HasPrefix(secretKey, "sk_live_") || strings.HasPrefix(secretKey, "rk_live_"),
	}
}

func (s *StripeSettingsService) UpdateStripeKeys(ctx context.Context, req models.UpdateStripeKeysRequest) error {
	if !s.cfg.UseGCPSecretManager || s.smClient == nil {
		return fmt.Errorf("GCP Secret Manager is not enabled; cannot update keys remotely")
	}

	if s.cfg.GCPProjectID == "" {
		return fmt.Errorf("GCP_PROJECT_ID is not configured")
	}

	if req.SecretKey != nil && *req.SecretKey != "" {
		// Verify the key before storing
		result := s.VerifyStripeKey(ctx, *req.SecretKey)
		if !result.Valid {
			return fmt.Errorf("stripe key verification failed: %s", result.Error)
		}

		secretName := envOrDefault("STRIPE_SECRET_KEY_SECRET_NAME", "stripe-secret-key")
		if err := s.createOrUpdateGCPSecret(ctx, secretName, *req.SecretKey); err != nil {
			return fmt.Errorf("failed to update Stripe secret key in GCP: %w", err)
		}
		log.Printf("Stripe secret key updated in GCP Secret Manager")
	}

	if req.WebhookSecret != nil && *req.WebhookSecret != "" {
		secretName := envOrDefault("STRIPE_WEBHOOK_SECRET_SECRET_NAME", "stripe-webhook-secret")
		if err := s.createOrUpdateGCPSecret(ctx, secretName, *req.WebhookSecret); err != nil {
			return fmt.Errorf("failed to update Stripe webhook secret in GCP: %w", err)
		}
		log.Printf("Stripe webhook secret updated in GCP Secret Manager")
	}

	// Reload keys into memory after update
	s.cfg.ReloadStripeKeys()

	return nil
}

func (s *StripeSettingsService) ReloadKeys(ctx context.Context) models.StripeSettingsResponse {
	s.cfg.ReloadStripeKeys()
	return s.GetStripeStatus(ctx)
}

// createOrUpdateGCPSecret creates or updates a secret in GCP Secret Manager.
func (s *StripeSettingsService) createOrUpdateGCPSecret(ctx context.Context, secretID, value string) error {
	parent := fmt.Sprintf("projects/%s", s.cfg.GCPProjectID)
	secretPath := fmt.Sprintf("%s/secrets/%s", parent, secretID)

	// Check if secret exists
	_, err := s.smClient.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{
		Name: secretPath,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// Create the secret
			_, err = s.smClient.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
				Parent:   parent,
				SecretId: secretID,
				Secret: &secretmanagerpb.Secret{
					Replication: &secretmanagerpb.Replication{
						Replication: &secretmanagerpb.Replication_Automatic_{
							Automatic: &secretmanagerpb.Replication_Automatic{},
						},
					},
					Labels: map[string]string{
						"managed_by": "subscription-service",
					},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to create secret %s: %w", secretID, err)
			}
		} else {
			return fmt.Errorf("failed to check secret %s: %w", secretID, err)
		}
	}

	// Add a new version with the value
	_, err = s.smClient.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretPath,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(value),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add secret version for %s: %w", secretID, err)
	}

	return nil
}

// envOrDefault reads an env var or returns a fallback.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return key[:2] + "...****"
	}
	// Show prefix (up to first underscore group) + last 4 chars
	// e.g. "sk_live_...abcd" or "whsec_...efgh"
	parts := strings.SplitN(key, "_", 3)
	prefix := key[:4] // fallback
	if len(parts) >= 2 {
		prefix = parts[0] + "_" + parts[1] + "_"
	}
	last4 := key[len(key)-4:]
	return prefix + "..." + last4
}
