"use client";

import type {
  AudioBriefingPersonaVoice,
  AivisModelsResponse,
  AzureSpeechVoicesResponse,
  ElevenLabsVoicesResponse,
  GeminiTTSVoicesResponse,
  OpenAITTSVoicesResponse,
  XAIVoicesResponse,
} from "@/lib/api";
import { getAudioBriefingTTSProviderDefaultModel } from "@/components/settings/providers/tts-provider-metadata";

export function findAudioBriefingActiveVoice(audioBriefingVoices: AudioBriefingPersonaVoice[], persona: string | null) {
  if (!persona) return null;
  return audioBriefingVoices.find((voice) => voice.persona === persona) ?? null;
}

export function buildVoicePickerCatalogData(
  aivisModelsData: AivisModelsResponse | null,
  xaiVoicesData: XAIVoicesResponse | null,
  elevenLabsVoicesData: ElevenLabsVoicesResponse | null,
  openAITTSVoicesData: OpenAITTSVoicesResponse | null,
  geminiTTSVoicesData: GeminiTTSVoicesResponse | null,
  azureSpeechVoicesData: AzureSpeechVoicesResponse | null,
) {
  const audioBriefingAivisModels = aivisModelsData?.models ?? [];
  const audioBriefingXAIVoices = xaiVoicesData?.voices ?? [];
  const audioBriefingElevenLabsVoices = elevenLabsVoicesData?.voices ?? [];
  const audioBriefingOpenAITTSVoices = openAITTSVoicesData?.voices ?? [];
  const audioBriefingGeminiTTSVoices = geminiTTSVoicesData?.voices ?? [];
  const audioBriefingAzureSpeechVoices = azureSpeechVoicesData?.voices ?? [];
  return {
    audioBriefingAivisModels,
    audioBriefingXAIVoices,
    audioBriefingElevenLabsVoices,
    audioBriefingOpenAITTSVoices,
    audioBriefingGeminiTTSVoices,
    audioBriefingAzureSpeechVoices,
    summaryAudioAivisModels: audioBriefingAivisModels,
    summaryAudioXAIVoices: audioBriefingXAIVoices,
    summaryAudioElevenLabsVoices: audioBriefingElevenLabsVoices,
    summaryAudioOpenAITTSVoices: audioBriefingOpenAITTSVoices,
    summaryAudioGeminiTTSVoices: audioBriefingGeminiTTSVoices,
    summaryAudioAzureSpeechVoices: audioBriefingAzureSpeechVoices,
  };
}

type AudioBriefingPickerState = {
  aivisPickerPersona: string | null;
  fishPickerPersona: string | null;
  xaiPickerPersona: string | null;
  elevenLabsPickerPersona: string | null;
  openAITTPickerPersona: string | null;
  geminiTTSPickerPersona: string | null;
  azureSpeechPickerPersona: string | null;
};

type AudioBriefingPickerOpeners = {
  setAivisPickerPersona: (persona: string | null) => void;
  setFishPickerPersona: (persona: string | null) => void;
  setXAIPickerPersona: (persona: string | null) => void;
  setElevenLabsPickerPersona: (persona: string | null) => void;
  setOpenAITTPickerPersona: (persona: string | null) => void;
  setGeminiTTSPickerPersona: (persona: string | null) => void;
  setAzureSpeechPickerPersona: (persona: string | null) => void;
};

