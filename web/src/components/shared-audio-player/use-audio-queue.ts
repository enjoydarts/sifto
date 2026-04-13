"use client";

import { useEffect, useEffectEvent, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Item, type ItemDetail } from "@/lib/api";
import {
  base64ToBlob,
  isPlaybackPermissionError,
  preparedSummaryItemDetail,
  preparedSummaryPreprocessedText,
  resolvedAudioDuration,
  sameSummaryItemDetail,
  waitForLoadedMetadata,
} from "./use-audio-playback";
import type {
  SharedPlaybackState,
  SharedSummaryQueueState,
  SummaryAudioPendingPrefetch,
  SummaryAudioPrepared,
  SummaryAudioQueueKind,
} from "./types";

export const PLAYBACK_QUEUE_BUFFER_SIZE = 24;
export const PLAYBACK_QUEUE_VISIBLE_COUNT = 12;

function summaryQueueParamsForKind(queueKind: SummaryAudioQueueKind): Parameters<typeof api.getItems>[0] | null {
  if (queueKind === "brief") return null;
  return queueKind === "later"
    ? { status: "summarized", page_size: PLAYBACK_QUEUE_BUFFER_SIZE, sort: "newest", unread_only: true, later_only: true }
    : queueKind === "favorite"
      ? { status: "summarized", page_size: PLAYBACK_QUEUE_BUFFER_SIZE, sort: "newest", favorite_only: true }
      : { status: "summarized", page_size: PLAYBACK_QUEUE_BUFFER_SIZE, sort: "newest", unread_only: true };
}

function parseSummaryViewQuery(queueQuery: string): Parameters<typeof api.getItems>[0] {
  const params = new URLSearchParams(queueQuery);
  const feed = params.get("feed");
  const filter = params.get("status");
  const pendingMode = feed === "pending";
  const deletedMode = feed === "deleted" || filter === "deleted";
  const laterMode = feed === "later";
  const unreadMode = feed === "unread";
  const readMode = feed === "read";
  const searchQuery = (params.get("q") ?? "").trim();
  const searchMode = (params.get("search_mode") ?? "").trim();
  const sourceID = (params.get("source_id") ?? "").trim();
  const topic = (params.get("topic") ?? "").trim();
  const sort = params.get("sort") || (unreadMode ? "personal_score" : "newest");
  const unreadOnly = !pendingMode && !deletedMode && (unreadMode || params.get("unread") === "1" || laterMode);
  const favoriteOnly = !pendingMode && !deletedMode && params.get("favorite") === "1";
  return {
    status: deletedMode ? "deleted" : filter || (pendingMode ? "pending" : "summarized"),
    ...(sourceID ? { source_id: sourceID } : {}),
    ...(topic ? { topic } : {}),
    ...(searchQuery ? { q: searchQuery } : {}),
    ...(searchQuery && searchMode ? { search_mode: searchMode } : {}),
    sort: pendingMode ? "newest" : sort,
    unread_only: unreadOnly,
    read_only: pendingMode || deletedMode ? false : readMode,
    favorite_only: favoriteOnly,
    later_only: pendingMode || deletedMode ? false : laterMode,
  };
}

export async function fetchSummaryQueue(queueKind: SummaryAudioQueueKind, queueQuery?: string | null, excludedItemIDs?: string[]): Promise<Item[]> {
  if (queueKind === "brief") return [];
  if (queueKind !== "view") {
    const params = summaryQueueParamsForKind(queueKind);
    const response = await api.getItems(params ?? undefined);
    return response.items;
  }
  if (!queueQuery) return [];
  const baseParams = parseSummaryViewQuery(queueQuery);
  const excluded = new Set(excludedItemIDs ?? []);
  const items: Item[] = [];
  let page = 1;
  let hasNext = true;
  while (hasNext && items.length < PLAYBACK_QUEUE_BUFFER_SIZE) {
    const response = await api.getItems({ ...baseParams, page, page_size: 200 });
    for (const item of response.items) {
      if (excluded.has(item.id)) continue;
      items.push(item);
      if (items.length >= PLAYBACK_QUEUE_BUFFER_SIZE) break;
    }
    hasNext = response.has_next;
    page += 1;
  }
  return items;
}

