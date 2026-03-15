"use client";

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { BellRing, Brain, ChevronDown, Coins, KeyRound, Link2, Mail, Settings as SettingsIcon } from "lucide-react";
import { api, LLMCatalog, LLMCatalogModel, NotificationPriorityRule, ProviderModelChangeEvent, ReadingGoal, UserSettings } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";
import OneSignalSettings from "@/components/onesignal-settings";
import ApiKeyCard from "@/components/settings/api-key-card";
import ModelGuideTable from "@/components/settings/model-guide-table";
import ModelSelect, { type ModelOption } from "@/components/settings/model-select";
import ProviderModelUpdatesPanel from "@/components/settings/provider-model-updates-panel";
import SettingsMetricCard from "@/components/settings/settings-metric-card";

const MODEL_UPDATES_DISMISSED_AT_KEY = "provider-model-updates:dismissed-at";

function formatUSDPerMTok(value: number): string {
  const rounded = value >= 1 ? value.toFixed(2) : value.toFixed(4);
  return `$${rounded.replace(/\.?0+$/, "")}`;
}

function formatModelOptionNote(item: LLMCatalogModel): string | undefined {
  if (!item.pricing) return undefined;
  const parts: string[] = [];
  if (item.pricing.cache_read_per_mtok_usd > 0) {
    parts.push(`cached in ${formatUSDPerMTok(item.pricing.cache_read_per_mtok_usd)}`);
  }
  parts.push(`in ${formatUSDPerMTok(item.pricing.input_per_mtok_usd)}`);
  if (item.pricing.output_per_mtok_usd > 0) {
    parts.push(`out ${formatUSDPerMTok(item.pricing.output_per_mtok_usd)}`);
  }
  parts.push("1M tok");
  return parts.join(" / ");
}

function firstMatchingModelId(models: LLMCatalogModel[], candidates: string[]): string {
  for (const candidate of candidates) {
    if (models.some((item) => item.id === candidate)) {
      return candidate;
    }
  }
  return "";
}

function buildCostPerformancePreset(catalog: LLMCatalog | null): NonNullable<UserSettings["llm_models"]> {
  const chatModels = catalog?.chat_models ?? [];
  const embeddingModels = catalog?.embedding_models ?? [];
  const purposeModels = (purpose: string) =>
    chatModels.filter((item) => (item.available_purposes ?? []).includes(purpose));

  return {
    facts: firstMatchingModelId(purposeModels("facts"), [
      "openai/gpt-oss-20b",
      "gemini-2.5-flash-lite",
      "gpt-5-mini",
    ]),
    summary: firstMatchingModelId(purposeModels("summary"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5",
    ]),
    digest_cluster: firstMatchingModelId(purposeModels("digest_cluster_draft"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5",
    ]),
    digest: firstMatchingModelId(purposeModels("digest"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5",
    ]),
    ask: firstMatchingModelId(purposeModels("ask"), [
      "openai/gpt-oss-20b",
      "gemini-2.5-flash",
      "gpt-5-mini",
    ]),
    source_suggestion: firstMatchingModelId(purposeModels("source_suggestion"), [
      "openai/gpt-oss-20b",
      "gemini-2.5-flash-lite",
      "gpt-5-mini",
    ]),
    facts_check: firstMatchingModelId(purposeModels("facts"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5",
    ]),
    faithfulness_check: firstMatchingModelId(purposeModels("summary"), [
      "openai/gpt-oss-120b",
      "gemini-2.5-flash",
      "gpt-5",
    ]),
    embedding: firstMatchingModelId(embeddingModels, [
      "text-embedding-3-small",
      "text-embedding-3-large",
    ]),
  };
}


