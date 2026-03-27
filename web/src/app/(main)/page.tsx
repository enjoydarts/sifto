"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Bell, BookOpen, Flame, Sparkles, X } from "lucide-react";
import { api, BriefingCluster, Item, ProviderModelChangeEvent, ReadingGoal, ReviewQueueItem, TodayQueueItem, WeeklyReviewSnapshot } from "@/lib/api";
import { ReadingGoalsPanel } from "@/components/briefing/reading-goals-panel";
import { DueReviewPanel } from "@/components/reviews/due-review-panel";
import { WeeklyReviewPanel } from "@/components/reviews/weekly-review-panel";
import { InlineReader } from "@/components/inline-reader";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { EmptyState } from "@/components/empty-state";
import { SkeletonCard } from "@/components/skeleton";
import { AINavigatorAvatar } from "@/components/briefing/ai-navigator-avatar";
import { useToast } from "@/components/toast-provider";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";

const EMPTY_ITEMS: Item[] = [];
const EMPTY_CLUSTERS: BriefingCluster[] = [];
const EMPTY_MODEL_UPDATES: ProviderModelChangeEvent[] = [];
const EMPTY_GOALS: ReadingGoal[] = [];
const EMPTY_TODAY_QUEUE: TodayQueueItem[] = [];
const EMPTY_REVIEW_QUEUE: ReviewQueueItem[] = [];
const MODEL_UPDATES_DISMISSED_AT_KEY = "provider-model-updates:dismissed-at";

