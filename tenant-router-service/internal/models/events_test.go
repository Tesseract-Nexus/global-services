package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTenantCreatedEvent_JSON(t *testing.T) {
	event := TenantCreatedEvent{
		EventType:    "tenant.created",
		TenantID:     "test-tenant-id",
		SessionID:    "test-session-id",
		Product:      "marketplace",
		BusinessName: "Test Business",
		Slug:         "test-business",
		Email:        "test@example.com",
		Timestamp:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	// Test marshaling
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	// Test unmarshaling
	var decoded TenantCreatedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if decoded.TenantID != event.TenantID {
		t.Errorf("expected tenant_id %s, got %s", event.TenantID, decoded.TenantID)
	}

	if decoded.Slug != event.Slug {
		t.Errorf("expected slug %s, got %s", event.Slug, decoded.Slug)
	}

	if decoded.BusinessName != event.BusinessName {
		t.Errorf("expected business_name %s, got %s", event.BusinessName, decoded.BusinessName)
	}
}

func TestTenantDeletedEvent_JSON(t *testing.T) {
	event := TenantDeletedEvent{
		EventType: "tenant.deleted",
		TenantID:  "test-tenant-id",
		Slug:      "test-business",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	// Test marshaling
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	// Test unmarshaling
	var decoded TenantDeletedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if decoded.TenantID != event.TenantID {
		t.Errorf("expected tenant_id %s, got %s", event.TenantID, decoded.TenantID)
	}

	if decoded.Slug != event.Slug {
		t.Errorf("expected slug %s, got %s", event.Slug, decoded.Slug)
	}
}

func TestTenantHost_Fields(t *testing.T) {
	now := time.Now()
	host := TenantHost{
		TenantID:       "test-tenant-id",
		Slug:           "test-business",
		AdminHost:      "test-business-admin.tesserix.app",
		StorefrontHost: "test-business.tesserix.app",
		CertName:       "test-business-tenant-tls",
		Status:         "provisioned",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if host.TenantID != "test-tenant-id" {
		t.Errorf("expected tenant_id test-tenant-id, got %s", host.TenantID)
	}

	if host.AdminHost != "test-business-admin.tesserix.app" {
		t.Errorf("expected admin host test-business-admin.tesserix.app, got %s", host.AdminHost)
	}

	if host.Status != "provisioned" {
		t.Errorf("expected status provisioned, got %s", host.Status)
	}
}

func TestProvisionResult_Success(t *testing.T) {
	result := ProvisionResult{
		TenantID:       "test-tenant-id",
		Slug:           "test-business",
		AdminHost:      "test-business-admin.tesserix.app",
		StorefrontHost: "test-business.tesserix.app",
		CertName:       "test-business-tenant-tls",
		Errors:         []string{},
		Success:        true,
	}

	if !result.Success {
		t.Error("expected success to be true")
	}

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestProvisionResult_Failure(t *testing.T) {
	result := ProvisionResult{
		TenantID:       "test-tenant-id",
		Slug:           "test-business",
		AdminHost:      "test-business-admin.tesserix.app",
		StorefrontHost: "test-business.tesserix.app",
		CertName:       "test-business-tenant-tls",
		Errors:         []string{"certificate: failed to create", "gateway: failed to patch"},
		Success:        false,
	}

	if result.Success {
		t.Error("expected success to be false")
	}

	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
}
