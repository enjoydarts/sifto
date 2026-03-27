"use client";

import { useEffect, useEffectEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle, Play, Volume2 } from "lucide-react";
import { AINavigatorAvatar } from "@/components/briefing/ai-navigator-avatar";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { useSharedAudioPlayer } from "@/components/shared-audio-player/provider";
import type { SummaryAudioQueueKind } from "@/components/shared-audio-player/types";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";

const queueKinds: SummaryAudioQueueKind[] = ["unread", "later", "favorite"];

function parseQueueKind(raw: string | null): SummaryAudioQueueKind {
  return raw === "later" ? "later" : raw === "favorite" ? "favorite" : "unread";
}

export default function SummaryAudioPlayerPage() {
  const { t, locale } = useI18n();
  const router = useRouter();
  const searchParams = useSearchParams();
  const player = useSharedAudioPlayer();
  const queueKind = parseQueueKind(searchParams.get("queue"));

  const requestQueueStart = useEffectEvent(async () => {
    if (
      player.mode === "summary_queue" &&
      player.summaryQueue.queueKind === queueKind &&
      player.summaryQueue.queue.length > 0
    ) {
      return;
    }
    await player.startSummaryQueuePlayback(queueKind);
  });

  useEffect(() => {
    void requestQueueStart();
  }, [queueKind]);

  const detail = player.summaryQueue.currentItemDetail;
  const hasQueuedItem = player.summaryQueue.queue.length > 0 || Boolean(player.summaryQueue.currentItemID);
  const titleForDisplay =
    detail?.translated_title ||
    detail?.summary?.translated_title ||
    detail?.title ||
    t("summaryAudio.untitled");
  const originalTitle = detail?.title || t("summaryAudio.originalTitleEmpty");
  const sourceTitle = detail?.source_title || t("summaryAudio.sourceUnknown");
  const queueCountLabel = `${player.display.queueCount.toLocaleString(locale)} ${t("summaryAudio.queueCount")}`;

  return (
    <PageTransition>
      <div className="space-y-4">
        <PageHeader
          eyebrow={t("summaryAudio.eyebrow")}
          title={t("summaryAudio.title")}
          titleIcon={Volume2}
          description={t("sharedAudio.pageDescription")}
          meta={(
            <>
              <Tag tone="default">{queueCountLabel}</Tag>
              {player.display.queueProgressLabel ? <Tag tone="default">{player.display.queueProgressLabel}</Tag> : null}
            </>
          )}
          actions={(
            <div className="flex w-full flex-wrap items-center justify-end gap-2">
              <Link
                href="/playback-history"
                className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {t("nav.playbackHistory")}
              </Link>
              <Link
                href="/items"
                className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
              >
                {t("summaryAudio.backToItems")}
              </Link>
            </div>
          )}
        />

        {player.isPreparing || player.isPrefetching || player.playbackState === "finished" || player.errorMessage ? (
          <SectionCard>
            <div className="flex flex-wrap items-center gap-2 text-sm text-editorial-muted">
              {player.isPreparing ? (
                <Tag tone="default">
                  <span className="inline-flex items-center gap-1.5">
                    <LoaderCircle className="size-3.5 animate-spin" aria-hidden="true" />
                    <span>{t("summaryAudio.preparing")}</span>
                  </span>
                </Tag>
              ) : null}
              {player.isPrefetching ? <Tag tone="default">{t("summaryAudio.prefetching")}</Tag> : null}
              {player.playbackState === "finished" ? <Tag tone="default">{t("summaryAudio.finished")}</Tag> : null}
              {player.errorMessage ? <p className="text-sm text-red-600">{player.errorMessage}</p> : null}
            </div>
          </SectionCard>
        ) : null}

        <div className="grid gap-4 lg:grid-cols-[minmax(0,1.8fr)_minmax(320px,0.9fr)]">
          <SectionCard>
            <div className="space-y-4">
              <div className="space-y-2">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                  {t("summaryAudio.nowPlaying")}
                </div>
                <h2 className="font-serif text-2xl text-editorial-strong">{titleForDisplay}</h2>
                <p className="text-sm text-editorial-muted">{originalTitle}</p>
                <div className="flex flex-wrap items-center gap-2 text-sm text-editorial-muted">
                  <span>{sourceTitle}</span>
                  {player.display.personaKey ? (
                    <span className="inline-flex items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1 text-xs font-medium text-[var(--color-editorial-ink-soft)]">
                      <AINavigatorAvatar persona={player.display.personaKey} className="size-5" />
                      <span>{t("sharedAudio.persona", locale === "ja" ? "話者" : "Voice")}</span>
                      <span>{player.display.personaName || t(`settings.navigator.persona.${player.display.personaKey}`, player.display.personaKey)}</span>
                    </span>
                  ) : null}
                  {detail?.url ? (
                    <a
                      href={detail.url}
                      target="_blank"
                      rel="noreferrer"
                      className="text-[var(--color-editorial-accent)] underline-offset-2 hover:underline"
                    >
                      {t("summaryAudio.openSource")}
                    </a>
                  ) : null}
                </div>
              </div>

              <div className="rounded-[var(--radius-card)] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                <p className="whitespace-pre-wrap text-sm leading-7 text-editorial-strong">
                  {hasQueuedItem ? detail?.summary?.summary || t("summaryAudio.summaryPending") : t("summaryAudio.empty")}
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
              {player.summaryQueue.queue.length === 0 ? (
                <p className="text-sm text-editorial-muted">{t("summaryAudio.empty")}</p>
              ) : (
                <div className="space-y-2">
                  {player.summaryQueue.queue.slice(0, 12).map((item, index) => {
                    const isActive = player.summaryQueue.currentItemID === item.id;
                    const queueSourceTitle = item.source_title || t("summaryAudio.sourceUnknown");
                    return (
                      <button
                        key={item.id}
                        type="button"
                        onClick={() => void player.selectSummaryQueueItem(index)}
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
