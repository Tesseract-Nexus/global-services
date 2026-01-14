package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"location-service/internal/models"
)

// AddressProvider defines the interface for address lookup providers
type AddressProvider interface {
	Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error)
	Geocode(ctx context.Context, address string) (*models.GeocodingResult, error)
	ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error)
	GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error)
	ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error)
}

// AutocompleteOptions contains options for address autocomplete
type AutocompleteOptions struct {
	SessionToken string              // For billing optimization
	Types        []string            // Address types to filter (address, geocode, establishment, etc.)
	Components   string              // Country restriction (e.g., "country:us|country:ca")
	Language     string              // Response language
	Location     *models.GeoLocation // Bias results toward this location
	Radius       int                 // Radius in meters for location bias
}

// AddressService handles address lookup operations
type AddressService struct {
	provider   AddressProvider
	httpClient *http.Client
}

// AddressServiceConfig holds configuration for address service providers
type AddressServiceConfig struct {
	MapboxToken      string
	GoogleAPIKey     string
	HereAPIKey       string
	LocationIQAPIKey string
	PhotonURL        string // Custom Photon URL (default: https://photon.komoot.io)
	EnableFailover   bool   // If true, creates failover chain
}

// NewAddressService creates a new address service
func NewAddressService(providerType, apiKey string) *AddressService {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	var provider AddressProvider
	switch strings.ToLower(providerType) {
	case "google":
		provider = NewGoogleAddressProvider(apiKey, httpClient)
	case "mapbox":
		provider = NewMapboxAddressProvider(apiKey, httpClient)
	case "openstreetmap", "osm", "nominatim":
		provider = NewOpenStreetMapProvider(httpClient)
	default:
		provider = NewMockAddressProvider()
	}

	return &AddressService{
		provider:   provider,
		httpClient: httpClient,
	}
}

// NewAddressServiceWithFailover creates an address service with failover chain
// Failover order: Mapbox → Photon → LocationIQ → OpenStreetMap → Google
// Google is always last (pay-per-use), free providers are prioritized
// If a provider's API key is empty, that provider is skipped (except free ones)
func NewAddressServiceWithFailover(config AddressServiceConfig) *AddressService {
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	var providers []AddressProvider
	var names []string

	// 1. Primary: Mapbox (if token provided) - Best accuracy, 100k free/month
	if config.MapboxToken != "" {
		providers = append(providers, NewMapboxAddressProvider(config.MapboxToken, httpClient))
		names = append(names, "mapbox")
	}

	// 2. Photon/Komoot (free, higher rate limits than Nominatim)
	if config.PhotonURL != "" {
		providers = append(providers, NewPhotonProviderWithURL(config.PhotonURL, httpClient))
		names = append(names, "photon")
	} else {
		providers = append(providers, NewPhotonProvider(httpClient))
		names = append(names, "photon")
	}

	// 3. LocationIQ (if API key provided) - 5k free/day
	if config.LocationIQAPIKey != "" {
		providers = append(providers, NewLocationIQProvider(config.LocationIQAPIKey, httpClient))
		names = append(names, "locationiq")
	}

	// 4. OpenStreetMap/Nominatim (free, 1 req/sec rate limit)
	providers = append(providers, NewOpenStreetMapProvider(httpClient))
	names = append(names, "openstreetmap")

	// 5. Google Maps (LAST - if API key provided) - Pay per use, best accuracy
	if config.GoogleAPIKey != "" {
		providers = append(providers, NewGoogleAddressProvider(config.GoogleAPIKey, httpClient))
		names = append(names, "google")
	}

	// If somehow no providers are available, add mock as final fallback
	if len(providers) == 0 {
		providers = append(providers, NewMockAddressProvider())
		names = append(names, "mock")
	}

	var provider AddressProvider
	if len(providers) == 1 {
		provider = providers[0]
	} else {
		provider = NewFailoverAddressProvider(providers, names)
	}

	return &AddressService{
		provider:   provider,
		httpClient: httpClient,
	}
}

// GetProvider returns the underlying address provider
// This is used by CachedAddressProvider to wrap the provider with caching
func (s *AddressService) GetProvider() AddressProvider {
	return s.provider
}

// Autocomplete returns address suggestions based on user input
func (s *AddressService) Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error) {
	if input == "" {
		return []models.AddressSuggestion{}, nil
	}
	return s.provider.Autocomplete(ctx, input, opts)
}

// Geocode converts an address to coordinates
func (s *AddressService) Geocode(ctx context.Context, address string) (*models.GeocodingResult, error) {
	if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}
	return s.provider.Geocode(ctx, address)
}

// ReverseGeocode converts coordinates to an address
func (s *AddressService) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error) {
	return s.provider.ReverseGeocode(ctx, lat, lng)
}

// GetPlaceDetails retrieves detailed information about a place
func (s *AddressService) GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error) {
	if placeID == "" {
		return nil, fmt.Errorf("placeID cannot be empty")
	}
	return s.provider.GetPlaceDetails(ctx, placeID)
}

// ValidateAddress validates and standardizes an address
func (s *AddressService) ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error) {
	if address == "" {
		return &models.AddressValidationResult{
			Valid:  false,
			Issues: []string{"Address cannot be empty"},
		}, nil
	}
	return s.provider.ValidateAddress(ctx, address)
}

// ==================== GOOGLE PLACES PROVIDER ====================

// GoogleAddressProvider implements AddressProvider using Google Places API
type GoogleAddressProvider struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewGoogleAddressProvider creates a new Google Places provider
func NewGoogleAddressProvider(apiKey string, httpClient *http.Client) *GoogleAddressProvider {
	return &GoogleAddressProvider{
		apiKey:     apiKey,
		httpClient: httpClient,
		baseURL:    "https://maps.googleapis.com/maps/api",
	}
}

// Autocomplete implements AddressProvider
func (g *GoogleAddressProvider) Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error) {
	params := url.Values{}
	params.Set("input", input)
	params.Set("key", g.apiKey)

	if opts.Types != nil && len(opts.Types) > 0 {
		params.Set("types", strings.Join(opts.Types, "|"))
	}
	if opts.Components != "" {
		params.Set("components", opts.Components)
	}
	if opts.Language != "" {
		params.Set("language", opts.Language)
	}
	if opts.SessionToken != "" {
		params.Set("sessiontoken", opts.SessionToken)
	}
	if opts.Location != nil {
		params.Set("location", fmt.Sprintf("%f,%f", opts.Location.Latitude, opts.Location.Longitude))
		if opts.Radius > 0 {
			params.Set("radius", fmt.Sprintf("%d", opts.Radius))
		}
	}

	reqURL := fmt.Sprintf("%s/place/autocomplete/json?%s", g.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Google API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Status      string `json:"status"`
		Predictions []struct {
			PlaceID              string `json:"place_id"`
			Description          string `json:"description"`
			StructuredFormatting struct {
				MainText      string `json:"main_text"`
				SecondaryText string `json:"secondary_text"`
			} `json:"structured_formatting"`
			Types             []string `json:"types"`
			MatchedSubstrings []struct {
				Offset int `json:"offset"`
				Length int `json:"length"`
			} `json:"matched_substrings"`
		} `json:"predictions"`
		ErrorMessage string `json:"error_message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "OK" && result.Status != "ZERO_RESULTS" {
		return nil, fmt.Errorf("Google API error: %s - %s", result.Status, result.ErrorMessage)
	}

	suggestions := make([]models.AddressSuggestion, len(result.Predictions))
	for i, pred := range result.Predictions {
		suggestions[i] = models.AddressSuggestion{
			PlaceID:       pred.PlaceID,
			Description:   pred.Description,
			MainText:      pred.StructuredFormatting.MainText,
			SecondaryText: pred.StructuredFormatting.SecondaryText,
			Types:         pred.Types,
		}
	}

	return suggestions, nil
}

// Geocode implements AddressProvider
func (g *GoogleAddressProvider) Geocode(ctx context.Context, address string) (*models.GeocodingResult, error) {
	params := url.Values{}
	params.Set("address", address)
	params.Set("key", g.apiKey)

	reqURL := fmt.Sprintf("%s/geocode/json?%s", g.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Google API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Status  string `json:"status"`
		Results []struct {
			FormattedAddress string `json:"formatted_address"`
			PlaceID          string `json:"place_id"`
			Geometry         struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
				Viewport struct {
					Northeast struct {
						Lat float64 `json:"lat"`
						Lng float64 `json:"lng"`
					} `json:"northeast"`
					Southwest struct {
						Lat float64 `json:"lat"`
						Lng float64 `json:"lng"`
					} `json:"southwest"`
				} `json:"viewport"`
			} `json:"geometry"`
			AddressComponents []struct {
				LongName  string   `json:"long_name"`
				ShortName string   `json:"short_name"`
				Types     []string `json:"types"`
			} `json:"address_components"`
			Types []string `json:"types"`
		} `json:"results"`
		ErrorMessage string `json:"error_message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "OK" {
		if result.Status == "ZERO_RESULTS" {
			return nil, nil
		}
		return nil, fmt.Errorf("Google API error: %s - %s", result.Status, result.ErrorMessage)
	}

	if len(result.Results) == 0 {
		return nil, nil
	}

	r := result.Results[0]
	components := make([]models.AddressComponent, len(r.AddressComponents))
	for i, comp := range r.AddressComponents {
		compType := ""
		if len(comp.Types) > 0 {
			compType = comp.Types[0]
		}
		components[i] = models.AddressComponent{
			Type:      compType,
			LongName:  comp.LongName,
			ShortName: comp.ShortName,
		}
	}

	return &models.GeocodingResult{
		FormattedAddress: r.FormattedAddress,
		PlaceID:          r.PlaceID,
		Location: models.GeoLocation{
			Latitude:  r.Geometry.Location.Lat,
			Longitude: r.Geometry.Location.Lng,
		},
		Components: components,
		Types:      r.Types,
		Viewport: &models.Viewport{
			Northeast: models.GeoLocation{
				Latitude:  r.Geometry.Viewport.Northeast.Lat,
				Longitude: r.Geometry.Viewport.Northeast.Lng,
			},
			Southwest: models.GeoLocation{
				Latitude:  r.Geometry.Viewport.Southwest.Lat,
				Longitude: r.Geometry.Viewport.Southwest.Lng,
			},
		},
	}, nil
}

