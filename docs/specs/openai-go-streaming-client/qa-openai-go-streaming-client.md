# QA 测试用例 Spec：OpenAI Go 原生流式调用与前端流式适配

## 1. 测试目标
- 对应 MRD：`docs/specs/openai-go-streaming-client/mrd-openai-go-streaming-client.md`
- 对应设计文档：`docs/specs/openai-go-streaming-client/design-openai-go-streaming-client.md`

## 2. 测试范围
- In Scope：
  - AI 流式客户端（后端）
  - Holdings/Symbol SSE 接口
  - 前端流式消费与最终结果渲染
- Out of Scope：
  - 非 AI 功能模块（交易、价格等）

## 3. 测试策略
- 单元测试：
  - Base URL 规范化
  - 流式 delta 聚合与空响应错误
  - SSE handler 事件输出
- 集成测试：
  - API 层调用 mock AI 上游，验证流式 endpoint 事件顺序。
- 端到端/手工验证：
  - 在 Holdings/Symbol 页面触发分析，观察增量文本和最终卡片刷新。

## 4. 用例清单
- TC-001（正向）：`requestAIChatCompletion` 通过 streaming 累积返回完整 content。
- TC-002（异常）：上游错误或空流返回时，后端返回明确错误。
- TC-003（边界）：`base_url` 输入 `example.com`、`/v1`、`/chat/completions` 均能规范化成功。
- TC-004（回归）：原 `/api/ai/holdings-analysis`、`/api/ai/symbol-analysis` 仍可返回最终 JSON。
- TC-005（正向）：`/api/ai/holdings-analysis/stream` 输出 `progress -> delta* -> result -> done`。
- TC-006（正向）：`/api/ai/symbol-analysis/stream` 输出阶段 `progress` 并最终 `result`。

## 5. 覆盖率策略
- 统计口径（命令/工具）：`go test -cover ./pkg/investlog ./internal/api`
- 当前覆盖盲区：前端 JS 无自动化测试；通过手工回归补足。
- 提升计划（目标 >=80%）：对新增 Go 逻辑补表驱动测试，覆盖成功/失败/边界路径。

## 6. 退出标准
- 所有 P0/P1 用例通过
- 自动化用例稳定通过
- 覆盖率达到或超过 80%

## 7. 缺陷记录
- Defect-1：待执行后补充
