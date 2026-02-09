# 技术设计：AI 持仓分析与建议

> 文件名：`docs/specs/design-ai-holdings-analysis.md`

## 1. 设计目标
- 对应 MRD：`docs/specs/mrd-ai-holdings-analysis.md`
- 目标与非目标：
  - 目标：新增 OpenAI 兼容分析能力，形成“持仓快照 -> AI 分析 -> 结构化建议 -> UI 展示”的闭环。
  - 非目标：不新增 DB 表、不做自动交易、不做异步任务队列。

## 2. 现状分析
- 相关模块：
  - API 路由与 Handler：`go-backend/internal/api/api.go`、`go-backend/internal/api/handlers.go`、`go-backend/internal/api/types.go`
  - 领域核心：`go-backend/pkg/investlog/*`
  - 前端持仓页：`static/app.js`、`static/style.css`
- 当前实现限制：
  - 无 AI 接口与数据结构。
  - 无持仓分析 UI 操作入口。

## 3. 方案概述
- 总体架构：
  1. 前端在 Holdings 页触发 `POST /api/ai/holdings-analysis`。
  2. 后端校验配置，构建持仓快照与系统提示词。
  3. 调用 OpenAI 兼容 Chat Completions 接口。
  4. 解析模型 JSON，返回结构化分析结果给前端。
  5. 前端渲染分析卡片（总结 + 关键发现 + 建议列表）。
- 关键流程：同步请求（无队列），失败直接回传错误。
- 关键设计决策：
  - 在 `investlog` 包新增独立文件 `ai_holdings_analysis.go`，避免污染已有价格逻辑。
  - 使用类型化请求/响应结构，边界强校验。
  - 使用上下文超时控制（20s）保护外部调用。
  - 通过可替换函数变量注入测试桩，避免单测依赖外网。

## 4. 详细设计
- 接口/API 变更：
  - 新增路由：`POST /api/ai/holdings-analysis`
  - 请求体：
    - `base_url` string（可选）
    - `api_key` string（必填）
    - `model` string（必填）
    - `currency` string（可选）
    - `risk_profile` string（必填，默认 `balanced`）
    - `horizon` string（必填，默认 `medium`）
    - `allow_new_symbols` bool（默认 `true`）
    - `advice_style` string（默认 `balanced`）
  - 响应体：
    - `generated_at`、`model`、`currency`
    - `overall_summary`、`risk_level`、`key_findings[]`
    - `recommendations[]`（symbol/action/theory_tag/rationale/target_weight/priority）
    - `disclaimer`
- 数据模型/存储变更：无。
- 核心算法与规则：
  - 持仓快照来自 `GetHoldingsBySymbol()`，按币种过滤。
  - 提示词中嵌入三种理念约束，要求 JSON 输出。
  - 解析时清理 markdown code fence，再进行 JSON 反序列化。
  - Base URL 规范化：支持直接给 `/v1` 或完整 `/chat/completions`。
- 错误处理与降级：
  - 参数缺失：返回 400。
  - 上游失败：包装错误并返回 400（handler 语义沿用现有业务错误返回策略）。
  - JSON 不可解析：返回错误提示“模型返回格式不符合预期”。

## 5. 兼容性与迁移
- 向后兼容性：
  - 新增接口与按钮，不影响现有接口和数据结构。
- 数据迁移计划：无。
- 发布/回滚策略：
  - 若异常，可回滚新增路由与前端按钮，不影响数据库。

## 6. 风险与权衡
- 风险 R1：不同 OpenAI 兼容服务响应格式差异。
- 规避措施：
  - 放宽对响应字段的最小依赖（只取首个 choice 的 message.content）。
  - 增加 code fence 清理与错误提示。
- 备选方案与取舍：
  - 备选：前端直连模型服务。
  - 取舍：采用后端中转以复用持仓数据拼装并统一错误处理。

## 7. 实施计划
- 任务拆分：
  1. 后端类型与核心分析逻辑。
  2. API 路由与 handler 接入。
  3. Holdings 页面按钮、配置输入、结果渲染。
  4. 单元测试与接口测试。
- 里程碑：
  - M1：API 可返回结构化结果。
  - M2：前端可触发并展示。
  - M3：测试通过且覆盖率达标。
- 影响范围：`go-backend/internal/api`、`go-backend/pkg/investlog`、`static/*`。

## 8. 验证计划
- 与 QA Spec 的映射：见 `docs/specs/qa-ai-holdings-analysis.md`。
- 关键验证点：
  - 参数校验分支。
  - Base URL 归一化分支。
  - 模型返回解析（正常 JSON / code fence / 非法 JSON）。
  - 前端触发流程与结果展示。
