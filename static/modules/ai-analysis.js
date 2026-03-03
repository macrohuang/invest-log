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

function renderAIStreamCard(currency) {
  const streamState = state.aiStreamingByCurrency[currency];
  if (!streamState) {
    return '';
  }

  const stage = streamState.stage ? escapeHtml(String(streamState.stage)) : 'Analyzing...';
  const rawText = streamState.text || streamState.content || '';
  const text = rawText ? escapeHtml(String(rawText)) : '';
  const error = streamState.error ? escapeHtml(String(streamState.error)) : '';
  const statusLabel = streamState.active ? 'Streaming' : (error ? 'Failed' : 'Completed');

  return `
    <div class="card ai-stream-card" data-ai-stream-card="${currency}">
      <div class="ai-analysis-head">
        <h4>AI Streaming <span class="tag other">${statusLabel}</span></h4>
        <div class="section-sub">${stage}</div>
      </div>
      ${error ? `<div class="section-sub ai-stream-error">${error}</div>` : ''}
      ${text ? `<pre class="ai-stream-content">${text}</pre>` : '<div class="section-sub">Waiting for model output...</div>'}
    </div>
  `;
}

// Updates only the AI stream card in-place to avoid a full page re-render during streaming.
// Returns true if the card was found and updated; false if a full render is needed instead.
function updateAIStreamCardInPlace(currency) {
  const el = document.querySelector(`[data-ai-stream-card="${CSS.escape(currency)}"]`);
  if (!el) return false;
  const html = renderAIStreamCard(currency);
  if (!html) return false;
  const tmp = document.createElement('div');
  tmp.innerHTML = html;
  const newEl = tmp.firstElementChild;
  if (newEl) {
    el.replaceWith(newEl);
  }
  return true;
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
