package investlog

import (
	"fmt"
)

// UpdatePrice fetches and stores latest price for a symbol.
func (c *Core) UpdatePrice(symbol, currency string) (PriceResult, error) {
	result, err := c.FetchPrice(symbol, currency, "stock")
	if result.Price != nil {
		_ = c.UpdateLatestPrice(symbol, currency, *result.Price)
		_, _ = c.AddOperationLog(OperationLog{
			Operation:    "PRICE_UPDATE",
			Symbol:       stringPtr(normalizeSymbol(symbol)),
			Currency:     stringPtr(normalizeCurrency(currency)),
			Details:      stringPtr(result.Message),
			PriceFetched: result.Price,
		})
		return result, nil
	}
	_, _ = c.AddOperationLog(OperationLog{
		Operation: "PRICE_UPDATE_FAILED",
		Symbol:    stringPtr(normalizeSymbol(symbol)),
		Currency:  stringPtr(normalizeCurrency(currency)),
		Details:   stringPtr(result.Message),
	})
	return result, err
}

// ManualUpdatePrice stores a manual price override.
func (c *Core) ManualUpdatePrice(symbol, currency string, price float64) error {
	if err := c.UpdateLatestPrice(symbol, currency, price); err != nil {
		return err
	}
	_, _ = c.AddOperationLog(OperationLog{
		Operation:    "MANUAL_PRICE_UPDATE",
		Symbol:       stringPtr(normalizeSymbol(symbol)),
		Currency:     stringPtr(normalizeCurrency(currency)),
		Details:      stringPtr("Manual price update"),
		PriceFetched: floatPtr(price),
	})
	return nil
}

// UpdateAllPrices updates all auto-update symbols within a currency.
func (c *Core) UpdateAllPrices(currency string) (int, []string, error) {
	currency = normalizeCurrency(currency)
	holdings, err := c.GetHoldingsBySymbol()
	if err != nil {
		return 0, nil, err
	}
	currencyData, ok := holdings[currency]
	if !ok {
		return 0, nil, fmt.Errorf("currency not found")
	}

	updated := 0
	var errors []string
	for _, s := range currencyData.Symbols {
		if s.AutoUpdate == 0 {
			continue
		}
		result, err := c.UpdatePrice(s.Symbol, currency)
		if result.Price != nil {
			updated++
		} else if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", s.Symbol, result.Message))
		}
	}
	return updated, errors, nil
}
