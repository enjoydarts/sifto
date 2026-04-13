"use client";

import { useCallback, useMemo, useState } from "react";
import { Copy, Link2, X } from "lucide-react";
import { api, OpenRouterModelListEntry, OpenRouterModelsResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { useModelCatalog, useModelSort, parseObject, formatPrice, limitSummaryModels } from "@/components/model-catalog/use-model-catalog";
import { ModelCatalogPage, ModelCatalogFilters, SectionHeading, EmptyState } from "@/components/model-catalog/model-catalog-page";

type SortKey = "provider" | "model" | "context" | "pricing" | "params";
type OpenRouterSection = "overview" | "available" | "constrained";

function parseStringArray(raw: unknown): string[] {
  if (!raw) return [];
  if (Array.isArray(raw)) return raw.filter((value): value is string => typeof value === "string");
  if (typeof raw === "string") {
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed.filter((value): value is string => typeof value === "string") : [];
    } catch { return []; }
  }
  return [];
}

function pricingSummary(model: OpenRouterModelListEntry) {
  const pricing = parseObject(model.pricing_json);
  const input = formatPrice(pricing.prompt);
  const output = formatPrice(pricing.completion);
  const cacheRead = formatPrice(pricing.cache_read ?? pricing.input_cache_read);
  const parts = [input ? `in ${input}` : null, output ? `out ${output}` : null, cacheRead ? `cache ${cacheRead}` : null].filter(Boolean);
  return parts.length > 0 ? `${parts.join(" / ")} / 1M tok` : null;
}

function syncProgressLabel(
  t: (key: string, fallback?: string) => string,
  run: OpenRouterModelsResponse["latest_run"] | undefined,
) {
  if (!run || run.translation_target_count <= 0) return null;
  return t("openrouterModels.progressLabel").replace("{{completed}}", String(run.translation_completed_count)).replace("{{total}}", String(run.translation_target_count));
}

function pricingScore(model: OpenRouterModelListEntry) {
  const pricing = parseObject(model.pricing_json);
  const prompt = typeof pricing.prompt === "number" ? pricing.prompt : typeof pricing.prompt === "string" ? Number(pricing.prompt) : NaN;
  const completion = typeof pricing.completion === "number" ? pricing.completion : typeof pricing.completion === "string" ? Number(pricing.completion) : NaN;
  const cacheReadRaw = pricing.cache_read ?? pricing.input_cache_read;
  const cacheRead = typeof cacheReadRaw === "number" ? cacheReadRaw : typeof cacheReadRaw === "string" ? Number(cacheReadRaw) : NaN;
  return [prompt, completion, cacheRead].reduce((sum, value) => (Number.isFinite(value) ? sum + value : sum), 0);
}

function recentChangeLabelKey(change?: OpenRouterModelListEntry["recent_change"]) {
  switch (change) {
    case "available": return "openrouterModels.recentChange.added";
    case "constrained": return "openrouterModels.recentChange.constrained";
    case "removed": return "openrouterModels.recentChange.removed";
    default: return null;
  }
}

function recentChangeClassName(change?: OpenRouterModelListEntry["recent_change"]) {
  switch (change) {
    case "available": return "bg-emerald-50 text-emerald-700 border-emerald-200";
    case "constrained": return "bg-amber-50 text-amber-700 border-amber-200";
    case "removed": return "bg-red-50 text-red-700 border-red-200";
    default: return "bg-zinc-50 text-zinc-700 border-zinc-200";
  }
}

