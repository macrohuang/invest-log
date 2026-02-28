# 技术设计：AI 标的分析多框架（3选）与综合决策

> 文件名：`docs/specs/ai-symbol-analysis-multi-framework/design-ai-symbol-analysis-multi-framework.md`

## 1. 设计目标
- 对应 MRD：`docs/specs/ai-symbol-analysis-multi-framework/mrd-ai-symbol-analysis-multi-framework.md`。
- 在现有 `AnalyzeSymbol` 流程上，将固定四维改造为“给定框架池中动态选择 3 个框架”。
- 引入可解释、可测试的框架选择规则，避免纯提示词黑盒选择。
- 综合建议必须显式纳入：持仓数量、仓位占比、资产配置区间、用户偏好策略。
- 强化输出硬约束：首句结论、明确概率、禁含混表达、短免责声明（<=16 字）。
- 保持现有 API 和历史数据可读，支持灰度、降级、回滚。

非目标：
- 不引入自动交易/自动下单。
- 不重构数据库主模型，不做历史回填。

## 2. 现状分析
- 相关模块：
  - `go-backend/pkg/investlog/ai_symbol_analysis.go`
  - `go-backend/pkg/investlog/ai_symbol_data_fetcher.go`
- 当前实现：
  - 维度代理固定为 `macro/industry/company/international`，并行全量执行。
  - 已有仓位相关上下文（`position_percent`、`allocation_*`）和用户偏好（`risk_profile/horizon/advice_style`）。
  - 综合输出已包含概率字段与首句约束提示，但缺少严格后置校验与冲突量化权衡。
  - 外部数据抓取已支持新闻/财务/研报，但未结构化成“近5季+近3年+宏观/产业/经营”优先证据层。
- 主要限制：
  - 框架数量固定，无法按标的特征做 3 选优。
  - 输入证据缺少统一优先级与覆盖度判定。
  - 综合建议对“持仓数量”的权重未显式建模。

## 3. 方案概述
- 总体架构：
  1. `Evidence Builder`：构建结构化证据包并按优先级排序。
  2. `Framework Selector`：对给定框架池打分，确定 Top-3。
  3. `Framework Runners`：并行执行 3 个框架 Agent。
  4. `Conflict Resolver + Synthesizer`：做冲突权衡并输出综合建议。
  5. `Output Validator`：执行硬约束校验与兜底修正。
- 关键流程：
  1. 归一化请求 + 构建 symbol/持仓/偏好上下文。
  2. 从外部与内部数据构建证据层（含优先级、覆盖度、时效性）。
  3. 框架池评分并选择 3 个框架。
  4. 并行生成每框架结构化输出。
  5. 综合层融合框架结论与持仓权重因子。
  6. 执行冲突权衡逻辑并产出最终动作。
  7. 执行输出硬约束校验；不满足则自动修正。
  8. 兼容写回历史结构并返回新字段。
- 关键设计决策：
  - 选择逻辑“代码打分 + 决定性排序”，不依赖模型临场自由发挥。
  - 冲突时“规则优先于文风”，先给动作再给理由。
  - 对旧客户端保持 `dimensions + synthesis` 可读，新增字段全可选。

## 4. 详细设计
### 4.1 输入优先级模型（近5季+近3年+宏观+产业+经营）
- 新增证据类型（`EvidenceType`）：
  - `quarterly_financials_5q`
  - `annual_financials_3y`
  - `macro_policy`
  - `industry_cycle`
  - `company_operation_updates`
- 固定优先级顺序（从高到低）：
  1. 近5季财报（P0）
  2. 近3年年报（P1）
  3. 宏观政策（P2）
  4. 产业周期（P3）
  5. 公司经营动态（P4）
- 证据质量分：
  - `evidence_score = priority_weight * freshness_weight * source_reliability * completeness`
  - `priority_weight`：P0=1.00, P1=0.92, P2=0.82, P3=0.76, P4=0.70
  - `freshness_weight`：7日内=1.0；30日内=0.85；90日内=0.65；>90日=0.40
- Prompt 注入策略：
  - 按优先级分段注入，每段携带 `coverage_ratio` 与 `as_of_date`。
  - 若 P0/P1 不足，必须在段内显式标注“财报覆盖不足”，并降低对应框架置信。
- 数据来源映射（与现有实现对齐）：
  - 财报/估值：`parseEastmoneyFinancials`、`parseYahooQuoteSummary`
  - 宏观/产业/经营：`news/research` 数据经 `summarizeExternalDataFn` 结构化提炼

### 4.2 框架池与 3 框架选择规则（可解释、可测试）
- 框架池（严格按需求）：
  - `dupont_roic`：杜邦分析 + ROIC 拆解（财务质量框架）
  - `capital_cycle`：资本周期框架 (Capital Cycle)
  - `industry_s_curve`：产业生命周期与 S 曲线 (S-Curve)
  - `reverse_dcf`：反向 DCF (Reverse Discounted Cash Flow)
  - `dynamic_moat`：动态护城河分析 (Dynamic Moat)
  - `dcf`：DCF（自由现金流折现）
  - `porter_moat`：波特五力 + 护城河分析
  - `expectations_investing`：预期差框架 (Expectations Investing)
  - `relative_valuation`：相对估值（P/E、P/S、EV/EBITDA）
