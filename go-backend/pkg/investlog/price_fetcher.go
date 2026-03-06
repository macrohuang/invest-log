package investlog

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// priceScaleThreshold is the threshold above which prices are scaled down.
	// Some data sources return prices in cents/fen, requiring division by 100.
	priceScaleThreshold = 1000.0
	priceScaleFactor    = 100.0

	// Gold price conversion constants.
	ouncesToGrams       = 31.1035 // Troy ounces to grams
	defaultUSDToCNYRate = 7.2     // Default USD/CNY rate; should be overridden with real-time rate
)

// Pre-compiled regexes for symbol detection and parsing.
// These are compiled once at package init to avoid repeated compilation in hot paths.
var (
	reSixDigit   = regexp.MustCompile(`^\d{6}$`)  // Chinese stock/fund codes
	reHKStock    = regexp.MustCompile(`^0\d{4}$`) // Hong Kong stock codes (e.g., 00001)
	reHKConnect  = regexp.MustCompile(`^H\d{5}$`) // Stock Connect (港股通) codes (e.g., H00700)
	reUSStock    = regexp.MustCompile(`^[A-Z]+$`) // US stock tickers (e.g., AAPL)
	reFundLsjzTD = regexp.MustCompile(`<td[^>]*>\d{4}-\d{2}-\d{2}</td>\s*<td[^>]*>([\d.]+)</td>`)
)

// Price fetcher errors. Use errors.Is() to check for these conditions.
var (
	// ErrInvalidSymbol indicates the symbol format is not recognized by the data source.
	ErrInvalidSymbol = errors.New("invalid symbol format")
	// ErrNoData indicates the data source returned no price data for the symbol.
	ErrNoData = errors.New("no price data available")
	// ErrBondNotSupported indicates bond price fetching is not implemented.
	ErrBondNotSupported = errors.New("bond price not supported")
	// ErrUnknownSymbol indicates the symbol type could not be determined.
	ErrUnknownSymbol = errors.New("unknown symbol type")
)

// Symbol classification prefixes for Chinese markets.
// A-share stocks: main board (000, 001, 600, 601, 603, 605), SME board (002, 003),
// ChiNext (300, 301), STAR market (688, 689).
var aSharePrefixes = []string{
	"000", "001", "002", "003", // Shenzhen main board & SME
	"300", "301", // ChiNext
	"600", "601", "603", "605", // Shanghai main board
	"688", "689", // STAR market
}

// ETF/LOF prefixes for Chinese markets.
// Shanghai: 510 (ETF), 513 (cross-border ETF), 588 (sci-tech ETF), 501/502 (LOF).
// Shenzhen: 159 (ETF), 160-166 (LOF).
var etfLofPrefixes = []string{
	"510", "513", "588", "501", "502", // Shanghai
	"159", "160", "161", "162", "163", "164", "165", "166", // Shenzhen
}

// hasAnyPrefix checks if s starts with any of the given prefixes.
func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// HTTPDoer is an interface for making HTTP requests. It enables dependency
// injection for testing without network calls.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type priceFetcherOptions struct {
	Logger        *slog.Logger
	CacheTTL      time.Duration
	FailThreshold int
	FailWindow    time.Duration
	Cooldown      time.Duration
	HTTPTimeout   time.Duration
	HTTPClient    HTTPDoer                                   // Optional: inject custom client for testing
	USDToCNYRate  float64                                    // Optional: USD/CNY exchange rate for gold price conversion
	RateResolver  func(fromCurrency string) (float64, error) // Optional: resolve FX rates at runtime (e.g. HKD→CNY)
}

type priceFetcher struct {
	logger        *slog.Logger
	cacheTTL      time.Duration
	failThreshold int
	failWindow    time.Duration
	cooldown      time.Duration
	client        HTTPDoer
	usdToCNYRate  float64
	rateResolver  func(fromCurrency string) (float64, error)

	// Separate locks for cache and circuit breaker to reduce contention.
	// Cache operations are frequent reads; circuit breaker updates are less frequent.
	cacheMu      sync.RWMutex
	cache        map[string]cacheEntry
	circuitMu    sync.Mutex
	serviceState map[string]*serviceState
}

