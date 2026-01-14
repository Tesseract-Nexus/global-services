package services

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/tesseract-hub/settings-service/internal/cache"
	"github.com/tesseract-hub/settings-service/internal/clients/frankfurter"
	"github.com/tesseract-hub/settings-service/internal/models"
	"github.com/tesseract-hub/settings-service/internal/repository"
)

// CurrencyService defines the interface for currency operations
type CurrencyService interface {
	// Convert converts an amount from one currency to another
	Convert(amount float64, fromCurrency, toCurrency string) (float64, error)

	// GetRate retrieves the exchange rate between two currencies
	GetRate(fromCurrency, toCurrency string) (float64, error)

	// GetAllRates retrieves all exchange rates for a base currency
	GetAllRates(baseCurrency string) (map[string]float64, error)

	// GetSupportedCurrencies returns a list of supported currencies
	GetSupportedCurrencies() ([]models.SupportedCurrency, error)

	// RefreshRates fetches and stores the latest exchange rates
	RefreshRates() error

	// BulkConvert converts multiple amounts to a single currency
	BulkConvert(items []models.BulkConvertItem, toCurrency string) (*models.BulkConvertResponse, error)

	// GetRateDate returns the date of the current rates
	GetRateDate() (string, error)
}

type currencyService struct {
	frankfurterClient *frankfurter.Client
	rateRepo          repository.ExchangeRateRepository
	cache             *cache.CurrencyCache
	baseCurrency      string // Default base currency for storing rates
}

// NewCurrencyService creates a new currency service
func NewCurrencyService(
	frankfurterClient *frankfurter.Client,
	rateRepo repository.ExchangeRateRepository,
	currencyCache *cache.CurrencyCache,
) CurrencyService {
	return &currencyService{
		frankfurterClient: frankfurterClient,
		rateRepo:          rateRepo,
		cache:             currencyCache,
		baseCurrency:      "EUR", // ECB uses EUR as base
	}
}

// Convert converts an amount from one currency to another
func (s *currencyService) Convert(amount float64, fromCurrency, toCurrency string) (float64, error) {
	fromCurrency = strings.ToUpper(fromCurrency)
	toCurrency = strings.ToUpper(toCurrency)

	// Same currency, no conversion needed
	if fromCurrency == toCurrency {
		return amount, nil
	}

	rate, err := s.GetRate(fromCurrency, toCurrency)
	if err != nil {
		return 0, err
	}

	return amount * rate, nil
}

// GetRate retrieves the exchange rate between two currencies
func (s *currencyService) GetRate(fromCurrency, toCurrency string) (float64, error) {
	fromCurrency = strings.ToUpper(fromCurrency)
	toCurrency = strings.ToUpper(toCurrency)

	// Same currency
	if fromCurrency == toCurrency {
		return 1.0, nil
	}

	// Try cache first
	if cachedRate, ok := s.cache.GetRate(fromCurrency, toCurrency); ok {
		return cachedRate.Rate, nil
	}

	// Try database
	dbRate, err := s.rateRepo.GetRate(fromCurrency, toCurrency)
	if err == nil {
		// Cache the rate
		s.cache.SetRate(fromCurrency, toCurrency, dbRate.Rate, dbRate.FetchedAt)
		return dbRate.Rate, nil
	}

	// Calculate cross rate if direct rate not found
	// Try: from -> EUR -> to
	rate, err := s.calculateCrossRate(fromCurrency, toCurrency)
	if err == nil {
		return rate, nil
	}

	// Fetch from API as last resort
	return s.fetchAndCacheRate(fromCurrency, toCurrency)
}

// calculateCrossRate calculates a rate through the base currency (EUR)
func (s *currencyService) calculateCrossRate(fromCurrency, toCurrency string) (float64, error) {
	// Get rate from -> EUR
	fromToBase, err := s.getDirectOrDBRate(fromCurrency, s.baseCurrency)
	if err != nil {
		return 0, err
	}

	// Get rate EUR -> to
	baseToTo, err := s.getDirectOrDBRate(s.baseCurrency, toCurrency)
	if err != nil {
		return 0, err
	}

	return fromToBase * baseToTo, nil
}

// getDirectOrDBRate gets a rate from cache or database
func (s *currencyService) getDirectOrDBRate(from, to string) (float64, error) {
	// Same currency
	if from == to {
		return 1.0, nil
	}

	// Try cache
	if cachedRate, ok := s.cache.GetRate(from, to); ok {
		return cachedRate.Rate, nil
	}

	// Try database
	dbRate, err := s.rateRepo.GetRate(from, to)
	if err != nil {
		// Try inverse rate
		inverseRate, err := s.rateRepo.GetRate(to, from)
		if err != nil {
			return 0, fmt.Errorf("rate not found for %s/%s", from, to)
		}
		return 1.0 / inverseRate.Rate, nil
	}

	return dbRate.Rate, nil
}

