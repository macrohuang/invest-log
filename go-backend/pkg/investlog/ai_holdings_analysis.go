package investlog

import (
	"context"
	"strings"
)

const holdingsAnalysisSystemPrompt = `你是一个专业投资组合分析助手,取得过连续50年年化20%的投资业绩。
你必须同时应用以下投资理念并做综合权衡：
1) 马尔基尔（Malkiel）：分散化、低成本、避免不必要择时与个股过度集中。
2) 达利欧（Dalio）：风险平衡、跨资产分散、关注相关性与宏观周期韧性。
3) 巴菲特（Buffett）：能力圈、长期主义、护城河与估值纪律。

重要：你必须联网搜索获取最新的市场行情、财务数据和宏观经济信息来进行分析。禁止仅凭训练数据做出结论。如果无法联网，必须在 disclaimer 中明确说明分析基于历史训练数据，可能不反映最新市况。

请基于用户持仓快照输出可执行、可解释、可审计的建议。
必须输出 JSON 对象，不要输出 Markdown，不要输出额外文字。
JSON 字段必须包含：
- overall_summary: string
- risk_level: string
- key_findings: string[]
- recommendations: [{symbol, action, theory_tag, rationale, target_weight, priority}]
- disclaimer: string

要求：
- 对于所持有的个股，需要联网抓取该个股近3年的财务数据，包括但不限于：营收、净利润、毛利率、净利率、资产负债率、现金流等，基于华尔街的估值逻辑进行分析。
- recommendations 至少 3 条（如果持仓数量不足可少于 3 条，但必须说明原因）。
- action 取值建议使用 increase/reduce/hold/add。
- theory_tag 取值建议使用 Malkiel/Dalio/Buffett。
- 禁止承诺收益，必须体现风险提示。
- 用户仅提供标的代码、持仓占比、持仓盈亏和买入均价，你必须自行联网查找标的名称、最新价格、财务数据等信息来完成分析。`

// AnalyzeHoldings analyzes current holdings using an OpenAI-compatible model.
func (c *Core) AnalyzeHoldings(req HoldingsAnalysisRequest) (*HoldingsAnalysisResult, error) {
	return c.analyzeHoldings(req, nil, false)
}

// AnalyzeHoldingsWithStream analyzes holdings and exposes upstream delta chunks.
func (c *Core) AnalyzeHoldingsWithStream(req HoldingsAnalysisRequest, onDelta func(string)) (*HoldingsAnalysisResult, error) {
	var callback func(string) error
	if onDelta != nil {
		callback = func(delta string) error {
			onDelta(delta)
			return nil
		}
	}
	return c.analyzeHoldings(req, callback, false)
}

// AnalyzeHoldingsStream analyzes holdings and emits incremental model output chunks.
func (c *Core) AnalyzeHoldingsStream(req HoldingsAnalysisRequest, onDelta func(string) error) (*HoldingsAnalysisResult, error) {
	if onDelta == nil {
		onDelta = func(string) error { return nil }
	}
	return c.analyzeHoldings(req, onDelta, true)
}

