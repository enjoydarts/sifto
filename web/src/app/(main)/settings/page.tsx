"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Brain, Coins, KeyRound, Mail, Settings as SettingsIcon } from "lucide-react";
import { api, UserSettings } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";
import OneSignalSettings from "@/components/onesignal-settings";

type ModelOption = {
  value: string;
  label: string;
  note?: string;
};

type ModelComparisonEntry = {
  model: string;
  provider: "anthropic" | "google" | "groq" | "openai" | "deepseek";
  inputPrice: string;
  outputPrice: string;
  recommendation: "recommended" | "strong" | "experimental";
  bestFor: "facts" | "summary" | "ask" | "digest" | "embedding" | "balanced";
  highlights: Array<"lowestCost" | "fast" | "jsonStable">;
  status?: "preview";
  comment?: string;
};

export default function SettingsPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const [loading, setLoading] = useState(true);
  const [savingBudget, setSavingBudget] = useState(false);
  const [savingDigestDelivery, setSavingDigestDelivery] = useState(false);
  const [savingReadingPlan, setSavingReadingPlan] = useState(false);
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
  const [deletingInoreaderOAuth, setDeletingInoreaderOAuth] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [budgetUSD, setBudgetUSD] = useState<string>("");
  const [alertEnabled, setAlertEnabled] = useState(false);
  const [thresholdPct, setThresholdPct] = useState<number>(20);
  const [digestEmailEnabled, setDigestEmailEnabled] = useState(true);
  const [anthropicApiKeyInput, setAnthropicApiKeyInput] = useState("");
  const [openAIApiKeyInput, setOpenAIApiKeyInput] = useState("");
  const [googleApiKeyInput, setGoogleApiKeyInput] = useState("");
  const [groqApiKeyInput, setGroqApiKeyInput] = useState("");
  const [deepseekApiKeyInput, setDeepseekApiKeyInput] = useState("");
  const [isModelGuideOpen, setIsModelGuideOpen] = useState(false);
  const [readingPlanWindow, setReadingPlanWindow] = useState<"24h" | "today_jst" | "7d">("24h");
  const [readingPlanSize, setReadingPlanSize] = useState<string>("15");
  const [readingPlanDiversifyTopics, setReadingPlanDiversifyTopics] = useState(true);
  const [anthropicFactsModel, setAnthropicFactsModel] = useState("");
  const [anthropicSummaryModel, setAnthropicSummaryModel] = useState("");
  const [anthropicDigestClusterModel, setAnthropicDigestClusterModel] = useState("");
  const [anthropicDigestModel, setAnthropicDigestModel] = useState("");
  const [anthropicAskModel, setAnthropicAskModel] = useState("");
  const [anthropicSourceSuggestionModel, setAnthropicSourceSuggestionModel] = useState("");
  const [openAIEmbeddingModel, setOpenAIEmbeddingModel] = useState("");

  const llmModelOptions: ModelOption[] = [
    { value: "claude-haiku-4-5", label: "claude-haiku-4-5", note: "in $1 / out $5 / 1M tok" },
    { value: "claude-sonnet-4-6", label: "claude-sonnet-4-6", note: "in $3 / out $15 / 1M tok" },
    { value: "claude-opus-4-6", label: "claude-opus-4-6", note: "in $5 / out $25 / 1M tok" },
    { value: "gemini-3.1-pro-preview", label: "gemini-3.1-pro-preview", note: "Google AI Studio / in $2.00 ($4.00 >200k) / out $12.00 ($18.00 >200k) / 1M tok" },
    { value: "gemini-3.1-flash-lite-preview", label: "gemini-3.1-flash-lite-preview", note: "Google AI Studio / in $0.25 / out $1.50 / 1M tok" },
    { value: "gemini-3-flash-preview", label: "gemini-3-flash-preview", note: "Google AI Studio / in $0.50 / out $3.00 / 1M tok" },
    { value: "gemini-2.5-flash", label: "gemini-2.5-flash", note: "Google AI Studio / in $0.30 / out $2.50 / 1M tok" },
    { value: "gemini-2.5-flash-lite", label: "gemini-2.5-flash-lite", note: "Google AI Studio / in $0.10 / out $0.40 / 1M tok" },
    { value: "gemini-2.5-pro", label: "gemini-2.5-pro", note: "Google AI Studio / in $1.25 ($2.50 >200k) / out $10.00 ($15.00 >200k) / 1M tok" },
    { value: "openai/gpt-oss-20b", label: "openai/gpt-oss-20b", note: "Groq / in $0.075 / out $0.30 / cached in $0.0375 / 1M tok" },
    { value: "openai/gpt-oss-120b", label: "openai/gpt-oss-120b", note: "Groq / in $0.15 / out $0.60 / cached in $0.075 / 1M tok" },
    { value: "llama-3.1-8b-instant", label: "llama-3.1-8b-instant", note: "Groq / in $0.05 / out $0.08 / 1M tok" },
    { value: "llama-3.3-70b-versatile", label: "llama-3.3-70b-versatile", note: "Groq / in $0.59 / out $0.79 / 1M tok" },
    { value: "meta-llama/llama-4-scout-17b-16e-instruct", label: "meta-llama/llama-4-scout-17b-16e-instruct", note: "Groq Preview / in $0.11 / out $0.34 / 1M tok" },
    { value: "qwen/qwen3-32b", label: "qwen/qwen3-32b", note: "Groq / in $0.29 / out $0.59 / 1M tok" },
    { value: "deepseek-chat", label: "deepseek-chat", note: "DeepSeek / cached in $0.028 / in $0.28 / out $0.42 / 1M tok" },
    { value: "deepseek-reasoner", label: "deepseek-reasoner", note: "DeepSeek / cached in $0.028 / in $0.28 / out $0.42 / 1M tok" },
    { value: "gpt-5-nano", label: "gpt-5-nano", note: "OpenAI / cached in $0.005 / in $0.05 / out $0.40 / 1M tok" },
    { value: "gpt-5-mini", label: "gpt-5-mini", note: "OpenAI / cached in $0.025 / in $0.25 / out $2.00 / 1M tok" },
    { value: "gpt-5", label: "gpt-5", note: "OpenAI / cached in $0.125 / in $1.25 / out $10.00 / 1M tok" },
    { value: "gpt-5-pro", label: "gpt-5-pro", note: "OpenAI / in $15.00 / out $120.00 / 1M tok" },
    { value: "gpt-5.1", label: "gpt-5.1", note: "OpenAI / cached in $0.125 / in $1.25 / out $10.00 / 1M tok" },
    { value: "gpt-5.2", label: "gpt-5.2", note: "OpenAI / cached in $0.175 / in $1.75 / out $14.00 / 1M tok" },
    { value: "gpt-5.2-pro", label: "gpt-5.2-pro", note: "OpenAI / in $21.00 / out $168.00 / 1M tok" },
    { value: "gpt-5.4", label: "gpt-5.4", note: "OpenAI / cached in $0.25 / in $2.50 / out $15.00 / 1M tok" },
    { value: "gpt-5.4-pro", label: "gpt-5.4-pro", note: "OpenAI / in $30.00 / out $180.00 / 1M tok" },
  ];
  const openAIEmbeddingModelOptions: ModelOption[] = [
    { value: "text-embedding-3-small", label: "text-embedding-3-small", note: "$0.02 / 1M tok" },
    { value: "text-embedding-3-large", label: "text-embedding-3-large", note: "$0.13 / 1M tok" },
  ];
  const modelComparisonEntries: ModelComparisonEntry[] = [
    { model: "claude-haiku-4-5", provider: "anthropic", inputPrice: "$1", outputPrice: "$5", recommendation: "strong", bestFor: "facts", highlights: [] },
    { model: "claude-sonnet-4-6", provider: "anthropic", inputPrice: "$3", outputPrice: "$15", recommendation: "recommended", bestFor: "balanced", highlights: ["jsonStable"] },
    { model: "claude-opus-4-6", provider: "anthropic", inputPrice: "$5", outputPrice: "$25", recommendation: "strong", bestFor: "digest", highlights: [] },
    { model: "gemini-3.1-pro-preview", provider: "google", inputPrice: "$2", outputPrice: "$12", recommendation: "strong", bestFor: "digest", highlights: [], status: "preview" },
    { model: "gemini-3.1-flash-lite-preview", provider: "google", inputPrice: "$0.25", outputPrice: "$1.50", recommendation: "recommended", bestFor: "facts", highlights: ["lowestCost", "fast"], status: "preview" },
    { model: "gemini-3-flash-preview", provider: "google", inputPrice: "$0.50", outputPrice: "$3.00", recommendation: "strong", bestFor: "summary", highlights: ["fast"], status: "preview" },
    { model: "gemini-2.5-flash", provider: "google", inputPrice: "$0.30", outputPrice: "$2.50", recommendation: "recommended", bestFor: "ask", highlights: ["fast"] },
    { model: "gemini-2.5-flash-lite", provider: "google", inputPrice: "$0.10", outputPrice: "$0.40", recommendation: "strong", bestFor: "facts", highlights: ["lowestCost", "fast"] },
    { model: "gemini-2.5-pro", provider: "google", inputPrice: "$1.25", outputPrice: "$10", recommendation: "strong", bestFor: "digest", highlights: [] },
    { model: "openai/gpt-oss-20b", provider: "groq", inputPrice: "$0.075", outputPrice: "$0.30", recommendation: "recommended", bestFor: "ask", highlights: ["jsonStable", "fast"], comment: "超高速でコスパ抜群。ツール利用に強く、抽出段階の定番候補。" },
    { model: "openai/gpt-oss-120b", provider: "groq", inputPrice: "$0.15", outputPrice: "$0.60", recommendation: "recommended", bestFor: "summary", highlights: ["jsonStable", "fast"], comment: "長文処理と推論のバランスが良く、検索結果を混ぜる要約で扱いやすい。" },
    { model: "llama-3.1-8b-instant", provider: "groq", inputPrice: "$0.05", outputPrice: "$0.08", recommendation: "strong", bestFor: "facts", highlights: ["lowestCost", "fast"], comment: "最安かつ爆速で日常タスク向き。複雑推論は弱いが低予算運用の第一候補。" },
    { model: "llama-3.3-70b-versatile", provider: "groq", inputPrice: "$0.59", outputPrice: "$0.79", recommendation: "strong", bestFor: "summary", highlights: ["fast"], comment: "知能は非常に高いが、速度とコストは重めで大量処理では負担が出やすい。" },
    { model: "meta-llama/llama-4-scout-17b-16e-instruct", provider: "groq", inputPrice: "$0.11", outputPrice: "$0.34", recommendation: "experimental", bestFor: "summary", highlights: ["fast"], status: "preview", comment: "MoEで賢さと価格のバランスが良く、要約やRAGを安く回したい時に有力。" },
    { model: "qwen/qwen3-32b", provider: "groq", inputPrice: "$0.29", outputPrice: "$0.59", recommendation: "experimental", bestFor: "summary", highlights: [], comment: "日本語と多言語の安定感が高く、混在記事の忠実要約やRAGで特に強い。" },
    { model: "deepseek-chat", provider: "deepseek", inputPrice: "$0.28", outputPrice: "$0.42", recommendation: "strong", bestFor: "facts", highlights: ["fast"], comment: "低価格で扱いやすく、事実抽出や通常要約のコストを抑えたい時の有力候補。" },
    { model: "deepseek-reasoner", provider: "deepseek", inputPrice: "$0.28", outputPrice: "$0.42", recommendation: "strong", bestFor: "digest", highlights: [], comment: "推論寄りの長文整理に向くが、応答の重さを許容する高難度タスク向け。" },
    { model: "gpt-5-nano", provider: "openai", inputPrice: "$0.05", outputPrice: "$0.40", recommendation: "strong", bestFor: "facts", highlights: ["lowestCost", "jsonStable"], comment: "OpenAI系の最安枠。軽い抽出や比較用途で試しやすいが、長文生成の主力には向かない。" },
    { model: "gpt-5-mini", provider: "openai", inputPrice: "$0.25", outputPrice: "$2.00", recommendation: "recommended", bestFor: "facts", highlights: ["fast", "jsonStable"], comment: "OpenAI系の実用最小構成。事実抽出や短めの要約でコストと安定性のバランスが良い。" },
    { model: "gpt-5", provider: "openai", inputPrice: "$1.25", outputPrice: "$10", recommendation: "strong", bestFor: "summary", highlights: ["jsonStable"], comment: "通常要約や質問応答の本命候補。コストは上がるが、JSON整形と長文品質は期待しやすい。" },
    { model: "gpt-5-pro", provider: "openai", inputPrice: "$15", outputPrice: "$120", recommendation: "experimental", bestFor: "digest", highlights: [], comment: "高価だが比較検証用の上限。日常運用向きではなく、難しいダイジェスト生成の比較対象。" },
    { model: "gpt-5.1", provider: "openai", inputPrice: "$1.25", outputPrice: "$10", recommendation: "strong", bestFor: "summary", highlights: ["jsonStable"], comment: "gpt-5 系の安定比較枠。標準的な要約品質を見たい時に選びやすい。" },
    { model: "gpt-5.2", provider: "openai", inputPrice: "$1.75", outputPrice: "$14", recommendation: "strong", bestFor: "ask", highlights: ["jsonStable"], comment: "gpt-5 より一段強めの比較候補。RAG質問応答や少し難しい整理で試す価値がある。" },
    { model: "gpt-5.2-pro", provider: "openai", inputPrice: "$21", outputPrice: "$168", recommendation: "experimental", bestFor: "digest", highlights: [], comment: "高額な上位比較用。構造化出力制約もあるため、常用ではなく精度比較に限定したい。" },
    { model: "gpt-5.4", provider: "openai", inputPrice: "$2.50", outputPrice: "$15", recommendation: "recommended", bestFor: "digest", highlights: ["jsonStable"], comment: "OpenAI 5系の上位比較候補。ダイジェストやクラスタ草稿の品質確認に向く。" },
    { model: "gpt-5.4-pro", provider: "openai", inputPrice: "$30", outputPrice: "$180", recommendation: "experimental", bestFor: "digest", highlights: [], comment: "現行の最上位比較枠。価格は極端に高いので、常用ではなくベンチマーク専用。" },
    { model: "text-embedding-3-small", provider: "openai", inputPrice: "$0.02", outputPrice: "-", recommendation: "recommended", bestFor: "embedding", highlights: ["lowestCost"] },
    { model: "text-embedding-3-large", provider: "openai", inputPrice: "$0.13", outputPrice: "-", recommendation: "strong", bestFor: "embedding", highlights: [] },
  ];

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.getSettings();
      setSettings(data);
      setBudgetUSD(data.monthly_budget_usd == null ? "" : String(data.monthly_budget_usd));
      setAlertEnabled(Boolean(data.budget_alert_enabled));
      setThresholdPct(data.budget_alert_threshold_pct ?? 20);
      setDigestEmailEnabled(Boolean(data.digest_email_enabled ?? true));
      setReadingPlanWindow((data.reading_plan?.window as "24h" | "today_jst" | "7d") ?? "24h");
      const rpSize = data.reading_plan?.size;
      setReadingPlanSize(String(rpSize === 7 || rpSize === 15 || rpSize === 25 ? rpSize : 15));
      setReadingPlanDiversifyTopics(Boolean(data.reading_plan?.diversify_topics ?? true));
      setAnthropicFactsModel(data.llm_models?.anthropic_facts ?? "");
      setAnthropicSummaryModel(data.llm_models?.anthropic_summary ?? "");
      setAnthropicDigestClusterModel(data.llm_models?.anthropic_digest_cluster ?? "");
      setAnthropicDigestModel(data.llm_models?.anthropic_digest ?? "");
      setAnthropicAskModel(data.llm_models?.anthropic_ask ?? "");
      setAnthropicSourceSuggestionModel(data.llm_models?.anthropic_source_suggestion ?? "");
      setOpenAIEmbeddingModel(data.llm_models?.openai_embedding ?? "");
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    const inoreaderStatus = new URLSearchParams(window.location.search).get("inoreader");
    if (inoreaderStatus === "connected") {
      showToast(t("settings.toast.inoreaderConnected"), "success");
    } else if (inoreaderStatus === "error") {
      showToast(t("settings.toast.inoreaderConnectError"), "error");
    }
  }, [showToast, t]);

  const budgetRemainingTone = useMemo(() => {
    const v = settings?.current_month.remaining_budget_pct;
    if (v == null) return "text-zinc-700";
    if (v < 0) return "text-red-600";
    if (v < thresholdPct) return "text-amber-600";
    return "text-zinc-700";
  }, [settings?.current_month.remaining_budget_pct, thresholdPct]);

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
      await api.updateLLMModelSettings({
        anthropic_facts: emptyToNull(anthropicFactsModel),
        anthropic_summary: emptyToNull(anthropicSummaryModel),
        anthropic_digest_cluster: emptyToNull(anthropicDigestClusterModel),
        anthropic_digest: emptyToNull(anthropicDigestModel),
        anthropic_ask: emptyToNull(anthropicAskModel),
        anthropic_source_suggestion: emptyToNull(anthropicSourceSuggestionModel),
        openai_embedding: emptyToNull(openAIEmbeddingModel),
      });
      await load();
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
    <div className="mx-auto max-w-5xl space-y-6">
      <div>
        <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
          <SettingsIcon className="size-6 text-zinc-500" aria-hidden="true" />
          <span>{t("settings.title")}</span>
        </h1>
        <p className="mt-1 text-sm text-zinc-500">{t("settings.subtitle")}</p>
      </div>

      <section className="grid gap-3 md:grid-cols-3">
        <MetricCard
          label={t("settings.metric.mtdCost")}
          value={`$${settings.current_month.estimated_cost_usd.toFixed(6)}`}
        />
        <MetricCard
          label={t("settings.metric.monthlyBudget")}
          value={settings.monthly_budget_usd == null ? "—" : `$${settings.monthly_budget_usd.toFixed(2)}`}
        />
        <MetricCard
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
        <form onSubmit={submitAnthropicApiKey} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <KeyRound className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.anthropicTitle")}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {t("settings.anthropicDescription")}
            </p>
          </div>

          <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
            {settings.has_anthropic_api_key ? (
              <>
                {t("settings.configured")}{" "}
                <span className="font-mono text-xs text-zinc-500">
                  ••••{settings.anthropic_api_key_last4 ?? "****"}
                </span>
              </>
            ) : (
              <span className="text-zinc-500">
                {t("settings.anthropicNotSet")}
              </span>
            )}
          </div>

          <label className="mt-4 block text-sm font-medium text-zinc-700">
            {t("settings.newApiKey")}
          </label>
          <input
            type="password"
            autoComplete="off"
            value={anthropicApiKeyInput}
            onChange={(e) => setAnthropicApiKeyInput(e.target.value)}
            placeholder="sk-ant-..."
            className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none ring-0 placeholder:text-zinc-400 focus:border-zinc-400"
          />

          <div className="mt-4 flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={savingAnthropicKey}
              className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {savingAnthropicKey
                ? t("common.saving")
                : t("settings.saveOrUpdate")}
            </button>
            <button
              type="button"
              disabled={deletingAnthropicKey || !settings.has_anthropic_api_key}
              onClick={handleDeleteAnthropicApiKey}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
            >
              {deletingAnthropicKey
                ? t("settings.deleting")
                : t("settings.deleteKey")}
            </button>
          </div>
        </form>

        <form onSubmit={submitOpenAIApiKey} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <KeyRound className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.openaiTitle")}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {t("settings.openaiDescription")}
            </p>
          </div>

          <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
            {settings.has_openai_api_key ? (
              <>
                {t("settings.configured")}{" "}
                <span className="font-mono text-xs text-zinc-500">
                  ••••{settings.openai_api_key_last4 ?? "****"}
                </span>
              </>
            ) : (
              <span className="text-zinc-500">
                {t("settings.openaiNotSet")}
              </span>
            )}
          </div>

          <label className="mt-4 block text-sm font-medium text-zinc-700">
            {t("settings.newApiKey")}
          </label>
          <input
            type="password"
            autoComplete="off"
            value={openAIApiKeyInput}
            onChange={(e) => setOpenAIApiKeyInput(e.target.value)}
            placeholder="sk-..."
            className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none ring-0 placeholder:text-zinc-400 focus:border-zinc-400"
          />

          <div className="mt-4 flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={savingOpenAIKey}
              className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {savingOpenAIKey
                ? t("common.saving")
                : t("settings.saveOrUpdate")}
            </button>
            <button
              type="button"
              disabled={deletingOpenAIKey || !settings.has_openai_api_key}
              onClick={handleDeleteOpenAIApiKey}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
            >
              {deletingOpenAIKey
                ? t("settings.deleting")
                : t("settings.deleteKey")}
            </button>
          </div>
        </form>

        <form onSubmit={submitGoogleApiKey} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <KeyRound className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.googleTitle")}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {t("settings.googleDescription")}
            </p>
          </div>

          <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
            {settings.has_google_api_key ? (
              <>
                {t("settings.configured")}{" "}
                <span className="font-mono text-xs text-zinc-500">
                  ••••{settings.google_api_key_last4 ?? "****"}
                </span>
              </>
            ) : (
              <span className="text-zinc-500">
                {t("settings.googleNotSet")}
              </span>
            )}
          </div>

          <label className="mt-4 block text-sm font-medium text-zinc-700">
            {t("settings.newApiKey")}
          </label>
          <input
            type="password"
            autoComplete="off"
            value={googleApiKeyInput}
            onChange={(e) => setGoogleApiKeyInput(e.target.value)}
            placeholder="AIza..."
            className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none ring-0 placeholder:text-zinc-400 focus:border-zinc-400"
          />

          <div className="mt-4 flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={savingGoogleKey}
              className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {savingGoogleKey
                ? t("common.saving")
                : t("settings.saveOrUpdate")}
            </button>
            <button
              type="button"
              disabled={deletingGoogleKey || !settings.has_google_api_key}
              onClick={handleDeleteGoogleApiKey}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
            >
              {deletingGoogleKey
                ? t("settings.deleting")
                : t("settings.deleteKey")}
            </button>
          </div>
        </form>

        <form onSubmit={submitGroqApiKey} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <KeyRound className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.groqTitle")}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {t("settings.groqDescription")}
            </p>
          </div>

          <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
            {settings.has_groq_api_key ? (
              <>
                {t("settings.configured")}{" "}
                <span className="font-mono text-xs text-zinc-500">
                  ••••{settings.groq_api_key_last4 ?? "****"}
                </span>
              </>
            ) : (
              <span className="text-zinc-500">
                {t("settings.groqNotSet")}
              </span>
            )}
          </div>

          <label className="mt-4 block text-sm font-medium text-zinc-700">
            {t("settings.newApiKey")}
          </label>
          <input
            type="password"
            autoComplete="off"
            value={groqApiKeyInput}
            onChange={(e) => setGroqApiKeyInput(e.target.value)}
            placeholder="gsk_..."
            className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none ring-0 placeholder:text-zinc-400 focus:border-zinc-400"
          />

          <div className="mt-4 flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={savingGroqKey}
              className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {savingGroqKey ? t("common.saving") : t("settings.saveOrUpdate")}
            </button>
            <button
              type="button"
              disabled={deletingGroqKey || !settings.has_groq_api_key}
              onClick={handleDeleteGroqApiKey}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
            >
              {deletingGroqKey ? t("settings.deleting") : t("settings.deleteKey")}
            </button>
          </div>
        </form>

        <form onSubmit={submitDeepSeekApiKey} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <KeyRound className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.deepseekTitle")}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {t("settings.deepseekDescription")}
            </p>
          </div>

          <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
            {settings.has_deepseek_api_key ? (
              <>
                {t("settings.configured")}{" "}
                <span className="font-mono text-xs text-zinc-500">
                  ••••{settings.deepseek_api_key_last4 ?? "****"}
                </span>
              </>
            ) : (
              <span className="text-zinc-500">
                {t("settings.deepseekNotSet")}
              </span>
            )}
          </div>

          <label className="mt-4 block text-sm font-medium text-zinc-700">
            {t("settings.newApiKey")}
          </label>
          <input
            type="password"
            autoComplete="off"
            value={deepseekApiKeyInput}
            onChange={(e) => setDeepseekApiKeyInput(e.target.value)}
            placeholder="sk-..."
            className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none ring-0 placeholder:text-zinc-400 focus:border-zinc-400"
          />

          <div className="mt-4 flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={savingDeepSeekKey}
              className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {savingDeepSeekKey ? t("common.saving") : t("settings.saveOrUpdate")}
            </button>
            <button
              type="button"
              disabled={deletingDeepSeekKey || !settings.has_deepseek_api_key}
              onClick={handleDeleteDeepSeekApiKey}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
            >
              {deletingDeepSeekKey ? t("settings.deleting") : t("settings.deleteKey")}
            </button>
          </div>
        </form>

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

        <OneSignalSettings />

        <form onSubmit={submitLLMModels} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <Brain className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.modelsTitle")}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {t("settings.modelsDescription")}
            </p>
            <p className="mt-1 text-xs text-zinc-400">
              {t("settings.pricingDescription")}
            </p>
            <div className="mt-3">
              <button
                type="button"
                onClick={() => setIsModelGuideOpen(true)}
                className="inline-flex items-center rounded-full border border-zinc-300 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 hover:border-zinc-400 hover:text-zinc-900"
              >
                {t("settings.modelGuide.open")}
              </button>
            </div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <ModelSelect
              label={t("settings.model.facts")}
              value={anthropicFactsModel}
              onChange={setAnthropicFactsModel}
              options={llmModelOptions}
            />
            <ModelSelect
              label={t("settings.model.summary")}
              value={anthropicSummaryModel}
              onChange={setAnthropicSummaryModel}
              options={llmModelOptions}
            />
            <ModelSelect
              label={t("settings.model.digestCluster")}
              value={anthropicDigestClusterModel}
              onChange={setAnthropicDigestClusterModel}
              options={llmModelOptions}
            />
            <ModelSelect
              label={t("settings.model.digest")}
              value={anthropicDigestModel}
              onChange={setAnthropicDigestModel}
              options={llmModelOptions}
            />
            <ModelSelect
              label={t("settings.model.ask")}
              value={anthropicAskModel}
              onChange={setAnthropicAskModel}
              options={llmModelOptions}
            />
            <ModelSelect
              label={t("settings.model.sourceSuggestion")}
              value={anthropicSourceSuggestionModel}
              onChange={setAnthropicSourceSuggestionModel}
              options={llmModelOptions}
            />
            <ModelSelect
              label={t("settings.model.embeddings")}
              value={openAIEmbeddingModel}
              onChange={setOpenAIEmbeddingModel}
              options={openAIEmbeddingModelOptions}
            />
          </div>
          <div className="mt-4">
            <button
              type="submit"
              disabled={savingLLMModels}
              className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {savingLLMModels
                ? t("common.saving")
                : t("settings.saveModels")}
            </button>
          </div>
        </form>

      </section>

      <section className="space-y-6">
        <form onSubmit={submitReadingPlan} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <Brain className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.recommendedTitle")}
            </h2>
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
              {savingReadingPlan
                ? t("common.saving")
                : t("settings.saveRecommended")}
            </button>
          </div>
        </form>

        <form onSubmit={submitDigestDelivery} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <Mail className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.digestTitle")}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {t("settings.digestDescription")}
            </p>
          </div>

          <div className="flex items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
            <div>
              <div className="text-sm font-medium text-zinc-800">
                {t("settings.digestEmailSending")}
              </div>
              <div className="text-xs text-zinc-500">
                {t("settings.digestDisabledHint")}
              </div>
            </div>
            <label className="inline-flex cursor-pointer items-center gap-2 text-sm text-zinc-700">
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
              {savingDigestDelivery
                ? t("common.saving")
                : t("settings.saveDelivery")}
            </button>
          </div>
        </form>

        <form onSubmit={submitBudget} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <Coins className="size-4 text-zinc-500" aria-hidden="true" />
              {t("settings.budgetTitle")}
            </h2>
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
              <div>
                <div className="text-sm font-medium text-zinc-800">
                  {t("settings.budgetAlertEmail")}
                </div>
                <div className="text-xs text-zinc-500">
                  {t("settings.budgetAlertHint")}
                </div>
              </div>
              <label className="inline-flex cursor-pointer items-center gap-2 text-sm text-zinc-700">
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
              {savingBudget
                ? t("common.saving")
                : t("settings.saveBudget")}
            </button>
            <span className="text-xs text-zinc-500">
              {`${t("settings.currentMonth")}: ${settings.current_month.month_jst}`}
            </span>
          </div>
        </form>
      </section>

      {isModelGuideOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6">
          <div className="flex max-h-[90vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-2xl">
            <div className="flex items-start justify-between gap-4 border-b border-zinc-200 px-5 py-4">
              <div>
                <h2 className="text-base font-semibold text-zinc-900">
                  {t("settings.modelGuide.title")}
                </h2>
                <p className="mt-1 text-sm text-zinc-500">
                  {t("settings.modelGuide.description")}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsModelGuideOpen(false)}
                className="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-sm font-medium text-zinc-700 hover:border-zinc-400 hover:text-zinc-900"
              >
                {t("common.close")}
              </button>
            </div>
            <div className="overflow-auto px-5 py-4">
              <table className="min-w-[1600px] table-auto border-separate border-spacing-0 text-sm">
                <thead>
                  <tr className="border-b border-zinc-200 text-xs font-semibold uppercase tracking-wide text-zinc-500">
                    <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.model")}</th>
                    <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.provider")}</th>
                    <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.inputPrice")}</th>
                    <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.outputPrice")}</th>
                    <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.recommendation")}</th>
                    <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.highlights")}</th>
                    <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.bestFor")}</th>
                    <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.comment")}</th>
                  </tr>
                </thead>
                <tbody>
                  {modelComparisonEntries.map((entry) => (
                    <tr key={entry.model} className="text-zinc-700">
                      <td className="border-b border-zinc-100 px-3 py-3 align-top">
                        <div className="whitespace-nowrap font-medium text-zinc-900">{entry.model}</div>
                      </td>
                      <td className="border-b border-zinc-100 px-3 py-3 align-top text-zinc-600 whitespace-nowrap">{t(`settings.modelGuide.provider.${entry.provider}`)}</td>
                      <td className="border-b border-zinc-100 px-3 py-3 align-top text-zinc-600 whitespace-nowrap">{entry.inputPrice}</td>
                      <td className="border-b border-zinc-100 px-3 py-3 align-top text-zinc-600 whitespace-nowrap">{entry.outputPrice}</td>
                      <td className="border-b border-zinc-100 px-3 py-3 align-top whitespace-nowrap">
                        <span
                          className={`inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${
                            entry.recommendation === "recommended"
                              ? "bg-emerald-50 text-emerald-700"
                              : entry.recommendation === "strong"
                                ? "bg-blue-50 text-blue-700"
                                : "bg-zinc-100 text-zinc-700"
                          }`}
                        >
                          {t(`settings.modelGuide.recommendation.${entry.recommendation}`)}
                        </span>
                      </td>
                      <td className="border-b border-zinc-100 px-3 py-3 align-top">
                        <div className="flex flex-wrap gap-1.5">
                          {entry.highlights.length > 0 ? entry.highlights.map((highlight) => (
                            <span
                              key={highlight}
                              className="inline-flex rounded-full bg-zinc-100 px-2.5 py-1 text-xs font-medium text-zinc-700 whitespace-nowrap"
                            >
                              {t(`settings.modelGuide.highlights.${highlight}`)}
                            </span>
                          )) : (
                            <span className="text-zinc-400">-</span>
                          )}
                        </div>
                      </td>
                      <td className="border-b border-zinc-100 px-3 py-3 align-top text-zinc-600 whitespace-nowrap">{t(`settings.modelGuide.bestFor.${entry.bestFor}`)}</td>
                      <td className="border-b border-zinc-100 px-3 py-3 align-top whitespace-nowrap text-xs leading-5 text-zinc-600">{entry.comment ?? "-"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function MetricCard({
  label,
  value,
  valueClassName,
}: {
  label: string;
  value: string;
  valueClassName?: string;
}) {
  return (
    <div className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
      <div className="text-xs font-medium text-zinc-500">{label}</div>
      <div className={`mt-2 text-xl font-semibold tracking-tight ${valueClassName ?? "text-zinc-900"}`}>
        {value}
      </div>
    </div>
  );
}

function ModelSelect({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: ModelOption[];
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-zinc-700">{label}</label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
      >
        <option value="">Default</option>
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.note ? `${opt.label} (${opt.note})` : opt.label}
          </option>
        ))}
      </select>
    </div>
  );
}
