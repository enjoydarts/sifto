"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  api,
  LLMExecutionCurrentMonthSummary,
  LLMUsageDailySummary,
  LLMUsageLog,
  LLMUsageModelSummary,
  LLMUsageProviderMonthSummary,
  LLMUsagePurposeMonthSummary,
  UserSettings,
} from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { normalizeProvider } from "@/lib/model-display";

type SummaryRow = LLMUsageDailySummary & {
  key: string;
};

export type LLMUsageSectionID =
  | "overview"
  | "forecast"
  | "daily"
  | "providers"
  | "purposes"
  | "reliability"
  | "models"
  | "logs";

const PROVIDER_COLORS: Record<string, { stroke: string; fill: string; fillOpacity: number }> = {
  openai: { stroke: "#10b981", fill: "#34d399", fillOpacity: 0.65 },
  anthropic: { stroke: "#3b82f6", fill: "#60a5fa", fillOpacity: 0.6 },
  google: { stroke: "#f59e0b", fill: "#fbbf24", fillOpacity: 0.6 },
  groq: { stroke: "#8b5cf6", fill: "#a78bfa", fillOpacity: 0.55 },
  deepseek: { stroke: "#ef4444", fill: "#f87171", fillOpacity: 0.55 },
  alibaba: { stroke: "#0f766e", fill: "#14b8a6", fillOpacity: 0.55 },
  minimax: { stroke: "#65a30d", fill: "#a3e635", fillOpacity: 0.55 },
  mistral: { stroke: "#be123c", fill: "#fb7185", fillOpacity: 0.55 },
  together: { stroke: "#0f766e", fill: "#5eead4", fillOpacity: 0.55 },
  xai: { stroke: "#4338ca", fill: "#818cf8", fillOpacity: 0.55 },
  zai: { stroke: "#0891b2", fill: "#22d3ee", fillOpacity: 0.55 },
  fireworks: { stroke: "#c2410c", fill: "#fb923c", fillOpacity: 0.55 },
  moonshot: { stroke: "#db2777", fill: "#f9a8d4", fillOpacity: 0.55 },
  openrouter: { stroke: "#7c3aed", fill: "#c4b5fd", fillOpacity: 0.55 },
  poe: { stroke: "#0f766e", fill: "#2dd4bf", fillOpacity: 0.55 },
  siliconflow: { stroke: "#2563eb", fill: "#93c5fd", fillOpacity: 0.55 },
};

const FALLBACK_PROVIDER_COLORS = [
  { stroke: "#0f766e", fill: "#14b8a6", fillOpacity: 0.55 },
  { stroke: "#be123c", fill: "#fb7185", fillOpacity: 0.55 },
  { stroke: "#4338ca", fill: "#818cf8", fillOpacity: 0.55 },
  { stroke: "#a16207", fill: "#facc15", fillOpacity: 0.55 },
];

export function fmtUSD(v: number) {
  return `$${v.toFixed(6)}`;
}

export function fmtNum(v: number) {
  return new Intl.NumberFormat("ja-JP").format(v);
}

export function fmtUSDShort(v: number) {
  if (v >= 1) return `$${v.toFixed(2)}`;
  if (v >= 0.01) return `$${v.toFixed(3)}`;
  return `$${v.toFixed(4)}`;
}

