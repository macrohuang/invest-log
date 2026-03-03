package investlog

import (
	"fmt"
	"strings"
)

func normalizeSymbolAnalysisRequest(req SymbolAnalysisRequest) (SymbolAnalysisRequest, error) {
	normalized := req
	normalized.APIKey = strings.TrimSpace(req.APIKey)
	if normalized.APIKey == "" {
		return SymbolAnalysisRequest{}, fmt.Errorf("api_key is required")
	}
	normalized.Model = strings.TrimSpace(req.Model)
	if normalized.Model == "" {
		return SymbolAnalysisRequest{}, fmt.Errorf("model is required")
	}
	normalized.Symbol = strings.TrimSpace(strings.ToUpper(req.Symbol))
	if normalized.Symbol == "" {
		return SymbolAnalysisRequest{}, fmt.Errorf("symbol is required")
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		return SymbolAnalysisRequest{}, fmt.Errorf("currency is required")
	}
	if !contains(Currencies, currency) {
		return SymbolAnalysisRequest{}, fmt.Errorf("invalid currency: %s", req.Currency)
	}
	normalized.Currency = currency

	riskProfile, err := normalizeEnum(strings.TrimSpace(req.RiskProfile), "balanced", map[string]struct{}{
		"conservative": {},
		"balanced":     {},
		"aggressive":   {},
	})
	if err != nil {
		return SymbolAnalysisRequest{}, fmt.Errorf("invalid risk_profile: %w", err)
	}
	normalized.RiskProfile = riskProfile

	horizon, err := normalizeEnum(strings.TrimSpace(req.Horizon), "medium", map[string]struct{}{
		"short":  {},
		"medium": {},
		"long":   {},
	})
	if err != nil {
		return SymbolAnalysisRequest{}, fmt.Errorf("invalid horizon: %w", err)
	}
	normalized.Horizon = horizon

	adviceStyle, err := normalizeEnum(strings.TrimSpace(req.AdviceStyle), "balanced", map[string]struct{}{
		"conservative": {},
		"balanced":     {},
		"aggressive":   {},
	})
	if err != nil {
		return SymbolAnalysisRequest{}, fmt.Errorf("invalid advice_style: %w", err)
	}
	normalized.AdviceStyle = adviceStyle

	normalized.StrategyPrompt = strings.TrimSpace(req.StrategyPrompt)
	return normalized, nil
}
