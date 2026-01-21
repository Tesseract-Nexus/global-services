package services

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"custom-domain-service/internal/config"
	"custom-domain-service/internal/models"

	"github.com/rs/zerolog/log"
)

// DNSVerifier handles DNS record verification
type DNSVerifier struct {
	cfg      *config.Config
	resolver *net.Resolver
}

// NewDNSVerifier creates a new DNS verifier
func NewDNSVerifier(cfg *config.Config) *DNSVerifier {
	// Use the system's default DNS resolver (CoreDNS in Kubernetes)
	// This works within cluster network policies unlike hardcoded external DNS
	return &DNSVerifier{
		cfg: cfg,
		resolver: &net.Resolver{
			PreferGo: true,
			// Use default dialer which respects /etc/resolv.conf (CoreDNS in K8s)
		},
	}
}

// VerificationResult contains the result of DNS verification
type VerificationResult struct {
	IsVerified     bool
	RecordFound    string
	ExpectedRecord string
	Message        string
	CheckedAt      time.Time
}

// VerifyDomain verifies DNS configuration for a domain
func (v *DNSVerifier) VerifyDomain(ctx context.Context, domain *models.CustomDomain) (*VerificationResult, error) {
	result := &VerificationResult{
		CheckedAt: time.Now(),
	}

	switch domain.VerificationMethod {
	case models.VerificationMethodTXT:
		return v.verifyTXTRecord(ctx, domain, result)
	case models.VerificationMethodCNAME:
		return v.verifyCNAMERecord(ctx, domain, result)
	default:
		return v.verifyTXTRecord(ctx, domain, result)
	}
}

// verifyTXTRecord verifies TXT record for domain ownership
func (v *DNSVerifier) verifyTXTRecord(ctx context.Context, domain *models.CustomDomain, result *VerificationResult) (*VerificationResult, error) {
	// Expected TXT record format: _tesserix-verification.example.com TXT "tesserix-verify=<token>"
	verificationHost := "_tesserix-verification." + domain.Domain
	expectedValue := "tesserix-verify=" + domain.VerificationToken

	result.ExpectedRecord = expectedValue

	txtRecords, err := v.resolver.LookupTXT(ctx, verificationHost)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok {
			if dnsErr.IsNotFound {
				result.Message = fmt.Sprintf("TXT record not found at %s. Please add the verification record.", verificationHost)
				return result, nil
			}
		}
		log.Warn().Err(err).Str("host", verificationHost).Msg("DNS lookup failed")
		result.Message = "DNS lookup failed. Please try again later."
		return result, nil
	}

	for _, txt := range txtRecords {
		if strings.TrimSpace(txt) == expectedValue {
			result.IsVerified = true
			result.RecordFound = txt
			result.Message = "Domain ownership verified successfully"
			return result, nil
		}
	}

	if len(txtRecords) > 0 {
		result.RecordFound = txtRecords[0]
		result.Message = fmt.Sprintf("TXT record found but value doesn't match. Expected: %s", expectedValue)
	} else {
		result.Message = "No TXT records found at verification host"
	}

	return result, nil
}

// verifyCNAMERecord verifies CNAME record pointing to proxy domain or tenant subdomain
func (v *DNSVerifier) verifyCNAMERecord(ctx context.Context, domain *models.CustomDomain, result *VerificationResult) (*VerificationResult, error) {
	// Build list of acceptable CNAME targets
	acceptableTargets := []string{v.cfg.DNS.ProxyDomain}

	// Also accept CNAME to tenant subdomain (e.g., oh-my-god.tesserix.app)
	if domain.TenantSlug != "" && v.cfg.DNS.PlatformDomain != "" {
		tenantSubdomain := domain.TenantSlug + "." + v.cfg.DNS.PlatformDomain
		acceptableTargets = append(acceptableTargets, tenantSubdomain)
	}

	result.ExpectedRecord = strings.Join(acceptableTargets, " or ")

	cname, err := v.resolver.LookupCNAME(ctx, domain.Domain)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok {
			if dnsErr.IsNotFound {
				result.Message = fmt.Sprintf("CNAME record not found. Please add CNAME pointing to %s", acceptableTargets[0])
				return result, nil
			}
		}
		log.Warn().Err(err).Str("domain", domain.Domain).Msg("CNAME lookup failed")
		result.Message = "DNS lookup failed. Please try again later."
		return result, nil
	}

	// Remove trailing dot from CNAME
	cname = strings.TrimSuffix(cname, ".")
	result.RecordFound = cname

	// Check if CNAME matches any acceptable target
	for _, target := range acceptableTargets {
		if strings.EqualFold(cname, target) || strings.EqualFold(cname, target+".") {
			result.IsVerified = true
			result.Message = "CNAME record verified successfully"
			return result, nil
		}
	}

	result.Message = fmt.Sprintf("CNAME record found (%s) but doesn't point to an accepted target: %s", cname, strings.Join(acceptableTargets, " or "))
	return result, nil
}

