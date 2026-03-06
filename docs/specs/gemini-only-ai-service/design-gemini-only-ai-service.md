# 技术设计：Gemini-only AI Service

> 文件名：`docs/specs/gemini-only-ai-service/design-gemini-only-ai-service.md`

## 1. 设计目标
- 对应 MRD：`docs/specs/gemini-only-ai-service/mrd-gemini-only-ai-service.md`
- 目标：
  - 将全部 AI 能力统一收口到 Gemini 协议。
  - 默认通过 AICodeMirror Gemini 代理访问：`https://api.aicodemirror.com/api/gemini`。
  - 删除 OpenAI / Responses / Perplexity / Hybrid 等 provider 分支，减少传输层复杂度。
  - 个股分析移除 `retrieval_*` 次级 provider 配置，统一复用主 Gemini 配置。
  - 前端与 iOS 镜像 UI 改为 Gemini-only，移除 Perplexity 专属设置。
- 非目标：
  - 不新增新的 AI 功能。
  - 不引入新的数据库表或复杂迁移脚本。
  - 不新增前端可调 `temperature` / `maxOutputTokens` 配置页。

## 2. 现状分析
- 相关模块：
  - 后端 AI 传输层：`go-backend/pkg/investlog/ai_chat_client.go`
  - AI 设置：`go-backend/pkg/investlog/ai_settings.go`
  - 个股分析：`go-backend/pkg/investlog/ai_symbol_analysis_types.go`、`go-backend/pkg/investlog/ai_symbol_analysis_service.go`
  - API 类型/处理器：`go-backend/internal/api/types.go`、`go-backend/internal/api/handlers.go`
  - Web UI：`static/modules/ai-settings.js`、`static/modules/pages/settings.js`、`static/modules/pages/holdings.js`、`static/modules/pages/symbol-analysis.js`
  - iOS 镜像：`ios/App/App/public/app.js`
- 当前实现限制：
  - `ai_chat_client.go` 同时支持 chat-completions、responses、hybrid fallback、Gemini SSE，逻辑分叉多。
  - `AISettings` 默认值仍偏 OpenAI 风格。
  - 个股分析支持 `retrieval_base_url` / `retrieval_api_key` / `retrieval_model`，实际形成双 provider 模式。
  - 设置页仍展示 “Gemini supported” 与 Perplexity API Key，产品语义与目标不一致。

## 3. 方案概述
- 总体架构：
  - 保留现有业务层 API（持仓分析 / 配置建议 / 个股分析 / stream）；
  - 仅替换统一 AI 传输层，使所有上游请求都转为 Gemini `streamGenerateContent?alt=sse`；
  - 同步接口在服务端复用 SSE 聚合完整文本，再走现有 JSON 解析。
- 关键流程：
  1. 读取 AI 设置。
  2. 将 `base_url` 和 `model` 归一化为 Gemini-only 配置。
  3. 后端拼接最终 Gemini SSE URL：`{normalizedBaseURL}/v1beta/models/{model}:streamGenerateContent?alt=sse`
  4. 发送请求头 `x-goog-api-key`，请求体使用 Gemini `systemInstruction + contents + generationConfig`。
  5. 流式接口逐块转发；非流式接口聚合文本并返回现有业务响应。
- 关键设计决策：
  - 默认 provider 固定为 AICodeMirror Gemini，但保留 `base_url` 可编辑能力。
  - 不再接受非 Gemini 模型/地址作为有效运行时配置。
  - 自动迁移旧配置，优先保证用户“已有设置可继续用”，而非让用户手工修正。
  - 个股分析外部摘要直接复用主 Gemini 模型，不再维护次级 provider 概念。

## 4. 详细设计

