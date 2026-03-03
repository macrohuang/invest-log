async function renderSymbolAnalysis() {
  const query = getRouteQuery();
  const symbol = (query.get('symbol') || '').trim();
  const currency = (query.get('currency') || '').trim().toUpperCase();

  if (!symbol || !currency) {
    view.innerHTML = renderEmptyState('Missing symbol or currency parameter.');
    return;
  }

  const hasPerplexityKey = !!getPerplexityAPIKey();
  const usePerplexity = hasPerplexityKey && getSymbolAnalysisUsePerplexity();
  const perplexityToggleHTML = hasPerplexityKey
    ? `<button class="btn secondary${usePerplexity ? ' active' : ''}" id="toggle-perplexity-provider" title="切换为 Perplexity sonar-pro 分析">Perplexity: ${usePerplexity ? 'ON' : 'OFF'}</button>`
    : '';

  view.innerHTML = `
    <div class="section-title">${escapeHtml(symbol)} Analysis</div>
    <div class="section-sub">Multi-dimensional AI deep analysis · ${escapeHtml(currency)}</div>
    <div class="symbol-analysis-actions">
      <a href="#/holdings" class="btn secondary">Back to Holdings</a>
      ${perplexityToggleHTML}
      <button class="btn primary" id="run-symbol-analysis">Run Analysis</button>
    </div>
    <div id="symbol-analysis-content">
      <div class="card">Loading latest analysis...</div>
    </div>
  `;

  const toggleBtn = document.getElementById('toggle-perplexity-provider');
  if (toggleBtn) {
    toggleBtn.addEventListener('click', () => {
      const current = getSymbolAnalysisUsePerplexity();
      setSymbolAnalysisUsePerplexity(!current);
      toggleBtn.textContent = `Perplexity: ${!current ? 'ON' : 'OFF'}`;
      toggleBtn.classList.toggle('active', !current);
    });
  }

  const runBtn = document.getElementById('run-symbol-analysis');
  runBtn.addEventListener('click', async () => {
    runBtn.disabled = true;
    runBtn.textContent = 'Analyzing...';
    const contentEl = document.getElementById('symbol-analysis-content');
    const streamState = {
      stage: 'Connecting to AI service...',
      text: '',
      error: '',
    };
    if (contentEl) {
      contentEl.innerHTML = renderSymbolAnalysisStreamingCard(streamState);
    }
    try {
      await runSymbolAnalysis(symbol, currency, {
        onProgress: (payload) => {
          streamState.stage = payload && payload.message
            ? String(payload.message)
            : 'Analyzing...';
          if (contentEl) {
            contentEl.innerHTML = renderSymbolAnalysisStreamingCard(streamState);
          }
        },
        onDelta: (payload) => {
          const text = payload && payload.text ? String(payload.text) : '';
          if (!text) return;
          streamState.text += text;
          if (contentEl) {
            contentEl.innerHTML = renderSymbolAnalysisStreamingCard(streamState);
          }
        },
        onError: (payload) => {
          streamState.error = payload && payload.error
            ? String(payload.error)
            : 'Analysis failed';
          if (contentEl) {
            contentEl.innerHTML = renderSymbolAnalysisStreamingCard(streamState);
          }
        },
      });
      showToast('Analysis complete');
      renderSymbolAnalysis();
    } catch (err) {
      let message = 'Analysis failed';
      if (err && err.message) {
        try {
          const parsed = JSON.parse(err.message);
          if (parsed && parsed.error) message = String(parsed.error);
        } catch (e) {
          message = err.message;
        }
        message = message.split('\n')[0];
        if (message.length > 140) message = message.slice(0, 137) + '...';
      }
      showToast(message);
    } finally {
      runBtn.disabled = false;
      runBtn.textContent = 'Run Analysis';
    }
  });

  // Load latest analysis
  try {
    const latest = await fetchJSON(`/api/ai/symbol-analysis?symbol=${encodeURIComponent(symbol)}&currency=${encodeURIComponent(currency)}`);
    const history = await fetchJSON(`/api/ai/symbol-analysis/history?symbol=${encodeURIComponent(symbol)}&currency=${encodeURIComponent(currency)}&limit=12`);

    const contentEl = document.getElementById('symbol-analysis-content');
    if (!latest) {
      contentEl.innerHTML = '<div class="card"><div class="section-sub">No analysis yet. Click "Run Analysis" to start.</div></div>';
      return;
    }

    contentEl.innerHTML = `
      ${renderReviewModeSection(history || [])}
      ${renderSynthesisCard(latest.synthesis, latest)}
      ${renderDimensionsGrid(latest.dimensions)}
      ${renderSymbolAnalysisHistory(history || [])}
    `;
  } catch (err) {
    const contentEl = document.getElementById('symbol-analysis-content');
    if (contentEl) {
      contentEl.innerHTML = '<div class="card"><div class="section-sub">Failed to load analysis.</div></div>';
    }
  }
}

