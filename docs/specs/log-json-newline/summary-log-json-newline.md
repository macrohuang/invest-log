# 变更总结

> 文件名：`docs/specs/log-json-newline/summary-log-json-newline.md`

## 1. 变更概述
- 需求来源：用户要求移除 AI 请求日志 Pretty JSON 在控制台中的冗余换行。
- 本次目标：让 `ai request json` 日志保持单行紧凑输出，同时不影响脱敏与请求体信息记录。
- 完成情况：已完成实现、单元测试、覆盖率验证和交付文档补充。

## 2. 实现内容
- 代码变更点：将 `formatAIRequestForLog` 的最终编码由 `json.MarshalIndent` 改为 `json.Marshal`；新增原始 body 紧凑输出回归测试。
- 文档变更点：新增 `docs/specs/log-json-newline/` 下的 MRD、设计、QA、Summary 文档。
- 关键设计调整：只调整日志文本格式，不改变字段结构、Header 脱敏逻辑和 body 解析策略。

## 3. 测试与质量
- 已执行测试：`go test -v -run 'TestFormatAIRequestForLog_' ./pkg/investlog`
- 测试结果：2 个目标测试通过。
- 覆盖率结果（含命令与数值）：`go test -cover ./pkg/investlog/...` -> `coverage: 80.9% of statements`

## 4. 风险与已知问题
- 已知限制：日志不再以缩进形式展示，人工直接浏览深层 JSON 会略差，但可读性问题已转移到外部格式化工具解决。
- 风险评估：低风险；只影响日志字符串格式，不影响请求发送和业务逻辑。
- 后续建议：如果以后需要区分文件日志与控制台日志，可再评估按 logger handler 类型切换输出格式。

## 5. 待确认事项
- 需要 Reviewer 确认：当前“去掉多余换行、保留紧凑 JSON”是否符合你的预期。
- 合并前阻塞项：无；是否提交由你确认。
