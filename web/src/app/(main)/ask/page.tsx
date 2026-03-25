"use client";

import Link from "next/link";
import { FormEvent, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Loader2, Search, X } from "lucide-react";
import { InsightSaveDialog } from "@/components/ask/insight-save-dialog";
import { AINavigatorAvatar } from "@/components/briefing/ai-navigator-avatar";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { api, AskNavigator, AskResponse, ReadingGoal } from "@/lib/api";
import { formatModelDisplayName } from "@/lib/model-display";
import { useToast } from "@/components/toast-provider";

const EMPTY: AskResponse | null = null;
const EMPTY_GOALS: ReadingGoal[] = [];
const PRESET_KEYS = [
  "ask.preset.topics",
  "ask.preset.unread",
  "ask.preset.ai",
  "ask.preset.followups",
] as const;
const ASK_LIMIT = 12;
const ASK_CITATION_MARKER = /(\[\d+\])/g;

function renderAnswerWithCitationSuperscript(text: string) {
  return text.split("\n").map((line, lineIndex) => {
    if (line.trim() === "") {
      return <div key={`line-${lineIndex}`} className="h-4" />;
    }
    const parts = line.split(ASK_CITATION_MARKER);
    return (
      <p key={`line-${lineIndex}`} className="mt-4 text-[15px] leading-[2] text-[var(--color-editorial-ink-soft)] first:mt-0">
        {parts.map((part, partIndex) => {
          if (/^\[\d+\]$/.test(part)) {
            return (
              <sup key={`part-${partIndex}`} className="ml-0.5 align-super text-[0.68em] font-semibold leading-none text-[var(--color-editorial-ink-faint)]">
                {part}
              </sup>
            );
          }
          return <span key={`part-${partIndex}`}>{part}</span>;
        })}
      </p>
    );
  });
}

