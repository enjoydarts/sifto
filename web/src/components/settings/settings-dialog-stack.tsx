"use client";

import {
  AudioBriefingPersonaVoice,
  AudioBriefingPreset,
  AivisModelSnapshot,
  AzureSpeechVoiceCatalogEntry,
  ElevenLabsVoiceCatalogEntry,
  GeminiTTSVoiceCatalogEntry,
  LLMCatalogModel,
  OpenAITTSVoiceSnapshot,
  XAIVoiceSnapshot,
} from "@/lib/api";
import AivisVoicePickerModal from "@/components/settings/aivis-voice-picker-modal";
import AudioBriefingPresetApplyModal from "@/components/settings/audio-briefing-preset-apply-modal";
import AudioBriefingPresetSaveModal from "@/components/settings/audio-briefing-preset-save-modal";
import AzureSpeechVoicePickerModal from "@/components/settings/azure-speech-voice-picker-modal";
import ElevenLabsVoicePickerModalExternal from "@/components/settings/elevenlabs-voice-picker-modal";
import FishVoicePickerModal from "@/components/settings/fish-voice-picker-modal";
import GeminiTTSVoicePickerModal from "@/components/settings/gemini-tts-voice-picker-modal";
import ModelGuideModal from "@/components/settings/model-guide-modal";
import OpenAITTSVoicePickerModal from "@/components/settings/openai-tts-voice-picker-modal";
import UIFontPickerModal from "@/components/settings/ui-font-picker-modal";
import XAIVoicePickerModal from "@/components/settings/xai-voice-picker-modal";
import { type UIFontCatalogEntry } from "@/lib/ui-fonts";

type AudioBriefingVoicePickerStackProps = {
  pickerState: {
    aivisPickerPersona: string | null;
    fishPickerPersona: string | null;
    xaiPickerPersona: string | null;
    openAITTPickerPersona: string | null;
    elevenLabsPickerPersona: string | null;
    geminiTTSPickerPersona: string | null;
    azureSpeechPickerPersona: string | null;
    activeAivisVoice: AudioBriefingPersonaVoice | null;
    activeXAIVoice: AudioBriefingPersonaVoice | null;
    activeOpenAITTSVoice: AudioBriefingPersonaVoice | null;
    activeElevenLabsVoice: AudioBriefingPersonaVoice | null;
    activeGeminiTTSVoice: AudioBriefingPersonaVoice | null;
    activeAzureSpeechVoice: AudioBriefingPersonaVoice | null;
    audioBriefingVoices: AudioBriefingPersonaVoice[];
  };
  catalogs: {
    aivisModels: AivisModelSnapshot[];
    aivisModelsLoading: boolean;
    aivisModelsSyncing: boolean;
    aivisModelsError: string | null;
    xaiVoices: XAIVoiceSnapshot[];
    xaiVoicesLoading: boolean;
    xaiVoicesSyncing: boolean;
    xaiVoicesError: string | null;
    openAITTSVoices: OpenAITTSVoiceSnapshot[];
    openAITTSVoicesLoading: boolean;
    openAITTSVoicesSyncing: boolean;
    openAITTSVoicesError: string | null;
    elevenLabsVoices: ElevenLabsVoiceCatalogEntry[];
    elevenLabsVoicesLoading: boolean;
    elevenLabsVoicesError: string | null;
    geminiTTSVoices: GeminiTTSVoiceCatalogEntry[];
    geminiTTSVoicesLoading: boolean;
    geminiTTSVoicesError: string | null;
    azureSpeechVoices: AzureSpeechVoiceCatalogEntry[];
    azureSpeechVoicesLoading: boolean;
    azureSpeechVoicesError: string | null;
  };
  actions: {
    onCloseAivis: () => void;
    onCloseFish: () => void;
    onCloseXAI: () => void;
    onCloseOpenAI: () => void;
    onCloseElevenLabs: () => void;
    onCloseGemini: () => void;
    onCloseAzureSpeech: () => void;
    onSyncAivis: () => void;
    onSyncXAI: () => void;
    onSyncOpenAI: () => void;
    onRefreshElevenLabs: () => void;
    onRefreshGemini: () => void;
    onRefreshAzureSpeech: () => void;
    onSelectAivis: (selection: { voice_model: string; voice_style: string }) => void;
    onSelectFish: (selection: { voice_model: string; provider_voice_label: string; provider_voice_description: string }) => void;
    onSelectXAI: (selection: { voice_id: string }) => void;
    onSelectOpenAI: (selection: { voice_id: string }) => void;
    onSelectElevenLabs: (selection: { voice_id: string; label: string; description: string }) => void;
    onSelectGemini: (selection: { voice_name: string }) => void;
    onSelectAzureSpeech: (selection: { voice_id: string; label: string; description: string }) => void;
  };
};

