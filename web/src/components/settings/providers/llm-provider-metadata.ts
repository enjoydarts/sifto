"use client";

import type { LLMCatalog, LLMCatalogModel } from "../../../types/api/model-catalog";
import type { UserSettings } from "../../../types/api/settings";
import { formatModelDisplayName } from "../../../lib/model-display";
import { getFeatherlessModelState } from "./featherless-model-state";

type Translate = (key: string, fallback?: string) => string;

export function formatUSDPerMTok(value: number): string {
  const rounded = value >= 1 ? value.toFixed(2) : value.toFixed(4);
  return `$${rounded.replace(/\.?0+$/, "")}`;
}

export function formatModelOptionNote(item: LLMCatalogModel): string | undefined {
  if (!item.pricing) return undefined;
  const parts: string[] = [];
  if (item.pricing.cache_read_per_mtok_usd > 0) {
    parts.push(`cached in ${formatUSDPerMTok(item.pricing.cache_read_per_mtok_usd)}`);
  }
  parts.push(`in ${formatUSDPerMTok(item.pricing.input_per_mtok_usd)}`);
  if (item.pricing.output_per_mtok_usd > 0) {
    parts.push(`out ${formatUSDPerMTok(item.pricing.output_per_mtok_usd)}`);
  }
  parts.push("1M tok");
  return parts.join(" / ");
}

export function inferProviderLabelFromModelID(modelID: string, t: Translate): string | null {
  if (modelID.startsWith("openrouter::")) {
    return t("settings.modelGuide.provider.openrouter", "OpenRouter");
  }
  if (modelID.startsWith("featherless::")) {
    return t("settings.modelGuide.provider.featherless", "Featherless.ai");
  }
  if (modelID.startsWith("deepinfra::") || modelID.startsWith("deepinfra/")) {
    return t("settings.modelGuide.provider.deepinfra", "DeepInfra");
  }
  if (modelID.startsWith("siliconflow::")) {
    return t("settings.modelGuide.provider.siliconflow", "SiliconFlow");
  }
  if (modelID.startsWith("minimax::") || modelID.startsWith("minimax/")) {
    return t("settings.modelGuide.provider.minimax", "MiniMax");
  }
  if (modelID.startsWith("mimo-v2-")) {
    return t("settings.modelGuide.provider.xiaomi_mimo_token_plan", "Xiaomi MiMo (TokenPlan)");
  }
  if (modelID.startsWith("together::")) {
    return t("settings.modelGuide.provider.together", "Together AI");
  }
  if (modelID.includes("/")) {
    const provider = modelID.split("/", 1)[0]?.toLowerCase();
    if (
      provider === "moonshotai" ||
      provider === "zai-org" ||
      provider === "liquidai" ||
      provider === "essentialai" ||
      provider === "openai" ||
      provider === "deepseek-ai" ||
      provider === "deepcogito" ||
      provider === "mistralai" ||
      provider === "meta-llama" ||
      provider === "google" ||
      provider === "qwen"
    ) {
      return t("settings.modelGuide.provider.together", "Together AI");
    }
  }
  return null;
}

export function formatProviderModelLabel(providerLabel: string | null | undefined, modelID: string): string {
  const modelLabel = formatModelDisplayName(modelID);
  return providerLabel ? `${providerLabel} / ${modelLabel}` : modelLabel;
}

export function firstMatchingModelId(models: LLMCatalogModel[], candidates: string[]): string {
  for (const candidate of candidates) {
    if (models.some((item) => item.id === candidate)) {
      return candidate;
    }
  }
  return "";
}

