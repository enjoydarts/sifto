"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { Brain, Play } from "lucide-react";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { useSharedAudioPlayer } from "@/components/shared-audio-player/provider";
import { useToast } from "@/components/toast-provider";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";
import { api } from "@/lib/api";

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

  async function handlePlayAll() {
    if (!briefID) return;
    const queue = await api.appendAINavigatorBriefToSummaryAudioQueue(briefID);
    await player.startSummaryQueuePlayback("brief", queue.items);
    router.push("/audio-player?queue=brief");
    showToast(t("aiNavigatorBriefs.toast.queueAdded"), "success");
  }

  return (
    <PageTransition>
      <div className="space-y-4">
        <PageHeader
          eyebrow={t("aiNavigatorBriefs.eyebrow")}
          title={brief?.title || t("aiNavigatorBriefs.title")}
          titleIcon={Brain}
          description={t("aiNavigatorBriefs.detailDescription")}
          meta={brief ? (
            <>
              <Tag tone="default">{t(`aiNavigatorBriefs.slot.${brief.slot}`)}</Tag>
              <Tag tone="default">{t(`aiNavigatorBriefs.status.${brief.status}`)}</Tag>
              <Tag tone="default">{formatDateTime(brief.generated_at ?? brief.created_at, locale)}</Tag>
            </>
          ) : undefined}
          actions={brief ? (
            <button
              type="button"
              onClick={() => void handlePlayAll()}
              className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-panel-strong)] hover:opacity-90"
            >
              <Play className="size-4" aria-hidden="true" />
              {t("aiNavigatorBriefs.playAll")}
            </button>
          ) : undefined}
        />

        {detailQuery.isLoading ? (
          <SectionCard>
            <p className="text-sm text-editorial-muted">{t("common.loading")}</p>
          </SectionCard>
        ) : !brief ? (
          <SectionCard>
            <p className="text-sm text-editorial-muted">{t("aiNavigatorBriefs.empty")}</p>
          </SectionCard>
        ) : (
          <div className="space-y-4">
            <SectionCard>
              <div className="grid gap-3 md:grid-cols-2">
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("aiNavigatorBriefs.generatedAt")}
                  </div>
                  <p className="mt-1 text-sm text-editorial-strong">{formatDateTime(brief.generated_at ?? brief.created_at, locale)}</p>
                </div>
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("aiNavigatorBriefs.persona")}
                  </div>
                  <p className="mt-1 text-sm text-editorial-strong">{t(`settings.navigator.persona.${brief.persona}`, brief.persona)}</p>
                </div>
                {brief.notification_sent_at ? (
                  <div>
                    <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                      {t("aiNavigatorBriefs.notificationSentAt")}
                    </div>
                    <p className="mt-1 text-sm text-editorial-strong">{formatDateTime(brief.notification_sent_at, locale)}</p>
                  </div>
                ) : null}
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("aiNavigatorBriefs.model")}
                  </div>
                  <p className="mt-1 break-all text-sm text-editorial-strong">{brief.model || "—"}</p>
                </div>
              </div>
              {brief.error_message ? (
                <p className="mt-4 text-sm text-red-600">{brief.error_message}</p>
              ) : null}
            </SectionCard>

            <SectionCard>
              <div className="space-y-4">
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("aiNavigatorBriefs.introTitle")}
                  </div>
                  <p className="mt-2 whitespace-pre-wrap text-sm leading-7 text-editorial-strong">{brief.intro || "—"}</p>
                </div>
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("aiNavigatorBriefs.summaryTitle")}
                  </div>
                  <p className="mt-2 whitespace-pre-wrap text-sm leading-7 text-editorial-strong">{brief.summary || "—"}</p>
                </div>
              </div>
            </SectionCard>

            <SectionCard>
              <div className="space-y-3">
                <div className="flex items-center justify-between gap-3">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                    {t("aiNavigatorBriefs.itemsTitle")}
                  </div>
                  <button
                    type="button"
                    onClick={() => void handlePlayAll()}
                    className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                  >
                    <Play className="size-4" aria-hidden="true" />
                    {t("aiNavigatorBriefs.playAll")}
                  </button>
                </div>
                <div className="space-y-2">
                  {(brief.items ?? []).map((item) => (
                    <div
                      key={item.id}
                      className="rounded-[var(--radius-card)] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4"
                    >
                      <div className="flex flex-wrap items-center gap-2">
                        <Tag tone="default">{item.rank}</Tag>
                        <Link
                          href={`/items/${item.item_id}`}
                          className="text-sm font-semibold text-editorial-strong hover:underline"
                        >
                          {item.translated_title_snapshot || item.title_snapshot || "—"}
                        </Link>
                      </div>
                      <p className="mt-1 text-xs text-editorial-muted">{item.source_title_snapshot || "—"}</p>
                      <p className="mt-3 whitespace-pre-wrap text-sm leading-7 text-editorial-strong">{item.comment}</p>
                    </div>
                  ))}
                </div>
              </div>
            </SectionCard>
          </div>
        )}
      </div>
    </PageTransition>
  );
}
