"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { BarChart3, Brain, LayoutDashboard, Mail } from "lucide-react";
import { api, Digest, Item, ItemStats, LLMUsageDailySummary, Source, TopicTrend } from "@/lib/api";
import { useI18n } from "@/components/i18n-provider";

export default function DashboardPage() {
  const { t, locale } = useI18n();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sources, setSources] = useState<Source[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [itemStats, setItemStats] = useState<ItemStats | null>(null);
  const [digests, setDigests] = useState<Digest[]>([]);
  const [llmSummary, setLlmSummary] = useState<LLMUsageDailySummary[]>([]);
  const [topicTrends, setTopicTrends] = useState<TopicTrend[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [srcs, its, stats, dgs, llm, topics] = await Promise.all([
        api.getSources(),
        api.getItems({ page: 1, page_size: 200 }),
        api.getItemStats(),
        api.getDigests(),
        api.getLLMUsageSummary({ days: 7 }),
        api.getItemTopicTrends({ limit: 8 }),
      ]);
      setSources(srcs ?? []);
      setItems(its?.items ?? []);
      setItemStats(stats ?? null);
      setDigests(dgs ?? []);
      setLlmSummary(llm ?? []);
      setTopicTrends(topics?.items ?? []);
      setError(null);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const itemStatus = useMemo(() => {
    const counts: Record<string, number> = {};
    if (itemStats?.by_status) return { ...itemStats.by_status };
    for (const item of items) counts[item.status] = (counts[item.status] ?? 0) + 1;
    return counts;
  }, [itemStats, items]);

  const llmTotals = useMemo(() => {
    return llmSummary.reduce(
      (acc, r) => {
        acc.calls += r.calls;
        acc.cost += r.estimated_cost_usd;
        return acc;
      },
      { calls: 0, cost: 0 }
    );
  }, [llmSummary]);

  const latestDigests = digests.slice(0, 5);
  const latestSummaryDays = Array.from(
    new Map(llmSummary.map((r) => [r.date_jst, r.date_jst])).keys()
  )
    .slice(0, 5)
    .map((date) => ({
      date,
      rows: llmSummary.filter((r) => r.date_jst === date),
    }));

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <LayoutDashboard className="size-6 text-zinc-500" aria-hidden="true" />
            <span>{t("dashboard.title")}</span>
          </h1>
          <p className="mt-1 text-sm text-zinc-500">{t("dashboard.subtitle")}</p>
        </div>
        <button
          type="button"
          onClick={load}
          className="rounded border border-zinc-300 bg-white px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-50"
        >
          {t("common.refresh")}
        </button>
      </div>

      {loading && <p className="text-sm text-zinc-500">{t("common.loading")}</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <Card label={t("dashboard.card.sources")} value={String(sources.length)} />
        <Card label={t("dashboard.card.items")} value={String(itemStats?.total ?? items.length)} />
        <Card label={t("dashboard.card.failedItems")} value={String(itemStatus.failed ?? 0)} />
        <Card label={t("dashboard.card.digests")} value={String(digests.length)} />
        <Card label={t("dashboard.card.llmCalls")} value={String(llmTotals.calls)} />
        <Card label={t("dashboard.card.llmCost")} value={`$${llmTotals.cost.toFixed(6)}`} />
      </section>

      <section className="grid gap-4 lg:grid-cols-[1.1fr_1fr]">
        <div className="rounded-xl border border-zinc-200 bg-white p-4">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
              <BarChart3 className="size-4 text-zinc-500" aria-hidden="true" />
              <span>{t("dashboard.section.itemsStatus")}</span>
            </h2>
            <Link href="/items" className="text-xs text-zinc-500 hover:text-zinc-900">
              {locale === "ja" ? "記事一覧へ" : "Open Items"}
            </Link>
          </div>
          <div className="space-y-2">
            {(["new", "fetched", "facts_extracted", "summarized", "failed"] as const).map((key) => {
              const count = itemStatus[key] ?? 0;
              const total = Math.max(1, itemStats?.total ?? items.length);
              const ratio = (count / total) * 100;
              return (
                <div key={key}>
                  <div className="mb-1 flex items-center justify-between text-sm">
                    <span className="text-zinc-700">{t(`status.${key}`)}</span>
                    <span className="text-zinc-500">{count}</span>
                  </div>
                  <div className="h-2 rounded-full bg-zinc-100">
                    <div className="h-2 rounded-full bg-zinc-800" style={{ width: `${Math.max(2, ratio)}%` }} />
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <div className="rounded-xl border border-zinc-200 bg-white p-4">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
              <Mail className="size-4 text-zinc-500" aria-hidden="true" />
              <span>{t("dashboard.section.latestDigests")}</span>
            </h2>
            <Link href="/digests" className="text-xs text-zinc-500 hover:text-zinc-900">
              {locale === "ja" ? "一覧へ" : "View all"}
            </Link>
          </div>
          {latestDigests.length === 0 ? (
            <p className="text-sm text-zinc-400">{t("common.noData")}</p>
          ) : (
            <ul className="space-y-2">
              {latestDigests.map((d) => (
                <li key={d.id}>
                  <Link
                    href={`/digests/${d.id}`}
                    className="flex items-center justify-between rounded-lg border border-zinc-200 px-3 py-2 hover:bg-zinc-50"
                  >
                    <div>
                      <div className="text-sm font-medium text-zinc-900">{d.digest_date}</div>
                      <div className="text-xs text-zinc-400">
                        {new Date(d.created_at).toLocaleString(locale === "ja" ? "ja-JP" : "en-US")}
                      </div>
                    </div>
                    <span
                      className={`rounded px-2 py-0.5 text-xs font-medium ${
                        d.sent_at ? "bg-green-50 text-green-700" : "bg-zinc-100 text-zinc-500"
                      }`}
                    >
                      {d.sent_at ? t("digests.sent") : t("digests.pending")}
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <BarChart3 className="size-4 text-zinc-500" aria-hidden="true" />
            <span>{locale === "ja" ? "トレンドトピック（24h）" : "Trending Topics (24h)"}</span>
          </h2>
          <Link href="/items?feed=all&sort=score" className="text-xs text-zinc-500 hover:text-zinc-900">
            {locale === "ja" ? "記事一覧へ" : "Open Items"}
          </Link>
        </div>
        {topicTrends.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("common.noData")}</p>
        ) : (
          <div className="grid gap-2 md:grid-cols-2">
            {topicTrends.map((row) => (
              <Link
                key={row.topic}
                href={`/items?feed=all&sort=score&topic=${encodeURIComponent(row.topic)}`}
                className="rounded-lg border border-zinc-200 bg-zinc-50/70 px-3 py-2 transition-colors hover:border-zinc-300 hover:bg-zinc-100/70"
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="truncate text-sm font-medium text-zinc-900">{row.topic}</div>
                  <div
                    className={`shrink-0 rounded px-2 py-0.5 text-xs font-medium ${
                      row.delta > 0
                        ? "bg-green-50 text-green-700"
                        : row.delta < 0
                          ? "bg-zinc-100 text-zinc-600"
                          : "bg-blue-50 text-blue-700"
                    }`}
                  >
                    {row.delta > 0 ? "+" : ""}
                    {row.delta}
                  </div>
                </div>
                <div className="mt-1 flex items-center gap-3 text-xs text-zinc-500">
                  <span>{locale === "ja" ? `直近24h ${row.count_24h}` : `24h ${row.count_24h}`}</span>
                  <span>{locale === "ja" ? `前24h ${row.count_prev_24h}` : `prev ${row.count_prev_24h}`}</span>
                  {row.max_score_24h != null && <span>{`max score ${row.max_score_24h.toFixed(2)}`}</span>}
                </div>
              </Link>
            ))}
          </div>
        )}
      </section>

      <section className="rounded-xl border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="inline-flex items-center gap-2 text-sm font-semibold text-zinc-800">
            <Brain className="size-4 text-zinc-500" aria-hidden="true" />
            <span>{t("dashboard.section.llmSummary")}</span>
          </h2>
          <Link href="/llm-usage" className="text-xs text-zinc-500 hover:text-zinc-900">
            {locale === "ja" ? "LLM利用へ" : "Open LLM Usage"}
          </Link>
        </div>
        {latestSummaryDays.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("common.noData")}</p>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {latestSummaryDays.map(({ date, rows }) => {
              const cost = rows.reduce((a, r) => a + r.estimated_cost_usd, 0);
              const calls = rows.reduce((a, r) => a + r.calls, 0);
              return (
                <div key={date} className="rounded-lg border border-zinc-200 p-3">
                  <div className="mb-2 flex items-center justify-between">
                    <div className="text-sm font-medium text-zinc-900">{date}</div>
                    <div className="text-xs text-zinc-500">${cost.toFixed(6)}</div>
                  </div>
                  <div className="mb-2 text-xs text-zinc-500">
                    {locale === "ja" ? `呼び出し ${calls}` : `Calls ${calls}`}
                  </div>
                  <div className="space-y-1">
                    {rows.slice(0, 4).map((r) => (
                      <div key={`${r.date_jst}-${r.purpose}-${r.pricing_source}`} className="flex items-center justify-between text-xs">
                        <span className="text-zinc-600">{r.purpose}</span>
                        <span className="text-zinc-500">{r.calls}</span>
                      </div>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-zinc-200 bg-white p-4 shadow-sm">
      <div className="text-xs font-medium text-zinc-500">{label}</div>
      <div className="mt-2 text-2xl font-semibold tracking-tight text-zinc-900">{value}</div>
    </div>
  );
}
