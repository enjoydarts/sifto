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
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";
import { formatModelDisplayName } from "@/lib/model-display";

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
    case "together":
      return "Together AI";
    case "xai":
      return "xAI";
    case "zai":
      return "Z.ai";
    case "fireworks":
      return "Fireworks";
    case "moonshot":
      return "Moonshot";
    case "openrouter":
      return "OpenRouter";
    case "poe":
      return "Poe";
    case "siliconflow":
      return "SiliconFlow";
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
    case "together":
      return "#0ea5a4";
    case "xai":
      return "#818cf8";
    case "zai":
      return "#22d3ee";
    case "fireworks":
      return "#f97316";
    case "moonshot":
      return "#db2777";
    case "poe":
      return "#0f766e";
    case "siliconflow":
      return "#2563eb";
    default:
      return "#71717a";
  }
}

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}

type ChartTooltipValue = number | string | ReadonlyArray<number | string>;
type ChartTooltipName = number | string;

function tooltipValueToNumber(value: ChartTooltipValue | undefined) {
  if (Array.isArray(value)) return Number(value[0] ?? 0);
  return Number(value ?? 0);
}

function tooltipValueToText(value: ChartTooltipValue | undefined) {
  if (Array.isArray(value)) return value.map(String).join(", ");
  return String(value ?? "");
}

function tooltipNameToText(name: ChartTooltipName | undefined) {
  return String(name ?? "");
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

type UsageScatterRow = AnalysisRow & {
  totalTokensPerCall: number;
  avgCostPerCall: number;
  bubbleSize: number;
  costCallBubbleSize: number;
  label: string;
  pricingLabel: string;
};

type LLMAnalysisSectionID =
  | "overview"
  | "charts"
  | "mix"
  | "quality"
  | "recommend"
  | "details";

function UsageScatterChart({
  rows,
  variant,
  t,
  onPointClick,
}: {
  rows: UsageScatterRow[];
  variant: "efficiency" | "costCalls";
  t: (key: string) => string;
  onPointClick: (row: UsageScatterRow) => void;
}) {
  const isEfficiency = variant === "efficiency";
  return (
    <div className="h-[24rem] w-full">
      <ResponsiveContainer width="100%" height="100%">
        <ScatterChart margin={{ top: 12, right: 20, left: 8, bottom: 12 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" />
          <XAxis
            type="number"
            dataKey={isEfficiency ? "totalTokensPerCall" : "calls"}
            name={isEfficiency ? "Tokens / call" : "Calls"}
            tick={{ fontSize: 12, fill: "#8f877f" }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v) => fmtNum(Number(v))}
          />
          <YAxis
            type="number"
            dataKey="avgCostPerCall"
            name="Cost / call"
            tick={{ fontSize: 12, fill: "#8f877f" }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v) => fmtUSD(Number(v))}
          />
          <ZAxis type="number" dataKey={isEfficiency ? "bubbleSize" : "costCallBubbleSize"} range={[80, 460]} />
          <Tooltip
            cursor={{ strokeDasharray: "4 4" }}
            formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => {
              if (name === "Cost / call") return [fmtUSD(tooltipValueToNumber(value)), tooltipNameToText(name)];
              if (name === "Tokens / call" || name === "Calls") return [fmtNum(tooltipValueToNumber(value)), tooltipNameToText(name)];
              return [tooltipValueToText(value), tooltipNameToText(name)];
            }}
            labelFormatter={(_, payload) => {
              const row = payload?.[0]?.payload as UsageScatterRow | undefined;
              if (!row) return "";
              return `${providerLabel(row.provider)} / ${formatModelDisplayName(row.model)} (${row.purpose})`;
            }}
            content={({ active, payload }) => {
              const row = payload?.[0]?.payload as UsageScatterRow | undefined;
              if (!active || !row) return null;
              return (
                <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-white px-3 py-2 shadow-[var(--shadow-dropdown)]">
                  <div className="text-xs font-semibold text-[var(--color-editorial-ink)]">{providerLabel(row.provider)} / {formatModelDisplayName(row.model)}</div>
                  <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{row.purpose}</div>
                  <div className="mt-2 grid gap-1 text-xs text-[var(--color-editorial-ink-soft)]">
                    <div>{t("llm.totalCalls")}: {fmtNum(row.calls)}</div>
                    <div>tokens/call: {fmtNum(row.totalTokensPerCall)}</div>
                    <div>avg cost/call: {fmtUSD(row.avgCostPerCall)}</div>
                    <div>total cost: {fmtUSD(row.estimated_cost_usd)}</div>
                    <div>pricing: {row.pricingLabel}</div>
                  </div>
                </div>
              );
            }}
          />
          <Scatter
            data={rows}
            name={isEfficiency ? "Calls" : "Tokens / call"}
            shape={(props: { cx?: number; cy?: number; payload?: UsageScatterRow }) => {
              const row = props.payload;
              if (props.cx == null || props.cy == null || !row) return null;
              return (
                <circle
                  cx={props.cx}
                  cy={props.cy}
                  r={isEfficiency ? row.bubbleSize : row.costCallBubbleSize}
                  fill={providerColor(row.provider)}
                  fillOpacity={0.72}
                  stroke="#ffffff"
                  strokeWidth={2}
                  className="cursor-pointer transition-opacity hover:opacity-100"
                  onClick={() => onPointClick(row)}
                />
              );
            }}
          />
        </ScatterChart>
      </ResponsiveContainer>
    </div>
  );
}

