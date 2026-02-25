"use client";

import { Suspense, useState, useEffect, useCallback, useMemo, useRef } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Image as ImageIcon, Newspaper, Star, ThumbsDown, ThumbsUp } from "lucide-react";
import { api, Item, ReadingPlanResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import Pagination from "@/components/pagination";
import { useToast } from "@/components/toast-provider";

const FILTERS = ["", "summarized", "new", "fetched", "facts_extracted", "failed"] as const;
type SortMode = "newest" | "score";
type FocusSize = 7 | 15 | 25;
type FocusWindow = "24h" | "today_jst" | "7d";
type FeedMode = "recommended" | "all";
type ItemsFeedQueryData = {
  items: Item[];
  total: number;
  planPoolCount: number;
  planClusters?: ReadingPlanResponse["clusters"];
};

function scoreTone(score: number) {
  if (score >= 0.8) return "bg-green-50 text-green-700 border-green-200";
  if (score >= 0.65) return "bg-blue-50 text-blue-700 border-blue-200";
  if (score >= 0.5) return "bg-zinc-50 text-zinc-700 border-zinc-200";
  return "bg-amber-50 text-amber-700 border-amber-200";
}

function ItemsPageContent() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryState = useMemo(() => {
    const qFeed = searchParams.get("feed");
    const feedMode: FeedMode = qFeed === "all" ? "all" : "recommended";

    const qSort = searchParams.get("sort");
    const sortMode: SortMode = qSort === "score" ? "score" : "newest";

    const qFilter = searchParams.get("status");
    const filter =
      qFilter && FILTERS.includes(qFilter as (typeof FILTERS)[number]) ? qFilter : "";
    const topic = (searchParams.get("topic") ?? "").trim();

    const unreadOnly = searchParams.get("unread") === "1";
    const favoriteOnly = searchParams.get("favorite") === "1";

    const qPage = Number(searchParams.get("page"));
    const page = Number.isFinite(qPage) && qPage >= 1 ? Math.floor(qPage) : 1;

    return { feedMode, sortMode, filter, topic, unreadOnly, favoriteOnly, page };
  }, [searchParams]);
  const { feedMode, sortMode, filter, topic, unreadOnly, favoriteOnly, page } = queryState;
  const focusMode = feedMode === "recommended";
  const pageSize = 20;
  const [error, setError] = useState<string | null>(null);
  const [retryingIds, setRetryingIds] = useState<Record<string, boolean>>({});
  const [readUpdatingIds, setReadUpdatingIds] = useState<Record<string, boolean>>({});
  const restoredScrollRef = useRef<string | null>(null);
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
      page,
      sortMode,
      unreadOnly ? 1 : 0,
      favoriteOnly ? 1 : 0,
      focusWindow,
      focusSize,
      diversifyTopics ? 1 : 0,
    ] as const,
    [diversifyTopics, favoriteOnly, feedMode, filter, focusSize, focusWindow, page, sortMode, topic, unreadOnly]
  );

  const listQuery = useQuery<ItemsFeedQueryData>({
    queryKey: listQueryKey,
    queryFn: async () => {
      if (feedMode === "recommended") {
        const data = await api.getReadingPlan({
          window: focusWindow,
          size: focusSize,
          diversify_topics: diversifyTopics,
          exclude_read: false,
        });
        return {
          items: data?.items ?? [],
          total: data?.items?.length ?? 0,
          planPoolCount: data?.source_pool_count ?? 0,
          planClusters: data?.clusters ?? [],
        };
      }
      const data = await api.getItems({
        ...(filter ? { status: filter } : {}),
        ...(topic ? { topic } : {}),
        page,
        page_size: pageSize,
        sort: sortMode,
        unread_only: unreadOnly,
        favorite_only: favoriteOnly,
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
      const nextUnread = patch.unread ?? unreadOnly;
      const nextFavorite = patch.favorite ?? favoriteOnly;
      const nextPage = patch.page ?? page;

      if (nextFeed === "all") {
        if (nextStatus) q.set("status", nextStatus);
        else q.delete("status");
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
    [favoriteOnly, feedMode, filter, page, pathname, router, searchParams, sortMode, topic, unreadOnly]
  );

  const itemsQueryString = useMemo(() => {
    const q = new URLSearchParams();
    q.set("feed", feedMode);
    if (!focusMode) {
      if (filter) q.set("status", filter);
      if (topic) q.set("topic", topic);
      q.set("sort", sortMode);
      if (page > 1) q.set("page", String(page));
      if (unreadOnly) q.set("unread", "1");
      if (favoriteOnly) q.set("favorite", "1");
    }
    return q.toString();
  }, [favoriteOnly, feedMode, filter, focusMode, page, sortMode, topic, unreadOnly]);

  const currentItemsHref = useMemo(
    () => (itemsQueryString ? `${pathname}?${itemsQueryString}` : pathname),
    [itemsQueryString, pathname]
  );

  const scrollStorageKey = useMemo(() => `items-scroll:${currentItemsHref}`, [currentItemsHref]);
  const lastItemStorageKey = useMemo(() => `items-last-item:${currentItemsHref}`, [currentItemsHref]);

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
        showToast(locale === "ja" ? "再試行をキュー投入しました" : "Retry queued", "success");
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
    [locale, queryClient, showToast, t]
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
              }
            : prev
        );
        if (item.is_read) {
          await api.markItemUnread(item.id);
          showToast(locale === "ja" ? "未読に戻しました" : "Marked as unread", "success");
        } else {
          await api.markItemRead(item.id);
          showToast(locale === "ja" ? "既読にしました" : "Marked as read", "success");
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
    [listQueryKey, locale, queryClient, showToast, t]
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

  const renderItemRow = useCallback((item: Item, opts?: { featured?: boolean; rank?: number }) => {
    const featured = Boolean(opts?.featured);
    const rank = opts?.rank ?? 0;
    const href = detailHref(item.id);
    const openDetail = () => {
      rememberScroll(item.id);
      router.push(href);
    };
    return (
      <div data-item-row-id={item.id}>
        <div
          role="link"
          tabIndex={0}
          onClick={openDetail}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              openDetail();
            }
          }}
          className={`group ${featured ? "flex flex-col gap-3 sm:flex-row sm:items-start" : "flex items-stretch gap-3"} rounded-xl px-4 py-3.5 transition-all ${
            featured
              ? item.is_read
                ? "cursor-pointer border border-zinc-300 bg-gradient-to-b from-zinc-200 to-zinc-100 shadow-sm hover:border-zinc-400 hover:shadow-md"
                : "cursor-pointer border border-zinc-200 bg-white shadow-sm hover:border-zinc-300 hover:shadow-md"
              : item.is_read
                ? "cursor-pointer border border-zinc-300 bg-zinc-200/80 shadow-sm hover:border-zinc-400"
                : "cursor-pointer border border-zinc-200 bg-white shadow-sm hover:border-zinc-300"
          }`}
        >
          <div className={`min-w-0 flex-1 transition-colors group-hover:text-zinc-700 ${featured ? "flex flex-col gap-3 sm:flex-row sm:items-start" : "flex items-stretch gap-3"}`}>
            <div
              className={`shrink-0 overflow-hidden rounded-lg border border-zinc-200 bg-zinc-50 ${
                featured ? "hidden h-[104px] w-[136px] sm:flex" : "hidden h-[72px] w-[72px] sm:flex"
              }`}
            >
              {item.thumbnail_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={item.thumbnail_url}
                  alt=""
                  loading="lazy"
                  referrerPolicy="no-referrer"
                  className="h-full w-full object-cover"
                />
              ) : (
                <div className="flex h-full w-full items-center justify-center text-zinc-300">
                  <ImageIcon className={featured ? "size-5" : "size-4"} aria-hidden="true" />
                </div>
              )}
            </div>
            <div className={`flex min-w-0 flex-1 flex-col ${featured ? "justify-start gap-2 py-0.5" : "justify-between gap-1.5 py-0.5"}`}>
              <div className={`${featured ? "space-y-2" : "flex items-start gap-2"}`}>
                <div className="min-w-0 flex-1">
                  {featured && rank > 0 && (
                    <div className="mb-1 inline-flex items-center gap-1 rounded-full bg-zinc-900 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-white">
                      {locale === "ja" ? "PICK" : "PICK"} #{rank}
                    </div>
                  )}
                  <div
                    className={`overflow-hidden font-semibold ${
                      featured
                        ? item.is_read ? "text-base leading-6 text-zinc-700" : "text-[17px] leading-6 text-zinc-950"
                        : item.is_read ? "text-[15px] leading-6 text-zinc-600" : "text-[15px] leading-6 text-zinc-900"
                    }`}
                  >
                    {item.title ?? item.url}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-zinc-400">
                    <span
                      className={`rounded-full border px-2 py-0.5 text-[11px] font-semibold ${
                        item.is_read
                          ? "border-zinc-300 bg-zinc-50 text-zinc-700"
                          : "border-zinc-200 bg-white text-zinc-700"
                      }`}
                    >
                      {item.is_read ? (locale === "ja" ? "既読" : "Read") : (locale === "ja" ? "未読" : "Unread")}
                    </span>
                    <span>{new Date(item.published_at ?? item.created_at).toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US")}</span>
                    {item.is_favorite && (
                      <span className="inline-flex items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-700">
                        <Star className="size-3 fill-current" aria-hidden="true" />
                        {locale === "ja" ? "お気に入り" : "Favorite"}
                      </span>
                    )}
                    {item.feedback_rating === 1 && (
                      <span className="inline-flex items-center gap-1 rounded-full border border-green-200 bg-green-50 px-2 py-0.5 text-[11px] font-semibold text-green-700">
                        <ThumbsUp className="size-3" aria-hidden="true" />
                        {locale === "ja" ? "良い" : "Like"}
                      </span>
                    )}
                    {item.feedback_rating === -1 && (
                      <span className="inline-flex items-center gap-1 rounded-full border border-rose-200 bg-rose-50 px-2 py-0.5 text-[11px] font-semibold text-rose-700">
                        <ThumbsDown className="size-3" aria-hidden="true" />
                        {locale === "ja" ? "微妙" : "Dislike"}
                      </span>
                    )}
                    {featured &&
                      (item.summary_score != null ? (
                        <span
                          className={`rounded border px-2 py-0.5 text-xs font-semibold ${scoreTone(item.summary_score)}`}
                          title={locale === "ja" ? "要約スコア" : "Summary score"}
                        >
                          {item.summary_score.toFixed(2)}
                        </span>
                      ) : (
                        <span className="rounded border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-xs font-medium text-zinc-400">
                          {locale === "ja" ? "未採点" : "N/A"}
                        </span>
                      ))}
                  </div>
                </div>
                {!featured && (
                  <div>
                    {item.summary_score != null ? (
                      <span
                        className={`shrink-0 rounded border px-2 py-0.5 text-xs font-semibold ${scoreTone(item.summary_score)}`}
                        title={locale === "ja" ? "要約スコア" : "Summary score"}
                      >
                        {item.summary_score.toFixed(2)}
                      </span>
                    ) : (
                      <span className="shrink-0 rounded border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-xs font-medium text-zinc-400">
                        {locale === "ja" ? "未採点" : "N/A"}
                      </span>
                    )}
                  </div>
                )}
              </div>
              <div className={`${featured ? "h-4 truncate text-[12px] text-zinc-500" : "h-4 truncate text-[12px] text-zinc-400"}`}>
                {item.title ? item.url : "\u00A0"}
              </div>
              {featured && (
                <div className="flex flex-wrap items-center gap-2 pt-1">
                  <button
                    type="button"
                    disabled={!!readUpdatingIds[item.id]}
                    onClick={(e) => {
                      e.stopPropagation();
                      void toggleRead(item);
                    }}
                    className="rounded border border-zinc-300 bg-white px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {readUpdatingIds[item.id]
                      ? locale === "ja"
                        ? "更新中..."
                        : "Updating..."
                      : item.is_read
                        ? locale === "ja"
                          ? "未読に戻す"
                          : "Mark unread"
                        : locale === "ja"
                          ? "既読にする"
                          : "Mark read"}
                  </button>
                  {item.status === "failed" && (
                    <button
                      type="button"
                      disabled={!!retryingIds[item.id]}
                      onClick={(e) => {
                        e.stopPropagation();
                        void retryItem(item.id);
                      }}
                      className="rounded border border-zinc-300 bg-white px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {retryingIds[item.id] ? t("items.retrying") : t("items.retry")}
                    </button>
                  )}
                </div>
              )}
            </div>
          </div>
          {!featured && (
            <div className="flex min-h-[72px] shrink-0 flex-col items-end justify-between gap-2">
            <button
              type="button"
              disabled={!!readUpdatingIds[item.id]}
              onClick={(e) => {
                e.stopPropagation();
                void toggleRead(item);
              }}
              className={`rounded border border-zinc-300 bg-white px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50 ${
                featured ? "flex-1 sm:flex-none" : ""
              }`}
            >
              {readUpdatingIds[item.id]
                ? locale === "ja"
                  ? "更新中..."
                  : "Updating..."
                : item.is_read
                  ? locale === "ja"
                    ? "未読に戻す"
                    : "Mark unread"
                  : locale === "ja"
                    ? "既読にする"
                    : "Mark read"}
            </button>
            {item.status === "failed" ? (
              <button
                type="button"
                disabled={!!retryingIds[item.id]}
                onClick={(e) => {
                  e.stopPropagation();
                  void retryItem(item.id);
                }}
                className={`rounded border border-zinc-300 bg-white px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50 ${
                  featured ? "flex-1 sm:flex-none" : ""
                }`}
              >
                {retryingIds[item.id] ? t("items.retrying") : t("items.retry")}
              </button>
            ) : (
              !featured && (
                <span className="invisible rounded border border-zinc-300 px-3 py-1 text-xs font-medium">
                  _
                </span>
              )
            )}
            </div>
          )}
        </div>
      </div>
    );
  }, [detailHref, locale, readUpdatingIds, rememberScroll, retryItem, retryingIds, router, t, toggleRead]);

  return (
    <div className={`space-y-4 ${focusMode ? "pb-8" : ""}`}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1 rounded-lg border border-zinc-200 bg-white p-1">
            <button
              type="button"
              onClick={() => replaceItemsQuery({ feed: "recommended" })}
              className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
                focusMode ? "bg-zinc-900 text-white" : "text-zinc-600 hover:bg-zinc-50"
              }`}
            >
              {locale === "ja" ? "おすすめ" : "Recommended"}
            </button>
            <button
              type="button"
              onClick={() => replaceItemsQuery({ feed: "all", page: 1 })}
              className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
                !focusMode ? "bg-zinc-900 text-white" : "text-zinc-600 hover:bg-zinc-50"
              }`}
            >
              {locale === "ja" ? "すべて" : "All"}
            </button>
          </div>
          {focusMode && (
            <Link
              href="/settings"
              className="rounded-lg border border-zinc-200 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50"
            >
              {locale === "ja" ? "おすすめ設定" : "Feed settings"}
            </Link>
          )}
        </div>
        {!focusMode && (
          <div className="flex items-center gap-1 rounded-lg border border-zinc-200 bg-white p-1">
            <button
              type="button"
              onClick={() => replaceItemsQuery({ sort: "newest", page: 1 })}
              className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
                sortMode === "newest"
                  ? "bg-zinc-900 text-white"
                  : "text-zinc-600 hover:bg-zinc-50"
              }`}
            >
              {locale === "ja" ? "新着順" : "Newest"}
            </button>
            <button
              type="button"
              onClick={() => replaceItemsQuery({ sort: "score", page: 1 })}
              className={`rounded px-3 py-1.5 text-xs font-medium transition-colors ${
                sortMode === "score"
                  ? "bg-zinc-900 text-white"
                  : "text-zinc-600 hover:bg-zinc-50"
              }`}
            >
              {locale === "ja" ? "スコア順" : "Score"}
            </button>
          </div>
        )}
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
                {locale === "ja" ? `（トピック: ${topic}）` : `(topic: ${topic})`}
              </span>
            )}
            {focusMode && (
              <span className="ml-2 text-zinc-400">
                {locale === "ja"
                  ? `（${displayItems.length.toLocaleString()}件を選定 / 対象 ${planPoolCount.toLocaleString()} 件）`
                  : `(${displayItems.length.toLocaleString()} selected / ${planPoolCount.toLocaleString()} in window)`}
              </span>
            )}
          </p>
        </div>
      </div>

      {/* Filters */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        {!focusMode && topic && (
          <div className="inline-flex items-center gap-2 rounded border border-blue-200 bg-blue-50 px-3 py-1 text-sm text-blue-800">
            <span className="font-medium">
              {locale === "ja" ? "トピック" : "Topic"}: {topic}
            </span>
            <button
              type="button"
              onClick={() => replaceItemsQuery({ topic: "", page: 1 })}
              className="rounded px-1.5 py-0.5 text-xs text-blue-700 hover:bg-blue-100"
            >
              {locale === "ja" ? "解除" : "Clear"}
            </button>
          </div>
        )}
        {!focusMode && (
          <label className="inline-flex items-center gap-2 rounded border border-zinc-200 bg-white px-3 py-1 text-sm text-zinc-700">
            <input
              type="checkbox"
              checked={unreadOnly}
              onChange={(e) => replaceItemsQuery({ unread: e.target.checked, page: 1 })}
              className="size-4 rounded border-zinc-300"
            />
            {locale === "ja" ? "未読のみ" : "Unread only"}
          </label>
        )}
        {!focusMode && (
          <label className="inline-flex items-center gap-2 rounded border border-zinc-200 bg-white px-3 py-1 text-sm text-zinc-700">
            <input
              type="checkbox"
              checked={favoriteOnly}
              onChange={(e) => replaceItemsQuery({ favorite: e.target.checked, page: 1 })}
              className="size-4 rounded border-zinc-300"
            />
            <span className="inline-flex items-center gap-1">
              <Star className="size-3.5 text-amber-500" aria-hidden="true" />
              {locale === "ja" ? "お気に入りのみ" : "Favorites only"}
            </span>
          </label>
        )}
      </div>

      {/* State */}
      {loading && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {visibleError && <p className="text-sm text-red-500">{visibleError}</p>}
      {!loading && items.length === 0 && (
        <p className="text-sm text-zinc-400">{t("items.empty")}</p>
      )}

      {/* List */}
      {focusMode && featuredItems.length > 0 && (
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-amber-700">
                {locale === "ja" ? "今日のピックアップ" : "Today's Picks"}
              </div>
              <div className="text-sm text-zinc-500">
                {locale === "ja" ? "まず読む価値が高い記事" : "Start here for high-value reads"}
              </div>
            </div>
          </div>
          <ul className="grid list-none gap-3 lg:grid-cols-2">
            {featuredItems.map((item, idx) => (
              <li key={item.id} className={`${idx === 0 ? "lg:col-span-2" : ""} list-none`}>
                {renderItemRow(item, { featured: true, rank: idx + 1 })}
              </li>
            ))}
          </ul>
        </section>
      )}
      {focusMode && visibleClusterSections.length > 0 && (
        <section className="space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
                {locale === "ja" ? "話題ごとに読む" : "Read by Topic"}
              </div>
              <div className="text-sm text-zinc-500">
                {locale === "ja" ? "関連記事がまとまっている話題" : "Clusters of related stories"}
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
                        {locale === "ja" ? `${section.items.length}本` : `${section.items.length} stories`}
                      </span>
                    </div>
                  </div>
                  <ul className="list-none space-y-2">
                    <li className="list-none">{renderItemRow(hero, { featured: true })}</li>
                    {rest.length > 0 && (
                      <li className="list-none">
                        <ul className="list-none space-y-2 pt-1">
                          {rest.map((item) => (
                            <li key={item.id} className="list-none">
                              {renderItemRow(item)}
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
      {focusMode && recommendedLooseItems.length > 0 && (
        <div className={`${featuredItems.length > 0 || visibleClusterSections.length > 0 ? "pt-2" : ""}`}>
          <div className="mb-2 flex items-center justify-between">
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
                {locale === "ja" ? "その他のおすすめ" : "More Picks"}
              </div>
              <div className="text-sm text-zinc-500">
                {locale === "ja" ? "単発で読んでおきたい記事" : "Good standalone reads"}
              </div>
            </div>
          </div>
        </div>
      )}
      <ul className={`list-none space-y-2 ${focusMode && (featuredItems.length > 0 || visibleClusterSections.length > 0) ? "pt-1" : ""}`}>
        {recommendedLooseItems.map((item) => (
          <li key={item.id} className="list-none">
            {renderItemRow(item)}
          </li>
        ))}
      </ul>
      {!focusMode && (
        <Pagination
          total={itemsTotal}
          page={page}
          pageSize={pageSize}
          onPageChange={(nextPage) => replaceItemsQuery({ page: nextPage })}
        />
      )}
    </div>
  );
}

export default function ItemsPage() {
  return (
    <Suspense fallback={<p className="text-sm text-zinc-500">Loading...</p>}>
      <ItemsPageContent />
    </Suspense>
  );
}
