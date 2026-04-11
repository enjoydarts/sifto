"use client";

import {
  type AudioBriefingPersonaVoice,
  type NavigatorPersonaDefinition,
  type SaveAudioBriefingPresetRequest,
  type SummaryAudioVoiceSettings,
  type UserSettings,
} from "@/lib/api";
import { type SettingsSectionID } from "@/components/settings/settings-page-shell";

export function tWithVars(
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

export function localizePreferenceProfileErrorMessage(raw: unknown, t: (key: string, fallback?: string) => string): string {
  const message = String(raw instanceof Error ? raw.message : raw).replace(/^Error:\s*/, "").trim();
  if (message.startsWith("401:")) return t("settings.personalization.error.auth");
  if (message.startsWith("403:")) return t("settings.personalization.error.auth");
  if (message.startsWith("429:")) return t("settings.personalization.error.rateLimited");
  if (message.startsWith("500:")) return t("settings.personalization.error.server");
  if (!message) return t("settings.personalization.error.unknown");
  return t("settings.personalization.error.detail").replace("{{message}}", message);
}

export function buildPodcastRSSURL(feedSlug: string | null | undefined, fallbackURL?: string | null): string {
  const slug = (feedSlug ?? "").trim();
  if (!slug) {
    return fallbackURL ?? "";
  }
  if (typeof window === "undefined") {
    return fallbackURL ?? "";
  }
  return `${window.location.origin}/podcasts/${encodeURIComponent(slug)}/feed.xml`;
}

export type NavigatorPersonaKey = "editor" | "hype" | "analyst" | "concierge" | "snark" | "native" | "junior" | "urban";

export type AudioBriefingScheduleSelection = "interval3h" | "interval6h" | "fixed3x";

export type AudioBriefingNumericInputField =
  | "speech_rate"
  | "tempo_dynamics"
  | "emotional_intensity"
  | "line_break_silence_seconds"
  | "aivis_volume"
  | "pitch"
  | "volume_gain";

export type AudioBriefingVoiceInputDrafts = Record<string, Record<AudioBriefingNumericInputField, string>>;

export type SummaryAudioNumericInputField =
  | "speech_rate"
  | "tempo_dynamics"
  | "emotional_intensity"
  | "line_break_silence_seconds"
  | "pitch"
  | "volume_gain";

export type SummaryAudioVoiceInputDrafts = Record<SummaryAudioNumericInputField, string>;

export const NAVIGATOR_PERSONA_KEYS: NavigatorPersonaKey[] = ["editor", "hype", "analyst", "concierge", "snark", "native", "junior", "urban"];

export const EMPTY_NAVIGATOR_PERSONA: NavigatorPersonaDefinition = {
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

export function buildDefaultAudioBriefingVoices(personaKeys: NavigatorPersonaKey[]): AudioBriefingPersonaVoice[] {
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

export function formatAudioBriefingDecimalInput(value: number): string {
  if (!Number.isFinite(value)) return "";
  return value.toFixed(4).replace(/\.?0+$/, "");
}

export function resolveAudioBriefingScheduleSelection(
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

export function formatAudioBriefingScheduleSelection(
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

export function isCompleteDecimalInput(raw: string): boolean {
  return /^-?(?:\d+\.?\d*|\.\d+)$/.test(raw);
}

export function isSettingsSectionID(section: string | null): section is SettingsSectionID {
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

export function buildAudioBriefingPresetRequest(
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

export function buildAudioBriefingVoiceInputDrafts(voices: AudioBriefingPersonaVoice[]): AudioBriefingVoiceInputDrafts {
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

export function buildDefaultSummaryAudioVoiceSettings(): SummaryAudioVoiceSettings {
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

export function buildSummaryAudioVoiceInputDrafts(settings: SummaryAudioVoiceSettings): SummaryAudioVoiceInputDrafts {
  return {
    speech_rate: formatAudioBriefingDecimalInput(settings.speech_rate),
    tempo_dynamics: formatAudioBriefingDecimalInput(settings.tempo_dynamics),
    emotional_intensity: formatAudioBriefingDecimalInput(settings.emotional_intensity),
    line_break_silence_seconds: formatAudioBriefingDecimalInput(settings.line_break_silence_seconds),
    pitch: formatAudioBriefingDecimalInput(settings.pitch),
    volume_gain: formatAudioBriefingDecimalInput(settings.volume_gain),
  };
}
