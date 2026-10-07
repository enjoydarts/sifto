const SW_VERSION = "v5";
const STATIC_CACHE = `sifto-static-${SW_VERSION}`;
const PRECACHE_URLS = [
  "/offline.html", "/manifest.webmanifest", "/logo.png", "/logo-192.png",
  "/logo-512.png", "/logo-maskable-512.png", "/apple-touch-icon.png",
];

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(STATIC_CACHE)
    .then((cache) => cache.addAll(PRECACHE_URLS)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(caches.keys().then((keys) => Promise.all(keys
    .filter((key) => key.startsWith("sifto-") && key !== STATIC_CACHE)
    .map((key) => caches.delete(key))))
    .then(async () => {
      if ("navigationPreload" in self.registration) await self.registration.navigationPreload.enable();
      await self.clients.claim();
    }));
});

self.addEventListener("message", (event) => {
  if (event.data?.type === "SKIP_WAITING") event.waitUntil(self.skipWaiting());
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin || url.pathname.startsWith("/api/")) return;

  // Authenticated HTML, RSC, API and proxied images must never persist across sessions.
  if (req.mode === "navigate") {
    event.respondWith((async () => {
      try { return (await event.preloadResponse) || (await fetch(req)); }
      catch { return caches.open(STATIC_CACHE).then((cache) => cache.match("/offline.html")); }
    })());
    return;
  }
  if (!url.pathname.startsWith("/_next/static/") && !PRECACHE_URLS.includes(url.pathname)) return;
  event.respondWith(caches.open(STATIC_CACHE).then(async (cache) => {
    const cached = await cache.match(req);
    if (cached) return cached;
    const response = await fetch(req);
    if (response.ok) await cache.put(req, response.clone());
    return response;
  }));
});
