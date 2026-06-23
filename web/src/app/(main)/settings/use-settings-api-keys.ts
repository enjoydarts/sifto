"use client";

import { type FormEvent, useCallback, useMemo, useState } from "react";

import { api, type UserSettings } from "@/lib/api";
import { createAPIKeyActionHandlers } from "@/components/settings/settings-api-key-actions";
import { buildApiKeyCardLabels } from "@/components/settings/settings-system-helpers";
import { buildAccessCards, createAccessCardRuntime, type AccessCardRuntime } from "@/components/settings/system-access-cards";

type ToastTone = "success" | "error";
type ShowToast = (message: string, tone: ToastTone) => void;
type Translate = (key: string, fallback?: string) => string;
type Confirm = (options: { title: string; message: string; confirmLabel: string; tone: "danger" }) => Promise<boolean>;

type UseSettingsApiKeysArgs = {
  settings: UserSettings | null;
  reload: () => Promise<unknown>;
  confirm: Confirm;
  showToast: ShowToast;
  t: Translate;
  onResetXAIVoices: () => void;
  onMarkAivisUserDictionariesStale: () => void;
  onResetAivisUserDictionaries: () => void;
  onClearAivisUserDictionarySelection: () => void;
  onResetElevenLabsVoices: () => void;
};

export function useSettingsApiKeys({
  settings,
  reload,
  confirm,
  showToast,
  t,
  onResetXAIVoices,
  onMarkAivisUserDictionariesStale,
  onResetAivisUserDictionaries,
  onClearAivisUserDictionarySelection,
  onResetElevenLabsVoices,
}: UseSettingsApiKeysArgs) {
  // Use maps for LLM key states (data-driven, no per-provider useState declarations).
  // Adding a provider no longer requires new useState lines here.
  const [saving, setSaving] = useState<Record<string, boolean>>({});
  const [deleting, setDeleting] = useState<Record<string, boolean>>({});
  const [inputs, setInputs] = useState<Record<string, string>>({});

  const setSavingFor = (p: string) => (v: boolean) => setSaving(m => ({...m, [p]: v}));
  const setDeletingFor = (p: string) => (v: boolean) => setDeleting(m => ({...m, [p]: v}));
  const getInput = (p: string) => inputs[p] || "";
  const setInputFor = (p: string) => (v: string) => setInputs(m => ({...m, [p]: v}));

  // Early ids from payload (catalog) for generation, before later memo. Avoids TDZ and drives data-only lists.
  const llmIdsForGeneration = (() => {
    const s = settings as { llm_api_keys?: Record<string, { has: boolean; last4?: string | null }> } | null;
    const keyed = s?.llm_api_keys;
    return keyed && Object.keys(keyed).length > 0 ? Object.keys(keyed) : [];
  })();

  // Azure region is special (non-LLM map for now)
  const [azureSpeechRegionInput, setAzureSpeechRegionInput] = useState("");

  const [savingAzureSpeechConfig, setSavingAzureSpeechConfig] = useState(false);
  const [deletingAzureSpeechConfig, setDeletingAzureSpeechConfig] = useState(false);

  // Small map only for LLM providers that have special after* side effects.
  // Plain catalog LLM providers require ZERO entries here thanks to generic + i18n convention.
  const llmSpecialCallbacks: Record<string, { afterSave?: () => void; afterDelete?: () => void }> = {
    xai: { afterSave: onResetXAIVoices, afterDelete: onResetXAIVoices },
  };

  // Convert catalog id (e.g. xiaomi_mimo_token_plan) to the camelCase used in i18n dictionaries
  // (e.g. xiaomiMimoTokenPlan). Simple ids pass through unchanged.
  const toI18nProviderBase = (id: string): string =>
    id.split('_').map((part, i) => (i === 0 ? part : part.charAt(0).toUpperCase() + part.slice(1))).join('');

  // Generate definitions purely from llm ids in data (catalog) using generic api + i18n convention.
  // Adding a standard LLM provider: no change to this file (i18n keys still needed per AGENTS).
  const llmDefinitionsBase = (() => {
    const ids = llmIdsForGeneration.length > 0 ? llmIdsForGeneration : [];
    const out: Record<string, unknown> = {};
    for (const id of ids) {
      const cb = llmSpecialCallbacks[id] || {};
      const base = toI18nProviderBase(id);
      out[id] = {
        value: getInput(id),
        setValue: setInputFor(id),
        setSaving: setSavingFor(id),
        setDeleting: setDeletingFor(id),
        save: (k: string) => api.setLlmApiKey(id, k),
        remove: () => api.deleteLlmApiKey(id),
        deleteTitle: t(`settings.${base}DeleteTitle`),
        deleteMessage: t(`settings.${base}DeleteMessage`),
        emptyValueMessage: t("settings.error.enterApiKey"),
        saveSuccessMessage: t(`settings.toast.${base}Saved`),
        deleteSuccessMessage: t(`settings.toast.${base}Deleted`),
        afterSave: cb.afterSave,
        afterDelete: cb.afterDelete,
      };
    }
    return out;
  })();

  const apiKeyHandlers = createAPIKeyActionHandlers({
    confirm,
    confirmLabel: t("settings.delete"),
    reload,
    showToast,
    definitions: {
      ...llmDefinitionsBase,
      aivis: {
        value: getInput("aivis"), setValue: setInputFor("aivis"), setSaving: setSavingFor("aivis"), setDeleting: setDeletingFor("aivis"),
        save: api.setAivisApiKey, remove: api.deleteAivisApiKey,
        deleteTitle: t("settings.aivisDeleteTitle"), deleteMessage: t("settings.aivisDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.aivisSaved"), deleteSuccessMessage: t("settings.toast.aivisDeleted"),
        afterSave: onMarkAivisUserDictionariesStale,
        afterDelete: () => {
          onClearAivisUserDictionarySelection();
          onResetAivisUserDictionaries();
        },
      },
      elevenlabs: {
        value: getInput("elevenlabs"), setValue: setInputFor("elevenlabs"), setSaving: setSavingFor("elevenlabs"), setDeleting: setDeletingFor("elevenlabs"),
        save: api.setElevenLabsApiKey, remove: api.deleteElevenLabsApiKey,
        deleteTitle: t("settings.elevenlabsDeleteTitle"), deleteMessage: t("settings.elevenlabsDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.elevenlabsSaved"), deleteSuccessMessage: t("settings.toast.elevenlabsDeleted"),
        afterSave: onResetElevenLabsVoices,
        afterDelete: onResetElevenLabsVoices,
      },
      cartesia: {
        value: getInput("cartesia"), setValue: setInputFor("cartesia"), setSaving: setSavingFor("cartesia"), setDeleting: setDeletingFor("cartesia"),
        save: api.setCartesiaApiKey, remove: api.deleteCartesiaApiKey,
        deleteTitle: t("settings.cartesiaDeleteTitle"), deleteMessage: t("settings.cartesiaDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.cartesiaSaved"), deleteSuccessMessage: t("settings.toast.cartesiaDeleted"),
      },
      fish: {
        value: getInput("fish"), setValue: setInputFor("fish"), setSaving: setSavingFor("fish"), setDeleting: setDeletingFor("fish"),
        save: api.setFishApiKey, remove: api.deleteFishApiKey,
        deleteTitle: t("settings.fishDeleteTitle"), deleteMessage: t("settings.fishDeleteMessage"),
        emptyValueMessage: t("settings.error.enterApiKey"), saveSuccessMessage: t("settings.toast.fishSaved"), deleteSuccessMessage: t("settings.toast.fishDeleted"),
      },
    },
  });

  // Data-driven providers list (from llm_api_keys populated via catalog iteration).
  const llmProvidersFromData = useMemo(() => {
    const s = settings as { llm_api_keys?: Record<string, { has: boolean; last4?: string | null }> } | null;
    const keyed = s?.llm_api_keys;
    return keyed && Object.keys(keyed).length > 0 ? Object.keys(keyed) : [];
  }, [settings]);
  // llmProvidersFromData is now actively used to produce data-driven cards (see below)


  const submitAzureSpeechConfig = useCallback(async (event: FormEvent) => {
    event.preventDefault();
    setSavingAzureSpeechConfig(true);
    try {
      const apiKey = getInput("azurespeech").trim();
      const region = azureSpeechRegionInput.trim();
      if (!apiKey) throw new Error(t("settings.error.enterApiKey"));
      if (!region) throw new Error(t("settings.azureSpeechRegionRequired"));
      await api.setAzureSpeechConfig(apiKey, region);
      setInputFor("azurespeech")("");
      await reload();
      showToast(t("settings.toast.azureSpeechSaved"), "success");
    } catch (error) {
      showToast(String(error), "error");
    } finally {
      setSavingAzureSpeechConfig(false);
    }
  }, [getInput("azurespeech"), azureSpeechRegionInput, reload, showToast, t]);

  const deleteAzureSpeechConfig = useCallback(async () => {
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
      await reload();
      showToast(t("settings.toast.azureSpeechDeleted"), "success");
    } catch (error) {
      showToast(String(error), "error");
    } finally {
      setDeletingAzureSpeechConfig(false);
    }
  }, [confirm, reload, showToast, t]);

  const apiKeyCardLabels = useMemo(() => buildApiKeyCardLabels(t), [t]);

  // State registry built purely from llm ids in data (catalog-driven) + fixed TTS list.
  // No fallback to specs; new LLM from llm_api_keys just works (no edit to this file).
  const llmStateRegistry: Record<string, {value: string; setValue: (v: string) => void; saving: boolean; deleting: boolean}> = (() => {
    const ttsIds = ["aivis", "elevenlabs", "cartesia", "fish"];
    const ids = [...(llmIdsForGeneration.length > 0 ? llmIdsForGeneration : []), ...ttsIds];
    const r: Record<string, {value: string; setValue: (v: string) => void; saving: boolean; deleting: boolean}> = {};
    for (const id of ids) {
      r[id] = { value: getInput(id), setValue: setInputFor(id), saving: saving[id] || false, deleting: deleting[id] || false };
    }
    return r;
  })();

  const llmCardConfig: Record<string, AccessCardRuntime> = {};
  // Strictly data-driven for LLM (from catalog via llm_api_keys); TTS handled via registry add-below.
  const providersForCards = llmProvidersFromData.length > 0 ? llmProvidersFromData : [];
  providersForCards.forEach((id: string) => {
    const st = llmStateRegistry[id];
    const h = (apiKeyHandlers as unknown as Record<string, { submit: (e: FormEvent) => Promise<void>; remove: () => Promise<void> }>)[id];
    if (st && h) {
      llmCardConfig[id] = createAccessCardRuntime(st.value, st.setValue, h.submit, h.remove, st.saving, st.deleting);
    }
  });

  // Special non-LLM / TTS cards always included (using direct for specials that have extra like region)
  llmCardConfig.azure_speech = createAccessCardRuntime(
    getInput("azurespeech"),
    setInputFor("azurespeech"),
    submitAzureSpeechConfig,
    deleteAzureSpeechConfig,
    savingAzureSpeechConfig,
    deletingAzureSpeechConfig,
    azureSpeechRegionInput,
    setAzureSpeechRegionInput,
  );

  // Add any remaining from registry (TTS etc) that weren't added as LLM
  Object.keys(llmStateRegistry).forEach(id => {
    if (!llmCardConfig[id]) {
      const st = llmStateRegistry[id];
      const h = (apiKeyHandlers as unknown as Record<string, { submit: (e: FormEvent) => Promise<void>; remove: () => Promise<void> }>)[id];
      if (st && st.value !== undefined && h) {
        llmCardConfig[id] = createAccessCardRuntime(st.value, st.setValue, h.submit, h.remove, st.saving, st.deleting);
      }
    }
  });

  const rawAccessCards = buildAccessCards(
    settings,
    llmCardConfig,
    t,
  );

  const accessCards = rawAccessCards;

  return {
    accessCards,
    apiKeyCardLabels,
    llmProviders: llmProvidersFromData,
  };
}
