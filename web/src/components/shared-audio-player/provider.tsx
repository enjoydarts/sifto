"use client";

import {
  createContext,
  useContext,
  useEffect,
  useEffectEvent,
  useMemo,
  useRef,
  useState,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, type Item, type ItemDetail, type PlaybackSession } from "@/lib/api";
import type {
  SharedAudioBriefingPayload,
  SharedAudioDisplayMeta,
  SharedAudioMode,
  SharedAudioPlayerContextValue,
  SharedPlaybackState,
  SharedSummaryQueueState,
  SummaryAudioPendingPrefetch,
  SummaryAudioPrepared,
  SummaryAudioQueueKind,
} from "@/components/shared-audio-player/types";

const PLAYBACK_QUEUE_BUFFER_SIZE = 24;
const PLAYBACK_QUEUE_VISIBLE_COUNT = 12;

const SharedAudioPlayerContext = createContext<SharedAudioPlayerContextValue | null>(null);

function base64ToBlob(base64: string, contentType: string): Blob {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return new Blob([bytes], { type: contentType || "audio/mpeg" });
}

async function fetchSummaryQueue(queueKind: SummaryAudioQueueKind): Promise<Item[]> {
  const params =
    queueKind === "later"
      ? { status: "summarized", page_size: PLAYBACK_QUEUE_BUFFER_SIZE, sort: "newest", unread_only: true, later_only: true }
      : queueKind === "favorite"
        ? { status: "summarized", page_size: PLAYBACK_QUEUE_BUFFER_SIZE, sort: "newest", favorite_only: true }
        : { status: "summarized", page_size: PLAYBACK_QUEUE_BUFFER_SIZE, sort: "newest", unread_only: true };
  const response = await api.getItems(params);
  return response.items;
}

function createEmptySummaryQueue(): SharedSummaryQueueState {
  return {
    queueKind: null,
    queue: [],
    currentItemID: null,
    currentItemDetail: null,
    currentIndex: 0,
    excludedItemIDs: [],
    prefetchedItemID: null,
    prefetchingItemID: null,
  };
}

type SummaryResumePayload = {
  queue_kind: SummaryAudioQueueKind;
  queue_items: Item[];
  current_item_id: string | null;
  current_queue_index: number;
  current_item_offset_sec: number;
  excluded_item_ids: string[];
};

type AudioBriefingResumePayload = {
  briefing_id: string;
  current_offset_sec: number;
};

function progressRatio(positionSec: number, durationSec: number): number | null {
  if (durationSec <= 0) {
    return null;
  }
  return Math.max(0, Math.min(1, positionSec / durationSec));
}

function isPlaybackPermissionError(err: unknown): boolean {
  if (!(err instanceof Error)) {
    return false;
  }
  const message = `${err.name} ${err.message}`.toLowerCase();
  return message.includes("notallowederror")
    || message.includes("the play method is not allowed")
    || message.includes("user denied permission");
}

async function waitForLoadedMetadata(audio: HTMLAudioElement): Promise<void> {
  if (audio.readyState >= 1) {
    return;
  }
  await new Promise<void>((resolve) => {
    const handle = () => resolve();
    audio.addEventListener("loadedmetadata", handle, { once: true });
  });
}

