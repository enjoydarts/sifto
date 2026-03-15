"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Bell, BookOpen, Flame, Sparkles, X } from "lucide-react";
import { api, BriefingCluster, Item, ProviderModelChangeEvent, ReadingGoal, ReviewQueueItem, TodayQueueItem, WeeklyReviewSnapshot } from "@/lib/api";
import { ReadingGoalsPanel } from "@/components/briefing/reading-goals-panel";
import { TodayQueue } from "@/components/briefing/today-queue";
import { DueReviewPanel } from "@/components/reviews/due-review-panel";
import { WeeklyReviewPanel } from "@/components/reviews/weekly-review-panel";
import { InlineReader } from "@/components/inline-reader";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { EmptyState } from "@/components/empty-state";
import { SkeletonCard } from "@/components/skeleton";
import { useToast } from "@/components/toast-provider";

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
  const queryClient = useQueryClient();
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [savingClusterId, setSavingClusterId] = useState<string | null>(null);
  const [dismissedModelUpdatesAt, setDismissedModelUpdatesAt] = useState<string | null>(null);
  const [hydrated, setHydrated] = useState(false);
  const briefingQuery = useQuery({
    queryKey: ["briefing-today", 18] as const,
    queryFn: () => api.getBriefingToday({ size: 18 }),
    staleTime: 15_000,
    refetchInterval: 60_000,
    refetchIntervalInBackground: false,
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
  const settingsQuery = useQuery({
    queryKey: ["settings"] as const,
    queryFn: api.getSettings,
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
  const visibleModelUpdates = useMemo(() => {
    if (!dismissedModelUpdatesAt) return modelUpdates;
    const dismissedMs = Date.parse(dismissedModelUpdatesAt);
    if (Number.isNaN(dismissedMs)) return modelUpdates;
    return modelUpdates.filter((event) => Date.parse(event.detected_at) > dismissedMs);
  }, [dismissedModelUpdatesAt, modelUpdates]);
  const highlights = data?.highlight_items ?? EMPTY_ITEMS;
  const activeGoals = readingGoalsQuery.data?.active ?? EMPTY_GOALS;
  const readingPlanPrefs = settingsQuery.data?.reading_plan;
  const focusWindow = readingPlanPrefs?.window ?? "24h";
  const focusSize = readingPlanPrefs?.size ?? 15;
  const diversifyTopics = Boolean(readingPlanPrefs?.diversify_topics ?? true);
  const quickTriageQuery = useQuery({
    queryKey: ["triage-queue", "quick", focusWindow, focusSize, diversifyTopics ? 1 : 0] as const,
    queryFn: () =>
      api.getFocusQueue({
        window: focusWindow === "today_jst" || focusWindow === "7d" ? focusWindow : "24h",
        size: focusSize,
        diversify_topics: diversifyTopics,
        exclude_later: true,
      }),
    staleTime: 30_000,
    placeholderData: (prev) => prev,
  });
  const todayQueue = todayQueueQuery.data?.items ?? EMPTY_TODAY_QUEUE;
  const reviewQueue = reviewQueueQuery.data?.items ?? EMPTY_REVIEW_QUEUE;
  const weeklyReview = (weeklyReviewQuery.data ?? null) as WeeklyReviewSnapshot | null;
  const quickTriageCount = useMemo(
    () => (quickTriageQuery.data?.items ?? EMPTY_ITEMS).filter((item) => !item.is_read).length,
    [quickTriageQuery.data?.items]
  );
  const clusters = data?.clusters ?? EMPTY_CLUSTERS;
  const unreadHighlights = useMemo(() => highlights.filter((item) => !item.is_read), [highlights]);
  const nowReading = unreadHighlights[0] ?? highlights[0] ?? null;
  const topHighlightCards = useMemo(() => {
    if (!nowReading) return highlights.slice(0, 4);
    return highlights.filter((item) => item.id !== nowReading.id).slice(0, 4);
  }, [highlights, nowReading]);
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

  useEffect(() => {
    if (typeof window === "undefined") return;
    setHydrated(true);
    setDismissedModelUpdatesAt(window.localStorage.getItem(MODEL_UPDATES_DISMISSED_AT_KEY));
  }, []);

  const isRefreshing = hydrated && briefingQuery.isFetching;
  const refreshBriefingData = async () => {
    const next = await api.getBriefingToday({ size: 18, cache_bust: true });
    queryClient.setQueryData(["briefing-today", 18], next);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["today-queue"] }),
      queryClient.invalidateQueries({ queryKey: ["triage-queue"] }),
      queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
    ]);
  };

  const saveClusterForLater = async (cluster: BriefingCluster) => {
    const itemIds = Array.from(new Set((cluster.items ?? EMPTY_ITEMS).map((item) => item.id).filter(Boolean)));
    if (itemIds.length === 0) {
      showToast(t("briefing.clusterLaterEmpty"), "info");
      return;
    }
    setSavingClusterId(cluster.id);
    try {
      const res = await api.markItemsLaterBulk({ item_ids: itemIds });
      if (res.updated_count <= 0) {
        showToast(t("briefing.clusterLaterEmpty"), "info");
        return;
      }
      showToast(`${res.updated_count}${locale === "ja" ? "" : " "}${t("briefing.clusterLaterDone")}`, "success");
      await refreshBriefingData();
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setSavingClusterId(null);
    }
  };
  const markClusterRead = async (cluster: BriefingCluster) => {
    const itemIds = Array.from(
      new Set((cluster.items ?? EMPTY_ITEMS).filter((item) => !item.is_read).map((item) => item.id).filter(Boolean))
    );
    if (itemIds.length === 0) {
      showToast(t("briefing.clusterReadEmpty"), "info");
      return;
    }
    setSavingClusterId(`read:${cluster.id}`);
    try {
      const res = await api.markItemsReadByIDs(itemIds);
      if (res.updated_count <= 0) {
        showToast(t("briefing.clusterReadEmpty"), "info");
        return;
      }
      showToast(`${res.updated_count}${locale === "ja" ? "" : " "}${t("briefing.clusterReadDone")}`, "success");
      await refreshBriefingData();
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setSavingClusterId(null);
    }
  };

  const markTodayQueueRead = async (itemId: string) => {
    try {
      await api.markItemRead(itemId);
      await refreshBriefingData();
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
  };

  const saveTodayQueueForLater = async (itemId: string) => {
    try {
      await api.markItemLater(itemId);
      await refreshBriefingData();
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
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
        <section className="rounded-2xl border border-amber-200 bg-amber-50/70 p-4 shadow-sm">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2">
              <Bell className="size-4 shrink-0 text-amber-700" aria-hidden="true" />
              <div className="min-w-0">
                <h2 className="text-sm font-semibold text-amber-900">{t("briefing.providerModelUpdates")}</h2>
                <p className="text-sm text-amber-800">
                  {visibleModelUpdates.slice(0, 3).map((event) => `${event.provider} ${event.change_type === "added" ? "+" : "-"} ${event.model_id}`).join(" / ")}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={dismissModelUpdates}
                className="inline-flex items-center rounded-lg border border-amber-300 bg-white px-3 py-2 text-sm font-medium text-amber-900 hover:bg-amber-100 press focus-ring"
              >
                <X className="mr-1 size-4" aria-hidden="true" />
                {t("briefing.providerModelUpdatesDismiss")}
              </button>
              <Link
                href="/settings"
                className="inline-flex items-center rounded-lg border border-amber-300 bg-white px-3 py-2 text-sm font-medium text-amber-900 hover:bg-amber-100 press focus-ring"
              >
                {t("briefing.providerModelUpdatesOpen")}
              </Link>
            </div>
          </div>
        </section>
        )}

        <section className="rounded-2xl border border-zinc-200 bg-white p-6 shadow-sm">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h1 className="inline-flex items-center gap-2 text-2xl font-bold tracking-tight text-zinc-900">
                <Sparkles className="size-6 text-blue-600" aria-hidden="true" />
                <span>{t("briefing.title")}</span>
              </h1>
              {briefingDateLabel ? (
                <p className="mt-1 text-sm text-zinc-600">{briefingDateLabel}</p>
              ) : null}
              <p className="mt-1 text-sm text-zinc-600">
                {`${greetingLabel}、${greetingSummary}。`}
              </p>
              <p className="mt-1 text-sm text-zinc-500">
                {greetingTail}
              </p>
              {generatedAtLabel ? (
                <p className="mt-1 text-xs text-zinc-500">
                  {t("briefing.generatedAt")} {generatedAtLabel}
                  {data?.status === "stale" ? ` · ${t("briefing.statusStale")}` : ""}
                </p>
              ) : null}
            </div>
            <div className="flex flex-col items-start gap-2 sm:items-end">
              {data?.stats.streak_days ? (
                <div className="text-right">
                  <div className="inline-flex items-center gap-1 text-sm font-medium text-zinc-700">
                    <Flame className="size-4 text-amber-500" aria-hidden="true" />
                    <span>{data.stats.streak_days}{t("briefing.kpi.streakUnit")}</span>
                  </div>
                  {data.stats.streak_at_risk ? (
                    <p className="mt-1 text-xs text-rose-700">
                      {t("briefing.streakRisk.prefix")}
                      {data?.stats.streak_remaining ?? 0}
                      {t("briefing.streakRisk.unit")}
                      {t("briefing.streakRisk.suffix")}
                    </p>
                  ) : null}
                </div>
              ) : null}
              <button
                type="button"
                onClick={() => {
                  void api
                    .getBriefingToday({ size: 18, cache_bust: true })
                    .then((next) => {
                      queryClient.setQueryData(["briefing-today", 18], next);
                    })
                    .catch(() => {
                      // keep current snapshot on refresh failure
                    });
                }}
                disabled={isRefreshing}
                className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50 press focus-ring"
              >
                {isRefreshing ? t("briefing.refreshing") : t("common.refresh")}
              </button>
            </div>
          </div>

          {error && <p className="mt-4 text-sm text-red-500">{error}</p>}

          <div className="mt-5 grid gap-3 md:grid-cols-3">
            <Link
              href="/items?feed=unread&sort=newest"
              className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4 transition-colors hover:border-zinc-300 hover:bg-white"
            >
              <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
                {t("briefing.hub.unread")}
              </p>
              <p className="mt-2 text-2xl font-semibold text-zinc-900">
                {String(data?.stats.total_unread ?? 0)}
              </p>
              <p className="mt-1 text-sm text-zinc-500">{t("briefing.hub.openInbox")}</p>
            </Link>
            <Link
              href="/items?feed=later&sort=newest"
              className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4 transition-colors hover:border-zinc-300 hover:bg-white"
            >
              <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
                {t("briefing.hub.queue")}
              </p>
              <p className="mt-2 text-2xl font-semibold text-zinc-900">
                {String(nextReads.length)}
              </p>
              <p className="mt-1 text-sm text-zinc-500">{t("briefing.hub.openQueue")}</p>
            </Link>
            <Link
              href="/triage"
              className="rounded-2xl border border-zinc-900 bg-zinc-900 p-4 text-white transition-colors hover:bg-zinc-800"
            >
              <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-300">
                {t("briefing.hub.triage")}
              </p>
              <p className="mt-2 text-2xl font-semibold">
                {String(quickTriageCount)}
              </p>
              <p className="mt-1 text-sm text-zinc-300">{t("briefing.hub.start")}</p>
            </Link>
          </div>

          <div className="mt-5">
            <p className="text-xs font-medium uppercase tracking-[0.12em] text-zinc-500">
              {t("briefing.nowReadingLabel", "NOW READING")}
            </p>
          </div>
          {!loading && topHighlightCards.length > 0 ? (
            <div className="mt-3">
              <div className="mb-2 flex items-center justify-between gap-2">
                <h2 className="text-sm font-semibold text-zinc-900">{t("briefing.highlights")}</h2>
              </div>
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                {topHighlightCards.map((item, idx) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => setInlineItemId(item.id)}
                    className="overflow-hidden rounded-[16px] border border-zinc-200 bg-white text-left shadow-[0_4px_12px_rgba(0,0,0,0.06)] hover:border-zinc-300 hover:bg-zinc-50"
                  >
                    <ThumbnailArtwork item={item} className="h-40 w-full" />
                    <div className="p-5">
                      <div className="flex items-center justify-between gap-2">
                        <span className="rounded-full bg-amber-100 px-2 py-1 text-[11px] font-semibold text-amber-800">
                          {t("briefing.highlightBadge")} {idx + 1}
                        </span>
                        <span className="text-xs text-zinc-500">{fmtDate(item.published_at || item.created_at, locale)}</span>
                      </div>
                      <div className="mt-3 line-clamp-3 break-words [overflow-wrap:anywhere] text-[15px] font-semibold text-zinc-900">
                        {item.translated_title || item.title || item.url}
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            </div>
          ) : null}
          {loading ? (
            <div className="mt-3">
              <SkeletonCard />
            </div>
          ) : !nowReading ? (
            <EmptyState
              icon={Sparkles}
              title={t("emptyState.briefing.title")}
              description={t("emptyState.briefing.desc")}
            />
          ) : (
            <article className="mt-3 overflow-hidden rounded-[20px] border border-zinc-200 bg-white shadow-[0_4px_16px_rgba(0,0,0,0.04),0_0_0_1px_rgba(0,0,0,0.02)]">
              <div className="grid gap-0 bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.08),_transparent_34%),linear-gradient(135deg,_#ffffff_0%,_#f8fafc_52%,_#ffffff_100%)] md:grid-cols-[1fr_1.2fr]">
                <div className="flex flex-col justify-between p-[28px]">
                  <div>
                    <div className="flex flex-wrap items-center gap-2 text-xs text-zinc-500">
                      <span className="rounded-full bg-blue-100 px-2.5 py-1 font-semibold text-blue-700">
                        {t("briefing.highlightBadge")} 1
                      </span>
                      <span className="rounded-full border border-zinc-300 bg-white px-2 py-0.5">
                        {nowReading.is_read ? t("items.read.read") : t("items.read.unread")}
                      </span>
                      <span>{fmtDate(nowReading.published_at || nowReading.created_at, locale)}</span>
                    </div>
                    <button
                      type="button"
                      onClick={() => setInlineItemId(nowReading.id)}
                      className="mt-4 block w-full text-left"
                    >
                      <h2 className="line-clamp-4 break-words [overflow-wrap:anywhere] text-[28px] font-bold leading-[1.25] text-zinc-950 hover:underline md:text-[2rem]">
                        {nowReading.translated_title || nowReading.title || nowReading.url}
                      </h2>
                    </button>
                    <p className="mt-3 line-clamp-2 break-all text-sm text-zinc-500" title={nowReading.url}>
                      {nowReading.url}
                    </p>
                  </div>
                  <div className="mt-5 flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      onClick={() => setInlineItemId(nowReading.id)}
                      className="inline-flex items-center gap-1 rounded-[12px] bg-zinc-950 px-[24px] py-[12px] text-[15px] font-semibold text-white hover:bg-zinc-800 press focus-ring"
                    >
                      <BookOpen className="size-4" aria-hidden="true" />
                      {t("briefing.readNow", "今すぐ読む")}
                    </button>
                    <Link
                      href={`/items/${nowReading.id}?from=${encodeURIComponent("/items?feed=unread&sort=newest")}`}
                      className="inline-flex items-center gap-1 rounded-[12px] border border-zinc-300 bg-white px-[24px] py-[12px] text-[15px] font-semibold text-zinc-700 hover:bg-zinc-50 press focus-ring"
                    >
                      {t("items.action.openDetail")}
                      <ArrowRight className="size-4" aria-hidden="true" />
                    </Link>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => setInlineItemId(nowReading.id)}
                  className="group relative min-h-[240px] overflow-hidden border-t border-zinc-200 md:min-h-full md:border-l md:border-t-0"
                >
                  <ThumbnailArtwork item={nowReading} className="h-full min-h-[240px] w-full md:min-h-[280px]" />
                  <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/35 via-black/5 to-transparent opacity-80 transition-opacity group-hover:opacity-100" />
                </button>
              </div>
            </article>
          )}

          <div className="mt-5 flex items-center justify-between gap-2">
            <h2 className="text-sm font-semibold text-zinc-900">{t("briefing.nextReads", "Next Up")}</h2>
            <Link href="/items?feed=unread&sort=newest" className="text-xs text-zinc-500 hover:text-zinc-900 transition-colors">
              {t("briefing.openRecommended")}
            </Link>
          </div>
          {loading ? (
            <div className="mt-3 grid gap-3 md:grid-cols-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <SkeletonCard key={i} />
              ))}
            </div>
          ) : nextReads.length === 0 ? (
            <p className="mt-2 text-sm text-zinc-500">{t("briefing.emptyHighlights", "次に読む記事はありません。")}</p>
          ) : (
            <ul className="mt-3 space-y-3">
              {nextReads.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    onClick={() => setInlineItemId(item.id)}
                    className="grid w-full grid-cols-[120px_1fr] gap-3 rounded-[16px] border border-zinc-200 bg-white p-4 text-left shadow-[0_2px_8px_rgba(0,0,0,0.04)] hover:border-zinc-300 hover:bg-zinc-50"
                  >
                    <ThumbnailArtwork item={item} className="h-20 w-full rounded-xl" />
                    <div className="min-w-0">
                      <div className="line-clamp-2 break-words [overflow-wrap:anywhere] text-[15px] font-semibold text-zinc-900">
                        {item.translated_title || item.title || item.url}
                      </div>
                      <div className="mt-1 flex items-center gap-2 text-xs text-zinc-500">
                        <span>{item.is_read ? t("items.read.read") : t("items.read.unread")}</span>
                        <span>{fmtDate(item.published_at || item.created_at, locale)}</span>
                      </div>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <ReadingGoalsPanel goals={activeGoals} />

        <TodayQueue
          items={todayQueue}
          onOpen={setInlineItemId}
          onRead={(itemId) => {
            void markTodayQueueRead(itemId);
          }}
          onLater={(itemId) => {
            void saveTodayQueueForLater(itemId);
          }}
        />

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

        <section className="rounded-2xl border border-zinc-200 bg-white p-6 shadow-sm">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-sm font-semibold text-zinc-900">{t("briefing.clusters")}</h2>
            <div className="flex items-center gap-3">
              <Link href="/clusters" className="text-xs text-zinc-500 hover:text-zinc-900 transition-colors">
                {t("briefing.openClusters")}
              </Link>
              <Link href="/items?feed=unread&sort=newest" className="text-xs text-zinc-500 hover:text-zinc-900 transition-colors">
                {t("briefing.openRecommended")}
              </Link>
            </div>
          </div>
          {loading ? (
            <div className="mt-3 grid gap-3 md:grid-cols-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <SkeletonCard key={i} />
              ))}
            </div>
          ) : clusterRows.length === 0 ? (
            <p className="mt-2 text-sm text-zinc-500">{t("briefing.emptyClusters")}</p>
          ) : (
            <div className="mt-3 grid gap-4 xl:grid-cols-2">
              {clusterRows.slice(0, 6).map((cluster) => (
                <section key={cluster.id} className="rounded-2xl border border-zinc-200 bg-zinc-50/50 p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <h3 className="truncate text-base font-semibold text-zinc-900">
                        {cluster.label || t("briefing.clusterFallback")}
                      </h3>
                      {cluster.summary ? (
                        <p className="mt-1 line-clamp-2 text-sm text-zinc-600">{cluster.summary}</p>
                      ) : null}
                    </div>
                    <span className="rounded-full border border-zinc-300 bg-white px-2 py-1 text-xs text-zinc-600">
                      {cluster.items.length}
                    </span>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <button
                      type="button"
                      onClick={() => void saveClusterForLater(cluster)}
                      disabled={savingClusterId === cluster.id || savingClusterId === `read:${cluster.id}`}
                      className="inline-flex items-center rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50 press focus-ring disabled:cursor-wait disabled:opacity-60"
                    >
                      {savingClusterId === cluster.id ? t("briefing.clusterLaterSaving") : t("briefing.clusterLater")}
                    </button>
                    <button
                      type="button"
                      onClick={() => void markClusterRead(cluster)}
                      disabled={savingClusterId === cluster.id || savingClusterId === `read:${cluster.id}`}
                      className="inline-flex items-center rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50 press focus-ring disabled:cursor-wait disabled:opacity-60"
                    >
                      {savingClusterId === `read:${cluster.id}` ? t("briefing.clusterReadSaving") : t("briefing.clusterRead")}
                    </button>
                  </div>
                  <ul className="mt-3 space-y-2">
                    {cluster.topItems.map((item) => (
                      <li key={item.id}>
                        <button
                          type="button"
                          onClick={() => setInlineItemId(item.id)}
                          className="grid w-full grid-cols-[100px_1fr] gap-3 rounded-xl border border-zinc-200 bg-white p-3 text-left hover:border-zinc-300 hover:bg-zinc-50"
                        >
                          <ThumbnailArtwork item={item} className="h-20 w-full" />
                          <div className="min-w-0">
                            <div className="line-clamp-2 break-words [overflow-wrap:anywhere] text-sm font-medium text-zinc-900">
                              {item.translated_title || item.title || item.url}
                            </div>
                            <div className="mt-1 text-xs text-zinc-500">
                              {fmtDate(item.published_at || item.created_at, locale)}
                            </div>
                          </div>
                        </button>
                      </li>
                    ))}
                  </ul>
                </section>
              ))}
            </div>
          )}
        </section>

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
      <div className="absolute bottom-3 left-3 right-3 line-clamp-3 text-left text-xs font-medium text-white/90">
        {item.translated_title || item.title || item.url}
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
