package services

import (
	"context"
	"strings"

	"github.com/tesseract-hub/domains/common/services/location-service/internal/models"
	"github.com/tesseract-hub/domains/common/services/location-service/internal/repository"
)

// LocationService handles location-related business logic
type LocationService struct {
	countryRepo  repository.CountryRepository
	stateRepo    repository.StateRepository
	currencyRepo repository.CurrencyRepository
	timezoneRepo repository.TimezoneRepository
	cacheRepo    repository.LocationCacheRepository
}

// NewLocationService creates a new location service
func NewLocationService(
	countryRepo repository.CountryRepository,
	stateRepo repository.StateRepository,
	currencyRepo repository.CurrencyRepository,
	timezoneRepo repository.TimezoneRepository,
	cacheRepo repository.LocationCacheRepository,
) *LocationService {
	return &LocationService{
		countryRepo:  countryRepo,
		stateRepo:    stateRepo,
		currencyRepo: currencyRepo,
		timezoneRepo: timezoneRepo,
		cacheRepo:    cacheRepo,
	}
}

// ==================== COUNTRY OPERATIONS ====================

// GetCountries retrieves all countries with optional filtering and pagination
func (s *LocationService) GetCountries(ctx context.Context, search string, region string, limit, offset int) ([]models.Country, int64, error) {
	if s.countryRepo == nil {
		return s.getMockCountries(search, region, limit, offset), int64(len(s.getMockCountries("", "", 300, 0))), nil
	}
	return s.countryRepo.GetAll(ctx, search, region, limit, offset)
}

// GetCountryByID retrieves a country by ID
func (s *LocationService) GetCountryByID(ctx context.Context, id string) (*models.Country, error) {
	if s.countryRepo == nil {
		countries := s.getMockCountries("", "", 300, 0)
		for _, country := range countries {
			if country.ID == id {
				return &country, nil
			}
		}
		return nil, nil
	}
	return s.countryRepo.GetByID(ctx, id)
}

// CreateCountry creates a new country
func (s *LocationService) CreateCountry(ctx context.Context, country *models.Country) error {
	if s.countryRepo == nil {
		return nil
	}
	return s.countryRepo.Create(ctx, country)
}

// UpdateCountry updates an existing country
func (s *LocationService) UpdateCountry(ctx context.Context, country *models.Country) error {
	if s.countryRepo == nil {
		return nil
	}
	return s.countryRepo.Update(ctx, country)
}

// DeleteCountry deletes a country
func (s *LocationService) DeleteCountry(ctx context.Context, id string) error {
	if s.countryRepo == nil {
		return nil
	}
	return s.countryRepo.Delete(ctx, id)
}

// ==================== STATE OPERATIONS ====================

// GetStates retrieves all states with optional filtering and pagination
func (s *LocationService) GetStates(ctx context.Context, search string, countryID string, limit, offset int) ([]models.State, int64, error) {
	if s.stateRepo == nil {
		states := s.getMockStates(countryID, search, limit, offset)
		return states, int64(len(states)), nil
	}
	return s.stateRepo.GetAll(ctx, search, countryID, limit, offset)
}

// GetStatesByCountryID retrieves all states for a specific country
func (s *LocationService) GetStatesByCountryID(ctx context.Context, countryID string, search string) ([]models.State, error) {
	if s.stateRepo == nil {
		return s.getMockStates(countryID, search, 100, 0), nil
	}
	return s.stateRepo.GetByCountryID(ctx, countryID, search)
}

// GetStateByID retrieves a state by ID
func (s *LocationService) GetStateByID(ctx context.Context, id string) (*models.State, error) {
	if s.stateRepo == nil {
		states := s.getMockStates("", "", 500, 0)
		for _, state := range states {
			if state.ID == id {
				return &state, nil
			}
		}
		return nil, nil
	}
	return s.stateRepo.GetByID(ctx, id)
}

// CreateState creates a new state
func (s *LocationService) CreateState(ctx context.Context, state *models.State) error {
	if s.stateRepo == nil {
		return nil
	}
	return s.stateRepo.Create(ctx, state)
}

// UpdateState updates an existing state
func (s *LocationService) UpdateState(ctx context.Context, state *models.State) error {
	if s.stateRepo == nil {
		return nil
	}
	return s.stateRepo.Update(ctx, state)
}

// DeleteState deletes a state
func (s *LocationService) DeleteState(ctx context.Context, id string) error {
	if s.stateRepo == nil {
		return nil
	}
	return s.stateRepo.Delete(ctx, id)
}

// ==================== CURRENCY OPERATIONS ====================

