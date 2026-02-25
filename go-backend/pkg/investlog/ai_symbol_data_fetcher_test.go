package investlog

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestDetectMarket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		symbol   string
		currency string
		want     string
	}{
		{"A-share by prefix", "600519", "CNY", "cn"},
		{"A-share SH prefix", "SH600519", "CNY", "cn"},
		{"ETF", "510300", "CNY", "cn"},
		{"US stock", "AAPL", "USD", "us"},
		{"HK stock", "00700", "HKD", "hk"},
		{"Unknown", "CASH", "CNY", ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := detectMarket(tc.symbol, tc.currency)
			if got != tc.want {
				t.Fatalf("detectMarket(%q, %q) = %q, want %q", tc.symbol, tc.currency, got, tc.want)
			}
		})
	}
}

func TestBuildDataSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		market   string
		symbol   string
		currency string
		wantMin  int
	}{
		{"CN sources", "cn", "600519", "CNY", 3},
		{"US sources", "us", "AAPL", "USD", 2},
		{"HK sources", "hk", "00700", "HKD", 2},
		{"Unknown market", "xx", "XYZ", "EUR", 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sources := buildDataSources(tc.market, tc.symbol, tc.currency)
			if len(sources) < tc.wantMin {
				t.Fatalf("buildDataSources(%q, %q, %q) returned %d sources, want >= %d",
					tc.market, tc.symbol, tc.currency, len(sources), tc.wantMin)
			}
			for _, s := range sources {
				if s.Name == "" {
					t.Fatal("source has empty Name")
				}
				if s.URL == "" {
					t.Fatal("source has empty URL")
				}
				if s.Parser == nil {
					t.Fatalf("source %q has nil Parser", s.Name)
				}
			}
		})
	}
}

func TestBuildEastmoneySecID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code string
		want string
	}{
		{"600519", "1.600519"},
		{"510300", "1.510300"},
		{"000001", "0.000001"},
		{"300750", "0.300750"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()
			got := buildEastmoneySecID(tc.code)
			if got != tc.want {
				t.Fatalf("buildEastmoneySecID(%q) = %q, want %q", tc.code, got, tc.want)
			}
		})
	}
}

