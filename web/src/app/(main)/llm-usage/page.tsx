"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { api, LLMUsageDailySummary, LLMUsageLog } from "@/lib/api";
import Pagination from "@/components/pagination";
import { useI18n } from "@/components/i18n-provider";

function fmtUSD(v: number) {
  return `$${v.toFixed(6)}`;
}

function fmtNum(v: number) {
  return new Intl.NumberFormat("ja-JP").format(v);
}

type SummaryRow = LLMUsageDailySummary & {
  key: string;
};

export default function LLMUsagePage() {
  const { t, locale } = useI18n();
  const [days, setDays] = useState(14);
  const [limit, setLimit] = useState(100);
  const [logPage, setLogPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [summaryRows, setSummaryRows] = useState<LLMUsageDailySummary[]>([]);
  const [logs, setLogs] = useState<LLMUsageLog[]>([]);

  const load = useCallback(async (daysParam: number, limitParam: number) => {
    setLoading(true);
    try {
      const [summary, recent] = await Promise.all([
        api.getLLMUsageSummary({ days: daysParam }),
        api.getLLMUsage({ limit: limitParam }),
      ]);
      setSummaryRows(summary ?? []);
      setLogs(recent ?? []);
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
      cacheWrite: 0,
      cacheRead: 0,
      cost: 0,
    };
    for (const r of summaryRows) {
      t.calls += r.calls;
      t.input += r.input_tokens;
      t.output += r.output_tokens;
      t.cacheWrite += r.cache_creation_input_tokens;
      t.cacheRead += r.cache_read_input_tokens;
      t.cost += r.estimated_cost_usd;
    }
    return t;
  }, [summaryRows]);

  const groupedByDate = useMemo(() => {
    const m = new Map<string, SummaryRow[]>();
    for (const row of summaryRows) {
      const key = `${row.date_jst}:${row.purpose}:${row.pricing_source}`;
      const list = m.get(row.date_jst) ?? [];
      list.push({ ...row, key });
      m.set(row.date_jst, list);
    }
    return Array.from(m.entries());
  }, [summaryRows]);

  const logsPageSize = 20;
  const pagedLogs = logs.slice((logPage - 1) * logsPageSize, logPage * logsPageSize);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">LLM Usage</h1>
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
                  {locale === "ja" ? `${d}日` : `${d}d`}
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

      <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-6">
        <MetricCard label={t("llm.totalCost")} value={fmtUSD(totals.cost)} />
        <MetricCard label={t("llm.totalCalls")} value={fmtNum(totals.calls)} />
        <MetricCard label={t("llm.input")} value={fmtNum(totals.input)} />
        <MetricCard label={t("llm.output")} value={fmtNum(totals.output)} />
        <MetricCard label="Cache Write" value={fmtNum(totals.cacheWrite)} />
        <MetricCard label="Cache Read" value={fmtNum(totals.cacheRead)} />
      </section>

      <section className="rounded-lg border border-zinc-200 bg-white p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-zinc-800">{t("llm.dailySummary")}</h2>
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
          <h2 className="text-sm font-semibold text-zinc-800">{t("llm.recentLogs")}</h2>
          <span className="text-xs text-zinc-400">{logs.length} rows</span>
        </div>
        {logs.length === 0 ? (
          <p className="text-sm text-zinc-400">{t("llm.noLogs")}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="text-xs text-zinc-500">
                <tr className="border-b border-zinc-100">
                  <th className="px-3 py-2 text-left font-medium">時刻</th>
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
                      <div className="max-w-[260px] break-all text-xs text-zinc-700">{r.model}</div>
                      {r.pricing_model_family && (
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

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-zinc-200 bg-white px-4 py-3">
      <div className="text-xs font-medium text-zinc-500">{label}</div>
      <div className="mt-1 text-lg font-semibold text-zinc-900">{value}</div>
    </div>
  );
}
