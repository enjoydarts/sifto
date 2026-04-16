"use client";

import { useRef } from "react";
import { api, type Item, type PlaybackSession } from "@/lib/api";
import { swallowPlaybackSessionError } from "./playback-session-guard";
import { progressRatio } from "./use-audio-playback";
import type {
  SharedAudioBriefingPayload,
  SharedAudioMode,
  SharedSummaryQueueState,
  SummaryAudioQueueKind,
} from "./types";

type SummaryResumePayload = {
  queue_kind: SummaryAudioQueueKind;
  queue_query?: string | null;
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

function reportPlaybackSessionError(error: unknown) {
  console.error("playback session request failed", error);
}

function buildSummaryResumePayload(
  queueKind: SummaryAudioQueueKind,
  queueQuery: string | null,
  queue: Item[],
  currentIndex: number,
  excludedItemIDs: string[],
  offsetSec: number,
): SummaryResumePayload {
  return {
    queue_kind: queueKind,
    queue_query: queueQuery,
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

export function useAudioSession(
  getMode: () => SharedAudioMode,
  getSummaryQueue: () => SharedSummaryQueueState,
  getAudioBriefing: () => SharedAudioBriefingPayload | null,
  getCurrentTimeSec: () => number,
  getDurationSec: () => number,
) {
  const remoteSessionIDRef = useRef<string | null>(null);
  const lastPersistedPositionSecRef = useRef<number>(0);

  async function createSummaryPlaybackSession(
    queueKind: SummaryAudioQueueKind,
    queueQuery: string | null,
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
    const session = await swallowPlaybackSessionError(
      () => api.createPlaybackSession({
        mode: "summary_queue",
        title: current?.translated_title || current?.title || "",
        subtitle: current?.source_title || "",
        current_position_sec: offsetSec,
        duration_sec: getDurationSec(),
        progress_ratio: progressRatio(offsetSec, getDurationSec()),
        resume_payload: buildSummaryResumePayload(queueKind, queueQuery, queue, currentIndex, excludedItemIDs, offsetSec),
      }),
      reportPlaybackSessionError,
    );
    if (!session) {
      remoteSessionIDRef.current = null;
      return;
    }
    remoteSessionIDRef.current = session.id;
    lastPersistedPositionSecRef.current = offsetSec;
  }

  async function createAudioBriefingPlaybackSession(payload: SharedAudioBriefingPayload, offsetSec: number) {
    const session = await swallowPlaybackSessionError(
      () => api.createPlaybackSession({
        mode: "audio_briefing",
        title: payload.title,
        subtitle: payload.summary ?? "",
        current_position_sec: offsetSec,
        duration_sec: getDurationSec(),
        progress_ratio: progressRatio(offsetSec, getDurationSec()),
        resume_payload: buildAudioBriefingResumePayload(payload, offsetSec),
      }),
      reportPlaybackSessionError,
    );
    if (!session) {
      remoteSessionIDRef.current = null;
      return;
    }
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
    if (!sessionID) return;
    const effectiveMode = options?.modeOverride ?? getMode();
    const effectivePosition = Math.max(0, Math.floor(options?.positionSec ?? getCurrentTimeSec()));
    const effectiveDuration = Math.max(0, Math.floor(options?.durationSec ?? getDurationSec()));
    if (effectiveMode === "summary_queue") {
      const state = options?.summaryQueueState ?? getSummaryQueue();
      if (!state.queueKind || state.queue.length === 0) return;
      const current = state.queue[0];
      const body = {
        title: current?.translated_title || current?.title || "",
        subtitle: current?.source_title || "",
        current_position_sec: effectivePosition,
        duration_sec: effectiveDuration,
        progress_ratio: progressRatio(effectivePosition, effectiveDuration),
        resume_payload: buildSummaryResumePayload(
          state.queueKind,
          state.queueQuery,
          state.queue,
          state.currentIndex,
          state.excludedItemIDs,
          effectivePosition,
        ),
      };
      if (kind === "complete") {
        await swallowPlaybackSessionError(() => api.completePlaybackSession(sessionID, body), reportPlaybackSessionError);
        remoteSessionIDRef.current = null;
        return;
      }
      if (kind === "interrupt") {
        await swallowPlaybackSessionError(() => api.interruptPlaybackSession(sessionID, body), reportPlaybackSessionError);
        remoteSessionIDRef.current = null;
        return;
      }
      const updated = await swallowPlaybackSessionError(() => api.updatePlaybackSession(sessionID, body), reportPlaybackSessionError);
      if (!updated) return;
      lastPersistedPositionSecRef.current = effectivePosition;
      return;
    }
    if (effectiveMode === "audio_briefing") {
      const payload = options?.audioBriefingPayload ?? getAudioBriefing();
      if (!payload) return;
      const body = {
        title: payload.title,
        subtitle: payload.summary ?? "",
        current_position_sec: effectivePosition,
        duration_sec: effectiveDuration,
        progress_ratio: progressRatio(effectivePosition, effectiveDuration),
        resume_payload: buildAudioBriefingResumePayload(payload, effectivePosition),
      };
      if (kind === "complete") {
        await swallowPlaybackSessionError(() => api.completePlaybackSession(sessionID, body), reportPlaybackSessionError);
        remoteSessionIDRef.current = null;
        return;
      }
      if (kind === "interrupt") {
        await swallowPlaybackSessionError(() => api.interruptPlaybackSession(sessionID, body), reportPlaybackSessionError);
        remoteSessionIDRef.current = null;
        return;
      }
      const updated = await swallowPlaybackSessionError(() => api.updatePlaybackSession(sessionID, body), reportPlaybackSessionError);
      if (!updated) return;
      lastPersistedPositionSecRef.current = effectivePosition;
    }
  }

  async function interruptRemoteSessionIfNeeded() {
    if (!remoteSessionIDRef.current) return;
    await persistRemoteSession("interrupt");
  }

  async function resumePlaybackSession(
    session: PlaybackSession,
    onStartSummaryQueue: (
      queueKind: SummaryAudioQueueKind,
      items: Item[] | undefined,
      options: {
        currentIndex: number;
        excludedItemIDs: string[];
        startOffsetSec: number;
        queueQuery: string | null;
      },
    ) => Promise<void>,
    onStartAudioBriefing: (payload: SharedAudioBriefingPayload, offsetSec: number) => Promise<void>,
  ) {
    if (session.mode === "summary_queue") {
      const payload = (session.resume_payload ?? {}) as Partial<SummaryResumePayload>;
      const queueItems = Array.isArray(payload.queue_items) ? (payload.queue_items as Item[]) : [];
      const queueKind = payload.queue_kind;
      const queueQuery = typeof payload.queue_query === "string" ? payload.queue_query : null;
      if (!queueKind || (queueKind !== "view" && queueItems.length === 0)) return;
      await onStartSummaryQueue(queueKind, queueItems, {
        currentIndex: payload.current_queue_index ?? 0,
        excludedItemIDs: Array.isArray(payload.excluded_item_ids) ? (payload.excluded_item_ids as string[]) : [],
        startOffsetSec: payload.current_item_offset_sec ?? session.current_position_sec ?? 0,
        queueQuery,
      });
      return;
    }
    if (session.mode === "audio_briefing") {
      const payload = (session.resume_payload ?? {}) as Partial<AudioBriefingResumePayload>;
      const briefingID = typeof payload.briefing_id === "string" ? payload.briefing_id : null;
      if (!briefingID) return;
      const detail = await api.getAudioBriefing(briefingID);
      if (!detail.audio_url) throw new Error("audio briefing audio is unavailable");
      await onStartAudioBriefing(
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

  return {
    remoteSessionIDRef,
    lastPersistedPositionSecRef,
    createSummaryPlaybackSession,
    createAudioBriefingPlaybackSession,
    persistRemoteSession,
    interruptRemoteSessionIfNeeded,
    resumePlaybackSession,
  };
}