export function AudioBriefingVoicePickerStack({ pickerState, catalogs, actions }: AudioBriefingVoicePickerStackProps) {
  return (
    <>
      <AivisVoicePickerModal
        open={Boolean(pickerState.aivisPickerPersona)}
        loading={catalogs.aivisModelsLoading}
        syncing={catalogs.aivisModelsSyncing}
        error={catalogs.aivisModelsError}
        models={catalogs.aivisModels}
        currentVoiceModel={pickerState.activeAivisVoice?.voice_model ?? ""}
        currentVoiceStyle={pickerState.activeAivisVoice?.voice_style ?? ""}
        onClose={actions.onCloseAivis}
        onSync={actions.onSyncAivis}
        onSelect={actions.onSelectAivis}
      />

      <FishVoicePickerModal
        open={Boolean(pickerState.fishPickerPersona)}
        currentVoiceModel={pickerState.audioBriefingVoices.find((voice) => voice.persona === pickerState.fishPickerPersona)?.voice_model ?? ""}
        onClose={actions.onCloseFish}
        onSelect={actions.onSelectFish}
      />

      <XAIVoicePickerModal
        open={Boolean(pickerState.xaiPickerPersona)}
        loading={catalogs.xaiVoicesLoading}
        syncing={catalogs.xaiVoicesSyncing}
        error={catalogs.xaiVoicesError}
        voices={catalogs.xaiVoices}
        currentVoiceID={pickerState.activeXAIVoice?.voice_model ?? ""}
        onClose={actions.onCloseXAI}
        onSync={actions.onSyncXAI}
        onSelect={actions.onSelectXAI}
      />

      <OpenAITTSVoicePickerModal
        open={Boolean(pickerState.openAITTPickerPersona)}
        loading={catalogs.openAITTSVoicesLoading}
        syncing={catalogs.openAITTSVoicesSyncing}
        error={catalogs.openAITTSVoicesError}
        voices={catalogs.openAITTSVoices}
        currentVoiceID={pickerState.activeOpenAITTSVoice?.voice_model ?? ""}
        onClose={actions.onCloseOpenAI}
        onSync={actions.onSyncOpenAI}
        onSelect={actions.onSelectOpenAI}
      />

      <ElevenLabsVoicePickerModalExternal
        open={Boolean(pickerState.elevenLabsPickerPersona)}
        loading={catalogs.elevenLabsVoicesLoading}
        error={catalogs.elevenLabsVoicesError}
        voices={catalogs.elevenLabsVoices}
        currentVoiceID={pickerState.activeElevenLabsVoice?.voice_model ?? ""}
        onClose={actions.onCloseElevenLabs}
        onRefresh={actions.onRefreshElevenLabs}
        onSelect={actions.onSelectElevenLabs}
      />

      <GeminiTTSVoicePickerModal
        open={Boolean(pickerState.geminiTTSPickerPersona)}
        loading={catalogs.geminiTTSVoicesLoading}
        error={catalogs.geminiTTSVoicesError}
        voices={catalogs.geminiTTSVoices}
        currentVoiceName={pickerState.activeGeminiTTSVoice?.voice_model ?? ""}
        onClose={actions.onCloseGemini}
        onRefresh={actions.onRefreshGemini}
        onSelect={actions.onSelectGemini}
      />

      <AzureSpeechVoicePickerModal
        open={Boolean(pickerState.azureSpeechPickerPersona)}
        loading={catalogs.azureSpeechVoicesLoading}
        error={catalogs.azureSpeechVoicesError}
        voices={catalogs.azureSpeechVoices}
        currentVoiceID={pickerState.activeAzureSpeechVoice?.voice_model ?? ""}
        onClose={actions.onCloseAzureSpeech}
        onRefresh={actions.onRefreshAzureSpeech}
        onSelect={actions.onSelectAzureSpeech}
      />
    </>
  );
}

