"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Copy, Link2, RefreshCw, X } from "lucide-react";
import { api, PoeModelSnapshot, PoeModelsResponse, PoeUsageResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useModelCatalog, useModelSort, parseObject, formatPrice, limitSummaryModels, formatDateTime, formatUSD, formatMetricNumber } from "@/components/model-catalog/use-model-catalog";
import { ModelCatalogPage, ModelCatalogFilters, SectionHeading, EmptyState } from "@/components/model-catalog/model-catalog-page";

type PoeSection = "overview" | "available" | "removed" | "usage";
type SortKey = "provider" | "model" | "context" | "pricing" | "transport";

function pricingSummary(model: PoeModelSnapshot) {
  const pricing = parseObject(model.pricing_json);
  const input = formatPrice(pricing.prompt);
  const output = formatPrice(pricing.completion);
  const cacheRead = formatPrice(pricing.cache_read);
  const parts = [input ? `in ${input}` : null, output ? `out ${output}` : null, cacheRead ? `cache ${cacheRead}` : null].filter(Boolean);
  return parts.length > 0 ? `${parts.join(" / ")} / 1M tok` : null;
}

function syncProgressLabel(
  t: (key: string, fallback?: string) => string,
  run: PoeModelsResponse["latest_run"] | undefined | null,
) {
  if (!run || run.translation_target_count <= 0) return null;
  return t("poeModels.progressLabel").replace("{{completed}}", String(run.translation_completed_count)).replace("{{total}}", String(run.translation_target_count));
}

function recentChangeClassName(change: "added" | "removed") {
  return change === "added" ? "bg-emerald-50 text-emerald-700 border-emerald-200" : "bg-red-50 text-red-700 border-red-200";
}

function transportLabel(model: PoeModelSnapshot) {
  return model.preferred_transport === "anthropic" ? "poeModels.transport.anthropicCompat" : "poeModels.transport.openaiCompat";
}

function pricingScore(model: PoeModelSnapshot) {
  const pricing = parseObject(model.pricing_json);
  const prompt = typeof pricing.prompt === "number" ? pricing.prompt : typeof pricing.prompt === "string" ? Number(pricing.prompt) : NaN;
  const completion = typeof pricing.completion === "number" ? pricing.completion : typeof pricing.completion === "string" ? Number(pricing.completion) : NaN;
  const cacheRead = typeof pricing.cache_read === "number" ? pricing.cache_read : typeof pricing.cache_read === "string" ? Number(pricing.cache_read) : NaN;
  return [prompt, completion, cacheRead].reduce((sum, value) => (Number.isFinite(value) ? sum + value : sum), 0);
}

