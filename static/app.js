const state = {
  apiBase: '',
  privacy: false,
  aiAnalysisByCurrency: {},
};

const aiAnalysisSettingsKey = 'aiHoldingsAnalysisSettings';

const view = document.getElementById('view');
const toastEl = document.getElementById('toast');
const connectionPill = document.getElementById('connection-pill');
const privacyToggle = document.getElementById('privacy-toggle');
const navLinks = Array.from(document.querySelectorAll('.nav a'));

const currencySymbols = {
  CNY: '¥',
  USD: '$',
  HKD: 'HK$'
};

const chartPalette = [
  '#f06c3b',
  '#1aa6b7',
  '#f2a93b',
  '#6b7f66',
  '#c44b22',
  '#5b8fb9',
  '#b76a58',
  '#7a9a4e',
  '#8a6bd4',
  '#3f463e'
];

function init() {
  state.apiBase = resolveApiBase();
  state.privacy = localStorage.getItem('privacyMode') === '1';
  document.body.classList.toggle('privacy', state.privacy);

  privacyToggle.addEventListener('click', () => {
    state.privacy = !state.privacy;
    document.body.classList.toggle('privacy', state.privacy);
    localStorage.setItem('privacyMode', state.privacy ? '1' : '0');
  });

  window.addEventListener('hashchange', renderRoute);
  renderRoute();
  updateConnectionStatus();
  registerServiceWorker();
}

function resolveApiBase() {
  const params = new URLSearchParams(window.location.search);
  const paramBase = params.get('api');
  if (paramBase) {
    localStorage.setItem('apiBase', paramBase);
    return trimTrailingSlash(paramBase);
  }
  const stored = localStorage.getItem('apiBase');
  if (stored) {
    return trimTrailingSlash(stored);
  }
  if (window.location.protocol === 'http:' || window.location.protocol === 'https:') {
    return '';
  }
  return '';
}

function trimTrailingSlash(value) {
  return value.replace(/\/+$/, '');
}

function apiUrl(path) {
  if (!state.apiBase) {
    return path;
  }
  return `${state.apiBase}${path}`;
}

