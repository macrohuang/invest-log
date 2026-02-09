# QA 测试用例 Spec：AI 持仓分析与建议

> 文件名：`docs/specs/qa-ai-holdings-analysis.md`

## 1. 测试目标
- 对应 MRD：`docs/specs/mrd-ai-holdings-analysis.md`
- 对应设计文档：`docs/specs/design-ai-holdings-analysis.md`

## 2. 测试范围
- In Scope：
  - 后端 `AnalyzeHoldings` 核心逻辑与 URL 归一化。
  - API 端点 `/api/ai/holdings-analysis` 的成功与参数校验分支。
  - 前端 Holdings 页面触发分析并展示结果。
- Out of Scope：
  - 第三方模型真实质量评估。
  - 不同服务商账户计费/限流策略。

## 3. 测试策略
- 单元测试：
  - `go-backend/pkg/investlog/ai_holdings_analysis_test.go`
  - 覆盖参数校验、响应解析、异常路径。
- 集成测试：
  - `go-backend/internal/api/handlers_ai_test.go`
  - 使用 `httptest.Server` 模拟 OpenAI 兼容接口。
- 端到端/手工验证：
  - 启动服务后在 Holdings 页面点击 `AI Analyze` 验证交互与渲染。

## 4. 用例清单
- TC-001（正向）：
  - 前置：存在持仓，模型服务返回合法 JSON。
  - 步骤：调用新 API。
  - 预期：200，响应含 summary/key_findings/recommendations。
- TC-002（异常）：
  - 前置：`api_key` 为空。
  - 步骤：调用新 API。
  - 预期：400，提示 `api_key is required`。
- TC-003（边界）：
  - 前置：`base_url` 无 `/v1`。
  - 步骤：调用分析。
  - 预期：自动补齐为 `/v1/chat/completions` 并可成功请求。
- TC-004（回归）：
  - 前置：既有 holdings/transactions 功能正常。
  - 步骤：运行相关测试套件。
  - 预期：历史接口无回归失败。

## 5. 覆盖率策略
- 统计口径（命令/工具）：
  - `cd go-backend && go test ./... -coverprofile=coverage.out`
  - `cd go-backend && go tool cover -func=coverage.out`
- 当前覆盖盲区：
  - 前端渲染细节主要依赖手工验证。
- 提升计划（目标 >=80%）：
  - 为核心解析/归一化/错误分支补足单测。
  - 为 API 入口的成功与失败路径增加集成测试。

## 6. 退出标准
- 所有 P0/P1 用例通过。
- 自动化测试稳定通过。
- 覆盖率达到或超过 80%（若全仓覆盖不可达，至少保障新增逻辑 >80% 并说明原因）。

## 7. 缺陷记录
- Defect-1：待执行后记录。
