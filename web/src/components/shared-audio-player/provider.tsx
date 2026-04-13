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
import { useQuery } from "@tanstack/react-query";
import { api, type Item, type ItemDetail, type PlaybackSession } from "@/lib/api";
import { hasSummaryAudioPlaybackAccess, isNaturalEndingPause, resolvedAudioDuration, waitForLoadedMetadata } from "./use-audio-playback";
import { useAudioPlayback } from "./use-audio-playback";
import { createEmptySummaryQueue, useAudioQueue } from "./use-audio-queue";
import type { SummaryAudioQueueKind } from "./types";
import { useAudioSession } from "./use-audio-session";
import type {
  SharedAudioBriefingPayload,
  SharedAudioDisplayMeta,
  SharedAudioMode,
  SharedAudioPlayerContextValue,
  SharedSummaryQueueState,
} from "./types";

export { PLAYBACK_QUEUE_VISIBLE_COUNT, fetchSummaryQueue } from "./use-audio-queue";

const SharedAudioPlayerContext = createContext<SharedAudioPlayerContextValue | null>(null);

export function SharedAudioPlayerProvider({ children }: { children: React.ReactNode }) {
  const [mode, setMode] = useState<SharedAudioMode>(null);
  const [expanded, setExpanded] = useState(false);
  const [summaryPersonaKey, setSummaryPersonaKey] = useState<string | null>(null);
  const [audioBriefing, setAudioBriefing] = useState<SharedAudioBriefingPayload | null>(null);

  const playback = useAudioPlayback();

  const modeRef = useRef(mode);
  modeRef.current = mode;

  const settingsQuery = useQuery({
    queryKey: ["shared-audio-player-settings"],
    queryFn: () => api.getSettings(),
    staleTime: 5 * 60 * 1000,
  });
  const summaryAudioSettingsLoaded = settingsQuery.isSuccess;
  const summaryAudioConfigured = hasSummaryAudioPlaybackAccess(settingsQuery.data ?? null);

  const navigatorPersonasQuery = useQuery({
    queryKey: ["navigator-personas"],
    queryFn: () => api.getNavigatorPersonas(),
  });

  const summaryQueueGetterRef = useRef<() => SharedSummaryQueueState>(() => createEmptySummaryQueue());
  const audioBriefingGetterRef = useRef<() => SharedAudioBriefingPayload | null>(() => null);
  const currentTimeGetterRef = useRef<() => number>(() => 0);
  const durationGetterRef = useRef<() => number>(() => 0);

  audioBriefingGetterRef.current = () => audioBriefing;
  currentTimeGetterRef.current = () => playback.currentTimeSec;
  durationGetterRef.current = () => playback.durationSec;

  const session = useAudioSession(
    () => modeRef.current,
    () => summaryQueueGetterRef.current(),
    () => audioBriefingGetterRef.current(),
    () => currentTimeGetterRef.current(),
    () => durationGetterRef.current(),
  );

  const queueHook = useAudioQueue(
    {
      audioRef: playback.audioRef,
      currentAudioRef: playback.currentAudioRef,
      stoppingPlaybackRef: playback.stoppingPlaybackRef,
      setPlaybackState: playback.setPlaybackState,
      setCurrentTimeSec: playback.setCurrentTimeSec,
      setDurationSec: playback.setDurationSec,
      setErrorMessage: playback.setErrorMessage,
      playbackState: playback.playbackState,
    },
    {
      persistRemoteSession: session.persistRemoteSession,
      interruptRemoteSessionIfNeeded: session.interruptRemoteSessionIfNeeded,
      createSummaryPlaybackSession: session.createSummaryPlaybackSession,
      remoteSessionIDRef: session.remoteSessionIDRef,
      lastPersistedPositionSecRef: session.lastPersistedPositionSecRef,
    },
    () => modeRef.current,
    () => summaryAudioSettingsLoaded,
    () => summaryAudioConfigured,
    () => playback.durationSec,
    () => playback.currentTimeSec,
  );

  summaryQueueGetterRef.current = () => queueHook.summaryQueue;

  const requestSummaryAutoPlay = useEffectEvent(() => {
    void (async () => {
      if (summaryAudioSettingsLoaded && !summaryAudioConfigured) return;
      const started = await queueHook.playSummaryQueue(queueHook.summaryQueue.queue, true);
      if (!started || !queueHook.summaryQueue.queueKind) return;
      await session.createSummaryPlaybackSession(
        queueHook.summaryQueue.queueKind,
        queueHook.summaryQueue.queueQuery,
        queueHook.summaryQueue.queue,
        queueHook.summaryQueue.currentIndex,
        queueHook.summaryQueue.excludedItemIDs,
        0,
      );
    })();
  });

  useEffect(() => {
    if (mode !== "summary_queue") return;
    if (summaryAudioSettingsLoaded && !summaryAudioConfigured) return;
    if (
      queueHook.summaryQueue.queue.length === 0 ||
      queueHook.summaryQueue.currentItemID ||
      playback.playbackState !== "idle" ||
      queueHook.summaryPlaybackPreparingRef.current
    ) return;
    requestSummaryAutoPlay();
  }, [mode, playback.playbackState, requestSummaryAutoPlay, summaryAudioConfigured, summaryAudioSettingsLoaded, queueHook.summaryQueue.currentItemID, queueHook.summaryQueue.queue]);

  async function startSummaryQueuePlayback(
    queueKind: SummaryAudioQueueKind,
    initialItems?: Item[],
    options?: { queueQuery?: string | null },
  ) {
    await queueHook.startSummaryQueuePlaybackInternal(
      queueKind,
      initialItems,
      (m) => { setMode(m); setAudioBriefing(null); setExpanded(false); },
      () => { setSummaryPersonaKey(null); },
      { queueQuery: options?.queueQuery ?? null },
    );
  }

  async function startAudioBriefingPlayback(payload: SharedAudioBriefingPayload) {
    const audio = playback.audioRef.current;
    if (!audio) return;
    await session.interruptRemoteSessionIfNeeded();
    await queueHook.stopPlaybackInternal();
    session.remoteSessionIDRef.current = null;
    session.lastPersistedPositionSecRef.current = 0;
    setMode("audio_briefing");
    setExpanded(false);
    queueHook.setSummaryQueue(createEmptySummaryQueue());
    setSummaryPersonaKey(null);
    setAudioBriefing(payload);
    playback.setPlaybackState("preparing");
    playback.setErrorMessage(null);
    try {
      audio.src = payload.audioURL;
      audio.load();
      await waitForLoadedMetadata(audio);
      const duration = resolvedAudioDuration(audio);
      audio.currentTime = 0;
      playback.setCurrentTimeSec(0);
      playback.setDurationSec(duration);
      try {
        await audio.play();
      } catch (err) {
        if (err instanceof Error && err.message.toLowerCase().includes("notallowederror")) {
          playback.setPlaybackState("paused");
          await session.createAudioBriefingPlaybackSession(payload, 0);
          return;
        }
        throw err;
      }
      await session.createAudioBriefingPlaybackSession(payload, 0);
    } catch (err) {
      playback.setPlaybackState("error");
      playback.setErrorMessage(err instanceof Error ? err.message : String(err));
    }
  }

  async function resumePlayback() {
    const audio = playback.audioRef.current;
    if (!audio) return;
    if (mode === "summary_queue") {
      if (!playback.currentAudioRef.current?.itemID || !audio.src) {
        await queueHook.playSummaryQueue(queueHook.summaryQueue.queue, true);
        return;
      }
    }
    try {
      await audio.play();
      void session.persistRemoteSession("update");
    } catch (err) {
      playback.setPlaybackState("error");
      playback.setErrorMessage(err instanceof Error ? err.message : String(err));
    }
  }

  async function stopPlayback() {
    await session.persistRemoteSession("interrupt");
    await queueHook.stopPlaybackInternal();
    queueHook.readProgressSecRef.current = new Map();
    queueHook.readProgressLastStartedAtRef.current = null;
    queueHook.readProgressActiveItemIDRef.current = null;
    session.remoteSessionIDRef.current = null;
    session.lastPersistedPositionSecRef.current = 0;
    setMode(null);
    queueHook.setSummaryQueue(createEmptySummaryQueue());
    setAudioBriefing(null);
    setExpanded(false);
  }

  async function resumePlaybackSession(sessionData: PlaybackSession) {
    await session.resumePlaybackSession(
      sessionData,
      async (queueKind, items, options) => {
        await queueHook.startSummaryQueuePlaybackInternal(
          queueKind,
          items,
          (m) => { setMode(m); setAudioBriefing(null); setExpanded(false); },
          () => { setSummaryPersonaKey(null); },
          {
            currentIndex: options.currentIndex,
            excludedItemIDs: options.excludedItemIDs,
            startOffsetSec: options.startOffsetSec,
            queueQuery: options.queueQuery,
          },
        );
      },
      async (payload, offsetSec) => {
        const audio = playback.audioRef.current;
        if (!audio) return;
        await session.interruptRemoteSessionIfNeeded();
        await queueHook.stopPlaybackInternal();
        session.remoteSessionIDRef.current = null;
        session.lastPersistedPositionSecRef.current = 0;
        setMode("audio_briefing");
        setExpanded(false);
        queueHook.setSummaryQueue(createEmptySummaryQueue());
        setSummaryPersonaKey(null);
        setAudioBriefing(payload);
        playback.setPlaybackState("preparing");
        playback.setErrorMessage(null);
        try {
          audio.src = payload.audioURL;
          audio.load();
          await waitForLoadedMetadata(audio);
          const duration = resolvedAudioDuration(audio);
          const clampedOffset = Math.min(Math.max(offsetSec, 0), duration || offsetSec);
          audio.currentTime = clampedOffset;
          playback.setCurrentTimeSec(clampedOffset);
          playback.setDurationSec(duration);
          try {
            await audio.play();
          } catch (err) {
            if (err instanceof Error && err.message.toLowerCase().includes("notallowederror")) {
              playback.setPlaybackState("paused");
              await session.createAudioBriefingPlaybackSession(payload, clampedOffset);
              return;
            }
            throw err;
          }
          await session.createAudioBriefingPlaybackSession(payload, clampedOffset);
        } catch (err) {
          playback.setPlaybackState("error");
          playback.setErrorMessage(err instanceof Error ? err.message : String(err));
        }
      },
    );
  }

  const handleAudioPlay = useEffectEvent(() => {
    playback.setPlaybackState("playing");
    if (mode === "summary_queue" && queueHook.summaryQueue.currentItemID && !queueHook.markedReadIDsRef.current.has(queueHook.summaryQueue.currentItemID)) {
      queueHook.readProgressActiveItemIDRef.current = queueHook.summaryQueue.currentItemID;
      queueHook.readProgressLastStartedAtRef.current = Date.now();
    }
    void session.persistRemoteSession("update");
  });

  const handleAudioPause = useEffectEvent(() => {
    if (playback.stoppingPlaybackRef.current) return;
    if (isNaturalEndingPause(playback.audioRef.current)) return;
    if (playback.playbackState !== "idle" && playback.playbackState !== "finished") {
      playback.setPlaybackState("paused");
    }
    if (mode === "summary_queue") void queueHook.flushReadProgress(queueHook.summaryQueue.currentItemID);
    void session.persistRemoteSession("update");
  });

  const handleAudioTimeUpdate = useEffectEvent(() => {
    if (playback.stoppingPlaybackRef.current) return;
    const audio = playback.audioRef.current;
    if (!audio) return;
    playback.setCurrentTimeSec(audio.currentTime || 0);
    playback.setDurationSec(resolvedAudioDuration(audio));
    if (session.remoteSessionIDRef.current && Math.abs((audio.currentTime || 0) - session.lastPersistedPositionSecRef.current) >= 15) {
      void session.persistRemoteSession("update", {
        positionSec: audio.currentTime || 0,
        durationSec: resolvedAudioDuration(audio),
      });
    }
  });

  const handleAudioEnded = useEffectEvent(() => {
    if (playback.stoppingPlaybackRef.current) return;
    if (mode === "summary_queue") {
      const activeItemID = playback.currentAudioRef.current?.itemID ?? null;
      if (activeItemID && queueHook.summaryQueue.currentItemID && activeItemID !== queueHook.summaryQueue.currentItemID) return;
      void queueHook.flushReadProgress(queueHook.summaryQueue.currentItemID);
      void queueHook.skipToNext();
      return;
    }
    void session.persistRemoteSession("complete", {
      audioBriefingPayload: audioBriefing,
      modeOverride: "audio_briefing",
      positionSec: playback.durationSec || playback.currentTimeSec,
      durationSec: playback.durationSec || playback.currentTimeSec,
    });
    playback.setPlaybackState("finished");
    playback.setCurrentTimeSec(0);
  });

  useEffect(() => {
    const audio = playback.audioRef.current;
    if (!audio) return;
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
      const detail = queueHook.summaryQueue.currentItemDetail as ItemDetail | null;
      const title =
        detail?.translated_title || detail?.summary?.translated_title || detail?.title || queueHook.summaryQueue.queue[0]?.translated_title || queueHook.summaryQueue.queue[0]?.title || "";
      const subtitle = detail?.source_title || queueHook.summaryQueue.queue[0]?.source_title || null;
      const queueProgressLabel = queueHook.summaryQueue.queue.length > 0 ? `${queueHook.summaryQueue.currentIndex + 1}/${queueHook.summaryQueue.currentIndex + queueHook.summaryQueue.queue.length}` : null;
      const personaName = summaryPersonaKey && navigatorPersonasQuery.data?.[summaryPersonaKey]?.name ? navigatorPersonasQuery.data[summaryPersonaKey].name : null;
      return { title, subtitle, modeLabelKey: "sharedAudio.mode.summaryQueue", queueCount: queueHook.summaryQueue.queue.length, queueProgressLabel, personaKey: summaryPersonaKey, personaName };
    }
    if (mode === "audio_briefing" && audioBriefing) {
      return { title: audioBriefing.title, subtitle: audioBriefing.summary ?? null, modeLabelKey: "sharedAudio.mode.audioBriefing", queueCount: 0, queueProgressLabel: null, personaKey: null, personaName: null };
    }
    return { title: "", subtitle: null, modeLabelKey: null, queueCount: 0, queueProgressLabel: null, personaKey: null, personaName: null };
  }, [audioBriefing, mode, navigatorPersonasQuery.data, summaryPersonaKey, queueHook.summaryQueue]);

  const value: SharedAudioPlayerContextValue = {
    mode,
    playbackState: playback.playbackState,
    expanded,
    errorMessage: playback.errorMessage,
    summaryAudioSettingsLoaded,
    summaryAudioConfigured,
    currentTimeSec: playback.currentTimeSec,
    durationSec: playback.durationSec,
    isPlaying: playback.playbackState === "playing",
    isPaused: playback.playbackState === "paused",
    isPreparing: playback.playbackState === "preparing",
    isPrefetching: Boolean(queueHook.summaryQueue.prefetchingItemID),
    canSkip: mode === "summary_queue" && queueHook.summaryQueue.queue.length > 1,
    display,
    summaryQueue: queueHook.summaryQueue,
    audioBriefing,
    startSummaryQueuePlayback,
    startAudioBriefingPlayback,
    resumePlaybackSession,
    selectSummaryQueueItem: queueHook.selectSummaryQueueItem,
    pausePlayback: playback.pausePlayback,
    resumePlayback,
    seekTo: playback.seekTo,
    skipToNext: queueHook.skipToNext,
    stopPlayback,
    expandPlayer: () => setExpanded(true),
    collapsePlayer: () => setExpanded(false),
  };

  return (
    <SharedAudioPlayerContext.Provider value={value}>
      {children}
      <audio ref={playback.audioRef} preload="auto" className="hidden" />
    </SharedAudioPlayerContext.Provider>
  );
}

export function useSharedAudioPlayer() {
  const context = useContext(SharedAudioPlayerContext);
  if (!context) throw new Error("useSharedAudioPlayer must be used within SharedAudioPlayerProvider");
  return context;
}