// GetCurrencies retrieves all currencies with optional filtering
func (s *LocationService) GetCurrencies(ctx context.Context, search string, activeOnly bool) ([]models.Currency, error) {
	if s.currencyRepo == nil {
		return s.getMockCurrencies(search, activeOnly), nil
	}
	currencies, _, err := s.currencyRepo.GetAll(ctx, search, activeOnly, 0, 0)
	return currencies, err
}

// GetCurrenciesPaginated retrieves currencies with pagination
func (s *LocationService) GetCurrenciesPaginated(ctx context.Context, search string, activeOnly bool, limit, offset int) ([]models.Currency, int64, error) {
	if s.currencyRepo == nil {
		currencies := s.getMockCurrencies(search, activeOnly)
		return currencies, int64(len(currencies)), nil
	}
	return s.currencyRepo.GetAll(ctx, search, activeOnly, limit, offset)
}

// GetCurrencyByCode returns a specific currency by code
func (s *LocationService) GetCurrencyByCode(ctx context.Context, code string) (*models.Currency, error) {
	if s.currencyRepo == nil {
		currencies := s.getMockCurrencies("", false)
		for _, currency := range currencies {
			if currency.Code == code {
				return &currency, nil
			}
		}
		return nil, nil
	}
	return s.currencyRepo.GetByCode(ctx, code)
}

// CreateCurrency creates a new currency
func (s *LocationService) CreateCurrency(ctx context.Context, currency *models.Currency) error {
	if s.currencyRepo == nil {
		return nil
	}
	return s.currencyRepo.Create(ctx, currency)
}

// UpdateCurrency updates an existing currency
func (s *LocationService) UpdateCurrency(ctx context.Context, currency *models.Currency) error {
	if s.currencyRepo == nil {
		return nil
	}
	return s.currencyRepo.Update(ctx, currency)
}

// DeleteCurrency deletes a currency
func (s *LocationService) DeleteCurrency(ctx context.Context, code string) error {
	if s.currencyRepo == nil {
		return nil
	}
	return s.currencyRepo.Delete(ctx, code)
}

// ==================== TIMEZONE OPERATIONS ====================

// GetTimezones retrieves all timezones with optional filtering
func (s *LocationService) GetTimezones(ctx context.Context, search string, countryID string) ([]models.Timezone, error) {
	if s.timezoneRepo == nil {
		return s.getMockTimezones(search, countryID), nil
	}
	timezones, _, err := s.timezoneRepo.GetAll(ctx, search, countryID, 0, 0)
	return timezones, err
}

// GetTimezonesPaginated retrieves timezones with pagination
func (s *LocationService) GetTimezonesPaginated(ctx context.Context, search string, countryID string, limit, offset int) ([]models.Timezone, int64, error) {
	if s.timezoneRepo == nil {
		timezones := s.getMockTimezones(search, countryID)
		return timezones, int64(len(timezones)), nil
	}
	return s.timezoneRepo.GetAll(ctx, search, countryID, limit, offset)
}

// GetTimezoneByID returns a specific timezone by ID
func (s *LocationService) GetTimezoneByID(ctx context.Context, id string) (*models.Timezone, error) {
	if s.timezoneRepo == nil {
		timezones := s.getMockTimezones("", "")
		for _, tz := range timezones {
			if tz.ID == id {
				return &tz, nil
			}
		}
		return nil, nil
	}
	return s.timezoneRepo.GetByID(ctx, id)
}

// CreateTimezone creates a new timezone
func (s *LocationService) CreateTimezone(ctx context.Context, timezone *models.Timezone) error {
	if s.timezoneRepo == nil {
		return nil
	}
	return s.timezoneRepo.Create(ctx, timezone)
}

// UpdateTimezone updates an existing timezone
func (s *LocationService) UpdateTimezone(ctx context.Context, timezone *models.Timezone) error {
	if s.timezoneRepo == nil {
		return nil
	}
	return s.timezoneRepo.Update(ctx, timezone)
}

// DeleteTimezone deletes a timezone
func (s *LocationService) DeleteTimezone(ctx context.Context, id string) error {
	if s.timezoneRepo == nil {
		return nil
	}
	return s.timezoneRepo.Delete(ctx, id)
}

// ==================== CACHE OPERATIONS ====================

// GetCachedLocation retrieves cached location for an IP
func (s *LocationService) GetCachedLocation(ctx context.Context, ip string) (*models.LocationCache, error) {
	if s.cacheRepo == nil {
		return nil, nil
	}
	return s.cacheRepo.GetByIP(ctx, ip)
}

// SetCachedLocation caches a location for an IP
func (s *LocationService) SetCachedLocation(ctx context.Context, cache *models.LocationCache) error {
	if s.cacheRepo == nil {
		return nil
	}
	return s.cacheRepo.Set(ctx, cache)
}

