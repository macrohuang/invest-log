# Claude GO SDK + SSE 技术设计

## 1. 设计目标
- 对应 MRD：`mrd-claude-go-sdk-sse.md`
- 目标与非目标：
  - 目标：Claude 请求改为官方 SDK；持仓分析新增 SSE 流输出；前端实现增量消费。
  - 非目标：Symbol/Advice 页面流式 UI 改造；Settings 存储结构调整。

## 2. 现状分析
- 相关模块：
  - `go-backend/pkg/investlog/ai_holdings_analysis.go`（统一 AI 请求与持仓分析）
  - `go-backend/internal/api/handlers.go`、`api.go`（HTTP 路由）
  - `static/app.js`（Holdings AI 触发与渲染）
- 当前实现限制：
  - 后端对 Claude 无官方 SDK 专用路径。
  - 前端仅 `fetchJSON` 全量返回，无流式能力。

## 3. 方案概述
- 总体架构：
  - 在 `ai_holdings_analysis.go` 新增 Claude SDK 调用与流式聚合函数。
  - 新增 `Core.AnalyzeHoldingsStream(...)`：在流式过程中回调增量文本，结束后解析 JSON 并持久化。
  - API 新增 `POST /api/ai/holdings-analysis/stream`：以 SSE 输出 `chunk/done/error`。
  - 前端新增 `fetchSSE` 解析器，替换 Holdings AI 的调用链。
- 关键流程：
  - 前端 POST stream endpoint -> 后端调用 SDK `Messages.NewStreaming` -> 每个 `TextDelta` 推送 chunk -> 完成后推送 done(最终结果)。
- 关键设计决策：
  - Claude 判断规则：模型名含 `claude` 或 base URL 指向 anthropic。
  - 非 Claude 请求走原逻辑；若通过 stream endpoint 调用，降级发送单个 chunk + done。

## 4. 详细设计
- 接口/API 变更：
  - 新增 `POST /api/ai/holdings-analysis/stream`。
  - SSE 事件：
    - `event: chunk` + `data: {"delta":"..."}`
    - `event: done` + `data: {"result":<HoldingsAnalysisResult>}`
    - `event: error` + `data: {"error":"..."}`
- 数据模型/存储变更：
  - 无 schema 变更；完成后沿用 `saveHoldingsAnalysis`。
- 核心算法与规则：
  - SDK 流事件中读取 `ContentBlockDeltaEvent` + `TextDelta`，累计文本缓冲。
  - 完成后对累计文本执行现有 `parseHoldingsAnalysisResponse` 与 normalize 流程。
- 错误处理与降级：
  - SDK 流 error -> SSE `error` 并记录日志。
  - 非 Claude 或 SDK 不适配 -> 使用原非流式函数并推送单块结果。

## 5. 兼容性与迁移
- 向后兼容性：
  - 原 `POST /api/ai/holdings-analysis` 不变。
  - 前端仅改 Holdings AI 入口到新 stream endpoint。
- 数据迁移计划：无。
- 发布/回滚策略：
  - 回滚可仅恢复前端调用旧 endpoint，后端新增路由可保留不使用。

## 6. 风险与权衡
- 风险 R1：SSE 在某些容器环境的 flush 行为不稳定。
- 规避措施：后端显式 `Flusher` + 事件边界；前端增加 stream 失败回退提示。
- 备选方案与取舍：
  - 备选：WebSocket；
  - 取舍：SSE 更轻量，符合单向增量输出场景。

## 7. 实施计划
- 任务拆分：
  - T1：增加后端 SDK 流式能力与单测。
  - T2：增加 SSE handler/route 与 API 测试。
  - T3：前端 stream 解析与 Holdings 页面增量渲染。
  - T4：同步 iOS public 资源。
- 里程碑：后端测试通过 -> 前端联调 -> 全量回归。
- 影响范围：AI 相关 Go 模块、API handler、`static/app.js` 与 iOS 镜像文件。

## 8. 验证计划
- 与 QA Spec 的映射：详见 `qa-claude-go-sdk-sse.md`。
- 关键验证点：
  - SDK 流事件可累积成合法 JSON。
  - SSE 协议格式正确且前端可稳定消费。
  - 完成后结果落库并可在历史接口读取。