export function buildAudioBriefingPickerOpeners(params: {
  pickers: AudioBriefingPickerOpeners;
  aivisModelsData: AivisModelsResponse | null;
  xaiVoicesData: XAIVoicesResponse | null;
  elevenLabsVoicesData: ElevenLabsVoicesResponse | null;
  openAITTSVoicesData: OpenAITTSVoicesResponse | null;
  geminiTTSVoicesData: GeminiTTSVoicesResponse | null;
  azureSpeechVoicesData: AzureSpeechVoicesResponse | null;
  loadAivisModels: () => Promise<unknown>;
  loadXAIVoices: () => Promise<unknown>;
  loadElevenLabsVoices: () => Promise<unknown>;
  loadOpenAITTSVoices: () => Promise<unknown>;
  loadGeminiTTSVoices: () => Promise<unknown>;
  loadAzureSpeechVoices: () => Promise<unknown>;
}) {
  return {
    openAivisPicker: async (persona: string) => {
      params.pickers.setAivisPickerPersona(persona);
      if (params.aivisModelsData == null) {
        try {
          await params.loadAivisModels();
        } catch {
          return;
        }
      }
    },
    openXAIPicker: async (persona: string) => {
      params.pickers.setXAIPickerPersona(persona);
      if (params.xaiVoicesData == null) {
        try {
          await params.loadXAIVoices();
        } catch {
          return;
        }
      }
    },
    openFishPicker: async (persona: string) => {
      params.pickers.setFishPickerPersona(persona);
    },
    openElevenLabsPicker: async (persona: string) => {
      params.pickers.setElevenLabsPickerPersona(persona);
      if (params.elevenLabsVoicesData == null) {
        try {
          await params.loadElevenLabsVoices();
        } catch {
          return;
        }
      }
    },
    openOpenAITTSPicker: async (persona: string) => {
      params.pickers.setOpenAITTPickerPersona(persona);
      if (params.openAITTSVoicesData == null) {
        try {
          await params.loadOpenAITTSVoices();
        } catch {
          return;
        }
      }
    },
    openGeminiTTSPicker: async (persona: string) => {
      params.pickers.setGeminiTTSPickerPersona(persona);
      if (params.geminiTTSVoicesData == null) {
        try {
          await params.loadGeminiTTSVoices();
        } catch {
          return;
        }
      }
    },
    openAzureSpeechPicker: async (persona: string) => {
      params.pickers.setAzureSpeechPickerPersona(persona);
      if (params.azureSpeechVoicesData == null) {
        try {
          await params.loadAzureSpeechVoices();
        } catch {
          return;
        }
      }
    },
  };
}

export function buildSummaryAudioPickerOpenAction(params: {
  provider: string;
  openers: {
    setSummaryAudioAivisPickerOpen: (open: boolean) => void;
    setSummaryAudioFishPickerOpen: (open: boolean) => void;
    setSummaryAudioElevenLabsPickerOpen: (open: boolean) => void;
    setSummaryAudioXAIPickerOpen: (open: boolean) => void;
    setSummaryAudioOpenAITTPickerOpen: (open: boolean) => void;
    setSummaryAudioGeminiTTSPickerOpen: (open: boolean) => void;
    setSummaryAudioAzureSpeechPickerOpen: (open: boolean) => void;
  };
  aivisModelsData: AivisModelsResponse | null;
  xaiVoicesData: XAIVoicesResponse | null;
  elevenLabsVoicesData: ElevenLabsVoicesResponse | null;
  openAITTSVoicesData: OpenAITTSVoicesResponse | null;
  geminiTTSVoicesData: GeminiTTSVoicesResponse | null;
  azureSpeechVoicesData: AzureSpeechVoicesResponse | null;
  loadAivisModels: () => Promise<unknown>;
  loadXAIVoices: () => Promise<unknown>;
  loadElevenLabsVoices: () => Promise<unknown>;
  loadOpenAITTSVoices: () => Promise<unknown>;
  loadGeminiTTSVoices: () => Promise<unknown>;
  loadAzureSpeechVoices: () => Promise<unknown>;
}) {
  return () => {
    if (params.provider === "aivis") {
      params.openers.setSummaryAudioAivisPickerOpen(true);
      if (params.aivisModelsData == null) void params.loadAivisModels().catch(() => undefined);
    } else if (params.provider === "fish") {
      params.openers.setSummaryAudioFishPickerOpen(true);
    } else if (params.provider === "elevenlabs") {
      params.openers.setSummaryAudioElevenLabsPickerOpen(true);
      if (params.elevenLabsVoicesData == null) void params.loadElevenLabsVoices().catch(() => undefined);
    } else if (params.provider === "xai") {
      params.openers.setSummaryAudioXAIPickerOpen(true);
      if (params.xaiVoicesData == null) void params.loadXAIVoices().catch(() => undefined);
    } else if (params.provider === "openai") {
      params.openers.setSummaryAudioOpenAITTPickerOpen(true);
      if (params.openAITTSVoicesData == null) void params.loadOpenAITTSVoices().catch(() => undefined);
    } else if (params.provider === "gemini_tts") {
      params.openers.setSummaryAudioGeminiTTSPickerOpen(true);
      if (params.geminiTTSVoicesData == null) void params.loadGeminiTTSVoices().catch(() => undefined);
    } else if (params.provider === "azure_speech") {
      params.openers.setSummaryAudioAzureSpeechPickerOpen(true);
      if (params.azureSpeechVoicesData == null) void params.loadAzureSpeechVoices().catch(() => undefined);
    }
  };
}

