package services

import (
	"context"
	"errors"
	"fmt"

	"location-service/internal/models"
	"location-service/internal/repository"
)

// ErrNoDatabase is returned when the service has no database connection
var ErrNoDatabase = errors.New("location service is unavailable: no database connection")

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
		return nil, 0, ErrNoDatabase
	}
	return s.countryRepo.GetAll(ctx, search, region, limit, offset)
}

// GetCountryByID retrieves a country by ID
func (s *LocationService) GetCountryByID(ctx context.Context, id string) (*models.Country, error) {
	if s.countryRepo == nil {
		return nil, ErrNoDatabase
	}
	return s.countryRepo.GetByID(ctx, id)
}

// CreateCountry creates a new country
func (s *LocationService) CreateCountry(ctx context.Context, country *models.Country) error {
	if s.countryRepo == nil {
		return ErrNoDatabase
	}
	return s.countryRepo.Create(ctx, country)
}

// UpdateCountry updates an existing country
func (s *LocationService) UpdateCountry(ctx context.Context, country *models.Country) error {
	if s.countryRepo == nil {
		return ErrNoDatabase
	}
	return s.countryRepo.Update(ctx, country)
}

// DeleteCountry deletes a country
func (s *LocationService) DeleteCountry(ctx context.Context, id string) error {
	if s.countryRepo == nil {
		return ErrNoDatabase
	}
	return s.countryRepo.Delete(ctx, id)
}

// ==================== STATE OPERATIONS ====================

// GetStates retrieves all states with optional filtering and pagination
func (s *LocationService) GetStates(ctx context.Context, search string, countryID string, limit, offset int) ([]models.State, int64, error) {
	if s.stateRepo == nil {
		return nil, 0, ErrNoDatabase
	}
	return s.stateRepo.GetAll(ctx, search, countryID, limit, offset)
}

// GetStatesByCountryID retrieves all states for a specific country
func (s *LocationService) GetStatesByCountryID(ctx context.Context, countryID string, search string) ([]models.State, error) {
	if s.stateRepo == nil {
		return nil, ErrNoDatabase
	}
	return s.stateRepo.GetByCountryID(ctx, countryID, search)
}

// GetStateByID retrieves a state by ID
func (s *LocationService) GetStateByID(ctx context.Context, id string) (*models.State, error) {
	if s.stateRepo == nil {
		return nil, ErrNoDatabase
	}
	return s.stateRepo.GetByID(ctx, id)
}

// CreateState creates a new state
func (s *LocationService) CreateState(ctx context.Context, state *models.State) error {
	if s.stateRepo == nil {
		return ErrNoDatabase
	}
	return s.stateRepo.Create(ctx, state)
}

// UpdateState updates an existing state
func (s *LocationService) UpdateState(ctx context.Context, state *models.State) error {
	if s.stateRepo == nil {
		return ErrNoDatabase
	}
	return s.stateRepo.Update(ctx, state)
}

// DeleteState deletes a state
func (s *LocationService) DeleteState(ctx context.Context, id string) error {
	if s.stateRepo == nil {
		return ErrNoDatabase
	}
	return s.stateRepo.Delete(ctx, id)
}

// ==================== CURRENCY OPERATIONS ====================

// GetCurrencies retrieves all currencies with optional filtering
func (s *LocationService) GetCurrencies(ctx context.Context, search string, activeOnly bool) ([]models.Currency, error) {
	if s.currencyRepo == nil {
		return nil, ErrNoDatabase
	}
	currencies, _, err := s.currencyRepo.GetAll(ctx, search, activeOnly, 0, 0)
	return currencies, err
}

// GetCurrenciesPaginated retrieves currencies with pagination
func (s *LocationService) GetCurrenciesPaginated(ctx context.Context, search string, activeOnly bool, limit, offset int) ([]models.Currency, int64, error) {
	if s.currencyRepo == nil {
		return nil, 0, ErrNoDatabase
	}
	return s.currencyRepo.GetAll(ctx, search, activeOnly, limit, offset)
}

// GetCurrencyByCode returns a specific currency by code
func (s *LocationService) GetCurrencyByCode(ctx context.Context, code string) (*models.Currency, error) {
	if s.currencyRepo == nil {
		return nil, ErrNoDatabase
	}
	return s.currencyRepo.GetByCode(ctx, code)
}

