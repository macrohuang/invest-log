async function renderAIAnalysis() {
  view.innerHTML = `
    <div class="section-title">AI Analysis</div>
    <div class="section-sub">Choose a configured method, fill variables, and run a streamed analysis.</div>
    <div class="card">Loading AI analysis methods...</div>
  `;

  try {
    const methods = await loadAIAnalysisMethods({ forceRefresh: true });
    if (!methods.length) {
      view.innerHTML = `
        <div class="section-title">AI Analysis</div>
        <div class="section-sub">Choose a configured method, fill variables, and run a streamed analysis.</div>
        ${renderEmptyState('No analysis methods yet. Create one in Settings > API.', '<a class="primary" href="#/settings">Open Settings</a>')}
      `;
      return;
    }

    let selectedMethod = methods.find((item) => item.id === Number(state.aiAnalysisSelectedMethodId));
    if (!selectedMethod) {
      selectedMethod = methods[0];
      state.aiAnalysisSelectedMethodId = selectedMethod.id;
    }

    const history = await loadAIAnalysisHistory(selectedMethod.id, 12);
    let selectedRun = history.find((item) => item.id === Number(state.aiAnalysisSelectedRunId));
    if (!selectedRun) {
      selectedRun = history[0] || null;
      state.aiAnalysisSelectedRunId = selectedRun ? selectedRun.id : 0;
    }

    const draftValues = getAIAnalysisDraftValues(selectedMethod);
    const variableRows = selectedMethod.variables.length
      ? selectedMethod.variables.map((name) => `
          <div class="field">
            <label>${escapeHtml(name)}</label>
            <input type="text" data-ai-analysis-var="${escapeHtml(name)}" value="${escapeHtml(draftValues[name] || '')}" placeholder="Enter ${escapeHtml(name)}">
          </div>
        `).join('')
      : '<div class="section-sub">This method has no variables. You can run it directly.</div>';

    const methodOptions = methods.map((item) => `
      <option value="${item.id}" ${item.id === selectedMethod.id ? 'selected' : ''}>${escapeHtml(item.name)}</option>
    `).join('');

    view.innerHTML = `
      <div class="section-title">AI Analysis</div>
      <div class="section-sub">Choose a configured method, fill variables, and run a streamed analysis.</div>
      <div class="grid two ai-analysis-layout">
        <div class="card">
          <h3>Run Method</h3>
          <div class="section-sub">Prompts are managed in Settings. Variables come from \${VAR} placeholders.</div>
          <div class="form">
            <div class="form-row">
              <div class="field">
                <label>Method</label>
                <select id="ai-analysis-method-select">${methodOptions}</select>
              </div>
            </div>
            <div class="form-row ai-analysis-variable-grid">
              ${variableRows}
            </div>
            <div class="card-actions">
              <button class="btn primary" id="run-ai-analysis" type="button">Run Analysis</button>
              <a class="btn secondary" href="#/settings" id="open-ai-analysis-settings">Manage Methods</a>
            </div>
          </div>
        </div>
        <div id="ai-analysis-output">
          ${renderAIAnalysisOutput(selectedRun)}
        </div>
      </div>
      <div class="card">
        <h3>History</h3>
        <div class="section-sub">Recent runs for the selected method.</div>
        ${renderAIAnalysisHistory(history)}
      </div>
    `;

    bindAIAnalysisActions(selectedMethod, history);
  } catch (err) {
    view.innerHTML = `
      <div class="section-title">AI Analysis</div>
      <div class="section-sub">Choose a configured method, fill variables, and run a streamed analysis.</div>
      ${renderEmptyState('Unable to load AI analysis methods.')}
    `;
  }
}

function getAIAnalysisDraftValues(method) {
  const methodID = Number(method && method.id ? method.id : 0);
  if (!methodID) {
    return {};
  }
  if (!state.aiAnalysisDraftValuesByMethod[methodID]) {
    state.aiAnalysisDraftValuesByMethod[methodID] = {};
  }
  return state.aiAnalysisDraftValuesByMethod[methodID];
}

function renderAIAnalysisOutput(selectedRun) {
  if (state.aiAnalysisStreaming && state.aiAnalysisStreaming.active) {
    return renderAIAnalysisStreamCard(state.aiAnalysisStreaming);
  }
  if (!selectedRun) {
    return `
      <div class="card">
        <h3>Latest Result</h3>
        <div class="section-sub">No runs yet. Start the first analysis.</div>
      </div>
    `;
  }

  const variableBadges = Object.keys(selectedRun.variables || {}).map((key) => `
    <span class="tag other">${escapeHtml(key)}=${escapeHtml(selectedRun.variables[key] || '')}</span>
  `).join('');

  return `
    <div class="card ai-analysis-result-card">
      <div class="ai-analysis-head">
        <h3>${escapeHtml(selectedRun.methodName || 'AI Analysis Result')}</h3>
        <div class="section-sub">${escapeHtml(formatDateTimeInDisplayTimezone(selectedRun.createdAt))} · ${escapeHtml(selectedRun.model || '—')}</div>
      </div>
      ${variableBadges ? `<div class="ai-analysis-tags">${variableBadges}</div>` : ''}
      <div class="ai-analysis-result-block">
        <div class="section-sub">Result</div>
        <div class="ai-markdown-content">${renderMarkdownLite(selectedRun.resultText || selectedRun.errorMessage || '')}</div>
      </div>
      <div class="ai-analysis-result-block">
        <div class="section-sub">Rendered System Prompt</div>
        <pre class="ai-stream-content">${escapeHtml(selectedRun.renderedSystemPrompt || '—')}</pre>
      </div>
      <div class="ai-analysis-result-block">
        <div class="section-sub">Rendered User Prompt</div>
        <pre class="ai-stream-content">${escapeHtml(selectedRun.renderedUserPrompt || '—')}</pre>
      </div>
    </div>
  `;
}

