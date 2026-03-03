package investlog

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
	BaseURL          string
	APIKey           string
	Model            string
	RetrievalBaseURL string
	RetrievalAPIKey  string
	RetrievalModel   string
	Symbol           string
	Currency         string
	RiskProfile      string
	Horizon          string
	AdviceStyle      string
	StrategyPrompt   string
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
