"use client";

import { useEffect, useEffectEvent, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, LoaderCircle, Play, Volume2 } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { useSharedAudioPlayer } from "@/components/shared-audio-player/provider";
import { api } from "@/lib/api";
import { getSummaryAudioReadiness } from "@/lib/summary-audio-readiness";
import type { SummaryAudioQueueKind } from "@/components/shared-audio-player/types";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";

const queueKinds: SummaryAudioQueueKind[] = ["unread", "later", "favorite"];

function parseQueueKind(raw: string | null): SummaryAudioQueueKind {
  if (raw === "later") return "later";
  if (raw === "favorite") return "favorite";
  if (raw === "brief") return "brief";
  return "unread";
}

export default function SummaryAudioPlayerPage() {
  const { t, locale } = useI18n();
  const router = useRouter();
  const searchParams = useSearchParams();
  const player = useSharedAudioPlayer();
  const queueKind = parseQueueKind(searchParams.get("queue"));
  const autoStartedQueueKindRef = useRef<SummaryAudioQueueKind | null>(null);
  const [showPreprocessedText, setShowPreprocessedText] = useState(false);
  const settingsQuery = useQuery({
    queryKey: ["settings", "summary-audio-readiness"],
    queryFn: () => api.getSettings(),
  });
  const summaryAudioReadiness = getSummaryAudioReadiness(settingsQuery.data ?? null);
  const summaryAudioSettingsLoaded = settingsQuery.isSuccess;
  const summaryAudioPlaybackBlocked = summaryAudioSettingsLoaded && !summaryAudioReadiness.ready;

  const requestQueueStart = useEffectEvent(async () => {
    if (!summaryAudioSettingsLoaded) {
      return;
    }
    if (queueKind === "brief") {
      return;
    }
    if (summaryAudioPlaybackBlocked) {
      return;
    }
    if (autoStartedQueueKindRef.current === queueKind) {
      return;
    }
    if (
      player.mode === "summary_queue" &&
      player.summaryQueue.queueKind === queueKind
    ) {
      return;
    }
    autoStartedQueueKindRef.current = queueKind;
    await player.startSummaryQueuePlayback(queueKind);
  });

  useEffect(() => {
    autoStartedQueueKindRef.current = null;
    setShowPreprocessedText(false);
  }, [queueKind]);

  useEffect(() => {
    setShowPreprocessedText(false);
  }, [player.summaryQueue.currentItemID]);

  useEffect(() => {
    void requestQueueStart();
  }, [queueKind, requestQueueStart, summaryAudioSettingsLoaded, summaryAudioReadiness.ready]);

  const detail = player.summaryQueue.currentItemDetail;
  const hasQueuedItem = player.summaryQueue.queue.length > 0 || Boolean(player.summaryQueue.currentItemID);
  const titleForDisplay =
    detail?.translated_title ||
    detail?.summary?.translated_title ||
    detail?.title ||
    t("summaryAudio.untitled");
  const originalTitle = detail?.title || t("summaryAudio.originalTitleEmpty");
  const sourceTitle = detail?.source_title || t("summaryAudio.sourceUnknown");
  const preprocessedText = player.summaryQueue.currentPreprocessedText;
  const showFishPreprocessedText = Boolean(preprocessedText);
  const queueCountLabel = `${player.display.queueCount.toLocaleString(locale)} ${t("summaryAudio.queueCount")}`;
  const queueButtons =
    queueKind === "brief" ? [...queueKinds, "brief" as SummaryAudioQueueKind] : queueKinds;
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

              {showFishPreprocessedText ? (
                <div className="rounded-[var(--radius-card)] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)]">
                  <button
                    type="button"
                    onClick={() => setShowPreprocessedText((prev) => !prev)}
                    aria-expanded={showPreprocessedText}
                    className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left"
                  >
                    <div className="space-y-1">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[var(--color-editorial-ink-faint)]">
                        {t("summaryAudio.preprocessedTextLabel")}
                      </div>
                      <p className="text-sm text-editorial-muted">{t("summaryAudio.preprocessedTextHelp")}</p>
                    </div>
                    <ChevronDown
                      className={`size-4 shrink-0 text-[var(--color-editorial-ink-faint)] transition ${showPreprocessedText ? "rotate-180" : ""}`}
                      aria-hidden="true"
                    />
                  </button>
                  {showPreprocessedText ? (
                    <div className="border-t border-[var(--color-editorial-line)] px-4 py-4">
                      <p className="whitespace-pre-wrap text-sm leading-7 text-editorial-strong">
                        {preprocessedText}
                      </p>
                    </div>
                  ) : null}
                </div>
              ) : null}
            </div>
          </SectionCard>

          <SectionCard>
            <div className="space-y-3">
              <div className="flex flex-wrap items-center gap-2">
                {queueButtons.map((kind) => {
                  const active = queueKind === kind;
                  const disabled = summaryAudioPlaybackBlocked || (kind === "brief" && player.summaryQueue.queueKind !== "brief");
                  return (
                    <button
                      key={kind}
                      type="button"
                      onClick={() => {
                        if (disabled) return;
                        if (kind === "brief" && player.summaryQueue.queueKind !== "brief") return;
                        router.replace(`/audio-player?queue=${kind}`);
                      }}
                      disabled={disabled}
                      className={`inline-flex min-h-10 items-center justify-center rounded-full border px-4 py-2 text-sm font-medium ${
                        active
                          ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                          : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-default disabled:opacity-50"
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
              {summaryAudioPlaybackBlocked ? (
                <div className="rounded-[18px] border border-[rgba(245,158,11,0.35)] bg-[rgba(255,251,235,0.82)] p-4">
                  <div className="text-sm font-semibold text-[#b45309]">{t("summaryAudio.playbackBlocked.title")}</div>
                  <p className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                    {t(summaryAudioReadiness.reasonKey || "summaryAudio.playbackBlocked.notConfigured")}
                  </p>
                  <div className="mt-3">
                    <Link
                      href="/settings?section=summary-audio"
                      className="inline-flex min-h-10 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-white px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] hover:bg-[var(--color-editorial-panel-strong)]"
                    >
                      {t("summaryAudio.playbackBlocked.openSettings")}
                    </Link>
                  </div>
                </div>
              ) : null}
              {player.summaryQueue.queue.length === 0 ? (
                <p className="text-sm text-editorial-muted">{t("summaryAudio.empty")}</p>
              ) : (
                <div className="space-y-2">
                  {player.summaryQueue.queue.slice(0, 12).map((item, index) => {
                    const isActive = player.summaryQueue.currentItemID === item.id;
                    const queueSourceTitle = item.source_title || t("summaryAudio.sourceUnknown");
                    const disabled = summaryAudioPlaybackBlocked;
                    return (
                      <button
                        key={item.id}
                        type="button"
                        onClick={() => {
                          if (disabled) return;
                          void player.selectSummaryQueueItem(index);
                        }}
                        disabled={disabled}
                        className={`group flex w-full items-start justify-between gap-3 rounded-[var(--radius-card)] border px-4 py-3 text-left transition hover:-translate-y-0.5 hover:shadow-[0_12px_30px_rgba(15,23,42,0.08)] focus:outline-none focus:ring-2 focus:ring-[var(--color-editorial-accent)] ${
                          isActive
                            ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-accent-soft)]"
                            : "cursor-pointer border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] hover:border-[var(--color-editorial-ink-faint)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-50"
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
