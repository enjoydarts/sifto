"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { History, Play } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { useSharedAudioPlayer } from "@/components/shared-audio-player/provider";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";
import { api, type Item, type PlaybackMode, type PlaybackSession } from "@/lib/api";

type HistoryFilter = "all" | PlaybackMode;

function parseFilter(raw: string | null): HistoryFilter {
  return raw === "summary_queue" || raw === "audio_briefing" ? raw : "all";
}

function formatDateTime(value: string | null | undefined, locale: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(locale === "ja" ? "ja-JP" : "en-US", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function formatDuration(seconds: number | null | undefined) {
  if (seconds == null || Number.isNaN(seconds)) return "—";
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}:${secs.toString().padStart(2, "0")}`;
}

function summaryQueueItems(session: PlaybackSession): Item[] {
  const payload = session.resume_payload as { queue_items?: Item[] };
  return Array.isArray(payload.queue_items) ? payload.queue_items : [];
}

function currentSummaryItem(session: PlaybackSession): Item | null {
  return summaryQueueItems(session)[0] ?? null;
}

function detailHref(session: PlaybackSession): string | null {
  if (session.mode === "audio_briefing") {
    const payload = session.resume_payload as { briefing_id?: string };
    return typeof payload.briefing_id === "string" ? `/audio-briefings/${payload.briefing_id}` : null;
  }
  const current = currentSummaryItem(session);
  return current?.id ? `/items/${current.id}` : null;
}

const filters: HistoryFilter[] = ["all", "summary_queue", "audio_briefing"];

export default function PlaybackHistoryPage() {
  const { t, locale } = useI18n();
  const router = useRouter();
  const searchParams = useSearchParams();
  const player = useSharedAudioPlayer();
  const filter = parseFilter(searchParams.get("mode"));
  const historyQuery = useQuery({
    queryKey: ["playback-history", filter],
    queryFn: () =>
      api.getPlaybackSessions({
        mode: filter === "all" ? undefined : filter,
        limit: 20,
      }),
  });

  async function handleResume(session: PlaybackSession) {
    await player.resumePlaybackSession(session);
    player.expandPlayer();
  }

  return (
    <PageTransition>
      <div className="space-y-4">
        <PageHeader
          eyebrow={t("playbackHistory.eyebrow")}
          title={t("playbackHistory.title")}
          titleIcon={History}
          description={t("playbackHistory.description")}
          actions={(
            <div className="flex flex-wrap items-center justify-end gap-2">
              <Link
                href="/audio-player"
                className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {t("summaryAudio.title")}
              </Link>
              <Link
                href="/audio-briefings"
                className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {t("nav.audioBriefings")}
              </Link>
            </div>
          )}
        />

        <SectionCard>
          <div className="flex flex-wrap gap-2">
            {filters.map((candidate) => {
              const active = filter === candidate;
              const labelKey =
                candidate === "all"
                  ? "playbackHistory.filter.all"
                  : candidate === "summary_queue"
                    ? "playbackHistory.filter.summary"
                    : "playbackHistory.filter.audioBriefing";
              return (
                <button
                  key={candidate}
                  type="button"
                  onClick={() => router.replace(candidate === "all" ? "/playback-history" : `/playback-history?mode=${candidate}`)}
                  className={`inline-flex min-h-10 items-center justify-center rounded-full border px-4 py-2 text-sm font-medium ${
                    active
                      ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                      : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                  }`}
                >
                  {t(labelKey)}
                </button>
              );
            })}
          </div>
        </SectionCard>

        {historyQuery.isLoading ? (
          <SectionCard>
            <p className="text-sm text-editorial-muted">{t("common.loading")}</p>
          </SectionCard>
        ) : historyQuery.data?.items.length ? (
          <div className="space-y-3">
            {historyQuery.data.items.map((session) => {
              const ratio = Math.max(0, Math.min(100, Math.round((session.progress_ratio ?? 0) * 100)));
              const summaryCurrent = currentSummaryItem(session);
              const summaryRemaining = session.mode === "summary_queue" ? summaryQueueItems(session).length : 0;
              const href = detailHref(session);
              return (
                <SectionCard key={session.id}>
                  <div className="flex h-full flex-col gap-4">
                    <div className="space-y-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <Tag tone="default">{t(`playbackHistory.mode.${session.mode === "summary_queue" ? "summaryQueue" : "audioBriefing"}`)}</Tag>
                        <Tag tone="default">{t(`playbackHistory.status.${session.status}`)}</Tag>
                      </div>
                      <h2 className="font-serif text-xl text-editorial-strong">{session.title || "—"}</h2>
                      {session.subtitle ? <p className="text-sm text-editorial-muted">{session.subtitle}</p> : null}
                    </div>

                    <div className="space-y-2">
                      <div className="flex flex-wrap items-center justify-between gap-2 text-sm text-editorial-muted">
                        <span>{t("playbackHistory.lastPlayedAt")}: {formatDateTime(session.updated_at, locale)}</span>
                        <span>
                          {t("playbackHistory.progress")}: {ratio}%
                        </span>
                      </div>
                      <div className="h-2 overflow-hidden rounded-full bg-[var(--color-editorial-line)]">
                        <div
                          className="h-full rounded-full bg-[var(--color-editorial-ink)] transition-[width]"
                          style={{ width: `${ratio}%` }}
                        />
                      </div>
                    </div>

                    <div className="mt-auto flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
                      <div className="flex flex-wrap gap-x-4 gap-y-2 text-sm text-editorial-muted">
                        <div className="flex flex-wrap items-center gap-2">
                          <span>{formatDuration(session.current_position_sec)} / {formatDuration(session.duration_sec)}</span>
                          {session.mode === "summary_queue" ? (
                            <>
                              <span>{t("playbackHistory.remainingItems").replace("{{count}}", String(summaryRemaining))}</span>
                              {summaryCurrent ? (
                                <span>
                                  {t("playbackHistory.currentItem").replace("{{title}}", summaryCurrent.translated_title || summaryCurrent.title || "—")}
                                </span>
                              ) : null}
                            </>
                          ) : null}
                        </div>
                      </div>

                      <div className="flex flex-wrap items-center gap-2 md:justify-end">
                        <button
                          type="button"
                          onClick={() => void handleResume(session)}
                          className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
                        >
                          <Play className="size-4" aria-hidden="true" />
                          {t("playbackHistory.continue")}
                        </button>
                        {href ? (
                          <Link
                            href={href}
                            className="inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                          >
                            {t("playbackHistory.openDetail")}
                          </Link>
                        ) : null}
                      </div>
                    </div>
                  </div>
                </SectionCard>
              );
            })}
          </div>
        ) : (
          <SectionCard>
            <p className="text-sm text-editorial-muted">{t("playbackHistory.empty")}</p>
          </SectionCard>
        )}
      </div>
    </PageTransition>
  );
}
