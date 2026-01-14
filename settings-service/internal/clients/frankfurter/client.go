package frankfurter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.frankfurter.app"
	DefaultTimeout = 10 * time.Second
)

// Client is an HTTP client for the Frankfurter currency exchange API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// LatestRatesResponse represents the response from the /latest endpoint
type LatestRatesResponse struct {
	Amount float64            `json:"amount"`
	Base   string             `json:"base"`
	Date   string             `json:"date"`
	Rates  map[string]float64 `json:"rates"`
}

// CurrenciesResponse represents the response from the /currencies endpoint
type CurrenciesResponse map[string]string

// ConvertResponse represents the response from a conversion request
type ConvertResponse struct {
	Amount float64            `json:"amount"`
	Base   string             `json:"base"`
	Date   string             `json:"date"`
	Rates  map[string]float64 `json:"rates"`
}

// Currency represents a supported currency
type Currency struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// NewClient creates a new Frankfurter API client
func NewClient(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// NewDefaultClient creates a new client with default settings
func NewDefaultClient() *Client {
	return NewClient(DefaultBaseURL, DefaultTimeout)
}

// GetLatestRates fetches the latest exchange rates for a base currency
func (c *Client) GetLatestRates(baseCurrency string) (*LatestRatesResponse, error) {
	if baseCurrency == "" {
		baseCurrency = "EUR"
	}

	endpoint := fmt.Sprintf("%s/latest?from=%s", c.baseURL, url.QueryEscape(baseCurrency))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result LatestRatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetLatestRatesForCurrencies fetches rates for specific target currencies
func (c *Client) GetLatestRatesForCurrencies(baseCurrency string, targetCurrencies []string) (*LatestRatesResponse, error) {
	if baseCurrency == "" {
		baseCurrency = "EUR"
	}

	endpoint := fmt.Sprintf("%s/latest?from=%s", c.baseURL, url.QueryEscape(baseCurrency))

	if len(targetCurrencies) > 0 {
		endpoint += "&to=" + url.QueryEscape(strings.Join(targetCurrencies, ","))
	}

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result LatestRatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetSupportedCurrencies fetches the list of supported currencies
func (c *Client) GetSupportedCurrencies() ([]Currency, error) {
	endpoint := fmt.Sprintf("%s/currencies", c.baseURL)

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch currencies: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var currenciesMap CurrenciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&currenciesMap); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	currencies := make([]Currency, 0, len(currenciesMap))
	for code, name := range currenciesMap {
		currencies = append(currencies, Currency{
			Code: code,
			Name: name,
		})
	}

	return currencies, nil
}

// Convert converts an amount from one currency to another
func (c *Client) Convert(amount float64, from, to string) (*ConvertResponse, error) {
	if from == "" {
		from = "EUR"
	}
	if to == "" {
		return nil, fmt.Errorf("target currency is required")
	}

	endpoint := fmt.Sprintf("%s/latest?amount=%f&from=%s&to=%s",
		c.baseURL, amount, url.QueryEscape(from), url.QueryEscape(to))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to convert currency: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result ConvertResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetHistoricalRates fetches exchange rates for a specific date
func (c *Client) GetHistoricalRates(date, baseCurrency string) (*LatestRatesResponse, error) {
	if baseCurrency == "" {
		baseCurrency = "EUR"
	}

	endpoint := fmt.Sprintf("%s/%s?from=%s", c.baseURL, date, url.QueryEscape(baseCurrency))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical rates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result LatestRatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
