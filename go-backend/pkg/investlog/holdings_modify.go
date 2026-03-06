package investlog

import (
	"errors"
	"fmt"
	"strings"
)

// ModifyHolding updates a holding to the target shares and average cost by
// recording a MODIFY transaction for the delta.
func (c *Core) ModifyHolding(req ModifyHoldingRequest) (int64, error) {
	if req.Symbol == "" {
		return 0, errors.New("symbol required")
	}
	if req.AccountID == "" {
		return 0, errors.New("account_id required")
	}
	if req.Currency == "" {
		return 0, errors.New("currency required")
	}
	if !isValidCurrency(req.Currency) {
		return 0, fmt.Errorf("invalid currency: %s", req.Currency)
	}
	if req.TargetShares.IsNegative() {
		return 0, errors.New("target_shares cannot be negative")
	}
	if req.TargetAvgCost.IsNegative() {
		return 0, errors.New("target_avg_cost cannot be negative")
	}
	if req.TransactionDate == "" {
		req.TransactionDate = todayISO()
	}

	normalizedSymbol := normalizeSymbol(req.Symbol)
	normalizedCurrency := normalizeCurrency(req.Currency)
	normalizedAssetType := normalizeAssetType(req.AssetType)

	holdings, err := c.GetHoldings(req.AccountID)
	if err != nil {
		return 0, fmt.Errorf("load holdings: %w", err)
	}

	var current *Holding
	for i := range holdings {
		item := holdings[i]
		if item.Symbol != normalizedSymbol || item.Currency != normalizedCurrency || item.AccountID != req.AccountID {
			continue
		}
		current = &item
		break
	}
	if current == nil {
		return 0, errors.New("holding not found")
	}

	if normalizedAssetType == "" {
		normalizedAssetType = current.AssetType
	}
	if normalizedAssetType == "" {
		normalizedAssetType = "stock"
	}

	targetTotalCost := Amount{req.TargetShares.Mul(req.TargetAvgCost.Decimal)}
	if normalizedAssetType == "cash" {
		targetTotalCost = req.TargetShares
		req.TargetAvgCost = NewAmountFromInt(1)
	}

	shareDelta := Amount{req.TargetShares.Sub(current.TotalShares.Decimal)}
	costDelta := Amount{targetTotalCost.Sub(current.TotalCost.Decimal)}
	if shareDelta.IsZero() && costDelta.IsZero() {
		return 0, errors.New("no holding changes detected")
	}

	price := NewAmountFromInt(0)
	if !shareDelta.IsZero() {
		price = Amount{costDelta.Div(shareDelta.Decimal).Abs()}
	}

	notes := req.Notes
	if notes == nil || strings.TrimSpace(*notes) == "" {
		msg := fmt.Sprintf(
			"Modify holding: shares %s -> %s, avg cost %s -> %s",
			current.TotalShares.Round(4).String(),
			req.TargetShares.Round(4).String(),
			current.AvgCost.Round(4).String(),
			req.TargetAvgCost.Round(4).String(),
		)
		notes = &msg
	}

	return c.AddTransaction(AddTransactionRequest{
		TransactionDate: req.TransactionDate,
		TransactionTime: req.TransactionTime,
		Symbol:          normalizedSymbol,
		TransactionType: "MODIFY",
		Quantity:        shareDelta,
		Price:           price,
		AccountID:       req.AccountID,
		AssetType:       normalizedAssetType,
		Commission:      NewAmountFromInt(0),
		Currency:        normalizedCurrency,
		AccountName:     req.AccountName,
		Notes:           notes,
		Tags:            req.Tags,
		TotalAmount:     amountPtr(costDelta),
	})
}
