# 大文件拆分重构技术设计

## 1. 设计目标
1. 在不改变功能、不改变对外签名/调用方式的前提下，拆分超大源码文件。
2. 按职责重组前端 Settings 页面与 Go 内部实现，降低单文件复杂度。
3. 优先采用“薄入口 + 私有 helper 文件”的低风险方案。
4. 保持现有测试可继续直接覆盖关键函数，尽量减少断言迁移成本。

## 2. 现状与问题

### 2.1 `static/modules/pages/settings.js`
- 文件约 `1313` 行，集中了：数据加载、HTML 拼装、tab 初始化、表单绑定、symbols 过滤/保存、AI Advisor 弹窗与流式状态。
- 当前通过 `static/index.html` 的普通 `<script defer>` 加载，依赖全局作用域共享函数。
- 风险点：任何拆分都必须保证脚本加载顺序与全局函数可见性。

### 2.2 `go-backend/pkg/investlog/ai_chat_client.go`
- 文件约 `1241` 行，集中了：endpoint 归一化、provider 判断、鉴权、日志脱敏、请求执行、fallback、SSE 解析、响应解码。
- 现有测试直接覆盖其中多个私有 helper，说明这些 helper 已形成隐式测试契约。

### 2.3 `go-backend/pkg/investlog/price_fetcher.go`
- 文件约 `964` 行，集中了：类型定义、缓存、服务熔断/冷却、symbol 类型识别、数据源尝试顺序、Eastmoney/Yahoo/Stooq 拉取逻辑。
- 风险点：优先级、缓存 key、service name、cooldown 语义不可变。

### 2.4 `go-backend/pkg/investlog/ai_symbol_data_fetcher.go`
- 文件约 `928` 行，集中了：市场识别、抓取主流程、按市场的数据源构建、Eastmoney/Yahoo 解析、摘要归一化、fallback summary、HTTP 请求 helper。
- 风险点：source 顺序、section 文案与 summary 文本格式容易被测试间接依赖。

### 2.5 `internal/handlers/handlers.go`
- 文件仅约 `27` 行，且仓库内未发现其它引用，疑似独立/遗留代码。
- 按用户字面路径纳入范围，但不建议为了“拆分”而过度拆分。

## 3. 方案总览

### 3.1 前端：Settings 页面拆分策略
保留 `static/modules/pages/settings.js` 作为稳定入口，只承担以下职责：
- 定义/保留 `renderSettings()`。
- 串起数据加载、页面渲染、tab 初始化、动作绑定。
- 保留对外可调用的 `showAIAdvisorModal(assetTypes)`（可由 helper 文件实现后再挂回全局）。

新增同目录 helper 文件：
- `static/modules/pages/settings_sections.js`
  - 负责 Settings tabs/section HTML 片段构造。
- `static/modules/pages/settings_actions.js`
  - 负责保存 API、AI 设置、账户、资产类型、汇率等动作绑定。
- `static/modules/pages/settings_symbols.js`
  - 负责 symbols 表格过滤、高亮、保存单个 symbol。
- `static/modules/pages/settings_ai_advisor.js`
  - 负责 AI Advisor 弹窗流程、步骤状态与流式展示。

配套调整：
- 在 `static/index.html` 中，将上述 helper script 以 `defer` 方式插入到 `settings.js` 之前，保证运行顺序。
- helper 统一通过 `window` 下命名空间或全局函数暴露给 `settings.js` 使用，避免引入 ES module 改造。

### 3.2 Go：`ai_chat_client.go` 拆分策略
保留以下稳定入口与函数签名：
- `requestAIChatCompletion`
- `requestAIChatCompletionStream`
- 包级变量 `aiChatCompletion` / `aiChatCompletionStream`
- 现有被测试直接调用的 helper（如 `buildAICompletionsEndpoint`、`parseSSEStream` 等）函数名不变

