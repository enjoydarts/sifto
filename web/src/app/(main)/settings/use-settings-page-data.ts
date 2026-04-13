"use client";

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { AivisModelsResponse, AivisUserDictionary, api, AudioBriefingPersonaVoice, AudioBriefingPreset, AzureSpeechVoiceCatalogEntry, AzureSpeechVoicesResponse, ElevenLabsVoiceCatalogEntry, ElevenLabsVoicesResponse, GeminiTTSVoiceCatalogEntry, GeminiTTSVoicesResponse, LLMCatalog, NavigatorPersonaDefinition, NotificationPriorityRule, OpenAITTSVoiceSnapshot, OpenAITTSVoicesResponse, PodcastCategoryOption, PreferenceProfile, ProviderModelChangeEvent, UserSettings, XAIVoiceSnapshot, XAIVoicesResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";
import { getSummaryAudioReadiness } from "@/lib/summary-audio-readiness";
import { queryKeys } from "@/lib/query-keys";
import { normalizeAudioBriefingPresetVoices } from "@/components/settings/audio-briefing-preset-modal-helpers";
import { type ModelOption } from "@/components/settings/model-select";
import { type SettingsSectionID } from "@/components/settings/settings-page-shell";
import {
  buildSettingsRailNotes,
  buildSettingsSectionMeta,
  buildSettingsSectionNavItems,
} from "@/components/settings/settings-page-descriptors";
import { createAPIKeyActionHandlers } from "@/components/settings/settings-api-key-actions";
import { copyToClipboardAction, readFileAsBase64DataURL } from "@/components/settings/settings-media-actions";
import {
  runSavingAction,
  submitSavingFormAction,
} from "@/components/settings/settings-submit-actions";
import {
  openAudioBriefingPresetApplyAction,
  saveAudioBriefingPresetAction,
} from "@/components/settings/settings-preset-actions";
import {
  clearAivisUserDictionaryAction,
  deleteInoreaderOAuthAction,
  resetPreferenceProfileAction,
  runObsidianExportNowAction,
  saveAivisUserDictionaryAction,
  saveBudgetSettingsAction,
  saveDigestDeliveryAction,
  saveObsidianExportAction,
  saveReadingPlanAction,
} from "@/components/settings/settings-domain-actions";
import {
  buildAudioBriefingSettingsPayload,
  buildPodcastSettingsAfterArtworkUpload,
  buildPodcastSettingsPayload,
  buildSummaryAudioSettingsPayload,
  mergeAudioBriefingIntoSettings,
  mergeAudioBriefingVoicesIntoSettings,
  mergePodcastIntoSettings,
  mergeSummaryAudioIntoSettings,
} from "@/components/settings/settings-persistence-helpers";
import {
  buildAudioBriefingPickerOpeners,
  buildAudioBriefingPickerSelectActions,
  buildSummaryAudioPickerOpenAction,
  buildSummaryAudioPickerSelectActions,
  buildVoicePickerCatalogData,
  findAudioBriefingActiveVoice,
} from "@/components/settings/settings-picker-helpers";
import { loadResourceAction, syncResourceAction } from "@/components/settings/settings-resource-actions";
import {
  buildApiKeyCardLabels,
  buildUIFontState,
  dismissProviderModelUpdatesToLocalStorage,
  MODEL_UPDATES_DISMISSED_AT_KEY,
  restoreProviderModelUpdatesFromLocalStorage,
} from "@/components/settings/settings-system-helpers";
import { buildAccessCards, createAccessCardRuntime, resolveAccessCardSelection } from "@/components/settings/system-access-cards";
import { useSettingsDialogState } from "@/components/settings/use-settings-dialog-state";
import {
  AudioBriefingNumericInputField,
  AudioBriefingScheduleSelection,
  AudioBriefingVoiceInputDrafts,
  buildAudioBriefingPresetRequest,
  buildAudioBriefingVoiceInputDrafts,
  buildDefaultAudioBriefingVoices,
  buildDefaultSummaryAudioVoiceSettings,
  buildPodcastRSSURL,
  buildSummaryAudioVoiceInputDrafts,
  EMPTY_NAVIGATOR_PERSONA,
  formatAudioBriefingScheduleSelection,
  formatAudioBriefingDecimalInput,
  isCompleteDecimalInput,
  isSettingsSectionID,
  localizePreferenceProfileErrorMessage,
  NAVIGATOR_PERSONA_KEYS,
  resolveAudioBriefingScheduleSelection,
  SummaryAudioNumericInputField,
  SummaryAudioVoiceInputDrafts,
  tWithVars,
} from "@/components/settings/settings-page-helpers";
import {
  buildCostPerformancePreset,
  localizeSettingsErrorMessage,
} from "@/components/settings/providers/llm-provider-metadata";
import {
  buildEmbeddingModelOptions,
  buildModelComparisonEntries,
  buildModelSelectLabels,
  buildOptionsForChatModel,
  buildOptionsForPurpose,
  buildUnavailableSelectedModelWarnings,
  buildVisibleProviderModelUpdates,
} from "@/components/settings/providers/llm-model-options";
import {
  getAudioBriefingTTSProviderDefaultModel,
  getTTSProviderDefaultModel,
} from "@/components/settings/providers/tts-provider-metadata";
import { resolveTTSVoiceDisplay } from "@/components/settings/providers/tts-voice-display";
import {
  getAudioBriefingProviderCapabilities,
  getAudioBriefingVoiceStatus,
  getSummaryAudioVoiceStatus,
  resolveAivisVoiceSelection,
  resolveElevenLabsVoiceSelection,
  resolveGeminiTTSVoiceSelection,
  resolveOpenAITTSVoiceSelection,
  resolveAzureSpeechVoiceSelection,
  resolveXAIVoiceSelection,
} from "@/components/settings/providers/tts-provider-readiness";
import {
  buildAudioBriefingDictionaryState,
  buildAudioBriefingDuoReadiness,
  buildAudioBriefingScriptModels,
  buildAudioBriefingSettingsState,
  buildAudioBriefingVoiceMatrixAvailability,
  buildAudioBriefingVoiceMatrixCatalogs,
  buildAudioBriefingVoiceMatrixStatus,
  buildBudgetActions,
  buildBudgetState,
  buildDigestActions,
  buildDigestState,
  buildIntegrationsState,
  buildLLMModelsExtras,
  buildLLMModelsState,
  buildNavigatorActions,
  buildNavigatorState,
  buildPodcastState,
  buildReadingPlanActions,
  buildReadingPlanState,
  buildSavingForm,
  buildSummaryAudioIntegrations,
  buildSummaryAudioState,
  buildSystemAccessState,
} from "@/components/settings/sections/settings-page-adapters";
import { DEFAULT_UI_FONT_SANS_KEY, DEFAULT_UI_FONT_SERIF_KEY, ensureUIFontPreviewLoaded, getSelectableSansFonts, getSelectableSerifFonts, persistUIFontSelection } from "@/lib/ui-fonts";

export function useSettingsPageData() {
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const searchParams = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [savingAudioBriefing, setSavingAudioBriefing] = useState(false);
  const [savingPodcast, setSavingPodcast] = useState(false);
  const [uploadingPodcastArtwork, setUploadingPodcastArtwork] = useState(false);
  const [savingAudioBriefingVoices, setSavingAudioBriefingVoices] = useState(false);
  const [savingBudget, setSavingBudget] = useState(false);
  const [savingDigestDelivery, setSavingDigestDelivery] = useState(false);
  const [savingReadingPlan, setSavingReadingPlan] = useState(false);
  const [savingObsidianExport, setSavingObsidianExport] = useState(false);
  const [runningObsidianExport, setRunningObsidianExport] = useState(false);
  const [savingLLMModels, setSavingLLMModels] = useState(false);
  const [savingAnthropicKey, setSavingAnthropicKey] = useState(false);
  const [deletingAnthropicKey, setDeletingAnthropicKey] = useState(false);
  const [savingOpenAIKey, setSavingOpenAIKey] = useState(false);
  const [deletingOpenAIKey, setDeletingOpenAIKey] = useState(false);
  const [savingGoogleKey, setSavingGoogleKey] = useState(false);
  const [deletingGoogleKey, setDeletingGoogleKey] = useState(false);
  const [savingGroqKey, setSavingGroqKey] = useState(false);
  const [deletingGroqKey, setDeletingGroqKey] = useState(false);
  const [savingDeepSeekKey, setSavingDeepSeekKey] = useState(false);
  const [deletingDeepSeekKey, setDeletingDeepSeekKey] = useState(false);
  const [savingAlibabaKey, setSavingAlibabaKey] = useState(false);
  const [deletingAlibabaKey, setDeletingAlibabaKey] = useState(false);
  const [savingMistralKey, setSavingMistralKey] = useState(false);
  const [deletingMistralKey, setDeletingMistralKey] = useState(false);
  const [savingMoonshotKey, setSavingMoonshotKey] = useState(false);
  const [deletingMoonshotKey, setDeletingMoonshotKey] = useState(false);
  const [savingXAIKey, setSavingXAIKey] = useState(false);
  const [deletingXAIKey, setDeletingXAIKey] = useState(false);
  const [savingZAIKey, setSavingZAIKey] = useState(false);
  const [deletingZAIKey, setDeletingZAIKey] = useState(false);
  const [savingFireworksKey, setSavingFireworksKey] = useState(false);
  const [deletingFireworksKey, setDeletingFireworksKey] = useState(false);
  const [savingTogetherKey, setSavingTogetherKey] = useState(false);
  const [deletingTogetherKey, setDeletingTogetherKey] = useState(false);
  const [savingPoeKey, setSavingPoeKey] = useState(false);
  const [deletingPoeKey, setDeletingPoeKey] = useState(false);
  const [savingSiliconFlowKey, setSavingSiliconFlowKey] = useState(false);
  const [deletingSiliconFlowKey, setDeletingSiliconFlowKey] = useState(false);
  const [savingAzureSpeechConfig, setSavingAzureSpeechConfig] = useState(false);
  const [deletingAzureSpeechConfig, setDeletingAzureSpeechConfig] = useState(false);
  const [savingOpenRouterKey, setSavingOpenRouterKey] = useState(false);
  const [deletingOpenRouterKey, setDeletingOpenRouterKey] = useState(false);
  const [savingAivisKey, setSavingAivisKey] = useState(false);
  const [deletingAivisKey, setDeletingAivisKey] = useState(false);
  const [savingElevenLabsKey, setSavingElevenLabsKey] = useState(false);
  const [deletingElevenLabsKey, setDeletingElevenLabsKey] = useState(false);
  const [savingFishKey, setSavingFishKey] = useState(false);
  const [deletingFishKey, setDeletingFishKey] = useState(false);
  const [savingAivisDictionary, setSavingAivisDictionary] = useState(false);
  const [deletingAivisDictionary, setDeletingAivisDictionary] = useState(false);
  const [deletingInoreaderOAuth, setDeletingInoreaderOAuth] = useState(false);
  const [resettingPreferenceProfile, setResettingPreferenceProfile] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [preferenceProfile, setPreferenceProfile] = useState<PreferenceProfile | null>(null);
  const [preferenceProfileError, setPreferenceProfileError] = useState<string | null>(null);
  const [catalog, setCatalog] = useState<LLMCatalog | null>(null);
  const [providerModelUpdates, setProviderModelUpdates] = useState<ProviderModelChangeEvent[]>([]);
  const [dismissedModelUpdatesAt, setDismissedModelUpdatesAt] = useState<string | null>(() => {
    if (typeof window === "undefined") return null;
    return window.localStorage.getItem(MODEL_UPDATES_DISMISSED_AT_KEY);
  });
  const [budgetUSD, setBudgetUSD] = useState<string>("");
  const [alertEnabled, setAlertEnabled] = useState(false);
  const [thresholdPct, setThresholdPct] = useState<number>(20);
  const [digestEmailEnabled, setDigestEmailEnabled] = useState(true);
  const [anthropicApiKeyInput, setAnthropicApiKeyInput] = useState("");
  const [openAIApiKeyInput, setOpenAIApiKeyInput] = useState("");
  const [googleApiKeyInput, setGoogleApiKeyInput] = useState("");
  const [groqApiKeyInput, setGroqApiKeyInput] = useState("");
  const [deepseekApiKeyInput, setDeepseekApiKeyInput] = useState("");
  const [alibabaApiKeyInput, setAlibabaApiKeyInput] = useState("");
  const [mistralApiKeyInput, setMistralApiKeyInput] = useState("");
  const [moonshotApiKeyInput, setMoonshotApiKeyInput] = useState("");
  const [xaiApiKeyInput, setXaiApiKeyInput] = useState("");
  const [zaiApiKeyInput, setZaiApiKeyInput] = useState("");
  const [fireworksApiKeyInput, setFireworksApiKeyInput] = useState("");
  const [togetherApiKeyInput, setTogetherApiKeyInput] = useState("");
  const [poeApiKeyInput, setPoeApiKeyInput] = useState("");
  const [siliconFlowApiKeyInput, setSiliconFlowApiKeyInput] = useState("");
  const [azureSpeechApiKeyInput, setAzureSpeechApiKeyInput] = useState("");
  const [azureSpeechRegionInput, setAzureSpeechRegionInput] = useState("");
  const [openRouterApiKeyInput, setOpenRouterApiKeyInput] = useState("");
  const [aivisApiKeyInput, setAivisApiKeyInput] = useState("");
  const [elevenLabsApiKeyInput, setElevenLabsApiKeyInput] = useState("");
  const [fishApiKeyInput, setFishApiKeyInput] = useState("");
  const [aivisUserDictionaryUUID, setAivisUserDictionaryUUID] = useState("");
  const [activeAccessProvider, setActiveAccessProvider] = useState("anthropic");
  const [activeSection, setActiveSection] = useState<SettingsSectionID>("models");
  const [uiFontSansKey, setUIFontSansKey] = useState(DEFAULT_UI_FONT_SANS_KEY);
  const [uiFontSerifKey, setUIFontSerifKey] = useState(DEFAULT_UI_FONT_SERIF_KEY);
  const [savingUIFontSettings, setSavingUIFontSettings] = useState(false);
  const [readingPlanWindow, setReadingPlanWindow] = useState<"24h" | "today_jst" | "7d">("24h");
  const [readingPlanSize, setReadingPlanSize] = useState<string>("15");
  const [readingPlanDiversifyTopics, setReadingPlanDiversifyTopics] = useState(true);
  const [audioBriefingEnabled, setAudioBriefingEnabled] = useState(false);
  const [audioBriefingScheduleSelection, setAudioBriefingScheduleSelection] = useState<AudioBriefingScheduleSelection>("interval6h");
  const [audioBriefingArticlesPerEpisode, setAudioBriefingArticlesPerEpisode] = useState("5");
  const [audioBriefingTargetDurationMinutes, setAudioBriefingTargetDurationMinutes] = useState("20");
  const [audioBriefingChunkTrailingSilenceSeconds, setAudioBriefingChunkTrailingSilenceSeconds] = useState("1.0");
  const [audioBriefingProgramName, setAudioBriefingProgramName] = useState("");
  const [audioBriefingDefaultPersonaMode, setAudioBriefingDefaultPersonaMode] = useState<"fixed" | "random">("fixed");
  const [audioBriefingDefaultPersona, setAudioBriefingDefaultPersona] = useState("editor");
  const [audioBriefingConversationMode, setAudioBriefingConversationMode] = useState<"single" | "duo">("single");
  const [audioBriefingBGMEnabled, setAudioBriefingBGMEnabled] = useState(false);
  const [audioBriefingBGMR2Prefix, setAudioBriefingBGMR2Prefix] = useState("");
  const [summaryAudioProvider, setSummaryAudioProvider] = useState("aivis");
  const [summaryAudioTTSModel, setSummaryAudioTTSModel] = useState("");
  const [summaryAudioVoiceModel, setSummaryAudioVoiceModel] = useState("");
  const [summaryAudioVoiceStyle, setSummaryAudioVoiceStyle] = useState("");
  const [summaryAudioProviderVoiceLabel, setSummaryAudioProviderVoiceLabel] = useState("");
  const [summaryAudioProviderVoiceDescription, setSummaryAudioProviderVoiceDescription] = useState("");
  const [summaryAudioSpeechRate, setSummaryAudioSpeechRate] = useState("1");
  const [summaryAudioEmotionalIntensity, setSummaryAudioEmotionalIntensity] = useState("1");
  const [summaryAudioTempoDynamics, setSummaryAudioTempoDynamics] = useState("1");
  const [summaryAudioLineBreakSilenceSeconds, setSummaryAudioLineBreakSilenceSeconds] = useState("0.4");
  const [summaryAudioPitch, setSummaryAudioPitch] = useState("0");
  const [summaryAudioVolumeGain, setSummaryAudioVolumeGain] = useState("0");
  const [summaryAudioAivisUserDictionaryUUID, setSummaryAudioAivisUserDictionaryUUID] = useState("");
  const [summaryAudioVoiceInputDrafts, setSummaryAudioVoiceInputDrafts] = useState<SummaryAudioVoiceInputDrafts>(() =>
    buildSummaryAudioVoiceInputDrafts(buildDefaultSummaryAudioVoiceSettings())
  );
  const [summaryAudioSaving, setSummaryAudioSaving] = useState(false);
  const [podcastEnabled, setPodcastEnabled] = useState(false);
  const [podcastFeedSlug, setPodcastFeedSlug] = useState("");
  const [podcastRSSURL, setPodcastRSSURL] = useState("");
  const [podcastTitle, setPodcastTitle] = useState("");
  const [podcastDescription, setPodcastDescription] = useState("");
  const [podcastAuthor, setPodcastAuthor] = useState("");
  const [podcastLanguage, setPodcastLanguage] = useState("ja");
  const [podcastCategory, setPodcastCategory] = useState("");
  const [podcastSubcategory, setPodcastSubcategory] = useState("");
  const [podcastAvailableCategories, setPodcastAvailableCategories] = useState<PodcastCategoryOption[]>([]);
  const [podcastExplicit, setPodcastExplicit] = useState(false);
  const [podcastArtworkURL, setPodcastArtworkURL] = useState("");
  const [audioBriefingVoices, setAudioBriefingVoices] = useState<AudioBriefingPersonaVoice[]>(buildDefaultAudioBriefingVoices(NAVIGATOR_PERSONA_KEYS));
  const [audioBriefingVoiceInputDrafts, setAudioBriefingVoiceInputDrafts] = useState<AudioBriefingVoiceInputDrafts>(() =>
    buildAudioBriefingVoiceInputDrafts(buildDefaultAudioBriefingVoices(NAVIGATOR_PERSONA_KEYS))
  );
  const [audioBriefingPresets, setAudioBriefingPresets] = useState<AudioBriefingPreset[]>([]);
  const [audioBriefingPresetsLoading, setAudioBriefingPresetsLoading] = useState(false);
  const [audioBriefingPresetsLoaded, setAudioBriefingPresetsLoaded] = useState(false);
  const [audioBriefingPresetsError, setAudioBriefingPresetsError] = useState<string | null>(null);
  const [aivisModelsData, setAivisModelsData] = useState<AivisModelsResponse | null>(null);
  const [aivisModelsLoading, setAivisModelsLoading] = useState(false);
  const [aivisModelsSyncing, setAivisModelsSyncing] = useState(false);
  const [aivisModelsError, setAivisModelsError] = useState<string | null>(null);
  const [xaiVoicesData, setXAIVoicesData] = useState<XAIVoicesResponse | null>(null);
  const [xaiVoicesLoading, setXAIVoicesLoading] = useState(false);
  const [xaiVoicesSyncing, setXAIVoicesSyncing] = useState(false);
  const [xaiVoicesError, setXAIVoicesError] = useState<string | null>(null);
  const [openAITTSVoicesData, setOpenAITTSVoicesData] = useState<OpenAITTSVoicesResponse | null>(null);
  const [openAITTSVoicesLoading, setOpenAITTSVoicesLoading] = useState(false);
  const [openAITTSVoicesSyncing, setOpenAITTSVoicesSyncing] = useState(false);
  const [openAITTSVoicesError, setOpenAITTSVoicesError] = useState<string | null>(null);
  const [elevenLabsVoicesData, setElevenLabsVoicesData] = useState<ElevenLabsVoicesResponse | null>(null);
  const [elevenLabsVoicesLoading, setElevenLabsVoicesLoading] = useState(false);
  const [elevenLabsVoicesError, setElevenLabsVoicesError] = useState<string | null>(null);
  const [geminiTTSVoicesData, setGeminiTTSVoicesData] = useState<GeminiTTSVoicesResponse | null>(null);
  const [geminiTTSVoicesLoading, setGeminiTTSVoicesLoading] = useState(false);
  const [geminiTTSVoicesError, setGeminiTTSVoicesError] = useState<string | null>(null);
  const [azureSpeechVoicesData, setAzureSpeechVoicesData] = useState<AzureSpeechVoicesResponse | null>(null);
  const [azureSpeechVoicesLoading, setAzureSpeechVoicesLoading] = useState(false);
  const [azureSpeechVoicesError, setAzureSpeechVoicesError] = useState<string | null>(null);
  const [aivisUserDictionaries, setAivisUserDictionaries] = useState<AivisUserDictionary[]>([]);
  const [aivisUserDictionariesLoading, setAivisUserDictionariesLoading] = useState(false);
  const [aivisUserDictionariesLoaded, setAivisUserDictionariesLoaded] = useState(false);
  const [aivisUserDictionariesError, setAivisUserDictionariesError] = useState<string | null>(null);
  const uiFontSansOptions = useMemo(() => getSelectableSansFonts(), []);
  const uiFontSerifOptions = useMemo(() => getSelectableSerifFonts(), []);
  const [expandedAudioBriefingPersonas, setExpandedAudioBriefingPersonas] = useState<string[]>(["editor"]);
  const [obsidianEnabled, setObsidianEnabled] = useState(false);
  const [notificationPriority, setNotificationPriority] = useState<NotificationPriorityRule>({
    sensitivity: "medium",
    daily_cap: 3,
    theme_weight: 1,
    immediate_enabled: true,
    briefing_enabled: true,
    review_enabled: true,
    goal_match_enabled: true,
  });
  const [obsidianRepoOwner, setObsidianRepoOwner] = useState("");
  const [obsidianRepoName, setObsidianRepoName] = useState("");
  const [obsidianRepoBranch, setObsidianRepoBranch] = useState("main");
  const [obsidianRootPath, setObsidianRootPath] = useState("Sifto/Favorites");
  const [anthropicFactsModel, setAnthropicFactsModel] = useState("");
  const [anthropicFactsSecondaryModel, setAnthropicFactsSecondaryModel] = useState("");
  const [anthropicFactsSecondaryRatePercent, setAnthropicFactsSecondaryRatePercent] = useState("0");
  const [anthropicFactsFallbackModel, setAnthropicFactsFallbackModel] = useState("");
  const [anthropicSummaryModel, setAnthropicSummaryModel] = useState("");
  const [anthropicSummarySecondaryModel, setAnthropicSummarySecondaryModel] = useState("");
  const [anthropicSummarySecondaryRatePercent, setAnthropicSummarySecondaryRatePercent] = useState("0");
  const [anthropicSummaryFallbackModel, setAnthropicSummaryFallbackModel] = useState("");
  const [anthropicDigestClusterModel, setAnthropicDigestClusterModel] = useState("");
  const [anthropicDigestModel, setAnthropicDigestModel] = useState("");
  const [anthropicAskModel, setAnthropicAskModel] = useState("");
  const [anthropicSourceSuggestionModel, setAnthropicSourceSuggestionModel] = useState("");
  const [openAIEmbeddingModel, setOpenAIEmbeddingModel] = useState("");
  const [factsCheckModel, setFactsCheckModel] = useState("");
  const [faithfulnessCheckModel, setFaithfulnessCheckModel] = useState("");
  const [navigatorEnabled, setNavigatorEnabled] = useState(false);
  const [aiNavigatorBriefEnabled, setAINavigatorBriefEnabled] = useState(false);
  const [navigatorPersonaMode, setNavigatorPersonaMode] = useState<"fixed" | "random">("fixed");
  const [navigatorPersona, setNavigatorPersona] = useState("editor");
  const dialogState = useSettingsDialogState();
  const { uiFonts, llm, presets, audioBriefingPickers, summaryAudioPickers } = dialogState;
  const [navigatorModel, setNavigatorModel] = useState("");
  const [navigatorFallbackModel, setNavigatorFallbackModel] = useState("");
  const [aiNavigatorBriefModel, setAINavigatorBriefModel] = useState("");
  const [aiNavigatorBriefFallbackModel, setAINavigatorBriefFallbackModel] = useState("");
  const [audioBriefingScriptModel, setAudioBriefingScriptModel] = useState("");
  const [audioBriefingScriptFallbackModel, setAudioBriefingScriptFallbackModel] = useState("");
  const [ttsMarkupPreprocessModel, setTTSMarkupPreprocessModel] = useState("");
  const [navigatorPersonaDefinitions, setNavigatorPersonaDefinitions] = useState<Record<string, NavigatorPersonaDefinition>>({});
  const loadSeqRef = useRef(0);
  const llmModelsDirtyRef = useRef(false);
  const uiFontsDirtyRef = useRef(false);
  const llmExtrasRef = useRef<HTMLDivElement | null>(null);
  const summaryAudioVoiceStatus = useMemo(() => {
    return getSummaryAudioVoiceStatus(
      {
        tts_provider: summaryAudioProvider,
        tts_model: summaryAudioTTSModel,
        voice_model: summaryAudioVoiceModel,
        voice_style: summaryAudioVoiceStyle,
        provider_voice_label: summaryAudioProviderVoiceLabel,
        provider_voice_description: summaryAudioProviderVoiceDescription,
        speech_rate: Number(summaryAudioSpeechRate),
        emotional_intensity: Number(summaryAudioEmotionalIntensity),
        tempo_dynamics: Number(summaryAudioTempoDynamics),
        line_break_silence_seconds: Number(summaryAudioLineBreakSilenceSeconds),
        pitch: Number(summaryAudioPitch),
        volume_gain: Number(summaryAudioVolumeGain),
        aivis_user_dictionary_uuid: summaryAudioAivisUserDictionaryUUID || null,
      },
      aivisModelsData?.models ?? [],
      [],
      xaiVoicesData?.voices ?? [],
      openAITTSVoicesData?.voices ?? [],
      geminiTTSVoicesData?.voices ?? [],
      elevenLabsVoicesData?.voices ?? [],
      azureSpeechVoicesData?.voices ?? [],
      Boolean(settings?.has_aivis_api_key),
      Boolean(settings?.has_fish_api_key),
      Boolean(settings?.has_xai_api_key),
      Boolean(settings?.has_openai_api_key),
      Boolean(settings?.has_elevenlabs_api_key),
      Boolean(settings?.has_azure_speech_api_key),
      settings?.azure_speech_region ?? "",
      Boolean(settings?.gemini_tts_enabled),
      t
    );
  }, [
    aivisModelsData?.models,
    azureSpeechVoicesData?.voices,
    elevenLabsVoicesData?.voices,
    geminiTTSVoicesData?.voices,
    openAITTSVoicesData?.voices,
    settings?.has_fish_api_key,
    settings?.has_elevenlabs_api_key,
    settings?.has_azure_speech_api_key,
    settings?.gemini_tts_enabled,
    settings?.has_aivis_api_key,
    settings?.has_openai_api_key,
    settings?.has_xai_api_key,
    settings?.azure_speech_region,
    summaryAudioAivisUserDictionaryUUID,
    summaryAudioEmotionalIntensity,
    summaryAudioLineBreakSilenceSeconds,
    summaryAudioPitch,
    summaryAudioProvider,
    summaryAudioSpeechRate,
    summaryAudioTempoDynamics,
    summaryAudioTTSModel,
    summaryAudioVoiceModel,
    summaryAudioVoiceStyle,
    summaryAudioVolumeGain,
    t,
    xaiVoicesData?.voices,
  ]);
  const summaryAudioConfigured = getSummaryAudioReadiness(settings).ready;
  const navigatorPersonaCards = useMemo(
    () =>
      NAVIGATOR_PERSONA_KEYS.map((key) => ({
        key,
        ...EMPTY_NAVIGATOR_PERSONA,
        ...(navigatorPersonaDefinitions[key] as NavigatorPersonaDefinition | undefined),
      })),
    [navigatorPersonaDefinitions]
  );
  const syncAudioBriefingVoiceForm = useCallback((voices?: UserSettings["audio_briefing_persona_voices"] | AudioBriefingPersonaVoice[] | null) => {
    const defaults = buildDefaultAudioBriefingVoices(NAVIGATOR_PERSONA_KEYS);
    const byPersona = new Map((voices ?? []).map((voice) => [voice.persona, voice] as const));
    const nextVoices = defaults.map((voice) => byPersona.get(voice.persona) ?? voice);
    setAudioBriefingVoices(nextVoices);
    setAudioBriefingVoiceInputDrafts(buildAudioBriefingVoiceInputDrafts(nextVoices));
  }, []);

  const syncAudioBriefingSettingsForm = useCallback((audioBriefing?: UserSettings["audio_briefing"] | null) => {
    setAudioBriefingEnabled(Boolean(audioBriefing?.enabled));
    setAudioBriefingScheduleSelection(resolveAudioBriefingScheduleSelection(audioBriefing));
    setAudioBriefingArticlesPerEpisode(String(audioBriefing?.articles_per_episode ?? 5));
    setAudioBriefingTargetDurationMinutes(String(audioBriefing?.target_duration_minutes ?? 20));
    setAudioBriefingChunkTrailingSilenceSeconds(formatAudioBriefingDecimalInput(audioBriefing?.chunk_trailing_silence_seconds ?? 1.0));
    setAudioBriefingProgramName(audioBriefing?.program_name ?? "");
    setAudioBriefingDefaultPersonaMode(audioBriefing?.default_persona_mode === "random" ? "random" : "fixed");
    setAudioBriefingDefaultPersona(audioBriefing?.default_persona ?? "editor");
    setAudioBriefingConversationMode(audioBriefing?.conversation_mode === "duo" ? "duo" : "single");
    setAudioBriefingBGMEnabled(Boolean(audioBriefing?.bgm_enabled));
    setAudioBriefingBGMR2Prefix(audioBriefing?.bgm_r2_prefix ?? "");
  }, []);

  const syncAudioBriefingForm = useCallback(
    (audioBriefing?: UserSettings["audio_briefing"] | null, voices?: UserSettings["audio_briefing_persona_voices"] | null) => {
      syncAudioBriefingSettingsForm(audioBriefing);
      syncAudioBriefingVoiceForm(voices);
    },
    [syncAudioBriefingSettingsForm, syncAudioBriefingVoiceForm]
  );

  const syncAudioBriefingPresetForm = useCallback((preset: AudioBriefingPreset) => {
    syncAudioBriefingSettingsForm({
      enabled: audioBriefingEnabled,
      schedule_mode: audioBriefingScheduleSelection === "fixed3x" ? "fixed_slots_3x" : "interval",
      interval_hours: audioBriefingScheduleSelection === "interval3h" ? 3 : 6,
      articles_per_episode: Number(audioBriefingArticlesPerEpisode),
      target_duration_minutes: Number(audioBriefingTargetDurationMinutes),
      chunk_trailing_silence_seconds: Number(audioBriefingChunkTrailingSilenceSeconds),
      program_name: audioBriefingProgramName.trim() || null,
      default_persona_mode: preset.default_persona_mode === "random" ? "random" : "fixed",
      default_persona: preset.default_persona || "editor",
      conversation_mode: preset.conversation_mode === "duo" ? "duo" : "single",
      bgm_enabled: audioBriefingBGMEnabled,
      bgm_r2_prefix: audioBriefingBGMR2Prefix.trim() || null,
    });
    syncAudioBriefingVoiceForm(normalizeAudioBriefingPresetVoices(preset.voices ?? []));
    setExpandedAudioBriefingPersonas((prev) =>
      prev.includes(preset.default_persona) ? prev : [preset.default_persona]
    );
  }, [
    audioBriefingArticlesPerEpisode,
    audioBriefingBGMEnabled,
    audioBriefingBGMR2Prefix,
    audioBriefingChunkTrailingSilenceSeconds,
    audioBriefingEnabled,
    audioBriefingProgramName,
    audioBriefingScheduleSelection,
    audioBriefingTargetDurationMinutes,
    syncAudioBriefingSettingsForm,
    syncAudioBriefingVoiceForm,
  ]);

  const syncSummaryAudioForm = useCallback((summaryAudio?: UserSettings["summary_audio"] | null) => {
    const next = summaryAudio ?? buildDefaultSummaryAudioVoiceSettings();
    setSummaryAudioProvider(next.tts_provider || "aivis");
    setSummaryAudioTTSModel(next.tts_model ?? "");
    setSummaryAudioVoiceModel(next.voice_model ?? "");
    setSummaryAudioVoiceStyle(next.voice_style ?? "");
    setSummaryAudioProviderVoiceLabel(next.provider_voice_label ?? "");
    setSummaryAudioProviderVoiceDescription(next.provider_voice_description ?? "");
    setSummaryAudioSpeechRate(formatAudioBriefingDecimalInput(next.speech_rate ?? 1));
    setSummaryAudioEmotionalIntensity(formatAudioBriefingDecimalInput(next.emotional_intensity ?? 1));
    setSummaryAudioTempoDynamics(formatAudioBriefingDecimalInput(next.tempo_dynamics ?? 1));
    setSummaryAudioLineBreakSilenceSeconds(formatAudioBriefingDecimalInput(next.line_break_silence_seconds ?? 0.4));
    setSummaryAudioPitch(formatAudioBriefingDecimalInput(next.pitch ?? 0));
    setSummaryAudioVolumeGain(formatAudioBriefingDecimalInput(next.volume_gain ?? 0));
    setSummaryAudioAivisUserDictionaryUUID(next.aivis_user_dictionary_uuid ?? "");
    setSummaryAudioVoiceInputDrafts(buildSummaryAudioVoiceInputDrafts(next));
  }, []);

  const syncUIFontForm = useCallback((nextSettings?: UserSettings | null) => {
    setUIFontSansKey(nextSettings?.ui_font_sans_key?.trim() || DEFAULT_UI_FONT_SANS_KEY);
    setUIFontSerifKey(nextSettings?.ui_font_serif_key?.trim() || DEFAULT_UI_FONT_SERIF_KEY);
  }, []);

  const loadAudioBriefingPresets = useCallback(async () => {
    setAudioBriefingPresetsLoading(true);
    setAudioBriefingPresetsError(null);
    try {
      const presets = await api.listAudioBriefingPresets();
      setAudioBriefingPresets(presets ?? []);
    } catch (e) {
      setAudioBriefingPresets([]);
      setAudioBriefingPresetsError(String(e));
    } finally {
      setAudioBriefingPresetsLoaded(true);
      setAudioBriefingPresetsLoading(false);
    }
  }, []);

  useEffect(() => {
    const section = searchParams.get("section");
    if (isSettingsSectionID(section)) {
      setActiveSection(section);
      return;
    }
    setActiveSection("models");
  }, [searchParams]);

  const syncPodcastForm = useCallback((podcast?: UserSettings["podcast"] | null) => {
    setPodcastEnabled(Boolean(podcast?.enabled));
    setPodcastFeedSlug(podcast?.feed_slug ?? "");
    setPodcastRSSURL(buildPodcastRSSURL(podcast?.feed_slug, podcast?.rss_url));
    setPodcastTitle(podcast?.title ?? "");
    setPodcastDescription(podcast?.description ?? "");
    setPodcastAuthor(podcast?.author ?? "");
    setPodcastLanguage(podcast?.language ?? "ja");
    setPodcastCategory(podcast?.category ?? "");
    setPodcastSubcategory(podcast?.subcategory ?? "");
    setPodcastAvailableCategories(podcast?.available_categories ?? []);
    setPodcastExplicit(Boolean(podcast?.explicit));
    setPodcastArtworkURL(podcast?.artwork_url ?? "");
  }, []);

  const selectedPodcastCategory = useMemo(
    () => podcastAvailableCategories.find((option) => option.category === podcastCategory) ?? null,
    [podcastAvailableCategories, podcastCategory]
  );

  const syncLLMModelForm = useCallback((llmModels?: UserSettings["llm_models"] | null) => {
    setAnthropicFactsModel(llmModels?.facts ?? "");
    setAnthropicFactsSecondaryModel(llmModels?.facts_secondary ?? "");
    setAnthropicFactsSecondaryRatePercent(String(llmModels?.facts_secondary_rate_percent ?? 0));
    setAnthropicFactsFallbackModel(llmModels?.facts_fallback ?? "");
    setAnthropicSummaryModel(llmModels?.summary ?? "");
    setAnthropicSummarySecondaryModel(llmModels?.summary_secondary ?? "");
    setAnthropicSummarySecondaryRatePercent(String(llmModels?.summary_secondary_rate_percent ?? 0));
    setAnthropicSummaryFallbackModel(llmModels?.summary_fallback ?? "");
    setAnthropicDigestClusterModel(llmModels?.digest_cluster ?? "");
    setAnthropicDigestModel(llmModels?.digest ?? "");
    setAnthropicAskModel(llmModels?.ask ?? "");
    setAnthropicSourceSuggestionModel(llmModels?.source_suggestion ?? "");
    setOpenAIEmbeddingModel(llmModels?.embedding ?? "");
    setFactsCheckModel(llmModels?.facts_check ?? "");
    setFaithfulnessCheckModel(llmModels?.faithfulness_check ?? "");
    setNavigatorEnabled(Boolean(llmModels?.navigator_enabled ?? false));
    setAINavigatorBriefEnabled(Boolean(llmModels?.ai_navigator_brief_enabled ?? false));
    setNavigatorPersonaMode(llmModels?.navigator_persona_mode === "random" ? "random" : "fixed");
    setNavigatorPersona(llmModels?.navigator_persona ?? "editor");
    setNavigatorModel(llmModels?.navigator ?? "");
    setNavigatorFallbackModel(llmModels?.navigator_fallback ?? "");
    setAINavigatorBriefModel(llmModels?.ai_navigator_brief ?? "");
    setAINavigatorBriefFallbackModel(llmModels?.ai_navigator_brief_fallback ?? "");
    setAudioBriefingScriptModel(llmModels?.audio_briefing_script ?? "");
    setAudioBriefingScriptFallbackModel(llmModels?.audio_briefing_script_fallback ?? "");
    setTTSMarkupPreprocessModel(llmModels?.tts_markup_preprocess_model ?? "");
  }, []);

  const onChangeLLMModel = useCallback((setter: (value: string) => void, value: string) => {
    llmModelsDirtyRef.current = true;
    setter(value);
  }, []);

  const buildLLMModelPayload = useCallback(
    (overrides?: Partial<{
      facts: string | null;
      facts_secondary: string | null;
      facts_secondary_rate_percent: number;
      facts_fallback: string | null;
      summary: string | null;
      summary_secondary: string | null;
      summary_secondary_rate_percent: number;
      summary_fallback: string | null;
      digest_cluster: string | null;
      digest: string | null;
      ask: string | null;
      source_suggestion: string | null;
      embedding: string | null;
      facts_check: string | null;
      faithfulness_check: string | null;
      navigator_enabled: boolean;
      ai_navigator_brief_enabled: boolean;
      navigator_persona_mode: string | null;
      navigator_persona: string | null;
      navigator: string | null;
      navigator_fallback: string | null;
      ai_navigator_brief: string | null;
      ai_navigator_brief_fallback: string | null;
      audio_briefing_script: string | null;
      audio_briefing_script_fallback: string | null;
      tts_markup_preprocess_model: string | null;
    }>) => {
      const emptyToNull = (v: string) => {
        const s = v.trim();
        return s === "" ? null : s;
      };
      const normalizeRate = (v: string) => {
        const n = Number(v);
        if (!Number.isFinite(n)) return 0;
        return Math.min(100, Math.max(0, Math.round(n)));
      };
      return {
        facts: emptyToNull(anthropicFactsModel),
        facts_secondary: emptyToNull(anthropicFactsSecondaryModel),
        facts_secondary_rate_percent: normalizeRate(anthropicFactsSecondaryRatePercent),
        facts_fallback: emptyToNull(anthropicFactsFallbackModel),
        summary: emptyToNull(anthropicSummaryModel),
        summary_secondary: emptyToNull(anthropicSummarySecondaryModel),
        summary_secondary_rate_percent: normalizeRate(anthropicSummarySecondaryRatePercent),
        summary_fallback: emptyToNull(anthropicSummaryFallbackModel),
        digest_cluster: emptyToNull(anthropicDigestClusterModel),
        digest: emptyToNull(anthropicDigestModel),
        ask: emptyToNull(anthropicAskModel),
        source_suggestion: emptyToNull(anthropicSourceSuggestionModel),
        embedding: emptyToNull(openAIEmbeddingModel),
        facts_check: emptyToNull(factsCheckModel),
        faithfulness_check: emptyToNull(faithfulnessCheckModel),
        navigator_enabled: navigatorEnabled,
        ai_navigator_brief_enabled: aiNavigatorBriefEnabled,
        navigator_persona_mode: navigatorPersonaMode,
        navigator_persona: navigatorPersona,
        navigator: emptyToNull(navigatorModel),
        navigator_fallback: emptyToNull(navigatorFallbackModel),
        ai_navigator_brief: emptyToNull(aiNavigatorBriefModel),
        ai_navigator_brief_fallback: emptyToNull(aiNavigatorBriefFallbackModel),
        audio_briefing_script: emptyToNull(audioBriefingScriptModel),
        audio_briefing_script_fallback: emptyToNull(audioBriefingScriptFallbackModel),
        tts_markup_preprocess_model: emptyToNull(ttsMarkupPreprocessModel),
        ...overrides,
      };
    },
    [
      anthropicAskModel,
      anthropicDigestClusterModel,
      anthropicDigestModel,
      anthropicFactsFallbackModel,
      anthropicFactsModel,
      anthropicFactsSecondaryModel,
      anthropicFactsSecondaryRatePercent,
      anthropicSourceSuggestionModel,
      anthropicSummaryFallbackModel,
      anthropicSummaryModel,
      anthropicSummarySecondaryModel,
      anthropicSummarySecondaryRatePercent,
      factsCheckModel,
      faithfulnessCheckModel,
      aiNavigatorBriefEnabled,
      aiNavigatorBriefFallbackModel,
      aiNavigatorBriefModel,
      navigatorEnabled,
      navigatorFallbackModel,
      navigatorModel,
      navigatorPersonaMode,
      navigatorPersona,
      audioBriefingScriptFallbackModel,
      audioBriefingScriptModel,
      ttsMarkupPreprocessModel,
      openAIEmbeddingModel,
    ]
  );

  const persistLLMModels = useCallback(
    async (
      payload: ReturnType<typeof buildLLMModelPayload>,
      successMessage?: string
    ) => {
      const resp = await api.updateLLMModelSettings(payload);
      let nextSettingsSnapshot: UserSettings | null = null;
      setSettings((prev) => {
        if (!prev) return prev;
        const next = {
          ...prev,
          llm_models: {
            ...prev.llm_models,
            ...resp.llm_models,
          },
        };
        nextSettingsSnapshot = next;
        return next;
      });
      if (nextSettingsSnapshot) {
        queryClient.setQueryData(queryKeys.settings.all(), nextSettingsSnapshot);
      }
      syncLLMModelForm(resp.llm_models);
      llmModelsDirtyRef.current = false;
      if (successMessage) {
        showToast(successMessage, "success");
      }
      return resp;
    },
    [queryClient, showToast, syncLLMModelForm]
  );

  const loadAivisModels = useCallback(async () => {
    return loadResourceAction({
      setLoading: setAivisModelsLoading,
      fetch: api.getAivisModels,
      setData: setAivisModelsData,
      setError: setAivisModelsError,
    });
  }, []);

  const loadXAIVoices = useCallback(async () => {
    return loadResourceAction({
      setLoading: setXAIVoicesLoading,
      fetch: api.getXAIVoices,
      setData: setXAIVoicesData,
      setError: setXAIVoicesError,
    });
  }, []);

  const loadOpenAITTSVoices = useCallback(async () => {
    return loadResourceAction({
      setLoading: setOpenAITTSVoicesLoading,
      fetch: api.getOpenAITTSVoices,
      setData: setOpenAITTSVoicesData,
      setError: setOpenAITTSVoicesError,
    });
  }, []);

  const loadElevenLabsVoices = useCallback(async () => {
    return loadResourceAction({
      setLoading: setElevenLabsVoicesLoading,
      fetch: api.getElevenLabsVoices,
      setData: setElevenLabsVoicesData,
      setError: setElevenLabsVoicesError,
    });
  }, []);

  const loadGeminiTTSVoices = useCallback(async () => {
    return loadResourceAction({
      setLoading: setGeminiTTSVoicesLoading,
      fetch: api.getGeminiTTSVoices,
      setData: setGeminiTTSVoicesData,
      setError: setGeminiTTSVoicesError,
    });
  }, []);

  const loadAzureSpeechVoices = useCallback(async () => {
    return loadResourceAction({
      setLoading: setAzureSpeechVoicesLoading,
      fetch: api.getAzureSpeechVoices,
      setData: setAzureSpeechVoicesData,
      setError: setAzureSpeechVoicesError,
    });
  }, []);

  const syncAivisModels = useCallback(async () => {
    return syncResourceAction({
      setSyncing: setAivisModelsSyncing,
      sync: api.syncAivisModels,
      setData: setAivisModelsData,
      setError: setAivisModelsError,
      showToast,
      successMessage: t("aivisModels.syncCompleted"),
    });
  }, [showToast, t]);

  const syncXAIVoices = useCallback(async () => {
    return syncResourceAction({
      setSyncing: setXAIVoicesSyncing,
      sync: api.syncXAIVoices,
      setData: setXAIVoicesData,
      setError: setXAIVoicesError,
      showToast,
      successMessage: t("settings.audioBriefing.xaiSyncCompleted"),
    });
  }, [showToast, t]);

  const syncOpenAITTSVoices = useCallback(async () => {
    return syncResourceAction({
      setSyncing: setOpenAITTSVoicesSyncing,
      sync: api.syncOpenAITTSVoices,
      setData: setOpenAITTSVoicesData,
      setError: setOpenAITTSVoicesError,
      showToast,
      successMessage: t("settings.audioBriefing.openAITTSSyncCompleted"),
    });
  }, [showToast, t]);

  const loadAivisUserDictionaries = useCallback(async (force = false) => {
    if (!force && aivisUserDictionariesLoaded) {
      return aivisUserDictionaries;
    }
    setAivisUserDictionariesLoading(true);
    try {
      const next = await api.getAivisUserDictionaries();
      setAivisUserDictionaries(next.user_dictionaries ?? []);
      setAivisUserDictionariesLoaded(true);
      setAivisUserDictionariesError(null);
      return next.user_dictionaries ?? [];
    } catch (e) {
      const message = String(e);
      setAivisUserDictionariesError(message);
      if (force) {
        showToast(message, "error");
      }
      throw e;
    } finally {
      setAivisUserDictionariesLoading(false);
    }
  }, [aivisUserDictionaries, aivisUserDictionariesLoaded, showToast]);

  const load = useCallback(async () => {
    const seq = ++loadSeqRef.current;
    setLoading(true);
    setError(null);
    try {
      let data: UserSettings | null = null;
      try { data = await api.getSettings(); } catch { /* partial failure */ }
      let nextCatalog: LLMCatalog | null = null;
      try { nextCatalog = await api.getLLMCatalog(); } catch { /* partial failure */ }
      let navigatorPersonas: Record<string, NavigatorPersonaDefinition> | null = null;
      try { navigatorPersonas = await api.getNavigatorPersonas(); } catch { /* partial failure */ }
      let preferenceProfileResult: { profile: PreferenceProfile | null; error: string | null } = { profile: null, error: null };
      try {
        const profile = await api.getPreferenceProfile();
        preferenceProfileResult = { profile, error: null };
      } catch (profileError) {
        preferenceProfileResult = { profile: null, error: localizePreferenceProfileErrorMessage(profileError, t) };
      }
      if (seq !== loadSeqRef.current) return;
      if (!data) {
        setError(t("settings.loadFailed"));
        return;
      }
      if (data) {
        setSettings(data);
        setAivisUserDictionaryUUID(data.aivis_user_dictionary_uuid ?? "");
        if (!data.has_aivis_api_key) {
          setAivisUserDictionaries([]);
          setAivisUserDictionariesLoaded(false);
          setAivisUserDictionariesError(null);
        }
        if (!data.has_elevenlabs_api_key) {
          setElevenLabsVoicesData(null);
          setElevenLabsVoicesError(null);
        }
        if (!data.has_xai_api_key) {
          setXAIVoicesData(null);
          setXAIVoicesError(null);
        }
        setBudgetUSD(data.monthly_budget_usd == null ? "" : String(data.monthly_budget_usd));
        setAlertEnabled(Boolean(data.budget_alert_enabled));
        setThresholdPct(data.budget_alert_threshold_pct ?? 20);
        setDigestEmailEnabled(Boolean(data.digest_email_enabled ?? true));
        syncAudioBriefingForm(data.audio_briefing, data.audio_briefing_persona_voices);
        syncPodcastForm(data.podcast);
        setReadingPlanWindow((data.reading_plan?.window as "24h" | "today_jst" | "7d") ?? "24h");
        const rpSize = data.reading_plan?.size;
        setReadingPlanSize(String(rpSize === 7 || rpSize === 15 || rpSize === 25 ? rpSize : 15));
        setReadingPlanDiversifyTopics(Boolean(data.reading_plan?.diversify_topics ?? true));
        setObsidianEnabled(Boolean(data.obsidian_export?.enabled));
        setNotificationPriority(data.notification_priority ?? { sensitivity: "medium", daily_cap: 3, theme_weight: 1, immediate_enabled: true, briefing_enabled: true, review_enabled: true, goal_match_enabled: true });
        setObsidianRepoOwner(data.obsidian_export?.github_repo_owner ?? "");
        setObsidianRepoName(data.obsidian_export?.github_repo_name ?? "");
        setObsidianRepoBranch(data.obsidian_export?.github_repo_branch ?? "main");
        setObsidianRootPath(data.obsidian_export?.vault_root_path ?? "Sifto/Favorites");
        syncSummaryAudioForm(data.summary_audio ?? null);
        if (!uiFontsDirtyRef.current) {
          syncUIFontForm(data);
        }
        if (!llmModelsDirtyRef.current) {
          syncLLMModelForm(data.llm_models);
        }
      }
      if (nextCatalog) setCatalog(nextCatalog);
      if (navigatorPersonas) setNavigatorPersonaDefinitions(navigatorPersonas);
      setPreferenceProfile(preferenceProfileResult.profile);
      setPreferenceProfileError(preferenceProfileResult.error);
      setError(null);
    } catch (e) {
      if (seq !== loadSeqRef.current) return;
      setError(String(e));
    } finally {
      if (seq === loadSeqRef.current) {
        setLoading(false);
      }
    }
  }, [syncAudioBriefingForm, syncLLMModelForm, syncPodcastForm, syncSummaryAudioForm, syncUIFontForm, t]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && !presets.audioBriefingPresetSaveOpen && !presets.audioBriefingPresetApplyOpen) {
      return;
    }
    if (audioBriefingPresetsLoaded || audioBriefingPresetsLoading || audioBriefingPresets.length > 0) {
      return;
    }
    void loadAudioBriefingPresets().catch(() => undefined);
  }, [
    activeSection,
    presets.audioBriefingPresetApplyOpen,
    presets.audioBriefingPresetSaveOpen,
    audioBriefingPresets.length,
    audioBriefingPresetsLoading,
    audioBriefingPresetsLoaded,
    loadAudioBriefingPresets,
  ]);

  useEffect(() => {
    ensureUIFontPreviewLoaded(uiFontSansKey);
  }, [uiFontSansKey]);

  useEffect(() => {
    ensureUIFontPreviewLoaded(uiFontSerifKey);
  }, [uiFontSerifKey]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || aivisModelsData != null || aivisModelsLoading) return;
    void loadAivisModels().catch(() => undefined);
  }, [activeSection, aivisModelsData, aivisModelsLoading, loadAivisModels]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || !settings?.has_xai_api_key || xaiVoicesData != null || xaiVoicesLoading) {
      return;
    }
    void loadXAIVoices().catch(() => undefined);
  }, [activeSection, loadXAIVoices, settings?.has_xai_api_key, xaiVoicesData, xaiVoicesLoading]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || !settings?.has_openai_api_key || openAITTSVoicesData != null || openAITTSVoicesLoading) {
      return;
    }
    void loadOpenAITTSVoices().catch(() => undefined);
  }, [activeSection, loadOpenAITTSVoices, openAITTSVoicesData, openAITTSVoicesLoading, settings?.has_openai_api_key]);

  useEffect(() => {
    if (
      (activeSection !== "audio-briefing" && activeSection !== "summary-audio")
      || !settings?.has_elevenlabs_api_key
      || elevenLabsVoicesData != null
      || elevenLabsVoicesLoading
    ) {
      return;
    }
    void loadElevenLabsVoices().catch(() => undefined);
  }, [
    activeSection,
    elevenLabsVoicesData,
    elevenLabsVoicesLoading,
    loadElevenLabsVoices,
    settings?.has_elevenlabs_api_key,
  ]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || geminiTTSVoicesData != null || geminiTTSVoicesLoading) {
      return;
    }
    void loadGeminiTTSVoices().catch(() => undefined);
  }, [activeSection, geminiTTSVoicesData, geminiTTSVoicesLoading, loadGeminiTTSVoices]);

  useEffect(() => {
    if ((activeSection !== "audio-briefing" && activeSection !== "summary-audio") || !settings?.has_azure_speech_api_key || !settings?.azure_speech_region?.trim() || azureSpeechVoicesData != null || azureSpeechVoicesLoading) {
      return;
    }
    void loadAzureSpeechVoices().catch(() => undefined);
  }, [activeSection, azureSpeechVoicesData, azureSpeechVoicesLoading, loadAzureSpeechVoices, settings?.has_azure_speech_api_key, settings?.azure_speech_region]);

  useEffect(() => {
    if (activeSection !== "audio-briefing" && activeSection !== "summary-audio" || !settings?.has_aivis_api_key || aivisUserDictionariesLoading || aivisUserDictionariesLoaded) {
      return;
    }
    void loadAivisUserDictionaries().catch(() => undefined);
  }, [
    activeSection,
    aivisUserDictionariesLoaded,
    aivisUserDictionariesLoading,
    loadAivisUserDictionaries,
    settings?.has_aivis_api_key,
  ]);

  useEffect(() => {
    setExpandedAudioBriefingPersonas((prev) => {
      if (prev.length === 0) return [audioBriefingDefaultPersona];
      if (prev.length === 1 && prev[0] === "editor" && audioBriefingDefaultPersona !== "editor") {
        return [audioBriefingDefaultPersona];
      }
      return prev;
    });
  }, [audioBriefingDefaultPersona]);

  useEffect(() => {
    if (audioBriefingConversationMode !== "duo") {
      return;
    }
    setAudioBriefingVoices((prev) =>
      prev.map((voice) =>
        voice.tts_provider === "elevenlabs" && voice.tts_model !== "eleven_v3"
          ? { ...voice, tts_model: "eleven_v3" }
          : voice,
      ),
    );
  }, [audioBriefingConversationMode]);

  useEffect(() => {
    setPodcastRSSURL(buildPodcastRSSURL(podcastFeedSlug, podcastRSSURL));
  }, [podcastFeedSlug]);

  useEffect(() => {
    let cancelled = false;
    api.getProviderModelUpdates({ days: 14, limit: 20 })
      .then((modelUpdates) => {
        if (cancelled) return;
        setProviderModelUpdates(modelUpdates ?? []);
      })
      .catch(() => {
        if (cancelled) return;
        setProviderModelUpdates([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const inoreaderStatus = params.get("inoreader");
    if (inoreaderStatus === "connected") {
      showToast(t("settings.toast.inoreaderConnected"), "success");
    } else if (inoreaderStatus === "error") {
      showToast(t("settings.toast.inoreaderConnectError"), "error");
    }
    const obsidianStatus = params.get("obsidian_github");
    if (obsidianStatus === "connected") {
      showToast(t("settings.toast.obsidianGithubConnected"), "success");
    } else if (obsidianStatus === "error") {
      showToast(t("settings.toast.obsidianGithubConnectError"), "error");
    }
    if (inoreaderStatus || obsidianStatus) {
      params.delete("inoreader");
      params.delete("obsidian_github");
      const qs = params.toString();
      const nextURL = `${window.location.pathname}${qs ? `?${qs}` : ""}${window.location.hash}`;
      window.history.replaceState(null, "", nextURL);
    }
  }, [showToast, t]);

  const modelSelectLabels = useMemo(() => buildModelSelectLabels(t), [t]);

  const applyCostPerformancePreset = useCallback(() => {
    const preset = buildCostPerformancePreset(catalog);
    llmModelsDirtyRef.current = true;
    setAnthropicFactsModel(preset.facts ?? "");
    setAnthropicFactsSecondaryModel("");
    setAnthropicFactsSecondaryRatePercent("0");
    setAnthropicFactsFallbackModel("");
    setAnthropicSummaryModel(preset.summary ?? "");
    setAnthropicSummarySecondaryModel("");
    setAnthropicSummarySecondaryRatePercent("0");
    setAnthropicSummaryFallbackModel("");
    setAnthropicDigestClusterModel(preset.digest_cluster ?? "");
    setAnthropicDigestModel(preset.digest ?? "");
    setAnthropicAskModel(preset.ask ?? "");
    setAnthropicSourceSuggestionModel(preset.source_suggestion ?? "");
    setOpenAIEmbeddingModel(preset.embedding ?? "");
    setFactsCheckModel(preset.facts_check ?? "");
    setFaithfulnessCheckModel(preset.faithfulness_check ?? "");
    setTTSMarkupPreprocessModel(preset.tts_markup_preprocess_model ?? "");
  }, [catalog]);

  const optionsForPurpose = useCallback(
    (purpose: string, currentValue?: string): ModelOption[] => buildOptionsForPurpose(catalog, purpose, currentValue, t),
    [catalog, t],
  );

  const optionsForChatModel = useCallback(
    (currentValue?: string): ModelOption[] => buildOptionsForChatModel(catalog, currentValue, t),
    [catalog, t],
  );

  const unavailableSelectedModelWarnings = useMemo(
    () => buildUnavailableSelectedModelWarnings(catalog, settings?.llm_models, t),
    [catalog, settings?.llm_models, t],
  );

  const sourceSuggestionModelOptions = useMemo(
    () => optionsForPurpose("source_suggestion", anthropicSourceSuggestionModel),
    [anthropicSourceSuggestionModel, optionsForPurpose]
  );
  const openAIEmbeddingModelOptions = useMemo(() => buildEmbeddingModelOptions(catalog, t), [catalog, t]);
  const modelComparisonEntries = useMemo(() => buildModelComparisonEntries(catalog), [catalog]);
  const visibleProviderModelUpdates = useMemo(
    () => buildVisibleProviderModelUpdates(providerModelUpdates, dismissedModelUpdatesAt),
    [dismissedModelUpdatesAt, providerModelUpdates],
  );

  const budgetRemainingTone = useMemo(() => {
    const v = settings?.current_month.remaining_budget_pct;
    if (v == null) return "text-zinc-700";
    if (v < 0) return "text-red-600";
    if (v < thresholdPct) return "text-amber-600";
    return "text-zinc-700";
  }, [settings?.current_month.remaining_budget_pct, thresholdPct]);

  const apiKeyHandlers = createAPIKeyActionHandlers({
    confirm,
    confirmLabel: t("settings.delete"),
    reload: load,
    showToast,
    definitions: {
      anthropic: {
        value: anthropicApiKeyInput, setValue: setAnthropicApiKeyInput, setSaving: setSavingAnthropicKey, setDeleting: setDeletingAnthropicKey,
        save: api.setAnthropicApiKey, remove: api.deleteAnthropicApiKey,
        deleteTitle: t("settings.anthropicDeleteTitle"), deleteMessage: t("settings.anthropicDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.anthropicSaved"), deleteSuccessMessage: t("settings.toast.anthropicDeleted"),
      },
      openai: {
        value: openAIApiKeyInput, setValue: setOpenAIApiKeyInput, setSaving: setSavingOpenAIKey, setDeleting: setDeletingOpenAIKey,
        save: api.setOpenAIApiKey, remove: api.deleteOpenAIApiKey,
        deleteTitle: t("settings.openaiDeleteTitle"), deleteMessage: t("settings.openaiDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.openaiSaved"), deleteSuccessMessage: t("settings.toast.openaiDeleted"),
      },
      google: {
        value: googleApiKeyInput, setValue: setGoogleApiKeyInput, setSaving: setSavingGoogleKey, setDeleting: setDeletingGoogleKey,
        save: api.setGoogleApiKey, remove: api.deleteGoogleApiKey,
        deleteTitle: t("settings.googleDeleteTitle"), deleteMessage: t("settings.googleDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.googleSaved"), deleteSuccessMessage: t("settings.toast.googleDeleted"),
      },
      groq: {
        value: groqApiKeyInput, setValue: setGroqApiKeyInput, setSaving: setSavingGroqKey, setDeleting: setDeletingGroqKey,
        save: api.setGroqApiKey, remove: api.deleteGroqApiKey,
        deleteTitle: t("settings.groqDeleteTitle"), deleteMessage: t("settings.groqDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.groqSaved"), deleteSuccessMessage: t("settings.toast.groqDeleted"),
      },
      deepseek: {
        value: deepseekApiKeyInput, setValue: setDeepseekApiKeyInput, setSaving: setSavingDeepSeekKey, setDeleting: setDeletingDeepSeekKey,
        save: api.setDeepSeekApiKey, remove: api.deleteDeepSeekApiKey,
        deleteTitle: t("settings.deepseekDeleteTitle"), deleteMessage: t("settings.deepseekDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.deepseekSaved"), deleteSuccessMessage: t("settings.toast.deepseekDeleted"),
      },
      alibaba: {
        value: alibabaApiKeyInput, setValue: setAlibabaApiKeyInput, setSaving: setSavingAlibabaKey, setDeleting: setDeletingAlibabaKey,
        save: api.setAlibabaApiKey, remove: api.deleteAlibabaApiKey,
        deleteTitle: t("settings.alibabaDeleteTitle"), deleteMessage: t("settings.alibabaDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.alibabaSaved"), deleteSuccessMessage: t("settings.toast.alibabaDeleted"),
      },
      mistral: {
        value: mistralApiKeyInput, setValue: setMistralApiKeyInput, setSaving: setSavingMistralKey, setDeleting: setDeletingMistralKey,
        save: api.setMistralApiKey, remove: api.deleteMistralApiKey,
        deleteTitle: t("settings.mistralDeleteTitle"), deleteMessage: t("settings.mistralDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.mistralSaved"), deleteSuccessMessage: t("settings.toast.mistralDeleted"),
      },
      moonshot: {
        value: moonshotApiKeyInput, setValue: setMoonshotApiKeyInput, setSaving: setSavingMoonshotKey, setDeleting: setDeletingMoonshotKey,
        save: api.setMoonshotApiKey, remove: api.deleteMoonshotApiKey,
        deleteTitle: t("settings.moonshotDeleteTitle"), deleteMessage: t("settings.moonshotDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.moonshotSaved"), deleteSuccessMessage: t("settings.toast.moonshotDeleted"),
      },
      xai: {
        value: xaiApiKeyInput, setValue: setXaiApiKeyInput, setSaving: setSavingXAIKey, setDeleting: setDeletingXAIKey,
        save: api.setXAIApiKey, remove: api.deleteXAIApiKey,
        deleteTitle: t("settings.xaiDeleteTitle"), deleteMessage: t("settings.xaiDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.xaiSaved"), deleteSuccessMessage: t("settings.toast.xaiDeleted"),
        afterSave: () => { setXAIVoicesData(null); setXAIVoicesError(null); },
        afterDelete: () => { setXAIVoicesData(null); setXAIVoicesError(null); },
      },
      zai: {
        value: zaiApiKeyInput, setValue: setZaiApiKeyInput, setSaving: setSavingZAIKey, setDeleting: setDeletingZAIKey,
        save: api.setZAIApiKey, remove: api.deleteZAIApiKey,
        deleteTitle: t("settings.zaiDeleteTitle"), deleteMessage: t("settings.zaiDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.zaiSaved"), deleteSuccessMessage: t("settings.toast.zaiDeleted"),
      },
      fireworks: {
        value: fireworksApiKeyInput, setValue: setFireworksApiKeyInput, setSaving: setSavingFireworksKey, setDeleting: setDeletingFireworksKey,
        save: api.setFireworksApiKey, remove: api.deleteFireworksApiKey,
        deleteTitle: t("settings.fireworksDeleteTitle"), deleteMessage: t("settings.fireworksDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.fireworksSaved"), deleteSuccessMessage: t("settings.toast.fireworksDeleted"),
      },
      together: {
        value: togetherApiKeyInput, setValue: setTogetherApiKeyInput, setSaving: setSavingTogetherKey, setDeleting: setDeletingTogetherKey,
        save: api.setTogetherApiKey, remove: api.deleteTogetherApiKey,
        deleteTitle: t("settings.togetherDeleteTitle"), deleteMessage: t("settings.togetherDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.togetherSaved"), deleteSuccessMessage: t("settings.toast.togetherDeleted"),
      },
      poe: {
        value: poeApiKeyInput, setValue: setPoeApiKeyInput, setSaving: setSavingPoeKey, setDeleting: setDeletingPoeKey,
        save: api.setPoeApiKey, remove: api.deletePoeApiKey,
        deleteTitle: t("settings.poeDeleteTitle"), deleteMessage: t("settings.poeDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.poeSaved"), deleteSuccessMessage: t("settings.toast.poeDeleted"),
      },
      siliconflow: {
        value: siliconFlowApiKeyInput, setValue: setSiliconFlowApiKeyInput, setSaving: setSavingSiliconFlowKey, setDeleting: setDeletingSiliconFlowKey,
        save: api.setSiliconFlowApiKey, remove: api.deleteSiliconFlowApiKey,
        deleteTitle: t("settings.siliconflowDeleteTitle"), deleteMessage: t("settings.siliconflowDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.siliconflowSaved"), deleteSuccessMessage: t("settings.toast.siliconflowDeleted"),
      },
      openrouter: {
        value: openRouterApiKeyInput, setValue: setOpenRouterApiKeyInput, setSaving: setSavingOpenRouterKey, setDeleting: setDeletingOpenRouterKey,
        save: api.setOpenRouterApiKey, remove: api.deleteOpenRouterApiKey,
        deleteTitle: t("settings.openrouterDeleteTitle"), deleteMessage: t("settings.openrouterDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.openrouterSaved"), deleteSuccessMessage: t("settings.toast.openrouterDeleted"),
      },
      aivis: {
        value: aivisApiKeyInput, setValue: setAivisApiKeyInput, setSaving: setSavingAivisKey, setDeleting: setDeletingAivisKey,
        save: api.setAivisApiKey, remove: api.deleteAivisApiKey,
        deleteTitle: t("settings.aivisDeleteTitle"), deleteMessage: t("settings.aivisDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.aivisSaved"), deleteSuccessMessage: t("settings.toast.aivisDeleted"),
        afterSave: () => { setAivisUserDictionariesLoaded(false); },
        afterDelete: () => {
          setAivisUserDictionaryUUID("");
          setAivisUserDictionaries([]);
          setAivisUserDictionariesLoaded(false);
          setAivisUserDictionariesError(null);
        },
      },
      elevenlabs: {
        value: elevenLabsApiKeyInput, setValue: setElevenLabsApiKeyInput, setSaving: setSavingElevenLabsKey, setDeleting: setDeletingElevenLabsKey,
        save: api.setElevenLabsApiKey, remove: api.deleteElevenLabsApiKey,
        deleteTitle: t("settings.elevenlabsDeleteTitle"), deleteMessage: t("settings.elevenlabsDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.elevenlabsSaved"), deleteSuccessMessage: t("settings.toast.elevenlabsDeleted"),
        afterSave: () => { setElevenLabsVoicesData(null); setElevenLabsVoicesError(null); },
        afterDelete: () => { setElevenLabsVoicesData(null); setElevenLabsVoicesError(null); },
      },
      fish: {
        value: fishApiKeyInput, setValue: setFishApiKeyInput, setSaving: setSavingFishKey, setDeleting: setDeletingFishKey,
        save: api.setFishApiKey, remove: api.deleteFishApiKey,
        deleteTitle: t("settings.fishDeleteTitle"), deleteMessage: t("settings.fishDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.fishSaved"), deleteSuccessMessage: t("settings.toast.fishDeleted"),
      },
    },
  });

  const submitAzureSpeechConfig = async (event: FormEvent) => {
    event.preventDefault();
    setSavingAzureSpeechConfig(true);
    try {
      const apiKey = azureSpeechApiKeyInput.trim();
      const region = azureSpeechRegionInput.trim();
      if (!apiKey) throw new Error(t("settings.error.enterApiKey"));
      if (!region) throw new Error(t("settings.azureSpeechRegionRequired"));
      await api.setAzureSpeechConfig(apiKey, region);
      setAzureSpeechApiKeyInput("");
      await load();
      showToast(t("settings.toast.azureSpeechSaved"), "success");
    } catch (error) {
      showToast(String(error), "error");
    } finally {
      setSavingAzureSpeechConfig(false);
    }
  };

  const deleteAzureSpeechConfig = async () => {
    if (!(await confirm({
      title: t("settings.azureSpeechDeleteTitle"),
      message: t("settings.azureSpeechDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingAzureSpeechConfig(true);
    try {
      await api.deleteAzureSpeechConfig();
      setAzureSpeechRegionInput("");
      await load();
      showToast(t("settings.toast.azureSpeechDeleted"), "success");
    } catch (error) {
      showToast(String(error), "error");
    } finally {
      setDeletingAzureSpeechConfig(false);
    }
  };

  const apiKeyCardLabels = useMemo(() => buildApiKeyCardLabels(t), [t]);

  const accessCards = buildAccessCards(
    settings,
    {
      anthropic: createAccessCardRuntime(anthropicApiKeyInput, setAnthropicApiKeyInput, apiKeyHandlers.anthropic!.submit, apiKeyHandlers.anthropic!.remove, savingAnthropicKey, deletingAnthropicKey),
      openai: createAccessCardRuntime(openAIApiKeyInput, setOpenAIApiKeyInput, apiKeyHandlers.openai!.submit, apiKeyHandlers.openai!.remove, savingOpenAIKey, deletingOpenAIKey),
      google: createAccessCardRuntime(googleApiKeyInput, setGoogleApiKeyInput, apiKeyHandlers.google!.submit, apiKeyHandlers.google!.remove, savingGoogleKey, deletingGoogleKey),
      groq: createAccessCardRuntime(groqApiKeyInput, setGroqApiKeyInput, apiKeyHandlers.groq!.submit, apiKeyHandlers.groq!.remove, savingGroqKey, deletingGroqKey),
      deepseek: createAccessCardRuntime(deepseekApiKeyInput, setDeepseekApiKeyInput, apiKeyHandlers.deepseek!.submit, apiKeyHandlers.deepseek!.remove, savingDeepSeekKey, deletingDeepSeekKey),
      alibaba: createAccessCardRuntime(alibabaApiKeyInput, setAlibabaApiKeyInput, apiKeyHandlers.alibaba!.submit, apiKeyHandlers.alibaba!.remove, savingAlibabaKey, deletingAlibabaKey),
      mistral: createAccessCardRuntime(mistralApiKeyInput, setMistralApiKeyInput, apiKeyHandlers.mistral!.submit, apiKeyHandlers.mistral!.remove, savingMistralKey, deletingMistralKey),
      moonshot: createAccessCardRuntime(moonshotApiKeyInput, setMoonshotApiKeyInput, apiKeyHandlers.moonshot!.submit, apiKeyHandlers.moonshot!.remove, savingMoonshotKey, deletingMoonshotKey),
      xai: createAccessCardRuntime(xaiApiKeyInput, setXaiApiKeyInput, apiKeyHandlers.xai!.submit, apiKeyHandlers.xai!.remove, savingXAIKey, deletingXAIKey),
      zai: createAccessCardRuntime(zaiApiKeyInput, setZaiApiKeyInput, apiKeyHandlers.zai!.submit, apiKeyHandlers.zai!.remove, savingZAIKey, deletingZAIKey),
      fireworks: createAccessCardRuntime(fireworksApiKeyInput, setFireworksApiKeyInput, apiKeyHandlers.fireworks!.submit, apiKeyHandlers.fireworks!.remove, savingFireworksKey, deletingFireworksKey),
      together: createAccessCardRuntime(togetherApiKeyInput, setTogetherApiKeyInput, apiKeyHandlers.together!.submit, apiKeyHandlers.together!.remove, savingTogetherKey, deletingTogetherKey),
      poe: createAccessCardRuntime(poeApiKeyInput, setPoeApiKeyInput, apiKeyHandlers.poe!.submit, apiKeyHandlers.poe!.remove, savingPoeKey, deletingPoeKey),
      siliconflow: createAccessCardRuntime(siliconFlowApiKeyInput, setSiliconFlowApiKeyInput, apiKeyHandlers.siliconflow!.submit, apiKeyHandlers.siliconflow!.remove, savingSiliconFlowKey, deletingSiliconFlowKey),
      azure_speech: createAccessCardRuntime(
        azureSpeechApiKeyInput,
        setAzureSpeechApiKeyInput,
        submitAzureSpeechConfig,
        deleteAzureSpeechConfig,
        savingAzureSpeechConfig,
        deletingAzureSpeechConfig,
        azureSpeechRegionInput,
        setAzureSpeechRegionInput,
      ),
      openrouter: createAccessCardRuntime(openRouterApiKeyInput, setOpenRouterApiKeyInput, apiKeyHandlers.openrouter!.submit, apiKeyHandlers.openrouter!.remove, savingOpenRouterKey, deletingOpenRouterKey),
      aivis: createAccessCardRuntime(aivisApiKeyInput, setAivisApiKeyInput, apiKeyHandlers.aivis!.submit, apiKeyHandlers.aivis!.remove, savingAivisKey, deletingAivisKey),
      elevenlabs: createAccessCardRuntime(elevenLabsApiKeyInput, setElevenLabsApiKeyInput, apiKeyHandlers.elevenlabs!.submit, apiKeyHandlers.elevenlabs!.remove, savingElevenLabsKey, deletingElevenLabsKey),
      fish: createAccessCardRuntime(fishApiKeyInput, setFishApiKeyInput, apiKeyHandlers.fish!.submit, apiKeyHandlers.fish!.remove, savingFishKey, deletingFishKey),
    },
    t,
  );

  const { configuredProviderCount, activeAccessCard } = resolveAccessCardSelection(accessCards, activeAccessProvider);
  const {
    savedSansKey: savedUIFontSansKey,
    savedSerifKey: savedUIFontSerifKey,
    selectedSans: selectedUIFontSans,
    selectedSerif: selectedUIFontSerif,
    dirty: uiFontsDirty,
  } = buildUIFontState(settings, uiFontSansKey, uiFontSerifKey);

  useEffect(() => {
    uiFontsDirtyRef.current = uiFontsDirty;
  }, [uiFontsDirty]);

  function dismissProviderModelUpdates() {
    setDismissedModelUpdatesAt(dismissProviderModelUpdatesToLocalStorage(providerModelUpdates));
  }

  function restoreProviderModelUpdates() {
    setDismissedModelUpdatesAt(restoreProviderModelUpdatesFromLocalStorage());
  }

  function toggleLLMExtras() {
    llm.setLLMExtrasOpen((prev) => {
      const next = !prev;
      if (next) {
        window.requestAnimationFrame(() => {
          llmExtrasRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
        });
      }
      return next;
    });
  }

  async function submitBudget(e: FormEvent) {
    e.preventDefault();
    await saveBudgetSettingsAction({
      budgetUSD,
      alertEnabled,
      thresholdPct,
      digestEmailEnabled,
      setSaving: setSavingBudget,
      showToast,
      t,
      reload: load,
    });
  }

  async function submitUIFontSettings(e: FormEvent) {
    await submitSavingFormAction({
      event: e,
      setSaving: setSavingUIFontSettings,
      showToast,
      successMessage: t("settings.uiFonts.saved"),
      run: async () => {
        const resp = await api.updateUIFontSettings({
          ui_font_sans_key: uiFontSansKey,
          ui_font_serif_key: uiFontSerifKey,
        });
        persistUIFontSelection({
          sansKey: resp.ui_font_sans_key,
          serifKey: resp.ui_font_serif_key,
        });
        setSettings((prev) =>
          prev
            ? {
                ...prev,
                ui_font_sans_key: resp.ui_font_sans_key,
                ui_font_serif_key: resp.ui_font_serif_key,
              }
            : prev
        );
        void queryClient.invalidateQueries({ queryKey: ["settings"] });
      },
    });
  }

  async function submitLLMModels(e: FormEvent) {
    await submitSavingFormAction({
      event: e,
      setSaving: setSavingLLMModels,
      showToast,
      mapError: (error) => localizeSettingsErrorMessage(error, t),
      run: () => persistLLMModels(buildLLMModelPayload(), t("settings.toast.modelsSaved")),
    });
  }

  async function submitAudioBriefingModels(e: FormEvent) {
    await submitSavingFormAction({
      event: e,
      setSaving: setSavingLLMModels,
      showToast,
      mapError: (error) => localizeSettingsErrorMessage(error, t),
      run: () =>
        persistLLMModels(
          buildLLMModelPayload({
            audio_briefing_script: audioBriefingScriptModel || null,
            audio_briefing_script_fallback: audioBriefingScriptFallbackModel || null,
          }),
          t("settings.toast.modelsSaved")
        ),
    });
  }

  async function submitDigestDelivery(e: FormEvent) {
    e.preventDefault();
    if (!settings) return;
    await saveDigestDeliveryAction({
      monthlyBudgetUSD: settings.monthly_budget_usd,
      budgetAlertEnabled: settings.budget_alert_enabled,
      budgetAlertThresholdPct: settings.budget_alert_threshold_pct,
      digestEmailEnabled,
      setSaving: setSavingDigestDelivery,
      showToast,
      t,
      reload: load,
    });
  }

  async function submitReadingPlan(e: FormEvent) {
    e.preventDefault();
    await saveReadingPlanAction({
      readingPlanWindow,
      readingPlanSize,
      readingPlanDiversifyTopics,
      setSaving: setSavingReadingPlan,
      showToast,
      t,
      reload: load,
    });
  }

  async function submitObsidianExport(e: FormEvent) {
    e.preventDefault();
    await saveObsidianExportAction({
      enabled: obsidianEnabled,
      repoOwner: obsidianRepoOwner,
      repoName: obsidianRepoName,
      repoBranch: obsidianRepoBranch,
      rootPath: obsidianRootPath,
      setSaving: setSavingObsidianExport,
      showToast,
      t,
      setSettings,
    });
  }

  async function runObsidianExportNow() {
    await runObsidianExportNowAction({
      setSaving: setRunningObsidianExport,
      showToast,
      t,
      reload: load,
    });
  }

  async function saveAivisUserDictionary() {
    await saveAivisUserDictionaryAction({
      dictionaryUUID: aivisUserDictionaryUUID,
      setSaving: setSavingAivisDictionary,
      showToast,
      t,
      setDictionaryUUID: setAivisUserDictionaryUUID,
      setSettings,
    });
  }

  async function clearAivisUserDictionary() {
    await clearAivisUserDictionaryAction({
      setSaving: setDeletingAivisDictionary,
      showToast,
      t,
      setDictionaryUUID: setAivisUserDictionaryUUID,
      setSettings,
    });
  }

  async function handleDeleteInoreaderOAuth() {
    await deleteInoreaderOAuthAction({
      confirm,
      setSaving: setDeletingInoreaderOAuth,
      showToast,
      t,
      reload: load,
    });
  }

  async function handleResetPreferenceProfile() {
    await resetPreferenceProfileAction({
      confirm,
      setSaving: setResettingPreferenceProfile,
      showToast,
      t,
      reload: load,
    });
  }

  async function persistAudioBriefingSettings() {
    if (audioBriefingConversationMode === "duo" && configuredAudioBriefingVoiceCount < 2) {
      showToast(t("settings.audioBriefing.duoRequiresTwoVoices"), "error");
      return;
    }
    await runSavingAction({
      setSaving: setSavingAudioBriefing,
      showToast,
      successMessage: t("settings.toast.audioBriefingSaved"),
      run: async () => {
        const resp = await api.updateAudioBriefingSettings(
          buildAudioBriefingSettingsPayload({
            scheduleSelection: audioBriefingScheduleSelection,
            enabled: audioBriefingEnabled,
            articlesPerEpisode: audioBriefingArticlesPerEpisode,
            targetDurationMinutes: audioBriefingTargetDurationMinutes,
            chunkTrailingSilenceSeconds: audioBriefingChunkTrailingSilenceSeconds,
            programName: audioBriefingProgramName,
            defaultPersonaMode: audioBriefingDefaultPersonaMode,
            defaultPersona: audioBriefingDefaultPersona,
            conversationMode: audioBriefingConversationMode,
            bgmEnabled: audioBriefingBGMEnabled,
            bgmPrefix: audioBriefingBGMR2Prefix,
          })
        );
        setSettings((prev) => mergeAudioBriefingIntoSettings(prev, resp.audio_briefing));
        syncAudioBriefingSettingsForm(resp.audio_briefing);
      },
    });
  }

  async function submitAudioBriefingSettings(e: FormEvent) {
    e.preventDefault();
    await persistAudioBriefingSettings();
  }

  async function persistPodcastSettings() {
    await runSavingAction({
      setSaving: setSavingPodcast,
      showToast,
      successMessage: t("settings.toast.podcastSaved"),
      run: async () => {
        const resp = await api.updatePodcastSettings(
          buildPodcastSettingsPayload({
            enabled: podcastEnabled,
            feedSlug: podcastFeedSlug,
            rssURL: podcastRSSURL,
            title: podcastTitle,
            description: podcastDescription,
            author: podcastAuthor,
            language: podcastLanguage,
            category: podcastCategory,
            subcategory: podcastSubcategory,
            explicit: podcastExplicit,
            artworkURL: podcastArtworkURL,
          })
        );
        setSettings((prev) => mergePodcastIntoSettings(prev, resp.podcast));
        syncPodcastForm(resp.podcast);
      },
    });
  }

  async function submitPodcastSettings(e: FormEvent) {
    e.preventDefault();
    await persistPodcastSettings();
  }

  async function copyPodcastRSSURL() {
    await copyToClipboardAction({
      value: podcastRSSURL,
      writeText: (value) => navigator.clipboard.writeText(value),
      showToast,
      successMessage: t("settings.toast.podcastRSSCopied"),
    });
  }

  async function handlePodcastArtworkFileChange(file: File | null) {
    if (!file) return;
    setUploadingPodcastArtwork(true);
    try {
      const contentBase64 = await readFileAsBase64DataURL(file);
      const resp = await api.uploadPodcastArtwork({
        content_type: file.type || "image/jpeg",
        content_base64: contentBase64,
      });
      setPodcastArtworkURL(resp.artwork_url ?? "");
      setSettings((prev) =>
        buildPodcastSettingsAfterArtworkUpload({
          previousSettings: prev,
          artworkURL: resp.artwork_url ?? null,
          enabled: podcastEnabled,
          feedSlug: podcastFeedSlug,
          rssURL: podcastRSSURL,
          title: podcastTitle,
          description: podcastDescription,
          author: podcastAuthor,
          language: podcastLanguage,
          category: podcastCategory,
          subcategory: podcastSubcategory,
          availableCategories: podcastAvailableCategories,
          explicit: podcastExplicit,
        })
      );
      showToast(t("settings.toast.podcastArtworkUploaded"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setUploadingPodcastArtwork(false);
    }
  }

  async function persistAudioBriefingVoices() {
    await runSavingAction({
      setSaving: setSavingAudioBriefingVoices,
      showToast,
      successMessage: t("settings.toast.audioBriefingVoicesSaved"),
      run: async () => {
        const resp = await api.updateAudioBriefingPersonaVoices(audioBriefingVoices);
        setSettings((prev) => mergeAudioBriefingVoicesIntoSettings(prev, resp.audio_briefing_persona_voices));
        syncAudioBriefingVoiceForm(resp.audio_briefing_persona_voices);
      },
    });
  }

  async function submitAudioBriefingVoices(e: FormEvent) {
    e.preventDefault();
    await persistAudioBriefingVoices();
  }

  async function submitAudioBriefingPresetSave() {
    await saveAudioBriefingPresetAction({
      name: presets.audioBriefingPresetName,
      presetsLoaded: audioBriefingPresetsLoaded,
      presets: audioBriefingPresets,
      loadPresets: api.listAudioBriefingPresets,
      setPresets: setAudioBriefingPresets,
      setPresetsLoaded: setAudioBriefingPresetsLoaded,
      setPresetsError: setAudioBriefingPresetsError,
      buildPayload: () =>
        buildAudioBriefingPresetRequest(
          presets.audioBriefingPresetName,
          audioBriefingDefaultPersonaMode,
          audioBriefingDefaultPersona,
          audioBriefingConversationMode,
          audioBriefingVoices,
        ),
      createPreset: api.createAudioBriefingPreset,
      updatePreset: api.updateAudioBriefingPreset,
      confirmOverwrite: async (name) =>
        confirm({
          title: t("settings.audioBriefing.presetOverwriteTitle"),
          message: t("settings.audioBriefing.presetOverwriteMessage").replace("{{name}}", name),
          confirmLabel: t("settings.audioBriefing.presetOverwriteConfirm"),
        }),
      onSaved: () => {
        presets.setAudioBriefingPresetSaveOpen(false);
        presets.setAudioBriefingPresetName("");
      },
      showToast,
      requiredNameMessage: t("settings.audioBriefing.presetNameRequired"),
      updatedMessage: t("settings.audioBriefing.presetUpdated"),
      savedMessage: t("settings.audioBriefing.presetSaved"),
      setSaving: presets.setAudioBriefingPresetSaving,
    });
  }

  function openAudioBriefingPresetSaveModal() {
    presets.setAudioBriefingPresetName("");
    presets.setAudioBriefingPresetSaveOpen(true);
  }

  async function openAudioBriefingPresetApplyModal() {
    await openAudioBriefingPresetApplyAction({
      presetsCount: audioBriefingPresets.length,
      setSelection: presets.setAudioBriefingPresetApplySelection,
      setOpen: presets.setAudioBriefingPresetApplyOpen,
      loadPresets: loadAudioBriefingPresets,
    });
  }

  function applyAudioBriefingPreset(preset: AudioBriefingPreset) {
    syncAudioBriefingPresetForm(preset);
    presets.setAudioBriefingPresetApplySelection(preset.id);
    presets.setAudioBriefingPresetApplyOpen(false);
    showToast(t("settings.audioBriefing.presetApplied"), "success");
  }

  async function persistSummaryAudioSettings() {
    await runSavingAction({
      setSaving: setSummaryAudioSaving,
      showToast,
      successMessage: t("settings.toast.summaryAudioSaved"),
      run: async () => {
        const resp = await api.updateSummaryAudioSettings(
          buildSummaryAudioSettingsPayload({
            provider: summaryAudioProvider,
            ttsModel: summaryAudioTTSModel,
            voiceModel: summaryAudioVoiceModel,
            voiceStyle: summaryAudioVoiceStyle,
            providerVoiceLabel: summaryAudioProviderVoiceLabel,
            providerVoiceDescription: summaryAudioProviderVoiceDescription,
            speechRate: summaryAudioSpeechRate,
            emotionalIntensity: summaryAudioEmotionalIntensity,
            tempoDynamics: summaryAudioTempoDynamics,
            lineBreakSilenceSeconds: summaryAudioLineBreakSilenceSeconds,
            pitch: summaryAudioPitch,
            volumeGain: summaryAudioVolumeGain,
            aivisUserDictionaryUUID: summaryAudioAivisUserDictionaryUUID,
          })
        );
        const nextSettings = mergeSummaryAudioIntoSettings(settings, resp.summary_audio ?? null);
        setSettings(nextSettings);
        if (nextSettings) {
          queryClient.setQueryData(["shared-audio-player-settings"], nextSettings);
          queryClient.setQueryData(["settings", "summary-audio-readiness"], nextSettings);
        }
        syncSummaryAudioForm(resp.summary_audio ?? null);
        void queryClient.invalidateQueries({ queryKey: ["shared-audio-player-settings"] });
        void queryClient.invalidateQueries({ queryKey: ["settings", "summary-audio-readiness"] });
      },
    });
  }

  async function submitSummaryAudioSettings(e: FormEvent) {
    e.preventDefault();
    await persistSummaryAudioSettings();
  }

  function updateSummaryAudioProvider(nextProvider: string) {
    const normalized = nextProvider.trim().toLowerCase();
    const capabilities = getAudioBriefingProviderCapabilities(normalized);
    setSummaryAudioProvider(normalized);
    setSummaryAudioVoiceModel("");
    setSummaryAudioVoiceStyle("");
    setSummaryAudioProviderVoiceLabel("");
    setSummaryAudioProviderVoiceDescription("");
    setSummaryAudioTTSModel(normalized === "openai" ? "tts-1" : getTTSProviderDefaultModel(normalized));
    if (!capabilities.requiresVoiceStyle) {
      setSummaryAudioVoiceStyle("");
    }
  }

  function updateSummaryAudioVoiceNumberInput(
    field: SummaryAudioNumericInputField,
    raw: string,
    applyParsedValue: (value: number) => void
  ) {
    setSummaryAudioVoiceInputDrafts((prev) => ({
      ...prev,
      [field]: raw,
    }));
    if (!raw || !isCompleteDecimalInput(raw)) return;
    const parsed = Number(raw);
    if (!Number.isFinite(parsed)) return;
    applyParsedValue(parsed);
  }

  function resetSummaryAudioVoiceNumberInput(field: SummaryAudioNumericInputField) {
    setSummaryAudioVoiceInputDrafts((prev) => ({
      ...prev,
      [field]: buildSummaryAudioVoiceInputDrafts({
        tts_provider: summaryAudioProvider,
        tts_model: summaryAudioTTSModel,
        voice_model: summaryAudioVoiceModel,
        voice_style: summaryAudioVoiceStyle,
        provider_voice_label: summaryAudioProviderVoiceLabel,
        provider_voice_description: summaryAudioProviderVoiceDescription,
        speech_rate: Number(summaryAudioSpeechRate),
        emotional_intensity: Number(summaryAudioEmotionalIntensity),
        tempo_dynamics: Number(summaryAudioTempoDynamics),
        line_break_silence_seconds: Number(summaryAudioLineBreakSilenceSeconds),
        pitch: Number(summaryAudioPitch),
        volume_gain: Number(summaryAudioVolumeGain),
        aivis_user_dictionary_uuid: summaryAudioAivisUserDictionaryUUID || null,
      })[field],
    }));
  }

  function updateAudioBriefingVoice(persona: string, patch: Partial<AudioBriefingPersonaVoice>) {
    setAudioBriefingVoices((prev) =>
      prev.map((voice) => {
        if (voice.persona !== persona) return voice;
        const nextVoice = { ...voice, ...patch };
        if ("tts_provider" in patch) {
          const nextProvider = nextVoice.tts_provider.trim().toLowerCase();
          const capabilities = getAudioBriefingProviderCapabilities(nextProvider);
          nextVoice.tts_model = getAudioBriefingTTSProviderDefaultModel(nextProvider, audioBriefingConversationMode);
          if (!capabilities.requiresVoiceStyle) {
            nextVoice.voice_style = "";
          }
          if (nextProvider !== "fish" && nextProvider !== "elevenlabs") {
            nextVoice.provider_voice_label = "";
            nextVoice.provider_voice_description = "";
          }
          setAudioBriefingVoiceInputDrafts((drafts) => ({
            ...drafts,
            [persona]: buildAudioBriefingVoiceInputDrafts([nextVoice])[persona],
          }));
        }
        return nextVoice;
      })
    );
  }

  function updateAudioBriefingVoiceNumberInput(
    persona: string,
    field: AudioBriefingNumericInputField,
    raw: string,
    applyParsedValue: (value: number) => Partial<AudioBriefingPersonaVoice>
  ) {
    setAudioBriefingVoiceInputDrafts((prev) => ({
      ...prev,
      [persona]: {
        ...prev[persona],
        [field]: raw,
      },
    }));
    if (!raw || !isCompleteDecimalInput(raw)) return;
    const parsed = Number(raw);
    if (!Number.isFinite(parsed)) return;
    updateAudioBriefingVoice(persona, applyParsedValue(parsed));
  }

  function resetAudioBriefingVoiceNumberInput(persona: string, field: AudioBriefingNumericInputField) {
    setAudioBriefingVoiceInputDrafts((prev) => {
      const voice = audioBriefingVoices.find((item) => item.persona === persona);
      if (!voice) return prev;
      return {
        ...prev,
        [persona]: {
          ...prev[persona],
          [field]: buildAudioBriefingVoiceInputDrafts([voice])[persona][field],
        },
      };
    });
  }

  function toggleAudioBriefingPersona(persona: string) {
    setExpandedAudioBriefingPersonas((prev) =>
      prev.includes(persona) ? prev.filter((item) => item !== persona) : [...prev, persona]
    );
  }

  const {
    openAivisPicker,
    openXAIPicker,
    openFishPicker,
    openElevenLabsPicker,
    openOpenAITTSPicker,
    openGeminiTTSPicker,
    openAzureSpeechPicker,
  } = buildAudioBriefingPickerOpeners({
    pickers: {
      setAivisPickerPersona: audioBriefingPickers.setAivisPickerPersona,
      setFishPickerPersona: audioBriefingPickers.setFishPickerPersona,
      setXAIPickerPersona: audioBriefingPickers.setXAIPickerPersona,
      setElevenLabsPickerPersona: audioBriefingPickers.setElevenLabsPickerPersona,
      setOpenAITTPickerPersona: audioBriefingPickers.setOpenAITTPickerPersona,
      setGeminiTTSPickerPersona: audioBriefingPickers.setGeminiTTSPickerPersona,
      setAzureSpeechPickerPersona: audioBriefingPickers.setAzureSpeechPickerPersona,
    },
    aivisModelsData,
    xaiVoicesData,
    elevenLabsVoicesData,
    openAITTSVoicesData,
    geminiTTSVoicesData,
    azureSpeechVoicesData,
    loadAivisModels,
    loadXAIVoices,
    loadElevenLabsVoices,
    loadOpenAITTSVoices,
    loadGeminiTTSVoices,
    loadAzureSpeechVoices,
  });

  if (loading) return { loading: true as const, error: null, settings: null, t, load };
  if (error) return { loading: false as const, error, settings: null, t, load };
  if (!settings) return { loading: false as const, error: t("settings.loadFailed"), settings: null, t, load };

  const activeAivisVoice = findAudioBriefingActiveVoice(audioBriefingVoices, audioBriefingPickers.aivisPickerPersona);
  const activeXAIVoice = findAudioBriefingActiveVoice(audioBriefingVoices, audioBriefingPickers.xaiPickerPersona);
  const activeElevenLabsVoice = findAudioBriefingActiveVoice(audioBriefingVoices, audioBriefingPickers.elevenLabsPickerPersona);
  const activeOpenAITTSVoice = findAudioBriefingActiveVoice(audioBriefingVoices, audioBriefingPickers.openAITTPickerPersona);
  const activeGeminiTTSVoice = findAudioBriefingActiveVoice(audioBriefingVoices, audioBriefingPickers.geminiTTSPickerPersona);
  const activeAzureSpeechVoice = findAudioBriefingActiveVoice(audioBriefingVoices, audioBriefingPickers.azureSpeechPickerPersona);
  const {
    audioBriefingAivisModels,
    audioBriefingXAIVoices,
    audioBriefingElevenLabsVoices,
    audioBriefingOpenAITTSVoices,
    audioBriefingGeminiTTSVoices,
    audioBriefingAzureSpeechVoices,
    summaryAudioAivisModels,
    summaryAudioXAIVoices,
    summaryAudioElevenLabsVoices,
    summaryAudioOpenAITTSVoices,
    summaryAudioGeminiTTSVoices,
    summaryAudioAzureSpeechVoices,
  } = buildVoicePickerCatalogData(aivisModelsData, xaiVoicesData, elevenLabsVoicesData, openAITTSVoicesData, geminiTTSVoicesData, azureSpeechVoicesData);
  const hasUserAivisAPIKey = Boolean(settings?.has_aivis_api_key);
  const hasUserFishAPIKey = Boolean(settings?.has_fish_api_key);
  const hasUserXAIAPIKey = Boolean(settings?.has_xai_api_key);
  const hasUserElevenLabsAPIKey = Boolean(settings?.has_elevenlabs_api_key);
  const hasUserOpenAIAPIKey = Boolean(settings?.has_openai_api_key);
  const hasUserAzureSpeechAPIKey = Boolean(settings?.has_azure_speech_api_key);
  const azureSpeechRegion = settings?.azure_speech_region?.trim() || "";
  const geminiTTSEnabled = Boolean(settings?.gemini_tts_enabled);
  const summaryAudioProviderCapabilities = getAudioBriefingProviderCapabilities(summaryAudioProvider);
  const summaryAudioResolvedVoice = summaryAudioProvider === "aivis"
    ? resolveAivisVoiceSelection(summaryAudioAivisModels, {
        voice_model: summaryAudioVoiceModel,
        voice_style: summaryAudioVoiceStyle,
      })
    : summaryAudioProvider === "fish"
      ? null
      : summaryAudioProvider === "xai"
        ? resolveXAIVoiceSelection(summaryAudioXAIVoices, { voice_model: summaryAudioVoiceModel })
        : summaryAudioProvider === "elevenlabs"
          ? resolveElevenLabsVoiceSelection(summaryAudioElevenLabsVoices, { voice_model: summaryAudioVoiceModel })
        : summaryAudioProvider === "openai"
        ? resolveOpenAITTSVoiceSelection(summaryAudioOpenAITTSVoices, { voice_model: summaryAudioVoiceModel })
        : summaryAudioProvider === "gemini_tts"
          ? resolveGeminiTTSVoiceSelection(summaryAudioGeminiTTSVoices, { voice_model: summaryAudioVoiceModel })
          : summaryAudioProvider === "azure_speech"
            ? resolveAzureSpeechVoiceSelection(summaryAudioAzureSpeechVoices, { voice_model: summaryAudioVoiceModel })
          : null;
  const summaryAudioResolvedAivisVoice = summaryAudioProvider === "aivis"
    ? (summaryAudioResolvedVoice as ReturnType<typeof resolveAivisVoiceSelection> | null)
    : null;
  const summaryAudioResolvedXAIVoice = summaryAudioProvider === "xai"
    ? (summaryAudioResolvedVoice as XAIVoiceSnapshot | null)
    : null;
  const summaryAudioResolvedElevenLabsVoice = summaryAudioProvider === "elevenlabs"
    ? (summaryAudioResolvedVoice as ElevenLabsVoiceCatalogEntry | null)
    : null;
  const summaryAudioResolvedOpenAIVoice = summaryAudioProvider === "openai"
    ? (summaryAudioResolvedVoice as OpenAITTSVoiceSnapshot | null)
    : null;
  const summaryAudioResolvedGeminiVoice = summaryAudioProvider === "gemini_tts"
    ? (summaryAudioResolvedVoice as GeminiTTSVoiceCatalogEntry | null)
    : null;
  const summaryAudioResolvedAzureSpeechVoice = summaryAudioProvider === "azure_speech"
    ? (summaryAudioResolvedVoice as AzureSpeechVoiceCatalogEntry | null)
    : null;
  const audioBriefingVoiceSummaries = audioBriefingVoices.map((voice) => ({
    voice,
    resolved: voice.tts_provider === "aivis"
      ? resolveAivisVoiceSelection(audioBriefingAivisModels, voice)
      : voice.tts_provider === "fish"
        ? null
      : voice.tts_provider === "xai"
        ? resolveXAIVoiceSelection(audioBriefingXAIVoices, voice)
        : voice.tts_provider === "elevenlabs"
          ? resolveElevenLabsVoiceSelection(audioBriefingElevenLabsVoices, voice)
        : voice.tts_provider === "openai"
        ? resolveOpenAITTSVoiceSelection(audioBriefingOpenAITTSVoices, voice)
          : voice.tts_provider === "gemini_tts"
            ? resolveGeminiTTSVoiceSelection(audioBriefingGeminiTTSVoices, voice)
          : voice.tts_provider === "azure_speech"
            ? resolveAzureSpeechVoiceSelection(audioBriefingAzureSpeechVoices, voice)
        : null,
    status: getAudioBriefingVoiceStatus(
      voice,
      audioBriefingAivisModels,
      [],
      audioBriefingXAIVoices,
      audioBriefingOpenAITTSVoices,
      audioBriefingGeminiTTSVoices,
      audioBriefingElevenLabsVoices,
      audioBriefingAzureSpeechVoices,
      hasUserAivisAPIKey,
      hasUserFishAPIKey,
      hasUserXAIAPIKey,
      hasUserOpenAIAPIKey,
      hasUserElevenLabsAPIKey,
      hasUserAzureSpeechAPIKey,
      azureSpeechRegion,
      geminiTTSEnabled,
      audioBriefingConversationMode,
      t
    ),
  }));
  const configuredAudioBriefingVoiceCount = audioBriefingVoiceSummaries.filter((entry) => entry.status.configured).length;
  const audioBriefingVoiceAttentionCount = audioBriefingVoiceSummaries.filter((entry) => entry.status.tone === "warn").length;
  const audioBriefingVoiceReadyCount = audioBriefingVoiceSummaries.filter((entry) => entry.status.tone === "ok").length;
  const summaryAudioVoiceDisplay = resolveTTSVoiceDisplay({
    provider: summaryAudioProvider,
    voiceModel: summaryAudioVoiceModel,
    voiceStyle: summaryAudioVoiceStyle,
    providerVoiceLabel: summaryAudioProviderVoiceLabel,
    providerVoiceDescription: summaryAudioProviderVoiceDescription,
    unsetText: t("settings.summaryAudio.unsetShort"),
    t,
    aivisResolved: summaryAudioResolvedAivisVoice,
    xaiResolved: summaryAudioResolvedXAIVoice,
    openAIResolved: summaryAudioResolvedOpenAIVoice,
    geminiResolved: summaryAudioResolvedGeminiVoice,
    elevenLabsResolved: summaryAudioResolvedElevenLabsVoice,
    azureSpeechResolved: summaryAudioResolvedAzureSpeechVoice,
  });
  const summaryAudioResolvedVoiceLabel = summaryAudioVoiceDisplay.label;
  const summaryAudioResolvedVoiceDetail = summaryAudioVoiceDisplay.detail;
  const audioBriefingUsesAivisCloud = audioBriefingVoices.some((voice) => voice.tts_provider === "aivis");
  const audioBriefingNeedsAivisAPIKey = audioBriefingUsesAivisCloud && !hasUserAivisAPIKey;
  const audioBriefingUsesFish = audioBriefingVoices.some((voice) => voice.tts_provider === "fish");
  const audioBriefingNeedsFishAPIKey = audioBriefingUsesFish && !hasUserFishAPIKey;
  const audioBriefingUsesXAI = audioBriefingVoices.some((voice) => voice.tts_provider === "xai");
  const audioBriefingNeedsXAIAPIKey = audioBriefingUsesXAI && !hasUserXAIAPIKey;
  const audioBriefingUsesElevenLabs = audioBriefingVoices.some((voice) => voice.tts_provider === "elevenlabs");
  const audioBriefingNeedsElevenLabsAPIKey = audioBriefingUsesElevenLabs && !hasUserElevenLabsAPIKey;
  const audioBriefingUsesOpenAI = audioBriefingVoices.some((voice) => voice.tts_provider === "openai");
  const audioBriefingNeedsOpenAIAPIKey = audioBriefingUsesOpenAI && !hasUserOpenAIAPIKey;
  const audioBriefingUsesAzureSpeech = audioBriefingVoices.some((voice) => voice.tts_provider === "azure_speech");
  const audioBriefingNeedsAzureSpeechAPIKey = audioBriefingUsesAzureSpeech && !hasUserAzureSpeechAPIKey;
  const audioBriefingNeedsAzureSpeechRegion = audioBriefingUsesAzureSpeech && !azureSpeechRegion;
  const audioBriefingUsesGeminiTTS = audioBriefingVoices.some((voice) => voice.tts_provider === "gemini_tts");
  const audioBriefingNeedsGeminiAccess = audioBriefingUsesGeminiTTS && !geminiTTSEnabled;
  const geminiDuoModelCounts = audioBriefingVoices.reduce((acc, voice) => {
    if (voice.tts_provider !== "gemini_tts") return acc;
    const model = voice.tts_model.trim();
    const selectedVoice = voice.voice_model.trim();
    if (!model || !selectedVoice) return acc;
    acc.set(model, (acc.get(model) ?? 0) + 1);
    return acc;
  }, new Map<string, number>());
  const geminiDuoBestModelEntry = Array.from(geminiDuoModelCounts.entries()).sort((a, b) => b[1] - a[1])[0] ?? null;
  const geminiDuoCompatiblePersonaCount = geminiDuoBestModelEntry?.[1] ?? 0;
  const geminiDuoCompatibleModel = geminiDuoBestModelEntry?.[0] ?? "";
  const geminiDuoReady = geminiTTSEnabled && geminiDuoCompatiblePersonaCount >= 2;
  const fishDuoCompatibleVoices = audioBriefingVoices.filter((voice) =>
    voice.tts_provider === "fish" && voice.tts_model === "s2-pro" && voice.voice_model.trim().length > 0
  );
  const fishDuoDistinctVoiceCount = new Set(fishDuoCompatibleVoices.map((voice) => voice.voice_model.trim())).size;
  const fishDuoReady = hasUserFishAPIKey && fishDuoDistinctVoiceCount >= 2;
  const elevenLabsDuoCompatibleVoices = audioBriefingVoices.filter((voice) =>
    voice.tts_provider === "elevenlabs" && voice.tts_model === "eleven_v3" && voice.voice_model.trim().length > 0
  );
  const elevenLabsDuoDistinctVoiceCount = new Set(elevenLabsDuoCompatibleVoices.map((voice) => voice.voice_model.trim())).size;
  const elevenLabsDuoReady = hasUserElevenLabsAPIKey && elevenLabsDuoDistinctVoiceCount >= 2;

  const sectionNavItems = buildSettingsSectionNavItems({
    t,
    configuredProviderCount,
    accessCardCount: accessCards.length,
    readingPlanWindow,
    readingPlanSize,
    readingPlanDiversifyTopics,
    navigatorEnabled,
    navigatorPersonaMode,
    navigatorPersona,
    navigatorModel,
    audioBriefingEnabled,
    audioBriefingScheduleSummary: formatAudioBriefingScheduleSelection(audioBriefingScheduleSelection, t),
    audioBriefingArticlesPerEpisode,
    summaryAudioProvider,
    summaryAudioVoiceModel,
    preferenceProfileStatus: preferenceProfile?.status ?? null,
    preferenceProfileConfidence: preferenceProfile?.confidence ?? null,
    preferenceProfileError: Boolean(preferenceProfileError),
    digestEmailEnabled,
    notificationBriefingEnabled: notificationPriority.briefing_enabled,
    notificationDailyCap: notificationPriority.daily_cap,
    hasInoreaderOAuth: settings.has_inoreader_oauth,
    hasObsidianGithubInstallation: Boolean(settings.obsidian_export?.github_installation_id),
    monthlyBudgetUSD: settings.monthly_budget_usd,
    remainingBudgetPct: settings.current_month.remaining_budget_pct,
  });

  const railNotes = buildSettingsRailNotes({
    t,
    providerModelUpdateCount: visibleProviderModelUpdates.length,
    notificationBriefingEnabled: notificationPriority.briefing_enabled,
    notificationImmediateEnabled: notificationPriority.immediate_enabled,
    notificationDailyCap: notificationPriority.daily_cap,
    currentMonthJST: settings.current_month.month_jst,
    remainingBudgetPct: settings.current_month.remaining_budget_pct,
  });

  const selectedSectionMeta = buildSettingsSectionMeta(activeSection, t);

  const audioBriefingOpenSystemForProvider = (provider: string) => {
    setActiveSection("system");
    setActiveAccessProvider(provider);
  };

  const audioBriefingSettingsForm = {
    onSubmitSettings: submitAudioBriefingSettings,
    savingSettings: savingAudioBriefing,
    onSubmitModels: submitAudioBriefingModels,
    savingModels: savingLLMModels,
  };

  const audioBriefingSettingsState = buildAudioBriefingSettingsState({
    presetsLoading: audioBriefingPresetsLoading,
    presetsCount: audioBriefingPresets.length,
    enabled: audioBriefingEnabled,
    programName: audioBriefingProgramName,
    scheduleSelection: audioBriefingScheduleSelection,
    articlesPerEpisode: audioBriefingArticlesPerEpisode,
    targetDurationMinutes: audioBriefingTargetDurationMinutes,
    chunkTrailingSilenceSeconds: audioBriefingChunkTrailingSilenceSeconds,
    conversationMode: audioBriefingConversationMode,
    defaultPersonaMode: audioBriefingDefaultPersonaMode,
    defaultPersona: audioBriefingDefaultPersona,
    navigatorPersonaCards,
    bgmEnabled: audioBriefingBGMEnabled,
    bgmPrefix: audioBriefingBGMR2Prefix,
  });

  const audioBriefingDuoReadiness = buildAudioBriefingDuoReadiness({
    geminiDuoReady,
    geminiDuoCompatiblePersonaCount,
    geminiDuoCompatibleModel,
    fishDuoReady,
    fishDuoDistinctVoiceCount,
    elevenLabsDuoReady,
    elevenLabsDuoDistinctVoiceCount,
  });

  const audioBriefingScriptModels = buildAudioBriefingScriptModels({
    audioBriefingScriptModel,
    audioBriefingScriptOptions: optionsForPurpose("summary", audioBriefingScriptModel),
    audioBriefingScriptFallbackModel,
    audioBriefingScriptFallbackOptions: optionsForPurpose("summary", audioBriefingScriptFallbackModel),
  });

  const audioBriefingDictionaryState = buildAudioBriefingDictionaryState({
    hasAivisAPIKey: Boolean(settings?.has_aivis_api_key),
    aivisUserDictionariesLoading,
    aivisUserDictionariesError,
    aivisUserDictionaries,
    aivisUserDictionaryUUID,
    savingAivisDictionary,
    deletingAivisDictionary,
    savedAivisUserDictionaryUUID: settings?.aivis_user_dictionary_uuid ?? "",
  });

  const audioBriefingSettingsActions = {
    onOpenPresetApplyModal: () => {
      void openAudioBriefingPresetApplyModal();
    },
    onOpenPresetSaveModal: () => {
      void openAudioBriefingPresetSaveModal();
    },
    onChangeEnabled: setAudioBriefingEnabled,
    onChangeProgramName: setAudioBriefingProgramName,
    onChangeScheduleSelection: setAudioBriefingScheduleSelection,
    onChangeArticlesPerEpisode: setAudioBriefingArticlesPerEpisode,
    onChangeTargetDurationMinutes: setAudioBriefingTargetDurationMinutes,
    onChangeChunkTrailingSilenceSeconds: setAudioBriefingChunkTrailingSilenceSeconds,
    onChangeConversationMode: setAudioBriefingConversationMode,
    onChangeDefaultPersonaMode: setAudioBriefingDefaultPersonaMode,
    onChangeDefaultPersona: setAudioBriefingDefaultPersona,
    onChangeBGMEnabled: setAudioBriefingBGMEnabled,
    onChangeBGMPrefix: setAudioBriefingBGMR2Prefix,
    onChangeAudioBriefingScriptModel: (value: string) => onChangeLLMModel(setAudioBriefingScriptModel, value),
    onChangeAudioBriefingScriptFallbackModel: (value: string) => onChangeLLMModel(setAudioBriefingScriptFallbackModel, value),
    onRefreshAivisUserDictionaries: () => {
      void loadAivisUserDictionaries(true).catch(() => undefined);
    },
    onChangeAivisUserDictionaryUUID: setAivisUserDictionaryUUID,
    onSaveAivisUserDictionary: () => {
      void saveAivisUserDictionary();
    },
    onClearAivisUserDictionary: () => {
      void clearAivisUserDictionary();
    },
    onOpenSystem: () => setActiveSection("system"),
    onOpenSystemForProvider: audioBriefingOpenSystemForProvider,
  };

  const audioBriefingVoiceMatrixForm = {
    onSubmit: submitAudioBriefingVoices,
    saving: savingAudioBriefingVoices,
    onPersistAudioBriefingVoices: () => {
      void persistAudioBriefingVoices();
    },
  };

  const audioBriefingVoiceMatrixStatus = buildAudioBriefingVoiceMatrixStatus({
    readyCount: audioBriefingVoiceReadyCount,
    attentionCount: audioBriefingVoiceAttentionCount,
    configuredCount: configuredAudioBriefingVoiceCount,
    totalCount: audioBriefingVoiceSummaries.length,
    aivisModelsError,
    xaiVoicesError,
    elevenLabsVoicesError,
    geminiTTSVoicesError,
    needsAivisAPIKey: audioBriefingNeedsAivisAPIKey,
    needsXAIAPIKey: audioBriefingNeedsXAIAPIKey,
    needsFishAPIKey: audioBriefingNeedsFishAPIKey,
    needsElevenLabsAPIKey: audioBriefingNeedsElevenLabsAPIKey,
    needsOpenAIAPIKey: audioBriefingNeedsOpenAIAPIKey,
    needsAzureSpeechAPIKey: audioBriefingNeedsAzureSpeechAPIKey,
    needsAzureSpeechRegion: audioBriefingNeedsAzureSpeechRegion,
    needsGeminiAccess: audioBriefingNeedsGeminiAccess,
    aivisLatestSyncedAt: aivisModelsData?.latest_run?.finished_at ?? undefined,
    openAITTSLatestSyncedAt: openAITTSVoicesData?.latest_run?.finished_at ?? undefined,
  });

  const audioBriefingVoiceMatrixAvailability = buildAudioBriefingVoiceMatrixAvailability({
    voiceSummaries: audioBriefingVoiceSummaries,
    expandedPersonas: expandedAudioBriefingPersonas,
    defaultPersona: audioBriefingDefaultPersona,
    conversationMode: audioBriefingConversationMode,
    hasUserFishAPIKey,
    hasUserXAIAPIKey,
    hasUserOpenAIAPIKey,
    hasUserElevenLabsAPIKey,
    hasUserAzureSpeechAPIKey,
    geminiTTSEnabled,
  });

  const audioBriefingVoiceMatrixCatalogs = buildAudioBriefingVoiceMatrixCatalogs({
    audioBriefingAivisModels,
    audioBriefingXAIVoices,
    audioBriefingOpenAITTSVoices,
    audioBriefingGeminiTTSVoices,
    audioBriefingAzureSpeechVoices,
    audioBriefingElevenLabsVoices,
    audioBriefingVoiceInputDrafts,
    aivisModelsSyncing,
    xaiVoicesSyncing,
    openAITTSVoicesSyncing,
    geminiTTSVoicesLoading,
    azureSpeechVoicesLoading,
  });

  const audioBriefingVoiceMatrixActions = {
    onOpenSystemForProvider: audioBriefingOpenSystemForProvider,
    onSyncAivisModels: () => {
      void syncAivisModels();
    },
    onTogglePersona: toggleAudioBriefingPersona,
    onUpdateVoice: updateAudioBriefingVoice,
    onOpenAivisPicker: (persona: string) => {
      void openAivisPicker(persona);
    },
    onOpenFishPicker: (persona: string) => {
      void openFishPicker(persona);
    },
    onOpenXAIPicker: (persona: string) => {
      void openXAIPicker(persona);
    },
    onOpenOpenAITTSPicker: (persona: string) => {
      void openOpenAITTSPicker(persona);
    },
    onOpenGeminiTTSPicker: (persona: string) => {
      void openGeminiTTSPicker(persona);
    },
    onOpenAzureSpeechPicker: (persona: string) => {
      void openAzureSpeechPicker(persona);
    },
    onOpenElevenLabsPicker: (persona: string) => {
      void openElevenLabsPicker(persona);
    },
    onUpdateVoiceNumberInput: (
      persona: string,
      field: "speech_rate" | "tempo_dynamics" | "emotional_intensity" | "line_break_silence_seconds" | "aivis_volume" | "pitch" | "volume_gain",
      raw: string
    ) => {
      const handlers = {
        speech_rate: (value: number) => ({ speech_rate: value }),
        tempo_dynamics: (value: number) => ({ tempo_dynamics: value }),
        emotional_intensity: (value: number) => ({ emotional_intensity: value }),
        line_break_silence_seconds: (value: number) => ({ line_break_silence_seconds: value }),
        aivis_volume: (value: number) => ({ volume_gain: value - 1 }),
        pitch: (value: number) => ({ pitch: value }),
        volume_gain: (value: number) => ({ volume_gain: value }),
      } as const;
      updateAudioBriefingVoiceNumberInput(persona, field, raw, handlers[field]);
    },
    onResetVoiceNumberInput: resetAudioBriefingVoiceNumberInput,
    onSyncXAIVoices: () => {
      void syncXAIVoices();
    },
    onSyncOpenAITTSVoices: () => {
      void syncOpenAITTSVoices().catch(() => undefined);
    },
    onLoadGeminiTTSVoices: () => {
      void loadGeminiTTSVoices().catch(() => undefined);
    },
    onLoadAzureSpeechVoices: () => {
      void loadAzureSpeechVoices().catch(() => undefined);
    },
  };

  const podcastForm = {
    onSubmit: submitPodcastSettings,
    saving: savingPodcast,
  };

  const podcastState = buildPodcastState({
    enabled: podcastEnabled,
    rssURL: podcastRSSURL,
    feedSlug: podcastFeedSlug,
    language: podcastLanguage,
    category: podcastCategory,
    subcategory: podcastSubcategory,
    availableCategories: podcastAvailableCategories,
    selectedCategory: selectedPodcastCategory,
    title: podcastTitle,
    author: podcastAuthor,
    description: podcastDescription,
    artworkURL: podcastArtworkURL,
    uploadingArtwork: uploadingPodcastArtwork,
    explicit: podcastExplicit,
  });

  const podcastActions = {
    onChangeEnabled: setPodcastEnabled,
    onCopyRSSURL: copyPodcastRSSURL,
    onChangeLanguage: setPodcastLanguage,
    onChangeCategory: (value: string) => {
      setPodcastCategory(value);
      setPodcastSubcategory("");
    },
    onChangeSubcategory: setPodcastSubcategory,
    onChangeTitle: setPodcastTitle,
    onChangeAuthor: setPodcastAuthor,
    onChangeDescription: setPodcastDescription,
    onChangeArtworkURL: setPodcastArtworkURL,
    onUploadArtwork: (file: File | null) => {
      void handlePodcastArtworkFileChange(file);
    },
    onUseDefaultArtwork: () => setPodcastArtworkURL(""),
    onChangeExplicit: setPodcastExplicit,
  };

  const summaryAudioForm = {
    onSubmit: submitSummaryAudioSettings,
    saving: summaryAudioSaving,
  };

  const summaryAudioState = buildSummaryAudioState({
    voiceStatus: summaryAudioVoiceStatus,
    configured: summaryAudioConfigured,
    provider: summaryAudioProvider,
    providerCapabilities: summaryAudioProviderCapabilities,
    ttsModel: summaryAudioTTSModel,
    resolvedVoiceLabel: summaryAudioResolvedVoiceLabel,
    resolvedVoiceDetail: summaryAudioResolvedVoiceDetail,
    voicePickerDisabled: summaryAudioProvider === "elevenlabs" && !hasUserElevenLabsAPIKey && !summaryAudioElevenLabsVoices.length,
    voiceStyle: summaryAudioVoiceStyle,
    voiceInputDrafts: summaryAudioVoiceInputDrafts,
  });

  const summaryAudioActions = {
    onChangeProvider: updateSummaryAudioProvider,
    onChangeTTSModel: setSummaryAudioTTSModel,
    onOpenVoicePicker: buildSummaryAudioPickerOpenAction({
      provider: summaryAudioProvider,
      openers: {
        setSummaryAudioAivisPickerOpen: summaryAudioPickers.setSummaryAudioAivisPickerOpen,
        setSummaryAudioFishPickerOpen: summaryAudioPickers.setSummaryAudioFishPickerOpen,
        setSummaryAudioElevenLabsPickerOpen: summaryAudioPickers.setSummaryAudioElevenLabsPickerOpen,
        setSummaryAudioXAIPickerOpen: summaryAudioPickers.setSummaryAudioXAIPickerOpen,
        setSummaryAudioOpenAITTPickerOpen: summaryAudioPickers.setSummaryAudioOpenAITTPickerOpen,
        setSummaryAudioGeminiTTSPickerOpen: summaryAudioPickers.setSummaryAudioGeminiTTSPickerOpen,
        setSummaryAudioAzureSpeechPickerOpen: summaryAudioPickers.setSummaryAudioAzureSpeechPickerOpen,
      },
      aivisModelsData,
      xaiVoicesData,
      elevenLabsVoicesData,
      openAITTSVoicesData,
      geminiTTSVoicesData,
      azureSpeechVoicesData,
      loadAivisModels,
      loadXAIVoices,
      loadElevenLabsVoices,
      loadOpenAITTSVoices,
      loadGeminiTTSVoices,
      loadAzureSpeechVoices,
    }),
    onChangeVoiceStyle: setSummaryAudioVoiceStyle,
    onChangeNumberInput: (field: "speech_rate" | "tempo_dynamics" | "emotional_intensity" | "line_break_silence_seconds" | "pitch" | "volume_gain", raw: string) => {
      const handlers = {
        speech_rate: (value: number) => setSummaryAudioSpeechRate(String(value)),
        emotional_intensity: (value: number) => setSummaryAudioEmotionalIntensity(String(value)),
        tempo_dynamics: (value: number) => setSummaryAudioTempoDynamics(String(value)),
        line_break_silence_seconds: (value: number) => setSummaryAudioLineBreakSilenceSeconds(String(value)),
        pitch: (value: number) => setSummaryAudioPitch(String(value)),
        volume_gain: (value: number) => setSummaryAudioVolumeGain(String(value)),
      } as const;
      updateSummaryAudioVoiceNumberInput(field, raw, handlers[field]);
    },
    onBlurNumberInput: resetSummaryAudioVoiceNumberInput,
    onOpenSystem: () => setActiveSection("system"),
    onChangeAivisUserDictionaryUUID: setSummaryAudioAivisUserDictionaryUUID,
  };

  const audioBriefingPickerSelectActions = buildAudioBriefingPickerSelectActions({
    pickers: audioBriefingPickers,
    audioBriefingVoices,
    activeOpenAITTSVoice,
    activeGeminiTTSVoice,
    activeElevenLabsVoice,
    conversationMode: audioBriefingConversationMode,
    updateAudioBriefingVoice,
  });

  const summaryAudioPickerSelectActions = buildSummaryAudioPickerSelectActions({
    setSummaryAudioProvider,
    setSummaryAudioTTSModel,
    summaryAudioTTSModel,
    setSummaryAudioVoiceModel,
    setSummaryAudioVoiceStyle,
    setSummaryAudioProviderVoiceLabel,
    setSummaryAudioProviderVoiceDescription,
  });

  const summaryAudioIntegrations = buildSummaryAudioIntegrations({
    hasAivisAPIKey: Boolean(settings?.has_aivis_api_key),
    hasFishAPIKey: Boolean(settings?.has_fish_api_key),
    aivisUserDictionaryUUID: summaryAudioAivisUserDictionaryUUID,
    aivisUserDictionariesLoading,
    aivisUserDictionaries,
  });

  const integrationsState = buildIntegrationsState({
    hasInoreaderOAuth: Boolean(settings.has_inoreader_oauth),
    inoreaderTokenExpiresAt: settings.inoreader_token_expires_at,
    deletingInoreaderOAuth,
    obsidianEnabled,
    obsidianGithubConnected: Boolean(settings.obsidian_export?.github_installation_id),
    obsidianRepoOwner,
    obsidianRepoName,
    obsidianRepoBranch,
    obsidianRootPath,
    obsidianLastSuccessAt: settings.obsidian_export?.last_success_at,
    savingObsidianExport,
    runningObsidianExport,
  });

  const integrationsActions = {
    onDeleteInoreaderOAuth: handleDeleteInoreaderOAuth,
    onSubmitObsidianExport: submitObsidianExport,
    onChangeObsidianEnabled: setObsidianEnabled,
    onChangeObsidianRepoOwner: setObsidianRepoOwner,
    onChangeObsidianRepoName: setObsidianRepoName,
    onChangeObsidianRepoBranch: setObsidianRepoBranch,
    onChangeObsidianRootPath: setObsidianRootPath,
    onRunObsidianExportNow: () => {
      void runObsidianExportNow();
    },
  };

  const systemUIFontState = {
    onSubmit: submitUIFontSettings,
    saving: savingUIFontSettings,
    dirty: uiFontsDirty,
    selectedSans: selectedUIFontSans,
    selectedSerif: selectedUIFontSerif,
    onOpenSansPicker: uiFonts.openSansPicker,
    onOpenSerifPicker: uiFonts.openSerifPicker,
  };

  const systemAccessState = buildSystemAccessState({
    configuredProviderCount,
    accessCards,
    activeAccessCard,
    apiKeyCardLabels,
    onSelectProvider: setActiveAccessProvider,
  });

  const llmModelsForm = buildSavingForm({
    onSubmit: submitLLMModels,
    saving: savingLLMModels,
  });

  const llmModelsState = buildLLMModelsState({
    summary: {
      facts: { value: anthropicFactsModel, options: optionsForPurpose("facts", anthropicFactsModel) },
      factsSecondary: { value: anthropicFactsSecondaryModel, options: optionsForPurpose("facts", anthropicFactsSecondaryModel) },
      factsSecondaryRatePercent: anthropicFactsSecondaryRatePercent,
      factsFallback: { value: anthropicFactsFallbackModel, options: optionsForPurpose("facts", anthropicFactsFallbackModel) },
      summary: { value: anthropicSummaryModel, options: optionsForPurpose("summary", anthropicSummaryModel) },
      summarySecondary: { value: anthropicSummarySecondaryModel, options: optionsForPurpose("summary", anthropicSummarySecondaryModel) },
      summarySecondaryRatePercent: anthropicSummarySecondaryRatePercent,
      summaryFallback: { value: anthropicSummaryFallbackModel, options: optionsForPurpose("summary", anthropicSummaryFallbackModel) },
    },
    digest: {
      digestCluster: { value: anthropicDigestClusterModel, options: optionsForPurpose("digest_cluster_draft", anthropicDigestClusterModel) },
      digest: { value: anthropicDigestModel, options: optionsForPurpose("digest", anthropicDigestModel) },
    },
    validation: {
      factsCheck: { value: factsCheckModel, options: optionsForPurpose("facts", factsCheckModel) },
      faithfulnessCheck: { value: faithfulnessCheckModel, options: optionsForPurpose("summary", faithfulnessCheckModel) },
    },
    other: {
      sourceSuggestion: { value: anthropicSourceSuggestionModel, options: sourceSuggestionModelOptions },
      ask: { value: anthropicAskModel, options: optionsForPurpose("ask", anthropicAskModel) },
      embeddings: { value: openAIEmbeddingModel, options: openAIEmbeddingModelOptions },
    },
    preprocess: {
      ttsMarkupPreprocess: { value: ttsMarkupPreprocessModel, options: optionsForChatModel(ttsMarkupPreprocessModel) },
    },
  });

  const llmModelsActions = {
    onChangeModel: (key: string, value: string) => {
      const handlers: Record<string, (next: string) => void> = {
        facts: (next) => onChangeLLMModel(setAnthropicFactsModel, next),
        factsSecondary: (next) => onChangeLLMModel(setAnthropicFactsSecondaryModel, next),
        factsFallback: (next) => onChangeLLMModel(setAnthropicFactsFallbackModel, next),
        summary: (next) => onChangeLLMModel(setAnthropicSummaryModel, next),
        summarySecondary: (next) => onChangeLLMModel(setAnthropicSummarySecondaryModel, next),
        summaryFallback: (next) => onChangeLLMModel(setAnthropicSummaryFallbackModel, next),
        digestCluster: (next) => onChangeLLMModel(setAnthropicDigestClusterModel, next),
        digest: (next) => onChangeLLMModel(setAnthropicDigestModel, next),
        factsCheck: (next) => onChangeLLMModel(setFactsCheckModel, next),
        faithfulnessCheck: (next) => onChangeLLMModel(setFaithfulnessCheckModel, next),
        sourceSuggestion: (next) => onChangeLLMModel(setAnthropicSourceSuggestionModel, next),
        ask: (next) => onChangeLLMModel(setAnthropicAskModel, next),
        embeddings: (next) => onChangeLLMModel(setOpenAIEmbeddingModel, next),
        ttsMarkupPreprocess: (next) => onChangeLLMModel(setTTSMarkupPreprocessModel, next),
      };
      handlers[key]?.(value);
    },
    onChangeRate: (key: "factsSecondaryRatePercent" | "summarySecondaryRatePercent", value: string) => {
      llmModelsDirtyRef.current = true;
      if (key === "factsSecondaryRatePercent") {
        setAnthropicFactsSecondaryRatePercent(value);
      } else {
        setAnthropicSummarySecondaryRatePercent(value);
      }
    },
    onOpenModelGuide: llm.openModelGuide,
    onDismissProviderModelUpdates: dismissProviderModelUpdates,
    onRestoreProviderModelUpdates: restoreProviderModelUpdates,
  };

  const llmModelsExtras = buildLLMModelsExtras({
    llmExtrasOpen: llm.llmExtrasOpen,
    llmExtrasRef,
    providerModelUpdates,
    visibleProviderModelUpdates,
  });

  const navigatorForm = buildSavingForm({
    onSubmit: submitLLMModels,
    saving: savingLLMModels,
  });

  const navigatorState = buildNavigatorState({
    enabled: navigatorEnabled,
    aiNavigatorBriefEnabled,
    personaMode: navigatorPersonaMode,
    persona: navigatorPersona,
    navigatorPersonaCards,
    navigatorModel,
    navigatorModelOptions: optionsForPurpose("summary", navigatorModel),
    navigatorFallbackModel,
    navigatorFallbackModelOptions: optionsForPurpose("summary", navigatorFallbackModel),
    aiNavigatorBriefModel,
    aiNavigatorBriefModelOptions: optionsForPurpose("summary", aiNavigatorBriefModel),
    aiNavigatorBriefFallbackModel,
    aiNavigatorBriefFallbackModelOptions: optionsForPurpose("summary", aiNavigatorBriefFallbackModel),
  });

  const navigatorActions = buildNavigatorActions({
    onChangeEnabled: (value: boolean) => {
      llmModelsDirtyRef.current = true;
      setNavigatorEnabled(value);
    },
    onChangeBriefEnabled: (value: boolean) => {
      llmModelsDirtyRef.current = true;
      setAINavigatorBriefEnabled(value);
    },
    onChangePersonaMode: (value: "fixed" | "random") => {
      llmModelsDirtyRef.current = true;
      setNavigatorPersonaMode(value);
    },
    onSelectPersona: async (personaKey: string) => {
      if (navigatorPersonaMode !== "fixed" || personaKey === navigatorPersona || savingLLMModels) return;
      const previousPersona = settings?.llm_models?.navigator_persona ?? "editor";
      llmModelsDirtyRef.current = true;
      setNavigatorPersona(personaKey);
      setSavingLLMModels(true);
      try {
        await persistLLMModels(
          buildLLMModelPayload({ navigator_persona: personaKey }),
          t("settings.toast.navigatorSaved")
        );
      } catch (e) {
        setNavigatorPersona(previousPersona);
        showToast(localizeSettingsErrorMessage(e, t), "error");
      } finally {
        setSavingLLMModels(false);
      }
    },
    onChangeModel: (key: "navigator" | "navigatorFallback" | "aiNavigatorBrief" | "aiNavigatorBriefFallback", value: string) => {
      const handlers = {
        navigator: (next: string) => onChangeLLMModel(setNavigatorModel, next),
        navigatorFallback: (next: string) => onChangeLLMModel(setNavigatorFallbackModel, next),
        aiNavigatorBrief: (next: string) => onChangeLLMModel(setAINavigatorBriefModel, next),
        aiNavigatorBriefFallback: (next: string) => onChangeLLMModel(setAINavigatorBriefFallbackModel, next),
      } as const;
      handlers[key](value);
    },
  });

  const readingPlanForm = buildSavingForm({
    onSubmit: submitReadingPlan,
    saving: savingReadingPlan,
  });

  const readingPlanState = buildReadingPlanState({
    window: readingPlanWindow,
    size: readingPlanSize,
    diversifyTopics: readingPlanDiversifyTopics,
  });

  const readingPlanActions = buildReadingPlanActions({
    onChangeWindow: setReadingPlanWindow,
    onChangeSize: setReadingPlanSize,
    onChangeDiversifyTopics: setReadingPlanDiversifyTopics,
    windowValue: readingPlanWindow,
    sizeValue: readingPlanSize,
  });

  const digestForm = buildSavingForm({
    onSubmit: submitDigestDelivery,
    saving: savingDigestDelivery,
  });

  const digestState = buildDigestState({
    enabled: digestEmailEnabled,
  });

  const digestActions = buildDigestActions({
    onChangeEnabled: setDigestEmailEnabled,
  });

  const budgetForm = buildSavingForm({
    onSubmit: submitBudget,
    saving: savingBudget,
  });

  const budgetState = buildBudgetState({
    budgetUSD,
    alertEnabled,
    thresholdPct,
    budgetRemainingTone,
    monthJst: settings.current_month.month_jst,
  });

  const budgetActions = buildBudgetActions({
    onChangeBudgetUSD: setBudgetUSD,
    onChangeAlertEnabled: setAlertEnabled,
    onChangeThresholdPct: setThresholdPct,
    budgetValue: budgetUSD,
    thresholdValue: thresholdPct,
  });

  const saveNotificationPriority = async (rule: NotificationPriorityRule) => {
    const res = await api.updateNotificationPriority(rule);
    setNotificationPriority(res.notification_priority);
    setSettings((prev) => (prev ? { ...prev, notification_priority: res.notification_priority } : prev));
  };

  return {
    loading: false as const,
    error: null,
    settings,
    t,
    activeSection,
    setActiveSection,
    sectionNavItems,
    railNotes,
    selectedSectionMeta,
    applyCostPerformancePreset,
    toggleLLMExtras,
    llm,
    modelSelectLabels,
    audioBriefingSettingsForm,
    audioBriefingSettingsState,
    audioBriefingDuoReadiness,
    audioBriefingScriptModels,
    audioBriefingDictionaryState,
    audioBriefingSettingsActions,
    audioBriefingVoiceMatrixForm,
    audioBriefingVoiceMatrixStatus,
    audioBriefingVoiceMatrixAvailability,
    audioBriefingVoiceMatrixCatalogs,
    audioBriefingVoiceMatrixActions,
    podcastForm,
    podcastState,
    podcastActions,
    summaryAudioForm,
    summaryAudioState,
    summaryAudioActions,
    summaryAudioIntegrations,
    readingPlanForm,
    readingPlanState,
    readingPlanActions,
    preferenceProfile,
    preferenceProfileError,
    resettingPreferenceProfile,
    handleResetPreferenceProfile,
    load,
    digestForm,
    digestState,
    digestActions,
    notificationPriority,
    saveNotificationPriority,
    integrationsState,
    integrationsActions,
    llmModelsForm,
    llmModelsState,
    llmModelsActions,
    llmModelsExtras,
    unavailableSelectedModelWarnings,
    navigatorForm,
    navigatorState,
    navigatorActions,
    budgetForm,
    budgetState,
    budgetActions,
    systemUIFontState,
    systemAccessState,
    audioBriefingPickers,
    summaryAudioPickers,
    audioBriefingVoices,
    audioBriefingVoiceSummaries,
    activeAivisVoice,
    activeXAIVoice,
    activeElevenLabsVoice,
    activeOpenAITTSVoice,
    activeGeminiTTSVoice,
    activeAzureSpeechVoice,
    aivisModelsData,
    aivisModelsLoading,
    aivisModelsSyncing,
    aivisModelsError,
    xaiVoicesLoading,
    xaiVoicesSyncing,
    xaiVoicesError,
    openAITTSVoicesLoading,
    openAITTSVoicesSyncing,
    openAITTSVoicesError,
    elevenLabsVoicesLoading,
    elevenLabsVoicesError,
    geminiTTSVoicesLoading,
    geminiTTSVoicesError,
    azureSpeechVoicesLoading,
    azureSpeechVoicesError,
    audioBriefingPresets,
    audioBriefingPresetsLoading,
    audioBriefingPresetsError,
    audioBriefingDefaultPersonaMode,
    audioBriefingDefaultPersona,
    audioBriefingConversationMode,
    audioBriefingVoiceInputDrafts,
    syncAivisModels,
    syncXAIVoices,
    syncOpenAITTSVoices,
    loadElevenLabsVoices,
    loadGeminiTTSVoices,
    loadAzureSpeechVoices,
    audioBriefingPickerSelectActions,
    summaryAudioPickerSelectActions,
    summaryAudioVoiceModel,
    summaryAudioVoiceStyle,
    uiFonts,
    presets,
    uiFontSansOptions,
    uiFontSerifOptions,
    savedUIFontSansKey,
    savedUIFontSerifKey,
    uiFontSansKey,
    uiFontSerifKey,
    setUIFontSansKey,
    setUIFontSerifKey,
    modelComparisonEntries,
    llmExtrasRef,
    audioBriefingAivisModels,
    audioBriefingXAIVoices,
    audioBriefingOpenAITTSVoices,
    audioBriefingGeminiTTSVoices,
    audioBriefingAzureSpeechVoices,
    audioBriefingElevenLabsVoices,
    summaryAudioAivisModels,
    summaryAudioXAIVoices,
    summaryAudioElevenLabsVoices,
    summaryAudioOpenAITTSVoices,
    summaryAudioGeminiTTSVoices,
    summaryAudioAzureSpeechVoices,
    openAudioBriefingPresetApplyModal,
    applyAudioBriefingPreset,
    submitAudioBriefingPresetSave,
    loadAudioBriefingPresets,
    tWithVars,
  };
}
