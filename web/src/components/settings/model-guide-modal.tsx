"use client";

import { useMemo, useState } from "react";
import type { LLMCatalogModel } from "@/lib/api";
import ModelGuideTable from "@/components/settings/model-guide-table";

export default function ModelGuideModal({
  open,
  onClose,
  entries,
  t,
}: {
  open: boolean;
  onClose: () => void;
  entries: LLMCatalogModel[];
  t: (key: string, fallback?: string) => string;
}) {
  const [providerFilter, setProviderFilter] = useState<string>("all");
  const [query, setQuery] = useState("");

  const providerOptions = useMemo(
    () => ["all", ...Array.from(new Set(entries.map((entry) => entry.provider))).sort()],
    [entries]
  );

  const filteredEntries = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return entries.filter((entry) => {
      if (providerFilter !== "all" && entry.provider !== providerFilter) {
        return false;
      }
      if (!normalizedQuery) {
        return true;
      }
      const haystack = [entry.id, entry.provider].join(" ").toLowerCase();
      return haystack.includes(normalizedQuery);
    });
  }, [entries, providerFilter, query]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
      onClick={onClose}
    >
      <div className="flex max-h-[90vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-2xl">
        <div
          className="flex max-h-[90vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-2xl"
          onClick={(event) => event.stopPropagation()}
        >
        <div className="flex items-start justify-between gap-4 border-b border-zinc-200 px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-zinc-900">{t("settings.modelGuide.title")}</h2>
            <p className="mt-1 text-sm text-zinc-500">{t("settings.modelGuide.description")}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-zinc-300 bg-white px-3 py-1.5 text-sm font-medium text-zinc-700 hover:border-zinc-400 hover:text-zinc-900"
          >
            {t("common.close")}
          </button>
        </div>
        <div className="border-b border-zinc-200 px-5 py-4">
          <div className="grid gap-3 sm:grid-cols-[220px_minmax(0,1fr)]">
            <label className="space-y-1">
              <span className="text-xs font-medium text-zinc-600">{t("settings.modelGuide.filters.provider")}</span>
              <select
                value={providerFilter}
                onChange={(event) => setProviderFilter(event.target.value)}
                className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700"
              >
                {providerOptions.map((provider) => (
                  <option key={provider} value={provider}>
                    {provider === "all" ? t("settings.modelGuide.filters.allProviders") : t(`settings.modelGuide.provider.${provider}`, provider)}
                  </option>
                ))}
              </select>
            </label>
            <label className="space-y-1">
              <span className="text-xs font-medium text-zinc-600">{t("settings.modelGuide.filters.search")}</span>
              <div className="flex gap-2">
                <input
                  type="search"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t("settings.modelGuide.filters.searchPlaceholder")}
                  className="w-full rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700"
                />
                {query ? (
                  <button
                    type="button"
                    onClick={() => setQuery("")}
                    className="rounded-lg border border-zinc-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700"
                  >
                    {t("common.clear")}
                  </button>
                ) : null}
              </div>
            </label>
          </div>
        </div>
        <div className="overflow-auto px-5 py-4">
          <ModelGuideTable entries={filteredEntries} t={t} />
        </div>
      </div>
      </div>
    </div>
  );
}
