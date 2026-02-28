package investlog

import "testing"

func TestGetAISettingsDefaults(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	settings, err := core.GetAISettings()
	assertNoError(t, err, "get defaults")

	if settings.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("unexpected default base url: %q", settings.BaseURL)
	}
	if settings.Model != "" {
		t.Fatalf("expected empty default model, got %q", settings.Model)
	}
	if settings.RiskProfile != "balanced" {
		t.Fatalf("unexpected default risk profile: %q", settings.RiskProfile)
	}
	if settings.Horizon != "medium" {
		t.Fatalf("unexpected default horizon: %q", settings.Horizon)
	}
	if settings.AdviceStyle != "balanced" {
		t.Fatalf("unexpected default advice style: %q", settings.AdviceStyle)
	}
	if !settings.AllowNewSymbols {
		t.Fatal("expected allow_new_symbols default true")
	}
	if settings.StrategyPrompt != "" {
		t.Fatalf("expected empty strategy prompt, got %q", settings.StrategyPrompt)
	}
}

func TestSetAISettingsPersistsAndNormalizes(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	saved, err := core.SetAISettings(AISettings{
		BaseURL:         " https://example.com/v1/ ",
		Model:           " gpt-4o-mini ",
		RiskProfile:     "aggressive",
		Horizon:         "long",
		AdviceStyle:     "conservative",
		AllowNewSymbols: false,
		StrategyPrompt:  " prefer dividends ",
	})
	assertNoError(t, err, "set ai settings")

	if saved.BaseURL != "https://example.com/v1" {
		t.Fatalf("unexpected normalized base url: %q", saved.BaseURL)
	}
	if saved.Model != "gpt-4o-mini" {
		t.Fatalf("unexpected normalized model: %q", saved.Model)
	}
	if saved.RiskProfile != "aggressive" {
		t.Fatalf("unexpected risk profile: %q", saved.RiskProfile)
	}
	if saved.Horizon != "long" {
		t.Fatalf("unexpected horizon: %q", saved.Horizon)
	}
	if saved.AdviceStyle != "conservative" {
		t.Fatalf("unexpected advice style: %q", saved.AdviceStyle)
	}
	if saved.AllowNewSymbols {
		t.Fatal("expected allow_new_symbols false")
	}
	if saved.StrategyPrompt != "prefer dividends" {
		t.Fatalf("unexpected strategy prompt: %q", saved.StrategyPrompt)
	}

	loaded, err := core.GetAISettings()
	assertNoError(t, err, "get ai settings")

	if loaded != saved {
		t.Fatalf("loaded settings mismatch: got %+v, want %+v", loaded, saved)
	}
}

func TestSetAISettingsInvalidEnumsFallbackToDefault(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	saved, err := core.SetAISettings(AISettings{
		BaseURL:         "",
		Model:           "m",
		RiskProfile:     "wild",
		Horizon:         "daytrade",
		AdviceStyle:     "rocket",
		AllowNewSymbols: true,
		StrategyPrompt:  "",
	})
	assertNoError(t, err, "set ai settings with invalid enums")

	if saved.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("expected default base url, got %q", saved.BaseURL)
	}
	if saved.RiskProfile != "balanced" {
		t.Fatalf("expected default risk profile, got %q", saved.RiskProfile)
	}
	if saved.Horizon != "medium" {
		t.Fatalf("expected default horizon, got %q", saved.Horizon)
	}
	if saved.AdviceStyle != "balanced" {
		t.Fatalf("expected default advice style, got %q", saved.AdviceStyle)
	}
}

func TestGetAISettingsReturnsDefaultsWhenRowMissing(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	if _, err := core.db.Exec("DELETE FROM ai_settings WHERE id = 1"); err != nil {
		t.Fatalf("delete ai_settings row: %v", err)
	}

	settings, err := core.GetAISettings()
	assertNoError(t, err, "get ai settings with missing row")

	if settings.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("unexpected default base url: %q", settings.BaseURL)
	}
	if !settings.AllowNewSymbols {
		t.Fatal("expected default allow_new_symbols true")
	}
}

func TestAISettingsMethodsWhenDBClosed(t *testing.T) {
	core, cleanup := setupTestDB(t)
	defer cleanup()

	if err := core.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	if _, err := core.GetAISettings(); err == nil {
		t.Fatal("expected GetAISettings error on closed db")
	}
	if _, err := core.SetAISettings(AISettings{Model: "m"}); err == nil {
		t.Fatal("expected SetAISettings error on closed db")
	}
}
