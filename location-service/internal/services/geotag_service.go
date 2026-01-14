package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"location-service/internal/models"
	"location-service/internal/repository"
)

// GeoTagService provides high-level GeoTag API operations
type GeoTagService struct {
	placesRepo   repository.PlacesRepository
	cacheRepo    repository.AddressCacheRepository
	addressSvc   *AddressService
	cachedProvider *CachedAddressProvider
}

// NewGeoTagService creates a new GeoTag service
func NewGeoTagService(
	placesRepo repository.PlacesRepository,
	cacheRepo repository.AddressCacheRepository,
	addressSvc *AddressService,
	cachedProvider *CachedAddressProvider,
) *GeoTagService {
	return &GeoTagService{
		placesRepo:     placesRepo,
		cacheRepo:      cacheRepo,
		addressSvc:     addressSvc,
		cachedProvider: cachedProvider,
	}
}

// Geocode performs forward geocoding with caching
func (s *GeoTagService) Geocode(ctx context.Context, address string) (*models.GeoTagResult, *models.CacheInfo, error) {
	if address == "" {
		return nil, nil, errors.New("address is required")
	}

	startTime := time.Now()
	var cached bool

	// Use cached provider if available
	var result *models.GeocodingResult
	var err error

	if s.cachedProvider != nil {
		result, err = s.cachedProvider.Geocode(ctx, address)
		// Check if it was a cache hit by looking at metrics
		// (This is a simplified check - in production, you'd track this more precisely)
		cached = time.Since(startTime) < 50*time.Millisecond
	} else {
		result, err = s.addressSvc.Geocode(ctx, address)
	}

	if err != nil {
		return nil, nil, err
	}

	if result == nil {
		return nil, nil, errors.New("no results found")
	}

	geoTagResult := geocodingResultToGeoTagResult(result, "failover", cached)

	cacheInfo := &models.CacheInfo{
		Hit:        cached,
		AgeSeconds: 0, // Would need to track this separately
	}

	return geoTagResult, cacheInfo, nil
}

// ReverseGeocode performs reverse geocoding with caching
func (s *GeoTagService) ReverseGeocode(ctx context.Context, lat, lng float64) (*models.GeoTagResult, *models.CacheInfo, error) {
	if lat < -90 || lat > 90 {
		return nil, nil, errors.New("invalid latitude: must be between -90 and 90")
	}
	if lng < -180 || lng > 180 {
		return nil, nil, errors.New("invalid longitude: must be between -180 and 180")
	}

	startTime := time.Now()
	var cached bool

	var result *models.ReverseGeocodingResult
	var err error

	if s.cachedProvider != nil {
		result, err = s.cachedProvider.ReverseGeocode(ctx, lat, lng)
		cached = time.Since(startTime) < 50*time.Millisecond
	} else {
		result, err = s.addressSvc.ReverseGeocode(ctx, lat, lng)
	}

	if err != nil {
		return nil, nil, err
	}

	if result == nil {
		return nil, nil, errors.New("no results found")
	}

	geoTagResult := reverseGeocodingResultToGeoTagResult(result, lat, lng, "failover", cached)

	cacheInfo := &models.CacheInfo{
		Hit:        cached,
		AgeSeconds: 0,
	}

	return geoTagResult, cacheInfo, nil
}

// Autocomplete returns address suggestions
func (s *GeoTagService) Autocomplete(ctx context.Context, input string, countryCode string) ([]models.AddressSuggestion, *models.CacheInfo, error) {
	if input == "" {
		return nil, nil, errors.New("input is required")
	}

	startTime := time.Now()
	var cached bool

	opts := AutocompleteOptions{}
	if countryCode != "" {
		opts.Components = "country:" + countryCode
	}

	var suggestions []models.AddressSuggestion
	var err error

	if s.cachedProvider != nil {
		suggestions, err = s.cachedProvider.Autocomplete(ctx, input, opts)
		cached = time.Since(startTime) < 50*time.Millisecond
	} else {
		suggestions, err = s.addressSvc.Autocomplete(ctx, input, opts)
	}

	if err != nil {
		return nil, nil, err
	}

	cacheInfo := &models.CacheInfo{
		Hit:        cached,
		AgeSeconds: 0,
	}

	return suggestions, cacheInfo, nil
}

// GetPlace retrieves a place by ID
func (s *GeoTagService) GetPlace(ctx context.Context, id uuid.UUID) (*models.Place, error) {
	return s.placesRepo.GetByID(ctx, id)
}

// SearchPlaces searches for places with filters
func (s *GeoTagService) SearchPlaces(ctx context.Context, filters models.SearchFilters) ([]models.Place, int64, error) {
	return s.placesRepo.Search(ctx, filters.Query, filters)
}

// FindNearby finds places near a location
func (s *GeoTagService) FindNearby(ctx context.Context, query models.NearbyQuery) ([]models.Place, error) {
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.placesRepo.FindNearby(ctx, query.Latitude, query.Longitude, query.RadiusKm, limit)
}

// GetPlacesByCity retrieves places in a city
func (s *GeoTagService) GetPlacesByCity(ctx context.Context, city, countryCode string, limit, offset int) ([]models.Place, int64, error) {
	return s.placesRepo.GetByCity(ctx, city, countryCode, limit, offset)
}

