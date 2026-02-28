package investlog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// externalDataSection represents one chunk of fetched external data.
type externalDataSection struct {
	Source  string `json:"source"`
	Type    string `json:"type"` // "news", "financials", "research"
	Content string `json:"content"`
}

// symbolExternalData holds all fetched external data for a symbol.
type symbolExternalData struct {
	Symbol            string
	Market            string
	FetchedAt         time.Time
	RawSections       []externalDataSection
	Summary           string
	StructuredSummary string
}

// externalDataSource defines a single data source to fetch.
type externalDataSource struct {
	Name    string
	URL     string
	Headers map[string]string
	Parser  func(body []byte) (string, error)
}

const (
	externalDataFetchTimeout     = 30 * time.Second
	externalDataSummarizeTimeout = 30 * time.Second
	externalDataMaxChars         = 8000
)

type externalSummarySectionSpec struct {
	Header  string
	GapNote string
}

var externalSummarySectionSpecs = []externalSummarySectionSpec{
	{Header: "近5个季度财报", GapNote: "未抓取到近5个季度财报"},
	{Header: "近3年年报", GapNote: "未抓取到近3年年报"},
	{Header: "行业宏观政策", GapNote: "未抓取到行业宏观政策"},
	{Header: "产业周期", GapNote: "未抓取到产业周期信息"},
	{Header: "公司最新经营", GapNote: "未抓取到公司最新经营进展"},
}

// Function variables for testing/mocking.
var fetchExternalDataFn = fetchExternalDataImpl
var summarizeExternalDataFn = summarizeExternalDataImpl

// detectMarket maps a symbol+currency to a market string using the existing detectSymbolType.
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

func normalizeStructuredExternalSummary(summary string, data *symbolExternalData) string {
	normalized := strings.TrimSpace(summary)
	if normalized == "" {
		return buildFallbackStructuredExternalSummary(data)
	}

	missingSections := make([]string, 0)
	for _, spec := range externalSummarySectionSpecs {
		header := fmt.Sprintf("【%s】", spec.Header)
		if strings.Contains(normalized, header) {
			continue
		}
		normalized += fmt.Sprintf("\n\n%s\n- 缺口：%s", header, spec.GapNote)
		missingSections = append(missingSections, spec.Header)
	}

	if !strings.Contains(normalized, "【数据缺口】") {
		normalized += "\n\n【数据缺口】"
		if len(missingSections) == 0 {
			normalized += "\n- 无新增结构化缺口（仍需后续刷新）"
		} else {
			for _, section := range missingSections {
				normalized += fmt.Sprintf("\n- 缺口：%s数据不足", section)
			}
		}
	}

	return strings.TrimSpace(normalized)
}

func buildFallbackStructuredExternalSummary(data *symbolExternalData) string {
	if data == nil {
		return buildAllGapSummary()
	}
	lines := flattenExternalDataLines(data.RawSections)
	if len(lines) == 0 {
		return buildAllGapSummary()
	}

	quarterLines := pickEvidenceLines(lines, []string{"q1", "q2", "q3", "q4", "季度", "季报"}, 3)
	annualLines := pickEvidenceLines(lines, []string{"年报", "年度", "fy", "annual"}, 3)
	policyLines := pickEvidenceLines(lines, []string{"政策", "监管", "央行", "利率", "财政", "补贴", "关税"}, 3)
	cycleLines := pickEvidenceLines(lines, []string{"周期", "景气", "库存", "产能", "供需", "cycle"}, 3)
	operationLines := pickEvidenceLines(lines, []string{"营收", "净利润", "订单", "指引", "回购", "并购", "产线", "产品", "经营"}, 3)

	var builder strings.Builder
	missing := make([]string, 0)

	writeSummarySection(&builder, "近5个季度财报", quarterLines, "未抓取到近5个季度财报", &missing)
	writeSummarySection(&builder, "近3年年报", annualLines, "未抓取到近3年年报", &missing)
	writeSummarySection(&builder, "行业宏观政策", policyLines, "未抓取到行业宏观政策", &missing)
	writeSummarySection(&builder, "产业周期", cycleLines, "未抓取到产业周期信息", &missing)
	writeSummarySection(&builder, "公司最新经营", operationLines, "未抓取到公司最新经营进展", &missing)

	builder.WriteString("\n\n【数据缺口】")
	if len(missing) == 0 {
		builder.WriteString("\n- 无明确结构化缺口（建议继续刷新）")
	} else {
		for _, section := range missing {
			builder.WriteString(fmt.Sprintf("\n- 缺口：%s", section))
		}
	}
	return strings.TrimSpace(builder.String())
}