// CheckARecord checks if an A record points to our proxy IP
func (v *DNSVerifier) CheckARecord(ctx context.Context, domain string) (bool, []string, error) {
	ips, err := v.resolver.LookupIP(ctx, "ip4", domain)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound {
			return false, nil, nil
		}
		return false, nil, err
	}

	var ipStrings []string
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}

	expectedIP := v.cfg.DNS.ProxyIP
	if expectedIP == "" {
		return len(ips) > 0, ipStrings, nil
	}

	for _, ip := range ips {
		if ip.String() == expectedIP {
			return true, ipStrings, nil
		}
	}

	return false, ipStrings, nil
}

// GetRequiredDNSRecords returns the DNS records needed for domain setup
func (v *DNSVerifier) GetRequiredDNSRecords(domain *models.CustomDomain) []models.DNSRecord {
	records := []models.DNSRecord{}

	// Verification record
	if domain.VerificationMethod == models.VerificationMethodTXT {
		records = append(records, models.DNSRecord{
			RecordType: "TXT",
			Host:       "_tesserix-verification." + domain.Domain,
			Value:      "tesserix-verify=" + domain.VerificationToken,
			TTL:        300,
			Purpose:    "verification",
			IsVerified: domain.DNSVerified,
		})
	}

	// Determine CNAME target - prefer tenant subdomain if available
	cnameTarget := v.cfg.DNS.ProxyDomain
	if domain.TenantSlug != "" && v.cfg.DNS.PlatformDomain != "" {
		cnameTarget = domain.TenantSlug + "." + v.cfg.DNS.PlatformDomain
	}

	// Routing record - CNAME preferred for all domain types
	// Modern DNS providers (Cloudflare, Route53, etc.) support CNAME flattening for apex domains
	if domain.DomainType == models.DomainTypeApex {
		// For apex domains: offer CNAME (preferred) with A record as fallback
		// CNAME to tenant subdomain (recommended - works with Cloudflare, Route53 ALIAS, etc.)
		records = append(records, models.DNSRecord{
			RecordType: "CNAME",
			Host:       domain.Domain,
			Value:      cnameTarget,
			TTL:        300,
			Purpose:    "routing",
			IsVerified: false,
		})
		// A record fallback (if DNS provider doesn't support CNAME flattening)
		if v.cfg.DNS.ProxyIP != "" {
			records = append(records, models.DNSRecord{
				RecordType: "A",
				Host:       domain.Domain,
				Value:      v.cfg.DNS.ProxyIP,
				TTL:        300,
				Purpose:    "routing_fallback",
				IsVerified: false,
			})
		}
	} else {
		// Subdomains can always use CNAME
		records = append(records, models.DNSRecord{
			RecordType: "CNAME",
			Host:       domain.Domain,
			Value:      cnameTarget,
			TTL:        300,
			Purpose:    "routing",
			IsVerified: false,
		})
	}

	// WWW record if enabled (always CNAME)
	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		records = append(records, models.DNSRecord{
			RecordType: "CNAME",
			Host:       "www." + domain.Domain,
			Value:      cnameTarget,
			TTL:        300,
			Purpose:    "routing",
			IsVerified: false,
		})
	}

	return records
}

// DetectDomainType determines if domain is apex or subdomain
func (v *DNSVerifier) DetectDomainType(domain string) models.DomainType {
	parts := strings.Split(domain, ".")
	// If domain has more than 2 parts (e.g., shop.example.com), it's a subdomain
	// Exception for common TLDs like co.uk, com.au, etc.
	if len(parts) > 2 {
		// Check for common two-part TLDs
		twoPartTLDs := map[string]bool{
			"co.uk": true, "com.au": true, "co.in": true, "co.nz": true,
			"com.br": true, "com.mx": true, "co.za": true, "com.sg": true,
		}
		lastTwo := parts[len(parts)-2] + "." + parts[len(parts)-1]
		if twoPartTLDs[lastTwo] && len(parts) == 3 {
			return models.DomainTypeApex
		}
		return models.DomainTypeSubdomain
	}
	return models.DomainTypeApex
}

