"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, TrendingUp } from "lucide-react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { api } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";
import { PageTransition } from "@/components/page-transition";
import { PageHeader } from "@/components/ui/page-header";

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
  const risingRows = useMemo(
    () =>
      [...rows].sort((a, b) => {
        if (a.delta !== b.delta) return b.delta - a.delta;
        if (a.total !== b.total) return b.total - a.total;
        const leftMax = typeof a.max_score === "number" ? a.max_score : -1;
        const rightMax = typeof b.max_score === "number" ? b.max_score : -1;
        if (leftMax !== rightMax) return rightMax - leftMax;
        return a.topic.localeCompare(b.topic);
      }),
    [rows]
  );

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
  const heatmapMinWidth = useMemo(
    () => Math.max(640, 220 + heatmapDates.length * 42),
    [heatmapDates.length]
  );

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

  const topicLabel = (topic: string) => (topic === "__untagged__" ? t("pulse.topic.untagged") : topic);
  const topicHref = (topic: string) =>
    topic === "__untagged__"
      ? "/items?feed=all&sort=score&status=summarized"
      : `/items?feed=all&sort=score&topic=${encodeURIComponent(topic)}`;

  return (
    <PageTransition>
      <div className="space-y-6">
        <PageHeader
          title={
            <span className="flex items-center gap-2 font-serif">
              <Activity className="size-6 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
              <span>{t("pulse.title")}</span>
            </span>
          }
          description={t("pulse.subtitle")}
          actions={
            <div className="flex items-center gap-2">
              <select
                value={days}
                onChange={(e) => setDays(Number(e.target.value))}
                className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink-soft)] outline-none"
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
                className="rounded-full border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] px-3 py-2 text-sm text-[var(--color-editorial-ink-soft)] outline-none"
              >
                {[8, 12, 16, 24].map((v) => (
                  <option key={v} value={v}>
                    Top {v}
                  </option>
                ))}
              </select>
            </div>
          }
        />

        {pulseQuery.isLoading && !pulseQuery.data && <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("common.loading")}</p>}
        {pulseQuery.error && <p className="text-sm text-[var(--color-editorial-error)]">{String(pulseQuery.error)}</p>}

        <section className="surface-editorial rounded-[28px] p-5 shadow-[var(--shadow-card)]">
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
            Trend Chart
          </div>
          <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
            {t("pulse.chart.title")}
          </h2>
          {chartRows.length === 0 ? (
            <p className="mt-4 text-sm text-[var(--color-editorial-ink-faint)]">{t("common.noData")}</p>
          ) : (
            <div className="mt-4 rounded-[24px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
              <div className="mb-4 flex flex-wrap gap-3 text-xs text-[var(--color-editorial-ink-soft)]">
                {topForChart.map((topic, idx) => (
                  <div key={topic.topic} className="inline-flex items-center gap-2">
                    <span
                      className="inline-block size-2.5 rounded-full"
                      style={{ backgroundColor: COLORS[idx % COLORS.length] }}
                    />
                    <span>{topicLabel(topic.topic)}</span>
                  </div>
                ))}
              </div>
              <div className="h-[320px] w-full">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={chartRows} margin={{ top: 8, right: 16, left: 0, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#d9d1c4" />
                    <XAxis dataKey="date" stroke="#8b7f71" tick={{ fontSize: 12 }} />
                    <YAxis stroke="#8b7f71" allowDecimals={false} tick={{ fontSize: 12 }} />
                    <Tooltip />
                    {topForChart.map((topic, idx) => (
                      <Line
                        key={topic.topic}
                        type="monotone"
                        dataKey={topic.topic}
                        name={topicLabel(topic.topic)}
                        stroke={COLORS[idx % COLORS.length]}
                        strokeWidth={2}
                        dot={false}
                      />
                    ))}
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </div>
          )}
        </section>

        <section className="surface-editorial rounded-[28px] p-5 shadow-[var(--shadow-card)]">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <div>
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--color-editorial-ink-faint)]">
                Heatmap
              </div>
              <h2 className="mt-2 font-serif text-[2rem] leading-none tracking-[-0.03em] text-[var(--color-editorial-ink)]">
                {t("pulse.heatmap.title")}
              </h2>
            </div>
            <div className="inline-flex items-center gap-2 text-xs text-[var(--color-editorial-ink-faint)]">
              <span>{t("pulse.heatmap.low")}</span>
              <div className="h-2 w-24 rounded-full bg-gradient-to-r from-zinc-100 via-blue-300 to-blue-700" />
              <span>{t("pulse.heatmap.high")}</span>
            </div>
          </div>
          {heatmapRows.length === 0 || heatmapDates.length === 0 ? (
            <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("common.noData")}</p>
          ) : (
            <div className="overflow-x-auto rounded-[24px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)] p-4">
              <div className="space-y-2" style={{ minWidth: `${heatmapMinWidth}px` }}>
                <div className="grid grid-cols-[minmax(160px,240px)_1fr] items-center gap-3 px-1">
                  <div className="text-xs font-medium text-[var(--color-editorial-ink-faint)]">{t("pulse.table.topic")}</div>
                  <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${heatmapDates.length}, minmax(28px, 1fr))` }}>
                    {heatmapDates.map((date) => (
                      <div key={date} className="text-center text-[10px] text-[var(--color-editorial-ink-faint)]">
                        {date.slice(5)}
                      </div>
                    ))}
                  </div>
                </div>
                {heatmapRows.map((row) => (
                  <div key={row.topic} className="grid grid-cols-[minmax(160px,240px)_1fr] items-center gap-3 rounded-[18px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel-strong)] p-2.5">
                    <div className="min-w-0">
                      <a
                        href={topicHref(row.topic)}
                        className="block truncate text-sm font-medium text-[var(--color-editorial-ink)] hover:underline"
                        title={topicLabel(row.topic)}
                      >
                        {topicLabel(row.topic)}
                      </a>
                      <div className="mt-0.5 text-[11px] text-[var(--color-editorial-ink-faint)]">{`${t("pulse.table.total")}: ${row.total}`}</div>
                    </div>
                    <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${heatmapDates.length}, minmax(28px, 1fr))` }}>
                      {heatmapDates.map((date) => {
                        const point = row.points.find((v) => v.date === date);
                        const count = point?.count ?? 0;
                        const color = heatColor(count);
                        const textColor = count > maxHeatCount * 0.4 ? "text-white" : "text-[var(--color-editorial-ink-soft)]";
                        return (
                          <div
                            key={`${row.topic}-${date}`}
                            className={`rounded-md py-1 text-center text-[11px] font-semibold ${textColor}`}
                            style={{ backgroundColor: color }}
                            title={`${topicLabel(row.topic)} ${date}: ${count}`}
                          >
                            {count}
                          </div>
                        );
                      })}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </section>

        <section className="surface-editorial rounded-[28px] p-5 shadow-[var(--shadow-card)]">
          <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-[var(--color-editorial-ink)]">
            <TrendingUp className="size-4 text-[var(--color-editorial-ink-faint)]" aria-hidden="true" />
            <span>{t("pulse.rising.title")}</span>
          </div>
          {rows.length === 0 ? (
            <p className="text-sm text-[var(--color-editorial-ink-faint)]">{t("common.noData")}</p>
          ) : (
            <div className="overflow-hidden rounded-[24px] border border-[var(--color-editorial-line)] bg-[var(--color-editorial-panel)]">
              <div className="overflow-x-auto">
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className="text-left text-[var(--color-editorial-ink-faint)]">
                      <th className="border-b border-[var(--color-editorial-line)] px-3 py-3">{t("pulse.table.topic")}</th>
                      <th className="border-b border-[var(--color-editorial-line)] px-3 py-3">{t("pulse.table.total")}</th>
                      <th className="border-b border-[var(--color-editorial-line)] px-3 py-3">{t("pulse.table.delta")}</th>
                      <th className="border-b border-[var(--color-editorial-line)] px-3 py-3">{t("pulse.table.maxScore")}</th>
                      <th className="border-b border-[var(--color-editorial-line)] px-3 py-3">{t("pulse.table.open")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {risingRows.map((row) => (
                      <tr key={row.topic}>
                        <td className="border-b border-[#ece4d6] px-3 py-3 font-medium text-[var(--color-editorial-ink)]">{topicLabel(row.topic)}</td>
                        <td className="border-b border-[#ece4d6] px-3 py-3 text-[var(--color-editorial-ink-soft)]">{row.total}</td>
                        <td className="border-b border-[#ece4d6] px-3 py-3">
                          <span
                            className={`rounded-full px-2.5 py-1 text-xs font-medium ${
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
                        <td className="border-b border-[#ece4d6] px-3 py-3 text-[var(--color-editorial-ink-soft)]">
                          {typeof row.max_score === "number" ? row.max_score.toFixed(2) : "-"}
                        </td>
                        <td className="border-b border-[#ece4d6] px-3 py-3">
                          <a
                            href={topicHref(row.topic)}
                            className="text-xs text-[var(--color-editorial-accent)] hover:underline"
                          >
                            {t("pulse.table.openItems")}
                          </a>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </section>
      </div>
    </PageTransition>
  );
}