// ReverseGeocode implements AddressProvider
func (g *GoogleAddressProvider) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error) {
	params := url.Values{}
	params.Set("latlng", fmt.Sprintf("%f,%f", lat, lng))
	params.Set("key", g.apiKey)

	reqURL := fmt.Sprintf("%s/geocode/json?%s", g.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Google API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Status  string `json:"status"`
		Results []struct {
			FormattedAddress  string `json:"formatted_address"`
			PlaceID           string `json:"place_id"`
			AddressComponents []struct {
				LongName  string   `json:"long_name"`
				ShortName string   `json:"short_name"`
				Types     []string `json:"types"`
			} `json:"address_components"`
			Types []string `json:"types"`
		} `json:"results"`
		ErrorMessage string `json:"error_message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "OK" {
		if result.Status == "ZERO_RESULTS" {
			return nil, nil
		}
		return nil, fmt.Errorf("Google API error: %s - %s", result.Status, result.ErrorMessage)
	}

	if len(result.Results) == 0 {
		return nil, nil
	}

	r := result.Results[0]
	components := make([]models.AddressComponent, len(r.AddressComponents))
	for i, comp := range r.AddressComponents {
		compType := ""
		if len(comp.Types) > 0 {
			compType = comp.Types[0]
		}
		components[i] = models.AddressComponent{
			Type:      compType,
			LongName:  comp.LongName,
			ShortName: comp.ShortName,
		}
	}

	return &models.ReverseGeocodingResult{
		FormattedAddress: r.FormattedAddress,
		PlaceID:          r.PlaceID,
		Components:       components,
		Types:            r.Types,
	}, nil
}

// GetPlaceDetails implements AddressProvider
func (g *GoogleAddressProvider) GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error) {
	params := url.Values{}
	params.Set("place_id", placeID)
	params.Set("fields", "formatted_address,geometry,address_components,types,place_id")
	params.Set("key", g.apiKey)

	reqURL := fmt.Sprintf("%s/place/details/json?%s", g.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Google API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Status string `json:"status"`
		Result struct {
			FormattedAddress string `json:"formatted_address"`
			PlaceID          string `json:"place_id"`
			Geometry         struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
				Viewport struct {
					Northeast struct {
						Lat float64 `json:"lat"`
						Lng float64 `json:"lng"`
					} `json:"northeast"`
					Southwest struct {
						Lat float64 `json:"lat"`
						Lng float64 `json:"lng"`
					} `json:"southwest"`
				} `json:"viewport"`
			} `json:"geometry"`
			AddressComponents []struct {
				LongName  string   `json:"long_name"`
				ShortName string   `json:"short_name"`
				Types     []string `json:"types"`
			} `json:"address_components"`
			Types []string `json:"types"`
		} `json:"result"`
		ErrorMessage string `json:"error_message"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Status != "OK" {
		return nil, fmt.Errorf("Google API error: %s - %s", result.Status, result.ErrorMessage)
	}

	r := result.Result
	components := make([]models.AddressComponent, len(r.AddressComponents))
	for i, comp := range r.AddressComponents {
		compType := ""
		if len(comp.Types) > 0 {
			compType = comp.Types[0]
		}
		components[i] = models.AddressComponent{
			Type:      compType,
			LongName:  comp.LongName,
			ShortName: comp.ShortName,
		}
	}

	return &models.GeocodingResult{
		FormattedAddress: r.FormattedAddress,
		PlaceID:          r.PlaceID,
		Location: models.GeoLocation{
			Latitude:  r.Geometry.Location.Lat,
			Longitude: r.Geometry.Location.Lng,
		},
		Components: components,
		Types:      r.Types,
		Viewport: &models.Viewport{
			Northeast: models.GeoLocation{
				Latitude:  r.Geometry.Viewport.Northeast.Lat,
				Longitude: r.Geometry.Viewport.Northeast.Lng,
			},
			Southwest: models.GeoLocation{
				Latitude:  r.Geometry.Viewport.Southwest.Lat,
				Longitude: r.Geometry.Viewport.Southwest.Lng,
			},
		},
	}, nil
}

// ValidateAddress implements AddressProvider
func (g *GoogleAddressProvider) ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error) {
	result, err := g.Geocode(ctx, address)
	if err != nil {
		return &models.AddressValidationResult{
			Valid:  false,
			Issues: []string{err.Error()},
		}, nil
	}

	if result == nil {
		return &models.AddressValidationResult{
			Valid:  false,
			Issues: []string{"Address not found"},
		}, nil
	}

	// Check for required components
	var issues []string
	hasStreet := false
	hasLocality := false
	hasCountry := false
	hasPostalCode := false

	for _, comp := range result.Components {
		switch comp.Type {
		case "street_number", "route":
			hasStreet = true
		case "locality", "sublocality":
			hasLocality = true
		case "country":
			hasCountry = true
		case "postal_code":
			hasPostalCode = true
		}
	}

	if !hasStreet {
		issues = append(issues, "Missing street address")
	}
	if !hasLocality {
		issues = append(issues, "Missing city/locality")
	}
	if !hasCountry {
		issues = append(issues, "Missing country")
	}
	if !hasPostalCode {
		issues = append(issues, "Missing postal code")
	}

	return &models.AddressValidationResult{
		Valid:            len(issues) == 0,
		FormattedAddress: result.FormattedAddress,
		Components:       result.Components,
		Location:         &result.Location,
		Deliverable:      len(issues) == 0,
		Issues:           issues,
	}, nil
}

// ==================== MAPBOX PROVIDER ====================

// MapboxAddressProvider implements AddressProvider using Mapbox Geocoding API
type MapboxAddressProvider struct {
	accessToken string
	httpClient  *http.Client
	baseURL     string
}

// NewMapboxAddressProvider creates a new Mapbox provider
func NewMapboxAddressProvider(accessToken string, httpClient *http.Client) *MapboxAddressProvider {
	return &MapboxAddressProvider{
		accessToken: accessToken,
		httpClient:  httpClient,
		baseURL:     "https://api.mapbox.com/search/geocode/v6", // Using v6 API
	}
}

