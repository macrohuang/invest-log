# 技术设计：OpenAI Go 原生流式调用与前端流式适配

## 1. 设计目标
- 对应 MRD：`docs/specs/openai-go-streaming-client/mrd-openai-go-streaming-client.md`
- 目标：
  - 统一 AI 调用栈为 `openai-go`。
  - 通过流式事件打通后端到前端的实时反馈。
- 非目标：重构 AI 业务逻辑与数据库结构。

## 2. 现状分析
- 相关模块：
  - 后端：`pkg/investlog/ai_holdings_analysis.go`（AI 调用核心）、`ai_symbol_analysis.go`、`ai_allocation_advice.go`。
  - API：`internal/api/handlers.go`。
  - 前端：`static/app.js`。
- 当前实现限制：
  - 手写 HTTP + 非流式，前端无增量状态。
  - 路由只返回最终 JSON。

## 3. 方案概述
- 总体架构：
  - `aiChatCompletion` 改为基于 `openai-go` 的 Chat Completions Streaming。
  - 增加可选流回调 `OnDelta`，供上层（SSE handler）订阅增量。
  - API 新增/扩展 SSE 分析接口，把增量与阶段状态推送到前端。
- 关键流程：
  - 前端 POST 到流式端点 -> 后端写入 `text/event-stream` -> Core 内部 `OnDelta` 回调推送 `delta` -> 解析完成后推送 `result`。
- 关键设计决策：
  - 兼容 Base URL：将 Settings 的 `base_url` 规范化为客户端 `base`（去掉 `/chat/completions`/`/responses` 尾缀）。
  - 不改业务返回结构：最终 `result` 仍复用现有结构化结果。

## 4. 详细设计
- 接口/API 变更：
  - 新增（或扩展）SSE endpoint：
    - `POST /api/ai/holdings-analysis/stream`
    - `POST /api/ai/symbol-analysis/stream`
  - 事件格式：
    - `event: progress` + `{stage,message}`
    - `event: delta` + `{text}`
    - `event: result` + `{...existing result json...}`
    - `event: error` + `{error}`
    - `event: done` + `{ok:true}`
- 数据模型/存储变更：无。
- 核心算法与规则：
  - `requestAIChatCompletion` 使用 `openai-go` streaming 逐片读取 `delta` 并拼接完整文本。
  - 结束后复用现有 JSON 清洗与反序列化逻辑。
- 错误处理与降级：
  - 流创建失败/中断：返回 `ai request failed` 类错误。
  - 空内容：返回 `ai response content is empty`。
  - SSE 写入失败：记录 warn 并中断当前请求上下文。

## 5. 兼容性与迁移
- 向后兼容性：
  - 原非流式端点保留，避免已有调用方中断。
  - 前端分析入口改为流式端点，历史加载接口不变。
- 数据迁移计划：无。
- 发布/回滚策略：
  - 若异常可回滚到旧分支版本；不涉及 schema 迁移。

## 6. 风险与权衡
- 风险 R1：兼容服务对 OpenAI 流字段实现不完整。
- 规避措施：
  - 保留严格错误提示并记录原始错误。
  - Base URL 规范化兼容更多输入形式。
- 备选方案与取舍：
  - 方案 B：继续手写 HTTP 流解析。取舍：不符合“使用 Go 原生客户端”要求，放弃。

## 7. 实施计划
- 任务拆分：
  - T1：引入 `openai-go` 并替换 AI 客户端。
  - T2：新增 SSE handler 与事件协议。
  - T3：前端分析页面流式渲染。
  - T4：补充/更新测试与文档。
- 里程碑：后端单测通过 -> 前端手工链路可用 -> 覆盖率检查达标。
- 影响范围：`go-backend/pkg/investlog`、`go-backend/internal/api`、`static/app.js`。

## 8. 验证计划
- 与 QA Spec 的映射：TC-001~TC-006。
- 关键验证点：
  - `openai-go` streaming 被调用。
  - SSE 事件顺序正确。
  - 前端可见增量输出并最终落地结果。
