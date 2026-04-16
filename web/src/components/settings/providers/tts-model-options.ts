"use client";

import type { ModelOption } from "@/components/settings/model-select";

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
    value: "gemini-3.1-flash-tts-preview",
    label: "gemini-3.1-flash-tts-preview",
    selectedLabel: "Gemini TTS / gemini-3.1-flash-tts-preview",
    note: "Latest preview Gemini speech generation model",
    provider: "Gemini TTS",
  },
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

const ELEVENLABS_TTS_MODEL_OPTIONS: ModelOption[] = [
  {
    value: "eleven_flash_v2_5",
    label: "eleven_flash_v2_5",
    selectedLabel: "ElevenLabs / eleven_flash_v2_5",
    note: "Lowest latency ElevenLabs model",
    provider: "ElevenLabs",
  },
  {
    value: "eleven_turbo_v2_5",
    label: "eleven_turbo_v2_5",
    selectedLabel: "ElevenLabs / eleven_turbo_v2_5",
    note: "Balanced speed and quality",
    provider: "ElevenLabs",
  },
  {
    value: "eleven_multilingual_v2",
    label: "eleven_multilingual_v2",
    selectedLabel: "ElevenLabs / eleven_multilingual_v2",
    note: "Multilingual speech model",
    provider: "ElevenLabs",
  },
  {
    value: "eleven_v3",
    label: "eleven_v3",
    selectedLabel: "ElevenLabs / eleven_v3",
    note: "Required for dialogue synthesis",
    provider: "ElevenLabs",
  },
];

function withCurrentModel(currentValue: string, provider: string, options: ModelOption[]): ModelOption[] {
  const trimmed = currentValue.trim();
  if (!trimmed || options.some((option) => option.value === trimmed)) {
    return options;
  }
  return [
    {
      value: trimmed,
      label: trimmed,
      selectedLabel: `${provider} / ${trimmed}`,
      provider,
    },
    ...options,
  ];
}

export function buildOpenAITTSModelOptions(currentValue: string): ModelOption[] {
  return withCurrentModel(currentValue, "OpenAI TTS", OPENAI_TTS_MODEL_OPTIONS);
}

export function buildGeminiTTSModelOptions(currentValue: string): ModelOption[] {
  return withCurrentModel(currentValue, "Gemini TTS", GEMINI_TTS_MODEL_OPTIONS);
}

export function buildFishTTSModelOptions(currentValue: string): ModelOption[] {
  return withCurrentModel(currentValue, "Fish Audio", FISH_TTS_MODEL_OPTIONS);
}

export function buildElevenLabsTTSModelOptions(
  currentValue: string,
  conversationMode: "single" | "duo" = "single",
): ModelOption[] {
  if (conversationMode === "duo") {
    return ELEVENLABS_TTS_MODEL_OPTIONS.filter((option) => option.value === "eleven_v3");
  }
  return withCurrentModel(currentValue, "ElevenLabs", ELEVENLABS_TTS_MODEL_OPTIONS);
}
