# OpenAI Go Streaming Client 变更总结

## 目标
- 对 OpenAI 及兼容 OpenAI 标准的 AI 调用，改为使用 `openai-go` 原生客户端。
- 持仓分析、个股分析、AI 分配建议均支持流式返回（SSE）。
- 前端页面适配流式显示。
- `OPENAI_API_KEY` 与 `OPENAI_BASE_URL` 继续复用 Settings 配置。

## 核心实现
- 后端 AI 调用主路径切换为 `openai-go` 流式（`Chat.Completions.NewStreaming`）。
- 保留旧有 fallback 策略（chat/responses/alt/hybrid）以兼容不同上游实现和历史行为。
- 新增 `Core.AnalyzeHoldingsWithStream(...)`、`Core.AnalyzeSymbolWithStream(...)`、`Core.GetAllocationAdviceWithStream(...)`，通过 `OnDelta` 向上层输出增量文本。
- 新增 SSE API：
  - `POST /api/ai/holdings-analysis/stream`
  - `POST /api/ai/symbol-analysis/stream`
  - `POST /api/ai/allocation-advice/stream`
- 修复日志中间件对 `http.Flusher` 的透传，确保 SSE 在中间件链后可用。
- 前端新增/切换流式消费逻辑，实时展示 `progress`、`delta`、`result`、`error`、`done` 事件状态。

## 兼容与配置
- API key、base URL、model 继续从现有 Settings 页面输入并下发。
- `base_url` 兼容以下输入：
  - 裸域名（自动补全 `/v1`）
  - `/v1`、`/v1/chat/completions`、`/v1/responses`
- 保持原有非流式接口可用，前端逐步切换到流式端点。

## 测试与验证
- 回归测试：
  - `go test ./pkg/investlog ./internal/api -count=1`
- 覆盖率：
  - `go test -cover ./pkg/investlog ./internal/api -count=1`
  - `pkg/investlog`: `80.7%`
  - `internal/api`: `80.3%`

## 关键文件
- 后端：
  - `go-backend/pkg/investlog/ai_holdings_analysis.go`
  - `go-backend/internal/api/handlers.go`
  - `go-backend/internal/api/api.go`
  - `go-backend/internal/api/logging_middleware.go`
  - `go-backend/go.mod`
- 前端：
  - `static/app.js`
  - `static/style.css`
- 测试：
  - `go-backend/pkg/investlog/ai_holdings_analysis_test.go`
  - `go-backend/pkg/investlog/ai_openai_client_test.go`
  - `go-backend/pkg/investlog/ai_allocation_advice_test.go`
  - `go-backend/internal/api/handlers_ai_stream_test.go`