func (c *Core) analyzeHoldings(req HoldingsAnalysisRequest, onDelta func(string) error, streamMode bool) (*HoldingsAnalysisResult, error) {
	normalizedReq, err := normalizeHoldingsAnalysisRequest(req)
	if err != nil {
		return nil, err
	}

	promptInput, err := c.buildHoldingsAnalysisPromptInput(normalizedReq.Currency)
	if err != nil {
		return nil, err
	}

	// Collect available symbol-level AI analysis for context.
	symbolRefs := c.fetchSymbolAnalysisRefs(promptInput.Holdings)

	userPrompt, err := buildHoldingsAnalysisUserPrompt(promptInput, normalizedReq, symbolRefs)
	if err != nil {
		return nil, err
	}

	endpointURL, err := buildAICompletionsEndpoint(normalizedReq.BaseURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), aiTotalRequestTimeout)
	defer cancel()

	chatReq := aiChatCompletionRequest{
		EndpointURL:  endpointURL,
		APIKey:       normalizedReq.APIKey,
		Model:        normalizedReq.Model,
		SystemPrompt: holdingsAnalysisSystemPrompt,
		UserPrompt:   userPrompt,
		Logger:       c.Logger(),
	}
	if !streamMode && onDelta != nil {
		chatReq.OnDelta = func(delta string) {
			_ = onDelta(delta)
		}
	}

	var chatResult aiChatCompletionResult
	if streamMode {
		chatResult, err = aiChatCompletionStream(ctx, chatReq, onDelta)
	} else {
		chatResult, err = aiChatCompletion(ctx, chatReq)
	}
	if err != nil {
		return nil, err
	}

	parsed, err := parseHoldingsAnalysisResponse(chatResult.Content)
	if err != nil {
		return nil, err
	}

	model := strings.TrimSpace(chatResult.Model)
	if model == "" {
		model = normalizedReq.Model
	}

	riskLevel := strings.TrimSpace(parsed.RiskLevel)
	if riskLevel == "" {
		riskLevel = "unknown"
	}
	overallSummary := strings.TrimSpace(parsed.OverallSummary)
	if overallSummary == "" {
		overallSummary = "模型未返回总结，请重试或更换模型。"
	}
	disclaimer := strings.TrimSpace(parsed.Disclaimer)
	if disclaimer == "" {
		disclaimer = "本分析仅供参考，不构成投资建议。"
	}

	result := &HoldingsAnalysisResult{
		GeneratedAt:     NowRFC3339InShanghai(),
		Model:           model,
		Currency:        normalizedReq.Currency,
		AnalysisType:    normalizedReq.AnalysisType,
		OverallSummary:  overallSummary,
		RiskLevel:       riskLevel,
		KeyFindings:     normalizeFindings(parsed.KeyFindings),
		Recommendations: normalizeRecommendations(parsed.Recommendations),
		Disclaimer:      disclaimer,
		SymbolRefs:      symbolRefs,
	}

	if id, err := c.saveHoldingsAnalysis(result); err != nil {
		c.Logger().Warn("failed to save holdings analysis", "err", err)
	} else {
		result.ID = id
	}

	return result, nil
}

// fetchSymbolAnalysisRefs collects the latest completed symbol analysis summary for each holding.
func (c *Core) fetchSymbolAnalysisRefs(holdings []holdingsAnalysisCurrencySnapshot) []HoldingsSymbolRef {
	var refs []HoldingsSymbolRef
	seen := make(map[string]bool)
	for _, snap := range holdings {
		for _, item := range snap.Symbols {
			key := item.Symbol + ":" + snap.Currency
			if seen[key] {
				continue
			}
			seen[key] = true
			analysis, err := c.GetSymbolAnalysis(item.Symbol, snap.Currency)
			if err != nil || analysis == nil || analysis.Synthesis == nil {
				continue
			}
			summary := analysis.Synthesis.OverallSummary
			if len(summary) > 250 {
				summary = summary[:250] + "…"
			}
			refs = append(refs, HoldingsSymbolRef{
				Symbol:    item.Symbol,
				ID:        analysis.ID,
				Rating:    analysis.Synthesis.OverallRating,
				Action:    analysis.Synthesis.TargetAction,
				Summary:   summary,
				CreatedAt: analysis.CreatedAt,
			})
		}
	}
	return refs
}

func normalizeFindings(findings []string) []string {
	result := make([]string, 0, len(findings))
	for _, item := range findings {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func normalizeRecommendations(items []HoldingsAnalysisRecommendation) []HoldingsAnalysisRecommendation {
	result := make([]HoldingsAnalysisRecommendation, 0, len(items))
	for _, item := range items {
		action := strings.TrimSpace(strings.ToLower(item.Action))
		if action == "" {
			action = "hold"
		}
		theory := strings.TrimSpace(item.TheoryTag)
		if theory == "" {
			theory = "Malkiel"
		}
		rationale := strings.TrimSpace(item.Rationale)
		if rationale == "" {
			rationale = "模型未提供理由。"
		}
		result = append(result, HoldingsAnalysisRecommendation{
			Symbol:       strings.TrimSpace(item.Symbol),
			Action:       action,
			TheoryTag:    theory,
			Rationale:    rationale,
			TargetWeight: strings.TrimSpace(item.TargetWeight),
			Priority:     strings.TrimSpace(item.Priority),
		})
	}
	return result
}
