# QA 设计：AI Analysis 顶级 Tab 与可配置分析方法

## 1. 测试目标
- 验证分析方法配置、变量提取、流式执行、历史持久化四条主链路正确工作。
- 确保新能力不影响现有 holdings/symbol AI 分析功能。

## 2. 测试范围
- In Scope
  - 分析方法 CRUD
  - `${VAR}` 提取与去重
  - Prompt 渲染与变量缺失校验
  - SSE 执行流程
  - 运行历史持久化与回读
  - 顶级导航与新页面基础交互
- Out of Scope
  - AI 内容质量的投资正确性评估
  - 多语言文案准确性

## 3. 测试策略

### 3.1 单元测试（Go）
- 变量提取函数
- Prompt 渲染函数
- 方法 CRUD 逻辑
- 历史保存与读取
- 运行失败状态回写

### 3.2 API 测试（Go + httptest）
- 方法管理接口
- 运行接口校验分支
- SSE 事件顺序和结果结构
- 历史查询接口

### 3.3 前端手工验证
- 导航进入 `AI Analysis`
- 方法切换、变量输入、流式输出
- Settings 中增删改方法
- 历史查看和方法删除后的历史兼容性

## 4. 核心测试用例

### 4.1 方法管理
- TC-001：创建方法成功
  - 输入：合法 `name/system_prompt/user_prompt`
  - 预期：200；返回 `id` 和提取出的 `variables`
- TC-002：方法名重复
  - 预期：400；错误可读
- TC-003：更新方法成功
  - 预期：变量列表随 Prompt 变化同步更新
- TC-004：删除方法成功
  - 预期：方法列表不再返回该记录

### 4.2 变量提取与渲染
- TC-005：同时从 system/user prompt 提取变量
  - 输入：两个 Prompt 都含 `${SYMBOL}`
  - 预期：只生成一个 `SYMBOL`
- TC-006：重复变量去重并保持首次出现顺序
- TC-007：不符合 `${VAR}` 规则的占位符不识别
  - 输入：`${symbol}`
  - 预期：不进入变量列表
- TC-008：缺少变量值
  - 预期：400
- TC-009：变量替换成功
  - 预期：渲染后 Prompt 中不存在 `${VAR}`

### 4.3 分析执行
- TC-010：运行成功并流式返回
  - 预期：收到 `progress -> delta* -> result -> done`
- TC-011：AI 上游失败
  - 预期：收到 `error -> done`；历史记录状态为 `failed`
- TC-012：未配置 API Key 或 Model
  - 预期：400
- TC-013：方法不存在
  - 预期：404

### 4.4 历史持久化
- TC-014：完成后写入历史
  - 预期：历史包含变量、模板快照、渲染后 Prompt、结果文本
- TC-015：删除方法后历史仍可读取
  - 预期：历史通过快照展示原方法名和 Prompt
- TC-016：按 `method_id` 过滤历史
  - 预期：仅返回对应方法记录
- TC-017：历史详情读取单条记录
  - 预期：字段完整

### 4.5 前端交互
- TC-018：导航栏进入 `AI Analysis`
- TC-019：切换方法后变量表单更新
- TC-020：运行中按钮禁用，完成后恢复
- TC-021：刷新页面后历史仍可查看
- TC-022：Settings 中新增、编辑、删除方法后页面状态正确刷新

## 5. 自动化测试映射
- `go-backend/pkg/investlog/ai_analysis_methods_test.go`
  - 方法 CRUD、变量提取、Prompt 渲染
- `go-backend/pkg/investlog/ai_analysis_service_test.go`
  - 成功流、失败流、历史写入
- `go-backend/internal/api/handlers_ai_analysis_methods_test.go`
  - 方法接口测试
- `go-backend/internal/api/handlers_ai_analysis_stream_test.go`
  - SSE 事件与错误分支
- `go-backend/pkg/investlog/schema_migration_test.go`
  - 新表迁移校验

## 6. 回归检查
- `GET/PUT /api/ai-settings` 不受影响
- holdings/symbol analysis 接口和 SSE 行为不回归
- Settings 现有账户、汇率、资产类型操作不受影响

## 7. 覆盖率目标
- 新增 Go 逻辑覆盖率 >= 80%
- 关键失败路径必须自动化覆盖：
  - 缺变量
  - 方法不存在
  - 名称重复
  - 上游 AI 失败
  - 历史回读

## 8. 退出标准
- 关键接口测试通过
- `go test ./...` 通过
- 手工验证通过：
  - 新导航
  - 方法管理
  - 流式执行
  - 历史查看
