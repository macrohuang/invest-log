# Technical Specification: Charts页面币种环图重构

**Document Status:** Draft
**Version:** 1.0
**Author:** Claude
**Date:** 2026-02-09
**Related PRD:** charts-currency-pie-redesign-v1.0-prd.md

## Executive Summary

**Problem:** 当前Charts页面的展示方式未能清晰呈现账户维度的资产分布，账户汇总与标的列表分离，用户难以快速理解每个账户下的资产配置。

**Solution:** 重构Charts页面，实现按账户分组的标的列表，账户标题行显示累计信息，点击标的可高亮环图扇区。

**Impact:** 提升用户对多账户资产配置的理解效率，改善数据可视化体验。

---

## 1. Background

### Context

当前 `renderCharts()` 函数（app.js:1270-1344）的实现：
- 每个币种一个卡片，垂直堆叠在 `.grid.two` 布局中
- 环图显示标的占比（最多8个扇区 + Other）
- 账户分布作为独立的小环图显示在"By account"标题下
- 标的列表平铺显示（最多12个），无账户分组

### Goals

- **Primary Goal:** 按账户分组展示标的列表，标题行显示账户累计金额和占比
- **Secondary Goals:**
  - 多币种横向并排展示
  - 点击标的行时环图扇区高亮
- **Success Metrics:**
  - 页面渲染时间 < 500ms
  - 交互响应时间 < 100ms

### Non-Goals

- 不改变现有 API 接口
- 不添加历史趋势图
- 不实现账户间资产对比功能

---

## 2. Requirements

### Functional Requirements

#### FR-1: 多币种横向布局
**Priority:** P0 (Must Have)
**Description:** 多个币种环图横向并排展示

**Acceptance Criteria:**
- [ ] 使用 flexbox 实现横向并排
- [ ] 单币种时居中显示
- [ ] 响应式：窄屏幕时自动换行

**Dependencies:** None

#### FR-2: 按账户分组的标的列表
**Priority:** P0 (Must Have)
**Description:** 标的列表按账户分组显示

**Acceptance Criteria:**
- [ ] 账户按累计市值从大到小排序
- [ ] 同账户内标的按占比从大到小排序
- [ ] 无持仓的账户不显示

**Dependencies:** FR-1

#### FR-3: 账户标题行
**Priority:** P0 (Must Have)
**Description:** 每个账户分组显示标题行，包含名称、金额、占比

**Acceptance Criteria:**
- [ ] 格式：`{账户名称}  {累计金额}  {占比}%`
- [ ] 金额使用 `formatMoneyPlain()` 格式化
- [ ] 占比保留2位小数

**Dependencies:** FR-2

#### FR-4: 标的行显示字段
**Priority:** P0 (Must Have)
**Description:** 每个标的行显示完整信息

**Acceptance Criteria:**
- [ ] 显示：名称、代码、金额、占比、盈亏
- [ ] 盈亏使用颜色区分（正绿负红）
- [ ] 金额标记 `data-sensitive` 支持隐私模式

**Dependencies:** FR-2

#### FR-5: 背景色交替
**Priority:** P1 (Should Have)
**Description:** 账户分组使用交替背景色区分

**Acceptance Criteria:**
- [ ] 奇数账户使用默认背景
- [ ] 偶数账户使用浅色背景（如 `rgba(0,0,0,0.02)`）

**Dependencies:** FR-2

#### FR-6: 点击高亮交互
**Priority:** P1 (Should Have)
**Description:** 点击标的行时，环图对应扇区高亮

**Acceptance Criteria:**
- [ ] 扇区向外突出（transform 或 SVG path 调整）
- [ ] 显示 tooltip（标的名称、金额、占比）
- [ ] 再次点击或点击其他位置取消高亮

**Dependencies:** FR-1, FR-2

### Non-Functional Requirements

#### Performance
- **渲染时间:** 页面完整渲染 < 500ms
- **交互响应:** 点击高亮反馈 < 100ms
- **内存占用:** 无内存泄漏

