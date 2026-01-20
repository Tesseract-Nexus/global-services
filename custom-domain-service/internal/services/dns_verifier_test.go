package services

import (
	"testing"

	"custom-domain-service/internal/config"
	"custom-domain-service/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestDNSVerifier_ValidateDomainFormat(t *testing.T) {
	cfg := &config.Config{
		DNS: config.DNSConfig{
			VerificationDomain: "tesserix.app",
			ProxyDomain:        "proxy.tesserix.app",
		},
	}
	verifier := NewDNSVerifier(cfg)

	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{
			name:    "valid apex domain",
			domain:  "example.com",
			wantErr: false,
		},
		{
			name:    "valid subdomain",
			domain:  "shop.example.com",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			domain:  "shop123.example.com",
			wantErr: false,
		},
		{
			name:    "valid with hyphen",
			domain:  "my-shop.example.com",
			wantErr: false,
		},
		{
			name:    "empty domain",
			domain:  "",
			wantErr: true,
		},
		{
			name:    "single part",
			domain:  "example",
			wantErr: true,
		},
		{
			name:    "platform domain",
			domain:  "shop.tesserix.app",
			wantErr: true,
		},
		{
			name:    "starts with hyphen",
			domain:  "-example.com",
			wantErr: true,
		},
		{
			name:    "ends with hyphen",
			domain:  "example-.com",
			wantErr: true,
		},
		{
			name:    "too long label",
			domain:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.com",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			domain:  "exam_ple.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifier.ValidateDomainFormat(tt.domain)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDNSVerifier_DetectDomainType(t *testing.T) {
	cfg := &config.Config{}
	verifier := NewDNSVerifier(cfg)

	tests := []struct {
		name     string
		domain   string
		expected models.DomainType
	}{
		{
			name:     "apex domain",
			domain:   "example.com",
			expected: models.DomainTypeApex,
		},
		{
			name:     "subdomain",
			domain:   "shop.example.com",
			expected: models.DomainTypeSubdomain,
		},
		{
			name:     "deep subdomain",
			domain:   "api.shop.example.com",
			expected: models.DomainTypeSubdomain,
		},
		{
			name:     "co.uk apex",
			domain:   "example.co.uk",
			expected: models.DomainTypeApex,
		},
		{
			name:     "co.uk subdomain",
			domain:   "shop.example.co.uk",
			expected: models.DomainTypeSubdomain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := verifier.DetectDomainType(tt.domain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDNSVerifier_GetRequiredDNSRecords(t *testing.T) {
	cfg := &config.Config{
		DNS: config.DNSConfig{
			VerificationDomain: "tesserix.app",
			ProxyDomain:        "proxy.tesserix.app",
			ProxyIP:            "1.2.3.4",
		},
	}
	verifier := NewDNSVerifier(cfg)

	t.Run("apex domain with www", func(t *testing.T) {
		domain := &models.CustomDomain{
			Domain:             "example.com",
			DomainType:         models.DomainTypeApex,
			VerificationMethod: models.VerificationMethodTXT,
			VerificationToken:  "test-token-123",
			IncludeWWW:         true,
		}

		records := verifier.GetRequiredDNSRecords(domain)

		assert.Len(t, records, 3) // TXT + A + CNAME for www

		// Check TXT record
		var txtFound bool
		for _, r := range records {
			if r.RecordType == "TXT" {
				txtFound = true
				assert.Equal(t, "_tesserix-verification.example.com", r.Host)
				assert.Equal(t, "tesserix-verify=test-token-123", r.Value)
				assert.Equal(t, "verification", r.Purpose)
			}
		}
		assert.True(t, txtFound, "TXT record should be present")

		// Check A record for apex
		var aFound bool
		for _, r := range records {
			if r.RecordType == "A" {
				aFound = true
				assert.Equal(t, "example.com", r.Host)
				assert.Equal(t, "1.2.3.4", r.Value)
			}
		}
		assert.True(t, aFound, "A record should be present for apex domain")

		// Check CNAME for www
		var wwwFound bool
		for _, r := range records {
			if r.RecordType == "CNAME" && r.Host == "www.example.com" {
				wwwFound = true
				assert.Equal(t, "proxy.tesserix.app", r.Value)
			}
		}
		assert.True(t, wwwFound, "CNAME record for www should be present")
	})

	t.Run("subdomain without www", func(t *testing.T) {
		domain := &models.CustomDomain{
			Domain:             "shop.example.com",
			DomainType:         models.DomainTypeSubdomain,
			VerificationMethod: models.VerificationMethodTXT,
			VerificationToken:  "test-token-456",
			IncludeWWW:         false,
		}

		records := verifier.GetRequiredDNSRecords(domain)

		assert.Len(t, records, 2) // TXT + CNAME

		// Check CNAME for subdomain
		var cnameFound bool
		for _, r := range records {
			if r.RecordType == "CNAME" && r.Purpose == "routing" {
				cnameFound = true
				assert.Equal(t, "shop.example.com", r.Host)
				assert.Equal(t, "proxy.tesserix.app", r.Value)
			}
		}
		assert.True(t, cnameFound, "CNAME record should be present for subdomain")
	})
}
