# 个股分析 Perplexity 检索增强技术设计

## 1. 设计目标

1. 修复当前个股分析中“开启 Perplexity 会切换整条分析链路模型”的行为偏差。
2. 明确并固化分工：
   - Perplexity 仅用于 external summary（外部信息摘要）检索增强。
   - framework 分析与 synthesis 综合分析继续使用主模型（`base_url/api_key/model`）。
3. 新增可选检索增强字段 `retrieval_base_url` / `retrieval_api_key` / `retrieval_model`，并定义可预测降级策略。
4. 保持现有接口与数据库兼容，不引入破坏性变更。

## 2. 现状

### 2.1 现有链路（实现事实）

1. 前端 `static/modules/pages/symbol-analysis.js`：
   - `runSymbolAnalysis()` 在开启 Perplexity 时，直接把请求中的 `base_url/model/api_key` 替换为 Perplexity 配置。
2. API 层 `go-backend/internal/api/types.go` + `go-backend/internal/api/handlers.go`：
   - `aiSymbolAnalysisPayload` 仅有 `base_url/api_key/model`。
   - `analyzeSymbolWithAI` 与 `analyzeSymbolWithAIStream` 直接透传到 `SymbolAnalysisRequest`。
3. Core 层 `go-backend/pkg/investlog/ai_symbol_analysis_service.go`：
   - `analyzeSymbol()` 内仅构造一个 `endpointURL`（来自 `BaseURL`）。
   - `summarizeExternalDataFn`、`runDimensionAgents`、`runSynthesisAgent` 全部共享同一 `endpoint/api_key/model`。

### 2.2 问题结论

开启 Perplexity 后，摘要、框架分析、综合分析全部切到 Perplexity，违背 MRD 目标（主模型应继续负责 framework+synthesis）。

## 3. 方案

### 3.1 路由原则

1. 主模型配置（必填）：`base_url` + `api_key` + `model`。
2. 检索增强配置（可选）：`retrieval_base_url` + `retrieval_api_key` + `retrieval_model`。
3. 执行规则：
   - external summary 阶段优先使用检索增强配置。
   - framework + synthesis 阶段始终使用主模型配置。

### 3.2 接口字段定义

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `retrieval_base_url` | string | 否 | 检索增强 provider 的 base URL（当前由前端填 Perplexity）。 |
| `retrieval_api_key` | string | 否 | 检索增强 provider 的 API Key。 |
| `retrieval_model` | string | 否 | 检索增强模型名（如 `sonar-pro`）。 |

启用条件：三项字段经 trim 后均非空，且 `retrieval_base_url` 可被 `buildAICompletionsEndpoint` 成功归一化。

### 3.3 降级策略

1. 任一检索增强字段缺失：忽略检索增强，摘要阶段回退主模型。
2. `retrieval_base_url` 归一化失败：记录 warning，摘要阶段回退主模型；不终止主流程。
3. 摘要调用失败：维持现有 graceful degradation（fallback summary），框架/综合继续执行。
4. 主模型字段校验失败：保持现有失败语义（返回 4xx / 业务错误），不因检索增强逻辑改变。

## 4. 详细设计

### 4.1 数据结构与接口变更

1. `go-backend/internal/api/types.go`
   - `aiSymbolAnalysisPayload` 增加：
     - `RetrievalBaseURL string \`json:"retrieval_base_url"\``
     - `RetrievalAPIKey string \`json:"retrieval_api_key"\``
     - `RetrievalModel string \`json:"retrieval_model"\``
2. `go-backend/pkg/investlog/ai_symbol_analysis_types.go`
   - `SymbolAnalysisRequest` 增加：
     - `RetrievalBaseURL string`
     - `RetrievalAPIKey string`
     - `RetrievalModel string`
3. `go-backend/internal/api/handlers.go`
   - 两个入口 `analyzeSymbolWithAI` / `analyzeSymbolWithAIStream` 透传新增字段到 `SymbolAnalysisRequest`。

### 4.2 请求归一化

文件：`go-backend/pkg/investlog/ai_symbol_analysis_request.go`

1. 对新增 retrieval 字段执行 `strings.TrimSpace`。
2. retrieval 字段不作为必填，不新增阻断式校验错误。
3. 归一化结果只提供“规范值”，是否启用在 service 层判定（保证降级策略集中在执行层）。

### 4.3 服务层路由与执行

文件：`go-backend/pkg/investlog/ai_symbol_analysis_service.go`

1. 继续先构造主模型 endpoint（失败即返回错误，保持现有主流程保障）。
2. 新增“摘要阶段 provider 选择”逻辑（可用私有 helper）：
   - 默认：摘要使用主模型 endpoint/api_key/model。
   - 若 retrieval 三字段完整，尝试构造 retrieval endpoint。
   - 构造成功：摘要使用 retrieval 配置。
   - 构造失败：`logger.Warn(...)` 后回退主模型。
3. 调用点调整：
   - `summarizeExternalDataFn(...)` 使用“摘要 provider”。
   - `runDimensionAgents(...)` 与 `runSynthesisAgent(...)` 继续使用主模型 provider（不受 retrieval 开关影响）。
