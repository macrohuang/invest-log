# Price Fetcher Specification

## Overview

The price fetcher module is responsible for retrieving the latest market prices for various asset types (stocks, funds, bonds, cash, gold) from multiple data sources with fallback mechanisms, caching, and circuit breaker patterns.

## Architecture

### Core Components

1. **priceFetcher**: Main struct that manages price fetching operations
2. **Cache Layer**: In-memory cache with TTL to reduce API calls
3. **Circuit Breaker**: Service-level failure tracking and cooldown mechanism
4. **Multi-Source Fallback**: Tries multiple data providers in priority order

### Configuration Options

```go
type priceFetcherOptions struct {
    Logger        *slog.Logger  // Structured logger
    CacheTTL      time.Duration // Cache time-to-live
    FailThreshold int           // Number of failures before circuit breaks
    FailWindow    time.Duration // Time window for counting failures
    Cooldown      time.Duration // Circuit breaker cooldown period
    HTTPTimeout   time.Duration // HTTP request timeout
}
```

## Price Fetching Flow

### Entry Point: `FetchPrice(symbol, currency, assetType)`

The public API exposed through `Core`:

```
Core.FetchPrice(symbol, currency, assetType) → PriceResult
```

### Main Flow: `priceFetcher.fetch()`

```
1. Normalize Input
   ├─ symbol: Trim whitespace, uppercase
   ├─ currency: Normalize format
   └─ assetType: Default to "stock" if empty

2. Check Cache
   ├─ Generate cache key: "symbol|currency|assetType"
   ├─ If cached and not expired
   │  └─ Return cached price with source
   └─ Otherwise, proceed to fetch

3. Detect Symbol Type
   ├─ Call detectSymbolType(symbol, currency)
   └─ Returns: "a_share", "fund", "hk_stock", "us_stock", "gold", "cash", "bond", "unknown"

4. Handle Special Cases
   ├─ bond → Return error (not supported)
   ├─ cash → Return fixed price 1.0
   └─ unknown → Return error (unrecognized type)

5. Build Attempt List
   ├─ Based on symbol type, create ordered list of data sources
   └─ Each attempt contains: {service_name, fetch_function}

6. Execute Attempts (with fallback)
   For each attempt:
   ├─ Check if service is available (circuit breaker)
   │  └─ If in cooldown, skip and add to error list
   ├─ Execute fetch function
   ├─ If success:
   │  ├─ Record service success (reset circuit breaker)
   │  ├─ Store in cache
   │  └─ Return price with source message
   └─ If failure:
      ├─ Add error to error list
      └─ Record service failure (increment failure count)

7. All Attempts Failed
   └─ Return combined error message with all failures
```

## Symbol Type Detection

### Algorithm: `detectSymbolType(symbol, currency)`

The function analyzes symbol patterns and currency to determine asset type:

```
Input: symbol, currency

Detection Logic:
├─ Starts with "SH" or "SZ" → a_share
├─ Currency = CNY + 6-digit code
│  ├─ Matches A-share prefixes (000,001,002,003,300,301,600,601,603,605,688,689) → a_share
│  ├─ Matches ETF/LOF prefixes (510,513,588,159,160,161,162,163,164,165,166,501,502) → a_share
│  └─ Otherwise → fund
├─ Currency = HKD or matches pattern "0XXXX" → hk_stock
├─ Contains "AU" or "GOLD" → gold
├─ Symbol = "CASH" → cash
├─ Currency = USD or all uppercase letters → us_stock
├─ Contains "BOND" → bond
└─ Otherwise → unknown
```

### A-Share Stock Prefixes
- **000-003**: Shenzhen Main Board
- **300-301**: ChiNext (Growth Enterprise Market)
- **600-605**: Shanghai Main Board
- **688-689**: STAR Market (Science and Technology Innovation Board)

### ETF/LOF Prefixes
- **510,513,588,159,160,161,162,163,164,165,166,501,502**: Exchange-traded funds and Listed Open-ended Funds

## Data Source Priority

### A-Share Stocks

**Default Priority:**
1. Eastmoney (东方财富)
2. Tencent Finance (腾讯财经)
3. Sina Finance (新浪财经)
4. Eastmoney Fund
5. Yahoo Finance

**Fund Asset Type Priority:**
1. Eastmoney Fund (preferred for non-stock asset types)
2. Eastmoney
3. Tencent Finance
4. Sina Finance
5. Yahoo Finance

### Funds (6-digit codes)
1. Eastmoney Fund GZ (估值 - estimated value)
2. Eastmoney Fund PZ (品种 - variety data)
3. Eastmoney Fund LSJZ (历史净值 - historical net value)
4. Eastmoney (as fallback)

