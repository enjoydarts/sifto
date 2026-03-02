const SW_VERSION = "v2";
const STATIC_CACHE = `sifto-static-${SW_VERSION}`;
const PAGE_CACHE = `sifto-pages-${SW_VERSION}`;
const MAX_PAGE_CACHE_ENTRIES = 40;
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
          .filter((key) => key.startsWith("sifto-") && key !== STATIC_CACHE && key !== PAGE_CACHE)
          .map((key) => caches.delete(key))
      )
    ).then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith("/api/")) return;
  if (url.pathname.startsWith("/_next/webpack-hmr")) return;

  if (req.mode === "navigate") {
    event.respondWith(
      fetch(req)
        .then((res) => {
          const copy = res.clone();
          caches.open(PAGE_CACHE).then((cache) => cache.put(req, copy).then(trimPageCache));
          return res;
        })
        .catch(async () => {
          const cached = await caches.match(req);
          if (cached) return cached;
          return caches.match("/offline.html");
        })
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
