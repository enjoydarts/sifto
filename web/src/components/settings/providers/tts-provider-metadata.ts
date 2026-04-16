"use client";

export type TTSProviderCapabilities = {
  requiresVoiceStyle: boolean;
  supportsCatalogPicker: boolean;
  supportsSeparateTTSModel: boolean;
  supportsSpeechTuning: boolean;
  requiresRegion: boolean;
};

export type TTSProviderMetadata = {
  capabilities: TTSProviderCapabilities;
  defaultTTSModel: string;
};

const DEFAULT_TTS_PROVIDER_CAPABILITIES: TTSProviderCapabilities = {
  requiresVoiceStyle: true,
  supportsCatalogPicker: false,
  supportsSeparateTTSModel: false,
  supportsSpeechTuning: false,
  requiresRegion: false,
};

export const TTS_PROVIDER_METADATA: Record<string, TTSProviderMetadata> = {
  aivis: {
    capabilities: {
      requiresVoiceStyle: true,
      supportsCatalogPicker: true,
      supportsSeparateTTSModel: false,
      supportsSpeechTuning: true,
      requiresRegion: false,
    },
    defaultTTSModel: "",
  },
  elevenlabs: {
    capabilities: {
      requiresVoiceStyle: false,
      supportsCatalogPicker: true,
      supportsSeparateTTSModel: true,
      supportsSpeechTuning: false,
      requiresRegion: false,
    },
    defaultTTSModel: "eleven_flash_v2_5",
  },
  xai: {
    capabilities: {
      requiresVoiceStyle: false,
      supportsCatalogPicker: true,
      supportsSeparateTTSModel: false,
      supportsSpeechTuning: false,
      requiresRegion: false,
    },
    defaultTTSModel: "",
  },
  azure_speech: {
    capabilities: {
      requiresVoiceStyle: false,
      supportsCatalogPicker: true,
      supportsSeparateTTSModel: false,
      supportsSpeechTuning: false,
      requiresRegion: true,
    },
    defaultTTSModel: "",
  },
  fish: {
    capabilities: {
      requiresVoiceStyle: false,
      supportsCatalogPicker: true,
      supportsSeparateTTSModel: true,
      supportsSpeechTuning: false,
      requiresRegion: false,
    },
    defaultTTSModel: "s2-pro",
  },
  openai: {
    capabilities: {
      requiresVoiceStyle: false,
      supportsCatalogPicker: true,
      supportsSeparateTTSModel: true,
      supportsSpeechTuning: false,
      requiresRegion: false,
    },
    defaultTTSModel: "gpt-4o-mini-tts",
  },
  gemini_tts: {
    capabilities: {
      requiresVoiceStyle: false,
      supportsCatalogPicker: true,
      supportsSeparateTTSModel: true,
      supportsSpeechTuning: false,
      requiresRegion: false,
    },
    defaultTTSModel: "gemini-3.1-flash-tts-preview",
  },
  mock: {
    capabilities: {
      requiresVoiceStyle: false,
      supportsCatalogPicker: false,
      supportsSeparateTTSModel: false,
      supportsSpeechTuning: false,
      requiresRegion: false,
    },
    defaultTTSModel: "",
  },
};

type Translate = (key: string, fallback?: string) => string;

export function formatTTSProviderLabel(provider: string, t: Translate): string {
  switch (provider.trim().toLowerCase()) {
    case "aivis":
      return "Aivis";
    case "fish":
      return t("settings.summaryAudio.provider.fish");
    case "xai":
      return t("settings.summaryAudio.provider.xai");
    case "azure_speech":
      return t("settings.summaryAudio.provider.azure_speech");
    case "openai":
      return t("settings.summaryAudio.provider.openai");
    case "gemini_tts":
      return t("settings.summaryAudio.provider.gemini_tts");
    case "elevenlabs":
      return t("settings.summaryAudio.provider.elevenlabs");
    case "mock":
      return "Mock";
    default:
      return provider;
  }
}

export function getAudioBriefingProviderCapabilities(provider: string): TTSProviderCapabilities {
  return TTS_PROVIDER_METADATA[provider.trim().toLowerCase()]?.capabilities ?? DEFAULT_TTS_PROVIDER_CAPABILITIES;
}

export function getTTSProviderDefaultModel(provider: string): string {
  const normalized = provider.trim().toLowerCase();
  const metadata = TTS_PROVIDER_METADATA[normalized];
  if (!metadata) return "";
  return metadata.capabilities.supportsSeparateTTSModel ? metadata.defaultTTSModel : "";
}

export function getAudioBriefingTTSProviderDefaultModel(
  provider: string,
  conversationMode: "single" | "duo",
): string {
  const normalized = provider.trim().toLowerCase();
  if (normalized === "elevenlabs" && conversationMode === "duo") {
    return "eleven_v3";
  }
  return getTTSProviderDefaultModel(normalized);
}
