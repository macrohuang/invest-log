# 变更总结：Perplexity 检索增强与主模型分离

## 1. 变更概述
- 需求来源：用户反馈“个股分析勾选 Perplexity 后，不应替换主模型做最终综合分析”。
- 本次目标：让 Perplexity 仅用于最新信息检索/摘要增强，框架分析与综合分析保持主模型。
- 完成情况：已完成后端路由改造、前端请求语义改造、回归测试补齐并通过。

## 2. 实现内容
- 代码变更点：
  - `go-backend/pkg/investlog/ai_symbol_analysis_types.go`
    - `SymbolAnalysisRequest` 新增 `RetrievalBaseURL/RetrievalAPIKey/RetrievalModel`。
  - `go-backend/pkg/investlog/ai_symbol_analysis_request.go`
    - 新增 retrieval 字段归一化（trim）。
  - `go-backend/pkg/investlog/ai_symbol_analysis_service.go`
    - 新增 `resolveExternalSummaryProvider`。
    - `summarizeExternalDataFn` 改为可使用 retrieval provider。
    - 当外部抓取为空或摘要为空时，新增 `retrieveLatestSymbolContext` 主动走 retrieval provider 做实时检索补充，避免被跳过。
    - framework + synthesis 仍使用主模型。
  - `go-backend/internal/api/types.go`
    - `aiSymbolAnalysisPayload` 增加 `retrieval_*` 字段。
  - `go-backend/internal/api/handlers.go`
    - 普通/流式个股分析接口透传 `retrieval_*` 字段。
  - `static/modules/pages/symbol-analysis.js`
    - Perplexity 开关语义改为“检索增强”。
    - 请求始终携带主模型 `base_url/api_key/model`；
    - 开启开关时额外附带 `retrieval_base_url/retrieval_api_key/retrieval_model`。
- 文档变更点：
  - 新增 MRD：`mrd-perplexity-retrieval-selected-model.md`
  - 新增技术设计：`design-perplexity-retrieval-selected-model.md`
  - 新增 QA 设计：`qa-perplexity-retrieval-selected-model.md`
  - 新增变更总结：`summary-perplexity-retrieval-selected-model.md`
- 关键设计调整：
  - 摘要阶段 provider 可独立配置并降级回主模型。
  - 返回结果中的 `model` 继续表示主模型，不被检索增强模型覆盖。

## 3. 测试与质量
- 已执行测试：
  - `go test ./pkg/investlog -run 'TestAnalyzeSymbol_UsesRetrievalProviderOnlyForExternalSummary|TestResolveExternalSummaryProvider_FallsBackWhenRetrievalConfigIncomplete|TestResolveExternalSummaryProvider_FallsBackWhenRetrievalBaseURLInvalid|TestNormalizeSymbolAnalysisRequest'`
  - `go test ./pkg/investlog -run 'TestAnalyzeSymbol_UsesRetrievalProviderWhenExternalDataMissing'`
  - `go test ./internal/api -run 'TestSymbolAnalysisEndpoint_Success|TestAISymbolAnalysisStreamEndpoint_Success'`
  - `go test ./pkg/investlog ./internal/api`
  - `go test ./pkg/investlog ./internal/api -coverprofile=coverage_perplexity_retrieval_selected_model.out && go tool cover -func=coverage_perplexity_retrieval_selected_model.out`
- 测试结果：全部通过。
- 覆盖率结果（含命令与数值）：
  - 统计命令：同上 `coverprofile` 命令
  - `pkg/investlog`: `81.7%`
  - `internal/api`: `80.1%`
  - total: `81.4%`（>=80%）

## 4. 风险与已知问题
- 已知限制：
  - 前端当前仅支持 Perplexity 作为 retrieval provider（字段本身是通用的）。
- 风险评估：
  - retrieval 配置非法时会自动回退主模型；功能正确，但用户侧可见性依赖日志。
- 后续建议：
  - 可在 UI 增加“检索增强是否生效”的可视化提示（例如运行状态提示或结果元信息）。

## 5. 待确认事项
- 需要 Reviewer 确认：
  - Perplexity 按钮文案 `Perplexity Retrieval` 是否符合产品语义预期。
  - 回退策略（静默回退 + warning 日志）是否需要提升为前端显式提示。
- 合并前阻塞项：
  - 无代码阻塞项。
