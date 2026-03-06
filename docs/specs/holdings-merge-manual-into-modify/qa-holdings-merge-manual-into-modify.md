# Holdings Manual 并入 Modify QA Spec

## 1. 测试目标
- 对应 MRD：`docs/specs/holdings-merge-manual-into-modify/mrd-holdings-merge-manual-into-modify.md`
- 对应设计文档：`docs/specs/holdings-merge-manual-into-modify/design-holdings-merge-manual-into-modify.md`
- 目标：验证 `Manual` 被整合进 `Modify` 后，功能不丢失且交互不回归。

## 2. 测试范围
- In Scope：
  - Holdings action 区按钮展示。
  - `Modify` 中的 holdings 修改和 manual price 录入流程。
  - action 区响应式布局。
- Out of Scope：
  - 价格更新后对外部行情源的影响。
  - 其它页面的 manual price 入口。

## 3. 测试策略
- 自动化：
  - 运行现有 Go API 测试，确保后端接口回归无异常。
- 手工验证：
  - 前端当前无单测 / E2E 基建，本次以页面交互回归验证为主。

## 4. 核心用例
- TC-001：
  - 步骤：打开 Holdings 页面。
  - 预期：每行不再显示 `Manual` 按钮，仅显示 `Trade`、`Update`、`Modify`、`AI`。
- TC-002：
  - 步骤：点击 `Modify`，修改 shares / avg cost，不设置 manual price。
  - 预期：调用 `/api/holdings/modify`，持仓更新成功。
- TC-003：
  - 步骤：点击 `Modify`，保持 shares / avg cost 不变，选择设置 manual price 并输入有效数字。
  - 预期：不调用无效 holdings modify，仍成功调用 `/api/prices/manual`。
- TC-004：
  - 步骤：点击 `Modify`，同时修改 holdings 并设置 manual price。
  - 预期：两个请求都成功，页面刷新后显示更新结果。
- TC-005：
  - 步骤：在 `Modify` 中输入非法 shares / avg cost / manual price。
  - 预期：toast 提示对应字段非法，且不发送后续请求。
- TC-006：
  - 步骤：将视口缩到 960px / 700px 附近查看 action 区。
  - 预期：布局仍为两行两列，无重叠。

## 5. 覆盖与退出标准
- 自动化测试通过：`go test ./...`
- 手工验证覆盖 TC-001 至 TC-006 的核心路径。
- 已知限制：前端缺少自动覆盖率工具，本次无法给出 JS 语句覆盖率；后续可用 Playwright 补齐。
