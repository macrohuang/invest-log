# MRD：OpenAI Go 原生流式调用与前端流式适配

## 1. 背景与目标
- 背景问题：当前 AI 调用为手写 HTTP 非流式实现，前端只能在完成后一次性展示结果，等待时间长且无过程反馈。
- 业务目标：统一改为 `openai-go` 原生客户端流式调用，并在前端页面实时展示生成进度/增量内容。
- 成功指标（可量化）：
  - 所有核心 AI 分析链路（持仓分析、个股分析、仓位建议中的共享 AI 调用）改为 `openai-go`。
  - Holdings/Symbol 分析页面支持流式展示，用户在请求开始后 2s 内看到首个进度事件（本地/测试环境）。
  - 现有 AI 相关回归单测通过。

## 2. 用户与场景
- 目标用户：在 Invest Log 中使用 AI 分析能力的普通投资用户。
- 核心使用场景：
  - 在 Holdings 页面触发 AI 分析，边生成边看到增量文本，完成后自动展示结构化结果。
  - 在 Symbol Analysis 页面触发分析，看到阶段性进度并在完成后刷新结果。
- 非目标用户：仅使用账本录入、不使用 AI 功能的用户。

## 3. 范围定义
- In Scope：
  - 后端 AI 调用切换到 `github.com/openai/openai-go`。
  - 使用流式 API 获取增量内容并在后端聚合为最终文本。
  - 提供 SSE 接口给前端消费流式事件。
  - 前端 Holdings/Symbol 分析入口的流式 UI 适配。
  - Settings 配置来源保持不变：`OPENAI_API_KEY` / `OPENAI_BASE_URL` 仍从 Settings 读取并透传。
- Out of Scope：
  - 新增/修改 AI 业务提示词策略。
  - 新增多模型路由或重试策略平台化。
  - 完整前端样式重设计。

## 4. 需求明细
- 功能需求 FR-1：AI 请求必须通过 `openai-go` 发起，支持 OpenAI 与兼容 OpenAI 标准的 Base URL。
- 功能需求 FR-2：AI 请求必须使用流式读取增量 token，并最终返回完整文本给现有解析逻辑。
- 功能需求 FR-3：后端提供可消费的 SSE 事件流，至少包含 `delta`、`progress`、`result`、`error`、`done` 事件。
- 功能需求 FR-4：前端在 Holdings 与 Symbol 分析页面可实时呈现流式内容与阶段状态。
- 非功能需求 NFR-1：错误信息可追踪（`slog` 结构化日志），并确保请求超时与取消行为可控。

## 5. 约束与依赖
- 技术约束：Go 1.22+；必须沿用当前后端路由与前端 SPA 架构。
- 数据约束：不改动现有 AI 结果存储表结构。
- 外部依赖：`github.com/openai/openai-go`。

## 6. 边界与异常
- 边界条件：用户 Base URL 可能输入为域名根、`/v1`、`/chat/completions`、`/responses` 等形式。
- 异常处理：上游返回非 2xx、流中断、超时、内容为空都需返回明确错误。
- 失败回退：SSE 失败时前端给出错误提示，不阻塞已有历史分析查看。

## 7. 验收标准
- AC-1（可验证）：触发 Holdings 分析时，前端能收到并展示流式 `delta/progress`，最终拿到 `result`。
- AC-2（可验证）：触发 Symbol 分析时，前端能收到阶段进度事件，完成后页面刷新并展示最新分析。
- AC-3（可验证）：Settings 中配置的 `baseUrl/model/apiKey` 仍为唯一来源，无新增配置入口。

## 8. 待确认问题
- Q1：Symbol 分析是否需要显示每个维度的完整 token 流，还是仅阶段性进度即可。
- Q2：Allocation Advice 页面是否也需要后续升级为前端流式展示。

## 9. 假设与决策记录
- 假设：本次“前端页面适配升级”优先覆盖 Holdings 与 Symbol 两个主分析入口。
- 决策：共享 `aiChatCompletion` 底层统一切到 `openai-go` 流式，减少重复改造与行为不一致。
