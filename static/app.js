const state = {
  apiBase: '',
  privacy: false,
};

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

function formatNumber(value) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  return Number(value).toFixed(2);
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
    <div class="section-sub">Market value, allocations, and warning bands.</div>
    <div class="grid two">
      <div class="card">Loading summary...</div>
      <div class="card">Loading allocations...</div>
    </div>
  `;

  try {
    const [byCurrency, bySymbol] = await Promise.all([
      fetchJSON('/api/holdings-by-currency'),
      fetchJSON('/api/holdings-by-symbol')
    ]);

    const currencyList = Object.keys(byCurrency || {}).sort();
    let totalMarket = 0;
    let totalCost = 0;

    if (bySymbol) {
      Object.values(bySymbol).forEach((entry) => {
        totalMarket += entry.total_market_value || 0;
        totalCost += entry.total_cost || 0;
      });
    }

    const totalPnL = totalMarket - totalCost;
    const summaryCard = `
      <div class="card">
        <h3>Total Market Value</h3>
        <div class="value" data-sensitive>${formatNumber(totalMarket)}</div>
        <div class="section-sub">Sum across currencies (no FX conversion)</div>
        <div class="pill" data-sensitive>${formatNumber(totalPnL)} total PnL</div>
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
      <div class="section-sub">Market value, allocations, and warning bands.</div>
      <div class="grid two">${summaryCard}${allocationCards}</div>
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
      const symbols = data[currency].symbols || [];
      const canUpdateAll = symbols.some((s) => s.auto_update !== 0);
      const rows = symbols.map((s) => {
        const pnlClass = s.unrealized_pnl !== null && s.unrealized_pnl !== undefined ? (s.unrealized_pnl >= 0 ? 'pill' : 'alert') : '';
        const autoUpdate = s.auto_update !== 0;
        const updateDisabled = autoUpdate ? '' : 'disabled title="Auto sync off"';
        const autoTag = autoUpdate ? '' : '<span class="tag other">Auto off</span>';
        const pnlMarkup = s.unrealized_pnl !== null && s.unrealized_pnl !== undefined
          ? `<span class="${pnlClass}" data-sensitive>${formatMoney(s.unrealized_pnl, currency)}</span>`
          : '<span class="section-sub">—</span>';
        const accountSort = (s.account_name || s.account_id || '').toString();
        return `
          <tr data-account="${escapeHtml(accountSort.toLowerCase())}" data-market="${s.market_value || 0}" data-pnl="${s.unrealized_pnl || 0}">
            <td><strong>${escapeHtml(s.display_name)}</strong><br><span class="section-sub">${escapeHtml(s.symbol)}</span></td>
            <td><strong>${escapeHtml(s.account_name || s.account_id || '')}</strong><br><span class="section-sub">${escapeHtml(s.account_id || '')}</span></td>
            <td data-sensitive>${formatNumber(s.total_shares)}</td>
            <td data-sensitive>${formatMoney(s.avg_cost, currency)}</td>
            <td data-sensitive>${s.latest_price !== null ? formatMoney(s.latest_price, currency) : '—'}</td>
            <td data-sensitive>${formatMoney(s.market_value, currency)}</td>
            <td>${pnlMarkup}</td>
            <td>
              <div class="actions">
                <button class="btn secondary" data-action="update" data-symbol="${escapeHtml(s.symbol)}" data-currency="${currency}" ${updateDisabled}>Update</button>
                <button class="btn" data-action="manual" data-symbol="${escapeHtml(s.symbol)}" data-currency="${currency}">Manual</button>
                ${autoTag}
              </div>
            </td>
          </tr>
        `;
      }).join('');

      return `
        <div class="tab-panel" data-holdings-panel="${currency}">
          <div class="card">
            <div class="panel-head">
              <h3>${currency} Holdings</h3>
              <div class="actions">
                <button class="btn secondary" data-action="update-all" data-currency="${currency}" ${canUpdateAll ? '' : 'disabled title="No auto-sync symbols"'}>Update all</button>
              </div>
            </div>
            <table class="table" data-holdings-table>
              <thead>
                <tr>
                  <th>Symbol</th>
                  <th class="sortable" data-sort="account">Account</th>
                  <th>Shares</th>
                  <th>Avg Cost</th>
                  <th>Price</th>
                  <th class="sortable" data-sort="market">Market Value</th>
                  <th class="sortable" data-sort="pnl">PnL</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>${rows}</tbody>
            </table>
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
  view.querySelectorAll('button[data-action]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      if (btn.disabled) {
        return;
      }
      const action = btn.dataset.action;
      const symbol = btn.dataset.symbol;
      const currency = btn.dataset.currency;
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
            body: JSON.stringify({ symbol, currency }),
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
        renderHoldings();
      } catch (err) {
        showToast('Price update failed');
      }
    });
  });
}

async function renderTransactions() {
  view.innerHTML = `
    <div class="section-title">Transactions</div>
    <div class="section-sub">Filter by symbol, account, or type.</div>
    <div class="card">Loading transactions...</div>
  `;

  try {
    const [transactions, accounts] = await Promise.all([
      fetchJSON('/api/transactions?limit=200'),
      fetchJSON('/api/accounts')
    ]);
    const accountNameMap = new Map((accounts || []).map((a) => [a.account_id, a.account_name || a.account_id]));
    const rows = transactions.map((t) => {
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
          <td data-sensitive>${formatNumber(t.quantity)}</td>
          <td data-sensitive>${formatMoney(t.price, t.currency)}</td>
          <td data-sensitive>${formatMoney(t.total_amount, t.currency)}</td>
          <td><strong>${escapeHtml(resolvedAccountName)}</strong>${showAccountSub ? `<br><span class="section-sub">${escapeHtml(t.account_id || '')}</span>` : ''}</td>
          <td>
            <button class="btn danger" data-action="delete" data-id="${t.id}">Delete</button>
          </td>
        </tr>
      `;
    }).join('');

    view.innerHTML = `
      <div class="section-title">Transactions</div>
      <div class="section-sub">Filter by symbol, account, or type.</div>
      <div class="card">
        <table class="table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Symbol</th>
              <th>Type</th>
              <th>Qty</th>
              <th>Price</th>
              <th>Total</th>
              <th>Account</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>${rows || '<tr><td colspan="8">No transactions found.</td></tr>'}</tbody>
        </table>
      </div>
    `;

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

