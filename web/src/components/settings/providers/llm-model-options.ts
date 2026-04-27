"use client";

import type { ProviderModelChangeEvent } from "../../../types/api/common";
import type { LLMCatalog, LLMCatalogModel } from "../../../types/api/model-catalog";
import type { UserSettings } from "../../../types/api/settings";
import {
  formatModelOptionNote,
  formatProviderModelLabel,
  inferProviderLabelFromModelID,
  isUnavailableCatalogModel,
  localizeLLMSettingKey,
} from "./llm-provider-metadata";
import { formatModelDisplayName, providerLabel as formatProviderLabel } from "../../../lib/model-display";
import { getFeatherlessModelState } from "./featherless-model-state";

type ModelOption = {
  value: string;
  label: string;
  selectedLabel?: string;
  note?: string;
  provider?: string;
  disabled?: boolean;
  badge?: string;
};

export function buildModelSelectLabels(t: (key: string, fallback?: string) => string) {
  return {
    defaultOption: t("settings.modelSelect.default"),
    searchPlaceholder: t("settings.modelSelect.searchPlaceholder"),
    noResults: t("settings.modelSelect.noResults"),
    providerAll: t("settings.modelSelect.providerAll"),
    modalChoose: t("settings.modelSelect.modalChoose"),
    close: t("common.close"),
    confirmTitle: t("settings.modelSelect.confirmTitle"),
    confirmYes: t("settings.modelSelect.confirmYes"),
    confirmNo: t("settings.modelSelect.confirmNo"),
    confirmSuffix: t("settings.modelSelect.confirmSuffix"),
    providerColumn: t("settings.modelSelect.providerColumn"),
    modelColumn: t("settings.modelSelect.modelColumn"),
    pricingColumn: t("settings.modelSelect.pricingColumn"),
  };
}

export function toModelOption(item: LLMCatalogModel, t: (key: string, fallback?: string) => string): ModelOption {
  const providerLabel = formatProviderLabel(item.provider);
  const featherlessState = getFeatherlessModelState(item);
  const unavailableOpenRouter = item.provider === "openrouter" && item.capabilities?.supports_structured_output === false;
  const badge =
    item.provider === "featherless"
      ? featherlessState.kind === "gated"
        ? t("settings.modelOption.badge.gated", "Gated")
        : featherlessState.kind === "unavailable"
          ? t("settings.modelOption.badge.unavailable", "Unavailable")
        : featherlessState.kind === "removed"
          ? t("settings.modelOption.badge.removed", "Removed")
          : null
      : unavailableOpenRouter
        ? t("settings.modelOption.badge.unavailable", "Unavailable")
        : null;

  return {
    value: item.id,
    label: formatModelDisplayName(item.id),
    selectedLabel: formatProviderModelLabel(providerLabel, item.id),
    note: formatModelOptionNote(item),
    provider: providerLabel,
    disabled: unavailableOpenRouter || (item.provider === "featherless" && !featherlessState.selectable),
    badge: badge ?? undefined,
  };
}

function withCurrentModelFallback(items: ModelOption[], currentValue: string | undefined, t: (key: string, fallback?: string) => string): ModelOption[] {
  if (!currentValue || items.some((item) => item.value === currentValue)) {
    return items;
  }
  const providerLabel = inferProviderLabelFromModelID(currentValue, t);
  const missingFeatherless = currentValue.startsWith("featherless::");
  return [
    {
      value: currentValue,
      label: formatModelDisplayName(currentValue),
      selectedLabel: formatProviderModelLabel(providerLabel, currentValue),
      provider: providerLabel ?? undefined,
      disabled: missingFeatherless,
      badge: missingFeatherless ? t("settings.modelOption.badge.removed", "Removed") : undefined,
    },
    ...items,
  ];
}

