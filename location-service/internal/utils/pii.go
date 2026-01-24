package utils

import (
	"regexp"
	"strings"
)

// PIIMasker provides utilities for masking personally identifiable information in logs
type PIIMasker struct {
	emailRegex   *regexp.Regexp
	ipv4Regex    *regexp.Regexp
	ipv6Regex    *regexp.Regexp
	phoneRegex   *regexp.Regexp
	creditCard   *regexp.Regexp
	ssnRegex     *regexp.Regexp
	coordsRegex  *regexp.Regexp
}

// NewPIIMasker creates a new PII masker instance
func NewPIIMasker() *PIIMasker {
	return &PIIMasker{
		emailRegex:   regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
		ipv4Regex:    regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		ipv6Regex:    regexp.MustCompile(`(?i)([0-9a-f]{1,4}:){7}[0-9a-f]{1,4}|([0-9a-f]{1,4}:){1,7}:|([0-9a-f]{1,4}:){1,6}:[0-9a-f]{1,4}`),
		phoneRegex:   regexp.MustCompile(`\+?[1-9]\d{1,14}|\(\d{3}\)\s?\d{3}[-.]?\d{4}|\d{3}[-.]?\d{3}[-.]?\d{4}`),
		creditCard:   regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
		ssnRegex:     regexp.MustCompile(`\b\d{3}[-]?\d{2}[-]?\d{4}\b`),
		coordsRegex:  regexp.MustCompile(`[-+]?([1-8]?\d(\.\d+)?|90(\.0+)?),\s*[-+]?(180(\.0+)?|((1[0-7]\d)|([1-9]?\d))(\.\d+)?)`),
	}
}

// MaskEmail masks an email address: john.doe@example.com -> j*****e@e****e.com
func (m *PIIMasker) MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "[MASKED_EMAIL]"
	}
	local := parts[0]
	domain := parts[1]

	maskedLocal := maskMiddle(local)
	domainParts := strings.Split(domain, ".")
	if len(domainParts) >= 2 {
		maskedDomain := maskMiddle(domainParts[0]) + "." + domainParts[len(domainParts)-1]
		return maskedLocal + "@" + maskedDomain
	}
	return maskedLocal + "@" + "[MASKED]"
}

// MaskIP masks an IP address: 192.168.1.100 -> 192.168.***.***
func (m *PIIMasker) MaskIP(ip string) string {
	if ip == "" {
		return ""
	}
	// IPv4
	if strings.Contains(ip, ".") {
		parts := strings.Split(ip, ".")
		if len(parts) == 4 {
			return parts[0] + "." + parts[1] + ".***.***"
		}
	}
	// IPv6
	if strings.Contains(ip, ":") {
		parts := strings.Split(ip, ":")
		if len(parts) >= 2 {
			return parts[0] + ":" + parts[1] + ":****:****:****:****:****:****"
		}
	}
	return "[MASKED_IP]"
}

// MaskPhone masks a phone number: +1-555-123-4567 -> +1-555-***-****
func (m *PIIMasker) MaskPhone(phone string) string {
	if phone == "" {
		return ""
	}
	// Keep first few digits visible for debugging, mask the rest
	digits := regexp.MustCompile(`\d`).FindAllStringIndex(phone, -1)
	if len(digits) < 4 {
		return "[MASKED_PHONE]"
	}
	// Mask last 6 digits
	result := []byte(phone)
	masked := 0
	for i := len(digits) - 1; i >= 0 && masked < 6; i-- {
		result[digits[i][0]] = '*'
		masked++
	}
	return string(result)
}

// MaskCoordinates masks lat/lng coordinates: 40.7128,-74.0060 -> 40.7***,-74.0***
func (m *PIIMasker) MaskCoordinates(lat, lng float64) string {
	// Only show 1 decimal place for privacy
	return "[COORDS_MASKED]"
}

// MaskAddress masks a street address
func (m *PIIMasker) MaskAddress(address string) string {
	if address == "" {
		return ""
	}
	// Mask street numbers and specific identifiers
	words := strings.Split(address, " ")
	if len(words) <= 2 {
		return "[MASKED_ADDRESS]"
	}
	// Keep city/region visible, mask street details
	return "****** " + strings.Join(words[len(words)-2:], " ")
}

// MaskString masks the middle portion of any string
func (m *PIIMasker) MaskString(s string, visibleStart, visibleEnd int) string {
	if len(s) <= visibleStart+visibleEnd {
		return strings.Repeat("*", len(s))
	}
	return s[:visibleStart] + strings.Repeat("*", len(s)-visibleStart-visibleEnd) + s[len(s)-visibleEnd:]
}

// MaskAll masks all PII patterns in a string (for log sanitization)
func (m *PIIMasker) MaskAll(text string) string {
	// Mask emails
	text = m.emailRegex.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskEmail(match)
	})

	// Mask IPv4 addresses
	text = m.ipv4Regex.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskIP(match)
	})

	// Mask IPv6 addresses
	text = m.ipv6Regex.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskIP(match)
	})

	// Mask phone numbers
	text = m.phoneRegex.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskPhone(match)
	})

	// Mask credit card numbers
	text = m.creditCard.ReplaceAllString(text, "****-****-****-****")

	// Mask SSN
	text = m.ssnRegex.ReplaceAllString(text, "***-**-****")

	// Mask coordinates
	text = m.coordsRegex.ReplaceAllString(text, "[COORDS_MASKED]")

	return text
}

// Helper function to mask the middle of a string
func maskMiddle(s string) string {
	if len(s) <= 2 {
		return strings.Repeat("*", len(s))
	}
	return string(s[0]) + strings.Repeat("*", len(s)-2) + string(s[len(s)-1])
}

// Global instance for convenience
var DefaultMasker = NewPIIMasker()

// Convenience functions using the default masker
func MaskEmail(email string) string {
	return DefaultMasker.MaskEmail(email)
}

func MaskIP(ip string) string {
	return DefaultMasker.MaskIP(ip)
}

func MaskPhone(phone string) string {
	return DefaultMasker.MaskPhone(phone)
}

func MaskAddress(address string) string {
	return DefaultMasker.MaskAddress(address)
}

func MaskAll(text string) string {
	return DefaultMasker.MaskAll(text)
}

// LogSafeIP returns a log-safe version of an IP address
func LogSafeIP(ip string) string {
	return MaskIP(ip)
}

// LogSafeAddress returns a log-safe version of an address
func LogSafeAddress(address string) string {
	return MaskAddress(address)
}
