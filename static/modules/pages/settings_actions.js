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

