package investlog

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

func detectMarket(symbol, currency string) string {
	symbolType := detectSymbolType(symbol, currency, "stock")
	switch symbolType {
	case "a_share", "etf", "fund":
		return "cn"
	case "hk_stock", "hk_connect":
		return "hk"
	case "us_stock":
		return "us"
	default:
		return ""
	}
}

// fetchExternalDataImpl fetches external data in parallel from market-specific sources.
func fetchExternalDataImpl(ctx context.Context, symbol, currency string, logger *slog.Logger) *symbolExternalData {
	market := detectMarket(symbol, currency)
	if market == "" {
		if logger != nil {
			logger.Info("external data: unknown market, skipping", "symbol", symbol, "currency", currency)
		}
		return nil
	}

	sources := buildDataSources(market, symbol, currency)
	if len(sources) == 0 {
		return nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, externalDataFetchTimeout)
	defer cancel()

	type result struct {
		section externalDataSection
		err     error
	}

	ch := make(chan result, len(sources))
	var wg sync.WaitGroup

	for _, src := range sources {
		wg.Add(1)
		go func(s externalDataSource) {
			defer wg.Done()
			body, err := httpGetExternal(fetchCtx, s.URL, s.Headers)
			if err != nil {
				ch <- result{err: fmt.Errorf("%s: %w", s.Name, err)}
				return
			}
			content, err := s.Parser(body)
			if err != nil {
				ch <- result{err: fmt.Errorf("%s parse: %w", s.Name, err)}
				return
			}
			if content == "" {
				ch <- result{err: fmt.Errorf("%s: empty content", s.Name)}
				return
			}
			ch <- result{section: externalDataSection{
				Source:  s.Name,
				Type:    inferDataType(s.Name),
				Content: content,
			}}
		}(src)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var sections []externalDataSection
	for r := range ch {
		if r.err != nil {
			if logger != nil {
				logger.Warn("external data fetch failed", "error", r.err)
			}
			continue
		}
		sections = append(sections, r.section)
	}

	if len(sections) == 0 {
		if logger != nil {
			logger.Info("external data: all sources failed, degrading gracefully", "symbol", symbol)
		}
		return nil
	}

	return &symbolExternalData{
		Symbol:      symbol,
		Market:      market,
		FetchedAt:   time.Now(),
		RawSections: sections,
	}
}

// summarizeExternalDataImpl uses AI to summarize the raw external data sections.
func summarizeExternalDataImpl(ctx context.Context, data *symbolExternalData, endpoint, apiKey, model string, logger *slog.Logger) string {
	if data == nil || len(data.RawSections) == 0 {
		return ""
	}

	rawText := buildRawSectionsText(data.RawSections)
	if rawText == "" {
		return ""
	}

	// Truncate to maxChars to stay within token budget.
	if len([]rune(rawText)) > externalDataMaxChars {
		rawText = string([]rune(rawText)[:externalDataMaxChars])
	}

	systemPrompt := `你是一个投资数据整理助手。你会收到多源原始数据（财报、新闻、研报）。
请优先按以下结构输出，并允许数据不足但必须明确缺口。

输出格式（纯文本，非JSON）：
【近5个季度财报】
- ...
【近3年年报】
- ...
【行业宏观政策】
- ...
【产业周期】
- ...
【公司最新经营】
- ...
【数据缺口】
- 缺口：...

硬要求：
- 不做投资建议，只提事实
- 缺数据时必须写“缺口：...”，不能省略
- 每条尽量短句，优先数字与时间
- 如果同一事实在多个来源出现，只保留最有信息密度的一条`

	userPrompt := fmt.Sprintf(`标的: %s
市场: %s
数据采集时间: %s

任务优先级：
1) 近5个季度财报
2) 近3年年报
3) 行业宏观政策
4) 产业周期
5) 公司最新经营

原始数据：
%s`,
		data.Symbol, data.Market, data.FetchedAt.Format("2006-01-02 15:04"), rawText)

	summarizeCtx, cancel := context.WithTimeout(ctx, externalDataSummarizeTimeout)
	defer cancel()

	result, err := aiChatCompletion(summarizeCtx, aiChatCompletionRequest{
		EndpointURL:  endpoint,
		APIKey:       apiKey,
		Model:        model,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Logger:       logger,
	})
	if err != nil {
		if logger != nil {
			logger.Warn("external data summarization failed", "symbol", data.Symbol, "error", err)
		}
		fallback := buildFallbackStructuredExternalSummary(data)
		data.StructuredSummary = fallback
		return fallback
	}

	normalized := normalizeStructuredExternalSummary(strings.TrimSpace(result.Content), data)
	if normalized == "" {
		normalized = buildFallbackStructuredExternalSummary(data)
	}
	data.StructuredSummary = normalized
	return normalized
}
