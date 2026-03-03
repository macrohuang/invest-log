package investlog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// aiJSON returns a JSON string containing only the fields allowed for AI consumption:
// symbol, name, avg_cost, pnl_percent, position_percent, allocation_max_percent, allocation_status.
func (ctx *symbolContextData) aiJSON() (string, error) {
	slim := struct {
		Symbol               string  `json:"symbol"`
		Name                 string  `json:"name,omitempty"`
		AvgCost              float64 `json:"avg_cost,omitempty"`
		PnLPercent           float64 `json:"pnl_percent,omitempty"`
		PositionPercent      float64 `json:"position_percent,omitempty"`
		AllocationMaxPercent float64 `json:"allocation_max_percent,omitempty"`
		AllocationStatus     string  `json:"allocation_status,omitempty"`
	}{
		Symbol:               ctx.Symbol,
		Name:                 ctx.Name,
		AvgCost:              ctx.AvgCost,
		PnLPercent:           ctx.PnLPercent,
		PositionPercent:      ctx.PositionPercent,
		AllocationMaxPercent: ctx.AllocationMaxPercent,
		AllocationStatus:     ctx.AllocationStatus,
	}
	data, err := json.Marshal(slim)
	if err != nil {
		return "", fmt.Errorf("marshal symbol AI context: %w", err)
	}
	return string(data), nil
}

func (c *Core) buildSymbolContext(symbol, currency string) (*symbolContextData, error) {
	bySymbol, err := c.GetHoldingsBySymbol()
	if err != nil {
		return nil, fmt.Errorf("load holdings: %w", err)
	}

	currData, ok := bySymbol[currency]
	if !ok {
		return nil, fmt.Errorf("no holdings found for currency: %s", currency)
	}

	matched := make([]SymbolHolding, 0)
	for _, s := range currData.Symbols {
		if strings.EqualFold(s.Symbol, symbol) {
			matched = append(matched, s)
		}
	}

	if len(matched) == 0 {
		// Allow analysis even without holdings (just symbol + currency)
		return &symbolContextData{
			Symbol:   symbol,
			Currency: currency,
		}, nil
	}

	name := symbol
	assetType := ""
	var totalShares float64
	var totalCostBasis float64
	var totalMarketValue float64
	accountNameSet := map[string]struct{}{}

	for _, item := range matched {
		if name == symbol && item.Name != nil && strings.TrimSpace(*item.Name) != "" {
			name = strings.TrimSpace(*item.Name)
		}
		if assetType == "" {
			assetType = strings.TrimSpace(item.AssetType)
		}

		totalShares += item.TotalShares.InexactFloat64()
		totalCostBasis += item.CostBasis.InexactFloat64()
		totalMarketValue += item.MarketValue.InexactFloat64()

		accountName := strings.TrimSpace(item.AccountName)
		if accountName == "" {
			accountName = strings.TrimSpace(item.AccountID)
		}
		if accountName != "" {
			accountNameSet[accountName] = struct{}{}
		}
	}

	if assetType == "" {
		assetType = "stock"
	}

	accountNames := make([]string, 0, len(accountNameSet))
	for accountName := range accountNameSet {
		accountNames = append(accountNames, accountName)
	}
	sort.Strings(accountNames)

	avgCost := 0.0
	latestPrice := 0.0
	if totalShares > 0 {
		avgCost = round2(totalCostBasis / totalShares)
		latestPrice = round2(totalMarketValue / totalShares)
	}
	pnlPercent := 0.0
	if totalCostBasis > 0 {
		pnlPercent = round2((totalMarketValue - totalCostBasis) / totalCostBasis * 100)
	}
	positionPercent := 0.0
	if currData.TotalMarketValue.IsPositive() {
		positionPercent = round2(totalMarketValue / currData.TotalMarketValue.InexactFloat64() * 100)
	}

	ctx := &symbolContextData{
		Symbol:                   symbol,
		Name:                     name,
		Currency:                 currency,
		AssetType:                assetType,
		TotalShares:              round2(totalShares),
		AvgCost:                  avgCost,
		CostBasis:                round2(totalCostBasis),
		LatestPrice:              latestPrice,
		MarketValue:              round2(totalMarketValue),
		PnLPercent:               pnlPercent,
		PositionPercent:          positionPercent,
		CurrencyTotalMarketValue: round2(currData.TotalMarketValue.InexactFloat64()),
		AccountNames:             accountNames,
	}
	if len(accountNames) > 0 {
		ctx.AccountName = accountNames[0]
	}

	allocationSettings, err := c.GetAllocationSettings(currency)
	if err != nil {
		c.Logger().Warn("load allocation settings failed", "currency", currency, "err", err)
	} else {
		ctx.AllocationStatus = "no_target"
		for _, setting := range allocationSettings {
			if strings.EqualFold(setting.AssetType, assetType) {
				ctx.AllocationMinPercent = round2(setting.MinPercent)
				ctx.AllocationMaxPercent = round2(setting.MaxPercent)
				switch {
				case positionPercent < setting.MinPercent:
					ctx.AllocationStatus = "below_target"
				case positionPercent > setting.MaxPercent:
					ctx.AllocationStatus = "above_target"
				default:
					ctx.AllocationStatus = "within_target"
				}
				break
			}
		}
	}

	return ctx, nil
}
