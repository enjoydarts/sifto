"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { Brain, Coins, KeyRound, Mail, Settings as SettingsIcon } from "lucide-react";
import { api, UserSettings } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";

type ModelOption = {
  value: string;
  label: string;
  note?: string;
};

export default function SettingsPage() {
  const { t, locale } = useI18n();
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
  const [error, setError] = useState<string | null>(null);
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [budgetUSD, setBudgetUSD] = useState<string>("");
  const [alertEnabled, setAlertEnabled] = useState(false);
  const [thresholdPct, setThresholdPct] = useState<number>(20);
  const [digestEmailEnabled, setDigestEmailEnabled] = useState(true);
  const [anthropicApiKeyInput, setAnthropicApiKeyInput] = useState("");
  const [openAIApiKeyInput, setOpenAIApiKeyInput] = useState("");
  const [readingPlanWindow, setReadingPlanWindow] = useState<"24h" | "today_jst" | "7d">("24h");
  const [readingPlanSize, setReadingPlanSize] = useState<string>("15");
  const [readingPlanDiversifyTopics, setReadingPlanDiversifyTopics] = useState(true);
  const [anthropicFactsModel, setAnthropicFactsModel] = useState("");
  const [anthropicSummaryModel, setAnthropicSummaryModel] = useState("");
  const [anthropicDigestModel, setAnthropicDigestModel] = useState("");
  const [anthropicSourceSuggestionModel, setAnthropicSourceSuggestionModel] = useState("");
  const [openAIEmbeddingModel, setOpenAIEmbeddingModel] = useState("");

  const anthropicModelOptions: ModelOption[] = [
    { value: "claude-haiku-4-5", label: "claude-haiku-4-5", note: "in $1 / out $5 / 1M tok" },
    { value: "claude-sonnet-4-6", label: "claude-sonnet-4-6", note: "in $3 / out $15 / 1M tok" },
    { value: "claude-opus-4-6", label: "claude-opus-4-6", note: "in $5 / out $25 / 1M tok" },
  ];
  const openAIEmbeddingModelOptions: ModelOption[] = [
    { value: "text-embedding-3-small", label: "text-embedding-3-small", note: "$0.02 / 1M tok" },
    { value: "text-embedding-3-large", label: "text-embedding-3-large", note: "$0.13 / 1M tok" },
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
      setAnthropicDigestModel(data.llm_models?.anthropic_digest ?? "");
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
        throw new Error(locale === "ja" ? "月次予算の値が不正です" : "Invalid monthly budget");
      }
      await api.updateSettings({
        monthly_budget_usd: parsed,
        budget_alert_enabled: alertEnabled,
        budget_alert_threshold_pct: thresholdPct,
        digest_email_enabled: digestEmailEnabled,
      });
      await load();
      showToast(locale === "ja" ? "予算設定を保存しました" : "Budget settings saved", "success");
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
        anthropic_digest: emptyToNull(anthropicDigestModel),
        anthropic_source_suggestion: emptyToNull(anthropicSourceSuggestionModel),
        openai_embedding: emptyToNull(openAIEmbeddingModel),
      });
      await load();
      showToast(locale === "ja" ? "LLMモデル設定を保存しました" : "LLM model settings saved", "success");
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
      showToast(locale === "ja" ? "ダイジェスト配信設定を保存しました" : "Digest delivery settings saved", "success");
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
        throw new Error(locale === "ja" ? "件数は 7 / 15 / 25 から選択してください" : "Size must be one of 7 / 15 / 25");
      }
      await api.updateReadingPlanSettings({
        window: readingPlanWindow,
        size: parsedSize,
        diversify_topics: readingPlanDiversifyTopics,
      });
      await load();
      showToast(locale === "ja" ? "おすすめ記事設定を保存しました" : "Reading plan settings saved", "success");
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
        throw new Error(locale === "ja" ? "APIキーを入力してください" : "Enter API key");
      }
      await api.setAnthropicApiKey(anthropicApiKeyInput.trim());
      setAnthropicApiKeyInput("");
      await load();
      showToast(locale === "ja" ? "Anthropic APIキーを保存しました" : "Anthropic API key saved", "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingAnthropicKey(false);
    }
  }

  async function handleDeleteAnthropicApiKey() {
    if (!(await confirm({
      title: locale === "ja" ? "Anthropic APIキーを削除しますか？" : "Delete Anthropic API key?",
      message:
        locale === "ja"
          ? "削除後はこのユーザーの要約・ダイジェスト生成が失敗します。再利用するには再設定が必要です。"
          : "After deletion, facts/summary/digest generation for this user will fail until a new key is configured.",
      confirmLabel: locale === "ja" ? "削除" : "Delete",
      tone: "danger",
    }))) {
      return;
    }
    setDeletingAnthropicKey(true);
    try {
      await api.deleteAnthropicApiKey();
      await load();
      showToast(locale === "ja" ? "Anthropic APIキーを削除しました" : "Anthropic API key deleted", "success");
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
        throw new Error(locale === "ja" ? "APIキーを入力してください" : "Enter API key");
      }
      await api.setOpenAIApiKey(openAIApiKeyInput.trim());
      setOpenAIApiKeyInput("");
      await load();
      showToast(locale === "ja" ? "OpenAI APIキーを保存しました" : "OpenAI API key saved", "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingOpenAIKey(false);
    }
  }

  async function handleDeleteOpenAIApiKey() {
    if (!(await confirm({
      title: locale === "ja" ? "OpenAI APIキーを削除しますか？" : "Delete OpenAI API key?",
      message:
        locale === "ja"
          ? "削除後はこのユーザーのembedding生成が失敗します。再利用するには再設定が必要です。"
          : "After deletion, embedding generation for this user will fail until a new key is configured.",
      confirmLabel: locale === "ja" ? "削除" : "Delete",
      tone: "danger",
    }))) {
      return;
    }
    setDeletingOpenAIKey(true);
    try {
      await api.deleteOpenAIApiKey();
      await load();
      showToast(locale === "ja" ? "OpenAI APIキーを削除しました" : "OpenAI API key deleted", "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setDeletingOpenAIKey(false);
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
          label={locale === "ja" ? "今月の推定利用額" : "MTD estimated cost"}
          value={`$${settings.current_month.estimated_cost_usd.toFixed(6)}`}
        />
        <MetricCard
          label={locale === "ja" ? "今月の予算" : "Monthly budget"}
          value={settings.monthly_budget_usd == null ? "—" : `$${settings.monthly_budget_usd.toFixed(2)}`}
        />
        <MetricCard
          label={locale === "ja" ? "残予算率" : "Budget remaining"}
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
              {locale === "ja" ? "Anthropic APIキー（ユーザー別）" : "Anthropic API Key (Per User)"}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {locale === "ja"
                ? "このユーザーの記事の事実抽出・要約・ダイジェスト生成に使います。"
                : "Required for this user's facts extraction, summaries, and digest generation."}
            </p>
          </div>

          <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
            {settings.has_anthropic_api_key ? (
              <>
                {locale === "ja" ? "設定済み" : "Configured"}{" "}
                <span className="font-mono text-xs text-zinc-500">
                  ••••{settings.anthropic_api_key_last4 ?? "****"}
                </span>
              </>
            ) : (
              <span className="text-zinc-500">
                {locale === "ja" ? "未設定（要約・ダイジェスト生成は実行不可）" : "Not set (LLM processing unavailable)"}
              </span>
            )}
          </div>

          <label className="mt-4 block text-sm font-medium text-zinc-700">
            {locale === "ja" ? "新しいAPIキー" : "New API key"}
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
                ? locale === "ja"
                  ? "保存中…"
                  : "Saving…"
                : locale === "ja"
                  ? "保存 / 更新"
                  : "Save / Update"}
            </button>
            <button
              type="button"
              disabled={deletingAnthropicKey || !settings.has_anthropic_api_key}
              onClick={handleDeleteAnthropicApiKey}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
            >
              {deletingAnthropicKey
                ? locale === "ja"
                  ? "削除中…"
                  : "Deleting…"
                : locale === "ja"
                  ? "キーを削除"
                  : "Delete key"}
            </button>
          </div>
        </form>

        <form onSubmit={submitOpenAIApiKey} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <KeyRound className="size-4 text-zinc-500" aria-hidden="true" />
              {locale === "ja" ? "OpenAI APIキー（ユーザー別）" : "OpenAI API Key (Per User)"}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {locale === "ja"
                ? "このユーザーの記事 embedding 生成と関連記事判定に使います。"
                : "Used for this user's embedding generation and related-article retrieval."}
            </p>
          </div>

          <div className="rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
            {settings.has_openai_api_key ? (
              <>
                {locale === "ja" ? "設定済み" : "Configured"}{" "}
                <span className="font-mono text-xs text-zinc-500">
                  ••••{settings.openai_api_key_last4 ?? "****"}
                </span>
              </>
            ) : (
              <span className="text-zinc-500">
                {locale === "ja" ? "未設定（embedding生成は実行不可）" : "Not set (embedding generation unavailable)"}
              </span>
            )}
          </div>

          <label className="mt-4 block text-sm font-medium text-zinc-700">
            {locale === "ja" ? "新しいAPIキー" : "New API key"}
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
                ? locale === "ja"
                  ? "保存中…"
                  : "Saving…"
                : locale === "ja"
                  ? "保存 / 更新"
                  : "Save / Update"}
            </button>
            <button
              type="button"
              disabled={deletingOpenAIKey || !settings.has_openai_api_key}
              onClick={handleDeleteOpenAIApiKey}
              className="rounded-lg border border-zinc-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700 disabled:opacity-50"
            >
              {deletingOpenAIKey
                ? locale === "ja"
                  ? "削除中…"
                  : "Deleting…"
                : locale === "ja"
                  ? "キーを削除"
                  : "Delete key"}
            </button>
          </div>
        </form>

        <form onSubmit={submitLLMModels} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <Brain className="size-4 text-zinc-500" aria-hidden="true" />
              {locale === "ja" ? "LLMモデル設定（フェーズ別）" : "LLM Model Settings (Per Phase)"}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {locale === "ja"
                ? "未設定時はサーバー既定モデルを使います。AnthropicはOpusも選択できます。"
                : "Uses server defaults when empty. Anthropic Opus is also available."}
            </p>
            <p className="mt-1 text-xs text-zinc-400">
              {locale === "ja"
                ? "単価はコード内の静的テーブル表示（将来改定時は更新が必要）"
                : "Prices shown from the app's static pricing table (update required if provider pricing changes)."}
            </p>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <ModelSelect
              label={locale === "ja" ? "事実抽出 (Claude)" : "Facts (Claude)"}
              value={anthropicFactsModel}
              onChange={setAnthropicFactsModel}
              options={anthropicModelOptions}
            />
            <ModelSelect
              label={locale === "ja" ? "要約 (Claude)" : "Summary (Claude)"}
              value={anthropicSummaryModel}
              onChange={setAnthropicSummaryModel}
              options={anthropicModelOptions}
            />
            <ModelSelect
              label={locale === "ja" ? "ダイジェスト生成 (Claude)" : "Digest (Claude)"}
              value={anthropicDigestModel}
              onChange={setAnthropicDigestModel}
              options={anthropicModelOptions}
            />
            <ModelSelect
              label={locale === "ja" ? "ソース提案 (Claude)" : "Source Suggestions (Claude)"}
              value={anthropicSourceSuggestionModel}
              onChange={setAnthropicSourceSuggestionModel}
              options={anthropicModelOptions}
            />
            <ModelSelect
              label={locale === "ja" ? "Embeddings (OpenAI)" : "Embeddings (OpenAI)"}
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
                ? locale === "ja" ? "保存中…" : "Saving…"
                : locale === "ja" ? "モデル設定を保存" : "Save model settings"}
            </button>
          </div>
        </form>

      </section>

      <section className="space-y-6">
        <form onSubmit={submitReadingPlan} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <Brain className="size-4 text-zinc-500" aria-hidden="true" />
              {locale === "ja" ? "おすすめ記事設定" : "Recommended Feed Settings"}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {locale === "ja"
                ? "記事一覧の「おすすめ」タブで使う既定の選定条件です。"
                : "Default selection rules used by the Recommended tab in Items."}
            </p>
          </div>

          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-zinc-700">
                {locale === "ja" ? "対象期間" : "Window"}
              </label>
              <select
                value={readingPlanWindow}
                onChange={(e) => setReadingPlanWindow(e.target.value as "24h" | "today_jst" | "7d")}
                className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
              >
                <option value="24h">{locale === "ja" ? "過去24時間" : "Last 24h"}</option>
                <option value="today_jst">{locale === "ja" ? "今日 (JST)" : "Today (JST)"}</option>
                <option value="7d">{locale === "ja" ? "過去7日" : "Last 7d"}</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-zinc-700">
                {locale === "ja" ? "表示件数" : "Size"}
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
              <span>{locale === "ja" ? "トピック分散を有効化" : "Diversify topics"}</span>
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
                ? locale === "ja" ? "保存中…" : "Saving…"
                : locale === "ja" ? "おすすめ設定を保存" : "Save recommended settings"}
            </button>
          </div>
        </form>

        <form onSubmit={submitDigestDelivery} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <Mail className="size-4 text-zinc-500" aria-hidden="true" />
              {locale === "ja" ? "ダイジェスト配信" : "Digest Delivery"}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {locale === "ja"
                ? "ダイジェストは生成したまま、メール送信だけ停止できます。"
                : "Keep generating digests while disabling email delivery."}
            </p>
          </div>

          <div className="flex items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
            <div>
              <div className="text-sm font-medium text-zinc-800">
                {locale === "ja" ? "ダイジェストメール送信" : "Digest email sending"}
              </div>
              <div className="text-xs text-zinc-500">
                {locale === "ja"
                  ? "無効でもダイジェスト生成とアプリ内表示は継続します"
                  : "When off, digest generation and in-app viewing continue without email sending"}
              </div>
            </div>
            <label className="inline-flex cursor-pointer items-center gap-2 text-sm text-zinc-700">
              <input
                type="checkbox"
                checked={digestEmailEnabled}
                onChange={(e) => setDigestEmailEnabled(e.target.checked)}
                className="size-4 rounded border-zinc-300"
              />
              {digestEmailEnabled ? (locale === "ja" ? "有効" : "On") : locale === "ja" ? "無効" : "Off"}
            </label>
          </div>

          <div className="mt-4">
            <button
              type="submit"
              disabled={savingDigestDelivery}
              className="rounded-lg bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
            >
              {savingDigestDelivery
                ? locale === "ja"
                  ? "保存中…"
                  : "Saving…"
                : locale === "ja"
                  ? "配信設定を保存"
                  : "Save delivery settings"}
            </button>
          </div>
        </form>

        <form onSubmit={submitBudget} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="inline-flex items-center gap-2 text-base font-semibold text-zinc-900">
              <Coins className="size-4 text-zinc-500" aria-hidden="true" />
              {locale === "ja" ? "月次LLM予算と警告" : "Monthly LLM Budget & Alerts"}
            </h2>
            <p className="mt-1 text-sm text-zinc-500">
              {locale === "ja"
                ? "残予算率がしきい値を下回ったときにメールで通知します。"
                : "Send email alerts when remaining budget ratio falls below the threshold."}
            </p>
          </div>

          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-zinc-700">
                {locale === "ja" ? "月次予算 (USD)" : "Monthly budget (USD)"}
              </label>
              <input
                type="number"
                min={0}
                step="0.01"
                value={budgetUSD}
                onChange={(e) => setBudgetUSD(e.target.value)}
                placeholder={locale === "ja" ? "未設定で無効" : "Leave empty to disable"}
                className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900 outline-none placeholder:text-zinc-400 focus:border-zinc-400"
              />
            </div>

            <div className="flex items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
              <div>
                <div className="text-sm font-medium text-zinc-800">
                  {locale === "ja" ? "予算警告メール" : "Budget alert email"}
                </div>
                <div className="text-xs text-zinc-500">
                  {locale === "ja" ? "Resend が有効な環境で送信されます" : "Sent when Resend is enabled"}
                </div>
              </div>
              <label className="inline-flex cursor-pointer items-center gap-2 text-sm text-zinc-700">
                <input
                  type="checkbox"
                  checked={alertEnabled}
                  onChange={(e) => setAlertEnabled(e.target.checked)}
                  className="size-4 rounded border-zinc-300"
                />
                {alertEnabled ? (locale === "ja" ? "有効" : "On") : locale === "ja" ? "無効" : "Off"}
              </label>
            </div>

            <div>
              <label className="block text-sm font-medium text-zinc-700">
                {locale === "ja" ? "警告しきい値（残予算率 %）" : "Alert threshold (remaining budget %)"}
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
                ? locale === "ja"
                  ? "保存中…"
                  : "Saving…"
                : locale === "ja"
                  ? "予算設定を保存"
                  : "Save budget settings"}
            </button>
            <span className="text-xs text-zinc-500">
              {locale === "ja"
                ? `今月: ${settings.current_month.month_jst}`
                : `Current month: ${settings.current_month.month_jst}`}
            </span>
          </div>
        </form>
      </section>
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
