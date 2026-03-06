package investlog

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
