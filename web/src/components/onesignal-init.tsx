"use client";

import { useEffect, useState } from "react";
import { useUser } from "@clerk/nextjs";

function isOneSignalLike(
  v: unknown
): v is {
  init: (options: Record<string, unknown>) => Promise<void>;
  login?: (externalId: string) => Promise<void>;
} {
  return typeof v === "object" && v !== null && typeof (v as { init?: unknown }).init === "function";
}

async function cleanupLegacyOneSignalRootWorker() {
  if (typeof window === "undefined" || !("serviceWorker" in navigator)) return;
  const registrations = await navigator.serviceWorker.getRegistrations();
  await Promise.all(
    registrations.map(async (registration) => {
      const activeScript = registration.active?.scriptURL ?? "";
      const waitingScript = registration.waiting?.scriptURL ?? "";
      const installingScript = registration.installing?.scriptURL ?? "";
      const scriptURL = activeScript || waitingScript || installingScript;
      const isLegacyOneSignalRoot =
        registration.scope === `${window.location.origin}/` && scriptURL.includes("OneSignalSDK");
      if (isLegacyOneSignalRoot) {
        await registration.unregister();
      }
    })
  );
}

export default function OneSignalInit() {
  const { user } = useUser();
  return <OneSignalInitInner externalId={user?.primaryEmailAddress?.emailAddress ?? null} />;
}

function OneSignalInitInner({ externalId }: { externalId: string | null }) {
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

    window.__siftoOneSignalScriptError = undefined;
    window.__siftoOneSignalScriptLoaded = false;
    window.__siftoOneSignalLoading = true;

    const startedAt = Date.now();
    const poll = window.setInterval(() => {
      if (isOneSignalLike(window.OneSignal)) {
        window.__siftoOneSignalScriptLoaded = true;
        window.__siftoOneSignalLoading = false;
        window.clearInterval(poll);
        return;
      }
      if (Date.now() - startedAt > 15000) {
        window.__siftoOneSignalScriptLoaded = false;
        window.__siftoOneSignalLoading = false;
        window.__siftoOneSignalScriptError = "sdk not loaded";
        window.clearInterval(poll);
      }
    }, 300);

    window.OneSignalDeferred = window.OneSignalDeferred || [];
    window.__siftoOneSignalInitEnqueued = (window.__siftoOneSignalInitEnqueued ?? 0) + 1;
    window.OneSignalDeferred.push(async (OneSignal) => {
      if (window.__siftoOneSignalReady) return;
      window.__siftoOneSignalDeferredExecuted = true;
      try {
        window.__siftoOneSignalInitError = undefined;
        await cleanupLegacyOneSignalRootWorker();
        await OneSignal.init({
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
    });

    return () => {
      window.clearInterval(poll);
    };
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!ready && !window.__siftoOneSignalReady) return;
    if (!externalId) return;
    const oneSignal = isOneSignalLike(window.OneSignal) ? window.OneSignal : undefined;
    if (!oneSignal?.login) return;
    oneSignal.login(externalId).catch(() => {
      // no-op
    });
  }, [externalId, ready]);

  return null;
}
