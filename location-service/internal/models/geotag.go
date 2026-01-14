package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// CacheType represents the type of cached query
type CacheType string

const (
	CacheTypeGeocode      CacheType = "geocode"
	CacheTypeReverse      CacheType = "reverse"
	CacheTypeAutocomplete CacheType = "autocomplete"
	CacheTypePlaceDetails CacheType = "place_details"
)

// AddressCacheEntry represents a cached address lookup result
type AddressCacheEntry struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CacheType        string    `gorm:"size:20;not null;index:idx_cache_lookup,priority:1" json:"cache_type"`
	CacheKeyHash     string    `gorm:"size:64;not null;index:idx_cache_lookup,priority:2" json:"cache_key_hash"`
	CacheKey         string    `gorm:"type:text;not null" json:"cache_key"`
	FormattedAddress string    `gorm:"size:500" json:"formatted_address,omitempty"`
	PlaceID          string    `gorm:"size:500" json:"place_id,omitempty"`
	Latitude         *float64  `json:"latitude,omitempty"`
	Longitude        *float64  `json:"longitude,omitempty"`
	StreetNumber     string    `gorm:"size:50" json:"street_number,omitempty"`
	StreetName       string    `gorm:"size:255" json:"street_name,omitempty"`
	City             string    `gorm:"size:255" json:"city,omitempty"`
	District         string    `gorm:"size:255" json:"district,omitempty"`
	StateCode        string    `gorm:"size:10" json:"state_code,omitempty"`
	StateName        string    `gorm:"size:255" json:"state_name,omitempty"`
	CountryCode      string    `gorm:"size:2" json:"country_code,omitempty"`
	CountryName      string    `gorm:"size:100" json:"country_name,omitempty"`
	PostalCode       string    `gorm:"size:20" json:"postal_code,omitempty"`
	ResponseJSON     JSONB     `gorm:"type:jsonb" json:"response_json,omitempty"`
	Provider         string    `gorm:"size:50;not null" json:"provider"`
	HitCount         int       `gorm:"default:0" json:"hit_count"`
	ExpiresAt        time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName returns the table name for AddressCacheEntry
func (AddressCacheEntry) TableName() string {
	return "address_cache"
}

