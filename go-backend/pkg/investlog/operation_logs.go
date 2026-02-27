package investlog

import "database/sql"

// AddOperationLog adds a new operation log entry.
func (c *Core) AddOperationLog(log OperationLog) (int64, error) {
	result, err := c.db.Exec(`
		INSERT INTO operation_logs (operation_type, symbol, currency, details, old_value, new_value, price_fetched)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, log.Operation, log.Symbol, log.Currency, log.Details, log.OldValue, log.NewValue, log.PriceFetched)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetOperationLogs returns recent operation logs.
func (c *Core) GetOperationLogs(limit, offset int) ([]OperationLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := c.db.Query(
		"SELECT id, operation_type, symbol, currency, details, old_value, new_value, price_fetched, created_at FROM operation_logs ORDER BY created_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []OperationLog
	for rows.Next() {
		var log OperationLog
		var symbol, currency, details, createdAt sql.NullString
		var oldValue, newValue, priceFetched sql.NullFloat64
		if err := rows.Scan(&log.ID, &log.Operation, &symbol, &currency, &details, &oldValue, &newValue, &priceFetched, &createdAt); err != nil {
			return nil, err
		}
		if symbol.Valid {
			log.Symbol = &symbol.String
		}
		if currency.Valid {
			log.Currency = &currency.String
		}
		if details.Valid {
			log.Details = &details.String
		}
		if oldValue.Valid {
			a := NewAmount(oldValue.Float64)
			log.OldValue = &a
		}
		if newValue.Valid {
			a := NewAmount(newValue.Float64)
			log.NewValue = &a
		}
		if priceFetched.Valid {
			a := NewAmount(priceFetched.Float64)
			log.PriceFetched = &a
		}
		if createdAt.Valid {
			log.CreatedAt = &createdAt.String
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}
