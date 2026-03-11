"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Brain, CalendarDays } from "lucide-react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
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
};

const FALLBACK_PROVIDER_COLORS = [
  { stroke: "#0f766e", fill: "#14b8a6", fillOpacity: 0.55 },
  { stroke: "#be123c", fill: "#fb7185", fillOpacity: 0.55 },
  { stroke: "#4338ca", fill: "#818cf8", fillOpacity: 0.55 },
  { stroke: "#a16207", fill: "#facc15", fillOpacity: 0.55 },
];

function providerLabel(provider: string) {
  return PROVIDER_COLORS[provider]?.label ?? provider;
}

export default function LLMUsagePage() {
  const { t, locale } = useI18n();
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
    for (const r of summaryRows) {
      t.calls += r.calls;
      t.input += r.input_tokens;
      t.output += r.output_tokens;
      t.cacheWrite += r.cache_creation_input_tokens;
      t.cacheRead += r.cache_read_input_tokens;
      t.cost += r.estimated_cost_usd;
      t.byProviderCost.set(r.provider, (t.byProviderCost.get(r.provider) ?? 0) + r.estimated_cost_usd);
    }
    return t;
  }, [summaryRows]);

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
    return currentMonthProviderRows.map((row) => ({
      ...row,
      share_pct: total > 0 ? (row.estimated_cost_usd / total) * 100 : 0,
      call_share_pct: totalCalls > 0 ? (row.calls / totalCalls) * 100 : 0,
      avg_cost_per_call_usd: row.calls > 0 ? row.estimated_cost_usd / row.calls : 0,
    }));
  }, [currentMonthProviderRows, settings]);

  const currentMonthPurposeTableRows = useMemo(() => {
    const totalCost = currentMonthPurposeRows.reduce((acc, row) => acc + row.estimated_cost_usd, 0);
    const totalCalls = currentMonthPurposeRows.reduce((acc, row) => acc + row.calls, 0);
    return currentMonthPurposeRows.map((row) => ({
      ...row,
      call_share_pct: totalCalls > 0 ? (row.calls / totalCalls) * 100 : 0,
      share_pct: totalCost > 0 ? (row.estimated_cost_usd / totalCost) * 100 : 0,
      avg_cost_per_call_usd: row.calls > 0 ? row.estimated_cost_usd / row.calls : 0,
    }));
  }, [currentMonthPurposeRows]);

  const currentMonthExecutionTableRows = useMemo(
    () =>
      currentMonthExecutionRows
        .filter((row) => row.attempts > 0)
        .sort((a, b) => {
          if (b.failures !== a.failures) return b.failures - a.failures;
          if (b.retries !== a.retries) return b.retries - a.retries;
          if (b.attempts !== a.attempts) return b.attempts - a.attempts;
          if (a.purpose !== b.purpose) return a.purpose.localeCompare(b.purpose);
          if (a.provider !== b.provider) return a.provider.localeCompare(b.provider);
          return a.model.localeCompare(b.model);
        }),
    [currentMonthExecutionRows]
  );

  const groupedByDate = useMemo(() => {
    const m = new Map<string, SummaryRow[]>();
    for (const row of summaryRows) {
      const key = `${row.date_jst}:${row.provider}:${row.purpose}:${row.pricing_source}`;
      const list = m.get(row.date_jst) ?? [];
      list.push({ ...row, key });
      m.set(row.date_jst, list);
    }
    return Array.from(m.entries());
  }, [summaryRows]);

  const dailyChartRows = useMemo(() => {
    const m = new Map<string, { date: string; total: number; [provider: string]: string | number }>();
    for (const row of summaryRows) {
      const cur = m.get(row.date_jst) ?? { date: row.date_jst, total: 0 };
      cur.total = Number(cur.total ?? 0) + row.estimated_cost_usd;
      cur[row.provider] = Number(cur[row.provider] ?? 0) + row.estimated_cost_usd;
      m.set(row.date_jst, cur);
    }
    const providers = Array.from(
      summaryRows.reduce((acc, row) => {
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
  }, [summaryRows]);

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

  const visibleModelRows = useMemo(
    () => modelRows.filter((r) => r.estimated_cost_usd > 0),
    [modelRows]
  );

  const mergedModelRows = useMemo(() => {
    const m = new Map<string, LLMUsageModelSummary & { pricing_sources: string[] }>();
    for (const r of visibleModelRows) {
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
    return Array.from(m.values()).sort((a, b) => {
      if (b.estimated_cost_usd !== a.estimated_cost_usd) return b.estimated_cost_usd - a.estimated_cost_usd;
      if (b.calls !== a.calls) return b.calls - a.calls;
      if (a.provider !== b.provider) return a.provider.localeCompare(b.provider);
      return a.model.localeCompare(b.model);
    });
  }, [visibleModelRows]);

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

  const logsPageSize = 20;
  const pagedLogs = logs.slice((logPage - 1) * logsPageSize, logPage * logsPageSize);

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

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            <Brain className="size-6 text-zinc-500" aria-hidden="true" />
            <span>{t("llm.title")}</span>
          </h1>
          <p className="mt-1 text-sm text-zinc-500">
            {t("llm.subtitle")}
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-zinc-600">{t("llm.days")}</span>
            <select
              value={daysFilter}
              onChange={(e) => setDaysFilter(e.target.value as "7" | "14" | "30" | "90" | "mtd")}
              className="rounded border border-zinc-300 bg-white px-3 py-2 text-sm"
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
            <span className="mb-1 block text-xs font-medium text-zinc-600">{t("llm.limit")}</span>
            <select
              value={limit}
              onChange={(e) => setLimit(Number(e.target.value))}
              className="rounded border border-zinc-300 bg-white px-3 py-2 text-sm"
            >
              {[50, 100, 200, 500].map((v) => (
                <option key={v} value={v}>
                  {v}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>

      {loading && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard label={totalCostLabel} value={fmtUSD(totals.cost)} />
        <MetricCard label={t("llm.totalCalls")} value={fmtNum(totals.calls)} />
        <MetricCard label={t("llm.input")} value={fmtNum(totals.input)} />
        <MetricCard label={t("llm.output")} value={fmtNum(totals.output)} />
      </section>

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(0,1.2fr)]">
        <div className="rounded-lg border border-zinc-200 bg-white p-4">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-zinc-800">Provider Cost</h2>
            <span className="text-xs text-zinc-400">{providerCardRows.length} providers</span>
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
        </div>

        <div className="rounded-lg border border-zinc-200 bg-white p-4">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-zinc-800">Cache</h2>
            <span className="text-xs text-zinc-400">{totals.input > 0 ? ((totals.cacheRead / totals.input) * 100).toFixed(1) : "0.0"}% read</span>
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 xl:grid-cols-1 2xl:grid-cols-3">
            <MetricCard className="w-full" label="Cache Write Tokens" value={fmtNum(totals.cacheWrite)} />
            <MetricCard className="w-full" label="Cache Read Tokens" value={fmtNum(totals.cacheRead)} />
            <MetricCard
              className="w-full"
              label="Cache Read Ratio"
              value={`${totals.input > 0 ? ((totals.cacheRead / totals.input) * 100).toFixed(1) : "0.0"}%`}
            />
          </div>
        </div>
      </section>

      <CurrentMonthByProviderTable
        title={t("llm.currentMonthByProvider")}
        rows={currentMonthProviderTableRows}
        monthLabel={settings?.current_month?.month_jst ?? currentMonthProviderRows[0]?.month_jst ?? "—"}
        totalCostLabel={fmtUSD(settings?.current_month?.estimated_cost_usd ?? 0)}
        noSummaryLabel={t("llm.noSummary")}
        fmtNum={fmtNum}
        fmtUSD={fmtUSD}
      />

      <CurrentMonthByPurposeTable
        title={t("llm.currentMonthByPurpose")}
        rows={currentMonthPurposeTableRows}
        monthLabel={settings?.current_month?.month_jst ?? currentMonthPurposeRows[0]?.month_jst ?? "—"}
        noSummaryLabel={t("llm.noSummary")}
        fmtNum={fmtNum}
        fmtUSD={fmtUSD}
      />

      <ReliabilityTable
        rows={currentMonthExecutionTableRows}
        monthLabel={settings?.current_month?.month_jst ?? currentMonthExecutionRows[0]?.month_jst ?? "—"}
        noSummaryLabel={t("llm.noSummary")}
        fmtNum={fmtNum}
        labels={{
          title: t("llm.currentMonthReliability"),
          attempts: t("llm.attempts"),
          failures: t("llm.failures"),
          failureRate: t("llm.failureRate"),
          retries: t("llm.retries"),
          retryRate: t("llm.retryRate"),
          emptyResponses: t("llm.emptyResponses"),
          emptyRate: t("llm.emptyRate"),
        }}
      />

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <CalendarDays className="size-4 text-zinc-500" aria-hidden="true" />
            <span>{t("llm.monthEndForecast")}</span>
          </h2>
          <div className="flex items-center gap-2">
            <select
              value={forecastMonth ?? ""}
              onChange={(e) => setForecastMonth(e.target.value)}
              className="rounded border border-zinc-300 bg-white px-2 py-1 text-xs"
            >
              {availableForecastMonths.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </select>
            <div className="inline-flex rounded-md border border-zinc-200 bg-zinc-50 p-0.5 text-xs">
              <button
                type="button"
                onClick={() => setForecastMode("month_avg")}
                className={`rounded px-2 py-1 ${forecastMode === "month_avg" ? "bg-white text-zinc-900 shadow-sm" : "text-zinc-500"}`}
              >
                {t("llm.forecast.refMonthAvg")}
              </button>
              <button
                type="button"
                onClick={() => setForecastMode("recent_7d")}
                className={`rounded px-2 py-1 ${forecastMode === "recent_7d" ? "bg-white text-zinc-900 shadow-sm" : "text-zinc-500"}`}
              >
                {t("llm.forecast.refRecent7d")}
              </button>
            </div>
            <span className="text-xs text-zinc-400">{monthlyForecast?.monthLabel ?? "—"}</span>
          </div>
        </div>
        {!monthlyForecast ? (
          <p className="text-sm text-zinc-400">{t("common.loading")}</p>
        ) : (
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 lg:grid-cols-4">
              <MetricCard className="w-full sm:w-full lg:w-full" label={t("llm.monthToDate")} value={fmtUSD(monthlyForecast.actualTotal)} />
              <MetricCard
                className="w-full sm:w-full lg:w-full"
                label={
                  monthlyForecast.isCurrentMonth
                    ? t("llm.forecastEom")
                    : t("llm.monthTotal")
                }
                value={fmtUSD(monthlyForecast.forecastTotal)}
              />
              <MetricCard className="w-full sm:w-full lg:w-full" label={t("llm.currentPacePerDay")} value={fmtUSD(monthlyForecast.dailyPace)} />
              <MetricCard
                className="w-full sm:w-full lg:w-full"
                label={t("llm.budgetDelta")}
                value={
                  monthlyForecast.budget == null
                    ? "—"
                    : `${monthlyForecast.forecastTotal - monthlyForecast.budget >= 0 ? "+" : ""}${fmtUSD(
                        monthlyForecast.forecastTotal - monthlyForecast.budget
                      )}`
                }
              />
            </div>
            <p className="text-xs text-zinc-500">
              {monthlyForecast.isCurrentMonth
                ? `${t("llm.forecast.modeLabel")} ${forecastMode === "month_avg" ? t("llm.forecast.monthAvg") : t("llm.forecast.recent7d")}${t("llm.forecast.refOpen")}${t("llm.forecast.refMonthAvg")} ${fmtUSD(monthlyForecast.monthAvgDailyPace)} / ${t("llm.forecast.refRecent7d")} ${fmtUSD(monthlyForecast.recent7dDailyPace)}${t("llm.forecast.refClose")}`
                : t("llm.forecast.pastMonthsOnly")}
            </p>
            <div className="h-80 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={monthlyForecast.rows} margin={{ top: 8, right: 16, left: 8, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" vertical={false} />
                  <XAxis dataKey="label" tick={{ fontSize: 12, fill: "#71717a" }} tickLine={false} axisLine={false} />
                  <YAxis
                    tick={{ fontSize: 12, fill: "#71717a" }}
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(v) => fmtUSDShort(Number(v))}
                  />
                  <Tooltip
                    formatter={(value: number | string | undefined, name?: string) => [
                      fmtUSD(Number(value ?? 0)),
                      name === "actual"
                        ? t("llm.actualCumulative")
                        : t("llm.forecastLabel"),
                    ]}
                    labelFormatter={(label) => `${monthlyForecast.monthLabel}-${String(label).padStart(2, "0")}`}
                    contentStyle={{ borderRadius: 10, borderColor: "#e4e4e7" }}
                  />
                  <Legend
                    wrapperStyle={{ fontSize: 12 }}
                    formatter={(value) =>
                      value === "actual"
                        ? t("llm.actualCumulative")
                        : value === "forecast"
                          ? t("llm.forecastLabel")
                          : value
                    }
                  />
                  {monthlyForecast.budget != null && (
                    <ReferenceLine
                      y={monthlyForecast.budget}
                      stroke="#ef4444"
                      strokeDasharray="5 5"
                      label={{
                        value: `${t("llm.budget")} ${fmtUSDShort(monthlyForecast.budget)}`,
                        fill: "#ef4444",
                        fontSize: 11,
                        position: "insideTopRight",
                      }}
                    />
                  )}
                  <Line
                    type="monotone"
                    dataKey="actual"
                    name="actual"
                    stroke="#18181b"
                    strokeWidth={2.5}
                    dot={false}
                    connectNulls={false}
                  />
                  {monthlyForecast.isCurrentMonth && (
                    <Line
                      type="monotone"
                      dataKey="forecast"
                      name="forecast"
                      stroke="#2563eb"
                      strokeWidth={2}
                      strokeDasharray="6 4"
                      dot={false}
                      connectNulls={false}
                    />
                  )}
                </LineChart>
              </ResponsiveContainer>
            </div>
          </div>
        )}
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <CalendarDays className="size-4 text-zinc-500" aria-hidden="true" />
            <span>{t("llm.dailyCostTrend")}</span>
          </h2>
          <span className="text-xs text-zinc-400">{dailyChartRows.length} days</span>
        </div>
        {dailyChartRows.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("llm.noSummary")}</p>
        ) : (
          <div className="h-72 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={dailyChartRows} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" vertical={false} />
                <XAxis dataKey="date" tick={{ fontSize: 12, fill: "#71717a" }} tickLine={false} axisLine={false} />
                <YAxis
                  tick={{ fontSize: 12, fill: "#71717a" }}
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(v) => fmtUSDShort(Number(v))}
                />
                <Tooltip
                  formatter={(value: number | string | undefined, name?: string) => [
                    fmtUSD(Number(value ?? 0)),
                    providerLabel(name ?? ""),
                  ]}
                  labelFormatter={(label) => `${label}`}
                  contentStyle={{ borderRadius: 10, borderColor: "#e4e4e7" }}
                />
                <Legend wrapperStyle={{ fontSize: 12 }} />
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
      </section>

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
            <div className="h-80 w-full rounded border border-zinc-100 p-2">
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
                    formatter={(value: number | string | undefined, name?: string) => [
                      name === "calls" ? fmtNum(Number(value ?? 0)) : fmtUSD(Number(value ?? 0)),
                      name ?? "",
                    ]}
                    labelFormatter={(_, payload) => {
                      const row = payload?.[0]?.payload as { label?: string; pricingSource?: string } | undefined;
                      if (!row) return "";
                      return `${row.label} (${row.pricingSource ?? ""})`;
                    }}
                    contentStyle={{ borderRadius: 10, borderColor: "#e4e4e7" }}
                  />
                  <Legend wrapperStyle={{ fontSize: 12 }} />
                  <Bar dataKey="cost" name="Cost (USD)" fill="#18181b" radius={[0, 4, 4, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="text-xs text-zinc-500">
                  <tr className="border-b border-zinc-100">
                    <th className="px-3 py-2 text-left font-medium">provider</th>
                    <th className="px-3 py-2 text-left font-medium">model</th>
                    <th className="px-3 py-2 text-left font-medium">pricing</th>
                    <th className="px-3 py-2 text-right font-medium">calls</th>
                    <th className="px-3 py-2 text-right font-medium">input</th>
                    <th className="px-3 py-2 text-right font-medium">output</th>
                    <th className="px-3 py-2 text-right font-medium">cache w</th>
                    <th className="px-3 py-2 text-right font-medium">cache r</th>
                    <th className="px-3 py-2 text-right font-medium">avg/call</th>
                    <th className="px-3 py-2 text-right font-medium">cost</th>
                  </tr>
                </thead>
                <tbody>
                  {mergedModelRows.map((r) => (
                    <tr key={`${r.provider}:${r.model}:${r.pricing_source}`} className="border-b border-zinc-100 last:border-0">
                      <td className="px-3 py-2">{r.provider}</td>
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

      <DailySummaryGroups
        title={t("llm.dailySummary")}
        groupedByDate={groupedByDate}
        noSummaryLabel={t("llm.noSummary")}
        fmtNum={fmtNum}
        fmtUSD={fmtUSD}
      />

      <RecentLogsTable
        logs={logs}
        pagedLogs={pagedLogs}
        logPage={logPage}
        setLogPage={setLogPage}
        logsPageSize={logsPageSize}
        locale={locale}
        noLogsLabel={t("llm.noLogs")}
        labels={{ title: t("llm.recentLogs"), time: t("llm.time") }}
        fmtNum={fmtNum}
        fmtUSD={fmtUSD}
      />
    </div>
  );
}
