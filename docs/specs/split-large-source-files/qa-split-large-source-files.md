# 大文件拆分重构 QA / 测试设计

## 1. 测试目标
- 验证本次重构不改变对外行为。
- 利用现有 Go 自动化测试覆盖关键隐式契约。
- 对无自动化测试基础的前端 Settings 页执行手工 smoke 回归。

## 2. 现有测试基础

### 2.1 `go-backend/pkg/investlog/ai_chat_client.go`
- 现有覆盖主要位于：
  - `go-backend/pkg/investlog/ai_holdings_analysis_test.go`
  - `go-backend/pkg/investlog/ai_openai_client_test.go`
- 已覆盖：
  - `buildAICompletionsEndpoint`
  - chat/responses fallback
  - alt endpoint fallback
  - Gemini endpoint 归一化
  - timeout/friendly error
  - debug log / raw response log
  - streaming / non-streaming / Claude SSE / Gemini stream

### 2.2 `go-backend/pkg/investlog/price_fetcher.go`
- 现有覆盖：`go-backend/pkg/investlog/price_fetcher_test.go`
- 已覆盖：
  - `detectSymbolType`
  - A 股/ETF/美股/HK fetcher 分支
  - Eastmoney/Yahoo 抓取器
  - 部分 service cooldown 路径

### 2.3 `go-backend/pkg/investlog/ai_symbol_data_fetcher.go`
- 现有覆盖：`go-backend/pkg/investlog/ai_symbol_data_fetcher_test.go`
- 已覆盖：
  - `buildEastmoneySecID`
  - Eastmoney/Yahoo parsers
  - `extractYahooValue`
  - `fetchExternalDataImpl`
  - `summarizeExternalDataImpl`

### 2.4 `static/modules/pages/settings.js`
- 未见自动化测试基础。
- 当前只能依赖手工 smoke + 路由可达性验证。

### 2.5 `internal/handlers/handlers.go`
- 未见现成测试，且文件当前较小、疑似遗留。

## 3. 必测回归点

| 编号 | 模块 | 级别 | 回归点 | 预期 |
|---|---|---|---|---|
| TC-001 | `ai_chat_client` | P0 | endpoint 归一化与 fallback 顺序 | 与重构前完全一致 |
| TC-002 | `ai_chat_client` | P0 | SSE chunk 解析与 `onDelta` 行为 | 与重构前完全一致 |
| TC-003 | `ai_chat_client` | P0 | 错误文案、timeout、debug 脱敏日志 | 与重构前完全一致 |
| TC-004 | `price_fetcher` | P0 | `detectSymbolType` + `buildAttempts` 优先级 | 与重构前完全一致 |
| TC-005 | `price_fetcher` | P0 | service cooldown / cache key / provider fallback | 与重构前完全一致 |
| TC-006 | `ai_symbol_data_fetcher` | P0 | market -> source builder -> parser 绑定 | 与重构前完全一致 |
| TC-007 | `ai_symbol_data_fetcher` | P0 | summary 文本结构与 fallback summary | 与重构前完全一致 |
| TC-008 | `settings.js` | P1 | Settings 页加载、tab 切换、保存与弹窗流程 | 手工 smoke 通过 |
| TC-009 | `internal/handlers` | P2 | `TransactionsHandler` 模板渲染 | 若保留不动，可不新增自动化 |

## 4. 建议新增/补强测试

### 4.1 `go-backend/pkg/investlog/ai_holdings_analysis_test.go`
- 保留现有所有 `requestAIChatCompletion*` / `buildAICompletionsEndpoint` 回归测试。
- 可补 table-driven 用例：
  - `stripKnownAIEndpointSuffix`
  - `toResponsesEndpoint`
  - `supportsResponsesFallbackModel`
  - `parseAIErrorMessage`

### 4.2 `go-backend/pkg/investlog/price_fetcher_test.go`
- 建议补充：
  - `TestPriceFetcherBuildAttempts_Order`
  - `TestPriceFetcherCacheKey_Stable`
  - `TestPriceFetcherServiceState_CooldownTransitions`
  - `TestPreferFundFirstForAShare`

### 4.3 `go-backend/pkg/investlog/ai_symbol_data_fetcher_test.go`
- 建议补充：
  - `TestBuildDataSources_ByMarket`
  - `TestInferDataType`
  - `TestBuildRawSectionsText`
  - 针对 `normalizeStructuredExternalSummary` / `buildFallbackStructuredExternalSummary` 的格式回归

### 4.4 `static/modules/pages/settings.js`
- 不新增自动化测试框架。
- 采用手工 smoke checklist：
  - Settings 页面首屏加载成功。
  - tab 切换成功。
  - 保存 API Base URL 成功。
  - 保存 AI Settings 成功。
  - accounts / asset types / FX / symbols 编辑与保存成功。
  - symbol filter / clear / highlight 正常。
  - AI Advisor 四步流程、关闭重开、流式展示正常。

### 4.5 `internal/handlers/handlers.go`
- 若最终仅保持原状或极小整理，则不强制新增自动化测试。
- 若确有改动，可考虑补一个轻量模板渲染测试；否则以手工 smoke 为准。

## 5. 覆盖率策略（目标 >= 80%）
- 主要依赖 `go-backend/pkg/investlog` 现有测试体系。
- 本次新增测试以“补足拆分后最容易回归的 helper”为主，不额外引入前端测试基础设施。
- 验证命令建议：
```bash
cd go-backend

go test ./pkg/investlog ./internal/api

go test ./pkg/investlog ./internal/api -coverprofile=coverage_split_large_source_files.out

go tool cover -func=coverage_split_large_source_files.out
```
- 若只改 `pkg/investlog`，则以该包为主要 coverage 观测对象；若 `internal/api` 未改，可不将其视为本次阻塞项。

## 6. 退出标准
- Go 相关测试通过。
- 变更相关 Go 包 coverage >= 80%。
- Settings 页手工 smoke checklist 通过。
- 未出现签名、路由、返回结构、summary 文本格式等兼容性回归。