export function buildAudioBriefingPickerSelectActions(params: {
  pickers: AudioBriefingPickerState;
  audioBriefingVoices: AudioBriefingPersonaVoice[];
  activeOpenAITTSVoice: AudioBriefingPersonaVoice | null;
  activeGeminiTTSVoice: AudioBriefingPersonaVoice | null;
  activeElevenLabsVoice: AudioBriefingPersonaVoice | null;
  conversationMode: "single" | "duo";
  updateAudioBriefingVoice: (persona: string, patch: Partial<AudioBriefingPersonaVoice>) => void;
}) {
  return {
    onSelectAivis: (selection: { voice_model: string; voice_style: string }) => {
      if (!params.pickers.aivisPickerPersona) return;
      params.updateAudioBriefingVoice(params.pickers.aivisPickerPersona, {
        tts_provider: "aivis",
        voice_model: selection.voice_model,
        voice_style: selection.voice_style,
      });
    },
    onSelectFish: (selection: { voice_model: string; provider_voice_label: string; provider_voice_description: string }) => {
      if (!params.pickers.fishPickerPersona) return;
      params.updateAudioBriefingVoice(params.pickers.fishPickerPersona, {
        tts_provider: "fish",
        tts_model: params.audioBriefingVoices.find((voice) => voice.persona === params.pickers.fishPickerPersona)?.tts_model || "s2-pro",
        voice_model: selection.voice_model,
        voice_style: "",
        provider_voice_label: selection.provider_voice_label,
        provider_voice_description: selection.provider_voice_description,
      });
    },
    onSelectXAI: (selection: { voice_id: string }) => {
      if (!params.pickers.xaiPickerPersona) return;
      params.updateAudioBriefingVoice(params.pickers.xaiPickerPersona, {
        tts_provider: "xai",
        voice_model: selection.voice_id,
        voice_style: "",
      });
    },
    onSelectOpenAI: (selection: { voice_id: string }) => {
      if (!params.pickers.openAITTPickerPersona) return;
      params.updateAudioBriefingVoice(params.pickers.openAITTPickerPersona, {
        tts_provider: "openai",
        tts_model: params.activeOpenAITTSVoice?.tts_model || "gpt-4o-mini-tts",
        voice_model: selection.voice_id,
        voice_style: "",
      });
    },
    onSelectElevenLabs: (selection: { voice_id: string; label: string; description: string }) => {
      if (!params.pickers.elevenLabsPickerPersona) return;
      const elevenLabsModel = params.activeElevenLabsVoice?.tts_model?.trim() || "";
      params.updateAudioBriefingVoice(params.pickers.elevenLabsPickerPersona, {
        tts_provider: "elevenlabs",
        tts_model: params.conversationMode === "duo"
          ? "eleven_v3"
          : elevenLabsModel || getAudioBriefingTTSProviderDefaultModel("elevenlabs", params.conversationMode),
        voice_model: selection.voice_id,
        voice_style: "",
        provider_voice_label: selection.label,
        provider_voice_description: selection.description,
      });
    },
    onSelectGemini: (selection: { voice_name: string }) => {
      if (!params.pickers.geminiTTSPickerPersona) return;
      params.updateAudioBriefingVoice(params.pickers.geminiTTSPickerPersona, {
        tts_provider: "gemini_tts",
        tts_model: params.activeGeminiTTSVoice?.tts_model || "gemini-2.5-flash-tts",
        voice_model: selection.voice_name,
        voice_style: "",
      });
    },
    onSelectAzureSpeech: (selection: { voice_id: string; label: string; description: string }) => {
      if (!params.pickers.azureSpeechPickerPersona) return;
      params.updateAudioBriefingVoice(params.pickers.azureSpeechPickerPersona, {
        tts_provider: "azure_speech",
        voice_model: selection.voice_id,
        voice_style: "",
        provider_voice_label: selection.label,
        provider_voice_description: selection.description,
      });
    },
  };
}

