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

    // Load saved analysis history in parallel for currencies that have no in-memory result.
    await Promise.all(currencies.map(async (curr) => {
      if (state.aiAnalysisByCurrency[curr]) return;
      try {
        const history = await fetchJSON(`/api/ai/holdings-analysis/history?currency=${encodeURIComponent(curr)}&limit=6`);
        if (Array.isArray(history) && history.length > 0) {
          state.aiAnalysisByCurrency[curr] = history[0];
          state.aiAnalysisHistoryByCurrency[curr] = history.slice(1);
        }
      } catch (_) { /* ignore */ }
    }));

    const tabButtons = currencies.map((currency) => `
      <button class="tab-btn" type="button" data-holdings-tab="${currency}">${currency}</button>
    `).join('');

    const panels = currencies.map((currency) => {
      const currencyData = data[currency] || {};
      const allSymbols = currencyData.symbols || [];

      // Initialize filter state safely
      if (!state.holdingsFilters[currency]) {
        state.holdingsFilters[currency] = { accountIds: [], symbols: [] };
      }
      const activeFilters = state.holdingsFilters[currency];

      // Apply filtering
      const filteredSymbols = allSymbols.filter(s => {
        if ((s.total_shares || 0) <= 0) return false;
        const matchesAccount = !activeFilters.accountIds.length || activeFilters.accountIds.includes(String(s.account_id || ''));
        const matchesSymbol = !activeFilters.symbols.length || activeFilters.symbols.includes(String(s.symbol || ''));
        return matchesAccount && matchesSymbol;
      });

      // Default Sort: Account ASC, Symbol Name ASC
      filteredSymbols.sort((a, b) => {
        const accA = String(a.account_name || a.account_id || '').toLowerCase();
        const accB = String(b.account_name || b.account_id || '').toLowerCase();
        if (accA !== accB) return accA.localeCompare(accB);
        const nameA = String(a.display_name || a.symbol || '').toLowerCase();
        const nameB = String(b.display_name || b.symbol || '').toLowerCase();
        return nameA.localeCompare(nameB);
      });

      const canUpdateAll = allSymbols.some((s) => s.auto_update !== 0);
      const aiResult = state.aiAnalysisByCurrency[currency] || null;
      const streamState = state.aiStreamingByCurrency[currency] || null;
      const totalMarketValue = Number(currencyData.total_market_value ?? 0);
      const totalCost = Number(currencyData.total_cost ?? 0);
      const totalPnL = Number(currencyData.total_pnl ?? (totalMarketValue - totalCost));
      const totalPnLPercent = totalCost > 0 ? (totalPnL / totalCost) * 100 : null;
      const pnlClass = totalPnL >= 0 ? 'pnl-positive' : 'pnl-negative';
      
      const rows = filteredSymbols.map((s) => {
        const pnlRowClass = s.unrealized_pnl !== null && s.unrealized_pnl !== undefined ? (s.unrealized_pnl >= 0 ? 'pnl-positive' : 'pnl-negative') : '';
        const autoUpdate = s.auto_update !== 0;
        const updateDisabled = autoUpdate ? '' : 'disabled title="Auto sync off"';
        const symbolLink = `#/transactions?symbol=${encodeURIComponent(s.symbol || '')}&account=${encodeURIComponent(s.account_id || '')}`;
        const symbolCost = (s.avg_cost || 0) * (s.total_shares || 0);
        const pnlPercent = symbolCost > 0 && s.unrealized_pnl !== null ? (s.unrealized_pnl / symbolCost) * 100 : null;
        const pnlPercentLabel = pnlPercent !== null ? `<span class="pnl-percent">${formatPercent(pnlPercent)}</span>` : '';
        const pnlMarkup = s.unrealized_pnl !== null && s.unrealized_pnl !== undefined
          ? `<div class="pnl-cell"><span class="pnl-value ${pnlRowClass}" data-sensitive>${formatMoneyPlain(s.unrealized_pnl)}</span>${pnlPercentLabel}</div>`
          : '<span class="section-sub">—</span>';
        const accountSort = (s.account_name || s.account_id || '').toString();
        const symbolLabel = (s.symbol || 'Unknown symbol').toString();
        const actionA11yLabel = `${symbolLabel} ${currency} actions`;
        return `
          <tr data-account="${escapeHtml(accountSort.toLowerCase())}" data-market="${s.market_value || 0}" data-pnl="${s.unrealized_pnl || 0}">
            <td><strong>${escapeHtml(s.display_name)}</strong><br><a class="symbol-link section-sub" href="${symbolLink}">${escapeHtml(s.symbol)}</a></td>
            <td><strong>${escapeHtml(s.account_name || s.account_id || '')}</strong><br><span class="section-sub">${escapeHtml(s.account_id || '')}</span></td>
            <td class="num" data-sensitive>${formatNumber(s.total_shares)}</td>
            <td class="num" data-sensitive>${formatMoneyPlain(s.avg_cost)}</td>
            <td class="num" data-sensitive>${s.latest_price !== null ? formatMoneyPlain(s.latest_price) : '—'}</td>
            <td class="num" data-sensitive>${formatMoneyPlain(s.market_value)}</td>
            <td class="num pnl-column">${pnlMarkup}</td>
            <td class="actions-column">
              <div class="actions holdings-actions" role="group" aria-label="${escapeHtml(actionA11yLabel)}">
                <select
                  class="btn trade-select holdings-action-control holdings-action-trade"
                  data-symbol="${escapeHtml(s.symbol)}"
                  data-currency="${currency}"
                  data-account="${escapeHtml(s.account_id || '')}"
                  data-asset-type="${escapeHtml(s.asset_type || '')}"
                  aria-label="Trade action for ${escapeHtml(symbolLabel)} in ${currency}"
                >
                  <option value="">Trade</option>
                  <option value="BUY">Buy</option>
                  <option value="SELL">Sell</option>
                  <option value="DIVIDEND">Dividend</option>
                  <option value="TRANSFER_IN">Transfer In</option>
                  <option value="TRANSFER_OUT">Transfer Out</option>
                </select>
                <button
                  type="button"
                  class="btn secondary holdings-action-control holdings-action-update"
                  data-action="update"
                  data-symbol="${escapeHtml(s.symbol)}"
                  data-currency="${currency}"
                  data-asset-type="${escapeHtml(s.asset_type || '')}"
                  aria-label="Update price for ${escapeHtml(symbolLabel)} in ${currency}"
                  ${updateDisabled}
                >
                  Update
                </button>
                <button
                  type="button"
                  class="btn tertiary holdings-action-control holdings-action-modify"
                  data-action="modify"
                  data-symbol="${escapeHtml(s.symbol)}"
                  data-display-name="${escapeHtml(symbolLabel)}"
                  data-currency="${currency}"
                  data-account="${escapeHtml(s.account_id || '')}"
                  data-account-name="${escapeHtml(s.account_name || s.account_id || '')}"
                  data-asset-type="${escapeHtml(s.asset_type || '')}"
                  data-total-shares="${escapeHtml(String(s.total_shares ?? 0))}"
                  data-avg-cost="${escapeHtml(String(s.avg_cost ?? 0))}"
                  data-latest-price="${escapeHtml(s.latest_price === null || s.latest_price === undefined ? '' : String(s.latest_price))}"
                  aria-label="Modify holding for ${escapeHtml(symbolLabel)} in ${currency}"
                >
                  Modify
                </button>
                <button
                  type="button"
                  class="btn tertiary holdings-action-control holdings-action-ai"
                  data-action="symbol-ai"
                  data-symbol="${escapeHtml(s.symbol)}"
                  data-currency="${currency}"
                  aria-label="Run AI analysis for ${escapeHtml(symbolLabel)} in ${currency}"
                >
                  AI
                </button>
              </div>
            </td>
          </tr>
        `;
      }).join('');

      // Build unique filter options safely
      const accountMap = new Map();
      const symbolMap = new Map();
      allSymbols.forEach(s => {
        accountMap.set(String(s.account_id || ''), String(s.account_name || s.account_id || 'Unknown'));
        symbolMap.set(String(s.symbol || ''), String(s.display_name || s.symbol || 'Unknown'));
      });

      const uniqueAccounts = Array.from(accountMap.entries()).sort((a, b) => a[1].localeCompare(b[1]));
      const uniqueSymbols = Array.from(symbolMap.entries()).sort((a, b) => a[1].localeCompare(b[1]));

      const getFilterIcon = (count) => `
        <span class="filter-trigger ${count > 0 ? 'active' : ''}">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"></polygon></svg>
          ${count > 0 ? `<small style="font-size:9px; margin-left:2px;">${count}</small>` : ''}
        </span>
      `;

      return `
        <div class="tab-panel" data-holdings-panel="${currency}">
          <div class="card holdings-card">
            <div class="panel-head">
              <div class="panel-left">
                <h3>${currency} Holdings</h3>
                <div class="panel-meta">
                  <div class="panel-metric">
                    <span class="section-sub">Market Value</span>
                    <span class="metric-value" data-sensitive>${formatMoney(totalMarketValue, currency)}</span>
                  </div>
                  <div class="panel-metric">
                    <span class="section-sub">Total P&amp;L</span>
                    <div class="metric-value-group">
                      <span class="metric-value ${pnlClass}" data-sensitive>${formatMoney(totalPnL, currency)}</span>
                      ${totalPnLPercent !== null ? `<span class="total-pnl-percent">${formatPercent(totalPnLPercent)}</span>` : ''}
                    </div>
                  </div>
                </div>
              </div>
              <div class="actions">
                <button class="btn secondary" data-action="update-all" data-currency="${currency}" ${canUpdateAll ? '' : 'disabled title="No auto-sync symbols"'}>Update all</button>
                <div class="ai-analyze-group">
                  <select class="btn ai-type-select" data-ai-type-currency="${currency}" title="Analysis type">
                    <option value="adhoc">临时分析</option>
                    <option value="weekly">周报</option>
                    <option value="monthly">月报</option>
                  </select>
                  <button class="btn tertiary" data-action="ai-analyze" data-currency="${currency}">AI</button>
                </div>
              </div>
            </div>
            <table class="table" data-holdings-table>
              <thead>
                <tr>
                  <th style="position:relative;">
                    Symbol ${getFilterIcon(activeFilters.symbols.length)}
                    <div class="filter-popover" data-filter-type="symbol" data-currency="${currency}">
                      <div class="filter-list">
                        ${uniqueSymbols.map(([id, name]) => `
                          <label class="filter-item">
                            <input type="checkbox" value="${escapeHtml(id)}" ${activeFilters.symbols.includes(id) ? 'checked' : ''}>
                            <span>${escapeHtml(name)}</span>
                          </label>
                        `).join('')}
                      </div>
                      <div class="filter-actions">
                        <button class="btn secondary" data-filter-action="clear">Reset</button>
                      </div>
                    </div>
                  </th>
                  <th class="sortable" data-sort="account" style="position:relative;">
                    Account ${getFilterIcon(activeFilters.accountIds.length)}
                    <div class="filter-popover" data-filter-type="account" data-currency="${currency}">
                      <div class="filter-list">
                        ${uniqueAccounts.map(([id, name]) => `
                          <label class="filter-item">
                            <input type="checkbox" value="${escapeHtml(id)}" ${activeFilters.accountIds.includes(id) ? 'checked' : ''}>
                            <span>${escapeHtml(name)}</span>
                          </label>
                        `).join('')}
                      </div>
                      <div class="filter-actions">
                        <button class="btn secondary" data-filter-action="clear">Reset</button>
                      </div>
                    </div>
                  </th>
                  <th class="num">Shares</th>
                  <th class="num">Avg Cost</th>
                  <th class="num">Price</th>
                  <th class="sortable num" data-sort="market">Market Value</th>
                  <th class="sortable num pnl-column" data-sort="pnl">PnL</th>
                  <th class="actions-column">Actions</th>
                </tr>
              </thead>
              <tbody>${rows || '<tr><td colspan="8" style="text-align:center; padding: 20px;">No positions match filters.</td></tr>'}</tbody>
            </table>
            ${renderAIStreamCard(currency)}
            ${renderAIAnalysisCard(aiResult, currency)}
            ${renderAIAnalysisHistory(state.aiAnalysisHistoryByCurrency[currency] || [], currency)}
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
    initHoldingsFilters();
    bindHoldingsActions();
  } catch (err) {
    console.error('renderHoldings failed', err);
    view.innerHTML = renderEmptyState('Unable to load holdings. Check API connection.');
  }
}

function initHoldingsFilters() {
  // Restore open popover state after re-render.
  if (_openPopover) {
    const { filterType, currency } = _openPopover;
    const toRestore = view.querySelector(
      `.filter-popover[data-filter-type="${filterType}"][data-currency="${currency}"]`
    );
    if (toRestore) {
      toRestore.classList.add('show');
    } else {
      _openPopover = null;
    }
  }

  // Toggle popover on trigger click.
  view.querySelectorAll('.filter-trigger').forEach((trigger) => {
    trigger.addEventListener('click', (e) => {
      e.stopPropagation(); // prevent document listener from closing it immediately
      const popover = trigger.nextElementSibling;
      const isAlreadyOpen = popover.classList.contains('show');
      view.querySelectorAll('.filter-popover.show').forEach(p => p.classList.remove('show'));
      if (isAlreadyOpen) {
        _openPopover = null;
      } else {
        popover.classList.add('show');
        _openPopover = { filterType: popover.dataset.filterType, currency: popover.dataset.currency };
      }
    });
  });

  // Live-apply on checkbox change.
  view.querySelectorAll('.filter-popover').forEach((popover) => {
    popover.querySelectorAll('input[type="checkbox"]').forEach((checkbox) => {
      checkbox.addEventListener('change', (e) => {
        e.stopPropagation();
        const currency = popover.dataset.currency;
        const type = popover.dataset.filterType;
        const checked = Array.from(popover.querySelectorAll('input:checked')).map(i => i.value);
        if (!state.holdingsFilters[currency]) {
          state.holdingsFilters[currency] = { accountIds: [], symbols: [] };
        }
        if (type === 'account') {
          state.holdingsFilters[currency].accountIds = checked;
        } else {
          state.holdingsFilters[currency].symbols = checked;
        }
        renderHoldings(); // _openPopover already set from trigger click; will be restored after render
      });
    });
  });

  // Handle Reset button: clear filter and keep popover open.
  view.querySelectorAll('[data-filter-action="clear"]').forEach((btn) => {
    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      const popover = btn.closest('.filter-popover');
      const currency = popover.dataset.currency;
      const type = popover.dataset.filterType;
      if (state.holdingsFilters[currency]) {
        if (type === 'account') {
          state.holdingsFilters[currency].accountIds = [];
        } else {
          state.holdingsFilters[currency].symbols = [];
        }
      }
      renderHoldings(); // _openPopover already set from trigger click; will be restored after render
    });
  });

  // Note: document click listener is registered once in init(), not here.
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
      const displayName = btn.dataset.displayName || symbol;
      const currency = btn.dataset.currency;
      const account = btn.dataset.account || '';
      const accountName = btn.dataset.accountName || account;
      const assetType = btn.dataset.assetType || '';

      if (action === 'symbol-ai') {
        window.location.hash = `#/symbol-analysis?symbol=${encodeURIComponent(symbol)}&currency=${encodeURIComponent(currency)}`;
        return;
      }

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
        if (action === 'modify') {
          const currentShares = Number(btn.dataset.totalShares || 0);
          const currentAvgCost = Number(btn.dataset.avgCost || 0);
          const latestPriceValue = btn.dataset.latestPrice || '';
          const sharesValue = await showPromptModal(
            `Target shares for ${displayName} (${accountName}, ${currency})`,
            String(currentShares),
          );
          if (sharesValue === null) {
            return;
          }
          const avgCostValue = await showPromptModal(
            `Target avg cost for ${displayName} (${currency})`,
            String(currentAvgCost),
          );
          if (avgCostValue === null) {
            return;
          }
          const targetShares = Number(sharesValue);
          const targetAvgCost = Number(avgCostValue);
          if (Number.isNaN(targetShares) || targetShares < 0) {
            showToast('Invalid target shares');
            return;
          }
          if (Number.isNaN(targetAvgCost) || targetAvgCost < 0) {
            showToast('Invalid target avg cost');
            return;
          }
          let manualPrice = null;
          const shouldSetManualPrice = await showConfirmModal(
            `Set manual price for ${displayName} (${currency}) too?`,
          );
          if (shouldSetManualPrice) {
            const manualPriceValue = await showPromptModal(
              `Manual price for ${displayName} (${currency})`,
              latestPriceValue,
            );
            if (manualPriceValue === null) {
              return;
            }
            manualPrice = Number(manualPriceValue);
            if (Number.isNaN(manualPrice)) {
              showToast('Invalid manual price');
              return;
            }
          }

          const holdingsChanged = targetShares !== currentShares || targetAvgCost !== currentAvgCost;
          const manualPriceChanged = manualPrice !== null;
          if (!holdingsChanged && !manualPriceChanged) {
            showToast('No changes to save');
            return;
          }

          if (holdingsChanged) {
            try {
              await fetchJSON('/api/holdings/modify', {
                method: 'POST',
                body: JSON.stringify({
                  symbol,
                  currency,
                  account_id: account,
                  account_name: accountName,
                  asset_type: assetType,
                  target_shares: targetShares,
                  target_avg_cost: targetAvgCost,
                }),
              });
            } catch (err) {
              showToast('Modify holding failed');
              return;
            }
          }

          if (manualPriceChanged) {
            try {
              await fetchJSON('/api/prices/manual', {
                method: 'POST',
                body: JSON.stringify({ symbol, currency, price: manualPrice }),
              });
            } catch (err) {
              showToast('Manual price save failed');
              return;
            }
          }

          if (holdingsChanged && manualPriceChanged) {
            showToast(`${symbol} modified and price saved`);
          } else if (holdingsChanged) {
            showToast(`${symbol} modified`);
          } else {
            showToast(`${symbol} saved`);
          }
          renderHoldings();
          return;
        }
        if (action === 'ai-analyze') {
          const typeSelect = document.querySelector(`[data-ai-type-currency="${currency}"]`);
          const analysisType = typeSelect ? typeSelect.value : 'adhoc';
          btn.disabled = true;
          btn.textContent = 'Analyzing...';
          try {
            const analyzed = await runAIHoldingsAnalysis(currency, analysisType);
            if (analyzed) {
              // Refresh history from server so the new result appears in history.
              try {
                const history = await fetchJSON(`/api/ai/holdings-analysis/history?currency=${encodeURIComponent(currency)}&limit=6`);
                if (Array.isArray(history) && history.length > 0) {
                  state.aiAnalysisByCurrency[currency] = history[0];
                  state.aiAnalysisHistoryByCurrency[currency] = history.slice(1);
                }
              } catch (_) { /* ignore */ }
              showToast(`${currency} analysis ready`);
            }
          } finally {
            btn.disabled = false;
            btn.textContent = 'AI';
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
        } else if (action === 'modify') {
          showToast('Modify holding failed');
        } else {
          showToast('Price update failed');
        }
      }
    });
  });
}

