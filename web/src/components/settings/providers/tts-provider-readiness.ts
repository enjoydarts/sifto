"use client";

import type {
  AivisModelSnapshot,
  AudioBriefingPersonaVoice,
  ElevenLabsVoiceCatalogEntry,
  FishModelSnapshot,
  GeminiTTSVoiceCatalogEntry,
  OpenAITTSVoiceSnapshot,
  SummaryAudioVoiceSettings,
  XAIVoiceSnapshot,
} from "@/lib/api";
import { getAudioBriefingProviderCapabilities, type TTSProviderCapabilities } from "@/components/settings/providers/tts-provider-metadata";

type Translate = (key: string, fallback?: string) => string;

export type VoiceModelSelection = {
  voice_model: string;
  provider_voice_label?: string;
  provider_voice_description?: string;
};

export type VoiceStyleSelection = VoiceModelSelection & {
  voice_style: string;
};

export type VoiceStatus = {
  tone: "ok" | "warn" | "muted";
  label: string;
  detail: string;
  configured: boolean;
};

export function resolveAivisVoiceSelection(models: AivisModelSnapshot[], voice: VoiceStyleSelection) {
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

export function formatAivisVoiceStyleLabel(voiceStyle: string, t: Translate): string {
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

export function resolveXAIVoiceSelection(voices: XAIVoiceSnapshot[], voice: VoiceModelSelection) {
  return voices.find((item) => item.voice_id === voice.voice_model) ?? null;
}

export function resolveFishVoiceSelection(models: FishModelSnapshot[], voice: VoiceModelSelection) {
  return models.find((item) => item._id === voice.voice_model) ?? null;
}

export function resolveOpenAITTSVoiceSelection(voices: OpenAITTSVoiceSnapshot[], voice: VoiceModelSelection) {
  return voices.find((item) => item.voice_id === voice.voice_model) ?? null;
}

export function resolveGeminiTTSVoiceSelection(voices: GeminiTTSVoiceCatalogEntry[], voice: VoiceModelSelection) {
  return voices.find((item) => item.voice_name === voice.voice_model) ?? null;
}

export function resolveElevenLabsVoiceSelection(voices: ElevenLabsVoiceCatalogEntry[], voice: VoiceModelSelection) {
  return voices.find((item) => item.voice_id === voice.voice_model) ?? null;
}

export function isAudioBriefingVoiceConfigured(voice: AudioBriefingPersonaVoice) {
  const capabilities = getAudioBriefingProviderCapabilities(voice.tts_provider);
  if (!voice.voice_model.trim()) return false;
  if (capabilities.requiresVoiceStyle) return voice.voice_style.trim().length > 0;
  return true;
}

export function getAudioBriefingVoiceStatus(
  voice: AudioBriefingPersonaVoice,
  models: AivisModelSnapshot[],
  fishModels: FishModelSnapshot[],
  xaiVoices: XAIVoiceSnapshot[],
  openAIVoices: OpenAITTSVoiceSnapshot[],
  geminiVoices: GeminiTTSVoiceCatalogEntry[],
  elevenLabsVoices: ElevenLabsVoiceCatalogEntry[],
  hasAivisAPIKey: boolean,
  hasFishAPIKey: boolean,
  hasXAIAPIKey: boolean,
  hasOpenAIAPIKey: boolean,
  hasElevenLabsAPIKey: boolean,
  geminiTTSEnabled: boolean,
  conversationMode: "single" | "duo",
  t: Translate,
): VoiceStatus {
  const provider = voice.tts_provider.trim().toLowerCase();
  if (!isAudioBriefingVoiceConfigured(voice)) {
    return {
      tone: "warn",
      label: t("settings.audioBriefing.status.unconfigured"),
      detail: t("settings.audioBriefing.status.unconfiguredDetail"),
      configured: false,
    };
  }
  if (provider === "openai") {
    const resolved = resolveOpenAITTSVoiceSelection(openAIVoices, voice);
    if (!voice.tts_model.trim()) {
      return { tone: "warn", label: t("settings.audioBriefing.status.openaiModelMissing"), detail: t("settings.audioBriefing.status.openaiModelMissingDetail"), configured: true };
    }
    if (!hasOpenAIAPIKey) {
      return { tone: "warn", label: t("settings.audioBriefing.status.openaiApiKeyMissing"), detail: t("settings.audioBriefing.status.openaiApiKeyMissingDetail"), configured: true };
    }
    if (!resolved) {
      return { tone: "warn", label: t("settings.audioBriefing.status.openaiVoiceMissing"), detail: t("settings.audioBriefing.status.openaiVoiceMissingDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.audioBriefing.status.openaiReady"), detail: t("settings.audioBriefing.status.openaiReadyDetail"), configured: true };
  }
  if (provider === "gemini_tts") {
    const resolved = resolveGeminiTTSVoiceSelection(geminiVoices, voice);
    if (!voice.tts_model.trim()) {
      return { tone: "warn", label: t("settings.audioBriefing.status.geminiModelMissing"), detail: t("settings.audioBriefing.status.geminiModelMissingDetail"), configured: true };
    }
    if (!geminiTTSEnabled) {
      return { tone: "warn", label: t("settings.audioBriefing.status.geminiNotAllowed"), detail: t("settings.audioBriefing.status.geminiNotAllowedDetail"), configured: true };
    }
    if (!resolved) {
      return { tone: "warn", label: t("settings.audioBriefing.status.geminiVoiceMissing"), detail: t("settings.audioBriefing.status.geminiVoiceMissingDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.audioBriefing.status.geminiReady"), detail: t("settings.audioBriefing.status.geminiReadyDetail"), configured: true };
  }
  if (provider === "xai") {
    const resolved = resolveXAIVoiceSelection(xaiVoices, voice);
    if (!hasXAIAPIKey) {
      return { tone: "warn", label: t("settings.audioBriefing.status.xaiApiKeyMissing"), detail: t("settings.audioBriefing.status.xaiApiKeyMissingDetail"), configured: true };
    }
    if (!resolved) {
      return { tone: "warn", label: t("settings.audioBriefing.status.xaiVoiceMissing"), detail: t("settings.audioBriefing.status.xaiVoiceMissingDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.audioBriefing.status.xaiReady"), detail: t("settings.audioBriefing.status.xaiReadyDetail"), configured: true };
  }
  if (provider === "fish") {
    if (!voice.tts_model.trim()) {
      return { tone: "warn", label: t("settings.audioBriefing.status.fishModelMissing"), detail: t("settings.audioBriefing.status.fishModelMissingDetail"), configured: true };
    }
    if (!hasFishAPIKey) {
      return { tone: "warn", label: t("settings.audioBriefing.status.fishApiKeyMissing"), detail: t("settings.audioBriefing.status.fishApiKeyMissingDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.audioBriefing.status.fishReady"), detail: t("settings.audioBriefing.status.fishReadyDetail"), configured: true };
  }
  if (provider === "elevenlabs") {
    const resolved = resolveElevenLabsVoiceSelection(elevenLabsVoices, voice);
    if (!voice.tts_model.trim()) {
      return { tone: "warn", label: t("settings.audioBriefing.status.elevenlabsModelMissing"), detail: t("settings.audioBriefing.status.elevenlabsModelMissingDetail"), configured: true };
    }
    if (!hasElevenLabsAPIKey) {
      return { tone: "warn", label: t("settings.audioBriefing.status.elevenlabsApiKeyMissing"), detail: t("settings.audioBriefing.status.elevenlabsApiKeyMissingDetail"), configured: true };
    }
    if (!voice.voice_model.trim()) {
      return { tone: "warn", label: t("settings.audioBriefing.status.elevenlabsVoiceMissing"), detail: t("settings.audioBriefing.status.elevenlabsVoiceMissingDetail"), configured: true };
    }
    if (conversationMode === "duo" && voice.tts_model.trim() !== "eleven_v3") {
      return { tone: "warn", label: t("settings.audioBriefing.status.elevenlabsDuoModelWarning"), detail: t("settings.audioBriefing.status.elevenlabsDuoModelWarningDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.audioBriefing.status.elevenlabsReady"), detail: resolved?.description || t("settings.audioBriefing.status.elevenlabsReadyDetail"), configured: true };
  }
  if (provider !== "aivis") {
    return { tone: "muted", label: t("settings.audioBriefing.status.customProvider"), detail: t("settings.audioBriefing.status.customProviderDetail").replace("{{provider}}", voice.tts_provider), configured: true };
  }
  const resolved = resolveAivisVoiceSelection(models, voice);
  if (!resolved.model) {
    return { tone: "warn", label: t("settings.audioBriefing.status.modelMissing"), detail: t("settings.audioBriefing.status.modelMissingDetail"), configured: true };
  }
  if (!resolved.speaker || !resolved.style) {
    return { tone: "warn", label: t("settings.audioBriefing.status.styleMissing"), detail: t("settings.audioBriefing.status.styleMissingDetail"), configured: true };
  }
  if (!hasAivisAPIKey) {
    return { tone: "warn", label: t("settings.audioBriefing.status.apiKeyMissing"), detail: t("settings.audioBriefing.status.apiKeyMissingDetail"), configured: true };
  }
  return { tone: "ok", label: t("settings.audioBriefing.status.ready"), detail: t("settings.audioBriefing.status.readyDetail"), configured: true };
}

export function isSummaryAudioVoiceConfigured(voice: SummaryAudioVoiceSettings) {
  const capabilities = getAudioBriefingProviderCapabilities(voice.tts_provider);
  if (!voice.voice_model.trim()) return false;
  if (capabilities.requiresVoiceStyle && !voice.voice_style.trim()) return false;
  return true;
}

export function getSummaryAudioVoiceStatus(
  voice: SummaryAudioVoiceSettings,
  models: AivisModelSnapshot[],
  fishModels: FishModelSnapshot[],
  xaiVoices: XAIVoiceSnapshot[],
  openAIVoices: OpenAITTSVoiceSnapshot[],
  geminiVoices: GeminiTTSVoiceCatalogEntry[],
  elevenLabsVoices: ElevenLabsVoiceCatalogEntry[],
  hasAivisAPIKey: boolean,
  hasFishAPIKey: boolean,
  hasXAIAPIKey: boolean,
  hasOpenAIAPIKey: boolean,
  hasElevenLabsAPIKey: boolean,
  geminiTTSEnabled: boolean,
  t: Translate,
): VoiceStatus {
  const provider = voice.tts_provider.trim().toLowerCase();
  if (!isSummaryAudioVoiceConfigured(voice)) {
    return { tone: "warn", label: t("settings.summaryAudio.status.unconfigured"), detail: t("settings.summaryAudio.status.unconfiguredDetail"), configured: false };
  }
  if (provider === "openai") {
    const resolved = resolveOpenAITTSVoiceSelection(openAIVoices, voice);
    if (!voice.tts_model.trim()) {
      return { tone: "warn", label: t("settings.summaryAudio.status.openaiModelMissing"), detail: t("settings.summaryAudio.status.openaiModelMissingDetail"), configured: true };
    }
    if (!hasOpenAIAPIKey) {
      return { tone: "warn", label: t("settings.summaryAudio.status.openaiApiKeyMissing"), detail: t("settings.summaryAudio.status.openaiApiKeyMissingDetail"), configured: true };
    }
    if (!resolved) {
      return { tone: "warn", label: t("settings.summaryAudio.status.openaiVoiceMissing"), detail: t("settings.summaryAudio.status.openaiVoiceMissingDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.summaryAudio.status.openaiReady"), detail: t("settings.summaryAudio.status.openaiReadyDetail"), configured: true };
  }
  if (provider === "gemini_tts") {
    const resolved = resolveGeminiTTSVoiceSelection(geminiVoices, voice);
    if (!voice.tts_model.trim()) {
      return { tone: "warn", label: t("settings.summaryAudio.status.geminiModelMissing"), detail: t("settings.summaryAudio.status.geminiModelMissingDetail"), configured: true };
    }
    if (!geminiTTSEnabled) {
      return { tone: "warn", label: t("settings.summaryAudio.status.geminiNotAllowed"), detail: t("settings.summaryAudio.status.geminiNotAllowedDetail"), configured: true };
    }
    if (!resolved) {
      return { tone: "warn", label: t("settings.summaryAudio.status.geminiVoiceMissing"), detail: t("settings.summaryAudio.status.geminiVoiceMissingDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.summaryAudio.status.geminiReady"), detail: t("settings.summaryAudio.status.geminiReadyDetail"), configured: true };
  }
  if (provider === "xai") {
    const resolved = resolveXAIVoiceSelection(xaiVoices, voice);
    if (!hasXAIAPIKey) {
      return { tone: "warn", label: t("settings.summaryAudio.status.xaiApiKeyMissing"), detail: t("settings.summaryAudio.status.xaiApiKeyMissingDetail"), configured: true };
    }
    if (!resolved) {
      return { tone: "warn", label: t("settings.summaryAudio.status.xaiVoiceMissing"), detail: t("settings.summaryAudio.status.xaiVoiceMissingDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.summaryAudio.status.xaiReady"), detail: t("settings.summaryAudio.status.xaiReadyDetail"), configured: true };
  }
  if (provider === "fish") {
    if (!voice.tts_model.trim()) {
      return { tone: "warn", label: t("settings.summaryAudio.status.fishModelMissing"), detail: t("settings.summaryAudio.status.fishModelMissingDetail"), configured: true };
    }
    if (!hasFishAPIKey) {
      return { tone: "warn", label: t("settings.summaryAudio.status.fishApiKeyMissing"), detail: t("settings.summaryAudio.status.fishApiKeyMissingDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.summaryAudio.status.fishReady"), detail: t("settings.summaryAudio.status.fishReadyDetail"), configured: true };
  }
  if (provider === "elevenlabs") {
    const resolved = resolveElevenLabsVoiceSelection(elevenLabsVoices, voice);
    if (!voice.tts_model.trim()) {
      return { tone: "warn", label: t("settings.summaryAudio.status.elevenlabsModelMissing"), detail: t("settings.summaryAudio.status.elevenlabsModelMissingDetail"), configured: true };
    }
    if (!hasElevenLabsAPIKey) {
      return { tone: "warn", label: t("settings.summaryAudio.status.elevenlabsApiKeyMissing"), detail: t("settings.summaryAudio.status.elevenlabsApiKeyMissingDetail"), configured: true };
    }
    if (!voice.voice_model.trim()) {
      return { tone: "warn", label: t("settings.summaryAudio.status.elevenlabsVoiceMissing"), detail: t("settings.summaryAudio.status.elevenlabsVoiceMissingDetail"), configured: false };
    }
    if (!resolved) {
      return { tone: "warn", label: t("settings.summaryAudio.status.elevenlabsVoiceMissing"), detail: t("settings.summaryAudio.status.elevenlabsVoiceMissingDetail"), configured: true };
    }
    return { tone: "ok", label: t("settings.summaryAudio.status.elevenlabsReady"), detail: resolved.description || t("settings.summaryAudio.status.elevenlabsReadyDetail"), configured: true };
  }
  if (provider !== "aivis") {
    return { tone: "muted", label: t("settings.summaryAudio.status.customProvider"), detail: t("settings.summaryAudio.status.customProviderDetail").replace("{{provider}}", voice.tts_provider), configured: true };
  }
  const resolved = resolveAivisVoiceSelection(models, voice);
  if (!resolved.model) {
    return { tone: "warn", label: t("settings.summaryAudio.status.modelMissing"), detail: t("settings.summaryAudio.status.modelMissingDetail"), configured: true };
  }
  if (!resolved.speaker || !resolved.style) {
    return { tone: "warn", label: t("settings.summaryAudio.status.styleMissing"), detail: t("settings.summaryAudio.status.styleMissingDetail"), configured: true };
  }
  if (!hasAivisAPIKey) {
    return { tone: "warn", label: t("settings.summaryAudio.status.apiKeyMissing"), detail: t("settings.summaryAudio.status.apiKeyMissingDetail"), configured: true };
  }
  if (!voice.aivis_user_dictionary_uuid?.trim()) {
    return { tone: "warn", label: t("settings.summaryAudio.status.aivisDictionaryMissing"), detail: t("settings.summaryAudio.status.aivisDictionaryMissingDetail"), configured: true };
  }
  return { tone: "ok", label: t("settings.summaryAudio.status.ready"), detail: t("settings.summaryAudio.status.readyDetail"), configured: true };
}

export { getAudioBriefingProviderCapabilities, type TTSProviderCapabilities };
