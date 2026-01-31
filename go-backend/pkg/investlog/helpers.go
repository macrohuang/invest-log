package investlog

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
)

func normalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func normalizeAssetType(assetType string) string {
	return strings.ToLower(strings.TrimSpace(assetType))
}

func normalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func isValidCurrency(currency string) bool {
	currency = normalizeCurrency(currency)
	for _, c := range Currencies {
		if c == currency {
			return true
		}
	}
	return false
}

func isValidTransactionType(t string) bool {
	for _, v := range TransactionTypes {
		if v == t {
			return true
		}
	}
	return false
}

func (c *Core) assetTypeExists(tx *sql.Tx, assetType string) (bool, error) {
	var exists int
	err := tx.QueryRow("SELECT 1 FROM asset_types WHERE code = ?", assetType).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *Core) ensureSymbol(tx *sql.Tx, symbol string, assetType *string) (int64, string, string, error) {
	normalizedSymbol := normalizeSymbol(symbol)
	var normalizedAssetType string
	if assetType != nil && *assetType != "" {
		normalizedAssetType = normalizeAssetType(*assetType)
		exists, err := c.assetTypeExists(tx, normalizedAssetType)
		if err != nil {
			return 0, "", "", err
		}
		if !exists {
			return 0, "", "", fmt.Errorf("invalid asset_type: %s", normalizedAssetType)
		}
	}

	var id int64
	var currentAssetType string
	err := tx.QueryRow("SELECT id, asset_type FROM symbols WHERE symbol = ?", normalizedSymbol).Scan(&id, &currentAssetType)
	if err == nil {
		if normalizedAssetType != "" && currentAssetType != normalizedAssetType {
			if _, err := tx.Exec("UPDATE symbols SET asset_type = ? WHERE id = ?", normalizedAssetType, id); err != nil {
				return 0, "", "", err
			}
			currentAssetType = normalizedAssetType
		}
		return id, normalizedSymbol, currentAssetType, nil
	}
	if err != sql.ErrNoRows {
		return 0, "", "", err
	}

	insertAssetType := normalizedAssetType
	if insertAssetType == "" {
		insertAssetType = "stock"
	}
	result, err := tx.Exec("INSERT INTO symbols (symbol, asset_type) VALUES (?, ?)", normalizedSymbol, insertAssetType)
	if err != nil {
		return 0, "", "", err
	}
	newID, err := result.LastInsertId()
	if err != nil {
		return 0, "", "", err
	}
	return newID, normalizedSymbol, insertAssetType, nil
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}