type cacheEntry struct {
	price  float64
	source string
	ts     time.Time
}

type serviceState struct {
	failCount     int
	firstFailAt   time.Time
	cooldownUntil time.Time
}

func newPriceFetcher(opts priceFetcherOptions) *priceFetcher {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: opts.HTTPTimeout,
		}
	}
	usdToCNYRate := opts.USDToCNYRate
	if usdToCNYRate <= 0 {
		usdToCNYRate = defaultUSDToCNYRate
	}
	return &priceFetcher{
		logger:        logger,
		cacheTTL:      opts.CacheTTL,
		failThreshold: opts.FailThreshold,
		failWindow:    opts.FailWindow,
		cooldown:      opts.Cooldown,
		client:        client,
		usdToCNYRate:  usdToCNYRate,
		rateResolver:  opts.RateResolver,
		cache:         map[string]cacheEntry{},
		serviceState:  map[string]*serviceState{},
	}
}

// FetchPrice fetches latest price with fallback.
func (c *Core) FetchPrice(symbol, currency, assetType string) (PriceResult, error) {
	priceF, message, err := c.price.fetch(symbol, currency, assetType)
	if err != nil {
		return PriceResult{Price: nil, Message: message}, err
	}
	if priceF != nil {
		a := NewAmount(*priceF)
		return PriceResult{Price: &a, Message: message}, nil
	}
	return PriceResult{Price: nil, Message: message}, nil
}

func (pf *priceFetcher) fetch(symbol, currency, assetType string) (*float64, string, error) {
	symbol = normalizeSymbol(symbol)
	currency = normalizeCurrency(currency)
	assetType = strings.ToLower(strings.TrimSpace(assetType))
	if assetType == "" {
		assetType = "stock"
	}

	if cachedPrice, source, ok := pf.getCached(symbol, currency, assetType); ok {
		msg := fmt.Sprintf("价格获取成功 (缓存, 来源: %s)", source)
		return &cachedPrice, msg, nil
	}

	symbolType := detectSymbolType(symbol, currency, assetType)
	pf.logger.Info("fetching price", "symbol", symbol, "currency", currency, "assetType", assetType, "type", symbolType)

	if symbolType == "bond" {
		return nil, "债券价格暂不支持自动获取", ErrBondNotSupported
	}
	if symbolType == "cash" {
		price := 1.0
		return &price, "现金价格固定为 1.0", nil
	}
	if symbolType == "unknown" {
		return nil, fmt.Sprintf("无法识别标的类型: %s", symbol), ErrUnknownSymbol
	}

	attempts := pf.buildAttempts(symbolType, symbol, currency, assetType)
	var errorsList []string
	for _, attempt := range attempts {
		service := attempt.name
		if !pf.serviceAvailable(service) {
			errorsList = append(errorsList, fmt.Sprintf("%s: 熔断冷却中", service))
			continue
		}
		price, err := attempt.fn()
		if err == nil && price != nil {
			pf.recordServiceSuccess(service)
			pf.setCached(symbol, currency, assetType, *price, service)
			msg := fmt.Sprintf("价格获取成功 (来源: %s)", service)
			return price, msg, nil
		}
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: %v", service, err))
		} else {
			errorsList = append(errorsList, fmt.Sprintf("%s: 未获取到数据", service))
		}
		pf.recordServiceFailure(service)
	}

	if len(errorsList) == 0 {
		errorsList = append(errorsList, "所有数据源均不可用")
	}
	msg := fmt.Sprintf("价格获取失败: %s", strings.Join(errorsList, "; "))
	return nil, msg, errors.New(msg)
}

type fetchAttempt struct {
	name string
	fn   func() (*float64, error)
}

