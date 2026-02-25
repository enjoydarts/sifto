"use client";

import { Suspense, useState, useEffect, useCallback, useMemo, useRef } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Image as ImageIcon, Newspaper, Star, ThumbsDown, ThumbsUp } from "lucide-react";
import { api, Item } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import Pagination from "@/components/pagination";
import { useToast } from "@/components/toast-provider";

const STATUS_COLOR: Record<string, string> = {
  new: "bg-zinc-100 text-zinc-600",
  fetched: "bg-blue-50 text-blue-600",
  facts_extracted: "bg-purple-50 text-purple-600",
  summarized: "bg-green-50 text-green-700",
  failed: "bg-red-50 text-red-600",
};

const FILTERS = ["", "summarized", "new", "fetched", "facts_extracted", "failed"] as const;
type SortMode = "newest" | "score";
type FocusSize = 7 | 15 | 25;
type FocusWindow = "24h" | "today_jst" | "7d";
type FeedMode = "recommended" | "all";
type ItemsFeedQueryData = {
  items: Item[];
  total: number;
  planPoolCount: number;
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
  const detailHref = useCallback(
    (itemId: string) => `/items/${itemId}?from=${encodeURIComponent(currentItemsHref)}`,
    [currentItemsHref]
  );
  const rememberScroll = useCallback((itemId: string) => {
    if (typeof window === "undefined") return;
    sessionStorage.setItem(scrollStorageKey, String(window.scrollY));
    sessionStorage.setItem(lastItemStorageKey, itemId);
  }, [lastItemStorageKey, scrollStorageKey]);

  return (
    <div className="space-y-4">
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
            {(focusMode ? displayItems.length : itemsTotal).toLocaleString()} {t("common.rows")}
            {!focusMode && topic && (
              <span className="ml-2 text-zinc-400">
                {locale === "ja" ? `（トピック: ${topic}）` : `(topic: ${topic})`}
              </span>
            )}
            {focusMode && (
              <span className="ml-2 text-zinc-400">
                {locale === "ja"
                  ? `（対象 ${planPoolCount.toLocaleString()} 件から厳選）`
                  : `(selected from ${planPoolCount.toLocaleString()} items in window)`}
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
	      <ul className="space-y-2">
        {pagedItems.map((item) => (
	          <li key={item.id} data-item-row-id={item.id}>
	            <div className={`flex min-h-[92px] items-stretch gap-3 rounded-xl border px-4 py-3.5 shadow-sm transition-colors ${
                item.is_read
                  ? "border-zinc-200 bg-zinc-50/80"
                  : "border-zinc-300 bg-white ring-1 ring-amber-100"
              }`}>
                <span
                  aria-hidden="true"
                  className={`mt-0.5 h-16 w-1 shrink-0 self-start rounded-full ${
                    item.is_read ? "bg-zinc-200" : "bg-amber-400"
                  }`}
                />
	              <Link
	                href={detailHref(item.id)}
                  onClick={() => rememberScroll(item.id)}
	                className="flex min-w-0 flex-1 items-stretch gap-3 transition-colors hover:text-zinc-700"
	              >
                  <div className="hidden h-[72px] w-[72px] shrink-0 overflow-hidden rounded-lg border border-zinc-200 bg-zinc-50 sm:flex">
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
                        <ImageIcon className="size-4" aria-hidden="true" />
                      </div>
                    )}
                  </div>
	                <div className="flex min-w-0 flex-1 flex-col justify-between gap-1.5 py-0.5">
	                  <div className="flex items-start gap-2">
	                    <div className="min-w-0 flex-1">
	                      <div className={`overflow-hidden text-[15px] leading-6 font-semibold ${item.is_read ? "text-zinc-600" : "text-zinc-900"}`}>
	                        {item.title ?? item.url}
	                      </div>
                        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-zinc-400">
                          <span
                            className={`rounded-full border px-2 py-0.5 text-[11px] font-semibold ${
                              item.is_read
                                ? "border-zinc-200 bg-white text-zinc-500"
                                : "border-amber-200 bg-amber-50 text-amber-700"
                            }`}
                          >
                            {item.is_read
                              ? locale === "ja"
                                ? "既読"
                                : "Read"
                              : locale === "ja"
                                ? "未読"
                                : "Unread"}
                          </span>
                          <span
                            className={`rounded border px-1.5 py-0.5 text-[10px] font-medium ${
                              STATUS_COLOR[item.status] ?? "bg-zinc-100 text-zinc-600"
                            }`}
                          >
                            {t(`status.${item.status}`, item.status)}
                          </span>
                          <span>
                            {new Date(
                              item.published_at ?? item.created_at
                            ).toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US")}
                          </span>
                          {item.is_favorite && (
                            <span className="inline-flex items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-700">
                              <Star className="size-3 fill-current" aria-hidden="true" />
                              {locale === "ja" ? "お気に入り" : "Favorite"}
                            </span>
                          )}
                          {item.feedback_rating === 1 && (
                            <span
                              className="inline-flex items-center gap-1 rounded-full border border-green-200 bg-green-50 px-2 py-0.5 text-[11px] font-semibold text-green-700"
                              title={locale === "ja" ? "良い評価" : "Liked"}
                            >
                              <ThumbsUp className="size-3" aria-hidden="true" />
                              {locale === "ja" ? "良い" : "Like"}
                            </span>
                          )}
                          {item.feedback_rating === -1 && (
                            <span
                              className="inline-flex items-center gap-1 rounded-full border border-rose-200 bg-rose-50 px-2 py-0.5 text-[11px] font-semibold text-rose-700"
                              title={locale === "ja" ? "微妙評価" : "Disliked"}
                            >
                              <ThumbsDown className="size-3" aria-hidden="true" />
                              {locale === "ja" ? "微妙" : "Dislike"}
                            </span>
                          )}
                        </div>
                      </div>
	                    {item.summary_score != null ? (
	                      <span
	                        className={`shrink-0 rounded border px-2 py-0.5 text-xs font-semibold ${scoreTone(item.summary_score)}`}
	                        title={locale === "ja" ? "要約スコア" : "Summary score"}
	                      >
	                        {item.summary_score.toFixed(2)}
	                      </span>
	                    ) : (
	                      <span
	                        className="shrink-0 rounded border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-xs font-medium text-zinc-400"
	                        title={locale === "ja" ? "未採点" : "Not scored"}
	                      >
	                        {locale === "ja" ? "未採点" : "N/A"}
	                      </span>
	                    )}
	                  </div>
	                  <div className="h-4 truncate text-[12px] text-zinc-400">
	                    {item.title ? item.url : "\u00A0"}
	                  </div>
	                </div>
	              </Link>
                <div className="flex min-h-[72px] shrink-0 flex-col items-end justify-between gap-2">
                  <button
                    type="button"
                    disabled={!!readUpdatingIds[item.id]}
                    onClick={() => toggleRead(item)}
                    className="rounded border border-zinc-300 px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50"
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
	                    onClick={() => retryItem(item.id)}
	                    className="rounded border border-zinc-300 px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50"
	                  >
	                    {retryingIds[item.id] ? t("items.retrying") : t("items.retry")}
	                  </button>
	                ) : (
                    <span className="invisible rounded border border-zinc-300 px-3 py-1 text-xs font-medium">
                      _
                    </span>
                  )}
                </div>
	            </div>
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