export function buildCostPerformancePreset(catalog: LLMCatalog | null): NonNullable<UserSettings["llm_models"]> {
  const chatModels = catalog?.chat_models ?? [];
  const embeddingModels = catalog?.embedding_models ?? [];
  const purposeModels = (purpose: string) =>
    chatModels.filter((item) => (item.available_purposes ?? []).includes(purpose));

  return {
    facts: firstMatchingModelId(purposeModels("facts"), [
      "openai/gpt-oss-20b",
      "gemini-2.5-flash-lite",
      "gpt-5.4-mini",
      "gpt-5-mini",
    ]),
    summary: firstMatchingModelId(purposeModels("summary"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5.4",
      "gpt-5",
    ]),
    digest_cluster: firstMatchingModelId(purposeModels("digest_cluster_draft"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5.4",
      "gpt-5",
    ]),
    digest: firstMatchingModelId(purposeModels("digest"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5.4",
      "gpt-5",
    ]),
    ask: firstMatchingModelId(purposeModels("ask"), [
      "openai/gpt-oss-20b",
      "gemini-2.5-flash",
      "gpt-5.4-mini",
      "gpt-5-mini",
    ]),
    tts_markup_preprocess_model: firstMatchingModelId(purposeModels("summary"), [
      "openai/gpt-oss-20b",
      "gemini-2.5-flash-lite",
      "gpt-5.4-mini",
      "gpt-5-mini",
    ]),
    source_suggestion: firstMatchingModelId(purposeModels("source_suggestion"), [
      "openai/gpt-oss-20b",
      "gemini-2.5-flash-lite",
      "gpt-5.4-mini",
      "gpt-5-mini",
    ]),
    facts_check: firstMatchingModelId(purposeModels("facts"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5.4",
      "gpt-5",
    ]),
    faithfulness_check: firstMatchingModelId(purposeModels("summary"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5.4",
      "gpt-5",
    ]),
    embedding: firstMatchingModelId(embeddingModels, [
      "text-embedding-3-small",
      "text-embedding-3-large",
    ]),
  };
}

export function localizeLLMSettingKey(settingKey: string, t: Translate): string {
  switch (settingKey) {
    case "facts":
      return t("settings.model.facts");
    case "facts_secondary":
      return t("settings.model.factsSecondary");
    case "facts_fallback":
      return t("settings.model.factsFallback");
    case "summary":
      return t("settings.model.summary");
    case "summary_secondary":
      return t("settings.model.summarySecondary");
    case "summary_fallback":
      return t("settings.model.summaryFallback");
    case "digest_cluster":
      return t("settings.model.digestCluster");
    case "digest":
      return t("settings.model.digest");
    case "ask":
      return t("settings.model.ask");
    case "source_suggestion":
      return t("settings.model.sourceSuggestion");
    case "facts_check":
      return t("settings.model.factsCheck");
    case "facts_check_fallback":
      return t("settings.model.factsCheckFallback");
    case "faithfulness_check":
      return t("settings.model.faithfulnessCheck");
    case "faithfulness_check_fallback":
      return t("settings.model.faithfulnessCheckFallback");
    case "navigator":
      return t("settings.model.navigator");
    case "navigator_fallback":
      return t("settings.model.navigatorFallback");
    case "ai_navigator_brief":
      return t("settings.model.aiNavigatorBrief");
    case "ai_navigator_brief_fallback":
      return t("settings.model.aiNavigatorBriefFallback");
    case "audio_briefing_script":
      return t("settings.model.audioBriefingScript");
    case "audio_briefing_script_fallback":
      return t("settings.model.audioBriefingScriptFallback");
    case "tts_markup_preprocess_model":
      return t("settings.model.ttsMarkupPreprocess");
    case "embedding":
      return t("settings.model.embeddings");
    default:
      return settingKey;
  }
}

export function localizeSettingsErrorMessage(raw: unknown, t: Translate): string {
  const message = String(raw);
  const capabilityMatch = message.match(/model missing required capability for ([a-z_]+)/);
  if (capabilityMatch) {
    return t("settings.error.modelMissingCapability").replace("{{field}}", localizeLLMSettingKey(capabilityMatch[1], t));
  }
  const invalidModelMatch = message.match(/invalid model for ([a-z_]+)/);
  if (invalidModelMatch) {
    return t("settings.error.invalidModelForPurpose").replace("{{field}}", localizeLLMSettingKey(invalidModelMatch[1], t));
  }
  if (message.includes("invalid embedding model")) {
    return t("settings.error.invalidEmbeddingModel");
  }
  return message;
}

export function isUnavailableOpenRouterModel(item: LLMCatalogModel): boolean {
  return item.provider === "openrouter" && item.capabilities?.supports_structured_output === false;
}

export function isUnavailableCatalogModel(item: LLMCatalogModel): boolean {
  if (isUnavailableOpenRouterModel(item)) return true;
  return item.provider === "featherless" && !getFeatherlessModelState(item).selectable;
}
