"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, ItemDetail, LLMExecutionCurrentMonthSummary, LLMUsageAnalysisSummary } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { type ModelOption } from "@/components/settings/model-select";
import { formatModelDisplayName, providerLabel } from "@/lib/model-display";

export function fmtUSD(v: number) {
  return `$${v.toFixed(6)}`;
}

export type AnalysisRow = LLMUsageAnalysisSummary & {
  avg_input_tokens_per_call: number;
  avg_output_tokens_per_call: number;
  avg_cost_per_call_usd: number;
  cost_share_pct: number;
};

export type ExecutionRow = LLMExecutionCurrentMonthSummary;

export type RankedModelRow = {
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

export type UsageScatterRow = AnalysisRow & {
  totalTokensPerCall: number;
  avgCostPerCall: number;
  bubbleSize: number;
  costCallBubbleSize: number;
  label: string;
  pricingLabel: string;
};

export type LLMAnalysisSectionID =
  | "overview"
  | "charts"
  | "mix"
  | "quality"
  | "recommend"
  | "details";

export function useLLMAnalysisPageData() {
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
  const seqRef = useRef(0);

  const load = useCallback(async (selectedDays: number) => {
    const seq = ++seqRef.current;
    setLoading(true);
    try {
      const [nextRows, nextExecutionRows] = await Promise.all([
        api.getLLMUsageAnalysis({ days: selectedDays }),
        api.getLLMExecutionCurrentMonthSummary({ days: selectedDays }),
      ]);
      if (seq !== seqRef.current) return;
      setRows(nextRows ?? []);
      setExecutionRows(nextExecutionRows ?? []);
      setError(null);
    } catch (e) {
      if (seq !== seqRef.current) return;
      setError(String(e));
    } finally {
      if (seq === seqRef.current) {
        setLoading(false);
      }
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

  return {
    t,
    activeSection,
    setActiveSection,
    days,
    setDays,
    providerFilter,
    setProviderFilter,
    purposeFilter,
    setPurposeFilter,
    scatterPurpose,
    setScatterPurpose,
    rankingPurpose,
    setRankingPurpose,
    modelQuery,
    setModelQuery,
    selectedModelKey,
    setSelectedModelKey,
    loading,
    error,
    providers,
    purposes,
    modelOptions,
    sortedRows,
    totals,
    scatterRows,
    providerMixRows,
    rankingRows,
    bestCostModel,
    bestTokenModel,
    bestQualityModel,
    rankingMedianCost,
    rankingPurposeLabel,
    rankingModelOptions,
    selectedRankingRow,
    toggleSort,
    sortMark,
    applyRowFilter,
    clearFilters,
    hasActiveDrilldown,
    hasScatterFilter,
    clearScatterFilters,
    railSections,
    activeSectionTitle,
    activeSectionDescription,
    qaLoading,
    qaSamples,
  };
}
