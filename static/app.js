const state = {
  apiBase: '',
  privacy: false,
  aiAnalysisByCurrency: {},
  aiAnalysisHistoryByCurrency: {}, // { currency: HoldingsAnalysisResult[] }
  holdingsFilters: {}, // { currency: { accountIds: [], symbols: [] } }
};

// Tracks which filter popover is open across re-renders: { filterType, currency } | null
let _openPopover = null;

const aiAnalysisSettingsKey = 'aiHoldingsAnalysisSettings';

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

const displayTimeZone = 'Asia/Shanghai';
const displayDateFormatter = new Intl.DateTimeFormat('en-CA', {
  timeZone: displayTimeZone,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
});
const displayDateTimeFormatter = new Intl.DateTimeFormat('en-CA', {
  timeZone: displayTimeZone,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
});

function extractDateParts(formatter, value) {
  const parts = {};
  formatter.formatToParts(value).forEach((part) => {
    if (part.type !== 'literal') {
      parts[part.type] = part.value;
    }
  });
  return parts;
}

function parseTimestampAsDate(value) {
  if (value === null || value === undefined) {
    return null;
  }

  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value;
  }

  const text = String(value).trim();
  if (!text) {
    return null;
  }

  if (/^\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}$/.test(text)) {
    const parsedUTC = new Date(text.replace(' ', 'T') + 'Z');
    return Number.isNaN(parsedUTC.getTime()) ? null : parsedUTC;
  }

  if (/^\d{4}-\d{2}-\d{2}$/.test(text)) {
    const parsedDate = new Date(`${text}T00:00:00Z`);
    return Number.isNaN(parsedDate.getTime()) ? null : parsedDate;
  }

  const parsed = new Date(text);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function formatDateInDisplayTimezone(value = new Date()) {
  const date = parseTimestampAsDate(value);
  if (!date) {
    return '';
  }
  const parts = extractDateParts(displayDateFormatter, date);
  return `${parts.year}-${parts.month}-${parts.day}`;
}

function formatDateTimeInDisplayTimezone(value) {
  const date = parseTimestampAsDate(value);
  if (!date) {
    return value ? String(value) : '—';
  }
  const parts = extractDateParts(displayDateTimeFormatter, date);
  return `${parts.year}-${parts.month}-${parts.day} ${parts.hour}:${parts.minute}:${parts.second}`;
}

