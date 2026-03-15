"use client";

import { Source, SourceOptimizationItem } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

interface SourceOptimizationPanelProps {
  items: SourceOptimizationItem[];
  sources: Source[];
}

const toneMap: Record<string, string> = {
  keep: "border-emerald-200 bg-emerald-50 text-emerald-800",
  promote: "border-sky-200 bg-sky-50 text-sky-800",
  mute: "border-amber-200 bg-amber-50 text-amber-800",
  prune: "border-rose-200 bg-rose-50 text-rose-800",
};

export function SourceOptimizationPanel({ items, sources }: SourceOptimizationPanelProps) {
  const { t } = useI18n();
  if (items.length === 0) return null;
  const sourceMap = new Map(sources.map((source) => [source.id, source]));

  const localizeRecommendation = (value: string) => {
    switch (value) {
      case "keep":
        return t("sources.optimization.recommendation.keep");
      case "promote":
        return t("sources.optimization.recommendation.promote");
      case "mute":
        return t("sources.optimization.recommendation.mute");
      case "prune":
        return t("sources.optimization.recommendation.prune");
      default:
        return value;
    }
  };

  const localizeReason = (value: string) => {
    switch (value) {
      case "high engagement and high value":
        return t("sources.optimization.reason.promote");
      case "large backlog with low engagement":
        return t("sources.optimization.reason.prune");
      case "backlog is growing and notifications are ignored":
        return t("sources.optimization.reason.mute");
      case "source is still useful":
        return t("sources.optimization.reason.keep");
      default:
        return value;
    }
  };

  return (
    <section className="rounded-3xl border border-zinc-200 bg-white p-5 shadow-sm">
      <div>
        <h2 className="text-lg font-semibold text-zinc-950">{t("sources.optimization.title")}</h2>
        <p className="mt-1 text-sm text-zinc-500">{t("sources.optimization.subtitle")}</p>
      </div>
      <div className="mt-4 grid gap-3">
        {items.map((item) => {
          const source = sourceMap.get(item.source_id);
          return (
            <div key={item.source_id} className="rounded-2xl border border-zinc-200 p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="text-sm font-semibold text-zinc-900">{source?.title || source?.url || item.source_id}</div>
                  <p className="mt-1 text-sm text-zinc-500">{localizeReason(item.reason)}</p>
                </div>
                <span className={`rounded-full border px-3 py-1 text-xs font-medium ${toneMap[item.recommendation] ?? "border-zinc-200 bg-zinc-50 text-zinc-700"}`}>
                  {localizeRecommendation(item.recommendation)}
                </span>
              </div>
              <div className="mt-3 grid gap-2 text-xs text-zinc-600 sm:grid-cols-4">
                <div>{t("sources.optimization.backlog")}: {item.metrics.unread_backlog}</div>
                <div>{t("sources.optimization.readRate")}: {Math.round(item.metrics.read_rate * 100)}%</div>
                <div>{t("sources.optimization.favoriteRate")}: {Math.round(item.metrics.favorite_rate * 100)}%</div>
                <div>{t("sources.optimization.avgScore")}: {item.metrics.average_summary_score.toFixed(2)}</div>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