- 选择输入：
  - `framework_pool`（请求可选；未传使用默认池）
  - `evidence_pack`（4.1）
  - `symbol_context`（含 position/allocation/shares）
  - `preference_context`（risk/horizon/advice/strategy）
- 评分公式（0-100）：
  - `score = 40*coverage + 25*priority_fit + 20*portfolio_relevance + 15*preference_fit`
  - `coverage`：该框架所需证据覆盖率（0-1）
  - `priority_fit`：该框架命中高优先级证据的比例（P0/P1 权重更高）
  - `portfolio_relevance`：与当前仓位偏离、资产类型相关度
  - `preference_fit`：与风险偏好/期限/策略一致度
- 选择规则：
  1. 先过滤：`coverage < 0.45` 的框架不参与排序。
  2. 若 `P0+P1 coverage >= 0.5`，财报敏感框架至少入选 1 个：`dupont_roic`、`reverse_dcf`、`dcf`、`relative_valuation`。
  3. 若产业/政策证据覆盖高（`P2+P3 >= 0.5`），`capital_cycle` 或 `industry_s_curve` 至少入选 1 个。
  4. 若竞争格局与经营动态证据高（`P3+P4 >= 0.5`），`dynamic_moat` 或 `porter_moat` 至少入选 1 个。
  5. 其余按 `score` 降序补满 3 个。
  6. 同分按稳定顺序打破平局：`dupont_roic > reverse_dcf > dcf > relative_valuation > capital_cycle > industry_s_curve > dynamic_moat > porter_moat > expectations_investing`。
- 可解释输出：
  - 返回 `selected_frameworks`（含 `framework_id`、`score`、`rank`、`selected_reason`、`rejected_reason`）。
- 可测试性：
  - 单测使用固定证据夹具，验证“同输入必同输出”。
  - 边界测试：覆盖不足、约束入选规则生效、平局顺序稳定、候选池大小=3/9。

### 4.3 每框架输出与顶部综合建议组织
- 每框架输出（新字段 `framework_outputs[]`）：
  - `framework_id`
  - `thesis`（一句话核心判断）
  - `action_bias`（increase/hold/reduce）
  - `bias_score`（-100..100）
  - `probability_percent`（1..99）
  - `confidence`（high/medium/low）
  - `key_evidence[]`（证据引用，含类型与时间）
  - `risk_points[]`
  - `counter_points[]`（反方论据）
  - `summary`
- 顶部综合建议（复用 `synthesis`，新增权重拆解）：
  - `target_action`
  - `action_probability_percent`
  - `overall_rating`
  - `overall_summary`
  - `position_suggestion`
  - `weight_breakdown`：
    - `framework_consensus_weight = 0.60`
    - `portfolio_constraints_weight = 0.40`
  - `portfolio_constraints_detail`：
    - 持仓数量权重 `0.15`
    - 仓位占比权重 `0.30`
    - 资产配置区间权重 `0.30`
    - 用户偏好策略权重 `0.25`

### 4.4 综合建议加权模型（纳入持仓数量/仓位/区间/偏好）
- 框架共识分（按动作分别计算）：
  - `FrameworkScore(action) = Σ(framework_weight_i * action_score_i)`，其中 `i` 仅包含被选中的 3 个框架。
  - `framework_weight_i = framework_score_i * confidence_factor_i`
- 组合约束分：
  - `PortfolioScore(action) = 0.15*Q + 0.30*P + 0.30*A + 0.25*U`
  - `Q`（持仓数量）：仓位手数/持仓规模对动作的约束强度
  - `P`（仓位占比）：当前占比越偏离目标，增减仓倾向越强
  - `A`（配置区间）：`below_target` 倾向增仓，`above_target` 倾向减仓
  - `U`（用户偏好策略）：风险偏好、期限、策略提示对动作的修正
- 最终动作分：
  - `FinalScore(action) = 0.60*FrameworkScore(action) + 0.40*PortfolioScore(action)`
  - 取分值最大动作为 `target_action`，并通过归一化映射 `action_probability_percent`。

### 4.5 冲突结论权衡逻辑
- 冲突检测：
  - `disagreement_count`：与多数动作相反的框架数量
  - `score_gap = top1_score - top2_score`
- 权衡规则：
  1. 若 `disagreement_count >= 2` 且 `score_gap < 8`，默认 `hold`，概率上限 58%。
  2. 若仓位偏离区间绝对值 `> 5%`，组合约束优先，可覆盖框架微弱多数。
  3. 若财报证据（P0/P1）与其余框架冲突，优先财报证据结论。
  4. 冲突解释必须落地到 `conflict_resolution` 字段，说明“谁压过谁”。

