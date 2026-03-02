"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { useState } from "react";

function loadOneSignalSDK() {
  if (typeof window === "undefined") return;
  if (window.__siftoOneSignalLoading) return;
  if (window.OneSignal) return;
  window.__siftoOneSignalLoading = true;
  const script = document.createElement("script");
  script.src = "https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.page.js";
  script.defer = true;
  script.onload = () => {
    window.__siftoOneSignalLoading = false;
  };
  script.onerror = () => {
    window.__siftoOneSignalLoading = false;
  };
  document.head.appendChild(script);
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
      await OneSignal.init({
        appId,
        serviceWorkerPath: "onesignal/OneSignalSDKWorker.js",
        serviceWorkerUpdaterPath: "onesignal/OneSignalSDKUpdaterWorker.js",
        serviceWorkerParam: { scope: "/onesignal/" },
        notifyButton: { enable: false },
      });
      window.__siftoOneSignalReady = true;
      setReady(true);
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
