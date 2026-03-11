"use client";

import Pagination from "@/components/pagination";
import type {
  LLMExecutionCurrentMonthSummary,
  LLMUsageDailySummary,
  LLMUsageLog,
  LLMUsageProviderMonthSummary,
  LLMUsagePurposeMonthSummary,
} from "@/lib/api";

export function MetricCard({ label, value, className = "" }: { label: string; value: string; className?: string }) {
  return (
    <div className={`min-w-0 w-full rounded-lg border border-zinc-200 bg-white px-4 py-3 ${className}`.trim()}>
      <div className="text-xs font-medium text-zinc-500">{label}</div>
      <div className="mt-1 truncate text-lg font-semibold text-zinc-900">{value}</div>
    </div>
  );
}

export function CurrentMonthByProviderTable({
  title,
  rows,
  monthLabel,
  totalCostLabel,
  noSummaryLabel,
  fmtNum,
  fmtUSD,
}: {
  title: string;
  rows: Array<LLMUsageProviderMonthSummary & { share_pct: number; call_share_pct: number; avg_cost_per_call_usd: number }>;
  monthLabel: string;
  totalCostLabel: string;
  noSummaryLabel: string;
  fmtNum: (v: number) => string;
  fmtUSD: (v: number) => string;
}) {
  return (
    <section className="rounded-lg border border-zinc-200 bg-white p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-zinc-800">{title}</h2>
        <span className="text-xs text-zinc-400">{monthLabel} / total {totalCostLabel}</span>
      </div>
      {rows.length === 0 ? (
        <p className="text-sm text-zinc-400">{noSummaryLabel}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="text-xs text-zinc-500">
              <tr className="border-b border-zinc-100">
                <th className="px-3 py-2 text-left font-medium">provider</th>
                <th className="px-3 py-2 text-right font-medium">calls</th>
                <th className="px-3 py-2 text-right font-medium">input</th>
                <th className="px-3 py-2 text-right font-medium">output</th>
                <th className="px-3 py-2 text-right font-medium">cache r</th>
                <th className="px-3 py-2 text-right font-medium">call share</th>
                <th className="px-3 py-2 text-right font-medium">share</th>
                <th className="px-3 py-2 text-right font-medium">avg/call</th>
                <th className="px-3 py-2 text-right font-medium">cost</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${row.month_jst}:${row.provider}`} className="border-b border-zinc-100 last:border-0">
                  <td className="px-3 py-2 font-medium text-zinc-800">{row.provider}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.calls)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.input_tokens)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.output_tokens)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.cache_read_input_tokens)}</td>
                  <td className="px-3 py-2 text-right">{row.call_share_pct.toFixed(1)}%</td>
                  <td className="px-3 py-2 text-right">{row.share_pct.toFixed(1)}%</td>
                  <td className="px-3 py-2 text-right">{fmtUSD(row.avg_cost_per_call_usd)}</td>
                  <td className="px-3 py-2 text-right">{fmtUSD(row.estimated_cost_usd)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

export function CurrentMonthByPurposeTable({
  title,
  rows,
  monthLabel,
  noSummaryLabel,
  fmtNum,
  fmtUSD,
}: {
  title: string;
  rows: Array<LLMUsagePurposeMonthSummary & { share_pct: number; call_share_pct: number; avg_cost_per_call_usd: number }>;
  monthLabel: string;
  noSummaryLabel: string;
  fmtNum: (v: number) => string;
  fmtUSD: (v: number) => string;
}) {
  return (
    <section className="rounded-lg border border-zinc-200 bg-white p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-zinc-800">{title}</h2>
        <span className="text-xs text-zinc-400">{monthLabel}</span>
      </div>
      {rows.length === 0 ? (
        <p className="text-sm text-zinc-400">{noSummaryLabel}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="text-xs text-zinc-500">
              <tr className="border-b border-zinc-100">
                <th className="px-3 py-2 text-left font-medium">purpose</th>
                <th className="px-3 py-2 text-right font-medium">calls</th>
                <th className="px-3 py-2 text-right font-medium">input</th>
                <th className="px-3 py-2 text-right font-medium">output</th>
                <th className="px-3 py-2 text-right font-medium">cache r</th>
                <th className="px-3 py-2 text-right font-medium">call share</th>
                <th className="px-3 py-2 text-right font-medium">share</th>
                <th className="px-3 py-2 text-right font-medium">avg/call</th>
                <th className="px-3 py-2 text-right font-medium">cost</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${row.month_jst}:${row.purpose}`} className="border-b border-zinc-100 last:border-0">
                  <td className="px-3 py-2 font-medium text-zinc-800">{row.purpose}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.calls)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.input_tokens)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.output_tokens)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.cache_read_input_tokens)}</td>
                  <td className="px-3 py-2 text-right">{row.call_share_pct.toFixed(1)}%</td>
                  <td className="px-3 py-2 text-right">{row.share_pct.toFixed(1)}%</td>
                  <td className="px-3 py-2 text-right">{fmtUSD(row.avg_cost_per_call_usd)}</td>
                  <td className="px-3 py-2 text-right">{fmtUSD(row.estimated_cost_usd)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

export function ReliabilityTable({
  rows,
  monthLabel,
  noSummaryLabel,
  fmtNum,
  labels,
}: {
  rows: LLMExecutionCurrentMonthSummary[];
  monthLabel: string;
  noSummaryLabel: string;
  fmtNum: (v: number) => string;
  labels: {
    title: string;
    attempts: string;
    failures: string;
    failureRate: string;
    retries: string;
    retryRate: string;
    emptyResponses: string;
    emptyRate: string;
  };
}) {
  return (
    <section className="rounded-lg border border-zinc-200 bg-white p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-zinc-800">{labels.title}</h2>
        <span className="text-xs text-zinc-400">{monthLabel}</span>
      </div>
      {rows.length === 0 ? (
        <p className="text-sm text-zinc-400">{noSummaryLabel}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="text-xs text-zinc-500">
              <tr className="border-b border-zinc-100">
                <th className="px-3 py-2 text-left font-medium">purpose</th>
                <th className="px-3 py-2 text-left font-medium">model</th>
                <th className="px-3 py-2 text-right font-medium">{labels.attempts}</th>
                <th className="px-3 py-2 text-right font-medium">{labels.failures}</th>
                <th className="px-3 py-2 text-right font-medium">{labels.failureRate}</th>
                <th className="px-3 py-2 text-right font-medium">{labels.retries}</th>
                <th className="px-3 py-2 text-right font-medium">{labels.retryRate}</th>
                <th className="px-3 py-2 text-right font-medium">{labels.emptyResponses}</th>
                <th className="px-3 py-2 text-right font-medium">{labels.emptyRate}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${row.month_jst}:${row.purpose}:${row.provider}:${row.model}`} className="border-b border-zinc-100 last:border-0">
                  <td className="px-3 py-2 font-medium text-zinc-800">{row.purpose}</td>
                  <td className="px-3 py-2 text-zinc-700 whitespace-nowrap">{row.provider}/{row.model}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.attempts)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.failures)}</td>
                  <td className="px-3 py-2 text-right">{row.failure_rate_pct.toFixed(1)}%</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.retries)}</td>
                  <td className="px-3 py-2 text-right">{row.retry_rate_pct.toFixed(1)}%</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.empty_responses)}</td>
                  <td className="px-3 py-2 text-right">{row.empty_rate_pct.toFixed(1)}%</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

export function DailySummaryGroups({
  title,
  groupedByDate,
  noSummaryLabel,
  fmtNum,
  fmtUSD,
}: {
  title: string;
  groupedByDate: Array<[string, Array<LLMUsageDailySummary & { key: string }>]>;
  noSummaryLabel: string;
  fmtNum: (v: number) => string;
  fmtUSD: (v: number) => string;
}) {
  return (
    <section className="rounded-lg border border-zinc-200 bg-white p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-zinc-800">{title}</h2>
        <span className="text-xs text-zinc-400">{groupedByDate.length} rows</span>
      </div>
      {groupedByDate.length === 0 ? (
        <p className="text-sm text-zinc-400">{noSummaryLabel}</p>
      ) : (
        <div className="space-y-4">
          {groupedByDate.map(([date, rows]) => {
            const dayCost = rows.reduce((acc, r) => acc + r.estimated_cost_usd, 0);
            const dayCalls = rows.reduce((acc, r) => acc + r.calls, 0);
            return (
              <div key={date} className="rounded border border-zinc-200">
                <div className="flex items-center justify-between border-b border-zinc-200 bg-zinc-50 px-3 py-2">
                  <div className="text-sm font-medium text-zinc-800">{date}</div>
                  <div className="text-xs text-zinc-500">calls {fmtNum(dayCalls)} / cost {fmtUSD(dayCost)}</div>
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
                        <th className="px-3 py-2 text-right font-medium">cache w</th>
                        <th className="px-3 py-2 text-right font-medium">cache r</th>
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
                          <td className="px-3 py-2 text-right">{fmtNum(r.cache_creation_input_tokens)}</td>
                          <td className="px-3 py-2 text-right">{fmtNum(r.cache_read_input_tokens)}</td>
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
  );
}

export function RecentLogsTable({
  logs,
  pagedLogs,
  logPage,
  setLogPage,
  logsPageSize,
  locale,
  noLogsLabel,
  labels,
  fmtNum,
  fmtUSD,
}: {
  logs: LLMUsageLog[];
  pagedLogs: LLMUsageLog[];
  logPage: number;
  setLogPage: (page: number) => void;
  logsPageSize: number;
  locale: string;
  noLogsLabel: string;
  labels: { title: string; time: string };
  fmtNum: (v: number) => string;
  fmtUSD: (v: number) => string;
}) {
  return (
    <section className="rounded-lg border border-zinc-200 bg-white p-4">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-zinc-800">{labels.title}</h2>
        <span className="text-xs text-zinc-400">{logs.length} rows</span>
      </div>
      {logs.length === 0 ? (
        <p className="text-sm text-zinc-400">{noLogsLabel}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="text-xs text-zinc-500">
              <tr className="border-b border-zinc-100">
                <th className="px-3 py-2 text-left font-medium">{labels.time}</th>
                <th className="px-3 py-2 text-left font-medium">purpose</th>
                <th className="px-3 py-2 text-left font-medium">model</th>
                <th className="px-3 py-2 text-left font-medium">pricing</th>
                <th className="px-3 py-2 text-right font-medium">in</th>
                <th className="px-3 py-2 text-right font-medium">out</th>
                <th className="px-3 py-2 text-right font-medium">cache w</th>
                <th className="px-3 py-2 text-right font-medium">cache r</th>
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
                  <td className="px-3 py-2 text-right">{fmtNum(r.cache_creation_input_tokens)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(r.cache_read_input_tokens)}</td>
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
  );
}
