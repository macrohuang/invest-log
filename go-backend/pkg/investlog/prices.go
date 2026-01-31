package investlog

import "database/sql"

// UpdateLatestPrice inserts or updates a latest price.
func (c *Core) UpdateLatestPrice(symbol, currency string, price float64) error {
	symbol = normalizeSymbol(symbol)
	currency = normalizeCurrency(currency)
	_, err := c.db.Exec(`
		INSERT INTO latest_prices (symbol, currency, price, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(symbol, currency) DO UPDATE SET
			price = excluded.price,
			updated_at = CURRENT_TIMESTAMP
	`, symbol, currency, price)
	return err
}

// GetLatestPrice returns the latest price for a symbol.
func (c *Core) GetLatestPrice(symbol, currency string) (*LatestPrice, error) {
	symbol = normalizeSymbol(symbol)
	currency = normalizeCurrency(currency)
	row := c.db.QueryRow("SELECT symbol, currency, price, updated_at FROM latest_prices WHERE symbol = ? AND currency = ?", symbol, currency)
	var p LatestPrice
	if err := row.Scan(&p.Symbol, &p.Currency, &p.Price, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// GetAllLatestPrices returns a map keyed by symbol+currency.
func (c *Core) GetAllLatestPrices() (map[[2]string]LatestPrice, error) {
	rows, err := c.db.Query("SELECT symbol, currency, price, updated_at FROM latest_prices")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[[2]string]LatestPrice{}
	for rows.Next() {
		var p LatestPrice
		if err := rows.Scan(&p.Symbol, &p.Currency, &p.Price, &p.UpdatedAt); err != nil {
			return nil, err
		}
		key := [2]string{p.Symbol, p.Currency}
		result[key] = p
	}
	return result, rows.Err()
}
