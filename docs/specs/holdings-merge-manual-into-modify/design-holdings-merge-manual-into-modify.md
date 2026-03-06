# Holdings Manual 并入 Modify 技术设计

## 1. 设计目标
- 对应 MRD：`docs/specs/holdings-merge-manual-into-modify/mrd-holdings-merge-manual-into-modify.md`
- 目标：减少 Holdings action 区一级操作数量，并把 manual price 能力纳入 `Modify` 统一入口。
- 非目标：不修改 Go 后端接口，不新增弹窗组件。

## 2. 现状分析
- 当前 `static/modules/pages/holdings.js` 中：
  - `Manual` 按钮独立绑定 `data-action="manual"`，点击后直接 prompt 并请求 `/api/prices/manual`。
  - `Modify` 按钮只处理 `target_shares` 和 `target_avg_cost`，请求 `/api/holdings/modify`。
- 当前问题：
  - 一级操作过多，`Manual` 与 `Modify` 心智边界接近。
  - 用户若只想修正一条持仓信息，需要在两个按钮间切换。

## 3. 方案概述
- UI 结构：
  - 删除独立 `Manual` 按钮。
  - 保留 `Trade`、`Update`、`Modify`、`AI` 四个动作槽位，其中 `Modify` 占据原第二行左侧。
- 交互流程：
  - `Modify` 先采集 target shares。
  - 再采集 target avg cost。
  - 然后以确认弹窗询问是否需要同步设置 manual price。
  - 若需要，再采集价格并请求 `/api/prices/manual`。
- 请求编排：
  - 仅当 shares / avg cost 与当前值存在变化时才调用 `/api/holdings/modify`。
  - 手动价格仅在用户确认后调用 `/api/prices/manual`。
  - 两者都未发生变化时提示 `No changes to save`。

## 4. 详细设计
- 前端模板变更：
  - 从 Holdings action DOM 中移除 `data-action="manual"` 按钮。
  - 在 `Modify` 按钮上补充 `data-latest-price`，用于手动价格输入默认值。
- 事件逻辑变更：
  - 删除 `action === 'manual'` 分支。
  - 将 `action === 'modify'` 重构为：
    - 读取当前 shares / avg cost / latest price。
    - 完成两个数值输入校验。
    - 通过 `showConfirmModal` 询问是否设置 manual price。
    - 分别执行 holdings modify 与 manual price 请求，并汇总 toast。
- 样式变更：
  - Holdings action grid 从 `trade update / manual ai` 改为 `trade update / modify ai`。
  - 删除 `.holdings-action-manual` 样式，新增 `.holdings-action-modify` 占位。

## 5. 兼容性与风险
- 兼容性：
  - 后端接口和参数不变，数据层无迁移。
  - 事件绑定仍基于 `button[data-action]`，对其它 action 无影响。
- 风险：
  - 顺序 prompt 会让 `Modify` 流程变长。
  - 用户可能只想改价格，不想经过前两步 prompt。
- 缓解：
  - shares / avg cost prompt 预填当前值，用户可直接回车通过。
  - 当持仓字段未变化时跳过 `/api/holdings/modify`，只保存 manual price。

## 6. 验证计划
- 检查 `Manual` 按钮从 DOM 中消失，`Modify` 仍存在。
- 验证三种路径：
  - 只改 holdings。
  - 只改 manual price。
  - 同时改 holdings 和 manual price。
- 验证异常路径：
  - 非法数字输入。
  - 用户取消 prompt/confirm。
  - 任一请求失败时 toast 正确。
