"use client";

import {
  createContext,
  ReactNode,
  useContext,
  useEffect,
  useMemo,
  useSyncExternalStore,
} from "react";
import { dict } from "@/i18n/dictionaries";
import type { Locale } from "@/i18n/types";


type I18nValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string, fallback?: string) => string;
};

const I18nContext = createContext<I18nValue | null>(null);
const localeKey = "sifto.locale";
const localeChangedEvent = "sifto:locale-changed";

function readLocale(): Locale {
  return window.localStorage.getItem(localeKey) === "en" ? "en" : "ja";
}

function subscribeToLocale(listener: () => void) {
  window.addEventListener("storage", listener);
  window.addEventListener(localeChangedEvent, listener);
  return () => {
    window.removeEventListener("storage", listener);
    window.removeEventListener(localeChangedEvent, listener);
  };
}

function setLocale(locale: Locale) {
  window.localStorage.setItem(localeKey, locale);
  window.dispatchEvent(new Event(localeChangedEvent));
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const locale = useSyncExternalStore<Locale>(subscribeToLocale, readLocale, () => "ja");

  useEffect(() => {
    if (typeof document !== "undefined") {
      document.documentElement.lang = locale;
    }
  }, [locale]);

  const value = useMemo<I18nValue>(
    () => ({
      locale,
      setLocale,
      t: (key, fallback) => dict[locale][key] ?? fallback ?? key,
    }),
    [locale]
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error("useI18n must be used within I18nProvider");
  }
  return ctx;
}
