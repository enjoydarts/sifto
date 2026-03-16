"use client";

import { Suspense, useState, useEffect, useCallback, useMemo, useRef } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCheck, Newspaper } from "lucide-react";
import { api, Item } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import Pagination from "@/components/pagination";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";
import { InlineReader } from "@/components/inline-reader";
import { PageTransition } from "@/components/page-transition";
import { EmptyState } from "@/components/empty-state";
import { SkeletonItemRow } from "@/components/skeleton";
import { FiltersBar } from "@/components/items/filters-bar";
import { ItemCard } from "@/components/items/item-card";
import { FeedTabs, type FeedMode, type SortMode } from "@/components/items/feed-tabs";

const FILTERS = ["", "summarized", "pending", "new", "fetched", "facts_extracted", "failed"] as const;
type ItemsFeedQueryData = {
  items: Item[];
  total: number;
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
          : qFeed === "pending"
            ? "pending"
          : "unread";

    const qSort = searchParams.get("sort");
    const sortMode: SortMode = qSort === "score" ? "score" : qSort === "personal_score" ? "personal_score" : "newest";

    const qFilter = searchParams.get("status");
    const filter =
      qFilter && FILTERS.includes(qFilter as (typeof FILTERS)[number]) ? qFilter : "";
    const topic = (searchParams.get("topic") ?? "").trim();
    const sourceID = (searchParams.get("source_id") ?? "").trim();

    const pendingFeed = qFeed === "pending";
    const unreadOnly = !pendingFeed && searchParams.get("unread") === "1";
    const favoriteOnly = !pendingFeed && searchParams.get("favorite") === "1";

    const qPage = Number(searchParams.get("page"));
    const page = Number.isFinite(qPage) && qPage >= 1 ? Math.floor(qPage) : 1;

    return { feedMode, sortMode, filter, topic, sourceID, unreadOnly, favoriteOnly, page };
  }, [searchParams]);
  const { feedMode, sortMode, filter, topic, sourceID, unreadOnly, favoriteOnly, page } = queryState;
  const unreadMode = feedMode === "unread";
  const readMode = feedMode === "read";
  const laterMode = feedMode === "later";
  const pendingMode = feedMode === "pending";
  const pageSize = 20;
  const [error, setError] = useState<string | null>(null);
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [retryingIds, setRetryingIds] = useState<Record<string, boolean>>({});
  const [readUpdatingIds, setReadUpdatingIds] = useState<Record<string, boolean>>({});
  const [bulkMarkingRead, setBulkMarkingRead] = useState(false);
  const [toolbarAction, setToolbarAction] = useState<"" | "triage_all" | "bulk_filtered" | "bulk_older">("");
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
        status: filter || (pendingMode ? "pending" : "summarized"),
        ...(sourceID ? { source_id: sourceID } : {}),
        ...(topic ? { topic } : {}),
        page,
        page_size: pageSize,
        sort: pendingMode ? "newest" : sortMode,
        unread_only: pendingMode ? false : unreadMode || unreadOnly,
        read_only: pendingMode ? false : readMode,
        favorite_only: pendingMode ? false : favoriteOnly,
        later_only: pendingMode ? false : laterMode,
      });
      return {
        items: data?.items ?? [],
        total: data?.total ?? 0,
      };
    },
    placeholderData: (prev) => prev,
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
      const implicitStatus = nextFeed === "pending" ? "pending" : "";
      const nextStatus = patch.status ?? (patch.feed ? implicitStatus : filter);
      const nextTopic = patch.topic ?? topic;
      const nextSourceID = patch.sourceId ?? sourceID;
      const nextUnread = nextFeed === "pending" ? false : patch.unread ?? unreadOnly;
      const nextFavorite = nextFeed === "pending" ? false : patch.favorite ?? favoriteOnly;
      const nextPage = patch.page ?? page;

      if (nextStatus) q.set("status", nextStatus);
      else q.delete("status");
      if (nextSourceID) q.set("source_id", nextSourceID);
      else q.delete("source_id");
      if (nextTopic) q.set("topic", nextTopic);
      else q.delete("topic");
      q.set("sort", nextFeed === "pending" ? "newest" : nextSort);
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
    q.set("sort", pendingMode ? "newest" : sortMode);
    if (page > 1) q.set("page", String(page));
    if (!pendingMode && unreadOnly) q.set("unread", "1");
    if (!pendingMode && favoriteOnly) q.set("favorite", "1");
    return q.toString();
  }, [favoriteOnly, feedMode, filter, page, pendingMode, sortMode, sourceID, topic, unreadOnly]);

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
    if (sortMode === "personal_score") {
      const as = a.personal_score ?? a.summary_score ?? -1;
      const bs = b.personal_score ?? b.summary_score ?? -1;
      if (bs !== as) return bs - as;
    } else if (sortMode === "score") {
      const as = a.summary_score ?? -1;
      const bs = b.summary_score ?? -1;
      if (bs !== as) return bs - as;
    }
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
  });
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
  const inlineItemStatus = useMemo(
    () => sortedItems.find((item) => item.id === inlineItemId)?.status ?? null,
    [inlineItemId, sortedItems]
  );

  const pageSubtitleKey =
    feedMode === "later"
      ? "items.subtitle.later"
      : feedMode === "read"
        ? "items.subtitle.read"
        : feedMode === "pending"
          ? "items.subtitle.pending"
          : "items.subtitle.unread";
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
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="flex items-center gap-2 text-2xl font-bold">
              <Newspaper className="size-6 text-zinc-500" aria-hidden="true" />
              <span>{t("items.title")}</span>
            </h1>
            <p className="mt-1 text-sm text-zinc-500">
              {t(pageSubtitleKey)} · {itemsTotal.toLocaleString()} {t("common.rows")}
            </p>
          </div>
          {!pendingMode && (
            <button
              type="button"
              onClick={() => router.push("/triage?mode=all")}
              className="inline-flex min-h-9 items-center gap-2 rounded-lg border border-zinc-900 bg-zinc-900 px-3.5 py-2 text-sm font-medium text-white hover:bg-zinc-800 press focus-ring"
            >
              <CheckCheck className="size-4" aria-hidden="true" />
              <span>{t("items.openAllTriage")}</span>
            </button>
          )}
        </div>

        <section className="overflow-hidden rounded-xl border border-zinc-200 bg-zinc-50/80 shadow-sm">
          <div className="flex flex-col gap-2 px-3 py-2 xl:flex-row xl:items-center">
            <div className="shrink-0 xl:w-[320px]">
              <FeedTabs
                feedMode={feedMode}
                onSelect={(feed) => replaceItemsQuery({ feed, page: 1, unread: false })}
                t={t}
              />
            </div>

            <div className="min-w-0 flex-1">
              <FiltersBar
                feedMode={feedMode}
                sortMode={sortMode}
                topic={topic}
                favoriteOnly={favoriteOnly}
                onSortChange={(sort) => replaceItemsQuery({ sort, page: 1 })}
                onTopicClear={() => replaceItemsQuery({ topic: "", page: 1 })}
                onFavoriteChange={(v) => replaceItemsQuery({ favorite: v, page: 1 })}
                t={t}
              />
            </div>

            {!pendingMode && (
              <div className="flex shrink-0 items-center gap-2 xl:w-[320px] xl:justify-end">
                <select
                  value={toolbarAction}
                  onChange={(e) => setToolbarAction(e.target.value as typeof toolbarAction)}
                  className="min-h-9 min-w-0 flex-1 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-700 focus-ring xl:w-[220px] xl:flex-none"
                  aria-label={t("items.toolbar.actions")}
                >
                  <option value="">{t("items.actions.placeholder")}</option>
                  <option value="bulk_filtered">{t("items.bulkRead.filtered")}</option>
                  <option value="bulk_older">{t("items.bulkRead.olderThan7d")}</option>
                </select>
                <button
                  type="button"
                  disabled={!toolbarAction || bulkMarkingRead}
                  onClick={() => {
                    if (toolbarAction === "bulk_filtered") {
                      void bulkMarkRead("filtered");
                      return;
                    }
                    if (toolbarAction === "bulk_older") {
                      void bulkMarkRead("older_than_7d");
                    }
                  }}
                  className="inline-flex min-h-9 items-center justify-center rounded-lg border border-zinc-900 bg-zinc-900 px-3.5 py-2 text-sm font-medium text-white hover:bg-zinc-800 press focus-ring disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {bulkMarkingRead ? t("common.saving") : t("items.actions.run")}
                </button>
              </div>
            )}

            {(sourceID || filter) && (
              <div className="flex flex-wrap items-center gap-2 xl:order-4 xl:basis-full">
                {topic && (
                  <div className="inline-flex items-center gap-2 rounded-full border border-blue-200 bg-blue-50 px-2.5 py-1 text-xs text-blue-800">
                    <span className="font-medium">
                      {t("items.topic")}: {topic}
                    </span>
                    <button
                      type="button"
                      onClick={() => replaceItemsQuery({ topic: "", page: 1 })}
                      className="rounded-full px-1.5 py-0.5 text-xs text-blue-700 hover:bg-blue-100 press"
                    >
                      {t("items.clear")}
                    </button>
                  </div>
                )}
                {sourceID && (
                  <span className="inline-flex items-center rounded-full border border-zinc-200 bg-white px-2.5 py-1 text-xs font-medium text-zinc-700">
                    {t("items.filter.sourceApplied")}
                  </span>
                )}
                {filter && (
                  filter !== "pending" && (
                  <span className="inline-flex items-center rounded-full border border-zinc-200 bg-white px-2.5 py-1 text-xs font-medium text-zinc-700">
                    {t(`items.filter.${filter}`)}
                  </span>
                  )
                )}
              </div>
            )}
          </div>
        </section>

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
            title={t(pendingMode ? "emptyState.itemsPending.title" : "emptyState.items.title")}
            description={t(pendingMode ? "emptyState.itemsPending.desc" : "emptyState.items.desc")}
            action={{ label: t("emptyState.items.action"), href: "/sources" }}
          />
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
            itemStatus={inlineItemStatus}
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
                      items: prev.items
                        .map((v) => (v.id === itemId ? { ...v, is_read: isRead } : v))
                        .filter((v) => {
                          if (unreadMode || unreadOnly) return !v.is_read;
                          if (readMode) return v.is_read;
                          return true;
                        }),
                      total:
                        (unreadMode || unreadOnly || readMode) && prev.items.some((v) => v.id === itemId)
                          ? Math.max(
                              0,
                              prev.total -
                                (unreadMode || unreadOnly
                                  ? isRead
                                    ? 1
                                    : 0
                                  : readMode
                                    ? isRead
                                      ? 0
                                      : 1
                                    : 0)
                            )
                          : prev.total,
                    }
                  : prev
              );
              void queryClient.invalidateQueries({ queryKey: ["items-feed"] });
              void queryClient.invalidateQueries({ queryKey: ["focus-queue"] });
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