// fetchAndCacheRate fetches a rate from the API and caches it
func (s *currencyService) fetchAndCacheRate(from, to string) (float64, error) {
	resp, err := s.frankfurterClient.Convert(1.0, from, to)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch rate from API: %w", err)
	}

	rate, ok := resp.Rates[to]
	if !ok {
		return 0, fmt.Errorf("rate for %s not found in API response", to)
	}

	// Cache the rate
	now := time.Now()
	s.cache.SetRate(from, to, rate, now)

	// Store in database
	s.rateRepo.UpsertRate(&models.ExchangeRate{
		BaseCurrency:   from,
		TargetCurrency: to,
		Rate:           rate,
		FetchedAt:      now,
	})

	return rate, nil
}

// GetAllRates retrieves all exchange rates for a base currency
func (s *currencyService) GetAllRates(baseCurrency string) (map[string]float64, error) {
	baseCurrency = strings.ToUpper(baseCurrency)

	// Try to get from database first
	dbRates, err := s.rateRepo.GetRatesForBase(baseCurrency)
	if err == nil && len(dbRates) > 0 {
		rates := make(map[string]float64)
		for _, r := range dbRates {
			rates[r.TargetCurrency] = r.Rate
		}
		return rates, nil
	}

	// Fetch from API
	resp, err := s.frankfurterClient.GetLatestRates(baseCurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch rates: %w", err)
	}

	return resp.Rates, nil
}

// GetSupportedCurrencies returns a list of supported currencies
func (s *currencyService) GetSupportedCurrencies() ([]models.SupportedCurrency, error) {
	currencies, err := s.frankfurterClient.GetSupportedCurrencies()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch supported currencies: %w", err)
	}

	result := make([]models.SupportedCurrency, 0, len(currencies))
	for _, c := range currencies {
		result = append(result, models.SupportedCurrency{
			Code:          c.Code,
			Name:          c.Name,
			Symbol:        models.GetCurrencySymbol(c.Code),
			DecimalPlaces: models.GetCurrencyDecimalPlaces(c.Code),
		})
	}

	// Sort by code
	sort.Slice(result, func(i, j int) bool {
		return result[i].Code < result[j].Code
	})

	return result, nil
}

// RefreshRates fetches and stores the latest exchange rates
func (s *currencyService) RefreshRates() error {
	log.Println("Refreshing exchange rates from Frankfurter.app...")

	// Fetch latest rates with EUR as base (ECB standard)
	resp, err := s.frankfurterClient.GetLatestRates(s.baseCurrency)
	if err != nil {
		return fmt.Errorf("failed to fetch rates: %w", err)
	}

	now := time.Now()
	rates := make([]models.ExchangeRate, 0, len(resp.Rates)*2)

	// Store rates from EUR to all currencies
	for targetCurrency, rate := range resp.Rates {
		rates = append(rates, models.ExchangeRate{
			BaseCurrency:   s.baseCurrency,
			TargetCurrency: targetCurrency,
			Rate:           rate,
			FetchedAt:      now,
		})

		// Also store inverse rate
		if rate > 0 {
			rates = append(rates, models.ExchangeRate{
				BaseCurrency:   targetCurrency,
				TargetCurrency: s.baseCurrency,
				Rate:           1.0 / rate,
				FetchedAt:      now,
			})
		}

		// Cache the rates
		s.cache.SetRate(s.baseCurrency, targetCurrency, rate, now)
		if rate > 0 {
			s.cache.SetRate(targetCurrency, s.baseCurrency, 1.0/rate, now)
		}
	}

	// Bulk upsert to database
	if err := s.rateRepo.BulkUpsertRates(rates); err != nil {
		return fmt.Errorf("failed to store rates: %w", err)
	}

	log.Printf("Successfully refreshed %d exchange rates", len(rates))
	return nil
}

// BulkConvert converts multiple amounts to a single currency
func (s *currencyService) BulkConvert(items []models.BulkConvertItem, toCurrency string) (*models.BulkConvertResponse, error) {
	toCurrency = strings.ToUpper(toCurrency)

	conversions := make([]models.BulkConvertResult, 0, len(items))
	var totalAmount float64

	for _, item := range items {
		fromCurrency := strings.ToUpper(item.From)

		rate, err := s.GetRate(fromCurrency, toCurrency)
		if err != nil {
			return nil, fmt.Errorf("failed to get rate for %s to %s: %w", fromCurrency, toCurrency, err)
		}

		convertedAmount := item.Amount * rate
		totalAmount += convertedAmount

		conversions = append(conversions, models.BulkConvertResult{
			OriginalAmount:  item.Amount,
			FromCurrency:    fromCurrency,
			ConvertedAmount: convertedAmount,
			Rate:            rate,
		})
	}

	rateDate, _ := s.GetRateDate()

	return &models.BulkConvertResponse{
		Success:     true,
		ToCurrency:  toCurrency,
		Conversions: conversions,
		TotalAmount: totalAmount,
		RateDate:    rateDate,
	}, nil
}

// GetRateDate returns the date of the current rates
func (s *currencyService) GetRateDate() (string, error) {
	fetchTime, err := s.rateRepo.GetLatestFetchTime()
	if err != nil || fetchTime == nil {
		return time.Now().Format("2006-01-02"), nil
	}
	return fetchTime.Format("2006-01-02"), nil
}
