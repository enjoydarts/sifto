"use client";

import { type PointerEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, ArrowUp, ArrowRight, ExternalLink, Hand } from "lucide-react";
import { api, Item } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { InlineReader } from "@/components/inline-reader";

type ActionType = "read" | "favorite" | "later";
type TriageMetricEvent = {
  ts: number;
  item_id: string;
  action: ActionType;
  elapsed_ms: number;
};

const TRIAGE_METRICS_KEY = "triage:metrics:v1";
const TRIAGE_METRICS_MAX = 300;

function rateTone(rate: number) {
  if (rate >= 80) return "text-green-700";
  if (rate >= 50) return "text-blue-700";
  return "text-rose-700";
}

export default function TriagePage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [actioned, setActioned] = useState<Record<string, true>>({});
  const [updating, setUpdating] = useState(false);
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [metricEvents, setMetricEvents] = useState<TriageMetricEvent[]>([]);
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const [swipeExit, setSwipeExit] = useState<ActionType | null>(null);
  const [mobileMetricsOpen, setMobileMetricsOpen] = useState(false);
  const startRef = useRef<{ x: number; y: number } | null>(null);
  const currentShownAtRef = useRef<{ id: string; shownAtMs: number } | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    try {
      const raw = localStorage.getItem(TRIAGE_METRICS_KEY);
      if (!raw) return;
      const parsed = JSON.parse(raw) as TriageMetricEvent[];
      if (!Array.isArray(parsed)) return;
      setMetricEvents(
        parsed
          .filter((v) => v && typeof v.ts === "number" && typeof v.elapsed_ms === "number" && typeof v.action === "string")
          .slice(-TRIAGE_METRICS_MAX)
      );
    } catch {
      // ignore parse errors
    }
  }, []);

  const settingsQuery = useQuery({
    queryKey: ["settings"],
    queryFn: api.getSettings,
  });
  const readingPlanPrefs = settingsQuery.data?.reading_plan;
  const focusWindow = readingPlanPrefs?.window ?? "24h";
  const focusSize = readingPlanPrefs?.size ?? 15;
  const diversifyTopics = Boolean(readingPlanPrefs?.diversify_topics ?? true);

  const queueQuery = useQuery({
    queryKey: ["triage-queue", focusWindow, focusSize, diversifyTopics ? 1 : 0] as const,
    queryFn: () =>
      api.getFocusQueue({
        window: focusWindow === "today_jst" || focusWindow === "7d" ? focusWindow : "24h",
        size: focusSize,
        diversify_topics: diversifyTopics,
      }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });

  const queue = useMemo(() => queueQuery.data?.items ?? [], [queueQuery.data?.items]);
  const pendingItems = useMemo(
    () => queue.filter((it) => !it.is_read && !actioned[it.id]),
    [actioned, queue]
  );
  const current = pendingItems[0] ?? null;
  const done = queue.length - pendingItems.length;
  const progress = queue.length > 0 ? Math.round((done / queue.length) * 100) : 0;

  const currentDetailQuery = useQuery({
    queryKey: ["item-detail", current?.id ?? ""],
    queryFn: () => api.getItem(current?.id ?? ""),
    enabled: !!current?.id,
    staleTime: 60_000,
  });

  useEffect(() => {
    if (!current?.id) return;
    currentShownAtRef.current = { id: current.id, shownAtMs: Date.now() };
    setDragOffset({ x: 0, y: 0 });
    setDragging(false);
    setSwipeExit(null);
  }, [current?.id]);

  const recordMetric = useCallback((itemID: string, action: ActionType) => {
    const shown = currentShownAtRef.current;
    if (!shown || shown.id !== itemID) return;
    const elapsed = Math.max(0, Date.now() - shown.shownAtMs);
    const event: TriageMetricEvent = {
      ts: Date.now(),
      item_id: itemID,
      action,
      elapsed_ms: elapsed,
    };
    setMetricEvents((prev) => {
      const next = [...prev, event].slice(-TRIAGE_METRICS_MAX);
      try {
        localStorage.setItem(TRIAGE_METRICS_KEY, JSON.stringify(next));
      } catch {
        // ignore quota errors
      }
      return next;
    });
  }, []);

  const applyAction = useCallback(
    async (item: Item, action: ActionType) => {
      if (updating) return;
      setUpdating(true);
      try {
        if (action === "read") {
          await api.markItemRead(item.id);
          showToast(t("itemDetail.toast.markRead"), "success");
        } else if (action === "favorite") {
          await api.setItemFeedback(item.id, { rating: item.feedback_rating ?? 0, is_favorite: true });
          await api.markItemRead(item.id);
          showToast(t("triage.toast.favorited"), "success");
        } else {
          showToast(t("triage.toast.later"), "success");
        }
        queryClient.setQueriesData(
          { queryKey: ["triage-queue"] },
          (prev: unknown) => {
            if (!prev || typeof prev !== "object") return prev;
            const data = prev as { items?: Array<Record<string, unknown>> };
            if (!Array.isArray(data.items)) return prev;
            return {
              ...data,
              items: data.items.map((v) =>
                v.id === item.id ? { ...v, is_read: action === "read" || action === "favorite" } : v
              ),
            };
          }
        );
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
          queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
        ]);
        recordMetric(item.id, action);
        setActioned((prev) => ({ ...prev, [item.id]: true }));
      } catch (e) {
        showToast(`${t("common.error")}: ${String(e)}`, "error");
      } finally {
        setUpdating(false);
      }
    },
    [queryClient, recordMetric, showToast, t, updating]
  );

  const metricSummary = useMemo(() => {
    const now = Date.now();
    const oneDayAgo = now - 24 * 60 * 60 * 1000;
    const recent24h = metricEvents.filter((v) => v.ts >= oneDayAgo);
    const recent20 = metricEvents.slice(-20);
    const avg24hMs =
      recent24h.length > 0
        ? Math.round(recent24h.reduce((acc, v) => acc + v.elapsed_ms, 0) / recent24h.length)
        : 0;
    const avgRecentMs =
      recent20.length > 0
        ? Math.round(recent20.reduce((acc, v) => acc + v.elapsed_ms, 0) / recent20.length)
        : 0;
    const counts: Record<ActionType, number> = { read: 0, favorite: 0, later: 0 };
    for (const v of recent24h) counts[v.action] += 1;
    const within5s24h =
      recent24h.length > 0
        ? Math.round((recent24h.filter((v) => v.elapsed_ms <= 5000).length / recent24h.length) * 100)
        : 0;
    const within5sRecent =
      recent20.length > 0
        ? Math.round((recent20.filter((v) => v.elapsed_ms <= 5000).length / recent20.length) * 100)
        : 0;
    return {
      recent24hCount: recent24h.length,
      avg24hSec: avg24hMs / 1000,
      avgRecentSec: avgRecentMs / 1000,
      within5s24h,
      within5sRecent,
      counts,
    };
  }, [metricEvents]);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (!current || updating) return;
      if (e.key === "ArrowLeft") {
        e.preventDefault();
        void applyAction(current, "read");
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        void applyAction(current, "favorite");
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        void applyAction(current, "later");
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [applyAction, current, updating]);

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    if (updating) return;
    startRef.current = { x: e.clientX, y: e.clientY };
    setDragging(true);
    setSwipeExit(null);
  };
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    if (!startRef.current || !dragging || updating) return;
    setDragOffset({
      x: e.clientX - startRef.current.x,
      y: e.clientY - startRef.current.y,
    });
  };
  const resetSwipeState = () => {
    setDragging(false);
    setDragOffset({ x: 0, y: 0 });
    setSwipeExit(null);
    startRef.current = null;
  };
  const onPointerUp = (e: PointerEvent<HTMLDivElement>) => {
    if (!current || !startRef.current || updating) {
      resetSwipeState();
      return;
    }
    const dx = e.clientX - startRef.current.x;
    const dy = e.clientY - startRef.current.y;
    const absX = Math.abs(dx);
    const absY = Math.abs(dy);
    const threshold = 50;
    const animateAndApply = (action: ActionType) => {
      setDragging(false);
      setSwipeExit(action);
      window.setTimeout(() => {
        void applyAction(current, action);
        setDragOffset({ x: 0, y: 0 });
        setSwipeExit(null);
      }, 160);
    };
    if (absX >= absY && absX > threshold) {
      if (dx < 0) animateAndApply("read");
      if (dx > 0) animateAndApply("favorite");
      startRef.current = null;
      return;
    }
    if (absY > absX && dy < -threshold) {
      animateAndApply("later");
      startRef.current = null;
      return;
    }
    resetSwipeState();
  };

  const swipeBadgeOpacity = useMemo(() => {
    const x = dragOffset.x;
    const y = dragOffset.y;
    return {
      read: Math.max(0, Math.min(1, Math.abs(x) > 20 && x < 0 ? Math.abs(x) / 120 : 0)),
      favorite: Math.max(0, Math.min(1, Math.abs(x) > 20 && x > 0 ? Math.abs(x) / 120 : 0)),
      later: Math.max(0, Math.min(1, y < -20 ? Math.abs(y) / 120 : 0)),
    };
  }, [dragOffset.x, dragOffset.y]);

  const cardTransform = useMemo(() => {
    if (swipeExit === "read") return "translate(-120%, 0) rotate(-10deg)";
    if (swipeExit === "favorite") return "translate(120%, 0) rotate(10deg)";
    if (swipeExit === "later") return "translate(0, -120%)";
    return `translate(${dragOffset.x}px, ${dragOffset.y}px) rotate(${(dragOffset.x / 18).toFixed(2)}deg)`;
  }, [dragOffset.x, dragOffset.y, swipeExit]);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="inline-flex items-center gap-2 text-2xl font-bold tracking-tight text-zinc-900">
            <Hand className="size-6 text-zinc-600" aria-hidden="true" />
            <span>{t("triage.title")}</span>
          </h1>
          <p className="mt-1 text-sm text-zinc-500">{t("triage.subtitle")}</p>
        </div>
        <Link href="/items?feed=recommended" className="rounded border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50">
          {t("triage.backToItems")}
        </Link>
      </div>

      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <div className="mb-2 flex items-center justify-between text-sm">
          <span className="text-zinc-600">
            {done}/{queue.length} {t("triage.done")}
          </span>
          <div className="flex items-center gap-2">
            <span className="text-zinc-500">{progress}%</span>
            <button
              type="button"
              onClick={() => setMobileMetricsOpen((v) => !v)}
              className="rounded border border-zinc-200 px-2 py-1 text-xs text-zinc-600 md:hidden"
            >
              {mobileMetricsOpen ? t("triage.metrics.hide") : t("triage.metrics.show")}
            </button>
          </div>
        </div>
        <div className="h-2 w-full overflow-hidden rounded-full bg-zinc-100">
          <div className="h-full rounded-full bg-zinc-900 transition-all" style={{ width: `${progress}%` }} />
        </div>
        <div
          className={`mt-3 grid gap-2 text-xs text-zinc-600 sm:grid-cols-2 lg:grid-cols-6 ${
            mobileMetricsOpen ? "grid" : "hidden md:grid"
          }`}
        >
          <div className="rounded border border-zinc-200 bg-zinc-50 px-2.5 py-2">
            <span className="text-zinc-500">{t("triage.metrics.avg24h")}</span>
            <div className="mt-0.5 text-sm font-semibold text-zinc-800">
              {metricSummary.avg24hSec > 0 ? `${metricSummary.avg24hSec.toFixed(1)}s` : "-"}
            </div>
          </div>
          <div className="rounded border border-zinc-200 bg-zinc-50 px-2.5 py-2">
            <span className="text-zinc-500">{t("triage.metrics.avgRecent")}</span>
            <div className="mt-0.5 text-sm font-semibold text-zinc-800">
              {metricSummary.avgRecentSec > 0 ? `${metricSummary.avgRecentSec.toFixed(1)}s` : "-"}
            </div>
          </div>
          <div className="rounded border border-zinc-200 bg-zinc-50 px-2.5 py-2">
            <span className="text-zinc-500">{t("triage.metrics.actions24h")}</span>
            <div className="mt-0.5 text-sm font-semibold text-zinc-800">{metricSummary.recent24hCount}</div>
          </div>
          <div className="rounded border border-zinc-200 bg-zinc-50 px-2.5 py-2">
            <span className="text-zinc-500">{t("triage.metrics.within5s24h")}</span>
            <div
              className={`mt-0.5 text-sm font-semibold ${
                metricSummary.recent24hCount > 0 ? rateTone(metricSummary.within5s24h) : "text-zinc-800"
              }`}
            >
              {metricSummary.recent24hCount > 0 ? `${metricSummary.within5s24h}%` : "-"}
            </div>
          </div>
          <div className="rounded border border-zinc-200 bg-zinc-50 px-2.5 py-2">
            <span className="text-zinc-500">{t("triage.metrics.within5sRecent")}</span>
            <div
              className={`mt-0.5 text-sm font-semibold ${
                metricEvents.length > 0 ? rateTone(metricSummary.within5sRecent) : "text-zinc-800"
              }`}
            >
              {metricEvents.length > 0 ? `${metricSummary.within5sRecent}%` : "-"}
            </div>
          </div>
          <div className="rounded border border-zinc-200 bg-zinc-50 px-2.5 py-2">
            <span className="text-zinc-500">{t("triage.metrics.breakdown")}</span>
            <div className="mt-0.5 text-sm font-semibold text-zinc-800">
              {`R ${metricSummary.counts.read} / F ${metricSummary.counts.favorite} / L ${metricSummary.counts.later}`}
            </div>
          </div>
        </div>
      </section>

      {queueQuery.isLoading && !queueQuery.data && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {queueQuery.error && <p className="text-sm text-red-500">{String(queueQuery.error)}</p>}

      {!queueQuery.isLoading && queue.length === 0 && (
        <section className="rounded-xl border border-zinc-200 bg-white p-6 text-center text-zinc-500">
          {t("triage.empty")}
        </section>
      )}

      {!queueQuery.isLoading && queue.length > 0 && !current && (
        <section className="rounded-xl border border-zinc-200 bg-white p-6 text-center">
          <p className="text-zinc-700">{t("triage.completed")}</p>
        </section>
      )}

      {current && (
        <section
          className="relative touch-none select-none rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm sm:p-6"
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={resetSwipeState}
          style={{
            transform: cardTransform,
            opacity: swipeExit ? 0.35 : 1,
            transition: dragging
              ? "none"
              : "transform 180ms ease, opacity 180ms ease",
          }}
        >
          <div className="pointer-events-none absolute inset-0">
            <div
              className="absolute left-3 top-3 rounded-full border border-green-300 bg-green-50 px-2 py-1 text-[10px] font-semibold text-green-700"
              style={{ opacity: swipeBadgeOpacity.favorite }}
            >
              {t("triage.action.favorite")}
            </div>
            <div
              className="absolute right-3 top-3 rounded-full border border-blue-300 bg-blue-50 px-2 py-1 text-[10px] font-semibold text-blue-700"
              style={{ opacity: swipeBadgeOpacity.read }}
            >
              {t("triage.action.read")}
            </div>
            <div
              className="absolute left-1/2 top-3 -translate-x-1/2 rounded-full border border-zinc-300 bg-zinc-50 px-2 py-1 text-[10px] font-semibold text-zinc-700"
              style={{ opacity: swipeBadgeOpacity.later }}
            >
              {t("triage.action.later")}
            </div>
          </div>
          <div className="mb-2 text-xs text-zinc-500">{new Date(current.published_at ?? current.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}</div>
          <h2 className="text-xl font-semibold leading-snug text-zinc-900">
            {current.translated_title || current.title || current.url}
          </h2>
          <a
            href={current.url}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-2 inline-flex items-center gap-1 break-all text-sm text-blue-600 hover:underline"
          >
            <ExternalLink className="size-3.5" aria-hidden="true" />
            <span>{current.url}</span>
          </a>

          <div className="mt-4 rounded-lg border border-zinc-100 bg-zinc-50 p-3">
            <div className="mb-1 text-xs font-semibold text-zinc-600">{t("triage.summary")}</div>
            {currentDetailQuery.isLoading ? (
              <p className="text-sm text-zinc-500">{t("common.loading")}</p>
            ) : currentDetailQuery.data?.summary?.summary ? (
              <p
                className="max-h-40 overflow-y-auto whitespace-pre-wrap text-sm leading-7 text-zinc-700 touch-auto"
                onPointerDown={(e) => e.stopPropagation()}
                onPointerMove={(e) => e.stopPropagation()}
                onPointerUp={(e) => e.stopPropagation()}
              >
                {currentDetailQuery.data.summary.summary}
              </p>
            ) : (
              <p className="text-sm text-zinc-500">{t("triage.noSummary")}</p>
            )}
          </div>

          <div className="mt-3">
            <button
              type="button"
              onClick={() => setInlineItemId(current.id)}
              className="rounded border border-zinc-300 bg-white px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50"
            >
              {t("triage.openInline")}
            </button>
          </div>

          <div className="mt-4 grid gap-2 sm:grid-cols-3">
            <button
              type="button"
              disabled={updating}
              onClick={() => void applyAction(current, "read")}
              className="inline-flex items-center justify-center gap-2 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              <span>{t("triage.action.read")}</span>
            </button>
            <button
              type="button"
              disabled={updating}
              onClick={() => void applyAction(current, "later")}
              className="inline-flex items-center justify-center gap-2 rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-50"
            >
              <ArrowUp className="size-4" aria-hidden="true" />
              <span>{t("triage.action.later")}</span>
            </button>
            <button
              type="button"
              disabled={updating}
              onClick={() => void applyAction(current, "favorite")}
              className="inline-flex items-center justify-center gap-2 rounded-lg border border-zinc-900 bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50"
            >
              <ArrowRight className="size-4" aria-hidden="true" />
              <span>{t("triage.action.favorite")}</span>
            </button>
          </div>

          <p className="mt-3 text-xs text-zinc-500">{t("triage.hint")}</p>
        </section>
      )}

      {inlineItemId && (
        <InlineReader
          open={!!inlineItemId}
          itemId={inlineItemId}
          locale={locale}
          onClose={() => setInlineItemId(null)}
          onOpenDetail={(itemId) => {
            setInlineItemId(null);
            router.push(`/items/${itemId}?from=${encodeURIComponent("/triage")}`);
          }}
          onOpenItem={(itemId) => setInlineItemId(itemId)}
          autoMarkRead={false}
          onReadToggled={(itemId, isRead) => {
            if (!isRead) return;
            setActioned((prev) => ({ ...prev, [itemId]: true }));
            void Promise.all([
              queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
              queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
            ]);
          }}
        />
      )}
    </div>
  );
}
