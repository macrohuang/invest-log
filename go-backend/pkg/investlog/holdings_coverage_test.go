package investlog

import "testing"

func TestHoldingsAggregations(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert account with empty name to cover fallback.
	if _, err := core.db.Exec("INSERT INTO accounts (account_id, account_name) VALUES (?, ?)", "acct", ""); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	// Insert asset type with empty label to cover label fallback.
	if _, err := core.db.Exec("INSERT INTO asset_types (code, label) VALUES (?, ?)", "crypto", ""); err != nil {
		t.Fatalf("insert asset type: %v", err)
	}

	// Stock transaction.
	if _, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-01",
		Symbol:          "AAA",
		TransactionType: "BUY",
		Quantity:        10,
		Price:           10,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	}); err != nil {
		t.Fatalf("add stock: %v", err)
	}
	// Cash transaction.
	if _, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-01",
		Symbol:          "CASH",
		TransactionType: "TRANSFER_IN",
		Quantity:        100,
		Price:           1,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "cash",
	}); err != nil {
		t.Fatalf("add cash: %v", err)
	}
	// Crypto transaction.
	if _, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-01",
		Symbol:          "BTC",
		TransactionType: "BUY",
		Quantity:        1,
		Price:           50,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "crypto",
	}); err != nil {
		t.Fatalf("add crypto: %v", err)
	}

	// Set symbol name for displayName branch.
	if _, err := core.UpdateSymbolMetadata("AAA", stringPtr("Alpha"), nil, nil, nil, nil); err != nil {
		t.Fatalf("update symbol metadata: %v", err)
	}

	// Latest prices to calculate PnL.
	_ = core.UpdateLatestPrice("AAA", "USD", 12)
	_ = core.UpdateLatestPrice("BTC", "USD", 40)

	holdings, err := core.GetHoldings("")
	if err != nil {
		t.Fatalf("GetHoldings: %v", err)
	}
	if len(holdings) == 0 {
		t.Fatalf("expected holdings")
	}
	for _, h := range holdings {
		if h.Symbol == "CASH" && h.AvgCost != 1 {
			t.Fatalf("expected cash avg cost 1, got %.2f", h.AvgCost)
		}
	}

	bySymbol, err := core.GetHoldingsBySymbol()
	if err != nil {
		t.Fatalf("GetHoldingsBySymbol: %v", err)
	}
	usd := bySymbol["USD"]
	if len(usd.Symbols) == 0 {
		t.Fatalf("expected USD symbols")
	}
	foundCrypto := false
	for _, s := range usd.Symbols {
		if s.Symbol == "BTC" {
			foundCrypto = true
			if s.AssetTypeLabel != "crypto" {
				t.Fatalf("expected crypto label fallback, got %q", s.AssetTypeLabel)
			}
		}
		if s.Symbol == "AAA" && s.DisplayName != "Alpha" {
			t.Fatalf("expected display name from metadata")
		}
		if s.AccountName != "acct" {
			t.Fatalf("expected account name fallback to id")
		}
	}
	if !foundCrypto {
		t.Fatalf("expected crypto symbol")
	}

	// Allocation settings to trigger warnings.
	_, _ = core.SetAllocationSetting("USD", "stock", 80, 90) // low warning
	_, _ = core.SetAllocationSetting("USD", "crypto", 0, 10) // high warning

	byCurrency, err := core.GetHoldingsByCurrency()
	if err != nil {
		t.Fatalf("GetHoldingsByCurrency: %v", err)
	}
	alloc := byCurrency["USD"].Allocations
	if len(alloc) == 0 {
		t.Fatalf("expected allocations")
	}
	warned := 0
	for _, a := range alloc {
		if a.Warning != nil {
			warned++
		}
	}
	if warned == 0 {
		t.Fatalf("expected allocation warnings")
	}

	byCurrAcct, err := core.GetHoldingsByCurrencyAndAccount()
	if err != nil {
		t.Fatalf("GetHoldingsByCurrencyAndAccount: %v", err)
	}
	acctHoldings := byCurrAcct["USD"].Accounts["acct"]
	if acctHoldings.AccountName != "acct" {
		t.Fatalf("expected account name fallback")
	}

	// AdjustAssetValue with nil notes should generate default message.
	if _, err := core.AdjustAssetValue("AAA", 200, "USD", "acct", "stock", nil); err != nil {
		t.Fatalf("AdjustAssetValue: %v", err)
	}
}
