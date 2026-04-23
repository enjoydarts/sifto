"use client";

import { useCallback, useMemo, useState } from "react";
import { Copy, Link2, X } from "lucide-react";
import { api, DeepInfraModelListEntry, DeepInfraModelsResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { EmptyState, ModelCatalogFilters, ModelCatalogPage, SectionHeading } from "@/components/model-catalog/model-catalog-page";
import { formatDateTime, formatNumber, formatUSD, limitSummaryModels, useModelCatalog, useModelSort } from "@/components/model-catalog/use-model-catalog";

type DeepInfraSection = "overview" | "available" | "unavailable";
type SortKey = "provider" | "model" | "context" | "pricing";

function formatPrice(value: number | null | undefined, unavailableLabel: string) {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) return unavailableLabel;
  return `${formatUSD(value)} / 1M tok`;
}

function pricingSummary(model: DeepInfraModelListEntry, unavailableLabel: string) {
  const input = formatPrice(model.input_per_mtok_usd, unavailableLabel);
  const output = formatPrice(model.output_per_mtok_usd, unavailableLabel);
  const cacheRead = typeof model.cache_read_per_mtok_usd === "number" ? formatPrice(model.cache_read_per_mtok_usd, unavailableLabel) : null;
  const parts = [
    input !== unavailableLabel ? `in ${input}` : null,
    output !== unavailableLabel ? `out ${output}` : null,
    cacheRead && cacheRead !== unavailableLabel ? `cache ${cacheRead}` : null,
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(" / ") : unavailableLabel;
}

function pricingScore(model: DeepInfraModelListEntry) {
  return [model.input_per_mtok_usd, model.output_per_mtok_usd, model.cache_read_per_mtok_usd].reduce<number>((sum, value) => {
    return typeof value === "number" && Number.isFinite(value) ? sum + value : sum;
  }, 0);
}

function syncProgressLabel(t: (key: string, fallback?: string) => string, run: DeepInfraModelsResponse["latest_run"] | undefined) {
  if (!run || run.translation_target_count <= 0) return null;
  return t("deepinfraModels.progressLabel")
    .replace("{{completed}}", String(run.translation_completed_count))
    .replace("{{total}}", String(run.translation_target_count));
}

function recentChangeLabel(t: (key: string, fallback?: string) => string, change?: string | null) {
  switch (change) {
    case "added":
      return t("deepinfraModels.recentChange.added");
    case "pricing_changed":
      return t("deepinfraModels.recentChange.pricingChanged");
    case "context_changed":
      return t("deepinfraModels.recentChange.contextChanged");
    case "removed":
      return t("deepinfraModels.recentChange.removed");
    default:
      return null;
  }
}

function recentChangeClassName(change?: string | null) {
  switch (change) {
    case "added":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "pricing_changed":
      return "border-[#ead5af] bg-[#faf1dd] text-[#916321]";
    case "context_changed":
      return "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink)]";
    case "removed":
      return "border-red-200 bg-red-50 text-red-700";
    default:
      return "border-zinc-300 bg-zinc-100 text-zinc-700";
  }
}

function deepInfraProviderDisplay(model: DeepInfraModelListEntry) {
  const modelNamespace = model.model_id.includes("/") ? model.model_id.split("/", 1)[0] : "";
  if (modelNamespace) return modelNamespace;
  return model.provider_slug || "";
}

export default function DeepInfraModelsPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [providerFilter, setProviderFilter] = useState("");
  const [activeSection, setActiveSection] = useState<DeepInfraSection>("overview");
  const [selectedModel, setSelectedModel] = useState<DeepInfraModelListEntry | null>(null);
  const { loading, syncing, error, data, query, setQuery, handleSync } = useModelCatalog<DeepInfraModelsResponse>({
    fetchData: () => api.getDeepInfraModels(),
    syncData: () => api.syncDeepInfraModels(),
    syncSuccessKey: "deepinfraModels.syncCompleted",
  });
  const { sortKey, setSort, sortMarker } = useModelSort<SortKey>("provider", ["context", "pricing"]);

  const availableBase = useMemo(() => data?.models ?? [], [data?.models]);
  const unavailableBase = useMemo(() => data?.unavailable_models ?? [], [data?.unavailable_models]);

  const providerOptions = useMemo(() => {
    const values = new Set<string>();
    for (const model of [...availableBase, ...unavailableBase]) {
      const provider = deepInfraProviderDisplay(model);
      if (provider) values.add(provider);
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [availableBase, unavailableBase]);

  const filterModels = useCallback((models: DeepInfraModelListEntry[]) => {
    const q = query.trim().toLowerCase();
    return models.filter((model) => {
      const provider = deepInfraProviderDisplay(model);
      if (providerFilter && provider !== providerFilter) return false;
      if (!q) return true;
      return [
        model.model_id,
        model.display_name,
        provider,
        model.model_type,
        model.description_en,
        model.description_ja,
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
        .includes(q);
    });
  }, [providerFilter, query]);

  const sortModels = useCallback((models: DeepInfraModelListEntry[]) => {
    const arr = [...models];
    arr.sort((a, b) => {
      let result = 0;
      switch (sortKey) {
        case "provider":
          result = deepInfraProviderDisplay(a).localeCompare(deepInfraProviderDisplay(b));
          break;
        case "model":
          result = (a.display_name || a.model_id).localeCompare(b.display_name || b.model_id);
          break;
        case "context":
          result = (a.context_length ?? -1) - (b.context_length ?? -1);
          break;
        case "pricing":
          result = pricingScore(a) - pricingScore(b);
          break;
      }
      if (result === 0) result = a.model_id.localeCompare(b.model_id);
      return result;
    });
    return arr;
  }, [sortKey]);

  const availableModels = useMemo(() => sortModels(filterModels(availableBase)), [availableBase, filterModels, sortModels]);
  const unavailableModels = useMemo(() => sortModels(filterModels(unavailableBase)), [unavailableBase, filterModels, sortModels]);

  const latestRunLabel = data?.latest_run?.finished_at ? new Date(data.latest_run.finished_at).toLocaleString() : t("deepinfraModels.latestRunEmpty");
  const latestSummary = data?.latest_change_summary ?? null;
  const latestSummaryTriggerLabel = latestSummary?.trigger === "manual" ? t("deepinfraModels.summaryTrigger.manual") : t("deepinfraModels.summaryTrigger.cron");
  const fetchedCount = data?.latest_run?.fetched_count ?? availableBase.length + unavailableBase.length;
  const acceptedCount = data?.latest_run?.accepted_count ?? availableBase.length;
  const translatedCount = data?.latest_run?.translation_completed_count ?? 0;
  const translationTargetCount = data?.latest_run?.translation_target_count ?? 0;
  const removedCount = unavailableBase.length;
  const unavailableLabel = t("deepinfraModels.pricingUnavailable");

  const sections = [
    { key: "overview", label: t("modelCatalog.overview"), meta: `${t("deepinfraModels.latestRun")} · ${latestRunLabel}` },
    { key: "available", label: t("deepinfraModels.table.availableModels"), meta: `${availableModels.length} ${t("common.rows")}` },
    { key: "unavailable", label: t("deepinfraModels.table.unavailableModels"), meta: `${unavailableModels.length} ${t("common.rows")}` },
  ];

  const statusContent = (
    <>
      <div>{t("deepinfraModels.latestRun")} · {latestRunLabel}</div>
      <div>{t("deepinfraModels.fetched")} · {fetchedCount}</div>
      <div>{t("deepinfraModels.accepted")} · {acceptedCount}</div>
      <div>{syncProgressLabel(t, data?.latest_run) ?? t("deepinfraModels.progressPreparing")}</div>
    </>
  );

  const handleCopyModelId = useCallback(async () => {
    if (!selectedModel) return;
    try {
      await navigator.clipboard.writeText(selectedModel.model_id);
      showToast(t("openrouterModels.toast.modelIdCopied"), "success");
    } catch {
      showToast(t("openrouterModels.toast.modelIdCopyFailed"), "error");
    }
  }, [selectedModel, showToast, t]);

  const selectedDescription = selectedModel ? selectedModel.description_ja ?? selectedModel.description_en ?? t("deepinfraModels.descriptionFallback") : "";
  const selectedDescriptionEn = selectedModel?.description_en ?? "";

  const renderStateBadge = (model: DeepInfraModelListEntry) => {
    if (model.availability !== "removed") return null;
    return (
      <span className="inline-flex rounded-full border border-red-200 bg-red-50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.08em] text-red-700">
        {t("deepinfraModels.badge.removed")}
      </span>
    );
  };

  const renderTable = (models: DeepInfraModelListEntry[], emptyLabel: string, showState: boolean) => (
    <section className="surface-editorial rounded-[28px] p-5">
      <SectionHeading
        badge={t("nav.deepinfraModels")}
        title={showState ? t("deepinfraModels.table.unavailableModels") : t("deepinfraModels.table.availableModels")}
        count={models.length}
        countLabel={t("common.rows")}
      />

      {models.length === 0 ? (
        <EmptyState>{emptyLabel}</EmptyState>
      ) : (
        <div className="mt-4 overflow-hidden rounded-[22px] border border-[var(--color-editorial-line)]">
          <div className="overflow-x-auto">
            <table className="min-w-[1200px] divide-y divide-[var(--color-editorial-line)] text-sm">
              <thead className="bg-[var(--color-editorial-panel)]">
                <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                  <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("provider")}>{t("deepinfraModels.table.provider")}{sortMarker("provider")}</button></th>
                  <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("model")}>{t("deepinfraModels.table.model")}{sortMarker("model")}</button></th>
                  <th className="px-4 py-3">{t("deepinfraModels.table.type")}</th>
                  <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("context")}>{t("deepinfraModels.table.context")}{sortMarker("context")}</button></th>
                  <th className="px-4 py-3">{t("deepinfraModels.table.maxTokens")}</th>
                  <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("pricing")}>{t("deepinfraModels.table.pricing")}{sortMarker("pricing")}</button></th>
                  {showState ? <th className="px-4 py-3">{t("deepinfraModels.table.state")}</th> : null}
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                {models.map((model) => {
                  const changeLabel = recentChangeLabel(t, model.recent_change);
                  return (
                    <tr key={model.model_id} className="cursor-pointer transition hover:bg-[var(--color-editorial-panel)]" onClick={() => setSelectedModel(model)}>
                      <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{deepInfraProviderDisplay(model) || "—"}</td>
                      <td className="px-4 py-3 align-top">
                        <div className="flex items-center gap-2">
                          <div className="font-medium text-[var(--color-editorial-ink)]">{model.display_name || model.model_id}</div>
                          {renderStateBadge(model)}
                          {!showState && changeLabel ? (
                            <span className={`inline-flex rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.08em] ${recentChangeClassName(model.recent_change)}`}>
                              {changeLabel}
                            </span>
                          ) : null}
                        </div>
                        <div className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">{model.model_id}</div>
                        <div className="mt-2 text-xs text-[var(--color-editorial-ink-faint)]">{formatDateTime(model.fetched_at)}</div>
                      </td>
                      <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.model_type || "—"}</td>
                      <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{typeof model.context_length === "number" ? formatNumber(model.context_length) : "—"}</td>
                      <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{typeof model.max_tokens === "number" ? formatNumber(model.max_tokens) : "—"}</td>
                      <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{pricingSummary(model, unavailableLabel)}</td>
                      {showState ? <td className="whitespace-nowrap px-4 py-3 align-top">{renderStateBadge(model)}</td> : null}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </section>
  );

  return (
    <ModelCatalogPage
      title={t("deepinfraModels.title")}
      titleIcon={Link2}
      description={t("deepinfraModels.subtitle")}
      syncing={syncing}
      onSync={handleSync}
      syncLabel={t("deepinfraModels.sync")}
      syncingLabel={t("deepinfraModels.syncing")}
      sections={sections}
      activeSection={activeSection}
      onSectionChange={(key) => setActiveSection(key as DeepInfraSection)}
      statusContent={statusContent}
      loading={loading}
      error={error}
    >
      {activeSection === "overview" ? (
        <>
          <section className="surface-editorial rounded-[28px] p-5">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("modelCatalog.overview")}</div>
                <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("deepinfraModels.overview.heading")}</h2>
                <p className="mt-3 max-w-3xl text-[14px] leading-7 text-[var(--color-editorial-ink-soft)]">{t("deepinfraModels.overview.description")}</p>
              </div>
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                {data?.latest_run?.status === "running" && data.latest_run.trigger_type === "manual" ? t("deepinfraModels.progressRunning") : latestRunLabel}
              </div>
            </div>
            <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.fetched")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{fetchedCount}</div>
              </div>
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.accepted")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{acceptedCount}</div>
              </div>
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.table.unavailableModels")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{removedCount}</div>
              </div>
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("modelCatalog.translated")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{translatedCount} / {translationTargetCount}</div>
              </div>
            </div>
            {data?.latest_run?.status === "running" && data.latest_run.trigger_type === "manual" ? (
              <div className="mt-4 rounded-[22px] border border-[#ead5af] bg-[#faf1dd] px-4 py-3 text-sm text-[#916321]">
                <div className="font-medium">{t("deepinfraModels.progressRunning")}</div>
                <div className="mt-1">{syncProgressLabel(t, data.latest_run) ?? t("deepinfraModels.progressPreparing")}</div>
              </div>
            ) : null}
          </section>

          <section className="grid gap-4 xl:grid-cols-[1.25fr_1fr]">
            <section className="surface-editorial rounded-[28px] p-5">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.latestSummary.title")}</div>
                  <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{latestSummaryTriggerLabel}</h3>
                </div>
                <div className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1 text-xs text-[var(--color-editorial-ink-soft)]">
                  {latestRunLabel}
                </div>
              </div>
              <div className="mt-4 grid gap-3 md:grid-cols-2">
                {[
                  { key: "added", label: t("deepinfraModels.recentChange.added"), models: latestSummary?.added ?? [], className: "border-emerald-200 bg-emerald-50 text-emerald-700" },
                  { key: "removed", label: t("deepinfraModels.recentChange.removed"), models: latestSummary?.removed ?? [], className: "border-red-200 bg-red-50 text-red-700" },
                  { key: "pricing_changed", label: t("deepinfraModels.recentChange.pricingChanged"), models: latestSummary?.pricing_changed ?? [], className: "border-[#ead5af] bg-[#faf1dd] text-[#916321]" },
                  { key: "context_changed", label: t("deepinfraModels.recentChange.contextChanged"), models: latestSummary?.context_changed ?? [], className: "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink)]" },
                ].map((group) => {
                  const summary = limitSummaryModels(group.models ?? []);
                  return (
                    <div key={group.key} className={`rounded-[22px] border px-4 py-3 ${group.className}`}>
                      <div className="text-sm font-semibold">{group.label}{group.models.length > 0 ? ` (${group.models.length})` : ""}</div>
                      {group.models.length > 0 ? (
                        <div className="mt-3 space-y-2 text-xs">
                          {summary.items.map((modelID) => (
                            <div key={modelID} className="rounded-[14px] border border-current/15 bg-white/70 px-3 py-2">{modelID}</div>
                          ))}
                          {summary.remaining > 0 ? <div className="rounded-[14px] border border-current/15 bg-white/70 px-3 py-2">+{summary.remaining}</div> : null}
                        </div>
                      ) : <div className="mt-3 text-xs opacity-80">{t("deepinfraModels.latestSummary.none")}</div>}
                    </div>
                  );
                })}
              </div>
            </section>

            <section className="surface-editorial rounded-[28px] p-5">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("modelCatalog.translation")}</div>
              <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("deepinfraModels.translation.heading")}</h3>
              <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.progressGlobal")}</div>
                <div className="mt-3 text-lg leading-none text-[var(--color-editorial-ink)]">{translatedCount} / {translationTargetCount}</div>
                <div className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{syncProgressLabel(t, data?.latest_run) ?? t("deepinfraModels.progressPreparing")}</div>
              </div>
              <div className="mt-3 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("deepinfraModels.latestRun")} · {latestRunLabel}
              </div>
            </section>
          </section>
        </>
      ) : null}

      {activeSection === "available" || activeSection === "unavailable" ? (
        <>
          <ModelCatalogFilters
            query={query}
            onQueryChange={setQuery}
            searchLabel={t("deepinfraModels.search")}
            searchPlaceholder={t("deepinfraModels.searchPlaceholder")}
            clearLabel={t("common.clear")}
            providerFilter={providerFilter}
            onProviderFilterChange={setProviderFilter}
            providerFilterLabel={t("deepinfraModels.providerFilter")}
            providerAllLabel={t("deepinfraModels.providerAll")}
            providerOptions={providerOptions}
          />
          {activeSection === "available"
            ? renderTable(
                availableModels,
                availableBase.length === 0 ? t("deepinfraModels.empty") : t("deepinfraModels.noAvailableModels"),
                false,
              )
            : renderTable(unavailableModels, t("deepinfraModels.noUnavailableModels"), true)}
        </>
      ) : null}

      {selectedModel ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(31,26,23,0.45)] px-4 py-6" onClick={() => setSelectedModel(null)}>
          <div className="flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] shadow-[var(--shadow-card)]" onClick={(event) => event.stopPropagation()}>
            <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.72),rgba(255,253,249,0.96))] px-5 py-4">
              <div className="min-w-0">
                <div className="text-xs font-medium uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">{deepInfraProviderDisplay(selectedModel) || t("common.unknown")}</div>
                <h2 className="mt-2 break-words font-serif text-[1.55rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{selectedModel.display_name || selectedModel.model_id}</h2>
                <div className="mt-1 flex items-center gap-2">
                  <p className="min-w-0 break-all text-xs text-[var(--color-editorial-ink-faint)]">{selectedModel.model_id}</p>
                  <button type="button" onClick={handleCopyModelId} className="inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-faint)] hover:border-[var(--color-editorial-line-strong)] hover:text-[var(--color-editorial-ink)]" aria-label={t("deepinfraModels.modal.copyModelId")} title={t("deepinfraModels.modal.copyModelId")}>
                    <Copy className="size-3.5" aria-hidden="true" />
                  </button>
                </div>
              </div>
              <button type="button" onClick={() => setSelectedModel(null)} className="inline-flex size-9 items-center justify-center rounded-lg border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:border-[var(--color-editorial-line-strong)] hover:text-[var(--color-editorial-ink)]" aria-label={t("common.close")}>
                <X className="size-4" aria-hidden="true" />
              </button>
            </div>
            <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-5">
              <section className="grid gap-3 md:grid-cols-3">
                <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.table.context")}</div>
                  <div className="mt-2 text-sm text-[var(--color-editorial-ink)]">{typeof selectedModel.context_length === "number" ? formatNumber(selectedModel.context_length) : "—"}</div>
                </div>
                <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.table.maxTokens")}</div>
                  <div className="mt-2 text-sm text-[var(--color-editorial-ink)]">{typeof selectedModel.max_tokens === "number" ? formatNumber(selectedModel.max_tokens) : "—"}</div>
                </div>
                <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.table.pricing")}</div>
                  <div className="mt-2 text-sm text-[var(--color-editorial-ink)]">{pricingSummary(selectedModel, unavailableLabel)}</div>
                </div>
              </section>
              <section className="rounded-[24px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.modal.descriptionJa")}</div>
                <p className="mt-3 whitespace-pre-wrap text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedDescription}</p>
              </section>
              {selectedDescriptionEn && selectedDescriptionEn !== selectedDescription ? (
                <section className="rounded-[24px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-5">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("deepinfraModels.modal.descriptionEn")}</div>
                  <p className="mt-3 whitespace-pre-wrap text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedDescriptionEn}</p>
                </section>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </ModelCatalogPage>
  );
}
