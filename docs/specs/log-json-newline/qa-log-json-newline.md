# QA 测试用例 Spec

> 文件名：`docs/specs/log-json-newline/qa-log-json-newline.md`

## 1. 测试目标
- 对应 MRD：`docs/specs/log-json-newline/mrd-log-json-newline.md`
- 对应设计文档：`docs/specs/log-json-newline/design-log-json-newline.md`

## 2. 测试范围
- In Scope：`formatAIRequestForLog` 的日志编码格式、Header 脱敏、JSON/非 JSON body 行为。
- Out of Scope：真实终端渲染差异、其他 `slog` 日志路径。

## 3. 测试策略
- 单元测试：覆盖 JSON body 与非 JSON body 两种输入，断言日志内容和换行行为。
- 集成测试：本次不新增，现有调用链不变。
- 端到端/手工验证：可选地在本地启动服务观察一条 `ai request json` 日志是否保持单行。

## 4. 用例清单
- TC-001（正向）：前置条件：构造带 JSON body 的 AI 请求。步骤：调用 `formatAIRequestForLog`。预期结果：输出包含 method/url/headers/body，且不包含 Pretty Print 产生的真实换行。
- TC-002（异常）：前置条件：构造非 JSON 原始 body。步骤：调用 `formatAIRequestForLog`。预期结果：输出写入 `body_raw`，原始文本以 JSON 转义形式保留，不出现真实换行。
- TC-003（边界）：前置条件：请求头包含 `x-goog-api-key` 与 `Authorization`。步骤：调用格式化函数。预期结果：敏感值被脱敏。
- TC-004（回归）：前置条件：合法 JSON body 中包含 `systemInstruction`、`contents`。步骤：调用格式化函数。预期结果：原有关键字段仍在日志中。

## 5. 覆盖率策略
- 统计口径（命令/工具）：`go test -cover ./pkg/investlog/...`
- 当前覆盖盲区：本次只针对日志格式化函数，不扩大到非相关模块。
- 提升计划（目标 >=80%）：补两条高价值单测，优先覆盖本次改动分支。

## 6. 退出标准
- 所有 P0/P1 用例通过
- 自动化用例稳定通过
- 覆盖率达到或超过 80%

## 7. 缺陷记录
- Defect-1：现象：Pretty JSON 在控制台中拆成多行，导致日志凌乱。根因：最终日志字符串使用 `json.MarshalIndent`。修复：改为紧凑 JSON 编码并补充回归测试。回归结果：待测试执行记录补充。
