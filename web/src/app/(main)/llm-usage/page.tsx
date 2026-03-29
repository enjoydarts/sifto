"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Brain, CalendarDays } from "lucide-react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  ReferenceLine,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { api, LLMExecutionCurrentMonthSummary, LLMUsageDailySummary, LLMUsageLog, LLMUsageModelSummary, LLMUsageProviderMonthSummary, LLMUsagePurposeMonthSummary, UserSettings } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import {
  CurrentMonthByProviderTable,
  CurrentMonthByPurposeTable,
  DailySummaryGroups,
  MetricCard,
  RecentLogsTable,
  ReliabilityTable,
} from "@/components/llm-usage/tables";
import { PageHeader } from "@/components/ui/page-header";
import { SectionCard } from "@/components/ui/section-card";

function fmtUSD(v: number) {
  return `$${v.toFixed(6)}`;
}

function fmtNum(v: number) {
  return new Intl.NumberFormat("ja-JP").format(v);
}

function fmtUSDShort(v: number) {
  if (v >= 1) return `$${v.toFixed(2)}`;
  if (v >= 0.01) return `$${v.toFixed(3)}`;
  return `$${v.toFixed(4)}`;
}

type SummaryRow = LLMUsageDailySummary & {
  key: string;
};

const PROVIDER_COLORS: Record<string, { stroke: string; fill: string; fillOpacity: number; label: string }> = {
  openai: { stroke: "#10b981", fill: "#34d399", fillOpacity: 0.65, label: "OpenAI" },
  anthropic: { stroke: "#3b82f6", fill: "#60a5fa", fillOpacity: 0.6, label: "Anthropic" },
  google: { stroke: "#f59e0b", fill: "#fbbf24", fillOpacity: 0.6, label: "Google" },
  groq: { stroke: "#8b5cf6", fill: "#a78bfa", fillOpacity: 0.55, label: "Groq" },
  deepseek: { stroke: "#ef4444", fill: "#f87171", fillOpacity: 0.55, label: "DeepSeek" },
  alibaba: { stroke: "#0f766e", fill: "#14b8a6", fillOpacity: 0.55, label: "Alibaba" },
  mistral: { stroke: "#be123c", fill: "#fb7185", fillOpacity: 0.55, label: "Mistral" },
  xai: { stroke: "#4338ca", fill: "#818cf8", fillOpacity: 0.55, label: "xAI" },
  zai: { stroke: "#0891b2", fill: "#22d3ee", fillOpacity: 0.55, label: "Z.ai" },
  fireworks: { stroke: "#c2410c", fill: "#fb923c", fillOpacity: 0.55, label: "Fireworks" },
  moonshot: { stroke: "#db2777", fill: "#f9a8d4", fillOpacity: 0.55, label: "Moonshot" },
  openrouter: { stroke: "#7c3aed", fill: "#c4b5fd", fillOpacity: 0.55, label: "OpenRouter" },
  poe: { stroke: "#0f766e", fill: "#2dd4bf", fillOpacity: 0.55, label: "Poe" },
  siliconflow: { stroke: "#2563eb", fill: "#93c5fd", fillOpacity: 0.55, label: "SiliconFlow" },
};

const FALLBACK_PROVIDER_COLORS = [
  { stroke: "#0f766e", fill: "#14b8a6", fillOpacity: 0.55 },
  { stroke: "#be123c", fill: "#fb7185", fillOpacity: 0.55 },
  { stroke: "#4338ca", fill: "#818cf8", fillOpacity: 0.55 },
  { stroke: "#a16207", fill: "#facc15", fillOpacity: 0.55 },
];

function normalizeProvider(provider: string) {
  const p = provider.trim().toLowerCase();
  if (p.startsWith("poe::") || p.startsWith("poe/")) {
    return "poe";
  }
  if (p.startsWith("siliconflow::") || p.startsWith("siliconflow/")) {
    return "siliconflow";
  }
  return p;
}

