package db

import (
	"database/sql"
	"fmt"
	"strings"
)

func GetTransactions(filters map[string]interface{}, limit, offset int) ([]map[string]interface{}, error) {
	query := `
        SELECT 
            t.id, t.transaction_date, t.transaction_time, t.symbol_id, t.transaction_type,
            t.quantity, t.price, t.total_amount, t.commission, t.currency,
            t.account_id, t.account_name, t.notes, t.tags, t.created_at, t.updated_at,
            s.symbol, s.name, s.asset_type
        FROM transactions t
        JOIN symbols s ON s.id = t.symbol_id
        WHERE 1=1
    `
	var params []interface{}

	if symbol, ok := filters["symbol"].(string); ok && symbol != "" {
		query += " AND s.symbol = ?"
		params = append(params, strings.ToUpper(symbol))
	}
	if accountID, ok := filters["account_id"].(string); ok && accountID != "" {
		query += " AND t.account_id = ?"
		params = append(params, accountID)
	}
	if tType, ok := filters["transaction_type"].(string); ok && tType != "" {
		query += " AND t.transaction_type = ?"
		params = append(params, tType)
	}
	if currency, ok := filters["currency"].(string); ok && currency != "" {
		query += " AND t.currency = ?"
		params = append(params, currency)
	}

	query += " ORDER BY t.transaction_date DESC, t.id DESC LIMIT ? OFFSET ?"
	params = append(params, limit, offset)

	rows, err := DB.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []map[string]interface{}
	for rows.Next() {
		var id, symbolID int
		var quantity, price, totalAmount, commission float64
		var symbol, name, assetType sql.NullString
		var tDate, tTime, tType, currency, accountID, accountName, notes, tags, createdAt sql.NullString
		var updatedAt sql.NullTime

		err := rows.Scan(
			&id, &tDate, &tTime, &symbolID, &tType,
			&quantity, &price, &totalAmount, &commission, &currency,
			&accountID, &accountName, &notes, &tags, &createdAt, &updatedAt,
			&symbol, &name, &assetType,
		)
		if err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}

		transactions = append(transactions, map[string]interface{}{
			"id":               id,
			"transaction_date": tDate.String,
			"transaction_time": tTime.String,
			"symbol_id":        symbolID,
			"transaction_type": tType.String,
			"quantity":         quantity,
			"price":            price,
			"total_amount":     totalAmount,
			"commission":       commission,
			"currency":         currency.String,
			"account_id":       accountID.String,
			"account_name":     accountName.String,
			"notes":            notes.String,
			"tags":             tags.String,
			"created_at":       createdAt.String,
			"symbol":           symbol.String,
			"name":             name.String,
			"asset_type":       assetType.String,
		})
	}
	return transactions, nil
}