func (pf *priceFetcher) buildAttempts(symbolType, symbol, currency, assetType string) []fetchAttempt {
	switch symbolType {
	case "a_share":
		if preferFundFirstForAShare(assetType) {
			return []fetchAttempt{
				{"Eastmoney Fund", func() (*float64, error) { return pf.eastmoneyFetchFund(symbol) }},
				{"Eastmoney", func() (*float64, error) { return pf.eastmoneyFetchAShare(symbol) }},
				{"Tencent Finance", func() (*float64, error) { return pf.tencentFetchAShare(symbol) }},
				{"Sina Finance", func() (*float64, error) { return pf.sinaFetchAShare(symbol) }},
				{"Yahoo Finance", func() (*float64, error) { return pf.yahooFetchStock(symbol, currency) }},
			}
		}
		return []fetchAttempt{
			{"Eastmoney", func() (*float64, error) { return pf.eastmoneyFetchAShare(symbol) }},
			{"Tencent Finance", func() (*float64, error) { return pf.tencentFetchAShare(symbol) }},
			{"Sina Finance", func() (*float64, error) { return pf.sinaFetchAShare(symbol) }},
			{"Eastmoney Fund", func() (*float64, error) { return pf.eastmoneyFetchFund(symbol) }},
			{"Yahoo Finance", func() (*float64, error) { return pf.yahooFetchStock(symbol, currency) }},
		}
	case "fund", "etf":
		return []fetchAttempt{
			{"Eastmoney Fund GZ", func() (*float64, error) { return pf.eastmoneyFetchFund(symbol) }},
			{"Eastmoney Fund PZ", func() (*float64, error) { return pf.eastmoneyFetchFundPingzhong(symbol) }},
			{"Eastmoney Fund LSJZ", func() (*float64, error) { return pf.eastmoneyFetchFundLsjz(symbol) }},
			{"Eastmoney", func() (*float64, error) { return pf.eastmoneyFetchAShare(symbol) }},
		}
	case "hk_connect":
		hkCode := hkConnectToHKCode(symbol)
		return []fetchAttempt{
			{"Eastmoney HK Connect", func() (*float64, error) {
				return pf.convertHKDToCNY(func() (*float64, error) {
					return pf.eastmoneyFetchHKConnect(hkCode)
				})
			}},
			{"Yahoo Finance (HK Connect)", func() (*float64, error) {
				return pf.convertHKDToCNY(func() (*float64, error) {
					return pf.yahooFetchStock(hkCode, "HKD")
				})
			}},
			{"Sina Finance (HK Connect)", func() (*float64, error) {
				return pf.convertHKDToCNY(func() (*float64, error) {
					return pf.sinaFetchHKStock(hkCode)
				})
			}},
			{"Tencent Finance (HK Connect)", func() (*float64, error) {
				return pf.convertHKDToCNY(func() (*float64, error) {
					return pf.tencentFetchHKStock(hkCode)
				})
			}},
		}
	case "hk_stock":
		return []fetchAttempt{
			{"Yahoo Finance", func() (*float64, error) { return pf.yahooFetchStock(symbol, currency) }},
			{"Sina Finance", func() (*float64, error) { return pf.sinaFetchHKStock(symbol) }},
			{"Tencent Finance", func() (*float64, error) { return pf.tencentFetchHKStock(symbol) }},
		}
	case "us_stock":
		return []fetchAttempt{
			{"Yahoo Finance", func() (*float64, error) { return pf.yahooFetchStock(symbol, currency) }},
			{"Sina Finance", func() (*float64, error) { return pf.sinaFetchUSStock(symbol) }},
			{"Tencent Finance", func() (*float64, error) { return pf.tencentFetchUSStock(symbol) }},
		}
	case "gold":
		return []fetchAttempt{{"Yahoo Finance", pf.yahooFetchGold}}
	default:
		return nil
	}
}

