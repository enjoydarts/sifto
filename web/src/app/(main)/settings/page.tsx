"use client";

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { Brain, ChevronDown, KeyRound, RefreshCw, Search, Settings as SettingsIcon, X } from "lucide-react";
import { AivisModelSnapshot, AivisModelsResponse, AivisUserDictionary, api, AudioBriefingPersonaVoice, AudioBriefingPreset, FishModelSnapshot, GeminiTTSVoiceCatalogEntry, GeminiTTSVoicesResponse, LLMCatalog, LLMCatalogModel, NavigatorPersonaDefinition, NotificationPriorityRule, OpenAITTSVoiceSnapshot, OpenAITTSVoicesResponse, PodcastCategoryOption, PreferenceProfile, ProviderModelChangeEvent, SaveAudioBriefingPresetRequest, SummaryAudioVoiceSettings, UserSettings, XAIVoiceSnapshot, XAIVoicesResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";
import { getSummaryAudioReadiness } from "@/lib/summary-audio-readiness";
import OneSignalSettings from "@/components/onesignal-settings";
import ApiKeyCard from "@/components/settings/api-key-card";
import AivisVoicePickerModal from "@/components/settings/aivis-voice-picker-modal";
import FishVoicePickerModal from "@/components/settings/fish-voice-picker-modal";
import ModelGuideModal from "@/components/settings/model-guide-modal";
import ModelSelect, { type ModelOption } from "@/components/settings/model-select";
import { PreferenceProfilePanel } from "@/components/settings/preference-profile-panel";
import ProviderModelUpdatesPanel from "@/components/settings/provider-model-updates-panel";
import { AINavigatorAvatar } from "@/components/briefing/ai-navigator-avatar";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import { formatModelDisplayName } from "@/lib/model-display";

const MODEL_UPDATES_DISMISSED_AT_KEY = "provider-model-updates:dismissed-at";

function formatUSDPerMTok(value: number): string {
  const rounded = value >= 1 ? value.toFixed(2) : value.toFixed(4);
  return `$${rounded.replace(/\.?0+$/, "")}`;
}

function formatModelOptionNote(item: LLMCatalogModel): string | undefined {
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

function inferProviderLabelFromModelID(
  modelID: string,
  t: (key: string, fallback?: string) => string,
): string | null {
  if (modelID.startsWith("openrouter::")) {
    return t("settings.modelGuide.provider.openrouter", "OpenRouter");
  }
  if (modelID.startsWith("siliconflow::")) {
    return t("settings.modelGuide.provider.siliconflow", "SiliconFlow");
  }
  return null;
}

function tWithVars(
  t: (key: string, fallback?: string) => string,
  key: string,
  vars: Record<string, string | number>,
  fallback?: string,
): string {
  let message = t(key, fallback);
  for (const [name, value] of Object.entries(vars)) {
    message = message.replaceAll(`{{${name}}}`, String(value));
  }
  return message;
}

function formatProviderModelLabel(
  providerLabel: string | null | undefined,
  modelID: string,
): string {
  const modelLabel = formatModelDisplayName(modelID);
  return providerLabel ? `${providerLabel} / ${modelLabel}` : modelLabel;
}

function firstMatchingModelId(models: LLMCatalogModel[], candidates: string[]): string {
  for (const candidate of candidates) {
    if (models.some((item) => item.id === candidate)) {
      return candidate;
    }
  }
  return "";
}

function buildCostPerformancePreset(catalog: LLMCatalog | null): NonNullable<UserSettings["llm_models"]> {
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
    fish_preprocess_model: firstMatchingModelId(purposeModels("summary"), [
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

function localizeLLMSettingKey(settingKey: string, t: (key: string, fallback?: string) => string): string {
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
    case "faithfulness_check":
      return t("settings.model.faithfulnessCheck");
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
    case "fish_preprocess_model":
      return t("settings.model.fishPreprocess");
    case "embedding":
      return t("settings.model.embeddings");
    default:
      return settingKey;
  }
}

function localizeSettingsErrorMessage(raw: unknown, t: (key: string, fallback?: string) => string): string {
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

function localizePreferenceProfileErrorMessage(raw: unknown, t: (key: string, fallback?: string) => string): string {
  const message = String(raw instanceof Error ? raw.message : raw).replace(/^Error:\s*/, "").trim();
  if (message.startsWith("401:")) return t("settings.personalization.error.auth");
  if (message.startsWith("403:")) return t("settings.personalization.error.auth");
  if (message.startsWith("429:")) return t("settings.personalization.error.rateLimited");
  if (message.startsWith("500:")) return t("settings.personalization.error.server");
  if (!message) return t("settings.personalization.error.unknown");
  return t("settings.personalization.error.detail").replace("{{message}}", message);
}

function isUnavailableOpenRouterModel(item: LLMCatalogModel): boolean {
  return item.provider === "openrouter" && item.capabilities?.supports_structured_output === false;
}

type SettingsSectionID =
  | "audio-briefing"
  | "summary-audio"
  | "reading-plan"
  | "personalization"
  | "digest"
  | "notifications"
  | "integrations"
  | "models"
  | "navigator"
  | "budget"
  | "system";

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

function buildPodcastRSSURL(feedSlug: string | null | undefined, fallbackURL?: string | null): string {
  const slug = (feedSlug ?? "").trim();
  if (!slug) {
    return fallbackURL ?? "";
  }
  if (typeof window === "undefined") {
    return fallbackURL ?? "";
  }
  return `${window.location.origin}/podcasts/${encodeURIComponent(slug)}/feed.xml`;
}

type NavigatorPersonaKey = "editor" | "hype" | "analyst" | "concierge" | "snark" | "native" | "junior" | "urban";
const NAVIGATOR_PERSONA_KEYS: NavigatorPersonaKey[] = ["editor", "hype", "analyst", "concierge", "snark", "native", "junior", "urban"];
type AudioBriefingScheduleSelection = "interval3h" | "interval6h" | "fixed3x";
type AudioBriefingNumericInputField =
  | "speech_rate"
  | "tempo_dynamics"
  | "emotional_intensity"
  | "line_break_silence_seconds"
  | "aivis_volume"
  | "pitch"
  | "volume_gain";
type AudioBriefingVoiceInputDrafts = Record<string, Record<AudioBriefingNumericInputField, string>>;
type SummaryAudioNumericInputField =
  | "speech_rate"
  | "tempo_dynamics"
  | "emotional_intensity"
  | "line_break_silence_seconds"
  | "pitch"
  | "volume_gain";
type SummaryAudioVoiceInputDrafts = Record<SummaryAudioNumericInputField, string>;
type TTSProviderCapabilities = {
  requiresVoiceStyle: boolean;
  supportsCatalogPicker: boolean;
  supportsSeparateTTSModel: boolean;
  supportsSpeechTuning: boolean;
};
const EMPTY_NAVIGATOR_PERSONA: NavigatorPersonaDefinition = {
  name: "",
  gender: "",
  age_vibe: "",
  first_person: "",
  speech_style: "",
  occupation: "",
  experience: "",
  personality: "",
  values: "",
  interests: "",
  dislikes: "",
  voice: "",
};

function buildDefaultAudioBriefingVoices(personaKeys: NavigatorPersonaKey[]): AudioBriefingPersonaVoice[] {
  return personaKeys.map((persona) => ({
    persona,
    tts_provider: "aivis",
    tts_model: "",
    voice_model: "",
    voice_style: "",
    provider_voice_label: "",
    provider_voice_description: "",
    speech_rate: 1,
    emotional_intensity: 1,
    tempo_dynamics: 1,
    line_break_silence_seconds: 0.4,
    pitch: 0,
    volume_gain: 0,
  }));
}

function normalizeAudioBriefingPresetVoices(voices: AudioBriefingPersonaVoice[]): AudioBriefingPersonaVoice[] {
  const defaults = buildDefaultAudioBriefingVoices(NAVIGATOR_PERSONA_KEYS);
  const byPersona = new Map(voices.map((voice) => [voice.persona, voice] as const));
  return defaults.map((voice) => byPersona.get(voice.persona) ?? voice);
}

function formatAudioBriefingDecimalInput(value: number): string {
  if (!Number.isFinite(value)) return "";
  return value.toFixed(4).replace(/\.?0+$/, "");
}

function buildAudioBriefingPresetRequest(
  name: string,
  audioBriefingDefaultPersonaMode: "fixed" | "random",
  audioBriefingDefaultPersona: string,
  audioBriefingConversationMode: "single" | "duo",
  audioBriefingVoices: AudioBriefingPersonaVoice[],
): SaveAudioBriefingPresetRequest {
  return {
    name: name.trim(),
    default_persona_mode: audioBriefingDefaultPersonaMode,
    default_persona: audioBriefingDefaultPersona,
    conversation_mode: audioBriefingConversationMode,
    voices: audioBriefingVoices,
  };
}

function formatAudioBriefingPresetVoiceLabel(voice: AudioBriefingPersonaVoice, t: ReturnType<typeof useI18n>["t"]): string {
  const provider = voice.tts_provider.trim();
  const primary = voice.provider_voice_label?.trim() || voice.voice_model.trim();
  if (!primary) return t("settings.audioBriefing.unsetShort");
  return provider ? `${provider} / ${primary}` : primary;
}

function formatAudioBriefingPresetVoiceDetail(voice: AudioBriefingPersonaVoice, t: ReturnType<typeof useI18n>["t"]): string {
  return (
    voice.provider_voice_description?.trim() ||
    voice.voice_style.trim() ||
    voice.voice_model.trim() ||
    t("settings.audioBriefing.unsetShort")
  );
}

function buildAudioBriefingVoiceInputDrafts(voices: AudioBriefingPersonaVoice[]): AudioBriefingVoiceInputDrafts {
  return Object.fromEntries(
    voices.map((voice) => [
      voice.persona,
      {
        speech_rate: formatAudioBriefingDecimalInput(voice.speech_rate),
        tempo_dynamics: formatAudioBriefingDecimalInput(voice.tempo_dynamics),
        emotional_intensity: formatAudioBriefingDecimalInput(voice.emotional_intensity),
        line_break_silence_seconds: formatAudioBriefingDecimalInput(voice.line_break_silence_seconds),
        aivis_volume: formatAudioBriefingDecimalInput(voice.volume_gain + 1),
        pitch: formatAudioBriefingDecimalInput(voice.pitch),
        volume_gain: formatAudioBriefingDecimalInput(voice.volume_gain),
      },
    ])
  );
}

function buildDefaultSummaryAudioVoiceSettings(): SummaryAudioVoiceSettings {
  return {
    tts_provider: "aivis",
    tts_model: "",
    voice_model: "",
    voice_style: "",
    provider_voice_label: "",
    provider_voice_description: "",
    speech_rate: 1,
    emotional_intensity: 1,
    tempo_dynamics: 1,
    line_break_silence_seconds: 0.4,
    pitch: 0,
    volume_gain: 0,
    aivis_user_dictionary_uuid: null,
  };
}

function buildSummaryAudioVoiceInputDrafts(settings: SummaryAudioVoiceSettings): SummaryAudioVoiceInputDrafts {
  return {
    speech_rate: formatAudioBriefingDecimalInput(settings.speech_rate),
    tempo_dynamics: formatAudioBriefingDecimalInput(settings.tempo_dynamics),
    emotional_intensity: formatAudioBriefingDecimalInput(settings.emotional_intensity),
    line_break_silence_seconds: formatAudioBriefingDecimalInput(settings.line_break_silence_seconds),
    pitch: formatAudioBriefingDecimalInput(settings.pitch),
    volume_gain: formatAudioBriefingDecimalInput(settings.volume_gain),
  };
}

type VoiceModelSelection = {
  voice_model: string;
  provider_voice_label?: string;
  provider_voice_description?: string;
};

type VoiceStyleSelection = VoiceModelSelection & {
  voice_style: string;
};

function resolveAudioBriefingScheduleSelection(
  audioBriefing?: UserSettings["audio_briefing"] | null,
): AudioBriefingScheduleSelection {
  if (audioBriefing?.schedule_mode === "fixed_slots_3x") {
    return "fixed3x";
  }
  if (audioBriefing?.interval_hours === 3) {
    return "interval3h";
  }
  return "interval6h";
}

function buildAudioBriefingSchedulePayload(selection: AudioBriefingScheduleSelection): {
  schedule_mode: "interval" | "fixed_slots_3x";
  interval_hours: 3 | 6;
} {
  switch (selection) {
    case "interval3h":
      return { schedule_mode: "interval", interval_hours: 3 };
    case "fixed3x":
      return { schedule_mode: "fixed_slots_3x", interval_hours: 6 };
    case "interval6h":
    default:
      return { schedule_mode: "interval", interval_hours: 6 };
  }
}

function formatAudioBriefingScheduleSelection(
  selection: AudioBriefingScheduleSelection,
  t: (key: string, fallback?: string) => string,
): string {
  switch (selection) {
    case "interval3h":
      return t("settings.audioBriefing.interval3h");
    case "fixed3x":
      return t("settings.audioBriefing.fixed3x");
    case "interval6h":
    default:
      return t("settings.audioBriefing.interval6h");
  }
}

const TTS_PROVIDER_CAPABILITIES: Record<string, TTSProviderCapabilities> = {
  aivis: {
    requiresVoiceStyle: true,
    supportsCatalogPicker: true,
    supportsSeparateTTSModel: false,
    supportsSpeechTuning: true,
  },
  xai: {
    requiresVoiceStyle: false,
    supportsCatalogPicker: true,
    supportsSeparateTTSModel: false,
    supportsSpeechTuning: false,
  },
  fish: {
    requiresVoiceStyle: false,
    supportsCatalogPicker: true,
    supportsSeparateTTSModel: true,
    supportsSpeechTuning: false,
  },
  openai: {
    requiresVoiceStyle: false,
    supportsCatalogPicker: true,
    supportsSeparateTTSModel: true,
    supportsSpeechTuning: false,
  },
  gemini_tts: {
    requiresVoiceStyle: false,
    supportsCatalogPicker: true,
    supportsSeparateTTSModel: true,
    supportsSpeechTuning: false,
  },
  mock: {
    requiresVoiceStyle: false,
    supportsCatalogPicker: false,
    supportsSeparateTTSModel: false,
    supportsSpeechTuning: false,
  },
};

const OPENAI_TTS_MODEL_OPTIONS: ModelOption[] = [
  {
    value: "tts-1",
    label: "tts-1",
    selectedLabel: "OpenAI TTS / tts-1",
    note: "Standard quality",
    provider: "OpenAI TTS",
  },
  {
    value: "tts-1-hd",
    label: "tts-1-hd",
    selectedLabel: "OpenAI TTS / tts-1-hd",
    note: "Higher quality",
    provider: "OpenAI TTS",
  },
  {
    value: "gpt-4o-mini-tts",
    label: "gpt-4o-mini-tts",
    selectedLabel: "OpenAI TTS / gpt-4o-mini-tts",
    note: "Latest OpenAI TTS model",
    provider: "OpenAI TTS",
  },
];

const GEMINI_TTS_MODEL_OPTIONS: ModelOption[] = [
  {
    value: "gemini-2.5-flash-tts",
    label: "gemini-2.5-flash-tts",
    selectedLabel: "Gemini TTS / gemini-2.5-flash-tts",
    note: "Fast Gemini speech generation",
    provider: "Gemini TTS",
  },
  {
    value: "gemini-2.5-pro-tts",
    label: "gemini-2.5-pro-tts",
    selectedLabel: "Gemini TTS / gemini-2.5-pro-tts",
    note: "Higher quality Gemini speech generation",
    provider: "Gemini TTS",
  },
  {
    value: "gemini-2.5-flash-lite-preview-tts",
    label: "gemini-2.5-flash-lite-preview-tts",
    selectedLabel: "Gemini TTS / gemini-2.5-flash-lite-preview-tts",
    note: "Lowest latency Gemini speech generation",
    provider: "Gemini TTS",
  },
];

const FISH_TTS_MODEL_OPTIONS: ModelOption[] = [
  {
    value: "s2-pro",
    label: "s2-pro",
    selectedLabel: "Fish Audio / s2-pro",
    note: "Default Fish Audio dialogue-capable model",
    provider: "Fish Audio",
  },
];

function buildOpenAITTSModelOptions(currentValue: string): ModelOption[] {
  const trimmed = currentValue.trim();
  if (!trimmed || OPENAI_TTS_MODEL_OPTIONS.some((option) => option.value === trimmed)) {
    return OPENAI_TTS_MODEL_OPTIONS;
  }
  return [
    {
      value: trimmed,
      label: trimmed,
      selectedLabel: `OpenAI TTS / ${trimmed}`,
      provider: "OpenAI TTS",
    },
    ...OPENAI_TTS_MODEL_OPTIONS,
  ];
}

function buildGeminiTTSModelOptions(currentValue: string): ModelOption[] {
  const trimmed = currentValue.trim();
  if (!trimmed || GEMINI_TTS_MODEL_OPTIONS.some((option) => option.value === trimmed)) {
    return GEMINI_TTS_MODEL_OPTIONS;
  }
  return [
    {
      value: trimmed,
      label: trimmed,
      selectedLabel: `Gemini TTS / ${trimmed}`,
      provider: "Gemini TTS",
    },
    ...GEMINI_TTS_MODEL_OPTIONS,
  ];
}

function buildFishTTSModelOptions(currentValue: string): ModelOption[] {
  const trimmed = currentValue.trim();
  if (!trimmed || FISH_TTS_MODEL_OPTIONS.some((option) => option.value === trimmed)) {
    return FISH_TTS_MODEL_OPTIONS;
  }
  return [
    {
      value: trimmed,
      label: trimmed,
      selectedLabel: `Fish Audio / ${trimmed}`,
      provider: "Fish Audio",
    },
    ...FISH_TTS_MODEL_OPTIONS,
  ];
}

function isCompleteDecimalInput(raw: string): boolean {
  return /^-?(?:\d+\.?\d*|\.\d+)$/.test(raw);
}

function isSettingsSectionID(section: string | null): section is SettingsSectionID {
  return section === "summary-audio"
    || section === "audio-briefing"
    || section === "models"
    || section === "reading-plan"
    || section === "navigator"
    || section === "personalization"
    || section === "digest"
    || section === "notifications"
    || section === "integrations"
    || section === "budget"
    || section === "system";
}

function resolveAivisVoiceSelection(models: AivisModelSnapshot[], voice: VoiceStyleSelection) {
  const model = models.find((item) => item.aivm_model_uuid === voice.voice_model);
  if (!model) {
    return { model: null, speaker: null, style: null };
  }
  const [speakerUUID, styleIDRaw] = voice.voice_style.split(":");
  const styleID = Number(styleIDRaw);
  const speaker = (model.speakers_json ?? []).find((item) => item.aivm_speaker_uuid === speakerUUID) ?? null;
  const style = speaker?.styles.find((item) => item.local_id === styleID) ?? null;
  return { model, speaker, style };
}

function formatAivisVoiceStyleLabel(
  voiceStyle: string,
  t: (key: string, fallback?: string) => string
): string {
  const trimmed = voiceStyle.trim();
  if (!trimmed) {
    return t("settings.summaryAudio.unsetShort");
  }
  const [, styleIDRaw] = trimmed.split(":");
  if (styleIDRaw && /^\d+$/.test(styleIDRaw)) {
    return t("settings.aivisStyleLocalId").replace("{{id}}", styleIDRaw);
  }
  return trimmed;
}

function resolveXAIVoiceSelection(voices: XAIVoiceSnapshot[], voice: VoiceModelSelection) {
  return voices.find((item) => item.voice_id === voice.voice_model) ?? null;
}

function resolveFishVoiceSelection(models: FishModelSnapshot[], voice: VoiceModelSelection) {
  return models.find((item) => item._id === voice.voice_model) ?? null;
}

function resolveOpenAITTSVoiceSelection(voices: OpenAITTSVoiceSnapshot[], voice: VoiceModelSelection) {
  return voices.find((item) => item.voice_id === voice.voice_model) ?? null;
}

function resolveGeminiTTSVoiceSelection(voices: GeminiTTSVoiceCatalogEntry[], voice: VoiceModelSelection) {
  return voices.find((item) => item.voice_name === voice.voice_model) ?? null;
}

function getAudioBriefingProviderCapabilities(provider: string): TTSProviderCapabilities {
  return TTS_PROVIDER_CAPABILITIES[provider.trim().toLowerCase()] ?? {
    requiresVoiceStyle: true,
    supportsCatalogPicker: false,
    supportsSeparateTTSModel: false,
    supportsSpeechTuning: false,
  };
}

function isAudioBriefingVoiceConfigured(voice: AudioBriefingPersonaVoice) {
  const capabilities = getAudioBriefingProviderCapabilities(voice.tts_provider);
  if (!voice.voice_model.trim()) return false;
  if (capabilities.requiresVoiceStyle) return voice.voice_style.trim().length > 0;
  return true;
}

function getAudioBriefingVoiceStatus(
  voice: AudioBriefingPersonaVoice,
  models: AivisModelSnapshot[],
  fishModels: FishModelSnapshot[],
  xaiVoices: XAIVoiceSnapshot[],
  openAIVoices: OpenAITTSVoiceSnapshot[],
  geminiVoices: GeminiTTSVoiceCatalogEntry[],
  hasAivisAPIKey: boolean,
  hasFishAPIKey: boolean,
  hasXAIAPIKey: boolean,
  hasOpenAIAPIKey: boolean,
  geminiTTSEnabled: boolean,
  t: (key: string, fallback?: string) => string
) {
  const provider = voice.tts_provider.trim().toLowerCase();
  if (!isAudioBriefingVoiceConfigured(voice)) {
    return {
      tone: "warn" as const,
      label: t("settings.audioBriefing.status.unconfigured"),
      detail: t("settings.audioBriefing.status.unconfiguredDetail"),
      configured: false,
    };
  }
  if (provider === "openai") {
    const resolved = resolveOpenAITTSVoiceSelection(openAIVoices, voice);
    if (!voice.tts_model.trim()) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.openaiModelMissing"),
        detail: t("settings.audioBriefing.status.openaiModelMissingDetail"),
        configured: true,
      };
    }
    if (!hasOpenAIAPIKey) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.openaiApiKeyMissing"),
        detail: t("settings.audioBriefing.status.openaiApiKeyMissingDetail"),
        configured: true,
      };
    }
    if (!resolved) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.openaiVoiceMissing"),
        detail: t("settings.audioBriefing.status.openaiVoiceMissingDetail"),
        configured: true,
      };
    }
    return {
      tone: "ok" as const,
      label: t("settings.audioBriefing.status.openaiReady"),
      detail: t("settings.audioBriefing.status.openaiReadyDetail"),
      configured: true,
    };
  }
  if (provider === "gemini_tts") {
    const resolved = resolveGeminiTTSVoiceSelection(geminiVoices, voice);
    if (!voice.tts_model.trim()) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.geminiModelMissing"),
        detail: t("settings.audioBriefing.status.geminiModelMissingDetail"),
        configured: true,
      };
    }
    if (!geminiTTSEnabled) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.geminiNotAllowed"),
        detail: t("settings.audioBriefing.status.geminiNotAllowedDetail"),
        configured: true,
      };
    }
    if (!resolved) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.geminiVoiceMissing"),
        detail: t("settings.audioBriefing.status.geminiVoiceMissingDetail"),
        configured: true,
      };
    }
    return {
      tone: "ok" as const,
      label: t("settings.audioBriefing.status.geminiReady"),
      detail: t("settings.audioBriefing.status.geminiReadyDetail"),
      configured: true,
    };
  }
  if (provider === "xai") {
    const resolved = resolveXAIVoiceSelection(xaiVoices, voice);
    if (!hasXAIAPIKey) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.xaiApiKeyMissing"),
        detail: t("settings.audioBriefing.status.xaiApiKeyMissingDetail"),
        configured: true,
      };
    }
    if (!resolved) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.xaiVoiceMissing"),
        detail: t("settings.audioBriefing.status.xaiVoiceMissingDetail"),
        configured: true,
      };
    }
    return {
      tone: "ok" as const,
      label: t("settings.audioBriefing.status.xaiReady"),
      detail: t("settings.audioBriefing.status.xaiReadyDetail"),
      configured: true,
    };
  }
  if (provider === "fish") {
    if (!voice.tts_model.trim()) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.fishModelMissing"),
        detail: t("settings.audioBriefing.status.fishModelMissingDetail"),
        configured: true,
      };
    }
    if (!hasFishAPIKey) {
      return {
        tone: "warn" as const,
        label: t("settings.audioBriefing.status.fishApiKeyMissing"),
        detail: t("settings.audioBriefing.status.fishApiKeyMissingDetail"),
        configured: true,
      };
    }
    return {
      tone: "ok" as const,
      label: t("settings.audioBriefing.status.fishReady"),
      detail: t("settings.audioBriefing.status.fishReadyDetail"),
      configured: true,
    };
  }
  if (provider !== "aivis") {
    return {
      tone: "muted" as const,
      label: t("settings.audioBriefing.status.customProvider"),
      detail: t("settings.audioBriefing.status.customProviderDetail").replace("{{provider}}", voice.tts_provider),
      configured: true,
    };
  }
  const resolved = resolveAivisVoiceSelection(models, voice);
  if (!resolved.model) {
    return {
      tone: "warn" as const,
      label: t("settings.audioBriefing.status.modelMissing"),
      detail: t("settings.audioBriefing.status.modelMissingDetail"),
      configured: true,
    };
  }
  if (!resolved.speaker || !resolved.style) {
    return {
      tone: "warn" as const,
      label: t("settings.audioBriefing.status.styleMissing"),
      detail: t("settings.audioBriefing.status.styleMissingDetail"),
      configured: true,
    };
  }
  if (!hasAivisAPIKey) {
    return {
      tone: "warn" as const,
      label: t("settings.audioBriefing.status.apiKeyMissing"),
      detail: t("settings.audioBriefing.status.apiKeyMissingDetail"),
      configured: true,
    };
  }
  return {
    tone: "ok" as const,
    label: t("settings.audioBriefing.status.ready"),
    detail: t("settings.audioBriefing.status.readyDetail"),
    configured: true,
  };
}

function isSummaryAudioVoiceConfigured(voice: SummaryAudioVoiceSettings) {
  const capabilities = getAudioBriefingProviderCapabilities(voice.tts_provider);
  if (!voice.voice_model.trim()) {
    return false;
  }
  if (capabilities.supportsSeparateTTSModel && !voice.tts_model.trim()) {
    return false;
  }
  if (capabilities.requiresVoiceStyle && !voice.voice_style.trim()) {
    return false;
  }
  return true;
}

function getSummaryAudioVoiceStatus(
  voice: SummaryAudioVoiceSettings,
  models: AivisModelSnapshot[],
  fishModels: FishModelSnapshot[],
  xaiVoices: XAIVoiceSnapshot[],
  openAIVoices: OpenAITTSVoiceSnapshot[],
  geminiVoices: GeminiTTSVoiceCatalogEntry[],
  hasAivisAPIKey: boolean,
  hasFishAPIKey: boolean,
  hasXAIAPIKey: boolean,
  hasOpenAIAPIKey: boolean,
  geminiTTSEnabled: boolean,
  t: (key: string, fallback?: string) => string
) {
  const provider = voice.tts_provider.trim().toLowerCase();
  if (!isSummaryAudioVoiceConfigured(voice)) {
    return {
      tone: "warn" as const,
      label: t("settings.summaryAudio.status.unconfigured"),
      detail: t("settings.summaryAudio.status.unconfiguredDetail"),
      configured: false,
    };
  }
  if (provider === "openai") {
    const resolved = resolveOpenAITTSVoiceSelection(openAIVoices, voice);
    if (!voice.tts_model.trim()) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.openaiModelMissing"),
        detail: t("settings.summaryAudio.status.openaiModelMissingDetail"),
        configured: true,
      };
    }
    if (!hasOpenAIAPIKey) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.openaiApiKeyMissing"),
        detail: t("settings.summaryAudio.status.openaiApiKeyMissingDetail"),
        configured: true,
      };
    }
    if (!resolved) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.openaiVoiceMissing"),
        detail: t("settings.summaryAudio.status.openaiVoiceMissingDetail"),
        configured: true,
      };
    }
    return {
      tone: "ok" as const,
      label: t("settings.summaryAudio.status.openaiReady"),
      detail: t("settings.summaryAudio.status.openaiReadyDetail"),
      configured: true,
    };
  }
  if (provider === "gemini_tts") {
    const resolved = resolveGeminiTTSVoiceSelection(geminiVoices, voice);
    if (!voice.tts_model.trim()) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.geminiModelMissing"),
        detail: t("settings.summaryAudio.status.geminiModelMissingDetail"),
        configured: true,
      };
    }
    if (!geminiTTSEnabled) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.geminiNotAllowed"),
        detail: t("settings.summaryAudio.status.geminiNotAllowedDetail"),
        configured: true,
      };
    }
    if (!resolved) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.geminiVoiceMissing"),
        detail: t("settings.summaryAudio.status.geminiVoiceMissingDetail"),
        configured: true,
      };
    }
    return {
      tone: "ok" as const,
      label: t("settings.summaryAudio.status.geminiReady"),
      detail: t("settings.summaryAudio.status.geminiReadyDetail"),
      configured: true,
    };
  }
  if (provider === "xai") {
    const resolved = resolveXAIVoiceSelection(xaiVoices, voice);
    if (!hasXAIAPIKey) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.xaiApiKeyMissing"),
        detail: t("settings.summaryAudio.status.xaiApiKeyMissingDetail"),
        configured: true,
      };
    }
    if (!resolved) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.xaiVoiceMissing"),
        detail: t("settings.summaryAudio.status.xaiVoiceMissingDetail"),
        configured: true,
      };
    }
    return {
      tone: "ok" as const,
      label: t("settings.summaryAudio.status.xaiReady"),
      detail: t("settings.summaryAudio.status.xaiReadyDetail"),
      configured: true,
    };
  }
  if (provider === "fish") {
    if (!voice.tts_model.trim()) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.fishModelMissing"),
        detail: t("settings.summaryAudio.status.fishModelMissingDetail"),
        configured: true,
      };
    }
    if (!hasFishAPIKey) {
      return {
        tone: "warn" as const,
        label: t("settings.summaryAudio.status.fishApiKeyMissing"),
        detail: t("settings.summaryAudio.status.fishApiKeyMissingDetail"),
        configured: true,
      };
    }
    return {
      tone: "ok" as const,
      label: t("settings.summaryAudio.status.fishReady"),
      detail: t("settings.summaryAudio.status.fishReadyDetail"),
      configured: true,
    };
  }
  if (provider !== "aivis") {
    return {
      tone: "muted" as const,
      label: t("settings.summaryAudio.status.customProvider"),
      detail: t("settings.summaryAudio.status.customProviderDetail").replace("{{provider}}", voice.tts_provider),
      configured: true,
    };
  }
  const resolved = resolveAivisVoiceSelection(models, voice);
  if (!resolved.model) {
    return {
      tone: "warn" as const,
      label: t("settings.summaryAudio.status.modelMissing"),
      detail: t("settings.summaryAudio.status.modelMissingDetail"),
      configured: true,
    };
  }
  if (!resolved.speaker || !resolved.style) {
    return {
      tone: "warn" as const,
      label: t("settings.summaryAudio.status.styleMissing"),
      detail: t("settings.summaryAudio.status.styleMissingDetail"),
      configured: true,
    };
  }
  if (!hasAivisAPIKey) {
    return {
      tone: "warn" as const,
      label: t("settings.summaryAudio.status.apiKeyMissing"),
      detail: t("settings.summaryAudio.status.apiKeyMissingDetail"),
      configured: true,
    };
  }
  if (!voice.aivis_user_dictionary_uuid?.trim()) {
    return {
      tone: "warn" as const,
      label: t("settings.summaryAudio.status.aivisDictionaryMissing"),
      detail: t("settings.summaryAudio.status.aivisDictionaryMissingDetail"),
      configured: true,
    };
  }
  return {
    tone: "ok" as const,
    label: t("settings.summaryAudio.status.ready"),
    detail: t("settings.summaryAudio.status.readyDetail"),
    configured: true,
  };
}

type AudioBriefingPresetSaveModalProps = {
  open: boolean;
  saving: boolean;
  presetName: string;
  defaultPersonaMode: "fixed" | "random";
  defaultPersona: string;
  conversationMode: "single" | "duo";
  voices: AudioBriefingPersonaVoice[];
  onClose: () => void;
  onChangeName: (value: string) => void;
  onSave: () => void;
};

