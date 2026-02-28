# Claude GO SDK + SSE 变更总结

## 1. 变更概述
- 需求来源：用户要求按“意图驱动开发”改造 Claude 调用链，使用 `anthropic-sdk-go` 并通过 SSE 逐块返回，前端同步适配。
- 本次目标：
  - 后端 Claude 路径改为官方 SDK；
  - 新增持仓分析 SSE 接口；
  - Holdings 前端支持实时流式展示；
  - 保持 API Key / Base URL 仍来自 Settings。
- 完成情况：已完成并通过回归测试与覆盖率门槛。

## 2. 实现内容
- 代码变更点：
  - `go-backend/pkg/investlog/ai_holdings_analysis.go`
    - 新增 `AnalyzeHoldingsStream` 与流式内部执行路径。
    - 新增 `requestAIChatCompletionStream`。
    - Claude 请求自动走 `anthropic-sdk-go`（模型含 `claude` 或 endpoint 含 `anthropic`）。
    - 新增 Anthropic base URL 归一化与流式文本增量聚合。
  - `go-backend/internal/api/api.go`
    - 新增路由 `POST /api/ai/holdings-analysis/stream`。
  - `go-backend/internal/api/handlers.go`
    - 新增 SSE handler，输出 `chunk/done/error` 事件。
  - `static/app.js` 与 `ios/App/App/public/app.js`
    - 新增 SSE 读取与事件解析。
    - Holdings AI 调用改为流式 endpoint。
    - 新增流式状态管理与实时卡片渲染。
- 文档变更点：
  - 新增 `docs/specs/claude-go-sdk-sse/mrd-claude-go-sdk-sse.md`
  - 新增 `docs/specs/claude-go-sdk-sse/design-claude-go-sdk-sse.md`
  - 新增 `docs/specs/claude-go-sdk-sse/qa-claude-go-sdk-sse.md`
  - 新增 `docs/specs/claude-go-sdk-sse/summary-claude-go-sdk-sse.md`
- 关键设计调整：
  - 仅对 Holdings 页面做流式 UI 升级；非 Holdings 页面继续沿用现有非流式展示。
  - 非 Claude 模型经 stream endpoint 时，降级为单块输出 + done，保证兼容。

## 3. 测试与质量
- 已执行测试：
  - `cd go-backend && go test ./...`
  - `cd go-backend && go test -coverprofile=coverage_claude_sse.out ./pkg/investlog ./internal/api`
  - `cd go-backend && go tool cover -func=coverage_claude_sse.out`
- 测试结果：全部通过。
- 覆盖率结果（含命令与数值）：
  - `investlog/pkg/investlog`: `81.0%`
  - `investlog/internal/api`: `81.5%`
  - total: `81.1%`

## 4. 风险与已知问题
- 已知限制：
  - Symbol Analysis / Allocation Advice 前端暂未做流式 UI（本次 scope 外）。
- 风险评估：
  - SSE 在少数代理环境可能被缓冲；已通过 `X-Accel-Buffering: no` 与可选 flush 机制降低风险。
- 后续建议：
  - 若需要一致体验，可将 Symbol Analysis/Allocation Advice 页面也升级到同一 SSE 协议。

## 5. 待确认事项
- 需要 Reviewer 确认：
  - 是否接受本次仅升级 Holdings 页面流式 UI 的范围。
  - 是否需要下一步扩展到 Symbol/Advice 页面。
- 合并前阻塞项：
  - 等待用户审阅确认后再提交。