function providerLabel(provider: string) {
  return PROVIDER_COLORS[provider]?.label ?? provider;
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

function tooltipNameToText(name: ChartTooltipName | undefined) {
  return String(name ?? "");
}

type LLMUsageSectionID =
  | "overview"
  | "forecast"
  | "daily"
  | "providers"
  | "purposes"
  | "reliability"
  | "models"
  | "logs";

export default function LLMUsagePage() {
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

  const normalizedSummaryRows = useMemo(() => {
    return summaryRows.map((row) => ({
      ...row,
      provider: normalizeProvider(row.provider),
    }));
  }, [summaryRows]);

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

  const load = useCallback(async (daysParam: number, limitParam: number) => {
    setLoading(true);
    try {
      const [summary, byModel, byProviderCurrentMonth, byPurposeCurrentMonth, executionCurrentMonth, recent, userSettings] = await Promise.all([
        api.getLLMUsageSummary({ days: daysParam }),
        api.getLLMUsageByModel({ days: daysParam }),
        api.getLLMUsageCurrentMonthByProvider(),
        api.getLLMUsageCurrentMonthByPurpose(),
        api.getLLMExecutionCurrentMonthSummary(),
        api.getLLMUsage({ limit: limitParam }),
        api.getSettings(),
      ]);
      setSummaryRows(summary ?? []);
      setModelRows(byModel ?? []);
      setCurrentMonthProviderRows(byProviderCurrentMonth ?? []);
      setCurrentMonthPurposeRows(byPurposeCurrentMonth ?? []);
      setCurrentMonthExecutionRows(executionCurrentMonth ?? []);
      setLogs(recent ?? []);
      setSettings(userSettings);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    setLogPage(1);
    load(selectedDays, limit);
  }, [limit, load, selectedDays]);

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
    const monthProviders = new Set(currentMonthProviderRows.map((row) => row.provider));
    const selectedProviders = new Set(providerTotals.map((row) => row.provider));
    const allProviders = new Set<string>([...monthProviders, ...selectedProviders]);
    return Array.from(allProviders)
      .map((provider) => ({
        provider,
        selectedCost: totals.byProviderCost.get(provider) ?? 0,
        monthCost: currentMonthProviderRows.find((row) => row.provider === provider)?.estimated_cost_usd ?? 0,
      }))
      .sort((a, b) => {
        if (b.selectedCost !== a.selectedCost) return b.selectedCost - a.selectedCost;
        if (b.monthCost !== a.monthCost) return b.monthCost - a.monthCost;
        return a.provider.localeCompare(b.provider);
      });
  }, [currentMonthProviderRows, providerTotals, totals]);

  const currentMonthProviderTableRows = useMemo(() => {
    const total = settings?.current_month?.estimated_cost_usd ?? currentMonthProviderRows.reduce((acc, row) => acc + row.estimated_cost_usd, 0);
    const totalCalls = currentMonthProviderRows.reduce((acc, row) => acc + row.calls, 0);
    const dir = providerSortDir === "asc" ? 1 : -1;
    return currentMonthProviderRows.map((row) => ({
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
  }, [currentMonthProviderRows, providerSortDir, providerSortKey, settings]);

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
    const rows = currentMonthExecutionRows.filter((row) => row.attempts > 0);
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
  }, [currentMonthExecutionRows, reliabilitySortDir, reliabilitySortKey]);

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
    const map = new Map<string, { stroke: string; fill: string; fillOpacity: number; label: string }>();
    let fallbackIndex = 0;
    for (const provider of chartProviders) {
      const preset = PROVIDER_COLORS[provider];
      if (preset) {
        map.set(provider, preset);
        continue;
      }
      const fallback = FALLBACK_PROVIDER_COLORS[fallbackIndex % FALLBACK_PROVIDER_COLORS.length];
      fallbackIndex += 1;
      map.set(provider, { ...fallback, label: provider });
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
    return Array.from(months).sort((a, b) => b.localeCompare(a));
  }, [settings, summaryRows]);

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
          // Keep forecast as a daily cumulative trajectory; avoid injecting EOM total on "today".
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

  return (
    <div className="space-y-6 overflow-x-hidden">
      <PageHeader
        eyebrow={t("llm.title")}
        title={t("llm.title")}
        titleIcon={Brain}
        description={t("llm.subtitle")}
        compact
        meta={
          <>
            <span>{`${t("llm.currentMonth")}: ${settings?.current_month?.month_jst ?? "—"}`}</span>
            <span>{`${t("llm.totalCost")}: ${fmtUSD(totals.cost)}`}</span>
            <span>{`${t("llm.totalCalls")}: ${fmtNum(totals.calls)}`}</span>
          </>
        }
        actions={
          <div className="grid w-full grid-cols-2 gap-2 sm:flex sm:w-auto sm:flex-wrap sm:justify-end">
            <label className="text-sm">
              <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llm.days")}</span>
              <select
                value={daysFilter}
                onChange={(e) => setDaysFilter(e.target.value as "7" | "14" | "30" | "90" | "mtd")}
                className="min-h-10 w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm text-[var(--color-editorial-ink)]"
              >
                {(["7", "14", "30", "90"] as const).map((d) => (
                  <option key={d} value={d}>
                    {`${d}${t("llm.daysSuffix")}`}
                  </option>
                ))}
                <option value="mtd">{t("llm.currentMonth")}</option>
              </select>
            </label>
            <label className="text-sm">
              <span className="mb-1 block text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("llm.limit")}</span>
              <select
                value={limit}
                onChange={(e) => setLimit(Number(e.target.value))}
                className="min-h-10 w-full rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-4 py-2 text-sm text-[var(--color-editorial-ink)]"
              >
                {[50, 100, 200, 500].map((v) => (
                  <option key={v} value={v}>
                    {v}
                  </option>
                ))}
              </select>
            </label>
          </div>
        }
      />

      {loading && <p className="text-sm text-[var(--color-editorial-ink-soft)]">{t("common.loading")}</p>}
      {error && (
        <div className="rounded-[16px] border border-[var(--color-editorial-error-line)] bg-[var(--color-editorial-error-soft)] px-4 py-3 text-sm text-[var(--color-editorial-error)]">
          {error}
        </div>
      )}

      <div className="grid gap-6 xl:grid-cols-[248px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <SectionCard>
            <div className="text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--color-editorial-ink-faint)]">
              Usage Sections
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
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmUsage.status.currentMonth")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {settings?.current_month?.month_jst ?? "—"} / {fmtUSD(settings?.current_month?.estimated_cost_usd ?? 0)}
                </div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmUsage.status.budget")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {monthlyForecast?.budget == null ? "—" : `${fmtUSD(monthlyForecast.budget)} / ${fmtUSD(monthlyForecast.forecastTotal)}`}
                </div>
              </div>
              <div className="rounded-[16px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-4 py-3">
                <div className="text-sm font-semibold text-[var(--color-editorial-ink)]">{t("llmUsage.status.reliability")}</div>
                <div className="mt-1 text-xs leading-5 text-[var(--color-editorial-ink-soft)]">
                  {currentMonthExecutionTableRows.length} rows / {fmtNum(logs.length)} logs
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
                <MetricCard label={totalCostLabel} value={fmtUSD(totals.cost)} />
                <MetricCard label={t("llm.totalCalls")} value={fmtNum(totals.calls)} />
                <MetricCard label={t("llm.input")} value={fmtNum(totals.input)} />
                <MetricCard label={t("llm.output")} value={fmtNum(totals.output)} />
              </section>

              <section className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(0,1.2fr)]">
                <SectionCard>
                  <div className="mb-3 flex items-center justify-between">
                    <h2 className="font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llm.providerCost")}</h2>
                    <span className="text-xs text-[var(--color-editorial-ink-faint)]">
                      {providerCardRows.length} {t("llm.providers")}
                    </span>
                  </div>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 2xl:grid-cols-3">
                    {providerCardRows.map((row) => (
                      <MetricCard
                        key={row.provider}
                        className="w-full"
                        label={`${providerLabel(row.provider)}${row.selectedCost <= 0 && row.monthCost > 0 ? " (MTD)" : ""}`}
                        value={fmtUSD(row.selectedCost > 0 ? row.selectedCost : row.monthCost)}
                      />
                    ))}
                  </div>
                </SectionCard>

                <SectionCard>
                  <div className="mb-3 flex items-center justify-between">
                    <h2 className="font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">{t("llm.cacheTitle")}</h2>
                    <span className="text-xs text-[var(--color-editorial-ink-faint)]">{totals.input > 0 ? ((totals.cacheRead / totals.input) * 100).toFixed(1) : "0.0"}%</span>
                  </div>
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 xl:grid-cols-1 2xl:grid-cols-3">
                    <MetricCard className="w-full" label={t("llm.cacheWriteTokens")} value={fmtNum(totals.cacheWrite)} />
                    <MetricCard className="w-full" label={t("llm.cacheReadTokens")} value={fmtNum(totals.cacheRead)} />
                    <MetricCard
                      className="w-full"
                      label={t("llm.cacheReadRatio")}
                      value={`${totals.input > 0 ? ((totals.cacheRead / totals.input) * 100).toFixed(1) : "0.0"}%`}
                    />
                  </div>
                </SectionCard>
              </section>

              <SectionCard>
                <div className="mb-3 flex items-center justify-between">
                  <h2 className="inline-flex items-center gap-2 font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                    <CalendarDays className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
                    <span>{t("llm.dailyCostTrend")}</span>
                  </h2>
                  <span className="text-xs text-[var(--color-editorial-ink-faint)]">{dailyChartRows.length} days</span>
                </div>
                {dailyChartRows.length === 0 ? (
                  <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("llm.noSummary")}</p>
                ) : (
                  <div className="h-72 w-full overflow-visible rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3">
                    <ResponsiveContainer width="100%" height="100%">
                      <AreaChart data={dailyChartRows} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" vertical={false} />
                        <XAxis dataKey="date" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} />
                        <YAxis
                          tick={{ fontSize: 12, fill: "#8f877f" }}
                          tickLine={false}
                          axisLine={false}
                          tickFormatter={(v) => fmtUSDShort(Number(v))}
                        />
                        <Tooltip
                          formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [
                            fmtUSD(tooltipValueToNumber(value)),
                            providerLabel(tooltipNameToText(name)),
                          ]}
                          labelFormatter={(label) => `${label}`}
                          contentStyle={{ borderRadius: 16, borderColor: "#d9d1c4", background: "#fff" }}
                        />
                        {chartProviders.map((provider) => {
                          const colors = providerColorMap.get(provider);
                          if (!colors) return null;
                          return (
                            <Area
                              key={provider}
                              type="monotone"
                              dataKey={provider}
                              name={providerLabel(provider)}
                              stackId="cost"
                              stroke={colors.stroke}
                              fill={colors.fill}
                              fillOpacity={colors.fillOpacity}
                            />
                          );
                        })}
                      </AreaChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </SectionCard>
            </>
          ) : null}

          {activeSection === "forecast" ? (
            <SectionCard>
              <div className="mb-3 flex items-center justify-between">
                <h2 className="inline-flex items-center gap-2 font-serif text-[1.25rem] leading-[1.15] tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                  <CalendarDays className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
                  <span>{t("llm.monthEndForecast")}</span>
                </h2>
                <div className="flex items-center gap-2">
                  <select
                    value={forecastMonth ?? ""}
                    onChange={(e) => setForecastMonth(e.target.value)}
                    className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] px-3 py-1.5 text-xs text-[var(--color-editorial-ink)]"
                  >
                    {availableForecastMonths.map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                  </select>
                  <div className="grid min-w-0 grid-cols-2 rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-0.5 text-xs">
                    <button
                      type="button"
                      onClick={() => setForecastMode("month_avg")}
                      className={joinClassNames(
                        "min-w-0 rounded-full px-2 py-1 text-center sm:px-3",
                        forecastMode === "month_avg" ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]" : "text-[var(--color-editorial-ink-soft)]"
                      )}
                    >
                      {t("llm.forecast.refMonthAvg")}
                    </button>
                    <button
                      type="button"
                      onClick={() => setForecastMode("recent_7d")}
                      className={joinClassNames(
                        "min-w-0 rounded-full px-2 py-1 text-center sm:px-3",
                        forecastMode === "recent_7d" ? "bg-[var(--color-editorial-ink)] text-[var(--color-editorial-panel-strong)]" : "text-[var(--color-editorial-ink-soft)]"
                      )}
                    >
                      {t("llm.forecast.refRecent7d")}
                    </button>
                  </div>
                </div>
              </div>
              {!monthlyForecast ? (
                <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("common.loading")}</p>
              ) : (
                <div className="space-y-4">
                  <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 lg:grid-cols-4">
                    <MetricCard className="w-full" label={t("llm.monthToDate")} value={fmtUSD(monthlyForecast.actualTotal)} />
                    <MetricCard className="w-full" label={monthlyForecast.isCurrentMonth ? t("llm.forecastEom") : t("llm.monthTotal")} value={fmtUSD(monthlyForecast.forecastTotal)} />
                    <MetricCard className="w-full" label={t("llm.currentPacePerDay")} value={fmtUSD(monthlyForecast.dailyPace)} />
                    <MetricCard
                      className="w-full"
                      label={t("llm.budgetDelta")}
                      value={
                        monthlyForecast.budget == null
                          ? "—"
                          : `${monthlyForecast.forecastTotal - monthlyForecast.budget >= 0 ? "+" : ""}${fmtUSD(monthlyForecast.forecastTotal - monthlyForecast.budget)}`
                      }
                    />
                  </div>
                  <p className="text-xs text-[var(--color-editorial-ink-soft)]">
                    {monthlyForecast.isCurrentMonth
                      ? `${t("llm.forecast.modeLabel")} ${forecastMode === "month_avg" ? t("llm.forecast.monthAvg") : t("llm.forecast.recent7d")}${t("llm.forecast.refOpen")}${t("llm.forecast.refMonthAvg")} ${fmtUSD(monthlyForecast.monthAvgDailyPace)} / ${t("llm.forecast.refRecent7d")} ${fmtUSD(monthlyForecast.recent7dDailyPace)}${t("llm.forecast.refClose")}`
                      : t("llm.forecast.pastMonthsOnly")}
                  </p>
                  <div className="h-80 w-full overflow-visible rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-3">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={monthlyForecast.rows} margin={{ top: 8, right: 16, left: 8, bottom: 0 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" vertical={false} />
                        <XAxis dataKey="label" tick={{ fontSize: 12, fill: "#8f877f" }} tickLine={false} axisLine={false} />
                        <YAxis
                          tick={{ fontSize: 12, fill: "#8f877f" }}
                          tickLine={false}
                          axisLine={false}
                          tickFormatter={(v) => fmtUSDShort(Number(v))}
                        />
                        <Tooltip
                          formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [
                            fmtUSD(tooltipValueToNumber(value)),
                            name === "actual" ? t("llm.actualCumulative") : t("llm.forecastLabel"),
                          ]}
                          labelFormatter={(label) => `${monthlyForecast.monthLabel}-${String(label).padStart(2, "0")}`}
                          contentStyle={{ borderRadius: 16, borderColor: "#d9d1c4", background: "#fff" }}
                        />
                        {monthlyForecast.budget != null && (
                          <ReferenceLine
                            y={monthlyForecast.budget}
                            stroke="#c05032"
                            strokeDasharray="5 5"
                            label={{
                              value: `${t("llm.budget")} ${fmtUSDShort(monthlyForecast.budget)}`,
                              fill: "#c05032",
                              fontSize: 11,
                              position: "insideTopRight",
                            }}
                          />
                        )}
                        <Line type="monotone" dataKey="actual" name="actual" stroke="#171412" strokeWidth={2.5} dot={false} connectNulls={false} />
                        {monthlyForecast.isCurrentMonth ? (
                          <Line type="monotone" dataKey="forecast" name="forecast" stroke="#275d8a" strokeWidth={2} strokeDasharray="6 4" dot={false} connectNulls={false} />
                        ) : null}
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              )}
            </SectionCard>
          ) : null}

          {activeSection === "daily" ? (
            <SectionCard>
              <DailySummaryGroups
                title={t("llm.dailySummary")}
                groupedByDate={groupedByDate}
                noSummaryLabel={t("llm.noSummary")}
                fmtNum={fmtNum}
                fmtUSD={fmtUSD}
              />
            </SectionCard>
          ) : null}

          {activeSection === "providers" ? (
            <CurrentMonthByProviderTable
              title={t("llm.currentMonthByProvider")}
              rows={currentMonthProviderTableRows}
              monthLabel={settings?.current_month?.month_jst ?? currentMonthProviderRows[0]?.month_jst ?? "—"}
              totalCostLabel={fmtUSD(settings?.current_month?.estimated_cost_usd ?? 0)}
              noSummaryLabel={t("llm.noSummary")}
              fmtNum={fmtNum}
              fmtUSD={fmtUSD}
              sortKey={providerSortKey}
              sortDir={providerSortDir}
              onSort={(key) => toggleSort(key, providerSortKey, setProviderSortKey, setProviderSortDir)}
            />
          ) : null}

          {activeSection === "purposes" ? (
            <CurrentMonthByPurposeTable
              title={t("llm.currentMonthByPurpose")}
              rows={currentMonthPurposeTableRows}
              monthLabel={settings?.current_month?.month_jst ?? currentMonthPurposeRows[0]?.month_jst ?? "—"}
              noSummaryLabel={t("llm.noSummary")}
              fmtNum={fmtNum}
              fmtUSD={fmtUSD}
              sortKey={purposeSortKey}
              sortDir={purposeSortDir}
              onSort={(key) => toggleSort(key, purposeSortKey, setPurposeSortKey, setPurposeSortDir)}
            />
          ) : null}

          {activeSection === "reliability" ? (
            <ReliabilityTable
              rows={currentMonthExecutionTableRows}
              monthLabel={settings?.current_month?.month_jst ?? currentMonthExecutionRows[0]?.month_jst ?? "—"}
              noSummaryLabel={t("llm.noSummary")}
              fmtNum={fmtNum}
              fmtUSD={fmtUSD}
              sortKey={reliabilitySortKey}
              sortDir={reliabilitySortDir}
              onSort={handleReliabilitySort}
              labels={{
                title: t("llm.currentMonthReliability"),
                attempts: t("llm.attempts"),
                cost: t("llm.totalCost"),
                failures: t("llm.failures"),
                failureRate: t("llm.failureRate"),
                retries: t("llm.retries"),
                retryRate: t("llm.retryRate"),
                emptyResponses: t("llm.emptyResponses"),
                emptyRate: t("llm.emptyRate"),
              }}
            />
          ) : null}

          {activeSection === "models" ? (
            <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <Brain className="size-4 text-zinc-500" aria-hidden="true" />
            <span>{t("llm.usageByModel")}</span>
          </h2>
          <span className="text-xs text-zinc-400">{mergedModelRows.length} models</span>
        </div>
        {mergedModelRows.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("llm.noSummary")}</p>
        ) : (
          <div className="space-y-4">
            <div className="h-80 w-full overflow-visible rounded border border-zinc-100 p-2">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart
                  data={modelChartRows}
                  layout="vertical"
                  margin={{ top: 8, right: 24, left: 8, bottom: 8 }}
                >
                  <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" horizontal={true} vertical={false} />
                  <XAxis
                    type="number"
                    tick={{ fontSize: 12, fill: "#71717a" }}
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(v) => fmtUSDShort(Number(v))}
                  />
                  <YAxis
                    type="category"
                    dataKey="shortLabel"
                    width={220}
                    tick={{ fontSize: 12, fill: "#3f3f46" }}
                    tickLine={false}
                    axisLine={false}
                  />
                  <Tooltip
                    formatter={(value: ChartTooltipValue | undefined, name?: ChartTooltipName) => [
                      name === "calls" ? fmtNum(tooltipValueToNumber(value)) : fmtUSD(tooltipValueToNumber(value)),
                      tooltipNameToText(name),
                    ]}
                    labelFormatter={(_, payload) => {
                      const row = payload?.[0]?.payload as { label?: string; pricingSource?: string } | undefined;
                      if (!row) return "";
                      return `${row.label} (${row.pricingSource ?? ""})`;
                    }}
                    contentStyle={{ borderRadius: 10, borderColor: "#e4e4e7" }}
                  />
                  <Bar dataKey="cost" name="Cost (USD)" fill="#18181b" radius={[0, 4, 4, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="text-xs text-zinc-500">
                  <tr className="border-b border-zinc-100">
                    <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("provider", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">provider{modelSortKey === "provider" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("model", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">model{modelSortKey === "model" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => toggleSort("pricing_source", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">pricing{modelSortKey === "pricing_source" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("calls", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">calls{modelSortKey === "calls" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("input_tokens", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">input{modelSortKey === "input_tokens" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("output_tokens", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">output{modelSortKey === "output_tokens" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("avg_input_tokens_per_call", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">avg in/call{modelSortKey === "avg_input_tokens_per_call" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("avg_output_tokens_per_call", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">avg out/call{modelSortKey === "avg_output_tokens_per_call" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("cache_creation_input_tokens", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">cache w{modelSortKey === "cache_creation_input_tokens" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("cache_read_input_tokens", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">cache r{modelSortKey === "cache_read_input_tokens" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("avg_cost_per_call_usd", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">avg/call{modelSortKey === "avg_cost_per_call_usd" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                    <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => toggleSort("estimated_cost_usd", modelSortKey, setModelSortKey, setModelSortDir)} className="inline-flex items-center hover:text-zinc-800">cost{modelSortKey === "estimated_cost_usd" ? <span className="ml-1 text-zinc-400">{modelSortDir === "asc" ? "↑" : "↓"}</span> : null}</button></th>
                  </tr>
                </thead>
                <tbody>
                  {mergedModelRows.map((r) => (
                    <tr key={`${r.provider}:${r.model}:${r.pricing_source}`} className="border-b border-zinc-100 last:border-0">
                      <td className="px-3 py-2">{providerLabel(r.provider)}</td>
                      <td className="px-3 py-2 text-xs whitespace-nowrap">{r.model}</td>
                      <td className="px-3 py-2 text-xs">
                        <div className="relative inline-flex items-center gap-1">
                          <span>{r.pricing_source}</span>
                          {"pricing_sources" in r &&
                            Array.isArray(r.pricing_sources) &&
                            r.pricing_sources.length > 1 && (
                              <span className="group relative inline-flex">
                                <button
                                  type="button"
                                  className="inline-flex size-4 items-center justify-center rounded-full border border-zinc-300 text-[10px] leading-none text-zinc-500 hover:bg-zinc-50"
                                  aria-label="pricing source breakdown"
                                >
                                  i
                                </button>
                                <span className="pointer-events-none absolute left-1/2 top-full z-20 mt-1 hidden w-72 -translate-x-1/2 rounded-md border border-zinc-200 bg-white p-2 text-[11px] text-zinc-700 shadow-lg group-hover:block">
                                  <span className="mb-1 block font-medium text-zinc-900">Pricing sources</span>
                                  <span className="block whitespace-pre-line">
                                    {r.pricing_sources.join("\n")}
                                  </span>
                                </span>
                              </span>
                            )}
                        </div>
                      </td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.calls)}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.input_tokens)}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.output_tokens)}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(Math.round(r.avg_input_tokens_per_call ?? 0))}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(Math.round(r.avg_output_tokens_per_call ?? 0))}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.cache_creation_input_tokens)}</td>
                      <td className="px-3 py-2 text-right">{fmtNum(r.cache_read_input_tokens)}</td>
                      <td className="px-3 py-2 text-right">{fmtUSD(r.calls > 0 ? r.estimated_cost_usd / r.calls : 0)}</td>
                      <td className="px-3 py-2 text-right">{fmtUSD(r.estimated_cost_usd)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
            </section>
          ) : null}

          {activeSection === "logs" ? (
            <RecentLogsTable
              logs={sortedLogs}
              pagedLogs={pagedLogs}
              logPage={logPage}
              setLogPage={setLogPage}
              logsPageSize={logsPageSize}
              locale={locale}
              noLogsLabel={t("llm.noLogs")}
              labels={{ title: t("llm.recentLogs"), time: t("llm.time") }}
              fmtNum={fmtNum}
              fmtUSD={fmtUSD}
              sortKey={logSortKey}
              sortDir={logSortDir}
              onSort={(key) => toggleSort(key, logSortKey, setLogSortKey, setLogSortDir)}
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}
