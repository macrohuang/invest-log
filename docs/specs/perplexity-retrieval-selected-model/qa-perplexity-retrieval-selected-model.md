# QA 测试设计：个股分析 Perplexity 检索增强（主模型不切换）

## 1. 测试目标
- 对应 MRD：`docs/specs/perplexity-retrieval-selected-model/mrd-perplexity-retrieval-selected-model.md`
- 验证 FR-1/FR-2/FR-3 与 NFR-1：
  - 个股分析支持可选 `retrieval_base_url/retrieval_api_key/retrieval_model`。
  - 检索增强配置完整时，仅外部信息摘要阶段使用检索配置。
  - 框架分析与综合分析始终使用主模型配置。
  - 检索配置异常或不完整时自动回退主模型，主流程不失败。

## 2. 测试范围
- In Scope：
  - 后端核心：`go-backend/pkg/investlog` 个股分析模型路由与回退。
  - API 层：`/api/ai/symbol-analysis`、`/api/ai/symbol-analysis/stream` 参数解码与透传。
  - 回归：结果 `model` 字段、主流程状态、已有参数校验行为。
- Out of Scope：
  - 持仓分析（`AnalyzeHoldings`）模型路由。
  - 新增第三方检索源或前端视觉交互。

## 3. 测试策略
- 单元测试（主）：
  - 在 `pkg/investlog` 通过 stub `summarizeExternalDataFn` / `aiChatCompletion` 记录调用参数，断言“摘要阶段模型”和“框架/综合模型”路由差异。
  - 对“不完整检索字段”“非法 endpoint”做回退分支断言。
- 集成测试：
  - 在 `internal/api` 使用 `httptest` 模拟 AI 上游，覆盖同步与流式接口请求。
  - 断言接口成功返回、事件序列（stream）和模型字段无回归。
- 回归策略：
  - 主模型必填校验保持不变（`api_key/model/symbol/currency`）。
  - 未开启检索增强时行为与历史一致。

## 4. 测试用例
| 用例ID | 类型 | 优先级 | 场景与步骤 | 预期结果 | 自动化映射（建议） |
| --- | --- | --- | --- | --- | --- |
| TC-001 | 正向 | P0 | 开启检索增强：传入完整 `retrieval_*`，执行个股分析 | 外部摘要使用检索 endpoint/key/model；分析成功 | `go-backend/pkg/investlog/ai_symbol_analysis_test.go`：`TestAnalyzeSymbol_UsesRetrievalConfigForExternalSummary_WhenRetrievalFieldsComplete` |
| TC-002 | 正向 | P1 | 开启检索增强并走流式接口 `/api/ai/symbol-analysis/stream` | SSE 正常输出 `progress/result/done`，结果可用 | `go-backend/internal/api/handlers_ai_stream_test.go`：`TestAISymbolAnalysisStreamEndpoint_RetrievalEnabled_Success` |
| TC-003 | 异常 | P0 | 检索字段不完整（如仅 `retrieval_api_key` 无 `retrieval_model`） | 忽略检索配置，摘要回退主模型，主流程成功 | `go-backend/pkg/investlog/ai_symbol_analysis_test.go`：`TestAnalyzeSymbol_FallsBackToPrimaryModel_WhenRetrievalFieldsIncomplete` |
| TC-004 | 异常 | P1 | API 请求中传入部分 `retrieval_*` 字段 | 接口不因检索字段不完整而 4xx；最终走主模型 | `go-backend/internal/api/handlers_symbol_ai_test.go`：`TestSymbolAnalysisEndpoint_PartialRetrievalFields_FallbackToPrimary` |
| TC-005 | 边界 | P0 | `retrieval_base_url` 非法（无法归一化） | 记录 warning；摘要自动回退主模型；分析仍完成 | `go-backend/pkg/investlog/ai_symbol_analysis_test.go`：`TestAnalyzeSymbol_FallsBackToPrimaryModel_WhenRetrievalEndpointInvalid` |
| TC-006 | 回归 | P0 | 开启检索增强，观察框架分析与综合分析调用参数 | 框架与综合阶段始终使用主模型，不被检索模型覆盖 | `go-backend/pkg/investlog/ai_symbol_analysis_test.go`：`TestAnalyzeSymbol_FrameworkAndSynthesis_KeepPrimaryModel_WhenRetrievalEnabled` |
| TC-007 | 回归 | P0 | 开启检索增强后检查响应体 | 返回结果 `model` 字段保持主模型 | `go-backend/internal/api/handlers_symbol_ai_test.go`：`TestSymbolAnalysisEndpoint_ResponseModel_RemainsPrimaryModel_WhenRetrievalEnabled` |
| TC-008 | 回归 | P1 | 不传 `retrieval_*` 调用同步/流式接口 | 与历史行为一致，现有用例持续通过 | `go-backend/internal/api/handlers_symbol_ai_test.go`、`go-backend/internal/api/handlers_ai_stream_test.go`（扩展现有 success/missing-key 用例） |

## 5. 覆盖率策略（目标 >= 80%）
- 覆盖门槛：
  - 变更相关包总体覆盖率 >= 80%。
  - P0 用例必须 100% 自动化。
- 建议命令：
```bash
cd go-backend

# 1) 先跑目标回归
go test -v ./pkg/investlog ./internal/api -run 'TestAnalyzeSymbol|TestSymbolAnalysisEndpoint|TestAISymbolAnalysisStreamEndpoint'

# 2) 统计覆盖率
go test ./pkg/investlog ./internal/api -coverprofile=coverage_perplexity_retrieval_selected_model.out
go tool cover -func=coverage_perplexity_retrieval_selected_model.out

# 3) CI 门禁（<80 失败）
go tool cover -func=coverage_perplexity_retrieval_selected_model.out | awk '/total:/ { if ($3+0 < 80) exit 1 }'
```

## 6. 退出标准
- P0 用例全部通过（TC-001/TC-003/TC-005/TC-006/TC-007）。
- P1 用例通过或有已批准的风险豁免记录。
- `go test ./pkg/investlog ./internal/api` 全量通过。
- 覆盖率达到或超过 80%。

## 7. 缺陷记录
- Defect-ID：
- 对应用例：
- 发现版本/日期：
- 复现步骤：
- 实际结果：
- 预期结果：
- 严重级别（P0/P1/P2）：
- 根因分析：
- 修复与回归结论：
