# 技术设计：AI Analysis 顶级 Tab 与可配置分析方法

## 1. 设计目标
- 在现有 SPA + Go API + SQLite 架构上新增一个“通用 AI 分析工作台”。
- 最小化复用现有 AI 设置和 SSE 能力，避免为新能力引入第二套 AI 调用栈。
- 保证方法配置和运行历史可审计、可回放。

## 2. 现状
- 前端已有顶级路由：`overview / holdings / charts / transactions / settings`。
- 后端已有：
  - 全局 AI 设置表 `ai_settings`
  - 多个 AI 分析接口及 SSE 输出模式
  - 持仓/标的分析历史持久化模式
- 缺失：
  - 通用 AI 分析方法配置模型
  - 通用 AI 分析历史表
  - `AI Analysis` 页面与对应路由

## 3. 总体方案

### 3.1 模块划分
- 后端新增：
  - `AIAnalysisMethod` 配置模型与 CRUD
  - `AIAnalysisRun` 历史模型与查询
  - `RunAIAnalysis` / `RunAIAnalysisStream` 通用执行服务
- 前端新增：
  - `#/ai-analysis` 页面
  - Settings 中的“Analysis Methods”管理区
  - 变量提取、表单生成、历史展示逻辑

### 3.2 执行流程
1. 前端加载方法列表。
2. 用户选择方法。
3. 前端从方法的 `system_prompt` 和 `user_prompt` 提取 `${VAR}`，渲染输入表单。
4. 用户填写变量并点击运行。
5. 前端调用 `/api/ai-analysis/stream`。
6. 后端校验方法、变量、AI 设置。
7. 后端渲染 Prompt，写入一条 `running` 状态历史记录。
8. 后端调用通用 AI Chat Client，以 SSE 将 `delta` 持续发回前端。
9. 完成后更新历史记录为 `completed` 并返回 `result` 事件。
10. 页面刷新后通过历史接口读取已持久化结果。

## 4. 数据模型

### 4.1 新增方法表
- 表名：`ai_analysis_methods`
- 字段：
  - `id INTEGER PRIMARY KEY AUTOINCREMENT`
  - `name TEXT NOT NULL UNIQUE`
  - `system_prompt TEXT NOT NULL`
  - `user_prompt TEXT NOT NULL`
  - `created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`
  - `updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`

用途：
- 存储 Settings 中管理的可执行分析方法。

### 4.2 新增运行历史表
- 表名：`ai_analysis_runs`
- 字段：
  - `id INTEGER PRIMARY KEY AUTOINCREMENT`
  - `method_id INTEGER`
  - `method_name TEXT NOT NULL`
  - `system_prompt_template TEXT NOT NULL`
  - `user_prompt_template TEXT NOT NULL`
  - `variables_json TEXT NOT NULL`
  - `rendered_system_prompt TEXT NOT NULL`
  - `rendered_user_prompt TEXT NOT NULL`
  - `model TEXT NOT NULL`
  - `status TEXT NOT NULL CHECK(status IN ('running','completed','failed'))`
  - `result_text TEXT`
  - `error_message TEXT`
  - `created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`
  - `completed_at DATETIME`

用途：
- 保留方法快照和渲染后 Prompt，确保方法被修改或删除后，历史仍可追溯。

### 4.3 Go 类型
- `AIAnalysisMethod`
- `AIAnalysisRun`
- `RunAIAnalysisRequest`
- `RunAIAnalysisResult`

建议放置：
- `go-backend/pkg/investlog/ai_analysis_methods.go`
- `go-backend/pkg/investlog/ai_analysis_runs.go`
- `go-backend/pkg/investlog/ai_analysis_service.go`

## 5. 变量提取与替换规则

### 5.1 提取规则
- 正则：`\$\{([A-Z0-9_]+)\}`
- 同时扫描 `system_prompt` 与 `user_prompt`
- 去重后按首次出现顺序输出变量名数组

理由：
- 与用户要求的 `${VAR}` 语法一致
- 避免首版支持过宽语法带来歧义

### 5.2 替换规则
- 对每个变量执行纯字符串替换：
  - `${SYMBOL}` -> `AAPL`
- 不支持默认值、条件、函数、嵌套模板。
- 若某个提取出的变量未提供值，接口返回 `400`。

### 5.3 安全边界
- 不执行模板语言，不对输入做脚本解释。
- 仅将用户输入视为普通字符串传入 Prompt。

## 6. API 设计

### 6.1 方法管理
- `GET /api/ai-analysis-methods`
  - 返回所有方法，含变量列表
- `POST /api/ai-analysis-methods`
  - 创建方法
- `PUT /api/ai-analysis-methods/{id}`
  - 更新方法
- `DELETE /api/ai-analysis-methods/{id}`
  - 删除方法

