# 个股 AI 多框架分析变更总结

## 1. 变更概述
- 需求来源：用户要求在 `ai_symbol_analysis.go` 中改为“先抓财报与宏观产业经营信息，再从 9 个框架中按标的特征选 3 个分析，并输出顶部综合建议”。
- 本次目标：
  - 实现 9 选 3 框架并行分析；
  - 综合建议纳入持仓数量/仓位占比/配置区间/用户偏好与策略；
  - 强制输出硬约束（首句结论、明确概率、禁含混措辞、短 disclaimer）。
- 完成情况：已完成实现、测试与覆盖率门禁（>=80%）。

## 2. 实现内容
- 代码变更点：
  - `go-backend/pkg/investlog/ai_symbol_analysis.go`
    - 新增 9 框架目录与选择逻辑 `selectSymbolFrameworks`，按标的与上下文打分并稳定选出 3 个。
    - 框架并行执行改造：`runDimensionAgents` 由固定四维改为选中框架并行，失败不足 3 个即报错。
    - 框架输出字段增强：`SymbolDimensionResult` 增加 `suggestion`，结果 `dimensions` 的 key 为框架 ID。
    - 综合权重上下文增强：纳入 `total_shares/position_percent/allocation_* / risk/horizon/advice/strategy_prompt`。
    - 强约束归一化：`normalizeSynthesisResult` + `normalizeSynthesisSummary` + `normalizeSynthesisDisclaimer`，保证结论、概率、禁词、免责声明长度。
    - 持久化兼容：不改 schema，通过 `mapDimensionOutputsToLegacyColumns` 映射写入 legacy 4 列；读取时 `buildSymbolAnalysisResult` 优先使用 `parsed.Dimension` 恢复框架 ID。
  - `go-backend/pkg/investlog/ai_symbol_data_fetcher.go`
    - 外部摘要重构为“先抓取”结构：
      - 近5个季度财报
      - 近3年年报
      - 行业宏观政策
      - 产业周期
      - 公司最新经营
      - 数据缺口
    - 新增 `normalizeStructuredExternalSummary` 与 `buildFallbackStructuredExternalSummary`，保证缺口可见且可降级。
- 文档变更点：
  - `docs/specs/ai-symbol-analysis-multi-framework/mrd-ai-symbol-analysis-multi-framework.md`
  - `docs/specs/ai-symbol-analysis-multi-framework/design-ai-symbol-analysis-multi-framework.md`
  - `docs/specs/ai-symbol-analysis-multi-framework/qa-ai-symbol-analysis-multi-framework.md`
- 关键设计调整：
  - 动态框架选择替代固定四维；
  - 在不改 DB schema 前提下保留回读一致性；
  - 以后置归一化兜底模型不稳定输出。

## 3. 测试与质量
- 已执行测试：
  - `go test ./pkg/investlog ./internal/api`
  - `go test ./pkg/investlog ./internal/api -coverprofile=coverage_ai_symbol_multi_framework.out`
  - `go tool cover -func=coverage_ai_symbol_multi_framework.out | tail -n 8`
- 测试结果：全部通过。
- 覆盖率结果（含命令与数值）：
  - `total: (statements) 81.4%`
  - 达到 >=80% 门禁。

## 4. 风险与已知问题
- 已知限制：
  - 外部源不稳定时，结构化摘要可能更多依赖 fallback 规则抽取。
  - 9 框架中的选择仍是启发式打分，不是离线训练模型。
- 风险评估：中低；有明确降级路径与归一化兜底。
- 后续建议：
  - 增加线上样本回放评估框架选择准确率。
  - 若前端强依赖框架字段，可在后续版本引入独立持久化列以提升可观测性。

## 5. 待确认事项
- 需要 Reviewer 确认：
  - 9 选 3 的启发式规则与当前业务预期是否一致。
  - `disclaimer<=16` 的默认风险锚点文案是否接受。
- 合并前阻塞项：无（测试与覆盖率已达标）。