// Autocomplete implements AddressProvider - Uses Mapbox Geocoding v6 Suggest endpoint
func (m *MapboxAddressProvider) Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error) {
	params := url.Values{}
	params.Set("access_token", m.accessToken)
	params.Set("q", input)
	params.Set("limit", "5")

	if opts.Types != nil && len(opts.Types) > 0 {
		params.Set("types", strings.Join(opts.Types, ","))
	}
	if opts.Language != "" {
		params.Set("language", opts.Language)
	}
	if opts.Location != nil {
		params.Set("proximity", fmt.Sprintf("%f,%f", opts.Location.Longitude, opts.Location.Latitude))
	}

	// Handle country restriction (Components format: "country:AU" or "country:au|country:us")
	if opts.Components != "" {
		// Parse country codes from components string (e.g., "country:AU" -> "AU")
		var countryCodes []string
		parts := strings.Split(opts.Components, "|")
		for _, part := range parts {
			if strings.HasPrefix(strings.ToLower(part), "country:") {
				code := strings.TrimPrefix(strings.ToLower(part), "country:")
				countryCodes = append(countryCodes, strings.ToLower(code))
			}
		}
		if len(countryCodes) > 0 {
			params.Set("country", strings.Join(countryCodes, ","))
		}
	}

	// Use v6 suggest endpoint for autocomplete
	reqURL := fmt.Sprintf("%s/forward?%s", m.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Mapbox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse Mapbox v6 response
	var result struct {
		Type     string `json:"type"`
		Features []struct {
			Type     string `json:"type"`
			ID       string `json:"id"`
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"` // [lng, lat]
			} `json:"geometry"`
			Properties struct {
				MapboxID       string `json:"mapbox_id"`
				FeatureType    string `json:"feature_type"`
				FullAddress    string `json:"full_address"`
				Name           string `json:"name"`
				NamePreferred  string `json:"name_preferred"`
				PlaceFormatted string `json:"place_formatted"`
				Context        struct {
					Postcode *struct {
						Name string `json:"name"`
					} `json:"postcode,omitempty"`
					Place *struct {
						Name string `json:"name"`
					} `json:"place,omitempty"`
					Region *struct {
						Name       string `json:"name"`
						RegionCode string `json:"region_code"`
					} `json:"region,omitempty"`
					Country *struct {
						Name        string `json:"name"`
						CountryCode string `json:"country_code"`
					} `json:"country,omitempty"`
				} `json:"context"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	suggestions := make([]models.AddressSuggestion, len(result.Features))
	for i, feature := range result.Features {
		props := feature.Properties
		ctx := props.Context

		// Build secondary text from context
		var secondaryParts []string
		if ctx.Postcode != nil && ctx.Postcode.Name != "" {
			secondaryParts = append(secondaryParts, ctx.Postcode.Name)
		}
		if ctx.Place != nil && ctx.Place.Name != "" {
			secondaryParts = append(secondaryParts, ctx.Place.Name)
		}
		if ctx.Region != nil && ctx.Region.Name != "" {
			secondaryParts = append(secondaryParts, ctx.Region.Name)
		}
		if ctx.Country != nil && ctx.Country.Name != "" {
			secondaryParts = append(secondaryParts, ctx.Country.Name)
		}
		secondaryText := strings.Join(secondaryParts, ", ")

		// Use full_address for place_id since Mapbox v6 Geocoding API doesn't have a retrieve endpoint
		// When place-details is called, we'll geocode this address to get the full components
		placeIdValue := props.FullAddress
		if placeIdValue == "" {
			placeIdValue = props.PlaceFormatted
		}
		if placeIdValue == "" {
			placeIdValue = props.Name
		}
		suggestions[i] = models.AddressSuggestion{
			PlaceID:       placeIdValue,
			Description:   props.FullAddress,
			MainText:      props.Name,
			SecondaryText: secondaryText,
			Types:         []string{props.FeatureType},
		}
	}

	return suggestions, nil
}

// Geocode implements AddressProvider - Uses Mapbox Geocoding v6 forward endpoint
func (m *MapboxAddressProvider) Geocode(ctx context.Context, address string) (*models.GeocodingResult, error) {
	params := url.Values{}
	params.Set("access_token", m.accessToken)
	params.Set("q", address)
	params.Set("limit", "1")

	reqURL := fmt.Sprintf("%s/forward?%s", m.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Mapbox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse Mapbox v6 response
	var result struct {
		Type     string `json:"type"`
		Features []struct {
			Type     string `json:"type"`
			ID       string `json:"id"`
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"` // [lng, lat]
			} `json:"geometry"`
			Properties struct {
				MapboxID       string `json:"mapbox_id"`
				FeatureType    string `json:"feature_type"`
				FullAddress    string `json:"full_address"`
				Name           string `json:"name"`
				NamePreferred  string `json:"name_preferred"`
				PlaceFormatted string `json:"place_formatted"`
				Context        struct {
					Address *struct {
						AddressNumber string `json:"address_number"`
						StreetName    string `json:"street_name"`
						Name          string `json:"name"`
					} `json:"address,omitempty"`
					Street *struct {
						Name string `json:"name"`
					} `json:"street,omitempty"`
					Postcode *struct {
						Name string `json:"name"`
					} `json:"postcode,omitempty"`
					Place *struct {
						Name string `json:"name"`
					} `json:"place,omitempty"`
					Region *struct {
						Name       string `json:"name"`
						RegionCode string `json:"region_code"`
					} `json:"region,omitempty"`
					Country *struct {
						Name        string `json:"name"`
						CountryCode string `json:"country_code"`
					} `json:"country,omitempty"`
				} `json:"context"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Features) == 0 {
		return nil, nil
	}

	feature := result.Features[0]
	props := feature.Properties
	context := props.Context

	// Build address components from v6 context structure
	components := []models.AddressComponent{}

	// Street number and route
	if context.Address != nil {
		if context.Address.AddressNumber != "" {
			components = append(components, models.AddressComponent{
				Type:      "street_number",
				LongName:  context.Address.AddressNumber,
				ShortName: context.Address.AddressNumber,
			})
		}
		if context.Address.StreetName != "" {
			components = append(components, models.AddressComponent{
				Type:      "route",
				LongName:  context.Address.StreetName,
				ShortName: context.Address.StreetName,
			})
		}
	} else if context.Street != nil {
		components = append(components, models.AddressComponent{
			Type:      "route",
			LongName:  context.Street.Name,
			ShortName: context.Street.Name,
		})
	} else if props.Name != "" {
		components = append(components, models.AddressComponent{
			Type:      "route",
			LongName:  props.Name,
			ShortName: props.Name,
		})
	}

	// Locality (city/place)
	if context.Place != nil {
		components = append(components, models.AddressComponent{
			Type:      "locality",
			LongName:  context.Place.Name,
			ShortName: context.Place.Name,
		})
	}

	// State/Region
	if context.Region != nil {
		shortCode := context.Region.RegionCode
		if shortCode == "" {
			shortCode = context.Region.Name
		}
		components = append(components, models.AddressComponent{
			Type:      "administrative_area_level_1",
			LongName:  context.Region.Name,
			ShortName: shortCode,
		})
	}

	// Postal code
	if context.Postcode != nil {
		components = append(components, models.AddressComponent{
			Type:      "postal_code",
			LongName:  context.Postcode.Name,
			ShortName: context.Postcode.Name,
		})
	}

	// Country
	if context.Country != nil {
		countryCode := context.Country.CountryCode
		if countryCode == "" {
			countryCode = context.Country.Name
		}
		components = append(components, models.AddressComponent{
			Type:      "country",
			LongName:  context.Country.Name,
			ShortName: strings.ToUpper(countryCode),
		})
	}

	// Build formatted address
	formattedAddress := props.FullAddress
	if formattedAddress == "" {
		formattedAddress = props.PlaceFormatted
	}
	if formattedAddress == "" {
		formattedAddress = props.Name
	}

	return &models.GeocodingResult{
		FormattedAddress: formattedAddress,
		PlaceID:          props.MapboxID,
		Location: models.GeoLocation{
			Latitude:  feature.Geometry.Coordinates[1],
			Longitude: feature.Geometry.Coordinates[0],
		},
		Components: components,
		Types:      []string{props.FeatureType},
	}, nil
}

// ReverseGeocode implements AddressProvider - Uses Mapbox Geocoding v6 reverse endpoint
func (m *MapboxAddressProvider) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error) {
	params := url.Values{}
	params.Set("access_token", m.accessToken)
	params.Set("longitude", fmt.Sprintf("%f", lng))
	params.Set("latitude", fmt.Sprintf("%f", lat))
	params.Set("limit", "1")

	reqURL := fmt.Sprintf("%s/reverse?%s", m.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Mapbox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse Mapbox v6 response
	var result struct {
		Type     string `json:"type"`
		Features []struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Properties struct {
				MapboxID       string `json:"mapbox_id"`
				FeatureType    string `json:"feature_type"`
				FullAddress    string `json:"full_address"`
				Name           string `json:"name"`
				PlaceFormatted string `json:"place_formatted"`
				Context        struct {
					Postcode *struct {
						Name string `json:"name"`
					} `json:"postcode,omitempty"`
					Place *struct {
						Name string `json:"name"`
					} `json:"place,omitempty"`
					Region *struct {
						Name       string `json:"name"`
						RegionCode string `json:"region_code"`
					} `json:"region,omitempty"`
					Country *struct {
						Name        string `json:"name"`
						CountryCode string `json:"country_code"`
					} `json:"country,omitempty"`
				} `json:"context"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Features) == 0 {
		return nil, nil
	}

	feature := result.Features[0]
	props := feature.Properties
	context := props.Context
	components := []models.AddressComponent{}

	// Add main name
	if props.Name != "" {
		components = append(components, models.AddressComponent{
			Type:      "route",
			LongName:  props.Name,
			ShortName: props.Name,
		})
	}

	// Locality (city/place)
	if context.Place != nil {
		components = append(components, models.AddressComponent{
			Type:      "locality",
			LongName:  context.Place.Name,
			ShortName: context.Place.Name,
		})
	}

	// State/Region
	if context.Region != nil {
		shortCode := context.Region.RegionCode
		if shortCode == "" {
			shortCode = context.Region.Name
		}
		components = append(components, models.AddressComponent{
			Type:      "administrative_area_level_1",
			LongName:  context.Region.Name,
			ShortName: shortCode,
		})
	}

	// Postal code
	if context.Postcode != nil {
		components = append(components, models.AddressComponent{
			Type:      "postal_code",
			LongName:  context.Postcode.Name,
			ShortName: context.Postcode.Name,
		})
	}

	// Country
	if context.Country != nil {
		countryCode := context.Country.CountryCode
		if countryCode == "" {
			countryCode = context.Country.Name
		}
		components = append(components, models.AddressComponent{
			Type:      "country",
			LongName:  context.Country.Name,
			ShortName: strings.ToUpper(countryCode),
		})
	}

	// Build formatted address
	formattedAddress := props.FullAddress
	if formattedAddress == "" {
		formattedAddress = props.PlaceFormatted
	}
	if formattedAddress == "" {
		formattedAddress = props.Name
	}

	return &models.ReverseGeocodingResult{
		FormattedAddress: formattedAddress,
		PlaceID:          props.MapboxID,
		Components:       components,
		Types:            []string{props.FeatureType},
	}, nil
}

// GetPlaceDetails implements AddressProvider - Uses Mapbox Geocoding v6 forward endpoint
// Note: Mapbox v6 Geocoding API doesn't have a separate retrieve endpoint.
// Instead, we use the placeID (which is the full address from autocomplete) to geocode and get full details.
func (m *MapboxAddressProvider) GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error) {
	// The placeID from v6 autocomplete is a mapbox_id which doesn't work with a retrieve endpoint
	// in the Geocoding API (retrieve is only for Search Box API).
	// For Geocoding v6, we need to use the forward endpoint with the address.
	// Since the frontend stores the description as well, we should use that.
	// As a fallback, we decode the mapbox_id or use it as a search query.

	// Try decoding the base64 mapbox_id - if it's a URN, extract useful info
	// Otherwise, use the placeID as-is for geocoding
	searchQuery := placeID

	// mapbox_ids are often base64 encoded URNs, but we'll just use geocode with the ID
	// If that fails, the frontend should pass the description instead

	params := url.Values{}
	params.Set("access_token", m.accessToken)
	params.Set("q", searchQuery)
	params.Set("limit", "1")

	// Use the v6 forward endpoint to get full address details
	retrieveURL := fmt.Sprintf("%s/forward?%s", m.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", retrieveURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Mapbox API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse Mapbox v6 retrieve response
	var result struct {
		Type     string `json:"type"`
		Features []struct {
			Type     string `json:"type"`
			ID       string `json:"id"`
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"` // [lng, lat]
			} `json:"geometry"`
			Properties struct {
				MapboxID       string `json:"mapbox_id"`
				FeatureType    string `json:"feature_type"`
				FullAddress    string `json:"full_address"`
				Name           string `json:"name"`
				NamePreferred  string `json:"name_preferred"`
				PlaceFormatted string `json:"place_formatted"`
				Context        struct {
					Address *struct {
						MapboxID      string `json:"mapbox_id"`
						AddressNumber string `json:"address_number"`
						StreetName    string `json:"street_name"`
						Name          string `json:"name"`
					} `json:"address,omitempty"`
					Street *struct {
						MapboxID string `json:"mapbox_id"`
						Name     string `json:"name"`
					} `json:"street,omitempty"`
					Postcode *struct {
						MapboxID string `json:"mapbox_id"`
						Name     string `json:"name"`
					} `json:"postcode,omitempty"`
					Place *struct {
						MapboxID string `json:"mapbox_id"`
						Name     string `json:"name"`
					} `json:"place,omitempty"`
					Region *struct {
						MapboxID       string `json:"mapbox_id"`
						Name           string `json:"name"`
						RegionCode     string `json:"region_code"`
						RegionCodeFull string `json:"region_code_full"`
					} `json:"region,omitempty"`
					Country *struct {
						MapboxID          string `json:"mapbox_id"`
						Name              string `json:"name"`
						CountryCode       string `json:"country_code"`
						CountryCodeAlpha3 string `json:"country_code_alpha_3"`
					} `json:"country,omitempty"`
				} `json:"context"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Features) == 0 {
		return nil, nil
	}

	feature := result.Features[0]
	props := feature.Properties
	context := props.Context

	// Build address components from v6 context structure
	components := []models.AddressComponent{}

	// Street number and route from address context
	if context.Address != nil {
		if context.Address.AddressNumber != "" {
			components = append(components, models.AddressComponent{
				Type:      "street_number",
				LongName:  context.Address.AddressNumber,
				ShortName: context.Address.AddressNumber,
			})
		}
		if context.Address.StreetName != "" {
			components = append(components, models.AddressComponent{
				Type:      "route",
				LongName:  context.Address.StreetName,
				ShortName: context.Address.StreetName,
			})
		}
	} else if context.Street != nil {
		components = append(components, models.AddressComponent{
			Type:      "route",
			LongName:  context.Street.Name,
			ShortName: context.Street.Name,
		})
	} else if props.Name != "" {
		components = append(components, models.AddressComponent{
			Type:      "route",
			LongName:  props.Name,
			ShortName: props.Name,
		})
	}

	// Locality (city/place)
	if context.Place != nil {
		components = append(components, models.AddressComponent{
			Type:      "locality",
			LongName:  context.Place.Name,
			ShortName: context.Place.Name,
		})
	}

	// State/Region
	if context.Region != nil {
		shortCode := context.Region.RegionCode
		if shortCode == "" {
			shortCode = context.Region.Name
		}
		components = append(components, models.AddressComponent{
			Type:      "administrative_area_level_1",
			LongName:  context.Region.Name,
			ShortName: shortCode,
		})
	}

	// Postal code
	if context.Postcode != nil {
		components = append(components, models.AddressComponent{
			Type:      "postal_code",
			LongName:  context.Postcode.Name,
			ShortName: context.Postcode.Name,
		})
	}

	// Country
	if context.Country != nil {
		countryCode := context.Country.CountryCode
		if countryCode == "" {
			countryCode = context.Country.Name
		}
		components = append(components, models.AddressComponent{
			Type:      "country",
			LongName:  context.Country.Name,
			ShortName: strings.ToUpper(countryCode),
		})
	}

	// Build formatted address
	formattedAddress := props.FullAddress
	if formattedAddress == "" {
		formattedAddress = props.PlaceFormatted
	}
	if formattedAddress == "" {
		formattedAddress = props.Name
	}

	return &models.GeocodingResult{
		FormattedAddress: formattedAddress,
		PlaceID:          placeID,
		Location: models.GeoLocation{
			Latitude:  feature.Geometry.Coordinates[1],
			Longitude: feature.Geometry.Coordinates[0],
		},
		Components: components,
		Types:      []string{props.FeatureType},
	}, nil
}

// ValidateAddress implements AddressProvider
func (m *MapboxAddressProvider) ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error) {
	result, err := m.Geocode(ctx, address)
	if err != nil {
		return &models.AddressValidationResult{
			Valid:  false,
			Issues: []string{err.Error()},
		}, nil
	}

	if result == nil {
		return &models.AddressValidationResult{
			Valid:  false,
			Issues: []string{"Address not found"},
		}, nil
	}

	return &models.AddressValidationResult{
		Valid:            true,
		FormattedAddress: result.FormattedAddress,
		Components:       result.Components,
		Location:         &result.Location,
		Deliverable:      true,
	}, nil
}

// ==================== MOCK PROVIDER ====================

// MockAddressProvider implements AddressProvider with mock data
type MockAddressProvider struct{}

// NewMockAddressProvider creates a new mock provider
func NewMockAddressProvider() *MockAddressProvider {
	return &MockAddressProvider{}
}

// Autocomplete implements AddressProvider with mock data
func (m *MockAddressProvider) Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error) {
	input = strings.ToLower(input)

	suggestions := []models.AddressSuggestion{
		{
			PlaceID:       "mock-place-1",
			Description:   "123 Main Street, San Francisco, CA 94102, USA",
			MainText:      "123 Main Street",
			SecondaryText: "San Francisco, CA 94102, USA",
			Types:         []string{"street_address"},
		},
		{
			PlaceID:       "mock-place-2",
			Description:   "456 Market Street, San Francisco, CA 94103, USA",
			MainText:      "456 Market Street",
			SecondaryText: "San Francisco, CA 94103, USA",
			Types:         []string{"street_address"},
		},
		{
			PlaceID:       "mock-place-3",
			Description:   "789 Broadway, New York, NY 10003, USA",
			MainText:      "789 Broadway",
			SecondaryText: "New York, NY 10003, USA",
			Types:         []string{"street_address"},
		},
		{
			PlaceID:       "mock-place-4",
			Description:   "100 Queen Street, Melbourne VIC 3000, Australia",
			MainText:      "100 Queen Street",
			SecondaryText: "Melbourne VIC 3000, Australia",
			Types:         []string{"street_address"},
		},
		{
			PlaceID:       "mock-place-5",
			Description:   "50 MG Road, Bangalore, Karnataka 560001, India",
			MainText:      "50 MG Road",
			SecondaryText: "Bangalore, Karnataka 560001, India",
			Types:         []string{"street_address"},
		},
	}

	// Filter based on input
	var filtered []models.AddressSuggestion
	for _, s := range suggestions {
		if strings.Contains(strings.ToLower(s.Description), input) ||
			strings.Contains(strings.ToLower(s.MainText), input) {
			filtered = append(filtered, s)
		}
	}

	// If no matches, return first 3 as suggestions
	if len(filtered) == 0 && len(input) > 0 {
		filtered = suggestions[:3]
	}

	return filtered, nil
}

// Geocode implements AddressProvider with mock data
func (m *MockAddressProvider) Geocode(ctx context.Context, address string) (*models.GeocodingResult, error) {
	// Return mock geocoding result
	return &models.GeocodingResult{
		FormattedAddress: address,
		PlaceID:          "mock-geocoded-place",
		Location: models.GeoLocation{
			Latitude:  37.7749,
			Longitude: -122.4194,
		},
		Components: []models.AddressComponent{
			{Type: "street_number", LongName: "123", ShortName: "123"},
			{Type: "route", LongName: "Main Street", ShortName: "Main St"},
			{Type: "locality", LongName: "San Francisco", ShortName: "SF"},
			{Type: "administrative_area_level_1", LongName: "California", ShortName: "CA"},
			{Type: "country", LongName: "United States", ShortName: "US"},
			{Type: "postal_code", LongName: "94102", ShortName: "94102"},
		},
		Types: []string{"street_address"},
		Viewport: &models.Viewport{
			Northeast: models.GeoLocation{Latitude: 37.7850, Longitude: -122.4094},
			Southwest: models.GeoLocation{Latitude: 37.7648, Longitude: -122.4294},
		},
	}, nil
}

// ReverseGeocode implements AddressProvider with mock data
func (m *MockAddressProvider) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error) {
	return &models.ReverseGeocodingResult{
		FormattedAddress: "123 Main Street, San Francisco, CA 94102, USA",
		PlaceID:          "mock-reverse-geocoded-place",
		Components: []models.AddressComponent{
			{Type: "street_number", LongName: "123", ShortName: "123"},
			{Type: "route", LongName: "Main Street", ShortName: "Main St"},
			{Type: "locality", LongName: "San Francisco", ShortName: "SF"},
			{Type: "administrative_area_level_1", LongName: "California", ShortName: "CA"},
			{Type: "country", LongName: "United States", ShortName: "US"},
			{Type: "postal_code", LongName: "94102", ShortName: "94102"},
		},
		Types: []string{"street_address"},
	}, nil
}

// GetPlaceDetails implements AddressProvider with mock data
func (m *MockAddressProvider) GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error) {
	return m.Geocode(ctx, placeID)
}

// ValidateAddress implements AddressProvider with mock data
func (m *MockAddressProvider) ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error) {
	if address == "" {
		return &models.AddressValidationResult{
			Valid:  false,
			Issues: []string{"Address cannot be empty"},
		}, nil
	}

	result, _ := m.Geocode(ctx, address)

	return &models.AddressValidationResult{
		Valid:            true,
		FormattedAddress: result.FormattedAddress,
		Components:       result.Components,
		Location:         &result.Location,
		Deliverable:      true,
	}, nil
}

// ==================== OPENSTREETMAP (NOMINATIM) PROVIDER ====================

// OpenStreetMapProvider implements AddressProvider using OpenStreetMap Nominatim API
// Free to use with proper attribution and rate limiting (1 request/second)
type OpenStreetMapProvider struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

// NewOpenStreetMapProvider creates a new OpenStreetMap Nominatim provider
func NewOpenStreetMapProvider(httpClient *http.Client) *OpenStreetMapProvider {
	return &OpenStreetMapProvider{
		httpClient: httpClient,
		baseURL:    "https://nominatim.openstreetmap.org",
		userAgent:  "TesseractHub/1.0 (https://tesserix.app)",
	}
}

// Autocomplete implements AddressProvider using Nominatim search
func (o *OpenStreetMapProvider) Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error) {
	params := url.Values{}
	params.Set("q", input)
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	params.Set("limit", "5")

	// Apply country restriction if specified
	if opts.Components != "" {
		// Parse "country:AU" format
		if strings.HasPrefix(opts.Components, "country:") {
			countryCode := strings.TrimPrefix(opts.Components, "country:")
			params.Set("countrycodes", strings.ToLower(countryCode))
		}
	}

	reqURL := fmt.Sprintf("%s/search?%s", o.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", o.userAgent)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Nominatim API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Nominatim API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var results []struct {
		PlaceID     int64  `json:"place_id"`
		OsmType     string `json:"osm_type"`
		OsmID       int64  `json:"osm_id"`
		DisplayName string `json:"display_name"`
		Type        string `json:"type"`
		Class       string `json:"class"`
		Address     struct {
			HouseNumber   string `json:"house_number"`
			Road          string `json:"road"`
			Suburb        string `json:"suburb"`
			City          string `json:"city"`
			Town          string `json:"town"`
			Village       string `json:"village"`
			State         string `json:"state"`
			Postcode      string `json:"postcode"`
			Country       string `json:"country"`
			CountryCode   string `json:"country_code"`
		} `json:"address"`
	}

	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	suggestions := make([]models.AddressSuggestion, 0, len(results))
	for _, r := range results {
		// Build main text and secondary text
		mainText := ""
		if r.Address.HouseNumber != "" {
			mainText = r.Address.HouseNumber + " "
		}
		if r.Address.Road != "" {
			mainText += r.Address.Road
		}
		if mainText == "" {
			parts := strings.SplitN(r.DisplayName, ",", 2)
			mainText = strings.TrimSpace(parts[0])
		}

		// Get city (could be city, town, or village)
		city := r.Address.City
		if city == "" {
			city = r.Address.Town
		}
		if city == "" {
			city = r.Address.Village
		}

		secondaryText := ""
		if city != "" {
			secondaryText = city
		}
		if r.Address.State != "" {
			if secondaryText != "" {
				secondaryText += ", "
			}
			secondaryText += r.Address.State
		}
		if r.Address.Country != "" {
			if secondaryText != "" {
				secondaryText += ", "
			}
			secondaryText += r.Address.Country
		}

		suggestions = append(suggestions, models.AddressSuggestion{
			PlaceID:       fmt.Sprintf("osm:%s:%d", r.OsmType, r.OsmID),
			Description:   r.DisplayName,
			MainText:      mainText,
			SecondaryText: secondaryText,
			Types:         []string{r.Type, r.Class},
		})
	}

	return suggestions, nil
}

// Geocode implements AddressProvider using Nominatim search
func (o *OpenStreetMapProvider) Geocode(ctx context.Context, address string) (*models.GeocodingResult, error) {
	params := url.Values{}
	params.Set("q", address)
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")
	params.Set("limit", "1")

	reqURL := fmt.Sprintf("%s/search?%s", o.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", o.userAgent)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Nominatim API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var results []struct {
		PlaceID     int64   `json:"place_id"`
		OsmType     string  `json:"osm_type"`
		OsmID       int64   `json:"osm_id"`
		Lat         string  `json:"lat"`
		Lon         string  `json:"lon"`
		DisplayName string  `json:"display_name"`
		Address     struct {
			HouseNumber string `json:"house_number"`
			Road        string `json:"road"`
			City        string `json:"city"`
			Town        string `json:"town"`
			Village     string `json:"village"`
			State       string `json:"state"`
			Postcode    string `json:"postcode"`
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
		} `json:"address"`
	}

	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found for address: %s", address)
	}

	r := results[0]
	lat := 0.0
	lng := 0.0
	fmt.Sscanf(r.Lat, "%f", &lat)
	fmt.Sscanf(r.Lon, "%f", &lng)

	// Get city (could be city, town, or village)
	city := r.Address.City
	if city == "" {
		city = r.Address.Town
	}
	if city == "" {
		city = r.Address.Village
	}

	components := []models.AddressComponent{}
	if r.Address.HouseNumber != "" {
		components = append(components, models.AddressComponent{Type: "street_number", LongName: r.Address.HouseNumber, ShortName: r.Address.HouseNumber})
	}
	if r.Address.Road != "" {
		components = append(components, models.AddressComponent{Type: "route", LongName: r.Address.Road, ShortName: r.Address.Road})
	}
	if city != "" {
		components = append(components, models.AddressComponent{Type: "locality", LongName: city, ShortName: city})
	}
	if r.Address.State != "" {
		components = append(components, models.AddressComponent{Type: "administrative_area_level_1", LongName: r.Address.State, ShortName: r.Address.State})
	}
	if r.Address.Country != "" {
		components = append(components, models.AddressComponent{Type: "country", LongName: r.Address.Country, ShortName: strings.ToUpper(r.Address.CountryCode)})
	}
	if r.Address.Postcode != "" {
		components = append(components, models.AddressComponent{Type: "postal_code", LongName: r.Address.Postcode, ShortName: r.Address.Postcode})
	}

	return &models.GeocodingResult{
		FormattedAddress: r.DisplayName,
		PlaceID:          fmt.Sprintf("osm:%s:%d", r.OsmType, r.OsmID),
		Location: models.GeoLocation{
			Latitude:  lat,
			Longitude: lng,
		},
		Components: components,
		Types:      []string{"street_address"},
	}, nil
}

// ReverseGeocode implements AddressProvider using Nominatim reverse
func (o *OpenStreetMapProvider) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error) {
	params := url.Values{}
	params.Set("lat", fmt.Sprintf("%f", lat))
	params.Set("lon", fmt.Sprintf("%f", lng))
	params.Set("format", "jsonv2")
	params.Set("addressdetails", "1")

	reqURL := fmt.Sprintf("%s/reverse?%s", o.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", o.userAgent)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Nominatim API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var r struct {
		PlaceID     int64  `json:"place_id"`
		OsmType     string `json:"osm_type"`
		OsmID       int64  `json:"osm_id"`
		DisplayName string `json:"display_name"`
		Address     struct {
			HouseNumber string `json:"house_number"`
			Road        string `json:"road"`
			City        string `json:"city"`
			Town        string `json:"town"`
			Village     string `json:"village"`
			State       string `json:"state"`
			Postcode    string `json:"postcode"`
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
		} `json:"address"`
		Error string `json:"error"`
	}

	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if r.Error != "" {
		return nil, fmt.Errorf("Nominatim error: %s", r.Error)
	}

	// Get city (could be city, town, or village)
	city := r.Address.City
	if city == "" {
		city = r.Address.Town
	}
	if city == "" {
		city = r.Address.Village
	}

	components := []models.AddressComponent{}
	if r.Address.HouseNumber != "" {
		components = append(components, models.AddressComponent{Type: "street_number", LongName: r.Address.HouseNumber, ShortName: r.Address.HouseNumber})
	}
	if r.Address.Road != "" {
		components = append(components, models.AddressComponent{Type: "route", LongName: r.Address.Road, ShortName: r.Address.Road})
	}
	if city != "" {
		components = append(components, models.AddressComponent{Type: "locality", LongName: city, ShortName: city})
	}
	if r.Address.State != "" {
		components = append(components, models.AddressComponent{Type: "administrative_area_level_1", LongName: r.Address.State, ShortName: r.Address.State})
	}
	if r.Address.Country != "" {
		components = append(components, models.AddressComponent{Type: "country", LongName: r.Address.Country, ShortName: strings.ToUpper(r.Address.CountryCode)})
	}
	if r.Address.Postcode != "" {
		components = append(components, models.AddressComponent{Type: "postal_code", LongName: r.Address.Postcode, ShortName: r.Address.Postcode})
	}

	return &models.ReverseGeocodingResult{
		FormattedAddress: r.DisplayName,
		PlaceID:          fmt.Sprintf("osm:%s:%d", r.OsmType, r.OsmID),
		Components:       components,
		Types:            []string{"street_address"},
	}, nil
}

// GetPlaceDetails implements AddressProvider - uses geocode as Nominatim doesn't have place details
func (o *OpenStreetMapProvider) GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error) {
	// For OSM place IDs, we can use lookup API
	// Format: osm:N:123456 or osm:W:123456 or osm:R:123456
	if strings.HasPrefix(placeID, "osm:") {
		parts := strings.Split(placeID, ":")
		if len(parts) == 3 {
			osmType := parts[1]
			osmID := parts[2]

			params := url.Values{}
			params.Set("osm_ids", fmt.Sprintf("%s%s", strings.ToUpper(osmType[:1]), osmID))
			params.Set("format", "jsonv2")
			params.Set("addressdetails", "1")

			reqURL := fmt.Sprintf("%s/lookup?%s", o.baseURL, params.Encode())

			req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}
			req.Header.Set("User-Agent", o.userAgent)

			resp, err := o.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("failed to call Nominatim API: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read response: %w", err)
			}

			var results []struct {
				PlaceID     int64  `json:"place_id"`
				OsmType     string `json:"osm_type"`
				OsmID       int64  `json:"osm_id"`
				Lat         string `json:"lat"`
				Lon         string `json:"lon"`
				DisplayName string `json:"display_name"`
				Address     struct {
					HouseNumber string `json:"house_number"`
					Road        string `json:"road"`
					City        string `json:"city"`
					Town        string `json:"town"`
					Village     string `json:"village"`
					State       string `json:"state"`
					Postcode    string `json:"postcode"`
					Country     string `json:"country"`
					CountryCode string `json:"country_code"`
				} `json:"address"`
			}

			if err := json.Unmarshal(body, &results); err != nil {
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}

			if len(results) > 0 {
				r := results[0]
				lat := 0.0
				lng := 0.0
				fmt.Sscanf(r.Lat, "%f", &lat)
				fmt.Sscanf(r.Lon, "%f", &lng)

				city := r.Address.City
				if city == "" {
					city = r.Address.Town
				}
				if city == "" {
					city = r.Address.Village
				}

				components := []models.AddressComponent{}
				if r.Address.HouseNumber != "" {
					components = append(components, models.AddressComponent{Type: "street_number", LongName: r.Address.HouseNumber, ShortName: r.Address.HouseNumber})
				}
				if r.Address.Road != "" {
					components = append(components, models.AddressComponent{Type: "route", LongName: r.Address.Road, ShortName: r.Address.Road})
				}
				if city != "" {
					components = append(components, models.AddressComponent{Type: "locality", LongName: city, ShortName: city})
				}
				if r.Address.State != "" {
					components = append(components, models.AddressComponent{Type: "administrative_area_level_1", LongName: r.Address.State, ShortName: r.Address.State})
				}
				if r.Address.Country != "" {
					components = append(components, models.AddressComponent{Type: "country", LongName: r.Address.Country, ShortName: strings.ToUpper(r.Address.CountryCode)})
				}
				if r.Address.Postcode != "" {
					components = append(components, models.AddressComponent{Type: "postal_code", LongName: r.Address.Postcode, ShortName: r.Address.Postcode})
				}

				return &models.GeocodingResult{
					FormattedAddress: r.DisplayName,
					PlaceID:          placeID,
					Location: models.GeoLocation{
						Latitude:  lat,
						Longitude: lng,
					},
					Components: components,
					Types:      []string{"street_address"},
				}, nil
			}
		}
	}

	// Fallback: treat placeID as an address and geocode it
	return o.Geocode(ctx, placeID)
}

// ValidateAddress implements AddressProvider - geocodes and checks result
func (o *OpenStreetMapProvider) ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error) {
	result, err := o.Geocode(ctx, address)
	if err != nil {
		return &models.AddressValidationResult{
			Valid:  false,
			Issues: []string{err.Error()},
		}, nil
	}

	return &models.AddressValidationResult{
		Valid:            true,
		FormattedAddress: result.FormattedAddress,
		Components:       result.Components,
		Location:         &result.Location,
		Deliverable:      true,
	}, nil
}

// ==================== FAILOVER PROVIDER ====================

// FailoverAddressProvider wraps multiple providers and tries them in order
type FailoverAddressProvider struct {
	providers []AddressProvider
	names     []string
}

// NewFailoverAddressProvider creates a failover provider with multiple backends
// Providers are tried in order until one succeeds
func NewFailoverAddressProvider(providers []AddressProvider, names []string) *FailoverAddressProvider {
	return &FailoverAddressProvider{
		providers: providers,
		names:     names,
	}
}

// Autocomplete tries each provider in order until one succeeds
func (f *FailoverAddressProvider) Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error) {
	var lastErr error
	for i, provider := range f.providers {
		results, err := provider.Autocomplete(ctx, input, opts)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", f.names[i], err)
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	return []models.AddressSuggestion{}, nil
}

// Geocode tries each provider in order until one succeeds
func (f *FailoverAddressProvider) Geocode(ctx context.Context, address string) (*models.GeocodingResult, error) {
	var lastErr error
	for i, provider := range f.providers {
		result, err := provider.Geocode(ctx, address)
		if err == nil {
			return result, nil
		}
		lastErr = fmt.Errorf("%s: %w", f.names[i], err)
	}
	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// ReverseGeocode tries each provider in order until one succeeds
func (f *FailoverAddressProvider) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error) {
	var lastErr error
	for i, provider := range f.providers {
		result, err := provider.ReverseGeocode(ctx, lat, lng)
		if err == nil {
			return result, nil
		}
		lastErr = fmt.Errorf("%s: %w", f.names[i], err)
	}
	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// GetPlaceDetails tries each provider in order until one succeeds
func (f *FailoverAddressProvider) GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error) {
	var lastErr error
	for i, provider := range f.providers {
		result, err := provider.GetPlaceDetails(ctx, placeID)
		if err == nil {
			return result, nil
		}
		lastErr = fmt.Errorf("%s: %w", f.names[i], err)
	}
	return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// ValidateAddress tries each provider in order until one succeeds
func (f *FailoverAddressProvider) ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error) {
	var lastErr error
	for i, provider := range f.providers {
		result, err := provider.ValidateAddress(ctx, address)
		if err == nil && result.Valid {
			return result, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", f.names[i], err)
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("all providers failed, last error: %w", lastErr)
	}
	return &models.AddressValidationResult{
		Valid:  false,
		Issues: []string{"Address could not be validated by any provider"},
	}, nil
}

// ==================== LOCATIONIQ PROVIDER ====================

// LocationIQProvider implements AddressProvider using LocationIQ API
// Free tier: 5,000 requests/day, paid plans for higher volume
type LocationIQProvider struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewLocationIQProvider creates a new LocationIQ provider
func NewLocationIQProvider(apiKey string, httpClient *http.Client) *LocationIQProvider {
	return &LocationIQProvider{
		apiKey:     apiKey,
		httpClient: httpClient,
		baseURL:    "https://us1.locationiq.com/v1",
	}
}

// Autocomplete implements AddressProvider using LocationIQ autocomplete
func (l *LocationIQProvider) Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error) {
	params := url.Values{}
	params.Set("key", l.apiKey)
	params.Set("q", input)
	params.Set("format", "json")
	params.Set("addressdetails", "1")
	params.Set("limit", "5")

	// Apply country restriction if specified
	if opts.Components != "" {
		if strings.HasPrefix(opts.Components, "country:") {
			countryCode := strings.TrimPrefix(opts.Components, "country:")
			params.Set("countrycodes", strings.ToLower(countryCode))
		}
	}

	reqURL := fmt.Sprintf("%s/autocomplete?%s", l.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call LocationIQ API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LocationIQ API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var results []struct {
		PlaceID     string `json:"place_id"`
		OsmType     string `json:"osm_type"`
		OsmID       string `json:"osm_id"`
		DisplayName string `json:"display_name"`
		Type        string `json:"type"`
		Class       string `json:"class"`
		Address     struct {
			HouseNumber string `json:"house_number"`
			Road        string `json:"road"`
			City        string `json:"city"`
			Town        string `json:"town"`
			Village     string `json:"village"`
			State       string `json:"state"`
			Postcode    string `json:"postcode"`
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
		} `json:"address"`
	}

	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	suggestions := make([]models.AddressSuggestion, 0, len(results))
	for _, r := range results {
		mainText := ""
		if r.Address.HouseNumber != "" {
			mainText = r.Address.HouseNumber + " "
		}
		if r.Address.Road != "" {
			mainText += r.Address.Road
		}
		if mainText == "" {
			parts := strings.SplitN(r.DisplayName, ",", 2)
			mainText = strings.TrimSpace(parts[0])
		}

		city := r.Address.City
		if city == "" {
			city = r.Address.Town
		}
		if city == "" {
			city = r.Address.Village
		}

		secondaryText := ""
		if city != "" {
			secondaryText = city
		}
		if r.Address.State != "" {
			if secondaryText != "" {
				secondaryText += ", "
			}
			secondaryText += r.Address.State
		}
		if r.Address.Country != "" {
			if secondaryText != "" {
				secondaryText += ", "
			}
			secondaryText += r.Address.Country
		}

		suggestions = append(suggestions, models.AddressSuggestion{
			PlaceID:       fmt.Sprintf("liq:%s", r.PlaceID),
			Description:   r.DisplayName,
			MainText:      mainText,
			SecondaryText: secondaryText,
			Types:         []string{r.Type, r.Class},
		})
	}

	return suggestions, nil
}

// Geocode implements AddressProvider using LocationIQ search
func (l *LocationIQProvider) Geocode(ctx context.Context, address string) (*models.GeocodingResult, error) {
	params := url.Values{}
	params.Set("key", l.apiKey)
	params.Set("q", address)
	params.Set("format", "json")
	params.Set("addressdetails", "1")
	params.Set("limit", "1")

	reqURL := fmt.Sprintf("%s/search?%s", l.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call LocationIQ API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var results []struct {
		PlaceID     string `json:"place_id"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
		Address     struct {
			HouseNumber string `json:"house_number"`
			Road        string `json:"road"`
			City        string `json:"city"`
			Town        string `json:"town"`
			Village     string `json:"village"`
			State       string `json:"state"`
			Postcode    string `json:"postcode"`
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
		} `json:"address"`
	}

	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found for address: %s", address)
	}

	r := results[0]
	lat := 0.0
	lng := 0.0
	fmt.Sscanf(r.Lat, "%f", &lat)
	fmt.Sscanf(r.Lon, "%f", &lng)

	city := r.Address.City
	if city == "" {
		city = r.Address.Town
	}
	if city == "" {
		city = r.Address.Village
	}

	components := []models.AddressComponent{}
	if r.Address.HouseNumber != "" {
		components = append(components, models.AddressComponent{Type: "street_number", LongName: r.Address.HouseNumber, ShortName: r.Address.HouseNumber})
	}
	if r.Address.Road != "" {
		components = append(components, models.AddressComponent{Type: "route", LongName: r.Address.Road, ShortName: r.Address.Road})
	}
	if city != "" {
		components = append(components, models.AddressComponent{Type: "locality", LongName: city, ShortName: city})
	}
	if r.Address.State != "" {
		components = append(components, models.AddressComponent{Type: "administrative_area_level_1", LongName: r.Address.State, ShortName: r.Address.State})
	}
	if r.Address.Country != "" {
		components = append(components, models.AddressComponent{Type: "country", LongName: r.Address.Country, ShortName: strings.ToUpper(r.Address.CountryCode)})
	}
	if r.Address.Postcode != "" {
		components = append(components, models.AddressComponent{Type: "postal_code", LongName: r.Address.Postcode, ShortName: r.Address.Postcode})
	}

	return &models.GeocodingResult{
		FormattedAddress: r.DisplayName,
		PlaceID:          fmt.Sprintf("liq:%s", r.PlaceID),
		Location: models.GeoLocation{
			Latitude:  lat,
			Longitude: lng,
		},
		Components: components,
		Types:      []string{"street_address"},
	}, nil
}

// ReverseGeocode implements AddressProvider using LocationIQ reverse
func (l *LocationIQProvider) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error) {
	params := url.Values{}
	params.Set("key", l.apiKey)
	params.Set("lat", fmt.Sprintf("%f", lat))
	params.Set("lon", fmt.Sprintf("%f", lng))
	params.Set("format", "json")
	params.Set("addressdetails", "1")

	reqURL := fmt.Sprintf("%s/reverse?%s", l.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call LocationIQ API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var r struct {
		PlaceID     string `json:"place_id"`
		DisplayName string `json:"display_name"`
		Address     struct {
			HouseNumber string `json:"house_number"`
			Road        string `json:"road"`
			City        string `json:"city"`
			Town        string `json:"town"`
			Village     string `json:"village"`
			State       string `json:"state"`
			Postcode    string `json:"postcode"`
			Country     string `json:"country"`
			CountryCode string `json:"country_code"`
		} `json:"address"`
		Error string `json:"error"`
	}

	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if r.Error != "" {
		return nil, fmt.Errorf("LocationIQ error: %s", r.Error)
	}

	city := r.Address.City
	if city == "" {
		city = r.Address.Town
	}
	if city == "" {
		city = r.Address.Village
	}

	components := []models.AddressComponent{}
	if r.Address.HouseNumber != "" {
		components = append(components, models.AddressComponent{Type: "street_number", LongName: r.Address.HouseNumber, ShortName: r.Address.HouseNumber})
	}
	if r.Address.Road != "" {
		components = append(components, models.AddressComponent{Type: "route", LongName: r.Address.Road, ShortName: r.Address.Road})
	}
	if city != "" {
		components = append(components, models.AddressComponent{Type: "locality", LongName: city, ShortName: city})
	}
	if r.Address.State != "" {
		components = append(components, models.AddressComponent{Type: "administrative_area_level_1", LongName: r.Address.State, ShortName: r.Address.State})
	}
	if r.Address.Country != "" {
		components = append(components, models.AddressComponent{Type: "country", LongName: r.Address.Country, ShortName: strings.ToUpper(r.Address.CountryCode)})
	}
	if r.Address.Postcode != "" {
		components = append(components, models.AddressComponent{Type: "postal_code", LongName: r.Address.Postcode, ShortName: r.Address.Postcode})
	}

	return &models.ReverseGeocodingResult{
		FormattedAddress: r.DisplayName,
		PlaceID:          fmt.Sprintf("liq:%s", r.PlaceID),
		Components:       components,
		Types:            []string{"street_address"},
	}, nil
}

// GetPlaceDetails implements AddressProvider - uses geocode as LocationIQ uses same API
func (l *LocationIQProvider) GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error) {
	// LocationIQ doesn't have a separate place details API, use geocode
	return l.Geocode(ctx, placeID)
}

// ValidateAddress implements AddressProvider
func (l *LocationIQProvider) ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error) {
	result, err := l.Geocode(ctx, address)
	if err != nil {
		return &models.AddressValidationResult{
			Valid:  false,
			Issues: []string{err.Error()},
		}, nil
	}

	return &models.AddressValidationResult{
		Valid:            true,
		FormattedAddress: result.FormattedAddress,
		Components:       result.Components,
		Location:         &result.Location,
		Deliverable:      true,
	}, nil
}