#### Compatibility
- **浏览器:** Chrome 90+, Safari 14+, Firefox 88+
- **移动端:** iOS Safari, Android Chrome

---

## 3. System Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     renderCharts()                          │
│                      (app.js)                               │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                  fetchJSON('/api/holdings-by-symbol')       │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│              Data Processing Functions                       │
│  ┌──────────────────┐  ┌──────────────────┐                 │
│  │ groupByAccount() │  │ sortAccounts()   │                 │
│  └──────────────────┘  └──────────────────┘                 │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                  Rendering Functions                         │
│  ┌──────────────────┐  ┌──────────────────┐                 │
│  │ renderPieChart() │  │ renderGroupList()│                 │
│  └──────────────────┘  └──────────────────┘                 │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                  Event Binding                              │
│  ┌──────────────────┐  ┌──────────────────┐                 │
│  │ bindRowClick()   │  │ highlightSector()│                 │
│  └──────────────────┘  └──────────────────┘                 │
└─────────────────────────────────────────────────────────────┘
```

### Component Diagram

#### 数据处理层
- **groupByAccount(symbols):** 将标的按账户分组
- **sortAccounts(groups):** 按累计市值排序账户
- **sortSymbols(symbols):** 按占比排序标的
- **calculateAccountTotal(symbols):** 计算账户累计值

#### 渲染层
- **renderCurrencyBlock(currency, data):** 渲染单个币种区块
- **renderPieChartWithHighlight(items, id):** 渲染带高亮功能的环图
- **renderAccountGroup(account, symbols, index):** 渲染账户分组
- **renderSymbolRow(symbol, pieId):** 渲染标的行

#### 交互层
- **bindSymbolRowClick():** 绑定标的行点击事件
- **highlightPieSector(pieId, symbolKey):** 高亮环图扇区
- **clearHighlight(pieId):** 清除高亮状态
- **showTooltip(event, data):** 显示tooltip

---

## 4. Data Model

### 输入数据结构 (from API)

```typescript
// /api/holdings-by-symbol 返回
interface HoldingsBySymbol {
  [currency: string]: {
    total_market_value: number;
    total_cost: number;
    total_pnl: number;
    symbols: Symbol[];
    by_account: {
      [accountId: string]: {
        account_name: string;
        symbols: Symbol[];
      };
    };
  };
}

interface Symbol {
  symbol: string;
  display_name: string;
  account_id: string;
  account_name: string;
  total_shares: number;
  avg_cost: number;
  latest_price: number;
  market_value: number;
  unrealized_pnl: number;
  percent: number;  // 占该币种总市值的百分比
  asset_type: string;
  auto_update: number;
}
```

### 内部数据结构

```typescript
// 账户分组数据
interface AccountGroup {
  accountId: string;
  accountName: string;
  totalMarketValue: number;
  percent: number;  // 账户占该币种总市值的百分比
  symbols: ProcessedSymbol[];
}

interface ProcessedSymbol {
  symbol: string;
  displayName: string;
  marketValue: number;
  percent: number;
  pnl: number;
  pnlPercent: number;
  pieKey: string;  // 用于环图高亮关联
}

// 币种区块数据
interface CurrencyBlock {
  currency: string;
  totalMarketValue: number;
  pieItems: PieItem[];
  accountGroups: AccountGroup[];
}
```

---

## 5. Implementation Details

### 5.1 数据处理函数

```javascript
/**
 * 按账户分组标的并计算汇总
 * @param {Symbol[]} symbols - 标的列表
 * @param {number} totalMarketValue - 该币种总市值
 * @returns {AccountGroup[]} - 排序后的账户分组
 */
