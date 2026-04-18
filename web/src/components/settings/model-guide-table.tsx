"use client";

import type { LLMCatalogModel } from "@/lib/api";
import { formatModelDisplayName, providerLabel } from "@/lib/model-display";

function formatModelPriceCell(
  pricing: LLMCatalogModel["pricing"],
  kind: "input" | "output"
): string {
  if (!pricing) return "-";
  const value = kind === "input" ? pricing.input_per_mtok_usd : pricing.output_per_mtok_usd;
  if (value <= 0) return "-";
  const rounded = value >= 1 ? value.toFixed(2) : value.toFixed(4);
  return `$${rounded.replace(/\.?0+$/, "")}`;
}

export default function ModelGuideTable({
  entries,
  t,
}: {
  entries: LLMCatalogModel[];
  t: (key: string, fallback?: string) => string;
}) {
  return (
    <table className="min-w-[1600px] table-auto border-separate border-spacing-0 text-sm">
      <thead>
        <tr className="border-b border-zinc-200 text-xs font-semibold uppercase tracking-wide text-zinc-500">
          <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.model")}</th>
          <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.provider")}</th>
          <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.inputPrice")}</th>
          <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.outputPrice")}</th>
          <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.recommendation")}</th>
          <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.highlights")}</th>
          <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.bestFor")}</th>
          <th className="border-b border-zinc-200 px-3 pb-2 text-left">{t("settings.modelGuide.columns.comment")}</th>
        </tr>
      </thead>
      <tbody>
        {entries.map((entry) => (
          <tr key={entry.id} className="text-zinc-700">
            <td className="border-b border-zinc-100 px-3 py-3 align-top">
              <div className="whitespace-nowrap font-medium text-zinc-900">{formatModelDisplayName(entry.id)}</div>
            </td>
            <td className="border-b border-zinc-100 px-3 py-3 align-top whitespace-nowrap text-zinc-600">
              {providerLabel(entry.provider)}
            </td>
            <td className="border-b border-zinc-100 px-3 py-3 align-top whitespace-nowrap text-zinc-600">
              {formatModelPriceCell(entry.pricing, "input")}
            </td>
            <td className="border-b border-zinc-100 px-3 py-3 align-top whitespace-nowrap text-zinc-600">
              {formatModelPriceCell(entry.pricing, "output")}
            </td>
            <td className="border-b border-zinc-100 px-3 py-3 align-top whitespace-nowrap">
              <span
                className={`inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${
                  entry.recommendation === "recommended"
                    ? "bg-emerald-50 text-emerald-700"
                    : entry.recommendation === "strong"
                      ? "bg-blue-50 text-blue-700"
                      : "bg-zinc-100 text-zinc-700"
                }`}
              >
                {t(`settings.modelGuide.recommendation.${entry.recommendation}`)}
              </span>
            </td>
            <td className="border-b border-zinc-100 px-3 py-3 align-top">
              <div className="flex flex-wrap gap-1.5">
                {(entry.highlights ?? []).length > 0 ? (entry.highlights ?? []).map((highlight) => (
                  <span
                    key={highlight}
                    className="inline-flex whitespace-nowrap rounded-full bg-zinc-100 px-2.5 py-1 text-xs font-medium text-zinc-700"
                  >
                    {t(`settings.modelGuide.highlights.${highlight}`)}
                  </span>
                )) : (
                  <span className="text-zinc-400">-</span>
                )}
              </div>
            </td>
            <td className="border-b border-zinc-100 px-3 py-3 align-top whitespace-nowrap text-zinc-600">
              {entry.best_for ? t(`settings.modelGuide.bestFor.${entry.best_for}`) : "-"}
            </td>
            <td className="border-b border-zinc-100 px-3 py-3 align-top whitespace-nowrap text-xs leading-5 text-zinc-600">
              {entry.comment ?? "-"}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
