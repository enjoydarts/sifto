"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ItemDetail, ItemGenreCount, ItemListResponse, RelatedItem } from "@/lib/api";
import {
  genreValueFromCountEntry,
  normalizeOtherGenreLabel,
  normalizeStoredGenreValue,
  orderGenreCountEntries,
  OTHER_GENRE_KEY,
} from "@/components/items/item-genre";
import { patchGenreSuggestionsResponse } from "@/components/items/item-genre-suggestions-cache.js";
import { patchItemsInFeedCaches, removeItemFromFeedCaches } from "@/lib/query-cache-helpers";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useConfirm } from "@/components/confirm-provider";

export type RelatedCluster = {
  id: string;
  label: string;
  size: number;
  max_similarity: number;
  representative: RelatedItem;
  items: RelatedItem[];
};

function resolveEffectiveOtherGenreLabel({
  effectiveGenre,
  effectiveOtherGenreLabel,
  userGenre,
  userOtherGenreLabel,
  summaryGenre,
  summaryOtherGenreLabel,
}: {
  effectiveGenre: string;
  effectiveOtherGenreLabel?: string | null;
  userGenre: string;
  userOtherGenreLabel?: string | null;
  summaryGenre: string;
  summaryOtherGenreLabel?: string | null;
}) {
  if (effectiveGenre !== OTHER_GENRE_KEY) return "";
  if (userGenre === OTHER_GENRE_KEY) return normalizeOtherGenreLabel(userOtherGenreLabel);
  if (summaryGenre === OTHER_GENRE_KEY) return normalizeOtherGenreLabel(summaryOtherGenreLabel);
  return normalizeOtherGenreLabel(effectiveOtherGenreLabel);
}

function extractHttpStatus(error: unknown): number | null {
  const message = error instanceof Error ? error.message : String(error);
  const match = message.match(/^(\d{3}):/);
  return match ? Number(match[1]) : null;
}

function localizeActionError(
  error: unknown,
  action:
    | "markRead"
    | "feedback"
    | "delete"
    | "retry"
    | "retryFromFacts"
    | "saveGenre"
    | "saveNote"
    | "createHighlight"
    | "deleteHighlight",
  t: (key: string, fallback?: string) => string
): string {
  const status = extractHttpStatus(error);
  if (status === 404) return t("itemDetail.actionError.notFound");
  if (status === 409) {
    switch (action) {
      case "retry":
        return t("itemDetail.actionError.retryUnavailable");
      case "retryFromFacts":
        return t("itemDetail.actionError.retryFromFactsUnavailable");
      case "delete":
      case "markRead":
      case "feedback":
      case "saveGenre":
      case "saveNote":
      case "createHighlight":
      case "deleteHighlight":
        return t("itemDetail.actionError.deletedReadonly");
    }
  }
  return error instanceof Error ? error.message : String(error);
}

