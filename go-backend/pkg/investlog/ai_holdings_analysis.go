package investlog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"google.golang.org/genai"
)

const (
	defaultAIBaseURL      = "https://api.openai.com/v1"
	defaultGeminiBaseURL  = "https://generativelanguage.googleapis.com/v1beta"
	aiRequestTimeout      = 5 * time.Minute
	aiTotalRequestTimeout = 15 * time.Minute
	maxAIResponseBodySize = 2 << 20
	aiMaxOutputTokens     = 128000
	aiMaxInputTokens      = 200000
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

type aiChatCompletionRequest struct {
	EndpointURL  string
	APIKey       string
	Model        string
	SystemPrompt string
	UserPrompt   string
	Logger       *slog.Logger
}

type aiChatCompletionResult struct {
	Model   string
	Content string
}

var aiChatCompletion = requestAIChatCompletion
var aiChatCompletionStream = requestAIChatCompletionStream
var aiGeminiCompletion = requestAIByGeminiNative

type aiChunkCallback func(string) error

// AnalyzeHoldings analyzes current holdings using an OpenAI-compatible model.
func (c *Core) AnalyzeHoldings(req HoldingsAnalysisRequest) (*HoldingsAnalysisResult, error) {
	return c.analyzeHoldings(req, nil)
}

// AnalyzeHoldingsStream analyzes holdings and streams raw model chunks.
func (c *Core) AnalyzeHoldingsStream(req HoldingsAnalysisRequest, onChunk func(string) error) (*HoldingsAnalysisResult, error) {
	return c.analyzeHoldings(req, onChunk)
}

func (c *Core) analyzeHoldings(req HoldingsAnalysisRequest, onChunk aiChunkCallback) (*HoldingsAnalysisResult, error) {
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

	chatRequest := aiChatCompletionRequest{
		EndpointURL:  endpointURL,
		APIKey:       normalizedReq.APIKey,
		Model:        normalizedReq.Model,
		SystemPrompt: holdingsAnalysisSystemPrompt,
		UserPrompt:   userPrompt,
		Logger:       c.Logger(),
	}

	var chatResult aiChatCompletionResult
	if onChunk != nil {
		chatResult, err = aiChatCompletionStream(ctx, chatRequest, onChunk)
	} else {
		chatResult, err = aiChatCompletion(ctx, chatRequest)
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

func buildAICompletionsEndpoint(baseURL string) (string, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		trimmed = defaultAIBaseURL
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	trimmed = strings.TrimRight(trimmed, "/")
	lower := strings.ToLower(trimmed)

	endpoint := ""
	switch {
	case strings.HasSuffix(lower, "/chat/completions"):
		endpoint = trimmed
	case strings.HasSuffix(lower, "/responses"):
		endpoint = trimmed
	case strings.HasSuffix(lower, "/v1"):
		endpoint = trimmed + "/chat/completions"
	default:
		endpoint = trimmed + "/v1/chat/completions"
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("invalid base_url scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid base_url host")
	}
	return endpoint, nil
}

func requestAIChatCompletionStream(ctx context.Context, req aiChatCompletionRequest, onChunk aiChunkCallback) (aiChatCompletionResult, error) {
	if onChunk == nil {
		return requestAIChatCompletion(ctx, req)
	}
	if isGeminiRequest(req.EndpointURL, req.Model) {
		return aiGeminiCompletion(ctx, req, onChunk)
	}

	result, err := requestAIChatCompletion(ctx, req)
	if err != nil {
		return aiChatCompletionResult{}, err
	}
	if err := onChunk(result.Content); err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("stream callback failed: %w", err)
	}
	return result, nil
}

func requestAIByGeminiNative(ctx context.Context, req aiChatCompletionRequest, onChunk aiChunkCallback) (aiChatCompletionResult, error) {
	logAIPromptDebug(req.Logger, req.EndpointURL, req.Model, req.SystemPrompt, req.UserPrompt)

	if shouldFallbackToGeminiDefaultBaseURL(req.EndpointURL) {
		logger := req.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("gemini request uses openai default base url; fallback to gemini base url",
			"configured_endpoint", req.EndpointURL,
			"fallback_base_url", defaultGeminiBaseURL,
		)
	}

	clientConfig, err := buildGeminiClientConfig(req.EndpointURL, req.APIKey)
	if err != nil {
		return aiChatCompletionResult{}, err
	}
	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("create gemini client failed: %w", err)
	}

	requestConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: req.SystemPrompt}},
		},
		Temperature:      genai.Ptr(float32(0.2)),
		MaxOutputTokens:  aiMaxOutputTokens,
		ResponseMIMEType: "application/json",
	}
	contents := genai.Text(req.UserPrompt)

	if onChunk == nil {
		response, err := client.Models.GenerateContent(ctx, req.Model, contents, requestConfig)
		if err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("gemini generate content failed: %w", err)
		}
		content := strings.TrimSpace(response.Text())
		if content == "" {
			return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
		}
		model := strings.TrimSpace(response.ModelVersion)
		if model == "" {
			model = req.Model
		}
		return aiChatCompletionResult{Model: model, Content: content}, nil
	}

	accumulated := ""
	model := strings.TrimSpace(req.Model)
	for response, err := range client.Models.GenerateContentStream(ctx, req.Model, contents, requestConfig) {
		if err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("gemini stream generate content failed: %w", err)
		}
		if response == nil {
			continue
		}

		if model == "" {
			model = strings.TrimSpace(response.ModelVersion)
		}

		chunkText := response.Text()
		if chunkText == "" {
			continue
		}
		delta := chunkText
		if strings.HasPrefix(chunkText, accumulated) {
			delta = chunkText[len(accumulated):]
		}
		if delta == "" {
			continue
		}

		accumulated += delta
		if err := onChunk(delta); err != nil {
			return aiChatCompletionResult{}, fmt.Errorf("stream callback failed: %w", err)
		}
	}

	content := strings.TrimSpace(accumulated)
	if content == "" {
		return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
	}
	if model == "" {
		model = req.Model
	}
	return aiChatCompletionResult{Model: model, Content: content}, nil
}

