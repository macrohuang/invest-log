package investlog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

func (pf *priceFetcher) eastmoneyFetchAShare(symbol string) (*float64, error) {
	code := normalizeSymbol(symbol)
	market := 1
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") {
		market = 0
		if strings.HasPrefix(code, "SH") {
			market = 1
		}
		code = code[2:]
	} else if strings.HasPrefix(code, "6") {
		market = 1
	}
	if !reSixDigit.MatchString(code) {
		return nil, nil
	}
	url := fmt.Sprintf("http://push2.eastmoney.com/api/qt/stock/get?secid=%d.%s&fields=f43&ut=fa5fd1943c7b386f172d6893dbfba10b", market, code)
	body, err := pf.httpGet(context.Background(), url, map[string]string{"User-Agent": "Mozilla/5.0", "Referer": "http://quote.eastmoney.com/"})
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	value, ok := data["f43"]
	if !ok {
		return nil, nil
	}
	price, err := parseFloat(value)
	if err != nil {
		return nil, err
	}
	if price > priceScaleThreshold {
		price = price / priceScaleFactor
	}
	return &price, nil
}

func (pf *priceFetcher) eastmoneyFetchFund(symbol string) (*float64, error) {
	code := normalizeSymbol(symbol)
	if !reSixDigit.MatchString(code) {
		return nil, nil
	}
	url := fmt.Sprintf("http://fundgz.1234567.com.cn/js/%s.js", code)
	body, err := pf.httpGet(context.Background(), url, map[string]string{"User-Agent": "Mozilla/5.0", "Referer": "http://fund.eastmoney.com/"})
	if err != nil {
		return nil, err
	}
	text := string(body)
	start := strings.Index(text, "(")
	end := strings.LastIndex(text, ")")
	if start == -1 || end == -1 || end <= start {
		return nil, nil
	}
	jsonText := text[start+1 : end]
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		return nil, err
	}
	value := data["gsz"]
	if value == nil {
		value = data["dwjz"]
	}
	price, err := parseFloat(value)
	if err != nil {
		return nil, err
	}
	return &price, nil
}

func (pf *priceFetcher) eastmoneyFetchFundPingzhong(symbol string) (*float64, error) {
	code := normalizeSymbol(symbol)
	if !reSixDigit.MatchString(code) {
		return nil, nil
	}
	url := fmt.Sprintf("http://fund.eastmoney.com/pingzhongdata/%s.js", code)
	body, err := pf.httpGet(context.Background(), url, map[string]string{"User-Agent": "Mozilla/5.0", "Referer": "http://fund.eastmoney.com/"})
	if err != nil {
		return nil, err
	}
	text := string(body)
	marker := "var Data_netWorthTrend ="
	idx := strings.Index(text, marker)
	if idx == -1 {
		return nil, nil
	}
	bracketStart := strings.Index(text[idx:], "[")
	bracketEnd := strings.Index(text[idx:], "];\n")
	if bracketStart == -1 || bracketEnd == -1 {
		bracketEnd = strings.Index(text[idx:], "];\r\n")
	}
	if bracketStart == -1 || bracketEnd == -1 {
		return nil, nil
	}
	start := idx + bracketStart
	end := idx + bracketEnd + 1
	raw := text[start:end]
	var data []any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	last := data[len(data)-1]
	switch val := last.(type) {
	case map[string]any:
		price, err := parseFloat(val["y"])
		if err != nil {
			return nil, err
		}
		return &price, nil
	case []any:
		if len(val) >= 2 {
			price, err := parseFloat(val[1])
			if err != nil {
				return nil, err
			}
			return &price, nil
		}
	}
	return nil, nil
}

func (pf *priceFetcher) eastmoneyFetchFundLsjz(symbol string) (*float64, error) {
	code := normalizeSymbol(symbol)
	if !reSixDigit.MatchString(code) {
		return nil, nil
	}
	url := fmt.Sprintf("http://fund.eastmoney.com/f10/F10DataApi.aspx?type=lsjz&code=%s&page=1&per=1", code)
	body, err := pf.httpGet(context.Background(), url, map[string]string{"User-Agent": "Mozilla/5.0", "Referer": "http://fund.eastmoney.com/"})
	if err != nil {
		return nil, err
	}
	matches := reFundLsjzTD.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return nil, nil
	}
	price, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return nil, err
	}
	return &price, nil
}