async function runAIHoldingsAnalysis(currency, analysisType) {
  const settings = await loadAIAnalysisSettings();
  const model = (settings.model || '').trim();
  const baseUrl = normalizeAIBaseUrlForModel(settings.baseUrl, model);

  const normalizedSettings = {
    baseUrl,
    model,
    apiKey: (settings.apiKey || '').trim(),
    riskProfile: settings.riskProfile || 'balanced',
    horizon: settings.horizon || 'medium',
    adviceStyle: settings.adviceStyle || 'balanced',
    allowNewSymbols: settings.allowNewSymbols !== false,
    strategyPrompt: (settings.strategyPrompt || '').trim(),
  };

  if (!normalizedSettings.model || !normalizedSettings.apiKey) {
    localStorage.setItem('activeSettingsTab', 'api');
    window.location.hash = '#/settings';
    showToast('Set AI model and API Key in Settings > API');
    return false;
  }

  state.aiStreamingByCurrency[currency] = {
    active: true,
    stage: 'Connecting to AI service...',
    text: '',
    content: '',
    error: '',
  };
  renderHoldings();

  let result = null;
  let streamError = '';
  let pendingRender = null;
  const scheduleRender = () => {
    if (pendingRender !== null) {
      return;
    }
    pendingRender = setTimeout(() => {
      pendingRender = null;
      if ((window.location.hash || '').startsWith('#/holdings')) {
        // Update only the stream card in-place to avoid full page re-render flickering.
        if (!updateAIStreamCardInPlace(currency)) {
          renderHoldings();
        }
      }
    }, 120);
  };

  try {
    await postSSE('/api/ai/holdings-analysis/stream', {
      base_url: normalizedSettings.baseUrl,
      api_key: normalizedSettings.apiKey,
      model: normalizedSettings.model,
      currency,
      risk_profile: normalizedSettings.riskProfile,
      horizon: normalizedSettings.horizon,
      advice_style: normalizedSettings.adviceStyle,
      allow_new_symbols: normalizedSettings.allowNewSymbols,
      strategy_prompt: normalizedSettings.strategyPrompt,
      analysis_type: analysisType || 'adhoc',
    }, {
      onProgress: (payload) => {
        const nextStage = payload && payload.message
          ? String(payload.message)
          : 'Analyzing...';
        const current = state.aiStreamingByCurrency[currency] || {};
        state.aiStreamingByCurrency[currency] = {
          ...current,
          stage: nextStage,
        };
        scheduleRender();
      },
      onDelta: (payload) => {
        const deltaText = payload && (payload.text || payload.delta)
          ? String(payload.text || payload.delta)
          : '';
        if (!deltaText) return;
        const current = state.aiStreamingByCurrency[currency] || {};
        const existingText = current.text
          ? String(current.text)
          : (current.content ? String(current.content) : '');
        const nextText = existingText + deltaText;
        state.aiStreamingByCurrency[currency] = {
          ...current,
          text: nextText,
          content: nextText,
        };
        scheduleRender();
      },
      onResult: (payload) => {
        if (payload) {
          result = payload;
        }
      },
      onDone: (payload) => {
        if (!result && payload && payload.result) {
          result = payload.result;
        }
      },
      onError: (payload) => {
        streamError = payload && payload.error
          ? String(payload.error)
          : 'AI analysis failed';
        const current = state.aiStreamingByCurrency[currency] || {};
        state.aiStreamingByCurrency[currency] = {
          ...current,
          active: false,
          error: streamError,
        };
        scheduleRender();
      },
    });
  } catch (err) {
    streamError = err && err.message ? String(err.message) : 'AI analysis failed';
  } finally {
    if (pendingRender !== null) {
      clearTimeout(pendingRender);
      pendingRender = null;
    }
  }

  if (streamError) {
    const current = state.aiStreamingByCurrency[currency] || {};
    const currentText = current.text
      ? String(current.text)
      : (current.content ? String(current.content) : '');
    state.aiStreamingByCurrency[currency] = {
      active: false,
      stage: 'Streaming ended with error',
      text: currentText,
      content: currentText,
      error: streamError,
    };
    renderHoldings();
    throw new Error(streamError);
  }

  if (!result) {
    const message = 'AI analysis returned no result';
    const current = state.aiStreamingByCurrency[currency] || {};
    const currentText = current.text
      ? String(current.text)
      : (current.content ? String(current.content) : '');
    state.aiStreamingByCurrency[currency] = {
      active: false,
      stage: 'Streaming ended with empty result',
      text: currentText,
      content: currentText,
      error: message,
    };
    renderHoldings();
    throw new Error(message);
  }

  delete state.aiStreamingByCurrency[currency];
  state.aiAnalysisByCurrency[currency] = result;
  return true;
}
