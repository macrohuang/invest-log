package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestTransferEndpoint_Success(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	// Setup accounts
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id": "acct-a", "account_name": "Account A",
	})
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id": "acct-b", "account_name": "Account B",
	})

	// Buy shares in acct-a
	doRequest(router, "POST", "/api/transactions", map[string]interface{}{
		"symbol": "AAPL", "transaction_type": "BUY",
		"quantity": 100, "price": 150, "currency": "USD",
		"account_id": "acct-a", "asset_type": "stock",
	})

	// Transfer
	rr := doRequest(router, "POST", "/api/transfers", map[string]interface{}{
		"symbol":          "AAPL",
		"quantity":        40,
		"from_account_id": "acct-a",
		"to_account_id":   "acct-b",
		"from_currency":   "USD",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/transfers: expected 200, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var result struct {
		TransferOutID int64   `json:"transfer_out_id"`
		TransferInID  int64   `json:"transfer_in_id"`
		ExchangeRate  float64 `json:"exchange_rate"`
	}
	json.NewDecoder(rr.Body).Decode(&result)

	if result.TransferOutID == 0 || result.TransferInID == 0 {
		t.Errorf("expected non-zero IDs, got out=%d in=%d", result.TransferOutID, result.TransferInID)
	}
}

func TestTransferEndpoint_ValidationErrors(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id": "acct-a", "account_name": "Account A",
	})
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id": "acct-b", "account_name": "Account B",
	})

	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "missing symbol",
			body: map[string]interface{}{
				"quantity": 10, "from_account_id": "acct-a",
				"to_account_id": "acct-b", "from_currency": "USD",
			},
		},
		{
			name: "same account",
			body: map[string]interface{}{
				"symbol": "AAPL", "quantity": 10,
				"from_account_id": "acct-a", "to_account_id": "acct-a",
				"from_currency": "USD",
			},
		},
		{
			name: "zero quantity",
			body: map[string]interface{}{
				"symbol": "AAPL", "quantity": 0,
				"from_account_id": "acct-a", "to_account_id": "acct-b",
				"from_currency": "USD",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := doRequest(router, "POST", "/api/transfers", tc.body)
			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d, body: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestTransferEndpoint_DeleteLinkedTransaction(t *testing.T) {
	router, cleanup := setupTestRouter(t)
	defer cleanup()

	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id": "acct-a", "account_name": "Account A",
	})
	doRequest(router, "POST", "/api/accounts", map[string]interface{}{
		"account_id": "acct-b", "account_name": "Account B",
	})
	doRequest(router, "POST", "/api/transactions", map[string]interface{}{
		"symbol": "AAPL", "transaction_type": "BUY",
		"quantity": 100, "price": 150, "currency": "USD",
		"account_id": "acct-a", "asset_type": "stock",
	})

	// Transfer
	rr := doRequest(router, "POST", "/api/transfers", map[string]interface{}{
		"symbol": "AAPL", "quantity": 30,
		"from_account_id": "acct-a", "to_account_id": "acct-b",
		"from_currency": "USD",
	})
	var result struct {
		TransferOutID int64 `json:"transfer_out_id"`
		TransferInID  int64 `json:"transfer_in_id"`
	}
	json.NewDecoder(rr.Body).Decode(&result)

	// Delete via the TRANSFER_IN side
	rr = doRequest(router, "DELETE", fmt.Sprintf("/api/transactions/%d", result.TransferInID), nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("DELETE linked transaction: expected 200, got %d", rr.Code)
	}

	// Both should be gone
	rr = doRequest(router, "GET", "/api/transactions?paged=1", nil)
	var txns struct {
		Items []map[string]interface{} `json:"items"`
		Total int                      `json:"total"`
	}
	json.NewDecoder(rr.Body).Decode(&txns)

	// Only the original BUY should remain
	if txns.Total != 1 {
		t.Errorf("expected 1 transaction remaining (BUY), got %d", txns.Total)
	}
}

