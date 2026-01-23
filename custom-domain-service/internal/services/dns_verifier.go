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

// NSDelegationResult contains the result of NS delegation verification
type NSDelegationResult struct {
	IsVerified       bool
	FoundNameservers []string
	ExpectedNS       []string
	Message          string
	CheckedAt        time.Time
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

// VerifyNSDelegation checks if _acme-challenge.{domain} NS records point to our nameservers
// This enables DNS-01 ACME challenges for automatic certificate management
func (v *DNSVerifier) VerifyNSDelegation(ctx context.Context, domain string) (*NSDelegationResult, error) {
	result := &NSDelegationResult{
		CheckedAt:  time.Now(),
		ExpectedNS: v.cfg.NSDelegation.Nameservers,
	}

	if !v.cfg.NSDelegation.Enabled {
		result.Message = "NS delegation feature is not enabled"
		return result, nil
	}

	// Check NS records for _acme-challenge.{domain}
	challengeHost := "_acme-challenge." + domain

	nsRecords, err := v.resolver.LookupNS(ctx, challengeHost)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok {
			if dnsErr.IsNotFound {
				result.Message = fmt.Sprintf("NS records not found for %s. Please add NS records pointing to %v",
					challengeHost, v.cfg.NSDelegation.Nameservers)
				return result, nil
			}
		}
		log.Warn().Err(err).Str("host", challengeHost).Msg("NS delegation lookup failed")
		result.Message = "DNS lookup failed. Please try again later."
		return result, nil
	}

	// Extract hostnames from NS records
	foundNS := make([]string, 0, len(nsRecords))
	for _, ns := range nsRecords {
		// Remove trailing dot and convert to lowercase
		host := strings.TrimSuffix(strings.ToLower(ns.Host), ".")
		foundNS = append(foundNS, host)
	}
	result.FoundNameservers = foundNS

	// Check if any of our expected nameservers are in the NS records
	matchCount := 0
	for _, found := range foundNS {
		for _, expected := range v.cfg.NSDelegation.Nameservers {
			expectedLower := strings.ToLower(expected)
			if found == expectedLower || found == expectedLower+"." {
				matchCount++
				break
			}
		}
	}

	// Consider verified if at least one of our nameservers is in the NS records
	if matchCount > 0 {
		result.IsVerified = true
		result.Message = fmt.Sprintf("NS delegation verified. Found %d of our nameservers delegated for %s",
			matchCount, challengeHost)
		log.Info().
			Str("domain", domain).
			Strs("found_ns", foundNS).
			Int("match_count", matchCount).
			Msg("NS delegation verified successfully")
	} else {
		result.Message = fmt.Sprintf("NS records found at %s but none point to our nameservers. Found: %v, Expected: %v",
			challengeHost, foundNS, v.cfg.NSDelegation.Nameservers)
		log.Info().
			Str("domain", domain).
			Strs("found_ns", foundNS).
			Strs("expected_ns", v.cfg.NSDelegation.Nameservers).
			Msg("NS delegation not verified - nameservers don't match")
	}

	return result, nil
}