// ValidateAndStore validates an address and stores it permanently
func (s *GeoTagService) ValidateAndStore(ctx context.Context, address string) (*models.Place, error) {
	// First geocode the address
	result, err := s.addressSvc.Geocode(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to geocode address: %w", err)
	}

	if result == nil {
		return nil, errors.New("address not found")
	}

	// Check if place already exists by external ID
	if result.PlaceID != "" {
		existing, _ := s.placesRepo.GetByExternalID(ctx, result.PlaceID)
		if existing != nil {
			return existing, nil
		}
	}

	// Create new place
	place := models.NewPlaceFromGeocodingResult(result, "failover")

	if err := s.placesRepo.Create(ctx, place); err != nil {
		return nil, fmt.Errorf("failed to store place: %w", err)
	}

	return place, nil
}

// BulkGeocode geocodes multiple addresses concurrently
func (s *GeoTagService) BulkGeocode(ctx context.Context, addresses []string) ([]models.BulkGeocodeResult, error) {
	if len(addresses) == 0 {
		return nil, errors.New("no addresses provided")
	}
	if len(addresses) > 100 {
		return nil, errors.New("maximum 100 addresses per request")
	}

	results := make([]models.BulkGeocodeResult, len(addresses))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Process in parallel with concurrency limit
	semaphore := make(chan struct{}, 10) // Max 10 concurrent requests

	for i, addr := range addresses {
		wg.Add(1)
		go func(index int, address string) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			result := models.BulkGeocodeResult{
				Index:   index,
				Address: address,
			}

			startTime := time.Now()
			geoResult, _, err := s.Geocode(ctx, address)
			cached := time.Since(startTime) < 50*time.Millisecond

			if err != nil {
				result.Error = err.Error()
			} else if geoResult != nil {
				result.Result = geoResult
				result.Cached = cached
			}

			mu.Lock()
			results[index] = result
			mu.Unlock()
		}(i, addr)
	}

	wg.Wait()
	return results, nil
}

// GetCacheStats returns cache statistics
func (s *GeoTagService) GetCacheStats(ctx context.Context) (*models.CacheStats, error) {
	return s.cacheRepo.GetStats(ctx)
}

// GetPlacesStats returns statistics about stored places
func (s *GeoTagService) GetPlacesStats(ctx context.Context) (*models.PlacesStats, error) {
	return s.placesRepo.GetStats(ctx)
}

// ClearExpiredCache removes expired cache entries
func (s *GeoTagService) ClearExpiredCache(ctx context.Context, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 1000
	}
	return s.cacheRepo.DeleteExpired(ctx, batchSize)
}

// SetPlaceVerified marks a place as verified
func (s *GeoTagService) SetPlaceVerified(ctx context.Context, id uuid.UUID, verified bool) error {
	return s.placesRepo.SetVerified(ctx, id, verified)
}

// DeletePlace deletes a place
func (s *GeoTagService) DeletePlace(ctx context.Context, id uuid.UUID) error {
	return s.placesRepo.Delete(ctx, id)
}

// GetCacheMetrics returns the cache metrics from the cached provider
func (s *GeoTagService) GetCacheMetrics() *CacheMetrics {
	if s.cachedProvider != nil {
		return s.cachedProvider.GetMetrics()
	}
	return nil
}

// Helper functions

func geocodingResultToGeoTagResult(result *models.GeocodingResult, provider string, cached bool) *models.GeoTagResult {
	geoTagResult := &models.GeoTagResult{
		FormattedAddress: result.FormattedAddress,
		Location: models.GeoTagLocation{
			Latitude:  result.Location.Latitude,
			Longitude: result.Location.Longitude,
		},
		Metadata: models.GeoTagMetadata{
			Provider:   provider,
			Cached:     cached,
			PlaceTypes: result.Types,
			PlaceID:    result.PlaceID,
		},
	}

	// Extract components
	for _, comp := range result.Components {
		switch comp.Type {
		case "street_number":
			geoTagResult.Components.StreetNumber = comp.LongName
		case "route":
			geoTagResult.Components.StreetName = comp.LongName
		case "locality":
			geoTagResult.Components.City = comp.LongName
		case "sublocality", "sublocality_level_1":
			geoTagResult.Components.District = comp.LongName
		case "administrative_area_level_1":
			geoTagResult.Components.StateName = comp.LongName
			geoTagResult.Components.StateCode = comp.ShortName
		case "country":
			geoTagResult.Components.CountryName = comp.LongName
			geoTagResult.Components.CountryCode = comp.ShortName
		case "postal_code":
			geoTagResult.Components.PostalCode = comp.LongName
		}
	}

	return geoTagResult
}

func reverseGeocodingResultToGeoTagResult(result *models.ReverseGeocodingResult, lat, lng float64, provider string, cached bool) *models.GeoTagResult {
	geoTagResult := &models.GeoTagResult{
		FormattedAddress: result.FormattedAddress,
		Location: models.GeoTagLocation{
			Latitude:  lat,
			Longitude: lng,
		},
		Metadata: models.GeoTagMetadata{
			Provider:   provider,
			Cached:     cached,
			PlaceTypes: result.Types,
			PlaceID:    result.PlaceID,
		},
	}

	// Extract components
	for _, comp := range result.Components {
		switch comp.Type {
		case "street_number":
			geoTagResult.Components.StreetNumber = comp.LongName
		case "route":
			geoTagResult.Components.StreetName = comp.LongName
		case "locality":
			geoTagResult.Components.City = comp.LongName
		case "sublocality", "sublocality_level_1":
			geoTagResult.Components.District = comp.LongName
		case "administrative_area_level_1":
			geoTagResult.Components.StateName = comp.LongName
			geoTagResult.Components.StateCode = comp.ShortName
		case "country":
			geoTagResult.Components.CountryName = comp.LongName
			geoTagResult.Components.CountryCode = comp.ShortName
		case "postal_code":
			geoTagResult.Components.PostalCode = comp.LongName
		}
	}

	return geoTagResult
}
