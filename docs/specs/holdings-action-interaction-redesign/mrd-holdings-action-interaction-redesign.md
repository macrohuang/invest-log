# Holding Action 交互区重构 MRD

## 1. 背景与目标
- 背景问题：`#/holdings` 列表中每行的 Action 区（Trade/Update/Manual/AI）当前视觉层级混乱、按钮堆叠感强，导致识别成本高、误触风险高，且与页面其他区域的交互风格不一致。
- 业务目标：在不改变业务能力的前提下，提升 Holding 行内操作的可读性、可达性与交互反馈质量。
- 成功指标（可量化）：
  - 操作区改为清晰的“主操作 + 辅助操作”分层布局。
  - 行内 Action 区在桌面端保持紧凑（目标高度约 80-90px），不再出现四个按钮单列堆叠。
  - 键盘可达（`Tab` 可聚焦）与焦点可见性满足基本可访问性要求。

## 2. 用户与场景
- 目标用户：日常查看持仓并进行快速交易/更新价格的投资用户。
- 核心使用场景：
  - 在 Holding 列表直接发起 Trade（Buy/Sell/Dividend/Transfer）。
  - 在单个 Symbol 上执行 Update、Manual Price、AI 分析。
- 非目标用户：不在本次关注新手引导、策略解释、AI 结果内容消费等场景。

## 3. 范围定义
- In Scope：
  - `static/modules/pages/holdings.js` 中 Action 区 DOM 结构与语义增强。
  - `static/styles/button.css` 与 `static/styles/responsive.css` 中 Holding Action 样式重构。
  - 保持现有 API 调用与页面路由行为不变。
- Out of Scope：
  - 后端接口、数据库、AI 分析逻辑。
  - Holdings 表格其它列（非 Action）视觉重设计。
  - 全局设计系统大规模改版。

## 4. 需求明细
- 功能需求 FR-1：Action 区必须体现明确优先级，Trade 为主操作，Update 为次操作，Manual/AI 为同级辅助操作。
- 功能需求 FR-2：Action 区布局在桌面与窄屏下均保持稳定，不出现不可点击或严重拥挤。
- 功能需求 FR-3：现有四个动作行为保持不变（Trade 跳转、Update 调价、Manual 录入、AI 分析）。
- 非功能需求 NFR-1（可用性/可访问性）：
  - 可聚焦控件需有清晰 `focus-visible` 样式。
  - 点击目标尺寸不低于当前实现（>=32px）。
  - 状态反馈（hover/active/disabled）可被用户直观看到。

## 5. 约束与依赖
- 技术约束：当前前端为原生 HTML/CSS/Vanilla JS 模块化结构，不引入新框架。
- 数据约束：不新增数据字段，继续使用现有 `data-*` 属性驱动交互。
- 外部依赖：无新增外部依赖；继续复用现有 `fetchJSON`、`showToast`、`showPromptModal`。

## 6. 边界与异常
- 边界条件：
  - `auto_update=0` 时 Update 按钮必须禁用并保留提示。
  - 行内数据缺失（如 symbol/account 为空）不应造成操作区渲染异常。
  - 小屏下表格横向滚动时，Action 区需保持可点击。
- 异常处理：交互触发后的 API 异常继续沿用现有 toast/error 处理逻辑。
- 失败回退：若新样式出现兼容性问题，可回退到旧 Action 布局与按钮样式。

## 7. 验收标准
- AC-1（可验证）：Holdings 页面中每行 Action 区呈现为分层紧凑布局（非单列堆叠），视觉优先级清晰。
- AC-2（可验证）：Trade/Update/Manual/AI 四个动作行为与改造前一致。
- AC-3（可验证）：键盘导航可依次聚焦四个控件，焦点边框清晰。
- AC-4（可验证）：禁用态（Update 在 auto sync off）样式明显且不可触发请求。

## 8. 待确认问题
- Q1：按钮文案是否需要统一中文化（本次默认保持现有英文文案以降低认知迁移成本）。
- Q2：是否需要在后续迭代将行内 Action 改为“更多菜单”模式（本次不做交互模型变更）。

## 9. 假设与决策记录
- 假设：用户期望在表格内快速完成常用操作，优先关注清晰层级而非新增功能。
- 决策：本次仅做结构和样式重构，不改动业务流程与接口语义，确保低风险上线。
