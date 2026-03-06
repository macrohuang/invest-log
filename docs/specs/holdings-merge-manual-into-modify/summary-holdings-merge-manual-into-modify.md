# Holdings Manual 并入 Modify 变更总结

## 1. 变更内容
- Holdings action 区移除了独立 `Manual` 按钮，保持 `Trade`、`Update`、`Modify`、`AI` 四个入口。
- `Modify` 流程现在支持：
  - 修改目标 shares
  - 修改目标 avg cost
  - 可选继续录入 manual price
- 当 shares / avg cost 未变化时，`Modify` 会跳过 `/api/holdings/modify`，仍允许只保存 manual price。
- Holdings action grid 样式同步从 `manual` 槽位切换为 `modify` 槽位。

## 2. 变更原因
- 降低 Holdings 行内一级操作数量。
- 把“人工修正持仓”和“人工修正价格”统一到一个动作里，减少心智切换。
- 复用现有后端接口，避免增加额外后端复杂度。

## 3. 验证结果
- 自动化测试：
  - `cd go-backend && go test ./...` 通过
  - `cd go-backend && go test -cover ./...` 通过
  - `cd go-backend && go test -coverpkg=./... -coverprofile=coverage.out ./...` 通过
- 覆盖率证据：
  - `investlog/cmd/server`: 80.0%
  - `investlog/internal/api`: 80.4%
  - `investlog/pkg/investlog`: 80.9%
  - `investlog/pkg/mobile`: 83.7%
- 手工验证建议：
  - 检查 Holdings 页面已无 `Manual` 按钮
  - 验证只改 holdings / 只改 manual price / 同时修改两者三条路径
  - 验证 960px 与 700px 左右布局不重叠

## 4. 已知限制与后续
- 前端缺少自动化 UI 测试，本次未能为 `holdings.js` 提供 JS 覆盖率。
- `Modify` 仍采用多次 prompt，交互比表单弹窗更线性；若后续需要进一步优化，可考虑合并成单个结构化 modal。
