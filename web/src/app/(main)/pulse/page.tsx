"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, TrendingUp } from "lucide-react";
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { api } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

const COLORS = ["#2563eb", "#16a34a", "#ea580c", "#7c3aed", "#0891b2", "#dc2626"];

export default function PulsePage() {
  const { t } = useI18n();
  const [days, setDays] = useState(7);
  const [limit, setLimit] = useState(12);

  const pulseQuery = useQuery({
    queryKey: ["topics-pulse", days, limit] as const,
    queryFn: () => api.getTopicPulse({ days, limit }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });

  const rows = useMemo(() => pulseQuery.data?.items ?? [], [pulseQuery.data?.items]);
  const topForChart = rows.slice(0, 6);

  const chartRows = useMemo(() => {
    const dateSet = new Set<string>();
    for (const topic of topForChart) {
      for (const p of topic.points) dateSet.add(p.date);
    }
    const dates = Array.from(dateSet).sort((a, b) => a.localeCompare(b));
    return dates.map((date) => {
      const row: Record<string, string | number> = { date: date.slice(5) };
      for (const topic of topForChart) {
        const p = topic.points.find((v) => v.date === date);
        row[topic.topic] = p?.count ?? 0;
      }
      return row;
    });
  }, [topForChart]);

  const heatmapDates = useMemo(() => {
    const dateSet = new Set<string>();
    for (const topic of rows) {
      for (const p of topic.points) dateSet.add(p.date);
    }
    return Array.from(dateSet).sort((a, b) => a.localeCompare(b));
  }, [rows]);

  const heatmapRows = useMemo(() => rows.slice(0, 10), [rows]);

  const maxHeatCount = useMemo(() => {
    let max = 0;
    for (const topic of heatmapRows) {
      for (const p of topic.points) {
        if (p.count > max) max = p.count;
      }
    }
    return max;
  }, [heatmapRows]);

  const heatColor = (count: number) => {
    if (count <= 0 || maxHeatCount <= 0) return "hsl(220 14% 96%)";
    const ratio = Math.max(0, Math.min(1, count / maxHeatCount));
    const lightness = 96 - ratio * 52;
    return `hsl(218 88% ${lightness.toFixed(1)}%)`;
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight text-zinc-900">
            <Activity className="size-6 text-zinc-600" aria-hidden="true" />
            <span>{t("pulse.title")}</span>
          </h1>
          <p className="mt-1 text-sm text-zinc-500">{t("pulse.subtitle")}</p>
        </div>
        <div className="flex items-center gap-2">
          <select
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
            className="rounded border border-zinc-300 bg-white px-2 py-1 text-sm text-zinc-700"
          >
            {[3, 7, 14, 30].map((v) => (
              <option key={v} value={v}>
                {v}{t("llm.daysSuffix")}
              </option>
            ))}
          </select>
          <select
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value))}
            className="rounded border border-zinc-300 bg-white px-2 py-1 text-sm text-zinc-700"
          >
            {[8, 12, 16, 24].map((v) => (
              <option key={v} value={v}>
                Top {v}
              </option>
            ))}
          </select>
        </div>
      </div>

      {pulseQuery.isLoading && !pulseQuery.data && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {pulseQuery.error && <p className="text-sm text-red-500">{String(pulseQuery.error)}</p>}

      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <div className="mb-2 text-sm font-semibold text-zinc-800">{t("pulse.chart.title")}</div>
        {chartRows.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("common.noData")}</p>
        ) : (
          <div className="h-[320px] w-full">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={chartRows} margin={{ top: 8, right: 16, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" />
                <XAxis dataKey="date" stroke="#71717a" tick={{ fontSize: 12 }} />
                <YAxis stroke="#71717a" allowDecimals={false} tick={{ fontSize: 12 }} />
                <Tooltip />
                <Legend />
                {topForChart.map((topic, idx) => (
                  <Line
                    key={topic.topic}
                    type="monotone"
                    dataKey={topic.topic}
                    stroke={COLORS[idx % COLORS.length]}
                    strokeWidth={2}
                    dot={false}
                  />
                ))}
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <div className="text-sm font-semibold text-zinc-800">{t("pulse.heatmap.title")}</div>
          <div className="inline-flex items-center gap-2 text-xs text-zinc-500">
            <span>{t("pulse.heatmap.low")}</span>
            <div className="h-2 w-24 rounded-full bg-gradient-to-r from-zinc-100 via-blue-300 to-blue-700" />
            <span>{t("pulse.heatmap.high")}</span>
          </div>
        </div>
        {heatmapRows.length === 0 || heatmapDates.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("common.noData")}</p>
        ) : (
          <div className="space-y-2">
            <div className="grid grid-cols-[minmax(160px,240px)_1fr] items-center gap-3 px-1">
              <div className="text-xs font-medium text-zinc-500">{t("pulse.table.topic")}</div>
              <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${heatmapDates.length}, minmax(0, 1fr))` }}>
                {heatmapDates.map((date) => (
                  <div key={date} className="text-center text-[10px] text-zinc-500">
                    {date.slice(5)}
                  </div>
                ))}
              </div>
            </div>
            {heatmapRows.map((row) => (
              <div key={row.topic} className="grid grid-cols-[minmax(160px,240px)_1fr] items-center gap-3 rounded-lg border border-zinc-100 bg-zinc-50/50 p-2">
                <div className="min-w-0">
                  <a
                    href={`/items?feed=all&sort=score&topic=${encodeURIComponent(row.topic)}`}
                    className="block truncate text-sm font-medium text-zinc-900 hover:underline"
                    title={row.topic}
                  >
                    {row.topic}
                  </a>
                  <div className="mt-0.5 text-[11px] text-zinc-500">{`${t("pulse.table.total")}: ${row.total}`}</div>
                </div>
                <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${heatmapDates.length}, minmax(0, 1fr))` }}>
                  {heatmapDates.map((date) => {
                    const point = row.points.find((v) => v.date === date);
                    const count = point?.count ?? 0;
                    const color = heatColor(count);
                    const textColor = count > maxHeatCount * 0.4 ? "text-white" : "text-zinc-700";
                    return (
                      <div
                        key={`${row.topic}-${date}`}
                        className={`rounded-md py-1 text-center text-[11px] font-semibold ${textColor}`}
                        style={{ backgroundColor: color }}
                        title={`${row.topic} ${date}: ${count}`}
                      >
                        {count}
                      </div>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <div className="mb-2 flex items-center gap-2 text-sm font-semibold text-zinc-800">
          <TrendingUp className="size-4 text-zinc-500" aria-hidden="true" />
          <span>{t("pulse.rising.title")}</span>
        </div>
        {rows.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("common.noData")}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="text-left text-zinc-500">
                  <th className="border-b border-zinc-200 px-2 py-2">{t("pulse.table.topic")}</th>
                  <th className="border-b border-zinc-200 px-2 py-2">{t("pulse.table.total")}</th>
                  <th className="border-b border-zinc-200 px-2 py-2">{t("pulse.table.delta")}</th>
                  <th className="border-b border-zinc-200 px-2 py-2">{t("pulse.table.maxScore")}</th>
                  <th className="border-b border-zinc-200 px-2 py-2">{t("pulse.table.open")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={row.topic}>
                    <td className="border-b border-zinc-100 px-2 py-2 font-medium text-zinc-900">{row.topic}</td>
                    <td className="border-b border-zinc-100 px-2 py-2 text-zinc-700">{row.total}</td>
                    <td className="border-b border-zinc-100 px-2 py-2">
                      <span
                        className={`rounded px-2 py-0.5 text-xs font-medium ${
                          row.delta > 0
                            ? "bg-green-50 text-green-700"
                            : row.delta < 0
                              ? "bg-zinc-100 text-zinc-700"
                              : "bg-blue-50 text-blue-700"
                        }`}
                      >
                        {row.delta > 0 ? "+" : ""}
                        {row.delta}
                      </span>
                    </td>
                    <td className="border-b border-zinc-100 px-2 py-2 text-zinc-700">
                      {typeof row.max_score === "number" ? row.max_score.toFixed(2) : "-"}
                    </td>
                    <td className="border-b border-zinc-100 px-2 py-2">
                      <a
                        href={`/items?feed=all&sort=score&topic=${encodeURIComponent(row.topic)}`}
                        className="text-xs text-blue-600 hover:underline"
                      >
                        {t("pulse.table.openItems")}
                      </a>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