建议拆分文件：
- `ai_chat_client_types.go`：常量、请求/响应类型、包级变量。
- `ai_chat_client_endpoints.go`：endpoint 构建、Gemini 判定、path 变换。
- `ai_chat_client_auth.go`：鉴权 header 与 secret masking。
- `ai_chat_client_logging.go`：请求/响应日志与 debug helper。
- `ai_chat_client_transport.go`：同步/流式请求主流程、`executeAIRequest`。
- `ai_chat_client_stream.go`：SSE chunk 解析、OpenAI/Gemini stream 提取。
- `ai_chat_client_decode.go`：响应 JSON 解码与 content 提取。
- `ai_chat_client_fallback.go`：responses/chat/alt endpoint fallback 判定。

约束：
- 不改变 fallback 顺序。
- 不改变日志文案、错误文案、脱敏规则。
- 不新增导出符号。

### 3.3 Go：`price_fetcher.go` 拆分策略
保留稳定入口：
- `(*Core).FetchPrice`
- `(*priceFetcher).fetch`

建议拆分文件：
- `price_fetcher_types.go`：接口、options、struct、常量与前缀表。
- `price_fetcher_cache.go`：缓存与服务状态（`cacheKey`、`getCached`、`setCached`、cooldown）。
- `price_fetcher_detect.go`：`detectSymbolType`、`buildAttempts`、`preferFundFirstForAShare`。
- `price_fetcher_eastmoney.go`：A 股/基金/HK Connect 对应抓取器。
- `price_fetcher_yahoo.go`：Yahoo 系列抓取器与 symbol 转换。
- `price_fetcher_stooq.go`：Stooq 或其它剩余 provider 逻辑。

约束：
- `buildAttempts` 顺序保持不变。
- service 名称字符串保持不变。
- 缓存 key 格式保持不变。

### 3.4 Go：`ai_symbol_data_fetcher.go` 拆分策略
保留稳定入口：
- `fetchExternalDataImpl`
- `summarizeExternalDataImpl`
- `fetchExternalDataFn` / `summarizeExternalDataFn`

建议拆分文件：
- `ai_symbol_data_fetcher_types.go`：常量、类型、函数变量。
- `ai_symbol_data_sources_cn.go`：CN market source builder。
- `ai_symbol_data_sources_us.go`：US market source builder。
- `ai_symbol_data_sources_hk.go`：HK market source builder。
- `ai_symbol_data_parsers_eastmoney.go`：Eastmoney parser。
- `ai_symbol_data_parsers_yahoo.go`：Yahoo parser。
- `ai_symbol_data_summary.go`：summary 归一化、fallback summary、证据提取 helper。
- `ai_symbol_data_http.go`：HTTP GET helper、通用文本清洗。

约束：
- summary 文本格式、section 顺序、gap note 尽量保持不变。
- parser 绑定关系与 source name 文案保持不变。

### 3.5 `internal/handlers/handlers.go` 处理策略
- 维持单文件为默认方案。
- 若需要轻量整理，仅允许把 `render()` 下沉到 `internal/handlers/render.go`；否则保持原状更稳妥。
- 原因：文件本身不大，且疑似遗留代码；过度拆分几乎无收益。

## 4. 实施顺序
1. 先拆 `ai_chat_client.go`、`price_fetcher.go`、`ai_symbol_data_fetcher.go`，通过编译与现有测试验证 Go 侧稳定性。
2. 再拆 `settings.js`，同时补 `static/index.html` 的 helper script 加载顺序。
3. 最后对 `internal/handlers/handlers.go` 做最小必要处理（可能不改）。

## 5. 兼容性与风险
- 前端风险：helper 文件加载顺序错误会直接导致 `ReferenceError`。
- 前端风险：DOM `id`、`data-*` 选择器和事件绑定作用域变化可能造成静默回归。
- Go 风险：私有 helper 被现有测试直接调用，函数名/签名/行为细节不可轻易调整。
- Go 风险：fallback 顺序、source 顺序、错误文案、summary 格式都可能被现有测试隐式绑定。
- Scope 风险：`internal/handlers/handlers.go` 与此前统计的大文件 `go-backend/internal/api/handlers.go` 不是同一个文件；本设计按用户字面路径执行。

## 6. 设计决议
- 采用“保持 package/module 不变 + 新增同目录私有文件”的方案。
- 不引入 ES module，不调整 Go package 边界。
- 不主动触碰 `go-backend/internal/api/handlers.go`，除非用户明确改 scope。
