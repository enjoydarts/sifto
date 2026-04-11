"use client";

import { useState } from "react";

export function useSettingsDialogState() {
  const [uiFontSansPickerOpen, setUIFontSansPickerOpen] = useState(false);
  const [uiFontSerifPickerOpen, setUIFontSerifPickerOpen] = useState(false);
  const [llmExtrasOpen, setLLMExtrasOpen] = useState(false);
  const [modelGuideOpen, setModelGuideOpen] = useState(false);
  const [audioBriefingPresetSaveOpen, setAudioBriefingPresetSaveOpen] = useState(false);
  const [audioBriefingPresetApplyOpen, setAudioBriefingPresetApplyOpen] = useState(false);
  const [audioBriefingPresetName, setAudioBriefingPresetName] = useState("");
  const [audioBriefingPresetSaving, setAudioBriefingPresetSaving] = useState(false);
  const [audioBriefingPresetApplySelection, setAudioBriefingPresetApplySelection] = useState<string | null>(null);
  const [aivisPickerPersona, setAivisPickerPersona] = useState<string | null>(null);
  const [xaiPickerPersona, setXAIPickerPersona] = useState<string | null>(null);
  const [fishPickerPersona, setFishPickerPersona] = useState<string | null>(null);
  const [elevenLabsPickerPersona, setElevenLabsPickerPersona] = useState<string | null>(null);
  const [openAITTPickerPersona, setOpenAITTPickerPersona] = useState<string | null>(null);
  const [geminiTTSPickerPersona, setGeminiTTSPickerPersona] = useState<string | null>(null);
  const [azureSpeechPickerPersona, setAzureSpeechPickerPersona] = useState<string | null>(null);
  const [summaryAudioAivisPickerOpen, setSummaryAudioAivisPickerOpen] = useState(false);
  const [summaryAudioFishPickerOpen, setSummaryAudioFishPickerOpen] = useState(false);
  const [summaryAudioElevenLabsPickerOpen, setSummaryAudioElevenLabsPickerOpen] = useState(false);
  const [summaryAudioXAIPickerOpen, setSummaryAudioXAIPickerOpen] = useState(false);
  const [summaryAudioOpenAITTPickerOpen, setSummaryAudioOpenAITTPickerOpen] = useState(false);
  const [summaryAudioGeminiTTSPickerOpen, setSummaryAudioGeminiTTSPickerOpen] = useState(false);
  const [summaryAudioAzureSpeechPickerOpen, setSummaryAudioAzureSpeechPickerOpen] = useState(false);

  return {
    uiFonts: {
      uiFontSansPickerOpen,
      uiFontSerifPickerOpen,
      openSansPicker: () => setUIFontSansPickerOpen(true),
      openSerifPicker: () => setUIFontSerifPickerOpen(true),
      closeSansPicker: () => setUIFontSansPickerOpen(false),
      closeSerifPicker: () => setUIFontSerifPickerOpen(false),
    },
    llm: {
      llmExtrasOpen,
      setLLMExtrasOpen,
      modelGuideOpen,
      openModelGuide: () => setModelGuideOpen(true),
      closeModelGuide: () => setModelGuideOpen(false),
    },
    presets: {
      audioBriefingPresetSaveOpen,
      setAudioBriefingPresetSaveOpen,
      audioBriefingPresetApplyOpen,
      setAudioBriefingPresetApplyOpen,
      audioBriefingPresetName,
      setAudioBriefingPresetName,
      audioBriefingPresetSaving,
      setAudioBriefingPresetSaving,
      audioBriefingPresetApplySelection,
      setAudioBriefingPresetApplySelection,
    },
    audioBriefingPickers: {
      aivisPickerPersona,
      xaiPickerPersona,
      fishPickerPersona,
      elevenLabsPickerPersona,
      openAITTPickerPersona,
      geminiTTSPickerPersona,
      azureSpeechPickerPersona,
      setAivisPickerPersona,
      setXAIPickerPersona,
      setFishPickerPersona,
      setElevenLabsPickerPersona,
      setOpenAITTPickerPersona,
      setGeminiTTSPickerPersona,
      setAzureSpeechPickerPersona,
      closeAivisPicker: () => setAivisPickerPersona(null),
      closeXAIPicker: () => setXAIPickerPersona(null),
      closeFishPicker: () => setFishPickerPersona(null),
      closeElevenLabsPicker: () => setElevenLabsPickerPersona(null),
      closeOpenAITTPicker: () => setOpenAITTPickerPersona(null),
      closeGeminiTTSPicker: () => setGeminiTTSPickerPersona(null),
      closeAzureSpeechPicker: () => setAzureSpeechPickerPersona(null),
    },
    summaryAudioPickers: {
      summaryAudioAivisPickerOpen,
      summaryAudioFishPickerOpen,
      summaryAudioElevenLabsPickerOpen,
      summaryAudioXAIPickerOpen,
      summaryAudioOpenAITTPickerOpen,
      summaryAudioGeminiTTSPickerOpen,
      summaryAudioAzureSpeechPickerOpen,
      setSummaryAudioAivisPickerOpen,
      setSummaryAudioFishPickerOpen,
      setSummaryAudioElevenLabsPickerOpen,
      setSummaryAudioXAIPickerOpen,
      setSummaryAudioOpenAITTPickerOpen,
      setSummaryAudioGeminiTTSPickerOpen,
      setSummaryAudioAzureSpeechPickerOpen,
      closeAivisPicker: () => setSummaryAudioAivisPickerOpen(false),
      closeFishPicker: () => setSummaryAudioFishPickerOpen(false),
      closeElevenLabsPicker: () => setSummaryAudioElevenLabsPickerOpen(false),
      closeXAIPicker: () => setSummaryAudioXAIPickerOpen(false),
      closeOpenAITTPicker: () => setSummaryAudioOpenAITTPickerOpen(false),
      closeGeminiTTSPicker: () => setSummaryAudioGeminiTTSPickerOpen(false),
      closeAzureSpeechPicker: () => setSummaryAudioAzureSpeechPickerOpen(false),
    },
  };
}