4. 返回值：`SymbolAnalysisResult.Model` 仍写入主模型 `normalizedReq.Model`。
5. 日志约束：
   - 可记录 `retrieval_enabled`、`retrieval_model`、`fallback_reason`。
   - 不记录任何 API Key 明文。

### 4.4 前端语义与请求拼装

文件：`static/modules/pages/symbol-analysis.js`

1. 调整开关语义：从“Perplexity 全链路分析”改为“Perplexity 检索增强”。
2. `runSymbolAnalysis()` 请求拼装改为：
   - 始终发送主模型 `base_url/api_key/model`（来自 AI settings）。
   - 当 Perplexity 开关开启且 key 存在时，额外发送：
     - `retrieval_base_url = defaultPerplexityBaseURL`
     - `retrieval_api_key = perplexityKey`
     - `retrieval_model = defaultPerplexityModel`
3. 设置校验改为“主模型必填优先”：即使开启 Perplexity，也必须先有主模型配置。

### 4.5 代码落点与影响范围

**后端（接口与业务）**
- `go-backend/internal/api/types.go`：symbol-analysis payload 入参扩展。
- `go-backend/internal/api/handlers.go`：普通/流式接口透传新增字段。
- `go-backend/pkg/investlog/ai_symbol_analysis_types.go`：请求结构扩展。
- `go-backend/pkg/investlog/ai_symbol_analysis_request.go`：新增字段归一化。
- `go-backend/pkg/investlog/ai_symbol_analysis_service.go`：摘要 provider 路由与降级核心改造。

**前端（页面请求层）**
- `static/modules/pages/symbol-analysis.js`：开关语义、请求字段拼装、主模型校验逻辑。

**测试（回归覆盖）**
- `go-backend/pkg/investlog/ai_symbol_analysis_test.go`：新增/调整检索增强路由与降级测试。
- `go-backend/internal/api/handlers_symbol_ai_test.go`：新增字段透传与兼容性测试。
- `go-backend/internal/api/handlers_ai_stream_test.go`：流式入口新增字段透传覆盖。

## 5. 兼容性

1. API 向后兼容：新增字段均为可选，旧客户端不传仍按旧路径执行（主模型全链路）。
2. 存储兼容：不改数据库 schema，不影响历史个股分析数据。
3. 行为兼容：仅在显式提供完整 retrieval 字段时改变摘要阶段 provider；其余行为保持不变。

## 6. 风险

1. **隐式降级导致用户感知不清**
   - 风险：retrieval 配置无效时自动回退，用户可能误以为已使用 Perplexity。
   - 缓解：后端 warning 日志附 `fallback_reason`；前端后续可考虑展示“检索增强未生效”提示（本次不强制）。
2. **外部 provider 稳定性波动**
   - 风险：Perplexity 抖动导致摘要失败。
   - 缓解：沿用现有 fallback summary，确保 framework+synthesis 主流程不中断。
3. **字段扩展带来的测试缺口**
   - 风险：普通接口与 SSE 接口行为不一致。
   - 缓解：同一组回归用例覆盖 POST 与 stream 两条入口。

## 7. 实施计划

1. 扩展 API payload 与 Core 请求结构（types + handlers + request struct）。
2. 在 `normalizeSymbolAnalysisRequest` 增加 retrieval 字段归一化。
3. 在 `analyzeSymbol` 实现摘要 provider 选择与降级逻辑，保持 framework+synthesis 走主模型。
4. 修改前端 symbol-analysis 请求拼装与文案语义。
5. 增补单元测试、接口测试、流式测试并回归 `go test ./...`。

## 8. 验证计划

### 8.1 单元测试（Core）

1. **检索增强生效**：传入完整 retrieval 字段，断言 `summarizeExternalDataFn` 收到 retrieval endpoint/key/model。
2. **主模型不变**：同一次请求中，断言 framework+synthesis 仍使用主模型 model。
3. **部分字段降级**：仅传 retrieval key 或 model，断言摘要回退主模型。
4. **非法 retrieval_base_url 降级**：断言不报错中断，摘要回退主模型。

### 8.2 API 测试

1. POST `/api/ai/symbol-analysis` 传 retrieval 三字段，响应 200 且结果 `model` 为主模型。
2. POST `/api/ai/symbol-analysis/stream` 传 retrieval 三字段，SSE 流包含 `result`/`done`，且无主流程回归。
3. 不传 retrieval 字段时行为与现状一致（回归旧用例）。

### 8.3 前端联调与手工验证

1. 关闭 Perplexity 开关：请求仅主模型字段。
2. 开启 Perplexity 开关：请求同时包含主模型字段与 retrieval 三字段。
3. 故意构造无效 retrieval_base_url：分析不失败，最终仍有 framework+synthesis 结果，日志出现回退警告。

### 8.4 验收对齐（MRD AC）

1. AC-1：摘要阶段使用 retrieval provider（可通过 stub/日志/断言验证）。
2. AC-2：framework+synthesis 保持主模型，返回 `model` 为主模型。
3. AC-3：新增回归测试稳定通过。