function renderSymbolAnalysisStreamingCard(streamState) {
  const stage = streamState && streamState.stage
    ? escapeHtml(String(streamState.stage))
    : 'Analyzing...';
  const text = streamState && streamState.text
    ? escapeHtml(String(streamState.text))
    : '';
  const error = streamState && streamState.error
    ? escapeHtml(String(streamState.error))
    : '';

  return `
    <div class="card ai-stream-card symbol-stream-card">
      <div class="ai-analysis-head">
        <h4>AI Streaming <span class="tag other">${error ? 'Failed' : 'Streaming'}</span></h4>
        <div class="section-sub">${stage}</div>
      </div>
      ${error ? `<div class="section-sub ai-stream-error">${error}</div>` : ''}
      ${text ? `<pre class="ai-stream-content">${text}</pre>` : '<div class="section-sub">Waiting for model output...</div>'}
    </div>
  `;
}

async function runSymbolAnalysis(symbol, currency, handlers = {}) {
  const settings = await loadAIAnalysisSettings();

  // Determine provider: Perplexity sonar-pro or main AI
  const perplexityKey = getPerplexityAPIKey();
  const usePerplexity = !!perplexityKey && getSymbolAnalysisUsePerplexity();

  let base_url, model, api_key;
  if (usePerplexity) {
    base_url = defaultPerplexityBaseURL;
    model = defaultPerplexityModel;
    api_key = perplexityKey;
  } else {
    const mainModel = (settings.model || '').trim();
    base_url = normalizeAIBaseUrlForModel(settings.baseUrl, mainModel);
    model = mainModel;
    api_key = (settings.apiKey || '').trim();
  }

  const normalizedSettings = {
    baseUrl: base_url,
    model,
    apiKey: api_key,
    riskProfile: (settings.riskProfile || 'balanced').trim(),
    horizon: (settings.horizon || 'medium').trim(),
    adviceStyle: (settings.adviceStyle || 'balanced').trim(),
    strategyPrompt: (settings.strategyPrompt || '').trim(),
  };

  if (!normalizedSettings.model || !normalizedSettings.apiKey) {
    localStorage.setItem('activeSettingsTab', 'api');
    window.location.hash = '#/settings';
    showToast(usePerplexity ? 'Set Perplexity API Key in Settings' : 'Set AI model and API Key in Settings > API');
    throw new Error('AI settings not configured');
  }

  let result = null;
  let streamError = '';

  await postSSE('/api/ai/symbol-analysis/stream', {
    base_url: normalizedSettings.baseUrl,
    api_key: normalizedSettings.apiKey,
    model: normalizedSettings.model,
    symbol,
    currency,
    risk_profile: normalizedSettings.riskProfile,
    horizon: normalizedSettings.horizon,
    advice_style: normalizedSettings.adviceStyle,
    strategy_prompt: normalizedSettings.strategyPrompt,
  }, {
    onProgress: (payload) => {
      if (handlers.onProgress) handlers.onProgress(payload);
    },
    onDelta: (payload) => {
      if (handlers.onDelta) handlers.onDelta(payload);
    },
    onResult: (payload) => {
      result = payload;
      if (handlers.onResult) handlers.onResult(payload);
    },
    onError: (payload) => {
      streamError = payload && payload.error
        ? String(payload.error)
        : 'Analysis failed';
      if (handlers.onError) handlers.onError(payload);
    },
    onDone: (payload) => {
      if (handlers.onDone) handlers.onDone(payload);
    },
  });

  if (streamError) {
    throw new Error(streamError);
  }
  if (!result) {
    throw new Error('Analysis returned no result');
  }
  return result;
}

