"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { LoaderCircle, Pause, Play, SkipForward, Square, Volume2, X } from "lucide-react";
import { api, type Item, type SummaryAudioSynthesisResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";

type QueueKind = "unread" | "later" | "favorite";

type PreparedAudio = {
  itemID: string;
  objectURL: string;
  response: SummaryAudioSynthesisResponse;
};

type PendingPrefetch = {
  itemID: string;
  promise: Promise<PreparedAudio>;
};

const queueKinds: QueueKind[] = ["unread", "later", "favorite"];
const PLAYBACK_QUEUE_BUFFER_SIZE = 24;
const PLAYBACK_QUEUE_VISIBLE_COUNT = 12;

function parseQueueKind(raw: string | null): QueueKind {
  return raw === "later" ? "later" : raw === "favorite" ? "favorite" : "unread";
}

function base64ToBlob(base64: string, contentType: string): Blob {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return new Blob([bytes], { type: contentType || "audio/mpeg" });
}

export default function SummaryAudioPlayerPage() {
  const { t, locale } = useI18n();
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const queueKind = parseQueueKind(searchParams.get("queue"));
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const currentAudioRef = useRef<PreparedAudio | null>(null);
  const prefetchedAudioRef = useRef<PreparedAudio | null>(null);
  const pendingPrefetchRef = useRef<PendingPrefetch | null>(null);
  const markedReadIDsRef = useRef<Set<string>>(new Set());
  const prefetchingItemIDRef = useRef<string | null>(null);
  const readProgressSecRef = useRef<Map<string, number>>(new Map());
  const readProgressLastStartedAtRef = useRef<number | null>(null);
  const readProgressActiveItemIDRef = useRef<string | null>(null);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [activeItemID, setActiveItemID] = useState<string | null>(null);
  const [playbackQueue, setPlaybackQueue] = useState<Item[]>([]);
  const [isPreparing, setIsPreparing] = useState(false);
  const [playbackError, setPlaybackError] = useState<string | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [isFinished, setIsFinished] = useState(false);
  const [positionSec, setPositionSec] = useState(0);
  const skipHandlerRef = useRef<(() => Promise<void>) | null>(null);
  const markReadHandlerRef = useRef<((itemID: string) => Promise<void>) | null>(null);

  const queueQuery = useQuery({
    queryKey: ["summary-audio-queue", queueKind],
    queryFn: async () => {
      const params =
        queueKind === "later"
          ? { status: "summarized", page_size: PLAYBACK_QUEUE_BUFFER_SIZE, sort: "newest", unread_only: true, later_only: true }
          : queueKind === "favorite"
            ? { status: "summarized", page_size: PLAYBACK_QUEUE_BUFFER_SIZE, sort: "newest", favorite_only: true }
            : { status: "summarized", page_size: PLAYBACK_QUEUE_BUFFER_SIZE, sort: "newest", unread_only: true };
      return api.getItems(params);
    },
  });

  const queueItems = useMemo(() => queueQuery.data?.items ?? [], [queueQuery.data?.items]);
  const currentItem = playbackQueue[0] ?? null;
  const nextItem = playbackQueue[1] ?? null;

  const currentItemDetailQuery = useQuery({
    queryKey: ["summary-audio-item", currentItem?.id],
    queryFn: async () => {
      if (!currentItem?.id) {
        throw new Error("missing item id");
      }
      return api.getItem(currentItem.id);
    },
    enabled: Boolean(currentItem?.id),
  });

  useEffect(() => {
    setCurrentIndex(0);
    setActiveItemID(null);
    setPlaybackQueue([]);
    setIsFinished(false);
    setPlaybackError(null);
    setPositionSec(0);
    markedReadIDsRef.current = new Set();
    if (audioRef.current) {
      audioRef.current.pause();
      audioRef.current.removeAttribute("src");
      audioRef.current.load();
    }
    if (currentAudioRef.current) {
      URL.revokeObjectURL(currentAudioRef.current.objectURL);
      currentAudioRef.current = null;
    }
    if (prefetchedAudioRef.current) {
      URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
      prefetchedAudioRef.current = null;
    }
    pendingPrefetchRef.current = null;
    readProgressSecRef.current = new Map();
    readProgressLastStartedAtRef.current = null;
    readProgressActiveItemIDRef.current = null;
  }, [queueKind]);

  useEffect(() => {
    setPlaybackQueue((prev) => {
      const incoming = queueItems.slice(0, PLAYBACK_QUEUE_BUFFER_SIZE);
      if (incoming.length === 0) {
        return prev;
      }
      if (prev.length === 0) {
        return incoming;
      }
      const existing = new Set(prev.map((item) => item.id));
      const appended = incoming.filter((item) => !existing.has(item.id));
      if (appended.length === 0) {
        return prev;
      }
      return [...prev, ...appended].slice(0, PLAYBACK_QUEUE_BUFFER_SIZE);
    });
  }, [queueItems]);

  useEffect(() => {
    return () => {
      if (currentAudioRef.current) {
        URL.revokeObjectURL(currentAudioRef.current.objectURL);
      }
      if (prefetchedAudioRef.current) {
        URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
      }
    };
  }, []);

  const markCurrentItemRead = useCallback(async (itemID: string) => {
    if (!itemID || markedReadIDsRef.current.has(itemID)) {
      return;
    }
    markedReadIDsRef.current.add(itemID);
    try {
      await api.markItemRead(itemID);
      void queryClient.invalidateQueries({ queryKey: ["items-feed"] });
      void queryClient.invalidateQueries({ queryKey: ["summary-audio-item", itemID] });
      void queryClient.invalidateQueries({ queryKey: ["summary-audio-queue", queueKind] });
    } catch {
      return;
    }
  }, [queryClient, queueKind]);

  function resetReadProgressForItem(itemID: string | null) {
    if (!itemID) {
      return;
    }
    readProgressSecRef.current.delete(itemID);
    if (readProgressActiveItemIDRef.current === itemID) {
      readProgressActiveItemIDRef.current = null;
      readProgressLastStartedAtRef.current = null;
    }
  }

  const flushReadProgress = useCallback(async (itemID: string | null) => {
    if (!itemID || markedReadIDsRef.current.has(itemID)) {
      return;
    }
    if (readProgressActiveItemIDRef.current !== itemID || readProgressLastStartedAtRef.current == null) {
      return;
    }
    const elapsedSec = Math.max(0, (Date.now() - readProgressLastStartedAtRef.current) / 1000);
    const nextTotalSec = (readProgressSecRef.current.get(itemID) ?? 0) + elapsedSec;
    readProgressSecRef.current.set(itemID, nextTotalSec);
    readProgressLastStartedAtRef.current = null;
    if (nextTotalSec >= 30) {
      await markCurrentItemRead(itemID);
    }
  }, [markCurrentItemRead]);

  async function synthesizeItem(itemID: string): Promise<PreparedAudio> {
    const response = await api.synthesizeSummaryAudio(itemID);
    const blob = base64ToBlob(response.audio_base64, response.content_type);
    return {
      itemID,
      objectURL: URL.createObjectURL(blob),
      response,
    };
  }

  async function ensurePrefetch(queue: Item[], index: number) {
    const item = queue[index];
    if (!item) {
      return;
    }
    if (prefetchedAudioRef.current?.itemID === item.id || pendingPrefetchRef.current?.itemID === item.id || prefetchingItemIDRef.current === item.id) {
      return;
    }
    prefetchingItemIDRef.current = item.id;
    const promise = synthesizeItem(item.id);
    pendingPrefetchRef.current = { itemID: item.id, promise };
    try {
      const prepared = await promise;
      if (prefetchedAudioRef.current) {
        URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
      }
      prefetchedAudioRef.current = prepared;
      void queryClient.prefetchQuery({
        queryKey: ["summary-audio-item", item.id],
        queryFn: () => api.getItem(item.id),
      });
    } catch {
      if (prefetchedAudioRef.current?.itemID === item.id) {
        URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
        prefetchedAudioRef.current = null;
      }
    } finally {
      if (pendingPrefetchRef.current?.itemID === item.id) {
        pendingPrefetchRef.current = null;
      }
      prefetchingItemIDRef.current = null;
    }
  }

  async function playQueue(queue: Item[], autoplay: boolean) {
    const item = queue[0];
    const audio = audioRef.current;
    if (!item || !audio) {
      return;
    }
    setPlaybackError(null);
    setIsPreparing(true);
    setIsFinished(false);
    try {
      let prepared: PreparedAudio;
      if (prefetchedAudioRef.current?.itemID === item.id) {
        prepared = prefetchedAudioRef.current;
        prefetchedAudioRef.current = null;
      } else if (pendingPrefetchRef.current?.itemID === item.id) {
        try {
          prepared = await pendingPrefetchRef.current.promise;
          if (prefetchedAudioRef.current?.itemID === item.id) {
            prefetchedAudioRef.current = null;
          }
        } catch {
          if (pendingPrefetchRef.current?.itemID === item.id) {
            pendingPrefetchRef.current = null;
          }
          prefetchingItemIDRef.current = null;
          prepared = await synthesizeItem(item.id);
        }
      } else {
        if (queue[1]) {
          void ensurePrefetch(queue, 1);
        }
        prepared = await synthesizeItem(item.id);
      }
      if (currentAudioRef.current && currentAudioRef.current.itemID !== prepared.itemID) {
        URL.revokeObjectURL(currentAudioRef.current.objectURL);
      }
      currentAudioRef.current = prepared;
      setActiveItemID(prepared.itemID);
      audio.src = prepared.objectURL;
      audio.currentTime = 0;
      audio.load();
      setPositionSec(0);
      if (autoplay) {
        await audio.play();
      }
      if (queue[1]) {
        void ensurePrefetch(queue, 1);
      }
    } catch (err) {
      setPlaybackError(err instanceof Error ? err.message : String(err));
    } finally {
      setIsPreparing(false);
    }
  }

  async function handlePrimaryPlay() {
    const audio = audioRef.current;
    if (!currentItem || !audio) {
      return;
    }
    if (currentAudioRef.current?.itemID !== currentItem.id || !audio.src) {
      await playQueue(playbackQueue, true);
      return;
    }
    try {
      await audio.play();
    } catch (err) {
      setPlaybackError(err instanceof Error ? err.message : String(err));
    }
  }

  function handlePause() {
    audioRef.current?.pause();
  }

  function handleStop() {
    const audio = audioRef.current;
    if (!audio) {
      return;
    }
    void flushReadProgress(activeItemID);
    audio.pause();
    audio.currentTime = 0;
    setPositionSec(0);
    setIsPlaying(false);
  }

  function handleFinish() {
    void flushReadProgress(activeItemID);
    handleStop();
    setIsFinished(true);
    if (currentAudioRef.current) {
      URL.revokeObjectURL(currentAudioRef.current.objectURL);
      currentAudioRef.current = null;
    }
    if (prefetchedAudioRef.current) {
      URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
      prefetchedAudioRef.current = null;
    }
    pendingPrefetchRef.current = null;
    readProgressActiveItemIDRef.current = null;
    readProgressLastStartedAtRef.current = null;
    if (audioRef.current) {
      audioRef.current.removeAttribute("src");
      audioRef.current.load();
    }
    setActiveItemID(null);
  }

  async function handleSkip() {
    if (playbackQueue.length <= 1 || !nextItem) {
      setPlaybackQueue([]);
      setCurrentIndex((prev) => prev + (playbackQueue.length > 0 ? 1 : 0));
      handleFinish();
      return;
    }
    const nextQueue = playbackQueue.slice(1);
    setPlaybackQueue(nextQueue);
    setCurrentIndex((prev) => prev + 1);
    await playQueue(nextQueue, true);
  }

  useEffect(() => {
    skipHandlerRef.current = handleSkip;
    markReadHandlerRef.current = markCurrentItemRead;
  });

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) {
      return;
    }
    const onPlay = () => {
      setIsPlaying(true);
      if (activeItemID && !markedReadIDsRef.current.has(activeItemID)) {
        readProgressActiveItemIDRef.current = activeItemID;
        readProgressLastStartedAtRef.current = Date.now();
      }
    };
    const onPause = () => {
      setIsPlaying(false);
      void flushReadProgress(activeItemID);
    };
    const onTimeUpdate = () => setPositionSec(audio.currentTime || 0);
    const onEnded = () => {
      void flushReadProgress(activeItemID);
      setIsPlaying(false);
      resetReadProgressForItem(activeItemID);
      void skipHandlerRef.current?.();
    };
    audio.addEventListener("play", onPlay);
    audio.addEventListener("pause", onPause);
    audio.addEventListener("timeupdate", onTimeUpdate);
    audio.addEventListener("ended", onEnded);
    return () => {
      void flushReadProgress(activeItemID);
      audio.removeEventListener("play", onPlay);
      audio.removeEventListener("pause", onPause);
      audio.removeEventListener("timeupdate", onTimeUpdate);
      audio.removeEventListener("ended", onEnded);
    };
  }, [activeItemID, flushReadProgress, nextItem?.id, playbackQueue.length]);

  const currentDetail = currentItemDetailQuery.data ?? null;
  const summaryText = currentDetail?.summary?.summary ?? "";
  const translatedTitle = currentDetail?.translated_title || currentDetail?.summary?.translated_title || currentDetail?.title || "";
  const originalTitle = currentDetail?.title || "";
  const queueCountLabel = `${playbackQueue.length.toLocaleString(locale)} ${t("summaryAudio.queueCount")}`;
  const titleForDisplay = translatedTitle || originalTitle || t("summaryAudio.untitled");
  const sourceTitle = currentDetail?.source_title || t("summaryAudio.sourceUnknown");
  const progressLabel = playbackQueue.length > 0 ? `${currentIndex + 1}/${currentIndex + playbackQueue.length}` : "0/0";

  return (
    <PageTransition>
      <div className="space-y-4">
        <PageHeader
          eyebrow={t("summaryAudio.eyebrow")}
          title={t("summaryAudio.title")}
          titleIcon={Volume2}
          description={t("summaryAudio.description")}
          meta={(
            <>
              <Tag tone="default">{queueCountLabel}</Tag>
              <Tag tone="default">{progressLabel}</Tag>
            </>
          )}
          actions={(
            <div className="flex flex-wrap items-center gap-2">
              <Link
                href="/items"
                className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {t("summaryAudio.backToItems")}
              </Link>
            </div>
          )}
        />

        <SectionCard>
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() => void handlePrimaryPlay()}
                disabled={!currentItem || isPreparing}
                className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] transition hover:-translate-y-0.5 hover:shadow-[0_12px_30px_rgba(15,23,42,0.12)] disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:translate-y-0 disabled:hover:shadow-none"
              >
                <Play className="size-4" aria-hidden="true" />
                <span>{isPlaying ? t("summaryAudio.resume") : t("summaryAudio.play")}</span>
              </button>
              <button
                type="button"
                onClick={handlePause}
                disabled={!currentAudioRef.current || !isPlaying}
                className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:border-[var(--color-editorial-ink-faint)] hover:bg-[var(--color-editorial-panel-strong)] hover:shadow-[0_12px_30px_rgba(15,23,42,0.08)] disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:translate-y-0 disabled:hover:border-[var(--color-editorial-line)] disabled:hover:bg-[var(--color-editorial-panel)] disabled:hover:shadow-none"
              >
                <Pause className="size-4" aria-hidden="true" />
                <span>{t("summaryAudio.pause")}</span>
              </button>
              <button
                type="button"
                onClick={handleStop}
                disabled={!currentAudioRef.current}
                className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:border-[var(--color-editorial-ink-faint)] hover:bg-[var(--color-editorial-panel-strong)] hover:shadow-[0_12px_30px_rgba(15,23,42,0.08)] disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:translate-y-0 disabled:hover:border-[var(--color-editorial-line)] disabled:hover:bg-[var(--color-editorial-panel)] disabled:hover:shadow-none"
              >
                <Square className="size-4" aria-hidden="true" />
                <span>{t("summaryAudio.stop")}</span>
              </button>
              <button
                type="button"
                onClick={() => void handleSkip()}
                disabled={!currentItem}
                className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:border-[var(--color-editorial-ink-faint)] hover:bg-[var(--color-editorial-panel-strong)] hover:shadow-[0_12px_30px_rgba(15,23,42,0.08)] disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:translate-y-0 disabled:hover:border-[var(--color-editorial-line)] disabled:hover:bg-[var(--color-editorial-panel)] disabled:hover:shadow-none"
              >
                <SkipForward className="size-4" aria-hidden="true" />
                <span>{t("summaryAudio.skip")}</span>
              </button>
              <button
                type="button"
                onClick={handleFinish}
                className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:border-[var(--color-editorial-ink-faint)] hover:bg-[var(--color-editorial-panel-strong)] hover:shadow-[0_12px_30px_rgba(15,23,42,0.08)]"
              >
                <X className="size-4" aria-hidden="true" />
                <span>{t("summaryAudio.finish")}</span>
              </button>
            </div>

            <div className="flex flex-wrap items-center gap-2 text-sm text-editorial-muted">
              <span>{`${Math.floor(positionSec)}${t("summaryAudio.positionSuffix")}`}</span>
              {isPreparing ? (
                <Tag tone="default">
                  <span className="inline-flex items-center gap-1.5">
                    <LoaderCircle className="size-3.5 animate-spin" aria-hidden="true" />
                    <span>{t("summaryAudio.preparing")}</span>
                  </span>
                </Tag>
              ) : null}
              {prefetchingItemIDRef.current ? (
                <Tag tone="default">
                  <span className="inline-flex items-center gap-1.5">
                    <LoaderCircle className="size-3.5 animate-spin" aria-hidden="true" />
                    <span>{t("summaryAudio.prefetching")}</span>
                  </span>
                </Tag>
              ) : null}
              {isFinished ? <Tag tone="default">{t("summaryAudio.finished")}</Tag> : null}
            </div>
            {playbackError ? <p className="text-sm text-red-600">{playbackError}</p> : null}
            <audio ref={audioRef} controls className="w-full" preload="auto" />
          </div>
        </SectionCard>

        <div className="grid gap-4 lg:grid-cols-[minmax(0,1.8fr)_minmax(320px,0.9fr)]">
          <SectionCard>
            <div className="space-y-4">
              <div className="space-y-2">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                  {t("summaryAudio.nowPlaying")}
                </div>
                <h2 className="font-serif text-2xl text-editorial-strong">{titleForDisplay}</h2>
                <p className="text-sm text-editorial-muted">{originalTitle || t("summaryAudio.originalTitleEmpty")}</p>
                <div className="flex flex-wrap items-center gap-2 text-sm text-editorial-muted">
                  <span>{sourceTitle}</span>
                  {currentDetail?.url ? (
                    <a
                      href={currentDetail.url}
                      target="_blank"
                      rel="noreferrer"
                      className="text-[var(--color-editorial-accent)] underline-offset-2 hover:underline"
                    >
                      {t("summaryAudio.openSource")}
                    </a>
                  ) : null}
                  {currentAudioRef.current?.response.persona ? <Tag tone="default">{currentAudioRef.current.response.persona}</Tag> : null}
                </div>
              </div>

              <div className="rounded-[var(--radius-card)] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                <p className="whitespace-pre-wrap text-sm leading-7 text-editorial-strong">
                  {summaryText || t("summaryAudio.summaryPending")}
                </p>
              </div>
            </div>
          </SectionCard>

          <SectionCard>
            <div className="space-y-3">
              <div className="flex flex-wrap items-center gap-2">
                {queueKinds.map((kind) => {
                  const active = queueKind === kind;
                  return (
                    <button
                      key={kind}
                      type="button"
                      onClick={() => router.replace(`/audio-player?queue=${kind}`)}
                      className={`inline-flex min-h-10 items-center justify-center rounded-full border px-4 py-2 text-sm font-medium ${
                        active
                          ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                          : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                      }`}
                    >
                      {t(`summaryAudio.queue.${kind}`)}
                    </button>
                  );
                })}
              </div>
              <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                {t("summaryAudio.queueTitle")}
              </div>
              {playbackQueue.length === 0 ? (
                <p className="text-sm text-editorial-muted">{t("summaryAudio.empty")}</p>
              ) : (
                <div className="space-y-2">
                  {playbackQueue.slice(0, PLAYBACK_QUEUE_VISIBLE_COUNT).map((item, index) => {
                    const isActive = activeItemID === item.id;
                    const queueSourceTitle = item.source_title || t("summaryAudio.sourceUnknown");
                    return (
                      <button
                        key={item.id}
                        type="button"
                        onClick={() => {
                          const nextQueue = playbackQueue.slice(index);
                          setPlaybackQueue(nextQueue);
                          setCurrentIndex((prev) => prev + index);
                          void playQueue(nextQueue, true);
                        }}
                        className={`group flex w-full items-start justify-between gap-3 rounded-[var(--radius-card)] border px-4 py-3 text-left transition hover:-translate-y-0.5 hover:shadow-[0_12px_30px_rgba(15,23,42,0.08)] focus:outline-none focus:ring-2 focus:ring-[var(--color-editorial-accent)] ${
                          isActive
                            ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-accent-soft)]"
                            : "cursor-pointer border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] hover:border-[var(--color-editorial-ink-faint)] hover:bg-[var(--color-editorial-panel-strong)]"
                        }`}
                      >
                        <div className="flex min-w-0 flex-1 flex-col items-start">
                          <span className="text-xs font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                            {index + 1}
                          </span>
                          <span className="mt-1 text-sm font-semibold text-editorial-strong">
                            {item.translated_title || item.title || t("summaryAudio.untitled")}
                          </span>
                          <span className="mt-1 text-xs text-editorial-muted">{queueSourceTitle}</span>
                        </div>
                        <span
                          className={`inline-flex size-9 shrink-0 items-center justify-center rounded-full border transition ${
                            isActive
                              ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                              : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink-faint)] group-hover:border-[var(--color-editorial-ink)] group-hover:text-[var(--color-editorial-ink)]"
                          }`}
                        >
                          <Play className="size-4 translate-x-[1px]" aria-hidden="true" />
                        </span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </SectionCard>
        </div>
      </div>
    </PageTransition>
  );
}
