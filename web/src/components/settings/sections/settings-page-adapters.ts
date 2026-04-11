"use client";

import type { FormEvent } from "react";

export function buildAudioBriefingSettingsState<T extends {
  presetsLoading: boolean;
  presetsCount: number;
  enabled: boolean;
  programName: string;
  scheduleSelection: "interval3h" | "interval6h" | "fixed3x";
  articlesPerEpisode: string;
  targetDurationMinutes: string;
  chunkTrailingSilenceSeconds: string;
  conversationMode: "single" | "duo";
  defaultPersonaMode: "fixed" | "random";
  defaultPersona: string;
  navigatorPersonaCards: Array<{ key: string }>;
  bgmEnabled: boolean;
  bgmPrefix: string;
}>(params: T): T {
  return { ...params };
}

export function buildAudioBriefingDuoReadiness<T extends {
  geminiDuoReady: boolean;
  geminiDuoCompatiblePersonaCount: number;
  geminiDuoCompatibleModel: string;
  fishDuoReady: boolean;
  fishDuoDistinctVoiceCount: number;
  elevenLabsDuoReady: boolean;
  elevenLabsDuoDistinctVoiceCount: number;
}>(params: T): T {
  return { ...params };
}

export function buildAudioBriefingScriptModels<T extends {
  audioBriefingScriptModel: string;
  audioBriefingScriptOptions: unknown[];
  audioBriefingScriptFallbackModel: string;
  audioBriefingScriptFallbackOptions: unknown[];
}>(params: T): T {
  return { ...params };
}

export function buildAudioBriefingDictionaryState<T extends {
  hasAivisAPIKey: boolean;
  aivisUserDictionariesLoading: boolean;
  aivisUserDictionariesError: string | null;
  aivisUserDictionaries: unknown[];
  aivisUserDictionaryUUID: string;
  savingAivisDictionary: boolean;
  deletingAivisDictionary: boolean;
  savedAivisUserDictionaryUUID: string;
}>(params: T): T {
  return { ...params };
}

export function buildAudioBriefingVoiceMatrixStatus<T extends {
  readyCount: number;
  attentionCount: number;
  configuredCount: number;
  totalCount: number;
  aivisModelsError: string | null;
  xaiVoicesError: string | null;
  elevenLabsVoicesError: string | null;
  geminiTTSVoicesError: string | null;
  needsAivisAPIKey: boolean;
  needsXAIAPIKey: boolean;
  needsFishAPIKey: boolean;
  needsElevenLabsAPIKey: boolean;
  needsOpenAIAPIKey: boolean;
  needsAzureSpeechAPIKey: boolean;
  needsAzureSpeechRegion: boolean;
  needsGeminiAccess: boolean;
  aivisLatestSyncedAt?: string;
  openAITTSLatestSyncedAt?: string;
}>(params: T): T {
  return { ...params };
}

export function buildAudioBriefingVoiceMatrixAvailability<T extends {
  voiceSummaries: unknown[];
  expandedPersonas: string[];
  defaultPersona: string;
  hasUserFishAPIKey: boolean;
  hasUserXAIAPIKey: boolean;
  hasUserOpenAIAPIKey: boolean;
  hasUserElevenLabsAPIKey: boolean;
  hasUserAzureSpeechAPIKey: boolean;
  geminiTTSEnabled: boolean;
}>(params: T): T {
  return { ...params };
}

export function buildAudioBriefingVoiceMatrixCatalogs<T extends {
  audioBriefingAivisModels: unknown[];
  audioBriefingXAIVoices: unknown[];
  audioBriefingOpenAITTSVoices: unknown[];
  audioBriefingGeminiTTSVoices: unknown[];
  audioBriefingAzureSpeechVoices: unknown[];
  audioBriefingElevenLabsVoices: unknown[];
  audioBriefingVoiceInputDrafts: Record<string, unknown>;
  aivisModelsSyncing: boolean;
  xaiVoicesSyncing: boolean;
  openAITTSVoicesSyncing: boolean;
  geminiTTSVoicesLoading: boolean;
  azureSpeechVoicesLoading: boolean;
}>(params: T): T {
  return { ...params };
}

export function buildPodcastState<T extends {
  enabled: boolean;
  rssURL: string;
  feedSlug: string;
  language: string;
  category: string;
  subcategory: string;
  availableCategories: unknown[];
  selectedCategory: unknown;
  title: string;
  author: string;
  description: string;
  artworkURL: string;
  uploadingArtwork: boolean;
  explicit: boolean;
}>(params: T): T {
  return { ...params };
}

export function buildSummaryAudioState<T extends {
  voiceStatus: unknown;
  configured: boolean;
  provider: string;
  providerCapabilities: unknown;
  ttsModel: string;
  resolvedVoiceLabel: string;
  resolvedVoiceDetail: string;
  voicePickerDisabled: boolean;
  voiceStyle: string;
  voiceInputDrafts: Record<string, string>;
}>(params: T): T {
  return { ...params };
}

