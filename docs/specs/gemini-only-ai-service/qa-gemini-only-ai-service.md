# QA 测试用例 Spec

> 文件名：`docs/specs/gemini-only-ai-service/qa-gemini-only-ai-service.md`

## 1. 测试目标
- 对应 MRD：`mrd-gemini-only-ai-service.md`
- 对应设计文档：`design-gemini-only-ai-service.md`
- 验证所有 AI 功能仅通过 Gemini 协议访问，且统一走 AICodeMirror Gemini 兼容端点
- 验证旧的 OpenAI/Perplexity 风格配置会被安全归一化或明确拒绝
- 验证个股分析不再接受 `retrieval_base_url`、`retrieval_api_key`、`retrieval_model`

## 2. 测试范围
- In Scope：
  - `go-backend/pkg/investlog` 的 Gemini 端点归一化、请求体编码、错误处理、配置迁移
  - `go-backend/internal/api` 的 AI 设置、持仓分析、配置建议、个股分析及流式接口
  - `static/` 与 `ios/App/App/public` 的 AI 设置页与个股分析页文案/字段收口
  - 日志脱敏与回归验证
- Out of Scope：
  - 非 AI 功能（交易、持仓、价格更新等）
  - 第三方 Gemini 服务真实可用性
  - 浏览器自动化 E2E（本次以接口自动化 + 手工 UI 验证为主）

## 3. 测试策略
- 单元测试：
  - 优先覆盖纯函数与 transport 逻辑：base URL 归一化、Gemini endpoint 拼接、请求头选择、请求体序列化、旧配置迁移
  - 使用表驱动测试覆盖正向、异常、边界输入
- 集成测试：
  - 以 `httptest.Server` 模拟 Gemini SSE/JSON 响应
  - 通过 API handler 验证请求入参、出站 header、出站 path、流式事件、错误码与响应体
- 端到端/手工验证：
  - 手工验证 Settings 页、Holdings 页、Symbol Analysis 页在 Web 与 iOS 镜像资源中的一致性
  - 验证移除 Perplexity 设置后无残留入口或误导文案

## 4. 用例清单

### A. 配置与迁移

- TC-001（正向）：默认 AI 设置改为 Gemini
  - 前置条件：新建数据库/空 `ai_settings`
  - 步骤：调用 `GET /api/ai-settings`
  - 预期结果：
    - `base_url` 返回 AICodeMirror Gemini 基地址
    - `model` 默认空或 Gemini 示例允许的空值
    - 不再返回 OpenAI 默认地址

- TC-002（正向）：保存 Gemini 设置后读回归一化值
  - 前置条件：无
  - 步骤：`PUT /api/ai-settings` 提交带尾斜杠或半成品 base URL 的 Gemini 配置
  - 预期结果：
    - `base_url` 被裁剪并归一化
    - `model`、`api_key` 原样保留（仅 trim）
    - 风险偏好等现有字段不受影响

- TC-003（迁移）：旧 OpenAI 配置自动迁移
  - 前置条件：数据库中已有 `https://api.openai.com/v1` 或等价旧值
  - 步骤：调用 `GET /api/ai-settings`
  - 预期结果：
    - 返回值已被映射为 Gemini 基地址
    - 不需要用户手工修复即可继续使用

- TC-004（迁移）：旧 Perplexity 配置自动迁移
  - 前置条件：数据库中已有 Perplexity 风格 `base_url`
  - 步骤：调用 `GET /api/ai-settings`
  - 预期结果：
    - 返回值被映射为 Gemini 基地址
    - 不保留 Perplexity 作为可运行 provider

- TC-005（异常）：保存非 Gemini 模型或非 Gemini provider
  - 前置条件：无
  - 步骤：`PUT /api/ai-settings` 提交 `model=gpt-4o-mini` 或显式 OpenAI/Perplexity base URL
  - 预期结果：
    - 若设计为自动迁移：返回已迁移后的 Gemini 配置
    - 若设计为拒绝保存：返回 4xx，错误信息明确说明仅支持 Gemini
  - 备注：该行为需与设计文档保持一致，只允许一种最终语义

