package investlog

import "testing"

func TestMethodsOnClosedDBReturnError(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	_ = core.Close()

	if _, err := core.CheckAccountInUse("acct"); err == nil {
		t.Fatalf("expected error from CheckAccountInUse")
	}
	if _, err := core.AddOperationLog(OperationLog{Operation: "TEST"}); err == nil {
		t.Fatalf("expected error from AddOperationLog")
	}
	if _, err := core.SetAllocationSetting("USD", "stock", 0, 10); err == nil {
		t.Fatalf("expected error from SetAllocationSetting")
	}
	if _, err := core.DeleteAllocationSetting("USD", "stock"); err == nil {
		t.Fatalf("expected error from DeleteAllocationSetting")
	}
	if _, err := core.GetSymbols(); err == nil {
		t.Fatalf("expected error from GetSymbols")
	}
	if _, err := core.GetAccounts(); err == nil {
		t.Fatalf("expected error from GetAccounts")
	}
	if _, err := core.GetAssetTypes(); err == nil {
		t.Fatalf("expected error from GetAssetTypes")
	}
	if _, err := core.GetTransactions(TransactionFilter{}); err == nil {
		t.Fatalf("expected error from GetTransactions")
	}
	if _, err := core.GetTransaction(1); err == nil {
		t.Fatalf("expected error from GetTransaction")
	}
	if _, err := core.DeleteTransaction(1); err == nil {
		t.Fatalf("expected error from DeleteTransaction")
	}
	name := "Name"
	if _, err := core.UpdateSymbolMetadata("AAA", &name, nil, nil, nil, nil); err == nil {
		t.Fatalf("expected error from UpdateSymbolMetadata")
	}
	if _, _, _, err := core.UpdateSymbolAssetType("AAA", "stock"); err == nil {
		t.Fatalf("expected error from UpdateSymbolAssetType")
	}
	if _, err := core.UpdateSymbolAutoUpdate("AAA", 1); err == nil {
		t.Fatalf("expected error from UpdateSymbolAutoUpdate")
	}
	if err := core.ManualUpdatePrice("AAA", "USD", NewAmount(1.0)); err == nil {
		t.Fatalf("expected error from ManualUpdatePrice")
	}
}
