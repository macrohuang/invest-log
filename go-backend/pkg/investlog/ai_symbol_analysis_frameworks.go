package investlog

import (
	"fmt"
	"sort"
	"strings"
)

func buildFrameworkSystemPrompt(spec symbolFrameworkSpec) string {
	return fmt.Sprintf(frameworkAnalysisSystemPromptTemplate, spec.ID, spec.Name, spec.Focus, spec.ID)
}

func containsAnyKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func selectSymbolFrameworks(contextData *symbolContextData, enrichedContext string) []symbolFrameworkSpec {
	scores := make(map[string]int, len(symbolFrameworkCatalog))
	for idx, spec := range symbolFrameworkCatalog {
		// Base score keeps selection deterministic when heuristic scores tie.
		scores[spec.ID] = len(symbolFrameworkCatalog) - idx
	}

	addScore := func(frameworkID string, score int) {
		if _, ok := scores[frameworkID]; ok {
			scores[frameworkID] += score
		}
	}

	// Default baseline for single-stock analysis.
	addScore("dcf", 8)
	addScore("dynamic_moat", 7)
	addScore("relative_valuation", 7)

	if contextData != nil {
		if contextData.PnLPercent >= 20 || contextData.PnLPercent <= -10 {
			addScore("reverse_dcf", 8)
			addScore("expectations_investing", 6)
		}

		switch contextData.AllocationStatus {
		case "above_target":
			addScore("relative_valuation", 8)
			addScore("reverse_dcf", 6)
		case "below_target":
			addScore("dcf", 6)
			addScore("porter_moat", 5)
		}

		symbolType := detectSymbolType(contextData.Symbol, contextData.Currency, contextData.AssetType)
		switch symbolType {
		case "etf", "fund":
			addScore("capital_cycle", 8)
			addScore("industry_s_curve", 5)
			addScore("relative_valuation", 5)
		default:
			addScore("dupont_roic", 3)
		}
	}

	traitParts := []string{enrichedContext}
	if contextData != nil {
		traitParts = append(traitParts, contextData.Symbol, contextData.Name, contextData.AssetType)
	}
	traitsText := strings.ToLower(strings.Join(traitParts, " "))

	if containsAnyKeyword(traitsText, []string{"bank", "银行", "insurance", "保险", "broker", "券商", "financial"}) {
		addScore("dupont_roic", 10)
		addScore("relative_valuation", 6)
	}
	if containsAnyKeyword(traitsText, []string{"周期", "commodity", "shipping", "steel", "煤", "油", "化工", "有色"}) {
		addScore("capital_cycle", 10)
		addScore("industry_s_curve", 5)
	}
	if containsAnyKeyword(traitsText, []string{"高增长", "growth", "ai", "云", "新能源", "biotech", "半导体", "渗透率"}) {
		addScore("industry_s_curve", 9)
		addScore("expectations_investing", 8)
		addScore("reverse_dcf", 4)
	}
	if containsAnyKeyword(traitsText, []string{"护城河", "brand", "平台", "network", "专利", "粘性"}) {
		addScore("dynamic_moat", 8)
		addScore("porter_moat", 8)
	}

	type scoredFramework struct {
		Spec  symbolFrameworkSpec
		Score int
		Index int
	}

	scored := make([]scoredFramework, 0, len(symbolFrameworkCatalog))
	for idx, spec := range symbolFrameworkCatalog {
		scored = append(scored, scoredFramework{
			Spec:  spec,
			Score: scores[spec.ID],
			Index: idx,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Index < scored[j].Index
		}
		return scored[i].Score > scored[j].Score
	})

	selected := make([]symbolFrameworkSpec, 0, minFrameworkAnalyses)
	for _, item := range scored {
		selected = append(selected, item.Spec)
		if len(selected) == minFrameworkAnalyses {
			break
		}
	}
	return selected
}

func frameworkIDsFromSpecs(specs []symbolFrameworkSpec) []string {
	ids := make([]string, 0, len(specs))
	for _, spec := range specs {
		ids = append(ids, spec.ID)
	}
	return ids
}

func buildSynthesisWeightContext(contextData *symbolContextData, preference symbolPreferenceContext) symbolSynthesisWeightContext {
	weight := symbolSynthesisWeightContext{
		AllocationMaxPercent: 100,
		AllocationStatus:     "unknown",
		RiskProfile:          preference.RiskProfile,
		Horizon:              preference.Horizon,
		AdviceStyle:          preference.AdviceStyle,
		StrategyPrompt:       preference.StrategyPrompt,
	}
	if contextData == nil {
		return weight
	}

	weight.HoldingsQuantity = round2(contextData.TotalShares)
	weight.PositionPercent = round2(contextData.PositionPercent)
	weight.AllocationMinPercent = round2(contextData.AllocationMinPercent)
	weight.AllocationMaxPercent = round2(contextData.AllocationMaxPercent)
	weight.AllocationStatus = contextData.AllocationStatus
	weight.AssetType = contextData.AssetType

	if weight.AllocationMinPercent == 0 && weight.AllocationMaxPercent == 0 {
		weight.AllocationMaxPercent = 100
	}
	if strings.TrimSpace(weight.AllocationStatus) == "" {
		weight.AllocationStatus = "unknown"
	}
	return weight
}