### 4.1 接口/API 变更
- `GET /api/ai-settings`
  - 返回字段保持不变：`base_url`、`model`、`api_key`、`risk_profile`、`horizon`、`advice_style`、`allow_new_symbols`、`strategy_prompt`
  - 行为变更：
    - 默认 `base_url` 改为 `https://api.aicodemirror.com/api/gemini`
    - 若数据库中为 OpenAI/Perplexity 旧地址，返回前归一化为 Gemini 默认地址
- `PUT /api/ai-settings`
  - 入参字段保持不变
  - 保存时校验并归一化：
    - `model` 必须为 Gemini 模型名（如 `gemini-2.5-flash`）
    - `base_url` 为空或为旧 provider 地址时，自动改写为 Gemini 默认地址
- `POST /api/ai/symbol-analysis`
  - 删除请求字段：`retrieval_base_url`、`retrieval_api_key`、`retrieval_model`
  - 仅使用主模型配置 `base_url` / `api_key` / `model`
- `POST /api/ai/symbol-analysis/stream`
  - 同上删除 `retrieval_*`
- 其他 AI 接口：
  - 不新增字段
  - 非 Gemini 配置统一返回 4xx，错误文案明确说明“仅支持 Gemini 配置”

### 4.2 数据模型/存储变更
- `AISettings`
  - 保持结构不变，避免 DB schema 变更
  - 仅修改默认值与归一化逻辑
- `SymbolAnalysisRequest`
  - 删除：
    - `RetrievalBaseURL`
    - `RetrievalAPIKey`
    - `RetrievalModel`
- API payload 类型同步删除上述字段。
- 数据库存量数据不做离线 migration：
  - 采用“读取时归一化 + 保存时覆盖”的软迁移策略。

### 4.3 核心算法与规则

#### A. Gemini-only URL 归一化
- 输入允许：
  - 空值
  - `https://api.aicodemirror.com/api/gemini`
  - 已带 `/v1beta`
  - 已带 `/v1beta/models/...:streamGenerateContent`
  - 历史 OpenAI / Perplexity 地址
- 归一化规则：
  - 空值或旧 provider 地址 -> `https://api.aicodemirror.com/api/gemini`
  - 去掉尾部 `/`
  - 去掉已存在的 `/v1beta`、`/models/...`、`:streamGenerateContent`
  - 最终统一由后端追加 `/v1beta/models/{model}:streamGenerateContent?alt=sse`

#### B. Gemini 请求体统一
- 统一构造：
  - `systemInstruction.parts[0].text = SystemPrompt`
  - `contents[0].parts[0].text = UserPrompt`
  - `generationConfig.temperature = 默认值`
  - `generationConfig.maxOutputTokens = 默认值`
- 不再生成 OpenAI chat-completions / responses payload。

#### C. 同步与流式共用同一实现
- 流式：
  - 直接消费 Gemini SSE
  - 解析增量文本，透传到现有 SSE envelope
- 同步：
  - 复用流式 Gemini 请求器
  - 聚合完整文本后复用既有 `cleanupModelJSON`、业务 JSON decode 逻辑

#### D. 个股分析检索链路简化
- 删除：
  - `resolveExternalSummaryProvider`
  - `isRetrievalProviderConfigured`
  - `isResolvedRetrievalProvider`
  - 所有 retrieval fallback 判断
- `retrieveLatestSymbolContext` 直接使用主 Gemini endpoint/apiKey/model
- 效果：
  - 仍保留“先整理外部事实、再多框架分析、再综合”的流程
  - 但 provider 只有一个，配置面更简单

### 4.4 错误处理与降级
- 配置错误：
  - `model` 非 Gemini -> 返回明确校验错误
  - `api_key` 缺失 -> 保持现有必填校验
- 上游错误：
  - 优先解析 Gemini 返回体中的错误消息
  - 超时仍保留专门提示
- 不再做：
  - chat -> responses fallback
  - responses -> hybrid fallback
  - Perplexity retrieval fallback
- 这样会减少“自动兜底成功率”，但换来行为确定性和更强可观测性。

