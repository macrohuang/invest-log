package investlog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

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
	if strings.TrimSpace(enrichedContext) == "" {
		retrievalContext := c.retrieveLatestSymbolContext(
			ctx,
			endpointURL,
			normalizedReq.APIKey,
			normalizedReq.Model,
			symbolContextJSON,
			normalizedReq.Symbol,
			normalizedReq.Currency,
		)
		if retrievalContext != "" {
			enrichedContext = retrievalContext
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

func (c *Core) retrieveLatestSymbolContext(
	ctx context.Context,
	endpoint, apiKey, model, symbolContext, symbol, currency string,
) string {
	systemPrompt := `你是投资研究实时检索助手。
必须联网检索并整理该标的的最新公开信息，只输出事实，不做投资建议。`

	userPrompt := fmt.Sprintf(`请检索并整理以下标的的最新信息：
symbol: %s
currency: %s
持仓上下文(JSON):
%s

输出格式（纯文本）：
【价格与估值】
- ...
【财报与经营进展】
- ...
【行业与政策】
- ...
【近期催化与风险】
- ...
【来源与日期】
- 来源名 + 日期(YYYY-MM-DD)

规则：
- 优先最近90天信息，尽量给出具体日期。
- 缺失信息必须写“缺口：...”。
- 禁止编造来源。`, symbol, currency, symbolContext)

	result, err := aiChatCompletion(ctx, aiChatCompletionRequest{
		EndpointURL:  endpoint,
		APIKey:       apiKey,
		Model:        model,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Logger:       c.Logger(),
	})
	if err != nil {
		c.Logger().Warn("symbol retrieval context failed",
			"symbol", symbol,
			"currency", currency,
			"model", model,
			"err", err,
		)
		return ""
	}

	return strings.TrimSpace(result.Content)
}
