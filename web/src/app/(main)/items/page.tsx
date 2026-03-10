"use client";

import { Suspense, useState, useEffect, useCallback, useMemo, useRef } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Newspaper, Star } from "lucide-react";
import { api, Item } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import Pagination from "@/components/pagination";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";
import { InlineReader } from "@/components/inline-reader";
import { PageTransition } from "@/components/page-transition";
import { EmptyState } from "@/components/empty-state";
import { SkeletonItemRow } from "@/components/skeleton";
import { ItemCard } from "@/components/items/item-card";
import { FeedTabs, type FeedMode, type SortMode } from "@/components/items/feed-tabs";

const FILTERS = ["", "summarized", "new", "fetched", "facts_extracted", "failed"] as const;
type ItemsFeedQueryData = {
  items: Item[];
  total: number;
};
type FocusQueueData = {
  items: Item[];
};

function ItemsPageContent() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const queryClient = useQueryClient();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryState = useMemo(() => {
    const qFeed = searchParams.get("feed");
    const feedMode: FeedMode =
      qFeed === "later"
        ? "later"
        : qFeed === "read"
          ? "read"
          : "unread";

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
  const unreadMode = feedMode === "unread";
  const readMode = feedMode === "read";
  const laterMode = feedMode === "later";
  const pageSize = 20;
  const [error, setError] = useState<string | null>(null);
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [retryingIds, setRetryingIds] = useState<Record<string, boolean>>({});
  const [readUpdatingIds, setReadUpdatingIds] = useState<Record<string, boolean>>({});
  const [bulkMarkingRead, setBulkMarkingRead] = useState(false);
  const restoredScrollRef = useRef<string | null>(null);
  const prefetchedDetailIDsRef = useRef<Record<string, true>>({});

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
      readMode ? 1 : 0,
    ] as const,
    [favoriteOnly, feedMode, filter, page, readMode, sortMode, sourceID, topic, unreadOnly]
  );

  const listQuery = useQuery<ItemsFeedQueryData>({
    queryKey: listQueryKey,
    queryFn: async () => {
      const data = await api.getItems({
        ...(filter ? { status: filter } : {}),
        ...(sourceID ? { source_id: sourceID } : {}),
        ...(topic ? { topic } : {}),
        page,
        page_size: pageSize,
        sort: sortMode,
        unread_only: unreadMode || unreadOnly,
        read_only: readMode,
        favorite_only: favoriteOnly,
        later_only: laterMode,
      });
      return {
        items: data?.items ?? [],
        total: data?.total ?? 0,
      };
    },
    placeholderData: (prev) => prev,
  });
  const focusQueueQuery = useQuery<FocusQueueData>({
    queryKey: ["focus-queue", "24h", 20],
    queryFn: () => api.getFocusQueue({ window: "24h", size: 20, diversify_topics: true, exclude_later: true }),
    enabled: unreadMode,
    staleTime: 30_000,
  });
  const cachedItemsLength = listQuery.data?.items?.length ?? 0;
  const items = listQuery.data?.items ?? [];
  const itemsTotal = listQuery.data?.total ?? 0;
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

      const nextQuery = q.toString();
      const nextUrl = nextQuery ? `${pathname}?${nextQuery}` : pathname;
      router.replace(nextUrl, { scroll: false });
    },
    [favoriteOnly, feedMode, filter, page, pathname, router, searchParams, sortMode, sourceID, topic, unreadOnly]
  );

  const itemsQueryString = useMemo(() => {
    const q = new URLSearchParams();
    q.set("feed", feedMode);
    if (filter) q.set("status", filter);
    if (sourceID) q.set("source_id", sourceID);
    if (topic) q.set("topic", topic);
    q.set("sort", sortMode);
    if (page > 1) q.set("page", String(page));
    if (unreadOnly) q.set("unread", "1");
    if (favoriteOnly) q.set("favorite", "1");
    return q.toString();
  }, [favoriteOnly, feedMode, filter, page, sortMode, sourceID, topic, unreadOnly]);

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

  const bulkMarkRead = useCallback(
    async (mode: "filtered" | "older_than_7d") => {
      const ok = await confirm({
        title: mode === "filtered" ? t("items.bulkRead.filteredTitle") : t("items.bulkRead.olderTitle"),
        message: mode === "filtered" ? t("items.bulkRead.filteredMessage") : t("items.bulkRead.olderMessage"),
        confirmLabel: t("items.bulkRead.confirm"),
        tone: "danger",
      });
      if (!ok) return;
      setBulkMarkingRead(true);
      try {
        const result = await api.markItemsReadBulk({
          status: filter || null,
          source_id: sourceID || null,
          topic: topic || null,
          unread_only: unreadMode || unreadOnly || mode === "older_than_7d",
          read_only: readMode,
          favorite_only: favoriteOnly,
          later_only: laterMode,
          older_than_days: mode === "older_than_7d" ? 7 : null,
        });
        showToast(`${result.updated_count}${t("items.bulkRead.doneSuffix")}`, "success");
        await queryClient.invalidateQueries({ queryKey: ["items-feed"] });
        await queryClient.invalidateQueries({ queryKey: ["focus-queue"] });
        await queryClient.invalidateQueries({ queryKey: ["briefing-today"] });
      } catch (e) {
        setError(String(e));
        showToast(`${t("common.error")}: ${String(e)}`, "error");
      } finally {
        setBulkMarkingRead(false);
      }
    },
    [confirm, favoriteOnly, filter, laterMode, queryClient, readMode, showToast, sourceID, t, topic, unreadMode, unreadOnly]
  );

  const sortedItems = [...items].sort((a, b) => {
    if (sortMode === "score") {
      const as = a.summary_score ?? -1;
      const bs = b.summary_score ?? -1;
      if (bs !== as) return bs - as;
    }
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });
  const unreadSuggestion = useMemo(() => {
    const fromQueue = (focusQueueQuery.data?.items ?? []).find((item) => !item.is_read) ?? null;
    if (fromQueue) return fromQueue;
    return sortedItems.find((item) => !item.is_read) ?? null;
  }, [focusQueueQuery.data?.items, sortedItems]);
  const dateSections = useMemo(() => {
    const map = new Map<string, Item[]>();
    for (const item of sortedItems) {
      const d = new Date(item.published_at ?? item.created_at);
      const key = Number.isNaN(d.getTime())
        ? (locale === "ja" ? "日付不明" : "Unknown Date")
        : d.toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US", { year: "numeric", month: "2-digit", day: "2-digit" });
      const curr = map.get(key) ?? [];
      curr.push(item);
      map.set(key, curr);
    }
    return Array.from(map.entries()).map(([date, sectionItems]) => ({ date, items: sectionItems }));
  }, [locale, sortedItems]);
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
      saveReadQueue(sortedItems.map((v) => v.id));
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
  }, [detailHref, locale, prefetchItemDetail, readUpdatingIds, rememberScroll, retryItem, retryingIds, router, saveReadQueue, sortedItems, t, toggleRead]);

  return (
    <PageTransition>
      <div className="space-y-4 pb-8">
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
              {itemsTotal.toLocaleString()} {t("common.rows")}
              {topic && (
                <span className="ml-2 text-zinc-400">
                  {`(${t("items.topic")}: ${topic})`}
                </span>
              )}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
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
            <button
              type="button"
              onClick={() => router.push("/triage?mode=all")}
              className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 press focus-ring"
            >
              {t("items.openAllTriage")}
            </button>
            <button
              type="button"
              disabled={bulkMarkingRead}
              onClick={() => void bulkMarkRead("filtered")}
              className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 press focus-ring disabled:opacity-60"
            >
              {bulkMarkingRead ? t("common.saving") : t("items.bulkRead.filtered")}
            </button>
            <button
              type="button"
              disabled={bulkMarkingRead}
              onClick={() => void bulkMarkRead("older_than_7d")}
              className="rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-900 hover:bg-amber-100 press focus-ring disabled:opacity-60"
            >
              {bulkMarkingRead ? t("common.saving") : t("items.bulkRead.olderThan7d")}
            </button>
          </div>
        </div>

        {/* Filters */}
        <div className="mb-4 flex flex-wrap items-center gap-2">
          {topic && (
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

        {!loading && unreadMode && unreadSuggestion && (
          <section className="rounded-xl border border-zinc-200 bg-white p-4">
            <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
              {locale === "ja" ? "次に読む" : "Next to Read"}
            </div>
            <button
              type="button"
              onClick={() => setInlineItemId(unreadSuggestion.id)}
              className="w-full rounded-lg border border-zinc-200 px-3 py-3 text-left hover:bg-zinc-50"
            >
              <div className="line-clamp-2 text-base font-semibold text-zinc-900">
                {unreadSuggestion.translated_title || unreadSuggestion.title || unreadSuggestion.url}
              </div>
              <div className="mt-2 inline-flex items-center gap-1 text-xs text-zinc-500">
                <span>{new Date(unreadSuggestion.published_at ?? unreadSuggestion.created_at).toLocaleDateString(locale === "ja" ? "ja-JP" : "en-US")}</span>
                <ArrowRight className="size-3.5" />
              </div>
              {unreadSuggestion.recommendation_reason && (
                <div className="mt-2 text-xs text-zinc-600">
                  {locale === "ja" ? "推薦理由: " : "Why: "}
                  {unreadSuggestion.recommendation_reason}
                </div>
              )}
            </button>
          </section>
        )}

        {!loading && (
          <div className="space-y-5">
            {dateSections.map((section) => (
              <section key={section.date} className="space-y-2">
                <h2 className="px-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-500">
                  {section.date}
                </h2>
                <ul className="list-none space-y-2">
                  {section.items.map((item, idx) => (
                    <li key={item.id} className="min-w-0 list-none">
                      {renderItem(item, { animIdx: idx })}
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        )}

        <Pagination
          total={itemsTotal}
          page={page}
          pageSize={pageSize}
          onPageChange={(nextPage) => replaceItemsQuery({ page: nextPage })}
        />

        {inlineItemId && (
          <InlineReader
            open={!!inlineItemId}
            itemId={inlineItemId}
            locale={locale}
            queueItemIds={sortedItems.map((v) => v.id)}
            onClose={() => setInlineItemId(null)}
            onOpenDetail={(itemId) => {
              setInlineItemId(null);
              rememberScroll(itemId);
              saveReadQueue(sortedItems.map((v) => v.id));
              router.push(detailHref(itemId));
            }}
            onOpenItem={(itemId) => setInlineItemId(itemId)}
            onReadToggled={(itemId, isRead) => {
              queryClient.setQueryData<ItemsFeedQueryData>(listQueryKey, (prev) =>
                prev
                  ? {
                      ...prev,
                      items: prev.items.map((v) => (v.id === itemId ? { ...v, is_read: isRead } : v)),
                    }
                  : prev
              );
            }}
            onFeedbackUpdated={() => {
              void queryClient.invalidateQueries({ queryKey: ["items-feed"] });
              void queryClient.invalidateQueries({ queryKey: ["focus-queue"] });
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