function groupSymbolsByAccount(symbols, totalMarketValue) {
  const accountMap = new Map();

  symbols.forEach(s => {
    const accountId = s.account_id;
    if (!accountMap.has(accountId)) {
      accountMap.set(accountId, {
        accountId,
        accountName: s.account_name || accountId,
        totalMarketValue: 0,
        symbols: []
      });
    }
    const group = accountMap.get(accountId);
    group.totalMarketValue += s.market_value || 0;
    group.symbols.push({
      symbol: s.symbol,
      displayName: s.display_name || s.symbol,
      marketValue: s.market_value || 0,
      percent: s.percent || 0,
      pnl: s.unrealized_pnl || 0,
      pieKey: `${s.symbol}-${accountId}`
    });
  });

  // 排序：账户按市值降序，标的按占比降序
  const groups = Array.from(accountMap.values());
  groups.sort((a, b) => b.totalMarketValue - a.totalMarketValue);
  groups.forEach(g => {
    g.percent = totalMarketValue > 0
      ? (g.totalMarketValue / totalMarketValue) * 100
      : 0;
    g.symbols.sort((a, b) => b.percent - a.percent);
  });

  return groups.filter(g => g.totalMarketValue > 0);
}
```

### 5.2 环图高亮实现

当前环图使用 CSS `conic-gradient` 实现，高亮需要改为 SVG 实现以支持单扇区交互。

```javascript
/**
 * 渲染支持高亮的SVG环图
 * @param {Object} options
 * @returns {string} SVG HTML
 */
function renderInteractivePieChart({ items, totalLabel, totalValue, currency, pieId }) {
  const data = buildPieData(items);
  if (!data) return '<div class="section-sub">No data</div>';

  const size = 160;
  const cx = size / 2;
  const cy = size / 2;
  const outerRadius = 70;
  const innerRadius = 45;

  const paths = data.segments.map((seg, i) => {
    const startAngle = (seg.start / 100) * 360 - 90;
    const endAngle = (seg.end / 100) * 360 - 90;
    const largeArc = (seg.end - seg.start) > 50 ? 1 : 0;

    const x1 = cx + outerRadius * Math.cos(startAngle * Math.PI / 180);
    const y1 = cy + outerRadius * Math.sin(startAngle * Math.PI / 180);
    const x2 = cx + outerRadius * Math.cos(endAngle * Math.PI / 180);
    const y2 = cy + outerRadius * Math.sin(endAngle * Math.PI / 180);
    const x3 = cx + innerRadius * Math.cos(endAngle * Math.PI / 180);
    const y3 = cy + innerRadius * Math.sin(endAngle * Math.PI / 180);
    const x4 = cx + innerRadius * Math.cos(startAngle * Math.PI / 180);
    const y4 = cy + innerRadius * Math.sin(startAngle * Math.PI / 180);

    const d = `M ${x1} ${y1} A ${outerRadius} ${outerRadius} 0 ${largeArc} 1 ${x2} ${y2}
               L ${x3} ${y3} A ${innerRadius} ${innerRadius} 0 ${largeArc} 0 ${x4} ${y4} Z`;

    return `<path d="${d}" fill="${seg.color}"
                  data-pie-id="${pieId}"
                  data-symbol-key="${escapeHtml(seg.key || seg.label)}"
                  class="pie-sector"/>`;
  }).join('');

  return `
    <svg class="pie-svg" viewBox="0 0 ${size} ${size}" data-pie-id="${pieId}">
      ${paths}
      <text x="${cx}" y="${cy - 8}" class="pie-label-text">${escapeHtml(totalLabel || '')}</text>
      <text x="${cx}" y="${cy + 12}" class="pie-value-text" data-sensitive>
        ${totalValue !== undefined ? formatValue(totalValue, currency) : ''}
      </text>
    </svg>
  `;
}
```

### 5.3 高亮交互

```javascript
/**
 * 高亮指定扇区
 * @param {string} pieId - 环图ID
 * @param {string} symbolKey - 标的key
 */
