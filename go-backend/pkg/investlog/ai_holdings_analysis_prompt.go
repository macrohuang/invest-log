package investlog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func normalizeHoldingsAnalysisRequest(req HoldingsAnalysisRequest) (HoldingsAnalysisRequest, error) {
	normalized := req
	normalized.APIKey = strings.TrimSpace(req.APIKey)
	if normalized.APIKey == "" {
		return HoldingsAnalysisRequest{}, fmt.Errorf("api_key is required")
	}
	normalized.Model = strings.TrimSpace(req.Model)
	if normalized.Model == "" {
		return HoldingsAnalysisRequest{}, fmt.Errorf("model is required")
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency != "" && !contains(Currencies, currency) {
		return HoldingsAnalysisRequest{}, fmt.Errorf("invalid currency: %s", req.Currency)
	}
	normalized.Currency = currency

	riskProfile, err := normalizeEnum(strings.TrimSpace(req.RiskProfile), "balanced", map[string]struct{}{
		"conservative": {},
		"balanced":     {},
		"aggressive":   {},
	})
	if err != nil {
		return HoldingsAnalysisRequest{}, fmt.Errorf("invalid risk_profile: %w", err)
	}
	normalized.RiskProfile = riskProfile

	horizon, err := normalizeEnum(strings.TrimSpace(req.Horizon), "medium", map[string]struct{}{
		"short":  {},
		"medium": {},
		"long":   {},
	})
	if err != nil {
		return HoldingsAnalysisRequest{}, fmt.Errorf("invalid horizon: %w", err)
	}
	normalized.Horizon = horizon

	adviceStyle, err := normalizeEnum(strings.TrimSpace(req.AdviceStyle), "balanced", map[string]struct{}{
		"conservative": {},
		"balanced":     {},
		"aggressive":   {},
	})
	if err != nil {
		return HoldingsAnalysisRequest{}, fmt.Errorf("invalid advice_style: %w", err)
	}
	normalized.AdviceStyle = adviceStyle
	normalized.StrategyPrompt = strings.TrimSpace(req.StrategyPrompt)

	analysisType, err := normalizeEnum(strings.TrimSpace(req.AnalysisType), "adhoc", map[string]struct{}{
		"adhoc":   {},
		"weekly":  {},
		"monthly": {},
	})
	if err != nil {
		return HoldingsAnalysisRequest{}, fmt.Errorf("invalid analysis_type: %w", err)
	}
	normalized.AnalysisType = analysisType

	return normalized, nil
}

func (c *Core) buildHoldingsAnalysisPromptInput(currency string) (*holdingsAnalysisPromptInput, error) {
	bySymbol, err := c.GetHoldingsBySymbol()
	if err != nil {
		return nil, fmt.Errorf("load holdings by symbol: %w", err)
	}
	if len(bySymbol) == 0 {
		return nil, fmt.Errorf("no holdings found")
	}

	currencies := make([]string, 0, len(bySymbol))
	for curr := range bySymbol {
		if currency != "" && curr != currency {
			continue
		}
		currencies = append(currencies, curr)
	}
	sort.Strings(currencies)
	if len(currencies) == 0 {
		return nil, fmt.Errorf("no holdings found for currency: %s", currency)
	}

	holdings := make([]holdingsAnalysisCurrencySnapshot, 0, len(currencies))
	for _, curr := range currencies {
		currData := bySymbol[curr]
		symbols := make([]holdingsAnalysisSymbolItem, 0, len(currData.Symbols))
		for _, item := range currData.Symbols {
			symbols = append(symbols, holdingsAnalysisSymbolItem{
				Symbol:    item.Symbol,
				WeightPct: item.Percent,
				PnLPct:    item.PnlPercent,
				AvgCost:   item.AvgCost.InexactFloat64(),
			})
		}

		holdings = append(holdings, holdingsAnalysisCurrencySnapshot{
			Currency: curr,
			Symbols:  symbols,
		})
	}

	return &holdingsAnalysisPromptInput{Holdings: holdings}, nil
}

func buildHoldingsAnalysisUserPrompt(input *holdingsAnalysisPromptInput, req HoldingsAnalysisRequest, symbolRefs []HoldingsSymbolRef) (string, error) {
	promptInput := holdingsAnalysisPromptInput{
		RiskProfile:     req.RiskProfile,
		Horizon:         req.Horizon,
		AdviceStyle:     req.AdviceStyle,
		AllowNewSymbols: req.AllowNewSymbols,
		StrategyPrompt:  req.StrategyPrompt,
		Holdings:        input.Holdings,
	}
	payload, err := json.Marshal(promptInput)
	if err != nil {
		return "", fmt.Errorf("marshal holdings prompt input: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("请基于以下输入完成分析并给出建议：\n")
	sb.WriteString(string(payload))
	sb.WriteString("\n\n输出要求：\n")
	sb.WriteString("1) 必须是 JSON 对象。\n")
	sb.WriteString("2) recommendations 中建议尽量覆盖：仓位集中风险、资产分散、回撤防御、长期价值。\n")
	sb.WriteString("3) 允许新增标的时，可给出 add 建议并点名标的。\n")
	sb.WriteString("4) 每条建议必须给出 theory_tag 和 rationale。\n")
	sb.WriteString("5) 若 strategy_prompt 非空，需优先吸收为策略偏好，但不得违反风险提示原则。")

	// Append analysis-type-specific focus instructions.
	switch req.AnalysisType {
	case "weekly":
		sb.WriteString("\n\n分析类型：周报（Weekly）\n重点关注：\n- 近1-2周的价格催化剂和市场动量\n- 短期技术形态与成交量变化\n- 近期新闻事件对持仓的潜在影响\n- 适合本周可能的调仓时机建议")
	case "monthly":
		sb.WriteString("\n\n分析类型：月报（Monthly）\n重点关注：\n- 近1-3个月的组合再平衡需求\n- 基本面变化与行业轮动机会\n- 宏观经济指标对整体配置的影响\n- 月度仓位优化和风险调整建议")
	}

	// Append symbol-level analysis summaries as reference context.
	if len(symbolRefs) > 0 {
		refsJSON, err := json.Marshal(symbolRefs)
		if err == nil {
			sb.WriteString("\n\n以下是各标的的最新深度AI分析摘要（供参考，可直接引用结论）：\n")
			sb.Write(refsJSON)
		}
	}

	return sb.String(), nil
}

func parseHoldingsAnalysisResponse(content string) (*holdingsAnalysisModelResponse, error) {
	cleaned := cleanupModelJSON(content)
	var parsed holdingsAnalysisModelResponse
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w", err)
	}
	return &parsed, nil
}
