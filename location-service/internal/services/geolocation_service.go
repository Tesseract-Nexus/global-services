package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// LocationData represents detected location information (inline to avoid circular import)
type LocationData struct {
	Ip          string   `json:"ip"`
	Country     string   `json:"country"`
	CountryName string   `json:"country_name"`
	CallingCode string   `json:"calling_code"`
	FlagEmoji   string   `json:"flag_emoji"`
	State       *string  `json:"state,omitempty"`
	StateName   *string  `json:"state_name,omitempty"`
	City        *string  `json:"city,omitempty"`
	PostalCode  *string  `json:"postal_code,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	Timezone    string   `json:"timezone"`
	Currency    string   `json:"currency"`
	Locale      *string  `json:"locale,omitempty"`
}

// ipAPIResponse represents the response from ip-api.com
type ipAPIResponse struct {
	Status      string  `json:"status"`
	Message     string  `json:"message,omitempty"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
	Zip         string  `json:"zip"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Timezone    string  `json:"timezone"`
	ISP         string  `json:"isp"`
	Query       string  `json:"query"`
}

// GeoLocationService handles IP-based location detection
type GeoLocationService struct {
	provider   string // "mock", "maxmind", "ipapi", "ip-api"
	httpClient *http.Client
}

// NewGeoLocationService creates a new geolocation service
func NewGeoLocationService() *GeoLocationService {
	return &GeoLocationService{
		provider: "mock", // Default to mock for now
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewGeoLocationServiceWithProvider creates a new geolocation service with specified provider
func NewGeoLocationServiceWithProvider(provider string) *GeoLocationService {
	return &GeoLocationService{
		provider: provider,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DetectLocationFromIP detects location based on IP address
func (s *GeoLocationService) DetectLocationFromIP(ctx context.Context, ip string) (*LocationData, error) {
	// For private IPs, return default location
	if s.isPrivateIP(ip) {
		return s.getDefaultLocation(ip), nil
	}

	// Use the configured provider
	switch s.provider {
	case "ip-api":
		return s.detectFromIPAPI(ctx, ip)
	case "ipapi":
		// ipapi.co provider - can add support later
		log.Printf("ipapi provider not implemented, falling back to ip-api")
		return s.detectFromIPAPI(ctx, ip)
	case "maxmind":
		// MaxMind provider - requires license
		log.Printf("maxmind provider not implemented, falling back to ip-api")
		return s.detectFromIPAPI(ctx, ip)
	default:
		// Mock provider for development
		return s.detectFromMock(ip)
	}
}

// detectFromIPAPI uses ip-api.com for geolocation (free, accurate, 45 req/min limit)
func (s *GeoLocationService) detectFromIPAPI(ctx context.Context, ip string) (*LocationData, error) {
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode,region,regionName,city,zip,lat,lon,timezone,isp,query", ip)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("Error creating request for ip-api.com: %v", err)
		return s.detectFromMock(ip)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("Error calling ip-api.com: %v, falling back to mock", err)
		return s.detectFromMock(ip)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ip-api.com returned status %d, falling back to mock", resp.StatusCode)
		return s.detectFromMock(ip)
	}

	var apiResp ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Printf("Error decoding ip-api.com response: %v, falling back to mock", err)
		return s.detectFromMock(ip)
	}

	if apiResp.Status != "success" {
		log.Printf("ip-api.com lookup failed: %s, falling back to mock", apiResp.Message)
		return s.detectFromMock(ip)
	}

	// Map the response to LocationData
	return s.mapIPAPIResponse(&apiResp), nil
}

// mapIPAPIResponse converts ip-api.com response to LocationData
func (s *GeoLocationService) mapIPAPIResponse(resp *ipAPIResponse) *LocationData {
	// Get country metadata
	countryData := getCountryMetadata(resp.CountryCode)

	state := fmt.Sprintf("%s-%s", resp.CountryCode, resp.Region)
	locale := fmt.Sprintf("en_%s", resp.CountryCode)

	return &LocationData{
		Ip:          resp.Query,
		Country:     resp.CountryCode,
		CountryName: resp.Country,
		CallingCode: countryData.callingCode,
		FlagEmoji:   countryData.flagEmoji,
		State:       &state,
		StateName:   &resp.RegionName,
		City:        &resp.City,
		PostalCode:  &resp.Zip,
		Latitude:    &resp.Lat,
		Longitude:   &resp.Lon,
		Timezone:    resp.Timezone,
		Currency:    countryData.currency,
		Locale:      &locale,
	}
}

// countryMetadata holds static country data
type countryMetadata struct {
	callingCode string
	flagEmoji   string
	currency    string
}

// getCountryMetadata returns metadata for a country code
func getCountryMetadata(countryCode string) countryMetadata {
	metadata := map[string]countryMetadata{
		"US": {"+1", "🇺🇸", "USD"},
		"CA": {"+1", "🇨🇦", "CAD"},
		"GB": {"+44", "🇬🇧", "GBP"},
		"AU": {"+61", "🇦🇺", "AUD"},
		"IN": {"+91", "🇮🇳", "INR"},
		"SG": {"+65", "🇸🇬", "SGD"},
		"AE": {"+971", "🇦🇪", "AED"},
		"DE": {"+49", "🇩🇪", "EUR"},
		"FR": {"+33", "🇫🇷", "EUR"},
		"JP": {"+81", "🇯🇵", "JPY"},
		"CN": {"+86", "🇨🇳", "CNY"},
		"NZ": {"+64", "🇳🇿", "NZD"},
		"HK": {"+852", "🇭🇰", "HKD"},
		"MY": {"+60", "🇲🇾", "MYR"},
		"ID": {"+62", "🇮🇩", "IDR"},
		"PH": {"+63", "🇵🇭", "PHP"},
		"TH": {"+66", "🇹🇭", "THB"},
		"VN": {"+84", "🇻🇳", "VND"},
		"KR": {"+82", "🇰🇷", "KRW"},
		"BR": {"+55", "🇧🇷", "BRL"},
		"MX": {"+52", "🇲🇽", "MXN"},
		"ZA": {"+27", "🇿🇦", "ZAR"},
		"RU": {"+7", "🇷🇺", "RUB"},
		"IT": {"+39", "🇮🇹", "EUR"},
		"ES": {"+34", "🇪🇸", "EUR"},
		"NL": {"+31", "🇳🇱", "EUR"},
		"SE": {"+46", "🇸🇪", "SEK"},
		"CH": {"+41", "🇨🇭", "CHF"},
		"PL": {"+48", "🇵🇱", "PLN"},
		"AT": {"+43", "🇦🇹", "EUR"},
		"BE": {"+32", "🇧🇪", "EUR"},
		"PT": {"+351", "🇵🇹", "EUR"},
		"IE": {"+353", "🇮🇪", "EUR"},
	}

	if m, ok := metadata[countryCode]; ok {
		return m
	}
	// Default fallback
	return countryMetadata{"+1", "🏳️", "USD"}
}

// getDefaultLocation returns default location for private IPs
func (s *GeoLocationService) getDefaultLocation(ip string) *LocationData {
	locale := "en_US"
	state := "US-CA"
	stateName := "California"
	city := "San Francisco"
	postalCode := "94102"
	lat := 37.7749
	lng := -122.4194

	return &LocationData{
		Ip:          ip,
		Country:     "US",
		CountryName: "United States",
		CallingCode: "+1",
		FlagEmoji:   "🇺🇸",
		State:       &state,
		StateName:   &stateName,
		City:        &city,
		PostalCode:  &postalCode,
		Latitude:    &lat,
		Longitude:   &lng,
		Timezone:    "America/Los_Angeles",
		Currency:    "USD",
		Locale:      &locale,
	}
}

// detectFromMock returns mock location data based on IP patterns (for development)
func (s *GeoLocationService) detectFromMock(ip string) (*LocationData, error) {
	locationData := s.getDefaultLocation(ip)

	// Mock geographic detection based on IP ranges
	switch {
	case strings.HasPrefix(ip, "1.") || strings.HasPrefix(ip, "27."):
		// Mock Indian IPs
		state := "IN-MH"
		stateName := "Maharashtra"
		city := "Mumbai"
		postalCode := "400001"
		locale := "en_IN"
		lat := 19.0760
		lng := 72.8777

		locationData.Country = "IN"
		locationData.CountryName = "India"
		locationData.CallingCode = "+91"
		locationData.FlagEmoji = "🇮🇳"
		locationData.State = &state
		locationData.StateName = &stateName
		locationData.City = &city
		locationData.PostalCode = &postalCode
		locationData.Timezone = "Asia/Kolkata"
		locationData.Currency = "INR"
		locationData.Locale = &locale
		locationData.Latitude = &lat
		locationData.Longitude = &lng
	case strings.HasPrefix(ip, "2.") || strings.HasPrefix(ip, "80.") || strings.HasPrefix(ip, "81."):
		// Mock UK IPs
		city := "London"
		postalCode := "EC1A"
		locale := "en_GB"
		lat := 51.5074
		lng := -0.1278

		locationData.Country = "GB"
		locationData.CountryName = "United Kingdom"
		locationData.CallingCode = "+44"
		locationData.FlagEmoji = "🇬🇧"
		locationData.State = nil
		locationData.StateName = nil
		locationData.City = &city
		locationData.PostalCode = &postalCode
		locationData.Timezone = "Europe/London"
		locationData.Currency = "GBP"
		locationData.Locale = &locale
		locationData.Latitude = &lat
		locationData.Longitude = &lng
	case strings.HasPrefix(ip, "3.") || strings.HasPrefix(ip, "103."):
		// Mock Australian IPs
		state := "AU-NSW"
		stateName := "New South Wales"
		city := "Sydney"
		postalCode := "2000"
		locale := "en_AU"
		lat := -33.8688
		lng := 151.2093

		locationData.Country = "AU"
		locationData.CountryName = "Australia"
		locationData.CallingCode = "+61"
		locationData.FlagEmoji = "🇦🇺"
		locationData.State = &state
		locationData.StateName = &stateName
		locationData.City = &city
		locationData.PostalCode = &postalCode
		locationData.Timezone = "Australia/Sydney"
		locationData.Currency = "AUD"
		locationData.Locale = &locale
		locationData.Latitude = &lat
		locationData.Longitude = &lng
	default:
		// Default to US location (already set above)
	}

	return locationData, nil
}

// isPrivateIP checks if an IP address is private
func (s *GeoLocationService) isPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check for private IP ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
	}

	for _, privateRange := range privateRanges {
		_, cidr, err := net.ParseCIDR(privateRange)
		if err != nil {
			continue
		}
		if cidr.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// GetIPFromRequest extracts IP address from various sources
func (s *GeoLocationService) GetIPFromRequest(forwardedFor, realIP, remoteAddr string) string {
	// Check X-Forwarded-For header first
	if forwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" && !s.isPrivateIP(ip) {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	if realIP != "" && !s.isPrivateIP(realIP) {
		return realIP
	}

	// Fall back to remote address
	if remoteAddr != "" {
		// Remove port if present
		ip := remoteAddr
		if strings.Contains(ip, ":") {
			host, _, err := net.SplitHostPort(ip)
			if err == nil {
				ip = host
			}
		}
		return ip
	}

	// Default fallback
	return "8.8.8.8" // Google DNS as fallback for testing
}
