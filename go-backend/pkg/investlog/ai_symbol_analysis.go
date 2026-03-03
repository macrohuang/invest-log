package investlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	symbolAnalysisTimeout       = aiTotalRequestTimeout
	minFrameworkAnalyses        = 3
	maxSynthesisDisclaimerChars = 16
)

type symbolFrameworkSpec struct {
	ID    string
	Name  string
	Focus string
}

var symbolFrameworkCatalog = []symbolFrameworkSpec{
	{ID: "dupont_roic", Name: "杜邦ROIC", Focus: "拆解ROE/ROIC驱动，检验资本效率与杠杆质量"},
	{ID: "capital_cycle", Name: "资本周期", Focus: "识别供给扩张/出清节奏，判断利润中枢拐点"},
	{ID: "industry_s_curve", Name: "产业S曲线", Focus: "定位渗透率阶段、增长斜率与天花板"},
	{ID: "reverse_dcf", Name: "反向DCF", Focus: "反推出市场隐含增长与利润假设并检验可达性"},
	{ID: "dynamic_moat", Name: "动态护城河", Focus: "评估护城河强化或弱化趋势及触发因素"},
	{ID: "dcf", Name: "DCF估值", Focus: "以现金流折现估算内在价值与安全边际"},
	{ID: "porter_moat", Name: "波特护城河", Focus: "从五力与竞争位置评估超额收益可持续性"},
	{ID: "expectations_investing", Name: "预期差投资", Focus: "比较市场一致预期与现实偏差的方向与幅度"},
	{ID: "relative_valuation", Name: "相对估值", Focus: "与历史区间及可比公司估值横向对照"},
}

var legacyDimensionColumnOrder = []string{"macro", "industry", "company", "international"}

const frameworkAnalysisSystemPromptTemplate = `你是一个只使用单一分析框架的投资研究助手。
本次必须使用框架ID: %s
框架名称: %s
框架重点: %s

必须输出 JSON 对象，不要输出 Markdown，不要输出额外文字。
JSON 字段必须包含：
- dimension: "%s"
- rating: string (positive/neutral/negative)
- confidence: string (high/medium/low)
- key_points: string[]
- risks: string[]
- opportunities: string[]
- summary: string
- suggestion: string (必须给出明确动作建议，且包含 increase/hold/reduce 之一)
- valuation_assessment: string (可选)

要求：
- 只按该框架推理，不要混用其他框架
- 结论必须明确，禁止“看情况/视情况/it depends”
- 用短句输出，信息密度高`

const symbolSynthesisSystemPrompt = `你是一个综合投资分析师，负责整合三个已选框架的结果给出最终投资建议。
你将收到：1) 标的信息，2) 三个框架分析结果，3) 权重上下文。
权重上下文包含持仓数量、仓位占比、资产类别配置区间、用户偏好与策略（含 StrategyPrompt）。
你必须显式基于这三框架做综合判断，不得引用未给出的框架。

必须输出 JSON 对象，不要输出 Markdown，不要输出额外文字。
JSON 字段必须包含：
- overall_rating: string (strong_buy/buy/hold/reduce/strong_sell)
- confidence: string (high/medium/low)
- action_probability_percent: number (0-100，表示“执行当前 target_action 后在目标持有周期内跑赢无调整方案”的主观概率)
- target_action: string (increase/hold/reduce)
- position_suggestion: string (对仓位的具体建议)
- overall_summary: string (综合分析总结，200字以内)
- key_factors: string[] (影响评级的关键因素，3-5条)
- risk_warnings: string[] (主要风险提示，2-4条)
- action_items: [{action: string, rationale: string, priority: string(high/medium/low)}] (具体行动建议，2-4条)
- time_horizon_notes: string (投资时间维度的建议)
- disclaimer: string (风险免责声明)

要求：
- 综合评级必须有充分的逻辑依据，并显式说明三框架如何加权
- 如果框架结论冲突，需要明确说明权衡逻辑
- 行动建议必须具体可执行
- 必须把“持仓数量 + 仓位占比 + 资产类别配置区间 + 用户偏好与策略”纳入权重计算
- 禁止输出“看情况/视情况而定/it depends”等含混表达
- 必须给出明确概率，不得只给定性判断
- 语言要直接、锋利、去客套、去公关腔，句子短，信息密度高
- 禁止起手寒暄；第一句直接给结论
- 禁止冗长免责声明，disclaimer 字段只允许简短风险锚点（<=16字）
- 禁止承诺收益，必须体现风险提示`

// SymbolAnalysisRequest defines inputs for per-symbol AI deep analysis.
type SymbolAnalysisRequest struct {
	BaseURL        string
	APIKey         string
	Model          string
	Symbol         string
	Currency       string
	RiskProfile    string
	Horizon        string
	AdviceStyle    string
	StrategyPrompt string
}

// SymbolDimensionResult is one dimension's analysis output.
type SymbolDimensionResult struct {
	Dimension           string   `json:"dimension"`
	Rating              string   `json:"rating"`
	Confidence          string   `json:"confidence"`
	KeyPoints           []string `json:"key_points"`
	Risks               []string `json:"risks"`
	Opportunities       []string `json:"opportunities"`
	Summary             string   `json:"summary"`
	Suggestion          string   `json:"suggestion,omitempty"`
	ValuationAssessment string   `json:"valuation_assessment,omitempty"`
}

// SymbolAnalysisActionItem is one action item in the synthesis.
type SymbolAnalysisActionItem struct {
	Action    string `json:"action"`
	Rationale string `json:"rationale"`
	Priority  string `json:"priority"`
}

