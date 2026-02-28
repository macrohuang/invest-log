# Gemini 原生客户端 + SSE 流式返回 QA 测试用例 Spec

> 文件名：`docs/specs/gemini-go-genai-sse-streaming/qa-gemini-go-genai-sse-streaming.md`

## 1. 测试目标
- 对应 MRD：`docs/specs/gemini-go-genai-sse-streaming/mrd-gemini-go-genai-sse-streaming.md`
- 对应设计文档：`docs/specs/gemini-go-genai-sse-streaming/design-gemini-go-genai-sse-streaming.md`

## 2. 测试范围
- In Scope：
  - Gemini 判定与 go-genai 调用分支（后端单元测试）。
  - `/api/ai/holdings-analysis/stream` SSE 协议输出。
  - Holdings 页面流式渲染与完成态更新。
  - Settings 配置来源保持不变。
- Out of Scope：
  - symbol-analysis 页面流式化。
  - 真实线上 Gemini API 连通性（本地用 stub/模拟验证）。

## 3. 测试策略
- 单元测试：
  - `pkg/investlog`：Gemini 判定、流式聚合与错误处理。
  - `internal/api`：SSE 路由返回状态与事件格式。
- 集成测试：
  - 执行 `go test ./...` 回归。
- 端到端/手工验证：
  - 在 Holdings 页面点击 AI，观察 chunk 渐进展示和最终 done 结果。

## 4. 用例清单
- TC-001（正向）：前置条件 / model 为 `gemini-*`；步骤 / 请求 holdings stream；预期结果 / 收到 `start -> chunk* -> done`。
- TC-002（正向）：前置条件 / 前端配置好 model/api key/base url；步骤 / Holdings 点击 AI；预期结果 / 卡片出现“生成中”，文本逐步增长，最终展示结构化结果。
- TC-003（异常）：前置条件 / API key 为空或上游失败；步骤 / 发起 stream；预期结果 / 前端收到 `error` 事件并提示失败。
- TC-004（回归）：前置条件 / 非 Gemini 模型（如 gpt-*）；步骤 / 调用旧接口 `/api/ai/holdings-analysis`；预期结果 / 功能不回归。
- TC-005（回归）：前置条件 / 分析完成；步骤 / 调用 history 接口；预期结果 / 新记录可查，前端历史区域刷新正常。

## 5. 覆盖率策略
- 统计口径（命令/工具）：
  - `cd go-backend && go test ./...`
  - `cd go-backend && go test ./... -coverprofile=coverage_gemini_sse.out`
  - `cd go-backend && go tool cover -func=coverage_gemini_sse.out`
- 当前覆盖盲区：前端 SSE 解析逻辑暂无自动化测试框架。
- 提升计划（目标 >=80%）：
  - 重点补齐 `pkg/investlog` 和 `internal/api` 新增分支用例；
  - 前端先手工回归，后续再引入 jsdom 测试基建。

## 6. 退出标准
- 所有 P0/P1 用例通过。
- 新增后端单元测试通过。
- `go test ./...` 全绿。
- 覆盖率达到或超过 80%（至少对受影响包达标并记录结果）。

## 7. 缺陷记录
- Defect-1：待测试执行后回填。