function AudioBriefingPresetSaveModal({
  open,
  saving,
  presetName,
  defaultPersonaMode,
  defaultPersona,
  conversationMode,
  voices,
  onClose,
  onChangeName,
  onSave,
}: AudioBriefingPresetSaveModalProps) {
  const { t } = useI18n();

  const configuredCount = useMemo(
    () => voices.filter((voice) => voice.voice_model.trim().length > 0).length,
    [voices],
  );

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-3xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.presetSaveTitle")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetSaveSubtitle")}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            aria-label={t("common.close")}
          >
            <X className="size-4" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-5">
          <div className="space-y-5">
          <label className="block">
            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.presetName")}</div>
            <input
              value={presetName}
              onChange={(e) => onChangeName(e.target.value)}
              placeholder={t("settings.audioBriefing.presetNamePlaceholder")}
              className="mt-2 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-white px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
            <p className="mt-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetNameHelp")}</p>
          </label>

          <div className="grid gap-3 sm:grid-cols-3">
            <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.defaultPersona")}
              </div>
              <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                {defaultPersonaMode === "random"
                  ? t("settings.personaMode.random")
                  : t(`settings.navigator.persona.${defaultPersona}`, defaultPersona)}
              </div>
            </div>
            <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.conversationMode")}
              </div>
              <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                {t(`settings.audioBriefing.conversationMode.${conversationMode}`)}
              </div>
            </div>
            <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.audioBriefing.summary.configured")}
              </div>
              <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                {`${configuredCount}/${voices.length}`}
              </div>
            </div>
          </div>

          <div className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
            <div className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.audioBriefing.presetSaveIncludes")}
            </div>
            <div className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetSaveIncludesHelp")}</div>
            <div className="mt-4 grid gap-2 sm:grid-cols-2">
              {voices.map((voice) => (
                <div key={voice.persona} className="rounded-[16px] border border-[var(--color-editorial-line)] bg-white px-4 py-3">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">
                    {t(`settings.navigator.persona.${voice.persona}`, voice.persona)}
                  </div>
                  <div className="mt-1 text-sm font-medium text-[var(--color-editorial-ink)]">
                    {formatAudioBriefingPresetVoiceLabel(voice, t)}
                  </div>
                  <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                    {formatAudioBriefingPresetVoiceDetail(voice, t)}
                  </div>
                </div>
              ))}
            </div>
          </div>
          </div>
        </div>

        <div className="shrink-0 flex flex-wrap items-center justify-end gap-2 border-t border-[var(--color-editorial-line)] px-5 py-4">
          <button
            type="button"
            onClick={onClose}
            className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
          >
            {t("common.cancel")}
          </button>
          <button
            type="button"
            onClick={onSave}
            disabled={saving || !presetName.trim()}
            className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {saving ? t("common.saving") : t("settings.audioBriefing.presetSaveConfirm")}
          </button>
        </div>
      </div>
    </div>
  );
}

type AudioBriefingPresetApplyModalProps = {
  open: boolean;
  loading: boolean;
  error: string | null;
  presets: AudioBriefingPreset[];
  selectedPresetID: string | null;
  onClose: () => void;
  onRefresh: () => void;
  onSelectPreset: (presetID: string) => void;
  onApplyPreset: (preset: AudioBriefingPreset) => void;
};

function AudioBriefingPresetApplyModal({
  open,
  loading,
  error,
  presets,
  selectedPresetID,
  onClose,
  onRefresh,
  onSelectPreset,
  onApplyPreset,
}: AudioBriefingPresetApplyModalProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");

  const filteredPresets = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return presets;
    return presets.filter((preset) =>
      [preset.name, preset.default_persona_mode, preset.default_persona, preset.conversation_mode, ...preset.voices.flatMap((voice) => [
        voice.persona,
        voice.tts_provider,
        voice.tts_model,
        voice.voice_model,
        voice.voice_style,
        voice.provider_voice_label ?? "",
        voice.provider_voice_description ?? "",
      ])]
        .join(" ")
        .toLowerCase()
        .includes(q)
    );
  }, [presets, query]);

  useEffect(() => {
    if (open) {
      setQuery("");
    }
  }, [open]);

  const selectedPreset = useMemo(() => {
    if (!filteredPresets.length) return null;
    return filteredPresets.find((preset) => preset.id === selectedPresetID) ?? filteredPresets[0];
  }, [filteredPresets, selectedPresetID]);

  useEffect(() => {
    if (!open || selectedPreset) return;
    if (filteredPresets[0]) {
      onSelectPreset(filteredPresets[0].id);
    }
  }, [filteredPresets, onSelectPreset, open, selectedPreset]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.presetApplyTitle")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetApplySubtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={onRefresh}
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              {t("common.refresh")}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              aria-label={t("common.close")}
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.9fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] px-5 py-5 lg:border-b-0 lg:border-r">
            <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
              <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("settings.audioBriefing.presetSearchPlaceholder")}
                className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
              />
            </div>

            {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}

            <div className="mt-4 space-y-2">
              {loading ? (
                <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white px-4 py-6 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                  {t("common.loading")}
                </div>
              ) : filteredPresets.length === 0 ? (
                <div className="rounded-[18px] border border-dashed border-[var(--color-editorial-line)] bg-white/80 px-4 py-6 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                  {presets.length === 0 ? t("settings.audioBriefing.presetEmpty") : t("settings.audioBriefing.presetNoResults")}
                </div>
              ) : (
                filteredPresets.map((preset) => {
                  const active = preset.id === selectedPreset?.id;
                  return (
                    <button
                      key={preset.id}
                      type="button"
                      onClick={() => onSelectPreset(preset.id)}
                      className={joinClassNames(
                        "w-full rounded-[18px] border px-4 py-4 text-left transition",
                        active
                          ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-panel-strong)] shadow-[var(--shadow-card)]"
                          : "border-[var(--color-editorial-line)] bg-white hover:bg-[var(--color-editorial-panel-strong)]"
                      )}
                    >
                      <div className="flex flex-wrap items-start justify-between gap-2">
                        <div>
                          <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{preset.name}</div>
                          <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                            {t(`settings.audioBriefing.conversationMode.${preset.conversation_mode === "duo" ? "duo" : "single"}`)}
                          </div>
                        </div>
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-soft)]">
                          {preset.default_persona_mode === "random"
                            ? t("settings.personaMode.random")
                            : t(`settings.navigator.persona.${preset.default_persona}`, preset.default_persona)}
                        </span>
                      </div>
                      <div className="mt-3 flex flex-wrap gap-2 text-[12px] text-[var(--color-editorial-ink-soft)]">
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1">
                          {`${preset.voices.filter((voice) => voice.voice_model.trim()).length}/${preset.voices.length} ${t("settings.audioBriefing.summary.configured")}`}
                        </span>
                        <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1">
                          {preset.updated_at ? new Date(preset.updated_at).toLocaleString() : t("common.unknown")}
                        </span>
                      </div>
                    </button>
                  );
                })
              )}
            </div>
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {selectedPreset ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                    {t("settings.audioBriefing.presetSelected")}
                  </div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedPreset.name}</h3>
                  <p className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.presetApplyHelp")}</p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.audioBriefing.defaultPersona")}
                    </div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                      {selectedPreset.default_persona_mode === "random"
                        ? t("settings.personaMode.random")
                        : t(`settings.navigator.persona.${selectedPreset.default_persona}`, selectedPreset.default_persona)}
                    </div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.audioBriefing.conversationMode")}
                    </div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                      {t(`settings.audioBriefing.conversationMode.${selectedPreset.conversation_mode === "duo" ? "duo" : "single"}`)}
                    </div>
                  </div>
                </div>

                <div className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                  <div className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                    {t("settings.audioBriefing.presetVoices")}
                  </div>
                  <div className="mt-3 space-y-2">
                    {normalizeAudioBriefingPresetVoices(selectedPreset.voices).map((voice) => (
                      <div key={voice.persona} className="rounded-[16px] border border-[var(--color-editorial-line)] bg-white px-4 py-3">
                        <div className="flex flex-wrap items-start justify-between gap-2">
                          <div>
                            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">
                              {t(`settings.navigator.persona.${voice.persona}`, voice.persona)}
                            </div>
                            <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                              {formatAudioBriefingPresetVoiceLabel(voice, t)}
                            </div>
                          </div>
                          <div className="text-[12px] text-[var(--color-editorial-ink-soft)]">
                            {formatAudioBriefingPresetVoiceDetail(voice, t)}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => onApplyPreset(selectedPreset)}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("settings.audioBriefing.presetApplyConfirm")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.presetApplyEmpty")}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

type XAIVoicePickerModalProps = {
  open: boolean;
  loading: boolean;
  syncing: boolean;
  error: string | null;
  voices: XAIVoiceSnapshot[];
  currentVoiceID: string;
  onClose: () => void;
  onSync: () => Promise<void> | void;
  onSelect: (selection: { voice_id: string }) => void;
};

function XAIVoicePickerModal({
  open,
  loading,
  syncing,
  error,
  voices,
  currentVoiceID,
  onClose,
  onSync,
  onSelect,
}: XAIVoicePickerModalProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [selectedVoiceID, setSelectedVoiceID] = useState<string | null>(null);

  const filteredVoices = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return voices;
    return voices.filter((voice) =>
      [voice.voice_id, voice.name, voice.description, voice.language]
        .join(" ")
        .toLowerCase()
        .includes(q)
    );
  }, [query, voices]);

  const selectedVoice = useMemo(() => {
    const activeVoiceID = selectedVoiceID ?? currentVoiceID;
    return (
      filteredVoices.find((voice) => voice.voice_id === activeVoiceID) ??
      voices.find((voice) => voice.voice_id === activeVoiceID) ??
      null
    );
  }, [currentVoiceID, filteredVoices, selectedVoiceID, voices]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.xaiPickerTitle")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.xaiPickerSubtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => void onSync()}
              disabled={syncing}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
            >
              <RefreshCw className={`size-4 ${syncing ? "animate-spin" : ""}`} />
              {syncing ? t("common.saving") : t("settings.audioBriefing.refreshXaiCatalog")}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              aria-label={t("common.close")}
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        <div className="border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
            <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("settings.audioBriefing.xaiPickerSearch")}
              className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
          </div>
          {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.85fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] lg:border-b-0 lg:border-r">
            <div className="overflow-x-auto">
              <table className="min-w-[760px] divide-y divide-[var(--color-editorial-line)] text-sm">
                <thead className="bg-[var(--color-editorial-panel-strong)]">
                  <tr className="text-left text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                    <th className="px-4 py-3">{t("settings.audioBriefing.xaiTable.voice")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.xaiTable.language")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.xaiTable.description")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-editorial-line)] bg-white">
                  {loading ? (
                    <tr>
                      <td colSpan={3} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("common.loading")}
                      </td>
                    </tr>
                  ) : filteredVoices.length === 0 ? (
                    <tr>
                      <td colSpan={3} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.xaiPickerNoResults")}
                      </td>
                    </tr>
                  ) : (
                    filteredVoices.map((voice) => (
                      <tr
                        key={voice.voice_id}
                        className={`cursor-pointer transition hover:bg-[var(--color-editorial-panel)] ${selectedVoice?.voice_id === voice.voice_id ? "bg-[var(--color-editorial-panel)]" : ""}`}
                        onClick={() => setSelectedVoiceID(voice.voice_id)}
                      >
                        <td className="px-4 py-3">
                          <div className="font-medium text-[var(--color-editorial-ink)]">{voice.name || voice.voice_id}</div>
                          <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{voice.voice_id}</div>
                        </td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink)]">{voice.language || "—"}</td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.description || "—"}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {selectedVoice ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.xaiPickerSelected")}</div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.name || selectedVoice.voice_id}</h3>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedVoice.description || t("settings.audioBriefing.xaiPickerNoDescription")}</p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.xaiTable.language")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.language || "—"}</div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.xaiTable.voice")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.voice_id}</div>
                  </div>
                </div>

                {selectedVoice.preview_url ? (
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.xaiPreview")}</div>
                    <div className="mt-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                      <audio controls preload="none" className="w-full" src={selectedVoice.preview_url} />
                    </div>
                  </div>
                ) : null}

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onSelect({ voice_id: selectedVoice.voice_id });
                      onClose();
                    }}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("settings.audioBriefing.xaiPickerSelect")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.xaiPickerEmptySelection")}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

type OpenAITTSVoicePickerModalProps = {
  open: boolean;
  loading: boolean;
  syncing: boolean;
  error: string | null;
  voices: OpenAITTSVoiceSnapshot[];
  currentVoiceID: string;
  onClose: () => void;
  onSync: () => Promise<void> | void;
  onSelect: (selection: { voice_id: string }) => void;
};

function parseOpenAITTSMetadata(metadataJSON?: string | null): Record<string, unknown> | null {
  if (!metadataJSON) return null;
  try {
    const parsed = JSON.parse(metadataJSON);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return null;
    return parsed as Record<string, unknown>;
  } catch {
    return null;
  }
}

function getOpenAITTSSupportedModels(voice: OpenAITTSVoiceSnapshot): string[] {
  const metadata = parseOpenAITTSMetadata(voice.metadata_json);
  const supportedModels = metadata?.supported_models;
  if (!Array.isArray(supportedModels)) return [];
  return supportedModels.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
}

function OpenAITTSVoicePickerModal({
  open,
  loading,
  syncing,
  error,
  voices,
  currentVoiceID,
  onClose,
  onSync,
  onSelect,
}: OpenAITTSVoicePickerModalProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [selectedVoiceID, setSelectedVoiceID] = useState<string | null>(null);

  const filteredVoices = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return voices;
    return voices.filter((voice) =>
      [voice.voice_id, voice.name, voice.description, voice.language, getOpenAITTSSupportedModels(voice).join(" ")]
        .join(" ")
        .toLowerCase()
        .includes(q)
    );
  }, [query, voices]);

  const selectedVoice = useMemo(() => {
    const activeVoiceID = selectedVoiceID ?? currentVoiceID;
    return (
      filteredVoices.find((voice) => voice.voice_id === activeVoiceID) ??
      voices.find((voice) => voice.voice_id === activeVoiceID) ??
      null
    );
  }, [currentVoiceID, filteredVoices, selectedVoiceID, voices]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.openAITTSPickerTitle")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.openAITTSPickerSubtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <Link
              href="/openai-tts-voices"
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              {t("settings.audioBriefing.openOpenAITTSVoices")}
            </Link>
            <button
              type="button"
              onClick={() => void onSync()}
              disabled={syncing}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
            >
              <RefreshCw className={`size-4 ${syncing ? "animate-spin" : ""}`} />
              {syncing ? t("settings.audioBriefing.syncingOpenAITTSCatalog") : t("settings.audioBriefing.refreshOpenAITTSCatalog")}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              aria-label={t("common.close")}
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        <div className="border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
            <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("settings.audioBriefing.openAITTSPickerSearch")}
              className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
          </div>
          {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.85fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] lg:border-b-0 lg:border-r">
            <div className="overflow-x-auto">
              <table className="min-w-[900px] divide-y divide-[var(--color-editorial-line)] text-sm">
                <thead className="bg-[var(--color-editorial-panel-strong)]">
                  <tr className="text-left text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                    <th className="px-4 py-3">{t("settings.audioBriefing.openAITTSVoiceTable.voice")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.openAITTSVoiceTable.language")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.openAITTSVoiceTable.models")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.openAITTSVoiceTable.description")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-editorial-line)] bg-white">
                  {loading ? (
                    <tr>
                      <td colSpan={4} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("common.loading")}
                      </td>
                    </tr>
                  ) : filteredVoices.length === 0 ? (
                    <tr>
                      <td colSpan={4} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.openAITTSPickerNoResults")}
                      </td>
                    </tr>
                  ) : (
                    filteredVoices.map((voice) => {
                      const supportedModels = getOpenAITTSSupportedModels(voice);
                      return (
                        <tr
                          key={voice.voice_id}
                          className={`cursor-pointer transition hover:bg-[var(--color-editorial-panel)] ${selectedVoice?.voice_id === voice.voice_id ? "bg-[var(--color-editorial-panel)]" : ""}`}
                          onClick={() => setSelectedVoiceID(voice.voice_id)}
                        >
                          <td className="px-4 py-3">
                            <div className="font-medium text-[var(--color-editorial-ink)]">{voice.name || voice.voice_id}</div>
                            <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{voice.voice_id}</div>
                          </td>
                          <td className="px-4 py-3 text-[var(--color-editorial-ink)]">{voice.language || "—"}</td>
                          <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">
                            {supportedModels.length > 0 ? supportedModels.join(", ") : "—"}
                          </td>
                          <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.description || "—"}</td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {selectedVoice ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.openAITTSPickerSelected")}</div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.name || selectedVoice.voice_id}</h3>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    {selectedVoice.description || t("settings.audioBriefing.openAITTSPickerNoDescription")}
                  </p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.openAITTSVoiceTable.language")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.language || "—"}</div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.openAITTSVoiceTable.voice")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.voice_id}</div>
                  </div>
                </div>

                <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.openAITTSVoiceTable.models")}</div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {getOpenAITTSSupportedModels(selectedVoice).length > 0 ? (
                      getOpenAITTSSupportedModels(selectedVoice).map((model) => (
                        <span
                          key={`${selectedVoice.voice_id}-${model}`}
                          className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1 text-xs text-[var(--color-editorial-ink-soft)]"
                        >
                          {model}
                        </span>
                      ))
                    ) : (
                      <span className="text-sm text-[var(--color-editorial-ink-soft)]">—</span>
                    )}
                  </div>
                </div>

                {selectedVoice.preview_url ? (
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.openAITTSPreview")}</div>
                    <div className="mt-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                      <audio controls preload="none" className="w-full" src={selectedVoice.preview_url} />
                    </div>
                  </div>
                ) : null}

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onSelect({ voice_id: selectedVoice.voice_id });
                      onClose();
                    }}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("settings.audioBriefing.openAITTSPickerSelect")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.openAITTSPickerEmptySelection")}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

type GeminiTTSVoicePickerModalProps = {
  open: boolean;
  loading: boolean;
  error: string | null;
  voices: GeminiTTSVoiceCatalogEntry[];
  currentVoiceName: string;
  onClose: () => void;
  onRefresh: () => Promise<void> | void;
  onSelect: (selection: { voice_name: string }) => void;
};