func TestParseEastmoneyNews(t *testing.T) {
	t.Parallel()

	// Normal response
	body := []byte(`{"Data":[{"Title":"Test news","Content":"<p>Summary text</p>","Date":"2024-01-15"}]}`)
	result, err := parseEastmoneyNews(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !contains([]string{result}, "[2024-01-15] Test news - Summary text") {
		t.Fatalf("unexpected result: %s", result)
	}

	// JSONP response
	jsonpBody := []byte(`callback({"Data":[{"Title":"JSONP news","Content":"content","Date":"2024-01-10"}]})`)
	result2, err := parseEastmoneyNews(jsonpBody)
	if err != nil {
		t.Fatalf("unexpected JSONP error: %v", err)
	}
	if result2 == "" {
		t.Fatal("expected non-empty JSONP result")
	}

	// Empty response
	emptyBody := []byte(`{"Data":[]}`)
	result3, _ := parseEastmoneyNews(emptyBody)
	if result3 != "" {
		t.Fatalf("expected empty result for empty data, got: %s", result3)
	}
}

func TestParseEastmoneyFinancials(t *testing.T) {
	t.Parallel()

	body := []byte(`{"data":{"f9":15.5,"f23":3.2,"f57":"600519","f58":"贵州茅台","f170":1.5}}`)
	result, err := parseEastmoneyFinancials(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Should contain some field labels
	for _, label := range []string{"PE(动)", "PB"} {
		found := false
		for _, line := range splitLines(result) {
			if len(line) > 0 && containsStr(line, label) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected result to contain %q, got: %s", label, result)
		}
	}

	// Nil data
	nilBody := []byte(`{"data":null}`)
	result2, _ := parseEastmoneyFinancials(nilBody)
	if result2 != "" {
		t.Fatalf("expected empty result for nil data, got: %s", result2)
	}
}

func TestParseEastmoneyResearch(t *testing.T) {
	t.Parallel()

	body := []byte(`{"data":[{"title":"Research report","stockName":"Test","orgSName":"Goldman","publishDate":"2024-01-15 10:00:00","emRatingName":"Buy"}]}`)
	result, err := parseEastmoneyResearch(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !containsStr(result, "Goldman") {
		t.Fatalf("expected result to contain Goldman, got: %s", result)
	}
	if !containsStr(result, "Buy") {
		t.Fatalf("expected result to contain Buy, got: %s", result)
	}
}

func TestParseYahooQuoteSummary(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"quoteSummary": {
			"result": [{
				"financialData": {
					"currentPrice": {"raw": 180.5, "fmt": "180.50"},
					"targetMeanPrice": {"raw": 200.0, "fmt": "200.00"},
					"recommendationKey": "buy",
					"returnOnEquity": {"raw": 0.35, "fmt": "35.00%"}
				},
				"defaultKeyStatistics": {
					"trailingPE": {"raw": 28.5, "fmt": "28.50"},
					"forwardPE": {"raw": 25.0, "fmt": "25.00"}
				},
				"recommendationTrend": {
					"trend": [{"period": "0m", "strongBuy": 10, "buy": 15, "hold": 5, "sell": 1, "strongSell": 0}]
				}
			}]
		}
	}`)

	result, err := parseYahooQuoteSummary(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !containsStr(result, "180.50") {
		t.Fatalf("expected result to contain price, got: %s", result)
	}
	if !containsStr(result, "Analyst Ratings") {
		t.Fatalf("expected result to contain analyst ratings, got: %s", result)
	}

	// Empty result
	emptyBody := []byte(`{"quoteSummary":{"result":[]}}`)
	result2, _ := parseYahooQuoteSummary(emptyBody)
	if result2 != "" {
		t.Fatalf("expected empty result, got: %s", result2)
	}
}

func TestParseYahooChartContext(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"chart": {
			"result": [{
				"meta": {
					"symbol": "AAPL",
					"regularMarketPrice": 180.5,
					"previousClose": 178.0,
					"fiftyTwoWeekHigh": 200.0,
					"fiftyTwoWeekLow": 140.0
				}
			}]
		}
	}`)

	result, err := parseYahooChartContext(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !containsStr(result, "180.50") {
		t.Fatalf("expected market price, got: %s", result)
	}
	if !containsStr(result, "Day Change") {
		t.Fatalf("expected day change, got: %s", result)
	}
}

func TestStripHTMLTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no tags", "hello world", "hello world"},
		{"simple tag", "<p>hello</p>", "hello"},
		{"nested tags", "<div><p>hello</p></div>", "hello"},
		{"empty", "", ""},
		{"tag with attrs", `<a href="url">link</a>`, "link"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := stripHTMLTags(tc.input)
			if got != tc.want {
				t.Fatalf("stripHTMLTags(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestInferDataType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{"Eastmoney News", "news"},
		{"Yahoo Finance Summary", "financials"},
		{"Eastmoney Research", "research"},
		{"Yahoo Finance HK Chart", "news"},
		{"Something Else", "news"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := inferDataType(tc.name)
			if got != tc.want {
				t.Fatalf("inferDataType(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestBuildRawSectionsText(t *testing.T) {
	t.Parallel()

	sections := []externalDataSection{
		{Source: "Source1", Type: "news", Content: "content1"},
		{Source: "Source2", Type: "financials", Content: "content2"},
	}

	result := buildRawSectionsText(sections)
	if !containsStr(result, "=== Source1 (news) ===") {
		t.Fatalf("expected source header, got: %s", result)
	}
	if !containsStr(result, "content1") {
		t.Fatalf("expected content1, got: %s", result)
	}
	if !containsStr(result, "=== Source2 (financials) ===") {
		t.Fatalf("expected source2 header, got: %s", result)
	}

	// Empty
	if buildRawSectionsText(nil) != "" {
		t.Fatal("expected empty for nil sections")
	}
}

func TestFetchExternalData_NilForUnknownMarket(t *testing.T) {
	t.Parallel()

	result := fetchExternalDataImpl(context.Background(), "CASH", "CNY", slog.Default())
	if result != nil {
		t.Fatal("expected nil for unknown market (CASH)")
	}
}

func TestSummarizeExternalData_NilData(t *testing.T) {
	t.Parallel()

	result := summarizeExternalDataImpl(context.Background(), nil, "http://example.com", "key", "model", slog.Default())
	if result != "" {
		t.Fatalf("expected empty for nil data, got: %s", result)
	}
}

func TestSummarizeExternalData_EmptySections(t *testing.T) {
	t.Parallel()

	data := &symbolExternalData{
		Symbol:      "AAPL",
		Market:      "us",
		RawSections: []externalDataSection{},
	}
	result := summarizeExternalDataImpl(context.Background(), data, "http://example.com", "key", "model", slog.Default())
	if result != "" {
		t.Fatalf("expected empty for empty sections, got: %s", result)
	}
}

func TestExtractYahooValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data map[string]any
		key  string
		want string
	}{
		{"wrapped fmt", map[string]any{"price": map[string]any{"raw": 180.5, "fmt": "180.50"}}, "price", "180.50"},
		{"wrapped raw only", map[string]any{"price": map[string]any{"raw": 180.5}}, "price", "180.5000"},
		{"string value", map[string]any{"rec": "buy"}, "rec", "buy"},
		{"missing key", map[string]any{"a": 1}, "b", ""},
		{"nil data", nil, "x", ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractYahooValue(tc.data, tc.key)
			if got != tc.want {
				t.Fatalf("extractYahooValue got %q, want %q", got, tc.want)
			}
		})
	}
}

// helpers

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		lines = append(lines, line)
	}
	return lines
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}
