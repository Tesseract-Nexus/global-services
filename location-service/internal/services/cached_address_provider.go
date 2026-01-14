package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"location-service/internal/models"
	"location-service/internal/repository"
)

// CacheConfig holds configuration for the address cache
type CacheConfig struct {
	// Enable/disable caching
	Enabled bool

	// TTL configurations by cache type
	GeocodeTTL        time.Duration // Default: 30 days
	ReverseGeocodeTTL time.Duration // Default: 30 days
	AutocompleteTTL   time.Duration // Default: 7 days
	PlaceDetailsTTL   time.Duration // Default: 30 days

	// Enable permanent storage of places
	StorePlaces bool

	// Minimum confidence to store a place
	MinConfidence float64
}

// DefaultCacheConfig returns sensible defaults
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:           true,
		GeocodeTTL:        30 * 24 * time.Hour,
		ReverseGeocodeTTL: 30 * 24 * time.Hour,
		AutocompleteTTL:   7 * 24 * time.Hour,
		PlaceDetailsTTL:   30 * 24 * time.Hour,
		StorePlaces:       true,
		MinConfidence:     0.7,
	}
}

// CacheMetrics tracks cache statistics
type CacheMetrics struct {
	Hits       map[string]int64
	Misses     map[string]int64
	Sets       map[string]int64
	Errors     map[string]int64
	TotalHits  int64
	TotalMisses int64
}

// NewCacheMetrics creates a new metrics tracker
func NewCacheMetrics() *CacheMetrics {
	return &CacheMetrics{
		Hits:   make(map[string]int64),
		Misses: make(map[string]int64),
		Sets:   make(map[string]int64),
		Errors: make(map[string]int64),
	}
}

// RecordHit records a cache hit
func (m *CacheMetrics) RecordHit(cacheType string) {
	m.Hits[cacheType]++
	m.TotalHits++
}

// RecordMiss records a cache miss
func (m *CacheMetrics) RecordMiss(cacheType string) {
	m.Misses[cacheType]++
	m.TotalMisses++
}

// RecordSet records a cache set operation
func (m *CacheMetrics) RecordSet(cacheType string) {
	m.Sets[cacheType]++
}

// RecordError records a cache error
func (m *CacheMetrics) RecordError(cacheType string) {
	m.Errors[cacheType]++
}

// GetHitRate returns the overall cache hit rate
func (m *CacheMetrics) GetHitRate() float64 {
	total := m.TotalHits + m.TotalMisses
	if total == 0 {
		return 0
	}
	return float64(m.TotalHits) / float64(total)
}

// CachedAddressProvider wraps an AddressProvider with database caching
type CachedAddressProvider struct {
	inner      AddressProvider
	cacheRepo  repository.AddressCacheRepository
	placesRepo repository.PlacesRepository
	config     CacheConfig
	metrics    *CacheMetrics
}

// NewCachedAddressProvider creates a cached wrapper around an address provider
func NewCachedAddressProvider(
	inner AddressProvider,
	cacheRepo repository.AddressCacheRepository,
	placesRepo repository.PlacesRepository,
	config CacheConfig,
) *CachedAddressProvider {
	return &CachedAddressProvider{
		inner:      inner,
		cacheRepo:  cacheRepo,
		placesRepo: placesRepo,
		config:     config,
		metrics:    NewCacheMetrics(),
	}
}

// GetMetrics returns the cache metrics
func (c *CachedAddressProvider) GetMetrics() *CacheMetrics {
	return c.metrics
}

// generateHash generates a SHA256 hash of the input
func generateHash(input string) string {
	hasher := sha256.New()
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))
}

// normalizeQuery normalizes a query string for consistent caching
func normalizeQuery(query string) string {
	query = strings.ToLower(query)
	query = strings.TrimSpace(query)
	query = strings.Join(strings.Fields(query), " ")
	return query
}

// generateGeocodeKey creates a cache key for geocode queries
func generateGeocodeKey(address string) (key string, hash string) {
	key = "geocode:" + normalizeQuery(address)
	hash = generateHash(key)
	return
}

// generateReverseGeocodeKey creates a cache key for reverse geocode queries
func generateReverseGeocodeKey(lat, lng float64) (key string, hash string) {
	// Round to 6 decimal places (~0.1m precision) for consistent caching
	key = fmt.Sprintf("reverse:%.6f,%.6f", lat, lng)
	hash = generateHash(key)
	return
}