func writeSummarySection(builder *strings.Builder, header string, lines []string, gapNote string, missing *[]string) {
	if builder.Len() > 0 {
		builder.WriteString("\n\n")
	}
	builder.WriteString(fmt.Sprintf("【%s】", header))
	if len(lines) == 0 {
		builder.WriteString(fmt.Sprintf("\n- 缺口：%s", gapNote))
		*missing = append(*missing, header)
		return
	}
	for _, line := range lines {
		builder.WriteString("\n- ")
		builder.WriteString(line)
	}
}

func buildAllGapSummary() string {
	var builder strings.Builder
	for idx, spec := range externalSummarySectionSpecs {
		if idx > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(fmt.Sprintf("【%s】\n- 缺口：%s", spec.Header, spec.GapNote))
	}
	builder.WriteString("\n\n【数据缺口】")
	for _, spec := range externalSummarySectionSpecs {
		builder.WriteString(fmt.Sprintf("\n- 缺口：%s数据不足", spec.Header))
	}
	return builder.String()
}

func flattenExternalDataLines(sections []externalDataSection) []string {
	lines := make([]string, 0, len(sections)*3)
	for _, section := range sections {
		source := strings.TrimSpace(section.Source)
		for _, rawLine := range strings.Split(section.Content, "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" {
				continue
			}
			if len([]rune(line)) > 80 {
				line = string([]rune(line)[:80]) + "..."
			}
			if source != "" {
				line = fmt.Sprintf("%s: %s", source, line)
			}
			lines = append(lines, line)
			if len(lines) >= 120 {
				return lines
			}
		}
	}
	return lines
}

func pickEvidenceLines(lines []string, keywords []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	result := make([]string, 0, limit)
	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		if len(keywords) > 0 && !containsAnyLowerKeyword(lowerLine, keywords) {
			continue
		}
		result = append(result, line)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func containsAnyLowerKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// buildDataSources returns the data sources for the given market.
func buildDataSources(market, symbol, currency string) []externalDataSource {
	switch market {
	case "cn":
		return buildCNDataSources(symbol)
	case "us":
		return buildUSDataSources(symbol)
	case "hk":
		return buildHKDataSources(symbol, currency)
	default:
		return nil
	}
}

// ---------- Chinese A-share data sources ----------

func buildCNDataSources(symbol string) []externalDataSource {
	code := normalizeSymbol(symbol)
	// Strip SH/SZ prefix for API calls
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") {
		code = code[2:]
	}

	return []externalDataSource{
		{
			Name: "Eastmoney News",
			URL: fmt.Sprintf(
				"https://search-api-web.eastmoney.com/search/jsonp?cb=&type=14&pageindex=1&pagesize=10&keyword=%s&name=zixun",
				code,
			),
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0",
				"Referer":    "https://so.eastmoney.com/",
			},
			Parser: parseEastmoneyNews,
		},
		{
			Name: "Eastmoney Financials",
			URL: fmt.Sprintf(
				"https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f9,f23,f37,f40,f45,f49,f57,f58,f162,f167,f170&ut=fa5fd1943c7b386f172d6893dbfba10b",
				buildEastmoneySecID(code),
			),
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0",
				"Referer":    "https://quote.eastmoney.com/",
			},
			Parser: parseEastmoneyFinancials,
		},
		{
			Name: "Eastmoney Research",
			URL: fmt.Sprintf(
				"https://reportapi.eastmoney.com/report/list?industryCode=&pageNo=1&pageSize=5&ticker=%s&queryText=&beginTime=&endTime=&qType=0",
				code,
			),
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0",
				"Referer":    "https://data.eastmoney.com/",
			},
			Parser: parseEastmoneyResearch,
		},
	}
}

