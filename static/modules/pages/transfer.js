async function renderTransfer() {
  view.innerHTML = `
    <div class="section-title">Transfer</div>
    <div class="section-sub">Transfer cash or securities between accounts.</div>
    <div class="card">Loading form...</div>
  `;

  try {
    const [accounts, assetTypes, holdingsBySymbol] = await Promise.all([
      fetchJSON('/api/accounts'),
      fetchJSON('/api/asset-types'),
      fetchJSON('/api/holdings-by-symbol')
    ]);

    if (!accounts.length || accounts.length < 2) {
      view.innerHTML = renderEmptyState(
        accounts.length < 2
          ? 'You need at least two accounts for transfers.'
          : 'Create accounts first in Settings.',
        '<a class="primary" href="#/settings">Open Settings</a>'
      );
      return;
    }

    const accountMap = new Map(accounts.map((a) => [a.account_id, a.account_name || a.account_id]));
    const holdings = [];
    Object.entries(holdingsBySymbol || {}).forEach(([currency, data]) => {
      (data.symbols || []).forEach((h) => {
        holdings.push({
          currency,
          symbol: h.symbol,
          displayName: h.display_name || h.symbol,
          accountId: h.account_id,
          assetType: (h.asset_type || '').toLowerCase(),
          totalShares: h.total_shares || 0,
          avgCost: h.avg_cost || 0,
        });
      });
    });
    const today = formatDateInDisplayTimezone();

    const buildAccountOptions = (items) => items.map((a) => `
      <option value="${escapeHtml(a.account_id)}">${escapeHtml(a.account_name || a.account_id)}</option>
    `).join('');

    const buildAssetOptions = (items) => items.map((a) => `
      <option value="${escapeHtml(a.code)}">${escapeHtml(a.label)}</option>
    `).join('');

    view.innerHTML = `
      <div class="section-title">Transfer</div>
      <div class="section-sub">Transfer cash or securities between accounts.</div>
      <div class="card">
        <form id="transfer-form" class="form">
          <div class="form-row">
            <div class="field">
              <label>Date</label>
              <input type="date" name="transaction_date" value="${today}" required>
            </div>
            <div class="field">
              <label>From Currency</label>
              <select name="from_currency" id="tf-from-currency">
                <option value="CNY">CNY</option>
                <option value="USD">USD</option>
                <option value="HKD">HKD</option>
              </select>
            </div>
            <div class="field">
              <label>To Currency</label>
              <select name="to_currency" id="tf-to-currency">
                <option value="">(Same)</option>
                <option value="CNY">CNY</option>
                <option value="USD">USD</option>
                <option value="HKD">HKD</option>
              </select>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>From Account</label>
              <select name="from_account_id" id="tf-from-account" required></select>
            </div>
            <div class="field">
              <label>To Account</label>
              <select name="to_account_id" id="tf-to-account" required></select>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>Asset Type</label>
              <select name="asset_type" id="tf-asset-type"></select>
            </div>
            <div class="field">
              <label>Symbol</label>
              <select name="symbol" id="tf-symbol" required></select>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>Quantity</label>
              <input type="number" step="0.0001" name="quantity" id="tf-quantity" required>
              <small id="tf-quantity-hint" class="section-sub"></small>
            </div>
            <div class="field">
              <label>Commission</label>
              <input type="number" step="0.0001" name="commission" value="0">
            </div>
          </div>
          <div class="field">
            <label>Notes</label>
            <textarea name="notes" rows="2"></textarea>
          </div>
          <div class="actions">
            <button class="btn" type="submit">Execute Transfer</button>
          </div>
        </form>
      </div>
    `;

    const fromCurrency = document.getElementById('tf-from-currency');
    const toCurrency = document.getElementById('tf-to-currency');
    const fromAccount = document.getElementById('tf-from-account');
    const toAccount = document.getElementById('tf-to-account');
    const assetType = document.getElementById('tf-asset-type');
    const symbolSelect = document.getElementById('tf-symbol');
    const quantityInput = document.getElementById('tf-quantity');
    const quantityHint = document.getElementById('tf-quantity-hint');

    function getSourceHoldings() {
      const currency = fromCurrency.value;
      const accountId = fromAccount.value;
      const asset = String(assetType.value || '').toLowerCase();
      return holdings.filter((h) =>
        h.currency === currency &&
        h.accountId === accountId &&
        h.assetType === asset &&
        h.totalShares > 0
      );
    }

    function updateFromAccountOptions() {
      const currency = fromCurrency.value;
      const accountIds = new Set(holdings.filter((h) => h.currency === currency).map((h) => h.accountId));
      let candidates = accountIds.size
        ? accounts.filter((a) => accountIds.has(a.account_id))
        : accounts;
      if (!candidates.length) candidates = accounts;
      const current = fromAccount.value;
      fromAccount.innerHTML = buildAccountOptions(candidates);
      if (current && candidates.some((a) => a.account_id === current)) {
        fromAccount.value = current;
      }
    }

    function updateToAccountOptions() {
      const fromId = fromAccount.value;
      const candidates = accounts.filter((a) => a.account_id !== fromId);
      const current = toAccount.value;
      toAccount.innerHTML = buildAccountOptions(candidates);
      if (current && candidates.some((a) => a.account_id === current)) {
        toAccount.value = current;
      }
    }

    function updateAssetTypeOptions() {
      const currency = fromCurrency.value;
      const accountId = fromAccount.value;
      const typesInHoldings = new Set(
        holdings
          .filter((h) => h.currency === currency && h.accountId === accountId && h.totalShares > 0)
          .map((h) => h.assetType)
          .filter(Boolean)
      );
      let options = [];
      if (typesInHoldings.size) {
        options = assetTypes
          .filter((a) => typesInHoldings.has(String(a.code).toLowerCase()))
          .map((a) => ({ code: a.code, label: a.label }));
      }
      if (!options.length) {
        options = assetTypes.map((a) => ({ code: a.code, label: a.label }));
      }
      const current = assetType.value;
      assetType.innerHTML = buildAssetOptions(options);
      if (current && options.some((opt) => String(opt.code).toLowerCase() === String(current).toLowerCase())) {
        assetType.value = current;
      }
    }

    function updateSymbolOptions() {
      const srcHoldings = getSourceHoldings();
      if (srcHoldings.length) {
        symbolSelect.disabled = false;
        symbolSelect.innerHTML = srcHoldings.map((h) => `
          <option value="${escapeHtml(h.symbol)}">${escapeHtml(h.displayName)} (${escapeHtml(h.symbol)})</option>
        `).join('');
      } else {
        symbolSelect.innerHTML = '<option value="">No holdings</option>';
        symbolSelect.disabled = true;
      }
      updateQuantityHint();
    }

    function updateQuantityHint() {
      const srcHoldings = getSourceHoldings();
      const selected = srcHoldings.find((h) => h.symbol === symbolSelect.value);
      if (selected) {
        quantityInput.max = selected.totalShares;
        quantityInput.disabled = false;
        quantityHint.textContent = `Available: ${formatNumber(selected.totalShares)}`;
      } else {
        quantityInput.removeAttribute('max');
        quantityHint.textContent = '';
        quantityInput.disabled = true;
      }
    }

    function refreshAll() {
      updateFromAccountOptions();
      updateToAccountOptions();
      updateAssetTypeOptions();
      updateSymbolOptions();
    }

    fromCurrency.addEventListener('change', refreshAll);
    toCurrency.addEventListener('change', () => {}); // no cascading needed
    fromAccount.addEventListener('change', () => {
      updateToAccountOptions();
      updateAssetTypeOptions();
      updateSymbolOptions();
    });
    toAccount.addEventListener('change', () => {});
    assetType.addEventListener('change', updateSymbolOptions);
    symbolSelect.addEventListener('change', updateQuantityHint);

    refreshAll();

    const form = document.getElementById('transfer-form');
    form.addEventListener('submit', async (event) => {
      event.preventDefault();
      const formData = new FormData(form);
      const payload = {};
      payload.transaction_date = formData.get('transaction_date');
      payload.symbol = formData.get('symbol');
      payload.quantity = Number(formData.get('quantity'));
      payload.from_account_id = formData.get('from_account_id');
      payload.to_account_id = formData.get('to_account_id');
      payload.from_currency = formData.get('from_currency');
      payload.to_currency = formData.get('to_currency') || formData.get('from_currency');
      payload.commission = Number(formData.get('commission') || 0);
      payload.asset_type = formData.get('asset_type');
      const notes = formData.get('notes');
      if (notes && notes.trim()) {
        payload.notes = notes.trim();
      }

      if (!payload.symbol) {
        showToast('Please select a symbol');
        return;
      }
      if (payload.quantity <= 0) {
        showToast('Quantity must be positive');
        return;
      }
      if (payload.from_account_id === payload.to_account_id) {
        showToast('From and To accounts must be different');
        return;
      }

      const srcHoldings = getSourceHoldings();
      const selected = srcHoldings.find((h) => h.symbol === payload.symbol);
      if (selected && payload.quantity > selected.totalShares) {
        showToast('Quantity exceeds available holdings');
        return;
      }

      try {
        const result = await fetchJSON('/api/transfers', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        let msg = 'Transfer completed';
        if (result.exchange_rate) {
          msg += ` (rate: ${result.exchange_rate.toFixed(4)})`;
        }
        showToast(msg);
        window.location.hash = '#/transactions';
      } catch (err) {
        const errorText = err.message || 'Transfer failed';
        try {
          const parsed = JSON.parse(errorText);
          showToast(parsed.error || errorText);
        } catch {
          showToast(errorText);
        }
      }
    });
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load transfer form.');
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
    exported_at: formatDateTimeISOInDisplayTimezone(),
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

  const stamp = formatDateInDisplayTimezone();
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

