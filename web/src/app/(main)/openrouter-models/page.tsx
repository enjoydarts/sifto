"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Copy, Link2, RefreshCw, Search, X } from "lucide-react";
import { api, OpenRouterModelListEntry, OpenRouterModelsResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { Tabs, TabList, Tab, TabPanel } from "@/components/tabs";

function parseObject(raw: unknown): Record<string, unknown> {
  if (!raw) return {};
  if (typeof raw === "string") {
    try {
      return JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return {};
    }
  }
  if (typeof raw === "object") {
    return raw as Record<string, unknown>;
  }
  return {};
}

function parseStringArray(raw: unknown): string[] {
  if (!raw) return [];
  if (Array.isArray(raw)) {
    return raw.filter((value): value is string => typeof value === "string");
  }
  if (typeof raw === "string") {
    try {
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed.filter((value): value is string => typeof value === "string") : [];
    } catch {
      return [];
    }
  }
  return [];
}

function formatPrice(value: unknown) {
  const num = typeof value === "number" ? value : typeof value === "string" ? Number(value) : NaN;
  if (!Number.isFinite(num)) return null;
  const perMillion = num * 1_000_000;
  if (perMillion === 0) return "free";
  if (perMillion >= 1) return `$${perMillion.toFixed(2)}`;
  if (perMillion >= 0.01) return `$${perMillion.toFixed(3)}`;
  return `$${perMillion.toFixed(4)}`;
}

function pricingSummary(model: OpenRouterModelListEntry) {
  const pricing = parseObject(model.pricing_json);
  const input = formatPrice(pricing.prompt);
  const output = formatPrice(pricing.completion);
  const cacheRead = formatPrice(pricing.cache_read ?? pricing.input_cache_read);
  const parts = [
    input ? `in ${input}` : null,
    output ? `out ${output}` : null,
    cacheRead ? `cache ${cacheRead}` : null,
  ].filter(Boolean);
  return parts.length > 0 ? `${parts.join(" / ")} / 1M tok` : null;
}

function syncProgressLabel(t: (key: string, fallback?: string) => string, run: OpenRouterModelsResponse["latest_run"]) {
  if (!run || run.translation_target_count <= 0) return null;
  return t("openrouterModels.progressLabel")
    .replace("{{completed}}", String(run.translation_completed_count))
    .replace("{{total}}", String(run.translation_target_count));
}

type SortKey = "provider" | "model" | "context" | "pricing" | "params";
type SortDirection = "asc" | "desc";

function pricingScore(model: OpenRouterModelListEntry) {
  const pricing = parseObject(model.pricing_json);
  const prompt = typeof pricing.prompt === "number" ? pricing.prompt : typeof pricing.prompt === "string" ? Number(pricing.prompt) : NaN;
  const completion =
    typeof pricing.completion === "number" ? pricing.completion : typeof pricing.completion === "string" ? Number(pricing.completion) : NaN;
  const cacheReadRaw = pricing.cache_read ?? pricing.input_cache_read;
  const cacheRead =
    typeof cacheReadRaw === "number" ? cacheReadRaw : typeof cacheReadRaw === "string" ? Number(cacheReadRaw) : NaN;
  return [prompt, completion, cacheRead].reduce((sum, value) => (Number.isFinite(value) ? sum + value : sum), 0);
}

function recentChangeLabelKey(change?: OpenRouterModelListEntry["recent_change"]) {
  switch (change) {
    case "available":
      return "openrouterModels.recentChange.added";
    case "constrained":
      return "openrouterModels.recentChange.constrained";
    case "removed":
      return "openrouterModels.recentChange.removed";
    default:
      return null;
  }
}

function recentChangeClassName(change?: OpenRouterModelListEntry["recent_change"]) {
  switch (change) {
    case "available":
      return "bg-emerald-50 text-emerald-700 border-emerald-200";
    case "constrained":
      return "bg-amber-50 text-amber-700 border-amber-200";
    case "removed":
      return "bg-red-50 text-red-700 border-red-200";
    default:
      return "bg-zinc-50 text-zinc-700 border-zinc-200";
  }
}

function limitSummaryModels(models: { model_id: string }[], limit = 5) {
  return {
    items: models.slice(0, limit).map((item) => item.model_id),
    remaining: Math.max(models.length - limit, 0),
  };
}

export default function OpenRouterModelsPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [providerFilter, setProviderFilter] = useState("");
  const [data, setData] = useState<OpenRouterModelsResponse | null>(null);
  const [selectedModel, setSelectedModel] = useState<OpenRouterModelListEntry | null>(null);
  const [activeTab, setActiveTab] = useState<"available" | "unavailable">("available");
  const [sortKey, setSortKey] = useState<SortKey>("provider");
  const [sortDirection, setSortDirection] = useState<SortDirection>("asc");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const next = await api.getOpenRouterModels();
      setData(next);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (data?.latest_run?.status !== "running" || data.latest_run.trigger_type !== "manual") return;
    const timer = window.setInterval(() => {
      load();
    }, 3000);
    return () => window.clearInterval(timer);
  }, [data?.latest_run, load]);

  const handleSync = useCallback(async () => {
    setSyncing(true);
    try {
      const next = await api.syncOpenRouterModels();
      setData(next);
      setError(null);
      window.dispatchEvent(new Event("openrouter-sync-started"));
      showToast(t("openrouterModels.syncCompleted"), "success");
    } catch (e) {
      showToast(String(e), "error");
    } finally {
      setSyncing(false);
    }
  }, [showToast, t]);

  const handleCopyModelId = useCallback(async () => {
    if (!selectedModel) return;
    try {
      await navigator.clipboard.writeText(selectedModel.model_id);
      showToast(t("openrouterModels.toast.modelIdCopied"), "success");
    } catch {
      showToast(t("openrouterModels.toast.modelIdCopyFailed"), "error");
    }
  }, [selectedModel, showToast, t]);

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
    const models = [...filtered];
    models.sort((a, b) => {
      let result = 0;
      switch (sortKey) {
        case "provider":
          result = (a.provider_slug || "").localeCompare(b.provider_slug || "");
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
        case "params":
          result = parseStringArray(a.supported_parameters_json).length - parseStringArray(b.supported_parameters_json).length;
          break;
      }
      if (result === 0) {
        result = a.model_id.localeCompare(b.model_id);
      }
      return sortDirection === "asc" ? result : -result;
    });
    return models;
  }, [filtered, sortDirection, sortKey]);

  const availableModels = useMemo(() => sorted.filter((model) => model.availability === "available"), [sorted]);
  const unavailableModels = useMemo(() => sorted.filter((model) => model.availability !== "available"), [sorted]);

  const setSort = useCallback(
    (nextKey: SortKey) => {
      if (sortKey === nextKey) {
        setSortDirection(sortDirection === "asc" ? "desc" : "asc");
        return;
      }
      setSortKey(nextKey);
      setSortDirection(nextKey === "context" || nextKey === "pricing" || nextKey === "params" ? "desc" : "asc");
    },
    [sortDirection, sortKey],
  );

  const sortMarker = useCallback(
    (key: SortKey) => {
      if (sortKey !== key) return "";
      return sortDirection === "asc" ? " ↑" : " ↓";
    },
    [sortDirection, sortKey],
  );

  const selectedDescription = selectedModel
    ? selectedModel.description_ja ?? selectedModel.description_en ?? t("openrouterModels.descriptionFallback")
    : "";
  const selectedDescriptionEn = selectedModel?.description_en ?? "";
  const selectedSupported = selectedModel ? parseStringArray(selectedModel.supported_parameters_json) : [];
  const selectedPricing = selectedModel ? pricingSummary(selectedModel) : null;
  const latestSummary = data?.latest_change_summary ?? null;
  const latestSummaryTriggerLabel =
    latestSummary?.trigger === "manual" ? t("openrouterModels.summaryTrigger.manual") : t("openrouterModels.summaryTrigger.cron");

  if (loading) return <p className="text-sm text-zinc-500">{t("common.loading")}</p>;
  if (error) return <p className="text-sm text-red-500">{error}</p>;

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight text-zinc-900">
            <Link2 className="size-6 text-zinc-500" aria-hidden="true" />
            <span>{t("openrouterModels.title")}</span>
          </h1>
          <p className="mt-1 text-sm text-zinc-500">{t("openrouterModels.subtitle")}</p>
        </div>
        <button
          type="button"
          onClick={handleSync}
          disabled={syncing}
          className="inline-flex h-10 items-center justify-center gap-2 rounded-lg border border-zinc-200 bg-white px-4 text-sm font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-60"
        >
          <RefreshCw className={`size-4 ${syncing ? "animate-spin" : ""}`} aria-hidden="true" />
          {syncing ? t("openrouterModels.syncing") : t("openrouterModels.sync")}
        </button>
      </div>

      <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
        <div className="grid gap-3 md:grid-cols-3">
          <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
            <div className="text-xs font-medium text-zinc-500">{t("openrouterModels.latestRun")}</div>
            <div className="mt-1 text-sm font-semibold text-zinc-900">
              {data?.latest_run?.finished_at ? new Date(data.latest_run.finished_at).toLocaleString() : t("openrouterModels.latestRunEmpty")}
            </div>
          </div>
          <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
            <div className="text-xs font-medium text-zinc-500">{t("openrouterModels.fetched")}</div>
            <div className="mt-1 text-sm font-semibold text-zinc-900">{data?.latest_run?.fetched_count ?? 0}</div>
          </div>
          <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
            <div className="text-xs font-medium text-zinc-500">{t("openrouterModels.accepted")}</div>
            <div className="mt-1 text-sm font-semibold text-zinc-900">{data?.latest_run?.accepted_count ?? 0}</div>
          </div>
        </div>
        {data?.latest_run?.status === "running" && data.latest_run.trigger_type === "manual" ? (
          <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
            <div className="font-medium">{t("openrouterModels.progressRunning")}</div>
            <div className="mt-1 text-amber-800">
              {syncProgressLabel(t, data.latest_run) ?? t("openrouterModels.progressPreparing")}
            </div>
          </div>
        ) : null}
        <div className="mt-4 flex flex-col gap-2 sm:flex-row">
          <label className="flex shrink-0 items-center gap-2 rounded-xl border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">
            <span className="whitespace-nowrap text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
              {t("openrouterModels.providerFilter")}
            </span>
            <select
              value={providerFilter}
              onChange={(e) => setProviderFilter(e.target.value)}
              className="min-w-0 bg-transparent text-sm text-zinc-900 outline-none"
            >
              <option value="">{t("openrouterModels.providerAll")}</option>
              {providerOptions.map((provider) => (
                <option key={provider} value={provider}>
                  {provider}
                </option>
              ))}
            </select>
          </label>
          <label className="flex min-w-0 flex-1 items-center gap-2 rounded-xl border border-zinc-200 bg-zinc-50 px-3 py-2">
            <Search className="size-4 text-zinc-400" aria-hidden="true" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("openrouterModels.search")}
              className="w-full bg-transparent text-sm text-zinc-900 outline-none placeholder:text-zinc-400"
            />
            {query ? (
              <button
                type="button"
                onClick={() => setQuery("")}
                className="shrink-0 rounded-md px-2 py-1 text-xs font-medium text-zinc-500 hover:bg-white hover:text-zinc-700"
              >
                {t("common.clear")}
              </button>
            ) : null}
          </label>
        </div>
      </section>

      {latestSummary && (latestSummary.added.length > 0 || latestSummary.constrained.length > 0 || latestSummary.removed.length > 0) ? (
        <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 className="text-base font-semibold text-zinc-900">{t("openrouterModels.latestSummary.title")}</h2>
              <p className="mt-1 text-sm text-zinc-500">
                {latestSummaryTriggerLabel} · {new Date(latestSummary.detected_at).toLocaleString()}
              </p>
            </div>
          </div>
          <div className="mt-4 grid gap-3 md:grid-cols-3">
            {[
              { key: "added", label: t("openrouterModels.recentChange.added"), models: latestSummary.added, className: "border-emerald-200 bg-emerald-50 text-emerald-800" },
              { key: "constrained", label: t("openrouterModels.recentChange.constrained"), models: latestSummary.constrained, className: "border-amber-200 bg-amber-50 text-amber-800" },
              { key: "removed", label: t("openrouterModels.recentChange.removed"), models: latestSummary.removed, className: "border-red-200 bg-red-50 text-red-800" },
            ].map((group) => {
              const summary = limitSummaryModels(group.models);
              return (
                <div key={group.key} className={`rounded-xl border px-4 py-3 ${group.className}`}>
                  <div className="text-sm font-semibold">
                    {group.label} {group.models.length > 0 ? `(${group.models.length})` : ""}
                  </div>
                  {group.models.length > 0 ? (
                    <div className="mt-2 flex flex-wrap gap-1.5 text-xs">
                      {summary.items.map((modelID) => (
                        <span key={modelID} className="rounded-full border border-current/20 bg-white/70 px-2 py-1">
                          {modelID}
                        </span>
                      ))}
                      {summary.remaining > 0 ? (
                        <span className="rounded-full border border-current/20 bg-white/70 px-2 py-1">+{summary.remaining}</span>
                      ) : null}
                    </div>
                  ) : (
                    <div className="mt-2 text-xs opacity-70">{t("openrouterModels.latestSummary.none")}</div>
                  )}
                </div>
              );
            })}
          </div>
        </section>
      ) : null}

      {sorted.length === 0 ? (
        <section className="rounded-2xl border border-dashed border-zinc-300 bg-white p-6 text-sm text-zinc-500">
          {t("openrouterModels.noModels")}
        </section>
      ) : (
        <div className="space-y-6">
          <section className="rounded-2xl border border-zinc-200 bg-white shadow-sm">
            <Tabs defaultValue="available" value={activeTab} onChange={(value) => setActiveTab(value as "available" | "unavailable")}>
              <div className="flex items-center justify-between gap-3 px-4 pt-4 md:px-5">
                <div>
                  <div className="text-xs font-medium uppercase tracking-[0.12em] text-zinc-400">{t("openrouterModels.providerGroup")}</div>
                  <h2 className="mt-1 text-lg font-semibold text-zinc-900">
                    {activeTab === "available" ? t("openrouterModels.table.availableModels") : t("openrouterModels.table.unavailableModels")}
                  </h2>
                </div>
                <div className="text-xs text-zinc-500">
                  {(activeTab === "available" ? availableModels.length : unavailableModels.length)} {t("common.rows")}
                </div>
              </div>
              <TabList className="px-4 pt-3 md:px-5">
                <Tab value="available">{t("openrouterModels.table.availableModels")}</Tab>
                <Tab value="unavailable">{t("openrouterModels.table.unavailableModels")}</Tab>
              </TabList>
              <TabPanel value="available" className="px-4 py-4 md:px-5">
                {availableModels.length === 0 ? (
                  <div className="rounded-xl border border-dashed border-zinc-300 bg-zinc-50 px-4 py-6 text-sm text-zinc-500">
                    {t("openrouterModels.noAvailableModels")}
                  </div>
                ) : (
                  <div className="overflow-hidden rounded-xl border border-zinc-200">
                    <div className="overflow-x-auto">
                      <table className="min-w-[1120px] divide-y divide-zinc-200 text-sm">
                <thead className="bg-zinc-50">
                  <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                    <th className="px-4 py-3">
                      <button type="button" className="hover:text-zinc-700" onClick={() => setSort("provider")}>
                        {t("openrouterModels.table.provider")}{sortMarker("provider")}
                      </button>
                    </th>
                    <th className="px-4 py-3">
                      <button type="button" className="hover:text-zinc-700" onClick={() => setSort("model")}>
                        {t("openrouterModels.table.model")}{sortMarker("model")}
                      </button>
                    </th>
                    <th className="px-4 py-3">
                      <button type="button" className="hover:text-zinc-700" onClick={() => setSort("context")}>
                        {t("openrouterModels.table.context")}{sortMarker("context")}
                      </button>
                    </th>
                    <th className="px-4 py-3">
                      <button type="button" className="hover:text-zinc-700" onClick={() => setSort("pricing")}>
                        {t("openrouterModels.table.pricing")}{sortMarker("pricing")}
                      </button>
                    </th>
                    <th className="px-4 py-3">
                      <button type="button" className="hover:text-zinc-700" onClick={() => setSort("params")}>
                        {t("openrouterModels.table.params")}{sortMarker("params")}
                      </button>
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-200 bg-white">
                  {availableModels.map((model) => {
                    const supported = parseStringArray(model.supported_parameters_json);
                    const pricing = pricingSummary(model);
                    const recentChangeKey = recentChangeLabelKey(model.recent_change);
                    return (
                      <tr
                        key={model.model_id}
                        className="cursor-pointer transition hover:bg-zinc-50"
                        onClick={() => setSelectedModel(model)}
                      >
                        <td className="whitespace-nowrap px-4 py-3 align-top text-zinc-700">{model.provider_slug || "—"}</td>
                        <td className="whitespace-nowrap px-4 py-3 align-top">
                          <div className="flex items-center gap-2 whitespace-nowrap">
                            <div className="whitespace-nowrap font-medium text-zinc-900">{model.display_name || model.model_id}</div>
                            {recentChangeKey ? (
                              <span className={`rounded-full border px-2 py-1 text-[11px] font-medium ${recentChangeClassName(model.recent_change)}`}>
                                {t(recentChangeKey)}
                              </span>
                            ) : null}
                          </div>
                          <div className="mt-1 whitespace-nowrap text-xs text-zinc-500">{model.model_id}</div>
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 align-top text-zinc-700">
                          {model.context_length ? model.context_length.toLocaleString() : "—"}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 align-top text-zinc-700">{pricing ?? "—"}</td>
                        <td className="whitespace-nowrap px-4 py-3 align-top text-zinc-700">
                          {supported.length > 0 ? (
                            <div className="flex flex-nowrap gap-1.5">
                              {supported.slice(0, 4).map((param) => (
                                <span
                                  key={param}
                                  className="whitespace-nowrap rounded-full border border-zinc-200 bg-zinc-50 px-2 py-1 text-[11px] text-zinc-600"
                                >
                                  {param}
                                </span>
                              ))}
                              {supported.length > 4 ? (
                                <span className="whitespace-nowrap rounded-full border border-zinc-200 bg-zinc-50 px-2 py-1 text-[11px] text-zinc-500">
                                  +{supported.length - 4}
                                </span>
                              ) : null}
                            </div>
                          ) : (
                            "—"
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
                      </table>
                    </div>
                  </div>
                )}
              </TabPanel>
              <TabPanel value="unavailable" className="px-4 py-4 md:px-5">
                {unavailableModels.length === 0 ? (
                  <div className="rounded-xl border border-dashed border-zinc-300 bg-zinc-50 px-4 py-6 text-sm text-zinc-500">
                    {t("openrouterModels.noUnavailableModels")}
                  </div>
                ) : (
                  <div className="overflow-hidden rounded-xl border border-zinc-200">
                    <div className="overflow-x-auto">
                      <table className="min-w-[1240px] divide-y divide-zinc-200 text-sm">
                  <thead className="bg-zinc-50">
                    <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                      <th className="px-4 py-3">{t("openrouterModels.table.state")}</th>
                      <th className="px-4 py-3">
                        <button type="button" className="hover:text-zinc-700" onClick={() => setSort("provider")}>
                          {t("openrouterModels.table.provider")}{sortMarker("provider")}
                        </button>
                      </th>
                      <th className="px-4 py-3">
                        <button type="button" className="hover:text-zinc-700" onClick={() => setSort("model")}>
                          {t("openrouterModels.table.model")}{sortMarker("model")}
                        </button>
                      </th>
                      <th className="px-4 py-3">{t("openrouterModels.table.reason")}</th>
                      <th className="px-4 py-3">
                        <button type="button" className="hover:text-zinc-700" onClick={() => setSort("context")}>
                          {t("openrouterModels.table.context")}{sortMarker("context")}
                        </button>
                      </th>
                      <th className="px-4 py-3">
                        <button type="button" className="hover:text-zinc-700" onClick={() => setSort("pricing")}>
                          {t("openrouterModels.table.pricing")}{sortMarker("pricing")}
                        </button>
                      </th>
                      <th className="px-4 py-3">
                        <button type="button" className="hover:text-zinc-700" onClick={() => setSort("params")}>
                          {t("openrouterModels.table.params")}{sortMarker("params")}
                        </button>
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-zinc-200 bg-white">
                    {unavailableModels.map((model) => {
                      const supported = parseStringArray(model.supported_parameters_json);
                      const pricing = pricingSummary(model);
                      const stateKey = model.availability === "removed" ? "openrouterModels.state.removed" : "openrouterModels.state.constrained";
                      const reasonKey = model.reason === "removed" ? "openrouterModels.reason.removed" : "openrouterModels.reason.structuredOutput";
                      const recentChangeKey = recentChangeLabelKey(model.recent_change);
                      return (
                        <tr key={model.model_id} className="cursor-pointer transition hover:bg-zinc-50" onClick={() => setSelectedModel(model)}>
                          <td className="whitespace-nowrap px-4 py-3 align-top">
                            <span className="rounded-full border border-zinc-200 bg-zinc-50 px-2 py-1 text-[11px] text-zinc-700">{t(stateKey)}</span>
                          </td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-zinc-700">{model.provider_slug || "—"}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top">
                            <div className="flex items-center gap-2 whitespace-nowrap">
                              <div className="whitespace-nowrap font-medium text-zinc-900">{model.display_name || model.model_id}</div>
                              {recentChangeKey ? (
                                <span className={`rounded-full border px-2 py-1 text-[11px] font-medium ${recentChangeClassName(model.recent_change)}`}>
                                  {t(recentChangeKey)}
                                </span>
                              ) : null}
                            </div>
                            <div className="mt-1 whitespace-nowrap text-xs text-zinc-500">{model.model_id}</div>
                          </td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-zinc-700">{t(reasonKey)}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-zinc-700">{model.context_length ? model.context_length.toLocaleString() : "—"}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-zinc-700">{pricing ?? "—"}</td>
                          <td className="whitespace-nowrap px-4 py-3 align-top text-zinc-700">
                            {supported.length > 0 ? (
                              <div className="flex flex-nowrap gap-1.5">
                                {supported.slice(0, 4).map((param) => (
                                  <span key={param} className="whitespace-nowrap rounded-full border border-zinc-200 bg-zinc-50 px-2 py-1 text-[11px] text-zinc-600">
                                    {param}
                                  </span>
                                ))}
                                {supported.length > 4 ? <span className="whitespace-nowrap rounded-full border border-zinc-200 bg-zinc-50 px-2 py-1 text-[11px] text-zinc-500">+{supported.length - 4}</span> : null}
                              </div>
                            ) : (
                              "—"
                            )}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                      </table>
                    </div>
                  </div>
                )}
              </TabPanel>
            </Tabs>
          </section>
        </div>
      )}

      {selectedModel ? (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 py-6"
          onClick={() => setSelectedModel(null)}
        >
          <div
            className="flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-2xl"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="flex items-start justify-between gap-4 border-b border-zinc-200 px-5 py-4">
              <div className="min-w-0">
                <div className="text-xs font-medium uppercase tracking-[0.12em] text-zinc-400">
                  {selectedModel.provider_slug || t("common.unknown")}
                </div>
                <h2 className="mt-1 break-words text-base font-semibold text-zinc-900">
                  {selectedModel.display_name || selectedModel.model_id}
                </h2>
                <div className="mt-1 flex items-center gap-2">
                  <p className="min-w-0 break-all text-xs text-zinc-500">{selectedModel.model_id}</p>
                  <button
                    type="button"
                    onClick={handleCopyModelId}
                    className="inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-zinc-200 bg-white text-zinc-500 hover:border-zinc-300 hover:text-zinc-800"
                    aria-label={t("openrouterModels.modal.copyModelId")}
                    title={t("openrouterModels.modal.copyModelId")}
                  >
                    <Copy className="size-3.5" aria-hidden="true" />
                  </button>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setSelectedModel(null)}
                className="inline-flex size-9 items-center justify-center rounded-lg border border-zinc-300 bg-white text-zinc-700 hover:border-zinc-400 hover:text-zinc-900"
                aria-label={t("common.close")}
              >
                <X className="size-4" aria-hidden="true" />
              </button>
            </div>
            <div className="overflow-auto px-5 py-4">
              <div className="grid gap-3 sm:grid-cols-3">
                <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-4">
                  <div className="text-xs font-medium text-zinc-500">{t("openrouterModels.context")}</div>
                  <div className="mt-1 text-sm font-semibold text-zinc-900">
                    {selectedModel.context_length ? selectedModel.context_length.toLocaleString() : "—"}
                  </div>
                </div>
                <div className="rounded-xl border border-zinc-200 bg-zinc-50 p-4 sm:col-span-2">
                  <div className="text-xs font-medium text-zinc-500">{t("openrouterModels.pricing")}</div>
                  <div className="mt-1 text-sm font-semibold text-zinc-900">{selectedPricing ?? "—"}</div>
                </div>
              </div>

              <section className="mt-4">
                <h3 className="text-sm font-semibold text-zinc-900">{t("openrouterModels.modal.descriptionJa")}</h3>
                <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-zinc-700">{selectedDescription}</p>
              </section>

              {selectedDescriptionEn && selectedDescriptionEn !== selectedDescription ? (
                <section className="mt-4">
                  <h3 className="text-sm font-semibold text-zinc-900">{t("openrouterModels.modal.descriptionEn")}</h3>
                  <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-zinc-700">{selectedDescriptionEn}</p>
                </section>
              ) : null}

              <section className="mt-4">
                <h3 className="text-sm font-semibold text-zinc-900">{t("openrouterModels.supportedParameters")}</h3>
                {selectedSupported.length > 0 ? (
                  <div className="mt-2 flex flex-wrap gap-2">
                    {selectedSupported.map((param) => (
                      <span key={param} className="rounded-full border border-zinc-200 bg-zinc-50 px-2 py-1 text-xs text-zinc-600">
                        {param}
                      </span>
                    ))}
                  </div>
                ) : (
                  <p className="mt-2 text-sm text-zinc-500">—</p>
                )}
              </section>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
