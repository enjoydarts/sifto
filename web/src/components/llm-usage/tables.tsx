"use client";

import Pagination from "@/components/pagination";
import { formatModelDisplayName } from "@/lib/model-display";
import type {
  LLMExecutionCurrentMonthSummary,
  LLMUsageDailySummary,
  LLMUsageLog,
  LLMUsageProviderMonthSummary,
  LLMUsagePurposeMonthSummary,
} from "@/lib/api";

function providerLabel(provider: string) {
  switch (provider) {
    case "openai":
      return "OpenAI";
    case "anthropic":
      return "Anthropic";
    case "google":
      return "Google";
    case "groq":
      return "Groq";
    case "deepseek":
      return "DeepSeek";
    case "alibaba":
      return "Alibaba";
    case "mistral":
      return "Mistral";
    case "xai":
      return "xAI";
    case "zai":
      return "Z.ai";
    case "fireworks":
      return "Fireworks";
    case "moonshot":
      return "Moonshot";
    case "openrouter":
      return "OpenRouter";
    case "poe":
      return "Poe";
    case "siliconflow":
      return "SiliconFlow";
    default:
      return provider;
  }
}

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
  sortKey,
  sortDir,
  onSort,
}: {
  title: string;
  rows: Array<LLMUsageProviderMonthSummary & {
    share_pct: number;
    call_share_pct: number;
    avg_cost_per_call_usd: number;
    avg_input_tokens_per_call: number;
    avg_output_tokens_per_call: number;
  }>;
  monthLabel: string;
  totalCostLabel: string;
  noSummaryLabel: string;
  fmtNum: (v: number) => string;
  fmtUSD: (v: number) => string;
  sortKey: string;
  sortDir: "asc" | "desc";
  onSort: (key: string) => void;
}) {
  const renderSortMark = (key: string) => {
    if (sortKey !== key) return null;
    return <span className="ml-1 text-zinc-400">{sortDir === "asc" ? "↑" : "↓"}</span>;
  };
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
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("provider")} className="inline-flex items-center hover:text-zinc-800">provider{renderSortMark("provider")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("calls")} className="inline-flex items-center hover:text-zinc-800">calls{renderSortMark("calls")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("input_tokens")} className="inline-flex items-center hover:text-zinc-800">input{renderSortMark("input_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("output_tokens")} className="inline-flex items-center hover:text-zinc-800">output{renderSortMark("output_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("avg_input_tokens_per_call")} className="inline-flex items-center hover:text-zinc-800">avg in/call{renderSortMark("avg_input_tokens_per_call")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("avg_output_tokens_per_call")} className="inline-flex items-center hover:text-zinc-800">avg out/call{renderSortMark("avg_output_tokens_per_call")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("cache_read_input_tokens")} className="inline-flex items-center hover:text-zinc-800">cache r{renderSortMark("cache_read_input_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("call_share_pct")} className="inline-flex items-center hover:text-zinc-800">call share{renderSortMark("call_share_pct")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("share_pct")} className="inline-flex items-center hover:text-zinc-800">share{renderSortMark("share_pct")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("avg_cost_per_call_usd")} className="inline-flex items-center hover:text-zinc-800">avg/call{renderSortMark("avg_cost_per_call_usd")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("estimated_cost_usd")} className="inline-flex items-center hover:text-zinc-800">cost{renderSortMark("estimated_cost_usd")}</button></th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${row.month_jst}:${row.provider}`} className="border-b border-zinc-100 last:border-0">
                  <td className="px-3 py-2 font-medium text-zinc-800">{providerLabel(row.provider)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.calls)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.input_tokens)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.output_tokens)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(Math.round(row.avg_input_tokens_per_call))}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(Math.round(row.avg_output_tokens_per_call))}</td>
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
  sortKey,
  sortDir,
  onSort,
}: {
  title: string;
  rows: Array<LLMUsagePurposeMonthSummary & {
    share_pct: number;
    call_share_pct: number;
    avg_cost_per_call_usd: number;
    avg_input_tokens_per_call: number;
    avg_output_tokens_per_call: number;
  }>;
  monthLabel: string;
  noSummaryLabel: string;
  fmtNum: (v: number) => string;
  fmtUSD: (v: number) => string;
  sortKey: string;
  sortDir: "asc" | "desc";
  onSort: (key: string) => void;
}) {
  const renderSortMark = (key: string) => {
    if (sortKey !== key) return null;
    return <span className="ml-1 text-zinc-400">{sortDir === "asc" ? "↑" : "↓"}</span>;
  };
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
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("purpose")} className="inline-flex items-center hover:text-zinc-800">purpose{renderSortMark("purpose")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("calls")} className="inline-flex items-center hover:text-zinc-800">calls{renderSortMark("calls")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("input_tokens")} className="inline-flex items-center hover:text-zinc-800">input{renderSortMark("input_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("output_tokens")} className="inline-flex items-center hover:text-zinc-800">output{renderSortMark("output_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("avg_input_tokens_per_call")} className="inline-flex items-center hover:text-zinc-800">avg in/call{renderSortMark("avg_input_tokens_per_call")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("avg_output_tokens_per_call")} className="inline-flex items-center hover:text-zinc-800">avg out/call{renderSortMark("avg_output_tokens_per_call")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("cache_read_input_tokens")} className="inline-flex items-center hover:text-zinc-800">cache r{renderSortMark("cache_read_input_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("call_share_pct")} className="inline-flex items-center hover:text-zinc-800">call share{renderSortMark("call_share_pct")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("share_pct")} className="inline-flex items-center hover:text-zinc-800">share{renderSortMark("share_pct")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("avg_cost_per_call_usd")} className="inline-flex items-center hover:text-zinc-800">avg/call{renderSortMark("avg_cost_per_call_usd")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("estimated_cost_usd")} className="inline-flex items-center hover:text-zinc-800">cost{renderSortMark("estimated_cost_usd")}</button></th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${row.month_jst}:${row.purpose}`} className="border-b border-zinc-100 last:border-0">
                  <td className="px-3 py-2 font-medium text-zinc-800">{row.purpose}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.calls)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.input_tokens)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.output_tokens)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(Math.round(row.avg_input_tokens_per_call))}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(Math.round(row.avg_output_tokens_per_call))}</td>
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
  fmtUSD,
  labels,
  sortKey,
  sortDir,
  onSort,
}: {
  rows: LLMExecutionCurrentMonthSummary[];
  monthLabel: string;
  noSummaryLabel: string;
  fmtNum: (v: number) => string;
  fmtUSD: (v: number) => string;
  labels: {
    title: string;
    attempts: string;
    cost: string;
    failures: string;
    failureRate: string;
    retries: string;
    retryRate: string;
    emptyResponses: string;
    emptyRate: string;
  };
  sortKey: string;
  sortDir: "asc" | "desc";
  onSort: (key: string) => void;
}) {
  const headerClass = "px-3 py-2 font-medium";
  const renderSortMark = (key: string) => {
    if (sortKey !== key) return null;
    return <span className="ml-1 text-zinc-400">{sortDir === "asc" ? "↑" : "↓"}</span>;
  };

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
                <th className={`${headerClass} text-left`}>
                  <button type="button" onClick={() => onSort("purpose")} className="inline-flex items-center hover:text-zinc-800">
                    purpose{renderSortMark("purpose")}
                  </button>
                </th>
                <th className={`${headerClass} text-left`}>
                  <button type="button" onClick={() => onSort("model")} className="inline-flex items-center hover:text-zinc-800">
                    model{renderSortMark("model")}
                  </button>
                </th>
                <th className={`${headerClass} text-right`}>
                  <button type="button" onClick={() => onSort("attempts")} className="inline-flex items-center hover:text-zinc-800">
                    {labels.attempts}{renderSortMark("attempts")}
                  </button>
                </th>
                <th className={`${headerClass} text-right`}>
                  <button type="button" onClick={() => onSort("estimated_cost_usd")} className="inline-flex items-center hover:text-zinc-800">
                    {labels.cost}{renderSortMark("estimated_cost_usd")}
                  </button>
                </th>
                <th className={`${headerClass} text-right`}>
                  <button type="button" onClick={() => onSort("failures")} className="inline-flex items-center hover:text-zinc-800">
                    {labels.failures}{renderSortMark("failures")}
                  </button>
                </th>
                <th className={`${headerClass} text-right`}>
                  <button type="button" onClick={() => onSort("failure_rate_pct")} className="inline-flex items-center hover:text-zinc-800">
                    {labels.failureRate}{renderSortMark("failure_rate_pct")}
                  </button>
                </th>
                <th className={`${headerClass} text-right`}>
                  <button type="button" onClick={() => onSort("retries")} className="inline-flex items-center hover:text-zinc-800">
                    {labels.retries}{renderSortMark("retries")}
                  </button>
                </th>
                <th className={`${headerClass} text-right`}>
                  <button type="button" onClick={() => onSort("retry_rate_pct")} className="inline-flex items-center hover:text-zinc-800">
                    {labels.retryRate}{renderSortMark("retry_rate_pct")}
                  </button>
                </th>
                <th className={`${headerClass} text-right`}>
                  <button type="button" onClick={() => onSort("empty_responses")} className="inline-flex items-center hover:text-zinc-800">
                    {labels.emptyResponses}{renderSortMark("empty_responses")}
                  </button>
                </th>
                <th className={`${headerClass} text-right`}>
                  <button type="button" onClick={() => onSort("empty_rate_pct")} className="inline-flex items-center hover:text-zinc-800">
                    {labels.emptyRate}{renderSortMark("empty_rate_pct")}
                  </button>
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${row.month_jst}:${row.purpose}:${row.provider}:${row.model}`} className="border-b border-zinc-100 last:border-0">
                  <td className="px-3 py-2 font-medium text-zinc-800">{row.purpose}</td>
                  <td className="px-3 py-2 text-zinc-700 whitespace-nowrap">{providerLabel(row.provider)}/{formatModelDisplayName(row.model)}</td>
                  <td className="px-3 py-2 text-right">{fmtNum(row.attempts)}</td>
                  <td className="px-3 py-2 text-right">{fmtUSD(row.estimated_cost_usd)}</td>
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
                          <td className="px-3 py-2">{providerLabel(r.provider)}</td>
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
  sortKey,
  sortDir,
  onSort,
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
  sortKey: string;
  sortDir: "asc" | "desc";
  onSort: (key: string) => void;
}) {
  const renderSortMark = (key: string) => {
    if (sortKey !== key) return null;
    return <span className="ml-1 text-zinc-400">{sortDir === "asc" ? "↑" : "↓"}</span>;
  };
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
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("created_at")} className="inline-flex items-center hover:text-zinc-800">{labels.time}{renderSortMark("created_at")}</button></th>
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("purpose")} className="inline-flex items-center hover:text-zinc-800">purpose{renderSortMark("purpose")}</button></th>
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("model")} className="inline-flex items-center hover:text-zinc-800">model{renderSortMark("model")}</button></th>
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("pricing_source")} className="inline-flex items-center hover:text-zinc-800">pricing{renderSortMark("pricing_source")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("input_tokens")} className="inline-flex items-center hover:text-zinc-800">in{renderSortMark("input_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("output_tokens")} className="inline-flex items-center hover:text-zinc-800">out{renderSortMark("output_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("cache_creation_input_tokens")} className="inline-flex items-center hover:text-zinc-800">cache w{renderSortMark("cache_creation_input_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("cache_read_input_tokens")} className="inline-flex items-center hover:text-zinc-800">cache r{renderSortMark("cache_read_input_tokens")}</button></th>
                <th className="px-3 py-2 text-right font-medium"><button type="button" onClick={() => onSort("estimated_cost_usd")} className="inline-flex items-center hover:text-zinc-800">cost{renderSortMark("estimated_cost_usd")}</button></th>
                <th className="px-3 py-2 text-left font-medium"><button type="button" onClick={() => onSort("ref")} className="inline-flex items-center hover:text-zinc-800">ref{renderSortMark("ref")}</button></th>
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