// generateAutocompleteKey creates a cache key for autocomplete queries
func generateAutocompleteKey(input string, opts AutocompleteOptions) (key string, hash string) {
	parts := []string{"autocomplete", normalizeQuery(input)}

	if opts.Components != "" {
		parts = append(parts, "comp:"+opts.Components)
	}
	if opts.Language != "" {
		parts = append(parts, "lang:"+opts.Language)
	}
	if len(opts.Types) > 0 {
		parts = append(parts, "types:"+strings.Join(opts.Types, ","))
	}

	key = strings.Join(parts, ":")
	hash = generateHash(key)
	return
}

// generatePlaceDetailsKey creates a cache key for place details queries
func generatePlaceDetailsKey(placeID string) (key string, hash string) {
	key = "place_details:" + placeID
	hash = generateHash(key)
	return
}

// Autocomplete implements AddressProvider with caching
func (c *CachedAddressProvider) Autocomplete(ctx context.Context, input string, opts AutocompleteOptions) ([]models.AddressSuggestion, error) {
	if !c.config.Enabled {
		return c.inner.Autocomplete(ctx, input, opts)
	}

	cacheType := string(models.CacheTypeAutocomplete)
	key, hash := generateAutocompleteKey(input, opts)

	// Try cache first
	cached, err := c.cacheRepo.GetByHash(ctx, cacheType, hash)
	if err == nil && cached != nil {
		c.metrics.RecordHit(cacheType)

		// Increment hit count asynchronously
		go func() {
			if err := c.cacheRepo.IncrementHits(context.Background(), cached.ID); err != nil {
				log.Printf("Failed to increment cache hits: %v", err)
			}
		}()

		// Deserialize suggestions from JSON
		var suggestions []models.AddressSuggestion
		if cached.ResponseJSON != nil {
			data, _ := json.Marshal(cached.ResponseJSON)
			if err := json.Unmarshal(data, &suggestions); err == nil {
				return suggestions, nil
			}
		}
	}

	c.metrics.RecordMiss(cacheType)

	// Cache miss - call underlying provider
	suggestions, err := c.inner.Autocomplete(ctx, input, opts)
	if err != nil {
		return nil, err
	}

	// Store in cache asynchronously
	if len(suggestions) > 0 {
		go c.storeAutocompleteCacheAsync(key, hash, input, suggestions)
	}

	return suggestions, nil
}

// storeAutocompleteCacheAsync stores autocomplete results in cache
func (c *CachedAddressProvider) storeAutocompleteCacheAsync(key, hash, input string, suggestions []models.AddressSuggestion) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Convert suggestions to JSON
	responseData := make(map[string]interface{})
	responseData["suggestions"] = suggestions

	entry := &models.AddressCacheEntry{
		CacheType:    string(models.CacheTypeAutocomplete),
		CacheKey:     key,
		CacheKeyHash: hash,
		ResponseJSON: responseData,
		Provider:     "failover",
		ExpiresAt:    time.Now().Add(c.config.AutocompleteTTL),
	}

	if err := c.cacheRepo.Set(ctx, entry); err != nil {
		log.Printf("Failed to cache autocomplete results: %v", err)
		c.metrics.RecordError(string(models.CacheTypeAutocomplete))
	} else {
		c.metrics.RecordSet(string(models.CacheTypeAutocomplete))
	}
}

// Geocode implements AddressProvider with caching
func (c *CachedAddressProvider) Geocode(ctx context.Context, address string) (*models.GeocodingResult, error) {
	if !c.config.Enabled {
		return c.inner.Geocode(ctx, address)
	}

	cacheType := string(models.CacheTypeGeocode)
	key, hash := generateGeocodeKey(address)

	// Try cache first
	cached, err := c.cacheRepo.GetByHash(ctx, cacheType, hash)
	if err == nil && cached != nil {
		c.metrics.RecordHit(cacheType)

		go func() {
			if err := c.cacheRepo.IncrementHits(context.Background(), cached.ID); err != nil {
				log.Printf("Failed to increment cache hits: %v", err)
			}
		}()

		return cacheEntryToGeocodingResult(cached), nil
	}

	c.metrics.RecordMiss(cacheType)

	// Cache miss - call underlying provider
	result, err := c.inner.Geocode(ctx, address)
	if err != nil {
		return nil, err
	}

	if result != nil {
		// Store in cache asynchronously
		go c.storeGeocodeCacheAsync(key, hash, address, result)

		// Store in places table if enabled
		if c.config.StorePlaces {
			go c.storePlaceAsync(result)
		}
	}

	return result, nil
}

