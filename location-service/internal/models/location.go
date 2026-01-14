package models

import (
	"time"

	"gorm.io/gorm"
)

// Country represents a country in the database
type Country struct {
	ID          string         `gorm:"primaryKey;size:2" json:"id"` // ISO 3166-1 alpha-2
	Name        string         `gorm:"size:100;not null" json:"name"`
	NativeName  string         `gorm:"size:100" json:"native_name"`
	Capital     string         `gorm:"size:100" json:"capital"`
	Region      string         `gorm:"size:50" json:"region"`
	Subregion   string         `gorm:"size:50" json:"subregion"`
	Currency    string         `gorm:"size:3" json:"currency"`     // ISO 4217
	Languages   string         `gorm:"type:text" json:"languages"` // JSON array as string
	CallingCode string         `gorm:"size:10" json:"calling_code"`
	FlagEmoji   string         `gorm:"size:10" json:"flag_emoji"`
	Latitude    *float64       `json:"latitude"`
	Longitude   *float64       `json:"longitude"`
	Active      bool           `gorm:"default:true" json:"active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	States []State `gorm:"foreignKey:CountryID" json:"states,omitempty"`
}

// State represents a state/province in the database
type State struct {
	ID         string         `gorm:"primaryKey;size:10" json:"id"` // Country-State format: US-CA
	Name       string         `gorm:"size:100;not null" json:"name"`
	NativeName string         `gorm:"size:100" json:"native_name"`
	Code       string         `gorm:"size:10;not null" json:"code"` // State/province code
	CountryID  string         `gorm:"size:2;not null" json:"country_id"`
	Type       string         `gorm:"size:20;default:'state'" json:"type"` // state, province, territory, etc.
	Latitude   *float64       `json:"latitude"`
	Longitude  *float64       `json:"longitude"`
	Active     bool           `gorm:"default:true" json:"active"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Country Country `gorm:"foreignKey:CountryID" json:"country,omitempty"`
}

// Currency represents a currency in the database
type Currency struct {
	Code          string         `gorm:"primaryKey;size:3" json:"code"` // ISO 4217
	Name          string         `gorm:"size:100;not null" json:"name"`
	Symbol        string         `gorm:"size:10;not null" json:"symbol"`
	DecimalPlaces int            `gorm:"default:2" json:"decimal_places"`
	Active        bool           `gorm:"default:true" json:"active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// Timezone represents a timezone in the database
type Timezone struct {
	ID           string         `gorm:"primaryKey;size:50" json:"id"` // e.g., America/New_York
	Name         string         `gorm:"size:100;not null" json:"name"`
	Abbreviation string         `gorm:"size:10" json:"abbreviation"`
	Offset       string         `gorm:"size:10;not null" json:"offset"` // e.g., -05:00
	DST          bool           `gorm:"default:false" json:"dst"`
	Countries    string         `gorm:"type:text" json:"countries"` // JSON array as string
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// LocationCache represents cached location data for IP addresses
type LocationCache struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	IP         string    `gorm:"uniqueIndex;size:45;not null" json:"ip"` // Supports IPv4 and IPv6
	CountryID  string    `gorm:"size:2" json:"country_id"`
	StateID    string    `gorm:"size:10" json:"state_id"`
	City       string    `gorm:"size:100" json:"city"`
	PostalCode string    `gorm:"size:20" json:"postal_code"`
	Latitude   *float64  `json:"latitude"`
	Longitude  *float64  `json:"longitude"`
	TimezoneID string    `gorm:"size:50" json:"timezone_id"`
	ISP        string    `gorm:"size:200" json:"isp"`
	ExpiresAt  time.Time `gorm:"index" json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Relationships
	Country  *Country  `gorm:"foreignKey:CountryID" json:"country,omitempty"`
	State    *State    `gorm:"foreignKey:StateID" json:"state,omitempty"`
	Timezone *Timezone `gorm:"foreignKey:TimezoneID" json:"timezone,omitempty"`
}

// TableName specifies the table name for LocationCache
func (LocationCache) TableName() string {
	return "location_cache"
}

// AddressComponent represents a component of a parsed address
type AddressComponent struct {
	Type      string `json:"type"` // street_number, route, locality, administrative_area_level_1, country, postal_code
	LongName  string `json:"long_name"`
	ShortName string `json:"short_name"`
}

// Address represents a complete address with all components
type Address struct {
	ID               uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	FormattedAddress string         `gorm:"size:500;not null" json:"formatted_address"`
	StreetNumber     string         `gorm:"size:20" json:"street_number"`
	Route            string         `gorm:"size:200" json:"route"`              // Street name
	Locality         string         `gorm:"size:100" json:"locality"`           // City
	Sublocality      string         `gorm:"size:100" json:"sublocality"`        // Neighborhood/District
	AdminAreaLevel1  string         `gorm:"size:100" json:"admin_area_level_1"` // State/Province
	AdminAreaLevel2  string         `gorm:"size:100" json:"admin_area_level_2"` // County
	CountryID        string         `gorm:"size:2" json:"country_id"`
	CountryName      string         `gorm:"size:100" json:"country_name"`
	PostalCode       string         `gorm:"size:20" json:"postal_code"`
	Latitude         *float64       `json:"latitude"`
	Longitude        *float64       `json:"longitude"`
	PlaceID          string         `gorm:"size:500;index" json:"place_id"` // Google Place ID or similar
	Types            string         `gorm:"type:text" json:"types"`         // JSON array of address types
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Country *Country `gorm:"foreignKey:CountryID" json:"country,omitempty"`
}

// TableName specifies the table name for Address
func (Address) TableName() string {
	return "addresses"
}

// AddressSuggestion represents an address autocomplete suggestion
type AddressSuggestion struct {
	PlaceID           string   `json:"place_id"`
	Description       string   `json:"description"`
	MainText          string   `json:"main_text"`
	SecondaryText     string   `json:"secondary_text"`
	Types             []string `json:"types"`
	MatchedSubstrings []struct {
		Offset int `json:"offset"`
		Length int `json:"length"`
	} `json:"matched_substrings,omitempty"`
}

// GeocodingResult represents the result of geocoding an address
type GeocodingResult struct {
	FormattedAddress string             `json:"formatted_address"`
	PlaceID          string             `json:"place_id"`
	Location         GeoLocation        `json:"location"`
	Components       []AddressComponent `json:"components"`
	Types            []string           `json:"types"`
	Viewport         *Viewport          `json:"viewport,omitempty"`
}

// GeoLocation represents a geographic coordinate
type GeoLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Viewport represents the recommended viewport for displaying a location
type Viewport struct {
	Northeast GeoLocation `json:"northeast"`
	Southwest GeoLocation `json:"southwest"`
}

// ReverseGeocodingResult represents the result of reverse geocoding coordinates
type ReverseGeocodingResult struct {
	FormattedAddress string             `json:"formatted_address"`
	PlaceID          string             `json:"place_id"`
	Components       []AddressComponent `json:"components"`
	Types            []string           `json:"types"`
}

// AddressValidationResult represents the result of address validation
type AddressValidationResult struct {
	Valid            bool               `json:"valid"`
	FormattedAddress string             `json:"formatted_address,omitempty"`
	Components       []AddressComponent `json:"components,omitempty"`
	Location         *GeoLocation       `json:"location,omitempty"`
	Deliverable      bool               `json:"deliverable"`
	Issues           []string           `json:"issues,omitempty"`
	Suggestions      []string           `json:"suggestions,omitempty"`
}