### 4.6 输出硬约束与自动修正
- 硬约束清单：
  1. 第一行必须为结论句：`结论：<动作>，执行概率<整数>%`。
  2. `action_probability_percent` 必须是 `1..99` 的数字，不得区间或模糊表述。
  3. 禁止含混词：`看情况/视情况/it depends/可能吧/再观察`。
  4. `disclaimer` 长度 <= 16 字。
- 校验函数（新增建议）：
  - `validateSynthesisHardConstraints(result *SymbolSynthesisResult) error`
  - `normalizeSynthesisHardConstraints(result *SymbolSynthesisResult)`
- 兜底修正规则：
  - 概率缺失：按置信映射（high=72, medium=58, low=42）。
  - 首句不合规：强制重写首句。
  - `disclaimer` 过长：截断或改为 `“仅供参考，谨慎决策”`。

### 4.7 接口、存储与兼容
- 接口新增（可选）：
  - 请求：`framework_pool[]`、`framework_selection_mode`（默认 `auto_top3`）
  - 响应：`selected_frameworks[]`、`framework_outputs[]`、`conflict_resolution`
- 保持旧字段：
  - `dimensions` 与 `synthesis` 继续返回。
  - 未入选旧维度补 `not_selected` 占位摘要，保证旧前端读取不崩。
- 存储策略：
  - 复用 `symbol_analyses.synthesis` JSON 持久化新字段（首版不强制 schema 迁移）。
  - 如需查询优化，二期再加 `selected_frameworks_json`。

### 4.8 降级策略
- D1：P0/P1 财报不足时，允许继续分析，但必须降低置信并输出覆盖不足说明。
- D2：有效框架 < 3 时，按固定兜底顺序补齐：`dupont_roic -> dcf -> relative_valuation -> capital_cycle -> porter_moat`。
- D3：若最终成功框架 < 2，直接失败并返回可诊断错误。
- D4：综合层 JSON 解析失败时，走本地最小可用综合模板（含明确动作与概率）。

## 5. 兼容性与迁移
- 向后兼容性：
  - 旧请求不传 `framework_pool` 时，行为与当前接近（默认池自动选 3）。
  - 旧响应消费者继续读 `dimensions/synthesis` 不受破坏。
- 数据迁移计划：
  - 首版零迁移，写入 JSON 扩展字段。
  - 若后续新增列，采用 `tableHasColumn + ALTER TABLE` 增量迁移。
- 发布/回滚策略：
  - 增加特性开关：`ai_symbol_multi_framework_enabled`。
  - 灰度顺序：内部账号 -> 10% -> 50% -> 全量。
  - 回滚：关闭开关立即回到固定四维旧流程。

## 6. 风险与权衡
- 风险 R1：框架评分参数不合理，导致选框架偏移。
  - 规避：离线样本回放 + A/B 对比，参数外置可调。
- 风险 R2：高优先级证据不足时输出质量波动。
  - 规避：覆盖不足显式披露 + 概率上限约束 + 低置信降级。
- 风险 R3：模型输出违反硬约束。
  - 规避：后置校验器强制修正，失败则本地模板兜底。
- 风险 R4：多框架并行增加时延与 token 成本。
  - 规避：严格只跑 3 框架，证据文本限长与分段裁剪。
- 备选方案与取舍：
  - 备选：始终固定 3 框架（不做动态选择）。
  - 取舍：实现简单但解释性差，放弃。

## 7. 实施计划
- 任务拆分：
  1. 证据层：补齐优先级类型、覆盖率、质量分。
  2. 选择器：实现 `selectTopFrameworks` 与测试。
  3. 执行器：将固定 `dimensionAgents` 改为可选 3 框架并行。
  4. 综合层：实现权重模型与冲突权衡。
  5. 约束层：硬约束校验与修正器。
  6. 兼容层：旧字段映射与占位。
- 里程碑：
  - M1：选择器 + 单测通过。
  - M2：端到端返回 3 框架 + 综合建议。
  - M3：硬约束全通过 + 灰度准备。
- 影响范围：
  - 主要：`ai_symbol_analysis.go`、`ai_symbol_data_fetcher.go`
  - 次要：API handler 响应结构、前端展示字段（如启用新字段）

## 8. 验证计划
- 与 QA Spec 的映射：
  - Q1：输入优先级是否按 P0->P4 注入。
  - Q2：给定固定输入时，框架 Top-3 是否稳定可复现。
  - Q3：综合建议是否显式包含持仓数量/仓位/区间/偏好权重。
  - Q4：冲突样例是否触发既定权衡规则。
  - Q5：首句结论/概率/禁词/免责声明长度是否全部通过。
  - Q6：关闭开关是否回退到旧流程。
- 关键验证点：
  - 单测：`selectTopFrameworks`、`resolveConflict`、`validateSynthesisHardConstraints`。
  - 集成：模拟 4 框架池，校验最终只执行 3 个且解释字段完整。
  - 回归：旧接口消费端不改代码可正常读取结果。
