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

const symbolAnalysisTimeout = aiTotalRequestTimeout

const macroAnalysisSystemPrompt = `你是一个专注于宏观经济政策分析的投资研究助手。
你需要分析以下维度：
1) 货币政策：利率走向、流动性环境、央行政策倾向
2) 财政政策：政府支出、税收政策、产业补贴
3) 监管环境：行业监管变化、政策风险
4) 经济周期：当前经济阶段、GDP增长、通胀水平、就业数据

请基于给定标的的相关市场和经济环境进行分析。
必须输出 JSON 对象，不要输出 Markdown，不要输出额外文字。
JSON 字段必须包含：
- dimension: "macro"
- rating: string (positive/neutral/negative)
- confidence: string (high/medium/low)
- key_points: string[]
- risks: string[]
- opportunities: string[]
- summary: string

要求：
- 分析必须具体到该标的所在的市场和行业
- 结合最新的宏观经济数据进行判断
- 评级必须有充分的数据支撑`

const industryAnalysisSystemPrompt = `你是一个专注于行业趋势分析的投资研究助手。
你需要分析以下维度：
1) 行业趋势：行业增长率、市场规模、发展阶段
2) 竞争格局：市场集中度、主要竞争者、进入壁垒
3) 技术颠覆：技术创新、数字化转型、替代技术威胁
4) 供应链：上下游关系、原材料成本、供应链稳定性

请基于给定标的所在行业进行深入分析。
必须输出 JSON 对象，不要输出 Markdown，不要输出额外文字。
JSON 字段必须包含：
- dimension: "industry"
- rating: string (positive/neutral/negative)
- confidence: string (high/medium/low)
- key_points: string[]
- risks: string[]
- opportunities: string[]
- summary: string

要求：
- 分析必须聚焦于该标的所在的具体细分行业
- 对比同行业其他主要公司
- 识别行业的关键驱动因素和风险因素`

const companyAnalysisSystemPrompt = `你是一个专注于公司基本面分析的投资研究助手。
你需要分析以下维度：
1) 财务分析：营收增长、利润率（毛利率/净利率）、资产负债率、自由现金流
2) 估值分析：PE、PB、DCF估值、与历史和同行的对比
3) 护城河：品牌优势、网络效应、成本优势、转换成本、规模效应
4) 管理层：管理团队能力、公司治理、股东回报政策

请基于给定标的进行深入的基本面分析。
必须输出 JSON 对象，不要输出 Markdown，不要输出额外文字。
JSON 字段必须包含：
- dimension: "company"
- rating: string (positive/neutral/negative)
- confidence: string (high/medium/low)
- key_points: string[]
- risks: string[]
- opportunities: string[]
- summary: string
- valuation_assessment: string (对当前估值水平的评估)

要求：
- 必须获取该公司近3年的财务数据进行分析
- 估值分析必须包含具体的估值指标数据
- 基于华尔街估值逻辑进行分析
- 对护城河的分析要具体而非泛泛而谈`

const internationalAnalysisSystemPrompt = `你是一个专注于国际政治经济分析的投资研究助手。
你需要分析以下维度：
1) 贸易关系：贸易摩擦、关税政策、贸易协定
2) 地缘政治：国际关系、地区冲突、制裁风险
3) 汇率风险：汇率走势、资本管制、外汇政策
4) 跨境资本流动：外资流向、QFII/QDII、互联互通机制

请基于给定标的的国际环境进行分析。
必须输出 JSON 对象，不要输出 Markdown，不要输出额外文字。
JSON 字段必须包含：
- dimension: "international"
- rating: string (positive/neutral/negative)
- confidence: string (high/medium/low)
- key_points: string[]
- risks: string[]
- opportunities: string[]
- summary: string

要求：
- 分析必须结合标的所在市场的国际环境
- 评估跨境风险对该标的的具体影响
- 关注最新的国际政治经济动态`

