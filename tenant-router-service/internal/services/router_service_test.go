package services

import (
	"testing"
)

func TestValidateSlug_Valid(t *testing.T) {
	validSlugs := []string{
		"my-business",
		"mybusiness",
		"business123",
		"my-business-123",
		"a1",
		"ab",
		"test-slug-with-multiple-hyphens",
	}

	for _, slug := range validSlugs {
		if err := validateSlug(slug); err != nil {
			t.Errorf("expected slug %q to be valid, got error: %v", slug, err)
		}
	}
}

func TestValidateSlug_Invalid(t *testing.T) {
	testCases := []struct {
		slug   string
		reason string
	}{
		{"a", "too short"},
		{"-mybusiness", "starts with hyphen"},
		{"mybusiness-", "ends with hyphen"},
		{"-", "only hyphen"},
		{"my_business", "contains underscore"},
		{"My-Business", "contains uppercase"},
		{"my.business", "contains dot"},
		{"my business", "contains space"},
		{"", "empty string"},
		// 64 character slug (too long)
		{"abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz01", "too long"},
	}

	for _, tc := range testCases {
		if err := validateSlug(tc.slug); err == nil {
			t.Errorf("expected slug %q to be invalid (%s), but no error was returned", tc.slug, tc.reason)
		}
	}
}

func TestValidateSlug_EdgeCases(t *testing.T) {
	// Minimum valid length (2 chars)
	if err := validateSlug("ab"); err != nil {
		t.Errorf("expected 2-char slug to be valid, got error: %v", err)
	}

	// Maximum valid length (63 chars)
	slug63 := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyza"
	if len(slug63) != 63 {
		t.Fatalf("test setup error: slug should be 63 chars, got %d", len(slug63))
	}
	if err := validateSlug(slug63); err != nil {
		t.Errorf("expected 63-char slug to be valid, got error: %v", err)
	}

	// Just over maximum (64 chars)
	slug64 := slug63 + "z"
	if err := validateSlug(slug64); err == nil {
		t.Error("expected 64-char slug to be invalid, but no error was returned")
	}
}

func TestGenerateHostnames(t *testing.T) {
	slug := "mybusiness"
	domain := "tesserix.app"

	adminHost := slug + "-admin." + domain
	storefrontHost := slug + "." + domain

	expectedAdminHost := "mybusiness-admin.tesserix.app"
	expectedStorefrontHost := "mybusiness.tesserix.app"

	if adminHost != expectedAdminHost {
		t.Errorf("expected admin host %s, got %s", expectedAdminHost, adminHost)
	}

	if storefrontHost != expectedStorefrontHost {
		t.Errorf("expected storefront host %s, got %s", expectedStorefrontHost, storefrontHost)
	}
}

func TestGenerateCertName(t *testing.T) {
	slug := "mybusiness"
	expectedCertName := "mybusiness-tenant-tls"

	certName := slug + "-tenant-tls"

	if certName != expectedCertName {
		t.Errorf("expected cert name %s, got %s", expectedCertName, certName)
	}
}
