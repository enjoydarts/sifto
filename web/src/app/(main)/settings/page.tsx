"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { api, UserSettings } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";

export default function SettingsPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const [loading, setLoading] = useState(true);
  const [savingBudget, setSavingBudget] = useState(false);
  const [savingReadingPlan, setSavingReadingPlan] = useState(false);
  const [savingAnthropicKey, setSavingAnthropicKey] = useState(false);
  const [deletingAnthropicKey, setDeletingAnthropicKey] = useState(false);
  const [savingOpenAIKey, setSavingOpenAIKey] = useState(false);
  const [deletingOpenAIKey, setDeletingOpenAIKey] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [budgetUSD, setBudgetUSD] = useState<string>("");
  const [alertEnabled, setAlertEnabled] = useState(false);
  const [thresholdPct, setThresholdPct] = useState<number>(20);
  const [anthropicApiKeyInput, setAnthropicApiKeyInput] = useState("");
  const [openAIApiKeyInput, setOpenAIApiKeyInput] = useState("");
  const [readingPlanWindow, setReadingPlanWindow] = useState<"24h" | "today_jst" | "7d">("24h");
  const [readingPlanSize, setReadingPlanSize] = useState<string>("15");
  const [readingPlanDiversifyTopics, setReadingPlanDiversifyTopics] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.getSettings();
      setSettings(data);
      setBudgetUSD(data.monthly_budget_usd == null ? "" : String(data.monthly_budget_usd));
      setAlertEnabled(Boolean(data.budget_alert_enabled));
      setThresholdPct(data.budget_alert_threshold_pct ?? 20);
      setReadingPlanWindow((data.reading_plan?.window as "24h" | "today_jst" | "7d") ?? "24h");
      setReadingPlanSize(String(data.reading_plan?.size ?? 15));
      setReadingPlanDiversifyTopics(Boolean(data.reading_plan?.diversify_topics ?? true));
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
      });
      await load();
      showToast(locale === "ja" ? "予算設定を保存しました" : "Budget settings saved", "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSavingBudget(false);
    }
  }

  async function submitReadingPlan(e: FormEvent) {
    e.preventDefault();
    setSavingReadingPlan(true);
    try {
      const parsedSize = Number(readingPlanSize);
      if (!Number.isFinite(parsedSize) || parsedSize < 1 || parsedSize > 100) {
        throw new Error(locale === "ja" ? "件数は1〜100で指定してください" : "Size must be between 1 and 100");
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
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">{t("settings.title")}</h1>
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

      <section className="grid gap-6 xl:grid-cols-[1.1fr_1fr]">
        <form onSubmit={submitAnthropicApiKey} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="text-base font-semibold text-zinc-900">
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
            <h2 className="text-base font-semibold text-zinc-900">
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

      </section>

      <section className="grid gap-6 xl:grid-cols-[1fr_1fr]">
        <form onSubmit={submitReadingPlan} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="text-base font-semibold text-zinc-900">
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
              <input
                type="number"
                min={1}
                max={100}
                value={readingPlanSize}
                onChange={(e) => setReadingPlanSize(e.target.value)}
                className="mt-1 w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-900"
              />
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

        <div>
        <form onSubmit={submitBudget} className="rounded-xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="mb-4">
            <h2 className="text-base font-semibold text-zinc-900">
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
        </div>
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
