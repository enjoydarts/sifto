"use client";

import type {
  ElevenLabsVoiceCatalogEntry,
  GeminiTTSVoiceCatalogEntry,
  OpenAITTSVoiceSnapshot,
  XAIVoiceSnapshot,
} from "@/lib/api";
import {
  formatAivisVoiceStyleLabel,
  resolveAivisVoiceSelection,
} from "@/components/settings/providers/tts-provider-readiness";

type Translate = (key: string, fallback?: string) => string;

type ResolveTTSVoiceDisplayArgs = {
  provider: string;
  voiceModel: string;
  voiceStyle: string;
  providerVoiceLabel: string;
  providerVoiceDescription: string;
  unsetText: string;
  t: Translate;
  aivisResolved?: ReturnType<typeof resolveAivisVoiceSelection> | null;
  xaiResolved?: XAIVoiceSnapshot | null;
  openAIResolved?: OpenAITTSVoiceSnapshot | null;
  geminiResolved?: GeminiTTSVoiceCatalogEntry | null;
  elevenLabsResolved?: ElevenLabsVoiceCatalogEntry | null;
};

export function resolveTTSVoiceDisplay({
  provider,
  voiceModel,
  voiceStyle,
  providerVoiceLabel,
  providerVoiceDescription,
  unsetText,
  t,
  aivisResolved,
  xaiResolved,
  openAIResolved,
  geminiResolved,
  elevenLabsResolved,
}: ResolveTTSVoiceDisplayArgs): { label: string; detail: string } {
  const normalizedProvider = provider.trim().toLowerCase();
  if (normalizedProvider == "aivis") {
    return {
      label: aivisResolved?.model?.name || voiceModel || unsetText,
      detail: aivisResolved?.speaker && aivisResolved?.style
        ? `${aivisResolved.speaker.name} / ${aivisResolved.style.name}`
        : formatAivisVoiceStyleLabel(voiceStyle || voiceModel, t),
    };
  }
  if (normalizedProvider == "fish") {
    return {
      label: providerVoiceLabel || voiceModel || unsetText,
      detail: providerVoiceDescription || voiceModel || unsetText,
    };
  }
  if (normalizedProvider == "xai") {
    return {
      label: xaiResolved?.name || voiceModel || unsetText,
      detail: xaiResolved?.description || voiceModel || unsetText,
    };
  }
  if (normalizedProvider == "openai") {
    return {
      label: openAIResolved?.name || voiceModel || unsetText,
      detail: openAIResolved?.description || voiceModel || unsetText,
    };
  }
  if (normalizedProvider == "gemini_tts") {
    return {
      label: geminiResolved?.label || voiceModel || unsetText,
      detail: geminiResolved?.description || voiceModel || unsetText,
    };
  }
  if (normalizedProvider == "elevenlabs") {
    return {
      label: elevenLabsResolved?.name || providerVoiceLabel || voiceModel || unsetText,
      detail: elevenLabsResolved?.description || providerVoiceDescription || voiceModel || unsetText,
    };
  }
  return {
    label: voiceModel || unsetText,
    detail: voiceStyle || voiceModel || unsetText,
  };
}
