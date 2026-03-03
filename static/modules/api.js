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

function parseSSEEvent(block) {
  const normalized = String(block || '').replace(/\r/g, '');
  const lines = normalized.split('\n');
  let eventName = 'message';
  const dataLines = [];
  lines.forEach((line) => {
    if (line.startsWith('event:')) {
      eventName = line.slice(6).trim() || 'message';
      return;
    }
    if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trim());
    }
  });
  if (!dataLines.length) {
    return null;
  }
  return {
    event: eventName,
    data: dataLines.join('\n'),
  };
}

async function fetchSSE(path, options = {}) {
  if (!state.apiBase && window.location.protocol === 'file:') {
    throw new Error('API base not set');
  }

  const response = await fetch(apiUrl(path), {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  });
  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Request failed: ${response.status}`);
  }
  if (!response.body) {
    throw new Error('SSE response body unavailable');
  }

  const onEvent = typeof options.onEvent === 'function' ? options.onEvent : () => {};
  const reader = response.body.getReader();
  const decoder = new TextDecoder('utf-8');
  let buffer = '';

  while (true) {
    const { value, done } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });

    let separator = buffer.indexOf('\n\n');
    while (separator !== -1) {
      const rawEvent = buffer.slice(0, separator);
      buffer = buffer.slice(separator + 2);
      const parsed = parseSSEEvent(rawEvent);
      if (parsed) {
        await onEvent(parsed);
      }
      separator = buffer.indexOf('\n\n');
    }
  }

  const tail = parseSSEEvent(buffer.trim());
  if (tail) {
    await onEvent(tail);
  }
}

async function postSSE(path, payload, handlers = {}) {
  await fetchSSE(path, {
    method: 'POST',
    body: JSON.stringify(payload),
    onEvent: async (event) => {
      let data = event.data;
      try {
        data = JSON.parse(event.data);
      } catch (_) {
        // Keep raw string when JSON parsing fails.
      }

      if (handlers.onEvent) handlers.onEvent(event.event, data);
      if (event.event === 'progress' && handlers.onProgress) handlers.onProgress(data);
      if (event.event === 'delta' && handlers.onDelta) handlers.onDelta(data);
      if (event.event === 'chunk') {
        if (handlers.onChunk) handlers.onChunk(data);
        if (handlers.onDelta) {
          const chunkText = data && data.delta ? String(data.delta) : '';
          handlers.onDelta({ text: chunkText });
        }
      }
      if (event.event === 'result' && handlers.onResult) handlers.onResult(data);
      if (event.event === 'error' && handlers.onError) handlers.onError(data);
      if (event.event === 'done') {
        if (data && data.result && handlers.onResult) handlers.onResult(data.result);
        if (handlers.onDone) handlers.onDone(data);
      }
    },
  });
}
