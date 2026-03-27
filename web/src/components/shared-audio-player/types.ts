"use client";

import type { Item, ItemDetail, PlaybackSession, SummaryAudioSynthesisResponse } from "@/lib/api";

export type SharedAudioMode = "summary_queue" | "audio_briefing" | null;

export type SharedPlaybackState =
  | "idle"
  | "preparing"
  | "playing"
  | "paused"
  | "error"
  | "finished";

export type SummaryAudioQueueKind = "unread" | "later" | "favorite" | "brief";

export type SummaryAudioPrepared = {
  itemID: string;
  objectURL: string;
  response: SummaryAudioSynthesisResponse;
};

export type SummaryAudioPendingPrefetch = {
  itemID: string;
  promise: Promise<SummaryAudioPrepared>;
};

export type SharedAudioBriefingPayload = {
  jobID: string;
  title: string;
  summary?: string | null;
  audioURL: string;
  detailHref: string;
};

export type SharedSummaryQueueState = {
  queueKind: SummaryAudioQueueKind | null;
  queue: Item[];
  currentItemID: string | null;
  currentItemDetail: ItemDetail | null;
  currentIndex: number;
  excludedItemIDs: string[];
  prefetchedItemID: string | null;
  prefetchingItemID: string | null;
};

export type SharedAudioDisplayMeta = {
  title: string;
  subtitle: string | null;
  modeLabelKey: string | null;
  queueCount: number;
  queueProgressLabel: string | null;
  personaKey: string | null;
  personaName: string | null;
};

export type SharedAudioPlayerContextValue = {
  mode: SharedAudioMode;
  playbackState: SharedPlaybackState;
  expanded: boolean;
  errorMessage: string | null;
  currentTimeSec: number;
  durationSec: number;
  isPlaying: boolean;
  isPaused: boolean;
  isPreparing: boolean;
  isPrefetching: boolean;
  canSkip: boolean;
  display: SharedAudioDisplayMeta;
  summaryQueue: SharedSummaryQueueState;
  audioBriefing: SharedAudioBriefingPayload | null;
  startSummaryQueuePlayback: (queueKind: SummaryAudioQueueKind, initialItems?: Item[]) => Promise<void>;
  startAudioBriefingPlayback: (payload: SharedAudioBriefingPayload) => Promise<void>;
  resumePlaybackSession: (session: PlaybackSession) => Promise<void>;
  selectSummaryQueueItem: (index: number) => Promise<void>;
  pausePlayback: () => void;
  resumePlayback: () => Promise<void>;
  seekTo: (seconds: number) => void;
  skipToNext: () => Promise<void>;
  stopPlayback: () => Promise<void>;
  expandPlayer: () => void;
  collapsePlayer: () => void;
};