### B. Gemini Transport

- TC-006（正向）：Gemini SSE endpoint 拼接正确
  - 前置条件：无
  - 步骤：用不同输入调用 endpoint 构造逻辑：
    - 裸域名
    - AICodeMirror Gemini 基地址
    - 已带 `/v1beta`
    - 已带 `/models/...:streamGenerateContent`
  - 预期结果：
    - 最终都归一化为 `/api/gemini/v1beta/models/{model}:streamGenerateContent?alt=sse`
    - 不重复拼接 `/v1`、`/v1beta`、`models`

- TC-007（正向）：Gemini 请求头正确
  - 前置条件：模拟上游服务
  - 步骤：发起任意 AI 分析请求
  - 预期结果：
    - 出站 header 包含 `x-goog-api-key`
    - 不包含 `Authorization: Bearer ...`

- TC-008（正向）：Gemini 请求体编码正确
  - 前置条件：模拟上游服务，抓取 request body
  - 步骤：执行持仓分析/配置建议/个股分析
  - 预期结果：
    - 包含 `systemInstruction.parts[].text`
    - 包含 `contents[].parts[].text`
    - 包含 `generationConfig.temperature`
    - 包含 `generationConfig.maxOutputTokens`
    - 不再发送 OpenAI chat-completions 或 responses 形状

- TC-009（异常）：Gemini endpoint 输入非法
  - 前置条件：无
  - 步骤：传入空模型、非法 scheme、空 host
  - 预期结果：
    - 返回明确错误信息
    - handler 层返回 4xx，不出现 panic

### C. 持仓分析 / 配置建议

- TC-010（正向）：持仓分析同步接口走 Gemini 并成功聚合结果
  - 前置条件：有最小持仓数据；上游模拟 SSE 返回
  - 步骤：调用 `POST /api/ai/holdings-analysis`
  - 预期结果：
    - 服务端走 Gemini SSE 通道
    - 聚合后成功解析结构化结果
    - 响应中的 `model`、摘要、建议字段完整

- TC-011（正向）：持仓分析流式接口事件完整
  - 前置条件：有最小持仓数据；上游模拟 SSE 分块返回
  - 步骤：调用 `POST /api/ai/holdings-analysis/stream`
  - 预期结果：
    - 输出 `progress` / `chunk` / `result` / `done`（按现有实现约定）
    - 最终 payload 可被前端正常消费

- TC-012（正向）：配置建议同步接口走 Gemini
  - 前置条件：准备最小画像数据；上游模拟有效 Gemini 返回
  - 步骤：调用 `POST /api/ai/allocation-advice`
  - 预期结果：
    - 出站 path/header/body 符合 Gemini-only 约束
    - 正常返回建议结果

- TC-013（异常）：配置建议/持仓分析提交非 Gemini 配置
  - 前置条件：无
  - 步骤：提交非 Gemini 模型或未迁移旧 base URL
  - 预期结果：
    - 返回 4xx 或在保存阶段已被迁移
    - 错误信息对用户可理解

### D. 个股分析与 `retrieval_*` 下线

- TC-014（契约）：`retrieval_*` 字段从 API 契约移除
  - 前置条件：无
  - 步骤：向 `POST /api/ai/symbol-analysis` 与 `/stream` 提交 `retrieval_base_url`、`retrieval_api_key`、`retrieval_model`
  - 预期结果：
    - 若 `decodeJSON` 禁未知字段：返回 400
    - 错误信息指向未知字段/不再支持该字段

- TC-015（正向）：个股分析仅使用主 Gemini 配置
  - 前置条件：存在 symbol 持仓；上游模拟多轮 Gemini 返回
  - 步骤：调用个股分析同步接口
  - 预期结果：
    - 所有维度分析、综合分析、外部信息摘要均复用同一主 Gemini 配置
    - 无第二 provider 分支被调用

