"use client";

import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, Item, ItemSearchSuggestion } from "@/lib/api";
import { patchItemsInFeedCaches } from "@/lib/query-cache-helpers";
import { queryKeys } from "@/lib/query-keys";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";
import { useSharedAudioPlayer } from "@/components/shared-audio-player/provider";
import { buildItemsSearchParams, useItemsViewState } from "@/components/items/use-items-view-state";

export type ItemsFeedQueryData = {
  items: Item[];
  total: number;
  searchUnavailable?: boolean;
  searchMode?: "natural" | "and" | "or" | string | null;
};

export function useItemsPageData() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const player = useSharedAudioPlayer();
  const queryClient = useQueryClient();
  const router = useRouter();
  const { state: viewState, currentItemsHref, setFeed, setSort, setFilter, setTopic, setSource, setSearch, setFavorite, setPage, resetFilters } = useItemsViewState();
  const { feedMode, sortMode, filter, topic, sourceID, searchQuery, searchMode, unreadOnly, favoriteOnly, page } = viewState;
  const unreadMode = feedMode === "unread";
  const readMode = feedMode === "read";
  const laterMode = feedMode === "later";
  const pendingMode = feedMode === "pending";
  const deletedMode = feedMode === "deleted";
  const summaryAudioPlaybackBlocked = player.summaryAudioSettingsLoaded && !player.summaryAudioConfigured;
  const pageSize = 20;
  const [error, setError] = useState<string | null>(null);
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [retryingIds, setRetryingIds] = useState<Record<string, boolean>>({});
  const [readUpdatingIds, setReadUpdatingIds] = useState<Record<string, boolean>>({});
  const [feedbackUpdatingIds, setFeedbackUpdatingIds] = useState<Record<string, boolean>>({});
  const [bulkMarkingRead, setBulkMarkingRead] = useState(false);
  const [bulkRetrying, setBulkRetrying] = useState(false);
  const [bulkRetryingFromFacts, setBulkRetryingFromFacts] = useState(false);
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [selectedItemIDs, setSelectedItemIDs] = useState<string[]>([]);
  const [toolbarAction, setToolbarAction] = useState<"" | "triage_all" | "bulk_filtered" | "bulk_older">("");
  const [pendingBulkAction, setPendingBulkAction] = useState<"" | "retry" | "retry_from_facts" | "delete">("");
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchDraft, setSearchDraft] = useState(searchQuery);
  const [searchModeDraft, setSearchModeDraft] = useState<"natural" | "and" | "or">(searchMode);
  const [activeSuggestionIndex, setActiveSuggestionIndex] = useState(0);
  const restoredScrollRef = useRef<string | null>(null);
  const prefetchedDetailIDsRef = useRef<Record<string, true>>({});
  const [inlineQueueItemIds, setInlineQueueItemIds] = useState<string[]>([]);

  const listQueryKey = useMemo(
    () => [
      ...queryKeys.items.feedPrefix,
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
      setPendingBulkAction("");
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
    queryKey: queryKeys.items.searchSuggestions(normalizedSearchDraft),
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

  const submitSearch = useCallback(() => {
    setSearch(normalizedSearchDraft, searchModeDraft);
    setSearchOpen(false);
  }, [normalizedSearchDraft, searchModeDraft, setSearch]);

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
        setSearch(nextQuery, searchModeDraft);
        setSearchOpen(false);
      }
    },
    [searchModeDraft, setSearch]
  );
  const showFilterBadges = !!(sourceID || searchQuery || topic || (filter && filter !== "pending"));

  const scrollStorageKey = useMemo(() => `items-scroll:${currentItemsHref}`, [currentItemsHref]);
  const lastItemStorageKey = useMemo(() => `items-last-item:${currentItemsHref}`, [currentItemsHref]);
  const queueStorageKey = useMemo(() => `items-queue:${currentItemsHref}`, [currentItemsHref]);
  const summaryAudioViewQuery = useMemo(() => {
    const params = buildItemsSearchParams(viewState);
    params.delete("page");
    return params.toString();
  }, [viewState]);

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
        await queryClient.invalidateQueries({ queryKey: queryKeys.items.feedPrefix });
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
        await queryClient.invalidateQueries({ queryKey: queryKeys.items.feedPrefix });
        await queryClient.invalidateQueries({ queryKey: queryKeys.queues.focus() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.briefing.todayPrefix });
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
        await queryClient.invalidateQueries({ queryKey: queryKeys.items.feedPrefix });
        await queryClient.invalidateQueries({ queryKey: queryKeys.queues.focus() });
        await queryClient.invalidateQueries({ queryKey: queryKeys.briefing.todayPrefix });
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
      setPendingBulkAction("");
      await queryClient.invalidateQueries({ queryKey: queryKeys.items.feedPrefix });
      await queryClient.invalidateQueries({ queryKey: queryKeys.queues.focus() });
      await queryClient.invalidateQueries({ queryKey: queryKeys.briefing.todayPrefix });
    } catch (e) {
      setError(String(e));
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setBulkRetryingFromFacts(false);
    }
  }, [confirm, queryClient, selectedItemIDs, showToast, t]);

  const bulkRetry = useCallback(async () => {
    if (selectedItemIDs.length === 0) return;
    const ok = await confirm({
      title: t("items.bulkRetry.title").replace("{{count}}", String(selectedItemIDs.length)),
      message: t("items.bulkRetry.message"),
      confirmLabel: t("items.bulkRetry.confirm"),
      tone: "danger",
    });
    if (!ok) return;

    setBulkRetrying(true);
    try {
      const result = await api.retryItemsBulk(selectedItemIDs);
      if (result.skipped_count > 0) {
        showToast(
          t("items.bulkRetry.toastQueuedAndSkipped")
            .replace("{{queued}}", String(result.queued_count))
            .replace("{{skipped}}", String(result.skipped_count)),
          "info"
        );
      } else {
        showToast(t("items.bulkRetry.toastQueued").replace("{{count}}", String(result.queued_count)), "success");
      }
      setSelectedItemIDs([]);
      setPendingBulkAction("");
      await queryClient.invalidateQueries({ queryKey: queryKeys.items.feedPrefix });
      await queryClient.invalidateQueries({ queryKey: queryKeys.queues.focus() });
      await queryClient.invalidateQueries({ queryKey: queryKeys.briefing.todayPrefix });
    } catch (e) {
      setError(String(e));
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setBulkRetrying(false);
    }
  }, [confirm, queryClient, selectedItemIDs, showToast, t]);

  const bulkDelete = useCallback(async () => {
    if (selectedItemIDs.length === 0) return;
    const ok = await confirm({
      title: t("items.bulkDelete.title").replace("{{count}}", String(selectedItemIDs.length)),
      message: t("items.bulkDelete.message"),
      confirmLabel: t("items.bulkDelete.confirm"),
      tone: "danger",
    });
    if (!ok) return;

    setBulkDeleting(true);
    try {
      const result = await api.deleteItemsBulk(selectedItemIDs);
      if (result.skipped_count > 0) {
        showToast(
          t("items.bulkDelete.toastDeletedAndSkipped")
            .replace("{{deleted}}", String(result.updated_count))
            .replace("{{skipped}}", String(result.skipped_count)),
          "info"
        );
      } else {
        showToast(t("items.bulkDelete.toastDeleted").replace("{{count}}", String(result.updated_count)), "success");
      }
      setSelectedItemIDs([]);
      setPendingBulkAction("");
      await queryClient.invalidateQueries({ queryKey: queryKeys.items.feedPrefix });
      await queryClient.invalidateQueries({ queryKey: queryKeys.queues.focus() });
      await queryClient.invalidateQueries({ queryKey: queryKeys.briefing.todayPrefix });
    } catch (e) {
      setError(String(e));
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setBulkDeleting(false);
    }
  }, [confirm, queryClient, selectedItemIDs, showToast, t]);

  const sortedItems = useMemo(() => [...items].sort((a, b) => {
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
  }), [items, sortMode]);
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
      queryKey: queryKeys.items.detail(itemId),
      queryFn: () => api.getItem(itemId),
      staleTime: 60_000,
    });
    void queryClient.prefetchQuery({
      queryKey: queryKeys.items.related(itemId, 6),
      queryFn: () => api.getRelatedItems(itemId, { limit: 6 }),
      staleTime: 60_000,
    });
  }, [queryClient]);

  const syncFeedbackInFeeds = useCallback((itemId: string, isFavorite: boolean, rating: number) => {
    patchItemsInFeedCaches(queryClient, itemId, { is_favorite: isFavorite, feedback_rating: rating });
    queryClient.setQueryData(queryKeys.items.detail(itemId), (prev: unknown) => {
      if (!prev || typeof prev !== "object") return prev;
      return {
        ...(prev as Record<string, unknown>),
        is_favorite: isFavorite,
        feedback_rating: rating,
        feedback: {
          ...(((prev as { feedback?: Record<string, unknown> }).feedback ?? {}) as Record<string, unknown>),
          is_favorite: isFavorite,
          rating,
        },
      };
    });
  }, [queryClient]);

  const updateItemFeedback = useCallback(async (item: Item, patch: { rating?: -1 | 0 | 1; is_favorite?: boolean }) => {
    if (feedbackUpdatingIds[item.id]) {
      return;
    }
    setFeedbackUpdatingIds((prev) => ({ ...prev, [item.id]: true }));
    const currentRating = (item.feedback_rating ?? 0) as -1 | 0 | 1;
    const currentFavorite = Boolean(item.is_favorite);
    const nextRating = patch.rating != null ? patch.rating : currentRating;
    const nextFavorite = patch.is_favorite != null ? patch.is_favorite : currentFavorite;
    try {
      const next = await api.setItemFeedback(item.id, {
        rating: nextRating,
        is_favorite: nextFavorite,
      });
      syncFeedbackInFeeds(item.id, next.is_favorite, next.rating);
      await queryClient.invalidateQueries({ queryKey: queryKeys.items.feedPrefix, refetchType: "none" });
      await queryClient.invalidateQueries({ queryKey: queryKeys.preferenceProfile() });
      showToast(t("itemDetail.toast.feedbackSaved"), "success");
    } catch (e) {
      setError(String(e));
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setFeedbackUpdatingIds((prev) => {
        const next = { ...prev };
        delete next[item.id];
        return next;
      });
    }
  }, [feedbackUpdatingIds, queryClient, showToast, syncFeedbackInFeeds, t]);

  return {
    t, locale,
    viewState, currentItemsHref,
    setFeed, setSort, setFilter, setTopic, setSource, setSearch, setFavorite, setPage, resetFilters,
    feedMode, sortMode, filter, topic, sourceID, searchQuery, searchMode, unreadOnly, favoriteOnly, page,
    unreadMode, readMode, laterMode, pendingMode, deletedMode,
    summaryAudioPlaybackBlocked,
    pageSize,
    error, setError,
    inlineItemId, setInlineItemId,
    retryingIds,
    readUpdatingIds,
    feedbackUpdatingIds,
    bulkMarkingRead,
    bulkRetrying,
    bulkRetryingFromFacts,
    bulkDeleting,
    selectedItemIDs,
    toolbarAction, setToolbarAction,
    pendingBulkAction, setPendingBulkAction,
    searchOpen, setSearchOpen,
    searchDraft, setSearchDraft,
    searchModeDraft, setSearchModeDraft,
    activeSuggestionIndex, setActiveSuggestionIndex,
    inlineQueueItemIds, setInlineQueueItemIds,
    listQueryKey,
    items, itemsTotal, searchUnavailable,
    loading, visibleError,
    suggestions,
    suggestionsLoading: suggestionsQuery.isFetching && !suggestionsQuery.data,
    suggestionsEnabled,
    submitSearch,
    visibleSearchValue,
    applySuggestion,
    showFilterBadges,
    summaryAudioViewQuery,
    sortedItems,
    dateSections,
    inlineItemStatus,
    summaryMetrics,
    pageSubtitleKey,
    detailHref,
    rememberScroll,
    saveReadQueue,
    prefetchItemDetail,
    updateItemFeedback,
    retryItem,
    toggleRead,
    bulkMarkRead,
    selectedItemIDSet,
    visibleSelectedCount,
    toggleSelectedItem,
    selectAllVisibleItems,
    clearSelectedItems,
    bulkRetryFromFacts,
    bulkRetry,
    bulkDelete,
    queryClient,
    router,
  };
}
