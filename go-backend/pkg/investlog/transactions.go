package investlog

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// TransactionFilter controls transaction queries.
type TransactionFilter struct {
	Symbol          string
	AccountID       string
	TransactionType string
	Currency        string
	Year            int
	StartDate       string
	EndDate         string
	Limit           int
	Offset          int
}

// AddTransaction inserts a new transaction and returns its ID.
func (c *Core) AddTransaction(req AddTransactionRequest) (int64, error) {
	if req.TransactionType == "" {
		return 0, errors.New("transaction_type required")
	}
	if !isValidTransactionType(req.TransactionType) {
		return 0, fmt.Errorf("invalid transaction_type: %s", req.TransactionType)
	}
	if req.AccountID == "" {
		return 0, errors.New("account_id required")
	}
	if req.Currency == "" {
		req.Currency = "CNY"
	}
	if !isValidCurrency(req.Currency) {
		return 0, fmt.Errorf("invalid currency: %s", req.Currency)
	}
	if req.TransactionDate == "" {
		req.TransactionDate = todayISO()
	}
	if req.AssetType == "" {
		req.AssetType = "stock"
	}
	if strings.EqualFold(req.TransactionType, "INCOME") {
		req.Symbol = "CASH"
		req.AssetType = "cash"
		req.Price = 1.0
	}
	if req.Symbol == "" {
		return 0, errors.New("symbol required")
	}

	totalAmount := req.Quantity * req.Price
	if req.TotalAmount != nil {
		totalAmount = *req.TotalAmount
	}

	tx, err := c.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	symbolID, symbol, _, err := c.ensureSymbol(tx, req.Symbol, &req.AssetType)
	if err != nil {
		return 0, err
	}

	id, err := c.insertTransactionTx(tx, req, symbolID, totalAmount)
	if err != nil {
		return 0, err
	}

	if req.LinkCash && (req.TransactionType == "BUY" || req.TransactionType == "SELL") && symbol != "CASH" {
		cashType := "SELL"
		if req.TransactionType == "SELL" {
			cashType = "BUY"
		}
		cashAmount := totalAmount + req.Commission
		if req.TransactionType == "SELL" {
			cashAmount = totalAmount - req.Commission
		}
		cashReq := AddTransactionRequest{
			TransactionDate: req.TransactionDate,
			TransactionTime: req.TransactionTime,
			Symbol:          "CASH",
			TransactionType: cashType,
			Quantity:        cashAmount,
			Price:           1.0,
			AccountID:       req.AccountID,
			AssetType:       "cash",
			Commission:      0,
			Currency:        req.Currency,
			AccountName:     req.AccountName,
			Notes:           stringPtr(fmt.Sprintf("Linked to %s %s", req.TransactionType, symbol)),
		}
		cashSymbolID, _, _, err := c.ensureSymbol(tx, cashReq.Symbol, &cashReq.AssetType)
		if err != nil {
			return 0, err
		}
		if _, err := c.insertTransactionTx(tx, cashReq, cashSymbolID, cashAmount); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return id, nil
}

func (c *Core) insertTransactionTx(tx *sql.Tx, req AddTransactionRequest, symbolID int64, totalAmount float64) (int64, error) {
	result, err := tx.Exec(`
		INSERT INTO transactions (
			transaction_date, transaction_time, symbol_id, transaction_type,
			quantity, price, total_amount, commission, currency,
			account_id, account_name, notes, tags
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		req.TransactionDate,
		nullString(req.TransactionTime),
		symbolID,
		req.TransactionType,
		req.Quantity,
		req.Price,
		totalAmount,
		req.Commission,
		req.Currency,
		req.AccountID,
		nullString(req.AccountName),
		nullString(req.Notes),
		nullString(req.Tags),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetTransaction fetches a single transaction by ID.
func (c *Core) GetTransaction(id int64) (*Transaction, error) {
	row := c.db.QueryRow(`
		SELECT
			t.id, t.transaction_date, t.transaction_time, t.symbol_id, t.transaction_type,
			t.quantity, t.price, t.total_amount, t.commission, t.currency,
			t.account_id, t.account_name, t.notes, t.tags, t.created_at, t.updated_at,
			s.symbol, s.name, s.asset_type
		FROM transactions t
		JOIN symbols s ON s.id = t.symbol_id
		WHERE t.id = ?
	`, id)

	var t Transaction
	var transactionTime, accountName, notes, tags, createdAt, updatedAt, name sql.NullString
	if err := row.Scan(
		&t.ID, &t.TransactionDate, &transactionTime, &t.SymbolID, &t.TransactionType,
		&t.Quantity, &t.Price, &t.TotalAmount, &t.Commission, &t.Currency,
		&t.AccountID, &accountName, &notes, &tags, &createdAt, &updatedAt,
		&t.Symbol, &name, &t.AssetType,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if transactionTime.Valid {
		t.TransactionTime = &transactionTime.String
	}
	if accountName.Valid {
		t.AccountName = &accountName.String
	}
	if notes.Valid {
		t.Notes = &notes.String
	}
	if tags.Valid {
		t.Tags = &tags.String
	}
	if createdAt.Valid {
		t.CreatedAt = &createdAt.String
	}
	if updatedAt.Valid {
		t.UpdatedAt = &updatedAt.String
	}
	if name.Valid {
		t.Name = &name.String
	}
	return &t, nil
}

// GetTransactions returns transactions matching the filter.
func (c *Core) GetTransactions(filter TransactionFilter) ([]Transaction, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := strings.Builder{}
	query.WriteString(`
		SELECT
			t.id, t.transaction_date, t.transaction_time, t.symbol_id, t.transaction_type,
			t.quantity, t.price, t.total_amount, t.commission, t.currency,
			t.account_id, t.account_name, t.notes, t.tags, t.created_at, t.updated_at,
			s.symbol, s.name, s.asset_type
		FROM transactions t
		JOIN symbols s ON s.id = t.symbol_id
		WHERE 1=1
	`)
	params := []any{}

	if filter.Symbol != "" {
		query.WriteString(" AND s.symbol = ?")
		params = append(params, normalizeSymbol(filter.Symbol))
	}
	if filter.AccountID != "" {
		query.WriteString(" AND t.account_id = ?")
		params = append(params, filter.AccountID)
	}
	if filter.TransactionType != "" {
		query.WriteString(" AND t.transaction_type = ?")
		params = append(params, filter.TransactionType)
	}
	if filter.Currency != "" {
		query.WriteString(" AND t.currency = ?")
		params = append(params, normalizeCurrency(filter.Currency))
	}
	if filter.Year > 0 {
		query.WriteString(" AND strftime('%Y', t.transaction_date) = ?")
		params = append(params, fmt.Sprintf("%04d", filter.Year))
	}
	if filter.StartDate != "" {
		query.WriteString(" AND t.transaction_date >= ?")
		params = append(params, filter.StartDate)
	}
	if filter.EndDate != "" {
		query.WriteString(" AND t.transaction_date <= ?")
		params = append(params, filter.EndDate)
	}

	query.WriteString(" ORDER BY t.transaction_date DESC, t.id DESC LIMIT ? OFFSET ?")
	params = append(params, limit, offset)

	rows, err := c.db.Query(query.String(), params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Transaction
	for rows.Next() {
		var t Transaction
		var transactionTime, accountName, notes, tags, createdAt, updatedAt, name sql.NullString
		if err := rows.Scan(
			&t.ID, &t.TransactionDate, &transactionTime, &t.SymbolID, &t.TransactionType,
			&t.Quantity, &t.Price, &t.TotalAmount, &t.Commission, &t.Currency,
			&t.AccountID, &accountName, &notes, &tags, &createdAt, &updatedAt,
			&t.Symbol, &name, &t.AssetType,
		); err != nil {
			return nil, err
		}
		if transactionTime.Valid {
			t.TransactionTime = &transactionTime.String
		}
		if accountName.Valid {
			t.AccountName = &accountName.String
		}
		if notes.Valid {
			t.Notes = &notes.String
		}
		if tags.Valid {
			t.Tags = &tags.String
		}
		if createdAt.Valid {
			t.CreatedAt = &createdAt.String
		}
		if updatedAt.Valid {
			t.UpdatedAt = &updatedAt.String
		}
		if name.Valid {
			t.Name = &name.String
		}
		results = append(results, t)
	}
	return results, rows.Err()
}

// GetTransactionCount returns count of transactions matching the filter.
func (c *Core) GetTransactionCount(filter TransactionFilter) (int, error) {
	query := strings.Builder{}
	query.WriteString(`
		SELECT COUNT(*)
		FROM transactions t
		JOIN symbols s ON s.id = t.symbol_id
		WHERE 1=1
	`)
	params := []any{}

	if filter.Symbol != "" {
		query.WriteString(" AND s.symbol = ?")
		params = append(params, normalizeSymbol(filter.Symbol))
	}
	if filter.AccountID != "" {
		query.WriteString(" AND t.account_id = ?")
		params = append(params, filter.AccountID)
	}
	if filter.TransactionType != "" {
		query.WriteString(" AND t.transaction_type = ?")
		params = append(params, filter.TransactionType)
	}
	if filter.Currency != "" {
		query.WriteString(" AND t.currency = ?")
		params = append(params, normalizeCurrency(filter.Currency))
	}
	if filter.Year > 0 {
		query.WriteString(" AND strftime('%Y', t.transaction_date) = ?")
		params = append(params, fmt.Sprintf("%04d", filter.Year))
	}

	var count int
	if err := c.db.QueryRow(query.String(), params...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// DeleteTransaction deletes a transaction by ID.
func (c *Core) DeleteTransaction(id int64) (bool, error) {
	result, err := c.db.Exec("DELETE FROM transactions WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func nullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	if strings.TrimSpace(*value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}
