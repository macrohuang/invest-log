# Gemini-only AI 服务 MRD

## 1. 背景与目标
- 背景问题：当前 AI 能力同时兼容 OpenAI-compatible、Gemini、Perplexity 等多种 Provider，后端存在多套请求协议与回退逻辑，前端也暴露了多个 Provider 相关配置项，增加了维护成本、调试复杂度和错误面。
- 业务目标：将全部 AI 相关能力统一收口到 Gemini 协议，并通过 AICodeMirror 的 Gemini 兼容入口访问，降低配置复杂度并统一运行行为。
- 成功指标（可量化）：
  - 所有 `/api/ai/*` 能力仅通过 Gemini 协议发起请求。
  - 设置页不再暴露 Perplexity 专属配置项。
  - 个股分析接口不再接收 `retrieval_*` 字段。
  - 受影响自动化测试全部通过，AI 相关改动覆盖率达到或超过 80%。

## 2. 用户与场景
- 目标用户：使用 InvestLog 进行持仓分析、配置建议和个股分析的个人投资者。
- 核心使用场景：
  - 用户在设置页配置 AI Base URL、Model、API Key 后，发起持仓分析、配置建议、个股分析。
  - 历史用户升级后，旧的 OpenAI/Perplexity 风格配置会自动迁移为 Gemini 可用配置，无需手工清理旧 Provider。
- 非目标用户：需要在应用内自由切换多个 AI Provider 的高级集成用户。

## 3. 范围定义
- In Scope：
  - 后端 AI 传输层统一为 Gemini-only。
  - AI 设置默认值与归一化逻辑改为 AICodeMirror Gemini。
  - 移除个股分析中的 retrieval provider 配置与调用链路。
  - 更新 Web/iOS 设置与调用组装逻辑，使其与 Gemini-only 契约一致。
  - 更新 README 与 specs 文档。
- Out of Scope：
  - 新增温度、最大输出 token 等高级可调配置。
  - 新增新的 AI provider 或 provider 切换能力。
  - 对 AI 提示词本身做大规模重构。

## 4. 需求明细
- 功能需求 FR-1：所有 AI 请求只允许使用 Gemini 模型与 Gemini 协议端点，认证头固定为 `x-goog-api-key`。
- 功能需求 FR-2：默认 AI Base URL 为 `https://api.aicodemirror.com/api/gemini`，但允许高级用户编辑 Gemini 兼容地址。
- 功能需求 FR-3：持仓分析、配置建议、个股分析（含流式）统一经 Gemini SSE 端点发起请求。
- 功能需求 FR-4：个股分析接口删除 `retrieval_base_url`、`retrieval_api_key`、`retrieval_model` 字段，并且不再依赖第二 provider 进行外部信息摘要。
- 功能需求 FR-5：读取与保存 AI 设置时，旧的 OpenAI/Perplexity 风格配置自动归一化到 Gemini 可用配置。
- 非功能需求 NFR-1（可靠性）：非 Gemini 配置必须被明确拒绝或自动归一化，不能悄悄落回其他 provider 分支。
- 非功能需求 NFR-2（可维护性）：删除无效 provider 分支和 UI 配置项，减少协议分叉。

## 5. 约束与依赖
- 技术约束：
  - 保持现有 `/api/ai/*` 业务接口尽量稳定，避免扩大前后端改动面。
  - 继续使用现有 SSE 事件封装与 JSON 解析结果结构。
- 数据约束：
  - AI 设置仍沿用现有 `ai_settings` 表字段；本次尽量避免新增数据库 schema。
  - 旧配置迁移以读取/保存归一化为主，不引入离线批量迁移任务。
- 外部依赖：
  - AICodeMirror Gemini 兼容网关。
  - Gemini `streamGenerateContent` SSE 响应格式。

## 6. 边界与异常
- 边界条件：
  - 用户未填写 model 或 api_key 时，仍按现有行为提示先配置。
  - 用户填写 OpenAI/Perplexity 风格 base URL 或非 Gemini 模型时，需要归一化或返回清晰错误。
  - 用户直接调用旧的 symbol-analysis retrieval 字段时，应因 unknown field 或契约变化而失败。
- 异常处理：
  - Gemini 端点拼接失败、请求失败、SSE 解析失败时，返回可读错误并保留日志脱敏。
  - 若旧配置无法安全归一化，则在保存或调用阶段返回仅支持 Gemini 的错误。
- 失败回退：
  - 不再回退到 OpenAI-compatible、responses、hybrid 或 Perplexity provider。

## 7. 验收标准
- AC-1（可验证）：任一 AI 功能发起请求时，请求路径均为 Gemini `.../v1beta/models/{model}:streamGenerateContent?alt=sse`，且携带 `x-goog-api-key`。
- AC-2（可验证）：设置页不再展示 `Perplexity API Key`，个股分析请求不再发送 retrieval 字段。
- AC-3（可验证）：默认 AI 设置返回 AICodeMirror Gemini 基地址；旧 OpenAI/Perplexity 风格配置会被归一化。
- AC-4（可验证）：受影响 Go 测试通过，AI 相关覆盖率达到或超过 80%。

## 8. 待确认问题
- Q1：AICodeMirror 网关是否需要除 `x-goog-api-key` 外的额外 header？当前按现有示例只使用 `Content-Type` + `x-goog-api-key`。
- Q2：同步接口是否完全通过 SSE 聚合实现？当前决策为是，以减少协议分叉。

## 9. 假设与决策记录
- 假设：AICodeMirror Gemini 网关兼容当前仓库已实现的 Gemini SSE 解码逻辑，或仅需小幅适配字段结构。
- 假设：前端和 iOS 镜像代码都需要同步更新，且不引入新的前端测试框架。
- 决策：Gemini 收口覆盖全部 AI 功能，包括个股分析的外部信息摘要链路。
- 决策：默认 Base URL 使用 AICodeMirror Gemini，但仍允许用户编辑 Gemini 兼容地址。
- 决策：Perplexity 专属设置和 retrieval provider 字段从产品层面移除并停用。