export function createEmptySummaryQueue(): SharedSummaryQueueState {
  return {
    queueKind: null,
    queueQuery: null,
    queue: [],
    currentItemID: null,
    currentItemDetail: null,
    currentPreprocessedText: null,
    currentIndex: 0,
    excludedItemIDs: [],
    prefetchedItemID: null,
    prefetchingItemID: null,
  };
}

type PlaybackDeps = {
  audioRef: React.MutableRefObject<HTMLAudioElement | null>;
  currentAudioRef: React.MutableRefObject<SummaryAudioPrepared | null>;
  stoppingPlaybackRef: React.MutableRefObject<boolean>;
  playbackState: SharedPlaybackState;
  setPlaybackState: React.Dispatch<React.SetStateAction<SharedPlaybackState>>;
  setCurrentTimeSec: React.Dispatch<React.SetStateAction<number>>;
  setDurationSec: React.Dispatch<React.SetStateAction<number>>;
  setErrorMessage: React.Dispatch<React.SetStateAction<string | null>>;
};

type SessionDeps = {
  persistRemoteSession: (
    kind: "update" | "complete" | "interrupt",
    options?: {
      summaryQueueState?: SharedSummaryQueueState;
      positionSec?: number;
      durationSec?: number;
    },
  ) => Promise<void>;
  interruptRemoteSessionIfNeeded: () => Promise<void>;
  createSummaryPlaybackSession: (
    queueKind: SummaryAudioQueueKind,
    queueQuery: string | null,
    queue: Item[],
    currentIndex: number,
    excludedItemIDs: string[],
    offsetSec: number,
  ) => Promise<void>;
  remoteSessionIDRef: React.MutableRefObject<string | null>;
  lastPersistedPositionSecRef: React.MutableRefObject<number>;
};

