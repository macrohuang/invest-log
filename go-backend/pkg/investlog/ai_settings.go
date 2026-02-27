package investlog

import (
	"database/sql"
	"strings"
)

const (
	defaultAISettingsBaseURL     = "https://api.openai.com/v1"
	defaultAISettingsRiskProfile = "balanced"
	defaultAISettingsHorizon     = "medium"
	defaultAISettingsAdviceStyle = "balanced"
)

var validAIRiskProfiles = map[string]struct{}{
	"conservative": {},
	"balanced":     {},
	"aggressive":   {},
}

var validAIHorizons = map[string]struct{}{
	"short":  {},
	"medium": {},
	"long":   {},
}

var validAIAdviceStyles = map[string]struct{}{
	"conservative": {},
	"balanced":     {},
	"aggressive":   {},
}

func defaultAISettings() AISettings {
	return AISettings{
		BaseURL:         defaultAISettingsBaseURL,
		Model:           "",
		RiskProfile:     defaultAISettingsRiskProfile,
		Horizon:         defaultAISettingsHorizon,
		AdviceStyle:     defaultAISettingsAdviceStyle,
		AllowNewSymbols: true,
		StrategyPrompt:  "",
	}
}

func trimTrailingSlash(value string) string {
	trimmed := strings.TrimSpace(value)
	return strings.TrimRight(trimmed, "/")
}

func normalizeAISettings(settings AISettings) AISettings {
	normalized := settings
	normalized.BaseURL = trimTrailingSlash(normalized.BaseURL)
	if normalized.BaseURL == "" {
		normalized.BaseURL = defaultAISettingsBaseURL
	}
	normalized.Model = strings.TrimSpace(normalized.Model)
	normalized.RiskProfile = strings.ToLower(strings.TrimSpace(normalized.RiskProfile))
	normalized.Horizon = strings.ToLower(strings.TrimSpace(normalized.Horizon))
	normalized.AdviceStyle = strings.ToLower(strings.TrimSpace(normalized.AdviceStyle))
	normalized.StrategyPrompt = strings.TrimSpace(normalized.StrategyPrompt)

	if _, ok := validAIRiskProfiles[normalized.RiskProfile]; !ok {
		normalized.RiskProfile = defaultAISettingsRiskProfile
	}
	if _, ok := validAIHorizons[normalized.Horizon]; !ok {
		normalized.Horizon = defaultAISettingsHorizon
	}
	if _, ok := validAIAdviceStyles[normalized.AdviceStyle]; !ok {
		normalized.AdviceStyle = defaultAISettingsAdviceStyle
	}
	return normalized
}

// GetAISettings returns persisted AI settings (excluding API key).
func (c *Core) GetAISettings() (AISettings, error) {
	settings := defaultAISettings()
	var allowNewSymbols int

	err := c.db.QueryRow(`
		SELECT base_url, model, risk_profile, horizon, advice_style, allow_new_symbols, strategy_prompt
		FROM ai_settings
		WHERE id = 1
	`).Scan(
		&settings.BaseURL,
		&settings.Model,
		&settings.RiskProfile,
		&settings.Horizon,
		&settings.AdviceStyle,
		&allowNewSymbols,
		&settings.StrategyPrompt,
	)
	if err == sql.ErrNoRows {
		return settings, nil
	}
	if err != nil {
		return AISettings{}, err
	}
	settings.AllowNewSymbols = allowNewSymbols != 0
	return normalizeAISettings(settings), nil
}

// SetAISettings persists AI settings (excluding API key).
func (c *Core) SetAISettings(settings AISettings) (AISettings, error) {
	normalized := normalizeAISettings(settings)
	allowNewSymbols := 0
	if normalized.AllowNewSymbols {
		allowNewSymbols = 1
	}

	_, err := c.db.Exec(`
		INSERT INTO ai_settings (
			id, base_url, model, risk_profile, horizon, advice_style, allow_new_symbols, strategy_prompt, updated_at
		)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			base_url = excluded.base_url,
			model = excluded.model,
			risk_profile = excluded.risk_profile,
			horizon = excluded.horizon,
			advice_style = excluded.advice_style,
			allow_new_symbols = excluded.allow_new_symbols,
			strategy_prompt = excluded.strategy_prompt,
			updated_at = CURRENT_TIMESTAMP
	`, normalized.BaseURL, normalized.Model, normalized.RiskProfile, normalized.Horizon, normalized.AdviceStyle, allowNewSymbols, normalized.StrategyPrompt)
	if err != nil {
		return AISettings{}, err
	}
	return normalized, nil
}