// IsExpired checks if the cache entry has expired
func (e *AddressCacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Place represents a permanent geocoded place in our database
type Place struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ExternalPlaceID   string         `gorm:"size:500" json:"external_place_id,omitempty"`
	FormattedAddress  string         `gorm:"type:text;not null" json:"formatted_address"`
	Latitude          float64        `gorm:"type:decimal(10,8);not null" json:"latitude"`
	Longitude         float64        `gorm:"type:decimal(11,8);not null" json:"longitude"`
	Geohash           string         `gorm:"size:12" json:"geohash,omitempty"`
	StreetNumber      string         `gorm:"size:50" json:"street_number,omitempty"`
	StreetName        string         `gorm:"size:255" json:"street_name,omitempty"`
	City              string         `gorm:"size:255;index:idx_places_city,priority:1" json:"city,omitempty"`
	District          string         `gorm:"size:255" json:"district,omitempty"`
	StateCode         string         `gorm:"size:10" json:"state_code,omitempty"`
	StateName         string         `gorm:"size:255" json:"state_name,omitempty"`
	CountryCode       string         `gorm:"size:2;index:idx_places_city,priority:2" json:"country_code,omitempty"`
	CountryName       string         `gorm:"size:100" json:"country_name,omitempty"`
	PostalCode        string         `gorm:"size:20" json:"postal_code,omitempty"`
	PlaceTypes        pq.StringArray `gorm:"type:text[]" json:"place_types,omitempty"`
	SourceProvider    string         `gorm:"size:50" json:"source_provider,omitempty"`
	Confidence        *float64       `gorm:"type:decimal(3,2)" json:"confidence,omitempty"`
	Verified          bool           `gorm:"default:false" json:"verified"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         *time.Time     `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName returns the table name for Place
func (Place) TableName() string {
	return "places"
}

// GeoTagResult is the unified API response for geotag operations
type GeoTagResult struct {
	ID               string            `json:"id,omitempty"`
	FormattedAddress string            `json:"formatted_address"`
	Location         GeoTagLocation    `json:"location"`
	Components       AddressComponents `json:"components"`
	Metadata         GeoTagMetadata    `json:"metadata"`
}

// GeoTagLocation contains coordinates
type GeoTagLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// AddressComponents contains normalized address parts
type AddressComponents struct {
	StreetNumber string `json:"street_number,omitempty"`
	StreetName   string `json:"street_name,omitempty"`
	City         string `json:"city,omitempty"`
	District     string `json:"district,omitempty"`
	StateCode    string `json:"state_code,omitempty"`
	StateName    string `json:"state_name,omitempty"`
	CountryCode  string `json:"country_code,omitempty"`
	CountryName  string `json:"country_name,omitempty"`
	PostalCode   string `json:"postal_code,omitempty"`
}

// GeoTagMetadata contains additional information about the result
type GeoTagMetadata struct {
	Provider   string   `json:"provider"`
	Cached     bool     `json:"cached"`
	Confidence *float64 `json:"confidence,omitempty"`
	PlaceTypes []string `json:"place_types,omitempty"`
	PlaceID    string   `json:"place_id,omitempty"`
}

// CacheInfo contains information about cache status
type CacheInfo struct {
	Hit        bool  `json:"hit"`
	AgeSeconds int64 `json:"age_seconds,omitempty"`
}

// GeoTagAPIResponse is the standard API response wrapper
type GeoTagAPIResponse struct {
	Success   bool           `json:"success"`
	Data      interface{}    `json:"data,omitempty"`
	CacheInfo *CacheInfo     `json:"cache_info,omitempty"`
	Error     *APIError      `json:"error,omitempty"`
	Meta      *PaginationMeta `json:"meta,omitempty"`
}

// APIError represents an API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PaginationMeta contains pagination information
type PaginationMeta struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

// CacheStats holds cache statistics
type CacheStats struct {
	TotalEntries     int64            `json:"total_entries"`
	ValidEntries     int64            `json:"valid_entries"`
	ExpiredEntries   int64            `json:"expired_entries"`
	TotalHits        int64            `json:"total_hits"`
	ByType           map[string]int64 `json:"by_type"`
	AvgHitCount      float64          `json:"avg_hit_count"`
	OldestEntry      *time.Time       `json:"oldest_entry,omitempty"`
	NewestEntry      *time.Time       `json:"newest_entry,omitempty"`
	EstimatedSavings string           `json:"estimated_savings,omitempty"`
}

// PlacesStats holds statistics about stored places
type PlacesStats struct {
	TotalPlaces    int64            `json:"total_places"`
	VerifiedPlaces int64            `json:"verified_places"`
	ByCountry      map[string]int64 `json:"by_country"`
	ByProvider     map[string]int64 `json:"by_provider"`
}

// NearbyQuery contains parameters for nearby place searches
type NearbyQuery struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	RadiusKm  float64 `json:"radius_km" binding:"required,min=0.1,max=100"`
	Limit     int     `json:"limit" binding:"max=100"`
}

// SearchFilters contains filters for place searches
type SearchFilters struct {
	Query       string `json:"query,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	City        string `json:"city,omitempty"`
	StateCode   string `json:"state_code,omitempty"`
	PostalCode  string `json:"postal_code,omitempty"`
	Verified    *bool  `json:"verified,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
}

// BulkGeocodeRequest is the request for bulk geocoding
type BulkGeocodeRequest struct {
	Addresses []string `json:"addresses" binding:"required,min=1,max=100"`
}

// BulkGeocodeResult is the result of a bulk geocode operation
type BulkGeocodeResult struct {
	Index   int            `json:"index"`
	Address string         `json:"address"`
	Result  *GeoTagResult  `json:"result,omitempty"`
	Error   string         `json:"error,omitempty"`
	Cached  bool           `json:"cached"`
}

