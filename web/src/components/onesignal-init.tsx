"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { useState } from "react";

function isOneSignalLike(v: unknown): v is { init: (options: Record<string, unknown>) => Promise<void>; login?: (externalId: string) => Promise<void> } {
  return typeof v === "object" && v !== null && typeof (v as { init?: unknown }).init === "function";
}

function loadOneSignalSDK(onLoaded?: () => void) {
  if (typeof window === "undefined") return;
  const now = Date.now();
  if (window.__siftoOneSignalLoading) {
    const requestedAt = window.__siftoOneSignalScriptRequestedAt ?? 0;
    if (requestedAt > 0 && now - requestedAt < 12000) return;
    window.__siftoOneSignalLoading = false;
    window.__siftoOneSignalScriptError = "script load timeout";
  }
  if (isOneSignalLike(window.OneSignal)) return;
  const existing = document.querySelector<HTMLScriptElement>("script[data-sifto-onesignal='1']");
  if (existing) existing.remove();
  window.__siftoOneSignalScriptLoaded = false;
  window.__siftoOneSignalScriptError = undefined;
  window.__siftoOneSignalScriptRequestedAt = now;
  window.__siftoOneSignalLoadAttempt = (window.__siftoOneSignalLoadAttempt ?? 0) + 1;
  window.__siftoOneSignalLoading = true;
  const script = document.createElement("script");
  script.dataset.siftoOnesignal = "1";
  script.src = `https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.page.js?attempt=${window.__siftoOneSignalLoadAttempt}`;
  script.defer = true;
  const timeoutId = window.setTimeout(() => {
    if (!window.OneSignal) {
      window.__siftoOneSignalLoading = false;
      window.__siftoOneSignalScriptLoaded = false;
      window.__siftoOneSignalScriptError = "script load timeout";
      script.remove();
    }
  }, 12000);
  script.onload = () => {
    window.clearTimeout(timeoutId);
    window.__siftoOneSignalLoading = false;
    window.__siftoOneSignalScriptLoaded = true;
    onLoaded?.();
  };
  script.onerror = (e) => {
    window.clearTimeout(timeoutId);
    window.__siftoOneSignalLoading = false;
    window.__siftoOneSignalScriptLoaded = false;
    window.__siftoOneSignalScriptError = e instanceof ErrorEvent ? e.message : "script load failed";
  };
  document.head.appendChild(script);
}

async function cleanupLegacyOneSignalRootWorker() {
  if (typeof window === "undefined" || !("serviceWorker" in navigator)) return;
  const registrations = await navigator.serviceWorker.getRegistrations();
  await Promise.all(
    registrations.map(async (registration) => {
      const activeScript = registration.active?.scriptURL ?? "";
      const waitingScript = registration.waiting?.scriptURL ?? "";
      const installingScript = registration.installing?.scriptURL ?? "";
      const scriptUrl = activeScript || waitingScript || installingScript;
      const isLegacyOneSignalRoot =
        registration.scope === `${window.location.origin}/` && scriptUrl.includes("OneSignalSDK");
      if (isLegacyOneSignalRoot) {
        await registration.unregister();
      }
    }),
  );
}

function enqueueOneSignalInit(appId: string, setReady: (ready: boolean) => void) {
  const runInit = async (oneSignalArg?: { init: (options: Record<string, unknown>) => Promise<void> }) => {
    const oneSignal = oneSignalArg ?? (isOneSignalLike(window.OneSignal) ? window.OneSignal : undefined);
    if (!oneSignal) return;
    if (window.__siftoOneSignalReady) return;
    window.__siftoOneSignalDeferredExecuted = true;
    try {
      window.__siftoOneSignalInitError = undefined;
      await cleanupLegacyOneSignalRootWorker();
      await oneSignal.init({
        appId,
        serviceWorkerPath: "/onesignal/OneSignalSDKWorker.js",
        serviceWorkerUpdaterPath: "/onesignal/OneSignalSDKUpdaterWorker.js",
        serviceWorkerParam: { scope: "/onesignal/" },
        notifyButton: { enable: false },
      });
      window.__siftoOneSignalReady = true;
      setReady(true);
    } catch (e) {
      window.__siftoOneSignalReady = false;
      window.__siftoOneSignalInitError = e instanceof Error ? e.message : String(e);
    }
  };

  window.OneSignalDeferred = window.OneSignalDeferred || [];
  window.__siftoOneSignalInitEnqueued = (window.__siftoOneSignalInitEnqueued ?? 0) + 1;
  window.OneSignalDeferred.push(async (OneSignal) => runInit(OneSignal));

  // Fallback for SDK variants that still consume legacy queue (window.OneSignal = []).
  if (!Array.isArray(window.OneSignal)) {
    window.OneSignal = [];
  }
  (window.OneSignal as Array<() => void | Promise<void>>).push(async () => {
    await runInit();
  });
}

export default function OneSignalInit() {
  const { data: session } = useSession();
  const externalId = session?.user?.email ?? null;
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;
    cleanupLegacyOneSignalRootWorker().catch(() => {
      // no-op
    });
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const appId = process.env.NEXT_PUBLIC_ONESIGNAL_APP_ID?.trim();
    if (!appId) return;
    if (window.__siftoOneSignalReady) return;
    window.__siftoOneSignalDeferredExecuted = false;
    window.__siftoOneSignalInitEnqueued = 0;

    // Enqueue once before SDK load, and once right after load to handle Firefox timing edge cases.
    enqueueOneSignalInit(appId, setReady);
    loadOneSignalSDK(() => {
      if (!window.__siftoOneSignalReady) {
        enqueueOneSignalInit(appId, setReady);
      }
    });
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!ready && !window.__siftoOneSignalReady) return;
    if (!externalId) return;
    const OneSignal = isOneSignalLike(window.OneSignal) ? window.OneSignal : undefined;
    if (!OneSignal?.login) return;
    OneSignal.login(externalId).catch(() => {
      // no-op
    });
  }, [externalId, ready]);

  return null;
}