async function fetchJSON(path, options = {}) {
  if (!state.apiBase && window.location.protocol === 'file:') {
    throw new Error('API base not set');
  }
  const url = apiUrl(path);
  const config = {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  };
  const response = await fetch(url, config);
  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Request failed: ${response.status}`);
  }
  if (response.status === 204) {
    return null;
  }
  return response.json();
}

function setActiveRoute(routeKey) {
  navLinks.forEach((link) => {
    const isActive = link.dataset.route === routeKey;
    link.classList.toggle('active', isActive);
  });
}

function getRouteQuery() {
  const hash = window.location.hash || '';
  const queryIndex = hash.indexOf('?');
  if (queryIndex === -1) {
    return new URLSearchParams();
  }
  return new URLSearchParams(hash.slice(queryIndex + 1));
}

function loadAIAnalysisSettings() {
  try {
    const raw = localStorage.getItem(aiAnalysisSettingsKey);
    if (!raw) {
      return {
        baseUrl: 'https://api.openai.com/v1',
        model: '',
        apiKey: '',
        riskProfile: 'balanced',
        horizon: 'medium',
        adviceStyle: 'balanced',
        allowNewSymbols: true,
      };
    }
    const parsed = JSON.parse(raw);
    return {
      baseUrl: parsed.baseUrl || 'https://api.openai.com/v1',
      model: parsed.model || '',
      apiKey: parsed.apiKey || '',
      riskProfile: parsed.riskProfile || 'balanced',
      horizon: parsed.horizon || 'medium',
      adviceStyle: parsed.adviceStyle || 'balanced',
      allowNewSymbols: parsed.allowNewSymbols !== false,
    };
  } catch (err) {
    return {
      baseUrl: 'https://api.openai.com/v1',
      model: '',
      apiKey: '',
      riskProfile: 'balanced',
      horizon: 'medium',
      adviceStyle: 'balanced',
      allowNewSymbols: true,
    };
  }
}

function saveAIAnalysisSettings(settings) {
  localStorage.setItem(aiAnalysisSettingsKey, JSON.stringify(settings));
}

function formatActionLabel(action) {
  const normalized = String(action || '').toLowerCase();
  if (normalized === 'increase') return 'Increase';
  if (normalized === 'reduce') return 'Reduce';
  if (normalized === 'add') return 'Add';
  return 'Hold';
}

function renderAIAnalysisCard(result, currency) {
  if (!result) {
    return '';
  }
  const findings = Array.isArray(result.key_findings) ? result.key_findings : [];
  const recommendations = Array.isArray(result.recommendations) ? result.recommendations : [];
  const findingsMarkup = findings.length
    ? `<ul class="ai-findings">${findings.map((item) => `<li>${escapeHtml(item)}</li>`).join('')}</ul>`
    : '<div class="section-sub">No key findings.</div>';
  const recommendationMarkup = recommendations.length
    ? `<div class="ai-recommendations">${recommendations.map((item) => {
      const symbol = item.symbol ? `<strong>${escapeHtml(item.symbol)}</strong>` : '<strong>Portfolio</strong>';
      const action = formatActionLabel(item.action);
      const theory = escapeHtml(item.theory_tag || 'N/A');
      const rationale = escapeHtml(item.rationale || 'No rationale');
      const targetWeight = item.target_weight ? `<span class="section-sub">Target: ${escapeHtml(item.target_weight)}</span>` : '';
      const priority = item.priority ? `<span class="section-sub">Priority: ${escapeHtml(item.priority)}</span>` : '';
      return `
        <div class="ai-rec-item">
          <div class="ai-rec-head">
            ${symbol}
            <span class="tag other">${escapeHtml(action)}</span>
            <span class="tag other">${theory}</span>
          </div>
          <div class="section-sub">${rationale}</div>
          <div class="ai-rec-meta">${targetWeight}${priority}</div>
        </div>
      `;
    }).join('')}</div>`
    : '<div class="section-sub">No recommendations returned.</div>';

  const generatedAt = result.generated_at ? escapeHtml(String(result.generated_at)) : '—';
  const model = result.model ? escapeHtml(String(result.model)) : '—';
  const riskLevel = result.risk_level ? escapeHtml(String(result.risk_level)) : 'unknown';
  const summary = result.overall_summary ? escapeHtml(String(result.overall_summary)) : '—';
  const disclaimer = result.disclaimer ? escapeHtml(String(result.disclaimer)) : 'For reference only.';

  return `
    <div class="card ai-analysis-card" data-ai-analysis-card="${currency}">
      <div class="ai-analysis-head">
        <h4>AI Analysis</h4>
        <div class="section-sub">Model: ${model} · Generated: ${generatedAt}</div>
      </div>
      <div class="ai-summary">
        <div><strong>Risk Level:</strong> ${riskLevel}</div>
        <div class="section-sub">${summary}</div>
      </div>
      <div class="ai-section">
        <h5>Key Findings</h5>
        ${findingsMarkup}
      </div>
      <div class="ai-section">
        <h5>Recommendations</h5>
        ${recommendationMarkup}
      </div>
      <div class="section-sub">${disclaimer}</div>
    </div>
  `;
}

function renderRoute() {
  const hash = window.location.hash || '#/overview';
  const route = hash.replace('#/', '').split('?')[0];
  switch (route) {
    case 'holdings':
      setActiveRoute('holdings');
      renderHoldings();
      break;
    case 'transactions':
      setActiveRoute('transactions');
      renderTransactions();
      break;
    case 'charts':
      setActiveRoute('charts');
      renderCharts();
      break;
    case 'add':
      setActiveRoute('transactions');
      renderAddTransaction();
      break;
    case 'settings':
      setActiveRoute('settings');
      renderSettings();
      break;
    default:
      setActiveRoute('overview');
      renderOverview();
  }
}

function showToast(message) {
  toastEl.textContent = message;
  toastEl.classList.add('show');
  setTimeout(() => toastEl.classList.remove('show'), 2600);
}

function updateConnectionStatus() {
  if (!state.apiBase && window.location.protocol === 'file:') {
    connectionPill.textContent = 'API base required';
    connectionPill.classList.remove('online');
    return;
  }
  fetch(apiUrl('/api/health'))
    .then((res) => {
      if (res.ok) {
        connectionPill.textContent = 'Connected';
        connectionPill.classList.add('online');
      } else {
        connectionPill.textContent = 'API error';
        connectionPill.classList.remove('online');
      }
    })
    .catch(() => {
      connectionPill.textContent = 'Offline';
      connectionPill.classList.remove('online');
    });
}

function formatMoney(value, currency) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  const symbol = currencySymbols[currency] || '';
  try {
    return new Intl.NumberFormat('en-US', {
      style: symbol ? 'currency' : 'decimal',
      currency: currency,
      maximumFractionDigits: 2,
    }).format(value);
  } catch (err) {
    return `${symbol}${value.toFixed(2)}`;
  }
}

function formatMoneyPlain(value) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'decimal',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(value);
  } catch (err) {
    return Number(value).toFixed(2);
  }
}

function formatNumber(value) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'decimal',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(value);
  } catch (err) {
    return Number(value).toFixed(2);
  }
}

function formatPercent(value) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  return `${Number(value).toFixed(2)}%`;
}

function escapeHtml(value) {
  return String(value || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function escapeRegExp(value) {
  return String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function highlightMatchText(text, keyword) {
  const source = String(text || '');
  const needle = String(keyword || '').trim();
  if (!needle) {
    return escapeHtml(source);
  }
  const regex = new RegExp(escapeRegExp(needle), 'ig');
  let lastIndex = 0;
  let output = '';
  let matched = false;
  let hit = regex.exec(source);
  while (hit) {
    matched = true;
    const start = hit.index;
    const end = start + hit[0].length;
    output += escapeHtml(source.slice(lastIndex, start));
    output += `<mark>${escapeHtml(source.slice(start, end))}</mark>`;
    lastIndex = end;
    hit = regex.exec(source);
  }
  if (!matched) {
    return escapeHtml(source);
  }
  output += escapeHtml(source.slice(lastIndex));
  return output;
}

function formatValue(value, currency) {
  if (currency) {
    return formatMoney(value, currency);
  }
  return formatNumber(value);
}

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
  }));
  if (rest.length) {
    const otherValue = rest.reduce((sum, s) => sum + s.market_value, 0);
    if (otherValue > 0) {
      items.push({
        label: 'Other',
        value: otherValue,
        amount: otherValue,
      });
    }
  }
  return items;
}

function renderEmptyState(message, action) {
  return `
    <div class="card">
      <h3>Nothing here yet</h3>
      <p class="section-sub">${escapeHtml(message)}</p>
      ${action || ''}
    </div>
  `;
}

async function renderOverview() {
  view.innerHTML = `
    <div class="section-title">Overview</div>
    <div class="overview-summary card">Loading overview...</div>
    <div class="section-sub">Market value, allocations, and warning bands.</div>
    <div class="grid three">
      <div class="card">Loading CNY allocation...</div>
      <div class="card">Loading USD allocation...</div>
      <div class="card">Loading HKD allocation...</div>
    </div>
  `;

  try {
    const [byCurrency, bySymbol, exchangeRates] = await Promise.all([
      fetchJSON('/api/holdings-by-currency'),
      fetchJSON('/api/holdings-by-symbol'),
      fetchJSON('/api/exchange-rates'),
    ]);

    const currencyOrder = ['CNY', 'USD', 'HKD'];
    const currencyList = currencyOrder.filter((curr) => (byCurrency || {})[curr]);
    const extraCurrencies = Object.keys(byCurrency || {})
      .filter((curr) => !currencyOrder.includes(curr))
      .sort();
    currencyList.push(...extraCurrencies);
    const rateMap = { CNY: 1 };
    (exchangeRates || []).forEach((item) => {
      if (item && item.to_currency === 'CNY' && Number(item.rate) > 0) {
        rateMap[item.from_currency] = Number(item.rate);
      }
    });

    let totalMarket = 0;
    let totalCost = 0;

    if (bySymbol) {
      Object.entries(bySymbol).forEach(([currency, entry]) => {
        const rate = Number(rateMap[currency] || 0);
        if (rate <= 0) {
          return;
        }
        totalMarket += Number(entry.total_market_value || 0) * rate;
        totalCost += Number(entry.total_cost || 0) * rate;
      });
    }

    const totalPnL = totalMarket - totalCost;
    const pnlClass = totalPnL >= 0 ? 'pill positive' : 'pill negative';
    const summaryCard = `
      <div class="overview-summary card">
        <div class="summary-title">Total Market Value (CNY)</div>
        <div class="summary-value" data-sensitive>${formatMoney(totalMarket, 'CNY')}</div>
        <div class="summary-sub">Converted by Settings exchange rates.</div>
        <div class="summary-pill ${pnlClass}" data-sensitive>${formatMoney(totalPnL, 'CNY')} total PnL</div>
      </div>
    `;

    const allocationCards = currencyList.map((currency) => {
      const data = byCurrency[currency] || { total: 0, allocations: [] };
      const allocationItems = (data.allocations || []).map((alloc) => ({
        label: alloc.label,
        value: alloc.amount || 0,
        amount: alloc.amount,
      }));
      const pieChart = renderPieChart({
        items: allocationItems,
        totalLabel: 'Total',
        totalValue: data.total || 0,
        currency,
      });
      const allocations = (data.allocations || []).map((alloc) => {
        const warning = alloc.warning ? `<div class="alert">${escapeHtml(alloc.warning)}</div>` : '';
        return `
          <div class="list-item">
            <div>
              <strong>${escapeHtml(alloc.label)}</strong>
              <div class="bar"><span style="width:${alloc.percent}%;"></span></div>
            </div>
            <div style="text-align:right;">
              <div>${formatPercent(alloc.percent)}</div>
              <div data-sensitive>${formatMoney(alloc.amount, currency)}</div>
              ${warning}
            </div>
          </div>
        `;
      }).join('');
      const listMarkup = allocations ? `<div class="list">${allocations}</div>` : '';

      return `
        <div class="card">
          <h3>${currency} Allocation</h3>
          ${pieChart}
          ${listMarkup}
        </div>
      `;
    }).join('');

    view.innerHTML = `
      <div class="section-title">Overview</div>
      ${summaryCard}
      <div class="section-sub">Market value, allocations, and warning bands.</div>
      <div class="grid three">${allocationCards}</div>
    `;
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load overview data. Check API connection.');
  }
}

async function renderHoldings() {
  view.innerHTML = `
    <div class="section-title">Holdings</div>
    <div class="section-sub">Latest positions by symbol and currency.</div>
    <div class="card">Loading holdings...</div>
  `;

  try {
    const data = await fetchJSON('/api/holdings-by-symbol');
    const currencies = Object.keys(data || {});
    if (!currencies.length) {
      view.innerHTML = renderEmptyState('No holdings yet. Add your first transaction.', '<a class="primary" href="#/add">Add transaction</a>');
      return;
    }

    const tabButtons = currencies.map((currency) => `
      <button class="tab-button" data-holdings-tab="${currency}" type="button">${currency}</button>
    `).join('');

    const panels = currencies.map((currency) => {
      const currencyData = data[currency] || {};
      const symbols = currencyData.symbols || [];
      const canUpdateAll = symbols.some((s) => s.auto_update !== 0);
      const aiResult = state.aiAnalysisByCurrency[currency] || null;
      const totalMarketValue = Number(currencyData.total_market_value ?? 0);
      const totalCost = Number(currencyData.total_cost ?? 0);
      const totalPnL = Number(currencyData.total_pnl ?? (totalMarketValue - totalCost));
      const totalPnLPercent = totalCost > 0 ? (totalPnL / totalCost) * 100 : null;
      const pnlClass = totalPnL >= 0 ? 'pnl-positive' : 'pnl-negative';
      const totalMarketLabel = formatMoney(totalMarketValue, currency);
      const totalPnLLabel = formatMoney(totalPnL, currency);
      const totalPnLPercentLabel = totalPnLPercent !== null ? `<span class="total-pnl-percent">${formatPercent(totalPnLPercent)}</span>` : '';
      const rows = symbols.map((s) => {
        const pnlClass = s.unrealized_pnl !== null && s.unrealized_pnl !== undefined ? (s.unrealized_pnl >= 0 ? 'pnl-positive' : 'pnl-negative') : '';
        const autoUpdate = s.auto_update !== 0;
        const updateDisabled = autoUpdate ? '' : 'disabled title="Auto sync off"';
        const symbolLink = `#/transactions?symbol=${encodeURIComponent(s.symbol || '')}&account=${encodeURIComponent(s.account_id || '')}`;
        const symbolCost = (s.avg_cost || 0) * (s.total_shares || 0);
        const pnlPercent = symbolCost > 0 && s.unrealized_pnl !== null ? (s.unrealized_pnl / symbolCost) * 100 : null;
        const pnlPercentLabel = pnlPercent !== null ? `<span class="pnl-percent">${formatPercent(pnlPercent)}</span>` : '';
        const pnlMarkup = s.unrealized_pnl !== null && s.unrealized_pnl !== undefined
          ? `<div class="pnl-cell"><span class="pnl-value ${pnlClass}" data-sensitive>${formatMoneyPlain(s.unrealized_pnl)}</span>${pnlPercentLabel}</div>`
          : '<span class="section-sub">—</span>';
        const accountSort = (s.account_name || s.account_id || '').toString();
        return `
          <tr data-account="${escapeHtml(accountSort.toLowerCase())}" data-market="${s.market_value || 0}" data-pnl="${s.unrealized_pnl || 0}">
            <td><strong>${escapeHtml(s.display_name)}</strong><br><a class="symbol-link section-sub" href="${symbolLink}">${escapeHtml(s.symbol)}</a></td>
            <td><strong>${escapeHtml(s.account_name || s.account_id || '')}</strong><br><span class="section-sub">${escapeHtml(s.account_id || '')}</span></td>
            <td class="num" data-sensitive>${formatNumber(s.total_shares)}</td>
            <td class="num" data-sensitive>${formatMoneyPlain(s.avg_cost)}</td>
            <td class="num" data-sensitive>${s.latest_price !== null ? formatMoneyPlain(s.latest_price) : '—'}</td>
            <td class="num" data-sensitive>${formatMoneyPlain(s.market_value)}</td>
            <td class="num">${pnlMarkup}</td>
            <td class="actions-column">
              <div class="actions holdings-actions">
                <select class="btn trade-select" data-symbol="${escapeHtml(s.symbol)}" data-currency="${currency}" data-account="${escapeHtml(s.account_id || '')}" data-asset-type="${escapeHtml(s.asset_type || '')}">
                  <option value="">Trade</option>
                  <option value="BUY">Buy</option>
                  <option value="SELL">Sell</option>
                  <option value="DIVIDEND">Dividend</option>
                  <option value="TRANSFER_IN">Transfer In</option>
                  <option value="TRANSFER_OUT">Transfer Out</option>
                </select>
                <button class="btn secondary" data-action="update" data-symbol="${escapeHtml(s.symbol)}" data-currency="${currency}" data-asset-type="${escapeHtml(s.asset_type || '')}" ${updateDisabled}>Update</button>
                <button class="btn tertiary" data-action="manual" data-symbol="${escapeHtml(s.symbol)}" data-currency="${currency}">Manual</button>
              </div>
            </td>
          </tr>
        `;
      }).join('');

      return `
        <div class="tab-panel" data-holdings-panel="${currency}">
          <div class="card holdings-card">
            <div class="panel-head">
              <div class="panel-left">
                <h3>${currency} Holdings</h3>
                <div class="panel-meta">
                  <div class="panel-metric">
                    <span class="section-sub">Market Value</span>
                    <span class="metric-value" data-sensitive>${totalMarketLabel}</span>
                  </div>
                  <div class="panel-metric">
                    <span class="section-sub">Total P&amp;L</span>
                    <div class="metric-value-group">
                      <span class="metric-value ${pnlClass}" data-sensitive>${totalPnLLabel}</span>
                      ${totalPnLPercentLabel}
                    </div>
                  </div>
                </div>
              </div>
              <div class="actions">
                <button class="btn secondary" data-action="update-all" data-currency="${currency}" ${canUpdateAll ? '' : 'disabled title="No auto-sync symbols"'}>Update all</button>
                <button class="btn tertiary" data-action="ai-analyze" data-currency="${currency}">AI Analyze</button>
              </div>
            </div>
            <table class="table" data-holdings-table>
              <thead>
                <tr>
                  <th>Symbol</th>
                  <th class="sortable" data-sort="account">Account</th>
                  <th class="num">Shares</th>
                  <th class="num">Avg Cost</th>
                  <th class="num">Price</th>
                  <th class="sortable num" data-sort="market">Market Value</th>
                  <th class="sortable num" data-sort="pnl">PnL</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>${rows}</tbody>
            </table>
            ${renderAIAnalysisCard(aiResult, currency)}
          </div>
        </div>
      `;
    }).join('');

    view.innerHTML = `
      <div class="section-title">Holdings</div>
      <div class="section-sub">Latest positions by symbol and currency.</div>
      <div class="tab-bar" role="tablist">${tabButtons}</div>
      ${panels}
    `;

    initHoldingsTabs();
    initHoldingsSort();
    bindHoldingsActions();
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load holdings. Check API connection.');
  }
}

