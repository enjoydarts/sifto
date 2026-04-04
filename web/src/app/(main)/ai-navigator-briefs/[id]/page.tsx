"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { Brain, Play } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { useSharedAudioPlayer } from "@/components/shared-audio-player/provider";
import { useToast } from "@/components/toast-provider";
import { Tag } from "@/components/ui/tag";
import { api } from "@/lib/api";
import { getSummaryAudioReadiness } from "@/lib/summary-audio-readiness";

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

function MetaPill({ children }: { children: React.ReactNode }) {
  return (
    <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-[var(--color-editorial-ink-soft)]">
      {children}
    </span>
  );
}

function ReadableBlock({ title, body }: { title: string; body: string | null | undefined }) {
  return (
    <section className="surface-editorial rounded-[28px] px-5 py-5 sm:px-6">
      <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">{title}</div>
      <div className="mt-4 whitespace-pre-wrap break-words font-serif text-[18px] leading-[1.95] text-[var(--color-editorial-ink)]">
        {body || "—"}
      </div>
    </section>
  );
}

export default function AINavigatorBriefDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const { t, locale } = useI18n();
  const { showToast } = useToast();
  const player = useSharedAudioPlayer();
  const briefID = typeof params?.id === "string" ? params.id : "";
  const detailQuery = useQuery({
    queryKey: ["ai-navigator-brief", briefID],
    queryFn: () => api.getAINavigatorBrief(briefID),
    enabled: briefID.length > 0,
  });
  const brief = detailQuery.data?.brief ?? null;
  const settingsQuery = useQuery({
    queryKey: ["settings", "summary-audio-readiness"],
    queryFn: () => api.getSettings(),
  });
  const summaryAudioReadiness = getSummaryAudioReadiness(settingsQuery.data ?? null);

  async function handlePlayAll() {
    if (!briefID) return;
    if (!summaryAudioReadiness.ready) return;
    const queue = await api.appendAINavigatorBriefToSummaryAudioQueue(briefID);
    await player.startSummaryQueuePlayback("brief", queue.items);
    router.push("/audio-player?queue=brief");
    showToast(t("aiNavigatorBriefs.toast.queueAdded"), "success");
  }

  if (detailQuery.isLoading) {
    return (
      <PageTransition>
        <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>
      </PageTransition>
    );
  }

  return (
    <PageTransition>
      <div className="min-w-0 space-y-5 overflow-x-hidden">
        <Link
          href="/ai-navigator-briefs"
          className="inline-flex items-center gap-2 text-sm text-[var(--color-editorial-ink-soft)] hover:text-[var(--color-editorial-ink)]"
        >
          ← {t("aiNavigatorBriefs.backToList")}
        </Link>

        {!brief ? (
          <section className="surface-editorial rounded-[28px] px-5 py-5 sm:px-6">
            <p className="text-sm text-editorial-muted">{t("aiNavigatorBriefs.empty")}</p>
          </section>
        ) : (
          <>
            <section
              className="rounded-[30px] border border-[var(--color-editorial-line)] px-5 py-5 shadow-[var(--shadow-card)] sm:px-6"
              style={{
                background: "linear-gradient(180deg, rgba(255,255,255,0.72), rgba(255,253,249,0.96)), #fbf8f2",
              }}
            >
              <div className="flex flex-wrap items-center gap-2 text-xs">
                <Tag tone="default">{t(`aiNavigatorBriefs.slot.${brief.slot}`)}</Tag>
                <Tag tone="default">{t(`aiNavigatorBriefs.status.${brief.status}`)}</Tag>
                <Tag tone="default">{formatDateTime(brief.generated_at ?? brief.created_at, locale)}</Tag>
              </div>
              <div className="mt-4 flex items-center gap-3">
                <span className="inline-flex size-12 items-center justify-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] text-[var(--color-editorial-ink)]">
                  <Brain className="size-5" aria-hidden="true" />
                </span>
                <div className="min-w-0">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("aiNavigatorBriefs.eyebrow")}
                  </div>
                  <h1 className="mt-2 font-serif text-[2.25rem] leading-[1.08] tracking-[-0.04em] text-[var(--color-editorial-ink)] sm:text-[3.2rem]">
                    {brief.title || t("aiNavigatorBriefs.title")}
                  </h1>
                </div>
              </div>
              <p className="mt-4 max-w-3xl text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("aiNavigatorBriefs.detailDescription")}
              </p>
              <div className="mt-5 flex flex-wrap gap-2 text-xs">
                <MetaPill>{t("aiNavigatorBriefs.generatedAt")} {formatDateTime(brief.generated_at ?? brief.created_at, locale)}</MetaPill>
                <MetaPill>{t("aiNavigatorBriefs.persona")} {t(`settings.navigator.persona.${brief.persona}`, brief.persona)}</MetaPill>
                <MetaPill>{t("aiNavigatorBriefs.model")} {brief.model || "—"}</MetaPill>
                {brief.notification_sent_at ? (
                  <MetaPill>{t("aiNavigatorBriefs.notificationSentAt")} {formatDateTime(brief.notification_sent_at, locale)}</MetaPill>
                ) : null}
              </div>
              <div className="mt-5 flex flex-wrap gap-3">
                <button
                  type="button"
                  onClick={() => void handlePlayAll()}
                  disabled={!summaryAudioReadiness.ready}
                  className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Play className="size-4" aria-hidden="true" />
                  {t("aiNavigatorBriefs.playAll")}
                </button>
                {!summaryAudioReadiness.ready ? (
                  <Link
                    href="/settings?section=summary-audio"
                    className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                  >
                    {t("summaryAudio.playbackBlocked.openSettings")}
                  </Link>
                ) : null}
              </div>
              {!summaryAudioReadiness.ready ? (
                <div className="mt-4 rounded-[18px] border border-[rgba(245,158,11,0.35)] bg-[rgba(255,251,235,0.82)] px-4 py-4 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">
                  <div className="font-semibold text-[#b45309]">{t("summaryAudio.playbackBlocked.title")}</div>
                  <p className="mt-2">{t(summaryAudioReadiness.reasonKey || "summaryAudio.playbackBlocked.notConfigured")}</p>
                </div>
              ) : null}
              {brief.error_message ? (
                <div className="mt-5 border-t border-[var(--color-editorial-line)] pt-4 text-sm leading-7 text-[#7a4337]">
                  {brief.error_message}
                </div>
              ) : null}
            </section>

            <ReadableBlock title={t("aiNavigatorBriefs.introTitle")} body={brief.intro} />
            <ReadableBlock title={t("aiNavigatorBriefs.summaryTitle")} body={brief.summary} />

            <section className="surface-editorial rounded-[28px] px-5 py-5 sm:px-6">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                  {t("aiNavigatorBriefs.itemsTitle")}
                </div>
                <button
                  type="button"
                  onClick={() => void handlePlayAll()}
                  disabled={!summaryAudioReadiness.ready}
                  className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Play className="size-4" aria-hidden="true" />
                  {t("aiNavigatorBriefs.playAll")}
                </button>
              </div>
              <div className="mt-4 grid gap-3">
                {(brief.items ?? []).map((item) => (
                  <article
                    key={item.id}
                    className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[rgba(255,255,255,0.62)] p-4"
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-soft)]">
                        #{item.rank}
                      </span>
                      <Link
                        href={`/items/${item.item_id}`}
                        className="text-sm font-semibold text-[var(--color-editorial-ink)] hover:underline"
                      >
                        {item.translated_title_snapshot || item.title_snapshot || "—"}
                      </Link>
                    </div>
                    <div className="mt-2 text-xs text-[var(--color-editorial-ink-soft)]">{item.source_title_snapshot || "—"}</div>
                    <div className="mt-4 whitespace-pre-wrap break-words font-serif text-[17px] leading-[1.95] text-[var(--color-editorial-ink)]">
                      {item.comment}
                    </div>
                  </article>
                ))}
              </div>
            </section>

            <ReadableBlock title={t("aiNavigatorBriefs.endingTitle")} body={brief.ending} />
          </>
        )}
      </div>
    </PageTransition>
  );
}
