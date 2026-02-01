package investlog

import (
	"strings"
	"testing"
)

func TestGetAllocationSettings(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Initially no settings
	settings, err := core.GetAllocationSettings("")
	assertNoError(t, err, "get allocation settings")
	if len(settings) != 0 {
		t.Errorf("expected 0 settings initially, got %d", len(settings))
	}
}

func TestSetAllocationSetting(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add a setting
	success, err := core.SetAllocationSetting("USD", "stock", 40, 60)
	assertNoError(t, err, "set allocation setting")
	if !success {
		t.Error("expected success")
	}

	// Verify it exists
	settings, err := core.GetAllocationSettings("")
	assertNoError(t, err, "get settings")
	if len(settings) != 1 {
		t.Fatalf("expected 1 setting, got %d", len(settings))
	}

	s := settings[0]
	if s.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", s.Currency)
	}
	if s.AssetType != "stock" {
		t.Errorf("expected asset_type stock, got %s", s.AssetType)
	}
	assertFloatEquals(t, s.MinPercent, 40, "min percent")
	assertFloatEquals(t, s.MaxPercent, 60, "max percent")
}

func TestSetAllocationSetting_Upsert(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add initial setting
	_, err := core.SetAllocationSetting("USD", "stock", 40, 60)
	assertNoError(t, err, "set initial setting")

	// Update with same currency/asset_type
	_, err = core.SetAllocationSetting("USD", "stock", 50, 70)
	assertNoError(t, err, "update setting")

	// Should still only have 1 setting with new values
	settings, _ := core.GetAllocationSettings("")
	if len(settings) != 1 {
		t.Fatalf("expected 1 setting after upsert, got %d", len(settings))
	}

	s := settings[0]
	assertFloatEquals(t, s.MinPercent, 50, "updated min percent")
	assertFloatEquals(t, s.MaxPercent, 70, "updated max percent")
}

func TestSetAllocationSetting_Validation(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name      string
		currency  string
		assetType string
		minPct    float64
		maxPct    float64
		wantErr   string
	}{
		{
			name:      "invalid currency",
			currency:  "EUR",
			assetType: "stock",
			minPct:    40,
			maxPct:    60,
			wantErr:   "invalid currency",
		},
		{
			name:      "min greater than max",
			currency:  "USD",
			assetType: "stock",
			minPct:    70,
			maxPct:    60,
			wantErr:   "invalid percent range",
		},
		{
			name:      "negative min",
			currency:  "USD",
			assetType: "stock",
			minPct:    -10,
			maxPct:    60,
			wantErr:   "invalid percent range",
		},
		{
			name:      "max over 100",
			currency:  "USD",
			assetType: "stock",
			minPct:    40,
			maxPct:    110,
			wantErr:   "invalid percent range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := core.SetAllocationSetting(tt.currency, tt.assetType, tt.minPct, tt.maxPct)
			assertError(t, err, tt.name)
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestSetAllocationSetting_ValidValues(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Test valid edge cases
	validCases := []struct {
		currency  string
		assetType string
		minPct    float64
		maxPct    float64
	}{
		{"CNY", "stock", 0, 100},       // Full range
		{"USD", "bond", 0, 0},          // Zero allocation
		{"HKD", "cash", 100, 100},      // All in one type
		{"CNY", "metal", 25.5, 75.5},   // Decimal values
	}

	for _, tc := range validCases {
		_, err := core.SetAllocationSetting(tc.currency, tc.assetType, tc.minPct, tc.maxPct)
		assertNoError(t, err, "valid case")
	}

	settings, _ := core.GetAllocationSettings("")
	if len(settings) != 4 {
		t.Errorf("expected 4 settings, got %d", len(settings))
	}
}

func TestGetAllocationSettings_FilterByCurrency(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add settings for different currencies
	_, _ = core.SetAllocationSetting("USD", "stock", 40, 60)
	_, _ = core.SetAllocationSetting("USD", "bond", 20, 40)
	_, _ = core.SetAllocationSetting("CNY", "stock", 50, 70)

	// Get all
	allSettings, err := core.GetAllocationSettings("")
	assertNoError(t, err, "get all settings")
	if len(allSettings) != 3 {
		t.Errorf("expected 3 total settings, got %d", len(allSettings))
	}

	// Get USD only
	usdSettings, err := core.GetAllocationSettings("USD")
	assertNoError(t, err, "get USD settings")
	if len(usdSettings) != 2 {
		t.Errorf("expected 2 USD settings, got %d", len(usdSettings))
	}
	for _, s := range usdSettings {
		if s.Currency != "USD" {
			t.Errorf("expected USD currency, got %s", s.Currency)
		}
	}

	// Get CNY only
	cnySettings, err := core.GetAllocationSettings("CNY")
	assertNoError(t, err, "get CNY settings")
	if len(cnySettings) != 1 {
		t.Errorf("expected 1 CNY setting, got %d", len(cnySettings))
	}
}

func TestDeleteAllocationSetting(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add a setting
	_, _ = core.SetAllocationSetting("USD", "stock", 40, 60)

	// Delete it
	deleted, err := core.DeleteAllocationSetting("USD", "stock")
	assertNoError(t, err, "delete setting")
	if !deleted {
		t.Error("expected setting to be deleted")
	}

	// Verify it's gone
	settings, _ := core.GetAllocationSettings("")
	if len(settings) != 0 {
		t.Error("expected 0 settings after deletion")
	}
}

func TestDeleteAllocationSetting_NonExistent(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	deleted, err := core.DeleteAllocationSetting("USD", "nonexistent")
	assertNoError(t, err, "delete non-existent")
	if deleted {
		t.Error("should not report deleted for non-existent setting")
	}
}

func TestAllocationSetting_CurrencyNormalization(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add with lowercase currency
	_, err := core.SetAllocationSetting("usd", "stock", 40, 60)
	assertNoError(t, err, "set with lowercase currency")

	// Should be stored as uppercase
	settings, _ := core.GetAllocationSettings("USD")
	if len(settings) != 1 {
		t.Error("expected setting with normalized currency")
	}
}

func TestAllocationSetting_AssetTypeNormalization(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	// Add with uppercase asset type
	_, err := core.SetAllocationSetting("USD", "STOCK", 40, 60)
	assertNoError(t, err, "set with uppercase asset type")

	// Should be stored as lowercase
	settings, _ := core.GetAllocationSettings("")
	if len(settings) != 1 {
		t.Fatalf("expected 1 setting")
	}
	if settings[0].AssetType != "stock" {
		t.Errorf("expected normalized asset type 'stock', got '%s'", settings[0].AssetType)
	}
}
