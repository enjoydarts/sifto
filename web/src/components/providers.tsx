"use client";

import { useEffect, useState } from "react";
import { ClerkProvider } from "@clerk/nextjs";
import { QueryClient, QueryClientProvider, useQueryClient } from "@tanstack/react-query";
import { I18nProvider } from "@/components/i18n-provider";
import { useI18n } from "@/components/i18n-provider";
import { ToastProvider } from "@/components/toast-provider";
import { ConfirmProvider } from "@/components/confirm-provider";
import AuthTokenBridge from "@/components/auth-token-bridge";
import PWARegister from "@/components/pwa-register";
import OneSignalInit from "@/components/onesignal-init";

type ProvidersProps = {
  children: React.ReactNode;
  clerkEnabled: boolean;
  clerkPublishableKey?: string;
};

const clerkJapaneseLocalization = {
  signIn: {
    start: {
      actionText: "アカウントをお持ちでないですか？",
      actionLink: "新規登録",
    },
    password: {
    },
    alternativeMethods: {
      title: "別の方法でログイン",
      actionLink: "別の方法を使う",
      blockButton: "別の方法を使う",
    },
  },
  formFieldLabel__emailAddress: "メールアドレス",
  formFieldLabel__password: "パスワード",
  formButtonPrimary: "続ける",
  footerActionLink__useAnotherMethod: "別の方法を使う",
  dividerText: "または",
  socialButtonsBlockButton: "{{provider}}で続ける",
};

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

export function Providers({ children, clerkEnabled, clerkPublishableKey }: ProvidersProps) {
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

  const content = (
    <QueryClientProvider client={queryClient}>
      <I18nProvider>
        <ProvidersWithI18n
          clerkEnabled={clerkEnabled}
          clerkPublishableKey={clerkPublishableKey}
        >
          <ToastProvider>
            <ConfirmProvider>
              {clerkEnabled ? <AuthTokenBridge /> : null}
              <QueryRefreshOnResume />
              <PWARegister />
              <OneSignalInit />
              {children}
            </ConfirmProvider>
          </ToastProvider>
        </ProvidersWithI18n>
      </I18nProvider>
    </QueryClientProvider>
  );

  return content;
}

function ProvidersWithI18n({
  children,
  clerkEnabled,
  clerkPublishableKey,
}: {
  children: React.ReactNode;
  clerkEnabled: boolean;
  clerkPublishableKey?: string;
}) {
  const { locale } = useI18n();

  if (!clerkEnabled) {
    return children;
  }

  return (
    <ClerkProvider
      publishableKey={clerkPublishableKey}
      localization={locale === "ja" ? clerkJapaneseLocalization : undefined}
    >
      {children}
    </ClerkProvider>
  );
}
