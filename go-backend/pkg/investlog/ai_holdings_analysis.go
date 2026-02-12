package investlog

import (
	"bytes"
	"context"
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
)

const (
	defaultAIBaseURL      = "https://api.openai.com/v1"
	aiRequestTimeout      = 75 * time.Second
	maxAIResponseBodySize = 2 << 20
)

const holdingsAnalysisSystemPrompt = `你是一个专业投资组合分析助手,取得过连续50年年化20%的投资业绩。
你必须同时应用以下投资理念并做综合权衡：
1) 马尔基尔（Malkiel）：分散化、低成本、避免不必要择时与个股过度集中。
2) 达利欧（Dalio）：风险平衡、跨资产分散、关注相关性与宏观周期韧性。
3) 巴菲特（Buffett）：能力圈、长期主义、护城河与估值纪律。

请基于用户持仓快照输出“可执行、可解释、可审计”的建议。
必须输出 JSON 对象，不要输出 Markdown，不要输出额外文字。
JSON 字段必须包含：
- overall_summary: string
- risk_level: string
- key_findings: string[]
- recommendations: [{symbol, action, theory_tag, rationale, target_weight, priority}]
- disclaimer: string

要求：
- 对于所持有的个股，需要抓取该个股近3年的财务数据，包括但不限于：营收、净利润、毛利率、净利率、资产负债率、现金流等，基于华尔街的估值逻辑进行分析。
- recommendations 至少 3 条（如果持仓数量不足可少于 3 条，但必须说明原因）。
- action 取值建议使用 increase/reduce/hold/add。
- theory_tag 取值建议使用 Malkiel/Dalio/Buffett。
- 禁止承诺收益，必须体现风险提示。`

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

// HoldingsAnalysisResult is the structured response returned to clients.
type HoldingsAnalysisResult struct {
	GeneratedAt     string                           `json:"generated_at"`
	Model           string                           `json:"model"`
	Currency        string                           `json:"currency,omitempty"`
	OverallSummary  string                           `json:"overall_summary"`
	RiskLevel       string                           `json:"risk_level"`
	KeyFindings     []string                         `json:"key_findings"`
	Recommendations []HoldingsAnalysisRecommendation `json:"recommendations"`
	Disclaimer      string                           `json:"disclaimer"`
}

type holdingsAnalysisCurrencySnapshot struct {
	Currency         string                       `json:"currency"`
	TotalMarketValue float64                      `json:"total_market_value"`
	Symbols          []holdingsAnalysisSymbolItem `json:"symbols"`
}

type holdingsAnalysisSymbolItem struct {
	Symbol      string   `json:"symbol"`
	Name        string   `json:"name"`
	AssetType   string   `json:"asset_type"`
	AccountName string   `json:"account_name"`
	MarketValue float64  `json:"market_value"`
	WeightPct   float64  `json:"weight_pct"`
	PnLPct      *float64 `json:"pnl_pct,omitempty"`
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

// AnalyzeHoldings analyzes current holdings using an OpenAI-compatible model.
func (c *Core) AnalyzeHoldings(req HoldingsAnalysisRequest) (*HoldingsAnalysisResult, error) {
	normalizedReq, err := normalizeHoldingsAnalysisRequest(req)
	if err != nil {
		return nil, err
	}

	promptInput, err := c.buildHoldingsAnalysisPromptInput(normalizedReq.Currency)
	if err != nil {
		return nil, err
	}

	userPrompt, err := buildHoldingsAnalysisUserPrompt(promptInput, normalizedReq)
	if err != nil {
		return nil, err
	}

	endpointURL, err := buildAICompletionsEndpoint(normalizedReq.BaseURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), aiRequestTimeout)
	defer cancel()

	chatResult, err := aiChatCompletion(ctx, aiChatCompletionRequest{
		EndpointURL:  endpointURL,
		APIKey:       normalizedReq.APIKey,
		Model:        normalizedReq.Model,
		SystemPrompt: holdingsAnalysisSystemPrompt,
		UserPrompt:   userPrompt,
		Logger:       c.Logger(),
	})
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
		GeneratedAt:     time.Now().Format(time.RFC3339),
		Model:           model,
		Currency:        normalizedReq.Currency,
		OverallSummary:  overallSummary,
		RiskLevel:       riskLevel,
		KeyFindings:     normalizeFindings(parsed.KeyFindings),
		Recommendations: normalizeRecommendations(parsed.Recommendations),
		Disclaimer:      disclaimer,
	}
	return result, nil
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
			name := item.Symbol
			if item.Name != nil && strings.TrimSpace(*item.Name) != "" {
				name = strings.TrimSpace(*item.Name)
			}
			symbols = append(symbols, holdingsAnalysisSymbolItem{
				Symbol:      item.Symbol,
				Name:        name,
				AssetType:   item.AssetType,
				AccountName: item.AccountName,
				MarketValue: item.MarketValue,
				WeightPct:   item.Percent,
				PnLPct:      item.PnlPercent,
			})
		}

		holdings = append(holdings, holdingsAnalysisCurrencySnapshot{
			Currency:         curr,
			TotalMarketValue: currData.TotalMarketValue,
			Symbols:          symbols,
		})
	}

	return &holdingsAnalysisPromptInput{Holdings: holdings}, nil
}

func buildHoldingsAnalysisUserPrompt(input *holdingsAnalysisPromptInput, req HoldingsAnalysisRequest) (string, error) {
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

	prompt := fmt.Sprintf(`请基于以下输入完成分析并给出建议：
%s

输出要求：
1) 必须是 JSON 对象。
2) recommendations 中建议尽量覆盖：仓位集中风险、资产分散、回撤防御、长期价值。
3) 允许新增标的时，可给出 add 建议并点名标的。
4) 每条建议必须给出 theory_tag 和 rationale。
5) 若 strategy_prompt 非空，需优先吸收为策略偏好，但不得违反风险提示原则。`, string(payload))
	return prompt, nil
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

func requestAIChatCompletion(ctx context.Context, req aiChatCompletionRequest) (aiChatCompletionResult, error) {
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
	payload := map[string]any{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"temperature": 0.2,
		"stream":      false,
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

	respBody, err := executeAIRequest(httpReq)
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
		"model":        req.Model,
		"instructions": req.SystemPrompt,
		"input":        req.UserPrompt,
		"temperature":  0.2,
		"stream":       false,
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
		"input":        req.UserPrompt,
		"instructions": req.SystemPrompt,
		"temperature":  0.2,
		"stream":       false,
	}
	return requestAIByPayload(ctx, req, endpoint, payload)
}

func requestAIByPayload(ctx context.Context, req aiChatCompletionRequest, endpoint string, payload map[string]any) (aiChatCompletionResult, error) {
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

	respBody, err := executeAIRequest(httpReq)
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

func executeAIRequest(httpReq *http.Request) ([]byte, error) {
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
