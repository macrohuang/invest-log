# QA 测试用例 Spec：Settings API 管理非 Key 配置入库

## 1. 测试目标
- 对应 MRD：`docs/specs/settings-api-management-db-storage/mrd-settings-api-management-db-storage.md`
- 对应设计文档：`docs/specs/settings-api-management-db-storage/design-settings-api-management-db-storage.md`

## 2. 测试范围
- In Scope：
  - Core 层 AI 设置读写及归一化。
  - `GET/PUT /api/ai-settings`。
  - 前端保存后读取流程中的关键逻辑（通过现有静态逻辑与手工验证）。
- Out of Scope：
  - 第三方 AI 服务连通性。
  - iOS/macOS 容器端完整手工回归。

## 3. 测试策略
- 单元测试：
  - `pkg/investlog` 新增 `ai_settings_test.go`，覆盖默认值、upsert、非法枚举回退。
- 集成测试：
  - `internal/api` 新增 `handlers_ai_settings_test.go`，覆盖 GET 初始值、PUT 保存、GET 回读。
- 端到端/手工验证：
  - Web 端 Settings > API 保存后刷新，检查非 Key 字段回填与 API Key 仍需本地输入。

## 4. 用例清单
- TC-001（正向）：
  - 前置条件：新建测试库。
  - 步骤：调用 `PUT /api/ai-settings` 写入自定义 non-key 配置，再 `GET /api/ai-settings`。
  - 预期结果：返回值与写入一致（规范化后），状态码 200。
- TC-002（异常）：
  - 前置条件：构造非法 JSON / 非法字段类型。
  - 步骤：调用 `PUT /api/ai-settings`。
  - 预期结果：返回 400，错误结构符合统一格式。
- TC-003（边界）：
  - 前置条件：风险偏好等枚举传非法值。
  - 步骤：调用 Core `SetAISettings`。
  - 预期结果：字段回退到默认枚举值，不报错。
- TC-004（回归）：
  - 前置条件：API Key 已在本地存储。
  - 步骤：触发 holdings/symbol/allocation AI 分析请求。
  - 预期结果：请求仍含 `api_key`，且 UI 提示逻辑不变。

## 5. 覆盖率策略
- 统计口径（命令/工具）：
  - `go test ./...`
  - `go test -coverprofile=coverage.out ./...`
  - `go tool cover -func=coverage.out`
- 当前覆盖盲区：前端 JS 自动化测试缺失。
- 提升计划（目标 >=80%）：新增 Core + API 针对性测试，保证受影响 Go 包覆盖率不低于 80%。

## 6. 退出标准
- 所有 P0/P1 用例通过
- 自动化用例稳定通过
- 覆盖率达到或超过 80%

## 7. 缺陷记录
- Defect-1：待测试后补充