function renderAIAnalysisStreamCard(streamState) {
  const stage = streamState && streamState.stage ? escapeHtml(String(streamState.stage)) : 'Analyzing...';
  const text = streamState && streamState.text ? String(streamState.text) : '';
  const error = streamState && streamState.error ? escapeHtml(String(streamState.error)) : '';

  return `
    <div class="card ai-stream-card" id="ai-analysis-stream-card">
      <div class="ai-analysis-head">
        <h3>Streaming Result</h3>
        <div class="section-sub">${stage}</div>
      </div>
      ${error ? `<div class="section-sub ai-stream-error">${error}</div>` : ''}
      ${text ? `<div class="ai-markdown-content">${renderMarkdownLite(text)}</div>` : '<div class="section-sub">Waiting for model output...</div>'}
    </div>
  `;
}

function renderAIAnalysisHistory(items) {
  if (!Array.isArray(items) || !items.length) {
    return '<div class="section-sub">No history yet.</div>';
  }

  const rows = items.map((item) => `
    <button class="list-item ai-analysis-history-item ${item.id === Number(state.aiAnalysisSelectedRunId) ? 'active' : ''}" data-ai-analysis-run="${item.id}" type="button">
      <div>
        <strong>${escapeHtml(item.methodName || 'AI Analysis')}</strong>
        <div class="section-sub">${escapeHtml(formatDateTimeInDisplayTimezone(item.createdAt))}</div>
      </div>
      <div>
        <span class="tag other">${escapeHtml(item.status || 'completed')}</span>
      </div>
    </button>
  `).join('');

  return `<div class="list ai-analysis-history-list">${rows}</div>`;
}

function updateAIAnalysisStreamCardInPlace() {
  const container = document.getElementById('ai-analysis-output');
  if (!container) {
    return false;
  }
  container.innerHTML = renderAIAnalysisStreamCard(state.aiAnalysisStreaming);
  return true;
}

function bindAIAnalysisActions(selectedMethod, history) {
  const methodSelect = document.getElementById('ai-analysis-method-select');
  if (methodSelect) {
    methodSelect.addEventListener('change', () => {
      state.aiAnalysisSelectedMethodId = Number(methodSelect.value || 0);
      state.aiAnalysisSelectedRunId = 0;
      renderAIAnalysis();
    });
  }

  selectedMethod.variables.forEach((name) => {
    const input = document.querySelector(`[data-ai-analysis-var="${CSS.escape(name)}"]`);
    if (!input) {
      return;
    }
    input.addEventListener('input', () => {
      const draftValues = getAIAnalysisDraftValues(selectedMethod);
      draftValues[name] = input.value;
    });
  });

  document.querySelectorAll('[data-ai-analysis-run]').forEach((btn) => {
    btn.addEventListener('click', () => {
      state.aiAnalysisSelectedRunId = Number(btn.dataset.aiAnalysisRun || 0);
      renderAIAnalysis();
    });
  });

  const runBtn = document.getElementById('run-ai-analysis');
  if (runBtn) {
    runBtn.addEventListener('click', async () => {
      if (runBtn.disabled) {
        return;
      }

      const settings = await loadAIAnalysisSettings();
      if (!settings.model || !settings.apiKey) {
        localStorage.setItem('activeSettingsTab', 'api');
        window.location.hash = '#/settings';
        showToast('Set AI model and API Key in Settings > API');
        return;
      }

      const variables = {};
      selectedMethod.variables.forEach((name) => {
        const input = document.querySelector(`[data-ai-analysis-var="${CSS.escape(name)}"]`);
        variables[name] = input ? input.value : '';
      });
      state.aiAnalysisDraftValuesByMethod[selectedMethod.id] = { ...variables };

      runBtn.disabled = true;
      state.aiAnalysisStreaming = {
        active: true,
        stage: 'Connecting to AI service...',
        text: '',
        error: '',
      };
      updateAIAnalysisStreamCardInPlace();

      try {
        await postSSE('/api/ai-analysis/stream', {
          method_id: selectedMethod.id,
          variables,
        }, {
          onProgress: (payload) => {
            state.aiAnalysisStreaming.stage = payload && payload.message
              ? String(payload.message)
              : 'Analyzing...';
            updateAIAnalysisStreamCardInPlace();
          },
          onDelta: (payload) => {
            const text = payload && payload.text ? String(payload.text) : '';
            if (!text) {
              return;
            }
            state.aiAnalysisStreaming.text += text;
            updateAIAnalysisStreamCardInPlace();
          },
          onError: (payload) => {
            state.aiAnalysisStreaming.error = payload && payload.error
              ? String(payload.error)
              : 'Analysis failed';
            updateAIAnalysisStreamCardInPlace();
          },
          onResult: (payload) => {
            if (payload && payload.id) {
              state.aiAnalysisSelectedRunId = Number(payload.id || 0);
            }
          },
        });
        showToast('Analysis complete');
      } catch (err) {
        const message = err && err.message ? err.message : 'Analysis failed';
        showToast(message);
      } finally {
        runBtn.disabled = false;
        if (state.aiAnalysisStreaming) {
          state.aiAnalysisStreaming.active = false;
        }
        await renderAIAnalysis();
        state.aiAnalysisStreaming = null;
      }
    });
  }
}
