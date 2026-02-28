# Gemini 原生客户端 + SSE 流式返回 变更总结

> 文件名：`docs/specs/gemini-go-genai-sse-streaming/summary-gemini-go-genai-sse-streaming.md`

## 1. 变更概述
- 需求来源：用户要求“Gemini 调用使用 Go 原生客户端（go-genai），并通过 SSE 逐块返回，前端同步适配；API Key 与 Base URL 继续来自 Settings”。
- 本次目标：
  - Gemini 路径切换到 `google.golang.org/genai`；
  - 新增 holdings AI 分析 SSE 接口并提供 chunk/done/error 事件；
  - 前端 holdings 页面实时显示流式输出。
- 完成情况：已完成后端、前端、iOS 静态资源同步、测试与覆盖率达标。

## 2. 实现内容
- 代码变更点：
  - 后端（`pkg/investlog`）：
    - 新增 `AnalyzeHoldingsStream` 与统一内部流程 `analyzeHoldings`；
    - 新增 `requestAIChatCompletionStream`；
    - 新增 Gemini 原生分支 `requestAIByGeminiNative`；
    - 新增 Gemini 识别与 baseURL/version 解析函数（`isGeminiRequest`、`parseGeminiBaseURLAndVersion`）。
  - 后端 API（`internal/api`）：
    - 新增路由 `POST /api/ai/holdings-analysis/stream`；
    - 新增 SSE handler：`start/chunk/done/error` 事件输出。
  - 前端（`static/app.js`, `static/style.css`）：
    - 新增 SSE 读取器（`streamSSE`）与事件解析；
    - `runAIHoldingsAnalysis` 改为调用 SSE 接口；
    - 新增“生成中”流式卡片与 chunk 实时追加；
    - 错误处理和完成态刷新逻辑适配。
  - iOS：已执行 `scripts/sync_spa.sh` 同步 `static` 到 `ios/App/App/public`。
- 文档变更点：新增 MRD/设计/QA/Summary 四份文档。
- 关键设计调整：
  - Gemini 请求通过 go-genai 直接调用 Gemini API，不再走 OpenAI 兼容 HTTP 分支；
  - SSE 采用 `fetch + ReadableStream`（支持 POST body）。

## 3. 测试与质量
- 已执行测试：
  - `go test ./pkg/investlog ./internal/api`
  - `go test ./pkg/investlog ./internal/api -coverprofile=coverage_gemini_sse.out`
  - `go tool cover -func=coverage_gemini_sse.out`
- 测试结果：通过。
- 覆盖率结果（含命令与数值）：
  - `pkg/investlog`: 80.3%
  - `internal/api`: 81.6%
  - 总体（两包合并）：80.5%

## 4. 风险与已知问题
- 已知限制：
  - 本次仅对 holdings AI 分析做 SSE 前端适配，symbol-analysis 仍为一次性返回。
- 风险评估：
  - 低到中。已通过新增单测覆盖 Gemini 识别、baseURL/version 解析、原生客户端非流/流分支、SSE handler 基础行为。
- 后续建议：
  - 若用户继续要求，下一步可将 symbol-analysis 也升级为 SSE。

## 5. 待确认事项
- 需要 Reviewer 确认：
  - Holdings 页面流式体验与文案是否符合预期；
  - Settings 中不同 Gemini base_url 配置的兼容范围是否满足现网使用。
- 合并前阻塞项：无（等待你评审确认）。
