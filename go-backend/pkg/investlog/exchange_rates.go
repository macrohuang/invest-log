package investlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultHKDToCNYRate = 0.92

	exchangeRateSourceDefault   = "default"
	exchangeRateSourceManual    = "manual"
	exchangeRateSourceAutoFetch = "auto_fetch"

	exchangeRateRequestTimeout = 10 * time.Second
	maxExchangeRateBodySize    = 1 << 20
)

var exchangeRateFetcher = fetchExchangeRateFromProviders

// GetExchangeRates returns all maintained exchange rates.
func (c *Core) GetExchangeRates() ([]ExchangeRateSetting, error) {
	rows, err := c.db.Query(`
		SELECT id, from_currency, to_currency, rate, source, updated_at
		FROM exchange_rates
		ORDER BY CASE from_currency WHEN 'USD' THEN 1 WHEN 'HKD' THEN 2 ELSE 99 END, to_currency
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]ExchangeRateSetting, 0, 2)
	for rows.Next() {
		var item ExchangeRateSetting
		if err := rows.Scan(
			&item.ID,
			&item.FromCurrency,
			&item.ToCurrency,
			&item.Rate,
			&item.Source,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// SetExchangeRate inserts or updates a maintained exchange rate.
func (c *Core) SetExchangeRate(fromCurrency, toCurrency string, rate float64, source string) (bool, error) {
	fromCurrency = normalizeCurrency(fromCurrency)
	toCurrency = normalizeCurrency(toCurrency)
	if err := validateExchangeRatePair(fromCurrency, toCurrency); err != nil {
		return false, err
	}
	if rate <= 0 {
		return false, fmt.Errorf("rate must be greater than 0")
	}
	normalizedSource := normalizeExchangeRateSource(source)

	_, err := c.db.Exec(`
		INSERT INTO exchange_rates (from_currency, to_currency, rate, source, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(from_currency, to_currency) DO UPDATE SET
			rate = excluded.rate,
			source = excluded.source,
			updated_at = CURRENT_TIMESTAMP
	`, fromCurrency, toCurrency, rate, normalizedSource)
	if err != nil {
		return false, err
	}
	c.invalidateHoldingsCache()
	return true, nil
}

// GetRateToCNY returns configured FX rate to CNY.
func (c *Core) GetRateToCNY(currency string) (float64, error) {
	currency = normalizeCurrency(currency)
	if currency == "CNY" {
		return 1, nil
	}
	if err := validateExchangeRatePair(currency, "CNY"); err != nil {
		return 0, err
	}

	var rate float64
	err := c.db.QueryRow(
		"SELECT rate FROM exchange_rates WHERE from_currency = ? AND to_currency = 'CNY'",
		currency,
	).Scan(&rate)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("exchange rate not found for %s/CNY", currency)
		}
		return 0, err
	}
	if rate <= 0 {
		return 0, fmt.Errorf("invalid exchange rate for %s/CNY", currency)
	}
	return rate, nil
}

// RefreshExchangeRates fetches USD/CNY and HKD/CNY from online providers.
func (c *Core) RefreshExchangeRates() (int, []string, error) {
	pairs := [][2]string{
		{"USD", "CNY"},
		{"HKD", "CNY"},
	}

	updated := 0
	errors := []string{}
	for _, pair := range pairs {
		rate, _, err := exchangeRateFetcher(pair[0], pair[1])
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s/%s: %v", pair[0], pair[1], err))
			continue
		}
		if _, err := c.SetExchangeRate(pair[0], pair[1], rate, exchangeRateSourceAutoFetch); err != nil {
			return updated, errors, err
		}
		updated++
	}

	return updated, errors, nil
}

func validateExchangeRatePair(fromCurrency, toCurrency string) error {
	fromCurrency = normalizeCurrency(fromCurrency)
	toCurrency = normalizeCurrency(toCurrency)
	if toCurrency != "CNY" {
		return fmt.Errorf("invalid to_currency: %s", toCurrency)
	}
	switch fromCurrency {
	case "USD", "HKD":
		return nil
	default:
		return fmt.Errorf("invalid from_currency: %s", fromCurrency)
	}
}

func normalizeExchangeRateSource(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return exchangeRateSourceManual
	}
	return strings.ToLower(trimmed)
}

func fetchExchangeRateFromProviders(fromCurrency, toCurrency string) (float64, string, error) {
	client := &http.Client{Timeout: exchangeRateRequestTimeout}
	ctx, cancel := context.WithTimeout(context.Background(), exchangeRateRequestTimeout)
	defer cancel()

	providers := []struct {
		name string
		fn   func(context.Context, *http.Client, string, string) (float64, error)
	}{
		{name: "frankfurter", fn: fetchExchangeRateFromFrankfurter},
		{name: "open_er_api", fn: fetchExchangeRateFromOpenERAPI},
	}

	errs := make([]string, 0, len(providers))
	for _, provider := range providers {
		rate, err := provider.fn(ctx, client, fromCurrency, toCurrency)
		if err == nil {
			return rate, provider.name, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", provider.name, err))
	}

	return 0, "", fmt.Errorf("all providers failed (%s)", strings.Join(errs, "; "))
}

type frankfurterRateResponse struct {
	Rates map[string]float64 `json:"rates"`
}

func fetchExchangeRateFromFrankfurter(ctx context.Context, client *http.Client, fromCurrency, toCurrency string) (float64, error) {
	url := fmt.Sprintf("https://api.frankfurter.app/latest?from=%s&to=%s", fromCurrency, toCurrency)
	var payload frankfurterRateResponse
	if err := fetchJSONWithClient(ctx, client, url, &payload); err != nil {
		return 0, err
	}
	rate := payload.Rates[toCurrency]
	if rate <= 0 {
		return 0, fmt.Errorf("rate missing in response")
	}
	return rate, nil
}

type openERAPIRateResponse struct {
	Result string             `json:"result"`
	Rates  map[string]float64 `json:"rates"`
}

func fetchExchangeRateFromOpenERAPI(ctx context.Context, client *http.Client, fromCurrency, toCurrency string) (float64, error) {
	url := fmt.Sprintf("https://open.er-api.com/v6/latest/%s", fromCurrency)
	var payload openERAPIRateResponse
	if err := fetchJSONWithClient(ctx, client, url, &payload); err != nil {
		return 0, err
	}
	if payload.Result != "" && strings.ToLower(payload.Result) != "success" {
		return 0, fmt.Errorf("provider status: %s", payload.Result)
	}
	rate := payload.Rates[toCurrency]
	if rate <= 0 {
		return 0, fmt.Errorf("rate missing in response")
	}
	return rate, nil
}

func fetchJSONWithClient(ctx context.Context, client *http.Client, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "InvestLog/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxExchangeRateBodySize))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
