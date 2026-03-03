# 个股分析 Perplexity 检索增强 MRD

## 1. 背景与目标
- 背景问题：当前个股 AI 分析页面勾选 Perplexity 后，会将整条分析链路（框架分析 + 综合分析）都切换到 Perplexity，导致用户已配置的主模型失效。
- 业务目标：实现“Perplexity 负责最新信息检索增强，主模型负责框架与综合分析”。
- 成功指标（可量化）：
  - 勾选 Perplexity 时，外部信息摘要阶段使用 Perplexity 配置。
  - 框架分析与综合分析阶段继续使用主模型配置。
  - 新增回归测试覆盖上述行为并稳定通过。

## 2. 用户与场景
- 目标用户：已在设置页配置主模型，并额外配置 Perplexity API Key 的投资用户。
- 核心使用场景：用户在“个股分析”页开启 Perplexity 开关后发起分析，期望保留主模型分析风格，同时引入更实时的信息增强。
- 非目标用户：未配置主模型 API Key、未进行个股分析流程的用户。

## 3. 范围定义
- In Scope：
  - 个股分析请求参数增加“检索增强模型”可选配置。
  - 后端在外部信息摘要阶段支持单独 provider/model。
  - 前端 Perplexity 开关语义调整为“检索增强”而非“整体切换模型”。
  - 对应单元测试与回归测试更新。
- Out of Scope：
  - 持仓分析 (`AnalyzeHoldings`) 的模型路由调整。
  - 新增额外第三方数据源或改造抓取链路。
  - 历史分析结果的数据迁移。

## 4. 需求明细
- 功能需求 FR-1：个股分析接口支持可选的 `retrieval_base_url/retrieval_api_key/retrieval_model`。
- 功能需求 FR-2：若检索增强参数完整有效，则仅外部信息摘要阶段使用该配置；否则回退主模型配置。
- 功能需求 FR-3：框架分析与综合分析始终使用主模型配置，不受 Perplexity 开关影响。
- 非功能需求 NFR-1：检索增强配置无效时必须降级，不得导致主流程失败。

## 5. 约束与依赖
- 技术约束：保持现有 `aiChatCompletion` 与 `summarizeExternalDataFn` 架构，不引入破坏性接口变更。
- 数据约束：不修改已有数据库表结构。
- 外部依赖：Perplexity OpenAI-compatible endpoint (`https://api.perplexity.ai/chat/completions`)。

## 6. 边界与异常
- 边界条件：仅传入部分检索增强字段（例如只有 key 无 model）时，忽略该配置并使用主模型。
- 异常处理：检索增强 endpoint 无法归一化时记录 warning，自动回退主模型摘要配置。
- 失败回退：外部信息摘要失败保持既有 graceful degradation（fallback summary），不影响主分析链路继续执行。

## 7. 验收标准
- AC-1（可验证）：开启 Perplexity 后，摘要阶段请求使用 Perplexity endpoint/api_key/model。
- AC-2（可验证）：同一次分析中框架与综合阶段调用模型仍为主模型，返回结果中的 `model` 字段保持主模型。
- AC-3（可验证）：新增回归测试可复现并覆盖该行为，测试通过。

## 8. 待确认问题
- Q1：前端按钮文案是否固定为“检索增强”语义（本次默认改为该语义）。
- Q2：未来是否需要支持除 Perplexity 以外的检索增强 provider（本次设计保留通用字段，默认由前端填充 Perplexity）。

## 9. 假设与决策记录
- 假设：用户已在设置页配置主模型 API Key；Perplexity 仅作为可选增强。
- 假设：用户原始描述可视为本次范围确认，不额外等待澄清。
- 决策：保持主模型作为分析主引擎，仅将 Perplexity 下沉到外部数据摘要步骤。
