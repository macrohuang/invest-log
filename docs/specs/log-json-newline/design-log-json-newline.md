# 技术设计

> 文件名：`docs/specs/log-json-newline/design-log-json-newline.md`

## 1. 设计目标
- 对应 MRD：`docs/specs/log-json-newline/mrd-log-json-newline.md`
- 目标与非目标：目标是让 `ai request json` 日志变成单行紧凑 JSON；非目标是改变日志字段结构或引入新的日志配置项。

## 2. 现状分析
- 相关模块：`go-backend/pkg/investlog/ai_chat_client_logging.go`、相关测试位于 `go-backend/pkg/investlog/ai_holdings_analysis_test.go`。
- 当前实现限制：`formatAIRequestForLog` 使用 `json.MarshalIndent`，即使请求 body 已被解析成结构体，也会在最终日志字符串里插入多行缩进。

## 3. 方案概述
- 总体架构：保持现有日志 payload 组装逻辑不变，仅替换最终编码方式。
- 关键流程：收集 method/url -> 脱敏 headers -> 尝试解析 body 为 JSON -> 将整个 payload 使用紧凑 JSON 编码。
- 关键设计决策：使用 `json.Marshal` 替代 `json.MarshalIndent`，这样 JSON body 会被标准库重新编码为单行结构。

## 4. 详细设计
- 接口/API 变更：无外部 API 变更。
- 数据模型/存储变更：无。
- 核心算法与规则：若 body 是合法 JSON，继续写入 `body` 字段；若不是合法 JSON，继续写入 `body_raw`。两种情况最终都通过紧凑 JSON 编码输出。
- 错误处理与降级：编码失败时返回单行错误 JSON，字段名保持 `error` 与 `detail`。

## 5. 兼容性与迁移
- 向后兼容性：日志字段名、脱敏逻辑和业务逻辑保持兼容，仅文本显示格式从多行改为单行。
- 数据迁移计划：无。
- 发布/回滚策略：单文件改动，可直接回滚到旧实现。

## 6. 风险与权衡
- 风险 R1：日志可读性从“缩进展示”变为“单行展示”，人工肉眼阅读局部结构可能稍差。
- 规避措施：控制台与日志系统的可检索性、可复制性更重要，且 JSON 仍是合法结构，可借助外部工具格式化。
- 备选方案与取舍：也可考虑按 logger 类型动态决定是否 pretty-print，但复杂度与收益不匹配，本次不做。

## 7. 实施计划
- 任务拆分：调整编码实现；补 JSON/非 JSON 两类测试；执行包级测试与覆盖率。
- 里程碑：实现完成 -> 测试通过 -> 输出 summary。
- 影响范围：仅 AI 请求日志格式化路径。

## 8. 验证计划
- 与 QA Spec 的映射：见 `docs/specs/log-json-newline/qa-log-json-newline.md`。
- 关键验证点：日志为单行；敏感 Header 被脱敏；JSON body 字段仍然存在；原始文本 body 不丢失。
