function renderInteractivePieChart({ items, totalLabel, totalValue, currency, pieId }) {
  const data = buildPieData(items);
  if (!data) {
    return '<div class="section-sub">No data</div>';
  }

  const size = 160;
  const cx = size / 2;
  const cy = size / 2;
  const outerRadius = 70;
  const innerRadius = 45;

  const paths = data.segments.map((seg) => {
    const startAngle = (seg.start / 100) * 360 - 90;
    const endAngle = (seg.end / 100) * 360 - 90;
    const largeArc = seg.end - seg.start > 50 ? 1 : 0;

    const startRad = (startAngle * Math.PI) / 180;
    const endRad = (endAngle * Math.PI) / 180;

    const x1 = cx + outerRadius * Math.cos(startRad);
    const y1 = cy + outerRadius * Math.sin(startRad);
    const x2 = cx + outerRadius * Math.cos(endRad);
    const y2 = cy + outerRadius * Math.sin(endRad);
    const x3 = cx + innerRadius * Math.cos(endRad);
    const y3 = cy + innerRadius * Math.sin(endRad);
    const x4 = cx + innerRadius * Math.cos(startRad);
    const y4 = cy + innerRadius * Math.sin(startRad);

    const d = `M ${x1} ${y1} A ${outerRadius} ${outerRadius} 0 ${largeArc} 1 ${x2} ${y2} L ${x3} ${y3} A ${innerRadius} ${innerRadius} 0 ${largeArc} 0 ${x4} ${y4} Z`;

    const segKey = seg.key || seg.label || '';
    const pnlAttr = seg.pnl !== null && seg.pnl !== undefined ? seg.pnl : '';
    const costAttr = seg.cost !== null && seg.cost !== undefined ? seg.cost : '';
    return `<path d="${d}" fill="${seg.color}"
                  data-pie-id="${pieId}"
                  data-symbol-key="${escapeHtml(segKey)}"
                  data-label="${escapeHtml(seg.label)}"
                  data-value="${seg.value || 0}"
                  data-percent="${seg.percent || 0}"
                  data-pnl="${pnlAttr}"
                  data-cost="${costAttr}"
                  class="pie-sector"/>`;
  }).join('');

  const centerValue = totalValue !== undefined && totalValue !== null
    ? formatValue(totalValue, currency)
    : '';

  return `
    <div class="pie-svg-container">
      <svg class="pie-svg" viewBox="0 0 ${size} ${size}" data-pie-id="${pieId}">
        ${paths}
      </svg>
      <div class="pie-center-label">
        ${totalLabel ? `<div class="pie-label">${escapeHtml(totalLabel)}</div>` : ''}
        ${centerValue ? `<div class="pie-value" data-sensitive>${centerValue}</div>` : ''}
      </div>
      <div class="pie-tooltip" data-pie-id="${pieId}" style="display:none;"></div>
    </div>
  `;
}

/**
 * 渲染账户分组列表
 */
function renderAccountGroupList(accountGroups, currency, pieId) {
  if (!accountGroups || !accountGroups.length) {
    return '<div class="section-sub">No holdings</div>';
  }

  return accountGroups.map((group, index) => {
    const rows = group.symbols.map((s) => {
      const hasPnl = s.pnl !== null && s.pnl !== undefined;
      const pnlClass = hasPnl ? (s.pnl >= 0 ? 'positive' : 'negative') : '';
      const pnlDisplay = hasPnl ? formatMoneyPlain(s.pnl) : '—';
      return `
        <div class="symbol-row" data-pie-id="${pieId}" data-symbol-key="${escapeHtml(s.pieKey)}">
          <div class="symbol-info">
            <span class="symbol-name">${escapeHtml(s.displayName)}</span>
            <span class="symbol-code">${escapeHtml(s.symbol)}</span>
          </div>
          <div class="symbol-value num" data-sensitive>${formatMoneyPlain(s.marketValue)}</div>
          <div class="symbol-percent num">${formatPercent(s.percent)}</div>
          <div class="symbol-pnl num ${pnlClass}" data-sensitive>${pnlDisplay}</div>
        </div>
      `;
    }).join('');

    return `
      <div class="account-group ${index % 2 === 1 ? 'alt' : ''}">
        <div class="account-header">
          <span class="account-name">${escapeHtml(group.accountName)}</span>
          <span class="account-total" data-sensitive>${formatMoneyPlain(group.totalMarketValue)}</span>
          <span class="account-percent">${formatPercent(group.percent)}</span>
        </div>
        <div class="account-symbols">${rows}</div>
      </div>
    `;
  }).join('');
}

/**
 * 高亮指定扇区
 */