function initHoldingsTabs() {
  const tabs = Array.from(view.querySelectorAll('[data-holdings-tab]'));
  const panels = Array.from(view.querySelectorAll('[data-holdings-panel]'));
  if (!tabs.length || !panels.length) {
    return;
  }
  const available = tabs.map((btn) => btn.dataset.holdingsTab);
  const saved = localStorage.getItem('activeHoldingsTab');
  const initial = available.includes(saved) ? saved : available[0];

  const setActive = (currency) => {
    tabs.forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.holdingsTab === currency);
    });
    panels.forEach((panel) => {
      panel.classList.toggle('active', panel.dataset.holdingsPanel === currency);
    });
    localStorage.setItem('activeHoldingsTab', currency);
  };

  tabs.forEach((btn) => {
    btn.addEventListener('click', () => {
      setActive(btn.dataset.holdingsTab);
    });
  });

  setActive(initial);
}

function initHoldingsSort() {
  const tables = Array.from(view.querySelectorAll('table[data-holdings-table]'));
  tables.forEach((table) => {
    const headers = Array.from(table.querySelectorAll('th[data-sort]'));
    if (!headers.length) return;
    headers.forEach((th) => {
      th.addEventListener('click', () => {
        const key = th.dataset.sort;
        const currentKey = table.dataset.sortKey;
        let dir = table.dataset.sortDir === 'asc' ? 'desc' : 'asc';
        if (currentKey !== key) {
          dir = key === 'account' ? 'asc' : 'desc';
        }
        table.dataset.sortKey = key;
        table.dataset.sortDir = dir;
        headers.forEach((header) => {
          header.classList.toggle('sorted', header.dataset.sort === key);
          header.classList.toggle('asc', header.dataset.sort === key && dir === 'asc');
          header.classList.toggle('desc', header.dataset.sort === key && dir === 'desc');
        });

        const tbody = table.tBodies[0];
        const rows = Array.from(tbody.rows);
        const multiplier = dir === 'asc' ? 1 : -1;
        rows.sort((a, b) => {
          if (key === 'account') {
            const aVal = (a.dataset.account || '').toString();
            const bVal = (b.dataset.account || '').toString();
            return aVal.localeCompare(bVal) * multiplier;
          }
          const aNum = Number(a.dataset[key] || 0);
          const bNum = Number(b.dataset[key] || 0);
          return (aNum - bNum) * multiplier;
        });
        rows.forEach((row) => tbody.appendChild(row));
      });
    });
  });
}

function bindHoldingsActions() {
  // Trade dropdown navigation
  view.querySelectorAll('select.trade-select').forEach((sel) => {
    sel.addEventListener('change', () => {
      const type = sel.value;
      if (!type) return;
      const symbol = sel.dataset.symbol;
      const currency = sel.dataset.currency;
      const account = sel.dataset.account;
      const assetType = sel.dataset.assetType;
      const params = new URLSearchParams();
      params.set('type', type);
      params.set('symbol', symbol);
      params.set('currency', currency);
      if (account) params.set('account', account);
      if (assetType) params.set('asset_type', assetType);
      window.location.hash = `#/add?${params.toString()}`;
    });
  });

  view.querySelectorAll('button[data-action]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      if (btn.disabled) {
        return;
      }
      const action = btn.dataset.action;
      const symbol = btn.dataset.symbol;
      const currency = btn.dataset.currency;
      const assetType = btn.dataset.assetType || '';
      try {
        if (action === 'update-all') {
          await fetchJSON('/api/prices/update-all', {
            method: 'POST',
            body: JSON.stringify({ currency }),
          });
          showToast(`${currency} prices updated`);
        }
        if (action === 'update') {
          await fetchJSON('/api/prices/update', {
            method: 'POST',
            body: JSON.stringify({ symbol, currency, asset_type: assetType }),
          });
          showToast(`${symbol} updated`);
        }
        if (action === 'manual') {
          const value = window.prompt(`Manual price for ${symbol} (${currency})`);
          if (!value) return;
          const price = Number(value);
          if (Number.isNaN(price)) {
            showToast('Invalid price');
            return;
          }
          await fetchJSON('/api/prices/manual', {
            method: 'POST',
            body: JSON.stringify({ symbol, currency, price }),
          });
          showToast(`${symbol} saved`);
        }
        if (action === 'ai-analyze') {
          btn.disabled = true;
          btn.textContent = 'Analyzing...';
          try {
            const analyzed = await runAIHoldingsAnalysis(currency);
            if (analyzed) {
              showToast(`${currency} analysis ready`);
            }
          } finally {
            btn.disabled = false;
            btn.textContent = 'AI Analyze';
          }
        }
        renderHoldings();
      } catch (err) {
        if (action === 'ai-analyze') {
          let message = 'AI analysis failed';
          if (err && err.message) {
            const raw = String(err.message);
            try {
              const parsed = JSON.parse(raw);
              if (parsed && parsed.error) {
                message = String(parsed.error);
              }
            } catch (parseErr) {
              message = raw;
            }
            const firstLine = message.split('\n')[0];
            const trimmed = firstLine.length > 140 ? `${firstLine.slice(0, 137)}...` : firstLine;
            message = trimmed;
          }
          showToast(message);
        } else {
          showToast('Price update failed');
        }
      }
    });
  });
}

async function runAIHoldingsAnalysis(currency) {
  const settings = loadAIAnalysisSettings();

  const normalizedSettings = {
    baseUrl: (settings.baseUrl || 'https://api.openai.com/v1').trim(),
    model: (settings.model || '').trim(),
    apiKey: (settings.apiKey || '').trim(),
    riskProfile: settings.riskProfile || 'balanced',
    horizon: settings.horizon || 'medium',
    adviceStyle: settings.adviceStyle || 'balanced',
    allowNewSymbols: settings.allowNewSymbols !== false,
  };

  if (!normalizedSettings.model || !normalizedSettings.apiKey) {
    localStorage.setItem('activeSettingsTab', 'api');
    window.location.hash = '#/settings';
    showToast('Set AI model and API Key in Settings > API');
    return false;
  }

  saveAIAnalysisSettings(normalizedSettings);

  const result = await fetchJSON('/api/ai/holdings-analysis', {
    method: 'POST',
    body: JSON.stringify({
      base_url: normalizedSettings.baseUrl,
      api_key: normalizedSettings.apiKey,
      model: normalizedSettings.model,
      currency,
      risk_profile: normalizedSettings.riskProfile,
      horizon: normalizedSettings.horizon,
      advice_style: normalizedSettings.adviceStyle,
      allow_new_symbols: normalizedSettings.allowNewSymbols,
    }),
  });

  state.aiAnalysisByCurrency[currency] = result;
  return true;
}

