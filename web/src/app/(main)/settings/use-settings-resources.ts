"use client";

import { useCallback, useEffect, useState } from "react";

import {
  AivisModelsResponse,
  AivisUserDictionary,
  api,
  AzureSpeechVoicesResponse,
  CartesiaTTSCatalogResponse,
  ElevenLabsVoicesResponse,
  GeminiTTSVoicesResponse,
  LLMCatalog,
  OpenAITTSVoicesResponse,
  ProviderModelChangeEvent,
  UserSettings,
  XAIVoicesResponse,
} from "@/lib/api";
import { type SettingsSectionID } from "@/components/settings/settings-page-shell";
import { loadResourceAction, syncResourceAction } from "@/components/settings/settings-resource-actions";
import {
  dismissProviderModelUpdatesToLocalStorage,
  MODEL_UPDATES_DISMISSED_AT_KEY,
  restoreProviderModelUpdatesFromLocalStorage,
} from "@/components/settings/settings-system-helpers";

type ShowToast = (message: string, tone?: "success" | "error" | "info") => void;
type Translate = (key: string, fallback?: string) => string;

type UseSettingsResourcesArgs = {
  activeSection: SettingsSectionID;
  settings: UserSettings | null;
  showToast: ShowToast;
  t: Translate;
};

export function useSettingsResources({
  activeSection,
  settings,
  showToast,
  t,
}: UseSettingsResourcesArgs) {
  const [catalog, setCatalog] = useState<LLMCatalog | null>(null);
  const [providerModelUpdates, setProviderModelUpdates] = useState<ProviderModelChangeEvent[]>([]);
  const [dismissedModelUpdatesAt, setDismissedModelUpdatesAt] = useState<string | null>(() => {
    if (typeof window === "undefined") return null;
    return window.localStorage.getItem(MODEL_UPDATES_DISMISSED_AT_KEY);
  });

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
  const [cartesiaTTSCatalogData, setCartesiaTTSCatalogData] = useState<CartesiaTTSCatalogResponse | null>(null);
  const [cartesiaTTSCatalogLoading, setCartesiaTTSCatalogLoading] = useState(false);
  const [cartesiaTTSCatalogError, setCartesiaTTSCatalogError] = useState<string | null>(null);
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

  const loadCartesiaTTSCatalog = useCallback(async () => {
    return loadResourceAction({
      setLoading: setCartesiaTTSCatalogLoading,
      fetch: api.getCartesiaTTSCatalog,
      setData: setCartesiaTTSCatalogData,
      setError: setCartesiaTTSCatalogError,
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

  const resetXAIVoices = useCallback(() => {
    setXAIVoicesData(null);
    setXAIVoicesError(null);
  }, []);

  const resetElevenLabsVoices = useCallback(() => {
    setElevenLabsVoicesData(null);
    setElevenLabsVoicesError(null);
  }, []);

  const resetAivisUserDictionaries = useCallback(() => {
    setAivisUserDictionaries([]);
    setAivisUserDictionariesLoaded(false);
    setAivisUserDictionariesError(null);
  }, []);

  const markAivisUserDictionariesStale = useCallback(() => {
    setAivisUserDictionariesLoaded(false);
  }, []);

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
    if (
      activeSection !== "summary-audio"
      || !settings?.has_cartesia_api_key
      || cartesiaTTSCatalogData != null
      || cartesiaTTSCatalogLoading
    ) {
      return;
    }
    void loadCartesiaTTSCatalog().catch(() => undefined);
  }, [
    activeSection,
    cartesiaTTSCatalogData,
    cartesiaTTSCatalogLoading,
    loadCartesiaTTSCatalog,
    settings?.has_cartesia_api_key,
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

  function dismissProviderModelUpdates() {
    setDismissedModelUpdatesAt(dismissProviderModelUpdatesToLocalStorage(providerModelUpdates));
  }

  function restoreProviderModelUpdates() {
    setDismissedModelUpdatesAt(restoreProviderModelUpdatesFromLocalStorage());
  }

  return {
    catalog,
    setCatalog,
    providerModelUpdates,
    dismissedModelUpdatesAt,
    dismissProviderModelUpdates,
    restoreProviderModelUpdates,
    aivisModelsData,
    setAivisModelsData,
    aivisModelsLoading,
    aivisModelsSyncing,
    aivisModelsError,
    setAivisModelsError,
    xaiVoicesData,
    setXAIVoicesData,
    xaiVoicesLoading,
    xaiVoicesSyncing,
    xaiVoicesError,
    setXAIVoicesError,
    openAITTSVoicesData,
    setOpenAITTSVoicesData,
    openAITTSVoicesLoading,
    openAITTSVoicesSyncing,
    openAITTSVoicesError,
    setOpenAITTSVoicesError,
    elevenLabsVoicesData,
    setElevenLabsVoicesData,
    elevenLabsVoicesLoading,
    elevenLabsVoicesError,
    setElevenLabsVoicesError,
    cartesiaTTSCatalogData,
    setCartesiaTTSCatalogData,
    cartesiaTTSCatalogLoading,
    cartesiaTTSCatalogError,
    setCartesiaTTSCatalogError,
    geminiTTSVoicesData,
    geminiTTSVoicesLoading,
    geminiTTSVoicesError,
    azureSpeechVoicesData,
    azureSpeechVoicesLoading,
    azureSpeechVoicesError,
    aivisUserDictionaries,
    setAivisUserDictionaries,
    aivisUserDictionariesLoading,
    aivisUserDictionariesLoaded,
    setAivisUserDictionariesLoaded,
    aivisUserDictionariesError,
    setAivisUserDictionariesError,
    loadAivisModels,
    loadXAIVoices,
    loadOpenAITTSVoices,
    loadElevenLabsVoices,
    loadCartesiaTTSCatalog,
    loadGeminiTTSVoices,
    loadAzureSpeechVoices,
    syncAivisModels,
    syncXAIVoices,
    syncOpenAITTSVoices,
    loadAivisUserDictionaries,
    resetXAIVoices,
    resetElevenLabsVoices,
    resetAivisUserDictionaries,
    markAivisUserDictionariesStale,
  };
}
