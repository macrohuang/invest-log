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
