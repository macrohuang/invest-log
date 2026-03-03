# Holding Action 交互区重构技术设计

## 1. 设计目标
- 对应 MRD：`docs/specs/holdings-action-interaction-redesign/mrd-holdings-action-interaction-redesign.md`
- 目标与非目标：
  - 目标：重构 Holding 行内 Action 区视觉层级和交互可读性，保持行为不变。
  - 非目标：不改后端 API、不改路由语义、不新增功能按钮。

## 2. 现状分析
- 相关模块：
  - `static/modules/pages/holdings.js`
  - `static/styles/button.css`
  - `static/styles/form.css`
  - `static/styles/responsive.css`
- 当前实现限制：
  - Action 容器同时使用 `.actions` 与 `.holdings-actions`；由于 `form.css` 在 `button.css` 后加载，`.actions` 的 `display:flex` 覆盖了 `.holdings-actions` 的 `display:grid`，导致控件单列堆叠。
  - 4 个操作视觉权重接近，主次关系弱，焦点态与禁用态对比不足。

## 3. 方案概述
- 总体架构：
  - 保持现有事件绑定与 `data-action` 协议。
  - 仅重构 Action 区 DOM 语义标记与局部样式命名空间。
- 关键流程：
  - `Trade` 仍走 `select.trade-select` 的 `change` 跳转。
  - `Update/Manual/AI` 仍走 `button[data-action]` 点击分发逻辑。
- 关键设计决策：
  - 使用 `.actions.holdings-actions` 提升选择器优先级，显式覆盖 `.actions` 的通用样式。
  - 将 Action 布局改为两层：首行 `Trade + Update`，次行 `Manual + AI`，形成明显主次层级。
  - 增加 `role="group"`、`aria-label` 与更清晰的 `focus-visible`。

## 4. 详细设计
- 接口/API 变更：
  - 无接口变更。
- 数据模型/存储变更：
  - 无数据结构变更。
- 核心算法与规则：
  - 在 `holdings.js` 为 Action 区控件增加语义 class（`holdings-action-*`）和辅助无障碍标签。
  - 在 `button.css` 为 `.actions.holdings-actions` 重写网格布局与交互态样式。
  - 在 `responsive.css` 为窄屏进一步收紧 Action 列宽并保持 2x2 操作布局稳定。
- 错误处理与降级：
  - 保留现有 `try/catch + toast` 逻辑。
  - 保留 `updateDisabled` 的禁用条件与 tooltip 提示。

## 5. 兼容性与迁移
- 向后兼容性：
  - 保持 `trade-select` 与 `button[data-action]` 选择器不变，兼容现有事件绑定。
- 数据迁移计划：
  - 无。
- 发布/回滚策略：
  - 单次前端静态资源发布；若异常可回滚三处文件改动。

## 6. 风险与权衡
- 风险 R1：Action 列宽调整后可能影响小屏可读性。
- 规避措施：在 `responsive.css` 保持最小宽度并维持可横向滚动。
- 备选方案与取舍：
  - 备选：改为下拉菜单聚合次级操作。
  - 取舍：本次不改交互模型，优先低风险重构现有控件层级。

## 7. 实施计划
- 任务拆分：
  - Task-1：更新 `holdings.js` 的 Action 区模板语义结构。
  - Task-2：更新 `button.css` 的 Action 区视觉/交互样式。
  - Task-3：更新 `responsive.css` 的 Action 区断点策略。
  - Task-4：执行回归验证并输出 summary。
- 里程碑：
  - M1 文档完成 -> M2 UI 实现 -> M3 回归验证。
- 影响范围：
  - 仅 Holdings 页面表格 Action 列；其他页面按钮样式不应受影响。

## 8. 验证计划
- 与 QA Spec 的映射：
  - FR-1/AC-1 -> 布局层级验证。
  - FR-3/AC-2 -> 行为一致性验证（路由/API）。
  - NFR-1/AC-3/AC-4 -> 焦点态、禁用态与键盘可达验证。
- 关键验证点：
  - Action 区不再单列堆叠。
  - 4 个动作行为完全保持。
  - 桌面和窄屏均可点击且焦点可见。
