"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Brain, Filter, TableProperties } from "lucide-react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Scatter,
  ScatterChart,
  Tooltip,
  XAxis,
  YAxis,
  ZAxis,
} from "recharts";
import { api, ItemDetail, LLMExecutionCurrentMonthSummary, LLMUsageAnalysisSummary } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import ModelSelect, { type ModelOption } from "@/components/settings/model-select";

function fmtUSD(v: number) {
  return `$${v.toFixed(6)}`;
}

function fmtNum(v: number) {
  return new Intl.NumberFormat("ja-JP").format(v);
}

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
    default:
      return provider;
  }
}

function providerColor(provider: string) {
  switch (provider) {
    case "openai":
      return "#10b981";
    case "anthropic":
      return "#3b82f6";
    case "google":
      return "#f59e0b";
    case "groq":
      return "#8b5cf6";
    case "deepseek":
      return "#ef4444";
    case "alibaba":
      return "#14b8a6";
    case "mistral":
      return "#fb7185";
    case "xai":
      return "#818cf8";
    default:
      return "#71717a";
  }
}

type AnalysisRow = LLMUsageAnalysisSummary & {
  avg_input_tokens_per_call: number;
  avg_output_tokens_per_call: number;
  avg_cost_per_call_usd: number;
  cost_share_pct: number;
};

type ExecutionRow = LLMExecutionCurrentMonthSummary;

type RankedModelRow = {
  provider: string;
  model: string;
  purpose: string;
  calls: number;
  input_tokens: number;
  output_tokens: number;
  estimated_cost_usd: number;
  avg_input_tokens_per_call: number;
  avg_output_tokens_per_call: number;
  avg_cost_per_call_usd: number;
  total_tokens_per_call: number;
  failure_rate_pct: number | null;
  retry_rate_pct: number | null;
  empty_rate_pct: number | null;
  attempts: number;
  vs_cheapest_pct: number;
  vs_median_pct: number;
};