export default function PoeModelsPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [providerFilter, setProviderFilter] = useState("");
  const [selectedModel, setSelectedModel] = useState<PoeModelSnapshot | null>(null);
  const [activeSection, setActiveSection] = useState<PoeSection>("overview");
  const [usageData, setUsageData] = useState<PoeUsageResponse | null>(null);
  const [usageLoading, setUsageLoading] = useState(false);
  const [usageSyncing, setUsageSyncing] = useState(false);
  const [usageError, setUsageError] = useState<string | null>(null);
  const [usageRange, setUsageRange] = useState("30d");
  const [usageEntryLimit, setUsageEntryLimit] = useState(100);

  const { loading, syncing, error, data, query, setQuery, handleSync } = useModelCatalog<PoeModelsResponse>({
    fetchData: () => api.getPoeModels(),
    syncData: () => api.syncPoeModels(),
    syncSuccessKey: "poeModels.syncCompleted",
    isSyncRunning: (d) => {
      const run = d?.latest_run;
      return !!run && run.status === "running" && run.trigger_type === "manual";
    },
  });

  const { sortKey, setSort, sortMarker } = useModelSort<SortKey>("provider", ["context", "pricing"]);

  const models = useMemo(() => (Array.isArray(data?.models) ? data?.models : []), [data?.models]);
  const removedModels = useMemo(() => (Array.isArray(data?.removed_models) ? data?.removed_models : []), [data?.removed_models]);

  const loadUsage = useCallback(async () => {
    setUsageLoading(true);
    try {
      const next = await api.getPoeUsage(usageRange, usageEntryLimit);
      setUsageData(next);
      setUsageError(null);
    } catch (e) {
      setUsageError(String(e));
    } finally {
      setUsageLoading(false);
    }
  }, [usageEntryLimit, usageRange]);

  useEffect(() => {
    if (activeSection !== "usage") return;
    loadUsage();
  }, [activeSection, loadUsage]);

  const handleUsageSync = useCallback(async () => {
    setUsageSyncing(true);
    try {
      await api.syncPoeUsage();
      await loadUsage();
      showToast(t("poeModels.usage.syncCompleted"), "success");
    } catch (e) {
      setUsageError(String(e));
      showToast(String(e), "error");
    } finally {
      setUsageSyncing(false);
    }
  }, [loadUsage, showToast, t]);

  const handleCopyModelId = useCallback(async () => {
    if (!selectedModel) return;
    try {
      await navigator.clipboard.writeText(selectedModel.model_id);
      showToast(t("poeModels.toast.modelIdCopied"), "success");
    } catch {
      showToast(t("poeModels.toast.modelIdCopyFailed"), "error");
    }
  }, [selectedModel, showToast, t]);

  const providerOptions = useMemo(() => {
    const values = new Set<string>();
    for (const model of models) {
      if (model.owned_by) values.add(model.owned_by);
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [models]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return models.filter((model) => {
      if (providerFilter && model.owned_by !== providerFilter) return false;
      if (!q) return true;
      return [model.model_id, model.display_name, model.owned_by, model.description_en, model.description_ja].filter(Boolean).join(" ").toLowerCase().includes(q);
    });
  }, [models, providerFilter, query]);

  const filteredRemoved = useMemo(() => {
    const q = query.trim().toLowerCase();
    return removedModels.filter((model) => {
      if (providerFilter && model.owned_by !== providerFilter) return false;
      if (!q) return true;
      return [model.model_id, model.display_name, model.owned_by, model.description_en, model.description_ja].filter(Boolean).join(" ").toLowerCase().includes(q);
    });
  }, [removedModels, providerFilter, query]);

  const sorted = useMemo(() => {
    const arr = [...filtered];
    arr.sort((a, b) => {
      let result = 0;
      switch (sortKey) {
        case "provider": result = (a.owned_by || "").localeCompare(b.owned_by || ""); break;
        case "model": result = (a.display_name || a.model_id).localeCompare(b.display_name || b.model_id); break;
        case "context": result = (a.context_length ?? -1) - (b.context_length ?? -1); break;
        case "pricing": result = pricingScore(a) - pricingScore(b); break;
        case "transport": result = t(transportLabel(a)).localeCompare(t(transportLabel(b))); break;
      }
      if (result === 0) result = a.model_id.localeCompare(b.model_id);
      return result;
    });
    return arr;
  }, [filtered, sortKey]);

  const sortedRemoved = useMemo(() => {
    const arr = [...filteredRemoved];
    arr.sort((a, b) => {
      let result = 0;
      switch (sortKey) {
        case "provider": result = (a.owned_by || "").localeCompare(b.owned_by || ""); break;
        case "model": result = (a.display_name || a.model_id).localeCompare(b.display_name || b.model_id); break;
        case "context": result = (a.context_length ?? -1) - (b.context_length ?? -1); break;
        case "pricing": result = pricingScore(a) - pricingScore(b); break;
        case "transport": result = t(transportLabel(a)).localeCompare(t(transportLabel(b))); break;
      }
      if (result === 0) result = a.model_id.localeCompare(b.model_id);
      return result;
    });
    return arr;
  }, [filteredRemoved, sortKey]);

  const latestSummary = data?.latest_change_summary ?? null;
  const latestRunLabel = data?.latest_run?.finished_at ? new Date(data.latest_run.finished_at).toLocaleString() : t("poeModels.latestRunEmpty");
  const fetchedCount = data?.latest_run?.fetched_count ?? models.length;
  const acceptedCount = data?.latest_run?.accepted_count ?? models.length;
  const translatedCount = data?.latest_run?.translation_completed_count ?? 0;
  const translationTargetCount = data?.latest_run?.translation_target_count ?? 0;
  const removedCount = latestSummary?.removed?.length ?? 0;
  const addedCount = latestSummary?.added?.length ?? 0;
  const selectedDescription = selectedModel ? selectedModel.description_ja ?? selectedModel.description_en ?? t("poeModels.descriptionFallback") : "";
  const selectedDescriptionEn = selectedModel?.description_en ?? "";
  const usageEntryCount = usageData?.summary.entry_count ?? 0;
  const usageConfigured = usageData?.configured ?? false;
  const averagePointsPerCall = usageData?.summary.average_cost_points ?? 0;
  const averageUsdPerCall = usageData?.summary.average_cost_usd ?? 0;

  const sections = [
    { key: "overview", label: t("modelCatalog.overview"), meta: `${t("poeModels.latestRun")} · ${latestRunLabel}` },
    { key: "available", label: t("poeModels.table.availableModels"), meta: `${sorted.length} ${t("common.rows")}` },
    { key: "removed", label: t("poeModels.table.removedModels"), meta: `${removedCount} ${t("common.rows")}` },
    { key: "usage", label: t("poeModels.section.usage"), meta: usageConfigured ? `${usageEntryCount} ${t("common.rows")}` : t("poeModels.usage.meta") },
  ];

  const statusContent = (
    <>
      <div>{t("poeModels.latestRun")} · {latestRunLabel}</div>
      <div>{t("poeModels.fetched")} · {fetchedCount}</div>
      <div>{t("poeModels.accepted")} · {acceptedCount}</div>
      <div>{syncProgressLabel(t, data?.latest_run) ?? t("poeModels.progressPreparing")}</div>
    </>
  );

  return (
    <ModelCatalogPage
      title={t("nav.poeModels")}
      titleIcon={Link2}
      description={t("poeModels.subtitle")}
      syncing={syncing}
      onSync={handleSync}
      syncLabel={t("poeModels.sync")}
      syncingLabel={t("poeModels.syncing")}
      sections={sections}
      activeSection={activeSection}
      onSectionChange={(k) => setActiveSection(k as PoeSection)}
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
                <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("poeModels.overview.heading")}</h2>
                <p className="mt-3 max-w-3xl text-[14px] leading-7 text-[var(--color-editorial-ink-soft)]">{t("poeModels.overview.description")}</p>
              </div>
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                {data?.latest_run?.status === "running" && data.latest_run.trigger_type === "manual" ? t("poeModels.progressRunning") : latestRunLabel}
              </div>
            </div>

            <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.fetched")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{fetchedCount}</div>
              </div>
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.accepted")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{acceptedCount}</div>
              </div>
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.recentChange.added")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{addedCount}</div>
              </div>
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("modelCatalog.translated")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{translatedCount} / {translationTargetCount}</div>
              </div>
            </div>

            {data?.latest_run?.status === "running" && data.latest_run.trigger_type === "manual" ? (
              <div className="mt-4 rounded-[22px] border border-[#ead5af] bg-[#faf1dd] px-4 py-3 text-sm text-[#916321]">
                <div className="font-medium">{t("poeModels.progressRunning")}</div>
                <div className="mt-1">{syncProgressLabel(t, data.latest_run) ?? t("poeModels.progressPreparing")}</div>
              </div>
            ) : null}
          </section>

          <section className="grid gap-4 xl:grid-cols-[1.25fr_1fr]">
            <section className="surface-editorial rounded-[28px] p-5">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.latestSummary.title")}</div>
              <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("poeModels.latestSummary.title")}</h3>
              <div className="mt-4 space-y-3">
                {(["added", "removed"] as const).map((key) => {
                  const groupModels = key === "added" ? (latestSummary?.added ?? []) : (latestSummary?.removed ?? []);
                  const summary = limitSummaryModels(groupModels);
                  return (
                    <div key={key} className={`rounded-[22px] border px-4 py-3 ${recentChangeClassName(key)}`}>
                      <div className="text-sm font-semibold">{t(`poeModels.recentChange.${key}`)} {groupModels.length > 0 ? `(${groupModels.length})` : ""}</div>
                      {groupModels.length > 0 ? (
                        <div className="mt-3 space-y-2 text-xs">
                          {summary.items.map((modelID) => (<div key={modelID} className="rounded-[14px] border border-current/15 bg-white/70 px-3 py-2">{modelID}</div>))}
                          {summary.remaining > 0 ? (<div className="rounded-[14px] border border-current/15 bg-white/70 px-3 py-2">+{summary.remaining}</div>) : null}
                        </div>
                      ) : (<div className="mt-3 text-xs opacity-70">{t("poeModels.latestSummary.none")}</div>)}
                    </div>
                  );
                })}
              </div>
            </section>

            <section className="surface-editorial rounded-[28px] p-5">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("modelCatalog.translation")}</div>
              <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("poeModels.translation.heading")}</h3>
              <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.progressGlobal")}</div>
                <div className="mt-3 text-lg leading-none text-[var(--color-editorial-ink)]">{translatedCount} / {translationTargetCount}</div>
                <div className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{syncProgressLabel(t, data?.latest_run) ?? t("poeModels.progressPreparing")}</div>
              </div>
              <div className="mt-3 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("poeModels.latestRun")} · {latestRunLabel}<br />{t("poeModels.recentChange.removed")} · {removedCount}
              </div>
            </section>
          </section>
        </>
      ) : null}

      {activeSection === "available" ? (
        <>
          <ModelCatalogFilters
            query={query}
            onQueryChange={setQuery}
            searchLabel={t("poeModels.search")}
            searchPlaceholder={t("poeModels.search")}
            clearLabel={t("common.clear")}
            providerFilter={providerFilter}
            onProviderFilterChange={setProviderFilter}
            providerFilterLabel={t("poeModels.providerFilter")}
            providerAllLabel={t("poeModels.providerAll")}
            providerOptions={providerOptions}
          />
          <section className="surface-editorial rounded-[28px] p-5">
            <SectionHeading badge={t("poeModels.providerGroup")} title={t("poeModels.table.availableModels")} count={sorted.length} countLabel={t("common.rows")} />
            {sorted.length === 0 ? (
              <EmptyState>{t("poeModels.noAvailableModels")}</EmptyState>
            ) : (
              <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)]">
                <div className="overflow-x-auto rounded-[22px]">
                  <table className="min-w-[1080px] divide-y divide-[var(--color-editorial-line)] text-sm">
                    <thead className="bg-[var(--color-editorial-panel)]">
                      <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("provider")}>{t("poeModels.table.provider")}{sortMarker("provider")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("model")}>{t("poeModels.table.model")}{sortMarker("model")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("transport")}>{t("poeModels.table.transport")}{sortMarker("transport")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("context")}>{t("poeModels.table.context")}{sortMarker("context")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("pricing")}>{t("poeModels.table.pricing")}{sortMarker("pricing")}</button></th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                      {sorted.map((model) => (
                        <tr key={model.model_id} className="cursor-pointer transition hover:bg-[var(--color-editorial-panel)]" onClick={() => setSelectedModel(model)}>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.owned_by || "—"}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top">
                            <div className="whitespace-nowrap font-medium text-[var(--color-editorial-ink)]">{model.display_name || model.model_id}</div>
                            <div className="mt-1 whitespace-nowrap text-xs text-[var(--color-editorial-ink-faint)]">{model.model_id}</div>
                          </td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{t(transportLabel(model))}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.context_length ? model.context_length.toLocaleString() : "—"}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{pricingSummary(model) ?? "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </section>
        </>
      ) : null}

      {activeSection === "removed" ? (
        <>
          <ModelCatalogFilters
            query={query}
            onQueryChange={setQuery}
            searchLabel={t("poeModels.search")}
            searchPlaceholder={t("poeModels.search")}
            clearLabel={t("common.clear")}
            providerFilter={providerFilter}
            onProviderFilterChange={setProviderFilter}
            providerFilterLabel={t("poeModels.providerFilter")}
            providerAllLabel={t("poeModels.providerAll")}
            providerOptions={providerOptions}
          />
          <section className="surface-editorial rounded-[28px] p-5">
            <SectionHeading badge={t("poeModels.providerGroup")} title={t("poeModels.table.removedModels")} count={sortedRemoved.length} countLabel={t("common.rows")} />
            {sortedRemoved.length === 0 ? (
              <EmptyState>{t("poeModels.noUnavailableModels")}</EmptyState>
            ) : (
              <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)]">
                <div className="overflow-x-auto rounded-[22px]">
                  <table className="min-w-[1080px] divide-y divide-[var(--color-editorial-line)] text-sm">
                    <thead className="bg-[var(--color-editorial-panel)]">
                      <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("provider")}>{t("poeModels.table.provider")}{sortMarker("provider")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("model")}>{t("poeModels.table.model")}{sortMarker("model")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("transport")}>{t("poeModels.table.transport")}{sortMarker("transport")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("context")}>{t("poeModels.table.context")}{sortMarker("context")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("pricing")}>{t("poeModels.table.pricing")}{sortMarker("pricing")}</button></th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                      {sortedRemoved.map((model) => (
                        <tr key={model.model_id} className="cursor-pointer bg-red-50/35 transition hover:bg-red-50/60" onClick={() => setSelectedModel(model)}>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.owned_by || "—"}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top">
                            <div className="whitespace-nowrap font-medium text-[var(--color-editorial-ink)]">{model.display_name || model.model_id}</div>
                            <div className="mt-1 whitespace-nowrap text-xs text-[var(--color-editorial-ink-faint)]">{model.model_id}</div>
                          </td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{t(transportLabel(model))}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.context_length ? model.context_length.toLocaleString() : "—"}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{pricingSummary(model) ?? "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </section>
        </>
      ) : null}

      {activeSection === "usage" ? (
        <section className="space-y-4">
          <section className="surface-editorial rounded-[28px] p-5">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.section.usage")}</div>
                <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("poeModels.usage.title")}</h2>
                <p className="mt-3 max-w-3xl text-[14px] leading-7 text-[var(--color-editorial-ink-soft)]">{t("poeModels.usage.description")}</p>
              </div>
              <div className="flex flex-wrap items-center justify-end gap-2">
                <button type="button" onClick={handleUsageSync} disabled={usageSyncing} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:opacity-60">
                  <RefreshCw className={`size-4 ${usageSyncing ? "animate-spin" : ""}`} aria-hidden="true" />
                  {usageSyncing ? t("poeModels.usage.syncing") : t("poeModels.usage.sync")}
                </button>
              </div>
            </div>
            <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-[var(--color-editorial-ink-soft)]">
              <div className="flex flex-wrap items-center gap-2">
                <span>{t("poeModels.usage.range")}</span>
                <select value={usageRange} onChange={(e) => setUsageRange(e.target.value)} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink)] outline-none">
                  {(usageData?.available_ranges ?? [{ key: "today" }, { key: "yesterday" }, { key: "7d" }, { key: "14d" }, { key: "30d" }, { key: "mtd" }, { key: "prev_month" }]).map((option) => (
                    <option key={option.key} value={option.key}>{t(`poeModels.usage.range.${option.key}`)}</option>
                  ))}
                </select>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <span>{t("poeModels.usage.entryLimit")}</span>
                <select value={usageEntryLimit} onChange={(e) => setUsageEntryLimit(Number(e.target.value))} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink)] outline-none">
                  {[50, 100, 200].map((value) => (<option key={value} value={value}>{value}</option>))}
                </select>
              </div>
            </div>
            {usageLoading && !usageData ? (<div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-6 text-sm text-[var(--color-editorial-ink-faint)]">{t("common.loading")}</div>) : null}
            {usageError ? (<div className="mt-4 rounded-[22px] border border-red-200 bg-red-50 px-4 py-4 text-sm text-red-800">{usageError}</div>) : null}
            {usageData && !usageData.configured ? (
              <div className="mt-4 rounded-[22px] border border-dashed border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] px-4 py-6 text-sm text-[var(--color-editorial-ink-faint)]">
                <div className="font-medium text-[var(--color-editorial-ink)]">{t("poeModels.usage.notConfiguredTitle")}</div>
                <div className="mt-2 leading-7">{t("poeModels.usage.notConfiguredDescription")}</div>
              </div>
            ) : null}
            {usageConfigured ? (
              <>
                <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                  <div>{t("poeModels.usage.lastSync")} · {formatDateTime(usageData?.last_sync_run?.finished_at ?? usageData?.last_sync_run?.started_at)}</div>
                  <div>{t("poeModels.usage.syncStatus")} · {usageData?.last_sync_run?.status ?? "—"}</div>
                  <div>{t("poeModels.usage.rangeWindow")} · {formatDateTime(usageData?.range_started_at)} - {formatDateTime(usageData?.range_ended_at)}</div>
                </div>
                <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-5">
                  <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.usage.currentBalance")}</div>
                    <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{(usageData?.current_point_balance ?? 0).toLocaleString()}</div>
                  </div>
                  <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.usage.totalPoints")}</div>
                    <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{(usageData?.summary.total_cost_points ?? 0).toLocaleString()}</div>
                  </div>
                  <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.usage.totalUsd")}</div>
                    <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{formatUSD(usageData?.summary.total_cost_usd ?? 0)}</div>
                  </div>
                  <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.usage.avgPointsPerCall")}</div>
                    <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{formatMetricNumber(averagePointsPerCall)}</div>
                  </div>
                  <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.usage.avgUsdPerCall")}</div>
                    <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{formatUSD(averageUsdPerCall)}</div>
                  </div>
                </div>
              </>
            ) : null}
          </section>
          {usageConfigured ? (
            <section className="space-y-4">
              <section className="surface-editorial rounded-[28px] p-5">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.usage.byModel")}</div>
                <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("poeModels.usage.byModelTitle")}</h3>
                {usageData?.model_summaries?.length ? (
                  <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)]">
                    <div className="overflow-x-auto rounded-[22px]">
                      <table className="w-full divide-y divide-[var(--color-editorial-line)] text-sm">
                        <thead className="bg-[var(--color-editorial-panel)] text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                          <tr>
                            <th className="px-4 py-3">{t("poeModels.usage.table.model")}</th>
                            <th className="px-4 py-3 text-right">{t("poeModels.usage.totalPoints")}</th>
                            <th className="px-4 py-3 text-right">{t("poeModels.usage.totalUsd")}</th>
                            <th className="px-4 py-3 text-right">{t("poeModels.usage.avgPointsPerCall")}</th>
                            <th className="px-4 py-3 text-right">{t("poeModels.usage.avgUsdPerCall")}</th>
                            <th className="px-4 py-3">{t("poeModels.usage.table.calls")}</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                          {usageData.model_summaries.slice(0, 8).map((row) => (
                            <tr key={row.bot_name}>
                              <td className="px-4 py-3 text-[var(--color-editorial-ink)]">{row.bot_name}</td>
                              <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">{row.total_cost_points.toLocaleString()}</td>
                              <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">{formatUSD(row.total_cost_usd)}</td>
                              <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">{formatMetricNumber(row.average_cost_points)}</td>
                              <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">{formatUSD(row.average_cost_usd)}</td>
                              <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{row.entry_count.toLocaleString()}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                ) : (<EmptyState>{t("poeModels.usage.empty")}</EmptyState>)}
              </section>
              <section className="surface-editorial rounded-[28px] p-5">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("poeModels.usage.recent")}</div>
                <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("poeModels.usage.recentTitle")}</h3>
                {usageData?.entries?.length ? (
                  <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)]">
                    <div className="overflow-x-auto rounded-[22px]">
                      <table className="w-full divide-y divide-[var(--color-editorial-line)] text-sm">
                        <thead className="bg-[var(--color-editorial-panel)] text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                          <tr>
                            <th className="px-4 py-3">{t("poeModels.usage.table.model")}</th>
                            <th className="px-4 py-3">{t("poeModels.usage.table.type")}</th>
                            <th className="px-4 py-3 text-right">{t("poeModels.usage.totalPoints")}</th>
                            <th className="px-4 py-3 text-right">{t("poeModels.usage.totalUsd")}</th>
                            <th className="px-4 py-3">{t("poeModels.usage.table.createdAt")}</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                          {usageData.entries.map((row) => (
                            <tr key={row.query_id}>
                              <td className="px-4 py-3">
                                <div className="text-[var(--color-editorial-ink)]">{row.bot_name}</div>
                                {row.chat_name ? (<div className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">{row.chat_name}</div>) : (<div className="mt-1 text-xs text-[var(--color-editorial-ink-faint)]">{row.query_id}</div>)}
                              </td>
                              <td className="px-4 py-3 text-[var(--color-editorial-ink-soft)]">{row.usage_type || "API"}</td>
                              <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">{row.cost_points.toLocaleString()}</td>
                              <td className="px-4 py-3 text-right text-[var(--color-editorial-ink-soft)]">{formatUSD(row.cost_usd)}</td>
                              <td className="whitespace-nowrap px-4 py-3 text-[var(--color-editorial-ink-soft)]">{formatDateTime(row.created_at)}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                ) : (<EmptyState>{t("poeModels.usage.empty")}</EmptyState>)}
              </section>
            </section>
          ) : null}
        </section>
      ) : null}

      {selectedModel ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(31,26,23,0.45)] px-4 py-6" onClick={() => setSelectedModel(null)}>
          <div className="flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] shadow-[var(--shadow-card)]" onClick={(event) => event.stopPropagation()}>
            <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.72),rgba(255,253,249,0.96))] px-5 py-4">
              <div className="min-w-0">
                <div className="text-xs font-medium uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">{selectedModel.owned_by || t("common.unknown")}</div>
                <h2 className="mt-2 break-words font-serif text-[1.55rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{selectedModel.display_name || selectedModel.model_id}</h2>
                <div className="mt-1 flex items-center gap-2">
                  <p className="min-w-0 break-all text-xs text-[var(--color-editorial-ink-faint)]">{selectedModel.model_id}</p>
                  <button type="button" onClick={handleCopyModelId} className="inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-faint)] hover:border-[var(--color-editorial-line-strong)] hover:text-[var(--color-editorial-ink)]" aria-label={t("poeModels.modal.copyModelId")} title={t("poeModels.modal.copyModelId")}>
                    <Copy className="size-3.5" aria-hidden="true" />
                  </button>
                </div>
              </div>
              <button type="button" onClick={() => setSelectedModel(null)} className="inline-flex size-9 items-center justify-center rounded-lg border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:border-[var(--color-editorial-line-strong)] hover:text-[var(--color-editorial-ink)]" aria-label={t("common.close")}>
                <X className="size-4" aria-hidden="true" />
              </button>
            </div>
            <div className="overflow-auto px-5 py-4">
              <div className="grid gap-3 sm:grid-cols-3">
                <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                  <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("poeModels.context")}</div>
                  <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedModel.context_length ? selectedModel.context_length.toLocaleString() : "—"}</div>
                </div>
                <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                  <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("poeModels.table.transport")}</div>
                  <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">{t(transportLabel(selectedModel))}</div>
                </div>
                <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                  <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("poeModels.pricing")}</div>
                  <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">{pricingSummary(selectedModel) ?? "—"}</div>
                </div>
              </div>
              <section className="mt-4">
                <h3 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("poeModels.modal.descriptionJa")}</h3>
                <p className="mt-2 whitespace-pre-wrap text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedDescription}</p>
              </section>
              {selectedDescriptionEn && selectedDescriptionEn !== selectedDescription ? (
                <section className="mt-4">
                  <h3 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("poeModels.modal.descriptionEn")}</h3>
                  <p className="mt-2 whitespace-pre-wrap text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedDescriptionEn}</p>
                </section>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </ModelCatalogPage>
  );
}
