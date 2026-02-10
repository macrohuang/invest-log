package investlog

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// GetHoldings calculates holdings aggregated by symbol, currency, and account.
func (c *Core) GetHoldings(accountID string) ([]Holding, error) {
	if accountID == "" && c.cache != nil {
		if cached, ok := c.cache.getHoldings(); ok {
			return cached, nil
		}
	}
	query := `
		SELECT
			s.symbol AS symbol,
			s.name AS name,
			t.account_id,
			t.currency,
			s.asset_type AS asset_type,
			SUM(CASE
				WHEN t.transaction_type IN ('BUY', 'TRANSFER_IN', 'INCOME') THEN t.quantity
				WHEN t.transaction_type IN ('SELL', 'TRANSFER_OUT') THEN -t.quantity
				WHEN t.transaction_type IN ('SPLIT', 'ADJUST') THEN t.quantity
				ELSE 0
			END) as total_shares,
			SUM(CASE
				WHEN t.transaction_type IN ('BUY', 'INCOME') THEN t.total_amount + t.commission
				WHEN t.transaction_type = 'SELL' THEN -(t.total_amount - t.commission)
				WHEN t.transaction_type = 'ADJUST' THEN t.total_amount
				WHEN t.transaction_type = 'TRANSFER_IN' AND t.linked_transaction_id IS NOT NULL
					THEN t.total_amount
				WHEN t.transaction_type = 'TRANSFER_OUT' AND t.linked_transaction_id IS NOT NULL
					THEN -t.total_amount
				ELSE 0
			END) as total_cost
		FROM transactions t
		JOIN symbols s ON s.id = t.symbol_id
	`
	params := []any{}
	if accountID != "" {
		query += " WHERE t.account_id = ?"
		params = append(params, accountID)
	}
	query += " GROUP BY t.symbol_id, s.symbol, s.name, s.asset_type, t.account_id, t.currency HAVING total_shares > 0 OR total_cost != 0"

	rows, err := c.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var holdings []Holding
	for rows.Next() {
		var h Holding
		var name sql.NullString
		if err := rows.Scan(&h.Symbol, &name, &h.AccountID, &h.Currency, &h.AssetType, &h.TotalShares, &h.TotalCost); err != nil {
			return nil, err
		}
		if name.Valid {
			h.Name = &name.String
		}
		if strings.ToLower(h.AssetType) == "cash" {
			h.TotalCost = h.TotalShares
			if h.TotalShares != 0 {
				h.AvgCost = 1
			}
		} else if h.TotalShares > 0 {
			h.AvgCost = h.TotalCost / h.TotalShares
		}
		holdings = append(holdings, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if accountID == "" && c.cache != nil {
		c.cache.setHoldings(holdings)
	}
	return holdings, nil
}

// GetHoldingsBySymbol returns holdings grouped by currency with PnL data.
func (c *Core) GetHoldingsBySymbol() (HoldingsBySymbolResult, error) {
	if c.cache != nil {
		if cached, ok := c.cache.getBySymbol(); ok {
			return cached, nil
		}
	}
	holdings, err := c.GetHoldings("")
	if err != nil {
		return nil, err
	}
	latestPrices, err := c.GetAllLatestPrices()
	if err != nil {
		return nil, err
	}
	assetTypeLabels, err := c.GetAssetTypeLabels()
	if err != nil {
		assetTypeLabels = DefaultAssetTypeLabels
	}
	accounts, err := c.GetAccounts()
	if err != nil {
		return nil, err
	}
	accountNames := map[string]string{}
	for _, acc := range accounts {
		name := strings.TrimSpace(acc.AccountName)
		if name == "" {
			name = acc.AccountID
		}
		accountNames[acc.AccountID] = name
	}
	autoUpdateMap, err := c.getAutoUpdateMap()
	if err != nil {
		return nil, err
	}

	byCurrency := map[string]struct {
		totalCost float64
		symbols   []Holding
	}{}
	for _, h := range holdings {
		curr := h.Currency
		entry := byCurrency[curr]
		entry.totalCost += h.TotalCost
		entry.symbols = append(entry.symbols, h)
		byCurrency[curr] = entry
	}

	result := HoldingsBySymbolResult{}
	for currency, data := range byCurrency {
		// sort by cost basis desc
		sort.Slice(data.symbols, func(i, j int) bool {
			return data.symbols[i].TotalCost > data.symbols[j].TotalCost
		})
		symbolsData := make([]SymbolHolding, 0, len(data.symbols))
		var totalMarketValue float64

		for _, h := range data.symbols {
			name := ""
			if h.Name != nil {
				name = strings.TrimSpace(*h.Name)
			}
			displayName := h.Symbol
			if name != "" {
				displayName = name
			}
			priceKey := [2]string{h.Symbol, currency}
			var latestPrice *float64
			var priceUpdatedAt *string
			if p, ok := latestPrices[priceKey]; ok {
				lp := p.Price
				latestPrice = &lp
				priceUpdatedAt = &p.UpdatedAt
			}

			marketValue := h.TotalCost
			var unrealizedPnL *float64
			var pnlPercent *float64
			if latestPrice != nil && h.TotalShares > 0 {
				marketValue = (*latestPrice) * h.TotalShares
				pnl := marketValue - h.TotalCost
				unrealizedPnL = &pnl
				if h.TotalCost > 0 {
					percent := round2(pnl / h.TotalCost * 100)
					pnlPercent = &percent
				}
			}
			totalMarketValue += marketValue

			assetType := h.AssetType
			if assetType == "" {
				assetType = "stock"
			}
			label := assetTypeLabels[assetType]
			if label == "" {
				label = assetType
			}

			accountName := accountNames[h.AccountID]
			if strings.TrimSpace(accountName) == "" {
				accountName = h.AccountID
			}
			autoUpdate, ok := autoUpdateMap[h.Symbol]
			if !ok {
				autoUpdate = 1
			}
			symbolsData = append(symbolsData, SymbolHolding{
				Symbol:         h.Symbol,
				Name:           h.Name,
				DisplayName:    displayName,
				AssetType:      assetType,
				AssetTypeLabel: label,
				AutoUpdate:     autoUpdate,
				AccountID:      h.AccountID,
				AccountName:    accountName,
				TotalShares:    h.TotalShares,
				AvgCost:        h.AvgCost,
				CostBasis:      h.TotalCost,
				LatestPrice:    latestPrice,
				PriceUpdatedAt: priceUpdatedAt,
				MarketValue:    marketValue,
				UnrealizedPnL:  unrealizedPnL,
				PnlPercent:     pnlPercent,
			})
		}

		for i := range symbolsData {
			if totalMarketValue > 0 {
				symbolsData[i].Percent = round2(symbolsData[i].MarketValue / totalMarketValue * 100)
			}
		}

		byAccount := map[string]SymbolHoldingsByAccount{}
		for _, s := range symbolsData {
			entry := byAccount[s.AccountID]
			entry.AccountName = s.AccountName
			entry.Symbols = append(entry.Symbols, s)
			byAccount[s.AccountID] = entry
		}

		result[currency] = SymbolHoldingsCurrency{
			TotalCost:        data.totalCost,
			TotalMarketValue: totalMarketValue,
			TotalPnL:         totalMarketValue - data.totalCost,
			Symbols:          symbolsData,
			ByAccount:        byAccount,
		}
	}
	if c.cache != nil {
		c.cache.setBySymbol(result)
	}
	return result, nil
}

// GetHoldingsByCurrency calculates allocation by asset type within currency.
func (c *Core) GetHoldingsByCurrency() (HoldingsByCurrencyResult, error) {
	if c.cache != nil {
		if cached, ok := c.cache.getByCurrency(); ok {
			return cached, nil
		}
	}
	holdings, err := c.GetHoldings("")
	if err != nil {
		return nil, err
	}
	latestPrices, err := c.GetAllLatestPrices()
	if err != nil {
		return nil, err
	}
	settings, err := c.GetAllocationSettings("")
	if err != nil {
		return nil, err
	}
	assetTypes, err := c.GetAssetTypes()
	if err != nil {
		return nil, err
	}
	labels, err := c.GetAssetTypeLabels()
	if err != nil {
		labels = DefaultAssetTypeLabels
	}

	settingsMap := map[[2]string]struct {
		min float64
		max float64
	}{}
	for _, s := range settings {
		key := [2]string{s.Currency, strings.ToLower(s.AssetType)}
		settingsMap[key] = struct{ min, max float64 }{s.MinPercent, s.MaxPercent}
	}

	byCurrency := map[string]struct {
		total       float64
		byAssetType map[string]float64
	}{}
	for _, h := range holdings {
		curr := h.Currency
		entry := byCurrency[curr]
		if entry.byAssetType == nil {
			entry.byAssetType = map[string]float64{}
		}
		priceKey := [2]string{h.Symbol, curr}
		marketValue := h.TotalCost
		if p, ok := latestPrices[priceKey]; ok && h.TotalShares > 0 {
			marketValue = p.Price * h.TotalShares
		}
		entry.total += marketValue
		asset := strings.ToLower(h.AssetType)
		if asset == "" {
			asset = "stock"
		}
		entry.byAssetType[asset] += marketValue
		byCurrency[curr] = entry
	}

	assetTypeCodes := make([]string, 0, len(assetTypes))
	assetTypeSet := map[string]struct{}{}
	for _, t := range assetTypes {
		assetTypeCodes = append(assetTypeCodes, t.Code)
		assetTypeSet[t.Code] = struct{}{}
	}

	holdingsTypes := map[string]struct{}{}
	for _, h := range holdings {
		asset := strings.ToLower(h.AssetType)
		if asset == "" {
			asset = "stock"
		}
		holdingsTypes[asset] = struct{}{}
	}

	missingTypes := []string{}
	for asset := range holdingsTypes {
		if _, ok := assetTypeSet[asset]; !ok {
			missingTypes = append(missingTypes, asset)
		}
	}

	orderedAssetTypes := []string{}
	for _, t := range DefaultAssetTypes {
		if _, ok := assetTypeSet[t]; ok {
			orderedAssetTypes = append(orderedAssetTypes, t)
		}
	}
	for _, t := range assetTypeCodes {
		if !contains(DefaultAssetTypes, t) {
			orderedAssetTypes = append(orderedAssetTypes, t)
		}
	}
	sort.Strings(missingTypes)
	orderedAssetTypes = append(orderedAssetTypes, missingTypes...)

	result := HoldingsByCurrencyResult{}
	for curr, data := range byCurrency {
		allocations := []AllocationEntry{}
		for _, assetType := range orderedAssetTypes {
			amount := data.byAssetType[assetType]
			percent := 0.0
			if data.total > 0 {
				percent = amount / data.total * 100
			}
			setting, ok := settingsMap[[2]string{curr, assetType}]
			if !ok {
				setting = struct{ min, max float64 }{0, 100}
			}
			warning := ""
			if percent < setting.min {
				warning = fmt.Sprintf("低于最小配置 %.0f%%", setting.min)
			} else if percent > setting.max {
				warning = fmt.Sprintf("超过最大配置 %.0f%%", setting.max)
			}
			label := labels[assetType]
			if label == "" {
				label = assetType
			}
			var warningPtr *string
			if warning != "" {
				warningPtr = &warning
			}
			allocations = append(allocations, AllocationEntry{
				AssetType:  assetType,
				Label:      label,
				Amount:     amount,
				Percent:    round2(percent),
				MinPercent: setting.min,
				MaxPercent: setting.max,
				Warning:    warningPtr,
			})
		}
		result[curr] = CurrencyAllocation{Total: data.total, Allocations: allocations}
	}
	if c.cache != nil {
		c.cache.setByCurrency(result)
	}
	return result, nil
}

// GetHoldingsByCurrencyAndAccount returns holdings grouped by currency and account.
func (c *Core) GetHoldingsByCurrencyAndAccount() (HoldingsByCurrencyAccountResult, error) {
	if c.cache != nil {
		if cached, ok := c.cache.getByCurrencyAccount(); ok {
			return cached, nil
		}
	}
	holdings, err := c.GetHoldings("")
	if err != nil {
		return nil, err
	}
	latestPrices, err := c.GetAllLatestPrices()
	if err != nil {
		return nil, err
	}
	labels, err := c.GetAssetTypeLabels()
	if err != nil {
		labels = DefaultAssetTypeLabels
	}
	accounts, err := c.GetAccounts()
	if err != nil {
		return nil, err
	}
	accountNames := map[string]string{}
	for _, acc := range accounts {
		name := strings.TrimSpace(acc.AccountName)
		if name == "" {
			name = acc.AccountID
		}
		accountNames[acc.AccountID] = name
	}

	byCurrency := map[string]map[string][]Holding{}
	for _, h := range holdings {
		if byCurrency[h.Currency] == nil {
			byCurrency[h.Currency] = map[string][]Holding{}
		}
		byCurrency[h.Currency][h.AccountID] = append(byCurrency[h.Currency][h.AccountID], h)
	}

	result := HoldingsByCurrencyAccountResult{}
	for curr, accountsMap := range byCurrency {
		currencyTotal := 0.0
		accountResults := map[string]AccountHoldings{}

		for accountID, items := range accountsMap {
			sort.Slice(items, func(i, j int) bool {
				return items[i].TotalCost > items[j].TotalCost
			})
			accountTotal := 0.0
			symbols := []AccountSymbolHolding{}
			for _, h := range items {
				priceKey := [2]string{h.Symbol, curr}
				marketValue := h.TotalCost
				if p, ok := latestPrices[priceKey]; ok && h.TotalShares > 0 {
					marketValue = p.Price * h.TotalShares
				}
				accountTotal += marketValue

				name := ""
				if h.Name != nil {
					name = strings.TrimSpace(*h.Name)
				}
				displayName := h.Symbol
				if name != "" {
					displayName = name
				}

				assetType := strings.ToLower(h.AssetType)
				label := labels[assetType]
				if label == "" {
					label = assetType
				}

				symbols = append(symbols, AccountSymbolHolding{
					Symbol:         h.Symbol,
					Name:           h.Name,
					DisplayName:    displayName,
					AssetType:      assetType,
					AssetTypeLabel: label,
					MarketValue:    marketValue,
					TotalShares:    h.TotalShares,
				})
			}
			for i := range symbols {
				if accountTotal > 0 {
					symbols[i].Percent = round2(symbols[i].MarketValue / accountTotal * 100)
				}
			}

			currencyTotal += accountTotal
			accountName := accountNames[accountID]
			if strings.TrimSpace(accountName) == "" {
				accountName = accountID
			}
			accountResults[accountID] = AccountHoldings{
				AccountName:      accountName,
				TotalMarketValue: accountTotal,
				Symbols:          symbols,
			}
		}

		result[curr] = CurrencyAccountHoldings{
			TotalMarketValue: currencyTotal,
			Accounts:         accountResults,
		}
	}
	if c.cache != nil {
		c.cache.setByCurrencyAccount(result)
	}
	return result, nil
}

// AdjustAssetValue creates an ADJUST transaction for value changes.
func (c *Core) AdjustAssetValue(symbol string, newValue float64, currency string, accountID string, assetType string, notes *string) (int64, error) {
	holdings, err := c.GetHoldings("")
	if err != nil {
		return 0, err
	}
	var currentValue float64
	for _, h := range holdings {
		if h.Symbol == normalizeSymbol(symbol) && h.Currency == normalizeCurrency(currency) && h.AccountID == accountID {
			currentValue = h.TotalCost
			break
		}
	}
	adjustment := newValue - currentValue
	text := notes
	if text == nil || strings.TrimSpace(*text) == "" {
		msg := fmt.Sprintf("价值调整: %.2f -> %.2f", currentValue, newValue)
		text = &msg
	}
	return c.AddTransaction(AddTransactionRequest{
		TransactionDate: todayISO(),
		Symbol:          symbol,
		TransactionType: "ADJUST",
		Quantity:        0,
		Price:           adjustment,
		AccountID:       accountID,
		AssetType:       assetType,
		Currency:        currency,
		Notes:           text,
		TotalAmount:     &adjustment,
	})
}

func (c *Core) getAutoUpdateMap() (map[string]int, error) {
	rows, err := c.db.Query("SELECT symbol, auto_update FROM symbols")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]int{}
	for rows.Next() {
		var symbol string
		var autoUpdate int
		if err := rows.Scan(&symbol, &autoUpdate); err != nil {
			return nil, err
		}
		result[symbol] = autoUpdate
	}
	return result, rows.Err()
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