// SymbolSynthesisResult is the synthesis agent's output.
type SymbolSynthesisResult struct {
	OverallRating      string                     `json:"overall_rating"`
	Confidence         string                     `json:"confidence"`
	ActionProbability  float64                    `json:"action_probability_percent,omitempty"`
	TargetAction       string                     `json:"target_action"`
	PositionSuggestion string                     `json:"position_suggestion"`
	OverallSummary     string                     `json:"overall_summary"`
	KeyFactors         []string                   `json:"key_factors"`
	RiskWarnings       []string                   `json:"risk_warnings"`
	ActionItems        []SymbolAnalysisActionItem `json:"action_items"`
	TimeHorizonNotes   string                     `json:"time_horizon_notes"`
	Disclaimer         string                     `json:"disclaimer"`
}

// SymbolAnalysisResult is the full result returned to clients.
type SymbolAnalysisResult struct {
	ID           int64                             `json:"id"`
	Symbol       string                            `json:"symbol"`
	Currency     string                            `json:"currency"`
	Model        string                            `json:"model"`
	Status       string                            `json:"status"`
	Dimensions   map[string]*SymbolDimensionResult `json:"dimensions"`
	Synthesis    *SymbolSynthesisResult            `json:"synthesis"`
	ErrorMessage string                            `json:"error_message,omitempty"`
	CreatedAt    string                            `json:"created_at"`
	CompletedAt  string                            `json:"completed_at,omitempty"`
}

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

type symbolContextData struct {
	Symbol                   string   `json:"symbol"`
	Name                     string   `json:"name,omitempty"`
	Currency                 string   `json:"currency"`
	AssetType                string   `json:"asset_type,omitempty"`
	TotalShares              float64  `json:"total_shares,omitempty"`
	AvgCost                  float64  `json:"avg_cost,omitempty"`
	CostBasis                float64  `json:"cost_basis,omitempty"`
	LatestPrice              float64  `json:"latest_price,omitempty"`
	MarketValue              float64  `json:"market_value,omitempty"`
	PnLPercent               float64  `json:"pnl_percent,omitempty"`
	PositionPercent          float64  `json:"position_percent,omitempty"`
	CurrencyTotalMarketValue float64  `json:"currency_total_market_value,omitempty"`
	AccountName              string   `json:"account_name,omitempty"`
	AccountNames             []string `json:"account_names,omitempty"`
	AllocationMinPercent     float64  `json:"allocation_min_percent,omitempty"`
	AllocationMaxPercent     float64  `json:"allocation_max_percent,omitempty"`
	AllocationStatus         string   `json:"allocation_status,omitempty"`
}

type symbolPreferenceContext struct {
	RiskProfile    string `json:"risk_profile"`
	Horizon        string `json:"horizon"`
	AdviceStyle    string `json:"advice_style"`
	StrategyPrompt string `json:"strategy_prompt,omitempty"`
}

type symbolSynthesisWeightContext struct {
	HoldingsQuantity     float64 `json:"holdings_quantity"`
	PositionPercent      float64 `json:"position_percent"`
	AllocationMinPercent float64 `json:"allocation_min_percent"`
	AllocationMaxPercent float64 `json:"allocation_max_percent"`
	AllocationStatus     string  `json:"allocation_status"`
	AssetType            string  `json:"asset_type,omitempty"`
	RiskProfile          string  `json:"risk_profile"`
	Horizon              string  `json:"horizon"`
	AdviceStyle          string  `json:"advice_style"`
	StrategyPrompt       string  `json:"strategy_prompt,omitempty"`
}

// aiJSON returns a JSON string containing only the fields allowed for AI consumption:
// symbol, name, avg_cost, pnl_percent, position_percent, allocation_max_percent, allocation_status.
func (ctx *symbolContextData) aiJSON() (string, error) {
	slim := struct {
		Symbol               string  `json:"symbol"`
		Name                 string  `json:"name,omitempty"`
		AvgCost              float64 `json:"avg_cost,omitempty"`
		PnLPercent           float64 `json:"pnl_percent,omitempty"`
		PositionPercent      float64 `json:"position_percent,omitempty"`
		AllocationMaxPercent float64 `json:"allocation_max_percent,omitempty"`
		AllocationStatus     string  `json:"allocation_status,omitempty"`
	}{
		Symbol:               ctx.Symbol,
		Name:                 ctx.Name,
		AvgCost:              ctx.AvgCost,
		PnLPercent:           ctx.PnLPercent,
		PositionPercent:      ctx.PositionPercent,
		AllocationMaxPercent: ctx.AllocationMaxPercent,
		AllocationStatus:     ctx.AllocationStatus,
	}
	data, err := json.Marshal(slim)
	if err != nil {
		return "", fmt.Errorf("marshal symbol AI context: %w", err)
	}
	return string(data), nil
}