type SummaryAudioVoicePickerStackProps = {
  pickerState: {
    summaryAudioAivisPickerOpen: boolean;
    summaryAudioFishPickerOpen: boolean;
    summaryAudioElevenLabsPickerOpen: boolean;
    summaryAudioXAIPickerOpen: boolean;
    summaryAudioOpenAITTPickerOpen: boolean;
    summaryAudioGeminiTTSPickerOpen: boolean;
    summaryAudioAzureSpeechPickerOpen: boolean;
    summaryAudioVoiceModel: string;
    summaryAudioVoiceStyle: string;
  };
  catalogs: {
    aivisModels: AivisModelSnapshot[];
    aivisModelsLoading: boolean;
    aivisModelsSyncing: boolean;
    aivisModelsError: string | null;
    xaiVoices: XAIVoiceSnapshot[];
    xaiVoicesLoading: boolean;
    xaiVoicesSyncing: boolean;
    xaiVoicesError: string | null;
    openAITTSVoices: OpenAITTSVoiceSnapshot[];
    openAITTSVoicesLoading: boolean;
    openAITTSVoicesSyncing: boolean;
    openAITTSVoicesError: string | null;
    elevenLabsVoices: ElevenLabsVoiceCatalogEntry[];
    elevenLabsVoicesLoading: boolean;
    elevenLabsVoicesError: string | null;
    geminiTTSVoices: GeminiTTSVoiceCatalogEntry[];
    geminiTTSVoicesLoading: boolean;
    geminiTTSVoicesError: string | null;
    azureSpeechVoices: AzureSpeechVoiceCatalogEntry[];
    azureSpeechVoicesLoading: boolean;
    azureSpeechVoicesError: string | null;
  };
  actions: {
    onCloseAivis: () => void;
    onCloseFish: () => void;
    onCloseElevenLabs: () => void;
    onCloseXAI: () => void;
    onCloseOpenAI: () => void;
    onCloseGemini: () => void;
    onCloseAzureSpeech: () => void;
    onSyncAivis: () => void;
    onSyncXAI: () => void;
    onSyncOpenAI: () => void;
    onRefreshElevenLabs: () => void;
    onRefreshGemini: () => void;
    onRefreshAzureSpeech: () => void;
    onSelectAivis: (selection: { voice_model: string; voice_style: string }) => void;
    onSelectFish: (selection: { voice_model: string; provider_voice_label: string; provider_voice_description: string }) => void;
    onSelectElevenLabs: (selection: { voice_id: string; label: string; description: string }) => void;
    onSelectXAI: (selection: { voice_id: string }) => void;
    onSelectOpenAI: (selection: { voice_id: string }) => void;
    onSelectGemini: (selection: { voice_name: string }) => void;
    onSelectAzureSpeech: (selection: { voice_id: string; label: string; description: string }) => void;
  };
};