func (pf *priceFetcher) yahooFetchStock(symbol, currency string) (*float64, error) {
	yahooSymbols := buildYahooSymbolCandidates(symbol, currency)
	if len(yahooSymbols) == 0 {
		return nil, nil
	}

	var lastErr error
	for _, yahooSymbol := range yahooSymbols {
		price, err := pf.yahooFetchStockByYahooSymbol(yahooSymbol)
		if err != nil {
			lastErr = err
			continue
		}
		if price != nil {
			return price, nil
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, nil
}

func (pf *priceFetcher) yahooFetchStockByYahooSymbol(yahooSymbol string) (*float64, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1d&range=1d", yahooSymbol)
	body, err := pf.httpGet(context.Background(), url, map[string]string{"User-Agent": "Mozilla/5.0"})
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	chart, _ := payload["chart"].(map[string]any)
	results, _ := chart["result"].([]any)
	if len(results) == 0 {
		return nil, nil
	}
	result, _ := results[0].(map[string]any)
	meta, _ := result["meta"].(map[string]any)
	if meta != nil {
		if price, err := parseFloat(meta["regularMarketPrice"]); err == nil {
			if price > 0 {
				return &price, nil
			}
		}
	}
	indicators, _ := result["indicators"].(map[string]any)
	quoteArr, _ := indicators["quote"].([]any)
	if len(quoteArr) == 0 {
		return nil, nil
	}
	quote, _ := quoteArr[0].(map[string]any)
	closes, _ := quote["close"].([]any)
	if len(closes) == 0 {
		return nil, nil
	}
	price, err := parseFloat(closes[len(closes)-1])
	if err != nil {
		return nil, err
	}
	if price <= 0 {
		return nil, nil
	}
	return &price, nil
}

func buildYahooSymbolCandidates(symbol, currency string) []string {
	primary := buildYahooSymbol(symbol, currency)
	if primary == "" {
		return nil
	}

	candidates := []string{primary}
	code := normalizeSymbol(symbol)
	currency = normalizeCurrency(currency)

	if currency == "USD" && shouldTryLSEYahooFallback(code, primary) {
		candidates = append(candidates, code+".L")
	}

	return candidates
}

func shouldTryLSEYahooFallback(symbol, primaryYahooSymbol string) bool {
	if symbol == "" || primaryYahooSymbol == "" {
		return false
	}
	if strings.Contains(symbol, ".") || strings.Contains(symbol, "=") {
		return false
	}
	if strings.HasSuffix(primaryYahooSymbol, ".L") {
		return false
	}
	for _, ch := range symbol {
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		return false
	}
	return true
}

func buildYahooSymbol(symbol, currency string) string {
	code := normalizeSymbol(symbol)
	currency = normalizeCurrency(currency)
	if currency == "CNY" {
		if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") {
			code = code[2:]
		}
		if strings.HasPrefix(code, "6") {
			return code + ".SS"
		}
		if reSixDigit.MatchString(code) {
			return code + ".SZ"
		}
	}
	if currency == "HKD" {
		code = strings.TrimPrefix(code, "HK")
		if len(code) < 4 {
			code = strings.Repeat("0", 4-len(code)) + code
		}
		return code + ".HK"
	}
	if currency == "USD" {
		return code
	}
	return code
}

func (pf *priceFetcher) yahooFetchGold() (*float64, error) {
	price, err := pf.yahooFetchStock("GC=F", "USD")
	if err != nil || price == nil {
		return nil, err
	}
	pricePerOz := *price
	if pricePerOz <= 0 {
		return nil, nil
	}
	// Convert to CNY per gram using configured exchange rate.
	converted := pricePerOz / ouncesToGrams * pf.usdToCNYRate
	converted = math.Round(converted*100) / 100
	return &converted, nil
}

// Sina Finance APIs.
func (pf *priceFetcher) sinaFetchAShare(symbol string) (*float64, error) {
	code := normalizeSymbol(symbol)
	prefix := "sz"
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") {
		prefix = strings.ToLower(code[:2])
		code = code[2:]
	} else if strings.HasPrefix(code, "6") {
		prefix = "sh"
	}
	url := fmt.Sprintf("http://hq.sinajs.cn/list=%s%s", prefix, code)
	body, err := pf.httpGet(context.Background(), url, map[string]string{"Referer": "http://finance.sina.com.cn"})
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(body), "=\"", 2)
	if len(parts) < 2 {
		return nil, nil
	}
	data := strings.Split(parts[1], ",")
	if len(data) > 3 {
		price, err := strconv.ParseFloat(data[3], 64)
		if err == nil {
			return &price, nil
		}
	}
	return nil, nil
}

func (pf *priceFetcher) sinaFetchHKStock(symbol string) (*float64, error) {
	code := normalizeSymbol(symbol)
	if len(code) < 5 {
		code = strings.Repeat("0", 5-len(code)) + code
	}
	url := fmt.Sprintf("http://hq.sinajs.cn/list=hk%s", code)
	body, err := pf.httpGet(context.Background(), url, map[string]string{"Referer": "http://finance.sina.com.cn"})
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(body), "=\"", 2)
	if len(parts) < 2 {
		return nil, nil
	}
	data := strings.Split(parts[1], ",")
	if len(data) > 6 {
		price, err := strconv.ParseFloat(data[6], 64)
		if err == nil {
			return &price, nil
		}
	}
	return nil, nil
}

