function siftoResolveNotificationTargetURL(notification) {
  const raw = notification?.data;
  const candidates = [
    raw?.target_url,
    raw?.url,
    raw?.custom?.a?.target_url,
    raw?.custom?.a?.audio_briefing_url,
    raw?.custom?.u,
  ];
  for (const candidate of candidates) {
    if (typeof candidate !== "string" || !candidate.trim()) continue;
    try {
      const url = new URL(candidate, self.location.origin);
      if (url.origin !== self.location.origin) continue;
      return url.toString();
    } catch {
      // ignore malformed urls
    }
  }
  return self.location.origin + "/";
}

async function siftoOpenNotificationTarget(targetURL) {
  if (!targetURL) return;
  const clientsList = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
  for (const client of clientsList) {
    try {
      const currentURL = new URL(client.url);
      if (currentURL.origin !== self.location.origin) continue;
      await client.focus();
      if (client.url !== targetURL && "navigate" in client) {
        await client.navigate(targetURL);
      }
      await client.focus();
      return;
    } catch {
      // continue to openWindow fallback
    }
  }
  await self.clients.openWindow(targetURL);
}

self.addEventListener("notificationclick", (event) => {
  const targetURL = siftoResolveNotificationTargetURL(event.notification);
  event.stopImmediatePropagation();
  event.notification.close();
  event.waitUntil(siftoOpenNotificationTarget(targetURL));
});

importScripts("https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.sw.js");
