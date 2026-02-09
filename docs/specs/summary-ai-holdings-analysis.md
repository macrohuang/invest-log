# 变更总结：AI 持仓分析与建议

> 文件名：`docs/specs/summary-ai-holdings-analysis.md`

## 1. 变更概述
- 需求来源：新增“基于马尔基尔 + 达利欧 + 巴菲特理念”的 AI 持仓分析与建议能力。
- 本次目标：在不改数据库结构的前提下，完成从 Holdings 页面触发到后端分析返回的闭环。
- 完成情况：已完成 MRD、设计、QA Spec、功能开发与自动化测试，并达到覆盖率门槛。

## 2. 实现内容
- 代码变更点：
  - 后端新增 AI 分析核心能力：`go-backend/pkg/investlog/ai_holdings_analysis.go`
  - 新增 API 入口：`POST /api/ai/holdings-analysis`
  - 新增/扩展类型与 Handler：`go-backend/internal/api/types.go`、`go-backend/internal/api/handlers.go`、`go-backend/internal/api/api.go`
  - 前端 Holdings 页新增 `AI Analyze` 按钮、配置输入、结果卡片渲染：`static/app.js`、`static/style.css`
- 文档变更点：
  - 需求：`docs/specs/mrd-ai-holdings-analysis.md`
  - 设计：`docs/specs/design-ai-holdings-analysis.md`
  - QA：`docs/specs/qa-ai-holdings-analysis.md`
  - 总结：本文档
- 关键设计调整：
  - 采用 OpenAI 兼容协议 + 可配置 `base_url/model/api_key`。
  - API Key 仅前端本地存储，不落库。
  - 对模型输出做 JSON 约束与 code-fence 清理，增强兼容性。

## 3. 测试与质量
- 已执行测试：
  - `cd go-backend && go test ./...`
  - `cd go-backend && go test ./pkg/investlog ./internal/api -coverprofile=coverage_ai.out`
  - `cd go-backend && go tool cover -func=coverage_ai.out`
- 测试结果：全通过。
- 覆盖率结果（含命令与数值）：
  - `pkg/investlog`: 81.1%
  - `internal/api`: 86.1%
  - 相关模块合计：82.0%（>=80%）

## 4. 风险与已知问题
- 已知限制：
  - 建议质量依赖外部模型能力与提示词，不保证收益。
  - API Key 当前保存在浏览器本地（localStorage），未做系统级加密管理。
- 风险评估：中等（外部服务稳定性、模型输出格式差异）。
- 后续建议：
  - 增加“分析历史记录/对比”能力。
  - 增加 `risk_profile/horizon` 可视化配置入口。

## 5. 待确认事项
- 需要 Reviewer 确认：
  - 当前 `prompt` 交互方式是否满足你的使用习惯。
  - 建议字段展示形式（是否需表格化、是否显示更多解释项）。
- 合并前阻塞项：
  - 等你 Review 与确认后，再生成 commit message 并提交。
