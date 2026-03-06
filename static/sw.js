/**
 * Service Worker for Invest Log SPA
 */

const CACHE_NAME = 'invest-log-v5';
const STATIC_ASSETS = [
  './',
  './index.html',
  './style.css',
  './app.js',
  './modules/state.js',
  './modules/utils.js',
  './modules/api.js',
  './modules/ui.js',
  './modules/charts.js',
  './modules/ai-settings.js',
  './modules/ai-analysis.js',
  './modules/router.js',
  './modules/pages/overview.js',
  './modules/pages/holdings.js',
  './modules/pages/symbol-analysis.js',
  './modules/pages/transactions.js',
  './modules/pages/charts.js',
  './modules/pages/add-transaction.js',
  './modules/pages/transfer.js',
  './modules/pages/settings_render.js',
  './modules/pages/settings_actions.js',
  './modules/pages/settings_ai_advisor.js',
  './modules/pages/settings.js',
  './manifest.json',
  './icons/icon-192.png',
  './icons/icon-512.png'
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => cache.addAll(STATIC_ASSETS))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE_NAME).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;

  const url = new URL(event.request.url);
  if (STATIC_ASSETS.some((asset) => url.pathname.endsWith(asset.replace('./', '')))) {
    event.respondWith(
      caches.match(event.request).then((cached) => cached || fetch(event.request))
    );
    return;
  }

  if (url.pathname.startsWith('/api/')) {
    event.respondWith(fetch(event.request));
    return;
  }

  event.respondWith(
    fetch(event.request).catch(() => caches.match('./index.html'))
  );
});