export default function OpenRouterModelsPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [providerFilter, setProviderFilter] = useState("");
  const [selectedModel, setSelectedModel] = useState<OpenRouterModelListEntry | null>(null);
  const [activeSection, setActiveSection] = useState<OpenRouterSection>("overview");
  const [savingOverrideModelId, setSavingOverrideModelId] = useState<string | null>(null);

  const { loading, syncing, error, data, query, setQuery, load, handleSync } = useModelCatalog<OpenRouterModelsResponse>({
    fetchData: () => api.getOpenRouterModels(),
    syncData: async () => {
      const next = await api.syncOpenRouterModels();
      window.dispatchEvent(new Event("openrouter-sync-started"));
      return next;
    },
    syncSuccessKey: "openrouterModels.syncCompleted",
  });

  const { sortKey, setSort, sortMarker } = useModelSort<SortKey>("provider", ["context", "pricing", "params"]);

  const handleCopyModelId = useCallback(async () => {
    if (!selectedModel) return;
    try {
      await navigator.clipboard.writeText(selectedModel.model_id);
      showToast(t("openrouterModels.toast.modelIdCopied"), "success");
    } catch {
      showToast(t("openrouterModels.toast.modelIdCopyFailed"), "error");
    }
  }, [selectedModel, showToast, t]);

  const handleStructuredOutputOverride = useCallback(
    async (model: OpenRouterModelListEntry, enabled: boolean) => {
      setSavingOverrideModelId(model.model_id);
      try {
        await api.setOpenRouterStructuredOutputOverride(model.model_id, enabled);
        await load();
        if (selectedModel?.model_id === model.model_id) {
          setSelectedModel((current) =>
            current ? { ...current, override_enabled: enabled, availability: enabled ? "available" : current.raw_availability } : current,
          );
        }
        showToast(enabled ? t("openrouterModels.toast.overrideEnabled") : t("openrouterModels.toast.overrideDisabled"), "success");
      } catch (e) {
        showToast(String(e), "error");
      } finally {
        setSavingOverrideModelId(null);
      }
    },
    [load, selectedModel?.model_id, showToast, t],
  );

  const isOverrideEligible = useCallback(
    (model: OpenRouterModelListEntry) => model.raw_availability === "constrained" || model.override_enabled,
    [],
  );

  const providerOptions = useMemo(() => {
    const values = new Set<string>();
    for (const model of [...(data?.models ?? []), ...(data?.unavailable_models ?? [])]) {
      if (model.provider_slug) values.add(model.provider_slug);
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [data?.models, data?.unavailable_models]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const models = [...(data?.models ?? []), ...(data?.unavailable_models ?? [])];
    return models.filter((model) => {
      if (providerFilter && model.provider_slug !== providerFilter) return false;
      if (!q) return true;
      return [model.model_id, model.display_name, model.provider_slug].join(" ").toLowerCase().includes(q);
    });
  }, [data?.models, data?.unavailable_models, providerFilter, query]);

  const sorted = useMemo(() => {
    const arr = [...filtered];
    arr.sort((a, b) => {
      let result = 0;
      switch (sortKey) {
        case "provider": result = (a.provider_slug || "").localeCompare(b.provider_slug || ""); break;
        case "model": result = (a.display_name || a.model_id).localeCompare(b.display_name || b.model_id); break;
        case "context": result = (a.context_length ?? -1) - (b.context_length ?? -1); break;
        case "pricing": result = pricingScore(a) - pricingScore(b); break;
        case "params": result = parseStringArray(a.supported_parameters_json).length - parseStringArray(b.supported_parameters_json).length; break;
      }
      if (result === 0) result = a.model_id.localeCompare(b.model_id);
      return result;
    });
    return arr;
  }, [filtered, sortKey]);

  const availableModels = useMemo(() => sorted.filter((model) => model.availability === "available"), [sorted]);
  const unavailableModels = useMemo(() => sorted.filter((model) => model.availability !== "available"), [sorted]);

  const latestSummary = data?.latest_change_summary ?? null;
  const latestSummaryTriggerLabel = latestSummary?.trigger === "manual" ? t("openrouterModels.summaryTrigger.manual") : t("openrouterModels.summaryTrigger.cron");
  const translatedCount = data?.latest_run?.translation_completed_count ?? 0;
  const translationTargetCount = data?.latest_run?.translation_target_count ?? 0;
  const fetchedCount = data?.latest_run?.fetched_count ?? data?.models.length ?? 0;
  const acceptedCount = data?.latest_run?.accepted_count ?? data?.models.length ?? 0;
  const constrainedCount = data?.unavailable_models.filter((model) => model.availability === "constrained").length ?? 0;
  const removedCount = data?.unavailable_models.filter((model) => model.availability === "removed").length ?? 0;
  const latestRunLabel = data?.latest_run?.finished_at ? new Date(data.latest_run.finished_at).toLocaleString() : t("openrouterModels.latestRunEmpty");
  const selectedDescription = selectedModel ? selectedModel.description_ja ?? selectedModel.description_en ?? t("openrouterModels.descriptionFallback") : "";
  const selectedDescriptionEn = selectedModel?.description_en ?? "";
  const selectedSupported = selectedModel ? parseStringArray(selectedModel.supported_parameters_json) : [];
  const selectedPricing = selectedModel ? pricingSummary(selectedModel) : null;

  const sections = [
    { key: "overview", label: t("modelCatalog.overview"), meta: `${t("openrouterModels.latestRun")} · ${latestRunLabel}` },
    { key: "available", label: t("openrouterModels.table.availableModels"), meta: `${availableModels.length} ${t("common.rows")}` },
    { key: "constrained", label: t("openrouterModels.table.unavailableModels"), meta: `${unavailableModels.length} ${t("common.rows")}` },
  ];

  const statusContent = (
    <>
      <div>{t("openrouterModels.latestRun")} · {latestRunLabel}</div>
      <div>{t("openrouterModels.fetched")} · {fetchedCount}</div>
      <div>{t("openrouterModels.accepted")} · {acceptedCount}</div>
      <div>{syncProgressLabel(t, data?.latest_run) ?? t("openrouterModels.progressPreparing")}</div>
    </>
  );

  const renderParamsCell = (model: OpenRouterModelListEntry) => {
    const supported = parseStringArray(model.supported_parameters_json);
    if (supported.length === 0) return "—";
    return (
      <div className="flex flex-nowrap gap-1.5">
        {supported.slice(0, 4).map((param) => (
          <span key={param} className="whitespace-nowrap rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-1 text-[11px] text-[var(--color-editorial-ink-soft)]">{param}</span>
        ))}
        {supported.length > 4 ? (<span className="whitespace-nowrap rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-1 text-[11px] text-[var(--color-editorial-ink-faint)]">+{supported.length - 4}</span>) : null}
      </div>
    );
  };

  return (
    <ModelCatalogPage
      title={t("nav.openrouterModels")}
      titleIcon={Link2}
      description={t("openrouterModels.subtitle")}
      syncing={syncing}
      onSync={handleSync}
      syncLabel={t("openrouterModels.sync")}
      syncingLabel={t("openrouterModels.syncing")}
      sections={sections}
      activeSection={activeSection}
      onSectionChange={(k) => setActiveSection(k as OpenRouterSection)}
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
                <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("openrouterModels.overview.heading")}</h2>
                <p className="mt-3 max-w-3xl text-[14px] leading-7 text-[var(--color-editorial-ink-soft)]">{t("openrouterModels.overview.description")}</p>
              </div>
              <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                {data?.latest_run?.status === "running" && data.latest_run.trigger_type === "manual" ? t("openrouterModels.progressRunning") : latestRunLabel}
              </div>
            </div>
            <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("openrouterModels.fetched")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{fetchedCount}</div>
              </div>
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("openrouterModels.accepted")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{acceptedCount}</div>
              </div>
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("openrouterModels.recentChange.constrained")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{constrainedCount}</div>
              </div>
              <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("modelCatalog.translated")}</div>
                <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{translatedCount} / {translationTargetCount}</div>
              </div>
            </div>
            {data?.latest_run?.status === "running" && data.latest_run.trigger_type === "manual" ? (
              <div className="mt-4 rounded-[22px] border border-[#ead5af] bg-[#faf1dd] px-4 py-3 text-sm text-[#916321]">
                <div className="font-medium">{t("openrouterModels.progressRunning")}</div>
                <div className="mt-1">{syncProgressLabel(t, data.latest_run) ?? t("openrouterModels.progressPreparing")}</div>
              </div>
            ) : null}
          </section>
          <section className="grid gap-4 xl:grid-cols-[1.25fr_1fr]">
            <section className="surface-editorial rounded-[28px] p-5">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("openrouterModels.latestSummary.title")}</div>
              <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("openrouterModels.latestSummary.title")}</h3>
              {latestSummary ? (
                <p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{latestSummaryTriggerLabel} · {new Date(latestSummary.detected_at).toLocaleString()}</p>
              ) : (<p className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-faint)]">{t("openrouterModels.latestSummary.none")}</p>)}
              <div className="mt-4 space-y-3">
                {[
                  { key: "added", label: t("openrouterModels.recentChange.added"), models: latestSummary?.added ?? [], className: "border-emerald-200 bg-emerald-50 text-emerald-800" },
                  { key: "constrained", label: t("openrouterModels.recentChange.constrained"), models: latestSummary?.constrained ?? [], className: "border-amber-200 bg-amber-50 text-amber-800" },
                  { key: "removed", label: t("openrouterModels.recentChange.removed"), models: latestSummary?.removed ?? [], className: "border-red-200 bg-red-50 text-red-800" },
                ].map((group) => {
                  const summary = limitSummaryModels(group.models);
                  return (
                    <div key={group.key} className={`rounded-[22px] border px-4 py-3 ${group.className}`}>
                      <div className="text-sm font-semibold">{group.label} {group.models.length > 0 ? `(${group.models.length})` : ""}</div>
                      {group.models.length > 0 ? (
                        <div className="mt-3 space-y-2 text-xs">
                          {summary.items.map((modelID) => (<div key={modelID} className="rounded-[14px] border border-current/15 bg-white/70 px-3 py-2">{modelID}</div>))}
                          {summary.remaining > 0 ? (<div className="rounded-[14px] border border-current/15 bg-white/70 px-3 py-2">+{summary.remaining}</div>) : null}
                        </div>
                      ) : (<div className="mt-3 text-xs opacity-70">{t("openrouterModels.latestSummary.none")}</div>)}
                    </div>
                  );
                })}
              </div>
            </section>
            <section className="surface-editorial rounded-[28px] p-5">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("modelCatalog.translation")}</div>
              <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("openrouterModels.translation.heading")}</h3>
              <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">{t("openrouterModels.progressGlobal")}</div>
                <div className="mt-3 text-lg leading-none text-[var(--color-editorial-ink)]">{translatedCount} / {translationTargetCount}</div>
                <div className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{syncProgressLabel(t, data?.latest_run) ?? t("openrouterModels.progressPreparing")}</div>
              </div>
              <div className="mt-3 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                {t("openrouterModels.latestRun")} · {latestRunLabel}<br />{t("openrouterModels.recentChange.removed")} · {removedCount}
              </div>
            </section>
          </section>
        </>
      ) : null}

      {activeSection === "available" ? (
        <>
          <ModelCatalogFilters query={query} onQueryChange={setQuery} searchLabel={t("openrouterModels.search")} searchPlaceholder={t("openrouterModels.search")} clearLabel={t("common.clear")} providerFilter={providerFilter} onProviderFilterChange={setProviderFilter} providerFilterLabel={t("openrouterModels.providerFilter")} providerAllLabel={t("openrouterModels.providerAll")} providerOptions={providerOptions} />
          <section className="surface-editorial rounded-[28px] p-5">
            <SectionHeading badge={t("openrouterModels.providerGroup")} title={t("openrouterModels.table.availableModels")} count={availableModels.length} countLabel={t("common.rows")} />
            {availableModels.length === 0 ? (
              <EmptyState>{t("openrouterModels.noAvailableModels")}</EmptyState>
            ) : (
              <div className="mt-4 overflow-hidden rounded-[22px] border border-[var(--color-editorial-line)]">
                <div className="overflow-x-auto">
                  <table className="min-w-[1120px] divide-y divide-[var(--color-editorial-line)] text-sm">
                    <thead className="bg-[var(--color-editorial-panel)]">
                      <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("provider")}>{t("openrouterModels.table.provider")}{sortMarker("provider")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("model")}>{t("openrouterModels.table.model")}{sortMarker("model")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("context")}>{t("openrouterModels.table.context")}{sortMarker("context")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("pricing")}>{t("openrouterModels.table.pricing")}{sortMarker("pricing")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("params")}>{t("openrouterModels.table.params")}{sortMarker("params")}</button></th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                      {availableModels.map((model) => {
                        const recentChangeKey = recentChangeLabelKey(model.recent_change);
                        return (
                          <tr key={model.model_id} className="cursor-pointer transition hover:bg-[var(--color-editorial-panel)]" onClick={() => setSelectedModel(model)}>
                            <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.provider_slug || "—"}</td>
                            <td className="whitespace-nowrap px-4 py-3 align-top">
                              <div className="flex items-center gap-2 whitespace-nowrap">
                                <div className="whitespace-nowrap font-medium text-[var(--color-editorial-ink)]">{model.display_name || model.model_id}</div>
                                {recentChangeKey ? (<span className={`rounded-full border px-2 py-1 text-[11px] font-medium ${recentChangeClassName(model.recent_change)}`}>{t(recentChangeKey)}</span>) : null}
                              </div>
                              <div className="mt-1 whitespace-nowrap text-xs text-[var(--color-editorial-ink-faint)]">{model.model_id}</div>
                              {model.override_enabled ? (<div className="mt-2"><span className="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-1 text-[11px] font-medium text-emerald-700">{t("openrouterModels.override.enabledBadge")}</span></div>) : null}
                            </td>
                            <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.context_length ? model.context_length.toLocaleString() : "—"}</td>
                            <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{pricingSummary(model) ?? "—"}</td>
                            <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{renderParamsCell(model)}</td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </section>
        </>
      ) : null}

      {activeSection === "constrained" ? (
        <>
          <ModelCatalogFilters query={query} onQueryChange={setQuery} searchLabel={t("openrouterModels.search")} searchPlaceholder={t("openrouterModels.search")} clearLabel={t("common.clear")} providerFilter={providerFilter} onProviderFilterChange={setProviderFilter} providerFilterLabel={t("openrouterModels.providerFilter")} providerAllLabel={t("openrouterModels.providerAll")} providerOptions={providerOptions} />
          <section className="surface-editorial rounded-[28px] p-5">
            <SectionHeading badge={t("openrouterModels.providerGroup")} title={t("openrouterModels.table.unavailableModels")} count={unavailableModels.length} countLabel={t("common.rows")} />
            {unavailableModels.length === 0 ? (
              <EmptyState>{t("openrouterModels.noUnavailableModels")}</EmptyState>
            ) : (
              <div className="mt-4 overflow-hidden rounded-[22px] border border-[var(--color-editorial-line)]">
                <div className="overflow-x-auto">
                  <table className="min-w-[1240px] divide-y divide-[var(--color-editorial-line)] text-sm">
                    <thead className="bg-[var(--color-editorial-panel)]">
                      <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                        <th className="px-4 py-3">{t("openrouterModels.table.state")}</th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("provider")}>{t("openrouterModels.table.provider")}{sortMarker("provider")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("model")}>{t("openrouterModels.table.model")}{sortMarker("model")}</button></th>
                        <th className="px-4 py-3">{t("openrouterModels.table.reason")}</th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("context")}>{t("openrouterModels.table.context")}{sortMarker("context")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("pricing")}>{t("openrouterModels.table.pricing")}{sortMarker("pricing")}</button></th>
                        <th className="px-4 py-3"><button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("params")}>{t("openrouterModels.table.params")}{sortMarker("params")}</button></th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                      {unavailableModels.map((model) => {
                        const stateKey = model.availability === "removed" ? "openrouterModels.state.removed" : "openrouterModels.state.constrained";
                        const reasonKey = model.reason === "removed" ? "openrouterModels.reason.removed" : "openrouterModels.reason.structuredOutput";
                        const recentChangeKey = recentChangeLabelKey(model.recent_change);
                        return (
                          <tr key={model.model_id} className="cursor-pointer transition hover:bg-[var(--color-editorial-panel)]" onClick={() => setSelectedModel(model)}>
                            <td className="whitespace-nowrap px-4 py-3 align-top"><span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-1 text-[11px] text-[var(--color-editorial-ink-soft)]">{t(stateKey)}</span></td>
                            <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.provider_slug || "—"}</td>
                            <td className="whitespace-nowrap px-4 py-3 align-top">
                              <div className="flex items-center gap-2 whitespace-nowrap">
                                <div className="whitespace-nowrap font-medium text-[var(--color-editorial-ink)]">{model.display_name || model.model_id}</div>
                                {recentChangeKey ? (<span className={`rounded-full border px-2 py-1 text-[11px] font-medium ${recentChangeClassName(model.recent_change)}`}>{t(recentChangeKey)}</span>) : null}
                              </div>
                              <div className="mt-1 whitespace-nowrap text-xs text-[var(--color-editorial-ink-faint)]">{model.model_id}</div>
                              {model.override_enabled ? (<div className="mt-2"><span className="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-1 text-[11px] font-medium text-emerald-700">{t("openrouterModels.override.enabledBadge")}</span></div>) : null}
                            </td>
                            <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{t(reasonKey)}</td>
                            <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.context_length ? model.context_length.toLocaleString() : "—"}</td>
                            <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{pricingSummary(model) ?? "—"}</td>
                            <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{renderParamsCell(model)}</td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </section>
        </>
      ) : null}

      {sorted.length === 0 ? (
        <section className="surface-editorial rounded-[28px] border border-dashed border-[var(--color-editorial-line-strong)] p-6 text-sm text-[var(--color-editorial-ink-faint)]">{t("openrouterModels.noModels")}</section>
      ) : null}

      {selectedModel ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(31,26,23,0.45)] px-4 py-6" onClick={() => setSelectedModel(null)}>
          <div className="flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] shadow-[var(--shadow-card)]" onClick={(event) => event.stopPropagation()}>
            <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.72),rgba(255,253,249,0.96))] px-5 py-4">
              <div className="min-w-0">
                <div className="text-xs font-medium uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">{selectedModel.provider_slug || t("common.unknown")}</div>
                <h2 className="mt-2 break-words font-serif text-[1.55rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">{selectedModel.display_name || selectedModel.model_id}</h2>
                <div className="mt-1 flex items-center gap-2">
                  <p className="min-w-0 break-all text-xs text-[var(--color-editorial-ink-faint)]">{selectedModel.model_id}</p>
                  <button type="button" onClick={handleCopyModelId} className="inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-faint)] hover:border-[var(--color-editorial-line-strong)] hover:text-[var(--color-editorial-ink)]" aria-label={t("openrouterModels.modal.copyModelId")} title={t("openrouterModels.modal.copyModelId")}>
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
                  <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("openrouterModels.context")}</div>
                  <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedModel.context_length ? selectedModel.context_length.toLocaleString() : "—"}</div>
                </div>
                <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4 sm:col-span-2">
                  <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("openrouterModels.pricing")}</div>
                  <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">{selectedPricing ?? "—"}</div>
                </div>
              </div>
              <section className="mt-4">
                <h3 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("openrouterModels.modal.descriptionJa")}</h3>
                <p className="mt-2 whitespace-pre-wrap text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedDescription}</p>
              </section>
              {selectedDescriptionEn && selectedDescriptionEn !== selectedDescription ? (
                <section className="mt-4">
                  <h3 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("openrouterModels.modal.descriptionEn")}</h3>
                  <p className="mt-2 whitespace-pre-wrap text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{selectedDescriptionEn}</p>
                </section>
              ) : null}
              <section className="mt-4">
                <h3 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("openrouterModels.supportedParameters")}</h3>
                {selectedSupported.length > 0 ? (
                  <div className="mt-2 flex flex-wrap gap-2">
                    {selectedSupported.map((param) => (<span key={param} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-1 text-xs text-[var(--color-editorial-ink-soft)]">{param}</span>))}
                  </div>
                ) : (<p className="mt-2 text-sm text-[var(--color-editorial-ink-faint)]">—</p>)}
              </section>
              {isOverrideEligible(selectedModel) ? (
                <section className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                  <h3 className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("openrouterModels.override.title")}</h3>
                  <p className="mt-2 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">{t("openrouterModels.override.help")}</p>
                  <div className="mt-3">
                    <button type="button" disabled={savingOverrideModelId === selectedModel.model_id} onClick={() => void handleStructuredOutputOverride(selectedModel, !selectedModel.override_enabled)} className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink)] hover:bg-[var(--color-editorial-panel)] disabled:opacity-60">
                      {selectedModel.override_enabled ? t("openrouterModels.override.disable") : t("openrouterModels.override.enable")}
                    </button>
                  </div>
                </section>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </ModelCatalogPage>
  );
}