// storeGeocodeCacheAsync stores geocode results in cache
func (c *CachedAddressProvider) storeGeocodeCacheAsync(key, hash, address string, result *models.GeocodingResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := models.NewAddressCacheEntryFromGeocodingResult(
		models.CacheTypeGeocode,
		key,
		hash,
		result,
		"failover",
		c.config.GeocodeTTL,
	)

	if err := c.cacheRepo.Set(ctx, entry); err != nil {
		log.Printf("Failed to cache geocode results: %v", err)
		c.metrics.RecordError(string(models.CacheTypeGeocode))
	} else {
		c.metrics.RecordSet(string(models.CacheTypeGeocode))
	}
}

// storePlaceAsync stores the result in the permanent places table
func (c *CachedAddressProvider) storePlaceAsync(result *models.GeocodingResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if place already exists
	if result.PlaceID != "" {
		existing, _ := c.placesRepo.GetByExternalID(ctx, result.PlaceID)
		if existing != nil {
			return // Already exists
		}
	}

	place := models.NewPlaceFromGeocodingResult(result, "failover")

	if err := c.placesRepo.Create(ctx, place); err != nil {
		log.Printf("Failed to store place: %v", err)
	}
}

// ReverseGeocode implements AddressProvider with caching
func (c *CachedAddressProvider) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.ReverseGeocodingResult, error) {
	if !c.config.Enabled {
		return c.inner.ReverseGeocode(ctx, lat, lng)
	}

	cacheType := string(models.CacheTypeReverse)
	key, hash := generateReverseGeocodeKey(lat, lng)

	// Try cache first
	cached, err := c.cacheRepo.GetByHash(ctx, cacheType, hash)
	if err == nil && cached != nil {
		c.metrics.RecordHit(cacheType)

		go func() {
			if err := c.cacheRepo.IncrementHits(context.Background(), cached.ID); err != nil {
				log.Printf("Failed to increment cache hits: %v", err)
			}
		}()

		return cacheEntryToReverseGeocodingResult(cached), nil
	}

	c.metrics.RecordMiss(cacheType)

	// Cache miss - call underlying provider
	result, err := c.inner.ReverseGeocode(ctx, lat, lng)
	if err != nil {
		return nil, err
	}

	if result != nil {
		go c.storeReverseGeocodeCacheAsync(key, hash, lat, lng, result)
	}

	return result, nil
}

// storeReverseGeocodeCacheAsync stores reverse geocode results in cache
func (c *CachedAddressProvider) storeReverseGeocodeCacheAsync(key, hash string, lat, lng float64, result *models.ReverseGeocodingResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := &models.AddressCacheEntry{
		CacheType:        string(models.CacheTypeReverse),
		CacheKey:         key,
		CacheKeyHash:     hash,
		FormattedAddress: result.FormattedAddress,
		PlaceID:          result.PlaceID,
		Latitude:         &lat,
		Longitude:        &lng,
		Provider:         "failover",
		ExpiresAt:        time.Now().Add(c.config.ReverseGeocodeTTL),
	}

	// Extract components
	for _, comp := range result.Components {
		switch comp.Type {
		case "street_number":
			entry.StreetNumber = comp.LongName
		case "route":
			entry.StreetName = comp.LongName
		case "locality":
			entry.City = comp.LongName
		case "sublocality", "sublocality_level_1":
			entry.District = comp.LongName
		case "administrative_area_level_1":
			entry.StateName = comp.LongName
			entry.StateCode = comp.ShortName
		case "country":
			entry.CountryName = comp.LongName
			entry.CountryCode = comp.ShortName
		case "postal_code":
			entry.PostalCode = comp.LongName
		}
	}

	if err := c.cacheRepo.Set(ctx, entry); err != nil {
		log.Printf("Failed to cache reverse geocode results: %v", err)
		c.metrics.RecordError(string(models.CacheTypeReverse))
	} else {
		c.metrics.RecordSet(string(models.CacheTypeReverse))
	}
}

