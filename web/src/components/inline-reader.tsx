"use client";

import { type PointerEvent, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ExternalLink, Star, ThumbsDown, ThumbsUp, X } from "lucide-react";
import { api, ItemDetail } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { CheckStatusBadges } from "@/components/items/check-status-badges";
import { ItemNoteEditor } from "@/components/items/item-note-editor";
import { ItemHighlightList } from "@/components/items/item-highlight-list";
import { Tabs, TabList, Tab, TabPanel } from "@/components/tabs";

export function InlineReader({
  itemId,
  open,
  locale,
  itemStatus,
  onClose,
  onOpenDetail,
  onOpenItem,
  queueItemIds,
  autoMarkRead = true,
  onReadToggled,
  onFeedbackUpdated,
}: {
  itemId: string | null;
  open: boolean;
  locale: "ja" | "en";
  itemStatus?: string | null;
  onClose: () => void;
  onOpenDetail: (itemId: string) => void;
  onOpenItem?: (itemId: string) => void;
  queueItemIds?: string[];
  autoMarkRead?: boolean;
  onReadToggled?: (itemId: string, isRead: boolean) => void;
  onFeedbackUpdated?: (itemId: string) => void;
}) {
  const { t } = useI18n();
  const { showToast } = useToast();
  const queryClient = useQueryClient();
  const [feedbackUpdating, setFeedbackUpdating] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [dragY, setDragY] = useState(0);
  const [mounted, setMounted] = useState(false);
  const startYRef = useRef<number | null>(null);
  const startAtRef = useRef<number>(0);
  const lastMoveAtRef = useRef<number>(0);
  const lastMoveYRef = useRef<number>(0);
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
  const effectiveStatus = itemStatus ?? item?.status ?? null;
  const canMarkRead = effectiveStatus === "summarized";
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

  useEffect(() => {
    setMounted(true);
    return () => setMounted(false);
  }, []);

  async function toggleRead(current: ItemDetail) {
    if (!canMarkRead) return;
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
      if (onFeedbackUpdated) onFeedbackUpdated(current.id);
      showToast(t("itemDetail.toast.feedbackSaved"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    } finally {
      setFeedbackUpdating(false);
    }
  }

  async function saveNote(current: ItemDetail, content: string) {
    try {
      const next = await api.saveItemNote(current.id, { content });
      queryClient.setQueryData<ItemDetail>(["item-detail", current.id], (prev) =>
        prev ? { ...prev, note: next } : prev
      );
      showToast(t("itemNote.saved"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
  }

  async function createHighlight(current: ItemDetail, input: { quote_text: string; anchor_text?: string; section?: string }) {
    try {
      const next = await api.createItemHighlight(current.id, input);
      queryClient.setQueryData<ItemDetail>(["item-detail", current.id], (prev) =>
        prev ? { ...prev, highlights: [next, ...(prev.highlights ?? [])] } : prev
      );
      showToast(t("itemHighlight.saved"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
  }

  async function deleteHighlight(current: ItemDetail, highlightId: string) {
    try {
      await api.deleteItemHighlight(current.id, highlightId);
      queryClient.setQueryData<ItemDetail>(["item-detail", current.id], (prev) =>
        prev ? { ...prev, highlights: (prev.highlights ?? []).filter((highlight) => highlight.id !== highlightId) } : prev
      );
      showToast(t("itemHighlight.deleted"), "success");
    } catch (e) {
      showToast(`${t("common.error")}: ${String(e)}`, "error");
    }
  }

  const autoMarkedRef = useRef<Record<string, true>>({});
  useEffect(() => {
    if (!autoMarkRead || !open || !item || !canMarkRead || item.is_read || autoMarkedRef.current[item.id]) return;
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
  }, [autoMarkRead, canMarkRead, item, onReadToggled, open, queryClient]);

  const onHandlePointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return;
    const now = Date.now();
    startYRef.current = e.clientY;
    startAtRef.current = now;
    lastMoveAtRef.current = now;
    lastMoveYRef.current = e.clientY;
    setDragging(true);
    try {
      e.currentTarget.setPointerCapture(e.pointerId);
    } catch {
      // no-op
    }
  };
  const onHandlePointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (!dragging || startYRef.current == null) return;
    const dy = Math.max(0, e.clientY - startYRef.current);
    lastMoveAtRef.current = Date.now();
    lastMoveYRef.current = e.clientY;
    setDragY(dy);
  };
  const closeBySwipe = () => {
    setDragging(false);
    const closeY = typeof window !== "undefined" ? Math.max(window.innerHeight, 720) : 720;
    setDragY(closeY);
    window.setTimeout(() => onClose(), 180);
  };
  const resetDrag = () => {
    setDragging(false);
    setDragY(0);
    startYRef.current = null;
    startAtRef.current = 0;
    lastMoveAtRef.current = 0;
    lastMoveYRef.current = 0;
  };
  const onHandlePointerUp = (e: PointerEvent<HTMLDivElement>) => {
    if (!dragging || startYRef.current == null) {
      resetDrag();
      return;
    }
    const dy = e.clientY - startYRef.current;
    const now = Date.now();
    const elapsedMs = Math.max(1, (lastMoveAtRef.current || now) - (startAtRef.current || now));
    const velocityY = (lastMoveYRef.current - startYRef.current) / elapsedMs;
    const shouldClose = dy > 72 || (dy > 32 && velocityY > 0.7);
    if (shouldClose) {
      startYRef.current = null;
      closeBySwipe();
      return;
    }
    resetDrag();
  };

  if (!open || !itemId || !mounted) return null;

  const feedbackRating = (item?.feedback?.rating ?? item?.feedback_rating ?? 0) as -1 | 0 | 1;
  const isFavorite = Boolean(item?.feedback?.is_favorite ?? item?.is_favorite ?? false);

  return createPortal(
    <div className="fixed inset-0 z-40 bg-[rgba(18,14,11,0.52)] backdrop-blur-[3px]" onClick={onClose}>
      <div
        className={`absolute inset-x-0 inset-y-0 z-[61] w-full overflow-x-hidden overflow-y-auto overscroll-y-contain border-l border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] opacity-100 shadow-[0_24px_60px_rgba(28,20,14,0.24)] will-change-transform md:left-auto md:right-0 md:max-w-[72rem] ${
          dragging ? "transition-none" : "transition-transform duration-200 ease-out"
        }`}
        onClick={(e) => e.stopPropagation()}
        style={{ transform: `translateY(${dragY}px)`, overscrollBehaviorY: "contain" }}
      >
        {/* Swipe handle (mobile) */}
        <div
          className="mb-1 flex min-h-9 items-center justify-center px-2 py-1.5 touch-none md:hidden"
          style={{ touchAction: "none" }}
          onPointerDown={onHandlePointerDown}
          onPointerMove={onHandlePointerMove}
          onPointerUp={onHandlePointerUp}
          onPointerCancel={resetDrag}
        >
          <span
            className={`rounded-full transition-all duration-150 ${
              dragging
                ? "h-1.5 w-16 bg-[var(--color-editorial-line-strong)] shadow-[0_0_0_4px_rgba(128,108,88,0.14)]"
                : "h-1.5 w-12 bg-[var(--color-editorial-line)]"
            }`}
          />
        </div>

        {loading && <p className="p-5 text-sm text-[var(--color-editorial-ink-soft)] sm:p-6">{t("common.loading")}</p>}
        {error && <p className="p-5 text-sm text-[var(--color-editorial-error)] sm:p-6">{error}</p>}
        {!loading && !item && <p className="p-5 text-sm text-[var(--color-editorial-ink-soft)] sm:p-6">{t("common.noData")}</p>}

        {item && (
          <>
            {/* Header: title + URL + close */}
            <div className="border-b border-[var(--color-editorial-line)] px-5 pb-5 pt-3 sm:p-6">
              <div className="mb-2 flex items-center justify-between gap-3">
                {item.source_title ? (
                  <div className="min-w-0 text-xs font-medium uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {item.source_title}
                  </div>
                ) : (
                  <div />
                )}
                <button
                  type="button"
                  onClick={onClose}
                  className="shrink-0 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-2 text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] press focus-ring"
                  aria-label={t("common.close")}
                >
                  <X className="size-4" aria-hidden="true" />
                </button>
              </div>
              <h3 className="text-2xl font-bold leading-snug text-[var(--color-editorial-ink)] sm:text-[30px]">
                {item.translated_title || item.title || item.url}
              </h3>
              <a
                href={item.url}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-2 block break-all text-[13px] text-[var(--color-editorial-accent)] hover:underline"
              >
                {item.url}
              </a>
              <div className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">
                {new Date(item.published_at ?? item.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}
              </div>
            </div>

            {/* Primary actions + feedback (outside tabs) */}
            <div className="space-y-3 border-b border-[var(--color-editorial-line)] p-5 sm:p-6">
              {nextQueueItemId && (
                <button
                  type="button"
                  onClick={() => (onOpenItem ? onOpenItem(nextQueueItemId) : onOpenDetail(nextQueueItemId))}
                  className="flex w-full items-center justify-center gap-2 rounded-[16px] bg-[var(--color-editorial-ink)] px-4 py-[14px] text-[15px] font-semibold text-[var(--color-editorial-panel-strong)] hover:opacity-90 press focus-ring"
                >
                  <span>{t("itemDetail.next")}</span>
                  <span className="text-lg">→</span>
                </button>
              )}
              <div className="grid grid-cols-2 gap-2">
                {canMarkRead ? (
                  <button
                    type="button"
                    onClick={() => void toggleRead(item)}
                    className={`min-h-10 rounded-full px-3 py-[10px] text-[14px] font-medium ${
                      item.is_read
                        ? "border border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                        : "border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                    }`}
                  >
                    {item.is_read ? t("items.action.markUnread") : t("items.action.markRead")}
                  </button>
                ) : null}
                <button
                  type="button"
                  onClick={() => onOpenDetail(item.id)}
                  className={`inline-flex min-h-10 items-center justify-center gap-1 rounded-full border border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] px-3 py-[10px] text-[14px] font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] ${
                    canMarkRead ? "" : "col-span-2"
                  }`}
                >
                  <ExternalLink className="size-3.5" aria-hidden="true" />
                  <span>{t("items.action.openDetail")}</span>
                </button>
              </div>
              <div className="grid grid-cols-3 gap-2">
                <button
                  type="button"
                  disabled={feedbackUpdating}
                  onClick={() =>
                    void updateFeedback(item, {
                      rating: feedbackRating === 1 ? 0 : 1,
                    })
                  }
                  className={`inline-flex min-h-10 items-center justify-center gap-1 rounded-full border p-3 text-sm font-medium transition-colors ${
                    feedbackRating === 1
                      ? "border-[var(--color-editorial-success-line)] bg-[var(--color-editorial-success-soft)] text-[var(--color-editorial-success)]"
                      : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  }`}
                >
                  <ThumbsUp className="size-[18px]" aria-hidden="true" />
                  <span className="hidden sm:inline">{t("items.feedback.like")}</span>
                </button>
                <button
                  type="button"
                  disabled={feedbackUpdating}
                  onClick={() =>
                    void updateFeedback(item, {
                      rating: feedbackRating === -1 ? 0 : -1,
                    })
                  }
                  className={`inline-flex min-h-10 items-center justify-center gap-1 rounded-full border p-3 text-sm font-medium transition-colors ${
                    feedbackRating === -1
                      ? "border-[var(--color-editorial-error-line)] bg-[var(--color-editorial-error-soft)] text-[var(--color-editorial-error)]"
                      : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  }`}
                >
                  <ThumbsDown className="size-[18px]" aria-hidden="true" />
                  <span className="hidden sm:inline">{t("items.feedback.dislike")}</span>
                </button>
                <button
                  type="button"
                  disabled={feedbackUpdating}
                  onClick={() =>
                    void updateFeedback(item, {
                      is_favorite: !isFavorite,
                    })
                  }
                  className={`inline-flex min-h-10 items-center justify-center gap-1 rounded-full border p-3 text-sm font-medium transition-colors ${
                    isFavorite
                      ? "border-[var(--color-editorial-accent-line)] bg-[var(--color-editorial-accent-soft)] text-[var(--color-editorial-accent)]"
                      : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                  }`}
                >
                  <Star className={`size-[18px] ${isFavorite ? "fill-current" : ""}`} aria-hidden="true" />
                  <span className="hidden sm:inline">{t("items.feedback.favorite")}</span>
                </button>
              </div>
            </div>

            {/* Tab navigation + content */}
            <Tabs defaultValue="summary">
              <TabList>
                <Tab value="summary">{t("tabs.summary")}</Tab>
                <Tab value="facts">{t("tabs.facts")}</Tab>
                <Tab value="body">{t("tabs.body")}</Tab>
                <Tab value="related">{t("tabs.related")}</Tab>
                <Tab value="notes">{t("tabs.notes")}</Tab>
              </TabList>

              <TabPanel value="summary" className="px-5 py-7 sm:px-6">
                {item.summary ? (
                  <div>
                    <CheckStatusBadges
                      factsCheckResult={item.facts_check?.final_result}
                      faithfulnessResult={item.faithfulness?.final_result}
                      t={t}
                    />
                    <p className="mt-3 whitespace-pre-wrap text-base leading-[1.8] text-[var(--color-editorial-ink)]">
                      {item.summary.summary}
                    </p>
                  </div>
                ) : (
                  <p className="text-base text-[var(--color-editorial-ink-faint)]">{t("common.noData")}</p>
                )}
              </TabPanel>

              <TabPanel value="facts" className="px-5 py-7 sm:px-6">
                {item.facts && item.facts.facts.length > 0 ? (
                  <ul className="list-disc space-y-2 pl-5 text-base leading-[1.8] text-[var(--color-editorial-ink)]">
                    {item.facts.facts.map((f, idx) => (
                      <li key={`${item.id}-fact-${idx}`}>{f}</li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-base text-[var(--color-editorial-ink-faint)]">{t("common.noData")}</p>
                )}
              </TabPanel>

              <TabPanel value="body" className="px-5 py-7 sm:px-6">
                {item.content_text ? (
                  <div className="max-w-prose whitespace-pre-wrap text-base leading-[1.8] text-[var(--color-editorial-ink)]">
                    {item.content_text}
                  </div>
                ) : (
                  <p className="text-base text-[var(--color-editorial-ink-faint)]">{t("tabs.bodyUnavailable")}</p>
                )}
              </TabPanel>

              <TabPanel value="related" className="px-5 py-7 sm:px-6">
                {related.length === 0 ? (
                  <p className="text-base text-[var(--color-editorial-ink-faint)]">{t("itemDetail.relatedEmpty")}</p>
                ) : (
                  <ul className="space-y-2">
                    {related.map((r) => (
                      <li key={r.id}>
                        <button
                          type="button"
                          onClick={() => (onOpenItem ? onOpenItem(r.id) : onOpenDetail(r.id))}
                          className="w-full truncate rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-left text-base text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                          title={r.title || r.url}
                        >
                          {r.title || r.url}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </TabPanel>

              <TabPanel value="notes" className="px-5 py-7 sm:px-6">
                <div className="space-y-3">
                  <div>
                    <h4 className="text-base font-semibold text-[var(--color-editorial-ink)]">{t("itemDetail.savedNotes")}</h4>
                    <p className="mt-1 text-sm text-[var(--color-editorial-ink-faint)]">{t("itemDetail.savedNotesDesc")}</p>
                  </div>
                  <div className="grid items-stretch gap-4 lg:grid-cols-2">
                    <ItemNoteEditor key={item.id} note={item.note ?? null} onSave={(content) => saveNote(item, content)} />
                    <ItemHighlightList
                      highlights={item.highlights ?? []}
                      onCreate={(input) => createHighlight(item, input)}
                      onDelete={(highlightId) => deleteHighlight(item, highlightId)}
                    />
                  </div>
                </div>
              </TabPanel>
            </Tabs>
          </>
        )}
      </div>
    </div>,
    document.body
  );
}
