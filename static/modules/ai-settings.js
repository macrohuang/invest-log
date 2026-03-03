function defaultAIAnalysisSettings() {
  return {
    baseUrl: defaultOpenAIBaseURL,
    model: '',
    riskProfile: 'balanced',
    horizon: 'medium',
    adviceStyle: 'balanced',
    allowNewSymbols: true,
    strategyPrompt: '',
  };
}

function normalizeChoice(value, allowed, fallback) {
  const normalized = String(value || '').trim().toLowerCase();
  return allowed.includes(normalized) ? normalized : fallback;
}

function isGeminiModel(value) {
  return String(value || '').trim().toLowerCase().startsWith('gemini');
}

function normalizeAIBaseUrl(value, fallback = defaultOpenAIBaseURL) {
  return trimTrailingSlash(String(value || '').trim()) || fallback;
}

function normalizeAIBaseUrlForModel(value, model) {
  const normalizedBaseUrl = normalizeAIBaseUrl(value);
  if (isGeminiModel(model) && normalizedBaseUrl.toLowerCase() === defaultOpenAIBaseURL) {
    return defaultGeminiBaseURL;
  }
  return normalizedBaseUrl;
}

function normalizeAIAnalysisSettings(raw) {
  const defaults = defaultAIAnalysisSettings();
  const source = raw && typeof raw === 'object' ? raw : {};

  const model = String(source.model || '').trim();
  const baseUrlRaw = source.baseUrl || source.base_url || defaults.baseUrl;
  const baseUrl = normalizeAIBaseUrlForModel(baseUrlRaw, model);
  const riskProfile = normalizeChoice(source.riskProfile || source.risk_profile, ['conservative', 'balanced', 'aggressive'], defaults.riskProfile);
  const horizon = normalizeChoice(source.horizon, ['short', 'medium', 'long'], defaults.horizon);
  const adviceStyle = normalizeChoice(source.adviceStyle || source.advice_style, ['conservative', 'balanced', 'aggressive'], defaults.adviceStyle);
  const strategyPrompt = String(source.strategyPrompt || source.strategy_prompt || '').trim();

  let allowNewSymbols = defaults.allowNewSymbols;
  if (typeof source.allowNewSymbols === 'boolean') {
    allowNewSymbols = source.allowNewSymbols;
  } else if (typeof source.allow_new_symbols === 'boolean') {
    allowNewSymbols = source.allow_new_symbols;
  }

  return {
    baseUrl,
    model,
    riskProfile,
    horizon,
    adviceStyle,
    allowNewSymbols,
    strategyPrompt,
  };
}

function loadLegacyAIAnalysisSettings() {
  try {
    const raw = localStorage.getItem(aiAnalysisSettingsKey);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object') {
      return null;
    }
    return parsed;
  } catch (err) {
    return null;
  }
}

function getAIAnalysisAPIKey(legacySettings) {
  const stored = (localStorage.getItem(aiAnalysisAPIKeyStorageKey) || '').trim();
  if (stored) {
    return stored;
  }

  const fallback = legacySettings && legacySettings.apiKey
    ? String(legacySettings.apiKey).trim()
    : '';
  if (fallback) {
    localStorage.setItem(aiAnalysisAPIKeyStorageKey, fallback);
  }
  return fallback;
}

function setAIAnalysisAPIKey(apiKey) {
  const normalized = String(apiKey || '').trim();
  if (!normalized) {
    localStorage.removeItem(aiAnalysisAPIKeyStorageKey);
    return '';
  }
  localStorage.setItem(aiAnalysisAPIKeyStorageKey, normalized);
  return normalized;
}

function getPerplexityAPIKey() {
  return (localStorage.getItem(perplexityAPIKeyStorageKey) || '').trim();
}

function setPerplexityAPIKey(apiKey) {
  const normalized = String(apiKey || '').trim();
  if (!normalized) {
    localStorage.removeItem(perplexityAPIKeyStorageKey);
    return '';
  }
  localStorage.setItem(perplexityAPIKeyStorageKey, normalized);
  return normalized;
}

