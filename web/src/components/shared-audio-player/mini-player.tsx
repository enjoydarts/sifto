"use client";

import { LoaderCircle, Maximize2, Pause, Play, SkipForward, Square, Volume2 } from "lucide-react";
import { AINavigatorAvatar } from "@/components/briefing/ai-navigator-avatar";
import { useI18n } from "@/components/i18n-provider";
import { useSharedAudioPlayer } from "@/components/shared-audio-player/provider";

function formatTime(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return `${mins}:${secs.toString().padStart(2, "0")}`;
}

export function SharedAudioMiniPlayer() {
  const { t, locale } = useI18n();
  const player = useSharedAudioPlayer();

  if (!player.mode) {
    return null;
  }

  return (
    <div className="fixed inset-x-0 bottom-[calc(env(safe-area-inset-bottom)+4.75rem)] z-40 px-3 md:bottom-0 md:left-6 md:right-auto md:w-[min(50vw,42rem)] md:px-0 md:pb-[calc(env(safe-area-inset-bottom)+0.75rem)]">
      <div className="w-full rounded-[28px] border border-[color:rgba(190,179,160,0.7)] bg-[color:rgba(252,251,248,0.94)] px-4 py-3 shadow-[0_-12px_32px_rgba(15,23,42,0.16)] backdrop-blur">
        <div className="flex items-center gap-3">
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="hidden size-11 shrink-0 items-center justify-center rounded-full bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)] sm:inline-flex">
              <Volume2 className="size-5" aria-hidden="true" />
            </div>
            <div className="min-w-0">
              {player.display.modeLabelKey ? (
                <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                  {t(player.display.modeLabelKey)}
                </div>
              ) : null}
              <div className="truncate text-sm font-semibold text-[var(--color-editorial-ink)]">
                {player.display.title || t("sharedAudio.emptyTitle")}
              </div>
              {player.display.personaKey ? (
                <div className="mt-1 inline-flex items-center gap-1.5 rounded-full border border-[var(--color-editorial-line)] bg-white/70 px-2 py-1 text-[11px] font-medium text-[var(--color-editorial-ink-soft)]">
                  <AINavigatorAvatar persona={player.display.personaKey} className="size-4" />
                  <span>{t("sharedAudio.persona", locale === "ja" ? "話者" : "Voice")}</span>
                  <span>{player.display.personaName || t(`settings.navigator.persona.${player.display.personaKey}`, player.display.personaKey)}</span>
                </div>
              ) : null}
              <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--color-editorial-ink-soft)]">
                {player.display.subtitle ? <span className="truncate">{player.display.subtitle}</span> : null}
                {player.display.queueProgressLabel ? <span>{player.display.queueProgressLabel}</span> : null}
                {player.durationSec > 0 ? (
                  <span>{`${formatTime(player.currentTimeSec)} / ${formatTime(player.durationSec)}`}</span>
                ) : (
                  <span>{formatTime(player.currentTimeSec)}</span>
                )}
                {player.isPreparing ? (
                  <span className="inline-flex items-center gap-1.5">
                    <LoaderCircle className="size-3.5 animate-spin" aria-hidden="true" />
                    {t("sharedAudio.preparing")}
                  </span>
                ) : null}
                {player.isPrefetching ? (
                  <span className="inline-flex items-center gap-1.5">
                    <LoaderCircle className="size-3.5 animate-spin" aria-hidden="true" />
                    {t("sharedAudio.prefetching")}
                  </span>
                ) : null}
              </div>
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-1 sm:gap-2">
            <button
              type="button"
              onClick={() => {
                if (player.isPlaying) {
                  player.pausePlayback();
                  return;
                }
                void player.resumePlayback();
              }}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)] transition hover:-translate-y-0.5 hover:opacity-90"
              aria-label={player.isPlaying ? t("sharedAudio.pause") : t("sharedAudio.play")}
            >
              {player.isPlaying ? <Pause className="size-4" aria-hidden="true" /> : <Play className="size-4 translate-x-[1px]" aria-hidden="true" />}
            </button>
            <button
              type="button"
              onClick={() => void player.skipToNext()}
              disabled={!player.canSkip}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-45 disabled:hover:translate-y-0"
              aria-label={t("sharedAudio.next")}
            >
              <SkipForward className="size-4" aria-hidden="true" />
            </button>
            <button
              type="button"
              onClick={() => void player.stopPlayback()}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:bg-[var(--color-editorial-panel-strong)]"
              aria-label={t("sharedAudio.stop")}
            >
              <Square className="size-4" aria-hidden="true" />
            </button>
            <button
              type="button"
              onClick={player.expandPlayer}
              className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:bg-[var(--color-editorial-panel-strong)]"
              aria-label={t("sharedAudio.expand")}
            >
              <Maximize2 className="size-4" aria-hidden="true" />
            </button>
          </div>
        </div>

        <input
          type="range"
          min={0}
          max={player.durationSec || 0}
          step={1}
          value={Math.min(player.currentTimeSec, player.durationSec || 0)}
          onChange={(event) => player.seekTo(Number(event.target.value))}
          disabled={player.durationSec <= 0}
          aria-label={t("sharedAudio.seek")}
          className="mt-3 h-1.5 w-full cursor-pointer accent-[var(--color-editorial-ink)] disabled:cursor-not-allowed disabled:opacity-45"
        />
      </div>
    </div>
  );
}
