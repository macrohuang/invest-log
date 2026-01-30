/**
 * Service Worker for Invest Log PWA
 * 
 * Provides basic caching for static assets and offline support.
 * Note: The app requires the backend server to be running for full functionality.
 */

const CACHE_NAME = 'invest-log-v1';
const STATIC_ASSETS = [
  '/static/style.css',
  '/static/manifest.json',
];

// Install event - cache static assets
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => {
        console.log('[SW] Caching static assets');
        return cache.addAll(STATIC_ASSETS);
      })
      .then(() => {
        // Skip waiting to activate immediately
        return self.skipWaiting();
      })
  );
});

// Activate event - clean up old caches
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((cacheNames) => {
        return Promise.all(
          cacheNames
            .filter((name) => name !== CACHE_NAME)
            .map((name) => {
              console.log('[SW] Deleting old cache:', name);
              return caches.delete(name);
            })
        );
      })
      .then(() => {
        // Take control of all clients immediately
        return self.clients.claim();
      })
  );
});

// Fetch event - network first with cache fallback for static assets
self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);
  
  // Only cache GET requests
  if (event.request.method !== 'GET') {
    return;
  }
  
  // For static assets, try cache first
  if (url.pathname.startsWith('/static/')) {
    event.respondWith(
      caches.match(event.request)
        .then((cachedResponse) => {
          if (cachedResponse) {
            // Return cached version and update cache in background
            fetch(event.request)
              .then((response) => {
                if (response.ok) {
                  caches.open(CACHE_NAME)
                    .then((cache) => cache.put(event.request, response));
                }
              })
              .catch(() => {});
            return cachedResponse;
          }
          
          // Not in cache, fetch from network
          return fetch(event.request)
            .then((response) => {
              if (response.ok) {
                const responseClone = response.clone();
                caches.open(CACHE_NAME)
                  .then((cache) => cache.put(event.request, responseClone));
              }
              return response;
            });
        })
    );
    return;
  }
  
  // For API requests and pages, always use network
  // This ensures data is always fresh
  event.respondWith(
    fetch(event.request)
      .catch(() => {
        // If offline and it's a navigation request, show offline page
        if (event.request.mode === 'navigate') {
          return new Response(
            '<!DOCTYPE html><html><head><title>Offline</title></head>' +
            '<body style="font-family:system-ui;text-align:center;padding:50px;">' +
            '<h1>You are offline</h1>' +
            '<p>Invest Log requires an internet connection to access the backend server.</p>' +
            '<button onclick="location.reload()">Retry</button>' +
            '</body></html>',
            { headers: { 'Content-Type': 'text/html' } }
          );
        }
        throw new Error('Network request failed');
      })
  );
});
