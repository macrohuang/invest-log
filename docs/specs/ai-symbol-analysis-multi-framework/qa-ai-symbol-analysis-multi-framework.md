# QA 测试用例 Spec：个股 AI 多框架分析（3 Framework + 综合建议）

## 1. 测试目标
- 对应 MRD：`docs/specs/ai-symbol-analysis-multi-framework/mrd-ai-symbol-analysis-multi-framework.md`
- 对应代码范围：
  - `go-backend/pkg/investlog/ai_symbol_analysis.go`
  - `go-backend/pkg/investlog/ai_symbol_analysis_test.go`
- 核心目标：验证“财报优先 + 宏观产业实时信息 + 3 框架分析 + 综合建议”在成功、失败、边界条件下均可预测、可回归、可自动化断言。

## 2. 测试范围
- In Scope：
  - 框架分析产出数量与结构（必须为 3 个框架结果，且来自指定 9 框架池）。
  - 综合建议输出硬约束：
    - `action_probability_percent` 必须为明确数值。
    - 禁止“看情况/视情况而定/it depends”等含混措辞。
    - `disclaimer` 长度 <= 16。
    - 首句直接给结论。
  - 降级流程：外部数据缺失、抓取失败、单框架失败、多框架失败。
  - 权重决策要素可验证性：持仓数量、仓位占比、配置区间、用户偏好/策略。
  - 与既有个股分析接口的兼容回归。
- Out of Scope：
  - 前端展示层视觉和交互动效。
  - 外部数据源供应商 SLA 与内容真实性评估。

### 指定框架池（用于断言合法性）
- `dupont_roic`
- `capital_cycle`
- `industry_s_curve`
- `reverse_dcf`
- `dynamic_moat`
- `dcf`
- `porter_moat`
- `expectations_investing`
- `relative_valuation`

## 3. 测试优先级定义
- P0：阻断发布，影响结果可信度或接口可用性。
- P1：高价值质量项，不阻断主流程，但需在发布前回归通过。

## 4. 测试策略
- 单元测试（主）：
  - 对 `normalizeSynthesisProbability`、`normalizeSynthesisSummary`、`normalizeSynthesisPositionSuggestion`、请求归一化、框架执行与解析流程进行表驱动测试。
  - 对输出 JSON 字段、关键词和数值边界做可自动化断言。
- 集成测试：
  - 基于 `setupTestDB` + stub AI 调用（替换 `aiChatCompletion`）验证 `AnalyzeSymbol` 全流程、降级、持久化与回读。
- 手工/探索性验证（补充）：
  - 对真实模型进行抽样验证，确认提示词约束在生产模型上的遵循度。

## 5. 自动化硬约束断言（必须实现）
| 断言ID | 规则 | 自动化断言建议 |
| --- | --- | --- |
| A-001 | 概率必须为明确数值 | 解析后断言 `action_probability_percent` 为 `float64`，且 `0 < p <= 100`，禁止区间字符串（如 `60-70`）。 |
| A-002 | 禁含混措辞 | 断言 `overall_summary`、`position_suggestion` 不包含 `看情况`、`视情况`、`it depends`。 |
| A-003 | disclaimer 长度 <= 16 | 断言 `len([]rune(disclaimer)) <= 16`。 |
| A-004 | 首句直接结论 | 断言 `overall_summary` 以明确结论句开头（示例：`结论：加仓...`），且第一句包含动作与概率。 |

## 6. 权重决策可验证点
| 验证ID | 关键要素 | 断言方式 | 级别 |
| --- | --- | --- | --- |
| W-001 | 持仓数量 | 综合决策输入中出现 `total_shares` 或等效“持仓数量”字段，并影响行动建议力度。 | P0 |
| W-002 | 仓位占比 | 输入或归一化文本中出现 `position_percent` / `当前占比xx%`。 | P0 |
| W-003 | 配置区间 | 输出含 `目标区间min%-max%` 与 `差值`，且区间缺失时有默认值逻辑。 | P0 |
| W-004 | 用户偏好与策略 | 风险偏好/期限/建议风格/策略偏好进入综合决策输入并影响结论表达。 | P0 |

## 7. 用例清单

### 7.1 正向用例
| 编号 | 优先级 | 类型 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- |
| TC-001 | P0 | 正向 | stub 3 个框架均返回合法 JSON | 调用 `AnalyzeSymbol` | 返回 `status=completed`，且框架结果数量“恰好 3”；综合建议存在。 |
| TC-001A | P0 | 正向 | 候选池为 9 框架 | 调用 `AnalyzeSymbol` | 入选框架“恰好 3 个”，且全部属于指定框架池，不允许池外值。 |
| TC-002 | P0 | 正向 | 3 框架结论有冲突（1 正向、1 中性、1 负向） | 调用 `AnalyzeSymbol` | 综合建议包含冲突权衡逻辑，不是简单拼接。 |
| TC-003 | P0 | 正向 | 合法 synthesis JSON | 解析并归一化结果 | 满足 A-001/A-002/A-004。 |
| TC-004 | P0 | 正向 | 存在持仓与配置 | 调用 `AnalyzeSymbol` | `position_suggestion` 明确包含“当前占比 + 目标区间 + 差值 + 动作”。 |
| TC-005 | P1 | 正向 | 带风险偏好、期限、风格、策略偏好 | 调用 `AnalyzeSymbol` 并捕获综合 prompt | 权重输入中包含用户偏好/策略字段，且综合建议语气与动作强度可观察变化。 |

