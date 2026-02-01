package investlog

import "testing"

func TestOperationLogs(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	val := 123.45
	logID, err := core.AddOperationLog(OperationLog{
		Operation:    "PRICE_UPDATE",
		Symbol:       stringPtr("AAPL"),
		Currency:     stringPtr("USD"),
		Details:      stringPtr("ok"),
		OldValue:     &val,
		NewValue:     &val,
		PriceFetched: &val,
	})
	if err != nil {
		t.Fatalf("AddOperationLog: %v", err)
	}
	if logID == 0 {
		t.Fatalf("expected log id")
	}

	_, err = core.AddOperationLog(OperationLog{Operation: "PRICE_UPDATE_FAILED"})
	if err != nil {
		t.Fatalf("AddOperationLog (empty): %v", err)
	}

	logs, err := core.GetOperationLogs(0, -1)
	if err != nil {
		t.Fatalf("GetOperationLogs: %v", err)
	}
	if len(logs) < 2 {
		t.Fatalf("expected at least 2 logs, got %d", len(logs))
	}
	if logs[0].Operation == "" {
		t.Fatalf("expected operation in log")
	}
}