export default function AskPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [days, setDays] = useState("30");
  const [unreadOnly, setUnreadOnly] = useState(true);
  const [loading, setLoading] = useState(false);
  const [savingInsight, setSavingInsight] = useState(false);
  const [saveDialogOpen, setSaveDialogOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<AskResponse | null>(EMPTY);
  const [askNavigator, setAskNavigator] = useState<AskNavigator | null>(null);
  const [askNavigatorLoading, setAskNavigatorLoading] = useState(false);
  const [askNavigatorError, setAskNavigatorError] = useState<string | null>(null);
  const [askNavigatorDismissed, setAskNavigatorDismissed] = useState(false);
  const readingGoalsQuery = useQuery({
    queryKey: ["reading-goals"] as const,
    queryFn: () => api.getReadingGoals(),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
  const settingsQuery = useQuery({
    queryKey: ["settings"] as const,
    queryFn: () => api.getSettings(),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });

  const canSubmit = query.trim().length > 1 && !loading;
  const relatedItems = useMemo(() => result?.related_items ?? [], [result]);
  const bullets = useMemo(() => result?.bullets ?? [], [result]);
  const citations = useMemo(() => result?.citations ?? [], [result]);
  const activeGoals = readingGoalsQuery.data?.active ?? EMPTY_GOALS;
  const presets = useMemo(() => PRESET_KEYS.map((key) => t(key)), [t]);
  const scopeLabel = useMemo(
    () => `${days}d / ${unreadOnly ? t("ask.unreadOnly") : t("ask.allItems")}`,
    [days, unreadOnly, t]
  );
  const askNavigatorDisplayPersona = askNavigator?.avatar_style?.trim() || askNavigator?.persona?.trim() || settingsQuery.data?.llm_models?.navigator_persona?.trim() || "editor";

  async function loadAskNavigator(next: AskResponse) {
    setAskNavigator(null);
    setAskNavigatorError(null);
    setAskNavigatorLoading(true);
    setAskNavigatorDismissed(false);
    try {
      const resp = await api.getAskNavigator({
        query: next.query,
        answer: next.answer,
        bullets: next.bullets ?? [],
        citations: next.citations ?? [],
        related_items: next.related_items ?? [],
      });
      setAskNavigator(resp.navigator ?? null);
    } catch {
      setAskNavigatorError(t("ask.navigator.error"));
    } finally {
      setAskNavigatorLoading(false);
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setLoading(true);
    setError(null);
    setAskNavigator(null);
    setAskNavigatorError(null);
    setAskNavigatorLoading(false);
    setAskNavigatorDismissed(false);
    try {
      const next = await api.ask({
        query: query.trim(),
        days: Number(days),
        unread_only: unreadOnly,
        limit: ASK_LIMIT,
      });
      setResult(next);
      void loadAskNavigator(next);
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  }

  async function saveInsight(input: { title: string; body: string; goal_id?: string | null; tags: string[]; item_ids: string[] }) {
    if (!result) return;
    setSavingInsight(true);
    try {
      await api.saveAskInsight({
        title: input.title,
        body: input.body,
        goal_id: input.goal_id,
        tags: input.tags,
        item_ids: input.item_ids,
        query: result.query,
      });
      await queryClient.invalidateQueries({ queryKey: ["ask-insights"] });
      setSaveDialogOpen(false);
      showToast(t("ask.insight.saved"), "success");
    } catch (err) {
      showToast(`${t("common.error")}: ${String(err)}`, "error");
    } finally {
      setSavingInsight(false);
    }
  }

  return (
    <PageTransition>
      <div className="space-y-5 overflow-x-hidden">
        <PageHeader
          eyebrow="Ask Desk"
          title={t("ask.title")}
          titleIcon={Search}
          description={t("ask.subtitle")}
        />

        <section className="surface-editorial rounded-[30px] px-5 py-5 sm:px-6">
          <div className="flex flex-wrap gap-2">
            {presets.map((preset) => (
              <button
                key={preset}
                type="button"
                onClick={() => setQuery(preset)}
                className="rounded-full border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] px-3.5 py-2 text-sm text-[var(--color-editorial-ink-soft)] transition hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {preset}
              </button>
            ))}
          </div>

          <form onSubmit={onSubmit} className="mt-4 space-y-4">
            <textarea
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("ask.placeholder")}
              className="min-h-36 w-full rounded-[24px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] px-5 py-4 text-[15px] leading-8 text-[var(--color-editorial-ink)] outline-none transition placeholder:text-[var(--color-editorial-ink-faint)] focus:border-[var(--color-editorial-line-strong)]"
            />

            <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <div className="flex flex-wrap items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
                <label className="inline-flex items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2">
                  <span>{t("ask.days")}</span>
                  <select
                    value={days}
                    onChange={(e) => setDays(e.target.value)}
                    className="bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none"
                  >
                    <option value="7">7d</option>
                    <option value="30">30d</option>
                    <option value="90">90d</option>
                  </select>
                </label>
                <label className="inline-flex items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2">
                  <input
                    type="checkbox"
                    checked={unreadOnly}
                    onChange={(e) => setUnreadOnly(e.target.checked)}
                    className="h-4 w-4 rounded border-[var(--color-editorial-line-strong)]"
                  />
                  <span>{t("ask.unreadOnly")}</span>
                </label>
              </div>

              <button
                type="submit"
                disabled={!canSubmit}
                className="inline-flex min-h-[46px] items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-5 text-sm font-semibold text-[var(--color-editorial-panel-strong)] transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
                <span>{t("ask.submit")}</span>
              </button>
            </div>
          </form>
        </section>

        {error ? (
          <div className="rounded-[22px] border border-[#e5b7ac] bg-[#f6e8e4] px-4 py-3 text-sm text-[#7a4337]">{error}</div>
        ) : null}

        {result ? (
          <section className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]">
            <section className="min-w-0 surface-editorial rounded-[28px] px-5 py-5 sm:px-6">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("ask.answerLabel")}
                  </div>
                  <div className="mt-2 text-xs text-[var(--color-editorial-ink-faint)]">
                    {result.ask_llm ? `${result.ask_llm.provider} / ${formatModelDisplayName(result.ask_llm.model)} · ` : ""}
                    {scopeLabel}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => setSaveDialogOpen(true)}
                  className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[rgba(255,255,255,0.62)]"
                >
                  {t("ask.insight.cta")}
                </button>
              </div>

              <div className="mt-5 min-w-0 whitespace-pre-wrap break-words">{renderAnswerWithCitationSuperscript(result.answer)}</div>

              {bullets.length > 0 ? (
                <div className="mt-5 grid gap-2">
                  {bullets.map((bullet, idx) => (
                    <div
                      key={`${idx}-${bullet}`}
                      className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[rgba(247,242,234,0.92)] px-4 py-3 text-[13px] leading-7 text-[var(--color-editorial-ink-soft)]"
                    >
                      {bullet}
                    </div>
                  ))}
                </div>
              ) : null}

              {!askNavigatorDismissed && (askNavigatorLoading || askNavigator || askNavigatorError) ? (
                <div className="mt-5 rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] p-4">
                  <div className="flex items-start gap-3">
                    <div className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-1.5 shadow-sm">
                      <AINavigatorAvatar persona={askNavigatorDisplayPersona} className="size-10" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                            {t("ask.navigator.label")}
                          </div>
                          <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
                            <h3 className="font-serif text-[1rem] font-semibold leading-none text-[var(--color-editorial-ink)]">
                              {askNavigator?.character_name ?? t("ask.navigator.label")}
                            </h3>
                            {askNavigator?.character_title ? (
                              <span className="text-xs text-[var(--color-editorial-ink-faint)]">{askNavigator.character_title}</span>
                            ) : null}
                          </div>
                        </div>
                        <button
                          type="button"
                          onClick={() => setAskNavigatorDismissed(true)}
                          className="inline-flex size-8 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white/70 text-[var(--color-editorial-ink-soft)] hover:bg-white"
                          aria-label={t("briefing.navigator.close")}
                        >
                          <X className="size-4" aria-hidden="true" />
                        </button>
                      </div>

                      {askNavigatorLoading ? (
                        <div className="mt-3 flex items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)]">
                          <Loader2 className="size-4 animate-spin" />
                          <span>{t("ask.navigator.loading")}</span>
                        </div>
                      ) : null}

                      {askNavigatorError ? (
                        <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{askNavigatorError}</p>
                      ) : null}

                      {askNavigator ? (
                        <div className="mt-3 space-y-3">
                          <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3">
                            <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{askNavigator.headline}</div>
                            <p className="mt-2 whitespace-pre-line text-[14px] leading-7 text-[var(--color-editorial-ink-soft)]">{askNavigator.commentary}</p>
                          </div>
                          {askNavigator.next_angles && askNavigator.next_angles.length > 0 ? (
                            <div>
                              <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                                {t("ask.navigator.nextAngles")}
                              </div>
                              <div className="mt-2 flex flex-wrap gap-2">
                                {askNavigator.next_angles.map((angle) => (
                                  <span
                                    key={angle}
                                    className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)]"
                                  >
                                    {angle}
                                  </span>
                                ))}
                              </div>
                            </div>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                  </div>
                </div>
              ) : null}
            </section>

            <aside className="min-w-0 grid gap-4">
              <section className="min-w-0 surface-editorial rounded-[28px] px-5 py-5">
                <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                  {t("ask.citationsLabel")}
                </div>
                <div className="mt-4 grid gap-3">
                  {citations.map((citation, index) => (
                    <article key={citation.item_id} className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] p-4">
                      <span className="inline-grid min-h-[30px] min-w-[30px] place-items-center rounded-full bg-[var(--color-editorial-ink)] px-2 text-xs font-semibold text-[var(--color-editorial-panel-strong)]">
                        [{index + 1}]
                      </span>
                      <Link href={`/items/${citation.item_id}`} className="mt-3 block break-words text-[15px] font-semibold leading-6 text-[var(--color-editorial-ink)] hover:underline">
                        {citation.title}
                      </Link>
                      {citation.reason ? (
                        <p className="mt-2 break-words text-[12px] leading-[1.75] text-[var(--color-editorial-ink-faint)]">{citation.reason}</p>
                      ) : null}
                    </article>
                  ))}
                </div>
              </section>

              <section className="min-w-0 surface-editorial rounded-[28px] px-5 py-5">
                <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                  {t("ask.relatedLabel")}
                </div>
                <div className="mt-4 grid gap-3">
                  {relatedItems.map((item) => (
                    <article key={item.id} className="min-w-0 rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] p-4">
                      <Link href={`/items/${item.id}`} className="block min-w-0 break-all text-[15px] font-semibold leading-6 text-[var(--color-editorial-ink)] hover:underline">
                        {item.translated_title || item.title || item.url}
                      </Link>
                      <p className="mt-2 min-w-0 break-all text-[13px] leading-7 text-[var(--color-editorial-ink-soft)]">{item.summary}</p>
                    </article>
                  ))}
                </div>
              </section>
            </aside>
          </section>
        ) : (
          <div className="rounded-[28px] border border-dashed border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.45)] px-5 py-12 text-center text-sm text-[var(--color-editorial-ink-faint)]">
            {t("ask.empty")}
          </div>
        )}

        <InsightSaveDialog
          open={saveDialogOpen}
          loading={savingInsight}
          result={result}
          goals={activeGoals}
          onClose={() => setSaveDialogOpen(false)}
          onSave={saveInsight}
        />
      </div>
    </PageTransition>
  );
}