请求体：
- `name`
- `system_prompt`
- `user_prompt`

响应体建议包含：
- `id`
- `name`
- `system_prompt`
- `user_prompt`
- `variables`
- `created_at`
- `updated_at`

### 6.2 运行分析
- `POST /api/ai-analysis/stream`

请求体：
- `method_id`
- `variables`：`map[string]string`

后端执行时从 `ai_settings` 中读取：
- `base_url`
- `model`
- `api_key`

SSE 事件：
- `progress`
- `delta`
- `result`
- `error`
- `done`

`result` 事件返回：
- `id`
- `method_id`
- `method_name`
- `variables`
- `result_text`
- `model`
- `status`
- `created_at`
- `completed_at`

### 6.3 历史查询
- `GET /api/ai-analysis/history?method_id=<id>&limit=<n>`
- `GET /api/ai-analysis/runs/{id}`

查询结果包含：
- 方法快照
- 变量值
- 渲染后 Prompt
- 最终结果
- 状态与时间

## 7. 后端实现细节

### 7.1 Schema 迁移
- 在 `schema.go` 中新增两个 `CREATE TABLE IF NOT EXISTS`
- 增加索引：
  - `idx_ai_analysis_methods_name`
  - `idx_ai_analysis_runs_method_created`

### 7.2 Core 服务
- 方法 CRUD：
  - `ListAIAnalysisMethods`
  - `CreateAIAnalysisMethod`
  - `UpdateAIAnalysisMethod`
  - `DeleteAIAnalysisMethod`
- 历史：
  - `ListAIAnalysisRuns`
  - `GetAIAnalysisRun`
- 执行：
  - `RunAIAnalysis`
  - `RunAIAnalysisStream`

### 7.3 执行逻辑
- 读取方法
- 提取变量集合
- 校验请求中的变量是否覆盖全部占位符
- 使用模板和变量渲染 Prompt
- 插入 `running` 记录
- 复用现有 AI Chat Client 流式调用
- 流式内容累积到 `strings.Builder`
- 完成后更新 `result_text/status/completed_at`
- 异常时更新 `failed/error_message/completed_at`

### 7.4 Prompt 组织
- `system_prompt` 直接作为系统消息
- `user_prompt` 直接作为用户消息
- 不再额外注入持仓、交易、symbol 上下文

## 8. 前端实现细节

### 8.1 新路由与导航
- `static/index.html` 顶部导航新增 `AI Analysis`
- `static/modules/router.js` 新增 `ai-analysis` 路由
- 新页面文件：`static/modules/pages/ai-analysis.js`

### 8.2 页面结构
- 方法选择区
- 自动生成的变量输入区
- 运行按钮
- 流式输出区
- 历史列表区
- 历史详情区

首版推荐布局：
- 上部表单 + 右侧输出/下部历史，保持与现有卡片式风格一致

### 8.3 Settings 页面
- 在现有 AI Analysis 配置卡片后新增 `Analysis Methods` 卡片
- 包含：
  - 方法列表
  - 新增/编辑表单
  - 删除操作
- 前端直接复用后端返回的 `variables` 做辅助展示，不单独保存变量定义

### 8.4 状态管理
- `state.js` 增加：
  - `aiAnalysisMethods`
  - `aiAnalysisRuns`
  - `aiAnalysisActiveMethodId`
  - `aiAnalysisStreaming`

### 8.5 流式交互
- 复用现有 `postSSE`
- 运行期间禁用重复提交
- `delta` 逐步拼接为可见文本
- `result` 到达后刷新历史列表

## 9. 错误处理
- 方法不存在：`404`
- 名称重复：`400`
- Prompt 为空：`400`
- 缺少变量：`400`
- 未配置模型或 API Key：`400`
- AI 调用失败：SSE 返回 `error`，同时持久化 `failed` 历史

## 10. 兼容性与迁移
- 现有 holdings/symbol analysis 行为不变
- `ai_settings` 继续仅负责全局 AI 连接设置
- 历史快照设计保证删除方法后不会破坏历史浏览
- iOS 包内静态资源继续通过 `scripts/sync_spa.sh` 同步

## 11. 风险与折中
- 首版变量没有类型元数据，复杂输入体验有限
- `${VAR}` 语法限制为大写风格，旧 Prompt 若使用 `${symbol}` 不会被识别
- 通用文本结果不做结构化解析，首版以完整文本回显为主

## 12. 实施顺序
1. 新增数据库表和 Go 数据模型
2. 实现方法 CRUD 与历史查询接口
3. 用 TDD 实现通用分析执行服务与 SSE 接口
4. 新增 `AI Analysis` 页面
5. 扩展 Settings 方法管理 UI
6. 补充测试并同步 SPA 到 iOS 公共目录