func buildGeminiClientConfig(endpoint, apiKey string) (*genai.ClientConfig, error) {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	if shouldFallbackToGeminiDefaultBaseURL(normalizedEndpoint) {
		normalizedEndpoint = defaultGeminiBaseURL
	}

	baseURL, apiVersion, err := parseGeminiBaseURLAndVersion(normalizedEndpoint)
	if err != nil {
		return nil, err
	}
	return &genai.ClientConfig{
		APIKey:  strings.TrimSpace(apiKey),
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			BaseURL:    baseURL,
			APIVersion: apiVersion,
		},
	}, nil
}

func shouldFallbackToGeminiDefaultBaseURL(endpoint string) bool {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return true
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "api.openai.com")
}

func parseGeminiBaseURLAndVersion(endpoint string) (string, string, error) {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		trimmed = defaultGeminiBaseURL
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", fmt.Errorf("invalid gemini endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", fmt.Errorf("invalid gemini endpoint scheme: %s", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", "", fmt.Errorf("invalid gemini endpoint host")
	}

	path := strings.Trim(parsed.Path, "/")
	segments := []string{}
	if path != "" {
		segments = strings.Split(path, "/")
	}

	apiVersion := "v1beta"
	prefixSegments := []string{}
	foundVersion := false
	for idx, segment := range segments {
		segmentLower := strings.ToLower(strings.TrimSpace(segment))
		if strings.HasPrefix(segmentLower, "v1") {
			apiVersion = segment
			prefixSegments = segments[:idx]
			foundVersion = true
			break
		}
	}
	if !foundVersion {
		prefixSegments = segments
	}

	basePath := strings.Trim(strings.Join(prefixSegments, "/"), "/")
	baseURL := fmt.Sprintf("%s://%s/", parsed.Scheme, parsed.Host)
	if basePath != "" {
		baseURL += basePath + "/"
	}
	return baseURL, apiVersion, nil
}

func isGeminiRequest(endpointURL, model string) bool {
	modelLower := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(modelLower, "gemini") {
		return true
	}

	endpointLower := strings.ToLower(strings.TrimSpace(endpointURL))
	if endpointLower == "" {
		return false
	}
	if strings.Contains(endpointLower, "generativelanguage.googleapis.com") {
		return true
	}
	if strings.Contains(endpointLower, "/gemini") {
		return true
	}
	return false
}

