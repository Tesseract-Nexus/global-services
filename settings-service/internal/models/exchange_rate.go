package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ExchangeRate represents a currency exchange rate stored in the database
type ExchangeRate struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	BaseCurrency   string         `json:"base_currency" gorm:"type:varchar(3);not null;index:idx_exchange_rates_currencies"`
	TargetCurrency string         `json:"target_currency" gorm:"type:varchar(3);not null;index:idx_exchange_rates_currencies"`
	Rate           float64        `json:"rate" gorm:"type:decimal(20,10);not null"`
	FetchedAt      time.Time      `json:"fetched_at" gorm:"not null"`
	CreatedAt      time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName returns the table name for the ExchangeRate model
func (ExchangeRate) TableName() string {
	return "exchange_rates"
}

// SupportedCurrency represents a currency supported by the system
type SupportedCurrency struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	DecimalPlaces int    `json:"decimal_places"`
}

// CurrencyRateResponse represents the API response for exchange rates
type CurrencyRateResponse struct {
	Success bool                   `json:"success"`
	Base    string                 `json:"base"`
	Date    string                 `json:"date"`
	Rates   map[string]float64     `json:"rates"`
	Message string                 `json:"message,omitempty"`
}

// CurrencyConvertRequest represents a conversion request
type CurrencyConvertRequest struct {
	Amount float64 `json:"amount" binding:"required"`
	From   string  `json:"from" binding:"required,len=3"`
	To     string  `json:"to" binding:"required,len=3"`
}

// CurrencyConvertResponse represents the API response for currency conversion
type CurrencyConvertResponse struct {
	Success         bool    `json:"success"`
	OriginalAmount  float64 `json:"original_amount"`
	ConvertedAmount float64 `json:"converted_amount"`
	FromCurrency    string  `json:"from_currency"`
	ToCurrency      string  `json:"to_currency"`
	Rate            float64 `json:"rate"`
	RateDate        string  `json:"rate_date"`
	Message         string  `json:"message,omitempty"`
}

// SupportedCurrenciesResponse represents the API response for supported currencies
type SupportedCurrenciesResponse struct {
	Success    bool                `json:"success"`
	Currencies []SupportedCurrency `json:"currencies"`
	Message    string              `json:"message,omitempty"`
}

// BulkConvertRequest represents a bulk conversion request
type BulkConvertRequest struct {
	Amounts []BulkConvertItem `json:"amounts" binding:"required"`
	To      string            `json:"to" binding:"required,len=3"`
}

// BulkConvertItem represents a single item in a bulk conversion request
type BulkConvertItem struct {
	Amount float64 `json:"amount" binding:"required"`
	From   string  `json:"from" binding:"required,len=3"`
}

// BulkConvertResponse represents the API response for bulk conversion
type BulkConvertResponse struct {
	Success     bool                   `json:"success"`
	ToCurrency  string                 `json:"to_currency"`
	Conversions []BulkConvertResult    `json:"conversions"`
	TotalAmount float64                `json:"total_amount"`
	RateDate    string                 `json:"rate_date"`
	Message     string                 `json:"message,omitempty"`
}

// BulkConvertResult represents a single conversion result
type BulkConvertResult struct {
	OriginalAmount  float64 `json:"original_amount"`
	FromCurrency    string  `json:"from_currency"`
	ConvertedAmount float64 `json:"converted_amount"`
	Rate            float64 `json:"rate"`
}

// CurrencySymbols maps currency codes to their symbols
var CurrencySymbols = map[string]string{
	"USD": "$",
	"EUR": "€",
	"GBP": "£",
	"JPY": "¥",
	"AUD": "A$",
	"CAD": "C$",
	"CHF": "Fr",
	"CNY": "¥",
	"HKD": "HK$",
	"NZD": "NZ$",
	"SEK": "kr",
	"KRW": "₩",
	"SGD": "S$",
	"NOK": "kr",
	"MXN": "MX$",
	"INR": "₹",
	"RUB": "₽",
	"ZAR": "R",
	"TRY": "₺",
	"BRL": "R$",
	"TWD": "NT$",
	"DKK": "kr",
	"PLN": "zł",
	"THB": "฿",
	"IDR": "Rp",
	"HUF": "Ft",
	"CZK": "Kč",
	"ILS": "₪",
	"CLP": "CLP$",
	"PHP": "₱",
	"AED": "د.إ",
	"COP": "COL$",
	"SAR": "﷼",
	"MYR": "RM",
	"RON": "lei",
	"BGN": "лв",
	"ISK": "kr",
}

// CurrencyDecimalPlaces maps currency codes to their decimal places
var CurrencyDecimalPlaces = map[string]int{
	"JPY": 0,
	"KRW": 0,
	"HUF": 0,
	"TWD": 0,
	"ISK": 0,
	"CLP": 0,
}

// GetCurrencySymbol returns the symbol for a currency code
func GetCurrencySymbol(code string) string {
	if symbol, ok := CurrencySymbols[code]; ok {
		return symbol
	}
	return code
}

// GetCurrencyDecimalPlaces returns the decimal places for a currency code
func GetCurrencyDecimalPlaces(code string) int {
	if places, ok := CurrencyDecimalPlaces[code]; ok {
		return places
	}
	return 2 // Default to 2 decimal places
}