async function renderTransactions() {
  view.innerHTML = `
    <div class="section-title">Transactions</div>
    <div class="section-sub">Filter by account, symbol, or date range.</div>
    <div class="card">Loading transactions...</div>
  `;

  try {
    const query = getRouteQuery();
    const rawSymbol = (query.get('symbol') || '').trim();
    const filterSymbol = rawSymbol.split(' - ')[0].trim();
    const filterAccount = (query.get('account') || '').trim();
    const filterStartDate = (query.get('start_date') || '').trim();
    const filterEndDate = (query.get('end_date') || '').trim();
    const pageSize = 100;
    const pageRaw = Number.parseInt(query.get('page') || '1', 10);
    const page = Number.isNaN(pageRaw) || pageRaw < 1 ? 1 : pageRaw;
    const offset = (page - 1) * pageSize;
    const txParams = new URLSearchParams({
      limit: String(pageSize),
      offset: String(offset),
      paged: '1'
    });
    if (filterSymbol) {
      txParams.set('symbol', filterSymbol);
    }
    if (filterAccount) {
      txParams.set('account_id', filterAccount);
    }
    if (filterStartDate) {
      txParams.set('start_date', filterStartDate);
    }
    if (filterEndDate) {
      txParams.set('end_date', filterEndDate);
    }
    const [transactionsResponse, accounts] = await Promise.all([
      fetchJSON(`/api/transactions?${txParams.toString()}`),
      fetchJSON('/api/accounts')
    ]);
    const isLegacyTransactions = Array.isArray(transactionsResponse);
    const transactions = isLegacyTransactions
      ? transactionsResponse
      : (transactionsResponse.items || []);
    const totalCount = isLegacyTransactions
      ? transactions.length
      : Number(transactionsResponse.total ?? transactions.length);
    const limitUsed = isLegacyTransactions
      ? pageSize
      : Number(transactionsResponse.limit ?? pageSize);
    const offsetUsed = isLegacyTransactions
      ? offset
      : Number(transactionsResponse.offset ?? offset);
    const accountNameMap = new Map((accounts || []).map((a) => [a.account_id, a.account_name || a.account_id]));
    const symbolKey = filterSymbol ? filterSymbol.toUpperCase() : '';
    const filteredTransactions = isLegacyTransactions
      ? (transactions || []).filter((t) => {
          if (symbolKey && String(t.symbol || '').toUpperCase() !== symbolKey) {
            return false;
          }
          if (filterAccount && String(t.account_id || '') !== filterAccount) {
            return false;
          }
          const txnDate = String(t.transaction_date || '');
          if (filterStartDate && txnDate < filterStartDate) {
            return false;
          }
          if (filterEndDate && txnDate > filterEndDate) {
            return false;
          }
          return true;
        })
      : (transactions || []);
    const effectiveTotal = isLegacyTransactions ? filteredTransactions.length : totalCount;
    const currentPage = limitUsed > 0 ? Math.floor(offsetUsed / limitUsed) + 1 : 1;
    const totalPages = limitUsed > 0 ? Math.max(1, Math.ceil(effectiveTotal / limitUsed)) : 1;
    const symbolMeta = new Map();
    (transactions || []).forEach((t) => {
      const symbol = String(t.symbol || '').toUpperCase();
      if (!symbol) return;
      const entry = symbolMeta.get(symbol) || { name: '', accounts: new Set() };
      if (!entry.name && t.name) {
        entry.name = String(t.name);
      }
      const accountLabel = t.account_name || accountNameMap.get(t.account_id) || t.account_id || '';
      if (accountLabel) {
        entry.accounts.add(String(accountLabel));
      }
      symbolMeta.set(symbol, entry);
    });
    let symbolOptions = Array.from(symbolMeta.entries())
      .sort((a, b) => a[0].localeCompare(b[0]))
      .map(([symbol, meta]) => {
        let label = symbol;
        if (meta.name) {
          label += ` - ${meta.name}`;
        }
        const accounts = Array.from(meta.accounts);
        if (accounts.length) {
          label += ` · ${accounts.join(', ')}`;
        }
        const selected = symbol === symbolKey ? ' selected' : '';
        return `<option value="${escapeHtml(symbol)}"${selected}>${escapeHtml(label)}</option>`;
      }).join('');
    if (filterSymbol && !symbolMeta.has(symbolKey)) {
      symbolOptions = `<option value="${escapeHtml(filterSymbol)}" selected>${escapeHtml(filterSymbol)}</option>${symbolOptions}`;
    }
    const accountIds = new Set((accounts || []).map((a) => String(a.account_id || '')));
    let accountOptions = (accounts || []).map((a) => {
      const accountId = String(a.account_id || '');
      const label = a.account_name && a.account_name !== accountId
        ? `${a.account_name} (${accountId})`
        : accountId;
      const selected = accountId === filterAccount ? ' selected' : '';
      return `<option value="${escapeHtml(accountId)}"${selected}>${escapeHtml(label)}</option>`;
    }).join('');
    if (filterAccount && !accountIds.has(filterAccount)) {
      accountOptions = `<option value="${escapeHtml(filterAccount)}" selected>${escapeHtml(filterAccount)}</option>${accountOptions}`;
    }
    const rows = filteredTransactions.map((t) => {
      const tagClass = t.transaction_type === 'BUY' ? 'buy' : t.transaction_type === 'SELL' ? 'sell' : 'other';
      const displayName = t.name ? t.name : t.symbol;
      const showSymbolSub = Boolean(t.name);
      const resolvedAccountName = t.account_name || accountNameMap.get(t.account_id) || t.account_id || '';
      const showAccountSub = Boolean(t.account_name || accountNameMap.get(t.account_id));
      return `
        <tr>
          <td>${escapeHtml(t.transaction_date)}</td>
          <td><strong>${escapeHtml(displayName)}</strong>${showSymbolSub ? `<br><span class="section-sub">${escapeHtml(t.symbol)}</span>` : ''}</td>
          <td><span class="tag ${tagClass}">${escapeHtml(t.transaction_type)}</span></td>
          <td class="num" data-sensitive>${formatNumber(t.quantity)}</td>
          <td class="num" data-sensitive>${formatMoneyPlain(t.price)}</td>
          <td class="num" data-sensitive>${formatMoneyPlain(t.total_amount)}</td>
          <td><strong>${escapeHtml(resolvedAccountName)}</strong>${showAccountSub ? `<br><span class="section-sub">${escapeHtml(t.account_id || '')}</span>` : ''}</td>
          <td>
            <button class="btn danger" data-action="delete" data-id="${t.id}">Delete</button>
          </td>
        </tr>
      `;
    }).join('');
    const filterTags = [];
    if (filterAccount) {
      const accountLabel = accountNameMap.get(filterAccount) || filterAccount;
      filterTags.push(`<span class="tag other">Account: ${escapeHtml(accountLabel)}</span>`);
    }
    if (filterSymbol) {
      const meta = symbolMeta.get(symbolKey);
      const symbolLabel = meta && meta.name ? `${filterSymbol} - ${meta.name}` : filterSymbol;
      filterTags.push(`<span class="tag other">Symbol: ${escapeHtml(symbolLabel)}</span>`);
    }
    if (filterStartDate || filterEndDate) {
      let dateLabel = 'Date: ';
      if (filterStartDate && filterEndDate) {
        dateLabel += `${escapeHtml(filterStartDate)} to ${escapeHtml(filterEndDate)}`;
      } else if (filterStartDate) {
        dateLabel += `from ${escapeHtml(filterStartDate)}`;
      } else {
        dateLabel += `until ${escapeHtml(filterEndDate)}`;
      }
      filterTags.push(`<span class="tag other">${dateLabel}</span>`);
    }
    const filterBar = filterTags.length
      ? `
        <div class="filter-bar">
          ${filterTags.join('')}
          <a class="inline-link" href="#/transactions">Clear</a>
        </div>
      `
      : '<div class="section-sub">Filter by account, symbol, or date range.</div>';

    const pagination = `
      <div class="card">
        <div class="pagination-bar">
          <button class="btn secondary" data-action="page-prev" ${currentPage <= 1 ? 'disabled' : ''}>Prev</button>
          <div class="section-sub">Page ${currentPage} / ${totalPages} · Total ${effectiveTotal}</div>
          <button class="btn secondary" data-action="page-next" ${currentPage >= totalPages ? 'disabled' : ''}>Next</button>
        </div>
      </div>
    `;

    view.innerHTML = `
      <div class="section-title">Transactions</div>
      ${filterBar}
      <div class="card">
        <form id="tx-filter" class="form">
          <div class="form-row">
            <div class="field">
              <label for="filter-account">Account</label>
              <select id="filter-account" name="account">
                <option value="">All</option>
                ${accountOptions}
              </select>
            </div>
            <div class="field">
              <label for="filter-symbol">Symbol</label>
              <select id="filter-symbol" name="symbol">
                <option value="">All</option>
                ${symbolOptions}
              </select>
            </div>
            <div class="field">
              <label for="filter-start">Start date</label>
              <input id="filter-start" name="start_date" type="date" value="${escapeHtml(filterStartDate)}">
            </div>
            <div class="field">
              <label for="filter-end">End date</label>
              <input id="filter-end" name="end_date" type="date" value="${escapeHtml(filterEndDate)}">
            </div>
          </div>
          <div class="actions">
            <button class="btn" type="submit">Apply</button>
            <button class="btn secondary" type="button" data-action="clear-filters">Clear</button>
          </div>
        </form>
      </div>
      <div class="card">
        <table class="table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Symbol</th>
              <th>Type</th>
              <th class="num">Qty</th>
              <th class="num">Price</th>
              <th class="num">Total</th>
              <th>Account</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>${rows || '<tr><td colspan="8">No transactions found.</td></tr>'}</tbody>
        </table>
      </div>
      ${totalPages > 1 ? pagination : ''}
    `;

    const filterForm = view.querySelector('#tx-filter');
    if (filterForm) {
      filterForm.addEventListener('submit', (event) => {
        event.preventDefault();
        const formData = new FormData(filterForm);
        const symbolRaw = (formData.get('symbol') || '').toString().trim();
        const symbol = symbolRaw.split(' - ')[0].trim();
        const account = (formData.get('account') || '').toString().trim();
        const startDate = (formData.get('start_date') || '').toString().trim();
        const endDate = (formData.get('end_date') || '').toString().trim();
        if (startDate && endDate && startDate > endDate) {
          showToast('Start date must be before end date');
          return;
        }
        const params = new URLSearchParams();
        if (symbol) params.set('symbol', symbol);
        if (account) params.set('account', account);
        if (startDate) params.set('start_date', startDate);
        if (endDate) params.set('end_date', endDate);
        const queryString = params.toString();
        window.location.hash = queryString ? `#/transactions?${queryString}` : '#/transactions';
      });
    }

    const clearButton = view.querySelector('[data-action="clear-filters"]');
    if (clearButton) {
      clearButton.addEventListener('click', () => {
        window.location.hash = '#/transactions';
      });
    }

    const buildPageQuery = (nextPage) => {
      const params = new URLSearchParams();
      if (filterSymbol) params.set('symbol', filterSymbol);
      if (filterAccount) params.set('account', filterAccount);
      if (filterStartDate) params.set('start_date', filterStartDate);
      if (filterEndDate) params.set('end_date', filterEndDate);
      if (nextPage > 1) params.set('page', String(nextPage));
      const queryString = params.toString();
      window.location.hash = queryString ? `#/transactions?${queryString}` : '#/transactions';
    };

    const prevButton = view.querySelector('[data-action="page-prev"]');
    if (prevButton) {
      prevButton.addEventListener('click', () => {
        if (currentPage <= 1) return;
        buildPageQuery(currentPage - 1);
      });
    }

    const nextButton = view.querySelector('[data-action="page-next"]');
    if (nextButton) {
      nextButton.addEventListener('click', () => {
        if (currentPage >= totalPages) return;
        buildPageQuery(currentPage + 1);
      });
    }

    view.querySelectorAll('button[data-action="delete"]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        const id = btn.dataset.id;
        if (!confirm('Delete this transaction?')) return;
        try {
          await fetchJSON(`/api/transactions/${id}`, { method: 'DELETE' });
          showToast('Deleted');
          renderTransactions();
        } catch (err) {
          showToast('Delete failed');
        }
      });
    });
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load transactions.');
  }
}

/**
 * 按账户分组标的并计算汇总
 * @param {Array} symbols - 标的列表
 * @param {number} totalMarketValue - 该币种总市值
 * @returns {Array} - 排序后的账户分组
 */
