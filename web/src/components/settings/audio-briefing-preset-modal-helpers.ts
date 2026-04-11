"use client";

import type { AudioBriefingPersonaVoice } from "@/lib/api";

const NAVIGATOR_PERSONA_KEYS = ["editor", "hype", "analyst", "concierge", "snark", "native", "junior", "urban"] as const;

function buildDefaultAudioBriefingVoices(): AudioBriefingPersonaVoice[] {
  return NAVIGATOR_PERSONA_KEYS.map((persona) => ({
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

export function normalizeAudioBriefingPresetVoices(voices: AudioBriefingPersonaVoice[]): AudioBriefingPersonaVoice[] {
  const defaults = buildDefaultAudioBriefingVoices();
  const byPersona = new Map(voices.map((voice) => [voice.persona, voice] as const));
  return defaults.map((voice) => byPersona.get(voice.persona) ?? voice);
}

export function formatAudioBriefingPresetVoiceLabel(
  voice: AudioBriefingPersonaVoice,
  t: (key: string, fallback?: string) => string,
): string {
  const provider = voice.tts_provider.trim();
  const primary = voice.provider_voice_label?.trim() || voice.voice_model.trim();
  if (!primary) return t("settings.audioBriefing.unsetShort");
  return provider ? `${provider} / ${primary}` : primary;
}

export function formatAudioBriefingPresetVoiceDetail(
  voice: AudioBriefingPersonaVoice,
  t: (key: string, fallback?: string) => string,
): string {
  return (
    voice.provider_voice_description?.trim() ||
    voice.voice_style.trim() ||
    voice.voice_model.trim() ||
    t("settings.audioBriefing.unsetShort")
  );
}
