"use client";

import type { AudioBriefingPersonaVoice } from "@/lib/api";
import type { VoiceStatus } from "@/components/settings/providers/tts-provider-readiness";

export type Translate = (key: string, fallback?: string) => string;

export type ModelSelectLabels = {
  defaultOption: string;
  searchPlaceholder: string;
  noResults: string;
  providerAll: string;
  modalChoose: string;
  close: string;
  confirmTitle: string;
  confirmYes: string;
  confirmNo: string;
  confirmSuffix: string;
  providerColumn: string;
  modelColumn: string;
  pricingColumn: string;
};

export type AudioBriefingNumericInputField =
  | "speech_rate"
  | "tempo_dynamics"
  | "emotional_intensity"
  | "line_break_silence_seconds"
  | "aivis_volume"
  | "pitch"
  | "volume_gain";

export type AudioBriefingVoiceInputDrafts = Record<string, Record<AudioBriefingNumericInputField, string>>;

export type AudioBriefingVoiceSummary = {
  voice: AudioBriefingPersonaVoice;
  status: VoiceStatus;
};