// ValidateAddressRequest is the request for address validation
type ValidateAddressRequest struct {
	Address string `json:"address" binding:"required"`
}

// JSONB is a custom type for JSONB columns
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// ToGeoTagResult converts a Place to a GeoTagResult
func (p *Place) ToGeoTagResult() *GeoTagResult {
	return &GeoTagResult{
		ID:               p.ID.String(),
		FormattedAddress: p.FormattedAddress,
		Location: GeoTagLocation{
			Latitude:  p.Latitude,
			Longitude: p.Longitude,
		},
		Components: AddressComponents{
			StreetNumber: p.StreetNumber,
			StreetName:   p.StreetName,
			City:         p.City,
			District:     p.District,
			StateCode:    p.StateCode,
			StateName:    p.StateName,
			CountryCode:  p.CountryCode,
			CountryName:  p.CountryName,
			PostalCode:   p.PostalCode,
		},
		Metadata: GeoTagMetadata{
			Provider:   p.SourceProvider,
			Cached:     true,
			Confidence: p.Confidence,
			PlaceTypes: p.PlaceTypes,
			PlaceID:    p.ExternalPlaceID,
		},
	}
}

// ToGeoTagResult converts a cache entry to a GeoTagResult
func (e *AddressCacheEntry) ToGeoTagResult() *GeoTagResult {
	result := &GeoTagResult{
		FormattedAddress: e.FormattedAddress,
		Components: AddressComponents{
			StreetNumber: e.StreetNumber,
			StreetName:   e.StreetName,
			City:         e.City,
			District:     e.District,
			StateCode:    e.StateCode,
			StateName:    e.StateName,
			CountryCode:  e.CountryCode,
			CountryName:  e.CountryName,
			PostalCode:   e.PostalCode,
		},
		Metadata: GeoTagMetadata{
			Provider: e.Provider,
			Cached:   true,
			PlaceID:  e.PlaceID,
		},
	}

	if e.Latitude != nil && e.Longitude != nil {
		result.Location = GeoTagLocation{
			Latitude:  *e.Latitude,
			Longitude: *e.Longitude,
		}
	}

	return result
}

// NewPlace creates a new Place from a GeocodingResult
func NewPlaceFromGeocodingResult(result *GeocodingResult, provider string) *Place {
	place := &Place{
		ID:               uuid.New(),
		FormattedAddress: result.FormattedAddress,
		Latitude:         result.Location.Latitude,
		Longitude:        result.Location.Longitude,
		PlaceTypes:       result.Types,
		SourceProvider:   provider,
		ExternalPlaceID:  result.PlaceID,
	}

	// Extract components
	for _, comp := range result.Components {
		switch comp.Type {
		case "street_number":
			place.StreetNumber = comp.LongName
		case "route":
			place.StreetName = comp.LongName
		case "locality":
			place.City = comp.LongName
		case "sublocality", "sublocality_level_1":
			place.District = comp.LongName
		case "administrative_area_level_1":
			place.StateName = comp.LongName
			place.StateCode = comp.ShortName
		case "country":
			place.CountryName = comp.LongName
			place.CountryCode = comp.ShortName
		case "postal_code":
			place.PostalCode = comp.LongName
		}
	}

	return place
}

// NewAddressCacheEntry creates a cache entry from a GeocodingResult
func NewAddressCacheEntryFromGeocodingResult(
	cacheType CacheType,
	cacheKey string,
	cacheKeyHash string,
	result *GeocodingResult,
	provider string,
	ttl time.Duration,
) *AddressCacheEntry {
	entry := &AddressCacheEntry{
		CacheType:        string(cacheType),
		CacheKey:         cacheKey,
		CacheKeyHash:     cacheKeyHash,
		FormattedAddress: result.FormattedAddress,
		PlaceID:          result.PlaceID,
		Latitude:         &result.Location.Latitude,
		Longitude:        &result.Location.Longitude,
		Provider:         provider,
		ExpiresAt:        time.Now().Add(ttl),
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

	return entry
}
