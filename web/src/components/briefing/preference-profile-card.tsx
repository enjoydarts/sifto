"use client";

import Link from "next/link";
import { PreferenceProfileSummary } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { SectionCard } from "@/components/ui/section-card";
import { Tag } from "@/components/ui/tag";

type PreferenceProfileCardProps = {
  summary: PreferenceProfileSummary | null;
};

function statusTone(status: string): "success" | "warning" | "info" {
  if (status === "active") return "success";
  if (status === "learning") return "warning";
  return "info";
}

export function PreferenceProfileCard({ summary }: PreferenceProfileCardProps) {
  const { t, locale } = useI18n();
  if (!summary) return null;

  const percent = Math.max(0, Math.min(100, Math.round(summary.confidence * 100)));
  const computedAtLabel = summary.computed_at
    ? new Date(summary.computed_at).toLocaleTimeString(locale === "ja" ? "ja-JP" : "en-US", {
        hour: "2-digit",
        minute: "2-digit",
      })
    : null;

  return (
    <SectionCard className="p-5 sm:p-6">
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("briefing.profile.eyebrow")}
            </div>
            <h2 className="mt-2 font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
              {t("briefing.profile.title")}
            </h2>
            <p className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
              {t(`briefing.profile.description.${summary.status}`, t("briefing.profile.description.default"))}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Tag tone={statusTone(summary.status)}>
              {t(`briefing.profile.status.${summary.status}`, summary.status)}
            </Tag>
            <Tag>{t("briefing.profile.feedbackCount").replace("{{count}}", String(summary.feedback_count))}</Tag>
          </div>
        </div>

        <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                {t("briefing.profile.progress")}
              </div>
              <div className="mt-2 text-[1.9rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)] tabular-nums">
                {percent}%
              </div>
            </div>
            <div className="min-w-0 flex-1">
              <div className="h-2 rounded-full bg-[#e9e1d3]">
                <div className="h-2 rounded-full bg-[var(--color-editorial-ink)]" style={{ width: `${Math.max(percent, 6)}%` }} />
              </div>
              <div className="mt-2 text-[12px] leading-5 text-[var(--color-editorial-ink-soft)]">
                {t("briefing.profile.strongestWeight").replace("{{weight}}", t(`itemDetail.score.${summary.strongest_weight}`, summary.strongest_weight))}
              </div>
            </div>
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_220px]">
          <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("briefing.profile.topTopics")}
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              {summary.top_topics.length > 0 ? (
                summary.top_topics.map((topic) => <Tag key={topic} tone="accent">{topic}</Tag>)
              ) : (
                <span className="text-sm text-[var(--color-editorial-ink-soft)]">{t("briefing.profile.noTopics")}</span>
              )}
            </div>
          </div>
          <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("briefing.profile.updatedAt")}
            </div>
            <div className="mt-2 text-sm text-[var(--color-editorial-ink-soft)]">
              {computedAtLabel ?? t("briefing.profile.notReady")}
            </div>
            <Link
              href="/settings"
              className="mt-4 inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
            >
              {t("briefing.profile.openSettings")}
            </Link>
          </div>
        </div>
      </div>
    </SectionCard>
  );
}
