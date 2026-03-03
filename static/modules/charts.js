function buildPieData(items) {
  const cleaned = (items || []).map((item) => {
    const value = Number(item && item.value) || 0;
    return { ...item, value };
  }).filter((item) => item && item.value > 0);
  const total = cleaned.reduce((sum, item) => sum + item.value, 0);
  if (!total) {
    return null;
  }
  let offset = 0;
  const segments = cleaned.map((item, index) => {
    const share = (item.value / total) * 100;
    const percent = index === cleaned.length - 1 ? Math.max(0, 100 - offset) : share;
    const start = offset;
    const end = start + percent;
    offset = end;
    return {
      ...item,
      percent,
      start,
      end,
      color: chartPalette[index % chartPalette.length],
    };
  });
  const gradient = segments.map((seg) => `${seg.color} ${seg.start}% ${seg.end}%`).join(', ');
  return { total, segments, gradient };
}

function renderPieChart({ items, totalLabel, totalValue, currency }) {
  const data = buildPieData(items);
  if (!data) {
    return '<div class="section-sub">No data</div>';
  }
  const centerValue = totalValue !== undefined && totalValue !== null
    ? formatValue(totalValue, currency)
    : '';
  const centerMarkup = (totalLabel || centerValue) ? `
    <div class="pie-center">
      ${totalLabel ? `<div class="pie-label">${escapeHtml(totalLabel)}</div>` : ''}
      ${centerValue ? `<div class="pie-value" data-sensitive>${centerValue}</div>` : ''}
    </div>
  ` : '';

  const legend = data.segments.map((seg) => {
    const amountMarkup = seg.amount !== undefined && seg.amount !== null
      ? `<span data-sensitive>${formatValue(seg.amount, currency)}</span>`
      : '';
    return `
      <div class="legend-item">
        <span class="legend-swatch" style="background:${seg.color};"></span>
        <div class="legend-label">${escapeHtml(seg.label)}</div>
        <div class="legend-meta">
          <span>${formatPercent(seg.percent)}</span>
          ${amountMarkup}
        </div>
      </div>
    `;
  }).join('');

  return `
    <div class="pie-layout">
      <div class="pie-chart" style="background: conic-gradient(${data.gradient});">
        ${centerMarkup}
      </div>
      <div class="pie-legend">${legend}</div>
    </div>
  `;
}

function renderAccountPieChart({ items }) {
  const data = buildPieData(items);
  if (!data) {
    return '<div class="section-sub">No account data</div>';
  }
  const legend = data.segments.map((seg) => `
    <div class="legend-item">
      <span class="legend-swatch" style="background:${seg.color};"></span>
      <div class="legend-label">
        <span>${escapeHtml(seg.label)}</span>
        <span class="legend-percent" style="color:${seg.color};">${formatPercent(seg.percent)}</span>
      </div>
    </div>
  `).join('');

  return `
    <div class="pie-layout account-pie">
      <div class="pie-chart" style="background: conic-gradient(${data.gradient});"></div>
      <div class="pie-legend account-legend">${legend}</div>
    </div>
  `;
}

function buildSymbolPieItems(symbols, limit = 8) {
  const filtered = (symbols || []).filter((s) => (s.market_value || 0) > 0);
  if (!filtered.length) {
    return [];
  }
  const sorted = [...filtered].sort((a, b) => b.market_value - a.market_value);
  const primary = sorted.slice(0, limit);
  const rest = sorted.slice(limit);
  const items = primary.map((s) => ({
    label: s.display_name || s.symbol,
    value: s.market_value,
    amount: s.market_value,
    pnl: s.unrealized_pnl ?? null,
    cost: (s.avg_cost || 0) * (s.total_shares || 0),
    _symbol: s.symbol,
    _accountId: s.account_id,
  }));
  if (rest.length) {
    const otherValue = rest.reduce((sum, s) => sum + s.market_value, 0);
    if (otherValue > 0) {
      items.push({
        label: 'Other',
        value: otherValue,
        amount: otherValue,
        pnl: null,
        cost: null,
      });
    }
  }
  return items;
}