const symbolSynthesisSystemPrompt = `你是一个综合投资分析师，负责整合多维度分析结果给出最终投资建议。
你将收到四个维度的分析结果（宏观经济政策、行业竞争格局、公司基本面、国际政治经济），
需要综合权衡所有维度，结合持仓规模、当前仓位占比、用户投资偏好和策略约束，给出最终投资建议。

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
- 综合评级必须有充分的逻辑依据
- 如果各维度结论冲突，需要明确说明权衡逻辑
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
	RiskProfile string `json:"risk_profile"`
	Horizon     string `json:"horizon"`
	AdviceStyle string `json:"advice_style"`
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

type dimensionAgent struct {
	Dimension    string
	SystemPrompt string
}

var dimensionAgents = []dimensionAgent{
	{"macro", macroAnalysisSystemPrompt},
	{"industry", industryAnalysisSystemPrompt},
	{"company", companyAnalysisSystemPrompt},
	{"international", internationalAnalysisSystemPrompt},
}

type agentResult struct {
	Dimension string
	Content   string
	Error     error
}

func (c *Core) runDimensionAgents(ctx context.Context, endpoint, apiKey, model, userPrompt string, onDelta func(string)) (map[string]string, error) {
	ch := make(chan agentResult, len(dimensionAgents))
	var wg sync.WaitGroup

	for _, a := range dimensionAgents {
		wg.Add(1)
		go func(dim, sysPrompt string) {
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
					onDelta("[" + dim + "] " + delta)
				},
			})
			if err != nil {
				ch <- agentResult{Dimension: dim, Error: err}
				return
			}
			ch <- agentResult{Dimension: dim, Content: res.Content}
		}(a.Dimension, a.SystemPrompt)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	outputs := make(map[string]string)
	var errs []string
	for r := range ch {
		if r.Error != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", r.Dimension, r.Error))
			continue
		}
		outputs[r.Dimension] = r.Content
	}

	if len(outputs) < 2 {
		return nil, fmt.Errorf("too many dimension agents failed (%d/%d): %s", len(errs), len(dimensionAgents), strings.Join(errs, "; "))
	}
	return outputs, nil
}

func runSynthesisAgent(
	ctx context.Context,
	endpoint, apiKey, model, symbolContext string,
	dimensionOutputs map[string]string,
	preferenceContext symbolPreferenceContext,
	onDelta func(string),
) (string, error) {
	dimensionJSON, err := json.Marshal(dimensionOutputs)
	if err != nil {
		return "", fmt.Errorf("marshal dimension outputs: %w", err)
	}
	preferenceJSON, err := json.Marshal(preferenceContext)
	if err != nil {
		return "", fmt.Errorf("marshal preference context: %w", err)
	}

	userPrompt := fmt.Sprintf(`请基于以下标的信息和四维度分析结果，综合给出投资建议：

标的信息：
%s

用户投资偏好与策略约束：
%s

各维度分析结果：
%s

决策硬约束：
1) 结论第一句必须直接给出 target_action + action_probability_percent。
2) action_probability_percent 必须是具体数字（例如 67），不能给区间，不能模糊措辞。
3) 必须明确说明“当前仓位占比”与“目标区间（如有）”之间差距。
4) 语言直接，不要客套，不要公关腔。`, symbolContext, string(preferenceJSON), string(dimensionJSON))

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

func normalizeSynthesisResult(result *SymbolSynthesisResult, context *symbolContextData) {
	if result == nil {
		return
	}
	result.ActionProbability = normalizeSynthesisProbability(result.Confidence, result.ActionProbability)
	result.PositionSuggestion = normalizeSynthesisPositionSuggestion(*result, context)
	result.OverallSummary = normalizeSynthesisSummary(*result)
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

func normalizeSynthesisSummary(result SymbolSynthesisResult) string {
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
	builder.WriteString(fmt.Sprintf("结论：%s，执行概率%.0f%%。", actionLabel, probability))
	builder.WriteString("仓位：")
	builder.WriteString(position)
	builder.WriteString(factors)
	builder.WriteString(risks)

	summary := builder.String()
	summary = strings.ReplaceAll(summary, "看情况", "")
	summary = strings.ReplaceAll(summary, "视情况", "")
	summary = strings.ReplaceAll(summary, "it depends", "")
	summary = strings.TrimSpace(summary)

	if len([]rune(summary)) > 200 {
		summary = string([]rune(summary)[:200])
		if !strings.HasSuffix(summary, "。") {
			summary += "。"
		}
	}

	if summary == "" {
		summary = fmt.Sprintf("结论：%s，执行概率%.0f%%。", actionLabel, probability)
	}

	return summary
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
	trimmed := strings.TrimSpace(input)
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

// buildDimensionUserPrompt constructs the user prompt for dimension agents,
// optionally injecting enriched context from external data.
func buildDimensionUserPrompt(symbolContext, enrichedContext string, req SymbolAnalysisRequest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请分析以下投资标的：\n%s\n", symbolContext))

	if enrichedContext != "" {
		sb.WriteString(fmt.Sprintf(`