export default function LLMAnalysisPage() {
  const { t } = useI18n();
  const [activeSection, setActiveSection] = useState<LLMAnalysisSectionID>("overview");
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
        api.getLLMExecutionCurrentMonthSummary({ days: selectedDays }),
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
        label: formatModelDisplayName(row.model),
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

  const scatterRows = useMemo<UsageScatterRow[]>(() => {
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
        costCallBubbleSize: Math.max(6, Math.min(22, 6 + Math.sqrt(Math.max(1, Math.round(row.avg_input_tokens_per_call + row.avg_output_tokens_per_call))) / 6)),
        label: `${providerLabel(row.provider)} / ${formatModelDisplayName(row.model)}`,
        pricingLabel: row.pricingSources.length === 1 ? row.pricingSources[0] : t("llmAnalysis.mixedPricingSources"),
      }))
      .sort((a, b) => b.calls - a.calls)
      .slice(0, 36);
  }, [scatterPurpose, sortedRows, t]);

  const providerMixRows = useMemo(() => {
    const preferredOrder = ["facts", "summary", "digest", "ask", "facts_check", "faithfulness_check"];
    const countsByPurpose = new Map<string, Record<string, number>>();
    for (const row of sortedRows) {
      if (!preferredOrder.includes(row.purpose)) continue;
      const current = countsByPurpose.get(row.purpose) ?? {};
      current[row.provider] = (current[row.provider] ?? 0) + row.calls;
      countsByPurpose.set(row.purpose, current);
    }
    return preferredOrder
      .filter((purpose) => countsByPurpose.has(purpose))
      .map((purpose) => {
        const counts = countsByPurpose.get(purpose) ?? {};
        const total = Object.values(counts).reduce((sum, value) => sum + value, 0);
        return {
          purpose,
          ...Object.fromEntries(
            providers.map((provider) => [provider, total > 0 ? ((counts[provider] ?? 0) / total) * 100 : 0])
          ),
        };
      });
  }, [providers, sortedRows]);

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
        label: formatModelDisplayName(row.model),
        provider: providerLabel(row.provider),
        note: `${row.purpose} · ${fmtUSD(row.avg_cost_per_call_usd)}`,
      })),
    [rankingRows]
  );
  const selectedRankingRow = useMemo(() => {
    if (!selectedModelKey) return null;
    return rankingRows.find((row) => `${row.provider}:${row.model}:${row.purpose}` === selectedModelKey) ?? null;
  }, [rankingRows, selectedModelKey]);

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

  const railSections = useMemo(
    () => [
      {
        id: "overview" as const,
        label: t("llmAnalysis.section.overview"),
        meta: t("llmAnalysis.section.overviewDescription"),
      },
      {
        id: "charts" as const,
        label: t("llmAnalysis.section.charts"),
        meta: t("llmAnalysis.section.chartsDescription"),
      },
      {
        id: "mix" as const,
        label: t("llmAnalysis.section.mix"),
        meta: t("llmAnalysis.section.mixDescription"),
      },
      {
        id: "quality" as const,
        label: t("llmAnalysis.section.quality"),
        meta: t("llmAnalysis.section.qualityDescription"),
      },
      {
        id: "recommend" as const,
        label: t("llmAnalysis.section.recommend"),
        meta: t("llmAnalysis.rankingHelp"),
      },
      {
        id: "details" as const,
        label: t("llmAnalysis.section.details"),
        meta: t("llmAnalysis.section.detailsDescription"),
      },
    ],
    [t]
  );

  const activeSectionTitle = useMemo(() => {
    switch (activeSection) {
      case "overview":
        return t("llmAnalysis.section.overviewTitle");
      case "charts":
        return t("llmAnalysis.section.chartsTitle");
      case "mix":
        return t("llmAnalysis.section.mixTitle");
      case "quality":
        return t("llmAnalysis.section.qualityTitle");
      case "recommend":
        return t("llmAnalysis.rankingTitle");
      case "details":
        return t("llmAnalysis.section.detailsTitle");
    }
  }, [activeSection, t]);

  const activeSectionDescription = useMemo(() => {
    switch (activeSection) {
      case "overview":
        return t("llmAnalysis.section.overviewDescription");
      case "charts":
        return t("llmAnalysis.section.chartsDescription");
      case "mix":
        return t("llmAnalysis.section.mixDescription");
      case "quality":
        return t("llmAnalysis.section.qualityDescription");
      case "recommend":
        return t("llmAnalysis.rankingHelp");
      case "details":
        return t("llmAnalysis.section.detailsDescription");
    }
  }, [activeSection, t]);

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
    <div className="space-y-6 overflow-x-hidden">
      <PageHeader
        eyebrow={t("llmAnalysis.title")}
        title={t("llmAnalysis.title")}
        titleIcon={TableProperties}
        description={t("llmAnalysis.subtitle")}
        compact
        meta={
          <>
            <span>{`${t("llm.totalCost")}: ${fmtUSD(totals.cost)}`}</span>
            <span>{`${t("llm.totalCalls")}: ${fmtNum(totals.calls)}`}</span>
            <span>{`${t("llm.input")}: ${fmtNum(totals.input)}`}</span>
            <span>{`${t("llm.output")}: ${fmtNum(totals.output)}`}</span>
          </>
        }
        actions={
          <Link
            href="/llm-usage"
            className="inline-flex min-h-10 items-center gap-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)] press focus-ring"
          >
            <Brain className="size-4" aria-hidden="true" />
            <span>{t("llmAnalysis.backToUsage")}</span>
          </Link>
        }
      />

      {loading && <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>}
      {error && <div className="rounded-[16px] border border-[var(--color-editorial-error-line)] bg-[var(--color-editorial-error-soft)] px-4 py-3 text-sm text-[var(--color-editorial-error)]">{error}</div>}

      <SectionCard>
        <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
          <Filter className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
          <span>{t("llmAnalysis.filters")}</span>
        </div>
        <div className="grid gap-3 md:grid-cols-4">
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llm.days")}</span>
            <select value={days} onChange={(e) => setDays(e.target.value as typeof days)} className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]">
              <option value="14">14{t("llm.daysSuffix")}</option>
              <option value="30">30{t("llm.daysSuffix")}</option>
              <option value="90">90{t("llm.daysSuffix")}</option>
              <option value="180">180{t("llm.daysSuffix")}</option>
            </select>
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.provider")}</span>
            <select value={providerFilter} onChange={(e) => setProviderFilter(e.target.value)} className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]">
              <option value="all">{t("llmAnalysis.all")}</option>
              {providers.map((provider) => (
                <option key={provider} value={provider}>{providerLabel(provider)}</option>
              ))}
            </select>
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.purpose")}</span>
            <select value={purposeFilter} onChange={(e) => setPurposeFilter(e.target.value)} className="w-full rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3 text-sm text-[var(--color-editorial-ink)]">
              <option value="all">{t("llmAnalysis.all")}</option>
              {purposes.map((purpose) => (
                <option key={purpose} value={purpose}>{purpose}</option>
              ))}
            </select>
          </label>
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.modelSearch")}</span>
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
                providerAll: t("settings.modelSelect.providerAll"),
                modalChoose: t("settings.modelSelect.modalChoose"),
                close: t("common.close"),
                confirmTitle: t("settings.modelSelect.confirmTitle"),
                confirmYes: t("settings.modelSelect.confirmYes"),
                confirmNo: t("settings.modelSelect.confirmNo"),
                confirmSuffix: t("settings.modelSelect.confirmSuffix"),
                providerColumn: t("settings.modelSelect.providerColumn"),
                modelColumn: t("settings.modelSelect.modelColumn"),
                pricingColumn: t("settings.modelSelect.pricingColumn"),
              }}
            />
          </label>
        </div>
        {hasActiveDrilldown ? (
          <div className="mt-3 flex items-center justify-between gap-3 rounded-[14px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-3">
            <div className="text-xs text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.drilldownActive")}</div>
            <button
              type="button"
              onClick={clearFilters}
              className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
            >
              {t("llmAnalysis.clearDrilldown")}
            </button>
          </div>
        ) : null}
      </SectionCard>

      <div className="grid gap-6 xl:grid-cols-[248px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <SectionCard>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              Analysis Sections
            </div>
            <div className="mt-4 grid gap-0">
              {railSections.map((section) => (
                <button
                  key={section.id}
                  type="button"
                  onClick={() => setActiveSection(section.id)}
                  className={joinClassNames(
                    "relative border-t border-[var(--color-editorial-line)] px-4 py-3 text-left first:border-t-0 first:pt-0",
                    activeSection === section.id
                      ? "bg-[linear-gradient(90deg,rgba(243,236,227,0.92),rgba(243,236,227,0.28)_78%,transparent)]"
                      : ""
                  )}
                >
                  {activeSection === section.id ? (
                    <span className="absolute bottom-3 left-0 top-3 w-[3px] rounded-full bg-[var(--color-editorial-ink)] first:top-0" />
                  ) : null}
                  <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{section.label}</div>
                  <div className="mt-1 text-[11px] leading-5 text-[var(--color-editorial-ink-soft)] sm:text-xs">{section.meta}</div>
                </button>
              ))}
            </div>
          </SectionCard>

          <SectionCard>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              Status
            </div>
            <div className="mt-4 grid gap-3">
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llm.totalCost")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {fmtUSD(totals.cost)} / {fmtNum(totals.calls)} calls
                </div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmAnalysis.section.mix")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {providerMixRows.length} purposes / {providers.length} providers
                </div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmAnalysis.section.details")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {sortedRows.length} rows / {rankingRows.length} ranked
                </div>
              </div>
            </div>
          </SectionCard>
        </aside>

        <div className="min-w-0 space-y-6">
          <SectionCard>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              {railSections.find((section) => section.id === activeSection)?.label}
            </div>
            <h2 className="mt-2 font-serif text-[1.8rem] leading-[1.1] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
              {activeSectionTitle}
            </h2>
            <p className="mt-3 max-w-3xl text-sm leading-7 text-[var(--color-editorial-ink-soft)]">
              {activeSectionDescription}
            </p>
          </SectionCard>

          {activeSection === "overview" ? (
            <>
              <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <InsightCard title={t("llm.totalCost")} body={fmtUSD(totals.cost)} meta={`${fmtNum(totals.calls)} calls`} />
                <InsightCard title={t("llm.input")} body={fmtNum(totals.input)} meta={`${fmtNum(totals.output)} output`} />
                <InsightCard
                  title={t("llmAnalysis.insight.topCost")}
                  body={sortedRows[0] ? `${providerLabel(sortedRows[0].provider)} / ${formatModelDisplayName(sortedRows[0].model)}` : "—"}
                  meta={sortedRows[0] ? `${sortedRows[0].purpose} · ${fmtUSD(sortedRows[0].estimated_cost_usd)}` : "—"}
                />
                <InsightCard
                  title={t("llmAnalysis.insight.topPurpose")}
                  body={providerMixRows[0]?.purpose ?? "—"}
                  meta={providerMixRows[0] ? `${fmtNum(Math.round(Object.values(providerMixRows[0]).filter((value) => typeof value === "number").reduce((sum, value) => sum + Number(value), 0)))}% mix tracked` : "—"}
                />
              </section>

              <section className="grid gap-4 xl:grid-cols-[minmax(0,1.65fr)_minmax(0,1fr)]">
                <SectionCard>
                  <SectionTitle
                    eyebrow={t("llmAnalysis.section.charts")}
                    title={t("llmAnalysis.efficiencyScatter")}
                    description={t("llmAnalysis.efficiencyScatterHelp")}
                    compact
                  />
                  <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    {scatterRows.length === 0 ? (
                      <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                    ) : (
                      <div className="h-[24rem] w-full">
                        <ResponsiveContainer width="100%" height="100%">
                          <ScatterChart margin={{ top: 12, right: 20, left: 8, bottom: 12 }}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" />
                            <XAxis type="number" dataKey="totalTokensPerCall" name="Tokens / call" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} tickFormatter={(v) => fmtNum(Number(v))} />
                            <YAxis type="number" dataKey="avgCostPerCall" name="Cost / call" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} tickFormatter={(v) => fmtUSD(Number(v))} />
                            <ZAxis type="number" dataKey="bubbleSize" range={[80, 460]} />
                            <Tooltip
                              cursor={{ strokeDasharray: "4 4" }}
                              formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => {
                                if (name === "Cost / call") return [fmtUSD(tooltipValueToNumber(value)), tooltipNameToText(name)];
                                if (name === "Tokens / call") return [fmtNum(tooltipValueToNumber(value)), tooltipNameToText(name)];
                                if (name === "Calls") return [fmtNum(tooltipValueToNumber(value)), tooltipNameToText(name)];
                                return [tooltipValueToText(value), tooltipNameToText(name)];
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
                                  <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-white px-3 py-2 shadow-[var(--shadow-dropdown)]">
                                    <div className="text-xs font-semibold text-[var(--color-editorial-ink)]">{providerLabel(row.provider)} / {formatModelDisplayName(row.model)}</div>
                                    <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{row.purpose}</div>
                                    <div className="mt-2 grid gap-1 text-xs text-[var(--color-editorial-ink-soft)]">
                                      <div>{t("llm.totalCalls")}: {fmtNum(row.calls)}</div>
                                      <div>tokens/call: {fmtNum(row.totalTokensPerCall)}</div>
                                      <div>avg cost/call: {fmtUSD(row.avgCostPerCall)}</div>
                                      <div>pricing: {row.pricingLabel}</div>
                                    </div>
                                  </div>
                                );
                              }}
                            />
                            <Scatter
                              data={scatterRows}
                              name="Calls"
                              shape={(props: { cx?: number; cy?: number; payload?: (typeof scatterRows)[number] }) => {
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
                  </div>
                </SectionCard>

                <SectionCard>
                  <SectionTitle
                    eyebrow={t("llmAnalysis.section.mix")}
                    title={t("llmAnalysis.section.mixTitle")}
                    description={t("llmAnalysis.section.mixDescription")}
                    compact
                  />
                  <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                    {providerMixRows.length === 0 ? (
                      <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                    ) : (
                      <div className="h-[24rem] w-full">
                        <ResponsiveContainer width="100%" height="100%">
                          <BarChart data={providerMixRows} layout="vertical" margin={{ top: 8, right: 16, left: 12, bottom: 0 }}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#e4dbcf" horizontal={false} />
                            <XAxis type="number" domain={[0, 100]} tick={{ fontSize: 12, fill: "#8f877f" }} tickFormatter={(v) => `${v}%`} tickLine={false} axisLine={false} />
                            <YAxis type="category" dataKey="purpose" width={130} tick={{ fontSize: 12, fill: "#5f5750" }} tickLine={false} axisLine={false} />
                            <Tooltip
                              formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [`${tooltipValueToNumber(value).toFixed(1)}%`, providerLabel(tooltipNameToText(name))]}
                              contentStyle={{ borderRadius: 16, borderColor: "#d9d1c4", background: "#fff" }}
                            />
                            {providers.map((provider, index) => (
                              <Bar key={provider} dataKey={provider} stackId="purpose" fill={providerColor(provider)} radius={index === providers.length - 1 ? [999, 999, 999, 999] : 0}>
                                {providerMixRows.map((row) => (
                                  <Cell key={`${provider}:${row.purpose}`} fill={providerColor(provider)} />
                                ))}
                              </Bar>
                            ))}
                          </BarChart>
                        </ResponsiveContainer>
                      </div>
                    )}
                    <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-xs text-[var(--color-editorial-ink-soft)]">
                      {providers.map((provider) => (
                        <span key={provider} className="inline-flex items-center gap-2">
                          <span className="size-2.5 rounded-full" style={{ backgroundColor: providerColor(provider) }} />
                          {providerLabel(provider)}
                        </span>
                      ))}
                    </div>
                  </div>
                </SectionCard>
              </section>
            </>
          ) : null}

          {activeSection === "charts" ? (
            <SectionCard>
              <SectionTitle eyebrow={t("llmAnalysis.section.charts")} title={t("llmAnalysis.section.chartsTitle")} description={t("llmAnalysis.section.chartsDescription")} />
              <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <h2 className="font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llmAnalysis.efficiencyScatter")}</h2>
                    <p className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.efficiencyScatterHelp")}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    {hasScatterFilter ? (
                      <button
                        type="button"
                        onClick={clearScatterFilters}
                        className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                      >
                        {t("llmAnalysis.clearDrilldown")}
                      </button>
                    ) : null}
                    <span className="text-xs text-[var(--color-editorial-ink-faint)]">{scatterRows.length} rows</span>
                  </div>
                </div>
                <div className="mb-3 flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => setScatterPurpose("all")}
                    className={joinClassNames(
                      "rounded-full border px-3 py-1 text-xs font-medium transition",
                      scatterPurpose === "all"
                        ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                        : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                    )}
                  >
                    {t("llmAnalysis.all")}
                  </button>
                  {purposes.map((purpose) => (
                    <button
                      key={purpose}
                      type="button"
                      onClick={() => setScatterPurpose(purpose)}
                      className={joinClassNames(
                        "rounded-full border px-3 py-1 text-xs font-medium transition",
                        scatterPurpose === purpose
                          ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                          : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                      )}
                    >
                      {purpose}
                    </button>
                  ))}
                </div>
                {scatterRows.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                ) : (
                  <div className="space-y-4">
                    <UsageScatterChart rows={scatterRows} variant="efficiency" t={t} onPointClick={applyRowFilter} />
                    <div>
                      <h3 className="font-serif text-[1.2rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llmAnalysis.costCallsScatter")}</h3>
                      <p className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.costCallsScatterHelp")}</p>
                      <div className="mt-4">
                        <UsageScatterChart rows={scatterRows} variant="costCalls" t={t} onPointClick={applyRowFilter} />
                      </div>
                    </div>
                  </div>
                )}
              </section>
            </SectionCard>
          ) : null}

          {activeSection === "mix" ? (
            <SectionCard>
              <SectionTitle eyebrow={t("llmAnalysis.section.mix")} title={t("llmAnalysis.section.mixTitle")} description={t("llmAnalysis.section.mixDescription")} />
              <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                {providerMixRows.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                ) : (
                  <div className="h-[24rem] w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={providerMixRows} layout="vertical" margin={{ top: 8, right: 16, left: 12, bottom: 0 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#e4dbcf" horizontal={false} />
                        <XAxis type="number" domain={[0, 100]} tick={{ fontSize: 12, fill: "#8f877f" }} tickFormatter={(v) => `${v}%`} tickLine={false} axisLine={false} />
                        <YAxis type="category" dataKey="purpose" width={130} tick={{ fontSize: 12, fill: "#5f5750" }} tickLine={false} axisLine={false} />
                        <Tooltip
                          formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [`${tooltipValueToNumber(value).toFixed(1)}%`, providerLabel(tooltipNameToText(name))]}
                          contentStyle={{ borderRadius: 16, borderColor: "#d9d1c4", background: "#fff" }}
                        />
                        {providers.map((provider, index) => (
                          <Bar key={provider} dataKey={provider} stackId="purpose" fill={providerColor(provider)} radius={index === providers.length - 1 ? [999, 999, 999, 999] : 0}>
                            {providerMixRows.map((row) => (
                              <Cell key={`${provider}:${row.purpose}`} fill={providerColor(provider)} />
                            ))}
                          </Bar>
                        ))}
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                )}
                <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-xs text-[var(--color-editorial-ink-soft)]">
                  {providers.map((provider) => (
                    <span key={provider} className="inline-flex items-center gap-2">
                      <span className="size-2.5 rounded-full" style={{ backgroundColor: providerColor(provider) }} />
                      {providerLabel(provider)}
                    </span>
                  ))}
                </div>
              </section>
            </SectionCard>
          ) : null}

          {activeSection === "quality" ? (
            <SectionCard>
              <SectionTitle eyebrow={t("llmAnalysis.section.quality")} title={t("llmAnalysis.section.qualityTitle")} description={t("llmAnalysis.section.qualityDescription")} />
              <section className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4">
                {rankingRows.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                ) : (
                  <div className="h-[24rem] w-full">
                    <ResponsiveContainer width="100%" height="100%">
                      <ScatterChart margin={{ top: 12, right: 20, left: 8, bottom: 12 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" />
                        <XAxis type="number" dataKey="calls" name="Calls" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} />
                        <YAxis type="number" dataKey="failure_rate_pct" name="Failure Rate" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} tickFormatter={(v) => `${Number(v).toFixed(1)}%`} />
                        <ZAxis type="number" dataKey="calls" range={[80, 420]} />
                        <Tooltip
                          formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => {
                            if (name === "Failure Rate") return [`${tooltipValueToNumber(value).toFixed(1)}%`, tooltipNameToText(name)];
                            if (name === "Calls") return [fmtNum(tooltipValueToNumber(value)), tooltipNameToText(name)];
                            return [tooltipValueToText(value), tooltipNameToText(name)];
                          }}
                          labelFormatter={(_, payload) => {
                            const row = payload?.[0]?.payload as RankedModelRow | undefined;
                            if (!row) return "";
                            return `${providerLabel(row.provider)} / ${formatModelDisplayName(row.model)} (${row.purpose})`;
                          }}
                          content={({ active, payload }) => {
                            const row = payload?.[0]?.payload as RankedModelRow | undefined;
                            if (!active || !row) return null;
                            return (
                              <div className="rounded-[14px] border border-[var(--color-editorial-line)] bg-white px-3 py-2 shadow-[var(--shadow-dropdown)]">
                                <div className="text-xs font-semibold text-[var(--color-editorial-ink)]">{providerLabel(row.provider)} / {formatModelDisplayName(row.model)}</div>
                                <div className="mt-1 text-xs text-[var(--color-editorial-ink-soft)]">{row.purpose}</div>
                                <div className="mt-2 grid gap-1 text-xs text-[var(--color-editorial-ink-soft)]">
                                  <div>{t("llm.totalCalls")}: {fmtNum(row.calls)}</div>
                                  <div>avg/call: {fmtUSD(row.avg_cost_per_call_usd)}</div>
                                  <div>tokens/call: {fmtNum(Math.round(row.total_tokens_per_call))}</div>
                                  <div>failure: {row.failure_rate_pct == null ? "—" : `${row.failure_rate_pct.toFixed(1)}%`}</div>
                                  <div>retry: {row.retry_rate_pct == null ? "—" : `${row.retry_rate_pct.toFixed(1)}%`}</div>
                                  <div>empty: {row.empty_rate_pct == null ? "—" : `${row.empty_rate_pct.toFixed(1)}%`}</div>
                                </div>
                              </div>
                            );
                          }}
                        />
                        <Scatter
                          data={rankingRows.filter((row) => row.failure_rate_pct != null)}
                          shape={(props: { cx?: number; cy?: number; payload?: RankedModelRow }) => {
                            const row = props.payload;
                            if (props.cx == null || props.cy == null || !row) return null;
                            return (
                              <circle
                                cx={props.cx}
                                cy={props.cy}
                                r={Math.max(6, Math.min(20, 6 + Math.sqrt(row.calls)))}
                                fill={providerColor(row.provider)}
                                fillOpacity={0.75}
                                stroke="#ffffff"
                                strokeWidth={2}
                              />
                            );
                          }}
                        />
                      </ScatterChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </section>
            </SectionCard>
          ) : null}

          {activeSection === "recommend" ? (
            <SectionCard>
              <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
                <SectionTitle eyebrow={t("llmAnalysis.section.recommend")} title={t("llmAnalysis.rankingTitle")} description={t("llmAnalysis.rankingHelp")} compact />
                <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.rankingThreshold")}</span>
              </div>
              <div className="mb-4 flex flex-wrap gap-2">
                {purposes.map((purpose) => (
                  <button
                    key={purpose}
                    type="button"
                    onClick={() => setRankingPurpose(purpose)}
                    className={joinClassNames(
                      "rounded-full border px-3 py-1 text-xs font-medium transition",
                      rankingPurpose === purpose
                        ? "border-[var(--color-editorial-ink)] bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]"
                        : "border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel-strong)]"
                    )}
                  >
                    {purpose}
                  </button>
                ))}
              </div>
              <div className="grid gap-3 lg:grid-cols-3">
                <InsightCard
                  title={t("llmAnalysis.ranking.bestCost")}
                  body={bestCostModel ? `${providerLabel(bestCostModel.provider)} / ${bestCostModel.model}` : "—"}
                  meta={bestCostModel ? `${fmtUSD(bestCostModel.avg_cost_per_call_usd)} avg/call · ${fmtNum(bestCostModel.calls)} calls` : "—"}
                />
                <InsightCard
                  title={t("llmAnalysis.ranking.bestTokens")}
                  body={bestTokenModel ? `${providerLabel(bestTokenModel.provider)} / ${bestTokenModel.model}` : "—"}
                  meta={bestTokenModel ? `${fmtNum(Math.round(bestTokenModel.total_tokens_per_call))} tokens/call · ${fmtNum(bestTokenModel.calls)} calls` : "—"}
                />
                <InsightCard
                  title={t("llmAnalysis.ranking.bestQuality")}
                  body={bestQualityModel ? `${providerLabel(bestQualityModel.provider)} / ${bestQualityModel.model}` : "—"}
                  meta={bestQualityModel ? `${(bestQualityModel.failure_rate_pct ?? 0).toFixed(1)}% fail · ${(bestQualityModel.retry_rate_pct ?? 0).toFixed(1)}% retry` : t("llmAnalysis.ranking.noQualityData")}
                />
              </div>
              <div className="mt-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-4">
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
                      providerAll: t("settings.modelSelect.providerAll"),
                      modalChoose: t("settings.modelSelect.modalChoose"),
                      close: t("common.close"),
                      confirmTitle: t("settings.modelSelect.confirmTitle"),
                      confirmYes: t("settings.modelSelect.confirmYes"),
                      confirmNo: t("settings.modelSelect.confirmNo"),
                      confirmSuffix: t("settings.modelSelect.confirmSuffix"),
                      providerColumn: t("settings.modelSelect.providerColumn"),
                      modelColumn: t("settings.modelSelect.modelColumn"),
                      pricingColumn: t("settings.modelSelect.pricingColumn"),
                    }}
                  />
                  <p className="text-xs text-[var(--color-editorial-ink-soft)] lg:pb-2">{t("llmAnalysis.diff.selectHelp")}</p>
                </div>
              </div>
              <div className="mt-3 grid gap-3 lg:grid-cols-2">
                <InsightCard
                  title={`${rankingPurposeLabel} ${t("llmAnalysis.diff.spread")}`}
                  body={bestCostModel && rankingMedianCost != null ? `${fmtUSD(bestCostModel.avg_cost_per_call_usd)} -> ${fmtUSD(rankingMedianCost)}` : "—"}
                  meta={bestCostModel && rankingMedianCost != null && bestCostModel.avg_cost_per_call_usd > 0 ? `${t("llmAnalysis.diff.betweenCheapestAndMedian")} · ${(((rankingMedianCost - bestCostModel.avg_cost_per_call_usd) / bestCostModel.avg_cost_per_call_usd) * 100).toFixed(0)}% ${t("llmAnalysis.diff.moreThanCheapest")}` : "—"}
                />
                <InsightCard
                  title={`${rankingPurposeLabel} ${t("llmAnalysis.diff.selected")}`}
                  body={selectedRankingRow ? `${providerLabel(selectedRankingRow.provider)} / ${selectedRankingRow.model}` : t("llmAnalysis.diff.selectPlaceholder")}
                  meta={selectedRankingRow ? `${selectedRankingRow.vs_cheapest_pct >= 0 ? "+" : ""}${selectedRankingRow.vs_cheapest_pct.toFixed(0)}% ${t("llmAnalysis.diff.vsCheapestInPurpose")} · ${selectedRankingRow.vs_median_pct >= 0 ? "+" : ""}${selectedRankingRow.vs_median_pct.toFixed(0)}% ${t("llmAnalysis.diff.vsMedianInPurpose")}` : t("llmAnalysis.diff.selectedHelp")}
                />
              </div>
              <div className="mt-4 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">{t("llmAnalysis.section.qa")}</div>
                    <h3 className="mt-2 font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llmAnalysis.qa.title")}</h3>
                    <p className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.qa.description")}</p>
                  </div>
                  <div className="flex flex-wrap items-center justify-end gap-2">
                    {selectedRankingRow ? (
                      <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-2.5 py-1 text-xs text-[var(--color-editorial-ink-faint)]">
                        {providerLabel(selectedRankingRow.provider)} / {selectedRankingRow.model}
                      </span>
                    ) : null}
                    {selectedModelKey ? (
                      <button
                        type="button"
                        onClick={() => setSelectedModelKey(null)}
                        className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-1.5 text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:bg-[var(--color-editorial-panel)]"
                      >
                        {t("llmAnalysis.diff.clearSelection")}
                      </button>
                    ) : null}
                  </div>
                </div>
                {qaLoading ? (
                  <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>
                ) : !selectedRankingRow ? (
                  <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.qa.emptySelection")}</p>
                ) : qaSamples.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("llmAnalysis.qa.noSamples")}</p>
                ) : (
                  <div className="grid gap-3 lg:grid-cols-3">
                    {qaSamples.map((item) => (
                      <article key={item.id} className="rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-4 shadow-[var(--shadow-card)]">
                        <div className="text-xs text-[var(--color-editorial-ink-faint)]">
                          {item.summary_llm ? `${providerLabel(item.summary_llm.provider)} / ${item.summary_llm.model}` : providerLabel(selectedRankingRow.provider)}
                        </div>
                        <h4 className="mt-2 line-clamp-2 font-serif text-[1.05rem] leading-[1.35] text-[var(--color-editorial-ink)]">{item.translated_title || item.title || t("llmAnalysis.qa.untitled")}</h4>
                        <div className="mt-3 flex flex-wrap gap-1 text-[11px] text-[var(--color-editorial-ink-soft)]">
                          {item.summary ? <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5">summary</span> : null}
                          {item.facts?.facts?.length ? <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5">facts {item.facts.facts.length}</span> : null}
                          {item.facts_check?.final_result ? <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5">facts check {item.facts_check.final_result}</span> : null}
                          {item.faithfulness?.final_result ? <span className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-2 py-0.5">faithfulness {item.faithfulness.final_result}</span> : null}
                        </div>
                        {selectedRankingRow.purpose === "facts" || selectedRankingRow.purpose === "facts_check" ? (
                          <ul className="mt-3 space-y-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                            {(item.facts?.facts ?? []).slice(0, 4).map((fact, idx) => (
                              <li key={idx} className="line-clamp-2">• {fact}</li>
                            ))}
                          </ul>
                        ) : (
                          <p className="mt-3 line-clamp-6 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">{item.summary?.summary || t("llmAnalysis.qa.noSummary")}</p>
                        )}
                        <Link href={`/items/${item.id}`} className="mt-3 inline-flex text-xs font-medium text-[var(--color-editorial-ink-soft)] hover:text-[var(--color-editorial-ink)]">
                          {t("llmAnalysis.qa.openItem")}
                        </Link>
                      </article>
                    ))}
                  </div>
                )}
              </div>
              <div className="mt-4 overflow-x-auto rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                <table className="min-w-full border-separate border-spacing-0 text-sm">
                  <thead className="bg-[var(--color-editorial-panel)] text-[11px] text-[var(--color-editorial-ink-faint)]">
                    <tr>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.provider")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.purpose")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.model")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">calls</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">cost</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">avg/call</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">tokens/call</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.quality.failureRate")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.quality.retryRate")}</th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]">{t("llmAnalysis.quality.emptyRate")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rankingRows.length === 0 ? (
                      <tr>
                        <td colSpan={10} className="px-4 py-8 text-center text-sm text-[var(--color-editorial-ink-faint)]">
                          {t("llm.noSummary")}
                        </td>
                      </tr>
                    ) : (
                      rankingRows.slice(0, 18).map((row) => (
                        <tr
                          key={`${row.provider}:${row.model}:${row.purpose}`}
                          className="cursor-pointer transition hover:bg-[var(--color-editorial-panel)]"
                          onClick={() => applyRowFilter(row)}
                        >
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-[var(--color-editorial-ink)]">{providerLabel(row.provider)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-[var(--color-editorial-ink-soft)]">{row.purpose}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 whitespace-nowrap text-xs text-[var(--color-editorial-ink)]">{formatModelDisplayName(row.model)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink)]">{fmtNum(row.calls)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink)]">{fmtUSD(row.estimated_cost_usd)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtUSD(row.avg_cost_per_call_usd)}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtNum(Math.round(row.total_tokens_per_call))}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{row.failure_rate_pct == null ? "—" : `${row.failure_rate_pct.toFixed(1)}%`}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{row.retry_rate_pct == null ? "—" : `${row.retry_rate_pct.toFixed(1)}%`}</td>
                          <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{row.empty_rate_pct == null ? "—" : `${row.empty_rate_pct.toFixed(1)}%`}</td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </SectionCard>
          ) : null}

          {activeSection === "details" ? (
            <SectionCard>
              <SectionTitle eyebrow={t("llmAnalysis.section.details")} title={t("llmAnalysis.section.detailsTitle")} description={t("llmAnalysis.section.detailsDescription")} />
              <div className="mb-3 flex items-center justify-between">
                <h2 className="font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llmAnalysis.matrixTitle")}</h2>
                <span className="text-xs text-[var(--color-editorial-ink-faint)]">{sortedRows.length} rows</span>
              </div>
              <div className="overflow-x-auto rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)]">
                <table className="min-w-full border-separate border-spacing-0 text-sm">
                  <thead className="bg-[var(--color-editorial-panel)] text-[11px] text-[var(--color-editorial-ink-faint)]">
                    <tr>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("provider")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">{t("llmAnalysis.provider")}{sortMark("provider")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("model")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">{t("llmAnalysis.model")}{sortMark("model")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("purpose")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">{t("llmAnalysis.purpose")}{sortMark("purpose")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-left font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("pricing_source")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">{t("llmAnalysis.pricing")}{sortMark("pricing_source")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("calls")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">calls{sortMark("calls")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("avg_input_tokens_per_call")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">avg in/call{sortMark("avg_input_tokens_per_call")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("avg_output_tokens_per_call")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">avg out/call{sortMark("avg_output_tokens_per_call")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("avg_cost_per_call_usd")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">avg/call{sortMark("avg_cost_per_call_usd")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("cost_share_pct")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">share{sortMark("cost_share_pct")}</button></th>
                      <th className="border-b border-[var(--color-editorial-line-strong)] px-4 py-3 text-right font-medium uppercase tracking-[0.14em]"><button type="button" onClick={() => toggleSort("estimated_cost_usd")} className="inline-flex items-center hover:text-[var(--color-editorial-ink)]">cost{sortMark("estimated_cost_usd")}</button></th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedRows.map((row) => (
                      <tr key={`${row.provider}:${row.model}:${row.purpose}:${row.pricing_source}`} className="transition hover:bg-[var(--color-editorial-panel)]">
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-[var(--color-editorial-ink)]">{providerLabel(row.provider)}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 whitespace-nowrap text-xs text-[var(--color-editorial-ink)]">{formatModelDisplayName(row.model)}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-[var(--color-editorial-ink-soft)]">{row.purpose}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-xs text-[var(--color-editorial-ink-faint)]">{row.pricing_source}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink)]">{fmtNum(row.calls)}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtNum(Math.round(row.avg_input_tokens_per_call))}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtNum(Math.round(row.avg_output_tokens_per_call))}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{fmtUSD(row.avg_cost_per_call_usd)}</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink-soft)]">{row.cost_share_pct.toFixed(1)}%</td>
                        <td className="border-b border-[var(--color-editorial-line)] px-4 py-3.5 text-right tabular-nums text-[var(--color-editorial-ink)]">{fmtUSD(row.estimated_cost_usd)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </SectionCard>
          ) : null}
        </div>
      </div>
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
      <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">{eyebrow}</div>
      <h2 className="mt-2 font-serif text-[1.45rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{title}</h2>
      <p className="mt-2 text-[13px] leading-6 text-[var(--color-editorial-ink-soft)]">{description}</p>
    </div>
  );
}

function InsightCard({ title, body, meta, onClick }: { title: string; body: string; meta: string; onClick?: () => void }) {
  const Comp = onClick ? "button" : "div";
  return (
    <Comp
      {...(onClick ? { type: "button", onClick } : {})}
      className={joinClassNames(
        "rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-4 text-left shadow-[var(--shadow-card)]",
        onClick ? "transition hover:bg-[var(--color-editorial-panel)]" : ""
      )}
    >
      <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">{title}</div>
      <div className="mt-2 text-sm font-semibold leading-6 text-[var(--color-editorial-ink)]">{body}</div>
      <div className="mt-2 text-xs text-[var(--color-editorial-ink-soft)]">{meta}</div>
    </Comp>
  );
}
