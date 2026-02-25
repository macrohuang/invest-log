# 技术设计：AI 分析增强（外部情报注入）

> 文件名：`docs/specs/ai-analysis/design-ai-analysis.md`

## 1. 设计目标
- 对应 MRD：`docs/specs/ai-analysis/mrd-ai-analysis.md`
- 构建统一的外部情报层，服务 `AnalyzeSymbol` 与 `AnalyzeHoldings`。
- 在现有同步请求模型下，完成“抓取 -> 提炼 -> 分析 -> 回传/持久化”。
- 保持现有接口兼容，新增字段均为可选且有默认值。

## 2. 现状与改造点
- 现状：
  - `AnalyzeSymbol` 直接以持仓上下文驱动四维分析 + 综合分析。
  - `AnalyzeHoldings` 直接以持仓快照进行组合分析。
  - 无外部数据抓取、无时效控制、无来源追溯字段。
- 改造点：
  1. 新增外部情报采集与缓存模块。
  2. 新增 AI 预提炼流程（pre-summary）。
  3. 将提炼结果注入后续分析 prompt。
  4. 结果结构和持久化扩展 `external_context`。

## 3. 总体架构
### 3.1 组件
- `ExternalIntelService`：统一编排抓取、阈值判定、缓存与预提炼。
- `ExternalSourceProvider`：白名单来源适配器（按市场与分类分组）。
- `ExternalIntelSummarizer`：将原始文档压缩为结构化摘要。
- `ExternalIntelCacheStore`：外部情报缓存读写层（SQLite）。

### 3.2 流程
1. 归一化请求参数（包含 external 配置）。
2. 生成抓取请求（symbol/holdings、市场、分类、时间窗）。
3. 读取缓存，命中且未过期则直接复用。
4. 并发调用白名单 provider 抓取文档。
5. 按来源域名去重，计算有效来源数。
6. 若来源数 `< min_sources`，返回错误中止分析。
7. 调用 AI summarizer 生成 `external_context`。
8. 将 `external_context` 注入维度/综合（或组合）分析 prompt。
9. 返回分析结果并持久化（symbol_analyses + cache）。

## 4. 数据结构与接口
### 4.1 请求结构扩展
- `SymbolAnalysisRequest` / `HoldingsAnalysisRequest` 新增：
  - `ExternalDataEnabled bool`
  - `ExternalMinSources int`
  - `ExternalCacheHours int`
  - `ExternalCategories []string`

### 4.2 外部情报结构
- 新增类型（建议放 `ai_external_intel.go`）：
  - `ExternalDoc`：`title/url/source/type/published_at/content_snippet`
  - `ExternalContext`：
    - `fetched_at`
    - `cache_hit`
    - `min_sources`
    - `source_count`
    - `sources[]`
    - `summary`
    - `highlights[]`
    - `risks[]`
    - `opportunities[]`
    - `coverage`

### 4.3 结果结构扩展
- `SymbolAnalysisResult` 新增：`ExternalContext *ExternalContext `json:"external_context,omitempty"``
- `HoldingsAnalysisResult` 新增：`ExternalContext *ExternalContext `json:"external_context,omitempty"``

## 5. API 变更
### 5.1 请求新增字段（两条 AI 分析接口一致）
- `external_data_enabled`（默认 `true`）
- `external_min_sources`（默认 `2`）
- `external_cache_hours`（默认 `24`）
- `external_categories`（默认 `filing/announcement/news/research`）

### 5.2 响应新增字段
- `external_context`：用于前端展示来源和提炼摘要。

## 6. 数据库设计
### 6.1 现有表扩展
- `symbol_analyses` 新增列：
  - `external_context_json TEXT`
  - `external_sources_count INTEGER`
  - `external_fetched_at DATETIME`

### 6.2 新增缓存表
- `external_intel_cache`：
  - `id INTEGER PRIMARY KEY AUTOINCREMENT`
  - `cache_key TEXT NOT NULL UNIQUE`
  - `scope TEXT NOT NULL`（`symbol` / `holdings`）
  - `payload_json TEXT NOT NULL`
  - `sources_count INTEGER NOT NULL`
  - `fetched_at DATETIME NOT NULL`
  - `expires_at DATETIME NOT NULL`
  - 索引：`idx_external_intel_cache_expires_at`

### 6.3 迁移策略
- 在 `schema.go` 采用增量迁移：
  - `tableHasColumn` 检查后 `ALTER TABLE` 补列。
  - `CREATE TABLE IF NOT EXISTS external_intel_cache`。

## 7. 关键规则
### 7.1 市场识别
- 优先级：`symbols.exchange` > symbol 格式规则 > currency 兜底。

### 7.2 来源阈值
- 按“去重后的来源域名数”计算有效来源。
- 若 `< external_min_sources`，错误示例：
  - `external intel insufficient sources: got 1, require 2`

### 7.3 缓存键
- `symbol`：`symbol|currency|categories|min_sources|model_version`
- `holdings`：`currency|top_symbols_hash|categories|min_sources|model_version`

### 7.4 Prompt 注入
- `AnalyzeSymbol`：外部摘要注入四维 agent 与 synthesis agent 的用户 prompt。
- `AnalyzeHoldings`：注入组合级外部摘要 + Top 持仓摘要。

## 8. 前端改造
- 文件：`static/app.js`（并同步 `ios/App/App/public/app.js`）。
- 设置页 AI 模块新增控件：
  - External Data 开关
  - Min Sources 选择
  - Cache Hours 选择
  - Categories 复选
- 请求透传新增字段。
- 结果卡片新增 `External Context` 展示（来源列表、抓取时间、摘要）。

## 9. 错误处理与日志
- Provider 级错误记录 warning，不立即终止。
- 编排层最终按阈值统一判定成功/失败。
- 记录日志字段：`symbol/currency/scope/source_count/min_sources/cache_hit/latency_ms`。

## 10. 安全与合规
- 仅允许白名单来源，禁止用户输入任意来源 URL。
- 抓取内容仅保留必要摘要字段，避免存储超长原文。
- 对外只暴露已清洗来源与摘要，不回传完整抓取正文。

## 11. 实施顺序
1. 后端：外部情报模块 + 缓存表 + schema 迁移。
2. 后端：`AnalyzeSymbol` 接入外部情报并持久化。
3. 后端：`AnalyzeHoldings` 接入外部情报。
4. API：payload/response 扩展与 handler 透传。
5. 前端：设置页与结果渲染。
6. 测试：单测/集成/回归补齐。

## 12. 兼容性
- 新字段全可选，旧客户端不传时按默认值执行。
- 旧历史数据无 `external_context` 时保持可读，不影响既有功能。

