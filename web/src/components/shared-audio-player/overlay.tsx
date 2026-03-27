"use client";

import Link from "next/link";
import { ExternalLink, Minimize2, Pause, Play, SkipForward, Square, Volume2, X } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import { PLAYBACK_QUEUE_VISIBLE_COUNT, useSharedAudioPlayer } from "@/components/shared-audio-player/provider";

function formatTime(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return `${mins}:${secs.toString().padStart(2, "0")}`;
}

export function SharedAudioOverlay() {
  const { t } = useI18n();
  const player = useSharedAudioPlayer();

  if (!player.mode || !player.expanded) {
    return null;
  }

  const detail = player.summaryQueue.currentItemDetail;
  const summaryVisibleQueue = player.summaryQueue.queue.slice(0, PLAYBACK_QUEUE_VISIBLE_COUNT);

  return (
    <div className="fixed inset-0 z-50 bg-[rgba(22,16,10,0.42)] backdrop-blur-sm" onClick={player.collapsePlayer}>
      <div className="flex min-h-full items-end justify-center p-0 lg:items-center lg:p-6">
        <div
          className="flex h-[85vh] w-full max-w-[1080px] flex-col overflow-hidden rounded-t-[32px] border border-[color:rgba(190,179,160,0.7)] bg-[color:rgba(252,251,248,0.98)] shadow-[0_24px_60px_rgba(15,23,42,0.2)] lg:h-auto lg:max-h-[82vh] lg:rounded-[32px]"
          onClick={(event) => event.stopPropagation()}
        >
          <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] px-5 py-4 lg:px-7">
            <div className="min-w-0">
              {player.display.modeLabelKey ? (
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                  {t(player.display.modeLabelKey)}
                </div>
              ) : null}
              <h2 className="mt-1 truncate font-serif text-2xl text-[var(--color-editorial-ink)]">
                {player.display.title || t("sharedAudio.emptyTitle")}
              </h2>
              {player.display.subtitle ? (
                <p className="mt-1 text-sm text-[var(--color-editorial-ink-soft)]">{player.display.subtitle}</p>
              ) : null}
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <button
                type="button"
                onClick={player.collapsePlayer}
                className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:bg-[var(--color-editorial-panel-strong)]"
                aria-label={t("sharedAudio.collapse")}
              >
                <Minimize2 className="size-4" aria-hidden="true" />
              </button>
              <button
                type="button"
                onClick={() => void player.stopPlayback()}
                className="inline-flex size-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:bg-[var(--color-editorial-panel-strong)]"
                aria-label={t("sharedAudio.close")}
              >
                <X className="size-4" aria-hidden="true" />
              </button>
            </div>
          </div>

          <div className="flex flex-col gap-5 overflow-y-auto px-5 py-5 lg:grid lg:grid-cols-[minmax(0,1.65fr)_minmax(320px,0.95fr)] lg:px-7">
            <div className="space-y-5">
              <section className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5">
                <div className="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    onClick={() => {
                      if (player.isPlaying) {
                        player.pausePlayback();
                        return;
                      }
                      void player.resumePlayback();
                    }}
                    className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] transition hover:-translate-y-0.5 hover:opacity-90"
                  >
                    {player.isPlaying ? <Pause className="size-4" aria-hidden="true" /> : <Play className="size-4 translate-x-[1px]" aria-hidden="true" />}
                    <span>{player.isPlaying ? t("sharedAudio.pause") : t("sharedAudio.play")}</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => void player.skipToNext()}
                    disabled={!player.canSkip}
                    className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:bg-white disabled:cursor-not-allowed disabled:opacity-45 disabled:hover:translate-y-0"
                  >
                    <SkipForward className="size-4" aria-hidden="true" />
                    <span>{t("sharedAudio.next")}</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => void player.stopPlayback()}
                    className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] transition hover:-translate-y-0.5 hover:bg-white"
                  >
                    <Square className="size-4" aria-hidden="true" />
                    <span>{t("sharedAudio.stop")}</span>
                  </button>
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-3 text-sm text-[var(--color-editorial-ink-soft)]">
                  <span>{player.durationSec > 0 ? `${formatTime(player.currentTimeSec)} / ${formatTime(player.durationSec)}` : formatTime(player.currentTimeSec)}</span>
                  {player.display.queueProgressLabel ? <span>{player.display.queueProgressLabel}</span> : null}
                  {player.isPreparing ? <span>{t("sharedAudio.preparing")}</span> : null}
                  {player.isPrefetching ? <span>{t("sharedAudio.prefetching")}</span> : null}
                  {player.errorMessage ? <span className="text-[#a23d2a]">{player.errorMessage}</span> : null}
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
                  className="mt-4 h-2 w-full cursor-pointer accent-[var(--color-editorial-ink)] disabled:cursor-not-allowed disabled:opacity-45"
                />
              </section>

              {player.mode === "summary_queue" ? (
                <section className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                    {t("sharedAudio.nowPlaying")}
                  </div>
                  <div className="mt-3 space-y-3">
                    <h3 className="font-serif text-2xl leading-tight text-[var(--color-editorial-ink)]">
                      {detail?.translated_title || detail?.summary?.translated_title || detail?.title || t("sharedAudio.emptyTitle")}
                    </h3>
                    <p className="text-sm text-[var(--color-editorial-ink-soft)]">
                      {detail?.title || t("summaryAudio.originalTitleEmpty")}
                    </p>
                    <div className="flex flex-wrap items-center gap-3 text-sm text-[var(--color-editorial-ink-soft)]">
                      <span>{detail?.source_title || t("summaryAudio.sourceUnknown")}</span>
                      {detail?.url ? (
                        <a href={detail.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-[var(--color-editorial-accent)] underline-offset-2 hover:underline">
                          <ExternalLink className="size-3.5" aria-hidden="true" />
                          {t("summaryAudio.openSource")}
                        </a>
                      ) : null}
                    </div>
                    <div className="rounded-[24px] border border-[var(--color-editorial-line)] bg-white/75 p-4 text-sm leading-7 text-[var(--color-editorial-ink)]">
                      {detail?.summary?.summary || t("summaryAudio.summaryPending")}
                    </div>
                  </div>
                </section>
              ) : null}

              {player.mode === "audio_briefing" && player.audioBriefing ? (
                <section className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                    {t("sharedAudio.audioBriefingDetail")}
                  </div>
                  <div className="mt-3 space-y-3">
                    <h3 className="font-serif text-2xl leading-tight text-[var(--color-editorial-ink)]">
                      {player.audioBriefing.title}
                    </h3>
                    {player.audioBriefing.summary ? (
                      <p className="text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                        {player.audioBriefing.summary}
                      </p>
                    ) : null}
                    <Link
                      href={player.audioBriefing.detailHref}
                      className="inline-flex items-center gap-2 text-sm font-medium text-[var(--color-editorial-accent)] underline-offset-2 hover:underline"
                    >
                      <ExternalLink className="size-3.5" aria-hidden="true" />
                      {t("sharedAudio.openBriefingDetail")}
                    </Link>
                  </div>
                </section>
              ) : null}
            </div>

            <aside className="space-y-4">
              {player.mode === "summary_queue" ? (
                <section className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5">
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                      {t("summaryAudio.queueTitle")}
                    </div>
                    <span className="text-xs text-[var(--color-editorial-ink-soft)]">
                      {player.display.queueCount} {t("summaryAudio.queueCount")}
                    </span>
                  </div>
                  <div className="mt-4 space-y-2">
                    {summaryVisibleQueue.map((item, index) => {
                      const isActive = player.summaryQueue.currentItemID === item.id;
                      return (
                        <button
                          key={item.id}
                          type="button"
                          onClick={() => void player.selectSummaryQueueItem(index)}
                          className={`group flex w-full items-start justify-between gap-3 rounded-[22px] border px-4 py-3 text-left transition hover:-translate-y-0.5 hover:shadow-[0_12px_30px_rgba(15,23,42,0.08)] ${
                            isActive
                              ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-accent-soft)]"
                              : "border-[var(--color-editorial-line)] bg-white/75 hover:border-[var(--color-editorial-ink-faint)]"
                          }`}
                        >
                          <div className="min-w-0 flex-1">
                            <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                              {index + 1}
                            </div>
                            <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">
                              {item.translated_title || item.title || t("summaryAudio.untitled")}
                            </div>
                            <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">
                              {item.source_title || t("summaryAudio.sourceUnknown")}
                            </div>
                          </div>
                          <span className={`inline-flex size-9 shrink-0 items-center justify-center rounded-full border ${
                            isActive
                              ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                              : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] group-hover:border-[var(--color-editorial-ink)] group-hover:text-[var(--color-editorial-ink)]"
                          }`}>
                            <Play className="size-4 translate-x-[1px]" aria-hidden="true" />
                          </span>
                        </button>
                      );
                    })}
                  </div>
                </section>
              ) : (
                <section className="rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                    {t("sharedAudio.status")}
                  </div>
                  <div className="mt-4 flex items-center gap-3 text-sm text-[var(--color-editorial-ink-soft)]">
                    <span className="inline-flex size-11 items-center justify-center rounded-full bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]">
                      <Volume2 className="size-5" aria-hidden="true" />
                    </span>
                    <div>
                      <div className="font-medium text-[var(--color-editorial-ink)]">{t("sharedAudio.audioBriefingReady")}</div>
                      <div>{player.playbackState}</div>
                    </div>
                  </div>
                </section>
              )}
            </aside>
          </div>
        </div>
      </div>
    </div>
  );
}