func requestAIChatCompletion(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
	if isGeminiRequest(req.EndpointURL, req.Model) {
		return aiGeminiCompletion(ctx, req, nil)
	}

	logger := req.Logger
	if logger == nil {
		logger = slog.Default()
	}

	endpoint := strings.TrimSpace(req.EndpointURL)
	if strings.HasSuffix(strings.ToLower(endpoint), "/responses") {
		return requestAIByResponsesCandidates(ctx, req, endpoint)
	}

	chatCandidates := collectChatCandidates(endpoint)
	chatErrors := make([]string, 0, len(chatCandidates))
	sameEndpointErrors := []string{}
	allowResponsesFallback := false

	for _, candidate := range chatCandidates {
		logger.Info("ai analyze: try chat endpoint", "endpoint", candidate, "model", req.Model)
		chatResult, err := requestAIByChatCompletions(ctx, req, candidate)
		if err == nil {
			logger.Info("ai analyze: chat endpoint succeeded", "endpoint", candidate)
			return chatResult, nil
		}
		logger.Warn("ai analyze: chat endpoint failed", "endpoint", candidate, "err", err)
		chatErrors = append(chatErrors, fmt.Sprintf("%s -> %v", candidate, err))
		if shouldFallbackToResponses(err) || shouldFallbackToAltEndpoint(err) {
			allowResponsesFallback = true
		}

		if shouldFallbackToResponses(err) {
			logger.Info("ai analyze: try responses payload on same endpoint", "endpoint", candidate)
			sameEndpointResult, sameErr := requestAIByResponses(ctx, req, candidate)
			if sameErr == nil {
				logger.Info("ai analyze: same endpoint with responses payload succeeded", "endpoint", candidate)
				return sameEndpointResult, nil
			}
			logger.Warn("ai analyze: same endpoint with responses payload failed", "endpoint", candidate, "err", sameErr)
			sameEndpointErrors = append(sameEndpointErrors, fmt.Sprintf("%s -> %v", candidate, sameErr))

			logger.Info("ai analyze: try hybrid payload on same endpoint", "endpoint", candidate)
			hybridResult, hybridErr := requestAIByHybridPayload(ctx, req, candidate)
			if hybridErr == nil {
				logger.Info("ai analyze: same endpoint with hybrid payload succeeded", "endpoint", candidate)
				return hybridResult, nil
			}
			logger.Warn("ai analyze: same endpoint with hybrid payload failed", "endpoint", candidate, "err", hybridErr)
			sameEndpointErrors = append(sameEndpointErrors, fmt.Sprintf("%s(hybrid) -> %v", candidate, hybridErr))

			if isTimeoutError(sameErr) || isTimeoutError(hybridErr) {
				return aiChatCompletionResult{}, fmt.Errorf("ai upstream timeout on %s; try a faster model or retry later", candidate)
			}
		}
	}

	if !allowResponsesFallback {
		if len(sameEndpointErrors) > 0 {
			return aiChatCompletionResult{}, fmt.Errorf("chat completion failed: %s; same-endpoint responses attempts failed: %s", strings.Join(chatErrors, " | "), strings.Join(sameEndpointErrors, " | "))
		}
		return aiChatCompletionResult{}, fmt.Errorf("chat completion failed: %s", strings.Join(chatErrors, " | "))
	}

	responsesResult, err := requestAIByResponsesCandidates(ctx, req, endpoint)
	if err == nil {
		logger.Info("ai analyze: responses fallback succeeded")
		return responsesResult, nil
	}
	logger.Error("ai analyze: responses fallback failed", "err", err)

	if len(sameEndpointErrors) > 0 {
		return aiChatCompletionResult{}, fmt.Errorf("chat completion failed (%s); same-endpoint responses attempts failed (%s); responses fallback failed: %w", strings.Join(chatErrors, " | "), strings.Join(sameEndpointErrors, " | "), err)
	}
	return aiChatCompletionResult{}, fmt.Errorf("chat completion failed (%s); responses fallback failed: %w", strings.Join(chatErrors, " | "), err)
}

func requestAIByResponsesCandidates(ctx context.Context, req aiChatCompletionRequest, endpoint string) (aiChatCompletionResult, error) {
	responseCandidates := collectResponsesCandidates(endpoint)
	errors := make([]string, 0, len(responseCandidates))
	for _, candidate := range responseCandidates {
		result, err := requestAIByResponses(ctx, req, candidate)
		if err == nil {
			return result, nil
		}
		errors = append(errors, fmt.Sprintf("%s -> %v", candidate, err))
	}
	return aiChatCompletionResult{}, fmt.Errorf("responses attempts failed: %s", strings.Join(errors, " | "))
}

