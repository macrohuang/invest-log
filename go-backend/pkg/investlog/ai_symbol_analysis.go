package investlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

const symbolAnalysisTimeout = 225 * time.Second // 3 * aiRequestTimeout

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
需要综合权衡所有维度给出最终投资建议。

必须输出 JSON 对象，不要输出 Markdown，不要输出额外文字。
JSON 字段必须包含：
- overall_rating: string (strong_buy/buy/hold/reduce/strong_sell)
- confidence: string (high/medium/low)
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
- 禁止承诺收益，必须体现风险提示`

// SymbolAnalysisRequest defines inputs for per-symbol AI deep analysis.
type SymbolAnalysisRequest struct {
	BaseURL        string
	APIKey         string
	Model          string
	Symbol         string
	Currency       string
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
	normalized.StrategyPrompt = strings.TrimSpace(req.StrategyPrompt)
	return normalized, nil
}

type symbolContextData struct {
	Symbol      string  `json:"symbol"`
	Name        string  `json:"name,omitempty"`
	Currency    string  `json:"currency"`
	AssetType   string  `json:"asset_type,omitempty"`
	TotalShares float64 `json:"total_shares,omitempty"`
	AvgCost     float64 `json:"avg_cost,omitempty"`
	CostBasis   float64 `json:"cost_basis,omitempty"`
	LatestPrice float64 `json:"latest_price,omitempty"`
	MarketValue float64 `json:"market_value,omitempty"`
	PnLPercent  float64 `json:"pnl_percent,omitempty"`
	AccountName string  `json:"account_name,omitempty"`
}

func (c *Core) buildSymbolContext(symbol, currency string) (string, error) {
	bySymbol, err := c.GetHoldingsBySymbol()
	if err != nil {
		return "", fmt.Errorf("load holdings: %w", err)
	}

	currData, ok := bySymbol[currency]
	if !ok {
		return "", fmt.Errorf("no holdings found for currency: %s", currency)
	}

	var ctx symbolContextData
	found := false
	for _, s := range currData.Symbols {
		if strings.EqualFold(s.Symbol, symbol) {
			name := s.Symbol
			if s.Name != nil && strings.TrimSpace(*s.Name) != "" {
				name = strings.TrimSpace(*s.Name)
			}
			pnlPct := 0.0
			if s.PnlPercent != nil {
				pnlPct = *s.PnlPercent
			}
			latestPrice := 0.0
			if s.LatestPrice != nil {
				latestPrice = *s.LatestPrice
			}
			ctx = symbolContextData{
				Symbol:      s.Symbol,
				Name:        name,
				Currency:    currency,
				AssetType:   s.AssetType,
				TotalShares: s.TotalShares,
				AvgCost:     s.AvgCost,
				CostBasis:   s.CostBasis,
				LatestPrice: latestPrice,
				MarketValue: s.MarketValue,
				PnLPercent:  pnlPct,
				AccountName: s.AccountName,
			}
			found = true
			break
		}
	}
	if !found {
		// Allow analysis even without holdings (just symbol + currency)
		ctx = symbolContextData{
			Symbol:   symbol,
			Currency: currency,
		}
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		return "", fmt.Errorf("marshal symbol context: %w", err)
	}
	return string(data), nil
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

func (c *Core) runDimensionAgents(ctx context.Context, endpoint, apiKey, model, userPrompt string) (map[string]string, error) {
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

func runSynthesisAgent(ctx context.Context, endpoint, apiKey, model, symbolContext string, dimensionOutputs map[string]string, strategyPrompt string) (string, error) {
	dimensionJSON, err := json.Marshal(dimensionOutputs)
	if err != nil {
		return "", fmt.Errorf("marshal dimension outputs: %w", err)
	}

	userPrompt := fmt.Sprintf(`请基于以下标的信息和四维度分析结果，综合给出投资建议：

标的信息：
%s

各维度分析结果：
%s`, symbolContext, string(dimensionJSON))

	if strategyPrompt != "" {
		userPrompt += fmt.Sprintf(`

用户投资策略偏好（优先考虑）：
%s`, strategyPrompt)
	}

	result, err := aiChatCompletion(ctx, aiChatCompletionRequest{
		EndpointURL:  endpoint,
		APIKey:       apiKey,
		Model:        model,
		SystemPrompt: symbolSynthesisSystemPrompt,
		UserPrompt:   userPrompt,
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
	return &result, nil
}

// AnalyzeSymbol runs a multi-agent deep analysis for a single symbol.
func (c *Core) AnalyzeSymbol(req SymbolAnalysisRequest) (*SymbolAnalysisResult, error) {
	normalizedReq, err := normalizeSymbolAnalysisRequest(req)
	if err != nil {
		return nil, err
	}

	symbolContext, err := c.buildSymbolContext(normalizedReq.Symbol, normalizedReq.Currency)
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

	// Build user prompt for dimension agents.
	userPrompt := fmt.Sprintf(`请分析以下投资标的：
%s

请根据你的专业维度进行深入分析，并输出指定格式的 JSON 结果。`, symbolContext)

	if normalizedReq.StrategyPrompt != "" {
		userPrompt += fmt.Sprintf(`

用户投资策略偏好：
%s`, normalizedReq.StrategyPrompt)
	}

	// Run 4 dimension agents in parallel.
	dimensionOutputs, err := c.runDimensionAgents(ctx, endpointURL, normalizedReq.APIKey, normalizedReq.Model, userPrompt)
	if err != nil {
		_ = c.updateSymbolAnalysisStatus(rowID, "failed", err.Error())
		return nil, err
	}

	// Run synthesis agent sequentially.
	synthesisOutput, err := runSynthesisAgent(ctx, endpointURL, normalizedReq.APIKey, normalizedReq.Model, symbolContext, dimensionOutputs, normalizedReq.StrategyPrompt)
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

	// Save completed result.
	result := &SymbolAnalysisResult{
		ID:         rowID,
		Symbol:     normalizedReq.Symbol,
		Currency:   normalizedReq.Currency,
		Model:      normalizedReq.Model,
		Status:     "completed",
		Dimensions: dimensions,
		Synthesis:  synthesis,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	if err := c.saveCompletedSymbolAnalysis(rowID, dimensionOutputs, synthesisOutput); err != nil {
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

func (c *Core) saveCompletedSymbolAnalysis(id int64, dimensionOutputs map[string]string, synthesisOutput string) error {
	_, err := c.db.Exec(
		`UPDATE symbol_analyses
		 SET status = 'completed',
		     macro_analysis = ?,
		     industry_analysis = ?,
		     company_analysis = ?,
		     international_analysis = ?,
		     synthesis = ?,
		     completed_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		dimensionOutputs["macro"],
		dimensionOutputs["industry"],
		dimensionOutputs["company"],
		dimensionOutputs["international"],
		synthesisOutput,
		id,
	)
	return err
}

// GetSymbolAnalysis returns the latest completed analysis for a symbol.
func (c *Core) GetSymbolAnalysis(symbol, currency string) (*SymbolAnalysisResult, error) {
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	currency = strings.TrimSpace(strings.ToUpper(currency))

	var (
		id                   int64
		model, status        string
		macroRaw             sql.NullString
		industryRaw          sql.NullString
		companyRaw           sql.NullString
		internationalRaw     sql.NullString
		synthesisRaw         sql.NullString
		errorMessage         sql.NullString
		createdAt            string
		completedAtRaw       sql.NullString
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
