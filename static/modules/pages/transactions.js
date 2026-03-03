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
        if (!await showConfirmModal('Delete this transaction?')) return;
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
      pnl: s.unrealized_pnl ?? null,
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
