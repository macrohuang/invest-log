# QA 测试用例 Spec：AI 分析增强（外部情报注入）

> 文件名：`docs/specs/ai-analysis/qa-ai-analysis.md`

## 1. 测试目标
- 对应 MRD：`docs/specs/ai-analysis/mrd-ai-analysis.md`
- 对应设计：`docs/specs/ai-analysis/design-ai-analysis.md`
- 验证新增外部情报层在标的分析与持仓分析中正确生效，且不引入回归。

## 2. 测试范围
- In Scope：
  - `AnalyzeSymbol` 与 `AnalyzeHoldings` 外部情报注入流程。
  - 来源阈值判定、缓存命中与过期重抓。
  - API 新字段解码与透传。
  - 响应中的 `external_context` 返回与持久化回读。
  - 设置页新增 external 参数保存与请求透传。
- Out of Scope：
  - 第三方网站长期可用性 SLA 验证。
  - 外部资讯内容质量的金融研究正确性评估。

## 3. 测试策略
- 单元测试（Go）：
  - provider 解析与清洗。
  - 缓存键计算与过期逻辑。
  - 阈值判定（来源去重后计数）。
  - prompt 注入断言（外部摘要确实进入分析 prompt）。
- 集成测试（Go + httptest）：
  - API 接口成功/失败路径。
  - schema 迁移后读写 `external_context`。
- 前端手工验证：
  - 设置页配置保存。
  - 分析结果 external 卡片渲染。

## 4. 用例清单
### 4.1 核心正向
- TC-001：来源 >= 阈值时标的分析成功
  - 前置：模拟 3 个来源返回
  - 预期：200；`external_context.source_count >= 2`；维度和综合结果正常
- TC-002：来源 >= 阈值时持仓分析成功
  - 前置：持仓存在，模拟 2+ 来源返回
  - 预期：200；返回组合建议 + `external_context`
- TC-003：缓存命中
  - 前置：第一次抓取后第二次同参数调用
  - 预期：第二次 `cache_hit=true`，不重复请求 provider

### 4.2 阈值与失败
- TC-004：来源不足阈值
  - 前置：仅 1 个有效来源，阈值=2
  - 预期：请求失败，错误含 `got 1, require 2`
- TC-005：部分 provider 失败但达阈值
  - 前置：4 个 provider 中 2 个失败，2 个成功
  - 预期：整体成功；日志记录失败 provider
- TC-006：全部 provider 失败
  - 前置：所有 provider 返回错误
  - 预期：请求失败，错误可读且包含失败摘要

### 4.3 参数与兼容
- TC-007：不传 external 参数
  - 预期：走默认值（enabled=true, min=2, cache=24）
- TC-008：非法 `external_min_sources`
  - 输入：0 或负数
  - 预期：400，参数校验错误
- TC-009：非法 `external_cache_hours`
  - 输入：0 或过大值
  - 预期：400，参数校验错误
- TC-010：旧数据回读兼容
  - 前置：历史 `symbol_analyses` 无 `external_context_json`
  - 预期：历史查询成功，不报错

### 4.4 数据库与迁移
- TC-011：老库启动迁移
  - 预期：新增列与缓存表创建成功
- TC-012：写入与读取一致
  - 前置：完成一次 symbol 分析
  - 预期：`external_context_json` 与 API 返回结构一致

### 4.5 前端验证
- TC-013：设置页保存 external 参数
  - 预期：localStorage 正确保存并回显
- TC-014：分析请求透传 external 参数
  - 预期：请求体包含 `external_*` 字段
- TC-015：结果卡片显示 external context
  - 预期：来源列表、抓取时间、摘要均可见

## 5. 自动化测试映射
- `go-backend/pkg/investlog/ai_symbol_analysis_test.go`
  - 新增：symbol 外部情报注入、阈值、缓存、回读断言
- `go-backend/pkg/investlog/ai_holdings_analysis_test.go`
  - 新增：holdings 外部情报注入、阈值、缓存断言
- `go-backend/internal/api/handlers_symbol_ai_test.go`
  - 新增：symbol 接口 external 参数透传与响应字段断言
- `go-backend/internal/api/handlers_ai_test.go`
  - 新增：holdings 接口 external 参数透传与响应字段断言
- `go-backend/pkg/investlog/schema_*_test.go`
  - 新增：迁移与新表/新列断言

## 6. 回归检查
- 持仓、交易、价格更新、历史分析查询无行为变化。
- 未启用 external 配置时，原有分析路径可用。

## 7. 覆盖率目标
- 新增后端逻辑（external 模块 + 接入逻辑）覆盖率 >= 80%。
- 关键失败分支（阈值不足、解析失败、超时）必须有自动化用例。

## 8. 退出标准
- P0/P1 用例全部通过。
- `go test ./...` 通过，无新增回归失败。
- 手工验证通过：设置页、标的分析、持仓分析、历史回读。

## 9. 缺陷记录模板
- Defect-ID：
- 场景：
- 复现步骤：
- 实际结果：
- 预期结果：
- 严重级别：