export function SummaryAudioVoicePickerStack({ pickerState, catalogs, actions }: SummaryAudioVoicePickerStackProps) {
  return (
    <>
      <AivisVoicePickerModal
        open={pickerState.summaryAudioAivisPickerOpen}
        loading={catalogs.aivisModelsLoading}
        syncing={catalogs.aivisModelsSyncing}
        error={catalogs.aivisModelsError}
        models={catalogs.aivisModels}
        currentVoiceModel={pickerState.summaryAudioVoiceModel}
        currentVoiceStyle={pickerState.summaryAudioVoiceStyle}
        onClose={actions.onCloseAivis}
        onSync={actions.onSyncAivis}
        onSelect={actions.onSelectAivis}
      />

      <FishVoicePickerModal
        open={pickerState.summaryAudioFishPickerOpen}
        currentVoiceModel={pickerState.summaryAudioVoiceModel}
        onClose={actions.onCloseFish}
        onSelect={actions.onSelectFish}
      />

      <ElevenLabsVoicePickerModalExternal
        open={pickerState.summaryAudioElevenLabsPickerOpen}
        loading={catalogs.elevenLabsVoicesLoading}
        error={catalogs.elevenLabsVoicesError}
        voices={catalogs.elevenLabsVoices}
        currentVoiceID={pickerState.summaryAudioVoiceModel}
        onClose={actions.onCloseElevenLabs}
        onRefresh={actions.onRefreshElevenLabs}
        onSelect={actions.onSelectElevenLabs}
      />

      <XAIVoicePickerModal
        open={pickerState.summaryAudioXAIPickerOpen}
        loading={catalogs.xaiVoicesLoading}
        syncing={catalogs.xaiVoicesSyncing}
        error={catalogs.xaiVoicesError}
        voices={catalogs.xaiVoices}
        currentVoiceID={pickerState.summaryAudioVoiceModel}
        onClose={actions.onCloseXAI}
        onSync={actions.onSyncXAI}
        onSelect={actions.onSelectXAI}
      />

      <OpenAITTSVoicePickerModal
        open={pickerState.summaryAudioOpenAITTPickerOpen}
        loading={catalogs.openAITTSVoicesLoading}
        syncing={catalogs.openAITTSVoicesSyncing}
        error={catalogs.openAITTSVoicesError}
        voices={catalogs.openAITTSVoices}
        currentVoiceID={pickerState.summaryAudioVoiceModel}
        onClose={actions.onCloseOpenAI}
        onSync={actions.onSyncOpenAI}
        onSelect={actions.onSelectOpenAI}
      />

      <GeminiTTSVoicePickerModal
        open={pickerState.summaryAudioGeminiTTSPickerOpen}
        loading={catalogs.geminiTTSVoicesLoading}
        error={catalogs.geminiTTSVoicesError}
        voices={catalogs.geminiTTSVoices}
        currentVoiceName={pickerState.summaryAudioVoiceModel}
        onClose={actions.onCloseGemini}
        onRefresh={actions.onRefreshGemini}
        onSelect={actions.onSelectGemini}
      />

      <AzureSpeechVoicePickerModal
        open={pickerState.summaryAudioAzureSpeechPickerOpen}
        loading={catalogs.azureSpeechVoicesLoading}
        error={catalogs.azureSpeechVoicesError}
        voices={catalogs.azureSpeechVoices}
        currentVoiceID={pickerState.summaryAudioVoiceModel}
        onClose={actions.onCloseAzureSpeech}
        onRefresh={actions.onRefreshAzureSpeech}
        onSelect={actions.onSelectAzureSpeech}
      />
    </>
  );
}