async function renderCharts() {
  view.innerHTML = `
    <div class="section-title">Charts</div>
    <div class="section-sub">Symbol composition snapshots by currency.</div>
    <div class="card">Loading charts...</div>
  `;

  try {
    const bySymbol = await fetchJSON('/api/holdings-by-symbol');

    const symbolBlocks = Object.entries(bySymbol || {}).map(([currency, data]) => {
      const symbols = data.symbols || [];
      const totalMarketValue = data.total_market_value !== undefined && data.total_market_value !== null
        ? data.total_market_value
        : symbols.reduce((sum, s) => sum + (s.market_value || 0), 0);
      const accountEntries = Object.entries(data.by_account || {}).map(([accountId, account]) => {
        const total = (account.symbols || []).reduce((sum, s) => sum + (s.market_value || 0), 0);
        return {
          id: accountId,
          name: account.account_name || accountId,
          total
        };
      }).sort((a, b) => b.total - a.total);
      const accountRows = accountEntries.map((acc) => `
        <div class="chart-row">
          <div>
            <strong>${escapeHtml(acc.name)}</strong>
            <div class="section-sub">${escapeHtml(acc.id)}</div>
          </div>
          <div class="bar"><span style="width:${totalMarketValue ? (acc.total / totalMarketValue) * 100 : 0}%;"></span></div>
          <div>${formatPercent(totalMarketValue ? (acc.total / totalMarketValue) * 100 : 0)}</div>
          <div data-sensitive>${formatMoney(acc.total, currency)}</div>
        </div>
      `).join('');
      const pieChart = renderPieChart({
        items: buildSymbolPieItems(symbols, 8),
        totalLabel: 'Total',
        totalValue: totalMarketValue,
        currency,
      });
      const rows = symbols.slice(0, 12).map((s) => `
        <div class="chart-row">
          <div>
            <strong>${escapeHtml(s.display_name)}</strong>
            <div class="section-sub">${escapeHtml(s.symbol)}</div>
          </div>
          <div class="bar"><span style="width:${s.percent}%;"></span></div>
          <div>${formatPercent(s.percent)}</div>
          <div data-sensitive>${formatMoney(s.market_value, currency)}</div>
        </div>
      `).join('');
      const listMarkup = rows ? `<div class="list">${rows}</div>` : '';
      const accountMarkup = accountRows ? `
        <div class="section-sub" style="margin-top:10px;">By account</div>
        <div class="list">${accountRows}</div>
      ` : '';

      return `
        <div class="card">
          <h3>${currency} Symbols</h3>
          ${pieChart}
          ${accountMarkup}
          ${listMarkup}
        </div>
      `;
    }).join('');

    view.innerHTML = `
      <div class="section-title">Charts</div>
    <div class="section-sub">Symbol composition snapshots by currency.</div>
      <div class="grid two">
        ${symbolBlocks || renderEmptyState('No symbol data yet.')}
      </div>
    `;
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

async function renderSettings() {
  view.innerHTML = `
    <div class="section-title">Settings</div>
    <div class="section-sub">Accounts, asset types, allocation ranges, and API connection.</div>
    <div class="card">Loading settings...</div>
  `;

  try {
    const [accounts, assetTypes, allocationSettings, symbols] = await Promise.all([
      fetchJSON('/api/accounts'),
      fetchJSON('/api/asset-types'),
      fetchJSON('/api/allocation-settings'),
      fetchJSON('/api/symbols')
    ]);

    const settingsMap = {};
    allocationSettings.forEach((s) => {
      settingsMap[`${s.currency}_${s.asset_type}`] = s;
    });

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

    const accountsList = accounts.map((a) => `
      <div class="list-item">
        <div>
          <strong>${escapeHtml(a.account_name)}</strong>
          <div class="section-sub">${escapeHtml(a.account_id)}</div>
        </div>
        <button class="btn danger" data-account="${escapeHtml(a.account_id)}">Delete</button>
      </div>
    `).join('');

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

    const assetList = assetTypes.map((a) => `
      <div class="list-item">
        <div><strong>${escapeHtml(a.label)}</strong> <span class="section-sub">${escapeHtml(a.code)}</span></div>
        <button class="btn danger" data-asset="${escapeHtml(a.code)}">Delete</button>
      </div>
    `).join('');

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
      const symbol = escapeHtml(sym.symbol);
      const nameValue = sym.name ? escapeHtml(sym.name) : '';
      const symAsset = sym.asset_type ? String(sym.asset_type).toLowerCase() : '';
      const assetOptions = assetTypes.map((a) => {
        const assetCode = String(a.code).toLowerCase();
        const selected = assetCode === symAsset ? 'selected' : '';
        return `<option value="${escapeHtml(a.code)}" ${selected}>${escapeHtml(a.label)}</option>`;
      }).join('');
      const autoChecked = sym.auto_update ? 'checked' : '';
      return `
        <tr>
          <td><strong>${symbol}</strong></td>
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

    const symbolsSection = `
      <div class="card span-2">
        <h3>Symbols</h3>
        <div class="section-sub">Update display names, asset types, and auto sync status.</div>
        ${symbolRows
          ? `<table class="table">
              <thead>
                <tr>
                  <th>Symbol</th>
                  <th>Name</th>
                  <th>Asset Type</th>
                  <th>Auto</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>${symbolRows}</tbody>
            </table>`
          : '<div class="section-sub">No symbols found yet.</div>'}
      </div>
    `;

    view.innerHTML = `
      <div class="section-title">Settings</div>
      <div class="section-sub">Accounts, asset types, allocation ranges, and API connection.</div>
      <div class="grid two">
        ${apiSection}
        ${accountsSection}
        ${assetSection}
        ${allocationCards}
        ${symbolsSection}
      </div>
    `;

    bindSettingsActions();
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load settings.');
  }
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