function formatDateTimeISOInDisplayTimezone(value = new Date()) {
  const date = parseTimestampAsDate(value);
  if (!date) {
    return '';
  }
  const parts = extractDateParts(displayDateTimeFormatter, date);
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}:${parts.second}+08:00`;
}

function init() {
  state.apiBase = resolveApiBase();
  state.privacy = localStorage.getItem('privacyMode') === '1';
  document.body.classList.toggle('privacy', state.privacy);

  privacyToggle.addEventListener('click', () => {
    state.privacy = !state.privacy;
    document.body.classList.toggle('privacy', state.privacy);
    localStorage.setItem('privacyMode', state.privacy ? '1' : '0');
  });

  // Persistent outside-click handler: closes open filter popovers across re-renders.
  document.addEventListener('click', () => {
    if (!_openPopover) return;
    view.querySelectorAll('.filter-popover.show').forEach(p => p.classList.remove('show'));
    _openPopover = null;
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

function getRouteQuery() {
  const hash = window.location.hash || '';
  const queryIndex = hash.indexOf('?');
  if (queryIndex === -1) {
    return new URLSearchParams();
  }
  return new URLSearchParams(hash.slice(queryIndex + 1));
}

function loadAIAnalysisSettings() {
  try {
    const raw = localStorage.getItem(aiAnalysisSettingsKey);
    if (!raw) {
      return {
        baseUrl: 'https://api.openai.com/v1',
        model: '',
        apiKey: '',
        riskProfile: 'balanced',
        horizon: 'medium',
        adviceStyle: 'balanced',
        allowNewSymbols: true,
        strategyPrompt: '',
      };
    }
    const parsed = JSON.parse(raw);
    return {
      baseUrl: parsed.baseUrl || 'https://api.openai.com/v1',
      model: parsed.model || '',
      apiKey: parsed.apiKey || '',
      riskProfile: parsed.riskProfile || 'balanced',
      horizon: parsed.horizon || 'medium',
      adviceStyle: parsed.adviceStyle || 'balanced',
      allowNewSymbols: parsed.allowNewSymbols !== false,
      strategyPrompt: parsed.strategyPrompt || '',
    };
  } catch (err) {
    return {
      baseUrl: 'https://api.openai.com/v1',
      model: '',
      apiKey: '',
      riskProfile: 'balanced',
      horizon: 'medium',
      adviceStyle: 'balanced',
      allowNewSymbols: true,
      strategyPrompt: '',
    };
  }
}

function saveAIAnalysisSettings(settings) {
  localStorage.setItem(aiAnalysisSettingsKey, JSON.stringify(settings));
}

function formatActionLabel(action) {
  const normalized = String(action || '').toLowerCase();
  if (normalized === 'increase') return 'Increase';
  if (normalized === 'reduce') return 'Reduce';
  if (normalized === 'add') return 'Add';
  return 'Hold';
}

function parsePositionSuggestionMeta(positionSuggestion) {
  const text = String(positionSuggestion || '').trim();
  if (!text) {
    return null;
  }

  const currentMatch = text.match(/当前占比\s*([+-]?\d+(?:\.\d+)?)%/);
  const targetMatch = text.match(/目标区间\s*([+-]?\d+(?:\.\d+)?)%\s*-\s*([+-]?\d+(?:\.\d+)?)%/);
  const deltaMatch = text.match(/差值\s*([+-]?\d+(?:\.\d+)?)%(?:（([^）]+)）)?/);
  const actionMatch = text.match(/动作[:：]\s*([^；。]+)/);
  const executionMatch = text.match(/执行[:：]\s*([^；。]+)/);

  if (!currentMatch || !targetMatch || !deltaMatch) {
    return null;
  }

  return {
    current: `${currentMatch[1]}%`,
    target: `${targetMatch[1]}%-${targetMatch[2]}%`,
    delta: `${deltaMatch[1]}%`,
    status: (deltaMatch[2] || '').trim(),
    action: (actionMatch && actionMatch[1] ? actionMatch[1].trim() : ''),
    execution: (executionMatch && executionMatch[1] ? executionMatch[1].trim() : ''),
  };
}

function parseAnalysisTimestamp(value) {
  const date = parseTimestampAsDate(value);
  return date ? date.getTime() : null;
}

function parseReviewNumber(value) {
  const n = Number(value);
  return Number.isFinite(n) ? n : null;
}

function normalizeRiskWarnings(synthesis) {
  if (!synthesis || !Array.isArray(synthesis.risk_warnings)) {
    return [];
  }
  return synthesis.risk_warnings
    .map((item) => String(item || '').trim())
    .filter(Boolean);
}

function pickBaselineAnalysis(sortedResults, latestTs, windowDays) {
  if (!Array.isArray(sortedResults) || sortedResults.length < 2 || !Number.isFinite(latestTs)) {
    return sortedResults && sortedResults.length > 1 ? sortedResults[1] : null;
  }

  const targetTs = latestTs - windowDays * 24 * 60 * 60 * 1000;
  let candidate = null;

  for (let i = 1; i < sortedResults.length; i += 1) {
    const ts = parseAnalysisTimestamp(sortedResults[i].created_at);
    if (ts === null) {
      continue;
    }
    if (ts <= targetTs && (!candidate || ts > candidate.ts)) {
      candidate = { ts, item: sortedResults[i] };
    }
  }

  if (candidate) {
    return candidate.item;
  }
  return sortedResults[1] || null;
}

function computeReviewSnapshot(results, windowDays) {
  if (!Array.isArray(results) || results.length < 2) {
    return null;
  }

  const sorted = [...results].sort((a, b) => {
    const aTs = parseAnalysisTimestamp(a.created_at) || 0;
    const bTs = parseAnalysisTimestamp(b.created_at) || 0;
    return bTs - aTs;
  });

  const latest = sorted[0];
  const latestTs = parseAnalysisTimestamp(latest.created_at);
  const baseline = pickBaselineAnalysis(sorted, latestTs || 0, windowDays);
  if (!baseline) {
    return null;
  }

  const latestSynthesis = latest.synthesis || {};
  const baselineSynthesis = baseline.synthesis || {};

  const probabilityNow = parseReviewNumber(latestSynthesis.action_probability_percent);
  const probabilityPrev = parseReviewNumber(baselineSynthesis.action_probability_percent);
  const probabilityDelta = probabilityNow !== null && probabilityPrev !== null
    ? Number((probabilityNow - probabilityPrev).toFixed(2))
    : null;

  const positionNow = parsePositionSuggestionMeta(latestSynthesis.position_suggestion);
  const positionPrev = parsePositionSuggestionMeta(baselineSynthesis.position_suggestion);
  const offsetNow = positionNow ? parseReviewNumber(positionNow.delta) : null;
  const offsetPrev = positionPrev ? parseReviewNumber(positionPrev.delta) : null;
  const offsetDelta = offsetNow !== null && offsetPrev !== null
    ? Number((offsetNow - offsetPrev).toFixed(2))
    : null;

  const riskNow = normalizeRiskWarnings(latestSynthesis);
  const riskPrev = normalizeRiskWarnings(baselineSynthesis);
  const riskNowSet = new Set(riskNow.map((item) => item.toLowerCase()));
  const riskPrevSet = new Set(riskPrev.map((item) => item.toLowerCase()));

  const newRisks = riskNow.filter((item) => !riskPrevSet.has(item.toLowerCase()));
  const clearedRisks = riskPrev.filter((item) => !riskNowSet.has(item.toLowerCase()));

  return {
    latest,
    baseline,
    probabilityNow,
    probabilityPrev,
    probabilityDelta,
    actionNow: String(latestSynthesis.target_action || '').trim() || '—',
    actionPrev: String(baselineSynthesis.target_action || '').trim() || '—',
    offsetNow,
    offsetPrev,
    offsetDelta,
    newRisks,
    clearedRisks,
  };
}

function formatReviewSigned(value, suffix, digits = 0) {
  if (value === null || !Number.isFinite(value)) {
    return '—';
  }
  const sign = value > 0 ? '+' : '';
  return `${sign}${value.toFixed(digits)}${suffix}`;
}

function renderReviewCard(results, title, windowDays) {
  const snapshot = computeReviewSnapshot(results, windowDays);
  if (!snapshot) {
    return `
      <div class="card review-mode-card">
        <div class="review-mode-header">
          <h4>${escapeHtml(title)}</h4>
          <span class="section-sub">Need 2+ analyses</span>
        </div>
        <div class="section-sub">Run another analysis to unlock trend comparison.</div>
      </div>
    `;
  }

  const probabilityTone = snapshot.probabilityDelta === null
    ? ''
    : (snapshot.probabilityDelta >= 0 ? 'review-up' : 'review-down');
  const offsetTone = snapshot.offsetDelta === null
    ? ''
    : (snapshot.offsetDelta >= 0 ? 'review-up' : 'review-down');

  const newRiskMarkup = snapshot.newRisks.length
    ? `<ul class="review-risk-list">${snapshot.newRisks.slice(0, 3).map((item) => `<li>+ ${escapeHtml(item)}</li>`).join('')}</ul>`
    : '<div class="section-sub">No new risk flag.</div>';

  const clearedRiskMarkup = snapshot.clearedRisks.length
    ? `<ul class="review-risk-list review-cleared">${snapshot.clearedRisks.slice(0, 3).map((item) => `<li>- ${escapeHtml(item)}</li>`).join('')}</ul>`
    : '<div class="section-sub">No cleared risk.</div>';

  return `
    <div class="card review-mode-card">
      <div class="review-mode-header">
        <h4>${escapeHtml(title)}</h4>
        <span class="section-sub">${escapeHtml(formatDateTimeInDisplayTimezone(snapshot.baseline.created_at))} → ${escapeHtml(formatDateTimeInDisplayTimezone(snapshot.latest.created_at))}</span>
      </div>
      <div class="review-row">
        <span class="review-label">Probability</span>
        <span class="review-value ${probabilityTone}">${escapeHtml(snapshot.probabilityNow !== null ? `${snapshot.probabilityNow.toFixed(0)}%` : '—')} (${escapeHtml(formatReviewSigned(snapshot.probabilityDelta, 'pp', 0))})</span>
      </div>
      <div class="review-row">
        <span class="review-label">Action</span>
        <span class="review-value">${escapeHtml(snapshot.actionNow)} (${escapeHtml(snapshot.actionPrev)} → ${escapeHtml(snapshot.actionNow)})</span>
      </div>
      <div class="review-row">
        <span class="review-label">Position Offset</span>
        <span class="review-value ${offsetTone}">${escapeHtml(snapshot.offsetNow !== null ? `${snapshot.offsetNow.toFixed(2)}%` : '—')} (${escapeHtml(formatReviewSigned(snapshot.offsetDelta, 'pp', 2))})</span>
      </div>
      <div class="review-split">
        <div>
          <div class="review-label">New Risks</div>
          ${newRiskMarkup}
        </div>
        <div>
          <div class="review-label">Cleared Risks</div>
          ${clearedRiskMarkup}
        </div>
      </div>
    </div>
  `;
}

function renderReviewModeSection(results) {
  if (!Array.isArray(results) || results.length === 0) {
    return '';
  }
  return `
    <div class="review-mode-section">
      ${renderReviewCard(results, 'Weekly Review', 7)}
      ${renderReviewCard(results, 'Monthly Review', 30)}
    </div>
  `;
}

function renderAIAnalysisCard(result, currency) {
  if (!result) {
    return '';
  }
  const findings = Array.isArray(result.key_findings) ? result.key_findings : [];
  const recommendations = Array.isArray(result.recommendations) ? result.recommendations : [];
  const findingsMarkup = findings.length
    ? `<ul class="ai-findings">${findings.map((item) => `<li>${escapeHtml(item)}</li>`).join('')}</ul>`
    : '<div class="section-sub">No key findings.</div>';
  const recommendationMarkup = recommendations.length
    ? `<div class="ai-recommendations">${recommendations.map((item) => {
      const symbol = item.symbol ? `<strong>${escapeHtml(item.symbol)}</strong>` : '<strong>Portfolio</strong>';
      const action = formatActionLabel(item.action);
      const theory = escapeHtml(item.theory_tag || 'N/A');
      const rationale = escapeHtml(item.rationale || 'No rationale');
      const targetWeight = item.target_weight ? `<span class="section-sub">Target: ${escapeHtml(item.target_weight)}</span>` : '';
      const priority = item.priority ? `<span class="section-sub">Priority: ${escapeHtml(item.priority)}</span>` : '';
      return `
        <div class="ai-rec-item">
          <div class="ai-rec-head">
            ${symbol}
            <span class="tag other">${escapeHtml(action)}</span>
            <span class="tag other">${theory}</span>
          </div>
          <div class="section-sub">${rationale}</div>
          <div class="ai-rec-meta">${targetWeight}${priority}</div>
        </div>
      `;
    }).join('')}</div>`
    : '<div class="section-sub">No recommendations returned.</div>';

  const generatedAt = result.generated_at
    ? escapeHtml(formatDateTimeInDisplayTimezone(result.generated_at))
    : '—';
  const model = result.model ? escapeHtml(String(result.model)) : '—';
  const riskLevel = result.risk_level ? escapeHtml(String(result.risk_level)) : 'unknown';
  const summary = result.overall_summary ? escapeHtml(String(result.overall_summary)) : '—';
  const disclaimer = result.disclaimer ? escapeHtml(String(result.disclaimer)) : 'For reference only.';
  const typeLabel = formatAnalysisTypeLabel(result.analysis_type);
  const symbolRefsCount = Array.isArray(result.symbol_refs) ? result.symbol_refs.length : 0;
  const symbolRefsBadge = symbolRefsCount > 0
    ? `<span class="tag other" title="引用了 ${symbolRefsCount} 个标的深度分析">含${symbolRefsCount}标的分析</span>`
    : '';

  return `
    <div class="card ai-analysis-card" data-ai-analysis-card="${currency}">
      <div class="ai-analysis-head">
        <h4>AI Analysis <span class="tag other">${typeLabel}</span>${symbolRefsBadge}</h4>
        <div class="section-sub">Model: ${model} · Generated: ${generatedAt}</div>
      </div>
      <div class="ai-summary">
        <div><strong>Risk Level:</strong> ${riskLevel}</div>
        <div class="section-sub">${summary}</div>
      </div>
      <div class="ai-section">
        <h5>Key Findings</h5>
        ${findingsMarkup}
      </div>
      <div class="ai-section">
        <h5>Recommendations</h5>
        ${recommendationMarkup}
      </div>
      <div class="section-sub">${disclaimer}</div>
    </div>
  `;
}

function formatAnalysisTypeLabel(type) {
  if (type === 'weekly') return '周报';
  if (type === 'monthly') return '月报';
  return '临时分析';
}

function renderAIAnalysisHistory(history, currency) {
  if (!Array.isArray(history) || history.length === 0) return '';

  const items = history.map((result) => {
    const generatedAt = result.generated_at
      ? escapeHtml(formatDateTimeInDisplayTimezone(result.generated_at))
      : '—';
    const typeLabel = formatAnalysisTypeLabel(result.analysis_type);
    const riskLevel = result.risk_level ? escapeHtml(String(result.risk_level)) : 'unknown';
    const summary = result.overall_summary ? escapeHtml(String(result.overall_summary)) : '—';
    const model = result.model ? escapeHtml(String(result.model)) : '—';
    const id = result.id || 0;
    const symbolRefsCount = Array.isArray(result.symbol_refs) ? result.symbol_refs.length : 0;
    const symbolRefsBadge = symbolRefsCount > 0
      ? `<span class="tag other">含${symbolRefsCount}标的</span>`
      : '';
    return `
      <details class="ai-history-item" data-history-id="${id}">
        <summary class="ai-history-summary">
          <span class="tag other">${typeLabel}</span>${symbolRefsBadge}
          <span class="section-sub">${generatedAt}</span>
          <span class="ai-history-risk">风险: ${riskLevel}</span>
        </summary>
        <div class="ai-history-body section-sub">${summary}</div>
        <div class="section-sub" style="margin-top:4px">Model: ${model}</div>
      </details>
    `;
  }).join('');

  return `
    <div class="ai-history-section">
      <div class="ai-history-title section-sub">历史分析记录</div>
      ${items}
    </div>
  `;
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
    case 'transfer':
      setActiveRoute('transactions');
      renderTransfer();
      break;
    case 'settings':
      setActiveRoute('settings');
      renderSettings();
      break;
    case 'symbol-analysis':
      setActiveRoute('holdings');
      renderSymbolAnalysis();
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

/**
 * Custom prompt modal to replace window.prompt(), which is blocked in WKWebView.
 * Returns a Promise that resolves with the entered string, or null if cancelled.
 */
function showPromptModal(label) {
  return new Promise((resolve) => {
    const overlay = document.getElementById('prompt-overlay');
    const labelEl = document.getElementById('prompt-label');
    const input = document.getElementById('prompt-input');
    const okBtn = document.getElementById('prompt-ok');
    const cancelBtn = document.getElementById('prompt-cancel');

    labelEl.textContent = label;
    input.value = '';
    overlay.classList.remove('hidden');
    input.focus();

    function cleanup() {
      overlay.classList.add('hidden');
      okBtn.removeEventListener('click', onOk);
      cancelBtn.removeEventListener('click', onCancel);
      input.removeEventListener('keydown', onKeydown);
    }

    function onOk() {
      cleanup();
      resolve(input.value.trim() || null);
    }

    function onCancel() {
      cleanup();
      resolve(null);
    }

    function onKeydown(e) {
      if (e.key === 'Enter') onOk();
      if (e.key === 'Escape') onCancel();
    }

    okBtn.addEventListener('click', onOk);
    cancelBtn.addEventListener('click', onCancel);
    input.addEventListener('keydown', onKeydown);
  });
}

/**
 * Custom confirm modal to replace window.confirm(), which is blocked in WKWebView.
 * Returns a Promise that resolves with true (confirmed) or false (cancelled).
 */
function showConfirmModal(message) {
  return new Promise((resolve) => {
    const overlay = document.getElementById('confirm-overlay');
    const messageEl = document.getElementById('confirm-message');
    const okBtn = document.getElementById('confirm-ok');
    const cancelBtn = document.getElementById('confirm-cancel');

    messageEl.textContent = message;
    overlay.classList.remove('hidden');

    function cleanup() {
      overlay.classList.add('hidden');
      okBtn.removeEventListener('click', onOk);
      cancelBtn.removeEventListener('click', onCancel);
      document.removeEventListener('keydown', onKeydown);
    }

    function onOk() {
      cleanup();
      resolve(true);
    }

    function onCancel() {
      cleanup();
      resolve(false);
    }

    function onKeydown(e) {
      if (e.key === 'Enter') onOk();
      if (e.key === 'Escape') onCancel();
    }

    okBtn.addEventListener('click', onOk);
    cancelBtn.addEventListener('click', onCancel);
    document.addEventListener('keydown', onKeydown);
  });
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
      maximumFractionDigits: 4,
    }).format(value);
  } catch (err) {
    return `${symbol}${value.toFixed(4)}`;
  }
}

function formatMoneyPlain(value) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'decimal',
      minimumFractionDigits: 4,
      maximumFractionDigits: 4,
    }).format(value);
  } catch (err) {
    return Number(value).toFixed(4);
  }
}

function formatNumber(value) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return '—';
  }
  try {
    return new Intl.NumberFormat('en-US', {
      style: 'decimal',
      minimumFractionDigits: 4,
      maximumFractionDigits: 4,
    }).format(value);
  } catch (err) {
    return Number(value).toFixed(4);
  }
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

function escapeRegExp(value) {
  return String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function highlightMatchText(text, keyword) {
  const source = String(text || '');
  const needle = String(keyword || '').trim();
  if (!needle) {
    return escapeHtml(source);
  }
  const regex = new RegExp(escapeRegExp(needle), 'ig');
  let lastIndex = 0;
  let output = '';
  let matched = false;
  let hit = regex.exec(source);
  while (hit) {
    matched = true;
    const start = hit.index;
    const end = start + hit[0].length;
    output += escapeHtml(source.slice(lastIndex, start));
    output += `<mark>${escapeHtml(source.slice(start, end))}</mark>`;
    lastIndex = end;
    hit = regex.exec(source);
  }
  if (!matched) {
    return escapeHtml(source);
  }
  output += escapeHtml(source.slice(lastIndex));
  return output;
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

function renderAccountPieChart({ items }) {
  const data = buildPieData(items);
  if (!data) {
    return '<div class="section-sub">No account data</div>';
  }
  const legend = data.segments.map((seg) => `
    <div class="legend-item">
      <span class="legend-swatch" style="background:${seg.color};"></span>
      <div class="legend-label">
        <span>${escapeHtml(seg.label)}</span>
        <span class="legend-percent" style="color:${seg.color};">${formatPercent(seg.percent)}</span>
      </div>
    </div>
  `).join('');

  return `
    <div class="pie-layout account-pie">
      <div class="pie-chart" style="background: conic-gradient(${data.gradient});"></div>
      <div class="pie-legend account-legend">${legend}</div>
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
    pnl: s.unrealized_pnl ?? null,
    cost: (s.avg_cost || 0) * (s.total_shares || 0),
  }));
  if (rest.length) {
    const otherValue = rest.reduce((sum, s) => sum + s.market_value, 0);
    if (otherValue > 0) {
      items.push({
        label: 'Other',
        value: otherValue,
        amount: otherValue,
        pnl: null,
        cost: null,
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
    <div class="overview-summary card">Loading overview...</div>
    <div class="section-sub">Market value, allocations, and warning bands.</div>
    <div class="grid three">
      <div class="card">Loading CNY allocation...</div>
      <div class="card">Loading USD allocation...</div>
      <div class="card">Loading HKD allocation...</div>
    </div>
  `;

  try {
    const [byCurrency, bySymbol, exchangeRates] = await Promise.all([
      fetchJSON('/api/holdings-by-currency'),
      fetchJSON('/api/holdings-by-symbol'),
      fetchJSON('/api/exchange-rates'),
    ]);

    const currencyOrder = ['CNY', 'USD', 'HKD'];
    const currencyList = currencyOrder.filter((curr) => (byCurrency || {})[curr]);
    const extraCurrencies = Object.keys(byCurrency || {})
      .filter((curr) => !currencyOrder.includes(curr))
      .sort();
    currencyList.push(...extraCurrencies);
    const rateMap = { CNY: 1 };
    (exchangeRates || []).forEach((item) => {
      if (item && item.to_currency === 'CNY' && Number(item.rate) > 0) {
        rateMap[item.from_currency] = Number(item.rate);
      }
    });

    let totalMarket = 0;
    let totalCost = 0;

    if (bySymbol) {
      Object.entries(bySymbol).forEach(([currency, entry]) => {
        const rate = Number(rateMap[currency] || 0);
        if (rate <= 0) {
          return;
        }
        totalMarket += Number(entry.total_market_value || 0) * rate;
        totalCost += Number(entry.total_cost || 0) * rate;
      });
    }

    const totalPnL = totalMarket - totalCost;
    const pnlClass = totalPnL >= 0 ? 'pill positive' : 'pill negative';
    const summaryCard = `
      <div class="overview-summary card">
        <div class="summary-title">Total Market Value (CNY)</div>
        <div class="summary-value" data-sensitive>${formatMoney(totalMarket, 'CNY')}</div>
        <div class="summary-sub">Converted by Settings exchange rates.</div>
        <div class="summary-pill ${pnlClass}" data-sensitive>${formatMoney(totalPnL, 'CNY')} total PnL</div>
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
      ${summaryCard}
      <div class="section-sub">Market value, allocations, and warning bands.</div>
      <div class="grid three">${allocationCards}</div>
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
              <div class="actions holdings-actions">
                <select class="btn trade-select" data-symbol="${escapeHtml(s.symbol)}" data-currency="${currency}" data-account="${escapeHtml(s.account_id || '')}" data-asset-type="${escapeHtml(s.asset_type || '')}">
                  <option value="">Trade</option>
                  <option value="BUY">Buy</option>
                  <option value="SELL">Sell</option>
                  <option value="DIVIDEND">Dividend</option>
                  <option value="TRANSFER_IN">Transfer In</option>
                  <option value="TRANSFER_OUT">Transfer Out</option>
                </select>
                <button class="btn secondary" data-action="update" data-symbol="${escapeHtml(s.symbol)}" data-currency="${currency}" data-asset-type="${escapeHtml(s.asset_type || '')}" ${updateDisabled}>Update</button>
                <button class="btn tertiary" data-action="manual" data-symbol="${escapeHtml(s.symbol)}" data-currency="${currency}">Manual</button>
                <button class="btn tertiary" data-action="symbol-ai" data-symbol="${escapeHtml(s.symbol)}" data-currency="${currency}">AI</button>
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
      const currency = btn.dataset.currency;
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
        if (action === 'manual') {
          const value = await showPromptModal(`Manual price for ${symbol} (${currency})`);
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
        } else {
          showToast('Price update failed');
        }
      }
    });
  });
}