func collectChatCandidates(endpoint string) []string {
	result := []string{}
	addUniqueString(&result, strings.TrimSpace(endpoint))
	addUniqueString(&result, toAltChatEndpoint(endpoint))
	return result
}

func collectResponsesCandidates(endpoint string) []string {
	chatCandidates := collectChatCandidates(endpoint)
	result := []string{}
	for _, candidate := range chatCandidates {
		responsesEndpoint := toResponsesEndpoint(candidate)
		addUniqueString(&result, responsesEndpoint)
		addUniqueString(&result, toAltResponsesEndpoint(responsesEndpoint))
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(endpoint)), "/responses") {
		addUniqueString(&result, strings.TrimSpace(endpoint))
		addUniqueString(&result, toAltResponsesEndpoint(endpoint))
	}
	return result
}

func addUniqueString(items *[]string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	for _, item := range *items {
		if item == trimmed {
			return
		}
	}
	*items = append(*items, trimmed)
}

func toAltChatEndpoint(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, "/v1/chat/completions") {
		return trimmed[:len(trimmed)-len("/v1/chat/completions")] + "/chat/completions"
	}
	if strings.HasSuffix(lower, "/chat/completions") && !strings.HasSuffix(lower, "/v1/chat/completions") {
		return trimmed[:len(trimmed)-len("/chat/completions")] + "/v1/chat/completions"
	}
	return ""
}

func toAltResponsesEndpoint(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, "/v1/responses") {
		return trimmed[:len(trimmed)-len("/v1/responses")] + "/responses"
	}
	if strings.HasSuffix(lower, "/responses") && !strings.HasSuffix(lower, "/v1/responses") {
		return trimmed[:len(trimmed)-len("/responses")] + "/v1/responses"
	}
	return ""
}

