"use client";

import type { ProviderModelChangeEvent, UserSettings } from "@/lib/api";
import { DEFAULT_UI_FONT_SANS_KEY, DEFAULT_UI_FONT_SERIF_KEY, getUIFontByKey } from "@/lib/ui-fonts";

export const MODEL_UPDATES_DISMISSED_AT_KEY = "provider-model-updates:dismissed-at";

export function buildUIFontState(
  settings: UserSettings | null | undefined,
  uiFontSansKey: string,
  uiFontSerifKey: string,
) {
  const savedSansKey = settings?.ui_font_sans_key?.trim() || DEFAULT_UI_FONT_SANS_KEY;
  const savedSerifKey = settings?.ui_font_serif_key?.trim() || DEFAULT_UI_FONT_SERIF_KEY;
  const savedSans = getUIFontByKey(savedSansKey);
  const savedSerif = getUIFontByKey(savedSerifKey);
  return {
    savedSansKey,
    savedSerifKey,
    selectedSans: getUIFontByKey(uiFontSansKey) ?? savedSans,
    selectedSerif: getUIFontByKey(uiFontSerifKey) ?? savedSerif,
    dirty: savedSansKey !== uiFontSansKey || savedSerifKey !== uiFontSerifKey,
  };
}

export function buildApiKeyCardLabels(t: (key: string, fallback?: string) => string) {
  return {
    configured: t("settings.configured"),
    newApiKey: t("settings.newApiKey"),
    region: t("settings.azureSpeechRegionLabel"),
    saveOrUpdate: t("settings.saveOrUpdate"),
    saving: t("common.saving"),
    deleteKey: t("settings.deleteKey"),
    deleting: t("settings.deleting"),
  };
}

export function latestProviderModelUpdateDetectedAt(events: ProviderModelChangeEvent[]): string | null {
  return events.reduce<string | null>((max, event) => {
    if (!max) return event.detected_at;
    return Date.parse(event.detected_at) > Date.parse(max) ? event.detected_at : max;
  }, null);
}

export function dismissProviderModelUpdatesToLocalStorage(events: ProviderModelChangeEvent[]): string | null {
  if (typeof window === "undefined") return null;
  const latest = latestProviderModelUpdateDetectedAt(events);
  if (!latest) return null;
  window.localStorage.setItem(MODEL_UPDATES_DISMISSED_AT_KEY, latest);
  return latest;
}

export function restoreProviderModelUpdatesFromLocalStorage(): null {
  if (typeof window !== "undefined") {
    window.localStorage.removeItem(MODEL_UPDATES_DISMISSED_AT_KEY);
  }
  return null;
}
