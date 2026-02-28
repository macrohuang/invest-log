# 变更总结：Settings API 管理非 Key 配置入库

## 1. 变更概述
- 需求来源：用户需求“Settings 中 API 管理，除 API Key 外其余信息存数据库”。
- 本次目标：确认现有实现满足需求，并补齐自动化测试覆盖关键分支。
- 完成情况：完成。

## 2. 实现内容
- 代码变更点：
  - `go-backend/pkg/investlog/ai_settings_test.go`
    - 新增/补强用例：
      - 缺省行不存在时回退默认值
      - DB 关闭时 `GetAISettings/SetAISettings` 返回错误
  - `go-backend/internal/api/handlers_ai_settings_test.go`
    - 新增/补强用例：
      - `allow_new_symbols` 缺省时默认 `true`
      - DB 关闭时 `GET/PUT /api/ai-settings` 返回 500
- 文档变更点：
  - `docs/specs/settings-api-management-db-storage/summary-settings-api-management-db-storage.md`（本文件）
- 关键设计调整：
  - 本次未新增设计变更；主逻辑已在当前分支基线上存在并符合需求。

## 3. 测试与质量
- 已执行测试：
  - `go test ./pkg/investlog ./internal/api`
  - `go test -coverprofile=coverage_settings.out ./pkg/investlog ./internal/api`
  - `go tool cover -func=coverage_settings.out | tail -n 1`
  - `go tool cover -func=coverage_settings.out | rg -n "ai_settings.go|handlers.go:275|handlers.go:284"`
- 测试结果：
  - `pkg/investlog`：通过
  - `internal/api`：通过
- 覆盖率结果（含命令与数值）：
  - 组合包总覆盖率：`78.0%`
  - 功能相关代码覆盖率：
    - `pkg/investlog/ai_settings.go`：各函数均 `100.0%`
    - `internal/api/handlers.go` 的 `getAISettings/setAISettings`：`100.0%`

## 4. 风险与已知问题
- 已知限制：
  - `./pkg/investlog + ./internal/api` 组合总覆盖率为 `78.0%`，低于 80%；主要受历史存量模块覆盖率影响。
- 风险评估：
  - 本次需求相关路径（AI settings 核心与接口）覆盖已达 100%，回归风险可控。
- 后续建议：
  - 若需强制总覆盖率 >=80%，需要对存量低覆盖 handler/core 路径做额外补测（超出本次需求范围）。

## 5. 待确认事项
- 需要 Reviewer 确认：
  - 是否接受“需求相关路径覆盖率达标（100%）”作为本次质量门禁。
- 合并前阻塞项：
  - 无功能阻塞；若团队坚持总覆盖率 >=80%，需追加历史模块补测。
