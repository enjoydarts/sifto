"use client";

import { type Dispatch, type FormEvent, type RefObject, type SetStateAction, useCallback, useMemo, useRef, useState } from "react";
import { type QueryClient } from "@tanstack/react-query";

import { api, type LLMCatalog, type NavigatorPersonaDefinition, type ProviderModelChangeEvent, type UserSettings } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { type ModelOption } from "@/components/settings/model-select";
import { submitSavingFormAction } from "@/components/settings/settings-submit-actions";
import {
  buildAudioBriefingScriptModels,
  buildLLMModelsExtras,
  buildLLMModelsState,
  buildNavigatorActions,
  buildNavigatorState,
  buildSavingForm,
} from "@/components/settings/sections/settings-page-adapters";
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

type ToastTone = "success" | "error";
type ShowToast = (message: string, tone: ToastTone) => void;
type Translate = (key: string, fallback?: string) => string;

type LLMDialogState = {
  llmExtrasOpen: boolean;
  setLLMExtrasOpen: Dispatch<SetStateAction<boolean>>;
  openModelGuide: () => void;
};

type NavigatorPersonaCard = { key: string } & NavigatorPersonaDefinition;

type LLMModelPayload = Partial<{
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
  facts_check_fallback: string | null;
  faithfulness_check: string | null;
  faithfulness_check_fallback: string | null;
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
}>;

type UseSettingsLLMModelsArgs = {
  settings: UserSettings | null;
  setSettings: Dispatch<SetStateAction<UserSettings | null>>;
  catalog: LLMCatalog | null;
  providerModelUpdates: ProviderModelChangeEvent[];
  dismissedModelUpdatesAt: string | null;
  dismissProviderModelUpdates: () => void;
  restoreProviderModelUpdates: () => void;
  navigatorPersonaCards: NavigatorPersonaCard[];
  queryClient: QueryClient;
  showToast: ShowToast;
  t: Translate;
  llm: LLMDialogState;
};