func buildEastmoneySecID(code string) string {
	if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "5") {
		return "1." + code
	}
	return "0." + code
}

func parseEastmoneyNews(body []byte) (string, error) {
	text := string(body)
	// Response may be JSONP wrapped: callback(...)
	if idx := strings.Index(text, "("); idx >= 0 {
		end := strings.LastIndex(text, ")")
		if end > idx {
			text = text[idx+1 : end]
		}
	}

	var payload struct {
		Data []struct {
			Title   string `json:"Title"`
			Content string `json:"Content"`
			Date    string `json:"Date"`
		} `json:"Data"`
	}
	// Try alternate structure
	var altPayload struct {
		Result struct {
			Data []struct {
				Title   string `json:"title"`
				Content string `json:"content"`
				Date    string `json:"date"`
			} `json:"data"`
		} `json:"result"`
	}

	var lines []string
	if err := json.Unmarshal([]byte(text), &payload); err == nil && len(payload.Data) > 0 {
		for _, item := range payload.Data {
			title := strings.TrimSpace(item.Title)
			if title == "" {
				continue
			}
			// Strip HTML tags from content
			summary := stripHTMLTags(item.Content)
			if len([]rune(summary)) > 80 {
				summary = string([]rune(summary)[:80]) + "..."
			}
			line := title
			if summary != "" {
				line += " - " + summary
			}
			if item.Date != "" {
				line = "[" + item.Date + "] " + line
			}
			lines = append(lines, line)
		}
	} else if err := json.Unmarshal([]byte(text), &altPayload); err == nil && len(altPayload.Result.Data) > 0 {
		for _, item := range altPayload.Result.Data {
			title := strings.TrimSpace(item.Title)
			if title == "" {
				continue
			}
			summary := stripHTMLTags(item.Content)
			if len([]rune(summary)) > 80 {
				summary = string([]rune(summary)[:80]) + "..."
			}
			line := title
			if summary != "" {
				line += " - " + summary
			}
			if item.Date != "" {
				line = "[" + item.Date + "] " + line
			}
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n"), nil
}

func parseEastmoneyFinancials(body []byte) (string, error) {
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Data == nil {
		return "", nil
	}

	fieldNames := map[string]string{
		"f9":   "PE(动)",
		"f23":  "PB",
		"f37":  "ROE(%)",
		"f40":  "营收(元)",
		"f45":  "净利润(元)",
		"f49":  "每股收益",
		"f57":  "代码",
		"f58":  "名称",
		"f162": "PE(静)",
		"f167": "市净率",
		"f170": "涨跌幅(%)",
	}

	var lines []string
	for key, label := range fieldNames {
		val, ok := payload.Data[key]
		if !ok || val == nil {
			continue
		}
		// Eastmoney sometimes returns "-" for unavailable fields
		s := fmt.Sprintf("%v", val)
		if s == "-" || s == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, s))
	}

	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n"), nil
}

