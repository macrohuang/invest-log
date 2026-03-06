# 变更总结：大文件拆分重构

## 1. 变更概述
- 需求来源：用户要求使用并行任务，将若干大文件按前端与 Go 工程规范拆分，降低单文件体积，同时保持签名和功能完全一致。
- 本次目标：
  - 拆分 `static/modules/pages/settings.js`
  - 拆分 `go-backend/pkg/investlog/ai_chat_client.go`
  - 拆分 `go-backend/pkg/investlog/price_fetcher.go`
  - 拆分 `go-backend/pkg/investlog/ai_symbol_data_fetcher.go`
  - 对 `internal/handlers/handlers.go` 做范围核查并维持最小处理
- 完成情况：已完成前端与 Go 拆分、前端脚本接线、Service Worker 缓存更新、Go 回归测试与覆盖率验证。

## 2. 实现内容

### 2.1 前端 Settings 页面
- `static/modules/pages/settings.js`
  - 从 `1313` 行缩减到 `3` 行，保留稳定入口 `renderSettings()`。
- 新增：
  - `static/modules/pages/settings_render.js`
  - `static/modules/pages/settings_actions.js`
  - `static/modules/pages/settings_ai_advisor.js`
- `static/index.html`
  - 在 `settings.js` 前增加 helper 脚本，以保持原有全局脚本模式下的依赖顺序。
- `static/sw.js`
  - 更新缓存版本为 `invest-log-v5`
  - 将新增的 settings helper 文件纳入预缓存清单。

### 2.2 AI Chat Client
- `go-backend/pkg/investlog/ai_chat_client.go`
  - 从 `1241` 行缩减到 `446` 行，保留：
    - 常量/类型/包级函数变量
    - `normalizeEnum`
    - `cleanupModelJSON`
    - 请求主流程函数
- 新增：
  - `go-backend/pkg/investlog/ai_chat_client_endpoints.go`
  - `go-backend/pkg/investlog/ai_chat_client_logging.go`
  - `go-backend/pkg/investlog/ai_chat_client_stream.go`
  - `go-backend/pkg/investlog/ai_chat_client_helpers.go`
- 保持不变：
  - `requestAIChatCompletion`
  - `requestAIChatCompletionStream`
  - endpoint fallback 顺序
  - 脱敏与错误文案行为

### 2.3 Price Fetcher
- `go-backend/pkg/investlog/price_fetcher.go`
  - 从 `964` 行缩减到 `421` 行，保留类型、缓存/熔断主逻辑、`FetchPrice` 与 `fetch` 主流程。
- 新增：
  - `go-backend/pkg/investlog/price_fetcher_providers.go`
- 保持不变：
  - `buildAttempts` 顺序
  - cache key 语义
  - service 名称与 cooldown 逻辑
  - provider 方法签名

### 2.4 Symbol External Data Fetcher
- `go-backend/pkg/investlog/ai_symbol_data_fetcher.go`
  - 从 `928` 行缩减到 `189` 行，保留：
    - `detectMarket`
    - `fetchExternalDataImpl`
    - `summarizeExternalDataImpl`
- 新增：
  - `go-backend/pkg/investlog/ai_symbol_data_fetcher_types.go`
  - `go-backend/pkg/investlog/ai_symbol_data_fetcher_summary.go`
  - `go-backend/pkg/investlog/ai_symbol_data_fetcher_sources.go`
  - `go-backend/pkg/investlog/ai_symbol_data_fetcher_http.go`
- 保持不变：
  - source name 文案
  - source builder 分发顺序
  - structured/fallback summary 文本结构

### 2.5 `internal/handlers/handlers.go`
- 范围核查结果：该文件当前仅 `27` 行，仓库内未见其它引用，且不属于本次高风险/高体积目标。
- 本次决策：保持不变，避免过度重构引入无收益风险。

## 3. 测试与质量
- 已执行前端语法检查：
  - `node --check static/modules/pages/settings.js`
  - `node --check static/modules/pages/settings_render.js`
  - `node --check static/modules/pages/settings_actions.js`
  - `node --check static/modules/pages/settings_ai_advisor.js`
  - `node --check static/sw.js`
- 已执行 Go 定向回归：
  - `cd go-backend && go test ./pkg/investlog -run 'TestBuildAICompletionsEndpoint|TestRequestAIChatCompletion|TestRequestAIByChatCompletions_StreamingDelta|TestPriceFetcher|TestPriceFetcher_CacheKey|TestBuildDataSources|TestParseEastmoney|TestParseYahoo|TestInferDataType|TestBuildRawSectionsText|TestFetchExternalData|TestSummarizeExternalData'`
- 已执行 Go 全量包测试：
  - `cd go-backend && go test ./pkg/investlog`
  - `cd go-backend && go test ./internal/api`
- 覆盖率结果：
  - `cd go-backend && go test ./pkg/investlog -coverprofile=coverage_split_large_source_files.out`
  - `go tool cover -func=coverage_split_large_source_files.out`
  - total: `81.1%`

## 4. 风险与已知问题
- 前端 Settings 页目前没有自动化 UI 测试，本次仍建议做一次手工 smoke：
  - 页面加载
  - tab 切换
  - API / AI 设置保存
  - symbol filter / save
  - AI Advisor 弹窗与流式展示
- 本次前端拆分未继续把 symbols 逻辑从 `settings_actions.js` 中进一步独立；如果后续仍需更细颗粒度，可再拆 `settings_symbols.js`。

## 5. 待确认事项
- 若你后续想把真正的大 handler 文件 `go-backend/internal/api/handlers.go` 也纳入本任务，需要单独扩 scope。
- 当前代码与测试已通过，尚未提交 commit，等待你的 review / 下一步指令。
