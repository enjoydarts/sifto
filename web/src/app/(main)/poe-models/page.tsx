"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Copy, Link2, RefreshCw, Search, X } from "lucide-react";
import { api, PoeModelSnapshot, PoeModelsResponse } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { useToast } from "@/components/toast-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";

function parseObject(raw: unknown): Record<string, unknown> {
  if (!raw) return {};
  if (typeof raw === "string") {
    try {
      return JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return {};
    }
  }
  if (typeof raw === "object") return raw as Record<string, unknown>;
  return {};
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

function pricingSummary(model: PoeModelSnapshot) {
  const pricing = parseObject(model.pricing_json);
  const input = formatPrice(pricing.prompt);
  const output = formatPrice(pricing.completion);
  const cacheRead = formatPrice(pricing.cache_read);
  const parts = [
    input ? `in ${input}` : null,
    output ? `out ${output}` : null,
    cacheRead ? `cache ${cacheRead}` : null,
  ].filter(Boolean);
  return parts.length > 0 ? `${parts.join(" / ")} / 1M tok` : null;
}

function syncProgressLabel(
  t: (key: string, fallback?: string) => string,
  run: PoeModelsResponse["latest_run"] | undefined | null,
) {
  if (!run || run.translation_target_count <= 0) return null;
  return t("poeModels.progressLabel")
    .replace("{{completed}}", String(run.translation_completed_count))
    .replace("{{total}}", String(run.translation_target_count));
}

function recentChangeClassName(change: "added" | "removed") {
  return change === "added"
    ? "bg-emerald-50 text-emerald-700 border-emerald-200"
    : "bg-red-50 text-red-700 border-red-200";
}

function limitSummaryModels(models: { model_id: string }[], limit = 5) {
  return {
    items: models.slice(0, limit).map((item) => item.model_id),
    remaining: Math.max(models.length - limit, 0),
  };
}

type PoeSection = "overview" | "available" | "removed";
type SortKey = "provider" | "model" | "context" | "pricing" | "transport";
type SortDirection = "asc" | "desc";

function transportLabel(model: PoeModelSnapshot) {
  return model.preferred_transport === "anthropic" ? "Anthropic compat" : "OpenAI compat";
}

function pricingScore(model: PoeModelSnapshot) {
  const pricing = parseObject(model.pricing_json);
  const prompt = typeof pricing.prompt === "number" ? pricing.prompt : typeof pricing.prompt === "string" ? Number(pricing.prompt) : NaN;
  const completion =
    typeof pricing.completion === "number" ? pricing.completion : typeof pricing.completion === "string" ? Number(pricing.completion) : NaN;
  const cacheRead = typeof pricing.cache_read === "number" ? pricing.cache_read : typeof pricing.cache_read === "string" ? Number(pricing.cache_read) : NaN;
  return [prompt, completion, cacheRead].reduce((sum, value) => (Number.isFinite(value) ? sum + value : sum), 0);
}

export default function PoeModelsPage() {
  const { t } = useI18n();
  const { showToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [providerFilter, setProviderFilter] = useState("");
  const [selectedModel, setSelectedModel] = useState<PoeModelSnapshot | null>(null);
  const [activeSection, setActiveSection] = useState<PoeSection>("overview");
  const [sortKey, setSortKey] = useState<SortKey>("provider");
  const [sortDirection, setSortDirection] = useState<SortDirection>("asc");
  const [data, setData] = useState<PoeModelsResponse | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const next = await api.getPoeModels();
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
    const timer = window.setInterval(load, 3000);
    return () => window.clearInterval(timer);
  }, [data?.latest_run, load]);

  const handleSync = useCallback(async () => {
    setSyncing(true);
    try {
      const next = await api.syncPoeModels();
      setData(next);
      setError(null);
      showToast(t("poeModels.syncCompleted"), "success");
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
      showToast(t("poeModels.toast.modelIdCopied"), "success");
    } catch {
      showToast(t("poeModels.toast.modelIdCopyFailed"), "error");
    }
  }, [selectedModel, showToast, t]);

  const providerOptions = useMemo(() => {
    const values = new Set<string>();
    for (const model of data?.models ?? []) {
      if (model.owned_by) values.add(model.owned_by);
    }
    return Array.from(values).sort((a, b) => a.localeCompare(b));
  }, [data?.models]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (data?.models ?? []).filter((model) => {
      if (providerFilter && model.owned_by !== providerFilter) return false;
      if (!q) return true;
      return [model.model_id, model.display_name, model.owned_by, model.description_en, model.description_ja]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
        .includes(q);
    });
  }, [data?.models, providerFilter, query]);

  const sorted = useMemo(() => {
    const models = [...filtered];
    models.sort((a, b) => {
      let result = 0;
      switch (sortKey) {
        case "provider":
          result = (a.owned_by || "").localeCompare(b.owned_by || "");
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
        case "transport":
          result = transportLabel(a).localeCompare(transportLabel(b));
          break;
      }
      if (result === 0) result = a.model_id.localeCompare(b.model_id);
      return sortDirection === "asc" ? result : -result;
    });
    return models;
  }, [filtered, sortDirection, sortKey]);

  const setSort = useCallback(
    (nextKey: SortKey) => {
      if (sortKey === nextKey) {
        setSortDirection(sortDirection === "asc" ? "desc" : "asc");
        return;
      }
      setSortKey(nextKey);
      setSortDirection(nextKey === "context" || nextKey === "pricing" ? "desc" : "asc");
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

  const latestSummary = data?.latest_change_summary ?? null;
  const latestRunLabel = data?.latest_run?.finished_at ? new Date(data.latest_run.finished_at).toLocaleString() : t("poeModels.latestRunEmpty");
  const fetchedCount = data?.latest_run?.fetched_count ?? data?.models.length ?? 0;
  const acceptedCount = data?.latest_run?.accepted_count ?? data?.models.length ?? 0;
  const translatedCount = data?.latest_run?.translation_completed_count ?? 0;
  const translationTargetCount = data?.latest_run?.translation_target_count ?? 0;
  const removedCount = latestSummary?.removed?.length ?? 0;
  const addedCount = latestSummary?.added?.length ?? 0;
  const selectedDescription = selectedModel ? selectedModel.description_ja ?? selectedModel.description_en ?? t("poeModels.descriptionFallback") : "";
  const selectedDescriptionEn = selectedModel?.description_en ?? "";

  if (loading) return <p className="text-sm text-zinc-500">{t("common.loading")}</p>;
  if (error) return <p className="text-sm text-red-500">{error}</p>;

  const renderFilters = (
    <section className="surface-editorial rounded-[24px] p-4">
      <div className="flex flex-col gap-3 md:flex-row">
        <label className="flex shrink-0 items-center gap-2 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2 text-sm text-[var(--color-editorial-ink-soft)]">
          <span className="whitespace-nowrap text-[11px] font-semibold uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">
            {t("poeModels.providerFilter")}
          </span>
          <select
            value={providerFilter}
            onChange={(e) => setProviderFilter(e.target.value)}
            className="min-w-0 bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none"
          >
            <option value="">{t("poeModels.providerAll")}</option>
            {providerOptions.map((provider) => (
              <option key={provider} value={provider}>
                {provider}
              </option>
            ))}
          </select>
        </label>
        <label className="flex min-w-0 flex-1 items-center gap-2 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-2">
          <Search className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("poeModels.search")}
            className="w-full bg-transparent text-sm text-[var(--color-editorial-ink)] outline-none placeholder:text-[var(--color-editorial-ink-faint)]"
          />
          {query ? (
            <button
              type="button"
              onClick={() => setQuery("")}
              className="shrink-0 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-[11px] font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
            >
              {t("common.clear")}
            </button>
          ) : null}
        </label>
      </div>
    </section>
  );

  return (
    <PageTransition>
      <div className="space-y-6 overflow-x-hidden">
        <PageHeader
          title={t("nav.poeModels")}
          titleIcon={Link2}
          description={t("poeModels.subtitle")}
          actions={
            <button
              type="button"
              onClick={handleSync}
              disabled={syncing}
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)] disabled:opacity-60"
            >
              <RefreshCw className={`size-4 ${syncing ? "animate-spin" : ""}`} aria-hidden="true" />
              {syncing ? t("poeModels.syncing") : t("poeModels.sync")}
            </button>
          }
        />

        <div className="grid gap-5 xl:grid-cols-[260px_minmax(0,1fr)]">
          <aside className="surface-editorial rounded-[24px] p-4">
            <div className="space-y-1">
              {[
                {
                  key: "overview" as const,
                  label: "Overview",
                  meta: `${t("poeModels.latestRun")} · ${latestRunLabel}`,
                },
                {
                  key: "available" as const,
                  label: t("poeModels.table.availableModels"),
                  meta: `${sorted.length} ${t("common.rows")}`,
                },
                {
                  key: "removed" as const,
                  label: t("poeModels.table.removedModels"),
                  meta: `${removedCount} ${t("common.rows")}`,
                },
              ].map((section) => (
                <button
                  key={section.key}
                  type="button"
                  onClick={() => setActiveSection(section.key)}
                  className={`relative block w-full rounded-[16px] px-4 py-[13px] text-left ${
                    activeSection === section.key
                      ? "bg-[linear-gradient(90deg,rgba(243,236,227,0.92),rgba(243,236,227,0.28)_78%,transparent)]"
                      : "bg-transparent"
                  }`}
                >
                  {activeSection === section.key ? (
                    <span className="absolute bottom-3 left-0 top-3 w-[3px] rounded-full bg-[var(--color-editorial-ink)]" />
                  ) : null}
                  <div className="text-[15px] font-semibold text-[var(--color-editorial-ink)]">{section.label}</div>
                  <div className="mt-1 text-[12px] leading-6 text-[var(--color-editorial-ink-faint)]">{section.meta}</div>
                </button>
              ))}
            </div>

            <div className="mt-5 border-t border-[var(--color-editorial-line)] pt-5">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                Status
              </div>
              <div className="mt-3 space-y-2 text-[12px] leading-6 text-[var(--color-editorial-ink-soft)]">
                <div>{t("poeModels.latestRun")} · {latestRunLabel}</div>
                <div>{t("poeModels.fetched")} · {fetchedCount}</div>
                <div>{t("poeModels.accepted")} · {acceptedCount}</div>
                <div>{syncProgressLabel(t, data?.latest_run) ?? t("poeModels.progressPreparing")}</div>
              </div>
            </div>
          </aside>

          <section className="min-w-0 space-y-4">
            {activeSection === "overview" ? (
              <>
                <section className="surface-editorial rounded-[28px] p-5">
                  <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                    <div>
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        Overview
                      </div>
                      <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                        最新の変化と運用状態
                      </h2>
                      <p className="mt-3 max-w-3xl text-[14px] leading-7 text-[var(--color-editorial-ink-soft)]">
                        最新同期、翻訳進捗、追加と削除の差分を先に把握してから、利用可能モデル一覧へ降ります。
                      </p>
                    </div>
                    <div className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3 text-sm text-[var(--color-editorial-ink-soft)]">
                      {data?.latest_run?.status === "running" && data.latest_run.trigger_type === "manual"
                        ? t("poeModels.progressRunning")
                        : latestRunLabel}
                    </div>
                  </div>

                  <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                    <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("poeModels.fetched")}
                      </div>
                      <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{fetchedCount}</div>
                    </div>
                    <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("poeModels.accepted")}
                      </div>
                      <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{acceptedCount}</div>
                    </div>
                    <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("poeModels.recentChange.added")}
                      </div>
                      <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">{addedCount}</div>
                    </div>
                    <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        Translated
                      </div>
                      <div className="mt-3 font-serif text-[1.8rem] leading-none text-[var(--color-editorial-ink)]">
                        {translatedCount} / {translationTargetCount}
                      </div>
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
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("poeModels.latestSummary.title")}
                    </div>
                    <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                      最新のモデル差分
                    </h3>
                    <div className="mt-4 space-y-3">
                      {[
                        {
                          key: "added" as const,
                          label: t("poeModels.recentChange.added"),
                          models: latestSummary?.added ?? [],
                        },
                        {
                          key: "removed" as const,
                          label: t("poeModels.recentChange.removed"),
                          models: latestSummary?.removed ?? [],
                        },
                      ].map((group) => {
                        const summary = limitSummaryModels(group.models);
                        return (
                          <div key={group.key} className={`rounded-[22px] border px-4 py-3 ${recentChangeClassName(group.key)}`}>
                            <div className="text-sm font-semibold">
                              {group.label} {group.models.length > 0 ? `(${group.models.length})` : ""}
                            </div>
                            {group.models.length > 0 ? (
                              <div className="mt-3 space-y-2 text-xs">
                                {summary.items.map((modelID) => (
                                  <div key={modelID} className="rounded-[14px] border border-current/15 bg-white/70 px-3 py-2">
                                    {modelID}
                                  </div>
                                ))}
                                {summary.remaining > 0 ? (
                                  <div className="rounded-[14px] border border-current/15 bg-white/70 px-3 py-2">
                                    +{summary.remaining}
                                  </div>
                                ) : null}
                              </div>
                            ) : (
                              <div className="mt-3 text-xs opacity-70">{t("poeModels.latestSummary.none")}</div>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  </section>

                  <section className="surface-editorial rounded-[28px] p-5">
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      Translation
                    </div>
                    <h3 className="mt-2 font-serif text-[1.6rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                      日本語説明の反映状況
                    </h3>
                    <div className="mt-4 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("poeModels.progressGlobal")}
                      </div>
                      <div className="mt-3 text-lg leading-none text-[var(--color-editorial-ink)]">
                        {translatedCount} / {translationTargetCount}
                      </div>
                      <div className="mt-3 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                        {syncProgressLabel(t, data?.latest_run) ?? t("poeModels.progressPreparing")}
                      </div>
                    </div>
                    <div className="mt-3 rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4 text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
                      {t("poeModels.latestRun")} · {latestRunLabel}
                      <br />
                      {t("poeModels.recentChange.removed")} · {removedCount}
                    </div>
                  </section>
                </section>
              </>
            ) : null}

            {activeSection === "available" ? (
              <>
                {renderFilters}
                <section className="surface-editorial rounded-[28px] p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                        {t("poeModels.providerGroup")}
                      </div>
                      <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                        {t("poeModels.table.availableModels")}
                      </h2>
                    </div>
                    <div className="text-xs text-[var(--color-editorial-ink-faint)]">
                      {sorted.length} {t("common.rows")}
                    </div>
                  </div>

                  {sorted.length === 0 ? (
                    <div className="mt-4 rounded-[22px] border border-dashed border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] px-4 py-6 text-sm text-[var(--color-editorial-ink-faint)]">
                      {t("poeModels.noAvailableModels")}
                    </div>
                  ) : (
                    <div className="mt-4 overflow-hidden rounded-[22px] border border-[var(--color-editorial-line)]">
                      <div className="overflow-x-auto">
                        <table className="min-w-[1080px] divide-y divide-[var(--color-editorial-line)] text-sm">
                          <thead className="bg-[var(--color-editorial-panel)]">
                            <tr className="text-left text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">
                              <th className="px-4 py-3">
                                <button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("provider")}>
                                  {t("poeModels.table.provider")}{sortMarker("provider")}
                                </button>
                              </th>
                              <th className="px-4 py-3">
                                <button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("model")}>
                                  {t("poeModels.table.model")}{sortMarker("model")}
                                </button>
                              </th>
                              <th className="px-4 py-3">
                                <button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("transport")}>
                                  {t("poeModels.table.transport")}{sortMarker("transport")}
                                </button>
                              </th>
                              <th className="px-4 py-3">
                                <button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("context")}>
                                  {t("poeModels.table.context")}{sortMarker("context")}
                                </button>
                              </th>
                              <th className="px-4 py-3">
                                <button type="button" className="hover:text-[var(--color-editorial-ink)]" onClick={() => setSort("pricing")}>
                                  {t("poeModels.table.pricing")}{sortMarker("pricing")}
                                </button>
                              </th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                            {sorted.map((model) => (
                              <tr
                                key={model.model_id}
                                className="cursor-pointer transition hover:bg-[var(--color-editorial-panel)]"
                                onClick={() => setSelectedModel(model)}
                              >
                                <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{model.owned_by || "—"}</td>
                                <td className="whitespace-nowrap px-4 py-3 align-top">
                                  <div className="whitespace-nowrap font-medium text-[var(--color-editorial-ink)]">{model.display_name || model.model_id}</div>
                                  <div className="mt-1 whitespace-nowrap text-xs text-[var(--color-editorial-ink-faint)]">{model.model_id}</div>
                                </td>
                                <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">{transportLabel(model)}</td>
                                <td className="whitespace-nowrap px-4 py-3 align-top text-[var(--color-editorial-ink-soft)]">
                                  {model.context_length ? model.context_length.toLocaleString() : "—"}
                                </td>
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
              <section className="surface-editorial rounded-[28px] p-5">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                      {t("poeModels.providerGroup")}
                    </div>
                    <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                      {t("poeModels.table.removedModels")}
                    </h2>
                  </div>
                  <div className="text-xs text-[var(--color-editorial-ink-faint)]">
                    {removedCount} {t("common.rows")}
                  </div>
                </div>

                {removedCount === 0 ? (
                  <div className="mt-4 rounded-[22px] border border-dashed border-[var(--color-editorial-line-strong)] bg-[var(--color-editorial-panel)] px-4 py-6 text-sm text-[var(--color-editorial-ink-faint)]">
                    {t("poeModels.noUnavailableModels")}
                  </div>
                ) : (
                  <div className="mt-4 space-y-3">
                    {(latestSummary?.removed ?? []).map((model) => (
                      <div key={model.model_id} className="rounded-[22px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
                        {model.model_id}
                      </div>
                    ))}
                  </div>
                )}
              </section>
            ) : null}
          </section>
        </div>

        {selectedModel ? (
          <div
            className="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(31,26,23,0.45)] px-4 py-6"
            onClick={() => setSelectedModel(null)}
          >
            <div
              className="flex max-h-[90vh] w-full max-w-3xl flex-col overflow-hidden rounded-[28px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] shadow-[var(--shadow-card)]"
              onClick={(event) => event.stopPropagation()}
            >
              <div className="flex items-start justify-between gap-4 border-b border-[var(--color-editorial-line)] bg-[linear-gradient(180deg,rgba(255,255,255,0.72),rgba(255,253,249,0.96))] px-5 py-4">
                <div className="min-w-0">
                  <div className="text-xs font-medium uppercase tracking-[0.12em] text-[var(--color-editorial-ink-faint)]">
                    {selectedModel.owned_by || t("common.unknown")}
                  </div>
                  <h2 className="mt-2 break-words font-serif text-[1.55rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                    {selectedModel.display_name || selectedModel.model_id}
                  </h2>
                  <div className="mt-1 flex items-center gap-2">
                    <p className="min-w-0 break-all text-xs text-[var(--color-editorial-ink-faint)]">{selectedModel.model_id}</p>
                    <button
                      type="button"
                      onClick={handleCopyModelId}
                      className="inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-faint)] hover:border-[var(--color-editorial-line-strong)] hover:text-[var(--color-editorial-ink)]"
                      aria-label={t("poeModels.modal.copyModelId")}
                      title={t("poeModels.modal.copyModelId")}
                    >
                      <Copy className="size-3.5" aria-hidden="true" />
                    </button>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => setSelectedModel(null)}
                  className="inline-flex size-9 items-center justify-center rounded-lg border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:border-[var(--color-editorial-line-strong)] hover:text-[var(--color-editorial-ink)]"
                  aria-label={t("common.close")}
                >
                  <X className="size-4" aria-hidden="true" />
                </button>
              </div>
              <div className="overflow-auto px-5 py-4">
                <div className="grid gap-3 sm:grid-cols-3">
                  <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                    <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("poeModels.context")}</div>
                    <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">
                      {selectedModel.context_length ? selectedModel.context_length.toLocaleString() : "—"}
                    </div>
                  </div>
                  <div className="rounded-[22px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                    <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("poeModels.table.transport")}</div>
                    <div className="mt-1 text-sm font-semibold text-[var(--color-editorial-ink)]">{transportLabel(selectedModel)}</div>
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
      </div>
    </PageTransition>
  );
}