### HK Stocks
1. Yahoo Finance
2. Sina Finance
3. Tencent Finance

### US Stocks
1. Yahoo Finance
2. Sina Finance
3. Tencent Finance

### Gold
- Yahoo Finance only (symbol: GC=F)
- Converts price from USD/oz to CNY/gram using rate: `price / 31.1035 * 7.2`

## Data Source APIs

### 1. Eastmoney A-Share API

**Endpoint:**
```
http://push2.eastmoney.com/api/qt/stock/get?secid={market}.{code}&fields=f43&ut=fa5fd1943c7b386f172d6893dbfba10b
```

**Market Code:**
- 1 = Shanghai (SH prefix or starts with 6)
- 0 = Shenzhen (SZ prefix or others)

**Response:** JSON with `data.f43` containing price (divide by 100 if > 1000)

### 2. Eastmoney Fund API (估值)

**Endpoint:**
```
http://fundgz.1234567.com.cn/js/{code}.js
```

**Response:** JSONP format, extracts `gsz` (estimated value) or `dwjz` (unit net value)

### 3. Eastmoney Fund PZ API (品种)

**Endpoint:**
```
http://fund.eastmoney.com/pingzhongdata/{code}.js
```

**Response:** JavaScript file, parses `Data_netWorthTrend` array for latest value

### 4. Eastmoney Fund LSJZ API (历史净值)

**Endpoint:**
```
http://fund.eastmoney.com/f10/F10DataApi.aspx?type=lsjz&code={code}&page=1&per=1
```

**Response:** HTML table, regex extracts latest net value

### 5. Yahoo Finance API

**Endpoint:**
```
https://query1.finance.yahoo.com/v8/finance/chart/{symbol}?interval=1d&range=1d
```

**Symbol Mapping:**
- CNY A-Share: code.SS (Shanghai) or code.SZ (Shenzhen)
- HKD: 0000-padded code.HK
- USD: code as-is

**Response:** JSON, extracts `meta.regularMarketPrice` or latest `quote.close`

### 6. Sina Finance APIs

**A-Share Endpoint:**
```
http://hq.sinajs.cn/list={prefix}{code}
```
- Prefix: "sh" for Shanghai, "sz" for Shenzhen
- Response: CSV format, price at index 3

**HK Stock Endpoint:**
```
http://hq.sinajs.cn/list=hk{code}
```
- Code: 5-digit padded
- Response: CSV format, price at index 6

**US Stock Endpoint:**
```
http://hq.sinajs.cn/list=gb_{code}
```
- Response: CSV format, price at index 1

### 7. Tencent Finance APIs

**Endpoint Pattern:**
```
http://qt.gtimg.cn/q={prefix}{code}
```

**Prefixes:**
- A-Share: "sh" or "sz"
- HK Stock: "hk" + 5-digit code
- US Stock: "us" + code

**Response:** Tilde-separated values, price at index 3

## Caching Mechanism

### Cache Structure
```go
type cacheEntry struct {
    price  float64   // Cached price value
    source string    // Data source name
    ts     time.Time // Timestamp of cache entry
}
```

### Cache Key Format
```
"{symbol}|{currency}|{assetType}"
```

### Cache Behavior
- **Cache Hit**: Return immediately if entry exists and not expired (within TTL)
- **Cache Miss**: Fetch from data sources and store result
- **Expiration**: Based on configured `CacheTTL` duration

## Circuit Breaker Pattern

### Purpose
Prevents repeated failures to unreliable data sources by temporarily disabling them.

### Service State Tracking
```go
type serviceState struct {
    failCount     int       // Number of failures in current window
    firstFailAt   time.Time // Timestamp of first failure in window
    cooldownUntil time.Time // Time until service is available again
}
```

### Circuit Breaker Logic

**Failure Recording:**
1. Increment `failCount` for the service
2. If outside `failWindow`, reset counter and start new window
3. If `failCount >= failThreshold`, enter cooldown period
4. Service becomes unavailable until `cooldownUntil`

**Success Recording:**
- Reset service state (remove from tracking map)
- Service immediately becomes available

**Availability Check:**
- Service is available if `time.Now() > cooldownUntil`
- Services not in state map are available by default

### Example Flow
```
Config: failThreshold=3, failWindow=5min, cooldown=10min

Timeline:
├─ T+0min: Service fails (count=1)
├─ T+1min: Service fails (count=2)
├─ T+2min: Service fails (count=3) → Circuit opens, cooldown until T+12min
├─ T+3min: Service skipped (in cooldown)
├─ T+12min: Circuit closes, service available again
└─ T+13min: Service succeeds → Reset state
```

