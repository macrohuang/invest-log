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