func (c *Core) buildSymbolContext(symbol, currency string) (*symbolContextData, error) {
	bySymbol, err := c.GetHoldingsBySymbol()
	if err != nil {
		return nil, fmt.Errorf("load holdings: %w", err)
	}

	currData, ok := bySymbol[currency]
	if !ok {
		return nil, fmt.Errorf("no holdings found for currency: %s", currency)
	}

	matched := make([]SymbolHolding, 0)
	for _, s := range currData.Symbols {
		if strings.EqualFold(s.Symbol, symbol) {
			matched = append(matched, s)
		}
	}

	if len(matched) == 0 {
		// Allow analysis even without holdings (just symbol + currency)
		return &symbolContextData{
			Symbol:   symbol,
			Currency: currency,
		}, nil
	}

	name := symbol
	assetType := ""
	var totalShares float64
	var totalCostBasis float64
	var totalMarketValue float64
	accountNameSet := map[string]struct{}{}

	for _, item := range matched {
		if name == symbol && item.Name != nil && strings.TrimSpace(*item.Name) != "" {
			name = strings.TrimSpace(*item.Name)
		}
		if assetType == "" {
			assetType = strings.TrimSpace(item.AssetType)
		}

		totalShares += item.TotalShares.InexactFloat64()
		totalCostBasis += item.CostBasis.InexactFloat64()
		totalMarketValue += item.MarketValue.InexactFloat64()

		accountName := strings.TrimSpace(item.AccountName)
		if accountName == "" {
			accountName = strings.TrimSpace(item.AccountID)
		}
		if accountName != "" {
			accountNameSet[accountName] = struct{}{}
		}
	}

	if assetType == "" {
		assetType = "stock"
	}

	accountNames := make([]string, 0, len(accountNameSet))
	for accountName := range accountNameSet {
		accountNames = append(accountNames, accountName)
	}
	sort.Strings(accountNames)

	avgCost := 0.0
	latestPrice := 0.0
	if totalShares > 0 {
		avgCost = round2(totalCostBasis / totalShares)
		latestPrice = round2(totalMarketValue / totalShares)
	}
	pnlPercent := 0.0
	if totalCostBasis > 0 {
		pnlPercent = round2((totalMarketValue - totalCostBasis) / totalCostBasis * 100)
	}
	positionPercent := 0.0
	if currData.TotalMarketValue.IsPositive() {
		positionPercent = round2(totalMarketValue / currData.TotalMarketValue.InexactFloat64() * 100)
	}

	ctx := &symbolContextData{
		Symbol:                   symbol,
		Name:                     name,
		Currency:                 currency,
		AssetType:                assetType,
		TotalShares:              round2(totalShares),
		AvgCost:                  avgCost,
		CostBasis:                round2(totalCostBasis),
		LatestPrice:              latestPrice,
		MarketValue:              round2(totalMarketValue),
		PnLPercent:               pnlPercent,
		PositionPercent:          positionPercent,
		CurrencyTotalMarketValue: round2(currData.TotalMarketValue.InexactFloat64()),
		AccountNames:             accountNames,
	}
	if len(accountNames) > 0 {
		ctx.AccountName = accountNames[0]
	}

	allocationSettings, err := c.GetAllocationSettings(currency)
	if err != nil {
		c.Logger().Warn("load allocation settings failed", "currency", currency, "err", err)
	} else {
		ctx.AllocationStatus = "no_target"
		for _, setting := range allocationSettings {
			if strings.EqualFold(setting.AssetType, assetType) {
				ctx.AllocationMinPercent = round2(setting.MinPercent)
				ctx.AllocationMaxPercent = round2(setting.MaxPercent)
				switch {
				case positionPercent < setting.MinPercent:
					ctx.AllocationStatus = "below_target"
				case positionPercent > setting.MaxPercent:
					ctx.AllocationStatus = "above_target"
				default:
					ctx.AllocationStatus = "within_target"
				}
				break
			}
		}
	}

	return ctx, nil
}

type frameworkAgent struct {
	FrameworkID  string
	SystemPrompt string
}

type agentResult struct {
	FrameworkID string
	Content     string
	Error       error
}

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

func (c *Core) runDimensionAgents(
	ctx context.Context,
	endpoint, apiKey, model string,
	frameworks []symbolFrameworkSpec,
	userPrompt string,
	onDelta func(string),
) (map[string]string, error) {
	if len(frameworks) < minFrameworkAnalyses {
		return nil, fmt.Errorf("selected frameworks less than %d", minFrameworkAnalyses)
	}

	agents := make([]frameworkAgent, 0, len(frameworks))
	for _, framework := range frameworks {
		agents = append(agents, frameworkAgent{
			FrameworkID:  framework.ID,
			SystemPrompt: buildFrameworkSystemPrompt(framework),
		})
	}

	ch := make(chan agentResult, len(agents))
	var wg sync.WaitGroup

	for _, a := range agents {
		wg.Add(1)
		go func(frameworkID, sysPrompt string) {
			defer wg.Done()
			res, err := aiChatCompletion(ctx, aiChatCompletionRequest{
				EndpointURL:  endpoint,
				APIKey:       apiKey,
				Model:        model,
				SystemPrompt: sysPrompt,
				UserPrompt:   userPrompt,
				Logger:       c.Logger(),
				OnDelta: func(delta string) {
					delta = strings.TrimSpace(delta)
					if delta == "" || onDelta == nil {
						return
					}
					onDelta("[" + frameworkID + "] " + delta)
				},
			})
			if err != nil {
				ch <- agentResult{FrameworkID: frameworkID, Error: err}
				return
			}
			ch <- agentResult{FrameworkID: frameworkID, Content: res.Content}
		}(a.FrameworkID, a.SystemPrompt)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	outputs := make(map[string]string, len(agents))
	var errs []string
	for r := range ch {
		if r.Error != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.FrameworkID, r.Error))
			continue
		}
		outputs[r.FrameworkID] = r.Content
	}

	if len(outputs) < minFrameworkAnalyses {
		return nil, fmt.Errorf("framework analyses insufficient (%d/%d): %s", len(outputs), len(agents), strings.Join(errs, "; "))
	}
	return outputs, nil
}

