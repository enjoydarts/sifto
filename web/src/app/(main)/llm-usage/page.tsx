"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Brain, CalendarDays, ReceiptText } from "lucide-react";
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
import { api, LLMUsageDailySummary, LLMUsageLog, LLMUsageModelSummary, UserSettings } from "@/lib/api";
import Pagination from "@/components/pagination";
import { useI18n } from "@/components/i18n-provider";

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

export default function LLMUsagePage() {
  const { t, locale } = useI18n();
  const [forecastMode, setForecastMode] = useState<"month_avg" | "recent_7d">("month_avg");
  const [forecastMonth, setForecastMonth] = useState<string | null>(null);
  const [days, setDays] = useState(14);
  const [limit, setLimit] = useState(100);
  const [logPage, setLogPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [summaryRows, setSummaryRows] = useState<LLMUsageDailySummary[]>([]);
  const [modelRows, setModelRows] = useState<LLMUsageModelSummary[]>([]);
  const [logs, setLogs] = useState<LLMUsageLog[]>([]);
  const [settings, setSettings] = useState<UserSettings | null>(null);

  const load = useCallback(async (daysParam: number, limitParam: number) => {
    setLoading(true);
    try {
      const [summary, byModel, recent, userSettings] = await Promise.all([
        api.getLLMUsageSummary({ days: daysParam }),
        api.getLLMUsageByModel({ days: daysParam }),
        api.getLLMUsage({ limit: limitParam }),
        api.getSettings(),
      ]);
      setSummaryRows(summary ?? []);
      setModelRows(byModel ?? []);
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
    load(days, limit);
  }, [days, limit, load]);

  const totals = useMemo(() => {
    const t = {
      calls: 0,
      input: 0,
      output: 0,
      cost: 0,
      byProviderCost: new Map<string, number>(),
    };
    for (const r of summaryRows) {
      t.calls += r.calls;
      t.input += r.input_tokens;
      t.output += r.output_tokens;
      t.cost += r.estimated_cost_usd;
      t.byProviderCost.set(r.provider, (t.byProviderCost.get(r.provider) ?? 0) + r.estimated_cost_usd);
    }
    return t;
  }, [summaryRows]);

  const providerTotals = useMemo(() => {
    const openai = totals.byProviderCost.get("openai") ?? 0;
    const anthropic = totals.byProviderCost.get("anthropic") ?? 0;
    const google = totals.byProviderCost.get("google") ?? 0;
    const others = [...totals.byProviderCost.entries()]
      .filter(([k]) => k !== "openai" && k !== "anthropic" && k !== "google")
      .reduce((acc, [, v]) => acc + v, 0);
    return { openai, anthropic, google, others };
  }, [totals]);

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
    const m = new Map<string, { date: string; total: number; openai: number; anthropic: number; google: number; other: number }>();
    for (const row of summaryRows) {
      const cur = m.get(row.date_jst) ?? { date: row.date_jst, total: 0, openai: 0, anthropic: 0, google: 0, other: 0 };
      cur.total += row.estimated_cost_usd;
      if (row.provider === "openai") cur.openai += row.estimated_cost_usd;
      else if (row.provider === "anthropic") cur.anthropic += row.estimated_cost_usd;
      else if (row.provider === "google") cur.google += row.estimated_cost_usd;
      else cur.other += row.estimated_cost_usd;
      m.set(row.date_jst, cur);
    }
    return Array.from(m.values()).sort((a, b) => a.date.localeCompare(b.date));
  }, [summaryRows]);

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

  const topModelRows = useMemo(() => mergedModelRows.slice(0, 10), [mergedModelRows]);
  const topModelChartRows = useMemo(
    () =>
      topModelRows
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
    [topModelRows]
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
          forecast: isCurrentMonth && d === today ? forecastTotal : null,
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
            <span>LLM Usage</span>
          </h1>
          <p className="mt-1 text-sm text-zinc-500">
            {t("llm.subtitle")}
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <label className="text-sm">
            <span className="mb-1 block text-xs font-medium text-zinc-600">{t("llm.days")}</span>
            <select
              value={days}
              onChange={(e) => setDays(Number(e.target.value))}
              className="rounded border border-zinc-300 bg-white px-3 py-2 text-sm"
            >
              {[7, 14, 30, 90].map((d) => (
                <option key={d} value={d}>
                  {`${d}${t("llm.daysSuffix")}`}
                </option>
              ))}
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

      <section className="flex w-full flex-wrap gap-3 lg:flex-nowrap">
        <MetricCard label={t("llm.totalCost")} value={fmtUSD(totals.cost)} />
        <MetricCard label="OpenAI" value={fmtUSD(providerTotals.openai)} />
        <MetricCard label="Anthropic" value={fmtUSD(providerTotals.anthropic)} />
        <MetricCard label="Google" value={fmtUSD(providerTotals.google)} />
        {providerTotals.others > 0 && <MetricCard label="Other" value={fmtUSD(providerTotals.others)} />}
        <MetricCard label={t("llm.totalCalls")} value={fmtNum(totals.calls)} />
        <MetricCard label={t("llm.input")} value={fmtNum(totals.input)} />
        <MetricCard label={t("llm.output")} value={fmtNum(totals.output)} />
      </section>

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
                    name ?? "",
                  ]}
                  labelFormatter={(label) => `${label}`}
                  contentStyle={{ borderRadius: 10, borderColor: "#e4e4e7" }}
                />
                <Legend wrapperStyle={{ fontSize: 12 }} />
                <Area
                  type="monotone"
                  dataKey="openai"
                  name="OpenAI"
                  stackId="cost"
                  stroke="#10b981"
                  fill="#34d399"
                  fillOpacity={0.65}
                />
                <Area
                  type="monotone"
                  dataKey="anthropic"
                  name="Anthropic"
                  stackId="cost"
                  stroke="#3b82f6"
                  fill="#60a5fa"
                  fillOpacity={0.6}
                />
                <Area
                  type="monotone"
                  dataKey="google"
                  name="Google"
                  stackId="cost"
                  stroke="#f59e0b"
                  fill="#fbbf24"
                  fillOpacity={0.6}
                />
                <Area
                  type="monotone"
                  dataKey="other"
                  name="Other"
                  stackId="cost"
                  stroke="#71717a"
                  fill="#a1a1aa"
                  fillOpacity={0.45}
                />
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
                  data={topModelChartRows}
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

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <CalendarDays className="size-4 text-zinc-500" aria-hidden="true" />
            <span>{t("llm.dailySummary")}</span>
          </h2>
          <span className="text-xs text-zinc-400">{summaryRows.length} rows</span>
        </div>
        {groupedByDate.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("llm.noSummary")}</p>
        ) : (
          <div className="space-y-4">
            {groupedByDate.map(([date, rows]) => {
              const dayCost = rows.reduce((acc, r) => acc + r.estimated_cost_usd, 0);
              const dayCalls = rows.reduce((acc, r) => acc + r.calls, 0);
              return (
                <div key={date} className="rounded border border-zinc-200">
                  <div className="flex items-center justify-between border-b border-zinc-200 bg-zinc-50 px-3 py-2">
                    <div className="text-sm font-medium text-zinc-800">{date}</div>
                    <div className="text-xs text-zinc-500">
                      calls {fmtNum(dayCalls)} / cost {fmtUSD(dayCost)}
                    </div>
                  </div>
                  <div className="overflow-x-auto">
                    <table className="min-w-full text-sm">
                      <thead className="text-xs text-zinc-500">
                        <tr className="border-b border-zinc-100">
                          <th className="px-3 py-2 text-left font-medium">purpose</th>
                          <th className="px-3 py-2 text-left font-medium">provider</th>
                          <th className="px-3 py-2 text-left font-medium">pricing</th>
                          <th className="px-3 py-2 text-right font-medium">calls</th>
                          <th className="px-3 py-2 text-right font-medium">input</th>
                          <th className="px-3 py-2 text-right font-medium">output</th>
                          <th className="px-3 py-2 text-right font-medium">cost</th>
                        </tr>
                      </thead>
                      <tbody>
                        {rows.map((r) => (
                          <tr key={r.key} className="border-b border-zinc-100 last:border-0">
                            <td className="px-3 py-2">{r.purpose}</td>
                            <td className="px-3 py-2">{r.provider}</td>
                            <td className="px-3 py-2">{r.pricing_source}</td>
                            <td className="px-3 py-2 text-right">{fmtNum(r.calls)}</td>
                            <td className="px-3 py-2 text-right">{fmtNum(r.input_tokens)}</td>
                            <td className="px-3 py-2 text-right">{fmtNum(r.output_tokens)}</td>
                            <td className="px-3 py-2 text-right">{fmtUSD(r.estimated_cost_usd)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <ReceiptText className="size-4 text-zinc-500" aria-hidden="true" />
            <span>{t("llm.recentLogs")}</span>
          </h2>
          <span className="text-xs text-zinc-400">{logs.length} rows</span>
        </div>
        {logs.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("llm.noLogs")}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="text-xs text-zinc-500">
                <tr className="border-b border-zinc-100">
                  <th className="px-3 py-2 text-left font-medium">{t("llm.time")}</th>
                  <th className="px-3 py-2 text-left font-medium">purpose</th>
                  <th className="px-3 py-2 text-left font-medium">model</th>
                  <th className="px-3 py-2 text-left font-medium">pricing</th>
                  <th className="px-3 py-2 text-right font-medium">in</th>
                  <th className="px-3 py-2 text-right font-medium">out</th>
                  <th className="px-3 py-2 text-right font-medium">cost</th>
                  <th className="px-3 py-2 text-left font-medium">ref</th>
                </tr>
              </thead>
              <tbody>
                {pagedLogs.map((r) => (
                  <tr key={r.id} className="border-b border-zinc-100 last:border-0 align-top">
                    <td className="px-3 py-2 whitespace-nowrap text-xs text-zinc-500">
                      {new Date(r.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}
                    </td>
                    <td className="px-3 py-2">{r.purpose}</td>
                    <td className="px-3 py-2">
                      <div className="whitespace-nowrap text-xs text-zinc-700">{r.model}</div>
                      {r.pricing_model_family && r.pricing_model_family !== r.model && (
                        <div className="text-[11px] text-zinc-400">{r.pricing_model_family}</div>
                      )}
                    </td>
                    <td className="px-3 py-2 text-xs">{r.pricing_source}</td>
                    <td className="px-3 py-2 text-right">{fmtNum(r.input_tokens)}</td>
                    <td className="px-3 py-2 text-right">{fmtNum(r.output_tokens)}</td>
                    <td className="px-3 py-2 text-right">{fmtUSD(r.estimated_cost_usd)}</td>
                    <td className="px-3 py-2 text-[11px] text-zinc-500">
                      {r.item_id ? `item:${r.item_id.slice(0, 8)}` : ""}
                      {r.digest_id ? `digest:${r.digest_id.slice(0, 8)}` : ""}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <Pagination total={logs.length} page={logPage} pageSize={logsPageSize} onPageChange={setLogPage} className="mt-3" />
      </section>
    </div>
  );
}

function MetricCard({ label, value, className = "" }: { label: string; value: string; className?: string }) {
  return (
    <div className={`min-w-0 w-[calc(50%-0.375rem)] rounded-lg border border-zinc-200 bg-white px-4 py-3 sm:w-[calc(33.333%-0.5rem)] lg:w-auto lg:flex-1 ${className}`.trim()}>
      <div className="text-xs font-medium text-zinc-500">{label}</div>
      <div className="mt-1 truncate text-lg font-semibold text-zinc-900">{value}</div>
    </div>
  );
}
