# Settings-Symbols 快速筛选 技术设计

> 文件名：`docs/specs/settings-symbols-quick-filter/design-settings-symbols-quick-filter.md`

## 1. 设计目标
- 对应 MRD：`docs/specs/settings-symbols-quick-filter/mrd-settings-symbols-quick-filter.md`
- 目标与非目标：
  - 目标：在现有 Symbols 管理表格上增加低侵入快速筛选能力。
  - 非目标：不改后端接口，不引入前端框架，不做跨会话筛选状态持久化。

## 2. 现状分析
- 相关模块：
  - `static/app.js` 的 `renderSettings()` 负责渲染 Symbols 表格。
  - `static/app.js` 的 `bindSettingsActions()` 负责绑定保存行为。
  - `static/style.css` 负责 Settings 页面样式。
- 当前实现限制：
  - Symbols 仅支持逐行编辑，没有查找入口。
  - 数据量大时定位成本高。

## 3. 方案概述
- 总体架构：在前端渲染层新增筛选 UI，并在事件层新增 `input` 监听，基于行级索引文本（`symbol + name + asset_type`）做显隐控制，同时对命中内容做高亮。
- 关键流程：
  1. 渲染 Symbols 卡片时输出筛选输入框、计数文案容器、Clear 按钮。
  2. `bindSettingsActions()` 中初始化筛选逻辑。
  3. 输入关键字后遍历 `tbody tr[data-symbol-row]` 并更新 `display`。
  4. 对匹配字段应用高亮（Symbol 文本片段、Name 输入框边框/背景、Asset Type 下拉边框/背景）。
  5. 更新计数文本；若无匹配显示空状态行。
- 关键设计决策：
  - 使用 `data-*` 标记行与可检索字段，减少对列结构变更的耦合。
  - 维持表格 DOM 常驻，仅切换显示状态，避免重渲染导致焦点与输入内容丢失。

## 4. 详细设计
- 接口/API 变更：无。
- 数据模型/存储变更：无。
- 核心算法与规则：
  - 归一化规则：`keyword = input.trim().toLowerCase()`。
  - 匹配目标：`symbol.toLowerCase()`、`name.toLowerCase()` 或 `assetType.toLowerCase()` 包含 `keyword`。
  - 可见数量：`visibleCount`；总数：`totalCount`。
  - 空状态：`visibleCount === 0` 时显示占位行。
  - 高亮规则：
    - Symbol：使用 `<mark>` 包裹匹配片段。
    - Name/Asset Type：当字段命中时增加高亮 class（不改控件值）。
- 错误处理与降级：
  - 若未找到筛选相关节点，直接返回，不影响原有保存逻辑。

## 5. 兼容性与迁移
- 向后兼容性：完全向后兼容，仅新增前端 UI/样式。
- 数据迁移计划：无。
- 发布/回滚策略：
  - 发布：随静态资源发布。
  - 回滚：回退 `static/app.js` 与 `static/style.css` 改动即可。

## 6. 风险与权衡
- 风险 R1：当列表很大时，每次输入遍历全部行可能有轻微性能开销。
- 规避措施：逻辑保持 O(n) 且仅做字符串包含判断；未来可按需加节流。
- 备选方案与取舍：
  - 备选：后端查询式筛选。
  - 取舍：当前数据量通常可在前端处理，前端方案改动更小、体验更即时。

## 7. 实施计划
- 任务拆分：
  1. 修改 Symbols 渲染模板。
  2. 增加筛选绑定逻辑。
  3. 增加样式与空状态展示。
  4. 同步 `ios/App/App/public` 对应静态文件。
- 里程碑：本次一次性完成 UI + 行为 + 回归验证。
- 影响范围：Settings-Symbols 子页面。

## 8. 验证计划
- 与 QA Spec 的映射：TC-001~TC-005 映射输入筛选、Asset Type 命中、高亮反馈、清空恢复、保存回归。
- 关键验证点：
  - 快速筛选响应是否实时。
  - 命中高亮反馈是否准确且不影响编辑。
  - 无匹配占位是否正确。
  - 行内保存功能不受影响。
