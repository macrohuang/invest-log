package investlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
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

// saveHoldingsAnalysis persists a completed holdings analysis to the database.
func (c *Core) saveHoldingsAnalysis(result *HoldingsAnalysisResult) (int64, error) {
	findingsJSON, err := json.Marshal(result.KeyFindings)
	if err != nil {
		return 0, fmt.Errorf("marshal key_findings: %w", err)
	}
	recsJSON, err := json.Marshal(result.Recommendations)
	if err != nil {
		return 0, fmt.Errorf("marshal recommendations: %w", err)
	}
	var refsJSON []byte
	if len(result.SymbolRefs) > 0 {
		refsJSON, err = json.Marshal(result.SymbolRefs)
		if err != nil {
			return 0, fmt.Errorf("marshal symbol_refs: %w", err)
		}
	}

	res, err := c.db.Exec(
		`INSERT INTO holdings_analyses
			(currency, model, analysis_type, risk_level, overall_summary, key_findings, recommendations, disclaimer, symbol_refs)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		result.Currency,
		result.Model,
		result.AnalysisType,
		result.RiskLevel,
		result.OverallSummary,
		string(findingsJSON),
		string(recsJSON),
		result.Disclaimer,
		nullableString(string(refsJSON)),
	)
	if err != nil {
		return 0, fmt.Errorf("insert holdings_analysis: %w", err)
	}
	return res.LastInsertId()
}

func nullableString(s string) any {
	if s == "" || s == "null" {
		return nil
	}
	return s
}

// GetHoldingsAnalysis returns the latest saved analysis for the given currency.
func (c *Core) GetHoldingsAnalysis(currency string) (*HoldingsAnalysisResult, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	results, err := c.GetHoldingsAnalysisHistory(currency, 1)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

// GetHoldingsAnalysisHistory returns up to limit recent analyses for the given currency.
func (c *Core) GetHoldingsAnalysisHistory(currency string, limit int) ([]HoldingsAnalysisResult, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if limit <= 0 {
		limit = 10
	}

	var (
		query string
		args  []any
	)
	if currency != "" {
		query = `SELECT id, currency, model, analysis_type, risk_level, overall_summary, key_findings, recommendations, disclaimer, symbol_refs, created_at
		          FROM holdings_analyses WHERE currency = ? ORDER BY created_at DESC LIMIT ?`
		args = []any{currency, limit}
	} else {
		query = `SELECT id, currency, model, analysis_type, risk_level, overall_summary, key_findings, recommendations, disclaimer, symbol_refs, created_at
		          FROM holdings_analyses ORDER BY created_at DESC LIMIT ?`
		args = []any{limit}
	}

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query holdings_analyses: %w", err)
	}
	defer rows.Close()

	var results []HoldingsAnalysisResult
	for rows.Next() {
		var (
			id                        int64
			curr, model, analysisType string
			riskLevel, overallSummary sql.NullString
			keyFindingsRaw, recsRaw   sql.NullString
			disclaimer, symbolRefsRaw sql.NullString
			createdAt                 string
		)
		if err := rows.Scan(&id, &curr, &model, &analysisType, &riskLevel, &overallSummary,
			&keyFindingsRaw, &recsRaw, &disclaimer, &symbolRefsRaw, &createdAt); err != nil {
			return nil, fmt.Errorf("scan holdings_analysis row: %w", err)
		}

		result := HoldingsAnalysisResult{
			ID:             id,
			GeneratedAt:    createdAt,
			Model:          model,
			Currency:       curr,
			AnalysisType:   analysisType,
			RiskLevel:      riskLevel.String,
			OverallSummary: overallSummary.String,
			Disclaimer:     disclaimer.String,
		}

		if keyFindingsRaw.Valid && keyFindingsRaw.String != "" {
			var findings []string
			if err := json.Unmarshal([]byte(keyFindingsRaw.String), &findings); err == nil {
				result.KeyFindings = findings
			}
		}
		if result.KeyFindings == nil {
			result.KeyFindings = []string{}
		}

		if recsRaw.Valid && recsRaw.String != "" {
			var recs []HoldingsAnalysisRecommendation
			if err := json.Unmarshal([]byte(recsRaw.String), &recs); err == nil {
				result.Recommendations = recs
			}
		}
		if result.Recommendations == nil {
			result.Recommendations = []HoldingsAnalysisRecommendation{}
		}

		if symbolRefsRaw.Valid && symbolRefsRaw.String != "" {
			var refs []HoldingsSymbolRef
			if err := json.Unmarshal([]byte(symbolRefsRaw.String), &refs); err == nil {
				result.SymbolRefs = refs
			}
		}

		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []HoldingsAnalysisResult{}
	}
	return results, nil
}

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

func normalizeEnum(raw, fallback string, allowed map[string]struct{}) (string, error) {
	if raw == "" {
		return fallback, nil
	}
	normalized := strings.ToLower(raw)
	if _, ok := allowed[normalized]; !ok {
		return "", fmt.Errorf("unsupported value: %s", raw)
	}
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