export default function BriefingPage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [navigatorDismissed, setNavigatorDismissed] = useState(false);
  const [dismissedModelUpdatesAt, setDismissedModelUpdatesAt] = useState<string | null>(() => {
    if (typeof window === "undefined") return null;
    return window.localStorage.getItem(MODEL_UPDATES_DISMISSED_AT_KEY);
  });
  const briefingQuery = useQuery({
    queryKey: ["briefing-today", 18] as const,
    queryFn: () => api.getBriefingToday({ size: 18 }),
    staleTime: 15_000,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
    placeholderData: (prev) => prev,
  });
  const navigatorPreview = searchParams.get("navigator_preview") === "1";
  const settingsQuery = useQuery({
    queryKey: ["settings"] as const,
    queryFn: () => api.getSettings(),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
  const navigatorQuery = useQuery({
    queryKey: ["briefing-navigator", navigatorPreview, settingsQuery.data?.llm_models?.navigator_persona?.trim() || "editor"] as const,
    queryFn: () => api.getBriefingNavigator({ navigator_preview: navigatorPreview }),
    staleTime: 30 * 60 * 1000,
    placeholderData: (prev) => prev,
  });
  const modelUpdatesQuery = useQuery({
    queryKey: ["provider-model-updates", 7] as const,
    queryFn: () => api.getProviderModelUpdates({ days: 7, limit: 6 }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
  const readingGoalsQuery = useQuery({
    queryKey: ["reading-goals"] as const,
    queryFn: () => api.getReadingGoals(),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
  const todayQueueQuery = useQuery({
    queryKey: ["today-queue", 6] as const,
    queryFn: () => api.getTodayQueue({ size: 6 }),
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });
  const reviewQueueQuery = useQuery({
    queryKey: ["review-queue", 5] as const,
    queryFn: () => api.getReviewQueue({ size: 5 }),
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });
  const weeklyReviewQuery = useQuery({
    queryKey: ["weekly-review-latest"] as const,
    queryFn: () => api.getWeeklyReviewLatest(),
    staleTime: 120_000,
    placeholderData: (prev) => prev,
  });

  const loading = !briefingQuery.data && (briefingQuery.isLoading || briefingQuery.isFetching);
  const error = briefingQuery.error ? String(briefingQuery.error) : null;
  const data = briefingQuery.data;
  const modelUpdates = modelUpdatesQuery.data ?? EMPTY_MODEL_UPDATES;
  const navigator = navigatorQuery.data?.navigator ?? null;
  const navigatorDisplayPersona = navigator?.avatar_style || navigator?.persona || settingsQuery.data?.llm_models?.navigator_persona?.trim() || "editor";
  const navigatorTheme = navigator ? navigatorThemeTokens(navigator.persona, navigator.avatar_style) : null;
  const navigatorLoading = !navigator && (navigatorQuery.isLoading || navigatorQuery.isFetching);
  const navigatorLoadingPersona = settingsQuery.data?.llm_models?.navigator_persona?.trim() || "editor";
  const visibleModelUpdates = useMemo(() => {
    if (!dismissedModelUpdatesAt) return modelUpdates;
    const dismissedMs = Date.parse(dismissedModelUpdatesAt);
    if (Number.isNaN(dismissedMs)) return modelUpdates;
    return modelUpdates.filter((event) => Date.parse(event.detected_at) > dismissedMs);
  }, [dismissedModelUpdatesAt, modelUpdates]);
  const highlights = data?.highlight_items ?? EMPTY_ITEMS;
  const activeGoals = readingGoalsQuery.data?.active ?? EMPTY_GOALS;
  const todayQueue = todayQueueQuery.data?.items ?? EMPTY_TODAY_QUEUE;
  const reviewQueue = reviewQueueQuery.data?.items ?? EMPTY_REVIEW_QUEUE;
  const weeklyReview = (weeklyReviewQuery.data ?? null) as WeeklyReviewSnapshot | null;
  const clusters = data?.clusters ?? EMPTY_CLUSTERS;
  const unreadHighlights = useMemo(() => highlights.filter((item) => !item.is_read), [highlights]);
  const nowReading = unreadHighlights[0] ?? highlights[0] ?? null;
  const nextReads = useMemo(() => {
    const src = unreadHighlights.length > 0 ? unreadHighlights : highlights;
    return src.slice(1, 7);
  }, [highlights, unreadHighlights]);
  const generatedAtLabel = (() => {
    if (!data?.generated_at) return null;
    const date = new Date(data.generated_at);
    if (Number.isNaN(date.getTime())) return null;
    return date.toLocaleTimeString(locale === "ja" ? "ja-JP" : "en-US", {
      hour: "2-digit",
      minute: "2-digit",
    });
  })();

  const clusterRows = useMemo(
    () =>
      clusters
        .map((cluster) => ({
          ...cluster,
          topItems: (cluster.items ?? EMPTY_ITEMS).slice(0, 4),
        }))
        .filter((cluster) => cluster.topItems.length > 0),
    [clusters]
  );
  const greetingLabel = (() => {
    const key = data?.greeting_key;
    if (key === "morning") return t("briefing.greeting.morning");
    if (key === "afternoon") return t("briefing.greeting.afternoon");
    if (key === "evening") return t("briefing.greeting.evening");
    if (data?.greeting) return data.greeting;
    return t("briefing.greetingFallback");
  })();
  const greetingSummary = (() => {
    const unread = data?.stats.total_unread ?? 0;
    const picks = data?.stats.today_highlight_count ?? 0;
    if (locale === "ja") {
      return `${unread}件の未読と${picks}件の注目があります`;
    }
    return `${unread} unread and ${picks} picks ready`;
  })();
  const briefingDateLabel = (() => {
    const raw = data?.generated_at ?? data?.date ?? null;
    if (!raw) return null;
    const date = new Date(raw);
    if (Number.isNaN(date.getTime())) return data?.date ?? null;
    return date.toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US", {
      year: "numeric",
      month: "long",
      day: "numeric",
      weekday: "long",
    });
  })();
  const greetingTail = useMemo(() => {
    const seed = `${data?.date ?? ""}:${data?.greeting_key ?? ""}`;
    let hash = 0;
    for (let i = 0; i < seed.length; i += 1) {
      hash = (hash * 31 + seed.charCodeAt(i)) >>> 0;
    }
    const variants = [
      "briefing.tail.0",
      "briefing.tail.1",
      "briefing.tail.2",
      "briefing.tail.3",
      "briefing.tail.4",
      "briefing.tail.5",
      "briefing.tail.6",
      "briefing.tail.7",
      "briefing.tail.8",
      "briefing.tail.9",
      "briefing.tail.10",
      "briefing.tail.11",
    ] as const;
    return t(variants[hash % variants.length]);
  }, [data?.date, data?.greeting_key, t]);
  const briefingQueueItemIds = useMemo(() => {
    const ids: string[] = [];
    const seen = new Set<string>();
    for (const item of highlights) {
      if (seen.has(item.id)) continue;
      seen.add(item.id);
      ids.push(item.id);
    }
    for (const cluster of clusterRows) {
      for (const item of cluster.topItems) {
        if (seen.has(item.id)) continue;
        seen.add(item.id);
        ids.push(item.id);
      }
    }
    return ids;
  }, [clusterRows, highlights]);
  const nowQueueIds = useMemo(() => {
    const ids: string[] = [];
    const seen = new Set<string>();
    if (nowReading) {
      ids.push(nowReading.id);
      seen.add(nowReading.id);
    }
    for (const item of nextReads) {
      if (seen.has(item.id)) continue;
      ids.push(item.id);
      seen.add(item.id);
    }
    return ids.length > 0 ? ids : briefingQueueItemIds;
  }, [briefingQueueItemIds, nextReads, nowReading]);
  const dismissModelUpdates = () => {
    const latest = modelUpdates.reduce<string | null>((max, event) => {
      if (!max) return event.detected_at;
      return Date.parse(event.detected_at) > Date.parse(max) ? event.detected_at : max;
    }, null);
    if (!latest || typeof window === "undefined") return;
    window.localStorage.setItem(MODEL_UPDATES_DISMISSED_AT_KEY, latest);
    setDismissedModelUpdatesAt(latest);
  };

  const navigatorClosed = navigatorPreview ? false : navigatorDismissed;

  const refreshBriefingData = async () => {
    const next = await api.getBriefingToday({ size: 18, cache_bust: true });
    queryClient.setQueryData(["briefing-today", 18], next);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["today-queue"] }),
      queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
    ]);
  };

  const markReviewDone = async (id: string) => {
    try {
      await api.markReviewDone(id);
      await queryClient.invalidateQueries({ queryKey: ["review-queue"] });
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
  };

  const snoozeReview = async (id: string) => {
    try {
      await api.snoozeReview(id, { days: 3 });
      await queryClient.invalidateQueries({ queryKey: ["review-queue"] });
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
  };

  return (
    <PageTransition>
      <div className="space-y-6">
        {visibleModelUpdates.length > 0 && (
        <section className="rounded-[var(--radius-panel)] border border-[#e1cb9e] bg-[var(--warning-soft)]/70 p-4 shadow-[var(--shadow-card)]">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2">
              <Bell className="size-4 shrink-0 text-[var(--warning)]" aria-hidden="true" />
              <div className="min-w-0">
                <h2 className="font-sans text-sm font-semibold text-[var(--warning)]">{t("briefing.providerModelUpdates")}</h2>
                <p className="font-sans text-sm text-[var(--warning)]/90">
                  {visibleModelUpdates.slice(0, 3).map((event) => `${event.provider} ${event.change_type === "added" ? "+" : "-"} ${event.model_id}`).join(" / ")}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={dismissModelUpdates}
                className="inline-flex items-center rounded-full border border-[#e1cb9e] bg-[var(--panel)] px-3 py-2 text-sm font-medium text-[var(--warning)] hover:bg-[var(--panel-strong)] press focus-ring"
              >
                <X className="mr-1 size-4" aria-hidden="true" />
                {t("briefing.providerModelUpdatesDismiss")}
              </button>
              <Link
                href="/settings"
                className="inline-flex items-center rounded-full border border-[#e1cb9e] bg-[var(--panel)] px-3 py-2 text-sm font-medium text-[var(--warning)] hover:bg-[var(--panel-strong)] press focus-ring"
              >
                {t("briefing.providerModelUpdatesOpen")}
              </Link>
            </div>
          </div>
        </section>
        )}

        <SectionCard className="p-5 sm:p-6">
          <div className="min-w-0">
              <div className="font-sans text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-accent)]">
                Morning Briefing
              </div>
              <h1 className="mt-2.5 font-serif text-[2.25rem] leading-[1.05] tracking-[-0.03em] text-[var(--color-editorial-ink)] sm:text-[2.75rem]">
                {greetingLabel}
              </h1>
              <p className="mt-2.5 max-w-3xl font-sans text-[15px] leading-7 text-[var(--color-editorial-ink-soft)]">
                {`${greetingSummary}。 ${greetingTail}`}
              </p>
              <div className="mt-2.5 flex flex-wrap gap-x-4 gap-y-2 font-sans text-[12px] font-medium text-[var(--color-editorial-ink-faint)]">
                {briefingDateLabel ? <span>{briefingDateLabel}</span> : null}
                {generatedAtLabel ? (
                  <span>
                    {t("briefing.generatedAt")} {generatedAtLabel}
                    {data?.status === "stale" ? ` · ${t("briefing.statusStale")}` : ""}
                  </span>
                ) : null}
              </div>
              <div className="mt-4 grid gap-2 md:flex md:flex-wrap">
                <button
                  type="button"
                  onClick={() => {
                    if (nowReading) {
                      setInlineItemId(nowReading.id);
                      return;
                    }
                    router.push("/items?feed=unread&sort=newest");
                  }}
                  className="inline-flex min-h-11 w-full items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 press focus-ring md:w-auto"
                >
                  <BookOpen className="size-4" aria-hidden="true" />
                  {t("briefing.readNow", "今すぐ読む")}
                </button>
                <Link
                  href="/items?feed=unread&sort=newest"
                  className="inline-flex min-h-11 w-full items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring md:w-auto"
                >
                  {t("briefing.hub.openInbox")}
                </Link>
                <Link
                  href="/triage"
                  className="inline-flex min-h-11 w-full items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring md:w-auto"
                >
                  {t("briefing.hub.start")}
                </Link>
              </div>
              {error && <p className="mt-4 text-sm text-[var(--color-editorial-error)]">{String(error)}</p>}
          </div>

          <div className="mt-4 grid gap-3 md:grid-cols-3">
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3.5">
                <div className="font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                  {t("briefing.hub.unread")}
                </div>
                <div className="mt-2.5 text-[2.1rem] leading-none tracking-[-0.04em] text-[var(--color-editorial-ink)]">
                  {String(data?.stats.total_unread ?? 0)}
                </div>
                <p className="mt-1.5 font-sans text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                  {t("briefing.hub.openInbox")}
                </p>
              </div>
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3.5">
                <div className="font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                  {t("briefing.hub.queue")}
                </div>
                <div className="mt-2.5 text-[2.1rem] leading-none tracking-[-0.04em] text-[var(--color-editorial-ink)]">
                  {String(nextReads.length)}
                </div>
                <p className="mt-1.5 font-sans text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                  {t("briefing.hub.openQueue")}
                </p>
              </div>
              {data?.stats.streak_days ? (
                <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3.5">
                  <div className="inline-flex items-center gap-1 font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    <Flame className="size-3.5 text-[var(--warning)]" aria-hidden="true" />
                    <span>Reading Streak</span>
                  </div>
                  <div className="mt-2.5 text-[2.1rem] leading-none tracking-[-0.04em] text-[var(--color-editorial-ink)]">
                    {data.stats.streak_days}{t("briefing.kpi.streakUnit")}
                  </div>
                  {data.stats.streak_at_risk ? (
                    <p className="mt-1.5 font-sans text-[13px] leading-6 text-[var(--color-editorial-error)]">
                      {t("briefing.streakRisk.prefix")}
                      {data?.stats.streak_remaining ?? 0}
                      {t("briefing.streakRisk.unit")}
                      {t("briefing.streakRisk.suffix")}
                    </p>
                  ) : (
                    <p className="mt-1.5 font-sans text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                      {t("briefing.hub.start")}
                    </p>
                  )}
                </div>
              ) : null}
          </div>
        </SectionCard>

        <div className="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_340px] xl:items-start">
          <div className="grid gap-6">
            {loading ? (
              <SectionCard>
                <SkeletonCard />
              </SectionCard>
            ) : !nowReading ? (
              <SectionCard className="flex min-h-[420px] items-center justify-center">
                <EmptyState
                  icon={Sparkles}
                  title={t("emptyState.briefing.title")}
                  description={t("emptyState.briefing.desc")}
                />
              </SectionCard>
            ) : (
              <SectionCard className="overflow-hidden p-0">
                <div className="grid min-w-0 max-w-full gap-0 bg-[radial-gradient(circle_at_top_left,_rgba(143,61,37,0.08),_transparent_36%),linear-gradient(135deg,_#ffffff_0%,_#f7f3ec_52%,_#ffffff_100%)] lg:grid-cols-[minmax(0,1fr)_320px]">
                  <div className="flex min-w-0 max-w-full flex-col justify-between gap-6 p-6 sm:p-7">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <Tag tone="accent">{t("briefing.highlightBadge")} 1</Tag>
                        <Tag>{nowReading.is_read ? t("items.read.read") : t("items.read.unread")}</Tag>
                        <span className="font-sans text-[12px] font-medium text-[var(--color-editorial-ink-faint)]">
                          {fmtDate(nowReading.published_at || nowReading.created_at, locale)}
                        </span>
                      </div>
                      <button
                        type="button"
                        onClick={() => setInlineItemId(nowReading.id)}
                        className="mt-5 block w-full text-left"
                      >
                        <h2 className="line-clamp-4 min-w-0 max-w-full break-all break-words font-serif text-[2rem] leading-[1.18] tracking-[-0.03em] text-[var(--color-editorial-ink)] hover:underline">
                          {nowReading.translated_title || nowReading.title || nowReading.url}
                        </h2>
                      </button>
                      <p className="mt-4 line-clamp-4 min-w-0 max-w-full break-all break-words font-sans text-[14px] leading-7 text-[var(--color-editorial-ink-soft)]">
                        {nowReading.content_text?.trim() || nowReading.recommendation_reason || nowReading.url}
                      </p>
                    </div>
                    <div>
                      <div className="flex flex-wrap gap-x-4 gap-y-2 font-sans text-[11px] font-semibold uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                        <span className="min-w-0 max-w-full break-all break-words">{nowReading.source_title || "Source"}</span>
                        <span className="min-w-0 max-w-full break-all break-words">{fmtDate(nowReading.published_at || nowReading.created_at, locale)}</span>
                      </div>
                      <div className="mt-4 flex flex-wrap items-center gap-2">
                        <button
                          type="button"
                          onClick={() => setInlineItemId(nowReading.id)}
                          className="inline-flex min-h-11 items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 press focus-ring"
                        >
                          <BookOpen className="size-4" aria-hidden="true" />
                          {t("briefing.readNow", "今すぐ読む")}
                        </button>
                        <Link
                          href={`/items/${nowReading.id}?from=${encodeURIComponent("/items?feed=unread&sort=newest")}`}
                          className="inline-flex min-h-11 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring"
                        >
                          {t("items.action.openDetail")}
                          <ArrowRight className="size-4" aria-hidden="true" />
                        </Link>
                      </div>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => setInlineItemId(nowReading.id)}
                    className="group relative min-h-[260px] min-w-0 max-w-full overflow-hidden border-t border-[var(--color-editorial-line)] lg:border-l lg:border-t-0"
                  >
                    <ThumbnailArtwork item={nowReading} className="h-full min-h-[260px] w-full" />
                    <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/35 via-black/10 to-transparent opacity-75 transition-opacity group-hover:opacity-100" />
                  </button>
                </div>
              </SectionCard>
            )}

            <SectionCard>
              <div className="flex items-end justify-between gap-3">
                <div>
                  <div className="font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("briefing.nowReadingLabel", "NOW READING")}
                  </div>
                  <h2 className="mt-2 font-serif text-[1.6rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                    {t("briefing.nextReads", "Next Up")}
                  </h2>
                </div>
                <Link href="/items?feed=unread&sort=newest" className="text-[12px] font-semibold text-[var(--color-editorial-ink-faint)] hover:text-[var(--color-editorial-ink)]">
                  {t("briefing.openRecommended")}
                </Link>
              </div>
              {loading ? (
                <div className="mt-4 grid gap-3">
                  {Array.from({ length: 3 }).map((_, i) => (
                    <SkeletonCard key={i} />
                  ))}
                </div>
              ) : nextReads.length === 0 ? (
                <p className="mt-4 text-sm text-[var(--color-editorial-ink-soft)]">{t("briefing.emptyHighlights", "次に読む記事はありません。")}</p>
              ) : (
                <ul className="mt-5 grid gap-4">
                  {nextReads.slice(0, 4).map((item) => (
                    <li key={item.id}>
                      <button
                        type="button"
                        onClick={() => setInlineItemId(item.id)}
                        className="grid w-full gap-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4 text-left hover:bg-[var(--color-editorial-panel)] md:grid-cols-[122px_minmax(0,1fr)]"
                      >
                        <ThumbnailArtwork item={item} className="h-[180px] w-full rounded-[14px] md:h-[92px]" />
                        <div className="min-w-0 max-w-full">
                          <h3 className="line-clamp-2 min-w-0 max-w-full break-all break-words font-serif text-[1.05rem] font-semibold leading-[1.35] text-[var(--color-editorial-ink)]">
                            {item.translated_title || item.title || item.url}
                          </h3>
                          <p className="mt-2 line-clamp-2 min-w-0 max-w-full break-all break-words font-sans text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                            {item.content_text?.trim() || item.recommendation_reason || item.url}
                          </p>
                          <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 font-sans text-[11px] font-semibold uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                            <span className="min-w-0 max-w-full break-all break-words">{item.source_title || "Source"}</span>
                            <span className="min-w-0 max-w-full break-all break-words">{fmtDate(item.published_at || item.created_at, locale)}</span>
                          </div>
                        </div>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </SectionCard>
          </div>

          <aside className="grid gap-6 self-start">
            <SectionCard>
              <div className="font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                Today Queue
              </div>
              <h2 className="mt-2 font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                {t("briefing.todayQueue.title")}
              </h2>
              <p className="mt-2 font-sans text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                {t("briefing.todayQueue.subtitle")}
              </p>
              {todayQueue.length === 0 ? (
                <p className="mt-4 text-sm text-[var(--color-editorial-ink-soft)]">{t("briefing.emptyHighlights", "次に読む記事はありません。")}</p>
              ) : (
                <div className="mt-4 grid gap-3">
                  {todayQueue.slice(0, 3).map((entry, index) => (
                    <article key={entry.item.id} className="overflow-hidden rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                      <button type="button" onClick={() => setInlineItemId(entry.item.id)} className="block w-full text-left">
                        <ThumbnailArtwork item={entry.item} className="h-[168px] w-full" />
                        <div className="p-4">
                          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                            <span>{t("briefing.todayQueue.rank")} {index + 1}</span>
                            <span>{entry.estimated_reading_minutes}{t("briefing.todayQueue.minutes")}</span>
                          </div>
                          <h3 className="mt-2 line-clamp-2 break-words font-serif text-[1rem] font-semibold leading-[1.35] text-[var(--color-editorial-ink)] hover:underline">
                            {entry.item.translated_title || entry.item.title || entry.item.url}
                          </h3>
                          <p className="mt-2 line-clamp-3 font-sans text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
                            {entry.reason_labels.map((reason) => {
                              if (reason === "priority goal") return t("briefing.todayQueue.reason.goal");
                              if (reason === "fresh") return t("briefing.todayQueue.reason.fresh");
                              if (reason === "attention") return t("briefing.todayQueue.reason.attention");
                              return reason;
                            }).join(" / ")}
                          </p>
                          <div className="mt-3 flex flex-wrap gap-2">
                        {(entry.matched_goals ?? []).slice(0, 2).map((goal) => (
                          <Tag key={goal.id} tone="warning">{goal.title}</Tag>
                        ))}
                        <Tag>{entry.estimated_reading_minutes}{t("briefing.todayQueue.minutes")}</Tag>
                          </div>
                        </div>
                      </button>
                    </article>
                  ))}
                </div>
              )}
            </SectionCard>

            <SectionCard>
              <div className="font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                Cluster Watch
              </div>
              <h2 className="mt-2 font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                あとで見るテーマ
              </h2>
              {clusterRows.length === 0 ? (
                <p className="mt-4 text-sm text-[var(--color-editorial-ink-soft)]">{t("briefing.emptyClusters")}</p>
              ) : (
                <div className="mt-4 grid gap-3">
                  {clusterRows.slice(0, 2).map((cluster) => (
                    <article key={cluster.id} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                      <h3 className="font-serif text-[1rem] font-semibold leading-[1.35] text-[var(--color-editorial-ink)]">
                        {cluster.label || t("briefing.clusterFallback")}
                      </h3>
                      {cluster.summary ? (
                        <p className="mt-2 font-sans text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{cluster.summary}</p>
                      ) : null}
                      <div className="mt-3 flex flex-wrap gap-2">
                        <Tag>{cluster.items.length} items</Tag>
                        {cluster.topItems[0]?.source_title ? <Tag>{cluster.topItems[0].source_title}</Tag> : null}
                      </div>
                    </article>
                  ))}
                </div>
              )}
              <div className="mt-4 flex justify-end">
                <Link href="/clusters" className="text-[12px] font-semibold text-[var(--color-editorial-ink-faint)] hover:text-[var(--color-editorial-ink)]">
                  {t("briefing.openClusters")}
                </Link>
              </div>
            </SectionCard>
          </aside>
        </div>

        <ReadingGoalsPanel goals={activeGoals} />

        <DueReviewPanel
          items={reviewQueue}
          onOpen={setInlineItemId}
          onDone={(id) => {
            void markReviewDone(id);
          }}
          onSnooze={(id) => {
            void snoozeReview(id);
          }}
        />

        <WeeklyReviewPanel review={weeklyReview} />

        {inlineItemId && (
          <InlineReader
            open={!!inlineItemId}
            itemId={inlineItemId}
            locale={locale}
            queueItemIds={nowQueueIds}
            onClose={() => setInlineItemId(null)}
            onOpenDetail={(itemId) => {
              router.push(`/items/${itemId}?from=${encodeURIComponent("/items?feed=unread&sort=newest")}`);
            }}
            onOpenItem={(itemId) => setInlineItemId(itemId)}
            onReadToggled={() => {
              void refreshBriefingData();
            }}
          />
        )}

        {navigatorLoading && !navigatorClosed && !navigatorPreview ? (
          <aside className="fixed right-4 z-40 bottom-[calc(5rem+env(safe-area-inset-bottom))] md:bottom-4">
            <div className="flex items-end gap-3">
              <div className="flex size-14 shrink-0 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] shadow-[0_16px_36px_rgba(15,23,42,0.2)]">
                <AINavigatorAvatar persona={navigatorLoadingPersona} className="size-12" />
              </div>
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 shadow-[0_16px_36px_rgba(15,23,42,0.14)]">
                <div className="font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                  {t("briefing.navigator.label", "AI Navigator")}
                </div>
                <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">
                  {t("briefing.navigator.loading", "Generating picks...")}
                </p>
              </div>
            </div>
          </aside>
        ) : null}
        {navigator && !navigatorClosed ? (
          <aside className="fixed right-4 z-40 w-[min(420px,calc(100vw-1.5rem))] bottom-[calc(5rem+env(safe-area-inset-bottom))] md:bottom-4">
            <div className={`flex max-h-[min(80vh,720px)] flex-col overflow-hidden rounded-[24px] border shadow-[0_24px_60px_rgba(15,23,42,0.22)] ${navigatorTheme?.shell ?? "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]"}`}>
              <div className={`flex items-start gap-3 px-4 py-4 ${navigatorTheme?.header ?? ""}`}>
                <div className={`flex size-11 shrink-0 items-center justify-center rounded-full ${navigatorTheme?.avatar ?? ""}`}>
                  <AINavigatorAvatar persona={navigatorDisplayPersona} className="size-11" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="font-sans text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("briefing.navigator.label", "AI Navigator")}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
                    <h3 className="font-serif text-[1.1rem] font-semibold leading-none text-[var(--color-editorial-ink)]">
                      {navigator.character_name}
                    </h3>
                    <span className="font-sans text-xs text-[var(--color-editorial-ink-soft)]">{navigator.character_title}</span>
                  </div>
                </div>
                {navigatorPreview ? null : (
                  <button
                    type="button"
                    onClick={() => setNavigatorDismissed(true)}
                    className="inline-flex size-9 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white/70 text-[var(--color-editorial-ink-soft)] hover:bg-white"
                    aria-label={t("briefing.navigator.close", "Close navigator")}
                  >
                    <X className="size-4" aria-hidden="true" />
                  </button>
                )}
              </div>
              <div className="space-y-4 overflow-y-auto px-4 pb-4">
                <div className={`rounded-[18px] border px-4 py-3 ${navigatorTheme?.bubble ?? "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)]"}`}>
                  <p className="text-sm leading-7 text-[var(--color-editorial-ink)]">{navigator.intro}</p>
                </div>
                {navigator.picks.length > 0 ? (
                  <div className="space-y-3">
                    {navigator.picks.map((pick) => (
                      <button
                        key={pick.item_id}
                        type="button"
                        onClick={() => {
                          if (navigatorPreview && pick.item_id.startsWith("preview-")) {
                            return;
                          }
                          setInlineItemId(pick.item_id);
                        }}
                        className="block w-full rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-left hover:bg-[var(--color-editorial-panel-strong)]"
                      >
                        <div className="flex items-start gap-3">
                          <div className={`mt-0.5 inline-flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold ${navigatorTheme?.badge ?? ""}`}>
                            {pick.rank}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                              <h4 className="line-clamp-2 font-serif text-[1rem] font-semibold leading-[1.35] text-[var(--color-editorial-ink)]">
                                {pick.title}
                              </h4>
                              {pick.source_title ? (
                                <span className="font-sans text-[11px] font-semibold uppercase tracking-[0.08em] text-[var(--color-editorial-ink-faint)]">
                                  {pick.source_title}
                                </span>
                              ) : null}
                            </div>
                            <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{pick.comment}</p>
                            {pick.reason_tags && pick.reason_tags.length > 0 ? (
                              <div className="mt-3 flex flex-wrap gap-2">
                                {pick.reason_tags.map((tag) => (
                                  <Tag key={`${pick.item_id}-${tag}`}>{tag}</Tag>
                                ))}
                              </div>
                            ) : null}
                          </div>
                        </div>
                      </button>
                    ))}
                  </div>
                ) : (
                  <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3">
                    <p className="text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                      {t("briefing.navigator.empty", "There are no fresh unread stories right now. Check back a little later.")}
                    </p>
                  </div>
                )}
              </div>
            </div>
          </aside>
        ) : null}
        {navigator && navigatorClosed && !navigatorPreview ? (
          <button
            type="button"
            onClick={() => setNavigatorDismissed(false)}
            className={`fixed right-4 z-40 inline-flex size-14 items-center justify-center rounded-full border shadow-[0_16px_36px_rgba(15,23,42,0.2)] transition hover:scale-[1.03] bottom-[calc(5rem+env(safe-area-inset-bottom))] md:bottom-4 ${navigatorTheme?.shell ?? "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]"}`}
            aria-label={t("briefing.navigator.label", "AI Navigator")}
          >
            <AINavigatorAvatar persona={navigatorDisplayPersona} className="size-12" />
          </button>
        ) : null}
      </div>
    </PageTransition>
  );
}

function ThumbnailArtwork({ item, className }: { item: Item; className?: string }) {
  if (item.thumbnail_url) {
    return (
      <div className={`relative overflow-hidden bg-zinc-100 ${className ?? ""}`}>
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={item.thumbnail_url}
          alt={item.translated_title || item.title || item.url}
          className="h-full w-full object-cover"
          loading="lazy"
        />
      </div>
    );
  }
  return (
    <div className={`relative overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.2),_transparent_35%),linear-gradient(135deg,_#18181b_0%,_#3f3f46_45%,_#d4d4d8_100%)] ${className ?? ""}`}>
      <div className="absolute inset-0 bg-[linear-gradient(135deg,transparent_0%,rgba(255,255,255,0.12)_45%,transparent_100%)]" />
      <div className="absolute inset-x-3 bottom-3 min-w-0 max-w-full break-all break-words text-left text-xs font-medium text-white/90">
        <div className="line-clamp-3">{item.translated_title || item.title || item.url}</div>
      </div>
    </div>
  );
}

function fmtDate(value: string | null | undefined, locale: "ja" | "en") {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US");
}

function navigatorThemeTokens(persona: string, avatarStyle?: string) {
  const key = avatarStyle || persona;
  switch (key) {
    case "hype":
      return {
        shell: "border-[#f0b677] bg-[linear-gradient(180deg,#fff6e9_0%,#fffdf8_100%)]",
        header: "",
        avatar: "bg-[#d96c28] text-white",
        bubble: "border-[#f0b677] bg-[#fff0da]",
        badge: "bg-[#d96c28] text-white",
      };
    case "analyst":
      return {
        shell: "border-[#9db5d5] bg-[linear-gradient(180deg,#eef4fb_0%,#fbfdff_100%)]",
        header: "",
        avatar: "bg-[#365f93] text-white",
        bubble: "border-[#c8d8ec] bg-[#f3f8fd]",
        badge: "bg-[#365f93] text-white",
      };
    case "concierge":
      return {
        shell: "border-[#d9c7b2] bg-[linear-gradient(180deg,#fbf5ef_0%,#fffdfb_100%)]",
        header: "",
        avatar: "bg-[#8c6a52] text-white",
        bubble: "border-[#e7d8c8] bg-[#fff8f1]",
        badge: "bg-[#8c6a52] text-white",
      };
    case "snark":
      return {
        shell: "border-[#caa8a8] bg-[linear-gradient(180deg,#f9eeee_0%,#fffdfd_100%)]",
        header: "",
        avatar: "bg-[#7d3f3f] text-white",
        bubble: "border-[#dfc2c2] bg-[#fff5f5]",
        badge: "bg-[#7d3f3f] text-white",
      };
    case "native":
      return {
        shell: "border-[#efb2c6] bg-[linear-gradient(180deg,#fff0f6_0%,#fffdfd_100%)]",
        header: "",
        avatar: "bg-[#d24f7a] text-white",
        bubble: "border-[#f3c8d7] bg-[#fff5f8]",
        badge: "bg-[#d24f7a] text-white",
      };
    case "junior":
      return {
        shell: "border-[#e5b3d3] bg-[linear-gradient(180deg,#fff4fb_0%,#fffdfd_100%)]",
        header: "",
        avatar: "bg-[#c26aa3] text-white",
        bubble: "border-[#efd1e4] bg-[#fff7fb]",
        badge: "bg-[#c26aa3] text-white",
      };
    case "urban":
      return {
        shell: "border-[#c1cad6] bg-[linear-gradient(180deg,#f4f7fb_0%,#fffdfd_100%)]",
        header: "",
        avatar: "bg-[#59667a] text-white",
        bubble: "border-[#d6dde6] bg-[#f8fafc]",
        badge: "bg-[#59667a] text-white",
      };
    default:
      return {
        shell: "border-[#c7b79c] bg-[linear-gradient(180deg,#f8f3e7_0%,#fffdf8_100%)]",
        header: "",
        avatar: "bg-[#8f5a24] text-white",
        bubble: "border-[#ddcfb7] bg-[#fff8ea]",
        badge: "bg-[#8f5a24] text-white",
      };
  }
}
