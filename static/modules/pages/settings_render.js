async function renderSettingsPage() {
  view.innerHTML = `
    <div class="section-title">Settings</div>
    <div class="section-sub">Accounts, FX rates, asset types, allocation ranges, storage, and API connection.</div>
    <div class="card">Loading settings...</div>
  `;

  try {
    const [accounts, assetTypes, allocationSettings, exchangeRates, symbols, holdings, storageInfo, aiSettings, aiAnalysisMethods] = await Promise.all([
      fetchJSON('/api/accounts'),
      fetchJSON('/api/asset-types'),
      fetchJSON('/api/allocation-settings'),
      fetchJSON('/api/exchange-rates'),
      fetchJSON('/api/symbols'),
      fetchJSON('/api/holdings'),
      fetchJSON('/api/storage'),
      loadAIAnalysisSettings({ forceRefresh: true }),
      loadAIAnalysisMethods({ forceRefresh: true }),
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

    const aiAnalysisSection = `
      <div class="card">
        <h3>AI Analysis</h3>
        <div class="section-sub">Gemini-only base URL, model, and API key for AI analysis.</div>
        <div class="form">
          <div class="form-row">
            <div class="field">
              <label>AI Base URL</label>
              <input id="ai-base-url" type="text" placeholder="${defaultGeminiBaseURL}" value="${escapeHtml(aiSettings.baseUrl || defaultGeminiBaseURL)}">
            </div>
            <div class="field">
              <label>Model</label>
              <input id="ai-model" type="text" placeholder="gemini-2.5-flash" value="${escapeHtml(aiSettings.model || 'gemini-2.5-flash')}">
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>API Key</label>
              <input id="ai-api-key" type="password" autocomplete="off" placeholder="AIza..." value="${escapeHtml(aiSettings.apiKey || '')}">
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
          </div>
          <div class="form-row">
            <div class="field">
              <label>Strategy Prompt</label>
              <textarea id="ai-strategy-prompt" rows="4" placeholder="例如：优先控制回撤，新增标的仅考虑高现金流蓝筹。">${escapeHtml(aiSettings.strategyPrompt || '')}</textarea>
              <div class="section-sub">Optional. Used as your personal strategy preference in AI analysis.</div>
            </div>
          </div>
          <div class="form-row">
            <div class="actions">
              <button class="btn" id="save-ai-analysis" type="button">Save AI Settings</button>
            </div>
          </div>
        </div>
      </div>
    `;

    const aiAnalysisMethodRows = (aiAnalysisMethods || []).map((method) => `
      <div class="list-item">
        <div>
          <strong>${escapeHtml(method.name)}</strong>
          <div class="section-sub">Updated ${escapeHtml(formatDateTimeInDisplayTimezone(method.updatedAt || method.createdAt))}</div>
          <div class="section-sub">Variables: ${method.variables.length ? method.variables.map((item) => escapeHtml(item)).join(', ') : 'None'}</div>
        </div>
        <div class="card-actions">
          <button class="btn secondary" data-ai-method-edit="${method.id}" type="button">Edit</button>
          <button class="btn danger" data-ai-method-delete="${method.id}" type="button">Delete</button>
        </div>
      </div>
    `).join('');

    const aiAnalysisMethodsSection = `
      <div class="card">
        <h3>Analysis Methods</h3>
        <div class="section-sub">Manage reusable prompt templates for the AI Analysis workspace. Variables are extracted from \${VAR} placeholders.</div>
        <div class="list">${aiAnalysisMethodRows || '<div class="section-sub">No methods yet.</div>'}</div>
        <form id="ai-analysis-method-form" class="form">
          <input id="ai-analysis-method-id" type="hidden">
          <div class="form-row">
            <div class="field">
              <label>Name</label>
              <input id="ai-analysis-method-name" type="text" placeholder="例如：财报拆解" required>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>System Prompt</label>
              <textarea id="ai-analysis-method-system-prompt" rows="5" placeholder="You are an investment analyst for \${SYMBOL}."></textarea>
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>User Prompt</label>
              <textarea id="ai-analysis-method-user-prompt" rows="6" placeholder="Please analyze \${SYMBOL} and answer \${QUESTION}."></textarea>
            </div>
          </div>
          <div class="form-row">
            <div class="actions">
              <button class="btn" id="save-ai-analysis-method" type="submit">Save Method</button>
              <button class="btn secondary" id="reset-ai-analysis-method" type="button">Clear</button>
            </div>
          </div>
        </form>
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
      const updatedAt = item.updated_at
        ? escapeHtml(formatDateTimeInDisplayTimezone(item.updated_at))
        : '—';
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
            </div>
          </div>
        `;
      }).join('');
      return `
        <div class="card">
          <h3>${currency} Allocation Targets</h3>
          <div class="section-sub">Set min/max percentage bands for ${currency}.</div>
          <div class="list">${rows || '<div class="section-sub">No asset types.</div>'}</div>
          <div class="card-actions">
            <button class="btn" data-alloc-save-all data-currency="${currency}">Save ${currency}</button>
          </div>
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
        content: `<div class="alloc-tab-wrap"><div class="alloc-tab-actions"><button class="btn secondary" id="ai-advisor-btn" type="button">✨ AI 建议</button></div><div class="grid three">${allocationCards}</div></div>`,
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
        content: `<div class="grid two">${apiSection}${aiAnalysisSection}${aiAnalysisMethodsSection}</div>`,
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
    bindSettingsActions(assetTypes, aiAnalysisMethods || []);
  } catch (err) {
    view.innerHTML = renderEmptyState('Unable to load settings.');
  }
}