// ==================== PHOTON (KOMOOT) PROVIDER ====================

// PhotonProvider implements AddressProvider using Photon (Komoot's geocoding service)
// Free to use, based on OpenStreetMap data, higher rate limits than public Nominatim
type PhotonProvider struct {
	httpClient *http.Client
	baseURL    string
}

// NewPhotonProvider creates a new Photon provider
// Uses Komoot's public Photon instance by default
func NewPhotonProvider(httpClient *http.Client) *PhotonProvider {
	return &PhotonProvider{
		httpClient: httpClient,
		baseURL:    "https://photon.komoot.io",
	}
}

// NewPhotonProviderWithURL creates a new Photon provider with custom URL (for self-hosted)
func NewPhotonProviderWithURL(baseURL string, httpClient *http.Client) *PhotonProvider {
	return &PhotonProvider{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// Autocomplete implements AddressProvider using Photon API
func (p *PhotonProvider) Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error) {
	params := url.Values{}
	params.Set("q", input)
	params.Set("limit", "5")

	// Photon uses lang parameter for language
	if opts.Language != "" {
		params.Set("lang", opts.Language)
	}

	// Location bias
	if opts.Location != nil {
		params.Set("lat", fmt.Sprintf("%f", opts.Location.Latitude))
		params.Set("lon", fmt.Sprintf("%f", opts.Location.Longitude))
	}

	reqURL := fmt.Sprintf("%s/api?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Photon API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Photon API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Features []struct {
			Type     string `json:"type"`
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"` // [lon, lat]
			} `json:"geometry"`
			Properties struct {
				OsmID       int64  `json:"osm_id"`
				OsmType     string `json:"osm_type"`
				OsmKey      string `json:"osm_key"`
				OsmValue    string `json:"osm_value"`
				Name        string `json:"name"`
				HouseNumber string `json:"housenumber"`
				Street      string `json:"street"`
				City        string `json:"city"`
				District    string `json:"district"`
				State       string `json:"state"`
				Postcode    string `json:"postcode"`
				Country     string `json:"country"`
				CountryCode string `json:"countrycode"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Filter by country if restriction is set
	countryFilter := ""
	if opts.Components != "" && strings.HasPrefix(opts.Components, "country:") {
		countryFilter = strings.ToUpper(strings.TrimPrefix(opts.Components, "country:"))
	}

	suggestions := make([]models.AddressSuggestion, 0, len(result.Features))
	for _, f := range result.Features {
		// Apply country filter if specified
		if countryFilter != "" && strings.ToUpper(f.Properties.CountryCode) != countryFilter {
			continue
		}

		props := f.Properties

		// Build main text
		mainText := ""
		if props.HouseNumber != "" {
			mainText = props.HouseNumber + " "
		}
		if props.Street != "" {
			mainText += props.Street
		}
		if mainText == "" && props.Name != "" {
			mainText = props.Name
		}

		// Build secondary text
		city := props.City
		if city == "" {
			city = props.District
		}

		secondaryText := ""
		if city != "" {
			secondaryText = city
		}
		if props.State != "" {
			if secondaryText != "" {
				secondaryText += ", "
			}
			secondaryText += props.State
		}
		if props.Country != "" {
			if secondaryText != "" {
				secondaryText += ", "
			}
			secondaryText += props.Country
		}

		// Build full description
		description := mainText
		if secondaryText != "" {
			if description != "" {
				description += ", "
			}
			description += secondaryText
		}

		suggestions = append(suggestions, models.AddressSuggestion{
			PlaceID:       fmt.Sprintf("photon:%s:%d", props.OsmType, props.OsmID),
			Description:   description,
			MainText:      mainText,
			SecondaryText: secondaryText,
			Types:         []string{props.OsmKey, props.OsmValue},
		})
	}

	return suggestions, nil
}

// Geocode implements AddressProvider using Photon API
func (p *PhotonProvider) Geocode(ctx context.Context, address string) (*models.GeocodingResult, error) {
	params := url.Values{}
	params.Set("q", address)
	params.Set("limit", "1")

	reqURL := fmt.Sprintf("%s/api?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Photon API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"` // [lon, lat]
			} `json:"geometry"`
			Properties struct {
				OsmID       int64  `json:"osm_id"`
				OsmType     string `json:"osm_type"`
				Name        string `json:"name"`
				HouseNumber string `json:"housenumber"`
				Street      string `json:"street"`
				City        string `json:"city"`
				District    string `json:"district"`
				State       string `json:"state"`
				Postcode    string `json:"postcode"`
				Country     string `json:"country"`
				CountryCode string `json:"countrycode"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Features) == 0 {
		return nil, fmt.Errorf("no results found for address: %s", address)
	}

	f := result.Features[0]
	props := f.Properties
	coords := f.Geometry.Coordinates

	lat := 0.0
	lng := 0.0
	if len(coords) >= 2 {
		lng = coords[0]
		lat = coords[1]
	}

	city := props.City
	if city == "" {
		city = props.District
	}

	// Build formatted address
	parts := []string{}
	if props.HouseNumber != "" && props.Street != "" {
		parts = append(parts, props.HouseNumber+" "+props.Street)
	} else if props.Street != "" {
		parts = append(parts, props.Street)
	} else if props.Name != "" {
		parts = append(parts, props.Name)
	}
	if city != "" {
		parts = append(parts, city)
	}
	if props.State != "" {
		parts = append(parts, props.State)
	}
	if props.Postcode != "" {
		parts = append(parts, props.Postcode)
	}
	if props.Country != "" {
		parts = append(parts, props.Country)
	}
	formattedAddress := strings.Join(parts, ", ")

	components := []models.AddressComponent{}
	if props.HouseNumber != "" {
		components = append(components, models.AddressComponent{Type: "street_number", LongName: props.HouseNumber, ShortName: props.HouseNumber})
	}
	if props.Street != "" {
		components = append(components, models.AddressComponent{Type: "route", LongName: props.Street, ShortName: props.Street})
	}
	if city != "" {
		components = append(components, models.AddressComponent{Type: "locality", LongName: city, ShortName: city})
	}
	if props.State != "" {
		components = append(components, models.AddressComponent{Type: "administrative_area_level_1", LongName: props.State, ShortName: props.State})
	}
	if props.Country != "" {
		components = append(components, models.AddressComponent{Type: "country", LongName: props.Country, ShortName: strings.ToUpper(props.CountryCode)})
	}
	if props.Postcode != "" {
		components = append(components, models.AddressComponent{Type: "postal_code", LongName: props.Postcode, ShortName: props.Postcode})
	}

	return &models.GeocodingResult{
		FormattedAddress: formattedAddress,
		PlaceID:          fmt.Sprintf("photon:%s:%d", props.OsmType, props.OsmID),
		Location: models.GeoLocation{
			Latitude:  lat,
			Longitude: lng,
		},
		Components: components,
		Types:      []string{"street_address"},
	}, nil
}

// ReverseGeocode implements AddressProvider using Photon reverse API
func (p *PhotonProvider) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error) {
	params := url.Values{}
	params.Set("lat", fmt.Sprintf("%f", lat))
	params.Set("lon", fmt.Sprintf("%f", lng))

	reqURL := fmt.Sprintf("%s/reverse?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Photon API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Features []struct {
			Properties struct {
				OsmID       int64  `json:"osm_id"`
				OsmType     string `json:"osm_type"`
				Name        string `json:"name"`
				HouseNumber string `json:"housenumber"`
				Street      string `json:"street"`
				City        string `json:"city"`
				District    string `json:"district"`
				State       string `json:"state"`
				Postcode    string `json:"postcode"`
				Country     string `json:"country"`
				CountryCode string `json:"countrycode"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Features) == 0 {
		return nil, fmt.Errorf("no results found for coordinates: %f, %f", lat, lng)
	}

	props := result.Features[0].Properties

	city := props.City
	if city == "" {
		city = props.District
	}

	// Build formatted address
	parts := []string{}
	if props.HouseNumber != "" && props.Street != "" {
		parts = append(parts, props.HouseNumber+" "+props.Street)
	} else if props.Street != "" {
		parts = append(parts, props.Street)
	} else if props.Name != "" {
		parts = append(parts, props.Name)
	}
	if city != "" {
		parts = append(parts, city)
	}
	if props.State != "" {
		parts = append(parts, props.State)
	}
	if props.Postcode != "" {
		parts = append(parts, props.Postcode)
	}
	if props.Country != "" {
		parts = append(parts, props.Country)
	}
	formattedAddress := strings.Join(parts, ", ")

	components := []models.AddressComponent{}
	if props.HouseNumber != "" {
		components = append(components, models.AddressComponent{Type: "street_number", LongName: props.HouseNumber, ShortName: props.HouseNumber})
	}
	if props.Street != "" {
		components = append(components, models.AddressComponent{Type: "route", LongName: props.Street, ShortName: props.Street})
	}
	if city != "" {
		components = append(components, models.AddressComponent{Type: "locality", LongName: city, ShortName: city})
	}
	if props.State != "" {
		components = append(components, models.AddressComponent{Type: "administrative_area_level_1", LongName: props.State, ShortName: props.State})
	}
	if props.Country != "" {
		components = append(components, models.AddressComponent{Type: "country", LongName: props.Country, ShortName: strings.ToUpper(props.CountryCode)})
	}
	if props.Postcode != "" {
		components = append(components, models.AddressComponent{Type: "postal_code", LongName: props.Postcode, ShortName: props.Postcode})
	}

	return &models.ReverseGeocodingResult{
		FormattedAddress: formattedAddress,
		PlaceID:          fmt.Sprintf("photon:%s:%d", props.OsmType, props.OsmID),
		Components:       components,
		Types:            []string{"street_address"},
	}, nil
}

// GetPlaceDetails implements AddressProvider - Photon doesn't have place details, use geocode
func (p *PhotonProvider) GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error) {
	return p.Geocode(ctx, placeID)
}

// ValidateAddress implements AddressProvider
func (p *PhotonProvider) ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error) {
	result, err := p.Geocode(ctx, address)
	if err != nil {
		return &models.AddressValidationResult{
			Valid:  false,
			Issues: []string{err.Error()},
		}, nil
	}

	return &models.AddressValidationResult{
		Valid:            true,
		FormattedAddress: result.FormattedAddress,
		Components:       result.Components,
		Location:         &result.Location,
		Deliverable:      true,
	}, nil
}
