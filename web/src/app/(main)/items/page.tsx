"use client";

import { Suspense, useState, useEffect, useCallback, useMemo, useRef, startTransition } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCheck, Newspaper, Search, X } from "lucide-react";
import { api, Item, ItemSearchSuggestion } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";
import { InlineReader } from "@/components/inline-reader";
import { PageTransition } from "@/components/page-transition";
import { FiltersBar } from "@/components/items/filters-bar";
import { ItemCard } from "@/components/items/item-card";
import { FeedTabs, type FeedMode, type SortMode } from "@/components/items/feed-tabs";
import { ItemsSummaryStrip } from "@/components/items/items-summary-strip";
import { ItemsListState } from "@/components/items/items-list-state";
import { DenseArticleList } from "@/components/items/dense-article-list";
import { PageHeader } from "@/components/ui/page-header";
import { FilterBar } from "@/components/ui/filter-bar";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";
import { SkeletonList } from "@/components/ui/skeleton-list";

const FILTERS = ["", "summarized", "pending", "new", "fetched", "facts_extracted", "failed", "deleted"] as const;
type ItemsFeedQueryData = {
  items: Item[];
  total: number;
  searchUnavailable?: boolean;
  searchMode?: "natural" | "and" | "or" | string | null;
};

function parseItemsQueryState(searchParams: URLSearchParams) {
  const qFeed = searchParams.get("feed");
  const qFilter = searchParams.get("status");
  const deletedViaLegacyStatus = qFilter === "deleted";
  const feedMode: FeedMode =
    qFeed === "later"
      ? "later"
      : qFeed === "read"
        ? "read"
        : qFeed === "pending"
          ? "pending"
          : qFeed === "deleted"
            ? "deleted"
            : "unread";

  const qSort = searchParams.get("sort");
  const sortMode: SortMode = qSort === "score" ? "score" : qSort === "personal_score" ? "personal_score" : "newest";

  const filter =
    qFilter && FILTERS.includes(qFilter as (typeof FILTERS)[number]) && qFilter !== "deleted" ? qFilter : "";
  const topic = (searchParams.get("topic") ?? "").trim();
  const sourceID = (searchParams.get("source_id") ?? "").trim();
  const searchQuery = (searchParams.get("q") ?? "").trim();
  const qSearchMode = searchParams.get("search_mode");
  const searchMode: "natural" | "and" | "or" = qSearchMode === "and" ? "and" : qSearchMode === "or" ? "or" : "natural";

  const pendingFeed = qFeed === "pending";
  const unreadOnly = !pendingFeed && searchParams.get("unread") === "1";
  const favoriteOnly = !pendingFeed && searchParams.get("favorite") === "1";

  const qPage = Number(searchParams.get("page"));
  const page = Number.isFinite(qPage) && qPage >= 1 ? Math.floor(qPage) : 1;

  return {
    feedMode: deletedViaLegacyStatus ? "deleted" : feedMode,
    sortMode,
    filter,
    topic,
    sourceID,
    searchQuery,
    searchMode,
    unreadOnly,
    favoriteOnly,
    page,
  };
}

