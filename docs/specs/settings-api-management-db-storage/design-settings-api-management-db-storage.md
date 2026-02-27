# 技术设计：Settings API 管理非 Key 配置入库

## 1. 设计目标
- 对应 MRD：`docs/specs/settings-api-management-db-storage/mrd-settings-api-management-db-storage.md`
- 目标与非目标：
  - 目标：后端持久化并提供非 Key AI 配置；前端改为后端主存储 + 本地 API Key。
  - 非目标：密钥加密托管、多租户配置隔离。

## 2. 现状分析
- 相关模块：
  - 前端：`static/app.js` 中 `loadAIAnalysisSettings/saveAIAnalysisSettings`（`localStorage` 全量存储）。
  - 后端：`go-backend/internal/api` 缺少 AI 设置读写路由；`go-backend/pkg/investlog` 缺少对应表与 Core 方法。
- 当前实现限制：配置跨端不一致，且 API Key 与普通配置混存。

## 3. 方案概述
- 总体架构：
  - 新增 Core 层 `AISettings` 模型与 `GetAISettings/SetAISettings`。
  - 新增数据库表 `ai_settings`（单行，`id=1`）。
  - 新增 API 路由 `GET/PUT /api/ai-settings`。
  - 前端把 `loadAIAnalysisSettings` 改为异步：拉取后端非 Key 配置并注入本地 API Key。
- 关键流程：
  - 读取：前端 `GET /api/ai-settings` -> 合并本地 `apiKey` -> 渲染/分析请求。
  - 保存：前端读取表单 -> `PUT /api/ai-settings`（不含 key）+ 本地单独保存 `apiKey`。
- 关键设计决策：
  - 使用单行 upsert，避免多记录冲突。
  - API 层 payload 不接收/返回 API Key，减少误存储风险。

## 4. 详细设计
- 接口/API 变更：
  - `GET /api/ai-settings`
    - 响应：`base_url, model, risk_profile, horizon, advice_style, allow_new_symbols, strategy_prompt`
  - `PUT /api/ai-settings`
    - 请求：同上字段
    - 响应：保存后的设置对象
- 数据模型/存储变更：
  - 新增 `ai_settings` 表：
    - `id INTEGER PRIMARY KEY CHECK(id = 1)`
    - `base_url TEXT NOT NULL DEFAULT 'https://api.openai.com/v1'`
    - `model TEXT NOT NULL DEFAULT ''`
    - `risk_profile TEXT NOT NULL DEFAULT 'balanced'`
    - `horizon TEXT NOT NULL DEFAULT 'medium'`
    - `advice_style TEXT NOT NULL DEFAULT 'balanced'`
    - `allow_new_symbols INTEGER NOT NULL DEFAULT 1 CHECK(allow_new_symbols IN (0, 1))`
    - `strategy_prompt TEXT NOT NULL DEFAULT ''`
    - `updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`
  - 初始化时确保存在 `id=1` 默认行。
- 核心算法与规则：
  - Core 层对枚举值进行归一化（非法值回退默认）。
  - `base_url` 去尾斜杠，空值回退默认 OpenAI URL。
  - `allow_new_symbols` 与 DB 中 `0/1` 互转。
- 错误处理与降级：
  - 后端 DB 错误直接返回 500。
  - 前端读取失败时使用默认非 Key 配置 + 本地 API Key。

## 5. 兼容性与迁移
- 向后兼容性：
  - AI 分析请求体结构不变（仍传 `api_key`）。
  - 老版本本地 `aiHoldingsAnalysisSettings` 中的 `apiKey` 继续可读取（迁移兜底）。
- 数据迁移计划：
  - 无离线迁移脚本；应用启动时自动建表并写入默认行。
- 发布/回滚策略：
  - 回滚代码后，新表不影响旧逻辑（可忽略）。

## 6. 风险与权衡
- 风险 R1：后端设置接口暂不可用时，前端设置页体验退化。
- 规避措施：前端读取失败回退默认，并保持 API Key 本地可用。
- 备选方案与取舍：
  - 备选：继续全量本地存储并仅同步副本到后端。
  - 取舍：选择“后端为非 Key 主存储”，满足需求且行为更一致。

## 7. 实施计划
- 任务拆分：
  - A. `pkg/investlog` 新增模型、表初始化、Core 读写方法与测试。
  - B. `internal/api` 新增类型、handler、路由与接口测试。
  - C. `static/app.js` 改造设置加载/保存与 AI 分析调用链。
  - D. 运行测试与覆盖率验证，产出 summary。
- 里程碑：后端通过 -> 前端改造完成 -> 全量测试通过。
- 影响范围：AI 设置读取与保存路径、AI 分析触发路径。

## 8. 验证计划
- 与 QA Spec 的映射：每个 API 用例对应一个后端测试；前端流程做关键调用点校验。
- 关键验证点：
  - DB 不落 API Key。
  - Settings 保存后可回读。
  - `runAIHoldingsAnalysis/runSymbolAnalysis/allocation advice` 正常发送 `api_key`。