export function useSettingsLLMModels({
  settings,
  setSettings,
  catalog,
  providerModelUpdates,
  dismissedModelUpdatesAt,
  dismissProviderModelUpdates,
  restoreProviderModelUpdates,
  navigatorPersonaCards,
  queryClient,
  showToast,
  t,
  llm,
}: UseSettingsLLMModelsArgs) {
  const [savingLLMModels, setSavingLLMModels] = useState(false);
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
  const [factsCheckFallbackModel, setFactsCheckFallbackModel] = useState("");
  const [faithfulnessCheckModel, setFaithfulnessCheckModel] = useState("");
  const [faithfulnessCheckFallbackModel, setFaithfulnessCheckFallbackModel] = useState("");
  const [navigatorEnabled, setNavigatorEnabled] = useState(false);
  const [aiNavigatorBriefEnabled, setAINavigatorBriefEnabled] = useState(false);
  const [navigatorPersonaMode, setNavigatorPersonaMode] = useState<"fixed" | "random">("fixed");
  const [navigatorPersona, setNavigatorPersona] = useState("editor");
  const [navigatorModel, setNavigatorModel] = useState("");
  const [navigatorFallbackModel, setNavigatorFallbackModel] = useState("");
  const [aiNavigatorBriefModel, setAINavigatorBriefModel] = useState("");
  const [aiNavigatorBriefFallbackModel, setAINavigatorBriefFallbackModel] = useState("");
  const [audioBriefingScriptModel, setAudioBriefingScriptModel] = useState("");
  const [audioBriefingScriptFallbackModel, setAudioBriefingScriptFallbackModel] = useState("");
  const [ttsMarkupPreprocessModel, setTTSMarkupPreprocessModel] = useState("");
  const llmModelsDirtyRef = useRef(false);
  const llmExtrasRef = useRef<HTMLDivElement | null>(null);

  const syncForm = useCallback((llmModels?: UserSettings["llm_models"] | null) => {
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
    setFactsCheckFallbackModel(llmModels?.facts_check_fallback ?? "");
    setFaithfulnessCheckModel(llmModels?.faithfulness_check ?? "");
    setFaithfulnessCheckFallbackModel(llmModels?.faithfulness_check_fallback ?? "");
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

  const isDirty = useCallback(() => llmModelsDirtyRef.current, []);

  const onChangeLLMModel = useCallback((setter: (value: string) => void, value: string) => {
    llmModelsDirtyRef.current = true;
    setter(value);
  }, []);

  const buildLLMModelPayload = useCallback(
    (overrides?: LLMModelPayload) => {
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
        facts_check_fallback: emptyToNull(factsCheckFallbackModel),
        faithfulness_check: emptyToNull(faithfulnessCheckModel),
        faithfulness_check_fallback: emptyToNull(faithfulnessCheckFallbackModel),
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
      factsCheckFallbackModel,
      faithfulnessCheckModel,
      faithfulnessCheckFallbackModel,
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
      syncForm(resp.llm_models);
      llmModelsDirtyRef.current = false;
      if (successMessage) {
        showToast(successMessage, "success");
      }
      return resp;
    },
    [queryClient, setSettings, showToast, syncForm]
  );

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
    setFactsCheckFallbackModel("");
    setFaithfulnessCheckModel(preset.faithfulness_check ?? "");
    setFaithfulnessCheckFallbackModel("");
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

  const toggleLLMExtras = useCallback(() => {
    llm.setLLMExtrasOpen((prev) => {
      const next = !prev;
      if (next) {
        window.requestAnimationFrame(() => {
          llmExtrasRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
        });
      }
      return next;
    });
  }, [llm]);

  const submitLLMModels = useCallback(
    async (e: FormEvent) => {
      await submitSavingFormAction({
        event: e,
        setSaving: setSavingLLMModels,
        showToast,
        mapError: (error) => localizeSettingsErrorMessage(error, t),
        run: () => persistLLMModels(buildLLMModelPayload(), t("settings.toast.modelsSaved")),
      });
    },
    [buildLLMModelPayload, persistLLMModels, showToast, t]
  );

  const submitAudioBriefingModels = useCallback(
    async (e: FormEvent) => {
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
    },
    [
      audioBriefingScriptFallbackModel,
      audioBriefingScriptModel,
      buildLLMModelPayload,
      persistLLMModels,
      showToast,
      t,
    ]
  );

  const audioBriefingScriptModels = buildAudioBriefingScriptModels({
    audioBriefingScriptModel,
    audioBriefingScriptOptions: optionsForPurpose("summary", audioBriefingScriptModel),
    audioBriefingScriptFallbackModel,
    audioBriefingScriptFallbackOptions: optionsForPurpose("summary", audioBriefingScriptFallbackModel),
  });

  const audioBriefingScriptActions = {
    onChangeAudioBriefingScriptModel: (value: string) => onChangeLLMModel(setAudioBriefingScriptModel, value),
    onChangeAudioBriefingScriptFallbackModel: (value: string) => onChangeLLMModel(setAudioBriefingScriptFallbackModel, value),
  };

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
      factsCheckFallback: { value: factsCheckFallbackModel, options: optionsForPurpose("facts", factsCheckFallbackModel) },
      faithfulnessCheck: { value: faithfulnessCheckModel, options: optionsForPurpose("summary", faithfulnessCheckModel) },
      faithfulnessCheckFallback: { value: faithfulnessCheckFallbackModel, options: optionsForPurpose("summary", faithfulnessCheckFallbackModel) },
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
        factsCheckFallback: (next) => onChangeLLMModel(setFactsCheckFallbackModel, next),
        faithfulnessCheck: (next) => onChangeLLMModel(setFaithfulnessCheckModel, next),
        faithfulnessCheckFallback: (next) => onChangeLLMModel(setFaithfulnessCheckFallbackModel, next),
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
    llmExtrasRef: llmExtrasRef as RefObject<HTMLDivElement>,
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

  return {
    savingLLMModels,
    syncForm,
    isDirty,
    modelSelectLabels,
    applyCostPerformancePreset,
    toggleLLMExtras,
    llmModelsForm,
    llmModelsState,
    llmModelsActions,
    llmModelsExtras,
    unavailableSelectedModelWarnings,
    modelComparisonEntries,
    llmExtrasRef,
    visibleProviderModelUpdates,
    navigatorForm,
    navigatorState,
    navigatorActions,
    navigatorSummary: {
      enabled: navigatorEnabled,
      personaMode: navigatorPersonaMode,
      persona: navigatorPersona,
      model: navigatorModel,
    },
    audioBriefingScriptModels,
    audioBriefingScriptActions,
    audioBriefingSettingsForm: {
      onSubmitModels: submitAudioBriefingModels,
      savingModels: savingLLMModels,
    },
  };
}
