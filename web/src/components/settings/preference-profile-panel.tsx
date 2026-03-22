"use client";

import { PreferenceProfile } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { Tag } from "@/components/ui/tag";

type PreferenceProfilePanelProps = {
  profile: PreferenceProfile | null;
  error?: string | null;
  onReset: () => void;
  onRetry: () => void;
  resetting: boolean;
};

const WEIGHT_KEYS = ["importance", "novelty", "actionability", "reliability", "relevance"] as const;

export function PreferenceProfilePanel({ profile, error, onReset, onRetry, resetting }: PreferenceProfilePanelProps) {
  const { t, locale } = useI18n();
  if (!profile) {
    if (error) {
      return (
        <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
          <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("settings.personalization.loadFailed")}</div>
          <p className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">{error}</p>
          <button
            type="button"
            onClick={onRetry}
            className="mt-4 inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)]"
          >
            {t("settings.personalization.retry")}
          </button>
        </div>
      );
    }
    return <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.personalization.unavailable")}</p>;
  }

  const percent = Math.max(0, Math.min(100, Math.round(profile.confidence * 100)));
  const computedAtLabel = profile.computed_at
    ? new Date(profile.computed_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")
    : t("settings.personalization.notReady");

  return (
    <div className="space-y-5">
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_220px]">
        <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.personalization.status")}
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Tag tone={profile.status === "active" ? "success" : profile.status === "learning" ? "warning" : "info"}>
                  {t(`settings.personalization.status.${profile.status}`, profile.status)}
                </Tag>
                <Tag>{t("settings.personalization.feedbackCount").replace("{{count}}", String(profile.feedback_count))}</Tag>
                <Tag>{t("settings.personalization.readCount").replace("{{count}}", String(profile.read_count))}</Tag>
              </div>
            </div>
            <div className="text-right">
              <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
                {t("settings.personalization.confidence")}
              </div>
              <div className="mt-2 text-[1.9rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)] tabular-nums">
                {percent}%
              </div>
            </div>
          </div>
          <div className="mt-4 h-2 rounded-full bg-[#e9e1d3]">
            <div className="h-2 rounded-full bg-[var(--color-editorial-ink)]" style={{ width: `${Math.max(percent, 6)}%` }} />
          </div>
          <p className="mt-3 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">
            {t(`settings.personalization.description.${profile.status}`, t("settings.personalization.description.default"))}
          </p>
        </div>

        <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
          <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
            {t("settings.personalization.updatedAt")}
          </div>
          <div className="mt-2 text-sm leading-6 text-[var(--color-editorial-ink-soft)]">{computedAtLabel}</div>
          <button
            type="button"
            onClick={onReset}
            disabled={resetting}
            className="mt-4 inline-flex min-h-10 items-center rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] disabled:opacity-60"
          >
            {resetting ? t("settings.personalization.resetting") : t("settings.personalization.reset")}
          </button>
        </div>
      </div>

      <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
        <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
          {t("settings.personalization.weights")}
        </div>
        <div className="mt-3 grid gap-3 md:grid-cols-2 xl:grid-cols-5">
          {WEIGHT_KEYS.map((key) => {
            const weight = profile.learned_weights[key];
            if (!weight) return null;
            const deltaTone: "success" | "warning" | "neutral" = weight.delta > 0 ? "success" : weight.delta < 0 ? "warning" : "neutral";
            return (
              <div key={key} className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-3">
                <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t(`itemDetail.score.${key}`, key)}</div>
                <div className="mt-2 flex items-center justify-between gap-2">
                  <div className="h-2 flex-1 rounded-full bg-[#e9e1d3]">
                    <div className="h-2 rounded-full bg-[var(--color-editorial-ink)]" style={{ width: `${Math.max(weight.value * 100, 4)}%` }} />
                  </div>
                  <span className="w-12 text-right text-xs font-medium tabular-nums text-[var(--color-editorial-ink-soft)]">
                    {weight.value.toFixed(2)}
                  </span>
                </div>
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  <Tag tone={deltaTone}>{weight.delta >= 0 ? `+${weight.delta.toFixed(2)}` : weight.delta.toFixed(2)}</Tag>
                  <span className="text-[11px] text-[var(--color-editorial-ink-faint)]">
                    {t("settings.personalization.defaultWeight").replace("{{value}}", weight.default.toFixed(2))}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
          <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
            {t("settings.personalization.topTopics")}
          </div>
          <div className="mt-3 space-y-3">
            {profile.top_topics.length > 0 ? (
              profile.top_topics.map((topic) => (
                <div key={topic.topic}>
                  <div className="flex items-center justify-between gap-3 text-sm text-[var(--color-editorial-ink-soft)]">
                    <span className="truncate">{topic.topic}</span>
                    <span className="tabular-nums text-[12px]">{topic.signal_count}</span>
                  </div>
                  <div className="mt-1.5 h-2 rounded-full bg-[#e9e1d3]">
                    <div className="h-2 rounded-full bg-[var(--color-editorial-ink)]" style={{ width: `${Math.max(topic.score * 100, 4)}%` }} />
                  </div>
                </div>
              ))
            ) : (
              <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.personalization.noTopics")}</p>
            )}
          </div>
        </div>

        <div className="space-y-4">
          <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.personalization.topSources")}
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              {profile.top_sources.length > 0 ? (
                profile.top_sources.map((source) => (
                  <Tag key={source.source_id} tone="accent">
                    {source.source_title || source.source_id}
                  </Tag>
                ))
              ) : (
                <span className="text-sm text-[var(--color-editorial-ink-soft)]">{t("settings.personalization.noSources")}</span>
              )}
            </div>
          </div>

          <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4">
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {t("settings.personalization.readingPattern")}
            </div>
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-3">
                <div className="text-[11px] text-[var(--color-editorial-ink-faint)]">{t("settings.personalization.avgReadScore")}</div>
                <div className="mt-1 text-[1.4rem] tabular-nums text-[var(--color-editorial-ink)]">{profile.reading_pattern.avg_score_read.toFixed(2)}</div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-3">
                <div className="text-[11px] text-[var(--color-editorial-ink-faint)]">{t("settings.personalization.avgSkippedScore")}</div>
                <div className="mt-1 text-[1.4rem] tabular-nums text-[var(--color-editorial-ink)]">{profile.reading_pattern.avg_score_skipped.toFixed(2)}</div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-3">
                <div className="text-[11px] text-[var(--color-editorial-ink-faint)]">{t("settings.personalization.favoriteRate")}</div>
                <div className="mt-1 text-[1.4rem] tabular-nums text-[var(--color-editorial-ink)]">{Math.round(profile.reading_pattern.favorite_rate * 100)}%</div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-3">
                <div className="text-[11px] text-[var(--color-editorial-ink-faint)]">{t("settings.personalization.noteRate")}</div>
                <div className="mt-1 text-[1.4rem] tabular-nums text-[var(--color-editorial-ink)]">{Math.round(profile.reading_pattern.note_rate * 100)}%</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