// GetNSDelegationRecords returns the NS records needed for NS delegation setup
func (v *DNSVerifier) GetNSDelegationRecords(domain string) []models.DNSRecord {
	if !v.cfg.NSDelegation.Enabled {
		return nil
	}

	records := make([]models.DNSRecord, 0, len(v.cfg.NSDelegation.Nameservers))
	challengeHost := "_acme-challenge." + domain

	for _, ns := range v.cfg.NSDelegation.Nameservers {
		records = append(records, models.DNSRecord{
			RecordType: "NS",
			Host:       challengeHost,
			Value:      ns,
			TTL:        3600,
			Purpose:    "ns_delegation",
			IsVerified: false,
		})
	}

	return records
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

// verifyCNAMERecord verifies CNAME record with unique verification token in subdomain
// Format: _tesserix-<token>.<domain> → verify.tesserix.app
func (v *DNSVerifier) verifyCNAMERecord(ctx context.Context, domain *models.CustomDomain, result *VerificationResult) (*VerificationResult, error) {
	// Short token for CNAME verification (first 8 chars of verification token)
	shortToken := ""
	if len(domain.VerificationToken) >= 8 {
		shortToken = domain.VerificationToken[:8]
	}

	// Verification CNAME subdomain with unique token
	verificationHost := "_tesserix-" + shortToken + "." + domain.Domain
	expectedTarget := "verify.tesserix.app"

	result.ExpectedRecord = fmt.Sprintf("%s CNAME %s", verificationHost, expectedTarget)

	cname, err := v.resolver.LookupCNAME(ctx, verificationHost)
	if err != nil {
		if dnsErr, ok := err.(*net.DNSError); ok {
			if dnsErr.IsNotFound {
				result.Message = fmt.Sprintf("CNAME record not found at %s. Please add: %s CNAME %s", verificationHost, verificationHost, expectedTarget)
				return result, nil
			}
		}
		log.Warn().Err(err).Str("host", verificationHost).Msg("CNAME lookup failed")
		result.Message = "DNS lookup failed. Please try again later."
		return result, nil
	}

	// Remove trailing dot from CNAME
	cname = strings.TrimSuffix(cname, ".")
	result.RecordFound = cname

	// Check if CNAME points to our verification endpoint
	if strings.EqualFold(cname, expectedTarget) || strings.EqualFold(cname, expectedTarget+".") {
		result.IsVerified = true
		result.Message = "Domain ownership verified successfully via CNAME"
		return result, nil
	}

	// Also accept legacy format pointing to proxy domain (for backward compatibility)
	legacyTargets := []string{v.cfg.DNS.ProxyDomain}
	if v.cfg.Cloudflare.Enabled && v.cfg.Cloudflare.TunnelID != "" {
		tunnelCNAME := fmt.Sprintf("%s.cfargotunnel.com", v.cfg.Cloudflare.TunnelID)
		legacyTargets = append(legacyTargets, tunnelCNAME)
	}

	for _, target := range legacyTargets {
		if strings.EqualFold(cname, target) || strings.EqualFold(cname, target+".") {
			result.IsVerified = true
			result.Message = "Domain ownership verified successfully via CNAME"
			return result, nil
		}
	}

	result.Message = fmt.Sprintf("CNAME record found at %s but points to %s instead of %s", verificationHost, cname, expectedTarget)
	return result, nil
}

// LookupCNAME performs a CNAME lookup for a given host
func (v *DNSVerifier) LookupCNAME(ctx context.Context, host string) (string, error) {
	cname, err := v.resolver.LookupCNAME(ctx, host)
	if err != nil {
		return "", err
	}
	return cname, nil
}

// VerifyTunnelCNAME verifies that the domain's CNAME points to the Cloudflare tunnel
func (v *DNSVerifier) VerifyTunnelCNAME(ctx context.Context, domain string) (bool, string, error) {
	if !v.cfg.Cloudflare.Enabled || v.cfg.Cloudflare.TunnelID == "" {
		return false, "", fmt.Errorf("cloudflare tunnel not configured")
	}

	tunnelCNAME := fmt.Sprintf("%s.cfargotunnel.com", v.cfg.Cloudflare.TunnelID)

	cname, err := v.resolver.LookupCNAME(ctx, domain)
	if err != nil {
		return false, "", err
	}

	cname = strings.TrimSuffix(cname, ".")
	if strings.EqualFold(cname, tunnelCNAME) {
		return true, cname, nil
	}

	return false, cname, nil
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
// This includes NS delegation records when enabled for automatic certificate management
func (v *DNSVerifier) GetRequiredDNSRecords(domain *models.CustomDomain) []models.DNSRecord {
	records := []models.DNSRecord{}

	// NS Delegation records for automatic SSL (recommended when enabled)
	// Customer delegates _acme-challenge.{domain} to our nameservers
	if v.cfg.NSDelegation.Enabled && domain.NSDelegationEnabled {
		challengeHost := "_acme-challenge." + domain.Domain
		for _, ns := range v.cfg.NSDelegation.Nameservers {
			records = append(records, models.DNSRecord{
				RecordType: "NS",
				Host:       challengeHost,
				Value:      ns,
				TTL:        3600,
				Purpose:    "ns_delegation (automatic SSL)",
				IsVerified: domain.NSDelegationVerified,
			})
		}
	}

	// Determine CNAME target based on configuration
	// Priority: Cloudflare Tunnel > Tenant subdomain > Proxy domain
	cnameTarget := v.cfg.DNS.ProxyDomain
	cnameDescription := "proxy domain"

	if v.cfg.Cloudflare.Enabled && v.cfg.Cloudflare.TunnelID != "" {
		// Use Cloudflare Tunnel CNAME (preferred - no verification record needed)
		cnameTarget = fmt.Sprintf("%s.cfargotunnel.com", v.cfg.Cloudflare.TunnelID)
		cnameDescription = "Cloudflare Tunnel"
	} else if domain.TenantSlug != "" && v.cfg.DNS.PlatformDomain != "" {
		cnameTarget = domain.TenantSlug + "." + v.cfg.DNS.PlatformDomain
		cnameDescription = "tenant subdomain"
	}

	// Short token for CNAME verification (first 8 chars)
	shortToken := ""
	if len(domain.VerificationToken) >= 8 {
		shortToken = domain.VerificationToken[:8]
	}

	// Verification record needed for ownership proof
	// CNAME method: _tesserix-<token>.<domain> → verify.tesserix.app
	// TXT method: _tesserix.<domain> TXT "tesserix-verify=<full-token>"
	if domain.VerificationMethod == models.VerificationMethodCNAME {
		records = append(records, models.DNSRecord{
			RecordType: "CNAME",
			Host:       "_tesserix-" + shortToken + "." + domain.Domain,
			Value:      "verify.tesserix.app",
			TTL:        300,
			Purpose:    "verification",
			IsVerified: domain.DNSVerified,
		})
	} else {
		// TXT verification
		records = append(records, models.DNSRecord{
			RecordType: "TXT",
			Host:       "_tesserix." + domain.Domain,
			Value:      "tesserix-verify=" + domain.VerificationToken,
			TTL:        300,
			Purpose:    "verification",
			IsVerified: domain.DNSVerified,
		})
	}

	// Routing record - CNAME preferred for all domain types
	// Modern DNS providers (Cloudflare, Route53, etc.) support CNAME flattening for apex domains
	if domain.DomainType == models.DomainTypeApex {
		// For apex domains: offer CNAME (preferred) with A record as fallback
		records = append(records, models.DNSRecord{
			RecordType: "CNAME",
			Host:       domain.Domain,
			Value:      cnameTarget,
			TTL:        300,
			Purpose:    fmt.Sprintf("routing (%s)", cnameDescription),
			IsVerified: domain.CloudflareDNSConfigured || domain.DNSVerified,
		})
		// A record fallback only for legacy mode (not Cloudflare Tunnel)
		if !v.cfg.Cloudflare.Enabled && v.cfg.DNS.ProxyIP != "" {
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
			Purpose:    fmt.Sprintf("routing (%s)", cnameDescription),
			IsVerified: domain.CloudflareDNSConfigured || domain.DNSVerified,
		})
	}

	// WWW record if enabled (always CNAME)
	if domain.IncludeWWW && domain.DomainType == models.DomainTypeApex {
		records = append(records, models.DNSRecord{
			RecordType: "CNAME",
			Host:       "www." + domain.Domain,
			Value:      cnameTarget,
			TTL:        300,
			Purpose:    fmt.Sprintf("routing (%s)", cnameDescription),
			IsVerified: domain.CloudflareDNSConfigured || domain.DNSVerified,
		})
	}

	return records
}

// GetTunnelCNAMETarget returns the Cloudflare tunnel CNAME target
func (v *DNSVerifier) GetTunnelCNAMETarget() string {
	if v.cfg.Cloudflare.Enabled && v.cfg.Cloudflare.TunnelID != "" {
		return fmt.Sprintf("%s.cfargotunnel.com", v.cfg.Cloudflare.TunnelID)
	}
	return ""
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

// ValidateStorefrontDomain validates that a storefront domain is a subdomain (not apex)
// Apex domains are not supported because:
// 1. Apex domains can't use CNAME records (not all DNS providers support CNAME flattening)
// 2. Subdomains are more reliable for pointing to Cloudflare Tunnel
func (v *DNSVerifier) ValidateStorefrontDomain(domain string) error {
	// First run standard format validation
	if err := v.ValidateDomainFormat(domain); err != nil {
		return err
	}

	// Check if it's an apex domain (not allowed for storefront)
	domainType := v.DetectDomainType(domain)
	if domainType == models.DomainTypeApex {
		return fmt.Errorf("apex domains are not supported for storefront. Please use a subdomain like www.%s or store.%s", domain, domain)
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