// ValidateDomainFormat validates domain name format
func (v *DNSVerifier) ValidateDomainFormat(domain string) error {
	if len(domain) == 0 {
		return fmt.Errorf("domain cannot be empty")
	}

	if len(domain) > 253 {
		return fmt.Errorf("domain exceeds maximum length of 253 characters")
	}

	// Check for valid characters
	domain = strings.ToLower(domain)
	for i, r := range domain {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.') {
			return fmt.Errorf("invalid character '%c' at position %d", r, i)
		}
	}

	// Check parts
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return fmt.Errorf("domain must have at least two parts")
	}

	for _, part := range parts {
		if len(part) == 0 {
			return fmt.Errorf("domain parts cannot be empty")
		}
		if len(part) > 63 {
			return fmt.Errorf("domain label exceeds maximum length of 63 characters")
		}
		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return fmt.Errorf("domain labels cannot start or end with hyphen")
		}
	}

	// Check it's not our platform domain
	if strings.HasSuffix(domain, ".tesserix.app") || domain == "tesserix.app" {
		return fmt.Errorf("cannot use platform domain")
	}

	return nil
}

// CheckDomainExists verifies if a domain is registered by checking for NS records
// This is a quick check with a short timeout to avoid hanging
func (v *DNSVerifier) CheckDomainExists(ctx context.Context, domainName string) (bool, error) {
	// Use a short timeout for this check
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Extract the base domain (apex) for NS lookup
	// e.g., "shop.example.com" -> "example.com"
	baseDomain := v.getBaseDomain(domainName)

	// Use the system's default DNS resolver (CoreDNS in Kubernetes)
	// instead of the custom resolver which uses hardcoded Google DNS
	// This allows the lookup to work within cluster network policies
	systemResolver := &net.Resolver{
		PreferGo: true,
	}

	// Look up NS records for the base domain
	ns, err := systemResolver.LookupNS(checkCtx, baseDomain)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok {
			// NXDOMAIN means domain definitely doesn't exist
			if dnsErr.IsNotFound {
				log.Info().Str("domain", baseDomain).Msg("Domain not found (NXDOMAIN)")
				return false, nil
			}
			// Timeout - could be network issue, log but don't fail user
			if dnsErr.IsTimeout {
				log.Warn().Str("domain", baseDomain).Msg("DNS lookup timed out, assuming domain exists")
				return true, nil
			}
		}
		// For other errors, log and assume exists to not block user
		log.Warn().Err(err).Str("domain", baseDomain).Msg("NS lookup failed, assuming domain exists")
		return true, nil
	}

	// Domain exists if it has NS records
	return len(ns) > 0, nil
}

// getBaseDomain extracts the registrable domain from a full domain name
func (v *DNSVerifier) getBaseDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain
	}

	// Handle common two-part TLDs
	twoPartTLDs := map[string]bool{
		"co.uk": true, "com.au": true, "co.in": true, "co.nz": true,
		"com.br": true, "com.mx": true, "co.za": true, "com.sg": true,
		"org.uk": true, "net.au": true, "com.cn": true, "co.jp": true,
	}

	lastTwo := parts[len(parts)-2] + "." + parts[len(parts)-1]
	if twoPartTLDs[lastTwo] {
		// For two-part TLDs, include the third part
		if len(parts) >= 3 {
			return parts[len(parts)-3] + "." + lastTwo
		}
		return domain
	}

	// For standard TLDs, return last two parts
	return parts[len(parts)-2] + "." + parts[len(parts)-1]
}

// IsRoutingConfigured checks if DNS is properly configured for routing
func (v *DNSVerifier) IsRoutingConfigured(ctx context.Context, domain *models.CustomDomain) (bool, string, error) {
	if domain.DomainType == models.DomainTypeApex {
		// Check A record
		valid, ips, err := v.CheckARecord(ctx, domain.Domain)
		if err != nil {
			return false, "", err
		}
		if !valid {
			return false, fmt.Sprintf("A record not pointing to %s. Found: %v", v.cfg.DNS.ProxyIP, ips), nil
		}
	} else {
		// Check CNAME
		result := &VerificationResult{}
		_, err := v.verifyCNAMERecord(ctx, domain, result)
		if err != nil {
			return false, "", err
		}
		if !result.IsVerified {
			return false, result.Message, nil
		}
	}

	return true, "Routing DNS is configured correctly", nil
}
