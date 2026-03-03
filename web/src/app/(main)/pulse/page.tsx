"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueries, useQueryClient } from "@tanstack/react-query";
import { Activity, TrendingUp, BookmarkPlus, CheckCircle2, Star } from "lucide-react";
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
  const queryClient = useQueryClient();
  const [days, setDays] = useState(7);
  const [limit, setLimit] = useState(12);
  const [busyItemId, setBusyItemId] = useState<string | null>(null);

  const pulseQuery = useQuery({
    queryKey: ["topics-pulse", days, limit] as const,
    queryFn: () => api.getTopicPulse({ days, limit }),
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  });

  const rows = useMemo(() => pulseQuery.data?.items ?? [], [pulseQuery.data?.items]);
  const topForChart = rows.slice(0, 6);
  const topTopics = useMemo(() => rows.slice(0, 8), [rows]);

  const statsQuery = useQuery({
    queryKey: ["items-stats"] as const,
    queryFn: () => api.getItemStats(),
    staleTime: 30_000,
  });
  const uxQuery = useQuery({
    queryKey: ["items-ux", 7] as const,
    queryFn: () => api.getItemUXMetrics({ days: 7 }),
    staleTime: 30_000,
  });

  const topicQueueQueries = useQueries({
    queries: topTopics.map((topic) => ({
      queryKey: ["pulse-topic-queue", topic.topic, days] as const,
      queryFn: () =>
        api.getItems({
          topic: topic.topic,
          unread_only: true,
          sort: "score",
          page: 1,
          page_size: 3,
        }),
      staleTime: 30_000,
    })),
  });

  const topicQueues = useMemo(
    () =>
      topTopics.map((topic, idx) => {
        const q = topicQueueQueries[idx];
        const data = q?.data;
        return {
          ...topic,
          unread_total: data?.total ?? 0,
          picks: data?.items ?? [],
        };
      }),
    [topTopics, topicQueueQueries]
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

  const onMarkRead = async (itemId: string) => {
    setBusyItemId(itemId);
    try {
      await api.markItemRead(itemId);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["items-stats"] }),
        queryClient.invalidateQueries({ queryKey: ["topics-pulse"] }),
        queryClient.invalidateQueries({ queryKey: ["pulse-topic-queue"] }),
      ]);
    } finally {
      setBusyItemId(null);
    }
  };

  const onLater = async (itemId: string) => {
    setBusyItemId(itemId);
    try {
      await api.markItemLater(itemId);
      await queryClient.invalidateQueries({ queryKey: ["pulse-topic-queue"] });
    } finally {
      setBusyItemId(null);
    }
  };

  const onFav = async (itemId: string, currentRating: number, currentFav: boolean) => {
    setBusyItemId(itemId);
    try {
      await api.setItemFeedback(itemId, {
        rating: currentRating,
        is_favorite: !currentFav,
      });
      await queryClient.invalidateQueries({ queryKey: ["pulse-topic-queue"] });
    } finally {
      setBusyItemId(null);
    }
  };

  const reasonText = (delta: number, maxScore?: number | null, unreadTotal?: number) => {
    if ((unreadTotal ?? 0) >= 8) return t("pulse.reason.backlog");
    if (typeof maxScore === "number" && maxScore >= 0.9) return t("pulse.reason.highScore");
    if (delta > 0) return t("pulse.reason.rising");
    return t("pulse.reason.stable");
  };

  const topicLabel = (topic: string) => (topic === "__untagged__" ? t("pulse.topic.untagged") : topic);
  const topicHref = (topic: string) =>
    topic === "__untagged__"
      ? "/items?feed=all&sort=score&status=summarized"
      : `/items?feed=all&sort=score&topic=${encodeURIComponent(topic)}`;

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

      <section className="grid gap-3 sm:grid-cols-3">
        <div className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
          <div className="text-xs text-zinc-500">{t("pulse.metric.unread")}</div>
          <div className="mt-1 text-2xl font-semibold text-zinc-900">{statsQuery.data?.unread ?? "-"}</div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
          <div className="text-xs text-zinc-500">{t("pulse.metric.todayRate")}</div>
          <div className="mt-1 text-2xl font-semibold text-zinc-900">
            {typeof uxQuery.data?.today_consumption_rate === "number"
              ? `${Math.round(uxQuery.data.today_consumption_rate)}%`
              : "-"}
          </div>
        </div>
        <div className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
          <div className="text-xs text-zinc-500">{t("pulse.metric.streak")}</div>
          <div className="mt-1 text-2xl font-semibold text-zinc-900">{uxQuery.data?.current_streak_days ?? "-"}</div>
        </div>
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
        <div className="mb-3 text-sm font-semibold text-zinc-800">{t("pulse.queue.title")}</div>
        <div className="grid gap-3 md:grid-cols-2">
          {topicQueues.map((topic) => {
            const first = topic.picks[0];
            return (
              <div key={topic.topic} className="rounded-lg border border-zinc-200 p-3">
                <div className="flex items-center justify-between gap-2">
                  <a
                    href={topicHref(topic.topic)}
                    className="truncate text-sm font-semibold text-zinc-900 hover:underline"
                    title={topicLabel(topic.topic)}
                  >
                    {topicLabel(topic.topic)}
                  </a>
                  <span className="rounded bg-zinc-100 px-2 py-0.5 text-xs text-zinc-700">
                    {t("pulse.queue.unread")} {topic.unread_total}
                  </span>
                </div>
                <p className="mt-1 text-xs text-zinc-500">{reasonText(topic.delta, topic.max_score, topic.unread_total)}</p>
                {!first ? (
                  <p className="mt-3 text-sm text-zinc-400">{t("common.noData")}</p>
                ) : (
                  <div className="mt-3 space-y-2">
                    <a
                      href={`/items/${first.id}?from=${encodeURIComponent("/pulse")}`}
                      className="line-clamp-2 text-sm font-medium text-zinc-900 hover:underline"
                    >
                      {first.translated_title || first.title || first.url}
                    </a>
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => onMarkRead(first.id)}
                        disabled={busyItemId === first.id}
                        className="inline-flex items-center gap-1 rounded border border-zinc-300 px-2 py-1 text-xs text-zinc-700 disabled:opacity-50"
                      >
                        <CheckCircle2 className="size-3.5" />
                        {t("pulse.action.read")}
                      </button>
                      <button
                        type="button"
                        onClick={() => onLater(first.id)}
                        disabled={busyItemId === first.id}
                        className="inline-flex items-center gap-1 rounded border border-zinc-300 px-2 py-1 text-xs text-zinc-700 disabled:opacity-50"
                      >
                        <BookmarkPlus className="size-3.5" />
                        {t("pulse.action.later")}
                      </button>
                      <button
                        type="button"
                        onClick={() => onFav(first.id, Number(first.feedback_rating ?? 0), Boolean(first.is_favorite))}
                        disabled={busyItemId === first.id}
                        className="inline-flex items-center gap-1 rounded border border-zinc-300 px-2 py-1 text-xs text-zinc-700 disabled:opacity-50"
                      >
                        <Star className="size-3.5" />
                        {first.is_favorite ? t("pulse.action.unfav") : t("pulse.action.fav")}
                      </button>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </section>

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
                    name={topicLabel(topic.topic)}
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
          <div className="overflow-x-auto">
            <div className="space-y-2" style={{ minWidth: `${heatmapMinWidth}px` }}>
              <div className="grid grid-cols-[minmax(160px,240px)_1fr] items-center gap-3 px-1">
                <div className="text-xs font-medium text-zinc-500">{t("pulse.table.topic")}</div>
                <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${heatmapDates.length}, minmax(28px, 1fr))` }}>
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
                      href={topicHref(row.topic)}
                      className="block truncate text-sm font-medium text-zinc-900 hover:underline"
                      title={topicLabel(row.topic)}
                    >
                      {topicLabel(row.topic)}
                    </a>
                    <div className="mt-0.5 text-[11px] text-zinc-500">{`${t("pulse.table.total")}: ${row.total}`}</div>
                  </div>
                  <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${heatmapDates.length}, minmax(28px, 1fr))` }}>
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
                  <th className="border-b border-zinc-200 px-2 py-2">{t("pulse.table.unread")}</th>
                  <th className="border-b border-zinc-200 px-2 py-2">{t("pulse.table.maxScore")}</th>
                  <th className="border-b border-zinc-200 px-2 py-2">{t("pulse.table.open")}</th>
                </tr>
              </thead>
              <tbody>
                {topicQueues.map((row) => (
                  <tr key={row.topic}>
                    <td className="border-b border-zinc-100 px-2 py-2 font-medium text-zinc-900">{topicLabel(row.topic)}</td>
                    <td className="border-b border-zinc-100 px-2 py-2 text-zinc-700">{row.total}</td>
                    <td className="border-b border-zinc-100 px-2 py-2 text-zinc-700">{row.unread_total}</td>
                    <td className="border-b border-zinc-100 px-2 py-2 text-zinc-700">
                      {typeof row.max_score === "number" ? row.max_score.toFixed(2) : "-"}
                    </td>
                    <td className="border-b border-zinc-100 px-2 py-2">
                      <a
                        href={topicHref(row.topic)}
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
