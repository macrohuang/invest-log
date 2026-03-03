# Holding Action 交互区重构 QA Spec

## 1. 测试目标
- 对应 MRD：`docs/specs/holdings-action-interaction-redesign/mrd-holdings-action-interaction-redesign.md`
- 对应设计文档：`docs/specs/holdings-action-interaction-redesign/design-holdings-action-interaction-redesign.md`
- 目标：验证 Action 区视觉层级优化且行为不回归。

## 2. 测试范围
- In Scope：
  - Holdings 行内 Action 布局、交互态、可访问性。
  - `Trade/Update/Manual/AI` 行为一致性。
- Out of Scope：
  - 后端算法正确性深测。
  - 非 Action 列视觉重构。

## 3. 测试策略
- 单元测试：
  - 当前仓库无前端 JS 单测基建，本次以静态回归 + 行为验证替代。
- 集成测试：
  - 通过页面交互验证路由跳转与 API 请求行为。
- 端到端/手工验证：
  - 桌面宽屏与窄屏（960/700 附近）分别执行核心用例。

## 4. 用例清单
- TC-001（正向）：
  - 前置条件：存在一条可操作持仓（`auto_update=1`）。
  - 步骤：打开 Holdings，观察 Action 区并依次触发 Trade/Update/Manual/AI。
  - 预期结果：布局为分层 2x2，四种动作行为与改造前一致。
- TC-002（异常）：
  - 前置条件：存在 `auto_update=0` 持仓。
  - 步骤：点击 Update。
  - 预期结果：按钮禁用，不发请求，显示禁用提示。
- TC-003（边界）：
  - 前置条件：视口调为 960px 与 700px。
  - 步骤：检查 Action 区是否重叠/溢出并尝试点击所有控件。
  - 预期结果：布局稳定，可点击，无错位。
- TC-004（回归）：
  - 前置条件：Holdings 页面有多个币种 tab。
  - 步骤：切换 tab 后执行行内 Action。
  - 预期结果：当前 tab 状态与原有行为保持一致，无额外刷新问题。

## 5. 覆盖率策略
- 统计口径（命令/工具）：
  - 前端无自动覆盖率工具，本次记录手工用例覆盖与后端回归测试结果。
- 当前覆盖盲区：
  - 浏览器自动化回放与视觉回归快照未接入。
- 提升计划（目标 >=80%）：
  - 后续如引入 Playwright，可将 TC-001~TC-004 自动化并纳入 CI。

## 6. 退出标准
- 所有 P0/P1 手工用例通过。
- 自动化后端回归通过（`go test ./...`）。
- 关键验收项（AC-1~AC-4）均有证据。

## 7. 缺陷记录
- Defect-1：待执行阶段发现后补充。