- TC-016（正向）：个股分析流式接口继续工作
  - 前置条件：同上
  - 步骤：调用 `/api/ai/symbol-analysis/stream`
  - 预期结果：
    - 流式 progress/result/done 事件完整
    - 返回结果包含 synthesis 与框架分析

- TC-017（回归）：个股分析在没有 `retrieval_*` 字段时仍能完成
  - 前置条件：存在 symbol 持仓；只提交主 Gemini 配置
  - 步骤：调用同步/流式接口
  - 预期结果：
    - 无校验错误
    - 外部摘要逻辑不依赖旧 retrieval provider

### E. 日志、安全与错误处理

- TC-018（安全）：`x-goog-api-key` 日志脱敏
  - 前置条件：启用请求日志
  - 步骤：构造 Gemini 请求并抓取格式化日志
  - 预期结果：
    - key 被掩码
    - 不泄漏完整 token

- TC-019（异常）：上游 4xx/5xx 错误被正确透传
  - 前置条件：模拟上游返回错误 body
  - 步骤：调用任意 AI 接口
  - 预期结果：
    - 返回统一、可读的错误信息
    - 流式接口输出 `error`/`done=false` 等约定信号

- TC-020（异常）：SSE 中断或空内容
  - 前置条件：上游提前断流/返回空 chunk
  - 步骤：调用同步与流式接口
  - 预期结果：
    - 同步接口返回解析失败或上游异常
    - 流式接口输出错误事件，不出现 hang/panic

### F. 前端 / 手工验证

- TC-021（手工）：Settings 页仅保留 Gemini 相关设置
  - 步骤：打开 Web SPA 设置页与 iOS 镜像页面
  - 预期结果：
    - 文案为 Gemini-only
    - `Perplexity API Key` 输入与说明已移除
    - `AI Base URL` 默认值为 AICodeMirror Gemini 基地址
    - 模型 placeholder 为 Gemini 示例值

- TC-022（手工）：保存旧配置时被自动迁移
  - 步骤：用旧 localStorage / 已保存数据库配置进入页面后保存
  - 预期结果：
    - 页面显示 Gemini 基地址
    - 保存成功后可直接发起 AI 分析

- TC-023（手工）：Holdings / Symbol Analysis 页面分析功能正常
  - 步骤：分别触发持仓分析与个股分析
  - 预期结果：
    - 不再出现 Perplexity 依赖提示
    - 结果与流式状态正常展示

## 5. 覆盖率策略
- 统计口径（命令/工具）：
  - `go test ./pkg/investlog ./internal/api -coverprofile=coverage_gemini_only.out`
  - `go tool cover -func=coverage_gemini_only.out`
- 当前覆盖盲区：
  - 前端 UI 无自动化测试
  - 个股分析多分支 Gemini 错误路径可能覆盖不足
  - 配置迁移与旧 provider 兼容分支容易遗漏
- 提升计划（目标 >=80%）：
  - 优先补 transport/normalization 的表驱动测试
  - 对 handlers 的同步/流式接口分别增加正向与异常用例
  - 对 symbol analysis 移除 `retrieval_*` 后新增契约回归测试
  - 将日志脱敏与 SSE 断流作为高价值补点

## 6. 退出标准
- 所有 P0/P1 用例通过
- `go test ./pkg/investlog ./internal/api` 通过
- `go test ./...` 通过
- 受影响范围覆盖率达到或超过 80%
- Web 与 iOS 镜像手工验证完成，确认无 Perplexity 残留入口

## 7. 缺陷记录
- Defect-1：若发现仍有某条 AI 链路走 OpenAI/Perplexity 分支，记录调用入口、上游 path、根因、修复与回归结果
- Defect-2：若旧配置迁移后前端显示与后端保存不一致，记录字段差异、归一化逻辑根因、修复与回归结果
- Defect-3：若 symbol-analysis 在移除 `retrieval_*` 后丢失外部摘要，记录触发路径、回退逻辑、修复与回归结果
