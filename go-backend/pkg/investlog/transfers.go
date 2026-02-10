package investlog

import (
	"errors"
	"fmt"
	"strings"
)

// Transfer executes a cross-account transfer, creating paired TRANSFER_OUT and
// TRANSFER_IN records linked by linked_transaction_id.
func (c *Core) Transfer(req TransferRequest) (*TransferResult, error) {
	// --- 1. Defaults & validation ---
	if req.Symbol == "" {
		return nil, errors.New("symbol required")
	}
	if req.Quantity <= 0 {
		return nil, errors.New("quantity must be positive")
	}
	if req.FromAccountID == "" {
		return nil, errors.New("from_account_id required")
	}
	if req.ToAccountID == "" {
		return nil, errors.New("to_account_id required")
	}
	if req.FromAccountID == req.ToAccountID {
		return nil, errors.New("from_account_id and to_account_id must be different")
	}
	if req.FromCurrency == "" {
		req.FromCurrency = "CNY"
	}
	if !isValidCurrency(req.FromCurrency) {
		return nil, fmt.Errorf("invalid from_currency: %s", req.FromCurrency)
	}
	if req.ToCurrency == "" {
		req.ToCurrency = req.FromCurrency
	}
	if !isValidCurrency(req.ToCurrency) {
		return nil, fmt.Errorf("invalid to_currency: %s", req.ToCurrency)
	}
	if req.TransactionDate == "" {
		req.TransactionDate = todayISO()
	}
	if req.AssetType == "" {
		req.AssetType = "stock"
	}
	isCash := strings.EqualFold(normalizeSymbol(req.Symbol), "CASH")
	if isCash {
		req.AssetType = "cash"
	}

	// --- 2. Check source holdings ---
	currentShares, err := c.getCurrentShares(req.Symbol, req.FromCurrency, req.FromAccountID)
	if err != nil {
		return nil, fmt.Errorf("check source holdings: %w", err)
	}
	if req.Quantity > currentShares {
		return nil, fmt.Errorf("insufficient holdings: trying to transfer %.4f but only have %.4f", req.Quantity, currentShares)
	}

	// Get avg cost from source
	avgCost, err := c.getCurrentAvgCost(req.Symbol, req.FromCurrency, req.FromAccountID)
	if err != nil {
		return nil, fmt.Errorf("get avg cost: %w", err)
	}

	// --- 3. Exchange rate ---
	exchangeRate := 1.0
	crossCurrency := normalizeCurrency(req.FromCurrency) != normalizeCurrency(req.ToCurrency)
	if crossCurrency {
		exchangeRate, err = c.GetExchangeRate(req.FromCurrency, req.ToCurrency)
		if err != nil {
			return nil, fmt.Errorf("get exchange rate: %w", err)
		}
	}

	// --- 4. Calculate transfer parameters ---
	outPrice := avgCost
	outTotalAmount := avgCost * req.Quantity

	var inQuantity, inPrice, inTotalAmount float64
	if isCash {
		inQuantity = req.Quantity * exchangeRate
		inPrice = 1.0
		inTotalAmount = inQuantity
	} else {
		inQuantity = req.Quantity
		inPrice = avgCost * exchangeRate
		inTotalAmount = outTotalAmount * exchangeRate
	}

	// --- 5. DB transaction: insert paired records ---
	tx, err := c.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensureAccountTx(tx, req.FromAccountID, nil); err != nil {
		return nil, err
	}
	if err := ensureAccountTx(tx, req.ToAccountID, nil); err != nil {
		return nil, err
	}

	symbolID, _, _, err := c.ensureSymbol(tx, req.Symbol, &req.AssetType)
	if err != nil {
		return nil, err
	}

	// Build notes
	var outNotes, inNotes *string
	baseNote := ""
	if req.Notes != nil && strings.TrimSpace(*req.Notes) != "" {
		baseNote = *req.Notes + " | "
	}
	outNote := fmt.Sprintf("%s转出至 %s", baseNote, req.ToAccountID)
	outNotes = &outNote
	inNote := fmt.Sprintf("%s转入自 %s", baseNote, req.FromAccountID)
	inNotes = &inNote

	// Insert TRANSFER_OUT
	outReq := AddTransactionRequest{
		TransactionDate: req.TransactionDate,
		Symbol:          req.Symbol,
		TransactionType: "TRANSFER_OUT",
		Quantity:        req.Quantity,
		Price:           outPrice,
		AccountID:       req.FromAccountID,
		AssetType:       req.AssetType,
		Commission:      req.Commission,
		Currency:        req.FromCurrency,
		Notes:           outNotes,
	}
	outID, err := c.insertTransactionWithLinkTx(tx, outReq, symbolID, outTotalAmount, nil)
	if err != nil {
		return nil, fmt.Errorf("insert transfer_out: %w", err)
	}

	// Insert TRANSFER_IN with linked_transaction_id pointing to outID
	inReq := AddTransactionRequest{
		TransactionDate: req.TransactionDate,
		Symbol:          req.Symbol,
		TransactionType: "TRANSFER_IN",
		Quantity:        inQuantity,
		Price:           inPrice,
		AccountID:       req.ToAccountID,
		AssetType:       req.AssetType,
		Commission:      0,
		Currency:        req.ToCurrency,
		Notes:           inNotes,
	}
	inID, err := c.insertTransactionWithLinkTx(tx, inReq, symbolID, inTotalAmount, &outID)
	if err != nil {
		return nil, fmt.Errorf("insert transfer_in: %w", err)
	}

	// Update TRANSFER_OUT to point back to TRANSFER_IN
	if _, err := tx.Exec("UPDATE transactions SET linked_transaction_id = ? WHERE id = ?", inID, outID); err != nil {
		return nil, fmt.Errorf("link transfer_out: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	c.invalidateHoldingsCache()

	result := &TransferResult{
		TransferOutID: outID,
		TransferInID:  inID,
	}
	if crossCurrency {
		result.ExchangeRate = exchangeRate
	}
	return result, nil
}

// getCurrentAvgCost returns the weighted average cost for a symbol in a specific account and currency.
func (c *Core) getCurrentAvgCost(symbol, currency, accountID string) (float64, error) {
	holdings, err := c.GetHoldings("")
	if err != nil {
		return 0, err
	}
	sym := normalizeSymbol(symbol)
	cur := normalizeCurrency(currency)
	for _, h := range holdings {
		if h.Symbol == sym && h.Currency == cur && h.AccountID == accountID {
			return h.AvgCost, nil
		}
	}
	return 0, nil
}