func requestAIByChatCompletions(ctx context.Context, req aiChatCompletionRequest, endpoint string) (aiChatCompletionResult, error) {
	logAIPromptDebug(req.Logger, endpoint, req.Model, req.SystemPrompt, req.UserPrompt)

	payload := map[string]any{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"temperature":           0.2,
		"stream":                false,
		"max_tokens":            aiMaxOutputTokens,
		"max_completion_tokens": aiMaxOutputTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("marshal ai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("build ai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	respBody, err := executeAIRequest(httpReq, req.Logger)
	if err != nil {
		return aiChatCompletionResult{}, err
	}

	model, content, err := decodeAIModelAndContent(respBody)
	if err != nil {
		return aiChatCompletionResult{}, err
	}
	if content == "" {
		return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
	}
	return aiChatCompletionResult{Model: model, Content: content}, nil
}

func requestAIByResponses(ctx context.Context, req aiChatCompletionRequest, endpoint string) (aiChatCompletionResult, error) {
	payload := map[string]any{
		"model":             req.Model,
		"instructions":      req.SystemPrompt,
		"input":             req.UserPrompt,
		"temperature":       0.2,
		"stream":            false,
		"max_output_tokens": aiMaxOutputTokens,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
	}
	return requestAIByPayload(ctx, req, endpoint, payload)
}

func requestAIByHybridPayload(ctx context.Context, req aiChatCompletionRequest, endpoint string) (aiChatCompletionResult, error) {
	payload := map[string]any{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"input":                 req.UserPrompt,
		"instructions":          req.SystemPrompt,
		"temperature":           0.2,
		"stream":                false,
		"max_tokens":            aiMaxOutputTokens,
		"max_completion_tokens": aiMaxOutputTokens,
		"max_output_tokens":     aiMaxOutputTokens,
	}
	return requestAIByPayload(ctx, req, endpoint, payload)
}

func requestAIByPayload(ctx context.Context, req aiChatCompletionRequest, endpoint string, payload map[string]any) (aiChatCompletionResult, error) {
	logAIPromptDebug(req.Logger, endpoint, req.Model, req.SystemPrompt, req.UserPrompt)

	body, err := json.Marshal(payload)
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("marshal ai request: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, aiRequestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return aiChatCompletionResult{}, fmt.Errorf("build ai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	respBody, err := executeAIRequest(httpReq, req.Logger)
	if err != nil {
		return aiChatCompletionResult{}, err
	}

	model, content, err := decodeAIModelAndContent(respBody)
	if err != nil {
		return aiChatCompletionResult{}, err
	}
	if content == "" {
		return aiChatCompletionResult{}, fmt.Errorf("ai response content is empty")
	}
	return aiChatCompletionResult{Model: model, Content: content}, nil
}

func logAIPromptDebug(logger *slog.Logger, endpoint, model, systemPrompt, userPrompt string) {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Debug("ai request prompt",
		"endpoint", strings.TrimSpace(endpoint),
		"model", strings.TrimSpace(model),
		"system_prompt", systemPrompt,
		"user_prompt", userPrompt,
	)
}

func logAIRawResponseDebug(logger *slog.Logger, endpoint string, statusCode int, body []byte) {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Debug("ai raw response",
		"endpoint", strings.TrimSpace(endpoint),
		"status_code", statusCode,
		"body_bytes", len(body),
		"raw_body", string(body),
	)
}

func decodeAIModelAndContent(body []byte) (string, string, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", "", fmt.Errorf("decode ai response: %w", err)
	}

	model := asString(raw["model"])
	if outputText := asString(raw["output_text"]); outputText != "" {
		return model, outputText, nil
	}

	if text := extractChoicesContent(raw["choices"]); text != "" {
		return model, text, nil
	}
	if text := extractOutputContent(raw["output"]); text != "" {
		return model, text, nil
	}
	if text := extractText(raw["content"]); text != "" {
		return model, text, nil
	}

	return model, "", fmt.Errorf("ai response content is empty")
}

func extractChoicesContent(value any) string {
	choices, ok := value.([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	first, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}
	message, ok := first["message"].(map[string]any)
	if ok {
		if text := extractText(message["content"]); text != "" {
			return text
		}
	}
	return extractText(first["text"])
}

func extractOutputContent(value any) string {
	outputs, ok := value.([]any)
	if !ok {
		return ""
	}
	for _, output := range outputs {
		outputMap, ok := output.(map[string]any)
		if !ok {
			continue
		}
		if text := extractText(outputMap["content"]); text != "" {
			return text
		}
		if text := extractText(outputMap["text"]); text != "" {
			return text
		}
	}
	return ""
}

func extractText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := extractText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	case map[string]any:
		if text := asString(typed["text"]); text != "" {
			return text
		}
		if text := asString(typed["value"]); text != "" {
			return text
		}
		if text := asString(typed["content"]); text != "" {
			return text
		}
		if text := extractText(typed["content"]); text != "" {
			return text
		}
		if text := extractText(typed["output_text"]); text != "" {
			return text
		}
	}
	return ""
}

func asString(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func executeAIRequest(httpReq *http.Request, logger *slog.Logger) ([]byte, error) {
	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ai request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxAIResponseBodySize))
	if err != nil {
		return nil, fmt.Errorf("read ai response: %w", err)
	}

	logAIRawResponseDebug(logger, httpReq.URL.String(), resp.StatusCode, respBody)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := parseAIErrorMessage(respBody)
		if message == "" {
			message = strings.TrimSpace(string(respBody))
		}
		if message == "" {
			message = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("ai upstream error: %s", message)
	}

	return respBody, nil
}

func shouldFallbackToResponses(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "input is required") || strings.Contains(message, "missing required parameter: input")
}

func shouldFallbackToAltEndpoint(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not found") || strings.Contains(message, "404") || strings.Contains(message, "unknown path")
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "context deadline exceeded") || strings.Contains(message, "timeout")
}

func toResponsesEndpoint(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if strings.HasSuffix(lower, "/responses") {
		return trimmed
	}
	if strings.HasSuffix(lower, "/chat/completions") {
		return trimmed[:len(trimmed)-len("/chat/completions")] + "/responses"
	}
	if strings.HasSuffix(lower, "/v1") {
		return trimmed + "/responses"
	}
	return ""
}

func parseAIErrorMessage(body []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if strings.TrimSpace(payload.Error.Message) != "" {
		return strings.TrimSpace(payload.Error.Message)
	}
	return strings.TrimSpace(payload.Message)
}

func parseHoldingsAnalysisResponse(content string) (*holdingsAnalysisModelResponse, error) {
	cleaned := cleanupModelJSON(content)
	var parsed holdingsAnalysisModelResponse
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w", err)
	}
	return &parsed, nil
}

func cleanupModelJSON(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
			trimmed = strings.Join(lines, "\n")
		}
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}
	return strings.TrimSpace(trimmed)
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