// CreateCurrency creates a new currency
func (s *LocationService) CreateCurrency(ctx context.Context, currency *models.Currency) error {
	if s.currencyRepo == nil {
		return ErrNoDatabase
	}
	return s.currencyRepo.Create(ctx, currency)
}

// UpdateCurrency updates an existing currency
func (s *LocationService) UpdateCurrency(ctx context.Context, currency *models.Currency) error {
	if s.currencyRepo == nil {
		return ErrNoDatabase
	}
	return s.currencyRepo.Update(ctx, currency)
}

// DeleteCurrency deletes a currency
func (s *LocationService) DeleteCurrency(ctx context.Context, code string) error {
	if s.currencyRepo == nil {
		return ErrNoDatabase
	}
	return s.currencyRepo.Delete(ctx, code)
}

// ==================== TIMEZONE OPERATIONS ====================

// GetTimezones retrieves all timezones with optional filtering
func (s *LocationService) GetTimezones(ctx context.Context, search string, countryID string) ([]models.Timezone, error) {
	if s.timezoneRepo == nil {
		return nil, ErrNoDatabase
	}
	timezones, _, err := s.timezoneRepo.GetAll(ctx, search, countryID, 0, 0)
	return timezones, err
}

// GetTimezonesPaginated retrieves timezones with pagination
func (s *LocationService) GetTimezonesPaginated(ctx context.Context, search string, countryID string, limit, offset int) ([]models.Timezone, int64, error) {
	if s.timezoneRepo == nil {
		return nil, 0, ErrNoDatabase
	}
	return s.timezoneRepo.GetAll(ctx, search, countryID, limit, offset)
}

// GetTimezoneByID returns a specific timezone by ID
func (s *LocationService) GetTimezoneByID(ctx context.Context, id string) (*models.Timezone, error) {
	if s.timezoneRepo == nil {
		return nil, ErrNoDatabase
	}
	return s.timezoneRepo.GetByID(ctx, id)
}

// CreateTimezone creates a new timezone
func (s *LocationService) CreateTimezone(ctx context.Context, timezone *models.Timezone) error {
	if s.timezoneRepo == nil {
		return ErrNoDatabase
	}
	return s.timezoneRepo.Create(ctx, timezone)
}

// UpdateTimezone updates an existing timezone
func (s *LocationService) UpdateTimezone(ctx context.Context, timezone *models.Timezone) error {
	if s.timezoneRepo == nil {
		return ErrNoDatabase
	}
	return s.timezoneRepo.Update(ctx, timezone)
}

// DeleteTimezone deletes a timezone
func (s *LocationService) DeleteTimezone(ctx context.Context, id string) error {
	if s.timezoneRepo == nil {
		return ErrNoDatabase
	}
	return s.timezoneRepo.Delete(ctx, id)
}

// ==================== CACHE OPERATIONS ====================

// GetCachedLocation retrieves cached location for an IP
func (s *LocationService) GetCachedLocation(ctx context.Context, ip string) (*models.LocationCache, error) {
	if s.cacheRepo == nil {
		return nil, fmt.Errorf("cache unavailable: %w", ErrNoDatabase)
	}
	return s.cacheRepo.GetByIP(ctx, ip)
}

// SetCachedLocation caches a location for an IP
func (s *LocationService) SetCachedLocation(ctx context.Context, cache *models.LocationCache) error {
	if s.cacheRepo == nil {
		return fmt.Errorf("cache unavailable: %w", ErrNoDatabase)
	}
	return s.cacheRepo.Set(ctx, cache)
}

// GetCacheStats returns cache statistics
func (s *LocationService) GetCacheStats(ctx context.Context) (*repository.CacheStats, error) {
	if s.cacheRepo == nil {
		return nil, fmt.Errorf("cache unavailable: %w", ErrNoDatabase)
	}
	return s.cacheRepo.GetStats(ctx)
}

// CleanupExpiredCache removes expired cache entries
func (s *LocationService) CleanupExpiredCache(ctx context.Context) (int64, error) {
	if s.cacheRepo == nil {
		return 0, fmt.Errorf("cache unavailable: %w", ErrNoDatabase)
	}
	return s.cacheRepo.DeleteExpired(ctx)
}

