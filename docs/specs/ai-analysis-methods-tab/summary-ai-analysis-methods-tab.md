# 变更总结：AI Analysis 顶级 Tab 与可配置分析方法

## 1. 本次变更
- 新增顶级导航 `AI Analysis`，提供通用 AI 分析工作台。
- 新增分析方法配置能力，支持在 Settings 中对方法执行新增、编辑、删除。
- 新增 `${VAR}` 占位符提取和运行时变量替换。
- 新增通用 AI 分析运行历史持久化，保存方法快照、变量输入、渲染后 Prompt、结果、状态与时间。
- 新增后端 API：
  - `/api/ai-analysis-methods`
  - `/api/ai-analysis/stream`
  - `/api/ai-analysis/history`
  - `/api/ai-analysis/runs/{id}`
- 同步静态资源到 `ios/App/App/public`。

## 2. 关键实现点
- 后端新增 SQLite 表：
  - `ai_analysis_methods`
  - `ai_analysis_runs`
- 后端复用现有 AI Chat Client 与 SSE 事件模式，不引入第二套调用链。
- 历史记录保存方法快照，因此方法被修改或删除后，旧历史仍可回看。
- 前端 `AI Analysis` 页面支持：
  - 方法切换
  - 自动变量输入表单
  - 流式输出
  - 历史列表和结果详情

## 3. 验证结果
- 自动化测试：
  - `cd go-backend && go test ./pkg/investlog/... ./internal/api/...`
  - `cd go-backend && go test ./...`
- 静态检查：
  - `node --check` 已覆盖新增/修改的前端 JS 文件
- 资源同步：
  - `scripts/sync_spa.sh`
- UI 冒烟：
  - 本地服务启动后，Headless Chrome 实际访问了首页并触发多个初始化 API 请求
  - Playwright wrapper 本次没有稳定产出可用 snapshot，因此未完成更深入的可视化交互验证

## 4. 已知限制
- 变量输入首版统一为文本输入，不支持字段类型元数据。
- 占位符仅识别 `${VAR}`（大写字母、数字、下划线），`${symbol}` 不会被提取。
- `Settings > API` 仍然是全局 AI 模型配置，方法本身不支持独立模型配置。
