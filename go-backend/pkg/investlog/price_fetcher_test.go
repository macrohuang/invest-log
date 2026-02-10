package investlog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

// mockHTTPClient implements HTTPDoer for testing.
type mockHTTPClient struct {
	status int
	body   string
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}, nil
}

func newFetcherWithBody(status int, body string) *priceFetcher {
	return newPriceFetcher(priceFetcherOptions{
		CacheTTL:      time.Second,
		FailThreshold: 2,
		FailWindow:    time.Second,
		Cooldown:      time.Second,
		HTTPTimeout:   time.Second,
		HTTPClient:    &mockHTTPClient{status: status, body: body},
	})
}

func TestPriceFetcherCacheAndServiceState(t *testing.T) {
	pf := newPriceFetcher(priceFetcherOptions{
		CacheTTL:      time.Second,
		FailThreshold: 2,
		FailWindow:    time.Second,
		Cooldown:      time.Hour,
		HTTPTimeout:   time.Second,
	})

	pf.setCached("AAPL", "USD", "stock", 123.45, "Test")
	if price, source, ok := pf.getCached("AAPL", "USD", "stock"); !ok || source != "Test" || price != 123.45 {
		t.Fatalf("expected cached price")
	}

	key := cacheKey("AAPL", "USD", "stock")
	pf.cache[key] = cacheEntry{price: 1.23, source: "Test", ts: time.Now().Add(-2 * time.Second)}
	if _, _, ok := pf.getCached("AAPL", "USD", "stock"); ok {
		t.Fatalf("expected cache miss after expiry")
	}

	pf.recordServiceFailure("svc")
	pf.recordServiceFailure("svc")
	if pf.serviceAvailable("svc") {
		t.Fatalf("expected service to be in cooldown")
	}
	pf.recordServiceSuccess("svc")
	if !pf.serviceAvailable("svc") {
		t.Fatalf("expected service to be available after success")
	}

	// Cover fail window reset.
	pf.serviceState["svc"] = &serviceState{failCount: 5, firstFailAt: time.Now().Add(-2 * time.Second)}
	pf.recordServiceFailure("svc")
	if pf.serviceState["svc"].failCount != 1 {
		t.Fatalf("expected failCount reset, got %d", pf.serviceState["svc"].failCount)
	}
}

func TestBuildAttemptsInvoke(t *testing.T) {
	pf := newFetcherWithBody(http.StatusOK, "")
	cases := []struct {
		symbolType string
		symbol     string
		currency   string
	}{
		{"a_share", "600000", "CNY"},
		{"fund", "110001", "CNY"},
		{"etf", "510300", "CNY"},
		{"hk_connect", "H00700", "CNY"},
		{"hk_stock", "00001", "HKD"},
		{"us_stock", "AAPL", "USD"},
		{"gold", "AU9999", "CNY"},
	}
	for _, c := range cases {
		attempts := pf.buildAttempts(c.symbolType, c.symbol, c.currency, "")
		for _, attempt := range attempts {
			_, _ = attempt.fn()
		}
	}
}

func TestPriceFetcherFetchModes(t *testing.T) {
	pf := newPriceFetcher(priceFetcherOptions{CacheTTL: time.Second})

	price, msg, err := pf.fetch("CASH", "USD", "")
	if err != nil || price == nil || *price != 1.0 {
		t.Fatalf("cash fetch failed: %v %v", price, err)
	}
	if msg == "" {
		t.Fatalf("expected message")
	}

	price, _, err = pf.fetch("BOND123", "CNY", "")
	if err == nil || price != nil {
		t.Fatalf("expected bond fetch to error")
	}

	price, _, err = pf.fetch("???", "CNY", "")
	if err == nil || price != nil {
		t.Fatalf("expected unknown fetch to error")
	}

	pf.setCached("AAPL", "USD", "stock", 9.99, "Cache")
	price, msg, err = pf.fetch("AAPL", "USD", "stock")
	if err != nil || price == nil || *price != 9.99 {
		t.Fatalf("expected cached fetch")
	}
	if !strings.Contains(msg, "缓存") {
		t.Fatalf("expected cache message")
	}
}