export function buildSummaryAudioIntegrations<T extends {
  hasAivisAPIKey: boolean;
  hasFishAPIKey: boolean;
  aivisUserDictionaryUUID: string;
  aivisUserDictionariesLoading: boolean;
  aivisUserDictionaries: unknown[];
}>(params: T): T {
  return { ...params };
}

export function buildIntegrationsState<T extends {
  hasInoreaderOAuth: boolean;
  inoreaderTokenExpiresAt: string | null | undefined;
  deletingInoreaderOAuth: boolean;
  obsidianEnabled: boolean;
  obsidianGithubConnected: boolean;
  obsidianRepoOwner: string;
  obsidianRepoName: string;
  obsidianRepoBranch: string;
  obsidianRootPath: string;
  obsidianLastSuccessAt: string | null | undefined;
  savingObsidianExport: boolean;
  runningObsidianExport: boolean;
}>(params: T): T {
  return { ...params };
}

export function buildSavingForm<T extends {
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  saving: boolean;
}>(params: T): T {
  return { ...params };
}

export function buildReadingPlanActions<T extends {
  onChangeWindow: (value: T["windowValue"]) => void;
  onChangeSize: (value: T["sizeValue"]) => void;
  onChangeDiversifyTopics: (value: boolean) => void;
  windowValue: unknown;
  sizeValue: unknown;
}>(params: T): Omit<T, "windowValue" | "sizeValue"> {
  const { windowValue: _windowValue, sizeValue: _sizeValue, ...rest } = params;
  return rest;
}

export function buildDigestActions<T extends {
  onChangeEnabled: (value: boolean) => void;
}>(params: T): T {
  return { ...params };
}

export function buildBudgetActions<T extends {
  onChangeBudgetUSD: (value: T["budgetValue"]) => void;
  onChangeAlertEnabled: (value: boolean) => void;
  onChangeThresholdPct: (value: T["thresholdValue"]) => void;
  budgetValue: unknown;
  thresholdValue: unknown;
}>(params: T): Omit<T, "budgetValue" | "thresholdValue"> {
  const { budgetValue: _budgetValue, thresholdValue: _thresholdValue, ...rest } = params;
  return rest;
}

export function buildSystemAccessState<T extends {
  configuredProviderCount: number;
  accessCards: unknown[];
  activeAccessCard: unknown;
  apiKeyCardLabels: Record<string, string>;
  onSelectProvider: (provider: string) => void;
}>(params: T): T {
  return { ...params };
}

export function buildLLMModelsState<T extends {
  summary: Record<string, unknown>;
  digest: Record<string, unknown>;
  validation: Record<string, unknown>;
  other: Record<string, unknown>;
  preprocess: Record<string, unknown>;
}>(params: T): T {
  return { ...params };
}

export function buildLLMModelsExtras<T extends {
  llmExtrasOpen: boolean;
  llmExtrasRef: unknown;
  providerModelUpdates: unknown[];
  visibleProviderModelUpdates: unknown[];
}>(params: T): T {
  return { ...params };
}

export function buildNavigatorState<T extends {
  enabled: boolean;
  aiNavigatorBriefEnabled: boolean;
  personaMode: "fixed" | "random";
  persona: string;
  navigatorPersonaCards: unknown[];
  navigatorModel: string;
  navigatorModelOptions: unknown[];
  navigatorFallbackModel: string;
  navigatorFallbackModelOptions: unknown[];
  aiNavigatorBriefModel: string;
  aiNavigatorBriefModelOptions: unknown[];
  aiNavigatorBriefFallbackModel: string;
  aiNavigatorBriefFallbackModelOptions: unknown[];
}>(params: T): T {
  return { ...params };
}

export function buildNavigatorActions<T extends {
  onChangeEnabled: (value: boolean) => void;
  onChangeBriefEnabled: (value: boolean) => void;
  onChangePersonaMode: (value: "fixed" | "random") => void;
  onSelectPersona: (personaKey: string) => void | Promise<void>;
  onChangeModel: (key: "navigator" | "navigatorFallback" | "aiNavigatorBrief" | "aiNavigatorBriefFallback", value: string) => void;
}>(params: T): T {
  return { ...params };
}

export function buildReadingPlanState<T extends {
  window: string;
  size: string;
  diversifyTopics: boolean;
}>(params: T): T {
  return { ...params };
}

export function buildDigestState<T extends {
  enabled: boolean;
}>(params: T): T {
  return { ...params };
}

export function buildBudgetState<T extends {
  budgetUSD: string;
  alertEnabled: boolean;
  thresholdPct: number;
  budgetRemainingTone: string;
  monthJst: string;
}>(params: T): T {
  return { ...params };
}
