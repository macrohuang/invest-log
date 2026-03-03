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

function renderEmptyState(message, action) {
  return `
    <div class="card">
      <h3>Nothing here yet</h3>
      <p class="section-sub">${escapeHtml(message)}</p>
      ${action || ''}
    </div>
  `;
}