// GetPlaceDetails implements AddressProvider with caching
func (c *CachedAddressProvider) GetPlaceDetails(ctx context.Context, placeID string) (*models.GeocodingResult, error) {
	if !c.config.Enabled {
		return c.inner.GetPlaceDetails(ctx, placeID)
	}

	cacheType := string(models.CacheTypePlaceDetails)
	key, hash := generatePlaceDetailsKey(placeID)

	// Try cache first
	cached, err := c.cacheRepo.GetByHash(ctx, cacheType, hash)
	if err == nil && cached != nil {
		c.metrics.RecordHit(cacheType)

		go func() {
			if err := c.cacheRepo.IncrementHits(context.Background(), cached.ID); err != nil {
				log.Printf("Failed to increment cache hits: %v", err)
			}
		}()

		return cacheEntryToGeocodingResult(cached), nil
	}

	c.metrics.RecordMiss(cacheType)

	// Cache miss - call underlying provider
	result, err := c.inner.GetPlaceDetails(ctx, placeID)
	if err != nil {
		return nil, err
	}

	if result != nil {
		go c.storePlaceDetailsCacheAsync(key, hash, placeID, result)

		if c.config.StorePlaces {
			go c.storePlaceAsync(result)
		}
	}

	return result, nil
}

// storePlaceDetailsCacheAsync stores place details in cache
func (c *CachedAddressProvider) storePlaceDetailsCacheAsync(key, hash, placeID string, result *models.GeocodingResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := models.NewAddressCacheEntryFromGeocodingResult(
		models.CacheTypePlaceDetails,
		key,
		hash,
		result,
		"failover",
		c.config.PlaceDetailsTTL,
	)

	if err := c.cacheRepo.Set(ctx, entry); err != nil {
		log.Printf("Failed to cache place details: %v", err)
		c.metrics.RecordError(string(models.CacheTypePlaceDetails))
	} else {
		c.metrics.RecordSet(string(models.CacheTypePlaceDetails))
	}
}

// ValidateAddress implements AddressProvider (no caching - validation should be fresh)
func (c *CachedAddressProvider) ValidateAddress(ctx context.Context, address string) (*models.AddressValidationResult, error) {
	// Validation is not cached as it checks current deliverability
	return c.inner.ValidateAddress(ctx, address)
}

// cacheEntryToGeocodingResult converts a cache entry to a GeocodingResult
func cacheEntryToGeocodingResult(entry *models.AddressCacheEntry) *models.GeocodingResult {
	result := &models.GeocodingResult{
		FormattedAddress: entry.FormattedAddress,
		PlaceID:          entry.PlaceID,
		Components:       make([]models.AddressComponent, 0),
	}

	if entry.Latitude != nil && entry.Longitude != nil {
		result.Location = models.GeoLocation{
			Latitude:  *entry.Latitude,
			Longitude: *entry.Longitude,
		}
	}

	// Reconstruct components
	if entry.StreetNumber != "" {
		result.Components = append(result.Components, models.AddressComponent{
			Type:      "street_number",
			LongName:  entry.StreetNumber,
			ShortName: entry.StreetNumber,
		})
	}
	if entry.StreetName != "" {
		result.Components = append(result.Components, models.AddressComponent{
			Type:      "route",
			LongName:  entry.StreetName,
			ShortName: entry.StreetName,
		})
	}
	if entry.City != "" {
		result.Components = append(result.Components, models.AddressComponent{
			Type:      "locality",
			LongName:  entry.City,
			ShortName: entry.City,
		})
	}
	if entry.District != "" {
		result.Components = append(result.Components, models.AddressComponent{
			Type:      "sublocality",
			LongName:  entry.District,
			ShortName: entry.District,
		})
	}
	if entry.StateName != "" {
		result.Components = append(result.Components, models.AddressComponent{
			Type:      "administrative_area_level_1",
			LongName:  entry.StateName,
			ShortName: entry.StateCode,
		})
	}
	if entry.CountryName != "" {
		result.Components = append(result.Components, models.AddressComponent{
			Type:      "country",
			LongName:  entry.CountryName,
			ShortName: entry.CountryCode,
		})
	}
	if entry.PostalCode != "" {
		result.Components = append(result.Components, models.AddressComponent{
			Type:      "postal_code",
			LongName:  entry.PostalCode,
			ShortName: entry.PostalCode,
		})
	}

	return result
}

// cacheEntryToReverseGeocodingResult converts a cache entry to a ReverseGeocodingResult
func cacheEntryToReverseGeocodingResult(entry *models.AddressCacheEntry) *models.ReverseGeocodingResult {
	geocodeResult := cacheEntryToGeocodingResult(entry)

	return &models.ReverseGeocodingResult{
		FormattedAddress: geocodeResult.FormattedAddress,
		PlaceID:          geocodeResult.PlaceID,
		Components:       geocodeResult.Components,
		Types:            geocodeResult.Types,
	}
}