## 5. 兼容性与迁移
- 向后兼容性：
  - 后端业务 API 路径不变。
  - `GET/PUT /api/ai-settings` 字段名不变。
  - 唯一显式破坏性变更是 `symbol-analysis` 与 `symbol-analysis/stream` 不再接受 `retrieval_*` 字段。
- 数据迁移计划：
  - 不做 schema migration。
  - 通过 `normalizeAISettings()` 在读写时把旧 provider 地址自动切换为 Gemini 默认地址。
  - 前端加载设置时继续走 normalize，确保旧本地缓存也被收口。
- 发布/回滚策略：
  - 发布后若出现 Gemini 请求兼容性问题，可回滚整个 task branch。
  - 不依赖 DB 结构变更，回滚成本低。

## 6. 风险与权衡
- 风险 R1：AICodeMirror 代理与 Google 官方 Gemini SSE 细节可能存在响应差异。
  - 规避措施：以现有 Gemini SSE 测试为基线，新增 AICodeMirror base URL 归一化测试。
- 风险 R2：删除 retrieval provider 后，个股分析“实时信息丰富度”可能下降。
  - 规避措施：保留外部摘要步骤，只是改为主 Gemini 模型完成。
- 风险 R3：旧前端/移动端缓存仍携带 Perplexity 设置。
  - 规避措施：加载与保存时都做 Gemini-only normalize，并移除 UI 入口。
- 备选方案与取舍：
  - 备选：仅后端强制 Gemini，保留 UI 旧字段。
  - 取舍：放弃该方案，因为会继续制造产品误导和测试负担。

## 7. 实施计划
- 任务拆分：
  1. 先补失败测试：AI settings 默认值、Gemini URL 归一化、symbol analysis 移除 retrieval 字段、非 Gemini 配置拒绝
  2. 精简 `ai_chat_client.go` 为 Gemini-only 传输层
  3. 更新 settings / symbol analysis 类型与 handler
  4. 更新 SPA 与 iOS 镜像设置页、个股分析页
  5. 更新 README 与交付文档
- 里程碑：
  - M1：测试先红
  - M2：后端通过
  - M3：前端/iOS 同步
  - M4：全量测试与覆盖率达标
- 影响范围：
  - Go 后端 AI 相关包
  - Web SPA 设置/AI 页面
  - iOS 镜像静态资源
  - README 与 specs 文档

## 8. 验证计划
- 与 QA Spec 的映射：
  - 配置归一化 -> QA 配置类用例
  - Gemini-only transport -> AI 请求链路用例
  - symbol-analysis 去 retrieval -> API 兼容性与回归用例
  - 前端设置页变更 -> 手工 UI 回归
- 关键验证点：
  - `x-goog-api-key` 始终被发送且日志中被脱敏
  - 最终请求路径始终为 `.../v1beta/models/{model}:streamGenerateContent?alt=sse`
  - `GET /api/ai-settings` 默认返回 AICodeMirror Gemini 基地址
  - `POST /api/ai/symbol-analysis*` 不再依赖 `retrieval_*`
  - `static/` 与 `ios/App/App/public/` 行为一致

## 9. 测试映射
- 单元测试：
  - `ai_chat_client.go`：URL 归一化、Gemini 请求头、请求体编码、错误解析
  - `ai_settings.go`：默认值与旧配置自动迁移
  - `ai_symbol_analysis_service.go`：移除 retrieval provider 后仍能完成外部摘要与综合分析
- 集成/API 测试：
  - `handlers_ai_settings_test.go`：默认 Gemini base URL
  - `handlers_symbol_ai_test.go` / `handlers_ai_stream_test.go`：移除 retrieval 字段后的正向与异常路径
  - `handlers_ai_test.go`：持仓/配置建议接口仍走 Gemini endpoint
- 手工验证：
  - Settings 页只显示 Gemini 相关配置
  - 个股分析/持仓分析/配置建议在桌面 SPA 与 iOS 镜像中都能正常发起请求