function renderSynthesisCard(synthesis, result) {
  if (!synthesis) {
    return '<div class="card synthesis-card"><div class="section-sub">No synthesis available.</div></div>';
  }

  const ratingLabels = {
    strong_buy: 'Strong Buy',
    buy: 'Buy',
    hold: 'Hold',
    reduce: 'Reduce',
    strong_sell: 'Strong Sell',
  };
  const ratingColors = {
    strong_buy: 'rating-strong-buy',
    buy: 'rating-buy',
    hold: 'rating-hold',
    reduce: 'rating-reduce',
    strong_sell: 'rating-strong-sell',
  };

  const ratingLabel = ratingLabels[synthesis.overall_rating] || synthesis.overall_rating || '—';
  const ratingClass = ratingColors[synthesis.overall_rating] || 'rating-hold';
  const model = result?.model ? escapeHtml(String(result.model)) : '—';
  const createdAt = result?.created_at
    ? escapeHtml(formatDateTimeInDisplayTimezone(result.created_at))
    : '—';

  const keyFactors = Array.isArray(synthesis.key_factors)
    ? `<ul class="ai-findings">${synthesis.key_factors.map(f => `<li>${escapeHtml(f)}</li>`).join('')}</ul>`
    : '';
  const riskWarnings = Array.isArray(synthesis.risk_warnings)
    ? `<ul class="ai-findings risk-list">${synthesis.risk_warnings.map(w => `<li>${escapeHtml(w)}</li>`).join('')}</ul>`
    : '';
  const actionItems = Array.isArray(synthesis.action_items)
    ? `<div class="ai-recommendations">${synthesis.action_items.map(item => `
        <div class="ai-rec-item">
          <div class="ai-rec-head">
            <strong>${escapeHtml(item.action || '')}</strong>
            <span class="tag other">${escapeHtml(item.priority || '')}</span>
          </div>
          <div class="section-sub">${escapeHtml(item.rationale || '')}</div>
        </div>
      `).join('')}</div>`
    : '';
  const positionMeta = parsePositionSuggestionMeta(synthesis.position_suggestion);
  const positionMetaMarkup = positionMeta
    ? `
      <div class="position-meta-badges">
        <span class="tag other">Current ${escapeHtml(positionMeta.current)}</span>
        <span class="tag other">Target ${escapeHtml(positionMeta.target)}</span>
        <span class="tag ${positionMeta.delta.startsWith('-') ? 'sell' : 'buy'}">Delta ${escapeHtml(positionMeta.delta)}</span>
        ${positionMeta.status ? `<span class="tag other">${escapeHtml(positionMeta.status)}</span>` : ''}
        ${positionMeta.action ? `<span class="tag other">Action ${escapeHtml(positionMeta.action)}</span>` : ''}
      </div>
      ${positionMeta.execution ? `<div class="section-sub"><strong>Execution:</strong> ${escapeHtml(positionMeta.execution)}</div>` : ''}
    `
    : '';

  return `
    <div class="card synthesis-card">
      <div class="synthesis-header">
        <div class="synthesis-rating">
          <span class="rating-badge ${ratingClass}">${escapeHtml(ratingLabel)}</span>
          <span class="section-sub">Confidence: ${escapeHtml(synthesis.confidence || '—')}</span>
        </div>
        <div class="section-sub">Model: ${model} · ${createdAt}</div>
      </div>
      <div class="ai-summary">
        <div>${escapeHtml(synthesis.overall_summary || '')}</div>
      </div>
      ${Number.isFinite(Number(synthesis.action_probability_percent)) ? `<div class="section-sub"><strong>Action Probability:</strong> ${escapeHtml(String(synthesis.action_probability_percent))}%</div>` : ''}
      ${positionMetaMarkup}
      ${synthesis.position_suggestion && !positionMeta ? `<div class="section-sub"><strong>Position:</strong> ${escapeHtml(synthesis.position_suggestion)}</div>` : ''}
      <div class="ai-section">
        <h5>Key Factors</h5>
        ${keyFactors}
      </div>
      <div class="ai-section">
        <h5>Risk Warnings</h5>
        ${riskWarnings}
      </div>
      <div class="ai-section">
        <h5>Action Items</h5>
        ${actionItems}
      </div>
      ${synthesis.time_horizon_notes ? `<div class="section-sub"><strong>Time Horizon:</strong> ${escapeHtml(synthesis.time_horizon_notes)}</div>` : ''}
      ${synthesis.disclaimer ? `<div class="section-sub disclaimer">${escapeHtml(synthesis.disclaimer)}</div>` : ''}
    </div>
  `;
}

function renderDimensionsGrid(dimensions) {
  const entries = getOrderedDimensionEntries(dimensions);
  if (!entries.length) {
    return `
      <div class="card dimension-card dimension-empty">
        <h4>Framework Analysis</h4>
        <div class="section-sub">No framework results available</div>
      </div>
    `;
  }

  return `
    <div class="symbol-analysis-grid">
      ${entries.map(([dimension, data]) => renderDimensionCard(dimension, getDimensionLabel(dimension), data)).join('')}
    </div>
  `;
}