func parseEastmoneyResearch(body []byte) (string, error) {
	var payload struct {
		Data []struct {
			Title        string `json:"title"`
			StockName    string `json:"stockName"`
			OrgSName     string `json:"orgSName"`
			PublishDate  string `json:"publishDate"`
			EmRatingName string `json:"emRatingName"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if len(payload.Data) == 0 {
		return "", nil
	}

	var lines []string
	for _, item := range payload.Data {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		line := title
		if item.OrgSName != "" {
			line += " (" + item.OrgSName + ")"
		}
		if item.EmRatingName != "" {
			line += " [" + item.EmRatingName + "]"
		}
		if item.PublishDate != "" {
			date := item.PublishDate
			if len(date) > 10 {
				date = date[:10]
			}
			line = "[" + date + "] " + line
		}
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n"), nil
}

// ---------- US stock data sources ----------

func buildUSDataSources(symbol string) []externalDataSource {
	code := normalizeSymbol(symbol)
	yahooSymbol := code // US stocks use ticker directly

	return []externalDataSource{
		{
			Name: "Yahoo Finance Summary",
			URL: fmt.Sprintf(
				"https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=financialData,defaultKeyStatistics,recommendationTrend",
				yahooSymbol,
			),
			Headers: map[string]string{"User-Agent": "Mozilla/5.0"},
			Parser:  parseYahooQuoteSummary,
		},
		{
			Name: "Yahoo Finance News",
			URL: fmt.Sprintf(
				"https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=5d&includePrePost=false",
				yahooSymbol,
			),
			Headers: map[string]string{"User-Agent": "Mozilla/5.0"},
			Parser:  parseYahooChartContext,
		},
	}
}

func parseYahooQuoteSummary(body []byte) (string, error) {
	var payload struct {
		QuoteSummary struct {
			Result []struct {
				FinancialData        map[string]any `json:"financialData"`
				DefaultKeyStatistics map[string]any `json:"defaultKeyStatistics"`
				RecommendationTrend  struct {
					Trend []struct {
						Period     string `json:"period"`
						StrongBuy  int    `json:"strongBuy"`
						Buy        int    `json:"buy"`
						Hold       int    `json:"hold"`
						Sell       int    `json:"sell"`
						StrongSell int    `json:"strongSell"`
					} `json:"trend"`
				} `json:"recommendationTrend"`
			} `json:"result"`
		} `json:"quoteSummary"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if len(payload.QuoteSummary.Result) == 0 {
		return "", nil
	}

	r := payload.QuoteSummary.Result[0]
	var lines []string

	// Financial data
	financialFields := []struct{ key, label string }{
		{"currentPrice", "Current Price"},
		{"targetHighPrice", "Target High"},
		{"targetLowPrice", "Target Low"},
		{"targetMeanPrice", "Target Mean"},
		{"recommendationKey", "Recommendation"},
		{"totalRevenue", "Total Revenue"},
		{"revenueGrowth", "Revenue Growth"},
		{"grossMargins", "Gross Margin"},
		{"operatingMargins", "Operating Margin"},
		{"profitMargins", "Profit Margin"},
		{"returnOnEquity", "ROE"},
		{"debtToEquity", "Debt/Equity"},
		{"freeCashflow", "Free Cashflow"},
	}
	for _, f := range financialFields {
		val := extractYahooValue(r.FinancialData, f.key)
		if val != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", f.label, val))
		}
	}

	// Key statistics
	statsFields := []struct{ key, label string }{
		{"trailingPE", "Trailing PE"},
		{"forwardPE", "Forward PE"},
		{"priceToBook", "P/B"},
		{"pegRatio", "PEG"},
		{"enterpriseToRevenue", "EV/Revenue"},
		{"enterpriseToEbitda", "EV/EBITDA"},
		{"beta", "Beta"},
		{"52WeekChange", "52W Change"},
	}
	for _, f := range statsFields {
		val := extractYahooValue(r.DefaultKeyStatistics, f.key)
		if val != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", f.label, val))
		}
	}

	// Recommendation trend
	if len(r.RecommendationTrend.Trend) > 0 {
		t := r.RecommendationTrend.Trend[0]
		lines = append(lines, fmt.Sprintf("Analyst Ratings (%s): StrongBuy=%d Buy=%d Hold=%d Sell=%d StrongSell=%d",
			t.Period, t.StrongBuy, t.Buy, t.Hold, t.Sell, t.StrongSell))
	}

	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n"), nil
}