export function useItemDetailData() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const { confirm } = useConfirm();
  const queryClient = useQueryClient();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const [item, setItem] = useState<ItemDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [readUpdating, setReadUpdating] = useState(false);
  const [deleteUpdating, setDeleteUpdating] = useState(false);
  const [restoreUpdating, setRestoreUpdating] = useState(false);
  const [feedbackUpdating, setFeedbackUpdating] = useState(false);
  const [retryUpdating, setRetryUpdating] = useState(false);
  const [retryFromFactsUpdating, setRetryFromFactsUpdating] = useState(false);
  const [genreUpdating, setGenreUpdating] = useState(false);
  const [related, setRelated] = useState<RelatedItem[]>([]);
  const [relatedClusters, setRelatedClusters] = useState<RelatedCluster[]>([]);
  const [expandedRelatedClusterIds, setExpandedRelatedClusterIds] = useState<Record<string, boolean>>({});
  const [relatedSortMode, setRelatedSortMode] = useState<"similarity" | "recent">("similarity");
  const [detailTab, setDetailTab] = useState<"summary" | "facts" | "body" | "related" | "notes" | "genre">("summary");
  const [relatedError, setRelatedError] = useState<string | null>(null);
  const [nextItemHref, setNextItemHref] = useState<string | null>(null);
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [itemNavigator, setItemNavigator] = useState<Awaited<ReturnType<typeof api.getItemNavigator>>["navigator"] | null>(null);
  const [itemNavigatorLoading, setItemNavigatorLoading] = useState(false);
  const [itemNavigatorError, setItemNavigatorError] = useState<string | null>(null);
  const [itemNavigatorOpen, setItemNavigatorOpen] = useState(false);
  const autoMarkedRef = useRef<Record<string, true>>({});
  const readStateOverrideRef = useRef<Record<string, boolean>>({});
  const settingsQuery = useQuery({
    queryKey: ["settings"] as const,
    queryFn: () => api.getSettings(),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
  const genreSuggestionsQuery = useQuery({
    queryKey: ["item-genre-suggestions"] as const,
    queryFn: () => api.getItems({ status: "summarized", sort: "newest", page: 1, page_size: 1 }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });
  const itemNavigatorLoadingPersona = settingsQuery.data?.llm_models?.navigator_persona?.trim() || "editor";
  const itemNavigatorDisplayPersona = itemNavigator?.avatar_style || itemNavigator?.persona || itemNavigatorLoadingPersona;

  const applyReadOverride = useCallback((nextItem: ItemDetail): ItemDetail => {
    const override = readStateOverrideRef.current[nextItem.id];
    if (override == null) return nextItem;
    return { ...nextItem, is_read: override };
  }, []);

  const refreshReadDependentQueries = useCallback(async () => {
    await Promise.allSettled([
      queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
      queryClient.invalidateQueries({ queryKey: ["focus-queue"] }),
      queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
    ]);
  }, [queryClient]);

  const refreshItemQueries = useCallback(
    async (itemId: string) => {
      await Promise.allSettled([
        queryClient.invalidateQueries({ queryKey: ["item-detail", itemId] }),
        queryClient.invalidateQueries({ queryKey: ["item-related", itemId, 6] }),
        queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
        queryClient.invalidateQueries({ queryKey: ["focus-queue"] }),
        queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
        queryClient.invalidateQueries({ queryKey: ["dashboard"] }),
      ]);
    },
    [queryClient]
  );

  const syncItemReadInFeedCaches = useCallback((itemId: string, isRead: boolean) => {
    patchItemsInFeedCaches(queryClient, itemId, { is_read: isRead });
  }, [queryClient]);

  const syncItemFeedbackInFeedCaches = useCallback(
    (itemId: string, patch: { is_favorite?: boolean; feedback_rating?: -1 | 0 | 1 | number }) => {
      const patchObj: Partial<Record<string, unknown>> = {};
      if (patch.is_favorite != null) patchObj.is_favorite = patch.is_favorite;
      if (patch.feedback_rating != null) patchObj.feedback_rating = patch.feedback_rating;
      patchItemsInFeedCaches(queryClient, itemId, patchObj);
    },
    [queryClient]
  );

  const syncItemGenreInFeedCaches = useCallback(
    (itemId: string, patch: {
      genre?: string | null;
      other_genre_label?: string | null;
      user_genre?: string | null;
      user_other_genre_label?: string | null;
    }) => {
      patchItemsInFeedCaches(queryClient, itemId, patch);
    },
    [queryClient]
  );

  const removeItemFromFeedCachesLocal = useCallback((itemId: string) => {
    removeItemFromFeedCaches(queryClient, itemId);
  }, [queryClient]);

  useEffect(() => {
    setItemNavigator(null);
    setItemNavigatorLoading(false);
    setItemNavigatorError(null);
    setItemNavigatorOpen(false);
  }, [id]);

  useEffect(() => {
    const cachedItem = queryClient.getQueryData<ItemDetail>(["item-detail", id]);
    const cachedRelated = queryClient.getQueryData<{ items?: RelatedItem[]; clusters?: RelatedCluster[] }>([
      "item-related",
      id,
      6,
    ]);
    if (cachedItem) {
      setItem(applyReadOverride(cachedItem));
      setLoadError(null);
      setActionError(null);
      setLoading(false);
    } else {
      setLoading(true);
    }
    if (cachedRelated) {
      setRelated(cachedRelated.items ?? []);
      setRelatedClusters(cachedRelated.clusters ?? []);
      setExpandedRelatedClusterIds({});
      setRelatedError(null);
    }
    let cancelled = false;
    Promise.allSettled([api.getItem(id), api.getRelatedItems(id, { limit: 6 })])
      .then((results) => {
        if (cancelled) return;
        const [detailRes, relatedRes] = results;
        if (detailRes.status === "rejected") {
          if (!cachedItem) throw detailRes.reason;
        } else {
          const nextItem = applyReadOverride(detailRes.value);
          queryClient.setQueryData(["item-detail", id], nextItem);
          setItem(nextItem);
          setLoadError(null);
          setActionError(null);
        }

        if (relatedRes.status === "fulfilled") {
          queryClient.setQueryData(["item-related", id, 6], relatedRes.value);
          setRelated(relatedRes.value.items ?? []);
          setRelatedClusters(relatedRes.value.clusters ?? []);
          setExpandedRelatedClusterIds({});
          setRelatedError(null);
        } else if (!cachedRelated) {
          setRelated([]);
          setRelatedClusters([]);
          setExpandedRelatedClusterIds({});
          setRelatedError(String(relatedRes.reason));
        }
      })
      .catch((e) => {
        if (!cancelled) setLoadError(String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [applyReadOverride, id, queryClient]);

  useEffect(() => {
    if (!item || item.status !== "summarized" || item.is_read || autoMarkedRef.current[item.id]) return;

    autoMarkedRef.current[item.id] = true;
    setReadUpdating(true);
    api
      .markItemRead(item.id)
      .then(async (next) => {
        readStateOverrideRef.current[item.id] = next.is_read;
        syncItemReadInFeedCaches(item.id, next.is_read);
        queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
          prev ? { ...prev, is_read: next.is_read } : prev
        );
        setItem((prev) => (prev && prev.id === item.id ? { ...prev, is_read: next.is_read } : prev));
        await refreshReadDependentQueries();
      })
      .catch(() => {
        delete autoMarkedRef.current[item.id];
      })
      .finally(() => setReadUpdating(false));
  }, [item, queryClient, refreshReadDependentQueries, syncItemReadInFeedCaches]);

  const dateLocale = useMemo(() => (locale === "ja" ? "ja-JP" : "en-US"), [locale]);
  const genreSuggestions = useMemo(
    () =>
      orderGenreCountEntries(genreSuggestionsQuery.data?.genre_counts ?? [])
        .map((entry: ItemGenreCount) => ({
          value: genreValueFromCountEntry(entry),
          count: entry.count,
        }))
        .filter((entry, index, arr) => entry.value !== "" && arr.findIndex((v) => v.value === entry.value) === index),
    [genreSuggestionsQuery.data?.genre_counts]
  );
  const canMarkRead = !!item && item.status !== "deleted";
  const isDeleted = item?.status === "deleted";
  const canUseItemNavigator = Boolean(item && item.status !== "deleted" && item.summary?.summary && item.facts?.facts?.length);
  const disableMutations = Boolean(isDeleted);
  const backHref = useMemo(() => {
    const from = searchParams.get("from");
    return from && from.startsWith("/items") ? from : "/items";
  }, [searchParams]);
  const currentDetailHref = useMemo(() => {
    const nextQuery = searchParams.toString();
    return nextQuery ? `/items/${id}?${nextQuery}` : `/items/${id}`;
  }, [id, searchParams]);
  const queueStorageKey = useMemo(() => `items-queue:${backHref}`, [backHref]);
  useEffect(() => {
    try {
      const raw = sessionStorage.getItem(queueStorageKey);
      if (!raw) {
        setNextItemHref(null);
        return;
      }
      const ids = JSON.parse(raw) as string[];
      if (!Array.isArray(ids) || ids.length === 0) {
        setNextItemHref(null);
        return;
      }
      const idx = ids.findIndex((v) => v === id);
      if (idx < 0 || idx + 1 >= ids.length) {
        setNextItemHref(null);
        return;
      }
      const nextID = ids[idx + 1];
      setNextItemHref(nextID ? `/items/${nextID}?from=${encodeURIComponent(backHref)}` : null);
    } catch {
      setNextItemHref(null);
    }
  }, [backHref, id, queueStorageKey]);
  useEffect(() => {
    if (!nextItemHref) return;
    router.prefetch(nextItemHref);
    const nextId = nextItemHref.match(/\/items\/([^?]+)/)?.[1];
    if (!nextId) return;
    void queryClient.prefetchQuery({
      queryKey: ["item-detail", nextId],
      queryFn: () => api.getItem(nextId),
      staleTime: 60_000,
    });
    void queryClient.prefetchQuery({
      queryKey: ["item-related", nextId, 6],
      queryFn: () => api.getRelatedItems(nextId, { limit: 6 }),
      staleTime: 60_000,
    });
  }, [nextItemHref, queryClient, router]);
  const clusteredRelated = useMemo(() => {
    const clusters = relatedClusters.filter((c) => c.size >= 2).map((c) => ({
      ...c,
      items: [...c.items].sort((a, b) => {
        if (relatedSortMode === "recent") {
          return new Date(b.published_at ?? b.created_at).getTime() - new Date(a.published_at ?? a.created_at).getTime();
        }
        if (b.similarity !== a.similarity) return b.similarity - a.similarity;
        return new Date(b.published_at ?? b.created_at).getTime() - new Date(a.published_at ?? a.created_at).getTime();
      }),
    }));
    clusters.sort((a, b) => {
      if (relatedSortMode === "recent") {
        const aTime = Math.max(...a.items.map((v) => new Date(v.published_at ?? v.created_at).getTime()));
        const bTime = Math.max(...b.items.map((v) => new Date(v.published_at ?? v.created_at).getTime()));
        if (bTime !== aTime) return bTime - aTime;
      } else if (b.max_similarity !== a.max_similarity) {
        return b.max_similarity - a.max_similarity;
      }
      return b.size - a.size;
    });
    return clusters;
  }, [relatedClusters, relatedSortMode]);
  const singleRelated = useMemo(
    () =>
      relatedClusters.length > 0
        ? [...relatedClusters.filter((c) => c.size < 2).flatMap((c) => c.items)].sort((a, b) => {
            if (relatedSortMode === "recent") {
              return new Date(b.published_at ?? b.created_at).getTime() - new Date(a.published_at ?? a.created_at).getTime();
            }
            if (b.similarity !== a.similarity) return b.similarity - a.similarity;
            return new Date(b.published_at ?? b.created_at).getTime() - new Date(a.published_at ?? a.created_at).getTime();
          })
        : related,
    [related, relatedClusters, relatedSortMode]
  );
  const openInlineRelatedItem = useCallback((relatedItemId: string) => {
    setInlineItemId(relatedItemId);
  }, []);
  const openItemDetailFromInlineReader = useCallback(
    (nextId: string) => {
      router.push(`/items/${nextId}?from=${encodeURIComponent(currentDetailHref)}`);
    },
    [currentDetailHref, router]
  );

  const openItemNavigator = useCallback(async () => {
    if (!item || !canUseItemNavigator || itemNavigatorLoading) return;
    if (itemNavigator) {
      setItemNavigatorOpen(true);
      return;
    }
    setItemNavigatorLoading(true);
    setItemNavigatorError(null);
    try {
      const preview = searchParams.get("navigator_preview") === "1";
      const res = await api.getItemNavigator(item.id, preview ? { cache_bust: true, navigator_preview: true } : undefined);
      setItemNavigator(res.navigator ?? null);
      if (!res.navigator) {
        setItemNavigatorError(t("itemDetail.navigatorUnavailable"));
        return;
      }
      setItemNavigatorOpen(true);
    } catch (error) {
      setItemNavigatorError(t("itemDetail.navigatorError"));
      showToast(`${t("briefing.navigator.label")}: ${error instanceof Error ? error.message : String(error)}`, "error");
    } finally {
      setItemNavigatorLoading(false);
    }
  }, [canUseItemNavigator, item, itemNavigator, itemNavigatorLoading, searchParams, showToast, t]);

  const toggleRead = async () => {
    if (!item || item.status === "deleted") return;
    setReadUpdating(true);
    try {
      const next = item.is_read ? await api.markItemUnread(item.id) : await api.markItemRead(item.id);
      readStateOverrideRef.current[item.id] = next.is_read;
      syncItemReadInFeedCaches(item.id, next.is_read);
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, is_read: next.is_read } : prev
      );
      setItem({ ...item, is_read: next.is_read });
      await refreshReadDependentQueries();
      setActionError(null);
      showToast(
        next.is_read ? t("itemDetail.toast.markRead") : t("itemDetail.toast.markUnread"),
        "success"
      );
    } catch (e) {
      const message = localizeActionError(e, "markRead", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setReadUpdating(false);
    }
  };

  const updateFeedback = async (patch: { rating?: -1 | 0 | 1; is_favorite?: boolean }) => {
    if (!item || item.status === "deleted") return;
    setFeedbackUpdating(true);
    const nextRating =
      patch.rating != null ? patch.rating : ((item.feedback?.rating ?? 0) as -1 | 0 | 1);
    const nextFavorite =
      patch.is_favorite != null ? patch.is_favorite : Boolean(item.feedback?.is_favorite ?? false);
    try {
      const next = await api.setItemFeedback(item.id, {
        rating: nextRating,
        is_favorite: nextFavorite,
      });
      syncItemFeedbackInFeedCaches(item.id, {
        is_favorite: next.is_favorite,
        feedback_rating: next.rating,
      });
      setItem((prev) => (prev ? { ...prev, feedback: next } : prev));
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, feedback: next } : prev
      );
      setActionError(null);
      showToast(t("itemDetail.toast.feedbackSaved"), "success");
    } catch (e) {
      const message = localizeActionError(e, "feedback", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setFeedbackUpdating(false);
    }
  };

  const deleteItem = async () => {
    if (!item || deleteUpdating || item.status === "deleted") return;
    const ok = await confirm({
      title: t("itemDetail.delete.title"),
      message: t("itemDetail.delete.message"),
      tone: "danger",
      confirmLabel: t("itemDetail.delete.confirm"),
      cancelLabel: t("common.cancel"),
    });
    if (!ok) return;
    setDeleteUpdating(true);
    try {
      await api.deleteItem(item.id);
      removeItemFromFeedCachesLocal(item.id);
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) => (prev ? { ...prev, status: "deleted" } : prev));
      setItem((prev) => (prev ? { ...prev, status: "deleted" } : prev));
      setActionError(null);
      showToast(t("itemDetail.toast.deleted"), "success");
    } catch (e) {
      const message = localizeActionError(e, "delete", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setDeleteUpdating(false);
    }
  };

  const restoreItem = async () => {
    if (!item || restoreUpdating || item.status !== "deleted") return;
    setRestoreUpdating(true);
    try {
      const next = await api.restoreItem(item.id);
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], next);
      setItem(next);
      await refreshItemQueries(item.id);
      setActionError(null);
      showToast(t("itemDetail.toast.restored"), "success");
    } catch (e) {
      const message = localizeActionError(e, "delete", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setRestoreUpdating(false);
    }
  };

  const retryItem = async () => {
    if (!item || retryUpdating || item.status === "deleted") return;
    setRetryUpdating(true);
    try {
      await api.retryItem(item.id);
      const nextItem = (prev: ItemDetail): ItemDetail => ({
        ...prev,
        status: "new" as const,
        processing_error: null,
        content_text: null,
        facts: null,
        facts_llm: null,
        facts_check: null,
        facts_check_llm: null,
        summary: null,
        summary_llm: null,
        faithfulness: null,
        faithfulness_llm: null,
      });
      setItem((prev) => (prev ? nextItem(prev) : prev));
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) => (prev ? nextItem(prev) : prev));
      await refreshItemQueries(item.id);
      setActionError(null);
      showToast(t("itemDetail.toast.retryQueued"), "success");
    } catch (e) {
      const message = localizeActionError(e, "retry", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setRetryUpdating(false);
    }
  };

  const retryFromFacts = async () => {
    if (!item || retryFromFactsUpdating || item.status === "deleted") return;
    const ok = await confirm({
      title: t("itemDetail.retryFromFacts.title"),
      message: t("itemDetail.retryFromFacts.message"),
      tone: "default",
      confirmLabel: t("itemDetail.retryFromFacts.confirm"),
      cancelLabel: t("common.cancel"),
    });
    if (!ok) return;
    setRetryFromFactsUpdating(true);
    try {
      await api.retryItemFromFacts(item.id);
      const nextItem = (prev: ItemDetail): ItemDetail => ({
        ...prev,
        status: "fetched" as const,
        processing_error: null,
        facts: null,
        facts_llm: null,
        facts_check: null,
        facts_check_llm: null,
        summary: null,
        summary_llm: null,
        faithfulness: null,
        faithfulness_llm: null,
      });
      setItem((prev) => (prev ? nextItem(prev) : prev));
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) => (prev ? nextItem(prev) : prev));
      await refreshItemQueries(item.id);
      setActionError(null);
      showToast(t("itemDetail.toast.retryFromFactsQueued"), "success");
    } catch (e) {
      const message = localizeActionError(e, "retryFromFacts", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    } finally {
      setRetryFromFactsUpdating(false);
    }
  };

  const saveNote = async (content: string) => {
    if (!item || item.status === "deleted") return;
    try {
      const next = await api.saveItemNote(item.id, { content });
      setItem((prev) => (prev ? { ...prev, note: next } : prev));
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, note: next } : prev
      );
      setActionError(null);
      showToast(t("itemNote.saved"), "success");
    } catch (e) {
      const message = localizeActionError(e, "saveNote", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    }
  };

  const saveGenre = useCallback(
    async (input: { userGenre: string | null; userOtherGenreLabel: string | null }) => {
      if (!item || item.status === "deleted") {
        throw new Error(t("itemDetail.actionError.deletedReadonly"));
      }
      setGenreUpdating(true);
      try {
        const previousUserGenre = normalizeStoredGenreValue(item.user_genre);
        const previousSummaryGenre = normalizeStoredGenreValue(item.summary?.genre);
        const previousEffectiveGenre = normalizeStoredGenreValue(item.genre ?? (previousUserGenre || previousSummaryGenre));
        const normalizedUserGenre = normalizeStoredGenreValue(input.userGenre);
        const normalizedUserOtherGenreLabel = normalizedUserGenre === OTHER_GENRE_KEY
          ? normalizeOtherGenreLabel(input.userOtherGenreLabel)
          : "";
        const next = await api.updateItemGenre(item.id, {
          user_genre: normalizedUserGenre || null,
          user_other_genre_label: normalizedUserOtherGenreLabel || null,
        });
        const nextUserGenre = normalizeStoredGenreValue(next.user_genre ?? normalizedUserGenre);
        const nextUserOtherGenreLabel = nextUserGenre === OTHER_GENRE_KEY
          ? normalizeOtherGenreLabel(next.user_other_genre_label ?? normalizedUserOtherGenreLabel)
          : "";
        const nextSummaryGenre = normalizeStoredGenreValue(item.summary?.genre);
        const nextSummaryOtherGenreLabel = nextSummaryGenre === OTHER_GENRE_KEY
          ? normalizeOtherGenreLabel(item.summary?.other_genre_label)
          : "";
        const nextGenre = normalizeStoredGenreValue(next.genre ?? (nextUserGenre || nextSummaryGenre));
        const nextOtherGenreLabel = nextGenre === OTHER_GENRE_KEY
          ? normalizeOtherGenreLabel(
              next.other_genre_label ??
                resolveEffectiveOtherGenreLabel({
                  effectiveGenre: nextGenre,
                  effectiveOtherGenreLabel: item.other_genre_label,
                  userGenre: nextUserGenre,
                  userOtherGenreLabel: nextUserOtherGenreLabel,
                  summaryGenre: nextSummaryGenre,
                  summaryOtherGenreLabel: nextSummaryOtherGenreLabel,
                })
            )
          : "";
        const patch = {
          genre: nextGenre || null,
          other_genre_label: nextOtherGenreLabel || null,
          user_genre: nextUserGenre || null,
          user_other_genre_label: nextUserOtherGenreLabel || null,
        };
        syncItemGenreInFeedCaches(item.id, patch);
        setItem((prev) => (prev ? { ...prev, ...patch } : prev));
        queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
          prev ? { ...prev, ...patch } : prev
        );
        setActionError(null);
        await queryClient.invalidateQueries({ queryKey: ["items-feed"] });
        queryClient.setQueryData<ItemListResponse | undefined>(["item-genre-suggestions"], (prev) =>
          patchGenreSuggestionsResponse(prev, {
            beforeEffectiveGenre: previousEffectiveGenre,
            afterEffectiveGenre: nextGenre,
          })
        );
        await queryClient.invalidateQueries({ queryKey: ["item-genre-suggestions"] });
        showToast(
          nextUserGenre ? t("itemDetail.genre.saved") : t("itemDetail.genre.cleared"),
          "success"
        );
        return {
          genre: patch.genre,
          other_genre_label: patch.other_genre_label,
          user_genre: patch.user_genre,
          user_other_genre_label: patch.user_other_genre_label,
          summary_genre: nextSummaryGenre || null,
          summary_other_genre_label:
            nextSummaryGenre === OTHER_GENRE_KEY ? nextSummaryOtherGenreLabel || null : null,
        };
      } catch (e) {
        throw new Error(localizeActionError(e, "saveGenre", t));
      } finally {
        setGenreUpdating(false);
      }
    },
    [item, queryClient, showToast, syncItemGenreInFeedCaches, t]
  );

  const createHighlight = async (input: { quote_text: string; anchor_text?: string; section?: string }) => {
    if (!item || item.status === "deleted") return;
    try {
      const next = await api.createItemHighlight(item.id, input);
      setItem((prev) => (prev ? { ...prev, highlights: [next, ...(prev.highlights ?? [])] } : prev));
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, highlights: [next, ...(prev.highlights ?? [])] } : prev
      );
      setActionError(null);
      showToast(t("itemHighlight.saved"), "success");
    } catch (e) {
      const message = localizeActionError(e, "createHighlight", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    }
  };

  const deleteHighlight = async (highlightId: string) => {
    if (!item || item.status === "deleted") return;
    try {
      await api.deleteItemHighlight(item.id, highlightId);
      setItem((prev) =>
        prev ? { ...prev, highlights: (prev.highlights ?? []).filter((highlight) => highlight.id !== highlightId) } : prev
      );
      queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
        prev ? { ...prev, highlights: (prev.highlights ?? []).filter((highlight) => highlight.id !== highlightId) } : prev
      );
      setActionError(null);
      showToast(t("itemHighlight.deleted"), "success");
    } catch (e) {
      const message = localizeActionError(e, "deleteHighlight", t);
      setActionError(message);
      showToast(`${t("common.error")}: ${message}`, "error");
    }
  };

  return {
    t,
    locale,
    item,
    loading,
    loadError,
    actionError,
    readUpdating,
    deleteUpdating,
    restoreUpdating,
    feedbackUpdating,
    retryUpdating,
    retryFromFactsUpdating,
    genreUpdating,
    related,
    relatedClusters,
    expandedRelatedClusterIds,
    setExpandedRelatedClusterIds,
    relatedSortMode,
    setRelatedSortMode,
    detailTab,
    setDetailTab,
    relatedError,
    nextItemHref,
    inlineItemId,
    setInlineItemId,
    itemNavigator,
    itemNavigatorLoading,
    itemNavigatorError,
    itemNavigatorOpen,
    setItemNavigatorOpen,
    itemNavigatorDisplayPersona,
    genreSuggestions,
    dateLocale,
    canMarkRead,
    isDeleted,
    canUseItemNavigator,
    disableMutations,
    backHref,
    currentDetailHref,
    clusteredRelated,
    singleRelated,
    openInlineRelatedItem,
    openItemDetailFromInlineReader,
    openItemNavigator,
    toggleRead,
    updateFeedback,
    deleteItem,
    restoreItem,
    retryItem,
    retryFromFacts,
    saveGenre,
    saveNote,
    createHighlight,
    deleteHighlight,
  };
}
