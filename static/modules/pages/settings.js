async function renderSettings() {
  view.innerHTML = `
    <div class="section-title">Settings</div>
    <div class="section-sub">Accounts, FX rates, asset types, allocation ranges, storage, and API connection.</div>
    <div class="card">Loading settings...</div>
  `;

  try {
    const [accounts, assetTypes, allocationSettings, exchangeRates, symbols, holdings, storageInfo, aiSettings] = await Promise.all([
      fetchJSON('/api/accounts'),
      fetchJSON('/api/asset-types'),
      fetchJSON('/api/allocation-settings'),
      fetchJSON('/api/exchange-rates'),
      fetchJSON('/api/symbols'),
      fetchJSON('/api/holdings'),
      fetchJSON('/api/storage'),
      loadAIAnalysisSettings({ forceRefresh: true }),
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
    bindSettingsActions(assetTypes);
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

function bindSettingsActions(assetTypes) {
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
    saveAIAnalysis.addEventListener('click', async () => {
      if (saveAIAnalysis.disabled) return;
      const baseUrlInput = document.getElementById('ai-base-url');
      const modelInput = document.getElementById('ai-model');
      const apiKeyInput = document.getElementById('ai-api-key');
      const riskProfileInput = document.getElementById('ai-risk-profile');
      const horizonInput = document.getElementById('ai-horizon');
      const adviceStyleInput = document.getElementById('ai-advice-style');
      const allowNewSymbolsInput = document.getElementById('ai-allow-new-symbols');
      const strategyPromptInput = document.getElementById('ai-strategy-prompt');
      const model = modelInput && modelInput.value ? modelInput.value.trim() : '';
      const rawBaseUrl = baseUrlInput && baseUrlInput.value ? baseUrlInput.value.trim() : '';
      const normalizedRawBaseUrl = normalizeAIBaseUrl(rawBaseUrl);
      const normalizedBaseUrl = normalizeAIBaseUrlForModel(rawBaseUrl, model);
      const autoAdjustedGeminiBaseURL = isGeminiModel(model) &&
        normalizedRawBaseUrl.toLowerCase() === legacyOpenAIBaseURL &&
        normalizedBaseUrl === defaultGeminiBaseURL;

      if (baseUrlInput) {
        baseUrlInput.value = normalizedBaseUrl;
      }

      const settings = {
        baseUrl: normalizedBaseUrl,
        model,
        apiKey: apiKeyInput && apiKeyInput.value ? apiKeyInput.value.trim() : '',
        riskProfile: riskProfileInput && riskProfileInput.value ? riskProfileInput.value : 'balanced',
        horizon: horizonInput && horizonInput.value ? horizonInput.value : 'medium',
        adviceStyle: adviceStyleInput && adviceStyleInput.value ? adviceStyleInput.value : 'balanced',
        allowNewSymbols: allowNewSymbolsInput ? !!allowNewSymbolsInput.checked : true,
        strategyPrompt: strategyPromptInput && strategyPromptInput.value ? strategyPromptInput.value.trim() : '',
      };
      saveAIAnalysis.disabled = true;
      try {
        const saved = await saveAIAnalysisSettings(settings);
        if (!saved.model || !saved.apiKey) {
          showToast('Saved. Set model and API key before running analysis');
        } else if (autoAdjustedGeminiBaseURL) {
          showToast('AI settings saved. Gemini base URL auto-adjusted.');
        } else {
          showToast('AI settings saved');
        }
      } catch (err) {
        showToast('Save failed');
      } finally {
        saveAIAnalysis.disabled = false;
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
      if (!await showConfirmModal('Delete this account?')) return;
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

  view.querySelectorAll('button[data-asset]:not([data-alloc-save])').forEach((btn) => {
    btn.addEventListener('click', async () => {
      if (btn.disabled) return;
      const code = btn.dataset.asset;
      if (!await showConfirmModal('Delete this asset type?')) return;
      try {
        await fetchJSON(`/api/asset-types/${code}`, { method: 'DELETE' });
        showToast('Asset type deleted');
        renderSettings();
      } catch (err) {
        showToast('Delete failed');
      }
    });
  });

  view.querySelectorAll('button[data-alloc-save-all]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      const currency = btn.dataset.currency;
      const minInputs = Array.from(view.querySelectorAll(`input[data-alloc-min][data-currency="${currency}"]`));
      if (minInputs.length === 0) return;
      btn.disabled = true;
      const originalText = btn.textContent;
      btn.textContent = 'Saving…';
      try {
        await Promise.all(minInputs.map((minInput) => {
          const asset = minInput.dataset.asset;
          const maxInput = view.querySelector(`input[data-alloc-max][data-currency="${currency}"][data-asset="${asset}"]`);
          return fetchJSON('/api/allocation-settings', {
            method: 'PUT',
            body: JSON.stringify({
              currency,
              asset_type: asset,
              min_percent: Number(minInput.value || 0),
              max_percent: Number(maxInput ? maxInput.value : 100),
            }),
          });
        }));
        showToast(`${currency} allocations saved`);
      } catch (err) {
        showToast('Save failed');
      } finally {
        btn.disabled = false;
        btn.textContent = originalText;
      }
    });
  });

  const aiAdvisorBtn = view.querySelector('#ai-advisor-btn');
  if (aiAdvisorBtn) {
    aiAdvisorBtn.addEventListener('click', () => {
      showAIAdvisorModal(assetTypes);
    });
  }

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

function showAIAdvisorModal(assetTypes) {
  const overlay = document.getElementById('ai-advisor-overlay');
  const stepsEl = document.getElementById('ai-advisor-steps');
  const contentEl = document.getElementById('ai-advisor-step-content');
  const prevBtn = document.getElementById('ai-advisor-prev');
  const nextBtn = document.getElementById('ai-advisor-next');
  const closeBtn = document.getElementById('ai-advisor-close');
  if (!overlay || !contentEl) return;

  let currentStep = 0;
  let adviceResult = null;
  let isLoading = false;
  let adviceStreamState = {
    stage: '',
    text: '',
    error: '',
  };

  const profile = {
    ageRange: '30s',
    experienceLevel: 'intermediate',
    investGoal: 'balanced',
    riskTolerance: 'balanced',
    horizon: 'medium',
    currencies: ['CNY', 'USD', 'HKD'],
    customPrompt: '',
  };

  const stepTitles = ['个人信息', '投资偏好', '配置范围', 'AI 建议'];

  function renderStepIndicator() {
    stepsEl.innerHTML = stepTitles.map((title, i) => {
      const cls = i < currentStep ? 'advisor-step-done' : i === currentStep ? 'advisor-step-active' : 'advisor-step-pending';
      return `<span class="advisor-step ${cls}">${escapeHtml(title)}</span>`;
    }).join('<span class="advisor-step-sep">›</span>');
  }

  function renderStep0() {
    const ageOptions = [['20s', '20-29岁'], ['30s', '30-39岁'], ['40s', '40-49岁'], ['50s', '50-59岁'], ['60plus', '60岁以上']];
    const expOptions = [['beginner', '新手（< 2年）'], ['intermediate', '有一定经验（2-5年）'], ['experienced', '丰富经验（> 5年）']];
    return `
      <div class="ai-advisor-form">
        <div class="field">
          <label>年龄段</label>
          <div class="radio-group">
            ${ageOptions.map(([v, l]) => `
              <label class="radio-label"><input type="radio" name="ageRange" value="${v}" ${profile.ageRange === v ? 'checked' : ''}> ${escapeHtml(l)}</label>
            `).join('')}
          </div>
        </div>
        <div class="field">
          <label>投资经验</label>
          <div class="radio-group">
            ${expOptions.map(([v, l]) => `
              <label class="radio-label"><input type="radio" name="experienceLevel" value="${v}" ${profile.experienceLevel === v ? 'checked' : ''}> ${escapeHtml(l)}</label>
            `).join('')}
          </div>
        </div>
      </div>
    `;
  }

  function renderStep1() {
    const goalOptions = [['preserve', '资产保值（首要避免亏损）'], ['income', '稳定收益（现金流为主）'], ['growth', '资本增值（追求长期回报）'], ['balanced', '均衡（兼顾收益与安全）']];
    const riskOptions = [['conservative', '保守（最大可接受回撤 -10%）'], ['balanced', '均衡（最大可接受回撤 -25%）'], ['aggressive', '激进（最大可接受回撤 -40%+）']];
    const horizonOptions = [['short', '短期（1-3年）'], ['medium', '中期（3-10年）'], ['long', '长期（10年以上）']];
    return `
      <div class="ai-advisor-form">
        <div class="field">
          <label>投资目标</label>
          <div class="radio-group">
            ${goalOptions.map(([v, l]) => `
              <label class="radio-label"><input type="radio" name="investGoal" value="${v}" ${profile.investGoal === v ? 'checked' : ''}> ${escapeHtml(l)}</label>
            `).join('')}
          </div>
        </div>
        <div class="field">
          <label>风险承受能力</label>
          <div class="radio-group">
            ${riskOptions.map(([v, l]) => `
              <label class="radio-label"><input type="radio" name="riskTolerance" value="${v}" ${profile.riskTolerance === v ? 'checked' : ''}> ${escapeHtml(l)}</label>
            `).join('')}
          </div>
        </div>
        <div class="field">
          <label>投资期限</label>
          <div class="radio-group">
            ${horizonOptions.map(([v, l]) => `
              <label class="radio-label"><input type="radio" name="horizon" value="${v}" ${profile.horizon === v ? 'checked' : ''}> ${escapeHtml(l)}</label>
            `).join('')}
          </div>
        </div>
      </div>
    `;
  }

  function renderStep2() {
    const currencyOptions = [['CNY', '人民币 CNY'], ['USD', '美元 USD'], ['HKD', '港币 HKD']];
    return `
      <div class="ai-advisor-form">
        <div class="field">
          <label>需要建议的币种</label>
          <div class="radio-group">
            ${currencyOptions.map(([v, l]) => `
              <label class="radio-label"><input type="checkbox" name="currencies" value="${v}" ${profile.currencies.includes(v) ? 'checked' : ''}> ${escapeHtml(l)}</label>
            `).join('')}
          </div>
        </div>
        <div class="field">
          <label>补充说明（可选）</label>
          <textarea id="ai-advisor-custom" rows="3" placeholder="例如：偏向科技股，希望保留20%以上现金，不考虑加密货币…">${escapeHtml(profile.customPrompt)}</textarea>
          <div class="section-sub">将作为额外偏好传递给 AI，不填则基于标准模型建议。</div>
        </div>
      </div>
    `;
  }

  function renderStep3Loading() {
    const stage = adviceStreamState.stage
      ? escapeHtml(String(adviceStreamState.stage))
      : 'AI 正在分析您的投资画像，生成配置建议…';
    const streamText = adviceStreamState.text
      ? `<pre class="ai-stream-content">${escapeHtml(String(adviceStreamState.text))}</pre>`
      : '';
    const streamError = adviceStreamState.error
      ? `<div class="section-sub ai-stream-error">${escapeHtml(String(adviceStreamState.error))}</div>`
      : '';
    return `
      <div class="ai-advisor-loading">
        <div class="loading-spinner"></div>
        <p>${stage}</p>
        <div class="section-sub">通常需要 10-30 秒，请稍候</div>
        ${streamError}
        ${streamText}
      </div>
    `;
  }

  function renderStep3Results(result) {
    if (!result) {
      return `<div class="section-sub" style="padding:16px">未能获取建议，请检查 AI 配置后重试。</div>`;
    }
    const model = result.model ? escapeHtml(String(result.model)) : '—';
    const summary = result.summary ? escapeHtml(String(result.summary)) : '—';
    const rationale = result.rationale ? escapeHtml(String(result.rationale)) : '';
    const disclaimer = result.disclaimer ? escapeHtml(String(result.disclaimer)) : '仅供参考，不构成投资建议。';
    const allocs = Array.isArray(result.allocations) ? result.allocations : [];

    const groupByCurrency = {};
    allocs.forEach((a) => {
      const cur = String(a.currency || '').toUpperCase();
      if (!groupByCurrency[cur]) groupByCurrency[cur] = [];
      groupByCurrency[cur].push(a);
    });

    const currencyBlocks = Object.keys(groupByCurrency).sort().map((cur) => {
      const items = groupByCurrency[cur].map((a) => `
        <div class="advice-entry">
          <div class="advice-entry-head">
            <span class="advice-label">${escapeHtml(a.label || a.asset_type)}</span>
            <span class="advice-range">${Number(a.min_percent).toFixed(1)}% – ${Number(a.max_percent).toFixed(1)}%</span>
            <button class="btn secondary advice-apply-btn" type="button"
              data-apply-currency="${escapeHtml(String(a.currency))}"
              data-apply-asset="${escapeHtml(String(a.asset_type))}"
              data-apply-min="${Number(a.min_percent)}"
              data-apply-max="${Number(a.max_percent)}">应用</button>
          </div>
          <div class="section-sub">${escapeHtml(String(a.rationale || ''))}</div>
        </div>
      `).join('');
      return `
        <div class="advice-currency-block">
          <div class="advice-currency-header">
            <strong>${escapeHtml(cur)}</strong>
            <button class="btn secondary" type="button" data-apply-all-currency="${escapeHtml(cur)}">全部应用</button>
          </div>
          <div class="advice-entries">${items}</div>
        </div>
      `;
    }).join('');

    return `
      <div class="ai-advisor-results">
        <div class="section-sub advice-meta">模型: ${model}</div>
        <div class="advice-summary"><strong>建议摘要：</strong>${summary}</div>
        ${rationale ? `<div class="advice-rationale section-sub">${rationale}</div>` : ''}
        <div class="advice-list">${currencyBlocks || '<div class="section-sub">无配置建议返回。</div>'}</div>
        <div class="advice-disclaimer section-sub">${disclaimer}</div>
      </div>
    `;
  }

  function collectCurrentStep() {
    if (currentStep === 0) {
      const ageEl = contentEl.querySelector('input[name="ageRange"]:checked');
      const expEl = contentEl.querySelector('input[name="experienceLevel"]:checked');
      if (ageEl) profile.ageRange = ageEl.value;
      if (expEl) profile.experienceLevel = expEl.value;
    } else if (currentStep === 1) {
      const goalEl = contentEl.querySelector('input[name="investGoal"]:checked');
      const riskEl = contentEl.querySelector('input[name="riskTolerance"]:checked');
      const horizEl = contentEl.querySelector('input[name="horizon"]:checked');
      if (goalEl) profile.investGoal = goalEl.value;
      if (riskEl) profile.riskTolerance = riskEl.value;
      if (horizEl) profile.horizon = horizEl.value;
    } else if (currentStep === 2) {
      const checked = Array.from(contentEl.querySelectorAll('input[name="currencies"]:checked')).map((el) => el.value);
      if (checked.length > 0) profile.currencies = checked;
      const customEl = contentEl.querySelector('#ai-advisor-custom');
      if (customEl) profile.customPrompt = customEl.value.trim();
    }
  }

  function applyAdviceEntry(currency, asset, min, max) {
    const minInput = document.querySelector(`input[data-alloc-min][data-currency="${currency}"][data-asset="${asset}"]`);
    const maxInput = document.querySelector(`input[data-alloc-max][data-currency="${currency}"][data-asset="${asset}"]`);
    if (minInput) minInput.value = min;
    if (maxInput) maxInput.value = max;
  }

  function attachResultHandlers() {
    contentEl.querySelectorAll('.advice-apply-btn').forEach((applyBtn) => {
      applyBtn.addEventListener('click', () => {
        const { applyCurrency, applyAsset, applyMin, applyMax } = applyBtn.dataset;
        applyAdviceEntry(applyCurrency, applyAsset, applyMin, applyMax);
        applyBtn.textContent = '已应用';
        applyBtn.disabled = true;
        showToast(`已应用 ${applyCurrency} ${applyAsset} 配置`);
      });
    });
    contentEl.querySelectorAll('button[data-apply-all-currency]').forEach((applyAllBtn) => {
      applyAllBtn.addEventListener('click', () => {
        const currency = applyAllBtn.dataset.applyAllCurrency;
        const entries = Array.isArray(adviceResult && adviceResult.allocations)
          ? adviceResult.allocations.filter((a) => String(a.currency).toUpperCase() === currency)
          : [];
        entries.forEach((a) => applyAdviceEntry(String(a.currency).toUpperCase(), a.asset_type, a.min_percent, a.max_percent));
        applyAllBtn.textContent = '已应用';
        applyAllBtn.disabled = true;
        contentEl.querySelectorAll(`.advice-apply-btn[data-apply-currency="${currency}"]`).forEach((b) => {
          b.textContent = '已应用';
          b.disabled = true;
        });
        showToast(`已应用 ${currency} 全部配置建议`);
      });
    });
  }

  function renderCurrentStep() {
    renderStepIndicator();
    if (currentStep === 0) {
      contentEl.innerHTML = renderStep0();
    } else if (currentStep === 1) {
      contentEl.innerHTML = renderStep1();
    } else if (currentStep === 2) {
      contentEl.innerHTML = renderStep2();
    } else {
      if (isLoading) {
        contentEl.innerHTML = renderStep3Loading();
      } else {
        contentEl.innerHTML = renderStep3Results(adviceResult);
        attachResultHandlers();
      }
    }

    prevBtn.style.display = currentStep === 0 ? 'none' : '';
    if (currentStep < 2) {
      nextBtn.textContent = '下一步';
      nextBtn.disabled = false;
    } else if (currentStep === 2) {
      nextBtn.textContent = '获取 AI 建议';
      nextBtn.disabled = false;
    } else {
      nextBtn.textContent = isLoading ? '分析中…' : '重新咨询';
      nextBtn.disabled = isLoading;
    }
  }

  async function fetchAdvice() {
    const settings = await loadAIAnalysisSettings();
    const model = (settings.model || '').trim();
    const baseUrl = normalizeAIBaseUrlForModel(settings.baseUrl, model);
    const apiKey = (settings.apiKey || '').trim();
    if (!model || !apiKey) {
      closeModal();
      localStorage.setItem('activeSettingsTab', 'api');
      showToast('请先在 Settings > API 配置 AI 模型和 API Key');
      return;
    }
    isLoading = true;
    adviceStreamState = {
      stage: '连接 AI 服务中…',
      text: '',
      error: '',
    };
    renderCurrentStep();
    try {
      let streamError = '';
      adviceResult = null;
      await postSSE('/api/ai/allocation-advice/stream', {
        base_url: baseUrl,
        api_key: apiKey,
        model,
        age_range: profile.ageRange,
        invest_goal: profile.investGoal,
        risk_tolerance: profile.riskTolerance,
        horizon: profile.horizon,
        experience_level: profile.experienceLevel,
        currencies: profile.currencies,
        custom_prompt: profile.customPrompt,
      }, {
        onProgress: (payload) => {
          adviceStreamState.stage = payload && payload.message
            ? String(payload.message)
            : '分析中…';
          renderCurrentStep();
        },
        onDelta: (payload) => {
          const text = payload && payload.text ? String(payload.text) : '';
          if (!text) return;
          adviceStreamState.text = `${adviceStreamState.text || ''}${text}`;
          renderCurrentStep();
        },
        onResult: (payload) => {
          adviceResult = payload || null;
        },
        onError: (payload) => {
          streamError = payload && payload.error
            ? String(payload.error)
            : 'AI 建议获取失败';
          adviceStreamState.error = streamError;
          renderCurrentStep();
        },
      });
      if (streamError) {
        throw new Error(streamError);
      }
      if (!adviceResult) {
        throw new Error('AI 返回为空');
      }
    } catch (err) {
      adviceResult = null;
      adviceStreamState.error = (err && err.message) ? String(err.message) : 'AI 建议获取失败';
      showToast('AI 建议获取失败，请检查 API 配置');
    } finally {
      isLoading = false;
      renderCurrentStep();
    }
  }

  function closeModal() {
    overlay.classList.add('hidden');
    prevBtn.removeEventListener('click', onPrev);
    nextBtn.removeEventListener('click', onNext);
    closeBtn.removeEventListener('click', closeModal);
    document.removeEventListener('keydown', onKeydown);
  }

  function onPrev() {
    if (currentStep > 0 && currentStep < 3) {
      currentStep -= 1;
      renderCurrentStep();
    }
  }

  async function onNext() {
    if (currentStep < 2) {
      collectCurrentStep();
      currentStep += 1;
      renderCurrentStep();
    } else if (currentStep === 2) {
      collectCurrentStep();
      currentStep = 3;
      await fetchAdvice();
    } else if (!isLoading) {
      currentStep = 0;
      adviceResult = null;
      adviceStreamState = { stage: '', text: '', error: '' };
      renderCurrentStep();
    }
  }

  function onKeydown(e) {
    if (e.key === 'Escape') closeModal();
  }

  prevBtn.addEventListener('click', onPrev);
  nextBtn.addEventListener('click', onNext);
  closeBtn.addEventListener('click', closeModal);
  document.addEventListener('keydown', onKeydown);

  currentStep = 0;
  adviceResult = null;
  adviceStreamState = { stage: '', text: '', error: '' };
  isLoading = false;
  overlay.classList.remove('hidden');
  renderCurrentStep();
}
