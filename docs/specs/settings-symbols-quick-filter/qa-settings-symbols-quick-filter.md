# Settings-Symbols 快速筛选 QA 测试用例 Spec

> 文件名：`docs/specs/settings-symbols-quick-filter/qa-settings-symbols-quick-filter.md`

## 1. 测试目标
- 对应 MRD：`docs/specs/settings-symbols-quick-filter/mrd-settings-symbols-quick-filter.md`
- 对应设计文档：`docs/specs/settings-symbols-quick-filter/design-settings-symbols-quick-filter.md`

## 2. 测试范围
- In Scope：
  - Symbols 快速筛选输入、清空按钮、计数文案、无匹配占位。
  - 筛选后行内 `Save` 操作回归。
  - SPA 与 iOS 镜像静态资源一致性。
- Out of Scope：
  - 后端 API 正确性（本次未改后端）。
  - 复杂组合筛选与持久化能力。

## 3. 测试策略
- 单元测试：当前仓库无前端单元测试基建，本次不新增测试框架。
- 集成测试：执行 Go 后端现有自动化测试，确保本次前端改动未引入服务端回归。
- 端到端/手工验证：按 Symbols 页面真实交互进行手工验证（输入筛选、清空、无结果、保存）。

## 4. 用例清单
- TC-001（正向）：前置条件 / 已进入 `Settings > Symbols`；步骤 / 输入存在的 symbol（如 `AAPL`）；预期结果 / 仅显示匹配行，计数显示 `Showing x / y`。
- TC-002（正向）：前置条件 / 同上；步骤 / 输入资产类型关键字（如 `stock`）；预期结果 / 按 `asset_type` 匹配过滤成功。
- TC-003（异常）：前置条件 / 同上；步骤 / 输入不存在关键字（如 `__not_found__`）；预期结果 / 表格仅显示 `No matching symbols.` 占位行。
- TC-004（边界）：前置条件 / 同上；步骤 / 输入为空或点击 `Clear`；预期结果 / 全部行恢复可见，计数显示 `Total y symbol(s)`。
- TC-005（回归）：前置条件 / 已通过筛选定位某行；步骤 / 修改 Name 或 Asset Type 并点 `Save`；预期结果 / 请求成功，toast 显示 `<symbol> updated`。
- TC-006（交互反馈）：前置条件 / 同上；步骤 / 输入关键字；预期结果 / Symbol 命中片段 `<mark>` 高亮，Name/Asset Type 命中字段有高亮样式。

## 5. 覆盖率策略
- 统计口径（命令/工具）：
  - 后端回归：`cd go-backend && go test ./...`
  - 覆盖率（后端可测部分）：`cd go-backend && go test ./... -coverprofile=coverage_settings_symbols_quick_filter.out`
- 当前覆盖盲区：前端 `static/app.js` 与 `static/style.css` 暂无自动化覆盖工具。
- 提升计划（目标 >=80%）：
  - 中期引入前端测试基建（如 Vitest + jsdom）后，为 Symbols 筛选逻辑补齐自动化用例。

## 6. 退出标准
- 所有 P0/P1 用例通过。
- 自动化回归（Go 测试）通过。
- 前端覆盖率工具缺失时，记录限制并提供可复现手工验证结果。

## 7. 缺陷记录
- Defect-1：暂无。