func (pf *priceFetcher) sinaFetchUSStock(symbol string) (*float64, error) {
	code := strings.ToLower(symbol)
	url := fmt.Sprintf("http://hq.sinajs.cn/list=gb_%s", code)
	body, err := pf.httpGet(context.Background(), url, map[string]string{"Referer": "http://finance.sina.com.cn"})
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(body), "=\"", 2)
	if len(parts) < 2 {
		return nil, nil
	}
	data := strings.Split(parts[1], ",")
	if len(data) > 1 {
		price, err := strconv.ParseFloat(data[1], 64)
		if err == nil {
			return &price, nil
		}
	}
	return nil, nil
}

// Tencent Finance APIs.
func (pf *priceFetcher) tencentFetchAShare(symbol string) (*float64, error) {
	code := normalizeSymbol(symbol)
	prefix := "sz"
	if strings.HasPrefix(code, "SH") || strings.HasPrefix(code, "SZ") {
		prefix = strings.ToLower(code[:2])
		code = code[2:]
	} else if strings.HasPrefix(code, "6") {
		prefix = "sh"
	}
	url := fmt.Sprintf("http://qt.gtimg.cn/q=%s%s", prefix, code)
	body, err := pf.httpGet(context.Background(), url, nil)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(body), "~")
	if len(parts) > 3 {
		price, err := strconv.ParseFloat(parts[3], 64)
		if err == nil {
			return &price, nil
		}
	}
	return nil, nil
}

func (pf *priceFetcher) tencentFetchHKStock(symbol string) (*float64, error) {
	code := normalizeSymbol(symbol)
	if len(code) < 5 {
		code = strings.Repeat("0", 5-len(code)) + code
	}
	url := fmt.Sprintf("http://qt.gtimg.cn/q=hk%s", code)
	body, err := pf.httpGet(context.Background(), url, nil)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(body), "~")
	if len(parts) > 3 {
		price, err := strconv.ParseFloat(parts[3], 64)
		if err == nil {
			return &price, nil
		}
	}
	return nil, nil
}

func (pf *priceFetcher) tencentFetchUSStock(symbol string) (*float64, error) {
	code := normalizeSymbol(symbol)
	url := fmt.Sprintf("http://qt.gtimg.cn/q=us%s", code)
	body, err := pf.httpGet(context.Background(), url, nil)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(body), "~")
	if len(parts) > 3 {
		price, err := strconv.ParseFloat(parts[3], 64)
		if err == nil {
			return &price, nil
		}
	}
	return nil, nil
}

// hkConnectToHKCode strips the H prefix from a Stock Connect symbol to get the HK code.
// e.g. "H00700" -> "00700"
func hkConnectToHKCode(symbol string) string {
	if len(symbol) > 1 && (symbol[0] == 'H' || symbol[0] == 'h') {
		return symbol[1:]
	}
	return symbol
}

// convertHKDToCNY wraps a fetch function that returns HKD prices, converting to CNY.
func (pf *priceFetcher) convertHKDToCNY(fetchFn func() (*float64, error)) (*float64, error) {
	hkdPrice, err := fetchFn()
	if err != nil {
		return nil, err
	}
	if hkdPrice == nil {
		return nil, nil
	}

	rate := defaultHKDToCNYRate
	if pf.rateResolver != nil {
		if r, err := pf.rateResolver("HKD"); err == nil {
			rate = r
		}
	}
	cnyPrice := *hkdPrice * rate
	return &cnyPrice, nil
}

// eastmoneyFetchHKConnect fetches HK stock price via Eastmoney's Stock Connect endpoint.
// Returns price in HKD (caller must convert to CNY).
func (pf *priceFetcher) eastmoneyFetchHKConnect(hkCode string) (*float64, error) {
	url := fmt.Sprintf(
		"http://push2.eastmoney.com/api/qt/stock/get?secid=128.%s&fields=f43&ut=fa5fd1943c7b386f172d6893dbfba10b",
		hkCode,
	)
	body, err := pf.httpGet(context.Background(), url, map[string]string{
		"User-Agent": "Mozilla/5.0",
		"Referer":    "http://quote.eastmoney.com/",
	})
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	value, ok := data["f43"]
	if !ok {
		return nil, nil
	}
	price, err := parseFloat(value)
	if err != nil {
		return nil, err
	}
	// Eastmoney HK Connect f43 returns price * 1000 (e.g. 565000 = 565.000 HKD)
	price = price / 1000.0
	return &price, nil
}

// maxResponseSize limits external API responses to 1MB to prevent memory exhaustion.
const maxResponseSize = 1 << 20 // 1MB

func (pf *priceFetcher) httpGet(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := pf.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	// Limit response size to prevent memory exhaustion from malicious/buggy external APIs
	return io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
}

func parseFloat(value any) (float64, error) {
	switch v := value.(type) {
	case nil:
		return 0, errors.New("no value")
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	case string:
		if v == "" {
			return 0, errors.New("empty")
		}
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}