export default function SettingsPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const [loading, setLoading] = useState(true);
  const [savingBudget, setSavingBudget] = useState(false);
  const [savingDigestDelivery, setSavingDigestDelivery] = useState(false);
  const [savingReadingPlan, setSavingReadingPlan] = useState(false);
  const [savingObsidianExport, setSavingObsidianExport] = useState(false);
  const [runningObsidianExport, setRunningObsidianExport] = useState(false);
  const [savingReadingGoal, setSavingReadingGoal] = useState(false);
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
  const [savingXAIKey, setSavingXAIKey] = useState(false);
  const [deletingXAIKey, setDeletingXAIKey] = useState(false);
  const [deletingInoreaderOAuth, setDeletingInoreaderOAuth] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [settings, setSettings] = useState<UserSettings | null>(null);
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
  const [xaiApiKeyInput, setXaiApiKeyInput] = useState("");
  const [activeAccessProvider, setActiveAccessProvider] = useState("anthropic");
  const [llmSectionOpen, setLLMSectionOpen] = useState(true);
  const [goalsSectionOpen, setGoalsSectionOpen] = useState(true);
  const [operationsSectionOpen, setOperationsSectionOpen] = useState(true);
  const [notificationsSectionOpen, setNotificationsSectionOpen] = useState(true);
  const [integrationsSectionOpen, setIntegrationsSectionOpen] = useState(true);
  const [llmExtrasOpen, setLLMExtrasOpen] = useState(false);
  const [readingPlanWindow, setReadingPlanWindow] = useState<"24h" | "today_jst" | "7d">("24h");
  const [readingPlanSize, setReadingPlanSize] = useState<string>("15");
  const [readingPlanDiversifyTopics, setReadingPlanDiversifyTopics] = useState(true);
  const [activeReadingGoals, setActiveReadingGoals] = useState<ReadingGoal[]>([]);
  const [archivedReadingGoals, setArchivedReadingGoals] = useState<ReadingGoal[]>([]);
  const [editingReadingGoalId, setEditingReadingGoalId] = useState<string | null>(null);
  const [readingGoalTitle, setReadingGoalTitle] = useState("");
  const [readingGoalDescription, setReadingGoalDescription] = useState("");
  const [readingGoalPriority, setReadingGoalPriority] = useState("3");
  const [readingGoalDueDate, setReadingGoalDueDate] = useState("");
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
  const [anthropicSummaryModel, setAnthropicSummaryModel] = useState("");
  const [anthropicDigestClusterModel, setAnthropicDigestClusterModel] = useState("");
  const [anthropicDigestModel, setAnthropicDigestModel] = useState("");
  const [anthropicAskModel, setAnthropicAskModel] = useState("");
  const [anthropicSourceSuggestionModel, setAnthropicSourceSuggestionModel] = useState("");
  const [openAIEmbeddingModel, setOpenAIEmbeddingModel] = useState("");
  const [factsCheckModel, setFactsCheckModel] = useState("");
  const [faithfulnessCheckModel, setFaithfulnessCheckModel] = useState("");
  const loadSeqRef = useRef(0);
  const llmModelsDirtyRef = useRef(false);
  const llmExtrasRef = useRef<HTMLDivElement | null>(null);

  const syncLLMModelForm = useCallback((llmModels?: UserSettings["llm_models"] | null) => {
    setAnthropicFactsModel(llmModels?.facts ?? "");
    setAnthropicSummaryModel(llmModels?.summary ?? "");
    setAnthropicDigestClusterModel(llmModels?.digest_cluster ?? "");
    setAnthropicDigestModel(llmModels?.digest ?? "");
    setAnthropicAskModel(llmModels?.ask ?? "");
    setAnthropicSourceSuggestionModel(llmModels?.source_suggestion ?? "");
    setOpenAIEmbeddingModel(llmModels?.embedding ?? "");
    setFactsCheckModel(llmModels?.facts_check ?? "");
    setFaithfulnessCheckModel(llmModels?.faithfulness_check ?? "");
  }, []);

  const onChangeLLMModel = useCallback((setter: (value: string) => void, value: string) => {
    llmModelsDirtyRef.current = true;
    setter(value);
  }, []);

  const load = useCallback(async () => {
    const seq = ++loadSeqRef.current;
    setLoading(true);
    try {
      const [data, nextCatalog, readingGoals] = await Promise.all([
        api.getSettings(),
        api.getLLMCatalog(),
        api.getReadingGoals(),
      ]);
      if (seq !== loadSeqRef.current) return;
      setSettings(data);
      setCatalog(nextCatalog);
      setBudgetUSD(data.monthly_budget_usd == null ? "" : String(data.monthly_budget_usd));
      setAlertEnabled(Boolean(data.budget_alert_enabled));
      setThresholdPct(data.budget_alert_threshold_pct ?? 20);
      setDigestEmailEnabled(Boolean(data.digest_email_enabled ?? true));
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
      setActiveReadingGoals(readingGoals.active ?? []);
      setArchivedReadingGoals(readingGoals.archived ?? []);
      if (!llmModelsDirtyRef.current) {
        syncLLMModelForm(data.llm_models);
      }
      setError(null);
    } catch (e) {
      if (seq !== loadSeqRef.current) return;
      setError(String(e));
    } finally {
      if (seq === loadSeqRef.current) {
        setLoading(false);
      }
    }
  }, [syncLLMModelForm]);

  useEffect(() => {
    load();
  }, [load]);

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

  const toModelOption = useCallback((item: LLMCatalogModel): ModelOption => ({
    value: item.id,
    label: item.id,
    note: formatModelOptionNote(item),
    provider: t(`settings.modelGuide.provider.${item.provider}`, item.provider),
  }), []);

  const modelSelectLabels = useMemo(() => ({
    defaultOption: t("settings.modelSelect.default"),
    searchPlaceholder: t("settings.modelSelect.searchPlaceholder"),
    noResults: t("settings.modelSelect.noResults"),
  }), [t]);

  const applyCostPerformancePreset = useCallback(() => {
    const preset = buildCostPerformancePreset(catalog);
    llmModelsDirtyRef.current = true;
    setAnthropicFactsModel(preset.facts ?? "");
    setAnthropicSummaryModel(preset.summary ?? "");
    setAnthropicDigestClusterModel(preset.digest_cluster ?? "");
    setAnthropicDigestModel(preset.digest ?? "");
    setAnthropicAskModel(preset.ask ?? "");
    setAnthropicSourceSuggestionModel(preset.source_suggestion ?? "");
    setOpenAIEmbeddingModel(preset.embedding ?? "");
    setFactsCheckModel(preset.facts_check ?? "");
    setFaithfulnessCheckModel(preset.faithfulness_check ?? "");
  }, [catalog]);

  const optionsForPurpose = useCallback(
    (purpose: string): ModelOption[] =>
      (catalog?.chat_models ?? [])
        .filter((item) => (item.available_purposes ?? []).includes(purpose))
        .map(toModelOption),
    [catalog?.chat_models, toModelOption]
  );

  const sourceSuggestionModelOptions = useMemo(() => optionsForPurpose("source_suggestion"), [optionsForPurpose]);
  const openAIEmbeddingModelOptions = useMemo(
    () => (catalog?.embedding_models ?? []).map(toModelOption),
    [catalog?.embedding_models, toModelOption]
  );
  const modelComparisonEntries = useMemo(
    () => [...(catalog?.chat_models ?? []), ...(catalog?.embedding_models ?? [])],
    [catalog?.chat_models, catalog?.embedding_models]
  );
  const visibleProviderModelUpdates = useMemo(() => {
    if (!dismissedModelUpdatesAt) return providerModelUpdates;
    const dismissedMs = Date.parse(dismissedModelUpdatesAt);
    if (Number.isNaN(dismissedMs)) return providerModelUpdates;
    return providerModelUpdates.filter((event) => Date.parse(event.detected_at) > dismissedMs);
  }, [dismissedModelUpdatesAt, providerModelUpdates]);

  const budgetRemainingTone = useMemo(() => {
    const v = settings?.current_month.remaining_budget_pct;
    if (v == null) return "text-zinc-700";
    if (v < 0) return "text-red-600";
    if (v < thresholdPct) return "text-amber-600";
    return "text-zinc-700";
  }, [settings?.current_month.remaining_budget_pct, thresholdPct]);

  const apiKeyCardLabels = useMemo(() => ({
    configured: t("settings.configured"),
    newApiKey: t("settings.newApiKey"),
    saveOrUpdate: t("settings.saveOrUpdate"),
    saving: t("common.saving"),
    deleteKey: t("settings.deleteKey"),
    deleting: t("settings.deleting"),
  }), [t]);

  const accessCards = useMemo(() => {
    if (!settings) return [];
    return [
    {
      id: "anthropic",
      title: t("settings.anthropicTitle"),
      description: t("settings.anthropicDescription"),
      configured: settings.has_anthropic_api_key,
      last4: settings.anthropic_api_key_last4,
      value: anthropicApiKeyInput,
      onChange: setAnthropicApiKeyInput,
      onSubmit: submitAnthropicApiKey,
      onDelete: handleDeleteAnthropicApiKey,
      placeholder: "sk-ant-...",
      saving: savingAnthropicKey,
      deleting: deletingAnthropicKey,
      notSet: t("settings.anthropicNotSet"),
    },
    {
      id: "openai",
      title: t("settings.openaiTitle"),
      description: t("settings.openaiDescription"),
      configured: settings.has_openai_api_key,
      last4: settings.openai_api_key_last4,
      value: openAIApiKeyInput,
      onChange: setOpenAIApiKeyInput,
      onSubmit: submitOpenAIApiKey,
      onDelete: handleDeleteOpenAIApiKey,
      placeholder: "sk-...",
      saving: savingOpenAIKey,
      deleting: deletingOpenAIKey,
      notSet: t("settings.openaiNotSet"),
    },
    {
      id: "google",
      title: t("settings.googleTitle"),
      description: t("settings.googleDescription"),
      configured: settings.has_google_api_key,
      last4: settings.google_api_key_last4,
      value: googleApiKeyInput,
      onChange: setGoogleApiKeyInput,
      onSubmit: submitGoogleApiKey,
      onDelete: handleDeleteGoogleApiKey,
      placeholder: "AIza...",
      saving: savingGoogleKey,
      deleting: deletingGoogleKey,
      notSet: t("settings.googleNotSet"),
    },
    {
      id: "groq",
      title: t("settings.groqTitle"),
      description: t("settings.groqDescription"),
      configured: settings.has_groq_api_key,
      last4: settings.groq_api_key_last4,
      value: groqApiKeyInput,
      onChange: setGroqApiKeyInput,
      onSubmit: submitGroqApiKey,
      onDelete: handleDeleteGroqApiKey,
      placeholder: "gsk_...",
      saving: savingGroqKey,
      deleting: deletingGroqKey,
      notSet: t("settings.groqNotSet"),
    },
    {
      id: "deepseek",
      title: t("settings.deepseekTitle"),
      description: t("settings.deepseekDescription"),
      configured: settings.has_deepseek_api_key,
      last4: settings.deepseek_api_key_last4,
      value: deepseekApiKeyInput,
      onChange: setDeepseekApiKeyInput,
      onSubmit: submitDeepSeekApiKey,
      onDelete: handleDeleteDeepSeekApiKey,
      placeholder: "sk-...",
      saving: savingDeepSeekKey,
      deleting: deletingDeepSeekKey,
      notSet: t("settings.deepseekNotSet"),
    },
    {
      id: "alibaba",
      title: t("settings.alibabaTitle"),
      description: t("settings.alibabaDescription"),
      configured: settings.has_alibaba_api_key,
      last4: settings.alibaba_api_key_last4,
      value: alibabaApiKeyInput,
      onChange: setAlibabaApiKeyInput,
      onSubmit: submitAlibabaApiKey,
      onDelete: handleDeleteAlibabaApiKey,
      placeholder: "sk-...",
      saving: savingAlibabaKey,
      deleting: deletingAlibabaKey,
      notSet: t("settings.alibabaNotSet"),
    },
    {
      id: "mistral",
      title: t("settings.mistralTitle"),
      description: t("settings.mistralDescription"),
      configured: settings.has_mistral_api_key,
      last4: settings.mistral_api_key_last4,
      value: mistralApiKeyInput,
      onChange: setMistralApiKeyInput,
      onSubmit: submitMistralApiKey,
      onDelete: handleDeleteMistralApiKey,
      placeholder: "sk-...",
      saving: savingMistralKey,
      deleting: deletingMistralKey,
      notSet: t("settings.mistralNotSet"),
    },
    {
      id: "xai",
      title: t("settings.xaiTitle"),
      description: t("settings.xaiDescription"),
      configured: settings.has_xai_api_key,
      last4: settings.xai_api_key_last4,
      value: xaiApiKeyInput,
      onChange: setXaiApiKeyInput,
      onSubmit: submitXAIApiKey,
      onDelete: handleDeleteXAIApiKey,
      placeholder: "xai-...",
      saving: savingXAIKey,
      deleting: deletingXAIKey,
      notSet: t("settings.xaiNotSet"),
    },
  ];
  }, [
    t,
    settings,
    anthropicApiKeyInput,
    openAIApiKeyInput,
    googleApiKeyInput,
    groqApiKeyInput,
    deepseekApiKeyInput,
    alibabaApiKeyInput,
    mistralApiKeyInput,
    xaiApiKeyInput,
    savingAnthropicKey,
    deletingAnthropicKey,
    savingOpenAIKey,
    deletingOpenAIKey,
    savingGoogleKey,
    deletingGoogleKey,
    savingGroqKey,
    deletingGroqKey,
    savingDeepSeekKey,
    deletingDeepSeekKey,
    savingAlibabaKey,
    deletingAlibabaKey,
    savingMistralKey,
    deletingMistralKey,
    savingXAIKey,
    deletingXAIKey,
  ]);
  const configuredProviderCount = useMemo(
    () => accessCards.filter((card) => card.configured).length,
    [accessCards]
  );
  const activeAccessCard = useMemo(
    () => accessCards.find((card) => card.id === activeAccessProvider) ?? accessCards[0],
    [accessCards, activeAccessProvider]
  );

  function dismissProviderModelUpdates() {
    const latest = providerModelUpdates.reduce<string | null>((max, event) => {
      if (!max) return event.detected_at;
      return Date.parse(event.detected_at) > Date.parse(max) ? event.detected_at : max;
    }, null);
    if (!latest || typeof window === "undefined") return;
    window.localStorage.setItem(MODEL_UPDATES_DISMISSED_AT_KEY, latest);
    setDismissedModelUpdatesAt(latest);
  }

  function restoreProviderModelUpdates() {
    if (typeof window === "undefined") return;
    window.localStorage.removeItem(MODEL_UPDATES_DISMISSED_AT_KEY);
    setDismissedModelUpdatesAt(null);
  }

  function toggleLLMExtras() {
    setLLMExtrasOpen((prev) => {
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
    setSavingBudget(true);
    try {
      const parsed = budgetUSD.trim() === "" ? null : Number(budgetUSD);
      if (parsed != null && (!Number.isFinite(parsed) || parsed < 0)) {
        throw new Error(t("settings.error.invalidBudget"));
      }
      await api.updateSettings({
        monthly_budget_usd: parsed,
        budget_alert_enabled: alertEnabled,
        budget_alert_threshold_pct: thresholdPct,
        digest_email_enabled: digestEmailEnabled,
      });
      await load();
      showToast(t("settings.toast.budgetSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingBudget(false);
    }
  }

  async function submitLLMModels(e: FormEvent) {
    e.preventDefault();
    setSavingLLMModels(true);
    try {
      const emptyToNull = (v: string) => {
        const s = v.trim();
        return s === "" ? null : s;
      };
      const nextModels = {
        facts: emptyToNull(anthropicFactsModel),
        summary: emptyToNull(anthropicSummaryModel),
        digest_cluster: emptyToNull(anthropicDigestClusterModel),
        digest: emptyToNull(anthropicDigestModel),
        ask: emptyToNull(anthropicAskModel),
        source_suggestion: emptyToNull(anthropicSourceSuggestionModel),
        embedding: emptyToNull(openAIEmbeddingModel),
        facts_check: emptyToNull(factsCheckModel),
        faithfulness_check: emptyToNull(faithfulnessCheckModel),
      };
      const resp = await api.updateLLMModelSettings(nextModels);
      setSettings((prev) => {
        if (!prev) return prev;
        return {
          ...prev,
          llm_models: {
            ...prev.llm_models,
            ...resp.llm_models,
          },
        };
      });
      syncLLMModelForm(resp.llm_models);
      llmModelsDirtyRef.current = false;
      showToast(t("settings.toast.modelsSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingLLMModels(false);
    }
  }

  async function submitDigestDelivery(e: FormEvent) {
    e.preventDefault();
    if (!settings) return;
    setSavingDigestDelivery(true);
    try {
      await api.updateSettings({
        monthly_budget_usd: settings.monthly_budget_usd,
        budget_alert_enabled: settings.budget_alert_enabled,
        budget_alert_threshold_pct: settings.budget_alert_threshold_pct,
        digest_email_enabled: digestEmailEnabled,
      });
      await load();
      showToast(t("settings.toast.digestSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingDigestDelivery(false);
    }
  }

  async function submitReadingPlan(e: FormEvent) {
    e.preventDefault();
    setSavingReadingPlan(true);
    try {
      const parsedSize = Number(readingPlanSize);
      if (!(parsedSize === 7 || parsedSize === 15 || parsedSize === 25)) {
        throw new Error(t("settings.error.invalidSize"));
      }
      await api.updateReadingPlanSettings({
        window: readingPlanWindow,
        size: parsedSize,
        diversify_topics: readingPlanDiversifyTopics,
      });
      await load();
      showToast(t("settings.toast.readingPlanSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingReadingPlan(false);
    }
  }

  function resetReadingGoalForm() {
    setEditingReadingGoalId(null);
    setReadingGoalTitle("");
    setReadingGoalDescription("");
    setReadingGoalPriority("3");
    setReadingGoalDueDate("");
  }

  async function submitReadingGoal(e: FormEvent) {
    e.preventDefault();
    setSavingReadingGoal(true);
    try {
      const activeLimitReached = activeReadingGoals.length >= 7 && !editingReadingGoalId;
      if (activeLimitReached) {
        throw new Error(t("settings.readingGoals.limit"));
      }
      const payload = {
        title: readingGoalTitle,
        description: readingGoalDescription,
        priority: Number(readingGoalPriority),
        due_date: readingGoalDueDate.trim() || null,
      };
      if (editingReadingGoalId) {
        await api.updateReadingGoal(editingReadingGoalId, payload);
      } else {
        await api.createReadingGoal(payload);
      }
      await load();
      resetReadingGoalForm();
      showToast(t("settings.toast.readingGoalSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingReadingGoal(false);
    }
  }

  async function archiveReadingGoal(goalId: string) {
    try {
      await api.archiveReadingGoal(goalId);
      await load();
      if (editingReadingGoalId === goalId) resetReadingGoalForm();
      showToast(t("settings.toast.readingGoalArchived"), "success");
    } catch (e) {
      showToast(String(e), "error");
    }
  }

  async function restoreReadingGoal(goalId: string) {
    try {
      await api.restoreReadingGoal(goalId);
      await load();
      showToast(t("settings.toast.readingGoalRestored"), "success");
    } catch (e) {
      showToast(String(e), "error");
    }
  }

  async function deleteReadingGoal(goalId: string) {
    try {
      await api.deleteReadingGoal(goalId);
      await load();
      if (editingReadingGoalId === goalId) resetReadingGoalForm();
      showToast(t("settings.toast.readingGoalDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    }
  }

  function startEditReadingGoal(goal: ReadingGoal) {
    setEditingReadingGoalId(goal.id);
    setReadingGoalTitle(goal.title);
    setReadingGoalDescription(goal.description ?? "");
    setReadingGoalPriority(String(goal.priority));
    setReadingGoalDueDate(goal.due_date ?? "");
  }

  async function submitObsidianExport(e: FormEvent) {
    e.preventDefault();
    setSavingObsidianExport(true);
    try {
      const resp = await api.updateObsidianExport({
        enabled: obsidianEnabled,
        github_repo_owner: obsidianRepoOwner.trim() || null,
        github_repo_name: obsidianRepoName.trim() || null,
        github_repo_branch: obsidianRepoBranch.trim() || null,
        vault_root_path: obsidianRootPath.trim() || null,
        keyword_link_mode: "topics_only",
      });
      setSettings((prev) => (prev ? { ...prev, obsidian_export: resp.obsidian_export } : prev));
      showToast(t("settings.toast.obsidianExportSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingObsidianExport(false);
    }
  }

  async function runObsidianExportNow() {
    setRunningObsidianExport(true);
    try {
      const res = await api.runObsidianExportNow();
      await load();
      showToast(
        `${t("settings.toast.obsidianExportRunNowResult")} updated=${res.updated} skipped=${res.skipped} failed=${res.failed}`,
        res.failed > 0 ? "error" : "success"
      );
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setRunningObsidianExport(false);
    }
  }

  async function submitAnthropicApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingAnthropicKey(true);
    try {
      if (!anthropicApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setAnthropicApiKey(anthropicApiKeyInput.trim());
      setAnthropicApiKeyInput("");
      await load();
      showToast(t("settings.toast.anthropicSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingAnthropicKey(false);
    }
  }

  async function handleDeleteAnthropicApiKey() {
    if (!(await confirm({
      title: t("settings.anthropicDeleteTitle"),
      message:
        t("settings.anthropicDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingAnthropicKey(true);
    try {
      await api.deleteAnthropicApiKey();
      await load();
      showToast(t("settings.toast.anthropicDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingAnthropicKey(false);
    }
  }

  async function submitOpenAIApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingOpenAIKey(true);
    try {
      if (!openAIApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setOpenAIApiKey(openAIApiKeyInput.trim());
      setOpenAIApiKeyInput("");
      await load();
      showToast(t("settings.toast.openaiSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingOpenAIKey(false);
    }
  }

  async function handleDeleteOpenAIApiKey() {
    if (!(await confirm({
      title: t("settings.openaiDeleteTitle"),
      message:
        t("settings.openaiDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingOpenAIKey(true);
    try {
      await api.deleteOpenAIApiKey();
      await load();
      showToast(t("settings.toast.openaiDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingOpenAIKey(false);
    }
  }

  async function submitGoogleApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingGoogleKey(true);
    try {
      if (!googleApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setGoogleApiKey(googleApiKeyInput.trim());
      setGoogleApiKeyInput("");
      await load();
      showToast(t("settings.toast.googleSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingGoogleKey(false);
    }
  }

  async function handleDeleteGoogleApiKey() {
    if (!(await confirm({
      title: t("settings.googleDeleteTitle"),
      message:
        t("settings.googleDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingGoogleKey(true);
    try {
      await api.deleteGoogleApiKey();
      await load();
      showToast(t("settings.toast.googleDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingGoogleKey(false);
    }
  }

  async function submitGroqApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingGroqKey(true);
    try {
      if (!groqApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setGroqApiKey(groqApiKeyInput.trim());
      setGroqApiKeyInput("");
      await load();
      showToast(t("settings.toast.groqSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingGroqKey(false);
    }
  }

  async function handleDeleteGroqApiKey() {
    if (!(await confirm({
      title: t("settings.groqDeleteTitle"),
      message: t("settings.groqDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingGroqKey(true);
    try {
      await api.deleteGroqApiKey();
      await load();
      showToast(t("settings.toast.groqDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingGroqKey(false);
    }
  }

  async function submitDeepSeekApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingDeepSeekKey(true);
    try {
      if (!deepseekApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setDeepSeekApiKey(deepseekApiKeyInput.trim());
      setDeepseekApiKeyInput("");
      await load();
      showToast(t("settings.toast.deepseekSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingDeepSeekKey(false);
    }
  }

  async function handleDeleteDeepSeekApiKey() {
    if (!(await confirm({
      title: t("settings.deepseekDeleteTitle"),
      message: t("settings.deepseekDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingDeepSeekKey(true);
    try {
      await api.deleteDeepSeekApiKey();
      await load();
      showToast(t("settings.toast.deepseekDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingDeepSeekKey(false);
    }
  }

  async function submitAlibabaApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingAlibabaKey(true);
    try {
      if (!alibabaApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setAlibabaApiKey(alibabaApiKeyInput.trim());
      setAlibabaApiKeyInput("");
      await load();
      showToast(t("settings.toast.alibabaSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingAlibabaKey(false);
    }
  }

  async function handleDeleteAlibabaApiKey() {
    if (!(await confirm({
      title: t("settings.alibabaDeleteTitle"),
      message: t("settings.alibabaDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingAlibabaKey(true);
    try {
      await api.deleteAlibabaApiKey();
      await load();
      showToast(t("settings.toast.alibabaDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingAlibabaKey(false);
    }
  }

  async function submitMistralApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingMistralKey(true);
    try {
      if (!mistralApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setMistralApiKey(mistralApiKeyInput.trim());
      setMistralApiKeyInput("");
      await load();
      showToast(t("settings.toast.mistralSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingMistralKey(false);
    }
  }

  async function handleDeleteMistralApiKey() {
    if (!(await confirm({
      title: t("settings.mistralDeleteTitle"),
      message: t("settings.mistralDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingMistralKey(true);
    try {
      await api.deleteMistralApiKey();
      await load();
      showToast(t("settings.toast.mistralDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingMistralKey(false);
    }
  }

  async function submitXAIApiKey(e: FormEvent) {
    e.preventDefault();
    setSavingXAIKey(true);
    try {
      if (!xaiApiKeyInput.trim()) {
        throw new Error(t("settings.error.enterApiKey"));
      }
      await api.setXAIApiKey(xaiApiKeyInput.trim());
      setXaiApiKeyInput("");
      await load();
      showToast(t("settings.toast.xaiSaved"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingXAIKey(false);
    }
  }

  async function handleDeleteXAIApiKey() {
    if (!(await confirm({
      title: t("settings.xaiDeleteTitle"),
      message: t("settings.xaiDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingXAIKey(true);
    try {
      await api.deleteXAIApiKey();
      await load();
      showToast(t("settings.toast.xaiDeleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingXAIKey(false);
    }
  }

  async function handleDeleteInoreaderOAuth() {
    if (!(await confirm({
      title: t("settings.inoreaderDeleteTitle"),
      message: t("settings.inoreaderDeleteMessage"),
      confirmLabel: t("settings.delete"),
      tone: "danger",
    }))) {
      return;
    }
    setDeletingInoreaderOAuth(true);
    try {
      await api.deleteInoreaderOAuth();
      await load();
      showToast(t("settings.toast.inoreaderDisconnected"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingInoreaderOAuth(false);
    }
  }

  if (loading) return <p className="text-sm text-zinc-500">{t("common.loading")}</p>;
  if (error) return <p className="text-sm text-red-500">{error}</p>;
  if (!settings) return null;

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <div>
        <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
          <SettingsIcon className="size-6 text-zinc-500" aria-hidden="true" />
          <span>{t("settings.title")}</span>
        </h1>
        <p className="mt-1 text-sm text-zinc-500">{t("settings.subtitle")}</p>
      </div>

      <section className="grid gap-3 md:grid-cols-3">
        <SettingsMetricCard
          label={t("settings.metric.mtdCost")}
          value={`$${settings.current_month.estimated_cost_usd.toFixed(6)}`}
        />
        <SettingsMetricCard
          label={t("settings.metric.monthlyBudget")}
          value={settings.monthly_budget_usd == null ? "—" : `$${settings.monthly_budget_usd.toFixed(2)}`}
        />
        <SettingsMetricCard
          label={t("settings.metric.budgetRemaining")}
          value={
            settings.current_month.remaining_budget_pct == null
              ? "—"
              : `${settings.current_month.remaining_budget_pct.toFixed(1)}%`
          }
          valueClassName={budgetRemainingTone}
        />
      </section>

      <section className="space-y-6">
        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          <button
            type="button"
            onClick={() => setLLMSectionOpen((prev) => !prev)}
            className="flex w-full items-start justify-between gap-4 text-left"
          >
            <div>
              <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                <Brain className="size-4 text-zinc-500" aria-hidden="true" />
                {t("settings.section.llm")}
              </h2>
              <p className="mt-1 text-sm text-zinc-500">{t("settings.section.llmDescription")}</p>
            </div>
            <ChevronDown className={`mt-0.5 size-4 shrink-0 text-zinc-500 transition-transform ${llmSectionOpen ? "rotate-180" : ""}`} />
          </button>

          {llmSectionOpen ? (
            <>
              <div className="mt-4 flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={applyCostPerformancePreset}
                  className="inline-flex items-center rounded-full bg-zinc-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-zinc-800"
                >
                  {t("settings.modelPreset.costPerformance")}
                </button>
                <button
                  type="button"
                  onClick={toggleLLMExtras}
                  className="inline-flex items-center gap-1 rounded-full border border-zinc-300 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 hover:border-zinc-400 hover:text-zinc-900"
                >
                  {t("settings.section.llmExtras")}
                  <ChevronDown className={`size-3 transition-transform ${llmExtrasOpen ? "rotate-180" : ""}`} />
                </button>
              </div>

              <div className="mt-5 rounded-xl border border-zinc-200 bg-zinc-50 p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <div className="text-sm font-medium text-zinc-900">{t("settings.access.selectProvider")}</div>
                <div className="text-xs text-zinc-500">
                  {`${t("settings.access.configuredProviders")}: ${configuredProviderCount}/${accessCards.length}`}
                </div>
              </div>
            </div>
            <div className="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
              {accessCards.map((card) => {
                const selected = card.id === activeAccessCard?.id;
                return (
                  <button
                    key={card.id}
                    type="button"
                    onClick={() => setActiveAccessProvider(card.id)}
                    className={`rounded-xl border px-4 py-3 text-left transition ${
                      selected
                        ? "border-zinc-900 bg-white shadow-sm"
                        : "border-zinc-200 bg-white/70 hover:border-zinc-300 hover:bg-white"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="text-sm font-medium text-zinc-900">{card.title.replace(/（.*?）|\(.*?\)/g, "").trim()}</div>
                      <span
                        className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${
                          card.configured
                            ? "bg-emerald-100 text-emerald-700"
                            : "bg-zinc-200 text-zinc-600"
                        }`}
                      >
                        {card.configured ? t("settings.configured") : t("settings.access.notConfiguredShort")}
                      </span>
                    </div>
                    <div className="mt-2 text-xs text-zinc-500">
                      {card.configured ? `••••${card.last4 ?? "****"}` : card.notSet}
                    </div>
                  </button>
                );
              })}
            </div>
              </div>

              {activeAccessCard ? (
                <div className="mt-4">
                  <ApiKeyCard
                    icon={KeyRound}
                    title={activeAccessCard.title}
                    description={activeAccessCard.description}
                    configured={activeAccessCard.configured}
                    last4={activeAccessCard.last4}
                    value={activeAccessCard.value}
                    onChange={activeAccessCard.onChange}
                    onSubmit={activeAccessCard.onSubmit}
                    onDelete={activeAccessCard.onDelete}
                    placeholder={activeAccessCard.placeholder}
                    saving={activeAccessCard.saving}
                    deleting={activeAccessCard.deleting}
                    labels={{ ...apiKeyCardLabels, notSet: activeAccessCard.notSet }}
                  />
                </div>
              ) : null}

              <form onSubmit={submitLLMModels} className="mt-4 rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
            <div className="mb-4">
              <h3 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                <Brain className="size-4 text-zinc-500" aria-hidden="true" />
                {t("settings.modelsTitle")}
              </h3>
              <p className="mt-1 text-sm text-zinc-500">{t("settings.modelsDescription")}</p>
              <p className="mt-1 text-xs text-zinc-400">{t("settings.pricingDescription")}</p>
            </div>

            <div className="space-y-4">
              <section className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
                <h4 className="text-sm font-semibold text-zinc-900">{t("settings.group.summary")}</h4>
                <div className="mt-3 grid gap-4 md:grid-cols-2">
                  <ModelSelect
                    label={t("settings.model.facts")}
                    value={anthropicFactsModel}
                    onChange={(value) => onChangeLLMModel(setAnthropicFactsModel, value)}
                    options={optionsForPurpose("facts")}
                    labels={modelSelectLabels}
                  />
                  <ModelSelect
                    label={t("settings.model.summary")}
                    value={anthropicSummaryModel}
                    onChange={(value) => onChangeLLMModel(setAnthropicSummaryModel, value)}
                    options={optionsForPurpose("summary")}
                    labels={modelSelectLabels}
                  />
                </div>
              </section>

              <section className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
                <h4 className="text-sm font-semibold text-zinc-900">{t("settings.group.digest")}</h4>
                <div className="mt-3 grid gap-4 md:grid-cols-2">
                  <ModelSelect
                    label={t("settings.model.digestCluster")}
                    value={anthropicDigestClusterModel}
                    onChange={(value) => onChangeLLMModel(setAnthropicDigestClusterModel, value)}
                    options={optionsForPurpose("digest_cluster_draft")}
                    labels={modelSelectLabels}
                  />
                  <ModelSelect
                    label={t("settings.model.digest")}
                    value={anthropicDigestModel}
                    onChange={(value) => onChangeLLMModel(setAnthropicDigestModel, value)}
                    options={optionsForPurpose("digest")}
                    labels={modelSelectLabels}
                  />
                </div>
              </section>

              <section className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
                <h4 className="text-sm font-semibold text-zinc-900">{t("settings.group.validation")}</h4>
                <div className="mt-3 grid gap-4 md:grid-cols-2">
                  <ModelSelect
                    label={t("settings.model.factsCheck")}
                    value={factsCheckModel}
                    onChange={(value) => onChangeLLMModel(setFactsCheckModel, value)}
                    options={optionsForPurpose("facts")}
                    labels={modelSelectLabels}
                  />
                  <ModelSelect
                    label={t("settings.model.faithfulnessCheck")}
                    value={faithfulnessCheckModel}
                    onChange={(value) => onChangeLLMModel(setFaithfulnessCheckModel, value)}
                    options={optionsForPurpose("summary")}
                    labels={modelSelectLabels}
                  />
                </div>
              </section>

              <section className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
                <h4 className="text-sm font-semibold text-zinc-900">{t("settings.group.other")}</h4>
                <div className="mt-3 grid gap-4 md:grid-cols-2">
                  <ModelSelect
                    label={t("settings.model.sourceSuggestion")}
                    value={anthropicSourceSuggestionModel}
                    onChange={(value) => onChangeLLMModel(setAnthropicSourceSuggestionModel, value)}
                    options={sourceSuggestionModelOptions}
                    labels={modelSelectLabels}
                  />
                  <ModelSelect
                    label={t("settings.model.ask")}
                    value={anthropicAskModel}
                    onChange={(value) => onChangeLLMModel(setAnthropicAskModel, value)}
                    options={optionsForPurpose("ask")}
                    labels={modelSelectLabels}
                  />
                  <ModelSelect
                    label={t("settings.model.embeddings")}
                    value={openAIEmbeddingModel}
                    onChange={(value) => onChangeLLMModel(setOpenAIEmbeddingModel, value)}
                    options={openAIEmbeddingModelOptions}
                    labels={modelSelectLabels}
                  />
                </div>
              </section>
            </div>

            <div className="mt-4">
              <button
                type="submit"
                disabled={savingLLMModels}
                className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
              >
                {savingLLMModels ? t("common.saving") : t("settings.saveModels")}
              </button>
            </div>
              </form>

              {llmExtrasOpen ? (
                <div ref={llmExtrasRef} className="mt-4">
                  <div className="space-y-4">
                    <ProviderModelUpdatesPanel
                      allEvents={providerModelUpdates}
                      visibleEvents={visibleProviderModelUpdates}
                      onDismiss={dismissProviderModelUpdates}
                      onRestore={restoreProviderModelUpdates}
                      labels={{
                        title: t("settings.providerModelUpdates"),
                        description: t("settings.providerModelUpdatesDescription"),
                        dismiss: t("settings.providerModelUpdate.dismiss"),
                        empty: t("settings.providerModelUpdate.empty"),
                        dismissed: t("settings.providerModelUpdate.dismissed"),
                        restore: t("settings.providerModelUpdate.restore"),
                        added: t("settings.providerModelUpdate.added", "added"),
                        removed: t("settings.providerModelUpdate.removed", "removed"),
                      }}
                    />
                    <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
                      <div className="mb-3">
                        <h3 className="text-sm font-semibold text-zinc-900">{t("settings.modelGuide.title")}</h3>
                        <p className="mt-1 text-xs text-zinc-500">{t("settings.modelGuide.description")}</p>
                      </div>
                      <div className="overflow-auto">
                        <ModelGuideTable entries={modelComparisonEntries} t={t} />
                      </div>
                    </section>
                  </div>
                </div>
              ) : null}
            </>
          ) : null}
        </section>

        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          <button
            type="button"
            onClick={() => setGoalsSectionOpen((prev) => !prev)}
            className="flex w-full items-start justify-between gap-4 text-left"
          >
            <div>
              <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                <Brain className="size-4 text-zinc-500" aria-hidden="true" />
                {t("settings.section.goals")}
              </h2>
              <p className="mt-1 text-sm text-zinc-500">{t("settings.section.goalsDescription")}</p>
            </div>
            <ChevronDown className={`mt-0.5 size-4 shrink-0 text-zinc-500 transition-transform ${goalsSectionOpen ? "rotate-180" : ""}`} />
          </button>

          {goalsSectionOpen ? (
            <div className="mt-4">
              <form onSubmit={submitReadingGoal} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
                <div className="mb-4">
                  <h3 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                    <Brain className="size-4 text-zinc-500" aria-hidden="true" />
                    {t("settings.readingGoals.title")}
                  </h3>
                  <p className="mt-1 text-sm text-zinc-500">{t("settings.readingGoals.description")}</p>
                </div>

                <div className="grid gap-6 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
                  <div className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium text-zinc-700">{t("settings.readingGoals.goalTitle")}</label>
                      <input
                        value={readingGoalTitle}
                        onChange={(e) => setReadingGoalTitle(e.target.value)}
                        className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                        placeholder={t("settings.readingGoals.goalTitlePlaceholder")}
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-zinc-700">{t("settings.readingGoals.goalDescription")}</label>
                      <textarea
                        value={readingGoalDescription}
                        onChange={(e) => setReadingGoalDescription(e.target.value)}
                        rows={3}
                        className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                        placeholder={t("settings.readingGoals.goalDescriptionPlaceholder")}
                      />
                    </div>
                    <div className="grid gap-3 sm:grid-cols-2">
                      <div>
                        <label className="block text-sm font-medium text-zinc-700">{t("settings.readingGoals.priority")}</label>
                        <select
                          value={readingGoalPriority}
                          onChange={(e) => setReadingGoalPriority(e.target.value)}
                          className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                        >
                          {[5, 4, 3, 2, 1].map((value) => (
                            <option key={value} value={String(value)}>
                              {value}
                            </option>
                          ))}
                        </select>
                      </div>
                      <div>
                        <label className="block text-sm font-medium text-zinc-700">{t("settings.readingGoals.dueDate")}</label>
                        <input
                          type="date"
                          value={readingGoalDueDate}
                          onChange={(e) => setReadingGoalDueDate(e.target.value)}
                          className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                        />
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-2">
                      <button
                        type="submit"
                        disabled={savingReadingGoal}
                        className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
                      >
                        {savingReadingGoal ? t("common.saving") : editingReadingGoalId ? t("settings.readingGoals.update") : t("settings.readingGoals.save")}
                      </button>
                      {editingReadingGoalId ? (
                        <button
                          type="button"
                          onClick={resetReadingGoalForm}
                          className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm text-zinc-700"
                        >
                          {t("common.cancel")}
                        </button>
                      ) : null}
                      <span className="text-xs text-zinc-500">
                        {locale === "ja" ? `active ${activeReadingGoals.length}/7` : `${activeReadingGoals.length}/7 active`}
                      </span>
                    </div>
                  </div>

                  <div className="space-y-3">
                    {activeReadingGoals.map((goal) => (
                      <div key={goal.id} className="rounded-xl border border-zinc-200 bg-zinc-50 px-3 py-3">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <div className="line-clamp-1 text-sm font-semibold text-zinc-900">{goal.title}</div>
                              <span className="rounded-full bg-white px-2 py-1 text-[11px] text-zinc-600">P{goal.priority}</span>
                            </div>
                            {goal.description ? (
                              <p className="mt-1 line-clamp-2 text-xs text-zinc-500">{goal.description}</p>
                            ) : null}
                            {goal.due_date ? (
                              <p className="mt-1 text-xs text-zinc-500">{t("settings.readingGoals.dueDate")}: {goal.due_date}</p>
                            ) : null}
                          </div>
                          <div className="flex shrink-0 flex-wrap gap-2">
                            <button type="button" onClick={() => startEditReadingGoal(goal)} className="text-xs text-zinc-600 hover:text-zinc-900">
                              {t("settings.readingGoals.edit")}
                            </button>
                            <button type="button" onClick={() => void archiveReadingGoal(goal.id)} className="text-xs text-zinc-600 hover:text-zinc-900">
                              {t("settings.readingGoals.archive")}
                            </button>
                            <button type="button" onClick={() => void deleteReadingGoal(goal.id)} className="text-xs text-rose-600 hover:text-rose-700">
                              {t("settings.delete")}
                            </button>
                          </div>
                        </div>
                      </div>
                    ))}
                    {archivedReadingGoals.length > 0 ? (
                      <div className="space-y-2 border-t border-zinc-200 pt-3">
                        <div className="text-xs font-medium uppercase tracking-[0.12em] text-zinc-500">{t("settings.readingGoals.archived")}</div>
                        {archivedReadingGoals.map((goal) => (
                          <div key={goal.id} className="flex items-center justify-between gap-3 rounded-lg border border-zinc-200 px-3 py-2">
                            <div className="min-w-0 text-sm text-zinc-700">{goal.title}</div>
                            <button type="button" onClick={() => void restoreReadingGoal(goal.id)} className="text-xs text-zinc-600 hover:text-zinc-900">
                              {t("settings.readingGoals.restore")}
                            </button>
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </div>
                </div>
              </form>
            </div>
          ) : null}
        </section>

        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          <button
            type="button"
            onClick={() => setOperationsSectionOpen((prev) => !prev)}
            className="flex w-full items-start justify-between gap-4 text-left"
          >
            <div>
              <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                <BellRing className="size-4 text-zinc-500" aria-hidden="true" />
                {t("settings.section.operations")}
              </h2>
              <p className="mt-1 text-sm text-zinc-500">{t("settings.section.operationsDescription")}</p>
            </div>
            <ChevronDown className={`mt-0.5 size-4 shrink-0 text-zinc-500 transition-transform ${operationsSectionOpen ? "rotate-180" : ""}`} />
          </button>

          {operationsSectionOpen ? (
            <div className="mt-4 grid gap-6 lg:grid-cols-3">
            <form onSubmit={submitReadingPlan} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
              <div className="mb-4">
                <h3 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                  <Brain className="size-4 text-zinc-500" aria-hidden="true" />
                  {t("settings.recommendedTitle")}
                </h3>
                <p className="mt-1 text-sm text-zinc-500">
                  {t("settings.recommendedDescription")}
                </p>
              </div>

              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-zinc-700">
                    {t("settings.window")}
                  </label>
                  <select
                    value={readingPlanWindow}
                    onChange={(e) => setReadingPlanWindow(e.target.value as "24h" | "today_jst" | "7d")}
                    className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                  >
                    <option value="24h">{t("settings.window.24h")}</option>
                    <option value="today_jst">{t("settings.window.today")}</option>
                    <option value="7d">{t("settings.window.7d")}</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-zinc-700">
                    {t("settings.size")}
                  </label>
                  <select
                    value={readingPlanSize}
                    onChange={(e) => setReadingPlanSize(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                  >
                    {[7, 15, 25].map((n) => (
                      <option key={n} value={String(n)}>
                        {n}
                      </option>
                    ))}
                  </select>
                </div>
                <label className="flex items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
                  <span>{t("settings.diversifyTopics")}</span>
                  <input
                    type="checkbox"
                    checked={readingPlanDiversifyTopics}
                    onChange={(e) => setReadingPlanDiversifyTopics(e.target.checked)}
                    className="size-4 rounded border-zinc-300"
                  />
                </label>
              </div>

              <div className="mt-4">
                <button
                  type="submit"
                  disabled={savingReadingPlan}
                  className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
                >
                  {savingReadingPlan ? t("common.saving") : t("settings.saveRecommended")}
                </button>
              </div>
            </form>

            <form onSubmit={submitDigestDelivery} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
              <div className="mb-4">
                <h3 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                  <Mail className="size-4 text-zinc-500" aria-hidden="true" />
                  {t("settings.digestTitle")}
                </h3>
                <p className="mt-1 text-sm text-zinc-500">
                  {t("settings.digestDescription")}
                </p>
              </div>

              <div className="flex items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
                <div className="min-w-0">
                  <div className="text-sm font-medium text-zinc-800">
                    {t("settings.digestEmailSending")}
                  </div>
                  <div className="text-xs text-zinc-500">
                    {t("settings.digestDisabledHint")}
                  </div>
                </div>
                <label className="inline-flex shrink-0 cursor-pointer items-center gap-2 whitespace-nowrap text-sm text-zinc-700">
                  <input
                    type="checkbox"
                    checked={digestEmailEnabled}
                    onChange={(e) => setDigestEmailEnabled(e.target.checked)}
                    className="size-4 rounded border-zinc-300"
                  />
                  {digestEmailEnabled ? t("settings.on") : t("settings.off")}
                </label>
              </div>

              <div className="mt-4">
                <button
                  type="submit"
                  disabled={savingDigestDelivery}
                  className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
                >
                  {savingDigestDelivery ? t("common.saving") : t("settings.saveDelivery")}
                </button>
              </div>
            </form>

            <form onSubmit={submitBudget} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
              <div className="mb-4">
                <h3 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                  <Coins className="size-4 text-zinc-500" aria-hidden="true" />
                  {t("settings.budgetTitle")}
                </h3>
                <p className="mt-1 text-sm text-zinc-500">
                  {t("settings.budgetDescription")}
                </p>
              </div>

              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-zinc-700">
                    {t("settings.monthlyBudgetUsd")}
                  </label>
                  <input
                    type="number"
                    min={0}
                    step="0.01"
                    value={budgetUSD}
                    onChange={(e) => setBudgetUSD(e.target.value)}
                    placeholder={t("settings.budgetPlaceholder")}
                    className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none placeholder:text-zinc-400 focus:border-zinc-400"
                  />
                </div>

                <div className="flex items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-zinc-800">
                      {t("settings.budgetAlertEmail")}
                    </div>
                    <div className="text-xs text-zinc-500">
                      {t("settings.budgetAlertHint")}
                    </div>
                  </div>
                  <label className="inline-flex shrink-0 cursor-pointer items-center gap-2 whitespace-nowrap text-sm text-zinc-700">
                    <input
                      type="checkbox"
                      checked={alertEnabled}
                      onChange={(e) => setAlertEnabled(e.target.checked)}
                      className="size-4 rounded border-zinc-300"
                    />
                    {alertEnabled ? t("settings.on") : t("settings.off")}
                  </label>
                </div>

                <div>
                  <label className="block text-sm font-medium text-zinc-700">
                    {t("settings.alertThreshold")}
                  </label>
                  <div className="mt-1 flex items-center gap-3">
                    <input
                      type="range"
                      min={1}
                      max={99}
                      value={thresholdPct}
                      onChange={(e) => setThresholdPct(Number(e.target.value))}
                      className="w-full accent-zinc-900"
                    />
                    <span className="w-12 text-right text-sm font-medium text-zinc-800">{thresholdPct}%</span>
                  </div>
                </div>
              </div>

              <div className="mt-4 flex items-center gap-2">
                <button
                  type="submit"
                  disabled={savingBudget}
                  className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
                >
                  {savingBudget ? t("common.saving") : t("settings.saveBudget")}
                </button>
                <span className="text-xs text-zinc-500">
                  {`${t("settings.currentMonth")}: ${settings.current_month.month_jst}`}
                </span>
              </div>
            </form>
            </div>
          ) : null}
        </section>

        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          <button
            type="button"
            onClick={() => setNotificationsSectionOpen((prev) => !prev)}
            className="flex w-full items-start justify-between gap-4 text-left"
          >
            <div>
              <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                <BellRing className="size-4 text-zinc-500" aria-hidden="true" />
                {t("settings.section.notifications")}
              </h2>
              <p className="mt-1 text-sm text-zinc-500">{t("settings.section.notificationsDescription")}</p>
            </div>
            <ChevronDown className={`mt-0.5 size-4 shrink-0 text-zinc-500 transition-transform ${notificationsSectionOpen ? "rotate-180" : ""}`} />
          </button>

          {notificationsSectionOpen ? (
            <div className="mt-4">
              <OneSignalSettings
                rule={notificationPriority}
                onSaveRule={async (rule) => {
                  const res = await api.updateNotificationPriority(rule);
                  setNotificationPriority(res.notification_priority);
                  setSettings((prev) => (prev ? { ...prev, notification_priority: res.notification_priority } : prev));
                }}
              />
            </div>
          ) : null}
        </section>

        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          <button
            type="button"
            onClick={() => setIntegrationsSectionOpen((prev) => !prev)}
            className="flex w-full items-start justify-between gap-4 text-left"
          >
            <div>
              <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
                <Link2 className="size-4 text-zinc-500" aria-hidden="true" />
                {t("settings.section.integrations")}
              </h2>
              <p className="mt-1 text-sm text-zinc-500">{t("settings.section.integrationsDescription")}</p>
            </div>
            <ChevronDown className={`mt-0.5 size-4 shrink-0 text-zinc-500 transition-transform ${integrationsSectionOpen ? "rotate-180" : ""}`} />
          </button>

          {integrationsSectionOpen ? (
            <div className="mt-4 grid gap-4 lg:grid-cols-2">
            <section className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <KeyRound className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.inoreaderTitle")}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">{t("settings.inoreaderDescription")}</p>
          </div>

          <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
            {settings.has_inoreader_oauth ? t("settings.inoreaderConnected") : t("settings.inoreaderNotConnected")}
          </div>
          {settings.inoreader_token_expires_at && (
            <p className="mt-2 text-xs text-zinc-500">
              {t("settings.inoreaderTokenExpiresAt")}: {new Date(settings.inoreader_token_expires_at).toLocaleString()}
            </p>
          )}
          <div className="mt-4 flex flex-wrap gap-2">
            <a
              href="/api/settings/inoreader/connect"
              className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white"
            >
              {t("settings.inoreaderConnect")}
            </a>
            <button
              type="button"
              disabled={deletingInoreaderOAuth || !settings.has_inoreader_oauth}
              onClick={handleDeleteInoreaderOAuth}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
            >
              {deletingInoreaderOAuth ? t("settings.deleting") : t("settings.inoreaderDisconnect")}
            </button>
          </div>
            </section>

            <form onSubmit={submitObsidianExport} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm lg:col-span-2">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <SettingsIcon className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.obsidianTitle")}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">{t("settings.obsidianDescription")}</p>
          </div>

          <div className="space-y-4">
            <div className="flex items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
              <div className="min-w-0">
                <div className="text-sm font-medium text-zinc-800">{t("settings.obsidianEnabled")}</div>
                <div className="text-xs text-zinc-500">{t("settings.obsidianEnabledHint")}</div>
              </div>
              <label className="inline-flex shrink-0 cursor-pointer items-center gap-2 whitespace-nowrap text-sm text-zinc-700">
                <input
                  type="checkbox"
                  checked={obsidianEnabled}
                  onChange={(e) => setObsidianEnabled(e.target.checked)}
                  className="size-4 rounded border-zinc-300"
                />
                {obsidianEnabled ? t("settings.on") : t("settings.off")}
              </label>
            </div>

            <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
              {settings.obsidian_export?.github_installation_id
                ? t("settings.obsidianGithubConnected")
                : t("settings.obsidianGithubNotConnected")}
            </div>
            <div className="flex flex-wrap gap-2">
              <a
                href="/api/settings/obsidian-github/connect"
                className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white"
              >
                {t("settings.obsidianGithubConnect")}
              </a>
            </div>

            <div>
              <label className="block text-sm font-medium text-zinc-700">{t("settings.obsidianRepoOwner")}</label>
              <input
                type="text"
                value={obsidianRepoOwner}
                onChange={(e) => setObsidianRepoOwner(e.target.value)}
                className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                placeholder="your-org"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-zinc-700">{t("settings.obsidianRepoName")}</label>
              <input
                type="text"
                value={obsidianRepoName}
                onChange={(e) => setObsidianRepoName(e.target.value)}
                className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                placeholder="obsidian-vault"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-zinc-700">{t("settings.obsidianBranch")}</label>
              <input
                type="text"
                value={obsidianRepoBranch}
                onChange={(e) => setObsidianRepoBranch(e.target.value)}
                className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                placeholder="main"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-zinc-700">{t("settings.obsidianRootPath")}</label>
              <input
                type="text"
                value={obsidianRootPath}
                onChange={(e) => setObsidianRootPath(e.target.value)}
                className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
                placeholder="Sifto/Favorites"
              />
              <p className="mt-1 text-xs text-zinc-500">{t("settings.obsidianRootPathHint")}</p>
            </div>

            {settings.obsidian_export?.last_success_at && (
              <p className="text-xs text-zinc-500">
                {t("settings.obsidianLastSuccess")}: {new Date(settings.obsidian_export.last_success_at).toLocaleString()}
              </p>
            )}
          </div>

          <div className="mt-4 flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={savingObsidianExport}
              className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {savingObsidianExport ? t("common.saving") : t("settings.obsidianSave")}
            </button>
            <button
              type="button"
              onClick={() => {
                void runObsidianExportNow();
              }}
              disabled={runningObsidianExport}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-60"
            >
              {runningObsidianExport ? t("settings.obsidianRunNowRunning") : t("settings.obsidianRunNow")}
            </button>
          </div>
            </form>

            </div>
          ) : null}
        </section>
      </section>
    </div>
  );
}