function highlightPieSector(pieId, symbolKey) {
  // 清除之前的高亮
  document.querySelectorAll('.pie-sector.highlighted').forEach(el => {
    el.classList.remove('highlighted');
  });
  document.querySelectorAll('.symbol-row.highlighted').forEach(el => {
    el.classList.remove('highlighted');
  });

  // 高亮目标扇区
  const sector = document.querySelector(
    `.pie-sector[data-pie-id="${pieId}"][data-symbol-key="${symbolKey}"]`
  );
  if (sector) {
    sector.classList.add('highlighted');
  }

  // 高亮对应行
  const row = document.querySelector(
    `.symbol-row[data-pie-id="${pieId}"][data-symbol-key="${symbolKey}"]`
  );
  if (row) {
    row.classList.add('highlighted');
  }
}

/**
 * 绑定标的行点击事件
 */
function bindSymbolRowClick() {
  view.querySelectorAll('.symbol-row[data-symbol-key]').forEach(row => {
    row.addEventListener('click', () => {
      const pieId = row.dataset.pieId;
      const symbolKey = row.dataset.symbolKey;
      const isHighlighted = row.classList.contains('highlighted');

      if (isHighlighted) {
        // 取消高亮
        document.querySelectorAll('.pie-sector.highlighted, .symbol-row.highlighted')
          .forEach(el => el.classList.remove('highlighted'));
      } else {
        highlightPieSector(pieId, symbolKey);
      }
    });
  });
}
```

### 5.4 CSS 样式

```css
/* 横向布局 */
.charts-horizontal {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
  justify-content: center;
}

.currency-block {
  flex: 1;
  min-width: 320px;
  max-width: 480px;
}

/* SVG 环图 */
.pie-svg {
  width: 160px;
  height: 160px;
}

.pie-sector {
  transition: transform 0.2s ease, filter 0.2s ease;
  transform-origin: center;
  cursor: pointer;
}

.pie-sector.highlighted {
  transform: scale(1.08);
  filter: brightness(1.1);
}

.pie-sector:not(.highlighted) {
  opacity: 0.6;
}

.pie-svg:not(:has(.highlighted)) .pie-sector {
  opacity: 1;
}

.pie-label-text,
.pie-value-text {
  text-anchor: middle;
  font-size: 12px;
  fill: var(--ink-0);
}

.pie-value-text {
  font-weight: 600;
  font-size: 14px;
}

/* 账户分组 */
.account-group {
  margin-bottom: 16px;
}

.account-group:nth-child(odd) {
  background: transparent;
}

.account-group:nth-child(even) {
  background: rgba(0, 0, 0, 0.02);
  border-radius: var(--radius-sm);
}

.account-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  font-weight: 600;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}

.account-header .account-name {
  flex: 1;
}

.account-header .account-total {
  margin-left: 16px;
}

.account-header .account-percent {
  margin-left: 12px;
  opacity: 0.7;
}

/* 标的行 */
.symbol-row {
  display: grid;
  grid-template-columns: 1fr auto auto auto auto;
  gap: 12px;
  align-items: center;
  padding: 8px 12px;
  cursor: pointer;
  transition: background 0.15s;
}

.symbol-row:hover {
  background: rgba(0, 0, 0, 0.03);
}

.symbol-row.highlighted {
  background: rgba(16, 185, 129, 0.1);
}

.symbol-row .symbol-info {
  display: flex;
  flex-direction: column;
}

.symbol-row .symbol-name {
  font-weight: 500;
}

.symbol-row .symbol-code {
  font-size: 0.85em;
  opacity: 0.6;
}