## Error Handling

### Error Types

1. **Bond Not Supported**
   - Message: "债券价格暂不支持自动获取"
   - Reason: No reliable data source

2. **Unknown Symbol Type**
   - Message: "无法识别标的类型: {symbol}"
   - Reason: Cannot determine asset category

3. **All Sources Failed**
   - Message: "价格获取失败: {source1}: {error1}; {source2}: {error2}; ..."
   - Includes all attempted sources and their errors

4. **Service In Cooldown**
   - Message: "{service}: 熔断冷却中"
   - Added to error list but doesn't count as fetch attempt

### HTTP Error Handling

- **Response Size Limit**: 1MB maximum to prevent memory exhaustion
- **Status Code Check**: Only 2xx considered success
- **Timeout**: Configured via `HTTPTimeout` option
- **Network Errors**: Propagated to caller with error message

## Utility Functions

### Symbol Normalization
```go
normalizeSymbol(symbol) → uppercase, trimmed
```

### Currency Normalization
```go
normalizeCurrency(currency) → uppercase, trimmed
```

### Float Parsing
Handles multiple data types:
- `float64`, `float32`
- `int`, `int64`
- `json.Number`
- `string` (parsed with `strconv.ParseFloat`)
- Returns error for `nil` or unsupported types

### Yahoo Symbol Builder
Converts internal symbol format to Yahoo Finance ticker format based on currency:
- CNY: Adds `.SS` or `.SZ` suffix
- HKD: Pads to 4 digits, adds `.HK` suffix
- USD: Returns as-is

## Success Response Format

```go
type PriceResult struct {
    Price   *float64 // Nil if fetch failed
    Message string   // Human-readable status message
}
```

### Message Examples

**Success:**
- "价格获取成功 (缓存, 来源: Eastmoney)" (from cache)
- "价格获取成功 (来源: Yahoo Finance)" (fresh fetch)
- "现金价格固定为 1.0" (cash asset)

**Failure:**
- "价格获取失败: Eastmoney: http status 500; Yahoo Finance: 未获取到数据"
- "债券价格暂不支持自动获取"
- "无法识别标的类型: XYZ123"

## Performance Considerations

### Optimization Strategies

1. **Caching**: Reduces API calls for frequently queried symbols
2. **Circuit Breaker**: Avoids wasting time on unreliable services
3. **Prioritized Sources**: Tries most reliable sources first
4. **Response Size Limit**: Prevents memory issues from large responses
5. **HTTP Timeout**: Prevents hanging requests from blocking operations

### Concurrency Safety

- All cache and state operations protected by `sync.Mutex`
- Thread-safe for concurrent price fetching across multiple symbols

## Special Cases

### Cash Assets
- Symbol: "CASH"
- Always returns fixed price: 1.0
- No API calls required
- Message: "现金价格固定为 1.0"

### Gold Assets
- Fetches from Yahoo Finance (GC=F futures contract)
- Converts USD per troy ounce to CNY per gram
- Conversion formula: `price_usd / 31.1035 * 7.2`
- Rounds to 2 decimal places

### A-Share Funds vs Stocks
- Same 6-digit code format
- Asset type parameter determines priority order
- Non-"stock" asset types prefer fund APIs first
- Fallback to stock APIs if fund APIs fail

## Integration Points

### Initialization
```go
core := investlog.NewCore(dataDir, logger, priceFetcherOptions{
    CacheTTL:      5 * time.Minute,
    FailThreshold: 3,
    FailWindow:    5 * time.Minute,
    Cooldown:      10 * time.Minute,
    HTTPTimeout:   10 * time.Second,
})
```

### Usage
```go
result, err := core.FetchPrice("600519", "CNY", "stock")
if err != nil {
    // Handle error
}
if result.Price != nil {
    fmt.Printf("Price: %.2f, Source: %s\n", *result.Price, result.Message)
}
```

## Future Enhancements

### Potential Improvements

1. **Bond Price Support**: Add bond price data sources
2. **Currency Conversion**: Real-time FX rates for cross-currency pricing
3. **Historical Prices**: Extend API to support date-specific pricing
4. **Metrics Collection**: Add observability for source reliability
5. **Configurable Source Priority**: Allow runtime configuration of source order
6. **Batch Fetching**: Optimize for fetching multiple symbols at once
7. **Persistent Cache**: Redis or disk-based cache for longer TTL
8. **Rate Limiting**: Per-source rate limits to comply with API terms