func parseYahooChartContext(body []byte) (string, error) {
	var payload struct {
		Chart struct {
			Result []struct {
				Meta struct {
					Symbol             string  `json:"symbol"`
					RegularMarketPrice float64 `json:"regularMarketPrice"`
					PreviousClose      float64 `json:"previousClose"`
					FiftyTwoWeekHigh   float64 `json:"fiftyTwoWeekHigh"`
					FiftyTwoWeekLow    float64 `json:"fiftyTwoWeekLow"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if len(payload.Chart.Result) == 0 {
		return "", nil
	}

	m := payload.Chart.Result[0].Meta
	var lines []string
	if m.RegularMarketPrice > 0 {
		lines = append(lines, fmt.Sprintf("Market Price: %.2f", m.RegularMarketPrice))
	}
	if m.PreviousClose > 0 {
		lines = append(lines, fmt.Sprintf("Previous Close: %.2f", m.PreviousClose))
	}
	if m.FiftyTwoWeekHigh > 0 {
		lines = append(lines, fmt.Sprintf("52W High: %.2f", m.FiftyTwoWeekHigh))
	}
	if m.FiftyTwoWeekLow > 0 {
		lines = append(lines, fmt.Sprintf("52W Low: %.2f", m.FiftyTwoWeekLow))
	}
	if m.RegularMarketPrice > 0 && m.PreviousClose > 0 {
		change := (m.RegularMarketPrice - m.PreviousClose) / m.PreviousClose * 100
		lines = append(lines, fmt.Sprintf("Day Change: %.2f%%", change))
	}

	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n"), nil
}

// ---------- Hong Kong stock data sources ----------

func buildHKDataSources(symbol, currency string) []externalDataSource {
	code := normalizeSymbol(symbol)
	// Handle hk_connect (H-prefixed) symbols
	if strings.HasPrefix(code, "H") && len(code) > 1 {
		code = code[1:]
	}

	// Build Yahoo HK symbol
	yahooCode := code
	if len(yahooCode) < 4 {
		yahooCode = strings.Repeat("0", 4-len(yahooCode)) + yahooCode
	}
	yahooSymbol := yahooCode + ".HK"

	return []externalDataSource{
		{
			Name: "Yahoo Finance HK Summary",
			URL: fmt.Sprintf(
				"https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=financialData,defaultKeyStatistics,recommendationTrend",
				yahooSymbol,
			),
			Headers: map[string]string{"User-Agent": "Mozilla/5.0"},
			Parser:  parseYahooQuoteSummary,
		},
		{
			Name: "Yahoo Finance HK Chart",
			URL: fmt.Sprintf(
				"https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=5d&includePrePost=false",
				yahooSymbol,
			),
			Headers: map[string]string{"User-Agent": "Mozilla/5.0"},
			Parser:  parseYahooChartContext,
		},
	}
}

// ---------- Utility functions ----------

func extractYahooValue(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	val, ok := data[key]
	if !ok || val == nil {
		return ""
	}
	// Yahoo wraps numeric values in {"raw": 123, "fmt": "123"}
	if m, ok := val.(map[string]any); ok {
		if fmt, ok := m["fmt"].(string); ok && fmt != "" {
			return fmt
		}
		if raw, ok := m["raw"]; ok {
			return formatAny(raw)
		}
	}
	return formatAny(val)
}

func formatAny(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%.0f", val)
		}
		return fmt.Sprintf("%.4f", val)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

func inferDataType(sourceName string) string {
	lower := strings.ToLower(sourceName)
	switch {
	case strings.Contains(lower, "news") || strings.Contains(lower, "chart"):
		return "news"
	case strings.Contains(lower, "financial") || strings.Contains(lower, "summary"):
		return "financials"
	case strings.Contains(lower, "research"):
		return "research"
	default:
		return "news"
	}
}

func buildRawSectionsText(sections []externalDataSection) string {
	if len(sections) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, s := range sections {
		sb.WriteString(fmt.Sprintf("=== %s (%s) ===\n", s.Source, s.Type))
		sb.WriteString(s.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// httpGetExternal performs an HTTP GET with the given headers.
// It limits response size to 1MB.
func httpGetExternal(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