func runSynthesisAgent(
	ctx context.Context,
	endpoint, apiKey, model, symbolContext string,
	frameworkOutputs map[string]string,
	frameworkIDs []string,
	weightContext symbolSynthesisWeightContext,
	onDelta func(string),
) (string, error) {
	frameworkJSON, err := json.Marshal(frameworkOutputs)
	if err != nil {
		return "", fmt.Errorf("marshal framework outputs: %w", err)
	}
	frameworkIDsJSON, err := json.Marshal(frameworkIDs)
	if err != nil {
		return "", fmt.Errorf("marshal framework ids: %w", err)
	}
	weightJSON, err := json.Marshal(weightContext)
	if err != nil {
		return "", fmt.Errorf("marshal synthesis weight context: %w", err)
	}

	userPrompt := fmt.Sprintf(`请基于以下信息给出综合建议：

标的信息：
%s

三框架ID（必须逐一引用）：
%s

三框架分析结果：
%s

权重上下文（必须纳入计算）：
%s

硬约束：
1) overall_summary 的第一句必须直接给结论（target_action + action_probability_percent）。
2) action_probability_percent 必须是具体数值。
3) 必须明确给出当前仓位占比、目标配置区间、差值。
4) 禁止“看情况/视情况/it depends”。`, symbolContext, string(frameworkIDsJSON), string(frameworkJSON), string(weightJSON))

	result, err := aiChatCompletion(ctx, aiChatCompletionRequest{
		EndpointURL:  endpoint,
		APIKey:       apiKey,
		Model:        model,
		SystemPrompt: symbolSynthesisSystemPrompt,
		UserPrompt:   userPrompt,
		OnDelta: func(delta string) {
			delta = strings.TrimSpace(delta)
			if delta == "" || onDelta == nil {
				return
			}
			onDelta("[synthesis] " + delta)
		},
	})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

func parseSymbolDimensionResult(raw string) (*SymbolDimensionResult, error) {
	cleaned := cleanupModelJSON(raw)
	var result SymbolDimensionResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse dimension result: %w", err)
	}
	if strings.TrimSpace(result.Suggestion) == "" {
		var fallback struct {
			FrameworkID    string `json:"framework_id"`
			Suggestion     string `json:"suggestion"`
			Recommendation string `json:"recommendation"`
			Advice         string `json:"advice"`
		}
		if err := json.Unmarshal([]byte(cleaned), &fallback); err == nil {
			if strings.TrimSpace(result.Dimension) == "" {
				result.Dimension = strings.TrimSpace(fallback.FrameworkID)
			}
			result.Suggestion = firstNonEmptyString(fallback.Suggestion, fallback.Recommendation, fallback.Advice)
		}
	}
	return &result, nil
}

func parseSynthesisResult(raw string) (*SymbolSynthesisResult, error) {
	cleaned := cleanupModelJSON(raw)
	var result SymbolSynthesisResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("parse synthesis result: %w", err)
	}
	normalizeSynthesisResult(&result, nil)
	return &result, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeDimensionResult(result *SymbolDimensionResult, frameworkID string) {
	if result == nil {
		return
	}

	result.Dimension = strings.ToLower(strings.TrimSpace(result.Dimension))
	if strings.TrimSpace(frameworkID) != "" {
		result.Dimension = strings.ToLower(strings.TrimSpace(frameworkID))
	}
	result.Rating = strings.ToLower(strings.TrimSpace(result.Rating))
	if result.Rating == "" {
		result.Rating = "neutral"
	}
	result.Confidence = strings.ToLower(strings.TrimSpace(result.Confidence))
	if result.Confidence == "" {
		result.Confidence = "medium"
	}
	result.Summary = strings.TrimSpace(stripFuzzyPhrases(result.Summary))
	if result.Summary == "" {
		result.Summary = "数据不足，暂给中性判断。"
	}
	result.Suggestion = strings.TrimSpace(stripFuzzyPhrases(result.Suggestion))
	if result.Suggestion == "" {
		result.Suggestion = defaultDimensionSuggestion(result.Rating)
	}
}

func defaultDimensionSuggestion(rating string) string {
	switch strings.ToLower(strings.TrimSpace(rating)) {
	case "positive":
		return "increase：信号偏正，分批加仓。"
	case "negative":
		return "reduce：信号偏弱，先降仓。"
	default:
		return "hold：信号中性，维持仓位。"
	}
}

func normalizeSynthesisResult(result *SymbolSynthesisResult, context *symbolContextData, frameworkIDs ...[]string) {
	if result == nil {
		return
	}
	var selectedFrameworkIDs []string
	if len(frameworkIDs) > 0 {
		selectedFrameworkIDs = frameworkIDs[0]
	}
	result.TargetAction = normalizeSynthesisAction(result.TargetAction)
	result.ActionProbability = normalizeSynthesisProbability(result.Confidence, result.ActionProbability)
	result.PositionSuggestion = normalizeSynthesisPositionSuggestion(*result, context)
	result.OverallSummary = normalizeSynthesisSummary(*result, selectedFrameworkIDs)
	result.Disclaimer = normalizeSynthesisDisclaimer(result.Disclaimer)
}

func normalizeSynthesisAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "increase":
		return "increase"
	case "reduce":
		return "reduce"
	default:
		return "hold"
	}
}

func normalizeSynthesisProbability(confidence string, probability float64) float64 {
	if probability > 0 && probability <= 100 {
		return round2(probability)
	}

	switch strings.ToLower(strings.TrimSpace(confidence)) {
	case "high":
		return 72
	case "low":
		return 42
	default:
		return 58
	}
}

