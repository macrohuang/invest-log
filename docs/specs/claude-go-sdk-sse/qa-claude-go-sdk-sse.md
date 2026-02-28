# Claude GO SDK + SSE QA Spec

## 1. 测试目标
- 对应 MRD：`mrd-claude-go-sdk-sse.md`
- 对应设计文档：`design-claude-go-sdk-sse.md`

## 2. 测试范围
- In Scope：
  - Claude SDK 流式事件处理。
  - `POST /api/ai/holdings-analysis/stream` SSE 输出。
  - Holdings 前端流式消费与完成态渲染。
- Out of Scope：
  - Symbol Analysis/Allocation Advice 的流式 UI。

## 3. 测试策略
- 单元测试：
  - `pkg/investlog`: Claude stream 文本增量拼接与完成结果解析。
  - `internal/api`: SSE handler 返回事件格式与成功/异常路径。
- 集成测试：
  - 使用 `httptest` 模拟上游 Claude SSE，验证后端->前端协议数据。
- 端到端/手工验证：
  - Holdings 页面点击 AI，观察逐块输出、完成后历史刷新。

## 4. 用例清单
- TC-001（正向）：Claude 流式事件（多 chunk）-> 最终 JSON 解析成功并保存。
- TC-002（异常）：上游 stream 错误 -> SSE `error` 事件返回，前端提示失败。
- TC-003（边界）：非 Claude 请求走降级路径，stream endpoint 仍返回可完成结果。
- TC-004（回归）：原 `POST /api/ai/holdings-analysis` 非流式接口行为不变。

## 5. 覆盖率策略
- 统计口径（命令/工具）：`go test -cover ./pkg/investlog ./internal/api`
- 当前覆盖盲区：前端 JS 无自动化单测。
- 提升计划（目标 >=80%）：优先补齐后端流式核心逻辑与 handler 分支覆盖。

## 6. 退出标准
- 所有 P0/P1 用例通过。
- 自动化用例稳定通过。
- 变更模块覆盖率达到或超过 80%。

## 7. 缺陷记录
- Defect-1：待实现后补充。
