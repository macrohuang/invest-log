# Holdings Manual 并入 Modify MRD

## 1. 背景与目标
- 背景问题：Holdings 页面当前同时暴露 `Manual` 和 `Modify` 两个相邻动作。前者只更新手动价格，后者只调整持仓股数/成本，用户需要在两个入口之间判断差异，增加操作成本。
- 业务目标：将 `Manual` 功能并入 `Modify` 操作流，在不丢失现有能力的前提下减少一个按钮，统一“人工修正持仓”入口。
- 成功指标：
  - Holdings 行内 Action 区不再展示独立 `Manual` 按钮。
  - `Modify` 流程可继续修改 shares / avg cost，并支持手动价格录入。
  - 手动价格能力行为与现有 `/api/prices/manual` 保持一致。

## 2. 用户与场景
- 目标用户：需要在持仓列表中快速修正持仓数据与价格数据的投资用户。
- 核心场景：
  - 用户发现某一持仓股数或均价不准确，希望在 `Modify` 中修正。
  - 用户只想写入一个手动价格，不希望再额外寻找 `Manual` 按钮。
  - 用户需要在一次操作中同时修正持仓和价格。

## 3. 范围定义
- In Scope：
  - `static/modules/pages/holdings.js` 中 Holdings action 区按钮布局与 `Modify` 行为调整。
  - `static/styles/button.css` 与 `static/styles/responsive.css` 中 Holdings action 布局适配。
  - 同步 `ios/App/App/public` 静态资源。
- Out of Scope：
  - 后端接口语义重构或新增接口。
  - 非 Holdings 页面上的 manual price 入口调整。
  - Modal 组件重写。

## 4. 需求明细
- FR-1：每行持仓的 Actions 只保留 `Trade`、`Update`、`Modify`、`AI` 四个动作位，其中 `Manual` 不再单独显示。
- FR-2：点击 `Modify` 时，用户仍可依次输入目标 shares 和目标 avg cost。
- FR-3：`Modify` 流程中应提供进入手动价格录入的机会；若用户选择录入，则继续调用现有手动价格接口保存。
- FR-4：若用户仅想录入手动价格，也应能通过 `Modify` 流程完成，不应被强制要求修改持仓值。
- NFR-1：整合后 Action 区布局仍保持稳定，桌面和窄屏下不出现错位。
- NFR-2：错误反馈需区分 `Modify` 失败与手动价格失败，避免误导。

## 5. 约束与依赖
- 技术约束：前端继续使用现有 `showPromptModal` / `showConfirmModal`，不引入新表单弹窗组件。
- 接口约束：继续使用现有 `/api/holdings/modify` 与 `/api/prices/manual`，不改变后端 JSON 协议。
- 依赖：`showToast`、`fetchJSON`、Holdings 页面重新渲染逻辑。

## 6. 边界与异常
- 边界条件：
  - 用户取消任一步 prompt 时应终止当前 `Modify` 流程。
  - 若 shares 与 avg cost 都未变化，但用户选择录入手动价格，手动价格仍应保存成功。
  - 若 shares / avg cost 非法，流程应立即提示并终止，不继续提交。
- 异常处理：
  - `Modify` 接口失败时提示持仓修改失败。
  - 手动价格接口失败时提示手动价格保存失败。

## 7. 验收标准
- AC-1：Holdings 页面不再出现独立 `Manual` 按钮。
- AC-2：`Modify` 可成功修改持仓 shares / avg cost，行为与改动前一致。
- AC-3：`Modify` 流程允许用户选择录入 manual price，并成功触发 `/api/prices/manual`。
- AC-4：当持仓值未变但录入 manual price 时，手动价格仍可保存。
- AC-5：桌面与窄屏下 Actions 布局稳定，`Modify` 按钮可正常点击。

## 8. 假设与决策记录
- 假设：用户接受 `Manual` 能力进入 `Modify` 二级流程，而不是保留独立一级按钮。
- 决策：本次不新增复合后端接口，前端在 `Modify` 流程中按需串联两个现有请求，以降低改动风险。
