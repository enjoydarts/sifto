"use client";

import { useCallback, useRef, useState } from "react";
import type { ItemDetail, UserSettings } from "@/lib/api";
import { getSummaryAudioReadiness } from "@/lib/summary-audio-readiness";
import type { SharedPlaybackState, SummaryAudioPrepared } from "./types";

export function hasSummaryAudioPlaybackAccess(settings: UserSettings | null | undefined): boolean {
  const readiness = getSummaryAudioReadiness(settings ?? null);
  return readiness.ready;
}

export function base64ToBlob(base64: string, contentType: string): Blob {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return new Blob([bytes], { type: contentType || "audio/mpeg" });
}

export function progressRatio(positionSec: number, durationSec: number): number | null {
  if (durationSec <= 0) return null;
  return Math.max(0, Math.min(1, positionSec / durationSec));
}

export function isPlaybackPermissionError(err: unknown): boolean {
  if (!(err instanceof Error)) return false;
  const message = `${err.name} ${err.message}`.toLowerCase();
  return (
    message.includes("notallowederror") ||
    message.includes("the play method is not allowed") ||
    message.includes("user denied permission")
  );
}

export async function waitForLoadedMetadata(audio: HTMLAudioElement): Promise<void> {
  if (audio.readyState >= 1) return;
  await new Promise<void>((resolve) => {
    const handle = () => resolve();
    audio.addEventListener("loadedmetadata", handle, { once: true });
  });
}

export function resolvedAudioDuration(audio: HTMLAudioElement): number {
  return Number.isFinite(audio.duration) ? audio.duration : 0;
}

export function sameSummaryItemDetail(a: ItemDetail | null, b: ItemDetail | null): boolean {
  if (!a || !b) return a === b;
  return (
    a.id === b.id &&
    (a.summary?.id ?? null) === (b.summary?.id ?? null) &&
    (a.summary?.summary ?? null) === (b.summary?.summary ?? null) &&
    (a.summary?.translated_title ?? null) === (b.summary?.translated_title ?? null) &&
    (a.translated_title ?? null) === (b.translated_title ?? null) &&
    (a.source_title ?? null) === (b.source_title ?? null)
  );
}

export function preparedSummaryItemDetail(prepared: SummaryAudioPrepared | null, itemID: string | null): ItemDetail | null {
  if (!prepared || !itemID || prepared.itemID !== itemID) return null;
  const detail = prepared.response.item ?? null;
  return detail && detail.id === itemID ? detail : null;
}

export function preparedSummaryPreprocessedText(prepared: SummaryAudioPrepared | null, itemID: string | null): string | null {
  if (!prepared || !itemID || prepared.itemID !== itemID) return null;
  return prepared.response.preprocessed_text ?? null;
}

export function isNaturalEndingPause(audio: HTMLAudioElement | null): boolean {
  if (!audio) return false;
  const duration = resolvedAudioDuration(audio);
  if (audio.ended) return true;
  if (duration <= 0) return false;
  return audio.currentTime >= Math.max(0, duration - 0.35);
}

export function useAudioPlayback() {
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const currentAudioRef = useRef<SummaryAudioPrepared | null>(null);
  const stoppingPlaybackRef = useRef(false);
  const [playbackState, setPlaybackState] = useState<SharedPlaybackState>("idle");
  const [currentTimeSec, setCurrentTimeSec] = useState(0);
  const [durationSec, setDurationSec] = useState(0);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const pausePlayback = useCallback(() => {
    audioRef.current?.pause();
  }, []);

  const seekTo = useCallback((seconds: number) => {
    const audio = audioRef.current;
    if (!audio || !Number.isFinite(audio.duration) || audio.duration <= 0) return;
    const next = Math.min(Math.max(seconds, 0), audio.duration);
    audio.currentTime = next;
    setCurrentTimeSec(next);
  }, []);

  return {
    audioRef,
    currentAudioRef,
    stoppingPlaybackRef,
    playbackState,
    setPlaybackState,
    currentTimeSec,
    setCurrentTimeSec,
    durationSec,
    setDurationSec,
    errorMessage,
    setErrorMessage,
    pausePlayback,
    seekTo,
  };
}
