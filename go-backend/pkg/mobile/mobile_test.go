package mobile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupMobileCore(t *testing.T) (*Core, func()) {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	core, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	cleanup := func() {
		_ = core.Close()
		_ = os.RemoveAll(tmp)
	}
	return core, cleanup
}

func TestMobileCoreJSONFlows(t *testing.T) {
	core, cleanup := setupMobileCore(t)
	defer cleanup()

	payload := map[string]any{
		"symbol":           "AAPL",
		"transaction_type": "BUY",
		"quantity":         10,
		"price":            150,
		"currency":         "USD",
		"account_id":       "acct",
		"asset_type":       "stock",
	}
	payloadBytes, _ := json.Marshal(payload)
	resp, err := core.AddTransactionJSON(string(payloadBytes))
	if err != nil {
		t.Fatalf("AddTransactionJSON: %v", err)
	}
	var addResp map[string]any
	if err := json.Unmarshal([]byte(resp), &addResp); err != nil {
		t.Fatalf("unmarshal add response: %v", err)
	}
	idVal := addResp["id"]
	if idVal == nil {
		t.Fatalf("expected id in response")
	}

	filterJSON := `{"symbol":"AAPL","limit":10}`
	_, err = core.GetTransactionsJSON(filterJSON)
	if err != nil {
		t.Fatalf("GetTransactionsJSON: %v", err)
	}

	if _, err := core.GetHoldingsJSON(""); err != nil {
		t.Fatalf("GetHoldingsJSON: %v", err)
	}
	if _, err := core.GetHoldingsByCurrencyJSON(); err != nil {
		t.Fatalf("GetHoldingsByCurrencyJSON: %v", err)
	}
	if _, err := core.GetHoldingsBySymbolJSON(); err != nil {
		t.Fatalf("GetHoldingsBySymbolJSON: %v", err)
	}

	if err := core.ManualUpdatePrice("CASH", "USD", 1.0); err != nil {
		t.Fatalf("ManualUpdatePrice: %v", err)
	}
	priceResp, err := core.UpdatePriceJSON(`{"symbol":"CASH","currency":"USD"}`)
	if err != nil {
		t.Fatalf("UpdatePriceJSON: %v", err)
	}
	var priceData map[string]any
	if err := json.Unmarshal([]byte(priceResp), &priceData); err != nil {
		t.Fatalf("unmarshal price response: %v", err)
	}
	if priceData["price"] == nil {
		t.Fatalf("expected price in response")
	}

	// Delete transaction
	idFloat, ok := idVal.(float64)
	if !ok {
		t.Fatalf("expected id to be number, got %T", idVal)
	}
	deleted, err := core.DeleteTransaction(int64(idFloat))
	if err != nil {
		t.Fatalf("DeleteTransaction: %v", err)
	}
	if !deleted {
		t.Fatalf("expected delete to return true")
	}
}

func TestMobileCoreInvalidJSON(t *testing.T) {
	core, cleanup := setupMobileCore(t)
	defer cleanup()

	if _, err := core.GetTransactionsJSON("{bad json}"); err == nil {
		t.Fatalf("expected error for invalid filter JSON")
	}
	if _, err := core.AddTransactionJSON("{bad json}"); err == nil {
		t.Fatalf("expected error for invalid transaction JSON")
	}

	if _, err := core.UpdatePriceJSON(`{"symbol":"???","currency":"CNY"}`); err == nil {
		t.Fatalf("expected error for unknown price fetch")
	}
}

func TestMobileCoreCloseNil(t *testing.T) {
	var c *Core
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
