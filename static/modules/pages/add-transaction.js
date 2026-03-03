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
    const today = formatDateInDisplayTimezone();

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
      const sellMode = typeSelect.value === 'SELL';
      const currency = currencySelect.value;
      const accountId = accountSelect.value;
      let options = assetTypes.map((a) => ({ code: a.code, label: a.label }));
      if (sellMode) {
        const typesInHoldings = new Set(
          holdings
            .filter((h) => h.currency === currency && h.accountId === accountId)
            .map((h) => h.assetType)
            .filter(Boolean)
        );
        if (typesInHoldings.size) {
          const ordered = assetTypes
            .filter((a) => typesInHoldings.has(String(a.code).toLowerCase()))
            .map((a) => ({ code: a.code, label: a.label }));
          const extras = Array.from(typesInHoldings)
            .filter((code) => !assetLabelMap.has(code))
            .map((code) => ({ code, label: code }));
          options = [...ordered, ...extras];
        }
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

    typeSelect.addEventListener('change', () => {
      updateAssetTypeOptions();
      updatePriceLock();
      updateSymbolMode();
    });
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
      updateAssetTypeOptions();
      updatePriceLock();
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