function getSymbolAnalysisUsePerplexity() {
  return localStorage.getItem(symbolAnalysisUsePerplexityKey) === 'true';
}

function setSymbolAnalysisUsePerplexity(enabled) {
  if (enabled) {
    localStorage.setItem(symbolAnalysisUsePerplexityKey, 'true');
  } else {
    localStorage.removeItem(symbolAnalysisUsePerplexityKey);
  }
}

function isDefaultAIAnalysisSettings(settings) {
  const defaults = defaultAIAnalysisSettings();
  return settings.baseUrl === defaults.baseUrl &&
    settings.model === defaults.model &&
    settings.riskProfile === defaults.riskProfile &&
    settings.horizon === defaults.horizon &&
    settings.adviceStyle === defaults.adviceStyle &&
    settings.allowNewSymbols === defaults.allowNewSymbols &&
    settings.strategyPrompt === defaults.strategyPrompt;
}

async function persistAIAnalysisSettings(settings) {
  const normalized = normalizeAIAnalysisSettings(settings);
  const saved = await fetchJSON('/api/ai-settings', {
    method: 'PUT',
    body: JSON.stringify({
      base_url: normalized.baseUrl,
      model: normalized.model,
      risk_profile: normalized.riskProfile,
      horizon: normalized.horizon,
      advice_style: normalized.adviceStyle,
      allow_new_symbols: normalized.allowNewSymbols,
      strategy_prompt: normalized.strategyPrompt,
      api_key: String(settings.apiKey || '').trim(),
    }),
  });
  return normalizeAIAnalysisSettings(saved || normalized);
}

async function loadAIAnalysisSettings(options = {}) {
  const forceRefresh = !!options.forceRefresh;
  const legacySettings = loadLegacyAIAnalysisSettings();

  if (!forceRefresh && state.aiSettingsLoaded && state.aiSettings) {
    return {
      ...state.aiSettings,
      apiKey: state.aiSettings.apiKey || getAIAnalysisAPIKey(legacySettings),
    };
  }

  let normalized = defaultAIAnalysisSettings();
  let apiKey = '';
  let loadedFromServer = false;
  try {
    const remote = await fetchJSON('/api/ai-settings');
    normalized = normalizeAIAnalysisSettings(remote);
    apiKey = String(remote.api_key || '').trim();
    loadedFromServer = true;
  } catch (err) {
    if (legacySettings) {
      normalized = normalizeAIAnalysisSettings(legacySettings);
    }
    apiKey = getAIAnalysisAPIKey(legacySettings);
  }

  if (loadedFromServer) {
    // Migrate: if backend has no API key but localStorage does, save it to backend.
    if (!apiKey) {
      const localKey = getAIAnalysisAPIKey(legacySettings);
      if (localKey) {
        try {
          await persistAIAnalysisSettings({ ...normalized, apiKey: localKey });
          apiKey = localKey;
        } catch (_) {
          apiKey = localKey;
        }
      }
    }

    if (legacySettings) {
      const legacyNormalized = normalizeAIAnalysisSettings(legacySettings);
      if (isDefaultAIAnalysisSettings(normalized) && !isDefaultAIAnalysisSettings(legacyNormalized)) {
        try {
          normalized = await persistAIAnalysisSettings({ ...legacyNormalized, apiKey });
        } catch (err) {
          // Keep server settings when migration write fails.
        }
      }
      localStorage.removeItem(aiAnalysisSettingsKey);
    }
  }

  state.aiSettings = { ...normalized, apiKey };
  state.aiSettingsLoaded = true;
  return state.aiSettings;
}

async function saveAIAnalysisSettings(settings) {
  const saved = await persistAIAnalysisSettings(settings);
  const apiKey = String(settings && settings.apiKey ? settings.apiKey : '').trim();
  state.aiSettings = { ...saved, apiKey };
  state.aiSettingsLoaded = true;
  setAIAnalysisAPIKey(apiKey);  // keep localStorage in sync as fallback
  localStorage.removeItem(aiAnalysisSettingsKey);
  return state.aiSettings;
}