func normalizeSynthesisSummary(result SymbolSynthesisResult, frameworkIDs ...[]string) string {
	var selectedFrameworkIDs []string
	if len(frameworkIDs) > 0 {
		selectedFrameworkIDs = frameworkIDs[0]
	}
	actionLabel := mapSynthesisActionLabel(result.TargetAction)
	probability := normalizeSynthesisProbability(result.Confidence, result.ActionProbability)

	position := compactSynthesisText(result.PositionSuggestion)
	if position == "" {
		position = "维持当前仓位。"
	} else if !strings.HasSuffix(position, "。") {
		position += "。"
	}

	factors := buildSynthesisListLine("依据", result.KeyFactors)
	risks := buildSynthesisListLine("雷点", result.RiskWarnings)

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("%s，执行概率%.0f%%。", actionLabel, probability))
	if len(selectedFrameworkIDs) > 0 {
		builder.WriteString("框架：")
		builder.WriteString(strings.Join(selectedFrameworkIDs, "、"))
		builder.WriteString("。")
	}
	builder.WriteString("仓位：")
	builder.WriteString(position)
	builder.WriteString(factors)
	builder.WriteString(risks)

	summary := strings.TrimSpace(stripFuzzyPhrases(builder.String()))

	if len([]rune(summary)) > 200 {
		summary = string([]rune(summary)[:200])
		if !strings.HasSuffix(summary, "。") {
			summary += "。"
		}
	}

	if summary == "" {
		summary = fmt.Sprintf("%s，执行概率%.0f%%。", actionLabel, probability)
	}

	return summary
}

func normalizeSynthesisDisclaimer(disclaimer string) string {
	cleaned := strings.TrimSpace(stripFuzzyPhrases(disclaimer))
	cleaned = strings.NewReplacer("\n", "", "\r", "", " ", "", "。", "", "，", "").Replace(cleaned)
	if cleaned == "" {
		cleaned = "市场波动风险"
	}
	runes := []rune(cleaned)
	if len(runes) > maxSynthesisDisclaimerChars {
		cleaned = string(runes[:maxSynthesisDisclaimerChars])
	}
	if strings.TrimSpace(cleaned) == "" {
		return "市场波动风险"
	}
	return cleaned
}

func stripFuzzyPhrases(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"看情况", "",
		"视情况", "",
		"视情况而定", "",
		"it depends", "",
		"It depends", "",
		"IT DEPENDS", "",
	)
	return replacer.Replace(input)
}

func normalizeSynthesisPositionSuggestion(result SymbolSynthesisResult, context *symbolContextData) string {
	base := compactSynthesisText(result.PositionSuggestion)
	if context == nil {
		if base == "" {
			return "当前占比未知；目标区间未知；差值未知。"
		}
		if strings.Contains(base, "当前占比") && strings.Contains(base, "目标区间") && strings.Contains(base, "差值") {
			if strings.HasSuffix(base, "。") {
				return base
			}
			return base + "。"
		}
		return base
	}

	current := round2(context.PositionPercent)
	targetMin, targetMax := context.AllocationMinPercent, context.AllocationMaxPercent
	if targetMin == 0 && targetMax == 0 {
		targetMin = 0
		targetMax = 100
	}

	delta := 0.0
	status := "在区间内"
	switch {
	case current < targetMin:
		delta = round2(current - targetMin)
		status = "低于下限"
	case current > targetMax:
		delta = round2(current - targetMax)
		status = "高于上限"
	default:
		delta = 0
	}

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("当前占比%.2f%%；目标区间%.2f%%-%.2f%%；差值%s（%s）；动作：%s",
		current,
		targetMin,
		targetMax,
		formatSignedPercent(delta),
		status,
		mapSynthesisActionLabel(result.TargetAction),
	))

	if base != "" {
		builder.WriteString("；执行：")
		builder.WriteString(base)
	}

	position := builder.String()
	if !strings.HasSuffix(position, "。") {
		position += "。"
	}
	return position
}

func formatSignedPercent(value float64) string {
	if value > 0 {
		return fmt.Sprintf("+%.2f%%", value)
	}
	if value < 0 {
		return fmt.Sprintf("%.2f%%", value)
	}
	return "0.00%"
}

func buildSynthesisListLine(label string, items []string) string {
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		cleaned := compactSynthesisText(item)
		if cleaned != "" {
			normalized = append(normalized, cleaned)
		}
		if len(normalized) >= 2 {
			break
		}
	}
	if len(normalized) == 0 {
		return ""
	}
	return fmt.Sprintf("%s：%s。", label, strings.Join(normalized, "；"))
}

func compactSynthesisText(input string) string {
	trimmed := strings.TrimSpace(stripFuzzyPhrases(input))
	if trimmed == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"\n", " ",
		"\r", " ",
		"  ", " ",
		"。", "",
	)
	cleaned := replacer.Replace(trimmed)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return ""
	}
	if len([]rune(cleaned)) > 42 {
		cleaned = string([]rune(cleaned)[:42])
	}
	return cleaned
}

func mapSynthesisActionLabel(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "increase":
		return "加仓"
	case "reduce":
		return "减仓"
	case "hold":
		return "持有"
	default:
		return "持有"
	}
}

