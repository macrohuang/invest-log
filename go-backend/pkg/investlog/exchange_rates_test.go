package investlog

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetExchangeRates_Defaults(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	rates, err := core.GetExchangeRates()
	if err != nil {
		t.Fatalf("GetExchangeRates returned error: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("expected 2 default rates, got %d", len(rates))
	}

	lookup := map[string]ExchangeRateSetting{}
	for _, item := range rates {
		lookup[item.FromCurrency+"_"+item.ToCurrency] = item
	}

	usd, ok := lookup["USD_CNY"]
	if !ok {
		t.Fatalf("missing USD/CNY rate")
	}
	if usd.Rate <= 0 {
		t.Fatalf("invalid USD/CNY rate: %.6f", usd.Rate)
	}

	hkd, ok := lookup["HKD_CNY"]
	if !ok {
		t.Fatalf("missing HKD/CNY rate")
	}
	if hkd.Rate <= 0 {
		t.Fatalf("invalid HKD/CNY rate: %.6f", hkd.Rate)
	}
}

func TestSetExchangeRate_AndGetRateToCNY(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	if _, err := core.SetExchangeRate("usd", "cny", 7.31, "manual"); err != nil {
		t.Fatalf("SetExchangeRate returned error: %v", err)
	}

	rate, err := core.GetRateToCNY("USD")
	if err != nil {
		t.Fatalf("GetRateToCNY returned error: %v", err)
	}
	if !floatEquals(rate, 7.31, 0.0001) {
		t.Fatalf("unexpected USD/CNY rate, got %.6f", rate)
	}

	cnyRate, err := core.GetRateToCNY("CNY")
	if err != nil {
		t.Fatalf("GetRateToCNY CNY returned error: %v", err)
	}
	if cnyRate != 1 {
		t.Fatalf("expected CNY/CNY = 1, got %.6f", cnyRate)
	}
}

func TestSetExchangeRate_Validation(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name         string
		fromCurrency string
		toCurrency   string
		rate         float64
		wantErr      string
	}{
		{name: "invalid from", fromCurrency: "EUR", toCurrency: "CNY", rate: 1, wantErr: "invalid from_currency"},
		{name: "invalid to", fromCurrency: "USD", toCurrency: "HKD", rate: 1, wantErr: "invalid to_currency"},
		{name: "invalid rate", fromCurrency: "USD", toCurrency: "CNY", rate: 0, wantErr: "rate must be greater than 0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := core.SetExchangeRate(tc.fromCurrency, tc.toCurrency, tc.rate, "manual")
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestRefreshExchangeRates(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	originalFetcher := exchangeRateFetcher
	defer func() {
		exchangeRateFetcher = originalFetcher
	}()

	exchangeRateFetcher = func(fromCurrency, toCurrency string) (float64, string, error) {
		switch fromCurrency + "/" + toCurrency {
		case "USD/CNY":
			return 7.25, "stub", nil
		case "HKD/CNY":
			return 0.93, "stub", nil
		default:
			return 0, "", fmt.Errorf("unexpected pair: %s/%s", fromCurrency, toCurrency)
		}
	}

	updated, errors, err := core.RefreshExchangeRates()
	if err != nil {
		t.Fatalf("RefreshExchangeRates returned error: %v", err)
	}
	if updated != 2 {
		t.Fatalf("expected updated=2, got %d", updated)
	}
	if len(errors) != 0 {
		t.Fatalf("expected no errors, got %v", errors)
	}

	usdRate, err := core.GetRateToCNY("USD")
	if err != nil {
		t.Fatalf("GetRateToCNY USD returned error: %v", err)
	}
	if !floatEquals(usdRate, 7.25, 0.0001) {
		t.Fatalf("unexpected USD/CNY rate, got %.6f", usdRate)
	}

	hkdRate, err := core.GetRateToCNY("HKD")
	if err != nil {
		t.Fatalf("GetRateToCNY HKD returned error: %v", err)
	}
	if !floatEquals(hkdRate, 0.93, 0.0001) {
		t.Fatalf("unexpected HKD/CNY rate, got %.6f", hkdRate)
	}
}