以下是该标的的最新实时数据和新闻摘要（数据截至今日）：
%s

重要：请优先使用上述实时数据进行分析，而非你的训练数据中的过时信息。
`, enrichedContext))
	}

	sb.WriteString("\n请根据你的专业维度进行深入分析，并输出指定格式的 JSON 结果。")

	if req.RiskProfile != "" || req.Horizon != "" || req.AdviceStyle != "" {
		preferenceJSON, err := json.Marshal(symbolPreferenceContext{
			RiskProfile: req.RiskProfile,
			Horizon:     req.Horizon,
			AdviceStyle: req.AdviceStyle,
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

// AnalyzeSymbolWithStream runs symbol analysis and emits model delta chunks.
func (c *Core) AnalyzeSymbolWithStream(req SymbolAnalysisRequest, onDelta func(string)) (*SymbolAnalysisResult, error) {
	return c.analyzeSymbol(req, onDelta)
}

func (c *Core) analyzeSymbol(req SymbolAnalysisRequest, onDelta func(string)) (*SymbolAnalysisResult, error) {
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

	// Build user prompt for dimension agents.
	userPrompt := buildDimensionUserPrompt(symbolContextJSON, enrichedContext, normalizedReq)

	// Run 4 dimension agents in parallel.
	dimensionOutputs, err := c.runDimensionAgents(ctx, endpointURL, normalizedReq.APIKey, normalizedReq.Model, userPrompt, onDelta)
	if err != nil {
		_ = c.updateSymbolAnalysisStatus(rowID, "failed", err.Error())
		return nil, err
	}

	// Run synthesis agent sequentially.
	synthesisOutput, err := runSynthesisAgent(ctx, endpointURL, normalizedReq.APIKey, normalizedReq.Model, symbolContextJSON, dimensionOutputs, symbolPreferenceContext{
		RiskProfile: normalizedReq.RiskProfile,
		Horizon:     normalizedReq.Horizon,
		AdviceStyle: normalizedReq.AdviceStyle,
	}, onDelta)
	if err != nil {
		_ = c.updateSymbolAnalysisStatus(rowID, "failed", err.Error())
		return nil, fmt.Errorf("synthesis agent failed: %w", err)
	}

	// Parse all results.
	dimensions := make(map[string]*SymbolDimensionResult)
	for dim, raw := range dimensionOutputs {
		parsed, parseErr := parseSymbolDimensionResult(raw)
		if parseErr != nil {
			c.Logger().Warn("failed to parse dimension result", "dimension", dim, "err", parseErr)
			continue
		}
		dimensions[dim] = parsed
	}

	synthesis, err := parseSynthesisResult(synthesisOutput)
	if err != nil {
		_ = c.updateSymbolAnalysisStatus(rowID, "failed", err.Error())
		return nil, fmt.Errorf("parse synthesis result: %w", err)
	}

	normalizeSynthesisResult(synthesis, contextData)

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

	if err := c.saveCompletedSymbolAnalysis(rowID, dimensionOutputs, synthesisToSave, enrichedContext); err != nil {
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

func (c *Core) saveCompletedSymbolAnalysis(id int64, dimensionOutputs map[string]string, synthesisOutput string, externalDataSummary string) error {
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
		dimensionOutputs["macro"],
		dimensionOutputs["industry"],
		dimensionOutputs["company"],
		dimensionOutputs["international"],
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
	dimensionRaws := map[string]sql.NullString{
		"macro":         macroRaw,
		"industry":      industryRaw,
		"company":       companyRaw,
		"international": internationalRaw,
	}
	for dim, raw := range dimensionRaws {
		if raw.Valid && raw.String != "" {
			parsed, err := parseSymbolDimensionResult(raw.String)
			if err == nil {
				dimensions[dim] = parsed
			}
		}
	}

	var synthesis *SymbolSynthesisResult
	if synthesisRaw.Valid && synthesisRaw.String != "" {
		parsed, err := parseSynthesisResult(synthesisRaw.String)
		if err == nil {
			synthesis = parsed
		}
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
