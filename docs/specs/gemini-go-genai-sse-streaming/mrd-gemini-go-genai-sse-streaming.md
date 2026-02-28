# Gemini 原生客户端 + SSE 流式返回 MRD

> 文件名：`docs/specs/gemini-go-genai-sse-streaming/mrd-gemini-go-genai-sse-streaming.md`

## 1. 背景与目标
- 背景问题：当前 AI 分析请求以一次性 HTTP 返回为主，Gemini 通过 OpenAI 兼容协议调用，前端在长耗时分析期间缺少逐步反馈。
- 业务目标：
  - Gemini 调用切换为 Go 原生客户端 `github.com/googleapis/go-genai`。
  - Holdings AI 分析支持 SSE 分块输出，前端实时展示生成进度。
  - `GEMINI_API_KEY` 与 Base URL 继续沿用 Settings 页面配置来源，不新增独立配置入口。
- 成功指标（可量化）：
  - Gemini 模型调用路径不再走现有 OpenAI 兼容 HTTP 分支。
  - `/api/ai/holdings-analysis/stream` 能持续输出 `chunk` 事件并以 `done` 事件结束。
  - 前端“AI 分析”按钮触发后可见实时流式文本，最终展示结构化分析结果。

## 2. 用户与场景
- 目标用户：在 Holdings 页面发起 AI 组合分析的投资用户。
- 核心使用场景：用户点击 `AI` 后，页面实时看到 Gemini 输出片段，分析结束后看到完整结构化卡片。
- 非目标用户：
  - 仅使用非 Gemini 模型且不关注流式反馈的用户。
  - 需要 symbol-analysis 页流式化的用户（本次不覆盖）。

## 3. 范围定义
- In Scope：
  - 后端：Gemini 路径接入 `go-genai`，并新增 holdings 分析 SSE 接口。
  - 前端：Holdings 页面改为消费 SSE，支持“生成中”状态与错误提示。
  - 配置：继续从现有 Settings 流程读取 `base_url`、`model`、`api_key`。
- Out of Scope：
  - Symbol analysis / allocation advice 的前端流式改造。
  - 新增数据库字段或配置表结构变更。
  - 引入前端测试框架。

## 4. 需求明细
- 功能需求 FR-1：当请求判定为 Gemini 调用时，后端使用 `go-genai` 客户端发起内容生成。
- 功能需求 FR-2：新增 `POST /api/ai/holdings-analysis/stream`，响应类型为 `text/event-stream`。
- 功能需求 FR-3：SSE 至少包含三类事件：`start`、`chunk`、`done`；失败时返回 `error` 事件。
- 功能需求 FR-4：前端将 holdings AI 分析入口切换到 SSE 接口，并实时渲染分块文本。
- 功能需求 FR-5：流式结束后，前端状态与历史分析列表更新逻辑保持与当前一致。
- 非功能需求 NFR-1（性能/安全/可靠性等）：
  - 不在日志输出完整 API key。
  - SSE 连接应可在客户端断开时尽快停止后端处理。
  - 非 Gemini 模型路径保持现有行为，避免回归。

## 5. 约束与依赖
- 技术约束：
  - 后端为 Go 1.22+；前端为原生 `static/app.js`。
  - iOS 公共静态资源通过 `scripts/sync_spa.sh` 同步。
- 数据约束：AI settings 表不存储 API key，仍由前端请求体传入。
- 外部依赖：`github.com/googleapis/go-genai`。

## 6. 边界与异常
- 边界条件：
  - Base URL 为空时走默认值，Gemini 识别仍可依据 model/base_url 双条件判定。
  - 流式 chunk 可能为空文本，前端应忽略空增量。
- 异常处理：
  - 上游失败时 SSE 发送 `error` 事件，前端展示截断后错误信息。
  - 若 SSE 建连失败，前端按失败提示处理，不更新历史。
- 失败回退：非 Gemini 模型沿用现有一次性请求链路。

## 7. 验收标准
- AC-1（可验证）：设置 `model=gemini-*` 后，holdings 分析可通过 SSE 连续收到 chunk。
- AC-2（可验证）：最终 `done` 事件包含完整结构化结果，页面显示正常分析卡片。
- AC-3（可验证）：`base_url/api_key/model` 依旧来自 Settings（不新增额外配置输入）。
- AC-4（可验证）：现有 `POST /api/ai/holdings-analysis` 仍可正常工作（回归保障）。

## 8. 待确认问题
- Q1：后续是否需要将 symbol-analysis 也统一为 SSE？（本次先不做）
- Q2：是否需要在 UI 增加“停止生成”按钮？（本次先不做）

## 9. 假设与决策记录
- 假设：当前需求优先覆盖 Holdings 页面的 Gemini 流式体验。
- 决策：先实现 holdings SSE 端到端；Gemini 统一在后端传输层接入 go-genai，以便后续扩展到其他 AI 场景。