func TestPriceFetcherFetchSuccessAndCooldown(t *testing.T) {
	pf := newFetcherWithBody(http.StatusOK, `{"chart":{"result":[{"meta":{"regularMarketPrice":123.45}}]}}`)
	price, msg, err := pf.fetch("AAPL", "USD", "stock")
	if err != nil || price == nil || *price != 123.45 {
		t.Fatalf("fetch success: %v %v", price, err)
	}
	if !strings.Contains(msg, "价格获取成功") {
		t.Fatalf("expected success message")
	}
	if cached, _, ok := pf.getCached("AAPL", "USD", "stock"); !ok || cached != 123.45 {
		t.Fatalf("expected cached price after success")
	}

	// Force Yahoo into cooldown to cover serviceUnavailable branch.
	pf = newFetcherWithBody(http.StatusOK, "")
	pf.serviceState["Yahoo Finance"] = &serviceState{cooldownUntil: time.Now().Add(10 * time.Second)}
	if _, msg, err := pf.fetch("AAPL", "USD", "stock"); err == nil {
		t.Fatalf("expected error when all services fail")
	} else if !strings.Contains(msg, "熔断冷却中") {
		t.Fatalf("expected cooldown message, got %q", msg)
	}
}

func TestBuildAttemptsAndDetectSymbolType(t *testing.T) {
	pf := newPriceFetcher(priceFetcherOptions{})
	if attempts := pf.buildAttempts("a_share", "600000", "CNY", ""); len(attempts) == 0 {
		t.Fatalf("expected attempts for a_share")
	}
	if attempts := pf.buildAttempts("fund", "110001", "CNY", ""); len(attempts) == 0 {
		t.Fatalf("expected attempts for fund")
	}
	if attempts := pf.buildAttempts("hk_connect", "H00700", "CNY", ""); len(attempts) == 0 {
		t.Fatalf("expected attempts for hk_connect")
	}
	if attempts := pf.buildAttempts("hk_stock", "00001", "HKD", ""); len(attempts) == 0 {
		t.Fatalf("expected attempts for hk_stock")
	}
	if attempts := pf.buildAttempts("us_stock", "AAPL", "USD", ""); len(attempts) == 0 {
		t.Fatalf("expected attempts for us_stock")
	}
	if attempts := pf.buildAttempts("gold", "AU9999", "CNY", ""); len(attempts) == 0 {
		t.Fatalf("expected attempts for gold")
	}
	if attempts := pf.buildAttempts("unknown", "AAPL", "USD", ""); attempts != nil {
		t.Fatalf("expected nil attempts for unknown")
	}

	cases := []struct {
		symbol   string
		currency string
		want     string
	}{
		{"SH600000", "CNY", "a_share"},
		{"600000", "CNY", "a_share"},
		{"110001", "CNY", "etf"},
		{"H00700", "CNY", "hk_connect"},
		{"00001", "HKD", "hk_stock"},
		{"AAPL", "USD", "us_stock"},
		{"AU9999", "CNY", "gold"},
		{"CASH", "USD", "cash"},
		{"BOND123", "CNY", "bond"},
		{"???", "", "unknown"},
	}
	for _, c := range cases {
		if got := detectSymbolType(c.symbol, c.currency, ""); got != c.want {
			t.Fatalf("detectSymbolType(%s,%s)=%s want %s", c.symbol, c.currency, got, c.want)
		}
	}
}

