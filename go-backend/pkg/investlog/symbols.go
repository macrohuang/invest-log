package investlog

import (
	"database/sql"
	"fmt"
	"strings"
)

// GetSymbols returns all symbols.
func (c *Core) GetSymbols() ([]Symbol, error) {
	rows, err := c.db.Query(`
		SELECT id, symbol, name, asset_type, sector, exchange, auto_update
		FROM symbols
		ORDER BY symbol
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []Symbol
	for rows.Next() {
		var s Symbol
		var name, sector, exchange sql.NullString
		if err := rows.Scan(&s.ID, &s.Symbol, &name, &s.AssetType, &sector, &exchange, &s.AutoUpdate); err != nil {
			return nil, err
		}
		if name.Valid {
			s.Name = &name.String
		}
		if sector.Valid {
			s.Sector = &sector.String
		}
		if exchange.Valid {
			s.Exchange = &exchange.String
		}
		symbols = append(symbols, s)
	}
	return symbols, rows.Err()
}

// GetSymbolMetadata fetches a symbol by code.
func (c *Core) GetSymbolMetadata(symbol string) (*Symbol, error) {
	symbol = normalizeSymbol(symbol)
	row := c.db.QueryRow("SELECT id, symbol, name, asset_type, sector, exchange, auto_update FROM symbols WHERE symbol = ?", symbol)
	var s Symbol
	var name, sector, exchange sql.NullString
	if err := row.Scan(&s.ID, &s.Symbol, &name, &s.AssetType, &sector, &exchange, &s.AutoUpdate); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if name.Valid {
		s.Name = &name.String
	}
	if sector.Valid {
		s.Sector = &sector.String
	}
	if exchange.Valid {
		s.Exchange = &exchange.String
	}
	return &s, nil
}

// UpdateSymbolMetadata updates symbol fields.
func (c *Core) UpdateSymbolMetadata(symbol string, name *string, assetType *string, autoUpdate *int, sector *string, exchange *string) (bool, error) {
	updates := []string{}
	values := []any{}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			updates = append(updates, "name = NULL")
		} else {
			updates = append(updates, "name = ?")
			values = append(values, trimmed)
		}
	}
	if assetType != nil {
		normalized := strings.ToLower(strings.TrimSpace(*assetType))
		tx, err := c.db.Begin()
		if err != nil {
			return false, err
		}
		valid, err := c.assetTypeExists(tx, normalized)
		if err != nil {
			_ = tx.Rollback()
			return false, err
		}
		if !valid {
			_ = tx.Rollback()
			return false, fmt.Errorf("invalid asset_type: %s", normalized)
		}
		_ = tx.Rollback()
		updates = append(updates, "asset_type = ?")
		values = append(values, normalized)
	}
	if autoUpdate != nil {
		updates = append(updates, "auto_update = ?")
		if *autoUpdate != 0 {
			values = append(values, 1)
		} else {
			values = append(values, 0)
		}
	}
	if sector != nil {
		trimmed := strings.TrimSpace(*sector)
		if trimmed == "" {
			updates = append(updates, "sector = NULL")
		} else {
			updates = append(updates, "sector = ?")
			values = append(values, trimmed)
		}
	}
	if exchange != nil {
		trimmed := strings.TrimSpace(*exchange)
		if trimmed == "" {
			updates = append(updates, "exchange = NULL")
		} else {
			updates = append(updates, "exchange = ?")
			values = append(values, trimmed)
		}
	}

	if len(updates) == 0 {
		return false, nil
	}
	values = append(values, normalizeSymbol(symbol))
	query := fmt.Sprintf("UPDATE symbols SET %s WHERE symbol = ?", strings.Join(updates, ", "))
	result, err := c.db.Exec(query, values...)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// UpdateSymbolAssetType updates a symbol's asset type.
func (c *Core) UpdateSymbolAssetType(symbol, assetType string) (bool, string, string, error) {
	symbol = normalizeSymbol(symbol)
	assetType = strings.ToLower(strings.TrimSpace(assetType))

	tx, err := c.db.Begin()
	if err != nil {
		return false, "", "", err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	valid, err := c.assetTypeExists(tx, assetType)
	if err != nil {
		return false, "", "", err
	}
	if !valid {
		return false, "", "", fmt.Errorf("invalid asset_type: %s", assetType)
	}
	var id int64
	var current string
	if err := tx.QueryRow("SELECT id, asset_type FROM symbols WHERE symbol = ?", symbol).Scan(&id, &current); err != nil {
		if err == sql.ErrNoRows {
			return false, "", "", nil
		}
		return false, "", "", err
	}
	current = strings.ToLower(current)
	if current == assetType {
		return true, current, current, nil
	}
	if _, err := tx.Exec("UPDATE symbols SET asset_type = ? WHERE id = ?", assetType, id); err != nil {
		return false, "", "", err
	}
	if err := tx.Commit(); err != nil {
		return false, "", "", err
	}
	return true, current, assetType, nil
}

// UpdateSymbolAutoUpdate sets auto_update for a symbol.
func (c *Core) UpdateSymbolAutoUpdate(symbol string, autoUpdate int) (bool, error) {
	symbol = normalizeSymbol(symbol)
	_, err := c.db.Exec("UPDATE symbols SET auto_update = ? WHERE symbol = ?", autoUpdate, symbol)
	if err != nil {
		return false, err
	}
	return true, nil
}