// buildDimensionUserPrompt constructs the user prompt for framework agents,
// optionally injecting enriched context from external data.
func buildDimensionUserPrompt(symbolContext, enrichedContext string, req SymbolAnalysisRequest, selectedFrameworkIDs []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请分析以下投资标的：\n%s\n", symbolContext))

	if len(selectedFrameworkIDs) > 0 {
		sb.WriteString(fmt.Sprintf("\n本次只允许分析以下框架ID：%s\n", strings.Join(selectedFrameworkIDs, ", ")))
	}

	if enrichedContext != "" {
		sb.WriteString(fmt.Sprintf(`
以下是该标的的最新实时数据和新闻摘要（数据截至今日）：
%s

重要：请优先使用上述实时数据进行分析，而非你的训练数据中的过时信息。
`, enrichedContext))
	}

	sb.WriteString("\n请根据你的框架职责进行分析，并输出指定格式的 JSON 结果。")

	if req.RiskProfile != "" || req.Horizon != "" || req.AdviceStyle != "" || req.StrategyPrompt != "" {
		preferenceJSON, err := json.Marshal(symbolPreferenceContext{
			RiskProfile:    req.RiskProfile,
			Horizon:        req.Horizon,
			AdviceStyle:    req.AdviceStyle,
			StrategyPrompt: req.StrategyPrompt,
		})
		if err == nil {
			sb.WriteString(fmt.Sprintf("\n\n用户投资偏好（用于结论权重与表达强度）：\n%s", string(preferenceJSON)))
		}
	}

	return sb.String()
}

// AnalyzeSymbol runs a multi-agent deep analysis for a single symbol.
func (c *Core) AnalyzeSymbol(req SymbolAnalysisRequest) (*SymbolAnalysisResult, error) {
	return c.analyzeSymbol(req, nil)
}

// AnalyzeSymbolWithStream runs symbol analysis with stream envelope events.
// It suppresses intermediate model token deltas to avoid noisy UI output.
func (c *Core) AnalyzeSymbolWithStream(req SymbolAnalysisRequest, onDelta func(string)) (*SymbolAnalysisResult, error) {
	_ = onDelta
	return c.analyzeSymbol(req, nil)
}

func (c *Core) analyzeSymbol(req SymbolAnalysisRequest, onDelta func(string)) (*SymbolAnalysisResult, error) {
	// Suppress intermediate token output for symbol analysis stream.
	onDelta = nil

	normalizedReq, err := normalizeSymbolAnalysisRequest(req)
	if err != nil {
		return nil, err
	}

	contextData, err := c.buildSymbolContext(normalizedReq.Symbol, normalizedReq.Currency)
	if err != nil {
		return nil, err
	}

	symbolContextJSON, err := contextData.aiJSON()
	if err != nil {
		return nil, err
	}

	endpointURL, err := buildAICompletionsEndpoint(normalizedReq.BaseURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), symbolAnalysisTimeout)
	defer cancel()

	// Insert pending row.
	rowID, err := c.insertPendingSymbolAnalysis(normalizedReq)
	if err != nil {
		return nil, fmt.Errorf("save pending analysis: %w", err)
	}

	// Fetch and summarize external data (graceful degradation on failure).
	var enrichedContext string
	externalData := fetchExternalDataFn(ctx, normalizedReq.Symbol, normalizedReq.Currency, c.Logger())
	if externalData != nil {
		summary := summarizeExternalDataFn(ctx, externalData, endpointURL, normalizedReq.APIKey, normalizedReq.Model, c.Logger())
		if summary != "" {
			enrichedContext = summary
			externalData.Summary = summary
		}
	}

	selectedFrameworks := selectSymbolFrameworks(contextData, enrichedContext)
	if len(selectedFrameworks) < minFrameworkAnalyses {
		err := fmt.Errorf("selected frameworks less than %d", minFrameworkAnalyses)
		_ = c.updateSymbolAnalysisStatus(rowID, "failed", err.Error())
		return nil, err
	}
	selectedFrameworkIDs := frameworkIDsFromSpecs(selectedFrameworks)

	// Build user prompt for framework agents.
	userPrompt := buildDimensionUserPrompt(symbolContextJSON, enrichedContext, normalizedReq, selectedFrameworkIDs)

	// Run 3 framework agents in parallel.
	dimensionOutputs, err := c.runDimensionAgents(
		ctx,
		endpointURL,
		normalizedReq.APIKey,
		normalizedReq.Model,
		selectedFrameworks,
		userPrompt,
		onDelta,
	)
	if err != nil {
		_ = c.updateSymbolAnalysisStatus(rowID, "failed", err.Error())
		return nil, err
	}

	dimensions := make(map[string]*SymbolDimensionResult, len(selectedFrameworkIDs))
	normalizedDimensionOutputs := make(map[string]string, len(selectedFrameworkIDs))
	for _, frameworkID := range selectedFrameworkIDs {
		rawOutput, ok := dimensionOutputs[frameworkID]
		if !ok || strings.TrimSpace(rawOutput) == "" {
			continue
		}

		parsed, parseErr := parseSymbolDimensionResult(rawOutput)
		if parseErr != nil {
			c.Logger().Warn("failed to parse framework result", "framework", frameworkID, "err", parseErr)
			continue
		}
		normalizeDimensionResult(parsed, frameworkID)
		dimensions[frameworkID] = parsed

		normalizedJSON, marshalErr := json.Marshal(parsed)
		if marshalErr != nil {
			c.Logger().Warn("failed to marshal normalized framework result", "framework", frameworkID, "err", marshalErr)
			normalizedDimensionOutputs[frameworkID] = rawOutput
			continue
		}
		normalizedDimensionOutputs[frameworkID] = string(normalizedJSON)
	}
	if len(dimensions) < minFrameworkAnalyses {
		err := fmt.Errorf("framework analyses parsed less than %d", minFrameworkAnalyses)
		_ = c.updateSymbolAnalysisStatus(rowID, "failed", err.Error())
		return nil, err
	}

	preferenceContext := symbolPreferenceContext{
		RiskProfile:    normalizedReq.RiskProfile,
		Horizon:        normalizedReq.Horizon,
		AdviceStyle:    normalizedReq.AdviceStyle,
		StrategyPrompt: normalizedReq.StrategyPrompt,
	}
	weightContext := buildSynthesisWeightContext(contextData, preferenceContext)

	// Run synthesis agent sequentially.
	synthesisOutput, err := runSynthesisAgent(
		ctx,
		endpointURL,
		normalizedReq.APIKey,
		normalizedReq.Model,
		symbolContextJSON,
		normalizedDimensionOutputs,
		selectedFrameworkIDs,
		weightContext,
		onDelta,
	)
	if err != nil {
		_ = c.updateSymbolAnalysisStatus(rowID, "failed", err.Error())
		return nil, fmt.Errorf("synthesis agent failed: %w", err)
	}

	synthesis, err := parseSynthesisResult(synthesisOutput)
	if err != nil {
		_ = c.updateSymbolAnalysisStatus(rowID, "failed", err.Error())
		return nil, fmt.Errorf("parse synthesis result: %w", err)
	}

	normalizeSynthesisResult(synthesis, contextData, selectedFrameworkIDs)

	synthesisToSave := synthesisOutput
	if normalizedJSON, marshalErr := json.Marshal(synthesis); marshalErr == nil {
		synthesisToSave = string(normalizedJSON)
	} else {
		c.Logger().Warn("failed to marshal normalized synthesis", "err", marshalErr)
	}

	// Save completed result.
	result := &SymbolAnalysisResult{
		ID:         rowID,
		Symbol:     normalizedReq.Symbol,
		Currency:   normalizedReq.Currency,
		Model:      normalizedReq.Model,
		Status:     "completed",
		Dimensions: dimensions,
		Synthesis:  synthesis,
		CreatedAt:  NowRFC3339InShanghai(),
	}

	if err := c.saveCompletedSymbolAnalysis(rowID, normalizedDimensionOutputs, synthesisToSave, enrichedContext); err != nil {
		return nil, fmt.Errorf("save analysis result: %w", err)
	}

	return result, nil
}