function highlightPieSector(pieId, symbolKey) {
  // 清除所有高亮
  document.querySelectorAll('.pie-sector.highlighted').forEach((el) => {
    el.classList.remove('highlighted');
  });
  document.querySelectorAll('.pie-svg').forEach((svg) => {
    svg.classList.remove('has-highlight');
  });
  document.querySelectorAll('.symbol-row.highlighted').forEach((el) => {
    el.classList.remove('highlighted');
  });

  if (!symbolKey) return;

  // 高亮目标扇区
  const sector = document.querySelector(
    `.pie-sector[data-pie-id="${pieId}"][data-symbol-key="${symbolKey}"]`
  );
  if (sector) {
    sector.classList.add('highlighted');
    sector.closest('.pie-svg').classList.add('has-highlight');

    // 显示tooltip
    const tooltip = document.querySelector(`.pie-tooltip[data-pie-id="${pieId}"]`);
    if (tooltip) {
      const label = sector.dataset.label || '';
      const value = Number(sector.dataset.value || 0);
      const percent = Number(sector.dataset.percent || 0);
      const pnlRaw = sector.dataset.pnl;
      const costRaw = sector.dataset.cost;
      const hasPnl = pnlRaw !== '' && pnlRaw !== undefined;
      const pnl = hasPnl ? Number(pnlRaw) : null;
      const cost = costRaw !== '' && costRaw !== undefined ? Number(costRaw) : null;
      const pnlPercent = hasPnl && cost !== null && cost > 0 ? (pnl / cost) * 100 : null;
      const pnlClass = pnl !== null && pnl >= 0 ? 'positive' : 'negative';

      const pnlRows = hasPnl ? `
        <div class="tooltip-row">
          <span>盈亏</span>
          <span class="${pnlClass}" data-sensitive>${formatMoneyPlain(pnl)}</span>
        </div>
        ${pnlPercent !== null ? `
        <div class="tooltip-row">
          <span>盈亏率</span>
          <span class="${pnlClass}">${formatPercent(pnlPercent)}</span>
        </div>` : ''}
      ` : '';

      tooltip.innerHTML = `
        <div class="tooltip-name">${escapeHtml(label)}</div>
        <div class="tooltip-row">
          <span>占比</span>
          <span>${formatPercent(percent)}</span>
        </div>
        <div class="tooltip-row">
          <span>市值</span>
          <span data-sensitive>${formatMoneyPlain(value)}</span>
        </div>
        ${pnlRows}
      `;
      tooltip.style.display = 'block';
    }
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
 * 绑定Charts页面交互事件
 */
function bindChartsInteractions() {
  // 环图扇区：鼠标悬停高亮并显示tooltip
  view.querySelectorAll('.pie-sector[data-symbol-key]').forEach((sector) => {
    sector.addEventListener('mouseenter', () => {
      const pieId = sector.dataset.pieId;
      const symbolKey = sector.dataset.symbolKey;
      highlightPieSector(pieId, symbolKey);
    });
    sector.addEventListener('mouseleave', () => {
      const pieId = sector.dataset.pieId;
      highlightPieSector(pieId, null);
      const tooltip = document.querySelector(`.pie-tooltip[data-pie-id="${pieId}"]`);
      if (tooltip) tooltip.style.display = 'none';
    });
  });

  // 标的行点击
  view.querySelectorAll('.symbol-row[data-symbol-key]').forEach((row) => {
    row.addEventListener('click', () => {
      const pieId = row.dataset.pieId;
      const symbolKey = row.dataset.symbolKey;
      const isHighlighted = row.classList.contains('highlighted');

      if (isHighlighted) {
        highlightPieSector(pieId, null);
        const tooltip = document.querySelector(`.pie-tooltip[data-pie-id="${pieId}"]`);
        if (tooltip) tooltip.style.display = 'none';
      } else {
        highlightPieSector(pieId, symbolKey);
      }
    });
  });

  // 点击空白处取消高亮
  view.querySelectorAll('.currency-block').forEach((block) => {
    block.addEventListener('click', (e) => {
      if (e.target.closest('.symbol-row') || e.target.closest('.pie-sector')) return;
      const pieId = block.dataset.pieId;
      highlightPieSector(pieId, null);
      const tooltip = document.querySelector(`.pie-tooltip[data-pie-id="${pieId}"]`);
      if (tooltip) tooltip.style.display = 'none';
    });
  });
}

async function renderCharts() {
  view.innerHTML = `
    <div class="section-title">Charts</div>
    <div class="section-sub">Symbol composition snapshots by currency.</div>
    <div class="card">Loading charts...</div>
  `;

  try {
    const bySymbol = await fetchJSON('/api/holdings-by-symbol');
    const currencies = Object.keys(bySymbol || {});

    if (!currencies.length) {
      view.innerHTML = renderEmptyState('No holdings data yet. Add your first transaction.', '<a class="primary" href="#/add">Add transaction</a>');
      return;
    }

    const currencyBlocks = currencies.map((currency) => {
      const data = bySymbol[currency] || {};
      // 过滤零持仓标的，与 Holdings 页面保持一致（Holdings 也过滤 total_shares <= 0）
      const symbols = (data.symbols || []).filter((s) => (s.total_shares || 0) > 0);
      const totalMarketValue = symbols.reduce((sum, s) => sum + (s.market_value || 0), 0);

      const pieId = `pie-${currency}`;

      // 构建环图数据项，添加 key 用于高亮关联
      // 使用 _symbol/_accountId 精确匹配，避免同名标的跨账户时 key 错乱
      const pieItems = buildSymbolPieItems(symbols, 8).map((item) => ({
        ...item,
        key: item._symbol ? `${item._symbol}-${item._accountId || 'unknown'}` : item.label,
      }));

      // 渲染 SVG 环图
      const pieChart = renderInteractivePieChart({
        items: pieItems,
        totalLabel: 'Total',
        totalValue: totalMarketValue,
        currency,
        pieId,
      });

      // 按账户分组
      const accountGroups = groupSymbolsByAccount(symbols, totalMarketValue);
      const groupList = renderAccountGroupList(accountGroups, currency, pieId);

      return `
        <div class="currency-block card" data-pie-id="${pieId}">
          <h3>${currency}</h3>
          <div class="chart-content">
            ${pieChart}
            <div class="account-groups-list">
              ${groupList}
            </div>
          </div>
        </div>
      `;
    }).join('');

    view.innerHTML = `
      <div class="section-title">Charts</div>
      <div class="section-sub">Asset allocation by currency and account.</div>
      <div class="charts-horizontal">
        ${currencyBlocks}
      </div>
    `;

    bindChartsInteractions();
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load charts. Check API connection.');
  }
}