export function useAudioQueue(
  playback: PlaybackDeps,
  session: SessionDeps,
  getMode: () => "summary_queue" | "audio_briefing" | null,
  getSummaryAudioSettingsLoaded: () => boolean,
  getSummaryAudioConfigured: () => boolean,
  getDurationSec: () => number,
  getCurrentTimeSec: () => number,
) {
  const queryClient = useQueryClient();
  const prefetchedAudioRef = useRef<SummaryAudioPrepared | null>(null);
  const pendingPrefetchRef = useRef<SummaryAudioPendingPrefetch | null>(null);
  const summaryQueueRef = useRef<SharedSummaryQueueState>(createEmptySummaryQueue());
  const summaryPlaybackRequestSeqRef = useRef(0);
  const summaryPlaybackPreparingRef = useRef(false);
  const markedReadIDsRef = useRef<Set<string>>(new Set());
  const readProgressSecRef = useRef<Map<string, number>>(new Map());
  const readProgressLastStartedAtRef = useRef<number | null>(null);
  const readProgressActiveItemIDRef = useRef<string | null>(null);
  const [summaryQueue, setSummaryQueue] = useState<SharedSummaryQueueState>(() => createEmptySummaryQueue());

  useEffect(() => {
    summaryQueueRef.current = summaryQueue;
  }, [summaryQueue]);

  const summaryQueueQuery = useQuery({
    queryKey: ["shared-summary-audio-queue", summaryQueue.queueKind, summaryQueue.queueQuery],
    queryFn: async () => {
      if (!summaryQueue.queueKind || summaryQueue.queueKind === "view") return [];
      return fetchSummaryQueue(summaryQueue.queueKind, summaryQueue.queueQuery);
    },
    enabled: Boolean(summaryQueue.queueKind && summaryQueue.queueKind !== "view"),
  });

  const currentSummaryItem = useMemo(() => {
    if (!summaryQueue.currentItemID) return summaryQueue.queue[0] ?? null;
    return summaryQueue.queue.find((item) => item.id === summaryQueue.currentItemID) ?? summaryQueue.queue[0] ?? null;
  }, [summaryQueue.currentItemID, summaryQueue.queue]);

  const currentSummaryItemID = currentSummaryItem?.id ?? summaryQueue.currentItemID ?? null;

  const currentSummaryDetailQuery = useQuery({
    queryKey: ["shared-summary-audio-item", currentSummaryItemID],
    queryFn: async () => {
      if (!currentSummaryItemID) throw new Error("missing item id");
      return api.getItem(currentSummaryItemID);
    },
    enabled: getMode() === "summary_queue" && Boolean(currentSummaryItemID),
  });

  const syncDetailToQueue = useEffectEvent(() => {
    if (getMode() !== "summary_queue") return;
    const detail = currentSummaryDetailQuery.data ?? null;
    setSummaryQueue((prev) => {
      const nextDetail = detail && detail.id === currentSummaryItemID ? detail : null;
      if (!nextDetail) return prev;
      const expectedItemID = prev.currentItemID ?? prev.queue[0]?.id ?? null;
      if (expectedItemID !== currentSummaryItemID) return prev;
      if (sameSummaryItemDetail(prev.currentItemDetail as ItemDetail | null, nextDetail as ItemDetail | null)) return prev;
      return { ...prev, currentItemDetail: nextDetail };
    });
  });

  useEffect(() => { syncDetailToQueue(); }, [currentSummaryDetailQuery.data, currentSummaryItemID]);

  const syncQueueDataToQueue = useEffectEvent(() => {
    if (getMode() !== "summary_queue") return;
    const incoming = summaryQueueQuery.data ?? [];
    if (incoming.length === 0) return;
    setSummaryQueue((prev) => {
      if (!prev.queueKind) return prev;
      const excluded = new Set(prev.excludedItemIDs);
      const filteredIncoming = incoming.filter((item) => !excluded.has(item.id));
      if (filteredIncoming.length === 0) return prev;
      if (prev.queue.length === 0) {
        if (prev.currentIndex > 0) return prev;
        return { ...prev, queue: filteredIncoming.slice(0, PLAYBACK_QUEUE_BUFFER_SIZE) };
      }
      const existing = new Set(prev.queue.map((item) => item.id));
      const appended = filteredIncoming.filter((item) => !existing.has(item.id));
      if (appended.length === 0) return prev;
      return { ...prev, queue: [...prev.queue, ...appended].slice(0, PLAYBACK_QUEUE_BUFFER_SIZE) };
    });
  });

  useEffect(() => { syncQueueDataToQueue(); }, [summaryQueueQuery.data]);

  const triggerPrefetchIfNeeded = useEffectEvent(() => {
    if (getMode() !== "summary_queue") return;
    const ps = playback.playbackState;
    if (ps !== "playing" && ps !== "paused" && ps !== "preparing") return;
    if (summaryQueue.queue.length < 2) return;
    if (prefetchedAudioRef.current?.itemID === summaryQueue.queue[1]?.id) return;
    if (pendingPrefetchRef.current?.itemID === summaryQueue.queue[1]?.id) return;
    void ensureSummaryPrefetch(summaryQueue.queue, 1);
  });

  useEffect(() => { triggerPrefetchIfNeeded(); }, [summaryQueue.queue]);

  async function markItemRead(itemID: string) {
    if (!itemID || markedReadIDsRef.current.has(itemID)) return;
    markedReadIDsRef.current.add(itemID);
    try {
      await api.markItemRead(itemID);
      void queryClient.invalidateQueries({ queryKey: ["items-feed"] });
      void queryClient.invalidateQueries({ queryKey: ["summary-audio-item", itemID] });
      void queryClient.invalidateQueries({ queryKey: ["shared-summary-audio-item", itemID] });
      void queryClient.invalidateQueries({ queryKey: ["summary-audio-queue"] });
      void queryClient.invalidateQueries({ queryKey: ["shared-summary-audio-queue"] });
    } catch { /* noop */ }
  }

  function resetReadProgressForItem(itemID: string | null) {
    if (!itemID) return;
    readProgressSecRef.current.delete(itemID);
    if (readProgressActiveItemIDRef.current === itemID) {
      readProgressActiveItemIDRef.current = null;
      readProgressLastStartedAtRef.current = null;
    }
  }

  async function flushReadProgress(itemID: string | null) {
    if (!itemID || markedReadIDsRef.current.has(itemID)) return;
    if (readProgressActiveItemIDRef.current !== itemID || readProgressLastStartedAtRef.current == null) return;
    const elapsedSec = Math.max(0, (Date.now() - readProgressLastStartedAtRef.current) / 1000);
    const nextTotalSec = (readProgressSecRef.current.get(itemID) ?? 0) + elapsedSec;
    readProgressSecRef.current.set(itemID, nextTotalSec);
    readProgressLastStartedAtRef.current = null;
    if (nextTotalSec >= 30) await markItemRead(itemID);
  }

  async function synthesizeSummaryItem(itemID: string): Promise<SummaryAudioPrepared> {
    const response = await api.synthesizeSummaryAudio(itemID);
    const blob = base64ToBlob(response.audio_base64, response.content_type);
    return { itemID, objectURL: URL.createObjectURL(blob), response };
  }

  async function ensureSummaryPrefetch(queue: Item[], index: number) {
    const item = queue[index];
    if (!item) return;
    const requestSeq = summaryPlaybackRequestSeqRef.current;
    if (
      prefetchedAudioRef.current?.itemID === item.id ||
      pendingPrefetchRef.current?.itemID === item.id ||
      summaryQueue.prefetchingItemID === item.id
    ) return;
    setSummaryQueue((prev) => ({ ...prev, prefetchingItemID: item.id }));
    const promise = synthesizeSummaryItem(item.id);
    pendingPrefetchRef.current = { itemID: item.id, promise };
    try {
      const prepared = await promise;
      if (requestSeq != summaryPlaybackRequestSeqRef.current) {
        URL.revokeObjectURL(prepared.objectURL);
        return;
      }
      if (prefetchedAudioRef.current) URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
      prefetchedAudioRef.current = prepared;
      setSummaryQueue((prev) => ({ ...prev, prefetchedItemID: item.id }));
      void queryClient.prefetchQuery({
        queryKey: ["shared-summary-audio-item", item.id],
        queryFn: () => api.getItem(item.id),
      });
    } catch {
      if (prefetchedAudioRef.current?.itemID === item.id) {
        URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
        prefetchedAudioRef.current = null;
      }
    } finally {
      if (requestSeq != summaryPlaybackRequestSeqRef.current) {
        if (pendingPrefetchRef.current?.itemID === item.id) pendingPrefetchRef.current = null;
        return;
      }
      if (pendingPrefetchRef.current?.itemID === item.id) pendingPrefetchRef.current = null;
      setSummaryQueue((prev) => ({
        ...prev,
        prefetchingItemID: null,
        prefetchedItemID: prefetchedAudioRef.current?.itemID ?? null,
      }));
    }
  }

  async function playSummaryQueue(queue: Item[], autoplay: boolean, startOffsetSec = 0): Promise<boolean> {
    const item = queue[0];
    const audio = playback.audioRef.current;
    if (!item || !audio) return false;
    if (getSummaryAudioSettingsLoaded() && !getSummaryAudioConfigured()) return false;
    const requestSeq = summaryPlaybackRequestSeqRef.current + 1;
    summaryPlaybackRequestSeqRef.current = requestSeq;
    summaryPlaybackPreparingRef.current = true;
    playback.setErrorMessage(null);
    playback.setPlaybackState("preparing");
    let prepared: SummaryAudioPrepared;
    try {
      audio.pause();
      if (prefetchedAudioRef.current?.itemID === item.id) {
        prepared = prefetchedAudioRef.current;
        prefetchedAudioRef.current = null;
      } else if (pendingPrefetchRef.current?.itemID === item.id) {
        try {
          prepared = await pendingPrefetchRef.current.promise;
          if (prefetchedAudioRef.current?.itemID === item.id) prefetchedAudioRef.current = null;
        } catch {
          if (pendingPrefetchRef.current?.itemID === item.id) pendingPrefetchRef.current = null;
          prepared = await synthesizeSummaryItem(item.id);
        }
      } else {
        if (queue[1]) void ensureSummaryPrefetch(queue, 1);
        prepared = await synthesizeSummaryItem(item.id);
      }
      if (requestSeq != summaryPlaybackRequestSeqRef.current) {
        if (prefetchedAudioRef.current?.itemID !== prepared.itemID) URL.revokeObjectURL(prepared.objectURL);
        return false;
      }
      if (playback.currentAudioRef.current && playback.currentAudioRef.current.itemID !== prepared.itemID) {
        URL.revokeObjectURL(playback.currentAudioRef.current.objectURL);
      }
      playback.currentAudioRef.current = prepared;
      const immediateDetail =
        prepared.response.item && prepared.response.item.id === prepared.itemID ? prepared.response.item : null;
      setSummaryQueue((prev) => ({
        ...prev,
        queue,
        currentItemID: prepared.itemID,
        currentItemDetail: immediateDetail,
        currentPreprocessedText: prepared.response.preprocessed_text ?? null,
        prefetchedItemID: prefetchedAudioRef.current?.itemID ?? null,
      }));
      audio.src = prepared.objectURL;
      audio.load();
      await waitForLoadedMetadata(audio);
      const duration = resolvedAudioDuration(audio);
      const offsetSec = Math.min(Math.max(startOffsetSec, 0), duration || startOffsetSec);
      audio.currentTime = offsetSec;
      playback.setCurrentTimeSec(offsetSec);
      playback.setDurationSec(duration);
      if (autoplay) {
        try {
          await audio.play();
        } catch (err) {
          if (isPlaybackPermissionError(err)) {
            playback.setPlaybackState("paused");
            return true;
          }
          throw err;
        }
      }
      if (queue[1]) void ensureSummaryPrefetch(queue, 1);
      if (!autoplay) playback.setPlaybackState("paused");
      return true;
    } catch (err) {
      if (requestSeq != summaryPlaybackRequestSeqRef.current) return false;
      playback.setPlaybackState("error");
      playback.setErrorMessage(err instanceof Error ? err.message : String(err));
      return false;
    } finally {
      if (requestSeq === summaryPlaybackRequestSeqRef.current) summaryPlaybackPreparingRef.current = false;
    }
  }

  async function replenishSummaryQueueAfterCurrent(queueState: SharedSummaryQueueState): Promise<Item[]> {
    const queueKind = queueState.queueKind;
    if (!queueKind || queueKind === "brief") return [];
    const consumedIDs = new Set([...queueState.excludedItemIDs, ...queueState.queue.map((item) => item.id)]);
    const incoming = await fetchSummaryQueue(queueKind, queueState.queueQuery, [...consumedIDs]);
    return incoming.filter((item) => !consumedIDs.has(item.id)).slice(0, PLAYBACK_QUEUE_BUFFER_SIZE);
  }

  async function stopPlaybackInternal() {
    playback.stoppingPlaybackRef.current = true;
    summaryPlaybackRequestSeqRef.current += 1;
    summaryPlaybackPreparingRef.current = false;
    await flushReadProgress(summaryQueue.currentItemID);
    const audio = playback.audioRef.current;
    if (audio) {
      audio.pause();
      audio.currentTime = 0;
      audio.removeAttribute("src");
      audio.load();
    }
    if (playback.currentAudioRef.current) {
      URL.revokeObjectURL(playback.currentAudioRef.current.objectURL);
      playback.currentAudioRef.current = null;
    }
    if (prefetchedAudioRef.current) {
      URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
      prefetchedAudioRef.current = null;
    }
    pendingPrefetchRef.current = null;
    readProgressActiveItemIDRef.current = null;
    readProgressLastStartedAtRef.current = null;
    playback.setCurrentTimeSec(0);
    playback.setDurationSec(0);
    playback.setErrorMessage(null);
    playback.setPlaybackState("idle");
    playback.stoppingPlaybackRef.current = false;
  }

  async function startSummaryQueuePlaybackInternal(
    queueKind: SummaryAudioQueueKind,
    initialItems: Item[] | undefined,
    onModeSet: (mode: "summary_queue") => void,
    onQueueReset: () => void,
    options?: {
      currentIndex?: number;
      excludedItemIDs?: string[];
      startOffsetSec?: number;
      queueQuery?: string | null;
    },
  ) {
    if (getSummaryAudioSettingsLoaded() && !getSummaryAudioConfigured()) return;
    const seededQueue = initialItems ?? await fetchSummaryQueue(queueKind, options?.queueQuery, options?.excludedItemIDs);
    await session.interruptRemoteSessionIfNeeded();
    await stopPlaybackInternal();
    session.remoteSessionIDRef.current = null;
    session.lastPersistedPositionSecRef.current = 0;
    onModeSet("summary_queue");
    onQueueReset();
    markedReadIDsRef.current = new Set();
    readProgressSecRef.current = new Map();
    readProgressLastStartedAtRef.current = null;
    readProgressActiveItemIDRef.current = null;
    setSummaryQueue({
      queueKind,
      queueQuery: options?.queueQuery ?? null,
      queue: seededQueue,
      currentItemID: null,
      currentItemDetail: null,
      currentPreprocessedText: null,
      currentIndex: options?.currentIndex ?? 0,
      excludedItemIDs: options?.excludedItemIDs ?? [],
      prefetchedItemID: null,
      prefetchingItemID: null,
    });
    if (seededQueue[0]) {
      const started = await playSummaryQueue(seededQueue, true, options?.startOffsetSec ?? 0);
      if (started) {
        await session.createSummaryPlaybackSession(
          queueKind,
          options?.queueQuery ?? null,
          seededQueue,
          options?.currentIndex ?? 0,
          options?.excludedItemIDs ?? [],
          options?.startOffsetSec ?? 0,
        );
      }
    }
  }

  function updateQueueAfterAdvance(
    prev: SharedSummaryQueueState,
    nextState: SharedSummaryQueueState,
  ): SharedSummaryQueueState {
    const preparedDetail = preparedSummaryItemDetail(playback.currentAudioRef.current, nextState.currentItemID);
    const preparedText = preparedSummaryPreprocessedText(playback.currentAudioRef.current, nextState.currentItemID);
    return {
      ...nextState,
      currentItemDetail:
        prev.currentItemDetail && prev.currentItemDetail.id === nextState.currentItemID
          ? prev.currentItemDetail
          : preparedDetail ?? nextState.currentItemDetail,
      currentPreprocessedText:
        prev.currentItemID === nextState.currentItemID
          ? prev.currentPreprocessedText
          : preparedText ?? nextState.currentPreprocessedText,
      prefetchedItemID:
        prev.prefetchedItemID && prev.prefetchedItemID !== nextState.currentItemID ? prev.prefetchedItemID : null,
      prefetchingItemID:
        prev.prefetchingItemID && prev.prefetchingItemID !== nextState.currentItemID ? prev.prefetchingItemID : null,
    };
  }

  async function selectSummaryQueueItem(index: number) {
    const currentState = summaryQueueRef.current;
    if (getMode() !== "summary_queue") return;
    const nextQueue = currentState.queue.slice(index);
    if (nextQueue.length === 0) return;
    await flushReadProgress(currentState.currentItemID);
    const nextState: SharedSummaryQueueState = {
      queueKind: currentState.queueKind,
      queueQuery: currentState.queueQuery,
      queue: nextQueue,
      currentItemID: nextQueue[0]?.id ?? null,
      currentItemDetail: null,
      currentPreprocessedText: null,
      currentIndex: currentState.currentIndex + index,
      excludedItemIDs: [...currentState.excludedItemIDs, ...currentState.queue.slice(0, index).map((item) => item.id)],
      prefetchedItemID: null,
      prefetchingItemID: null,
    };
    const started = await playSummaryQueue(nextQueue, true);
    if (started) {
      setSummaryQueue((prev) => updateQueueAfterAdvance(prev, nextState));
      await session.persistRemoteSession("update", { summaryQueueState: nextState, positionSec: 0, durationSec: 0 });
    }
  }

  async function skipToNext() {
    const currentState = summaryQueueRef.current;
    if (getMode() !== "summary_queue") {
      await stopPlaybackInternal();
      return;
    }
    const queue = currentState.queue;
    if (queue.length <= 1) {
      const replenishedQueue = await replenishSummaryQueueAfterCurrent(currentState);
      if (replenishedQueue.length > 0) {
        resetReadProgressForItem(currentState.currentItemID);
        const nextState: SharedSummaryQueueState = {
          queueKind: currentState.queueKind,
          queueQuery: currentState.queueQuery,
          queue: replenishedQueue,
          currentItemID: replenishedQueue[0]?.id ?? null,
          currentItemDetail: null,
          currentPreprocessedText: null,
          currentIndex: currentState.currentIndex + queue.length,
          excludedItemIDs: [...currentState.excludedItemIDs, ...queue.map((item) => item.id)],
          prefetchedItemID: null,
          prefetchingItemID: null,
        };
        const started = await playSummaryQueue(replenishedQueue, true);
        if (started) {
          setSummaryQueue((prev) => updateQueueAfterAdvance(prev, nextState));
          await session.persistRemoteSession("update", { summaryQueueState: nextState, positionSec: 0, durationSec: 0 });
        }
        return;
      }
      const finalPosition = getDurationSec() > 0 ? getDurationSec() : getCurrentTimeSec();
      await session.persistRemoteSession("complete", {
        summaryQueueState: currentState,
        positionSec: finalPosition,
        durationSec: getDurationSec() || finalPosition,
      });
      await stopPlaybackInternal();
      resetReadProgressForItem(currentState.currentItemID);
      setSummaryQueue((prev) => ({
        ...prev,
        queue: [],
        currentIndex: prev.currentIndex + (queue.length > 0 ? 1 : 0),
        currentItemID: null,
        currentItemDetail: null,
        currentPreprocessedText: null,
        excludedItemIDs: [...prev.excludedItemIDs, ...queue.map((item) => item.id)],
        prefetchedItemID: null,
        prefetchingItemID: null,
      }));
      playback.setPlaybackState("finished");
      return;
    }
    const nextQueue = queue.slice(1);
    resetReadProgressForItem(currentState.currentItemID);
    const nextState: SharedSummaryQueueState = {
      queueKind: currentState.queueKind,
      queueQuery: currentState.queueQuery,
      queue: nextQueue,
      currentItemID: nextQueue[0]?.id ?? null,
      currentItemDetail: null,
      currentPreprocessedText: null,
      currentIndex: currentState.currentIndex + 1,
      excludedItemIDs: currentState.queue[0]
        ? [...currentState.excludedItemIDs, currentState.queue[0].id]
        : currentState.excludedItemIDs,
      prefetchedItemID: null,
      prefetchingItemID: null,
    };
    const started = await playSummaryQueue(nextQueue, true);
    if (started) {
      setSummaryQueue((prev) => updateQueueAfterAdvance(prev, nextState));
      await session.persistRemoteSession("update", { summaryQueueState: nextState, positionSec: 0, durationSec: 0 });
    }
  }

  useEffect(() => {
    return () => {
      if (playback.currentAudioRef.current) URL.revokeObjectURL(playback.currentAudioRef.current.objectURL);
      if (prefetchedAudioRef.current) URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
    };
  }, []);

  return {
    summaryQueue,
    setSummaryQueue,
    summaryQueueRef,
    prefetchedAudioRef,
    summaryPlaybackPreparingRef,
    markedReadIDsRef,
    readProgressSecRef,
    readProgressLastStartedAtRef,
    readProgressActiveItemIDRef,
    playSummaryQueue,
    startSummaryQueuePlaybackInternal,
    stopPlaybackInternal,
    selectSummaryQueueItem,
    skipToNext,
    flushReadProgress,
    resetReadProgressForItem,
  };
}
