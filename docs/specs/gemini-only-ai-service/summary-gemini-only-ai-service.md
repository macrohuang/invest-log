# 变更总结

## 1. 变更概述
- 需求来源：将 InvestLog 的 AI 能力统一收口为 Gemini-only，并按 AICodeMirror Gemini 兼容接口访问。
- 本次目标：后端只通过 Gemini 协议发起 AI 请求，前端/移动端只暴露 Gemini 配置，移除个股分析中的 retrieval provider 契约。
- 完成情况：已完成后端 Gemini-only 归一化、API 契约收缩、前端与 iOS 入口同步、README 更新，以及自动化测试与覆盖率验证。

## 2. 实现内容
- 代码变更点：
  - 后端默认 AI Base URL 改为 `https://api.aicodemirror.com/api/gemini`，默认模型改为 `gemini-2.5-flash`。
  - `AISettings`、持仓分析、配置建议、个股分析请求在运行前统一归一化到 Gemini-only 配置。
  - Gemini 请求体补齐 `generationConfig.temperature` 与 `generationConfig.maxOutputTokens`。
  - `POST /api/ai/symbol-analysis` 与 `/stream` 不再接受 `retrieval_base_url`、`retrieval_api_key`、`retrieval_model`。
  - 个股分析的外部摘要与补充上下文统一复用主 Gemini 配置。
  - SPA 设置页与个股分析页移除 Perplexity 专属入口，默认/占位改为 AICodeMirror + `gemini-2.5-flash`。
  - 通过 `scripts/sync_spa.sh` 将 `static/` 同步到 `ios/App/App/public`，确保 Web / iOS 行为一致。
- 文档变更点：
  - 新增 MRD、设计、QA、Summary 文档到 `docs/specs/gemini-only-ai-service/`。
  - 更新 `go-backend/README.md` 中 AI provider 描述为 Gemini-only。
- 关键设计调整：
  - 旧 OpenAI / Perplexity / Google Gemini base URL 会在读取/保存时自动迁移为 AICodeMirror Gemini Base URL。
  - 非 Gemini 模型会自动归一化为 `gemini-2.5-flash`，避免遗留配置继续走多 provider 分支。

## 3. 测试与质量
- 已执行测试：
  - `go test ./...`
  - `go test ./pkg/investlog ./internal/api -coverprofile=coverage_gemini_only.out && go tool cover -func=coverage_gemini_only.out`
  - `node --check static/modules/ai-settings.js`
  - `node --check static/modules/pages/settings.js`
  - `node --check static/modules/pages/symbol-analysis.js`
  - `node --check static/modules/state.js`
  - `node --check static/app.js`
  - `node --check ios/App/App/public/app.js`
- 测试结果：
  - Go 全量测试通过。
  - 关键 API 流式与同步回归通过，包括 AI settings、持仓分析、配置建议、个股分析，以及 `retrieval_*` 下线契约。
  - 静态资源语法检查通过。
- 覆盖率结果（含命令与数值）：
  - `investlog/pkg/investlog`: `81.1%`
  - `investlog/internal/api`: `80.7%`
  - 合并统计 total: `81.1%`

## 4. 风险与已知问题
- 已知限制：
  - 前端仍保留少量 legacy 常量/清理逻辑，仅用于迁移旧本地配置，不再作为运行时 provider 使用。
  - `ai_chat_client.go` 里仍保留部分旧 provider 兼容辅助逻辑，但当前业务入口均已在调用前归一化到 Gemini-only。
- 风险评估：
  - AICodeMirror 与 Gemini 官方 SSE 细节若将来出现兼容差异，可能需要继续补充 transport 适配测试。
  - 旧数据库中已保存的 base/model 将在读写时迁移；若外部脚本直接写入旧 provider 值，首次读取/保存后才会收口。
- 后续建议：
  - 若要进一步彻底清理历史分支，可后续做一轮只针对 `ai_chat_client.go` 的 dead-code 精简。
  - 可补充一条手工 smoke test，验证 iOS 容器内 Settings / Symbol Analysis 页面交互流程。

## 5. 待确认事项
- 需要 Reviewer 确认：
  - 是否接受默认模型固定为 `gemini-2.5-flash` 的迁移策略。
  - 是否接受保留少量 legacy 迁移常量而不立刻删除所有底层兼容辅助代码。
- 合并前阻塞项：
  - 无；代码、测试、覆盖率和文档已齐备。