async function runAIHoldingsAnalysis(currency, analysisType) {
  const settings = loadAIAnalysisSettings();

  const normalizedSettings = {
    baseUrl: (settings.baseUrl || 'https://api.openai.com/v1').trim(),
    model: (settings.model || '').trim(),
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

  saveAIAnalysisSettings(normalizedSettings);

  const result = await fetchJSON('/api/ai/holdings-analysis', {
    method: 'POST',
    body: JSON.stringify({
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
    }),
  });

  state.aiAnalysisByCurrency[currency] = result;
  return true;
}

async function renderSymbolAnalysis() {
  const query = getRouteQuery();
  const symbol = (query.get('symbol') || '').trim();
  const currency = (query.get('currency') || '').trim().toUpperCase();

  if (!symbol || !currency) {
    view.innerHTML = renderEmptyState('Missing symbol or currency parameter.');
    return;
  }

  view.innerHTML = `
    <div class="section-title">${escapeHtml(symbol)} Analysis</div>
    <div class="section-sub">Multi-dimensional AI deep analysis · ${escapeHtml(currency)}</div>
    <div class="symbol-analysis-actions">
      <a href="#/holdings" class="btn secondary">Back to Holdings</a>
      <button class="btn primary" id="run-symbol-analysis">Run Analysis</button>
    </div>
    <div id="symbol-analysis-content">
      <div class="card">Loading latest analysis...</div>
    </div>
  `;

  const runBtn = document.getElementById('run-symbol-analysis');
  runBtn.addEventListener('click', async () => {
    runBtn.disabled = true;
    runBtn.textContent = 'Analyzing...';
    try {
      await runSymbolAnalysis(symbol, currency);
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
      <div class="symbol-analysis-grid">
        ${renderDimensionCard('macro', '宏观经济政策', latest.dimensions?.macro)}
        ${renderDimensionCard('industry', '行业竞争格局', latest.dimensions?.industry)}
        ${renderDimensionCard('company', '公司基本面', latest.dimensions?.company)}
        ${renderDimensionCard('international', '国际政治经济', latest.dimensions?.international)}
      </div>
      ${renderSymbolAnalysisHistory(history || [])}
    `;
  } catch (err) {
    const contentEl = document.getElementById('symbol-analysis-content');
    if (contentEl) {
      contentEl.innerHTML = '<div class="card"><div class="section-sub">Failed to load analysis.</div></div>';
    }
  }
}

async function runSymbolAnalysis(symbol, currency) {
  const settings = loadAIAnalysisSettings();
  const normalizedSettings = {
    baseUrl: (settings.baseUrl || 'https://api.openai.com/v1').trim(),
    model: (settings.model || '').trim(),
    apiKey: (settings.apiKey || '').trim(),
    riskProfile: (settings.riskProfile || 'balanced').trim(),
    horizon: (settings.horizon || 'medium').trim(),
    adviceStyle: (settings.adviceStyle || 'balanced').trim(),
    strategyPrompt: (settings.strategyPrompt || '').trim(),
  };

  if (!normalizedSettings.model || !normalizedSettings.apiKey) {
    localStorage.setItem('activeSettingsTab', 'api');
    window.location.hash = '#/settings';
    showToast('Set AI model and API Key in Settings > API');
    throw new Error('AI settings not configured');
  }

  return await fetchJSON('/api/ai/symbol-analysis', {
    method: 'POST',
    body: JSON.stringify({
      base_url: normalizedSettings.baseUrl,
      api_key: normalizedSettings.apiKey,
      model: normalizedSettings.model,
      symbol,
      currency,
      risk_profile: normalizedSettings.riskProfile,
      horizon: normalizedSettings.horizon,
      advice_style: normalizedSettings.adviceStyle,
      strategy_prompt: normalizedSettings.strategyPrompt,
    }),
  });
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

  return `
    <div class="card dimension-card ${ratingClass}">
      <div class="dimension-header">
        <h4>${escapeHtml(label)}</h4>
        <span class="rating-badge rating-${escapeHtml(data.rating || 'neutral')}">${escapeHtml(data.rating || '—')}</span>
      </div>
      <div class="section-sub">Confidence: ${escapeHtml(data.confidence || '—')}</div>
      <div class="section-sub">${escapeHtml(data.summary || '')}</div>
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
      pnl: s.unrealized_pnl || 0,
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
function renderInteractivePieChart({ items, totalLabel, totalValue, currency, pieId }) {
  const data = buildPieData(items);
  if (!data) {
    return '<div class="section-sub">No data</div>';
  }

  const size = 160;
  const cx = size / 2;
  const cy = size / 2;
  const outerRadius = 70;
  const innerRadius = 45;

  const paths = data.segments.map((seg) => {
    const startAngle = (seg.start / 100) * 360 - 90;
    const endAngle = (seg.end / 100) * 360 - 90;
    const largeArc = seg.end - seg.start > 50 ? 1 : 0;

    const startRad = (startAngle * Math.PI) / 180;
    const endRad = (endAngle * Math.PI) / 180;

    const x1 = cx + outerRadius * Math.cos(startRad);
    const y1 = cy + outerRadius * Math.sin(startRad);
    const x2 = cx + outerRadius * Math.cos(endRad);
    const y2 = cy + outerRadius * Math.sin(endRad);
    const x3 = cx + innerRadius * Math.cos(endRad);
    const y3 = cy + innerRadius * Math.sin(endRad);
    const x4 = cx + innerRadius * Math.cos(startRad);
    const y4 = cy + innerRadius * Math.sin(startRad);

    const d = `M ${x1} ${y1} A ${outerRadius} ${outerRadius} 0 ${largeArc} 1 ${x2} ${y2} L ${x3} ${y3} A ${innerRadius} ${innerRadius} 0 ${largeArc} 0 ${x4} ${y4} Z`;

    const segKey = seg.key || seg.label || '';
    const pnlAttr = seg.pnl !== null && seg.pnl !== undefined ? seg.pnl : '';
    const costAttr = seg.cost !== null && seg.cost !== undefined ? seg.cost : '';
    return `<path d="${d}" fill="${seg.color}"
                  data-pie-id="${pieId}"
                  data-symbol-key="${escapeHtml(segKey)}"
                  data-label="${escapeHtml(seg.label)}"
                  data-value="${seg.value || 0}"
                  data-percent="${seg.percent || 0}"
                  data-pnl="${pnlAttr}"
                  data-cost="${costAttr}"
                  class="pie-sector"/>`;
  }).join('');

  const centerValue = totalValue !== undefined && totalValue !== null
    ? formatValue(totalValue, currency)
    : '';

  return `
    <div class="pie-svg-container">
      <svg class="pie-svg" viewBox="0 0 ${size} ${size}" data-pie-id="${pieId}">
        ${paths}
      </svg>
      <div class="pie-center-label">
        ${totalLabel ? `<div class="pie-label">${escapeHtml(totalLabel)}</div>` : ''}
        ${centerValue ? `<div class="pie-value" data-sensitive>${centerValue}</div>` : ''}
      </div>
      <div class="pie-tooltip" data-pie-id="${pieId}" style="display:none;"></div>
    </div>
  `;
}

/**
 * 渲染账户分组列表
 */
function renderAccountGroupList(accountGroups, currency, pieId) {
  if (!accountGroups || !accountGroups.length) {
    return '<div class="section-sub">No holdings</div>';
  }

  return accountGroups.map((group, index) => {
    const rows = group.symbols.map((s) => {
      const pnlClass = s.pnl >= 0 ? 'positive' : 'negative';
      return `
        <div class="symbol-row" data-pie-id="${pieId}" data-symbol-key="${escapeHtml(s.pieKey)}">
          <div class="symbol-info">
            <span class="symbol-name">${escapeHtml(s.displayName)}</span>
            <span class="symbol-code">${escapeHtml(s.symbol)}</span>
          </div>
          <div class="symbol-value num" data-sensitive>${formatMoneyPlain(s.marketValue)}</div>
          <div class="symbol-percent num">${formatPercent(s.percent)}</div>
          <div class="symbol-pnl num ${pnlClass}" data-sensitive>${formatMoneyPlain(s.pnl)}</div>
        </div>
      `;
    }).join('');

    return `
      <div class="account-group ${index % 2 === 1 ? 'alt' : ''}">
        <div class="account-header">
          <span class="account-name">${escapeHtml(group.accountName)}</span>
          <span class="account-total" data-sensitive>${formatMoneyPlain(group.totalMarketValue)}</span>
          <span class="account-percent">${formatPercent(group.percent)}</span>
        </div>
        <div class="account-symbols">${rows}</div>
      </div>
    `;
  }).join('');
}

/**
 * 高亮指定扇区
 */
function highlightPieSector(pieId, symbolKey) {
  // 清除所有高亮
  document.querySelectorAll('.pie-sector.highlighted').forEach((el) => {
    el.classList.remove('highlighted');
  });
  document.querySelectorAll('.pie-svg').forEach((svg) => {
    svg.classList.remove('has-highlight');
  });
  document.querySelectorAll('.symbol-row.highlighted').forEach((el) => {
    el.classList.remove('highlighted');
  });

  if (!symbolKey) return;

  // 高亮目标扇区
  const sector = document.querySelector(
    `.pie-sector[data-pie-id="${pieId}"][data-symbol-key="${symbolKey}"]`
  );
  if (sector) {
    sector.classList.add('highlighted');
    sector.closest('.pie-svg').classList.add('has-highlight');

    // 显示tooltip
    const tooltip = document.querySelector(`.pie-tooltip[data-pie-id="${pieId}"]`);
    if (tooltip) {
      const label = sector.dataset.label || '';
      const value = Number(sector.dataset.value || 0);
      const percent = Number(sector.dataset.percent || 0);
      const pnlRaw = sector.dataset.pnl;
      const costRaw = sector.dataset.cost;
      const hasPnl = pnlRaw !== '' && pnlRaw !== undefined;
      const pnl = hasPnl ? Number(pnlRaw) : null;
      const cost = costRaw !== '' && costRaw !== undefined ? Number(costRaw) : null;
      const pnlPercent = hasPnl && cost !== null && cost > 0 ? (pnl / cost) * 100 : null;
      const pnlClass = pnl !== null && pnl >= 0 ? 'positive' : 'negative';

      const pnlRows = hasPnl ? `
        <div class="tooltip-row">
          <span>盈亏</span>
          <span class="${pnlClass}" data-sensitive>${formatMoneyPlain(pnl)}</span>
        </div>
        ${pnlPercent !== null ? `
        <div class="tooltip-row">
          <span>盈亏率</span>
          <span class="${pnlClass}">${formatPercent(pnlPercent)}</span>
        </div>` : ''}
      ` : '';

      tooltip.innerHTML = `
        <div class="tooltip-name">${escapeHtml(label)}</div>
        <div class="tooltip-row">
          <span>占比</span>
          <span>${formatPercent(percent)}</span>
        </div>
        <div class="tooltip-row">
          <span>市值</span>
          <span data-sensitive>${formatMoneyPlain(value)}</span>
        </div>
        ${pnlRows}
      `;
      tooltip.style.display = 'block';
    }
  }

  // 高亮对应行
  const row = document.querySelector(
    `.symbol-row[data-pie-id="${pieId}"][data-symbol-key="${symbolKey}"]`
  );
  if (row) {
    row.classList.add('highlighted');
  }
}

/**
 * 绑定Charts页面交互事件
 */
function bindChartsInteractions() {
  // 环图扇区：鼠标悬停高亮并显示tooltip
  view.querySelectorAll('.pie-sector[data-symbol-key]').forEach((sector) => {
    sector.addEventListener('mouseenter', () => {
      const pieId = sector.dataset.pieId;
      const symbolKey = sector.dataset.symbolKey;
      highlightPieSector(pieId, symbolKey);
    });
    sector.addEventListener('mouseleave', () => {
      const pieId = sector.dataset.pieId;
      highlightPieSector(pieId, null);
      const tooltip = document.querySelector(`.pie-tooltip[data-pie-id="${pieId}"]`);
      if (tooltip) tooltip.style.display = 'none';
    });
  });

  // 标的行点击
  view.querySelectorAll('.symbol-row[data-symbol-key]').forEach((row) => {
    row.addEventListener('click', () => {
      const pieId = row.dataset.pieId;
      const symbolKey = row.dataset.symbolKey;
      const isHighlighted = row.classList.contains('highlighted');

      if (isHighlighted) {
        highlightPieSector(pieId, null);
        const tooltip = document.querySelector(`.pie-tooltip[data-pie-id="${pieId}"]`);
        if (tooltip) tooltip.style.display = 'none';
      } else {
        highlightPieSector(pieId, symbolKey);
      }
    });
  });

  // 点击空白处取消高亮
  view.querySelectorAll('.currency-block').forEach((block) => {
    block.addEventListener('click', (e) => {
      if (e.target.closest('.symbol-row') || e.target.closest('.pie-sector')) return;
      const pieId = block.dataset.pieId;
      highlightPieSector(pieId, null);
      const tooltip = document.querySelector(`.pie-tooltip[data-pie-id="${pieId}"]`);
      if (tooltip) tooltip.style.display = 'none';
    });
  });
}

async function renderCharts() {
  view.innerHTML = `
    <div class="section-title">Charts</div>
    <div class="section-sub">Symbol composition snapshots by currency.</div>
    <div class="card">Loading charts...</div>
  `;

  try {
    const bySymbol = await fetchJSON('/api/holdings-by-symbol');
    const currencies = Object.keys(bySymbol || {});

    if (!currencies.length) {
      view.innerHTML = renderEmptyState('No holdings data yet. Add your first transaction.', '<a class="primary" href="#/add">Add transaction</a>');
      return;
    }

    const currencyBlocks = currencies.map((currency) => {
      const data = bySymbol[currency] || {};
      const symbols = data.symbols || [];
      const totalMarketValue = data.total_market_value !== undefined && data.total_market_value !== null
        ? data.total_market_value
        : symbols.reduce((sum, s) => sum + (s.market_value || 0), 0);

      const pieId = `pie-${currency}`;

      // 构建环图数据项，添加 key 用于高亮关联
      const pieItems = buildSymbolPieItems(symbols, 8).map((item) => {
        const matchingSymbol = symbols.find((s) => s.display_name === item.label || s.symbol === item.label);
        return {
          ...item,
          key: matchingSymbol ? `${matchingSymbol.symbol}-${matchingSymbol.account_id}` : item.label,
          pnl: matchingSymbol ? (matchingSymbol.unrealized_pnl ?? null) : item.pnl,
          cost: matchingSymbol ? (matchingSymbol.avg_cost || 0) * (matchingSymbol.total_shares || 0) : item.cost,
        };
      });

      // 渲染 SVG 环图
      const pieChart = renderInteractivePieChart({
        items: pieItems,
        totalLabel: 'Total',
        totalValue: totalMarketValue,
        currency,
        pieId,
      });

      // 按账户分组
      const accountGroups = groupSymbolsByAccount(symbols, totalMarketValue);
      const groupList = renderAccountGroupList(accountGroups, currency, pieId);

      return `
        <div class="currency-block card" data-pie-id="${pieId}">
          <h3>${currency}</h3>
          <div class="chart-content">
            ${pieChart}
            <div class="account-groups-list">
              ${groupList}
            </div>
          </div>
        </div>
      `;
    }).join('');

    view.innerHTML = `
      <div class="section-title">Charts</div>
      <div class="section-sub">Asset allocation by currency and account.</div>
      <div class="charts-horizontal">
        ${currencyBlocks}
      </div>
    `;

    bindChartsInteractions();
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

async function renderSettings() {
  view.innerHTML = `
    <div class="section-title">Settings</div>
    <div class="section-sub">Accounts, FX rates, asset types, allocation ranges, storage, and API connection.</div>
    <div class="card">Loading settings...</div>
  `;

  try {
    const [accounts, assetTypes, allocationSettings, exchangeRates, symbols, holdings, storageInfo] = await Promise.all([
      fetchJSON('/api/accounts'),
      fetchJSON('/api/asset-types'),
      fetchJSON('/api/allocation-settings'),
      fetchJSON('/api/exchange-rates'),
      fetchJSON('/api/symbols'),
      fetchJSON('/api/holdings'),
      fetchJSON('/api/storage')
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

    const aiSettings = loadAIAnalysisSettings();
    const aiAnalysisSection = `
      <div class="card">
        <h3>AI Analysis</h3>
        <div class="section-sub">OpenAI-compatible configuration for holdings analysis.</div>
        <div class="form">
          <div class="form-row">
            <div class="field">
              <label>AI Base URL</label>
              <input id="ai-base-url" type="text" placeholder="https://api.openai.com/v1" value="${escapeHtml(aiSettings.baseUrl || 'https://api.openai.com/v1')}">
            </div>
            <div class="field">
              <label>Model</label>
              <input id="ai-model" type="text" placeholder="gpt-4o-mini" value="${escapeHtml(aiSettings.model || '')}">
            </div>
          </div>
          <div class="form-row">
            <div class="field">
              <label>API Key</label>
              <input id="ai-api-key" type="password" autocomplete="off" placeholder="sk-..." value="${escapeHtml(aiSettings.apiKey || '')}">
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
    saveAIAnalysis.addEventListener('click', () => {
      const baseUrlInput = document.getElementById('ai-base-url');
      const modelInput = document.getElementById('ai-model');
      const apiKeyInput = document.getElementById('ai-api-key');
      const riskProfileInput = document.getElementById('ai-risk-profile');
      const horizonInput = document.getElementById('ai-horizon');
      const adviceStyleInput = document.getElementById('ai-advice-style');
      const allowNewSymbolsInput = document.getElementById('ai-allow-new-symbols');
      const strategyPromptInput = document.getElementById('ai-strategy-prompt');

      const settings = {
        baseUrl: trimTrailingSlash(baseUrlInput && baseUrlInput.value ? baseUrlInput.value.trim() : '') || 'https://api.openai.com/v1',
        model: modelInput && modelInput.value ? modelInput.value.trim() : '',
        apiKey: apiKeyInput && apiKeyInput.value ? apiKeyInput.value.trim() : '',
        riskProfile: riskProfileInput && riskProfileInput.value ? riskProfileInput.value : 'balanced',
        horizon: horizonInput && horizonInput.value ? horizonInput.value : 'medium',
        adviceStyle: adviceStyleInput && adviceStyleInput.value ? adviceStyleInput.value : 'balanced',
        allowNewSymbols: allowNewSymbolsInput ? !!allowNewSymbolsInput.checked : true,
        strategyPrompt: strategyPromptInput && strategyPromptInput.value ? strategyPromptInput.value.trim() : '',
      };

      saveAIAnalysisSettings(settings);
      if (!settings.model || !settings.apiKey) {
        showToast('Saved. Set model and API key before running analysis');
      } else {
        showToast('AI settings saved');
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
    return `
      <div class="ai-advisor-loading">
        <div class="loading-spinner"></div>
        <p>AI 正在分析您的投资画像，生成配置建议…</p>
        <div class="section-sub">通常需要 10-30 秒，请稍候</div>
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
    const settings = loadAIAnalysisSettings();
    const model = (settings.model || '').trim();
    const apiKey = (settings.apiKey || '').trim();
    if (!model || !apiKey) {
      closeModal();
      localStorage.setItem('activeSettingsTab', 'api');
      showToast('请先在 Settings > API 配置 AI 模型和 API Key');
      return;
    }
    isLoading = true;
    renderCurrentStep();
    try {
      adviceResult = await fetchJSON('/api/ai/allocation-advice', {
        method: 'POST',
        body: JSON.stringify({
          base_url: (settings.baseUrl || 'https://api.openai.com/v1').trim(),
          api_key: apiKey,
          model,
          age_range: profile.ageRange,
          invest_goal: profile.investGoal,
          risk_tolerance: profile.riskTolerance,
          horizon: profile.horizon,
          experience_level: profile.experienceLevel,
          currencies: profile.currencies,
          custom_prompt: profile.customPrompt,
        }),
      });
    } catch (err) {
      adviceResult = null;
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
  isLoading = false;
  overlay.classList.remove('hidden');
  renderCurrentStep();
}

function registerServiceWorker() {
  if (!('serviceWorker' in navigator) || !window.location.protocol.startsWith('http')) {
    return;
  }

  const hostname = window.location.hostname;
  const isLocalHost = hostname === '127.0.0.1' || hostname === 'localhost';
  if (isLocalHost) {
    navigator.serviceWorker.getRegistrations()
      .then((registrations) => Promise.all(registrations.map((registration) => registration.unregister())))
      .catch(() => {});
    if ('caches' in window) {
      caches.keys()
        .then((keys) => Promise.all(keys.filter((key) => key.startsWith('invest-log-')).map((key) => caches.delete(key))))
        .catch(() => {});
    }
    return;
  }

  navigator.serviceWorker.register('sw.js').catch(() => {});
}

document.addEventListener('DOMContentLoaded', init);