function ItemsPageContent() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const queryClient = useQueryClient();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryState = useMemo(() => parseItemsQueryState(new URLSearchParams(searchParams.toString())), [searchParams]);
  const { feedMode, sortMode, filter, topic, sourceID, searchQuery, searchMode, unreadOnly, favoriteOnly, page } = queryState;
  const unreadMode = feedMode === "unread";
  const readMode = feedMode === "read";
  const laterMode = feedMode === "later";
  const pendingMode = feedMode === "pending";
  const deletedMode = feedMode === "deleted";
  const pageSize = 20;
  const [error, setError] = useState<string | null>(null);
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [retryingIds, setRetryingIds] = useState<Record<string, boolean>>({});
  const [readUpdatingIds, setReadUpdatingIds] = useState<Record<string, boolean>>({});
  const [bulkMarkingRead, setBulkMarkingRead] = useState(false);
  const [bulkRetryingFromFacts, setBulkRetryingFromFacts] = useState(false);
  const [selectedItemIDs, setSelectedItemIDs] = useState<string[]>([]);
  const [toolbarAction, setToolbarAction] = useState<"" | "triage_all" | "bulk_filtered" | "bulk_older">("");
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchDraft, setSearchDraft] = useState(searchQuery);
  const [searchModeDraft, setSearchModeDraft] = useState<"natural" | "and" | "or">(searchMode);
  const [activeSuggestionIndex, setActiveSuggestionIndex] = useState(0);
  const restoredScrollRef = useRef<string | null>(null);
  const prefetchedDetailIDsRef = useRef<Record<string, true>>({});
  const [inlineQueueItemIds, setInlineQueueItemIds] = useState<string[]>([]);

  const listQueryKey = useMemo(
    () => [
      "items-feed",
      feedMode,
      filter,
      topic,
      sourceID,
      searchQuery,
      searchMode,
      page,
      sortMode,
      unreadOnly ? 1 : 0,
      favoriteOnly ? 1 : 0,
      readMode ? 1 : 0,
    ] as const,
    [favoriteOnly, feedMode, filter, page, readMode, searchMode, searchQuery, sortMode, sourceID, topic, unreadOnly]
  );

  const listQuery = useQuery<ItemsFeedQueryData>({
    queryKey: listQueryKey,
    queryFn: async () => {
      const data = await api.getItems({
        status: deletedMode ? "deleted" : filter || (pendingMode ? "pending" : "summarized"),
        ...(sourceID ? { source_id: sourceID } : {}),
        ...(topic ? { topic } : {}),
        ...(searchQuery ? { q: searchQuery } : {}),
        ...(searchQuery ? { search_mode: searchMode } : {}),
        page,
        page_size: pageSize,
        sort: pendingMode ? "newest" : sortMode,
        unread_only: pendingMode || deletedMode ? false : unreadMode || unreadOnly || laterMode,
        read_only: pendingMode || deletedMode ? false : readMode,
        favorite_only: pendingMode || deletedMode ? false : favoriteOnly,
        later_only: pendingMode || deletedMode ? false : laterMode,
      });
      return {
        items: data?.items ?? [],
        total: data?.total ?? 0,
        searchUnavailable: data?.search_unavailable ?? false,
        searchMode: data?.search_mode ?? searchMode,
      };
    },
  });
  const cachedItemsLength = listQuery.data?.items?.length ?? 0;
  const items = useMemo(() => listQuery.data?.items ?? [], [listQuery.data?.items]);
  const itemsTotal = listQuery.data?.total ?? 0;
  const searchUnavailable = listQuery.data?.searchUnavailable ?? false;
  const loading = !listQuery.data && (listQuery.isLoading || listQuery.isFetching);
  const queryError = listQuery.error ? String(listQuery.error) : null;
  const visibleError = error ?? queryError;

  useEffect(() => {
    if (!pendingMode) {
      setSelectedItemIDs([]);
      return;
    }
    const visibleIDs = new Set(items.map((item) => item.id));
    setSelectedItemIDs((prev) => prev.filter((itemID) => visibleIDs.has(itemID)));
  }, [items, pendingMode]);

  useEffect(() => {
    if (!searchOpen) {
      setSearchDraft(searchQuery);
      setSearchModeDraft(searchMode);
    }
  }, [searchMode, searchOpen, searchQuery]);

  const normalizedSearchDraft = useMemo(() => searchDraft.trim(), [searchDraft]);
  const suggestionsEnabled = searchOpen && normalizedSearchDraft.length >= 2;
  const suggestionsQuery = useQuery({
    queryKey: ["item-search-suggestions", normalizedSearchDraft],
    queryFn: async () => api.getItemSearchSuggestions({ q: normalizedSearchDraft, limit: 10 }),
    enabled: suggestionsEnabled,
    staleTime: 15_000,
    placeholderData: (prev) => prev,
  });
  const suggestions = useMemo(() => suggestionsQuery.data?.items ?? [], [suggestionsQuery.data]);

  useEffect(() => {
    if (!searchOpen || suggestions.length === 0) {
      setActiveSuggestionIndex(-1);
      return;
    }
    setActiveSuggestionIndex((prev) => {
      if (prev < 0) return -1;
      if (prev >= suggestions.length) return suggestions.length - 1;
      return prev;
    });
  }, [searchOpen, suggestions.length]);

  const replaceItemsQuery = useCallback(
    (
        patch: Partial<{
          feed: FeedMode;
          sort: SortMode;
          status: string;
          topic: string;
          sourceId: string;
          q: string;
          searchMode: "natural" | "and" | "or";
          unread: boolean;
          favorite: boolean;
          page: number;
      }>
    ) => {
      const baseSearch =
        typeof window !== "undefined" ? window.location.search : searchParams.toString() ? `?${searchParams.toString()}` : "";
      const q = new URLSearchParams(baseSearch);
      const current = parseItemsQueryState(q);

      const nextFeed = patch.feed ?? current.feedMode;
      q.set("feed", nextFeed);

      const nextSort = patch.sort ?? current.sortMode;
      const implicitStatus = nextFeed === "pending" ? "pending" : "";
      const nextStatus = patch.status ?? (patch.feed ? implicitStatus : current.filter);
      const nextTopic = patch.topic ?? current.topic;
      const nextSourceID = patch.sourceId ?? current.sourceID;
      const nextSearch = patch.q ?? current.searchQuery;
      const nextSearchMode = patch.searchMode ?? current.searchMode;
      const isDeletedFeed = nextFeed === "deleted";
      const nextUnread = nextFeed === "pending" || isDeletedFeed ? false : nextFeed === "later" ? true : patch.unread ?? current.unreadOnly;
      const nextFavorite = nextFeed === "pending" || isDeletedFeed ? false : patch.favorite ?? current.favoriteOnly;
      const nextPage = patch.page ?? current.page;

      if (nextStatus) q.set("status", nextStatus);
      else q.delete("status");
      if (nextSourceID) q.set("source_id", nextSourceID);
      else q.delete("source_id");
      if (nextTopic) q.set("topic", nextTopic);
      else q.delete("topic");
      if (nextSearch) q.set("q", nextSearch);
      else q.delete("q");
      if (nextSearch) q.set("search_mode", nextSearchMode);
      else q.delete("search_mode");
      q.set("sort", nextFeed === "pending" ? "newest" : nextSort);
      if (nextUnread) q.set("unread", "1");
      else q.delete("unread");
      if (nextFavorite) q.set("favorite", "1");
      else q.delete("favorite");
      if (nextPage > 1) q.set("page", String(nextPage));
      else q.delete("page");

      const nextQuery = q.toString();
      const nextUrl = nextQuery ? `${pathname}?${nextQuery}` : pathname;
      startTransition(() => {
        router.replace(nextUrl, { scroll: false });
      });
    },
    [pathname, router, searchParams]
  );

  const itemsQueryString = useMemo(() => {
    const q = new URLSearchParams();
    q.set("feed", feedMode);
    if (filter) q.set("status", filter);
    if (sourceID) q.set("source_id", sourceID);
    if (topic) q.set("topic", topic);
    if (searchQuery) q.set("q", searchQuery);
    if (searchQuery) q.set("search_mode", searchMode);
    q.set("sort", pendingMode ? "newest" : sortMode);
    if (page > 1) q.set("page", String(page));
    if (!pendingMode && !deletedMode && (unreadOnly || laterMode)) q.set("unread", "1");
    if (!pendingMode && !deletedMode && favoriteOnly) q.set("favorite", "1");
    return q.toString();
  }, [deletedMode, favoriteOnly, feedMode, filter, laterMode, page, pendingMode, searchMode, searchQuery, sortMode, sourceID, topic, unreadOnly]);

  const submitSearch = useCallback(() => {
    replaceItemsQuery({ q: normalizedSearchDraft, searchMode: searchModeDraft, page: 1 });
    setSearchOpen(false);
  }, [normalizedSearchDraft, replaceItemsQuery, searchModeDraft]);

  const visibleSearchValue = useMemo(() => {
    if (activeSuggestionIndex >= 0 && suggestions[activeSuggestionIndex]) {
      return suggestions[activeSuggestionIndex].label;
    }
    return searchDraft;
  }, [activeSuggestionIndex, searchDraft, suggestions]);

  const applySuggestion = useCallback(
    (suggestion: ItemSearchSuggestion) => {
      if (suggestion.label.trim()) {
        const nextQuery = suggestion.label.trim();
        setSearchDraft(nextQuery);
        setActiveSuggestionIndex(-1);
        replaceItemsQuery({ q: nextQuery, searchMode: searchModeDraft, page: 1 });
        setSearchOpen(false);
      }
    },
    [replaceItemsQuery, searchModeDraft]
  );

  const currentItemsHref = useMemo(
    () => (itemsQueryString ? `${pathname}?${itemsQueryString}` : pathname),
    [itemsQueryString, pathname]
  );
  const showFilterBadges = !!(sourceID || searchQuery || topic || (filter && filter !== "pending"));

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
      const willMarkRead = !item.is_read;
      try {
        queryClient.setQueryData<ItemsFeedQueryData>(listQueryKey, (prev) =>
          prev
            ? {
                ...prev,
                items:
                  laterMode && willMarkRead
                    ? prev.items.filter((v) => v.id !== item.id)
                    : prev.items.map((v) => (v.id === item.id ? { ...v, is_read: !item.is_read } : v)),
                total:
                  laterMode && willMarkRead
                    ? Math.max(0, prev.total - (prev.items.some((v) => v.id === item.id) ? 1 : 0))
                    : prev.total,
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
        await queryClient.invalidateQueries({ queryKey: ["items-feed"] });
        await queryClient.invalidateQueries({ queryKey: ["focus-queue"] });
        await queryClient.invalidateQueries({ queryKey: ["briefing-today"] });
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
    [laterMode, listQueryKey, queryClient, showToast, t]
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

  const selectedItemIDSet = useMemo(() => new Set(selectedItemIDs), [selectedItemIDs]);
  const visibleSelectedCount = selectedItemIDs.length;

  const toggleSelectedItem = useCallback((itemID: string) => {
    setSelectedItemIDs((prev) => (prev.includes(itemID) ? prev.filter((id) => id !== itemID) : [...prev, itemID]));
  }, []);

  const selectAllVisibleItems = useCallback(() => {
    setSelectedItemIDs(items.map((item) => item.id));
  }, [items]);

  const clearSelectedItems = useCallback(() => {
    setSelectedItemIDs([]);
  }, []);

  const bulkRetryFromFacts = useCallback(async () => {
    if (selectedItemIDs.length === 0) return;
    const ok = await confirm({
      title: t("items.bulkRetryFromFacts.title").replace("{{count}}", String(selectedItemIDs.length)),
      message: t("items.bulkRetryFromFacts.message"),
      confirmLabel: t("items.bulkRetryFromFacts.confirm"),
      tone: "danger",
    });
    if (!ok) return;

    setBulkRetryingFromFacts(true);
    try {
      const result = await api.retryItemsFromFactsBulk(selectedItemIDs);
      if (result.skipped_count > 0) {
        showToast(
          t("items.bulkRetryFromFacts.toastQueuedAndSkipped")
            .replace("{{queued}}", String(result.queued_count))
            .replace("{{skipped}}", String(result.skipped_count)),
          "info"
        );
      } else {
        showToast(t("items.bulkRetryFromFacts.toastQueued").replace("{{count}}", String(result.queued_count)), "success");
      }
      setSelectedItemIDs([]);
      await queryClient.invalidateQueries({ queryKey: ["items-feed"] });
      await queryClient.invalidateQueries({ queryKey: ["focus-queue"] });
      await queryClient.invalidateQueries({ queryKey: ["briefing-today"] });
    } catch (e) {
      setError(String(e));
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setBulkRetryingFromFacts(false);
    }
  }, [confirm, queryClient, selectedItemIDs, showToast, t]);

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
  const visibleUnreadCount = useMemo(() => sortedItems.filter((item) => !item.is_read).length, [sortedItems]);
  const visibleReadCount = useMemo(() => sortedItems.filter((item) => item.is_read).length, [sortedItems]);
  const visibleRetryCount = useMemo(() => sortedItems.filter((item) => item.status === "failed").length, [sortedItems]);
  const summaryMetrics = useMemo(
    () => [
      {
        key: "results",
        label: t("items.kpi.results"),
        value: itemsTotal.toLocaleString(),
        hint: t("items.state.summaryResultsHint"),
      },
      {
        key: "unread",
        label: t("items.kpi.unreadVisible"),
        value: visibleUnreadCount.toLocaleString(),
        hint: t("items.state.summaryUnreadHint"),
        tone: "accent" as const,
      },
      {
        key: "read",
        label: t("items.kpi.readVisible"),
        value: visibleReadCount.toLocaleString(),
        hint: t("items.state.summaryReadHint"),
      },
      {
        key: "retry",
        label: t("items.state.retryLabel"),
        value: visibleRetryCount.toLocaleString(),
        hint: t("items.state.summaryRetryHint"),
      },
    ],
    [itemsTotal, t, visibleReadCount, visibleRetryCount, visibleUnreadCount]
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
      setInlineQueueItemIds(sortedItems.map((v) => v.id));
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
        selectable={pendingMode}
        selected={selectedItemIDSet.has(item.id)}
        readUpdating={!!readUpdatingIds[item.id]}
        retrying={!!retryingIds[item.id]}
        onOpen={openInlineReader}
        onOpenDetail={openDetail}
        onToggleSelected={() => toggleSelectedItem(item.id)}
        onToggleRead={() => void toggleRead(item)}
        onRetry={() => void retryItem(item.id)}
        onPrefetch={() => prefetchItemDetail(item.id)}
        animationDelay={(opts?.animIdx ?? 0) * 40}
        t={t}
      />
    );
  }, [detailHref, locale, pendingMode, prefetchItemDetail, readUpdatingIds, rememberScroll, retryItem, retryingIds, router, saveReadQueue, selectedItemIDSet, sortedItems, t, toggleRead, toggleSelectedItem]);

  const railFilterTags = [
    topic ? (
      <Tag key="topic" tone="accent" removable onRemove={() => replaceItemsQuery({ topic: "", page: 1 })}>
        {t("items.topic")}: {topic}
      </Tag>
    ) : null,
    sourceID ? (
      <Tag
        key="source"
        tone="accent"
        removable
        removeLabel={t("common.clear")}
        onRemove={() => replaceItemsQuery({ sourceId: "", page: 1 })}
      >
        {t("items.filter.sourceApplied")}
      </Tag>
    ) : null,
    searchQuery ? (
      <Tag key="search" tone="success" removable onRemove={() => replaceItemsQuery({ q: "", page: 1 })}>
        {t("items.search.active")}: {searchQuery}
      </Tag>
    ) : null,
    filter && filter !== "pending" && filter !== "deleted" ? (
      <Tag
        key="status"
        tone="accent"
        removable
        removeLabel={t("common.clear")}
        onRemove={() => replaceItemsQuery({ status: "", page: 1 })}
      >
        {t(`items.filter.${filter}`)}
      </Tag>
    ) : null,
  ].filter(Boolean);

  return (
    <PageTransition>
      <div className="space-y-3 pb-8">
        <div className="grid gap-3 xl:grid-cols-[248px_minmax(0,1fr)] xl:items-start">
          <aside className="hidden xl:sticky xl:top-[6.25rem] xl:flex xl:self-start xl:flex-col xl:gap-4">
            <SectionCard compact className="overflow-hidden">
              <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                {t("items.rail.metrics")}
              </div>
              <div className="mt-3 divide-y divide-[var(--color-editorial-line)]">
                {summaryMetrics.map((metric) => (
                  <div key={metric.key} className="grid gap-1 py-3 first:pt-0 last:pb-0">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {metric.label}
                    </div>
                    <div className="text-[2rem] leading-none tracking-[-0.04em] text-[var(--color-editorial-ink)]">
                      {metric.value}
                    </div>
                    <div className="text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                      {metric.hint}
                    </div>
                  </div>
                ))}
              </div>
            </SectionCard>

            <SectionCard compact>
              <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                {t("items.rail.actions")}
              </div>
              <div className="mt-3 grid gap-2">
                <button
                  type="button"
                  onClick={() => {
                    setSearchDraft(searchQuery);
                    setSearchModeDraft(searchMode);
                    setSearchOpen(true);
                  }}
                  className={`inline-flex min-h-10 items-center justify-center gap-2 rounded-full border px-3 py-2 text-sm font-medium press focus-ring ${
                    searchQuery
                      ? "border-[var(--color-editorial-accent-line)] bg-[var(--color-editorial-accent-soft)] text-[var(--color-editorial-accent)]"
                      : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  }`}
                >
                  <Search className="size-4" aria-hidden="true" />
                  <span>{t("items.search.open")}</span>
                </button>
                {!pendingMode && (
                  <button
                    type="button"
                    onClick={() => router.push("/triage?mode=all")}
                    className="inline-flex min-h-10 items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-3 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 press focus-ring"
                  >
                    <CheckCheck className="size-4" aria-hidden="true" />
                    <span>{t("items.openAllTriage")}</span>
                  </button>
                )}
              </div>
            </SectionCard>

            {showFilterBadges && (
              <SectionCard compact>
                <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                  {t("items.rail.filters")}
                </div>
                <div className="mt-3 flex flex-wrap gap-2">{railFilterTags}</div>
              </SectionCard>
            )}
          </aside>

          <div className="min-w-0 space-y-3">
            <PageHeader
              compact
              className="overflow-hidden"
              eyebrow={t("items.state.eyebrow")}
              title={t("nav.items")}
              titleIcon={Newspaper}
              description={`${t(pageSubtitleKey)} · ${itemsTotal.toLocaleString()} ${t("common.rows")}`}
              meta={(
                <div className="inline-flex items-center gap-2 text-[11px] font-medium uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                  <Newspaper className="size-3.5" aria-hidden="true" />
                  <span>{t("items.state.meta")}</span>
                </div>
              )}
              actions={(
                <div className="flex w-full flex-wrap items-center justify-end gap-2 xl:hidden">
                  <button
                    type="button"
                    onClick={() => {
                      setSearchDraft(searchQuery);
                      setSearchModeDraft(searchMode);
                      setSearchOpen(true);
                    }}
                    className={`inline-flex min-h-9 items-center justify-center rounded-full border px-3 py-1.5 text-sm font-medium press focus-ring ${
                      searchQuery
                        ? "border-[var(--color-editorial-accent-line)] bg-[var(--color-editorial-accent-soft)] text-[var(--color-editorial-accent)]"
                        : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                    }`}
                    aria-label={t("items.search.open")}
                  >
                    <Search className="size-4" aria-hidden="true" />
                  </button>
                  {!pendingMode && (
                    <button
                      type="button"
                      onClick={() => router.push("/triage?mode=all")}
                      className="inline-flex min-h-9 items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-3.5 py-1.5 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 press focus-ring"
                    >
                      <CheckCheck className="size-4" aria-hidden="true" />
                      <span>{t("items.openAllTriage")}</span>
                    </button>
                  )}
                </div>
              )}
            />

            <div className="hidden xl:hidden">
              <ItemsSummaryStrip metrics={summaryMetrics} />
            </div>

            <FilterBar
              compact
              leading={(
                <FeedTabs
                  feedMode={feedMode}
                  onSelect={(feed) => {
                    replaceItemsQuery({ feed, page: 1, unread: false });
                  }}
                  t={t}
                />
              )}
              filters={(
                <FiltersBar
                  feedMode={feedMode}
                  sortMode={sortMode}
                  favoriteOnly={favoriteOnly}
                  toolbarAction={toolbarAction}
                  bulkMarkingRead={bulkMarkingRead}
                  onSortChange={(sort) => replaceItemsQuery({ sort, page: 1 })}
                  onFavoriteChange={(v) => replaceItemsQuery({ favorite: v, page: 1 })}
                  onToolbarActionChange={setToolbarAction}
                  onToolbarRun={() => {
                    if (toolbarAction === "bulk_filtered") {
                      void bulkMarkRead("filtered");
                      return;
                    }
                    if (toolbarAction === "bulk_older") {
                      void bulkMarkRead("older_than_7d");
                    }
                  }}
                  t={t}
                />
              )}
              actions={
                pendingMode ? (
                  <div className="flex w-full flex-wrap items-center justify-end gap-2 xl:w-auto xl:flex-nowrap">
                    <button
                      type="button"
                      onClick={selectAllVisibleItems}
                      disabled={items.length === 0}
                      className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3.5 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {t("items.bulkRetryFromFacts.selectAll")}
                    </button>
                    <button
                      type="button"
                      onClick={clearSelectedItems}
                      disabled={visibleSelectedCount === 0}
                      className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3.5 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {t("items.bulkRetryFromFacts.clearSelection")}
                    </button>
                    <div className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3.5 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)]">
                      {t("items.bulkRetryFromFacts.selectedCount").replace("{{count}}", String(visibleSelectedCount))}
                    </div>
                    <button
                      type="button"
                      disabled={visibleSelectedCount === 0 || bulkRetryingFromFacts}
                      onClick={() => {
                        void bulkRetryFromFacts();
                      }}
                      className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      {bulkRetryingFromFacts ? t("common.saving") : t("items.bulkRetryFromFacts.run")}
                    </button>
                  </div>
                ) : (
                  <div
                    aria-hidden={readMode}
                    className={`hidden w-full flex-wrap items-center justify-end gap-2 xl:flex xl:w-auto xl:flex-nowrap ${
                      readMode ? "pointer-events-none invisible" : ""
                    }`}
                  >
                      <select
                        value={toolbarAction}
                        onChange={(e) => setToolbarAction(e.target.value as typeof toolbarAction)}
                        className="min-h-10 min-w-0 flex-1 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3.5 py-2 text-sm text-[var(--color-editorial-ink-soft)] focus-ring xl:w-[188px] xl:flex-none"
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
                        className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {bulkMarkingRead ? t("common.saving") : t("items.actions.run")}
                      </button>
                    </div>
                )
              }
            >
              {showFilterBadges ? <div className="flex flex-wrap items-center gap-2">{railFilterTags}</div> : null}
            </FilterBar>

            {searchQuery && searchUnavailable ? (
              <SectionCard className="border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">
                {t("items.search.unavailable")}
              </SectionCard>
            ) : null}

            {loading || items.length === 0 || !!visibleError ? (
              <ItemsListState
                loading={loading}
                error={visibleError}
                isSearchActive={Boolean(searchQuery)}
                hasFilters={Boolean(topic || sourceID || (filter && filter !== "pending"))}
                isPendingMode={pendingMode}
                onRetry={() => {
                  setError(null);
                  void queryClient.invalidateQueries({ queryKey: listQueryKey });
                }}
                onResetFilters={() => replaceItemsQuery({ q: "", topic: "", sourceId: "", status: "", page: 1, favorite: false })}
                t={t}
              />
            ) : (
              <DenseArticleList
                sections={dateSections.map((section) => ({
                  date: section.date,
                  items: section.items.map((item, idx) => renderItem(item, { animIdx: idx })),
                }))}
                total={itemsTotal}
                page={page}
                pageSize={pageSize}
                onPageChange={(nextPage) => replaceItemsQuery({ page: nextPage })}
              />
            )}
          </div>
        </div>

        {inlineItemId && (
          <InlineReader
            open={!!inlineItemId}
            itemId={inlineItemId}
            locale={locale}
            queueItemIds={inlineQueueItemIds}
            itemStatus={inlineItemStatus}
            onClose={() => {
              setInlineItemId(null);
              setInlineQueueItemIds([]);
            }}
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

        {searchOpen && (
          <div
            className="fixed inset-0 z-50 flex items-start justify-center bg-zinc-950/45 px-4 py-10 sm:items-center"
            onClick={() => setSearchOpen(false)}
          >
            <div
              className="w-full max-w-lg rounded-2xl border border-zinc-200 bg-white p-5 shadow-2xl"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                  <h2 className="text-lg font-semibold text-zinc-900">{t("items.search.title")}</h2>
                  <p className="text-sm text-zinc-500">{t("items.search.description")}</p>
                </div>
                <button
                  type="button"
                  onClick={() => setSearchOpen(false)}
                  className="inline-flex size-9 items-center justify-center rounded-lg border border-zinc-200 bg-white text-zinc-500 hover:bg-zinc-100 hover:text-zinc-700 press focus-ring"
                  aria-label={t("common.close")}
                >
                  <X className="size-4" aria-hidden="true" />
                </button>
              </div>

              <div className="mt-4 space-y-3">
                <input
                  autoFocus
                  type="search"
                  value={visibleSearchValue}
                  onChange={(e) => {
                    setSearchDraft(e.target.value);
                    setActiveSuggestionIndex(-1);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "ArrowDown" && suggestions.length > 0) {
                      e.preventDefault();
                      setActiveSuggestionIndex((prev) => {
                        if (prev < 0) return 0;
                        return (prev + 1) % suggestions.length;
                      });
                      return;
                    }
                    if (e.key === "ArrowUp" && suggestions.length > 0) {
                      e.preventDefault();
                      setActiveSuggestionIndex((prev) => {
                        if (prev < 0) return suggestions.length - 1;
                        return (prev - 1 + suggestions.length) % suggestions.length;
                      });
                      return;
                    }
                    if (e.key === "Enter") {
                      e.preventDefault();
                      if (activeSuggestionIndex >= 0 && suggestions[activeSuggestionIndex]) {
                        applySuggestion(suggestions[activeSuggestionIndex]);
                        return;
                      }
                      submitSearch();
                    }
                    if (e.key === "Escape") {
                      e.preventDefault();
                      setSearchOpen(false);
                    }
                  }}
                  placeholder={t("items.search.placeholder")}
                  className="min-h-11 w-full rounded-xl border border-zinc-200 bg-white px-3.5 py-2.5 text-sm text-zinc-900 placeholder:text-zinc-400 focus-ring"
                />
                <div className="rounded-xl border border-zinc-200 bg-zinc-50/80 p-2">
                  {!suggestionsEnabled ? (
                    <div className="px-2 py-1 text-xs text-zinc-500">{t("items.search.suggestions.helper")}</div>
                  ) : suggestionsQuery.isFetching ? (
                    <div className="px-2 py-1 text-xs text-zinc-500">{t("items.search.suggestions.loading")}</div>
                  ) : suggestions.length === 0 ? (
                    <div className="px-2 py-1 text-xs text-zinc-500">{t("items.search.suggestions.empty")}</div>
                  ) : (
                    <div className="space-y-1">
                      {suggestions.map((suggestion, index) => {
                        const active = index === activeSuggestionIndex;
                        return (
                          <button
                            key={`${suggestion.kind}:${suggestion.source_id ?? suggestion.topic ?? suggestion.item_id ?? suggestion.label}:${index}`}
                            type="button"
                            onClick={() => applySuggestion(suggestion)}
                            className={`block w-full rounded-lg px-3 py-2 text-left text-sm transition ${
                              active ? "bg-zinc-900 text-white" : "bg-white text-zinc-900 hover:bg-zinc-100"
                            }`}
                          >
                            <span className="block truncate font-medium">{suggestion.label}</span>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
                <div className="space-y-2">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
                    {t("items.search.modeLabel")}
                  </div>
                  <div className="grid grid-cols-3 gap-2">
                    {(["natural", "and", "or"] as const).map((mode) => {
                      const active = searchModeDraft === mode;
                      const labelKey =
                        mode === "natural"
                          ? "items.search.mode.natural"
                          : mode === "and"
                            ? "items.search.mode.and"
                            : "items.search.mode.or";
                      return (
                        <button
                          key={mode}
                          type="button"
                          onClick={() => setSearchModeDraft(mode)}
                          className={`rounded-xl border px-3 py-2 text-sm font-medium transition ${
                            active
                              ? "border-zinc-900 bg-zinc-900 text-white"
                              : "border-zinc-200 bg-white text-zinc-700 hover:bg-zinc-50"
                          }`}
                        >
                          {t(labelKey)}
                        </button>
                      );
                    })}
                  </div>
                </div>
                <div className="flex items-center justify-between gap-3">
                  <button
                    type="button"
                    onClick={() => {
                      setSearchDraft("");
                      setActiveSuggestionIndex(-1);
                    }}
                    className="text-sm font-medium text-zinc-500 hover:text-zinc-700 press"
                  >
                    {t("common.clear")}
                  </button>
                  <button
                    type="button"
                    onClick={submitSearch}
                    className="inline-flex min-h-10 items-center rounded-lg border border-zinc-900 bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 press focus-ring"
                  >
                    {t("items.search.submit")}
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </PageTransition>
  );
}

export default function ItemsPage() {
  return (
    <Suspense
      fallback={
        <SkeletonList rows={8} />
      }
    >
      <ItemsPageContent />
    </Suspense>
  );
}