export function buildSummaryAudioPickerSelectActions(params: {
  setSummaryAudioProvider: (provider: string) => void;
  setSummaryAudioTTSModel: (model: string) => void;
  summaryAudioTTSModel: string;
  setSummaryAudioVoiceModel: (voiceModel: string) => void;
  setSummaryAudioVoiceStyle: (voiceStyle: string) => void;
  setSummaryAudioProviderVoiceLabel: (label: string) => void;
  setSummaryAudioProviderVoiceDescription: (description: string) => void;
}) {
  return {
    onSelectAivis: (selection: { voice_model: string; voice_style: string }) => {
      params.setSummaryAudioProvider("aivis");
      params.setSummaryAudioVoiceModel(selection.voice_model);
      params.setSummaryAudioVoiceStyle(selection.voice_style);
      params.setSummaryAudioProviderVoiceLabel("");
      params.setSummaryAudioProviderVoiceDescription("");
    },
    onSelectFish: (selection: { voice_model: string; provider_voice_label: string; provider_voice_description: string }) => {
      params.setSummaryAudioProvider("fish");
      params.setSummaryAudioTTSModel(params.summaryAudioTTSModel.trim() || "s2-pro");
      params.setSummaryAudioVoiceModel(selection.voice_model);
      params.setSummaryAudioVoiceStyle("");
      params.setSummaryAudioProviderVoiceLabel(selection.provider_voice_label);
      params.setSummaryAudioProviderVoiceDescription(selection.provider_voice_description);
    },
    onSelectElevenLabs: (selection: { voice_id: string; label: string; description: string }) => {
      params.setSummaryAudioVoiceModel(selection.voice_id);
      params.setSummaryAudioVoiceStyle("");
      params.setSummaryAudioProviderVoiceLabel(selection.label);
      params.setSummaryAudioProviderVoiceDescription(selection.description);
    },
    onSelectXAI: (selection: { voice_id: string }) => {
      params.setSummaryAudioProvider("xai");
      params.setSummaryAudioVoiceModel(selection.voice_id);
      params.setSummaryAudioVoiceStyle("");
      params.setSummaryAudioProviderVoiceLabel("");
      params.setSummaryAudioProviderVoiceDescription("");
    },
    onSelectOpenAI: (selection: { voice_id: string }) => {
      params.setSummaryAudioProvider("openai");
      params.setSummaryAudioTTSModel(params.summaryAudioTTSModel.trim() || "tts-1");
      params.setSummaryAudioVoiceModel(selection.voice_id);
      params.setSummaryAudioVoiceStyle("");
      params.setSummaryAudioProviderVoiceLabel("");
      params.setSummaryAudioProviderVoiceDescription("");
    },
    onSelectGemini: (selection: { voice_name: string }) => {
      params.setSummaryAudioProvider("gemini_tts");
      params.setSummaryAudioTTSModel(params.summaryAudioTTSModel.trim() || "gemini-2.5-flash-tts");
      params.setSummaryAudioVoiceModel(selection.voice_name);
      params.setSummaryAudioVoiceStyle("");
      params.setSummaryAudioProviderVoiceLabel("");
      params.setSummaryAudioProviderVoiceDescription("");
    },
    onSelectAzureSpeech: (selection: { voice_id: string; label: string; description: string }) => {
      params.setSummaryAudioProvider("azure_speech");
      params.setSummaryAudioVoiceModel(selection.voice_id);
      params.setSummaryAudioVoiceStyle("");
      params.setSummaryAudioProviderVoiceLabel(selection.label);
      params.setSummaryAudioProviderVoiceDescription(selection.description);
    },
  };
}
