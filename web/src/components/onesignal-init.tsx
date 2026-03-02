"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { useState } from "react";

function loadOneSignalSDK() {
  if (typeof window === "undefined") return;
  if (window.__siftoOneSignalLoading) return;
  if (window.OneSignal) return;
  window.__siftoOneSignalScriptLoaded = false;
  window.__siftoOneSignalScriptError = undefined;
  window.__siftoOneSignalLoading = true;
  const script = document.createElement("script");
  script.src = "/api/onesignal/sdk/page";
  script.defer = true;
  script.onload = () => {
    window.__siftoOneSignalLoading = false;
    window.__siftoOneSignalScriptLoaded = true;
  };
  script.onerror = (e) => {
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

export default function OneSignalInit() {
  const { data: session } = useSession();
  const externalId = session?.user?.email ?? null;
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const appId = process.env.NEXT_PUBLIC_ONESIGNAL_APP_ID?.trim();
    if (!appId) return;
    if (window.__siftoOneSignalReady) return;

    window.OneSignalDeferred = window.OneSignalDeferred || [];
    window.OneSignalDeferred.push(async (OneSignal) => {
      if (window.__siftoOneSignalReady) return;
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
    loadOneSignalSDK();
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!ready && !window.__siftoOneSignalReady) return;
    if (!externalId) return;
    const OneSignal = window.OneSignal;
    if (!OneSignal?.login) return;
    OneSignal.login(externalId).catch(() => {
      // no-op
    });
  }, [externalId, ready]);

  return null;
}
