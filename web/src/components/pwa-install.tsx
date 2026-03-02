"use client";

import { useEffect, useState } from "react";
import { Download } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";

interface BeforeInstallPromptEvent extends Event {
  readonly platforms: string[];
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed"; platform: string }>;
}

function isStandaloneMode(): boolean {
  if (typeof window === "undefined") return false;
  return window.matchMedia?.("(display-mode: standalone)").matches || Boolean((window.navigator as Navigator & { standalone?: boolean }).standalone);
}

export default function PWAInstallButton() {
  const { t } = useI18n();
  const [deferredPrompt, setDeferredPrompt] = useState<BeforeInstallPromptEvent | null>(null);
  const [hidden, setHidden] = useState(false);
  const [installing, setInstalling] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (isStandaloneMode()) {
      setHidden(true);
      return;
    }
    if (!("serviceWorker" in navigator)) return;

    const isSecure = window.location.protocol === "https:" || window.location.hostname === "localhost";
    if (isSecure) {
      navigator.serviceWorker.register("/sw.js").catch(() => {
        // no-op
      });
    }

    const onBeforeInstallPrompt = (event: Event) => {
      event.preventDefault();
      setDeferredPrompt(event as BeforeInstallPromptEvent);
    };
    const onAppInstalled = () => {
      setDeferredPrompt(null);
      setHidden(true);
    };
    window.addEventListener("beforeinstallprompt", onBeforeInstallPrompt);
    window.addEventListener("appinstalled", onAppInstalled);
    return () => {
      window.removeEventListener("beforeinstallprompt", onBeforeInstallPrompt);
      window.removeEventListener("appinstalled", onAppInstalled);
    };
  }, []);

  const canInstall = !hidden && deferredPrompt != null;
  if (!canInstall) return null;

  return (
    <button
      type="button"
      disabled={installing}
      onClick={async () => {
        if (!deferredPrompt || installing) return;
        setInstalling(true);
        try {
          await deferredPrompt.prompt();
          await deferredPrompt.userChoice;
        } finally {
          setDeferredPrompt(null);
          setInstalling(false);
        }
      }}
      className="inline-flex items-center gap-1 rounded border border-zinc-200 bg-white px-2 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-50"
    >
      <Download className="size-3.5" aria-hidden="true" />
      <span>{t("pwa.install")}</span>
    </button>
  );
}