// GetCacheStats returns cache statistics
func (s *LocationService) GetCacheStats(ctx context.Context) (*repository.CacheStats, error) {
	if s.cacheRepo == nil {
		return &repository.CacheStats{}, nil
	}
	return s.cacheRepo.GetStats(ctx)
}

// CleanupExpiredCache removes expired cache entries
func (s *LocationService) CleanupExpiredCache(ctx context.Context) (int64, error) {
	if s.cacheRepo == nil {
		return 0, nil
	}
	return s.cacheRepo.DeleteExpired(ctx)
}

// ==================== HELPER FUNCTIONS ====================

// Helper function to check if a string contains a substring (case insensitive)
func contains(str, substr string) bool {
	if substr == "" {
		return true
	}
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

// getMockCountries returns mock country data
func (s *LocationService) getMockCountries(search string, region string, limit, offset int) []models.Country {
	countries := []models.Country{
		{ID: "US", Name: "United States", Region: "Americas", Currency: "USD", CallingCode: "+1", FlagEmoji: "🇺🇸", Active: true},
		{ID: "CA", Name: "Canada", Region: "Americas", Currency: "CAD", CallingCode: "+1", FlagEmoji: "🇨🇦", Active: true},
		{ID: "GB", Name: "United Kingdom", Region: "Europe", Currency: "GBP", CallingCode: "+44", FlagEmoji: "🇬🇧", Active: true},
		{ID: "AU", Name: "Australia", Region: "Oceania", Currency: "AUD", CallingCode: "+61", FlagEmoji: "🇦🇺", Active: true},
		{ID: "IN", Name: "India", Region: "Asia", Currency: "INR", CallingCode: "+91", FlagEmoji: "🇮🇳", Active: true},
		{ID: "DE", Name: "Germany", Region: "Europe", Currency: "EUR", CallingCode: "+49", FlagEmoji: "🇩🇪", Active: true},
		{ID: "FR", Name: "France", Region: "Europe", Currency: "EUR", CallingCode: "+33", FlagEmoji: "🇫🇷", Active: true},
		{ID: "JP", Name: "Japan", Region: "Asia", Currency: "JPY", CallingCode: "+81", FlagEmoji: "🇯🇵", Active: true},
		{ID: "SG", Name: "Singapore", Region: "Asia", Currency: "SGD", CallingCode: "+65", FlagEmoji: "🇸🇬", Active: true},
		{ID: "AE", Name: "United Arab Emirates", Region: "Asia", Currency: "AED", CallingCode: "+971", FlagEmoji: "🇦🇪", Active: true},
	}

	// Apply filters
	var filtered []models.Country
	for _, country := range countries {
		if search != "" && !contains(country.Name, search) && !contains(country.ID, search) {
			continue
		}
		if region != "" && country.Region != region {
			continue
		}
		filtered = append(filtered, country)
	}

	// Apply pagination
	if offset >= len(filtered) {
		return []models.Country{}
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end]
}

// getMockStates returns mock state data
func (s *LocationService) getMockStates(countryID string, search string, limit, offset int) []models.State {
	allStates := []models.State{
		{ID: "US-CA", Name: "California", Code: "CA", CountryID: "US", Type: "state", Active: true},
		{ID: "US-NY", Name: "New York", Code: "NY", CountryID: "US", Type: "state", Active: true},
		{ID: "US-TX", Name: "Texas", Code: "TX", CountryID: "US", Type: "state", Active: true},
		{ID: "US-FL", Name: "Florida", Code: "FL", CountryID: "US", Type: "state", Active: true},
		{ID: "CA-ON", Name: "Ontario", Code: "ON", CountryID: "CA", Type: "province", Active: true},
		{ID: "CA-QC", Name: "Quebec", Code: "QC", CountryID: "CA", Type: "province", Active: true},
		{ID: "CA-BC", Name: "British Columbia", Code: "BC", CountryID: "CA", Type: "province", Active: true},
		{ID: "AU-NSW", Name: "New South Wales", Code: "NSW", CountryID: "AU", Type: "state", Active: true},
		{ID: "AU-VIC", Name: "Victoria", Code: "VIC", CountryID: "AU", Type: "state", Active: true},
		{ID: "AU-QLD", Name: "Queensland", Code: "QLD", CountryID: "AU", Type: "state", Active: true},
		{ID: "IN-MH", Name: "Maharashtra", Code: "MH", CountryID: "IN", Type: "state", Active: true},
		{ID: "IN-KA", Name: "Karnataka", Code: "KA", CountryID: "IN", Type: "state", Active: true},
		{ID: "IN-DL", Name: "Delhi", Code: "DL", CountryID: "IN", Type: "union territory", Active: true},
		{ID: "GB-ENG", Name: "England", Code: "ENG", CountryID: "GB", Type: "country", Active: true},
		{ID: "GB-SCT", Name: "Scotland", Code: "SCT", CountryID: "GB", Type: "country", Active: true},
	}

	// Apply filters
	var filtered []models.State
	for _, state := range allStates {
		if countryID != "" && state.CountryID != countryID {
			continue
		}
		if search != "" && !contains(state.Name, search) && !contains(state.Code, search) {
			continue
		}
		filtered = append(filtered, state)
	}

	// Apply pagination
	if offset >= len(filtered) {
		return []models.State{}
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end]
}

// getMockCurrencies returns mock currency data
func (s *LocationService) getMockCurrencies(search string, activeOnly bool) []models.Currency {
	currencies := []models.Currency{
		{Code: "USD", Name: "US Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "EUR", Name: "Euro", Symbol: "€", DecimalPlaces: 2, Active: true},
		{Code: "GBP", Name: "British Pound", Symbol: "£", DecimalPlaces: 2, Active: true},
		{Code: "JPY", Name: "Japanese Yen", Symbol: "¥", DecimalPlaces: 0, Active: true},
		{Code: "CAD", Name: "Canadian Dollar", Symbol: "C$", DecimalPlaces: 2, Active: true},
		{Code: "AUD", Name: "Australian Dollar", Symbol: "A$", DecimalPlaces: 2, Active: true},
		{Code: "INR", Name: "Indian Rupee", Symbol: "₹", DecimalPlaces: 2, Active: true},
		{Code: "SGD", Name: "Singapore Dollar", Symbol: "S$", DecimalPlaces: 2, Active: true},
		{Code: "AED", Name: "UAE Dirham", Symbol: "د.إ", DecimalPlaces: 2, Active: true},
		{Code: "CHF", Name: "Swiss Franc", Symbol: "CHF", DecimalPlaces: 2, Active: true},
	}

	var filtered []models.Currency
	for _, currency := range currencies {
		if activeOnly && !currency.Active {
			continue
		}
		if search != "" && !contains(currency.Name, search) && !contains(currency.Code, search) {
			continue
		}
		filtered = append(filtered, currency)
	}

	return filtered
}

// getMockTimezones returns mock timezone data
func (s *LocationService) getMockTimezones(search string, countryID string) []models.Timezone {
	timezones := []models.Timezone{
		{ID: "America/New_York", Name: "Eastern Time", Abbreviation: "EST/EDT", Offset: "-05:00", DST: true, Countries: `["US","CA"]`},
		{ID: "America/Chicago", Name: "Central Time", Abbreviation: "CST/CDT", Offset: "-06:00", DST: true, Countries: `["US","CA","MX"]`},
		{ID: "America/Denver", Name: "Mountain Time", Abbreviation: "MST/MDT", Offset: "-07:00", DST: true, Countries: `["US","CA"]`},
		{ID: "America/Los_Angeles", Name: "Pacific Time", Abbreviation: "PST/PDT", Offset: "-08:00", DST: true, Countries: `["US","CA"]`},
		{ID: "Europe/London", Name: "Greenwich Mean Time", Abbreviation: "GMT/BST", Offset: "+00:00", DST: true, Countries: `["GB"]`},
		{ID: "Europe/Paris", Name: "Central European Time", Abbreviation: "CET/CEST", Offset: "+01:00", DST: true, Countries: `["FR","DE","IT"]`},
		{ID: "Asia/Tokyo", Name: "Japan Standard Time", Abbreviation: "JST", Offset: "+09:00", DST: false, Countries: `["JP"]`},
		{ID: "Asia/Kolkata", Name: "India Standard Time", Abbreviation: "IST", Offset: "+05:30", DST: false, Countries: `["IN"]`},
		{ID: "Asia/Singapore", Name: "Singapore Time", Abbreviation: "SGT", Offset: "+08:00", DST: false, Countries: `["SG"]`},
		{ID: "Asia/Dubai", Name: "Gulf Standard Time", Abbreviation: "GST", Offset: "+04:00", DST: false, Countries: `["AE"]`},
		{ID: "Australia/Sydney", Name: "Australian Eastern Time", Abbreviation: "AEST/AEDT", Offset: "+10:00", DST: true, Countries: `["AU"]`},
	}

	var filtered []models.Timezone
	for _, tz := range timezones {
		if search != "" && !contains(tz.Name, search) && !contains(tz.ID, search) {
			continue
		}
		if countryID != "" && !contains(tz.Countries, countryID) {
			continue
		}
		filtered = append(filtered, tz)
	}

	return filtered
}