### 7.2 异常用例
| 编号 | 优先级 | 类型 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- |
| TC-006 | P0 | 异常 | 外部数据返回 `nil` | 调用 `AnalyzeSymbol` | 主流程继续，分析成功返回；外部上下文为空但不崩溃。 |
| TC-007 | P0 | 异常 | 外部抓取失败/摘要失败 | 调用 `AnalyzeSymbol` | 走降级路径继续分析；记录失败日志；结果可返回。 |
| TC-008 | P0 | 异常 | 框架成功数低于最小门槛 | 调用 `AnalyzeSymbol` | 返回错误并更新状态为 failed。 |
| TC-009 | P1 | 异常 | 单个框架输出非法 JSON | 调用 `AnalyzeSymbol` | 非法框架被跳过，达到门槛时整体仍可完成。 |
| TC-010 | P0 | 异常 | synthesis 输出 malformed JSON | 调用 `AnalyzeSymbol` | 返回 `parse synthesis result` 错误，不写入 completed。 |

### 7.3 边界用例
| 编号 | 优先级 | 类型 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- |
| TC-011 | P0 | 边界 | `action_probability_percent=0` 或 `>100` | 归一化处理 | 概率回退到置信度默认值（high/medium/low）。 |
| TC-012 | P0 | 边界 | `disclaimer` 长度 16 与 17 | 解析并归一化 | 16 通过；17 触发修正或测试失败（阻断）。 |
| TC-012A | P0 | 边界 | `disclaimer` 为长免责声明 | 解析并归一化 | 最终 `disclaimer` 必须收敛为短风险锚点（<=16字）。 |
| TC-013 | P1 | 边界 | 无持仓、无配置区间 | 调用 `AnalyzeSymbol` | 仍可分析；仓位文案出现未知或默认区间逻辑。 |
| TC-014 | P1 | 边界 | summary 超长（>200 字） | 归一化处理 | 截断且保持首句结论与句末标点完整。 |

### 7.4 回归用例
| 编号 | 优先级 | 类型 | 前置条件 | 步骤 | 预期结果 |
| --- | --- | --- | --- | --- | --- |
| TC-015 | P1 | 回归 | 既有必填参数校验 | 提交缺失/非法请求 | 保持 `api_key/model/symbol/currency` 校验行为不回归。 |
| TC-016 | P1 | 回归 | 开启流式分析 | 调用 `AnalyzeSymbolWithStream` | 保持维度/综合阶段 delta 输出能力。 |
| TC-017 | P1 | 回归 | 已有历史记录 | 调用 `GetSymbolAnalysis`/`GetSymbolAnalysisHistory` | 历史回读、排序、空结果行为保持稳定。 |
| TC-018 | P1 | 回归 | AI 上下文字段裁剪 | 构建并检查 AI context JSON | 白名单字段策略保持，避免泄漏不必要字段。 |

## 8. 覆盖率策略（目标 >= 80%）
- 覆盖目标：
  - 变更核心包 `pkg/investlog` 的相关分支覆盖率 >= 80%。
  - P0 用例必须全部自动化。
- 推荐命令：
```bash
cd go-backend

# 1) 快速执行目标测试（开发期）
go test -v ./pkg/investlog -run 'TestAnalyzeSymbol|TestNormalizeSynthesis|TestParseSynthesis|TestBuildSymbolContext|TestNormalizeSymbolAnalysisRequest'

# 2) 统计核心包覆盖率
go test ./pkg/investlog -coverprofile=coverage_ai_symbol_multi_framework.out
go tool cover -func=coverage_ai_symbol_multi_framework.out

# 3) 对受影响后端包做回归覆盖率
go test ./pkg/investlog ./internal/api -coverprofile=coverage_ai_symbol_multi_framework_pkg_api.out
go tool cover -func=coverage_ai_symbol_multi_framework_pkg_api.out

# 4) 在 CI 中强制 >=80%
go tool cover -func=coverage_ai_symbol_multi_framework.out | awk '/total:/ { if ($3+0 < 80) { exit 1 } }'
```
- 当前覆盖盲区与补强建议：
  - 盲区：真实模型对 prompt 约束遵循存在漂移风险。
  - 补强：增加“多模型抽样回归”作业（夜间任务），并固化关键词与结构断言。

## 9. 退出标准
- 所有 P0/P1 用例通过。
- A-001 ~ A-004 自动化断言全部通过。
- 降级路径（TC-006 ~ TC-009）全部通过。
- 覆盖率达到或超过 80%。

## 10. 缺陷记录模板
- Defect-ID：
- 对应用例：
- 复现步骤：
- 实际结果：
- 预期结果：
- 严重级别（P0/P1/P2）：
- 修复结果与回归结论：