type SettingsDialogStackProps = {
  uiFonts: {
    uiFontSansPickerOpen: boolean;
    uiFontSerifPickerOpen: boolean;
    uiFontSansOptions: UIFontCatalogEntry[];
    uiFontSerifOptions: UIFontCatalogEntry[];
    savedUIFontSansKey: string;
    savedUIFontSerifKey: string;
    uiFontSansKey: string;
    uiFontSerifKey: string;
    defaultUIFontSansKey: string;
    defaultUIFontSerifKey: string;
    onCloseUIFontSans: () => void;
    onCloseUIFontSerif: () => void;
    onSelectUIFontSans: (key: string) => void;
    onSelectUIFontSerif: (key: string) => void;
  };
  presets: {
    audioBriefingPresetSaveOpen: boolean;
    audioBriefingPresetSaving: boolean;
    audioBriefingPresetName: string;
    audioBriefingDefaultPersonaMode: "fixed" | "random";
    audioBriefingDefaultPersona: string;
    audioBriefingConversationMode: "single" | "duo";
    audioBriefingVoices: AudioBriefingPersonaVoice[];
    onClosePresetSave: () => void;
    onChangePresetName: (value: string) => void;
    onSavePreset: () => void;
    audioBriefingPresetApplyOpen: boolean;
    audioBriefingPresetsLoading: boolean;
    audioBriefingPresetsError: string | null;
    audioBriefingPresets: AudioBriefingPreset[];
    audioBriefingPresetApplySelection: string | null;
    onClosePresetApply: () => void;
    onRefreshPresets: () => void;
    onSelectPreset: (id: string) => void;
    onApplyPreset: (preset: AudioBriefingPreset) => void;
  };
  modelGuide: {
    modelGuideOpen: boolean;
    modelComparisonEntries: LLMCatalogModel[];
    onCloseModelGuide: () => void;
  };
  t: (key: string, fallback?: string) => string;
};

export function SettingsDialogStack({ uiFonts, presets, modelGuide, t }: SettingsDialogStackProps) {
  return (
    <>
      <UIFontPickerModal
        open={uiFonts.uiFontSansPickerOpen}
        kind="sans"
        title={t("settings.uiFonts.sansModalTitle")}
        subtitle={t("settings.uiFonts.sansModalSubtitle")}
        fonts={uiFonts.uiFontSansOptions}
        currentKey={uiFonts.savedUIFontSansKey}
        selectedKey={uiFonts.uiFontSansKey}
        defaultKey={uiFonts.defaultUIFontSansKey}
        onClose={uiFonts.onCloseUIFontSans}
        onSelect={uiFonts.onSelectUIFontSans}
      />

      <UIFontPickerModal
        open={uiFonts.uiFontSerifPickerOpen}
        kind="serif"
        title={t("settings.uiFonts.serifModalTitle")}
        subtitle={t("settings.uiFonts.serifModalSubtitle")}
        fonts={uiFonts.uiFontSerifOptions}
        currentKey={uiFonts.savedUIFontSerifKey}
        selectedKey={uiFonts.uiFontSerifKey}
        defaultKey={uiFonts.defaultUIFontSerifKey}
        onClose={uiFonts.onCloseUIFontSerif}
        onSelect={uiFonts.onSelectUIFontSerif}
      />

      <AudioBriefingPresetSaveModal
        open={presets.audioBriefingPresetSaveOpen}
        saving={presets.audioBriefingPresetSaving}
        presetName={presets.audioBriefingPresetName}
        defaultPersonaMode={presets.audioBriefingDefaultPersonaMode}
        defaultPersona={presets.audioBriefingDefaultPersona}
        conversationMode={presets.audioBriefingConversationMode}
        voices={presets.audioBriefingVoices}
        onClose={presets.onClosePresetSave}
        onChangeName={presets.onChangePresetName}
        onSave={presets.onSavePreset}
      />

      <AudioBriefingPresetApplyModal
        open={presets.audioBriefingPresetApplyOpen}
        loading={presets.audioBriefingPresetsLoading}
        error={presets.audioBriefingPresetsError}
        presets={presets.audioBriefingPresets}
        selectedPresetID={presets.audioBriefingPresetApplySelection}
        onClose={presets.onClosePresetApply}
        onRefresh={presets.onRefreshPresets}
        onSelectPreset={presets.onSelectPreset}
        onApplyPreset={presets.onApplyPreset}
      />

      <ModelGuideModal
        open={modelGuide.modelGuideOpen}
        onClose={modelGuide.onCloseModelGuide}
        entries={modelGuide.modelComparisonEntries}
        t={t}
      />
    </>
  );
}
