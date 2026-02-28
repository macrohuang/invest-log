# Claude GO SDK + SSE MRD

## 1. 背景与目标
- 背景问题：当前 AI 调用统一走 OpenAI-compatible HTTP 请求，Claude 未使用官方 Go SDK；前端仅支持“整包返回”，无法逐块显示分析进度。
- 业务目标：Claude 路径切换到 `anthropic-sdk-go`，后端通过 SSE 逐块推送，前端实时渲染，降低等待焦虑并提升可观测性。
- 成功指标（可量化）：
  - Claude 请求链路 100% 通过 `anthropic-sdk-go` 发起。
  - 新增持仓分析 SSE 接口，前端可在响应完成前看到增量内容。
  - 保持现有 Settings 配置来源不变（API Key 与 Base URL 仍来自 Settings 页）。

## 2. 用户与场景
- 目标用户：使用 Claude 模型进行持仓分析的投资者。
- 核心使用场景：在 Holdings 页面点击 AI 后，界面实时看到分析文本逐步生成，最终得到结构化结果并落库。
- 非目标用户：仅使用非 Claude 模型且不需要流式展示的用户。

## 3. 范围定义
- In Scope：
  - 后端：Claude 调用接入 `anthropic-sdk-go`；新增 SSE 持仓分析接口；保持现有非流式接口兼容。
  - 前端：Holdings 页面 AI 分析改为消费 SSE，支持 chunk 渲染与完成态刷新。
  - 配置：沿用现有 Settings 的 `base_url` + `api_key`（不新增新配置项）。
- Out of Scope：
  - 非持仓页面（如 Symbol Analysis、Allocation Advice）的 UI 流式展示改造。
  - AI 设置项结构重构或数据库 schema 变更。

## 4. 需求明细
- 功能需求 FR-1：当模型/目标为 Claude 时，后端必须通过 `anthropic-sdk-go` 发起请求。
- 功能需求 FR-2：新增 `POST /api/ai/holdings-analysis/stream`，以 SSE 逐块返回分析内容与最终完成事件。
- 功能需求 FR-3：前端 `runAIHoldingsAnalysis` 支持 SSE 流读取，逐步更新页面展示，并在完成后写入原有状态结构。
- 非功能需求 NFR-1：保持错误可诊断性（SSE 中明确 error 事件，日志中包含上下文），且不泄露 API Key。

## 5. 约束与依赖
- 技术约束：Go 后端必须保持现有路由/handler 风格；日志继续使用 `slog`。
- 数据约束：不变更 `ai_settings` 存储结构；不持久化 API Key。
- 外部依赖：`github.com/anthropics/anthropic-sdk-go`。

## 6. 边界与异常
- 边界条件：
  - Settings 中 base URL 可能填写为根路径或带 `/v1` 路径，需要兼容归一化。
  - 非 Claude 模型调用 SSE 接口时，至少应返回可完成结果（可降级为单块事件）。
- 异常处理：
  - 上游错误通过 SSE `error` 事件返回；请求未开始流式前保持 HTTP 4xx/5xx。
- 失败回退：
  - 保留原 `POST /api/ai/holdings-analysis` 非流式接口作为回退路径。

## 7. 验收标准
- AC-1（可验证）：Claude 模型请求触发 SDK 路径，单测可验证文本增量事件被正确拼接。
- AC-2（可验证）：前端点击 Holdings AI 后，在完成前可见增量文本；完成后卡片显示最终结构化结果并可刷新历史。

## 8. 待确认问题
- Q1：本次是否需要把 Symbol Analysis / Allocation Advice 也升级为前端流式展示？（本次先按 out-of-scope 处理）
- Q2：SSE 事件格式采用 `chunk/done/error` 简化协议是否可接受？（本次先采用）

## 9. 假设与决策记录
- 假设：用户当前诉求优先是 Holdings 页面 Claude 分析体验升级。
- 决策：
  - 不新增 Settings 字段，复用现有 `base_url` 与 `api_key`。
  - 仅新增持仓分析流式接口；保留原接口不破坏兼容。