function getDimensionLabel(dimension) {
  const labelMap = {
    dupont_roic: '杜邦分析 + ROIC拆解',
    capital_cycle: '资本周期框架',
    industry_s_curve: '产业生命周期与S曲线',
    reverse_dcf: '反向DCF',
    dynamic_moat: '动态护城河分析',
    dcf: 'DCF自由现金流折现',
    porter_moat: '波特五力 + 护城河',
    expectations_investing: '预期差框架',
    relative_valuation: '相对估值',
    macro: '宏观经济政策',
    industry: '行业竞争格局',
    company: '公司基本面',
    international: '国际政治经济',
  };
  return labelMap[dimension] || dimension;
}

function getOrderedDimensionEntries(dimensions) {
  if (!dimensions || typeof dimensions !== 'object') {
    return [];
  }

  const preferredOrder = [
    'dupont_roic',
    'capital_cycle',
    'industry_s_curve',
    'reverse_dcf',
    'dynamic_moat',
    'dcf',
    'porter_moat',
    'expectations_investing',
    'relative_valuation',
    'macro',
    'industry',
    'company',
    'international',
  ];

  const entries = [];
  const seen = new Set();
  preferredOrder.forEach((key) => {
    if (dimensions[key]) {
      entries.push([key, dimensions[key]]);
      seen.add(key);
    }
  });

  Object.keys(dimensions)
    .sort()
    .forEach((key) => {
      if (!seen.has(key) && dimensions[key]) {
        entries.push([key, dimensions[key]]);
      }
    });

  return entries;
}

function renderDimensionCard(dimension, label, data) {
  if (!data) {
    return `
      <div class="card dimension-card dimension-empty">
        <h4>${escapeHtml(label)}</h4>
        <div class="section-sub">No data available</div>
      </div>
    `;
  }

  const ratingColors = {
    positive: 'dimension-positive',
    neutral: 'dimension-neutral',
    negative: 'dimension-negative',
  };
  const ratingClass = ratingColors[data.rating] || 'dimension-neutral';

  const keyPoints = Array.isArray(data.key_points)
    ? `<ul class="ai-findings">${data.key_points.map(p => `<li>${escapeHtml(p)}</li>`).join('')}</ul>`
    : '';
  const risks = Array.isArray(data.risks)
    ? `<ul class="ai-findings risk-list">${data.risks.map(r => `<li>${escapeHtml(r)}</li>`).join('')}</ul>`
    : '';
  const opportunities = Array.isArray(data.opportunities)
    ? `<ul class="ai-findings opportunity-list">${data.opportunities.map(o => `<li>${escapeHtml(o)}</li>`).join('')}</ul>`
    : '';
  const suggestion = data.suggestion ? `<div class="section-sub"><strong>Suggestion:</strong> ${escapeHtml(data.suggestion)}</div>` : '';

  return `
    <div class="card dimension-card ${ratingClass}">
      <div class="dimension-header">
        <h4>${escapeHtml(label)}</h4>
        <span class="rating-badge rating-${escapeHtml(data.rating || 'neutral')}">${escapeHtml(data.rating || '—')}</span>
      </div>
      <div class="section-sub">Confidence: ${escapeHtml(data.confidence || '—')}</div>
      <div class="section-sub">${escapeHtml(data.summary || '')}</div>
      ${suggestion}
      ${data.valuation_assessment ? `<div class="section-sub"><strong>Valuation:</strong> ${escapeHtml(data.valuation_assessment)}</div>` : ''}
      <div class="ai-section"><h5>Key Points</h5>${keyPoints}</div>
      <div class="ai-section"><h5>Risks</h5>${risks}</div>
      <div class="ai-section"><h5>Opportunities</h5>${opportunities}</div>
    </div>
  `;
}

function renderSymbolAnalysisHistory(results) {
  if (!Array.isArray(results) || results.length === 0) {
    return '';
  }

  const ratingLabels = {
    strong_buy: 'Strong Buy',
    buy: 'Buy',
    hold: 'Hold',
    reduce: 'Reduce',
    strong_sell: 'Strong Sell',
  };

  const items = results.map(r => {
    const rating = r.synthesis?.overall_rating || '—';
    const ratingLabel = ratingLabels[rating] || rating;
    const date = formatDateTimeInDisplayTimezone(r.created_at);
    const model = r.model || '—';
    return `
      <div class="analysis-history-item">
        <span class="rating-badge rating-${escapeHtml(rating)}">${escapeHtml(ratingLabel)}</span>
        <span class="section-sub">${escapeHtml(model)}</span>
        <span class="section-sub">${escapeHtml(date)}</span>
      </div>
    `;
  }).join('');

  return `
    <div class="card analysis-history">
      <h4>Analysis History</h4>
      ${items}
    </div>
  `;
}