export function SharedAudioPlayerProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const currentAudioRef = useRef<SummaryAudioPrepared | null>(null);
  const prefetchedAudioRef = useRef<SummaryAudioPrepared | null>(null);
  const pendingPrefetchRef = useRef<SummaryAudioPendingPrefetch | null>(null);
  const markedReadIDsRef = useRef<Set<string>>(new Set());
  const readProgressSecRef = useRef<Map<string, number>>(new Map());
  const readProgressLastStartedAtRef = useRef<number | null>(null);
  const readProgressActiveItemIDRef = useRef<string | null>(null);
  const remoteSessionIDRef = useRef<string | null>(null);
  const lastPersistedPositionSecRef = useRef<number>(0);
  const [mode, setMode] = useState<SharedAudioMode>(null);
  const [playbackState, setPlaybackState] = useState<SharedPlaybackState>("idle");
  const [expanded, setExpanded] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [currentTimeSec, setCurrentTimeSec] = useState(0);
  const [durationSec, setDurationSec] = useState(0);
  const [summaryPersonaKey, setSummaryPersonaKey] = useState<string | null>(null);
  const [summaryQueue, setSummaryQueue] = useState<SharedSummaryQueueState>(() => createEmptySummaryQueue());
  const [audioBriefing, setAudioBriefing] = useState<SharedAudioBriefingPayload | null>(null);

  const summaryQueueQuery = useQuery({
    queryKey: ["shared-summary-audio-queue", summaryQueue.queueKind],
    queryFn: async () => {
      if (!summaryQueue.queueKind) {
        return [];
      }
      return fetchSummaryQueue(summaryQueue.queueKind);
    },
    enabled: Boolean(summaryQueue.queueKind),
  });

  const navigatorPersonasQuery = useQuery({
    queryKey: ["navigator-personas"],
    queryFn: () => api.getNavigatorPersonas(),
  });

  const currentSummaryItem = useMemo(() => {
    return summaryQueue.queue[0] ?? null;
  }, [summaryQueue.queue]);

  const currentSummaryDetailQuery = useQuery({
    queryKey: ["shared-summary-audio-item", currentSummaryItem?.id],
    queryFn: async () => {
      if (!currentSummaryItem?.id) {
        throw new Error("missing item id");
      }
      return api.getItem(currentSummaryItem.id);
    },
    enabled: mode === "summary_queue" && Boolean(currentSummaryItem?.id),
  });

  useEffect(() => {
    if (mode !== "summary_queue") {
      return;
    }
    setSummaryQueue((prev) => ({
      ...prev,
      currentItemDetail: currentSummaryDetailQuery.data ?? null,
    }));
  }, [currentSummaryDetailQuery.data, mode]);

  useEffect(() => {
    if (mode !== "summary_queue") {
      return;
    }
    const incoming = summaryQueueQuery.data ?? [];
    if (incoming.length === 0) {
      return;
    }
    setSummaryQueue((prev) => {
      if (!prev.queueKind) {
        return prev;
      }
      const excluded = new Set(prev.excludedItemIDs);
      const filteredIncoming = incoming.filter((item) => !excluded.has(item.id));
      if (filteredIncoming.length === 0) {
        return prev;
      }
      if (prev.queue.length === 0) {
        if (prev.currentIndex > 0) {
          return prev;
        }
        return {
          ...prev,
          queue: filteredIncoming.slice(0, PLAYBACK_QUEUE_BUFFER_SIZE),
        };
      }
      const existing = new Set(prev.queue.map((item) => item.id));
      const appended = filteredIncoming.filter((item) => !existing.has(item.id));
      if (appended.length === 0) {
        return prev;
      }
      return {
        ...prev,
        queue: [...prev.queue, ...appended].slice(0, PLAYBACK_QUEUE_BUFFER_SIZE),
      };
    });
  }, [mode, summaryQueueQuery.data]);

  const requestSummaryAutoPlay = useEffectEvent(() => {
    void (async () => {
      const started = await playSummaryQueue(summaryQueue.queue, true);
      if (!started || !summaryQueue.queueKind) {
        return;
      }
      await createSummaryPlaybackSession(
        summaryQueue.queueKind,
        summaryQueue.queue,
        summaryQueue.currentIndex,
        summaryQueue.excludedItemIDs,
        0,
      );
    })();
  });

  useEffect(() => {
    if (mode !== "summary_queue") {
      return;
    }
    if (summaryQueue.queue.length === 0 || summaryQueue.currentItemID || playbackState !== "idle") {
      return;
    }
    requestSummaryAutoPlay();
  }, [mode, playbackState, summaryQueue.currentItemID, summaryQueue.queue]);

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

  async function markItemRead(itemID: string) {
    if (!itemID || markedReadIDsRef.current.has(itemID)) {
      return;
    }
    markedReadIDsRef.current.add(itemID);
    try {
      await api.markItemRead(itemID);
      void queryClient.invalidateQueries({ queryKey: ["items-feed"] });
      void queryClient.invalidateQueries({ queryKey: ["summary-audio-item", itemID] });
      void queryClient.invalidateQueries({ queryKey: ["shared-summary-audio-item", itemID] });
      void queryClient.invalidateQueries({ queryKey: ["summary-audio-queue"] });
      void queryClient.invalidateQueries({ queryKey: ["shared-summary-audio-queue"] });
    } catch {
      return;
    }
  }

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

  async function flushReadProgress(itemID: string | null) {
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
      await markItemRead(itemID);
    }
  }

  function buildSummaryResumePayload(
    queueKind: SummaryAudioQueueKind,
    queue: Item[],
    currentIndex: number,
    excludedItemIDs: string[],
    offsetSec: number,
  ): SummaryResumePayload {
    return {
      queue_kind: queueKind,
      queue_items: queue,
      current_item_id: queue[0]?.id ?? null,
      current_queue_index: currentIndex,
      current_item_offset_sec: offsetSec,
      excluded_item_ids: excludedItemIDs,
    };
  }

  function buildAudioBriefingResumePayload(
    payload: SharedAudioBriefingPayload,
    offsetSec: number,
  ): AudioBriefingResumePayload {
    return {
      briefing_id: payload.jobID,
      current_offset_sec: offsetSec,
    };
  }

  async function createSummaryPlaybackSession(
    queueKind: SummaryAudioQueueKind,
    queue: Item[],
    currentIndex: number,
    excludedItemIDs: string[],
    offsetSec: number,
  ) {
    if (!queueKind || queue.length === 0) {
      remoteSessionIDRef.current = null;
      return;
    }
    const current = queue[0];
    const session = await api.createPlaybackSession({
      mode: "summary_queue",
      title: current?.translated_title || current?.title || "",
      subtitle: current?.source_title || "",
      current_position_sec: offsetSec,
      duration_sec: durationSec,
      progress_ratio: progressRatio(offsetSec, durationSec),
      resume_payload: buildSummaryResumePayload(queueKind, queue, currentIndex, excludedItemIDs, offsetSec),
    });
    remoteSessionIDRef.current = session.id;
    lastPersistedPositionSecRef.current = offsetSec;
  }

  async function createAudioBriefingPlaybackSession(payload: SharedAudioBriefingPayload, offsetSec: number) {
    const session = await api.createPlaybackSession({
      mode: "audio_briefing",
      title: payload.title,
      subtitle: payload.summary ?? "",
      current_position_sec: offsetSec,
      duration_sec: durationSec,
      progress_ratio: progressRatio(offsetSec, durationSec),
      resume_payload: buildAudioBriefingResumePayload(payload, offsetSec),
    });
    remoteSessionIDRef.current = session.id;
    lastPersistedPositionSecRef.current = offsetSec;
  }

  async function persistRemoteSession(
    kind: "update" | "complete" | "interrupt",
    options?: {
      summaryQueueState?: SharedSummaryQueueState;
      audioBriefingPayload?: SharedAudioBriefingPayload | null;
      modeOverride?: SharedAudioMode;
      positionSec?: number;
      durationSec?: number;
    },
  ) {
    const sessionID = remoteSessionIDRef.current;
    if (!sessionID) {
      return;
    }
    const effectiveMode = options?.modeOverride ?? mode;
    const effectivePosition = Math.max(0, Math.floor(options?.positionSec ?? currentTimeSec));
    const effectiveDuration = Math.max(0, Math.floor(options?.durationSec ?? durationSec));
    if (effectiveMode === "summary_queue") {
      const state = options?.summaryQueueState ?? summaryQueue;
      if (!state.queueKind || state.queue.length === 0) {
        return;
      }
      const current = state.queue[0];
      const body = {
        title: current?.translated_title || current?.title || "",
        subtitle: current?.source_title || "",
        current_position_sec: effectivePosition,
        duration_sec: effectiveDuration,
        progress_ratio: progressRatio(effectivePosition, effectiveDuration),
        resume_payload: buildSummaryResumePayload(
          state.queueKind,
          state.queue,
          state.currentIndex,
          state.excludedItemIDs,
          effectivePosition,
        ),
      };
      if (kind === "complete") {
        await api.completePlaybackSession(sessionID, body);
        remoteSessionIDRef.current = null;
        return;
      }
      if (kind === "interrupt") {
        await api.interruptPlaybackSession(sessionID, body);
        remoteSessionIDRef.current = null;
        return;
      }
      await api.updatePlaybackSession(sessionID, body);
      lastPersistedPositionSecRef.current = effectivePosition;
      return;
    }
    if (effectiveMode === "audio_briefing") {
      const payload = options?.audioBriefingPayload ?? audioBriefing;
      if (!payload) {
        return;
      }
      const body = {
        title: payload.title,
        subtitle: payload.summary ?? "",
        current_position_sec: effectivePosition,
        duration_sec: effectiveDuration,
        progress_ratio: progressRatio(effectivePosition, effectiveDuration),
        resume_payload: buildAudioBriefingResumePayload(payload, effectivePosition),
      };
      if (kind === "complete") {
        await api.completePlaybackSession(sessionID, body);
        remoteSessionIDRef.current = null;
        return;
      }
      if (kind === "interrupt") {
        await api.interruptPlaybackSession(sessionID, body);
        remoteSessionIDRef.current = null;
        return;
      }
      await api.updatePlaybackSession(sessionID, body);
      lastPersistedPositionSecRef.current = effectivePosition;
    }
  }

  async function interruptRemoteSessionIfNeeded() {
    if (!remoteSessionIDRef.current) {
      return;
    }
    await persistRemoteSession("interrupt");
  }

  async function synthesizeSummaryItem(itemID: string): Promise<SummaryAudioPrepared> {
    const response = await api.synthesizeSummaryAudio(itemID);
    const blob = base64ToBlob(response.audio_base64, response.content_type);
    return {
      itemID,
      objectURL: URL.createObjectURL(blob),
      response,
    };
  }

  async function ensureSummaryPrefetch(queue: Item[], index: number) {
    const item = queue[index];
    if (!item) {
      return;
    }
    if (
      prefetchedAudioRef.current?.itemID === item.id ||
      pendingPrefetchRef.current?.itemID === item.id ||
      summaryQueue.prefetchingItemID === item.id
    ) {
      return;
    }
    setSummaryQueue((prev) => ({
      ...prev,
      prefetchingItemID: item.id,
    }));
    const promise = synthesizeSummaryItem(item.id);
    pendingPrefetchRef.current = { itemID: item.id, promise };
    try {
      const prepared = await promise;
      if (prefetchedAudioRef.current) {
        URL.revokeObjectURL(prefetchedAudioRef.current.objectURL);
      }
      prefetchedAudioRef.current = prepared;
      setSummaryQueue((prev) => ({
        ...prev,
        prefetchedItemID: item.id,
      }));
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
      if (pendingPrefetchRef.current?.itemID === item.id) {
        pendingPrefetchRef.current = null;
      }
      setSummaryQueue((prev) => ({
        ...prev,
        prefetchingItemID: null,
        prefetchedItemID: prefetchedAudioRef.current?.itemID ?? null,
      }));
    }
  }

  async function playSummaryQueue(queue: Item[], autoplay: boolean, startOffsetSec = 0): Promise<boolean> {
    const item = queue[0];
    const audio = audioRef.current;
    if (!item || !audio) {
      return false;
    }
    setErrorMessage(null);
    setPlaybackState("preparing");
    let prepared: SummaryAudioPrepared;
    try {
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
          prepared = await synthesizeSummaryItem(item.id);
        }
      } else {
        if (queue[1]) {
          void ensureSummaryPrefetch(queue, 1);
        }
        prepared = await synthesizeSummaryItem(item.id);
      }
      if (currentAudioRef.current && currentAudioRef.current.itemID !== prepared.itemID) {
        URL.revokeObjectURL(currentAudioRef.current.objectURL);
      }
      currentAudioRef.current = prepared;
      setSummaryPersonaKey(prepared.response.persona || null);
      setSummaryQueue((prev) => ({
        ...prev,
        currentItemID: prepared.itemID,
        prefetchedItemID: prefetchedAudioRef.current?.itemID ?? null,
      }));
      audio.src = prepared.objectURL;
      audio.load();
      await waitForLoadedMetadata(audio);
      const offsetSec = Math.min(Math.max(startOffsetSec, 0), Number.isFinite(audio.duration) ? audio.duration || startOffsetSec : startOffsetSec);
      audio.currentTime = offsetSec;
      setCurrentTimeSec(offsetSec);
      setDurationSec(0);
      if (autoplay) {
        try {
          await audio.play();
        } catch (err) {
          if (isPlaybackPermissionError(err)) {
            setPlaybackState("paused");
            return true;
          }
          throw err;
        }
      }
      if (queue[1]) {
        void ensureSummaryPrefetch(queue, 1);
      }
      if (!autoplay) {
        setPlaybackState("paused");
      }
      return true;
    } catch (err) {
      setPlaybackState("error");
      setErrorMessage(err instanceof Error ? err.message : String(err));
      return false;
    }
  }

  async function stopPlaybackInternal() {
    await flushReadProgress(summaryQueue.currentItemID);
    const audio = audioRef.current;
    if (audio) {
      audio.pause();
      audio.currentTime = 0;
      audio.removeAttribute("src");
      audio.load();
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
    readProgressActiveItemIDRef.current = null;
    readProgressLastStartedAtRef.current = null;
    setCurrentTimeSec(0);
    setDurationSec(0);
    setErrorMessage(null);
    setPlaybackState("idle");
    setSummaryPersonaKey(null);
  }

  async function startSummaryQueuePlaybackInternal(
    queueKind: SummaryAudioQueueKind,
    initialItems: Item[] | undefined,
    options?: {
      currentIndex?: number;
      excludedItemIDs?: string[];
      startOffsetSec?: number;
    },
  ) {
    const seededQueue = (initialItems ?? []).slice(0, PLAYBACK_QUEUE_BUFFER_SIZE);
    await interruptRemoteSessionIfNeeded();
    await stopPlaybackInternal();
    remoteSessionIDRef.current = null;
    lastPersistedPositionSecRef.current = 0;
    setMode("summary_queue");
    setAudioBriefing(null);
    setExpanded(false);
    markedReadIDsRef.current = new Set();
    readProgressSecRef.current = new Map();
    readProgressLastStartedAtRef.current = null;
    readProgressActiveItemIDRef.current = null;
    setSummaryQueue({
      queueKind,
      queue: seededQueue,
      currentItemID: null,
      currentItemDetail: null,
      currentIndex: options?.currentIndex ?? 0,
      excludedItemIDs: options?.excludedItemIDs ?? [],
      prefetchedItemID: null,
      prefetchingItemID: null,
    });
    if (seededQueue[0]) {
      const started = await playSummaryQueue(seededQueue, true, options?.startOffsetSec ?? 0);
      if (started) {
        await createSummaryPlaybackSession(
          queueKind,
          seededQueue,
          options?.currentIndex ?? 0,
          options?.excludedItemIDs ?? [],
          options?.startOffsetSec ?? 0,
        );
      }
    }
  }

  async function startSummaryQueuePlayback(queueKind: SummaryAudioQueueKind, initialItems?: Item[]) {
    await startSummaryQueuePlaybackInternal(queueKind, initialItems);
  }

  async function startAudioBriefingPlaybackInternal(payload: SharedAudioBriefingPayload, startOffsetSec = 0) {
    const audio = audioRef.current;
    if (!audio) {
      return;
    }
    await interruptRemoteSessionIfNeeded();
    await stopPlaybackInternal();
    remoteSessionIDRef.current = null;
    lastPersistedPositionSecRef.current = 0;
    readProgressSecRef.current = new Map();
    readProgressLastStartedAtRef.current = null;
    readProgressActiveItemIDRef.current = null;
    setMode("audio_briefing");
    setExpanded(false);
    setSummaryQueue(createEmptySummaryQueue());
    setSummaryPersonaKey(null);
    setAudioBriefing(payload);
    setPlaybackState("preparing");
    setErrorMessage(null);
    try {
      audio.src = payload.audioURL;
      audio.load();
      await waitForLoadedMetadata(audio);
      const offsetSec = Math.min(Math.max(startOffsetSec, 0), Number.isFinite(audio.duration) ? audio.duration || startOffsetSec : startOffsetSec);
      audio.currentTime = offsetSec;
      setCurrentTimeSec(offsetSec);
      setDurationSec(0);
      try {
        await audio.play();
      } catch (err) {
        if (isPlaybackPermissionError(err)) {
          setPlaybackState("paused");
          await createAudioBriefingPlaybackSession(payload, offsetSec);
          return;
        }
        throw err;
      }
      await createAudioBriefingPlaybackSession(payload, offsetSec);
    } catch (err) {
      setPlaybackState("error");
      setErrorMessage(err instanceof Error ? err.message : String(err));
    }
  }

  async function startAudioBriefingPlayback(payload: SharedAudioBriefingPayload) {
    await startAudioBriefingPlaybackInternal(payload);
  }

  async function selectSummaryQueueItem(index: number) {
    if (mode !== "summary_queue") {
      return;
    }
    const nextQueue = summaryQueue.queue.slice(index);
    if (nextQueue.length === 0) {
      return;
    }
    await flushReadProgress(summaryQueue.currentItemID);
    setSummaryQueue((prev) => ({
      ...prev,
      queue: nextQueue,
      currentIndex: prev.currentIndex + index,
      currentItemID: null,
      currentItemDetail: null,
      excludedItemIDs: [...prev.excludedItemIDs, ...prev.queue.slice(0, index).map((item) => item.id)],
      prefetchedItemID: null,
      prefetchingItemID: null,
    }));
    const nextState: SharedSummaryQueueState = {
      queueKind: summaryQueue.queueKind,
      queue: nextQueue,
      currentItemID: null,
      currentItemDetail: null,
      currentIndex: summaryQueue.currentIndex + index,
      excludedItemIDs: [...summaryQueue.excludedItemIDs, ...summaryQueue.queue.slice(0, index).map((item) => item.id)],
      prefetchedItemID: null,
      prefetchingItemID: null,
    };
    const started = await playSummaryQueue(nextQueue, true);
    if (started) {
      await persistRemoteSession("update", { summaryQueueState: nextState, positionSec: 0, durationSec: 0 });
    }
  }

  async function skipToNext() {
    if (mode !== "summary_queue") {
      await stopPlaybackInternal();
      return;
    }
    const queue = summaryQueue.queue;
    if (queue.length <= 1) {
      const finalState = summaryQueue;
      const finalPosition = durationSec > 0 ? durationSec : currentTimeSec;
      await persistRemoteSession("complete", {
        summaryQueueState: finalState,
        positionSec: finalPosition,
        durationSec: durationSec || finalPosition,
      });
      await stopPlaybackInternal();
      resetReadProgressForItem(summaryQueue.currentItemID);
      setSummaryQueue((prev) => ({
        ...prev,
        queue: [],
        currentIndex: prev.currentIndex + (queue.length > 0 ? 1 : 0),
        currentItemID: null,
        currentItemDetail: null,
        excludedItemIDs: [...prev.excludedItemIDs, ...queue.map((item) => item.id)],
        prefetchedItemID: null,
        prefetchingItemID: null,
      }));
      setPlaybackState("finished");
      return;
    }
    const nextQueue = queue.slice(1);
    resetReadProgressForItem(summaryQueue.currentItemID);
    setSummaryQueue((prev) => ({
      ...prev,
      queue: nextQueue,
      currentIndex: prev.currentIndex + 1,
      currentItemID: null,
      currentItemDetail: null,
      excludedItemIDs: prev.queue[0] ? [...prev.excludedItemIDs, prev.queue[0].id] : prev.excludedItemIDs,
      prefetchedItemID: null,
      prefetchingItemID: null,
    }));
    const nextState: SharedSummaryQueueState = {
      queueKind: summaryQueue.queueKind,
      queue: nextQueue,
      currentItemID: null,
      currentItemDetail: null,
      currentIndex: summaryQueue.currentIndex + 1,
      excludedItemIDs: summaryQueue.queue[0] ? [...summaryQueue.excludedItemIDs, summaryQueue.queue[0].id] : summaryQueue.excludedItemIDs,
      prefetchedItemID: null,
      prefetchingItemID: null,
    };
    const started = await playSummaryQueue(nextQueue, true);
    if (started) {
      await persistRemoteSession("update", { summaryQueueState: nextState, positionSec: 0, durationSec: 0 });
    }
  }

  function pausePlayback() {
    audioRef.current?.pause();
  }

  async function resumePlayback() {
    const audio = audioRef.current;
    if (!audio) {
      return;
    }
    if (mode === "summary_queue") {
      if (!currentAudioRef.current?.itemID || !audio.src) {
        await playSummaryQueue(summaryQueue.queue, true);
        return;
      }
    }
    try {
      await audio.play();
      void persistRemoteSession("update");
    } catch (err) {
      setPlaybackState("error");
      setErrorMessage(err instanceof Error ? err.message : String(err));
    }
  }

  async function stopPlayback() {
    await persistRemoteSession("interrupt");
    await stopPlaybackInternal();
    readProgressSecRef.current = new Map();
    readProgressLastStartedAtRef.current = null;
    readProgressActiveItemIDRef.current = null;
    remoteSessionIDRef.current = null;
    lastPersistedPositionSecRef.current = 0;
    setMode(null);
    setSummaryQueue(createEmptySummaryQueue());
    setAudioBriefing(null);
    setExpanded(false);
  }

  async function resumePlaybackSession(session: PlaybackSession) {
    if (session.mode === "summary_queue") {
      const payload = (session.resume_payload ?? {}) as Partial<SummaryResumePayload>;
      const queueItems = Array.isArray(payload.queue_items) ? (payload.queue_items as Item[]) : [];
      const queueKind = payload.queue_kind;
      if (!queueKind || queueItems.length === 0) {
        return;
      }
      await startSummaryQueuePlaybackInternal(queueKind, queueItems, {
        currentIndex: payload.current_queue_index ?? 0,
        excludedItemIDs: Array.isArray(payload.excluded_item_ids) ? (payload.excluded_item_ids as string[]) : [],
        startOffsetSec: payload.current_item_offset_sec ?? session.current_position_sec ?? 0,
      });
      return;
    }
    if (session.mode === "audio_briefing") {
      const payload = (session.resume_payload ?? {}) as Partial<AudioBriefingResumePayload>;
      const briefingID = typeof payload.briefing_id === "string" ? payload.briefing_id : null;
      if (!briefingID) {
        return;
      }
      const detail = await api.getAudioBriefing(briefingID);
      if (!detail.audio_url) {
        throw new Error("audio briefing audio is unavailable");
      }
      await startAudioBriefingPlaybackInternal(
        {
          jobID: detail.job.id,
          title: detail.job.title || session.title,
          summary: null,
          audioURL: detail.audio_url,
          detailHref: `/audio-briefings/${detail.job.id}`,
        },
        payload.current_offset_sec ?? session.current_position_sec ?? 0,
      );
    }
  }

  function seekTo(seconds: number) {
    const audio = audioRef.current;
    if (!audio || !Number.isFinite(audio.duration) || audio.duration <= 0) {
      return;
    }
    const next = Math.min(Math.max(seconds, 0), audio.duration);
    audio.currentTime = next;
    setCurrentTimeSec(next);
  }

  const handleAudioPlay = useEffectEvent(() => {
    setPlaybackState("playing");
    if (mode === "summary_queue" && summaryQueue.currentItemID && !markedReadIDsRef.current.has(summaryQueue.currentItemID)) {
      readProgressActiveItemIDRef.current = summaryQueue.currentItemID;
      readProgressLastStartedAtRef.current = Date.now();
    }
    void persistRemoteSession("update");
  });

  const handleAudioPause = useEffectEvent(() => {
    if (playbackState !== "idle" && playbackState !== "finished") {
      setPlaybackState("paused");
    }
    if (mode === "summary_queue") {
      void flushReadProgress(summaryQueue.currentItemID);
    }
    void persistRemoteSession("update");
  });

  const handleAudioTimeUpdate = useEffectEvent(() => {
    const audio = audioRef.current;
    if (!audio) {
      return;
    }
    setCurrentTimeSec(audio.currentTime || 0);
    setDurationSec(Number.isFinite(audio.duration) ? audio.duration : 0);
    if (remoteSessionIDRef.current && Math.abs((audio.currentTime || 0) - lastPersistedPositionSecRef.current) >= 15) {
      void persistRemoteSession("update", {
        positionSec: audio.currentTime || 0,
        durationSec: Number.isFinite(audio.duration) ? audio.duration : 0,
      });
    }
  });

  const handleAudioEnded = useEffectEvent(() => {
    if (mode === "summary_queue") {
      void flushReadProgress(summaryQueue.currentItemID);
      void skipToNext();
      return;
    }
    void persistRemoteSession("complete", {
      audioBriefingPayload: audioBriefing,
      modeOverride: "audio_briefing",
      positionSec: durationSec || currentTimeSec,
      durationSec: durationSec || currentTimeSec,
    });
    setPlaybackState("finished");
    setCurrentTimeSec(0);
  });

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) {
      return;
    }
    const onPlay = () => handleAudioPlay();
    const onPause = () => handleAudioPause();
    const onTimeUpdate = () => handleAudioTimeUpdate();
    const onEnded = () => handleAudioEnded();
    audio.addEventListener("play", onPlay);
    audio.addEventListener("pause", onPause);
    audio.addEventListener("timeupdate", onTimeUpdate);
    audio.addEventListener("ended", onEnded);
    audio.addEventListener("loadedmetadata", onTimeUpdate);
    audio.addEventListener("durationchange", onTimeUpdate);
    return () => {
      audio.removeEventListener("play", onPlay);
      audio.removeEventListener("pause", onPause);
      audio.removeEventListener("timeupdate", onTimeUpdate);
      audio.removeEventListener("ended", onEnded);
      audio.removeEventListener("loadedmetadata", onTimeUpdate);
      audio.removeEventListener("durationchange", onTimeUpdate);
    };
  }, []);

  const display = useMemo<SharedAudioDisplayMeta>(() => {
    if (mode === "summary_queue") {
      const detail = summaryQueue.currentItemDetail as ItemDetail | null;
      const title =
        detail?.translated_title ||
        detail?.summary?.translated_title ||
        detail?.title ||
        summaryQueue.queue[0]?.translated_title ||
        summaryQueue.queue[0]?.title ||
        "";
      const subtitle = detail?.source_title || summaryQueue.queue[0]?.source_title || null;
      const queueProgressLabel =
        summaryQueue.queue.length > 0
          ? `${summaryQueue.currentIndex + 1}/${summaryQueue.currentIndex + summaryQueue.queue.length}`
          : null;
      const personaName =
        summaryPersonaKey && navigatorPersonasQuery.data?.[summaryPersonaKey]?.name
          ? navigatorPersonasQuery.data[summaryPersonaKey].name
          : null;
      return {
        title,
        subtitle,
        modeLabelKey: "sharedAudio.mode.summaryQueue",
        queueCount: summaryQueue.queue.length,
        queueProgressLabel,
        personaKey: summaryPersonaKey,
        personaName,
      };
    }
    if (mode === "audio_briefing" && audioBriefing) {
      return {
        title: audioBriefing.title,
        subtitle: audioBriefing.summary ?? null,
        modeLabelKey: "sharedAudio.mode.audioBriefing",
        queueCount: 0,
        queueProgressLabel: null,
        personaKey: null,
        personaName: null,
      };
    }
    return {
      title: "",
      subtitle: null,
      modeLabelKey: null,
      queueCount: 0,
      queueProgressLabel: null,
      personaKey: null,
      personaName: null,
    };
  }, [audioBriefing, mode, navigatorPersonasQuery.data, summaryPersonaKey, summaryQueue]);

  const value: SharedAudioPlayerContextValue = {
    mode,
    playbackState,
    expanded,
    errorMessage,
    currentTimeSec,
    durationSec,
    isPlaying: playbackState === "playing",
    isPaused: playbackState === "paused",
    isPreparing: playbackState === "preparing",
    isPrefetching: Boolean(summaryQueue.prefetchingItemID),
    canSkip: mode === "summary_queue" && summaryQueue.queue.length > 1,
    display,
    summaryQueue,
    audioBriefing,
    startSummaryQueuePlayback,
    startAudioBriefingPlayback,
    resumePlaybackSession,
    selectSummaryQueueItem,
    pausePlayback,
    resumePlayback,
    seekTo,
    skipToNext,
    stopPlayback,
    expandPlayer: () => setExpanded(true),
    collapsePlayer: () => setExpanded(false),
  };

  return (
    <SharedAudioPlayerContext.Provider value={value}>
      {children}
      <audio ref={audioRef} preload="auto" className="hidden" />
    </SharedAudioPlayerContext.Provider>
  );
}

export function useSharedAudioPlayer() {
  const context = useContext(SharedAudioPlayerContext);
  if (!context) {
    throw new Error("useSharedAudioPlayer must be used within SharedAudioPlayerProvider");
  }
  return context;
}

export {
  PLAYBACK_QUEUE_BUFFER_SIZE,
  PLAYBACK_QUEUE_VISIBLE_COUNT,
  fetchSummaryQueue,
};
