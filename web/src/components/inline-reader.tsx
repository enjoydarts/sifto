"use client";

import { type PointerEvent, type TouchEvent, useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ExternalLink, Star, ThumbsDown, ThumbsUp, X } from "lucide-react";
import { api, ItemDetail } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";

export function InlineReader({
  itemId,
  open,
  locale,
  onClose,
  onOpenDetail,
  onOpenItem,
  queueItemIds,
  autoMarkRead = true,
  onReadToggled,
}: {
  itemId: string | null;
  open: boolean;
  locale: "ja" | "en";
  onClose: () => void;
  onOpenDetail: (itemId: string) => void;
  onOpenItem?: (itemId: string) => void;
  queueItemIds?: string[];
  autoMarkRead?: boolean;
  onReadToggled?: (itemId: string, isRead: boolean) => void;
}) {
  const { t } = useI18n();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [feedbackUpdating, setFeedbackUpdating] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [dragY, setDragY] = useState(0);
  const startYRef = useRef<number | null>(null);
  const enabled = open && !!itemId;
  const detailQuery = useQuery({
    queryKey: ["item-detail", itemId ?? ""],
    queryFn: () => api.getItem(itemId ?? ""),
    enabled,
    staleTime: 60_000,
  });
  const relatedQuery = useQuery({
    queryKey: ["item-related", itemId ?? "", 6],
    queryFn: () => api.getRelatedItems(itemId ?? "", { limit: 6 }),
    enabled,
    staleTime: 60_000,
  });

  const item = detailQuery.data ?? null;
  const related = useMemo(() => relatedQuery.data?.items ?? [], [relatedQuery.data?.items]);
  const nextQueueItemId = useMemo(() => {
    if (!itemId || !queueItemIds || queueItemIds.length === 0) return null;
    const idx = queueItemIds.indexOf(itemId);
    if (idx < 0 || idx + 1 >= queueItemIds.length) return null;
    return queueItemIds[idx + 1] ?? null;
  }, [itemId, queueItemIds]);
  const loading = detailQuery.isLoading || relatedQuery.isLoading;
  const error =
    (detailQuery.error ? String(detailQuery.error) : null) ??
    (relatedQuery.error ? String(relatedQuery.error) : null);

  async function toggleRead(current: ItemDetail) {
    const next = current.is_read ? await api.markItemUnread(current.id) : await api.markItemRead(current.id);
    queryClient.setQueryData<ItemDetail>(["item-detail", current.id], (prev) =>
      prev ? { ...prev, is_read: next.is_read } : prev
    );
    if (onReadToggled) onReadToggled(current.id, next.is_read);
  }

  function syncFeedbackInFeeds(itemIdVal: string, isFavorite: boolean, rating: number) {
    queryClient.setQueriesData({ queryKey: ["items-feed"] }, (prev: unknown) => {
      if (!prev || typeof prev !== "object") return prev;
      const data = prev as {
        items?: Array<Record<string, unknown>>;
        planClusters?: Array<Record<string, unknown>>;
      };
      const patchItem = (v: Record<string, unknown>) =>
        v.id === itemIdVal
          ? { ...v, is_favorite: isFavorite, feedback_rating: rating }
          : v;
      let changed = false;
      const next: Record<string, unknown> = { ...(data as Record<string, unknown>) };
      if (Array.isArray(data.items)) {
        next.items = data.items.map((v) => {
          const nv = patchItem(v);
          if (nv !== v) changed = true;
          return nv;
        });
      }
      if (Array.isArray(data.planClusters)) {
        next.planClusters = data.planClusters.map((cluster) => {
          const c = { ...cluster } as Record<string, unknown>;
          const rep = c.representative;
          if (rep && typeof rep === "object") {
            const nr = patchItem(rep as Record<string, unknown>);
            if (nr !== rep) {
              c.representative = nr;
              changed = true;
            }
          }
          const items = c.items;
          if (Array.isArray(items)) {
            c.items = items.map((v) => {
              if (!v || typeof v !== "object") return v;
              const nv = patchItem(v as Record<string, unknown>);
              if (nv !== v) changed = true;
              return nv;
            });
          }
          return c;
        });
      }
      return changed ? next : prev;
    });
  }

  async function updateFeedback(current: ItemDetail, patch: { rating?: -1 | 0 | 1; is_favorite?: boolean }) {
    if (feedbackUpdating) return;
    setFeedbackUpdating(true);
    const currentRating = (current.feedback?.rating ?? current.feedback_rating ?? 0) as -1 | 0 | 1;
    const currentFavorite = Boolean(current.feedback?.is_favorite ?? current.is_favorite ?? false);
    const nextRating = patch.rating != null ? patch.rating : currentRating;
    const nextFavorite = patch.is_favorite != null ? patch.is_favorite : currentFavorite;
    try {
      const next = await api.setItemFeedback(current.id, {
        rating: nextRating,
        is_favorite: nextFavorite,
      });
      queryClient.setQueryData<ItemDetail>(["item-detail", current.id], (prev) =>
        prev
          ? {
              ...prev,
              is_favorite: next.is_favorite,
              feedback_rating: next.rating,
              feedback: next,
            }
          : prev
      );
      syncFeedbackInFeeds(current.id, next.is_favorite, next.rating);
      showToast(t("itemDetail.toast.feedbackSaved"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setFeedbackUpdating(false);
    }
  }

  const autoMarkedRef = useRef<Record<string, true>>({});
  useEffect(() => {
    if (!autoMarkRead || !open || !item || item.is_read || autoMarkedRef.current[item.id]) return;
    autoMarkedRef.current[item.id] = true;
    api
      .markItemRead(item.id)
      .then((next) => {
        queryClient.setQueryData<ItemDetail>(["item-detail", item.id], (prev) =>
          prev ? { ...prev, is_read: next.is_read } : prev
        );
        if (onReadToggled) onReadToggled(item.id, next.is_read);
      })
      .catch(() => {
        delete autoMarkedRef.current[item.id];
      });
  }, [autoMarkRead, item, onReadToggled, open, queryClient]);

  const onHandlePointerDown = (e: PointerEvent<HTMLDivElement>) => {
    startYRef.current = e.clientY;
    setDragging(true);
  };
  const onHandlePointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (!dragging || startYRef.current == null) return;
    const dy = Math.max(0, e.clientY - startYRef.current);
    setDragY(dy);
  };
  const resetDrag = () => {
    setDragging(false);
    setDragY(0);
    startYRef.current = null;
  };
  const onHandlePointerUp = (e: PointerEvent<HTMLDivElement>) => {
    if (!dragging || startYRef.current == null) {
      resetDrag();
      return;
    }
    const dy = e.clientY - startYRef.current;
    if (dy > 90) {
      resetDrag();
      onClose();
      return;
    }
    resetDrag();
  };
  const onHandleTouchStart = (e: TouchEvent<HTMLDivElement>) => {
    const touch = e.touches[0];
    if (!touch) return;
    startYRef.current = touch.clientY;
    setDragging(true);
  };
  const onHandleTouchMove = (e: TouchEvent<HTMLDivElement>) => {
    if (!dragging || startYRef.current == null) return;
    const touch = e.touches[0];
    if (!touch) return;
    const dy = touch.clientY - startYRef.current;
    if (dy > 0) {
      e.preventDefault();
      setDragY(dy);
    }
  };
  const onHandleTouchEnd = (e: TouchEvent<HTMLDivElement>) => {
    if (!dragging || startYRef.current == null) {
      resetDrag();
      return;
    }
    const touch = e.changedTouches[0];
    const endY = touch ? touch.clientY : startYRef.current;
    const dy = endY - startYRef.current;
    if (dy > 90) {
      resetDrag();
      onClose();
      return;
    }
    resetDrag();
  };

  if (!open || !itemId) return null;

  return (
    <div className="fixed inset-0 z-40 bg-black/35" onClick={onClose}>
      <div
        className="absolute inset-y-0 right-0 w-full max-w-2xl overflow-y-auto border-l border-zinc-200 bg-white p-4 shadow-2xl transition-transform"
        onClick={(e) => e.stopPropagation()}
        style={{ transform: `translateY(${dragY}px)` }}
      >
        <div
          className="mb-2 flex justify-center py-1 touch-none md:hidden"
          onPointerDown={onHandlePointerDown}
          onPointerMove={onHandlePointerMove}
          onPointerUp={onHandlePointerUp}
          onPointerCancel={resetDrag}
          onTouchStart={onHandleTouchStart}
          onTouchMove={onHandleTouchMove}
          onTouchEnd={onHandleTouchEnd}
          onTouchCancel={resetDrag}
        >
          <span className="h-1.5 w-12 rounded-full bg-zinc-300" />
        </div>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-zinc-800">{t("items.inline.title")}</h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded border border-zinc-300 bg-white p-1 text-zinc-700 hover:bg-zinc-50"
            aria-label={t("common.close")}
          >
            <X className="size-4" aria-hidden="true" />
          </button>
        </div>

        {loading && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
        {error && <p className="text-sm text-red-500">{error}</p>}
        {!loading && !item && <p className="text-sm text-zinc-500">{t("common.noData")}</p>}

        {item && (
          <div className="space-y-4">
            <div>
              <h3 className="text-xl font-semibold leading-snug text-zinc-900">
                {item.translated_title || item.title || item.url}
              </h3>
              <a
                href={item.url}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-1 block break-all text-xs text-blue-600 hover:underline"
              >
                {item.url}
              </a>
            </div>

            <div className="space-y-2">
              <div className="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">
                {t("items.inline.primaryActions")}
              </div>
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
                <button
                  type="button"
                  onClick={() => void toggleRead(item)}
                  className="min-h-10 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50"
                >
                  {item.is_read ? t("items.action.markUnread") : t("items.action.markRead")}
                </button>
                <button
                  type="button"
                  onClick={() => onOpenDetail(item.id)}
                  className="inline-flex min-h-10 items-center justify-center gap-1 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50"
                >
                  <ExternalLink className="size-3.5" aria-hidden="true" />
                  <span>{t("items.action.openDetail")}</span>
                </button>
                {nextQueueItemId ? (
                  <button
                    type="button"
                    onClick={() => (onOpenItem ? onOpenItem(nextQueueItemId) : onOpenDetail(nextQueueItemId))}
                    className="min-h-10 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-medium text-blue-700 hover:bg-blue-100"
                  >
                    {t("itemDetail.next")}
                  </button>
                ) : (
                  <div className="hidden sm:block" aria-hidden="true" />
                )}
              </div>
              <div className="text-xs text-zinc-500">
                {new Date(item.published_at ?? item.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}
              </div>
            </div>

            <div className="space-y-2">
              <div className="text-[11px] font-semibold uppercase tracking-wide text-zinc-500">
                {t("items.inline.feedbackActions")}
              </div>
              <div className="grid grid-cols-3 gap-2">
                <button
                  type="button"
                  disabled={feedbackUpdating}
                  onClick={() =>
                    void updateFeedback(item, {
                      rating: (item.feedback?.rating ?? item.feedback_rating ?? 0) === 1 ? 0 : 1,
                    })
                  }
                  className={`inline-flex min-h-10 items-center justify-center gap-1 rounded-lg border px-2 py-2 text-sm font-medium transition-colors ${
                    (item.feedback?.rating ?? item.feedback_rating ?? 0) === 1
                      ? "border-green-200 bg-green-50 text-green-700"
                      : "border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
                  }`}
                >
                  <ThumbsUp className="size-4" aria-hidden="true" />
                  <span className="hidden sm:inline">{t("items.feedback.like")}</span>
                </button>
                <button
                  type="button"
                  disabled={feedbackUpdating}
                  onClick={() =>
                    void updateFeedback(item, {
                      rating: (item.feedback?.rating ?? item.feedback_rating ?? 0) === -1 ? 0 : -1,
                    })
                  }
                  className={`inline-flex min-h-10 items-center justify-center gap-1 rounded-lg border px-2 py-2 text-sm font-medium transition-colors ${
                    (item.feedback?.rating ?? item.feedback_rating ?? 0) === -1
                      ? "border-rose-200 bg-rose-50 text-rose-700"
                      : "border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
                  }`}
                >
                  <ThumbsDown className="size-4" aria-hidden="true" />
                  <span className="hidden sm:inline">{t("items.feedback.dislike")}</span>
                </button>
                <button
                  type="button"
                  disabled={feedbackUpdating}
                  onClick={() =>
                    void updateFeedback(item, {
                      is_favorite: !Boolean(item.feedback?.is_favorite ?? item.is_favorite ?? false),
                    })
                  }
                  className={`inline-flex min-h-10 items-center justify-center gap-1 rounded-lg border px-2 py-2 text-sm font-medium transition-colors ${
                    Boolean(item.feedback?.is_favorite ?? item.is_favorite ?? false)
                      ? "border-amber-200 bg-amber-50 text-amber-700"
                      : "border-zinc-200 bg-white text-zinc-600 hover:bg-zinc-50"
                  }`}
                >
                  <Star className={`size-4 ${Boolean(item.feedback?.is_favorite ?? item.is_favorite ?? false) ? "fill-current" : ""}`} aria-hidden="true" />
                  <span className="hidden sm:inline">{t("items.feedback.favorite")}</span>
                </button>
              </div>
            </div>

            {item.summary && (
              <section>
                <h4 className="mb-1 text-sm font-semibold text-zinc-800">{t("itemDetail.summary")}</h4>
                <p className="whitespace-pre-wrap text-sm leading-7 text-zinc-700">{item.summary.summary}</p>
              </section>
            )}

            {item.facts && item.facts.facts.length > 0 && (
              <section>
                <h4 className="mb-1 text-sm font-semibold text-zinc-800">{t("itemDetail.facts")}</h4>
                <ul className="list-disc space-y-1 pl-5 text-sm text-zinc-700">
                  {item.facts.facts.map((f, idx) => (
                    <li key={`${item.id}-fact-${idx}`}>{f}</li>
                  ))}
                </ul>
              </section>
            )}

            <section>
              <h4 className="mb-1 text-sm font-semibold text-zinc-800">{t("itemDetail.related")}</h4>
              {related.length === 0 ? (
                <p className="text-sm text-zinc-500">{t("itemDetail.relatedEmpty")}</p>
              ) : (
                <ul className="space-y-1.5">
                  {related.map((r) => (
                    <li key={r.id}>
                      <button
                        type="button"
                        onClick={() => (onOpenItem ? onOpenItem(r.id) : onOpenDetail(r.id))}
                        className="w-full truncate rounded border border-zinc-200 px-2 py-1.5 text-left text-sm text-zinc-700 hover:bg-zinc-50"
                        title={r.title || r.url}
                      >
                        {r.title || r.url}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </div>
        )}
      </div>
    </div>
  );
}