func (c *Core) insertPendingSymbolAnalysis(req SymbolAnalysisRequest) (int64, error) {
	result, err := c.db.Exec(
		`INSERT INTO symbol_analyses (symbol, currency, model, status, strategy_prompt)
		 VALUES (?, ?, ?, 'pending', ?)`,
		req.Symbol, req.Currency, req.Model, req.StrategyPrompt,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (c *Core) updateSymbolAnalysisStatus(id int64, status, errMsg string) error {
	_, err := c.db.Exec(
		`UPDATE symbol_analyses SET status = ?, error_message = ?, completed_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, errMsg, id,
	)
	return err
}

func orderedDimensionOutputKeys(dimensionOutputs map[string]string) []string {
	orderedKeys := make([]string, 0, len(dimensionOutputs))
	seen := make(map[string]struct{}, len(dimensionOutputs))

	for _, framework := range symbolFrameworkCatalog {
		output := strings.TrimSpace(dimensionOutputs[framework.ID])
		if output == "" {
			continue
		}
		orderedKeys = append(orderedKeys, framework.ID)
		seen[framework.ID] = struct{}{}
	}

	for _, legacyKey := range legacyDimensionColumnOrder {
		output := strings.TrimSpace(dimensionOutputs[legacyKey])
		if output == "" {
			continue
		}
		if _, exists := seen[legacyKey]; exists {
			continue
		}
		orderedKeys = append(orderedKeys, legacyKey)
		seen[legacyKey] = struct{}{}
	}

	var extras []string
	for key, output := range dimensionOutputs {
		if strings.TrimSpace(output) == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		extras = append(extras, key)
	}
	sort.Strings(extras)
	orderedKeys = append(orderedKeys, extras...)
	return orderedKeys
}

func mapDimensionOutputsToLegacyColumns(dimensionOutputs map[string]string) (string, string, string, string) {
	var slots [4]string
	orderedKeys := orderedDimensionOutputKeys(dimensionOutputs)
	for idx, key := range orderedKeys {
		if idx >= len(slots) {
			break
		}
		slots[idx] = dimensionOutputs[key]
	}
	return slots[0], slots[1], slots[2], slots[3]
}

func orderedDimensionIDs(dimensions map[string]*SymbolDimensionResult) []string {
	if len(dimensions) == 0 {
		return nil
	}

	ordered := make([]string, 0, len(dimensions))
	seen := make(map[string]struct{}, len(dimensions))
	for _, framework := range symbolFrameworkCatalog {
		if _, ok := dimensions[framework.ID]; !ok {
			continue
		}
		ordered = append(ordered, framework.ID)
		seen[framework.ID] = struct{}{}
	}
	for _, legacyKey := range legacyDimensionColumnOrder {
		if _, ok := dimensions[legacyKey]; !ok {
			continue
		}
		if _, exists := seen[legacyKey]; exists {
			continue
		}
		ordered = append(ordered, legacyKey)
		seen[legacyKey] = struct{}{}
	}

	var extras []string
	for key := range dimensions {
		if _, exists := seen[key]; exists {
			continue
		}
		extras = append(extras, key)
	}
	sort.Strings(extras)
	ordered = append(ordered, extras...)
	return ordered
}

func (c *Core) saveCompletedSymbolAnalysis(id int64, dimensionOutputs map[string]string, synthesisOutput string, externalDataSummary string) error {
	macroOutput, industryOutput, companyOutput, internationalOutput := mapDimensionOutputsToLegacyColumns(dimensionOutputs)

	_, err := c.db.Exec(
		`UPDATE symbol_analyses
		 SET status = 'completed',
		     macro_analysis = ?,
		     industry_analysis = ?,
		     company_analysis = ?,
		     international_analysis = ?,
		     synthesis = ?,
		     external_data_summary = ?,
		     completed_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		macroOutput,
		industryOutput,
		companyOutput,
		internationalOutput,
		synthesisOutput,
		externalDataSummary,
		id,
	)
	return err
}

// GetSymbolAnalysis returns the latest completed analysis for a symbol.
func (c *Core) GetSymbolAnalysis(symbol, currency string) (*SymbolAnalysisResult, error) {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	currency = strings.TrimSpace(strings.ToUpper(currency))

	var (
		id               int64
		model, status    string
		macroRaw         sql.NullString
		industryRaw      sql.NullString
		companyRaw       sql.NullString
		internationalRaw sql.NullString
		synthesisRaw     sql.NullString
		errorMessage     sql.NullString
		createdAt        string
		completedAtRaw   sql.NullString
	)

	err := c.db.QueryRow(
		`SELECT id, model, status, macro_analysis, industry_analysis, company_analysis, international_analysis,
		        synthesis, error_message, created_at, completed_at
		 FROM symbol_analyses
		 WHERE symbol = ? AND currency = ? AND status = 'completed'
		 ORDER BY created_at DESC LIMIT 1`,
		symbol, currency,
	).Scan(&id, &model, &status, &macroRaw, &industryRaw, &companyRaw, &internationalRaw,
		&synthesisRaw, &errorMessage, &createdAt, &completedAtRaw)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query symbol analysis: %w", err)
	}

	return buildSymbolAnalysisResult(id, symbol, currency, model, status,
		macroRaw, industryRaw, companyRaw, internationalRaw,
		synthesisRaw, errorMessage, createdAt, completedAtRaw)
}

// GetSymbolAnalysisHistory returns recent completed analyses for a symbol.
func (c *Core) GetSymbolAnalysisHistory(symbol, currency string, limit int) ([]SymbolAnalysisResult, error) {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	currency = strings.TrimSpace(strings.ToUpper(currency))
	if limit <= 0 {
		limit = 10
	}

	rows, err := c.db.Query(
		`SELECT id, model, status, macro_analysis, industry_analysis, company_analysis, international_analysis,
		        synthesis, error_message, created_at, completed_at
		 FROM symbol_analyses
		 WHERE symbol = ? AND currency = ? AND status = 'completed'
		 ORDER BY created_at DESC LIMIT ?`,
		symbol, currency, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query symbol analysis history: %w", err)
	}
	defer rows.Close()

	var results []SymbolAnalysisResult
	for rows.Next() {
		var (
			id               int64
			model, status    string
			macroRaw         sql.NullString
			industryRaw      sql.NullString
			companyRaw       sql.NullString
			internationalRaw sql.NullString
			synthesisRaw     sql.NullString
			errorMessage     sql.NullString
			createdAt        string
			completedAtRaw   sql.NullString
		)
		if err := rows.Scan(&id, &model, &status, &macroRaw, &industryRaw, &companyRaw, &internationalRaw,
			&synthesisRaw, &errorMessage, &createdAt, &completedAtRaw); err != nil {
			return nil, fmt.Errorf("scan symbol analysis row: %w", err)
		}
		result, err := buildSymbolAnalysisResult(id, symbol, currency, model, status,
			macroRaw, industryRaw, companyRaw, internationalRaw,
			synthesisRaw, errorMessage, createdAt, completedAtRaw)
		if err != nil {
			continue
		}
		results = append(results, *result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []SymbolAnalysisResult{}
	}
	return results, nil
}

func buildSymbolAnalysisResult(
	id int64, symbol, currency, model, status string,
	macroRaw, industryRaw, companyRaw, internationalRaw, synthesisRaw, errorMessage sql.NullString,
	createdAt string, completedAtRaw sql.NullString,
) (*SymbolAnalysisResult, error) {
	dimensions := make(map[string]*SymbolDimensionResult)
	dimensionRaws := []struct {
		FallbackKey string
		Raw         sql.NullString
	}{
		{FallbackKey: "macro", Raw: macroRaw},
		{FallbackKey: "industry", Raw: industryRaw},
		{FallbackKey: "company", Raw: companyRaw},
		{FallbackKey: "international", Raw: internationalRaw},
	}
	for _, item := range dimensionRaws {
		if !item.Raw.Valid || strings.TrimSpace(item.Raw.String) == "" {
			continue
		}

		parsed, err := parseSymbolDimensionResult(item.Raw.String)
		if err != nil {
			continue
		}

		dimensionKey := strings.ToLower(strings.TrimSpace(parsed.Dimension))
		if dimensionKey == "" {
			dimensionKey = item.FallbackKey
		}
		normalizeDimensionResult(parsed, dimensionKey)
		dimensions[parsed.Dimension] = parsed
	}

	var synthesis *SymbolSynthesisResult
	if synthesisRaw.Valid && synthesisRaw.String != "" {
		parsed, err := parseSynthesisResult(synthesisRaw.String)
		if err == nil {
			synthesis = parsed
		}
	}
	if synthesis != nil {
		normalizeSynthesisResult(synthesis, nil, orderedDimensionIDs(dimensions))
	}

	completedAt := ""
	if completedAtRaw.Valid {
		completedAt = completedAtRaw.String
	}
	errMsg := ""
	if errorMessage.Valid {
		errMsg = errorMessage.String
	}

	return &SymbolAnalysisResult{
		ID:           id,
		Symbol:       symbol,
		Currency:     currency,
		Model:        model,
		Status:       status,
		Dimensions:   dimensions,
		Synthesis:    synthesis,
		ErrorMessage: errMsg,
		CreatedAt:    createdAt,
		CompletedAt:  completedAt,
	}, nil
}
