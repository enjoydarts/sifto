"use client";

import { useCallback, useMemo, useState } from "react";
import { Link2 } from "lucide-react";
import { api, FeatherlessModelListEntry, FeatherlessModelsResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useModelCatalog, useModelSort } from "@/components/model-catalog/use-model-catalog";
import { EmptyState, ModelCatalogFilters, ModelCatalogPage, SectionHeading } from "@/components/model-catalog/model-catalog-page";
import { getFeatherlessModelState } from "@/components/settings/providers/featherless-model-state";

type FeatherlessSection = "available" | "unavailable";
type SortKey = "provider" | "model" | "class" | "context";

function badgeClassName(state: ReturnType<typeof getFeatherlessModelState>["kind"]) {
  switch (state) {
    case "gated":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "unavailable":
      return "border-zinc-300 bg-zinc-100 text-zinc-700";
    case "removed":
      return "border-red-200 bg-red-50 text-red-700";
    default:
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
  }
}

export default function FeatherlessModelsPage() {
  const { t } = useI18n();
  const [providerFilter, setProviderFilter] = useState("");
  const [activeSection, setActiveSection] = useState<FeatherlessSection>("available");
  const { loading, syncing, error, data, query, setQuery, handleSync } = useModelCatalog<FeatherlessModelsResponse>({
    fetchData: () => api.getFeatherlessModels(),
    syncData: () => api.syncFeatherlessModels(),
    syncSuccessKey: "featherlessModels.syncCompleted",
  });
  const { sortKey, setSort, sortMarker } = useModelSort<SortKey>("provider", ["class", "context"]);

  const availableBase = useMemo(() => data?.models ?? [], [data?.models]);
  const unavailableBase = useMemo(() => data?.unavailable_models ?? [], [data?.unavailable_models]);
  const providerOptions = useMemo(() => {
    const values = new Set<string>();
    for (const model of [...availableBase, ...unavailableBase]) {
      if (model.provider_slug) values.add(model.provider_slug);
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [availableBase, unavailableBase]);

  const filterModels = useCallback((models: FeatherlessModelListEntry[]) => {
    const q = query.trim().toLowerCase();
    return models.filter((model) => {
      if (providerFilter && model.provider_slug !== providerFilter) return false;
      if (!q) return true;
      return [model.model_id, model.display_name, model.provider_slug, model.model_class]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
        .includes(q);
    });
  }, [providerFilter, query]);

  const sortModels = useCallback((models: FeatherlessModelListEntry[]) => {
    const arr = [...models];
    arr.sort((a, b) => {
      let result = 0;
      switch (sortKey) {
        case "provider":
          result = (a.provider_slug || "").localeCompare(b.provider_slug || "");
          break;
        case "model":
          result = (a.display_name || a.model_id).localeCompare(b.display_name || b.model_id);
          break;
        case "class":
          result = (a.model_class || "").localeCompare(b.model_class || "");
          break;
        case "context":
          result = (a.context_length ?? -1) - (b.context_length ?? -1);
          break;
      }
      if (result === 0) result = a.model_id.localeCompare(b.model_id);
      return result;
    });
    return arr;
  }, [sortKey]);

  const availableModels = useMemo(() => sortModels(filterModels(availableBase)), [availableBase, filterModels, sortModels]);
  const unavailableModels = useMemo(() => sortModels(filterModels(unavailableBase)), [unavailableBase, filterModels, sortModels]);
  const latestRunLabel = data?.latest_run?.finished_at ? new Date(data.latest_run.finished_at).toLocaleString() : t("featherlessModels.latestRunEmpty");
  const fetchedCount = data?.latest_run?.fetched_count ?? availableBase.length + unavailableBase.length;
  const acceptedCount = data?.latest_run?.accepted_count ?? availableBase.length;

  const sections = [
    { key: "available", label: t("featherlessModels.table.availableModels"), meta: `${availableModels.length} ${t("common.rows")}` },
    { key: "unavailable", label: t("featherlessModels.table.unavailableModels"), meta: `${unavailableModels.length} ${t("common.rows")}` },
  ];

  const statusContent = (
    <>
      <div>{t("featherlessModels.latestRun")} · {latestRunLabel}</div>
      <div>{t("featherlessModels.fetched")} · {fetchedCount}</div>
      <div>{t("featherlessModels.accepted")} · {acceptedCount}</div>
      <div>{data?.latest_run?.status === "running" ? t("featherlessModels.progressRunning") : t("featherlessModels.progressPreparing")}</div>
    </>
  );

  const renderStateBadge = (model: FeatherlessModelListEntry) => {
    const state = getFeatherlessModelState(model);
    const label =
      state.kind === "gated"
        ? t("featherlessModels.badge.gated")
        : state.kind === "unavailable"
          ? t("featherlessModels.badge.unavailable")
          : state.kind === "removed"
            ? t("featherlessModels.badge.removed")
            : null;
    if (!label) return null;
    return (
      <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.08em] ${badgeClassName(state.kind)}`}>
        {label}
      </span>
    );
  };

  return (
    <ModelCatalogPage
      title={t("nav.featherlessModels")}
      titleIcon={Link2}
      description={t("featherlessModels.subtitle")}
      syncing={syncing}
      onSync={handleSync}
      syncLabel={t("featherlessModels.sync")}
      syncingLabel={t("featherlessModels.syncing")}
      sections={sections}
      activeSection={activeSection}
      onSectionChange={(key) => setActiveSection(key as FeatherlessSection)}
      statusContent={statusContent}
      loading={loading}
      error={error}
    >
      <ModelCatalogFilters
        query={query}
        onQueryChange={setQuery}
        searchLabel={t("featherlessModels.search")}
        searchPlaceholder={t("featherlessModels.search")}
        clearLabel={t("common.clear")}
        providerFilter={providerFilter}
        onProviderFilterChange={setProviderFilter}
        providerFilterLabel={t("featherlessModels.providerFilter")}
        providerAllLabel={t("featherlessModels.providerAll")}
        providerOptions={providerOptions}
      />

      <section className="surface-editorial rounded-[28px] p-5">
        <SectionHeading
          badge={t("featherlessModels.providerGroup")}
          title={activeSection === "available" ? t("featherlessModels.table.availableModels") : t("featherlessModels.table.unavailableModels")}
          count={activeSection === "available" ? availableModels.length : unavailableModels.length}
          countLabel={t("common.rows")}
        />

        {(activeSection === "available" ? availableModels : unavailableModels).length === 0 ? (
          <EmptyState>
            {activeSection === "available" ? t("featherlessModels.noAvailableModels") : t("featherlessModels.noUnavailableModels")}
          </EmptyState>
        ) : (
          <div className="mt-4 overflow-hidden rounded-[22px] border border-[var(--color-editorial-line)]">
            <div className="overflow-x-auto">
              <table className="min-w-[1080px] divide-y divide-[var(--color-editorial-line)] text-sm">
                <thead className="bg-[var(--color-editorial-panel)]">
                  <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                    <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("provider")}>{t("featherlessModels.table.provider")}{sortMarker("provider")}</button></th>
                    <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("model")}>{t("featherlessModels.table.model")}{sortMarker("model")}</button></th>
                    <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("class")}>{t("featherlessModels.table.class")}{sortMarker("class")}</button></th>
                    <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("context")}>{t("featherlessModels.table.context")}{sortMarker("context")}</button></th>
                    {activeSection === "unavailable" ? <th className="px-4 py-3">{t("featherlessModels.table.state")}</th> : null}
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-editorial-line)] bg-white">
                  {(activeSection === "available" ? availableModels : unavailableModels).map((model) => (
                    <tr key={model.model_id} className="align-top">
                      <td className="whitespace-nowrap px-4 py-3 text-zinc-700">{model.provider_slug || "—"}</td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <div className="font-medium text-zinc-900">{model.display_name || model.model_id}</div>
                          {activeSection === "available" ? renderStateBadge(model) : null}
                        </div>
                        <div className="mt-1 text-xs text-zinc-500">{model.model_id}</div>
                        {model.max_completion_tokens ? (
                          <div className="mt-2 text-xs leading-6 text-zinc-600">
                            {t("featherlessModels.maxCompletion")} {model.max_completion_tokens.toLocaleString()}
                          </div>
                        ) : null}
                      </td>
                      <td className="whitespace-nowrap px-4 py-3 text-zinc-600">{model.model_class || "—"}</td>
                      <td className="whitespace-nowrap px-4 py-3 text-zinc-600">{model.context_length?.toLocaleString() ?? "—"}</td>
                      {activeSection === "unavailable" ? <td className="whitespace-nowrap px-4 py-3">{renderStateBadge(model)}</td> : null}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </section>
    </ModelCatalogPage>
  );
}
