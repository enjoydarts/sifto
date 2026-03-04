"use client";

import { Suspense, useState, useEffect, useCallback, useMemo, useRef } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Newspaper, Star } from "lucide-react";
import { api, Item, ReadingPlanResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import Pagination from "@/components/pagination";
import { useToast } from "@/components/toast-provider";
import { InlineReader } from "@/components/inline-reader";
import { PageTransition } from "@/components/page-transition";
import { EmptyState } from "@/components/empty-state";
import { SkeletonItemRow } from "@/components/skeleton";
import { ItemCard } from "@/components/items/item-card";
import { FeedTabs, type FeedMode, type SortMode } from "@/components/items/feed-tabs";

const FILTERS = ["", "summarized", "new", "fetched", "facts_extracted", "failed"] as const;
type FocusSize = 7 | 15 | 25;
type FocusWindow = "24h" | "today_jst" | "7d";
type ItemsFeedQueryData = {
  items: Item[];
  total: number;
  planPoolCount: number;
  planClusters?: ReadingPlanResponse["clusters"];
  focusCompleted?: number;
  focusRemaining?: number;
};

function ItemsPageContent() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryState = useMemo(() => {
    const qFeed = searchParams.get("feed");
    const feedMode: FeedMode = qFeed === "all" ? "all" : qFeed === "later" ? "later" : "recommended";

    const qSort = searchParams.get("sort");
    const sortMode: SortMode = qSort === "score" ? "score" : "newest";

    const qFilter = searchParams.get("status");
    const filter =
      qFilter && FILTERS.includes(qFilter as (typeof FILTERS)[number]) ? qFilter : "";
    const topic = (searchParams.get("topic") ?? "").trim();
    const sourceID = (searchParams.get("source_id") ?? "").trim();

    const unreadOnly = searchParams.get("unread") === "1";
    const favoriteOnly = searchParams.get("favorite") === "1";

    const qPage = Number(searchParams.get("page"));
    const page = Number.isFinite(qPage) && qPage >= 1 ? Math.floor(qPage) : 1;

    return { feedMode, sortMode, filter, topic, sourceID, unreadOnly, favoriteOnly, page };
  }, [searchParams]);
  const { feedMode, sortMode, filter, topic, sourceID, unreadOnly, favoriteOnly, page } = queryState;
  const focusMode = feedMode === "recommended";
  const laterMode = feedMode === "later";
  const pageSize = 20;
  const [error, setError] = useState<string | null>(null);
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [retryingIds, setRetryingIds] = useState<Record<string, boolean>>({});
  const [readUpdatingIds, setReadUpdatingIds] = useState<Record<string, boolean>>({});
  const restoredScrollRef = useRef<string | null>(null);
  const prefetchedDetailIDsRef = useRef<Record<string, true>>({});
  const settingsQuery = useQuery({
    queryKey: ["settings"],
    queryFn: api.getSettings,
  });

  const readingPlanPrefs = settingsQuery.data?.reading_plan;
  const focusWindow: FocusWindow =
    readingPlanPrefs?.window === "today_jst" || readingPlanPrefs?.window === "7d" || readingPlanPrefs?.window === "24h"
      ? readingPlanPrefs.window
      : "24h";
  const focusSize: FocusSize =
    readingPlanPrefs?.size === 7 || readingPlanPrefs?.size === 15 || readingPlanPrefs?.size === 25
      ? readingPlanPrefs.size
      : 15;
  const diversifyTopics = Boolean(readingPlanPrefs?.diversify_topics ?? true);

  const listQueryKey = useMemo(
    () => [
      "items-feed",
      feedMode,
      filter,
      topic,
      sourceID,
      page,
      sortMode,
      unreadOnly ? 1 : 0,
      favoriteOnly ? 1 : 0,
      focusWindow,
      focusSize,
      diversifyTopics ? 1 : 0,
    ] as const,
    [diversifyTopics, favoriteOnly, feedMode, filter, focusSize, focusWindow, page, sortMode, sourceID, topic, unreadOnly]
  );

  const listQuery = useQuery<ItemsFeedQueryData>({
    queryKey: listQueryKey,
    queryFn: async () => {
      if (feedMode === "recommended") {
        const data = await api.getFocusQueue({
          window: focusWindow,
          size: focusSize,
          diversify_topics: diversifyTopics,
          exclude_later: true,
        });
        return {
          items: data?.items ?? [],
          total: data?.items?.length ?? 0,
          planPoolCount: data?.source_pool ?? 0,
          planClusters: [],
          focusCompleted: data?.completed ?? 0,
          focusRemaining: data?.remaining ?? 0,
        };
      }
      const data = await api.getItems({
        ...(filter ? { status: filter } : {}),
        ...(sourceID ? { source_id: sourceID } : {}),
        ...(topic ? { topic } : {}),
        page,
        page_size: pageSize,
        sort: sortMode,
        unread_only: unreadOnly,
        favorite_only: favoriteOnly,
        later_only: laterMode,
      });
      return {
        items: data?.items ?? [],
        total: data?.total ?? 0,
        planPoolCount: 0,
        planClusters: [],
      };
    },
    placeholderData: (prev) => prev,
  });
  const cachedItemsLength = listQuery.data?.items?.length ?? 0;
  const items = listQuery.data?.items ?? [];
  const itemsTotal = listQuery.data?.total ?? 0;
  const planPoolCount = listQuery.data?.planPoolCount ?? 0;
  const loading = !listQuery.data && (listQuery.isLoading || listQuery.isFetching);
  const queryError = listQuery.error ? String(listQuery.error) : null;
  const visibleError = error ?? queryError;

  const replaceItemsQuery = useCallback(
    (
        patch: Partial<{
          feed: FeedMode;
          sort: SortMode;
          status: string;
          topic: string;
          sourceId: string;
          unread: boolean;
          favorite: boolean;
          page: number;
      }>
    ) => {
      const q = new URLSearchParams(searchParams.toString());

      const nextFeed = patch.feed ?? feedMode;
      q.set("feed", nextFeed);

      const nextSort = patch.sort ?? sortMode;
      const nextStatus = patch.status ?? filter;
      const nextTopic = patch.topic ?? topic;
      const nextSourceID = patch.sourceId ?? sourceID;
      const nextUnread = patch.unread ?? unreadOnly;
      const nextFavorite = patch.favorite ?? favoriteOnly;
      const nextPage = patch.page ?? page;

      if (nextFeed === "all" || nextFeed === "later") {
        if (nextStatus) q.set("status", nextStatus);
        else q.delete("status");
        if (nextSourceID) q.set("source_id", nextSourceID);
        else q.delete("source_id");
        if (nextTopic) q.set("topic", nextTopic);
        else q.delete("topic");
        q.set("sort", nextSort);
        if (nextUnread) q.set("unread", "1");
        else q.delete("unread");
        if (nextFavorite) q.set("favorite", "1");
        else q.delete("favorite");
        if (nextPage > 1) q.set("page", String(nextPage));
        else q.delete("page");
      } else {
        q.delete("status");
        q.delete("source_id");
        q.delete("topic");
        q.delete("sort");
        q.delete("unread");
        q.delete("favorite");
        q.delete("page");
      }

      const nextQuery = q.toString();
      const nextUrl = nextQuery ? `${pathname}?${nextQuery}` : pathname;
      router.replace(nextUrl, { scroll: false });
    },
    [favoriteOnly, feedMode, filter, page, pathname, router, searchParams, sortMode, sourceID, topic, unreadOnly]
  );

  const itemsQueryString = useMemo(() => {
    const q = new URLSearchParams();
    q.set("feed", feedMode);
    if (!focusMode) {
      if (filter) q.set("status", filter);
      if (sourceID) q.set("source_id", sourceID);
      if (topic) q.set("topic", topic);
      q.set("sort", sortMode);
      if (page > 1) q.set("page", String(page));
      if (unreadOnly) q.set("unread", "1");
      if (favoriteOnly) q.set("favorite", "1");
    }
    return q.toString();
  }, [favoriteOnly, feedMode, filter, focusMode, page, sortMode, sourceID, topic, unreadOnly]);

  const currentItemsHref = useMemo(
    () => (itemsQueryString ? `${pathname}?${itemsQueryString}` : pathname),
    [itemsQueryString, pathname]
  );

  const scrollStorageKey = useMemo(() => `items-scroll:${currentItemsHref}`, [currentItemsHref]);
  const lastItemStorageKey = useMemo(() => `items-last-item:${currentItemsHref}`, [currentItemsHref]);
  const queueStorageKey = useMemo(() => `items-queue:${currentItemsHref}`, [currentItemsHref]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const onScroll = () => {
      sessionStorage.setItem(scrollStorageKey, String(window.scrollY));
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, [scrollStorageKey]);

  useEffect(() => {
    if (loading) return;
    if (restoredScrollRef.current === scrollStorageKey) return;
    const raw = sessionStorage.getItem(scrollStorageKey);
    if (!raw) {
      restoredScrollRef.current = scrollStorageKey;
      return;
    }
    const y = Number(raw);
    if (!Number.isFinite(y)) {
      restoredScrollRef.current = scrollStorageKey;
      return;
    }
    let attempts = 0;
    let cancelled = false;
    const targetItemId = sessionStorage.getItem(lastItemStorageKey);
    const restore = () => {
      if (cancelled) return;
      const canReachNow = document.documentElement.scrollHeight - window.innerHeight >= y;
      if (canReachNow) {
        window.scrollTo(0, y);
      }
      const reached = Math.abs(window.scrollY - y) <= 4;
      if (reached || attempts >= 10) {
        if (!reached && targetItemId) {
          const row = document.querySelector<HTMLElement>(`[data-item-row-id="${targetItemId}"]`);
          row?.scrollIntoView({ block: "center" });
        }
        restoredScrollRef.current = scrollStorageKey;
        return;
      }
      attempts += 1;
      window.setTimeout(restore, 50);
    };
    requestAnimationFrame(restore);
    return () => {
      cancelled = true;
    };
  }, [cachedItemsLength, lastItemStorageKey, loading, scrollStorageKey]);

  const retryItem = useCallback(
    async (itemId: string) => {
      setRetryingIds((prev) => ({ ...prev, [itemId]: true }));
      try {
        await api.retryItem(itemId);
        showToast(t("items.toast.retryQueued"), "success");
        await queryClient.invalidateQueries({ queryKey: ["items-feed"] });
      } catch (e) {
        setError(String(e));
        showToast(`${t("common.error")}: ${String(e)}`, "error");
      } finally {
        setRetryingIds((prev) => {
          const next = { ...prev };
          delete next[itemId];
          return next;
        });
      }
    },
    [queryClient, showToast, t]
  );

  const toggleRead = useCallback(
    async (item: Item) => {
      setReadUpdatingIds((prev) => ({ ...prev, [item.id]: true }));
      try {
        queryClient.setQueryData<ItemsFeedQueryData>(listQueryKey, (prev) =>
          prev
            ? {
                ...prev,
                items: prev.items.map((v) => (v.id === item.id ? { ...v, is_read: !item.is_read } : v)),
                planClusters: (prev.planClusters ?? []).map((c) => ({
                  ...c,
                  representative:
                    c.representative?.id === item.id
                      ? { ...c.representative, is_read: !item.is_read }
                      : c.representative,
                  items: (c.items ?? []).map((v) => (v.id === item.id ? { ...v, is_read: !item.is_read } : v)),
                })),
              }
            : prev
        );
        if (item.is_read) {
          await api.markItemUnread(item.id);
          showToast(t("itemDetail.toast.markUnread"), "success");
        } else {
          await api.markItemRead(item.id);
          showToast(t("itemDetail.toast.markRead"), "success");
        }
      } catch (e) {
        queryClient.invalidateQueries({ queryKey: listQueryKey });
        setError(String(e));
        showToast(`${t("common.error")}: ${String(e)}`, "error");
      } finally {
        setReadUpdatingIds((prev) => {
          const next = { ...prev };
          delete next[item.id];
          return next;
        });
      }
    },
    [listQueryKey, queryClient, showToast, t]
  );

  const sortedItems = [...items].sort((a, b) => {
    if (sortMode === "score") {
      const as = a.summary_score ?? -1;
      const bs = b.summary_score ?? -1;
      if (bs !== as) return bs - as;
    }
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });

  const displayItems = focusMode ? items : sortedItems;
  const pagedItems = focusMode ? displayItems : sortedItems;
  const recommendedEmbeddingSections = useMemo(() => {
    if (!focusMode) return [] as Array<{ id: string; topic: string; items: Item[] }>;
    return (listQuery.data?.planClusters ?? [])
      .filter((c) => (c.items?.length ?? 0) >= 2)
      .map((c) => ({
        id: c.id,
        topic: c.label,
        items: c.items ?? [],
      }))
      .filter((s) => s.items.length >= 2);
  }, [focusMode, listQuery.data?.planClusters]);
  const featuredItems = useMemo(() => {
    if (!focusMode) return [] as Item[];
    const fromClusters = recommendedEmbeddingSections.map((section) => section.items[0]).filter(Boolean);
    const picked: Item[] = [];
    const seen = new Set<string>();
    for (const it of fromClusters) {
      if (!it || seen.has(it.id)) continue;
      picked.push(it);
      seen.add(it.id);
      if (picked.length >= 3) return picked;
    }
    for (const it of pagedItems) {
      if (seen.has(it.id)) continue;
      picked.push(it);
      seen.add(it.id);
      if (picked.length >= 3) break;
    }
    return picked;
  }, [focusMode, pagedItems, recommendedEmbeddingSections]);
  const recommendedSectionItemIds = useMemo(
    () => new Set(recommendedEmbeddingSections.flatMap((section) => section.items.map((item) => item.id))),
    [recommendedEmbeddingSections]
  );
  const featuredItemIDs = useMemo(() => new Set(featuredItems.map((it) => it.id)), [featuredItems]);
  const visibleClusterSections = useMemo(() => {
    const seen = new Set<string>(featuredItemIDs);
    return recommendedEmbeddingSections
      .map((section) => {
        const dedupedItems = section.items.filter((item) => {
          if (seen.has(item.id)) return false;
          seen.add(item.id);
          return true;
        });
        return {
          ...section,
          items: dedupedItems,
        };
      })
      .filter((section) => section.items.length >= 1);
  }, [featuredItemIDs, recommendedEmbeddingSections]);
  const recommendedLooseItems = useMemo(
    () => (
      focusMode
        ? pagedItems.filter((item) => !recommendedSectionItemIds.has(item.id) && !featuredItemIDs.has(item.id))
        : pagedItems
    ),
    [featuredItemIDs, focusMode, pagedItems, recommendedSectionItemIds]
  );
  const recommendedRenderedCount = useMemo(() => {
    if (!focusMode) return pagedItems.length;
    const clusteredCount = visibleClusterSections.reduce((acc, section) => acc + section.items.length, 0);
    return featuredItems.length + clusteredCount + recommendedLooseItems.length;
  }, [featuredItems.length, focusMode, pagedItems.length, recommendedLooseItems.length, visibleClusterSections]);
  const detailHref = useCallback(
    (itemId: string) => `/items/${itemId}?from=${encodeURIComponent(currentItemsHref)}`,
    [currentItemsHref]
  );
  const rememberScroll = useCallback((itemId: string) => {
    if (typeof window === "undefined") return;
    sessionStorage.setItem(scrollStorageKey, String(window.scrollY));
    sessionStorage.setItem(lastItemStorageKey, itemId);
  }, [lastItemStorageKey, scrollStorageKey]);
  const saveReadQueue = useCallback((ids: string[]) => {
    if (typeof window === "undefined") return;
    sessionStorage.setItem(queueStorageKey, JSON.stringify(ids));
  }, [queueStorageKey]);
  const prefetchItemDetail = useCallback((itemId: string) => {
    if (prefetchedDetailIDsRef.current[itemId]) return;
    prefetchedDetailIDsRef.current[itemId] = true;
    void queryClient.prefetchQuery({
      queryKey: ["item-detail", itemId],
      queryFn: () => api.getItem(itemId),
      staleTime: 60_000,
    });
    void queryClient.prefetchQuery({
      queryKey: ["item-related", itemId, 6],
      queryFn: () => api.getRelatedItems(itemId, { limit: 6 }),
      staleTime: 60_000,
    });
  }, [queryClient]);

  const renderItem = useCallback((item: Item, opts?: { featured?: boolean; rank?: number; animIdx?: number }) => {
    const featured = Boolean(opts?.featured);
    const href = detailHref(item.id);
    const openDetail = () => {
      rememberScroll(item.id);
      saveReadQueue(focusMode ? displayItems.map((v) => v.id) : sortedItems.map((v) => v.id));
      router.push(href);
    };
    const openInlineReader = () => {
      setInlineItemId(item.id);
      prefetchItemDetail(item.id);
    };
    return (
      <ItemCard
        key={item.id}
        item={item}
        featured={featured}
        rank={opts?.rank}
        locale={locale}
        readUpdating={!!readUpdatingIds[item.id]}
        retrying={!!retryingIds[item.id]}
        onOpen={openInlineReader}
        onOpenDetail={openDetail}
        onToggleRead={() => void toggleRead(item)}
        onRetry={() => void retryItem(item.id)}
        onPrefetch={() => prefetchItemDetail(item.id)}
        animationDelay={(opts?.animIdx ?? 0) * 40}
        t={t}
      />
    );
  }, [detailHref, displayItems, focusMode, locale, prefetchItemDetail, readUpdatingIds, rememberScroll, retryItem, retryingIds, router, saveReadQueue, sortedItems, t, toggleRead]);

  return (
    <PageTransition>
      <div className={`space-y-4 ${focusMode ? "pb-8" : ""}`}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <FeedTabs
            feedMode={feedMode}
            onSelect={(feed) => replaceItemsQuery({ feed, page: 1 })}
            t={t}
          />
        </div>

        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="flex items-center gap-2 text-2xl font-bold">
              <Newspaper className="size-6 text-zinc-500" aria-hidden="true" />
              <span>{t("items.title")}</span>
            </h1>
            <p className="mt-1 text-sm text-zinc-500">
              {(focusMode ? recommendedRenderedCount : itemsTotal).toLocaleString()} {t("common.rows")}
              {!focusMode && topic && (
                <span className="ml-2 text-zinc-400">
                  {`(${t("items.topic")}: ${topic})`}
                </span>
              )}
              {focusMode && (
                <span className="ml-2 text-zinc-400">
                  {locale === "ja"
                    ? `${t("items.recommendedStatOpen")}${displayItems.length.toLocaleString()}${t("common.rows")}${t("items.recommendedStatSelected")}${t("items.recommendedStatTarget")} ${planPoolCount.toLocaleString()} ${t("common.rows")}${t("items.recommendedStatClose")}`
                    : `(${displayItems.length.toLocaleString()} ${t("items.selected")} / ${planPoolCount.toLocaleString()} ${t("items.inWindow")})`}
                </span>
              )}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {!focusMode && (
              <div className="flex items-center gap-1 rounded-lg border border-zinc-200 bg-white p-1">
                {(["newest", "score"] as SortMode[]).map((s) => (
                  <button
                    key={s}
                    type="button"
                    onClick={() => replaceItemsQuery({ sort: s, page: 1 })}
                    className={`rounded px-3 py-1.5 text-xs font-medium transition-colors press focus-ring ${
                      sortMode === s ? "bg-zinc-900 text-white" : "text-zinc-600 hover:bg-zinc-50"
                    }`}
                  >
                    {t(`items.sort.${s}`)}
                  </button>
                ))}
              </div>
            )}
            <button
              type="button"
              onClick={() => router.push("/triage?mode=all")}
              className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 press focus-ring"
            >
              {t("items.openAllTriage")}
            </button>
          </div>
        </div>

        {/* Filters */}
        <div className="mb-4 flex flex-wrap items-center gap-2">
          {!focusMode && topic && (
            <div className="inline-flex items-center gap-2 rounded border border-blue-200 bg-blue-50 px-3 py-1 text-sm text-blue-800">
              <span className="font-medium">
                {t("items.topic")}: {topic}
              </span>
              <button
                type="button"
                onClick={() => replaceItemsQuery({ topic: "", page: 1 })}
                className="rounded px-1.5 py-0.5 text-xs text-blue-700 hover:bg-blue-100 press"
              >
                {t("items.clear")}
              </button>
            </div>
          )}
          {!focusMode && (
            <label className="inline-flex cursor-pointer items-center gap-2 rounded border border-zinc-200 bg-white px-3 py-1 text-sm text-zinc-700 hover:bg-zinc-50 transition-colors">
              <input
                type="checkbox"
                checked={unreadOnly}
                onChange={(e) => replaceItemsQuery({ unread: e.target.checked, page: 1 })}
                className="size-4 rounded border-zinc-300"
              />
              {t("items.filter.unreadOnly")}
            </label>
          )}
          {!focusMode && (
            <label className="inline-flex cursor-pointer items-center gap-2 rounded border border-zinc-200 bg-white px-3 py-1 text-sm text-zinc-700 hover:bg-zinc-50 transition-colors">
              <input
                type="checkbox"
                checked={favoriteOnly}
                onChange={(e) => replaceItemsQuery({ favorite: e.target.checked, page: 1 })}
                className="size-4 rounded border-zinc-300"
              />
              <span className="inline-flex items-center gap-1">
                <Star className="size-3.5 text-amber-500" aria-hidden="true" />
                {t("items.filter.favoriteOnly")}
              </span>
            </label>
          )}
        </div>

        {/* State */}
        {visibleError && <p className="text-sm text-red-500">{visibleError}</p>}

        {loading && (
          <ul className="list-none space-y-2">
            {Array.from({ length: 8 }).map((_, i) => (
              <li key={i} className="list-none">
                <SkeletonItemRow />
              </li>
            ))}
          </ul>
        )}

        {!loading && items.length === 0 && (
          <EmptyState
            icon={Newspaper}
            title={t("emptyState.items.title")}
            description={t("emptyState.items.desc")}
            action={{ label: t("emptyState.items.action"), href: "/sources" }}
          />
        )}

        {/* Featured (Today's Picks) */}
        {!loading && focusMode && featuredItems.length > 0 && (
          <section className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-amber-700">
                  {t("items.section.todayPicks")}
                </div>
                <div className="text-sm text-zinc-500">
                  {t("items.section.todayPicksDesc")}
                </div>
              </div>
            </div>
            <ul className="grid list-none gap-3 lg:grid-cols-2">
              {featuredItems.map((item, idx) => (
                <li key={item.id} className={`${idx === 0 ? "lg:col-span-2" : ""} min-w-0 list-none`}>
                  {renderItem(item, { featured: true, rank: idx + 1, animIdx: idx })}
                </li>
              ))}
            </ul>
          </section>
        )}

        {/* Cluster sections */}
        {!loading && focusMode && visibleClusterSections.length > 0 && (
          <section className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
                  {t("items.section.byTopic")}
                </div>
                <div className="text-sm text-zinc-500">
                  {t("items.section.byTopicDesc")}
                </div>
              </div>
            </div>
            <div className="space-y-4">
              {visibleClusterSections.map((section) => {
                const [hero, ...rest] = section.items;
                if (!hero) return null;
                return (
                  <section key={section.id} className="space-y-2">
                    <div className="flex items-center justify-between gap-2 px-1">
                      <div className="inline-flex items-center gap-2">
                        <span className="rounded-full bg-zinc-100 px-2.5 py-1 text-xs font-semibold text-zinc-800">
                          {section.topic}
                        </span>
                        <span className="text-xs text-zinc-500">
                          {locale === "ja" ? `${section.items.length}${t("items.storyCountJa")}` : `${section.items.length} ${t("items.stories")}`}
                        </span>
                      </div>
                    </div>
                    <ul className="list-none space-y-2">
                      <li className="min-w-0 list-none">{renderItem(hero, { featured: true })}</li>
                      {rest.length > 0 && (
                        <li className="list-none">
                          <ul className="list-none space-y-2 pt-1">
                            {rest.map((item, idx) => (
                              <li key={item.id} className="min-w-0 list-none">
                                {renderItem(item, { animIdx: idx + 1 })}
                              </li>
                            ))}
                          </ul>
                        </li>
                      )}
                    </ul>
                  </section>
                );
              })}
            </div>
          </section>
        )}

        {/* More picks header */}
        {!loading && focusMode && recommendedLooseItems.length > 0 && (
          <div className={`${featuredItems.length > 0 || visibleClusterSections.length > 0 ? "pt-2" : ""}`}>
            <div className="mb-2 flex items-center justify-between">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
                  {t("items.section.morePicks")}
                </div>
                <div className="text-sm text-zinc-500">
                  {t("items.section.morePicksDesc")}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Main list */}
        {!loading && (
          <ul className={`list-none space-y-2 ${focusMode && (featuredItems.length > 0 || visibleClusterSections.length > 0) ? "pt-1" : ""}`}>
            {recommendedLooseItems.map((item, idx) => (
              <li key={item.id} className="min-w-0 list-none">
                {renderItem(item, { animIdx: idx })}
              </li>
            ))}
          </ul>
        )}

        {!focusMode && (
          <Pagination
            total={itemsTotal}
            page={page}
            pageSize={pageSize}
            onPageChange={(nextPage) => replaceItemsQuery({ page: nextPage })}
          />
        )}

        {inlineItemId && (
          <InlineReader
            open={!!inlineItemId}
            itemId={inlineItemId}
            locale={locale}
            onClose={() => setInlineItemId(null)}
            onOpenDetail={(itemId) => {
              setInlineItemId(null);
              rememberScroll(itemId);
              saveReadQueue(
                focusMode
                  ? displayItems.map((v) => v.id)
                  : sortedItems.map((v) => v.id)
              );
              router.push(detailHref(itemId));
            }}
            onOpenItem={(itemId) => setInlineItemId(itemId)}
            onReadToggled={(itemId, isRead) => {
              queryClient.setQueryData<ItemsFeedQueryData>(listQueryKey, (prev) =>
                prev
                  ? {
                      ...prev,
                      items: prev.items.map((v) => (v.id === itemId ? { ...v, is_read: isRead } : v)),
                      planClusters: (prev.planClusters ?? []).map((c) => ({
                        ...c,
                        representative:
                          c.representative?.id === itemId
                            ? { ...c.representative, is_read: isRead }
                            : c.representative,
                        items: (c.items ?? []).map((v) => (v.id === itemId ? { ...v, is_read: isRead } : v)),
                      })),
                    }
                  : prev
              );
            }}
          />
        )}
      </div>
    </PageTransition>
  );
}

export default function ItemsPage() {
  return (
    <Suspense
      fallback={
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <SkeletonItemRow key={i} />
          ))}
        </div>
      }
    >
      <ItemsPageContent />
    </Suspense>
  );
}