func TestEastmoneyFetchers(t *testing.T) {
	pf := newFetcherWithBody(http.StatusOK, `{"data":{"f43":12345}}`)
	price, err := pf.eastmoneyFetchAShare("600000")
	if err != nil || price == nil || *price != 123.45 {
		t.Fatalf("eastmoneyFetchAShare: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `{"data":{"f43":500}}`)
	price, err = pf.eastmoneyFetchAShare("600000")
	if err != nil || price == nil || *price != 500 {
		t.Fatalf("eastmoneyFetchAShare small: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `{"data":{"f43":500}}`)
	if price, err := pf.eastmoneyFetchAShare("ABC"); err != nil || price != nil {
		t.Fatalf("eastmoneyFetchAShare invalid: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `{"data":{}}`)
	if price, err := pf.eastmoneyFetchAShare("600000"); err != nil || price != nil {
		t.Fatalf("eastmoneyFetchAShare missing f43: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `{"data":{"f43":"bad"}}`)
	if price, err := pf.eastmoneyFetchAShare("600000"); err == nil || price != nil {
		t.Fatalf("eastmoneyFetchAShare parse error expected")
	}

	pf = newFetcherWithBody(http.StatusOK, `callback({"gsz":"1.23"})`)
	price, err = pf.eastmoneyFetchFund("000001")
	if err != nil || price == nil || *price != 1.23 {
		t.Fatalf("eastmoneyFetchFund: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `bad`)
	if price, err := pf.eastmoneyFetchFund("000001"); err != nil || price != nil {
		t.Fatalf("eastmoneyFetchFund missing parens: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `callback({"gsz":"bad"})`)
	if price, err := pf.eastmoneyFetchFund("000001"); err == nil || price != nil {
		t.Fatalf("eastmoneyFetchFund parse error expected")
	}
	pf = newFetcherWithBody(http.StatusOK, `callback({"dwjz":"2.34"})`)
	price, err = pf.eastmoneyFetchFund("000001")
	if err != nil || price == nil || *price != 2.34 {
		t.Fatalf("eastmoneyFetchFund fallback: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `callback({"gsz":"1.23"})`)
	if price, err := pf.eastmoneyFetchFund("ABC"); err != nil || price != nil {
		t.Fatalf("eastmoneyFetchFund invalid: %v %v", price, err)
	}

	pf = newFetcherWithBody(http.StatusOK, "var Data_netWorthTrend =[{\"y\":2.34}];\n")
	price, err = pf.eastmoneyFetchFundPingzhong("000001")
	if err != nil || price == nil || *price != 2.34 {
		t.Fatalf("eastmoneyFetchFundPingzhong: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, "var Data_netWorthTrend =[];\n")
	if price, err := pf.eastmoneyFetchFundPingzhong("000001"); err != nil || price != nil {
		t.Fatalf("eastmoneyFetchFundPingzhong empty: %v %v", price, err)
	}
	if price, err := pf.eastmoneyFetchFundPingzhong("ABC"); err != nil || price != nil {
		t.Fatalf("eastmoneyFetchFundPingzhong invalid: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, "var Data_netWorthTrend =[[0,1.11],[1,2.22]];\n")
	price, err = pf.eastmoneyFetchFundPingzhong("000001")
	if err != nil || price == nil || *price != 2.22 {
		t.Fatalf("eastmoneyFetchFundPingzhong array: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, "no marker here")
	if price, err := pf.eastmoneyFetchFundPingzhong("000001"); err != nil || price != nil {
		t.Fatalf("eastmoneyFetchFundPingzhong missing: %v %v", price, err)
	}

	pf = newFetcherWithBody(http.StatusOK, `<td>2024-01-01</td><td>3.45</td>`)
	price, err = pf.eastmoneyFetchFundLsjz("000001")
	if err != nil || price == nil || *price != 3.45 {
		t.Fatalf("eastmoneyFetchFundLsjz: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `no match`)
	if price, err := pf.eastmoneyFetchFundLsjz("000001"); err != nil || price != nil {
		t.Fatalf("eastmoneyFetchFundLsjz missing: %v %v", price, err)
	}
	if price, err := pf.eastmoneyFetchFundLsjz("ABC"); err != nil || price != nil {
		t.Fatalf("eastmoneyFetchFundLsjz invalid: %v %v", price, err)
	}
}

func TestYahooFetchers(t *testing.T) {
	pf := newFetcherWithBody(http.StatusOK, `{"chart":{"result":[{"meta":{"regularMarketPrice":123.45},"indicators":{"quote":[{"close":[120]}]}}]}}`)
	price, err := pf.yahooFetchStock("AAPL", "USD")
	if err != nil || price == nil || *price != 123.45 {
		t.Fatalf("yahooFetchStock meta: %v %v", price, err)
	}

	pf = newFetcherWithBody(http.StatusOK, `{"chart":{"result":[{"meta":{},"indicators":{"quote":[{"close":[10.5]}]}}]}}`)
	price, err = pf.yahooFetchStock("AAPL", "USD")
	if err != nil || price == nil || *price != 10.5 {
		t.Fatalf("yahooFetchStock quote: %v %v", price, err)
	}

	pf = newFetcherWithBody(http.StatusOK, `{"chart":{"result":[{"meta":{"regularMarketPrice":2000},"indicators":{"quote":[{"close":[2000]}]}}]}}`)
	price, err = pf.yahooFetchGold()
	if err != nil || price == nil {
		t.Fatalf("yahooFetchGold: %v %v", price, err)
	}
	expected := math.Round((2000/31.1035*7.2)*100) / 100
	if *price != expected {
		t.Fatalf("expected gold price %.2f, got %.2f", expected, *price)
	}
	pf = newFetcherWithBody(http.StatusOK, `{"chart":{"result":[]}}`)
	if price, err := pf.yahooFetchStock("AAPL", "USD"); err != nil || price != nil {
		t.Fatalf("yahooFetchStock empty: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `{"chart":{"result":[{"meta":{"regularMarketPrice":0},"indicators":{"quote":[{"close":[0]}]}}]}}`)
	if price, err := pf.yahooFetchGold(); err != nil || price != nil {
		t.Fatalf("yahooFetchGold zero: %v %v", price, err)
	}
}

func TestSinaFetchers(t *testing.T) {
	pf := newFetcherWithBody(http.StatusOK, `var hq_str_sh600000="a,b,c,12.34,d"`)
	price, err := pf.sinaFetchAShare("600000")
	if err != nil || price == nil || *price != 12.34 {
		t.Fatalf("sinaFetchAShare: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `bad`)
	if price, err := pf.sinaFetchAShare("600000"); err != nil || price != nil {
		t.Fatalf("sinaFetchAShare invalid: %v %v", price, err)
	}

	pf = newFetcherWithBody(http.StatusOK, `var hq_str_hk00001="0,1,2,3,4,5,6.78,9"`)
	price, err = pf.sinaFetchHKStock("00001")
	if err != nil || price == nil || *price != 6.78 {
		t.Fatalf("sinaFetchHKStock: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `bad`)
	if price, err := pf.sinaFetchHKStock("00001"); err != nil || price != nil {
		t.Fatalf("sinaFetchHKStock invalid: %v %v", price, err)
	}

	pf = newFetcherWithBody(http.StatusOK, `var hq_str_gb_aapl="0,9.87,1"`)
	price, err = pf.sinaFetchUSStock("AAPL")
	if err != nil || price == nil || *price != 9.87 {
		t.Fatalf("sinaFetchUSStock: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `bad`)
	if price, err := pf.sinaFetchUSStock("AAPL"); err != nil || price != nil {
		t.Fatalf("sinaFetchUSStock invalid: %v %v", price, err)
	}
}

func TestTencentFetchers(t *testing.T) {
	pf := newFetcherWithBody(http.StatusOK, `0~1~2~12.34~`)
	price, err := pf.tencentFetchAShare("600000")
	if err != nil || price == nil || *price != 12.34 {
		t.Fatalf("tencentFetchAShare: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `bad`)
	if price, err := pf.tencentFetchAShare("600000"); err != nil || price != nil {
		t.Fatalf("tencentFetchAShare invalid: %v %v", price, err)
	}

	pf = newFetcherWithBody(http.StatusOK, `0~1~2~6.78~`)
	price, err = pf.tencentFetchHKStock("00001")
	if err != nil || price == nil || *price != 6.78 {
		t.Fatalf("tencentFetchHKStock: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `bad`)
	if price, err := pf.tencentFetchHKStock("00001"); err != nil || price != nil {
		t.Fatalf("tencentFetchHKStock invalid: %v %v", price, err)
	}

	pf = newFetcherWithBody(http.StatusOK, `0~1~2~7.89~`)
	price, err = pf.tencentFetchUSStock("AAPL")
	if err != nil || price == nil || *price != 7.89 {
		t.Fatalf("tencentFetchUSStock: %v %v", price, err)
	}
	pf = newFetcherWithBody(http.StatusOK, `bad`)
	if price, err := pf.tencentFetchUSStock("AAPL"); err != nil || price != nil {
		t.Fatalf("tencentFetchUSStock invalid: %v %v", price, err)
	}
}

func TestHTTPGetNon2xx(t *testing.T) {
	pf := newFetcherWithBody(http.StatusInternalServerError, "")
	if _, err := pf.httpGet(context.Background(), "http://example.com", nil); err == nil {
		t.Fatalf("expected error for non-2xx")
	}
	if _, err := pf.httpGet(context.Background(), "://bad-url", nil); err == nil {
		t.Fatalf("expected error for bad url")
	}
}

func TestHKConnectToHKCode(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"H00700", "00700"},
		{"H09988", "09988"},
		{"h00700", "00700"},
		{"00700", "00700"},
		{"H", "H"},
		{"", ""},
	}
	for _, c := range cases {
		got := hkConnectToHKCode(c.input)
		if got != c.want {
			t.Errorf("hkConnectToHKCode(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestEastmoneyFetchHKConnect(t *testing.T) {
	// f43=565000 means 565.000 HKD (divided by 1000)
	pf := newFetcherWithBody(http.StatusOK, `{"data":{"f43":565000}}`)
	price, err := pf.eastmoneyFetchHKConnect("00700")
	if err != nil || price == nil || *price != 565.0 {
		t.Fatalf("eastmoneyFetchHKConnect: got %v %v, want 565.0", price, err)
	}

	// Missing f43
	pf = newFetcherWithBody(http.StatusOK, `{"data":{}}`)
	price, err = pf.eastmoneyFetchHKConnect("00700")
	if err != nil || price != nil {
		t.Fatalf("eastmoneyFetchHKConnect missing f43: got %v %v", price, err)
	}

	// Parse error
	pf = newFetcherWithBody(http.StatusOK, `{"data":{"f43":"bad"}}`)
	price, err = pf.eastmoneyFetchHKConnect("00700")
	if err == nil || price != nil {
		t.Fatalf("eastmoneyFetchHKConnect bad value: expected error")
	}
}

func TestConvertHKDToCNY(t *testing.T) {
	// Without resolver: uses default rate (0.92)
	pf := newFetcherWithBody(http.StatusOK, "")
	hkdPrice := 100.0
	cnyPrice, err := pf.convertHKDToCNY(func() (*float64, error) {
		return &hkdPrice, nil
	})
	if err != nil || cnyPrice == nil || *cnyPrice != 92.0 {
		t.Fatalf("convertHKDToCNY without resolver: got %v %v, want 92.0", cnyPrice, err)
	}

	// With resolver returning custom rate
	pf.rateResolver = func(fromCurrency string) (float64, error) {
		return 0.90, nil
	}
	cnyPrice, err = pf.convertHKDToCNY(func() (*float64, error) {
		return &hkdPrice, nil
	})
	if err != nil || cnyPrice == nil || *cnyPrice != 90.0 {
		t.Fatalf("convertHKDToCNY with resolver: got %v %v, want 90.0", cnyPrice, err)
	}

	// Nil price pass-through
	cnyPrice, err = pf.convertHKDToCNY(func() (*float64, error) {
		return nil, nil
	})
	if err != nil || cnyPrice != nil {
		t.Fatalf("convertHKDToCNY nil: got %v %v", cnyPrice, err)
	}

	// Error pass-through
	_, err = pf.convertHKDToCNY(func() (*float64, error) {
		return nil, fmt.Errorf("fetch error")
	})
	if err == nil {
		t.Fatalf("convertHKDToCNY error: expected error")
	}
}

func TestParseFloatPriceFetcher(t *testing.T) {
	cases := []struct {
		name    string
		value   any
		want    float64
		wantErr bool
	}{
		{"nil", nil, 0, true},
		{"float64", 1.5, 1.5, false},
		{"int", 2, 2, false},
		{"number", json.Number("3.5"), 3.5, false},
		{"string", "4.5", 4.5, false},
		{"empty", "", 0, true},
		{"unsupported", struct{}{}, 0, true},
	}
	for _, tt := range cases {
		got, err := parseFloat(tt.value)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", tt.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error %v", tt.name, err)
		}
		if got != tt.want {
			t.Fatalf("%s: got %.2f want %.2f", tt.name, got, tt.want)
		}
	}
}
