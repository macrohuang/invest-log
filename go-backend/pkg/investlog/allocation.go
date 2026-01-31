package investlog

import (
	"database/sql"
	"fmt"
	"strings"
)

// GetAllocationSettings returns allocation settings, optionally filtered by currency.
func (c *Core) GetAllocationSettings(currency string) ([]AllocationSetting, error) {
	currency = normalizeCurrency(currency)
	var rows *sql.Rows
	var err error
	if currency != "" {
		rows, err = c.db.Query("SELECT id, currency, asset_type, min_percent, max_percent FROM allocation_settings WHERE currency = ?", currency)
	} else {
		rows, err = c.db.Query("SELECT id, currency, asset_type, min_percent, max_percent FROM allocation_settings ORDER BY currency, asset_type")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []AllocationSetting
	for rows.Next() {
		var s AllocationSetting
		if err := rows.Scan(&s.ID, &s.Currency, &s.AssetType, &s.MinPercent, &s.MaxPercent); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

// SetAllocationSetting updates or inserts a setting.
func (c *Core) SetAllocationSetting(currency, assetType string, minPercent, maxPercent float64) (bool, error) {
	currency = normalizeCurrency(currency)
	if !isValidCurrency(currency) {
		return false, fmt.Errorf("invalid currency: %s", currency)
	}
	if minPercent < 0 || maxPercent > 100 || minPercent > maxPercent {
		return false, fmt.Errorf("invalid percent range")
	}
	assetType = strings.ToLower(strings.TrimSpace(assetType))
	if assetType == "" {
		return false, fmt.Errorf("asset_type required")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	valid, err := c.assetTypeExists(tx, assetType)
	if err != nil {
		return false, err
	}
	if !valid {
		return false, fmt.Errorf("invalid asset_type: %s", assetType)
	}

	_, err = tx.Exec(`
		INSERT INTO allocation_settings (currency, asset_type, min_percent, max_percent)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(currency, asset_type) DO UPDATE SET
			min_percent = excluded.min_percent,
			max_percent = excluded.max_percent
	`, currency, assetType, minPercent, maxPercent)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteAllocationSetting removes a setting.
func (c *Core) DeleteAllocationSetting(currency, assetType string) (bool, error) {
	currency = normalizeCurrency(currency)
	assetType = strings.ToLower(strings.TrimSpace(assetType))
	result, err := c.db.Exec("DELETE FROM allocation_settings WHERE currency = ? AND asset_type = ?", currency, assetType)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}
