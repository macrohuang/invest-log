package investlog

import (
	"encoding/json"
	"fmt"
)

// HoldingsAnalysisRequest defines inputs for AI holdings analysis.
type HoldingsAnalysisRequest struct {
	BaseURL         string
	APIKey          string
	Model           string
	Currency        string
	RiskProfile     string
	Horizon         string
	AdviceStyle     string
	AllowNewSymbols bool
	StrategyPrompt  string
	AnalysisType    string // "adhoc", "weekly", "monthly"
}

// HoldingsSymbolRef is a brief summary of a symbol's latest AI analysis used as context.
type HoldingsSymbolRef struct {
	Symbol    string `json:"symbol"`
	ID        int64  `json:"id"`
	Rating    string `json:"rating"`
	Action    string `json:"action"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

// HoldingsAnalysisRecommendation contains one actionable recommendation.
type HoldingsAnalysisRecommendation struct {
	Symbol       string `json:"symbol,omitempty"`
	Action       string `json:"action"`
	TheoryTag    string `json:"theory_tag"`
	Rationale    string `json:"rationale"`
	TargetWeight string `json:"target_weight,omitempty"`
	Priority     string `json:"priority,omitempty"`
}

// UnmarshalJSON handles model responses where target_weight or priority
// may be returned as a number instead of a string.
func (r *HoldingsAnalysisRecommendation) UnmarshalJSON(data []byte) error {
	var raw struct {
		Symbol       string `json:"symbol"`
		Action       string `json:"action"`
		TheoryTag    string `json:"theory_tag"`
		Rationale    string `json:"rationale"`
		TargetWeight any    `json:"target_weight"`
		Priority     any    `json:"priority"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Symbol = raw.Symbol
	r.Action = raw.Action
	r.TheoryTag = raw.TheoryTag
	r.Rationale = raw.Rationale
	r.TargetWeight = anyToString(raw.TargetWeight)
	r.Priority = anyToString(raw.Priority)
	return nil
}

func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

// HoldingsAnalysisResult is the structured response returned to clients.
type HoldingsAnalysisResult struct {
	ID              int64                            `json:"id,omitempty"`
	GeneratedAt     string                           `json:"generated_at"`
	Model           string                           `json:"model"`
	Currency        string                           `json:"currency,omitempty"`
	AnalysisType    string                           `json:"analysis_type,omitempty"`
	OverallSummary  string                           `json:"overall_summary"`
	RiskLevel       string                           `json:"risk_level"`
	KeyFindings     []string                         `json:"key_findings"`
	Recommendations []HoldingsAnalysisRecommendation `json:"recommendations"`
	Disclaimer      string                           `json:"disclaimer"`
	SymbolRefs      []HoldingsSymbolRef              `json:"symbol_refs,omitempty"`
}

type holdingsAnalysisCurrencySnapshot struct {
	Currency string                       `json:"currency"`
	Symbols  []holdingsAnalysisSymbolItem `json:"symbols"`
}

type holdingsAnalysisSymbolItem struct {
	Symbol    string   `json:"symbol"`
	WeightPct float64  `json:"weight_pct"`
	PnLPct    *float64 `json:"pnl_pct,omitempty"`
	AvgCost   float64  `json:"avg_cost"`
}

type holdingsAnalysisPromptInput struct {
	RiskProfile     string                             `json:"risk_profile"`
	Horizon         string                             `json:"horizon"`
	AdviceStyle     string                             `json:"advice_style"`
	AllowNewSymbols bool                               `json:"allow_new_symbols"`
	StrategyPrompt  string                             `json:"strategy_prompt,omitempty"`
	Holdings        []holdingsAnalysisCurrencySnapshot `json:"holdings"`
}

type holdingsAnalysisModelResponse struct {
	OverallSummary  string                           `json:"overall_summary"`
	RiskLevel       string                           `json:"risk_level"`
	KeyFindings     []string                         `json:"key_findings"`
	Recommendations []HoldingsAnalysisRecommendation `json:"recommendations"`
	Disclaimer      string                           `json:"disclaimer"`
}
