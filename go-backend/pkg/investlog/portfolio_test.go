package investlog

import (
	"strings"
	"testing"
)

func TestGetPortfolioHistory(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	testAccount(t, core, "acct", "Account")

	_, err := core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-01",
		Symbol:          "AAA",
		TransactionType: "BUY",
		Quantity:        10,
		Price:           10,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("AddTransaction BUY: %v", err)
	}
	_, err = core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-02",
		Symbol:          "AAA",
		TransactionType: "SELL",
		Quantity:        5,
		Price:           12,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("AddTransaction SELL: %v", err)
	}
	_, err = core.AddTransaction(AddTransactionRequest{
		TransactionDate: "2024-01-03",
		Symbol:          "AAA",
		TransactionType: "BUY",
		Quantity:        2,
		Price:           20,
		Currency:        "USD",
		AccountID:       "acct",
		AssetType:       "stock",
	})
	if err != nil {
		t.Fatalf("AddTransaction BUY 2: %v", err)
	}

	points, err := core.GetPortfolioHistory(10)
	if err != nil {
		t.Fatalf("GetPortfolioHistory: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	if dateOnly(points[0].Date) != "2024-01-01" || points[0].Value != 100 {
		t.Fatalf("unexpected first point: %+v", points[0])
	}
	if dateOnly(points[1].Date) != "2024-01-02" || points[1].Value != 40 {
		t.Fatalf("unexpected second point: %+v", points[1])
	}
	if dateOnly(points[2].Date) != "2024-01-03" || points[2].Value != 80 {
		t.Fatalf("unexpected third point: %+v", points[2])
	}
}

func dateOnly(value string) string {
	if idx := strings.Index(value, "T"); idx > 0 {
		return value[:idx]
	}
	return value
}