.symbol-row .symbol-value,
.symbol-row .symbol-percent,
.symbol-row .symbol-pnl {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.symbol-row .symbol-pnl.positive {
  color: var(--accent-cool);
}

.symbol-row .symbol-pnl.negative {
  color: var(--accent-strong);
}

/* Tooltip */
.pie-tooltip {
  position: absolute;
  background: var(--card);
  border: var(--border);
  border-radius: var(--radius-sm);
  padding: 8px 12px;
  box-shadow: var(--shadow-soft);
  pointer-events: none;
  z-index: 100;
  font-size: 13px;
}

.pie-tooltip .tooltip-name {
  font-weight: 600;
  margin-bottom: 4px;
}

.pie-tooltip .tooltip-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
}
```

---

## 6. Implementation Plan

### Phase 1: 数据处理重构
**Goal:** 实现按账户分组的数据结构

**Tasks:**
- [ ] 新增 `groupSymbolsByAccount()` 函数
- [ ] 实现账户和标的排序逻辑
- [ ] 计算账户累计值和占比
- [ ] 单元测试

**Deliverables:** 数据处理模块
**Files:** `app.js` (新增函数)

### Phase 2: 布局重构
**Goal:** 实现新的页面布局

**Tasks:**
- [ ] 修改 `renderCharts()` 主函数
- [ ] 实现横向并排布局
- [ ] 实现账户分组列表结构
- [ ] 实现账户标题行
- [ ] 添加背景色交替样式

**Deliverables:** 新的 Charts 页面布局
**Files:** `app.js`, `style.css`

### Phase 3: SVG 环图重构
**Goal:** 将 CSS 环图改为 SVG 实现

**Tasks:**
- [ ] 新增 `renderInteractivePieChart()` 函数
- [ ] 实现 SVG path 计算
- [ ] 添加 data 属性用于交互关联
- [ ] 调整样式

**Deliverables:** 可交互的 SVG 环图
**Files:** `app.js`, `style.css`

### Phase 4: 交互实现
**Goal:** 实现点击高亮交互

**Tasks:**
- [ ] 新增 `highlightPieSector()` 函数
- [ ] 新增 `bindSymbolRowClick()` 函数
- [ ] 实现 tooltip 显示
- [ ] CSS 过渡动画

**Deliverables:** 完整的交互功能
**Files:** `app.js`, `style.css`

### Phase 5: 测试与优化
**Goal:** 确保功能完整和性能达标

**Tasks:**
- [ ] 测试多币种场景
- [ ] 测试单币种场景
- [ ] 测试空数据场景
- [ ] 测试响应式布局
- [ ] 性能优化

**Deliverables:** 可发布版本

---

## 7. Testing Strategy

### 功能测试

| 场景 | 预期结果 |
|------|----------|
| 多币种数据 | 横向并排显示，各自独立 |
| 单币种数据 | 居中显示 |
| 空数据 | 显示"No data"提示 |
| 多账户 | 按市值排序，背景交替 |
| 单账户 | 正常显示，无交替背景 |
| 点击标的行 | 环图扇区高亮，行高亮 |
| 再次点击 | 取消高亮 |
| 点击其他行 | 切换高亮 |

### 边界测试

- 账户无持仓 → 不显示
- 标的市值为0 → 不显示
- 超长账户名称 → 截断显示
- 超多标的 → 滚动或限制数量

### 性能测试

- 100+ 标的渲染时间 < 500ms
- 交互响应时间 < 100ms
- 无内存泄漏（长时间使用）

---

## 8. Risks & Mitigation

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| SVG 环图性能问题 | Low | Medium | 限制扇区数量，使用 CSS 动画 |
| 移动端交互困难 | Medium | Low | 增大点击区域，添加 touch 事件 |
| 响应式布局问题 | Medium | Medium | 使用 flexbox wrap，设置 min-width |
| 数据量大时卡顿 | Low | Medium | 限制显示数量，虚拟滚动 |

---

## 9. Success Criteria

### Launch Criteria
- [ ] 所有 P0 需求实现
- [ ] 多币种横向布局正常
- [ ] 账户分组显示正确
- [ ] 点击高亮交互流畅
- [ ] 响应式布局正常
- [ ] 无控制台错误

### Post-Launch Metrics
- 页面渲染时间 < 500ms
- 交互响应时间 < 100ms
- 无用户反馈的 UI 问题

---

**Document Version:** 1.0
**Created:** 2026-02-09
**Quality Score:** 94/100