func preferFundFirstForAShare(assetType string) bool {
	assetType = strings.ToLower(strings.TrimSpace(assetType))
	return assetType != "" && assetType != "stock"
}

func (pf *priceFetcher) getCached(symbol, currency, assetType string) (float64, string, bool) {
	key := cacheKey(symbol, currency, assetType)
	pf.cacheMu.RLock()
	defer pf.cacheMu.RUnlock()
	entry, ok := pf.cache[key]
	if !ok {
		return 0, "", false
	}
	if time.Since(entry.ts) <= pf.cacheTTL {
		return entry.price, entry.source, true
	}
	return 0, "", false
}

func (pf *priceFetcher) setCached(symbol, currency, assetType string, price float64, source string) {
	key := cacheKey(symbol, currency, assetType)
	pf.cacheMu.Lock()
	defer pf.cacheMu.Unlock()
	pf.cache[key] = cacheEntry{price: price, source: source, ts: time.Now()}
}

func cacheKey(symbol, currency, assetType string) string {
	return fmt.Sprintf("%s|%s|%s", symbol, currency, assetType)
}

func (pf *priceFetcher) serviceAvailable(service string) bool {
	pf.circuitMu.Lock()
	defer pf.circuitMu.Unlock()
	state, ok := pf.serviceState[service]
	if !ok {
		return true
	}
	return time.Now().After(state.cooldownUntil)
}

func (pf *priceFetcher) recordServiceFailure(service string) {
	pf.circuitMu.Lock()
	defer pf.circuitMu.Unlock()
	state := pf.serviceState[service]
	now := time.Now()
	if state == nil {
		state = &serviceState{firstFailAt: now}
		pf.serviceState[service] = state
	}
	if now.Sub(state.firstFailAt) > pf.failWindow {
		state.failCount = 0
		state.firstFailAt = now
	}
	state.failCount++
	if state.failCount >= pf.failThreshold {
		state.cooldownUntil = now.Add(pf.cooldown)
	}
}

func (pf *priceFetcher) recordServiceSuccess(service string) {
	pf.circuitMu.Lock()
	defer pf.circuitMu.Unlock()
	delete(pf.serviceState, service)
}

func detectSymbolType(symbol, currency, assetType string) string {
	symbol = normalizeSymbol(symbol)
	currency = normalizeCurrency(currency)
	assetType = strings.ToLower(strings.TrimSpace(assetType))

	// Explicit exchange prefix (SH/SZ) -> A-share
	if strings.HasPrefix(symbol, "SH") || strings.HasPrefix(symbol, "SZ") {
		return "a_share"
	}

	// Chinese 6-digit codes
	if currency == "CNY" && reSixDigit.MatchString(symbol) {
		// Trust explicit asset type for ETF/fund
		if assetType == "etf" || assetType == "fund" {
			return "etf"
		}
		// Check by prefix
		if hasAnyPrefix(symbol, etfLofPrefixes) {
			return "etf"
		}
		if hasAnyPrefix(symbol, aSharePrefixes) {
			return "a_share"
		}
		// Default unknown 6-digit CNY codes to ETF (likely OTC funds)
		return "etf"
	}

	// Hong Kong Stock Connect (港股通) - H prefix + 5-digit HK code
	if reHKConnect.MatchString(symbol) {
		return "hk_connect"
	}

	// Hong Kong stocks
	if currency == "HKD" || reHKStock.MatchString(symbol) {
		return "hk_stock"
	}

	// Gold
	if strings.Contains(symbol, "AU") || strings.Contains(symbol, "GOLD") {
		return "gold"
	}

	// Cash
	if symbol == "CASH" {
		return "cash"
	}

	// US stocks
	if currency == "USD" || reUSStock.MatchString(symbol) {
		return "us_stock"
	}

	// Bonds
	if strings.Contains(symbol, "BOND") {
		return "bond"
	}

	return "unknown"
}

// Eastmoney A-share.
