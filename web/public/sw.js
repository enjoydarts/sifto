const SW_VERSION = "v3";
const STATIC_CACHE = `sifto-static-${SW_VERSION}`;
const PAGE_CACHE = `sifto-pages-${SW_VERSION}`;
const API_CACHE = `sifto-api-${SW_VERSION}`;
const MAX_PAGE_CACHE_ENTRIES = 40;
const MAX_API_CACHE_ENTRIES = 80;
const PRECACHE_URLS = [
  "/offline.html",
  "/manifest.webmanifest",
  "/logo.png",
  "/logo-192.png",
  "/logo-512.png",
  "/logo-maskable-512.png",
  "/apple-touch-icon.png",
];

async function trimPageCache() {
  const cache = await caches.open(PAGE_CACHE);
  const keys = await cache.keys();
  if (keys.length <= MAX_PAGE_CACHE_ENTRIES) return;
  const deleteCount = keys.length - MAX_PAGE_CACHE_ENTRIES;
  await Promise.all(keys.slice(0, deleteCount).map((req) => cache.delete(req)));
}

async function trimApiCache() {
  const cache = await caches.open(API_CACHE);
  const keys = await cache.keys();
  if (keys.length <= MAX_API_CACHE_ENTRIES) return;
  const deleteCount = keys.length - MAX_API_CACHE_ENTRIES;
  await Promise.all(keys.slice(0, deleteCount).map((req) => cache.delete(req)));
}

function shouldCacheAPI(url) {
  if (!url.pathname.startsWith("/api/")) return false;
  if (url.pathname.startsWith("/api/auth/")) return false;
  if (url.pathname.startsWith("/api/debug/")) return false;

  return [
    "/api/briefing",
    "/api/items",
    "/api/focus-queue",
    "/api/triage",
    "/api/digests",
    "/api/sources",
    "/api/topic-pulse",
    "/api/llm-usage",
    "/api/settings",
  ].some((prefix) => url.pathname.startsWith(prefix));
}

async function staleWhileRevalidateAPI(req) {
  const cache = await caches.open(API_CACHE);
  const cached = await cache.match(req);
  const networkFetch = fetch(req)
    .then((res) => {
      if (res && res.ok) {
        const copy = res.clone();
        cache.put(req, copy).then(trimApiCache);
      }
      return res;
    })
    .catch(() => cached);

  return cached || networkFetch;
}

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(STATIC_CACHE).then((cache) => cache.addAll(PRECACHE_URLS)).then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((key) => key.startsWith("sifto-") && key !== STATIC_CACHE && key !== PAGE_CACHE && key !== API_CACHE)
          .map((key) => caches.delete(key))
      )
    ).then(async () => {
      if ("navigationPreload" in self.registration) {
        await self.registration.navigationPreload.enable();
      }
      await self.clients.claim();
    })
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith("/_next/webpack-hmr")) return;

  if (shouldCacheAPI(url)) {
    event.respondWith(staleWhileRevalidateAPI(req));
    return;
  }

  if (req.mode === "navigate") {
    event.respondWith(
      (async () => {
        try {
          const preload = await event.preloadResponse;
          const res = preload || (await fetch(req));
          const copy = res.clone();
          await caches.open(PAGE_CACHE).then((cache) => cache.put(req, copy).then(trimPageCache));
          return res;
        } catch {
          const cached = await caches.match(req);
          if (cached) return cached;
          return caches.match("/offline.html");
        }
      })()
    );
    return;
  }

  const destination = req.destination;
  if (["style", "script", "worker", "image", "font"].includes(destination)) {
    if (destination === "image" && !url.pathname.startsWith("/_next/static/")) {
      event.respondWith(
        fetch(req)
          .then((res) => {
            const copy = res.clone();
            caches.open(STATIC_CACHE).then((cache) => cache.put(req, copy));
            return res;
          })
          .catch(() => caches.match(req))
      );
      return;
    }
    event.respondWith(
      caches.match(req).then((cached) => {
        const networkFetch = fetch(req)
          .then((res) => {
            const copy = res.clone();
            caches.open(STATIC_CACHE).then((cache) => cache.put(req, copy));
            return res;
          })
          .catch(() => cached);
        return cached || networkFetch;
      })
    );
  }
});