function GeminiTTSVoicePickerModal({
  open,
  loading,
  error,
  voices,
  currentVoiceName,
  onClose,
  onRefresh,
  onSelect,
}: GeminiTTSVoicePickerModalProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [selectedVoiceName, setSelectedVoiceName] = useState<string | null>(null);

  const filteredVoices = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return voices;
    return voices.filter((voice) =>
      [voice.voice_name, voice.label, voice.tone, voice.description]
        .join(" ")
        .toLowerCase()
        .includes(q)
    );
  }, [query, voices]);

  const selectedVoice = useMemo(() => {
    const activeVoiceName = selectedVoiceName ?? currentVoiceName;
    return (
      filteredVoices.find((voice) => voice.voice_name === activeVoiceName) ??
      voices.find((voice) => voice.voice_name === activeVoiceName) ??
      null
    );
  }, [currentVoiceName, filteredVoices, selectedVoiceName, voices]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6" onClick={onClose}>
      <div
        className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] shadow-[0_30px_80px_rgba(35,24,12,0.24)]"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.geminiTTSPickerTitle")}</h2>
            <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.geminiTTSPickerSubtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <Link
              href="/gemini-tts-voices"
              className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              {t("settings.audioBriefing.openGeminiTTSVoices")}
            </Link>
            <button
              type="button"
              onClick={() => void onRefresh()}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              <RefreshCw className="size-4" />
              {t("common.refresh")}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              aria-label={t("common.close")}
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        <div className="border-b border-[var(--color-editorial-line)] px-5 py-4">
          <div className="flex items-center gap-3 rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-3">
            <Search className="size-4 text-[var(--color-editorial-ink-soft)]" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("settings.audioBriefing.geminiTTSPickerSearch")}
              className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
            />
          </div>
          {error ? <p className="mt-3 text-sm text-red-600">{error}</p> : null}
        </div>

        <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.85fr)]">
          <div className="min-h-0 overflow-auto border-b border-[var(--color-editorial-line)] lg:border-b-0 lg:border-r">
            <div className="overflow-x-auto">
              <table className="min-w-[760px] divide-y divide-[var(--color-editorial-line)] text-sm">
                <thead className="bg-[var(--color-editorial-panel-strong)]">
                  <tr className="text-left text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                    <th className="px-4 py-3">{t("settings.audioBriefing.geminiTTSVoiceTable.voice")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.geminiTTSVoiceTable.tone")}</th>
                    <th className="px-4 py-3">{t("settings.audioBriefing.geminiTTSVoiceTable.description")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-editorial-line)] bg-white">
                  {loading ? (
                    <tr>
                      <td colSpan={3} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("common.loading")}
                      </td>
                    </tr>
                  ) : filteredVoices.length === 0 ? (
                    <tr>
                      <td colSpan={3} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.geminiTTSPickerNoResults")}
                      </td>
                    </tr>
                  ) : (
                    filteredVoices.map((voice) => (
                      <tr
                        key={voice.voice_name}
                        className={`cursor-pointer transition hover:bg-[var(--color-editorial-panel)] ${selectedVoice?.voice_name === voice.voice_name ? "bg-[var(--color-editorial-panel)]" : ""}`}
                        onClick={() => setSelectedVoiceName(voice.voice_name)}
                      >
                        <td className="px-4 py-3">
                          <div className="font-medium text-[var(--color-editorial-ink)]">{voice.label || voice.voice_name}</div>
                          <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{voice.voice_name}</div>
                        </td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink)]">{voice.tone || "—"}</td>
                        <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{voice.description || "—"}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <div className="min-h-0 overflow-auto px-5 py-5">
            {selectedVoice ? (
              <div className="space-y-5">
                <div>
                  <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.geminiTTSPickerSelected")}</div>
                  <h3 className="mt-2 text-lg font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.label || selectedVoice.voice_name}</h3>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    {selectedVoice.description || t("settings.audioBriefing.geminiTTSPickerNoDescription")}
                  </p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.geminiTTSVoiceTable.voice")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.voice_name}</div>
                  </div>
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.geminiTTSVoiceTable.tone")}</div>
                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedVoice.tone || "—"}</div>
                  </div>
                </div>

                {selectedVoice.sample_audio_path ? (
                  <div>
                    <div className="text-xs uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">{t("settings.audioBriefing.geminiTTSPreview")}</div>
                    <div className="mt-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-white p-4">
                      <audio controls preload="none" className="w-full" src={selectedVoice.sample_audio_path} />
                    </div>
                  </div>
                ) : null}

                <div className="flex justify-end">
                  <button
                    type="button"
                    onClick={() => {
                      onSelect({ voice_name: selectedVoice.voice_name });
                      onClose();
                    }}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                  >
                    {t("settings.audioBriefing.geminiTTSPickerSelect")}
                  </button>
                </div>
              </div>
            ) : (
              <div className="rounded-[22px] border border-dashed border-[var(--color-editorial-line)] bg-white/70 px-5 py-8 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("settings.audioBriefing.geminiTTSPickerEmptySelection")}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

export default function SettingsPage() {
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const searchParams = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [savingAudioBriefing, setSavingAudioBriefing] = useState(false);
  const [savingPodcast, setSavingPodcast] = useState(false);
  const [uploadingPodcastArtwork, setUploadingPodcastArtwork] = useState(false);
  const [savingAudioBriefingVoices, setSavingAudioBriefingVoices] = useState(false);
  const [savingBudget, setSavingBudget] = useState(false);
  const [savingDigestDelivery, setSavingDigestDelivery] = useState(false);
  const [savingReadingPlan, setSavingReadingPlan] = useState(false);
  const [savingObsidianExport, setSavingObsidianExport] = useState(false);
  const [runningObsidianExport, setRunningObsidianExport] = useState(false);
  const [savingLLMModels, setSavingLLMModels] = useState(false);
  const [savingAnthropicKey, setSavingAnthropicKey] = useState(false);
  const [deletingAnthropicKey, setDeletingAnthropicKey] = useState(false);
  const [savingOpenAIKey, setSavingOpenAIKey] = useState(false);
  const [deletingOpenAIKey, setDeletingOpenAIKey] = useState(false);
  const [savingGoogleKey, setSavingGoogleKey] = useState(false);
  const [deletingGoogleKey, setDeletingGoogleKey] = useState(false);
  const [savingGroqKey, setSavingGroqKey] = useState(false);
  const [deletingGroqKey, setDeletingGroqKey] = useState(false);
  const [savingDeepSeekKey, setSavingDeepSeekKey] = useState(false);
  const [deletingDeepSeekKey, setDeletingDeepSeekKey] = useState(false);
  const [savingAlibabaKey, setSavingAlibabaKey] = useState(false);
  const [deletingAlibabaKey, setDeletingAlibabaKey] = useState(false);
  const [savingMistralKey, setSavingMistralKey] = useState(false);
  const [deletingMistralKey, setDeletingMistralKey] = useState(false);
  const [savingMoonshotKey, setSavingMoonshotKey] = useState(false);
  const [deletingMoonshotKey, setDeletingMoonshotKey] = useState(false);
  const [savingXAIKey, setSavingXAIKey] = useState(false);
  const [deletingXAIKey, setDeletingXAIKey] = useState(false);
  const [savingZAIKey, setSavingZAIKey] = useState(false);
  const [deletingZAIKey, setDeletingZAIKey] = useState(false);
  const [savingFireworksKey, setSavingFireworksKey] = useState(false);
  const [deletingFireworksKey, setDeletingFireworksKey] = useState(false);
  const [savingPoeKey, setSavingPoeKey] = useState(false);
  const [deletingPoeKey, setDeletingPoeKey] = useState(false);
  const [savingSiliconFlowKey, setSavingSiliconFlowKey] = useState(false);
  const [deletingSiliconFlowKey, setDeletingSiliconFlowKey] = useState(false);
  const [savingOpenRouterKey, setSavingOpenRouterKey] = useState(false);
  const [deletingOpenRouterKey, setDeletingOpenRouterKey] = useState(false);
  const [savingAivisKey, setSavingAivisKey] = useState(false);
  const [deletingAivisKey, setDeletingAivisKey] = useState(false);
  const [savingFishKey, setSavingFishKey] = useState(false);
  const [deletingFishKey, setDeletingFishKey] = useState(false);
  const [savingAivisDictionary, setSavingAivisDictionary] = useState(false);
  const [deletingAivisDictionary, setDeletingAivisDictionary] = useState(false);
  const [deletingInoreaderOAuth, setDeletingInoreaderOAuth] = useState(false);
  const [resettingPreferenceProfile, setResettingPreferenceProfile] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [preferenceProfile, setPreferenceProfile] = useState<PreferenceProfile | null>(null);
  const [preferenceProfileError, setPreferenceProfileError] = useState<string | null>(null);
  const [catalog, setCatalog] = useState<LLMCatalog | null>(null);
  const [providerModelUpdates, setProviderModelUpdates] = useState<ProviderModelChangeEvent[]>([]);
  const [dismissedModelUpdatesAt, setDismissedModelUpdatesAt] = useState<string | null>(() => {
    if (typeof window === "undefined") return null;
    return window.localStorage.getItem(MODEL_UPDATES_DISMISSED_AT_KEY);
  });
  const [budgetUSD, setBudgetUSD] = useState<string>("");
  const [alertEnabled, setAlertEnabled] = useState(false);
  const [thresholdPct, setThresholdPct] = useState<number>(20);
  const [digestEmailEnabled, setDigestEmailEnabled] = useState(true);
  const [anthropicApiKeyInput, setAnthropicApiKeyInput] = useState("");
  const [openAIApiKeyInput, setOpenAIApiKeyInput] = useState("");
  const [googleApiKeyInput, setGoogleApiKeyInput] = useState("");
  const [groqApiKeyInput, setGroqApiKeyInput] = useState("");
  const [deepseekApiKeyInput, setDeepseekApiKeyInput] = useState("");
  const [alibabaApiKeyInput, setAlibabaApiKeyInput] = useState("");
  const [mistralApiKeyInput, setMistralApiKeyInput] = useState("");
  const [moonshotApiKeyInput, setMoonshotApiKeyInput] = useState("");
  const [xaiApiKeyInput, setXaiApiKeyInput] = useState("");
  const [zaiApiKeyInput, setZaiApiKeyInput] = useState("");
  const [fireworksApiKeyInput, setFireworksApiKeyInput] = useState("");
  const [poeApiKeyInput, setPoeApiKeyInput] = useState("");
  const [siliconFlowApiKeyInput, setSiliconFlowApiKeyInput] = useState("");
  const [openRouterApiKeyInput, setOpenRouterApiKeyInput] = useState("");
  const [aivisApiKeyInput, setAivisApiKeyInput] = useState("");
  const [fishApiKeyInput, setFishApiKeyInput] = useState("");
  const [aivisUserDictionaryUUID, setAivisUserDictionaryUUID] = useState("");
  const [activeAccessProvider, setActiveAccessProvider] = useState("anthropic");
  const [activeSection, setActiveSection] = useState<SettingsSectionID>("models");
  const [llmExtrasOpen, setLLMExtrasOpen] = useState(false);
  const [modelGuideOpen, setModelGuideOpen] = useState(false);
  const [readingPlanWindow, setReadingPlanWindow] = useState<"24h" | "today_jst" | "7d">("24h");
  const [readingPlanSize, setReadingPlanSize] = useState<string>("15");
  const [readingPlanDiversifyTopics, setReadingPlanDiversifyTopics] = useState(true);
  const [audioBriefingEnabled, setAudioBriefingEnabled] = useState(false);
  const [audioBriefingScheduleSelection, setAudioBriefingScheduleSelection] = useState<AudioBriefingScheduleSelection>("interval6h");
  const [audioBriefingArticlesPerEpisode, setAudioBriefingArticlesPerEpisode] = useState("5");
  const [audioBriefingTargetDurationMinutes, setAudioBriefingTargetDurationMinutes] = useState("20");
  const [audioBriefingChunkTrailingSilenceSeconds, setAudioBriefingChunkTrailingSilenceSeconds] = useState("1.0");
  const [audioBriefingProgramName, setAudioBriefingProgramName] = useState("");
  const [audioBriefingDefaultPersonaMode, setAudioBriefingDefaultPersonaMode] = useState<"fixed" | "random">("fixed");
  const [audioBriefingDefaultPersona, setAudioBriefingDefaultPersona] = useState("editor");
  const [audioBriefingConversationMode, setAudioBriefingConversationMode] = useState<"single" | "duo">("single");
  const [audioBriefingBGMEnabled, setAudioBriefingBGMEnabled] = useState(false);
  const [audioBriefingBGMR2Prefix, setAudioBriefingBGMR2Prefix] = useState("");
  const [summaryAudioProvider, setSummaryAudioProvider] = useState("aivis");
  const [summaryAudioTTSModel, setSummaryAudioTTSModel] = useState("");
  const [summaryAudioVoiceModel, setSummaryAudioVoiceModel] = useState("");
  const [summaryAudioVoiceStyle, setSummaryAudioVoiceStyle] = useState("");
  const [summaryAudioProviderVoiceLabel, setSummaryAudioProviderVoiceLabel] = useState("");
  const [summaryAudioProviderVoiceDescription, setSummaryAudioProviderVoiceDescription] = useState("");
  const [summaryAudioSpeechRate, setSummaryAudioSpeechRate] = useState("1");
  const [summaryAudioEmotionalIntensity, setSummaryAudioEmotionalIntensity] = useState("1");
  const [summaryAudioTempoDynamics, setSummaryAudioTempoDynamics] = useState("1");
  const [summaryAudioLineBreakSilenceSeconds, setSummaryAudioLineBreakSilenceSeconds] = useState("0.4");
  const [summaryAudioPitch, setSummaryAudioPitch] = useState("0");
  const [summaryAudioVolumeGain, setSummaryAudioVolumeGain] = useState("0");
  const [summaryAudioAivisUserDictionaryUUID, setSummaryAudioAivisUserDictionaryUUID] = useState("");
  const [summaryAudioVoiceInputDrafts, setSummaryAudioVoiceInputDrafts] = useState<SummaryAudioVoiceInputDrafts>(() =>
    buildSummaryAudioVoiceInputDrafts(buildDefaultSummaryAudioVoiceSettings())
  );
  const [summaryAudioSaving, setSummaryAudioSaving] = useState(false);
  const [podcastEnabled, setPodcastEnabled] = useState(false);
  const [podcastFeedSlug, setPodcastFeedSlug] = useState("");
  const [podcastRSSURL, setPodcastRSSURL] = useState("");
  const [podcastTitle, setPodcastTitle] = useState("");
  const [podcastDescription, setPodcastDescription] = useState("");
  const [podcastAuthor, setPodcastAuthor] = useState("");
  const [podcastLanguage, setPodcastLanguage] = useState("ja");
  const [podcastCategory, setPodcastCategory] = useState("");
  const [podcastSubcategory, setPodcastSubcategory] = useState("");
  const [podcastAvailableCategories, setPodcastAvailableCategories] = useState<PodcastCategoryOption[]>([]);
  const [podcastExplicit, setPodcastExplicit] = useState(false);
  const [podcastArtworkURL, setPodcastArtworkURL] = useState("");
  const [audioBriefingVoices, setAudioBriefingVoices] = useState<AudioBriefingPersonaVoice[]>(buildDefaultAudioBriefingVoices(NAVIGATOR_PERSONA_KEYS));
  const [audioBriefingVoiceInputDrafts, setAudioBriefingVoiceInputDrafts] = useState<AudioBriefingVoiceInputDrafts>(() =>
    buildAudioBriefingVoiceInputDrafts(buildDefaultAudioBriefingVoices(NAVIGATOR_PERSONA_KEYS))
  );
  const [audioBriefingPresets, setAudioBriefingPresets] = useState<AudioBriefingPreset[]>([]);
  const [audioBriefingPresetsLoading, setAudioBriefingPresetsLoading] = useState(false);
  const [audioBriefingPresetsLoaded, setAudioBriefingPresetsLoaded] = useState(false);
  const [audioBriefingPresetsError, setAudioBriefingPresetsError] = useState<string | null>(null);
  const [audioBriefingPresetSaveOpen, setAudioBriefingPresetSaveOpen] = useState(false);
  const [audioBriefingPresetApplyOpen, setAudioBriefingPresetApplyOpen] = useState(false);
  const [audioBriefingPresetName, setAudioBriefingPresetName] = useState("");
  const [audioBriefingPresetSaving, setAudioBriefingPresetSaving] = useState(false);
  const [audioBriefingPresetApplySelection, setAudioBriefingPresetApplySelection] = useState<string | null>(null);
  const [aivisModelsData, setAivisModelsData] = useState<AivisModelsResponse | null>(null);
  const [aivisModelsLoading, setAivisModelsLoading] = useState(false);
  const [aivisModelsSyncing, setAivisModelsSyncing] = useState(false);
  const [aivisModelsError, setAivisModelsError] = useState<string | null>(null);
  const [xaiVoicesData, setXAIVoicesData] = useState<XAIVoicesResponse | null>(null);
  const [xaiVoicesLoading, setXAIVoicesLoading] = useState(false);
  const [xaiVoicesSyncing, setXAIVoicesSyncing] = useState(false);
  const [xaiVoicesError, setXAIVoicesError] = useState<string | null>(null);
  const [openAITTSVoicesData, setOpenAITTSVoicesData] = useState<OpenAITTSVoicesResponse | null>(null);
  const [openAITTSVoicesLoading, setOpenAITTSVoicesLoading] = useState(false);
  const [openAITTSVoicesSyncing, setOpenAITTSVoicesSyncing] = useState(false);
  const [openAITTSVoicesError, setOpenAITTSVoicesError] = useState<string | null>(null);
  const [geminiTTSVoicesData, setGeminiTTSVoicesData] = useState<GeminiTTSVoicesResponse | null>(null);
  const [geminiTTSVoicesLoading, setGeminiTTSVoicesLoading] = useState(false);
  const [geminiTTSVoicesError, setGeminiTTSVoicesError] = useState<string | null>(null);
  const [aivisUserDictionaries, setAivisUserDictionaries] = useState<AivisUserDictionary[]>([]);
  const [aivisUserDictionariesLoading, setAivisUserDictionariesLoading] = useState(false);
  const [aivisUserDictionariesLoaded, setAivisUserDictionariesLoaded] = useState(false);
  const [aivisUserDictionariesError, setAivisUserDictionariesError] = useState<string | null>(null);
  const [aivisPickerPersona, setAivisPickerPersona] = useState<string | null>(null);
  const [xaiPickerPersona, setXAIPickerPersona] = useState<string | null>(null);
  const [fishPickerPersona, setFishPickerPersona] = useState<string | null>(null);
  const [openAITTPickerPersona, setOpenAITTPickerPersona] = useState<string | null>(null);
  const [geminiTTSPickerPersona, setGeminiTTSPickerPersona] = useState<string | null>(null);
  const [summaryAudioAivisPickerOpen, setSummaryAudioAivisPickerOpen] = useState(false);
  const [summaryAudioFishPickerOpen, setSummaryAudioFishPickerOpen] = useState(false);
  const [summaryAudioXAIPickerOpen, setSummaryAudioXAIPickerOpen] = useState(false);
  const [summaryAudioOpenAITTPickerOpen, setSummaryAudioOpenAITTPickerOpen] = useState(false);
  const [summaryAudioGeminiTTSPickerOpen, setSummaryAudioGeminiTTSPickerOpen] = useState(false);
  const [expandedAudioBriefingPersonas, setExpandedAudioBriefingPersonas] = useState<string[]>(["editor"]);
  const [obsidianEnabled, setObsidianEnabled] = useState(false);
  const [notificationPriority, setNotificationPriority] = useState<NotificationPriorityRule>({
    sensitivity: "medium",
    daily_cap: 3,
    theme_weight: 1,
    immediate_enabled: true,
    briefing_enabled: true,
    review_enabled: true,
    goal_match_enabled: true,
  });
  const [obsidianRepoOwner, setObsidianRepoOwner] = useState("");
  const [obsidianRepoName, setObsidianRepoName] = useState("");
  const [obsidianRepoBranch, setObsidianRepoBranch] = useState("main");
  const [obsidianRootPath, setObsidianRootPath] = useState("Sifto/Favorites");
  const [anthropicFactsModel, setAnthropicFactsModel] = useState("");
  const [anthropicFactsSecondaryModel, setAnthropicFactsSecondaryModel] = useState("");
  const [anthropicFactsSecondaryRatePercent, setAnthropicFactsSecondaryRatePercent] = useState("0");
  const [anthropicFactsFallbackModel, setAnthropicFactsFallbackModel] = useState("");
  const [anthropicSummaryModel, setAnthropicSummaryModel] = useState("");
  const [anthropicSummarySecondaryModel, setAnthropicSummarySecondaryModel] = useState("");
  const [anthropicSummarySecondaryRatePercent, setAnthropicSummarySecondaryRatePercent] = useState("0");
  const [anthropicSummaryFallbackModel, setAnthropicSummaryFallbackModel] = useState("");
  const [anthropicDigestClusterModel, setAnthropicDigestClusterModel] = useState("");
  const [anthropicDigestModel, setAnthropicDigestModel] = useState("");
  const [anthropicAskModel, setAnthropicAskModel] = useState("");
  const [anthropicSourceSuggestionModel, setAnthropicSourceSuggestionModel] = useState("");
  const [openAIEmbeddingModel, setOpenAIEmbeddingModel] = useState("");
  const [factsCheckModel, setFactsCheckModel] = useState("");
  const [faithfulnessCheckModel, setFaithfulnessCheckModel] = useState("");
  const [navigatorEnabled, setNavigatorEnabled] = useState(false);
  const [aiNavigatorBriefEnabled, setAINavigatorBriefEnabled] = useState(false);
  const [navigatorPersonaMode, setNavigatorPersonaMode] = useState<"fixed" | "random">("fixed");
  const [navigatorPersona, setNavigatorPersona] = useState("editor");
  const [navigatorModel, setNavigatorModel] = useState("");
  const [navigatorFallbackModel, setNavigatorFallbackModel] = useState("");
  const [aiNavigatorBriefModel, setAINavigatorBriefModel] = useState("");
  const [aiNavigatorBriefFallbackModel, setAINavigatorBriefFallbackModel] = useState("");
  const [audioBriefingScriptModel, setAudioBriefingScriptModel] = useState("");
  const [audioBriefingScriptFallbackModel, setAudioBriefingScriptFallbackModel] = useState("");
  const [fishPreprocessModel, setFishPreprocessModel] = useState("");
  const [navigatorPersonaDefinitions, setNavigatorPersonaDefinitions] = useState<Record<string, NavigatorPersonaDefinition>>({});
  const loadSeqRef = useRef(0);
  const llmModelsDirtyRef = useRef(false);
  const llmExtrasRef = useRef<HTMLDivElement | null>(null);
  const summaryAudioVoiceStatus = useMemo(() => {
    return getSummaryAudioVoiceStatus(
      {
        tts_provider: summaryAudioProvider,
        tts_model: summaryAudioTTSModel,
        voice_model: summaryAudioVoiceModel,
        voice_style: summaryAudioVoiceStyle,
        provider_voice_label: summaryAudioProviderVoiceLabel,
        provider_voice_description: summaryAudioProviderVoiceDescription,
        speech_rate: Number(summaryAudioSpeechRate),
        emotional_intensity: Number(summaryAudioEmotionalIntensity),
        tempo_dynamics: Number(summaryAudioTempoDynamics),
        line_break_silence_seconds: Number(summaryAudioLineBreakSilenceSeconds),
        pitch: Number(summaryAudioPitch),
        volume_gain: Number(summaryAudioVolumeGain),
        aivis_user_dictionary_uuid: summaryAudioAivisUserDictionaryUUID || null,
      },
      aivisModelsData?.models ?? [],
      [],
      xaiVoicesData?.voices ?? [],
      openAITTSVoicesData?.voices ?? [],
      geminiTTSVoicesData?.voices ?? [],
      Boolean(settings?.has_aivis_api_key),
      Boolean(settings?.has_fish_api_key),
      Boolean(settings?.has_xai_api_key),
      Boolean(settings?.has_openai_api_key),
      Boolean(settings?.gemini_tts_enabled),
      t
    );
  }, [
    aivisModelsData?.models,
    geminiTTSVoicesData?.voices,
    openAITTSVoicesData?.voices,
    settings?.has_fish_api_key,
    settings?.gemini_tts_enabled,
    settings?.has_aivis_api_key,
    settings?.has_openai_api_key,
    settings?.has_xai_api_key,
    summaryAudioAivisUserDictionaryUUID,
    summaryAudioEmotionalIntensity,
    summaryAudioLineBreakSilenceSeconds,
    summaryAudioPitch,
    summaryAudioProvider,
    summaryAudioSpeechRate,
    summaryAudioTempoDynamics,
    summaryAudioTTSModel,
    summaryAudioVoiceModel,
    summaryAudioVoiceStyle,
    summaryAudioVolumeGain,
    t,
    xaiVoicesData?.voices,
  ]);
  const summaryAudioConfigured = getSummaryAudioReadiness(settings).ready;
  const navigatorPersonaCards = useMemo(
    () =>
      NAVIGATOR_PERSONA_KEYS.map((key) => ({
        key,
        ...EMPTY_NAVIGATOR_PERSONA,
        ...(navigatorPersonaDefinitions[key] as NavigatorPersonaDefinition | undefined),
      })),
    [navigatorPersonaDefinitions]
  );
  const syncAudioBriefingVoiceForm = useCallback((voices?: UserSettings["audio_briefing_persona_voices"] | AudioBriefingPersonaVoice[] | null) => {
    const defaults = buildDefaultAudioBriefingVoices(NAVIGATOR_PERSONA_KEYS);
    const byPersona = new Map((voices ?? []).map((voice) => [voice.persona, voice] as const));
    const nextVoices = defaults.map((voice) => byPersona.get(voice.persona) ?? voice);
    setAudioBriefingVoices(nextVoices);
    setAudioBriefingVoiceInputDrafts(buildAudioBriefingVoiceInputDrafts(nextVoices));
  }, []);

  const syncAudioBriefingSettingsForm = useCallback((audioBriefing?: UserSettings["audio_briefing"] | null) => {
    setAudioBriefingEnabled(Boolean(audioBriefing?.enabled));
    setAudioBriefingScheduleSelection(resolveAudioBriefingScheduleSelection(audioBriefing));
    setAudioBriefingArticlesPerEpisode(String(audioBriefing?.articles_per_episode ?? 5));
    setAudioBriefingTargetDurationMinutes(String(audioBriefing?.target_duration_minutes ?? 20));
    setAudioBriefingChunkTrailingSilenceSeconds(formatAudioBriefingDecimalInput(audioBriefing?.chunk_trailing_silence_seconds ?? 1.0));
    setAudioBriefingProgramName(audioBriefing?.program_name ?? "");
    setAudioBriefingDefaultPersonaMode(audioBriefing?.default_persona_mode === "random" ? "random" : "fixed");
    setAudioBriefingDefaultPersona(audioBriefing?.default_persona ?? "editor");
    setAudioBriefingConversationMode(audioBriefing?.conversation_mode === "duo" ? "duo" : "single");
    setAudioBriefingBGMEnabled(Boolean(audioBriefing?.bgm_enabled));
    setAudioBriefingBGMR2Prefix(audioBriefing?.bgm_r2_prefix ?? "");
  }, []);

  const syncAudioBriefingForm = useCallback(
    (audioBriefing?: UserSettings["audio_briefing"] | null, voices?: UserSettings["audio_briefing_persona_voices"] | null) => {
      syncAudioBriefingSettingsForm(audioBriefing);
      syncAudioBriefingVoiceForm(voices);
    },
    [syncAudioBriefingSettingsForm, syncAudioBriefingVoiceForm]
  );

  const syncAudioBriefingPresetForm = useCallback((preset: AudioBriefingPreset) => {
    syncAudioBriefingSettingsForm({
      enabled: audioBriefingEnabled,
      schedule_mode: audioBriefingScheduleSelection === "fixed3x" ? "fixed_slots_3x" : "interval",
      interval_hours: audioBriefingScheduleSelection === "interval3h" ? 3 : 6,
      articles_per_episode: Number(audioBriefingArticlesPerEpisode),
      target_duration_minutes: Number(audioBriefingTargetDurationMinutes),
      chunk_trailing_silence_seconds: Number(audioBriefingChunkTrailingSilenceSeconds),
      program_name: audioBriefingProgramName.trim() || null,
      default_persona_mode: preset.default_persona_mode === "random" ? "random" : "fixed",
      default_persona: preset.default_persona || "editor",
      conversation_mode: preset.conversation_mode === "duo" ? "duo" : "single",
      bgm_enabled: audioBriefingBGMEnabled,
      bgm_r2_prefix: audioBriefingBGMR2Prefix.trim() || null,
    });
    syncAudioBriefingVoiceForm(normalizeAudioBriefingPresetVoices(preset.voices ?? []));
    setExpandedAudioBriefingPersonas((prev) =>
      prev.includes(preset.default_persona) ? prev : [preset.default_persona]
    );
  }, [
    audioBriefingArticlesPerEpisode,
    audioBriefingBGMEnabled,
    audioBriefingBGMR2Prefix,
    audioBriefingChunkTrailingSilenceSeconds,
    audioBriefingEnabled,
    audioBriefingProgramName,
    audioBriefingScheduleSelection,
    audioBriefingTargetDurationMinutes,
    syncAudioBriefingSettingsForm,
    syncAudioBriefingVoiceForm,
  ]);

  const syncSummaryAudioForm = useCallback((summaryAudio?: UserSettings["summary_audio"] | null) => {
    const next = summaryAudio ?? buildDefaultSummaryAudioVoiceSettings();
    setSummaryAudioProvider(next.tts_provider || "aivis");
    setSummaryAudioTTSModel(next.tts_model ?? "");
    setSummaryAudioVoiceModel(next.voice_model ?? "");
    setSummaryAudioVoiceStyle(next.voice_style ?? "");
    setSummaryAudioProviderVoiceLabel(next.provider_voice_label ?? "");
    setSummaryAudioProviderVoiceDescription(next.provider_voice_description ?? "");
    setSummaryAudioSpeechRate(formatAudioBriefingDecimalInput(next.speech_rate ?? 1));
    setSummaryAudioEmotionalIntensity(formatAudioBriefingDecimalInput(next.emotional_intensity ?? 1));
    setSummaryAudioTempoDynamics(formatAudioBriefingDecimalInput(next.tempo_dynamics ?? 1));
    setSummaryAudioLineBreakSilenceSeconds(formatAudioBriefingDecimalInput(next.line_break_silence_seconds ?? 0.4));
    setSummaryAudioPitch(formatAudioBriefingDecimalInput(next.pitch ?? 0));
    setSummaryAudioVolumeGain(formatAudioBriefingDecimalInput(next.volume_gain ?? 0));
    setSummaryAudioAivisUserDictionaryUUID(next.aivis_user_dictionary_uuid ?? "");
    setSummaryAudioVoiceInputDrafts(buildSummaryAudioVoiceInputDrafts(next));
  }, []);

  const loadAudioBriefingPresets = useCallback(async () => {
    setAudioBriefingPresetsLoading(true);
    setAudioBriefingPresetsError(null);
    try {
      const presets = await api.listAudioBriefingPresets();
      setAudioBriefingPresets(presets ?? []);
    } catch (e) {
      setAudioBriefingPresets([]);
      setAudioBriefingPresetsError(String(e));
    } finally {
      setAudioBriefingPresetsLoaded(true);
      setAudioBriefingPresetsLoading(false);
    }
  }, []);

  useEffect(() => {
    const section = searchParams.get("section");
    if (isSettingsSectionID(section)) {
      setActiveSection(section);
    }
  }, [searchParams]);

  const syncPodcastForm = useCallback((podcast?: UserSettings["podcast"] | null) => {
    setPodcastEnabled(Boolean(podcast?.enabled));
    setPodcastFeedSlug(podcast?.feed_slug ?? "");
    setPodcastRSSURL(buildPodcastRSSURL(podcast?.feed_slug, podcast?.rss_url));
    setPodcastTitle(podcast?.title ?? "");
    setPodcastDescription(podcast?.description ?? "");
    setPodcastAuthor(podcast?.author ?? "");
    setPodcastLanguage(podcast?.language ?? "ja");
    setPodcastCategory(podcast?.category ?? "");
    setPodcastSubcategory(podcast?.subcategory ?? "");
    setPodcastAvailableCategories(podcast?.available_categories ?? []);
    setPodcastExplicit(Boolean(podcast?.explicit));
    setPodcastArtworkURL(podcast?.artwork_url ?? "");
  }, []);

  const selectedPodcastCategory = useMemo(
    () => podcastAvailableCategories.find((option) => option.category === podcastCategory) ?? null,
    [podcastAvailableCategories, podcastCategory]
  );

  const syncLLMModelForm = useCallback((llmModels?: UserSettings["llm_models"] | null) => {
    setAnthropicFactsModel(llmModels?.facts ?? "");
    setAnthropicFactsSecondaryModel(llmModels?.facts_secondary ?? "");
    setAnthropicFactsSecondaryRatePercent(String(llmModels?.facts_secondary_rate_percent ?? 0));
    setAnthropicFactsFallbackModel(llmModels?.facts_fallback ?? "");
    setAnthropicSummaryModel(llmModels?.summary ?? "");
    setAnthropicSummarySecondaryModel(llmModels?.summary_secondary ?? "");
    setAnthropicSummarySecondaryRatePercent(String(llmModels?.summary_secondary_rate_percent ?? 0));
    setAnthropicSummaryFallbackModel(llmModels?.summary_fallback ?? "");
    setAnthropicDigestClusterModel(llmModels?.digest_cluster ?? "");
    setAnthropicDigestModel(llmModels?.digest ?? "");
    setAnthropicAskModel(llmModels?.ask ?? "");
    setAnthropicSourceSuggestionModel(llmModels?.source_suggestion ?? "");
    setOpenAIEmbeddingModel(llmModels?.embedding ?? "");
    setFactsCheckModel(llmModels?.facts_check ?? "");
    setFaithfulnessCheckModel(llmModels?.faithfulness_check ?? "");
    setNavigatorEnabled(Boolean(llmModels?.navigator_enabled ?? false));
    setAINavigatorBriefEnabled(Boolean(llmModels?.ai_navigator_brief_enabled ?? false));
    setNavigatorPersonaMode(llmModels?.navigator_persona_mode === "random" ? "random" : "fixed");
    setNavigatorPersona(llmModels?.navigator_persona ?? "editor");
    setNavigatorModel(llmModels?.navigator ?? "");
    setNavigatorFallbackModel(llmModels?.navigator_fallback ?? "");
    setAINavigatorBriefModel(llmModels?.ai_navigator_brief ?? "");
    setAINavigatorBriefFallbackModel(llmModels?.ai_navigator_brief_fallback ?? "");
    setAudioBriefingScriptModel(llmModels?.audio_briefing_script ?? "");
    setAudioBriefingScriptFallbackModel(llmModels?.audio_briefing_script_fallback ?? "");
    setFishPreprocessModel(llmModels?.fish_preprocess_model ?? "");
  }, []);

  const onChangeLLMModel = useCallback((setter: (value: string) => void, value: string) => {
    llmModelsDirtyRef.current = true;
    setter(value);
  }, []);

  const buildLLMModelPayload = useCallback(
    (overrides?: Partial<{
      facts: string | null;
      facts_secondary: string | null;
      facts_secondary_rate_percent: number;
      facts_fallback: string | null;
      summary: string | null;
      summary_secondary: string | null;
      summary_secondary_rate_percent: number;
      summary_fallback: string | null;
      digest_cluster: string | null;
      digest: string | null;
      ask: string | null;
      source_suggestion: string | null;
      embedding: string | null;
      facts_check: string | null;
      faithfulness_check: string | null;
      navigator_enabled: boolean;
      ai_navigator_brief_enabled: boolean;
      navigator_persona_mode: string | null;
      navigator_persona: string | null;
      navigator: string | null;
      navigator_fallback: string | null;
      ai_navigator_brief: string | null;
      ai_navigator_brief_fallback: string | null;
      audio_briefing_script: string | null;
      audio_briefing_script_fallback: string | null;
      fish_preprocess_model: string | null;
    }>) => {
      const emptyToNull = (v: string) => {
        const s = v.trim();
        return s === "" ? null : s;
      };
      const normalizeRate = (v: string) => {
        const n = Number(v);
        if (!Number.isFinite(n)) return 0;
        return Math.min(100, Math.max(0, Math.round(n)));
      };
      return {
        facts: emptyToNull(anthropicFactsModel),
        facts_secondary: emptyToNull(anthropicFactsSecondaryModel),
        facts_secondary_rate_percent: normalizeRate(anthropicFactsSecondaryRatePercent),
        facts_fallback: emptyToNull(anthropicFactsFallbackModel),
        summary: emptyToNull(anthropicSummaryModel),
        summary_secondary: emptyToNull(anthropicSummarySecondaryModel),
        summary_secondary_rate_percent: normalizeRate(anthropicSummarySecondaryRatePercent),
        summary_fallback: emptyToNull(anthropicSummaryFallbackModel),
        digest_cluster: emptyToNull(anthropicDigestClusterModel),
        digest: emptyToNull(anthropicDigestModel),
        ask: emptyToNull(anthropicAskModel),
        source_suggestion: emptyToNull(anthropicSourceSuggestionModel),
        embedding: emptyToNull(openAIEmbeddingModel),
        facts_check: emptyToNull(factsCheckModel),
        faithfulness_check: emptyToNull(faithfulnessCheckModel),
        navigator_enabled: navigatorEnabled,
        ai_navigator_brief_enabled: aiNavigatorBriefEnabled,
        navigator_persona_mode: navigatorPersonaMode,
        navigator_persona: navigatorPersona,
        navigator: emptyToNull(navigatorModel),
        navigator_fallback: emptyToNull(navigatorFallbackModel),
        ai_navigator_brief: emptyToNull(aiNavigatorBriefModel),
        ai_navigator_brief_fallback: emptyToNull(aiNavigatorBriefFallbackModel),
        audio_briefing_script: emptyToNull(audioBriefingScriptModel),
        audio_briefing_script_fallback: emptyToNull(audioBriefingScriptFallbackModel),
        fish_preprocess_model: emptyToNull(fishPreprocessModel),
        ...overrides,
      };
    },
    [
      anthropicAskModel,
      anthropicDigestClusterModel,
      anthropicDigestModel,
      anthropicFactsFallbackModel,
      anthropicFactsModel,
      anthropicFactsSecondaryModel,
      anthropicFactsSecondaryRatePercent,
      anthropicSourceSuggestionModel,
      anthropicSummaryFallbackModel,
      anthropicSummaryModel,
      anthropicSummarySecondaryModel,
      anthropicSummarySecondaryRatePercent,
      factsCheckModel,
      faithfulnessCheckModel,
      aiNavigatorBriefEnabled,
      aiNavigatorBriefFallbackModel,
      aiNavigatorBriefModel,
      navigatorEnabled,
      navigatorFallbackModel,
      navigatorModel,
      navigatorPersonaMode,
      navigatorPersona,
      audioBriefingScriptFallbackModel,
      audioBriefingScriptModel,
      fishPreprocessModel,
      openAIEmbeddingModel,
    ]
  );

  const persistLLMModels = useCallback(
    async (
      payload: ReturnType<typeof buildLLMModelPayload>,
      successMessage?: string
    ) => {
      const resp = await api.updateLLMModelSettings(payload);
      setSettings((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          llm_models: {
            ...prev.llm_models,
            ...resp.llm_models,
          },
        };
      });
      syncLLMModelForm(resp.llm_models);
      llmModelsDirtyRef.current = false;
      if (successMessage) {
        showToast(successMessage, "success");
      }
      return resp;
    },
    [showToast, syncLLMModelForm]
  );

  const loadAivisModels = useCallback(async () => {
    setAivisModelsLoading(true);
    try {
      const next = await api.getAivisModels();
      setAivisModelsData(next);
      setAivisModelsError(null);
      return next;
    } catch (e) {
      const message = String(e);
      setAivisModelsError(message);
      throw e;
    } finally {
      setAivisModelsLoading(false);
    }
  }, []);

  const loadXAIVoices = useCallback(async () => {
    setXAIVoicesLoading(true);
    try {
      const next = await api.getXAIVoices();
      setXAIVoicesData(next);
      setXAIVoicesError(null);
      return next;
    } catch (e) {
      const message = String(e);
      setXAIVoicesError(message);
      throw e;
    } finally {
      setXAIVoicesLoading(false);
    }
  }, []);

  const loadOpenAITTSVoices = useCallback(async () => {
    setOpenAITTSVoicesLoading(true);
    try {
      const next = await api.getOpenAITTSVoices();
      setOpenAITTSVoicesData(next);
      setOpenAITTSVoicesError(null);
      return next;
    } catch (e) {
      const message = String(e);
      setOpenAITTSVoicesError(message);
      throw e;
    } finally {
      setOpenAITTSVoicesLoading(false);
    }
  }, []);

  const loadGeminiTTSVoices = useCallback(async () => {
    setGeminiTTSVoicesLoading(true);
    try {
      const next = await api.getGeminiTTSVoices();
      setGeminiTTSVoicesData(next);
      setGeminiTTSVoicesError(null);
      return next;
    } catch (e) {
      const message = String(e);
      setGeminiTTSVoicesError(message);
      throw e;
    } finally {
      setGeminiTTSVoicesLoading(false);
    }
  }, []);

  const syncAivisModels = useCallback(async () => {
    setAivisModelsSyncing(true);
    try {
      const next = await api.syncAivisModels();
      setAivisModelsData(next);
      setAivisModelsError(null);
      showToast(t("aivisModels.syncCompleted"), "success");
      return next;
    } catch (e) {
      const message = String(e);
      setAivisModelsError(message);
      showToast(message, "error");
      throw e;
    } finally {
      setAivisModelsSyncing(false);
    }
  }, [showToast, t]);

  const syncXAIVoices = useCallback(async () => {
    setXAIVoicesSyncing(true);
    try {
      const next = await api.syncXAIVoices();
      setXAIVoicesData(next);
      setXAIVoicesError(null);
      showToast(t("settings.audioBriefing.xaiSyncCompleted"), "success");
      return next;
    } catch (e) {
      const message = String(e);
      setXAIVoicesError(message);
      showToast(message, "error");
      throw e;
    } finally {
      setXAIVoicesSyncing(false);
    }
  }, [showToast, t]);

  const syncOpenAITTSVoices = useCallback(async () => {
    setOpenAITTSVoicesSyncing(true);
    try {
      const next = await api.syncOpenAITTSVoices();
      setOpenAITTSVoicesData(next);
      setOpenAITTSVoicesError(null);
      showToast(t("settings.audioBriefing.openAITTSSyncCompleted"), "success");
      return next;
    } catch (e) {
      const message = String(e);
      setOpenAITTSVoicesError(message);
      showToast(message, "error");
      throw e;
    } finally {
      setOpenAITTSVoicesSyncing(false);
    }
  }, [showToast, t]);

  const loadAivisUserDictionaries = useCallback(async (force = false) => {
    if (!force && aivisUserDictionariesLoaded) {
      return aivisUserDictionaries;
    }
    setAivisUserDictionariesLoading(true);
    try {
      const next = await api.getAivisUserDictionaries();
      setAivisUserDictionaries(next.user_dictionaries ?? []);
      setAivisUserDictionariesLoaded(true);
      setAivisUserDictionariesError(null);
      return next.user_dictionaries ?? [];
    } catch (e) {
      const message = String(e);
      setAivisUserDictionariesError(message);
      if (force) {
        showToast(message, "error");
      }
      throw e;
    } finally {
      setAivisUserDictionariesLoading(false);
    }
  }, [aivisUserDictionaries, aivisUserDictionariesLoaded, showToast]);

  const load = useCallback(async () => {
    const seq = ++loadSeqRef.current;
    setLoading(true);
    try {
      const [data, nextCatalog, navigatorPersonas, preferenceProfileResult] = await Promise.all([
        api.getSettings(),
        api.getLLMCatalog(),
        api.getNavigatorPersonas(),
        api.getPreferenceProfile()
          .then((profile) => ({ profile, error: null as string | null }))
          .catch((profileError) => ({ profile: null, error: localizePreferenceProfileErrorMessage(profileError, t) })),
      ]);
      if (seq !== loadSeqRef.current) return;
      setSettings(data);
      setCatalog(nextCatalog);
      setNavigatorPersonaDefinitions(navigatorPersonas ?? {});
      setPreferenceProfile(preferenceProfileResult.profile);
      setPreferenceProfileError(preferenceProfileResult.error);
      setAivisUserDictionaryUUID(data.aivis_user_dictionary_uuid ?? "");
      if (!data.has_aivis_api_key) {
        setAivisUserDictionaries([]);
        setAivisUserDictionariesLoaded(false);
        setAivisUserDictionariesError(null);
      }
      if (!data.has_xai_api_key) {
        setXAIVoicesData(null);
        setXAIVoicesError(null);
      }
      setBudgetUSD(data.monthly_budget_usd == null ? "" : String(data.monthly_budget_usd));
      setAlertEnabled(Boolean(data.budget_alert_enabled));
      setThresholdPct(data.budget_alert_threshold_pct ?? 20);
      setDigestEmailEnabled(Boolean(data.digest_email_enabled ?? true));
      syncAudioBriefingForm(data.audio_briefing, data.audio_briefing_persona_voices);
      syncPodcastForm(data.podcast);
      setReadingPlanWindow((data.reading_plan?.window as "24h" | "today_jst" | "7d") ?? "24h");
      const rpSize = data.reading_plan?.size;
      setReadingPlanSize(String(rpSize === 7 || rpSize === 15 || rpSize === 25 ? rpSize : 15));
      setReadingPlanDiversifyTopics(Boolean(data.reading_plan?.diversify_topics ?? true));
      setObsidianEnabled(Boolean(data.obsidian_export?.enabled));
      setNotificationPriority(data.notification_priority ?? { sensitivity: "medium", daily_cap: 3, theme_weight: 1, immediate_enabled: true, briefing_enabled: true, review_enabled: true, goal_match_enabled: true });
      setObsidianRepoOwner(data.obsidian_export?.github_repo_owner ?? "");
      setObsidianRepoName(data.obsidian_export?.github_repo_name ?? "");
      setObsidianRepoBranch(data.obsidian_export?.github_repo_branch ?? "main");
      setObsidianRootPath(data.obsidian_export?.vault_root_path ?? "Sifto/Favorites");
      syncSummaryAudioForm(data.summary_audio ?? null);
      if (!llmModelsDirtyRef.current) {
        syncLLMModelForm(data.llm_models);
      }
      setError(null);
    } catch (e) {
      if (seq !== loadSeqRef.current) return;
      setError(String(e));
    } finally {
      if (seq === loadSeqRef.current) {
        setLoading(false);
      }
    }
  }, [syncAudioBriefingForm, syncLLMModelForm, syncPodcastForm, syncSummaryAudioForm, t]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && !audioBriefingPresetSaveOpen && !audioBriefingPresetApplyOpen) {
      return;
    }
    if (audioBriefingPresetsLoaded || audioBriefingPresetsLoading || audioBriefingPresets.length > 0) {
      return;
    }
    void loadAudioBriefingPresets().catch(() => undefined);
  }, [
    activeSection,
    audioBriefingPresetApplyOpen,
    audioBriefingPresetSaveOpen,
    audioBriefingPresets.length,
    audioBriefingPresetsLoading,
    audioBriefingPresetsLoaded,
    loadAudioBriefingPresets,
  ]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || aivisModelsData != null || aivisModelsLoading) return;
    void loadAivisModels().catch(() => undefined);
  }, [activeSection, aivisModelsData, aivisModelsLoading, loadAivisModels]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || !settings?.has_xai_api_key || xaiVoicesData != null || xaiVoicesLoading) {
      return;
    }
    void loadXAIVoices().catch(() => undefined);
  }, [activeSection, loadXAIVoices, settings?.has_xai_api_key, xaiVoicesData, xaiVoicesLoading]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || !settings?.has_openai_api_key || openAITTSVoicesData != null || openAITTSVoicesLoading) {
      return;
    }
    void loadOpenAITTSVoices().catch(() => undefined);
  }, [activeSection, loadOpenAITTSVoices, openAITTSVoicesData, openAITTSVoicesLoading, settings?.has_openai_api_key]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || geminiTTSVoicesData != null || geminiTTSVoicesLoading) {
      return;
    }
    void loadGeminiTTSVoices().catch(() => undefined);
  }, [activeSection, geminiTTSVoicesData, geminiTTSVoicesLoading, loadGeminiTTSVoices]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || !settings?.has_aivis_api_key || aivisUserDictionariesLoading || aivisUserDictionariesLoaded) {
      return;
    }
    void loadAivisUserDictionaries().catch(() => undefined);
  }, [
    activeSection,
    aivisUserDictionariesLoaded,
    aivisUserDictionariesLoading,
    loadAivisUserDictionaries,
    settings?.has_aivis_api_key,
  ]);

  useEffect(() => {
    setExpandedAudioBriefingPersonas((prev) => {
      if (prev.length === 0) return [audioBriefingDefaultPersona];
      if (prev.length === 1 && prev[0] === "editor" && audioBriefingDefaultPersona !== "editor") {
        return [audioBriefingDefaultPersona];
      }
      return prev;
    });
  }, [audioBriefingDefaultPersona]);

  useEffect(() => {
    setPodcastRSSURL(buildPodcastRSSURL(podcastFeedSlug, podcastRSSURL));
  }, [podcastFeedSlug]);

  useEffect(() => {
    let cancelled = false;
    api.getProviderModelUpdates({ days: 14, limit: 20 })
      .then((modelUpdates) => {
        if (cancelled) return;
        setProviderModelUpdates(modelUpdates ?? []);
      })
      .catch(() => {
        if (cancelled) return;
        setProviderModelUpdates([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const inoreaderStatus = params.get("inoreader");
    if (inoreaderStatus === "connected") {
      showToast(t("settings.toast.inoreaderConnected"), "success");
    } else if (inoreaderStatus === "error") {
      showToast(t("settings.toast.inoreaderConnectError"), "error");
    }
    const obsidianStatus = params.get("obsidian_github");
    if (obsidianStatus === "connected") {
      showToast(t("settings.toast.obsidianGithubConnected"), "success");
    } else if (obsidianStatus === "error") {
      showToast(t("settings.toast.obsidianGithubConnectError"), "error");
    }
    if (inoreaderStatus || obsidianStatus) {
      params.delete("inoreader");
      params.delete("obsidian_github");
      const qs = params.toString();
      const nextURL = `${window.location.pathname}${qs ? `?${qs}` : ""}${window.location.hash}`;
      window.history.replaceState(null, "", nextURL);
    }
  }, [showToast, t]);

  const toModelOption = useCallback((item: LLMCatalogModel): ModelOption => {
    const providerLabel = t(`settings.modelGuide.provider.${item.provider}`, item.provider);
    return {
      value: item.id,
      label: formatModelDisplayName(item.id),
      selectedLabel: formatProviderModelLabel(providerLabel, item.id),
      note: formatModelOptionNote(item),
      provider: providerLabel,
    };
  }, [t]);

  const modelSelectLabels = useMemo(() => ({
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
  }), [t]);

  const applyCostPerformancePreset = useCallback(() => {
    const preset = buildCostPerformancePreset(catalog);
    llmModelsDirtyRef.current = true;
    setAnthropicFactsModel(preset.facts ?? "");
    setAnthropicFactsSecondaryModel("");
    setAnthropicFactsSecondaryRatePercent("0");
    setAnthropicFactsFallbackModel("");
    setAnthropicSummaryModel(preset.summary ?? "");
    setAnthropicSummarySecondaryModel("");
    setAnthropicSummarySecondaryRatePercent("0");
    setAnthropicSummaryFallbackModel("");
    setAnthropicDigestClusterModel(preset.digest_cluster ?? "");
    setAnthropicDigestModel(preset.digest ?? "");
    setAnthropicAskModel(preset.ask ?? "");
    setAnthropicSourceSuggestionModel(preset.source_suggestion ?? "");
    setOpenAIEmbeddingModel(preset.embedding ?? "");
    setFactsCheckModel(preset.facts_check ?? "");
    setFaithfulnessCheckModel(preset.faithfulness_check ?? "");
    setFishPreprocessModel(preset.fish_preprocess_model ?? "");
  }, [catalog]);

  const optionsForPurpose = useCallback(
    (purpose: string, currentValue?: string): ModelOption[] => {
      const items = (catalog?.chat_models ?? [])
        .filter((item) => {
          if (!(item.available_purposes ?? []).includes(purpose)) return false;
          if (item.id === currentValue) return true;
          return !isUnavailableOpenRouterModel(item);
        })
        .map(toModelOption);
      if (!currentValue || items.some((item) => item.value === currentValue)) {
        return items;
      }
      const providerLabel = inferProviderLabelFromModelID(currentValue, t);
      return [
        {
          value: currentValue,
          label: formatModelDisplayName(currentValue),
          selectedLabel: formatProviderModelLabel(providerLabel, currentValue),
          provider: providerLabel ?? undefined,
        },
        ...items,
      ];
    },
    [catalog?.chat_models, t, toModelOption]
  );

  const optionsForChatModel = useCallback(
    (currentValue?: string): ModelOption[] => {
      const items = (catalog?.chat_models ?? []).map(toModelOption);
      if (!currentValue || items.some((item) => item.value === currentValue)) {
        return items;
      }
      const providerLabel = inferProviderLabelFromModelID(currentValue, t);
      return [
        {
          value: currentValue,
          label: formatModelDisplayName(currentValue),
          selectedLabel: formatProviderModelLabel(providerLabel, currentValue),
          provider: providerLabel ?? undefined,
        },
        ...items,
      ];
    },
    [catalog?.chat_models, t, toModelOption]
  );

  const unavailableSelectedModelWarnings = useMemo(() => {
    const chatModels = catalog?.chat_models ?? [];
    const byID = new Map(chatModels.map((item) => [item.id, item] as const));
    const entries: Array<{ key: string; label: string; modelLabel: string }> = [];
    const llmModels = settings?.llm_models ?? {};
    const candidates: Array<[string, string | null | undefined]> = [
      ["facts", llmModels.facts],
      ["facts_fallback", llmModels.facts_fallback],
      ["summary", llmModels.summary],
      ["summary_fallback", llmModels.summary_fallback],
      ["digest_cluster", llmModels.digest_cluster],
      ["digest", llmModels.digest],
      ["ask", llmModels.ask],
      ["source_suggestion", llmModels.source_suggestion],
      ["facts_check", llmModels.facts_check],
      ["faithfulness_check", llmModels.faithfulness_check],
      ["navigator", llmModels.navigator],
      ["navigator_fallback", llmModels.navigator_fallback],
      ["ai_navigator_brief", llmModels.ai_navigator_brief],
      ["ai_navigator_brief_fallback", llmModels.ai_navigator_brief_fallback],
    ];
    for (const [settingKey, modelID] of candidates) {
      if (!modelID) continue;
      const item = byID.get(modelID);
      if (!item || !isUnavailableOpenRouterModel(item)) continue;
      entries.push({
        key: settingKey,
        label: localizeLLMSettingKey(settingKey, t),
        modelLabel: formatModelDisplayName(modelID),
      });
    }
    return entries;
  }, [catalog?.chat_models, settings?.llm_models, t]);

  const sourceSuggestionModelOptions = useMemo(
    () => optionsForPurpose("source_suggestion", anthropicSourceSuggestionModel),
    [anthropicSourceSuggestionModel, optionsForPurpose]
  );
  const openAIEmbeddingModelOptions = useMemo(
    () => (catalog?.embedding_models ?? []).map(toModelOption),
    [catalog?.embedding_models, toModelOption]
  );
  const modelComparisonEntries = useMemo(
    () => [...(catalog?.chat_models ?? []), ...(catalog?.embedding_models ?? [])],
    [catalog?.chat_models, catalog?.embedding_models]
  );
  const visibleProviderModelUpdates = useMemo(() => {
    if (!dismissedModelUpdatesAt) return providerModelUpdates;
    const dismissedMs = Date.parse(dismissedModelUpdatesAt);
    if (Number.isNaN(dismissedMs)) return providerModelUpdates;
    return providerModelUpdates.filter((event) => Date.parse(event.detected_at) > dismissedMs);
  }, [dismissedModelUpdatesAt, providerModelUpdates]);

  const budgetRemainingTone = useMemo(() => {
    const v = settings?.current_month.remaining_budget_pct;
    if (v == null) return "text-zinc-700";
    if (v < 0) return "text-red-600";
    if (v < thresholdPct) return "text-amber-600";
    return "text-zinc-700";
  }, [settings?.current_month.remaining_budget_pct, thresholdPct]);

  const apiKeyCardLabels = useMemo(() => ({
    configured: t("settings.configured"),
    newApiKey: t("settings.newApiKey"),
    saveOrUpdate: t("settings.saveOrUpdate"),
    saving: t("common.saving"),
    deleteKey: t("settings.deleteKey"),
    deleting: t("settings.deleting"),
  }), [t]);

  const accessCards = settings
    ? [
        {
          id: "anthropic",
          title: t("settings.anthropicTitle"),
          description: t("settings.anthropicDescription"),
          configured: settings.has_anthropic_api_key,
          last4: settings.anthropic_api_key_last4,
          value: anthropicApiKeyInput,
          onChange: setAnthropicApiKeyInput,
          onSubmit: submitAnthropicApiKey,
          onDelete: handleDeleteAnthropicApiKey,
          placeholder: "sk-ant-...",
          saving: savingAnthropicKey,
          deleting: deletingAnthropicKey,
          notSet: t("settings.anthropicNotSet"),
        },
        {
          id: "openai",
          title: t("settings.openaiTitle"),
          description: t("settings.openaiDescription"),
          configured: settings.has_openai_api_key,
          last4: settings.openai_api_key_last4,
          value: openAIApiKeyInput,
          onChange: setOpenAIApiKeyInput,
          onSubmit: submitOpenAIApiKey,
          onDelete: handleDeleteOpenAIApiKey,
          placeholder: "sk-...",
          saving: savingOpenAIKey,
          deleting: deletingOpenAIKey,
          notSet: t("settings.openaiNotSet"),
        },
        {
          id: "google",
          title: t("settings.googleTitle"),
          description: t("settings.googleDescription"),
          configured: settings.has_google_api_key,
          last4: settings.google_api_key_last4,
          value: googleApiKeyInput,
          onChange: setGoogleApiKeyInput,
          onSubmit: submitGoogleApiKey,
          onDelete: handleDeleteGoogleApiKey,
          placeholder: "AIza...",
          saving: savingGoogleKey,
          deleting: deletingGoogleKey,
          notSet: t("settings.googleNotSet"),
        },
        {
          id: "groq",
          title: t("settings.groqTitle"),
          description: t("settings.groqDescription"),
          configured: settings.has_groq_api_key,
          last4: settings.groq_api_key_last4,
          value: groqApiKeyInput,
          onChange: setGroqApiKeyInput,
          onSubmit: submitGroqApiKey,
          onDelete: handleDeleteGroqApiKey,
          placeholder: "gsk_...",
          saving: savingGroqKey,
          deleting: deletingGroqKey,
          notSet: t("settings.groqNotSet"),
        },
        {
          id: "deepseek",
          title: t("settings.deepseekTitle"),
          description: t("settings.deepseekDescription"),
          configured: settings.has_deepseek_api_key,
          last4: settings.deepseek_api_key_last4,
          value: deepseekApiKeyInput,
          onChange: setDeepseekApiKeyInput,
          onSubmit: submitDeepSeekApiKey,
          onDelete: handleDeleteDeepSeekApiKey,
          placeholder: "sk-...",
          saving: savingDeepSeekKey,
          deleting: deletingDeepSeekKey,
          notSet: t("settings.deepseekNotSet"),
        },
        {
          id: "alibaba",
          title: t("settings.alibabaTitle"),
          description: t("settings.alibabaDescription"),
          configured: settings.has_alibaba_api_key,
          last4: settings.alibaba_api_key_last4,
          value: alibabaApiKeyInput,
          onChange: setAlibabaApiKeyInput,
          onSubmit: submitAlibabaApiKey,
          onDelete: handleDeleteAlibabaApiKey,
          placeholder: "sk-...",
          saving: savingAlibabaKey,
          deleting: deletingAlibabaKey,
          notSet: t("settings.alibabaNotSet"),
        },
        {
          id: "mistral",
          title: t("settings.mistralTitle"),
          description: t("settings.mistralDescription"),
          configured: settings.has_mistral_api_key,
          last4: settings.mistral_api_key_last4,
          value: mistralApiKeyInput,
          onChange: setMistralApiKeyInput,
          onSubmit: submitMistralApiKey,
          onDelete: handleDeleteMistralApiKey,
          placeholder: "sk-...",
          saving: savingMistralKey,
          deleting: deletingMistralKey,
          notSet: t("settings.mistralNotSet"),
        },
        {
          id: "moonshot",
          title: t("settings.moonshotTitle"),
          description: t("settings.moonshotDescription"),
          configured: settings.has_moonshot_api_key,
          last4: settings.moonshot_api_key_last4,
          value: moonshotApiKeyInput,
          onChange: setMoonshotApiKeyInput,
          onSubmit: submitMoonshotApiKey,
          onDelete: handleDeleteMoonshotApiKey,
          placeholder: "sk-...",
          saving: savingMoonshotKey,
          deleting: deletingMoonshotKey,
          notSet: t("settings.moonshotNotSet"),
        },
        {
          id: "xai",
          title: t("settings.xaiTitle"),
          description: t("settings.xaiDescription"),
          configured: settings.has_xai_api_key,
          last4: settings.xai_api_key_last4,
          value: xaiApiKeyInput,
          onChange: setXaiApiKeyInput,
          onSubmit: submitXAIApiKey,
          onDelete: handleDeleteXAIApiKey,
          placeholder: "xai-...",
          saving: savingXAIKey,
          deleting: deletingXAIKey,
          notSet: t("settings.xaiNotSet"),
        },
        {
          id: "zai",
          title: t("settings.zaiTitle"),
          description: t("settings.zaiDescription"),
          configured: settings.has_zai_api_key,
          last4: settings.zai_api_key_last4,
          value: zaiApiKeyInput,
          onChange: setZaiApiKeyInput,
          onSubmit: submitZAIApiKey,
          onDelete: handleDeleteZAIApiKey,
          placeholder: "zai-...",
          saving: savingZAIKey,
          deleting: deletingZAIKey,
          notSet: t("settings.zaiNotSet"),
        },
        {
          id: "fireworks",
          title: t("settings.fireworksTitle"),
          description: t("settings.fireworksDescription"),
          configured: settings.has_fireworks_api_key,
          last4: settings.fireworks_api_key_last4,
          value: fireworksApiKeyInput,
          onChange: setFireworksApiKeyInput,
          onSubmit: submitFireworksApiKey,
          onDelete: handleDeleteFireworksApiKey,
          placeholder: "fw_...",
          saving: savingFireworksKey,
          deleting: deletingFireworksKey,
          notSet: t("settings.fireworksNotSet"),
        },
        {
          id: "poe",
          title: t("settings.poeTitle"),
          description: t("settings.poeDescription"),
          configured: settings.has_poe_api_key,
          last4: settings.poe_api_key_last4,
          value: poeApiKeyInput,
          onChange: setPoeApiKeyInput,
          onSubmit: submitPoeApiKey,
          onDelete: handleDeletePoeApiKey,
          placeholder: "sk-...",
          saving: savingPoeKey,
          deleting: deletingPoeKey,
          notSet: t("settings.poeNotSet"),
        },
        {
          id: "siliconflow",
          title: t("settings.siliconflowTitle"),
          description: t("settings.siliconflowDescription"),
          configured: settings.has_siliconflow_api_key,
          last4: settings.siliconflow_api_key_last4,
          value: siliconFlowApiKeyInput,
          onChange: setSiliconFlowApiKeyInput,
          onSubmit: submitSiliconFlowApiKey,
          onDelete: handleDeleteSiliconFlowApiKey,
          placeholder: "sk-...",
          saving: savingSiliconFlowKey,
          deleting: deletingSiliconFlowKey,
          notSet: t("settings.siliconflowNotSet"),
        },
        {
          id: "openrouter",
          title: t("settings.openrouterTitle"),
          description: t("settings.openrouterDescription"),
          configured: settings.has_openrouter_api_key,
          last4: settings.openrouter_api_key_last4,
          value: openRouterApiKeyInput,
          onChange: setOpenRouterApiKeyInput,
          onSubmit: submitOpenRouterApiKey,
          onDelete: handleDeleteOpenRouterApiKey,
          placeholder: "sk-or-v1-...",
          saving: savingOpenRouterKey,
          deleting: deletingOpenRouterKey,
          notSet: t("settings.openrouterNotSet"),
        },
        {
          id: "aivis",
          title: t("settings.aivisTitle"),
          description: t("settings.aivisDescription"),
          configured: settings.has_aivis_api_key,
          last4: settings.aivis_api_key_last4,
          value: aivisApiKeyInput,
          onChange: setAivisApiKeyInput,
          onSubmit: submitAivisApiKey,
          onDelete: handleDeleteAivisApiKey,
          placeholder: "sk-...",
          saving: savingAivisKey,
          deleting: deletingAivisKey,
          notSet: t("settings.aivisNotSet"),
        },
        {
          id: "fish",
          title: t("settings.fishTitle"),
          description: t("settings.fishDescription"),
          configured: Boolean(settings.has_fish_api_key),
          last4: settings.fish_api_key_last4 ?? null,
          value: fishApiKeyInput,
          onChange: setFishApiKeyInput,
          onSubmit: submitFishApiKey,
          onDelete: handleDeleteFishApiKey,
          placeholder: "fish_...",
          saving: savingFishKey,
          deleting: deletingFishKey,
          notSet: t("settings.fishNotSet"),
        },
      ]
    : [];

  const configuredProviderCount = accessCards.filter((card) => card.configured).length;
  const activeAccessCard = accessCards.find((card) => card.id === activeAccessProvider) ?? accessCards[0];

  function dismissProviderModelUpdates() {
    const latest = providerModelUpdates.reduce<string | null>((max, event) => {
      if (!max) return event.detected_at;
      return Date.parse(event.detected_at) > Date.parse(max) ? event.detected_at : max;
    }, null);
    if (!latest || typeof window === "undefined") return;
    window.localStorage.setItem(MODEL_UPDATES_DISMISSED_AT_KEY, latest);
    setDismissedModelUpdatesAt(latest);
  }

  function restoreProviderModelUpdates() {
    if (typeof window === "undefined") return;
    window.localStorage.removeItem(MODEL_UPDATES_DISMISSED_AT_KEY);
    setDismissedModelUpdatesAt(null);
  }

  function toggleLLMExtras() {
    setLLMExtrasOpen((prev) => {
      const next = !prev;
      if (next) {
        window.requestAnimationFrame(() => {
          llmExtrasRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
        });
      }
      return next;
    });
  }

  async function submitBudget(e: FormEvent) {
    e.preventDefault();
    setSavingBudget(true);
    try {
      const parsed = budgetUSD.trim() === "" ? null : Number(budgetUSD);
      if (parsed != null && (!Number.isFinite(parsed) || parsed < 0)) {
        throw new Error(t("settings.error.invalidBudget"));
      }
      await api.updateSettings({
        monthly_budget_usd: parsed,
        budget_alert_enabled: alertEnabled,
        budget_alert_threshold_pct: thresholdPct,
        digest_email_enabled: digestEmailEnabled,
      });
      await load();
      showToast(t("settings.toast.budgetSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingBudget(false);
    }
  }

  async function submitLLMModels(e: FormEvent) {
    e.preventDefault();
    setSavingLLMModels(true);
    try {
      await persistLLMModels(buildLLMModelPayload(), t("settings.toast.modelsSaved"));
    } catch (e) {
      showToast(localizeSettingsErrorMessage(e, t), "error");
    } finally {
      setSavingLLMModels(false);
    }
  }

  async function submitAudioBriefingModels(e: FormEvent) {
    e.preventDefault();
    setSavingLLMModels(true);
    try {
      await persistLLMModels(
        buildLLMModelPayload({
          audio_briefing_script: audioBriefingScriptModel || null,
          audio_briefing_script_fallback: audioBriefingScriptFallbackModel || null,
        }),
        t("settings.toast.modelsSaved")
      );
    } catch (e) {
      showToast(localizeSettingsErrorMessage(e, t), "error");
    } finally {
      setSavingLLMModels(false);
    }
  }

  async function submitDigestDelivery(e: FormEvent) {
    e.preventDefault();
    if (!settings) return;
    setSavingDigestDelivery(true);
    try {
      await api.updateSettings({
        monthly_budget_usd: settings.monthly_budget_usd,
        budget_alert_enabled: settings.budget_alert_enabled,
        budget_alert_threshold_pct: settings.budget_alert_threshold_pct,
        digest_email_enabled: digestEmailEnabled,
      });
      await load();
      showToast(t("settings.toast.digestSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingDigestDelivery(false);
    }
  }

  async function submitReadingPlan(e: FormEvent) {
    e.preventDefault();
    setSavingReadingPlan(true);
    try {
      const parsedSize = Number(readingPlanSize);
      if (!(parsedSize === 7 || parsedSize === 15 || parsedSize === 25)) {
        throw new Error(t("settings.error.invalidSize"));
      }
      await api.updateReadingPlanSettings({
        window: readingPlanWindow,
        size: parsedSize,
        diversify_topics: readingPlanDiversifyTopics,
      });
      await load();
      showToast(t("settings.toast.readingPlanSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingReadingPlan(false);
    }
  }

  async function submitObsidianExport(e: FormEvent) {
    e.preventDefault();
    setSavingObsidianExport(true);
    try {
      const resp = await api.updateObsidianExport({
        enabled: obsidianEnabled,
        github_repo_owner: obsidianRepoOwner.trim() || null,
        github_repo_name: obsidianRepoName.trim() || null,
        github_repo_branch: obsidianRepoBranch.trim() || null,
        vault_root_path: obsidianRootPath.trim() || null,
        keyword_link_mode: "topics_only",
      });
      setSettings((prev) => (prev ? { ...prev, obsidian_export: resp.obsidian_export } : prev));
      showToast(t("settings.toast.obsidianExportSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingObsidianExport(false);
    }
  }

  async function runObsidianExportNow() {
    setRunningObsidianExport(true);
    try {
      const res = await api.runObsidianExportNow();
      await load();
      showToast(
        `${t("settings.toast.obsidianExportRunNowResult")} updated=${res.updated} skipped=${res.skipped} failed=${res.failed}`,
        res.failed > 0 ? "error" : "success"
      );
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setRunningObsidianExport(false);
    }
  }

  async function submitAnthropicApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingAnthropicKey(true);
    try {
      if (!anthropicApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setAnthropicApiKey(anthropicApiKeyInput.trim());
      setAnthropicApiKeyInput("");
      await load();
      showToast(t("settings.toast.anthropicSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingAnthropicKey(false);
    }
  }

  async function handleDeleteAnthropicApiKey() {
    if (!(await confirm({
      title: t("settings.anthropicDeleteTitle"),
      message:
        t("settings.anthropicDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingAnthropicKey(true);
    try {
      await api.deleteAnthropicApiKey();
      await load();
      showToast(t("settings.toast.anthropicDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingAnthropicKey(false);
    }
  }

  async function submitOpenAIApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingOpenAIKey(true);
    try {
      if (!openAIApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setOpenAIApiKey(openAIApiKeyInput.trim());
      setOpenAIApiKeyInput("");
      await load();
      showToast(t("settings.toast.openaiSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingOpenAIKey(false);
    }
  }

  async function handleDeleteOpenAIApiKey() {
    if (!(await confirm({
      title: t("settings.openaiDeleteTitle"),
      message:
        t("settings.openaiDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingOpenAIKey(true);
    try {
      await api.deleteOpenAIApiKey();
      await load();
      showToast(t("settings.toast.openaiDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingOpenAIKey(false);
    }
  }

  async function submitGoogleApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingGoogleKey(true);
    try {
      if (!googleApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setGoogleApiKey(googleApiKeyInput.trim());
      setGoogleApiKeyInput("");
      await load();
      showToast(t("settings.toast.googleSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingGoogleKey(false);
    }
  }

  async function handleDeleteGoogleApiKey() {
    if (!(await confirm({
      title: t("settings.googleDeleteTitle"),
      message:
        t("settings.googleDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingGoogleKey(true);
    try {
      await api.deleteGoogleApiKey();
      await load();
      showToast(t("settings.toast.googleDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingGoogleKey(false);
    }
  }

  async function submitGroqApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingGroqKey(true);
    try {
      if (!groqApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setGroqApiKey(groqApiKeyInput.trim());
      setGroqApiKeyInput("");
      await load();
      showToast(t("settings.toast.groqSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingGroqKey(false);
    }
  }

  async function handleDeleteGroqApiKey() {
    if (!(await confirm({
      title: t("settings.groqDeleteTitle"),
      message: t("settings.groqDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingGroqKey(true);
    try {
      await api.deleteGroqApiKey();
      await load();
      showToast(t("settings.toast.groqDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingGroqKey(false);
    }
  }

  async function submitDeepSeekApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingDeepSeekKey(true);
    try {
      if (!deepseekApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setDeepSeekApiKey(deepseekApiKeyInput.trim());
      setDeepseekApiKeyInput("");
      await load();
      showToast(t("settings.toast.deepseekSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingDeepSeekKey(false);
    }
  }

  async function handleDeleteDeepSeekApiKey() {
    if (!(await confirm({
      title: t("settings.deepseekDeleteTitle"),
      message: t("settings.deepseekDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingDeepSeekKey(true);
    try {
      await api.deleteDeepSeekApiKey();
      await load();
      showToast(t("settings.toast.deepseekDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingDeepSeekKey(false);
    }
  }

  async function submitAlibabaApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingAlibabaKey(true);
    try {
      if (!alibabaApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setAlibabaApiKey(alibabaApiKeyInput.trim());
      setAlibabaApiKeyInput("");
      await load();
      showToast(t("settings.toast.alibabaSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingAlibabaKey(false);
    }
  }

  async function handleDeleteAlibabaApiKey() {
    if (!(await confirm({
      title: t("settings.alibabaDeleteTitle"),
      message: t("settings.alibabaDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingAlibabaKey(true);
    try {
      await api.deleteAlibabaApiKey();
      await load();
      showToast(t("settings.toast.alibabaDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingAlibabaKey(false);
    }
  }

  async function submitMistralApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingMistralKey(true);
    try {
      if (!mistralApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setMistralApiKey(mistralApiKeyInput.trim());
      setMistralApiKeyInput("");
      await load();
      showToast(t("settings.toast.mistralSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingMistralKey(false);
    }
  }

  async function handleDeleteMistralApiKey() {
    if (!(await confirm({
      title: t("settings.mistralDeleteTitle"),
      message: t("settings.mistralDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingMistralKey(true);
    try {
      await api.deleteMistralApiKey();
      await load();
      showToast(t("settings.toast.mistralDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingMistralKey(false);
    }
  }

  async function submitMoonshotApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingMoonshotKey(true);
    try {
      if (!moonshotApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setMoonshotApiKey(moonshotApiKeyInput.trim());
      setMoonshotApiKeyInput("");
      await load();
      showToast(t("settings.toast.moonshotSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingMoonshotKey(false);
    }
  }

  async function handleDeleteMoonshotApiKey() {
    if (!(await confirm({
      title: t("settings.moonshotDeleteTitle"),
      message: t("settings.moonshotDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingMoonshotKey(true);
    try {
      await api.deleteMoonshotApiKey();
      await load();
      showToast(t("settings.toast.moonshotDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingMoonshotKey(false);
    }
  }

  async function submitXAIApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingXAIKey(true);
    try {
      if (!xaiApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setXAIApiKey(xaiApiKeyInput.trim());
      setXaiApiKeyInput("");
      setXAIVoicesData(null);
      setXAIVoicesError(null);
      await load();
      showToast(t("settings.toast.xaiSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingXAIKey(false);
    }
  }

  async function handleDeleteXAIApiKey() {
    if (!(await confirm({
      title: t("settings.xaiDeleteTitle"),
      message: t("settings.xaiDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingXAIKey(true);
    try {
      await api.deleteXAIApiKey();
      setXAIVoicesData(null);
      setXAIVoicesError(null);
      await load();
      showToast(t("settings.toast.xaiDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingXAIKey(false);
    }
  }

  async function submitZAIApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingZAIKey(true);
    try {
      if (!zaiApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setZAIApiKey(zaiApiKeyInput.trim());
      setZaiApiKeyInput("");
      await load();
      showToast(t("settings.toast.zaiSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingZAIKey(false);
    }
  }

  async function handleDeleteZAIApiKey() {
    if (!(await confirm({
      title: t("settings.zaiDeleteTitle"),
      message: t("settings.zaiDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingZAIKey(true);
    try {
      await api.deleteZAIApiKey();
      await load();
      showToast(t("settings.toast.zaiDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingZAIKey(false);
    }
  }

  async function submitFireworksApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingFireworksKey(true);
    try {
      if (!fireworksApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setFireworksApiKey(fireworksApiKeyInput.trim());
      setFireworksApiKeyInput("");
      await load();
      showToast(t("settings.toast.fireworksSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingFireworksKey(false);
    }
  }

  async function handleDeleteFireworksApiKey() {
    if (!(await confirm({
      title: t("settings.fireworksDeleteTitle"),
      message: t("settings.fireworksDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingFireworksKey(true);
    try {
      await api.deleteFireworksApiKey();
      await load();
      showToast(t("settings.toast.fireworksDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingFireworksKey(false);
    }
  }

  async function submitOpenRouterApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingOpenRouterKey(true);
    try {
      if (!openRouterApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setOpenRouterApiKey(openRouterApiKeyInput.trim());
      setOpenRouterApiKeyInput("");
      await load();
      showToast(t("settings.toast.openrouterSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingOpenRouterKey(false);
    }
  }

  async function submitPoeApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingPoeKey(true);
    try {
      if (!poeApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setPoeApiKey(poeApiKeyInput.trim());
      setPoeApiKeyInput("");
      await load();
      showToast(t("settings.toast.poeSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingPoeKey(false);
    }
  }

  async function submitSiliconFlowApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingSiliconFlowKey(true);
    try {
      if (!siliconFlowApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setSiliconFlowApiKey(siliconFlowApiKeyInput.trim());
      setSiliconFlowApiKeyInput("");
      await load();
      showToast(t("settings.toast.siliconflowSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingSiliconFlowKey(false);
    }
  }

  async function handleDeletePoeApiKey() {
    if (!(await confirm({
      title: t("settings.poeDeleteTitle"),
      message: t("settings.poeDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingPoeKey(true);
    try {
      await api.deletePoeApiKey();
      await load();
      showToast(t("settings.toast.poeDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingPoeKey(false);
    }
  }

  async function handleDeleteOpenRouterApiKey() {
    if (!(await confirm({
      title: t("settings.openrouterDeleteTitle"),
      message: t("settings.openrouterDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingOpenRouterKey(true);
    try {
      await api.deleteOpenRouterApiKey();
      await load();
      showToast(t("settings.toast.openrouterDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingOpenRouterKey(false);
    }
  }

  async function handleDeleteSiliconFlowApiKey() {
    if (!(await confirm({
      title: t("settings.siliconflowDeleteTitle"),
      message: t("settings.siliconflowDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingSiliconFlowKey(true);
    try {
      await api.deleteSiliconFlowApiKey();
      await load();
      showToast(t("settings.toast.siliconflowDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingSiliconFlowKey(false);
    }
  }

  async function submitAivisApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingAivisKey(true);
    try {
      if (!aivisApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setAivisApiKey(aivisApiKeyInput.trim());
      setAivisApiKeyInput("");
      setAivisUserDictionariesLoaded(false);
      await load();
      showToast(t("settings.toast.aivisSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingAivisKey(false);
    }
  }

  async function submitFishApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingFishKey(true);
    try {
      if (!fishApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setFishApiKey(fishApiKeyInput.trim());
      setFishApiKeyInput("");
      await load();
      showToast(t("settings.toast.fishSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingFishKey(false);
    }
  }

  async function handleDeleteAivisApiKey() {
    if (!(await confirm({
      title: t("settings.aivisDeleteTitle"),
      message: t("settings.aivisDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingAivisKey(true);
    try {
      await api.deleteAivisApiKey();
      setAivisUserDictionaryUUID("");
      setAivisUserDictionaries([]);
      setAivisUserDictionariesLoaded(false);
      setAivisUserDictionariesError(null);
      await load();
      showToast(t("settings.toast.aivisDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingAivisKey(false);
    }
  }

  async function handleDeleteFishApiKey() {
    if (!(await confirm({
      title: t("settings.fishDeleteTitle"),
      message: t("settings.fishDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingFishKey(true);
    try {
      await api.deleteFishApiKey();
      await load();
      showToast(t("settings.toast.fishDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingFishKey(false);
    }
  }

  async function saveAivisUserDictionary() {
    if (!aivisUserDictionaryUUID) {
      showToast(t("settings.aivisDictionarySelectRequired"), "error");
      return;
    }
    setSavingAivisDictionary(true);
    try {
      const next = await api.setAivisUserDictionary(aivisUserDictionaryUUID);
      setAivisUserDictionaryUUID(next.aivis_user_dictionary_uuid ?? "");
      setSettings((prev) => prev ? {
        ...prev,
        aivis_user_dictionary_uuid: next.aivis_user_dictionary_uuid ?? null,
      } : prev);
      showToast(t("settings.toast.aivisDictionarySaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingAivisDictionary(false);
    }
  }

  async function clearAivisUserDictionary() {
    setDeletingAivisDictionary(true);
    try {
      const next = await api.deleteAivisUserDictionary();
      setAivisUserDictionaryUUID("");
      setSettings((prev) => prev ? {
        ...prev,
        aivis_user_dictionary_uuid: next.aivis_user_dictionary_uuid ?? null,
      } : prev);
      showToast(t("settings.toast.aivisDictionaryDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingAivisDictionary(false);
    }
  }

  async function handleDeleteInoreaderOAuth() {
    if (!(await confirm({
      title: t("settings.inoreaderDeleteTitle"),
      message: t("settings.inoreaderDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingInoreaderOAuth(true);
    try {
      await api.deleteInoreaderOAuth();
      await load();
      showToast(t("settings.toast.inoreaderDisconnected"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingInoreaderOAuth(false);
    }
  }

  async function handleResetPreferenceProfile() {
    if (!(await confirm({
      title: t("settings.personalization.resetTitle"),
      message: t("settings.personalization.resetMessage"),
      confirmLabel: t("settings.personalization.reset"),
      tone: "danger",
    }))) {
      return;
    }
    setResettingPreferenceProfile(true);
    try {
      await api.resetPreferenceProfile();
      await load();
      showToast(t("settings.personalization.resetDone"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setResettingPreferenceProfile(false);
    }
  }

  async function persistAudioBriefingSettings() {
    if (audioBriefingConversationMode === "duo" && configuredAudioBriefingVoiceCount < 2) {
      showToast(t("settings.audioBriefing.duoRequiresTwoVoices"), "error");
      return;
    }
    setSavingAudioBriefing(true);
    try {
      const schedulePayload = buildAudioBriefingSchedulePayload(audioBriefingScheduleSelection);
      const payload = {
        enabled: audioBriefingEnabled,
        ...schedulePayload,
        articles_per_episode: Number(audioBriefingArticlesPerEpisode),
        target_duration_minutes: Number(audioBriefingTargetDurationMinutes),
        chunk_trailing_silence_seconds: Number(audioBriefingChunkTrailingSilenceSeconds),
        program_name: audioBriefingProgramName.trim() || null,
        default_persona_mode: audioBriefingDefaultPersonaMode,
        default_persona: audioBriefingDefaultPersona,
        conversation_mode: audioBriefingConversationMode,
        bgm_enabled: audioBriefingBGMEnabled,
        bgm_r2_prefix: audioBriefingBGMR2Prefix.trim() || null,
      };
      const resp = await api.updateAudioBriefingSettings(payload);
      setSettings((prev) => (prev ? { ...prev, audio_briefing: resp.audio_briefing } : prev));
      syncAudioBriefingSettingsForm(resp.audio_briefing);
      showToast(t("settings.toast.audioBriefingSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingAudioBriefing(false);
    }
  }

  async function submitAudioBriefingSettings(e: FormEvent) {
    e.preventDefault();
    await persistAudioBriefingSettings();
  }

  async function persistPodcastSettings() {
    setSavingPodcast(true);
    try {
      const resp = await api.updatePodcastSettings({
        enabled: podcastEnabled,
        feed_slug: podcastFeedSlug || null,
        rss_url: podcastRSSURL || null,
        title: podcastTitle || null,
        description: podcastDescription || null,
        author: podcastAuthor || null,
        language: podcastLanguage || "ja",
        category: podcastCategory || null,
        subcategory: podcastSubcategory || null,
        explicit: podcastExplicit,
        artwork_url: podcastArtworkURL || null,
      });
      setSettings((prev) => (prev ? { ...prev, podcast: resp.podcast } : prev));
      syncPodcastForm(resp.podcast);
      showToast(t("settings.toast.podcastSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingPodcast(false);
    }
  }

  async function submitPodcastSettings(e: FormEvent) {
    e.preventDefault();
    await persistPodcastSettings();
  }

  async function copyPodcastRSSURL() {
    if (!podcastRSSURL) return;
    try {
      await navigator.clipboard.writeText(podcastRSSURL);
      showToast(t("settings.toast.podcastRSSCopied"), "success");
    } catch (e) {
      showToast(String(e), "error");
    }
  }

  async function handlePodcastArtworkFileChange(file: File | null) {
    if (!file) return;
    setUploadingPodcastArtwork(true);
    try {
      const dataURL = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result ?? ""));
        reader.onerror = () => reject(new Error("failed to read artwork file"));
        reader.readAsDataURL(file);
      });
      const marker = "base64,";
      const index = dataURL.indexOf(marker);
      if (index < 0) {
        throw new Error("invalid artwork file");
      }
      const contentBase64 = dataURL.slice(index + marker.length);
      const resp = await api.uploadPodcastArtwork({
        content_type: file.type || "image/jpeg",
        content_base64: contentBase64,
      });
      setPodcastArtworkURL(resp.artwork_url ?? "");
      setSettings((prev) =>
        prev
          ? {
              ...prev,
              podcast: {
                enabled: prev.podcast?.enabled ?? podcastEnabled,
                feed_slug: prev.podcast?.feed_slug ?? (podcastFeedSlug || null),
                rss_url: prev.podcast?.rss_url ?? (podcastRSSURL || null),
                title: prev.podcast?.title ?? (podcastTitle || null),
                description: prev.podcast?.description ?? (podcastDescription || null),
                author: prev.podcast?.author ?? (podcastAuthor || null),
                language: prev.podcast?.language ?? podcastLanguage,
                category: prev.podcast?.category ?? (podcastCategory || null),
                subcategory: prev.podcast?.subcategory ?? (podcastSubcategory || null),
                available_categories: prev.podcast?.available_categories ?? podcastAvailableCategories,
                explicit: prev.podcast?.explicit ?? podcastExplicit,
                artwork_url: resp.artwork_url ?? null,
              },
            }
          : prev
      );
      showToast(t("settings.toast.podcastArtworkUploaded"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setUploadingPodcastArtwork(false);
    }
  }

  async function persistAudioBriefingVoices() {
    setSavingAudioBriefingVoices(true);
    try {
      const resp = await api.updateAudioBriefingPersonaVoices(audioBriefingVoices);
      setSettings((prev) => (prev ? { ...prev, audio_briefing_persona_voices: resp.audio_briefing_persona_voices } : prev));
      syncAudioBriefingVoiceForm(resp.audio_briefing_persona_voices);
      showToast(t("settings.toast.audioBriefingVoicesSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingAudioBriefingVoices(false);
    }
  }

  async function submitAudioBriefingVoices(e: FormEvent) {
    e.preventDefault();
    await persistAudioBriefingVoices();
  }

  async function submitAudioBriefingPresetSave() {
    const name = audioBriefingPresetName.trim();
    if (!name) {
      showToast(t("settings.audioBriefing.presetNameRequired"), "error");
      return;
    }
    setAudioBriefingPresetSaving(true);
    try {
      const presets = audioBriefingPresetsLoaded ? audioBriefingPresets : await api.listAudioBriefingPresets();
      if (!audioBriefingPresetsLoaded) {
        setAudioBriefingPresets(presets);
        setAudioBriefingPresetsLoaded(true);
        setAudioBriefingPresetsError(null);
      }
      const existing = presets.find((preset) => preset.name.trim() === name);
      const payload = buildAudioBriefingPresetRequest(
        name,
        audioBriefingDefaultPersonaMode,
        audioBriefingDefaultPersona,
        audioBriefingConversationMode,
        audioBriefingVoices,
      );
      const saved = existing
        ? await (async () => {
            const ok = await confirm({
              title: t("settings.audioBriefing.presetOverwriteTitle"),
              message: t("settings.audioBriefing.presetOverwriteMessage").replace("{{name}}", name),
              confirmLabel: t("settings.audioBriefing.presetOverwriteConfirm"),
            });
            if (!ok) return null;
            return api.updateAudioBriefingPreset(existing.id, payload);
          })()
        : await api.createAudioBriefingPreset(payload);
      if (!saved) return;
      setAudioBriefingPresets((prev) => [saved, ...prev.filter((preset) => preset.id !== saved.id)]);
      setAudioBriefingPresetSaveOpen(false);
      setAudioBriefingPresetName("");
      showToast(existing ? t("settings.audioBriefing.presetUpdated") : t("settings.audioBriefing.presetSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setAudioBriefingPresetSaving(false);
    }
  }

  function openAudioBriefingPresetSaveModal() {
    setAudioBriefingPresetName("");
    setAudioBriefingPresetSaveOpen(true);
  }

  async function openAudioBriefingPresetApplyModal() {
    setAudioBriefingPresetApplySelection(null);
    setAudioBriefingPresetApplyOpen(true);
    if (audioBriefingPresets.length === 0) {
      await loadAudioBriefingPresets();
    }
  }

  function applyAudioBriefingPreset(preset: AudioBriefingPreset) {
    syncAudioBriefingPresetForm(preset);
    setAudioBriefingPresetApplySelection(preset.id);
    setAudioBriefingPresetApplyOpen(false);
    showToast(t("settings.audioBriefing.presetApplied"), "success");
  }

  async function persistSummaryAudioSettings() {
    setSummaryAudioSaving(true);
    try {
      const payload: SummaryAudioVoiceSettings = {
        tts_provider: summaryAudioProvider.trim() || "aivis",
        tts_model: summaryAudioTTSModel.trim(),
        voice_model: summaryAudioVoiceModel.trim(),
        voice_style: summaryAudioVoiceStyle.trim(),
        provider_voice_label: summaryAudioProviderVoiceLabel.trim(),
        provider_voice_description: summaryAudioProviderVoiceDescription.trim(),
        speech_rate: Number(summaryAudioSpeechRate),
        emotional_intensity: Number(summaryAudioEmotionalIntensity),
        tempo_dynamics: Number(summaryAudioTempoDynamics),
        line_break_silence_seconds: Number(summaryAudioLineBreakSilenceSeconds),
        pitch: Number(summaryAudioPitch),
        volume_gain: Number(summaryAudioVolumeGain),
        aivis_user_dictionary_uuid: summaryAudioAivisUserDictionaryUUID.trim() || null,
      };
      const resp = await api.updateSummaryAudioSettings(payload);
      const nextSettings = settings ? { ...settings, summary_audio: resp.summary_audio } : null;
      setSettings(nextSettings);
      if (nextSettings) {
        queryClient.setQueryData(["shared-audio-player-settings"], nextSettings);
        queryClient.setQueryData(["settings", "summary-audio-readiness"], nextSettings);
      }
      syncSummaryAudioForm(resp.summary_audio ?? null);
      void queryClient.invalidateQueries({ queryKey: ["shared-audio-player-settings"] });
      void queryClient.invalidateQueries({ queryKey: ["settings", "summary-audio-readiness"] });
      showToast(t("settings.toast.summaryAudioSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSummaryAudioSaving(false);
    }
  }

  async function submitSummaryAudioSettings(e: FormEvent) {
    e.preventDefault();
    await persistSummaryAudioSettings();
  }

  function updateSummaryAudioProvider(nextProvider: string) {
    const normalized = nextProvider.trim().toLowerCase();
    const capabilities = getAudioBriefingProviderCapabilities(normalized);
    setSummaryAudioProvider(normalized);
    setSummaryAudioVoiceModel("");
    setSummaryAudioVoiceStyle("");
    setSummaryAudioProviderVoiceLabel("");
    setSummaryAudioProviderVoiceDescription("");
    if (!capabilities.supportsSeparateTTSModel) {
      setSummaryAudioTTSModel("");
    } else if (normalized === "fish") {
      setSummaryAudioTTSModel("s2-pro");
    } else if (normalized === "openai") {
      setSummaryAudioTTSModel("tts-1");
    } else if (normalized === "gemini_tts") {
      setSummaryAudioTTSModel("gemini-2.5-flash-tts");
    } else {
      setSummaryAudioTTSModel("");
    }
    if (!capabilities.requiresVoiceStyle) {
      setSummaryAudioVoiceStyle("");
    }
  }

  function updateSummaryAudioVoiceNumberInput(
    field: SummaryAudioNumericInputField,
    raw: string,
    applyParsedValue: (value: number) => void
  ) {
    setSummaryAudioVoiceInputDrafts((prev) => ({
      ...prev,
      [field]: raw,
    }));
    if (!raw || !isCompleteDecimalInput(raw)) return;
    const parsed = Number(raw);
    if (!Number.isFinite(parsed)) return;
    applyParsedValue(parsed);
  }

  function resetSummaryAudioVoiceNumberInput(field: SummaryAudioNumericInputField) {
    setSummaryAudioVoiceInputDrafts((prev) => ({
      ...prev,
      [field]: buildSummaryAudioVoiceInputDrafts({
        tts_provider: summaryAudioProvider,
        tts_model: summaryAudioTTSModel,
        voice_model: summaryAudioVoiceModel,
        voice_style: summaryAudioVoiceStyle,
        provider_voice_label: summaryAudioProviderVoiceLabel,
        provider_voice_description: summaryAudioProviderVoiceDescription,
        speech_rate: Number(summaryAudioSpeechRate),
        emotional_intensity: Number(summaryAudioEmotionalIntensity),
        tempo_dynamics: Number(summaryAudioTempoDynamics),
        line_break_silence_seconds: Number(summaryAudioLineBreakSilenceSeconds),
        pitch: Number(summaryAudioPitch),
        volume_gain: Number(summaryAudioVolumeGain),
        aivis_user_dictionary_uuid: summaryAudioAivisUserDictionaryUUID || null,
      })[field],
    }));
  }

  function updateAudioBriefingVoice(persona: string, patch: Partial<AudioBriefingPersonaVoice>) {
    setAudioBriefingVoices((prev) =>
      prev.map((voice) => {
        if (voice.persona !== persona) return voice;
        const nextVoice = { ...voice, ...patch };
        if ("tts_provider" in patch) {
          const nextProvider = nextVoice.tts_provider.trim().toLowerCase();
          const capabilities = getAudioBriefingProviderCapabilities(nextProvider);
          if (!capabilities.supportsSeparateTTSModel) {
            nextVoice.tts_model = "";
          } else if (nextProvider === "fish") {
            nextVoice.tts_model = "s2-pro";
          } else if (nextProvider === "openai") {
            nextVoice.tts_model = "gpt-4o-mini-tts";
          } else if (nextProvider === "gemini_tts") {
            nextVoice.tts_model = "gemini-2.5-flash-tts";
          } else {
            nextVoice.tts_model = "";
          }
          if (!capabilities.requiresVoiceStyle) {
            nextVoice.voice_style = "";
          }
          if (nextProvider !== "fish") {
            nextVoice.provider_voice_label = "";
            nextVoice.provider_voice_description = "";
          }
          setAudioBriefingVoiceInputDrafts((drafts) => ({
            ...drafts,
            [persona]: buildAudioBriefingVoiceInputDrafts([nextVoice])[persona],
          }));
        }
        return nextVoice;
      })
    );
  }

  function updateAudioBriefingVoiceNumberInput(
    persona: string,
    field: AudioBriefingNumericInputField,
    raw: string,
    applyParsedValue: (value: number) => Partial<AudioBriefingPersonaVoice>
  ) {
    setAudioBriefingVoiceInputDrafts((prev) => ({
      ...prev,
      [persona]: {
        ...prev[persona],
        [field]: raw,
      },
    }));
    if (!raw || !isCompleteDecimalInput(raw)) return;
    const parsed = Number(raw);
    if (!Number.isFinite(parsed)) return;
    updateAudioBriefingVoice(persona, applyParsedValue(parsed));
  }

  function resetAudioBriefingVoiceNumberInput(persona: string, field: AudioBriefingNumericInputField) {
    setAudioBriefingVoiceInputDrafts((prev) => {
      const voice = audioBriefingVoices.find((item) => item.persona === persona);
      if (!voice) return prev;
      return {
        ...prev,
        [persona]: {
          ...prev[persona],
          [field]: buildAudioBriefingVoiceInputDrafts([voice])[persona][field],
        },
      };
    });
  }

  function toggleAudioBriefingPersona(persona: string) {
    setExpandedAudioBriefingPersonas((prev) =>
      prev.includes(persona) ? prev.filter((item) => item !== persona) : [...prev, persona]
    );
  }

  async function openAivisPicker(persona: string) {
    setAivisPickerPersona(persona);
    if (aivisModelsData == null) {
      try {
        await loadAivisModels();
      } catch {
        return;
      }
    }
  }

  async function openXAIPicker(persona: string) {
    setXAIPickerPersona(persona);
    if (xaiVoicesData == null) {
      try {
        await loadXAIVoices();
      } catch {
        return;
      }
    }
  }

  async function openFishPicker(persona: string) {
    setFishPickerPersona(persona);
  }

  async function openOpenAITTSPicker(persona: string) {
    setOpenAITTPickerPersona(persona);
    if (openAITTSVoicesData == null) {
      try {
        await loadOpenAITTSVoices();
      } catch {
        return;
      }
    }
  }

  async function openGeminiTTSPicker(persona: string) {
    setGeminiTTSPickerPersona(persona);
    if (geminiTTSVoicesData == null) {
      try {
        await loadGeminiTTSVoices();
      } catch {
        return;
      }
    }
  }

  if (loading) return <p className="text-sm text-zinc-500">{t("common.loading")}</p>;
  if (error) return <p className="text-sm text-red-500">{error}</p>;
  if (!settings) return null;

  const activeAivisVoice = aivisPickerPersona
    ? audioBriefingVoices.find((voice) => voice.persona === aivisPickerPersona) ?? null
    : null;
  const activeXAIVoice = xaiPickerPersona
    ? audioBriefingVoices.find((voice) => voice.persona === xaiPickerPersona) ?? null
    : null;
  const activeOpenAITTSVoice = openAITTPickerPersona
    ? audioBriefingVoices.find((voice) => voice.persona === openAITTPickerPersona) ?? null
    : null;
  const activeGeminiTTSVoice = geminiTTSPickerPersona
    ? audioBriefingVoices.find((voice) => voice.persona === geminiTTSPickerPersona) ?? null
    : null;
  const audioBriefingAivisModels = aivisModelsData?.models ?? [];
  const audioBriefingXAIVoices = xaiVoicesData?.voices ?? [];
  const audioBriefingOpenAITTSVoices = openAITTSVoicesData?.voices ?? [];
  const audioBriefingGeminiTTSVoices = geminiTTSVoicesData?.voices ?? [];
  const summaryAudioAivisModels = audioBriefingAivisModels;
  const summaryAudioXAIVoices = audioBriefingXAIVoices;
  const summaryAudioOpenAITTSVoices = audioBriefingOpenAITTSVoices;
  const summaryAudioGeminiTTSVoices = audioBriefingGeminiTTSVoices;
  const hasUserAivisAPIKey = Boolean(settings?.has_aivis_api_key);
  const hasUserFishAPIKey = Boolean(settings?.has_fish_api_key);
  const hasUserXAIAPIKey = Boolean(settings?.has_xai_api_key);
  const hasUserOpenAIAPIKey = Boolean(settings?.has_openai_api_key);
  const geminiTTSEnabled = Boolean(settings?.gemini_tts_enabled);
  const summaryAudioProviderCapabilities = getAudioBriefingProviderCapabilities(summaryAudioProvider);
  const summaryAudioResolvedVoice = summaryAudioProvider === "aivis"
    ? resolveAivisVoiceSelection(summaryAudioAivisModels, {
        voice_model: summaryAudioVoiceModel,
        voice_style: summaryAudioVoiceStyle,
      })
    : summaryAudioProvider === "fish"
      ? null
      : summaryAudioProvider === "xai"
      ? resolveXAIVoiceSelection(summaryAudioXAIVoices, { voice_model: summaryAudioVoiceModel })
      : summaryAudioProvider === "openai"
        ? resolveOpenAITTSVoiceSelection(summaryAudioOpenAITTSVoices, { voice_model: summaryAudioVoiceModel })
        : summaryAudioProvider === "gemini_tts"
          ? resolveGeminiTTSVoiceSelection(summaryAudioGeminiTTSVoices, { voice_model: summaryAudioVoiceModel })
          : null;
  const summaryAudioResolvedAivisVoice = summaryAudioProvider === "aivis"
    ? (summaryAudioResolvedVoice as ReturnType<typeof resolveAivisVoiceSelection> | null)
    : null;
  const summaryAudioResolvedXAIVoice = summaryAudioProvider === "xai"
    ? (summaryAudioResolvedVoice as XAIVoiceSnapshot | null)
    : null;
  const summaryAudioResolvedFishVoice = summaryAudioProvider === "fish"
    ? (summaryAudioResolvedVoice as FishModelSnapshot | null)
    : null;
  const summaryAudioResolvedOpenAIVoice = summaryAudioProvider === "openai"
    ? (summaryAudioResolvedVoice as OpenAITTSVoiceSnapshot | null)
    : null;
  const summaryAudioResolvedGeminiVoice = summaryAudioProvider === "gemini_tts"
    ? (summaryAudioResolvedVoice as GeminiTTSVoiceCatalogEntry | null)
    : null;
  const audioBriefingVoiceSummaries = audioBriefingVoices.map((voice) => ({
    voice,
    resolved: voice.tts_provider === "aivis"
      ? resolveAivisVoiceSelection(audioBriefingAivisModels, voice)
      : voice.tts_provider === "fish"
        ? null
      : voice.tts_provider === "xai"
        ? resolveXAIVoiceSelection(audioBriefingXAIVoices, voice)
        : voice.tts_provider === "openai"
          ? resolveOpenAITTSVoiceSelection(audioBriefingOpenAITTSVoices, voice)
          : voice.tts_provider === "gemini_tts"
            ? resolveGeminiTTSVoiceSelection(audioBriefingGeminiTTSVoices, voice)
        : null,
    status: getAudioBriefingVoiceStatus(
      voice,
      audioBriefingAivisModels,
      [],
      audioBriefingXAIVoices,
      audioBriefingOpenAITTSVoices,
      audioBriefingGeminiTTSVoices,
      hasUserAivisAPIKey,
      hasUserFishAPIKey,
      hasUserXAIAPIKey,
      hasUserOpenAIAPIKey,
      geminiTTSEnabled,
      t
    ),
  }));
  const configuredAudioBriefingVoiceCount = audioBriefingVoiceSummaries.filter((entry) => entry.status.configured).length;
  const audioBriefingVoiceAttentionCount = audioBriefingVoiceSummaries.filter((entry) => entry.status.tone === "warn").length;
  const audioBriefingVoiceReadyCount = audioBriefingVoiceSummaries.filter((entry) => entry.status.tone === "ok").length;
  const summaryAudioResolvedVoiceLabel =
    summaryAudioProvider === "aivis"
      ? summaryAudioResolvedAivisVoice?.model?.name || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort")
      : summaryAudioProvider === "fish"
        ? summaryAudioResolvedFishVoice?.title || summaryAudioProviderVoiceLabel || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort")
      : summaryAudioProvider === "xai"
        ? summaryAudioResolvedXAIVoice?.name || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort")
        : summaryAudioProvider === "openai"
          ? summaryAudioResolvedOpenAIVoice?.name || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort")
          : summaryAudioProvider === "gemini_tts"
            ? summaryAudioResolvedGeminiVoice?.label || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort")
            : summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort");
  const summaryAudioResolvedVoiceDetail =
    summaryAudioProvider === "aivis"
      ? summaryAudioResolvedAivisVoice?.speaker && summaryAudioResolvedAivisVoice?.style
        ? `${summaryAudioResolvedAivisVoice.speaker.name} / ${summaryAudioResolvedAivisVoice.style.name}`
        : formatAivisVoiceStyleLabel(summaryAudioVoiceStyle || summaryAudioVoiceModel, t)
      : summaryAudioProvider === "fish"
        ? summaryAudioResolvedFishVoice?.description || summaryAudioProviderVoiceDescription || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort")
      : summaryAudioProvider === "xai"
        ? summaryAudioResolvedXAIVoice?.description || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort")
        : summaryAudioProvider === "openai"
          ? summaryAudioResolvedOpenAIVoice?.description || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort")
          : summaryAudioProvider === "gemini_tts"
            ? summaryAudioResolvedGeminiVoice?.description || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort")
            : summaryAudioVoiceStyle || summaryAudioVoiceModel || t("settings.summaryAudio.unsetShort");
  const audioBriefingUsesAivisCloud = audioBriefingVoices.some((voice) => voice.tts_provider === "aivis");
  const audioBriefingNeedsAivisAPIKey = audioBriefingUsesAivisCloud && !hasUserAivisAPIKey;
  const audioBriefingUsesFish = audioBriefingVoices.some((voice) => voice.tts_provider === "fish");
  const audioBriefingNeedsFishAPIKey = audioBriefingUsesFish && !hasUserFishAPIKey;
  const audioBriefingUsesXAI = audioBriefingVoices.some((voice) => voice.tts_provider === "xai");
  const audioBriefingNeedsXAIAPIKey = audioBriefingUsesXAI && !hasUserXAIAPIKey;
  const audioBriefingUsesOpenAI = audioBriefingVoices.some((voice) => voice.tts_provider === "openai");
  const audioBriefingNeedsOpenAIAPIKey = audioBriefingUsesOpenAI && !hasUserOpenAIAPIKey;
  const audioBriefingUsesGeminiTTS = audioBriefingVoices.some((voice) => voice.tts_provider === "gemini_tts");
  const audioBriefingNeedsGeminiAccess = audioBriefingUsesGeminiTTS && !geminiTTSEnabled;
  const geminiDuoModelCounts = audioBriefingVoices.reduce((acc, voice) => {
    if (voice.tts_provider !== "gemini_tts") return acc;
    const model = voice.tts_model.trim();
    const selectedVoice = voice.voice_model.trim();
    if (!model || !selectedVoice) return acc;
    acc.set(model, (acc.get(model) ?? 0) + 1);
    return acc;
  }, new Map<string, number>());
  const geminiDuoBestModelEntry = Array.from(geminiDuoModelCounts.entries()).sort((a, b) => b[1] - a[1])[0] ?? null;
  const geminiDuoCompatiblePersonaCount = geminiDuoBestModelEntry?.[1] ?? 0;
  const geminiDuoCompatibleModel = geminiDuoBestModelEntry?.[0] ?? "";
  const geminiDuoReady = geminiTTSEnabled && geminiDuoCompatiblePersonaCount >= 2;
  const fishDuoCompatibleVoices = audioBriefingVoices.filter((voice) =>
    voice.tts_provider === "fish" && voice.tts_model === "s2-pro" && voice.voice_model.trim().length > 0
  );
  const fishDuoDistinctVoiceCount = new Set(fishDuoCompatibleVoices.map((voice) => voice.voice_model.trim())).size;
  const fishDuoReady = hasUserFishAPIKey && fishDuoDistinctVoiceCount >= 2;

  const sectionNavItems: Array<{
    id: SettingsSectionID;
    title: string;
    summary: string;
  }> = [
    {
      id: "models",
      title: t("settings.section.llm"),
      summary: `${configuredProviderCount}/${accessCards.length} ${t("settings.access.configuredProviders")}`,
    },
    {
      id: "reading-plan",
      title: t("settings.recommendedTitle"),
      summary: `${t(`settings.window.${readingPlanWindow}`)} / ${readingPlanSize} / ${readingPlanDiversifyTopics ? t("settings.on") : t("settings.off")}`,
    },
    {
      id: "navigator",
      title: t("settings.group.navigator"),
      summary: navigatorEnabled
        ? `${navigatorPersonaMode === "random" ? t("settings.personaMode.random") : t(`settings.navigator.persona.${navigatorPersona}`, navigatorPersona)} / ${navigatorModel || t("settings.default")}`
        : t("settings.off"),
    },
    {
      id: "audio-briefing",
      title: t("settings.section.audioBriefing"),
      summary: audioBriefingEnabled
        ? `${formatAudioBriefingScheduleSelection(audioBriefingScheduleSelection, t)} / ${audioBriefingArticlesPerEpisode}${t("settings.audioBriefing.articlesSuffix")}`
        : t("settings.off"),
    },
    {
      id: "summary-audio",
      title: t("settings.section.summaryAudio"),
      summary: summaryAudioProvider
        ? `${t(`settings.summaryAudio.provider.${summaryAudioProvider}`, summaryAudioProvider)} / ${summaryAudioVoiceModel || t("settings.summaryAudio.unconfiguredShort")}`
        : t("settings.off"),
    },
    {
      id: "personalization",
      title: t("settings.personalization.title"),
      summary: preferenceProfile
        ? `${t(`settings.personalization.status.${preferenceProfile.status}`, preferenceProfile.status)} / ${Math.round(preferenceProfile.confidence * 100)}%`
        : preferenceProfileError
          ? t("settings.personalization.loadFailedShort")
          : t("settings.personalization.unavailable"),
    },
    {
      id: "digest",
      title: t("settings.digestTitle"),
      summary: digestEmailEnabled ? t("settings.controlRoom.digestEnabled") : t("settings.controlRoom.digestDisabled"),
    },
    {
      id: "notifications",
      title: t("settings.section.notifications"),
      summary: `${notificationPriority.briefing_enabled ? t("settings.pushTypeBriefing") : t("settings.controlRoom.briefingOff")} / cap ${notificationPriority.daily_cap}`,
    },
    {
      id: "integrations",
      title: t("settings.section.integrations"),
      summary: `${settings.has_inoreader_oauth ? t("settings.inoreaderConnected") : t("settings.inoreaderNotConnected")} / ${settings.obsidian_export?.github_installation_id ? t("settings.obsidianGithubConnected") : t("settings.obsidianGithubNotConnected")}`,
    },
    {
      id: "budget",
      title: t("settings.budgetTitle"),
      summary: settings.monthly_budget_usd == null
        ? t("settings.controlRoom.budgetUnset")
        : `$${settings.monthly_budget_usd.toFixed(2)} / ${settings.current_month.remaining_budget_pct == null ? "—" : `${settings.current_month.remaining_budget_pct.toFixed(1)}%`}`,
    },
    {
      id: "system",
      title: t("settings.section.system"),
      summary: `${configuredProviderCount}/${accessCards.length} ${t("settings.configured")}`,
    },
  ];

  const railNotes = [
    {
      title: t("settings.controlRoom.providerUpdatesTitle"),
      body: visibleProviderModelUpdates.length > 0
        ? t("settings.controlRoom.providerUpdatesBody").replace("{{count}}", String(visibleProviderModelUpdates.length))
        : t("settings.controlRoom.providerUpdatesEmpty"),
    },
    {
      title: t("settings.controlRoom.notificationHealthTitle"),
      body: `${notificationPriority.briefing_enabled ? t("settings.pushTypeBriefing") : t("settings.controlRoom.briefingOff")} / ${notificationPriority.immediate_enabled ? t("settings.pushTypeImmediate") : t("settings.controlRoom.immediateOff")} / cap ${notificationPriority.daily_cap}`,
    },
    {
      title: t("settings.controlRoom.budgetStatusTitle"),
      body:
        settings.current_month.remaining_budget_pct == null
          ? t("settings.controlRoom.budgetUnset")
          : t("settings.controlRoom.budgetStatusBody")
              .replace("{{month}}", settings.current_month.month_jst)
              .replace("{{remaining}}", `${settings.current_month.remaining_budget_pct.toFixed(1)}%`),
    },
  ];

  const selectedSectionMeta = {
    "audio-briefing": {
      kicker: t("settings.section.audioBriefing"),
      title: t("settings.controlRoom.audioBriefingTitle"),
      description: t("settings.controlRoom.audioBriefingDescription"),
    },
    "summary-audio": {
      kicker: t("settings.section.summaryAudio"),
      title: t("settings.summaryAudio.title"),
      description: t("settings.summaryAudio.description"),
    },
    "reading-plan": {
      kicker: t("settings.recommendedTitle"),
      title: t("settings.controlRoom.readingPlanTitle"),
      description: t("settings.controlRoom.readingPlanDescription"),
    },
    personalization: {
      kicker: t("settings.personalization.title"),
      title: t("settings.personalization.title"),
      description: t("settings.personalization.description.default"),
    },
    digest: {
      kicker: t("settings.digestTitle"),
      title: t("settings.controlRoom.digestTitle"),
      description: t("settings.controlRoom.digestDescription"),
    },
    notifications: {
      kicker: t("settings.section.notifications"),
      title: t("settings.controlRoom.notificationsTitle"),
      description: t("settings.controlRoom.notificationsDescription"),
    },
    integrations: {
      kicker: t("settings.section.integrations"),
      title: t("settings.controlRoom.integrationsTitle"),
      description: t("settings.controlRoom.integrationsDescription"),
    },
    navigator: {
      kicker: t("settings.group.navigator"),
      title: t("settings.controlRoom.navigatorTitle"),
      description: t("settings.controlRoom.navigatorDescription"),
    },
    models: {
      kicker: t("settings.section.llm"),
      title: t("settings.controlRoom.modelsTitle"),
      description: t("settings.controlRoom.modelsDescription"),
    },
    budget: {
      kicker: t("settings.budgetTitle"),
      title: t("settings.controlRoom.budgetTitle"),
      description: t("settings.controlRoom.budgetDescription"),
    },
    system: {
      kicker: t("settings.section.system"),
      title: t("settings.controlRoom.systemTitle"),
      description: t("settings.controlRoom.systemDescription"),
    },
  }[activeSection];

  return (
    <div className="mx-auto max-w-[1360px] space-y-6">
      <PageHeader
        eyebrow={t("settings.controlRoomEyebrow")}
        title={t("nav.settings")}
        titleIcon={SettingsIcon}
        description={t("settings.controlRoomSubtitle")}
      />

      <div className="grid gap-6 lg:grid-cols-[248px_minmax(0,1fr)] xl:grid-cols-[268px_minmax(0,1fr)]">
        <aside className="space-y-4 lg:sticky lg:top-24 lg:self-start">
          <SectionCard className="p-0">
            <div className="px-5 pt-5 text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.controlRoomSections")}
            </div>
            <div className="mt-3">
              {sectionNavItems.map((item, index) => {
                const active = item.id === activeSection;
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setActiveSection(item.id)}
                    className={joinClassNames(
                      "relative block w-full border-t border-[var(--color-editorial-line)] px-4 py-3 text-left transition-colors first:border-t-0",
                      active
                        ? "bg-[linear-gradient(90deg,rgba(243,236,227,0.92),rgba(243,236,227,0.28)_78%,transparent)]"
                        : "hover:bg-[var(--color-editorial-panel-strong)]"
                    )}
                  >
                    {active ? (
                      <span
                        aria-hidden="true"
                        className={joinClassNames(
                          "absolute left-0 w-[3px] rounded-full bg-[var(--color-editorial-ink)]",
                          index === 0 ? "top-0 bottom-3" : "bottom-3 top-3"
                        )}
                      />
                    ) : null}
                    <div className="text-[13px] font-semibold text-[var(--color-editorial-ink)]">{item.title}</div>
                    <div className="mt-1 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{item.summary}</div>
                  </button>
                );
              })}
            </div>
          </SectionCard>

          <SectionCard className="p-0">
            <div className="px-5 pt-5 text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.controlRoomStatusNotes")}
            </div>
            <div className="mt-3">
              {railNotes.map((note, index) => (
                <div
                  key={note.title}
                  className={joinClassNames(
                    "border-t border-[var(--color-editorial-line)] px-4 py-3 first:border-t-0",
                    index === 0 ? "" : ""
                  )}
                >
                  <div className="text-[13px] font-semibold text-[var(--color-editorial-ink)]">{note.title}</div>
                  <div className="mt-1 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{note.body}</div>
                </div>
              ))}
            </div>
          </SectionCard>
        </aside>

        <div className="space-y-5">
          <SectionCard>
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                  {selectedSectionMeta.kicker}
                </div>
                <h2 className="mt-2 font-serif text-[1.85rem] leading-[1.1] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                  {selectedSectionMeta.title}
                </h2>
                <p className="mt-2 max-w-3xl text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                  {selectedSectionMeta.description}
                </p>
              </div>
              {activeSection === "models" ? (
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={applyCostPerformancePreset}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 press focus-ring"
                  >
                    {t("settings.modelPreset.costPerformance")}
                  </button>
                  <button
                    type="button"
                    onClick={toggleLLMExtras}
                    className="inline-flex min-h-10 items-center gap-1 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring"
                  >
                    {t("settings.section.llmExtras")}
                    <ChevronDown className={`size-3 transition-transform ${llmExtrasOpen ? "rotate-180" : ""}`} />
                  </button>
                </div>
              ) : null}
            </div>
          </SectionCard>

          {activeSection === "audio-briefing" ? (
            <>
              <SectionCard>
                <form onSubmit={submitAudioBriefingSettings} className="space-y-5">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div>
                      <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.summaryTitle")}</div>
                      <p className="mt-1 max-w-3xl text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.audioBriefing.summaryHelp")}</p>
                    </div>
                    <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
                      <button
                        type="button"
                        onClick={() => {
                          void openAudioBriefingPresetApplyModal();
                        }}
                        disabled={audioBriefingPresetsLoading && audioBriefingPresets.length === 0}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                      >
                        {audioBriefingPresetsLoading && audioBriefingPresets.length === 0 ? t("common.loading") : t("settings.audioBriefing.applyPreset")}
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          void openAudioBriefingPresetSaveModal();
                        }}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {t("settings.audioBriefing.savePreset")}
                      </button>
                      <button
                        type="submit"
                        disabled={savingAudioBriefing}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {savingAudioBriefing ? t("common.saving") : t("settings.audioBriefing.saveSettings")}
                      </button>
                      <Link
                        href="/audio-briefings"
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                      >
                        {t("settings.audioBriefing.openEpisodes")}
                      </Link>
                    </div>
                  </div>

                  <div className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                    <div className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.modeGuideTitle", "Single / Duo guide")}</div>
                    <p className="mt-2">{t("settings.audioBriefing.modeGuideBody", "Single keeps the current one-person narration path. Duo adds a host-and-partner conversation, which increases turns, processing time, and TTS cost, but makes the listening experience more conversational.")}</p>
                    <p className="mt-2">
                      {audioBriefingConversationMode === "duo"
                        ? t("settings.audioBriefing.modeGuideDuoActive", "Duo is currently selected. If persona mode is random, the host follows the same random selection as single mode and the partner is chosen from a different persona.")
                        : t("settings.audioBriefing.modeGuideSingleActive", "Single is currently selected. This is the existing stable path, and you can switch back to it at any time if duo quality is not where you want it yet.")}
                    </p>
                  </div>

                  <div className="flex flex-wrap items-stretch gap-3">
                    <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.enableTitle")}
                      </div>
                      <div className="mt-3 flex items-center justify-between gap-3">
                        <div className="text-sm font-medium text-[var(--color-editorial-ink)] whitespace-nowrap">
                          {audioBriefingEnabled ? t("settings.on") : t("settings.off")}
                        </div>
                        <input
                          type="checkbox"
                          checked={audioBriefingEnabled}
                          onChange={(e) => setAudioBriefingEnabled(e.target.checked)}
                          className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                        />
                      </div>
                    </label>

                    <label className="flex min-w-[260px] flex-[1.5] flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.programName")}
                      </div>
                      <input
                        type="text"
                        value={audioBriefingProgramName}
                        onChange={(e) => setAudioBriefingProgramName(e.target.value)}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                      <p className="mt-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.programNameHelp")}
                      </p>
                    </label>

                    <label className="flex min-w-[240px] flex-[1.15] flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.schedule")}
                      </div>
                      <select
                        value={audioBriefingScheduleSelection}
                        onChange={(e) => setAudioBriefingScheduleSelection(e.target.value as AudioBriefingScheduleSelection)}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      >
                        <option value="interval3h">{t("settings.audioBriefing.interval3h")}</option>
                        <option value="interval6h">{t("settings.audioBriefing.interval6h")}</option>
                        <option value="fixed3x">{t("settings.audioBriefing.fixed3x")}</option>
                      </select>
                      <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.scheduleHelp")}
                      </p>
                    </label>

                    <label className="flex min-w-[180px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.articlesPerEpisode")}
                      </div>
                      <input
                        value={audioBriefingArticlesPerEpisode}
                        onChange={(e) => setAudioBriefingArticlesPerEpisode(e.target.value)}
                        inputMode="numeric"
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                    </label>

                    <label className="flex min-w-[180px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.targetDuration")}
                      </div>
                      <input
                        value={audioBriefingTargetDurationMinutes}
                        onChange={(e) => setAudioBriefingTargetDurationMinutes(e.target.value)}
                        inputMode="numeric"
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                    </label>

                    <label className="flex min-w-[180px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.chunkTrailingSilenceSeconds")}
                      </div>
                      <input
                        value={audioBriefingChunkTrailingSilenceSeconds}
                        onChange={(e) => setAudioBriefingChunkTrailingSilenceSeconds(e.target.value)}
                        inputMode="decimal"
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                      <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.chunkTrailingSilenceSecondsHelp")}
                      </p>
                    </label>

                    <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.conversationMode")}
                      </div>
                      <select
                        value={audioBriefingConversationMode}
                        onChange={(e) => setAudioBriefingConversationMode(e.target.value === "duo" ? "duo" : "single")}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      >
                        <option value="single">{t("settings.audioBriefing.conversationMode.single")}</option>
                        <option value="duo">{t("settings.audioBriefing.conversationMode.duo")}</option>
                      </select>
                    </label>

                    <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.personaMode.label")}
                      </div>
                      <select
                        value={audioBriefingDefaultPersonaMode}
                        onChange={(e) => setAudioBriefingDefaultPersonaMode(e.target.value === "random" ? "random" : "fixed")}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      >
                        <option value="fixed">{t("settings.personaMode.fixed")}</option>
                        <option value="random">{t("settings.personaMode.random")}</option>
                      </select>
                    </label>

                    <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.defaultPersona")}
                      </div>
                      <select
                        value={audioBriefingDefaultPersona}
                        onChange={(e) => setAudioBriefingDefaultPersona(e.target.value)}
                        disabled={audioBriefingDefaultPersonaMode === "random"}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      >
                        {navigatorPersonaCards.map((persona) => (
                          <option key={persona.key} value={persona.key}>
                            {t(`settings.navigator.persona.${persona.key}`, persona.key)}
                          </option>
                        ))}
                      </select>
                      <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                        {audioBriefingDefaultPersonaMode === "random"
                          ? t("settings.audioBriefing.randomPersonaHelp")
                          : t("settings.audioBriefing.defaultPersonaHelp")}
                      </p>
                    </label>

                    <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.bgmTitle")}
                      </div>
                      <div className="mt-3 flex items-center justify-between gap-3">
                        <div className="whitespace-nowrap text-sm font-medium text-[var(--color-editorial-ink)]">
                          {audioBriefingBGMEnabled ? t("settings.on") : t("settings.off")}
                        </div>
                        <input
                          type="checkbox"
                          checked={audioBriefingBGMEnabled}
                          onChange={(e) => setAudioBriefingBGMEnabled(e.target.checked)}
                          className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                        />
                      </div>
                      <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.bgmHelp")}
                      </p>
                    </label>

                    <label className="flex min-w-[260px] flex-[1.4] flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.bgmPrefix")}
                      </div>
                      <input
                        value={audioBriefingBGMR2Prefix}
                        onChange={(e) => setAudioBriefingBGMR2Prefix(e.target.value)}
                        placeholder="audio-briefings/bgm/"
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                      <p className="mt-2 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.bgmPrefixHelp")}
                      </p>
                    </label>
                  </div>

                  {audioBriefingConversationMode === "duo" ? (
                    <div className="grid gap-3 lg:grid-cols-2">
                      <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] px-4 py-4">
                        <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                          {t("settings.audioBriefing.duoHostRuleTitle", "Host selection")}
                        </div>
                        <p className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                          {audioBriefingDefaultPersonaMode === "random"
                            ? t("settings.audioBriefing.duoHostRuleRandom", "Because persona mode is random, the host also follows the same random selection used by single mode.")
                            : t("settings.audioBriefing.duoHostRuleFixed", "Because persona mode is fixed, the selected default persona will always act as the host.")}
                        </p>
                      </div>
                      <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] px-4 py-4">
                        <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                          {t("settings.audioBriefing.duoPartnerRuleTitle", "Partner selection")}
                        </div>
                        <p className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                          {t("settings.audioBriefing.duoPartnerRuleBody", "The partner is picked from a different persona than the host. Make sure multiple persona voices are configured if you plan to use duo regularly.")}
                        </p>
                      </div>
                      <div
                        className={`rounded-[18px] border px-4 py-4 ${
                          geminiDuoReady
                            ? "border-[rgba(34,197,94,0.24)] bg-[rgba(240,253,244,0.82)]"
                            : "border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)]"
                        }`}
                      >
                        <div
                          className={`text-[11px] font-semibold uppercase tracking-[0.14em] ${
                            geminiDuoReady ? "text-[#166534]" : "text-[#b45309]"
                          }`}
                        >
                          {geminiDuoReady
                            ? t("settings.audioBriefing.geminiDuoReadyTitle")
                            : t("settings.audioBriefing.geminiDuoNeedsSetupTitle")}
                        </div>
                        <p className={`mt-2 text-sm leading-6 ${geminiDuoReady ? "text-[#166534]" : "text-[#b45309]"}`}>
                          {geminiDuoReady
                            ? tWithVars(t, "settings.audioBriefing.geminiDuoReadyDetail", {
                                count: geminiDuoCompatiblePersonaCount,
                                model: geminiDuoCompatibleModel,
                              })
                            : t("settings.audioBriefing.geminiDuoNeedsSetupDetail")}
                        </p>
                      </div>
                      <div
                        className={`rounded-[18px] border px-4 py-4 ${
                          fishDuoReady
                            ? "border-[rgba(34,197,94,0.24)] bg-[rgba(240,253,244,0.82)]"
                            : "border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)]"
                        }`}
                      >
                        <div
                          className={`text-[11px] font-semibold uppercase tracking-[0.14em] ${
                            fishDuoReady ? "text-[#166534]" : "text-[#b45309]"
                          }`}
                        >
                          {fishDuoReady
                            ? t("settings.audioBriefing.fishDuoReadyTitle")
                            : t("settings.audioBriefing.fishDuoNeedsSetupTitle")}
                        </div>
                        <p className={`mt-2 text-sm leading-6 ${fishDuoReady ? "text-[#166534]" : "text-[#b45309]"}`}>
                          {fishDuoReady
                            ? tWithVars(t, "settings.audioBriefing.fishDuoReadyDetail", {
                                count: fishDuoDistinctVoiceCount,
                                model: "s2-pro",
                              })
                            : t("settings.audioBriefing.fishDuoNeedsSetupDetail")}
                        </p>
                      </div>
                    </div>
                  ) : null}
                </form>
              </SectionCard>

              <SectionCard>
                <form onSubmit={submitAudioBriefingModels} className="space-y-4">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div>
                      <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.model.audioBriefingScript")}</div>
                      <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">
                        {t("settings.audioBriefing.scriptModelHelp")}
                      </p>
                    </div>
                    <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
                      <button
                        type="submit"
                        disabled={savingLLMModels}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                      >
                        {savingLLMModels ? t("common.saving") : t("settings.saveModels")}
                      </button>
                    </div>
                  </div>
                  <div className="grid gap-4 md:grid-cols-2">
                    <ModelSelect
                      label={t("settings.model.audioBriefingScript")}
                      value={audioBriefingScriptModel}
                      onChange={(value) => onChangeLLMModel(setAudioBriefingScriptModel, value)}
                      options={optionsForPurpose("summary", audioBriefingScriptModel)}
                      labels={modelSelectLabels}
                      variant="modal"
                    />
                    <ModelSelect
                      label={t("settings.model.audioBriefingScriptFallback")}
                      value={audioBriefingScriptFallbackModel}
                      onChange={(value) => onChangeLLMModel(setAudioBriefingScriptFallbackModel, value)}
                      options={optionsForPurpose("summary", audioBriefingScriptFallbackModel)}
                      labels={modelSelectLabels}
                      variant="modal"
                    />
                  </div>
                </form>
              </SectionCard>

              <SectionCard>
                <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                  <div>
                    <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.aivisDictionaryTitle")}</div>
                    <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.aivisDictionaryDescription")}</p>
                  </div>
                  <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
                    <button
                      type="button"
                      onClick={() => {
                        void loadAivisUserDictionaries(true).catch(() => undefined);
                      }}
                      disabled={!settings?.has_aivis_api_key || aivisUserDictionariesLoading}
                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                    >
                      {aivisUserDictionariesLoading ? t("common.loading") : t("common.refresh")}
                    </button>
                  </div>
                </div>

                {!settings?.has_aivis_api_key ? (
                  <div className="mt-4 flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
                    <div>
                      <div className="font-semibold">{t("settings.audioBriefing.aivisApiKeyWarningTitle")}</div>
                      <div className="mt-1 leading-6">{t("settings.aivisDictionaryRequiresApiKey")}</div>
                    </div>
                    <button
                      type="button"
                      onClick={() => {
                        setActiveSection("system");
                        setActiveAccessProvider("aivis");
                      }}
                      className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
                    >
                      {t("settings.audioBriefing.openApiKeys")}
                    </button>
                  </div>
                ) : (
                  <div className="mt-4 space-y-4">
                    <div className="space-y-2">
                      <label className="text-xs font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.aivisDictionarySelectLabel")}
                      </label>
                      <select
                        value={aivisUserDictionaryUUID}
                        onChange={(e) => setAivisUserDictionaryUUID(e.target.value)}
                        disabled={aivisUserDictionariesLoading || aivisUserDictionaries.length === 0}
                        className="w-full rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] disabled:opacity-60"
                      >
                        <option value="">{t("settings.aivisDictionaryUnset")}</option>
                        {aivisUserDictionaries.map((item) => (
                          <option key={item.uuid} value={item.uuid}>
                            {`${item.name} (${item.word_count})`}
                          </option>
                        ))}
                      </select>
                      {aivisUserDictionariesError ? (
                        <p className="text-xs text-[var(--color-editorial-danger)]">{aivisUserDictionariesError}</p>
                      ) : null}
                      {!aivisUserDictionariesLoading && aivisUserDictionaries.length === 0 ? (
                        <p className="text-xs text-[var(--color-editorial-ink-faint)]">{t("settings.aivisDictionaryEmpty")}</p>
                      ) : null}
                      {aivisUserDictionaryUUID ? (
                        <p className="text-xs text-[var(--color-editorial-ink-faint)]">
                          {aivisUserDictionaries.find((item) => item.uuid === aivisUserDictionaryUUID)?.description || t("settings.aivisDictionarySelected")}
                        </p>
                      ) : null}
                    </div>
                    <div className="flex flex-wrap items-center gap-3">
                      <button
                        type="button"
                        onClick={() => {
                          void saveAivisUserDictionary();
                        }}
                        disabled={!aivisUserDictionaryUUID || savingAivisDictionary}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                      >
                        {savingAivisDictionary ? t("common.saving") : t("common.save")}
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          void clearAivisUserDictionary();
                        }}
                        disabled={!settings?.aivis_user_dictionary_uuid || deletingAivisDictionary}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] disabled:opacity-60"
                      >
                        {deletingAivisDictionary ? t("common.loading") : t("settings.delete")}
                      </button>
                    </div>
                  </div>
                )}
              </SectionCard>

              <SectionCard>
                <form onSubmit={submitAudioBriefingVoices} className="space-y-4">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div>
                      <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.audioBriefing.voiceMatrixTitle")}</div>
                      <div className="mt-1 flex flex-wrap items-center gap-3 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">
                        <p>{t("settings.audioBriefing.voiceMatrixHelp")}</p>
                        <Link href="/aivis-models" className="font-medium text-[var(--color-editorial-accent)] underline-offset-4 hover:underline">
                          {t("settings.audioBriefing.openAivisModels")}
                        </Link>
                        <Link href="/openai-tts-voices" className="font-medium text-[var(--color-editorial-accent)] underline-offset-4 hover:underline">
                          {t("settings.audioBriefing.openOpenAITTSVoices")}
                        </Link>
                        <Link href="/gemini-tts-voices" className="font-medium text-[var(--color-editorial-accent)] underline-offset-4 hover:underline">
                          {t("settings.audioBriefing.openGeminiTTSVoices")}
                        </Link>
                        {aivisModelsData?.latest_run?.finished_at ? (
                          <span>{`${t("aivisModels.lastSynced")}: ${new Date(aivisModelsData.latest_run.finished_at).toLocaleString()}`}</span>
                        ) : null}
                        {openAITTSVoicesData?.latest_run?.finished_at ? (
                          <span>{`${t("openaiTTS.lastSynced")}: ${new Date(openAITTSVoicesData.latest_run.finished_at).toLocaleString()}`}</span>
                        ) : null}
                      </div>
                    </div>
                    <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
                      <button
                        type="button"
                        onClick={() => {
                          void syncAivisModels();
                        }}
                        disabled={aivisModelsSyncing}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                      >
                        {aivisModelsSyncing ? t("aivisModels.syncing") : t("aivisModels.sync")}
                      </button>
                      <button
                        type="submit"
                        disabled={savingAudioBriefingVoices}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {savingAudioBriefingVoices ? t("common.saving") : t("settings.audioBriefing.saveVoices")}
                      </button>
                    </div>
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <div className="min-w-[180px] flex-1 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.summary.ready")}
                      </div>
                      <div className="mt-2 text-2xl font-semibold text-[var(--color-editorial-ink)]">{audioBriefingVoiceReadyCount}</div>
                    </div>
                    <div className="min-w-[180px] flex-1 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.summary.needsAttention")}
                      </div>
                      <div className="mt-2 text-2xl font-semibold text-[var(--color-editorial-ink)]">{audioBriefingVoiceAttentionCount}</div>
                    </div>
                    <div className="min-w-[180px] flex-1 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.audioBriefing.summary.configured")}
                      </div>
                      <div className="mt-2 text-2xl font-semibold text-[var(--color-editorial-ink)]">{configuredAudioBriefingVoiceCount}/{audioBriefingVoiceSummaries.length}</div>
                    </div>
                  </div>

                  {aivisModelsError ? (
                    <div className="rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-3 text-sm text-[#b45309]">
                      {aivisModelsError}
                    </div>
                  ) : null}

                  {xaiVoicesError ? (
                    <div className="rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-3 text-sm text-[#b45309]">
                      {xaiVoicesError}
                    </div>
                  ) : null}

                  {geminiTTSVoicesError ? (
                    <div className="rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-3 text-sm text-[#b45309]">
                      {geminiTTSVoicesError}
                    </div>
                  ) : null}

                  {audioBriefingNeedsAivisAPIKey ? (
                    <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
                      <div>
                        <div className="font-semibold">{t("settings.audioBriefing.aivisApiKeyWarningTitle")}</div>
                        <div className="mt-1 leading-6">{t("settings.audioBriefing.aivisApiKeyWarningDetail")}</div>
                      </div>
                      <button
                        type="button"
                        onClick={() => setActiveSection("system")}
                        className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
                      >
                        {t("settings.audioBriefing.openApiKeys")}
                      </button>
                    </div>
                  ) : null}

                  {audioBriefingNeedsXAIAPIKey ? (
                    <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
                      <div>
                        <div className="font-semibold">{t("settings.audioBriefing.xaiApiKeyWarningTitle")}</div>
                        <div className="mt-1 leading-6">{t("settings.audioBriefing.xaiApiKeyWarningDetail")}</div>
                      </div>
                      <button
                        type="button"
                        onClick={() => setActiveSection("system")}
                        className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
                      >
                        {t("settings.audioBriefing.openApiKeys")}
                      </button>
                    </div>
                  ) : null}

                  {audioBriefingNeedsFishAPIKey ? (
                    <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
                      <div>
                        <div className="font-semibold">{t("settings.audioBriefing.fishApiKeyWarningTitle")}</div>
                        <div className="mt-1 leading-6">{t("settings.audioBriefing.fishApiKeyWarningDetail")}</div>
                      </div>
                      <button
                        type="button"
                        onClick={() => setActiveSection("system")}
                        className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
                      >
                        {t("settings.audioBriefing.openApiKeys")}
                      </button>
                    </div>
                  ) : null}

                  {audioBriefingNeedsOpenAIAPIKey ? (
                    <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
                      <div>
                        <div className="font-semibold">{t("settings.audioBriefing.openAIApiKeyWarningTitle")}</div>
                        <div className="mt-1 leading-6">{t("settings.audioBriefing.openAIApiKeyWarningDetail")}</div>
                      </div>
                      <button
                        type="button"
                        onClick={() => setActiveSection("system")}
                        className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
                      >
                        {t("settings.audioBriefing.openApiKeys")}
                      </button>
                    </div>
                  ) : null}

                  {audioBriefingNeedsGeminiAccess ? (
                    <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
                      <div>
                        <div className="font-semibold">{t("settings.audioBriefing.geminiAccessWarningTitle")}</div>
                        <div className="mt-1 leading-6">{t("settings.audioBriefing.geminiAccessWarningDetail")}</div>
                      </div>
                    </div>
                  ) : null}

                  <div className="space-y-3">
                    {audioBriefingVoiceSummaries.map(({ voice, status }) => {
                      const expanded = expandedAudioBriefingPersonas.includes(voice.persona);
                      const isDefaultPersona = voice.persona === audioBriefingDefaultPersona;
                      const providerCapabilities = getAudioBriefingProviderCapabilities(voice.tts_provider);
                      const isAivisProvider = voice.tts_provider === "aivis";
                      const isFishProvider = voice.tts_provider === "fish";
                      const isXAIProvider = voice.tts_provider === "xai";
                      const isOpenAIProvider = voice.tts_provider === "openai";
                      const isGeminiProvider = voice.tts_provider === "gemini_tts";
                      const aivisResolved = isAivisProvider ? resolveAivisVoiceSelection(audioBriefingAivisModels, voice) : null;
                      const xaiResolved = isXAIProvider ? resolveXAIVoiceSelection(audioBriefingXAIVoices, voice) : null;
                      const openAIResolved = isOpenAIProvider ? resolveOpenAITTSVoiceSelection(audioBriefingOpenAITTSVoices, voice) : null;
                      const geminiResolved = isGeminiProvider ? resolveGeminiTTSVoiceSelection(audioBriefingGeminiTTSVoices, voice) : null;
                      const selectedVoiceLabel = isAivisProvider
                        ? aivisResolved?.model?.name || voice.voice_model || t("settings.audioBriefing.unsetShort")
                        : isFishProvider
                          ? voice.provider_voice_label || voice.voice_model || t("settings.audioBriefing.unsetShort")
                        : isXAIProvider
                          ? xaiResolved?.name || voice.voice_model || t("settings.audioBriefing.unsetShort")
                          : isOpenAIProvider
                            ? openAIResolved?.name || voice.voice_model || t("settings.audioBriefing.unsetShort")
                            : isGeminiProvider
                              ? geminiResolved?.label || voice.voice_model || t("settings.audioBriefing.unsetShort")
                            : voice.voice_model || t("settings.audioBriefing.unsetShort");
                      const selectedVoiceDetail = isAivisProvider
                        ? (aivisResolved?.speaker && aivisResolved?.style
                            ? `${aivisResolved.speaker.name} / ${aivisResolved.style.name}`
                            : formatAivisVoiceStyleLabel(voice.voice_style || voice.voice_model, t))
                        : isFishProvider
                          ? voice.provider_voice_description || voice.voice_model || t("settings.audioBriefing.unsetShort")
                        : isXAIProvider
                          ? xaiResolved?.description || voice.voice_model || t("settings.audioBriefing.unsetShort")
                          : isOpenAIProvider
                            ? openAIResolved?.description || voice.voice_model || t("settings.audioBriefing.unsetShort")
                            : isGeminiProvider
                              ? geminiResolved?.description || voice.voice_model || t("settings.audioBriefing.unsetShort")
                            : voice.voice_style || voice.voice_model || t("settings.audioBriefing.unsetShort");
                      const toneClasses = status.tone === "ok"
                        ? "border-[rgba(34,197,94,0.28)] bg-[rgba(240,253,244,0.72)]"
                        : status.tone === "warn"
                          ? "border-[rgba(245,158,11,0.35)] bg-[rgba(255,251,235,0.82)]"
                          : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]";
                      const badgeClasses = status.tone === "ok"
                        ? "border-[rgba(34,197,94,0.24)] bg-[rgba(220,252,231,0.85)] text-[#166534]"
                        : status.tone === "warn"
                          ? "border-[rgba(245,158,11,0.24)] bg-[rgba(254,243,199,0.88)] text-[#b45309]"
                          : "border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)]";

                      return (
                        <div key={voice.persona} className={`overflow-hidden rounded-[20px] border ${toneClasses}`}>
                          <button
                            type="button"
                            onClick={() => toggleAudioBriefingPersona(voice.persona)}
                            className="flex w-full flex-wrap items-center gap-3 px-4 py-4 text-left"
                            aria-expanded={expanded}
                          >
                            <div className="flex min-w-[220px] flex-1 items-center gap-3">
                              <div className="rounded-full border border-[var(--color-editorial-line)] bg-white p-1.5">
                                <AINavigatorAvatar persona={voice.persona} className="size-10" />
                              </div>
                              <div className="min-w-0">
                                <div className="flex flex-wrap items-center gap-2">
                                  <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">
                                    {t(`settings.navigator.persona.${voice.persona}`, voice.persona)}
                                  </div>
                                  {isDefaultPersona ? (
                                    <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-soft)]">
                                      {t("settings.audioBriefing.defaultPersonaBadge")}
                                    </span>
                                  ) : null}
                                </div>
                                <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">{voice.persona}</div>
                              </div>
                            </div>

                            <div className="flex min-w-[180px] flex-1 flex-wrap items-center gap-2 text-[12px] text-[var(--color-editorial-ink-soft)]">
                              <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2.5 py-1">
                                {voice.tts_provider}
                              </span>
                              <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2.5 py-1">
                                {selectedVoiceLabel}
                              </span>
                              <span className="rounded-full border border-[var(--color-editorial-line)] bg-white px-2.5 py-1">
                                {selectedVoiceDetail}
                              </span>
                            </div>

                            <div className="ml-auto flex items-center gap-3">
                              <div className={`rounded-full border px-3 py-1 text-[11px] font-semibold ${badgeClasses}`}>
                                {status.label}
                              </div>
                              <ChevronDown className={`size-4 text-[var(--color-editorial-ink-soft)] transition-transform ${expanded ? "rotate-180" : ""}`} />
                            </div>
                          </button>

                          {expanded ? (
                            <div className="border-t border-[var(--color-editorial-line)] bg-white/70 px-4 py-4">
                              <div className="flex flex-wrap gap-3">
                                <div className="min-w-[220px] flex-[1.4] rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                    {t("settings.audioBriefing.ttsProvider")}
                                  </div>
                                  <select
                                    value={voice.tts_provider}
                                    onChange={(e) => {
                                      updateAudioBriefingVoice(voice.persona, {
                                        tts_provider: e.target.value,
                                      });
                                    }}
                                    className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                  >
                                    {Array.from(new Set([
                                      voice.tts_provider,
                                      "aivis",
                                      hasUserFishAPIKey || voice.tts_provider === "fish" ? "fish" : null,
                                      hasUserXAIAPIKey || voice.tts_provider === "xai" ? "xai" : null,
                                      hasUserOpenAIAPIKey || voice.tts_provider === "openai" ? "openai" : null,
                                      geminiTTSEnabled || voice.tts_provider === "gemini_tts" ? "gemini_tts" : null,
                                      "mock",
                                    ].filter(Boolean) as string[])).map((provider) => (
                                      <option key={`${voice.persona}-${provider}`} value={provider}>
                                        {provider === "fish" ? t("settings.audioBriefing.provider.fish") : provider}
                                      </option>
                                    ))}
                                  </select>
                                  <p className="mt-3 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{status.detail}</p>
                                </div>

                                <div className="min-w-[260px] flex-[2] rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                  <div className="flex flex-wrap items-start justify-between gap-3">
                                    <div>
                                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                        {t("settings.audioBriefing.voiceModel")}
                                      </div>
                                      <div className="mt-3 text-sm font-semibold text-[var(--color-editorial-ink)]">
                                        {isAivisProvider
                                          ? aivisResolved?.model?.name ?? t("settings.audioBriefing.aivisVoiceEmpty")
                                          : isFishProvider
                                            ? voice.provider_voice_label || voice.voice_model || t("settings.audioBriefing.fishVoiceEmpty")
                                            : isXAIProvider
                                              ? xaiResolved?.name ?? t("settings.audioBriefing.xaiVoiceEmpty")
                                              : isOpenAIProvider
                                                ? openAIResolved?.name ?? t("settings.audioBriefing.openAITTSVoiceEmpty")
                                                : isGeminiProvider
                                                  ? geminiResolved?.label ?? t("settings.audioBriefing.geminiTTSVoiceEmpty")
                                                  : voice.voice_model || t("settings.audioBriefing.unsetShort")}
                                      </div>
                                      <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                                        {isAivisProvider
                                          ? aivisResolved?.speaker && aivisResolved?.style
                                            ? `${aivisResolved.speaker.name} / ${aivisResolved.style.name}`
                                            : voice.voice_style || voice.voice_model || t("settings.audioBriefing.unsetShort")
                                          : isFishProvider
                                            ? voice.provider_voice_description || voice.voice_model || t("settings.audioBriefing.unsetShort")
                                            : isXAIProvider
                                              ? xaiResolved?.description || voice.voice_model || t("settings.audioBriefing.unsetShort")
                                              : isOpenAIProvider
                                                ? openAIResolved?.description || voice.voice_model || t("settings.audioBriefing.unsetShort")
                                                : isGeminiProvider
                                                  ? geminiResolved?.description || voice.voice_model || t("settings.audioBriefing.unsetShort")
                                                  : voice.voice_style || voice.voice_model || t("settings.audioBriefing.unsetShort")}
                                      </div>
                                    </div>
                                  </div>

                                  {providerCapabilities.supportsCatalogPicker && isAivisProvider ? (
                                    <button
                                      type="button"
                                      onClick={() => void openAivisPicker(voice.persona)}
                                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] hover:bg-[var(--color-editorial-panel-strong)]"
                                    >
                                      {t("settings.audioBriefing.pickAivisVoice")}
                                    </button>
                                  ) : providerCapabilities.supportsCatalogPicker && isFishProvider ? (
                                    <button
                                      type="button"
                                      onClick={() => void openFishPicker(voice.persona)}
                                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] hover:bg-[var(--color-editorial-panel-strong)]"
                                    >
                                      {t("settings.audioBriefing.pickFishVoice")}
                                    </button>
                                  ) : providerCapabilities.supportsCatalogPicker && isXAIProvider ? (
                                    <button
                                      type="button"
                                      onClick={() => void openXAIPicker(voice.persona)}
                                      disabled={!hasUserXAIAPIKey && !audioBriefingXAIVoices.length}
                                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
                                    >
                                      {t("settings.audioBriefing.pickXaiVoice")}
                                    </button>
                                  ) : providerCapabilities.supportsCatalogPicker && isOpenAIProvider ? (
                                    <button
                                      type="button"
                                      onClick={() => void openOpenAITTSPicker(voice.persona)}
                                      disabled={!hasUserOpenAIAPIKey && !audioBriefingOpenAITTSVoices.length}
                                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
                                    >
                                      {t("settings.audioBriefing.pickOpenAITTSVoice")}
                                    </button>
                                  ) : providerCapabilities.supportsCatalogPicker && isGeminiProvider ? (
                                    <button
                                      type="button"
                                      onClick={() => void openGeminiTTSPicker(voice.persona)}
                                      disabled={geminiTTSVoicesLoading && !audioBriefingGeminiTTSVoices.length}
                                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-60"
                                    >
                                      {t("settings.audioBriefing.pickGeminiTTSVoice")}
                                    </button>
                                  ) : null}
                                </div>
                              </div>

                              {providerCapabilities.supportsSpeechTuning ? (
                                <div className="mt-4 flex flex-wrap gap-3 text-[11px] text-[var(--color-editorial-ink-faint)]">
                                  <span>{`${t("settings.audioBriefing.voiceModel")}: ${voice.voice_model || "—"}`}</span>
                                  <span>{`${t("settings.audioBriefing.voiceStyle")}: ${formatAivisVoiceStyleLabel(voice.voice_style, t)}`}</span>
                                </div>
                              ) : isXAIProvider ? (
                                <div className="mt-4 flex flex-wrap gap-3 text-[11px] text-[var(--color-editorial-ink-faint)]">
                                  <span>{`${t("settings.audioBriefing.voiceModel")}: ${voice.voice_model || "—"}`}</span>
                                  <span>{t("settings.audioBriefing.xaiVoiceStyleDisabled")}</span>
                                </div>
                              ) : isFishProvider ? (
                                <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.85fr)]">
                                  <ModelSelect
                                    key={`audio-briefing-fish-tts-model-${voice.persona}-${voice.tts_provider}`}
                                    label={t("settings.audioBriefing.fishTTSModel")}
                                    value={voice.tts_model}
                                    onChange={(value) => updateAudioBriefingVoice(voice.persona, { tts_model: value })}
                                    options={buildFishTTSModelOptions(voice.tts_model)}
                                    labels={modelSelectLabels}
                                    variant="modal"
                                  />
                                  <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                      {t("settings.audioBriefing.fishVoice")}
                                    </div>
                                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                                      {voice.voice_model || t("settings.audioBriefing.fishVoiceEmpty")}
                                    </div>
                                    <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                                      {voice.voice_model || t("settings.audioBriefing.fishVoiceEmpty")}
                                    </div>
                                  </div>
                                </div>
                              ) : isOpenAIProvider ? (
                                <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.85fr)]">
                                  <ModelSelect
                                    key={`audio-briefing-openai-tts-model-${voice.persona}-${voice.tts_provider}`}
                                    label={t("settings.audioBriefing.openAITTSModel")}
                                    value={voice.tts_model}
                                    onChange={(value) => updateAudioBriefingVoice(voice.persona, { tts_model: value })}
                                    options={buildOpenAITTSModelOptions(voice.tts_model)}
                                    labels={modelSelectLabels}
                                    variant="modal"
                                  />
                                  <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                      {t("settings.audioBriefing.openAITTSVoice")}
                                    </div>
                                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                                      {openAIResolved?.name || t("settings.audioBriefing.openAITTSVoiceEmpty")}
                                    </div>
                                    <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                                      {openAIResolved?.description || voice.voice_model || t("settings.audioBriefing.openAITTSVoiceEmpty")}
                                    </div>
                                  </div>
                                </div>
                              ) : isGeminiProvider ? (
                                <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(220px,0.85fr)]">
                                  <ModelSelect
                                    key={`audio-briefing-gemini-tts-model-${voice.persona}-${voice.tts_provider}`}
                                    label={t("settings.audioBriefing.geminiTTSModel")}
                                    value={voice.tts_model}
                                    onChange={(value) => updateAudioBriefingVoice(voice.persona, { tts_model: value })}
                                    options={buildGeminiTTSModelOptions(voice.tts_model)}
                                    labels={modelSelectLabels}
                                    variant="modal"
                                  />
                                  <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                      {t("settings.audioBriefing.geminiTTSVoice")}
                                    </div>
                                    <div className="mt-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
                                      {geminiResolved?.label || t("settings.audioBriefing.geminiTTSVoiceEmpty")}
                                    </div>
                                    <div className="mt-1 text-[12px] text-[var(--color-editorial-ink-soft)]">
                                      {geminiResolved?.description || voice.voice_model || t("settings.audioBriefing.geminiTTSVoiceEmpty")}
                                    </div>
                                  </div>
                                </div>
                              ) : (
                                <div className="mt-4 flex flex-wrap gap-3">
                                  <input
                                    value={voice.voice_model}
                                    onChange={(e) => updateAudioBriefingVoice(voice.persona, { voice_model: e.target.value })}
                                    placeholder={t("settings.audioBriefing.voiceModel")}
                                    className="min-w-[180px] flex-1 rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                  />
                                  {providerCapabilities.requiresVoiceStyle ? (
                                    <input
                                      value={voice.voice_style}
                                      onChange={(e) => updateAudioBriefingVoice(voice.persona, { voice_style: e.target.value })}
                                      placeholder={t("settings.audioBriefing.voiceStyle")}
                                      className="min-w-[180px] flex-1 rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                    />
                                  ) : null}
                                </div>
                              )}

                              <div className="mt-3 flex flex-wrap gap-3">
                                <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                    {t("settings.audioBriefing.speechRate")}
                                  </div>
                                  <input
                                    value={audioBriefingVoiceInputDrafts[voice.persona]?.speech_rate ?? formatAudioBriefingDecimalInput(voice.speech_rate)}
                                    onChange={(e) => updateAudioBriefingVoiceNumberInput(voice.persona, "speech_rate", e.target.value, (value) => ({ speech_rate: value }))}
                                    onBlur={() => resetAudioBriefingVoiceNumberInput(voice.persona, "speech_rate")}
                                    inputMode="decimal"
                                    className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                  />
                                </label>

                                {providerCapabilities.supportsSpeechTuning ? (
                                  <>
                                    <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                        {t("settings.audioBriefing.tempoDynamics")}
                                      </div>
                                      <input
                                        value={audioBriefingVoiceInputDrafts[voice.persona]?.tempo_dynamics ?? formatAudioBriefingDecimalInput(voice.tempo_dynamics)}
                                        onChange={(e) => updateAudioBriefingVoiceNumberInput(voice.persona, "tempo_dynamics", e.target.value, (value) => ({ tempo_dynamics: value }))}
                                        onBlur={() => resetAudioBriefingVoiceNumberInput(voice.persona, "tempo_dynamics")}
                                        inputMode="decimal"
                                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                      />
                                    </label>

                                    <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                        {t("settings.audioBriefing.emotionalIntensity")}
                                      </div>
                                      <input
                                        value={audioBriefingVoiceInputDrafts[voice.persona]?.emotional_intensity ?? formatAudioBriefingDecimalInput(voice.emotional_intensity)}
                                        onChange={(e) => updateAudioBriefingVoiceNumberInput(voice.persona, "emotional_intensity", e.target.value, (value) => ({ emotional_intensity: value }))}
                                        onBlur={() => resetAudioBriefingVoiceNumberInput(voice.persona, "emotional_intensity")}
                                        inputMode="decimal"
                                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                      />
                                    </label>

                                    <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                        {t("settings.audioBriefing.lineBreakSilenceSeconds")}
                                      </div>
                                      <input
                                        value={audioBriefingVoiceInputDrafts[voice.persona]?.line_break_silence_seconds ?? formatAudioBriefingDecimalInput(voice.line_break_silence_seconds)}
                                        onChange={(e) => updateAudioBriefingVoiceNumberInput(voice.persona, "line_break_silence_seconds", e.target.value, (value) => ({ line_break_silence_seconds: value }))}
                                        onBlur={() => resetAudioBriefingVoiceNumberInput(voice.persona, "line_break_silence_seconds")}
                                        inputMode="decimal"
                                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                      />
                                    </label>

                                    <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                        {t("settings.audioBriefing.aivisVolume")}
                                      </div>
                                      <input
                                        value={audioBriefingVoiceInputDrafts[voice.persona]?.aivis_volume ?? formatAudioBriefingDecimalInput(voice.volume_gain + 1)}
                                        onChange={(e) => updateAudioBriefingVoiceNumberInput(voice.persona, "aivis_volume", e.target.value, (value) => ({ volume_gain: value - 1 }))}
                                        onBlur={() => resetAudioBriefingVoiceNumberInput(voice.persona, "aivis_volume")}
                                        inputMode="decimal"
                                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                      />
                                    </label>
                                  </>
                                ) : (
                                  <>
                                    <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                        {t("settings.audioBriefing.pitchAdjustment")}
                                      </div>
                                      <input
                                        value={audioBriefingVoiceInputDrafts[voice.persona]?.pitch ?? formatAudioBriefingDecimalInput(voice.pitch)}
                                        onChange={(e) => updateAudioBriefingVoiceNumberInput(voice.persona, "pitch", e.target.value, (value) => ({ pitch: value }))}
                                        onBlur={() => resetAudioBriefingVoiceNumberInput(voice.persona, "pitch")}
                                        inputMode="decimal"
                                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                      />
                                    </label>

                                    <label className="min-w-[160px] flex-1 rounded-[16px] border border-[var(--color-editorial-line)] bg-white p-4">
                                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                                        {t("settings.audioBriefing.volumeAdjustment")}
                                      </div>
                                      <input
                                        value={audioBriefingVoiceInputDrafts[voice.persona]?.volume_gain ?? formatAudioBriefingDecimalInput(voice.volume_gain)}
                                        onChange={(e) => updateAudioBriefingVoiceNumberInput(voice.persona, "volume_gain", e.target.value, (value) => ({ volume_gain: value }))}
                                        onBlur={() => resetAudioBriefingVoiceNumberInput(voice.persona, "volume_gain")}
                                        inputMode="decimal"
                                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                                      />
                                    </label>
                                  </>
                                )}
                              </div>

                              <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
                                <p className="text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">
                                  {t("settings.audioBriefing.inlineHelp")}
                                </p>
                                <div className="flex flex-wrap gap-2">
                                  {isAivisProvider ? (
                                    <button
                                      type="button"
                                      onClick={() => {
                                        void syncAivisModels();
                                      }}
                                      disabled={aivisModelsSyncing}
                                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                                    >
                                      {aivisModelsSyncing ? t("aivisModels.syncing") : t("settings.audioBriefing.refreshCatalog")}
                                    </button>
                                  ) : isXAIProvider ? (
                                    <button
                                      type="button"
                                      onClick={() => {
                                        void syncXAIVoices();
                                      }}
                                      disabled={xaiVoicesSyncing}
                                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                                    >
                                      {xaiVoicesSyncing ? t("settings.audioBriefing.syncingXaiCatalog") : t("settings.audioBriefing.refreshXaiCatalog")}
                                    </button>
                                  ) : isOpenAIProvider ? (
                                    <button
                                      type="button"
                                      onClick={() => {
                                        void syncOpenAITTSVoices().catch(() => undefined);
                                      }}
                                      disabled={openAITTSVoicesSyncing}
                                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                                    >
                                      {openAITTSVoicesSyncing ? t("settings.audioBriefing.syncingOpenAITTSCatalog") : t("settings.audioBriefing.refreshOpenAITTSCatalog")}
                                    </button>
                                  ) : isGeminiProvider ? (
                                    <button
                                      type="button"
                                      onClick={() => {
                                        void loadGeminiTTSVoices().catch(() => undefined);
                                      }}
                                      disabled={geminiTTSVoicesLoading}
                                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                                    >
                                      {geminiTTSVoicesLoading ? t("common.loading") : t("settings.audioBriefing.refreshGeminiTTSCatalog")}
                                    </button>
                                  ) : null}

                                  <button
                                    type="button"
                                    onClick={() => {
                                      void persistAudioBriefingVoices();
                                    }}
                                    disabled={savingAudioBriefingVoices}
                                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
                                  >
                                    {savingAudioBriefingVoices ? t("common.saving") : t("settings.audioBriefing.savePersonaVoice")}
                                  </button>
                                </div>
                              </div>
                            </div>
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
                </form>
              </SectionCard>

              <SectionCard>
                <form onSubmit={submitPodcastSettings} className="space-y-5">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div>
                      <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.podcast.title")}</div>
                      <p className="mt-1 max-w-3xl text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.podcast.description")}</p>
                    </div>
                    <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
                      <button
                        type="submit"
                        disabled={savingPodcast}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                      >
                        {savingPodcast ? t("common.saving") : t("settings.podcast.save")}
                      </button>
                      <button
                        type="button"
                        disabled={!podcastRSSURL}
                        onClick={copyPodcastRSSURL}
                        className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:opacity-60"
                      >
                        {t("settings.podcast.copyRSS")}
                      </button>
                    </div>
                  </div>

                  <div className="grid gap-3 lg:grid-cols-2">
                    <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                            {t("settings.podcast.enabled")}
                          </div>
                          <p className="mt-2 text-sm text-[var(--color-editorial-ink)]">{podcastEnabled ? t("settings.on") : t("settings.off")}</p>
                        </div>
                        <input
                          type="checkbox"
                          checked={podcastEnabled}
                          onChange={(e) => setPodcastEnabled(e.target.checked)}
                          className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                        />
                      </div>
                    </label>

                    <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.podcast.rssUrl")}
                      </div>
                      <div className="mt-3 break-all rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]">
                        {podcastRSSURL || t("settings.podcast.rssUrlPending")}
                      </div>
                    </div>

                    <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.podcast.feedSlug")}
                      </div>
                      <input
                        value={podcastFeedSlug}
                        readOnly
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                    </label>

                    <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.podcast.language")}
                      </div>
                      <select
                        value={podcastLanguage}
                        onChange={(e) => setPodcastLanguage(e.target.value)}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      >
                        <option value="ja">ja</option>
                        <option value="en">en</option>
                      </select>
                    </label>

                    <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.podcast.category")}
                      </div>
                      <select
                        value={podcastCategory}
                        onChange={(e) => {
                          const nextCategory = e.target.value;
                          setPodcastCategory(nextCategory);
                          setPodcastSubcategory("");
                        }}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      >
                        <option value="">{t("settings.podcast.categoryUnset")}</option>
                        {podcastAvailableCategories.map((option) => (
                          <option key={option.category} value={option.category}>
                            {option.category}
                          </option>
                        ))}
                      </select>
                    </label>

                    <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.podcast.subcategory")}
                      </div>
                      <select
                        value={podcastSubcategory}
                        onChange={(e) => setPodcastSubcategory(e.target.value)}
                        disabled={!selectedPodcastCategory || selectedPodcastCategory.subcategories.length === 0}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)] disabled:opacity-60"
                      >
                        <option value="">{t("settings.podcast.subcategoryUnset")}</option>
                        {(selectedPodcastCategory?.subcategories ?? []).map((subcategory) => (
                          <option key={subcategory} value={subcategory}>
                            {subcategory}
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>

                  <div className="grid gap-3 lg:grid-cols-2">
                    <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.podcast.showTitle")}
                      </div>
                      <input
                        value={podcastTitle}
                        onChange={(e) => setPodcastTitle(e.target.value)}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                    </label>

                    <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.podcast.author")}
                      </div>
                      <input
                        value={podcastAuthor}
                        onChange={(e) => setPodcastAuthor(e.target.value)}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                    </label>
                  </div>

                  <label className="block rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.podcast.summary")}
                    </div>
                    <textarea
                      value={podcastDescription}
                      onChange={(e) => setPodcastDescription(e.target.value)}
                      rows={5}
                      className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                    />
                  </label>

                  <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_220px]">
                    <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.podcast.artworkUrl")}
                      </div>
                      <input
                        value={podcastArtworkURL}
                        onChange={(e) => setPodcastArtworkURL(e.target.value)}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                      <div className="mt-3 flex flex-wrap gap-2">
                        <label className="inline-flex min-h-10 cursor-pointer items-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)]">
                          <input
                            type="file"
                            accept="image/png,image/jpeg,image/webp"
                            className="hidden"
                            onChange={(e) => void handlePodcastArtworkFileChange(e.target.files?.[0] ?? null)}
                          />
                          {uploadingPodcastArtwork ? t("common.saving") : t("settings.podcast.uploadArtwork")}
                        </label>
                        <button
                          type="button"
                          onClick={() => setPodcastArtworkURL("")}
                          className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)]"
                        >
                          {t("settings.podcast.useDefaultArtwork")}
                        </button>
                      </div>
                    </div>

                    <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.podcast.explicit")}
                      </div>
                      <div className="mt-3 flex items-center justify-between gap-3">
                        <div className="text-sm text-[var(--color-editorial-ink)]">{podcastExplicit ? t("settings.podcast.explicitYes") : t("settings.podcast.explicitNo")}</div>
                        <input
                          type="checkbox"
                          checked={podcastExplicit}
                          onChange={(e) => setPodcastExplicit(e.target.checked)}
                          className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                        />
                      </div>
                    </label>
                  </div>

                  <p className="text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">
                    {t("settings.podcast.help")}
                  </p>
                </form>
              </SectionCard>
            </>
          ) : null}

          {activeSection === "summary-audio" ? (
            <SectionCard>
              <form onSubmit={submitSummaryAudioSettings} className="space-y-5">
                <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                  <div>
                    <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.summaryAudio.summaryTitle")}</div>
                    <p className="mt-1 max-w-3xl text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.summaryAudio.summaryHelp")}</p>
                  </div>
                  <div className="flex flex-wrap justify-end gap-2 lg:ml-auto">
                    <button
                      type="submit"
                      disabled={summaryAudioSaving}
                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      {summaryAudioSaving ? t("common.saving") : t("settings.summaryAudio.saveSettings")}
                    </button>
                  </div>
                </div>

                <div className={`rounded-[20px] border px-4 py-4 ${summaryAudioVoiceStatus.tone === "ok" ? "border-[rgba(34,197,94,0.28)] bg-[rgba(240,253,244,0.72)]" : summaryAudioVoiceStatus.tone === "warn" ? "border-[rgba(245,158,11,0.35)] bg-[rgba(255,251,235,0.82)]" : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]"}`}>
                  <div className="flex flex-wrap items-center gap-3">
                    <div className={`rounded-full border px-3 py-1 text-[11px] font-semibold ${
                      summaryAudioVoiceStatus.tone === "ok"
                        ? "border-[rgba(34,197,94,0.24)] bg-[rgba(220,252,231,0.85)] text-[#166534]"
                        : summaryAudioVoiceStatus.tone === "warn"
                          ? "border-[rgba(245,158,11,0.24)] bg-[rgba(254,243,199,0.88)] text-[#b45309]"
                          : "border-[var(--color-editorial-line)] bg-white text-[var(--color-editorial-ink-soft)]"
                    }`}>
                      {summaryAudioVoiceStatus.label}
                    </div>
                    <div className="text-sm text-[var(--color-editorial-ink-soft)]">{summaryAudioVoiceStatus.detail}</div>
                    <div className="ml-auto text-xs font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {summaryAudioConfigured ? t("settings.summaryAudio.playbackEnabled") : t("settings.summaryAudio.playbackDisabled")}
                    </div>
                  </div>
                </div>

                <div className="grid gap-3 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)]">
                  <label className="flex min-w-[220px] flex-1 flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.summaryAudio.provider")}
                    </div>
                    <select
                      value={summaryAudioProvider}
                      onChange={(e) => updateSummaryAudioProvider(e.target.value)}
                      className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                    >
                      <option value="aivis">{t("settings.summaryAudio.provider.aivis")}</option>
                      <option value="fish">{t("settings.summaryAudio.provider.fish")}</option>
                      <option value="xai">{t("settings.summaryAudio.provider.xai")}</option>
                      <option value="openai">{t("settings.summaryAudio.provider.openai")}</option>
                      <option value="gemini_tts">{t("settings.summaryAudio.provider.gemini_tts")}</option>
                    </select>
                  </label>

                  {summaryAudioProviderCapabilities.supportsSeparateTTSModel ? (
                    <ModelSelect
                      key={`summary-audio-tts-model-${summaryAudioProvider}`}
                      label={t("settings.summaryAudio.ttsModel")}
                      value={summaryAudioTTSModel}
                      onChange={(value) => setSummaryAudioTTSModel(value)}
                      options={
                        summaryAudioProvider === "fish"
                          ? buildFishTTSModelOptions(summaryAudioTTSModel)
                          : summaryAudioProvider === "openai"
                            ? buildOpenAITTSModelOptions(summaryAudioTTSModel)
                            : buildGeminiTTSModelOptions(summaryAudioTTSModel)
                      }
                      labels={modelSelectLabels}
                      variant="modal"
                    />
                  ) : (
                    <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.summaryAudio.voiceModel")}
                      </div>
                      <div className="mt-2 text-sm text-[var(--color-editorial-ink-soft)]">
                        {t("settings.summaryAudio.voiceModelHelp")}
                      </div>
                    </div>
                  )}
                </div>

                <label className="flex flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                    {t("settings.summaryAudio.voiceModel")}
                  </div>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <div className="rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]">
                      {summaryAudioResolvedVoiceLabel}
                    </div>
                    <button
                      type="button"
                      onClick={() => {
                        if (summaryAudioProvider === "aivis") {
                          setSummaryAudioAivisPickerOpen(true);
                          if (aivisModelsData == null) {
                            void loadAivisModels().catch(() => undefined);
                          }
                        } else if (summaryAudioProvider === "fish") {
                          setSummaryAudioFishPickerOpen(true);
                        } else if (summaryAudioProvider === "xai") {
                          setSummaryAudioXAIPickerOpen(true);
                          if (xaiVoicesData == null) {
                            void loadXAIVoices().catch(() => undefined);
                          }
                        } else if (summaryAudioProvider === "openai") {
                          setSummaryAudioOpenAITTPickerOpen(true);
                          if (openAITTSVoicesData == null) {
                            void loadOpenAITTSVoices().catch(() => undefined);
                          }
                        } else if (summaryAudioProvider === "gemini_tts") {
                          setSummaryAudioGeminiTTSPickerOpen(true);
                          if (geminiTTSVoicesData == null) {
                            void loadGeminiTTSVoices().catch(() => undefined);
                          }
                        }
                      }}
                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                    >
                      {summaryAudioProvider === "aivis"
                        ? t("settings.audioBriefing.pickAivisVoice")
                        : summaryAudioProvider === "fish"
                          ? t("settings.audioBriefing.pickFishVoice")
                        : summaryAudioProvider === "xai"
                          ? t("settings.audioBriefing.pickXaiVoice")
                          : summaryAudioProvider === "openai"
                            ? t("settings.audioBriefing.pickOpenAITTSVoice")
                            : t("settings.audioBriefing.pickGeminiTTSVoice")}
                    </button>
                  </div>
                  <div className="mt-3 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{summaryAudioResolvedVoiceDetail}</div>
                </label>

                {summaryAudioProviderCapabilities.requiresVoiceStyle ? (
                  <label className="block rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.summaryAudio.voiceStyle")}
                    </div>
                    {summaryAudioProvider === "aivis" ? (
                      <div className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]">
                        {formatAivisVoiceStyleLabel(summaryAudioVoiceStyle, t)}
                      </div>
                    ) : (
                      <input
                        value={summaryAudioVoiceStyle}
                        onChange={(e) => setSummaryAudioVoiceStyle(e.target.value)}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      />
                    )}
                    <p className="mt-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                      {t("settings.summaryAudio.voiceStyleHelp")}
                    </p>
                  </label>
                ) : null}

                <div className="grid gap-3 lg:grid-cols-3">
                  <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.summaryAudio.speechRate")}
                    </div>
                    <input
                      value={summaryAudioVoiceInputDrafts.speech_rate}
                      onChange={(e) => updateSummaryAudioVoiceNumberInput("speech_rate", e.target.value, (value) => setSummaryAudioSpeechRate(String(value)))}
                      onBlur={() => resetSummaryAudioVoiceNumberInput("speech_rate")}
                      inputMode="decimal"
                      className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                    />
                  </label>
                  <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.summaryAudio.emotionalIntensity")}
                    </div>
                    <input
                      value={summaryAudioVoiceInputDrafts.emotional_intensity}
                      onChange={(e) => updateSummaryAudioVoiceNumberInput("emotional_intensity", e.target.value, (value) => setSummaryAudioEmotionalIntensity(String(value)))}
                      onBlur={() => resetSummaryAudioVoiceNumberInput("emotional_intensity")}
                      inputMode="decimal"
                      className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                    />
                  </label>
                  <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.summaryAudio.tempoDynamics")}
                    </div>
                    <input
                      value={summaryAudioVoiceInputDrafts.tempo_dynamics}
                      onChange={(e) => updateSummaryAudioVoiceNumberInput("tempo_dynamics", e.target.value, (value) => setSummaryAudioTempoDynamics(String(value)))}
                      onBlur={() => resetSummaryAudioVoiceNumberInput("tempo_dynamics")}
                      inputMode="decimal"
                      className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                    />
                  </label>
                </div>

                <div className="grid gap-3 lg:grid-cols-3">
                  <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.summaryAudio.lineBreakSilenceSeconds")}
                    </div>
                    <input
                      value={summaryAudioVoiceInputDrafts.line_break_silence_seconds}
                      onChange={(e) => updateSummaryAudioVoiceNumberInput("line_break_silence_seconds", e.target.value, (value) => setSummaryAudioLineBreakSilenceSeconds(String(value)))}
                      onBlur={() => resetSummaryAudioVoiceNumberInput("line_break_silence_seconds")}
                      inputMode="decimal"
                      className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                    />
                  </label>
                  <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.summaryAudio.pitch")}
                    </div>
                    <input
                      value={summaryAudioVoiceInputDrafts.pitch}
                      onChange={(e) => updateSummaryAudioVoiceNumberInput("pitch", e.target.value, (value) => setSummaryAudioPitch(String(value)))}
                      onBlur={() => resetSummaryAudioVoiceNumberInput("pitch")}
                      inputMode="decimal"
                      className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                    />
                  </label>
                  <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("settings.summaryAudio.volumeGain")}
                    </div>
                    <input
                      value={summaryAudioVoiceInputDrafts.volume_gain}
                      onChange={(e) => updateSummaryAudioVoiceNumberInput("volume_gain", e.target.value, (value) => setSummaryAudioVolumeGain(String(value)))}
                      onBlur={() => resetSummaryAudioVoiceNumberInput("volume_gain")}
                      inputMode="decimal"
                      className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                    />
                  </label>
                </div>

                {summaryAudioProvider === "aivis" ? (
                  !settings?.has_aivis_api_key ? (
                    <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
                      <div>
                        <div className="font-semibold">{t("settings.summaryAudio.aivisApiKeyWarningTitle")}</div>
                        <div className="mt-1 leading-6">{t("settings.summaryAudio.aivisApiKeyWarningDetail")}</div>
                      </div>
                      <button
                        type="button"
                        onClick={() => setActiveSection("system")}
                        className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
                      >
                        {t("settings.summaryAudio.openApiKeys")}
                      </button>
                    </div>
                  ) : (
                    <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.8fr)]">
                      <label className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                        <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                          {t("settings.summaryAudio.aivisDictionary")}
                        </div>
                        <select
                          value={summaryAudioAivisUserDictionaryUUID}
                          onChange={(e) => setSummaryAudioAivisUserDictionaryUUID(e.target.value)}
                          disabled={aivisUserDictionariesLoading || aivisUserDictionaries.length === 0}
                          className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)] disabled:opacity-60"
                        >
                          <option value="">{t("settings.aivisDictionaryUnset")}</option>
                          {aivisUserDictionaries.map((item) => (
                            <option key={item.uuid} value={item.uuid}>
                              {`${item.name} (${item.word_count})`}
                            </option>
                          ))}
                        </select>
                        <p className="mt-2 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                          {t("settings.summaryAudio.aivisDictionaryHelp")}
                        </p>
                      </label>
                      <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                        <div className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.summaryAudio.aivisDictionaryTitle")}</div>
                        <p className="mt-2">{t("settings.summaryAudio.aivisDictionaryDetail")}</p>
                      </div>
                    </div>
                  )
                ) : summaryAudioProvider === "fish" ? (
                  !settings?.has_fish_api_key ? (
                    <div className="flex flex-col gap-3 rounded-[16px] border border-[rgba(245,158,11,0.28)] bg-[rgba(255,251,235,0.85)] px-4 py-4 text-sm text-[#b45309] lg:flex-row lg:items-center lg:justify-between">
                      <div>
                        <div className="font-semibold">{t("settings.summaryAudio.fishApiKeyWarningTitle")}</div>
                        <div className="mt-1 leading-6">{t("settings.summaryAudio.fishApiKeyWarningDetail")}</div>
                      </div>
                      <button
                        type="button"
                        onClick={() => setActiveSection("system")}
                        className="inline-flex min-h-10 items-center justify-center rounded-full border border-[rgba(180,83,9,0.22)] bg-white px-4 py-2 text-sm font-medium text-[#92400e] hover:bg-[rgba(255,255,255,0.72)]"
                      >
                        {t("settings.summaryAudio.openApiKeys")}
                      </button>
                    </div>
                  ) : (
                    <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                      <div className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.summaryAudio.fishVoiceTitle")}</div>
                      <p className="mt-2">{t("settings.summaryAudio.fishVoiceDetail")}</p>
                    </div>
                  )
                ) : null}
              </form>
            </SectionCard>
          ) : null}

          {activeSection === "reading-plan" ? (
            <SectionCard>
              <form onSubmit={submitReadingPlan} className="space-y-5">
                <div className="grid gap-4 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
                  <div>
                    <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.window")}</div>
                    <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.windowHelp")}</p>
                  </div>
                  <select
                    value={readingPlanWindow}
                    onChange={(e) => setReadingPlanWindow(e.target.value as "24h" | "today_jst" | "7d")}
                    className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                  >
                    <option value="24h">{t("settings.window.24h")}</option>
                    <option value="today_jst">{t("settings.window.today")}</option>
                    <option value="7d">{t("settings.window.7d")}</option>
                  </select>
                </div>
                <div className="grid gap-4 border-t border-[var(--color-editorial-line)] pt-5 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
                  <div>
                    <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.size")}</div>
                    <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.sizeHelp")}</p>
                  </div>
                  <select
                    value={readingPlanSize}
                    onChange={(e) => setReadingPlanSize(e.target.value)}
                    className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                  >
                    {[7, 15, 25].map((n) => (
                      <option key={n} value={String(n)}>
                        {n}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="grid gap-4 border-t border-[var(--color-editorial-line)] pt-5 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
                  <div>
                    <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.diversifyTopics")}</div>
                    <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.diversifyHelp")}</p>
                  </div>
                  <label className="flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                    <span>{readingPlanDiversifyTopics ? t("settings.controlRoom.topicBalanceOn") : t("settings.controlRoom.topicBalanceOff")}</span>
                    <input
                      type="checkbox"
                      checked={readingPlanDiversifyTopics}
                      onChange={(e) => setReadingPlanDiversifyTopics(e.target.checked)}
                      className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                    />
                  </label>
                </div>
                <button
                  type="submit"
                  disabled={savingReadingPlan}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                >
                  {savingReadingPlan ? t("common.saving") : t("settings.saveRecommended")}
                </button>
              </form>
            </SectionCard>
          ) : null}

          {activeSection === "personalization" ? (
            <SectionCard>
              <PreferenceProfilePanel
                profile={preferenceProfile}
                error={preferenceProfileError}
                onReset={() => {
                  void handleResetPreferenceProfile();
                }}
                onRetry={() => {
                  void load();
                }}
                resetting={resettingPreferenceProfile}
              />
            </SectionCard>
          ) : null}

          {activeSection === "digest" ? (
            <SectionCard>
              <form onSubmit={submitDigestDelivery} className="space-y-5">
                <div className="flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                  <div className="min-w-0">
                    <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.digestEmailSending")}</div>
                    <div className="mt-1 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{t("settings.digestDisabledHint")}</div>
                  </div>
                  <label className="inline-flex shrink-0 items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
                    <input
                      type="checkbox"
                      checked={digestEmailEnabled}
                      onChange={(e) => setDigestEmailEnabled(e.target.checked)}
                      className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                    />
                    {digestEmailEnabled ? t("settings.on") : t("settings.off")}
                  </label>
                </div>
                <button
                  type="submit"
                  disabled={savingDigestDelivery}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                >
                  {savingDigestDelivery ? t("common.saving") : t("settings.saveDelivery")}
                </button>
              </form>
            </SectionCard>
          ) : null}

          {activeSection === "notifications" ? (
            <SectionCard>
              <OneSignalSettings
                rule={notificationPriority}
                onSaveRule={async (rule) => {
                  const res = await api.updateNotificationPriority(rule);
                  setNotificationPriority(res.notification_priority);
                  setSettings((prev) => (prev ? { ...prev, notification_priority: res.notification_priority } : prev));
                }}
              />
            </SectionCard>
          ) : null}

          {activeSection === "integrations" ? (
            <div className="space-y-5">
              <SectionCard>
                <div className="mb-4">
                  <h3 className="inline-flex items-center gap-2 text-base font-semibold text-[var(--color-editorial-ink)]">
                    <KeyRound className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
                    {t("settings.inoreaderTitle")}
                  </h3>
                  <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.inoreaderDescription")}</p>
                </div>
                <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                  {settings.has_inoreader_oauth ? t("settings.inoreaderConnected") : t("settings.inoreaderNotConnected")}
                </div>
                {settings.inoreader_token_expires_at ? (
                  <p className="mt-2 break-words text-xs text-[var(--color-editorial-ink-faint)]">
                    {t("settings.inoreaderTokenExpiresAt")}: {new Date(settings.inoreader_token_expires_at).toLocaleString()}
                  </p>
                ) : null}
                <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:flex-wrap">
                  <a
                    href="/api/settings/inoreader/connect"
                    className="inline-flex min-h-10 w-full items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] sm:w-auto"
                  >
                    {t("settings.inoreaderConnect")}
                  </a>
                  <button
                    type="button"
                    disabled={deletingInoreaderOAuth || !settings.has_inoreader_oauth}
                    onClick={handleDeleteInoreaderOAuth}
                    className="min-h-10 w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:opacity-50 sm:w-auto"
                  >
                    {deletingInoreaderOAuth ? t("settings.deleting") : t("settings.inoreaderDisconnect")}
                  </button>
                </div>
              </SectionCard>

              <SectionCard>
                <form onSubmit={submitObsidianExport} className="space-y-4">
                  <div className="mb-4">
                    <h3 className="inline-flex items-center gap-2 text-base font-semibold text-[var(--color-editorial-ink)]">
                      <SettingsIcon className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
                      {t("settings.obsidianTitle")}
                    </h3>
                    <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.obsidianDescription")}</p>
                  </div>

                  <div className="flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                    <div className="min-w-0">
                      <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.obsidianEnabled")}</div>
                      <div className="mt-1 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">{t("settings.obsidianEnabledHint")}</div>
                    </div>
                    <label className="inline-flex shrink-0 items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
                      <input
                        type="checkbox"
                        checked={obsidianEnabled}
                        onChange={(e) => setObsidianEnabled(e.target.checked)}
                        className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                      />
                      {obsidianEnabled ? t("settings.on") : t("settings.off")}
                    </label>
                  </div>

                  <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                    {settings.obsidian_export?.github_installation_id
                      ? t("settings.obsidianGithubConnected")
                      : t("settings.obsidianGithubNotConnected")}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <a
                      href="/api/settings/obsidian-github/connect"
                      className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)]"
                    >
                      {t("settings.obsidianGithubConnect")}
                    </a>
                  </div>

                  <div className="grid gap-4 md:grid-cols-2">
                    <div>
                      <label className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.obsidianRepoOwner")}</label>
                      <input
                        type="text"
                        value={obsidianRepoOwner}
                        onChange={(e) => setObsidianRepoOwner(e.target.value)}
                        className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                        placeholder="your-org"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.obsidianRepoName")}</label>
                      <input
                        type="text"
                        value={obsidianRepoName}
                        onChange={(e) => setObsidianRepoName(e.target.value)}
                        className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                        placeholder="obsidian-vault"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.obsidianBranch")}</label>
                      <input
                        type="text"
                        value={obsidianRepoBranch}
                        onChange={(e) => setObsidianRepoBranch(e.target.value)}
                        className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                        placeholder="main"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.obsidianRootPath")}</label>
                      <input
                        type="text"
                        value={obsidianRootPath}
                        onChange={(e) => setObsidianRootPath(e.target.value)}
                        className="mt-1 w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]"
                        placeholder="Sifto/Favorites"
                      />
                      <p className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">{t("settings.obsidianRootPathHint")}</p>
                    </div>
                  </div>

                  {settings.obsidian_export?.last_success_at ? (
                    <p className="text-xs text-[var(--color-editorial-ink-faint)]">
                      {t("settings.obsidianLastSuccess")}: {new Date(settings.obsidian_export.last_success_at).toLocaleString()}
                    </p>
                  ) : null}

                  <div className="flex flex-wrap gap-2">
                    <button
                      type="submit"
                      disabled={savingObsidianExport}
                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                    >
                      {savingObsidianExport ? t("common.saving") : t("settings.obsidianSave")}
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        void runObsidianExportNow();
                      }}
                      disabled={runningObsidianExport}
                      className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:opacity-60"
                    >
                      {runningObsidianExport ? t("settings.obsidianRunNowRunning") : t("settings.obsidianRunNow")}
                    </button>
                  </div>
                </form>
              </SectionCard>
            </div>
          ) : null}

          {activeSection === "models" ? (
            <div className="space-y-5">
              <SectionCard>
                <form onSubmit={submitLLMModels} className="space-y-4">
                  <div>
                    <h3 className="inline-flex items-center gap-2 text-base font-semibold text-[var(--color-editorial-ink)]">
                      <Brain className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
                      {t("settings.modelsTitle")}
                    </h3>
                    <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.modelsDescription")}</p>
                    <p className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">{t("settings.pricingDescription")}</p>
                  </div>

                  {unavailableSelectedModelWarnings.length > 0 ? (
                    <div className="rounded-[16px] border border-[#e1cb9e] bg-[var(--color-warning-soft)] px-4 py-3 text-sm text-[var(--color-warning)]">
                      <div className="font-medium">{t("settings.modelUnavailable.title")}</div>
                      <div className="mt-2 space-y-1">
                        {unavailableSelectedModelWarnings.map((entry) => (
                          <p key={entry.key}>
                            {t("settings.modelUnavailable.message")
                              .replace("{{field}}", entry.label)
                              .replace("{{model}}", entry.modelLabel)}
                          </p>
                        ))}
                      </div>
                    </div>
                  ) : null}

                  <div className="space-y-4">
                    <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.summary")}</h4>
                      <div className="mt-3 grid gap-4 md:grid-cols-2">
                        <ModelSelect label={t("settings.model.facts")} value={anthropicFactsModel} onChange={(value) => onChangeLLMModel(setAnthropicFactsModel, value)} options={optionsForPurpose("facts", anthropicFactsModel)} labels={modelSelectLabels} variant="modal" />
                        <ModelSelect label={t("settings.model.factsSecondary")} value={anthropicFactsSecondaryModel} onChange={(value) => onChangeLLMModel(setAnthropicFactsSecondaryModel, value)} options={optionsForPurpose("facts", anthropicFactsSecondaryModel)} labels={modelSelectLabels} variant="modal" />
                        <label className="space-y-2 text-sm text-[var(--color-editorial-ink-soft)]">
                          <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.model.factsSecondaryRatePercent")}</span>
                          <input
                            type="number"
                            min={0}
                            max={100}
                            step={1}
                            value={anthropicFactsSecondaryRatePercent}
                            onChange={(event) => {
                              llmModelsDirtyRef.current = true;
                              setAnthropicFactsSecondaryRatePercent(event.target.value);
                            }}
                            className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
                          />
                        </label>
                        <ModelSelect label={t("settings.model.factsFallback")} value={anthropicFactsFallbackModel} onChange={(value) => onChangeLLMModel(setAnthropicFactsFallbackModel, value)} options={optionsForPurpose("facts", anthropicFactsFallbackModel)} labels={modelSelectLabels} variant="modal" />
                        <ModelSelect label={t("settings.model.summary")} value={anthropicSummaryModel} onChange={(value) => onChangeLLMModel(setAnthropicSummaryModel, value)} options={optionsForPurpose("summary", anthropicSummaryModel)} labels={modelSelectLabels} variant="modal" />
                        <ModelSelect label={t("settings.model.summarySecondary")} value={anthropicSummarySecondaryModel} onChange={(value) => onChangeLLMModel(setAnthropicSummarySecondaryModel, value)} options={optionsForPurpose("summary", anthropicSummarySecondaryModel)} labels={modelSelectLabels} variant="modal" />
                        <label className="space-y-2 text-sm text-[var(--color-editorial-ink-soft)]">
                          <span className="block text-sm font-medium text-[var(--color-editorial-ink)]">{t("settings.model.summarySecondaryRatePercent")}</span>
                          <input
                            type="number"
                            min={0}
                            max={100}
                            step={1}
                            value={anthropicSummarySecondaryRatePercent}
                            onChange={(event) => {
                              llmModelsDirtyRef.current = true;
                              setAnthropicSummarySecondaryRatePercent(event.target.value);
                            }}
                            className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
                          />
                        </label>
                        <ModelSelect label={t("settings.model.summaryFallback")} value={anthropicSummaryFallbackModel} onChange={(value) => onChangeLLMModel(setAnthropicSummaryFallbackModel, value)} options={optionsForPurpose("summary", anthropicSummaryFallbackModel)} labels={modelSelectLabels} variant="modal" />
                      </div>
                    </section>
                    <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.digest")}</h4>
                      <div className="mt-3 grid gap-4 md:grid-cols-2">
                        <ModelSelect label={t("settings.model.digestCluster")} value={anthropicDigestClusterModel} onChange={(value) => onChangeLLMModel(setAnthropicDigestClusterModel, value)} options={optionsForPurpose("digest_cluster_draft", anthropicDigestClusterModel)} labels={modelSelectLabels} variant="modal" />
                        <ModelSelect label={t("settings.model.digest")} value={anthropicDigestModel} onChange={(value) => onChangeLLMModel(setAnthropicDigestModel, value)} options={optionsForPurpose("digest", anthropicDigestModel)} labels={modelSelectLabels} variant="modal" />
                      </div>
                    </section>
                    <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.validation")}</h4>
                      <div className="mt-3 grid gap-4 md:grid-cols-2">
                        <ModelSelect label={t("settings.model.factsCheck")} value={factsCheckModel} onChange={(value) => onChangeLLMModel(setFactsCheckModel, value)} options={optionsForPurpose("facts", factsCheckModel)} labels={modelSelectLabels} variant="modal" />
                        <ModelSelect label={t("settings.model.faithfulnessCheck")} value={faithfulnessCheckModel} onChange={(value) => onChangeLLMModel(setFaithfulnessCheckModel, value)} options={optionsForPurpose("summary", faithfulnessCheckModel)} labels={modelSelectLabels} variant="modal" />
                      </div>
                    </section>
                    <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.other")}</h4>
                      <div className="mt-3 grid gap-4 md:grid-cols-2">
                        <ModelSelect label={t("settings.model.sourceSuggestion")} value={anthropicSourceSuggestionModel} onChange={(value) => onChangeLLMModel(setAnthropicSourceSuggestionModel, value)} options={sourceSuggestionModelOptions} labels={modelSelectLabels} variant="modal" />
                        <ModelSelect label={t("settings.model.ask")} value={anthropicAskModel} onChange={(value) => onChangeLLMModel(setAnthropicAskModel, value)} options={optionsForPurpose("ask", anthropicAskModel)} labels={modelSelectLabels} variant="modal" />
                        <ModelSelect label={t("settings.model.embeddings")} value={openAIEmbeddingModel} onChange={(value) => onChangeLLMModel(setOpenAIEmbeddingModel, value)} options={openAIEmbeddingModelOptions} labels={modelSelectLabels} variant="modal" />
                      </div>
                    </section>
                    <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.fishPreprocess")}</h4>
                      <p className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                        {t("settings.group.fishPreprocessDescription")}
                      </p>
                      <div className="mt-3 grid gap-4 md:grid-cols-2">
                        <ModelSelect
                          label={t("settings.model.fishPreprocess")}
                          value={fishPreprocessModel}
                          onChange={(value) => onChangeLLMModel(setFishPreprocessModel, value)}
                          options={optionsForChatModel(fishPreprocessModel)}
                          labels={modelSelectLabels}
                          variant="modal"
                        />
                      </div>
                    </section>
                  </div>

                  <button
                    type="submit"
                    disabled={savingLLMModels}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                  >
                    {savingLLMModels ? t("common.saving") : t("settings.saveModels")}
                  </button>
                </form>
              </SectionCard>

              {llmExtrasOpen ? (
                <div ref={llmExtrasRef} className="space-y-5">
                  <ProviderModelUpdatesPanel
                    allEvents={providerModelUpdates}
                    visibleEvents={visibleProviderModelUpdates}
                    onDismiss={dismissProviderModelUpdates}
                    onRestore={restoreProviderModelUpdates}
                    labels={{
                      title: t("settings.providerModelUpdates"),
                      description: t("settings.providerModelUpdatesDescription"),
                      dismiss: t("settings.providerModelUpdate.dismiss"),
                      empty: t("settings.providerModelUpdate.empty"),
                      dismissed: t("settings.providerModelUpdate.dismissed"),
                      restore: t("settings.providerModelUpdate.restore"),
                      added: t("settings.providerModelUpdate.added", "added"),
                      removed: t("settings.providerModelUpdate.removed", "removed"),
                    }}
                  />
                  <SectionCard>
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <h3 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.modelGuide.title")}</h3>
                        <p className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{t("settings.modelGuide.description")}</p>
                      </div>
                      <button
                        type="button"
                        onClick={() => setModelGuideOpen(true)}
                        className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                      >
                        {t("settings.modelGuide.open")}
                      </button>
                    </div>
                  </SectionCard>
                </div>
              ) : null}
            </div>
          ) : null}

          {activeSection === "navigator" ? (
            <SectionCard>
              <form onSubmit={submitLLMModels} className="space-y-5">
                <div>
                  <h3 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("settings.group.navigator")}</h3>
                  <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.navigator.description")}</p>
                </div>

                <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                  <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
                    <div>
                      <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.enabled")}</h4>
                      <p className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{t("settings.navigator.enabledHelp")}</p>
                    </div>
                    <label className="inline-flex min-h-10 items-center gap-2 self-center text-sm text-[var(--color-editorial-ink)] md:justify-self-end">
                      <input
                        type="checkbox"
                        checked={navigatorEnabled}
                        onChange={(e) => {
                          llmModelsDirtyRef.current = true;
                          setNavigatorEnabled(e.target.checked);
                        }}
                        className="size-4 rounded border-[var(--color-editorial-line)] text-[var(--color-editorial-ink)] focus:ring-[var(--color-editorial-ink)]"
                      />
                      {navigatorEnabled ? t("settings.on") : t("settings.off")}
                    </label>
                  </div>
                </section>

                <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                  <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
                    <div>
                      <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.briefEnabled")}</h4>
                      <p className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{t("settings.navigator.briefEnabledHelp")}</p>
                    </div>
                    <label className="inline-flex min-h-10 items-center gap-2 self-center text-sm text-[var(--color-editorial-ink)] md:justify-self-end">
                      <input
                        type="checkbox"
                        checked={aiNavigatorBriefEnabled}
                        onChange={(e) => {
                          llmModelsDirtyRef.current = true;
                          setAINavigatorBriefEnabled(e.target.checked);
                        }}
                        className="size-4 rounded border-[var(--color-editorial-line)] text-[var(--color-editorial-ink)] focus:ring-[var(--color-editorial-ink)]"
                      />
                      {aiNavigatorBriefEnabled ? t("settings.on") : t("settings.off")}
                    </label>
                  </div>
                </section>

                <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                  <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.persona")}</h4>
                  <div className="mt-4 flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                    <label className="flex min-w-[220px] flex-col rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("settings.personaMode.label")}
                      </div>
                      <select
                        value={navigatorPersonaMode}
                        onChange={(e) => {
                          llmModelsDirtyRef.current = true;
                          setNavigatorPersonaMode(e.target.value === "random" ? "random" : "fixed");
                        }}
                        className="mt-3 w-full rounded-[12px] border border-[var(--color-editorial-line)] bg-white px-3 py-2.5 text-sm text-[var(--color-editorial-ink)]"
                      >
                        <option value="fixed">{t("settings.personaMode.fixed")}</option>
                        <option value="random">{t("settings.personaMode.random")}</option>
                      </select>
                    </label>
                    <p className="max-w-[560px] text-xs leading-6 text-[var(--color-editorial-ink-soft)]">
                      {navigatorPersonaMode === "random"
                        ? t("settings.navigator.randomPersonaHelp")
                        : t("settings.navigator.fixedPersonaHelp")}
                    </p>
                  </div>
                  <div className="mt-4 grid gap-3 lg:grid-cols-2">
                    {navigatorPersonaCards.map((persona) => {
                      const selected = navigatorPersonaMode === "fixed" && persona.key === navigatorPersona;
                      const briefingHints = persona.briefing ?? {};
                      const itemHints = persona.item ?? {};
                      return (
                        <button
                          key={persona.key}
                          type="button"
                          onClick={async () => {
                            if (navigatorPersonaMode !== "fixed" || persona.key === navigatorPersona || savingLLMModels) return;
                            const previousPersona = settings?.llm_models?.navigator_persona ?? "editor";
                            llmModelsDirtyRef.current = true;
                            setNavigatorPersona(persona.key);
                            setSavingLLMModels(true);
                            try {
                              await persistLLMModels(
                                buildLLMModelPayload({ navigator_persona: persona.key }),
                                t("settings.toast.navigatorSaved")
                              );
                            } catch (e) {
                              setNavigatorPersona(previousPersona);
                              showToast(localizeSettingsErrorMessage(e, t), "error");
                            } finally {
                              setSavingLLMModels(false);
                            }
                          }}
                          className={joinClassNames(
                            "rounded-[18px] border bg-[var(--color-editorial-panel)] p-4 text-left transition hover:bg-[var(--color-editorial-panel-strong)]",
                            navigatorPersonaMode !== "fixed" ? "cursor-default opacity-70" : "",
                            selected
                              ? "border-[var(--color-editorial-ink)] shadow-[0_12px_32px_rgba(58,42,27,0.08)]"
                              : "border-[var(--color-editorial-line)]"
                          )}
                          aria-pressed={selected}
                          disabled={navigatorPersonaMode !== "fixed" || savingLLMModels}
                        >
                          <div className="flex items-start gap-3">
                            <div className="shrink-0 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-1.5">
                              <AINavigatorAvatar persona={persona.key} className="size-11" />
                            </div>
                            <div className="min-w-0 flex-1">
                              <div className="flex flex-wrap items-center gap-2">
                                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{persona.name}</div>
                                {selected ? (
                                  <span className="rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-panel-strong)]">
                                    {t("settings.navigator.card.selected")}
                                  </span>
                                ) : null}
                              </div>
                              <p className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                                {[persona.occupation, persona.gender, persona.age_vibe].filter(Boolean).join(" / ")}
                              </p>
                            </div>
                          </div>
                          <dl className="mt-4 space-y-3 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.personalityLabel")}</dt>
                              <dd>{persona.personality}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.firstPersonLabel")}</dt>
                              <dd>{persona.first_person}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.speechLabel")}</dt>
                              <dd>{persona.speech_style}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.experienceLabel")}</dt>
                              <dd>{persona.experience}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.valuesLabel")}</dt>
                              <dd>{persona.values}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.interestsLabel")}</dt>
                              <dd>{persona.interests}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.dislikesLabel")}</dt>
                              <dd>{persona.dislikes}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.voiceLabel")}</dt>
                              <dd>{persona.voice}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.briefingCommentRangeLabel")}</dt>
                              <dd>{briefingHints.comment_range || "-"}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.briefingIntroRangeLabel")}</dt>
                              <dd>{briefingHints.intro_range || "-"}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.briefingIntroStyleLabel")}</dt>
                              <dd>{briefingHints.intro_style || "-"}</dd>
                            </div>
                            <div>
                              <dt className="font-semibold text-[var(--color-editorial-ink)]">{t("settings.navigator.card.itemStyleLabel")}</dt>
                              <dd>{itemHints.style || "-"}</dd>
                            </div>
                          </dl>
                        </button>
                      );
                    })}
                  </div>
                </section>

                <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                  <h4 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.modelsTitle")}</h4>
                  <div className="mt-3 grid gap-4 md:grid-cols-2">
                    <ModelSelect label={t("settings.model.navigator")} value={navigatorModel} onChange={(value) => onChangeLLMModel(setNavigatorModel, value)} options={optionsForPurpose("summary", navigatorModel)} labels={modelSelectLabels} variant="modal" />
                    <ModelSelect label={t("settings.model.navigatorFallback")} value={navigatorFallbackModel} onChange={(value) => onChangeLLMModel(setNavigatorFallbackModel, value)} options={optionsForPurpose("summary", navigatorFallbackModel)} labels={modelSelectLabels} variant="modal" />
                    <ModelSelect label={t("settings.model.aiNavigatorBrief")} value={aiNavigatorBriefModel} onChange={(value) => onChangeLLMModel(setAINavigatorBriefModel, value)} options={optionsForPurpose("summary", aiNavigatorBriefModel)} labels={modelSelectLabels} variant="modal" />
                    <ModelSelect label={t("settings.model.aiNavigatorBriefFallback")} value={aiNavigatorBriefFallbackModel} onChange={(value) => onChangeLLMModel(setAINavigatorBriefFallbackModel, value)} options={optionsForPurpose("summary", aiNavigatorBriefFallbackModel)} labels={modelSelectLabels} variant="modal" />
                  </div>
                </section>

                <button
                  type="submit"
                  disabled={savingLLMModels}
                  className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                >
                  {savingLLMModels ? t("common.saving") : t("settings.saveModels")}
                </button>
              </form>
            </SectionCard>
          ) : null}

          {activeSection === "budget" ? (
            <SectionCard>
              <form onSubmit={submitBudget} className="space-y-5">
                <div className="grid gap-4 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
                  <div>
                    <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.monthlyBudgetUsd")}</div>
                    <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.monthlyBudgetHelp")}</p>
                  </div>
                  <input
                    type="number"
                    min={0}
                    step="0.01"
                    value={budgetUSD}
                    onChange={(e) => setBudgetUSD(e.target.value)}
                    placeholder={t("settings.budgetPlaceholder")}
                    className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
                  />
                </div>
                <div className="grid gap-4 border-t border-[var(--color-editorial-line)] pt-5 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
                  <div>
                    <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.budgetAlertEmail")}</div>
                    <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.budgetAlertHint")}</p>
                  </div>
                  <label className="flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                    <span>{alertEnabled ? t("settings.on") : t("settings.off")}</span>
                    <input
                      type="checkbox"
                      checked={alertEnabled}
                      onChange={(e) => setAlertEnabled(e.target.checked)}
                      className="size-4 rounded border-[var(--color-editorial-line-strong)]"
                    />
                  </label>
                </div>
                <div className="grid gap-4 border-t border-[var(--color-editorial-line)] pt-5 lg:grid-cols-[minmax(0,240px)_minmax(0,1fr)] lg:gap-6">
                  <div>
                    <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.alertThreshold")}</div>
                    <p className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("settings.controlRoom.thresholdHelp")}</p>
                  </div>
                  <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                    <div className="flex items-center gap-3">
                      <input
                        type="range"
                        min={1}
                        max={99}
                        value={thresholdPct}
                        onChange={(e) => setThresholdPct(Number(e.target.value))}
                        className="w-full accent-[var(--color-editorial-ink)]"
                      />
                      <span className={`w-12 text-right text-sm font-medium ${budgetRemainingTone}`}>{thresholdPct}%</span>
                    </div>
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  <button
                    type="submit"
                    disabled={savingBudget}
                    className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:opacity-60"
                  >
                    {savingBudget ? t("common.saving") : t("settings.saveBudget")}
                  </button>
                  <span className="text-xs text-[var(--color-editorial-ink-faint)]">
                    {`${t("settings.currentMonth")}: ${settings.current_month.month_jst}`}
                  </span>
                </div>
              </form>
            </SectionCard>
          ) : null}

          {activeSection === "system" ? (
            <div className="space-y-5">
              <SectionCard>
                <div>
                  <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.access.selectProvider")}</div>
                  <div className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">
                    {`${t("settings.access.configuredProviders")}: ${configuredProviderCount}/${accessCards.length}`}
                  </div>
                </div>
                <div className="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                  {accessCards.map((card) => {
                    const selected = card.id === activeAccessCard?.id;
                    return (
                      <button
                        key={card.id}
                        type="button"
                        onClick={() => setActiveAccessProvider(card.id)}
                        className={joinClassNames(
                          "rounded-[18px] border px-4 py-3 text-left transition",
                          selected
                            ? "border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel-strong)] shadow-[var(--shadow-card)]"
                            : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] hover:bg-[var(--color-editorial-panel-strong)]"
                        )}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <div className="text-sm font-medium text-[var(--color-editorial-ink)]">{card.title.replace(/（.*?）|\(.*?\)/g, "").trim()}</div>
                          <span className={joinClassNames(
                            "rounded-full px-2 py-0.5 text-[11px] font-medium",
                            card.configured
                              ? "border border-[var(--color-editorial-success-line)] bg-[var(--color-editorial-success-soft)] text-[var(--color-editorial-success)]"
                              : "border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-faint)]"
                          )}>
                            {card.configured ? t("settings.configured") : t("settings.access.notConfiguredShort")}
                          </span>
                        </div>
                        <div className="mt-2 text-xs text-[var(--color-editorial-ink-soft)]">
                          {card.configured ? `••••${card.last4 ?? "****"}` : card.notSet}
                        </div>
                      </button>
                    );
                  })}
                </div>
              </SectionCard>
              {activeAccessCard ? (
                <ApiKeyCard
                  icon={KeyRound}
                  title={activeAccessCard.title}
                  description={activeAccessCard.description}
                  configured={activeAccessCard.configured}
                  last4={activeAccessCard.last4}
                  value={activeAccessCard.value}
                  onChange={activeAccessCard.onChange}
                  onSubmit={activeAccessCard.onSubmit}
                  onDelete={activeAccessCard.onDelete}
                  placeholder={activeAccessCard.placeholder}
                  saving={activeAccessCard.saving}
                  deleting={activeAccessCard.deleting}
                  labels={{ ...apiKeyCardLabels, notSet: activeAccessCard.notSet }}
                />
              ) : null}
            </div>
          ) : null}
        </div>
      </div>

      <AivisVoicePickerModal
        open={Boolean(aivisPickerPersona)}
        loading={aivisModelsLoading}
        syncing={aivisModelsSyncing}
        error={aivisModelsError}
        models={aivisModelsData?.models ?? []}
        currentVoiceModel={activeAivisVoice?.voice_model ?? ""}
        currentVoiceStyle={activeAivisVoice?.voice_style ?? ""}
        onClose={() => setAivisPickerPersona(null)}
        onSync={() => {
          void syncAivisModels();
        }}
        onSelect={(selection) => {
          if (!aivisPickerPersona) return;
          updateAudioBriefingVoice(aivisPickerPersona, {
            tts_provider: "aivis",
            voice_model: selection.voice_model,
            voice_style: selection.voice_style,
          });
        }}
      />

      <FishVoicePickerModal
        open={Boolean(fishPickerPersona)}
        currentVoiceModel={audioBriefingVoices.find((voice) => voice.persona === fishPickerPersona)?.voice_model ?? ""}
        onClose={() => setFishPickerPersona(null)}
        onSelect={(selection) => {
          if (!fishPickerPersona) return;
          updateAudioBriefingVoice(fishPickerPersona, {
            tts_provider: "fish",
            tts_model: audioBriefingVoices.find((voice) => voice.persona === fishPickerPersona)?.tts_model || "s2-pro",
            voice_model: selection.voice_model,
            voice_style: "",
            provider_voice_label: selection.provider_voice_label,
            provider_voice_description: selection.provider_voice_description,
          });
        }}
      />

      <XAIVoicePickerModal
        open={Boolean(xaiPickerPersona)}
        loading={xaiVoicesLoading}
        syncing={xaiVoicesSyncing}
        error={xaiVoicesError}
        voices={audioBriefingXAIVoices}
        currentVoiceID={activeXAIVoice?.voice_model ?? ""}
        onClose={() => setXAIPickerPersona(null)}
        onSync={() => {
          void syncXAIVoices();
        }}
        onSelect={(selection) => {
          if (!xaiPickerPersona) return;
          updateAudioBriefingVoice(xaiPickerPersona, {
            tts_provider: "xai",
            voice_model: selection.voice_id,
            voice_style: "",
          });
        }}
      />

      <OpenAITTSVoicePickerModal
        open={Boolean(openAITTPickerPersona)}
        loading={openAITTSVoicesLoading}
        syncing={openAITTSVoicesSyncing}
        error={openAITTSVoicesError}
        voices={audioBriefingOpenAITTSVoices}
        currentVoiceID={activeOpenAITTSVoice?.voice_model ?? ""}
        onClose={() => setOpenAITTPickerPersona(null)}
        onSync={() => {
          void syncOpenAITTSVoices().catch(() => undefined);
        }}
        onSelect={(selection) => {
          if (!openAITTPickerPersona) return;
          updateAudioBriefingVoice(openAITTPickerPersona, {
            tts_provider: "openai",
            tts_model: activeOpenAITTSVoice?.tts_model || "gpt-4o-mini-tts",
            voice_model: selection.voice_id,
            voice_style: "",
          });
        }}
      />

      <GeminiTTSVoicePickerModal
        open={Boolean(geminiTTSPickerPersona)}
        loading={geminiTTSVoicesLoading}
        error={geminiTTSVoicesError}
        voices={audioBriefingGeminiTTSVoices}
        currentVoiceName={activeGeminiTTSVoice?.voice_model ?? ""}
        onClose={() => setGeminiTTSPickerPersona(null)}
        onRefresh={() => {
          void loadGeminiTTSVoices().catch(() => undefined);
        }}
        onSelect={(selection) => {
          if (!geminiTTSPickerPersona) return;
          updateAudioBriefingVoice(geminiTTSPickerPersona, {
            tts_provider: "gemini_tts",
            tts_model: activeGeminiTTSVoice?.tts_model || "gemini-2.5-flash-tts",
            voice_model: selection.voice_name,
            voice_style: "",
          });
        }}
      />

      <AivisVoicePickerModal
        open={summaryAudioAivisPickerOpen}
        loading={aivisModelsLoading}
        syncing={aivisModelsSyncing}
        error={aivisModelsError}
        models={aivisModelsData?.models ?? []}
        currentVoiceModel={summaryAudioVoiceModel}
        currentVoiceStyle={summaryAudioVoiceStyle}
        onClose={() => setSummaryAudioAivisPickerOpen(false)}
        onSync={() => {
          void syncAivisModels();
        }}
        onSelect={(selection) => {
          setSummaryAudioProvider("aivis");
          setSummaryAudioVoiceModel(selection.voice_model);
          setSummaryAudioVoiceStyle(selection.voice_style);
          setSummaryAudioProviderVoiceLabel("");
          setSummaryAudioProviderVoiceDescription("");
        }}
      />

      <FishVoicePickerModal
        open={summaryAudioFishPickerOpen}
        currentVoiceModel={summaryAudioVoiceModel}
        onClose={() => setSummaryAudioFishPickerOpen(false)}
        onSelect={(selection) => {
          setSummaryAudioProvider("fish");
          setSummaryAudioTTSModel(summaryAudioTTSModel.trim() || "s2-pro");
          setSummaryAudioVoiceModel(selection.voice_model);
          setSummaryAudioVoiceStyle("");
          setSummaryAudioProviderVoiceLabel(selection.provider_voice_label);
          setSummaryAudioProviderVoiceDescription(selection.provider_voice_description);
        }}
      />

      <XAIVoicePickerModal
        open={summaryAudioXAIPickerOpen}
        loading={xaiVoicesLoading}
        syncing={xaiVoicesSyncing}
        error={xaiVoicesError}
        voices={summaryAudioXAIVoices}
        currentVoiceID={summaryAudioVoiceModel}
        onClose={() => setSummaryAudioXAIPickerOpen(false)}
        onSync={() => {
          void syncXAIVoices();
        }}
        onSelect={(selection) => {
          setSummaryAudioProvider("xai");
          setSummaryAudioVoiceModel(selection.voice_id);
          setSummaryAudioVoiceStyle("");
          setSummaryAudioProviderVoiceLabel("");
          setSummaryAudioProviderVoiceDescription("");
        }}
      />

      <OpenAITTSVoicePickerModal
        open={summaryAudioOpenAITTPickerOpen}
        loading={openAITTSVoicesLoading}
        syncing={openAITTSVoicesSyncing}
        error={openAITTSVoicesError}
        voices={summaryAudioOpenAITTSVoices}
        currentVoiceID={summaryAudioVoiceModel}
        onClose={() => setSummaryAudioOpenAITTPickerOpen(false)}
        onSync={() => {
          void syncOpenAITTSVoices().catch(() => undefined);
        }}
        onSelect={(selection) => {
          setSummaryAudioProvider("openai");
          setSummaryAudioTTSModel(summaryAudioTTSModel.trim() || "tts-1");
          setSummaryAudioVoiceModel(selection.voice_id);
          setSummaryAudioVoiceStyle("");
          setSummaryAudioProviderVoiceLabel("");
          setSummaryAudioProviderVoiceDescription("");
        }}
      />

      <GeminiTTSVoicePickerModal
        open={summaryAudioGeminiTTSPickerOpen}
        loading={geminiTTSVoicesLoading}
        error={geminiTTSVoicesError}
        voices={summaryAudioGeminiTTSVoices}
        currentVoiceName={summaryAudioVoiceModel}
        onClose={() => setSummaryAudioGeminiTTSPickerOpen(false)}
        onRefresh={() => {
          void loadGeminiTTSVoices().catch(() => undefined);
        }}
        onSelect={(selection) => {
          setSummaryAudioProvider("gemini_tts");
          setSummaryAudioTTSModel(summaryAudioTTSModel.trim() || "gemini-2.5-flash-tts");
          setSummaryAudioVoiceModel(selection.voice_name);
          setSummaryAudioVoiceStyle("");
          setSummaryAudioProviderVoiceLabel("");
          setSummaryAudioProviderVoiceDescription("");
        }}
      />

      <AudioBriefingPresetSaveModal
        open={audioBriefingPresetSaveOpen}
        saving={audioBriefingPresetSaving}
        presetName={audioBriefingPresetName}
        defaultPersonaMode={audioBriefingDefaultPersonaMode}
        defaultPersona={audioBriefingDefaultPersona}
        conversationMode={audioBriefingConversationMode}
        voices={audioBriefingVoices}
        onClose={() => {
          setAudioBriefingPresetSaveOpen(false);
          setAudioBriefingPresetName("");
        }}
        onChangeName={setAudioBriefingPresetName}
        onSave={() => {
          void submitAudioBriefingPresetSave();
        }}
      />

      <AudioBriefingPresetApplyModal
        open={audioBriefingPresetApplyOpen}
        loading={audioBriefingPresetsLoading}
        error={audioBriefingPresetsError}
        presets={audioBriefingPresets}
        selectedPresetID={audioBriefingPresetApplySelection}
        onClose={() => {
          setAudioBriefingPresetApplyOpen(false);
          setAudioBriefingPresetApplySelection(null);
        }}
        onRefresh={() => {
          void loadAudioBriefingPresets();
        }}
        onSelectPreset={setAudioBriefingPresetApplySelection}
        onApplyPreset={applyAudioBriefingPreset}
      />

      <ModelGuideModal
        open={modelGuideOpen}
        onClose={() => setModelGuideOpen(false)}
        entries={modelComparisonEntries}
        t={t}
      />
    </div>
  );
}
