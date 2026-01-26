package clients

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GCPSecretManagerClient wraps the GCP Secret Manager client for provisioning operations
type GCPSecretManagerClient struct {
	client    *secretmanager.Client
	projectID string
	logger    *logrus.Entry
}

// NewGCPSecretManagerClient creates a new GCP Secret Manager client
// Uses Workload Identity when running in GKE
func NewGCPSecretManagerClient(ctx context.Context, projectID string, logger *logrus.Entry) (*GCPSecretManagerClient, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	return &GCPSecretManagerClient{
		client:    client,
		projectID: projectID,
		logger:    logger,
	}, nil
}

// CreateSecret creates a new secret in GCP Secret Manager
func (c *GCPSecretManagerClient) CreateSecret(ctx context.Context, secretID string, labels map[string]string) (*secretmanagerpb.Secret, error) {
	parent := fmt.Sprintf("projects/%s", c.projectID)

	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretID,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
			Labels: labels,
		},
	}

	secret, err := c.client.CreateSecret(ctx, req)
	if err != nil {
		// Log creation attempt (never log values)
		c.logger.WithFields(logrus.Fields{
			"secret_id": secretID,
			"operation": "create",
			"status":    "failed",
			"error":     err.Error(),
		}).Error("failed to create secret")
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"secret_id":   secretID,
		"secret_name": secret.Name,
		"operation":   "create",
		"status":      "success",
	}).Info("secret created")

	return secret, nil
}

// AddSecretVersion adds a new version to an existing secret
func (c *GCPSecretManagerClient) AddSecretVersion(ctx context.Context, secretName string, payload []byte) (*secretmanagerpb.SecretVersion, error) {
	req := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: payload,
		},
	}

	version, err := c.client.AddSecretVersion(ctx, req)
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"secret_name": secretName,
			"operation":   "add_version",
			"status":      "failed",
			"error":       err.Error(),
		}).Error("failed to add secret version")
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"secret_name": secretName,
		"version":     version.Name,
		"operation":   "add_version",
		"status":      "success",
	}).Info("secret version added")

	return version, nil
}

// GetSecret retrieves secret metadata (not the value)
func (c *GCPSecretManagerClient) GetSecret(ctx context.Context, secretID string) (*secretmanagerpb.Secret, error) {
	name := fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretID)

	req := &secretmanagerpb.GetSecretRequest{Name: name}
	return c.client.GetSecret(ctx, req)
}

// DeleteSecret deletes a secret
func (c *GCPSecretManagerClient) DeleteSecret(ctx context.Context, secretID string) error {
	name := fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretID)

	req := &secretmanagerpb.DeleteSecretRequest{Name: name}
	err := c.client.DeleteSecret(ctx, req)
	if err != nil {
		c.logger.WithFields(logrus.Fields{
			"secret_id": secretID,
			"operation": "delete",
			"status":    "failed",
			"error":     err.Error(),
		}).Error("failed to delete secret")
		return err
	}

	c.logger.WithFields(logrus.Fields{
		"secret_id": secretID,
		"operation": "delete",
		"status":    "success",
	}).Info("secret deleted")

	return nil
}

// UpdateSecretLabels updates the labels on a secret
func (c *GCPSecretManagerClient) UpdateSecretLabels(ctx context.Context, secretID string, labels map[string]string) error {
	name := fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretID)

	// Get current secret
	secret, err := c.client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{Name: name})
	if err != nil {
		return err
	}

	// Update labels
	secret.Labels = labels

	req := &secretmanagerpb.UpdateSecretRequest{
		Secret: secret,
	}

	_, err = c.client.UpdateSecret(ctx, req)
	return err
}

// SecretExists checks if a secret exists
func (c *GCPSecretManagerClient) SecretExists(ctx context.Context, secretID string) (bool, error) {
	_, err := c.GetSecret(ctx, secretID)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateOrUpdateSecret creates a secret if it doesn't exist, or adds a new version if it does
func (c *GCPSecretManagerClient) CreateOrUpdateSecret(ctx context.Context, secretID string, payload []byte, labels map[string]string) (string, error) {
	exists, err := c.SecretExists(ctx, secretID)
	if err != nil {
		return "", err
	}

	var secretName string
	if !exists {
		secret, err := c.CreateSecret(ctx, secretID, labels)
		if err != nil {
			return "", err
		}
		secretName = secret.Name
	} else {
		secretName = fmt.Sprintf("projects/%s/secrets/%s", c.projectID, secretID)
		// Update labels on existing secret
		if err := c.UpdateSecretLabels(ctx, secretID, labels); err != nil {
			c.logger.WithError(err).Warn("failed to update secret labels")
		}
	}

	version, err := c.AddSecretVersion(ctx, secretName, payload)
	if err != nil {
		return "", err
	}

	return version.Name, nil
}

// Close closes the client connection
func (c *GCPSecretManagerClient) Close() error {
	return c.client.Close()
}

// ExtractVersion extracts the version number from a version name
func ExtractVersion(versionName string) string {
	// Version name format: projects/PROJECT/secrets/SECRET/versions/VERSION
	// We want just the VERSION part
	if versionName == "" {
		return ""
	}
	// Simple extraction - get last part after "versions/"
	const prefix = "versions/"
	idx := len(versionName) - 1
	for idx >= 0 && versionName[idx] != '/' {
		idx--
	}
	if idx >= 0 {
		return versionName[idx+1:]
	}
	return versionName
}
