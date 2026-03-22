"use client";

import { type PointerEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCheck, ArrowLeft, ArrowRight, ArrowUp, ExternalLink } from "lucide-react";
import { api, Item, TriageBundle, TriageQueueEntry, TriageQueueResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { InlineReader } from "@/components/inline-reader";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";

type ActionType = "read" | "favorite" | "later";
type TriageMode = "quick" | "all";
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

function entryKey(entry: TriageQueueEntry) {
  if (entry.entry_type === "bundle") return `bundle:${entry.bundle?.id ?? "unknown"}`;
  return `item:${entry.item?.id ?? "unknown"}`;
}

function entryItemIds(entry: TriageQueueEntry) {
  if (entry.entry_type === "bundle") {
    return (entry.bundle?.items ?? []).map((item) => item.id);
  }
  return entry.item?.id ? [entry.item.id] : [];
}

function isEntryDone(entry: TriageQueueEntry, actioned: Record<string, true>) {
  const itemIds = entryItemIds(entry);
  if (itemIds.length === 0) return true;
  return itemIds.every((id) => actioned[id]);
}

function leadItem(entry: TriageQueueEntry) {
  return entry.entry_type === "bundle" ? (entry.bundle?.representative ?? null) : (entry.item ?? null);
}

function sourceCount(bundle: TriageBundle) {
  return new Set((bundle.items ?? []).map((item) => item.source_id).filter(Boolean)).size;
}

export default function TriagePage() {
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [triageMode, setTriageMode] = useState<TriageMode>("quick");
  const [actioned, setActioned] = useState<Record<string, true>>({});
  const [updating, setUpdating] = useState(false);
  const [inlineItemId, setInlineItemId] = useState<string | null>(null);
  const [metricEvents, setMetricEvents] = useState<TriageMetricEvent[]>([]);
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 });
  const [dragging, setDragging] = useState(false);
  const [swipeExit, setSwipeExit] = useState<ActionType | null>(null);
  const [mobileMetricsOpen, setMobileMetricsOpen] = useState(false);
  const [bundleExpanded, setBundleExpanded] = useState(false);
  const startRef = useRef<{ x: number; y: number } | null>(null);
  const currentShownAtRef = useRef<{ id: string; shownAtMs: number } | null>(null);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const applyFromUrl = () => {
      const mode = new URLSearchParams(window.location.search).get("mode");
      setTriageMode(mode === "all" ? "all" : "quick");
    };
    applyFromUrl();
    window.addEventListener("popstate", applyFromUrl);
    return () => window.removeEventListener("popstate", applyFromUrl);
  }, []);

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

  const queueQuery = useQuery<TriageQueueResponse>({
    queryKey: ["triage-queue", triageMode, focusWindow, focusSize, diversifyTopics ? 1 : 0] as const,
    queryFn: () =>
      api.getTriageQueue({
        mode: triageMode,
        window: triageMode === "all" ? undefined : focusWindow === "today_jst" || focusWindow === "7d" ? focusWindow : "24h",
        size: triageMode === "all" ? undefined : focusSize,
        diversify_topics: triageMode === "all" ? undefined : diversifyTopics,
        exclude_later: true,
      }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });

  const queue = useMemo(() => queueQuery.data?.entries ?? [], [queueQuery.data?.entries]);
  const pendingItems = useMemo(
    () => queue.filter((entry) => !isEntryDone(entry, actioned)),
    [actioned, queue]
  );
  const inlineQueueItemIds = useMemo(() => pendingItems.flatMap((entry) => entryItemIds(entry)), [pendingItems]);
  const current = pendingItems[0] ?? null;
  const done = queue.length - pendingItems.length;
  const progress = queue.length > 0 ? Math.round((done / queue.length) * 100) : 0;
  const currentLead = current ? leadItem(current) : null;
  const currentBundle = current?.entry_type === "bundle" ? current.bundle ?? null : null;

  const currentDetailQuery = useQuery({
    queryKey: ["item-detail", currentLead?.id ?? ""],
    queryFn: () => api.getItem(currentLead?.id ?? ""),
    enabled: !!currentLead?.id,
    staleTime: 60_000,
  });

  useEffect(() => {
    const id = current ? entryKey(current) : "";
    if (!id) return;
    currentShownAtRef.current = { id, shownAtMs: Date.now() };
    setDragOffset({ x: 0, y: 0 });
    setDragging(false);
    setSwipeExit(null);
    setBundleExpanded(false);
  }, [current]);

  const recordMetric = useCallback((entryID: string, action: ActionType) => {
    const shown = currentShownAtRef.current;
    if (!shown || shown.id !== entryID) return;
    const elapsed = Math.max(0, Date.now() - shown.shownAtMs);
    const event: TriageMetricEvent = {
      ts: Date.now(),
      item_id: entryID,
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
          await api.markItemLater(item.id);
          showToast(t("triage.toast.later"), "success");
        }
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: ["triage-queue"] }),
          queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
          queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
        ]);
        recordMetric(`item:${item.id}`, action);
        setActioned((prev) => ({ ...prev, [item.id]: true }));
      } catch (e) {
        showToast(`${t("common.error")}: ${String(e)}`, "error");
      } finally {
        setUpdating(false);
      }
    },
    [queryClient, recordMetric, showToast, t, updating]
  );

  const applyBundleAction = useCallback(
    async (bundle: TriageBundle, action: "read" | "later") => {
      if (updating) return;
      const itemIds = Array.from(new Set((bundle.items ?? []).map((item) => item.id).filter(Boolean)));
      if (itemIds.length === 0) return;
      setUpdating(true);
      try {
        if (action === "read") {
          const res = await api.markItemsReadByIDs(itemIds);
          showToast(`${res.updated_count}${locale === "ja" ? "" : " "}${t("briefing.clusterReadDone")}`, "success");
        } else {
          const res = await api.markItemsLaterBulk({ item_ids: itemIds });
          showToast(`${res.updated_count}${locale === "ja" ? "" : " "}${t("briefing.clusterLaterDone")}`, "success");
        }
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: ["triage-queue"] }),
          queryClient.invalidateQueries({ queryKey: ["briefing-today"] }),
          queryClient.invalidateQueries({ queryKey: ["items-feed"] }),
        ]);
        recordMetric(`bundle:${bundle.id}`, action);
        setActioned((prev) => {
          const next = { ...prev };
          for (const id of itemIds) next[id] = true;
          return next;
        });
      } catch (e) {
        showToast(`${t("common.error")}: ${String(e)}`, "error");
      } finally {
        setUpdating(false);
      }
    },
    [locale, queryClient, recordMetric, showToast, t, updating]
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
        if (current.entry_type === "bundle" && current.bundle) {
          void applyBundleAction(current.bundle, "read");
        } else if (current.item) {
          void applyAction(current.item, "read");
        }
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        if (current.entry_type === "bundle") {
          setBundleExpanded((v) => !v);
        } else if (current.item) {
          void applyAction(current.item, "favorite");
        }
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        if (current.entry_type === "bundle" && current.bundle) {
          void applyBundleAction(current.bundle, "later");
        } else if (current.item) {
          void applyAction(current.item, "later");
        }
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [applyAction, applyBundleAction, current, updating]);

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
        if (current.entry_type === "bundle" && current.bundle) {
          if (action === "read" || action === "later") {
            void applyBundleAction(current.bundle, action);
          } else {
            setBundleExpanded(true);
          }
        } else if (current.item) {
          void applyAction(current.item, action);
        }
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

  const changeMode = useCallback(
    (nextMode: TriageMode) => {
      if (nextMode === triageMode) return;
      setTriageMode(nextMode);
      setActioned({});
      const next = nextMode === "all" ? "/triage?mode=all" : "/triage";
      router.replace(next, { scroll: false });
    },
    [router, triageMode]
  );

  return (
    <PageTransition>
      <div className="space-y-5 overflow-x-hidden">
        <PageHeader
          title={t("triage.title")}
          titleIcon={CheckCheck}
          description={triageMode === "all" ? t("triage.subtitleAll") : t("triage.subtitle")}
          compact
          actions={
            <Link
              href={triageMode === "all" ? "/items?feed=all" : "/items?feed=recommended"}
              className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
            >
              {triageMode === "all" ? t("triage.backToAllItems") : t("triage.backToItems")}
            </Link>
          }
        />

        <section className="grid gap-4 xl:grid-cols-[220px_minmax(0,1fr)]">
          <aside className="surface-editorial hidden rounded-[24px] p-4 xl:block">
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => changeMode("quick")}
                className={`rounded-full px-3 py-2 text-xs font-medium transition-colors ${
                  triageMode === "quick"
                    ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                    : "border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                }`}
              >
                {t("triage.mode.quick")}
              </button>
              <button
                type="button"
                onClick={() => changeMode("all")}
                className={`rounded-full px-3 py-2 text-xs font-medium transition-colors ${
                  triageMode === "all"
                    ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                    : "border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                }`}
              >
                {t("triage.mode.all")}
              </button>
            </div>
            <p className="mt-3 text-xs leading-6 text-[var(--color-editorial-ink-faint)]">
              {triageMode === "all" ? t("triage.subtitleAll") : t("triage.subtitle")}
            </p>
          </aside>

          <section className="surface-editorial rounded-[18px] p-3 sm:rounded-[24px] sm:p-4">
            <div className="grid gap-2.5 md:grid-cols-2 xl:grid-cols-[1.2fr_repeat(5,minmax(0,1fr))] sm:gap-3">
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 sm:rounded-[20px] sm:px-4 sm:py-3">
                <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)] sm:text-[11px] sm:tracking-[0.14em]">
                  Progress
                </div>
                <div className="mt-1.5 text-[18px] leading-none text-[var(--color-editorial-ink)] sm:mt-2 sm:text-[22px]">
                  {done}/{queue.length}
                </div>
                <div className="mt-1 text-[11px] text-[var(--color-editorial-ink-soft)] sm:text-xs">
                  {progress}% {t("triage.done")}
                </div>
                <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-[#ece4d6] sm:mt-3 sm:h-2">
                  <div className="h-full rounded-full bg-[var(--color-editorial-ink)] transition-all" style={{ width: `${progress}%` }} />
                </div>
              </div>

              <div className={`grid gap-2.5 md:col-span-1 md:gap-3 xl:col-span-5 xl:grid-cols-5 xl:gap-3.5 ${mobileMetricsOpen ? "grid" : "hidden md:grid"}`}>
                <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 sm:rounded-[20px] sm:px-4 sm:py-3">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("triage.metrics.avg24h")}</div>
                  <div className="mt-2 text-lg leading-none text-[var(--color-editorial-ink)]">
                    {metricSummary.avg24hSec > 0 ? `${metricSummary.avg24hSec.toFixed(1)}s` : "-"}
                  </div>
                </div>
                <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 sm:rounded-[20px] sm:px-4 sm:py-3">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("triage.metrics.avgRecent")}</div>
                  <div className="mt-2 text-lg leading-none text-[var(--color-editorial-ink)]">
                    {metricSummary.avgRecentSec > 0 ? `${metricSummary.avgRecentSec.toFixed(1)}s` : "-"}
                  </div>
                </div>
                <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 sm:rounded-[20px] sm:px-4 sm:py-3">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("triage.metrics.actions24h")}</div>
                  <div className="mt-2 text-lg leading-none text-[var(--color-editorial-ink)]">{metricSummary.recent24hCount}</div>
                </div>
                <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 sm:rounded-[20px] sm:px-4 sm:py-3">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("triage.metrics.within5s24h")}</div>
                  <div className={`mt-2 text-lg leading-none ${metricSummary.recent24hCount > 0 ? rateTone(metricSummary.within5s24h) : "text-[var(--color-editorial-ink)]"}`}>
                    {metricSummary.recent24hCount > 0 ? `${metricSummary.within5s24h}%` : "-"}
                  </div>
                </div>
                <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2.5 sm:rounded-[20px] sm:px-4 sm:py-3">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("triage.metrics.breakdown")}</div>
                  <div className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink)]">
                    {`R ${metricSummary.counts.read} / F ${metricSummary.counts.favorite} / L ${metricSummary.counts.later}`}
                  </div>
                </div>
              </div>
            </div>

            <div className="mt-2.5 flex items-center justify-between gap-2 md:hidden">
              <div className="inline-flex items-center gap-1 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-1">
                <button
                  type="button"
                  onClick={() => changeMode("quick")}
                  className={`rounded-full px-3 py-1.5 text-[11px] font-medium transition-colors ${
                    triageMode === "quick"
                      ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                      : "text-[var(--color-editorial-ink-soft)]"
                  }`}
                >
                  {t("triage.mode.quick")}
                </button>
                <button
                  type="button"
                  onClick={() => changeMode("all")}
                  className={`rounded-full px-3 py-1.5 text-[11px] font-medium transition-colors ${
                    triageMode === "all"
                      ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                      : "text-[var(--color-editorial-ink-soft)]"
                  }`}
                >
                  {t("triage.mode.all")}
                </button>
              </div>
              <button
                type="button"
                onClick={() => setMobileMetricsOpen((v) => !v)}
                className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1.5 text-[11px] font-medium text-[var(--color-editorial-ink-soft)]"
              >
                {mobileMetricsOpen ? t("triage.metrics.hide") : t("triage.metrics.show")}
              </button>
            </div>
          </section>
        </section>

        {queueQuery.isLoading && !queueQuery.data && <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("common.loading")}</p>}
        {queueQuery.error && <p className="text-sm text-[var(--color-editorial-error)]">{String(queueQuery.error)}</p>}

        {!queueQuery.isLoading && queue.length === 0 && (
          <section className="surface-editorial rounded-[26px] px-6 py-10 text-center text-[var(--color-editorial-ink-faint)]">
            {t("triage.empty")}
          </section>
        )}

        {!queueQuery.isLoading && queue.length > 0 && !current && (
          <section className="surface-editorial rounded-[26px] px-6 py-10 text-center">
            <p className="text-[var(--color-editorial-ink-soft)]">{t("triage.completed")}</p>
          </section>
        )}

        {current && currentLead && (
          <section
            className="relative touch-none select-none overflow-hidden rounded-[30px] border border-[var(--color-editorial-line)] bg-[#fbf8f2] shadow-[var(--shadow-card)]"
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onPointerCancel={resetSwipeState}
            style={{
              transform: cardTransform,
              opacity: swipeExit ? 0.35 : 1,
              transition: dragging ? "none" : "transform 180ms ease, opacity 180ms ease",
            }}
          >
            <div className="pointer-events-none absolute inset-0">
              <div
                className="absolute left-4 top-4 rounded-full border border-green-300 bg-green-50 px-2.5 py-1 text-[10px] font-semibold text-green-700"
                style={{ opacity: swipeBadgeOpacity.favorite }}
              >
                {currentBundle ? t("triage.action.inspectBundle") : t("triage.action.favorite")}
              </div>
              <div
                className="absolute right-4 top-4 rounded-full border border-blue-300 bg-blue-50 px-2.5 py-1 text-[10px] font-semibold text-blue-700"
                style={{ opacity: swipeBadgeOpacity.read }}
              >
                {t("triage.action.read")}
              </div>
              <div
                className="absolute left-1/2 top-4 -translate-x-1/2 rounded-full border border-zinc-300 bg-zinc-50 px-2.5 py-1 text-[10px] font-semibold text-zinc-700"
                style={{ opacity: swipeBadgeOpacity.later }}
              >
                {t("triage.action.later")}
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-editorial-line)] bg-[rgba(250,246,238,0.95)] px-4 py-3 md:px-6 md:py-4">
              <div className="flex flex-wrap items-center gap-2 text-[11px] md:text-xs">
                {currentBundle ? (
                  <>
                    <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[var(--color-editorial-ink-soft)]">
                      {currentBundle.items.length} {t("triage.bundleCount")}
                    </span>
                    <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[var(--color-editorial-ink-soft)]">
                      {sourceCount(currentBundle)} {t("triage.bundleSources")}
                    </span>
                    {(currentBundle.shared_topics ?? []).slice(0, 3).map((topic) => (
                      <span key={topic} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[var(--color-editorial-ink-soft)]">
                        {topic}
                      </span>
                    ))}
                  </>
                ) : (
                  <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[var(--color-editorial-ink-soft)]">
                    {new Date(currentLead.published_at ?? currentLead.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}
                  </span>
                )}
              </div>
              <div className="flex flex-wrap items-center gap-2 text-[11px] md:text-xs">
                {currentLead.summary_score != null ? (
                  <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[var(--color-editorial-ink-soft)]">
                    score {currentLead.summary_score.toFixed(2)}
                  </span>
                ) : null}
              </div>
            </div>

            <div className="bg-[linear-gradient(180deg,rgba(255,255,255,0.72),rgba(255,253,249,0.96))] px-4 py-5 md:px-6 md:py-7">
              <div className="text-[11px] uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                {new Date(currentLead.published_at ?? currentLead.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}
              </div>
              <h2 className="mt-2.5 font-serif text-[26px] leading-[1.25] text-[var(--color-editorial-ink)] md:mt-3 md:text-[38px]">
                {currentLead.translated_title || currentLead.title || currentLead.url}
              </h2>
              <a
                href={currentLead.url}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-2 inline-flex items-center gap-1 break-all text-[13px] text-[var(--color-editorial-accent)] hover:underline md:mt-3 md:text-sm"
              >
                <ExternalLink className="size-3.5" aria-hidden="true" />
                <span>{currentLead.url}</span>
              </a>

              <div className="mt-4 rounded-[24px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.72)] px-4 py-4 md:mt-5 md:px-5">
                <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                  {t("triage.summary")}
                </div>
                {currentDetailQuery.isLoading ? (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("common.loading")}</p>
                ) : currentDetailQuery.data?.summary?.summary ? (
                  <p
                    className="max-h-48 overflow-y-auto whitespace-pre-wrap text-[15px] leading-[1.95] text-[var(--color-editorial-ink-soft)] touch-auto"
                    onPointerDown={(e) => e.stopPropagation()}
                    onPointerMove={(e) => e.stopPropagation()}
                    onPointerUp={(e) => e.stopPropagation()}
                  >
                    {currentDetailQuery.data.summary.summary}
                  </p>
                ) : currentBundle?.summary ? (
                  <p className="text-[15px] leading-[1.95] text-[var(--color-editorial-ink-soft)]">{currentBundle.summary}</p>
                ) : (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("triage.noSummary")}</p>
                )}
              </div>

              <div className="mt-4">
                <button
                  type="button"
                  onClick={() => (currentBundle ? setBundleExpanded((v) => !v) : setInlineItemId(currentLead.id))}
                  className="inline-flex min-h-11 items-center justify-center rounded-full border border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                >
                  {currentBundle ? t("triage.action.inspectBundle") : t("triage.openInline")}
                </button>
              </div>

              {currentBundle ? (
                <div className="mt-4 rounded-[24px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4 md:px-5">
                  <div className="flex items-center justify-between gap-3">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("triage.bundleMembers")}</div>
                    <button
                      type="button"
                      onClick={() => setBundleExpanded((v) => !v)}
                      className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                    >
                      {bundleExpanded ? t("triage.bundleCollapse") : t("triage.bundleExpand")}
                    </button>
                  </div>
                  {bundleExpanded ? (
                    <div className="mt-3 space-y-3">
                      {currentBundle.items.map((item) => (
                        <div key={item.id} className="rounded-[20px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                          <div className="text-sm font-semibold leading-7 text-[var(--color-editorial-ink)]">
                            {item.translated_title || item.title || item.url}
                          </div>
                          <div className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">{item.source_title || item.url}</div>
                          <div className="mt-3 flex flex-wrap gap-2">
                            <button
                              type="button"
                              onClick={() => setInlineItemId(item.id)}
                              className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                            >
                              {t("triage.bundleOpenItem")}
                            </button>
                            <Link
                              href={`/items/${item.id}?from=${encodeURIComponent("/triage")}`}
                              className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                            >
                              {t("triage.bundleOpenDetail")}
                            </Link>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="mt-3 text-xs text-[var(--color-editorial-ink-faint)]">{t("triage.bundleCollapsedHint")}</p>
                  )}
                </div>
              ) : null}

              <div className="mt-4 grid gap-3 sm:grid-cols-3">
                <button
                  type="button"
                  disabled={updating}
                  onClick={() =>
                    currentBundle ? void applyBundleAction(currentBundle, "read") : current.item ? void applyAction(current.item, "read") : undefined
                  }
                  className="inline-flex min-h-14 items-center justify-center gap-2 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-semibold text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:opacity-50"
                >
                  <ArrowLeft className="size-4" aria-hidden="true" />
                  <span>{t("triage.action.read")}</span>
                </button>
                <button
                  type="button"
                  disabled={updating}
                  onClick={() =>
                    currentBundle ? void applyBundleAction(currentBundle, "later") : current.item ? void applyAction(current.item, "later") : undefined
                  }
                  className="inline-flex min-h-14 items-center justify-center gap-2 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-semibold text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:opacity-50"
                >
                  <ArrowUp className="size-4" aria-hidden="true" />
                  <span>{t("triage.action.later")}</span>
                </button>
                <button
                  type="button"
                  disabled={updating}
                  onClick={() =>
                    currentBundle
                      ? setBundleExpanded((v) => !v)
                      : current.item
                        ? void applyAction(current.item, "favorite")
                        : undefined
                  }
                  className="inline-flex min-h-14 items-center justify-center gap-2 rounded-[18px] border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 text-sm font-semibold text-[var(--color-editorial-panel-strong)] hover:opacity-95 disabled:opacity-50"
                >
                  <ArrowRight className="size-4" aria-hidden="true" />
                  <span>{currentBundle ? t("triage.action.inspectBundle") : t("triage.action.favorite")}</span>
                </button>
              </div>

              <p className="mt-4 text-xs text-[var(--color-editorial-ink-faint)]">{currentBundle ? t("triage.hintBundle") : t("triage.hint")}</p>
            </div>
          </section>
        )}

        {inlineItemId && (
          <InlineReader
            open={!!inlineItemId}
            itemId={inlineItemId}
            locale={locale}
            queueItemIds={inlineQueueItemIds}
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
    </PageTransition>
  );
}
