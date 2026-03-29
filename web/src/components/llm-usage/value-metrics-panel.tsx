"use client";

import type { LLMValueMetric } from "@/lib/api";
import { formatModelDisplayName } from "@/lib/model-display";

function providerLabel(provider: string) {
  switch (provider) {
    case "openai":
      return "OpenAI";
    case "anthropic":
      return "Anthropic";
    case "google":
      return "Google";
    case "groq":
      return "Groq";
    case "deepseek":
      return "DeepSeek";
    case "alibaba":
      return "Alibaba";
    case "mistral":
      return "Mistral";
    case "xai":
      return "xAI";
    case "zai":
      return "Z.ai";
    case "fireworks":
      return "Fireworks";
    case "openrouter":
      return "OpenRouter";
    case "siliconflow":
      return "SiliconFlow";
    default:
      return provider;
  }
}

function buildReason(row: LLMValueMetric, labels: {
  benchmarkPrefix: string;
  keepInsight: string;
  keepFavorite: string;
  keepRead: string;
  keepDefault: string;
  lowSignal: string;
  reviewHigher: string;
  metricRead: string;
  metricFavorite: string;
  metricInsight: string;
}) {
  if (row.advisory_code === "review_model" && row.benchmark_provider && row.benchmark_model) {
    const metric =
      row.benchmark_metric === "insight"
        ? labels.metricInsight
        : row.benchmark_metric === "favorite"
          ? labels.metricFavorite
          : labels.metricRead;
    return `${metric}${labels.reviewHigher}${providerLabel(row.benchmark_provider)}/${formatModelDisplayName(row.benchmark_model)}`;
  }
  if (row.advisory_code === "low_signal") {
    return labels.lowSignal;
  }
  if (row.insight_count > 0) return labels.keepInsight;
  if (row.favorite_count > 0) return labels.keepFavorite;
  if (row.read_count > 0) return labels.keepRead;
  return labels.keepDefault;
}

export function ValueMetricsPanel({
  title,
  subtitle,
  rows,
  emptyLabel,
  fmtUSD,
  sortKey,
  sortDir,
  onSort,
  advisoryLabels,
}: {
  title: string;
  subtitle: string;
  rows: LLMValueMetric[];
  emptyLabel: string;
  fmtUSD: (v: number) => string;
  sortKey: string;
  sortDir: "asc" | "desc";
  onSort: (key: string) => void;
  advisoryLabels: {
    costToRead: string;
    costToFavorite: string;
    costToInsight: string;
    lowSignal: string;
    reviewModel: string;
    ok: string;
    benchmarkPrefix: string;
    keepInsight: string;
    keepFavorite: string;
    keepRead: string;
    keepDefault: string;
    reviewHigher: string;
    metricRead: string;
    metricFavorite: string;
    metricInsight: string;
  };
}) {
  const renderSortMark = (key: string) => {
    if (sortKey !== key) return null;
    return <span className="ml-1 text-zinc-400">{sortDir === "asc" ? "↑" : "↓"}</span>;
  };
  return (
    <section className="rounded-lg border border-zinc-200 bg-white p-4">
      <div className="mb-3">
        <h2 className="text-sm font-semibold text-zinc-800">{title}</h2>
        <p className="mt-1 text-sm text-zinc-500">{subtitle}</p>
      </div>
      {rows.length === 0 ? (
        <p className="text-sm text-zinc-400">{emptyLabel}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="text-xs text-zinc-500">
              <tr className="border-b border-zinc-100">
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("purpose")} className="inline-flex items-center hover:text-zinc-800">purpose{renderSortMark("purpose")}</button></th>
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("model")} className="inline-flex items-center hover:text-zinc-800">model{renderSortMark("model")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("cost_to_read_usd")} className="inline-flex items-center hover:text-zinc-800">{advisoryLabels.costToRead}{renderSortMark("cost_to_read_usd")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("cost_to_favorite_usd")} className="inline-flex items-center hover:text-zinc-800">{advisoryLabels.costToFavorite}{renderSortMark("cost_to_favorite_usd")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("cost_to_insight_usd")} className="inline-flex items-center hover:text-zinc-800">{advisoryLabels.costToInsight}{renderSortMark("cost_to_insight_usd")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("total_cost_usd")} className="inline-flex items-center hover:text-zinc-800">cost{renderSortMark("total_cost_usd")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("calls")} className="inline-flex items-center hover:text-zinc-800">calls{renderSortMark("calls")}</button></th>
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("advisory_code")} className="inline-flex items-center hover:text-zinc-800">advisory{renderSortMark("advisory_code")}</button></th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                let advisory = advisoryLabels.ok;
                if (row.advisory_code === "low_signal") advisory = advisoryLabels.lowSignal;
                if (row.advisory_code === "review_model") advisory = advisoryLabels.reviewModel;
                const reason = buildReason(row, advisoryLabels);
                return (
                  <tr key={`${row.purpose}:${row.provider}:${row.model}`} className="border-b border-zinc-100 last:border-0">
                    <td className="px-3 py-2 font-medium text-zinc-800">{row.purpose}</td>
                    <td className="px-3 py-2 text-zinc-700">
                      <div className="font-medium">{providerLabel(row.provider)}</div>
                      <div className="text-xs text-zinc-500">{formatModelDisplayName(row.model)}</div>
                    </td>
                    <td className="px-3 py-2 text-right">{row.cost_to_read_usd != null ? fmtUSD(row.cost_to_read_usd) : "—"}</td>
                    <td className="px-3 py-2 text-right">{row.cost_to_favorite_usd != null ? fmtUSD(row.cost_to_favorite_usd) : "—"}</td>
                    <td className="px-3 py-2 text-right">{row.cost_to_insight_usd != null ? fmtUSD(row.cost_to_insight_usd) : "—"}</td>
                    <td className="px-3 py-2 text-right">{fmtUSD(row.total_cost_usd)}</td>
                    <td className="px-3 py-2 text-right">{row.calls}</td>
                    <td className="px-3 py-2 text-zinc-700">
                      <div className={row.low_efficiency_flag ? "font-medium text-amber-700" : "text-zinc-600"}>{advisory}</div>
                      <div className="mt-1 whitespace-nowrap text-xs leading-5 text-zinc-500">{reason}</div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
