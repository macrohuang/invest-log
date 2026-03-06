function normalizeAIAnalysisMethodRecord(raw) {
  const source = raw && typeof raw === 'object' ? raw : {};
  const variables = Array.isArray(source.variables)
    ? source.variables.map((item) => String(item || '').trim()).filter(Boolean)
    : [];

  return {
    id: Number(source.id || 0),
    name: String(source.name || '').trim(),
    systemPrompt: String(source.system_prompt || source.systemPrompt || ''),
    userPrompt: String(source.user_prompt || source.userPrompt || ''),
    variables,
    createdAt: String(source.created_at || source.createdAt || ''),
    updatedAt: String(source.updated_at || source.updatedAt || ''),
  };
}

async function loadAIAnalysisMethods(options = {}) {
  const forceRefresh = !!options.forceRefresh;
  if (!forceRefresh && state.aiAnalysisMethodsLoaded && Array.isArray(state.aiAnalysisMethods)) {
    return state.aiAnalysisMethods;
  }

  const methods = await fetchJSON('/api/ai-analysis-methods');
  state.aiAnalysisMethods = Array.isArray(methods)
    ? methods.map(normalizeAIAnalysisMethodRecord)
    : [];
  state.aiAnalysisMethodsLoaded = true;
  return state.aiAnalysisMethods;
}

async function saveAIAnalysisMethod(method) {
  const payload = {
    name: String(method && method.name ? method.name : '').trim(),
    system_prompt: String(method && method.systemPrompt ? method.systemPrompt : '').trim(),
    user_prompt: String(method && method.userPrompt ? method.userPrompt : '').trim(),
  };

  const id = Number(method && method.id ? method.id : 0);
  const saved = await fetchJSON(id > 0 ? `/api/ai-analysis-methods/${id}` : '/api/ai-analysis-methods', {
    method: id > 0 ? 'PUT' : 'POST',
    body: JSON.stringify(payload),
  });

  state.aiAnalysisMethodsLoaded = false;
  return normalizeAIAnalysisMethodRecord(saved);
}

async function removeAIAnalysisMethod(id) {
  await fetchJSON(`/api/ai-analysis-methods/${id}`, {
    method: 'DELETE',
  });
  state.aiAnalysisMethodsLoaded = false;
}

function normalizeAIAnalysisRunRecord(raw) {
  const source = raw && typeof raw === 'object' ? raw : {};
  const variables = source.variables && typeof source.variables === 'object'
    ? Object.fromEntries(Object.entries(source.variables).map(([key, value]) => [String(key), String(value ?? '')]))
    : {};

  return {
    id: Number(source.id || 0),
    methodId: source.method_id === null || source.method_id === undefined || source.method_id === ''
      ? null
      : Number(source.method_id),
    methodName: String(source.method_name || ''),
    systemPromptTemplate: String(source.system_prompt_template || ''),
    userPromptTemplate: String(source.user_prompt_template || ''),
    variables,
    renderedSystemPrompt: String(source.rendered_system_prompt || ''),
    renderedUserPrompt: String(source.rendered_user_prompt || ''),
    model: String(source.model || ''),
    status: String(source.status || ''),
    resultText: String(source.result_text || ''),
    errorMessage: String(source.error_message || ''),
    createdAt: String(source.created_at || ''),
    completedAt: String(source.completed_at || ''),
  };
}

async function loadAIAnalysisHistory(methodId, limit = 12) {
  const params = new URLSearchParams();
  if (Number(methodId) > 0) {
    params.set('method_id', String(methodId));
  }
  params.set('limit', String(limit));
  const items = await fetchJSON(`/api/ai-analysis/history?${params.toString()}`);
  return Array.isArray(items) ? items.map(normalizeAIAnalysisRunRecord) : [];
}

async function loadAIAnalysisRun(runId) {
  const item = await fetchJSON(`/api/ai-analysis/runs/${encodeURIComponent(runId)}`);
  return normalizeAIAnalysisRunRecord(item);
}
