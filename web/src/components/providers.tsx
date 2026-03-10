"use client";

import { useEffect, useState } from "react";
import { SessionProvider } from "next-auth/react";
import { QueryClient, QueryClientProvider, useQueryClient } from "@tanstack/react-query";
import { I18nProvider } from "@/components/i18n-provider";
import { ToastProvider } from "@/components/toast-provider";
import { ConfirmProvider } from "@/components/confirm-provider";
import PWARegister from "@/components/pwa-register";
import OneSignalInit from "@/components/onesignal-init";

function QueryRefreshOnResume() {
  const queryClient = useQueryClient();

  useEffect(() => {
    let lastRefreshAt = 0;
    const refreshActiveQueries = () => {
      const now = Date.now();
      if (now - lastRefreshAt < 5000) return;
      lastRefreshAt = now;
      void queryClient.refetchQueries({ type: "active" });
    };
    const onVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        refreshActiveQueries();
      }
    };
    const onPageShow = () => refreshActiveQueries();
    const onOnline = () => refreshActiveQueries();

    document.addEventListener("visibilitychange", onVisibilityChange);
    window.addEventListener("pageshow", onPageShow);
    window.addEventListener("online", onOnline);
    return () => {
      document.removeEventListener("visibilitychange", onVisibilityChange);
      window.removeEventListener("pageshow", onPageShow);
      window.removeEventListener("online", onOnline);
    };
  }, [queryClient]);

  return null;
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 5_000,
            gcTime: 10 * 60_000,
            refetchOnWindowFocus: true,
            refetchOnReconnect: true,
            refetchOnMount: true,
            retry: 1,
          },
        },
      })
  );

  return (
    <QueryClientProvider client={queryClient}>
      <SessionProvider>
        <I18nProvider>
          <ToastProvider>
            <ConfirmProvider>
              <QueryRefreshOnResume />
              <PWARegister />
              <OneSignalInit />
              {children}
            </ConfirmProvider>
          </ToastProvider>
        </I18nProvider>
      </SessionProvider>
    </QueryClientProvider>
  );
}
