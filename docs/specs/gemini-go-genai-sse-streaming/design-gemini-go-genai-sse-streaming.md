# Gemini 原生客户端 + SSE 流式返回 技术设计

> 文件名：`docs/specs/gemini-go-genai-sse-streaming/design-gemini-go-genai-sse-streaming.md`

## 1. 设计目标
- 对应 MRD：`docs/specs/gemini-go-genai-sse-streaming/mrd-gemini-go-genai-sse-streaming.md`
- 目标与非目标：
  - 目标：Gemini 走 go-genai；Holdings 页面端到端 SSE 流式。
  - 非目标：symbol-analysis 页面改造；数据库 schema 变更。

## 2. 现状分析
- 相关模块：
  - 后端 AI 传输与解析：`go-backend/pkg/investlog/ai_holdings_analysis.go`
  - AI 路由与 handler：`go-backend/internal/api/api.go`, `go-backend/internal/api/handlers.go`
  - 前端 holdings AI 入口：`static/app.js` 的 `runAIHoldingsAnalysis()`
- 当前实现限制：
  - 后端使用 OpenAI 兼容 HTTP 请求，`stream=false`。
  - 前端 `fetchJSON` 一次性等待结果，无法展示生成过程。

## 3. 方案概述
- 总体架构：
  - 新增 Gemini 专用调用分支：识别 Gemini 请求后用 go-genai 客户端调用。
  - 新增 holdings SSE 接口：后端将 token/chunk 通过 SSE 推送给前端。
  - 前端新增 SSE 读取与解析器：实时更新“生成中”卡片，`done` 后落盘最终结果。
- 关键流程：
  1. 前端 `AI` 按钮调用 `/api/ai/holdings-analysis/stream`（POST + JSON body）。
  2. handler 初始化 SSE 响应并回写 `start`。
  3. Core `AnalyzeHoldingsStream` 在 Gemini 路径中逐块回调文本；handler 推送 `chunk`。
  4. Core 完成后返回结构化结果；handler 推送 `done`。
  5. 前端收到 `done` 后刷新历史并恢复按钮状态。
- 关键设计决策：
  - SSE 使用 `fetch + ReadableStream` 而非 `EventSource`，以支持 POST body（包含 settings 派生参数）。
  - Gemini 识别采用 model/base_url 双信号，提高兼容性。
  - 复用现有 `parseHoldingsAnalysisResponse` 与持久化逻辑，降低回归风险。

## 4. 详细设计
- 接口/API 变更：
  - 新增 `POST /api/ai/holdings-analysis/stream`。
  - SSE 事件协议：
    - `start`: `{ model, currency }`
    - `chunk`: `{ text }`
    - `done`: `{ result: HoldingsAnalysisResult }`
    - `error`: `{ error }`
- 数据模型/存储变更：无。
- 核心算法与规则：
  - `isGeminiRequest(baseURL, model)`：
    - model 以 `gemini` 开头，或
    - baseURL host/path 包含 `googleapis.com` / `generativelanguage` / `gemini`。
  - Gemini 调用：`go-genai` stream API，回调中仅发送非空增量。
  - 完成后聚合全文 -> JSON 清洗解析 -> 归一化 -> 保存 DB。
- 错误处理与降级：
  - go-genai 调用失败直接返回业务错误；SSE 下用 `error` 事件输出。
  - 非 Gemini 模型继续走原 `requestAIChatCompletion` 分支。

## 5. 兼容性与迁移
- 向后兼容性：
  - 旧接口 `/api/ai/holdings-analysis` 保持可用。
  - 非 Gemini 模型调用链不变。
- 数据迁移计划：无。
- 发布/回滚策略：
  - 发布：后端 + `static/app.js/style.css` + `scripts/sync_spa.sh` 同步 iOS。
  - 回滚：回退新增 SSE 路由和前端流式逻辑即可。

## 6. 风险与权衡
- 风险 R1：go-genai API 结构与当前抽象不一致导致集成复杂。
- 规避措施：将 go-genai 调用封装在独立函数，保留可替换变量用于测试注入。
- 风险 R2：频繁 chunk 触发前端重渲染导致卡顿。
- 规避措施：前端仅更新流式文本节点或轻量状态，不触发表格全量重建。
- 备选方案与取舍：
  - 备选：继续用兼容 `/chat/completions` 的 `stream=true`。
  - 取舍：按需求明确使用 go-genai 原生客户端。

## 7. 实施计划
- 任务拆分：
  1. 后端单测先行：Gemini 判定、SSE handler 基础协议。
  2. 集成 go-genai 并实现 `AnalyzeHoldingsStream`。
  3. 新增路由和 handler SSE 写出逻辑。
  4. 前端改造 `runAIHoldingsAnalysis` 为 SSE 消费。
  5. 样式补充（生成中态）并同步 iOS 资源。
  6. 跑测试、覆盖率、补 summary 文档。
- 里程碑：后端可流式输出 -> 前端可实时展示 -> 回归通过。
- 影响范围：AI holdings 分析链路（后端 + 静态前端）。

## 8. 验证计划
- 与 QA Spec 的映射：
  - TC-001/002/003 验证 SSE 协议与前端渲染。
  - TC-004/005 验证旧接口与非 Gemini 回归。
- 关键验证点：
  - SSE 事件顺序完整。
  - done 结果可正常落库并在历史接口可见。
  - Settings 来源逻辑未改变。