export function useLLMUsageData() {
  const { t, locale } = useI18n();
  const [activeSection, setActiveSection] = useState<LLMUsageSectionID>("overview");
  const [providerSortKey, setProviderSortKey] = useState<string>("estimated_cost_usd");
  const [providerSortDir, setProviderSortDir] = useState<"asc" | "desc">("desc");
  const [purposeSortKey, setPurposeSortKey] = useState<string>("estimated_cost_usd");
  const [purposeSortDir, setPurposeSortDir] = useState<"asc" | "desc">("desc");
  const [reliabilitySortKey, setReliabilitySortKey] = useState<string>("failures");
  const [reliabilitySortDir, setReliabilitySortDir] = useState<"asc" | "desc">("desc");
  const [modelSortKey, setModelSortKey] = useState<string>("estimated_cost_usd");
  const [modelSortDir, setModelSortDir] = useState<"asc" | "desc">("desc");
  const [logSortKey, setLogSortKey] = useState<string>("created_at");
  const [logSortDir, setLogSortDir] = useState<"asc" | "desc">("desc");
  const [forecastMode, setForecastMode] = useState<"month_avg" | "recent_7d">("month_avg");
  const [forecastMonth, setForecastMonth] = useState<string | null>(null);
  const [selectedMonth, setSelectedMonth] = useState<string | null>(null);
  const [daysFilter, setDaysFilter] = useState<"7" | "14" | "30" | "90" | "mtd">("mtd");
  const [limit, setLimit] = useState(100);
  const [logPage, setLogPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [summaryRows, setSummaryRows] = useState<LLMUsageDailySummary[]>([]);
  const [modelRows, setModelRows] = useState<LLMUsageModelSummary[]>([]);
  const [currentMonthProviderRows, setCurrentMonthProviderRows] = useState<LLMUsageProviderMonthSummary[]>([]);
  const [currentMonthPurposeRows, setCurrentMonthPurposeRows] = useState<LLMUsagePurposeMonthSummary[]>([]);
  const [currentMonthExecutionRows, setCurrentMonthExecutionRows] = useState<LLMExecutionCurrentMonthSummary[]>([]);
  const [logs, setLogs] = useState<LLMUsageLog[]>([]);
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const seqRef = useRef(0);

  const normalizedSummaryRows = useMemo(() => {
    return summaryRows.map((row) => ({
      ...row,
      provider: normalizeProvider(row.provider),
    }));
  }, [summaryRows]);
  const normalizedCurrentMonthProviderRows = useMemo(() => {
    return currentMonthProviderRows.map((row) => ({
      ...row,
      provider: normalizeProvider(row.provider),
    }));
  }, [currentMonthProviderRows]);
  const normalizedCurrentMonthExecutionRows = useMemo(() => {
    return currentMonthExecutionRows.map((row) => ({
      ...row,
      provider: normalizeProvider(row.provider),
    }));
  }, [currentMonthExecutionRows]);

  const selectedDays = useMemo(() => {
    if (daysFilter !== "mtd") {
      return Number(daysFilter);
    }
    const now = new Date();
    const jstNow = new Date(now.toLocaleString("en-US", { timeZone: "Asia/Tokyo" }));
    return jstNow.getDate();
  }, [daysFilter]);

  const totalCostLabel = useMemo(() => {
    return daysFilter === "mtd" ? t("llm.monthToDate") : t("llm.totalCost");
  }, [daysFilter, t]);

  const currentJSTMonth = useMemo(() => {
    const now = new Date();
    const jstNow = new Date(now.toLocaleString("en-US", { timeZone: "Asia/Tokyo" }));
    return `${jstNow.getFullYear()}-${String(jstNow.getMonth() + 1).padStart(2, "0")}`;
  }, []);

  const previousJSTMonth = useMemo(() => {
    const now = new Date();
    const jstNow = new Date(now.toLocaleString("en-US", { timeZone: "Asia/Tokyo" }));
    const prev = new Date(jstNow.getFullYear(), jstNow.getMonth() - 1, 1);
    return `${prev.getFullYear()}-${String(prev.getMonth() + 1).padStart(2, "0")}`;
  }, []);

  useEffect(() => {
    if (!selectedMonth) {
      setSelectedMonth(currentJSTMonth);
    }
  }, [currentJSTMonth, selectedMonth]);

  const load = useCallback(async (daysParam: number, limitParam: number, monthParam: string) => {
    const seq = ++seqRef.current;
    setLoading(true);
    try {
      const [summary, byModel, byProviderCurrentMonth, byPurposeCurrentMonth, executionCurrentMonth, recent, userSettings] = await Promise.all([
        api.getLLMUsageSummary({ days: daysParam }),
        api.getLLMUsageByModel({ days: daysParam }),
        api.getLLMUsageCurrentMonthByProvider({ month: monthParam }),
        api.getLLMUsageCurrentMonthByPurpose({ month: monthParam }),
        api.getLLMExecutionCurrentMonthSummary({ month: monthParam }),
        api.getLLMUsage({ limit: limitParam }),
        api.getSettings(),
      ]);
      if (seq !== seqRef.current) return;
      setSummaryRows(summary ?? []);
      setModelRows(byModel ?? []);
      setCurrentMonthProviderRows(byProviderCurrentMonth ?? []);
      setCurrentMonthPurposeRows(byPurposeCurrentMonth ?? []);
      setCurrentMonthExecutionRows(executionCurrentMonth ?? []);
      setLogs(recent ?? []);
      setSettings(userSettings);
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
    setLogPage(1);
    if (!selectedMonth) return;
    load(selectedDays, limit, selectedMonth);
  }, [limit, load, selectedDays, selectedMonth]);

  const totals = useMemo(() => {
    const t = {
      calls: 0,
      input: 0,
      output: 0,
      cacheWrite: 0,
      cacheRead: 0,
      cost: 0,
      byProviderCost: new Map<string, number>(),
    };
    for (const r of normalizedSummaryRows) {
      t.calls += r.calls;
      t.input += r.input_tokens;
      t.output += r.output_tokens;
      t.cacheWrite += r.cache_creation_input_tokens;
      t.cacheRead += r.cache_read_input_tokens;
      t.cost += r.estimated_cost_usd;
      t.byProviderCost.set(r.provider, (t.byProviderCost.get(r.provider) ?? 0) + r.estimated_cost_usd);
    }
    return t;
  }, [normalizedSummaryRows]);

  const providerTotals = useMemo(() => {
    return Array.from(totals.byProviderCost.entries())
      .map(([provider, cost]) => ({ provider, cost }))
      .sort((a, b) => {
        if (b.cost !== a.cost) return b.cost - a.cost;
        return a.provider.localeCompare(b.provider);
      });
  }, [totals]);

  const providerCardRows = useMemo(() => {
    const monthProviders = new Set(normalizedCurrentMonthProviderRows.map((row) => row.provider));
    const selectedProviders = new Set(providerTotals.map((row) => row.provider));
    const allProviders = new Set<string>([...monthProviders, ...selectedProviders]);
    return Array.from(allProviders)
      .map((provider) => ({
        provider,
        selectedCost: totals.byProviderCost.get(provider) ?? 0,
        monthCost: normalizedCurrentMonthProviderRows.find((row) => row.provider === provider)?.estimated_cost_usd ?? 0,
      }))
      .sort((a, b) => {
        if (b.selectedCost !== a.selectedCost) return b.selectedCost - a.selectedCost;
        if (b.monthCost !== a.monthCost) return b.monthCost - a.monthCost;
        return a.provider.localeCompare(b.provider);
      });
  }, [normalizedCurrentMonthProviderRows, providerTotals, totals]);

  const currentMonthProviderTableRows = useMemo(() => {
    const total = settings?.current_month?.estimated_cost_usd ?? normalizedCurrentMonthProviderRows.reduce((acc, row) => acc + row.estimated_cost_usd, 0);
    const totalCalls = normalizedCurrentMonthProviderRows.reduce((acc, row) => acc + row.calls, 0);
    const dir = providerSortDir === "asc" ? 1 : -1;
    return normalizedCurrentMonthProviderRows.map((row) => ({
      ...row,
      share_pct: total > 0 ? (row.estimated_cost_usd / total) * 100 : 0,
      call_share_pct: totalCalls > 0 ? (row.calls / totalCalls) * 100 : 0,
      avg_cost_per_call_usd: row.calls > 0 ? row.estimated_cost_usd / row.calls : 0,
      avg_input_tokens_per_call: row.calls > 0 ? row.input_tokens / row.calls : 0,
      avg_output_tokens_per_call: row.calls > 0 ? row.output_tokens / row.calls : 0,
    })).sort((a, b) => {
      const av = a as Record<string, string | number>;
      const bv = b as Record<string, string | number>;
      const aVal = av[providerSortKey];
      const bVal = bv[providerSortKey];
      let cmp = 0;
      if (typeof aVal === "number" && typeof bVal === "number") cmp = aVal - bVal;
      else cmp = String(aVal ?? "").localeCompare(String(bVal ?? ""));
      if (cmp !== 0) return cmp * dir;
      return a.provider.localeCompare(b.provider);
    });
  }, [normalizedCurrentMonthProviderRows, providerSortDir, providerSortKey, settings]);

  const currentMonthPurposeTableRows = useMemo(() => {
    const totalCost = currentMonthPurposeRows.reduce((acc, row) => acc + row.estimated_cost_usd, 0);
    const totalCalls = currentMonthPurposeRows.reduce((acc, row) => acc + row.calls, 0);
    const dir = purposeSortDir === "asc" ? 1 : -1;
    return currentMonthPurposeRows.map((row) => ({
      ...row,
      call_share_pct: totalCalls > 0 ? (row.calls / totalCalls) * 100 : 0,
      share_pct: totalCost > 0 ? (row.estimated_cost_usd / totalCost) * 100 : 0,
      avg_cost_per_call_usd: row.calls > 0 ? row.estimated_cost_usd / row.calls : 0,
      avg_input_tokens_per_call: row.calls > 0 ? row.input_tokens / row.calls : 0,
      avg_output_tokens_per_call: row.calls > 0 ? row.output_tokens / row.calls : 0,
    })).sort((a, b) => {
      const av = a as Record<string, string | number>;
      const bv = b as Record<string, string | number>;
      const aVal = av[purposeSortKey];
      const bVal = bv[purposeSortKey];
      let cmp = 0;
      if (typeof aVal === "number" && typeof bVal === "number") cmp = aVal - bVal;
      else cmp = String(aVal ?? "").localeCompare(String(bVal ?? ""));
      if (cmp !== 0) return cmp * dir;
      return a.purpose.localeCompare(b.purpose);
    });
  }, [currentMonthPurposeRows, purposeSortDir, purposeSortKey]);

  const currentMonthExecutionTableRows = useMemo(() => {
    const rows = normalizedCurrentMonthExecutionRows.filter((row) => row.attempts > 0);
    const dir = reliabilitySortDir === "asc" ? 1 : -1;
    return rows.sort((a, b) => {
      const modelA = `${a.provider}/${a.model}`;
      const modelB = `${b.provider}/${b.model}`;
      let cmp = 0;
      switch (reliabilitySortKey) {
        case "purpose":
          cmp = a.purpose.localeCompare(b.purpose);
          break;
        case "model":
          cmp = modelA.localeCompare(modelB);
          break;
        case "attempts":
          cmp = a.attempts - b.attempts;
          break;
        case "estimated_cost_usd":
          cmp = a.estimated_cost_usd - b.estimated_cost_usd;
          break;
        case "failures":
          cmp = a.failures - b.failures;
          break;
        case "failure_rate_pct":
          cmp = a.failure_rate_pct - b.failure_rate_pct;
          break;
        case "retries":
          cmp = a.retries - b.retries;
          break;
        case "retry_rate_pct":
          cmp = a.retry_rate_pct - b.retry_rate_pct;
          break;
        case "empty_responses":
          cmp = a.empty_responses - b.empty_responses;
          break;
        case "empty_rate_pct":
          cmp = a.empty_rate_pct - b.empty_rate_pct;
          break;
        default:
          cmp = a.failures - b.failures;
          break;
      }
      if (cmp !== 0) return cmp * dir;
      if (a.purpose !== b.purpose) return a.purpose.localeCompare(b.purpose);
      if (a.provider !== b.provider) return a.provider.localeCompare(b.provider);
      return a.model.localeCompare(b.model);
    });
  }, [normalizedCurrentMonthExecutionRows, reliabilitySortDir, reliabilitySortKey]);

  const handleReliabilitySort = useCallback((key: string) => {
    if (reliabilitySortKey === key) {
      setReliabilitySortDir((dir) => (dir === "asc" ? "desc" : "asc"));
      return;
    }
    setReliabilitySortKey(key);
    setReliabilitySortDir(key === "purpose" || key === "model" ? "asc" : "desc");
  }, [reliabilitySortKey]);

  const groupedByDate = useMemo(() => {
    const m = new Map<string, SummaryRow[]>();
    for (const row of normalizedSummaryRows) {
      const key = `${row.date_jst}:${row.provider}:${row.purpose}:${row.pricing_source}`;
      const list = m.get(row.date_jst) ?? [];
      list.push({ ...row, key });
      m.set(row.date_jst, list);
    }
    return Array.from(m.entries());
  }, [normalizedSummaryRows]);

  const dailyChartRows = useMemo(() => {
    const m = new Map<string, { date: string; total: number; [provider: string]: string | number }>();
    for (const row of normalizedSummaryRows) {
      const cur = m.get(row.date_jst) ?? { date: row.date_jst, total: 0 };
      cur.total = Number(cur.total ?? 0) + row.estimated_cost_usd;
      cur[row.provider] = Number(cur[row.provider] ?? 0) + row.estimated_cost_usd;
      m.set(row.date_jst, cur);
    }
    const providers = Array.from(
      normalizedSummaryRows.reduce((acc, row) => {
        acc.add(row.provider);
        return acc;
      }, new Set<string>())
    );
    return Array.from(m.values())
      .map((row) => {
        for (const provider of providers) {
          if (row[provider] == null) {
            row[provider] = 0;
          }
        }
        return row;
      })
      .sort((a, b) => a.date.localeCompare(b.date));
  }, [normalizedSummaryRows]);

  const chartProviders = useMemo(() => {
    return Array.from(totals.byProviderCost.keys()).sort((a, b) => {
      const costDiff = (totals.byProviderCost.get(b) ?? 0) - (totals.byProviderCost.get(a) ?? 0);
      if (costDiff !== 0) return costDiff;
      return a.localeCompare(b);
    });
  }, [totals]);

  const providerColorMap = useMemo(() => {
    const map = new Map<string, { stroke: string; fill: string; fillOpacity: number }>();
    let fallbackIndex = 0;
    for (const provider of chartProviders) {
      const preset = PROVIDER_COLORS[provider];
      if (preset) {
        map.set(provider, preset);
        continue;
      }
      const fallback = FALLBACK_PROVIDER_COLORS[fallbackIndex % FALLBACK_PROVIDER_COLORS.length];
      fallbackIndex += 1;
      map.set(provider, fallback);
    }
    return map;
  }, [chartProviders]);

  const mergedModelRows = useMemo(() => {
    const m = new Map<string, LLMUsageModelSummary & {
      pricing_sources: string[];
      avg_input_tokens_per_call?: number;
      avg_output_tokens_per_call?: number;
    }>();
    for (const r of modelRows) {
      const key = `${r.provider}:${r.model}`;
      const cur = m.get(key);
      if (!cur) {
        m.set(key, {
          ...r,
          pricing_sources: [r.pricing_source],
        });
        continue;
      }
      cur.calls += r.calls;
      cur.input_tokens += r.input_tokens;
      cur.output_tokens += r.output_tokens;
      cur.cache_creation_input_tokens += r.cache_creation_input_tokens;
      cur.cache_read_input_tokens += r.cache_read_input_tokens;
      cur.estimated_cost_usd += r.estimated_cost_usd;
      if (!cur.pricing_sources.includes(r.pricing_source)) {
        cur.pricing_sources.push(r.pricing_source);
      }
      cur.pricing_source =
        cur.pricing_sources.length === 1 ? cur.pricing_sources[0] : `mixed(${cur.pricing_sources.length})`;
    }
    for (const cur of m.values()) {
      cur.avg_input_tokens_per_call = cur.calls > 0 ? cur.input_tokens / cur.calls : 0;
      cur.avg_output_tokens_per_call = cur.calls > 0 ? cur.output_tokens / cur.calls : 0;
    }
    const dir = modelSortDir === "asc" ? 1 : -1;
    return Array.from(m.values()).sort((a, b) => {
      let cmp = 0;
      switch (modelSortKey) {
        case "provider":
          cmp = a.provider.localeCompare(b.provider);
          break;
        case "model":
          cmp = a.model.localeCompare(b.model);
          break;
        case "pricing_source":
          cmp = a.pricing_source.localeCompare(b.pricing_source);
          break;
        case "calls":
        case "input_tokens":
        case "output_tokens":
        case "avg_input_tokens_per_call":
        case "avg_output_tokens_per_call":
        case "cache_creation_input_tokens":
        case "cache_read_input_tokens":
        case "estimated_cost_usd":
          cmp = (a[modelSortKey] as number) - (b[modelSortKey] as number);
          break;
        case "avg_cost_per_call_usd":
          cmp = (a.calls > 0 ? a.estimated_cost_usd / a.calls : 0) - (b.calls > 0 ? b.estimated_cost_usd / b.calls : 0);
          break;
      }
      if (cmp !== 0) return cmp * dir;
      if (a.provider !== b.provider) return a.provider.localeCompare(b.provider);
      return a.model.localeCompare(b.model);
    });
  }, [modelRows, modelSortDir, modelSortKey]);

  const availableForecastMonths = useMemo(() => {
    const months = new Set<string>();
    for (const r of summaryRows) {
      if (r.date_jst.length >= 7) months.add(r.date_jst.slice(0, 7));
    }
    if (settings?.current_month?.month_jst) months.add(settings.current_month.month_jst);
    months.add(currentJSTMonth);
    months.add(previousJSTMonth);
    return Array.from(months).sort((a, b) => b.localeCompare(a));
  }, [currentJSTMonth, previousJSTMonth, settings, summaryRows]);

  const monthOptions = useMemo(() => {
    return [currentJSTMonth, previousJSTMonth]
      .filter((value, index, array) => array.indexOf(value) === index)
      .map((value) => ({
        value,
        label: value === currentJSTMonth ? t("llm.currentMonth") : t("llm.previousMonth"),
      }));
  }, [currentJSTMonth, previousJSTMonth, t]);

  useEffect(() => {
    if (availableForecastMonths.length === 0) return;
    if (!forecastMonth || !availableForecastMonths.includes(forecastMonth)) {
      setForecastMonth(availableForecastMonths[0]);
    }
  }, [availableForecastMonths, forecastMonth]);

  const modelChartRows = useMemo(
    () =>
      mergedModelRows
        .slice()
        .reverse()
        .map((r) => ({
          key: `${r.provider}:${r.model}:${r.pricing_source}`,
          label: `${r.provider}/${r.model}`,
          shortLabel: `${r.provider}:${r.model.length > 28 ? `${r.model.slice(0, 28)}…` : r.model}`,
          cost: r.estimated_cost_usd,
          calls: r.calls,
          pricingSource: r.pricing_source,
        })),
    [mergedModelRows]
  );

  const sortedLogs = useMemo(() => {
    const dir = logSortDir === "asc" ? 1 : -1;
    return logs.slice().sort((a, b) => {
      let cmp = 0;
      switch (logSortKey) {
        case "created_at":
          cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
          break;
        case "purpose":
          cmp = a.purpose.localeCompare(b.purpose);
          break;
        case "model":
          cmp = a.model.localeCompare(b.model);
          break;
        case "pricing_source":
          cmp = a.pricing_source.localeCompare(b.pricing_source);
          break;
        case "input_tokens":
        case "output_tokens":
        case "cache_creation_input_tokens":
        case "cache_read_input_tokens":
        case "estimated_cost_usd":
          cmp = (a[logSortKey] as number) - (b[logSortKey] as number);
          break;
        case "ref": {
          const aRef = `${a.item_id ?? ""}${a.digest_id ?? ""}`;
          const bRef = `${b.item_id ?? ""}${b.digest_id ?? ""}`;
          cmp = aRef.localeCompare(bRef);
          break;
        }
      }
      if (cmp !== 0) return cmp * dir;
      return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
    });
  }, [logSortDir, logSortKey, logs]);

  const logsPageSize = 20;
  const pagedLogs = sortedLogs.slice((logPage - 1) * logsPageSize, logPage * logsPageSize);

  const toggleSort = useCallback((
    key: string,
    currentKey: string,
    setKey: (v: string) => void,
    setDir: (v: "asc" | "desc" | ((prev: "asc" | "desc") => "asc" | "desc")) => void,
  ) => {
    if (currentKey === key) {
      setDir((prev) => (prev === "asc" ? "desc" : "asc"));
      return;
    }
    setKey(key);
    setDir(key === "provider" || key === "purpose" || key === "model" || key === "pricing_source" || key === "ref" ? "asc" : "desc");
  }, []);

  const monthlyForecast = useMemo(() => {
    if (!forecastMonth) return null;
    const now = new Date();
    const jstNow = new Date(now.toLocaleString("en-US", { timeZone: "Asia/Tokyo" }));
    const currentMonthKey = `${jstNow.getFullYear()}-${String(jstNow.getMonth() + 1).padStart(2, "0")}`;
    const [yearStr, monthStr] = forecastMonth.split("-");
    const year = Number(yearStr);
    const month = Number(monthStr);
    if (!Number.isFinite(year) || !Number.isFinite(month) || month < 1 || month > 12) return null;
    const isCurrentMonth = forecastMonth === currentMonthKey;
    const daysInMonth = new Date(year, month, 0).getDate();
    const today = isCurrentMonth ? jstNow.getDate() : daysInMonth;

    const monthPrefix = `${year}-${String(month).padStart(2, "0")}-`;
    const costByDate = new Map<string, number>();
    for (const r of summaryRows) {
      if (!r.date_jst.startsWith(monthPrefix)) continue;
      costByDate.set(r.date_jst, (costByDate.get(r.date_jst) ?? 0) + r.estimated_cost_usd);
    }

    let cumulative = 0;
    const rows: Array<{ day: number; label: string; actual: number | null; forecast: number | null }> = [];
    const actualTotal = isCurrentMonth
      ? (settings?.current_month?.estimated_cost_usd ?? 0)
      : Array.from(costByDate.values()).reduce((acc, v) => acc + v, 0);
    const monthAvgDailyPace = today > 0 ? actualTotal / today : 0;

    let recent7dSum = 0;
    let recent7dCount = 0;
    for (let d = Math.max(1, today - 6); d <= today; d += 1) {
      const date = `${monthPrefix}${String(d).padStart(2, "0")}`;
      recent7dSum += costByDate.get(date) ?? 0;
      recent7dCount += 1;
    }
    const recent7dDailyPace = recent7dCount > 0 ? recent7dSum / recent7dCount : monthAvgDailyPace;
    const selectedDailyPace = forecastMode === "recent_7d" ? recent7dDailyPace : monthAvgDailyPace;
    const forecastTotal = isCurrentMonth ? selectedDailyPace * daysInMonth : actualTotal;

    for (let d = 1; d <= daysInMonth; d += 1) {
      const date = `${monthPrefix}${String(d).padStart(2, "0")}`;
      if (d <= today) {
        cumulative += costByDate.get(date) ?? 0;
        rows.push({
          day: d,
          label: String(d),
          actual: cumulative,
          forecast: isCurrentMonth ? selectedDailyPace * d : null,
        });
      } else {
        rows.push({
          day: d,
          label: String(d),
          actual: null,
          forecast: isCurrentMonth ? selectedDailyPace * d : null,
        });
      }
    }

    return {
      monthLabel: forecastMonth,
      rows,
      today,
      daysInMonth,
      isCurrentMonth,
      actualTotal,
      dailyPace: selectedDailyPace,
      monthAvgDailyPace,
      recent7dDailyPace,
      forecastTotal,
      budget: isCurrentMonth ? (settings?.monthly_budget_usd ?? null) : null,
    };
  }, [forecastMode, forecastMonth, settings, summaryRows]);

  const railSections = useMemo(
    () => [
      {
        id: "overview" as const,
        label: t("llmUsage.section.overview"),
        meta: t("llmUsage.section.overviewMeta"),
      },
      {
        id: "forecast" as const,
        label: t("llmUsage.section.forecast"),
        meta: t("llmUsage.section.forecastMeta"),
      },
      {
        id: "daily" as const,
        label: t("llmUsage.section.daily"),
        meta: t("llmUsage.section.dailyMeta"),
      },
      {
        id: "providers" as const,
        label: t("llmUsage.section.providers"),
        meta: t("llmUsage.section.providersMeta"),
      },
      {
        id: "purposes" as const,
        label: t("llmUsage.section.purposes"),
        meta: t("llmUsage.section.purposesMeta"),
      },
      {
        id: "reliability" as const,
        label: t("llmUsage.section.reliability"),
        meta: t("llmUsage.section.reliabilityMeta"),
      },
      {
        id: "models" as const,
        label: t("llmUsage.section.models"),
        meta: t("llmUsage.section.modelsMeta"),
      },
      {
        id: "logs" as const,
        label: t("llmUsage.section.logs"),
        meta: t("llmUsage.section.logsMeta"),
      },
    ],
    [t]
  );

  const activeSectionTitle = useMemo(() => {
    switch (activeSection) {
      case "overview":
        return t("llmUsage.active.overviewTitle");
      case "forecast":
        return t("llmUsage.active.forecastTitle");
      case "daily":
        return t("llmUsage.active.dailyTitle");
      case "providers":
        return t("llmUsage.active.providersTitle");
      case "purposes":
        return t("llmUsage.active.purposesTitle");
      case "reliability":
        return t("llmUsage.active.reliabilityTitle");
      case "models":
        return t("llmUsage.active.modelsTitle");
      case "logs":
        return t("llmUsage.active.logsTitle");
    }
  }, [activeSection, t]);

  const activeSectionDescription = useMemo(() => {
    switch (activeSection) {
      case "overview":
        return t("llmUsage.active.overviewDescription");
      case "forecast":
        return t("llmUsage.active.forecastDescription");
      case "daily":
        return t("llmUsage.active.dailyDescription");
      case "providers":
        return t("llmUsage.active.providersDescription");
      case "purposes":
        return t("llmUsage.active.purposesDescription");
      case "reliability":
        return t("llmUsage.active.reliabilityDescription");
      case "models":
        return t("llmUsage.active.modelsDescription");
      case "logs":
        return t("llmUsage.active.logsDescription");
    }
  }, [activeSection, t]);

  return {
    t, locale,
    activeSection, setActiveSection,
    daysFilter, setDaysFilter,
    limit, setLimit,
    forecastMode, setForecastMode,
    forecastMonth, setForecastMonth,
    selectedMonth, setSelectedMonth,
    monthOptions,
    logPage, setLogPage,
    providerSortKey, setProviderSortKey,
    providerSortDir, setProviderSortDir,
    purposeSortKey, setPurposeSortKey,
    purposeSortDir, setPurposeSortDir,
    reliabilitySortKey, setReliabilitySortKey,
    reliabilitySortDir, setReliabilitySortDir,
    modelSortKey, setModelSortKey,
    modelSortDir, setModelSortDir,
    logSortKey, setLogSortKey,
    logSortDir, setLogSortDir,
    loading, error,
    settings,
    totals,
    providerCardRows,
    currentMonthProviderTableRows,
    currentMonthPurposeTableRows,
    currentMonthExecutionTableRows,
    groupedByDate,
    dailyChartRows,
    chartProviders,
    providerColorMap,
    mergedModelRows,
    modelChartRows,
    sortedLogs,
    pagedLogs,
    logsPageSize,
    availableForecastMonths,
    monthlyForecast,
    railSections,
    activeSectionTitle,
    activeSectionDescription,
    totalCostLabel,
    handleReliabilitySort,
    toggleSort,
  };
}