export function buildOptionsForPurpose(
  catalog: LLMCatalog | null | undefined,
  purpose: string,
  currentValue: string | undefined,
  t: (key: string, fallback?: string) => string,
): ModelOption[] {
  const items = (catalog?.chat_models ?? [])
    .filter((item) => {
      if (!(item.available_purposes ?? []).includes(purpose)) return false;
      if (item.id === currentValue) return true;
      if (item.provider === "featherless") return true;
      return !isUnavailableCatalogModel(item);
    })
    .map((item) => toModelOption(item, t));
  return withCurrentModelFallback(items, currentValue, t);
}

export function buildOptionsForChatModel(
  catalog: LLMCatalog | null | undefined,
  currentValue: string | undefined,
  t: (key: string, fallback?: string) => string,
): ModelOption[] {
  const items = (catalog?.chat_models ?? []).map((item) => toModelOption(item, t));
  return withCurrentModelFallback(items, currentValue, t);
}

export function buildUnavailableSelectedModelWarnings(
  catalog: LLMCatalog | null | undefined,
  llmModels: UserSettings["llm_models"] | null | undefined,
  t: (key: string, fallback?: string) => string,
): Array<{ key: string; label: string; modelLabel: string }> {
  const chatModels = catalog?.chat_models ?? [];
  const byID = new Map(chatModels.map((item) => [item.id, item] as const));
  const entries: Array<{ key: string; label: string; modelLabel: string }> = [];
  const candidates: Array<[string, string | null | undefined]> = [
    ["facts", llmModels?.facts],
    ["facts_fallback", llmModels?.facts_fallback],
    ["summary", llmModels?.summary],
    ["summary_fallback", llmModels?.summary_fallback],
    ["digest_cluster", llmModels?.digest_cluster],
    ["digest", llmModels?.digest],
    ["ask", llmModels?.ask],
    ["source_suggestion", llmModels?.source_suggestion],
    ["facts_check", llmModels?.facts_check],
    ["facts_check_fallback", llmModels?.facts_check_fallback],
    ["faithfulness_check", llmModels?.faithfulness_check],
    ["faithfulness_check_fallback", llmModels?.faithfulness_check_fallback],
    ["navigator", llmModels?.navigator],
    ["navigator_fallback", llmModels?.navigator_fallback],
    ["ai_navigator_brief", llmModels?.ai_navigator_brief],
    ["ai_navigator_brief_fallback", llmModels?.ai_navigator_brief_fallback],
  ];
  for (const [settingKey, modelID] of candidates) {
    if (!modelID) continue;
    const item = byID.get(modelID);
    if (!item) {
      if (modelID.startsWith("featherless::")) {
        entries.push({
          key: settingKey,
          label: localizeLLMSettingKey(settingKey, t),
          modelLabel: formatModelDisplayName(modelID),
        });
      }
      continue;
    }
    if (!isUnavailableCatalogModel(item)) continue;
    entries.push({
      key: settingKey,
      label: localizeLLMSettingKey(settingKey, t),
      modelLabel: formatModelDisplayName(modelID),
    });
  }
  return entries;
}

export function buildEmbeddingModelOptions(catalog: LLMCatalog | null | undefined, t: (key: string, fallback?: string) => string): ModelOption[] {
  return (catalog?.embedding_models ?? []).map((item) => toModelOption(item, t));
}

export function buildModelComparisonEntries(catalog: LLMCatalog | null | undefined): LLMCatalogModel[] {
  return [...(catalog?.chat_models ?? []), ...(catalog?.embedding_models ?? [])];
}

export function buildVisibleProviderModelUpdates(
  providerModelUpdates: ProviderModelChangeEvent[],
  dismissedModelUpdatesAt: string | null,
): ProviderModelChangeEvent[] {
  if (!dismissedModelUpdatesAt) return providerModelUpdates;
  const dismissedMs = Date.parse(dismissedModelUpdatesAt);
  if (Number.isNaN(dismissedMs)) return providerModelUpdates;
  return providerModelUpdates.filter((event) => Date.parse(event.detected_at) > dismissedMs);
}