function groupSymbolsByAccount(symbols, totalMarketValue) {
  const accountMap = new Map();

  (symbols || []).forEach((s) => {
    const accountId = s.account_id || 'unknown';
    if (!accountMap.has(accountId)) {
      accountMap.set(accountId, {
        accountId,
        accountName: s.account_name || accountId,
        totalMarketValue: 0,
        symbols: [],
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
      pieKey: `${s.symbol}-${accountId}`,
    });
  });

  // 排序：账户按市值降序，标的按占比降序
  const groups = Array.from(accountMap.values());
  groups.sort((a, b) => b.totalMarketValue - a.totalMarketValue);
  groups.forEach((g) => {
    g.percent = totalMarketValue > 0 ? (g.totalMarketValue / totalMarketValue) * 100 : 0;
    g.symbols.sort((a, b) => b.percent - a.percent);
  });

  return groups.filter((g) => g.totalMarketValue > 0);
}

/**
 * 渲染支持高亮的SVG环图
 */
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
    return `<path d="${d}" fill="${seg.color}"
                  data-pie-id="${pieId}"
                  data-symbol-key="${escapeHtml(segKey)}"
                  data-label="${escapeHtml(seg.label)}"
                  data-value="${seg.value || 0}"
                  data-percent="${seg.percent || 0}"
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
      const pnlClass = s.pnl >= 0 ? 'positive' : 'negative';
      return `
        <div class="symbol-row" data-pie-id="${pieId}" data-symbol-key="${escapeHtml(s.pieKey)}">
          <div class="symbol-info">
            <span class="symbol-name">${escapeHtml(s.displayName)}</span>
            <span class="symbol-code">${escapeHtml(s.symbol)}</span>
          </div>
          <div class="symbol-value num" data-sensitive>${formatMoneyPlain(s.marketValue)}</div>
          <div class="symbol-percent num">${formatPercent(s.percent)}</div>
          <div class="symbol-pnl num ${pnlClass}" data-sensitive>${formatMoneyPlain(s.pnl)}</div>
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
      tooltip.innerHTML = `
        <div class="tooltip-name">${escapeHtml(label)}</div>
        <div class="tooltip-row">
          <span>Value</span>
          <span data-sensitive>${formatMoneyPlain(value)}</span>
        </div>
        <div class="tooltip-row">
          <span>Percent</span>
          <span>${formatPercent(percent)}</span>
        </div>
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
      const symbols = data.symbols || [];
      const totalMarketValue = data.total_market_value !== undefined && data.total_market_value !== null
        ? data.total_market_value
        : symbols.reduce((sum, s) => sum + (s.market_value || 0), 0);

      const pieId = `pie-${currency}`;

      // 构建环图数据项，添加 key 用于高亮关联
      const pieItems = buildSymbolPieItems(symbols, 8).map((item) => {
        const matchingSymbol = symbols.find((s) => s.display_name === item.label || s.symbol === item.label);
        return {
          ...item,
          key: matchingSymbol ? `${matchingSymbol.symbol}-${matchingSymbol.account_id}` : item.label,
        };
      });

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

async function renderAddTransaction() {
  view.innerHTML = `
    <div class="section-title">New Transaction</div>
    <div class="section-sub">Record a buy, sell, or cash movement.</div>
    <div class="card">Loading form...</div>
  `;

  try {
    const [accounts, assetTypes, holdingsBySymbol] = await Promise.all([
      fetchJSON('/api/accounts'),
      fetchJSON('/api/asset-types'),
      fetchJSON('/api/holdings-by-symbol')
    ]);

    if (!accounts.length) {
      view.innerHTML = renderEmptyState('Create an account first in Settings.', '<a class="primary" href="#/settings">Open Settings</a>');
      return;
    }

    const accountMap = new Map(accounts.map((a) => [a.account_id, a.account_name || a.account_id]));
    const assetLabelMap = new Map(assetTypes.map((a) => [String(a.code).toLowerCase(), a.label]));
    const holdings = [];
    Object.entries(holdingsBySymbol || {}).forEach(([currency, data]) => {
      (data.symbols || []).forEach((h) => {
        holdings.push({
          currency,
          symbol: h.symbol,
          displayName: h.display_name || h.symbol,
          accountId: h.account_id,
          accountName: h.account_name || accountMap.get(h.account_id) || h.account_id,
          assetType: (h.asset_type || '').toLowerCase(),
          totalShares: h.total_shares || 0,
        });
      });
    });
    const today = new Date().toISOString().slice(0, 10);

    view.innerHTML = `
      <div class="section-title">New Transaction</div>
      <div class="section-sub">Record a buy, sell, or cash movement.</div>
      <div class="card">
        <form id="tx-form" class="form">
          <div class="form-row">
            <div class="field">
              <label>Date</label>
              <input type="date" name="transaction_date" value="${today}" required>
            </div>
            <div class="field">
              <label>Currency</label>
              <select name="currency" id="currency-select">
                <option value="CNY">CNY</option>
                <option value="USD">USD</option>
                <option value="HKD">HKD</option>
              </select>
            </div>
            <div class="field">
              <label>Account</label>
              <select name="account_id" id="account-select" required></select>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>Asset Type</label>
              <select name="asset_type" id="asset-select"></select>
            </div>
            <div class="field">
              <label>Type</label>
              <select name="transaction_type" id="type-select" required>
                <option value="BUY">BUY</option>
                <option value="SELL">SELL</option>
                <option value="DIVIDEND">DIVIDEND</option>
                <option value="TRANSFER_IN">TRANSFER_IN</option>
                <option value="TRANSFER_OUT">TRANSFER_OUT</option>
                <option value="ADJUST">ADJUST</option>
                <option value="INCOME">INCOME</option>
              </select>
            </div>
            <div class="field">
              <label>Symbol</label>
              <input type="text" name="symbol" id="symbol-input" placeholder="AAPL" required>
              <select id="symbol-select" style="display:none;"></select>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>Quantity</label>
              <input type="number" step="0.0001" name="quantity" id="quantity-input" required>
              <small id="quantity-hint" class="section-sub"></small>
            </div>
            <div class="field">
              <label>Price</label>
              <input type="number" step="0.0001" name="price" id="price-input" required>
            </div>
            <div class="field">
              <label>Commission</label>
              <input type="number" step="0.0001" name="commission" value="0">
            </div>
          </div>
          <div class="field">
            <label>Notes</label>
            <textarea name="notes" rows="3"></textarea>
          </div>
          <div class="actions">
            <button class="btn" type="submit">Save Transaction</button>
            <label class="pill">
              <input type="checkbox" name="link_cash" style="margin-right:6px;">Link cash
            </label>
          </div>
        </form>
      </div>
    `;

    const currencySelect = document.getElementById('currency-select');
    const accountSelect = document.getElementById('account-select');
    const assetSelect = document.getElementById('asset-select');
    const typeSelect = document.getElementById('type-select');
    const symbolInput = document.getElementById('symbol-input');
    const symbolSelect = document.getElementById('symbol-select');
    const quantityInput = document.getElementById('quantity-input');
    const quantityHint = document.getElementById('quantity-hint');
    const priceInput = document.getElementById('price-input');

    const buildAccountOptions = (items) => items.map((a) => `
      <option value="${escapeHtml(a.account_id)}">${escapeHtml(a.account_name || a.account_id)}</option>
    `).join('');

    const buildAssetOptions = (items) => items.map((a) => `
      <option value="${escapeHtml(a.code)}">${escapeHtml(a.label)}</option>
    `).join('');

    function updateAccountOptions() {
      const currency = currencySelect.value;
      const accountIds = new Set(holdings.filter((h) => h.currency === currency).map((h) => h.accountId));
      let candidates = accountIds.size
        ? accounts.filter((a) => accountIds.has(a.account_id))
        : accounts;
      if (!candidates.length) {
        candidates = accounts;
      }
      const current = accountSelect.value;
      accountSelect.innerHTML = buildAccountOptions(candidates);
      if (current && candidates.some((a) => a.account_id === current)) {
        accountSelect.value = current;
      }
    }

    function updateAssetTypeOptions() {
      const currency = currencySelect.value;
      const accountId = accountSelect.value;
      const typesInHoldings = new Set(
        holdings
          .filter((h) => h.currency === currency && h.accountId === accountId)
          .map((h) => h.assetType)
          .filter(Boolean)
      );
      let options = [];
      if (typesInHoldings.size) {
        const ordered = assetTypes
          .filter((a) => typesInHoldings.has(String(a.code).toLowerCase()))
          .map((a) => ({ code: a.code, label: a.label }));
        const extras = Array.from(typesInHoldings)
          .filter((code) => !assetLabelMap.has(code))
          .map((code) => ({ code, label: code }));
        options = [...ordered, ...extras];
      } else {
        options = assetTypes.map((a) => ({ code: a.code, label: a.label }));
      }
      const current = assetSelect.value;
      assetSelect.innerHTML = buildAssetOptions(options);
      if (current && options.some((opt) => String(opt.code).toLowerCase() === String(current).toLowerCase())) {
        assetSelect.value = current;
      }
    }

    function updatePriceLock() {
      const asset = String(assetSelect.value || '').toLowerCase();
      if (asset === 'cash') {
        priceInput.value = '1';
        priceInput.readOnly = true;
      } else {
        priceInput.readOnly = false;
      }
    }

    function getSellHoldings() {
      const currency = currencySelect.value;
      const accountId = accountSelect.value;
      const asset = String(assetSelect.value || '').toLowerCase();
      return holdings.filter((h) => (
        h.currency === currency &&
        h.accountId === accountId &&
        h.assetType === asset &&
        h.totalShares > 0
      ));
    }

    function updateSellConstraints() {
      const sellHoldings = getSellHoldings();
      const selectedSymbol = symbolSelect.value;
      const selected = sellHoldings.find((h) => h.symbol === selectedSymbol) || sellHoldings[0];
      if (!selected) {
        quantityInput.removeAttribute('max');
        quantityHint.textContent = '';
        quantityInput.disabled = true;
        return;
      }
      quantityInput.disabled = false;
      quantityInput.max = selected.totalShares;
      quantityHint.textContent = `Max: ${formatNumber(selected.totalShares)}`;
    }

    function updateSymbolMode() {
      const sellMode = typeSelect.value === 'SELL';
      if (sellMode) {
        symbolInput.style.display = 'none';
        symbolInput.name = '';
        symbolInput.required = false;
        symbolSelect.style.display = 'block';
        symbolSelect.name = 'symbol';
        symbolSelect.required = true;

        const sellHoldings = getSellHoldings();
        if (sellHoldings.length) {
          symbolSelect.disabled = false;
          symbolSelect.innerHTML = sellHoldings.map((h) => `
            <option value="${escapeHtml(h.symbol)}">${escapeHtml(h.displayName)} (${escapeHtml(h.symbol)})</option>
          `).join('');
          if (!sellHoldings.some((h) => h.symbol === symbolSelect.value)) {
            symbolSelect.value = sellHoldings[0].symbol;
          }
        } else {
          symbolSelect.innerHTML = '<option value="">No holdings</option>';
          symbolSelect.disabled = true;
        }
        updateSellConstraints();
      } else {
        symbolSelect.style.display = 'none';
        symbolSelect.name = '';
        symbolSelect.required = false;
        symbolInput.style.display = 'block';
        symbolInput.name = 'symbol';
        symbolInput.required = true;
        quantityInput.removeAttribute('max');
        quantityHint.textContent = '';
        quantityInput.disabled = false;
      }
    }

    currencySelect.addEventListener('change', () => {
      updateAccountOptions();
      updateAssetTypeOptions();
      updatePriceLock();
      updateSymbolMode();
    });

    accountSelect.addEventListener('change', () => {
      updateAssetTypeOptions();
      updatePriceLock();
      updateSymbolMode();
    });

    assetSelect.addEventListener('change', () => {
      updatePriceLock();
      updateSymbolMode();
    });

    typeSelect.addEventListener('change', updateSymbolMode);
    symbolSelect.addEventListener('change', updateSellConstraints);

    updateAccountOptions();
    updateAssetTypeOptions();
    updatePriceLock();
    updateSymbolMode();

    // Prefill from URL parameters (from Holdings quick trade)
    const hashParams = new URLSearchParams(window.location.hash.split('?')[1] || '');
    const prefillCurrency = hashParams.get('currency');
    const prefillAccount = hashParams.get('account');
    const prefillType = hashParams.get('type');
    const prefillSymbol = hashParams.get('symbol');
    const prefillAssetType = hashParams.get('asset_type');

    if (prefillCurrency && ['CNY', 'USD', 'HKD'].includes(prefillCurrency)) {
      currencySelect.value = prefillCurrency;
      updateAccountOptions();
      updateAssetTypeOptions();
    }
    if (prefillAccount && Array.from(accountSelect.options).some((o) => o.value === prefillAccount)) {
      accountSelect.value = prefillAccount;
      updateAssetTypeOptions();
    }
    if (prefillAssetType) {
      const assetMatch = Array.from(assetSelect.options).find((o) => o.value.toLowerCase() === prefillAssetType.toLowerCase());
      if (assetMatch) {
        assetSelect.value = assetMatch.value;
        updatePriceLock();
      }
    }
    if (prefillType && Array.from(typeSelect.options).some((o) => o.value === prefillType)) {
      typeSelect.value = prefillType;
      updateSymbolMode();
    }
    if (prefillSymbol) {
      if (typeSelect.value === 'SELL') {
        const sellMatch = Array.from(symbolSelect.options).find((o) => o.value === prefillSymbol);
        if (sellMatch) {
          symbolSelect.value = prefillSymbol;
          updateSellConstraints();
        }
      } else {
        symbolInput.value = prefillSymbol;
      }
    }

    const form = document.getElementById('tx-form');
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      const formData = new FormData(form);
      const payload = Object.fromEntries(formData.entries());
      payload.quantity = Number(payload.quantity);
      payload.price = Number(payload.price);
      payload.commission = Number(payload.commission || 0);
      payload.link_cash = formData.get('link_cash') === 'on';

      if (payload.transaction_type === 'SELL') {
        const sellHoldings = getSellHoldings();
        const selected = sellHoldings.find((h) => h.symbol === payload.symbol);
        if (!selected) {
          showToast('No holdings to sell');
          return;
        }
        if (payload.quantity > selected.totalShares) {
          showToast('Quantity exceeds holdings');
          return;
        }
      }

      if (String(payload.asset_type || '').toLowerCase() === 'cash') {
        payload.price = 1;
      }

      try {
        await fetchJSON('/api/transactions', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        showToast('Transaction saved');
        window.location.hash = '#/transactions';
      } catch (err) {
        showToast('Failed to save');
      }
    });
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load form. Add an account first in Settings.');
  }
}

async function fetchAllTransactionsForExport() {
  const pageSize = 500;
  let offset = 0;
  let total = 0;
  const items = [];

  while (true) {
    const page = await fetchJSON(`/api/transactions?paged=1&limit=${pageSize}&offset=${offset}`);
    const batch = Array.isArray(page.items) ? page.items : [];
    total = Number(page.total || total);
    items.push(...batch);
    offset += batch.length;
    if (total > 0) {
      if (offset >= total) {
        break;
      }
    } else if (batch.length < pageSize) {
      break;
    }
  }

  return items;
}

async function exportBackupData() {
  const [accounts, assetTypes, allocationSettings, exchangeRates, symbols, storageInfo, transactions] = await Promise.all([
    fetchJSON('/api/accounts'),
    fetchJSON('/api/asset-types'),
    fetchJSON('/api/allocation-settings'),
    fetchJSON('/api/exchange-rates'),
    fetchJSON('/api/symbols'),
    fetchJSON('/api/storage'),
    fetchAllTransactionsForExport(),
  ]);

  const payload = {
    exported_at: new Date().toISOString(),
    storage: {
      db_name: storageInfo && storageInfo.db_name ? storageInfo.db_name : '',
      data_dir: storageInfo && storageInfo.data_dir ? storageInfo.data_dir : '',
    },
    data: {
      accounts,
      asset_types: assetTypes,
      allocation_settings: allocationSettings,
      exchange_rates: exchangeRates,
      symbols,
      transactions,
    },
  };

  const stamp = new Date().toISOString().slice(0, 10);
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `invest-log-backup-${stamp}.json`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

async function renderSettings() {
  view.innerHTML = `
    <div class="section-title">Settings</div>
    <div class="section-sub">Accounts, FX rates, asset types, allocation ranges, storage, and API connection.</div>
    <div class="card">Loading settings...</div>
  `;

  try {
    const [accounts, assetTypes, allocationSettings, exchangeRates, symbols, holdings, storageInfo] = await Promise.all([
      fetchJSON('/api/accounts'),
      fetchJSON('/api/asset-types'),
      fetchJSON('/api/allocation-settings'),
      fetchJSON('/api/exchange-rates'),
      fetchJSON('/api/symbols'),
      fetchJSON('/api/holdings'),
      fetchJSON('/api/storage')
    ]);

    const settingsMap = {};
    allocationSettings.forEach((s) => {
      settingsMap[`${s.currency}_${s.asset_type}`] = s;
    });
    const accountsWithHoldings = new Set((holdings || []).map((h) => String(h.account_id || '')));
    const assetTypesWithHoldings = new Set((holdings || [])
      .map((h) => String(h.asset_type || '').toLowerCase())
      .filter(Boolean));

    const apiSection = `
      <div class="card">
        <h3>API Connection</h3>
        <div class="section-sub">Used when running from file or mobile container.</div>
        <div class="form-row">
          <div class="field">
            <label>API Base URL</label>
            <input id="api-base" type="text" placeholder="http://127.0.0.1:8000" value="${escapeHtml(state.apiBase)}">
          </div>
          <div class="actions">
            <button class="btn" id="save-api" type="button">Save</button>
          </div>
        </div>
      </div>
    `;

    const aiSettings = loadAIAnalysisSettings();
    const aiAnalysisSection = `
      <div class="card">
        <h3>AI Analysis</h3>
        <div class="section-sub">OpenAI-compatible configuration for holdings analysis.</div>
        <div class="form">
          <div class="form-row">
            <div class="field">
              <label>AI Base URL</label>
              <input id="ai-base-url" type="text" placeholder="https://api.openai.com/v1" value="${escapeHtml(aiSettings.baseUrl || 'https://api.openai.com/v1')}">
            </div>
            <div class="field">
              <label>Model</label>
              <input id="ai-model" type="text" placeholder="gpt-4o-mini" value="${escapeHtml(aiSettings.model || '')}">
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>API Key</label>
              <input id="ai-api-key" type="password" autocomplete="off" placeholder="sk-..." value="${escapeHtml(aiSettings.apiKey || '')}">
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>Risk Profile</label>
              <select id="ai-risk-profile">
                <option value="conservative" ${aiSettings.riskProfile === 'conservative' ? 'selected' : ''}>Conservative</option>
                <option value="balanced" ${aiSettings.riskProfile === 'balanced' ? 'selected' : ''}>Balanced</option>
                <option value="aggressive" ${aiSettings.riskProfile === 'aggressive' ? 'selected' : ''}>Aggressive</option>
              </select>
            </div>
            <div class="field">
              <label>Horizon</label>
              <select id="ai-horizon">
                <option value="short" ${aiSettings.horizon === 'short' ? 'selected' : ''}>Short</option>
                <option value="medium" ${aiSettings.horizon === 'medium' ? 'selected' : ''}>Medium</option>
                <option value="long" ${aiSettings.horizon === 'long' ? 'selected' : ''}>Long</option>
              </select>
            </div>
            <div class="field">
              <label>Advice Style</label>
              <select id="ai-advice-style">
                <option value="conservative" ${aiSettings.adviceStyle === 'conservative' ? 'selected' : ''}>Conservative</option>
                <option value="balanced" ${aiSettings.adviceStyle === 'balanced' ? 'selected' : ''}>Balanced</option>
                <option value="aggressive" ${aiSettings.adviceStyle === 'aggressive' ? 'selected' : ''}>Aggressive</option>
              </select>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>
                <input id="ai-allow-new-symbols" type="checkbox" ${aiSettings.allowNewSymbols !== false ? 'checked' : ''}>
                Allow new symbols in suggestions
              </label>
            </div>
            <div class="actions">
              <button class="btn" id="save-ai-analysis" type="button">Save AI Settings</button>
            </div>
          </div>
        </div>
      </div>
    `;

    const exchangeRateMap = {};
    (exchangeRates || []).forEach((item) => {
      if (!item) {
        return;
      }
      const key = `${item.from_currency}_${item.to_currency}`;
      exchangeRateMap[key] = item;
    });

    const exchangePairs = [
      { from: 'USD', to: 'CNY', label: 'USD → CNY' },
      { from: 'HKD', to: 'CNY', label: 'HKD → CNY' },
    ];
    const exchangeRows = exchangePairs.map((pair) => {
      const key = `${pair.from}_${pair.to}`;
      const item = exchangeRateMap[key] || {};
      const rate = Number(item.rate || 0);
      const updatedAt = item.updated_at ? escapeHtml(String(item.updated_at)) : '—';
      const source = item.source ? escapeHtml(String(item.source)) : 'manual';
      return `
        <div class="list-item allocation-item">
          <div>
            <strong>${pair.label}</strong>
            <div class="section-sub">Source: ${source} · Updated: ${updatedAt}</div>
          </div>
          <div class="allocation-controls fx-controls">
            <input type="number" step="0.0001" min="0.0001" value="${rate > 0 ? rate : ''}" data-fx-rate data-from="${pair.from}" data-to="${pair.to}">
            <button class="btn secondary" data-fx-save data-from="${pair.from}" data-to="${pair.to}">Save</button>
          </div>
        </div>
      `;
    }).join('');

    const exchangeSection = `
      <div class="card">
        <h3>Exchange Rates</h3>
        <div class="section-sub">Maintain USD/HKD conversion rates to CNY for total assets.</div>
        <div class="actions" style="margin-bottom: 12px;">
          <button class="btn" id="refresh-exchange-rates" type="button">Fetch Online</button>
        </div>
        <div class="list">${exchangeRows}</div>
      </div>
    `;

    const accountsList = accounts.map((a) => {
      const accountId = String(a.account_id || '');
      const hasHoldings = accountsWithHoldings.has(accountId);
      const deleteDisabled = hasHoldings ? 'disabled title="Account has holdings"' : '';
      const holdingTag = hasHoldings ? '<span class="tag other">In use</span>' : '';
      return `
        <div class="list-item">
          <div>
            <strong>${escapeHtml(a.account_name)}</strong>
            <div class="section-sub">${escapeHtml(accountId)}</div>
            ${holdingTag ? `<div>${holdingTag}</div>` : ''}
          </div>
          <button class="btn danger" data-account="${escapeHtml(accountId)}" ${deleteDisabled}>Delete</button>
        </div>
      `;
    }).join('');

    const accountsSection = `
      <div class="card">
        <h3>Accounts</h3>
        <div class="list">${accountsList || '<div class="section-sub">No accounts yet.</div>'}</div>
        <form id="account-form" class="form">
          <div class="form-row">
            <div class="field">
              <label>Account ID</label>
              <input name="account_id" required>
            </div>
            <div class="field">
              <label>Name</label>
              <input name="account_name" required>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>Initial CNY</label>
              <input type="number" step="0.01" name="initial_balance_cny" value="0">
            </div>
            <div class="field">
              <label>Initial USD</label>
              <input type="number" step="0.01" name="initial_balance_usd" value="0">
            </div>
            <div class="field">
              <label>Initial HKD</label>
              <input type="number" step="0.01" name="initial_balance_hkd" value="0">
            </div>
          </div>
          <button class="btn" type="submit">Add Account</button>
        </form>
      </div>
    `;

    const assetList = assetTypes.map((a) => {
      const assetCode = String(a.code || '');
      const hasHoldings = assetTypesWithHoldings.has(assetCode.toLowerCase());
      const deleteDisabled = hasHoldings ? 'disabled title="Asset type has holdings"' : '';
      const holdingTag = hasHoldings ? '<span class="tag other">In use</span>' : '';
      return `
        <div class="list-item">
          <div>
            <div><strong>${escapeHtml(a.label)}</strong> <span class="section-sub">${escapeHtml(assetCode)}</span></div>
            ${holdingTag ? `<div>${holdingTag}</div>` : ''}
          </div>
          <button class="btn danger" data-asset="${escapeHtml(assetCode)}" ${deleteDisabled}>Delete</button>
        </div>
      `;
    }).join('');

    const assetSection = `
      <div class="card">
        <h3>Asset Types</h3>
        <div class="list">${assetList || '<div class="section-sub">No asset types.</div>'}</div>
        <form id="asset-form" class="form">
          <div class="form-row">
            <div class="field">
              <label>Code</label>
              <input name="code" required>
            </div>
            <div class="field">
              <label>Label</label>
              <input name="label" required>
            </div>
          </div>
          <button class="btn" type="submit">Add Asset Type</button>
        </form>
      </div>
    `;

    const allocationCards = ['CNY', 'USD', 'HKD'].map((currency) => {
      const rows = assetTypes.map((a) => {
        const key = `${currency}_${a.code}`;
        const setting = settingsMap[key] || { min_percent: 0, max_percent: 100 };
        return `
          <div class="list-item allocation-item">
            <div>
              <strong>${escapeHtml(a.label)}</strong>
              <div class="section-sub">${currency}</div>
            </div>
            <div class="allocation-controls">
              <input type="number" step="0.1" value="${setting.min_percent}" data-alloc-min data-currency="${currency}" data-asset="${escapeHtml(a.code)}">
              <input type="number" step="0.1" value="${setting.max_percent}" data-alloc-max data-currency="${currency}" data-asset="${escapeHtml(a.code)}">
              <button class="btn secondary" data-alloc-save data-currency="${currency}" data-asset="${escapeHtml(a.code)}">Save</button>
            </div>
          </div>
        `;
      }).join('');
      return `
        <div class="card">
          <h3>${currency} Allocation Targets</h3>
          <div class="section-sub">Set min/max percentage bands for ${currency}.</div>
          <div class="list">${rows}</div>
        </div>
      `;
    }).join('');

    const symbolRows = (symbols || []).map((sym) => {
      const symbolRaw = String(sym.symbol || '');
      const symbol = escapeHtml(symbolRaw);
      const nameRaw = sym.name ? String(sym.name) : '';
      const nameValue = nameRaw ? escapeHtml(nameRaw) : '';
      const symAsset = sym.asset_type ? String(sym.asset_type).toLowerCase() : '';
      const assetOptions = assetTypes.map((a) => {
        const assetCode = String(a.code).toLowerCase();
        const selected = assetCode === symAsset ? 'selected' : '';
        return `<option value="${escapeHtml(a.code)}" ${selected}>${escapeHtml(a.label)}</option>`;
      }).join('');
      const autoChecked = sym.auto_update ? 'checked' : '';
      return `
        <tr data-symbol-row data-symbol-raw="${symbol}">
          <td><strong data-symbol-text>${symbol}</strong></td>
          <td><input class="table-input" type="text" value="${nameValue}" data-symbol-field="name" data-symbol="${symbol}"></td>
          <td>
            <select class="table-select" data-symbol-field="asset" data-symbol="${symbol}">
              ${assetOptions}
            </select>
          </td>
          <td>
            <label class="toggle">
              <input type="checkbox" data-symbol-field="auto" data-symbol="${symbol}" ${autoChecked}>
              Auto
            </label>
          </td>
          <td>
            <button class="btn secondary" data-action="save-symbol" data-symbol="${symbol}">Save</button>
          </td>
        </tr>
      `;
    }).join('');

    const symbolCount = (symbols || []).length;
    const symbolsSection = `
      <div class="card span-2">
        <h3>Symbols</h3>
        <div class="section-sub">Update display names, asset types, and auto sync status.</div>
        ${symbolRows
          ? `<div class="form-row symbols-filter-row">
              <div class="field">
                <label for="symbol-filter-input">Quick Filter</label>
                <input id="symbol-filter-input" type="search" placeholder="Search symbol, name, or asset type" autocomplete="off">
              </div>
              <div class="actions symbols-filter-actions">
                <button class="btn secondary" id="symbol-filter-clear" type="button" disabled>Clear</button>
                <span class="tag other" id="symbol-filter-count">Total ${symbolCount} symbol(s)</span>
              </div>
            </div>
            <table class="table">
              <thead>
                <tr>
                  <th>Symbol</th>
                  <th>Name</th>
                  <th>Asset Type</th>
                  <th>Auto</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody data-symbols-table-body>
                ${symbolRows}
                <tr class="symbols-empty-row" data-symbols-empty-row style="display:none;">
                  <td colspan="5">No matching symbols.</td>
                </tr>
              </tbody>
            </table>`
          : '<div class="section-sub">No symbols found yet.</div>'}
      </div>
    `;

    const storage = storageInfo || {};
    const availableFiles = Array.isArray(storage.available) ? [...storage.available] : [];
    const currentDBName = storage.db_name || '';
    if (currentDBName && !availableFiles.includes(currentDBName)) {
      availableFiles.unshift(currentDBName);
    }
    const canSwitch = storage.can_switch !== false;
    const switchDisabled = canSwitch ? '' : 'disabled';
    const switchNote = canSwitch
      ? ''
      : `<div class="section-sub">${escapeHtml(storage.switch_reason || 'Storage switching disabled.')}</div>`;
    const storageOptions = availableFiles.length
      ? availableFiles.map((name) => {
        const selected = name === currentDBName ? 'selected' : '';
        return `<option value="${escapeHtml(name)}" ${selected}>${escapeHtml(name)}</option>`;
      }).join('')
      : '<option value="">No storage files</option>';

    const storageSection = `
      <div class="card">
        <h3>Storage</h3>
        <div class="section-sub">Switch data files for different users.</div>
        <div class="form">
          <div class="form-row">
            <div class="field">
              <label>Current File</label>
              <input type="text" value="${escapeHtml(currentDBName || 'Unknown')}" disabled>
            </div>
            <div class="field">
              <label>Data Directory</label>
              <input type="text" value="${escapeHtml(storage.data_dir || 'Unknown')}" disabled>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>Existing Files</label>
              <select id="storage-select" ${switchDisabled}>
                ${storageOptions}
              </select>
            </div>
            <div class="actions">
              <button class="btn" id="storage-switch" type="button" ${switchDisabled}>Switch</button>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>New File</label>
              <input id="storage-new" placeholder="alice.db" ${switchDisabled}>
            </div>
            <div class="actions">
              <button class="btn secondary" id="storage-create" type="button" ${switchDisabled}>Create & Switch</button>
            </div>
          </div>
          ${switchNote}
        </div>
      </div>
    `;

    const backupSection = `
      <div class="card">
        <h3>Backup</h3>
        <div class="section-sub">Export all data as a JSON file for local backup.</div>
        <div class="actions">
          <button class="btn" id="export-data" type="button">Export data</button>
        </div>
      </div>
    `;

    const settingsTabs = [
      {
        key: 'accounts',
        label: 'Accounts',
        content: `<div class="grid two">${accountsSection}</div>`,
      },
      {
        key: 'exchange',
        label: 'Exchange Rates',
        content: `<div class="grid two">${exchangeSection}</div>`,
      },
      {
        key: 'assets',
        label: 'Asset Types',
        content: `<div class="grid two">${assetSection}</div>`,
      },
      {
        key: 'allocations',
        label: 'Allocations',
        content: `<div class="grid three">${allocationCards}</div>`,
      },
      {
        key: 'symbols',
        label: 'Symbols',
        content: `<div class="grid two">${symbolsSection}</div>`,
      },
      {
        key: 'storage',
        label: 'Storage',
        content: `<div class="grid two">${storageSection}${backupSection}</div>`,
      },
      {
        key: 'api',
        label: 'API',
        content: `<div class="grid two">${apiSection}${aiAnalysisSection}</div>`,
      },
    ];

    const tabButtons = settingsTabs.map((tab) => `
      <button class="tab-button" data-settings-tab="${tab.key}" type="button">${tab.label}</button>
    `).join('');

    const panels = settingsTabs.map((tab) => `
      <div class="tab-panel" data-settings-panel="${tab.key}">
        ${tab.content}
      </div>
    `).join('');

    view.innerHTML = `
      <div class="section-title">Settings</div>
      <div class="section-sub">Accounts, FX rates, asset types, allocation ranges, storage, and API connection.</div>
      <div class="tab-bar" role="tablist">${tabButtons}</div>
      ${panels}
    `;

    initSettingsTabs();
    bindSettingsActions();
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load settings.');
  }
}

function initSettingsTabs() {
  const tabs = Array.from(view.querySelectorAll('[data-settings-tab]'));
  const panels = Array.from(view.querySelectorAll('[data-settings-panel]'));
  if (!tabs.length || !panels.length) {
    return;
  }
  const available = tabs.map((btn) => btn.dataset.settingsTab);
  const saved = localStorage.getItem('activeSettingsTab');
  const initial = available.includes(saved) ? saved : available[0];

  const setActive = (key) => {
    tabs.forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.settingsTab === key);
    });
    panels.forEach((panel) => {
      panel.classList.toggle('active', panel.dataset.settingsPanel === key);
    });
    localStorage.setItem('activeSettingsTab', key);
  };

  tabs.forEach((btn) => {
    btn.addEventListener('click', () => {
      setActive(btn.dataset.settingsTab);
    });
  });

  setActive(initial);
}

function bindSettingsActions() {
  const saveApi = document.getElementById('save-api');
  if (saveApi) {
    saveApi.addEventListener('click', () => {
      const input = document.getElementById('api-base');
      if (!input) return;
      const value = trimTrailingSlash(input.value.trim());
      if (!value) {
        localStorage.removeItem('apiBase');
        state.apiBase = '';
      } else {
        localStorage.setItem('apiBase', value);
        state.apiBase = value;
      }
      updateConnectionStatus();
      showToast('API base saved');
    });
  }

  const saveAIAnalysis = document.getElementById('save-ai-analysis');
  if (saveAIAnalysis) {
    saveAIAnalysis.addEventListener('click', () => {
      const baseUrlInput = document.getElementById('ai-base-url');
      const modelInput = document.getElementById('ai-model');
      const apiKeyInput = document.getElementById('ai-api-key');
      const riskProfileInput = document.getElementById('ai-risk-profile');
      const horizonInput = document.getElementById('ai-horizon');
      const adviceStyleInput = document.getElementById('ai-advice-style');
      const allowNewSymbolsInput = document.getElementById('ai-allow-new-symbols');

      const settings = {
        baseUrl: trimTrailingSlash(baseUrlInput && baseUrlInput.value ? baseUrlInput.value.trim() : '') || 'https://api.openai.com/v1',
        model: modelInput && modelInput.value ? modelInput.value.trim() : '',
        apiKey: apiKeyInput && apiKeyInput.value ? apiKeyInput.value.trim() : '',
        riskProfile: riskProfileInput && riskProfileInput.value ? riskProfileInput.value : 'balanced',
        horizon: horizonInput && horizonInput.value ? horizonInput.value : 'medium',
        adviceStyle: adviceStyleInput && adviceStyleInput.value ? adviceStyleInput.value : 'balanced',
        allowNewSymbols: allowNewSymbolsInput ? !!allowNewSymbolsInput.checked : true,
      };

      saveAIAnalysisSettings(settings);
      if (!settings.model || !settings.apiKey) {
        showToast('Saved. Set model and API key before running analysis');
      } else {
        showToast('AI settings saved');
      }
    });
  }

  const storageSwitch = document.getElementById('storage-switch');
  if (storageSwitch) {
    storageSwitch.addEventListener('click', async () => {
      if (storageSwitch.disabled) return;
      const select = document.getElementById('storage-select');
      if (!select || !select.value) {
        showToast('Select a storage file');
        return;
      }
      try {
        await fetchJSON('/api/storage/switch', {
          method: 'POST',
          body: JSON.stringify({ db_name: select.value }),
        });
        showToast('Storage switched');
        renderSettings();
      } catch (err) {
        showToast('Switch failed');
      }
    });
  }

  const storageCreate = document.getElementById('storage-create');
  if (storageCreate) {
    storageCreate.addEventListener('click', async () => {
      if (storageCreate.disabled) return;
      const input = document.getElementById('storage-new');
      const value = input ? input.value.trim() : '';
      if (!value) {
        showToast('Enter a new file name');
        return;
      }
      try {
        await fetchJSON('/api/storage/switch', {
          method: 'POST',
          body: JSON.stringify({ db_name: value, create: true }),
        });
        if (input) {
          input.value = '';
        }
        showToast('Storage switched');
        renderSettings();
      } catch (err) {
        showToast('Create failed');
      }
    });
  }

  const exportBtn = document.getElementById('export-data');
  if (exportBtn) {
    exportBtn.addEventListener('click', async () => {
      if (exportBtn.disabled) return;
      try {
        showToast('Preparing export...');
        await exportBackupData();
        showToast('Export ready');
      } catch (err) {
        showToast('Export failed');
      }
    });
  }

  const accountForm = document.getElementById('account-form');
  if (accountForm) {
    accountForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      const formData = new FormData(accountForm);
      const payload = Object.fromEntries(formData.entries());
      payload.initial_balance_cny = Number(payload.initial_balance_cny || 0);
      payload.initial_balance_usd = Number(payload.initial_balance_usd || 0);
      payload.initial_balance_hkd = Number(payload.initial_balance_hkd || 0);
      try {
        await fetchJSON('/api/accounts', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        showToast('Account added');
        renderSettings();
      } catch (err) {
        showToast('Failed to add account');
      }
    });
  }

  view.querySelectorAll('button[data-account]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      if (btn.disabled) return;
      const accountID = btn.dataset.account;
      if (!confirm('Delete this account?')) return;
      try {
        await fetchJSON(`/api/accounts/${accountID}`, { method: 'DELETE' });
        showToast('Account deleted');
        renderSettings();
      } catch (err) {
        showToast('Delete failed');
      }
    });
  });

  const assetForm = document.getElementById('asset-form');
  if (assetForm) {
    assetForm.addEventListener('submit', async (event) => {
      event.preventDefault();
      const payload = Object.fromEntries(new FormData(assetForm).entries());
      try {
        await fetchJSON('/api/asset-types', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        showToast('Asset type added');
        renderSettings();
      } catch (err) {
        showToast('Failed to add asset type');
      }
    });
  }

  view.querySelectorAll('button[data-asset]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      if (btn.disabled) return;
      const code = btn.dataset.asset;
      if (!confirm('Delete this asset type?')) return;
      try {
        await fetchJSON(`/api/asset-types/${code}`, { method: 'DELETE' });
        showToast('Asset type deleted');
        renderSettings();
      } catch (err) {
        showToast('Delete failed');
      }
    });
  });

  view.querySelectorAll('button[data-alloc-save]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const currency = btn.dataset.currency;
      const asset = btn.dataset.asset;
      const minInput = view.querySelector(`input[data-alloc-min][data-currency="${currency}"][data-asset="${asset}"]`);
      const maxInput = view.querySelector(`input[data-alloc-max][data-currency="${currency}"][data-asset="${asset}"]`);
      if (!minInput || !maxInput) return;
      const payload = {
        currency,
        asset_type: asset,
        min_percent: Number(minInput.value || 0),
        max_percent: Number(maxInput.value || 100),
      };
      try {
        await fetchJSON('/api/allocation-settings', {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
        showToast('Allocation updated');
      } catch (err) {
        showToast('Update failed');
      }
    });
  });

  view.querySelectorAll('button[data-fx-save]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const fromCurrency = btn.dataset.from;
      const toCurrency = btn.dataset.to;
      const input = view.querySelector(`input[data-fx-rate][data-from="${fromCurrency}"][data-to="${toCurrency}"]`);
      if (!input) {
        return;
      }
      const rate = Number(input.value || 0);
      if (!(rate > 0)) {
        showToast('Enter a valid rate');
        return;
      }
      try {
        await fetchJSON('/api/exchange-rates', {
          method: 'PUT',
          body: JSON.stringify({
            from_currency: fromCurrency,
            to_currency: toCurrency,
            rate,
          }),
        });
        showToast(`${fromCurrency}/${toCurrency} updated`);
        renderSettings();
      } catch (err) {
        showToast('Exchange rate update failed');
      }
    });
  });

  const refreshExchangeRatesBtn = document.getElementById('refresh-exchange-rates');
  if (refreshExchangeRatesBtn) {
    refreshExchangeRatesBtn.addEventListener('click', async () => {
      if (refreshExchangeRatesBtn.disabled) {
        return;
      }
      refreshExchangeRatesBtn.disabled = true;
      try {
        const result = await fetchJSON('/api/exchange-rates/refresh', {
          method: 'POST',
          body: JSON.stringify({}),
        });
        const updated = Number(result && result.updated ? result.updated : 0);
        const errors = Array.isArray(result && result.errors) ? result.errors : [];
        if (errors.length) {
          showToast(`Fetched ${updated}, ${errors.length} failed`);
        } else {
          showToast(`Fetched ${updated} rate(s)`);
        }
        renderSettings();
      } catch (err) {
        showToast('Fetch exchange rates failed');
      } finally {
        refreshExchangeRatesBtn.disabled = false;
      }
    });
  }

  const symbolFilterInput = document.getElementById('symbol-filter-input');
  const symbolFilterClear = document.getElementById('symbol-filter-clear');
  const symbolFilterCount = document.getElementById('symbol-filter-count');
  const symbolsTableBody = view.querySelector('[data-symbols-table-body]');
  const symbolsEmptyRow = view.querySelector('[data-symbols-empty-row]');

  if (symbolFilterInput && symbolsTableBody) {
    const symbolTableRows = Array.from(symbolsTableBody.querySelectorAll('tr[data-symbol-row]'));
    const totalCount = symbolTableRows.length;

    const applySymbolFilter = () => {
      const keyword = symbolFilterInput.value.trim().toLowerCase();
      let visibleCount = 0;

      symbolTableRows.forEach((row) => {
        const symbolRaw = String(row.dataset.symbolRaw || '');
        const symbolEl = row.querySelector('[data-symbol-text]');
        const nameInput = row.querySelector('input[data-symbol-field="name"]');
        const assetSelect = row.querySelector('select[data-symbol-field="asset"]');
        const nameText = nameInput ? String(nameInput.value || '') : '';
        const assetValue = assetSelect ? String(assetSelect.value || '') : '';
        const assetLabel = assetSelect && assetSelect.selectedOptions && assetSelect.selectedOptions[0]
          ? String(assetSelect.selectedOptions[0].textContent || '')
          : '';
        const rowFilterText = `${symbolRaw} ${nameText} ${assetValue} ${assetLabel}`.toLowerCase();
        const symbolHit = keyword && symbolRaw.toLowerCase().includes(keyword);
        const nameHit = keyword && nameText.toLowerCase().includes(keyword);
        const assetHit = keyword && `${assetValue} ${assetLabel}`.toLowerCase().includes(keyword);
        const matched = !keyword || rowFilterText.includes(keyword);

        if (symbolEl) {
          symbolEl.innerHTML = keyword ? highlightMatchText(symbolRaw, keyword) : escapeHtml(symbolRaw);
          symbolEl.classList.toggle('symbol-match-hit', !!symbolHit);
        }
        if (nameInput) {
          nameInput.classList.toggle('filter-hit', !!nameHit);
        }
        if (assetSelect) {
          assetSelect.classList.toggle('filter-hit', !!assetHit);
        }

        row.style.display = matched ? '' : 'none';
        if (matched) {
          visibleCount += 1;
        }
      });

      if (symbolFilterCount) {
        symbolFilterCount.textContent = keyword
          ? `Showing ${visibleCount} / ${totalCount}`
          : `Total ${totalCount} symbol(s)`;
      }
      if (symbolsEmptyRow) {
        symbolsEmptyRow.style.display = visibleCount === 0 ? '' : 'none';
      }
      if (symbolFilterClear) {
        symbolFilterClear.disabled = keyword.length === 0;
      }
    };

    symbolFilterInput.addEventListener('input', applySymbolFilter);

    symbolsTableBody.addEventListener('input', (event) => {
      if (event.target && event.target.matches('input[data-symbol-field="name"]')) {
        applySymbolFilter();
      }
    });
    symbolsTableBody.addEventListener('change', (event) => {
      if (event.target && event.target.matches('select[data-symbol-field="asset"]')) {
        applySymbolFilter();
      }
    });

    if (symbolFilterClear) {
      symbolFilterClear.addEventListener('click', () => {
        symbolFilterInput.value = '';
        applySymbolFilter();
        symbolFilterInput.focus();
      });
    }

    applySymbolFilter();
  }

  view.querySelectorAll('button[data-action="save-symbol"]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const symbol = btn.dataset.symbol;
      const nameInput = view.querySelector(`input[data-symbol-field="name"][data-symbol="${symbol}"]`);
      const assetSelect = view.querySelector(`select[data-symbol-field="asset"][data-symbol="${symbol}"]`);
      const autoToggle = view.querySelector(`input[data-symbol-field="auto"][data-symbol="${symbol}"]`);
      const payload = {
        name: nameInput ? nameInput.value : '',
        asset_type: assetSelect ? assetSelect.value : '',
        auto_update: autoToggle && autoToggle.checked ? 1 : 0,
      };
      try {
        await fetchJSON(`/api/symbols/${encodeURIComponent(symbol)}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
        showToast(`${symbol} updated`);
      } catch (err) {
        showToast('Symbol update failed');
      }
    });
  });
}

function registerServiceWorker() {
  if ('serviceWorker' in navigator && window.location.protocol.startsWith('http')) {
    navigator.serviceWorker.register('sw.js').catch(() => {});
  }
}

document.addEventListener('DOMContentLoaded', init);