export default function LLMAnalysisPage() {
  const { t } = useI18n();
  const [days, setDays] = useState<"14" | "30" | "90" | "180">("30");
  const [providerFilter, setProviderFilter] = useState("all");
  const [purposeFilter, setPurposeFilter] = useState("all");
  const [scatterPurpose, setScatterPurpose] = useState("all");
  const [rankingPurpose, setRankingPurpose] = useState("all");
  const [modelQuery, setModelQuery] = useState("");
  const [selectedModelKey, setSelectedModelKey] = useState<string | null>(null);
  const [sortKey, setSortKey] = useState<string>("estimated_cost_usd");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rows, setRows] = useState<LLMUsageAnalysisSummary[]>([]);
  const [executionRows, setExecutionRows] = useState<ExecutionRow[]>([]);
  const [qaLoading, setQaLoading] = useState(false);
  const [qaSamples, setQaSamples] = useState<ItemDetail[]>([]);

  const load = useCallback(async (selectedDays: number) => {
    setLoading(true);
    try {
      const [nextRows, nextExecutionRows] = await Promise.all([
        api.getLLMUsageAnalysis({ days: selectedDays }),
        api.getLLMExecutionCurrentMonthSummary(),
      ]);
      setRows(nextRows ?? []);
      setExecutionRows(nextExecutionRows ?? []);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(Number(days));
  }, [days, load]);

  const providers = useMemo(
    () => Array.from(new Set(rows.map((row) => row.provider))).sort((a, b) => a.localeCompare(b)),
    [rows]
  );
  const purposes = useMemo(
    () => Array.from(new Set(rows.map((row) => row.purpose))).sort((a, b) => a.localeCompare(b)),
    [rows]
  );

  const enrichedRows = useMemo<AnalysisRow[]>(() => {
    const totalCost = rows.reduce((acc, row) => acc + row.estimated_cost_usd, 0);
    return rows.map((row) => ({
      ...row,
      avg_input_tokens_per_call: row.calls > 0 ? row.input_tokens / row.calls : 0,
      avg_output_tokens_per_call: row.calls > 0 ? row.output_tokens / row.calls : 0,
      avg_cost_per_call_usd: row.calls > 0 ? row.estimated_cost_usd / row.calls : 0,
      cost_share_pct: totalCost > 0 ? (row.estimated_cost_usd / totalCost) * 100 : 0,
    }));
  }, [rows]);

  const filteredRows = useMemo(() => {
    return enrichedRows
      .filter((row) => (providerFilter === "all" ? true : row.provider === providerFilter))
      .filter((row) => (purposeFilter === "all" ? true : row.purpose === purposeFilter))
      .filter((row) => (modelQuery === "" ? true : row.model === modelQuery));
  }, [enrichedRows, modelQuery, providerFilter, purposeFilter]);

  const modelOptions = useMemo<ModelOption[]>(() => {
    const seen = new Set<string>();
    return enrichedRows
      .filter((row) => (providerFilter === "all" ? true : row.provider === providerFilter))
      .filter((row) => (purposeFilter === "all" ? true : row.purpose === purposeFilter))
      .filter((row) => {
        const key = `${row.provider}:${row.model}`;
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      })
      .map((row) => ({
        value: row.model,
        label: row.model,
        provider: providerLabel(row.provider),
        note: row.purpose,
      }))
      .sort((a, b) => {
        const providerCmp = String(a.provider ?? "").localeCompare(String(b.provider ?? ""));
        if (providerCmp !== 0) return providerCmp;
        return a.label.localeCompare(b.label);
      });
  }, [enrichedRows, providerFilter, purposeFilter]);

  const sortedRows = useMemo(() => {
    const dir = sortDir === "asc" ? 1 : -1;
    return [...filteredRows].sort((a, b) => {
      const aVal = a[sortKey as keyof AnalysisRow];
      const bVal = b[sortKey as keyof AnalysisRow];
      let cmp = 0;
      if (typeof aVal === "number" && typeof bVal === "number") cmp = aVal - bVal;
      else cmp = String(aVal ?? "").localeCompare(String(bVal ?? ""));
      if (cmp !== 0) return cmp * dir;
      if (a.provider !== b.provider) return a.provider.localeCompare(b.provider);
      if (a.model !== b.model) return a.model.localeCompare(b.model);
      return a.purpose.localeCompare(b.purpose);
    });
  }, [filteredRows, sortDir, sortKey]);

  const totals = useMemo(() => {
    return sortedRows.reduce(
      (acc, row) => {
        acc.calls += row.calls;
        acc.cost += row.estimated_cost_usd;
        acc.input += row.input_tokens;
        acc.output += row.output_tokens;
        return acc;
      },
      { calls: 0, cost: 0, input: 0, output: 0 }
    );
  }, [sortedRows]);

  const providerCostRows = useMemo(() => {
    const totalsByProvider = new Map<string, { provider: string; cost: number; calls: number }>();
    for (const row of sortedRows) {
      const cur = totalsByProvider.get(row.provider) ?? { provider: row.provider, cost: 0, calls: 0 };
      cur.cost += row.estimated_cost_usd;
      cur.calls += row.calls;
      totalsByProvider.set(row.provider, cur);
    }
    return Array.from(totalsByProvider.values())
      .sort((a, b) => b.cost - a.cost)
      .slice(0, 8)
      .map((row) => ({
        ...row,
        label: providerLabel(row.provider),
      }));
  }, [sortedRows]);

  const tokenIntensityRows = useMemo(() => {
    return [...sortedRows]
      .filter((row) => row.calls > 0)
      .sort((a, b) => b.avg_input_tokens_per_call - a.avg_input_tokens_per_call)
      .slice(0, 10)
      .map((row) => ({
        key: `${row.provider}:${row.model}:${row.purpose}`,
        label: `${providerLabel(row.provider)} / ${row.model}`,
        purpose: row.purpose,
        avgInput: Math.round(row.avg_input_tokens_per_call),
        avgOutput: Math.round(row.avg_output_tokens_per_call),
      }));
  }, [sortedRows]);

  const scatterRows = useMemo(() => {
    const grouped = new Map<
      string,
      AnalysisRow & {
        pricingSources: string[];
      }
    >();
    for (const row of sortedRows) {
      if (scatterPurpose !== "all" && row.purpose !== scatterPurpose) continue;
      if (row.calls <= 0) continue;
      const key = `${row.provider}:${row.model}:${row.purpose}`;
      const existing = grouped.get(key);
      if (existing) {
        existing.calls += row.calls;
        existing.input_tokens += row.input_tokens;
        existing.output_tokens += row.output_tokens;
        existing.cache_creation_input_tokens += row.cache_creation_input_tokens;
        existing.cache_read_input_tokens += row.cache_read_input_tokens;
        existing.estimated_cost_usd += row.estimated_cost_usd;
        if (!existing.pricingSources.includes(row.pricing_source)) {
          existing.pricingSources.push(row.pricing_source);
        }
        continue;
      }
      grouped.set(key, {
        ...row,
        pricingSources: [row.pricing_source],
      });
    }

    return Array.from(grouped.values())
      .filter((row) => (scatterPurpose === "all" ? true : row.purpose === scatterPurpose))
      .map((row) => ({
        ...row,
        avg_input_tokens_per_call: row.calls > 0 ? row.input_tokens / row.calls : 0,
        avg_output_tokens_per_call: row.calls > 0 ? row.output_tokens / row.calls : 0,
        avg_cost_per_call_usd: row.calls > 0 ? row.estimated_cost_usd / row.calls : 0,
        totalTokensPerCall: Math.round(row.avg_input_tokens_per_call + row.avg_output_tokens_per_call),
        avgCostPerCall: row.avg_cost_per_call_usd,
        bubbleSize: Math.max(6, Math.min(22, 6 + Math.sqrt(row.calls))),
        label: `${providerLabel(row.provider)} / ${row.model}`,
        pricingLabel: row.pricingSources.length === 1 ? row.pricingSources[0] : t("llmAnalysis.mixedPricingSources"),
      }))
      .sort((a, b) => b.calls - a.calls)
      .slice(0, 36);
  }, [scatterPurpose, sortedRows, t]);

  useEffect(() => {
    if (purposeFilter !== "all") {
      setScatterPurpose(purposeFilter);
      setRankingPurpose(purposeFilter);
    }
  }, [purposeFilter]);

  useEffect(() => {
    if (rankingPurpose !== "all") return;
    if (purposeFilter !== "all") return;
    if (purposes.length === 0) return;
    setRankingPurpose(purposes[0]);
  }, [purposeFilter, purposes, rankingPurpose]);

  useEffect(() => {
    setSelectedModelKey(null);
  }, [rankingPurpose]);

  const executionMap = useMemo(() => {
    const map = new Map<
      string,
      {
        attempts: number;
        failures: number;
        retries: number;
        emptyResponses: number;
      }
    >();
    for (const row of executionRows) {
      const key = `${row.provider}:${row.model}:${row.purpose}`;
      const current = map.get(key) ?? { attempts: 0, failures: 0, retries: 0, emptyResponses: 0 };
      current.attempts += row.attempts;
      current.failures += row.failures;
      current.retries += row.retries;
      current.emptyResponses += row.empty_responses;
      map.set(key, current);
    }
    return map;
  }, [executionRows]);

  const rankingRows = useMemo<RankedModelRow[]>(() => {
    const grouped = new Map<
      string,
      {
        provider: string;
        model: string;
        purpose: string;
        calls: number;
        input_tokens: number;
        output_tokens: number;
        estimated_cost_usd: number;
      }
    >();
    for (const row of enrichedRows) {
      if (providerFilter !== "all" && row.provider !== providerFilter) continue;
      if (modelQuery !== "" && row.model !== modelQuery) continue;
      const effectivePurpose = rankingPurpose === "all" ? row.purpose : rankingPurpose;
      if (row.purpose !== effectivePurpose) continue;
      const key = `${row.provider}:${row.model}:${row.purpose}`;
      const current = grouped.get(key) ?? {
        provider: row.provider,
        model: row.model,
        purpose: row.purpose,
        calls: 0,
        input_tokens: 0,
        output_tokens: 0,
        estimated_cost_usd: 0,
      };
      current.calls += row.calls;
      current.input_tokens += row.input_tokens;
      current.output_tokens += row.output_tokens;
      current.estimated_cost_usd += row.estimated_cost_usd;
      grouped.set(key, current);
    }

    return Array.from(grouped.values())
      .map((row) => {
        const exec = executionMap.get(`${row.provider}:${row.model}:${row.purpose}`);
        const attempts = exec?.attempts ?? 0;
        const failures = exec?.failures ?? 0;
        const retries = exec?.retries ?? 0;
        const emptyResponses = exec?.emptyResponses ?? 0;
        return {
          ...row,
          avg_input_tokens_per_call: row.calls > 0 ? row.input_tokens / row.calls : 0,
          avg_output_tokens_per_call: row.calls > 0 ? row.output_tokens / row.calls : 0,
          avg_cost_per_call_usd: row.calls > 0 ? row.estimated_cost_usd / row.calls : 0,
          total_tokens_per_call: row.calls > 0 ? (row.input_tokens + row.output_tokens) / row.calls : 0,
          failure_rate_pct: attempts > 0 ? (failures / attempts) * 100 : null,
          retry_rate_pct: attempts > 0 ? (retries / attempts) * 100 : null,
          empty_rate_pct: attempts > 0 ? (emptyResponses / attempts) * 100 : null,
          attempts,
          vs_cheapest_pct: 0,
          vs_median_pct: 0,
        };
      })
      .filter((row) => row.calls >= 3)
      .sort((a, b) => {
        if (a.avg_cost_per_call_usd !== b.avg_cost_per_call_usd) return a.avg_cost_per_call_usd - b.avg_cost_per_call_usd;
        if (a.total_tokens_per_call !== b.total_tokens_per_call) return a.total_tokens_per_call - b.total_tokens_per_call;
        return b.calls - a.calls;
      })
      .map((row, _, arr) => {
        const cheapest = arr[0]?.avg_cost_per_call_usd ?? 0;
        const median = arr.length > 0 ? arr[Math.floor(arr.length / 2)]?.avg_cost_per_call_usd ?? 0 : 0;
        return {
          ...row,
          vs_cheapest_pct: cheapest > 0 ? ((row.avg_cost_per_call_usd - cheapest) / cheapest) * 100 : 0,
          vs_median_pct: median > 0 ? ((row.avg_cost_per_call_usd - median) / median) * 100 : 0,
        };
      });
  }, [enrichedRows, executionMap, modelQuery, providerFilter, rankingPurpose]);

  const bestCostModel = useMemo(() => rankingRows[0] ?? null, [rankingRows]);
  const bestTokenModel = useMemo(
    () => [...rankingRows].sort((a, b) => a.total_tokens_per_call - b.total_tokens_per_call || b.calls - a.calls)[0] ?? null,
    [rankingRows]
  );
  const bestQualityModel = useMemo(
    () =>
      [...rankingRows]
        .filter((row) => row.failure_rate_pct != null)
        .sort((a, b) => {
          const failureDiff = (a.failure_rate_pct ?? 999) - (b.failure_rate_pct ?? 999);
          if (failureDiff !== 0) return failureDiff;
          const retryDiff = (a.retry_rate_pct ?? 999) - (b.retry_rate_pct ?? 999);
          if (retryDiff !== 0) return retryDiff;
          return b.attempts - a.attempts;
        })[0] ?? null,
    [rankingRows]
  );
  const rankingMedianCost = useMemo(
    () => (rankingRows.length > 0 ? rankingRows[Math.floor(rankingRows.length / 2)]?.avg_cost_per_call_usd ?? null : null),
    [rankingRows]
  );
  const rankingPurposeLabel = rankingPurpose === "all" ? t("llmAnalysis.allPurposes") : rankingPurpose;
  const rankingModelOptions = useMemo<ModelOption[]>(
    () =>
      rankingRows.map((row) => ({
        value: `${row.provider}:${row.model}:${row.purpose}`,
        label: row.model,
        provider: providerLabel(row.provider),
        note: `${row.purpose} · ${fmtUSD(row.avg_cost_per_call_usd)}`,
      })),
    [rankingRows]
  );
  const selectedRankingRow = useMemo(() => {
    if (!selectedModelKey) return null;
    return rankingRows.find((row) => `${row.provider}:${row.model}:${row.purpose}` === selectedModelKey) ?? null;
  }, [rankingRows, selectedModelKey]);

  const topCostInsight = useMemo(() => sortedRows[0] ?? null, [sortedRows]);
  const topInputInsight = useMemo(
    () =>
      [...sortedRows]
        .filter((row) => row.calls > 0)
        .sort((a, b) => b.avg_input_tokens_per_call - a.avg_input_tokens_per_call)[0] ?? null,
    [sortedRows]
  );
  const topPurposeInsight = useMemo(() => {
    const totalsByPurpose = new Map<string, { purpose: string; cost: number; calls: number }>();
    for (const row of sortedRows) {
      const cur = totalsByPurpose.get(row.purpose) ?? { purpose: row.purpose, cost: 0, calls: 0 };
      cur.cost += row.estimated_cost_usd;
      cur.calls += row.calls;
      totalsByPurpose.set(row.purpose, cur);
    }
    return Array.from(totalsByPurpose.values()).sort((a, b) => b.cost - a.cost)[0] ?? null;
  }, [sortedRows]);

  const toggleSort = (key: string) => {
    if (sortKey === key) {
      setSortDir((prev) => (prev === "asc" ? "desc" : "asc"));
      return;
    }
    setSortKey(key);
    setSortDir(key === "provider" || key === "model" || key === "purpose" || key === "pricing_source" ? "asc" : "desc");
  };

  const sortMark = (key: string) => (sortKey === key ? <span className="ml-1 text-zinc-400">{sortDir === "asc" ? "↑" : "↓"}</span> : null);

  const applyRowFilter = (row: Pick<AnalysisRow, "provider" | "purpose" | "model">) => {
    setProviderFilter(row.provider);
    setPurposeFilter(row.purpose);
    setModelQuery(row.model);
    setSelectedModelKey(`${row.provider}:${row.model}:${row.purpose}`);
  };

  const applyPurposeFilter = (purpose: string) => {
    setPurposeFilter(purpose);
    setProviderFilter("all");
    setModelQuery("");
    setSelectedModelKey(null);
  };

  const clearFilters = () => {
    setProviderFilter("all");
    setPurposeFilter("all");
    setModelQuery("");
    setSelectedModelKey(null);
  };

  const hasActiveDrilldown = providerFilter !== "all" || purposeFilter !== "all" || modelQuery.trim() !== "";
  const hasScatterFilter = hasActiveDrilldown || scatterPurpose !== "all";

  const clearScatterFilters = () => {
    clearFilters();
    setScatterPurpose("all");
  };

  useEffect(() => {
    let cancelled = false;
    async function loadQASamples() {
      if (!selectedRankingRow) {
        setQaSamples([]);
        setQaLoading(false);
        return;
      }
      setQaLoading(true);
      try {
        const logs = await api.getLLMUsage({ limit: 300 });
        const matchedItemIDs = Array.from(
          new Set(
            (logs ?? [])
              .filter((log) => log.item_id)
              .filter((log) => log.provider === selectedRankingRow.provider)
              .filter((log) => log.model === selectedRankingRow.model)
              .filter((log) => log.purpose === selectedRankingRow.purpose)
              .map((log) => log.item_id as string)
          )
        ).slice(0, 3);

        if (matchedItemIDs.length === 0) {
          if (!cancelled) setQaSamples([]);
          return;
        }

        const details = await Promise.all(matchedItemIDs.map((itemID) => api.getItem(itemID).catch(() => null)));
        if (!cancelled) {
          setQaSamples(details.filter((item): item is ItemDetail => Boolean(item)));
        }
      } finally {
        if (!cancelled) setQaLoading(false);
      }
    }
    void loadQASamples();
    return () => {
      cancelled = true;
    };
  }, [selectedRankingRow]);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            <TableProperties className="size-6 text-zinc-500" aria-hidden="true" />
            <span>{t("llmAnalysis.title")}</span>
          </h1>
          <p className="mt-1 text-sm text-zinc-500">{t("llmAnalysis.subtitle")}</p>
        </div>
        <Link href="/llm-usage" className="inline-flex items-center gap-2 rounded-lg border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50">
          <Brain className="size-4" aria-hidden="true" />
          <span>{t("llmAnalysis.backToUsage")}</span>
        </Link>
      </div>

      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-zinc-800">
          <Filter className="size-4 text-zinc-500" aria-hidden="true" />
          <span>{t("llmAnalysis.filters")}</span>
        </div>
        <div className="grid gap-3 md:grid-cols-4">
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-zinc-600">{t("llm.days")}</span>
            <select value={days} onChange={(e) => setDays(e.target.value as typeof days)} className="w-full rounded border border-zinc-300 bg-white px-3 py-2 text-sm">
              <option value="14">14{t("llm.daysSuffix")}</option>
              <option value="30">30{t("llm.daysSuffix")}</option>
              <option value="90">90{t("llm.daysSuffix")}</option>
              <option value="180">180{t("llm.daysSuffix")}</option>
            </select>
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-zinc-600">{t("llmAnalysis.provider")}</span>
            <select value={providerFilter} onChange={(e) => setProviderFilter(e.target.value)} className="w-full rounded border border-zinc-300 bg-white px-3 py-2 text-sm">
              <option value="all">{t("llmAnalysis.all")}</option>
              {providers.map((provider) => (
                <option key={provider} value={provider}>{providerLabel(provider)}</option>
              ))}
            </select>
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-zinc-600">{t("llmAnalysis.purpose")}</span>
            <select value={purposeFilter} onChange={(e) => setPurposeFilter(e.target.value)} className="w-full rounded border border-zinc-300 bg-white px-3 py-2 text-sm">
              <option value="all">{t("llmAnalysis.all")}</option>
              {purposes.map((purpose) => (
                <option key={purpose} value={purpose}>{purpose}</option>
              ))}
            </select>
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-zinc-600">{t("llmAnalysis.modelSearch")}</span>
            <ModelSelect
              label={t("llmAnalysis.modelSearch")}
              value={modelQuery}
              onChange={setModelQuery}
              options={modelOptions}
              showMeta={false}
              hideLabel
              labels={{
                defaultOption: t("llmAnalysis.modelSearchDefault"),
                searchPlaceholder: t("settings.modelSelect.searchPlaceholder"),
                noResults: t("settings.modelSelect.noResults"),
              }}
            />
          </label>
        </div>
        {hasActiveDrilldown ? (
          <div className="mt-3 flex items-center justify-between gap-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-2">
            <div className="text-xs text-zinc-600">{t("llmAnalysis.drilldownActive")}</div>
            <button
              type="button"
              onClick={clearFilters}
              className="rounded-md border border-zinc-300 bg-white px-2.5 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-50"
            >
              {t("llmAnalysis.clearDrilldown")}
            </button>
          </div>
        ) : null}
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <SectionTitle
          eyebrow={t("llmAnalysis.section.overview")}
          title={t("llmAnalysis.section.overviewTitle")}
          description={t("llmAnalysis.section.overviewDescription")}
        />
        <div className="grid gap-3 md:grid-cols-4">
          <Metric label={t("llm.totalCost")} value={fmtUSD(totals.cost)} />
          <Metric label={t("llm.totalCalls")} value={fmtNum(totals.calls)} />
          <Metric label={t("llm.input")} value={fmtNum(totals.input)} />
          <Metric label={t("llm.output")} value={fmtNum(totals.output)} />
        </div>
        <div className="mt-4 grid gap-3 xl:grid-cols-3">
          <InsightCard
            title={t("llmAnalysis.insight.topCost")}
            body={
              topCostInsight
                ? `${providerLabel(topCostInsight.provider)} / ${topCostInsight.model} / ${topCostInsight.purpose}`
                : "—"
            }
            meta={
              topCostInsight
                ? `${fmtUSD(topCostInsight.estimated_cost_usd)} · ${fmtNum(topCostInsight.calls)} calls`
                : "—"
            }
            onClick={topCostInsight ? () => applyRowFilter(topCostInsight) : undefined}
          />
          <InsightCard
            title={t("llmAnalysis.insight.topInput")}
            body={
              topInputInsight
                ? `${providerLabel(topInputInsight.provider)} / ${topInputInsight.model} / ${topInputInsight.purpose}`
                : "—"
            }
            meta={
              topInputInsight
                ? `${fmtNum(Math.round(topInputInsight.avg_input_tokens_per_call))} avg in/call`
                : "—"
            }
            onClick={topInputInsight ? () => applyRowFilter(topInputInsight) : undefined}
          />
          <InsightCard
            title={t("llmAnalysis.insight.topPurpose")}
            body={topPurposeInsight ? topPurposeInsight.purpose : "—"}
            meta={
              topPurposeInsight
                ? `${fmtUSD(topPurposeInsight.cost)} · ${fmtNum(topPurposeInsight.calls)} calls`
                : "—"
            }
            onClick={topPurposeInsight ? () => applyPurposeFilter(topPurposeInsight.purpose) : undefined}
          />
        </div>
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <SectionTitle
          eyebrow={t("llmAnalysis.section.charts")}
          title={t("llmAnalysis.section.chartsTitle")}
          description={t("llmAnalysis.section.chartsDescription")}
        />
        <section className="rounded-xl border border-zinc-200 bg-zinc-50/60 p-4">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="text-sm font-semibold text-zinc-800">{t("llmAnalysis.efficiencyScatter")}</h2>
            <p className="mt-1 text-xs text-zinc-500">{t("llmAnalysis.efficiencyScatterHelp")}</p>
          </div>
          <div className="flex items-center gap-2">
            {hasScatterFilter ? (
              <button
                type="button"
                onClick={clearScatterFilters}
                className="rounded-md border border-zinc-300 bg-white px-2.5 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-50"
              >
                {t("llmAnalysis.clearDrilldown")}
              </button>
            ) : null}
            <span className="text-xs text-zinc-400">{scatterRows.length} rows</span>
          </div>
        </div>
        <div className="mb-3 flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => setScatterPurpose("all")}
            className={`rounded-full border px-3 py-1 text-xs font-medium transition ${
              scatterPurpose === "all"
                ? "border-zinc-900 bg-zinc-900 text-white"
                : "border-zinc-200 bg-white text-zinc-600 hover:border-zinc-300 hover:text-zinc-900"
            }`}
          >
            {t("llmAnalysis.all")}
          </button>
          {purposes.map((purpose) => (
            <button
              key={purpose}
              type="button"
              onClick={() => setScatterPurpose(purpose)}
              className={`rounded-full border px-3 py-1 text-xs font-medium transition ${
                scatterPurpose === purpose
                  ? "border-zinc-900 bg-zinc-900 text-white"
                  : "border-zinc-200 bg-white text-zinc-600 hover:border-zinc-300 hover:text-zinc-900"
              }`}
            >
              {purpose}
            </button>
          ))}
        </div>
        {scatterRows.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("llm.noSummary")}</p>
        ) : (
          <div className="h-80 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <ScatterChart margin={{ top: 12, right: 20, left: 8, bottom: 12 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" />
                <XAxis
                  type="number"
                  dataKey="totalTokensPerCall"
                  name="Tokens / call"
                  tick={{ fontSize: 12, fill: "#71717a" }}
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(v) => fmtNum(Number(v))}
                />
                <YAxis
                  type="number"
                  dataKey="avgCostPerCall"
                  name="Cost / call"
                  tick={{ fontSize: 12, fill: "#71717a" }}
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(v) => fmtUSD(Number(v))}
                />
                <ZAxis type="number" dataKey="bubbleSize" range={[80, 460]} />
                <Tooltip
                  cursor={{ strokeDasharray: "4 4" }}
                  formatter={(value: number | string | undefined, name?: string) => {
                    if (name === "Cost / call") return [fmtUSD(Number(value ?? 0)), name];
                    if (name === "Tokens / call") return [fmtNum(Number(value ?? 0)), name];
                    if (name === "Calls") return [fmtNum(Number(value ?? 0)), name];
                    return [String(value ?? ""), name ?? ""];
                  }}
                  labelFormatter={(_, payload) => {
                    const row = payload?.[0]?.payload as (typeof scatterRows)[number] | undefined;
                    if (!row) return "";
                    return `${row.label} (${row.purpose})`;
                  }}
                  content={({ active, payload }) => {
                    const row = payload?.[0]?.payload as (typeof scatterRows)[number] | undefined;
                    if (!active || !row) return null;
                    return (
                      <div className="rounded-lg border border-zinc-200 bg-white px-3 py-2 shadow-lg">
                        <div className="text-xs font-semibold text-zinc-900">{providerLabel(row.provider)} / {row.model}</div>
                        <div className="mt-1 text-xs text-zinc-500">{row.purpose}</div>
                        <div className="mt-2 grid gap-1 text-xs text-zinc-700">
                          <div>{t("llm.totalCalls")}: {fmtNum(row.calls)}</div>
                          <div>tokens/call: {fmtNum(row.totalTokensPerCall)}</div>
                          <div>avg cost/call: {fmtUSD(row.avgCostPerCall)}</div>
                          <div>pricing: {row.pricingLabel}</div>
                        </div>
                      </div>
                    );
                  }}
                  contentStyle={{ borderRadius: 10, borderColor: "#e4e4e7" }}
                />
                <Scatter
                  data={scatterRows}
                  name="Calls"
                  shape={(props: {
                    cx?: number;
                    cy?: number;
                    payload?: (typeof scatterRows)[number];
                  }) => {
                    const row = props.payload;
                    if (props.cx == null || props.cy == null || !row) return null;
                    return (
                      <circle
                        cx={props.cx}
                        cy={props.cy}
                        r={row.bubbleSize}
                        fill={providerColor(row.provider)}
                        fillOpacity={0.72}
                        stroke="#ffffff"
                        strokeWidth={2}
                        className="cursor-pointer transition-opacity hover:opacity-100"
                        onClick={() => applyRowFilter(row)}
                      />
                    );
                  }}
                />
              </ScatterChart>
            </ResponsiveContainer>
          </div>
        )}
        </section>
        <div className="mt-4 grid gap-4 xl:grid-cols-2">
        <section className="rounded-xl border border-zinc-200 bg-zinc-50/60 p-4">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-zinc-800">{t("llmAnalysis.providerCostChart")}</h2>
            <span className="text-xs text-zinc-400">{providerCostRows.length} providers</span>
          </div>
          {providerCostRows.length === 0 ? (
            <p className="text-sm text-zinc-400">{t("llm.noSummary")}</p>
          ) : (
            <div className="h-72 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={providerCostRows} margin={{ top: 8, right: 16, left: 8, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" vertical={false} />
                  <XAxis dataKey="label" tick={{ fontSize: 12, fill: "#71717a" }} tickLine={false} axisLine={false} />
                  <YAxis tick={{ fontSize: 12, fill: "#71717a" }} tickLine={false} axisLine={false} tickFormatter={(v) => fmtUSD(Number(v))} />
                  <Tooltip
                    formatter={(value: number | string | undefined, name?: string) => [
                      name === "calls" ? fmtNum(Number(value ?? 0)) : fmtUSD(Number(value ?? 0)),
                      name ?? "",
                    ]}
                    contentStyle={{ borderRadius: 10, borderColor: "#e4e4e7" }}
                  />
                  <Bar dataKey="cost" name="Cost (USD)" radius={[6, 6, 0, 0]}>
                    {providerCostRows.map((row) => (
                      <Cell key={row.provider} fill={providerColor(row.provider)} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}
        </section>

        <section className="rounded-xl border border-zinc-200 bg-zinc-50/60 p-4">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-zinc-800">{t("llmAnalysis.tokenIntensityChart")}</h2>
            <span className="text-xs text-zinc-400">{tokenIntensityRows.length} rows</span>
          </div>
          {tokenIntensityRows.length === 0 ? (
            <p className="text-sm text-zinc-400">{t("llm.noSummary")}</p>
          ) : (
            <div className="h-72 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={tokenIntensityRows} layout="vertical" margin={{ top: 8, right: 16, left: 8, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" horizontal={true} vertical={false} />
                  <XAxis type="number" tick={{ fontSize: 12, fill: "#71717a" }} tickLine={false} axisLine={false} />
                  <YAxis type="category" dataKey="label" width={240} tick={{ fontSize: 12, fill: "#3f3f46" }} tickLine={false} axisLine={false} />
                  <Tooltip
                    formatter={(value: number | string | undefined, name?: string) => [fmtNum(Number(value ?? 0)), name ?? ""]}
                    labelFormatter={(_, payload) => {
                      const row = payload?.[0]?.payload as { label?: string; purpose?: string } | undefined;
                      if (!row) return "";
                      return `${row.label} (${row.purpose ?? ""})`;
                    }}
                    contentStyle={{ borderRadius: 10, borderColor: "#e4e4e7" }}
                  />
                  <Bar dataKey="avgInput" name="Avg input/call" fill="#18181b" radius={[0, 4, 4, 0]} />
                  <Bar dataKey="avgOutput" name="Avg output/call" fill="#60a5fa" radius={[0, 4, 4, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}
        </section>
        </div>
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <SectionTitle
            eyebrow={t("llmAnalysis.section.recommend")}
            title={t("llmAnalysis.rankingTitle")}
            description={t("llmAnalysis.rankingHelp")}
            compact
          />
          <span className="rounded-full border border-zinc-200 bg-zinc-50 px-2.5 py-1 text-xs text-zinc-500">{t("llmAnalysis.rankingThreshold")}</span>
        </div>
        <div className="mb-4 flex flex-wrap gap-2">
          {purposes.map((purpose) => (
            <button
              key={purpose}
              type="button"
              onClick={() => setRankingPurpose(purpose)}
              className={`rounded-full border px-3 py-1 text-xs font-medium transition ${
                rankingPurpose === purpose
                  ? "border-zinc-900 bg-zinc-900 text-white"
                  : "border-zinc-200 bg-white text-zinc-600 hover:border-zinc-300 hover:text-zinc-900"
              }`}
            >
              {purpose}
            </button>
          ))}
        </div>
        <div className="grid gap-3 lg:grid-cols-3">
          <InsightCard
            title={t("llmAnalysis.ranking.bestCost")}
            body={bestCostModel ? `${providerLabel(bestCostModel.provider)} / ${bestCostModel.model}` : "—"}
            meta={
              bestCostModel
                ? `${fmtUSD(bestCostModel.avg_cost_per_call_usd)} avg/call · ${fmtNum(bestCostModel.calls)} calls`
                : "—"
            }
          />
          <InsightCard
            title={t("llmAnalysis.ranking.bestTokens")}
            body={bestTokenModel ? `${providerLabel(bestTokenModel.provider)} / ${bestTokenModel.model}` : "—"}
            meta={
              bestTokenModel
                ? `${fmtNum(Math.round(bestTokenModel.total_tokens_per_call))} tokens/call · ${fmtNum(bestTokenModel.calls)} calls`
                : "—"
            }
          />
          <InsightCard
            title={t("llmAnalysis.ranking.bestQuality")}
            body={bestQualityModel ? `${providerLabel(bestQualityModel.provider)} / ${bestQualityModel.model}` : "—"}
            meta={
              bestQualityModel
                ? `${(bestQualityModel.failure_rate_pct ?? 0).toFixed(1)}% fail · ${(bestQualityModel.retry_rate_pct ?? 0).toFixed(1)}% retry`
                : t("llmAnalysis.ranking.noQualityData")
            }
          />
        </div>
        <div className="mt-3 rounded-lg border border-zinc-200 bg-zinc-50 px-3 py-3">
          <div className="grid gap-3 lg:grid-cols-[minmax(0,320px)_1fr] lg:items-end">
            <ModelSelect
              label={t("llmAnalysis.diff.selectLabel")}
              value={selectedModelKey ?? ""}
              onChange={(value) => setSelectedModelKey(value || null)}
              options={rankingModelOptions}
              labels={{
                defaultOption: t("llmAnalysis.diff.selectPlaceholder"),
                searchPlaceholder: t("settings.modelSelect.searchPlaceholder"),
                noResults: t("settings.modelSelect.noResults"),
              }}
            />
            <p className="text-xs text-zinc-500 lg:pb-2">{t("llmAnalysis.diff.selectHelp")}</p>
          </div>
        </div>
        <div className="mt-3 grid gap-3 lg:grid-cols-2">
          <InsightCard
            title={`${rankingPurposeLabel} ${t("llmAnalysis.diff.spread")}`}
            body={bestCostModel && rankingMedianCost != null ? `${fmtUSD(bestCostModel.avg_cost_per_call_usd)} -> ${fmtUSD(rankingMedianCost)}` : "—"}
            meta={
              bestCostModel && rankingMedianCost != null && bestCostModel.avg_cost_per_call_usd > 0
                ? `${t("llmAnalysis.diff.betweenCheapestAndMedian")} · ${(((rankingMedianCost - bestCostModel.avg_cost_per_call_usd) / bestCostModel.avg_cost_per_call_usd) * 100).toFixed(0)}% ${t("llmAnalysis.diff.moreThanCheapest")}`
                : "—"
            }
          />
          <InsightCard
            title={`${rankingPurposeLabel} ${t("llmAnalysis.diff.selected")}`}
            body={
              selectedRankingRow
                ? `${providerLabel(selectedRankingRow.provider)} / ${selectedRankingRow.model}`
                : t("llmAnalysis.diff.selectPlaceholder")
            }
            meta={
              selectedRankingRow
                ? `${selectedRankingRow.vs_cheapest_pct >= 0 ? "+" : ""}${selectedRankingRow.vs_cheapest_pct.toFixed(0)}% ${t("llmAnalysis.diff.vsCheapestInPurpose")} · ${selectedRankingRow.vs_median_pct >= 0 ? "+" : ""}${selectedRankingRow.vs_median_pct.toFixed(0)}% ${t("llmAnalysis.diff.vsMedianInPurpose")}`
                : t("llmAnalysis.diff.selectedHelp")
            }
          />
        </div>
        <div className="mt-4 rounded-xl border border-zinc-200 bg-zinc-50/70 p-4">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-400">{t("llmAnalysis.section.qa")}</div>
              <h3 className="mt-1 text-sm font-semibold text-zinc-900">{t("llmAnalysis.qa.title")}</h3>
              <p className="mt-1 text-xs text-zinc-500">{t("llmAnalysis.qa.description")}</p>
            </div>
            <div className="flex flex-wrap items-center justify-end gap-2">
              {selectedRankingRow ? (
                <span className="rounded-full border border-zinc-200 bg-white px-2.5 py-1 text-xs text-zinc-500">
                  {providerLabel(selectedRankingRow.provider)} / {selectedRankingRow.model}
                </span>
              ) : null}
              {selectedModelKey ? (
                <button
                  type="button"
                  onClick={() => setSelectedModelKey(null)}
                  className="rounded-md border border-zinc-300 bg-white px-2.5 py-1 text-xs font-medium text-zinc-700 hover:bg-zinc-50"
                >
                  {t("llmAnalysis.diff.clearSelection")}
                </button>
              ) : null}
            </div>
          </div>
          {qaLoading ? (
            <p className="text-sm text-zinc-500">{t("common.loading")}</p>
          ) : !selectedRankingRow ? (
            <p className="text-sm text-zinc-500">{t("llmAnalysis.qa.emptySelection")}</p>
          ) : qaSamples.length === 0 ? (
            <p className="text-sm text-zinc-500">{t("llmAnalysis.qa.noSamples")}</p>
          ) : (
            <div className="grid gap-3 lg:grid-cols-3">
              {qaSamples.map((item) => (
                <article key={item.id} className="rounded-lg border border-zinc-200 bg-white p-3 shadow-sm">
                  <div className="text-xs text-zinc-400">
                    {item.summary_llm ? `${providerLabel(item.summary_llm.provider)} / ${item.summary_llm.model}` : providerLabel(selectedRankingRow.provider)}
                  </div>
                  <h4 className="mt-1 line-clamp-2 text-sm font-semibold text-zinc-900">{item.translated_title || item.title || t("llmAnalysis.qa.untitled")}</h4>
                  <div className="mt-2 flex flex-wrap gap-1 text-[11px] text-zinc-500">
                    {item.summary ? <span className="rounded-full bg-zinc-100 px-2 py-0.5">summary</span> : null}
                    {item.facts?.facts?.length ? <span className="rounded-full bg-zinc-100 px-2 py-0.5">facts {item.facts.facts.length}</span> : null}
                    {item.facts_check?.final_result ? <span className="rounded-full bg-zinc-100 px-2 py-0.5">facts check {item.facts_check.final_result}</span> : null}
                    {item.faithfulness?.final_result ? <span className="rounded-full bg-zinc-100 px-2 py-0.5">faithfulness {item.faithfulness.final_result}</span> : null}
                  </div>
                  {selectedRankingRow.purpose === "facts" || selectedRankingRow.purpose === "facts_check" ? (
                    <ul className="mt-3 space-y-1 text-xs leading-5 text-zinc-700">
                      {(item.facts?.facts ?? []).slice(0, 4).map((fact, idx) => (
                        <li key={idx} className="line-clamp-2">• {fact}</li>
                      ))}
                    </ul>
                  ) : (
                    <p className="mt-3 line-clamp-6 text-xs leading-5 text-zinc-700">{item.summary?.summary || t("llmAnalysis.qa.noSummary")}</p>
                  )}
                  <Link href={`/items/${item.id}`} className="mt-3 inline-flex text-xs font-medium text-zinc-600 hover:text-zinc-900">
                    {t("llmAnalysis.qa.openItem")}
                  </Link>
                </article>
              ))}
            </div>
          )}
        </div>
        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="text-xs text-zinc-500">
              <tr className="border-b border-zinc-100">
                <th className="px-3 py-2 text-left font-medium">{t("llmAnalysis.provider")}</th>
                <th className="px-3 py-2 text-left font-medium">{t("llmAnalysis.model")}</th>
                <th className="px-3 py-2 text-right font-medium">calls</th>
                <th className="px-3 py-2 text-right font-medium">avg/call</th>
                <th className="px-3 py-2 text-right font-medium">{t("llmAnalysis.diff.vsCheapest")}</th>
                <th className="px-3 py-2 text-right font-medium">{t("llmAnalysis.diff.vsMedian")}</th>
                <th className="px-3 py-2 text-right font-medium">tokens/call</th>
                <th className="px-3 py-2 text-right font-medium">{t("llmAnalysis.quality.failureRate")}</th>
                <th className="px-3 py-2 text-right font-medium">{t("llmAnalysis.quality.retryRate")}</th>
                <th className="px-3 py-2 text-right font-medium">{t("llmAnalysis.quality.emptyRate")}</th>
              </tr>
            </thead>
            <tbody>
              {rankingRows.length === 0 ? (
                <tr>
                  <td colSpan={10} className="px-3 py-6 text-center text-sm text-zinc-400">
                    {t("llm.noSummary")}
                  </td>
                </tr>
              ) : (
                rankingRows.slice(0, 12).map((row) => (
                  <tr
                    key={`${row.provider}:${row.model}:${row.purpose}`}
                    className="cursor-pointer border-b border-zinc-100 last:border-0 hover:bg-zinc-50"
                    onClick={() => applyRowFilter(row)}
                  >
                    <td className="px-3 py-2">{providerLabel(row.provider)}</td>
                    <td className="px-3 py-2 whitespace-nowrap text-xs">{row.model}</td>
                    <td className="px-3 py-2 text-right">{fmtNum(row.calls)}</td>
                    <td className="px-3 py-2 text-right">{fmtUSD(row.avg_cost_per_call_usd)}</td>
                    <td className={`px-3 py-2 text-right ${row.vs_cheapest_pct > 0 ? "text-amber-700" : "text-emerald-700"}`}>
                      {row.vs_cheapest_pct >= 0 ? "+" : ""}{row.vs_cheapest_pct.toFixed(0)}%
                    </td>
                    <td className={`px-3 py-2 text-right ${row.vs_median_pct > 0 ? "text-amber-700" : "text-emerald-700"}`}>
                      {row.vs_median_pct >= 0 ? "+" : ""}{row.vs_median_pct.toFixed(0)}%
                    </td>
                    <td className="px-3 py-2 text-right">{fmtNum(Math.round(row.total_tokens_per_call))}</td>
                    <td className="px-3 py-2 text-right">{row.failure_rate_pct == null ? "—" : `${row.failure_rate_pct.toFixed(1)}%`}</td>
                    <td className="px-3 py-2 text-right">{row.retry_rate_pct == null ? "—" : `${row.retry_rate_pct.toFixed(1)}%`}</td>
                    <td className="px-3 py-2 text-right">{row.empty_rate_pct == null ? "—" : `${row.empty_rate_pct.toFixed(1)}%`}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </section>

      {loading && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {error && <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>}


      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <SectionTitle
          eyebrow={t("llmAnalysis.section.details")}
          title={t("llmAnalysis.section.detailsTitle")}
          description={t("llmAnalysis.section.detailsDescription")}
        />
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-zinc-800">{t("llmAnalysis.matrixTitle")}</h2>
          <span className="text-xs text-zinc-400">{sortedRows.length} rows</span>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="text-xs text-zinc-500">
              <tr className="border-b border-zinc-100">
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("provider")} className="inline-flex items-center hover:text-zinc-800">{t("llmAnalysis.provider")}{sortMark("provider")}</button></th>
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("model")} className="inline-flex items-center hover:text-zinc-800">{t("llmAnalysis.model")}{sortMark("model")}</button></th>
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("purpose")} className="inline-flex items-center hover:text-zinc-800">{t("llmAnalysis.purpose")}{sortMark("purpose")}</button></th>
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("pricing_source")} className="inline-flex items-center hover:text-zinc-800">{t("llmAnalysis.pricing")}{sortMark("pricing_source")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("calls")} className="inline-flex items-center hover:text-zinc-800">calls{sortMark("calls")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("avg_input_tokens_per_call")} className="inline-flex items-center hover:text-zinc-800">avg in/call{sortMark("avg_input_tokens_per_call")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("avg_output_tokens_per_call")} className="inline-flex items-center hover:text-zinc-800">avg out/call{sortMark("avg_output_tokens_per_call")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("avg_cost_per_call_usd")} className="inline-flex items-center hover:text-zinc-800">avg/call{sortMark("avg_cost_per_call_usd")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("cost_share_pct")} className="inline-flex items-center hover:text-zinc-800">share{sortMark("cost_share_pct")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("estimated_cost_usd")} className="inline-flex items-center hover:text-zinc-800">cost{sortMark("estimated_cost_usd")}</button></th>
              </tr>
            </thead>
            <tbody>
              {sortedRows.map((row) => (
                <tr key={`${row.provider}:${row.model}:${row.purpose}:${row.pricing_source}`} className="border-b border-zinc-100 last:border-0">
                  <td className="px-3 py-2">{providerLabel(row.provider)}</td>
                  <td className="px-3 py-2 whitespace-nowrap text-xs">{row.model}</td>
                  <td className="px-3 py-2">{row.purpose}</td>
                  <td className="px-3 py-2 text-xs">{row.pricing_source}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.calls)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(Math.round(row.avg_input_tokens_per_call))}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(Math.round(row.avg_output_tokens_per_call))}</td>
                  <td className="px-3 py-2 text-right">{fmtUSD(row.avg_cost_per_call_usd)}</td>
                  <td className="px-3 py-2 text-right">{row.cost_share_pct.toFixed(1)}%</td>
                  <td className="px-3 py-2 text-right">{fmtUSD(row.estimated_cost_usd)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white px-4 py-3 shadow-sm">
      <div className="text-xs font-medium text-zinc-500">{label}</div>
      <div className="mt-1 text-lg font-semibold text-zinc-900">{value}</div>
    </div>
  );
}

function SectionTitle({
  eyebrow,
  title,
  description,
  compact = false,
}: {
  eyebrow: string;
  title: string;
  description: string;
  compact?: boolean;
}) {
  return (
    <div className={compact ? "" : "mb-4"}>
      <div className="text-[11px] font-semibold uppercase tracking-[0.12em] text-zinc-400">{eyebrow}</div>
      <h2 className="mt-1 text-base font-semibold text-zinc-900">{title}</h2>
      <p className="mt-1 text-sm text-zinc-500">{description}</p>
    </div>
  );
}

function InsightCard({ title, body, meta, onClick }: { title: string; body: string; meta: string; onClick?: () => void }) {
  const Comp = onClick ? "button" : "div";
  return (
    <Comp
      {...(onClick ? { type: "button", onClick } : {})}
      className={`rounded-lg border border-zinc-200 bg-white px-4 py-4 text-left shadow-sm ${onClick ? "transition hover:border-zinc-300 hover:bg-zinc-50" : ""}`}
    >
      <div className="text-xs font-medium uppercase tracking-[0.08em] text-zinc-500">{title}</div>
      <div className="mt-2 text-sm font-semibold leading-6 text-zinc-900">{body}</div>
      <div className="mt-2 text-xs text-zinc-500">{meta}</div>
    </Comp>
  );
}
